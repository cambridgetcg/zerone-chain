# R26-5 Auto-Resolution for Research and Bounties — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable automatic resolution of research submissions and bounty fulfillment via EndBlocker, eliminating the governance-only bottleneck.

**Architecture:** Two new keeper methods (`AutoResolveResearch`, `AutoFulfillBounties`) extract the proven accept/reject/slash/payout logic from `msg_server.go` and run in EndBlocker each block. Proto changes add `claimed_at` to Bounty and `bounty_fulfillment_period_blocks` to Params.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, protobuf (buf generate)

---

### Task 1: Add `claimed_at` to Bounty proto and `bounty_fulfillment_period_blocks` to Params

**Files:**
- Modify: `proto/zerone/research/v1/state.proto:27-39`
- Modify: `proto/zerone/research/v1/genesis.proto:18-27`

**Step 1: Add `claimed_at` field to Bounty message**

In `proto/zerone/research/v1/state.proto`, add field 12 after `created_at`:

```proto
message Bounty {
  string id              = 1;
  string creator         = 2;
  string title           = 3;
  string description     = 4;
  string domain          = 5;
  string reward          = 6;
  uint64 deadline_height = 7;
  string status          = 8;
  string claimed_by      = 9;
  string fulfilled_by    = 10;
  uint64 created_at      = 11;
  uint64 claimed_at      = 12; // block height when claimed
}
```

**Step 2: Add `bounty_fulfillment_period_blocks` to Params**

In `proto/zerone/research/v1/genesis.proto`, add field 9:

```proto
message Params {
  string min_research_stake          = 1;
  string min_challenge_stake         = 2;
  uint64 review_period_blocks        = 3;
  uint32 min_reviewer_count          = 4;
  uint32 acceptance_score_threshold  = 5;
  uint64 rejection_slash_bps         = 6;
  string max_bounty_reward           = 7;
  uint64 bounty_min_deadline_blocks  = 8;
  uint64 bounty_fulfillment_period_blocks = 9; // blocks after claim before auto-fulfillment
}
```

**Step 3: Regenerate protobuf Go code**

Run: `make proto-gen`
Expected: `state.pb.go` and `genesis.pb.go` regenerated with new fields.

**Step 4: Update DefaultParams with new field**

In `x/research/types/types.go`, add `BountyFulfillmentPeriodBlocks` to `DefaultParams()`:

```go
func DefaultParams() Params {
	return Params{
		MinResearchStake:               "1000000",
		MinChallengeStake:              "1000000",
		ReviewPeriodBlocks:             68544,
		MinReviewerCount:               3,
		AcceptanceScoreThreshold:       70,
		RejectionSlashBps:              330000,
		MaxBountyReward:                "10000000000",
		BountyMinDeadlineBlocks:        34272,
		BountyFulfillmentPeriodBlocks:  34272,   // ~1 day
	}
}
```

**Step 5: Set `claimed_at` in ClaimBounty msg handler**

In `x/research/keeper/msg_server.go`, in `ClaimBounty()` (around line 393), add `ClaimedAt` after setting `ClaimedBy`:

```go
	bounty.Status = string(types.BountyStatusClaimed)
	bounty.ClaimedBy = msg.Claimer
	bounty.ClaimedAt = uint64(ctx.BlockHeight())
	m.Keeper.SetBounty(ctx, bounty)
```

**Step 6: Verify compilation**

Run: `go build ./x/research/...`
Expected: compiles without errors.

**Step 7: Run existing tests**

Run: `go test ./x/research/... -count=1`
Expected: all 44 existing tests pass.

**Step 8: Commit**

```bash
git add proto/zerone/research/v1/state.proto proto/zerone/research/v1/genesis.proto \
  x/research/types/state.pb.go x/research/types/genesis.pb.go \
  x/research/types/types.go x/research/keeper/msg_server.go
git commit -m "feat(research): add claimed_at to Bounty proto and bounty_fulfillment_period_blocks param"
```

---

### Task 2: Add `AutoResolveResearch` keeper method with tests

**Files:**
- Modify: `x/research/keeper/keeper.go`
- Modify: `x/research/keeper/keeper_test.go`

**Step 1: Write failing tests for auto-resolve**

Add these tests to `x/research/keeper/keeper_test.go`:

```go
// -----------------------------------------------------------------------
// Tests: Auto-Resolution
// -----------------------------------------------------------------------

func TestAutoResolveResearchAccepted(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	// Set short review period for testing
	params := types.DefaultParams()
	params.ReviewPeriodBlocks = 10
	params.MinReviewerCount = 2
	params.AcceptanceScoreThreshold = 70
	k.SetParams(ctx, &params)

	// Submit research
	submitter := testAddr(1)
	bk.setBalance(submitter, "uzrn", sdkmath.NewInt(5000000))
	resp, err := msgServer.SubmitResearch(ctx, &types.MsgSubmitResearch{
		Submitter: submitter.String(),
		Title:     "Auto-Resolve Test",
		Description: "Testing auto-resolution",
		Domain:    "physics",
		Stake:     "1000000",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Add 2 reviews with high scores (approve)
	for i := 2; i <= 3; i++ {
		_, err := msgServer.ReviewResearch(ctx, &types.MsgReviewResearch{
			ResearchId:   resp.ResearchId,
			Reviewer:     testAddrStr(i),
			Verdict:      types.ReviewVerdict_REVIEW_VERDICT_APPROVE,
			QualityScore: 80,
			Reasoning:    "Good work",
		})
		if err != nil {
			t.Fatalf("review %d: %v", i, err)
		}
	}

	// Verify status is under_review
	research, _ := k.GetResearch(ctx, resp.ResearchId)
	if research.Status != "under_review" {
		t.Fatalf("expected under_review, got %s", research.Status)
	}

	// Advance past review period
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 11)

	// Run auto-resolve
	err = k.AutoResolveResearch(ctx)
	if err != nil {
		t.Fatalf("auto-resolve: %v", err)
	}

	// Research should be accepted
	research, _ = k.GetResearch(ctx, resp.ResearchId)
	if research.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", research.Status)
	}

	// Stake should be returned to submitter
	bal := bk.balances[submitter.String()+"/uzrn"]
	if !bal.Equal(sdkmath.NewInt(5000000)) {
		t.Fatalf("expected stake returned, balance: %s", bal)
	}
}

func TestAutoResolveResearchRejected(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.ReviewPeriodBlocks = 10
	params.MinReviewerCount = 2
	params.AcceptanceScoreThreshold = 70
	params.RejectionSlashBps = 330000 // 33%
	k.SetParams(ctx, &params)

	submitter := testAddr(1)
	bk.setBalance(submitter, "uzrn", sdkmath.NewInt(5000000))
	resp, _ := msgServer.SubmitResearch(ctx, &types.MsgSubmitResearch{
		Submitter: submitter.String(),
		Title:     "Low Score Research",
		Description: "Will be rejected",
		Domain:    "physics",
		Stake:     "1000000",
	})

	// Add 2 reviews with low scores
	for i := 2; i <= 3; i++ {
		msgServer.ReviewResearch(ctx, &types.MsgReviewResearch{
			ResearchId:   resp.ResearchId,
			Reviewer:     testAddrStr(i),
			Verdict:      types.ReviewVerdict_REVIEW_VERDICT_REJECT,
			QualityScore: 30,
			Reasoning:    "Poor quality",
		})
	}

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 11)
	k.AutoResolveResearch(ctx)

	research, _ := k.GetResearch(ctx, resp.ResearchId)
	if research.Status != "rejected" {
		t.Fatalf("expected rejected, got %s", research.Status)
	}

	// Submitter should get remainder (1M - 33% = 670000)
	// Original 5M - 1M stake + 670000 returned = 4670000
	bal := bk.balances[submitter.String()+"/uzrn"]
	if !bal.Equal(sdkmath.NewInt(4670000)) {
		t.Fatalf("expected 4670000 after slash, got %s", bal)
	}
}

func TestAutoResolveResearchInsufficientReviews(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.ReviewPeriodBlocks = 10
	params.MinReviewerCount = 3
	k.SetParams(ctx, &params)

	submitter := testAddr(1)
	bk.setBalance(submitter, "uzrn", sdkmath.NewInt(5000000))
	resp, _ := msgServer.SubmitResearch(ctx, &types.MsgSubmitResearch{
		Submitter: submitter.String(),
		Title:     "Not Enough Reviews",
		Description: "Only 1 review",
		Domain:    "physics",
		Stake:     "1000000",
	})

	// Only 1 review (need 3)
	msgServer.ReviewResearch(ctx, &types.MsgReviewResearch{
		ResearchId:   resp.ResearchId,
		Reviewer:     testAddrStr(2),
		Verdict:      types.ReviewVerdict_REVIEW_VERDICT_APPROVE,
		QualityScore: 90,
		Reasoning:    "Great",
	})

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 11)
	k.AutoResolveResearch(ctx)

	// Should still be under_review (not enough reviewers)
	research, _ := k.GetResearch(ctx, resp.ResearchId)
	if research.Status != "under_review" {
		t.Fatalf("expected under_review (insufficient reviews), got %s", research.Status)
	}
}

func TestAutoResolveResearchWithinPeriod(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.ReviewPeriodBlocks = 100
	params.MinReviewerCount = 2
	k.SetParams(ctx, &params)

	submitter := testAddr(1)
	bk.setBalance(submitter, "uzrn", sdkmath.NewInt(5000000))
	resp, _ := msgServer.SubmitResearch(ctx, &types.MsgSubmitResearch{
		Submitter: submitter.String(),
		Title:     "Too Early",
		Description: "Not enough time",
		Domain:    "physics",
		Stake:     "1000000",
	})

	for i := 2; i <= 3; i++ {
		msgServer.ReviewResearch(ctx, &types.MsgReviewResearch{
			ResearchId:   resp.ResearchId,
			Reviewer:     testAddrStr(i),
			Verdict:      types.ReviewVerdict_REVIEW_VERDICT_APPROVE,
			QualityScore: 90,
			Reasoning:    "Excellent",
		})
	}

	// Only advance 5 blocks (need 100)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 5)
	k.AutoResolveResearch(ctx)

	// Should still be under_review
	research, _ := k.GetResearch(ctx, resp.ResearchId)
	if research.Status != "under_review" {
		t.Fatalf("expected under_review (within period), got %s", research.Status)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/research/keeper/ -run "TestAutoResolve" -count=1`
