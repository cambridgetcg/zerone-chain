# R20-6 Agent Demand Signal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add agent demand signal tracking, auto-generated knowledge bounties, and demand-weighted energy income to the knowledge module.

**Architecture:** Agents report query demand via `MsgReportDemand`. Unfulfilled demand accumulates; at epoch boundaries, if a domain/subject pair exceeds the bounty threshold, a `KnowledgeBounty` is auto-created from the protocol treasury. When a new fact matches a bounty's subject, the submitter claims the reward. High-demand subjects boost energy income via a demand multiplier.

**Tech Stack:** Cosmos SDK v0.50, Protobuf, Go 1.24+

---

### Task 1: Proto — DemandSignal & KnowledgeBounty types

**Files:**
- Modify: `proto/zerone/knowledge/v1/types.proto` (after line 271, end of file)

**Step 1: Add DemandSignal and KnowledgeBounty messages**

Append to the end of `proto/zerone/knowledge/v1/types.proto`:

```protobuf
// DemandSignal tracks aggregate query demand for a domain/subject pair.
message DemandSignal {
    string domain               = 1;
    string subject              = 2;  // Normalized query subject
    uint64 query_count          = 3;  // Total queries (lifetime)
    uint64 fulfilled_count      = 4;  // Queries that returned results
    uint64 unfulfilled_count    = 5;  // Queries that returned nothing
    uint64 last_query_block     = 6;
    uint64 epoch_query_count    = 7;  // Queries this epoch (resets)
    uint64 epoch_unfulfilled    = 8;  // Unfulfilled this epoch (resets)
}

// KnowledgeBounty is an auto-generated reward for filling a knowledge gap.
message KnowledgeBounty {
    string id                   = 1;
    string domain               = 2;
    string subject              = 3;
    string reward_amount        = 4;  // uzrn
    uint64 created_at_block     = 5;
    uint64 expires_at_block     = 6;
    bool   claimed              = 7;
    string claimed_by_fact_id   = 8;
    uint64 demand_count         = 9;  // Demand that triggered this bounty
}
```

**Step 2: Regenerate protobuf**

Run: `cd /Users/yournameisai/Desktop/zerone && make proto-gen`

If `make proto-gen` is not available, run the manual protoc commands that match the existing `*.pb.go` generation pattern. Check `Makefile` or `scripts/` for the proto generation target.

**Step 3: Verify generated code compiles**

Run: `go build ./x/knowledge/types/...`
Expected: PASS

**Step 4: Commit**

```bash
git add proto/zerone/knowledge/v1/types.proto x/knowledge/types/*.pb.go
git commit -m "feat(knowledge): add DemandSignal and KnowledgeBounty proto types"
```

---

### Task 2: Proto — Demand params in genesis.proto

**Files:**
- Modify: `proto/zerone/knowledge/v1/genesis.proto` (Params message, after field 76)

**Step 1: Add demand params**

In `genesis.proto`, inside the `Params` message (after line 113, field 76), add:

```protobuf
  // ─── Agent demand ────────────────────────────────────────────────
  uint64 demand_bounty_threshold         = 77;  // Unfulfilled queries per epoch to trigger bounty
  string demand_bounty_base_reward       = 78;  // Base bounty reward (uzrn)
  string demand_bounty_per_query_bonus   = 79;  // Additional reward per unfulfilled query (uzrn)
  uint64 demand_bounty_expiry_epochs     = 80;  // Epochs before unclaimed bounty expires
  uint64 demand_multiplier_cap           = 81;  // Max demand multiplier for energy (BPS)
  bool   demand_tracking_enabled         = 82;  // Enable/disable demand tracking
  repeated string authorized_demand_reporters = 83; // Addresses allowed to report demand
```

**Step 2: Regenerate protobuf**

Run: `make proto-gen` (or equivalent)

**Step 3: Verify generated code compiles**

Run: `go build ./x/knowledge/types/...`
Expected: PASS

**Step 4: Commit**

```bash
git add proto/zerone/knowledge/v1/genesis.proto x/knowledge/types/*.pb.go
git commit -m "feat(knowledge): add demand signal params to genesis proto"
```

---

### Task 3: Proto — MsgReportDemand in tx.proto

**Files:**
- Modify: `proto/zerone/knowledge/v1/tx.proto` (service Msg + new messages)

**Step 1: Add RPC and messages**

In `tx.proto`, add to the `service Msg` block (after `rpc ExecuteResearchProposal` line 77):

```protobuf
  // ─── Agent demand ──────────────────────────────────────────────────

  // ReportDemand reports agent query demand (authorized reporters only).
  rpc ReportDemand(MsgReportDemand) returns (MsgReportDemandResponse);
```

After the last message (after `MsgExecuteResearchProposalResponse` on line 306), add:

```protobuf
// ─── Agent demand messages ───────────────────────────────────────────────────

message MsgReportDemand {
    option (cosmos.msg.v1.signer) = "reporter";

    string reporter = 1;  // Context server address (whitelisted)
    repeated DemandReport reports = 2;
}

message DemandReport {
    string domain     = 1;
    string subject    = 2;
    uint64 queries    = 3;  // Total queries in this batch
    uint64 fulfilled  = 4;  // How many returned results
    uint64 unfulfilled = 5;
}

message MsgReportDemandResponse {}
```

Note: `DemandReport` is a message in `tx.proto` (not in `types.proto`). It's local to the transaction message — not a stored type.

**Step 2: Regenerate protobuf**

Run: `make proto-gen`

**Step 3: Verify generated code compiles**

Run: `go build ./x/knowledge/types/...`
Expected: PASS

**Step 4: Commit**

```bash
git add proto/zerone/knowledge/v1/tx.proto x/knowledge/types/*.pb.go
git commit -m "feat(knowledge): add MsgReportDemand transaction proto"
```

---

### Task 4: Proto — Demand query RPCs in query.proto

**Files:**
- Modify: `proto/zerone/knowledge/v1/query.proto` (service Query + new messages)

**Step 1: Add query RPCs**

In `query.proto`, add to the `service Query` block (after `FactsAtRisk` at line 106):

