package main

import (
	"bytes"
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
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
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
	out := bytes.Buffer{}
	kCmd.SetOut(&out)
	kCmd.SetErr(&out)

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
	out := bytes.Buffer{}
	sCmd.SetOut(&out)
	sCmd.SetErr(&out)
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
	out := bytes.Buffer{}
	sCmd.SetOut(&out)
	sCmd.SetErr(&out)
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
	out := bytes.Buffer{}
	sCmd.SetOut(&out)
	sCmd.SetErr(&out)
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
	out := bytes.Buffer{}
	sCmd.SetOut(&out)
	sCmd.SetErr(&out)
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
