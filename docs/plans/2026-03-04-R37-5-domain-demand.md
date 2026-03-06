# R37-5 — Domain Management & Training Demand Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement domain proposal/endorsement, training demand tracking with auto-bounty generation, manual bounty funding, bounty fulfillment on sample creation, and scraped source registry.

**Architecture:** All proto types already exist (Domain, TrainingDemand, DataBounty, ScrapedSourceEntry, DemandReport). All tx messages already defined in tx.proto. State CRUD goes in `state.go`, business logic in new `domain.go` and `demand.go` keeper files. Msg server stubs already exist — just wire them to keeper methods.

**Tech Stack:** Cosmos SDK v0.50.15, protobuf, Go 1.24+

---

### Task 1: Add Missing Key Constructors & Sequence

**Files:**
- Modify: `x/knowledge/types/keys.go:46-50` (add BountySeqKey)
- Modify: `x/knowledge/keeper/state.go` (add CRUD for TrainingDemand, DataBounty, ScrapedSource + NextBountyID)

**Step 1: Add BountySeqKey to keys.go**

In `x/knowledge/types/keys.go`, add after `DatasetSeqKey`:

```go
BountySeqKey = []byte{0x84} // uint64 next bounty ID
```

Also add a `BountyDomainIndexKey` constructor and prefix for iterating bounties by domain. Add after the `ScrapedSourceKeyFn`:

```go
// BountyDomainIndexKey returns the index key for a bounty within a domain.
func BountyDomainIndexKey(domain, bountyID string) []byte {
	key := append(append([]byte{}, BountyByDomainSubjectPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(bountyID)...)
}

// BountyDomainByDomainPrefix returns the prefix for iterating bounties in a domain.
func BountyDomainByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, BountyByDomainSubjectPrefix...), []byte(domain)...)
	return append(key, '/')
}
```

**Step 2: Add TrainingDemand CRUD to state.go**

Add to `x/knowledge/keeper/state.go`:

```go
// ─── TrainingDemand CRUD ────────────────────────────────────────────────────

func (k Keeper) SetTrainingDemand(ctx context.Context, demand *types.TrainingDemand) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(demand)
	if err != nil {
		return fmt.Errorf("failed to marshal training demand: %w", err)
	}
	return store.Set(types.TrainingDemandKeyFn(demand.Domain, demand.Subject), bz)
}

func (k Keeper) GetTrainingDemand(ctx context.Context, domain, subject string) (*types.TrainingDemand, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TrainingDemandKeyFn(domain, subject))
	if err != nil || bz == nil {
		return nil, false
	}
	var demand types.TrainingDemand
	if err := proto.Unmarshal(bz, &demand); err != nil {
		return nil, false
	}
	return &demand, true
}

func (k Keeper) IterateTrainingDemands(ctx context.Context, cb func(demand *types.TrainingDemand) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.TrainingDemandKey, prefixEndBytes(types.TrainingDemandKey))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var demand types.TrainingDemand
		if err := proto.Unmarshal(iter.Value(), &demand); err != nil {
			continue
		}
		if cb(&demand) {
			break
		}
	}
}
```

**Step 3: Add DataBounty CRUD to state.go**

```go
// ─── DataBounty CRUD ────────────────────────────────────────────────────────

func (k Keeper) SetDataBounty(ctx context.Context, bounty *types.DataBounty) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(bounty)
	if err != nil {
		return fmt.Errorf("failed to marshal data bounty: %w", err)
	}
	return store.Set(types.DataBountyKey(bounty.Id), bz)
}

func (k Keeper) GetDataBounty(ctx context.Context, id string) (*types.DataBounty, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DataBountyKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var bounty types.DataBounty
	if err := proto.Unmarshal(bz, &bounty); err != nil {
		return nil, false
	}
	return &bounty, true
}

func (k Keeper) DeleteDataBounty(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.DataBountyKey(id))
}

func (k Keeper) SetBountyDomainIndex(ctx context.Context, domain, bountyID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.BountyDomainIndexKey(domain, bountyID), []byte{0x01})
}

func (k Keeper) DeleteBountyDomainIndex(ctx context.Context, domain, bountyID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.BountyDomainIndexKey(domain, bountyID))
}

func (k Keeper) GetActiveBounties(ctx context.Context, domain string) []*types.DataBounty {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.BountyDomainByDomainPrefix(domain)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var bounties []*types.DataBounty
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Key()[len(prefix):])
		bounty, found := k.GetDataBounty(ctx, id)
		if found && !bounty.Claimed {
			bounties = append(bounties, bounty)
		}
	}
	return bounties
}

func (k Keeper) NextBountyID(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.BountySeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	id := fmt.Sprintf("%x", seq)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = store.Set(types.BountySeqKey, next)
	return id
}
```

**Step 4: Add ScrapedSource CRUD to state.go**

```go
// ─── ScrapedSource CRUD ─────────────────────────────────────────────────────

func (k Keeper) SetScrapedSource(ctx context.Context, entry *types.ScrapedSourceEntry) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal scraped source: %w", err)
	}
	return store.Set(types.ScrapedSourceKeyFn(entry.Id), bz)
}

func (k Keeper) GetScrapedSource(ctx context.Context, id string) (*types.ScrapedSourceEntry, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ScrapedSourceKeyFn(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var entry types.ScrapedSourceEntry
	if err := proto.Unmarshal(bz, &entry); err != nil {
		return nil, false
	}
	return &entry, true
}

func (k Keeper) DeleteScrapedSource(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.ScrapedSourceKeyFn(id))
}

func (k Keeper) GetScrapedSourcePenalty(ctx context.Context, platform, domain string) uint64 {
	id := platform + "/" + domain
	entry, found := k.GetScrapedSource(ctx, id)
	if !found {
		return 0
	}
	return entry.NoveltyPenalty
}

func (k Keeper) IterateScrapedSources(ctx context.Context, cb func(entry *types.ScrapedSourceEntry) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ScrapedSourceKey, prefixEndBytes(types.ScrapedSourceKey))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var entry types.ScrapedSourceEntry
		if err := proto.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		if cb(&entry) {
			break
		}
	}
}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/...`
Expected: PASS (no test changes yet, just verifying compilation)