```protobuf
  // ─── Agent demand queries ──────────────────────────────────────────

  // ActiveBounties queries active (unclaimed) knowledge bounties.
  rpc ActiveBounties(QueryActiveBountiesRequest) returns (QueryActiveBountiesResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/bounties";
  }

  // DemandSignals queries demand signal data for a domain.
  rpc DemandSignals(QueryDemandSignalsRequest) returns (QueryDemandSignalsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/demand";
  }

  // TopDemandGaps queries the top unfulfilled demand gaps.
  rpc TopDemandGaps(QueryTopDemandGapsRequest) returns (QueryTopDemandGapsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/demand_gaps";
  }
```

After the last message in `query.proto` (after `QueryFactsAtRiskResponse` at line 281), add:

```protobuf
// ─── Agent demand query messages ─────────────────────────────────────────────

message QueryActiveBountiesRequest {
  string domain = 1;  // Optional domain filter
}

message QueryActiveBountiesResponse {
  repeated KnowledgeBounty bounties = 1;
}

message QueryDemandSignalsRequest {
  string domain          = 1;  // Optional domain filter
  uint64 min_unfulfilled = 2;  // Optional minimum unfulfilled count filter
}

message QueryDemandSignalsResponse {
  repeated DemandSignal signals = 1;
}

message QueryTopDemandGapsRequest {
  uint64 limit = 1;  // Max results (default 20)
}

message QueryTopDemandGapsResponse {
  repeated DemandSignal gaps = 1;  // Sorted by unfulfilled count desc
}
```

**Step 2: Regenerate protobuf**

Run: `make proto-gen`

**Step 3: Verify generated code compiles**

Run: `go build ./x/knowledge/types/...`
Expected: PASS

**Step 4: Commit**

```bash
git add proto/zerone/knowledge/v1/query.proto x/knowledge/types/*.pb.go
git commit -m "feat(knowledge): add bounty and demand query RPCs"
```

---

### Task 5: Store keys & genesis defaults

**Files:**
- Modify: `x/knowledge/types/keys.go` (add demand signal + bounty key prefixes after line 92)
- Modify: `x/knowledge/types/genesis.go` (add defaults to `DefaultParams()`, validation to `Validate()`)

**Step 1: Add key prefixes**

In `x/knowledge/types/keys.go`, after the `NewCitationsEpochPrefix` at line 92, add:

```go
	// ─── Agent demand tracking ─────────────────────────────────────────
	DemandSignalPrefix  = []byte{0x38} // 0x38 | domain / subject_hash → DemandSignal
	BountyPrefix        = []byte{0x39} // 0x39 | bounty_id → KnowledgeBounty
	BountyByDomainSubjectPrefix = []byte{0x3a} // 0x3a | domain / subject_hash → bounty_id (active index)
```

Add key constructors at the end of the file:

```go
// DemandSignalKey returns the store key for a demand signal.
func DemandSignalKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, DemandSignalPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// BountyKey returns the store key for a bounty.
func BountyKey(id string) []byte {
	return append(append([]byte{}, BountyPrefix...), []byte(id)...)
}

// BountyByDomainSubjectKey returns the index key for active bounties by domain/subject.
func BountyByDomainSubjectKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, BountyByDomainSubjectPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}
```

**Step 2: Add genesis defaults**

In `x/knowledge/types/genesis.go`, in `DefaultParams()`, after the metabolism params block (after line 111, `MetabolismExpiredToPrunedEpochs: 20`), add:

```go
		// ─── Agent demand ──────────────────────────────────────────────────
		DemandBountyThreshold:       100,            // 100 unfulfilled queries triggers bounty
		DemandBountyBaseReward:      "10000000",     // 10 ZRN base bounty
		DemandBountyPerQueryBonus:   "100000",       // +0.1 ZRN per additional unfulfilled query
		DemandBountyExpiryEpochs:    50,             // ~15 days to claim
		DemandMultiplierCap:         10_000_000,     // 10× max energy multiplier (BPS)
		DemandTrackingEnabled:       true,
		AuthorizedDemandReporters:   []string{},     // Empty — governance must add reporters
```

**Step 3: Add param validation**

In `x/knowledge/types/genesis.go`, in the `Validate()` method, after the bootstrap fund validation block (after line 365), add:

```go
	// ─── Demand params ──────────────────────────────────────────────
	if p.DemandTrackingEnabled {
		if p.DemandBountyThreshold == 0 {
			return fmt.Errorf("demand_bounty_threshold must be > 0 when demand tracking is enabled")
		}
		if p.DemandBountyBaseReward == "" || p.DemandBountyBaseReward == "0" {
			return fmt.Errorf("demand_bounty_base_reward must be > 0 when demand tracking is enabled")
		}
		if p.DemandBountyExpiryEpochs == 0 {
			return fmt.Errorf("demand_bounty_expiry_epochs must be > 0 when demand tracking is enabled")
		}
		if p.DemandMultiplierCap == 0 {
			return fmt.Errorf("demand_multiplier_cap must be > 0 when demand tracking is enabled")
		}
	}
```

**Step 4: Verify compilation**

Run: `go build ./x/knowledge/types/...`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/types/keys.go x/knowledge/types/genesis.go
git commit -m "feat(knowledge): add demand signal store keys and genesis defaults"
```

---

### Task 6: Demand tracking keeper (demand.go)

**Files:**
- Create: `x/knowledge/keeper/demand.go`

**Step 1: Write the demand keeper file**

Create `x/knowledge/keeper/demand.go`:

```go
package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Demand Signal CRUD ──────────────────────────────────────────────────────

// hashSubject normalizes a subject string to a fixed-length hex hash for key construction.
func hashSubject(subject string) string {
	h := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(h[:])
}

// GetDemandSignal retrieves a demand signal by domain and subject.
func (k Keeper) GetDemandSignal(ctx context.Context, domain, subject string) (*types.DemandSignal, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.DemandSignalKey(domain, hashSubject(subject))
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var signal types.DemandSignal
	if err := k.cdc.Unmarshal(bz, &signal); err != nil {
		return nil, false
	}
	return &signal, true
}

