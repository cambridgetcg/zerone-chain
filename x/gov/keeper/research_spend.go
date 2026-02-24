package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// --- Research Fund Voters CRUD ---

// GetResearchFundVoters returns the designated 2-of-2 voter config from the KV store.
// Falls back to Params.ResearchFundVoters if no dedicated store entry exists.
func (k Keeper) GetResearchFundVoters(ctx sdk.Context) *types.ResearchFundVoters {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ResearchVotersKey)
	if bz == nil {
		// Fall back to params
		params := k.GetParams(ctx)
		return params.ResearchFundVoters
	}
	var voters types.ResearchFundVoters
	if err := json.Unmarshal(bz, &voters); err != nil {
		return nil
	}
	return &voters
}

// SetResearchFundVoters stores the designated 2-of-2 voter config.
func (k Keeper) SetResearchFundVoters(ctx sdk.Context, voters *types.ResearchFundVoters) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(voters)
	if err != nil {
		panic("failed to marshal research fund voters: " + err.Error())
	}
	store.Set(types.ResearchVotersKey, bz)
}

// --- Research Spend Proposal CRUD ---

// GetResearchSpendProposal retrieves a research spend proposal by ID.
func (k Keeper) GetResearchSpendProposal(ctx sdk.Context, id uint64) (*types.ResearchSpendProposal, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ResearchSpendKey(id))
	if bz == nil {
		return nil, false
	}
	var prop types.ResearchSpendProposal
	if err := json.Unmarshal(bz, &prop); err != nil {
		return nil, false
	}
	return &prop, true
}

// SetResearchSpendProposal stores a research spend proposal.
func (k Keeper) SetResearchSpendProposal(ctx sdk.Context, prop *types.ResearchSpendProposal) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(prop)
	if err != nil {
		panic("failed to marshal research spend proposal: " + err.Error())
	}
	store.Set(types.ResearchSpendKey(prop.ProposalId), bz)
}

// GetNextResearchSpendID returns the next research spend proposal ID.
func (k Keeper) GetNextResearchSpendID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ResearchSpendCounterKey)
	if bz == nil {
		return 1
	}
	return binary.BigEndian.Uint64(bz)
}

// SetNextResearchSpendID sets the next research spend proposal ID.
func (k Keeper) SetNextResearchSpendID(ctx sdk.Context, id uint64) {
	store := ctx.KVStore(k.storeKey)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	store.Set(types.ResearchSpendCounterKey, bz)
}

// IterateResearchSpendProposals iterates over all research spend proposals.
func (k Keeper) IterateResearchSpendProposals(ctx sdk.Context, cb func(*types.ResearchSpendProposal) bool) {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.ResearchSpendKeyPrefix)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var prop types.ResearchSpendProposal
		if err := json.Unmarshal(iter.Value(), &prop); err != nil {
			continue
		}
		if cb(&prop) {
			break
		}
	}
}

// GetAllResearchSpendProposals returns all research spend proposals.
func (k Keeper) GetAllResearchSpendProposals(ctx sdk.Context) []*types.ResearchSpendProposal {
	var props []*types.ResearchSpendProposal
	k.IterateResearchSpendProposals(ctx, func(prop *types.ResearchSpendProposal) bool {
		props = append(props, prop)
		return false
	})
	return props
}

// --- Phase-Aware Voter Helpers ---

// isDesignatedResearchVoter checks if an address is authorized to vote on
// research spend proposals in the current phase.
func (k Keeper) isDesignatedResearchVoter(ctx sdk.Context, voter string) bool {
	voters := k.GetResearchFundVoters(ctx)
	if voters != nil && (voter == voters.Voter1 || voter == voters.Voter2) {
		return true
	}
	state := k.GetResearchFundGovernanceState(ctx)
	for _, seat := range state.CommunitySeats {
		if voter == seat {
			return true
		}
	}
	return false
}

// SetResearchCommunityVote records a community seat holder's vote on a research proposal.
func (k Keeper) SetResearchCommunityVote(ctx sdk.Context, proposalID uint64, voter, vote string) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.ResearchCommunityVoteKey(proposalID, voter), []byte(vote))
}

// GetResearchCommunityVote returns a community seat holder's vote, or "" if not voted.
func (k Keeper) GetResearchCommunityVote(ctx sdk.Context, proposalID uint64, voter string) string {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ResearchCommunityVoteKey(proposalID, voter))
	if bz == nil {
		return ""
	}
	return string(bz)
}

