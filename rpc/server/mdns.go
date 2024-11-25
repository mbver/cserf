package server

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	serf "github.com/mbver/cserf"
	mdns "github.com/mbver/mmdns"
)

const (
	mdnsPollInterval = 60 * time.Second
	mdnsWaitInterval = 100 * time.Millisecond
)

type ClusterMDNS struct {
	serf        *serf.Serf
	clusterName string
	logger      *log.Logger
	seen        map[string]struct{}
	ignoreOld   bool
	iface       *net.Interface
}

func NewClusterMDNS(
	serf *serf.Serf,
	clusterName string,
	logger *log.Logger,
	ignoreOld bool,
	iface *net.Interface,
) (*ClusterMDNS, error) {
	ip, port, err := serf.AdvertiseIpPort()
	if err != nil {
		return nil, err
	}
	mService := &mdns.MDNSService{
		Instance: serf.ID(),
		Service:  toMdnsName(clusterName),
		Addr:     ip,
		Port:     int(port),
		Info:     fmt.Sprintf("serf '%s' cluster", clusterName),
	}
	if err := mService.Init(); err != nil {
		return nil, err
	}

	mconf := &mdns.Config{
		Zone:  mService, // very confusing
		Iface: iface,
	}

	if _, err := mdns.NewServer(mconf); err != nil {
		return nil, err
	}

	m := &ClusterMDNS{
		serf:        serf,
		clusterName: clusterName,
		logger:      logger,
		seen:        map[string]struct{}{},
		ignoreOld:   ignoreOld,
		iface:       iface,
	}
	go m.run()
	return m, nil
}

func toMdnsName(serviceName string) string {
	return fmt.Sprintf("_serf_%s._tcp", serviceName)
}

func (m *ClusterMDNS) run() {
	resultCh := make(chan *mdns.ServiceEntry, 32)
	pollCh := time.After(0)
	var waitCh <-chan time.Time
	var join []string

	for {
		select {
		case r := <-resultCh:
			addr := net.JoinHostPort(r.Addr.String(), strconv.Itoa(r.Port))
			if _, ok := m.seen[addr]; ok {
				continue
			}
			m.seen[addr] = struct{}{}
			join = append(join, addr)
			waitCh = time.After(mdnsWaitInterval)
		case <-waitCh:
			n, err := m.serf.Join(join, m.ignoreOld)
			if err != nil {
				m.logger.Printf("[ERR] cluster_mdns: failed to join %v", err)
			}
			if n > 0 {
				m.logger.Printf("[ERR] cluser_mdns: joined %d hosts", n)
			}
		case <-pollCh:
			pollCh = time.After(mdnsPollInterval)
			go m.poll(resultCh)
		}
	}
}

func (m *ClusterMDNS) poll(resultCh chan *mdns.ServiceEntry) {
	p := mdns.QueryParam{
		Service:   toMdnsName(m.clusterName),
		Interface: m.iface,
		Entries:   resultCh,
	}
	if err := mdns.Query(&p); err != nil {
		m.logger.Printf("[ERR] cluster_mdns: failed to poll for new hosts: %v", err)
	}
}
