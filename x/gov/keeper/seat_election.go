package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Candidate Validation ----------

// CountCandidateGovernanceVotes counts the number of distinct LIPs on which
// the candidate has cast a governance vote.
func (k Keeper) CountCandidateGovernanceVotes(ctx sdk.Context, candidate string) uint64 {
	seen := make(map[string]bool)
	allVotes := k.GetAllVotes(ctx)
	for _, v := range allVotes {
		if v.Voter == candidate && !seen[v.LipId] {
			seen[v.LipId] = true
		}
	}
	return uint64(len(seen))
}

// ValidateSeatCandidate checks whether a candidate is eligible for a community seat:
// 1. Must be Guardian tier (via staking keeper)
// 2. Must have voted on at least MinCandidateGovernanceVotes distinct LIPs
// 3. Must not already hold a community seat
func (k Keeper) ValidateSeatCandidate(ctx sdk.Context, candidate string) error {
	// 1. Guardian tier check.
	if k.stakingKeeper != nil {
		isGuardian, err := k.stakingKeeper.IsGuardian(ctx, candidate)
		if err != nil {
			return fmt.Errorf("failed to check guardian status: %w", err)
		}
		if !isGuardian {
			return types.ErrNotGuardianTier
		}
	}

	// 2. Governance participation check.
	govVotes := k.CountCandidateGovernanceVotes(ctx, candidate)
	if govVotes < types.MinCandidateGovernanceVotes {
		return types.ErrInsufficientGovHistory
	}

	// 3. No double-seat check.
	state := k.GetResearchFundGovernanceState(ctx)
	for _, seat := range state.CommunitySeats {
		if seat == candidate {
			return types.ErrSeatAlreadyHeld
		}
	}

	return nil
}

// ---------- Nomination + Acceptance ----------

// NominateSeatElection creates a new seat election proposal in "nominated" stage.
func (k Keeper) NominateSeatElection(ctx sdk.Context, msg *types.MsgNominateSeatElection) (*types.MsgNominateSeatElectionResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	// Validate phase supports elections (OBSERVER or BALANCED).
	phase := k.GetResearchFundPhase(ctx)
	if phase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER &&
		phase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED {
		return nil, fmt.Errorf("seat elections are only available in Observer or Balanced phase")
	}

	// Validate seat index for current phase.
	maxSeat := maxSeatIndexForPhase(phase)
	if msg.SeatIndex >= maxSeat {
		return nil, types.ErrInvalidSeatIndex
	}

	// Allocate proposal ID.
	id := k.GetNextSeatElectionID(ctx)
	k.SetNextSeatElectionID(ctx, id+1)

	prop := &types.SeatElectionProposal{
		ProposalId:         id,
		Proposer:           msg.Proposer,
		Candidate:          msg.Candidate,
		SeatIndex:          msg.SeatIndex,
		Statement:          msg.Statement,
		Stage:              types.SeatStageNominated,
		YesStake:           "0",
		NoStake:            "0",
		AbstainStake:       "0",
		AcceptanceDeadline: currentHeight + types.SeatAcceptanceBlocks,
		CreatedAtBlock:     currentHeight,
		CandidateAccepted:  false,
		IsRunoff:           false,
	}

	k.SetSeatElection(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_election_nominated",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", id)),
			sdk.NewAttribute("proposer", msg.Proposer),
			sdk.NewAttribute("candidate", msg.Candidate),
			sdk.NewAttribute("seat_index", fmt.Sprintf("%d", msg.SeatIndex)),
		),
	)

	return &types.MsgNominateSeatElectionResponse{ProposalId: id}, nil
}

// AcceptSeatNomination accepts a pending nomination, advancing to "discussion" stage.
func (k Keeper) AcceptSeatNomination(ctx sdk.Context, msg *types.MsgAcceptSeatNomination) (*types.MsgAcceptSeatNominationResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	prop, found := k.GetSeatElection(ctx, msg.ProposalId)
	if !found {
		return nil, types.ErrSeatElectionNotFound
	}

	// Must be in nominated stage.
	if prop.Stage != types.SeatStageNominated {
		return nil, types.ErrSeatNominationNotAccepted
	}

	// Must be the nominated candidate.
	if msg.Candidate != prop.Candidate {
		return nil, types.ErrNotNominatedCandidate
	}

	// Check acceptance deadline.
	if currentHeight > prop.AcceptanceDeadline {
		return nil, types.ErrSeatNominationExpired
	}

	// Validate candidate eligibility.
	if err := k.ValidateSeatCandidate(ctx, msg.Candidate); err != nil {
		return nil, err
	}

	// Advance to discussion stage.
	prop.Stage = types.SeatStageDiscussion
	prop.CandidateAccepted = true
	prop.DiscussionEndBlock = currentHeight + types.SeatDiscussionBlocks

	k.SetSeatElection(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_nomination_accepted",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", msg.ProposalId)),
			sdk.NewAttribute("candidate", msg.Candidate),
			sdk.NewAttribute("discussion_end_block", fmt.Sprintf("%d", prop.DiscussionEndBlock)),
		),
	)

	return &types.MsgAcceptSeatNominationResponse{}, nil
}

