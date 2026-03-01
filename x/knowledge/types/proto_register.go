package types

// Bridge registration: our .pb.go files are generated with protoc-gen-go (Google)
// but Cosmos SDK v0.50 uses gogo protobuf's proto.MessageType() for the
// unknownproto field checker. This file registers all non-Msg proto messages
// so they pass the SDK's tx decoder.

import (
	"github.com/cosmos/gogoproto/proto"
)

func init() {
	proto.RegisterType((*ClaimStructure)(nil), "zerone.knowledge.v1.ClaimStructure")
	proto.RegisterType((*ClaimRelation)(nil), "zerone.knowledge.v1.ClaimRelation")
	proto.RegisterType((*Fact)(nil), "zerone.knowledge.v1.Fact")
	proto.RegisterType((*Claim)(nil), "zerone.knowledge.v1.Claim")
	proto.RegisterType((*Params)(nil), "zerone.knowledge.v1.Params")
}
