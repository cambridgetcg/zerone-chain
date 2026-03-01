# Consensus Diversity Scoring Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add per-round vote entropy, domain diversity aggregation, validator independence scores, and conformity alerts to the knowledge module, feeding into alignment's knowledge quality sensor.

**Architecture:** Diversity metrics are computed at round completion (entropy + independence), aggregated at fitness epoch boundaries (domain diversity + conformity alerts), and exposed to alignment via the existing KnowledgeKeeper interface. All entropy uses raw headcounts, not stake-weighted votes.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15 KV store, deterministic proto marshaling, fixed-point BPS arithmetic (0–1,000,000 scale).

---

### Task 1: Add Diversity Types

**Files:**
- Create: `x/knowledge/types/diversity.go`

**Step 1: Write the types file**

```go
package types

// RoundDiversity stores per-round vote entropy and raw headcounts.
type RoundDiversity struct {
	RoundID      string `json:"round_id"`
	Entropy      uint64 `json:"entropy"`       // BPS: 0 = unanimous, 1_000_000 = 50/50 split
	AcceptCount  uint64 `json:"accept_count"`  // raw headcount
	RejectCount  uint64 `json:"reject_count"`  // raw headcount
	TotalVoters  uint64 `json:"total_voters"`
	Domain       string `json:"domain"`
	Epoch        uint64 `json:"epoch"`
}

// DomainDiversityScore stores per-domain, per-epoch aggregated diversity.
type DomainDiversityScore struct {
	Domain         string `json:"domain"`
	Epoch          uint64 `json:"epoch"`
	AvgEntropy     uint64 `json:"avg_entropy"`     // BPS average across rounds
	RoundCount     uint64 `json:"round_count"`
	UnanimousCount uint64 `json:"unanimous_count"` // rounds with entropy = 0
}

// ValidatorIndependence tracks how often a validator dissents from the majority.
type ValidatorIndependence struct {
	Validator     string `json:"validator"`
	TotalVotes    uint64 `json:"total_votes"`
	MinorityVotes uint64 `json:"minority_votes"`
	LastEpoch     uint64 `json:"last_epoch"`
}

// ConformityStreak tracks consecutive low-diversity epochs for a domain.
type ConformityStreak struct {
	Domain            string `json:"domain"`
	ConsecutiveEpochs uint64 `json:"consecutive_epochs"`
	LastEpoch         uint64 `json:"last_epoch"`
}
```

**Step 2: Run build to verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/types/...`
Expected: PASS (no errors)

**Step 3: Commit**

```
git add x/knowledge/types/diversity.go
git commit -m "feat(knowledge): add diversity metric types (R28-2)"
```

---

### Task 2: Add KV Store Keys and Key Constructors

**Files:**
- Modify: `x/knowledge/types/keys.go:102-108` (add new prefixes after `QueryReceiptPrefix`)

**Step 1: Add key prefixes to `x/knowledge/types/keys.go`**

After line 107 (`QueryReceiptPrefix = []byte{0x3e}`), add:

```go
	// ─── Consensus diversity (R28-2) ────────────────────────────────────
	RoundDiversityPrefix         = []byte{0x40} // 0x40 | roundID → RoundDiversity (JSON)
	DomainDiversityPrefix        = []byte{0x41} // 0x41 | domain / epoch_bytes → DomainDiversityScore (JSON)
	ValidatorIndependencePrefix  = []byte{0x42} // 0x42 | validatorAddr → ValidatorIndependence (JSON)
	ConformityStreakPrefix       = []byte{0x43} // 0x43 | domain → ConformityStreak (JSON)
	DomainEpochRoundIndexPrefix = []byte{0x44} // 0x44 | domain / epoch_bytes / roundID → 0x01
```

**Step 2: Add key constructor functions**

At the bottom of `x/knowledge/types/keys.go`, add:

```go
// RoundDiversityKey returns the store key for a round's diversity data.
func RoundDiversityKey(roundID string) []byte {
	return append(append([]byte{}, RoundDiversityPrefix...), []byte(roundID)...)
}

// DomainDiversityKey returns the store key for a domain's epoch diversity score.
func DomainDiversityKey(domain string, epoch uint64) []byte {
	key := append(append([]byte{}, DomainDiversityPrefix...), []byte(domain)...)
	key = append(key, '/')
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epoch)
	return append(key, epochBz...)
}

// DomainDiversityByDomainPrefix returns the prefix for iterating all epochs for a domain.
func DomainDiversityByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, DomainDiversityPrefix...), []byte(domain)...)
	return append(key, '/')
}

// ValidatorIndependenceKey returns the store key for a validator's independence score.
func ValidatorIndependenceKey(validator string) []byte {
	return append(append([]byte{}, ValidatorIndependencePrefix...), []byte(validator)...)
}

// ConformityStreakKey returns the store key for a domain's conformity streak.
func ConformityStreakKey(domain string) []byte {
	return append(append([]byte{}, ConformityStreakPrefix...), []byte(domain)...)
}

