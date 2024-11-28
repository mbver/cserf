package main

// z to put it at the end
import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	"github.com/mbver/cserf/rpc/server"
	"github.com/mbver/mlist/testaddr"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

var rpcPort int32 = 50050

const (
	certPath   = "./cert.pem"
	keyPath    = "./priv.key"
	scriptname = "eventscript.sh"
	keyfile    = "keyring.json"
)

func nextRpcPort() int {
	return int(atomic.AddInt32(&rpcPort, 1))
}

type testNode struct {
	server  *server.Server
	rpcAddr string
}

func testConfig() (*server.ServerConfig, func(), error) {
	cleanup := func() {}
	conf, err := utils.CreateTestServerConfig()
	if err != nil {
		return nil, cleanup, err
	}
	ip, cleanup := testaddr.BindAddrs.NextAvailAddr()

	conf.MemberlistConfig.BindAddr = ip.String()
	conf.RpcPort = nextRpcPort()
	conf.CertPath = certPath
	conf.KeyPath = keyPath
	conf.LogPrefix = fmt.Sprintf("serf-%s: ", ip.String())
	snapshotPath := fmt.Sprintf("%s.snap", conf.MemberlistConfig.BindAddr)
	cleanup1 := server.CombineCleanup(cleanup, func() { os.Remove(snapshotPath) })
	conf.SerfConfig.SnapshotPath = snapshotPath // skip auto-join
	conf.ClusterName = ""                       // skip auto-join

	conf.SerfConfig.EventScript = fmt.Sprintf("./%s", scriptname)

	conf.SerfConfig.KeyringFile = keyfile

	return conf, cleanup1, err
}

func startServerWithConfig(conf *server.ServerConfig) (*testNode, func(), error) {
	s, cleanup, err := server.CreateServer(conf)
	if err != nil {
		return nil, cleanup, err
	}
	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	return &testNode{s, rpcAddr}, cleanup, nil

}

func startTestServer() (*testNode, func(), error) {
	conf, cleanup, err := testConfig()
	if err != nil {
		return nil, cleanup, err
	}
	node, cleanup1, err := startServerWithConfig(conf)
	cleanup2 := server.CombineCleanup(cleanup1, cleanup)
	if err != nil {
		return nil, cleanup2, err
	}
	return node, cleanup2, nil
}

var commonTestNode *testNode
var commonCluster *threeNodeCluster

func TestMain(m *testing.M) {
	err := utils.CreateTestEventScript("./", scriptname)
	if err != nil {
		panic(err)
	}
	defer os.Remove(scriptname)

	err = utils.CreateTestKeyringFile("./", keyfile, []string{"T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="})
	if err != nil {
		panic(err)
	}
	defer os.Remove(keyfile)

	os.Setenv("SERF_RPC_AUTH", "st@rship")
	defer os.Unsetenv("SERF_RPC_AUTH")

	cmd := CertGenCommand()
	cmd.SetArgs([]string{"./"})
	cmd.Execute()

	defer func() {
		os.Remove(certPath)
		os.Remove(keyPath)
	}()

	node, cleanup, err := startTestServer()
	defer cleanup()
	if err != nil {
		panic(err)
	}
	commonTestNode = node

	cluster, cleanup1, err := threeNodesJoined()
	defer cleanup1()
	if err != nil {
		panic(err)
	}
	commonCluster = cluster
	m.Run()
}

func captureOutput(cmd *cobra.Command) *bytes.Buffer {
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	return out
}

func TestInfo(t *testing.T) {
	cmd := InfoCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out := captureOutput(cmd)
	cmd.Execute()

	addr, err := commonTestNode.server.SerfAddress()
	require.Nil(t, err)

	res := out.String()
	require.Contains(t, res, addr)
	require.Contains(t, res, commonTestNode.server.ID())
	require.Contains(t, res, "node")
	require.Contains(t, res, "stats")
	require.NotContains(t, res, "error")
}

