# R29-3 Domain Role Elasticity Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace static R28-5 role bonuses with domain-specific elastic bonuses that scale based on each role's track record of correctness.

**Architecture:** JSON-encoded `DomainRoleRecord` stored per domain (prefix 0x55), updated on vindication and challenge resolution. `GetRoleElasticity()` computes scaled bonuses. 4 new params, 1 new query, 1 new event. Follows R29-1/R29-2 patterns exactly.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, protobuf for query messages, JSON for internal state.

---

### Task 1: Store Key and Type Definitions

**Files:**
- Modify: `x/knowledge/types/keys.go:129` (add after DomainStatsPrefix)
- Create: `x/knowledge/types/role_elasticity.go`

**Step 1: Add store key prefix**

In `x/knowledge/types/keys.go`, add after the `DomainStatsPrefix` line (line 129):

```go
	// ─── Domain role elasticity (R29-3) ────────────────────────────────
	DomainRoleRecordPrefix = []byte{0x55} // 0x55 | domain → DomainRoleRecord (JSON)
```

Add key constructor after `DomainStatsKey` function (after line 368):

```go
// DomainRoleRecordKey returns the store key for a domain's role track record.
func DomainRoleRecordKey(domain string) []byte {
	return append(append([]byte{}, DomainRoleRecordPrefix...), []byte(domain)...)
}
```

**Step 2: Create type definition**

Create `x/knowledge/types/role_elasticity.go`:

```go
package types

// DomainRoleRecord tracks the correctness of agent vs human majorities
// within a specific domain. Updated on vindication and challenge resolution.
type DomainRoleRecord struct {
	Domain              string `json:"domain"`
	AgentCorrectCalls   uint64 `json:"agent_correct_calls"`
	AgentIncorrectCalls uint64 `json:"agent_incorrect_calls"`
	HumanCorrectCalls   uint64 `json:"human_correct_calls"`
	HumanIncorrectCalls uint64 `json:"human_incorrect_calls"`
	LastUpdated         uint64 `json:"last_updated"`
}
```

**Step 3: Run tests to verify nothing is broken**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add x/knowledge/types/keys.go x/knowledge/types/role_elasticity.go
git commit -m "feat(knowledge): add DomainRoleRecord type and store key prefix (R29-3)"
```

---

### Task 2: Params — Proto, Defaults, Validation

**Files:**
- Modify: `proto/zerone/knowledge/v1/genesis.proto:183` (add after epistemic temperature params)
- Modify: `x/knowledge/types/genesis.go:179` (add defaults after epistemic params)
- Modify: `x/knowledge/types/genesis.go:561` (add validation before final `return nil`)

**Step 1: Add proto fields**

In `proto/zerone/knowledge/v1/genesis.proto`, add after line 183 (after `epistemic_temperature_window_blocks`):

```protobuf
  // ─── Domain role elasticity (R29-3) ──────────────────────────────────
  uint64 role_elasticity_min_calls          = 125; // Min calls per role before elasticity activates (default: 10)
  uint64 role_elasticity_max_multiplier_bps = 126; // Max bonus scaling (default: 2,000,000 = 200%)
  uint64 role_elasticity_min_multiplier_bps = 127; // Min bonus scaling (default: 500,000 = 50%)
  uint64 role_elasticity_decay_epochs       = 128; // Blocks between 5% decay cycles (default: 100)
```

**Step 2: Regenerate proto**

Run: `cd /Users/yournameisai/Desktop/zerone && make proto-gen` (or the project's proto generation command)
If proto-gen is unavailable, manually add the fields to `x/knowledge/types/genesis.pb.go` (the generated file).

**Step 3: Add default values**

In `x/knowledge/types/genesis.go`, add after the epistemic temperature defaults block (after line 179, before the closing `}`):

```go
		// ─── Domain role elasticity (R29-3) ──────────────────────────────
		RoleElasticityMinCalls:         10,
		RoleElasticityMaxMultiplierBps: 2_000_000, // 200% max bonus scaling
		RoleElasticityMinMultiplierBps: 500_000,   // 50% min bonus scaling
		RoleElasticityDecayEpochs:      100,        // decay every 100 blocks