// DomainEpochRoundKey returns the index key for a round in a domain's epoch.
func DomainEpochRoundKey(domain string, epoch uint64, roundID string) []byte {
	key := append(append([]byte{}, DomainEpochRoundIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epoch)
	key = append(key, epochBz...)
	key = append(key, '/')
	return append(key, []byte(roundID)...)
}

// DomainEpochRoundPrefix returns the prefix for iterating all rounds in a domain's epoch.
func DomainEpochRoundPrefix(domain string, epoch uint64) []byte {
	key := append(append([]byte{}, DomainEpochRoundIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epoch)
	key = append(key, epochBz...)
	return append(key, '/')
}
```

Note: You'll need to add `"encoding/binary"` to the imports in keys.go.

**Step 3: Run build to verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/types/...`
Expected: PASS

**Step 4: Commit**

```
git add x/knowledge/types/keys.go
git commit -m "feat(knowledge): add KV store keys for diversity metrics (R28-2)"
```

---

### Task 3: Add Diversity Params to Proto and Go Defaults

**Files:**
- Modify: `proto/zerone/knowledge/v1/genesis.proto:147-148` (add fields before closing brace)
- Regenerate: `x/knowledge/types/genesis.pb.go`
- Modify: `x/knowledge/types/genesis.go:144-147` (add defaults + validation)

**Step 1: Add proto fields**

In `proto/zerone/knowledge/v1/genesis.proto`, before the closing `}` of the Params message (line 148), add:

```protobuf
  // ─── Consensus diversity (R28-2) ──────────────────────────────────
  uint64 diversity_conformity_alert_threshold = 101; // BPS entropy below which a domain is "conforming" (default: 50,000 = 5%)
  uint64 diversity_conformity_alert_epochs    = 102; // Consecutive low-diversity epochs before alert (default: 3)
```

**Step 2: Regenerate protobuf Go code**

Run: `cd /Users/yournameisai/Desktop/zerone && make proto-gen`
Expected: genesis.pb.go updated with new fields

**Step 3: Add defaults in genesis.go**

In `x/knowledge/types/genesis.go`, inside `DefaultParams()`, add after `SatisfactionMinRatings: 3,` (line 145):

```go
		// ─── Consensus diversity (R28-2) ─────────────────────────────────
		DiversityConformityAlertThreshold: 50_000, // 5% entropy — catches pure unanimity on small validator sets
		DiversityConformityAlertEpochs:    3,      // 3 consecutive low-diversity epochs before alert
```

**Step 4: Add validation**

In `x/knowledge/types/genesis.go`, inside `Validate()`, before the final `return nil` (line 449), add:

```go
	// ─── Diversity params ──────────────────────────────────────────────
	if p.DiversityConformityAlertThreshold > 1_000_000 {
		return fmt.Errorf("diversity_conformity_alert_threshold must be <= 1,000,000")
	}
	if p.DiversityConformityAlertEpochs == 0 {
		return fmt.Errorf("diversity_conformity_alert_epochs must be > 0")
	}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/types/... -v -count=1`
Expected: PASS

**Step 6: Commit**

```
git add proto/zerone/knowledge/v1/genesis.proto x/knowledge/types/genesis.pb.go x/knowledge/types/genesis.go
git commit -m "feat(knowledge): add diversity params to proto and defaults (R28-2)"
```

---

### Task 4: Write Diversity Tests (TDD — Write Tests First)

**Files:**
- Create: `x/knowledge/keeper/diversity_test.go`

**Step 1: Write the full test file**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
)

// ─── Fixed-Point Entropy Tests ──────────────────────────────────────────────

func TestComputeRoundEntropy_Unanimous(t *testing.T) {
	// All accept, no reject → entropy = 0
	entropy := keeper.ComputeRoundEntropy(5, 0)
	require.Equal(t, uint64(0), entropy, "unanimous accept should have 0 entropy")

	// All reject, no accept → entropy = 0
	entropy = keeper.ComputeRoundEntropy(0, 5)
	require.Equal(t, uint64(0), entropy, "unanimous reject should have 0 entropy")
}

func TestComputeRoundEntropy_PerfectSplit(t *testing.T) {
	// 50/50 split → entropy = BPS (maximum = 1,000,000)
	entropy := keeper.ComputeRoundEntropy(5, 5)
	require.Equal(t, uint64(1_000_000), entropy, "50/50 split should give maximum entropy")
}

func TestComputeRoundEntropy_EightyTwenty(t *testing.T) {
	// 80/20 → entropy ~= 721,928 BPS (H(0.2) = 0.7219)
	entropy := keeper.ComputeRoundEntropy(8, 2)
	require.Greater(t, entropy, uint64(0), "80/20 should have non-zero entropy")
	require.Less(t, entropy, uint64(1_000_000), "80/20 should be less than max entropy")
	// Allow 5% tolerance for lookup table approximation
	require.InDelta(t, 721_928, float64(entropy), 50_000, "80/20 entropy should be approximately 722K BPS")
}

func TestComputeRoundEntropy_ZeroVoters(t *testing.T) {
	entropy := keeper.ComputeRoundEntropy(0, 0)
	require.Equal(t, uint64(0), entropy, "zero voters should have 0 entropy")
}

func TestComputeRoundEntropy_SingleVoter(t *testing.T) {
	entropy := keeper.ComputeRoundEntropy(1, 0)
	require.Equal(t, uint64(0), entropy, "single voter should have 0 entropy (unanimous)")
}

func TestComputeRoundEntropy_ThreeOneSmallSet(t *testing.T) {
	// 3-1 split with 4 validators → p=0.25 → H ≈ 811,278 BPS
	entropy := keeper.ComputeRoundEntropy(3, 1)
	require.Greater(t, entropy, uint64(0))
	require.Less(t, entropy, uint64(1_000_000))
	require.InDelta(t, 811_278, float64(entropy), 50_000, "3-1 split entropy should be approximately 811K BPS")
}

// ─── Diversity Store Tests ──────────────────────────────────────────────────

func TestSetGetRoundDiversity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	rd := &keeper.RoundDiversityRecord{
		RoundID:     "round123",
		Entropy:     500_000,
		AcceptCount: 3,
		RejectCount: 2,
		TotalVoters: 5,
		Domain:      "mathematics",
		Epoch:       10,
	}

	err := k.SetRoundDiversity(ctx, rd)
	require.NoError(t, err)

	got, found := k.GetRoundDiversity(ctx, "round123")
	require.True(t, found)
	require.Equal(t, rd.Entropy, got.Entropy)
	require.Equal(t, rd.AcceptCount, got.AcceptCount)
	require.Equal(t, rd.Domain, got.Domain)
}

