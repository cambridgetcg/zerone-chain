package types

// Bridge registration: our .pb.go files are generated with protoc-gen-go (Google)
// but Cosmos SDK v0.50 uses gogo protobuf's proto.MessageType() for the
// unknownproto field checker. This file registers all non-Msg proto messages
// so they pass the SDK's tx decoder.

import (
	"github.com/cosmos/gogoproto/proto"
)

func init() {
	RegisterMsgAttestStorageProto()
	proto.RegisterType((*ConsentProof)(nil), "zerone.knowledge.v1.ConsentProof")
	proto.RegisterType((*Submission)(nil), "zerone.knowledge.v1.Submission")
	proto.RegisterType((*Sample)(nil), "zerone.knowledge.v1.Sample")
	proto.RegisterType((*QualityVote)(nil), "zerone.knowledge.v1.QualityVote")
	proto.RegisterType((*QualityRound)(nil), "zerone.knowledge.v1.QualityRound")
	proto.RegisterType((*CommitEntry)(nil), "zerone.knowledge.v1.CommitEntry")
	proto.RegisterType((*RevealEntry)(nil), "zerone.knowledge.v1.RevealEntry")
	proto.RegisterType((*VRFProof)(nil), "zerone.knowledge.v1.VRFProof")
	proto.RegisterType((*Domain)(nil), "zerone.knowledge.v1.Domain")
	proto.RegisterType((*ValidatorInfo)(nil), "zerone.knowledge.v1.ValidatorInfo")
	proto.RegisterType((*TrainingDemand)(nil), "zerone.knowledge.v1.TrainingDemand")
	proto.RegisterType((*DataBounty)(nil), "zerone.knowledge.v1.DataBounty")
	proto.RegisterType((*ScrapedSourceEntry)(nil), "zerone.knowledge.v1.ScrapedSourceEntry")
	proto.RegisterType((*Dataset)(nil), "zerone.knowledge.v1.Dataset")
	proto.RegisterType((*CompletedRoundMeta)(nil), "zerone.knowledge.v1.CompletedRoundMeta")
	proto.RegisterType((*Params)(nil), "zerone.knowledge.v1.Params")
	proto.RegisterType((*GenesisState)(nil), "zerone.knowledge.v1.GenesisState")
}
