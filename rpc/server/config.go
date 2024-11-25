package server

import (
	"fmt"
	"net"
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
	LogLevel         string             `yaml:"log_level"`
	SyslogFacility   string             `yaml:"syslog_facility"`
	CertPath         string             `yaml:"cert_path"`
	KeyPath          string             `yaml:"key_path"`
	EncryptKey       string             `yaml:"encrypt_key"`
	AuthKey          string             `yaml:"auth_key"`
	ClusterName      string             `yaml:"cluster_name"`
	NetInterface     string             `yaml:"net_interface"` // iface has to be valid or empty
	IgnoreOld        bool               `yaml:"ignore_old"`
	MemberlistConfig *memberlist.Config `yaml:"memberlist_config"`
	SerfConfig       *serf.Config       `yaml:"serf_config"`
	// TODO: auth?
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

func bindIface(name string, addr string) (string, error) {
	iface, _ := getIface(name)
	if iface == nil {
		return addr, nil
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	if addr != "0.0.0.0" {
		found := false
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ipNet.IP.String() == addr {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("interface %s has no address %s", name, addr)
		}
		return addr, nil
	}
	found := false
	var ip net.IP
	for _, a := range addrs {
		switch addr := a.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}
		if ip.IsLinkLocalUnicast() {
			continue
		}
		found = true
		break
	}
	if !found {
		return addr, fmt.Errorf("no address for 0.0.0.0 on iface %s", name)
	}
	return ip.String(), nil
}
