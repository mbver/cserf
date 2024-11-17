package serf

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/mbver/mlist/testaddr"
	"github.com/stretchr/testify/require"
)

var testEventScript string
var testSnapRecoverScript string
var testSnapRecoverOutput string

func createTestEventScript() (func(), error) {
	cleanup := func() {}
	tmp, err := os.CreateTemp("", "*script.sh")
	if err != nil {
		return cleanup, err
	}
	defer tmp.Close()
	cleanup = func() {
		os.Remove(tmp.Name())
	}
	if _, err := tmp.Write([]byte(`echo "Hello"`)); err != nil {
		return cleanup, err
	}
	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		fmt.Println("Error making temp file executable:", err)
		return cleanup, err
	}
	testEventScript = tmp.Name()
	return cleanup, nil
}

func createTestSnapshotRecoverScript() (func(), error) {
	cleanup := func() {}
	tmp, err := os.CreateTemp("", "*script.sh")
	if err != nil {
		return cleanup, err
	}

	outfile := strconv.Itoa(rand.Int())
	testSnapRecoverOutput = filepath.Join(os.TempDir(), outfile)
	fh, err := os.OpenFile(testSnapRecoverOutput, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return cleanup, err
	}
	defer tmp.Close()
	cleanup = func() {
		os.Remove(tmp.Name())
		os.Remove(testSnapRecoverOutput)
	}
	err = fh.Close()
	if err != nil {
		return cleanup, err
	}
	script := fmt.Sprintf(`echo "$SERF_EVENT" >> %s`, testSnapRecoverOutput)
	if _, err := tmp.Write([]byte(script)); err != nil {
		return cleanup, err
	}
	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		fmt.Println("Error making temp file executable:", err)
		return cleanup, err
	}
	testSnapRecoverScript = tmp.Name()
	return cleanup, nil
}

func TestMain(m *testing.M) {
	cleanup1, err := createTestEventScript()
	defer cleanup1()
	if err != nil {
		panic(err)
	}
	cleanup2, err := createTestSnapshotRecoverScript()
	defer cleanup2()
	if err != nil {
		panic(err)
	}
	m.Run()
}

func testMemberlistConfig() *memberlist.Config {
	conf := memberlist.DefaultLANConfig()
	conf.PingTimeout = 20 * time.Millisecond
	conf.ProbeInterval = 60 * time.Millisecond
	conf.ProbeInterval = 5 * time.Millisecond
	conf.GossipInterval = 5 * time.Millisecond
	conf.PushPullInterval = 0
	conf.ReapInterval = 0
	return conf
}

func combineCleanup(cleanups ...func()) func() {
	return func() {
		for _, f := range cleanups {
			f()
		}
	}
}

type testNodeOpts struct {
	tags   map[string]string
	ip     net.IP
	port   int
	snap   string
	script string
	ping   PingDelegate
}

func testNode(opts *testNodeOpts) (*Serf, func(), error) {
	if opts == nil {
		opts = &testNodeOpts{}
	}
	b := &SerfBuilder{}
	cleanup := func() {}

	key := []byte{79, 216, 231, 114, 9, 125, 153, 178, 238, 179, 230, 218, 77, 54, 187, 171, 185, 207, 73, 74, 215, 193, 176, 226, 217, 216, 91, 182, 168, 171, 223, 187}
	keyRing, err := memberlist.NewKeyring(nil, key)
	if err != nil {
		return nil, cleanup, err
	}
	b.WithKeyring(keyRing)

	ip := opts.ip
	if ip == nil {
		ip, cleanup = testaddr.BindAddrs.NextAvailAddr()
	}
	mconf := testMemberlistConfig()
	mconf.BindAddr = ip.String()
	port := opts.port
	if port != 0 {
		mconf.BindPort = port
	}
	mconf.Label = "label"
	b.WithMemberlistConfig(mconf)

	prefix := fmt.Sprintf("serf-%s: ", mconf.BindAddr)
	logger := log.New(os.Stderr, prefix, log.LstdFlags)
	b.WithLogger(logger)

	snapPath := opts.snap
	if snapPath == "" {
		snapfile := strconv.Itoa(rand.Int())
		snapPath = filepath.Join(os.TempDir(), snapfile)
	}
	script := opts.script
	if script == "" {
		script = testEventScript
	}
	conf := &Config{
		EventScript:            script,
		LBufferSize:            1024,
		QueryTimeoutMult:       16,
		QueryResponseSizeLimit: 1024,
		QuerySizeLimit:         1024,
		ActionSizeLimit:        512,
		SnapshotPath:           snapPath,
		SnapshotMinCompactSize: 128 * 1024,
		SnapshotDrainTimeout:   10 * time.Millisecond,
		CoalesceInterval:       5 * time.Millisecond,
		ReapInterval:           10 * time.Millisecond,
		// ReconnectInterval:      1 * time.Millisecond,
		MaxQueueDepth:    1024,
		ReconnectTimeout: 5 * time.Millisecond,
		TombstoneTimeout: 5 * time.Millisecond,
	} // fill in later

	cleanup1 := combineCleanup(cleanup, func() {
		data, _ := os.ReadFile(conf.SnapshotPath)
		logger.Printf("### snapshot %s:", string(data))
		os.Remove(conf.SnapshotPath)
	})

	b.WithConfig(conf)

	b.WithTags(opts.tags)

	b.WithPingDelegate(opts.ping)

	s, err := b.Build()
	if err != nil {
		return nil, cleanup1, err
	}
	cleanup2 := combineCleanup(s.Shutdown, cleanup1)
	return s, cleanup2, nil
}