```

**Step 4: Add validation**

In `x/knowledge/types/genesis.go`, add before `return nil` (before line 562):

```go
	// ─── Domain role elasticity (R29-3) ──────────────────────────────
	if p.RoleElasticityMinCalls == 0 {
		return fmt.Errorf("role_elasticity_min_calls must be > 0")
	}
	if p.RoleElasticityMaxMultiplierBps < 1_000_000 {
		return fmt.Errorf("role_elasticity_max_multiplier_bps must be >= 1,000,000 (at least 100%%)")
	}
	if p.RoleElasticityMinMultiplierBps > 1_000_000 {
		return fmt.Errorf("role_elasticity_min_multiplier_bps must be <= 1,000,000 (at most 100%%)")
	}
	if p.RoleElasticityMinMultiplierBps >= p.RoleElasticityMaxMultiplierBps {
		return fmt.Errorf("role_elasticity_min_multiplier_bps must be < max_multiplier_bps")
	}
	if p.RoleElasticityDecayEpochs == 0 {
		return fmt.Errorf("role_elasticity_decay_epochs must be > 0")
	}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/types/... -v -run TestValidate -count=1`
Expected: PASS (existing validation tests should still pass)

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: BUILD SUCCESS

**Step 6: Commit**

```bash
git add proto/zerone/knowledge/v1/genesis.proto x/knowledge/types/genesis.go x/knowledge/types/genesis.pb.go
git commit -m "feat(knowledge): add role elasticity params to genesis (R29-3)"
```

---

### Task 3: CRUD and Core Logic — Write Tests First

**Files:**
- Create: `x/knowledge/keeper/role_elasticity_test.go`
- Create: `x/knowledge/keeper/role_elasticity.go`

**Step 1: Write the failing tests**

Create `x/knowledge/keeper/role_elasticity_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── CRUD Tests ──────────────────────────────────────────────────────────

func TestDomainRoleRecord_SetGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:              "physics",
		AgentCorrectCalls:   8,
		AgentIncorrectCalls: 2,
		HumanCorrectCalls:   6,
		HumanIncorrectCalls: 4,
		LastUpdated:         100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	got, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, record.AgentCorrectCalls, got.AgentCorrectCalls)
	require.Equal(t, record.HumanIncorrectCalls, got.HumanIncorrectCalls)
}

func TestDomainRoleRecord_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	_, found := k.GetDomainRoleRecord(ctx, "nonexistent")
	require.False(t, found)
}

// ─── Elasticity Calculation Tests ────────────────────────────────────────

func TestGetRoleElasticity_BelowMinCalls(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Only 5 calls per role — below min_calls=10
	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 4, AgentIncorrectCalls: 1,
		HumanCorrectCalls: 3, HumanIncorrectCalls: 2,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	// Should return base bonuses unchanged
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)
}

func TestGetRoleElasticity_EqualAccuracy(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Both 80% accurate — multiplier should be 1.0× for both
	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 16, AgentIncorrectCalls: 4,
		HumanCorrectCalls: 16, HumanIncorrectCalls: 4,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	// Equal accuracy → both get 1.0× base
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)
}

func TestGetRoleElasticity_AgentDominant(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Agents 90% accurate, humans 60% accurate
	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 18, AgentIncorrectCalls: 2,
		HumanCorrectCalls: 12, HumanIncorrectCalls: 8,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	// Agent should get > base, human should get < base
	require.Greater(t, agentBonus, params.AgentVerificationBonusBps)
	require.Less(t, humanBonus, params.HumanPatronageBonusBps)
}

func TestGetRoleElasticity_BoundedMax(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Agents 100% accurate, humans 10% — should hit 200% cap
	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 20, AgentIncorrectCalls: 0,
		HumanCorrectCalls: 2, HumanIncorrectCalls: 18,
		LastUpdated: 100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	// Agent capped at 200%
	require.Equal(t, params.AgentVerificationBonusBps*2, agentBonus)
	// Human at 50% floor
	require.Equal(t, params.HumanPatronageBonusBps/2, humanBonus)
}

func TestGetRoleElasticity_NoDomain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)
	// No record exists → base bonuses
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "unknown_domain")
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)
}

// ─── CountVotesByAccountType Tests ───────────────────────────────────────

func TestCountVotesByAccountType(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["agent2"] = "agent"
	mockAuth.accounts["human1"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.Reveal{
			{Verifier: "agent1", Vote: "accept"},
			{Verifier: "agent2", Vote: "accept"},
			{Verifier: "human1", Vote: "reject"},
		},
	}

	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	require.Equal(t, uint64(2), agentVotes)
	require.Equal(t, uint64(1), humanVotes)
}

func TestCountVotesByAccountType_NoAuthKeeper(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	// No auth keeper set

	round := &types.VerificationRound{
		Id: "round1",
		Reveals: []*types.Reveal{
			{Verifier: "someone", Vote: "accept"},
		},
	}

	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	require.Equal(t, uint64(0), agentVotes)
	require.Equal(t, uint64(0), humanVotes)
}

// ─── Decay Tests ─────────────────────────────────────────────────────────

func TestDecayRoleRecords(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:              "physics",
		AgentCorrectCalls:   1000,
		AgentIncorrectCalls: 200,
		HumanCorrectCalls:   800,
		HumanIncorrectCalls: 100,
		LastUpdated:         100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	k.DecayRoleRecords(ctx)

	got, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	// 1000 * 950,000 / 1,000,000 = 950
	require.Equal(t, uint64(950), got.AgentCorrectCalls)
	// 200 * 950,000 / 1,000,000 = 190
	require.Equal(t, uint64(190), got.AgentIncorrectCalls)
	// 800 * 950,000 / 1,000,000 = 760
	require.Equal(t, uint64(760), got.HumanCorrectCalls)
	// 100 * 950,000 / 1,000,000 = 95
	require.Equal(t, uint64(95), got.HumanIncorrectCalls)
}

