package serf

import "time"

type Config struct {
	EventScript              string // matching events and script to execute on them
	LBufferSize              int
	RelayFactor              int
	QueryTimeoutMult         int
	QueryResponseSizeLimit   int
	QuerySizeLimit           int // TODO: check it in query
	ActionSizeLimit          int
	SnapshotPath             string
	SnapshotMinCompactSize   int
	SnapshotDrainTimeout     time.Duration
	CoalesceInterval         time.Duration
	KeyringFile              string
	ReapInterval             time.Duration
	ReconnectInterval        time.Duration
	ReconnectTimeout         time.Duration // to remove failed nodes
	TombstoneTimeout         time.Duration // to remove left nodes
	MaxQueueDepth            int
	ManageQueueDepthInterval time.Duration
	DNSConfigPath            string
} // TODO: broadcast timeout should inject to memberlist config
