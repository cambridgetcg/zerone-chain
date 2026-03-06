# R37-3 — Sample Ecology Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement fitness scoring, energy metabolism, niche competition, topic saturation, thread bonuses, and pruning for training data samples.

**Architecture:** All ecology logic lives in a new `ecology.go` file. State infrastructure (iterators, index writers) extends `state.go`. Keys extend `keys.go`. The `createSampleFromSubmission` initializes ecology fields on sample creation. BeginBlocker runs ecology every 100 blocks (ecology epoch).

**Tech Stack:** Go 1.24, Cosmos SDK v0.50.15, protobuf (types already defined in proto)

---

### Task 1: State Infrastructure — Keys, IterateSamples, Niche & At-Risk Indexes

**Files:**
- Modify: `x/knowledge/types/keys.go` — add `TopicSaturationPrefix`, `AtRiskSampleIndexPrefix`
- Modify: `x/knowledge/keeper/state.go` — add `IterateSamples`, `SetNicheIndex`, `DeleteNicheIndex`, `GetSamplesByNiche`, `SetAtRiskIndex`, `DeleteAtRiskIndex`, `IterateAtRiskSamples`, `SetTopicSaturation`, `GetTopicSaturation`, `IncrementTopicCount`
- Test: `x/knowledge/keeper/ecology_test.go`

**Step 1: Write failing tests for state infrastructure**

Create `x/knowledge/keeper/ecology_test.go`:

```go
package keeper

import (
	"testing"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── State Infrastructure Tests ─────────────────────────────────────────────

func TestIterateSamples(t *testing.T) {
	k, ctx := setupKeeper(t)

	s1 := &types.Sample{Id: "1", Domain: "tech", Content: "a"}
	s2 := &types.Sample{Id: "2", Domain: "sci", Content: "b"}
	s3 := &types.Sample{Id: "3", Domain: "tech", Content: "c"}
	_ = k.SetSample(ctx, s1)
	_ = k.SetSample(ctx, s2)
	_ = k.SetSample(ctx, s3)

	var collected []string
	k.IterateSamples(ctx, func(s *types.Sample) bool {
		collected = append(collected, s.Id)
		return false
	})
	if len(collected) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(collected))
	}
}

func TestIterateSamples_EarlyStop(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetSample(ctx, &types.Sample{Id: "1", Content: "a"})
	_ = k.SetSample(ctx, &types.Sample{Id: "2", Content: "b"})

	count := 0
	k.IterateSamples(ctx, func(s *types.Sample) bool {
		count++
		return true // stop after first
	})
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestNicheIndex_SetAndGet(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_1")
	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_2")
	_ = k.SetNicheIndex(ctx, "niche_xyz", "sample_3")

	ids := k.GetSamplesByNiche(ctx, "niche_abc")
	if len(ids) != 2 {
		t.Fatalf("expected 2 samples in niche_abc, got %d", len(ids))
	}

	ids2 := k.GetSamplesByNiche(ctx, "niche_xyz")
	if len(ids2) != 1 {
		t.Fatalf("expected 1 sample in niche_xyz, got %d", len(ids2))
	}
}

func TestNicheIndex_Delete(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_1")
	_ = k.SetNicheIndex(ctx, "niche_abc", "sample_2")
	_ = k.DeleteNicheIndex(ctx, "niche_abc", "sample_1")

	ids := k.GetSamplesByNiche(ctx, "niche_abc")
	if len(ids) != 1 {
		t.Fatalf("expected 1 after delete, got %d", len(ids))
	}
}

func TestAtRiskIndex_SetIterateDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetAtRiskIndex(ctx, "sample_1")
	_ = k.SetAtRiskIndex(ctx, "sample_2")

	var atRisk []string
	k.IterateAtRiskSamples(ctx, func(sampleID string) bool {
		atRisk = append(atRisk, sampleID)
		return false
	})
	if len(atRisk) != 2 {
		t.Fatalf("expected 2 at-risk, got %d", len(atRisk))
	}

	_ = k.DeleteAtRiskIndex(ctx, "sample_1")
	atRisk = nil
	k.IterateAtRiskSamples(ctx, func(sampleID string) bool {
		atRisk = append(atRisk, sampleID)
		return false
	})
	if len(atRisk) != 1 {
		t.Fatalf("expected 1 after delete, got %d", len(atRisk))
	}
}

func TestTopicSaturation_IncrementAndGet(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.IncrementTopicCount(ctx, "tech", "golang")
	_ = k.IncrementTopicCount(ctx, "tech", "golang")
	_ = k.IncrementTopicCount(ctx, "tech", "golang")
	_ = k.IncrementTopicCount(ctx, "sci", "physics")

	count := k.GetTopicCount(ctx, "tech", "golang")
	if count != 3 {
		t.Fatalf("expected 3, got %d", count)
	}

	count2 := k.GetTopicCount(ctx, "sci", "physics")
	if count2 != 1 {
		t.Fatalf("expected 1, got %d", count2)
	}
}

func TestTopicSaturation_UnknownIsZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	count := k.GetTopicCount(ctx, "unknown", "topic")
	if count != 0 {
		t.Fatalf("expected 0 for unknown, got %d", count)
	}
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestIterateSamples|TestNicheIndex|TestAtRiskIndex|TestTopicSaturation" -v -count=1 2>&1 | head -40`
Expected: Compilation errors — methods don't exist yet.

**Step 3: Add key prefixes**

In `x/knowledge/types/keys.go`, add after `ValidatorParticipationPrefix` (line 143):

```go
	TopicSaturationPrefix    = []byte{0xa6} // domain/topic → uint64 count
	AtRiskSampleIndexPrefix  = []byte{0xa7} // sampleID → exists (at-risk samples)
```

Add key constructors after `QueryReceiptKey` (line 293):

```go
// TopicSaturationKey returns the key for a topic's sample count.
func TopicSaturationKey(domain, topic string) []byte {
	key := append(append([]byte{}, TopicSaturationPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(topic)...)
}

// AtRiskSampleKey returns the index key for an at-risk sample.
func AtRiskSampleKey(sampleID string) []byte {
	return append(append([]byte{}, AtRiskSampleIndexPrefix...), []byte(sampleID)...)
}
```

**Step 4: Implement state methods**

In `x/knowledge/keeper/state.go`, add after the existing `GetSamplesByThread` method (after line 407):

```go
// ─── Sample iteration ───────────────────────────────────────────────────────

func (k Keeper) IterateSamples(ctx context.Context, cb func(sample *types.Sample) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.SampleKeyPrefix, prefixEndBytes(types.SampleKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var sample types.Sample
		if err := proto.Unmarshal(iter.Value(), &sample); err != nil {
			continue
		}
		if cb(&sample) {
			break
		}
	}
}

// ─── Niche index ────────────────────────────────────────────────────────────

func (k Keeper) SetNicheIndex(ctx context.Context, nicheKey, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.NicheIndexKey(nicheKey, sampleID), []byte{0x01})
}

func (k Keeper) DeleteNicheIndex(ctx context.Context, nicheKey, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.NicheIndexKey(nicheKey, sampleID))
}

func (k Keeper) GetSamplesByNiche(ctx context.Context, nicheKey string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.NicheIndexByNichePrefix(nicheKey)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var ids []string
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		id := string(key[len(prefix):])
		ids = append(ids, id)
	}
	return ids
}

// ─── At-risk sample index ───────────────────────────────────────────────────

func (k Keeper) SetAtRiskIndex(ctx context.Context, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.AtRiskSampleKey(sampleID), []byte{0x01})
}

func (k Keeper) DeleteAtRiskIndex(ctx context.Context, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.AtRiskSampleKey(sampleID))
}

func (k Keeper) IterateAtRiskSamples(ctx context.Context, cb func(sampleID string) bool) {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.AtRiskSampleIndexPrefix
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Key()[len(prefix):])
		if cb(id) {
			break
		}
	}
}

// ─── Topic saturation counters ──────────────────────────────────────────────

func (k Keeper) IncrementTopicCount(ctx context.Context, domain, topic string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.TopicSaturationKey(domain, topic)
	current := k.GetTopicCount(ctx, domain, topic)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, current+1)
	return store.Set(key, next)
}

func (k Keeper) GetTopicCount(ctx context.Context, domain, topic string) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TopicSaturationKey(domain, topic))
	if err != nil || len(bz) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}
```

