package server

import (
	"context"
	"fmt"
	"net"

	serf "github.com/mbver/cserf"
	"github.com/mbver/cserf/rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Server struct {
	pb.UnimplementedSerfServer
	serf *serf.Serf
}

func CreateServer(addr string, cert credentials.TransportCredentials, serf *serf.Serf) (*grpc.Server, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := grpc.NewServer(grpc.Creds(cert))
	pb.RegisterSerfServer(s, &Server{
		serf: serf,
	})

	go func() {
		if err := s.Serve(l); err != nil {
			fmt.Printf("[ERR] grpc server: %v", err)
		}
	}()
	return s, nil
}

func (s *Server) Hello(ctx context.Context, name *pb.StringValue) (*pb.StringValue, error) {
	return &pb.StringValue{
		Value: fmt.Sprintf("Hallo doch %s", name),
	}, nil
}

func (s *Server) HelloStream(name *pb.StringValue, stream pb.Serf_HelloStreamServer) error {
	for i := 0; i < 3; i++ {
		if err := stream.Send(&pb.StringValue{
			Value: fmt.Sprintf("hello%s%d,", name.Value, i),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Query(e *pb.Empty, stream pb.Serf_QueryServer) error {
	respCh := make(chan string)
	s.serf.Query(respCh)
	for res := range respCh {
		stream.Send(&pb.StringValue{
			Value: res,
		})
	}
	return nil
}
