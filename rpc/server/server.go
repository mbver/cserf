package server

import (
	"context"
	"fmt"
	"net"

	serf "github.com/mbver/cserf"
	"github.com/mbver/cserf/rpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/durationpb"
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

func (s *Server) Action(ctx context.Context, req *pb.ActionRequest) (*pb.Empty, error) {
	err := s.serf.Action(req.Name, req.Payload)
	if err != nil {
		return nil, err
	}
	return nil, err
}

func (s *Server) Reach(ctx context.Context, req *pb.Empty) (*pb.ReachResponse, error) {
	res, err := s.serf.ReachQuery()
	if err != nil {
		return nil, err
	}
	return &pb.ReachResponse{
		NumNode: uint32(res.NumNode),
		NumRes:  uint32(res.NumResp),
		Acked:   res.Acked,
	}, nil
}

func toPbMember(n *serf.Member) *pb.Member {
	return &pb.Member{
		Id:    n.ID,
		Addr:  n.Addr,
		Tags:  n.Tags,
		State: n.State,
		Lives: uint32(n.Lives),
	}
}

func (s *Server) Active(ctx context.Context, req *pb.Empty) (*pb.MembersResponse, error) {
	nodes := s.serf.ActiveNodes()
	res := &pb.MembersResponse{
		Members: make([]*pb.Member, len(nodes)),
	}
	for i, n := range nodes {
		res.Members[i] = toPbMember(n)
	}
	return res, nil
}

func (s *Server) Members(ctx context.Context, req *pb.Empty) (*pb.MembersResponse, error) {
	nodes := s.serf.Members()
	res := &pb.MembersResponse{
		Members: make([]*pb.Member, len(nodes)),
	}
	for i, n := range nodes {
		res.Members[i] = toPbMember(n)
	}
	return res, nil
}

func (s *Server) Join(ctx context.Context, req *pb.JoinRequest) (*pb.IntValue, error) {
	n, err := s.serf.Join(req.Addrs, req.IgnoreOld)
	return &pb.IntValue{
		Value: int32(n),
	}, err
}

func (s *Server) Leave(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	err := s.serf.Leave()
	return &pb.Empty{}, err
}

func (s *Server) Rtt(ctx context.Context, req *pb.RttRequest) (*durationpb.Duration, error) {
	first := s.serf.GetCachedCoord(req.First)
	if first == nil {
		return nil, fmt.Errorf("no coord for node %s", req.First)
	}
	if req.Second == "" {
		req.Second = s.serf.ID()
	}
	second := s.serf.GetCachedCoord(req.Second)
	if second == nil {
		return nil, fmt.Errorf("no coord for node %s", req.Second)
	}
	rtt := first.DistanceTo(second)
	return durationpb.New(rtt), nil
}

const (
	tagUpdateCommand = "update"
	tagUnsetCommand  = "unset"
)

func (s *Server) Tag(ctx context.Context, req *pb.TagRequest) (*pb.Empty, error) {
	if req.Command == tagUpdateCommand {
		err := s.serf.SetTags(req.Tags)
		return &pb.Empty{}, err
	}
	if req.Command == tagUnsetCommand {
		err := s.serf.UnsetTagKeys(req.Keys)
		return &pb.Empty{}, err
	}
	return nil, fmt.Errorf("invalid tag command %s", req.Command)
}

func (s *Server) Info(ctx context.Context, req *pb.Empty) (*pb.Info, error) {
	node := s.serf.LocalMember()
	stats := s.serf.Stats()
	return &pb.Info{
		Node:  toPbMember(node),
		Stats: stats,
	}, nil
}