func twoNodes() (*Serf, *Serf, func(), error) {
	s1, cleanup1, err := testNode(nil)
	if err != nil {
		return nil, nil, cleanup1, err
	}
	s2, cleanup2, err := testNode(nil)
	cleanup := combineCleanup(cleanup1, cleanup2)
	if err != nil {
		return nil, nil, cleanup, err
	}
	return s1, s2, cleanup, err
}

func threeNodes() (*Serf, *Serf, *Serf, func(), error) {
	s1, s2, cleanup1, err := twoNodes()
	if err != nil {
		return nil, nil, nil, cleanup1, err
	}
	s3, cleanup2, err := testNode(nil)
	cleanup := combineCleanup(cleanup1, cleanup2)
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	return s1, s2, s3, cleanup, err
}

func TestSerf_Create(t *testing.T) {
	_, cleanup, err := testNode(nil)
	defer cleanup()
	// wait a bit before shutdown.
	// if we shutdown too soon, shutdownCh will race with nodeEventCh
	time.Sleep(10 * time.Millisecond)
	require.Nil(t, err)
}

func twoNodesJoined() (*Serf, *Serf, func(), error) {
	return twoNodesJoinedWithEventStream(nil)
}

func twoNodesJoinedWithEventStream(eventCh chan Event) (*Serf, *Serf, func(), error) {
	s1, s2, cleanup, err := twoNodes()
	if err != nil {
		return nil, nil, cleanup, err
	}
	time.Sleep(10 * time.Millisecond) // wait until initial join events flushed out of the pipeline
	if eventCh != nil {
		stream := StreamEventHandler{
			eventCh: eventCh,
		}
		s1.eventHandlers.stream.register(&stream)
	}
	addr, err := s2.AdvertiseAddress()
	if err != nil {
		return nil, nil, cleanup, err
	}
	n, err := s1.Join([]string{addr}, false)
	if err != nil {
		return nil, nil, cleanup, err
	}
	if n != 1 {
		return nil, nil, cleanup, fmt.Errorf("num success %d, expect %d", n, 1)
	}
	if s1.mlist.NumActive() != 2 {
		return nil, nil, cleanup, fmt.Errorf("not enough active nodes")
	}
	return s1, s2, cleanup, nil
}

func TestSerf_Join(t *testing.T) {
	_, _, cleanup, err := twoNodesJoined()
	defer cleanup()
	require.Nil(t, err)
}

func TestSerf_NumNodes(t *testing.T) {
	s1, cleanup1, err := testNode(nil)
	defer cleanup1()
	require.Nil(t, err)
	require.Equal(t, 1, s1.NumNodes())
	require.Equal(t, 1, s1.mlist.NumActive())

	s2, cleanup2, err := testNode(nil)
	defer cleanup2()
	require.Nil(t, err)
	require.Equal(t, 1, s2.NumNodes())
	require.Equal(t, 1, s2.mlist.NumActive())

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)
	n, err := s1.Join([]string{addr}, false)
	require.Equal(t, 1, n)
	require.Nil(t, err)

	require.Equal(t, 2, s1.NumNodes())
	require.Equal(t, 2, s1.mlist.NumActive())
	require.Equal(t, 2, s2.NumNodes())
	require.Equal(t, 2, s2.mlist.NumActive())

}

