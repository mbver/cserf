package serf

import (
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
	mbuilder := &memberlist.MemberlistBuilder{}
	mbuilder.WithConfig(b.mconf)
	mbuilder.WithLogger(b.logger)
	mbuilder.WithKeyRing(b.keyring)
	usrMsgCh := make(chan []byte)
	mbuilder.WithUserMessageCh(usrMsgCh)
	broadcasts := newBroadcastManager() // add a logger then?
	mbuilder.WithUserBroadcasts(broadcasts)
	m, err := mbuilder.Build()
	if err != nil {
		return nil, err
	}
	s := &Serf{
		mlist:      m,
		broadcasts: broadcasts,
		query:      newQueryManager(b.logger),
		userMsgCh:  usrMsgCh,
		logger:     b.logger,
		shutdownCh: make(chan struct{}),
	}
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