Expected: FAIL — `k.AutoResolveResearch` does not exist.

**Step 3: Implement `AutoResolveResearch` in keeper**

Add to `x/research/keeper/keeper.go`:

```go
// AutoResolveResearch resolves research submissions that have met review conditions.
// Called from EndBlocker each block.
func (k Keeper) AutoResolveResearch(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	researches := k.GetResearchesByStatus(ctx, types.ResearchStatusUnderReview)
	for _, research := range researches {
		// Skip if review period has not elapsed
		if currentBlock-research.UpdatedAt < params.ReviewPeriodBlocks {
			continue
		}

		// Skip if insufficient reviews
		if research.ReviewCount < params.MinReviewerCount {
			continue
		}

		// Determine outcome
		stakeInt := new(big.Int)
		stakeInt.SetString(research.Stake, 10)

		if research.AggregateScore >= params.AcceptanceScoreThreshold {
			// Accepted — return full stake to submitter
			research.Status = string(types.ResearchStatusAccepted)

			submitterAddr, err := sdk.AccAddressFromBech32(research.Submitter)
			if err != nil {
				k.Logger(ctx).Error("invalid submitter address", "research_id", research.Id, "error", err)
				continue
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeInt)))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins); err != nil {
				k.Logger(ctx).Error("failed to return stake", "research_id", research.Id, "error", err)
				continue
			}
		} else {
			// Rejected — slash stake, return remainder
			research.Status = string(types.ResearchStatusRejected)

			slashRate := new(big.Int).SetUint64(params.RejectionSlashBps)
			slashAmount := new(big.Int).Mul(stakeInt, slashRate)
			slashAmount.Div(slashAmount, new(big.Int).SetUint64(1000000))

			// Route slashed amount to development fund
			if slashAmount.Sign() > 0 {
				slashCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(slashAmount)))
				if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "development_fund", slashCoins); err != nil {
					k.Logger(ctx).Error("failed to slash to dev fund", "research_id", research.Id, "error", err)
					continue
				}
			}

			// Return remainder to submitter
			remainder := new(big.Int).Sub(stakeInt, slashAmount)
			if remainder.Sign() > 0 {
				submitterAddr, err := sdk.AccAddressFromBech32(research.Submitter)
				if err != nil {
					k.Logger(ctx).Error("invalid submitter address", "research_id", research.Id, "error", err)
					continue
				}
				returnCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(remainder)))
				if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, returnCoins); err != nil {
					k.Logger(ctx).Error("failed to return remainder", "research_id", research.Id, "error", err)
					continue
				}
			}
		}

		research.UpdatedAt = currentBlock
		k.SetResearch(ctx, research)

		var outcomeStr string
		if research.Status == string(types.ResearchStatusAccepted) {
			outcomeStr = "accepted"
		} else {
			outcomeStr = "rejected"
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.research.research_auto_resolved",
				sdk.NewAttribute("research_id", research.Id),
				sdk.NewAttribute("outcome", outcomeStr),
				sdk.NewAttribute("aggregate_score", fmt.Sprintf("%d", research.AggregateScore)),
			),
		)
	}
	return nil
}
```

Add imports to `keeper.go` if not already present: `"math/big"`, `sdkmath "cosmossdk.io/math"`.

**Step 4: Run tests to verify they pass**

