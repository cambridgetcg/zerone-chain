# Retroactive Vindication Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When a verified fact is later disproven via challenge, minority voters who were slashed for voting against the (now-disproven) majority are refunded from escrow plus receive a bonus from slashing the majority.

**Architecture:** Vindication logic in a dedicated `keeper/vindication.go` within the knowledge module. Minority wrong-vote slashes are escrowed (not sent to development_fund). On challenge success that disproves a fact, escrowed tokens are refunded and majority voters are slashed. Pruning cleans up expired entries, sending unclaimed escrow to treasury.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15 (store, bank, module accounts), protobuf for params

---

### Task 1: Add Vindication Types

**Files:**
- Create: `x/knowledge/types/vindication.go`

**Step 1: Create the types file**

```go
package types

// VindicationEntry tracks a minority voter who was slashed and may be vindicated.
// Stored as JSON under VindicationPendingPrefix.
type VindicationEntry struct {
	Verifier    string `json:"verifier"`
	Vote        string `json:"vote"`
	SlashAmount string `json:"slash_amount"` // string for big.Int compat
	SlashBps    uint64 `json:"slash_bps"`
	RoundId     string `json:"round_id"`
	FactId      string `json:"fact_id"`
	Height      uint64 `json:"height"`
}

// VindicationRecord is an immutable record of an executed vindication.
// Stored as JSON under VindicationRecordPrefix.
type VindicationRecord struct {
	Verifier     string `json:"verifier"`
	FactId       string `json:"fact_id"`
	RefundAmount string `json:"refund_amount"`
	BonusAmount  string `json:"bonus_amount"`
	VindicatedAt uint64 `json:"vindicated_at"`
	DisprovenBy  string `json:"disproven_by"`
	RoundId      string `json:"round_id"`
}

// VindicationEscrowModuleName is the module account that holds slashed minority tokens
// until vindication fires or the window expires.
const VindicationEscrowModuleName = "vindication_escrow"
```

**Step 2: Commit**

```bash
git add x/knowledge/types/vindication.go
git commit -m "feat(knowledge): add VindicationEntry and VindicationRecord types (R28-1)"
```

---

### Task 2: Add Store Prefixes and Key Constructors

**Files:**
- Modify: `x/knowledge/types/keys.go:111-117` (add prefixes after `DomainEpochRoundIndexPrefix`)

**Step 1: Add prefixes to `keys.go`**

After `DomainEpochRoundIndexPrefix = []byte{0x44}` (line 116), add:

```go
	// ─── Retroactive vindication (R28-1) ────────────────────────────────
	VindicationPendingPrefix = []byte{0x50} // 0x50 | factID → []VindicationEntry (JSON)
	VindicationRecordPrefix  = []byte{0x51} // 0x51 | factID / verifier → VindicationRecord (JSON)
```

**Step 2: Add key constructors**

After the existing key constructor functions (around line 200+), add:

```go
// VindicationPendingKey returns the store key for pending vindications for a fact.
func VindicationPendingKey(factId string) []byte {
	return append(append([]byte{}, VindicationPendingPrefix...), []byte(factId)...)
}

// VindicationRecordKey returns the store key for a vindication record.
func VindicationRecordKey(factId, verifier string) []byte {
	key := append([]byte{}, VindicationRecordPrefix...)
	key = append(key, []byte(factId)...)
	key = append(key, '/')
	key = append(key, []byte(verifier)...)
	return key
}

// VindicationRecordPrefixForFact returns the prefix for iterating all records for a fact.
func VindicationRecordPrefixForFact(factId string) []byte {
	key := append([]byte{}, VindicationRecordPrefix...)
	key = append(key, []byte(factId)...)
	key = append(key, '/')
	return key
}
```

**Step 3: Commit**

```bash
git add x/knowledge/types/keys.go
git commit -m "feat(knowledge): add vindication store prefixes 0x50-0x51 (R28-1)"
```

---

### Task 3: Add Vindication Parameters

**Files:**
- Modify: `proto/zerone/knowledge/v1/genesis.proto` (add 4 fields after field 102)
- Modify: `x/knowledge/types/genesis.go:147-151` (defaults) and `:453-462` (validation)

**Step 1: Add proto fields**

After `diversity_conformity_alert_epochs = 102;` in `genesis.proto`, add:

```proto
  // ─── Retroactive vindication (R28-1) ──────────────────────────────
  bool   vindication_refund_enabled  = 103; // Master switch for vindication escrow (default: true)
  uint64 vindication_bonus_bps       = 104; // % of majority slash pool as bonus to vindicated minority (default: 2000 = 20%)
  uint64 vindication_slash_bps       = 105; // Slash rate for majority on disproven fact (default: 500 = 5%)
  uint64 vindication_window_blocks   = 106; // How long escrowed entries are eligible (default: 100000)
```

**Step 2: Regenerate protobuf**

Run: `make proto-gen` (or whatever the project uses)

If protobuf generation isn't available, manually add the fields to `genesis.pb.go` following the existing pattern. The generated struct fields will be:

```go
VindicationRefundEnabled bool   `protobuf:"varint,103,opt,name=vindication_refund_enabled,json=vindicationRefundEnabled,proto3" json:"vindication_refund_enabled,omitempty"`
VindicationBonusBps      uint64 `protobuf:"varint,104,opt,name=vindication_bonus_bps,json=vindicationBonusBps,proto3" json:"vindication_bonus_bps,omitempty"`
VindicationSlashBps      uint64 `protobuf:"varint,105,opt,name=vindication_slash_bps,json=vindicationSlashBps,proto3" json:"vindication_slash_bps,omitempty"`
VindicationWindowBlocks  uint64 `protobuf:"varint,106,opt,name=vindication_window_blocks,json=vindicationWindowBlocks,proto3" json:"vindication_window_blocks,omitempty"`
```

**Step 3: Add defaults in `genesis.go`**

In `DefaultParams()`, after `DiversityConformityAlertEpochs: 3,` (line 149), add:

```go
		// ─── Retroactive vindication (R28-1) ─────────────────────────────
		VindicationRefundEnabled: true,
		VindicationBonusBps:      2_000,    // 20% of majority slash pool as bonus
		VindicationSlashBps:      500,      // 5% slash rate for majority on disproven fact
		VindicationWindowBlocks:  100_000,  // ~3 days at 2.5s blocks
```

**Step 4: Add validation in `genesis.go`**

In `Validate()`, after the diversity params validation block (line 459), add:

```go
	// ─── Vindication params ──────────────────────────────────────────
	if p.VindicationBonusBps > 10_000 {
		return fmt.Errorf("vindication_bonus_bps must be <= 10,000 (100%%)")
	}
	if p.VindicationSlashBps > 1_000_000 {
		return fmt.Errorf("vindication_slash_bps must be <= 1,000,000")
	}
	if p.VindicationRefundEnabled && p.VindicationWindowBlocks == 0 {
		return fmt.Errorf("vindication_window_blocks must be > 0 when vindication is enabled")
	}
```

**Step 5: Commit**