func TestDecayRoleRecords_SmallValues(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:            "physics",
		AgentCorrectCalls: 1, // 1 * 950,000 / 1,000,000 = 0
		LastUpdated:       100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	k.DecayRoleRecords(ctx)

	got, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(0), got.AgentCorrectCalls) // decayed to zero
}

// ─── GetRoleAccuracies Tests ─────────────────────────────────────────────

func TestGetRoleAccuracies(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	record := &types.DomainRoleRecord{
		Domain:              "physics",
		AgentCorrectCalls:   18,
		AgentIncorrectCalls: 2,
		HumanCorrectCalls:   12,
		HumanIncorrectCalls: 8,
		LastUpdated:         100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	agentAcc, humanAcc := k.GetRoleAccuracies(ctx, "physics")
	// Agent: 18/20 = 900,000 BPS
	require.Equal(t, uint64(900_000), agentAcc)
	// Human: 12/20 = 600,000 BPS
	require.Equal(t, uint64(600_000), humanAcc)
}

func TestGetRoleAccuracies_NoCalls(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	agentAcc, humanAcc := k.GetRoleAccuracies(ctx, "unknown")
	require.Equal(t, uint64(0), agentAcc)
	require.Equal(t, uint64(0), humanAcc)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -v -run "TestDomainRoleRecord|TestGetRoleElasticity|TestCountVotesByAccountType|TestDecayRoleRecords|TestGetRoleAccuracies" -count=1`
Expected: FAIL — methods don't exist yet

**Step 3: Write implementation**

Create `x/knowledge/keeper/role_elasticity.go`:

```go
package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── CRUD ────────────────────────────────────────────────────────────────────

// SetDomainRoleRecord stores the role track record for a domain as JSON.
func (k Keeper) SetDomainRoleRecord(ctx context.Context, record *types.DomainRoleRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal DomainRoleRecord: %w", err)
	}
	return store.Set(types.DomainRoleRecordKey(record.Domain), bz)
}

// GetDomainRoleRecord retrieves the role track record for a domain.
func (k Keeper) GetDomainRoleRecord(ctx context.Context, domain string) (*types.DomainRoleRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DomainRoleRecordKey(domain))
	if err != nil || bz == nil {
		return nil, false
	}
	var record types.DomainRoleRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return nil, false
	}
	return &record, true
}

// IterateDomainRoleRecords iterates all domain role records.
func (k Keeper) IterateDomainRoleRecords(ctx context.Context, cb func(record *types.DomainRoleRecord) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DomainRoleRecordPrefix, prefixEndBytes(types.DomainRoleRecordPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var record types.DomainRoleRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		if cb(&record) {
			break
		}
	}
}

// ─── Elasticity Calculation ──────────────────────────────────────────────────

// clampUint64 clamps a value between min and max.
func clampUint64(val, minVal, maxVal uint64) uint64 {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

// GetRoleElasticity returns domain-adjusted bonus BPS for agent and human roles.
// If the domain lacks sufficient track record, returns base bonuses from params.
func (k Keeper) GetRoleElasticity(ctx context.Context, domain string) (agentBonusBps, humanBonusBps uint64) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 0, 0
	}

	agentBase := params.AgentVerificationBonusBps
	humanBase := params.HumanPatronageBonusBps

	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		return agentBase, humanBase
	}

	agentTotal := record.AgentCorrectCalls + record.AgentIncorrectCalls
	humanTotal := record.HumanCorrectCalls + record.HumanIncorrectCalls

	if agentTotal < params.RoleElasticityMinCalls || humanTotal < params.RoleElasticityMinCalls {
		return agentBase, humanBase
	}

	agentAccuracy := safeMulDiv(record.AgentCorrectCalls, BPS, agentTotal)
	humanAccuracy := safeMulDiv(record.HumanCorrectCalls, BPS, humanTotal)

	total := agentAccuracy + humanAccuracy
	if total == 0 {
		return agentBase, humanBase
	}

	agentMultiplier := clampUint64(
		safeMulDiv(agentAccuracy*2, BPS, total),
		params.RoleElasticityMinMultiplierBps,
		params.RoleElasticityMaxMultiplierBps,
	)
	humanMultiplier := clampUint64(
		safeMulDiv(humanAccuracy*2, BPS, total),
		params.RoleElasticityMinMultiplierBps,
		params.RoleElasticityMaxMultiplierBps,
	)

	return safeMulDiv(agentBase, agentMultiplier, BPS), safeMulDiv(humanBase, humanMultiplier, BPS)
}