// countResearchSpendApprovals counts all "yes" votes on a research proposal
// across core voters (voter1, voter2) and community seat holders.
func (k Keeper) countResearchSpendApprovals(ctx sdk.Context, prop *types.ResearchSpendProposal) uint32 {
	var count uint32
	if prop.Voter1Vote == "yes" {
		count++
	}
	if prop.Voter2Vote == "yes" {
		count++
	}
	// Count community seat votes.
	state := k.GetResearchFundGovernanceState(ctx)
	for _, seat := range state.CommunitySeats {
		if k.GetResearchCommunityVote(ctx, prop.ProposalId, seat) == "yes" {
			count++
		}
	}
	return count
}

// CountCommunitySeatVotes counts how many community seat votes exist in the store.
func (k Keeper) CountCommunitySeatVotes(ctx sdk.Context, state *types.ResearchFundGovernanceState) uint64 {
	var count uint64
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.ResearchCommunityVotePrefix)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		count++
	}
	return count
}

// --- Handler Functions ---

// SubmitResearchSpend creates a new research fund spend proposal.
func (k Keeper) SubmitResearchSpend(ctx sdk.Context, msg *types.MsgSubmitResearchSpend) (*types.MsgSubmitResearchSpendResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	// Check voters are configured.
	voters := k.GetResearchFundVoters(ctx)
	if voters == nil || voters.Voter1 == "" || voters.Voter2 == "" {
		return nil, types.ErrResearchVotersNotSet
	}

	// Phase 3 (full governance) does not use multisig — reject.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.CurrentPhase == types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE {
		return nil, types.ErrPhaseFullGovernance
	}

	// Only designated voters (voter1, voter2, community seats) can submit.
	if !k.isDesignatedResearchVoter(ctx, msg.Proposer) {
		return nil, types.ErrNotDesignatedVoter
	}

	// Validate recipient.
	if _, err := sdk.AccAddressFromBech32(msg.Recipient); err != nil {
		return nil, fmt.Errorf("invalid recipient address: %w", err)
	}

	// Validate amount > 0.
	amountBig := new(big.Int)
	if _, ok := amountBig.SetString(msg.Amount, 10); !ok || amountBig.Sign() <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Get discussion and voting periods from params.
	params := k.GetParams(ctx)
	discussionBlocks := params.ResearchDiscussionBlocks
	if discussionBlocks == 0 {
		discussionBlocks = types.DefaultResearchDiscussionBlocks
	}
	votingBlocks := params.ResearchVotingBlocks
	if votingBlocks == 0 {
		votingBlocks = types.DefaultResearchVotingBlocks
	}

	// Create proposal.
	id := k.GetNextResearchSpendID(ctx)
	k.SetNextResearchSpendID(ctx, id+1)

	votingStartsAt := currentHeight + discussionBlocks
	votingEndsAt := votingStartsAt + votingBlocks

	prop := &types.ResearchSpendProposal{
		ProposalId:     id,
		Proposer:       msg.Proposer,
		Title:          msg.Title,
		Description:    msg.Description,
		Recipient:      msg.Recipient,
		Amount:         msg.Amount,
		Justification:  msg.Justification,
		Stage:          string(types.ResearchStageDiscussion),
		CreatedAt:      currentHeight,
		VotingStartsAt: votingStartsAt,
		VotingEndsAt:   votingEndsAt,
	}

	k.SetResearchSpendProposal(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.research_spend_submitted",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", id)),
			sdk.NewAttribute("proposer", msg.Proposer),
			sdk.NewAttribute("amount", msg.Amount),
			sdk.NewAttribute("recipient", msg.Recipient),
		),
	)

	return &types.MsgSubmitResearchSpendResponse{ProposalId: id}, nil
}