```bash
git add proto/zerone/knowledge/v1/genesis.proto x/knowledge/types/genesis.go x/knowledge/types/genesis.pb.go
git commit -m "feat(knowledge): add vindication params (R28-1)"
```

---

### Task 4: Add SlashValidatorToModule to StakingKeeper Interface

**Files:**
- Modify: `x/knowledge/types/expected_keepers.go:24-36` (interface)
- Modify: `x/staking/keeper/knowledge_adapters.go:76-99` (adapter)
- Modify: `x/knowledge/keeper/helpers_test.go:150-153` (mock)
- Modify: `x/knowledge/keeper/abci_test.go:101` (mock)

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/helpers_test.go` after the existing `SlashValidator` method (line 153):

```go
func (sk *trackingStakingKeeper) SlashValidatorToModule(_ context.Context, addr string, slashBps uint64, destModule string) (sdkmath.Int, error) {
	sk.slashes = append(sk.slashes, slashRecord{Validator: addr, SlashBps: slashBps})
	// Mock: return a computed slash amount based on validator stake
	stake := uint64(100_000)
	if v, ok := sk.validators[addr]; ok {
		stake = v.Stake
	}
	slashAmt := stake * slashBps / 1_000_000
	return sdkmath.NewIntFromUint64(slashAmt), nil
}
```

Make sure `sdkmath` is imported (check for `cosmossdk.io/math` in imports).

**Step 2: Add to interface**

In `x/knowledge/types/expected_keepers.go`, add to the `StakingKeeper` interface (after line 35):

```go
	// SlashValidatorToModule slashes a validator and routes tokens to a specific module account.
	// Returns the actual slashed amount.
	SlashValidatorToModule(ctx context.Context, addr string, slashBps uint64, destModule string) (sdkmath.Int, error)
```

Add import if needed: `sdkmath "cosmossdk.io/math"` (check if already imported).

**Step 3: Implement the adapter**

In `x/staking/keeper/knowledge_adapters.go`, after the `SlashValidator` method (line 99), add:

```go
// SlashValidatorToModule slashes a validator and routes slashed tokens to a specified module account.
// Returns the actual slashed amount (after escalation and SSI adjustments).
func (a *StakingKeeperAdapter) SlashValidatorToModule(ctx context.Context, addr string, slashBps uint64, destModule string) (sdkmath.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	val, found := a.k.GetValidator(sdkCtx, addr)
	if !found {
		return sdkmath.ZeroInt(), fmt.Errorf("validator %s not found", addr)
	}

	totalStake, _ := new(big.Int).SetString(val.TotalStake, 10)
	if totalStake == nil || totalStake.Sign() <= 0 {
		return sdkmath.ZeroInt(), nil
	}

	amount := new(big.Int).Mul(totalStake, new(big.Int).SetUint64(slashBps))
	amount.Div(amount, new(big.Int).SetUint64(1_000_000))

	if amount.Sign() <= 0 {
		return sdkmath.ZeroInt(), nil
	}

	// Use the keeper's SlashValidatorToModule which routes to a custom destination
	slashed := a.k.SlashValidatorToModule(sdkCtx, addr, amount, destModule, "knowledge_verification")
	return sdkmath.NewIntFromBigInt(slashed), nil
}
```

**Step 4: Add `SlashValidatorToModule` to the staking keeper**

In `x/staking/keeper/keeper.go`, add a new method after `SlashValidator` (around line 600). This is a copy of `SlashValidator` with parameterized destination:

```go
// SlashValidatorToModule slashes a validator and routes tokens to the specified module account.
// Returns the actual slashed amount.
func (k Keeper) SlashValidatorToModule(ctx sdk.Context, validatorAddr string, amount *big.Int, destModule string, reason string) *big.Int {
	val, found := k.GetValidator(ctx, validatorAddr)
	if !found {
		return new(big.Int)
	}
	params := k.GetParams(ctx)

	if params.MaxSlashesPerEpoch > 0 && val.SlashesThisEpoch >= params.MaxSlashesPerEpoch {
		return new(big.Int)
	}

	// Progressive escalation
	escalationFactor := new(big.Int).SetUint64(types.BPSScale + val.SlashCount*params.SlashEscalationBps)
	adjustedAmount := new(big.Int).Mul(amount, escalationFactor)
	adjustedAmount.Div(adjustedAmount, new(big.Int).SetUint64(types.BPSScale))

	// Autopoiesis SSI multiplier
	if k.autopoiesisKeeper != nil {
		multiplier := k.autopoiesisKeeper.GetMultiplier(ctx, "ssi")
		if multiplier != 0 && multiplier != types.BPSScale {
			adjustedAmount.Mul(adjustedAmount, new(big.Int).SetUint64(multiplier))
			adjustedAmount.Div(adjustedAmount, new(big.Int).SetUint64(types.BPSScale))
		}
	}

	// Slash from own stake first, delegated absorbs overflow
	selfStake, _ := new(big.Int).SetString(val.SelfDelegation, 10)
	if selfStake == nil {
		selfStake = new(big.Int)
	}
	slashInt := new(big.Int).Set(adjustedAmount)
	if slashInt.Cmp(selfStake) > 0 {
		slashInt.Set(selfStake)
		overflow := new(big.Int).Sub(adjustedAmount, selfStake)
		delegated, _ := new(big.Int).SetString(val.DelegatedStake, 10)
		if delegated == nil {
			delegated = new(big.Int)
		}
		if overflow.Cmp(delegated) > 0 {
			overflow.Set(delegated)
		}
		delegated.Sub(delegated, overflow)
		val.DelegatedStake = delegated.String()
		slashInt.Add(slashInt, overflow)
	}

	// Route slashed tokens to specified module
	if slashInt.Sign() > 0 {
		slashCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(slashInt)))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, destModule, slashCoins); err != nil {
			return new(big.Int)
		}

		selfStake.Sub(selfStake, new(big.Int).Sub(slashInt, new(big.Int)))
		if selfStake.Sign() < 0 {
			selfStake.SetInt64(0)
		}
	}

	if slashInt.Sign() > 0 {
		val.SlashCount++
		val.SlashesThisEpoch++
		val.LastSlashHeight = uint64(ctx.BlockHeight())
	}

	// Update stake values
	selfStakeNew, _ := new(big.Int).SetString(val.SelfDelegation, 10)
	if selfStakeNew == nil {
		selfStakeNew = new(big.Int)
	}
	selfStakeNew.Sub(selfStakeNew, slashInt)
	if selfStakeNew.Sign() < 0 {
		selfStakeNew.SetInt64(0)
	}
	val.SelfDelegation = selfStakeNew.String()

	delegated, _ := new(big.Int).SetString(val.DelegatedStake, 10)
	if delegated == nil {
		delegated = new(big.Int)
	}
	total := new(big.Int).Add(selfStakeNew, delegated)
	val.TotalStake = total.String()

	if val.ReputationScore >= params.ReputationSlashDelta {
		val.ReputationScore -= params.ReputationSlashDelta
	} else {
		val.ReputationScore = 0
	}

	k.SetValidator(ctx, val)
	return slashInt
}
```

**Step 5: Update abci_test.go mock**

In `x/knowledge/keeper/abci_test.go`, add to `mockStakingKeeper`:

```go
func (sk *mockStakingKeeper) SlashValidatorToModule(_ context.Context, _ string, _ uint64, _ string) (sdkmath.Int, error) {
	return sdkmath.ZeroInt(), nil
}
```

**Step 6: Verify compilation**

Run: `go build ./x/knowledge/... ./x/staking/...`
Expected: No compilation errors

**Step 7: Commit**

```bash
git add x/knowledge/types/expected_keepers.go x/staking/keeper/knowledge_adapters.go x/staking/keeper/keeper.go x/knowledge/keeper/helpers_test.go x/knowledge/keeper/abci_test.go
git commit -m "feat(staking): add SlashValidatorToModule for vindication escrow routing (R28-1)"
```

---

### Task 5: Register vindication_escrow Module Account

**Files:**
- Modify: `app/app.go:294-312` (maccPerms map)

**Step 1: Add module account**

In `maccPerms` map (around line 311), after the `knowledge_bootstrap_fund` entry, add:

```go
		zeroneknowledgetypes.VindicationEscrowModuleName: nil, // vindication_escrow: holds slashed minority tokens until vindication or expiry