// ---------- Voting ----------

// VoteSeatElection casts a stake-weighted vote on a seat election.
func (k Keeper) VoteSeatElection(ctx sdk.Context, msg *types.MsgVoteSeatElection) (*types.MsgVoteSeatElectionResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	prop, found := k.GetSeatElection(ctx, msg.ProposalId)
	if !found {
		return nil, types.ErrSeatElectionNotFound
	}

	// Must be in voting stage.
	if prop.Stage != types.SeatStageVoting {
		return nil, types.ErrSeatElectionNotVoting
	}

	// Must not be past voting end block.
	if currentHeight > prop.VotingEndBlock {
		return nil, types.ErrVotingPeriodEnded
	}

	// Check for double vote.
	if k.HasSeatElectionVoted(ctx, msg.ProposalId, msg.Voter) {
		return nil, types.ErrSeatElectionAlreadyVoted
	}

	// Get voter's bonded stake.
	stake := "0"
	if k.stakingKeeper != nil {
		bonded, err := k.stakingKeeper.GetDelegatorTotalBonded(ctx, msg.Voter)
		if err == nil {
			stake = bonded
		}
	}

	// Record vote.
	vote := &types.SeatElectionVote{
		ProposalId: msg.ProposalId,
		Voter:      msg.Voter,
		Option:     msg.Option,
		Stake:      stake,
		Block:      currentHeight,
	}
	k.SetSeatElectionVote(ctx, vote)

	// Accumulate stake on proposal.
	switch msg.Option {
	case types.VoteYes:
		prop.YesStake = types.AddBigIntStrings(prop.YesStake, stake)
	case types.VoteNo:
		prop.NoStake = types.AddBigIntStrings(prop.NoStake, stake)
	case types.VoteAbstain:
		prop.AbstainStake = types.AddBigIntStrings(prop.AbstainStake, stake)
	}

	k.SetSeatElection(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_election_voted",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", msg.ProposalId)),
			sdk.NewAttribute("voter", msg.Voter),
			sdk.NewAttribute("option", msg.Option),
			sdk.NewAttribute("stake", stake),
		),
	)

	return &types.MsgVoteSeatElectionResponse{EffectiveWeight: stake}, nil
}

// ---------- Tally ----------

// TallySeatElections groups voting-stage elections by seat_index where VotingEndBlock
// has passed, and resolves them. For contested seats (multiple candidates), the
// candidate with the highest yes_stake wins if they have a clear margin (>5%),
// otherwise a runoff is triggered.
func (k Keeper) TallySeatElections(ctx sdk.Context) {
	currentHeight := uint64(ctx.BlockHeight())
	params := k.GetParams(ctx)

	// Collect voting-stage proposals whose voting has ended.
	var readyToTally []*types.SeatElectionProposal
	k.IterateSeatElections(ctx, func(prop *types.SeatElectionProposal) bool {
		if prop.Stage == types.SeatStageVoting && prop.VotingEndBlock > 0 && currentHeight >= prop.VotingEndBlock {
			readyToTally = append(readyToTally, prop)
		}
		return false
	})

	if len(readyToTally) == 0 {
		return
	}

	// Group by seat index.
	bySeat := make(map[uint32][]*types.SeatElectionProposal)
	for _, prop := range readyToTally {
		bySeat[prop.SeatIndex] = append(bySeat[prop.SeatIndex], prop)
	}

	for seatIndex, candidates := range bySeat {
		if len(candidates) == 1 || candidates[0].IsRunoff {
			// Single candidate or runoff: standard quorum + support check.
			for _, prop := range candidates {
				quorumMet, passed := k.checkSeatElectionQuorum(ctx, prop, params)
				if quorumMet && passed {
					prop.Stage = types.SeatStagePassed
					k.SetSeatElection(ctx, prop)
					if err := k.InstallCommunitySeat(ctx, prop.SeatIndex, prop.Candidate, currentHeight); err != nil {
						k.Logger(ctx).Error("failed to install community seat",
							"proposal_id", prop.ProposalId,
							"candidate", prop.Candidate,
							"error", err,
						)
					}
				} else {
					prop.Stage = types.SeatStageFailed
					k.SetSeatElection(ctx, prop)
				}

				ctx.EventManager().EmitEvent(
					sdk.NewEvent(
						"zerone.gov.seat_election_tallied",
						sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", prop.ProposalId)),
						sdk.NewAttribute("outcome", prop.Stage),
						sdk.NewAttribute("yes_stake", prop.YesStake),
						sdk.NewAttribute("no_stake", prop.NoStake),
					),
				)
			}
		} else {
			// Multiple candidates for same seat: rank by yes_stake.
			k.resolveContestedSeat(ctx, seatIndex, candidates, currentHeight)
		}
	}
}

