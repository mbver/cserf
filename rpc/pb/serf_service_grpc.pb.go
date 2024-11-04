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
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SerfClient is the client API for Serf service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SerfClient interface {
	Hello(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (*wrapperspb.StringValue, error)
	HelloStream(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (Serf_HelloStreamClient, error)
	Query(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (Serf_QueryClient, error)
}

type serfClient struct {
	cc grpc.ClientConnInterface
}

func NewSerfClient(cc grpc.ClientConnInterface) SerfClient {
	return &serfClient{cc}
}

func (c *serfClient) Hello(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (*wrapperspb.StringValue, error) {
	out := new(wrapperspb.StringValue)
	err := c.cc.Invoke(ctx, "/pb.Serf/hello", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *serfClient) HelloStream(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (Serf_HelloStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &Serf_ServiceDesc.Streams[0], "/pb.Serf/helloStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &serfHelloStreamClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Serf_HelloStreamClient interface {
	Recv() (*wrapperspb.StringValue, error)
	grpc.ClientStream
}

type serfHelloStreamClient struct {
	grpc.ClientStream
}

func (x *serfHelloStreamClient) Recv() (*wrapperspb.StringValue, error) {
	m := new(wrapperspb.StringValue)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *serfClient) Query(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (Serf_QueryClient, error) {
	stream, err := c.cc.NewStream(ctx, &Serf_ServiceDesc.Streams[1], "/pb.Serf/query", opts...)
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
	Recv() (*wrapperspb.StringValue, error)
	grpc.ClientStream
}

type serfQueryClient struct {
	grpc.ClientStream
}

func (x *serfQueryClient) Recv() (*wrapperspb.StringValue, error) {
	m := new(wrapperspb.StringValue)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SerfServer is the server API for Serf service.
// All implementations must embed UnimplementedSerfServer
// for forward compatibility
type SerfServer interface {
	Hello(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error)
	HelloStream(*wrapperspb.StringValue, Serf_HelloStreamServer) error
	Query(*emptypb.Empty, Serf_QueryServer) error
	mustEmbedUnimplementedSerfServer()
}

// UnimplementedSerfServer must be embedded to have forward compatible implementations.
type UnimplementedSerfServer struct {
}

func (UnimplementedSerfServer) Hello(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Hello not implemented")
}
func (UnimplementedSerfServer) HelloStream(*wrapperspb.StringValue, Serf_HelloStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method HelloStream not implemented")
}
func (UnimplementedSerfServer) Query(*emptypb.Empty, Serf_QueryServer) error {
	return status.Errorf(codes.Unimplemented, "method Query not implemented")
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

func _Serf_Hello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(wrapperspb.StringValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SerfServer).Hello(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Serf/hello",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SerfServer).Hello(ctx, req.(*wrapperspb.StringValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _Serf_HelloStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(wrapperspb.StringValue)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SerfServer).HelloStream(m, &serfHelloStreamServer{stream})
}

type Serf_HelloStreamServer interface {
	Send(*wrapperspb.StringValue) error
	grpc.ServerStream
}

type serfHelloStreamServer struct {
	grpc.ServerStream
}

func (x *serfHelloStreamServer) Send(m *wrapperspb.StringValue) error {
	return x.ServerStream.SendMsg(m)
}

func _Serf_Query_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(emptypb.Empty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SerfServer).Query(m, &serfQueryServer{stream})
}

type Serf_QueryServer interface {
	Send(*wrapperspb.StringValue) error
	grpc.ServerStream
}

type serfQueryServer struct {
	grpc.ServerStream
}

func (x *serfQueryServer) Send(m *wrapperspb.StringValue) error {
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
			MethodName: "hello",
			Handler:    _Serf_Hello_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "helloStream",
			Handler:       _Serf_HelloStream_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "query",
			Handler:       _Serf_Query_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "pb/serf_service.proto",
}