```

The `nil` permission means receive-only — tokens can be sent to it but it can't mint/burn. Sending FROM it requires `SendCoinsFromModuleToModule` or `SendCoinsFromModuleToAccount` calls from keeper code.

**Step 2: Verify compilation**

Run: `go build ./app/...`
Expected: No errors

**Step 3: Commit**

```bash
git add app/app.go
git commit -m "feat(app): register vindication_escrow module account (R28-1)"
```

---

### Task 6: Vindication Store Methods

**Files:**
- Create: `x/knowledge/keeper/vindication.go`
- Create: `x/knowledge/keeper/vindication_test.go`

**Step 1: Write the failing tests**

Create `x/knowledge/keeper/vindication_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestVindicationPending_SetGetDelete(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId := "fact-abc123"
	entries := []types.VindicationEntry{
		{Verifier: "zerone1abc", Vote: "reject", SlashAmount: "5000", SlashBps: 50000, RoundId: "round-1", FactId: factId, Height: 100},
		{Verifier: "zerone1def", Vote: "reject", SlashAmount: "3000", SlashBps: 50000, RoundId: "round-1", FactId: factId, Height: 100},
	}

	// Initially empty
	got := k.GetVindicationPending(ctx, factId)
	require.Empty(t, got)

	// Set
	k.SetVindicationPending(ctx, factId, entries)

	// Get
	got = k.GetVindicationPending(ctx, factId)
	require.Len(t, got, 2)
	require.Equal(t, "zerone1abc", got[0].Verifier)
	require.Equal(t, "5000", got[0].SlashAmount)

	// Delete
	k.DeleteVindicationPending(ctx, factId)
	got = k.GetVindicationPending(ctx, factId)
	require.Empty(t, got)
}

func TestVindicationRecord_SetGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId := "fact-xyz789"
	record := types.VindicationRecord{
		Verifier:     "zerone1abc",
		FactId:       factId,
		RefundAmount: "5000",
		BonusAmount:  "1000",
		VindicatedAt: 500,
		DisprovenBy:  "fact-new",
		RoundId:      "round-1",
	}

	// Set
	k.SetVindicationRecord(ctx, factId, record)

	// Get
	got, found := k.GetVindicationRecord(ctx, factId, "zerone1abc")
	require.True(t, found)
	require.Equal(t, "5000", got.RefundAmount)
	require.Equal(t, "1000", got.BonusAmount)

	// Get non-existent
	_, found = k.GetVindicationRecord(ctx, factId, "zerone1zzz")
	require.False(t, found)
}

func TestVindicationPending_GetAllPending(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Store entries for two different facts
	k.SetVindicationPending(ctx, "fact-1", []types.VindicationEntry{
		{Verifier: "v1", Height: 100, FactId: "fact-1"},
	})
	k.SetVindicationPending(ctx, "fact-2", []types.VindicationEntry{
		{Verifier: "v2", Height: 200, FactId: "fact-2"},
	})

	all := k.GetAllVindicationPending(ctx)
	require.Len(t, all, 2)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/knowledge/keeper/ -run TestVindication -v`
Expected: FAIL — methods don't exist yet

**Step 3: Implement store methods**

Create `x/knowledge/keeper/vindication.go`:

```go
package keeper

import (
	"context"
	"encoding/json"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── VindicationPending store methods ───────────────────────────────────────────

// SetVindicationPending stores pending vindication entries for a fact.
func (k Keeper) SetVindicationPending(ctx context.Context, factId string, entries []types.VindicationEntry) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(entries)
	if err != nil {
		return
	}
	_ = store.Set(types.VindicationPendingKey(factId), bz)
}

// GetVindicationPending returns pending vindication entries for a fact.
func (k Keeper) GetVindicationPending(ctx context.Context, factId string) []types.VindicationEntry {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.VindicationPendingKey(factId))
	if err != nil || bz == nil {
		return nil
	}
	var entries []types.VindicationEntry
	if err := json.Unmarshal(bz, &entries); err != nil {
		return nil
	}
	return entries
}

// DeleteVindicationPending removes pending vindication entries for a fact.
func (k Keeper) DeleteVindicationPending(ctx context.Context, factId string) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.VindicationPendingKey(factId))
}

// GetAllVindicationPending returns all pending vindication entries across all facts.
// Used for pruning iteration.
func (k Keeper) GetAllVindicationPending(ctx context.Context) map[string][]types.VindicationEntry {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	storeAdapter := sdkCtx.KVStore(k.storeService.(storetypes.StoreKey))
	prefixStore := prefix.NewStore(storeAdapter, types.VindicationPendingPrefix)

	result := make(map[string][]types.VindicationEntry)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		factId := string(iter.Key())
		var entries []types.VindicationEntry
		if err := json.Unmarshal(iter.Value(), &entries); err != nil {
			continue
		}
		result[factId] = entries
	}
	return result
}

// ─── VindicationRecord store methods ────────────────────────────────────────────

// SetVindicationRecord stores an executed vindication record.
func (k Keeper) SetVindicationRecord(ctx context.Context, factId string, record types.VindicationRecord) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return
	}
	_ = store.Set(types.VindicationRecordKey(factId, record.Verifier), bz)
}

// GetVindicationRecord returns a vindication record for a specific fact and verifier.
func (k Keeper) GetVindicationRecord(ctx context.Context, factId, verifier string) (types.VindicationRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.VindicationRecordKey(factId, verifier))
	if err != nil || bz == nil {
		return types.VindicationRecord{}, false
	}
	var record types.VindicationRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.VindicationRecord{}, false
	}
	return record, true
}