// resolveContestedSeat resolves a contested seat election with multiple candidates.
// If the leading candidate has >5% margin over second place, they win.
// Otherwise, a runoff is triggered between the top two.
func (k Keeper) resolveContestedSeat(ctx sdk.Context, seatIndex uint32, candidates []*types.SeatElectionProposal, currentHeight uint64) {
	// Sort by yes_stake descending (simple selection for top 2).
	var first, second *types.SeatElectionProposal
	firstStake := big.NewInt(0)
	secondStake := big.NewInt(0)

	for _, c := range candidates {
		cStake, ok := new(big.Int).SetString(c.YesStake, 10)
		if !ok {
			cStake = big.NewInt(0)
		}
		if cStake.Cmp(firstStake) > 0 {
			second = first
			secondStake = firstStake
			first = c
			firstStake = cStake
		} else if cStake.Cmp(secondStake) > 0 {
			second = c
			secondStake = cStake
		}
	}

	if first == nil {
		// No candidates with stake — all fail.
		for _, c := range candidates {
			c.Stage = types.SeatStageFailed
			k.SetSeatElection(ctx, c)
		}
		return
	}

	// Check if gap between #1 and #2 > 5% of #1's yes_stake.
	// gap = firstStake - secondStake
	// threshold = firstStake * SeatRunoffThresholdBps / BPSScale
	gap := new(big.Int).Sub(firstStake, secondStake)
	threshold := new(big.Int).Mul(firstStake, big.NewInt(int64(types.SeatRunoffThresholdBps)))
	threshold.Div(threshold, big.NewInt(int64(types.BPSScale)))

	if gap.Cmp(threshold) > 0 {
		// Clear winner — #1 wins, rest fail.
		first.Stage = types.SeatStagePassed
		k.SetSeatElection(ctx, first)
		if err := k.InstallCommunitySeat(ctx, seatIndex, first.Candidate, currentHeight); err != nil {
			k.Logger(ctx).Error("failed to install community seat",
				"proposal_id", first.ProposalId,
				"candidate", first.Candidate,
				"error", err,
			)
		}

		for _, c := range candidates {
			if c.ProposalId != first.ProposalId {
				c.Stage = types.SeatStageFailed
				k.SetSeatElection(ctx, c)
			}
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.gov.seat_election_contested_resolved",
				sdk.NewAttribute("seat_index", fmt.Sprintf("%d", seatIndex)),
				sdk.NewAttribute("winner", first.Candidate),
				sdk.NewAttribute("winner_stake", first.YesStake),
			),
		)
	} else {
		// Runoff needed — mark originals as "runoff" stage, create new runoff proposals.
		k.createRunoff(ctx, seatIndex, first, second, candidates, currentHeight)
	}
}

