package server

// to nest memberlist config and serf config
// and see how they include default config
type ServerConfig struct {
	RpcAddress string
	RpcPort    int
	BindAddr   string
	BindPort   int
	LogOutput  string
	Syslog     string
	CertPath   string
	KeyPath    string
	Logoutput  string
	LogPrefix  string
	// TODO: keys, keyring, auth?
}
