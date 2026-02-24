package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- LIP CRUD ----------

// SetLIP stores a LIP in the KV store.
func (k Keeper) SetLIP(ctx sdk.Context, lip *types.LIP) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(lip)
	if err != nil {
		panic(err)
	}
	store.Set(types.LIPKey(lip.Id), bz)
}

// GetLIP retrieves a LIP by its ID.
func (k Keeper) GetLIP(ctx sdk.Context, lipID string) (*types.LIP, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.LIPKey(lipID))
	if bz == nil {
		return nil, false
	}
	var lip types.LIP
	if err := json.Unmarshal(bz, &lip); err != nil {
		panic(err)
	}
	return &lip, true
}

// DeleteLIP removes a LIP from the store.
func (k Keeper) DeleteLIP(ctx sdk.Context, lipID string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.LIPKey(lipID))
}

// IterateLIPs iterates over all LIPs. Return true from cb to stop.
func (k Keeper) IterateLIPs(ctx sdk.Context, cb func(lip *types.LIP) bool) {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.LIPKeyPrefix)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var lip types.LIP
		if err := json.Unmarshal(iter.Value(), &lip); err != nil {
			panic(err)
		}
		if cb(&lip) {
			break
		}
	}
}

// GetLIPsByStatus returns all LIPs with the given stage.
func (k Keeper) GetLIPsByStatus(ctx sdk.Context, status string) []*types.LIP {
	var result []*types.LIP
	k.IterateLIPs(ctx, func(lip *types.LIP) bool {
		if lip.Stage == status {
			result = append(result, lip)
		}
		return false
	})
	return result
}

// ---------- Vote CRUD ----------

// SetVote stores a vote in the KV store.
func (k Keeper) SetVote(ctx sdk.Context, vote *types.Vote) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(vote)
	if err != nil {
		panic(err)
	}
	store.Set(types.VoteKey(vote.LipId, vote.Voter), bz)
	// Set dedupe key
	store.Set(types.VoteDedupeKey(vote.LipId, vote.Voter), []byte{1})
}

// GetVote retrieves a vote by lip_id and voter.
func (k Keeper) GetVote(ctx sdk.Context, lipID, voter string) (*types.Vote, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.VoteKey(lipID, voter))
	if bz == nil {
		return nil, false
	}
	var vote types.Vote
	if err := json.Unmarshal(bz, &vote); err != nil {
		panic(err)
	}
	return &vote, true
}

// HasVoted checks if a voter has already voted on a LIP.
func (k Keeper) HasVoted(ctx sdk.Context, lipID, voter string) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.VoteDedupeKey(lipID, voter))
}

// DeleteVote removes a vote from the store.
func (k Keeper) DeleteVote(ctx sdk.Context, lipID, voter string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.VoteKey(lipID, voter))
	store.Delete(types.VoteDedupeKey(lipID, voter))
}

// GetVotesForLIP returns all votes for a given LIP.
func (k Keeper) GetVotesForLIP(ctx sdk.Context, lipID string) []*types.Vote {
	store := ctx.KVStore(k.storeKey)
	prefix := types.VotePrefixForLIP(lipID)
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()

	var result []*types.Vote
	for ; iter.Valid(); iter.Next() {
		var vote types.Vote
		if err := json.Unmarshal(iter.Value(), &vote); err != nil {
			panic(err)
		}
		result = append(result, &vote)
	}
	return result
}

// GetAllVotes returns all votes in the store.
func (k Keeper) GetAllVotes(ctx sdk.Context) []*types.Vote {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.VoteKeyPrefix)
	defer iter.Close()

	var result []*types.Vote
	for ; iter.Valid(); iter.Next() {
		var vote types.Vote
		if err := json.Unmarshal(iter.Value(), &vote); err != nil {
			panic(err)
		}
		result = append(result, &vote)
	}
	return result
}

// ---------- Params CRUD ----------