**Step 6: Commit**

```bash
git add x/knowledge/types/keys.go x/knowledge/keeper/state.go
git commit -m "feat(knowledge): state CRUD for training demand, bounties, scraped sources (R37-5)"
```

---

### Task 2: Write Failing Tests for State CRUD

**Files:**
- Modify: `x/knowledge/keeper/state_test.go`

**Step 1: Write failing tests for TrainingDemand, DataBounty, ScrapedSource CRUD**

Add to `x/knowledge/keeper/state_test.go`:

```go
// ─── TrainingDemand CRUD ────────────────────────────────────────────────────

func TestSetGetTrainingDemand(t *testing.T) {
	k, ctx := setupKeeper(t)
	demand := &types.TrainingDemand{
		Domain:           "science",
		Subject:          "quantum_computing",
		QueryCount:       10,
		FulfilledCount:   3,
		UnfulfilledCount: 7,
		LastQueryBlock:   100,
	}
	require.NoError(t, k.SetTrainingDemand(ctx, demand))

	got, found := k.GetTrainingDemand(ctx, "science", "quantum_computing")
	require.True(t, found)
	require.Equal(t, uint64(10), got.QueryCount)
	require.Equal(t, uint64(7), got.UnfulfilledCount)

	_, found = k.GetTrainingDemand(ctx, "science", "nonexistent")
	require.False(t, found)
}

func TestIterateTrainingDemands(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetTrainingDemand(ctx, &types.TrainingDemand{Domain: "science", Subject: "physics"}))
	require.NoError(t, k.SetTrainingDemand(ctx, &types.TrainingDemand{Domain: "science", Subject: "chemistry"}))
	require.NoError(t, k.SetTrainingDemand(ctx, &types.TrainingDemand{Domain: "tech", Subject: "golang"}))

	var count int
	k.IterateTrainingDemands(ctx, func(_ *types.TrainingDemand) bool {
		count++
		return false
	})
	require.Equal(t, 3, count)
}

// ─── DataBounty CRUD ────────────────────────────────────────────────────────

func TestSetGetDataBounty(t *testing.T) {
	k, ctx := setupKeeper(t)
	bounty := &types.DataBounty{
		Id:           "1",
		Domain:       "science",
		Subject:      "quantum_computing",
		RewardAmount: "10000000",
	}
	require.NoError(t, k.SetDataBounty(ctx, bounty))

	got, found := k.GetDataBounty(ctx, "1")
	require.True(t, found)
	require.Equal(t, "10000000", got.RewardAmount)

	_, found = k.GetDataBounty(ctx, "nonexistent")
	require.False(t, found)
}

func TestDeleteDataBounty(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{Id: "1", Domain: "science"}))
	require.NoError(t, k.DeleteDataBounty(ctx, "1"))
	_, found := k.GetDataBounty(ctx, "1")
	require.False(t, found)
}

func TestGetActiveBounties(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Active bounty
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{Id: "1", Domain: "science", Claimed: false}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "1"))

	// Claimed bounty (should not appear)
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{Id: "2", Domain: "science", Claimed: true}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "2"))

	// Different domain
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{Id: "3", Domain: "tech", Claimed: false}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "tech", "3"))

	bounties := k.GetActiveBounties(ctx, "science")
	require.Len(t, bounties, 1)
	require.Equal(t, "1", bounties[0].Id)
}

func TestNextBountyID(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.Equal(t, "1", k.NextBountyID(ctx))
	require.Equal(t, "2", k.NextBountyID(ctx))
	require.Equal(t, "3", k.NextBountyID(ctx))
}

// ─── ScrapedSource CRUD ─────────────────────────────────────────────────────

func TestSetGetScrapedSource(t *testing.T) {
	k, ctx := setupKeeper(t)
	entry := &types.ScrapedSourceEntry{
		Id:            "reddit/science",
		Platform:      "reddit",
		Domain:        "science",
		NoveltyPenalty: 200000,
	}
	require.NoError(t, k.SetScrapedSource(ctx, entry))

	got, found := k.GetScrapedSource(ctx, "reddit/science")
	require.True(t, found)
	require.Equal(t, uint64(200000), got.NoveltyPenalty)

	_, found = k.GetScrapedSource(ctx, "nonexistent")
	require.False(t, found)
}

func TestDeleteScrapedSource(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetScrapedSource(ctx, &types.ScrapedSourceEntry{Id: "reddit/science"}))
	require.NoError(t, k.DeleteScrapedSource(ctx, "reddit/science"))
	_, found := k.GetScrapedSource(ctx, "reddit/science")
	require.False(t, found)
}

func TestGetScrapedSourcePenalty(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetScrapedSource(ctx, &types.ScrapedSourceEntry{
		Id:            "stackoverflow/technology",
		Platform:      "stackoverflow",
		Domain:        "technology",
		NoveltyPenalty: 300000,
	}))

	penalty := k.GetScrapedSourcePenalty(ctx, "stackoverflow", "technology")
	require.Equal(t, uint64(300000), penalty)

	penalty = k.GetScrapedSourcePenalty(ctx, "unknown", "technology")
	require.Equal(t, uint64(0), penalty)
}

func TestIterateScrapedSources(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetScrapedSource(ctx, &types.ScrapedSourceEntry{Id: "reddit/science"}))
	require.NoError(t, k.SetScrapedSource(ctx, &types.ScrapedSourceEntry{Id: "stackoverflow/tech"}))

	var count int
	k.IterateScrapedSources(ctx, func(_ *types.ScrapedSourceEntry) bool {
		count++
		return false
	})
	require.Equal(t, 2, count)
}
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestSetGetTrainingDemand|TestIterateTrainingDemands|TestSetGetDataBounty|TestDeleteDataBounty|TestGetActiveBounties|TestNextBountyID|TestSetGetScrapedSource|TestDeleteScrapedSource|TestGetScrapedSourcePenalty|TestIterateScrapedSources" -v`
Expected: PASS (all 10 tests pass since CRUD was implemented in Task 1)

**Step 3: Commit**

```bash
git add x/knowledge/keeper/state_test.go
git commit -m "test(knowledge): CRUD tests for training demand, bounties, scraped sources (R37-5)"
```

