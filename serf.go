package serf

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	memberlist "github.com/mbver/mlist"
)

const tagMagicByte msgType = 255
const MaxActionSizeLimit = 9 * 1024

type SerfStateType int

const (
	SerfAlive SerfStateType = iota
	SerfLeft
	SerfShutdown
)

func (t SerfStateType) String() string {
	switch t {
	case SerfAlive:
		return "alive"
	case SerfLeft:
		return "left"
	case SerfShutdown:
		return "shutdown"
	}
	return "unknown-state"
}

type Serf struct {
	config         *Config
	keyring        *memberlist.Keyring
	inEventCh      chan Event
	outEventCh     chan Event
	nodeEventCh    chan *memberlist.NodeEvent
	keyQuery       *keyQueryReceptor
	invokeScriptCh chan *invokeScript
	eventHandlers  *eventHandlerManager
	mlist          *memberlist.Memberlist
	ping           *pingDelegate
	usrState       *userStateDelegate
	broadcasts     *broadcastManager
	query          *QueryManager
	action         *ActionManager
	userMsgCh      chan []byte
	logger         *log.Logger
	state          SerfStateType
	stateL         sync.Mutex
	inactive       *inactiveNodes
	snapshot       *Snapshotter
	shutdownCh     chan struct{}
	tags           map[string]string
}

type SerfBuilder struct {
	mconf   *memberlist.Config
	conf    *Config
	keyring *memberlist.Keyring
	logger  *log.Logger
	tags    map[string]string
}

func (b *SerfBuilder) WithMemberlistConfig(conf *memberlist.Config) {
	b.mconf = conf
}

func (b *SerfBuilder) WithConfig(conf *Config) {
	b.conf = conf
}

func (b *SerfBuilder) WithKeyring(k *memberlist.Keyring) {
	b.keyring = k
}

func (b *SerfBuilder) WithLogger(l *log.Logger) {
	b.logger = l
}

func (b *SerfBuilder) WithTags(tags map[string]string) {
	b.tags = tags
}

