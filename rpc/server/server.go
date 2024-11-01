package server

import (
	"context"
	"fmt"
	"net"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/mbver/cserf/rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Server struct {
	pb.UnimplementedSerfServer
}

func CreateServer(addr string) (*grpc.Server, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := grpc.NewServer()
	pb.RegisterSerfServer(s, &Server{})

	go func() {
		if err := s.Serve(l); err != nil {
			fmt.Printf("[ERR] grpc server: %v", err)
		}
	}()
	return s, nil
}

func (s *Server) Hello(ctx context.Context, name *wrappers.StringValue) (*wrappers.StringValue, error) {
	return &wrappers.StringValue{
		Value: fmt.Sprintf("Hallo doch %s", name),
	}, nil
}

func (s *Server) HelloStream(name *wrappers.StringValue, stream pb.Serf_HelloStreamServer) error {
	for i := 0; i < 3; i++ {
		if err := stream.Send(&wrapperspb.StringValue{
			Value: fmt.Sprintf("hello%s%d,", name.Value, i),
		}); err != nil {
			return err
		}
	}
	return nil
}