// SetParams stores governance parameters.
func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(params)
	if err != nil {
		panic(err)
	}
	store.Set(types.ParamsKey, bz)
}

// GetParams retrieves governance parameters.
func (k Keeper) GetParams(ctx sdk.Context) *types.Params {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := json.Unmarshal(bz, &params); err != nil {
		panic(err)
	}
	return &params
}

// ---------- LIP Counter ----------

// GetNextLIPNumber returns the next LIP number.
func (k Keeper) GetNextLIPNumber(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.LIPCounterKey)
	if bz == nil {
		return 1
	}
	return binary.BigEndian.Uint64(bz)
}

// SetNextLIPNumber sets the next LIP number.
func (k Keeper) SetNextLIPNumber(ctx sdk.Context, n uint64) {
	store := ctx.KVStore(k.storeKey)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, n)
	store.Set(types.LIPCounterKey, bz)
}

// ---------- Upgrade Plan CRUD ----------

// SetUpgradePlan stores an upgrade plan associated with a LIP.
func (k Keeper) SetUpgradePlan(ctx sdk.Context, lipID string, plan *types.UpgradePlan) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(plan)
	if err != nil {
		panic("failed to marshal upgrade plan: " + err.Error())
	}
	store.Set(types.UpgradePlanKey(lipID), bz)
}

// GetUpgradePlan retrieves the upgrade plan for a LIP.
func (k Keeper) GetUpgradePlan(ctx sdk.Context, lipID string) (*types.UpgradePlan, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.UpgradePlanKey(lipID))
	if bz == nil {
		return nil, false
	}
	var plan types.UpgradePlan
	if err := json.Unmarshal(bz, &plan); err != nil {
		return nil, false
	}
	return &plan, true
}

// DeleteUpgradePlan removes the upgrade plan for a LIP.
func (k Keeper) DeleteUpgradePlan(ctx sdk.Context, lipID string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.UpgradePlanKey(lipID))
}

// IterateUpgradePlans iterates over all stored upgrade plans. Return true from cb to stop.
func (k Keeper) IterateUpgradePlans(ctx sdk.Context, cb func(lipID string, plan *types.UpgradePlan) bool) {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.UpgradePlanKeyPrefix)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		lipID := string(iter.Key()[len(types.UpgradePlanKeyPrefix):])
		var plan types.UpgradePlan
		if err := json.Unmarshal(iter.Value(), &plan); err != nil {
			panic("failed to unmarshal upgrade plan: " + err.Error())
		}
		if cb(lipID, &plan) {
			break
		}
	}
}

// ---------- Research Fund Governance State ----------

// SetResearchFundGovernanceState stores the research fund governance state.
func (k Keeper) SetResearchFundGovernanceState(ctx sdk.Context, state *types.ResearchFundGovernanceState) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(state)
	if err != nil {
		panic(err)
	}
	store.Set(types.ResearchFundGovernanceKey, bz)
}

// GetResearchFundGovernanceState retrieves the research fund governance state.
func (k Keeper) GetResearchFundGovernanceState(ctx sdk.Context) *types.ResearchFundGovernanceState {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ResearchFundGovernanceKey)
	if bz == nil {
		return types.DefaultResearchFundGovernanceState()
	}
	var state types.ResearchFundGovernanceState
	if err := json.Unmarshal(bz, &state); err != nil {
		panic(err)
	}
	return &state
}

// GetResearchFundPhase returns the current research fund governance phase.
func (k Keeper) GetResearchFundPhase(ctx sdk.Context) types.ResearchFundPhase {
	state := k.GetResearchFundGovernanceState(ctx)
	return state.CurrentPhase
}

