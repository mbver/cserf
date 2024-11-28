package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	serf "github.com/mbver/cserf"
	"github.com/mbver/cserf/rpc/pb"
	memberlist "github.com/mbver/mlist"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

type Server struct {
	pb.UnimplementedSerfServer
	serf       *serf.Serf
	logStreams *logStreamManager
	logger     *log.Logger
}

func authInterceptor(authKeyHash string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		meta, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}
		keys := meta["authkey"]
		if len(keys) == 0 || !ComparePwdAndHash(keys[0], authKeyHash) {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authkey")
		}
		return handler(ctx, req)
	}
}

// TODO: authkey check for stream connection?
func CreateServer(conf *ServerConfig) (*Server, func(), error) {
	cleanup := func() {}
	if conf == nil {
		return nil, cleanup, fmt.Errorf("nil config")
	}
	if conf.AuthKeyHash == "" {
		return nil, cleanup, fmt.Errorf("empty authkey")
	}
	creds, err := getCredentials(conf.CertPath, conf.KeyPath)
	if err != nil {
		return nil, cleanup, err
	}
	logStreams := newLogStreamManager()
	logger, err := createLogger(
		conf.LogOutput,
		logStreams,
		conf.LogPrefix,
		conf.LogLevel,
		conf.SyslogFacility,
	)
	if err != nil {
		return nil, cleanup, err
	}

	serf, err := createSerf(conf, logger)
	if err != nil {
		return nil, cleanup, err
	}

	cleanup = serf.Shutdown

	// start mdns service
	if conf.ClusterName != "" {
		iface, _ := getIface(conf.NetInterface)
		_, err = NewClusterMDNS(serf, conf.ClusterName, logger, conf.IgnoreOld, iface)
		if err != nil {
			return nil, cleanup, err
		}
	}
	// start gprc server
	addr := net.JoinHostPort(conf.RpcAddress, strconv.Itoa(conf.RpcPort))
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, cleanup, err
	}
	cleanup1 := CombineCleanup(func() { l.Close() }, cleanup)

	server := &Server{
		serf:       serf,
		logStreams: logStreams,
		logger:     logger,
	}
	s := grpc.NewServer(
		grpc.Creds(creds),
		grpc.UnaryInterceptor(authInterceptor(conf.AuthKeyHash)),
	)
	pb.RegisterSerfServer(s, server)
	cleanup2 := CombineCleanup(s.Stop, cleanup1)
	go func() {
		if err := s.Serve(l); err != nil {
			logger.Printf("[ERR] grpc-server: failed serving %v", err)
		}
	}()
	return server, cleanup2, nil
}

func getCredentials(certPath, keyPath string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
	}), nil
}

func createSerf(conf *ServerConfig, logger *log.Logger) (*serf.Serf, error) {
	b := &serf.SerfBuilder{}
	b.WithLogger(logger)

	keyring, err := loadKeyring(conf.SerfConfig.KeyringFile)
	if err != nil {
		return nil, err
	}
	b.WithKeyring(keyring)

	b.WithMemberlistConfig(conf.MemberlistConfig)
	b.WithConfig(conf.SerfConfig)

	s, err := b.Build()
	if err != nil {
		return nil, err
	}
	if len(conf.StartJoin) > 0 {
		s.Join(conf.StartJoin, conf.IgnoreOld)
	}
	go retryJoin(s, conf, logger)
	return s, nil
}

func retryJoin(s *serf.Serf, conf *ServerConfig, logger *log.Logger) {
	if len(conf.RetryJoins) == 0 {
		return
	}
	if conf.RetryJoinMax == 0 {
		conf.RetryJoinMax = 1
	}
	if conf.RetryJoinInterval == 0 {
		conf.RetryJoinInterval = 1 * time.Second
	}
	for i := 0; i < conf.RetryJoinMax; i++ {
		n, err := s.Join(conf.RetryJoins, conf.IgnoreOld)
		if err == nil {
			logger.Printf("[INFO] server: successfully rejoin with %d nodes", n)
			return
		}
		time.Sleep(conf.RetryJoinInterval)
	}
	logger.Printf("[ERR] server: maximum retry join attempts failed, exiting...")
	s.Shutdown()
}

func (s *Server) ID() string {
	return s.serf.ID()
}

func (s *Server) SerfAddress() (string, error) {
	return s.serf.AdvertiseAddress()
}

func (s *Server) Shutdown() {
	s.serf.Shutdown()
}

func (s *Server) State() serf.SerfStateType {
	return s.serf.State()
}

func (s *Server) Connect(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
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
	numResp := 0
	s.serf.Query(respCh, p)
	for r := range respCh {
		numResp++
		n := len(r.Payload)
		if n > 0 && r.Payload[n-1] == '\n' {
			r.Payload = r.Payload[:n-1]
		}
		stream.Send(&pb.StringValue{
			Value: fmt.Sprintf("response from %s: %s", r.From, r.Payload),
		})
	}
	stream.Send(&pb.StringValue{
		Value: fmt.Sprintf("total number of responses: %d", numResp),
	})
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
		Tags:  serf.TagMapToString(n.Tags),
		State: n.State.String(),
		Lives: uint32(n.Lives),
	}
}