// VoteResearchSpend casts a vote on a research spend proposal.
func (k Keeper) VoteResearchSpend(ctx sdk.Context, msg *types.MsgVoteResearchSpend) (*types.MsgVoteResearchSpendResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	// Check voters are configured.
	voters := k.GetResearchFundVoters(ctx)
	if voters == nil || voters.Voter1 == "" || voters.Voter2 == "" {
		return nil, types.ErrResearchVotersNotSet
	}

	// Only designated voters for current phase can vote.
	if !k.isDesignatedResearchVoter(ctx, msg.Voter) {
		return nil, types.ErrNotDesignatedVoter
	}

	// Get proposal.
	prop, found := k.GetResearchSpendProposal(ctx, msg.ProposalId)
	if !found {
		return nil, types.ErrResearchProposalNotFound
	}

	// Must be in voting stage.
	if prop.Stage != string(types.ResearchStageVoting) {
		if prop.Stage == string(types.ResearchStageDiscussion) {
			if currentHeight < prop.VotingStartsAt {
				return nil, types.ErrDiscussionPeriodActive
			}
		}
		if types.IsTerminalResearchStage(types.ResearchSpendStage(prop.Stage)) {
			return nil, fmt.Errorf("proposal is in terminal stage: %s", prop.Stage)
		}
		if prop.Stage == string(types.ResearchStageDiscussion) {
			return nil, types.ErrDiscussionPeriodActive
		}
	}

	// Determine voter slot and record vote.
	isVoter1 := msg.Voter == voters.Voter1
	isVoter2 := msg.Voter == voters.Voter2

	if isVoter1 {
		if prop.Voter1Vote != "" {
			return nil, types.ErrResearchAlreadyVoted
		}
		prop.Voter1Vote = msg.Vote
		prop.Voter1Reason = msg.Reasoning
		prop.Voter1VotedAt = currentHeight
	} else if isVoter2 {
		if prop.Voter2Vote != "" {
			return nil, types.ErrResearchAlreadyVoted
		}
		prop.Voter2Vote = msg.Vote
		prop.Voter2Reason = msg.Reasoning
		prop.Voter2VotedAt = currentHeight
	} else {
		// Community seat voter.
		if k.GetResearchCommunityVote(ctx, msg.ProposalId, msg.Voter) != "" {
			return nil, types.ErrResearchAlreadyVoted
		}
		k.SetResearchCommunityVote(ctx, msg.ProposalId, msg.Voter, msg.Vote)
	}

	// Check for immediate resolution — phase-aware.
	if prop.Voter1Vote == "no" || prop.Voter2Vote == "no" {
		// Any core voter NO → rejected immediately.
		prop.Stage = string(types.ResearchStageRejected)
	} else {
		state := k.GetResearchFundGovernanceState(ctx)
		required, _ := types.GetResearchFundThreshold(state.CurrentPhase)
		approvals := k.countResearchSpendApprovals(ctx, prop)
		if approvals >= required {
			k.executeResearchSpend(ctx, prop)
			if prop.Stage == string(types.ResearchStageExecuted) {
				k.IncrementProposalsExecuted(ctx)
			}
		}
	}

	k.SetResearchSpendProposal(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.research_spend_voted",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", msg.ProposalId)),
			sdk.NewAttribute("voter", msg.Voter),
			sdk.NewAttribute("vote", msg.Vote),
			sdk.NewAttribute("stage", prop.Stage),
		),
	)

	return &types.MsgVoteResearchSpendResponse{}, nil
}

// SetResearchVoters configures the 2-of-2 research fund voters (authority only).
func (k Keeper) SetResearchVoters(ctx sdk.Context, msg *types.MsgSetResearchVoters) (*types.MsgSetResearchVotersResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, types.ErrUnauthorized
	}

	if msg.Voters == nil {
		return nil, fmt.Errorf("voters cannot be nil")
	}

	// Validate addresses.
	if _, err := sdk.AccAddressFromBech32(msg.Voters.Voter1); err != nil {
		return nil, fmt.Errorf("invalid voter1 address: %w", err)
	}
	if _, err := sdk.AccAddressFromBech32(msg.Voters.Voter2); err != nil {
		return nil, fmt.Errorf("invalid voter2 address: %w", err)
	}

	k.SetResearchFundVoters(ctx, msg.Voters)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.research_voters_set",
			sdk.NewAttribute("authority", msg.Authority),
			sdk.NewAttribute("voter1", msg.Voters.Voter1),
			sdk.NewAttribute("voter2", msg.Voters.Voter2),
		),
	)

	return &types.MsgSetResearchVotersResponse{}, nil
}

// executeResearchSpend executes a research fund disbursement via the vesting keeper.
func (k Keeper) executeResearchSpend(ctx sdk.Context, prop *types.ResearchSpendProposal) {
	currentHeight := uint64(ctx.BlockHeight())

	if k.vestingKeeper == nil {
		prop.ExecutionErr = "vesting rewards keeper not wired"
		return
	}

	// Parse amount.
	amountBig := new(big.Int)
	if _, ok := amountBig.SetString(prop.Amount, 10); !ok || amountBig.Sign() <= 0 {
		prop.ExecutionErr = "invalid amount"
		return
	}

	// Parse recipient.
	recipientAddr, err := sdk.AccAddressFromBech32(prop.Recipient)
	if err != nil {
		prop.ExecutionErr = fmt.Sprintf("invalid recipient: %v", err)
		return
	}

	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(amountBig)))

	if err := k.vestingKeeper.DisburseFromResearchFund(ctx, recipientAddr, coins); err != nil {
		prop.ExecutionErr = err.Error()
		return
	}

	prop.Stage = string(types.ResearchStageExecuted)
	prop.ExecutedAt = currentHeight
}

