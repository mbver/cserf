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

// specially use for only adjustment
func (r *latencyRing) avg() float64 {
	sum := 0.0
	for _, l := range r.latencies {
		sum += l
	}
	return sum / float64(r.size)
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

func (c *Node) GetCoordinate() *Coordinate {
	c.l.RLock()
	defer c.l.RUnlock()

	return c.coord.Clone()
}

func (c *Node) SetCoordinate(coord *Coordinate) error {
	c.l.Lock()
	defer c.l.Unlock()

	if err := c.checkCoordinate(coord); err != nil {
		return err
	}
	c.coord = coord.Clone()
	return nil
}

// remove the node's latecy-ring from the latenciesMap
func (c *Node) ForgetNode(node string) {
	c.l.Lock()
	defer c.l.Unlock()

	delete(c.latenciesMap, node)
}

// Stats returns a copy of stats for the client.
func (c *Node) NumResets() int {
	c.l.Lock()
	defer c.l.Unlock()

	return c.numResets
}

// check if the coordinate is valid for this client
func (c *Node) checkCoordinate(coord *Coordinate) error {
	if !matchDim(c.coord, coord) {
		return fmt.Errorf("dimensions aren't compatible")
	}

	if !isValidCoord(coord) {
		return fmt.Errorf("coordinate is invalid")
	}

	return nil
}

// add new value to node's latency-ring and get its median value
func (c *Node) addLatency(node string, rttSeconds float64) float64 {
	samples, ok := c.latenciesMap[node]
	if !ok {
		samples = newLatencyRing(c.config.LatencyFilterSize)
	}
	samples.add(rttSeconds)
	return samples.median()
}

// given from-coord and rtt, apply force to current coord to move it to balance
func (c *Node) relax(from *Coordinate, rttSeconds float64) {

	dist := TimeDist(c.coord, from).Seconds()
	if rttSeconds < zeroThreshold {
		rttSeconds = zeroThreshold
	}
	latDiff := rttSeconds - dist             // latency difference
	latDev := math.Abs(latDiff) / rttSeconds // latency deviation

	totalError := c.coord.Error + from.Error
	if totalError < zeroThreshold {
		totalError = zeroThreshold
	}
	errDev := c.coord.Error / totalError // error deviation

	// update new error
	c.coord.Error += c.config.VivaldiCE * errDev * (latDev - c.coord.Error)
	if c.coord.Error > c.config.VivaldiErrorMax {
		c.coord.Error = c.config.VivaldiErrorMax
	}

	// apply force
	force := c.config.VivaldiCC * errDev * latDiff
	c.coord = ApplyForce(from, c.coord, c.config.HeightMin, force)
}

// adjust happens after relaxation to correct
// the discrepancy between coord-distance and real latency
func (c *Node) adjust(other *Coordinate, rttSeconds float64) {
	if c.config.AdjustmentWindowSize == 0 {
		return
	}

	latDiff := rttSeconds - distance(c.coord, other)
	c.latenciesDiff.add(latDiff)

	c.coord.Adjustment = c.latenciesDiff.avg() / 2.0 // each node responsible for half latency diff
}

// pull the coord slightly toward center
// to combat drift
func (c *Node) centerNudge() {
	dist := TimeDist(c.origin, c.coord).Seconds()
	force := -1.0 * math.Pow(dist/c.config.GravityRho, 2.0)
	c.coord = ApplyForce(c.origin, c.coord.Clone(), c.config.HeightMin, force)
}

// update the current node's coordinate based on response from other node
func (c *Node) Relax(node string, other *Coordinate, rtt time.Duration) (*Coordinate, error) {
	c.l.Lock()
	defer c.l.Unlock()

	if err := c.checkCoordinate(other); err != nil {
		return nil, err
	}
	if rtt < 0 || rtt > maxRTT {
		return nil, fmt.Errorf("round trip time not in valid range, duration %v is not a positive value less than %v ", rtt, maxRTT)
	}
	rttSeconds := c.addLatency(node, rtt.Seconds())
	c.relax(other, rttSeconds)
	c.adjust(other, rttSeconds)
	c.centerNudge()

	if !isValidCoord(c.coord) { // drift too far!
		c.numResets++
		c.coord = NewCoordinate(c.config)
	}

	return c.coord.Clone(), nil
}

// coordinate for another node.
func (c *Node) DistanceTo(other *Coordinate) time.Duration {
	c.l.RLock()
	defer c.l.RUnlock()

	return TimeDist(c.coord, other)
}
