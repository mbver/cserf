package client

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/mbver/cserf/rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/durationpb"
)

type Client struct {
	client pb.SerfClient
	conn   *grpc.ClientConn
}

func CreateClient(addr string, creds credentials.TransportCredentials) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return &Client{
		client: pb.NewSerfClient(conn),
		conn:   conn,
	}, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

func defaultCtx() (context.Context, func()) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (c *Client) Query(params *pb.QueryParam) (string, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	stream, err := c.client.Query(ctx, params)
	if err != nil {
		return "", err
	}
	buf := strings.Builder{}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		buf.WriteString(res.Value)
	}
	return buf.String(), nil
}

func (c *Client) Key(command string, key string) (*pb.KeyResponse, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Key(ctx, &pb.KeyRequest{
		Command: command,
		Key:     key,
	})
}

func (c *Client) Action(name string, payload []byte) (*pb.Empty, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Action(ctx, &pb.ActionRequest{
		Name:    name,
		Payload: payload,
	})
}

func (c *Client) Reach() (*pb.ReachResponse, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Reach(ctx, &pb.Empty{})
}

func (c *Client) Active() (*pb.MembersResponse, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Active(ctx, &pb.Empty{})
}

func (c *Client) Members() (*pb.MembersResponse, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Members(ctx, &pb.Empty{})
}

func (c *Client) Join(req *pb.JoinRequest) (*pb.IntValue, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Join(ctx, req)
}

func (c *Client) Leave(req *pb.Empty) (*pb.Empty, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Leave(ctx, req)
}

func (c *Client) Rtt(req *pb.RttRequest) (*durationpb.Duration, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Rtt(ctx, req)
}

func (c *Client) Tag(req *pb.TagRequest) (*pb.Empty, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Tag(ctx, req)
}

func (c *Client) Info(req *pb.Empty) (*pb.Info, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Info(ctx, req)
}
