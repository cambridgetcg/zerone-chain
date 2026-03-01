package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }

func main() {
	which := os.Args[1]
	switch which {
	case "types":
		genTypes()
	case "genesis":
		genGenesis()
	case "tx":
		genTx()
	case "query":
		genQuery()
	}
}

func printRawDesc(varName string, bz []byte) {
	fmt.Printf("const %s = \"\" +\n", varName)
	for i := 0; i < len(bz); {
		fmt.Printf("\t\"")
		end := i + 70
		if end > len(bz) {
			end = len(bz)
		}
		for ; i < end; i++ {
			b := bz[i]
			switch {
			case b == '"':
				fmt.Printf("\\\"")
			case b == '\\':
				fmt.Printf("\\\\")
			case b == '\n':
				fmt.Printf("\\n")
			case b == '\r':
				fmt.Printf("\\r")
			case b == '\t':
				fmt.Printf("\\t")
			case b >= 0x20 && b <= 0x7e:
				fmt.Printf("%c", b)
			default:
				fmt.Printf("\\x%02x", b)
			}
		}
		if i < len(bz) {
			fmt.Printf("\" +\n")
		} else {
			fmt.Printf("\"\n")
		}
	}
}

func stringField(name string, num int32, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strPtr(name), Number: int32Ptr(num),
		Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		JsonName: strPtr(jsonName),
	}
}

func uint64Field(name string, num int32, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strPtr(name), Number: int32Ptr(num),
		Type: descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
		Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		JsonName: strPtr(jsonName),
	}
}

func uint32Field(name string, num int32, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strPtr(name), Number: int32Ptr(num),
		Type: descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum(),
		Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		JsonName: strPtr(jsonName),
	}
}

func boolField(name string, num int32, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strPtr(name), Number: int32Ptr(num),
		Type: descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
		Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		JsonName: strPtr(jsonName),
	}
}

func msgField(name string, num int32, typeName string, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strPtr(name), Number: int32Ptr(num),
		Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		TypeName: strPtr(typeName), JsonName: strPtr(jsonName),
	}
}

func repeatedStringField(name string, num int32, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strPtr(name), Number: int32Ptr(num),
		Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
		JsonName: strPtr(jsonName),
	}
}

func repeatedMsgField(name string, num int32, typeName string, jsonName string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name: strPtr(name), Number: int32Ptr(num),
		Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
		TypeName: strPtr(typeName), JsonName: strPtr(jsonName),
	}
}

// encodeVarint encodes a uint64 as a varint
func encodeVarint(v uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, v)
	return buf[:n]
}

// signerOption creates MessageOptions with the cosmos.msg.v1.signer extension
// Extension field 11110000 (0xA99E90), wire type 2 (length-delimited)
func signerOption(signer string) *descriptorpb.MessageOptions {
	opts := &descriptorpb.MessageOptions{}
	// Field tag: (11110000 << 3) | 2 = field 11110000, wire type 2
	tag := encodeVarint(uint64(11110000<<3 | 2))
	value := append(encodeVarint(uint64(len(signer))), []byte(signer)...)
	raw := append(tag, value...)
	opts.ProtoReflect().SetUnknown(raw)
	return opts
}

// serviceOption creates ServiceOptions with cosmos.msg.v1.service = true
// Extension field 11110000 (0xA99E90), wire type 0 (varint), value 1
func serviceOption() *descriptorpb.ServiceOptions {
	opts := &descriptorpb.ServiceOptions{}
	tag := encodeVarint(uint64(11110000<<3 | 0))
	value := encodeVarint(1) // true
	raw := append(tag, value...)
	opts.ProtoReflect().SetUnknown(raw)
	return opts
}