func checkEventsForNode(id string, ch chan Event, expected []EventType) (bool, string) {
	received := make([]EventType, 0, len(expected))
	n := len(ch)
	for i := 0; i < n; i++ {
		e := <-ch
		mEvent, ok := e.(*CoalescedMemberEvent)
		if !ok {
			received = append(received, e.EventType())
			continue
		}
		found := false
		for _, m := range mEvent.Members {
			if m.ID == id {
				found = true
				break
			}
		}
		if found {
			received = append(received, mEvent.EventType())
		}
	}
	if reflect.DeepEqual(expected, received) {
		return true, ""
	}
	return false, fmt.Sprintf("event not match: expect: %v, got %v", expected, received)
}

func TestSerf_EventJoinShutdown(t *testing.T) {
	eventCh := make(chan Event, 10)
	_, s2, cleanup, err := twoNodesJoinedWithEventStream(eventCh)
	defer cleanup()
	require.Nil(t, err)

	s2.Shutdown()

	success, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if len(eventCh) != 3 { // if eventCh can record s1's join, it will be 4 events!
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, success, msg)
	success, msg = checkEventsForNode(s2.ID(), eventCh, []EventType{
		EventMemberJoin, EventMemberFailed, EventMemberReap,
	})
	require.True(t, success, msg)
}

// TODO: this test fails but very rarely. inspect it more intensively
func TestSerf_EventLeave(t *testing.T) {
	eventCh := make(chan Event, 10)
	_, s2, cleanup, err := twoNodesJoinedWithEventStream(eventCh)
	defer cleanup()
	require.Nil(t, err)

	s2.Leave()
	success, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if len(eventCh) < 3 {
			return false, "not enough events"
		}
		return true, ""
	})
	if len(eventCh) < 3 {
		n := len(eventCh)
		for i := 0; i < n; i++ {
			e := <-eventCh
			fmt.Println("============ failed", e.EventType())
		}
	}
	require.True(t, success, msg)
	success, msg = checkEventsForNode(s2.ID(), eventCh, []EventType{
		EventMemberJoin, EventMemberLeave, EventMemberReap,
	})
	require.True(t, success, msg)
}

func TestSerf_Reconnect(t *testing.T) {
	eventCh := make(chan Event, 10)
	s1, s2, cleanup, err := twoNodesJoinedWithEventStream(eventCh)
	defer cleanup()
	require.Nil(t, err)

	s1.inactive.l.Lock()
	s1.inactive.failedTimeout = 5 * time.Second
	s1.inactive.l.Unlock()
	go scheduleFunc(5*time.Millisecond, s1.shutdownCh, s1.reconnect)

	ip, _, err := s2.mlist.GetAdvertiseAddr()
	require.Nil(t, err)
	s2.Shutdown()
	enough, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if len(eventCh) < 2 {
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, enough, msg)
	match, msg := checkEventsForNode(s2.ID(), eventCh, []EventType{
		EventMemberJoin, EventMemberFailed,
	})
	require.True(t, match, msg)

	s2, cleanup1, err := testNode(&testNodeOpts{ip: ip})
	defer cleanup1()
	require.Nil(t, err)
	enough, msg = retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		if len(eventCh) < 1 {
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, enough, msg)
	match, msg = checkEventsForNode(s2.ID(), eventCh, []EventType{
		EventMemberJoin,
	})
	require.True(t, match, msg)
}

func TestSerf_Reconnect_SameIP(t *testing.T) {
	s1, cleanup1, err := testNode(nil)
	defer cleanup1()
	require.Nil(t, err)
	stream := &StreamEventHandler{
		eventCh: make(chan Event, 10),
	}
	s1.eventHandlers.stream.register(stream)

	s1.inactive.l.Lock()
	s1.inactive.failedTimeout = 5 * time.Second
	s1.inactive.l.Unlock()
	go scheduleFunc(5*time.Millisecond, s1.shutdownCh, s1.reconnect)

	ip, port, err := s1.mlist.GetAdvertiseAddr()
	require.Nil(t, err)

	s2, cleanup2, err := testNode(&testNodeOpts{ip: ip, port: int(port) + 1})
	defer cleanup2()
	require.Nil(t, err)

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)

	n, err := s1.Join([]string{addr}, false)
	require.Equal(t, 1, n)
	require.Nil(t, err)
	require.Equal(t, 2, s1.mlist.NumActive())

	s2.Shutdown()

	enough, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if len(stream.eventCh) < 2 {
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, enough, msg)
	match, msg := checkEventsForNode(s2.ID(), stream.eventCh, []EventType{
		EventMemberJoin, EventMemberFailed,
	})
	require.True(t, match, msg)

	s2, cleanup2, err = testNode(&testNodeOpts{ip: ip, port: int(port) + 1})
	defer cleanup2()
	require.Nil(t, err)

	enough, msg = retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		if len(stream.eventCh) < 1 {
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, enough, msg)
	match, msg = checkEventsForNode(s2.ID(), stream.eventCh, []EventType{
		EventMemberJoin,
	})
	require.True(t, match, msg)
}

