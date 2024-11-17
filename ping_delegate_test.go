package serf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPing_Coordinates(t *testing.T) {
	s1, cleanup1, err := testNode(nil)
	defer cleanup1()
	require.Nil(t, err)

	s2, cleanup2, err := testNode(nil)
	defer cleanup2()
	require.Nil(t, err)

	zeroThreshold := 20.0e-6
	c1 := s1.ping.coord.GetCoordinate()
	c2 := s2.ping.coord.GetCoordinate()
	require.LessOrEqual(t, c1.DistanceTo(c2).Seconds(), zeroThreshold)

	addr, err := s2.AdvertiseAddress()
	require.Nil(t, err)
	n, err := s1.Join([]string{addr}, false)
	require.Equal(t, 1, n)
	require.Nil(t, err)

	updated, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if s1.ping.GetCachedCoord(s1.ID()) == nil {
			return false, "s2 is not cached in s1"
		}
		if s2.ping.GetCachedCoord(s1.ID()) == nil {
			return false, "s1 is not cached in s2"
		}
		c1 = s1.ping.coord.GetCoordinate()
		c2 = s2.ping.coord.GetCoordinate()
		if c1.DistanceTo(c2).Seconds() <= zeroThreshold {
			return false, "coordinates didn't update after probe"
		}
		if s1.ping.GetCachedCoord(s1.ID()) == nil {
			return false, "doesn't cache itself after update"
		}
		return true, ""
	})
	require.True(t, updated, msg)
}
