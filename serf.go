package serf

import (
	"fmt"
	"log"

	memberlist "github.com/mbver/mlist"
)

const tagMagicByte msgType = 255

type Serf struct {
	config         *Config
	inEventCh      chan Event
	outEventCh     chan Event
	invokeScriptCh chan *invokeScript
	eventHandlers  *eventHandlerManager
	mlist          *memberlist.Memberlist
	broadcasts     *broadcastManager
	query          *QueryManager
	userMsgCh      chan []byte
	logger         *log.Logger
	shutdownCh     chan struct{}
	tags           map[string]string
}

type SerfBuilder struct {
	mconf   *memberlist.Config
	conf    *Config
	keyring *memberlist.Keyring
	logger  *log.Logger
	tags    map[string]string
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

func (b *SerfBuilder) WithTags(tags map[string]string) {
	b.tags = tags
}

func (b *SerfBuilder) Build() (*Serf, error) {
	s := &Serf{}
	s.config = b.conf

	eventCh := make(chan Event, 1024)
	s.inEventCh = eventCh
	// TODO: setup snapshot, coalescer and key event handlers to change outEventCh later
	s.outEventCh = eventCh

	s.invokeScriptCh = make(chan *invokeScript)
	s.eventHandlers = newEventHandlerManager()
	scriptHandlers := CreateScriptHandlers(s.config.EventScript, s.invokeScriptCh)
	s.eventHandlers.script.update(scriptHandlers)

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

	s.tags = make(map[string]string)
	if len(b.tags) != 0 {
		encoded, err := encodeTags(s.tags)
		if err != nil {
			return nil, err
		}
		b.mconf.Tags = encoded
		s.tags = b.tags
	}

	m, err := mbuilder.Build()
	if err != nil {
		return nil, err
	}
	s.mlist = m

	s.logger = b.logger
	s.query = newQueryManager(b.logger, b.conf.QueryBufferSize)
	s.shutdownCh = make(chan struct{})

	go s.receiveMsgs()
	go s.receiveEvents()
	go s.receiveInvokeScripts()

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
