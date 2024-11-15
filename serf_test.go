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

func createTestEventScript() (string, func(), error) {
	cleanup := func() {}
	tmp, err := os.CreateTemp("", "*script.sh")
	if err != nil {
		return "", cleanup, err
	}
	defer tmp.Close()
	cleanup = func() {
		os.Remove(tmp.Name())
	}
	if _, err := tmp.Write([]byte(`echo "Hello"`)); err != nil {
		return "", cleanup, err
	}
	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		fmt.Println("Error making temp file executable:", err)
		return "", cleanup, err
	}
	return tmp.Name(), cleanup, nil
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

func testNode(tag map[string]string) (*Serf, func(), error) {
	return testNodeWithIP(nil, nil)
}

func testNodeWithIP(tags map[string]string, ip net.IP) (*Serf, func(), error) {
	b := &SerfBuilder{}
	cleanup := func() {}

	key := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	keyRing, err := memberlist.NewKeyring(nil, key)
	if err != nil {
		return nil, cleanup, err
	}
	b.WithKeyring(keyRing)

	if ip == nil {
		ip, cleanup = testaddr.BindAddrs.NextAvailAddr()
	}
	mconf := testMemberlistConfig()
	mconf.BindAddr = ip.String()
	mconf.Label = "label"
	b.WithMemberlistConfig(mconf)

	prefix := fmt.Sprintf("serf-%s: ", mconf.BindAddr)
	logger := log.New(os.Stderr, prefix, log.LstdFlags)
	b.WithLogger(logger)

	snapfile := strconv.Itoa(rand.Int())
	conf := &Config{
		EventScript:            testEventScript,
		LBufferSize:            1024,
		QueryTimeoutMult:       16,
		QueryResponseSizeLimit: 1024,
		SnapshotPath:           filepath.Join(os.TempDir(), snapfile),
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

	b.WithTags(tags)

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

func TestMain(m *testing.M) {
	tmp, cleanup, err := createTestEventScript()
	defer cleanup()
	if err != nil {
		panic(err)
	}
	testEventScript = tmp
	m.Run()
}

func TestSerf_Create(t *testing.T) {
	_, cleanup, err := testNode(nil)
	defer cleanup()
	// wait a bit before shutdown.
	// if we shutdown too soon, shutdownCh will race with nodeEventCh
	time.Sleep(10 * time.Millisecond)
	require.Nil(t, err)
}

func twoNodesJoinedWithEventStream() (*Serf, *Serf, chan Event, func(), error) {
	stream := StreamEventHandler{
		eventCh: make(chan Event, 10),
	}
	s1, s2, cleanup, err := twoNodes()
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	s1.eventHandlers.stream.register(&stream)
	addr, err := s2.AdvertiseAddress()
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	n, err := s1.Join([]string{addr}, false)
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	if n != 1 {
		return nil, nil, nil, cleanup, fmt.Errorf("num success %d, expect %d", n, 1)
	}
	if s1.mlist.NumActive() != 2 {
		return nil, nil, nil, cleanup, fmt.Errorf("not enough active nodes")
	}
	return s1, s2, stream.eventCh, cleanup, nil
}

func TestSerf_Join(t *testing.T) {
	_, _, _, cleanup, err := twoNodesJoinedWithEventStream()
	defer cleanup()
	require.Nil(t, err)
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

func TestSerf_EventFailedNode(t *testing.T) {
	_, s2, eventCh, cleanup, err := twoNodesJoinedWithEventStream()
	defer cleanup()
	require.Nil(t, err)

	s2.Shutdown()

	success, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if len(eventCh) != 3 {
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

func TestSerf_EventJoin(t *testing.T) {
	_, s2, eventCh, cleanup, err := twoNodesJoinedWithEventStream()
	defer cleanup()
	require.Nil(t, err)

	s2.Shutdown()
	success, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if len(eventCh) != 1 {
			return false, "not enough events"
		}
		return true, ""
	})
	require.True(t, success, msg)
	success, msg = checkEventsForNode(s2.ID(), eventCh, []EventType{
		EventMemberJoin,
	})
	require.True(t, success, msg)
}

func TestSerf_EventLeave(t *testing.T) {
	_, s2, eventCh, cleanup, err := twoNodesJoinedWithEventStream()
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
	require.True(t, success, msg)
	success, msg = checkEventsForNode(s2.ID(), eventCh, []EventType{
		EventMemberJoin, EventMemberLeave, EventMemberReap,
	})
	require.True(t, success, msg)
}

func Test_SerfReconnect(t *testing.T) {
	s1, s2, eventCh, cleanup, err := twoNodesJoinedWithEventStream()
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

	s2, cleanup1, err := testNodeWithIP(nil, ip)
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

func retry(times int, fn func() (bool, string)) (success bool, msg string) {
	for i := 0; i < times; i++ {
		success, msg = fn()
		if success {
			return
		}
	}
	return
}
