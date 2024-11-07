package serf

import (
	"fmt"
	"net"
)

type Event interface {
	EventType() EventType
	String() string
}

type EventType int

const (
	EventMemberJoin EventType = iota
	EventMemberLeave
	EventMemberFailed
	EventMemberUpdate
	EventMemberReap
	EventUser
	EventQuery
)

func (t EventType) String() string {
	switch t {
	case EventMemberJoin:
		return "member-join"
	case EventMemberLeave:
		return "member-leave"
	case EventMemberFailed:
		return "member-failed"
	case EventMemberUpdate:
		return "member-update"
	case EventMemberReap:
		return "member-reap"
	case EventUser:
		return "user"
	case EventQuery:
		return "query"
	}
	return "unknown- event"
}

type QueryEvent struct {
	Name       string
	LTime      LamportTime
	ID         uint32
	SourceIP   net.IP
	SourcePort uint16
	NodeID     string
	NumRelays  uint8
	Payload    []byte
}

func (q *QueryEvent) EventType() EventType {
	return EventQuery
}

func (q *QueryEvent) String() string {
	return fmt.Sprintf("query: %s", q.Name)
}
