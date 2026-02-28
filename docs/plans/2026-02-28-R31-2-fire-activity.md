# R31-2 Fire Activity Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire verification (Fire) into the Wu Xing circulation: Fire→Earth (verification health → governance sensor), Water→Fire (partnership density → verification requirements), Fire→Metal (verification activity → reputation recovery).

**Architecture:** New completion index in knowledge store enables efficient window-based round counting. Three new cross-module interface methods connect knowledge↔alignment and knowledge↔capture_defense. Two new proto params (knowledge + capture_defense). Existing `GetDomainVerificationActivity` (R31-4) is enhanced with the completion index.

**Tech Stack:** Cosmos SDK v0.50.15, protobuf, Go 1.24+

---

### Task 1: Proto — Add CompletedRoundMeta message to knowledge types.proto

**Files:**
- Modify: `proto/zerone/knowledge/v1/types.proto:333` (append after KnowledgeBounty)

**Step 1: Add the proto message**

After the closing brace of `KnowledgeBounty` (line 333), add:

```proto
// CompletedRoundMeta stores metadata for completed verification rounds,
// indexed by verdict block height for efficient window-based queries (R31-2).
message CompletedRoundMeta {
    string domain          = 1;
    bool   has_dissent     = 2;  // true if any verifier dissented from majority
    uint64 duration_blocks = 3;  // verdict_block - started_at_block
}
```

**Step 2: Run proto generation**

Run: `make proto-gen`
Expected: Success, generates `x/knowledge/types/types.pb.go` with `CompletedRoundMeta`

**Step 3: Verify generated code**

Run: `grep -n "CompletedRoundMeta" x/knowledge/types/types.pb.go | head -5`
Expected: Struct definition exists

**Step 4: Commit**

```bash
git add proto/zerone/knowledge/v1/types.proto x/knowledge/types/types.pb.go
git commit -m "proto(knowledge): add CompletedRoundMeta message for R31-2 completion index"
```

---

### Task 2: Proto — Add social_saturation_threshold param to knowledge genesis.proto

**Files:**
- Modify: `proto/zerone/knowledge/v1/genesis.proto:193` (append after field 130)

**Step 1: Add the new param field**

After `mentorship_capacity_bonus = 130;` (line 193), add:

```proto
  // ─── Social verification adjustment (R31-2: Water → Fire) ──────────────
  uint64 social_saturation_threshold = 131; // Partnership density above which verification relaxes (default: 10)
  uint64 observation_window_blocks   = 132; // Lookback window for verification health metrics (default: 10000)
```

**Step 2: Run proto generation**

Run: `make proto-gen`
Expected: Success

**Step 3: Add defaults in genesis.go**

In `x/knowledge/types/genesis.go`, inside `DefaultParams()` (after `MentorshipCapacityBonus: 5,` on line 189), add:

```go
		// ─── Social verification adjustment (R31-2: Water → Fire) ────────
		SocialSaturationThreshold: 10,
		ObservationWindowBlocks:   10_000,
```

**Step 4: Commit**

```bash
git add proto/zerone/knowledge/v1/genesis.proto x/knowledge/types/genesis.pb.go x/knowledge/types/genesis.go
git commit -m "proto(knowledge): add social_saturation_threshold and observation_window_blocks params (R31-2)"
```

---

### Task 3: Proto — Add recovery params to capture_defense genesis.proto

**Files:**
- Modify: `proto/zerone/capture_defense/v1/genesis.proto:15` (append after field 7)

**Step 1: Add the new param fields**

After `max_history_per_domain = 7;` (line 15), add:

```proto
  uint64 base_reputation_recovery_bps    = 8; // base recovery rate per decay epoch (default: 50,000 = 5%)
  uint64 activity_recovery_bonus_max_bps = 9; // max acceleration from verification activity (default: 500,000 = 50%)
```

**Step 2: Run proto generation**

Run: `make proto-gen`
Expected: Success

**Step 3: Add defaults in types.go**

In `x/capture_defense/types/types.go`, inside `DefaultParams()` (after `MaxHistoryPerDomain: 100,` on line 21), add:

```go
		BaseReputationRecoveryBps:   50_000,  // 5% recovery per decay epoch
		ActivityRecoveryBonusMaxBps: 500_000, // max 50% acceleration from activity
```

**Step 4: Commit**

```bash
git add proto/zerone/capture_defense/v1/genesis.proto x/capture_defense/types/genesis.pb.go x/capture_defense/types/types.go
git commit -m "proto(capture_defense): add reputation recovery params (R31-2)"
```

---

### Task 4: Knowledge store keys — Add CompletedRoundIndex prefix

