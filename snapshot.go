package serf

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	flushInterval                 = 500 * time.Millisecond
	clockUpdateInterval           = 500 * time.Millisecond
	snapshotErrorRecoveryInterval = 30 * time.Second
	eventChSize                   = 2048
	// drainTimeout                  = 250 * time.Millisecond
	compactExt               = ".compact"
	snapshotBytesPerNode     = 128
	snapshotCompactionFactor = 2 // compact_threshold = numNodes * bytesPerNode * factor
)

var snapshotPrefixes = []string{"alive: ", "not-alive: ", "action-clock: ", "query-clock: ", "coordinate: ", "leave", "#"}

type Snapshotter struct {
	aliveNodes      map[string]string
	path            string
	fh              *os.File
	offset          int64
	buffer          *bufio.Writer
	teeCh           chan Event
	lastActionClock LamportTime
	lastQueryClock  LamportTime
	lastFlush       time.Time
	leaveCh         chan struct{}
	leaving         bool
	logger          *log.Logger
	minCompactSize  int64
	drainTimeout    time.Duration
	stopCh          chan struct{} // to wait for draining events when shutdownCh fires
	shutdownCh      chan struct{} // serf's shutdownCh
	lastRecovery    time.Time
}

type NodeIDAddr struct {
	ID   string
	Addr string
}

func (p *NodeIDAddr) String() string {
	return fmt.Sprintf("%s: %s", p.ID, p.Addr)
}

func NewSnapshotter(path string,
	minCompactSize int,
	drainTimeout time.Duration,
	logger *log.Logger,
	inCh chan Event,
	shutdownCh chan struct{},
) (*Snapshotter, chan Event, error) {

	fh, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open snapshot: %v", err)
	}

	offset, err := fh.Seek(0, io.SeekEnd)
	if err != nil {
		fh.Close()
		return nil, nil, fmt.Errorf("failed to stat snapshot: %v", err)
	}

	// Create the snapshotter
	snap := &Snapshotter{
		aliveNodes:      make(map[string]string),
		path:            path,
		fh:              fh,
		buffer:          bufio.NewWriter(fh),
		offset:          offset,
		teeCh:           make(chan Event, eventChSize),
		lastActionClock: 0,
		lastQueryClock:  0,
		leaveCh:         make(chan struct{}),
		logger:          logger,
		minCompactSize:  int64(minCompactSize),
		drainTimeout:    drainTimeout,
		stopCh:          make(chan struct{}),
		shutdownCh:      shutdownCh,
	}
	// restore snapshotter from log
	if err := snap.replay(); err != nil {
		fh.Close()
		return nil, nil, err
	}

	outCh := make(chan Event, eventChSize)
	// process events from inCh
	go snap.teeEvents(inCh, outCh)
	go snap.receiveEvents()

	return snap, outCh, nil
}

func forwardEvent(e Event, ch chan Event) {
	select {
	case ch <- e:
	default:
	}
}

func (s *Snapshotter) teeEvents(inCh, outCh chan Event) {
	// drain remaining events to snapshotter
	defer func() {
		for {
			select {
			case e := <-inCh:
				forwardEvent(e, s.teeCh)
			default:
				return
			}
		}
	}()

	for {
		select {
		case e := <-inCh:
			forwardEvent(e, s.teeCh)
			forwardEvent(e, outCh)
		case <-s.shutdownCh:
			return
		}
	}
}

func (s *Snapshotter) receiveEvents() {

	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()

	// flush buffer and close file
	defer func() {
		if err := s.buffer.Flush(); err != nil {
			s.logger.Printf("[ERR] serf: failed to flush snapshot: %v", err)
		}
		if err := s.fh.Sync(); err != nil {
			s.logger.Printf("[ERR] serf: failed to sync snapshot: %v", err)
		}
		s.fh.Close()
		close(s.stopCh)
	}()
	// drain all events in teeCh
	defer func() {
		drainTimeout := time.After(s.drainTimeout)
		for {
			select {
			case e := <-s.teeCh:
				s.recordEvent(e)
			case <-drainTimeout:
				return
			}
		}
	}()

	for {
		select {
		case <-s.leaveCh:
			s.leaving = true // only accessed sequentially in this loop, no need for lock
			s.tryAppend("leave\n")
			if err := s.buffer.Flush(); err != nil {
				s.logger.Printf("[ERR] serf: failed to flush leave to snapshot: %v", err)
			}
		case e := <-s.teeCh:
			s.recordEvent(e)
		case <-flushTicker.C:
			if err := s.buffer.Flush(); err != nil {
				s.recover()
			}
		case <-s.shutdownCh:
			return // run the cleanup
		}
	}
}

