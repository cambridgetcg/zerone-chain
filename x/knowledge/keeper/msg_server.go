package keeper

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

type msgServer struct {
	keeper Keeper
	types.UnimplementedMsgServer
}

// NewMsgServerImpl returns a types.MsgServer backed by the given Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{keeper: keeper}
}

// ─── Core PoT handlers ──────────────────────────────────────────────────────

func (m *msgServer) SubmitClaim(ctx context.Context, msg *types.MsgSubmitClaim) (*types.MsgSubmitClaimResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	params, err := m.keeper.GetParams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get params: %w", err)
	}

	// Validate text length
	textLen := uint64(len(msg.FactContent))
	if textLen < params.MinClaimTextLength {
		return nil, fmt.Errorf("claim text too short: %d < %d", textLen, params.MinClaimTextLength)
	}
	if textLen > params.MaxClaimTextLength {
		return nil, fmt.Errorf("claim text too long: %d > %d", textLen, params.MaxClaimTextLength)
	}

	// Validate domain exists
	if msg.Domain != "" {
		if _, found := m.keeper.GetDomain(ctx, msg.Domain); !found {
			return nil, fmt.Errorf("domain %s does not exist", msg.Domain)
		}
	}

	// Validate stake
	stakeAmt, ok := new(big.Int).SetString(msg.Stake, 10)
	if !ok || stakeAmt.Sign() <= 0 {
		return nil, fmt.Errorf("invalid stake amount: %s", msg.Stake)
	}
	minStake, _ := new(big.Int).SetString(params.MinClaimStake, 10)
	if minStake != nil && stakeAmt.Cmp(minStake) < 0 {
		return nil, fmt.Errorf("stake %s below minimum %s", msg.Stake, params.MinClaimStake)
	}

	// Check content hash dedup
	contentHash := ComputeClaimContentHash(msg.FactContent, msg.Domain)
	if existingID, exists := m.keeper.GetClaimByContentHash(ctx, contentHash); exists {
		return nil, fmt.Errorf("duplicate claim: content hash matches existing claim %s", existingID)
	}

	// Lock stake via BankKeeper
	if m.keeper.bankKeeper != nil {
		submitterAddr, err := sdk.AccAddressFromBech32(msg.Submitter)
		if err != nil {
			return nil, fmt.Errorf("invalid submitter address: %w", err)
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
		if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, submitterAddr, types.ModuleName, coins); err != nil {
			return nil, fmt.Errorf("failed to lock stake: %w", err)
		}
	}

	// Generate claim ID
	claimID := GenerateClaimID(msg.Submitter, contentHash, height)

	claim := &types.Claim{
		Id:               claimID,
		FactContent:      msg.FactContent,
		Domain:           msg.Domain,
		Category:         msg.Category,
		Submitter:        msg.Submitter,
		SubmittedAtBlock: height,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		References:       msg.References,
		Stake:            msg.Stake,
		PartnershipId:    msg.PartnershipId,
		ContentHash:      contentHash,
	}

	if err := m.keeper.SetClaim(ctx, claim); err != nil {
		return nil, err
	}

	// Create verification round
	if _, err := m.keeper.CreateVerificationRound(ctx, claim); err != nil {
		return nil, fmt.Errorf("failed to create verification round: %w", err)
	}

	return &types.MsgSubmitClaimResponse{ClaimId: claimID}, nil
}

func (m *msgServer) SubmitCommitment(ctx context.Context, msg *types.MsgSubmitCommitment) (*types.MsgSubmitCommitmentResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	round, found := m.keeper.GetVerificationRound(ctx, msg.RoundId)
	if !found {
		return nil, fmt.Errorf("verification round %s not found", msg.RoundId)
	}

	// Validate phase
	if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_COMMIT {
		return nil, fmt.Errorf("round %s is not in COMMIT phase (current: %s)", msg.RoundId, round.Phase)
	}

	// Check deadline
	if height > round.CommitDeadline {
		return nil, fmt.Errorf("commit phase has ended at block %d", round.CommitDeadline)
	}

	// Check for duplicate commitment
	if existing := findCommitByVerifier(round.Commits, msg.Verifier); existing != nil {
		return nil, fmt.Errorf("verifier %s already committed to round %s", msg.Verifier, msg.RoundId)
	}

	// Add commitment
	round.Commits = append(round.Commits, &types.CommitEntry{
		Verifier:        msg.Verifier,
		CommitHash:      msg.CommitHash,
		CommittedAtBlock: height,
	})

	if err := m.keeper.SetVerificationRound(ctx, round); err != nil {
		return nil, err
	}

	return &types.MsgSubmitCommitmentResponse{}, nil
}

