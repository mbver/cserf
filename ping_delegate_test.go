package serf

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/mbver/cserf/coordinate"
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
	c1 := s1.ping.GetCoordinate()
	c2 := s2.ping.GetCoordinate()
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
		c1 = s1.ping.GetCoordinate()
		c2 = s2.ping.GetCoordinate()
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

type rouguePingDelegate struct {
	*pingDelegate
}

func (p *rouguePingDelegate) Payload() []byte {
	c := coordinate.NewCoordinate(coordinate.DefaultConfig())
	c.Euclide = make([]float64, 2*len(c.Euclide))
	encoded, err := encode(msgCoordType, c)
	if err != nil {
		p.logger.Printf("[ERR] serf: fail to encode coordinate %v", err)
		return nil
	}
	return encoded
}

func TestPing_RougeCoordinates(t *testing.T) {
	logger := log.New(os.Stderr, "rouge-ping: ", log.LstdFlags)
	p, err := newPingDelegate(logger)
	require.Nil(t, err)
	rouge := rouguePingDelegate{p}

	s1, s2, cleanup, err := twoNodesJoined(
		&testNodeOpts{ping: &rouge},
		nil,
	)
	defer cleanup()
	require.Nil(t, err)

	updated, msg := retry(5, func() (bool, string) {
		time.Sleep(50 * time.Millisecond)
		if s1.GetCachedCoord(s2.ID()) == nil {
			return false, "s1 didn't get a coord for s2"
		}
		if s2.GetCachedCoord(s1.ID()) != nil {
			return false, "s2 should not see rouge node!"
		}
		return true, ""
	})
	require.True(t, updated, msg)
}
