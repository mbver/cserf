package utils

import (
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ToTagMap(s string) map[string]string {
	m := map[string]string{}
	kvs := strings.Split(s, ",")
	for _, kv := range kvs {
		pair := strings.Split(kv, "=")
		if len(pair) < 2 {
			continue
		}
		m[pair[0]] = pair[1]
	}
	return m
}

func ToNodes(s string) []string {
	if len(s) == 0 {
		return nil
	}
	return strings.Split(s, ",")
}

func WaitForTerm(stop chan struct{}) {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	select {
	case <-sigCh:
		return
	case <-stop:
		return

	}
}

func ShouldStopStreaming(err error) bool {
	st, ok := status.FromError(err)
	if ok {
		if st.Code() == codes.Unavailable {
			return true
		}
	}
	if err == io.EOF {
		return true
	}
	return false
}
