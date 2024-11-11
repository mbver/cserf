package serf

import (
	"net"
	"regexp"

	memberlist "github.com/mbver/mlist"
)

type msgType uint8

const (
	msgQueryType msgType = iota
	msgQueryRespType
	msgRelayType
	msgActionType
	msgKeyRespType
)

func (t msgType) Code() int {
	return int(t)
}
func (t msgType) String() string {
	switch t {
	case msgQueryType:
		return "query"
	case msgQueryRespType:
		return "query response"
	}
	return "unknownn message type"
}

// cluster action request

func (s *Serf) receiveMsgs() {
	for {
		select {
		case msg := <-s.userMsgCh:
			s.handleMsg(msg)
		case <-s.shutdownCh:
			return
		}
	}
}

func (s *Serf) handleMsg(msg []byte) {
	if len(msg) == 0 {
		s.logger.Printf("[WARN] serf: empty message")
	}

	t := msgType(msg[0])
	switch t {
	case msgQueryType:
		s.handleQuery(msg)
	case msgQueryRespType:
		s.handleQueryResponse(msg)
	case msgRelayType:
		s.handleRelay(msg)
	case msgActionType:
		s.handleAction(msg)
	}
}

func (s *Serf) handleQuery(msg []byte) {
	var q msgQuery
	if err := decode(msg[1:], &q); err != nil {
		s.logger.Printf("[ERR] serf: Error decoding query msessage: %s", err)
		return
	}
	s.query.clock.Witness(q.LTime)

	if q.LTime < s.query.queryMinTime {
		return
	}
	if !s.query.addToBuffer(&q) {
		return
	}
	s.broadcasts.broadcastQuery(msgQueryType, q, nil)

	if !s.isQueryAccepted(&q) {
		return
	}

	// TODO: this will be removed because it the QueryEvent will do the respond further down the pipeline
	s.inEventCh <- &QueryEvent{
		Name:       q.Name,
		LTime:      q.LTime,
		ID:         q.ID,
		SourceIP:   q.SourceIP,
		SourcePort: q.SourcePort,
		NodeID:     q.NodeID,
		NumRelays:  q.NumRelays,
		Payload:    q.Payload,
	}
}

func (s *Serf) isQueryAccepted(q *msgQuery) bool {
	if len(q.ForNodes) != 0 {
		for _, id := range q.ForNodes {
			if id == s.ID() {
				return true
			}
		}
		return false
	}
	if len(q.FilterTags) != 0 {
		for _, f := range q.FilterTags {
			matched, err := regexp.MatchString(f.Expr, s.tags[f.Name])
			if err != nil || !matched {
				return false
			}
		}
	}
	return true
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

func (s *Serf) handleQueryResponse(msg []byte) {
	var r msgQueryResponse
	if err := decode(msg[1:], &r); err != nil {
		s.logger.Printf("[ERR] serf: Error decoding query response messages: %s", err)
		return
	}
	s.query.invokeResponseHandler(&r)
}

func (s *Serf) handleRelay(msg []byte) {
	var r msgRelay
	if err := decode(msg[1:], &r); err != nil {
		s.logger.Printf("[ERR] serf: Error decoding relay response messages %s", err)
	}
	addr := &net.UDPAddr{
		IP:   r.DestIP,
		Port: int(r.DestPort),
	}
	if err := s.mlist.SendUserMsg(addr, r.Msg); err != nil {
		s.logger.Printf("[ERR] serf: failed to send user msg to %s", addr.String())
	}
}

func (s *Serf) handleAction(msg []byte) {
	var a msgAction
	if err := decode(msg[1:], &a); err != nil {
		s.logger.Printf(("[ERR] serf: Error decoding action message %s"), err)
	}
	s.action.clock.Witness(a.LTime)

	if a.LTime < s.action.actionMinTime {
		return
	}

	if !s.action.addToBuffer(&a) {
		return
	}
	s.broadcasts.broadcastAction(msgActionType, a, nil)

	s.inEventCh <- &ActionEvent{
		LTime:   a.LTime,
		Name:    a.Name,
		ID:      a.ID,
		Payload: a.Payload,
	}
}
