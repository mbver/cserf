package serf

import (
	"os"
	"testing"
	"time"

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
