package serf

import (
	"fmt"
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
		clock:   &LamportClock{1},
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

func (m *ActionManager) getBuffer() []lGroupItem {
	m.l.Lock()
	defer m.l.Unlock()
	res := make([]lGroupItem, len(m.buffers))
	for i, group := range m.buffers {
		if group == nil {
			continue
		}
		res[i] = lGroupItem{
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

var ErrActionSizeLimitExceed = fmt.Errorf("action size limit exceeded")

// trigger a cluster action on all nodes
func (s *Serf) Action(name string, payload []byte) error {
	if len(payload) > s.config.ActionSizeLimit {
		return ErrActionSizeLimitExceed
	}
	msg := &msgAction{
		LTime:   s.action.clock.Time(), // witness will auto-increase it
		ID:      rand.Uint32(),
		Name:    name,
		Payload: payload,
	}
	encoded, err := encode(msgActionType, msg)
	if len(encoded) > s.config.ActionSizeLimit {
		return ErrActionSizeLimitExceed
	}
	if err != nil {
		return err
	}
	s.handleAction(encoded)
	return nil
}
