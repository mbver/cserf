package serf

import (
	"fmt"

	memberlist "github.com/mbver/mlist"
)

func nodeToMemberEvent(node *memberlist.NodeEvent) (*MemberEvent, error) {
	var eType EventType
	switch node.Type {
	case memberlist.NodeJoin:
		eType = EventMemberJoin
	case memberlist.NodeLeave:
		eType = EventMemberLeave
	case memberlist.NodeUpdate:
		eType = EventMemberUpdate
	default:
		return nil, fmt.Errorf("unknown member event")
	}
	return &MemberEvent{
		Type: eType,
		Member: &Member{
			ID:   node.Node.ID,
			IP:   node.Node.IP,
			Port: node.Node.Port,
			Tags: node.Node.Tags,
		},
	}, nil
}

// because inEventCh is buffered, so memberlist can start successfully
// even the whole event pipeline is not started
func (s *Serf) receiveNodeEvents() {
	for {
		select {
		case e := <-s.nodeEventCh:
			mEvent, err := nodeToMemberEvent(e)
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
