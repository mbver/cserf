// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import (
	"log"
	"sync"

	memberlist "github.com/mbver/mlist"
)

type messageUserState struct {
	From         string
	ActionLTime  LamportTime
	ActionBuffer []lGroupItem
	QueryLTime   LamportTime
	LeftNodes    []*memberlist.Node
}

type userStateDelegate struct {
	addr            string
	queryClock      *LamportClock
	action          *ActionManager
	logger          *log.Logger
	joining         map[string]bool
	ignoreActOnJoin map[string]bool
	joinL           sync.Mutex
	handleAction    func([]byte)
	getLeftNodes    func() []*memberlist.Node
	mergeLeftNodes  func(...*memberlist.Node)
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
		queryClock:      queryClock,
		action:          action,
		logger:          logger,
		joining:         make(map[string]bool),
		ignoreActOnJoin: make(map[string]bool),
		handleAction:    handleAct,
		getLeftNodes:    getLeftNodes,
		mergeLeftNodes:  mergeLeftNodes,
	}
}

func (u *userStateDelegate) isJoin(addr string) bool {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	return u.joining[addr]
}

func (u *userStateDelegate) isIgnoreActOnJoin(addr string) bool {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	return u.ignoreActOnJoin[addr]
}

func (u *userStateDelegate) setJoin(addr string) {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	u.joining[addr] = true
}

func (u *userStateDelegate) unsetJoin(addr string) {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	delete(u.joining, addr)
}

func (u *userStateDelegate) setIgnoreActOnJoin(addr string, ignore bool) {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	u.ignoreActOnJoin[addr] = ignore
}

func (u *userStateDelegate) unsetIgnoreActJoin(addr string) {
	u.joinL.Lock()
	defer u.joinL.Unlock()
	delete(u.ignoreActOnJoin, addr)
}

func (u *userStateDelegate) LocalState() []byte {
	msg := messageUserState{
		From:         u.addr,
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
	if u.isJoin(msg.From) && u.isIgnoreActOnJoin(msg.From) {
		if msg.ActionLTime > u.action.getActionMinTime() {
			u.action.setActionMinTime(msg.ActionLTime)
		}
	}
	u.unsetJoin(msg.From)
	u.unsetIgnoreActJoin(msg.From)
	// replay actions
	var msgAct msgAction
	for _, group := range msg.ActionBuffer {
		msgAct.LTime = group.LTime
		for _, item := range group.Items {
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
