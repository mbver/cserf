package serf

import "time"

type Config struct {
	EventScript            string // matching events and script to execute on them
	LBufferSize            int
	RelayFactor            int
	QueryTimeoutMult       int
	QueryResponseSizeLimit int
	SnapshotPath           string
	SnapshotMinCompactSize int
	SnapshotDrainTimeout   time.Duration
	CoalesceInterval       time.Duration
	KeyringFile            string
} // TODO: broadcast timeout should inject to memberlist config