**Step 5: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestIterateSamples|TestNicheIndex|TestAtRiskIndex|TestTopicSaturation" -v -count=1`
Expected: All 8 tests PASS.

**Step 6: Commit**

```bash
git add x/knowledge/types/keys.go x/knowledge/keeper/state.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): state infrastructure for sample ecology (R37-3)

Add IterateSamples, niche index, at-risk sample index, and topic
saturation counter methods. Add TopicSaturationPrefix and
AtRiskSampleIndexPrefix store keys."
```

---

### Task 2: Fitness Scoring

**Files:**
- Create: `x/knowledge/keeper/ecology.go`
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write failing tests for fitness scoring**

Append to `ecology_test.go`:

```go
// ─── Fitness Scoring Tests ──────────────────────────────────────────────────

func TestComputeSampleFitness_AllMax(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		QualityScore:   1_000_000,
		AccessCount:    1000,
		NoveltyScore:   1_000_000,
		DiversityScore: 1_000_000,
		ReasoningDepth: 1_000_000,
		TotalRevenue:   "1000000000",
	}

	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	if fitness != 1_000_000 {
		t.Fatalf("expected 1,000,000 for all-max, got %d", fitness)
	}
}

func TestComputeSampleFitness_AllZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		TotalRevenue: "0",
	}
	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	if fitness != 0 {
		t.Fatalf("expected 0 for all-zero, got %d", fitness)
	}
}

func TestComputeSampleFitness_MixedValues(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		QualityScore:   800_000,  // 80%
		AccessCount:    500,      // 50% of max
		NoveltyScore:   600_000,  // 60%
		DiversityScore: 400_000,  // 40%
		ReasoningDepth: 300_000,  // 30%
		TotalRevenue:   "200000000", // 20% of max
	}

	// quality*25 + access*25 + novelty*20 + diversity*10 + reasoning*10 + revenue*10
	// 800000*25 + 500000*25 + 600000*20 + 400000*10 + 300000*10 + 200000*10
	// = 20000000 + 12500000 + 12000000 + 4000000 + 3000000 + 2000000 = 53500000
	// / 100 = 535000
	expected := uint64(535_000)
	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	if fitness != expected {
		t.Fatalf("expected %d, got %d", expected, fitness)
	}
}

func TestComputeSampleFitness_OverMaxAccess_ClampedToMax(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		AccessCount:  5000, // Over maxAccess (1000)
		TotalRevenue: "0",
	}
	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	// access component should be clamped to 1,000,000
	// 1_000_000 * 25 / 100 = 250_000
	if fitness != 250_000 {
		t.Fatalf("expected 250000 with clamped access, got %d", fitness)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		value    uint64
		max      uint64
		expected uint64
	}{
		{"zero", 0, 1000, 0},
		{"half", 500, 1000, 500_000},
		{"full", 1000, 1000, 1_000_000},
		{"over", 2000, 1000, 1_000_000},
		{"max_zero", 100, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalize(tt.value, tt.max)
			if got != tt.expected {
				t.Fatalf("normalize(%d, %d) = %d, want %d", tt.value, tt.max, got, tt.expected)
			}
		})
	}
}

func TestParseUzrn(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"0", 0},
		{"1000000", 1_000_000},
		{"", 0},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseUzrn(tt.input)
			if got != tt.expected {
				t.Fatalf("parseUzrn(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeSampleFitness|TestNormalize|TestParseUzrn" -v -count=1 2>&1 | head -20`
Expected: Compilation errors — functions don't exist.

**Step 3: Implement fitness scoring**

Create `x/knowledge/keeper/ecology.go`:

```go
package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Constants ──────────────────────────────────────────────────────────────

const (
	// EcologyEpochBlocks is the interval at which ecology processing runs.
	EcologyEpochBlocks = 100

	// DefaultEnergyCap is the initial energy cap for new samples.
	DefaultEnergyCap uint64 = 1_000_000

	// maxAccess is the normalization ceiling for access count.
	maxAccess uint64 = 1000

	// maxRevenue is the normalization ceiling for total revenue (in uzrn).
	maxRevenue uint64 = 1_000_000_000 // 1000 ZRN

	// maxThreadBonus caps the thread fitness bonus at 30%.
	maxThreadBonus uint64 = 300_000

	// threadBonusPerMessage is the bonus per thread message.
	threadBonusPerMessage uint64 = 50_000
)

// ─── Fitness Scoring ────────────────────────────────────────────────────────

// ComputeSampleFitness calculates the weighted fitness score for a sample.
// Returns a value in 0–1,000,000 BPS range.
func (k Keeper) ComputeSampleFitness(ctx context.Context, sample *types.Sample, params *types.Params) uint64 {
	qualityComponent := sample.QualityScore
	accessComponent := normalize(sample.AccessCount, maxAccess)
	noveltyComponent := sample.NoveltyScore
	diversityComponent := sample.DiversityScore
	reasoningComponent := sample.ReasoningDepth
	revenueComponent := normalize(parseUzrn(sample.TotalRevenue), maxRevenue)

	fitness := (qualityComponent*25 +
		accessComponent*25 +
		noveltyComponent*20 +
		diversityComponent*10 +
		reasoningComponent*10 +
		revenueComponent*10) / 100

	return fitness
}

// normalize maps a value into 0–1,000,000 BPS, clamped at max.
func normalize(value, max uint64) uint64 {
	if max == 0 {
		return 0
	}
	if value >= max {
		return 1_000_000
	}
	return value * 1_000_000 / max
}

// parseUzrn parses a uzrn string amount to uint64. Returns 0 on error.
func parseUzrn(s string) uint64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
```

**Step 4: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeSampleFitness|TestNormalize|TestParseUzrn" -v -count=1`
Expected: All 9 tests PASS. (4 fitness + 5 normalize subtests)

**Step 5: Commit**

```bash
git add x/knowledge/keeper/ecology.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): fitness scoring for samples (R37-3)

Weighted fitness: quality 25%, access 25%, novelty 20%, diversity 10%,
reasoning 10%, revenue 10%. Normalize helper and uzrn parser included."
```

---

### Task 3: Energy Metabolism

**Files:**
- Modify: `x/knowledge/keeper/ecology.go`
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write failing tests**

Append to `ecology_test.go`:

```go
// ─── Energy Metabolism Tests ────────────────────────────────────────────────

func TestDecayEnergy_NormalDecay(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams() // EnergyDecayRate = 50,000 (5%)

	sample := &types.Sample{Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.DecayEnergy(ctx, sample, &params)
	// 5% of 1,000,000 = 50,000 decay → remaining 950,000
	if sample.Energy != 950_000 {
		t.Fatalf("expected 950000 after decay, got %d", sample.Energy)
	}
}

func TestDecayEnergy_MultipleCycles(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	// 3 decay cycles at 5%
	for i := 0; i < 3; i++ {
		k.DecayEnergy(ctx, sample, &params)
	}
	// 1M * 0.95^3 = 857375
	expected := uint64(857_375)
	if sample.Energy != expected {
		t.Fatalf("expected %d after 3 decays, got %d", expected, sample.Energy)
	}
}

func TestDecayEnergy_FloorAtZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{Id: "1", Energy: 10, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	// Many decay cycles should floor at 0
	for i := 0; i < 100; i++ {
		k.DecayEnergy(ctx, sample, &params)
	}
	if sample.Energy != 0 {
		t.Fatalf("expected 0 after many decays, got %d", sample.Energy)
	}
}

func TestRestoreEnergyOnAccess(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams() // EnergyPerAccess = 1,000

	sample := &types.Sample{Id: "1", Energy: 500_000, EnergyCap: 1_000_000, AtRiskSinceEpoch: 5, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.RestoreEnergyOnAccess(ctx, sample, &params)
	if sample.Energy != 501_000 {
		t.Fatalf("expected 501000, got %d", sample.Energy)
	}
	if sample.AtRiskSinceEpoch != 0 {
		t.Fatalf("expected at_risk cleared, got %d", sample.AtRiskSinceEpoch)
	}
}

func TestRestoreEnergyOnAccess_CappedAtMax(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{Id: "1", Energy: 999_500, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.RestoreEnergyOnAccess(ctx, sample, &params)
	if sample.Energy != 1_000_000 {
		t.Fatalf("expected capped at 1000000, got %d", sample.Energy)
	}
}

func TestAtRiskTransition_WhenEnergyHitsZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{Id: "1", Energy: 0, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.CheckAtRiskTransition(ctx, sample, 42, &params)
	if sample.AtRiskSinceEpoch != 42 {
		t.Fatalf("expected at_risk_since_epoch=42, got %d", sample.AtRiskSinceEpoch)
	}
}

func TestAtRiskTransition_AlreadyAtRisk_NoUpdate(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{Id: "1", Energy: 0, EnergyCap: 1_000_000, AtRiskSinceEpoch: 30, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.CheckAtRiskTransition(ctx, sample, 42, &params)
	if sample.AtRiskSinceEpoch != 30 {
		t.Fatalf("expected 30 (unchanged), got %d", sample.AtRiskSinceEpoch)
	}
}

func TestAtRiskTransition_EnergyAboveZero_NotAtRisk(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{Id: "1", Energy: 100, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.CheckAtRiskTransition(ctx, sample, 42, &params)
	if sample.AtRiskSinceEpoch != 0 {
		t.Fatalf("expected 0 (not at risk), got %d", sample.AtRiskSinceEpoch)
	}
}

func TestInitializeSampleEnergy(t *testing.T) {
	sample := &types.Sample{Id: "1"}
	initializeSampleEnergy(sample)
	if sample.Energy != DefaultEnergyCap {
		t.Fatalf("expected %d, got %d", DefaultEnergyCap, sample.Energy)
	}
	if sample.EnergyCap != DefaultEnergyCap {
		t.Fatalf("expected cap %d, got %d", DefaultEnergyCap, sample.EnergyCap)
	}
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestDecayEnergy|TestRestoreEnergy|TestAtRiskTransition|TestInitializeSampleEnergy" -v -count=1 2>&1 | head -20`
Expected: Compilation errors.

**Step 3: Implement energy metabolism**

Append to `x/knowledge/keeper/ecology.go`:

```go
// ─── Energy Metabolism ──────────────────────────────────────────────────────

// initializeSampleEnergy sets a new sample's energy fields to defaults.
func initializeSampleEnergy(sample *types.Sample) {
	sample.EnergyCap = DefaultEnergyCap
	sample.Energy = DefaultEnergyCap
}

// DecayEnergy reduces a sample's energy by the decay rate.
func (k Keeper) DecayEnergy(ctx context.Context, sample *types.Sample, params *types.Params) {
	decay := sample.Energy * params.EnergyDecayRate / 1_000_000
	if decay > sample.Energy {
		sample.Energy = 0
	} else {
		sample.Energy -= decay
	}
}

// RestoreEnergyOnAccess adds energy when a sample is accessed (purchased).
func (k Keeper) RestoreEnergyOnAccess(ctx context.Context, sample *types.Sample, params *types.Params) {
	sample.Energy += params.EnergyPerAccess
	if sample.Energy > sample.EnergyCap {
		sample.Energy = sample.EnergyCap
	}
	sample.AtRiskSinceEpoch = 0
}

// CheckAtRiskTransition marks a sample as at-risk if energy is 0.
func (k Keeper) CheckAtRiskTransition(ctx context.Context, sample *types.Sample, currentEpoch uint64, params *types.Params) {
	if sample.Energy == 0 && sample.AtRiskSinceEpoch == 0 {
		sample.AtRiskSinceEpoch = currentEpoch
		_ = k.SetAtRiskIndex(ctx, sample.Id)
	}
}
```

**Step 4: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestDecayEnergy|TestRestoreEnergy|TestAtRiskTransition|TestInitializeSampleEnergy" -v -count=1`
Expected: All 9 tests PASS.

**Step 5: Commit**

```bash
git add x/knowledge/keeper/ecology.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): energy metabolism for samples (R37-3)

Energy decay per epoch, restoration on access, at-risk transition
when energy hits zero, and sample energy initialization."
```

---

### Task 4: Niche Dynamics

**Files:**
- Modify: `x/knowledge/keeper/ecology.go`
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write failing tests**

Append to `ecology_test.go`:

```go
// ─── Niche Dynamics Tests ───────────────────────────────────────────────────

func TestComputeNicheKey_Deterministic(t *testing.T) {
	key1 := ComputeNicheKey("technology", types.SampleType_SAMPLE_TYPE_DISCUSSION, "golang")
	key2 := ComputeNicheKey("technology", types.SampleType_SAMPLE_TYPE_DISCUSSION, "golang")
	if key1 != key2 {
		t.Fatalf("niche keys not deterministic: %s vs %s", key1, key2)
	}
}

func TestComputeNicheKey_DifferentInputs(t *testing.T) {
	key1 := ComputeNicheKey("technology", types.SampleType_SAMPLE_TYPE_DISCUSSION, "golang")
	key2 := ComputeNicheKey("science", types.SampleType_SAMPLE_TYPE_DISCUSSION, "golang")
	key3 := ComputeNicheKey("technology", types.SampleType_SAMPLE_TYPE_TUTORIAL, "golang")
	key4 := ComputeNicheKey("technology", types.SampleType_SAMPLE_TYPE_DISCUSSION, "rust")

	if key1 == key2 || key1 == key3 || key1 == key4 {
		t.Fatal("different inputs should produce different niche keys")
	}
}

func TestComputeNicheKey_EmptyTopic(t *testing.T) {
	key := ComputeNicheKey("technology", types.SampleType_SAMPLE_TYPE_DISCUSSION, "")
	if len(key) != 16 { // 8 bytes hex = 16 chars
		t.Fatalf("expected 16 char hex key, got %d: %s", len(key), key)
	}
}

func TestComputeCompetitionTax_SmallNiche(t *testing.T) {
	tax := computeCompetitionTax(5, 50)
	if tax != 0 {
		t.Fatalf("expected 0 tax for small niche, got %d", tax)
	}
}

func TestComputeCompetitionTax_SaturatedNiche(t *testing.T) {
	tax := computeCompetitionTax(100, 50)
	// (100 - 50) * 10000 = 500,000 — capped at 500,000
	if tax != 500_000 {
		t.Fatalf("expected 500000 for saturated niche, got %d", tax)
	}
}

func TestComputeCompetitionTax_AtThreshold(t *testing.T) {
	tax := computeCompetitionTax(50, 50)
	if tax != 0 {
		t.Fatalf("expected 0 at threshold, got %d", tax)
	}
}

func TestUpdateNicheLeader(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create 3 samples in the same niche with different fitness
	s1 := &types.Sample{Id: "1", NicheKey: "niche_a", FitnessScore: 800_000, Content: "a"}
	s2 := &types.Sample{Id: "2", NicheKey: "niche_a", FitnessScore: 900_000, Content: "b"}
	s3 := &types.Sample{Id: "3", NicheKey: "niche_a", FitnessScore: 700_000, Content: "c"}
	_ = k.SetSample(ctx, s1)
	_ = k.SetSample(ctx, s2)
	_ = k.SetSample(ctx, s3)
	_ = k.SetNicheIndex(ctx, "niche_a", "1")
	_ = k.SetNicheIndex(ctx, "niche_a", "2")
	_ = k.SetNicheIndex(ctx, "niche_a", "3")

	k.UpdateNicheLeader(ctx, "niche_a")

	s2Updated, _ := k.GetSample(ctx, "2")
	if !s2Updated.NicheLeader {
		t.Fatal("expected sample 2 to be niche leader")
	}

	s1Updated, _ := k.GetSample(ctx, "1")
	if s1Updated.NicheLeader {
		t.Fatal("expected sample 1 to NOT be niche leader")
	}
}

func TestUpdateNicheLeader_EmptyNiche(t *testing.T) {
	k, ctx := setupKeeper(t)
	// Should not panic with empty niche
	k.UpdateNicheLeader(ctx, "empty_niche")
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeNicheKey|TestComputeCompetitionTax|TestUpdateNicheLeader" -v -count=1 2>&1 | head -20`
Expected: Compilation errors.

**Step 3: Implement niche dynamics**

Append to `x/knowledge/keeper/ecology.go`:

```go
// ─── Niche Dynamics ─────────────────────────────────────────────────────────

// ComputeNicheKey produces a deterministic 16-char hex key from domain + sample type + primary topic.
func ComputeNicheKey(domain string, sampleType types.SampleType, primaryTopic string) string {
	h := sha256.Sum256([]byte(domain + "|" + sampleType.String() + "|" + primaryTopic))
	return hex.EncodeToString(h[:8])
}

// computeCompetitionTax calculates the extra maintenance cost for saturated niches.
// Returns BPS (0–500,000). Niches below threshold pay no tax.
func computeCompetitionTax(nicheSize, saturationThreshold uint64) uint64 {
	if nicheSize <= saturationThreshold {
		return 0
	}
	tax := (nicheSize - saturationThreshold) * 10_000
	if tax > 500_000 {
		tax = 500_000
	}
	return tax
}

// UpdateNicheLeader finds the highest-fitness sample in a niche and marks it as leader.
func (k Keeper) UpdateNicheLeader(ctx context.Context, nicheKey string) {
	ids := k.GetSamplesByNiche(ctx, nicheKey)
	if len(ids) == 0 {
		return
	}

	var bestID string
	var bestFitness uint64
	for _, id := range ids {
		s, ok := k.GetSample(ctx, id)
		if !ok {
			continue
		}
		if s.FitnessScore > bestFitness {
			bestFitness = s.FitnessScore
			bestID = id
		}
	}

	// Update all samples: clear old leader, set new leader
	for _, id := range ids {
		s, ok := k.GetSample(ctx, id)
		if !ok {
			continue
		}
		wasLeader := s.NicheLeader
		isLeader := id == bestID
		if wasLeader != isLeader {
			s.NicheLeader = isLeader
			_ = k.SetSample(ctx, s)
		}
	}
}
```

**Step 4: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeNicheKey|TestComputeCompetitionTax|TestUpdateNicheLeader" -v -count=1`
Expected: All 8 tests PASS.

**Step 5: Commit**

```bash
git add x/knowledge/keeper/ecology.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): niche dynamics — key computation, competition tax, leader selection (R37-3)"
```

---

### Task 5: Topic Saturation

**Files:**
- Modify: `x/knowledge/keeper/ecology.go`
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write failing tests**

Append to `ecology_test.go`:

```go
// ─── Topic Saturation Tests ─────────────────────────────────────────────────

func TestComputeTopicSaturation_NoSamples(t *testing.T) {
	k, ctx := setupKeeper(t)
	sat := k.ComputeTopicSaturation(ctx, "tech", "golang")
	if sat != 0 {
		t.Fatalf("expected 0 for no samples, got %d", sat)
	}
}

func TestComputeTopicSaturation_BelowThreshold(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams() // NicheSaturationThreshold = 50
	for i := 0; i < 10; i++ {
		_ = k.IncrementTopicCount(ctx, "tech", "golang")
	}
	sat := k.ComputeTopicSaturation(ctx, "tech", "golang")
	// 10 samples, threshold 50: ratio = 10/50 * 1M = 200,000
	// Use params in the actual function
	_ = params
	if sat == 0 {
		t.Fatal("expected non-zero saturation")
	}
}

func TestComputeTopicSaturation_AboveThreshold(t *testing.T) {
	k, ctx := setupKeeper(t)
	for i := 0; i < 100; i++ {
		_ = k.IncrementTopicCount(ctx, "tech", "golang")
	}
	sat := k.ComputeTopicSaturation(ctx, "tech", "golang")
	if sat != 1_000_000 {
		t.Fatalf("expected capped at 1,000,000, got %d", sat)
	}
}

func TestApplyNoveltyAdjustment_LowSaturation(t *testing.T) {
	// Below threshold → no penalty
	adjusted := ApplyNoveltyAdjustment(800_000, 200_000)
	if adjusted != 800_000 {
		t.Fatalf("expected no adjustment at low saturation, got %d", adjusted)
	}
}

func TestApplyNoveltyAdjustment_HighSaturation(t *testing.T) {
	// saturation=800,000, threshold=500,000
	// penalty = (800,000 - 500,000) * 500,000 / 1,000,000 = 150,000
	// adjusted = 600,000 * (1,000,000 - 150,000) / 1,000,000 = 510,000
	adjusted := ApplyNoveltyAdjustment(600_000, 800_000)
	expected := uint64(510_000)
	if adjusted != expected {
		t.Fatalf("expected %d, got %d", expected, adjusted)
	}
}

func TestApplyNoveltyAdjustment_MaxSaturation(t *testing.T) {
	// saturation=1,000,000 → max penalty
	// penalty = (1,000,000 - 500,000) * 500,000 / 1,000,000 = 250,000
	// adjusted = 1,000,000 * (1,000,000 - 250,000) / 1,000,000 = 750,000
	adjusted := ApplyNoveltyAdjustment(1_000_000, 1_000_000)
	expected := uint64(750_000)
	if adjusted != expected {
		t.Fatalf("expected %d, got %d", expected, adjusted)
	}
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeTopicSaturation|TestApplyNoveltyAdjustment" -v -count=1 2>&1 | head -20`
Expected: Compilation errors.

**Step 3: Implement topic saturation**

Append to `x/knowledge/keeper/ecology.go`:

```go
// ─── Topic Saturation ───────────────────────────────────────────────────────

const (
	// saturationThreshold is the BPS level above which novelty is penalized.
	saturationThreshold uint64 = 500_000
)

// ComputeTopicSaturation returns a saturation score 0–1,000,000 for a domain+topic.
// Capped at 1,000,000 when topic count >= 2 * niche saturation threshold.
func (k Keeper) ComputeTopicSaturation(ctx context.Context, domain, topic string) uint64 {
	count := k.GetTopicCount(ctx, domain, topic)
	if count == 0 {
		return 0
	}
	// Use 2x niche saturation threshold (default 50 → 100) as the "fully saturated" count.
	// This maps count 0..100 → saturation 0..1,000,000.
	maxCount := uint64(100)
	return normalize(count, maxCount)
}

// ApplyNoveltyAdjustment reduces novelty score for over-saturated topics.
func ApplyNoveltyAdjustment(noveltyScore, saturation uint64) uint64 {
	if saturation <= saturationThreshold {
		return noveltyScore
	}
	penalty := (saturation - saturationThreshold) * 500_000 / 1_000_000
	return noveltyScore * (1_000_000 - penalty) / 1_000_000
}
```

**Step 4: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeTopicSaturation|TestApplyNoveltyAdjustment" -v -count=1`
Expected: All 6 tests PASS.

**Step 5: Commit**

```bash
git add x/knowledge/keeper/ecology.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): topic saturation with novelty diminishing returns (R37-3)"
```

---

### Task 6: Thread Bonus

**Files:**
- Modify: `x/knowledge/keeper/ecology.go`
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write failing tests**

Append to `ecology_test.go`:

```go
// ─── Thread Bonus Tests ─────────────────────────────────────────────────────

func TestComputeThreadBonus_NoThread(t *testing.T) {
	k, ctx := setupKeeper(t)
	sample := &types.Sample{Id: "1", ThreadId: ""}
	bonus := k.ComputeThreadBonus(ctx, sample)
	if bonus != 0 {
		t.Fatalf("expected 0 for no thread, got %d", bonus)
	}
}

func TestComputeThreadBonus_TwoMessages(t *testing.T) {
	k, ctx := setupKeeper(t)

	s1 := &types.Sample{Id: "1", ThreadId: "thread_1", Content: "a"}
	s2 := &types.Sample{Id: "2", ThreadId: "thread_1", Content: "b"}
	_ = k.SetSample(ctx, s1)
	_ = k.SetSample(ctx, s2)
	_ = k.SetSampleThreadIndex(ctx, "thread_1", "1")
	_ = k.SetSampleThreadIndex(ctx, "thread_1", "2")

	bonus := k.ComputeThreadBonus(ctx, s1)
	expected := uint64(100_000) // 2 * 50,000
	if bonus != expected {
		t.Fatalf("expected %d, got %d", expected, bonus)
	}
}

func TestComputeThreadBonus_FiveMessages(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i := 1; i <= 5; i++ {
		id := strconv.Itoa(i)
		_ = k.SetSample(ctx, &types.Sample{Id: id, ThreadId: "t1", Content: id})
		_ = k.SetSampleThreadIndex(ctx, "t1", id)
	}

	sample := &types.Sample{Id: "1", ThreadId: "t1"}
	bonus := k.ComputeThreadBonus(ctx, sample)
	expected := uint64(250_000) // 5 * 50,000
	if bonus != expected {
		t.Fatalf("expected %d, got %d", expected, bonus)
	}
}

func TestComputeThreadBonus_CappedAt300k(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i := 1; i <= 15; i++ {
		id := strconv.Itoa(i)
		_ = k.SetSample(ctx, &types.Sample{Id: id, ThreadId: "t2", Content: id})
		_ = k.SetSampleThreadIndex(ctx, "t2", id)
	}

	sample := &types.Sample{Id: "1", ThreadId: "t2"}
	bonus := k.ComputeThreadBonus(ctx, sample)
	if bonus != maxThreadBonus {
		t.Fatalf("expected capped at %d, got %d", maxThreadBonus, bonus)
	}
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeThreadBonus" -v -count=1 2>&1 | head -20`
Expected: Compilation errors.

**Step 3: Implement thread bonus**

Append to `x/knowledge/keeper/ecology.go`:

```go
// ─── Thread Bonus ───────────────────────────────────────────────────────────

// ComputeThreadBonus returns a fitness bonus for samples in a conversation thread.
// Bonus scales with thread length, capped at maxThreadBonus (300,000 = 30%).
func (k Keeper) ComputeThreadBonus(ctx context.Context, sample *types.Sample) uint64 {
	if sample.ThreadId == "" {
		return 0
	}
	threadSampleIDs := k.GetSamplesByThread(ctx, sample.ThreadId)
	threadLength := uint64(len(threadSampleIDs))
	bonus := threadLength * threadBonusPerMessage
	if bonus > maxThreadBonus {
		bonus = maxThreadBonus
	}
	return bonus
}
```

**Step 4: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestComputeThreadBonus" -v -count=1`
Expected: All 4 tests PASS.

**Step 5: Commit**

```bash
git add x/knowledge/keeper/ecology.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): thread bonus for conversation samples (R37-3)

Bonus scales with thread length: 50k BPS per message, capped at 300k (30%)."
```

---

### Task 7: Pruning

**Files:**
- Modify: `x/knowledge/keeper/ecology.go`
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write failing tests**

Append to `ecology_test.go`:

```go
// ─── Pruning Tests ──────────────────────────────────────────────────────────

func TestPruneSamples_AfterGracePeriod(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams() // PruneGraceEpochs = 10

	sample := &types.Sample{
		Id:               "1",
		Content:          "will be pruned",
		Energy:           0,
		AtRiskSinceEpoch: 5,
		Status:           types.SampleStatus_SAMPLE_STATUS_GOLD,
	}
	_ = k.SetSample(ctx, sample)
	_ = k.SetAtRiskIndex(ctx, "1")

	k.PruneSamples(ctx, 16, &params) // epoch 16: grace = 16-5 = 11 >= 10

	pruned, ok := k.GetSample(ctx, "1")
	if !ok {
		t.Fatal("sample should still exist (as record)")
	}
	if pruned.Status != types.SampleStatus_SAMPLE_STATUS_PRUNED {
		t.Fatalf("expected PRUNED status, got %v", pruned.Status)
	}
	if pruned.Content != "" {
		t.Fatal("expected content to be cleared after pruning")
	}
}

func TestPruneSamples_WithinGracePeriod(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		Id:               "1",
		Content:          "still alive",
		Energy:           0,
		AtRiskSinceEpoch: 5,
		Status:           types.SampleStatus_SAMPLE_STATUS_GOLD,
	}
	_ = k.SetSample(ctx, sample)
	_ = k.SetAtRiskIndex(ctx, "1")

	k.PruneSamples(ctx, 10, &params) // epoch 10: grace = 10-5 = 5 < 10

	alive, ok := k.GetSample(ctx, "1")
	if !ok {
		t.Fatal("sample should exist")
	}
	if alive.Status == types.SampleStatus_SAMPLE_STATUS_PRUNED {
		t.Fatal("sample should NOT be pruned within grace period")
	}
	if alive.Content == "" {
		t.Fatal("content should still exist")
	}
}

func TestPruneSamples_MultipleSamples_SelectivelyPrunes(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	// Sample 1: past grace
	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Content: "old", Energy: 0, AtRiskSinceEpoch: 1,
		Status: types.SampleStatus_SAMPLE_STATUS_BRONZE,
	})
	_ = k.SetAtRiskIndex(ctx, "1")

	// Sample 2: within grace
	_ = k.SetSample(ctx, &types.Sample{
		Id: "2", Content: "new", Energy: 0, AtRiskSinceEpoch: 9,
		Status: types.SampleStatus_SAMPLE_STATUS_SILVER,
	})
	_ = k.SetAtRiskIndex(ctx, "2")

	k.PruneSamples(ctx, 12, &params) // epoch 12: s1 grace=11>=10 (prune), s2 grace=3<10 (keep)

	s1, _ := k.GetSample(ctx, "1")
	if s1.Status != types.SampleStatus_SAMPLE_STATUS_PRUNED {
		t.Fatal("sample 1 should be pruned")
	}

	s2, _ := k.GetSample(ctx, "2")
	if s2.Status == types.SampleStatus_SAMPLE_STATUS_PRUNED {
		t.Fatal("sample 2 should NOT be pruned")
	}
}

func TestPruneSamples_ZeroAtRiskEpoch_NotPruned(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Content: "safe", Energy: 0, AtRiskSinceEpoch: 0,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
	})
	_ = k.SetAtRiskIndex(ctx, "1")

	k.PruneSamples(ctx, 100, &params)

	s, _ := k.GetSample(ctx, "1")
	if s.Status == types.SampleStatus_SAMPLE_STATUS_PRUNED {
		t.Fatal("should not prune sample with at_risk_since_epoch=0")
	}
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestPruneSamples" -v -count=1 2>&1 | head -20`
Expected: Compilation errors.

