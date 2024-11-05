package serf

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	memberlist "github.com/mbver/mlist"
	"github.com/mbver/mlist/testaddr"
	"github.com/stretchr/testify/require"
)

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

func testNode() (*Serf, func(), error) {
	b := &SerfBuilder{}
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

	conf := &Config{
		QueryBufferSize: 1024,
	} // fill in later
	b.WithConfig(conf)

	prefix := fmt.Sprintf("serf-%s: ", mconf.BindAddr)
	logger := log.New(os.Stderr, prefix, log.LstdFlags)
	b.WithLogger(logger)

	s, err := b.Build()
	if err != nil {
		return nil, cleanup, err
	}
	cleanup1 := combineCleanup(s.Shutdown, cleanup)
	return s, cleanup1, nil
}

func twoNodes() (*Serf, *Serf, func(), error) {
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

func threeNodes() (*Serf, *Serf, *Serf, func(), error) {
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

func TestSerf_Create(t *testing.T) {
	_, cleanup, err := testNode()
	defer cleanup()
	require.Nil(t, err)
}

func TestSerf_Join(t *testing.T) {
	s1, s2, cleanup, err := twoNodes()
	defer cleanup()
	require.Nil(t, err)

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)

	n, err := s1.Join([]string{addr})
	require.Nil(t, err)
	require.Equal(t, 1, n)
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
