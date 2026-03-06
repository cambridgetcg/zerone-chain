# R37-4 — Contest & Sponsor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement ContestSample (disputes with re-validation) and SponsorSample (preservation with patronage) for the knowledge module.

**Architecture:** ContestSample locks challenger stake, creates a re-validation QualityRound, and resolves based on the new round's verdict. SponsorSample transfers funds to the module account and sets patronage fields on the sample. Both delegate to existing state/round infrastructure.

**Tech Stack:** Go, Cosmos SDK v0.50, protobuf, commit-reveal quality rounds

---

### Task 1: Add Contest Index Key Constructors

**Files:**
- Modify: `x/knowledge/types/keys.go`

**Step 1: Add key prefix and constructors**

Add to the key prefixes section (after `AtRiskSampleIndexPrefix`):

```go
// ─── Contest ────────────────────────────────────────────────────────
ContestIndexPrefix = []byte{0xa8} // sampleID → contestRoundID (active contest)
```

Add key constructors at the end of the "New key constructors" section:

```go
// ContestIndexKey returns the index key mapping a contested sample to its re-validation round.
func ContestIndexKey(sampleID string) []byte {
	return append(append([]byte{}, ContestIndexPrefix...), []byte(sampleID)...)
}
```

**Step 2: Run vet**

Run: `go vet ./x/knowledge/types/...`
Expected: clean

**Step 3: Commit**

```bash
git add x/knowledge/types/keys.go
git commit -m "feat(knowledge): add contest index key prefix and constructor (R37-4)"
```

---

### Task 2: Add Contest State Methods

**Files:**
- Modify: `x/knowledge/keeper/state.go`

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/state_test.go`:

```go
func TestContestIndex_SetGetDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initially no contest
	_, found := k.GetContestRound(ctx, "sample-1")
	require.False(t, found)

	// Set contest index
	require.NoError(t, k.SetContestIndex(ctx, "sample-1", "round-42"))

	// Get it back
	roundID, found := k.GetContestRound(ctx, "sample-1")
	require.True(t, found)
	require.Equal(t, "round-42", roundID)

	// Delete
	require.NoError(t, k.DeleteContestIndex(ctx, "sample-1"))
	_, found = k.GetContestRound(ctx, "sample-1")
	require.False(t, found)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestContestIndex -v`
Expected: FAIL — `k.SetContestIndex` undefined

**Step 3: Write minimal implementation**

Add to `x/knowledge/keeper/state.go` after the At-risk section:

```go
// ─── Contest index ──────────────────────────────────────────────────────────

func (k Keeper) SetContestIndex(ctx context.Context, sampleID, roundID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.ContestIndexKey(sampleID), []byte(roundID))
}

func (k Keeper) GetContestRound(ctx context.Context, sampleID string) (string, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContestIndexKey(sampleID))
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

func (k Keeper) DeleteContestIndex(ctx context.Context, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.ContestIndexKey(sampleID))
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/knowledge/keeper/ -run TestContestIndex -v`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/state.go x/knowledge/keeper/state_test.go
git commit -m "feat(knowledge): add contest index CRUD methods (R37-4)"
```

---

### Task 3: Implement ContestSample Keeper Method

**Files:**
- Create: `x/knowledge/keeper/contest.go`
- Modify: `x/knowledge/keeper/msg_server.go`

**Step 1: Write the failing tests**

Add to a new file `x/knowledge/keeper/contest_test.go`:

```go
package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	challenger1 = "zrn1challenger1qqqqqqqqqqqqqqqpkaj9y"
	sampleOwner = testAddr
)

// helper: create an active (gold) sample for testing contests.
func createGoldSample(t *testing.T, k interface{ SetSample(ctx interface{}, s *types.Sample) error }, ctx interface{}, sampleID, submitter string) {
	t.Helper()
	// Type-assert properly in the actual test
}

func TestContestSample_Success(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Create a gold sample to contest
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test content",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "quality is overrated",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)

	// Sample should be CONTESTED
	sample, found := k.GetSample(ctx, "1")
	require.True(t, found)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_CONTESTED, sample.Status)

	// Contest index should be set
	roundID, found := k.GetContestRound(ctx, "1")
	require.True(t, found)
	require.Equal(t, resp.RoundId, roundID)

	// Stake should be locked (sent to module)
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(1000000))), bk.accountToModuleCalls[0].amount)
}

func TestContestSample_SampleNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "nonexistent",
		Stake:       "1000000",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrSampleNotFound)
}

func TestContestSample_CannotContestPruned(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:     "1",
		Status: types.SampleStatus_SAMPLE_STATUS_PRUNED,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestContestSample_CannotContestRejected(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:     "1",
		Status: types.SampleStatus_SAMPLE_STATUS_REJECTED,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestContestSample_AlreadyContested(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_CONTESTED,
	}))
	require.NoError(t, k.SetContestIndex(ctx, "1", "existing-round"))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrDuplicateChallenge)
}

func TestContestSample_SelfChallenge(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgContestSample{
		Challenger:  testAddr,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "self-challenge",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrSelfChallenge)
}

func TestContestSample_InsufficientStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "100", // Too low
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestContestSample_ConsentType_LowerStake(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_SILVER,
	}))

	// Consent contest: min stake is half of MinSubmissionStake (500000)
	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "500000",
		Reason:      "no real consent",
		ContestType: types.ContestType_CONTEST_TYPE_CONSENT,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)
	require.Len(t, bk.accountToModuleCalls, 1)
}

func TestContestSample_DuplicateFastPath(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Create two samples with the same content hash
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "duplicate content",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	// Set content hash for the original
	hash := k.ComputeContentHash("duplicate content")
	require.NoError(t, k.SetContentHash(ctx, hash, "sub-1"))

	// Create a second sample with different ID but same content
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "2",
		Content:   "duplicate content",
		Domain:    "technology",
		Submitter: challenger1,
		Status:    types.SampleStatus_SAMPLE_STATUS_SILVER,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "sample 2 is a duplicate of sample 1",
		ContestType: types.ContestType_CONTEST_TYPE_DUPLICATE,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	// Duplicate contests still create a round (validator must verify)
	require.NotEmpty(t, resp.RoundId)
	require.Len(t, bk.accountToModuleCalls, 1)
}

func TestContestSample_StakeLockFails(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	bk.failNextSend = true

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad quality",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestContestSample_BronzeSample(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_BRONZE,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "toxic content",
		ContestType: types.ContestType_CONTEST_TYPE_TOXIC,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)
}

func TestContestSample_CopyrightType(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "copyrighted text",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "this is copyrighted",
		ContestType: types.ContestType_CONTEST_TYPE_COPYRIGHT,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)
}

func TestContestSample_EmitsEvent(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)

	events := sdkCtx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == "sample_contested" {
			found = true
			break
		}
	}
	require.True(t, found, "expected sample_contested event")
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/knowledge/keeper/ -run TestContestSample -v`
Expected: FAIL — `k.ContestSample` undefined

**Step 3: Write the implementation**

Create `x/knowledge/keeper/contest.go`:

```go
package keeper

import (
	"context"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ContestSample handles a dispute against a validated sample.
// It locks the challenger's stake, marks the sample as CONTESTED,
// and creates a re-validation QualityRound.
func (k Keeper) ContestSample(ctx context.Context, msg *types.MsgContestSample) (*types.MsgContestSampleResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Verify sample exists
	sample, found := k.GetSample(ctx, msg.SampleId)
	if !found {
		return nil, types.ErrSampleNotFound.Wrapf("sample %q not found", msg.SampleId)
	}

	// 2. Verify sample is active (gold/silver/bronze)
	if !isActiveSampleStatus(sample.Status) {
		return nil, types.ErrInvalidChallenge.Wrapf("sample status %s is not contestable", sample.Status.String())
	}

	// 3. Check not already contested
	if _, contested := k.GetContestRound(ctx, msg.SampleId); contested {
		return nil, types.ErrDuplicateChallenge.Wrap("sample is already under contest")
	}

	// 4. Cannot contest own sample
	if msg.Challenger == sample.Submitter {
		return nil, types.ErrSelfChallenge
	}

	// 5. Validate stake amount
	stakeAmt, ok := sdkmath.NewIntFromString(msg.Stake)
	if !ok || !stakeAmt.IsPositive() {
		return nil, types.ErrInsufficientStake.Wrap("invalid stake amount")
	}

	minStake, _ := sdkmath.NewIntFromString(params.MinSubmissionStake)
	// Consent contests have lower stake requirement (half)
	if msg.ContestType == types.ContestType_CONTEST_TYPE_CONSENT {
		minStake = minStake.Quo(sdkmath.NewInt(2))
		if minStake.IsZero() {
			minStake = sdkmath.OneInt()
		}
	}
	if stakeAmt.LT(minStake) {
		return nil, types.ErrInsufficientStake.Wrapf("stake %s < minimum %s", msg.Stake, minStake.String())
	}

	// 6. Lock challenger's stake
	challengerAddr, _ := sdk.AccAddressFromBech32(msg.Challenger)
	stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		sdkCtx, challengerAddr, types.ModuleName, sdk.NewCoins(stakeCoin),
	); err != nil {
		return nil, types.ErrInsufficientStake.Wrap(err.Error())
	}

	// 7. Create a re-validation submission to anchor the quality round
	// We need a submission for the round; create a synthetic one from the sample
	subID := k.NextSubmissionID(ctx)
	sub := &types.Submission{
		Id:               subID,
		Submitter:        msg.Challenger,
		Content:          sample.Content,
		SampleType:       sample.SampleType,
		Domain:           sample.Domain,
		SourceUri:        sample.SourceUri,
		SourcePlatform:   sample.SourcePlatform,
		SourceTimestamp:   sample.SourceTimestamp,
		Consent:          sample.Consent,
		OriginalAuthor:   sample.OriginalAuthor,
		License:          sample.License,
		Tags:             sample.Tags,
		Language:         sample.Language,
		Stake:            msg.Stake,
		SubmittedAtBlock: uint64(sdkCtx.BlockHeight()),
		Status:           types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW,
		ContentHash:      k.ComputeContentHash(sample.Content),
	}
	if err := k.SetSubmission(ctx, sub); err != nil {
		return nil, err
	}

	// 8. Create re-validation round
	roundID, err := k.InitiateQualityRound(ctx, subID, "", []string{})
	if err != nil {
		return nil, err
	}

	// 9. Mark sample as CONTESTED and link to round
	prevStatus := sample.Status
	sample.Status = types.SampleStatus_SAMPLE_STATUS_CONTESTED
	if err := k.SetSample(ctx, sample); err != nil {
		return nil, err
	}
	if err := k.SetContestIndex(ctx, msg.SampleId, roundID); err != nil {
		return nil, err
	}

	// 10. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"sample_contested",
		sdk.NewAttribute("sample_id", msg.SampleId),
		sdk.NewAttribute("challenger", msg.Challenger),
		sdk.NewAttribute("contest_type", msg.ContestType.String()),
		sdk.NewAttribute("round_id", roundID),
		sdk.NewAttribute("stake", msg.Stake),
		sdk.NewAttribute("previous_status", prevStatus.String()),
		sdk.NewAttribute("reason", msg.Reason),
		sdk.NewAttribute("block", strconv.FormatInt(sdkCtx.BlockHeight(), 10)),
	))

	return &types.MsgContestSampleResponse{RoundId: roundID}, nil
}

// isActiveSampleStatus returns true if the sample is in a contestable state.
func isActiveSampleStatus(s types.SampleStatus) bool {
	switch s {
	case types.SampleStatus_SAMPLE_STATUS_GOLD,
		types.SampleStatus_SAMPLE_STATUS_SILVER,
		types.SampleStatus_SAMPLE_STATUS_BRONZE:
		return true
	default:
		return false
	}
}
```

**Step 4: Wire msg_server.go**

In `x/knowledge/keeper/msg_server.go`, replace the ContestSample stub:

```go
func (m msgServer) ContestSample(ctx context.Context, msg *types.MsgContestSample) (*types.MsgContestSampleResponse, error) {
	return m.keeper.ContestSample(ctx, msg)
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./x/knowledge/keeper/ -run TestContestSample -v`
Expected: PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/contest.go x/knowledge/keeper/contest_test.go x/knowledge/keeper/msg_server.go
git commit -m "feat(knowledge): implement ContestSample with stake locking and re-validation (R37-4)"
```

---

### Task 4: Implement SponsorSample Keeper Method

**Files:**
- Create: `x/knowledge/keeper/sponsor.go`
- Modify: `x/knowledge/keeper/msg_server.go`

**Step 1: Write the failing tests**

Add to a new file `x/knowledge/keeper/sponsor_test.go`:

```go
package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const sponsor1 = "zrn1sponsor1qqqqqqqqqqqqqqqqpl0ync"

func TestSponsorSample_Success(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
		Energy:    500,
		EnergyCap: 1000,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "5000000",
		DurationBlocks: 1000,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)

	// Check sample updated
	sample, found := k.GetSample(ctx, "1")
	require.True(t, found)
	require.Equal(t, "5000000", sample.PatronageAmount)
	require.Equal(t, uint64(1100), sample.PatronageExpiryBlock) // block 100 + 1000
	require.Equal(t, uint64(1000), sample.Energy) // Restored to cap

	// Check payment
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(5000000))), bk.accountToModuleCalls[0].amount)
}

func TestSponsorSample_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "nonexistent",
		Amount:         "5000000",
		DurationBlocks: 1000,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrSampleNotFound)
}

func TestSponsorSample_ExtendExisting(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Sample already has patronage
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:                   "1",
		Content:              "test",
		Domain:               "technology",
		Submitter:            testAddr,
		Status:               types.SampleStatus_SAMPLE_STATUS_SILVER,
		PatronageAmount:      "3000000",
		PatronageExpiryBlock: 500, // Existing expiry at block 500
		Energy:               800,
		EnergyCap:            1000,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "2000000",
		DurationBlocks: 200,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)

	sample, found := k.GetSample(ctx, "1")
	require.True(t, found)
	require.Equal(t, "5000000", sample.PatronageAmount) // 3M + 2M
	require.Equal(t, uint64(700), sample.PatronageExpiryBlock) // max(500, 100+200) = 500, extended by 200 → 700
	require.Equal(t, uint64(1000), sample.Energy)
}

func TestSponsorSample_PrunedSample(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:     "1",
		Status: types.SampleStatus_SAMPLE_STATUS_PRUNED,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "5000000",
		DurationBlocks: 1000,
	}

	// Can sponsor even pruned samples (restores them)
	_, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)
}

func TestSponsorSample_PaymentFails(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	bk.failNextSend = true

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "5000000",
		DurationBlocks: 1000,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestSponsorSample_ZeroDuration(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:     "1",
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "5000000",
		DurationBlocks: 0,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestSponsorSample_EmitsEvent(t *testing.T) {
	k, ctx := setupKeeper(t)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
		EnergyCap: 1000,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "5000000",
		DurationBlocks: 500,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)

	events := sdkCtx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == "sample_sponsored" {
			found = true
			break
		}
	}
	require.True(t, found, "expected sample_sponsored event")
}

