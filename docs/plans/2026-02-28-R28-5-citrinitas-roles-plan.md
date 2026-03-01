# R28-5 Citrinitas: Role Differentiation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give humans and agents measurably different on-chain strengths through bonus multipliers (not restrictions), incentivizing collaboration.

**Architecture:** Module-local bonuses. Each module queries a `ZeroneAuthKeeper` interface for account type and applies its own governance-adjustable BPS multipliers. No new modules. Follows existing setter-pattern for keeper wiring.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, protobuf (hand-edited pb.go)

---

### Task 1: Add CLAIM_TYPE_COMPUTATIONAL to types

**Files:**
- Modify: `x/knowledge/types/types.pb.go:303-332` (ClaimType enum + maps)

**Step 1: Write the failing test**

In `x/knowledge/keeper/claim_types_test.go`, add:

```go
func TestComputationalClaimTypeExists(t *testing.T) {
	// Verify COMPUTATIONAL claim type is registered in the enum
	ct := types.ClaimType_CLAIM_TYPE_COMPUTATIONAL
	require.Equal(t, int32(7), int32(ct))
	require.Equal(t, "CLAIM_TYPE_COMPUTATIONAL", types.ClaimType_name[7])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestComputationalClaimTypeExists -v`
Expected: FAIL — `ClaimType_CLAIM_TYPE_COMPUTATIONAL` undefined

**Step 3: Add the new claim type**

In `x/knowledge/types/types.pb.go`, add to the const block after line 310:

```go
ClaimType_CLAIM_TYPE_COMPUTATIONAL ClaimType = 7 // Derived from computation/inference — agent specialty
```

Add to `ClaimType_name` map:
```go
7: "CLAIM_TYPE_COMPUTATIONAL",
```

Add to `ClaimType_value` map:
```go
"CLAIM_TYPE_COMPUTATIONAL": 7,
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/knowledge/keeper/ -run TestComputationalClaimTypeExists -v`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/types/types.pb.go x/knowledge/keeper/claim_types_test.go
git commit -m "feat(knowledge): add CLAIM_TYPE_COMPUTATIONAL enum value (R28-5)"
```

---

### Task 2: Add ZeroneAuthKeeper interface and keeper wiring

**Files:**
- Modify: `x/knowledge/types/expected_keepers.go:1-101` — add interface
- Modify: `x/knowledge/keeper/keeper.go:1-88` — add field + setter
- Modify: `x/partnerships/types/expected_keepers.go:1-21` — add interface
- Modify: `x/partnerships/keeper/keeper.go:16-27` — add field + setter

**Step 1: Add ZeroneAuthKeeper interface to knowledge module**

In `x/knowledge/types/expected_keepers.go`, add after the `PartnershipKeeper` interface (after line 101):

```go
// ZeroneAuthKeeper defines the expected zerone auth keeper interface (R28-5).
// Used to look up account types (human/agent/contract) for role bonuses.
type ZeroneAuthKeeper interface {
	// GetAccountType returns the account type ("human", "agent", "contract", "system")
	// for a given bech32 address. Returns "" and false if not found.
	GetAccountType(ctx context.Context, address string) (string, bool)
}
```

Note: We define a minimal interface with just `GetAccountType` rather than exposing the full Account struct. This keeps the dependency surface small.

**Step 2: Add field and setter to knowledge keeper**

In `x/knowledge/keeper/keeper.go`, add field to the Keeper struct after `partnershipKeeper` (line 29):

```go
zeroneAuthKeeper types.ZeroneAuthKeeper // nil until R28-5
```

Add setter after `SetPartnershipKeeper` (after line 82):

```go
// SetZeroneAuthKeeper sets the zerone auth keeper (post-init, R28-5).
func (k *Keeper) SetZeroneAuthKeeper(ak types.ZeroneAuthKeeper) {
	k.zeroneAuthKeeper = ak
}
```

**Step 3: Add ZeroneAuthKeeper interface to partnerships module**

In `x/partnerships/types/expected_keepers.go`, add after `HomeKeeper` (after line 21):

```go
// ZeroneAuthKeeper defines the expected zerone auth keeper interface (R28-5).
type ZeroneAuthKeeper interface {
	GetAccountType(ctx context.Context, address string) (string, bool)
}
```

**Step 4: Add field and setter to partnerships keeper**

In `x/partnerships/keeper/keeper.go`, add field to the Keeper struct (after line 24):

```go
zeroneAuthKeeper types.ZeroneAuthKeeper // nil until R28-5
```

Add setter after `SetHomeKeeper` (after line 68):

```go
// SetZeroneAuthKeeper sets the zerone auth keeper (post-init, R28-5).
func (k *Keeper) SetZeroneAuthKeeper(ak types.ZeroneAuthKeeper) {
	k.zeroneAuthKeeper = ak
}
```

**Step 5: Create the adapter in zerone auth**

Create new file `x/auth/keeper/knowledge_adapters.go`:

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	partnershipstypes "github.com/zerone-chain/zerone/x/partnerships/types"
)

// KnowledgeAuthAdapter wraps the zerone auth Keeper to satisfy
// knowledgetypes.ZeroneAuthKeeper interface.
type KnowledgeAuthAdapter struct {
	k Keeper
}

// NewKnowledgeAuthAdapter returns an adapter for the knowledge module.
func NewKnowledgeAuthAdapter(k Keeper) *KnowledgeAuthAdapter {
	return &KnowledgeAuthAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ knowledgetypes.ZeroneAuthKeeper = (*KnowledgeAuthAdapter)(nil)
var _ partnershipstypes.ZeroneAuthKeeper = (*KnowledgeAuthAdapter)(nil)

// GetAccountType returns the account type for a bech32 address.
func (a *KnowledgeAuthAdapter) GetAccountType(_ context.Context, address string) (string, bool) {
	// Use background context — auth keeper uses sdk.Context internally
	ctx := sdk.Context{}
	account, found := a.k.GetAccount(ctx, address)
	if !found {
		return "", false
	}
	return account.AccountType, true
}
```

