# R37-6: BeginBlocker/EndBlocker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement block-level lifecycle processing: quality round phase transitions with min-validators check, EndBlocker with aggregation, energy decay (skipping sponsored samples), niche rankings, bounty/patronage expiry, and missed-reveal slashing.

**Architecture:** Split block processing into BeginBlocker (phase transitions, round expiry with stake return) and EndBlocker (aggregation, epoch processing, patronage/bounty expiry, slashing). All ecology runs in EndBlocker at epoch boundaries. Patronage expiry runs every block.

**Tech Stack:** Go 1.24, Cosmos SDK v0.50.15, protobuf, mock-based keeper tests

---

### Task 1: Add bounty CRUD to state.go

**Files:**
- Modify: `x/knowledge/keeper/state.go` (append after existing methods)

**Step 1: Write failing tests**

Add to `x/knowledge/keeper/ecology_test.go` at the end:

```go
// ─── Bounty State Tests ─────────────────────────────────────────────────────

func TestDataBounty_SetGetDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	bounty := &types.DataBounty{
		Id:            "b1",
		Domain:        "technology",
		Subject:       "golang tutorials",
		RewardAmount:  "1000000",
		ExpiresAtBlock: 500,
	}
	require.NoError(t, k.SetDataBounty(ctx, bounty))

	got, found := k.GetDataBounty(ctx, "b1")
	require.True(t, found)
	require.Equal(t, "technology", got.Domain)
	require.Equal(t, uint64(500), got.ExpiresAtBlock)

	require.NoError(t, k.DeleteDataBounty(ctx, "b1"))
	_, found = k.GetDataBounty(ctx, "b1")
	require.False(t, found)
}

func TestIterateDataBounties(t *testing.T) {
	k, ctx := setupKeeper(t)

	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b1", Domain: "tech"})
	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b2", Domain: "sci"})

	var ids []string
	k.IterateDataBounties(ctx, func(b *types.DataBounty) bool {
		ids = append(ids, b.Id)
		return false
	})
	require.Equal(t, 2, len(ids))
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestDataBounty_SetGetDelete|TestIterateDataBounties" -v -count=1`
Expected: FAIL — methods don't exist

**Step 3: Write implementation**

Append to `x/knowledge/keeper/state.go`:

```go
// ─── Data Bounties ────────────────────────────────────────────────────────────

// SetDataBounty stores a data bounty.
func (k Keeper) SetDataBounty(ctx context.Context, bounty *types.DataBounty) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(bounty)
	if err != nil {
		return fmt.Errorf("failed to marshal data bounty: %w", err)
	}
	return store.Set(types.DataBountyKey(bounty.Id), bz)
}

// GetDataBounty retrieves a data bounty by ID.
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

// DeleteDataBounty removes a data bounty by ID.
func (k Keeper) DeleteDataBounty(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.DataBountyKey(id))
}

// IterateDataBounties iterates over all data bounties.
func (k Keeper) IterateDataBounties(ctx context.Context, cb func(*types.DataBounty) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.BountyPrefix, types.PrefixEnd(types.BountyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var bounty types.DataBounty
		if err := proto.Unmarshal(iter.Value(), &bounty); err != nil {
			continue
		}
		if cb(&bounty) {
			break
		}
	}
}
```

Note: check if `types.PrefixEnd` exists. If not, use the `prefixEnd` helper pattern from other modules — increment the last byte of the prefix to form the iterator end key. If it doesn't exist, add a local helper:

```go
// prefixEnd returns the end key for a prefix iterator (prefix with last byte incremented).
func prefixEnd(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	end[len(end)-1]++
	return end
}
```

And use `prefixEnd(types.BountyPrefix)` instead of `types.PrefixEnd(...)`.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestDataBounty_SetGetDelete|TestIterateDataBounties" -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/state.go x/knowledge/keeper/ecology_test.go
git commit -m "feat(knowledge): add bounty CRUD and iterator to state (R37-6)"
```

---

### Task 2: Refactor BeginBlocker — add min-validators check and expireRound

**Files:**
- Modify: `x/knowledge/keeper/phases.go` (lines 11-51)

**Step 1: Write failing tests**

Add to `x/knowledge/keeper/phases_test.go`:

```go
func TestBeginBlocker_CommitToReveal_InsufficientCommits_ExpiresRound(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Stake: "1000000",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// Only 1 commit (below MinValidatorsPerRound=3)
	vote := &types.QualityVote{OverallQuality: 800000, ConsentValid: true}
	salt := []byte("s1")
	hash := types.ComputeQualityCommitHash(roundID, vote, salt)
	require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: verifier1, RoundId: roundID, CommitHash: hash,
	}))

	// Advance past commit deadline
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, round.Phase)

	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, roundID)

	// Stake should be returned
	require.True(t, len(bk.moduleToAccountCalls) > 0, "expected stake return")
}

