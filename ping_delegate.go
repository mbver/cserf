package serf

import (
	"log"
	"sync"
	"time"

	"github.com/mbver/cserf/coordinate"
	memberlist "github.com/mbver/mlist"
)

type pingDelegate struct {
	id         string // id only ready when mlist started
	coord      *coordinate.Node
	coordCache map[string]*coordinate.Coordinate
	cacheLock  sync.RWMutex
	logger     *log.Logger
}

func newPingDelegate(logger *log.Logger) (*pingDelegate, error) {
	coord, err := coordinate.NewNode(coordinate.DefaultConfig())
	if err != nil {
		return nil, err
	}
	return &pingDelegate{
		coord:      coord,
		coordCache: make(map[string]*coordinate.Coordinate),
		logger:     logger,
	}, nil
}

func (p *pingDelegate) Payload() []byte {
	encoded, err := encode(msgCoordType, p.coord.GetCoordinate())
	if err != nil {
		p.logger.Printf("[ERR] serf: fail to encode coordinate %v", err)
		return nil
	}
	return encoded
}

// receive coordinate from other serf's and update our coordinate
func (p *pingDelegate) Finish(other *memberlist.Node, rtt time.Duration, payload []byte) {
	if len(payload) == 0 || payload[0] != byte(msgCoordType) {
		p.logger.Print("[WARN] serf: ping delegate: invalid payload")
		return
	}
	var coord coordinate.Coordinate
	err := decode(payload[1:], &coord)
	if err != nil {
		p.logger.Printf("[ERR] serf: ping delegeate: failed to decode payload %v", err)
		return
	}
	_, err = p.coord.Relax(other.ID, &coord, rtt)
	if err != nil {
		p.logger.Printf("ERR] serf: ping delegate: failed to update coord %v", err)
		return
	}
	p.cacheLock.Lock()
	defer p.cacheLock.Unlock()
	p.coordCache[other.ID] = &coord
	p.coordCache[p.id] = p.coord.GetCoordinate()
}

func (p *pingDelegate) GetCachedCoord(id string) *coordinate.Coordinate {
	p.cacheLock.Lock()
	defer p.cacheLock.Unlock()
	return p.coordCache[id]
}

func (s *Serf) GetCoordinate() *coordinate.Coordinate {
	return s.ping.coord.GetCoordinate()
}

func (s *Serf) GetCachedCoord(id string) *coordinate.Coordinate {
	return s.ping.GetCachedCoord(id)
}
