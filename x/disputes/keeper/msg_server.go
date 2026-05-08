package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/disputes/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// InitiateDispute creates a new dispute.
func (m msgServer) InitiateDispute(goCtx context.Context, msg *types.MsgInitiateDispute) (*types.MsgInitiateDisputeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	challengerAddr, err := sdk.AccAddressFromBech32(msg.Challenger)
	if err != nil {
		return nil, fmt.Errorf("invalid challenger address: %w", err)
	}

	// Validate target exists (only for FACT type currently)
	var defender string
	if msg.TargetType == types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT {
		if m.knowledgeKeeper != nil {
			fact, found := m.knowledgeKeeper.GetFact(ctx, msg.TargetId)
			if !found {
				return nil, fmt.Errorf("%w: fact %s", types.ErrTargetNotFound, msg.TargetId)
			}
			defender = fact.GetSubmitter()
		}
	}
	if defender == "" {
		defender = "unknown"
	}

	// Check max active disputes
	params := m.GetParams(ctx)
	if uint32(m.CountActiveDisputes(ctx)) >= params.MaxActiveDisputes {
		return nil, types.ErrMaxActiveDisputes
	}

	// Validate bond meets tier minimum
	bond := new(big.Int)
	if _, ok := bond.SetString(msg.Bond, 10); !ok || bond.Sign() <= 0 {
		return nil, fmt.Errorf("%w: %s", types.ErrInvalidBond, msg.Bond)
	}

	tier := uint32(1) // start at tier 1
	tierCfg := types.GetTierConfig(params, tier)
	if tierCfg == nil {
		return nil, fmt.Errorf("tier 1 config not found")
	}

	minBond := new(big.Int)
	minBond.SetString(tierCfg.MinBond, 10)
	if bond.Cmp(minBond) < 0 {
		return nil, fmt.Errorf("%w: need %s, got %s (commitment 3: surviving challenge is the standard for credibility — the bond is what makes the challenge a serious attempt rather than a costless heckling)", types.ErrInsufficientBond, tierCfg.MinBond, msg.Bond)
	}

	// Escrow bond from challenger
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(bond)))
	if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, challengerAddr, types.ModuleName, coins); err != nil {
		return nil, fmt.Errorf("failed to escrow bond: %w", err)
	}

	// Select arbiters
	currentBlock := uint64(ctx.BlockHeight())
	arbiters, err := m.SelectArbiters(ctx, int(tierCfg.ArbiterCount), msg.Challenger, defender, currentBlock)
	if err != nil {
		// Refund bond on arbiter selection failure
		_ = m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, coins)
		return nil, err
	}

	// Generate dispute ID
	disputeID := GenerateDisputeID(msg.TargetId, msg.Challenger, currentBlock)

	dispute := &types.Dispute{
		Id:               disputeID,
		TargetId:         msg.TargetId,
		TargetType:       msg.TargetType,
		Challenger:       msg.Challenger,
		Defender:         defender,
		Reason:           msg.Reason,
		Bond:             msg.Bond,
		Tier:             tier,
		Phase:            types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
		Outcome:          types.DisputeOutcome_DISPUTE_OUTCOME_UNSPECIFIED,
		CreatedAt:        currentBlock,
		EvidenceDeadline: currentBlock + tierCfg.EvidencePeriod,
		VotingDeadline:   currentBlock + tierCfg.EvidencePeriod + tierCfg.VotingPeriod,
		Arbiters:         arbiters,
		EvidenceCount:    0,
	}

	m.SetDispute(ctx, dispute)

	// Commitment 3 (Popper, not popularity): a dispute is the formal
	// venue where a fact's claim to truth must survive serious challenge.
	// The opening of a dispute is the chain inviting the falsification
	// attempt — without this, "challengeable" is a property no one ever
	// exercises.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.dispute_initiated",
			sdk.NewAttribute("dispute_id", disputeID),
			sdk.NewAttribute("challenger", msg.Challenger),
			sdk.NewAttribute("defender", defender),
			sdk.NewAttribute("target_id", msg.TargetId),
			sdk.NewAttribute("bond", msg.Bond),
			sdk.NewAttribute("tier", fmt.Sprintf("%d", tier)),
			sdk.NewAttribute("creed_commitment", "3"),
		),
	)

	return &types.MsgInitiateDisputeResponse{DisputeId: disputeID}, nil
}

