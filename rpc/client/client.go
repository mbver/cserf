package client

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/mbver/cserf/rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

func (c *Client) Hello(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := c.client.Hello(ctx, &pb.StringValue{Value: name})
	if err != nil {
		return "", err
	}
	return res.Value, nil
}

func (c *Client) HelloStream(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := c.client.HelloStream(ctx, &pb.StringValue{Value: name})
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

func (c *Client) Query(params *pb.QueryParam) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