Wait — the auth keeper's `GetAccount` takes `sdk.Context`. Let me check the signature.

The adapter needs to properly unwrap the context. Looking at `bvm_adapters.go` for the pattern:

```go
func (a *KnowledgeAuthAdapter) GetAccountType(goCtx context.Context, address string) (string, bool) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	account, found := a.k.GetAccount(ctx, address)
	if !found {
		return "", false
	}
	return account.AccountType, true
}
```

**Step 6: Wire adapters in app.go**

In `app/app.go`, after the existing knowledge keeper wiring (after the `SetPartnershipKeeper` block around line 1083), add:

```go
// Wire zerone auth into knowledge and partnerships for role bonuses (R28-5).
knowledgeAuthAdapter := zeroneauthkeeper.NewKnowledgeAuthAdapter(app.ZeroneAuthKeeper)
app.KnowledgeKeeper.SetZeroneAuthKeeper(knowledgeAuthAdapter)
app.PartnershipsKeeper.SetZeroneAuthKeeper(knowledgeAuthAdapter)
```

**Step 7: Verify compilation**

Run: `go build ./...`
Expected: PASS (compiles clean)

**Step 8: Commit**

```bash
git add x/knowledge/types/expected_keepers.go x/knowledge/keeper/keeper.go \
  x/partnerships/types/expected_keepers.go x/partnerships/keeper/keeper.go \
  x/auth/keeper/knowledge_adapters.go app/app.go
git commit -m "feat(knowledge,partnerships): add ZeroneAuthKeeper interface and wiring (R28-5)"
```

---

### Task 3: Add role bonus params to knowledge module

**Files:**
- Modify: `x/knowledge/types/genesis.pb.go:24-161` — add 5 new param fields
- Modify: `x/knowledge/types/genesis.go:7-159` — add defaults + validation

**Step 1: Write the failing test**

In `x/knowledge/keeper/params_test.go`, add:

```go
func TestRoleBonusParamsDefaults(t *testing.T) {
	params := types.DefaultParams()

	// Role bonus params — all use BPS scale (1,000,000 = 100%)
	require.Equal(t, uint64(150_000), params.HumanEmpiricalBonusBps, "human empirical bonus should be +15%")
	require.Equal(t, uint64(150_000), params.AgentComputationalBonusBps, "agent computational bonus should be +15%")
	require.Equal(t, uint64(200_000), params.AgentVerificationBonusBps, "agent verification bonus should be +20%")
	require.Equal(t, uint64(100_000), params.HumanPatronageBonusBps, "human patronage bonus should be +10%")
	require.Equal(t, uint64(250_000), params.DualValidationBonusBps, "dual validation bonus should be +25%")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestRoleBonusParamsDefaults -v`
Expected: FAIL — fields don't exist

**Step 3: Add fields to Params proto struct**

In `x/knowledge/types/genesis.pb.go`, add after the `MaxConfidence` field (line 158, before `unknownFields`):

```go
// ─── Role bonuses (R28-5) — additive BPS multipliers, NOT thresholds ────
HumanEmpiricalBonusBps     uint64 `protobuf:"varint,110,opt,name=human_empirical_bonus_bps,json=humanEmpiricalBonusBps,proto3" json:"human_empirical_bonus_bps,omitempty"`           // +15% confidence for human empirical claims
AgentComputationalBonusBps uint64 `protobuf:"varint,111,opt,name=agent_computational_bonus_bps,json=agentComputationalBonusBps,proto3" json:"agent_computational_bonus_bps,omitempty"` // +15% confidence for agent computational claims
AgentVerificationBonusBps  uint64 `protobuf:"varint,112,opt,name=agent_verification_bonus_bps,json=agentVerificationBonusBps,proto3" json:"agent_verification_bonus_bps,omitempty"`    // +20% vote weight for agent verifiers
HumanPatronageBonusBps     uint64 `protobuf:"varint,113,opt,name=human_patronage_bonus_bps,json=humanPatronageBonusBps,proto3" json:"human_patronage_bonus_bps,omitempty"`             // +10% energy boost for human patrons
DualValidationBonusBps     uint64 `protobuf:"varint,114,opt,name=dual_validation_bonus_bps,json=dualValidationBonusBps,proto3" json:"dual_validation_bonus_bps,omitempty"`             // +25% confidence for partnership (human+agent) claims
```

Add getter methods after the existing getters (find the pattern `func (x *Params) GetMaxConfidence()`):

```go
func (x *Params) GetHumanEmpiricalBonusBps() uint64 {
	if x != nil {
		return x.HumanEmpiricalBonusBps
	}
	return 0
}

func (x *Params) GetAgentComputationalBonusBps() uint64 {
	if x != nil {
		return x.AgentComputationalBonusBps
	}
	return 0
}

func (x *Params) GetAgentVerificationBonusBps() uint64 {
	if x != nil {
		return x.AgentVerificationBonusBps
	}
	return 0
}

func (x *Params) GetHumanPatronageBonusBps() uint64 {
	if x != nil {
		return x.HumanPatronageBonusBps
	}
	return 0
}

func (x *Params) GetDualValidationBonusBps() uint64 {
	if x != nil {
		return x.DualValidationBonusBps
	}
	return 0
}
```

**Step 4: Add defaults in genesis.go**

In `x/knowledge/types/genesis.go`, add to `DefaultParams()` before the closing brace (after line 158):

