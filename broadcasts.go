package serf

import (
	memberlist "github.com/mbver/mlist"
)

type broadcastManager struct {
	maxQueueDepth    int
	actionBroadcasts *memberlist.TransmitCapQueue
	queryBroadcasts  *memberlist.TransmitCapQueue
}

func newBroadcastManager(numNodes func() int, transmitScale int, maxQueueDepth int) *broadcastManager {
	return &broadcastManager{
		maxQueueDepth:    maxQueueDepth,
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

func (m *broadcastManager) manageQueueDepth() {
	if m.maxQueueDepth == 0 {
		return
	}
	m.actionBroadcasts.Resize(m.maxQueueDepth)
	m.queryBroadcasts.Resize(m.maxQueueDepth)
}
