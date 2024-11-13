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
	item := &lItem{
		LTime:   msg.LTime,
		ID:      msg.ID,
		Payload: msg.Payload,
	}
	return m.buffers.addItem(m.clock.Time(), item)
}

func (m *ActionManager) getBuffer() []*lGroupItem {
	m.l.Lock()
	defer m.l.Unlock()
	res := make([]*lGroupItem, len(m.buffers))
	for i, group := range m.buffers {
		res[i] = &lGroupItem{
			LTime: group.LTime,
			Items: make([]*lItem, len(group.Items)),
		}
		for j, item := range group.Items {
			res[i].Items[j] = &lItem{
				LTime: item.LTime,
				ID:    item.ID,
			}
		}
	}
	return res
}

func (m *ActionManager) setActionMinTime(lTime LamportTime) {
	m.l.Lock()
	defer m.l.Unlock()
	m.actionMinTime = lTime
}

func (m *ActionManager) getActionMinTime() LamportTime {
	m.l.Lock()
	defer m.l.Unlock()
	return m.actionMinTime
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
