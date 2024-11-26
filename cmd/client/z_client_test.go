package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

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
	snapshotPath := fmt.Sprintf("%s.snap", conf.MemberlistConfig.BindAddr)
	conf.SerfConfig.SnapshotPath = snapshotPath // skip auto-join
	conf.ClusterName = ""                       // skip auto-join
	cleanup1, err := server.CreateServer(conf)
	cleanup2 := server.CombineCleanup(cleanup1, func() { os.Remove(snapshotPath) }, cleanup)
	if err != nil {
		return "", cleanup2, err
	}
	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	return rpcAddr, cleanup2, nil
}

var commonServerRpc string

func TestMain(m *testing.M) {
	addr, cleanup, err := startTestServer()
	defer cleanup()
	if err != nil {
		panic(err)
	}
	commonServerRpc = addr
	m.Run()
}

func TestInfo(t *testing.T) {
	cmd := InfoCommand()
	cmd.Flags().Set(FlagRpcAddr, commonServerRpc)
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

func TestActive(t *testing.T) {
	cmd := ActiveCommand()
	cmd.Flags().Set(FlagRpcAddr, commonServerRpc)
	cmd.Flags().Set(FlagCertPath, certPath)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "active")
	require.Contains(t, res, "alive")
	require.Contains(t, res, "role=something")
	require.NotContains(t, res, "error")
}

func TestAction(t *testing.T) {
	cmd := ActionCommand()
	cmd.Flags().Set(FlagRpcAddr, commonServerRpc)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.SetArgs([]string{"abc", "xyz"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "output: success")
}

func TestKey(t *testing.T) {
	addr, cleanup, err := startTestServer() // don't mess with the common test server
	defer cleanup()
	require.Nil(t, err)

	cmd := KeyCommand()
	cmd.Flags().Set(FlagRpcAddr, addr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.SetArgs([]string{"list"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	oldKey := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="
	res := out.String()
	require.Contains(t, res, "primary_key_count")
	require.Contains(t, res, oldKey)
	require.NotContains(t, res, "error")

	newKey := "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="
	cmd.SetArgs([]string{"install", newKey})
	cmd.Execute()
	res = out.String()
	require.NotContains(t, res, "error")

	cmd.SetArgs([]string{"list"})
	out = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.Contains(t, res, newKey)
	require.Contains(t, res, oldKey)

	cmd.SetArgs([]string{"remove", oldKey})
	cmd.Execute()

	res = out.String()
	require.Contains(t, res, "err_from")
	require.Contains(t, res, "removing primary key is not allowed")

	cmd.SetArgs([]string{"use", newKey})
	cmd.Execute()
	res = out.String()
	require.NotContains(t, "error", res)

	cmd.SetArgs(([]string{"remove", oldKey}))
	cmd.Execute()

	res = out.String()
	require.NotContains(t, res, "error")

	cmd.SetArgs([]string{"list"})
	out = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.NotContains(t, res, oldKey)
	require.Contains(t, res, newKey)
	require.NotContains(t, res, "error")
}

func TestLeave(t *testing.T) {
	addr, cleanup, err := startTestServer() // don't mess with the common test server
	defer cleanup()
	require.Nil(t, err)

	cmd := LeaveCommand()
	cmd.Flags().Set(FlagRpcAddr, addr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.NotContains(t, res, "error")
	require.Contains(t, res, "leave successfully")

	time.Sleep(100 * time.Millisecond)
	cmd = ActiveCommand()
	cmd.Flags().Set(FlagRpcAddr, addr)
	cmd.Flags().Set(FlagCertPath, certPath)

	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.NotContains(t, res, "error")
	require.Contains(t, res, "active-members: null")
}