// GetVindicationRecordsForFact returns all vindication records for a fact.
func (k Keeper) GetVindicationRecordsForFact(ctx context.Context, factId string) []types.VindicationRecord {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	storeAdapter := sdkCtx.KVStore(k.storeService.(storetypes.StoreKey))
	prefixStore := prefix.NewStore(storeAdapter, types.VindicationRecordPrefixForFact(factId))

	var records []types.VindicationRecord
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var record types.VindicationRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records
}
```

**Important:** The `GetAllVindicationPending` method uses `prefix.NewStore` for iteration. Check how other iteration is done in this codebase — some keepers use `k.storeService.OpenKVStore(ctx)` with a manual prefix scan, others use the `prefix.NewStore` pattern. Match the existing pattern. Look at how `x/knowledge/keeper/state.go` iterates (e.g., `IterateFacts`).

**Step 4: Run tests to verify they pass**

Run: `go test ./x/knowledge/keeper/ -run TestVindication -v`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/vindication.go x/knowledge/keeper/vindication_test.go
git commit -m "feat(knowledge): add vindication store methods (R28-1)"
```

---

### Task 7: Add VindicationEligible Flag to VerifierSlash

**Files:**
- Modify: `x/knowledge/keeper/confidence.go:25-29` (VerifierSlash struct)
- Modify: `x/knowledge/keeper/confidence.go:185-191` (wrong vote slash)

**Step 1: Add flag to VerifierSlash**

In `confidence.go`, modify the `VerifierSlash` struct (line 26):

```go
type VerifierSlash struct {
	Verifier            string
	SlashBps            uint64
	VindicationEligible bool // true for wrong-vote slashes, false for missed-reveal/equivocation
}
```

**Step 2: Mark wrong-vote slashes as vindication-eligible**

In `calculateRewardsAndSlashes` (line 187), change:

```go
		// Incorrect vote — slash
		result.Slashes = append(result.Slashes, VerifierSlash{
			Verifier:            commit.Verifier,
			SlashBps:            params.WrongVerificationSlashBps,
			VindicationEligible: true,
		})
```

Missed-reveal slashes (line 159) and any equivocation slashes remain `VindicationEligible: false` (zero value).

**Step 3: Verify compilation**

Run: `go build ./x/knowledge/...`
Expected: No errors

**Step 4: Commit**

```bash
git add x/knowledge/keeper/confidence.go
git commit -m "feat(knowledge): tag wrong-vote slashes as vindication-eligible (R28-1)"
```

---

### Task 8: Route Minority Slashes to Escrow + Record VindicationPending

