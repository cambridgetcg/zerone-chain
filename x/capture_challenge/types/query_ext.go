package types

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------- ActiveChallenges Query (manual, not proto-generated) ----------

// QueryActiveChallengesRequest is the request type for the ActiveChallenges query.
type QueryActiveChallengesRequest struct{}

// QueryActiveChallengesResponse is the response type for the ActiveChallenges query.
type QueryActiveChallengesResponse struct {
	Challenges []*CaptureChallenge `json:"challenges"`
}

// QueryExtServer defines extra query methods beyond the proto-generated QueryServer.
type QueryExtServer interface {
	ActiveChallenges(context.Context, *QueryActiveChallengesRequest) (*QueryActiveChallengesResponse, error)
}

// UnimplementedQueryExtServer provides default implementations.
type UnimplementedQueryExtServer struct{}

func (UnimplementedQueryExtServer) ActiveChallenges(context.Context, *QueryActiveChallengesRequest) (*QueryActiveChallengesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ActiveChallenges not implemented")
}

// RegisterQueryExtServer registers the extended query server with gRPC.
func RegisterQueryExtServer(s grpc.ServiceRegistrar, srv QueryExtServer) {
	s.RegisterService(&QueryExt_ServiceDesc, srv)
}

const QueryExt_ActiveChallenges_FullMethodName = "/zerone.capture_challenge.v1.QueryExt/ActiveChallenges"

func _QueryExt_ActiveChallenges_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryActiveChallengesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryExtServer).ActiveChallenges(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: QueryExt_ActiveChallenges_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryExtServer).ActiveChallenges(ctx, req.(*QueryActiveChallengesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// QueryExt_ServiceDesc is the gRPC service descriptor for the extended query service.
var QueryExt_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "zerone.capture_challenge.v1.QueryExt",
	HandlerType: (*QueryExtServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ActiveChallenges",
			Handler:    _QueryExt_ActiveChallenges_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "zerone/capture_challenge/v1/query_ext",
}