func TestMembers(t *testing.T) {
	cmd := MembersCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	addr, err := commonTestNode.server.SerfAddress()
	require.Nil(t, err)

	t.Run("no tag filters", func(t *testing.T) {
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, addr)
		require.Contains(t, res, commonTestNode.server.ID())
		require.Contains(t, res, "members")
		require.Contains(t, res, "alive")
		require.NotContains(t, res, "error")
	})

	t.Run("no tag filters with matching status", func(t *testing.T) {
		out := captureOutput(cmd)
		cmd.Flags().Set(FlagStatus, "active")
		cmd.Execute()
		res := out.String()
		require.Contains(t, res, addr)
		require.Contains(t, res, commonTestNode.server.ID())
		require.Contains(t, res, "members")
		require.Contains(t, res, "alive")
		require.NotContains(t, res, "error")
	})
	t.Run("with matching tag filter and status", func(t *testing.T) {
		cmd.Flags().Set(FlagTag, "role=some*")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, addr)
		require.Contains(t, res, commonTestNode.server.ID())
		require.Contains(t, res, "members")
		require.Contains(t, res, "alive")
		require.NotContains(t, res, "error")
	})

	t.Run("with non-matching filter", func(t *testing.T) {
		cmd.Flags().Set(FlagTag, "role=otherrole")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, "members: null")
	})

	t.Run("invalid tag filter input", func(t *testing.T) {
		cmd.Flags().Set(FlagTag, "role,")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, "invalid tag filter")
	})

	t.Run("invalid regex", func(t *testing.T) {
		cmd.Flags().Set(FlagTag, "role=[a")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, "error parsing regexp")
	})

	t.Run("multiple tag filters", func(t *testing.T) {
		cmd.Flags().Set(FlagTag, "role=some*,x=y")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, "members: null")
		require.NotContains(t, res, "error")
	})

	t.Run("multiple status filters, 1 matching", func(t *testing.T) {
		cmd.Flags().Set(FlagTag, "")
		cmd.Flags().Set(FlagStatus, "active,left")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, addr)
		require.Contains(t, res, commonTestNode.server.ID())
		require.Contains(t, res, "members")
		require.Contains(t, res, "alive")
		require.NotContains(t, res, "error")
	})

	t.Run("multiple status filters, no matching", func(t *testing.T) {
		cmd.Flags().Set(FlagStatus, "failed,left")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, "members: null")
		require.NotContains(t, res, "error")
	})

	t.Run("invalid status filter", func(t *testing.T) {
		cmd.Flags().Set(FlagStatus, "awesome")
		out := captureOutput(cmd)
		cmd.Execute()

		res := out.String()
		require.Contains(t, res, "error")
		require.Contains(t, res, "invalid status filter")
	})
}

func TestAction(t *testing.T) {
	cmd := ActionCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.SetArgs([]string{"abc", "xyz"})
	out := captureOutput(cmd)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "output: success")
	require.NotContains(t, res, "error")
}

func TestAction_ToFewArgs(t *testing.T) {
	cmd := ActionCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	out := captureOutput(cmd)
	cmd.Execute()

	res := out.String()
	fmt.Println(res)
	require.Contains(t, res, "error")
	require.Contains(t, res, ErrAtLeastOneArg.Error())
}

