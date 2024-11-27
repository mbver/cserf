package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStartServer_EventJoin(t *testing.T) {
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
