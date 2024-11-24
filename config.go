package serf

import "time"

type Config struct {
	EventScript              string            `yaml:"event_script"` // matching events and script to execute on them
	LBufferSize              int               `yaml:"l_buffer_size"`
	RelayFactor              int               `yaml:"relay_factor"`
	QueryTimeoutMult         int               `yaml:"query_timeout_mult"`
	QueryResponseSizeLimit   int               `yaml:"query_response_size_limit"`
	QuerySizeLimit           int               `yaml:"query_size_limit"`
	ActionSizeLimit          int               `yaml:"action_size_limit"`
	SnapshotPath             string            `yaml:"snapshot_path"`
	SnapshotMinCompactSize   int               `yaml:"snapshot_min_compact_size"`
	SnapshotDrainTimeout     time.Duration     `yaml:"snapshot_drain_timeout"`
	CoalesceInterval         time.Duration     `yaml:"coalesce_interval"`
	KeyringFile              string            `yaml:"keyring_file"`
	ReapInterval             time.Duration     `yaml:"reap_interval"`
	ReconnectInterval        time.Duration     `yaml:"reconnect_interval"`
	ReconnectTimeout         time.Duration     `yaml:"reconnect_timeout"` // to remove failed nodes
	TombstoneTimeout         time.Duration     `yaml:"tombstone_timeout"` // to remove left nodes
	MaxQueueDepth            int               `yaml:"max_queue_depth"`
	MinQueueDepth            int               `yaml:"min_queue_depth"`
	ManageQueueDepthInterval time.Duration     `yaml:"manage_queue_depth_interval"`
	DNSConfigPath            string            `yaml:"dns_config_path"`
	Tags                     map[string]string `yaml:"tags"`
} // TODO: broadcast timeout should inject to memberlist config

func DefaultConfig() *Config {
	return &Config{
		LBufferSize:            512,
		QueryTimeoutMult:       16,
		QueryResponseSizeLimit: 1024,
		QuerySizeLimit:         1024,
		ActionSizeLimit:        512,
		SnapshotPath:           "./serf.snap",
		SnapshotMinCompactSize: 128 * 1024,
		SnapshotDrainTimeout:   500 * time.Millisecond,
		CoalesceInterval:       1 * time.Second,
		// KEYRING FILE? DEFAULT? NO? OR FLAG?
		ReapInterval:             15 * time.Second,
		ReconnectInterval:        30 * time.Second,
		ReconnectTimeout:         24 * time.Hour,
		TombstoneTimeout:         24 * time.Hour,
		MaxQueueDepth:            4096,
		ManageQueueDepthInterval: 30 * time.Second,
		Tags:                     map[string]string{},
		// add a warning queue depth?
	}
}