**Step 3: Implement pruning**

Append to `x/knowledge/keeper/ecology.go`:

```go
// ─── Pruning ────────────────────────────────────────────────────────────────

// PruneSamples removes samples that have been at-risk beyond the grace period.
// Content is cleared to save storage, but the provenance record is kept.
func (k Keeper) PruneSamples(ctx context.Context, currentEpoch uint64, params *types.Params) {
	var toPrune []string

	k.IterateAtRiskSamples(ctx, func(sampleID string) bool {
		sample, ok := k.GetSample(ctx, sampleID)
		if !ok {
			toPrune = append(toPrune, sampleID) // orphaned index entry
			return false
		}
		if sample.AtRiskSinceEpoch == 0 {
			return false
		}
		gracePeriod := currentEpoch - sample.AtRiskSinceEpoch
		if gracePeriod >= params.PruneGraceEpochs {
			sample.Status = types.SampleStatus_SAMPLE_STATUS_PRUNED
			sample.Content = ""
			_ = k.SetSample(ctx, sample)
			toPrune = append(toPrune, sampleID)
		}
		return false
	})

	// Clean up at-risk index for pruned samples
	for _, id := range toPrune {
		_ = k.DeleteAtRiskIndex(ctx, id)
	}
}
```

**Step 4: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestPruneSamples" -v -count=1`
Expected: All 4 tests PASS.

**Step 5: Commit**

```bash
git add x/knowledge/keeper/ecology.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): sample pruning after grace period (R37-3)

Samples at zero energy for longer than PruneGraceEpochs get pruned.
Content is cleared but provenance record is kept."
```

---

### Task 8: Integration — Wire Ecology Into Sample Creation & BeginBlocker

**Files:**
- Modify: `x/knowledge/keeper/quality_round.go` — update `createSampleFromSubmission` and `createThreadSamples`
- Modify: `x/knowledge/keeper/phases.go` — add ecology epoch processing
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write failing tests for integration**

Append to `ecology_test.go`:

```go
// ─── Integration Tests ──────────────────────────────────────────────────────

func TestCreateSample_InitializesEcologyFields(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id:        "sub1",
		Content:   "test content",
		Domain:    "technology",
		Submitter: "zrn1submitter",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Topics:    []string{"golang", "testing"},
		Consent:   &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		License:   "MIT",
	}
	_ = k.SetSubmission(ctx, sub)

	scores := &types.QualityVote{
		OverallQuality: 850_000,
		Novelty:        700_000,
		ReasoningDepth: 600_000,
	}
	err := k.CreateSampleFromSubmission(ctx, sub, types.QualityVerdict_QUALITY_VERDICT_GOLD, scores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sample, ok := k.GetSample(ctx, "1")
	if !ok {
		t.Fatal("sample not found")
	}

	// Energy initialized
	if sample.Energy != DefaultEnergyCap {
		t.Fatalf("expected energy %d, got %d", DefaultEnergyCap, sample.Energy)
	}
	if sample.EnergyCap != DefaultEnergyCap {
		t.Fatalf("expected energy cap %d, got %d", DefaultEnergyCap, sample.EnergyCap)
	}

	// Niche key computed
	if sample.NicheKey == "" {
		t.Fatal("expected niche key to be set")
	}

	// Niche index created
	ids := k.GetSamplesByNiche(ctx, sample.NicheKey)
	if len(ids) != 1 || ids[0] != "1" {
		t.Fatalf("expected niche index entry, got %v", ids)
	}

	// Topics propagated
	if len(sample.Topics) == 0 || sample.Topics[0] != "golang" {
		t.Fatalf("expected topics propagated, got %v", sample.Topics)
	}
}

func TestCreateSample_IncreasesTopicSaturation(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id:         "sub1",
		Content:    "test content",
		Domain:     "technology",
		Submitter:  "zrn1submitter",
		SampleType: types.SampleType_SAMPLE_TYPE_TUTORIAL,
		Topics:     []string{"golang"},
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		License:    "MIT",
	}
	_ = k.SetSubmission(ctx, sub)

	scores := &types.QualityVote{OverallQuality: 800_000}
	_ = k.CreateSampleFromSubmission(ctx, sub, types.QualityVerdict_QUALITY_VERDICT_GOLD, scores)

	count := k.GetTopicCount(ctx, "technology", "golang")
	if count != 1 {
		t.Fatalf("expected topic count 1, got %d", count)
	}
}

func TestEcologyEpoch_DecaysAndPrunes(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Sample with energy — should decay
	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "alive",
	})

	// Sample at risk past grace — should be pruned
	_ = k.SetSample(ctx, &types.Sample{
		Id: "2", Energy: 0, EnergyCap: 1_000_000, AtRiskSinceEpoch: 1,
		Status: types.SampleStatus_SAMPLE_STATUS_BRONZE, Content: "dying",
	})
	_ = k.SetAtRiskIndex(ctx, "2")

	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	k.RunEcologyEpoch(ctx, 12) // epoch 12: s2 grace=11>=10

	s1, _ := k.GetSample(ctx, "1")
	if s1.Energy >= 1_000_000 {
		t.Fatal("expected energy to decay")
	}

	s2, _ := k.GetSample(ctx, "2")
	if s2.Status != types.SampleStatus_SAMPLE_STATUS_PRUNED {
		t.Fatalf("expected sample 2 pruned, got %v", s2.Status)
	}
}

func TestEcologyEpoch_MarksAtRisk(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Sample with zero energy, not yet at-risk
	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 0, EnergyCap: 1_000_000, AtRiskSinceEpoch: 0,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
	})

	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	k.RunEcologyEpoch(ctx, 5)

	s, _ := k.GetSample(ctx, "1")
	if s.AtRiskSinceEpoch != 5 {
		t.Fatalf("expected at_risk_since_epoch=5, got %d", s.AtRiskSinceEpoch)
	}
}

func TestEcologyEpoch_UpdatesFitness(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		QualityScore: 800_000, NoveltyScore: 600_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	k.RunEcologyEpoch(ctx, 1)

	s, _ := k.GetSample(ctx, "1")
	if s.FitnessScore == 0 {
		t.Fatal("expected fitness score to be computed")
	}
}
```

