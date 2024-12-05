package serf

import (
	"math/rand"
	"sync"
	"time"

	memberlist "github.com/mbver/mlist"
)

type InActiveNode struct {
	node *memberlist.Node
	time time.Time
}

type inactiveNodes struct {
	l             sync.Mutex
	failedTimeout time.Duration
	failed        []*InActiveNode
	failedMap     map[string]bool
	leftTimeout   time.Duration
	left          []*InActiveNode
	leftMap       map[string]bool
}

func newInactiveNodes(failedTimeout, leftTimeout time.Duration) *inactiveNodes {
	return &inactiveNodes{
		failedTimeout: failedTimeout,
		failedMap:     make(map[string]bool),
		leftTimeout:   leftTimeout,
		leftMap:       make(map[string]bool),
	}
}

func (i *inactiveNodes) addFail(n *memberlist.Node) {
	i.l.Lock()
	defer i.l.Unlock()
	f := &InActiveNode{
		node: n,
		time: time.Now(),
	}
	if i.failedMap[n.ID] {
		return
	}
	i.failed = append(i.failed, f)
	i.failedMap[n.ID] = true
}

func (i *inactiveNodes) removeFailed(id string) {
	i.l.Lock()
	defer i.l.Unlock()
	if !i.failedMap[id] {
		return
	}
	delete(i.failedMap, id)
	removeFromList(&i.failed, id)
}

func (i *inactiveNodes) numFailed() int {
	i.l.Lock()
	defer i.l.Unlock()
	return len(i.failed)
}

func (i *inactiveNodes) addLeft(n *memberlist.Node) {
	i.l.Lock()
	defer i.l.Unlock()
	if i.leftMap[n.ID] {
		return
	}
	l := &InActiveNode{
		node: n,
		time: time.Now(),
	}
	i.left = append(i.left, l)
	i.leftMap[n.ID] = true
}

func (i *inactiveNodes) addLeftBatch(nodes ...*memberlist.Node) {
	i.l.Lock()
	defer i.l.Unlock()
	for _, n := range nodes {
		if i.leftMap[n.ID] {
			return
		}
		l := &InActiveNode{
			node: n,
			time: time.Now(),
		}
		i.left = append(i.left, l)
		i.leftMap[n.ID] = true
	}
}

func (i *inactiveNodes) numLeft() int {
	i.l.Lock()
	defer i.l.Unlock()
	return len(i.left)
}

func (i *inactiveNodes) getLeftNodes() []*memberlist.Node {
	i.l.Lock()
	defer i.l.Unlock()
	res := make([]*memberlist.Node, len(i.left))
	for i, n := range i.left {
		res[i] = n.node
	}
	return res
}

func removeFromList(nodes *[]*InActiveNode, id string) {
	if len(*nodes) == 0 {
		return
	}
	var i int
	for i = 0; i < len(*nodes); i++ {
		if (*nodes)[i].node.ID == id {
			break
		}
	}
	if i == len(*nodes) { // not found
		return
	}
	// swap
	(*nodes)[i], (*nodes)[len(*nodes)-1] = (*nodes)[len(*nodes)-1], (*nodes)[i]
	// delete
	*nodes = (*nodes)[:len(*nodes)-1]
}

func (i *inactiveNodes) handleAlive(id string) {
	i.l.Lock()
	defer i.l.Unlock()
	if i.failedMap[id] {
		delete(i.failedMap, id)
		removeFromList(&i.failed, id)
	}
	if i.leftMap[id] { // this will not happen. left node will join with new id!
		delete(i.leftMap, id)
		removeFromList(&i.left, id)
	}
}

func removeTimeouts(nodes *[]*InActiveNode, m map[string]bool, timeout time.Duration) []*memberlist.Node {
	if timeout == 0 {
		return nil
	}
	toRemove := []*memberlist.Node{}
	for _, n := range *nodes {
		if time.Since(n.time) >= timeout {
			toRemove = append(toRemove, n.node)
		}
	}
	for _, n := range toRemove {
		removeFromList(nodes, n.ID)
		delete(m, n.ID)
	}
	return toRemove
}

func (i *inactiveNodes) reap() []*memberlist.Node {
	i.l.Lock()
	defer i.l.Unlock()
	fRemoved := removeTimeouts(&i.failed, i.failedMap, i.failedTimeout)
	lRemoved := removeTimeouts(&i.left, i.leftMap, i.leftTimeout)
	return append(fRemoved, lRemoved...)
}

func (i *inactiveNodes) shouldConnect(total int) bool {
	i.l.Lock()
	defer i.l.Unlock()
	numAlive := float32(total - len(i.failed) - len(i.left))
	if numAlive == 0 {
		numAlive = 1 // guard against zero divide
	}
	threshold := float32(len(i.failed)) / float32(numAlive)
	return rand.Float32() < threshold
}

func (i *inactiveNodes) pickRandomFailed() *memberlist.Node {
	i.l.Lock()
	defer i.l.Unlock()
	if len(i.failed) == 0 {
		return nil
	}
	idx := rand.Int31n(int32(len(i.failed)))
	return i.failed[idx].node.Clone()
}

func (s *Serf) reap() {
	removed := s.inactive.reap()
	for _, node := range removed {
		s.ping.RemoveCachedCoord(node.ID)
		s.inEventCh <- &MemberEvent{
			Type:   EventMemberReap,
			Member: node,
		}
	}

}

func (s *Serf) reconnect() {
	if s.inactive.numFailed() == 0 {
		return
	}
	if !s.inactive.shouldConnect(s.NumNodes()) {
		s.logger.Printf("[DEBUG] serf: forgoing reconnect for random throttling")
		return
	}
	node := s.inactive.pickRandomFailed()
	if node == nil {
		return
	}
	addr := node.UDPAddress().String()
	s.logger.Printf("[INFO] serf: attempting reconnect to %s %s", node.ID, addr)
	n, err := s.mlist.Join([]string{addr})
	if err != nil || n != 1 {
		return
	}
	s.inactive.removeFailed(node.ID) // successfully reconnect
}
