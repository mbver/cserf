package serf

type Config struct {
	QueryBufferSize  int
	RelayFactor      int
	QueryTimeoutMult int
} // TODO: broadcast timeout should inject to memberlist config