func (b *SerfBuilder) Build() (*Serf, error) {
	if b.conf.ActionSizeLimit > MaxActionSizeLimit {
		return nil, fmt.Errorf("action size limit exceeds %d", b.conf.ActionSizeLimit)
	}
	s := &Serf{}
	s.config = b.conf
	s.logger = b.logger
	s.shutdownCh = make(chan struct{})

	s.inEventCh = make(chan Event, 1024)
	snap, outCh, err := NewSnapshotter(s.config.SnapshotPath,
		s.config.SnapshotMinCompactSize,
		s.config.SnapshotDrainTimeout,
		s.logger,
		s.inEventCh,
		s.shutdownCh,
	)
	if err != nil {
		return nil, err
	}
	s.snapshot = snap
	prev := s.snapshot.AliveNodes()
	outCh = NewMemberEventCoalescer(s.config.CoalesceInterval,
		outCh,
		s.logger,
		s.shutdownCh,
	)

	s.keyQuery = &keyQueryReceptor{
		inCh:  outCh,
		outCh: make(chan Event, 1024),
	}

	s.outEventCh = s.keyQuery.outCh

	s.invokeScriptCh = make(chan *invokeScript)
	s.eventHandlers = newEventHandlerManager()
	scriptHandlers := CreateScriptHandlers(s.config.EventScript, s.invokeScriptCh)
	s.eventHandlers.script.update(scriptHandlers)

	mbuilder := &memberlist.MemberlistBuilder{}
	mbuilder.WithConfig(b.mconf)
	mbuilder.WithLogger(b.logger)

	mbuilder.WithKeyRing(b.keyring)
	s.keyring = b.keyring

	s.nodeEventCh = make(chan *memberlist.NodeEvent, 1024)
	mbuilder.WithEventCh(s.nodeEventCh)

	usrMsgCh := make(chan []byte)
	s.userMsgCh = usrMsgCh
	mbuilder.WithUserMessageCh(usrMsgCh)

	broadcasts := newBroadcastManager(s.NumNodes, b.mconf.RetransmitMult, b.conf.MaxQueueDepth) // TODO: add a logger then?
	s.broadcasts = broadcasts
	mbuilder.WithUserBroadcasts(broadcasts)

	ping, err := newPingDelegate(b.logger)
	if err != nil {
		return nil, err
	}
	s.ping = ping
	mbuilder.WithPingDelegate(ping)

	s.query = newQueryManager(b.logger, b.conf.LBufferSize)
	s.action = newActionManager(b.conf.LBufferSize)
	s.action.setActionMinTime(snap.LastActionClock() + 1)

	s.setState(SerfAlive)
	s.inactive = newInactiveNodes(s.config.ReconnectTimeout, s.config.TombstoneTimeout)

	s.usrState = newUserStateDelegate( // will not be used until memberlist join some nodes
		s.query.clock,
		s.action,
		s.logger,
		s.handleAction,
		s.inactive.getLeftNodes,
		s.inactive.addLeftBatch,
	)
	mbuilder.WithUserStateDelegate(s.usrState)

	s.tags = make(map[string]string)
	if len(b.tags) != 0 {
		encoded, err := encodeTags(b.tags)
		if len(encoded) > memberlist.TagMaxSize {
			return nil, memberlist.ErrMaxTagSizeExceed
		}
		if err != nil {
			return nil, err
		}
		b.mconf.Tags = encoded
		s.tags = b.tags
	}

	m, err := mbuilder.Build()
	if err != nil {
		return nil, err
	}
	s.mlist = m
	s.ping.id = m.ID()

	addr, err := s.AdvertiseAddress()
	if err != nil {
		return nil, err
	}
	s.usrState.addr = addr
	s.config.DNSConfigPath = b.mconf.DNSConfigPath

	s.schedule()
	go s.receiveNodeEvents()
	go s.receiveKeyEvents()
	go s.receiveEvents()
	go s.receiveInvokeScripts()
	go s.receiveMsgs()
	go s.rejoinSnapshot(prev)
	return s, nil
}