func TestKey(t *testing.T) {
	node, cleanup, err := startTestServer() // don't mess with the common test server
	defer cleanup()
	require.Nil(t, err)
	oldKey := "T9jncgl9mbLus+baTTa7q7nPSUrXwbDi2dhbtqir37s="
	cmd := KeyCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	t.Run("test-key-list", func(t *testing.T) {
		cmd.SetArgs([]string{"list"})
		out := captureOutput(cmd)
		cmd.Execute()
		res := out.String()
		require.Contains(t, res, "primary_key_count")
		require.Contains(t, res, oldKey)
		require.NotContains(t, res, "error")
	})

	newKey := "HvY8ubRZMgafUOWvrOadwOckVa1wN3QWAo46FVKbVN8="
	t.Run("test-key-use-non-exist", func(t *testing.T) {
		cmd.SetArgs([]string{"use", newKey})
		out := captureOutput(cmd)
		cmd.Execute()
		res := out.String()
		require.Contains(t, res, `"num_err": 1`)
		require.Contains(t, res, "not in keyring")
	})
	t.Run("test-key-install", func(t *testing.T) {
		cmd.SetArgs([]string{"install", newKey})
		out := captureOutput(cmd)
		cmd.Execute()
		res := out.String()
		require.NotContains(t, res, "error")

		cmd.SetArgs([]string{"list"})
		out1 := captureOutput(cmd)
		cmd.Execute()
		res = out1.String()
		require.Contains(t, res, newKey)
		require.Contains(t, res, oldKey)

	})

	t.Run("test-key-remove-primary", func(t *testing.T) {
		cmd.SetArgs([]string{"remove", oldKey})
		out := captureOutput(cmd)
		cmd.Execute()
		res := out.String()
		require.Contains(t, res, "err_from")
		require.Contains(t, res, "removing primary key is not allowed")
	})

	t.Run("test-key-use", func(t *testing.T) {
		cmd.SetArgs([]string{"use", newKey})
		out := captureOutput(cmd)
		cmd.Execute()
		res := out.String()
		require.NotContains(t, "error", res)
	})

	t.Run("test-key-remove-non-primary", func(t *testing.T) {
		cmd.SetArgs(([]string{"remove", oldKey}))
		out := captureOutput(cmd)
		cmd.Execute()
		res := out.String()
		require.NotContains(t, res, "error")

		cmd.SetArgs([]string{"list"})
		out1 := captureOutput(cmd)
		cmd.Execute()
		res = out1.String()
		require.NotContains(t, res, oldKey)
		require.Contains(t, res, newKey)
		require.NotContains(t, res, "error")
	})
}

func TestTags(t *testing.T) {
	node, cleanup, err := startTestServer() // don't mess with the common test server
	defer cleanup()
	require.Nil(t, err)

	cmd := TagsCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.SetArgs([]string{"unset", "role"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "successfully unset tags")
	require.NotContains(t, res, "error")

	cmd = MembersCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.NotContains(t, res, "role")

	cmd = TagsCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.SetArgs([]string{"update", "role=newrole"})

	out = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.Contains(t, res, "successfully update tags")

	cmd = MembersCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.Contains(t, res, "role=newrole")
}

func TestLeave(t *testing.T) {
	node, cleanup, err := startTestServer() // don't mess with the common test server
	defer cleanup()
	require.Nil(t, err)

	cmd := LeaveCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.NotContains(t, res, "error")
	require.Contains(t, res, "leave successfully")

	time.Sleep(100 * time.Millisecond)
	cmd = MembersCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagStatus, "active")

	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.NotContains(t, res, "error")
	require.Contains(t, res, "members: null")
}

type threeNodeCluster struct {
	node1 *testNode
	node2 *testNode
	node3 *testNode
}

func threeNodes() (*threeNodeCluster, func(), error) {
	node1, cleanup, err := startTestServer()
	if err != nil {
		return nil, cleanup, err
	}
	node2, cleanup1, err := startTestServer()
	cleanup2 := server.CombineCleanup(cleanup1, cleanup)
	if err != nil {
		return nil, cleanup2, err
	}
	node3, cleanup3, err := startTestServer()
	cleanup4 := server.CombineCleanup(cleanup3, cleanup2)
	if err != nil {
		return nil, cleanup4, err
	}
	return &threeNodeCluster{node1, node2, node3}, cleanup4, err
}

func threeNodesJoined() (*threeNodeCluster, func(), error) {
	cluster, cleanup, err := threeNodes()
	if err != nil {
		return nil, cleanup, err
	}
	addr2, err := cluster.node2.server.SerfAddress()
	if err != nil {
		return nil, cleanup, err
	}
	addr3, err := cluster.node3.server.SerfAddress()
	if err != nil {
		return nil, cleanup, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := &pb.JoinRequest{
		Addrs: []string{addr2, addr3},
	}
	v, err := cluster.node1.server.Join(ctx, req)
	if err != nil {
		return nil, cleanup, err
	}
	if v.Value != 2 {
		return nil, cleanup, fmt.Errorf("expect 2 nodes joined but got %d", v.Value)
	}
	return cluster, cleanup, err
}

func TestJoin(t *testing.T) {
	cluster, cleanup, err := threeNodes()
	defer cleanup()
	require.Nil(t, err)

	addr2, err := cluster.node2.server.SerfAddress()
	require.Nil(t, err)
	addr3, err := cluster.node3.server.SerfAddress()
	require.Nil(t, err)

	cmd := JoinCommand()
	cmd.Flags().Set(FlagRpcAddr, cluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, "./cert.pem")
	cmd.SetArgs([]string{addr2, addr3})

	out := captureOutput(cmd)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "successful joins")
	require.Contains(t, res, "2/2")
	require.NotContains(t, res, "error")
}

