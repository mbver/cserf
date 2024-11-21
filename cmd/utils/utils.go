package utils

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
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

func WaitForTerm() {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	<-sigCh
}