func TestSetGetDomainDiversity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	ds := &keeper.DomainDiversityRecord{
		Domain:         "physics",
		Epoch:          5,
		AvgEntropy:     400_000,
		RoundCount:     10,
		UnanimousCount: 3,
	}

	err := k.SetDomainDiversity(ctx, ds)
	require.NoError(t, err)

	got, found := k.GetDomainDiversity(ctx, "physics", 5)
	require.True(t, found)
	require.Equal(t, ds.AvgEntropy, got.AvgEntropy)
	require.Equal(t, ds.RoundCount, got.RoundCount)
}

func TestSetGetValidatorIndependence(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	vi := &keeper.ValidatorIndependenceRecord{
		Validator:     "zrn1validator1",
		TotalVotes:    20,
		MinorityVotes: 3,
		LastEpoch:     5,
	}

	err := k.SetValidatorIndependence(ctx, vi)
	require.NoError(t, err)

	got, found := k.GetValidatorIndependence(ctx, "zrn1validator1")
	require.True(t, found)
	require.Equal(t, vi.TotalVotes, got.TotalVotes)
	require.Equal(t, vi.MinorityVotes, got.MinorityVotes)
}

func TestSetGetConformityStreak(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	cs := &keeper.ConformityStreakRecord{
		Domain:            "general",
		ConsecutiveEpochs: 2,
		LastEpoch:         5,
	}

	err := k.SetConformityStreak(ctx, cs)
	require.NoError(t, err)

	got, found := k.GetConformityStreak(ctx, "general")
	require.True(t, found)
	require.Equal(t, cs.ConsecutiveEpochs, got.ConsecutiveEpochs)
}

// ─── Integration: RecordRoundDiversity ──────────────────────────────────────

func TestRecordRoundDiversity_Unanimous(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set block height to be in epoch 1 (with FitnessEpochBlocks=10000)
	ctx = ctx.WithBlockHeight(10100)

	k.RecordRoundDiversity(ctx, "round-u", "mathematics", 5, 0)

	rd, found := k.GetRoundDiversity(ctx, "round-u")
	require.True(t, found)
	require.Equal(t, uint64(0), rd.Entropy, "unanimous round should have 0 entropy")
	require.Equal(t, uint64(5), rd.AcceptCount)
	require.Equal(t, uint64(0), rd.RejectCount)
}

func TestRecordRoundDiversity_Split(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeight(10100)

	k.RecordRoundDiversity(ctx, "round-s", "physics", 3, 2)

	rd, found := k.GetRoundDiversity(ctx, "round-s")
	require.True(t, found)
	require.Greater(t, rd.Entropy, uint64(0), "split round should have non-zero entropy")
}

// ─── Integration: UpdateValidatorIndependence ───────────────────────────────

func TestUpdateValidatorIndependence_MajorityVoter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Validator votes with majority (accept, majority is accept)
	k.UpdateValidatorIndependence(ctx, "zrn1val1", "accept", "accept")

	vi, found := k.GetValidatorIndependence(ctx, "zrn1val1")
	require.True(t, found)
	require.Equal(t, uint64(1), vi.TotalVotes)
	require.Equal(t, uint64(0), vi.MinorityVotes, "majority voter should have 0 minority votes")
}

func TestUpdateValidatorIndependence_MinorityVoter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Validator votes against majority (reject, majority is accept)
	k.UpdateValidatorIndependence(ctx, "zrn1val1", "reject", "accept")

	vi, found := k.GetValidatorIndependence(ctx, "zrn1val1")
	require.True(t, found)
	require.Equal(t, uint64(1), vi.TotalVotes)
	require.Equal(t, uint64(1), vi.MinorityVotes, "minority voter should have 1 minority vote")
}

func TestUpdateValidatorIndependence_HealthyDissenter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 20 votes, 3 in minority → 15% independence
	for i := 0; i < 17; i++ {
		k.UpdateValidatorIndependence(ctx, "zrn1val1", "accept", "accept")
	}
	for i := 0; i < 3; i++ {
		k.UpdateValidatorIndependence(ctx, "zrn1val1", "reject", "accept")
	}

	vi, found := k.GetValidatorIndependence(ctx, "zrn1val1")
	require.True(t, found)
	require.Equal(t, uint64(20), vi.TotalVotes)
	require.Equal(t, uint64(3), vi.MinorityVotes)
	// Independence = 3 * 1_000_000 / 20 = 150_000 BPS (15%)
	independence := vi.MinorityVotes * 1_000_000 / vi.TotalVotes
	require.Equal(t, uint64(150_000), independence)
}

// ─── Integration: AggregateDomainDiversity ──────────────────────────────────

func TestAggregateDomainDiversity_AllUnanimous(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeight(10100)

	// Record 3 unanimous rounds in the same domain+epoch
	k.RecordRoundDiversity(ctx, "r1", "mathematics", 4, 0)
	k.RecordRoundDiversity(ctx, "r2", "mathematics", 5, 0)
	k.RecordRoundDiversity(ctx, "r3", "mathematics", 3, 0)

	epoch := uint64(1) // height 10100 / FitnessEpochBlocks 10000
	err := k.AggregateDomainDiversity(ctx, "mathematics", epoch)
	require.NoError(t, err)

	ds, found := k.GetDomainDiversity(ctx, "mathematics", epoch)
	require.True(t, found)
	require.Equal(t, uint64(0), ds.AvgEntropy, "all unanimous → avg entropy 0")
	require.Equal(t, uint64(3), ds.RoundCount)
	require.Equal(t, uint64(3), ds.UnanimousCount)
}