---

### Task 3: Domain Management — ProposeDomain & EndorseDomainProposal

**Files:**
- Create: `x/knowledge/keeper/domain.go`
- Modify: `x/knowledge/keeper/msg_server.go:54-59` (wire ProposeDomain, EndorseDomainProposal)
- Modify: `x/knowledge/types/msgs.go` (add ValidateBasic for ProposeDomain, EndorseDomainProposal)

**Step 1: Add ValidateBasic for domain messages**

Add to `x/knowledge/types/msgs.go`:

```go
// ValidateBasic performs stateless validation for MsgProposeDomain.
func (m *MsgProposeDomain) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return fmt.Errorf("invalid proposer address: %w", err)
	}
	if len(m.Name) == 0 {
		return ErrInvalidDomain.Wrap("domain name must not be empty")
	}
	if len(m.Name) > 64 {
		return ErrInvalidDomain.Wrap("domain name must be <= 64 characters")
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgEndorseDomainProposal.
func (m *MsgEndorseDomainProposal) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Endorser); err != nil {
		return fmt.Errorf("invalid endorser address: %w", err)
	}
	if len(m.ProposalId) == 0 {
		return ErrDomainNotFound.Wrap("proposal_id must not be empty")
	}
	return nil
}
```

**Step 2: Create domain.go with keeper business logic**

Create `x/knowledge/keeper/domain.go`:

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// MinDomainEndorsements is the number of endorsements required to activate a proposed domain.
	MinDomainEndorsements = 3
)

// ProposeDomain creates a new domain proposal. The domain starts in PROPOSED status
// and activates when it receives MinDomainEndorsements endorsements.
func (k Keeper) ProposeDomain(ctx context.Context, msg *types.MsgProposeDomain) (*types.MsgProposeDomainResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check domain doesn't already exist
	if _, found := k.GetDomain(ctx, msg.Name); found {
		return nil, types.ErrDomainExists.Wrapf("domain %q already exists", msg.Name)
	}

	domain := &types.Domain{
		Name:           msg.Name,
		Description:    msg.Description,
		Status:         types.DomainStatus_DOMAIN_STATUS_PROPOSED,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
		Proposer:       msg.Proposer,
		Endorsers:      []string{msg.Proposer}, // proposer auto-endorses
		Stratum:        msg.Stratum,
		Depth:          1,
	}

	if err := k.SetDomain(ctx, domain); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"propose_domain",
		sdk.NewAttribute("domain", msg.Name),
		sdk.NewAttribute("proposer", msg.Proposer),
	))

	return &types.MsgProposeDomainResponse{ProposalId: msg.Name}, nil
}

// EndorseDomainProposal adds an endorsement to a proposed domain.
// If the endorsement count reaches MinDomainEndorsements, the domain activates.
func (k Keeper) EndorseDomainProposal(ctx context.Context, msg *types.MsgEndorseDomainProposal) (*types.MsgEndorseDomainProposalResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	domain, found := k.GetDomain(ctx, msg.ProposalId)
	if !found {
		return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", msg.ProposalId)
	}

	if domain.Status != types.DomainStatus_DOMAIN_STATUS_PROPOSED {
		return nil, types.ErrInvalidDomain.Wrapf("domain %q is not in PROPOSED status", msg.ProposalId)
	}

	// Check for duplicate endorsement
	for _, e := range domain.Endorsers {
		if e == msg.Endorser {
			return nil, types.ErrInvalidDomain.Wrap("already endorsed")
		}
	}

	domain.Endorsers = append(domain.Endorsers, msg.Endorser)

	// Activate if enough endorsements
	if len(domain.Endorsers) >= MinDomainEndorsements {
		domain.Status = types.DomainStatus_DOMAIN_STATUS_ACTIVE
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"activate_domain",
			sdk.NewAttribute("domain", domain.Name),
			sdk.NewAttribute("endorsement_count", fmt.Sprintf("%d", len(domain.Endorsers))),
		))
	}

	if err := k.SetDomain(ctx, domain); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"endorse_domain",
		sdk.NewAttribute("domain", msg.ProposalId),
		sdk.NewAttribute("endorser", msg.Endorser),
	))

	return &types.MsgEndorseDomainProposalResponse{}, nil
}
```

Note: add `"fmt"` to imports.

**Step 3: Wire msg_server.go**

Replace the stubs in `x/knowledge/keeper/msg_server.go`:

```go
func (m msgServer) ProposeDomain(ctx context.Context, msg *types.MsgProposeDomain) (*types.MsgProposeDomainResponse, error) {
	return m.keeper.ProposeDomain(ctx, msg)
}