// GetOrCreateDemandSignal retrieves or initializes a demand signal.
func (k Keeper) GetOrCreateDemandSignal(ctx context.Context, domain, subject string) (*types.DemandSignal, bool) {
	signal, found := k.GetDemandSignal(ctx, domain, subject)
	if found {
		return signal, true
	}
	return &types.DemandSignal{
		Domain:  domain,
		Subject: subject,
	}, false
}

// SetDemandSignal stores a demand signal.
func (k Keeper) SetDemandSignal(ctx context.Context, signal *types.DemandSignal) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.DemandSignalKey(signal.Domain, hashSubject(signal.Subject))
	bz, err := k.cdc.Marshal(signal)
	if err != nil {
		return err
	}
	return store.Set(key, bz)
}

// IterateDemandSignals iterates over all demand signals.
func (k Keeper) IterateDemandSignals(ctx context.Context, cb func(signal *types.DemandSignal) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DemandSignalPrefix, prefixEndBytes(types.DemandSignalPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var signal types.DemandSignal
		if err := k.cdc.Unmarshal(iter.Value(), &signal); err != nil {
			continue
		}
		if cb(&signal) {
			break
		}
	}
}

// ResetDemandEpochCounters resets epoch-scoped counters on all demand signals.
func (k Keeper) ResetDemandEpochCounters(ctx context.Context) {
	var signals []*types.DemandSignal
	k.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if signal.EpochQueryCount > 0 || signal.EpochUnfulfilled > 0 {
			signal.EpochQueryCount = 0
			signal.EpochUnfulfilled = 0
			signals = append(signals, signal)
		}
		return false
	})
	for _, signal := range signals {
		_ = k.SetDemandSignal(ctx, signal)
	}
}

// IsAuthorizedDemandReporter checks if an address is whitelisted to report demand.
func (k Keeper) IsAuthorizedDemandReporter(ctx context.Context, reporter string) bool {
	params, err := k.GetParams(ctx)
	if err != nil {
		return false
	}
	for _, addr := range params.AuthorizedDemandReporters {
		if addr == reporter {
			return true
		}
	}
	return false
}

// ─── Bounty CRUD ─────────────────────────────────────────────────────────────

// GetBounty retrieves a bounty by ID.
func (k Keeper) GetBounty(ctx context.Context, id string) (*types.KnowledgeBounty, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.BountyKey(id)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var bounty types.KnowledgeBounty
	if err := k.cdc.Unmarshal(bz, &bounty); err != nil {
		return nil, false
	}
	return &bounty, true
}

// SetBounty stores a bounty and maintains the domain/subject index.
func (k Keeper) SetBounty(ctx context.Context, bounty *types.KnowledgeBounty) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := k.cdc.Marshal(bounty)
	if err != nil {
		return err
	}
	if err := store.Set(types.BountyKey(bounty.Id), bz); err != nil {
		return err
	}
	// Maintain active index (only for unclaimed bounties)
	indexKey := types.BountyByDomainSubjectKey(bounty.Domain, hashSubject(bounty.Subject))
	if bounty.Claimed {
		_ = store.Delete(indexKey)
	} else {
		_ = store.Set(indexKey, []byte(bounty.Id))
	}
	return nil
}

// HasActiveBounty checks if there's an active (unclaimed) bounty for a domain/subject.
func (k Keeper) HasActiveBounty(ctx context.Context, domain, subject string) bool {
	store := k.storeService.OpenKVStore(ctx)
	key := types.BountyByDomainSubjectKey(domain, hashSubject(subject))
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return false
	}
	// Verify bounty still exists and is unclaimed
	bounty, found := k.GetBounty(ctx, string(bz))
	return found && !bounty.Claimed
}

// FindMatchingBounty finds an active bounty matching a domain/subject.
func (k Keeper) FindMatchingBounty(ctx context.Context, domain, subject string) (*types.KnowledgeBounty, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.BountyByDomainSubjectKey(domain, hashSubject(subject))
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	bounty, found := k.GetBounty(ctx, string(bz))
	if !found || bounty.Claimed {
		return nil, false
	}
	return bounty, true
}

// IterateBounties iterates over all bounties.
func (k Keeper) IterateBounties(ctx context.Context, cb func(bounty *types.KnowledgeBounty) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.BountyPrefix, prefixEndBytes(types.BountyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var bounty types.KnowledgeBounty
		if err := k.cdc.Unmarshal(iter.Value(), &bounty); err != nil {
			continue
		}
		if cb(&bounty) {
			break
		}
	}
}

// GenerateBountyID creates a deterministic bounty ID from domain, subject, and epoch.
func GenerateBountyID(domain, subject string, epoch uint64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("bounty:%s:%s:%d", domain, subject, epoch)))
	return hex.EncodeToString(h[:16])
}

// ─── Bounty Processing ──────────────────────────────────────────────────────

// ProcessDemandBounties checks for knowledge gaps and creates bounties.
func (k Keeper) ProcessDemandBounties(ctx context.Context, epoch uint64) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	if !params.DemandTrackingEnabled {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	k.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if signal.EpochUnfulfilled < params.DemandBountyThreshold {
			return false
		}

		// Don't create duplicate bounties
		if k.HasActiveBounty(ctx, signal.Domain, signal.Subject) {
			return false
		}

		// Calculate reward: base + per-query bonus
		baseReward, ok := new(big.Int).SetString(params.DemandBountyBaseReward, 10)
		if !ok {
			return false
		}
		perQuery, ok := new(big.Int).SetString(params.DemandBountyPerQueryBonus, 10)
		if !ok {
			perQuery = new(big.Int)
		}
		bonus := new(big.Int).Mul(perQuery, new(big.Int).SetUint64(signal.EpochUnfulfilled))
		totalReward := new(big.Int).Add(baseReward, bonus)

		expiryBlocks := params.DemandBountyExpiryEpochs * params.FitnessEpochBlocks

		bounty := &types.KnowledgeBounty{
			Id:             GenerateBountyID(signal.Domain, signal.Subject, epoch),
			Domain:         signal.Domain,
			Subject:        signal.Subject,
			RewardAmount:   totalReward.String(),
			CreatedAtBlock: height,
			ExpiresAtBlock: height + expiryBlocks,
			DemandCount:    signal.EpochUnfulfilled,
		}

		// Fund bounty from protocol treasury
		if k.bankKeeper != nil {
			rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(totalReward)))
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, "protocol_treasury", types.ModuleName, rewardCoins); err != nil {
				k.Logger(ctx).Error("failed to fund bounty from treasury", "error", err)
				return false
			}
		}

		if err := k.SetBounty(ctx, bounty); err != nil {
			k.Logger(ctx).Error("failed to store bounty", "error", err)
			return false
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.bounty_created",
			sdk.NewAttribute("bounty_id", bounty.Id),
			sdk.NewAttribute("domain", bounty.Domain),
			sdk.NewAttribute("subject", bounty.Subject),
			sdk.NewAttribute("reward", bounty.RewardAmount),
			sdk.NewAttribute("demand_count", fmt.Sprintf("%d", bounty.DemandCount)),
		))

		return false
	})

	// Reset epoch counters after processing
	k.ResetDemandEpochCounters(ctx)

	return nil
}

