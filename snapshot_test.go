// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/stretchr/testify/require"
)

func TestSerf_SnapshotRecovery(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoined(nil, nil)
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

func getShutdownCh() (chan struct{}, func()) {
	shutdown := make(chan struct{})
	getCleanup := func() func() {
		closed := false
		return func() {
			if !closed {
				closed = true
				close(shutdown)
			}
		}
	}
	return shutdown, getCleanup()
}

func testSnapshotter(name string, path string, drainTimeout time.Duration) (chan Event, chan Event, func(), *Snapshotter, func(), error) {
	inCh := make(chan Event, 100)
	snapPath := path
	if snapPath == "" {
		snapPath = tmpPath()
	}
	if drainTimeout == 0 {
		drainTimeout = 20 * time.Millisecond
	}
	cleanup := func() { os.Remove(snapPath) }
	logger := log.New(os.Stderr, name, log.LstdFlags)
	shutdown, closeShutdown := getShutdownCh()
	cleanup1 := combineCleanup(closeShutdown, cleanup)
	snap, outCh, err := NewSnapshotter(
		snapPath,
		1024,
		drainTimeout,
		logger,
		inCh,
		shutdown,
	)
	if err != nil {
		return nil, nil, nil, nil, cleanup1, err
	}
	cleanup2 := combineCleanup(closeShutdown, snap.Wait, cleanup)
	return inCh, outCh, closeShutdown, snap, cleanup2, err
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
	inCh, outCh, closeShutdown, snap, cleanup, err := testSnapshotter("test-snapshotter", "", 0)
	defer cleanup()
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

	closeShutdown()
	snap.Wait()

	_, _, _, snap, cleanup1, err := testSnapshotter("new-snapshot: ", snap.path, 0)
	defer cleanup1()
	require.Nil(t, err)

	require.Equal(t, LamportTime(42), snap.LastActionClock())
	require.Equal(t, LamportTime(50), snap.LastQueryClock())

	prev := snap.AliveNodes()
	require.Equal(t, 1, len(prev))
	require.Equal(t, "foo", prev[0].ID)
	require.Equal(t, "127.0.0.1:5000", prev[0].Addr)
}

func TestSnapshotter_ForceCompact(t *testing.T) {
	inCh, _, closeShutdown, snap, cleanup, err := testSnapshotter("snap-force-compact", "", 350*time.Millisecond)
	defer cleanup()
	require.Nil(t, err)

	for i := 0; i < 1024; i++ {
		inCh <- &ActionEvent{
			LTime: LamportTime(i),
		}
	}
	for i := 0; i < 1024; i++ {
		inCh <- &QueryEvent{
			LTime: LamportTime(i),
		}
	}
	closeShutdown()
	snap.Wait()
	_, _, _, snap, cleanup1, err := testSnapshotter("new-snap-force-compact", snap.path, 0)
	defer cleanup1()
	require.Nil(t, err)
	require.Equal(
		t, LamportTime(1023), snap.LastActionClock(),
		fmt.Sprintf("got: %d", snap.LastActionClock()))
	require.Equal(
		t, LamportTime(1023), snap.LastQueryClock(),
		fmt.Sprintf("got: %d", snap.LastQueryClock()))
}

func TestSnapshotter_LeaveRejoin(t *testing.T) {
	inCh, _, closeShutdown, snap, cleanup, err := testSnapshotter("snap-leave-rejoin", "", 0)
	defer cleanup()
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
	inCh <- &jEvent

	closeShutdown()
	snap.Wait()

	_, _, _, snap, cleanup1, err := testSnapshotter("new-snap-leave-join", snap.path, 0)
	defer cleanup1()
	require.Nil(t, err)

	require.Equal(t, LamportTime(42), snap.LastActionClock())
	require.Equal(t, LamportTime(50), snap.LastQueryClock())

	prev := snap.AliveNodes()
	require.Equal(t, 1, len(prev))
	require.Equal(t, "foo", prev[0].ID)
	require.Equal(t, "127.0.0.1:5000", prev[0].Addr)
}

func TestSnapshotter_SlowDiskNotBlockingOutEventCh(t *testing.T) {
	inCh, outCh, _, _, cleanup, err := testSnapshotter("snap-slow-disk", "", 0)
	defer cleanup()
	require.Nil(t, err)

	errCh := make(chan error, 1)
	numEvents := 10000
	numRecvd := 0
	timeoutCh := time.After(500 * time.Millisecond)
	go func() {
		for numRecvd < numEvents {
			select {
			case <-outCh:
				numRecvd++
			case <-timeoutCh:
				errCh <- fmt.Errorf("timeout")
				return
			}
		}
		errCh <- nil
	}()
	for i := 0; i < numEvents; i++ {
		e := &MemberEvent{
			Type: EventMemberJoin,
			Member: &memberlist.Node{
				ID:   fmt.Sprintf("foo%d", i),
				IP:   []byte{127, 0, byte(i / 256 % 256), byte(i % 256)},
				Port: 5000,
			},
		}
		if i%10 == 0 {
			e.Type = EventMemberLeave
		}
		inCh <- e
		time.Sleep(1 * time.Microsecond)
	}

	err = <-errCh
	require.Nil(t, err)
}

func slowDownStreamSnapshotter(inCh chan Event) (*Snapshotter, func(), error) {
	cleanup := func() {}
	path := tmpPath()
	fh, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, cleanup, fmt.Errorf("failed to open snapshot: %v", err)
	}
	cleanup = func() { os.Remove(path) }
	offset, err := fh.Seek(0, io.SeekEnd)
	if err != nil {
		fh.Close()
		return nil, cleanup, fmt.Errorf("failed to stat snapshot: %v", err)
	}
	logger := log.New(os.Stderr, "snap-slow-downstream", log.LstdFlags)
	shutdownCh := make(chan struct{})
	snap := &Snapshotter{
		aliveNodes:      make(map[string]string),
		path:            path,
		fh:              fh,
		buffer:          bufio.NewWriter(fh),
		offset:          offset,
		teeCh:           make(chan Event, eventChSize),
		lastActionClock: 0,
		lastQueryClock:  0,
		leaveCh:         make(chan struct{}),
		logger:          logger,
		minCompactSize:  1024,
		drainTimeout:    250 * time.Millisecond,
		stopCh:          make(chan struct{}),
		shutdownCh:      shutdownCh,
	}
	outCh := make(chan Event) // unbuffered outCh simulates a slow downstream
	go snap.teeEvents(inCh, outCh)
	go snap.receiveEvents()
	return snap, cleanup, nil
}

func TestSnapshotter_SlowMemberlistNotBlockingSnapshotter(t *testing.T) {
	inCh := make(chan Event, 1024)
	snap, cleanup, err := slowDownStreamSnapshotter(inCh)
	defer cleanup()
	require.Nil(t, err)

	numEvents := 2048
	for i := 0; i < numEvents; i++ {
		e := &ActionEvent{
			LTime: LamportTime(i),
		}
		inCh <- e
		time.Sleep(1 * time.Microsecond)
	}

	close(snap.shutdownCh)
	snap.Wait()

	_, _, _, snap, cleanup1, err := testSnapshotter("snap-slow-new", snap.path, 0)
	defer cleanup1()
	require.Nil(t, err)
	require.Equal(t, LamportTime(2047), snap.LastActionClock())
}
