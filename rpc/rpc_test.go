package rpc

import (
	"fmt"
	"testing"

	"github.com/mbver/cserf/rpc/pb"
	"github.com/mbver/cserf/testutils"
	"github.com/stretchr/testify/require"
)

var cert1, key1, cert2, key2 string

func TestMain(m *testing.M) {
	var err error
	var cleanup1, cleanup2 func()

	cert1, key1, cleanup1, err = testutils.GenerateSelfSignedCert()
	defer cleanup1()
	if err != nil {
		panic(err)
	}

	cert2, key2, cleanup2, err = testutils.GenerateSelfSignedCert()
	defer cleanup2()
	if err != nil {
		panic(err)
	}

	m.Run()
}

// TODO: test fails something: other errors, including deadline exceeded
func TestRPC_MismatchedCerts(t *testing.T) {
	addr := fmt.Sprintf("localhost:%d", testutils.NextRpcPort())

	server, err := testutils.PrepareRPCServer(addr, cert1, key1, nil)
	require.Nil(t, err)
	defer server.Stop()

	client, err := testutils.PrepareRPCClient(addr, cert2)
	require.Nil(t, err)
	defer client.Close()

	_, err = client.Info(&pb.Empty{})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "connection error")
}

func TestRPC_Query(t *testing.T) {
	s1, s2, s3, cleanup, err := testutils.ThreeNodesJoined(nil, nil, nil)
	defer cleanup()
	require.Nil(t, err)

	client, _, cleanup1, err := testutils.ClientServerRPCFromSerf(s1)
	defer cleanup1()
	require.Nil(t, err)

	res, err := client.Query(nil)
	require.Nil(t, err)
	require.Contains(t, res, s1.ID())
	require.Contains(t, res, s2.ID())
	require.Contains(t, res, s3.ID())
}
