package serf

import (
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

type msgQuery struct {
	ID         string
	SourceIP   net.IP
	SourcePort uint16
}

type msgQueryResponse struct {
	ID   string
	From string
}

type QueryResponseHandler struct {
	respCh chan string
	// have a map to track duplicate responses (for relay, skip for now)
}

type QueryManager struct {
	l         sync.Mutex
	handlers  map[string]*QueryResponseHandler
	logger    log.Logger
	processed map[string]bool // for not rebroadcast already handle query msg. later will use buffer
}

func (m *QueryManager) setResponseHandler(id string, ch chan string, timeout time.Duration) {
	time.AfterFunc(timeout, func() {
		m.l.Lock()
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
	case <-time.After(2 * time.Second): // have a fixed value in config
		m.logger.Printf("[ERR] serf query: timeout streaming response from %s", r.From)
	}
}

func (s *Serf) Query(res chan string) error {
	addr, port, err := s.mlist.GetAdvertiseAddr()
	if err != nil {
		return err
	}
	q := msgQuery{
		ID:         strconv.Itoa(int(rand.Int31())),
		SourceIP:   addr,
		SourcePort: port,
	}
	s.query.setResponseHandler(q.ID, res, 3*time.Second)
	return nil
}
