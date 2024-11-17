package serf

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/stretchr/testify/require"
)

func TestSerf_SnapshotRecovery(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoined()
	defer cleanup()
	require.Nil(t, err)
	err = s1.Action("first", []byte("first-test"))
	require.Nil(t, err)

	err = s1.Action("second", []byte("second-test"))
	require.Nil(t, err)

	err = s1.Action("third", []byte("third-test"))
	require.Nil(t, err)

	time.Sleep(20 * time.Millisecond)
	s2.Shutdown()

	failed, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if s1.mlist.NumActive() != 1 {
			return false, "not see shutdown node failed"
		}
		return true, ""
	})
	require.True(t, failed, msg)

	ip, _, err := s2.mlist.GetAdvertiseAddr()
	require.Nil(t, err)

	s3, cleanup2, err := testNode(&testNodeOpts{
		ip:     ip,
		snap:   s2.config.SnapshotPath,
		script: testSnapRecoverScript,
	})
	defer cleanup2()
	require.Nil(t, err)

	joined, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if s3.mlist.NumActive() != 2 {
			return false, "not joining previous node"
		}
		return true, ""
	})
	require.True(t, joined, msg)
	require.Equal(t, LamportTime(4), s3.action.getActionMinTime())
	// all actions via pushpull with s1 will be rejected
	output, err := os.ReadFile(testSnapRecoverOutput)
	require.Nil(t, err)
	require.NotContains(t, string(output), "action")
}

func fetchEvent(ch chan Event) (Event, error) {
	select {
	case e := <-ch:
		return e, nil
	case <-time.After(20 * time.Millisecond):
		return nil, fmt.Errorf("timeout fetching event")
	}
}

func TestSnapshotter(t *testing.T) {
	inCh := make(chan Event, 100)
	snapPath := tmpPath()
	defer os.Remove(snapPath)
	logger := log.New(os.Stderr, "snapshotter-test: ", log.LstdFlags)
	shutdown := make(chan struct{}) // WHERE TO CLEANUP WITH SHUTDOWN?
	closed := false
	defer func() {
		if !closed {
			close(shutdown)
		}
	}()
	snap, outCh, err := NewSnapshotter(
		snapPath,
		1024,
		20*time.Millisecond,
		logger,
		inCh,
		shutdown,
	)
	require.Nil(t, err)

	aEvent := ActionEvent{
		LTime: 42,
		Name:  "bar",
	}
	inCh <- &aEvent

	qEvent := QueryEvent{
		LTime: 50,
		Name:  "uptime",
	}
	inCh <- &qEvent

	jEvent := MemberEvent{
		Type: EventMemberJoin,
		Member: &memberlist.Node{
			ID:   "foo",
			IP:   []byte{127, 0, 0, 1},
			Port: 5000,
		},
	}
	fEvent := MemberEvent{
		Type: EventMemberFailed,
		Member: &memberlist.Node{
			ID:   "foo",
			IP:   []byte{127, 0, 0, 1},
			Port: 5000,
		},
	}
	inCh <- &jEvent
	inCh <- &fEvent
	inCh <- &jEvent

	for _, e := range []Event{&aEvent, &qEvent, &jEvent, &fEvent, &jEvent} {
		event, err := fetchEvent(outCh)
		require.Nil(t, err)
		require.True(t,
			reflect.DeepEqual(event, e),
			fmt.Sprintf("unmatched: expect: %+v, got: %+v", e, event))
	}

	closed = true
	close(shutdown)
	snap.Wait()

	shutdown = make(chan struct{})
	closed = false

	snap, _, err = NewSnapshotter(
		snapPath,
		1024,
		20*time.Millisecond,
		logger,
		inCh,
		shutdown,
	)
	require.Nil(t, err)

	require.Equal(t, LamportTime(42), snap.LastActionClock())
	require.Equal(t, LamportTime(50), snap.LastQueryClock())

	prev := snap.AliveNodes()
	require.Equal(t, 1, len(prev))
	require.Equal(t, "foo", prev[0].ID)
	require.Equal(t, "127.0.0.1:5000", prev[0].Addr)
}
