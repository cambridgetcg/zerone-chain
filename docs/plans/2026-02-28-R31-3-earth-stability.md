# R31-3 Earth Stability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire governance (Earth) into the Wu Xing cycle — expedited LIP voting during alignment stress (Wood→Earth) and domain formation freezes (Earth→Water).

**Architecture:** Three connections: (1) verify existing Earth→Metal param change flow via test, (2) alignment health feeds into LIP voting period calculation, (3) new `MsgDomainFormationFreeze` governance message controls partnership formation per-domain.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, protobuf

---

### Task 1: Earth→Metal — Verify param change flow (test only)

**Files:**
- Create: `x/gov/keeper/earth_metal_test.go`

**Step 1: Write the test**

This test confirms that a governance LIP changing capture_defense's `hhi_threshold` param correctly takes effect after the LIP passes. Uses the existing param router flow.

```go
package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// TestEarthMetal_ParamChangeLIP_UpdatesCaptureDefense verifies that a
// governance LIP targeting capture_defense hhi_threshold correctly
// applies the param change when the LIP passes.
func TestEarthMetal_ParamChangeLIP_UpdatesCaptureDefense(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	// Wire a mock param router that records applied changes.
	mockPR := &mockParamRouter{}
	k.SetParamRouter(mockPR)

	// Submit a parameter LIP targeting capture_defense hhi_threshold.
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Capture Defense Threshold",
		Description:  "Lower HHI threshold to catch more capture risk",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "capture_defense", Key: "hhi_threshold", Value: "200000"},
		},
	})
	if err != nil {
		t.Fatalf("submit LIP failed: %v", err)
	}

	// Fast-forward to voting stage.
	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	// Cast majority yes vote.
	mock.delegations[testAddr("voter1")] = "500000"
	_, err = ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: resp.LipId, Option: types.VoteYes,
	})
	if err != nil {
		t.Fatalf("cast vote failed: %v", err)
	}

	// Tally via BeginBlocker.
	k.BeginBlocker(ctx)

	// LIP should pass.
	lip, _ = k.GetLIP(ctx, resp.LipId)
	if lip.Stage != types.StatusPassed {
		t.Fatalf("expected LIP passed, got %s", lip.Stage)
	}

	// Param router should have received the change.
	if len(mockPR.applied) != 1 {
		t.Fatalf("expected 1 param change applied, got %d", len(mockPR.applied))
	}
	if mockPR.applied[0].module != "capture_defense" {
		t.Errorf("expected module=capture_defense, got %s", mockPR.applied[0].module)
	}
	if mockPR.applied[0].key != "hhi_threshold" {
		t.Errorf("expected key=hhi_threshold, got %s", mockPR.applied[0].key)
	}
	if mockPR.applied[0].value != "200000" {
		t.Errorf("expected value=200000, got %s", mockPR.applied[0].value)
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -run TestEarthMetal -v`
Expected: PASS

**Step 3: Commit**

```bash
git add x/gov/keeper/earth_metal_test.go
git commit -m "test(gov): verify Earth→Metal param change flow for capture_defense (R31-3)"
```

---

### Task 2: Wood→Earth — Add GetHealthCategory to alignment keeper

**Files:**
- Modify: `x/alignment/keeper/state.go` (add convenience method)

**Step 1: Write the failing test**

Create `x/alignment/keeper/health_category_test.go`:

```go
package keeper_test

import (
	"testing"

	alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
)

func TestGetHealthCategory_ReturnsCategoryFromLatestIndex(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Store a health index at height 100 with "degraded" category.
	k.SetHealthIndex(ctx, &alignmenttypes.AlignmentHealthIndex{
		Height:         100,
		CompositeScore: 300000,
		Category:       alignmenttypes.CategoryDegraded,
	})

	got := k.GetHealthCategory(ctx)
	if got != alignmenttypes.CategoryDegraded {
		t.Errorf("expected %q, got %q", alignmenttypes.CategoryDegraded, got)
	}
}

func TestGetHealthCategory_ReturnsHealthyWhenNoData(t *testing.T) {
	k, ctx := setupKeeper(t)

	got := k.GetHealthCategory(ctx)
	if got != alignmenttypes.CategoryHealthy {
		t.Errorf("expected %q when no health index exists, got %q", alignmenttypes.CategoryHealthy, got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -run TestGetHealthCategory -v`
