package coordinate

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

const (
	maxRTT = 10 * time.Second
)

// store the latest latencies of a node in a circular buffer
type latencyRing struct {
	idx       int
	size      int
	latencies []float64
}

func newLatencyRing(size int) *latencyRing {
	return &latencyRing{
		idx:       -1,
		size:      size,
		latencies: make([]float64, 0, size),
	}
}

func (r *latencyRing) add(l float64) {
	if r.size == 0 {
		return
	}
	r.idx++
	if r.idx == r.size {
		r.idx = 0
	}
	if len(r.latencies) == r.idx {
		r.latencies = append(r.latencies, 0) // extend
	}
	r.latencies[r.idx] = l
}

func (r *latencyRing) median() float64 {
	if len(r.latencies) == 0 {
		return 0
	}
	tmp := make([]float64, len(r.latencies))
	copy(tmp, r.latencies)
	sort.Float64s(tmp)
	return tmp[len(tmp)/2]
}

// if adjustment is calculated from len(r.latencies),
// the initial relaxations can cause large latency differences.
// that can affect the convergence of circle topology test.
func getAdjustment(r *latencyRing) float64 {
	sum := 0.0
	for _, l := range r.latencies {
		sum += l
	}
	return sum / (float64(r.size) * 2.0)
}

type Node struct {
	coord *Coordinate
	// origin is a coordinate sitting at the origin.
	origin *Coordinate
	config *Config
	// latest latency-differences after relaxation
	latenciesDiff *latencyRing
	// map a node to its latencies ring
	latenciesMap map[string]*latencyRing
	numResets    int

	l sync.RWMutex // enable safe conccurent access
}

// NewNode creates a new Node.
func NewNode(config *Config) (*Node, error) {
	if config.Dimensionality <= 0 {
		return nil, fmt.Errorf("dimensionality must be >0")
	}
	return &Node{
		coord:         NewCoordinate(config),
		origin:        NewCoordinate(config),
		config:        config,
		latenciesDiff: newLatencyRing(config.AdjustmentWindowSize),
		latenciesMap:  make(map[string]*latencyRing),
	}, nil
}

func (n *Node) GetCoordinate() *Coordinate {
	n.l.RLock()
	defer n.l.RUnlock()

	return n.coord.Clone()
}

func (n *Node) SetCoordinate(coord *Coordinate) error {
	n.l.Lock()
	defer n.l.Unlock()

	if err := n.checkCoordinate(coord); err != nil {
		return err
	}
	n.coord = coord.Clone()
	return nil
}

// remove the node's latecy-ring from the latenciesMap
func (n *Node) ForgetNode(node string) {
	n.l.Lock()
	defer n.l.Unlock()

	delete(n.latenciesMap, node)
}

// Stats returns a copy of stats for the client.
func (n *Node) NumResets() int {
	n.l.Lock()
	defer n.l.Unlock()

	return n.numResets
}

// check if the coordinate is valid for this client
func (n *Node) checkCoordinate(coord *Coordinate) error {
	if !matchDim(n.coord, coord) {
		return fmt.Errorf("dimensions aren't compatible")
	}

	if !isValidCoord(coord) {
		return fmt.Errorf("coordinate is invalid")
	}

	return nil
}

// add new value to node's latency-ring and get its median value
func (n *Node) addLatency(node string, rttSeconds float64) float64 {
	samples, ok := n.latenciesMap[node]
	if !ok {
		samples = newLatencyRing(n.config.LatencyFilterSize)
	}
	samples.add(rttSeconds)
	return samples.median()
}

// given from-coord and rtt, apply force to current coord to move it to balance
func (n *Node) relax(from *Coordinate, rttSeconds float64) {

	dist := TimeDist(n.coord, from).Seconds()
	if rttSeconds < zeroThreshold {
		rttSeconds = zeroThreshold
	}
	latDiff := rttSeconds - dist             // latency difference
	latDev := math.Abs(latDiff) / rttSeconds // latency deviation

	totalError := n.coord.Error + from.Error
	if totalError < zeroThreshold {
		totalError = zeroThreshold
	}
	errDev := n.coord.Error / totalError // error deviation

	// update new error
	n.coord.Error += n.config.VivaldiCE * errDev * (latDev - n.coord.Error)
	if n.coord.Error > n.config.VivaldiErrorMax {
		n.coord.Error = n.config.VivaldiErrorMax
	}

	// apply force
	force := n.config.VivaldiCC * errDev * latDiff
	n.coord = ApplyForce(from, n.coord, n.config.HeightMin, force)
}

// adjust happens after relaxation to correct
// the discrepancy between coord-distance and real latency
func (n *Node) adjust(other *Coordinate, rttSeconds float64) {
	if n.config.AdjustmentWindowSize == 0 {
		return
	}

	latDiff := rttSeconds - distance(n.coord, other)
	n.latenciesDiff.add(latDiff)

	n.coord.Adjustment = getAdjustment(n.latenciesDiff) // each node responsible for half latency diff
}

// pull the coord slightly toward center
// to combat drift
func (n *Node) centerNudge() {
	dist := TimeDist(n.origin, n.coord).Seconds()
	force := -1.0 * math.Pow(dist/n.config.GravityRho, 2.0)
	n.coord = ApplyForce(n.origin, n.coord.Clone(), n.config.HeightMin, force)
}

// update the current node's coordinate based on response from other node
func (n *Node) Relax(node string, other *Coordinate, rtt time.Duration) (*Coordinate, error) {
	n.l.Lock()
	defer n.l.Unlock()

	if err := n.checkCoordinate(other); err != nil {
		return nil, err
	}
	if rtt < 0 || rtt > maxRTT {
		return nil, fmt.Errorf("round trip time not in valid range, duration %v is not a positive value less than %v ", rtt, maxRTT)
	}
	rttSeconds := n.addLatency(node, rtt.Seconds())
	n.relax(other, rttSeconds)
	n.adjust(other, rttSeconds)
	n.centerNudge()

	if !isValidCoord(n.coord) { // drift too far!
		n.numResets++
		n.coord = NewCoordinate(n.config)
	}

	return n.coord.Clone(), nil
}

// coordinate for another node.
func (n *Node) DistanceTo(other *Coordinate) time.Duration {
	n.l.RLock()
	defer n.l.RUnlock()

	return TimeDist(n.coord, other)
}
