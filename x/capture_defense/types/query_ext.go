package types

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------- FlaggedDomains Query (manual, not proto-generated) ----------

// QueryFlaggedDomainsRequest is the request type for the FlaggedDomains query.
type QueryFlaggedDomainsRequest struct{}

// QueryFlaggedDomainsResponse is the response type for the FlaggedDomains query.
type QueryFlaggedDomainsResponse struct {
	Metrics []*CaptureMetrics `json:"metrics"`
}

// QueryExtServer defines extra query methods beyond the proto-generated QueryServer.
type QueryExtServer interface {
	FlaggedDomains(context.Context, *QueryFlaggedDomainsRequest) (*QueryFlaggedDomainsResponse, error)
}

// UnimplementedQueryExtServer provides default implementations.
type UnimplementedQueryExtServer struct{}

func (UnimplementedQueryExtServer) FlaggedDomains(context.Context, *QueryFlaggedDomainsRequest) (*QueryFlaggedDomainsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method FlaggedDomains not implemented")
}

// RegisterQueryExtServer registers the extended query server with gRPC.
func RegisterQueryExtServer(s grpc.ServiceRegistrar, srv QueryExtServer) {
	s.RegisterService(&QueryExt_ServiceDesc, srv)
}

const QueryExt_FlaggedDomains_FullMethodName = "/zerone.capture_defense.v1.QueryExt/FlaggedDomains"

func _QueryExt_FlaggedDomains_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryFlaggedDomainsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryExtServer).FlaggedDomains(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: QueryExt_FlaggedDomains_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryExtServer).FlaggedDomains(ctx, req.(*QueryFlaggedDomainsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// QueryExt_ServiceDesc is the gRPC service descriptor for the extended query service.
var QueryExt_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "zerone.capture_defense.v1.QueryExt",
	HandlerType: (*QueryExtServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "FlaggedDomains",
			Handler:    _QueryExt_FlaggedDomains_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "zerone/capture_defense/v1/query_ext",
}