func (m msgServer) EndorseDomainProposal(ctx context.Context, msg *types.MsgEndorseDomainProposal) (*types.MsgEndorseDomainProposalResponse, error) {
	return m.keeper.EndorseDomainProposal(ctx, msg)
}
```

**Step 4: Run build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/...`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/domain.go x/knowledge/keeper/msg_server.go x/knowledge/types/msgs.go
git commit -m "feat(knowledge): domain proposal and endorsement logic (R37-5)"
```

---

### Task 4: Domain Management Tests

**Files:**
- Create: `x/knowledge/keeper/domain_test.go`

**Step 1: Write domain tests**

Create `x/knowledge/keeper/domain_test.go`:

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestProposeDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer:    testAddr,
		Name:        "mathematics",
		Description: "Pure and applied mathematics",
		Stratum:     "empirical",
	})
	require.NoError(t, err)
	require.Equal(t, "mathematics", resp.ProposalId)

	domain, found := k.GetDomain(ctx, "mathematics")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_PROPOSED, domain.Status)
	require.Equal(t, testAddr, domain.Proposer)
	require.Len(t, domain.Endorsers, 1) // proposer auto-endorses
}

func TestProposeDomain_AlreadyExists(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "technology", // already exists
	})
	require.ErrorIs(t, err, types.ErrDomainExists)
}

func TestEndorseDomainProposal(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Propose domain
	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "mathematics",
	})
	require.NoError(t, err)

	// First endorsement (second total — proposer is first)
	addr2 := "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5l3m3s"
	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   addr2,
		ProposalId: "mathematics",
	})
	require.NoError(t, err)

	domain, _ := k.GetDomain(ctx, "mathematics")
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_PROPOSED, domain.Status)
	require.Len(t, domain.Endorsers, 2)
}

func TestEndorseDomainProposal_ActivatesAt3(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "mathematics",
	})
	require.NoError(t, err)

	// 2nd endorsement
	addr2 := "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5l3m3s"
	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   addr2,
		ProposalId: "mathematics",
	})
	require.NoError(t, err)

	// 3rd endorsement → should activate
	addr3 := "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzp24dk"
	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   addr3,
		ProposalId: "mathematics",
	})
	require.NoError(t, err)

	domain, _ := k.GetDomain(ctx, "mathematics")
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, domain.Status)
	require.Len(t, domain.Endorsers, 3)
}

func TestEndorseDomainProposal_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   testAddr,
		ProposalId: "nonexistent",
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestEndorseDomainProposal_DuplicateEndorsement(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "mathematics",
	})
	require.NoError(t, err)

	// Proposer tries to endorse again
	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   testAddr,
		ProposalId: "mathematics",
	})
	require.ErrorIs(t, err, types.ErrInvalidDomain)
}

func TestEndorseDomainProposal_AlreadyActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx) // creates active domains

	_, err := k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   testAddr,
		ProposalId: "technology",
	})
	require.ErrorIs(t, err, types.ErrInvalidDomain)
}
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestProposeDomain|TestEndorseDomain" -v`
Expected: PASS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/domain_test.go
git commit -m "test(knowledge): domain proposal and endorsement tests (R37-5)"
```

---

### Task 5: Training Demand & Auto-Bounty Logic

**Files:**
- Create: `x/knowledge/keeper/demand.go`
- Modify: `x/knowledge/keeper/msg_server.go:74-75` (wire ReportDemand)
- Modify: `x/knowledge/types/msgs.go` (add ValidateBasic for MsgReportDemand)

**Step 1: Add ValidateBasic for MsgReportDemand**

Add to `x/knowledge/types/msgs.go`:

```go
// ValidateBasic performs stateless validation for MsgReportDemand.
func (m *MsgReportDemand) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Reporter); err != nil {
		return fmt.Errorf("invalid reporter address: %w", err)
	}
	if len(m.Reports) == 0 {
		return ErrInvalidSubmission.Wrap("reports must not be empty")
	}
	for i, r := range m.Reports {
		if len(r.Domain) == 0 {
			return ErrInvalidDomain.Wrapf("report[%d]: domain must not be empty", i)
		}
		if len(r.Subject) == 0 {
			return ErrInvalidSubmission.Wrapf("report[%d]: subject must not be empty", i)
		}
	}
	return nil
}
```

**Step 2: Create demand.go**

Create `x/knowledge/keeper/demand.go`:

```go
package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// DefaultBountyExpiryBlocks is the default number of blocks before an auto-bounty expires.
	DefaultBountyExpiryBlocks = 100_000
)

// ReportDemand processes training demand reports from authorized reporters.
// Only the module authority (governance) can report demand.
func (k Keeper) ReportDemand(ctx context.Context, msg *types.MsgReportDemand) (*types.MsgReportDemandResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Only authority can report demand
	if msg.Reporter != k.authority {
		return nil, types.ErrUnauthorized.Wrap("only governance authority can report demand")
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	for _, report := range msg.Reports {
		// Verify domain exists
		if _, found := k.GetDomain(ctx, report.Domain); !found {
			return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", report.Domain)
		}

		// Upsert training demand
		demand, found := k.GetTrainingDemand(ctx, report.Domain, report.Subject)
		if !found {
			demand = &types.TrainingDemand{
				Domain:  report.Domain,
				Subject: report.Subject,
			}
		}

		demand.QueryCount += report.Queries
		demand.FulfilledCount += report.Fulfilled
		demand.UnfulfilledCount += report.Unfulfilled
		demand.LastQueryBlock = uint64(sdkCtx.BlockHeight())
		demand.EpochQueryCount += report.Queries
		demand.EpochUnfulfilled += report.Unfulfilled

		if err := k.SetTrainingDemand(ctx, demand); err != nil {
			return nil, err
		}

		// Check if auto-bounty threshold reached
		k.checkAndCreateAutoBounty(ctx, demand, params, sdkCtx)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"report_demand",
		sdk.NewAttribute("reporter", msg.Reporter),
		sdk.NewAttribute("report_count", fmt.Sprintf("%d", len(msg.Reports))),
	))

	return &types.MsgReportDemandResponse{}, nil
}