Run: `go test ./x/research/keeper/ -run "TestAutoResolve" -count=1 -v`
Expected: all 4 PASS.

**Step 5: Run all existing tests**

Run: `go test ./x/research/... -count=1`
Expected: all tests pass.

**Step 6: Commit**

```bash
git add x/research/keeper/keeper.go x/research/keeper/keeper_test.go
git commit -m "feat(research): add AutoResolveResearch keeper method with tests"
```

---

### Task 3: Add `AutoFulfillBounties` keeper method with tests

**Files:**
- Modify: `x/research/keeper/keeper.go`
- Modify: `x/research/keeper/keeper_test.go`

**Step 1: Write failing tests for auto-fulfill**

Add to `x/research/keeper/keeper_test.go`:

```go
// -----------------------------------------------------------------------
// Tests: Auto-Fulfillment
// -----------------------------------------------------------------------

func TestAutoFulfillBountyAccepted(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.BountyFulfillmentPeriodBlocks = 10
	params.BountyMinDeadlineBlocks = 5
	k.SetParams(ctx, &params)

	creator := testAddr(1)
	claimer := testAddr(2)
	bk.setBalance(creator, "uzrn", sdkmath.NewInt(50000000))

	// Create bounty
	bResp, err := msgServer.CreateBounty(ctx, &types.MsgCreateBounty{
		Creator:        creator.String(),
		Title:          "Test Bounty",
		Description:    "Auto-fulfill test",
		Reward:         "5000000",
		DeadlineHeight: uint64(ctx.BlockHeight()) + 1000,
	})
	if err != nil {
		t.Fatalf("create bounty: %v", err)
	}

	// Claim bounty
	_, err = msgServer.ClaimBounty(ctx, &types.MsgClaimBounty{
		BountyId: bResp.BountyId,
		Claimer:  claimer.String(),
	})
	if err != nil {
		t.Fatalf("claim bounty: %v", err)
	}

	// Advance past fulfillment period
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 11)

	// Run auto-fulfill
	err = k.AutoFulfillBounties(ctx)
	if err != nil {
		t.Fatalf("auto-fulfill: %v", err)
	}

	// Bounty should be fulfilled
	bounty, _ := k.GetBounty(ctx, bResp.BountyId)
	if bounty.Status != "fulfilled" {
		t.Fatalf("expected fulfilled, got %s", bounty.Status)
	}
	if bounty.FulfilledBy != claimer.String() {
		t.Fatalf("expected fulfilled_by = %s, got %s", claimer.String(), bounty.FulfilledBy)
	}

	// Claimer should have reward
	bal := bk.balances[claimer.String()+"/uzrn"]
	if !bal.Equal(sdkmath.NewInt(5000000)) {
		t.Fatalf("expected claimer to have 5000000, got %s", bal)
	}
}

func TestAutoFulfillBountyWithinPeriod(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.BountyFulfillmentPeriodBlocks = 100
	params.BountyMinDeadlineBlocks = 5
	k.SetParams(ctx, &params)

	creator := testAddr(1)
	claimer := testAddr(2)
	bk.setBalance(creator, "uzrn", sdkmath.NewInt(50000000))

	bResp, _ := msgServer.CreateBounty(ctx, &types.MsgCreateBounty{
		Creator:        creator.String(),
		Title:          "Too Early Bounty",
		Description:    "Within period",
		Reward:         "5000000",
		DeadlineHeight: uint64(ctx.BlockHeight()) + 1000,
	})

	msgServer.ClaimBounty(ctx, &types.MsgClaimBounty{
		BountyId: bResp.BountyId,
		Claimer:  claimer.String(),
	})

	// Only advance 5 blocks (need 100)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 5)
	k.AutoFulfillBounties(ctx)

	// Should still be claimed
	bounty, _ := k.GetBounty(ctx, bResp.BountyId)
	if bounty.Status != "claimed" {
		t.Fatalf("expected claimed (within period), got %s", bounty.Status)
	}
}

func TestGovernanceOverrideStillWorks(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.ReviewPeriodBlocks = 1000 // Long period
	params.MinReviewerCount = 2
	k.SetParams(ctx, &params)

	submitter := testAddr(1)
	bk.setBalance(submitter, "uzrn", sdkmath.NewInt(5000000))
	resp, _ := msgServer.SubmitResearch(ctx, &types.MsgSubmitResearch{
		Submitter: submitter.String(),
		Title:     "Governance Override",
		Description: "Force resolve via authority",
		Domain:    "physics",
		Stake:     "1000000",
	})

	// Add reviews
	for i := 2; i <= 3; i++ {
		msgServer.ReviewResearch(ctx, &types.MsgReviewResearch{
			ResearchId:   resp.ResearchId,
			Reviewer:     testAddrStr(i),
			Verdict:      types.ReviewVerdict_REVIEW_VERDICT_APPROVE,
			QualityScore: 80,
			Reasoning:    "Good",
		})
	}

	// Governance can force-resolve immediately (no need to wait)
	bk.setBalance(sdk.AccAddress([]byte("research")), "uzrn", sdkmath.NewInt(1000000))
	resolveResp, err := msgServer.ResolveResearch(ctx, &types.MsgResolveResearch{
		Authority:  testAuthority,
		ResearchId: resp.ResearchId,
	})
	if err != nil {
		t.Fatalf("governance resolve: %v", err)
	}
	if resolveResp.Outcome != types.ResearchOutcome_RESEARCH_OUTCOME_ACCEPTED {
		t.Fatal("expected accepted via governance override")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/research/keeper/ -run "TestAutoFulfill|TestGovernanceOverride" -count=1`