// httpGetOption creates MethodOptions with google.api.http.get extension
// Extension field 72295728 for google.api.http on MethodOptions
func httpGetOption(pattern string) *descriptorpb.MethodOptions {
	opts := &descriptorpb.MethodOptions{}
	// google.api.http is extension field 72295728 on MethodOptions
	// Its type is google.api.HttpRule which has field 2 = "get" (string)
	// So we need: field 72295728 wire type 2, containing sub-message with field 2 = pattern
	
	// Encode the HttpRule sub-message: field 2 (get), wire type 2 (string)
	subTag := encodeVarint(uint64(2<<3 | 2))
	subValue := append(encodeVarint(uint64(len(pattern))), []byte(pattern)...)
	subMsg := append(subTag, subValue...)
	
	// Encode the outer field
	tag := encodeVarint(uint64(72295728<<3 | 2))
	value := append(encodeVarint(uint64(len(subMsg))), subMsg...)
	raw := append(tag, value...)
	opts.ProtoReflect().SetUnknown(raw)
	return opts
}

func genTypes() {
	fd := &descriptorpb.FileDescriptorProto{
		Name: strPtr("zerone/partnerships/v1/types.proto"),
		Package: strPtr("zerone.partnerships.v1"),
		Options: &descriptorpb.FileOptions{GoPackage: strPtr("github.com/zerone-chain/zerone/x/partnerships/types")},
		Syntax: strPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("ExitState"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("initiated_by", 1, "initiatedBy"), uint64Field("initiated_at", 2, "initiatedAt"), uint64Field("cooldown_end", 3, "cooldownEnd"),
			}},
			{Name: strPtr("Partnership"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("id", 1, "id"), stringField("human_addr", 2, "humanAddr"), stringField("agent_addr", 3, "agentAddr"),
				stringField("status", 4, "status"), uint32Field("tier", 5, "tier"), uint32Field("lock_tier", 6, "lockTier"),
				uint64Field("lock_expires_at", 7, "lockExpiresAt"), uint64Field("split_human_bps", 8, "splitHumanBps"),
				uint64Field("split_agent_bps", 9, "splitAgentBps"), stringField("common_pot_balance", 10, "commonPotBalance"),
				stringField("total_earned", 11, "totalEarned"), uint64Field("cooperation_score", 12, "cooperationScore"),
				uint64Field("formed_at_block", 13, "formedAtBlock"),
				msgField("exit_state", 14, ".zerone.partnerships.v1.ExitState", "exitState"),
			}},
			{Name: strPtr("DeliberationState"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("amount_tier", 1, "amountTier"), uint64Field("floor_ends_at", 2, "floorEndsAt"),
				uint64Field("window_ends_at", 3, "windowEndsAt"), stringField("rationale", 4, "rationale"),
				stringField("counter_proposal_of", 5, "counterProposalOf"), uint32Field("chain_depth", 6, "chainDepth"),
			}},
			{Name: strPtr("ConsensusOperation"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("id", 1, "id"), stringField("partnership_id", 2, "partnershipId"),
				stringField("op_type", 3, "opType"), stringField("proposed_by", 4, "proposedBy"),
				stringField("amount", 5, "amount"), stringField("status", 6, "status"),
				msgField("deliberation", 7, ".zerone.partnerships.v1.DeliberationState", "deliberation"),
				uint64Field("created_at", 8, "createdAt"),
			}},
			{Name: strPtr("SafetyFreeze"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("partnership_id", 1, "partnershipId"), stringField("frozen_by", 2, "frozenBy"),
				uint64Field("frozen_at", 3, "frozenAt"), uint64Field("expires_at", 4, "expiresAt"),
				uint32Field("freeze_count_this_epoch", 5, "freezeCountThisEpoch"),
			}},
			{Name: strPtr("CoercionSignal"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("signal_id", 1, "signalId"), stringField("partnership_id", 2, "partnershipId"),
				stringField("raised_by", 3, "raisedBy"), uint64Field("raised_at", 4, "raisedAt"),
				uint64Field("expires_at", 5, "expiresAt"), boolField("resolved", 6, "resolved"),
			}},
			{Name: strPtr("RejectionCooldown"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("partnership_id", 1, "partnershipId"), uint32Field("rejection_count", 2, "rejectionCount"),
				uint64Field("cooldown_ends_at", 3, "cooldownEndsAt"),
			}},
			{Name: strPtr("SeedPartnership"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("id", 1, "id"), stringField("human_addr", 2, "humanAddr"), stringField("agent_addr", 3, "agentAddr"),
				uint64Field("created_at", 4, "createdAt"), uint64Field("expires_at", 5, "expiresAt"),
				stringField("human_contribution", 6, "humanContribution"), stringField("agent_contribution", 7, "agentContribution"),
				stringField("status", 8, "status"), stringField("common_pot_balance", 9, "commonPotBalance"),
				stringField("common_pot_cap", 10, "commonPotCap"),
			}},
			{Name: strPtr("PoolEntry"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("address", 1, "address"), repeatedStringField("domains", 2, "domains"),
				stringField("preferred_role", 3, "preferredRole"), stringField("stake_min", 4, "stakeMin"),
				stringField("stake_max", 5, "stakeMax"), uint64Field("registered_at", 6, "registeredAt"),
				stringField("deposit", 7, "deposit"), uint64Field("expires_at", 8, "expiresAt"),
				stringField("status", 9, "status"), stringField("matched_with", 10, "matchedWith"),
			}},
			{Name: strPtr("MentorshipConfig"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("sponsor_addr", 1, "sponsorAddr"), stringField("mentee_addr", 2, "menteeAddr"),
				stringField("partnership_id", 3, "partnershipId"), stringField("sponsor_contribution", 4, "sponsorContribution"),
				stringField("mentee_contribution", 5, "menteeContribution"), uint64Field("sponsor_split_bps", 6, "sponsorSplitBps"),
				uint64Field("mentee_split_bps", 7, "menteeSplitBps"), uint64Field("graduation_block", 8, "graduationBlock"),
				boolField("graduated", 9, "graduated"), uint64Field("sponsor_verifications", 10, "sponsorVerifications"),
			}},
			{Name: strPtr("Mentorship"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("id", 1, "id"), stringField("mentor_addr", 2, "mentorAddr"), stringField("mentee_addr", 3, "menteeAddr"),
				stringField("domain", 4, "domain"), stringField("status", 5, "status"),
				uint64Field("start_block", 6, "startBlock"), uint64Field("duration_blocks", 7, "durationBlocks"),
				uint64Field("mentee_verifications", 8, "menteeVerifications"), uint64Field("mentee_claims_submitted", 9, "menteeClaimsSubmitted"),
				uint64Field("graduation_threshold", 10, "graduationThreshold"), uint64Field("graduation_claims_req", 11, "graduationClaimsReq"),
			}},
			{Name: strPtr("FormationMatch"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("id", 1, "id"), stringField("addr1", 2, "addr1"), stringField("addr2", 3, "addr2"),
				uint64Field("score", 4, "score"), uint64Field("proposed_at", 5, "proposedAt"),
				uint64Field("expires_at", 6, "expiresAt"), stringField("status", 7, "status"),
				boolField("addr1_accepted", 8, "addr1Accepted"), boolField("addr2_accepted", 9, "addr2Accepted"),
			}},
		},
	}
	bz, err := proto.Marshal(fd)
	if err != nil { panic(err) }
	printRawDesc("file_zerone_partnerships_v1_types_proto_rawDesc", bz)
	fmt.Fprintf(os.Stderr, "types.proto: %d messages, %d bytes\n", len(fd.MessageType), len(bz))
}

