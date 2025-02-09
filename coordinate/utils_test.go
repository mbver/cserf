// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package coordinate

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

func isFloatEqual(f1 float64, f2 float64) bool {
	return math.Abs(f1-f2) <= zeroThreshold
}

func isVectorEqual(vec1 []float64, vec2 []float64) bool {
	if len(vec1) != len(vec2) {
		return false
	}

	for i := range vec1 {
		if !isFloatEqual(vec1[i], vec2[i]) {
			return false
		}
	}
	return true
}

func catchDimensionPanic(f func(), ch chan bool) {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				ch <- false
				return
			}
			ch <- errors.Is(err, ErrDimMisMatch)
			return
		}
		ch <- false
	}()
	f()
}

// create a slice of Nodes with same config
func GenerateNodes(num int, config *Config) ([]*Node, error) {
	nodes := make([]*Node, num)
	for i := range nodes {
		node, err := NewNode(config)
		if err != nil {
			return nil, err
		}

		nodes[i] = node
	}
	return nodes, nil
}

// truth matrix as if nodes are in a
// straight line with equal spacing
func GenerateLineTruth(nodes int, spacing time.Duration) [][]time.Duration {
	truth := make([][]time.Duration, nodes)
	for i := range truth {
		truth[i] = make([]time.Duration, nodes)
	}

	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes; j++ {
			rtt := time.Duration(j-i) * spacing
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// truth matrix as if all the nodes are in a
// two dimensional grid with equal spacing
// n = sqrt(num), i = x + y*n
func GenerateGridTruth(num int, spacing time.Duration) [][]time.Duration {
	truth := make([][]time.Duration, num)
	for i := range truth {
		truth[i] = make([]time.Duration, num)
	}

	n := int(math.Sqrt(float64(num)))
	for i := 0; i < num; i++ {
		for j := i + 1; j < num; j++ {
			x1, y1 := float64(i%n), float64(i/n)
			x2, y2 := float64(j%n), float64(j/n)
			dx, dy := x2-x1, y2-y1
			dist := math.Sqrt(dx*dx + dy*dy)
			rtt := time.Duration(dist * float64(spacing))
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// truth matrix with 2 group of nodes
// separated by a wan distance
func GenerateSplitTruth(n int, lan time.Duration, wan time.Duration) [][]time.Duration {
	truth := make([][]time.Duration, n)
	for i := range truth {
		truth[i] = make([]time.Duration, n)
	}

	split := n / 2
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			rtt := lan
			if i <= split && j > split {
				rtt += wan
			}
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// truth matrix with first node at the center and above other nodes.
// remaining nodes are evently distributed on the circle.
// alpha = 2*pi*i/num, x = cos(alpha), y = sin(alpha).
func GenerateCircleTruth(num int, radius time.Duration) [][]time.Duration {
	truth := make([][]time.Duration, num)
	for i := range truth {
		truth[i] = make([]time.Duration, num)
	}

	for i := 0; i < num; i++ {
		for j := i + 1; j < num; j++ {
			var rtt time.Duration
			if i == 0 {
				rtt = 2 * radius
			} else {
				a1 := 2.0 * math.Pi * float64(i) / float64(num)
				x1, y1 := math.Cos(a1), math.Sin(a1)
				a2 := 2.0 * math.Pi * float64(j) / float64(num)
				x2, y2 := math.Cos(a2), math.Sin(a2)
				dx, dy := x2-x1, y2-y1
				dist := math.Sqrt(dx*dx + dy*dy)
				rtt = time.Duration(dist * float64(radius))
			}
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// truth matrix where node distance to other node
// is generated with standard distribution
func GenerateRandomTruth(nodes int, mean time.Duration, deviation time.Duration) [][]time.Duration {
	rand.Seed(1)

	truth := make([][]time.Duration, nodes)
	for i := range truth {
		truth[i] = make([]time.Duration, nodes)
	}

	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes; j++ {
			rttSeconds := rand.NormFloat64()*deviation.Seconds() + mean.Seconds()
			rtt := secondsToDuration(rttSeconds)
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}
	return truth
}

// nodes are relaxed with a truth table for
// many rounds, to make their states converge
func Simulate(nodes []*Node, truth [][]time.Duration, cycles int) {
	rand.Seed(1) // for consistent result because nodes to relax is chosen randomly
	n := len(nodes)
	for cycle := 0; cycle < cycles; cycle++ {
		for i := range nodes {
			if j := rand.Intn(n); j != i {
				c := nodes[j].GetCoordinate()
				rtt := truth[i][j]
				id := fmt.Sprintf("node_%d", j)
				nodes[i].Relax(id, c, rtt)
			}
		}
	}
}

// Stats is returned from the Evaluate function with a summary of the algorithm
// performance.
type Stats struct {
	ErrorMax float64
	ErrorAvg float64
}

// Assess the convergence of the algorithm:
// how close the coordinates compare to the truth.
func Evaluate(nodes []*Node, truth [][]time.Duration) (stats Stats) {
	n := len(nodes)
	count := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			est := nodes[i].DistanceTo(nodes[j].GetCoordinate()).Seconds()
			actual := truth[i][j].Seconds()
			error := math.Abs(est-actual) / actual
			stats.ErrorMax = math.Max(stats.ErrorMax, error)
			stats.ErrorAvg += error
			count += 1
		}
	}

	stats.ErrorAvg /= float64(count)
	fmt.Printf("Error avg=%9.6f max=%9.6f\n", stats.ErrorAvg, stats.ErrorMax)
	return
}