// CommitEvidence stores a hash commitment for evidence (commit phase).
func (m msgServer) CommitEvidence(goCtx context.Context, msg *types.MsgCommitEvidence) (*types.MsgCommitEvidenceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	dispute, found := m.GetDispute(ctx, msg.DisputeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrDisputeNotFound, msg.DisputeId)
	}

	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT {
		return nil, fmt.Errorf("%w: expected EVIDENCE_COMMIT, got %s", types.ErrWrongPhase, dispute.Phase.String())
	}

	currentBlock := uint64(ctx.BlockHeight())
	if currentBlock > dispute.EvidenceDeadline {
		return nil, fmt.Errorf("%w: evidence commit deadline passed", types.ErrDeadlinePassed)
	}

	// Verify submitter is a party
	if msg.Submitter != dispute.Challenger && msg.Submitter != dispute.Defender {
		return nil, fmt.Errorf("%w: %s is not challenger or defender", types.ErrNotParty, msg.Submitter)
	}

	// Check no existing commitment
	if _, exists := m.GetCommitment(ctx, msg.DisputeId, msg.Submitter); exists {
		return nil, fmt.Errorf("%w: %s already committed", types.ErrCommitmentExists, msg.Submitter)
	}

	side := "challenger"
	if msg.Submitter == dispute.Defender {
		side = "defender"
	}

	commitment := &types.EvidenceCommitment{
		DisputeId:   msg.DisputeId,
		Submitter:   msg.Submitter,
		Side:        side,
		ContentHash: msg.CommitmentHash,
		CommittedAt: currentBlock,
		Revealed:    false,
	}
	m.SetCommitment(ctx, commitment)

	// Commitment 10 (forward-only audit): the commitment hash binds the
	// submitter to specific content before reveal — the audit record
	// cannot be revised in place once the hash is on chain.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.evidence_committed",
			sdk.NewAttribute("dispute_id", msg.DisputeId),
			sdk.NewAttribute("submitter", msg.Submitter),
			sdk.NewAttribute("side", side),
			sdk.NewAttribute("creed_commitment", "10"),
		),
	)

	return &types.MsgCommitEvidenceResponse{}, nil
}

// RevealEvidence reveals previously committed evidence content (reveal phase).
func (m msgServer) RevealEvidence(goCtx context.Context, msg *types.MsgRevealEvidence) (*types.MsgRevealEvidenceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	dispute, found := m.GetDispute(ctx, msg.DisputeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrDisputeNotFound, msg.DisputeId)
	}

	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL {
		return nil, fmt.Errorf("%w: expected EVIDENCE_REVEAL, got %s", types.ErrWrongPhase, dispute.Phase.String())
	}

	currentBlock := uint64(ctx.BlockHeight())
	if currentBlock > dispute.EvidenceDeadline {
		return nil, fmt.Errorf("%w: evidence reveal deadline passed", types.ErrDeadlinePassed)
	}

	// Get commitment
	commitment, exists := m.GetCommitment(ctx, msg.DisputeId, msg.Submitter)
	if !exists {
		return nil, fmt.Errorf("%w: no commitment found for %s", types.ErrCommitmentNotFound, msg.Submitter)
	}
	if commitment.Revealed {
		return nil, fmt.Errorf("%w: commitment already revealed (commitment 10: a revealed evidence commitment is the corpus's record — re-revealing it would imply substitution, which the forward-only audit refuses)", types.ErrAlreadyRevealed)
	}

	// Verify hash: SHA256(content + nonce) must match commitment
	h := sha256.Sum256([]byte(msg.Content + msg.Nonce))
	revealedHash := hex.EncodeToString(h[:])
	if revealedHash != commitment.ContentHash {
		return nil, fmt.Errorf("%w: expected %s, got %s (commitment 10: the commitment hash is the chain's binding pre-commitment — content cannot be substituted at reveal time, the record runs forward only)", types.ErrHashMismatch, commitment.ContentHash, revealedHash)
	}

	// Mark commitment as revealed
	commitment.Revealed = true
	m.SetCommitment(ctx, commitment)

	// Store evidence
	evidenceID := fmt.Sprintf("%s-%s", msg.DisputeId, msg.Submitter)
	evidence := &types.DisputeEvidence{
		Id:          evidenceID,
		DisputeId:   msg.DisputeId,
		Submitter:   msg.Submitter,
		Side:        commitment.Side,
		Content:     msg.Content,
		SubmittedAt: currentBlock,
	}
	m.SetEvidence(ctx, evidence)

	// Increment evidence count
	dispute.EvidenceCount++
	m.SetDispute(ctx, dispute)

	// Commitment 10 (forward-only audit): revealing matches the prior
	// commitment hash, binding the evidence to the chain's record. The
	// pair (commit, reveal) is append-only — once revealed, the content
	// is the corpus's permanent record of what was offered as proof.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.evidence_revealed",
			sdk.NewAttribute("dispute_id", msg.DisputeId),
			sdk.NewAttribute("submitter", msg.Submitter),
			sdk.NewAttribute("evidence_id", evidenceID),
			sdk.NewAttribute("creed_commitment", "10"),
		),
	)

	return &types.MsgRevealEvidenceResponse{}, nil
}

