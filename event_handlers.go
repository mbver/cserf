package serf

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/armon/circbuf"
	memberlist "github.com/mbver/mlist"
)

type EventHandler interface {
	HandleEvent(Event)
}
type eventHandlerManager struct {
	script *scriptEventHandlerManager
	stream *streamEventHandlerManager
}

func newEventHandlerManager() *eventHandlerManager {
	return &eventHandlerManager{
		script: &scriptEventHandlerManager{},
		stream: &streamEventHandlerManager{
			handlerMap: make(map[*StreamEventHandler]struct{}),
		},
	}
}

func (m *eventHandlerManager) handleEvent(e Event) {
	m.script.handleEvent(e)
	m.stream.handleEvent(e)
}

type eventFilter struct {
	eventType string
	name      string
}

func (f *eventFilter) matches(e Event) bool {
	if f.eventType == "*" {
		return true
	}
	if e.EventType().String() != f.eventType {
		return false
	}
	if f.name == "" {
		return true
	}
	if q, ok := e.(*QueryEvent); ok {
		if q.Name != f.name {
			return false
		}
	}
	return true
}

func isValidEventFilter(f *eventFilter) bool {
	etype := f.eventType
	return etype == "member-join" || etype == "member-leave" ||
		etype == "member-failed" || etype == "member-update" ||
		etype == "member-reap" || etype == "user" ||
		etype == "query" || etype == "*"
}

type scriptEventHandlerManager struct {
	l           sync.Mutex
	handlers    []*ScriptEventHandler
	newHandlers []*ScriptEventHandler
}

func (m *scriptEventHandlerManager) handleEvent(e Event) {
	m.l.Lock()
	if len(m.newHandlers) != 0 {
		m.handlers = m.newHandlers
		m.newHandlers = nil
	}
	handlers := m.handlers
	m.l.Unlock()
	for _, h := range handlers {
		h.HandleEvent(e)
	}
}

// replace the whole handler list!
func (m *scriptEventHandlerManager) update(handlers []*ScriptEventHandler) {
	m.l.Lock()
	defer m.l.Unlock()
	m.newHandlers = handlers
}

type invokeScript struct {
	event  Event
	script string
}

type ScriptEventHandler struct {
	script         string // path to the script to be executed
	filter         *eventFilter
	invokeScriptCh chan *invokeScript
}

func (h *ScriptEventHandler) HandleEvent(e Event) {
	if h.filter.matches(e) {
		h.invokeScriptCh <- &invokeScript{
			event:  e,
			script: h.script,
		}
	}
}

type streamEventHandlerManager struct {
	l           sync.Mutex
	handlerMap  map[*StreamEventHandler]struct{} // for register/deregister
	handlerList []*StreamEventHandler            // for read access
}

func (m *streamEventHandlerManager) handleEvent(e Event) {
	m.l.Lock()
	handlers := m.handlerList
	m.l.Unlock()
	for _, h := range handlers {
		h.HandleEvent(e)
	}
}

func (m *streamEventHandlerManager) register(h *StreamEventHandler) {
	m.l.Lock()
	defer m.l.Unlock()
	m.handlerMap[h] = struct{}{}
	m.handlerList = make([]*StreamEventHandler, 0, len(m.handlerMap))
	for h := range m.handlerMap {
		m.handlerList = append(m.handlerList, h)
	}
}

func (m *streamEventHandlerManager) deregister(h *StreamEventHandler) {
	m.l.Lock()
	defer m.l.Unlock()
	delete(m.handlerMap, h)
	m.handlerList = make([]*StreamEventHandler, 0, len(m.handlerMap))
	for h := range m.handlerMap {
		m.handlerList = append(m.handlerList, h)
	}
}

type StreamEventHandler struct {
}

func (h *StreamEventHandler) HandleEvent(e Event) {}

func nodeToMemberEvent(node *memberlist.NodeEvent) (*MemberEvent, error) {
	var eType EventType
	switch node.Type {
	case memberlist.NodeJoin:
		eType = EventMemberJoin
	case memberlist.NodeLeave:
		eType = EventMemberLeave
	case memberlist.NodeUpdate:
		eType = EventMemberUpdate
	default:
		return nil, fmt.Errorf("unknown member event")
	}
	return &MemberEvent{
		Type: eType,
		Member: &Member{
			ID:   node.Node.ID,
			IP:   node.Node.IP,
			Port: node.Node.Port,
			Tags: node.Node.Tags,
		},
	}, nil
}

