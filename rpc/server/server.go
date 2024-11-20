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

func QueryParamFromPb(params *pb.QueryParam) *serf.QueryParam {
	var res = &serf.QueryParam{}
	res.Name = params.Name
	res.ForNodes = params.ForNodes
	for _, tag := range params.FilterTags {
		f := serf.FilterTag{
			Name: tag.Name,
			Expr: tag.Expr,
		}
		res.FilterTags = append(res.FilterTags, f)
	}
	res.Timeout = params.Timeout.AsDuration()
	res.NumRelays = uint8(params.NumRelays)
	res.Payload = params.Payload
	return res
}

func (s *Server) Query(params *pb.QueryParam, stream pb.Serf_QueryServer) error {
	var p *serf.QueryParam
	if params != nil {
		p = QueryParamFromPb(params)
	}
	respCh := make(chan *serf.QueryResponse)
	s.serf.Query(respCh, p)
	for r := range respCh {
		stream.Send(&pb.StringValue{
			Value: r.From,
		})
	}
	return nil
}

func toPbKeyResponse(r *serf.KeyQueryResponse) *pb.KeyResponse {
	pR := &pb.KeyResponse{
		NumNodes:        uint32(r.NumNode),
		NumRes:          uint32(r.NumResp),
		NumErr:          uint32(r.NumErr),
		ErrFrom:         map[string]string{},
		PrimaryKeyCount: map[string]uint32{},
		KeyCount:        map[string]uint32{},
	}
	for k, v := range r.ErrFrom {
		pR.ErrFrom[k] = v
	}
	for k, v := range r.PrimaryKeyCount {
		pR.PrimaryKeyCount[k] = uint32(v)
	}
	for k, v := range r.KeyCount {
		pR.KeyCount[k] = uint32(v)
	}
	return pR
}

func (s *Server) Key(ctx context.Context, req *pb.KeyRequest) (*pb.KeyResponse, error) {
	resp, err := s.serf.KeyQuery(req.Command, req.Key)
	if err != nil {
		return nil, err
	}
	return toPbKeyResponse(resp), nil
}