// only call in receiveEvents
func (s *Snapshotter) recordEvent(e Event) {
	if s.leaving { // no need for lock because accessed sequentially in receiveEvents loop
		return
	}
	switch event := e.(type) {
	case *MemberEvent:
		s.recordMemberEvent(event)
	case *ActionEvent:
		s.recordActionClock(event.LTime)
	case *QueryEvent:
		s.recordQueryClock(event.LTime)
	default:
		s.logger.Printf("[ERR] serf: Unknown event to snapshot: %#v", e)
	}
}

// at new node to aliveNodes and add a log alive.
// delete failed or left node from aliveNodes and add a log not-alive
func (s *Snapshotter) recordMemberEvent(e *MemberEvent) {
	switch e.Type {
	case EventMemberJoin:
		m := e.Member
		if m == nil {
			return
		}
		addr := net.TCPAddr{IP: m.IP, Port: int(m.Port)}
		s.aliveNodes[m.ID] = addr.String()
		s.tryAppend(fmt.Sprintf("alive: %s %s\n", m.ID, addr.String()))
	case EventMemberFailed:
		fallthrough
	case EventMemberLeave:
		m := e.Member
		delete(s.aliveNodes, m.ID)
		s.tryAppend(fmt.Sprintf("not-alive: %s\n", m.ID))
	}
}

func (s *Snapshotter) recordActionClock(lTime LamportTime) {
	// Ignore old clocks
	if lTime <= s.lastActionClock {
		return
	}
	s.lastActionClock = lTime
	s.tryAppend(fmt.Sprintf("action-clock: %d\n", lTime))
}

func (s *Snapshotter) recordQueryClock(lTime LamportTime) {
	// Ignore old clocks
	if lTime <= s.lastQueryClock {
		return
	}
	s.lastQueryClock = lTime
	s.tryAppend(fmt.Sprintf("query-clock: %d\n", lTime))
}

func (s *Snapshotter) appendLine(l string) error {
	n, err := s.buffer.WriteString(l) // write to buffer
	if err != nil {
		return err
	}
	s.offset += int64(n)
	// exceed threshold? compact
	if s.offset > s.snapshotThreshold() {
		return s.compact()
	}
	return nil
}

func (s *Snapshotter) tryAppend(l string) {
	if err := s.appendLine(l); err != nil {
		s.logger.Printf("[ERR] serf: Failed to update snapshot: %v", err)
		s.recover()
	}
}

func (s *Snapshotter) recover() {
	now := time.Now()
	if now.Sub(s.lastRecovery) > snapshotErrorRecoveryInterval {
		s.lastRecovery = now
		s.logger.Printf("[INFO] serf: Attempting compaction to recover from error...")
		err := s.compact()
		if err != nil {
			s.logger.Printf("[ERR] serf: Compaction failed, will reattempt after %v: %v", snapshotErrorRecoveryInterval, err)
		} else {
			s.logger.Printf("[INFO] serf: Finished compaction, successfully recovered from error state")
		}
	}
}

// threshold = nodes * bytesPerNode * factor
func (s *Snapshotter) snapshotThreshold() int64 {
	nodes := int64(len(s.aliveNodes))
	threshold := nodes * snapshotBytesPerNode * snapshotCompactionFactor

	// Apply a minimum threshold to avoid frequent compaction
	if threshold < s.minCompactSize {
		threshold = s.minCompactSize
	}
	return threshold
}

// dump aliveNodes, clocks from snapshotter to a new file
func (s *Snapshotter) dump() error {
	newPath := s.path + compactExt
	fh, err := os.OpenFile(newPath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("failed to open new path to dump snapshot: %w", err)
	}
	defer fh.Close()

	buf := bufio.NewWriter(fh) // buffered writing to file

	// Write out the live nodes
	for name, addr := range s.aliveNodes {
		line := fmt.Sprintf("alive: %s %s\n", name, addr)
		_, err := buf.WriteString(line)
		if err != nil {
			return err
		}
	}

	// Write out the event clock
	line := fmt.Sprintf("action-clock: %d\n", s.lastActionClock)
	_, err = buf.WriteString(line)
	if err != nil {
		return err
	}

	line = fmt.Sprintf("query-clock: %d\n", s.lastQueryClock)
	_, err = buf.WriteString(line)
	if err != nil {
		return err
	}

	// Write a leaveMsg!
	if s.leaving {
		_, err := buf.WriteString("leave\n")
		if err != nil {
			return err
		}
	}
	// Flush the new snapshot
	if err := buf.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffered of snapshot dump: %w", err)
	}
	if err := fh.Sync(); err != nil { // immediately flushes data in memory to disk
		return fmt.Errorf("failed to fsync new file for snapshot dump: %w", err)
	}
	return nil
}

