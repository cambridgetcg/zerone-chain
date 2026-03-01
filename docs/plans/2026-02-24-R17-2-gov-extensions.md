# R17-2: x/gov Extensions — Variable N-of-M Multisig + Phase Tracking

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend x/gov to support phase-aware variable multisig for research fund proposals, distinct voter tracking, and live exit condition monitoring.

**Architecture:** The existing 2-of-2 research spend voting extends to N-of-M (2-of-3 → 3-of-5 → LIP) based on `ResearchFundGovernanceState.CurrentPhase`. Community seat votes are tracked in a separate KV store keyed by `{proposalID}\x00{voter}`. Distinct LIP voters are tracked in an append-only KV store. Cross-module queries (staking, emergency, bank) provide live metrics for exit condition checking.

**Tech Stack:** Go 1.24, Cosmos SDK v0.50.15, existing proto types from R17-1 (no proto regeneration needed)

---

## Task 1: Add Store Keys and Error Codes

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/types/keys.go`
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/types/errors.go`

**Step 1: Add store key prefixes for distinct voters and community seat votes**

In `keys.go`, add after the `ResearchFundGovernanceKey` line (line 22):

```go
DistinctVoterKeyPrefix       = []byte{0x0E}
ResearchCommunityVotePrefix  = []byte{0x0F}
```

Add key helper functions at the bottom of `keys.go`:

```go
// DistinctVoterKey returns the store key for a distinct voter record.
func DistinctVoterKey(voter string) []byte {
	return append(DistinctVoterKeyPrefix, []byte(voter)...)
}

// ResearchCommunityVoteKey returns the key for a community seat vote on a research proposal.
func ResearchCommunityVoteKey(proposalID uint64, voter string) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(proposalID >> 56)
	bz[1] = byte(proposalID >> 48)
	bz[2] = byte(proposalID >> 40)
	bz[3] = byte(proposalID >> 32)
	bz[4] = byte(proposalID >> 24)
	bz[5] = byte(proposalID >> 16)
	bz[6] = byte(proposalID >> 8)
	bz[7] = byte(proposalID)
	key := append(ResearchCommunityVotePrefix, bz...)
	key = append(key, 0x00) // separator
	key = append(key, []byte(voter)...)
	return key
}

// ResearchCommunityVotePrefixForProposal returns the prefix for iterating all community votes on a proposal.
func ResearchCommunityVotePrefixForProposal(proposalID uint64) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(proposalID >> 56)
	bz[1] = byte(proposalID >> 48)
	bz[2] = byte(proposalID >> 40)
	bz[3] = byte(proposalID >> 32)
	bz[4] = byte(proposalID >> 24)
	bz[5] = byte(proposalID >> 16)
	bz[6] = byte(proposalID >> 8)
	bz[7] = byte(proposalID)
	key := append(ResearchCommunityVotePrefix, bz...)
	key = append(key, 0x00)
	return key
}
```

**Step 2: Add error codes**

In `errors.go`, add after `ErrResearchVotersNotSet` (line 25):

```go
ErrInsufficientApprovals    = errors.Register(ModuleName, 22, "insufficient approvals for research spend")
ErrNotResearchFundVoter     = errors.Register(ModuleName, 23, "not an authorized research fund voter for current phase")
ErrPhaseFullGovernance      = errors.Register(ModuleName, 24, "research fund is in full governance phase; use standard LIP process")
```

**Step 3: Run build to verify compilation**

Run: `go build ./x/gov/...`
Expected: PASS (no test needed for type-only changes)

**Step 4: Commit**

```bash
git add x/gov/types/keys.go x/gov/types/errors.go
git commit -m "R17-2: store keys and error codes for phase-aware multisig"
```

---

## Task 2: GetResearchFundThreshold

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/types/types.go`
- Test: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/keeper_test.go`

**Step 1: Write the failing test**

Add to the end of `keeper_test.go`:

