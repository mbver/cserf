// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import (
	memberlist "github.com/mbver/mlist"
)

type broadcastManager struct {
	numNodes         func() int
	maxQueueDepth    int
	minQueueDepth    int
	actionBroadcasts *memberlist.TransmitCapQueue
	queryBroadcasts  *memberlist.TransmitCapQueue
}

func newBroadcastManager(numNodes func() int, transmitScale int, maxQueueDepth int, minQueueDepth int) *broadcastManager {
	return &broadcastManager{
		numNodes:         numNodes,
		maxQueueDepth:    maxQueueDepth,
		minQueueDepth:    minQueueDepth,
		actionBroadcasts: memberlist.NewBroadcastQueue(numNodes, transmitScale),
		queryBroadcasts:  memberlist.NewBroadcastQueue(numNodes, transmitScale),
	}
}

func (m *broadcastManager) GetBroadcasts(overhead, limit int) [][]byte {
	if limit <= overhead {
		return nil
	}
	bytesUsed := 0

	queryMsgs := m.queryBroadcasts.GetMessages(overhead, limit-bytesUsed)
	for _, msg := range queryMsgs {
		bytesUsed += (len(msg) + overhead)
	}

	actionMsgs := m.actionBroadcasts.GetMessages(overhead, limit-bytesUsed)
	var msgs [][]byte
	msgs = append(msgs, queryMsgs...)
	msgs = append(msgs, actionMsgs...)
	return msgs
}

func (m *broadcastManager) broadcastQuery(t msgType, msg msgQuery, notify chan struct{}) {
	m.queryBroadcasts.QueueMsg("", t, msg, notify)
}

func (m *broadcastManager) broadcastAction(t msgType, msg msgAction, notify chan struct{}) {
	m.actionBroadcasts.QueueMsg("", t, msg, notify)
}

func (m *broadcastManager) maxQueueLen() int {
	max := m.maxQueueDepth
	if m.minQueueDepth > 0 {
		max = 2 * m.numNodes()
		if max < m.minQueueDepth {
			max = m.minQueueDepth
		}
	}
	return max
}

func (m *broadcastManager) manageQueueDepth() {
	max := m.maxQueueLen()
	if max == 0 {
		return
	}
	if m.actionBroadcasts.Len() > max {
		m.actionBroadcasts.Resize(max)
	}
	if m.queryBroadcasts.Len() > max {
		m.queryBroadcasts.Resize(max)
	}
}
