package serf

type Config struct {
	EventScript      string // matching events and script to execute on them
	QueryBufferSize  int
	RelayFactor      int
	QueryTimeoutMult int
} // TODO: broadcast timeout should inject to memberlist config