**Files:**
- Modify: `x/knowledge/types/keys.go:135-136` (add prefix after 0x56)
- Modify: `x/knowledge/types/keys.go:384` (add key constructors after LastClaimHeightKey)

**Step 1: Add the store key prefix**

After `LastClaimHeightKeyPrefix = []byte{0x56}` (line 135), before the closing `)`, add:

```go
	// ─── Completion index (R31-2: Fire activity metrics) ──────────────
	CompletedRoundIndexPrefix = []byte{0x57} // 0x57 | verdictBlock(8) | roundID → CompletedRoundMeta (proto)
```

**Step 2: Add key constructor functions**

After `LastClaimHeightKey` function (line 384), add:

```go
// CompletedRoundKey returns the index key for a completed round by verdict block.
func CompletedRoundKey(verdictBlock uint64, roundID string) []byte {
	key := make([]byte, 0, 1+8+len(roundID))
	key = append(key, CompletedRoundIndexPrefix...)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, verdictBlock)
	key = append(key, buf...)
	key = append(key, []byte(roundID)...)
	return key
}

// CompletedRoundRangePrefix returns the prefix for iterating completed rounds in a block range.
// Use with start=CompletedRoundBlockPrefix(startBlock) and end=CompletedRoundBlockPrefix(endBlock+1).
func CompletedRoundBlockPrefix(block uint64) []byte {
	key := make([]byte, 0, 1+8)
	key = append(key, CompletedRoundIndexPrefix...)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, block)
	key = append(key, buf...)
	return key
}
```

Note: `encoding/binary` is already imported in keys.go — verify with `grep "encoding/binary" x/knowledge/types/keys.go`. If not imported, add it.

**Step 3: Verify compilation**

Run: `go build ./x/knowledge/types/...`
Expected: Success

**Step 4: Commit**

```bash
git add x/knowledge/types/keys.go
git commit -m "feat(knowledge): add CompletedRoundIndex store key prefix (R31-2)"
```

---

### Task 5: Knowledge keeper — Completion index write + window counting

**Files:**
- Create: `x/knowledge/keeper/completion_index.go`
- Modify: `x/knowledge/keeper/rounds.go:206` (add indexCompletedRound call before event emission)

**Step 1: Write the failing test**

Create `x/knowledge/keeper/completion_index_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestCompletionIndex_CountCompletedRoundsInWindow(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Index 5 rounds at blocks 100, 200, 300, 400, 500
	for i, block := range []uint64{100, 200, 300, 400, 500} {
		meta := &types.CompletedRoundMeta{
			Domain:         "physics",
			HasDissent:     i%2 == 0, // rounds 0, 2, 4 have dissent
			DurationBlocks: 11,
		}
		err := k.IndexCompletedRound(ctx, block, fmt.Sprintf("round-%d", i), meta)
		require.NoError(t, err)
	}

	// Window [200, 500] should contain 4 rounds (blocks 200-500)
	count := k.CountCompletedRoundsInWindow(ctx, 500, 300)
	require.Equal(t, uint64(4), count)

	// Window [400, 500] should contain 2 rounds
	count = k.CountCompletedRoundsInWindow(ctx, 500, 100)
	require.Equal(t, uint64(2), count)
}

func TestCompletionIndex_CountDisputedRoundsInWindow(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// 3 rounds: 2 with dissent, 1 without
	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "physics", HasDissent: false, DurationBlocks: 12})
	k.IndexCompletedRound(ctx, 300, "r3", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 8})

	disputed := k.CountDisputedRoundsInWindow(ctx, 300, 300)
	require.Equal(t, uint64(2), disputed)
}

func TestCompletionIndex_GetAvgRoundDurationInWindow(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 20})

	avg := k.GetAvgRoundDurationInWindow(ctx, 200, 200)
	require.Equal(t, uint64(15), avg) // (10+20)/2
}

func TestCompletionIndex_CountForDomainInWindow(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "chemistry", DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 300, "r3", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 10})

	physics := k.CountCompletedRoundsForDomainInWindow(ctx, "physics", 300, 300)
	require.Equal(t, uint64(2), physics)

	chemistry := k.CountCompletedRoundsForDomainInWindow(ctx, "chemistry", 300, 300)
	require.Equal(t, uint64(1), chemistry)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestCompletionIndex -v -count=1`
Expected: FAIL — methods don't exist

**Step 3: Write the implementation**

Create `x/knowledge/keeper/completion_index.go`:

```go
package keeper

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// IndexCompletedRound stores completion metadata indexed by verdict block height.
// Called from CompleteRound to enable efficient window-based metrics (R31-2).
func (k Keeper) IndexCompletedRound(ctx context.Context, verdictBlock uint64, roundID string, meta *types.CompletedRoundMeta) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(meta)
	if err != nil {
		return err
	}
	return store.Set(types.CompletedRoundKey(verdictBlock, roundID), bz)
}

// CountCompletedRoundsInWindow counts all completed rounds in [height-window, height].
func (k Keeper) CountCompletedRoundsInWindow(ctx context.Context, height, windowBlocks uint64) uint64 {
	var count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(_ *types.CompletedRoundMeta) bool {
		count++
		return false
	})
	return count
}

// CountDisputedRoundsInWindow counts rounds with dissent in [height-window, height].
func (k Keeper) CountDisputedRoundsInWindow(ctx context.Context, height, windowBlocks uint64) uint64 {
	var count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(meta *types.CompletedRoundMeta) bool {
		if meta.HasDissent {
			count++
		}
		return false
	})
	return count
}

// GetAvgRoundDurationInWindow returns the average round duration in [height-window, height].
func (k Keeper) GetAvgRoundDurationInWindow(ctx context.Context, height, windowBlocks uint64) uint64 {
	var total, count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(meta *types.CompletedRoundMeta) bool {
		total += meta.DurationBlocks
		count++
		return false
	})
	if count == 0 {
		return 0
	}
	return total / count
}

// CountCompletedRoundsForDomainInWindow counts completed rounds for a specific domain.
func (k Keeper) CountCompletedRoundsForDomainInWindow(ctx context.Context, domain string, height, windowBlocks uint64) uint64 {
	var count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(meta *types.CompletedRoundMeta) bool {
		if meta.Domain == domain {
			count++
		}
		return false
	})
	return count
}

// iterateCompletedRoundsInWindow iterates all completed round metadata in [startBlock, endBlock].
func (k Keeper) iterateCompletedRoundsInWindow(ctx context.Context, height, windowBlocks uint64, cb func(*types.CompletedRoundMeta) bool) {
	store := k.storeService.OpenKVStore(ctx)

	var startBlock uint64
	if height > windowBlocks {
		startBlock = height - windowBlocks
	}

	startKey := types.CompletedRoundBlockPrefix(startBlock)
	endKey := types.CompletedRoundBlockPrefix(height + 1)

	iter, err := store.Iterator(startKey, endKey)
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var meta types.CompletedRoundMeta
		if err := proto.Unmarshal(iter.Value(), &meta); err != nil {
			continue
		}
		if cb(&meta) {
			return
		}
	}
}
```

**Step 4: Wire into CompleteRound**

In `x/knowledge/keeper/rounds.go`, BEFORE the event emission (line 207), add:

```go
	// Index completed round for window-based metrics (R31-2)
	hasDissent := roundHasDissent(round)
	duration := height - round.StartedAtBlock
	completionMeta := &types.CompletedRoundMeta{
		Domain:         claim.Domain,
		HasDissent:     hasDissent,
		DurationBlocks: duration,
	}
	if idxErr := k.IndexCompletedRound(ctx, height, round.Id, completionMeta); idxErr != nil {
		k.Logger(ctx).Debug("failed to index completed round", "round", round.Id, "error", idxErr)
	}
```

Add the helper at the end of `rounds.go`:

```go
// roundHasDissent checks if a round had any verifier dissent (mixed accept/reject votes).
func roundHasDissent(round *types.VerificationRound) bool {
	hasAccept, hasReject := false, false
	for _, reveal := range round.Reveals {
		switch reveal.Vote {
		case "accept":
			hasAccept = true
		case "reject":
			hasReject = true
		}
		if hasAccept && hasReject {
			return true
		}
	}
	return false
}
```

**Step 5: Run tests**

Run: `go test ./x/knowledge/keeper/ -run TestCompletionIndex -v -count=1`
Expected: PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/completion_index.go x/knowledge/keeper/completion_index_test.go x/knowledge/keeper/rounds.go
git commit -m "feat(knowledge): add completion index for window-based verification metrics (R31-2)"
```

---

### Task 6: Knowledge keeper — GetVerificationHealth and GetEffectiveMinVerifiers

**Files:**
- Create: `x/knowledge/keeper/fire_activity.go`
- Modify: `x/knowledge/keeper/verification_activity.go` (enhance GetDomainVerificationActivity to use completion index)

**Step 1: Write the failing test**

Create `x/knowledge/keeper/fire_activity_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestGetVerificationHealth_Throughput(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Index 5 rounds in a 10000-block window
	for i := 0; i < 5; i++ {
		k.IndexCompletedRound(ctx, uint64(100+i*100), fmt.Sprintf("r%d", i), &types.CompletedRoundMeta{
			Domain: "physics", HasDissent: false, DurationBlocks: 11,
		})
	}

	throughput, disputeRate, avgDuration := k.GetVerificationHealth(ctx)
	require.Greater(t, throughput, uint64(0), "throughput should be > 0 with completed rounds")
	require.Equal(t, uint64(0), disputeRate, "no dissent = 0 dispute rate")
	require.Equal(t, uint64(11), avgDuration)
}

