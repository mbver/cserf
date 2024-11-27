package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"

	serf "github.com/mbver/cserf"
	memberlist "github.com/mbver/mlist"
	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	RpcAddress       string             `yaml:"rpc_addr"`
	RpcPort          int                `yaml:"rpc_port"`
	LogOutput        string             `yaml:"log_output"`
	LogPrefix        string             `yaml:"log_prefix"`
	LogLevel         string             `yaml:"log_level"`
	SyslogFacility   string             `yaml:"syslog_facility"`
	CertPath         string             `yaml:"cert_path"`
	KeyPath          string             `yaml:"key_path"`
	AuthKeyHash      string             `yaml:"auth_key_hash"`
	ClusterName      string             `yaml:"cluster_name"`
	NetInterface     string             `yaml:"net_interface"` // iface has to be valid or empty
	IgnoreOld        bool               `yaml:"ignore_old"`
	StartJoin        []string           `yaml:"start_join"`
	MemberlistConfig *memberlist.Config `yaml:"memberlist_config"`
	SerfConfig       *serf.Config       `yaml:"serf_config"`
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		RpcAddress:       "0.0.0.0",
		RpcPort:          50051,
		LogOutput:        "",
		LogPrefix:        "",
		LogLevel:         "INFO",
		SyslogFacility:   "",
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
	bindAddr, err := bindIface(conf.NetInterface, conf.MemberlistConfig.BindAddr)
	if err != nil {
		return nil, err
	}
	conf.MemberlistConfig.BindAddr = bindAddr

	return conf, nil
}

func getIface(name string) (*net.Interface, error) {
	if name == "" {
		return nil, nil
	}
	return net.InterfaceByName(name)
}

func extractIP(addr net.Addr) (net.IP, bool) {
	switch addr := addr.(type) {
	case *net.IPAddr:
		return addr.IP, true
	case *net.IPNet:
		return addr.IP, true
	}
	return nil, false
}

func bindIface(name string, addr string) (string, error) {
	iface, _ := getIface(name)
	if iface == nil {
		return addr, nil
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	anyIsFine := addr == "0.0.0.0"
	found := false
	for _, a := range addrs {
		ip, ok := extractIP(a)
		if !ok {
			continue
		}
		if anyIsFine {
			if ip.IsLinkLocalUnicast() {
				continue
			}
			addr = ip.String()
			found = true
			break
		}
		if ip.String() == addr {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("interface %s has no address %s", name, addr)
	}
	return addr, nil
}

func loadKeyring(keyfile string) (*memberlist.Keyring, error) {
	if _, err := os.Stat(keyfile); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}
	encoded := []string{}
	if err := json.Unmarshal([]byte(data), &encoded); err != nil {
		return nil, fmt.Errorf("failed to unmarshal keys data %w", err)
	}
	keys := make([][]byte, len(encoded))
	for i, enc := range encoded {
		key, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, fmt.Errorf("failed to decode key %w", err)
		}
		keys[i] = key
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("required at least 1 key")
	}
	return memberlist.NewKeyring(keys, keys[0])
}