// GetRoleAccuracies returns accuracy BPS for each role in a domain.
// Returns (0, 0) if no track record exists.
func (k Keeper) GetRoleAccuracies(ctx context.Context, domain string) (agentAccBps, humanAccBps uint64) {
	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		return 0, 0
	}

	agentTotal := record.AgentCorrectCalls + record.AgentIncorrectCalls
	humanTotal := record.HumanCorrectCalls + record.HumanIncorrectCalls

	if agentTotal > 0 {
		agentAccBps = safeMulDiv(record.AgentCorrectCalls, BPS, agentTotal)
	}
	if humanTotal > 0 {
		humanAccBps = safeMulDiv(record.HumanCorrectCalls, BPS, humanTotal)
	}
	return
}

// ─── Vote Counting ───────────────────────────────────────────────────────────

// CountVotesByAccountType counts how many agent vs human verifiers participated
// in a verification round's reveals.
func (k Keeper) CountVotesByAccountType(ctx context.Context, round *types.VerificationRound) (agentVotes, humanVotes uint64) {
	for _, reveal := range round.Reveals {
		accountType := k.getAccountType(ctx, reveal.Verifier)
		switch accountType {
		case "agent":
			agentVotes++
		case "human":
			humanVotes++
		}
	}
	return
}

// ─── Track Record Updates ────────────────────────────────────────────────────

// RecordVindicationRoleImpact updates domain role records when vindication occurs.
// The majority was wrong — increment incorrect calls for the dominant role.
func (k Keeper) RecordVindicationRoleImpact(ctx context.Context, round *types.VerificationRound, domain string) {
	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	if agentVotes == humanVotes {
		return // mixed majority — no role-specific attribution
	}

	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		record = &types.DomainRoleRecord{Domain: domain}
	}

	if agentVotes > humanVotes {
		record.AgentIncorrectCalls++
	} else {
		record.HumanIncorrectCalls++
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	record.LastUpdated = uint64(sdkCtx.BlockHeight())
	_ = k.SetDomainRoleRecord(ctx, record)

	k.emitRoleElasticityEvent(ctx, domain)
}

// RecordChallengeRoleImpact updates domain role records when a challenge is resolved.
// If upheld (original verifiers were wrong), increment incorrect. If rejected, increment correct.
func (k Keeper) RecordChallengeRoleImpact(ctx context.Context, factId, domain string, upheld bool) error {
	// Find the verification round that established this fact
	round := k.GetVerificationRoundForFact(ctx, factId)
	if round == nil {
		return nil // no round found — skip silently
	}

	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	if agentVotes == humanVotes {
		return nil // mixed majority
	}

	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		record = &types.DomainRoleRecord{Domain: domain}
	}

	if upheld {
		// Challenge upheld = original verifiers were wrong
		if agentVotes > humanVotes {
			record.AgentIncorrectCalls++
		} else {
			record.HumanIncorrectCalls++
		}
	} else {
		// Challenge rejected = original verifiers were right
		if agentVotes > humanVotes {
			record.AgentCorrectCalls++
		} else {
			record.HumanCorrectCalls++
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	record.LastUpdated = uint64(sdkCtx.BlockHeight())
	_ = k.SetDomainRoleRecord(ctx, record)

	k.emitRoleElasticityEvent(ctx, domain)
	return nil
}

// GetVerificationRoundForFact finds the verification round that established a fact.
// Follows fact → claim → round chain via store lookups.
func (k Keeper) GetVerificationRoundForFact(ctx context.Context, factId string) *types.VerificationRound {
	fact, found := k.GetFact(ctx, factId)
	if !found || fact.ClaimId == "" {
		return nil
	}
	claim, found := k.GetClaim(ctx, fact.ClaimId)
	if !found || claim.VerificationRoundId == "" {
		return nil
	}
	round, found := k.GetVerificationRound(ctx, claim.VerificationRoundId)
	if !found {
		return nil
	}
	return round
}

// ─── Decay ───────────────────────────────────────────────────────────────────

// DecayRoleRecords applies 5% exponential decay to all role records.
// Called periodically from BeginBlocker.
func (k Keeper) DecayRoleRecords(ctx context.Context) {
	var records []*types.DomainRoleRecord
	k.IterateDomainRoleRecords(ctx, func(record *types.DomainRoleRecord) bool {
		records = append(records, record)
		return false
	})

	for _, record := range records {
		record.AgentCorrectCalls = safeMulDiv(record.AgentCorrectCalls, 950_000, BPS)
		record.AgentIncorrectCalls = safeMulDiv(record.AgentIncorrectCalls, 950_000, BPS)
		record.HumanCorrectCalls = safeMulDiv(record.HumanCorrectCalls, 950_000, BPS)
		record.HumanIncorrectCalls = safeMulDiv(record.HumanIncorrectCalls, 950_000, BPS)
		_ = k.SetDomainRoleRecord(ctx, record)
	}
}

// ─── Events ──────────────────────────────────────────────────────────────────

// emitRoleElasticityEvent emits an event when role elasticity is updated.
func (k Keeper) emitRoleElasticityEvent(ctx context.Context, domain string) {
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, domain)
	agentAcc, humanAcc := k.GetRoleAccuracies(ctx, domain)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.role_elasticity_updated",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("agent_bonus_bps", fmt.Sprintf("%d", agentBonus)),
		sdk.NewAttribute("human_bonus_bps", fmt.Sprintf("%d", humanBonus)),
		sdk.NewAttribute("agent_accuracy_bps", fmt.Sprintf("%d", agentAcc)),
		sdk.NewAttribute("human_accuracy_bps", fmt.Sprintf("%d", humanAcc)),
	))
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -v -run "TestDomainRoleRecord|TestGetRoleElasticity|TestCountVotesByAccountType|TestDecayRoleRecords|TestGetRoleAccuracies" -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/role_elasticity.go x/knowledge/keeper/role_elasticity_test.go
git commit -m "feat(knowledge): implement role elasticity CRUD, calculation, decay, and helpers (R29-3)"
```

---

### Task 4: Integration — Vindication Hook

**Files:**
- Modify: `x/knowledge/keeper/vindication.go:200` (after ExecuteVindication call in handleChallengeDisproven)

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/role_elasticity_test.go`:

```go
func TestVindicationRoleImpact_AgentMajority(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["agent2"] = "agent"
	mockAuth.accounts["human1"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	// Simulate: agents were majority but got it wrong (vindicated)
	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.Reveal{
			{Verifier: "agent1", Vote: "accept"},
			{Verifier: "agent2", Vote: "accept"},
			{Verifier: "human1", Vote: "reject"},
		},
	}

	k.RecordVindicationRoleImpact(ctx, round, "physics")

	record, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(1), record.AgentIncorrectCalls)
	require.Equal(t, uint64(0), record.HumanIncorrectCalls)
}

func TestVindicationRoleImpact_HumanMajority(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["human1"] = "human"
	mockAuth.accounts["human2"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.Reveal{
			{Verifier: "agent1", Vote: "reject"},
			{Verifier: "human1", Vote: "accept"},
			{Verifier: "human2", Vote: "accept"},
		},
	}

	k.RecordVindicationRoleImpact(ctx, round, "ecology")

	record, found := k.GetDomainRoleRecord(ctx, "ecology")
	require.True(t, found)
	require.Equal(t, uint64(0), record.AgentIncorrectCalls)
	require.Equal(t, uint64(1), record.HumanIncorrectCalls)
}
```

**Step 2: Run test to verify it passes (logic already in Task 3)**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -v -run "TestVindicationRoleImpact" -count=1`
Expected: PASS (RecordVindicationRoleImpact was already implemented)

**Step 3: Wire into vindication**

In `x/knowledge/keeper/vindication.go`, in `ExecuteVindication()`, add after `k.DeleteVindicationPending(ctx, factId)` (line 324) and before the event emission (line 327):

```go
	// Record role impact — the majority was wrong (R29-3)
	k.RecordVindicationRoleImpact(ctx, round, k.getDomainForFact(ctx, factId))
```

Add helper method to `role_elasticity.go`:

```go
// getDomainForFact returns the domain of a fact, or "" if not found.
func (k Keeper) getDomainForFact(ctx context.Context, factId string) string {
	fact, found := k.GetFact(ctx, factId)
	if !found {
		return ""
	}
	return fact.Domain
}
```

**Step 4: Run full knowledge test suite**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/vindication.go x/knowledge/keeper/role_elasticity.go x/knowledge/keeper/role_elasticity_test.go
git commit -m "feat(knowledge): wire vindication role impact into ExecuteVindication (R29-3)"
```

---

### Task 5: Integration — Challenge Resolution Hook

**Files:**
- Modify: `x/capture_challenge/types/expected_keepers.go:29-31` (expand KnowledgeKeeper interface)
- Modify: `x/capture_challenge/keeper/msg_server.go:258` (add call after IncreaseVerificationThreshold)

**Step 1: Expand KnowledgeKeeper interface**

In `x/capture_challenge/types/expected_keepers.go`, modify the `KnowledgeKeeper` interface (lines 28-31):

```go
// KnowledgeKeeper allows adjusting verification thresholds and recording role impact on confirmed capture.
type KnowledgeKeeper interface {
	IncreaseVerificationThreshold(ctx context.Context, domain string, additionalVerifiers uint32, expiryHeight uint64) error
	RecordChallengeRoleImpact(ctx context.Context, factId, domain string, upheld bool) error
}
```

**Step 2: Add call in ResolveChallenge**

In `x/capture_challenge/keeper/msg_server.go`, after the `IncreaseVerificationThreshold` call (after line 258), add:

```go
		// Record role impact for domain elasticity (R29-3)
		if err := m.knowledgeKeeper.RecordChallengeRoleImpact(ctx, challenge.FactId, challenge.Domain, true); err != nil {
			m.Logger(ctx).Error("failed to record challenge role impact", "domain", challenge.Domain, "err", err)
		}
```

For the REJECTED case (after line 297, after `ClearCaptureFlag`), add:

```go
		// Record role impact — challenge rejected means original verifiers were right (R29-3)
		if m.knowledgeKeeper != nil {
			if err := m.knowledgeKeeper.RecordChallengeRoleImpact(ctx, challenge.FactId, challenge.Domain, false); err != nil {
				m.Logger(ctx).Error("failed to record challenge role impact", "domain", challenge.Domain, "err", err)
			}
		}
```

**Step 3: Verify build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add x/capture_challenge/types/expected_keepers.go x/capture_challenge/keeper/msg_server.go
git commit -m "feat(capture_challenge): hook RecordChallengeRoleImpact into ResolveChallenge (R29-3)"
```

---

### Task 6: Integration — Replace Static Bonuses

**Files:**
- Modify: `x/knowledge/keeper/confidence.go:57-63` (agent vote weight)
- Modify: `x/knowledge/keeper/metabolism.go:250-256` (human patronage energy)
- Modify: `x/knowledge/keeper/rounds.go:296-298` (dual validation bonus)

**Step 1: Write integration test**

Add to `x/knowledge/keeper/role_elasticity_test.go`:

```go
func TestIntegration_AgentDominantDomain_BonusRises(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["human1"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	// Set up a domain where agents have been very accurate
	record := &types.DomainRoleRecord{
		Domain:              "physics",
		AgentCorrectCalls:   18,
		AgentIncorrectCalls: 2,
		HumanCorrectCalls:   10,
		HumanIncorrectCalls: 10,
		LastUpdated:         100,
	}
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	params, _ := k.GetParams(ctx)

	// Agent bonus should be > base
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	require.Greater(t, agentBonus, params.AgentVerificationBonusBps)
	// Human bonus should be < base (50% accuracy vs 90%)
	require.Less(t, humanBonus, params.HumanPatronageBonusBps)
}
```

**Step 2: Replace agent vote weight bonus**

In `x/knowledge/keeper/confidence.go`, replace lines 57-63:

```go
		// Apply agent verification bonus (R28-5), modulated by domain role elasticity (R29-3)
		if params.AgentVerificationBonusBps > 0 {
			accountType := k.getAccountType(ctx, reveal.Verifier)
			if accountType == "agent" {
				// Get domain from the claim associated with this round
				domain := k.getDomainForRound(ctx, round)
				agentBonus, _ := k.GetRoleElasticity(ctx, domain)
				stake = safeMulDiv(stake, 1_000_000+agentBonus, 1_000_000)
			}
		}
```

Add helper to `role_elasticity.go`:

```go
// getDomainForRound returns the domain of the claim in a verification round.
func (k Keeper) getDomainForRound(ctx context.Context, round *types.VerificationRound) string {
	if round == nil || round.ClaimId == "" {
		return ""
	}
	claim, found := k.GetClaim(ctx, round.ClaimId)
	if !found {
		return ""
	}
	return claim.Domain
}
```

**Step 3: Replace human patronage energy bonus**

In `x/knowledge/keeper/metabolism.go`, replace lines 250-256:

```go
	// Apply human patronage bonus (R28-5), modulated by domain role elasticity (R29-3)
	if params.HumanPatronageBonusBps > 0 && patronAddr != "" {
		accountType := k.getAccountType(ctx, patronAddr)
		if accountType == "human" {
			_, humanBonus := k.GetRoleElasticity(ctx, fact.Domain)
			boost = safeMulDiv(boost, 1_000_000+humanBonus, 1_000_000)
		}
	}
```

**Step 4: Scale dual validation bonus**

In `x/knowledge/keeper/rounds.go`, replace lines 295-298 (inside `createFactFromClaim`):

```go
	// Apply dual validation bonus for partnership claims (R28-5)
	// Scale by weaker role's accuracy in the domain (R29-3)
	if claim.PartnershipId != "" {
		agentAcc, humanAcc := k.GetRoleAccuracies(ctx, claim.Domain)
		weakerAccuracy := agentAcc
		if humanAcc < weakerAccuracy || weakerAccuracy == 0 {
			weakerAccuracy = humanAcc
		}
		if weakerAccuracy > 0 {
			scaledBonusBps := safeMulDiv(params.DualValidationBonusBps, weakerAccuracy, BPS)
			fact.Confidence = safeMulDiv(fact.Confidence, 1_000_000+scaledBonusBps, 1_000_000)
		} else {
			// No track record yet — use full static bonus
			fact.Confidence = ApplyDualValidationBonus(fact.Confidence, params)
		}
	}
```

**Step 5: Run all tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -count=1`
Expected: ALL PASS (existing role_bonus_test.go tests should still pass since they don't set domain records — elasticity returns base values when no record exists)

**Step 6: Commit**

```bash
git add x/knowledge/keeper/confidence.go x/knowledge/keeper/metabolism.go x/knowledge/keeper/rounds.go x/knowledge/keeper/role_elasticity.go
git commit -m "feat(knowledge): replace static role bonuses with domain-elastic bonuses (R29-3)"
```

---

### Task 7: BeginBlocker Decay Integration

**Files:**
- Modify: `x/knowledge/keeper/phases.go:67` (add decay call in fitness epoch block)

**Step 1: Add decay to BeginBlocker**

In `x/knowledge/keeper/phases.go`, add after the epistemic temperature update block (after line 67, still inside the `if params.FitnessEpochBlocks > 0` block):

```go
		// 10. Decay domain role elasticity records (R29-3)
		if params.RoleElasticityDecayEpochs > 0 && epoch%params.RoleElasticityDecayEpochs == 0 {
			k.DecayRoleRecords(ctx)
		}
```

**Step 2: Run build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: BUILD SUCCESS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/phases.go
git commit -m "feat(knowledge): add role elasticity decay to BeginBlocker (R29-3)"
```

---

### Task 8: Query — Proto, gRPC, CLI

**Files:**
- Modify: `proto/zerone/knowledge/v1/query.proto:192` (add RPC)
- Modify: `x/knowledge/keeper/grpc_query.go:901` (add handler after DomainCapacity)
- Modify: `x/knowledge/client/cli/query.go:85-86` (add CLI command)

**Step 1: Add proto definitions**

In `proto/zerone/knowledge/v1/query.proto`, add after `EpistemicTemperature` RPC (after line 192, before the closing `}`):

```protobuf

  // RoleElasticity queries domain role elasticity and track record (R29-3).
  rpc RoleElasticity(QueryRoleElasticityRequest) returns (QueryRoleElasticityResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/role_elasticity/{domain}";
  }
```

Add request/response messages at the end of the file (after line 564):

```protobuf

// ─── Role elasticity query messages (R29-3) ──────────────────────────────────

message QueryRoleElasticityRequest {
  string domain = 1;
}

message QueryRoleElasticityResponse {
  string domain              = 1;
  uint64 agent_correct       = 2;
  uint64 agent_incorrect     = 3;
  uint64 human_correct       = 4;
  uint64 human_incorrect     = 5;
  uint64 agent_bonus_bps     = 6;
  uint64 human_bonus_bps     = 7;
  uint64 agent_accuracy_bps  = 8;
  uint64 human_accuracy_bps  = 9;
}
```

**Step 2: Regenerate proto**

Run: `cd /Users/yournameisai/Desktop/zerone && make proto-gen` (or manually update `query.pb.go`, `query.pb.gw.go`, `query_grpc.pb.go`)

**Step 3: Add gRPC handler**

In `x/knowledge/keeper/grpc_query.go`, add after the `DomainCapacity` function (after line 901):

```go

// RoleElasticity queries domain role elasticity and track record (R29-3).
func (q *queryServer) RoleElasticity(ctx context.Context, req *types.QueryRoleElasticityRequest) (*types.QueryRoleElasticityResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	record, _ := q.keeper.GetDomainRoleRecord(ctx, req.Domain)
	agentBonus, humanBonus := q.keeper.GetRoleElasticity(ctx, req.Domain)
	agentAcc, humanAcc := q.keeper.GetRoleAccuracies(ctx, req.Domain)

	resp := &types.QueryRoleElasticityResponse{
		Domain:          req.Domain,
		AgentBonusBps:   agentBonus,
		HumanBonusBps:   humanBonus,
		AgentAccuracyBps: agentAcc,
		HumanAccuracyBps: humanAcc,
	}

	if record != nil {
		resp.AgentCorrect = record.AgentCorrectCalls
		resp.AgentIncorrect = record.AgentIncorrectCalls
		resp.HumanCorrect = record.HumanCorrectCalls
		resp.HumanIncorrect = record.HumanIncorrectCalls
	}

	return resp, nil
}
```

**Step 4: Add CLI command**

In `x/knowledge/client/cli/query.go`, add to the `queryCmd.AddCommand(...)` list (after line 85):

```go
		NewQueryRoleElasticityCmd(),
```

Add the function at the end of the file (after line 1042):

```go

// NewQueryRoleElasticityCmd queries domain role elasticity (R29-3).
func NewQueryRoleElasticityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role-elasticity [domain]",
		Short: "Query role elasticity and track record for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryRoleElasticityRequest{Domain: args[0]}
			resp := &types.QueryRoleElasticityResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/RoleElasticity", req, resp); err != nil {
				return fmt.Errorf("failed to query role elasticity: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 5: Run build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: BUILD SUCCESS

**Step 6: Commit**

```bash
git add proto/zerone/knowledge/v1/query.proto x/knowledge/types/query.pb.go x/knowledge/types/query.pb.gw.go x/knowledge/types/query_grpc.pb.go x/knowledge/keeper/grpc_query.go x/knowledge/client/cli/query.go
git commit -m "feat(knowledge): add RoleElasticity gRPC query and CLI command (R29-3)"
```

---

### Task 9: Full Integration Test

**Files:**
- Modify: `x/knowledge/keeper/role_elasticity_test.go` (add full lifecycle test)

**Step 1: Write full lifecycle integration test**

Add to `x/knowledge/keeper/role_elasticity_test.go`:

```go
func TestFullLifecycle_VindicationUpdatesElasticity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	mockAuth := newMockZeroneAuthKeeper()
	mockAuth.accounts["agent1"] = "agent"
	mockAuth.accounts["agent2"] = "agent"
	mockAuth.accounts["human1"] = "human"
	k.SetZeroneAuthKeeper(mockAuth)

	params, _ := k.GetParams(ctx)

	// Initially: no track record, base bonuses
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, "physics")
	require.Equal(t, params.AgentVerificationBonusBps, agentBonus)
	require.Equal(t, params.HumanPatronageBonusBps, humanBonus)

	// Simulate 15 vindications where agents were the majority (and wrong)
	round := &types.VerificationRound{
		Id:      "round1",
		ClaimId: "claim1",
		Reveals: []*types.Reveal{
			{Verifier: "agent1", Vote: "accept"},
			{Verifier: "agent2", Vote: "accept"},
			{Verifier: "human1", Vote: "reject"},
		},
	}

	for i := 0; i < 15; i++ {
		k.RecordVindicationRoleImpact(ctx, round, "physics")
	}

	record, found := k.GetDomainRoleRecord(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(15), record.AgentIncorrectCalls)
	require.Equal(t, uint64(0), record.AgentCorrectCalls)
	// Agents only have incorrect calls, but humans have 0 total —
	// elasticity won't activate until both roles have ≥ minCalls

	// Now add enough human data: 10 correct
	record.HumanCorrectCalls = 10
	require.NoError(t, k.SetDomainRoleRecord(ctx, record))

	// Now elasticity activates: agent accuracy = 0%, human accuracy = 100%
	// Agent should get min (50%), human should get max (200%)
	agentBonus, humanBonus = k.GetRoleElasticity(ctx, "physics")
	require.Equal(t, params.AgentVerificationBonusBps/2, agentBonus)    // 50% of base
	require.Equal(t, params.HumanPatronageBonusBps*2, humanBonus)       // 200% of base

	// Decay should reduce counts
	k.DecayRoleRecords(ctx)
	record, _ = k.GetDomainRoleRecord(ctx, "physics")
	require.Less(t, record.AgentIncorrectCalls, uint64(15))
	require.Less(t, record.HumanCorrectCalls, uint64(10))
}
```

**Step 2: Run the test**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -v -run "TestFullLifecycle" -count=1`
Expected: PASS

**Step 3: Run all knowledge tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -count=1`
Expected: ALL PASS

**Step 4: Run capture_challenge tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/capture_challenge/... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/role_elasticity_test.go
git commit -m "test(knowledge): add full lifecycle integration test for domain role elasticity (R29-3)"
```

---

### Task 10: Code Review

Use `superpowers:requesting-code-review` to review all R29-3 changes against the spec and coding standards.

---

## Files Summary

| File | Action | Task |
|------|--------|------|
| `x/knowledge/types/keys.go` | Modify | 1 |
| `x/knowledge/types/role_elasticity.go` | Create | 1 |
| `proto/zerone/knowledge/v1/genesis.proto` | Modify | 2 |
| `x/knowledge/types/genesis.go` | Modify | 2 |
| `x/knowledge/types/genesis.pb.go` | Regen | 2 |
| `x/knowledge/keeper/role_elasticity.go` | Create | 3 |
| `x/knowledge/keeper/role_elasticity_test.go` | Create | 3, 4, 9 |
| `x/knowledge/keeper/vindication.go` | Modify | 4 |
| `x/capture_challenge/types/expected_keepers.go` | Modify | 5 |
| `x/capture_challenge/keeper/msg_server.go` | Modify | 5 |
| `x/knowledge/keeper/confidence.go` | Modify | 6 |
| `x/knowledge/keeper/metabolism.go` | Modify | 6 |
| `x/knowledge/keeper/rounds.go` | Modify | 6 |
| `x/knowledge/keeper/phases.go` | Modify | 7 |
| `proto/zerone/knowledge/v1/query.proto` | Modify | 8 |
| `x/knowledge/types/query.pb.go` | Regen | 8 |
| `x/knowledge/types/query.pb.gw.go` | Regen | 8 |
| `x/knowledge/types/query_grpc.pb.go` | Regen | 8 |
| `x/knowledge/keeper/grpc_query.go` | Modify | 8 |
| `x/knowledge/client/cli/query.go` | Modify | 8 |