func TestSerf_SetTags(t *testing.T) {
	eventCh := make(chan Event, 10)
	s1, s2, cleanup, err := twoNodesJoinedWithEventStream(eventCh)
	defer cleanup()
	require.Nil(t, err)
	s1.SetTags(map[string]string{"port": "8000"})
	changed, msg := retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		n1 := s2.mlist.GetNodeState(s1.ID())
		tags, err := decodeTags(n1.Node.Tags)
		require.Nil(t, err)
		if tags["port"] != "8000" {
			return false, "wrong tags: " + tags["port"]
		}
		return true, ""
	})
	require.True(t, changed, msg)
	match, msg := checkEventsForNode(s1.ID(), eventCh, []EventType{
		EventMemberUpdate,
	})
	require.True(t, match, msg)
	s2.SetTags(map[string]string{"datacenter": "east-aws"})
	changed, msg = retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		n2 := s1.mlist.GetNodeState(s2.ID())
		tags, err := decodeTags(n2.Node.Tags)
		require.Nil(t, err)
		if tags["datacenter"] != "east-aws" {
			return false, "wrong tags: " + tags["port"]
		}
		return true, ""
	})
	require.True(t, changed, msg)
	match, msg = checkEventsForNode(s2.ID(), eventCh, []EventType{
		EventMemberUpdate,
	})
	require.True(t, match, msg)
}

func TestSerf_JoinLeave(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoined()
	defer cleanup()
	require.Nil(t, err)

	err = s1.Leave()
	require.Nil(t, err)

	time.Sleep(2*s2.config.ReapInterval + s2.config.TombstoneTimeout)

	nLeft := s2.inactive.numLeft()
	require.Zero(t, nLeft)

	require.Equal(t, 1, s2.mlist.NumActive())
}

func TestSerf_JoinLeaveJoin(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoined()
	defer cleanup()
	require.Nil(t, err)

	s1.inactive.l.Lock()
	s1.inactive.leftTimeout = 0 // never delete
	s1.inactive.l.Unlock()

	err = s2.Leave()
	require.Nil(t, err)

	s2.Shutdown()
	ip, _, err := s2.mlist.GetAdvertiseAddr()
	require.Nil(t, err)

	s2Left, msg := retry(5, func() (bool, string) {
		time.Sleep(20 * time.Millisecond)
		if s1.mlist.NumActive() != 1 {
			return false, "num of active nodes not reduced"
		}
		left := s1.inactive.getLeftNodes()
		if len(left) != 1 {
			return false, "not having left node"
		}
		if left[0].ID != s2.ID() {
			return false, "node 2 not leaving: " + s2.ID()
		}
		return true, ""
	})
	require.True(t, s2Left, msg)

	s3, cleanup1, err := testNode(&testNodeOpts{ip: ip})
	defer cleanup1()
	require.Nil(t, err)

	addr, err := s1.AdvertiseAddress()
	require.Nil(t, err)
	s3.Join([]string{addr}, false)

	joined, msg := retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		if s1.mlist.NumActive() != 2 || s3.mlist.NumActive() != 2 {
			return false, "num of active nodes not correct"
		}
		node3 := s1.mlist.GetNodeState(s3.ID())
		if node3 == nil || node3.Node.ID != s3.ID() {
			return false, "node 3 not found"
		}
		node1 := s3.mlist.GetNodeState(s1.ID())
		if node1 == nil || node1.Node.ID != s1.ID() {
			return false, "node 1 not found"
		}
		return true, ""
	})
	require.True(t, joined, msg)
}

func TestSerf_LeaveJoinDifferentRole(t *testing.T) {
	s1, s2, cleanup, err := twoNodesJoined()
	defer cleanup()
	require.Nil(t, err)

	err = s2.Leave()
	require.Nil(t, err)

	s2.Shutdown()
	ip, _, err := s2.mlist.GetAdvertiseAddr()
	require.Nil(t, err)

	tags := map[string]string{"role": "bar"}
	s3, cleanup1, err := testNode(&testNodeOpts{tags: tags, ip: ip})
	defer cleanup1()
	require.Nil(t, err)

	addr, err := s1.AdvertiseAddress()
	require.Nil(t, err)
	s3.Join([]string{addr}, false)

	found, msg := retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		node := s1.mlist.GetNodeState(s3.ID())
		if node == nil {
			return false, "node not exist"
		}
		tags, err := decodeTags(node.Node.Tags)
		require.Nil(t, err)
		if tags["role"] != "bar" {
			return false, "wrong role"
		}
		return true, ""
	})
	require.True(t, found, msg)
}