// createRunoff creates a runoff election between two candidates.
// Marks all original proposals as "runoff" stage, then creates two new
// proposals in "voting" stage with is_runoff=true.
func (k Keeper) createRunoff(ctx sdk.Context, seatIndex uint32, first, second *types.SeatElectionProposal, allCandidates []*types.SeatElectionProposal, currentHeight uint64) {
	// Mark all originals as "runoff" stage.
	var parentIds []uint64
	for _, c := range allCandidates {
		parentIds = append(parentIds, c.ProposalId)
		c.Stage = types.SeatStageRunoff
		k.SetSeatElection(ctx, c)
	}

	// Create runoff proposal for first candidate.
	id1 := k.GetNextSeatElectionID(ctx)
	k.SetNextSeatElectionID(ctx, id1+1)
	runoff1 := &types.SeatElectionProposal{
		ProposalId:     id1,
		Proposer:       first.Proposer,
		Candidate:      first.Candidate,
		SeatIndex:      seatIndex,
		Statement:      first.Statement,
		Stage:          types.SeatStageVoting,
		YesStake:       "0",
		NoStake:        "0",
		AbstainStake:   "0",
		VotingEndBlock: currentHeight + types.SeatVotingBlocks,
		CreatedAtBlock: currentHeight,
		CandidateAccepted: true,
		IsRunoff:       true,
		RunoffParentIds: parentIds,
	}
	k.SetSeatElection(ctx, runoff1)

	// Create runoff proposal for second candidate.
	id2 := k.GetNextSeatElectionID(ctx)
	k.SetNextSeatElectionID(ctx, id2+1)
	runoff2 := &types.SeatElectionProposal{
		ProposalId:     id2,
		Proposer:       second.Proposer,
		Candidate:      second.Candidate,
		SeatIndex:      seatIndex,
		Statement:      second.Statement,
		Stage:          types.SeatStageVoting,
		YesStake:       "0",
		NoStake:        "0",
		AbstainStake:   "0",
		VotingEndBlock: currentHeight + types.SeatVotingBlocks,
		CreatedAtBlock: currentHeight,
		CandidateAccepted: true,
		IsRunoff:       true,
		RunoffParentIds: parentIds,
	}
	k.SetSeatElection(ctx, runoff2)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_election_runoff_created",
			sdk.NewAttribute("seat_index", fmt.Sprintf("%d", seatIndex)),
			sdk.NewAttribute("runoff_proposal_1", fmt.Sprintf("%d", id1)),
			sdk.NewAttribute("runoff_proposal_2", fmt.Sprintf("%d", id2)),
			sdk.NewAttribute("candidate_1", first.Candidate),
			sdk.NewAttribute("candidate_2", second.Candidate),
		),
	)
}

// checkSeatElectionQuorum checks quorum (33.4%) and support (50%) for a seat election.
func (k Keeper) checkSeatElectionQuorum(ctx sdk.Context, prop *types.SeatElectionProposal, params *types.Params) (quorumMet bool, passed bool) {
	yesBig, _ := new(big.Int).SetString(prop.YesStake, 10)
	if yesBig == nil {
		yesBig = big.NewInt(0)
	}
	noBig, _ := new(big.Int).SetString(prop.NoStake, 10)
	if noBig == nil {
		noBig = big.NewInt(0)
	}
	abstainBig, _ := new(big.Int).SetString(prop.AbstainStake, 10)
	if abstainBig == nil {
		abstainBig = big.NewInt(0)
	}

	totalVoted := new(big.Int).Add(yesBig, noBig)
	totalVoted.Add(totalVoted, abstainBig)

	// Get total bonded stake.
	totalBonded := big.NewInt(0)
	if k.stakingKeeper != nil {
		bondedStr, err := k.stakingKeeper.GetTotalBondedStake(ctx)
		if err == nil {
			if tb, ok := new(big.Int).SetString(bondedStr, 10); ok {
				totalBonded = tb
			}
		}
	}

	// Quorum check: (totalVoted * BPSScale) / totalBonded >= quorumThresholdBps
	if totalBonded.Sign() > 0 {
		actualBps := new(big.Int).Mul(totalVoted, big.NewInt(int64(types.BPSScale)))
		actualBps.Div(actualBps, totalBonded)
		quorumMet = actualBps.Uint64() >= params.QuorumThresholdBps
	}

	// Support check: (yesStake * BPSScale) / (yesStake + noStake) >= supportThresholdBps
	yesNoTotal := new(big.Int).Add(yesBig, noBig)
	if yesNoTotal.Sign() > 0 {
		supportBps := new(big.Int).Mul(yesBig, big.NewInt(int64(types.BPSScale)))
		supportBps.Div(supportBps, yesNoTotal)
		passed = quorumMet && supportBps.Uint64() >= params.SupportThresholdBps
	}

	return quorumMet, passed
}

// ---------- Term Management ----------

// InstallCommunitySeat sets a community seat address and term end block.
func (k Keeper) InstallCommunitySeat(ctx sdk.Context, seatIndex uint32, address string, currentHeight uint64) error {
	state := k.GetResearchFundGovernanceState(ctx)

	// Ensure slices are large enough.
	for uint32(len(state.CommunitySeats)) <= seatIndex {
		state.CommunitySeats = append(state.CommunitySeats, "")
	}
	for uint32(len(state.SeatTermEndBlocks)) <= seatIndex {
		state.SeatTermEndBlocks = append(state.SeatTermEndBlocks, 0)
	}

	state.CommunitySeats[seatIndex] = address
	state.SeatTermEndBlocks[seatIndex] = currentHeight + types.SeatTermBlocks

	k.SetResearchFundGovernanceState(ctx, state)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.community_seat_installed",
			sdk.NewAttribute("seat_index", fmt.Sprintf("%d", seatIndex)),
			sdk.NewAttribute("address", address),
			sdk.NewAttribute("term_end_block", fmt.Sprintf("%d", currentHeight+types.SeatTermBlocks)),
		),
	)

	return nil
}

