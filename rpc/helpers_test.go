package rpc

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	serf "github.com/mbver/cserf"
	memberlist "github.com/mbver/mlist"
	"github.com/mbver/mlist/testaddr"
)

func retry(times int, fn func() (bool, string)) (success bool, msg string) {
	for i := 0; i < times; i++ {
		success, msg = fn()
		if success {
			return
		}
	}
	return
}

func testMemberlistConfig() *memberlist.Config {
	conf := memberlist.DefaultLANConfig()
	conf.ProbeInterval = 0
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

func testNode() (*serf.Serf, func(), error) {
	b := &serf.SerfBuilder{}
	cleanup := func() {}

	key := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	keyRing, err := memberlist.NewKeyring(nil, key)
	if err != nil {
		return nil, cleanup, err
	}
	b.WithKeyring(keyRing)

	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()
	mconf := testMemberlistConfig()
	mconf.BindAddr = ip.String()
	mconf.Label = "label"
	b.WithMemberlistConfig(mconf)

	prefix := fmt.Sprintf("serf-%s: ", mconf.BindAddr)
	logger := log.New(os.Stderr, prefix, log.LstdFlags)
	b.WithLogger(logger)

	snapfile := strconv.Itoa(rand.Int())
	conf := &serf.Config{
		EventScript:            testEventScript,
		LBufferSize:            1024,
		QueryTimeoutMult:       16,
		QueryResponseSizeLimit: 1024,
		SnapshotPath:           filepath.Join(os.TempDir(), snapfile),
		SnapshotMinCompactSize: 128 * 1024,
		SnapshotDrainTimeout:   10 * time.Millisecond,
		CoalesceInterval:       5 * time.Millisecond,
	} // fill in later

	cleanup1 := combineCleanup(cleanup, func() {
		data, _ := os.ReadFile(conf.SnapshotPath)
		logger.Printf("### snapshot %s:", string(data))
		os.Remove(conf.SnapshotPath)
	})

	b.WithConfig(conf)

	s, err := b.Build()
	if err != nil {
		return nil, cleanup1, err
	}
	cleanup2 := combineCleanup(s.Shutdown, cleanup1)
	return s, cleanup2, nil
}

func twoNodes() (*serf.Serf, *serf.Serf, func(), error) {
	s1, cleanup1, err := testNode()
	if err != nil {
		return nil, nil, cleanup1, err
	}
	s2, cleanup2, err := testNode()
	cleanup := combineCleanup(cleanup1, cleanup2)
	if err != nil {
		return nil, nil, cleanup, err
	}
	return s1, s2, cleanup, err
}

func threeNodes() (*serf.Serf, *serf.Serf, *serf.Serf, func(), error) {
	s1, s2, cleanup1, err := twoNodes()
	if err != nil {
		return nil, nil, nil, cleanup1, err
	}
	s3, cleanup2, err := testNode()
	cleanup := combineCleanup(cleanup1, cleanup2)
	if err != nil {
		return nil, nil, nil, cleanup, err
	}
	return s1, s2, s3, cleanup, err
}

var rpcPort uint32 = 50050

func nextRpcPort() uint32 {
	return atomic.AddUint32(&rpcPort, 1)
}
