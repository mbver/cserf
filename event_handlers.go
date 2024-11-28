package serf

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/armon/circbuf"
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
		etype == "member-reap" || etype == "action" ||
		etype == "query" || etype == "*"
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
	eventCh chan Event
	filters []*eventFilter
}

func (h *StreamEventHandler) HandleEvent(e Event) {
	accept := false
	for _, f := range h.filters {
		if f.matches(e) {
			accept = true
			break
		}
	}
	if !accept {
		return
	}
	forwardEvent(e, h.eventCh)
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
		"SERF_LOCAL_ID="+s.ID(),
		"SERF_LOCAL_ROLE="+s.getTag("role"),
	)

	// add serf_tag_X env
	s.tagL.Lock()
	for name, val := range s.tags {
		tag_env := fmt.Sprintf("SERF_TAG_%s=%s", toValidShellName(name), val)
		cmd.Env = append(cmd.Env, tag_env)
	}
	s.tagL.Unlock()
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
	warnSlowScript := time.AfterFunc(slowScriptTimeout, func() {
		s.logger.Printf("[WARN] serf: Script '%s' slow, execution exceeding %v",
			script, slowScriptTimeout)
	})
	if err := cmd.Start(); err != nil {
		return err
	}

	err = cmd.Wait()
	warnSlowScript.Stop()
	// Warn if buffer is overritten
	if output.TotalWritten() > output.Size() {
		s.logger.Printf("[WARN] serf: Script '%s' generated %d bytes of output, truncated to %d",
			script, output.TotalWritten(), output.Size())
	}

	s.logger.Printf("[DEBUG] serf: Event '%s' script output: %s, err: %v",
		event.String(), output.String(), err)

	if err != nil {
		return err
	}
	// If this is a query and we have output, respond
	if q, ok := event.(*QueryEvent); ok && output.TotalWritten() > 0 {
		if err := s.respondToQueryEvent(q, output.Bytes()); err != nil {
			s.logger.Printf("[ERR] serf: failed to respond to query %v", err)
		}
	}
	return nil
}

func escapeWhiteSpace(s string) string {
	s = strings.Replace(s, "\t", "\\t", -1)
	s = strings.Replace(s, "\n", "\\n", -1)
	return s
}

type kvPair struct {
	k, v string
}

func (p *kvPair) String() string {
	return fmt.Sprintf("%s=%s", p.k, p.v)
}

func toKVPairs(m map[string]string) []*kvPair {
	res := make([]*kvPair, 0, len(m))
	for k, v := range m {
		res = append(res, &kvPair{k, v})
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].k < res[j].k
	})
	return res
}

func toKVPairsString(m map[string]string) string {
	kvs := toKVPairs(m)
	strs := make([]string, len(kvs))
	for i, p := range kvs {
		strs[i] = p.String()
	}
	return strings.Join(strs, ",")
}

func decodeAndConcatTags(tags []byte) (string, string, error) {
	m, err := decodeTags(tags)
	if err != nil {
		return "", "", err
	}
	return m["role"], toKVPairsString(m), nil
}

func stdinMemberData(logger *log.Logger, stdin io.WriteCloser, e *CoalescedMemberEvent) {
	defer stdin.Close()
	for _, m := range e.Members {
		role, tags, err := decodeAndConcatTags(m.Tags)
		if err != nil {
			logger.Printf("[ERR] serf invoke-event-script: failed to decode tags %v", err)
			return
		}
		_, err = stdin.Write([]byte(fmt.Sprintf(
			"%s\t%s\t%s\t%s\n",
			m.ID,
			m.IP.String(),
			escapeWhiteSpace(role),
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

func parseEventFilter(s string) (*eventFilter, error) {
	f := &eventFilter{eventType: s}
	if strings.HasPrefix(s, "action:") {
		f.name = s[len("action:"):]
		f.eventType = "action"
	}
	if strings.HasPrefix(s, "query:") {
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

func CreateStreamHandler(eventCh chan Event, filter string) *StreamEventHandler {
	filters := ParseEventFilters(filter)
	return &StreamEventHandler{
		eventCh: eventCh,
		filters: filters,
	}
}
