package serf

import (
	"bytes"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/stretchr/testify/require"
)

func TestUserState_LocalState(t *testing.T) {
	eventCh := make(chan Event, 10)
	s1, cleanup1, err := testNode(&testNodeOpts{
		eventCh:   eventCh,
		tombStone: time.Hour, // don't reap
	})
	defer cleanup1()
	require.Nil(t, err)

	s2, cleanup2, err := testNode(nil)
	defer cleanup2()
	require.Nil(t, err)

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)
	n, err := s1.Join([]string{addr}, false)
	require.Equal(t, 1, n)
	require.Nil(t, err)

	s1.Action("deploy", []byte("something"))

	respCh := make(chan *QueryResponse, 2)
	s1.Query(respCh, nil)

	s2.Leave()
	// wait until s2 left
	left, msg := retry(5, func() (bool, string) {
		time.Sleep(20 * time.Millisecond)
		n := len(eventCh)
		for i := 0; i < n; i++ {
			e := <-eventCh
			cEvent, ok := e.(*CoalescedMemberEvent)
			if !ok {
				continue
			}
			if cEvent.EventType() != EventMemberLeave {
				continue
			}
			return true, ""
		}
		return false, "not left"
	})
	require.True(t, left, msg)

	encoded := s1.usrState.LocalState()
	require.Equal(t, byte(msgUsrStateType), encoded[0])

	var usrMsg messageUserState

	err = decode(encoded[1:], &usrMsg)
	require.Nil(t, err)

	require.Equal(t, usrMsg.ActionLTime, s1.action.clock.Time())
	require.Equal(t, usrMsg.QueryLTime, s1.query.clock.Time())
	require.Equal(t, 1, len(usrMsg.LeftNodes))
	require.Equal(t, s2.ID(), usrMsg.LeftNodes[0].ID)
	require.Equal(t, s1.config.LBufferSize, len(usrMsg.ActionBuffer))

	item := lItem{
		LTime:   s1.action.clock.Time() - 1,
		Payload: []byte("something"),
	}
	group := usrMsg.ActionBuffer[int(item.LTime)]
	require.NotNil(t, group)
	require.True(t, group.has(&item))
}

func TestUserState_Merge(t *testing.T) {
	aBuf := lBuffer(make([]*lGroupItem, 1024))
	item := &lItem{
		LTime:   45,
		Payload: []byte("something"),
	}
	aBuf.addItem(40, item)
	sentAbuf := make([]lGroupItem, 1024)
	for i, g := range aBuf {
		if g != nil {
			sentAbuf[i] = *g
		}
	}
	usrMsg := messageUserState{
		ActionLTime:  50,
		QueryLTime:   4,
		ActionBuffer: sentAbuf,
		LeftNodes: []*memberlist.Node{
			{ID: "node"},
		},
	}
	encoded, err := encode(msgUsrStateType, usrMsg)
	require.Equal(t, byte(msgUsrStateType), encoded[0])
	require.Nil(t, err)

	eventCh := make(chan Event, 10)
	s, cleanup, err := testNode(&testNodeOpts{eventCh: eventCh})
	defer cleanup()
	require.Nil(t, err)

	s.usrState.Merge(encoded)
	require.Equal(t, usrMsg.ActionLTime, s.action.clock.Time())
	require.Equal(t, usrMsg.QueryLTime, s.query.clock.Time())
	require.Equal(t, 1, len(s.inactive.getLeftNodes()))
	require.Equal(t, "node", s.inactive.getLeftNodes()[0].ID)

	gotAction, errMsg := retry(5, func() (bool, string) {
		time.Sleep(20 * time.Millisecond)
		n := len(eventCh)
		for i := 0; i < n; i++ {
			e := <-eventCh
			aEvent, ok := e.(*ActionEvent)
			if !ok {
				continue
			}
			if aEvent.LTime != item.LTime {
				return false, "not matching lTime"
			}
			if !bytes.Equal(aEvent.Payload, item.Payload) {
				return false, "not matching payload"
			}
			return true, ""
		}
		return false, "action event not found"
	})
	require.True(t, gotAction, errMsg)
}

func TestUserState_Merge_IgnoreOld(t *testing.T) {
	eventCh := make(chan Event, 10)
	s1, cleanup1, err := testNode(&testNodeOpts{eventCh: eventCh})
	defer cleanup1()
	require.Nil(t, err)

	s2, cleanup2, err := testNode(nil)
	defer cleanup2()
	require.Nil(t, err)
	s2.Action("first", []byte("small"))
	s2.Action("second", []byte("medium"))
	s2.Action("third", []byte("large"))

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)

	n, err := s1.Join([]string{addr}, true)
	require.Nil(t, err)
	require.Equal(t, 1, n)

	require.Equal(t, s2.action.clock.Time(), s1.action.getActionMinTime())

	noAction, errMsg := retry(5, func() (bool, string) {
		time.Sleep(20 * time.Millisecond)
		n := len(eventCh)
		for i := 0; i < n; i++ {
			e := <-eventCh
			e, ok := e.(*ActionEvent)
			if ok {
				return false, "got action"
			}
		}
		return true, ""
	})
	require.True(t, noAction, errMsg)
}
