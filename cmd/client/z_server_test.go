package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/stretchr/testify/require"
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