Expected: FAIL — `k.GetHealthCategory undefined`

**Step 3: Write minimal implementation**

Add to `x/alignment/keeper/state.go` (after `GetLastObservationHeight`):

```go
// GetHealthCategory returns the health category from the most recent health index.
// Returns "healthy" if no health index has been recorded yet.
func (k Keeper) GetHealthCategory(ctx context.Context) string {
	indices := k.GetRecentHealthIndices(ctx, 1)
	if len(indices) == 0 {
		return types.CategoryHealthy
	}
	return indices[0].Category
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -run TestGetHealthCategory -v`
Expected: PASS

**Step 5: Commit**

```bash
git add x/alignment/keeper/state.go x/alignment/keeper/health_category_test.go
git commit -m "feat(alignment): add GetHealthCategory convenience method (R31-3)"
```

---

### Task 3: Wood→Earth — Add AlignmentKeeper interface and expedited voting logic to gov

**Files:**
- Modify: `x/gov/types/expected_keepers.go` (add AlignmentKeeper interface)
- Modify: `x/gov/keeper/keeper.go` (add field + setter)
- Create: `x/gov/keeper/expedited.go` (expedited voting logic)

**Step 1: Write the failing test**

Create `x/gov/keeper/expedited_test.go`:

```go
package keeper_test

import (
	"context"
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Mock AlignmentKeeper ----------

type mockAlignmentKeeper struct {
	healthCategory string
}

func (m *mockAlignmentKeeper) GetHealthCategory(_ context.Context) string {
	return m.healthCategory
}

// ---------- Tests ----------

// Test 2: Wood→Earth — degraded health expedites knowledge param LIPs.
func TestWoodEarth_DegradedHealth_ExpeditesKnowledgeLIP(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	// Wire alignment keeper reporting "degraded".
	ak := &mockAlignmentKeeper{healthCategory: "degraded"}
	k.SetAlignmentKeeper(ak)

	params := k.GetParams(ctx)
	baseVotingPeriod := params.VotingPeriodBlocks

	// Submit a knowledge param LIP.
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Knowledge Params",
		Description:  "Update verification rate threshold",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "knowledge", Key: "min_claim_stake", Value: "500000"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to last_call and let BeginBlocker transition to voting.
	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0 // expired immediately
	k.SetLIP(ctx, lip)

	k.BeginBlocker(ctx)

	// Verify voting period is halved.
	lip, _ = k.GetLIP(ctx, resp.LipId)
	if lip.Stage != types.StatusVoting {
		t.Fatalf("expected voting stage, got %s", lip.Stage)
	}

	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod/2
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected expedited VotingEndBlock=%d, got %d (base=%d)", expectedEnd, lip.VotingEndBlock, baseVotingPeriod)
	}
}

// Test 3: Wood→Earth — healthy system uses normal voting period.
func TestWoodEarth_HealthySystem_NormalVotingPeriod(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	ak := &mockAlignmentKeeper{healthCategory: "healthy"}
	k.SetAlignmentKeeper(ak)

	params := k.GetParams(ctx)
	baseVotingPeriod := params.VotingPeriodBlocks

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Knowledge Params",
		Description:  "Same params but healthy system",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "knowledge", Key: "min_claim_stake", Value: "500000"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0
	k.SetLIP(ctx, lip)

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, resp.LipId)
	if lip.Stage != types.StatusVoting {
		t.Fatalf("expected voting stage, got %s", lip.Stage)
	}

	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected normal VotingEndBlock=%d, got %d", expectedEnd, lip.VotingEndBlock)
	}
}

// Test 4: Wood→Earth — non-knowledge LIPs get normal period even during degradation.
func TestWoodEarth_NonKnowledgeLIP_NormalPeriodDuringDegradation(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	ak := &mockAlignmentKeeper{healthCategory: "degraded"}
	k.SetAlignmentKeeper(ak)

	params := k.GetParams(ctx)
	baseVotingPeriod := params.VotingPeriodBlocks

	// Submit a non-knowledge param change (e.g. zerone_staking).
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Staking Params",
		Description:  "Not knowledge-related",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "zerone_staking", Key: "max_validators", Value: "200"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0
	k.SetLIP(ctx, lip)

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, resp.LipId)
	if lip.Stage != types.StatusVoting {
		t.Fatalf("expected voting stage, got %s", lip.Stage)
	}

	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected normal VotingEndBlock=%d, got %d (should NOT be expedited)", expectedEnd, lip.VotingEndBlock)
	}
}

// Verify manual AdvanceLIPStage also respects expedited voting.
func TestWoodEarth_ManualAdvance_ExpeditesKnowledgeLIP(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	ak := &mockAlignmentKeeper{healthCategory: "critical"}
	k.SetAlignmentKeeper(ak)

	params := k.GetParams(ctx)
	baseVotingPeriod := params.VotingPeriodBlocks

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Alignment Params",
		Description:  "Critical system",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "alignment", Key: "critical_threshold", Value: "100000"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Put in last_call with review done.
	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0
	k.SetLIP(ctx, lip)

	// Manual advance last_call → voting.
	_, err = ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"),
		LipId:     resp.LipId,
	})
	if err != nil {
		t.Fatalf("advance failed: %v", err)
	}

	lip, _ = k.GetLIP(ctx, resp.LipId)
	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod/2
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected expedited VotingEndBlock=%d, got %d", expectedEnd, lip.VotingEndBlock)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -run TestWoodEarth -v`
Expected: FAIL — `k.SetAlignmentKeeper undefined`