func TestSerf_Role(t *testing.T) {
	s1, cleanup1, err := testNode(&testNodeOpts{
		tags: map[string]string{"role": "web"},
	})
	defer cleanup1()
	require.Nil(t, err)

	s2, cleanup2, err := testNode(&testNodeOpts{
		tags: map[string]string{"role": "lb"},
	})
	defer cleanup2()
	require.Nil(t, err)

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)

	n, err := s1.Join([]string{addr}, false)
	require.Nil(t, err)
	require.Equal(t, 1, n)

	found, msg := retry(5, func() (bool, string) {
		n1 := s2.mlist.GetNodeState(s1.ID())
		tags, err := decodeTags(n1.Node.Tags)
		require.Nil(t, err)
		if tags["role"] != "web" {
			return false, "role for node 1 wrong: " + tags["role"]
		}
		n2 := s1.mlist.GetNodeState(s2.ID())
		tags, err = decodeTags(n2.Node.Tags)
		require.Nil(t, err)
		if tags["role"] != "lb" {
			return false, "role for node 2 wrong: " + tags["role"]
		}
		return true, ""
	})
	require.True(t, found, msg)
}

func TestSerf_JoinIgnoreOld(t *testing.T) {
	s1, s2, cleanup, err := twoNodes()
	defer cleanup()
	require.Nil(t, err)

	time.Sleep(10 * time.Millisecond) // wait for flushing initial join-event
	eventCh := make(chan Event, 10)
	stream := StreamEventHandler{
		eventCh: eventCh,
	}
	s1.eventHandlers.stream.register(&stream)

	err = s2.Action("first", []byte("first-test"))
	require.Nil(t, err)

	err = s2.Action("second", []byte("second-test"))
	require.Nil(t, err)

	time.Sleep(10 * time.Millisecond)

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)

	n, err := s1.Join([]string{addr}, true)
	require.Equal(t, 1, n)
	require.Nil(t, err)

	enough, msg := retry(5, func() (bool, string) {
		time.Sleep(10 * time.Millisecond)
		if len(eventCh) < 1 {
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, enough, msg)
	match, msg := checkActions(eventCh, []string{}, [][]byte{})
	require.True(t, match, msg)
}

func TestSerf_State(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)
	require.Equal(t, SerfAlive, s.State())

	err = s.Leave()
	require.Nil(t, err)
	require.Equal(t, SerfLeft, s.State())

	s.Shutdown()
	require.Equal(t, SerfShutdown, s.State())
}

func TestSerf_StateString(t *testing.T) {
	states := []SerfStateType{SerfAlive, SerfLeft, SerfShutdown, SerfLeft, SerfShutdown, SerfAlive, SerfStateType(100)}
	expect := []string{"alive", "left", "shutdown", "left", "shutdown", "alive", "unknown-state"}
	for i, state := range states {
		require.Equal(t, expect[i], state.String())
	}
}

func TestSerf_ReapHandlerShutdown(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	errCh := make(chan error, 1)
	go func() {
		s.Shutdown()
		time.Sleep(time.Millisecond)
		errCh <- fmt.Errorf("timeout")
	}()
	go func() {
		scheduleFunc(5*time.Millisecond, s.shutdownCh, s.reap)
		errCh <- nil
	}()
	err = <-errCh
	require.Nil(t, err)
}

func TestSerf_Stats(t *testing.T) {
	s, cleanup, err := testNode(nil)
	defer cleanup()
	require.Nil(t, err)

	stats := s.Stats()
	exp := map[string]string{
		"active":            "1",
		"failed":            "0",
		"left":              "0",
		"health_score":      "0",
		"action_time":       "1",
		"query_time":        "1",
		"action_queued":     "0",
		"query_queued":      "0",
		"coordinate_resets": "0",
		"encrypted":         "true",
	}
	for k, v := range exp {
		require.Equal(t, v, stats[k], fmt.Sprintf("%s: expect: %s, got %s", k, v, stats[k]))
	}
}

func retry(times int, fn func() (bool, string)) (success bool, msg string) {
	for i := 0; i < times; i++ {
		success, msg = fn()
		if success {
			return
		}
	}
	return
}
