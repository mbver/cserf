package serf

import (
	"log"

	memberlist "github.com/mbver/mlist"
)

type Serf struct {
	mlist      memberlist.Memberlist
	broadcasts *broadcastManager
	query      *QueryManager
	userMsgCh  chan []byte
	logger     *log.Logger
	shutdownCh chan struct{}
}

func NewSerf() *Serf {
	// bcasts := memberlist.NewBroadcastQueue(
	// 	func() int { return 10 },
	// 	2,
	// )
	// mbuilder := memberlist.MemberlistBuilder{}
	return nil
}

func (s *Serf) Shutdown() {
	close(s.shutdownCh)
}