func (s *Snapshotter) compact() error {
	// dump snapshotter to new file
	err := s.dump()
	if err != nil {
		return fmt.Errorf("fail to dump snapshotter %w", err)
	}

	// flush and delete old file
	s.buffer.Flush()
	s.buffer = nil
	s.fh.Close()
	s.fh = nil
	if err := os.Remove(s.path); err != nil {
		return fmt.Errorf("failed to remove old snapshot: %w", err)
	}

	// move the new file in
	dumpPath := s.path + compactExt
	if err := os.Rename(dumpPath, s.path); err != nil {
		return fmt.Errorf("failed to install new snapshot: %w", err)
	}

	// create the new handle and buffer
	fh, err := os.OpenFile(s.path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0755)
	if err != nil {
		return fmt.Errorf("failed to open snapshot: %v", err)
	}
	buf := bufio.NewWriter(fh)
	offset, err := fh.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// use new handle and buffer
	s.fh = fh
	s.buffer = buf
	s.offset = offset
	s.lastFlush = time.Now()
	return nil
}

func (s *Snapshotter) handleLine(l string) {
	var prefix string
	for _, p := range snapshotPrefixes {
		if strings.HasPrefix(l, p) {
			prefix = p
			break
		}
	}
	if prefix == "" {
		s.logger.Printf("[WARN] serf snapshoter: prefix not found %s", l)
		return
	}
	info := strings.TrimPrefix(l, prefix)
	switch prefix {
	case "alive: ":
		addrIdx := strings.LastIndex(info, " ")
		if addrIdx == -1 {
			s.logger.Printf("[WARN] serf: Failed to parse address: %v", l)
		}
		addr := info[addrIdx+1:]
		name := info[:addrIdx]
		s.aliveNodes[name] = addr // TODO: NOT USEFUL, AND MAY NOT CORRECTLY REFLECT THE NODES WHEN REJOIN. CONSIDER IGNORE!
	case "not-alive: ":
		delete(s.aliveNodes, info)
	case "action-clock: ":
		timeInt, err := strconv.ParseUint(info, 10, 64)
		if err != nil {
			s.logger.Printf("[WARN] serf: Failed to convert event clock time: %v", err)
			return
		}
		s.lastActionClock = LamportTime(timeInt)
	case "query-clock: ":
		timeInt, err := strconv.ParseUint(info, 10, 64)
		if err != nil {
			s.logger.Printf("[WARN] serf: Failed to convert event clock time: %v", err)
		}
		s.lastQueryClock = LamportTime(timeInt)
	case "coordinate: ": // skip coordinate
	case "#": // skip comment
	case "leave":
		s.logger.Printf("[INFO] serf snapshotter: ignore previous leave")
	}
}

func (s *Snapshotter) replay() error {
	if _, err := s.fh.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReader(s.fh)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = line[:len(line)-1]
		s.handleLine(line)
	}
	_, err := s.fh.Seek(0, io.SeekEnd)
	return err
}

// returns the latest event time the snapshotter knows. should be named lastEventTime
func (s *Snapshotter) LastActionClock() LamportTime {
	return s.lastActionClock
}

// LastQueryClock returns the last known query clock time
func (s *Snapshotter) LastQueryClock() LamportTime {
	return s.lastQueryClock
}

// get aliveNodes of snapshotter! and shuffle it.
func (s *Snapshotter) AliveNodes() []*NodeIDAddr {
	// Copy the previously known
	previous := make([]*NodeIDAddr, 0, len(s.aliveNodes))
	for id, addr := range s.aliveNodes {
		previous = append(previous, &NodeIDAddr{id, addr})
	}

	// FISHER-YATES SHUFFLE
	for i := range previous {
		j := rand.Intn(i + 1)
		previous[i], previous[j] = previous[j], previous[i]
	}
	return previous
}

// Wait in shutdown!
func (s *Snapshotter) Wait() {
	<-s.stopCh
}

// send signal to leaveCh or wait for shutdownCh? very messy!
func (s *Snapshotter) Leave() {
	select {
	case s.leaveCh <- struct{}{}:
	case <-s.shutdownCh:
	}
}