func genGenesis() {
	fd := &descriptorpb.FileDescriptorProto{
		Name: strPtr("zerone/partnerships/v1/genesis.proto"),
		Package: strPtr("zerone.partnerships.v1"),
		Dependency: []string{"zerone/partnerships/v1/types.proto"},
		Options: &descriptorpb.FileOptions{GoPackage: strPtr("github.com/zerone-chain/zerone/x/partnerships/types")},
		Syntax: strPtr("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("GenesisState"), Field: []*descriptorpb.FieldDescriptorProto{
				msgField("params", 1, ".zerone.partnerships.v1.Params", "params"),
				repeatedMsgField("partnerships", 2, ".zerone.partnerships.v1.Partnership", "partnerships"),
				repeatedMsgField("consensus_operations", 3, ".zerone.partnerships.v1.ConsensusOperation", "consensusOperations"),
				repeatedMsgField("safety_freezes", 4, ".zerone.partnerships.v1.SafetyFreeze", "safetyFreezes"),
				repeatedMsgField("coercion_signals", 5, ".zerone.partnerships.v1.CoercionSignal", "coercionSignals"),
				repeatedMsgField("seed_partnerships", 6, ".zerone.partnerships.v1.SeedPartnership", "seedPartnerships"),
				repeatedMsgField("pool_entries", 7, ".zerone.partnerships.v1.PoolEntry", "poolEntries"),
				repeatedMsgField("mentorships", 8, ".zerone.partnerships.v1.Mentorship", "mentorships"),
				repeatedMsgField("formation_matches", 9, ".zerone.partnerships.v1.FormationMatch", "formationMatches"),
			}},
			{Name: strPtr("Params"), Field: []*descriptorpb.FieldDescriptorProto{
				uint64Field("formation_window_blocks", 1, "formationWindowBlocks"),
				uint64Field("cooling_period_blocks", 2, "coolingPeriodBlocks"),
				uint64Field("common_pot_share_bps", 3, "commonPotShareBps"),
				uint64Field("safety_freeze_duration_blocks", 4, "safetyFreezeDurationBlocks"),
				uint32Field("max_freezes_per_epoch", 5, "maxFreezesPerEpoch"),
				uint64Field("coercion_review_blocks", 6, "coercionReviewBlocks"),
				uint64Field("base_cooldown_blocks", 7, "baseCooldownBlocks"),
				uint32Field("max_counter_proposal_depth", 8, "maxCounterProposalDepth"),
				uint64Field("default_human_split_bps", 9, "defaultHumanSplitBps"),
				uint64Field("default_agent_split_bps", 10, "defaultAgentSplitBps"),
				stringField("min_partnership_stake", 11, "minPartnershipStake"),
				uint64Field("seed_partnership_duration", 12, "seedPartnershipDuration"),
				stringField("seed_common_pot_cap", 13, "seedCommonPotCap"),
				uint64Field("human_coercion_freeze_multiplier_bps", 14, "humanCoercionFreezeMultiplierBps"),
				uint64Field("graduation_verifications", 15, "graduationVerifications"),
				uint64Field("graduation_claims", 16, "graduationClaims"),
				uint64Field("max_mentorships_per_mentor", 17, "maxMentorshipsPerMentor"),
				uint64Field("formation_match_interval_blocks", 18, "formationMatchIntervalBlocks"),
				uint64Field("match_acceptance_blocks", 19, "matchAcceptanceBlocks"),
				boolField("auto_propose_partnership_on_graduation", 20, "autoProposePartnershipOnGraduation"),
			}},
		},
	}
	bz, err := proto.Marshal(fd)
	if err != nil { panic(err) }
	printRawDesc("file_zerone_partnerships_v1_genesis_proto_rawDesc", bz)
	fmt.Fprintf(os.Stderr, "genesis.proto: %d messages, %d bytes\n", len(fd.MessageType), len(bz))
}