**Files:**
- Modify: `x/knowledge/keeper/rounds.go:104-108` (slash loop in CompleteRound)

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/vindication_test.go`:

```go
func TestCompleteRound_EscrowsMinoritySlash(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a claim and round where minority voter gets slashed
	claim := &types.Claim{
		Id:          "claim-1",
		FactContent: "test claim content that is long enough",
		Domain:      "science",
		Category:    "empirical",
		Submitter:   "zerone1submitter",
		Stake:       "1000000",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	_ = k.SetClaim(ctx, claim)

	round := &types.VerificationRound{
		Id:      "round-1",
		ClaimId: "claim-1",
		Phase:   types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		Commits: []*types.CommitEntry{
			{Verifier: "zerone1val1"},
			{Verifier: "zerone1val2"},
			{Verifier: "zerone1val3"}, // minority
		},
		Reveals: []*types.RevealEntry{
			{Verifier: "zerone1val1", Vote: "accept"},
			{Verifier: "zerone1val2", Vote: "accept"},
			{Verifier: "zerone1val3", Vote: "reject"}, // minority voter
		},
	}
	_ = k.SetVerificationRound(ctx, round)

	// Complete round — majority accepts, minority (val3) gets slashed
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800_000,
		Slashes: []keeper.VerifierSlash{
			{Verifier: "zerone1val3", SlashBps: 50_000, VindicationEligible: true},
		},
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// Verify: fact was created (round accepted)
	// Verify: VindicationPending entry exists for the fact
	// Note: factId is generated by createFactFromClaim, so we need to find it
	// For now, check that pending entries exist by iterating
	allPending := k.GetAllVindicationPending(ctx)
	require.NotEmpty(t, allPending, "should have pending vindication entries")

	// Find the entry
	for _, entries := range allPending {
		require.Len(t, entries, 1)
		require.Equal(t, "zerone1val3", entries[0].Verifier)
		require.Equal(t, "reject", entries[0].Vote)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestCompleteRound_EscrowsMinoritySlash -v`
Expected: FAIL — escrow not implemented yet

**Step 3: Modify CompleteRound slash loop**

In `rounds.go`, replace the slash loop (lines 104-108):

```go
	// Execute slashes — route vindication-eligible slashes to escrow
	params, _ := k.GetParams(ctx)
	var vindicationEntries []types.VindicationEntry

	for _, slash := range result.Slashes {
		if k.stakingKeeper == nil {
			continue
		}
		if slash.VindicationEligible && params.VindicationRefundEnabled {
			// Route to escrow instead of development_fund
			slashedAmt, err := k.stakingKeeper.SlashValidatorToModule(ctx, slash.Verifier, slash.SlashBps, types.VindicationEscrowModuleName)
			if err == nil && slashedAmt.IsPositive() {
				// Find the vote for this verifier from reveals
				vote := ""
				for _, reveal := range round.Reveals {
					if reveal.Verifier == slash.Verifier {
						vote = reveal.Vote
						break
					}
				}
				vindicationEntries = append(vindicationEntries, types.VindicationEntry{
					Verifier:    slash.Verifier,
					Vote:        vote,
					SlashAmount: slashedAmt.String(),
					SlashBps:    slash.SlashBps,
					RoundId:     round.Id,
					FactId:      "", // filled below after we know the factId
					Height:      height,
				})
			}
		} else {
			// Standard slash — goes to development_fund
			_ = k.stakingKeeper.SlashValidator(ctx, slash.Verifier, slash.SlashBps)
		}
	}
```

Then, after the fact is created (we need the factId), store the vindication entries. The factId is determined in `createFactFromClaim`. We need to capture it.

**Important implementation detail:** The factId is generated inside `createFactFromClaim`. We need to either:
- (a) Return the factId from `createFactFromClaim`, or
- (b) Look up the claim's associated fact after completion

Option (a) is cleaner. Modify `createFactFromClaim` to return `(string, error)` instead of `error`:

In `rounds.go`, find `createFactFromClaim` and change its signature. Then in the ACCEPT case:

```go
	case types.Verdict_VERDICT_ACCEPT:
		factId, err := k.createFactFromClaim(ctx, claim, round, result.Confidence)
		if err != nil {
			return err
		}
		claim.Status = types.ClaimStatus_CLAIM_STATUS_ACCEPTED

		// Store VindicationPending entries if any minority voters were slashed
		if len(vindicationEntries) > 0 {
			for i := range vindicationEntries {
				vindicationEntries[i].FactId = factId
			}
			k.SetVindicationPending(ctx, factId, vindicationEntries)
		}
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/knowledge/keeper/ -run TestCompleteRound_EscrowsMinoritySlash -v`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/rounds.go x/knowledge/keeper/vindication_test.go
git commit -m "feat(knowledge): route minority slashes to escrow and record VindicationPending (R28-1)"
```

---

### Task 9: DISPROVEN Transition on Challenge Success

**Files:**
- Modify: `x/knowledge/keeper/rounds.go:71-78` (ACCEPT case in CompleteRound)

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/vindication_test.go`:

```go
func TestChallengeSuccess_TransitionsToDisproven(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create an existing verified fact
	originalFact := &types.Fact{
		Id:         "original-fact",
		Content:    "water boils at 50C",
		Domain:     "science",
		Status:     types.FactStatus_FACT_STATUS_CHALLENGED,
		Confidence: 800_000,
	}
	_ = k.SetFact(ctx, originalFact)

	// Create a challenge claim that references the original fact
	challengeClaim := &types.Claim{
		Id:                "challenge-claim-1",
		FactContent:       "water boils at 100C at 1atm",
		Domain:            "science",
		Submitter:         "zerone1challenger",
		Status:            types.ClaimStatus_CLAIM_STATUS_PENDING,
		ProvisionalFactId: "original-fact", // link to challenged fact
	}
	_ = k.SetClaim(ctx, challengeClaim)

	round := &types.VerificationRound{
		Id:      "challenge-round-1",
		ClaimId: "challenge-claim-1",
		Phase:   types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		Commits: []*types.CommitEntry{{Verifier: "v1"}, {Verifier: "v2"}, {Verifier: "v3"}},
		Reveals: []*types.RevealEntry{
			{Verifier: "v1", Vote: "accept"},
			{Verifier: "v2", Vote: "accept"},
			{Verifier: "v3", Vote: "accept"},
		},
	}
	_ = k.SetVerificationRound(ctx, round)

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 900_000,
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// Original fact should now be DISPROVEN
	updatedFact, found := k.GetFact(ctx, "original-fact")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_DISPROVEN, updatedFact.Status)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestChallengeSuccess_TransitionsToDisproven -v`
Expected: FAIL — DISPROVEN transition not implemented

**Step 3: Implement DISPROVEN transition**

In the ACCEPT case of `CompleteRound` (after fact creation), add challenge-disproven logic:

```go
	case types.Verdict_VERDICT_ACCEPT:
		factId, err := k.createFactFromClaim(ctx, claim, round, result.Confidence)
		if err != nil {
			return err
		}
		claim.Status = types.ClaimStatus_CLAIM_STATUS_ACCEPTED

		// If this was a challenge claim, check if the original fact should be disproven
		if claim.ProvisionalFactId != "" {
			k.handleChallengeDisproven(ctx, claim, factId)
		}

		// Store VindicationPending entries (from Task 8)
		if len(vindicationEntries) > 0 {
			for i := range vindicationEntries {
				vindicationEntries[i].FactId = factId
			}
			k.SetVindicationPending(ctx, factId, vindicationEntries)
		}
```

Add the new method in `vindication.go`:

```go
// handleChallengeDisproven transitions the challenged fact to DISPROVEN
// when a challenge claim is accepted. Triggers vindication if pending entries exist.
func (k Keeper) handleChallengeDisproven(ctx context.Context, challengeClaim *types.Claim, newFactId string) {
	if challengeClaim.ProvisionalFactId == "" {
		return
	}

	originalFact, found := k.GetFact(ctx, challengeClaim.ProvisionalFactId)
	if !found {
		return
	}

	// Contradiction check: same domain + explicit challenge link (ProvisionalFactId)
	if originalFact.Domain != challengeClaim.Domain {
		return
	}

	// Transition to DISPROVEN
	originalFact.Status = types.FactStatus_FACT_STATUS_DISPROVEN
	_ = k.SetFact(ctx, originalFact)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.fact_disproven",
		sdk.NewAttribute("fact_id", originalFact.Id),
		sdk.NewAttribute("disproven_by", newFactId),
		sdk.NewAttribute("challenge_claim_id", challengeClaim.Id),
	))

	// Trigger vindication for original fact's minority voters
	k.executeVindication(ctx, originalFact.Id, newFactId)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/knowledge/keeper/ -run TestChallengeSuccess_TransitionsToDisproven -v`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/rounds.go x/knowledge/keeper/vindication.go x/knowledge/keeper/vindication_test.go
git commit -m "feat(knowledge): transition challenged fact to DISPROVEN on challenge success (R28-1)"
```

---

### Task 10: Execute Vindication Logic

**Files:**
- Modify: `x/knowledge/keeper/vindication.go` (add `executeVindication`)

**Step 1: Write the failing test**

Add to `vindication_test.go`:

```go
func TestExecuteVindication_RefundsAndBonuses(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Setup: create pending vindication entries (simulating minority was slashed)
	factId := "disproven-fact"
	k.SetVindicationPending(ctx, factId, []types.VindicationEntry{
		{Verifier: "zerone1minority1", Vote: "reject", SlashAmount: "5000", SlashBps: 50000, RoundId: "round-1", FactId: factId, Height: 100},
		{Verifier: "zerone1minority2", Vote: "reject", SlashAmount: "3000", SlashBps: 50000, RoundId: "round-1", FactId: factId, Height: 100},
	})

	// Setup: we also need to know who the majority voters were
	// Store the original round with its reveals
	originalRound := &types.VerificationRound{
		Id:      "round-1",
		ClaimId: "original-claim",
		Commits: []*types.CommitEntry{
			{Verifier: "zerone1majority1"},
			{Verifier: "zerone1majority2"},
			{Verifier: "zerone1minority1"},
			{Verifier: "zerone1minority2"},
		},
		Reveals: []*types.RevealEntry{
			{Verifier: "zerone1majority1", Vote: "accept"},
			{Verifier: "zerone1majority2", Vote: "accept"},
			{Verifier: "zerone1minority1", Vote: "reject"},
			{Verifier: "zerone1minority2", Vote: "reject"},
		},
		Verdict: types.Verdict_VERDICT_ACCEPT,
	}
	_ = k.SetVerificationRound(ctx, originalRound)

	// Execute vindication
	k.ExecuteVindication(ctx, factId, "new-fact-id")

	// Verify: pending entries deleted
	pending := k.GetVindicationPending(ctx, factId)
	require.Empty(t, pending)

	// Verify: vindication records created
	records := k.GetVindicationRecordsForFact(ctx, factId)
	require.Len(t, records, 2)

	// Verify: majority voters were slashed (check tracking staking keeper)
	// Verify: records have correct refund amounts
	for _, rec := range records {
		require.NotEmpty(t, rec.RefundAmount)
		require.Equal(t, factId, rec.FactId)
		require.Equal(t, "new-fact-id", rec.DisprovenBy)
	}
}

func TestExecuteVindication_NoPendingNoOp(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// No pending entries — should be a no-op
	k.ExecuteVindication(ctx, "nonexistent-fact", "new-fact")

	records := k.GetVindicationRecordsForFact(ctx, "nonexistent-fact")
	require.Empty(t, records)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestExecuteVindication -v`
Expected: FAIL — `ExecuteVindication` not implemented

**Step 3: Implement ExecuteVindication**

Add to `x/knowledge/keeper/vindication.go`:

```go
// executeVindication refunds slashed minority voters and distributes bonuses
// when a fact is disproven. Called from handleChallengeDisproven.
func (k Keeper) executeVindication(ctx context.Context, factId, disprovenBy string) {
	k.ExecuteVindication(ctx, factId, disprovenBy)
}

// ExecuteVindication is the public entry point for vindication execution.
func (k Keeper) ExecuteVindication(ctx context.Context, factId, disprovenBy string) {
	pending := k.GetVindicationPending(ctx, factId)
	if len(pending) == 0 {
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	params, _ := k.GetParams(ctx)

	// Find the original round to identify majority voters
	// All pending entries share the same roundId
	roundId := pending[0].RoundId
	round, found := k.GetVerificationRound(ctx, roundId)
	if !found {
		return
	}

	// Build set of minority verifiers (the ones being vindicated)
	minoritySet := make(map[string]bool)
	for _, entry := range pending {
		minoritySet[entry.Verifier] = true
	}

	// Identify majority voters: revealed voters who are NOT in the minority set
	var majorityVoters []string
	for _, reveal := range round.Reveals {
		if !minoritySet[reveal.Verifier] {
			majorityVoters = append(majorityVoters, reveal.Verifier)
		}
	}

	// Step 1: Slash majority voters at VindicationSlashBps
	totalMajoritySlash := sdkmath.ZeroInt()
	if k.stakingKeeper != nil && params.VindicationSlashBps > 0 {
		for _, voter := range majorityVoters {
			slashed, err := k.stakingKeeper.SlashValidatorToModule(ctx, voter, params.VindicationSlashBps, types.ModuleName)
			if err == nil {
				totalMajoritySlash = totalMajoritySlash.Add(slashed)
			}
		}
	}

	// Step 2: Calculate bonus pool = totalMajoritySlash * VindicationBonusBps / 10000
	bonusPool := sdkmath.ZeroInt()
	remainder := sdkmath.ZeroInt()
	if totalMajoritySlash.IsPositive() && params.VindicationBonusBps > 0 {
		bonusPool = totalMajoritySlash.MulRaw(int64(params.VindicationBonusBps)).QuoRaw(10_000)
		remainder = totalMajoritySlash.Sub(bonusPool)
	} else {
		remainder = totalMajoritySlash
	}

	// Step 3: Send remainder to protocol treasury
	if remainder.IsPositive() && k.bankKeeper != nil {
		treasuryCoins := sdk.NewCoins(sdk.NewCoin("uzrn", remainder))
		_ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "development_fund", treasuryCoins)
	}

	// Step 4: Calculate total minority slash for proportional bonus distribution
	totalMinoritySlash := new(big.Int)
	for _, entry := range pending {
		amt, _ := new(big.Int).SetString(entry.SlashAmount, 10)
		if amt != nil {
			totalMinoritySlash.Add(totalMinoritySlash, amt)
		}
	}

	// Step 5: Refund each minority voter + distribute proportional bonus
	for _, entry := range pending {
		refundAmt, _ := new(big.Int).SetString(entry.SlashAmount, 10)
		if refundAmt == nil || refundAmt.Sign() <= 0 {
			continue
		}

		// Refund from escrow to validator
		if k.bankKeeper != nil {
			addr, err := sdk.AccAddressFromBech32(entry.Verifier)
			if err == nil {
				refundCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(refundAmt)))
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.VindicationEscrowModuleName, addr, refundCoins)
			}
		}

		// Calculate proportional bonus
		bonusAmt := new(big.Int)
		if bonusPool.IsPositive() && totalMinoritySlash.Sign() > 0 {
			// bonus = bonusPool * (entrySlash / totalMinoritySlash)
			bonusAmt.Mul(bonusPool.BigInt(), refundAmt)
			bonusAmt.Div(bonusAmt, totalMinoritySlash)

			if bonusAmt.Sign() > 0 && k.bankKeeper != nil {
				addr, err := sdk.AccAddressFromBech32(entry.Verifier)
				if err == nil {
					bonusCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(bonusAmt)))
					_ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.ModuleName, bonusCoins) // no-op, just for accounting
					_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, addr, bonusCoins)
				}
			}
		}

		// Record vindication
		k.SetVindicationRecord(ctx, factId, types.VindicationRecord{
			Verifier:     entry.Verifier,
			FactId:       factId,
			RefundAmount: refundAmt.String(),
			BonusAmount:  bonusAmt.String(),
			VindicatedAt: height,
			DisprovenBy:  disprovenBy,
			RoundId:      entry.RoundId,
		})
	}

	// Step 6: Delete pending entries
	k.DeleteVindicationPending(ctx, factId)

	// Step 7: Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.vindication_executed",
		sdk.NewAttribute("fact_id", factId),
		sdk.NewAttribute("disproven_by", disprovenBy),
		sdk.NewAttribute("minority_count", fmt.Sprintf("%d", len(pending))),
		sdk.NewAttribute("majority_slashed", totalMajoritySlash.String()),
		sdk.NewAttribute("bonus_pool", bonusPool.String()),
	))
}
```

**Note:** Add imports for `"fmt"`, `"math/big"`, and `sdkmath "cosmossdk.io/math"` to `vindication.go`.

**Step 4: Run test to verify it passes**

Run: `go test ./x/knowledge/keeper/ -run TestExecuteVindication -v`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/vindication.go x/knowledge/keeper/vindication_test.go
git commit -m "feat(knowledge): implement ExecuteVindication with refunds and bonuses (R28-1)"
```

---

### Task 11: Pruning in BeginBlocker

**Files:**
- Modify: `x/knowledge/keeper/vindication.go` (add `PruneExpiredVindications`)
- Modify: `x/knowledge/keeper/phases.go:12-55` (add pruning call)

**Step 1: Write the failing test**

Add to `vindication_test.go`:

```go
func TestPruneExpiredVindications(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create entries at different heights
	k.SetVindicationPending(ctx, "old-fact", []types.VindicationEntry{
		{Verifier: "v1", Height: 100, FactId: "old-fact", SlashAmount: "5000"},
	})
	k.SetVindicationPending(ctx, "recent-fact", []types.VindicationEntry{
		{Verifier: "v2", Height: 90000, FactId: "recent-fact", SlashAmount: "3000"},
	})

	// Prune at height 100100 with window 100000
	// old-fact (height 100): 100100 - 100 = 100000 → exactly at window, NOT pruned
	// But at 100101: 100101 - 100 = 100001 > 100000 → pruned
	k.PruneExpiredVindications(ctx, 100101, 100000)

	// old-fact should be pruned
	require.Empty(t, k.GetVindicationPending(ctx, "old-fact"))

	// recent-fact should remain (90000 + 100000 = 190000 > 100101)
	require.NotEmpty(t, k.GetVindicationPending(ctx, "recent-fact"))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestPruneExpired -v`
Expected: FAIL

**Step 3: Implement PruneExpiredVindications**

Add to `vindication.go`:

```go
// PruneExpiredVindications removes pending vindication entries older than the window.
// Expired escrowed tokens are sent to protocol treasury.
func (k Keeper) PruneExpiredVindications(ctx context.Context, currentHeight, windowBlocks uint64) {
	allPending := k.GetAllVindicationPending(ctx)

	for factId, entries := range allPending {
		// Check the height of the first entry (all entries in a fact share the same height)
		if len(entries) == 0 {
			continue
		}
		entryHeight := entries[0].Height
		if currentHeight-entryHeight <= windowBlocks {
			continue // still within window
		}

		// Expired: transfer escrowed tokens to treasury
		if k.bankKeeper != nil {
			totalEscrowed := new(big.Int)
			for _, entry := range entries {
				amt, _ := new(big.Int).SetString(entry.SlashAmount, 10)
				if amt != nil {
					totalEscrowed.Add(totalEscrowed, amt)
				}
			}
			if totalEscrowed.Sign() > 0 {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(totalEscrowed)))
				_ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.VindicationEscrowModuleName, "development_fund", coins)
			}
		}

		k.DeleteVindicationPending(ctx, factId)

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.vindication_expired",
			sdk.NewAttribute("fact_id", factId),
			sdk.NewAttribute("entry_count", fmt.Sprintf("%d", len(entries))),
		))
	}
}
```

**Step 4: Add pruning to BeginBlocker**

In `phases.go`, after `k.AdvanceRoundPhases(ctx)` (around line 15) and before the fitness epoch block, add:

```go
	// Prune expired vindication entries every 1000 blocks
	if height > 0 && height%1000 == 0 {
		k.PruneExpiredVindications(ctx, height, params.VindicationWindowBlocks)
	}
