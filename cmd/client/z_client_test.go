package main

// z to put it at the end
import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
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

type testNode struct {
	server  *server.Server
	rpcAddr string
}

func startTestServer() (*testNode, func(), error) {
	cleanup := func() {}
	err := utils.CreateTestEventScript("./")
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() { os.Remove("./eventscript.sh") }

	conf, err := utils.CreateTestServerConfig()
	if err != nil {
		return nil, cleanup, err
	}
	conf.SerfConfig.EventScript = "./eventscript.sh"

	ip, cleanup1 := testaddr.BindAddrs.NextAvailAddr()
	cleanup2 := server.CombineCleanup(cleanup1, cleanup)

	conf.MemberlistConfig.BindAddr = ip.String()
	conf.RpcPort = nextRpcPort()
	conf.CertPath = certPath
	conf.KeyPath = keyPath
	conf.LogPrefix = fmt.Sprintf("serf-%s: ", ip.String())
	snapshotPath := fmt.Sprintf("%s.snap", conf.MemberlistConfig.BindAddr)
	conf.SerfConfig.SnapshotPath = snapshotPath // skip auto-join
	conf.ClusterName = ""                       // skip auto-join

	s, cleanup3, err := server.CreateServer(conf)
	cleanup4 := server.CombineCleanup(cleanup3, func() { os.Remove(snapshotPath) }, cleanup2)
	if err != nil {
		return nil, cleanup4, err
	}
	rpcAddr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	return &testNode{s, rpcAddr}, cleanup4, nil
}

var commonTestNode *testNode
var commonCluster *threeNodeCluster

func TestMain(m *testing.M) {
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

func TestInfo(t *testing.T) {
	cmd := InfoCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
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

func TestActive(t *testing.T) {
	cmd := ActiveCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	addr, err := commonTestNode.server.SerfAddress()
	require.Nil(t, err)

	res := out.String()
	require.Contains(t, res, addr)
	require.Contains(t, res, commonTestNode.server.ID())
	require.Contains(t, res, "active")
	require.Contains(t, res, "alive")
	require.Contains(t, res, "role=something")
	require.NotContains(t, res, "error")
}

func TestMembers(t *testing.T) {
	cmd := MembersCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	addr, err := commonTestNode.server.SerfAddress()
	require.Nil(t, err)

	res := out.String()
	require.Contains(t, res, addr)
	require.Contains(t, res, commonTestNode.server.ID())
	require.Contains(t, res, "members")
	require.Contains(t, res, "alive")
	require.NotContains(t, res, "error")
}

func TestAction(t *testing.T) {
	cmd := ActionCommand()
	cmd.Flags().Set(FlagRpcAddr, commonTestNode.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)
	cmd.SetArgs([]string{"abc", "xyz"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "output: success")
	require.NotContains(t, res, "error")
}

func TestKey(t *testing.T) {
	node, cleanup, err := startTestServer() // don't mess with the common test server
	defer cleanup()
	require.Nil(t, err)

	cmd := KeyCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
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
	cmd = ActiveCommand()
	cmd.Flags().Set(FlagRpcAddr, node.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res = out.String()
	require.NotContains(t, res, "error")
	require.Contains(t, res, "active-members: null")
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

	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "successful joins")
	require.Contains(t, res, "2/2")
	require.NotContains(t, res, "error")
}

func TestQuery(t *testing.T) {
	cmd := QueryCommand()
	cmd.Flags().Set(FlagRpcAddr, commonCluster.node1.rpcAddr)
	cmd.Flags().Set(FlagCertPath, certPath)

	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, commonCluster.node1.server.ID())
	require.Contains(t, res, commonCluster.node2.server.ID())
	require.Contains(t, res, commonCluster.node3.server.ID())
	require.Contains(t, res, "total number of responses: 3")
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