func TestAggregateDomainDiversity_Mixed(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeight(10100)

	// 1 unanimous + 1 split
	k.RecordRoundDiversity(ctx, "r1", "physics", 5, 0) // entropy = 0
	k.RecordRoundDiversity(ctx, "r2", "physics", 3, 2) // entropy > 0

	epoch := uint64(1)
	err := k.AggregateDomainDiversity(ctx, "physics", epoch)
	require.NoError(t, err)

	ds, found := k.GetDomainDiversity(ctx, "physics", epoch)
	require.True(t, found)
	require.Greater(t, ds.AvgEntropy, uint64(0), "mixed rounds should have non-zero avg entropy")
	require.Equal(t, uint64(2), ds.RoundCount)
	require.Equal(t, uint64(1), ds.UnanimousCount)
}

// ─── Integration: ConformityAlerts ──────────────────────────────────────────

func TestConformityStreak_IncrementAndReset(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 3 consecutive low-diversity epochs
	k.IncrementConformityStreak(ctx, "mathematics", 1)
	k.IncrementConformityStreak(ctx, "mathematics", 2)
	k.IncrementConformityStreak(ctx, "mathematics", 3)

	cs, found := k.GetConformityStreak(ctx, "mathematics")
	require.True(t, found)
	require.Equal(t, uint64(3), cs.ConsecutiveEpochs)

	// Reset on healthy diversity
	k.ResetConformityStreak(ctx, "mathematics")

	cs, found = k.GetConformityStreak(ctx, "mathematics")
	require.True(t, found)
	require.Equal(t, uint64(0), cs.ConsecutiveEpochs)
}

// ─── Alignment Integration ──────────────────────────────────────────────────

func TestGetConsensusDiversity_NoDomains(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	adapter := keeper.NewAlignmentKnowledgeAdapter(k)
	diversity := adapter.GetConsensusDiversity(ctx)
	// No domain diversity data → return NeutralBPS (500,000)
	require.Equal(t, uint64(500_000), diversity)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run TestComputeRoundEntropy -v -count=1 2>&1 | head -20`
Expected: FAIL — `ComputeRoundEntropy` not defined

**Step 3: Commit test file**

```
git add x/knowledge/keeper/diversity_test.go
git commit -m "test(knowledge): add failing diversity tests — TDD red phase (R28-2)"
```

---

### Task 5: Implement Core Diversity Logic

**Files:**
- Create: `x/knowledge/keeper/diversity.go`

**Step 1: Write the diversity keeper methods**

```go
package keeper

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Fixed-Point Entropy ─────────────────────────────────────────────────────

// BPS is the basis points scale (1,000,000 = 100%).
const BPS = 1_000_000

// fixedLog2BPS is a lookup table for -log2(p) * BPS where p is in BPS.
// Maps input BPS → output (-log2(input/BPS)) * BPS.
// Precomputed for determinism. 20 points, linearly interpolated.
var fixedLog2Table = []struct {
	pBPS    uint64 // input probability in BPS
	negLogBPS uint64 // -log2(p) * BPS
}{
	{50_000, 4_321_928},  // p=0.05, -log2 = 4.3219
	{100_000, 3_321_928}, // p=0.10, -log2 = 3.3219
	{150_000, 2_736_966}, // p=0.15
	{200_000, 2_321_928}, // p=0.20
	{250_000, 2_000_000}, // p=0.25
	{300_000, 1_736_966}, // p=0.30
	{350_000, 1_514_573}, // p=0.35
	{400_000, 1_321_928}, // p=0.40
	{450_000, 1_152_003}, // p=0.45
	{500_000, 1_000_000}, // p=0.50, -log2 = 1.0
	{550_000, 862_304},   // p=0.55
	{600_000, 736_966},   // p=0.60
	{650_000, 621_488},   // p=0.65
	{700_000, 514_573},   // p=0.70
	{750_000, 415_037},   // p=0.75
	{800_000, 321_928},   // p=0.80
	{850_000, 234_465},   // p=0.85
	{900_000, 152_003},   // p=0.90
	{950_000, 73_697},    // p=0.95
	{1_000_000, 0},       // p=1.00, -log2 = 0
}

// fixedNegLog2BPS returns -log2(pBPS/BPS) * BPS using linear interpolation.
// pBPS must be in range (0, BPS]. Returns 0 for pBPS=0.
func fixedNegLog2BPS(pBPS uint64) uint64 {
	if pBPS == 0 {
		return 0 // convention: 0 * log(0) = 0 in entropy
	}
	if pBPS >= BPS {
		return 0
	}
	// Clamp to table range
	if pBPS < fixedLog2Table[0].pBPS {
		return fixedLog2Table[0].negLogBPS // extrapolate floor
	}

	// Binary search for bracket
	for i := 0; i < len(fixedLog2Table)-1; i++ {
		lo := fixedLog2Table[i]
		hi := fixedLog2Table[i+1]
		if pBPS >= lo.pBPS && pBPS <= hi.pBPS {
			// Linear interpolation
			if hi.pBPS == lo.pBPS {
				return lo.negLogBPS
			}
			// Interpolate: result = lo.negLog + (hi.negLog - lo.negLog) * (pBPS - lo.pBPS) / (hi.pBPS - lo.pBPS)
			// Note: negLog decreases as p increases, so hi.negLog < lo.negLog
			diff := lo.negLogBPS - hi.negLogBPS
			frac := (pBPS - lo.pBPS) * BPS / (hi.pBPS - lo.pBPS)
			return lo.negLogBPS - diff*frac/BPS
		}
	}
	return 0
}

// ComputeRoundEntropy computes Shannon entropy for binary votes in BPS.
// Uses raw headcounts (1 validator = 1 signal).
// Returns 0 for unanimous, BPS for 50/50, values between for partial splits.
func ComputeRoundEntropy(acceptCount, rejectCount uint64) uint64 {
	total := acceptCount + rejectCount
	if total == 0 || acceptCount == 0 || rejectCount == 0 {
		return 0 // unanimous or empty
	}

	pAccept := acceptCount * BPS / total
	pReject := rejectCount * BPS / total

	// H = -Σ p_i * log2(p_i) = pAccept * (-log2(pAccept)) + pReject * (-log2(pReject))
	// All in BPS; need to divide by BPS once since both p and -log2 are in BPS
	entropy := (pAccept*fixedNegLog2BPS(pAccept) + pReject*fixedNegLog2BPS(pReject)) / BPS

	// Cap at BPS
	if entropy > BPS {
		return BPS
	}
	return entropy
}

// ─── Record Types (keeper-local, JSON-encoded for KV store) ─────────────────

// RoundDiversityRecord is the store representation of per-round diversity.
type RoundDiversityRecord struct {
	RoundID     string `json:"round_id"`
	Entropy     uint64 `json:"entropy"`
	AcceptCount uint64 `json:"accept_count"`
	RejectCount uint64 `json:"reject_count"`
	TotalVoters uint64 `json:"total_voters"`
	Domain      string `json:"domain"`
	Epoch       uint64 `json:"epoch"`
}

// DomainDiversityRecord is the store representation of per-domain epoch diversity.
type DomainDiversityRecord struct {
	Domain         string `json:"domain"`
	Epoch          uint64 `json:"epoch"`
	AvgEntropy     uint64 `json:"avg_entropy"`
	RoundCount     uint64 `json:"round_count"`
	UnanimousCount uint64 `json:"unanimous_count"`
}

// ValidatorIndependenceRecord is the store representation of per-validator independence.
type ValidatorIndependenceRecord struct {
	Validator     string `json:"validator"`
	TotalVotes    uint64 `json:"total_votes"`
	MinorityVotes uint64 `json:"minority_votes"`
	LastEpoch     uint64 `json:"last_epoch"`
}

// ConformityStreakRecord is the store representation of per-domain conformity streak.
type ConformityStreakRecord struct {
	Domain            string `json:"domain"`
	ConsecutiveEpochs uint64 `json:"consecutive_epochs"`
	LastEpoch         uint64 `json:"last_epoch"`
}

// ─── Store Methods ───────────────────────────────────────────────────────────

func (k Keeper) SetRoundDiversity(ctx context.Context, rd *RoundDiversityRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(rd)
	if err != nil {
		return fmt.Errorf("failed to marshal round diversity: %w", err)
	}
	return store.Set(types.RoundDiversityKey(rd.RoundID), bz)
}

func (k Keeper) GetRoundDiversity(ctx context.Context, roundID string) (*RoundDiversityRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.RoundDiversityKey(roundID))
	if err != nil || bz == nil {
		return nil, false
	}
	var rd RoundDiversityRecord
	if err := json.Unmarshal(bz, &rd); err != nil {
		return nil, false
	}
	return &rd, true
}