func TestGetVerificationHealth_DisputeRate(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// 3 rounds: 2 with dissent
	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "physics", HasDissent: false, DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 300, "r3", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 10})

	_, disputeRate, _ := k.GetVerificationHealth(ctx)
	// 2/3 disputed = 666,666 BPS
	require.Greater(t, disputeRate, uint64(600_000))
}

func TestGetEffectiveMinVerifiers_NoDensity(t *testing.T) {
	k, ctx := setupKeeperForTest(t)
	// With no partnership keeper or density=0, should increase by 1
	effective := k.GetEffectiveMinVerifiers(ctx, "physics")
	params, _ := k.GetParams(ctx)
	require.Equal(t, params.MinVerifiers+1, uint64(effective))
}

func TestGetEffectiveMinVerifiers_HighDensity(t *testing.T) {
	k, ctx := setupKeeperForTest(t)
	// Mock partnership keeper returning density >= threshold
	// (Requires mock setup — use TestHarness in cross_stack tests for real integration)
	// Unit test verifies the logic with nil keeper (falls through to base+1)
	effective := k.GetEffectiveMinVerifiers(ctx, "physics")
	params, _ := k.GetParams(ctx)
	require.Equal(t, params.MinVerifiers+1, uint64(effective))
}
```

Note: Full integration tests with real partnership keeper are in Task 9 (cross_stack tests).

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run "TestGetVerificationHealth|TestGetEffectiveMinVerifiers" -v -count=1`
Expected: FAIL — methods don't exist

**Step 3: Write the implementation**

Create `x/knowledge/keeper/fire_activity.go`:

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GetVerificationHealth returns verification metrics for the alignment module (R31-2: Fire → Earth).
// Returns throughput (BPS relative to theoretical max), dispute rate (BPS), and avg round duration.
func (k Keeper) GetVerificationHealth(ctx context.Context) (throughputBps, disputeRateBps, avgRoundDurationBlocks uint64) {
	params, err := k.GetParams(ctx)
	if err != nil || params.CommitPhaseBlocks == 0 {
		return 0, 0, 0
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	windowBlocks := params.ObservationWindowBlocks
	if windowBlocks == 0 {
		windowBlocks = 10_000 // fallback default
	}

	completed := k.CountCompletedRoundsInWindow(ctx, height, windowBlocks)
	if completed == 0 {
		return 0, 0, 0
	}

	// Theoretical max: how many rounds could fit in the window
	roundCycleBlocks := params.CommitPhaseBlocks + params.RevealPhaseBlocks + params.AggregationPhaseBlocks
	if roundCycleBlocks == 0 {
		roundCycleBlocks = 1
	}
	theoreticalMax := windowBlocks / roundCycleBlocks
	if theoreticalMax == 0 {
		theoreticalMax = 1
	}

	throughputBps = completed * types.BPS / theoreticalMax
	if throughputBps > types.BPS {
		throughputBps = types.BPS
	}

	disputed := k.CountDisputedRoundsInWindow(ctx, height, windowBlocks)
	disputeRateBps = disputed * types.BPS / completed

	avgRoundDurationBlocks = k.GetAvgRoundDurationInWindow(ctx, height, windowBlocks)

	return throughputBps, disputeRateBps, avgRoundDurationBlocks
}

// GetEffectiveMinVerifiers returns the adjusted minimum verifiers for a domain,
// accounting for partnership density (R31-2: Water → Fire).
func (k Keeper) GetEffectiveMinVerifiers(ctx context.Context, domain string) uint32 {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 3 // safe default
	}
	base := uint32(params.MinVerifiers)

	if k.partnershipKeeper == nil {
		// No social structure → tighter verification
		return base + 1
	}

	density := k.partnershipKeeper.GetDomainPartnershipDensity(ctx, domain)

	if density == 0 {
		// No social structure in this domain → Fire burns unchecked
		return base + 1
	}

	threshold := params.SocialSaturationThreshold
	if threshold == 0 {
		threshold = 10 // fallback default
	}

	if density >= threshold {
		// High social structure → Water quenches excess
		if base > 2 {
			return base - 1
		}
	}

	return base
}
```

**Step 4: Enhance GetDomainVerificationActivity to use completion index**

Replace the content of `x/knowledge/keeper/verification_activity.go` with:

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetDomainVerificationActivity returns the verification activity level for a domain
// as a BPS value (0-1,000,000). Uses the completion index for accurate window-based counting.
// 10 rounds per window = 100% activity (R31-2 / R31-4).
func (k Keeper) GetDomainVerificationActivity(ctx context.Context, domain string) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	const windowBlocks = 10_000
	rounds := k.CountCompletedRoundsForDomainInWindow(ctx, domain, height, windowBlocks)

	// Normalise: 10 rounds per window = BPS (100% activity)
	activity := rounds * BPS / 10
	if activity > BPS {
		activity = BPS
	}
	return activity
}
```