// ProcessExpiredBounties removes expired bounties and returns funds to treasury.
func (k Keeper) ProcessExpiredBounties(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	var expired []*types.KnowledgeBounty
	k.IterateBounties(ctx, func(bounty *types.KnowledgeBounty) bool {
		if !bounty.Claimed && height >= bounty.ExpiresAtBlock {
			expired = append(expired, bounty)
		}
		return false
	})

	for _, bounty := range expired {
		// Return funds to treasury
		if k.bankKeeper != nil {
			rewardAmt, ok := new(big.Int).SetString(bounty.RewardAmount, 10)
			if ok && rewardAmt.Sign() > 0 {
				rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(rewardAmt)))
				_ = k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "protocol_treasury", rewardCoins)
			}
		}

		// Mark as claimed (expired) so it's cleaned up
		bounty.Claimed = true
		bounty.ClaimedByFactId = "expired"
		_ = k.SetBounty(ctx, bounty)

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.bounty_expired",
			sdk.NewAttribute("bounty_id", bounty.Id),
			sdk.NewAttribute("domain", bounty.Domain),
			sdk.NewAttribute("subject", bounty.Subject),
			sdk.NewAttribute("reward_returned", bounty.RewardAmount),
		))
	}
}

// ClaimBountyForFact checks if a newly created fact fills an active bounty.
func (k Keeper) ClaimBountyForFact(ctx context.Context, fact *types.Fact, claim *types.Claim) {
	if fact.Structure == nil || fact.Structure.Subject == "" {
		return
	}

	bounty, found := k.FindMatchingBounty(ctx, fact.Domain, fact.Structure.Subject)
	if !found {
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Claim the bounty
	bounty.Claimed = true
	bounty.ClaimedByFactId = fact.Id
	_ = k.SetBounty(ctx, bounty)

	// Pay bounty to submitter
	if k.bankKeeper != nil {
		rewardAmt, ok := new(big.Int).SetString(bounty.RewardAmount, 10)
		if ok && rewardAmt.Sign() > 0 {
			submitterAddr, err := sdk.AccAddressFromBech32(claim.Submitter)
			if err == nil {
				rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(rewardAmt)))
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, rewardCoins)
			}
		}
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.bounty_claimed",
		sdk.NewAttribute("bounty_id", bounty.Id),
		sdk.NewAttribute("fact_id", fact.Id),
		sdk.NewAttribute("submitter", claim.Submitter),
		sdk.NewAttribute("reward", bounty.RewardAmount),
	))
}

// ─── Demand-Weighted Energy ──────────────────────────────────────────────────

// GetDemandMultiplier returns the demand multiplier for a domain/subject pair.
// Returns 1_000_000 (1×) if no demand data, capped by DemandMultiplierCap.
func (k Keeper) GetDemandMultiplier(ctx context.Context, domain, subject string) uint64 {
	if subject == "" {
		return 1_000_000 // 1× default
	}

	signal, found := k.GetDemandSignal(ctx, domain, subject)
	if !found || signal.EpochQueryCount == 0 {
		return 1_000_000 // 1× default
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		return 1_000_000
	}

	// Multiplier scales with unfulfilled demand in the epoch.
	// Base 1× (1M), +0.1× per 10 epoch queries (linear scaling).
	multiplier := uint64(1_000_000) + signal.EpochQueryCount*100_000
	if multiplier > params.DemandMultiplierCap {
		multiplier = params.DemandMultiplierCap
	}
	return multiplier
}

// ─── Query helpers ───────────────────────────────────────────────────────────

// GetActiveBounties returns all active (unclaimed, unexpired) bounties.
func (k Keeper) GetActiveBounties(ctx context.Context, domain string) []*types.KnowledgeBounty {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	var bounties []*types.KnowledgeBounty
	k.IterateBounties(ctx, func(bounty *types.KnowledgeBounty) bool {
		if bounty.Claimed || height >= bounty.ExpiresAtBlock {
			return false
		}
		if domain != "" && bounty.Domain != domain {
			return false
		}
		bounties = append(bounties, bounty)
		return false
	})
	return bounties
}

// GetTopDemandGaps returns demand signals sorted by unfulfilled count desc.
func (k Keeper) GetTopDemandGaps(ctx context.Context, limit uint64) []*types.DemandSignal {
	if limit == 0 {
		limit = 20
	}

	var signals []*types.DemandSignal
	k.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if signal.UnfulfilledCount > 0 {
			signals = append(signals, signal)
		}
		return false
	})

	sort.Slice(signals, func(i, j int) bool {
		return signals[i].UnfulfilledCount > signals[j].UnfulfilledCount
	})

	if uint64(len(signals)) > limit {
		signals = signals[:limit]
	}
	return signals
}
```

**Step 2: Verify compilation**

Run: `go build ./x/knowledge/keeper/...`
Expected: PASS (may need to stub `prefixEndBytes` if not already in scope — it should be from state.go)

**Step 3: Commit**

```bash
git add x/knowledge/keeper/demand.go
git commit -m "feat(knowledge): add demand tracking keeper with bounty lifecycle"
```

---

### Task 7: MsgReportDemand handler + bounty claim in rounds.go + demand energy in metabolism.go

**Files:**
- Modify: `x/knowledge/keeper/msg_server.go` (add ReportDemand handler)
- Modify: `x/knowledge/keeper/rounds.go` (add bounty claim in createFactFromClaim)
- Modify: `x/knowledge/keeper/metabolism.go` (add demand-weighted energy)
- Modify: `x/knowledge/keeper/phases.go` (add demand processing to epoch boundary)

**Step 1: Add ReportDemand to msg_server.go**

At the end of `x/knowledge/keeper/msg_server.go`, add:

```go
// ─── Agent demand handlers ───────────────────────────────────────────────────