type regexFilter struct {
	key   string
	regex *regexp.Regexp
}

func toRegexFilter(f *pb.TagFilter) (*regexFilter, error) {
	re, err := regexp.Compile(f.Expr)
	if err != nil {
		return nil, err
	}
	return &regexFilter{f.Key, re}, nil
}

func toRegexFilters(pFilters []*pb.TagFilter) ([]*regexFilter, error) {
	res := make([]*regexFilter, len(pFilters))
	for i, f := range pFilters {
		rFilter, err := toRegexFilter(f)
		if err != nil {
			return nil, err
		}
		res[i] = rFilter
	}
	return res, nil
}

func matchTag(tags map[string]string, filters []*regexFilter) bool {
	for _, f := range filters {
		v := tags[f.key]
		if !f.regex.Match([]byte(v)) {
			return false
		}
	}
	return true
}

type statusFilter struct {
	status string
}

func isActive(state memberlist.StateType) bool {
	return state != memberlist.StateDead && state != memberlist.StateLeft
}

func (f *statusFilter) match(state memberlist.StateType) bool {
	switch f.status {
	case "active":
		return isActive(state)
	case "inactive":
		return !isActive(state)
	case "failed":
		return state == memberlist.StateDead
	case "left":
		return state == memberlist.StateLeft
	}
	return true
}

func isValidStatusFilter(s string) bool {
	return s == "active" || s == "inactive" || s == "failed" || s == "left"
}

func toStatusFilters(str string) ([]*statusFilter, error) {
	if len(str) == 0 {
		return []*statusFilter{}, nil
	}
	split := strings.Split(str, ",")
	res := make([]*statusFilter, 0, len(split))
	for _, s := range split {
		if !isValidStatusFilter(s) {
			return nil, fmt.Errorf("invalid status filter: %s", s)
		}
		res = append(res, &statusFilter{s})
	}
	return res, nil
}

func matchStatus(state memberlist.StateType, filters []*statusFilter) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f.match(state) {
			return true
		}
	}
	return false
}

func (s *Server) Members(ctx context.Context, req *pb.MemberRequest) (*pb.MembersResponse, error) {
	members := s.serf.Members()
	res := &pb.MembersResponse{
		Members: make([]*pb.Member, 0, len(members)),
	}
	tagFilters, err := toRegexFilters(req.TagFilters)
	if err != nil {
		return nil, err
	}
	statusFilters, err := toStatusFilters(req.StatusFilter)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if !matchTag(m.Tags, tagFilters) {
			continue
		}
		if !matchStatus(m.State, statusFilters) {
			continue
		}
		res.Members = append(res.Members, toPbMember(m))
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
	tags, _ := serf.TagsEncodedToString(n.Tags)
	addr := n.UDPAddress().String()
	return fmt.Sprintf("%s - %s - %s", n.ID, addr, tags)
}

func actionEventToString(e *serf.ActionEvent) string {
	return fmt.Sprintf("action %d - %s - %s", e.LTime, e.Name, string(e.Payload))
}

func queryEventToString(e *serf.QueryEvent) string {
	addr := net.JoinHostPort(e.SourceIP.String(), strconv.Itoa(int(e.SourcePort)))
	return fmt.Sprintf(`query - %d - %d - %s - %s
from: %s - %s`, e.LTime, e.ID, e.Name, e.Payload, addr, e.NodeID)
}

func (s *Server) Monitor(req *pb.MonitorRequest, stream pb.Serf_MonitorServer) error {
	eventCh := make(chan serf.Event, 1024)
	h := s.serf.StartStreamEvents(eventCh, req.EventFilter)
	defer s.serf.StopStreamEvents(h)

	logCh := make(chan string, 1024) // TODO should it this big?
	ls, err := newLogStreamer(logCh, req.LogLevel, s.logger)
	if err != nil {
		return err
	}
	s.logStreams.register(ls)
	defer s.logStreams.deregister(ls)

	sendStr := func(str string) (bool, error) {
		err := stream.Send(&pb.StringValue{
			Value: str,
		})
		if err != nil {
			s.logger.Printf("[ERR] grpc-server: error sending stream %v", err)
			if ShouldStopStreaming(err) {
				return true, err
			}
		}
		return false, err
	}

	for {
		select {
		case <-stream.Context().Done(): // TODO: LOG TERMINATION
			s.logger.Println("[INFO] grpc-server: stop streaming gracefully")
			return nil
		case e := <-eventCh:
			stop, err := sendStr(eventToString(e))
			if stop {
				return err
			}
		case l := <-logCh:
			stop, err := sendStr(l)
			if stop {
				return err
			}
		}
	}
}
