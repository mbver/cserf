package serf

import (
	"fmt"
	"log"

	memberlist "github.com/mbver/mlist"
)

type Serf struct {
	mlist      *memberlist.Memberlist
	broadcasts *broadcastManager
	query      *QueryManager
	userMsgCh  chan []byte
	logger     *log.Logger
	shutdownCh chan struct{}
}

type SerfBuilder struct {
	mconf   *memberlist.Config
	conf    *Config
	keyring *memberlist.Keyring
	logger  *log.Logger
}

func (b *SerfBuilder) WithMemberlistConfig(conf *memberlist.Config) {
	b.mconf = conf
}

func (b *SerfBuilder) WithConfig(conf *Config) {
	b.conf = conf
}

func (b *SerfBuilder) WithKeyring(k *memberlist.Keyring) {
	b.keyring = k
}

func (b *SerfBuilder) WithLogger(l *log.Logger) {
	b.logger = l
}

func (b *SerfBuilder) Build() (*Serf, error) {
	s := &Serf{}
	mbuilder := &memberlist.MemberlistBuilder{}
	mbuilder.WithConfig(b.mconf)
	mbuilder.WithLogger(b.logger)
	mbuilder.WithKeyRing(b.keyring)

	usrMsgCh := make(chan []byte)
	s.userMsgCh = usrMsgCh
	mbuilder.WithUserMessageCh(usrMsgCh)

	broadcasts := newBroadcastManager(s.NumNodes, b.mconf.RetransmitMult) // TODO: add a logger then?
	s.broadcasts = broadcasts
	mbuilder.WithUserBroadcasts(broadcasts)
	m, err := mbuilder.Build()
	if err != nil {
		return nil, err
	}
	s.mlist = m

	s.logger = b.logger
	s.query = newQueryManager(b.logger, b.conf.QueryBufferSize)
	s.shutdownCh = make(chan struct{})

	go s.receiveMsgs()
	return s, nil
}

func (s *Serf) Join(existing []string) (int, error) {
	return s.mlist.Join(existing)
}

func (s *Serf) Shutdown() {
	s.mlist.Shutdown()
	close(s.shutdownCh)
}

func (s *Serf) NumNodes() int {
	return s.mlist.GetNumNodes()
}

func (s *Serf) AdvertiseAddress() (string, error) {
	ip, port, err := s.mlist.GetAdvertiseAddr()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", ip.String(), port), err
}

func (s *Serf) ID() string {
	return s.mlist.ID()
}