**Step 2: Run tests to verify failure**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestCreateSample_Initializes|TestCreateSample_Increases|TestEcologyEpoch" -v -count=1 2>&1 | head -20`
Expected: Compilation errors.

**Step 3: Update createSampleFromSubmission to initialize ecology fields**

In `x/knowledge/keeper/quality_round.go`, update the `createSampleFromSubmission` function. After the existing field assignments (after line 393 where `VerifiedAtBlock` is set), add ecology initialization:

```go
		// ─── Ecology initialization (R37-3) ─────────────────────────────
		Topics: sub.Topics,
```

And after calling `SetSample` and setting indexes but before the thread handling (between lines 409 and 411), add:

```go
	// Initialize ecology fields
	initializeSampleEnergy(sample)
	primaryTopic := ""
	if len(sub.Topics) > 0 {
		primaryTopic = sub.Topics[0]
	}
	sample.NicheKey = ComputeNicheKey(sub.Domain, sub.SampleType, primaryTopic)

	if err := k.SetSample(ctx, sample); err != nil {
		return err
	}
	if err := k.SetNicheIndex(ctx, sample.NicheKey, sampleID); err != nil {
		return err
	}
	// Track topic saturation for each topic
	for _, topic := range sub.Topics {
		_ = k.IncrementTopicCount(ctx, sub.Domain, topic)
	}
```

Also rename the existing private `createSampleFromSubmission` to public `CreateSampleFromSubmission` so tests can call it directly (or keep private and test via the aggregation path — decision: make it public for testability).

**Similarly update `createThreadSamples`** to initialize ecology fields on sibling samples.

**Step 4: Implement RunEcologyEpoch**

Append to `x/knowledge/keeper/ecology.go`:

```go
// ─── Ecology Epoch Processing ───────────────────────────────────────────────

// RunEcologyEpoch performs all ecology processing for the current epoch.
// Called from BeginBlocker every EcologyEpochBlocks.
func (k Keeper) RunEcologyEpoch(ctx context.Context, currentEpoch uint64) {
	params, err := k.GetParams(ctx)
	if err != nil || params == nil {
		return
	}

	// Phase 1: Iterate all active samples — decay energy, compute fitness, check at-risk
	k.IterateSamples(ctx, func(sample *types.Sample) bool {
		if sample.Status == types.SampleStatus_SAMPLE_STATUS_PRUNED ||
			sample.Status == types.SampleStatus_SAMPLE_STATUS_REJECTED {
			return false
		}

		// Decay energy
		k.DecayEnergy(ctx, sample, params)

		// Compute fitness
		sample.FitnessScore = k.ComputeSampleFitness(ctx, sample, params)
		sample.FitnessUpdatedBlock = currentEpoch * EcologyEpochBlocks
		sample.EnergyLastUpdated = currentEpoch * EcologyEpochBlocks

		// Check at-risk
		k.CheckAtRiskTransition(ctx, sample, currentEpoch, params)

		_ = k.SetSample(ctx, sample)
		return false
	})

	// Phase 2: Prune samples past grace period
	k.PruneSamples(ctx, currentEpoch, params)
}
```

**Step 5: Wire into BeginBlocker**

In `x/knowledge/keeper/phases.go`, add at the end of the `BeginBlocker` method (before the final `return nil`):

```go
	// ─── Ecology epoch processing (R37-3) ───────────────────────────────
	blockHeight := uint64(sdkCtx.BlockHeight())
	if blockHeight > 0 && blockHeight%EcologyEpochBlocks == 0 {
		epoch := blockHeight / EcologyEpochBlocks
		k.RunEcologyEpoch(ctx, epoch)
	}
```

**Step 6: Run tests to verify pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestCreateSample_Initializes|TestCreateSample_Increases|TestEcologyEpoch" -v -count=1`
Expected: All 5 tests PASS.

**Step 7: Run full test suite to check for regressions**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -v -count=1`
Expected: All existing + new tests PASS.

**Step 8: Commit**

```bash
git add x/knowledge/keeper/ecology.go x/knowledge/keeper/ecology_test.go x/knowledge/keeper/quality_round.go x/knowledge/keeper/phases.go
git commit -m "feat(knowledge): wire ecology into sample creation and BeginBlocker (R37-3)

Samples get initialized energy, niche key, and topic saturation on creation.
RunEcologyEpoch decays energy, computes fitness, marks at-risk, and prunes
every 100 blocks."
```

---

### Task 9: Final Test Coverage — Edge Cases & Full Lifecycle

**Files:**
- Test: `x/knowledge/keeper/ecology_test.go` (append)

**Step 1: Write remaining edge case tests**

Append to `ecology_test.go`:

```go
// ─── Edge Case Tests ────────────────────────────────────────────────────────

func TestDecayEnergy_ZeroDecayRate(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	params.EnergyDecayRate = 0

	sample := &types.Sample{Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.DecayEnergy(ctx, sample, &params)
	if sample.Energy != 1_000_000 {
		t.Fatalf("expected no decay with rate=0, got %d", sample.Energy)
	}
}

func TestDecayEnergy_MaxDecayRate(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	params.EnergyDecayRate = 1_000_000 // 100%

	sample := &types.Sample{Id: "1", Energy: 500_000, EnergyCap: 1_000_000, Content: "x"}
	_ = k.SetSample(ctx, sample)

	k.DecayEnergy(ctx, sample, &params)
	if sample.Energy != 0 {
		t.Fatalf("expected 0 with 100%% decay, got %d", sample.Energy)
	}
}

func TestComputeSampleFitness_EmptyRevenue(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		QualityScore: 500_000,
		TotalRevenue: "", // empty string
	}
	fitness := k.ComputeSampleFitness(ctx, sample, &params)
	// Only quality contributes: 500_000 * 25 / 100 = 125,000
	if fitness != 125_000 {
		t.Fatalf("expected 125000, got %d", fitness)
	}
}

func TestComputeNicheKey_Length(t *testing.T) {
	key := ComputeNicheKey("a", types.SampleType_SAMPLE_TYPE_UNSPECIFIED, "b")
	if len(key) != 16 {
		t.Fatalf("expected 16 char hex string, got %d: %s", len(key), key)
	}
}

func TestRestoreEnergyOnAccess_ClearsAtRisk(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	sample := &types.Sample{
		Id: "1", Energy: 0, EnergyCap: 1_000_000,
		AtRiskSinceEpoch: 42, Content: "x",
	}
	_ = k.SetSample(ctx, sample)
	_ = k.SetAtRiskIndex(ctx, "1")

	k.RestoreEnergyOnAccess(ctx, sample, &params)

	if sample.AtRiskSinceEpoch != 0 {
		t.Fatal("at-risk should be cleared on access")
	}
	if sample.Energy != params.EnergyPerAccess {
		t.Fatalf("expected %d, got %d", params.EnergyPerAccess, sample.Energy)
	}
}

func TestPruneSamples_NoAtRiskSamples(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()

	// Should not panic when no at-risk samples exist
	k.PruneSamples(ctx, 100, &params)
}

func TestComputeCompetitionTax_VeryLargeNiche(t *testing.T) {
	tax := computeCompetitionTax(10000, 50)
	if tax != 500_000 {
		t.Fatalf("expected capped at 500,000, got %d", tax)
	}
}