// ArbiterVote records an arbiter's vote during the arbitration phase.
func (m msgServer) ArbiterVote(goCtx context.Context, msg *types.MsgArbiterVote) (*types.MsgArbiterVoteResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	dispute, found := m.GetDispute(ctx, msg.DisputeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrDisputeNotFound, msg.DisputeId)
	}

	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_ARBITRATION {
		return nil, fmt.Errorf("%w: expected ARBITRATION, got %s", types.ErrWrongPhase, dispute.Phase.String())
	}

	currentBlock := uint64(ctx.BlockHeight())
	if currentBlock > dispute.VotingDeadline {
		return nil, fmt.Errorf("%w: voting deadline passed", types.ErrDeadlinePassed)
	}

	// Verify arbiter is assigned
	isArbiter := false
	for _, arb := range dispute.Arbiters {
		if arb == msg.Arbiter {
			isArbiter = true
			break
		}
	}
	if !isArbiter {
		return nil, fmt.Errorf("%w: %s", types.ErrNotArbiter, msg.Arbiter)
	}

	// Check not already voted
	if _, exists := m.GetVote(ctx, msg.DisputeId, msg.Arbiter); exists {
		return nil, fmt.Errorf("%w: %s (commitment 3: each arbiter's judgment counts once — Popperian survival is meaningful only when every test is independent, not a repeated stamp by the same hand)", types.ErrAlreadyVoted, msg.Arbiter)
	}

	vote := &types.DisputeVote{
		DisputeId: msg.DisputeId,
		Arbiter:   msg.Arbiter,
		Vote:      msg.Vote,
		Stake:     "1000000", // default stake weight (can be enhanced with actual stake lookup)
		Rationale: msg.Reasoning,
		VotedAt:   currentBlock,
	}
	m.SetVote(ctx, vote)

	// Commitment 3 (Popper, not popularity) AND commitment 10
	// (forward-only audit): the arbiter's vote is one independent test
	// of the claim's survival, recorded immutably so the dialectical
	// history of the dispute remains visible to off-chain observers.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.arbiter_voted",
			sdk.NewAttribute("dispute_id", msg.DisputeId),
			sdk.NewAttribute("arbiter", msg.Arbiter),
			sdk.NewAttribute("vote", msg.Vote.String()),
			sdk.NewAttribute("creed_commitment", "3,10"),
		),
	)

	return &types.MsgArbiterVoteResponse{}, nil
}

