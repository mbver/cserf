package serf

import "time"

type Config struct {
	EventScript            string // matching events and script to execute on them
	LBufferSize            int
	RelayFactor            int
	QueryTimeoutMult       int
	SnapshotPath           string
	SnapshotMinCompactSize int
	CoalesceInterval       time.Duration
} // TODO: broadcast timeout should inject to memberlist config