func (m *msgServer) ReportDemand(ctx context.Context, msg *types.MsgReportDemand) (*types.MsgReportDemandResponse, error) {
	if !m.keeper.IsAuthorizedDemandReporter(ctx, msg.Reporter) {
		return nil, fmt.Errorf("unauthorized demand reporter: %s", msg.Reporter)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	for _, report := range msg.Reports {
		signal, _ := m.keeper.GetOrCreateDemandSignal(ctx, report.Domain, report.Subject)
		signal.QueryCount += report.Queries
		signal.FulfilledCount += report.Fulfilled
		signal.UnfulfilledCount += report.Unfulfilled
		signal.EpochQueryCount += report.Queries
		signal.EpochUnfulfilled += report.Unfulfilled
		signal.LastQueryBlock = height
		if err := m.keeper.SetDemandSignal(ctx, signal); err != nil {
			return nil, fmt.Errorf("failed to store demand signal: %w", err)
		}
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.demand_reported",
		sdk.NewAttribute("reporter", msg.Reporter),
		sdk.NewAttribute("report_count", fmt.Sprintf("%d", len(msg.Reports))),
	))

	return &types.MsgReportDemandResponse{}, nil
}
```

**Step 2: Add bounty claim in createFactFromClaim (rounds.go)**

In `x/knowledge/keeper/rounds.go`, in the `createFactFromClaim` function, add after the vesting schedule block (after line 249, before the `fact_created` event emission):

```go
	// Check if this fact fills an active knowledge bounty
	k.ClaimBountyForFact(ctx, fact, claim)
```

**Step 3: Add demand-weighted energy in metabolism.go**

In `x/knowledge/keeper/metabolism.go`, in the `calculateEnergyIncome` function (line 118), modify the query energy calculation. Replace line 122:

```go
	income += fact.QueryCountEpoch * params.MetabolismEnergyPerQuery
```

with:

```go
	// Demand-weighted query energy
	subject := ""
	if fact.Structure != nil {
		subject = fact.Structure.Subject
	}
	demandMultiplier := k.GetDemandMultiplier(ctx, fact.Domain, subject)
	income += fact.QueryCountEpoch * params.MetabolismEnergyPerQuery * demandMultiplier / 1_000_000
```

**Step 4: Add demand processing to phases.go**

In `x/knowledge/keeper/phases.go`, in the `BeginBlocker` function, inside the epoch boundary block (after the `UpdateAllFitnessScores` call on line 32), add:

```go
		// Process agent demand bounties at epoch boundaries
		if err := k.ProcessDemandBounties(ctx, epoch); err != nil {
			k.Logger(ctx).Error("demand bounty processing failed", "epoch", epoch, "error", err)
		}
		// Clean up expired bounties
		k.ProcessExpiredBounties(ctx)
```

**Step 5: Verify compilation**

Run: `go build ./x/knowledge/keeper/...`
Expected: PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/msg_server.go x/knowledge/keeper/rounds.go x/knowledge/keeper/metabolism.go x/knowledge/keeper/phases.go
git commit -m "feat(knowledge): integrate demand signals into msg server, bounty claims, energy, and epoch processing"
```

---

### Task 8: gRPC query handlers

**Files:**
- Modify: `x/knowledge/keeper/grpc_query.go` (add 3 query handlers)

**Step 1: Add query handlers**

At the end of `x/knowledge/keeper/grpc_query.go`, add:

```go
// ─── Agent demand queries ────────────────────────────────────────────────────

func (q *queryServer) ActiveBounties(ctx context.Context, req *types.QueryActiveBountiesRequest) (*types.QueryActiveBountiesResponse, error) {
	bounties := q.keeper.GetActiveBounties(ctx, req.Domain)
	return &types.QueryActiveBountiesResponse{Bounties: bounties}, nil
}

func (q *queryServer) DemandSignals(ctx context.Context, req *types.QueryDemandSignalsRequest) (*types.QueryDemandSignalsResponse, error) {
	var signals []*types.DemandSignal
	q.keeper.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if req.Domain != "" && signal.Domain != req.Domain {
			return false
		}
		if req.MinUnfulfilled > 0 && signal.UnfulfilledCount < req.MinUnfulfilled {
			return false
		}
		signals = append(signals, signal)
		return false
	})
	return &types.QueryDemandSignalsResponse{Signals: signals}, nil
}

func (q *queryServer) TopDemandGaps(ctx context.Context, req *types.QueryTopDemandGapsRequest) (*types.QueryTopDemandGapsResponse, error) {
	gaps := q.keeper.GetTopDemandGaps(ctx, req.Limit)
	return &types.QueryTopDemandGapsResponse{Gaps: gaps}, nil
}
```

**Step 2: Verify compilation**

Run: `go build ./x/knowledge/keeper/...`
Expected: PASS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/grpc_query.go
git commit -m "feat(knowledge): add bounty and demand gRPC query handlers"
```

---

### Task 9: CLI commands

**Files:**
- Modify: `x/knowledge/client/cli/query.go` (add 3 query commands)

**Step 1: Add CLI query commands**

Check the existing CLI pattern in `x/knowledge/client/cli/query.go` for how commands are structured. Add these commands to the `GetQueryCmd()` function:

```go
	cmd.AddCommand(
		CmdQueryBounties(),
		CmdQueryDemandSignals(),
		CmdQueryDemandGaps(),
	)
