package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	serf "github.com/mbver/cserf"
	"github.com/mbver/cserf/cmd/utils"
	"github.com/mbver/cserf/rpc/pb"
	memberlist "github.com/mbver/mlist"
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

func eventToString(event serf.Event) string {
	switch e := event.(type) {
	case *serf.CoalescedMemberEvent:
		return memberEventToString(e)
	case *serf.ActionEvent:
		return actionEventToString(e)
	case *serf.QueryEvent:
		return queryEventToString(e)
	}
	return "unknow-event"
}

func memberEventToString(e *serf.CoalescedMemberEvent) string {
	buf := strings.Builder{}
	buf.WriteString(fmt.Sprintf("{\n  event-type: %s\n", e.Type.String()))
	buf.WriteString("  members:\n")
	for _, n := range e.Members {
		buf.WriteString(fmt.Sprintf("    %s,", nodeToString(n)))
	}
	buf.WriteString("\n}")
	return buf.String()
}

func nodeToString(n *memberlist.Node) string {
	tags, _ := serf.ToTagString(n.Tags)
	addr := n.UDPAddress().String()
	return fmt.Sprintf("%s - %s - %s", n.ID, addr, tags)
}

func actionEventToString(e *serf.ActionEvent) string {
	return fmt.Sprintf("action: %d - %s - %s", e.LTime, e.Name, string(e.Payload))
}

func queryEventToString(e *serf.QueryEvent) string {
	addr := net.JoinHostPort(e.SourceIP.String(), strconv.Itoa(int(e.SourcePort)))
	return fmt.Sprintf(`query - %d - %d - %s - %s
from: %s - %s`, e.LTime, e.ID, e.Name, e.Payload, addr, e.NodeID)
}

func (s *Server) Monitor(filter *pb.StringValue, stream pb.Serf_MonitorServer) error {
	eventCh := make(chan serf.Event, 1024)
	h := s.serf.StartStreamEvents(eventCh, filter.Value)
	defer s.serf.StopStreamEvents(h)
	for {
		select {
		case <-stream.Context().Done(): // TODO: LOG TERMINATION
			fmt.Println("==== stop streaming gracefully...")
			return nil
		case e := <-eventCh:
			err := stream.Send(&pb.StringValue{
				Value: eventToString(e),
			})
			if err != nil { // TODO: LOG ERROR
				if utils.ShouldStopStreaming(err) {
					return err
				}
				continue
			}
		}
	}
}