```

**Important:** The `params` variable is currently only loaded inside the fitness epoch check. Move the `params, err := k.GetParams(ctx)` call earlier (before the pruning check) so it's available for both:

```go
func (k Keeper) BeginBlocker(ctx context.Context) error {
	if err := k.AdvanceRoundPhases(ctx); err != nil {
		return err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil
	}

	// Prune expired vindication entries every 1000 blocks
	if height > 0 && height%1000 == 0 && params.VindicationRefundEnabled {
		k.PruneExpiredVindications(ctx, height, params.VindicationWindowBlocks)
	}

	// Existing fitness epoch logic...
	if params.FitnessEpochBlocks > 0 && height > 0 && height%params.FitnessEpochBlocks == 0 {
		// ... existing code ...
	}

	return nil
}
```

Check the existing code to see if `params` is already loaded before the fitness block. If so, just add the pruning call before the fitness check.

**Step 5: Run test to verify it passes**

Run: `go test ./x/knowledge/keeper/ -run TestPruneExpired -v`
Expected: PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/vindication.go x/knowledge/keeper/vindication_test.go x/knowledge/keeper/phases.go
git commit -m "feat(knowledge): add vindication pruning in BeginBlocker (R28-1)"
```

---

### Task 12: CLI Query Commands

**Files:**
- Modify: `x/knowledge/client/cli/query.go` (add 3 new commands)

