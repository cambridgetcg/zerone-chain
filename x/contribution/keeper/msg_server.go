package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/contribution/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	keeper *Keeper
}

// NewMsgServerImpl returns a Msg server for the keeper.
func NewMsgServerImpl(k *Keeper) types.MsgServer {
	return &msgServer{keeper: k}
}

var _ types.MsgServer = (*msgServer)(nil)

// SubmitContribution handles MsgSubmitContribution. For KNOWLEDGE_CLAIM
// at Phase 1, the hooks-driven path is the default; this handler exists
// as a parity entry that performs the same Classify+SubstrateLink+Verify
// dispatch but doesn't call x/knowledge.SubmitClaim. For other classes,
// returns ErrAdapterNotRegistered until those adapters land.
func (s *msgServer) SubmitContribution(ctx context.Context, msg *types.MsgSubmitContribution) (*types.MsgSubmitContributionResponse, error) {
	adapter, ok := s.keeper.GetAdapter(msg.Class)
	if !ok {
		return nil, types.ErrAdapterNotRegistered.Wrapf("class=%s", msg.Class)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Build the Contribution record.
	c := &types.Contribution{
		Id:                      computeID(msg),
		Contributor:             msg.Contributor,
		Class:                   msg.Class,
		Phase:                   msg.Phase,
		ManifestCid:             msg.ManifestCid,
		Lineage:                 msg.Lineage,
		Stake:                   msg.Stake,
		Status:                  types.ContributionStatus_STATUS_SUBMITTED,
		ClaimsAboutSelf:         msg.ClaimsAboutSelf,
		TruthFloorAttestation:   msg.TruthFloorAttestation,
		DeclaredSubCreedVersion: msg.DeclaredSubCreedVersion,
		CreatedAtBlock:          uint64(sdkCtx.BlockHeight()),
		Payload:                 msg.Payload,
		Recursion: &types.RecursionImpact{
			Type:          types.RecursionType_RECURSION_TYPE_NONE,
			MultiplierBps: 10_000, // 1× at Phase 1
			Revocable:     true,
			Axes:          &types.RecursionAxisScores{},
		},
	}

	// Stage ② — Write SUBMITTED record, then Classify.
	// Emit contribution_submitted while status is still SUBMITTED so indexers
	// see consistent state per event.
	if err := s.keeper.WriteContribution(ctx, c); err != nil {
		return nil, err
	}
	s.keeper.EmitContributionSubmitted(ctx, c)

	if err := adapter.Classify(ctx, c); err != nil {
		c.Status = types.ContributionStatus_STATUS_CLASSIFICATION_FAILED
		_ = s.keeper.WriteContribution(ctx, c)
		s.keeper.EmitClassificationFailed(ctx, c.Id, err.Error())
		return &types.MsgSubmitContributionResponse{ContributionId: c.Id, Status: c.Status}, nil
	}
	linkBps, err := adapter.SubstrateLink(ctx, c)
	if err != nil {
		c.Status = types.ContributionStatus_STATUS_CLASSIFICATION_FAILED
		_ = s.keeper.WriteContribution(ctx, c)
		s.keeper.EmitClassificationFailed(ctx, c.Id, err.Error())
		return &types.MsgSubmitContributionResponse{ContributionId: c.Id, Status: c.Status}, nil
	}
	c.SubstrateLinkBps = linkBps
	c.Status = types.ContributionStatus_STATUS_CLASSIFIED
	if err := s.keeper.WriteContribution(ctx, c); err != nil {
		return nil, err
	}
	s.keeper.EmitContributionClassified(ctx, c)

	// Stage ③ — Verify.
	score, vErr := adapter.Verify(ctx, c)
	c.VerificationScoreBps = score
	if vErr != nil || score < types.MinVerificationScoreBps {
		c.Status = types.ContributionStatus_STATUS_VERIFICATION_FAILED
		_ = s.keeper.WriteContribution(ctx, c)
		reason := "verification score below threshold"
		if vErr != nil {
			reason = vErr.Error()
		}
		s.keeper.EmitVerificationFailed(ctx, c, reason)
		return &types.MsgSubmitContributionResponse{ContributionId: c.Id, Status: c.Status}, nil
	}
	c.Status = types.ContributionStatus_STATUS_VERIFIED
	if err := s.keeper.WriteContribution(ctx, c); err != nil {
		return nil, err
	}
	s.keeper.EmitUsefulWorkAttested(ctx, c)
	s.keeper.EmitUsefulWorkSettled(ctx, c)
	s.keeper.EmitRecursionWeightComputed(ctx, c)

	// Stage ④ — Admission is NOT automatic at Phase 1 for the parity path.
	// For KNOWLEDGE_CLAIM, the hooks path drives ADMITTED via AfterClaimAccepted.
	// Other classes will define their own admission semantics in their phase.
	// Phase 1 returns the Contribution at STATUS_VERIFIED.

	return &types.MsgSubmitContributionResponse{ContributionId: c.Id, Status: c.Status}, nil
}

// computeID returns the canonical 32-byte sha256 id for a contribution.
// Combines class+phase+contributor+payload-bytes-if-any. Stable so
// that resubmission of the same contribution produces the same id.
func computeID(msg *types.MsgSubmitContribution) []byte {
	h := sha256.New()
	classBz := make([]byte, 4)
	binary.BigEndian.PutUint32(classBz, uint32(msg.Class))
	h.Write(classBz)
	phaseBz := make([]byte, 4)
	binary.BigEndian.PutUint32(phaseBz, uint32(msg.Phase))
	h.Write(phaseBz)
	h.Write([]byte(msg.Contributor))
	if msg.Payload != nil {
		// Marshal the payload deterministically. proto.MarshalOptions with
		// Deterministic:true ensures stable output within Go.
		// Phase 1 accepts this; future phases may use a deterministic
		// canonical form.
		bz, err := proto.MarshalOptions{Deterministic: true}.Marshal(msg.Payload)
		if err != nil {
			panic(fmt.Sprintf("marshal payload for id: %v", err))
		}
		h.Write(bz)
	}
	out := h.Sum(nil)
	return out[:]
}
