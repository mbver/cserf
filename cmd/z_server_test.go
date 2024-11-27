package main

import (
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/server"
	"github.com/mbver/mlist/testaddr"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestServer_Start_EventJoin(t *testing.T) {
	cmd := MonitorCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	out := captureOutput(cmd)
	go func() {
		cmd.Execute()
	}()
	time.Sleep(200 * time.Millisecond)
	res := out.String()
	require.Contains(t, res, "member-join")
}

func TestServer_ShutdownMultiple(t *testing.T) {
	node, cleanup, err := startTestServer()
	defer cleanup()
	require.Nil(t, err)

	for i := 0; i < 5; i++ {
		node.server.Shutdown()
	}
}

func TestServer_KeyringFile_BadPath(t *testing.T) {
	conf, cleanup, err := testConfig()
	defer cleanup()
	require.Nil(t, err)
	conf.SerfConfig.KeyringFile = "/bad/path"
	_, cleanup1, err := startServerWithConfig(conf)
	defer cleanup1()
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "no such file")
}

func TestServer_KeyringFile_Keysloaded(t *testing.T) {
	conf, cleanup, err := testConfig()
	defer cleanup()
	require.Nil(t, err)
	key1 := "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="
	key2 := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="
	key3 := "5K9OtfP7efFrNKe5WCQvXvnaXJ5cWP0SvXiwe0kkjM4="
	keys := []string{key1, key2, key3}

	keyfile := "keyring_loaded_test.json"
	err = utils.CreateTestKeyringFile("./", keyfile, keys)
	require.Nil(t, err)
	defer os.Remove(keyfile)

	conf.SerfConfig.KeyringFile = keyfile
	node, cleanup1, err := startServerWithConfig(conf)
	defer cleanup1()
	require.Nil(t, err)

	kCmd := KeyCommand()
	kCmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	kCmd.Flags().Set(FlagCertPath, certPath)
	kCmd.SetArgs([]string{"list"})
	out := captureOutput(kCmd)

	kCmd.Execute()
	res := out.String()
	require.Contains(t, res, key1)
	require.Contains(t, res, key2)
	require.Contains(t, res, key3)
}

func TestServer_KeyringFile_NoKey(t *testing.T) {
	conf, cleanup, err := testConfig()
	defer cleanup()
	require.Nil(t, err)

	keyfile := "keyring_no_key_test.json"
	err = utils.CreateTestKeyringFile("./", keyfile, []string{})
	require.Nil(t, err)
	defer os.Remove(keyfile)

	conf.SerfConfig.KeyringFile = keyfile
	_, cleanup1, err := startServerWithConfig(conf)
	defer cleanup1()
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "required at least 1 key")
}

func createConfigFileFromConfig(conf *server.ServerConfig, filename string) error {
	fh, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer fh.Close()
	ybytes, err := yaml.Marshal(conf)
	if err != nil {
		return err
	}
	_, err = fh.Write(ybytes)
	if err != nil {
		return err
	}
	return fh.Close()
}

func TestServer_CommandRun(t *testing.T) {
	configFile := "./server_run_conf.yaml"
	conf, cleanup, err := testConfig()
	defer cleanup()
	require.Nil(t, err)
	createConfigFileFromConfig(conf, configFile)
	defer os.Remove(configFile)

	sCmd := ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile)
	out := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res := out.String()

	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr)
	require.NotContains(t, res, "error")

	mCmd := MembersCommand()
	mCmd.Flags().Set(FlagRpcAddr, rpcAddr)
	mCmd.Flags().Set(FlagCertPath, certPath)
	out1 := captureOutput(mCmd)
	mCmd.Execute()

	res = out1.String()

	bindAddr := net.JoinHostPort(conf.MemberlistConfig.BindAddr, strconv.Itoa(conf.MemberlistConfig.BindPort))
	require.Contains(t, res, bindAddr)
	require.Contains(t, res, "alive")
}