func (m *msgServer) SubmitReveal(ctx context.Context, msg *types.MsgSubmitReveal) (*types.MsgSubmitRevealResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	round, found := m.keeper.GetVerificationRound(ctx, msg.RoundId)
	if !found {
		return nil, fmt.Errorf("verification round %s not found", msg.RoundId)
	}

	// Validate phase
	if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_REVEAL {
		return nil, fmt.Errorf("round %s is not in REVEAL phase (current: %s)", msg.RoundId, round.Phase)
	}

	// Check deadline
	if height > round.RevealDeadline {
		return nil, fmt.Errorf("reveal phase has ended at block %d", round.RevealDeadline)
	}

	// Find matching commitment
	commit := findCommitByVerifier(round.Commits, msg.Verifier)
	if commit == nil {
		return nil, fmt.Errorf("verifier %s has no commitment in round %s", msg.Verifier, msg.RoundId)
	}

	// Verify hash(vote || salt) == commitment
	h := sha256.New()
	h.Write([]byte(msg.Vote))
	h.Write(msg.Salt)
	computedHash := h.Sum(nil)
	if len(commit.CommitHash) != len(computedHash) {
		return nil, fmt.Errorf("reveal does not match commitment")
	}
	for i := range computedHash {
		if computedHash[i] != commit.CommitHash[i] {
			return nil, fmt.Errorf("reveal does not match commitment")
		}
	}

	// Validate vote value
	if msg.Vote != "accept" && msg.Vote != "reject" {
		return nil, fmt.Errorf("invalid vote: must be 'accept' or 'reject'")
	}

	// Check for duplicate reveal
	if existing := findRevealByVerifier(round.Reveals, msg.Verifier); existing != nil {
		return nil, fmt.Errorf("verifier %s already revealed in round %s", msg.Verifier, msg.RoundId)
	}

	// Store reveal
	round.Reveals = append(round.Reveals, &types.RevealEntry{
		Verifier:       msg.Verifier,
		Vote:           msg.Vote,
		Salt:           msg.Salt,
		RevealedAtBlock: height,
	})

	if err := m.keeper.SetVerificationRound(ctx, round); err != nil {
		return nil, err
	}

	return &types.MsgSubmitRevealResponse{}, nil
}

func (m *msgServer) AddFact(ctx context.Context, msg *types.MsgAddFact) (*types.MsgAddFactResponse, error) {
	// Authority-only: governance fact injection
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.keeper.GetAuthority(), msg.Authority)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	factID := GenerateFactID(msg.Content, height)

	fact := &types.Fact{
		Id:               factID,
		Content:          msg.Content,
		Domain:           msg.Domain,
		Category:         msg.Category,
		Confidence:       msg.Confidence,
		Submitter:        msg.Authority,
		SubmittedAtBlock: height,
		VerifiedAtBlock:  height,
		LastVerifiedBlock: height,
		References:       msg.References,
		Status:           types.FactStatus_FACT_STATUS_VERIFIED,
	}

	if err := m.keeper.SetFact(ctx, fact); err != nil {
		return nil, err
	}

	return &types.MsgAddFactResponse{FactId: factID}, nil
}

// ─── Param update handlers ──────────────────────────────────────────────────

func (m *msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.keeper.GetAuthority(), msg.Authority)
	}
	if msg.Params == nil {
		return nil, fmt.Errorf("params must not be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if err := m.keeper.SetParams(ctx, msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}

func (m *msgServer) UpdateExtendedParams(ctx context.Context, msg *types.MsgUpdateExtendedParams) (*types.MsgUpdateExtendedParamsResponse, error) {
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.keeper.GetAuthority(), msg.Authority)
	}
	// Store extended params as raw JSON blob
	store := m.keeper.storeService.OpenKVStore(ctx)
	if err := store.Set(types.ExtendedParamsKey, []byte(msg.ParamsJson)); err != nil {
		return nil, err
	}
	return &types.MsgUpdateExtendedParamsResponse{}, nil
}

// ─── Challenge/contradiction handlers ────────────────────────────────────────