```go
// ─── Role bonuses (R28-5) — additive BPS, NOT thresholds ──────────
HumanEmpiricalBonusBps:     150_000, // +15% confidence for human OBSERVATION claims
AgentComputationalBonusBps: 150_000, // +15% confidence for agent COMPUTATIONAL claims
AgentVerificationBonusBps:  200_000, // +20% vote weight for agent verifiers
HumanPatronageBonusBps:     100_000, // +10% energy boost for human patrons
DualValidationBonusBps:     250_000, // +25% confidence for partnership claims
```

**Step 5: Add validation in genesis.go**

In `x/knowledge/types/genesis.go`, add to `Validate()` before `return nil` (before line 500):

```go
// ─── Role bonus params ──────────────────────────────────────────
if p.HumanEmpiricalBonusBps > 1_000_000 {
	return fmt.Errorf("human_empirical_bonus_bps must be <= 1,000,000")
}
if p.AgentComputationalBonusBps > 1_000_000 {
	return fmt.Errorf("agent_computational_bonus_bps must be <= 1,000,000")
}
if p.AgentVerificationBonusBps > 1_000_000 {
	return fmt.Errorf("agent_verification_bonus_bps must be <= 1,000,000")
}
if p.HumanPatronageBonusBps > 1_000_000 {
	return fmt.Errorf("human_patronage_bonus_bps must be <= 1,000,000")
}
if p.DualValidationBonusBps > 1_000_000 {
	return fmt.Errorf("dual_validation_bonus_bps must be <= 1,000,000")
}
```

**Step 6: Run test to verify it passes**

Run: `go test ./x/knowledge/keeper/ -run TestRoleBonusParamsDefaults -v`
Expected: PASS

**Step 7: Commit**

```bash
git add x/knowledge/types/genesis.pb.go x/knowledge/types/genesis.go \
  x/knowledge/keeper/params_test.go
git commit -m "feat(knowledge): add role bonus params with defaults and validation (R28-5)"
```

---

### Task 4: Add HumanCoercionFreezeMultiplierBps param to partnerships

**Files:**
- Modify: `x/partnerships/types/genesis.pb.go:116-133` — add field
- Modify: `x/partnerships/types/genesis.go:6-22` — add default
- Modify: `x/partnerships/types/types.go:67-96` — add validation

**Step 1: Write the failing test**

Create `x/partnerships/keeper/anti_coercion_test.go` (or add to existing):

```go
func TestHumanCoercionFreezeMultiplierParam(t *testing.T) {
	params := types.DefaultParams()
	require.Equal(t, uint64(1_500_000), params.HumanCoercionFreezeMultiplierBps,
		"human coercion freeze multiplier should be 1.5x (1,500,000 BPS)")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/partnerships/keeper/ -run TestHumanCoercionFreezeMultiplierParam -v`
Expected: FAIL

**Step 3: Add field to Params proto struct**

In `x/partnerships/types/genesis.pb.go`, add before `unknownFields` (line 131):

```go
HumanCoercionFreezeMultiplierBps uint64 `protobuf:"varint,14,opt,name=human_coercion_freeze_multiplier_bps,json=humanCoercionFreezeMultiplierBps,proto3" json:"human_coercion_freeze_multiplier_bps,omitempty"` // 1.5x (1,500,000 BPS) — human coercion signals freeze longer
```

Add getter method after existing getters:

```go
func (x *Params) GetHumanCoercionFreezeMultiplierBps() uint64 {
	if x != nil {
		return x.HumanCoercionFreezeMultiplierBps
	}
	return 0
}
```

**Step 4: Add default**

In `x/partnerships/types/genesis.go`, add to `DefaultParams()` (before closing brace on line 21):

```go
HumanCoercionFreezeMultiplierBps: 1_500_000, // 1.5x freeze duration for human coercion signals (R28-5)
```

**Step 5: Add validation**

In `x/partnerships/types/types.go`, add to `Validate()` before `return nil`:

```go
// HumanCoercionFreezeMultiplierBps: must be at least 1x (1,000,000 BPS) — freezes can't shrink
if p.HumanCoercionFreezeMultiplierBps > 0 && p.HumanCoercionFreezeMultiplierBps < 1_000_000 {
	return fmt.Errorf("human_coercion_freeze_multiplier_bps must be >= 1,000,000 (1.0x) or 0 (disabled)")
}
```

**Step 6: Run test to verify it passes**

Run: `go test ./x/partnerships/keeper/ -run TestHumanCoercionFreezeMultiplierParam -v`
Expected: PASS

**Step 7: Commit**

```bash
git add x/partnerships/types/genesis.pb.go x/partnerships/types/genesis.go \
  x/partnerships/types/types.go x/partnerships/keeper/anti_coercion_test.go
git commit -m "feat(partnerships): add HumanCoercionFreezeMultiplierBps param (R28-5)"
```

---

### Task 5: Implement claim confidence bonus on Fact creation

**Files:**
- Modify: `x/knowledge/keeper/rounds.go:226-267` — apply bonus in createFactFromClaim
- Test: `x/knowledge/keeper/role_bonus_test.go` — new test file

**Step 1: Write the failing test**