func (k Keeper) SetDomainDiversity(ctx context.Context, ds *DomainDiversityRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(ds)
	if err != nil {
		return fmt.Errorf("failed to marshal domain diversity: %w", err)
	}
	return store.Set(types.DomainDiversityKey(ds.Domain, ds.Epoch), bz)
}

func (k Keeper) GetDomainDiversity(ctx context.Context, domain string, epoch uint64) (*DomainDiversityRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DomainDiversityKey(domain, epoch))
	if err != nil || bz == nil {
		return nil, false
	}
	var ds DomainDiversityRecord
	if err := json.Unmarshal(bz, &ds); err != nil {
		return nil, false
	}
	return &ds, true
}

func (k Keeper) SetValidatorIndependence(ctx context.Context, vi *ValidatorIndependenceRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(vi)
	if err != nil {
		return fmt.Errorf("failed to marshal validator independence: %w", err)
	}
	return store.Set(types.ValidatorIndependenceKey(vi.Validator), bz)
}

func (k Keeper) GetValidatorIndependence(ctx context.Context, validator string) (*ValidatorIndependenceRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ValidatorIndependenceKey(validator))
	if err != nil || bz == nil {
		return nil, false
	}
	var vi ValidatorIndependenceRecord
	if err := json.Unmarshal(bz, &vi); err != nil {
		return nil, false
	}
	return &vi, true
}

func (k Keeper) SetConformityStreak(ctx context.Context, cs *ConformityStreakRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(cs)
	if err != nil {
		return fmt.Errorf("failed to marshal conformity streak: %w", err)
	}
	return store.Set(types.ConformityStreakKey(cs.Domain), bz)
}

func (k Keeper) GetConformityStreak(ctx context.Context, domain string) (*ConformityStreakRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ConformityStreakKey(domain))
	if err != nil || bz == nil {
		return nil, false
	}
	var cs ConformityStreakRecord
	if err := json.Unmarshal(bz, &cs); err != nil {
		return nil, false
	}
	return &cs, true
}

// ─── Domain Epoch Round Index ────────────────────────────────────────────────

func (k Keeper) indexRoundInDomainEpoch(ctx context.Context, domain string, epoch uint64, roundID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.DomainEpochRoundKey(domain, epoch, roundID), []byte{0x01})
}

// ─── High-Level Operations ───────────────────────────────────────────────────

