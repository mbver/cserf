package serf

import (
	"sync"

	memberlist "github.com/mbver/mlist"
)

type inactiveNodes struct {
	l         sync.Mutex
	failed    []*memberlist.Node
	failedMap map[string]bool
	left      []*memberlist.Node
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
	i.failed = append(i.failed, n)
	i.failedMap[n.ID] = true
}

func (i *inactiveNodes) addLeft(n *memberlist.Node) {
	i.l.Lock()
	defer i.l.Unlock()
	i.left = append(i.left, n)
	i.leftMap[n.ID] = true
}

func removeNode(nodes *[]*memberlist.Node, id string) {
	if len(*nodes) == 0 {
		return
	}
	var i int
	for i = 0; i < len(*nodes); i++ {
		if (*nodes)[i].ID == id {
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
		removeNode(&i.failed, id)
	}
	if i.leftMap[id] { // this will not happen. left node will join with new id!
		delete(i.leftMap, id)
		removeNode(&i.left, id)
	}
}
