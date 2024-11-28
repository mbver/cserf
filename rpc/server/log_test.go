package server

import (
	"log"
	"os"
	"runtime"
	"testing"

	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/hashicorp/logutils"
	"github.com/stretchr/testify/require"
)

func TestLogStreamer(t *testing.T) {
	ch := make(chan string, 10)
	logger := log.New(os.Stdout, "test-log-streamer: ", log.LstdFlags)
	s, err := newLogStreamer(ch, "INFO", logger)
	require.Nil(t, err)

	l1 := "[DEBUG] this is a test log"
	l2 := "[INFO] This should pass"

	s.Stream(l1)
	s.Stream(l2)

	require.Equal(t, 1, len(ch))
	l := <-ch
	require.Equal(t, l2, l)
}

func TestFilterSyslog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	l, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, "LOCAL0", "serf")
	require.Nil(t, err)
	f := FilterSyslog{
		filter: &logutils.LevelFilter{
			Levels:   LogLevels,
			MinLevel: "INFO",
		},
		logger: l,
	}
	n, err := f.Write([]byte("[INFO] test"))
	require.Nil(t, err)
	require.NotZero(t, n)

	n, err = f.Write([]byte("[DEBUG] test"))
	require.Nil(t, err)
	require.Zero(t, n)
}