**Step 1: Implement query commands**

Follow the pattern from `NewQueryFactCmd` (line 103-123 in query.go). Add these functions:

```go
func NewQueryVindicationPendingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vindication-pending [fact-id]",
		Short: "Query pending vindication entries for a fact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			// Since vindication uses JSON state (not proto queries), we query via gRPC custom endpoint
			// or print a note that this requires direct state access.
			// For MVP, use a gRPC-free approach: query the KV store directly
			fmt.Fprintf(cmd.OutOrStdout(), "Querying vindication pending for fact: %s\n", args[0])
			req := &types.QueryVindicationPendingRequest{FactId: args[0]}
			resp := &types.QueryVindicationPendingResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/VindicationPending", req, resp); err != nil {
				return fmt.Errorf("failed to query vindication pending: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryVindicationRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vindication-record [fact-id]",
		Short: "Query vindication records for a fact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryVindicationRecordRequest{FactId: args[0]}
			resp := &types.QueryVindicationRecordResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/VindicationRecord", req, resp); err != nil {
				return fmt.Errorf("failed to query vindication record: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 2: Register commands**

In `GetQueryCmd()`, add to the `queryCmd.AddCommand(...)` call:

```go
		NewQueryVindicationPendingCmd(),
		NewQueryVindicationRecordCmd(),
```

**Step 3: Add query proto types**

This requires adding query request/response types to the proto file (`proto/zerone/knowledge/v1/query.proto`) and regenerating. If proto generation isn't available, add the types manually to `query.pb.go` following the existing pattern.

Alternatively, for MVP without proto changes, use a simpler CLI approach that queries the keeper directly (if the CLI has access to the keeper). Check how other non-proto queries work in this codebase.

**Step 4: Verify compilation**

Run: `go build ./x/knowledge/...`
Expected: No errors

**Step 5: Commit**

```bash
git add x/knowledge/client/cli/query.go
git commit -m "feat(knowledge): add vindication CLI query commands (R28-1)"
```

---

### Task 13: Full Integration Test

**Files:**
- Modify: `x/knowledge/keeper/vindication_test.go`

**Step 1: Write the full lifecycle test**

```go
func TestVindication_FullLifecycle(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// === Phase 1: Normal verification — minority gets slashed, tokens escrowed ===

	// Submit a claim and create a round where minority voter disagrees
	// (Use the higher-level helpers if available, or set up state directly)

	// Create claim
	claim := &types.Claim{
		Id:          "lifecycle-claim",
		FactContent: "the earth is flat with sufficient detail",
		Domain:      "science",
		Category:    "empirical",
		Submitter:   "zerone1submitter",
		Stake:       "1000000",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	_ = k.SetClaim(ctx, claim)

	round := &types.VerificationRound{
		Id:      "lifecycle-round",
		ClaimId: "lifecycle-claim",
		Phase:   types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		Commits: []*types.CommitEntry{
			{Verifier: "zerone1majority1"},
			{Verifier: "zerone1majority2"},
			{Verifier: "zerone1minority1"},
		},
		Reveals: []*types.RevealEntry{
			{Verifier: "zerone1majority1", Vote: "accept"},
			{Verifier: "zerone1majority2", Vote: "accept"},
			{Verifier: "zerone1minority1", Vote: "reject"},
		},
	}
	_ = k.SetVerificationRound(ctx, round)

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800_000,
		Slashes: []keeper.VerifierSlash{
			{Verifier: "zerone1minority1", SlashBps: 50_000, VindicationEligible: true},
		},
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// Find the created fact
	allPending := k.GetAllVindicationPending(ctx)
	require.Len(t, allPending, 1, "should have one fact with pending vindication")

	var factId string
	for fid := range allPending {
		factId = fid
	}
	require.NotEmpty(t, factId)

	// Verify pending entry exists
	pending := k.GetVindicationPending(ctx, factId)
	require.Len(t, pending, 1)
	require.Equal(t, "zerone1minority1", pending[0].Verifier)

	// === Phase 2: Challenge succeeds — fact disproven, minority vindicated ===

	challengeClaim := &types.Claim{
		Id:                "challenge-lifecycle",
		FactContent:       "the earth is round with evidence",
		Domain:            "science",
		Submitter:         "zerone1challenger",
		Status:            types.ClaimStatus_CLAIM_STATUS_PENDING,
		ProvisionalFactId: factId,
	}
	_ = k.SetClaim(ctx, challengeClaim)

	// Mark original fact as challenged
	origFact, _ := k.GetFact(ctx, factId)
	origFact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	_ = k.SetFact(ctx, origFact)

	challengeRound := &types.VerificationRound{
		Id:      "challenge-lifecycle-round",
		ClaimId: "challenge-lifecycle",
		Phase:   types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		Commits: []*types.CommitEntry{
			{Verifier: "zerone1v1"},
			{Verifier: "zerone1v2"},
			{Verifier: "zerone1v3"},
		},
		Reveals: []*types.RevealEntry{
			{Verifier: "zerone1v1", Vote: "accept"},
			{Verifier: "zerone1v2", Vote: "accept"},
			{Verifier: "zerone1v3", Vote: "accept"},
		},
	}
	_ = k.SetVerificationRound(ctx, challengeRound)

	challengeResult := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 900_000,
	}

	err = k.CompleteRound(ctx, challengeRound, challengeResult)
	require.NoError(t, err)

	// Verify: original fact is DISPROVEN
	updatedFact, found := k.GetFact(ctx, factId)
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_DISPROVEN, updatedFact.Status)

	// Verify: pending entries cleared
	pending = k.GetVindicationPending(ctx, factId)
	require.Empty(t, pending, "pending should be cleared after vindication")

	// Verify: vindication records created
	records := k.GetVindicationRecordsForFact(ctx, factId)
	require.Len(t, records, 1)
	require.Equal(t, "zerone1minority1", records[0].Verifier)
	require.NotEmpty(t, records[0].RefundAmount)
}