// checkAndCreateAutoBounty creates an auto-bounty if unfulfilled demand exceeds threshold.
func (k Keeper) checkAndCreateAutoBounty(ctx context.Context, demand *types.TrainingDemand, params *types.Params, sdkCtx sdk.Context) {
	if params.AutoBountyThreshold == 0 || params.AutoBountyAmount == "" || params.AutoBountyAmount == "0" {
		return
	}

	if demand.UnfulfilledCount < uint64(params.AutoBountyThreshold) {
		return
	}

	// Check if a bounty already exists for this domain/subject
	bounties := k.GetActiveBounties(ctx, demand.Domain)
	for _, b := range bounties {
		if b.Subject == demand.Subject {
			return // bounty already exists
		}
	}

	bountyID := k.NextBountyID(ctx)
	bounty := &types.DataBounty{
		Id:             bountyID,
		Domain:         demand.Domain,
		Subject:        demand.Subject,
		RewardAmount:   params.AutoBountyAmount,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
		ExpiresAtBlock: uint64(sdkCtx.BlockHeight()) + DefaultBountyExpiryBlocks,
		DemandCount:    demand.UnfulfilledCount,
	}

	if err := k.SetDataBounty(ctx, bounty); err != nil {
		return
	}
	_ = k.SetBountyDomainIndex(ctx, demand.Domain, bountyID)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"auto_bounty_created",
		sdk.NewAttribute("bounty_id", bountyID),
		sdk.NewAttribute("domain", demand.Domain),
		sdk.NewAttribute("subject", demand.Subject),
		sdk.NewAttribute("reward", params.AutoBountyAmount),
	))
}
```

**Step 3: Wire msg_server.go**

Replace the stub in `x/knowledge/keeper/msg_server.go`:

```go
func (m msgServer) ReportDemand(ctx context.Context, msg *types.MsgReportDemand) (*types.MsgReportDemandResponse, error) {
	return m.keeper.ReportDemand(ctx, msg)
}
```

**Step 4: Run build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/...`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/demand.go x/knowledge/keeper/msg_server.go x/knowledge/types/msgs.go
git commit -m "feat(knowledge): training demand reporting with auto-bounty generation (R37-5)"
```

---

### Task 6: Training Demand & Auto-Bounty Tests

**Files:**
- Create: `x/knowledge/keeper/demand_test.go`

**Step 1: Write demand tests**

Create `x/knowledge/keeper/demand_test.go`:

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestReportDemand_AuthorizedReporter(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority", // matches keeper authority
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum_computing", Queries: 50, Fulfilled: 10, Unfulfilled: 40},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	demand, found := k.GetTrainingDemand(ctx, "science", "quantum_computing")
	require.True(t, found)
	require.Equal(t, uint64(50), demand.QueryCount)
	require.Equal(t, uint64(40), demand.UnfulfilledCount)
}

func TestReportDemand_UnauthorizedReporter(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: testAddr, // not the authority
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum", Queries: 10, Unfulfilled: 10},
		},
	})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestReportDemand_DomainNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "nonexistent", Subject: "topic", Queries: 10, Unfulfilled: 10},
		},
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestReportDemand_UpsertAccumulates(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	// First report
	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "physics", Queries: 30, Fulfilled: 10, Unfulfilled: 20},
		},
	})
	require.NoError(t, err)

	// Second report (accumulates)
	_, err = k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "physics", Queries: 20, Fulfilled: 5, Unfulfilled: 15},
		},
	})
	require.NoError(t, err)

	demand, found := k.GetTrainingDemand(ctx, "science", "physics")
	require.True(t, found)
	require.Equal(t, uint64(50), demand.QueryCount)
	require.Equal(t, uint64(15), demand.FulfilledCount)
	require.Equal(t, uint64(35), demand.UnfulfilledCount)
}

func TestReportDemand_AutoBountyCreated(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	// Set params with low threshold for testing
	params := types.DefaultParams()
	params.AutoBountyThreshold = 50
	require.NoError(t, k.SetParams(ctx, &params))

	// Report demand exceeding threshold
	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum", Queries: 100, Fulfilled: 20, Unfulfilled: 80},
		},
	})
	require.NoError(t, err)

	// Should have created a bounty
	bounties := k.GetActiveBounties(ctx, "science")
	require.Len(t, bounties, 1)
	require.Equal(t, "science", bounties[0].Domain)
	require.Equal(t, "quantum", bounties[0].Subject)
	require.Equal(t, params.AutoBountyAmount, bounties[0].RewardAmount)
}

func TestReportDemand_NoDuplicateAutoBounty(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	params := types.DefaultParams()
	params.AutoBountyThreshold = 10
	require.NoError(t, k.SetParams(ctx, &params))

	// Report twice, both exceeding threshold
	for i := 0; i < 2; i++ {
		_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
			Reporter: "authority",
			Reports: []*types.DemandReport{
				{Domain: "science", Subject: "quantum", Queries: 50, Unfulfilled: 50},
			},
		})
		require.NoError(t, err)
	}

	// Should have only one bounty, not two
	bounties := k.GetActiveBounties(ctx, "science")
	require.Len(t, bounties, 1)
}

func TestReportDemand_BelowThresholdNoBounty(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	// Default threshold is 100
	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum", Queries: 50, Unfulfilled: 30},
		},
	})
	require.NoError(t, err)

	bounties := k.GetActiveBounties(ctx, "science")
	require.Len(t, bounties, 0)
}

func TestReportDemand_MultipleReports(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "physics", Queries: 10, Unfulfilled: 5},
			{Domain: "technology", Subject: "golang", Queries: 20, Unfulfilled: 15},
		},
	})
	require.NoError(t, err)

	d1, found := k.GetTrainingDemand(ctx, "science", "physics")
	require.True(t, found)
	require.Equal(t, uint64(10), d1.QueryCount)

	d2, found := k.GetTrainingDemand(ctx, "technology", "golang")
	require.True(t, found)
	require.Equal(t, uint64(20), d2.QueryCount)
}
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestReportDemand" -v`
Expected: PASS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/demand_test.go
git commit -m "test(knowledge): training demand and auto-bounty tests (R37-5)"
```

---

### Task 7: Manual Bounty Funding & Bounty Fulfillment

**Files:**
- Modify: `x/knowledge/keeper/demand.go` (add FundBounty, CheckBountyFulfillment)
- Modify: `x/knowledge/keeper/msg_server.go:78-79` (wire FundBounty)

**Step 1: Add FundBounty and CheckBountyFulfillment to demand.go**

Add to `x/knowledge/keeper/demand.go`:

```go
// FundBounty creates or adds to a data bounty. Transfers funds from funder to module account.
func (k Keeper) FundBounty(ctx context.Context, msg *types.MsgFundBounty) (*types.MsgFundBountyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Verify domain exists
	if _, found := k.GetDomain(ctx, msg.Domain); !found {
		return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", msg.Domain)
	}

	// Parse amount
	amount, ok := sdk.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, types.ErrInsufficientPayment.Wrap("invalid amount")
	}

	// Transfer from funder to module
	funderAddr, err := sdk.AccAddressFromBech32(msg.Funder)
	if err != nil {
		return nil, err
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, funderAddr, types.ModuleName, coins); err != nil {
		return nil, types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// Create bounty
	bountyID := k.NextBountyID(ctx)
	expiryBlocks := msg.ExpiresBlocks
	if expiryBlocks == 0 {
		expiryBlocks = DefaultBountyExpiryBlocks
	}

	bounty := &types.DataBounty{
		Id:             bountyID,
		Domain:         msg.Domain,
		Subject:        msg.Topic,
		RewardAmount:   msg.Amount,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
		ExpiresAtBlock: uint64(sdkCtx.BlockHeight()) + expiryBlocks,
	}

	if err := k.SetDataBounty(ctx, bounty); err != nil {
		return nil, err
	}
	_ = k.SetBountyDomainIndex(ctx, msg.Domain, bountyID)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"fund_bounty",
		sdk.NewAttribute("bounty_id", bountyID),
		sdk.NewAttribute("funder", msg.Funder),
		sdk.NewAttribute("domain", msg.Domain),
		sdk.NewAttribute("amount", msg.Amount),
	))

	return &types.MsgFundBountyResponse{BountyId: bountyID}, nil
}