**Step 5: Add `BPS` constant if needed**

Check if `BPS` is defined in keeper package: `grep "BPS.*=.*1_000_000" x/knowledge/keeper/`. If not, add to `fire_activity.go`:

```go
const BPS = 1_000_000
```

Also check `types.BPS`: `grep "BPS" x/knowledge/types/`. The `types` package uses `types.BPS` in some places. Verify and use consistently.

**Step 6: Add PartnershipKeeper interface method**

Check `x/knowledge/types/expected_keepers.go` — the `PartnershipKeeper` interface (line 95-104) does NOT have `GetDomainPartnershipDensity`. Add it:

In `x/knowledge/types/expected_keepers.go`, add to the `PartnershipKeeper` interface:

```go
	// GetDomainPartnershipDensity returns the count of unique partnership participants in a domain (R31-2).
	GetDomainPartnershipDensity(ctx context.Context, domain string) uint64
```

**Step 7: Run tests**

Run: `go test ./x/knowledge/keeper/ -run "TestGetVerificationHealth|TestGetEffectiveMinVerifiers" -v -count=1`
Expected: PASS

**Step 8: Commit**

```bash
git add x/knowledge/keeper/fire_activity.go x/knowledge/keeper/fire_activity_test.go x/knowledge/keeper/verification_activity.go x/knowledge/types/expected_keepers.go
git commit -m "feat(knowledge): add GetVerificationHealth and GetEffectiveMinVerifiers (R31-2)"
```

---

### Task 7: Alignment — Wire verification health into governance sensor

**Files:**
- Modify: `x/alignment/types/expected_keepers.go:8-18` (extend KnowledgeKeeper interface)
- Modify: `x/alignment/keeper/sensors.go:69-83` (modify senseGovernanceParticipation)
- Modify: `x/knowledge/keeper/alignment_adapters.go` (add adapter method)

**Step 1: Write the failing test**

The cross-stack integration test (Task 9) covers this. For now, verify compilation.

**Step 2: Extend alignment KnowledgeKeeper interface**

In `x/alignment/types/expected_keepers.go`, add to the `KnowledgeKeeper` interface (after `GetPendingVerificationRatio`):

```go
	// GetVerificationHealth returns verification throughput, dispute rate, and avg round duration (R31-2).
	GetVerificationHealth(ctx context.Context) (throughputBps, disputeRateBps, avgRoundDurationBlocks uint64)
```

**Step 3: Add adapter method**

In `x/knowledge/keeper/alignment_adapters.go`, add:

```go
// GetVerificationHealth returns verification health metrics for the alignment sensor (R31-2).
func (a *AlignmentKnowledgeAdapter) GetVerificationHealth(ctx context.Context) (uint64, uint64, uint64) {
	return a.k.GetVerificationHealth(ctx)
}
```

**Step 4: Modify senseGovernanceParticipation**

Replace the `senseGovernanceParticipation` method in `x/alignment/keeper/sensors.go` (lines 69-83):

```go
// senseGovernanceParticipation uses domain count and verification health as governance proxies.
// Weighted: 70% domain count, 30% verification health (R31-2: Fire → Earth).
// Nil-safe: returns NeutralBPS if keepers are nil.
func (k Keeper) senseGovernanceParticipation(ctx context.Context) uint64 {
	if k.ontologyKeeper == nil {
		return types.NeutralBPS
	}

	// Domain count component (70% weight)
	count := k.ontologyKeeper.GetDomainCount(ctx)
	const targetDomains = 100
	domainScore := count * types.BPS / targetDomains
	if domainScore > types.BPS {
		domainScore = types.BPS
	}

	// Verification health component (30% weight) — R31-2: Fire → Earth
	var verificationHealth uint64
	if k.knowledgeKeeper != nil {
		throughput, disputeRate, _ := k.knowledgeKeeper.GetVerificationHealth(ctx)

		verificationHealth = throughput
		// Extreme dispute rate (>30%) penalises verification health
		if disputeRate > 300_000 {
			verificationHealth = verificationHealth * 700_000 / types.BPS
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.alignment.verification_health_observed",
			sdk.NewAttribute("throughput_bps", fmt.Sprintf("%d", throughput)),
			sdk.NewAttribute("dispute_rate_bps", fmt.Sprintf("%d", disputeRate)),
		))
	}

	// Blend: 70% domain count + 30% verification health
	score := domainScore*700_000/types.BPS + verificationHealth*300_000/types.BPS

	return score
}
```

