package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"github.com/mbver/cserf/rpc/pb"
	"github.com/mbver/cserf/rpc/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/durationpb"
)

// TODO: a logger then?
type Client struct {
	client pb.SerfClient
	conn   *grpc.ClientConn
}

func getClientCredentials(certPath string) (credentials.TransportCredentials, error) {
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(cert)
	creds := credentials.NewTLS(&tls.Config{
		RootCAs: certPool,
	})
	return creds, nil
}

func CreateClient(addr string, certPath string) (*Client, error) {
	creds, err := getClientCredentials(certPath)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	client := &Client{
		client: pb.NewSerfClient(conn),
		conn:   conn,
	}
	_, err = client.Connect(&pb.Empty{})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

func defaultCtx() (context.Context, func()) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func (c *Client) Connect(req *pb.Empty) (*pb.Empty, error) {
	ctx, cancel := defaultCtx()
	defer cancel()
	return c.client.Connect(ctx, req)
}

func (c *Client) Query(params *pb.QueryParam) (chan string, func(), error) {
	ctx, cancel := defaultCtx()
	respCh := make(chan string, 1024)
	stream, err := c.client.Query(ctx, params)
	if err != nil {
		return nil, cancel, err
	}

	go func() {
		defer close(respCh)
		for {
			res, err := stream.Recv()
			if err != nil {
				if server.ShouldStopStreaming(err) {
					return
				}
				continue
			}
			respCh <- res.Value
		}
	}()
	return respCh, cancel, nil
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

func (c *Client) Monitor(req *pb.MonitorRequest) (pb.Serf_MonitorClient, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := c.client.Monitor(ctx, req)
	if err != nil {
		return nil, cancel, err
	}
	return stream, cancel, nil
}
