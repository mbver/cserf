package serf

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/stretchr/testify/require"
)

func TestCoalescer_Basic(t *testing.T) {
	enc0, err := EncodeTags(map[string]string{"role": "lb"})
	require.Nil(t, err)
	enc1, err := EncodeTags(map[string]string{"role": "db"})
	require.Nil(t, err)

	events := []MemberEvent{
		{EventMemberJoin, &memberlist.Node{ID: "first"}},
		{EventMemberLeave, &memberlist.Node{ID: "first"}},
		{EventMemberLeave, &memberlist.Node{ID: "second"}},
		{EventMemberUpdate, &memberlist.Node{ID: "third", Tags: enc0}},
		{EventMemberUpdate, &memberlist.Node{ID: "third", Tags: enc1}},
		{EventMemberReap, &memberlist.Node{ID: "fourth"}},
	}
	inCh := make(chan Event, 10)
	logger := log.New(os.Stderr, "coalescer: ", log.LstdFlags)
	shutdown := make(chan struct{})
	defer close(shutdown)
	outCh := NewMemberEventCoalescer(
		5*time.Millisecond,
		inCh,
		logger,
		shutdown,
	)
	cEvents := make(map[EventType]Event)
	timeoutCh := time.After(2 * time.Second)
	errCh := make(chan error, 1)
	go func() {
		for len(cEvents) < 3 {
			select {
			case e := <-outCh:
				cEvents[e.EventType()] = e
			case <-timeoutCh:
				errCh <- fmt.Errorf("timeout")
				return
			}
		}
		errCh <- nil
	}()

	for _, e := range events {
		inCh <- &e
	}
	err = <-errCh
	require.Nil(t, err)

	e, ok := cEvents[EventMemberLeave]
	require.True(t, ok)
	lEvent, ok := e.(*CoalescedMemberEvent)
	require.True(t, ok)
	require.Equal(t, 2, len(lEvent.Members))

	found := make(map[string]bool)
	for _, n := range lEvent.Members {
		found[n.ID] = true
	}
	for _, id := range []string{"first", "second"} {
		require.True(t, found[id])
	}

	e, ok = cEvents[EventMemberUpdate]
	require.True(t, ok)
	uEvent, ok := e.(*CoalescedMemberEvent)
	require.True(t, ok)
	require.Equal(t, 1, len(uEvent.Members))
	require.Equal(t, "third", uEvent.Members[0].ID)
	require.True(t, bytes.Equal(enc1, uEvent.Members[0].Tags))

	e, ok = cEvents[EventMemberReap]
	require.True(t, ok)
	rEvent, ok := e.(*CoalescedMemberEvent)
	require.True(t, ok)
	require.Equal(t, 1, len(rEvent.Members))
	require.Equal(t, "fourth", rEvent.Members[0].ID)
}

func TestCoalescer_TagUpdate(t *testing.T) {
	enc0, err := EncodeTags(map[string]string{"role": "lb"})
	require.Nil(t, err)
	enc1, err := EncodeTags(map[string]string{"role": "db"})
	require.Nil(t, err)

	inCh := make(chan Event, 10)
	logger := log.New(os.Stderr, "coalescer: ", log.LstdFlags)
	shutdown := make(chan struct{})
	defer close(shutdown)
	outCh := NewMemberEventCoalescer(
		5*time.Millisecond,
		inCh,
		logger,
		shutdown,
	)

	inCh <- &MemberEvent{
		Type:   EventMemberUpdate,
		Member: &memberlist.Node{ID: "first", Tags: enc0},
	}

	time.Sleep(10 * time.Millisecond)

	e, err := fetchEvent(outCh)
	require.Nil(t, err)
	uEvent, ok := e.(*CoalescedMemberEvent)
	require.True(t, ok)
	require.Equal(t, 1, len(uEvent.Members))
	require.Equal(t, "first", uEvent.Members[0].ID)
	require.True(t, bytes.Equal(uEvent.Members[0].Tags, enc0))

	inCh <- &MemberEvent{
		Type:   EventMemberUpdate,
		Member: &memberlist.Node{ID: "first", Tags: enc1},
	}

	time.Sleep(10 * time.Millisecond)

	e, err = fetchEvent(outCh)
	require.Nil(t, err)
	uEvent, ok = e.(*CoalescedMemberEvent)
	require.True(t, ok)
	require.Equal(t, 1, len(uEvent.Members))
	require.Equal(t, "first", uEvent.Members[0].ID)
	require.True(t, bytes.Equal(uEvent.Members[0].Tags, enc1))
}

func TestCoalescer_IsMemberEvent(t *testing.T) {
	cases := []struct {
		e      Event
		result bool
	}{
		{&ActionEvent{}, false},
		{&MemberEvent{Type: EventMemberJoin}, true},
		{&MemberEvent{Type: EventMemberLeave}, true},
		{&MemberEvent{Type: EventMemberFailed}, true},
		{&MemberEvent{Type: EventMemberUpdate}, true},
		{&MemberEvent{Type: EventMemberReap}, true},
	}

	for _, c := range cases {
		require.Equal(
			t,
			c.result,
			isMemberEvent(c.e.EventType()),
			fmt.Sprintf("%s, expect: %t", c.e.String(), c.result),
		)
	}
}