// CheckBountyFulfillment checks if a newly created sample fulfills any active bounties.
// Called after a sample is promoted from submission.
func (k Keeper) CheckBountyFulfillment(ctx context.Context, sample *types.Sample) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	bounties := k.GetActiveBounties(ctx, sample.Domain)

	for _, bounty := range bounties {
		if !matchesBounty(sample, bounty) {
			continue
		}

		// Check not expired
		if uint64(sdkCtx.BlockHeight()) > bounty.ExpiresAtBlock {
			continue
		}

		// Mark bounty as claimed
		bounty.Claimed = true
		bounty.ClaimedBySampleId = sample.Id
		_ = k.SetDataBounty(ctx, bounty)

		// Transfer reward to submitter
		amount, ok := sdk.NewIntFromString(bounty.RewardAmount)
		if ok && amount.IsPositive() {
			submitterAddr, err := sdk.AccAddressFromBech32(sample.Submitter)
			if err == nil {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins)
			}
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"bounty_fulfilled",
			sdk.NewAttribute("bounty_id", bounty.Id),
			sdk.NewAttribute("sample_id", sample.Id),
			sdk.NewAttribute("submitter", sample.Submitter),
			sdk.NewAttribute("reward", bounty.RewardAmount),
		))

		break // Only claim one bounty per sample
	}
}