// ExpireSeat clears a community seat at the given index.
func (k Keeper) ExpireSeat(ctx sdk.Context, state *types.ResearchFundGovernanceState, seatIndex uint32) {
	if uint32(len(state.CommunitySeats)) <= seatIndex {
		return
	}

	oldAddr := state.CommunitySeats[seatIndex]
	state.CommunitySeats[seatIndex] = ""
	state.SeatTermEndBlocks[seatIndex] = 0

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.community_seat_expired",
			sdk.NewAttribute("seat_index", fmt.Sprintf("%d", seatIndex)),
			sdk.NewAttribute("previous_holder", oldAddr),
		),
	)
}

// CheckSeatTermExpiry checks all community seats for term expiry and clears them.
func (k Keeper) CheckSeatTermExpiry(ctx sdk.Context) {
	currentHeight := uint64(ctx.BlockHeight())
	state := k.GetResearchFundGovernanceState(ctx)
	changed := false

	for i := uint32(0); i < uint32(len(state.SeatTermEndBlocks)); i++ {
		endBlock := state.SeatTermEndBlocks[i]
		if endBlock > 0 && currentHeight >= endBlock {
			k.ExpireSeat(ctx, state, i)
			changed = true
		}
	}

	if changed {
		k.SetResearchFundGovernanceState(ctx, state)
	}
}

// CheckSeatVacancy emits warning events for vacant community seats.
func (k Keeper) CheckSeatVacancy(ctx sdk.Context) {
	state := k.GetResearchFundGovernanceState(ctx)
	phase := state.CurrentPhase

	maxSeats := maxSeatIndexForPhase(phase)
	if maxSeats == 0 {
		return
	}

	for i := uint32(0); i < maxSeats; i++ {
		if i >= uint32(len(state.CommunitySeats)) || state.CommunitySeats[i] == "" {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"zerone.gov.community_seat_vacant",
					sdk.NewAttribute("seat_index", fmt.Sprintf("%d", i)),
					sdk.NewAttribute("phase", state.CurrentPhase.String()),
				),
			)
		}
	}
}

// ---------- BeginBlocker Helpers ----------

// ProcessSeatElectionExpiry handles automatic stage transitions:
// 1. Auto-expire nominations past acceptance deadline
// 2. Auto-advance discussion → voting when discussion period ends
func (k Keeper) ProcessSeatElectionExpiry(ctx sdk.Context, currentHeight uint64) {
	k.IterateSeatElections(ctx, func(prop *types.SeatElectionProposal) bool {
		changed := false

		// Auto-expire unaccepted nominations.
		if prop.Stage == types.SeatStageNominated && currentHeight > prop.AcceptanceDeadline {
			prop.Stage = types.SeatStageExpired
			changed = true

			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"zerone.gov.seat_nomination_expired",
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", prop.ProposalId)),
					sdk.NewAttribute("candidate", prop.Candidate),
				),
			)
		}

		// Auto-advance discussion → voting.
		if prop.Stage == types.SeatStageDiscussion && prop.DiscussionEndBlock > 0 && currentHeight >= prop.DiscussionEndBlock {
			prop.Stage = types.SeatStageVoting
			prop.VotingEndBlock = currentHeight + types.SeatVotingBlocks
			changed = true

			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"zerone.gov.seat_election_voting_started",
					sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", prop.ProposalId)),
					sdk.NewAttribute("voting_end_block", fmt.Sprintf("%d", prop.VotingEndBlock)),
				),
			)
		}

		if changed {
			k.SetSeatElection(ctx, prop)
		}

		return false
	})
}

// ---------- Helpers ----------

// GetActiveCommunitySeatCount returns the number of filled community seats.
func (k Keeper) GetActiveCommunitySeatCount(ctx sdk.Context) uint32 {
	state := k.GetResearchFundGovernanceState(ctx)
	var count uint32
	for _, seat := range state.CommunitySeats {
		if seat != "" {
			count++
		}
	}
	return count
}

// maxSeatIndexForPhase returns the maximum number of community seats for a given phase.
func maxSeatIndexForPhase(phase types.ResearchFundPhase) uint32 {
	switch phase {
	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		return 1 // 1 community seat in Phase 1
	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED:
		return 3 // 3 community seats in Phase 2
	default:
		return 0 // No community seats in Phase 0 or Phase 3
	}
}
