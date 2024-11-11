package serf

import (
	"log"
	"time"
)

// coalesce: eventype, members ==> map, coalesce interval, inCh, outCh
type MemberEventCoalescer struct {
	flushInterval time.Duration
	inCh          chan Event
	outCh         chan Event
	eventMap      map[EventType][]*Member
	logger        *log.Logger
	shutdownCh    chan struct{}
}

func NewMemberEventCoalescer(interval time.Duration, inCh chan Event, logger *log.Logger, shutdownCh chan struct{}) chan Event {
	outch := make(chan Event, 1024)
	c := &MemberEventCoalescer{
		flushInterval: interval,
		inCh:          inCh,
		outCh:         outch,
		eventMap:      make(map[EventType][]*Member),
		logger:        logger,
		shutdownCh:    shutdownCh,
	}
	go c.coalesce()
	return outch
}

func isMemberEvent(t EventType) bool {
	return t == EventMemberJoin || t == EventMemberLeave ||
		t == EventMemberUpdate || t == EventMemberReap
}

func (c *MemberEventCoalescer) flush() {
	for t, members := range c.eventMap {
		c.outCh <- &CoalescedMemberEvent{
			Type:    t,
			Members: members,
		}
	}
	c.eventMap = make(map[EventType][]*Member) // clean up flushed events
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
			c.eventMap[etype] = append(c.eventMap[etype], mEvent.Member)
		case <-flushTicker.C:
			c.flush()
		case <-c.shutdownCh:
			c.flush()
			c.logger.Printf("[WARN] serf member-event-coalescer: serf shutdown, quitting coalesce events")
			return
		}
	}
}
