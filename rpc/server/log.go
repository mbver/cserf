package server

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/hashicorp/logutils"
)

type logStreamer struct {
	logCh  chan string
	filter *logutils.LevelFilter // BAD
	logger *log.Logger
}

func newLogStreamer(ch chan string, levelStr string, logger *log.Logger) (*logStreamer, error) {
	level := logutils.LogLevel(strings.ToUpper(levelStr))
	if !isValidLevel(level) {
		return nil, fmt.Errorf("invalid log level")
	}
	filter := &logutils.LevelFilter{
		Levels:   LogLevels,
		MinLevel: level,
	}
	return &logStreamer{
		logCh:  ch,
		filter: filter,
		logger: logger,
	}, nil
}

func (s *logStreamer) Stream(l string) {
	if !s.filter.Check([]byte(l)) { // TODO: SO BAD
		return
	}
	select {
	case s.logCh <- l:
	default:
		go s.logger.Printf("[WARN] skipping log while streaming")
	}
}

// protected by manager's lock
type ringLogBuf struct {
	idx int
	buf []string
}

// store new log
func (r *ringLogBuf) write(l string) {
	r.buf[r.idx] = l
	r.idx = (r.idx + 1) % len(r.buf)
}

// send old logs
func (r *ringLogBuf) dump(s *logStreamer) {
	for i := 0; i < len(r.buf); i++ {
		j := (r.idx + i) % len(r.buf)
		if r.buf[j] == "" {
			continue
		}
		s.Stream(r.buf[j])
	}
}

type logStreamManager struct {
	l           sync.Mutex
	streamerMap map[*logStreamer]struct{}
	buf         *ringLogBuf
}

func newLogStreamManager() *logStreamManager {
	return &logStreamManager{
		streamerMap: map[*logStreamer]struct{}{},
		buf: &ringLogBuf{
			buf: make([]string, 512),
		},
	}
}

func (m *logStreamManager) register(s *logStreamer) {
	m.l.Lock()
	defer m.l.Unlock()
	m.streamerMap[s] = struct{}{}
	m.buf.dump(s)
}

func (m *logStreamManager) deregister(s *logStreamer) {
	m.l.Lock()
	defer m.l.Unlock()
	delete(m.streamerMap, s)
}

func (m *logStreamManager) Write(p []byte) (n int, err error) {
	m.l.Lock()
	defer m.l.Unlock()

	n = len(p)
	if p[n-1] == '\n' {
		p = p[:n-1]
	}
	str := string(p)
	m.buf.write(str)
	for s := range m.streamerMap {
		s.Stream(str)
	}
	return
}

var syslogPriorityMap = map[string]gsyslog.Priority{
	"TRACE": gsyslog.LOG_DEBUG,
	"DEBUG": gsyslog.LOG_INFO,
	"INFO":  gsyslog.LOG_NOTICE,
	"WARN":  gsyslog.LOG_WARNING,
	"ERR":   gsyslog.LOG_ERR,
	"CRIT":  gsyslog.LOG_CRIT,
}

type Syslog struct {
	logger gsyslog.Syslogger
}

func (s *Syslog) Write(p []byte) (int, error) {
	// Extract log level
	var level string
	afterLevel := p
	x := bytes.IndexByte(p, '[')
	if x >= 0 {
		y := bytes.IndexByte(p[x:], ']')
		if y >= 0 {
			level = string(p[x+1 : x+y])
			afterLevel = p[x+y+2:]
		}
	}

	// Each log level will be handled by a specific syslog priority
	priority, ok := syslogPriorityMap[level]
	if !ok {
		priority = gsyslog.LOG_NOTICE
	}

	// Attempt the write
	err := s.logger.WriteLevel(priority, afterLevel)
	return len(p), err
}

const (
	LogOutputStdout = "stdout"
	LogOutputStderr = "stderr"
)

func getLogOutput(s string) io.Writer {
	switch s {
	case LogOutputStdout:
		return os.Stdout
	}
	return os.Stderr
}

func createLogger(
	outputConfig string,
	streamer *logStreamManager,
	prefix string,
	levelStr string,
	syslogFacility string,
) (*log.Logger, error) {
	level := logutils.LogLevel(strings.ToUpper(levelStr))
	if !isValidLevel(level) {
		return nil, fmt.Errorf("invalid level %s", level)
	}

	output := getLogOutput(outputConfig)
	filterOutput := &logutils.LevelFilter{
		Levels:   LogLevels,
		MinLevel: level,
		Writer:   output,
	}

	fanOutOutput := io.MultiWriter(filterOutput, streamer)

	if syslogFacility != "" {
		logger, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, syslogFacility, "serf")
		if err != nil {
			return nil, err
		}
		syslog := &Syslog{logger}
		filterSyslog := &logutils.LevelFilter{
			Levels:   LogLevels,
			MinLevel: level,
			Writer:   syslog,
		}
		fanOutOutput = io.MultiWriter(filterOutput, filterSyslog, streamer)
	}

	// use a slice of io.Writer and spread it out later
	return log.New(fanOutOutput, prefix, log.LstdFlags), nil
}

var LogLevels = []logutils.LogLevel{"TRACE", "DEBUG", "INFO", "WARN", "ERR"}

func isValidLevel(l0 logutils.LogLevel) bool {
	for _, l := range LogLevels {
		if l == l0 {
			return true
		}
	}
	return false
}
