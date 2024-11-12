package coordinate

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNode_NewNode(t *testing.T) {
	config := DefaultConfig()

	config.Dimensionality = 0
	_, err := NewNode(config)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "dimensionality")

	config.Dimensionality = 7
	node, err := NewNode(config)
	require.Nil(t, err)

	origin := NewCoordinate(config)
	require.True(t,
		reflect.DeepEqual(node.GetCoordinate(), origin),
		"fresh node should be located at origin")
}

func TestNode_Update(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3

	node, err := NewNode(config)
	require.Nil(t, err)

	// coord of new node is origin
	c := node.GetCoordinate()
	require.True(t, isVectorEqual(c.Euclide, []float64{0.0, 0.0, 0.0}))

	other := NewCoordinate(config)
	other.Euclide[2] = 0.001
	rtt := secondsToDuration(2.0 * other.Euclide[2])
	// rtt > dist, node is pushed away
	c, err = node.Relax("node", other, rtt)
	require.Nil(t, err)
	require.True(t,
		c.Euclide[2] < 0.0,
		fmt.Sprintf("node should be pushed down %9.6f", c.Euclide[2]))

	// test set-coordinate
	c.Euclide[2] = 99.0
	node.SetCoordinate(c)
	c = node.GetCoordinate()
	require.True(t, isFloatEqual(c.Euclide[2], 99.0))
}

func TestNode_InvalidRTTs(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3
	node, err := NewNode(config)
	require.Nil(t, err)

	other := NewCoordinate(config)
	other.Euclide[2] = 0.001

	dist := node.DistanceTo(other)

	// Update with a series of invalid ping periods, should return an error and estimated rtt remains unchanged
	rtts := []int{1<<63 - 1, -35, 11}

	for _, ping := range rtts {
		_, err = node.Relax("node", other, secondsToDuration(float64(ping)))
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "round trip time not in valid range")

		// unchanged coord if rtt is invalid
		new_dist := node.DistanceTo(other)
		require.Equal(t, dist, new_dist, fmt.Sprintf("expect:%s, got:%s", dist, new_dist))
	}

}

func TestNode_DistanceTo(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3
	config.HeightMin = 0
	node, err := NewNode(config)
	require.Nil(t, err)

	other := NewCoordinate(config)
	other.Euclide[2] = 12.345
	expected := secondsToDuration(other.Euclide[2])
	dist := node.DistanceTo(other)

	require.Equal(t, expected, dist, "expect:%s, got:%s", expected, dist)
}

// change vec to NaN, update and set coordinate should reject
// set different dimension, update and set coordinate should reject
func TestNode_NaN_Defense(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3
	node, err := NewNode(config)
	require.Nil(t, err)

	// an invalid coord
	other := NewCoordinate(config)
	other.Euclide[0] = math.NaN()
	require.False(t, isValidCoord(other))

	rtt := 250 * time.Millisecond

	// Relax reject invalid coord
	_, err = node.Relax("node", other, rtt)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "coordinate is invalid")
	require.True(t, isValidCoord(node.GetCoordinate()))

	// SetCoordinate rejects invalid coordinate
	err = node.SetCoordinate(other)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "coordinate is invalid")
	require.True(t, isValidCoord(node.GetCoordinate()))

	// Relax rejects mismatched dimensions
	other.Euclide = make([]float64, 2*len(other.Euclide))
	_, err = node.Relax("node", other, rtt)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "dimensions aren't compatible")
	require.True(t, isValidCoord(node.GetCoordinate()))

	// SetCoordinate rejects mismatched dimensions
	err = node.SetCoordinate(other)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "dimensions aren't compatible")
	require.True(t, isValidCoord(node.GetCoordinate()))

	// Relax resets internal state if it was invalid
	node.coord.Euclide[0] = math.NaN()
	other = NewCoordinate(config)
	c, err := node.Relax("node", other, rtt)
	require.Nil(t, err)
	require.True(t, isValidCoord(c))
	require.Equal(t, node.NumResets(), 1)
}