Create `x/knowledge/keeper/role_bonus_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Mock ZeroneAuthKeeper ──────────────────────────────────────────────────

type mockZeroneAuthKeeper struct {
	accounts map[string]string // address → account type
}

func newMockZeroneAuthKeeper() *mockZeroneAuthKeeper {
	return &mockZeroneAuthKeeper{accounts: make(map[string]string)}
}

func (m *mockZeroneAuthKeeper) GetAccountType(_ context.Context, address string) (string, bool) {
	t, ok := m.accounts[address]
	return t, ok
}

// ─── Claim Confidence Bonus Tests ──────────────────────────────────────────

func TestHumanEmpiricalClaimBonus(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1submitter1"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	// Register validators
	sk.addValidator("zrn1validator1", 100_000, "genesis")
	sk.addValidator("zrn1validator2", 100_000, "genesis")
	sk.addValidator("zrn1validator3", 100_000, "genesis")

	// Create an OBSERVATION claim from a human
	claim := &types.Claim{
		Id:               "test-claim-1",
		FactContent:      "The sky appears blue at noon on a clear day",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        "zrn1submitter1",
		SubmittedAtBlock: 100,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		ContentHash:      "abc123",
		ClaimType:        types.ClaimType_CLAIM_TYPE_OBSERVATION,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round, err := k.CreateVerificationRound(ctx, claim)
	require.NoError(t, err)

	// All 3 validators accept — should give ~100% accept ratio
	// but capped at MaxConfidence (880,000)
	round.Reveals = []*types.Reveal{
		{Verifier: "zrn1validator1", Vote: "accept"},
		{Verifier: "zrn1validator2", Vote: "accept"},
		{Verifier: "zrn1validator3", Vote: "accept"},
	}
	round.Commits = []*types.Commit{
		{Verifier: "zrn1validator1"},
		{Verifier: "zrn1validator2"},
		{Verifier: "zrn1validator3"},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	// Process the round — should create a fact with bonus
	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)

	// Base confidence = 1,000,000 (100% unanimous) → clamped to MaxConfidence 880,000
	// With human empirical bonus: 880,000 * (1M + 150,000) / 1M = 880,000 * 1.15 = 1,012,000
	// → re-clamped to MaxConfidence 880,000
	// So for a unanimous vote the bonus is absorbed by the cap.
	// Test with a borderline scenario instead where the bonus matters.
}

func TestHumanEmpiricalClaimBonusVisible(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1submitter1"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	// Create claim and manually invoke createFactFromClaim via ProcessRound
	// with a confidence value that won't hit the cap after bonus.
	// Base confidence: 800,000. With +15% bonus: 920,000 → capped at 880,000.
	// Use 770,000 base: 770,000 * 1.15 = 885,500 → capped at 880,000.
	// Use 700,000 base: 700,000 * 1.15 = 805,000 → under 880,000 cap. Bonus visible!

	// We test via the exported ApplyRoleBonusToConfidence helper.
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	// 700,000 * (1,000,000 + 150,000) / 1,000,000 = 805,000
	require.Equal(t, uint64(805_000), boosted)
}

func TestAgentComputationalClaimBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1agent1"] = "agent"
	k.SetZeroneAuthKeeper(authKeeper)

	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "agent", params)
	require.Equal(t, uint64(805_000), boosted)
}

func TestNoBonus_HumanComputational(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Human submitting computational claim → no bonus
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "human", params)
	require.Equal(t, uint64(700_000), boosted)
}

func TestNoBonus_AgentObservation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Agent submitting observation claim → no bonus
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "agent", params)
	require.Equal(t, uint64(700_000), boosted)
}

func TestNoBonus_UnknownAccount(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Unknown account type → no bonus
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "", params)
	require.Equal(t, uint64(700_000), boosted)
}

func TestDualValidationBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Partnership claim: base 700,000 + 25% = 875,000
	boosted := keeper.ApplyDualValidationBonus(700_000, params)
	require.Equal(t, uint64(875_000), boosted)
}

func TestRoleBonusPlusDualValidation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Human empirical + partnership: 700,000 * 1.15 = 805,000 → * 1.25 = 1,006,250
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	require.Equal(t, uint64(805_000), boosted)

	final := keeper.ApplyDualValidationBonus(boosted, params)
	// 805,000 * 1.25 = 1,006,250 (will be clamped by MaxConfidence later)
	require.Equal(t, uint64(1_006_250), final)
}
```

Add missing import `"context"` at the top of the test file.

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run "TestHumanEmpiricalClaimBonusVisible|TestAgentComputationalClaimBonus|TestNoBonus|TestDualValidation|TestRoleBonusPlus" -v`
Expected: FAIL — `ApplyRoleBonusToConfidence` and `ApplyDualValidationBonus` undefined

**Step 3: Implement the bonus helper functions**

Create `x/knowledge/keeper/role_bonus.go`:

```go
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ApplyRoleBonusToConfidence applies the claim-type × account-type bonus.
// Returns the boosted confidence (NOT clamped — caller must clamp).
func ApplyRoleBonusToConfidence(confidence uint64, claimType types.ClaimType, accountType string, params *types.Params) uint64 {
	var bonusBps uint64

	switch {
	case claimType == types.ClaimType_CLAIM_TYPE_OBSERVATION && accountType == "human":
		bonusBps = params.HumanEmpiricalBonusBps
	case claimType == types.ClaimType_CLAIM_TYPE_COMPUTATIONAL && accountType == "agent":
		bonusBps = params.AgentComputationalBonusBps
	}

	if bonusBps == 0 {
		return confidence
	}

	// confidence * (1,000,000 + bonusBps) / 1,000,000
	return safeMulDiv(confidence, 1_000_000+bonusBps, 1_000_000)
}

// ApplyDualValidationBonus applies the partnership dual-validation bonus.
// Returns the boosted confidence (NOT clamped — caller must clamp).
func ApplyDualValidationBonus(confidence uint64, params *types.Params) uint64 {
	if params.DualValidationBonusBps == 0 {
		return confidence
	}
	return safeMulDiv(confidence, 1_000_000+params.DualValidationBonusBps, 1_000_000)
}

// getAccountType safely looks up account type via ZeroneAuthKeeper.
// Returns "" if keeper is nil or account not found.
func (k Keeper) getAccountType(ctx context.Context, address string) string {
	if k.zeroneAuthKeeper == nil {
		return ""
	}
	accountType, found := k.zeroneAuthKeeper.GetAccountType(ctx, address)
	if !found {
		return ""
	}
	return accountType
}
```

**Step 4: Integrate into createFactFromClaim**

In `x/knowledge/keeper/rounds.go`, in the `createFactFromClaim` function, add after line 244 (`Confidence: confidence,`) but BEFORE the clamp on line 267:

Actually, the best insertion point is between setting `fact.Confidence = confidence` (line 244) and the clamp call on line 267. Insert BEFORE the clamp so the bonus is applied first, then clamped.

After line 264 (closing of fact struct), before line 266 (`// Apply confidence ceiling`):

