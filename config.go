package serf

type Config struct {
	QueryBufferSize int
	RelayFactor     int
} // TODO: broadcast timeout should inject to memberlist config
