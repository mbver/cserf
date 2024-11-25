// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.7
// source: pb/serf_service.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SerfClient is the client API for Serf service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SerfClient interface {
	Connect(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error)
	Query(ctx context.Context, in *QueryParam, opts ...grpc.CallOption) (Serf_QueryClient, error)
	Key(ctx context.Context, in *KeyRequest, opts ...grpc.CallOption) (*KeyResponse, error)
	Action(ctx context.Context, in *ActionRequest, opts ...grpc.CallOption) (*Empty, error)
	Reach(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*ReachResponse, error)
	Active(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*MembersResponse, error)
	Members(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*MembersResponse, error)
	Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (*IntValue, error)
	Leave(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error)
	Rtt(ctx context.Context, in *RttRequest, opts ...grpc.CallOption) (*durationpb.Duration, error)
	Tag(ctx context.Context, in *TagRequest, opts ...grpc.CallOption) (*Empty, error)
	Info(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Info, error)
	Monitor(ctx context.Context, in *MonitorRequest, opts ...grpc.CallOption) (Serf_MonitorClient, error)
}

type serfClient struct {
	cc grpc.ClientConnInterface
}

func NewSerfClient(cc grpc.ClientConnInterface) SerfClient {
	return &serfClient{cc}
}

func (c *serfClient) Connect(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.Serf/connect", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Query(ctx context.Context, in *QueryParam, opts ...grpc.CallOption) (Serf_QueryClient, error) {
	stream, err := c.cc.NewStream(ctx, &Serf_ServiceDesc.Streams[0], "/pb.Serf/query", opts...)
	if err != nil {
		return nil, err
	}
	x := &serfQueryClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Serf_QueryClient interface {
	Recv() (*StringValue, error)
	grpc.ClientStream
}

type serfQueryClient struct {
	grpc.ClientStream
}

func (x *serfQueryClient) Recv() (*StringValue, error) {
	m := new(StringValue)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *serfClient) Key(ctx context.Context, in *KeyRequest, opts ...grpc.CallOption) (*KeyResponse, error) {
	out := new(KeyResponse)
	err := c.cc.Invoke(ctx, "/pb.Serf/key", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Action(ctx context.Context, in *ActionRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.Serf/action", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Reach(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*ReachResponse, error) {
	out := new(ReachResponse)
	err := c.cc.Invoke(ctx, "/pb.Serf/reach", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Active(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*MembersResponse, error) {
	out := new(MembersResponse)
	err := c.cc.Invoke(ctx, "/pb.Serf/active", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Members(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*MembersResponse, error) {
	out := new(MembersResponse)
	err := c.cc.Invoke(ctx, "/pb.Serf/members", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (*IntValue, error) {
	out := new(IntValue)
	err := c.cc.Invoke(ctx, "/pb.Serf/join", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Leave(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.Serf/leave", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Rtt(ctx context.Context, in *RttRequest, opts ...grpc.CallOption) (*durationpb.Duration, error) {
	out := new(durationpb.Duration)
	err := c.cc.Invoke(ctx, "/pb.Serf/rtt", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Tag(ctx context.Context, in *TagRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.Serf/tag", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Info(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Info, error) {
	out := new(Info)
	err := c.cc.Invoke(ctx, "/pb.Serf/info", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) Monitor(ctx context.Context, in *MonitorRequest, opts ...grpc.CallOption) (Serf_MonitorClient, error) {
	stream, err := c.cc.NewStream(ctx, &Serf_ServiceDesc.Streams[1], "/pb.Serf/monitor", opts...)
	if err != nil {
		return nil, err
	}
	x := &serfMonitorClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Serf_MonitorClient interface {
	Recv() (*StringValue, error)
	grpc.ClientStream
}

type serfMonitorClient struct {
	grpc.ClientStream
}

func (x *serfMonitorClient) Recv() (*StringValue, error) {
	m := new(StringValue)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SerfServer is the server API for Serf service.
// All implementations must embed UnimplementedSerfServer
// for forward compatibility
type SerfServer interface {
	Connect(context.Context, *Empty) (*Empty, error)
	Query(*QueryParam, Serf_QueryServer) error
	Key(context.Context, *KeyRequest) (*KeyResponse, error)
	Action(context.Context, *ActionRequest) (*Empty, error)
	Reach(context.Context, *Empty) (*ReachResponse, error)
	Active(context.Context, *Empty) (*MembersResponse, error)
	Members(context.Context, *Empty) (*MembersResponse, error)
	Join(context.Context, *JoinRequest) (*IntValue, error)
	Leave(context.Context, *Empty) (*Empty, error)
	Rtt(context.Context, *RttRequest) (*durationpb.Duration, error)
	Tag(context.Context, *TagRequest) (*Empty, error)
	Info(context.Context, *Empty) (*Info, error)
	Monitor(*MonitorRequest, Serf_MonitorServer) error
	mustEmbedUnimplementedSerfServer()
}

// UnimplementedSerfServer must be embedded to have forward compatible implementations.
type UnimplementedSerfServer struct {
}

func (UnimplementedSerfServer) Connect(context.Context, *Empty) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Connect not implemented")
}
func (UnimplementedSerfServer) Query(*QueryParam, Serf_QueryServer) error {
	return status.Errorf(codes.Unimplemented, "method Query not implemented")
}
func (UnimplementedSerfServer) Key(context.Context, *KeyRequest) (*KeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Key not implemented")
}
func (UnimplementedSerfServer) Action(context.Context, *ActionRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Action not implemented")
}
func (UnimplementedSerfServer) Reach(context.Context, *Empty) (*ReachResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Reach not implemented")
}
func (UnimplementedSerfServer) Active(context.Context, *Empty) (*MembersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Active not implemented")
}
func (UnimplementedSerfServer) Members(context.Context, *Empty) (*MembersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Members not implemented")
}
func (UnimplementedSerfServer) Join(context.Context, *JoinRequest) (*IntValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Join not implemented")
}
func (UnimplementedSerfServer) Leave(context.Context, *Empty) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Leave not implemented")
}
func (UnimplementedSerfServer) Rtt(context.Context, *RttRequest) (*durationpb.Duration, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Rtt not implemented")
}
func (UnimplementedSerfServer) Tag(context.Context, *TagRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Tag not implemented")
}
func (UnimplementedSerfServer) Info(context.Context, *Empty) (*Info, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Info not implemented")
}
func (UnimplementedSerfServer) Monitor(*MonitorRequest, Serf_MonitorServer) error {
	return status.Errorf(codes.Unimplemented, "method Monitor not implemented")
}
func (UnimplementedSerfServer) mustEmbedUnimplementedSerfServer() {}

// UnsafeSerfServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SerfServer will
// result in compilation errors.
type UnsafeSerfServer interface {
	mustEmbedUnimplementedSerfServer()
}

func RegisterSerfServer(s grpc.ServiceRegistrar, srv SerfServer) {
	s.RegisterService(&Serf_ServiceDesc, srv)
}

func _Serf_Connect_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Connect(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/connect",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Connect(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Query_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(QueryParam)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SerfServer).Query(m, &serfQueryServer{stream})
}

type Serf_QueryServer interface {
	Send(*StringValue) error
	grpc.ServerStream
}

type serfQueryServer struct {
	grpc.ServerStream
}

func (x *serfQueryServer) Send(m *StringValue) error {
	return x.ServerStream.SendMsg(m)
}

func _Serf_Key_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Key(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/key",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Key(ctx, req.(*KeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Action_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ActionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Action(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/action",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Action(ctx, req.(*ActionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Reach_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Reach(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/reach",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Reach(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Active_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Active(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/active",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Active(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Members_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Members(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/members",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Members(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Join_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JoinRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Join(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/join",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Join(ctx, req.(*JoinRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Leave_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Leave(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/leave",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Leave(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Rtt_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RttRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Rtt(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/rtt",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Rtt(ctx, req.(*RttRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Tag_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TagRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Tag(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/tag",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Tag(ctx, req.(*TagRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Info_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Info(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/info",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Info(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_Monitor_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(MonitorRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SerfServer).Monitor(m, &serfMonitorServer{stream})
}

type Serf_MonitorServer interface {
	Send(*StringValue) error
	grpc.ServerStream
}

type serfMonitorServer struct {
	grpc.ServerStream
}

func (x *serfMonitorServer) Send(m *StringValue) error {
	return x.ServerStream.SendMsg(m)
}

// Serf_ServiceDesc is the grpc.ServiceDesc for Serf service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Serf_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "pb.Serf",
	HandlerType: (*SerfServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "connect",
			Handler:    _Serf_Connect_Handler,
		},
		{
			MethodName: "key",
			Handler:    _Serf_Key_Handler,
		},
		{
			MethodName: "action",
			Handler:    _Serf_Action_Handler,
		},
		{
			MethodName: "reach",
			Handler:    _Serf_Reach_Handler,
		},
		{
			MethodName: "active",
			Handler:    _Serf_Active_Handler,
		},
		{
			MethodName: "members",
			Handler:    _Serf_Members_Handler,
		},
		{
			MethodName: "join",
			Handler:    _Serf_Join_Handler,
		},
		{
			MethodName: "leave",
			Handler:    _Serf_Leave_Handler,
		},
		{
			MethodName: "rtt",
			Handler:    _Serf_Rtt_Handler,
		},
		{
			MethodName: "tag",
			Handler:    _Serf_Tag_Handler,
		},
		{
			MethodName: "info",
			Handler:    _Serf_Info_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "query",
			Handler:       _Serf_Query_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "monitor",
			Handler:       _Serf_Monitor_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "pb/serf_service.proto",
}