Expected: FAIL — `k.AutoFulfillBounties` does not exist.

**Step 3: Implement `AutoFulfillBounties` in keeper**

Add to `x/research/keeper/keeper.go`:

```go
// AutoFulfillBounties fulfills bounties that have been claimed for longer than
// the fulfillment period. Called from EndBlocker each block.
func (k Keeper) AutoFulfillBounties(ctx sdk.Context) error {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	k.IterateBounties(ctx, func(b *types.Bounty) bool {
		if b.Status != string(types.BountyStatusClaimed) {
			return false
		}

		// Skip if fulfillment period has not elapsed
		if b.ClaimedAt == 0 || currentBlock-b.ClaimedAt < params.BountyFulfillmentPeriodBlocks {
			return false
		}

		// Pay reward to claimer
		rewardInt := new(big.Int)
		if _, ok := rewardInt.SetString(b.Reward, 10); !ok || rewardInt.Sign() <= 0 {
			k.Logger(ctx).Error("invalid bounty reward", "bounty_id", b.Id)
			return false
		}

		claimerAddr, err := sdk.AccAddressFromBech32(b.ClaimedBy)
		if err != nil {
			k.Logger(ctx).Error("invalid claimer address", "bounty_id", b.Id, "error", err)
			return false
		}

		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(rewardInt)))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, claimerAddr, coins); err != nil {
			k.Logger(ctx).Error("failed to pay bounty reward", "bounty_id", b.Id, "error", err)
			return false
		}

		b.Status = string(types.BountyStatusFulfilled)
		b.FulfilledBy = b.ClaimedBy
		k.SetBounty(ctx, b)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.research.bounty_auto_fulfilled",
				sdk.NewAttribute("bounty_id", b.Id),
				sdk.NewAttribute("fulfilled_by", b.ClaimedBy),
				sdk.NewAttribute("reward", b.Reward),
			),
		)

		return false
	})

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./x/research/keeper/ -run "TestAutoFulfill|TestGovernanceOverride" -count=1 -v`
Expected: all 3 PASS.

**Step 5: Run all tests**

Run: `go test ./x/research/... -count=1`
Expected: all tests pass.

**Step 6: Commit**

```bash
git add x/research/keeper/keeper.go x/research/keeper/keeper_test.go
git commit -m "feat(research): add AutoFulfillBounties keeper method with tests"
```

---

### Task 4: Wire EndBlocker and verify full chain

**Files:**
- Modify: `x/research/module.go:125-128`

**Step 1: Wire EndBlocker**

Replace the no-op EndBlock in `x/research/module.go`:

```go
// EndBlock auto-resolves research and auto-fulfills bounties.
func (am AppModule) EndBlock(goCtx context.Context) error {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := am.keeper.AutoResolveResearch(ctx); err != nil {
		ctx.Logger().Error("research auto-resolve failed", "error", err)
	}
	if err := am.keeper.AutoFulfillBounties(ctx); err != nil {
		ctx.Logger().Error("bounty auto-fulfill failed", "error", err)
	}

	return nil
}
```

**Step 2: Verify compilation**

Run: `go build ./...`
Expected: compiles.

**Step 3: Run all tests**

Run: `go test ./x/research/... -count=1`
Expected: all tests pass.

**Step 4: Run full project tests**

Run: `go test ./... -count=1 -timeout 300s`
Expected: all tests pass (or at least no regressions in research module).

**Step 5: Commit**

```bash
git add x/research/module.go
git commit -m "feat(research): wire AutoResolveResearch and AutoFulfillBounties into EndBlocker (R26-5)"
```