**Step 3: Add AlignmentKeeper interface**

Add to `x/gov/types/expected_keepers.go` (after EmergencyKeeper):

```go
// AlignmentKeeper defines the alignment module interface for health-aware governance.
type AlignmentKeeper interface {
	GetHealthCategory(ctx context.Context) string
}
```

**Step 4: Add field and setter to gov keeper**

Modify `x/gov/keeper/keeper.go`:

Add field `alignmentKeeper types.AlignmentKeeper` to Keeper struct (after emergencyKeeper).

Add setter:

```go
// SetAlignmentKeeper sets the alignment keeper (post-init to break circular deps).
func (k *Keeper) SetAlignmentKeeper(ak types.AlignmentKeeper) {
	k.alignmentKeeper = ak
}
```

**Step 5: Create expedited.go**

Create `x/gov/keeper/expedited.go`:

```go
package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// expeditedTargetModules lists modules whose param-change LIPs can be expedited
// when alignment health is degraded or critical.
var expeditedTargetModules = map[string]bool{
	"knowledge":       true,
	"alignment":       true,
	"capture_defense": true,
}

// getEffectiveVotingPeriod returns the voting period for a LIP, potentially
// halved if alignment health is degraded/critical and the LIP targets
// knowledge-related modules (Wood controls Earth).
func (k Keeper) getEffectiveVotingPeriod(ctx sdk.Context, lip *types.LIP, params *types.Params) uint64 {
	basePeriod := params.VotingPeriodBlocks

	if !isKnowledgeParamLIP(lip) {
		return basePeriod
	}

	if k.alignmentKeeper == nil {
		return basePeriod
	}

	health := k.alignmentKeeper.GetHealthCategory(ctx)
	if health == "degraded" || health == "critical" {
		expedited := basePeriod / 2
		if expedited == 0 {
			expedited = 1
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.gov.expedited_voting",
				sdk.NewAttribute("lip_id", lip.Id),
				sdk.NewAttribute("target_modules", targetModulesString(lip)),
				sdk.NewAttribute("health_category", health),
				sdk.NewAttribute("base_voting_period", fmt.Sprintf("%d", basePeriod)),
				sdk.NewAttribute("effective_voting_period", fmt.Sprintf("%d", expedited)),
			),
		)

		return expedited
	}

	return basePeriod
}

// isKnowledgeParamLIP returns true if the LIP has param changes targeting
// knowledge, alignment, or capture_defense modules.
func isKnowledgeParamLIP(lip *types.LIP) bool {
	if lip.Category != types.CategoryParameter {
		return false
	}
	for _, pc := range lip.ParamChanges {
		if expeditedTargetModules[pc.Module] {
			return true
		}
	}
	return false
}

// targetModulesString returns a comma-separated list of target modules in the LIP's param changes.
func targetModulesString(lip *types.LIP) string {
	seen := make(map[string]bool)
	var modules string
	for _, pc := range lip.ParamChanges {
		if !seen[pc.Module] {
			if modules != "" {
				modules += ","
			}
			modules += pc.Module
			seen[pc.Module] = true
		}
	}
	return modules
}
```