```go
// ---------- Research Fund Threshold Tests ----------

func TestGetResearchFundThreshold_GenesisPair(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR)
	if required != 2 || total != 2 {
		t.Errorf("Phase 0: expected 2-of-2, got %d-of-%d", required, total)
	}
}

func TestGetResearchFundThreshold_Observer(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER)
	if required != 2 || total != 3 {
		t.Errorf("Phase 1: expected 2-of-3, got %d-of-%d", required, total)
	}
}

func TestGetResearchFundThreshold_Balanced(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED)
	if required != 3 || total != 5 {
		t.Errorf("Phase 2: expected 3-of-5, got %d-of-%d", required, total)
	}
}

func TestGetResearchFundThreshold_FullGovernance(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE)
	if required != 0 || total != 0 {
		t.Errorf("Phase 3: expected 0-of-0, got %d-of-%d", required, total)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/gov/keeper/ -v -run "GetResearchFundThreshold" -count=1`
Expected: FAIL with "undefined: types.GetResearchFundThreshold"

**Step 3: Write minimal implementation**

Add to `types/types.go` after the `DefaultPhaseExitConditions` map (after line 258):

```go
// GetResearchFundThreshold returns the required approvals and total voters for a phase.
func GetResearchFundThreshold(phase ResearchFundPhase) (required uint32, total uint32) {
	switch phase {
	case ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR:
		return 2, 2
	case ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		return 2, 3
	case ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED:
		return 3, 5
	case ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
		return 0, 0 // not used — standard LIP
	default:
		return 0, 0
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/gov/keeper/ -v -run "GetResearchFundThreshold" -count=1`
Expected: PASS (all 4 tests)

**Step 5: Commit**

```bash
git add x/gov/types/types.go x/gov/keeper/keeper_test.go
git commit -m "R17-2: GetResearchFundThreshold — required/total per phase"
```

---

## Task 3: Phase State Convenience Methods + IncrementProposalsExecuted

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/state.go`
- Test: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/keeper_test.go`

**Step 1: Write the failing tests**

Add to the end of `keeper_test.go`:

```go
// ---------- Phase State Tests ----------

func TestGetResearchFundPhase_Default(t *testing.T) {
	k, ctx := setupKeeper(t)
	phase := k.GetResearchFundPhase(ctx)
	if phase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR {
		t.Errorf("expected default GENESIS_PAIR, got %v", phase)
	}
}

func TestSetResearchFundPhase(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER)
	phase := k.GetResearchFundPhase(ctx)
	if phase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER {
		t.Errorf("expected OBSERVER, got %v", phase)
	}

	// Verify full state is updated correctly.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.CurrentPhase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER {
		t.Error("state.CurrentPhase not updated")
	}
	if state.PhaseStartedAtBlock != 100 {
		t.Errorf("expected phase_started_at_block=100, got %d", state.PhaseStartedAtBlock)
	}
	if state.ProposalsExecutedInPhase != 0 {
		t.Error("expected proposals counter to reset on phase transition")
	}
}

func TestIncrementProposalsExecuted(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initial count should be 0.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.ProposalsExecutedInPhase != 0 {
		t.Errorf("expected 0, got %d", state.ProposalsExecutedInPhase)
	}

	// Increment twice.
	k.IncrementProposalsExecuted(ctx)
	k.IncrementProposalsExecuted(ctx)

	state = k.GetResearchFundGovernanceState(ctx)
	if state.ProposalsExecutedInPhase != 2 {
		t.Errorf("expected 2, got %d", state.ProposalsExecutedInPhase)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/gov/keeper/ -v -run "ResearchFundPhase|IncrementProposalsExecuted" -count=1`
Expected: FAIL with undefined methods

**Step 3: Write minimal implementation**

Add to the bottom of `state.go` (after `GetResearchFundGovernanceState`, before the Genesis section):

```go
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
```

Add `"fmt"` to the imports of `state.go` if not already present.

**Step 4: Run test to verify it passes**

Run: `go test ./x/gov/keeper/ -v -run "ResearchFundPhase|IncrementProposalsExecuted" -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/gov/keeper/state.go x/gov/keeper/keeper_test.go
git commit -m "R17-2: phase state convenience methods + IncrementProposalsExecuted"
```

---

## Task 4: Extend Expected Keepers + Wire Emergency Keeper

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/types/expected_keepers.go`
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/keeper.go`

**Step 1: Extend keeper interfaces**

Add to `expected_keepers.go`:

```go
// CountActiveGuardians method on StakingKeeper (add to existing interface):
// After GetDelegatorTotalBonded line, add:
CountActiveGuardians(ctx context.Context) (uint64, error)
```

