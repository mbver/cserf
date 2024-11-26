package main

import (
	"bytes"
	"net"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/server"
	"github.com/mbver/mlist/testaddr"
	"github.com/stretchr/testify/require"
)

var rpcPort int32 = 50050

const (
	certPath = "./cert.pem"
	keyPath  = "./priv.key"
)

func nextRpcPort() int {
	return int(atomic.AddInt32(&rpcPort, 1))
}

func startTestServer() (string, func(), error) {
	cleanup := func() {}
	conf, err := utils.CreateTestServerConfig()
	if err != nil {
		return "", cleanup, err
	}
	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()
	conf.MemberlistConfig.BindAddr = ip.String()
	conf.RpcPort = nextRpcPort()
	conf.CertPath = certPath
	conf.KeyPath = keyPath
	cleanup1, err := server.CreateServer(conf)
	cleanup2 := server.CombineCleanup(cleanup, cleanup1)
	if err != nil {
		return "", cleanup2, err
	}
	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	return rpcAddr, cleanup2, nil
}

func TestInfo(t *testing.T) {
	addr, cleanup, err := startTestServer()
	defer cleanup()
	require.Nil(t, err)
	cmd := InfoCommand()
	cmd.Flags().Set(FlagRpcAddr, addr)
	cmd.Flags().Set(FlagCertPath, certPath)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "node")
	require.Contains(t, res, "stats")
	require.NotContains(t, res, "error")
}