// RecordRoundDiversity computes and stores the entropy for a completed round.
// Called from CompleteRound. Uses raw headcounts, not stake-weighted.
func (k Keeper) RecordRoundDiversity(ctx context.Context, roundID, domain string, acceptCount, rejectCount uint64) {
	entropy := ComputeRoundEntropy(acceptCount, rejectCount)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	params, _ := k.GetParams(ctx)
	epoch := uint64(0)
	if params.FitnessEpochBlocks > 0 {
		epoch = height / params.FitnessEpochBlocks
	}

	rd := &RoundDiversityRecord{
		RoundID:     roundID,
		Entropy:     entropy,
		AcceptCount: acceptCount,
		RejectCount: rejectCount,
		TotalVoters: acceptCount + rejectCount,
		Domain:      domain,
		Epoch:       epoch,
	}
	_ = k.SetRoundDiversity(ctx, rd)
	_ = k.indexRoundInDomainEpoch(ctx, domain, epoch, roundID)
}

// UpdateValidatorIndependence updates a validator's independence counters.
// Called from CompleteRound for each voter.
// majorityVote is the winning vote string (the verdict mapped to vote string).
func (k Keeper) UpdateValidatorIndependence(ctx context.Context, validator, vote, majorityVote string) {
	vi, found := k.GetValidatorIndependence(ctx, validator)
	if !found {
		vi = &ValidatorIndependenceRecord{Validator: validator}
	}
	vi.TotalVotes++
	if vote != majorityVote {
		vi.MinorityVotes++
	}
	_ = k.SetValidatorIndependence(ctx, vi)
}

// AggregateDomainDiversity aggregates round entropies for a domain over an epoch.
// Called from BeginBlocker at fitness epoch boundaries.
func (k Keeper) AggregateDomainDiversity(ctx context.Context, domain string, epoch uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	pfx := types.DomainEpochRoundPrefix(domain, epoch)
	iter, err := store.Iterator(pfx, prefixEndBytes(pfx))
	if err != nil {
		return err
	}
	defer iter.Close()

	var totalEntropy, roundCount, unanimousCount uint64
	for ; iter.Valid(); iter.Next() {
		roundID := string(iter.Key()[len(pfx):])
		rd, found := k.GetRoundDiversity(ctx, roundID)
		if !found {
			continue
		}
		totalEntropy += rd.Entropy
		roundCount++
		if rd.Entropy == 0 {
			unanimousCount++
		}
	}

	if roundCount == 0 {
		return nil // no rounds this epoch
	}

	avgEntropy := totalEntropy / roundCount

	ds := &DomainDiversityRecord{
		Domain:         domain,
		Epoch:          epoch,
		AvgEntropy:     avgEntropy,
		RoundCount:     roundCount,
		UnanimousCount: unanimousCount,
	}
	return k.SetDomainDiversity(ctx, ds)
}

// IncrementConformityStreak increments a domain's conformity streak counter.
func (k Keeper) IncrementConformityStreak(ctx context.Context, domain string, epoch uint64) {
	cs, found := k.GetConformityStreak(ctx, domain)
	if !found {
		cs = &ConformityStreakRecord{Domain: domain}
	}
	cs.ConsecutiveEpochs++
	cs.LastEpoch = epoch
	_ = k.SetConformityStreak(ctx, cs)
}

// ResetConformityStreak resets a domain's conformity streak to 0.
func (k Keeper) ResetConformityStreak(ctx context.Context, domain string) {
	cs := &ConformityStreakRecord{Domain: domain, ConsecutiveEpochs: 0}
	_ = k.SetConformityStreak(ctx, cs)
}

// CheckConformityAlert checks if a domain should emit a conformity alert
// and emits the event if the streak threshold is met.
func (k Keeper) CheckConformityAlert(ctx context.Context, domain string, avgEntropy, epoch uint64) {
	params, _ := k.GetParams(ctx)
	threshold := params.DiversityConformityAlertThreshold
	alertEpochs := params.DiversityConformityAlertEpochs

	if avgEntropy < threshold {
		k.IncrementConformityStreak(ctx, domain, epoch)
		cs, found := k.GetConformityStreak(ctx, domain)
		if found && cs.ConsecutiveEpochs >= alertEpochs {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"zerone.knowledge.conformity_alert",
				sdk.NewAttribute("domain", domain),
				sdk.NewAttribute("avg_entropy", fmt.Sprintf("%d", avgEntropy)),
				sdk.NewAttribute("consecutive_epochs", fmt.Sprintf("%d", cs.ConsecutiveEpochs)),
				sdk.NewAttribute("epoch", fmt.Sprintf("%d", epoch)),
			))
		}
	} else {
		k.ResetConformityStreak(ctx, domain)
	}
}

// ProcessDiversity runs per-epoch diversity aggregation across all active domains.
// Called from BeginBlocker at fitness epoch boundaries.
func (k Keeper) ProcessDiversity(ctx context.Context, epoch uint64) {
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		if err := k.AggregateDomainDiversity(ctx, domain.Name, epoch); err != nil {
			k.Logger(ctx).Error("diversity aggregation failed", "domain", domain.Name, "error", err)
			return false
		}
		ds, found := k.GetDomainDiversity(ctx, domain.Name, epoch)
		if found {
			k.CheckConformityAlert(ctx, domain.Name, ds.AvgEntropy, epoch)
		}
		return false
	})
}