func TestJoin_NoAddr(t *testing.T) {
	cmd := JoinCommand()
	out := captureOutput(cmd)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "error")
	require.Contains(t, res, ErrAtLeastOneArg.Error())
}
func TestQuery(t *testing.T) {
	id1 := commonCluster.node1.server.ID()
	id2 := commonCluster.node2.server.ID()
	id3 := commonCluster.node3.server.ID()

	// no filter
	cmd := QueryCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, id1)
	require.Contains(t, res, id2)
	require.Contains(t, res, id3)
	require.Contains(t, res, "total number of responses: 3")
	require.NotContains(t, res, "error")

	// node filter
	cmd = QueryCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	cmd.Flags().Set(FlagNodeFilter, fmt.Sprintf("%s,%s", id2, id3))
	out1 := bytes.Buffer{}
	cmd.SetOut(&out1)
	cmd.SetErr(&out1)
	cmd.Execute()

	res = out1.String()
	require.NotContains(t, res, id1)
	require.Contains(t, res, id2)
	require.Contains(t, res, id3)
	require.Contains(t, res, "total number of responses: 2")
	require.NotContains(t, res, "error")

	// role filter
	cmd = QueryCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagTag, "role=superstar")
	out2 := bytes.Buffer{}
	cmd.SetOut(&out2)
	cmd.SetErr(&out2)
	cmd.Execute()

	res = out2.String()
	require.NotContains(t, res, "response from")
	require.Contains(t, res, "total number of responses: 0")
	require.NotContains(t, res, "error")

	// role filter
	cmd = QueryCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagTag, "role=something")
	out3 := bytes.Buffer{}
	cmd.SetOut(&out3)
	cmd.SetErr(&out3)
	cmd.Execute()

	res = out3.String()
	require.Contains(t, res, id1)
	require.Contains(t, res, id2)
	require.Contains(t, res, id3)
	require.Contains(t, res, "total number of responses: 3")
	require.NotContains(t, res, "error")

	// role filter
	cmd = QueryCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagTag, "role=something,name=x")
	out4 := bytes.Buffer{}
	cmd.SetOut(&out4)
	cmd.SetErr(&out4)
	cmd.Execute()

	res = out4.String()
	require.NotContains(t, res, "response from")
	require.Contains(t, res, "total number of responses: 0")
	require.NotContains(t, res, "error")
}

func TestReach(t *testing.T) {
	cmd := ReachCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "took")
	require.Contains(t, res, "response counts:")
	require.Contains(t, res, "3/3")
	require.NotContains(t, res, "error")
}

func extractRtt(s string) (time.Duration, error) {
	re := regexp.MustCompile(`rtt:\s*"([^"]+)"`)
	match := re.FindStringSubmatch(s)
	if len(match) > 1 {
		return time.ParseDuration(match[1])
	}
	return 0, fmt.Errorf("RTT not found in input string")
}

func TestRtt(t *testing.T) {
	cmd := RttCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.SetArgs([]string{commonCluster.node2.server.ID()})
	out := captureOutput(cmd)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "µs")
	rtt, err := extractRtt(res)
	require.Nil(t, err)
	require.Greater(t, rtt, 100*time.Microsecond)
	require.Less(t, rtt, 500*time.Microsecond)

	cmd.SetArgs([]string{commonCluster.node2.server.ID(), commonCluster.node3.server.ID()})
	out = captureOutput(cmd)
	cmd.Execute()

	res = out.String()
	require.Contains(t, res, "µs")
	rtt, err = extractRtt(res)
	require.Nil(t, err)
	require.Greater(t, rtt, 100*time.Microsecond)
	require.Less(t, rtt, 500*time.Microsecond)
}