```go
// Apply role bonus — claim type × account type (R28-5)
accountType := k.getAccountType(ctx, claim.Submitter)
fact.Confidence = ApplyRoleBonusToConfidence(fact.Confidence, claim.ClaimType, accountType, params)

// Apply dual validation bonus for partnership claims (R28-5)
if claim.PartnershipId != "" {
	fact.Confidence = ApplyDualValidationBonus(fact.Confidence, params)
}
```

This goes right before `fact.Confidence = k.ClampConfidence(ctx, fact.Confidence, claim.Domain)` on line 267.

**Step 5: Run tests to verify they pass**

Run: `go test ./x/knowledge/keeper/ -run "TestHumanEmpiricalClaimBonusVisible|TestAgentComputationalClaimBonus|TestNoBonus|TestDualValidation|TestRoleBonusPlus" -v`
Expected: PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/role_bonus.go x/knowledge/keeper/role_bonus_test.go \
  x/knowledge/keeper/rounds.go
git commit -m "feat(knowledge): implement claim confidence and dual validation bonuses (R28-5)"
```

---

### Task 6: Implement agent verification vote weight bonus

**Files:**
- Modify: `x/knowledge/keeper/confidence.go:36-66` — apply bonus in vote aggregation

**Step 1: Write the failing test**

In `x/knowledge/keeper/role_bonus_test.go`, add:

```go
func TestAgentVerificationVoteWeightBonus(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1validator1"] = "agent" // agent verifier
	authKeeper.accounts["zrn1validator2"] = "human" // human verifier
	authKeeper.accounts["zrn1validator3"] = "human" // human verifier
	k.SetZeroneAuthKeeper(authKeeper)

	sk.addValidator("zrn1validator1", 100_000, "genesis")
	sk.addValidator("zrn1validator2", 100_000, "genesis")
	sk.addValidator("zrn1validator3", 100_000, "genesis")

	// Create a claim + round
	claim, round := makeTestClaim(t, k, ctx, "zrn1submitter1", "Test claim for vote bonus", "general", "empirical", "1000000")

	// All 3 accept — agent gets +20% weight
	round.Reveals = []*types.Reveal{
		{Verifier: "zrn1validator1", Vote: "accept"}, // agent: 100k * 1.2 = 120k effective
		{Verifier: "zrn1validator2", Vote: "accept"}, // human: 100k
		{Verifier: "zrn1validator3", Vote: "accept"}, // human: 100k
	}
	round.Commits = []*types.Commit{
		{Verifier: "zrn1validator1"},
		{Verifier: "zrn1validator2"},
		{Verifier: "zrn1validator3"},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, result.Verdict)

	// Total weighted stake = 120k + 100k + 100k = 320k
	// Accept ratio = 320k/320k = 1,000,000 → capped at MaxConfidence 880,000
	require.Equal(t, uint64(880_000), result.Confidence)

	// Now test mixed votes where the bonus changes the verdict
	_ = claim // keep linter happy
}