Add `GetAllBalances` to the `BankKeeper` interface:

```go
GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
```

Add a new `EmergencyKeeper` interface:

```go
// EmergencyKeeper defines the emergency module interface for governance condition checking.
type EmergencyKeeper interface {
	CountHaltsForReason(ctx context.Context, reason string) uint64
}
```

The full `expected_keepers.go` should look like:

```go
package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// StakingKeeper defines the staking module interface required by governance.
type StakingKeeper interface {
	GetTotalBondedStake(ctx context.Context) (string, error)
	GetDelegatorTotalBonded(ctx context.Context, addr string) (string, error)
	CountActiveGuardians(ctx context.Context) (uint64, error)
}

// BankKeeper defines the bank module interface required by governance.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
}

// VestingRewardsKeeper defines the vesting rewards module interface for research fund disbursement.
type VestingRewardsKeeper interface {
	DisburseFromResearchFund(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coins) error
}

// UpgradeKeeper defines the upgrade module interface for scheduling software upgrades.
type UpgradeKeeper interface {
	ScheduleUpgrade(ctx context.Context, plan *UpgradePlan) error
}

// ParamRouter dispatches parameter changes from passed LIPs to the target module keepers.
type ParamRouter interface {
	ApplyParamChange(ctx context.Context, module, key, value string) error
}

// FundingRecorder is the interface used by the SybilFundingDecorator.
type FundingRecorder interface {
	RecordFunding(ctx sdk.Context, sender, recipient, amount string, blockHeight uint64)
}

// EmergencyKeeper defines the emergency module interface for governance condition checking.
type EmergencyKeeper interface {
	CountHaltsForReason(ctx context.Context, reason string) uint64
}
```

**Step 2: Add emergency keeper field to Keeper struct**

In `keeper.go`, add `emergencyKeeper` field to the struct (after `paramRouter`):

```go
emergencyKeeper types.EmergencyKeeper // set post-init (circular dep break)
```

Add setter method after `SetParamRouter`:

```go
// SetEmergencyKeeper sets the emergency keeper (post-init to break circular deps).
func (k *Keeper) SetEmergencyKeeper(ek types.EmergencyKeeper) {
	k.emergencyKeeper = ek
}
```

**Step 3: Update mock staking keeper in tests**

In `keeper_test.go`, add `CountActiveGuardians` to the mock:

```go
func (m *mockStakingKeeper) CountActiveGuardians(_ context.Context) (uint64, error) {
	return 0, nil
}
```

Also update the mock bank keeper if one exists (or handle nil gracefully).

**Step 4: Run build to verify compilation**

Run: `go build ./x/gov/...`
Expected: PASS

**Step 5: Run existing tests to verify no regressions**

Run: `go test ./x/gov/keeper/ -v -count=1 -timeout 120s`
Expected: All existing tests PASS

**Step 6: Commit**

```bash
git add x/gov/types/expected_keepers.go x/gov/keeper/keeper.go x/gov/keeper/keeper_test.go
git commit -m "R17-2: extend keeper interfaces — CountActiveGuardians, GetAllBalances, EmergencyKeeper"
```

---

## Task 5: Distinct Voter Tracking

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/state.go`
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/msg_server.go`
- Test: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/keeper_test.go`

**Step 1: Write the failing test**

Add to the end of `keeper_test.go`:

```go
// ---------- Distinct Voter Tracking Tests ----------

func TestDistinctVoterTracking_RecordAndCount(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initially zero.
	count := k.CountDistinctVoters(ctx)
	if count != 0 {
		t.Errorf("expected 0 distinct voters, got %d", count)
	}

	// Record three distinct voters.
	k.RecordDistinctVoter(ctx, testAddr("alice"))
	k.RecordDistinctVoter(ctx, testAddr("bob"))
	k.RecordDistinctVoter(ctx, testAddr("charlie"))

	count = k.CountDistinctVoters(ctx)
	if count != 3 {
		t.Errorf("expected 3 distinct voters, got %d", count)
	}
}