// because inEventCh is buffered, so memberlist can start successfully
// even the whole event pipeline is not started
func (s *Serf) receiveNodeEvents() {
	for {
		select {
		case e := <-s.nodeEventCh:
			mEvent, err := nodeToMemberEvent(e)
			if err != nil {
				s.logger.Printf("[ERR] serf: error receiving node event %v", err)
			}
			s.inEventCh <- mEvent
		case <-s.shutdownCh:
			s.logger.Printf("[WARN] serf: serf shutdown, quitting receiving node events")
			return
		}

	}
}

func (s *Serf) receiveEvents() {
	for {
		select {
		case e := <-s.outEventCh:
			s.logger.Printf("[INFO] serf: received event: %s", e.String())
			s.eventHandlers.handleEvent(e)
		case <-s.shutdownCh:
			s.logger.Printf("[WARN] serf: serf shutdown, quitting event reception")
			return
		}
	}
}

func (s *Serf) receiveInvokeScripts() {
	for {
		select {
		case in := <-s.invokeScriptCh:
			s.invokeEventScript(in.script, in.event)
		case <-s.shutdownCh:
			s.logger.Printf("[WARN] serf: serf shutdown, quitting receive invoke scripts")
			return
		}
	}
}

const (
	windows             = "windows"
	maxScriptOutputSize = 8 * 1024
	slowScriptTimeout   = time.Second
)

var invalidShellChars = regexp.MustCompile(`[^A-Z0-9_]`)

// if tags' name has characters not in [A-Z0-9], replace them with "_"
func toValidShellName(name string) string {
	return invalidShellChars.ReplaceAllString(strings.ToUpper(name), "_")
}

func (s *Serf) setupScriptCmd(script string, output *circbuf.Buffer) *exec.Cmd {
	var shell, flag = "/bin/sh", "-c"
	if runtime.GOOS == windows {
		shell = "cmd"
		flag = "/C"
	}

	cmd := exec.Command(shell, flag, script)

	cmd.Stderr = output
	cmd.Stdout = output

	cmd.Env = append(os.Environ(),
		"SERF_SELF_ID="+s.ID(),
		"SERF_SELF_ROLE="+s.tags["role"],
	)

	// add serf_tag_X env
	for name, val := range s.tags {
		tag_env := fmt.Sprintf("SERF_TAG_%s=%s", toValidShellName(name), val)
		cmd.Env = append(cmd.Env, tag_env)
	}
	return cmd
}

func (s *Serf) invokeEventScript(script string, event Event) error {
	// setup script cmd
	output, _ := circbuf.NewBuffer(maxScriptOutputSize)
	cmd := s.setupScriptCmd(script, output)
	cmd.Env = append(cmd.Env, "SERF_EVENT="+event.EventType().String())

	// to send payload to cmd
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	switch e := event.(type) {
	case *CoalescedMemberEvent:
		go stdinMemberData(s.logger, stdin, e)
	case *ActionEvent:
		cmd.Env = append(cmd.Env, "SERF_ACTION_EVENT="+e.Name) // will be read by the script
		cmd.Env = append(cmd.Env, fmt.Sprintf("SERF_ACTION_LTIME=%d", e.LTime))
		go stdinPayload(s.logger, stdin, e.Payload)
	case *QueryEvent:
		cmd.Env = append(cmd.Env, "SERF_QUERY_NAME="+e.Name)
		cmd.Env = append(cmd.Env, fmt.Sprintf("SERF_QUERY_LTIME=%d", e.LTime))
		go stdinPayload(s.logger, stdin, e.Payload)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType().String())
	}

	// Start a timer to warn about slow handlers
	slowTimer := time.AfterFunc(slowScriptTimeout, func() {
		s.logger.Printf("[WARN] agent: Script '%s' slow, execution exceeding %v",
			script, slowScriptTimeout)
	})

	if err := cmd.Start(); err != nil {
		return err
	}

	err = cmd.Wait()
	slowTimer.Stop()

	// Warn if buffer is overritten
	if output.TotalWritten() > output.Size() {
		s.logger.Printf("[WARN] agent: Script '%s' generated %d bytes of output, truncated to %d",
			script, output.TotalWritten(), output.Size())
	}

	s.logger.Printf("[DEBUG] agent: Event '%s' script output: %s, err: %v",
		event.String(), output.String(), err)

	if err != nil {
		return err
	}

	// If this is a query and we have output, respond
	if q, ok := event.(*QueryEvent); ok && output.TotalWritten() > 0 {
		s.respondToQueryEvent(q, output.Bytes())
	}
	return nil
}

