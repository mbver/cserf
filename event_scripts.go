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
	"time"

	"github.com/armon/circbuf"
)

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
	case *QueryEvent:
		cmd.Env = append(cmd.Env, "SERF_QUERY_NAME="+e.Name)
		cmd.Env = append(cmd.Env, fmt.Sprintf("SERF_QUERY_LTIME=%d", e.LTime))
		go streamPayload(s.logger, stdin, e.Payload)
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

// send payload so the script can read it
func streamPayload(logger *log.Logger, stdin io.WriteCloser, buf []byte) {
	defer stdin.Close()

	// Append a newline to payload if missing
	payload := buf
	if len(payload) > 0 && payload[len(payload)-1] != '\n' {
		payload = append(payload, '\n')
	}

	if _, err := stdin.Write(payload); err != nil {
		logger.Printf("[ERR] Error writing payload: %s", err)
		return
	}
}

func (s *Serf) respondToQueryEvent(q *QueryEvent, output []byte) {
	resp := msgQueryResponse{
		LTime:   q.LTime,
		ID:      q.ID,
		From:    s.mlist.ID(),
		Payload: output,
	}
	msg, err := encode(msgQueryRespType, resp)
	if err != nil {
		s.logger.Printf("[ERR] serf: encode query response message failed")
	}
	addr := net.UDPAddr{
		IP:   q.SourceIP,
		Port: int(q.SourcePort),
	}
	err = s.mlist.SendUserMsg(&addr, msg)
	if err != nil {
		s.logger.Printf("[ERR] serf: failed to send query response to %s", addr.String())
	}

	if err := s.relay(int(q.NumRelays), msg, q.SourceIP, q.SourcePort, q.NodeID); err != nil {
		s.logger.Printf("ERR serf: failed to relay query response to %s:%d", q.SourceIP, q.SourcePort)
	}
}
