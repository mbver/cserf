package serf

import (
	"fmt"

	memberlist "github.com/mbver/mlist"
)

func (s *Serf) handleNodeEvent(node *memberlist.NodeEvent) (*MemberEvent, error) {
	var eType EventType
	switch node.Type {
	case memberlist.NodeJoin:
		eType = s.handleNodeJoinEvent(node.Node)
	case memberlist.NodeUpdate:
		eType = s.handleNodeUpdateEvent(node.Node)
	case memberlist.NodeLeave:
		eType = s.handleNodeLeaveEvent(node.Node)
	default:
		return nil, fmt.Errorf("unknown member event")
	}
	return &MemberEvent{
		Type:   eType,
		Member: node.Node,
	}, nil
}

func (s *Serf) handleNodeJoinEvent(n *memberlist.Node) EventType {
	s.inactive.handleAlive(n.ID)
	return EventMemberJoin
}

func (s *Serf) handleNodeUpdateEvent(n *memberlist.Node) EventType {
	s.inactive.handleAlive(n.ID)
	return EventMemberUpdate
}

func (s *Serf) handleNodeLeaveEvent(n *memberlist.Node) EventType {
	node := s.mlist.GetNodeState(n.ID) // sure we have that node
	var eType EventType
	if node.State == memberlist.StateLeft {
		s.inactive.addLeft(n)
		eType = EventMemberLeave
	} else {
		s.inactive.addFail(n)
		eType = EventMemberFailed
	}
	s.logger.Printf("[INFO] serf: %s: %s %s",
		eType, n.ID, n.UDPAddress().String())
	return eType
}

// because inEventCh is buffered, so memberlist can start successfully
// even the whole event pipeline is not started
func (s *Serf) receiveNodeEvents() {
	for {
		select {
		case e := <-s.nodeEventCh:
			mEvent, err := s.handleNodeEvent(e) // TODO: handle events leave or failed or join alive before sending an event. we may have eventMemberfailed. also update members failed list for reap and reconnect
			if err != nil {
				s.logger.Printf("[ERR] serf: error receiving node event %v", err)
			}
			s.inEventCh <- mEvent
		case <-s.shutdownCh:
			s.logger.Printf("[WARN] serf: serf shutdown, quitting receiving node events")
			return
		}

	}
}
