package serf

import (
	"log"
	"sync"

	memberlist "github.com/mbver/mlist"
)

type messageUserState struct {
	ActionLTime  LamportTime
	ActionBuffer []lGroupItem
	QueryLTime   LamportTime
	LeftNodes    []*memberlist.Node
}

type userStateDelegate struct {
	queryClock          *LamportClock
	action              *ActionManager
	logger              *log.Logger
	joining             bool
	joinL               sync.Mutex
	ignoreActionsOnJoin bool
	handleAction        func([]byte)
	getLeftNodes        func() []*memberlist.Node
	mergeLeftNodes      func(...*memberlist.Node)
}

func newUserStateDelegate(
	queryClock *LamportClock,
	action *ActionManager,
	logger *log.Logger,
	handleAct func([]byte),
	getLeftNodes func() []*memberlist.Node,
	mergeLeftNodes func(...*memberlist.Node),
) *userStateDelegate {
	return &userStateDelegate{
		queryClock:     queryClock,
		action:         action,
		logger:         logger,
		handleAction:   handleAct,
		getLeftNodes:   getLeftNodes,
		mergeLeftNodes: mergeLeftNodes,
	}
}

func (u *userStateDelegate) isJoin() bool {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	return u.joining
}

func (u *userStateDelegate) setJoin(join bool) {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	u.joining = join
}

func (u *userStateDelegate) setIgnoreActionsOnJoin(ignore bool) {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	u.ignoreActionsOnJoin = ignore
}

func (u *userStateDelegate) LocalState() []byte {
	msg := messageUserState{
		ActionLTime:  u.action.clock.Time(),
		ActionBuffer: u.action.getBuffer(),
		QueryLTime:   u.queryClock.Time(),
		LeftNodes:    u.getLeftNodes(),
	}
	encoded, err := encode(msgUsrStateType, msg)
	if err != nil {
		u.logger.Printf("[ERR] serf: user state delegate: failed to encode local state")
		return nil
	}
	return encoded
}

func (u *userStateDelegate) Merge(buf []byte) {
	if len(buf) == 0 || buf[0] != byte(msgUsrStateType) {
		u.logger.Printf("[WARN] serf: user state delegate: not a user state message")
		return
	}
	var msg messageUserState
	err := decode(buf[1:], &msg)
	if err != nil {
		u.logger.Printf("[ERR] serf: user state delegate: failed to decode user state message %v", err)
		return
	}
	if msg.ActionLTime > 0 {
		u.action.clock.Witness(msg.ActionLTime - 1)
	}
	if msg.QueryLTime > 0 {
		u.queryClock.Witness(msg.QueryLTime - 1)
	}
	// ignore actions on join
	if u.isJoin() && u.ignoreActionsOnJoin {
		if msg.ActionLTime > u.action.getActionMinTime() {
			u.action.setActionMinTime(msg.ActionLTime)
		}
	}
	// replay actions
	var msgAct msgAction
	for _, group := range msg.ActionBuffer {
		msgAct.LTime = group.LTime
		for _, item := range group.Items {
			msgAct.ID = item.ID
			msgAct.Payload = item.Payload
			encoded, err := encode(msgActionType, msgAct)
			if err != nil {
				u.logger.Printf("[ERR] serf: user state delegate: failed to encode message action")
				return
			}
			u.handleAction(encoded)
		}
	}
	u.mergeLeftNodes(msg.LeftNodes...)
}