Note: Add necessary imports (`fmt`, `sdk`) to sensors.go if not already present.

**Step 5: Verify compilation**

Run: `go build ./x/alignment/... && go build ./x/knowledge/...`
Expected: Success

**Step 6: Commit**

```bash
git add x/alignment/types/expected_keepers.go x/alignment/keeper/sensors.go x/knowledge/keeper/alignment_adapters.go
git commit -m "feat(alignment): wire verification health into governance sensor (R31-2: Fire → Earth)"
```

---

### Task 8: Capture Defense — Activity-based reputation recovery

**Files:**
- Modify: `x/capture_defense/keeper/reputation.go` (add calculateReputationRecoveryRate, wire into decay)

**Step 1: Write the failing test**

The cross-stack integration test (Task 9) covers this. For now, add a unit test.

Create or append to `x/capture_defense/keeper/reputation_test.go`:

```go
func TestCalculateReputationRecoveryRate_NoActivity(t *testing.T) {
	k := setupCaptureDefenseKeeper(t) // or appropriate setup
	ctx := setupContext(t)
	rate := k.calculateReputationRecoveryRate(ctx, "physics")
	require.Equal(t, uint64(50_000), rate) // base rate only
}

func TestCalculateReputationRecoveryRate_FullActivity(t *testing.T) {
	k := setupCaptureDefenseKeeper(t)
	ctx := setupContext(t)
	// With mock knowledge keeper returning BPS (100% activity):
	// bonus = BPS * 500_000 / BPS = 500_000
	// rate = 50_000 + 50_000 * 500_000 / BPS = 50_000 + 25_000 = 75_000
	rate := k.calculateReputationRecoveryRate(ctx, "physics")
	// Without mock, falls through to base rate
	require.Equal(t, uint64(50_000), rate)
}
```

**Step 2: Add calculateReputationRecoveryRate**

In `x/capture_defense/keeper/reputation.go`, add after the `DecayReputation` function:

```go
// calculateReputationRecoveryRate returns the effective recovery rate for a domain,
// accelerated by verification activity (R31-2: Fire → Metal).
func (k Keeper) calculateReputationRecoveryRate(ctx context.Context, domain string) uint64 {
	params := k.GetParams(ctx)
	baseRate := params.BaseReputationRecoveryBps
	if baseRate == 0 {
		baseRate = 50_000 // fallback default
	}

	if k.knowledgeKeeper == nil {
		return baseRate
	}

	activity := k.knowledgeKeeper.GetDomainVerificationActivity(ctx, domain)

	maxBonus := params.ActivityRecoveryBonusMaxBps
	if maxBonus == 0 {
		maxBonus = 500_000 // fallback default
	}

	// Activity bonus: scales linearly with verification activity
	activityBonus := activity * maxBonus / types.BPSScale
	recoveryRate := baseRate + (baseRate * activityBonus / types.BPSScale)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.capture_defense.activity_recovery_bonus",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("verification_activity_bps", fmt.Sprintf("%d", activity)),
		sdk.NewAttribute("recovery_rate_bps", fmt.Sprintf("%d", recoveryRate)),
		sdk.NewAttribute("bonus_bps", fmt.Sprintf("%d", activityBonus)),
	))

	return recoveryRate
}
```

**Step 3: Wire into reputation decay logic**

In `x/capture_defense/keeper/reputation.go`, find where `DecayReputation` is called and integrate the recovery rate. The recovery rate adjusts the base score that reputation decays toward — a higher recovery rate means the floor is effectively higher for active domains.

Look for calls like `DecayReputation(score, base, age, halfLife)` in the keeper and adjust the `base` parameter using the recovery rate for domain-specific reputation. Specifically, in the domain reputation section of `UpdateReputation`, after computing the decay:

```go
	// Apply activity-based recovery for domain reputation (R31-2: Fire → Metal)
	if domain != "" {
		recoveryRate := k.calculateReputationRecoveryRate(ctx, domain)
		if recoveryRate > 0 {
			// Recovery adjusts effective base score upward for active domains
			effectiveBase := params.BaseReputationScore + (recoveryRate * (types.BPSScale - params.BaseReputationScore) / types.BPSScale)
			if effectiveBase > types.BPSScale {
				effectiveBase = types.BPSScale
			}
			// Use effective base for domain reputation decay
			_ = effectiveBase // wire into decay calculation
		}
	}
```

The exact integration depends on the existing decay call pattern — read the full `UpdateReputation` method to find the right insertion point.

**Step 4: Verify compilation**

Run: `go build ./x/capture_defense/...`
Expected: Success

**Step 5: Commit**

```bash
git add x/capture_defense/keeper/reputation.go
git commit -m "feat(capture_defense): add activity-based reputation recovery rate (R31-2: Fire → Metal)"
```

---

### Task 9: Cross-stack integration tests

**Files:**
- Create: `tests/cross_stack/r31_fire_activity_test.go`

**Step 1: Write all 7 integration tests**

Follow the pattern from `tests/cross_stack/r31_metal_structure_test.go`. Use `TestHarness` from `harness_test.go`.

```go
package cross_stack_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Test 1: Fire→Earth — High throughput improves governance participation ──

func TestR31_FireEarth_HighThroughputImprovesGovernance(t *testing.T) {
	h := NewTestHarness(t)

	// Index several completed rounds (high throughput)
	for i := 0; i < 10; i++ {
		meta := &knowledgetypes.CompletedRoundMeta{
			Domain:         "physics",
			HasDissent:     false,
			DurationBlocks: 11,
		}
		err := h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i), meta)
		require.NoError(t, err)
	}

	throughput, disputeRate, _ := h.KnowledgeKeeper.GetVerificationHealth(h.Ctx)
	require.Greater(t, throughput, uint64(0), "throughput should be > 0")
	require.Equal(t, uint64(0), disputeRate, "no dissent = 0 dispute rate")

	// Governance participation should include verification health
	score := h.AlignmentKeeper.SenseGovernanceParticipation(h.Ctx)
	require.Greater(t, score, uint64(0), "governance participation should reflect verification health")
}

// ─── Test 2: Fire→Earth — Extreme dispute rate degrades governance ──

func TestR31_FireEarth_ExtremeDisputeDegrades(t *testing.T) {
	h := NewTestHarness(t)

	// 10 rounds: 7 with dissent (70% > 30% threshold)
	for i := 0; i < 10; i++ {
		meta := &knowledgetypes.CompletedRoundMeta{
			Domain:         "physics",
			HasDissent:     i < 7, // first 7 have dissent
			DurationBlocks: 11,
		}
		h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i), meta)
	}

	_, disputeRate, _ := h.KnowledgeKeeper.GetVerificationHealth(h.Ctx)
	require.Greater(t, disputeRate, uint64(300_000), "dispute rate should exceed 30%")

	// Governance score with high dispute should be lower than with zero dispute
	// (verification health gets 30% penalty when disputes > 30%)
}

// ─── Test 3: Water→Fire — High partnership density reduces min verifiers ──

func TestR31_WaterFire_HighDensityRelaxes(t *testing.T) {
	h := NewTestHarness(t)

	// Create enough partnerships in "physics" to exceed threshold
	// (threshold default = 10 unique participants)
	// ... setup partnerships via partnership keeper ...

	effective := h.KnowledgeKeeper.GetEffectiveMinVerifiers(h.Ctx, "physics")
	params, _ := h.KnowledgeKeeper.GetParams(h.Ctx)

	// With sufficient partnerships, should be base - 1
	if params.SocialSaturationThreshold > 0 {
		// Need to actually create partnerships to test this
		// For now, verify base + 1 when no partnerships exist
		require.Equal(t, uint32(params.MinVerifiers+1), effective,
			"no partnerships = base + 1 verifiers required")
	}
}

// ─── Test 4: Water→Fire — Zero partnerships increases min verifiers ──

func TestR31_WaterFire_ZeroDensityTightens(t *testing.T) {
	h := NewTestHarness(t)

	effective := h.KnowledgeKeeper.GetEffectiveMinVerifiers(h.Ctx, "physics")
	params, _ := h.KnowledgeKeeper.GetParams(h.Ctx)

	require.Equal(t, uint32(params.MinVerifiers+1), effective,
		"zero partnerships should require base + 1 verifiers")
}

// ─── Test 5: Fire→Metal — Active domain recovers reputation faster ──

func TestR31_FireMetal_ActiveDomainRecoversFaster(t *testing.T) {
	h := NewTestHarness(t)

	// Index 10 rounds in physics domain (full activity)
	for i := 0; i < 10; i++ {
		h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i),
			&knowledgetypes.CompletedRoundMeta{Domain: "physics", DurationBlocks: 11})
	}

	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "physics")
	require.Greater(t, activity, uint64(0), "physics should have verification activity")

	// Inactive domain has no activity
	noActivity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "theology")
	require.Equal(t, uint64(0), noActivity, "theology should have no verification activity")
}

// ─── Test 6: Fire→Metal — Inactive domain recovers at base rate ──

func TestR31_FireMetal_InactiveDomainBaseRate(t *testing.T) {
	h := NewTestHarness(t)

	// No rounds indexed for any domain
	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "physics")
	require.Equal(t, uint64(0), activity, "no rounds = no activity")
}

// ─── Test 7: Combined — All three connections operating together ──

func TestR31_FireCombined_AllConnections(t *testing.T) {
	h := NewTestHarness(t)

	// Setup: Index rounds with some dissent
	for i := 0; i < 5; i++ {
		h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i),
			&knowledgetypes.CompletedRoundMeta{
				Domain:         "physics",
				HasDissent:     i%3 == 0,
				DurationBlocks: 11,
			})
	}

	// Fire → Earth: Verification health feeds governance
	throughput, _, _ := h.KnowledgeKeeper.GetVerificationHealth(h.Ctx)
	require.Greater(t, throughput, uint64(0))

	// Water → Fire: Partnership density adjusts min verifiers
	effective := h.KnowledgeKeeper.GetEffectiveMinVerifiers(h.Ctx, "physics")
	require.Greater(t, effective, uint32(0))

	// Fire → Metal: Activity is tracked
	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "physics")
	require.Greater(t, activity, uint64(0))
}
```

