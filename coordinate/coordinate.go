package coordinate

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	secToNano     = 1.0e9
	zeroThreshold = 1.0e-6
)

var ErrDimMisMatch = fmt.Errorf("coordinate dimensionality does not match")

func secondsToDuration(secs float64) time.Duration {
	return time.Duration(secs * secToNano)
}

type Coordinate struct {
	// The unit of time used for the following fields is millisecond
	Euclide    []float64 // represents geographic coordinates (high speed internet core).
	Height     float64   // represents the latency between access links of to internet core.
	Error      float64
	Adjustment float64
}

func NewCoordinate(config *Config) *Coordinate {
	return &Coordinate{
		Euclide:    make([]float64, config.Dimensionality),
		Height:     config.HeightMin,
		Error:      config.VivaldiErrorMax,
		Adjustment: 0.0,
	}
}

func (c *Coordinate) Clone() *Coordinate {
	e := make([]float64, len(c.Euclide))
	copy(e, c.Euclide)
	return &Coordinate{
		Euclide:    e,
		Error:      c.Error,
		Adjustment: c.Adjustment,
		Height:     c.Height,
	}
}

func isValidComponent(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f)
}

func isValidCoord(c *Coordinate) bool {
	for _, v := range c.Euclide {
		if !isValidComponent(v) {
			return false
		}
	}
	return isValidComponent(c.Height) &&
		isValidComponent(c.Error) &&
		isValidComponent(c.Adjustment)
}

// check if dimension matches
func matchDim(this, that *Coordinate) bool {
	return len(this.Euclide) == len(that.Euclide)
}

// pull/push "to" to/from "from"
func ApplyForce(from, to *Coordinate, minHeight, force float64) *Coordinate {
	if !matchDim(from, to) {
		panic(ErrDimMisMatch)
	}
	unit, mag := vectorFromTo(from.Euclide, to.Euclide)
	ret := to.Clone()
	ret.Euclide = addVec(ret.Euclide, scaleVec(unit, force))
	if mag > zeroThreshold {
		ret.Height += (from.Height + to.Height) * force / mag // does force really affects Height?
		ret.Height = math.Max(ret.Height, minHeight)
	}
	return ret
}

// distance between 2 points in time.Duration
// with adjustment included
func (c *Coordinate) DistanceTo(other *Coordinate) time.Duration {
	if !matchDim(c, other) {
		panic(ErrDimMisMatch)
	}
	d := distance(c, other)
	adjusted := d + c.Adjustment + other.Adjustment
	if adjusted <= 0 {
		adjusted = d
	}
	return secondsToDuration(adjusted)
}

// distance between 2 points in seconds
func distance(this, that *Coordinate) float64 {
	return magnitude(diffVec(this.Euclide, that.Euclide)) + this.Height + that.Height
}

func addVec(vec1 []float64, vec2 []float64) []float64 {
	ret := make([]float64, len(vec1))
	for i := range ret {
		ret[i] = vec1[i] + vec2[i]
	}
	return ret
}

func diffVec(vec1 []float64, vec2 []float64) []float64 {
	ret := make([]float64, len(vec1))
	for i := range ret {
		ret[i] = vec1[i] - vec2[i]
	}
	return ret
}

func scaleVec(vec []float64, factor float64) []float64 {
	ret := make([]float64, len(vec))
	for i := range vec {
		ret[i] = vec[i] * factor
	}
	return ret
}

// magnitude computes the magnitude of the vec.
func magnitude(vec []float64) float64 {
	sum := 0.0
	for i := range vec {
		sum += vec[i] * vec[i]
	}
	return math.Sqrt(sum)
}

// the vector from-to decomposed to a unit vector and magnitude
func vectorFromTo(from []float64, to []float64) ([]float64, float64) {
	vec := diffVec(to, from)
	if m := magnitude(vec); m > zeroThreshold {
		return scaleVec(vec, 1.0/m), m
	}
	return randUnitVector(len(vec)), 0.0
}

func randUnitVector(dim int) []float64 {
	vec := make([]float64, dim)
	var m float64
	for m <= zeroThreshold {
		for i := range vec {
			vec[i] = rand.Float64() - 0.5
		}
		m = magnitude(vec)
	}
	return scaleVec(vec, 1/m)
}