// --- BeginBlock Helper ---

// ProcessResearchSpendExpiry advances discussion→voting and expires timed-out voting proposals.
func (k Keeper) ProcessResearchSpendExpiry(ctx sdk.Context, currentHeight uint64) {
	k.IterateResearchSpendProposals(ctx, func(prop *types.ResearchSpendProposal) bool {
		changed := false

		if prop.Stage == string(types.ResearchStageDiscussion) && currentHeight >= prop.VotingStartsAt {
			prop.Stage = string(types.ResearchStageVoting)
			changed = true
		}

		if prop.Stage == string(types.ResearchStageVoting) && currentHeight >= prop.VotingEndsAt {
			prop.Stage = string(types.ResearchStageExpired)
			changed = true
		}

		if changed {
			k.SetResearchSpendProposal(ctx, prop)
		}

		return false
	})
}

// GetResearchFundBalance returns the research fund module account balance.
func (k Keeper) GetResearchFundBalance(ctx sdk.Context) sdk.Coins {
	if k.bankKeeper == nil {
		return sdk.NewCoins()
	}
	return k.bankKeeper.GetAllBalances(ctx, authtypes.NewModuleAddress("research_fund"))
}

// --- Phase Exit Condition Checking ---

// CheckPhaseExitConditions gathers live metrics and checks whether all
// conditions to transition out of the current phase are met.
func (k Keeper) CheckPhaseExitConditions(ctx sdk.Context) (*types.PhaseTransitionConditions, bool) {
	state := k.GetResearchFundGovernanceState(ctx)

	// Gather live metrics.
	var activeGuardians uint64
	if k.stakingKeeper != nil {
		ag, err := k.stakingKeeper.CountActiveGuardians(ctx)
		if err == nil {
			activeGuardians = ag
		}
	}

	var researchFundBalance string
	balance := k.GetResearchFundBalance(ctx)
	if uzrn := balance.AmountOf("uzrn"); !uzrn.IsZero() {
		researchFundBalance = uzrn.String()
	} else {
		researchFundBalance = "0"
	}

	var emergencyHalts uint64
	if k.emergencyKeeper != nil {
		emergencyHalts = k.emergencyKeeper.CountHaltsForReason(ctx, "research_fund")
	}

	conditions := &types.PhaseTransitionConditions{
		DistinctLipVoters:          k.CountDistinctVoters(ctx),
		ActiveGuardians:            activeGuardians,
		ResearchFundBalance:        researchFundBalance,
		ChainAgeBlocks:             uint64(ctx.BlockHeight()),
		ProposalsExecutedInPhase:   state.ProposalsExecutedInPhase,
		CommunitySeatParticipation: k.CountCommunitySeatVotes(ctx, state),
		EmergencyHaltsFromMisuse:   emergencyHalts,
	}

	// Check against exit conditions for current phase.
	exitConditions, exists := types.DefaultPhaseExitConditions[state.CurrentPhase]
	if !exists {
		// Full governance or unspecified — no exit conditions.
		return conditions, false
	}

	allMet := checkAllConditionsMet(conditions, &exitConditions)
	return conditions, allMet
}

// checkAllConditionsMet compares actual conditions against required thresholds.
func checkAllConditionsMet(actual *types.PhaseTransitionConditions, required *types.PhaseExitConditions) bool {
	if actual.DistinctLipVoters < required.MinDistinctVoters {
		return false
	}
	if actual.ActiveGuardians < required.MinActiveGuardians {
		return false
	}
	if actual.ChainAgeBlocks < required.MinChainAgeBlocks {
		return false
	}
	if actual.ProposalsExecutedInPhase < required.MinProposalsExecuted {
		return false
	}
	if required.MinCommunitySeatVotes > 0 && actual.CommunitySeatParticipation < required.MinCommunitySeatVotes {
		return false
	}
	if actual.EmergencyHaltsFromMisuse > required.MaxEmergencyHalts {
		return false
	}
	// Balance check.
	if required.MinResearchFundBalance != "" && required.MinResearchFundBalance != "0" {
		actualBal, ok1 := new(big.Int).SetString(actual.ResearchFundBalance, 10)
		requiredBal, ok2 := new(big.Int).SetString(required.MinResearchFundBalance, 10)
		if !ok1 || !ok2 || actualBal.Cmp(requiredBal) < 0 {
			return false
		}
	}
	return true
}
