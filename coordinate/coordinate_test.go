// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package coordinate

import (
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoordinate_NewCoordinate(t *testing.T) {
	config := DefaultConfig()
	c := NewCoordinate(config)
	if len(c.Euclide) != config.Dimensionality {
		t.Fatalf("dimensionality not set correctly %d != %d",
			len(c.Euclide), config.Dimensionality)
	}
}

func TestCoordinate_Clone(t *testing.T) {
	c := NewCoordinate(DefaultConfig())
	c.Euclide[0], c.Euclide[1], c.Euclide[2] = 1.0, 2.0, 3.0
	c.Error = 5.0
	c.Adjustment = 10.0
	c.Height = 4.2

	other := c.Clone()
	if !reflect.DeepEqual(c, other) {
		t.Fatalf("coordinate clone didn't make a proper copy")
	}

	other.Euclide[0] = c.Euclide[0] + 0.5
	if reflect.DeepEqual(c, other) {
		t.Fatalf("cloned coordinate is still pointing at its ancestor")
	}
}

// test every field with NaN, 0.0, Inf
func TestCoordinate_IsValid(t *testing.T) {
	c := NewCoordinate(DefaultConfig())

	var fields []*float64
	for i := range c.Euclide {
		fields = append(fields, &c.Euclide[i])
	}
	fields = append(fields, &c.Error)
	fields = append(fields, &c.Adjustment)
	fields = append(fields, &c.Height)

	for _, field := range fields {
		require.True(t, isValidCoord(c))

		*field = math.NaN()
		require.False(t, isValidCoord(c))

		*field = math.Inf(0)
		require.False(t, isValidCoord(c))

		*field = 0.0 // reset field
	}
}

func TestCoordinate_DimMatch(t *testing.T) {
	config := DefaultConfig()

	config.Dimensionality = 3
	c1 := NewCoordinate(config)
	c2 := NewCoordinate(config)

	config.Dimensionality = 2
	alien := NewCoordinate(config)

	require.True(t, matchDim(c1, c1) && matchDim(c2, c2))
	require.True(t, matchDim(c1, c2) && matchDim(c2, c1))
	require.False(t, matchDim(c1, alien) || matchDim(c2, alien) ||
		matchDim(alien, c1) || matchDim(alien, c2))
}

func TestCoordinate_ApplyForce(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3
	config.HeightMin = 0

	origin := NewCoordinate(config)
	above := NewCoordinate(config)
	above.Euclide = []float64{0.0, 0.0, 2.9}

	// push-away from above, means going down.
	c := ApplyForce(above, origin, config.HeightMin, 5.3)
	require.True(t, isVectorEqual(c.Euclide, []float64{0.0, 0.0, -5.3}))

	right := NewCoordinate(config)
	right.Euclide = []float64{3.4, 0.0, -5.3}

	// push-away from right node, means going left
	c = ApplyForce(right, c, config.HeightMin, 2.0)
	require.True(t, isVectorEqual(c.Euclide, []float64{-2.0, 0.0, -5.3}))

	// push-away from itself. moves in random direction
	c = ApplyForce(origin, origin, config.HeightMin, 1.0)
	require.True(t, isFloatEqual(c.DistanceTo(origin).Seconds(), 1.0))

	config.HeightMin = 10.0e-6
	origin = NewCoordinate(config)

	//push-away from the origin, going down
	c = ApplyForce(above, origin, config.HeightMin, 5.3)
	require.True(t, isVectorEqual(c.Euclide, []float64{0.0, 0.0, -5.3}))
	require.True(t, isFloatEqual(c.Height,
		config.HeightMin+5.3*config.HeightMin/2.9))

	// pull-up the point at origin up
	c = ApplyForce(above, origin, config.HeightMin, -5.3)
	require.True(t, isVectorEqual(c.Euclide, []float64{0.0, 0.0, 5.3}))
	require.True(t, isFloatEqual(c.Height, config.HeightMin)) // negative force reduce height

	bad := c.Clone()
	bad.Euclide = make([]float64, len(bad.Euclide)+1)
	ch := make(chan bool, 1)
	catchDimensionPanic(func() { ApplyForce(bad, c, config.HeightMin, 1.0) }, ch)
	require.True(t, <-ch)
}

func TestCoordinate_TimeDistance(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3
	config.HeightMin = 0

	c1, c2 := NewCoordinate(config), NewCoordinate(config)
	c1.Euclide = []float64{-0.5, 1.3, 2.4}
	c2.Euclide = []float64{1.2, -2.3, 3.4}

	require.True(t, isFloatEqual(c1.DistanceTo(c1).Seconds(), 0.0))
	require.True(t, isFloatEqual(c1.DistanceTo(c2).Seconds(), c2.DistanceTo(c1).Seconds()))
	require.True(t, isFloatEqual(c1.DistanceTo(c2).Seconds(), 4.104875150354758))

	// Make sure negative adjustment factors are ignored.
	c1.Adjustment = -1.0e6
	require.True(t, isFloatEqual(c1.DistanceTo(c2).Seconds(), 4.104875150354758))

	// Make sure positive adjustment factors affect the distance.
	c1.Adjustment = 0.1
	c2.Adjustment = 0.2
	require.True(t, isFloatEqual(c1.DistanceTo(c2).Seconds(), 4.104875150354758+0.3))

	// Make sure the heights affect the distance.
	c1.Height = 0.7
	c2.Height = 0.1
	require.True(t, isFloatEqual(c1.DistanceTo(c2).Seconds(), 4.104875150354758+0.3+0.8))

	bad := c1.Clone()
	bad.Euclide = make([]float64, len(bad.Euclide)+1)
	ch := make(chan bool, 1)
	catchDimensionPanic(func() { c1.DistanceTo(bad) }, ch)
	require.True(t, <-ch)
}

func TestCoordinate_rawDistance(t *testing.T) {
	config := DefaultConfig()
	config.Dimensionality = 3
	config.HeightMin = 0

	c1, c2 := NewCoordinate(config), NewCoordinate(config)
	c1.Euclide = []float64{-0.5, 1.3, 2.4}
	c2.Euclide = []float64{1.2, -2.3, 3.4}

	require.True(t, isFloatEqual(distance(c1, c1), 0.0))
	require.True(t, isFloatEqual(distance(c1, c2), distance(c2, c1)))
	require.True(t, isFloatEqual(distance(c1, c2), 4.104875150354758))

	// rawDistance doesn't care about adjustment
	c1.Adjustment = 1.0e6
	require.True(t, isFloatEqual(distance(c1, c2), 4.104875150354758))

	// Make sure the heights affect the distance.
	c1.Height = 0.7
	c2.Height = 0.1
	require.True(t, isFloatEqual(distance(c1, c2), 4.104875150354758+0.8))
}

func TestCoordinate_addVec(t *testing.T) {
	vec1 := []float64{1.0, -3.0, 3.0}
	vec2 := []float64{-4.0, 5.0, 6.0}
	require.True(t, isVectorEqual(addVec(vec1, vec2), []float64{-3.0, 2.0, 9.0}))

	zero := []float64{0.0, 0.0, 0.0}
	require.True(t, isVectorEqual(addVec(vec1, zero), vec1))
}

func TestCoordinate_diffVec(t *testing.T) {
	vec1 := []float64{1.0, -3.0, 3.0}
	vec2 := []float64{-4.0, 5.0, 6.0}
	require.True(t, isVectorEqual(diffVec(vec1, vec2), []float64{5.0, -8.0, -3.0}))

	zero := []float64{0.0, 0.0, 0.0}
	require.True(t, isVectorEqual(diffVec(vec1, zero), vec1))
}

func TestCoordinate_magnitude(t *testing.T) {
	zero := []float64{0.0, 0.0, 0.0}
	require.True(t, isFloatEqual(magnitude(zero), 0.0))

	vec := []float64{1.0, -2.0, 3.0}
	require.True(t, isFloatEqual(magnitude(vec), 3.7416573867739413))
}

func TestCoordinate_vectorFromTo(t *testing.T) {
	from := []float64{0.5, 0.6, 0.7}
	to := []float64{1.0, 2.0, 3.0}
	u, mag := vectorFromTo(from, to)
	require.True(t, isVectorEqual(u, []float64{0.18257418583505536, 0.511207720338155, 0.8398412548412546}))
	require.True(t, isFloatEqual(magnitude(u), 1.0))
	require.True(t, isFloatEqual(mag, magnitude(diffVec(to, from))))

	// random unit vector
	u, mag = vectorFromTo(to, to)
	require.True(t, isFloatEqual(magnitude(u), 1.0))
	require.True(t, isFloatEqual(mag, 0.0))
}