func TestMonitor(t *testing.T) {
	s, cleanup, err := startTestServer()
	defer cleanup()
	require.Nil(t, err)

	var actionPayload = "action_payload"
	var queryPayload = "query_payload"

	aCmd := ActionCommand()
	aCmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	aCmd.Flags().Set(FlagCertPath, certPath)
	aCmd.SetArgs([]string{"action1", actionPayload})

	qCmd := QueryCommand()
	qCmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	qCmd.Flags().Set(FlagCertPath, certPath)
	qCmd.Flags().Set(FlagName, "query1")
	qCmd.Flags().Set(FlagPayload, queryPayload)

	// no filter
	cmd := MonitorCommand()
	cmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	out := captureOutput(cmd)
	go func() {
		cmd.Execute()
	}()

	time.Sleep(10 * time.Millisecond)

	aCmd.Execute()
	qCmd.Execute()

	time.Sleep(25 * time.Millisecond)

	res := out.String()
	require.Contains(t, res, actionPayload)
	require.Contains(t, res, queryPayload)

	// event-filter action
	cmd = MonitorCommand()
	cmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagEventFilter, "action")
	out1 := captureOutput(cmd)
	go func() {
		cmd.Execute()
	}()

	time.Sleep(10 * time.Millisecond)
	aCmd.Execute()
	qCmd.Execute()

	time.Sleep(25 * time.Millisecond)
	res = out1.String()
	require.Contains(t, res, actionPayload)
	require.NotContains(t, res, queryPayload)

	// event-fitler: member-join,member-failed,member-reap
	cmd = MonitorCommand()
	cmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagEventFilter, "member-join,member-failed,member-reap")
	out2 := captureOutput(cmd)
	go func() {
		cmd.Execute()
	}()

	time.Sleep(10 * time.Millisecond)
	s1, cleanup1, err := startTestServer()
	defer cleanup1()
	require.Nil(t, err)
	addr1, err := s1.server.SerfAddress()
	require.Nil(t, err)

	jCmd := JoinCommand()
	jCmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	jCmd.Flags().Set(FlagCertPath, certPath)
	jCmd.SetArgs([]string{addr1})
	jCmd.Execute()
	aCmd.Execute()
	qCmd.Execute()

	time.Sleep(25 * time.Millisecond)

	res = out2.String()
	require.Contains(t, res, "event-type: member-join")
	str := fmt.Sprintf("%s - %s - role=something", s1.server.ID(), addr1)
	require.Contains(t, res, str)
	require.NotContains(t, res, queryPayload)
	require.NotContains(t, res, actionPayload)

	s1.server.Shutdown()
	time.Sleep(50 * time.Millisecond)
	res = out2.String()
	require.Contains(t, res, "event-type: member-failed")
	require.Contains(t, res, "event-type: member-reap")

	// loglevel DEBUG
	cmd = MonitorCommand()
	cmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagLogLevel, "DEBUG")
	out3 := captureOutput(cmd)
	go func() {
		cmd.Execute()
	}()
	time.Sleep(10 * time.Millisecond)
	aCmd.Execute()
	qCmd.Execute()

	time.Sleep(25 * time.Millisecond)

	res = out3.String()
	require.Contains(t, res, "DEBUG")

	// logleve ERR
	cmd = MonitorCommand()
	cmd.Flags().Set(FlagRpcAddr, s.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.Flags().Set(FlagLogLevel, "ERR")
	out4 := captureOutput(cmd)
	go func() {
		cmd.Execute()
	}()
	time.Sleep(10 * time.Millisecond)
	aCmd.Execute()
	qCmd.Execute()

	time.Sleep(25 * time.Millisecond)

	res = out4.String()
	require.NotContains(t, res, "INFO")
}

func TestKeyGen(t *testing.T) {
	cmd := KeyGenCommand()
	out := captureOutput(cmd)
	cmd.Execute()
	res := out.String()
	fmt.Println(res)

	re := regexp.MustCompile(`"([^"]+)"`)
	match := re.FindStringSubmatch(res)
	require.Greater(t, len(match), 1)
	encoded := match[1]
	key, err := base64.StdEncoding.DecodeString(encoded)
	require.Nil(t, err)
	require.Equal(t, 32, len(key))
}
