// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package coordinate

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	spacing = 10 * time.Millisecond
	cycles  = 1000
)

func TestTopology_Line(t *testing.T) {
	n := 10
	config := DefaultConfig()
	clients, err := GenerateNodes(n, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateLineTruth(n, spacing)
	// relax all nodes 1000 rounds
	Simulate(clients, truth, cycles)
	stats := Evaluate(clients, truth)
	if stats.ErrorAvg > 0.0018 || stats.ErrorMax > 0.0092 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestTopology_Grid(t *testing.T) {
	n := 25
	config := DefaultConfig()
	nodes, err := GenerateNodes(n, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateGridTruth(n, spacing)
	Simulate(nodes, truth, cycles)
	stats := Evaluate(nodes, truth)
	if stats.ErrorAvg > 0.0015 || stats.ErrorMax > 0.022 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestTopology_Split(t *testing.T) {
	lan, wan := 1*time.Millisecond, 10*time.Millisecond
	n := 25

	config := DefaultConfig()
	nodes, err := GenerateNodes(n, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateSplitTruth(n, lan, wan)
	Simulate(nodes, truth, cycles)
	stats := Evaluate(nodes, truth)
	if stats.ErrorAvg > 0.000060 || stats.ErrorMax > 0.00048 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestTopology_Circle(t *testing.T) {
	radius := 100 * time.Millisecond
	n := 25

	// 2D constraints to exactly represent the circle
	config := DefaultConfig()
	config.Dimensionality = 2
	nodes, err := GenerateNodes(n, config)
	if err != nil {
		t.Fatal(err)
	}

	truth := GenerateCircleTruth(n, radius)
	Simulate(nodes, truth, cycles)

	// check nodes' height
	for i := range nodes {
		coord := nodes[i].GetCoordinate()
		if i == 0 {
			// center's height = radius
			if coord.Height < 0.97*radius.Seconds() {
				t.Fatalf("height is out of spec: %9.6f", coord.Height)
			}
		} else {
			// remaining nodes height = 0
			if coord.Height > 0.03*radius.Seconds() {
				t.Fatalf("height is out of spec: %9.6f", coord.Height)
			}
		}
	}
	stats := Evaluate(nodes, truth)
	if stats.ErrorAvg > 0.0025 || stats.ErrorMax > 0.064 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}

func TestTopology_Square_Drift(t *testing.T) {
	dist := 500 * time.Millisecond
	n := 4

	config := DefaultConfig()
	config.Dimensionality = 2
	nodes, err := GenerateNodes(n, config)
	if err != nil {
		t.Fatal(err)
	}

	// put nodes on a square
	// the origin is at the center
	// (1)  <- dist ->  (2)
	//
	//  |                |
	//  |       x        |
	//  |                |
	//
	// (0)  <- dist ->  (3)
	nodes[0].coord.Euclide = []float64{-0.5 * dist.Seconds(), -0.5 * dist.Seconds()}
	nodes[1].coord.Euclide = []float64{0.5 * dist.Seconds(), -0.5 * dist.Seconds()}
	nodes[2].coord.Euclide = []float64{0.5 * dist.Seconds(), 0.5 * dist.Seconds()}
	nodes[3].coord.Euclide = []float64{-0.5 * dist.Seconds(), 0.5 * dist.Seconds()}

	// create a truth table for the square
	truth := make([][]time.Duration, n)
	for i := range truth {
		truth[i] = make([]time.Duration, n)
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			rtt := dist
			if (i%2 == 0) && (j%2 == 0) { // node 0 and nod 2
				rtt = time.Duration(math.Sqrt2 * float64(rtt))
			} else if (i%2 == 1) && (j%2 == 1) {
				rtt = time.Duration(math.Sqrt2 * float64(rtt))
			}
			truth[i][j], truth[j][i] = rtt, rtt
		}
	}

	// find how far the cluster center drifts from origin
	centerDrift := func() float64 {
		// min stored minX, minY of all nodes
		// max stored maxX, maxY of all nodes
		min, max := nodes[0].GetCoordinate(), nodes[0].GetCoordinate()
		for i := 1; i < n; i++ {
			coord := nodes[i].GetCoordinate()
			for j, v := range coord.Euclide {
				min.Euclide[j] = math.Min(min.Euclide[j], v)
				max.Euclide[j] = math.Max(max.Euclide[j], v)
			}
		}
		// mid is the middle of min and max
		mid := make([]float64, config.Dimensionality)
		for i := range mid {
			mid[i] = min.Euclide[i] + (max.Euclide[i]-min.Euclide[i])/2
		}
		return magnitude(mid)
	}

	Simulate(nodes, truth, 10000)
	drift := centerDrift()
	require.True(t, drift < 0.01, "drift should be less than 1%")
}

func TestPerformance_Random(t *testing.T) {
	mean, deviation := 100*time.Millisecond, 10*time.Millisecond
	n := 25
	config := DefaultConfig()
	clients, err := GenerateNodes(n, config)
	if err != nil {
		t.Fatal(err)
	}
	truth := GenerateRandomTruth(n, mean, deviation)
	Simulate(clients, truth, cycles)
	stats := Evaluate(clients, truth)

	// very bad ErrorMax
	if stats.ErrorAvg > 0.075 || stats.ErrorMax > 0.33 {
		t.Fatalf("performance stats are out of spec: %v", stats)
	}
}
