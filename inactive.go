package serf

import (
	"sync"
	"time"

	memberlist "github.com/mbver/mlist"
)

type InActiveNode struct {
	node *memberlist.Node
	time time.Time
}

type inactiveNodes struct {
	l         sync.Mutex
	failed    []*InActiveNode
	failedMap map[string]bool
	left      []*InActiveNode
	leftMap   map[string]bool
}

func newInactiveNodes() *inactiveNodes {
	return &inactiveNodes{
		failedMap: make(map[string]bool),
		leftMap:   make(map[string]bool),
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
