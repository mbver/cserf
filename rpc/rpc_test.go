package rpc

import (
	"fmt"
	"testing"

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

	_, err = client.Hello("world")
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "connection error")
}

func TestRPC_Hello(t *testing.T) {
	client, _, cleanup, err := testutils.ClientServerRPCFromSerf(nil)
	defer cleanup()
	require.Nil(t, err)

	res, err := client.Hello("world")
	require.Nil(t, err)
	require.Contains(t, res, "world")

	res, err = client.HelloStream("world")
	require.Nil(t, err)
	for i := 0; i < 3; i++ {
		require.Contains(t, res, fmt.Sprintf("world%d", i))
	}
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
