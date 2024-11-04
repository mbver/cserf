package serf

import memberlist "github.com/mbver/mlist"

type broadcastManager struct {
	serfBroadcasts   *memberlist.TransmitCapQueue
	actionBroadcasts *memberlist.TransmitCapQueue
	queryBroadcasts  *memberlist.TransmitCapQueue
}

func newBroadcastManager(numNodes func() int, transmitScale int) *broadcastManager {
	return &broadcastManager{
		serfBroadcasts:   memberlist.NewBroadcastQueue(numNodes, transmitScale),
		actionBroadcasts: memberlist.NewBroadcastQueue(numNodes, transmitScale),
		queryBroadcasts:  memberlist.NewBroadcastQueue(numNodes, transmitScale),
	}
}

func (m *broadcastManager) GetBroadcasts(overhead, limit int) [][]byte {
	if limit <= overhead {
		return nil
	}
	msgs := m.serfBroadcasts.GetMessages(overhead, limit)
	bytesUsed := 0
	for _, msg := range msgs {
		bytesUsed += (len(msg) + overhead)
	}

	queryMsgs := m.queryBroadcasts.GetMessages(overhead, limit-bytesUsed)
	for _, msg := range queryMsgs {
		bytesUsed += (len(msg) + overhead)
	}

	actionMsgs := m.queryBroadcasts.GetMessages(overhead, limit-bytesUsed)

	msgs = append(msgs, queryMsgs...)
	msgs = append(msgs, actionMsgs...)
	return msgs
}

func (m *broadcastManager) broadcastSerf(t msgType, msg interface{}, notify chan struct{}) {
	m.serfBroadcasts.QueueMsg("", t, msg, notify)
}

func (m *broadcastManager) broadcastQuery(t msgType, msg msgQuery, notify chan struct{}) {
	m.queryBroadcasts.QueueMsg("", t, msg, notify)
}

func (m *broadcastManager) broadcastAction(t msgType, msg msgAction, notify chan struct{}) {
	m.actionBroadcasts.QueueMsg("", t, msg, notify)
}