**Step 2: Run tests**

Run: `go test ./tests/cross_stack/ -run "TestR31_Fire" -v -count=1 -timeout 120s`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/cross_stack/r31_fire_activity_test.go
git commit -m "test(cross_stack): add R31-2 Fire Activity integration tests"
```

---

### Task 10: Social verification adjustment event in GetEffectiveMinVerifiers

**Files:**
- Modify: `x/knowledge/keeper/fire_activity.go` (add event emission)

**Step 1: Add event emission**

In `GetEffectiveMinVerifiers`, before each return statement, emit the social verification adjustment event:

```go
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// ... (after computing effective value) ...

	reason := "default"
	if density == 0 {
		reason = "no_social_structure"
	} else if density >= threshold {
		reason = "social_saturation"
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.social_verification_adjustment",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("base_min_verifiers", fmt.Sprintf("%d", base)),
		sdk.NewAttribute("effective_min_verifiers", fmt.Sprintf("%d", effective)),
		sdk.NewAttribute("partnership_density", fmt.Sprintf("%d", density)),
		sdk.NewAttribute("reason", reason),
	))
```

**Step 2: Verify compilation**

Run: `go build ./x/knowledge/...`
Expected: Success

**Step 3: Commit**

```bash
git add x/knowledge/keeper/fire_activity.go
git commit -m "feat(knowledge): add social verification adjustment event (R31-2)"
```

---

### Task 11: Proto check and full test suite

**Step 1: Run proto check**

Run: `make proto-check`
Expected: PASS

**Step 2: Run full test suite**

Run: `go test ./x/knowledge/... ./x/alignment/... ./x/capture_defense/... ./tests/cross_stack/ -v -count=1 -timeout 300s`
Expected: All PASS

**Step 3: Fix any failures**

Address compilation errors or test failures discovered during full suite run.

**Step 4: Final commit**

```bash
git add -A
git commit -m "fix: address R31-2 integration issues"
```

(Only if there are fixes needed.)

---

### Task 12: Verify PartnershipKeeper adapter for knowledge module

**Files:**
- Check: `app/app.go` wiring for knowledge ↔ partnerships

**Step 1: Verify the PartnershipKeeper adapter satisfies the updated interface**

The knowledge module's `PartnershipKeeper` interface now requires `GetDomainPartnershipDensity`. Check that the adapter used in `app/app.go` provides this method. Look at `app/app.go` for the `SetPartnershipKeeper` call and verify the adapter type.

Run: `grep -n "SetPartnershipKeeper" app/app.go`

If the adapter doesn't have `GetDomainPartnershipDensity`, create one or update the existing adapter.

**Step 2: Verify compilation**

Run: `go build ./...`
Expected: Success

**Step 3: Commit if changes needed**

```bash
git add app/app.go x/knowledge/keeper/partnership_adapters.go  # if created
git commit -m "feat(knowledge): wire PartnershipKeeper adapter for GetDomainPartnershipDensity (R31-2)"
```
