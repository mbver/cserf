package serf

import (
	"fmt"
	"log"
	"os"
	"testing"

	memberlist "github.com/mbver/mlist"
	"github.com/mbver/mlist/testaddr"
	"github.com/stretchr/testify/require"
)

func noScheduleMemberlistConfig() *memberlist.Config {
	conf := memberlist.DefaultLANConfig()
	conf.ProbeInterval = 0
	conf.GossipInterval = 0
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

func noScheduleTestNode() (*Serf, func(), error) {
	b := &SerfBuilder{}
	cleanup := func() {}

	key := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	keyRing, err := memberlist.NewKeyring(nil, key)
	if err != nil {
		return nil, cleanup, err
	}
	b.WithKeyring(keyRing)

	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()
	mconf := noScheduleMemberlistConfig()
	mconf.BindAddr = ip.String()
	mconf.Label = "label"
	b.WithMemberlistConfig(mconf)

	conf := &Config{} // fill in later
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

func twoNodesNoSchedule() (*Serf, *Serf, func(), error) {
	s1, cleanup1, err := noScheduleTestNode()
	if err != nil {
		return nil, nil, cleanup1, err
	}
	s2, cleanup2, err := noScheduleTestNode()
	cleanup := combineCleanup(cleanup1, cleanup2)
	if err != nil {
		return nil, nil, cleanup, err
	}
	return s1, s2, cleanup, err
}

func TestSerf_Create(t *testing.T) {
	_, cleanup, err := noScheduleTestNode()
	defer cleanup()
	require.Nil(t, err)
}

func TestSerf_Join(t *testing.T) {
	s1, s2, cleanup, err := twoNodesNoSchedule()
	defer cleanup()
	require.Nil(t, err)

	ip, port, err := s2.mlist.GetAdvertiseAddr()
	require.Nil(t, err)

	addr := fmt.Sprintf("%s:%d", ip, port)
	n, err := s1.Join([]string{addr})
	require.Nil(t, err)
	require.Equal(t, 1, n)
}