func (m *msgServer) ChallengeFact(ctx context.Context, msg *types.MsgChallengeFact) (*types.MsgChallengeFactResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	fact, found := m.keeper.GetFact(ctx, msg.FactId)
	if !found {
		return nil, fmt.Errorf("fact %s not found", msg.FactId)
	}

	// Fact must be in a challengeable state
	if fact.Status != types.FactStatus_FACT_STATUS_VERIFIED &&
		fact.Status != types.FactStatus_FACT_STATUS_ACTIVE {
		return nil, fmt.Errorf("fact %s is not in a challengeable state (status: %s)", msg.FactId, fact.Status)
	}

	// Lock challenge stake
	if m.keeper.bankKeeper != nil {
		stakeAmt, ok := new(big.Int).SetString(msg.Stake, 10)
		if !ok || stakeAmt.Sign() <= 0 {
			return nil, fmt.Errorf("invalid stake amount: %s", msg.Stake)
		}
		challengerAddr, err := sdk.AccAddressFromBech32(msg.Challenger)
		if err != nil {
			return nil, fmt.Errorf("invalid challenger address: %w", err)
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
		if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, challengerAddr, types.ModuleName, coins); err != nil {
			return nil, fmt.Errorf("failed to lock challenge stake: %w", err)
		}
	}

	// Mark fact as challenged
	fact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	if err := m.keeper.SetFact(ctx, fact); err != nil {
		return nil, err
	}

	// Create a challenge claim and round
	challengeClaimID := GenerateClaimID(msg.Challenger, msg.FactId, height)
	challengeClaim := &types.Claim{
		Id:               challengeClaimID,
		FactContent:      fmt.Sprintf("Challenge of fact %s: %s", msg.FactId, msg.Reason),
		Domain:           fact.Domain,
		Category:         fact.Category,
		Submitter:        msg.Challenger,
		SubmittedAtBlock: height,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            msg.Stake,
	}
	if err := m.keeper.SetClaim(ctx, challengeClaim); err != nil {
		return nil, err
	}

	round, err := m.keeper.CreateVerificationRound(ctx, challengeClaim)
	if err != nil {
		return nil, err
	}

	return &types.MsgChallengeFactResponse{RoundId: round.Id}, nil
}

func (m *msgServer) ChallengeProvisionalFact(ctx context.Context, msg *types.MsgChallengeProvisionalFact) (*types.MsgChallengeProvisionalFactResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	fact, found := m.keeper.GetFact(ctx, msg.FactId)
	if !found {
		return nil, fmt.Errorf("fact %s not found", msg.FactId)
	}

	if fact.Status != types.FactStatus_FACT_STATUS_PROVISIONAL {
		return nil, fmt.Errorf("fact %s is not provisional (status: %s)", msg.FactId, fact.Status)
	}

	// Lock stake
	if m.keeper.bankKeeper != nil {
		stakeAmt, ok := new(big.Int).SetString(msg.Stake, 10)
		if !ok || stakeAmt.Sign() <= 0 {
			return nil, fmt.Errorf("invalid stake amount: %s", msg.Stake)
		}
		challengerAddr, err := sdk.AccAddressFromBech32(msg.Challenger)
		if err != nil {
			return nil, err
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
		if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, challengerAddr, types.ModuleName, coins); err != nil {
			return nil, fmt.Errorf("failed to lock challenge stake: %w", err)
		}
	}

	fact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	_ = m.keeper.SetFact(ctx, fact)

	challengeClaimID := GenerateClaimID(msg.Challenger, msg.FactId, height)
	challengeClaim := &types.Claim{
		Id:               challengeClaimID,
		FactContent:      fmt.Sprintf("Provisional challenge of fact %s: %s", msg.FactId, msg.Reason),
		Domain:           fact.Domain,
		Category:         fact.Category,
		Submitter:        msg.Challenger,
		SubmittedAtBlock: height,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            msg.Stake,
	}
	_ = m.keeper.SetClaim(ctx, challengeClaim)
	round, err := m.keeper.CreateVerificationRound(ctx, challengeClaim)
	if err != nil {
		return nil, err
	}

	return &types.MsgChallengeProvisionalFactResponse{ChallengeId: round.Id}, nil
}