func TestDistinctVoterTracking_Deduplication(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Record same voter twice.
	k.RecordDistinctVoter(ctx, testAddr("alice"))
	k.RecordDistinctVoter(ctx, testAddr("alice"))

	count := k.CountDistinctVoters(ctx)
	if count != 1 {
		t.Errorf("expected 1 (deduped), got %d", count)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/gov/keeper/ -v -run "DistinctVoter" -count=1`
Expected: FAIL with "undefined: k.RecordDistinctVoter"

**Step 3: Write minimal implementation**

Add to `state.go` (in the Research Fund Governance section):

```go
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
```

**Step 4: Hook into CastVote handler**

In `msg_server.go`, in the `CastVote` method, after `ms.SetVote(ctx, vote)` (line 259), add:

```go
	// Track distinct governance participants for phase exit conditions.
	ms.RecordDistinctVoter(ctx, msg.Voter)
```

**Step 5: Run test to verify it passes**

Run: `go test ./x/gov/keeper/ -v -run "DistinctVoter" -count=1`
Expected: PASS

**Step 6: Run full test suite to verify no regressions**

Run: `go test ./x/gov/keeper/ -v -count=1 -timeout 120s`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add x/gov/keeper/state.go x/gov/keeper/msg_server.go x/gov/keeper/keeper_test.go
git commit -m "R17-2: distinct voter tracking — append-only KV store, hooked into CastVote"
```

---

## Task 6: Research Fund Balance Query

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/research_spend.go`
- Test: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/keeper_test.go`

**Step 1: Write the failing test**

Add to `keeper_test.go`:

```go
// ---------- Research Fund Balance Tests ----------

func TestGetResearchFundBalance_NilBankKeeper(t *testing.T) {
	k, ctx := setupKeeper(t) // bankKeeper is nil
	balance := k.GetResearchFundBalance(ctx)
	if !balance.IsZero() {
		t.Errorf("expected zero balance with nil bank keeper, got %s", balance)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/gov/keeper/ -v -run "GetResearchFundBalance" -count=1`
Expected: FAIL with undefined

**Step 3: Write minimal implementation**

Add to `research_spend.go` after the `ProcessResearchSpendExpiry` function:

```go
// GetResearchFundBalance returns the research fund module account balance.
func (k Keeper) GetResearchFundBalance(ctx sdk.Context) sdk.Coins {
	if k.bankKeeper == nil {
		return sdk.NewCoins()
	}
	return k.bankKeeper.GetAllBalances(ctx, authtypes.NewModuleAddress("research_fund"))
}
```

Add to imports in `research_spend.go`:

```go
authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/gov/keeper/ -v -run "GetResearchFundBalance" -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/gov/keeper/research_spend.go x/gov/keeper/keeper_test.go
git commit -m "R17-2: GetResearchFundBalance — query research fund module balance"
```

---

## Task 7: Phase-Aware Multisig Voting

This is the core change. The existing `VoteResearchSpend` and `SubmitResearchSpend` need phase-awareness.

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/research_spend.go`
- Test: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/research_spend_test.go`

**Step 1: Write the failing tests**

Add to `research_spend_test.go`:

```go
// ---------- Phase-Aware Multisig Tests ----------

func TestResearchSpend_PhaseFullGovernance_RejectsMultisig(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Set phase to full governance.
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE)

	// Submit should fail — full governance uses standard LIP path.
	_, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Should fail in full governance",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err == nil {
		t.Error("expected error in full governance phase")
	}
}

func TestResearchSpend_PhaseGenesisPair_2of2(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Default is phase 0 (genesis pair) — existing 2-of-2 behavior.
	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Phase 0 test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Both vote yes.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: resp.ProposalId, Vote: "yes",
	})

	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed, got %s", prop.Stage)
	}

	// Verify proposals executed counter incremented.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.ProposalsExecutedInPhase != 1 {
		t.Errorf("expected ProposalsExecutedInPhase=1, got %d", state.ProposalsExecutedInPhase)
	}
}

func TestResearchSpend_PhaseObserver_2of3(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Set phase to Observer (2-of-3) with one community seat.
	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Phase 1 test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter1 votes yes — 1-of-3, not enough.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not execute with only 1-of-3 approvals")
	}

	// Voter2 votes yes — 2-of-3, should execute.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed with 2-of-3, got %s", prop.Stage)
	}
}

func TestResearchSpend_PhaseObserver_CommunityVoterCanVote(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Set phase to Observer (2-of-3) with one community seat.
	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, _ := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Community voter test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Community voter votes yes.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	if err != nil {
		t.Fatalf("community voter should be able to vote: %v", err)
	}

	// Voter1 votes yes — 2-of-3, should execute.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed with voter1+community1 (2-of-3), got %s", prop.Stage)
	}
}

func TestResearchSpend_NonDesignatedVoter_PhaseBased(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Set phase to Observer with specific community seats.
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{testAddr("community1")}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, _ := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Non-voter test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Random outsider tries to vote — should fail.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: testAddr("outsider"), ProposalId: resp.ProposalId, Vote: "yes",
	})
	if err == nil {
		t.Error("expected error for non-designated voter")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/gov/keeper/ -v -run "Phase|CommunityVoter|NonDesignatedVoter_Phase" -count=1`
Expected: FAIL — new phase-aware logic not yet implemented

**Step 3: Implement phase-aware multisig**

The implementation involves these changes in `research_spend.go`:

**3a.** Add `isDesignatedResearchVoter` helper:

```go
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
```

**3b.** Add community seat vote CRUD:

```go
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

// countResearchSpendApprovals counts all "yes" votes on a research proposal.
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

// CountCommunitySeatVotes counts how many community seat holders have voted
// on any research proposal in the current phase.
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
```

**3c.** Modify `SubmitResearchSpend` — add full governance phase check at the top, after voters check:

After the line `if msg.Proposer != voters.Voter1 && msg.Proposer != voters.Voter2 {` block (line 130-132), replace the proposer validation with phase-aware logic:

```go
	// Phase 3 (full governance) does not use multisig — reject.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.CurrentPhase == types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE {
		return nil, types.ErrPhaseFullGovernance
	}

	// Only designated voters (voter1, voter2, community seats) can submit.
	if !k.isDesignatedResearchVoter(ctx, msg.Proposer) {
		return nil, types.ErrNotDesignatedVoter
	}
```

**3d.** Modify `VoteResearchSpend` — make phase-aware:

Replace the designated voter check (lines 202-205) with:

```go
	// Only designated voters for current phase can vote.
	if !k.isDesignatedResearchVoter(ctx, msg.Voter) {
		return nil, types.ErrNotDesignatedVoter
	}
```

After the voter slot detection (the `isVoter1` block, lines 229-244), add community seat vote handling:

```go
	// Community seat voter handling.
	isVoter1 := voters != nil && msg.Voter == voters.Voter1
	isVoter2 := voters != nil && msg.Voter == voters.Voter2

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
```

Replace the immediate resolution logic (lines 246-253) with phase-aware logic:

```go
	// Check for immediate resolution — phase-aware.
	state := k.GetResearchFundGovernanceState(ctx)
	if prop.Voter1Vote == "no" || prop.Voter2Vote == "no" {
		// Any core voter NO → rejected immediately.
		prop.Stage = string(types.ResearchStageRejected)
	} else {
		required, _ := types.GetResearchFundThreshold(state.CurrentPhase)
		approvals := k.countResearchSpendApprovals(ctx, prop)
		if approvals >= required {
			k.executeResearchSpend(ctx, prop)
			if prop.Stage == string(types.ResearchStageExecuted) {
				k.IncrementProposalsExecuted(ctx)
			}
		}
	}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./x/gov/keeper/ -v -run "Phase|CommunityVoter|NonDesignatedVoter_Phase" -count=1`
Expected: PASS

**Step 5: Run full test suite to verify no regressions**

Run: `go test ./x/gov/keeper/ -v -count=1 -timeout 120s`
Expected: All tests PASS (including existing Phase 0 tests)

**Step 6: Commit**

```bash
git add x/gov/keeper/research_spend.go x/gov/keeper/research_spend_test.go
git commit -m "R17-2: phase-aware N-of-M multisig voting — 2-of-2 → 2-of-3 → 3-of-5 → LIP"
```

---

## Task 8: CheckPhaseExitConditions

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/research_spend.go`
- Test: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/keeper_test.go`

**Step 1: Write the failing test**

Add to `keeper_test.go`:

```go
// ---------- Mock Emergency Keeper ----------

type mockEmergencyKeeper struct {
	halts map[string]uint64
}

func (m *mockEmergencyKeeper) CountHaltsForReason(_ context.Context, reason string) uint64 {
	if m.halts != nil {
		return m.halts[reason]
	}
	return 0
}

// ---------- Phase Exit Condition Tests ----------

func TestCheckPhaseExitConditions_GenesisPair(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Wire mocks.
	mockSK.guardianCount = 5
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	// Record 10 distinct voters.
	for i := 0; i < 10; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.DistinctLipVoters != 10 {
		t.Errorf("expected 10 distinct voters, got %d", conditions.DistinctLipVoters)
	}
	if conditions.ActiveGuardians != 5 {
		t.Errorf("expected 5 active guardians, got %d", conditions.ActiveGuardians)
	}
	if conditions.ChainAgeBlocks != 100 {
		t.Errorf("expected chain age 100, got %d", conditions.ChainAgeBlocks)
	}

	// Should not be all met — chain age is only 100, needs 2,200,000.
	if allMet {
		t.Error("conditions should NOT all be met (chain age too low)")
	}
}
```

Add `guardianCount` to the mock staking keeper:

```go
type mockStakingKeeper struct {
	totalBonded    string
	delegations    map[string]string
	guardianCount  uint64
}

func (m *mockStakingKeeper) CountActiveGuardians(_ context.Context) (uint64, error) {
	return m.guardianCount, nil
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/gov/keeper/ -v -run "CheckPhaseExitConditions" -count=1`
Expected: FAIL with undefined

**Step 3: Write minimal implementation**

Add to `research_spend.go`:

```go
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
		DistinctLipVoters:         k.CountDistinctVoters(ctx),
		ActiveGuardians:           activeGuardians,
		ResearchFundBalance:       researchFundBalance,
		ChainAgeBlocks:            uint64(ctx.BlockHeight()),
		ProposalsExecutedInPhase:  state.ProposalsExecutedInPhase,
		CommunitySeatParticipation: k.CountCommunitySeatVotes(ctx, state),
		EmergencyHaltsFromMisuse:  emergencyHalts,
	}

	// Check against exit conditions for current phase.
	exitConditions, exists := types.DefaultPhaseExitConditions[state.CurrentPhase]
	if !exists {
		// Full governance or unspecified — no conditions.
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/gov/keeper/ -v -run "CheckPhaseExitConditions" -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/gov/keeper/research_spend.go x/gov/keeper/keeper_test.go
git commit -m "R17-2: CheckPhaseExitConditions — live maturity metric gathering"
```

---

## Task 9: GRPC Query Enhancement

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/keeper/grpc_query.go`

**Step 1: Enhance the ResearchFundGovernance query**

Replace the existing `ResearchFundGovernance` handler (lines 177-194 of `grpc_query.go`) with:

```go
// ResearchFundGovernance returns the current governance state and a live snapshot of exit conditions.
func (qs *queryServer) ResearchFundGovernance(goCtx context.Context, _ *types.QueryResearchFundGovernanceRequest) (*types.QueryResearchFundGovernanceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	state := qs.GetResearchFundGovernanceState(ctx)
	conditions, _ := qs.CheckPhaseExitConditions(ctx)

	return &types.QueryResearchFundGovernanceResponse{
		State:             state,
		CurrentConditions: conditions,
	}, nil
}
```

**Step 2: Run build to verify compilation**

Run: `go build ./x/gov/...`
Expected: PASS

**Step 3: Run existing tests to verify no regressions**

Run: `go test ./x/gov/keeper/ -v -count=1 -timeout 120s`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add x/gov/keeper/grpc_query.go
git commit -m "R17-2: GRPC query — populate all exit condition metrics"
```

---

## Task 10: CLI Command

**Files:**
- Modify: `/Users/yournameisai/Desktop/zerone/x/gov/client/cli/query.go`

**Step 1: Add the CLI command**

Add `NewQueryResearchFundGovernanceCmd()` to the `queryCmd.AddCommand(...)` call:

```go
NewQueryResearchFundGovernanceCmd(),
```

Add the command function at the end of `query.go`:

```go
// NewQueryResearchFundGovernanceCmd returns the command to query research fund governance state.
func NewQueryResearchFundGovernanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research-fund-governance",
		Short: "Query the research fund governance phase, state, and exit conditions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryResearchFundGovernanceRequest{}
			resp := &types.QueryResearchFundGovernanceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.gov.v1.Query/ResearchFundGovernance", req, resp); err != nil {
				return fmt.Errorf("failed to query research fund governance: %w", err)
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 2: Run build to verify compilation**

Run: `go build ./x/gov/...`
Expected: PASS

**Step 3: Commit**

```bash
git add x/gov/client/cli/query.go
git commit -m "R17-2: CLI — zeroned query gov research-fund-governance"
```

---

## Task 11: Build Verification + Full Test Suite

**Step 1: Full build**

Run: `go build ./x/gov/...`
Expected: PASS

**Step 2: Full test suite**

Run: `go test ./x/gov/keeper/ -v -count=1 -timeout 120s`
Expected: All tests PASS (existing + new)

**Step 3: Targeted test suites per spec**

Run: `go test ./x/gov/keeper/ -v -run "ResearchFundPhase" -count=1`
Run: `go test ./x/gov/keeper/ -v -run "PhaseExitConditions" -count=1`
Run: `go test ./x/gov/keeper/ -v -run "GetResearchFundThreshold" -count=1`
Expected: All PASS

---

## Task 12: Staking Adapter — CountActiveGuardians

The `StakingKeeper` interface now requires `CountActiveGuardians`. This needs to be implemented on the adapter that gov module uses.

**Files:**
- Check: `/Users/yournameisai/Desktop/zerone/app/app.go` — how gov keeper's staking keeper is wired
- Potentially modify: staking adapter or create a gov-specific adapter

**Step 1: Determine wiring**

Check `app.go` to see what concrete type satisfies `gov.types.StakingKeeper`. It may be a direct keeper reference or an adapter. If needed, create a `GovStakingAdapter` in `x/staking/keeper/` similar to `EmergencyStakingAdapter`.

**Step 2: Implement CountActiveGuardians**

If using an adapter pattern, add:

```go
func (a *GovStakingAdapter) CountActiveGuardians(ctx context.Context) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var count uint64
	a.k.IterateValidators(sdkCtx, func(val *types.Validator) bool {
		if val.Tier == types.TierGuardian && val.IsActive {
			count++
		}
		return false
	})
	return count, nil
}
```

**Step 3: Implement CountHaltsForReason on emergency keeper**

Add to emergency keeper (or an adapter):

```go
func (k Keeper) CountHaltsForReason(ctx context.Context, reason string) uint64 {
	// Count finalized halt ceremonies. This is a simple approximation —
	// iterate ceremonies and count those with status finalized.
	var count uint64
	k.IterateCeremonies(ctx, func(c *types.EmergencyCeremony) bool {
		if c.Type == "halt" && c.Status == "finalized" {
			count++
		}
		return false
	})
	return count
}
```

**Step 4: Wire in app.go**

Ensure `govKeeper.SetEmergencyKeeper(emergencyKeeper)` is called in `app.go`.

**Step 5: Build and test**

Run: `go build ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add -A
git commit -m "R17-2: staking/emergency adapters — CountActiveGuardians, CountHaltsForReason"
```

---

## Task 13: Final Commit

After all tasks pass:

```bash
git add -A
git commit -m "R17-2: x/gov — variable N-of-M multisig + phase tracking

- ResearchFundGovernanceState stored on-chain (phase, proposals, seats)
- Phase-aware ResearchSpendProposal execution (2-of-2 → 2-of-3 → 3-of-5 → LIP)
- GetResearchFundThreshold() returns required/total per phase
- CheckPhaseExitConditions() validates on-chain maturity metrics
- Distinct voter tracking (append-only KV store)
- GRPC query: current phase + exit condition status
- CLI: zeroned query gov research-fund-governance"
```

---

## Implementation Notes

**What was NOT changed (intentional):**
- No proto regeneration — R17-1 proto types are sufficient
- No changes to `module.go` — no new message types or query types needed
- The `QueryResearchFundGovernanceResponse` proto has `state` + `current_conditions` (no `all_conditions_met` or `next_phase` fields in proto) — the CLI can derive these from the response

**Cross-module dependencies:**
- `StakingKeeper.CountActiveGuardians` — new method, needs adapter
- `EmergencyKeeper.CountHaltsForReason` — new interface + method
- `BankKeeper.GetAllBalances` — existing SDK method, just needs interface addition

**Circular dependency handling:**
- `emergencyKeeper` is set post-init via `SetEmergencyKeeper()`, same pattern as `vestingKeeper`