func escapeWhiteSpace(s string) string {
	s = strings.Replace(s, "\t", "\\t", -1)
	s = strings.Replace(s, "\n", "\\n", -1)
	return s
}

func decodeAndConcatTags(tags []byte) (map[string]string, string, error) {
	m, err := decodeTags(tags)
	if err != nil {
		return nil, "", err
	}
	kvs := []string{}
	for k, v := range m {
		kvs = append(kvs, fmt.Sprintf("%s=%s", k, v))
	}
	return m, strings.Join(kvs, ","), nil
}

func stdinMemberData(logger *log.Logger, stdin io.WriteCloser, e *CoalescedMemberEvent) {
	defer stdin.Close()
	for _, m := range e.Members {
		tagMap, tags, err := decodeAndConcatTags(m.Tags)
		if err != nil {
			logger.Printf("[ERR] serf invoke-event-script: failed to decode tags %v", err)
			return
		}
		_, err = stdin.Write([]byte(fmt.Sprintf(
			"%s\t%s\t%s\t%s\n",
			m.ID,
			m.IP.String(),
			escapeWhiteSpace(tagMap["role"]),
			escapeWhiteSpace(tags),
		)))
		if err != nil {
			logger.Printf("[ERR] serf invoke-event-script: failed to stream member data to sdin")
			return
		}
	}
}

// send payload so the script can read it
func stdinPayload(logger *log.Logger, stdin io.WriteCloser, buf []byte) {
	defer stdin.Close()

	// Append a newline to payload if missing
	if len(buf) > 0 && buf[len(buf)-1] != '\n' {
		buf = append(buf, '\n')
	}

	if _, err := stdin.Write(buf); err != nil {
		logger.Printf("[ERR] Error writing payload: %s", err)
	}
}

var ErrQueryRespLimitExceed = fmt.Errorf("query response exceed limit")

func (s *Serf) respondToQueryEvent(q *QueryEvent, output []byte) error {
	resp := msgQueryResponse{
		LTime:   q.LTime,
		ID:      q.ID,
		From:    s.mlist.ID(),
		Payload: output,
	}
	msg, err := encode(msgQueryRespType, resp)
	if err != nil {
		s.logger.Printf("[ERR] serf: encode query response message failed")
		return err
	}
	if len(msg) > s.config.QueryResponseSizeLimit {
		s.logger.Printf("[ERR] serf: query response size exceed limit: %d", len(msg))
		return ErrQueryRespLimitExceed
	}
	addr := net.UDPAddr{
		IP:   q.SourceIP,
		Port: int(q.SourcePort),
	}
	err = s.mlist.SendUserMsg(&addr, msg)
	if err != nil {
		s.logger.Printf("[ERR] serf: failed to send query response to %s", addr.String())
		return err
	}

	if err := s.relay(int(q.NumRelays), msg, q.SourceIP, q.SourcePort, q.NodeID); err != nil {
		s.logger.Printf("ERR serf: failed to relay query response to %s:%d", q.SourceIP, q.SourcePort)
		return err
	}
	return nil
}

func parseEventFilter(s string) (*eventFilter, error) {
	f := &eventFilter{eventType: s}
	if strings.HasPrefix(s, "user:") {
		f.name = s[len("user:"):]
		f.eventType = "user"
	} else if strings.HasPrefix(s, "query:") {
		f.name = s[len("query:"):]
		f.eventType = "query"
	}
	if !isValidEventFilter(f) {
		return nil, fmt.Errorf("invalid filter")
	}
	return f, nil
}

func ParseEventFilters(s string) []*eventFilter {
	if s == "" {
		return []*eventFilter{{eventType: "*", name: ""}}
	}
	splits := strings.Split(s, ",")
	filters := make([]*eventFilter, 0, len(splits))
	for _, s := range splits {
		f, err := parseEventFilter(s)
		if err == nil {
			filters = append(filters, f)
		}
	}
	return filters
}

func CreateScriptHandlers(s string, invokeCh chan *invokeScript) []*ScriptEventHandler {
	if s == "" {
		return nil
	}
	var filter, script string
	parts := strings.SplitN(s, "=", 2)
	if len(parts) == 1 {
		script = parts[0]
	} else {
		filter = parts[0]
		script = parts[1]
	}

	filters := ParseEventFilters(filter)
	handlers := make([]*ScriptEventHandler, 0, len(filters))
	for _, f := range filters {
		result := &ScriptEventHandler{
			script:         script,
			filter:         f,
			invokeScriptCh: invokeCh,
		}
		handlers = append(handlers, result)
	}
	return handlers
}
