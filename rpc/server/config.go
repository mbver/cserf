package server

import (
	"os"

	"sigs.k8s.io/yaml"
)

// to nest memberlist config and serf config
// and see how they include default config
type ServerConfig struct {
	RpcAddress string `yaml:"rpc_addr"`
	RpcPort    int    `yaml:"rpc_port"`
	BindAddr   string `yaml:"bind_addr`
	BindPort   int    `yaml:"bind_port"`
	LogOutput  string `yaml:"log_output"`
	LogPrefix  string `yaml:"log_prefix"`
	Syslog     string `yaml:"syslog"`
	CertPath   string `yaml:"cert_path"`
	KeyPath    string `yaml:"key_path"`
	// TODO: keys, keyring, auth?
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		RpcAddress: "127.0.0.1",
		RpcPort:    50051,
		BindAddr:   "0.0.0.0",
		BindPort:   7946,
		LogOutput:  "",
		LogPrefix:  "",
		Syslog:     "",
		CertPath:   "./server.cert",
		KeyPath:    "./key.key",
	}
}

func LoadConfig(path string) (*ServerConfig, error) {
	ybytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	conf := &ServerConfig{}
	err = yaml.UnmarshalStrict(ybytes, conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}