func TestSponsorSample_InvalidAmount(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:     "1",
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "0",
		DurationBlocks: 100,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/knowledge/keeper/ -run TestSponsorSample -v`
Expected: FAIL — `k.SponsorSample` undefined

**Step 3: Write the implementation**

Create `x/knowledge/keeper/sponsor.go`:

```go
package keeper

import (
	"context"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// SponsorSample handles patronage payments that preserve a sample from pruning.
func (k Keeper) SponsorSample(ctx context.Context, msg *types.MsgSponsorSample) (*types.MsgSponsorSampleResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// 1. Verify sample exists
	sample, found := k.GetSample(ctx, msg.SampleId)
	if !found {
		return nil, types.ErrSampleNotFound.Wrapf("sample %q not found", msg.SampleId)
	}

	// 2. Validate amount
	amount, ok := sdkmath.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, types.ErrInsufficientPayment.Wrap("amount must be positive")
	}

	// 3. Validate duration
	if msg.DurationBlocks == 0 {
		return nil, types.ErrInvalidChallenge.Wrap("duration_blocks must be > 0")
	}

	// 4. Transfer amount from sponsor to module account
	sponsorAddr, _ := sdk.AccAddressFromBech32(msg.Sponsor)
	coin := sdk.NewCoin("uzrn", amount)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		sdkCtx, sponsorAddr, types.ModuleName, sdk.NewCoins(coin),
	); err != nil {
		return nil, types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// 5. Update patronage amount (accumulate)
	existingAmount, _ := sdkmath.NewIntFromString(sample.PatronageAmount)
	newTotal := existingAmount.Add(amount)
	sample.PatronageAmount = newTotal.String()

	// 6. Set/extend patronage expiry
	block := uint64(sdkCtx.BlockHeight())
	newExpiry := block + msg.DurationBlocks
	if sample.PatronageExpiryBlock > block {
		// Extend from existing expiry
		newExpiry = sample.PatronageExpiryBlock + msg.DurationBlocks
	}
	sample.PatronageExpiryBlock = newExpiry

	// 7. Restore energy to cap
	if sample.EnergyCap > 0 {
		sample.Energy = sample.EnergyCap
	}
	sample.EnergyLastUpdated = block

	// 8. Save
	if err := k.SetSample(ctx, sample); err != nil {
		return nil, err
	}

	// 9. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"sample_sponsored",
		sdk.NewAttribute("sample_id", msg.SampleId),
		sdk.NewAttribute("sponsor", msg.Sponsor),
		sdk.NewAttribute("amount", msg.Amount),
		sdk.NewAttribute("duration_blocks", strconv.FormatUint(msg.DurationBlocks, 10)),
		sdk.NewAttribute("patronage_total", newTotal.String()),
		sdk.NewAttribute("expiry_block", strconv.FormatUint(newExpiry, 10)),
	))

	return &types.MsgSponsorSampleResponse{}, nil
}
```

**Step 4: Wire msg_server.go**

In `x/knowledge/keeper/msg_server.go`, replace the SponsorSample stub:

```go
func (m msgServer) SponsorSample(ctx context.Context, msg *types.MsgSponsorSample) (*types.MsgSponsorSampleResponse, error) {
	return m.keeper.SponsorSample(ctx, msg)
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./x/knowledge/keeper/ -run TestSponsorSample -v`
Expected: PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/sponsor.go x/knowledge/keeper/sponsor_test.go x/knowledge/keeper/msg_server.go
git commit -m "feat(knowledge): implement SponsorSample with patronage and energy restore (R37-4)"
```

---

### Task 5: Comprehensive Integration Tests

**Files:**
- Modify: `x/knowledge/keeper/contest_test.go`
- Modify: `x/knowledge/keeper/sponsor_test.go`

**Step 1: Add remaining integration tests to contest_test.go**

```go
func TestContestSample_AllContestTypes(t *testing.T) {
	contestTypes := []types.ContestType{
		types.ContestType_CONTEST_TYPE_CONSENT,
		types.ContestType_CONTEST_TYPE_QUALITY,
		types.ContestType_CONTEST_TYPE_DUPLICATE,
		types.ContestType_CONTEST_TYPE_TOXIC,
		types.ContestType_CONTEST_TYPE_COPYRIGHT,
	}

	for _, ct := range contestTypes {
		t.Run(ct.String(), func(t *testing.T) {
			k, ctx := setupKeeper(t)
			setupDefaultDomains(t, k, ctx)

			require.NoError(t, k.SetSample(ctx, &types.Sample{
				Id:        "1",
				Content:   "test content for " + ct.String(),
				Domain:    "technology",
				Submitter: testAddr,
				Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
			}))

			msg := &types.MsgContestSample{
				Challenger:  challenger1,
				SampleId:    "1",
				Stake:       "1000000",
				Reason:      "contest reason",
				ContestType: ct,
			}

			resp, err := k.ContestSample(ctx, msg)
			require.NoError(t, err)
			require.NotEmpty(t, resp.RoundId)

			// Verify round was created
			round, found := k.GetQualityRound(ctx, resp.RoundId)
			require.True(t, found)
			require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMMIT, round.Phase)
		})
	}
}

func TestContestSample_SilverSample(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_SILVER,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad quality",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.RoundId)
}

func TestContestSample_CannotContestExpired(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:     "1",
		Status: types.SampleStatus_SAMPLE_STATUS_EXPIRED,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestContestSample_CannotContestPending(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:     "1",
		Status: types.SampleStatus_SAMPLE_STATUS_PENDING,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestContestSample_RoundHasCorrectSubmission(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "original content",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "1000000",
		Reason:      "bad quality",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	resp, err := k.ContestSample(ctx, msg)
	require.NoError(t, err)

	// The round should reference the re-validation submission
	round, found := k.GetQualityRound(ctx, resp.RoundId)
	require.True(t, found)
	require.NotEmpty(t, round.SubmissionId)

	// The submission should contain the sample content
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	require.True(t, found)
	require.Equal(t, "original content", sub.Content)
	require.Equal(t, "technology", sub.Domain)
}

func TestContestSample_InvalidStakeString(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
	}))

	msg := &types.MsgContestSample{
		Challenger:  challenger1,
		SampleId:    "1",
		Stake:       "not-a-number",
		Reason:      "bad",
		ContestType: types.ContestType_CONTEST_TYPE_QUALITY,
	}

	_, err := k.ContestSample(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}
```

Add remaining tests to `sponsor_test.go`:

```go
func TestSponsorSample_RestoresEnergy_ZeroCap(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Sample with zero energy cap — energy stays at 0
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
		Energy:    0,
		EnergyCap: 0,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "1000000",
		DurationBlocks: 100,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)

	sample, found := k.GetSample(ctx, "1")
	require.True(t, found)
	require.Equal(t, uint64(0), sample.Energy)
}

func TestSponsorSample_MultipleSponsorships(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "1",
		Content:   "test",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
		EnergyCap: 1000,
	}))

	// First sponsorship
	msg1 := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "1000000",
		DurationBlocks: 100,
	}
	_, err := k.SponsorSample(ctx, msg1)
	require.NoError(t, err)

	// Second sponsorship
	msg2 := &types.MsgSponsorSample{
		Sponsor:        sponsor1,
		SampleId:       "1",
		Amount:         "2000000",
		DurationBlocks: 200,
	}
	_, err = k.SponsorSample(ctx, msg2)
	require.NoError(t, err)

	sample, found := k.GetSample(ctx, "1")
	require.True(t, found)
	require.Equal(t, "3000000", sample.PatronageAmount) // 1M + 2M
	// First: 100 + 100 = 200. Second: extend from 200 by 200 = 400
	require.Equal(t, uint64(400), sample.PatronageExpiryBlock)
}
```

**Step 2: Run all tests**

Run: `go test ./x/knowledge/keeper/ -run "TestContestSample|TestSponsorSample" -v -count=1`
Expected: PASS (≥25 tests)

**Step 3: Run full module tests**

Run: `go test ./x/knowledge/... -v -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add x/knowledge/keeper/contest_test.go x/knowledge/keeper/sponsor_test.go
git commit -m "test(knowledge): comprehensive contest and sponsor tests, ≥25 coverage (R37-4)"
```

---

### Task 6: Final Verification

**Step 1: Run go vet**

Run: `go vet ./x/knowledge/...`
Expected: clean

**Step 2: Count tests**

Run: `go test ./x/knowledge/keeper/ -run "TestContestSample|TestSponsorSample" -v -count=1 2>&1 | grep -c "=== RUN"`
Expected: ≥ 25

**Step 3: Run full test suite**

Run: `go test ./x/knowledge/... -count=1`
Expected: all pass

**Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix(knowledge): final R37-4 cleanup"
```
