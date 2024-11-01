package rpc

import (
	"testing"

	"github.com/mbver/cserf/rpc/client"
	"github.com/mbver/cserf/rpc/server"
	"github.com/stretchr/testify/require"
)

func TestRPC_Hello(t *testing.T) {
	addr := "localhost:50051"
	s, err := server.CreateServer(addr)
	require.Nil(t, err)
	defer s.Stop()
	c, err := client.CreateClient(addr)
	require.Nil(t, err)
	defer c.Close()

	res, err := c.Hello("world")
	require.Nil(t, err)
	require.Contains(t, res, "world")
}
