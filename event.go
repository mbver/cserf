package serf

import (
	"fmt"
	"net"
	"reflect"
	"time"

	memberlist "github.com/mbver/mlist"
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
	EventQuery
	EventAction
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
	case EventQuery:
		return "query"
	case EventAction:
		return "action"
	}
	return "unknown-event"
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
	Deadline   time.Time
}

func (q *QueryEvent) EventType() EventType {
	return EventQuery
}

func (q *QueryEvent) String() string {
	return fmt.Sprintf("query: %s", q.Name)
}

type ActionEvent struct {
	LTime   LamportTime
	Name    string
	Payload []byte
}

func (a *ActionEvent) EventType() EventType {
	return EventAction
}

func (a *ActionEvent) String() string {
	return fmt.Sprintf("action-event: %s", a.Name)
}

type MemberEvent struct {
	Type   EventType
	Member *memberlist.Node
}

func (m *MemberEvent) Equal(other *MemberEvent) bool {
	if other == nil {
		return false
	}
	if m.Type != other.Type {
		return false
	}
	if m.Member.ID != other.Member.ID {
		return false
	}
	if m.Type != EventMemberUpdate {
		return true
	}
	return reflect.DeepEqual(m.Member, other.Member)
}

func (m *MemberEvent) EventType() EventType {
	return m.Type
}

func (m *MemberEvent) String() string {
	return m.Type.String()
}

type CoalescedMemberEvent struct {
	Type    EventType
	Members []*memberlist.Node
}

func (m *CoalescedMemberEvent) EventType() EventType {
	return m.Type
}

func (m *CoalescedMemberEvent) String() string {
	return m.Type.String()
}