func TestServer_CommandRun_StartJoin(t *testing.T) {
	node1, cleanup, err := startTestServer()
	defer cleanup()
	require.Nil(t, err)

	bindAddr, err := node1.server.SerfAddress()
	require.Nil(t, err)

	configFile := "./start_join_conf.yaml"
	conf, cleanup1, err := testConfig()
	conf.StartJoin = []string{bindAddr}
	defer cleanup1()
	require.Nil(t, err)
	createConfigFileFromConfig(conf, configFile)
	defer os.Remove(configFile)

	sCmd := ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile)
	out := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res := out.String()

	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr)
	require.NotContains(t, res, "error")

	mCmd := MembersCommand()
	mCmd.Flags().Set(FlagRpcAddr, rpcAddr)
	mCmd.Flags().Set(FlagCertPath, certPath)
	out1 := captureOutput(mCmd)
	mCmd.Execute()

	res = out1.String()

	bindAddr1 := net.JoinHostPort(conf.MemberlistConfig.BindAddr, strconv.Itoa(conf.MemberlistConfig.BindPort))
	require.Contains(t, res, bindAddr)
	require.Contains(t, res, bindAddr1)
	require.Contains(t, res, "alive")
}

func TestServer_CommandRun_JoinFail(t *testing.T) {
	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()
	defer cleanup()
	bindAddr := net.JoinHostPort(ip.String(), "7946")

	configFile := "./start_join_failed_conf.yaml"
	conf, cleanup1, err := testConfig()
	defer cleanup1()
	require.Nil(t, err)
	conf.StartJoin = []string{bindAddr}
	createConfigFileFromConfig(conf, configFile)
	defer os.Remove(configFile)

	sCmd := ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile)
	out := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res := out.String()

	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr)
	require.NotContains(t, res, "error")

	mCmd := MembersCommand()
	mCmd.Flags().Set(FlagRpcAddr, rpcAddr)
	mCmd.Flags().Set(FlagCertPath, certPath)
	out1 := captureOutput(mCmd)
	mCmd.Execute()

	res = out1.String()

	bindAddr1 := net.JoinHostPort(conf.MemberlistConfig.BindAddr, strconv.Itoa(conf.MemberlistConfig.BindPort))
	require.NotContains(t, res, bindAddr)
	require.Contains(t, res, bindAddr1)
	require.Contains(t, res, "alive")
}

func TestServer_CommandRun_AdvertiseAddress(t *testing.T) {
	configFile := "./cmd_run_adv_addr_conf.yaml"
	conf, cleanup1, err := testConfig()
	defer cleanup1()
	require.Nil(t, err)
	advAdrr := "172.211.21.21"
	conf.MemberlistConfig.AdvertiseAddr = advAdrr
	createConfigFileFromConfig(conf, configFile)
	defer os.Remove(configFile)
	sCmd := ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile)
	out := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res := out.String()

	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr)
	require.NotContains(t, res, "error")

	mCmd := MembersCommand()
	mCmd.Flags().Set(FlagRpcAddr, rpcAddr)
	mCmd.Flags().Set(FlagCertPath, certPath)
	out1 := captureOutput(mCmd)
	mCmd.Execute()

	res = out1.String()

	require.Contains(t, res, advAdrr)
	require.Contains(t, res, "alive")
}