func resolveAddrs(addrs []string, dnsPath string) ([]string, []error) {
	var res []string
	var errs []error
	for _, addr := range addrs {
		ips, port, err := memberlist.ResolveAddr(addr, dnsPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, ip := range ips {
			res = append(res, net.JoinHostPort(ip.String(), strconv.Itoa(int(port))))
		}
	}
	return res, errs
}

func (s *Serf) Join(existing []string, ignoreOld bool) (int, error) {
	res, errs := resolveAddrs(existing, s.config.DNSConfigPath)
	if len(errs) != 0 {
		for _, err := range errs {
			s.logger.Printf("[ERR] serf: error resolving address %v", err)
		}
	}
	for _, addr := range res {
		s.usrState.setJoin(addr)
		s.usrState.setIgnoreActOnJoin(addr, ignoreOld)
	}
	defer func() { // unset in case join fails
		for _, addr := range existing {
			s.usrState.unsetJoin(addr)
			s.usrState.unsetIgnoreActJoin(addr)
		}
	}()
	return s.mlist.Join(existing)
}

// try to join 1 previously known-node
func (s *Serf) rejoinSnapshot(prev []*NodeIDAddr) {
	if len(prev) == 0 {
		return
	}
	myAddr, err := s.AdvertiseAddress()
	if err != nil {
		s.logger.Printf("[ERR] serf: error reading advertise address")
	}
	for _, n := range prev {
		if n.ID == s.ID() { // it will not happen as new node always has new id!
			continue
		}
		if n.Addr == myAddr { // don't connect to itself!
			continue
		}
		_, err := s.mlist.Join([]string{n.Addr})
		if err == nil {
			s.logger.Printf("[INFO] serf: rejoined successfully previously known node")
			return
		}
	}
	s.logger.Printf("[WARN] serf: failed to join any previously known node")
}

func (s *Serf) Leave() error {
	if s.hasLeft() {
		return fmt.Errorf("already left")
	}
	if s.hasShutdown() {
		return fmt.Errorf("leave after shutdown")
	}
	s.setState(SerfLeft)
	s.snapshot.Leave()
	return s.mlist.Leave()
}

func (s *Serf) Shutdown() {
	if s.hasShutdown() {
		return
	}
	if !s.hasLeft() {
		s.logger.Printf("[WARN] serf: Shutdown without a leave")
	}
	s.setState(SerfShutdown)

	s.mlist.Shutdown()
	close(s.shutdownCh)
	s.snapshot.Wait()
}

func (s *Serf) NumNodes() int {
	return s.mlist.GetNumNodes()
}

func (s *Serf) Members() []*memberlist.Node {
	return s.mlist.Members()
}

func (s *Serf) AdvertiseAddress() (string, error) {
	ip, port, err := s.mlist.GetAdvertiseAddr()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", ip.String(), port), err
}

func (s *Serf) ID() string {
	return s.mlist.ID()
}

func numToString(n int) string {
	return strconv.FormatUint(uint64(n), 10)
}
func (s *Serf) Stats() map[string]string {
	m := make(map[string]string)
	m["active"] = numToString(s.mlist.NumActive())
	m["failed"] = numToString(s.inactive.numFailed())
	m["left"] = numToString(s.inactive.numLeft())
	m["health_score"] = numToString(s.mlist.Health())
	m["action_time"] = numToString(int(s.action.clock.time))
	m["query_time"] = numToString(int(s.query.clock.time))
	m["action_queued"] = numToString(s.broadcasts.actionBroadcasts.Len())
	m["query_queued"] = numToString(s.broadcasts.queryBroadcasts.Len())
	m["coordinate_resets"] = numToString(s.ping.coord.NumResets())
	m["encrypted"] = fmt.Sprintf("%t", s.mlist.EncryptionEnabled())
	return m
}

func (s *Serf) setState(state SerfStateType) {
	s.stateL.Lock()
	defer s.stateL.Unlock()
	s.state = state
}

func (s *Serf) State() SerfStateType {
	s.stateL.Lock()
	defer s.stateL.Unlock()
	return s.state
}

func (s *Serf) hasLeft() bool {
	s.stateL.Lock()
	defer s.stateL.Unlock()
	return s.state == SerfLeft
}

func (s *Serf) hasShutdown() bool {
	s.stateL.Lock()
	defer s.stateL.Unlock()
	return s.state == SerfShutdown
}

func scheduleFunc(interval time.Duration, stopCh chan struct{}, f func()) {
	t := time.NewTicker(interval)
	jitter := time.Duration(uint64(rand.Int63()) % uint64(interval))
	time.Sleep(jitter) // wait random fraction of interval to avoid thundering herd
	for {
		select {
		case <-t.C:
			f()
		case <-stopCh:
			t.Stop()
			return
		}
	}
}

func (s *Serf) schedule() {
	if s.config.ReapInterval > 0 {
		go scheduleFunc(s.config.ReapInterval, s.shutdownCh, s.reap)
	}
	if s.config.ReconnectInterval > 0 {
		go scheduleFunc(s.config.ReconnectInterval, s.shutdownCh, s.reconnect)
	}
	if s.config.ManageQueueDepthInterval > 0 {
		go scheduleFunc(s.config.ManageQueueDepthInterval, s.shutdownCh, s.broadcasts.manageQueueDepth)
	}
}

func (s *Serf) SetTags(tags map[string]string) error {
	s.tags = tags
	encoded, err := encodeTags(s.tags)
	if len(encoded) > memberlist.TagMaxSize { // no need, memberlist can do this!
		return memberlist.ErrMaxTagSizeExceed
	}
	if err != nil {
		return err
	}
	return s.mlist.UpdateTags(encoded)
}
