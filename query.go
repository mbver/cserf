package serf

import (
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type msgQuery struct {
	LTime      LamportTime
	ID         uint32
	SourceIP   net.IP
	SourcePort uint16
}

type bufQuery struct {
	ltime LamportTime
	id    uint32
}

func (b *bufQuery) LTime() LamportTime {
	return b.ltime
}

func (b *bufQuery) Equal(item lItem) bool {
	b1 := item.(*bufQuery)
	return b1.id == b.id
}

type msgQueryResponse struct {
	ID   uint32
	From string
}

type QueryResponseHandler struct {
	respCh chan string
	// TODO: have a map to track duplicate responses (for relay, skip for now)
}

type QueryManager struct {
	l        sync.Mutex
	clock    *LamportClock
	buffers  lBuffer
	handlers map[uint32]*QueryResponseHandler
	logger   *log.Logger
}

func newQueryManager(logger *log.Logger, bufferSize int) *QueryManager {
	return &QueryManager{
		clock:    &LamportClock{},
		buffers:  make([]*lGroupItem, bufferSize),
		handlers: make(map[uint32]*QueryResponseHandler),
		logger:   logger,
	}
}

func (m *QueryManager) setResponseHandler(id uint32, ch chan string, timeout time.Duration) {
	time.AfterFunc(timeout, func() {
		m.l.Lock()
		close(m.handlers[id].respCh)
		delete(m.handlers, id)
		m.l.Unlock()
	})
	m.l.Lock()
	m.handlers[id] = &QueryResponseHandler{ch}
	m.l.Unlock()
}

func (m *QueryManager) invokeResponseHandler(r *msgQueryResponse) {
	m.l.Lock()
	h, ok := m.handlers[r.ID]
	m.l.Unlock()
	if !ok {
		return
	}
	select {
	case h.respCh <- r.From:
	case <-time.After(2 * time.Second): // TODO: have a fixed value in config
		m.logger.Printf("[ERR] serf query: timeout streaming response from %s", r.From)
	}
}

// lock here or lock outside?
func (m *QueryManager) addToBuffer(msg *msgQuery) (success bool) {
	m.l.Lock()
	defer m.l.Unlock()
	b := &bufQuery{msg.LTime, msg.ID}
	if m.buffers.isTooOld(m.clock.Time(), b) {
		return false
	}
	if m.buffers.isLTimeNew(b.LTime()) {
		m.buffers.addNewLTime(b)
		return true
	}
	idx := b.LTime() % m.buffers.len()
	group := m.buffers[idx]
	if group.has(b) {
		return false
	}
	group.add(b)
	return true
}

func (s *Serf) Query(res chan string) error {
	addr, port, err := s.mlist.GetAdvertiseAddr()
	if err != nil {
		return err
	}
	lTime := s.query.clock.Time()
	s.query.clock.Next()
	q := msgQuery{
		LTime:      lTime,
		ID:         uint32(rand.Int31()),
		SourceIP:   addr,
		SourcePort: port,
	}
	s.handleQuery(&q)
	s.query.setResponseHandler(q.ID, res, 2*time.Second) // TODO: have it as input or config value
	return nil
}