**Step 6: Wire into abci.go**

Modify `x/gov/keeper/abci.go:46` — replace:

```go
			lip.VotingEndBlock = currentHeight + params.VotingPeriodBlocks
```

with:

```go
			lip.VotingEndBlock = currentHeight + k.getEffectiveVotingPeriod(ctx, lip, params)
```

**Step 7: Wire into msg_server.go AdvanceLIPStage**

Modify `x/gov/keeper/msg_server.go:200` — replace:

```go
		lip.VotingEndBlock = currentHeight + params.VotingPeriodBlocks
```

with:

```go
		lip.VotingEndBlock = currentHeight + ms.getEffectiveVotingPeriod(ctx, lip, params)
```

**Step 8: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -run TestWoodEarth -v`
Expected: PASS (all 4 tests)

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -v`
Expected: PASS (all existing tests still pass — they have nil alignmentKeeper, which falls through to base period)

**Step 9: Commit**

```bash
git add x/gov/types/expected_keepers.go x/gov/keeper/keeper.go x/gov/keeper/expedited.go x/gov/keeper/expedited_test.go x/gov/keeper/abci.go x/gov/keeper/msg_server.go
git commit -m "feat(gov): add Wood→Earth expedited voting for knowledge LIPs during system stress (R31-3)"
```

---

### Task 4: Earth→Water — Add DomainFormationFreeze proto and message types

**Files:**
- Modify: `proto/zerone/gov/v1/tx.proto` (add msg + rpc)
- Modify: `proto/zerone/partnerships/v1/types.proto` (add freeze type)
- Run: `make proto-gen`

**Step 1: Add MsgDomainFormationFreeze to gov tx.proto**

Add RPC to service Msg block (after VoteSeatElection line):

```protobuf
  rpc DomainFormationFreeze(MsgDomainFormationFreeze) returns (MsgDomainFormationFreezeResponse);
```

Add messages at end of file:

```protobuf
// MsgDomainFormationFreeze imposes a formation cooldown on a domain.
// Only executable via governance (authority-gated).
message MsgDomainFormationFreeze {
  option (cosmos.msg.v1.signer) = "authority";
  string authority       = 1;
  string domain          = 2;
  uint64 duration_blocks = 3;
  string reason          = 4;
}

message MsgDomainFormationFreezeResponse {}
```

**Step 2: Add DomainFormationFreeze to partnerships types.proto**

Add at end of file:

```protobuf
message DomainFormationFreeze {
  string domain        = 1;
  uint64 expiry_height = 2;
  string reason        = 3;
}
```

**Step 3: Run proto-gen**

Run: `cd /Users/yournameisai/Desktop/zerone && make proto-gen`
Expected: Generated `*.pb.go` files updated

**Step 4: Run proto-check**

Run: `cd /Users/yournameisai/Desktop/zerone && make proto-check`
Expected: PASS

