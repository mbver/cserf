package serf

import (
	"fmt"
	"net"
)

type msgType uint8

const (
	msgQueryType msgType = iota
	msgQueryRespType
)

type msgQuery struct {
	ID         string
	SourceIP   net.IP
	SourcePort uint16
}

type msgQueryResponse struct {
	ID string
}

// cluster action request
type msgAction struct {
	Name string
}

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

var broadcasted map[string]bool

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
		fmt.Println("got msg query", q.ID)
		if !broadcasted[q.ID] {
			s.broadcasts.broadcastQuery(msgQueryType, q, nil)
			broadcasted[q.ID] = true
		}
	case msgQueryRespType:
		var r msgQueryResponse
		if err := decode(msg[1:], &r); err != nil {
			s.logger.Printf("[ERR] serf: Error decoding query response messages: %s", err)
			return
		}
		fmt.Println("got msg query response")
	}
}
