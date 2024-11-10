package serf

import (
	"math/rand"
	"sync"
)

// cluster action msg
type msgAction struct {
	LTime   LamportTime
	ID      uint32
	Name    string
	Payload []byte
}

type ActionManager struct {
	l             sync.Mutex
	actionMinTime LamportTime // TODO: set it in snapshotting recovery
	clock         *LamportClock
	buffers       lBuffer
}

func newActionManager(bufferSize int) *ActionManager {
	return &ActionManager{
		clock:   &LamportClock{},
		buffers: make([]*lGroupItem, bufferSize),
	}
}

func (m *ActionManager) addToBuffer(msg *msgAction) (succcess bool) {
	m.l.Lock()
	defer m.l.Unlock()
	item := &lItem{msg.LTime, msg.ID}
	return m.buffers.addItem(m.clock.Time(), item)
}

// trigger a cluster action on all nodes
func (s *Serf) Action(name string, payload []byte) error {
	// check size??
	lTime := s.action.clock.Time()
	s.action.clock.Next()
	msg := &msgAction{
		LTime:   lTime,
		ID:      rand.Uint32(),
		Name:    name,
		Payload: payload,
	}
	encoded, err := encode(msgActionType, msg)
	if err != nil {
		return err
	}
	s.handleAction(encoded)
	return nil
}