**Step 5: Register codec**

Add to `x/gov/types/codec.go` RegisterCodec:

```go
	cdc.RegisterConcrete(&MsgDomainFormationFreeze{}, "zerone_gov/MsgDomainFormationFreeze", nil)
```

Add to RegisterInterfaces:

```go
		&MsgDomainFormationFreeze{},
```

**Step 6: Add ValidateBasic and GetSigners to gov types**

If not auto-generated, add to a new or existing file in `x/gov/types/`:

```go
func (msg *MsgDomainFormationFreeze) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if msg.DurationBlocks == 0 {
		return fmt.Errorf("duration_blocks must be > 0")
	}
	return nil
}

func (msg *MsgDomainFormationFreeze) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}
```

**Step 7: Commit**

```bash
git add proto/ x/gov/types/ x/partnerships/types/
git commit -m "feat(proto): add MsgDomainFormationFreeze and DomainFormationFreeze types (R31-3)"
```

---

### Task 5: Earth→Water — Implement formation freeze in partnerships keeper

**Files:**
- Modify: `x/partnerships/types/keys.go` (add prefix)
- Modify: `x/partnerships/types/types.go` (add Go struct)
- Modify: `x/partnerships/types/errors.go` (add error)
- Modify: `x/partnerships/keeper/structural_immunity.go` (add CRUD + expire)
- Modify: `x/partnerships/module.go` (add to BeginBlocker)

**Step 1: Write the failing test**

Create `x/partnerships/keeper/formation_freeze_test.go`:

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/partnerships/keeper"
	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// Test 5: Earth→Water — freeze blocks partnership formation.
func TestEarthWater_FreezeBlocksFormation(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Set a formation freeze on domain "physics".
	k.SetDomainFormationFreeze(ctx, "physics", 200, "governance review")

	// Fund proposer.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)

	// Attempt to propose partnership — should be blocked.
	_, err := ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agentAddr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "physics",
	})
	if err == nil {
		t.Fatal("expected error due to formation freeze, got nil")
	}
	if !types.ErrDomainFrozen.Is(err) {
		t.Errorf("expected ErrDomainFrozen, got: %v", err)
	}
}

// Test 6: Earth→Water — expired freeze allows formation.
func TestEarthWater_ExpiredFreezeAllowsFormation(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Set freeze expiring at block 50 (current is 100).
	k.SetDomainFormationFreeze(ctx, "physics", 50, "old freeze")

	// Run BeginBlocker to expire it.
	k.ExpireFormationFreezes(ctx)

	// Fund proposer.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)

	// Partnership should succeed now.
	_, err := ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agentAddr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "physics",
	})
	if err != nil {
		t.Fatalf("expected success after freeze expired, got: %v", err)
	}
}