// SetResearchFundPhase stores the current phase, resets the proposals counter,
// records the transition block, and emits a transition event.
func (k Keeper) SetResearchFundPhase(ctx sdk.Context, phase types.ResearchFundPhase) {
	state := k.GetResearchFundGovernanceState(ctx)
	oldPhase := state.CurrentPhase
	state.CurrentPhase = phase
	state.PhaseStartedAtBlock = uint64(ctx.BlockHeight())
	state.LastTransitionBlock = uint64(ctx.BlockHeight())
	state.ProposalsExecutedInPhase = 0
	k.SetResearchFundGovernanceState(ctx, state)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.research_fund_phase_transition",
			sdk.NewAttribute("from_phase", oldPhase.String()),
			sdk.NewAttribute("to_phase", phase.String()),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)
}

// IncrementProposalsExecuted increments the executed proposal counter for the current phase.
func (k Keeper) IncrementProposalsExecuted(ctx sdk.Context) {
	state := k.GetResearchFundGovernanceState(ctx)
	state.ProposalsExecutedInPhase++
	k.SetResearchFundGovernanceState(ctx, state)
}

// ---------- Distinct Voter Tracking ----------

// RecordDistinctVoter records a unique governance participant. Append-only:
// once a voter is recorded, they are counted forever.
func (k Keeper) RecordDistinctVoter(ctx sdk.Context, voter string) {
	store := ctx.KVStore(k.storeKey)
	key := types.DistinctVoterKey(voter)
	if store.Has(key) {
		return
	}
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(ctx.BlockHeight()))
	store.Set(key, bz)
}

// CountDistinctVoters iterates the distinct voter prefix and counts entries.
func (k Keeper) CountDistinctVoters(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.DistinctVoterKeyPrefix)
	defer iter.Close()

	var count uint64
	for ; iter.Valid(); iter.Next() {
		count++
	}
	return count
}

// ---------- Genesis ----------

// InitGenesis initializes the module's state from a genesis state.
func (k Keeper) InitGenesis(ctx sdk.Context, gs *types.GenesisState) {
	k.SetParams(ctx, gs.Params)
	k.SetNextLIPNumber(ctx, gs.NextLipNumber)

	for _, lip := range gs.Lips {
		k.SetLIP(ctx, lip)
	}
	for _, vote := range gs.Votes {
		k.SetVote(ctx, vote)
	}

	for _, gup := range gs.UpgradePlans {
		k.SetUpgradePlan(ctx, gup.LipId, gup.Plan)
	}

	// Restore research fund voters from params if set.
	if gs.Params != nil && gs.Params.ResearchFundVoters != nil {
		v := gs.Params.ResearchFundVoters
		if v.Voter1 != "" && v.Voter2 != "" {
			k.SetResearchFundVoters(ctx, v)
		}
	}

	// Restore research fund governance state.
	if gs.ResearchFundGovernance != nil {
		k.SetResearchFundGovernanceState(ctx, gs.ResearchFundGovernance)
	} else {
		k.SetResearchFundGovernanceState(ctx, types.DefaultResearchFundGovernanceState())
	}
}

// ExportGenesis exports the module's current state as a genesis state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	var allLIPs []*types.LIP
	k.IterateLIPs(ctx, func(lip *types.LIP) bool {
		allLIPs = append(allLIPs, lip)
		return false
	})

	params := k.GetParams(ctx)

	// Export research voters into params.
	voters := k.GetResearchFundVoters(ctx)
	if voters != nil {
		params.ResearchFundVoters = voters
	}

	var upgradePlans []*types.GenesisUpgradePlan
	k.IterateUpgradePlans(ctx, func(lipID string, plan *types.UpgradePlan) bool {
		upgradePlans = append(upgradePlans, &types.GenesisUpgradePlan{
			LipId: lipID,
			Plan:  plan,
		})
		return false
	})

	return &types.GenesisState{
		Params:                 params,
		Lips:                   allLIPs,
		Votes:                  k.GetAllVotes(ctx),
		NextLipNumber:          k.GetNextLIPNumber(ctx),
		UpgradePlans:           upgradePlans,
		ResearchFundGovernance: k.GetResearchFundGovernanceState(ctx),
	}
}