// EscalateDispute moves a dispute to a higher tier with additional bond.
func (m msgServer) EscalateDispute(goCtx context.Context, msg *types.MsgEscalateDispute) (*types.MsgEscalateDisputeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	dispute, found := m.GetDispute(ctx, msg.DisputeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrDisputeNotFound, msg.DisputeId)
	}

	// Can only escalate from settled/draw outcomes or active arbitration
	if dispute.Phase == types.DisputePhase_DISPUTE_PHASE_TIMED_OUT {
		return nil, fmt.Errorf("%w: cannot escalate timed-out dispute", types.ErrWrongPhase)
	}

	// Check max tier
	if dispute.Tier >= 4 {
		return nil, types.ErrMaxTierReached
	}

	// Check escalation delay
	params := m.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())
	if currentBlock < dispute.CreatedAt+params.EscalationDelay {
		return nil, fmt.Errorf("%w: must wait until block %d", types.ErrEscalationTooEarly, dispute.CreatedAt+params.EscalationDelay)
	}

	// Verify requester is a party
	if msg.Requester != dispute.Challenger && msg.Requester != dispute.Defender {
		return nil, fmt.Errorf("%w: %s", types.ErrNotParty, msg.Requester)
	}

	// Escrow additional bond
	additionalBond := new(big.Int)
	if _, ok := additionalBond.SetString(msg.AdditionalBond, 10); !ok || additionalBond.Sign() <= 0 {
		return nil, fmt.Errorf("%w: %s", types.ErrInvalidBond, msg.AdditionalBond)
	}

	requesterAddr, err := sdk.AccAddressFromBech32(msg.Requester)
	if err != nil {
		return nil, fmt.Errorf("invalid requester address: %w", err)
	}

	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(additionalBond)))
	if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, requesterAddr, types.ModuleName, coins); err != nil {
		return nil, fmt.Errorf("failed to escrow additional bond: %w", err)
	}

	// Upgrade tier
	newTier := dispute.Tier + 1
	newTierCfg := types.GetTierConfig(params, newTier)
	if newTierCfg == nil {
		return nil, fmt.Errorf("tier %d config not found", newTier)
	}

	// Select new arbiters for the escalated tier
	newArbiters, err := m.SelectArbiters(ctx, int(newTierCfg.ArbiterCount), dispute.Challenger, dispute.Defender, currentBlock)
	if err != nil {
		// Refund additional bond on failure
		_ = m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, requesterAddr, coins)
		return nil, err
	}

	// Update bond total
	existingBond := new(big.Int)
	existingBond.SetString(dispute.Bond, 10)
	existingBond.Add(existingBond, additionalBond)

	dispute.Tier = newTier
	dispute.Bond = existingBond.String()
	dispute.Arbiters = newArbiters
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT
	dispute.EvidenceDeadline = currentBlock + newTierCfg.EvidencePeriod
	dispute.VotingDeadline = currentBlock + newTierCfg.EvidencePeriod + newTierCfg.VotingPeriod
	m.SetDispute(ctx, dispute)

	// Commitment 10 (forward-only audit): escalation re-enters evidence
	// at a higher tier without erasing the prior tier's record. The
	// dispute history runs forward only — every tier's votes and
	// evidence remain queryable as the audit substrate of how the
	// chain finally arrived at settlement.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.dispute_escalated",
			sdk.NewAttribute("dispute_id", msg.DisputeId),
			sdk.NewAttribute("new_tier", fmt.Sprintf("%d", newTier)),
			sdk.NewAttribute("additional_bond", msg.AdditionalBond),
			sdk.NewAttribute("total_bond", dispute.Bond),
			sdk.NewAttribute("creed_commitment", "10"),
		),
	)

	return &types.MsgEscalateDisputeResponse{NewTier: newTier}, nil
}

// SettleDispute manually triggers vote tallying and settlement (authority-gated).
func (m msgServer) SettleDispute(goCtx context.Context, msg *types.MsgSettleDispute) (*types.MsgSettleDisputeResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrUnauthorized, m.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	dispute, found := m.GetDispute(ctx, msg.DisputeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrDisputeNotFound, msg.DisputeId)
	}

	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_ARBITRATION {
		return nil, fmt.Errorf("%w: expected ARBITRATION, got %s (commitment 10: the dispute lifecycle is forward-only — once a phase has closed, settlement cannot re-enter it; the chain's record of how the dispute was resolved is bounded in time)", types.ErrWrongPhase, dispute.Phase.String())
	}

	// Tally votes
	outcome := m.TallyVotes(ctx, dispute)
	dispute.Outcome = outcome
	if outcome == types.DisputeOutcome_DISPUTE_OUTCOME_TIMED_OUT {
		dispute.Phase = types.DisputePhase_DISPUTE_PHASE_TIMED_OUT
	} else {
		dispute.Phase = types.DisputePhase_DISPUTE_PHASE_SETTLED
	}
	m.SetDispute(ctx, dispute)

	// Distribute bonds
	if err := m.DistributeSettlement(ctx, dispute); err != nil {
		return nil, fmt.Errorf("failed to distribute settlement: %w", err)
	}

	// Commitment 3 (Popper, not popularity) AND commitment 10
	// (forward-only audit): settlement IS the verdict on whether the
	// claim survived challenge — corroboration earned or refused. The
	// outcome is final post-resolve; future tiers can re-test, but this
	// resolution remains the chain's permanent record of how this
	// dispute closed.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.dispute_settled",
			sdk.NewAttribute("dispute_id", msg.DisputeId),
			sdk.NewAttribute("outcome", outcome.String()),
			sdk.NewAttribute("creed_commitment", "3,10"),
		),
	)

	return &types.MsgSettleDisputeResponse{Outcome: outcome}, nil
}

// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (m msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	m.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.disputes.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