```

Then add the command implementations at the end of the file:

```go
func CmdQueryBounties() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bounties",
		Short: "Query active knowledge bounties",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			domain, _ := cmd.Flags().GetString("domain")

			res, err := queryClient.ActiveBounties(cmd.Context(), &types.QueryActiveBountiesRequest{
				Domain: domain,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryDemandSignals() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demand-signals",
		Short: "Query demand signals",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			domain, _ := cmd.Flags().GetString("domain")
			minUnfulfilled, _ := cmd.Flags().GetUint64("min-unfulfilled")

			res, err := queryClient.DemandSignals(cmd.Context(), &types.QueryDemandSignalsRequest{
				Domain:         domain,
				MinUnfulfilled: minUnfulfilled,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	cmd.Flags().String("domain", "", "Filter by domain")
	cmd.Flags().Uint64("min-unfulfilled", 0, "Minimum unfulfilled count")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func CmdQueryDemandGaps() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demand-gaps",
		Short: "Query top knowledge gaps sorted by unfulfilled demand",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			queryClient := types.NewQueryClient(clientCtx)
			limit, _ := cmd.Flags().GetUint64("limit")

			res, err := queryClient.TopDemandGaps(cmd.Context(), &types.QueryTopDemandGapsRequest{
				Limit: limit,
			})
			if err != nil {
				return err
			}
			return clientCtx.PrintProto(res)
		},
	}
	cmd.Flags().Uint64("limit", 20, "Max results")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

Ensure the necessary imports are present: `"github.com/cosmos/cosmos-sdk/client"`, `"github.com/cosmos/cosmos-sdk/client/flags"`, `"github.com/spf13/cobra"`.

**Step 2: Verify compilation**

Run: `go build ./x/knowledge/client/...`
Expected: PASS

**Step 3: Commit**

```bash
git add x/knowledge/client/cli/query.go
git commit -m "feat(knowledge): add CLI commands for bounties and demand signals"
```

---

### Task 10: Context server demand tracking + bounty endpoints

**Files:**
- Modify: `tools/knowledge-context/main.go`

**Step 1: Add demand buffer and tracking**

At the top of `main.go`, after the imports and before the type definitions (after line 25), add:

```go
var demandBuffer struct {
	mu      sync.Mutex
	reports map[string]*demandReport // key: domain+"|"+subject
}

type demandReport struct {
	Domain     string `json:"domain"`
	Subject    string `json:"subject"`
	Queries    uint64 `json:"queries"`
	Fulfilled  uint64 `json:"fulfilled"`
	Unfulfilled uint64 `json:"unfulfilled"`
}

func init() {
	demandBuffer.reports = make(map[string]*demandReport)
}

func trackDemand(domain, subject string, factCount int) {
	demandBuffer.mu.Lock()
	defer demandBuffer.mu.Unlock()

	key := domain + "|" + subject
	report, ok := demandBuffer.reports[key]
	if !ok {
		report = &demandReport{Domain: domain, Subject: subject}
		demandBuffer.reports[key] = report
	}
	report.Queries++
	if factCount == 0 {
		report.Unfulfilled++
	} else {
		report.Fulfilled++
	}
}
```

Add `"sync"` to the imports.

**Step 2: Add bounties and demand_gaps endpoints**

In the `main()` function, add new handler registrations (after `/health`):

```go
	http.HandleFunc("/bounties", bountiesHandler)
	http.HandleFunc("/demand_gaps", demandGapsHandler)
```

Add the handler functions:

```go
func bountiesHandler(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	url := *nodeURL + "/zerone/knowledge/v1/bounties"
	if domain != "" {
		url += "?domain=" + domain
	}
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(resp.Body)
	w.Write(body)
}

func demandGapsHandler(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	url := *nodeURL + "/zerone/knowledge/v1/demand_gaps"
	if limit != "" {
		url += "?limit=" + limit
	}
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	body, _ := io.ReadAll(resp.Body)
	w.Write(body)
}
```

**Step 3: Integrate demand tracking into contextHandler**

In the `contextHandler` function, after the `filtered := filterFacts(...)` line, add demand tracking for each domain/subject pair that was queried:

```go
	// Track demand for each queried domain/subject
	for dom := range domains {
		trackDemand(dom, subjectFilter, len(filtered))
	}
```

**Step 4: Update log output in main()**

Add the new endpoints to the startup log:

```go
	log.Printf("  GET /bounties?domain=physics")
	log.Printf("  GET /demand_gaps?limit=20")
```

**Step 5: Verify compilation**

Run: `go build ./tools/knowledge-context/...`
Expected: PASS

**Step 6: Commit**

```bash
git add tools/knowledge-context/main.go
git commit -m "feat(knowledge): add demand tracking and bounty endpoints to context server"
```

---

### Task 11: Tests

**Files:**
- Create: `x/knowledge/keeper/demand_test.go`

**Step 1: Write all 12 tests**

Create `x/knowledge/keeper/demand_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestDemandTracking_FulfilledQuery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "quantum error correction")
	signal.QueryCount++
	signal.FulfilledCount++
	signal.EpochQueryCount++
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	retrieved, found := k.GetDemandSignal(ctx, "physics", "quantum error correction")
	require.True(t, found)
	require.Equal(t, uint64(1), retrieved.QueryCount)
	require.Equal(t, uint64(1), retrieved.FulfilledCount)
	require.Equal(t, uint64(0), retrieved.UnfulfilledCount)
}

func TestDemandTracking_UnfulfilledQuery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "quantum error correction")
	signal.QueryCount++
	signal.UnfulfilledCount++
	signal.EpochQueryCount++
	signal.EpochUnfulfilled++
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	retrieved, found := k.GetDemandSignal(ctx, "physics", "quantum error correction")
	require.True(t, found)
	require.Equal(t, uint64(1), retrieved.UnfulfilledCount)
	require.Equal(t, uint64(1), retrieved.EpochUnfulfilled)
}

func TestBountyCreation_ThresholdMet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set up authorized reporter and demand tracking
	params, _ := k.GetParams(ctx)
	params.DemandTrackingEnabled = true
	params.DemandBountyThreshold = 100
	params.DemandBountyBaseReward = "10000000"
	params.DemandBountyPerQueryBonus = "100000"
	params.DemandBountyExpiryEpochs = 50
	params.DemandMultiplierCap = 10_000_000
	require.NoError(t, k.SetParams(ctx, params))

	// Create a demand signal with 150 unfulfilled queries (above 100 threshold)
	signal := &types.DemandSignal{
		Domain:           "physics",
		Subject:          "quantum error correction",
		QueryCount:       150,
		UnfulfilledCount: 150,
		EpochQueryCount:  150,
		EpochUnfulfilled: 150,
	}
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	// Process bounties
	require.NoError(t, k.ProcessDemandBounties(ctx, 1))

	// Should have created a bounty
	bounties := k.GetActiveBounties(ctx, "physics")
	require.Len(t, bounties, 1)
	require.Equal(t, "physics", bounties[0].Domain)
	require.Equal(t, "quantum error correction", bounties[0].Subject)
	// Reward = 10M + 150 * 100K = 10M + 15M = 25M uzrn
	require.Equal(t, "25000000", bounties[0].RewardAmount)
}

func TestBountyCreation_ThresholdNotMet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)
	params.DemandTrackingEnabled = true
	params.DemandBountyThreshold = 100
	params.DemandBountyBaseReward = "10000000"
	params.DemandBountyPerQueryBonus = "100000"
	params.DemandBountyExpiryEpochs = 50
	params.DemandMultiplierCap = 10_000_000
	require.NoError(t, k.SetParams(ctx, params))

	// Only 99 unfulfilled — below threshold
	signal := &types.DemandSignal{
		Domain:           "physics",
		Subject:          "quantum error correction",
		EpochUnfulfilled: 99,
	}
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	require.NoError(t, k.ProcessDemandBounties(ctx, 1))

	bounties := k.GetActiveBounties(ctx, "physics")
	require.Len(t, bounties, 0, "should not create bounty below threshold")
}

func TestBountyCreation_AlreadyExists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)
	params.DemandTrackingEnabled = true
	params.DemandBountyThreshold = 100
	params.DemandBountyBaseReward = "10000000"
	params.DemandBountyPerQueryBonus = "100000"
	params.DemandBountyExpiryEpochs = 50
	params.DemandMultiplierCap = 10_000_000
	require.NoError(t, k.SetParams(ctx, params))

	// Create an active bounty for this domain/subject
	bounty := &types.KnowledgeBounty{
		Id:             "existing-bounty",
		Domain:         "physics",
		Subject:        "quantum error correction",
		RewardAmount:   "10000000",
		CreatedAtBlock: 100,
		ExpiresAtBlock: 1000000,
	}
	require.NoError(t, k.SetBounty(ctx, bounty))

	// Create demand that exceeds threshold
	signal := &types.DemandSignal{
		Domain:           "physics",
		Subject:          "quantum error correction",
		EpochUnfulfilled: 200,
	}
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	require.NoError(t, k.ProcessDemandBounties(ctx, 1))

	// Should still only have 1 bounty (no duplicate)
	bounties := k.GetActiveBounties(ctx, "physics")
	require.Len(t, bounties, 1)
	require.Equal(t, "existing-bounty", bounties[0].Id)
}

func TestBountyClaim_MatchingFact(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	// Create an active bounty
	bounty := &types.KnowledgeBounty{
		Id:             "bounty-1",
		Domain:         "physics",
		Subject:        "quantum error correction",
		RewardAmount:   "10000000",
		CreatedAtBlock: 100,
		ExpiresAtBlock: 1000000,
	}
	require.NoError(t, k.SetBounty(ctx, bounty))

	// Create a fact that matches the bounty subject
	submitter := makeValidBech32Addr("submitter1")
	fact := &types.Fact{
		Id:      "fact-1",
		Content: "Quantum error correction thresholds for surface codes",
		Domain:  "physics",
		Status:  types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject:   "quantum error correction",
			Predicate: "has threshold of",
		},
		Submitter: submitter,
	}
	claim := &types.Claim{
		Id:        "claim-1",
		Domain:    "physics",
		Submitter: submitter,
	}

	k.ClaimBountyForFact(ctx, fact, claim)

	// Bounty should be claimed
	updated, found := k.GetBounty(ctx, "bounty-1")
	require.True(t, found)
	require.True(t, updated.Claimed)
	require.Equal(t, "fact-1", updated.ClaimedByFactId)

	// Bank should have sent reward
	require.Len(t, bk.sendCalls, 1)
	require.Equal(t, types.ModuleName, bk.sendCalls[0].from)
}

func TestBountyClaim_WrongSubject(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create bounty for "quantum error correction"
	bounty := &types.KnowledgeBounty{
		Id:             "bounty-wrong",
		Domain:         "physics",
		Subject:        "quantum error correction",
		RewardAmount:   "10000000",
		CreatedAtBlock: 100,
		ExpiresAtBlock: 1000000,
	}
	require.NoError(t, k.SetBounty(ctx, bounty))

	// Create a fact with a different subject
	fact := &types.Fact{
		Id:     "fact-wrong",
		Domain: "physics",
		Structure: &types.ClaimStructure{
			Subject:   "classical mechanics",
			Predicate: "describes motion",
		},
		Submitter: "zrn1test",
	}
	claim := &types.Claim{
		Id:        "claim-wrong",
		Domain:    "physics",
		Submitter: "zrn1test",
	}

	k.ClaimBountyForFact(ctx, fact, claim)

	// Bounty should NOT be claimed
	updated, found := k.GetBounty(ctx, "bounty-wrong")
	require.True(t, found)
	require.False(t, updated.Claimed, "bounty should not be claimed by wrong subject")
}

func TestBountyExpiry(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	// Create a bounty that expires at block 200 (current is 100)
	bounty := &types.KnowledgeBounty{
		Id:             "bounty-exp",
		Domain:         "physics",
		Subject:        "dark matter composition",
		RewardAmount:   "5000000",
		CreatedAtBlock: 50,
		ExpiresAtBlock: 200,
	}
	require.NoError(t, k.SetBounty(ctx, bounty))

	// Advance to block 200 and process expired bounties
	ctx = advanceBlocks(ctx, 100) // now at block 200
	k.ProcessExpiredBounties(ctx)

	// Bounty should be marked as claimed (expired)
	updated, found := k.GetBounty(ctx, "bounty-exp")
	require.True(t, found)
	require.True(t, updated.Claimed)
	require.Equal(t, "expired", updated.ClaimedByFactId)

	// Funds should be returned to treasury
	require.Len(t, bk.sendCalls, 1)
	require.Equal(t, types.ModuleName, bk.sendCalls[0].from)
	require.Equal(t, "protocol_treasury", bk.sendCalls[0].to)
}

func TestDemandMultiplier_HighDemand(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)
	params.DemandTrackingEnabled = true
	params.DemandMultiplierCap = 10_000_000
	require.NoError(t, k.SetParams(ctx, params))

	// Create high demand signal
	signal := &types.DemandSignal{
		Domain:          "physics",
		Subject:         "quantum error correction",
		EpochQueryCount: 50,
	}
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	multiplier := k.GetDemandMultiplier(ctx, "physics", "quantum error correction")
	// Base 1M + 50 * 100K = 1M + 5M = 6M (6×)
	require.Equal(t, uint64(6_000_000), multiplier)
}

func TestDemandMultiplier_Cap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)
	params.DemandTrackingEnabled = true
	params.DemandMultiplierCap = 10_000_000
	require.NoError(t, k.SetParams(ctx, params))

	// Create extremely high demand
	signal := &types.DemandSignal{
		Domain:          "physics",
		Subject:         "everything",
		EpochQueryCount: 1000, // Would give 1M + 1000*100K = 101M, but capped
	}
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	multiplier := k.GetDemandMultiplier(ctx, "physics", "everything")
	require.Equal(t, uint64(10_000_000), multiplier, "multiplier should be capped at DemandMultiplierCap")
}

func TestTopDemandGaps_Sorted(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create demand signals with different unfulfilled counts
	signals := []struct {
		domain, subject string
		unfulfilled     uint64
	}{
		{"physics", "dark energy", 50},
		{"physics", "quantum gravity", 200},
		{"mathematics", "P vs NP", 100},
		{"biology", "abiogenesis", 10},
	}
	for _, s := range signals {
		signal := &types.DemandSignal{
			Domain:           s.domain,
			Subject:          s.subject,
			UnfulfilledCount: s.unfulfilled,
		}
		require.NoError(t, k.SetDemandSignal(ctx, signal))
	}

	gaps := k.GetTopDemandGaps(ctx, 3)
	require.Len(t, gaps, 3)
	require.Equal(t, uint64(200), gaps[0].UnfulfilledCount, "highest unfulfilled first")
	require.Equal(t, uint64(100), gaps[1].UnfulfilledCount)
	require.Equal(t, uint64(50), gaps[2].UnfulfilledCount)
}

func TestReportDemand_Unauthorized(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)
	params.DemandTrackingEnabled = true
	params.AuthorizedDemandReporters = []string{"zrn1authorized"}
	require.NoError(t, k.SetParams(ctx, params))

	ms := keeper.NewMsgServerImpl(k)

	// Unauthorized reporter should be rejected
	_, err := ms.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "zrn1unauthorized",
		Reports: []*types.DemandReport{
			{Domain: "physics", Subject: "test", Queries: 10, Unfulfilled: 5},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")

	// Authorized reporter should succeed
	_, err = ms.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "zrn1authorized",
		Reports: []*types.DemandReport{
			{Domain: "physics", Subject: "test", Queries: 10, Unfulfilled: 5},
		},
	})
	require.NoError(t, err)

	// Verify signal was stored
	signal, found := k.GetDemandSignal(ctx, "physics", "test")
	require.True(t, found)
	require.Equal(t, uint64(10), signal.QueryCount)
	require.Equal(t, uint64(5), signal.UnfulfilledCount)
}
```

**Step 2: Run tests**

Run: `go test ./x/knowledge/keeper/... -run TestDemand -v`
Expected: All 12 tests PASS

Also run: `go test ./x/knowledge/keeper/... -run TestBounty -v`
Expected: All bounty tests PASS

Also run: `go test ./x/knowledge/keeper/... -run TestReportDemand -v`
Expected: PASS

**Step 3: Run full knowledge keeper test suite to check for regressions**

Run: `go test ./x/knowledge/keeper/... -v -count=1`
Expected: All existing + new tests PASS

**Step 4: Commit**

```bash
git add x/knowledge/keeper/demand_test.go
git commit -m "test(knowledge): add 12 demand signal and bounty tests"
```

---

### Task 12: Module registration check + final build

**Files:**
- Verify: `x/knowledge/module.go` (RegisterServices should auto-pick up new RPCs from generated code)
- Verify: `app/ante.go` (check if MsgReportDemand needs to be added to BootstrapGasFreeTypes)

**Step 1: Verify module registration**

The `RegisterMsgServer` and `RegisterQueryServer` calls in `module.go` use the generated interfaces. Since we added new RPCs, the `UnimplementedMsgServer` and `UnimplementedQueryServer` from protobuf will include the new methods. Verify that the new handler methods satisfy the interface by building.

Run: `go build ./x/knowledge/...`
Expected: PASS

**Step 2: Check if MsgReportDemand should be gas-free during bootstrap**

Read `app/ante.go` and check `BootstrapGasFreeTypes`. `MsgReportDemand` is an infrastructure message from context servers — it should be gas-free during bootstrap. Add it if the pattern matches.

**Step 3: Full build**

Run: `go build ./...`
Expected: PASS

**Step 4: Final commit with all remaining changes**

```bash
git add -A
git commit -m "feat(knowledge): add agent demand signals with auto-generated knowledge bounties"
```

---

## Verification Checklist

After all tasks complete:

1. `go build ./...` — full project compiles
2. `go test ./x/knowledge/keeper/... -v` — all tests pass (existing + 12 new)
3. `go test ./x/knowledge/types/... -v` — type tests pass
4. Proto field numbers are sequential and non-conflicting (Params: 77-83, next available: 84)
5. Store key prefixes are non-conflicting (0x38, 0x39, 0x3a — next available: 0x3b)
6. Events use `zerone.knowledge.*` namespace consistently
7. Bounty lifecycle: create → claim OR expire → funds flow correctly
8. Demand multiplier: 1× default, scales linearly, capped at param