func TestRunEcologyEpoch_SkipsPrunedSamples(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_PRUNED, Content: "",
		TotalRevenue: "0",
	})

	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	k.RunEcologyEpoch(ctx, 1)

	s, _ := k.GetSample(ctx, "1")
	// Energy should not decay for pruned samples
	if s.Energy != 1_000_000 {
		t.Fatalf("expected pruned sample energy unchanged, got %d", s.Energy)
	}
}

func TestRunEcologyEpoch_SkipsRejectedSamples(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_REJECTED, Content: "",
		TotalRevenue: "0",
	})

	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	k.RunEcologyEpoch(ctx, 1)

	s, _ := k.GetSample(ctx, "1")
	if s.Energy != 1_000_000 {
		t.Fatalf("expected rejected sample energy unchanged, got %d", s.Energy)
	}
}

// ─── Full Lifecycle Test ────────────────────────────────────────────────────

func TestFullEcologyLifecycle(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	// 1. Create sample with full energy
	sample := &types.Sample{
		Id: "lifecycle", Content: "test lifecycle", Energy: DefaultEnergyCap,
		EnergyCap: DefaultEnergyCap, QualityScore: 800_000, NoveltyScore: 600_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, TotalRevenue: "0",
	}
	_ = k.SetSample(ctx, sample)

	// 2. Run ecology — energy should decay
	k.RunEcologyEpoch(ctx, 1)
	s, _ := k.GetSample(ctx, "lifecycle")
	if s.Energy >= DefaultEnergyCap {
		t.Fatal("energy should have decayed")
	}
	if s.FitnessScore == 0 {
		t.Fatal("fitness should be computed")
	}

	// 3. Run many epochs without access — energy drops to 0
	for epoch := uint64(2); epoch <= 200; epoch++ {
		k.RunEcologyEpoch(ctx, epoch)
	}

	s, _ = k.GetSample(ctx, "lifecycle")
	if s.Energy != 0 {
		t.Fatalf("expected energy 0 after many epochs, got %d", s.Energy)
	}
	if s.AtRiskSinceEpoch == 0 {
		t.Fatal("expected sample to be at-risk")
	}

	// 4. Access restores energy
	k.RestoreEnergyOnAccess(ctx, s, &params)
	_ = k.SetSample(ctx, s)
	if s.Energy == 0 {
		t.Fatal("expected energy restored after access")
	}
	if s.AtRiskSinceEpoch != 0 {
		t.Fatal("expected at-risk cleared after access")
	}
}
```

**Step 2: Run all ecology tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "Test.*" -v -count=1 2>&1 | grep -E "^--- (PASS|FAIL)" | wc -l`
Expected: ≥ 40 test functions pass.

**Step 3: Run full test suite for regressions**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -count=1`
Expected: All tests PASS.

**Step 4: Commit**

```bash
git add x/knowledge/keeper/ecology_test.go
git commit -m "test(knowledge): comprehensive ecology edge cases and lifecycle tests (R37-3)

Covers zero/max decay rates, empty revenue, pruned/rejected sample skipping,
competition tax caps, and a full create→decay→at-risk→restore lifecycle.
Target: ≥40 tests total for ecology."
```

---

## Test Inventory (≥ 40 tests)

| # | Test Name | Task |
|---|-----------|------|
| 1 | TestIterateSamples | 1 |
| 2 | TestIterateSamples_EarlyStop | 1 |
| 3 | TestNicheIndex_SetAndGet | 1 |
| 4 | TestNicheIndex_Delete | 1 |
| 5 | TestAtRiskIndex_SetIterateDelete | 1 |
| 6 | TestTopicSaturation_IncrementAndGet | 1 |
| 7 | TestTopicSaturation_UnknownIsZero | 1 |
| 8 | TestComputeSampleFitness_AllMax | 2 |
| 9 | TestComputeSampleFitness_AllZero | 2 |
| 10 | TestComputeSampleFitness_MixedValues | 2 |
| 11 | TestComputeSampleFitness_OverMaxAccess_ClampedToMax | 2 |
| 12 | TestNormalize (5 subtests) | 2 |
| 13 | TestParseUzrn (4 subtests) | 2 |
| 14 | TestDecayEnergy_NormalDecay | 3 |
| 15 | TestDecayEnergy_MultipleCycles | 3 |
| 16 | TestDecayEnergy_FloorAtZero | 3 |
| 17 | TestRestoreEnergyOnAccess | 3 |
| 18 | TestRestoreEnergyOnAccess_CappedAtMax | 3 |
| 19 | TestAtRiskTransition_WhenEnergyHitsZero | 3 |
| 20 | TestAtRiskTransition_AlreadyAtRisk_NoUpdate | 3 |
| 21 | TestAtRiskTransition_EnergyAboveZero_NotAtRisk | 3 |
| 22 | TestInitializeSampleEnergy | 3 |
| 23 | TestComputeNicheKey_Deterministic | 4 |
| 24 | TestComputeNicheKey_DifferentInputs | 4 |
| 25 | TestComputeNicheKey_EmptyTopic | 4 |
| 26 | TestComputeCompetitionTax_SmallNiche | 4 |
| 27 | TestComputeCompetitionTax_SaturatedNiche | 4 |
| 28 | TestComputeCompetitionTax_AtThreshold | 4 |
| 29 | TestUpdateNicheLeader | 4 |
| 30 | TestUpdateNicheLeader_EmptyNiche | 4 |
| 31 | TestComputeTopicSaturation_NoSamples | 5 |
| 32 | TestComputeTopicSaturation_BelowThreshold | 5 |
| 33 | TestComputeTopicSaturation_AboveThreshold | 5 |
| 34 | TestApplyNoveltyAdjustment_LowSaturation | 5 |
| 35 | TestApplyNoveltyAdjustment_HighSaturation | 5 |
| 36 | TestApplyNoveltyAdjustment_MaxSaturation | 5 |
| 37 | TestComputeThreadBonus_NoThread | 6 |
| 38 | TestComputeThreadBonus_TwoMessages | 6 |
| 39 | TestComputeThreadBonus_FiveMessages | 6 |
| 40 | TestComputeThreadBonus_CappedAt300k | 6 |
| 41 | TestPruneSamples_AfterGracePeriod | 7 |
| 42 | TestPruneSamples_WithinGracePeriod | 7 |
| 43 | TestPruneSamples_MultipleSamples_SelectivelyPrunes | 7 |
| 44 | TestPruneSamples_ZeroAtRiskEpoch_NotPruned | 7 |
| 45 | TestCreateSample_InitializesEcologyFields | 8 |
| 46 | TestCreateSample_IncreasesTopicSaturation | 8 |
| 47 | TestEcologyEpoch_DecaysAndPrunes | 8 |
| 48 | TestEcologyEpoch_MarksAtRisk | 8 |
| 49 | TestEcologyEpoch_UpdatesFitness | 8 |
| 50 | TestDecayEnergy_ZeroDecayRate | 9 |
| 51 | TestDecayEnergy_MaxDecayRate | 9 |
| 52 | TestComputeSampleFitness_EmptyRevenue | 9 |
| 53 | TestComputeNicheKey_Length | 9 |
| 54 | TestRestoreEnergyOnAccess_ClearsAtRisk | 9 |
| 55 | TestPruneSamples_NoAtRiskSamples | 9 |
| 56 | TestComputeCompetitionTax_VeryLargeNiche | 9 |
| 57 | TestRunEcologyEpoch_SkipsPrunedSamples | 9 |
| 58 | TestRunEcologyEpoch_SkipsRejectedSamples | 9 |
| 59 | TestFullEcologyLifecycle | 9 |

**Total: 59 test functions** (including 9 subtests in normalize and parseUzrn)
