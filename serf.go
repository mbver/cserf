package serf

import (
	"fmt"
	"log"

	memberlist "github.com/mbver/mlist"
)

type Serf struct {
	mlist      memberlist.Memberlist
	broadcasts *broadcastManager
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

func (s *Serf) receiveUserMsg() {
	for {
		select {
		case msg := <-s.userMsgCh:
			fmt.Println(string(msg))
		case <-s.shutdownCh:
			return
		}
	}
}

// unique id, channel, gather responses, stream to channel
// set timeout? receiver must drain the channel continuously
func (s *Serf) Query() {

}
func (s *Serf) Shutdown() {
	close(s.shutdownCh)
}