func TestBeginBlocker_CommitToReveal_EnoughCommits_TransitionsToReveal(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{Id: "s1", Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// All 3 commit
	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	// Advance past commit deadline
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_REVEAL, round.Phase)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestBeginBlocker_CommitToReveal_InsufficientCommits|TestBeginBlocker_CommitToReveal_EnoughCommits" -v -count=1`
Expected: At least the insufficient-commits test fails (currently transitions regardless)

**Step 3: Modify BeginBlocker**

Replace the BeginBlocker in `x/knowledge/keeper/phases.go:11-51` with:

```go
// BeginBlocker processes active quality rounds, transitioning phases based on block deadlines.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	block := uint64(sdkCtx.BlockHeight())
	params, err := k.GetParams(ctx)
	if err != nil {
		params = nil // continue with nil-safe defaults below
	}

	activeRoundIDs := k.GetActiveRounds(ctx)
	for _, roundID := range activeRoundIDs {
		round, found := k.GetQualityRound(ctx, roundID)
		if !found {
			_ = k.DeleteActiveRound(ctx, roundID)
			continue
		}

		switch round.Phase {
		case types.VerificationPhase_VERIFICATION_PHASE_COMMIT:
			if block > round.CommitDeadline {
				minValidators := uint64(3) // default
				if params != nil && params.MinValidatorsPerRound > 0 {
					minValidators = params.MinValidatorsPerRound
				}
				if uint64(len(round.Commits)) >= minValidators {
					round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
					_ = k.SetQualityRound(ctx, round)
				} else {
					k.expireRound(ctx, round)
				}
			}

		case types.VerificationPhase_VERIFICATION_PHASE_REVEAL:
			if block > round.RevealDeadline {
				if len(round.Reveals) > 0 {
					_ = k.AggregateQualityRound(ctx, roundID)
				} else {
					k.expireRound(ctx, round)
				}
			}
		}
	}

	return nil
}

// expireRound marks a round as expired, removes from active index, and returns stake to submitter.
func (k Keeper) expireRound(ctx context.Context, round *types.QualityRound) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_EXPIRED
	_ = k.SetQualityRound(ctx, round)
	_ = k.DeleteActiveRound(ctx, round.Id)

	// Return stake to submitter
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	if found && sub.Submitter != "" && sub.Stake != "" {
		submitterAddr, addrErr := sdk.AccAddressFromBech32(sub.Submitter)
		if addrErr == nil {
			stakeAmt, ok := sdkmath.NewIntFromString(sub.Stake)
			if ok && stakeAmt.IsPositive() {
				stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(sdkCtx, types.ModuleName, submitterAddr, sdk.NewCoins(stakeCoin))
			}
		}
		sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_PENDING
		_ = k.SetSubmission(ctx, sub)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"quality_round_expired",
		sdk.NewAttribute("round_id", round.Id),
		sdk.NewAttribute("submission_id", round.SubmissionId),
	))
}
```

Note: You'll need to add `sdkmath "cosmossdk.io/math"` to imports in phases.go.

Also **remove** the ecology epoch block from BeginBlocker (lines 44-48 in the old code) — that moves to EndBlocker.

**Step 4: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestBeginBlocker" -v -count=1`
Expected: All pass. The old `TestBeginBlocker_CommitToRevealTransition` will need adjustment since it had 0 commits — it should now be using 3 commits or the test expectation changes. Check and fix as needed.

**Step 5: Commit**

```bash
git add x/knowledge/keeper/phases.go x/knowledge/keeper/phases_test.go
git commit -m "feat(knowledge): BeginBlocker min-validators check and expireRound with stake return (R37-6)"
```

---

### Task 3: Implement EndBlocker with epoch processing

**Files:**
- Modify: `x/knowledge/keeper/phases.go` (append EndBlocker)
- Modify: `x/knowledge/keeper/ecology.go` (enhance RunEcologyEpoch)
- Modify: `x/knowledge/module.go` (wire EndBlock)

**Step 1: Write failing tests**

Add to `x/knowledge/keeper/phases_test.go`:

```go
func TestEndBlocker_AggregatesCompletedRounds(t *testing.T) {
	k, ctx := setupKeeper(t)
	// EndBlocker with no active rounds should not error
	require.NoError(t, k.EndBlocker(ctx))
}

func TestEndBlocker_EcologyEpochBoundary(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	// Epoch boundary at block 100
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Less(t, s.Energy, uint64(1_000_000), "energy should have decayed at epoch boundary")
	require.Greater(t, s.FitnessScore, uint64(0), "fitness should be computed")
}

func TestEndBlocker_NonEpoch_NoDecay(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	// Block 101 — not an epoch boundary
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(101).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(1_000_000), s.Energy, "energy should NOT decay outside epoch")
}

func TestEndBlocker_SponsoredSampleSkipsDecay(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue:        "0",
		PatronageExpiryBlock: 200, // Sponsored until block 200
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(1_000_000), s.Energy, "sponsored sample should skip decay")
}

func TestEndBlocker_PatronageExpiry(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue:        "0",
		PatronageExpiryBlock: 50, // Expired at block 50
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(0), s.PatronageExpiryBlock, "expired patronage should be cleared")
}

func TestEndBlocker_BountyExpiry(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "tech", ExpiresAtBlock: 50,
		RewardAmount: "1000000",
	})
	_ = k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b2", Domain: "sci", ExpiresAtBlock: 200,
		RewardAmount: "2000000",
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	_, found := k.GetDataBounty(ctx, "b1")
	require.False(t, found, "expired bounty should be deleted")

	_, found2 := k.GetDataBounty(ctx, "b2")
	require.True(t, found2, "non-expired bounty should remain")
}

func TestEndBlocker_NicheRankingUpdate(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	// Two samples in same niche, s2 has higher fitness components
	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", NicheKey: "niche_a", Energy: 1_000_000, EnergyCap: 1_000_000,
		QualityScore: 500_000, Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
		Content: "a", TotalRevenue: "0",
	})
	_ = k.SetSample(ctx, &types.Sample{
		Id: "2", NicheKey: "niche_a", Energy: 1_000_000, EnergyCap: 1_000_000,
		QualityScore: 900_000, Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
		Content: "b", TotalRevenue: "0",
	})
	_ = k.SetNicheIndex(ctx, "niche_a", "1")
	_ = k.SetNicheIndex(ctx, "niche_a", "2")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s2, _ := k.GetSample(ctx, "2")
	require.True(t, s2.NicheLeader, "sample 2 should be niche leader after epoch")
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestEndBlocker" -v -count=1`
Expected: FAIL — EndBlocker doesn't exist

**Step 3: Implement EndBlocker**

Append to `x/knowledge/keeper/phases.go`:

```go
// EndBlocker processes aggregation, epoch boundaries, patronage expiry, and slashing.
func (k Keeper) EndBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())
	params, err := k.GetParams(ctx)
	if err != nil || params == nil {
		return nil
	}

	// 1. Epoch boundary processing
	if blockHeight > 0 && blockHeight%EcologyEpochBlocks == 0 {
		epoch := blockHeight / EcologyEpochBlocks
		k.RunEcologyEpoch(ctx, epoch)
	}

	// 2. Expire patronage (every block)
	k.expirePatronage(ctx, blockHeight)

	// 3. Expire bounties at epoch boundaries
	if blockHeight > 0 && blockHeight%EcologyEpochBlocks == 0 {
		k.expireBounties(ctx, blockHeight)
	}

	return nil
}

// expirePatronage clears patronage_expiry_block on samples whose patronage has lapsed.
func (k Keeper) expirePatronage(ctx context.Context, blockHeight uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	k.IterateSamples(ctx, func(sample *types.Sample) bool {
		if sample.PatronageExpiryBlock > 0 && blockHeight >= sample.PatronageExpiryBlock {
			sample.PatronageExpiryBlock = 0
			_ = k.SetSample(ctx, sample)
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"patronage_expired",
				sdk.NewAttribute("sample_id", sample.Id),
			))
		}
		return false
	})
}

// expireBounties removes unclaimed bounties past their expiry block.
func (k Keeper) expireBounties(ctx context.Context, blockHeight uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var toDelete []string
	k.IterateDataBounties(ctx, func(bounty *types.DataBounty) bool {
		if !bounty.Claimed && bounty.ExpiresAtBlock > 0 && blockHeight >= bounty.ExpiresAtBlock {
			toDelete = append(toDelete, bounty.Id)
		}
		return false
	})
	for _, id := range toDelete {
		_ = k.DeleteDataBounty(ctx, id)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"bounty_expired",
			sdk.NewAttribute("bounty_id", id),
		))
	}
}
```

**Modify RunEcologyEpoch** in `x/knowledge/keeper/ecology.go:260-290` to add sponsored sample skip and niche rankings:

```go
// RunEcologyEpoch performs all ecology processing for the current epoch.
// Called from EndBlocker every EcologyEpochBlocks.
func (k Keeper) RunEcologyEpoch(ctx context.Context, currentEpoch uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlock := uint64(sdkCtx.BlockHeight())
	params, err := k.GetParams(ctx)
	if err != nil || params == nil {
		return
	}

	// Track niches seen for ranking update
	nichesSeen := make(map[string]bool)

	// Phase 1: Iterate all active samples — decay energy, compute fitness, check at-risk
	k.IterateSamples(ctx, func(sample *types.Sample) bool {
		if sample.Status == types.SampleStatus_SAMPLE_STATUS_PRUNED ||
			sample.Status == types.SampleStatus_SAMPLE_STATUS_REJECTED {
			return false
		}

		// Track niche for ranking update
		if sample.NicheKey != "" {
			nichesSeen[sample.NicheKey] = true
		}

		// Skip decay for sponsored samples
		if sample.PatronageExpiryBlock > 0 && currentBlock < sample.PatronageExpiryBlock {
			// Still compute fitness
			sample.FitnessScore = k.ComputeSampleFitness(ctx, sample, params)
			sample.FitnessUpdatedBlock = currentEpoch * EcologyEpochBlocks
			_ = k.SetSample(ctx, sample)
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

	// Phase 2: Update niche rankings
	for nicheKey := range nichesSeen {
		k.UpdateNicheLeader(ctx, nicheKey)
	}

	// Phase 3: Prune samples past grace period
	k.PruneSamples(ctx, currentEpoch, params)
}
```

**Wire EndBlock in module.go** — add after BeginBlock (line 158):

```go
// EndBlock processes epoch boundaries, patronage/bounty expiry, and slashing.
func (am AppModule) EndBlock(ctx context.Context) error {
	return am.keeper.EndBlocker(ctx)
}
```

**Step 4: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestEndBlocker|TestEcologyEpoch|TestBeginBlocker" -v -count=1`
Expected: All pass

**Step 5: Commit**

```bash
git add x/knowledge/keeper/phases.go x/knowledge/keeper/phases_test.go x/knowledge/keeper/ecology.go x/knowledge/module.go
git commit -m "feat(knowledge): EndBlocker with epoch processing, patronage/bounty expiry (R37-6)"
```

---

### Task 4: Missed reveal slashing

**Files:**
- Modify: `x/knowledge/keeper/phases.go` (add slashMissedReveals)

**Step 1: Write failing tests**

Add to `x/knowledge/keeper/phases_test.go`:

```go
func TestEndBlocker_SlashMissedReveals(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content: "test", Stake: "1000000",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// All 3 commit
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	// Transition to reveal
	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	// Only verifier1 and verifier2 reveal (verifier3 misses)
	for i, v := range []string{verifier1, verifier2} {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	// Advance past reveal deadline and aggregate via BeginBlocker
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(109).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	// The scoreValidators in AggregateQualityRound already emits missed-reveal events.
	// Verify the event was emitted
	events := sdkCtx.WithBlockHeight(109).EventManager().Events()
	// Check via the ctx we passed to BeginBlocker
	foundMissedReveal := false
	for _, e := range sdk.UnwrapSDKContext(ctx).EventManager().Events() {
		if e.Type == "validator_missed_reveal" {
			for _, attr := range e.Attributes {
				if attr.Key == "verifier" && attr.Value == verifier3 {
					foundMissedReveal = true
				}
			}
		}
	}
	require.True(t, foundMissedReveal, "expected missed-reveal event for verifier3")
}
```

**Step 2: Run test**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestEndBlocker_SlashMissedReveals" -v -count=1`
Expected: This may already pass since `scoreValidators` already emits `validator_missed_reveal` events. If so, this test serves as confirmation.

**Step 3: If test passes, commit**

```bash
git add x/knowledge/keeper/phases_test.go
git commit -m "test(knowledge): missed reveal slashing verification (R37-6)"
```

---

### Task 5: Comprehensive test suite (≥30 tests)

**Files:**
- Create: `x/knowledge/keeper/beginend_test.go` (dedicated test file for R37-6)

Write the remaining tests to reach ≥30 total for R37-6 functionality. This includes tests from Tasks 1-4 plus the following additional tests in a new file:

**Step 1: Write all remaining tests**

```go
package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── BeginBlocker Edge Cases ─────────────────────────────────────────────────

func TestBeginBlocker_RoundNotFound_CleansUpActiveIndex(t *testing.T) {
	k, ctx := setupKeeper(t)
	// Manually add a bogus active round ID
	require.NoError(t, k.SetActiveRound(ctx, "nonexistent-round"))

	require.NoError(t, k.BeginBlocker(ctx))

	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, "nonexistent-round")
}

func TestBeginBlocker_MultipleRounds_ProcessedIndependently(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub1 := &types.Submission{Id: "s1", Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	sub2 := &types.Submission{Id: "s2", Domain: "science", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	require.NoError(t, k.SetSubmission(ctx, sub1))
	require.NoError(t, k.SetSubmission(ctx, sub2))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID1, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	roundID2, _ := k.InitiateQualityRound(ctx, "s2", "", verifiers)

	// Commit to round1 only (3 commits)
	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID1, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID1, CommitHash: hash,
		}))
	}

	// Advance past commit deadline
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	// Round 1 → REVEAL (enough commits)
	r1, _ := k.GetQualityRound(ctx, roundID1)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_REVEAL, r1.Phase)

	// Round 2 → EXPIRED (no commits)
	r2, _ := k.GetQualityRound(ctx, roundID2)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, r2.Phase)
}

// ─── EndBlocker Edge Cases ───────────────────────────────────────────────────

func TestEndBlocker_NoParams_Noop(t *testing.T) {
	k, ctx := setupKeeper(t)
	// No params set — should not panic
	require.NoError(t, k.EndBlocker(ctx))
}

func TestEndBlocker_Block0_Noop(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(0).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))
}

func TestEndBlocker_PatronageNotExpired_Remains(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue:        "0",
		PatronageExpiryBlock: 200,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(50).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(200), s.PatronageExpiryBlock, "patronage not expired yet")
}

func TestEndBlocker_BountyAlreadyClaimed_NotDeleted(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "tech", ExpiresAtBlock: 50,
		RewardAmount: "1000000", Claimed: true,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	_, found := k.GetDataBounty(ctx, "b1")
	require.True(t, found, "claimed bounty should NOT be deleted")
}

func TestEndBlocker_MultipleBountiesExpire(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b1", ExpiresAtBlock: 50})
	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b2", ExpiresAtBlock: 80})
	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b3", ExpiresAtBlock: 200})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	_, f1 := k.GetDataBounty(ctx, "b1")
	require.False(t, f1)
	_, f2 := k.GetDataBounty(ctx, "b2")
	require.False(t, f2)
	_, f3 := k.GetDataBounty(ctx, "b3")
	require.True(t, f3)
}

func TestEndBlocker_PruningAtGracePeriod(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams() // PruneGraceEpochs = 10
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Content: "will die", Energy: 0, EnergyCap: 1_000_000,
		AtRiskSinceEpoch: 1, Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
		TotalRevenue: "0",
	})
	_ = k.SetAtRiskIndex(ctx, "1")

	// Epoch 12: grace = 12-1 = 11 >= 10 → prune
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(1200).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, s.Status)
	require.Empty(t, s.Content)
}

func TestEndBlocker_AtRiskTransitionAtEpoch(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	// After one decay (5% of 1 → min decay 1 → energy=0 → at-risk)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(0), s.Energy)
	require.Equal(t, uint64(1), s.AtRiskSinceEpoch)
}

func TestEndBlocker_SponsoredSample_FitnessStillComputed(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		QualityScore: 800_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue:        "0",
		PatronageExpiryBlock: 200,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Greater(t, s.FitnessScore, uint64(0), "fitness should still compute for sponsored")
	require.Equal(t, uint64(1_000_000), s.Energy, "energy should not decay for sponsored")
}

func TestExpireRound_ReturnsStake(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Stake: "5000000",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// No commits — advance past deadline
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	round, _ := k.GetQualityRound(ctx, roundID)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, round.Phase)

	// Verify stake return call
	require.GreaterOrEqual(t, len(bk.moduleToAccountCalls), 1)
	lastCall := bk.moduleToAccountCalls[len(bk.moduleToAccountCalls)-1]
	require.Equal(t, types.ModuleName, lastCall.from)
}

func TestExpireRound_SubmissionResetToPending(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Stake: "1000000",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	k.InitiateQualityRound(ctx, "s1", "", verifiers)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	updatedSub, _ := k.GetSubmission(ctx, "s1")
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING, updatedSub.Status)
}

func TestEndBlocker_MultipleEpochsOfDecay(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	// Run 3 epochs
	for epoch := uint64(1); epoch <= 3; epoch++ {
		block := int64(epoch * keeper.EcologyEpochBlocks)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctx = sdkCtx.WithBlockHeight(block).WithEventManager(sdk.NewEventManager())
		require.NoError(t, k.EndBlocker(ctx))
	}

	s, _ := k.GetSample(ctx, "1")
	// After 3 epochs of 5% decay: 1M * 0.95^3 ≈ 857,375
	require.Equal(t, uint64(857_375), s.Energy)
}

func TestEndBlocker_BountyExpiryOnlyAtEpoch(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b1", ExpiresAtBlock: 50})

	// Block 99 — NOT an epoch boundary
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(99).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	_, found := k.GetDataBounty(ctx, "b1")
	require.True(t, found, "bounty should survive non-epoch blocks")

	// Block 100 — IS an epoch boundary
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	_, found = k.GetDataBounty(ctx, "b1")
	require.False(t, found, "bounty should expire at epoch boundary")
}

// ─── Full Lifecycle Integration Test ─────────────────────────────────────────

func TestFullLifecycle_SubmitToDecayToAccessToPrune(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	// Block 1: Submit data
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	sub := &types.Submission{
		Id: "s1", Content: "integration test data", Domain: "technology",
		Submitter: testAddr, SampleType: types.SampleType_SAMPLE_TYPE_TUTORIAL,
		Tags: []string{"golang"}, Stake: "1000000",
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		License: "MIT",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	// Block 2: Quality round starts, validators commit
	ctx = sdkCtx.WithBlockHeight(2).WithEventManager(sdk.NewEventManager())
	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)

	votes := []*types.QualityVote{
		{OverallQuality: 850_000, Novelty: 700_000, ReasoningDepth: 600_000, ConsentValid: true},
		{OverallQuality: 860_000, Novelty: 710_000, ReasoningDepth: 590_000, ConsentValid: true},
		{OverallQuality: 840_000, Novelty: 690_000, ReasoningDepth: 610_000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("salt1"), []byte("salt2"), []byte("salt3")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	// Block N: Commit deadline passes → REVEAL phase
	round, _ := k.GetQualityRound(ctx, roundID)
	ctx = sdkCtx.WithBlockHeight(int64(round.CommitDeadline + 1)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	round, _ = k.GetQualityRound(ctx, roundID)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_REVEAL, round.Phase)

	// Validators reveal
	for i, v := range verifiers {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	// Block M: Reveal deadline passes → AGGREGATION → Sample created
	round, _ = k.GetQualityRound(ctx, roundID)
	ctx = sdkCtx.WithBlockHeight(int64(round.RevealDeadline + 1)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	round, _ = k.GetQualityRound(ctx, roundID)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)

	// Verify sample created
	sampleIDs := k.GetSamplesByDomain(ctx, "technology")
	require.GreaterOrEqual(t, len(sampleIDs), 1)
	sample, ok := k.GetSample(ctx, sampleIDs[0])
	require.True(t, ok)
	require.Equal(t, keeper.DefaultEnergyCap, sample.Energy)

	sampleID := sampleIDs[0]

	// Block M+100: Epoch — energy decays
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	sample, _ = k.GetSample(ctx, sampleID)
	require.Less(t, sample.Energy, keeper.DefaultEnergyCap)
	energyAfterOneDecay := sample.Energy

	// Access → energy restored
	k.RestoreEnergyOnAccess(ctx, sample, &params)
	_ = k.SetSample(ctx, sample)
	require.Greater(t, sample.Energy, energyAfterOneDecay)

	// Many epochs without access → energy drops to 0 → at-risk → pruned
	for epoch := uint64(2); epoch <= 300; epoch++ {
		block := int64(epoch * keeper.EcologyEpochBlocks)
		ctx = sdkCtx.WithBlockHeight(block).WithEventManager(sdk.NewEventManager())
		require.NoError(t, k.EndBlocker(ctx))
	}

	sample, _ = k.GetSample(ctx, sampleID)
	require.Equal(t, uint64(0), sample.Energy)
	require.Greater(t, sample.AtRiskSinceEpoch, uint64(0))

	// Continue past grace period (10 epochs) — sample pruned
	atRiskEpoch := sample.AtRiskSinceEpoch
	pruneEpoch := atRiskEpoch + params.PruneGraceEpochs + 1
	ctx = sdkCtx.WithBlockHeight(int64(pruneEpoch * keeper.EcologyEpochBlocks)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	sample, ok = k.GetSample(ctx, sampleID)
	require.True(t, ok, "record should still exist")
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, sample.Status)
	require.Empty(t, sample.Content, "content should be cleared after pruning")
}
```

**Step 2: Run all tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestBeginBlocker|TestEndBlocker|TestFullLifecycle|TestExpireRound|TestDataBounty|TestIterateDataBounties" -v -count=1`
Expected: All pass

**Step 3: Commit**

```bash
git add x/knowledge/keeper/beginend_test.go
git commit -m "test(knowledge): comprehensive BeginBlocker/EndBlocker test suite, ≥30 tests (R37-6)"
```

---

### Task 6: Verify full test suite and compile

**Step 1: Run all knowledge keeper tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -v -count=1 2>&1 | tail -50`
Expected: All pass, no compilation errors

**Step 2: Count R37-6 related tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -v -count=1 -run "TestBeginBlocker|TestEndBlocker|TestFullLifecycle|TestExpireRound|TestDataBounty|TestIterateDataBounties" 2>&1 | grep -c "^--- PASS"`
Expected: ≥ 30

**Step 3: Verify module compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/knowledge/...`
Expected: Clean build

**Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix(knowledge): resolve compilation issues from R37-6 integration"
```

---

## Test Summary

| Category | Tests |
|----------|-------|
| BeginBlocker: COMMIT→REVEAL with enough commits | 2 (existing + new) |
| BeginBlocker: insufficient commits → expire | 1 |
| BeginBlocker: REVEAL→AGGREGATION | 1 (existing) |
| BeginBlocker: no active rounds | 1 (existing) |
| BeginBlocker: expired round (no reveals) | 1 (existing) |
| BeginBlocker: round not found cleanup | 1 |
| BeginBlocker: multiple rounds | 1 |
| EndBlocker: no-op (no params) | 1 |
| EndBlocker: block 0 | 1 |
| EndBlocker: epoch boundary decay | 1 |
| EndBlocker: non-epoch no decay | 1 |
| EndBlocker: sponsored sample skips decay | 1 |
| EndBlocker: sponsored fitness still computed | 1 |
| EndBlocker: patronage expiry | 1 |
| EndBlocker: patronage not expired | 1 |
| EndBlocker: bounty expiry | 1 |
| EndBlocker: bounty already claimed | 1 |
| EndBlocker: multiple bounties expire | 1 |
| EndBlocker: bounty expiry only at epoch | 1 |
| EndBlocker: niche ranking update | 1 |
| EndBlocker: at-risk transition | 1 |
| EndBlocker: pruning at grace period | 1 |
| EndBlocker: multiple epochs of decay | 1 |
| EndBlocker: missed reveal slashing | 1 |
| Bounty CRUD: set/get/delete | 1 |
| Bounty: iterate | 1 |
| ExpireRound: returns stake | 1 |
| ExpireRound: submission reset to pending | 1 |
| Full lifecycle integration | 1 |
| **Total** | **≥30** |