// GetGlobalConsensusDiversity returns the average diversity across all domains
// for the most recent epoch that has data. Returns 500_000 (NeutralBPS) if no data.
func (k Keeper) GetGlobalConsensusDiversity(ctx context.Context) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	params, _ := k.GetParams(ctx)
	if params.FitnessEpochBlocks == 0 || height == 0 {
		return 500_000
	}

	epoch := height / params.FitnessEpochBlocks
	// Try current epoch and previous epoch
	for try := uint64(0); try < 2; try++ {
		if epoch < try {
			break
		}
		checkEpoch := epoch - try

		var totalEntropy, domainCount uint64
		k.IterateDomains(ctx, func(domain *types.Domain) bool {
			ds, found := k.GetDomainDiversity(ctx, domain.Name, checkEpoch)
			if found && ds.RoundCount > 0 {
				totalEntropy += ds.AvgEntropy
				domainCount++
			}
			return false
		})

		if domainCount > 0 {
			return totalEntropy / domainCount
		}
	}

	return 500_000 // NeutralBPS — no data yet
}
```

Note: `json.Marshal` is deterministic for these simple structs (no maps, no floating point) so it's safe for consensus. An alternative would be manual binary encoding, but JSON is simpler and follows the pattern used by other non-proto state in the codebase. The key guarantee is that identical inputs produce identical bytes, which `encoding/json` provides for structs with fixed-order fields.

**Step 2: Run tests to verify they pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run TestComputeRoundEntropy -v -count=1`
Expected: PASS (all entropy tests)

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestSetGet|TestRecord|TestUpdate|TestAggregate|TestConformity|TestGetConsensus" -v -count=1`
Expected: PASS (all store + integration tests)

**Step 3: Commit**

```
git add x/knowledge/keeper/diversity.go
git commit -m "feat(knowledge): implement diversity computation, store, and aggregation (R28-2)"
```

---

### Task 6: Wire Diversity into Round Completion

**Files:**
- Modify: `x/knowledge/keeper/rounds.go:57-134` (CompleteRound)
- Modify: `x/knowledge/keeper/confidence.go:31-115` (AggregateVerificationResult — need raw counts)

**Step 1: Extract raw vote counts in AggregateVerificationResult**

In `x/knowledge/keeper/confidence.go`, add raw headcount fields to `VerificationResult`:

```go
// Add to VerificationResult struct (after Slashes field):
	AcceptCount uint64 // raw headcount (not stake-weighted)
	RejectCount uint64 // raw headcount (not stake-weighted)
```

In `AggregateVerificationResult`, after the vote tallying loop (line 62), add raw headcount tracking:

```go
	// Count raw headcounts for diversity (1 validator = 1 signal)
	var rawAccept, rawReject uint64
	for _, reveal := range round.Reveals {
		switch reveal.Vote {
		case "accept":
			rawAccept++
		case "reject":
			rawReject++
		}
	}
```

Then set them on the result before returning:

```go
	result.AcceptCount = rawAccept
	result.RejectCount = rawReject
```

**Step 2: Hook diversity recording into CompleteRound**

In `x/knowledge/keeper/rounds.go`, in `CompleteRound`, after the domain qualification recording block (after line 124) and before the event emission (line 126), add:

```go
	// Record round diversity metrics (R28-2)
	k.RecordRoundDiversity(ctx, round.Id, claim.Domain, result.AcceptCount, result.RejectCount)

	// Update validator independence for each revealed voter
	majorityVote := verdictToVoteString(result.Verdict)
	for _, reveal := range round.Reveals {
		k.UpdateValidatorIndependence(ctx, reveal.Verifier, reveal.Vote, majorityVote)
	}
```

Also add a helper function (can go in rounds.go or diversity.go):

```go
// verdictToVoteString maps a verdict enum to the corresponding vote string.
func verdictToVoteString(v types.Verdict) string {
	switch v {
	case types.Verdict_VERDICT_ACCEPT:
		return "accept"
	case types.Verdict_VERDICT_REJECT:
		return "reject"
	case types.Verdict_VERDICT_MALFORMED:
		return "malformed"
	default:
		return ""
	}
}
```

**Step 3: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -v -count=1 -timeout 120s`
Expected: PASS (all existing tests still pass, diversity tests pass)

**Step 4: Commit**

```
git add x/knowledge/keeper/rounds.go x/knowledge/keeper/confidence.go x/knowledge/keeper/diversity.go
git commit -m "feat(knowledge): wire diversity recording into round completion (R28-2)"
```

---

### Task 7: Wire Diversity Aggregation into BeginBlocker

**Files:**
- Modify: `x/knowledge/keeper/phases.go:26-52` (BeginBlocker epoch boundary)

**Step 1: Add diversity processing to epoch boundary**

In `x/knowledge/keeper/phases.go`, inside the `BeginBlocker` epoch boundary block (after line 51, the `k.ClearQueryReceipts(ctx)` call), add:

```go
		// 8. Aggregate diversity metrics and check conformity alerts (R28-2)
		k.ProcessDiversity(ctx, epoch)
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -v -count=1 -timeout 120s`
Expected: PASS

**Step 3: Commit**

```
git add x/knowledge/keeper/phases.go
git commit -m "feat(knowledge): wire diversity aggregation into BeginBlocker epoch boundary (R28-2)"
```

---

### Task 8: Wire Alignment Integration

**Files:**
- Modify: `x/alignment/types/expected_keepers.go:8-14` (add GetConsensusDiversity)
- Modify: `x/knowledge/keeper/alignment_adapters.go` (implement GetConsensusDiversity)
- Modify: `x/alignment/keeper/sensors.go:28-39` (update senseKnowledgeQuality)

**Step 1: Add method to alignment's KnowledgeKeeper interface**

In `x/alignment/types/expected_keepers.go`, add to the `KnowledgeKeeper` interface:

```go
	// GetConsensusDiversity returns the global consensus diversity score in BPS.
	GetConsensusDiversity(ctx context.Context) uint64