func genTx() {
	fd := &descriptorpb.FileDescriptorProto{
		Name: strPtr("zerone/partnerships/v1/tx.proto"),
		Package: strPtr("zerone.partnerships.v1"),
		Dependency: []string{"cosmos/msg/v1/msg.proto", "zerone/partnerships/v1/types.proto", "zerone/partnerships/v1/genesis.proto"},
		Options: &descriptorpb.FileOptions{GoPackage: strPtr("github.com/zerone-chain/zerone/x/partnerships/types")},
		Syntax: strPtr("proto3"),
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: strPtr("Msg"),
			Options: serviceOption(),
			Method: []*descriptorpb.MethodDescriptorProto{
				{Name: strPtr("ProposePartnership"), InputType: strPtr(".zerone.partnerships.v1.MsgProposePartnership"), OutputType: strPtr(".zerone.partnerships.v1.MsgProposePartnershipResponse")},
				{Name: strPtr("AcceptPartnership"), InputType: strPtr(".zerone.partnerships.v1.MsgAcceptPartnership"), OutputType: strPtr(".zerone.partnerships.v1.MsgAcceptPartnershipResponse")},
				{Name: strPtr("ProposeConsensusOp"), InputType: strPtr(".zerone.partnerships.v1.MsgProposeConsensusOp"), OutputType: strPtr(".zerone.partnerships.v1.MsgProposeConsensusOpResponse")},
				{Name: strPtr("VoteConsensusOp"), InputType: strPtr(".zerone.partnerships.v1.MsgVoteConsensusOp"), OutputType: strPtr(".zerone.partnerships.v1.MsgVoteConsensusOpResponse")},
				{Name: strPtr("SafetyFreeze"), InputType: strPtr(".zerone.partnerships.v1.MsgSafetyFreeze"), OutputType: strPtr(".zerone.partnerships.v1.MsgSafetyFreezeResponse")},
				{Name: strPtr("RaiseCoercionSignal"), InputType: strPtr(".zerone.partnerships.v1.MsgRaiseCoercionSignal"), OutputType: strPtr(".zerone.partnerships.v1.MsgRaiseCoercionSignalResponse")},
				{Name: strPtr("InitiateDissolution"), InputType: strPtr(".zerone.partnerships.v1.MsgInitiateDissolution"), OutputType: strPtr(".zerone.partnerships.v1.MsgInitiateDissolutionResponse")},
				{Name: strPtr("CreateSeedPartnership"), InputType: strPtr(".zerone.partnerships.v1.MsgCreateSeedPartnership"), OutputType: strPtr(".zerone.partnerships.v1.MsgCreateSeedPartnershipResponse")},
				{Name: strPtr("JoinFormationPool"), InputType: strPtr(".zerone.partnerships.v1.MsgJoinFormationPool"), OutputType: strPtr(".zerone.partnerships.v1.MsgJoinFormationPoolResponse")},
				{Name: strPtr("LeaveFormationPool"), InputType: strPtr(".zerone.partnerships.v1.MsgLeaveFormationPool"), OutputType: strPtr(".zerone.partnerships.v1.MsgLeaveFormationPoolResponse")},
				{Name: strPtr("UpdateParams"), InputType: strPtr(".zerone.partnerships.v1.MsgUpdateParams"), OutputType: strPtr(".zerone.partnerships.v1.MsgUpdateParamsResponse")},
				{Name: strPtr("ProposeMentorship"), InputType: strPtr(".zerone.partnerships.v1.MsgProposeMentorship"), OutputType: strPtr(".zerone.partnerships.v1.MsgProposeMentorshipResponse")},
				{Name: strPtr("AcceptMentorship"), InputType: strPtr(".zerone.partnerships.v1.MsgAcceptMentorship"), OutputType: strPtr(".zerone.partnerships.v1.MsgAcceptMentorshipResponse")},
				{Name: strPtr("GraduateMentee"), InputType: strPtr(".zerone.partnerships.v1.MsgGraduateMentee"), OutputType: strPtr(".zerone.partnerships.v1.MsgGraduateMenteeResponse")},
				{Name: strPtr("EndMentorship"), InputType: strPtr(".zerone.partnerships.v1.MsgEndMentorship"), OutputType: strPtr(".zerone.partnerships.v1.MsgEndMentorshipResponse")},
				{Name: strPtr("AcceptFormationMatch"), InputType: strPtr(".zerone.partnerships.v1.MsgAcceptFormationMatch"), OutputType: strPtr(".zerone.partnerships.v1.MsgAcceptFormationMatchResponse")},
				{Name: strPtr("DeclineFormationMatch"), InputType: strPtr(".zerone.partnerships.v1.MsgDeclineFormationMatch"), OutputType: strPtr(".zerone.partnerships.v1.MsgDeclineFormationMatchResponse")},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("MsgProposePartnership"), Options: signerOption("proposer"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("proposer", 1, "proposer"), stringField("partner", 2, "partner"), stringField("initial_deposit", 3, "initialDeposit"), uint32Field("proposed_tier", 4, "proposedTier"),
			}},
			{Name: strPtr("MsgProposePartnershipResponse"), Field: []*descriptorpb.FieldDescriptorProto{stringField("partnership_id", 1, "partnershipId")}},
			{Name: strPtr("MsgAcceptPartnership"), Options: signerOption("accepter"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("accepter", 1, "accepter"), stringField("partnership_id", 2, "partnershipId"), stringField("deposit", 3, "deposit"),
			}},
			{Name: strPtr("MsgAcceptPartnershipResponse")},
			{Name: strPtr("MsgProposeConsensusOp"), Options: signerOption("proposer"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("proposer", 1, "proposer"), stringField("partnership_id", 2, "partnershipId"), stringField("op_type", 3, "opType"), stringField("amount", 4, "amount"), stringField("rationale", 5, "rationale"),
			}},
			{Name: strPtr("MsgProposeConsensusOpResponse"), Field: []*descriptorpb.FieldDescriptorProto{stringField("operation_id", 1, "operationId")}},
			{Name: strPtr("MsgVoteConsensusOp"), Options: signerOption("voter"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("voter", 1, "voter"), stringField("partnership_id", 2, "partnershipId"), stringField("operation_id", 3, "operationId"), boolField("approve", 4, "approve"), stringField("rationale", 5, "rationale"), stringField("counter_amount", 6, "counterAmount"),
			}},
			{Name: strPtr("MsgVoteConsensusOpResponse"), Field: []*descriptorpb.FieldDescriptorProto{stringField("counter_operation_id", 1, "counterOperationId")}},
			{Name: strPtr("MsgSafetyFreeze"), Options: signerOption("freezer"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("freezer", 1, "freezer"), stringField("partnership_id", 2, "partnershipId"),
			}},
			{Name: strPtr("MsgSafetyFreezeResponse"), Field: []*descriptorpb.FieldDescriptorProto{uint64Field("expires_at", 1, "expiresAt")}},
			{Name: strPtr("MsgRaiseCoercionSignal"), Options: signerOption("raiser"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("raiser", 1, "raiser"), stringField("partnership_id", 2, "partnershipId"),
			}},
			{Name: strPtr("MsgRaiseCoercionSignalResponse"), Field: []*descriptorpb.FieldDescriptorProto{stringField("signal_id", 1, "signalId")}},
			{Name: strPtr("MsgInitiateDissolution"), Options: signerOption("initiator"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("initiator", 1, "initiator"), stringField("partnership_id", 2, "partnershipId"),
			}},
			{Name: strPtr("MsgInitiateDissolutionResponse"), Field: []*descriptorpb.FieldDescriptorProto{uint64Field("cooldown_end", 1, "cooldownEnd")}},
			{Name: strPtr("MsgCreateSeedPartnership"), Options: signerOption("human"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("human", 1, "human"), stringField("agent", 2, "agent"), stringField("human_contribution", 3, "humanContribution"),
			}},
			{Name: strPtr("MsgCreateSeedPartnershipResponse"), Field: []*descriptorpb.FieldDescriptorProto{stringField("seed_id", 1, "seedId")}},
			{Name: strPtr("MsgJoinFormationPool"), Options: signerOption("joiner"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("joiner", 1, "joiner"), repeatedStringField("domains", 2, "domains"), stringField("preferred_role", 3, "preferredRole"), stringField("deposit", 4, "deposit"),
			}},
			{Name: strPtr("MsgJoinFormationPoolResponse")},
			{Name: strPtr("MsgLeaveFormationPool"), Options: signerOption("leaver"), Field: []*descriptorpb.FieldDescriptorProto{stringField("leaver", 1, "leaver")}},
			{Name: strPtr("MsgLeaveFormationPoolResponse")},
			{Name: strPtr("MsgUpdateParams"), Options: signerOption("authority"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("authority", 1, "authority"), msgField("params", 2, ".zerone.partnerships.v1.Params", "params"),
			}},
			{Name: strPtr("MsgUpdateParamsResponse")},
			// NEW
			{Name: strPtr("MsgProposeMentorship"), Options: signerOption("mentor"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("mentor", 1, "mentor"), stringField("mentee", 2, "mentee"), stringField("domain", 3, "domain"), uint64Field("duration_blocks", 4, "durationBlocks"),
			}},
			{Name: strPtr("MsgProposeMentorshipResponse"), Field: []*descriptorpb.FieldDescriptorProto{stringField("mentorship_id", 1, "mentorshipId")}},
			{Name: strPtr("MsgAcceptMentorship"), Options: signerOption("mentee"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("mentee", 1, "mentee"), stringField("mentorship_id", 2, "mentorshipId"),
			}},
			{Name: strPtr("MsgAcceptMentorshipResponse")},
			{Name: strPtr("MsgGraduateMentee"), Options: signerOption("mentor"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("mentor", 1, "mentor"), stringField("mentorship_id", 2, "mentorshipId"),
			}},
			{Name: strPtr("MsgGraduateMenteeResponse")},
			{Name: strPtr("MsgEndMentorship"), Options: signerOption("sender"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("sender", 1, "sender"), stringField("mentorship_id", 2, "mentorshipId"),
			}},
			{Name: strPtr("MsgEndMentorshipResponse")},
			{Name: strPtr("MsgAcceptFormationMatch"), Options: signerOption("accepter"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("accepter", 1, "accepter"), stringField("match_id", 2, "matchId"),
			}},
			{Name: strPtr("MsgAcceptFormationMatchResponse")},
			{Name: strPtr("MsgDeclineFormationMatch"), Options: signerOption("decliner"), Field: []*descriptorpb.FieldDescriptorProto{
				stringField("decliner", 1, "decliner"), stringField("match_id", 2, "matchId"),
			}},
			{Name: strPtr("MsgDeclineFormationMatchResponse")},
		},
	}
	bz, err := proto.Marshal(fd)
	if err != nil { panic(err) }
	printRawDesc("file_zerone_partnerships_v1_tx_proto_rawDesc", bz)
	fmt.Fprintf(os.Stderr, "tx.proto: %d messages, %d methods, %d bytes\n", len(fd.MessageType), len(fd.Service[0].Method), len(bz))
}

