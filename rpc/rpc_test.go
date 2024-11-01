package rpc

import (
	"fmt"
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

	res, err = c.HelloStream("world")
	require.Nil(t, err)
	for i := 0; i < 3; i++ {
		require.Contains(t, res, fmt.Sprintf("world%d", i))
	}
}