// Test 7: Earth→Water — freeze on domain A doesn't affect domain B.
func TestEarthWater_FreezeIsDomainSpecific(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Freeze only "physics".
	k.SetDomainFormationFreeze(ctx, "physics", 200, "governance review")

	// Fund proposer.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)

	// Partnership in "biology" should succeed.
	_, err := ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agentAddr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "biology",
	})
	if err != nil {
		t.Fatalf("expected success for unfrozen domain, got: %v", err)
	}

	// Partnership in "physics" should fail.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)
	_, err = ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agent2Addr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "physics",
	})
	if err == nil {
		t.Fatal("expected error for frozen domain physics, got nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/partnerships/keeper/ -run "TestEarthWater" -v`
Expected: FAIL — compile errors (missing methods, fields, errors)

**Step 3: Add store key prefix**

Add to `x/partnerships/types/keys.go`:

```go
	FormationFreezeKeyPrefix = []byte{0x1A} // R31-3: domain formation freezes
```

**Step 4: Add Go struct**

Add to `x/partnerships/types/types.go` (after `FormationBonus`):

```go
// DomainFormationFreeze represents a governance-imposed freeze on partnership formation in a domain (R31-3).
type DomainFormationFreeze struct {
	Domain       string
	ExpiryHeight uint64
	Reason       string
}
```

**Step 5: Add error**

Add to `x/partnerships/types/errors.go`:

```go
	ErrDomainFrozen = errors.Register(ModuleName, 70, "domain is under formation freeze")
```

**Step 6: Add Domain field to MsgProposePartnership**

Check if `MsgProposePartnership` in `proto/zerone/partnerships/v1/tx.proto` already has a domain field. If not, add it. Then check the msg_server.go to see how to pass it through.

**Important:** `ProposePartnership` currently doesn't have a `Domain` field — partnerships are domain-agnostic. The freeze check needs the domain from the message. We need to add `string domain` to `MsgProposePartnership` in the proto and regenerate. If this proto change is too invasive, we can instead make the freeze check operate only through mentorships (which have domains). However, per the design, we add it to `MsgProposePartnership`.

Add to `proto/zerone/partnerships/v1/tx.proto` in MsgProposePartnership:

```protobuf
  string domain = 5;  // optional: domain context for formation freeze checks
```

Run: `make proto-gen`

**Step 7: Add CRUD methods to structural_immunity.go**

Add to `x/partnerships/keeper/structural_immunity.go` (after formation bonus section):

```go
// ---------- Domain Formation Freeze CRUD (R31-3) ----------

func formationFreezeKey(domain string) []byte {
	return append(types.FormationFreezeKeyPrefix, []byte(domain)...)
}

// SetDomainFormationFreeze stores a formation freeze for a domain.
func (k Keeper) SetDomainFormationFreeze(ctx sdk.Context, domain string, expiryHeight uint64, reason string) {
	freeze := &types.DomainFormationFreeze{
		Domain:       domain,
		ExpiryHeight: expiryHeight,
		Reason:       reason,
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(freeze)
	if err != nil {
		panic("failed to marshal formation freeze: " + err.Error())
	}
	_ = kvStore.Set(formationFreezeKey(domain), bz)
}

// GetDomainFormationFreeze returns the formation freeze for a domain, if any.
func (k Keeper) GetDomainFormationFreeze(ctx sdk.Context, domain string) *types.DomainFormationFreeze {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(formationFreezeKey(domain))
	if err != nil || bz == nil {
		return nil
	}
	var freeze types.DomainFormationFreeze
	if err := json.Unmarshal(bz, &freeze); err != nil {
		return nil
	}
	return &freeze
}

// DeleteDomainFormationFreeze removes a formation freeze.
func (k Keeper) DeleteDomainFormationFreeze(ctx sdk.Context, domain string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(formationFreezeKey(domain))
}

// ExpireFormationFreezes removes expired formation freezes.
func (k Keeper) ExpireFormationFreezes(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.FormationFreezeKeyPrefix, prefixEndBytes(types.FormationFreezeKeyPrefix))
	if err != nil {
		return
	}

	var toDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		var freeze types.DomainFormationFreeze
		if err := json.Unmarshal(iter.Value(), &freeze); err != nil {
			continue
		}
		if freeze.ExpiryHeight > 0 && freeze.ExpiryHeight <= currentBlock {
			toDelete = append(toDelete, append([]byte{}, iter.Key()...))
		}
	}
	iter.Close()

	for _, key := range toDelete {
		_ = kvStore.Delete(key)
	}
}
```

**Step 8: Add freeze check to ProposePartnership**

Modify `x/partnerships/keeper/msg_server.go` — add at the top of `ProposePartnership` (after `currentBlock` assignment, before existing partnership check, around line 31):

```go
	// R31-3: Check domain formation freeze.
	if msg.Domain != "" {
		if freeze := k.GetDomainFormationFreeze(ctx, msg.Domain); freeze != nil {
			if currentBlock < freeze.ExpiryHeight {
				ctx.EventManager().EmitEvent(
					sdk.NewEvent("zerone.partnerships.formation_blocked",
						sdk.NewAttribute("domain", msg.Domain),
						sdk.NewAttribute("freeze_expiry", fmt.Sprintf("%d", freeze.ExpiryHeight)),
						sdk.NewAttribute("freeze_reason", freeze.Reason),
						sdk.NewAttribute("requester", msg.Proposer),
					),
				)
				return nil, fmt.Errorf("%w: domain %s is under formation freeze until block %d: %s",
					types.ErrDomainFrozen, msg.Domain, freeze.ExpiryHeight, freeze.Reason)
			}
			// Freeze expired — clear it.
			k.DeleteDomainFormationFreeze(ctx, msg.Domain)
		}
	}
```

**Step 9: Add to BeginBlocker**

Modify `x/partnerships/module.go` — add after `am.keeper.ExpireFormationBonuses(sdkCtx)` (line 155):

```go
	am.keeper.ExpireFormationFreezes(sdkCtx) // R31-3: expire domain formation freezes
```

**Step 10: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/partnerships/keeper/ -run "TestEarthWater" -v`
Expected: PASS (all 3 tests)

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/partnerships/keeper/ -v`
Expected: PASS (all existing tests still pass)

**Step 11: Commit**

```bash
git add x/partnerships/ proto/zerone/partnerships/
git commit -m "feat(partnerships): add domain formation freeze with expiry (R31-3 Earth→Water)"
```

---

### Task 6: Earth→Water — Add DomainFormationFreeze msg handler to governance

**Files:**
- Modify: `x/gov/types/expected_keepers.go` (add PartnershipsKeeper)
- Modify: `x/gov/keeper/keeper.go` (add field + setter)
- Modify: `x/gov/keeper/msg_server.go` (add handler)

**Step 1: Write the failing test**

Add to `x/gov/keeper/expedited_test.go` (or create new file `x/gov/keeper/formation_freeze_test.go`):

```go
// ---------- Mock PartnershipsKeeper ----------

type mockPartnershipsKeeper struct {
	freezes map[string]freezeRecord
}

type freezeRecord struct {
	expiryHeight uint64
	reason       string
}

func newMockPartnershipsKeeper() *mockPartnershipsKeeper {
	return &mockPartnershipsKeeper{freezes: make(map[string]freezeRecord)}
}

func (m *mockPartnershipsKeeper) SetDomainFormationFreeze(_ context.Context, domain string, expiryHeight uint64, reason string) {
	m.freezes[domain] = freezeRecord{expiryHeight: expiryHeight, reason: reason}
}

// ---------- Tests ----------

func TestDomainFormationFreeze_AuthorityOnly(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	pk := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(pk)

	// Non-authority should fail.
	_, err := ms.DomainFormationFreeze(ctx, &types.MsgDomainFormationFreeze{
		Authority:      testAddr("random"),
		Domain:         "physics",
		DurationBlocks: 1000,
		Reason:         "governance review",
	})
	if err == nil {
		t.Fatal("expected unauthorized error, got nil")
	}
}

func TestDomainFormationFreeze_DelegateToPartnerships(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	pk := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(pk)

	// Authority call should succeed.
	_, err := ms.DomainFormationFreeze(ctx, &types.MsgDomainFormationFreeze{
		Authority:      k.GetAuthority(),
		Domain:         "physics",
		DurationBlocks: 1000,
		Reason:         "governance review",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// Check freeze was set with correct expiry.
	freeze, ok := pk.freezes["physics"]
	if !ok {
		t.Fatal("expected freeze to be set on partnerships keeper")
	}
	expectedExpiry := uint64(ctx.BlockHeight()) + 1000
	if freeze.expiryHeight != expectedExpiry {
		t.Errorf("expected expiryHeight=%d, got %d", expectedExpiry, freeze.expiryHeight)
	}
	if freeze.reason != "governance review" {
		t.Errorf("expected reason='governance review', got %q", freeze.reason)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -run TestDomainFormation -v`
Expected: FAIL — `k.SetPartnershipsKeeper undefined`

**Step 3: Add PartnershipsKeeper interface**

Add to `x/gov/types/expected_keepers.go`:

```go
// PartnershipsKeeper defines the partnerships module interface for governance-controlled formation.
type PartnershipsKeeper interface {
	SetDomainFormationFreeze(ctx context.Context, domain string, expiryHeight uint64, reason string)
}
```

**Step 4: Add field and setter to gov keeper**

Add field `partnershipsKeeper types.PartnershipsKeeper` to Keeper struct (after alignmentKeeper).

Add setter:

```go
// SetPartnershipsKeeper sets the partnerships keeper (post-init to break circular deps).
func (k *Keeper) SetPartnershipsKeeper(pk types.PartnershipsKeeper) {
	k.partnershipsKeeper = pk
}
```

**Step 5: Add msg handler**

Add to `x/gov/keeper/msg_server.go` (before research spend handlers):

```go
// DomainFormationFreeze imposes a formation cooldown on a domain (authority only).
func (ms *msgServer) DomainFormationFreeze(goCtx context.Context, msg *types.MsgDomainFormationFreeze) (*types.MsgDomainFormationFreezeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if ms.GetAuthority() != msg.Authority {
		return nil, types.ErrUnauthorized
	}

	if msg.Domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}
	if msg.DurationBlocks == 0 {
		return nil, fmt.Errorf("duration_blocks must be > 0")
	}

	expiryHeight := uint64(ctx.BlockHeight()) + msg.DurationBlocks

	if ms.partnershipsKeeper != nil {
		ms.partnershipsKeeper.SetDomainFormationFreeze(ctx, msg.Domain, expiryHeight, msg.Reason)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.domain_formation_freeze",
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("duration_blocks", fmt.Sprintf("%d", msg.DurationBlocks)),
			sdk.NewAttribute("expiry_height", fmt.Sprintf("%d", expiryHeight)),
			sdk.NewAttribute("reason", msg.Reason),
		),
	)

	return &types.MsgDomainFormationFreezeResponse{}, nil
}
```

**Step 6: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -run TestDomainFormation -v`
Expected: PASS

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add x/gov/types/expected_keepers.go x/gov/keeper/keeper.go x/gov/keeper/msg_server.go x/gov/keeper/formation_freeze_test.go
git commit -m "feat(gov): add MsgDomainFormationFreeze handler delegating to partnerships (R31-3 Earth→Water)"
```

---

### Task 7: Wire keepers in app.go

**Files:**
- Modify: `app/app.go`

**Step 1: Add alignment keeper wiring**

After the existing `alignmentPacingAdapter` wiring block (around line 1044), add:

```go
	// R31-3: Wire alignment health signal into governance for expedited voting.
	app.ZeroneGovKeeper.SetAlignmentKeeper(&app.AlignmentKeeper)
```

**Step 2: Add partnerships keeper wiring**

In the same area, add:

```go
	// R31-3: Wire partnerships keeper into governance for domain formation freezes.
	app.ZeroneGovKeeper.SetPartnershipsKeeper(&app.PartnershipsKeeper)
```

**Step 3: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: PASS

Note: The alignment `Keeper` already has `GetHealthCategory`. The partnerships `Keeper` already has `SetDomainFormationFreeze`. Both satisfy their respective interfaces. If there's a type mismatch (sdk.Context vs context.Context), create a thin adapter following the `AlignmentPacingAdapter` pattern.

**Step 4: Commit**

```bash
git add app/app.go
git commit -m "feat(app): wire alignment and partnerships keepers into governance (R31-3)"
```

---

### Task 8: Final verification

**Step 1: Run all modified module tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/gov/keeper/ -v`
Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -v`
Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/partnerships/keeper/ -v`
Expected: All PASS

**Step 2: Run full build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: PASS

**Step 3: Run proto-check**

Run: `cd /Users/yournameisai/Desktop/zerone && make proto-check`
Expected: PASS

**Step 4: Final commit if any fixes needed**

Only if adjustments were required during verification.