func TestVindication_ChallengeFails_NoVindication(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a fact with pending vindication
	factId := "stable-fact"
	fact := &types.Fact{
		Id:     factId,
		Domain: "science",
		Status: types.FactStatus_FACT_STATUS_CHALLENGED,
	}
	_ = k.SetFact(ctx, fact)

	k.SetVindicationPending(ctx, factId, []types.VindicationEntry{
		{Verifier: "zerone1minority", Vote: "reject", SlashAmount: "5000", RoundId: "orig-round", FactId: factId, Height: 100},
	})

	// Challenge claim that gets REJECTED (fact survives)
	challengeClaim := &types.Claim{
		Id:                "failed-challenge",
		FactContent:       "attempted disproof but insufficient",
		Domain:            "science",
		Submitter:         "zerone1challenger",
		Status:            types.ClaimStatus_CLAIM_STATUS_PENDING,
		ProvisionalFactId: factId,
	}
	_ = k.SetClaim(ctx, challengeClaim)

	origRound := &types.VerificationRound{
		Id:      "orig-round",
		ClaimId: "original-claim",
	}
	_ = k.SetVerificationRound(ctx, origRound)

	challengeRound := &types.VerificationRound{
		Id:      "failed-challenge-round",
		ClaimId: "failed-challenge",
		Phase:   types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		Commits: []*types.CommitEntry{{Verifier: "v1"}, {Verifier: "v2"}},
		Reveals: []*types.RevealEntry{
			{Verifier: "v1", Vote: "reject"},
			{Verifier: "v2", Vote: "reject"},
		},
	}
	_ = k.SetVerificationRound(ctx, challengeRound)

	result := &keeper.VerificationResult{
		Verdict: types.Verdict_VERDICT_REJECT,
	}

	err := k.CompleteRound(ctx, challengeRound, result)
	require.NoError(t, err)

	// Fact should survive (restored to ACTIVE by handleChallengeSurvival)
	updatedFact, _ := k.GetFact(ctx, factId)
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updatedFact.Status)

	// Pending vindication should STILL exist (not triggered)
	pending := k.GetVindicationPending(ctx, factId)
	require.Len(t, pending, 1, "pending should remain — challenge failed, no vindication")
}

func TestVindication_DisabledParam(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Disable vindication
	params, _ := k.GetParams(ctx)
	params.VindicationRefundEnabled = false
	_ = k.SetParams(ctx, params)

	// Create a round with minority slashes
	claim := &types.Claim{
		Id:          "disabled-claim",
		FactContent: "test claim with vindication disabled long enough",
		Domain:      "science",
		Submitter:   "zerone1sub",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	_ = k.SetClaim(ctx, claim)

	round := &types.VerificationRound{
		Id:      "disabled-round",
		ClaimId: "disabled-claim",
		Phase:   types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION,
		Commits: []*types.CommitEntry{{Verifier: "v1"}, {Verifier: "v2"}},
		Reveals: []*types.RevealEntry{
			{Verifier: "v1", Vote: "accept"},
			{Verifier: "v2", Vote: "reject"},
		},
	}
	_ = k.SetVerificationRound(ctx, round)

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800_000,
		Slashes: []keeper.VerifierSlash{
			{Verifier: "v2", SlashBps: 50_000, VindicationEligible: true},
		},
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// No pending entries should exist (vindication disabled)
	allPending := k.GetAllVindicationPending(ctx)
	require.Empty(t, allPending, "no escrow when vindication disabled")
}
```

**Step 2: Run all tests**

Run: `go test ./x/knowledge/keeper/ -run TestVindication -v`
Expected: ALL PASS

**Step 3: Run existing tests to verify no regressions**

Run: `go test ./x/knowledge/... -v -timeout 300s`
Expected: ALL PASS (existing tests unaffected)

**Step 4: Commit**

```bash
git add x/knowledge/keeper/vindication_test.go
git commit -m "test(knowledge): add full vindication lifecycle tests (R28-1)"
```

---

### Task 14: Final Verification

**Step 1: Run all module tests**

Run: `go test ./x/knowledge/... -v -timeout 300s`
Expected: ALL PASS

**Step 2: Run staking module tests (for adapter changes)**

Run: `go test ./x/staking/... -v -timeout 300s`
Expected: ALL PASS

**Step 3: Full build check**

Run: `go build ./...`
Expected: No compilation errors

**Step 4: Commit any remaining fixes**

If any tests fail, fix and commit.

**Step 5: Final commit message**

```bash
git add -A
git commit -m "feat(knowledge): complete retroactive vindication system (R28-1)

Implements escrow-based vindication that refunds minority voters when a
challenge later proves them right. Zero-sum: refunds from escrow, bonuses
from majority slash pool, remainder to treasury.

Key changes:
- VindicationEntry/VindicationRecord types
- SlashValidatorToModule for escrow routing
- DISPROVEN status transition on challenge success
- ExecuteVindication with proportional bonus distribution
- Pruning in BeginBlocker every 1000 blocks
- CLI query commands for pending/records
- Full lifecycle tests"
```
