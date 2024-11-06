package serf

import (
	"net"
	"regexp"
)

type msgType uint8

const (
	msgQueryType msgType = iota
	msgQueryRespType
	msgActionType
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
type msgAction struct {
	Name string
}

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
		var q msgQuery
		if err := decode(msg[1:], &q); err != nil {
			s.logger.Printf("[ERR] serf: Error decoding query msessage: %s", err)
			return
		}
		s.handleQuery(&q)

	case msgQueryRespType:
		var r msgQueryResponse
		if err := decode(msg[1:], &r); err != nil {
			s.logger.Printf("[ERR] serf: Error decoding query response messages: %s", err)
			return
		}
		s.handleQueryResponse(&r)
	}
}

func (s *Serf) handleQuery(q *msgQuery) {
	s.query.clock.Witness(q.LTime)

	if !s.query.addToBuffer(q) {
		return
	}

	s.broadcasts.broadcastQuery(msgQueryType, *q, nil)

	if !s.isQueryAccepted(q) {
		return
	}

	// TODO: this will be removed because it the QueryEvent will do the respond further down the pipeline
	resp := msgQueryResponse{
		LTime: q.LTime,
		ID:    q.ID,
		From:  s.mlist.ID(),
	}
	msg, err := encode(msgQueryRespType, resp)
	if err != nil {
		s.logger.Printf("[ERR] serf: encode query response message failed")
	}
	addr := net.UDPAddr{
		IP:   q.SourceIP,
		Port: int(q.SourcePort),
	}
	err = s.mlist.SendUserMsg(&addr, msg)
	if err != nil {
		s.logger.Printf("[ERR] serf: failed to send query response to %s", addr.String())
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

func (s *Serf) handleQueryResponse(r *msgQueryResponse) {
	s.query.invokeResponseHandler(r)
}