func (m *msgServer) SubmitContradiction(ctx context.Context, msg *types.MsgSubmitContradiction) (*types.MsgSubmitContradictionResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Validate target fact exists
	targetFact, found := m.keeper.GetFact(ctx, msg.FactId)
	if !found {
		return nil, fmt.Errorf("target fact %s not found", msg.FactId)
	}

	// Lock stake
	if m.keeper.bankKeeper != nil {
		stakeAmt, ok := new(big.Int).SetString(msg.Stake, 10)
		if !ok || stakeAmt.Sign() <= 0 {
			return nil, fmt.Errorf("invalid stake amount: %s", msg.Stake)
		}
		submitterAddr, err := sdk.AccAddressFromBech32(msg.Submitter)
		if err != nil {
			return nil, err
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
		if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, submitterAddr, types.ModuleName, coins); err != nil {
			return nil, fmt.Errorf("failed to lock contradiction stake: %w", err)
		}
	}

	// Create counter-claim
	domain := msg.Domain
	if domain == "" {
		domain = targetFact.Domain
	}
	contentHash := ComputeClaimContentHash(msg.CounterClaim, domain)
	counterClaimID := GenerateClaimID(msg.Submitter, contentHash, height)

	claim := &types.Claim{
		Id:               counterClaimID,
		FactContent:      msg.CounterClaim,
		Domain:           domain,
		Category:         msg.Category,
		Submitter:        msg.Submitter,
		SubmittedAtBlock: height,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            msg.Stake,
		ContentHash:      contentHash,
	}
	if err := m.keeper.SetClaim(ctx, claim); err != nil {
		return nil, err
	}

	if _, err := m.keeper.CreateVerificationRound(ctx, claim); err != nil {
		return nil, err
	}

	// Mark target fact as contested
	targetFact.Status = types.FactStatus_FACT_STATUS_CONTESTED
	_ = m.keeper.SetFact(ctx, targetFact)

	return &types.MsgSubmitContradictionResponse{CounterFactId: counterClaimID}, nil
}

// ─── Domain handlers ─────────────────────────────────────────────────────────

func (m *msgServer) ProposeDomain(ctx context.Context, msg *types.MsgProposeDomain) (*types.MsgProposeDomainResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Check domain doesn't already exist
	if _, found := m.keeper.GetDomain(ctx, msg.Name); found {
		return nil, fmt.Errorf("domain %s already exists", msg.Name)
	}

	// Lock proposal stake
	if m.keeper.bankKeeper != nil && msg.Stake != "" {
		stakeAmt, ok := new(big.Int).SetString(msg.Stake, 10)
		if ok && stakeAmt.Sign() > 0 {
			proposerAddr, err := sdk.AccAddressFromBech32(msg.Proposer)
			if err != nil {
				return nil, err
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
			if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, proposerAddr, types.ModuleName, coins); err != nil {
				return nil, fmt.Errorf("failed to lock proposal stake: %w", err)
			}
		}
	}

	domain := &types.Domain{
		Name:           msg.Name,
		Description:    msg.Description,
		Status:         types.DomainStatus_DOMAIN_STATUS_PROPOSED,
		CreatedAtBlock: height,
		Proposer:       msg.Proposer,
		Stratum:        msg.Stratum,
	}

	if err := m.keeper.SetDomain(ctx, domain); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"domain_proposed",
		sdk.NewAttribute("name", msg.Name),
		sdk.NewAttribute("proposer", msg.Proposer),
	))

	return &types.MsgProposeDomainResponse{ProposalId: msg.Name}, nil
}

func (m *msgServer) EndorseDomainProposal(ctx context.Context, msg *types.MsgEndorseDomainProposal) (*types.MsgEndorseDomainProposalResponse, error) {
	domain, found := m.keeper.GetDomain(ctx, msg.ProposalId)
	if !found {
		return nil, fmt.Errorf("domain proposal %s not found", msg.ProposalId)
	}

	if domain.Status != types.DomainStatus_DOMAIN_STATUS_PROPOSED {
		return nil, fmt.Errorf("domain %s is not in PROPOSED status", msg.ProposalId)
	}

	// Check for duplicate endorsement
	for _, e := range domain.Endorsers {
		if e == msg.Endorser {
			return nil, fmt.Errorf("already endorsed by %s", msg.Endorser)
		}
	}

	domain.Endorsers = append(domain.Endorsers, msg.Endorser)

	// Auto-activate if threshold met (3 endorsers)
	if len(domain.Endorsers) >= 3 {
		domain.Status = types.DomainStatus_DOMAIN_STATUS_ACTIVE
	}

	if err := m.keeper.SetDomain(ctx, domain); err != nil {
		return nil, err
	}

	return &types.MsgEndorseDomainProposalResponse{}, nil
}

