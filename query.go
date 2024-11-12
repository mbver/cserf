package serf

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	memberlist "github.com/mbver/mlist"
)

type QueryParam struct {
	Name       string
	ForNodes   []string
	FilterTags []FilterTag
	Timeout    time.Duration
	NumRelays  uint8
	Payload    []byte
}

type FilterTag struct {
	Name string
	Expr string
}

type msgQuery struct {
	Name       string
	LTime      LamportTime
	ID         uint32
	SourceIP   net.IP
	SourcePort uint16
	NodeID     string
	ForNodes   []string    `codec:",omitempty"`
	FilterTags []FilterTag `codec:",omitempty"`
	NumRelays  uint8
	Payload    []byte
}

type msgQueryResponse struct {
	LTime   LamportTime
	ID      uint32
	From    string
	Payload []byte
}

type QueryResponse struct {
	From    string
	Payload []byte
}

type msgRelay struct {
	Msg      []byte
	DestIP   net.IP
	DestPort uint16
}

type QueryResponseHandler struct {
	id       uint32
	respCh   chan *QueryResponse
	received map[string]struct{} // because handleMsg is sequential, no need to protect received with lock. furthermore, lock on manager is hold during accessing the handler!
}

type QueryManager struct {
	l            sync.RWMutex
	queryMinTime LamportTime // TODO: set it in snapshot and check it
	clock        *LamportClock
	buffers      lBuffer
	handlers     map[LamportTime]*QueryResponseHandler
	logger       *log.Logger
}

func newQueryManager(logger *log.Logger, bufferSize int) *QueryManager {
	return &QueryManager{
		clock:    &LamportClock{},
		buffers:  make([]*lGroupItem, bufferSize),
		handlers: make(map[LamportTime]*QueryResponseHandler),
		logger:   logger,
	}
}

func (m *QueryManager) setResponseHandler(lTime LamportTime, id uint32, ch chan *QueryResponse, timeout time.Duration) {
	time.AfterFunc(timeout, func() {
		m.l.Lock()
		close(m.handlers[lTime].respCh)
		delete(m.handlers, lTime)
		m.l.Unlock()
	})
	m.l.Lock()
	m.handlers[lTime] = &QueryResponseHandler{
		id:       id,
		respCh:   ch,
		received: make(map[string]struct{}),
	}
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
	if _, ok := h.received[r.From]; ok {
		return
	}
	h.received[r.From] = struct{}{}
	select {
	case h.respCh <- &QueryResponse{r.From, r.Payload}:
	case <-time.After(5 * time.Millisecond): // TODO: have a fixed value in config
		m.logger.Printf("[ERR] serf query: timeout streaming response from %s", r.From)
	}
}

func (m *QueryManager) addToBuffer(msg *msgQuery) (success bool) {
	m.l.Lock()
	defer m.l.Unlock()
	item := &lItem{msg.LTime, msg.ID}
	return m.buffers.addItem(m.clock.Time(), item)
}

func (s *Serf) Query(resCh chan *QueryResponse, params *QueryParam) error {
	if params == nil {
		params = s.DefaultQueryParams()
	}
	if params.Timeout == 0 {
		params.Timeout = s.DefaultQueryTimeout()
	}

	addr, port, err := s.mlist.GetAdvertiseAddr()
	if err != nil {
		return err
	}
	lTime := s.query.clock.Time()
	s.query.clock.Next()
	q := msgQuery{
		Name:       params.Name,
		LTime:      lTime,
		ID:         rand.Uint32(),
		SourceIP:   addr,
		SourcePort: port,
		NodeID:     s.ID(),
		ForNodes:   params.ForNodes,
		FilterTags: params.FilterTags,
		NumRelays:  params.NumRelays,
		Payload:    params.Payload,
	}
	s.query.setResponseHandler(q.LTime, q.ID, resCh, params.Timeout) // TODO: have it as input or config value
	// handle query locally
	msg, err := encode(msgQueryType, q)
	if err != nil {
		return err
	}
	s.handleQuery(msg)
	return nil
}

func (s *Serf) DefaultQueryParams() *QueryParam {
	return &QueryParam{
		Timeout: s.DefaultQueryTimeout(),
	}
}

func (s *Serf) DefaultQueryTimeout() time.Duration {
	n := s.mlist.GetNumNodes()
	scale := math.Ceil(math.Log10(float64(n)+1)) * float64(s.config.QueryTimeoutMult)
	return time.Duration(scale) * s.mlist.GossipInterval()
}

var ErrQueryRespLimitExceed = fmt.Errorf("query response exceed limit")

func (s *Serf) respondToQueryEvent(q *QueryEvent, output []byte) error {
	resp := msgQueryResponse{
		LTime:   q.LTime,
		ID:      q.ID,
		From:    s.mlist.ID(),
		Payload: output,
	}
	msg, err := encode(msgQueryRespType, resp)
	if err != nil {
		s.logger.Printf("[ERR] serf: encode query response message failed")
		return err
	}
	if len(msg) > s.config.QueryResponseSizeLimit {
		s.logger.Printf("[ERR] serf: query response size exceed limit: %d", len(msg))
		return ErrQueryRespLimitExceed
	}
	addr := net.UDPAddr{
		IP:   q.SourceIP,
		Port: int(q.SourcePort),
	}
	err = s.mlist.SendUserMsg(&addr, msg)
	if err != nil {
		s.logger.Printf("[ERR] serf: failed to send query response to %s", addr.String())
		return err
	}

	if err := s.relay(int(q.NumRelays), msg, q.SourceIP, q.SourcePort, q.NodeID); err != nil {
		s.logger.Printf("ERR serf: failed to relay query response to %s:%d", q.SourceIP, q.SourcePort)
		return err
	}
	return nil
}

func (s *Serf) relay(numRelay int, msg []byte, desIP net.IP, destPort uint16, destID string) error {
	if numRelay == 0 {
		return nil
	}
	if s.mlist.NumActive() < numRelay+2 { // too few nodes
		return nil
	}
	r := msgRelay{
		Msg:      msg,
		DestIP:   desIP,
		DestPort: destPort,
	}
	encoded, err := encode(msgRelayType, r)
	if err != nil {
		return err
	}
	nodes := s.pickRelayNodes(numRelay, destID)
	for _, n := range nodes {
		addr := &net.UDPAddr{
			IP:   n.IP,
			Port: int(n.Port),
		}
		s.mlist.SendUserMsg(addr, encoded)
	}
	return nil
}

func (s *Serf) pickRelayNodes(numNodes int, destID string) []*memberlist.Node {
	nodes := s.mlist.ActiveNodes()
	l := len(nodes)
	picked := make([]*memberlist.Node, 0, numNodes)
PICKNODE:
	for i := 0; i < 3*l && len(picked) < numNodes; i++ {
		idx := randIntN(l)
		node := nodes[idx]
		if node.ID == s.ID() || node.ID == destID {
			continue
		}
		for j := 0; j < len(picked); j++ {
			if node.ID == picked[j].ID {
				continue PICKNODE
			}
		}
		picked = append(picked, node)
	}
	return picked
}