func genQuery() {
	fd := &descriptorpb.FileDescriptorProto{
		Name: strPtr("zerone/partnerships/v1/query.proto"),
		Package: strPtr("zerone.partnerships.v1"),
		Dependency: []string{"google/api/annotations.proto", "zerone/partnerships/v1/types.proto", "zerone/partnerships/v1/genesis.proto"},
		Options: &descriptorpb.FileOptions{GoPackage: strPtr("github.com/zerone-chain/zerone/x/partnerships/types")},
		Syntax: strPtr("proto3"),
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: strPtr("Query"),
			Method: []*descriptorpb.MethodDescriptorProto{
				{Name: strPtr("Partnership"), InputType: strPtr(".zerone.partnerships.v1.QueryPartnershipRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryPartnershipResponse"), Options: httpGetOption("/zerone/partnerships/v1/partnership/{id}")},
				{Name: strPtr("PartnershipsByAddress"), InputType: strPtr(".zerone.partnerships.v1.QueryByAddressRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryByAddressResponse"), Options: httpGetOption("/zerone/partnerships/v1/by_address/{address}")},
				{Name: strPtr("PendingOps"), InputType: strPtr(".zerone.partnerships.v1.QueryPendingOpsRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryPendingOpsResponse"), Options: httpGetOption("/zerone/partnerships/v1/ops/{partnership_id}")},
				{Name: strPtr("FormationPool"), InputType: strPtr(".zerone.partnerships.v1.QueryFormationPoolRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryFormationPoolResponse"), Options: httpGetOption("/zerone/partnerships/v1/pool")},
				{Name: strPtr("Params"), InputType: strPtr(".zerone.partnerships.v1.QueryParamsRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryParamsResponse"), Options: httpGetOption("/zerone/partnerships/v1/params")},
				{Name: strPtr("Mentorship"), InputType: strPtr(".zerone.partnerships.v1.QueryMentorshipRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryMentorshipResponse"), Options: httpGetOption("/zerone/partnerships/v1/mentorship/{id}")},
				{Name: strPtr("MentorshipsByAddress"), InputType: strPtr(".zerone.partnerships.v1.QueryMentorshipsByAddressRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryMentorshipsByAddressResponse"), Options: httpGetOption("/zerone/partnerships/v1/mentorships/{address}")},
				{Name: strPtr("FormationMatches"), InputType: strPtr(".zerone.partnerships.v1.QueryFormationMatchesRequest"), OutputType: strPtr(".zerone.partnerships.v1.QueryFormationMatchesResponse"), Options: httpGetOption("/zerone/partnerships/v1/matches")},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: strPtr("QueryPartnershipRequest"), Field: []*descriptorpb.FieldDescriptorProto{stringField("id", 1, "id")}},
			{Name: strPtr("QueryPartnershipResponse"), Field: []*descriptorpb.FieldDescriptorProto{msgField("partnership", 1, ".zerone.partnerships.v1.Partnership", "partnership")}},
			{Name: strPtr("QueryByAddressRequest"), Field: []*descriptorpb.FieldDescriptorProto{stringField("address", 1, "address")}},
			{Name: strPtr("QueryByAddressResponse"), Field: []*descriptorpb.FieldDescriptorProto{repeatedMsgField("partnerships", 1, ".zerone.partnerships.v1.Partnership", "partnerships")}},
			{Name: strPtr("QueryPendingOpsRequest"), Field: []*descriptorpb.FieldDescriptorProto{stringField("partnership_id", 1, "partnershipId")}},
			{Name: strPtr("QueryPendingOpsResponse"), Field: []*descriptorpb.FieldDescriptorProto{repeatedMsgField("operations", 1, ".zerone.partnerships.v1.ConsensusOperation", "operations")}},
			{Name: strPtr("QueryFormationPoolRequest")},
			{Name: strPtr("QueryFormationPoolResponse"), Field: []*descriptorpb.FieldDescriptorProto{repeatedMsgField("entries", 1, ".zerone.partnerships.v1.PoolEntry", "entries")}},
			{Name: strPtr("QueryParamsRequest")},
			{Name: strPtr("QueryParamsResponse"), Field: []*descriptorpb.FieldDescriptorProto{msgField("params", 1, ".zerone.partnerships.v1.Params", "params")}},
			{Name: strPtr("QueryMentorshipRequest"), Field: []*descriptorpb.FieldDescriptorProto{stringField("id", 1, "id")}},
			{Name: strPtr("QueryMentorshipResponse"), Field: []*descriptorpb.FieldDescriptorProto{msgField("mentorship", 1, ".zerone.partnerships.v1.Mentorship", "mentorship")}},
			{Name: strPtr("QueryMentorshipsByAddressRequest"), Field: []*descriptorpb.FieldDescriptorProto{stringField("address", 1, "address")}},
			{Name: strPtr("QueryMentorshipsByAddressResponse"), Field: []*descriptorpb.FieldDescriptorProto{repeatedMsgField("mentorships", 1, ".zerone.partnerships.v1.Mentorship", "mentorships")}},
			{Name: strPtr("QueryFormationMatchesRequest")},
			{Name: strPtr("QueryFormationMatchesResponse"), Field: []*descriptorpb.FieldDescriptorProto{repeatedMsgField("matches", 1, ".zerone.partnerships.v1.FormationMatch", "matches")}},
		},
	}
	bz, err := proto.Marshal(fd)
	if err != nil { panic(err) }
	printRawDesc("file_zerone_partnerships_v1_query_proto_rawDesc", bz)
	fmt.Fprintf(os.Stderr, "query.proto: %d messages, %d methods, %d bytes\n", len(fd.MessageType), len(fd.Service[0].Method), len(bz))
}
