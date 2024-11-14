package serf

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	memberlist "github.com/mbver/mlist"
)

const tagMagicByte msgType = 255

type SerfStateType int

const (
	SerfAlive SerfStateType = iota
	SerfLeft
	SerfShutdown
)

type Serf struct {
	config         *Config
	keyring        *memberlist.Keyring
	inEventCh      chan Event
	outEventCh     chan Event
	nodeEventCh    chan *memberlist.NodeEvent
	keyQuery       *keyQueryReceptor
	invokeScriptCh chan *invokeScript
	eventHandlers  *eventHandlerManager
	mlist          *memberlist.Memberlist
	ping           *pingDelegate
	usrState       *userStateDelegate
	broadcasts     *broadcastManager
	query          *QueryManager
	action         *ActionManager
	userMsgCh      chan []byte
	logger         *log.Logger
	state          SerfStateType
	stateL         sync.Mutex
	inactive       *inactiveNodes
	snapshot       *Snapshotter
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
	s.logger = b.logger
	s.shutdownCh = make(chan struct{})

	s.inEventCh = make(chan Event, 1024)
	snap, outCh, err := NewSnapshotter(s.config.SnapshotPath,
		s.config.SnapshotMinCompactSize,
		s.config.SnapshotDrainTimeout,
		s.logger,
		s.inEventCh,
		s.shutdownCh,
	)
	if err != nil {
		return nil, err
	}
	s.snapshot = snap

	outCh = NewMemberEventCoalescer(s.config.CoalesceInterval,
		outCh,
		s.logger,
		s.shutdownCh,
	)

	s.keyQuery = &keyQueryReceptor{
		inCh:  outCh,
		outCh: make(chan Event, 1024),
	}

	s.outEventCh = s.keyQuery.outCh

	s.invokeScriptCh = make(chan *invokeScript)
	s.eventHandlers = newEventHandlerManager()
	scriptHandlers := CreateScriptHandlers(s.config.EventScript, s.invokeScriptCh)
	s.eventHandlers.script.update(scriptHandlers)

	mbuilder := &memberlist.MemberlistBuilder{}
	mbuilder.WithConfig(b.mconf)
	mbuilder.WithLogger(b.logger)

	mbuilder.WithKeyRing(b.keyring)
	s.keyring = b.keyring

	s.nodeEventCh = make(chan *memberlist.NodeEvent, 1024)
	mbuilder.WithEventCh(s.nodeEventCh)

	usrMsgCh := make(chan []byte)
	s.userMsgCh = usrMsgCh
	mbuilder.WithUserMessageCh(usrMsgCh)

	broadcasts := newBroadcastManager(s.NumNodes, b.mconf.RetransmitMult, b.conf.MaxQueueDepth) // TODO: add a logger then?
	s.broadcasts = broadcasts
	mbuilder.WithUserBroadcasts(broadcasts)

	ping, err := newPingDelegate(b.logger)
	if err != nil {
		return nil, err
	}
	s.ping = ping
	mbuilder.WithPingDelegate(ping)

	s.query = newQueryManager(b.logger, b.conf.LBufferSize)
	s.action = newActionManager(b.conf.LBufferSize)

	s.usrState = newUserStateDelegate( // will not be used until memberlist join some nodes
		s.query.clock,
		s.action,
		s.logger,
		s.handleAction,
	)
	mbuilder.WithUserStateDelegate(s.usrState)

	s.tags = make(map[string]string)
	if len(b.tags) != 0 {
		encoded, err := encodeTags(s.tags)
		if len(encoded) > memberlist.TagMaxSize {
			return nil, memberlist.ErrMaxTagSizeExceed
		}
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
	s.ping.id = m.ID()

	s.setState(SerfAlive)
	s.inactive = newInactiveNodes(s.config.ReconnectTimeout, s.config.TombstoneTimeout)

	s.schedule()
	go s.receiveNodeEvents()
	go s.receiveKeyEvents()
	go s.receiveEvents()
	go s.receiveInvokeScripts()
	go s.receiveMsgs()
	go s.rejoinSnapshot()
	return s, nil
}

func (s *Serf) Join(existing []string, ignoreOld bool) (int, error) {
	s.usrState.setIgnoreActionsOnJoin(ignoreOld)
	s.usrState.setJoin(true)
	defer func() {
		s.usrState.setJoin(false)
		s.usrState.setIgnoreActionsOnJoin(false)
	}()
	return s.mlist.Join(existing)
}

// try to join 1 previously known-node
func (s *Serf) rejoinSnapshot() {
	prev := s.snapshot.AliveNodes()
	if len(prev) == 0 {
		return
	}
	for _, n := range prev {
		if n.ID == s.ID() { // it will not happen as new node always has new id!
			continue
		}
		_, err := s.mlist.Join([]string{n.Addr})
		if err == nil {
			s.logger.Printf("[INFO] serf: rejoined successfully previously known node")
			return
		}
	}
	s.logger.Printf("[WARN] serf: failed to join any previously known node")
}

func (s *Serf) Leave() error {
	if s.hasLeft() {
		return fmt.Errorf("already left")
	}
	if s.hasShutdown() {
		return fmt.Errorf("leave after shutdown")
	}
	s.setState(SerfLeft)
	s.snapshot.Leave()
	return s.mlist.Leave()
}

func (s *Serf) Shutdown() {
	s.mlist.Shutdown()
	close(s.shutdownCh)
	s.snapshot.Wait()
	s.setState(SerfShutdown)
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

func (s *Serf) setState(state SerfStateType) {
	s.stateL.Lock()
	defer s.stateL.Unlock()
	s.state = state
}

func (s *Serf) hasLeft() bool {
	s.stateL.Lock()
	defer s.stateL.Unlock()
	return s.state == SerfLeft
}

func (s *Serf) hasShutdown() bool {
	s.stateL.Lock()
	defer s.stateL.Unlock()
	return s.state == SerfShutdown
}

func scheduleFunc(interval time.Duration, stopCh chan struct{}, f func()) {
	t := time.NewTicker(interval)
	jitter := time.Duration(uint64(rand.Int63()) % uint64(interval))
	time.Sleep(jitter) // wait random fraction of interval to avoid thundering herd
	for {
		select {
		case <-t.C:
			f()
		case <-stopCh:
			t.Stop()
			return
		}
	}
}

func (s *Serf) schedule() {
	if s.config.ReapInterval > 0 {
		go scheduleFunc(s.config.ReapInterval, s.shutdownCh, s.reap)
	}
	if s.config.ReconnectInterval > 0 {
		go scheduleFunc(s.config.ReconnectInterval, s.shutdownCh, s.reconnect)
	}
	if s.config.ManageQueueDepthInterval > 0 {
		go scheduleFunc(s.config.ManageQueueDepthInterval, s.shutdownCh, s.broadcasts.manageQueueDepth)
	}
}

func (s *Serf) SetTags(tags map[string]string) error {
	s.tags = tags
	encoded, err := encodeTags(s.tags)
	if len(encoded) > memberlist.TagMaxSize { // no need, memberlist can do this!
		return memberlist.ErrMaxTagSizeExceed
	}
	if err != nil {
		return err
	}
	return s.mlist.UpdateTags(encoded)
}
