package serf

import (
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"
)

type QueryParam struct {
	ForNodes   []string
	FilterTags []FilterTag
	Timeout    time.Duration
}

type FilterTag struct {
	Name string
	Expr string
}

type msgQuery struct {
	LTime      LamportTime
	ID         uint32
	SourceIP   net.IP
	SourcePort uint16
	ForNodes   []string    `codec:",omitempty"`
	FilterTags []FilterTag `codec:",omitempty"`
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
	LTime LamportTime
	ID    uint32
	From  string
}

type QueryResponseHandler struct {
	id     uint32
	respCh chan string
	// TODO: have a map to track duplicate responses (for relay, skip for now)
}

type QueryManager struct {
	l        sync.RWMutex
	clock    *LamportClock
	buffers  lBuffer
	handlers map[LamportTime]*QueryResponseHandler
	logger   *log.Logger
}

func newQueryManager(logger *log.Logger, bufferSize int) *QueryManager {
	return &QueryManager{
		clock:    &LamportClock{},
		buffers:  make([]*lGroupItem, bufferSize),
		handlers: make(map[LamportTime]*QueryResponseHandler),
		logger:   logger,
	}
}

func (m *QueryManager) setResponseHandler(lTime LamportTime, id uint32, ch chan string, timeout time.Duration) {
	time.AfterFunc(timeout, func() {
		m.l.Lock()
		close(m.handlers[lTime].respCh)
		delete(m.handlers, lTime)
		m.l.Unlock()
	})
	m.l.Lock()
	m.handlers[lTime] = &QueryResponseHandler{id, ch}
	m.l.Unlock()
}

func (m *QueryManager) invokeResponseHandler(r *msgQueryResponse) {
	m.l.RLock()
	defer m.l.RUnlock() // wait until sending done or it will panic for sending to closed channel
	h, ok := m.handlers[r.LTime]
	if !ok {
		return
	}
	if h.id != r.ID {
		return
	}
	select {
	case h.respCh <- r.From:
	case <-time.After(5 * time.Millisecond): // TODO: have a fixed value in config
		m.logger.Printf("[ERR] serf query: timeout streaming response from %s", r.From)
	}
}

func (m *QueryManager) addToBuffer(msg *msgQuery) (success bool) {
	m.l.Lock()
	defer m.l.Unlock()
	b := &bufQuery{msg.LTime, msg.ID}
	return m.buffers.addItem(m.clock.Time(), b)
}

func (s *Serf) Query(res chan string, params *QueryParam) (chan string, error) {
	if params == nil {
		params = s.DefaultQueryParams()
	}
	if params.Timeout == 0 {
		params.Timeout = s.DefaultQueryTimeout()
	}

	addr, port, err := s.mlist.GetAdvertiseAddr()
	if err != nil {
		return nil, err
	}
	lTime := s.query.clock.Time()
	s.query.clock.Next()
	q := msgQuery{
		LTime:      lTime,
		ID:         uint32(rand.Int31()),
		SourceIP:   addr,
		SourcePort: port,
		ForNodes:   params.ForNodes,
		FilterTags: params.FilterTags,
	}
	s.query.setResponseHandler(q.LTime, q.ID, res, params.Timeout) // TODO: have it as input or config value
	s.handleQuery(&q)
	return res, nil
}

func (s *Serf) DefaultQueryParams() *QueryParam {
	return &QueryParam{
		ForNodes:   nil,
		FilterTags: nil,
		Timeout:    s.DefaultQueryTimeout(),
	}
}

func (s *Serf) DefaultQueryTimeout() time.Duration {
	n := s.mlist.GetNumNodes()
	scale := math.Ceil(math.Log10(float64(n)+1)) * float64(s.config.QueryTimeoutMult)
	return time.Duration(scale) * s.mlist.GossipInterval()
}
