package server

import (
	"os"

	serf "github.com/mbver/cserf"
	memberlist "github.com/mbver/mlist"
	"gopkg.in/yaml.v3"
)

// to nest memberlist config and serf config
// and see how they include default config
type ServerConfig struct {
	RpcAddress       string             `yaml:"rpc_addr"`
	RpcPort          int                `yaml:"rpc_port"`
	LogOutput        string             `yaml:"log_output"`
	LogPrefix        string             `yaml:"log_prefix"`
	Syslog           string             `yaml:"syslog"`
	CertPath         string             `yaml:"cert_path"`
	KeyPath          string             `yaml:"key_path"`
	MemberlistConfig *memberlist.Config `yaml:"memberlist_config"`
	SerfConfig       *serf.Config       `yaml:"serf_config"`
	// TODO: keys, keyring, auth?
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		RpcAddress:       "0.0.0.0",
		RpcPort:          50051,
		LogOutput:        "",
		LogPrefix:        "",
		Syslog:           "",
		CertPath:         "./cert.pem",
		KeyPath:          "./priv.key",
		MemberlistConfig: memberlist.DefaultLANConfig(),
		SerfConfig:       serf.DefaultConfig(),
	}
}

func LoadConfig(path string) (*ServerConfig, error) {
	ybytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	conf := &ServerConfig{}
	err = yaml.Unmarshal(ybytes, conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}
