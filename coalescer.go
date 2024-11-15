package serf

import (
	"log"
	"time"

	memberlist "github.com/mbver/mlist"
)

// coalesce: eventype, members ==> map, coalesce interval, inCh, outCh
type MemberEventCoalescer struct {
	flushInterval time.Duration
	inCh          chan Event
	outCh         chan Event
	newEvents     map[string]*MemberEvent
	oldEvents     map[string]*MemberEvent // TODO: cleanup it periodically too?
	logger        *log.Logger
	shutdownCh    chan struct{}
}

func NewMemberEventCoalescer(interval time.Duration, inCh chan Event, logger *log.Logger, shutdownCh chan struct{}) chan Event {
	outch := make(chan Event, 1024)
	c := &MemberEventCoalescer{
		flushInterval: interval,
		inCh:          inCh,
		outCh:         outch,
		oldEvents:     make(map[string]*MemberEvent),
		newEvents:     make(map[string]*MemberEvent),
		logger:        logger,
		shutdownCh:    shutdownCh,
	}
	go c.coalesce()
	return outch
}

func isMemberEvent(t EventType) bool {
	return t == EventMemberJoin || t == EventMemberLeave ||
		t == EventMemberFailed || t == EventMemberUpdate ||
		t == EventMemberReap
}

func (c *MemberEventCoalescer) flush() {
	coalesced := make(map[EventType]*CoalescedMemberEvent)
	for id, e := range c.newEvents {
		if e.Equal(c.oldEvents[id]) {
			continue
		}
		if _, ok := coalesced[e.Type]; !ok {
			coalesced[e.Type] = &CoalescedMemberEvent{
				Type:    e.Type,
				Members: []*memberlist.Node{e.Member},
			}
			continue
		}
		coalesced[e.Type].Members = append(coalesced[e.Type].Members, e.Member)
		c.oldEvents[id] = e
	}

	for _, e := range coalesced {
		c.outCh <- e
	}
	c.newEvents = make(map[string]*MemberEvent) // clean up flushed events
}

func (c *MemberEventCoalescer) coalesce() {
	flushTicker := time.NewTicker(c.flushInterval)
	defer flushTicker.Stop()
	for {
		select {
		case e := <-c.inCh:
			etype := e.EventType()
			if !isMemberEvent(etype) {
				c.outCh <- e
				continue
			}
			mEvent := e.(*MemberEvent)
			c.newEvents[mEvent.Member.ID] = mEvent
		case <-flushTicker.C:
			c.flush()
		case <-c.shutdownCh:
			c.flush()
			c.logger.Printf("[WARN] serf member-event-coalescer: serf shutdown, quitting coalesce events")
			return
		}
	}
}