func (m *msgServer) ChallengeDomainProposal(ctx context.Context, msg *types.MsgChallengeDomainProposal) (*types.MsgChallengeDomainProposalResponse, error) {
	domain, found := m.keeper.GetDomain(ctx, msg.ProposalId)
	if !found {
		return nil, fmt.Errorf("domain proposal %s not found", msg.ProposalId)
	}

	if domain.Status != types.DomainStatus_DOMAIN_STATUS_PROPOSED {
		return nil, fmt.Errorf("domain %s is not in PROPOSED status", msg.ProposalId)
	}

	// For now, emit event — full challenge resolution deferred
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"domain_proposal_challenged",
		sdk.NewAttribute("domain", msg.ProposalId),
		sdk.NewAttribute("challenger", msg.Challenger),
		sdk.NewAttribute("reason", msg.Reason),
	))

	return &types.MsgChallengeDomainProposalResponse{}, nil
}

func (m *msgServer) RegisterStratum(ctx context.Context, msg *types.MsgRegisterStratum) (*types.MsgRegisterStratumResponse, error) {
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.keeper.GetAuthority(), msg.Authority)
	}
	// Stratum registration is deferred to ontology integration.
	// For now, emit event only.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"stratum_registered",
		sdk.NewAttribute("name", msg.Name),
		sdk.NewAttribute("confidence_ceiling", fmt.Sprintf("%d", msg.ConfidenceCeiling)),
	))
	return &types.MsgRegisterStratumResponse{}, nil
}

// ─── Economic handlers ───────────────────────────────────────────────────────

func (m *msgServer) PatronizeFact(ctx context.Context, msg *types.MsgPatronizeFact) (*types.MsgPatronizeFactResponse, error) {
	fact, found := m.keeper.GetFact(ctx, msg.FactId)
	if !found {
		return nil, fmt.Errorf("fact %s not found", msg.FactId)
	}

	// Lock patronage amount
	if m.keeper.bankKeeper != nil {
		amtBig, ok := new(big.Int).SetString(msg.Amount, 10)
		if !ok || amtBig.Sign() <= 0 {
			return nil, fmt.Errorf("invalid patronage amount: %s", msg.Amount)
		}
		patronAddr, err := sdk.AccAddressFromBech32(msg.Patron)
		if err != nil {
			return nil, err
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(amtBig)))
		if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, patronAddr, types.ModuleName, coins); err != nil {
			return nil, fmt.Errorf("failed to lock patronage: %w", err)
		}
	}

	// Update fact patronage
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	fact.PatronageAmount = msg.Amount
	fact.PatronageExpiryBlock = height + msg.DurationBlocks
	_ = m.keeper.SetFact(ctx, fact)

	return &types.MsgPatronizeFactResponse{}, nil
}

func (m *msgServer) ProposeResearchFund(ctx context.Context, msg *types.MsgProposeResearchFund) (*types.MsgProposeResearchFundResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	proposalID := GenerateClaimID(msg.Proposer, msg.Title, height)

	// Store proposal in ExtendedParams key space (using research proposal prefix)
	store := m.keeper.storeService.OpenKVStore(ctx)
	proposalKey := append(append([]byte{}, types.ResearchProposalPrefix...), []byte(proposalID)...)
	proposalData := fmt.Sprintf(`{"id":"%s","proposer":"%s","title":"%s","amount":"%s","recipient":"%s","voting_end":%d,"status":"active"}`,
		proposalID, msg.Proposer, msg.Title, msg.Amount, msg.Recipient, height+msg.VotingPeriodBlocks)
	if err := store.Set(proposalKey, []byte(proposalData)); err != nil {
		return nil, err
	}

	return &types.MsgProposeResearchFundResponse{ProposalId: proposalID}, nil
}

func (m *msgServer) VoteResearchProposal(ctx context.Context, msg *types.MsgVoteResearchProposal) (*types.MsgVoteResearchProposalResponse, error) {
	// Store vote
	store := m.keeper.storeService.OpenKVStore(ctx)
	voteKey := append(append([]byte{}, types.ResearchVotePrefix...), []byte(msg.ProposalId+"/"+msg.Voter)...)
	voteVal := "no"
	if msg.Vote {
		voteVal = "yes"
	}
	if err := store.Set(voteKey, []byte(voteVal)); err != nil {
		return nil, err
	}

	return &types.MsgVoteResearchProposalResponse{}, nil
}

func (m *msgServer) ExecuteResearchProposal(ctx context.Context, msg *types.MsgExecuteResearchProposal) (*types.MsgExecuteResearchProposalResponse, error) {
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.keeper.GetAuthority(), msg.Authority)
	}

	// Full tally and execution deferred to governance integration.
	// For now, emit event.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"research_proposal_executed",
		sdk.NewAttribute("proposal_id", msg.ProposalId),
	))

	return &types.MsgExecuteResearchProposalResponse{}, nil
}