// matchesBounty checks if a sample matches a bounty's requirements.
func matchesBounty(sample *types.Sample, bounty *types.DataBounty) bool {
	if sample.Domain != bounty.Domain {
		return false
	}
	// If bounty has a specific subject, check for topic match
	if bounty.Subject != "" {
		found := false
		for _, topic := range sample.Topics {
			if topic == bounty.Subject {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
```

**Step 2: Wire FundBounty in msg_server.go**

```go
func (m msgServer) FundBounty(ctx context.Context, msg *types.MsgFundBounty) (*types.MsgFundBountyResponse, error) {
	return m.keeper.FundBounty(ctx, msg)
}
```

**Step 3: Run build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/...`
Expected: PASS

**Step 4: Commit**

```bash
git add x/knowledge/keeper/demand.go x/knowledge/keeper/msg_server.go
git commit -m "feat(knowledge): manual bounty funding and bounty fulfillment (R37-5)"
```

---

### Task 8: Bounty Funding & Fulfillment Tests

**Files:**
- Modify: `x/knowledge/keeper/demand_test.go` (add bounty tests)

**Step 1: Add bounty tests**

Append to `x/knowledge/keeper/demand_test.go`:

```go
// ─── Manual Bounty Funding ──────────────────────────────────────────────────

func TestFundBounty(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder:       testAddr,
		Domain:       "science",
		Topic:        "quantum",
		Amount:       "5000000",
		ExpiresBlocks: 50000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.BountyId)

	bounty, found := k.GetDataBounty(ctx, resp.BountyId)
	require.True(t, found)
	require.Equal(t, "science", bounty.Domain)
	require.Equal(t, "quantum", bounty.Subject)
	require.Equal(t, "5000000", bounty.RewardAmount)
	require.False(t, bounty.Claimed)
}

func TestFundBounty_DomainNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "nonexistent",
		Topic:  "topic",
		Amount: "1000000",
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestFundBounty_InvalidAmount(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "science",
		Topic:  "topic",
		Amount: "0",
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestFundBounty_InsufficientFunds(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	bk.failNextSend = true
	_, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "science",
		Topic:  "topic",
		Amount: "1000000",
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestFundBounty_DefaultExpiry(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "science",
		Topic:  "topic",
		Amount: "1000000",
		// ExpiresBlocks: 0 → should use default
	})
	require.NoError(t, err)

	bounty, found := k.GetDataBounty(ctx, resp.BountyId)
	require.True(t, found)
	require.Equal(t, uint64(100)+100_000, bounty.ExpiresAtBlock) // blockHeight(100) + default
}

// ─── Bounty Fulfillment ─────────────────────────────────────────────────────

func TestCheckBountyFulfillment_Match(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Create a bounty
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id:             "b1",
		Domain:         "science",
		Subject:        "quantum",
		RewardAmount:   "5000000",
		ExpiresAtBlock: 10000,
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	// Create a matching sample
	sample := &types.Sample{
		Id:        "s1",
		Domain:    "science",
		Submitter: testAddr,
		Topics:    []string{"quantum", "physics"},
	}
	k.CheckBountyFulfillment(ctx, sample)

	// Bounty should be claimed
	bounty, found := k.GetDataBounty(ctx, "b1")
	require.True(t, found)
	require.True(t, bounty.Claimed)
	require.Equal(t, "s1", bounty.ClaimedBySampleId)

	// Reward should have been sent
	require.Len(t, bk.moduleToAccountCalls, 1)
}

func TestCheckBountyFulfillment_NoMatch(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id:             "b1",
		Domain:         "science",
		Subject:        "quantum",
		RewardAmount:   "5000000",
		ExpiresAtBlock: 10000,
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	// Sample in different domain
	sample := &types.Sample{
		Id:        "s1",
		Domain:    "technology",
		Submitter: testAddr,
		Topics:    []string{"quantum"},
	}
	k.CheckBountyFulfillment(ctx, sample)

	// Bounty should NOT be claimed
	bounty, _ := k.GetDataBounty(ctx, "b1")
	require.False(t, bounty.Claimed)
	require.Len(t, bk.moduleToAccountCalls, 0)
}

func TestCheckBountyFulfillment_Expired(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Bounty expired (block 100, expiry at 50)
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id:             "b1",
		Domain:         "science",
		Subject:        "quantum",
		RewardAmount:   "5000000",
		ExpiresAtBlock: 50, // already expired at block 100
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	sample := &types.Sample{
		Id:        "s1",
		Domain:    "science",
		Submitter: testAddr,
		Topics:    []string{"quantum"},
	}
	k.CheckBountyFulfillment(ctx, sample)

	bounty, _ := k.GetDataBounty(ctx, "b1")
	require.False(t, bounty.Claimed)
	require.Len(t, bk.moduleToAccountCalls, 0)
}

func TestCheckBountyFulfillment_SubjectEmpty(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Bounty with empty subject matches any sample in domain
	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id:             "b1",
		Domain:         "science",
		Subject:        "", // matches any
		RewardAmount:   "5000000",
		ExpiresAtBlock: 10000,
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	sample := &types.Sample{
		Id:        "s1",
		Domain:    "science",
		Submitter: testAddr,
	}
	k.CheckBountyFulfillment(ctx, sample)

	bounty, _ := k.GetDataBounty(ctx, "b1")
	require.True(t, bounty.Claimed)
	require.Len(t, bk.moduleToAccountCalls, 1)
}
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestFundBounty|TestCheckBountyFulfillment" -v`
Expected: PASS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/demand_test.go
git commit -m "test(knowledge): bounty funding and fulfillment tests (R37-5)"
```

---

### Task 9: Scraped Source Registry

**Files:**
- Modify: `x/knowledge/keeper/msg_server.go:90-96` (wire AddScrapedSource, RemoveScrapedSource)
- Modify: `x/knowledge/types/msgs.go` (add ValidateBasic for scraped source messages)
- Create: `x/knowledge/keeper/scraped_source.go` (keeper logic)

**Step 1: Add ValidateBasic for scraped source messages**

Add to `x/knowledge/types/msgs.go`:

```go
// ValidateBasic performs stateless validation for MsgAddScrapedSource.
func (m *MsgAddScrapedSource) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if len(m.Platform) == 0 {
		return ErrInvalidSubmission.Wrap("platform must not be empty")
	}
	if len(m.Domain) == 0 {
		return ErrInvalidDomain.Wrap("domain must not be empty")
	}
	if m.NoveltyPenalty > MaxBPS {
		return ErrInvalidQualityScore.Wrapf("novelty_penalty %d exceeds max BPS %d", m.NoveltyPenalty, MaxBPS)
	}
	return nil
}

// ValidateBasic performs stateless validation for MsgRemoveScrapedSource.
func (m *MsgRemoveScrapedSource) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if len(m.Id) == 0 {
		return ErrInvalidSubmission.Wrap("id must not be empty")
	}
	return nil
}
```

**Step 2: Create scraped_source.go**

Create `x/knowledge/keeper/scraped_source.go`:

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// AddScrapedSource registers a platform/domain as heavily scraped. Authority-only.
func (k Keeper) AddScrapedSource(ctx context.Context, msg *types.MsgAddScrapedSource) (*types.MsgAddScrapedSourceResponse, error) {
	if msg.Authority != k.authority {
		return nil, types.ErrUnauthorized.Wrap("only governance authority can add scraped sources")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	id := msg.Platform + "/" + msg.Domain
	entry := &types.ScrapedSourceEntry{
		Id:            id,
		Platform:      msg.Platform,
		Domain:        msg.Domain,
		Description:   msg.Description,
		NoveltyPenalty: msg.NoveltyPenalty,
		AddedBlock:    uint64(sdkCtx.BlockHeight()),
	}

	if err := k.SetScrapedSource(ctx, entry); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"add_scraped_source",
		sdk.NewAttribute("id", id),
		sdk.NewAttribute("platform", msg.Platform),
		sdk.NewAttribute("domain", msg.Domain),
	))

	return &types.MsgAddScrapedSourceResponse{Id: id}, nil
}

// RemoveScrapedSource removes a scraped source entry. Authority-only.
func (k Keeper) RemoveScrapedSource(ctx context.Context, msg *types.MsgRemoveScrapedSource) (*types.MsgRemoveScrapedSourceResponse, error) {
	if msg.Authority != k.authority {
		return nil, types.ErrUnauthorized.Wrap("only governance authority can remove scraped sources")
	}

	_, found := k.GetScrapedSource(ctx, msg.Id)
	if !found {
		return nil, types.ErrSampleNotFound.Wrapf("scraped source %q not found", msg.Id)
	}

	if err := k.DeleteScrapedSource(ctx, msg.Id); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"remove_scraped_source",
		sdk.NewAttribute("id", msg.Id),
	))

	return &types.MsgRemoveScrapedSourceResponse{}, nil
}
```

**Step 3: Wire msg_server.go**

Replace stubs:

```go
func (m msgServer) AddScrapedSource(ctx context.Context, msg *types.MsgAddScrapedSource) (*types.MsgAddScrapedSourceResponse, error) {
	return m.keeper.AddScrapedSource(ctx, msg)
}

func (m msgServer) RemoveScrapedSource(ctx context.Context, msg *types.MsgRemoveScrapedSource) (*types.MsgRemoveScrapedSourceResponse, error) {
	return m.keeper.RemoveScrapedSource(ctx, msg)
}
```

**Step 4: Run build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/...`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/scraped_source.go x/knowledge/keeper/msg_server.go x/knowledge/types/msgs.go
git commit -m "feat(knowledge): scraped source registry with authority gating (R37-5)"
```

---

### Task 10: Scraped Source Tests

**Files:**
- Create: `x/knowledge/keeper/scraped_source_test.go`

**Step 1: Write scraped source tests**

Create `x/knowledge/keeper/scraped_source_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestAddScrapedSource(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:     "authority",
		Platform:      "reddit",
		Domain:        "science",
		Description:   "Reddit r/science heavily scraped",
		NoveltyPenalty: 200000,
	})
	require.NoError(t, err)
	require.Equal(t, "reddit/science", resp.Id)

	entry, found := k.GetScrapedSource(ctx, "reddit/science")
	require.True(t, found)
	require.Equal(t, "reddit", entry.Platform)
	require.Equal(t, uint64(200000), entry.NoveltyPenalty)
	require.Equal(t, uint64(100), entry.AddedBlock)
}

func TestAddScrapedSource_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:     testAddr, // not the authority
		Platform:      "reddit",
		Domain:        "science",
		NoveltyPenalty: 200000,
	})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestAddScrapedSource_Upsert(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Add with penalty 200000
	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:     "authority",
		Platform:      "reddit",
		Domain:        "science",
		NoveltyPenalty: 200000,
	})
	require.NoError(t, err)

	// Update penalty to 300000
	_, err = k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:     "authority",
		Platform:      "reddit",
		Domain:        "science",
		NoveltyPenalty: 300000,
	})
	require.NoError(t, err)

	entry, found := k.GetScrapedSource(ctx, "reddit/science")
	require.True(t, found)
	require.Equal(t, uint64(300000), entry.NoveltyPenalty)
}

func TestRemoveScrapedSource(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Add first
	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:     "authority",
		Platform:      "reddit",
		Domain:        "science",
		NoveltyPenalty: 200000,
	})
	require.NoError(t, err)

	// Remove
	_, err = k.RemoveScrapedSource(ctx, &types.MsgRemoveScrapedSource{
		Authority: "authority",
		Id:        "reddit/science",
	})
	require.NoError(t, err)

	_, found := k.GetScrapedSource(ctx, "reddit/science")
	require.False(t, found)
}

func TestRemoveScrapedSource_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RemoveScrapedSource(ctx, &types.MsgRemoveScrapedSource{
		Authority: testAddr,
		Id:        "reddit/science",
	})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestRemoveScrapedSource_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RemoveScrapedSource(ctx, &types.MsgRemoveScrapedSource{
		Authority: "authority",
		Id:        "nonexistent",
	})
	require.Error(t, err)
}

func TestScrapedSourcePenaltyIntegration(t *testing.T) {
	k, ctx := setupKeeper(t)

	// No penalty for unknown source
	require.Equal(t, uint64(0), k.GetScrapedSourcePenalty(ctx, "reddit", "science"))

	// Add source
	_, err := k.AddScrapedSource(ctx, &types.MsgAddScrapedSource{
		Authority:     "authority",
		Platform:      "stackoverflow",
		Domain:        "technology",
		NoveltyPenalty: 350000,
	})
	require.NoError(t, err)

	require.Equal(t, uint64(350000), k.GetScrapedSourcePenalty(ctx, "stackoverflow", "technology"))
	require.Equal(t, uint64(0), k.GetScrapedSourcePenalty(ctx, "stackoverflow", "science"))
}
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestAddScrapedSource|TestRemoveScrapedSource|TestScrapedSourcePenalty" -v`
Expected: PASS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/scraped_source_test.go
git commit -m "test(knowledge): scraped source registry tests (R37-5)"
```

---

### Task 11: Final Integration — Run All Tests & Verify Coverage

**Step 1: Run all knowledge module tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -v -count=1`
Expected: PASS (all tests)

**Step 2: Count tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -v -count=1 2>&1 | grep -c "^--- PASS"`
Expected: ≥ 25 tests

**Step 3: Run full build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: PASS

**Step 4: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "feat(knowledge): complete domain management, demand tracking, bounties, scraped sources (R37-5)"
```

---

## Test Inventory (≥ 25 tests)

| # | Test | File |
|---|------|------|
| 1 | TestSetGetTrainingDemand | state_test.go |
| 2 | TestIterateTrainingDemands | state_test.go |
| 3 | TestSetGetDataBounty | state_test.go |
| 4 | TestDeleteDataBounty | state_test.go |
| 5 | TestGetActiveBounties | state_test.go |
| 6 | TestNextBountyID | state_test.go |
| 7 | TestSetGetScrapedSource | state_test.go |
| 8 | TestDeleteScrapedSource | state_test.go |
| 9 | TestGetScrapedSourcePenalty | state_test.go |
| 10 | TestIterateScrapedSources | state_test.go |
| 11 | TestProposeDomain | domain_test.go |
| 12 | TestProposeDomain_AlreadyExists | domain_test.go |
| 13 | TestEndorseDomainProposal | domain_test.go |
| 14 | TestEndorseDomainProposal_ActivatesAt3 | domain_test.go |
| 15 | TestEndorseDomainProposal_NotFound | domain_test.go |
| 16 | TestEndorseDomainProposal_DuplicateEndorsement | domain_test.go |
| 17 | TestEndorseDomainProposal_AlreadyActive | domain_test.go |
| 18 | TestReportDemand_AuthorizedReporter | demand_test.go |
| 19 | TestReportDemand_UnauthorizedReporter | demand_test.go |
| 20 | TestReportDemand_DomainNotFound | demand_test.go |
| 21 | TestReportDemand_UpsertAccumulates | demand_test.go |
| 22 | TestReportDemand_AutoBountyCreated | demand_test.go |
| 23 | TestReportDemand_NoDuplicateAutoBounty | demand_test.go |
| 24 | TestReportDemand_BelowThresholdNoBounty | demand_test.go |
| 25 | TestReportDemand_MultipleReports | demand_test.go |
| 26 | TestFundBounty | demand_test.go |
| 27 | TestFundBounty_DomainNotFound | demand_test.go |
| 28 | TestFundBounty_InvalidAmount | demand_test.go |
| 29 | TestFundBounty_InsufficientFunds | demand_test.go |
| 30 | TestFundBounty_DefaultExpiry | demand_test.go |
| 31 | TestCheckBountyFulfillment_Match | demand_test.go |
| 32 | TestCheckBountyFulfillment_NoMatch | demand_test.go |
| 33 | TestCheckBountyFulfillment_Expired | demand_test.go |
| 34 | TestCheckBountyFulfillment_SubjectEmpty | demand_test.go |
| 35 | TestAddScrapedSource | scraped_source_test.go |
| 36 | TestAddScrapedSource_Unauthorized | scraped_source_test.go |
| 37 | TestAddScrapedSource_Upsert | scraped_source_test.go |
| 38 | TestRemoveScrapedSource | scraped_source_test.go |
| 39 | TestRemoveScrapedSource_Unauthorized | scraped_source_test.go |
| 40 | TestRemoveScrapedSource_NotFound | scraped_source_test.go |
| 41 | TestScrapedSourcePenaltyIntegration | scraped_source_test.go |

**Total: 41 tests (target ≥ 25 ✓)**