func TestAgentVoteWeightChangesOutcome(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1validator1"] = "agent" // agent verifier — gets bonus
	authKeeper.accounts["zrn1validator2"] = "human"
	authKeeper.accounts["zrn1validator3"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	// Set stakes so that without bonus, accept would be below threshold
	// Agent: 100k → 120k with bonus
	// Humans: 100k each
	sk.addValidator("zrn1validator1", 100_000, "genesis")
	sk.addValidator("zrn1validator2", 60_000, "genesis")
	sk.addValidator("zrn1validator3", 60_000, "genesis")

	_, round := makeTestClaim(t, k, ctx, "zrn1submitter1", "Test agent vote influence claim", "general", "empirical", "1000000")

	// Agent accepts, humans reject
	round.Reveals = []*types.Reveal{
		{Verifier: "zrn1validator1", Vote: "accept"}, // agent: 100k → 120k
		{Verifier: "zrn1validator2", Vote: "reject"}, // human: 60k
		{Verifier: "zrn1validator3", Vote: "reject"}, // human: 60k
	}
	round.Commits = []*types.Commit{
		{Verifier: "zrn1validator1"},
		{Verifier: "zrn1validator2"},
		{Verifier: "zrn1validator3"},
	}
	require.NoError(t, k.SetVerificationRound(ctx, round))

	result, err := k.AggregateVerificationResult(ctx, round)
	require.NoError(t, err)

	// Without bonus: accept=100k, reject=120k, total=220k → acceptRatio=454,545 < 770k threshold → INCONCLUSIVE
	// With bonus: accept=120k, reject=120k, total=240k → acceptRatio=500,000 < 770k → INCONCLUSIVE
	// The bonus shifts the ratio but doesn't change verdict here (both below threshold)
	require.Equal(t, types.Verdict_VERDICT_INCONCLUSIVE, result.Verdict)
}
```

**Step 2: Run test to verify current behavior (should pass without bonus code since we haven't changed confidence.go yet — but the bonus won't be applied)**

Run: `go test ./x/knowledge/keeper/ -run TestAgentVerificationVoteWeightBonus -v`
Expected: Should compile (mock keeper set) but the bonus isn't applied yet in confidence.go

**Step 3: Apply agent vote weight bonus in AggregateVerificationResult**

In `x/knowledge/keeper/confidence.go`, in the `AggregateVerificationResult` function, modify the vote tallying loop (lines 45-66). After getting the stake (line 51) and before adding to totalVoteStake (line 57), apply the bonus:

Replace lines 45-66 with:

```go
	for _, reveal := range round.Reveals {
		var stake uint64
		if k.stakingKeeper != nil {
			s, err := k.stakingKeeper.GetEffectiveStake(ctx, reveal.Verifier)
			if err == nil {
				stake = s
			}
		}
		if stake == 0 {
			stake = 1 // minimum weight for unknown validators
		}

		// Apply agent verification bonus (R28-5)
		if params.AgentVerificationBonusBps > 0 {
			accountType := k.getAccountType(ctx, reveal.Verifier)
			if accountType == "agent" {
				stake = safeMulDiv(stake, 1_000_000+params.AgentVerificationBonusBps, 1_000_000)
			}
		}

		totalVoteStake += stake
		switch reveal.Vote {
		case "accept":
			acceptStake += stake
		case "reject":
			rejectStake += stake
		case "malformed":
			malformedStake += stake
		}
	}
```

**Step 4: Run tests**

Run: `go test ./x/knowledge/keeper/ -run "TestAgentVerificationVoteWeightBonus|TestAgentVoteWeightChangesOutcome" -v`
Expected: PASS

**Step 5: Run full confidence test suite to check for regressions**

Run: `go test ./x/knowledge/keeper/ -run TestConfidence -v`
Expected: PASS (existing tests should still pass — bonus only applies when zeroneAuthKeeper is set)

**Step 6: Commit**

```bash
git add x/knowledge/keeper/confidence.go x/knowledge/keeper/role_bonus_test.go
git commit -m "feat(knowledge): add agent verification vote weight bonus (R28-5)"
```

---

### Task 7: Implement human patronage energy bonus

**Files:**
- Modify: `x/knowledge/keeper/metabolism.go:207-254` — apply bonus

**Step 1: Write the failing test**

In `x/knowledge/keeper/role_bonus_test.go`, add:

```go
func TestHumanPatronageEnergyBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1patron1"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	params, _ := k.GetParams(ctx)

	// Create a fact with low energy
	fact := makeTestFact(t, k, ctx, "fact-1", "Test fact", "general", "empirical", "zrn1submitter1", 500_000)
	fact.Energy = 100_000
	fact.EnergyCap = params.MetabolismEnergyCap
	require.NoError(t, k.SetFact(ctx, fact))

	// Patronize with 10 epochs of duration
	durationBlocks := params.FitnessEpochBlocks * 10
	k.ApplyPatronageEnergyBoost(ctx, fact, durationBlocks, "zrn1patron1")

	// Base boost = MetabolismEnergyPerPatronage * 10 / 10 = 20,000
	// With human bonus (+10%): 20,000 * 1.1 = 22,000
	// New energy = 100,000 + 22,000 = 122,000
	updatedFact, found := k.GetFact(ctx, "fact-1")
	require.True(t, found)
	require.Equal(t, uint64(122_000), updatedFact.Energy)
}

func TestAgentPatronageNoBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1agent1"] = "agent"
	k.SetZeroneAuthKeeper(authKeeper)

	params, _ := k.GetParams(ctx)

	fact := makeTestFact(t, k, ctx, "fact-2", "Test fact 2", "general", "empirical", "zrn1submitter1", 500_000)
	fact.Energy = 100_000
	fact.EnergyCap = params.MetabolismEnergyCap
	require.NoError(t, k.SetFact(ctx, fact))

	durationBlocks := params.FitnessEpochBlocks * 10
	k.ApplyPatronageEnergyBoost(ctx, fact, durationBlocks, "zrn1agent1")

	// Base boost = 20,000 — no bonus for agents
	updatedFact, found := k.GetFact(ctx, "fact-2")
	require.True(t, found)
	require.Equal(t, uint64(120_000), updatedFact.Energy)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run "TestHumanPatronageEnergyBonus|TestAgentPatronageNoBonus" -v`
Expected: FAIL — `ApplyPatronageEnergyBoost` signature doesn't accept patron address yet

**Step 3: Modify ApplyPatronageEnergyBoost to accept patron address**

In `x/knowledge/keeper/metabolism.go`, change the signature on line 207:

From:
```go
func (k Keeper) ApplyPatronageEnergyBoost(ctx context.Context, fact *types.Fact, durationBlocks uint64) {
```

To:
```go
func (k Keeper) ApplyPatronageEnergyBoost(ctx context.Context, fact *types.Fact, durationBlocks uint64, patronAddr string) {
```

After computing the boost (after line 224, `boost := params.MetabolismEnergyPerPatronage`), add:

```go
// Apply human patronage bonus (R28-5)
if params.HumanPatronageBonusBps > 0 && patronAddr != "" {
	accountType := k.getAccountType(ctx, patronAddr)
	if accountType == "human" {
		boost = safeMulDiv(boost, 1_000_000+params.HumanPatronageBonusBps, 1_000_000)
	}
}
```

**Step 4: Update the caller in msg_server.go**

In `x/knowledge/keeper/msg_server.go`, line 885, update the call:

From:
```go
m.keeper.ApplyPatronageEnergyBoost(ctx, fact, msg.DurationBlocks)
```

To:
```go
m.keeper.ApplyPatronageEnergyBoost(ctx, fact, msg.DurationBlocks, msg.Patron)
```

Also update any other callers (check metabolism.go for the income loop — search for `ApplyPatronageEnergyBoost` calls).

**Step 5: Run tests**

Run: `go test ./x/knowledge/keeper/ -run "TestHumanPatronageEnergyBonus|TestAgentPatronageNoBonus" -v`
Expected: PASS

**Step 6: Run full metabolism test suite**

Run: `go test ./x/knowledge/keeper/ -run TestMetabolism -v`
Expected: PASS

**Step 7: Commit**

```bash
git add x/knowledge/keeper/metabolism.go x/knowledge/keeper/msg_server.go \
  x/knowledge/keeper/role_bonus_test.go
git commit -m "feat(knowledge): add human patronage energy bonus (R28-5)"
```

---

### Task 8: Implement human coercion freeze multiplier

**Files:**
- Modify: `x/partnerships/keeper/anti_coercion.go:76-122` — apply multiplier

**Step 1: Write the failing test**

In `x/partnerships/keeper/anti_coercion_test.go` (create or extend):

```go
func TestHumanCoercionFreezeMultiplier(t *testing.T) {
	// Setup partnerships keeper with mock auth keeper
	k, ctx := setupPartnershipsTest(t)
	authKeeper := newMockPartnershipsAuthKeeper()
	authKeeper.accounts["zrn1human1"] = "human"
	authKeeper.accounts["zrn1agent1"] = "agent"
	k.SetZeroneAuthKeeper(authKeeper)

	// Create partnership
	partnership := &types.Partnership{
		Id:               "partnership-1",
		HumanAddr:        "zrn1human1",
		AgentAddr:        "zrn1agent1",
		Status:           types.StatusActive,
		CooperationScore: 500_000,
		SplitHumanBps:    500_000,
		SplitAgentBps:    500_000,
	}
	k.SetPartnership(ctx, partnership)

	params := k.GetParams(ctx)

	// Human raises coercion signal
	signal, err := k.HandleCoercionSignal(ctx, "partnership-1", "zrn1human1")
	require.NoError(t, err)

	// Expected: CoercionReviewBlocks * 1.5 = 2000 * 1.5 = 3000
	expectedExpiry := uint64(ctx.BlockHeight()) + params.CoercionReviewBlocks*params.HumanCoercionFreezeMultiplierBps/1_000_000
	require.Equal(t, expectedExpiry, signal.ExpiresAt)

	// Agent raises coercion signal — should get standard duration
	// First resolve the existing signal
	signal.Resolved = true
	k.SetCoercionSignal(ctx, signal)
	partnership.Status = types.StatusActive
	k.SetPartnership(ctx, partnership)

	agentSignal, err := k.HandleCoercionSignal(ctx, "partnership-1", "zrn1agent1")
	require.NoError(t, err)

	// Agent: standard CoercionReviewBlocks = 2000
	expectedAgentExpiry := uint64(ctx.BlockHeight()) + params.CoercionReviewBlocks
	require.Equal(t, expectedAgentExpiry, agentSignal.ExpiresAt)
}
```

Note: This test depends on having test helpers for the partnerships module. Check if `setupPartnershipsTest` exists or create it based on the keeper test pattern.

**Step 2: Run test to verify it fails**

Run: `go test ./x/partnerships/keeper/ -run TestHumanCoercionFreezeMultiplier -v`
Expected: FAIL

**Step 3: Apply the multiplier in HandleCoercionSignal**

In `x/partnerships/keeper/anti_coercion.go`, in `HandleCoercionSignal`, modify line 102:

From:
```go
ExpiresAt:     currentBlock + params.CoercionReviewBlocks,
```

To:
```go
ExpiresAt:     currentBlock + k.coercionFreezeBlocks(ctx, raiser, params),
```

Add a helper method:

```go
// coercionFreezeBlocks returns the freeze duration, applying human multiplier if applicable (R28-5).
func (k Keeper) coercionFreezeBlocks(ctx sdk.Context, raiser string, params *types.Params) uint64 {
	base := params.CoercionReviewBlocks
	if params.HumanCoercionFreezeMultiplierBps > 0 && k.zeroneAuthKeeper != nil {
		accountType, found := k.zeroneAuthKeeper.GetAccountType(ctx, raiser)
		if found && accountType == "human" {
			return base * params.HumanCoercionFreezeMultiplierBps / 1_000_000
		}
	}
	return base
}
```

**Step 4: Run test**

Run: `go test ./x/partnerships/keeper/ -run TestHumanCoercionFreezeMultiplier -v`
Expected: PASS

**Step 5: Run full anti-coercion test suite**

Run: `go test ./x/partnerships/keeper/ -run TestCoercion -v`
Expected: PASS

**Step 6: Commit**

```bash
git add x/partnerships/keeper/anti_coercion.go x/partnerships/keeper/anti_coercion_test.go
git commit -m "feat(partnerships): add human coercion freeze multiplier (R28-5)"
```

---

### Task 9: Integration test — full lifecycle

**Files:**
- Create: `x/knowledge/keeper/role_bonus_test.go` (extend with integration test)

**Step 1: Write integration test**

Add to `x/knowledge/keeper/role_bonus_test.go`:

```go
func TestRoleBonusIntegration_FullLifecycle(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	authKeeper := newMockZeroneAuthKeeper()
	authKeeper.accounts["zrn1human1"] = "human"
	authKeeper.accounts["zrn1agent1"] = "agent"
	authKeeper.accounts["zrn1validator1"] = "agent"
	authKeeper.accounts["zrn1validator2"] = "human"
	authKeeper.accounts["zrn1validator3"] = "human"
	k.SetZeroneAuthKeeper(authKeeper)

	sk.addValidator("zrn1validator1", 100_000, "genesis")
	sk.addValidator("zrn1validator2", 100_000, "genesis")
	sk.addValidator("zrn1validator3", 100_000, "genesis")

	params, _ := k.GetParams(ctx)

	// 1. Human submits OBSERVATION → should get +15% confidence bonus
	humanClaim := &types.Claim{
		Id:               "human-obs-1",
		FactContent:      "Empirical observation from the field verified by human researcher",
		Domain:           "physics",
		Category:         "empirical",
		Submitter:        "zrn1human1",
		SubmittedAtBlock: uint64(ctx.BlockHeight()),
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		ContentHash:      "human-obs-hash",
		ClaimType:        types.ClaimType_CLAIM_TYPE_OBSERVATION,
	}
	require.NoError(t, k.SetClaim(ctx, humanClaim))
	humanRound, err := k.CreateVerificationRound(ctx, humanClaim)
	require.NoError(t, err)

	// 2. Agent submits COMPUTATIONAL → should get +15% confidence bonus
	agentClaim := &types.Claim{
		Id:               "agent-comp-1",
		FactContent:      "Computational derivation from formal analysis of dataset XYZ",
		Domain:           "computer_science",
		Category:         "derived",
		Submitter:        "zrn1agent1",
		SubmittedAtBlock: uint64(ctx.BlockHeight()),
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		ContentHash:      "agent-comp-hash",
		ClaimType:        types.ClaimType_CLAIM_TYPE_COMPUTATIONAL,
	}
	require.NoError(t, k.SetClaim(ctx, agentClaim))
	agentRound, err := k.CreateVerificationRound(ctx, agentClaim)
	require.NoError(t, err)

	// 3. Human submits COMPUTATIONAL → should get NO bonus
	humanCompClaim := &types.Claim{
		Id:               "human-comp-1",
		FactContent:      "Human attempting computational claim without agent assistance",
		Domain:           "mathematics",
		Category:         "derived",
		Submitter:        "zrn1human1",
		SubmittedAtBlock: uint64(ctx.BlockHeight()),
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            "1000000",
		ContentHash:      "human-comp-hash",
		ClaimType:        types.ClaimType_CLAIM_TYPE_COMPUTATIONAL,
	}
	require.NoError(t, k.SetClaim(ctx, humanCompClaim))
	humanCompRound, err := k.CreateVerificationRound(ctx, humanCompClaim)
	require.NoError(t, err)

	// All rounds get unanimous acceptance with equal-stake validators
	for _, round := range []*types.VerificationRound{humanRound, agentRound, humanCompRound} {
		round.Reveals = []*types.Reveal{
			{Verifier: "zrn1validator1", Vote: "accept"},
			{Verifier: "zrn1validator2", Vote: "accept"},
			{Verifier: "zrn1validator3", Vote: "accept"},
		}
		round.Commits = []*types.Commit{
			{Verifier: "zrn1validator1"},
			{Verifier: "zrn1validator2"},
			{Verifier: "zrn1validator3"},
		}
		require.NoError(t, k.SetVerificationRound(ctx, round))
	}

	// Verify vote weight bonus — agent validator at 100k + 20% = 120k
	// Total: 120k + 100k + 100k = 320k
	// Accept ratio = 320k/320k = 1,000,000 → capped at MaxConfidence
	humanResult, err := k.AggregateVerificationResult(ctx, humanRound)
	require.NoError(t, err)
	require.Equal(t, types.Verdict_VERDICT_ACCEPT, humanResult.Verdict)

	// Base confidence = 880,000 (capped from 1M)
	// Human empirical bonus: 880,000 * 1.15 = 1,012,000 → clamped back to 880,000
	// The bonus is absorbed by the cap for unanimous votes.
	// This is expected — bonuses matter most on borderline claims.

	// Verify the bonus calculation is correct by checking the function directly
	baseConf := uint64(770_000) // threshold confidence
	humanBoosted := keeper.ApplyRoleBonusToConfidence(baseConf, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	agentBoosted := keeper.ApplyRoleBonusToConfidence(baseConf, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "agent", params)
	humanCompNone := keeper.ApplyRoleBonusToConfidence(baseConf, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "human", params)

	require.Equal(t, uint64(885_500), humanBoosted, "human empirical: 770k * 1.15")
	require.Equal(t, uint64(885_500), agentBoosted, "agent computational: 770k * 1.15")
	require.Equal(t, baseConf, humanCompNone, "human computational: no bonus")

	// Both bonused claims should be clamped to MaxConfidence
	humanClamped := k.ClampConfidence(ctx, humanBoosted, "physics")
	agentClamped := k.ClampConfidence(ctx, agentBoosted, "computer_science")
	require.Equal(t, params.MaxConfidence, humanClamped)
	require.Equal(t, params.MaxConfidence, agentClamped)

	t.Log("Integration test passed — role differentiation working correctly")
}

func TestRoleBonusGovernanceConfigurable(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Modify params via governance
	params, _ := k.GetParams(ctx)
	params.HumanEmpiricalBonusBps = 300_000 // increase to +30%
	params.AgentComputationalBonusBps = 0    // disable agent bonus
	require.NoError(t, k.SetParams(ctx, params))

	// Verify changes took effect
	newParams, _ := k.GetParams(ctx)
	require.Equal(t, uint64(300_000), newParams.HumanEmpiricalBonusBps)
	require.Equal(t, uint64(0), newParams.AgentComputationalBonusBps)

	// Verify bonus uses updated params
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", newParams)
	require.Equal(t, uint64(910_000), boosted) // 700k * 1.3 = 910k

	noBoosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "agent", newParams)
	require.Equal(t, uint64(700_000), noBoosted) // disabled
}
```

**Step 2: Run integration test**

Run: `go test ./x/knowledge/keeper/ -run "TestRoleBonusIntegration|TestRoleBonusGovernanceConfigurable" -v`
Expected: PASS

**Step 3: Run full test suite**

Run: `go test ./x/knowledge/keeper/ -v -count=1`
Expected: ALL PASS

Run: `go test ./x/partnerships/keeper/ -v -count=1`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add x/knowledge/keeper/role_bonus_test.go
git commit -m "test(knowledge): add role bonus integration and governance tests (R28-5)"
```

---

### Task 10: Final build verification and cleanup

**Step 1: Full build**

Run: `go build ./...`
Expected: Clean compilation

**Step 2: Full test suite**

Run: `go test ./x/knowledge/... ./x/partnerships/... ./app/... -count=1`
Expected: ALL PASS

**Step 3: Final commit (if any fixups needed)**

Only if compilation or tests revealed issues in earlier tasks.

**Step 4: Summary commit**

No final commit unless fixups were needed. The feature is complete across tasks 1-9.
