package main

// z to put it at the end
import (
	"bytes"
	"context"
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
	conf.SerfConfig.SnapshotPath = snapshotPath // skip auto-join
	conf.ClusterName = ""                       // skip auto-join

	scriptname := fmt.Sprintf("%s_eventscript.sh", ip.String())
	err = utils.CreateTestEventScript("./", scriptname)
	if err != nil {
		return nil, cleanup, err
	}
	cleanup1 := func() { os.Remove(scriptname) }
	conf.SerfConfig.EventScript = fmt.Sprintf("./%s", scriptname)
	cleanup2 := server.CombineCleanup(cleanup1, cleanup)

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
	fmt.Println(res)
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
	out := bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Execute()

	res := out.String()
	require.Contains(t, res, "µs")
	rtt, err := extractRtt(res)
	require.Nil(t, err)
	require.Greater(t, rtt, 100*time.Microsecond)
	require.Less(t, rtt, 500*time.Microsecond)

	cmd.SetArgs([]string{commonCluster.node2.server.ID(), commonCluster.node3.server.ID()})
	out = bytes.Buffer{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
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
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
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
	out1 := bytes.Buffer{}
	cmd.SetOut(&out1)
	cmd.SetErr(&out1)
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
	out2 := bytes.Buffer{}
	cmd.SetOut(&out2)
	cmd.SetErr(&out2)
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
	out3 := bytes.Buffer{}
	cmd.SetOut(&out3)
	cmd.SetErr(&out3)
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
	out4 := bytes.Buffer{}
	cmd.SetOut(&out4)
	cmd.SetErr(&out4)
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