func TestServer_CommandRun_Mdns(t *testing.T) {
	configFile1 := "./cmd_run_mdns_1_conf.yaml"
	conf, cleanup1, err := testConfig()
	defer cleanup1()
	require.Nil(t, err)
	conf.ClusterName = "sparrow"
	createConfigFileFromConfig(conf, configFile1)
	defer os.Remove(configFile1)
	sCmd := ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile1)
	out1 := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res := out1.String()

	rpcAddr1 := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr1)
	require.NotContains(t, res, "error")

	bindAddr1 := net.JoinHostPort(conf.MemberlistConfig.BindAddr, strconv.Itoa(conf.MemberlistConfig.BindPort))

	configFile2 := "./cmd_run_mdns_2_conf.yaml"
	conf, cleanup2, err := testConfig()
	defer cleanup2()
	require.Nil(t, err)
	conf.ClusterName = "sparrow"
	createConfigFileFromConfig(conf, configFile2)
	defer os.Remove(configFile2)
	sCmd = ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile2)
	out2 := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res = out2.String()

	rpcAddr2 := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr2)
	require.NotContains(t, res, "error")

	bindAddr2 := net.JoinHostPort(conf.MemberlistConfig.BindAddr, strconv.Itoa(conf.MemberlistConfig.BindPort))

	mCmd := MembersCommand()
	mCmd.Flags().Set(FlagRpcAddr, rpcAddr2)
	mCmd.Flags().Set(FlagCertPath, certPath)
	out3 := captureOutput(mCmd)
	mCmd.Execute()

	res = out3.String()
	require.Contains(t, res, bindAddr1)
	require.Contains(t, res, bindAddr2)
}

func TestServer_CommandRun_RetryJoin(t *testing.T) {
	configFile1 := "./cmd_run_retry_1_conf.yaml"
	conf, cleanup1, err := testConfig()
	defer cleanup1()
	require.Nil(t, err)
	createConfigFileFromConfig(conf, configFile1)
	defer os.Remove(configFile1)
	sCmd := ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile1)
	out1 := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res := out1.String()

	rpcAddr1 := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr1)
	require.NotContains(t, res, "error")

	bindAddr1 := net.JoinHostPort(conf.MemberlistConfig.BindAddr, strconv.Itoa(conf.MemberlistConfig.BindPort))

	configFile2 := "./cmd_run_retry_2_conf.yaml"
	conf, cleanup2, err := testConfig()
	defer cleanup2()
	require.Nil(t, err)

	conf.RetryJoins = []string{bindAddr1}
	conf.RetryJoinMax = 1
	conf.RetryJoinInterval = 25 * time.Millisecond

	createConfigFileFromConfig(conf, configFile2)
	defer os.Remove(configFile2)
	sCmd = ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile2)
	out2 := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res = out2.String()

	rpcAddr2 := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	require.Contains(t, res, rpcAddr2)
	require.NotContains(t, res, "error")

	bindAddr2 := net.JoinHostPort(conf.MemberlistConfig.BindAddr, strconv.Itoa(conf.MemberlistConfig.BindPort))

	mCmd := MembersCommand()
	mCmd.Flags().Set(FlagRpcAddr, rpcAddr2)
	mCmd.Flags().Set(FlagCertPath, certPath)
	out3 := captureOutput(mCmd)
	mCmd.Execute()

	res = out3.String()
	require.Contains(t, res, bindAddr1)
	require.Contains(t, res, bindAddr2)
}

func TestServer_CommandRun_RetryJoinFailed(t *testing.T) {
	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()
	defer cleanup()

	bindAddr1 := net.JoinHostPort(ip.String(), "7946")

	configFile2 := "./cmd_run_retry_fail_conf.yaml"
	conf, cleanup2, err := testConfig()
	defer cleanup2()
	require.Nil(t, err)

	conf.RetryJoins = []string{bindAddr1}
	conf.RetryJoinMax = 1
	conf.RetryJoinInterval = 25 * time.Millisecond

	createConfigFileFromConfig(conf, configFile2)
	defer os.Remove(configFile2)
	sCmd := ServerCommand()
	sCmd.Flags().Set(FlagConfig, configFile2)
	out2 := captureOutput(sCmd)
	go func() {
		sCmd.Execute()
	}()
	time.Sleep(100 * time.Millisecond)
	res := out2.String()
	require.Contains(t, res, "failed to rejoin")
	require.Contains(t, res, "exitting")
}