```

**Step 2: Implement in alignment adapter**

In `x/knowledge/keeper/alignment_adapters.go`, add:

```go
// GetConsensusDiversity returns the global consensus diversity score in BPS.
func (a *AlignmentKnowledgeAdapter) GetConsensusDiversity(ctx context.Context) uint64 {
	return a.k.GetGlobalConsensusDiversity(ctx)
}
```

**Step 3: Update senseKnowledgeQuality**

In `x/alignment/keeper/sensors.go`, replace `senseKnowledgeQuality` (lines 28-39):

```go
// senseKnowledgeQuality reads verification rate and consensus diversity from x/knowledge.
// Weighted: 60% verification rate, 40% diversity.
// A system that verifies everything unanimously scores LOWER on knowledge quality.
// Returns BPS. Nil-safe: returns NeutralBPS if keeper is nil.
func (k Keeper) senseKnowledgeQuality(ctx context.Context) uint64 {
	if k.knowledgeKeeper == nil {
		return types.NeutralBPS
	}
	rate := k.knowledgeKeeper.GetVerificationRate(ctx)
	if rate > types.BPS {
		rate = types.BPS
	}
	diversity := k.knowledgeKeeper.GetConsensusDiversity(ctx)
	if diversity > types.BPS {
		diversity = types.BPS
	}
	// Weighted: 60% verification rate, 40% diversity
	return (rate*6 + diversity*4) / 10
}
```

**Step 4: Run build and tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: PASS (compile-time interface check will catch mismatches)

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/... -v -count=1 -timeout 60s`
Expected: PASS

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run TestGetConsensusDiversity -v -count=1`
Expected: PASS

**Step 5: Commit**

```
git add x/alignment/types/expected_keepers.go x/knowledge/keeper/alignment_adapters.go x/alignment/keeper/sensors.go
git commit -m "feat(alignment): incorporate consensus diversity into knowledge quality sensor (R28-2)"
```

---

### Task 9: Add CLI Queries

**Files:**
- Modify: `x/knowledge/client/cli/query.go:48-79` (add 4 new commands to GetQueryCmd)

**Step 1: Add query commands to the registry**

In `x/knowledge/client/cli/query.go`, inside `GetQueryCmd()`, add to the `AddCommand` block (before the closing `)`):

```go
		NewQueryDomainDiversityCmd(),
		NewQueryDomainDiversityHistoryCmd(),
		NewQueryValidatorIndependenceCmd(),
		NewQueryConformityAlertsCmd(),
```

**Step 2: Implement the 4 query command functions**

Add after the last function in query.go:

```go
// NewQueryDomainDiversityCmd queries the current epoch diversity for a domain.
func NewQueryDomainDiversityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain-diversity [domain]",
		Short: "Query consensus diversity for a domain (current epoch)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryDomainDiversityRequest{Domain: args[0]}
			resp := &types.QueryDomainDiversityResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DomainDiversity", req, resp); err != nil {
				return fmt.Errorf("failed to query domain diversity: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryDomainDiversityHistoryCmd queries historical diversity for a domain.
func NewQueryDomainDiversityHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain-diversity-history [domain]",
		Short: "Query historical diversity for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			epochs, _ := cmd.Flags().GetUint64("epochs")
			req := &types.QueryDomainDiversityHistoryRequest{Domain: args[0], Epochs: epochs}
			resp := &types.QueryDomainDiversityHistoryResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/DomainDiversityHistory", req, resp); err != nil {
				return fmt.Errorf("failed to query domain diversity history: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint64("epochs", 10, "Number of epochs to look back")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryValidatorIndependenceCmd queries a validator's independence score.
func NewQueryValidatorIndependenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validator-independence [validator-addr]",
		Short: "Query a validator's independence score (how often they dissent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryValidatorIndependenceRequest{Validator: args[0]}
			resp := &types.QueryValidatorIndependenceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ValidatorIndependence", req, resp); err != nil {
				return fmt.Errorf("failed to query validator independence: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQueryConformityAlertsCmd queries active conformity alerts across domains.
func NewQueryConformityAlertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conformity-alerts",
		Short: "Query domains with active conformity alerts (sustained low diversity)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryConformityAlertsRequest{}
			resp := &types.QueryConformityAlertsResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/ConformityAlerts", req, resp); err != nil {
				return fmt.Errorf("failed to query conformity alerts: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 3: Add proto query messages and gRPC handler**

You'll need to add the query request/response types to the proto file and register the gRPC handlers. The types needed:

In `proto/zerone/knowledge/v1/query.proto`, add the 4 new RPC methods + message types. Then regenerate with `make proto-gen`.

In the knowledge gRPC query server (`x/knowledge/keeper/grpc_query.go` or similar), add handler implementations that call the keeper's diversity methods.

**Step 4: Run build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: PASS

**Step 5: Commit**

```
git add x/knowledge/client/cli/query.go proto/zerone/knowledge/v1/query.proto x/knowledge/types/query.pb.go x/knowledge/keeper/grpc_query*.go
git commit -m "feat(knowledge): add diversity CLI queries (R28-2)"
```

---

### Task 10: Full Test Suite and Verification

**Files:**
- Test: `x/knowledge/keeper/diversity_test.go` (already created in Task 4)

**Step 1: Run full test suite**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -v -count=1 -timeout 300s`
Expected: PASS (all existing + new diversity tests)

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/... -v -count=1 -timeout 60s`
Expected: PASS (alignment tests including updated sensor)

**Step 2: Run full build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: PASS

**Step 3: Final commit with any test fixes**

If any adjustments were needed:
```
git add -A
git commit -m "test(knowledge): fix diversity test adjustments (R28-2)"
```
