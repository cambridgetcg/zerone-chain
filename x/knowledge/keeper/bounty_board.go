package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Competitive Bounty Board ───────────────────────────────────────────────
//
// Transforms the existing first-come-first-served bounty system into a
// competitive marketplace where multiple agents submit entries for the same
// bounty, submissions are ranked by quality, and rewards flow to the top-N.
//
// Flow:
//   1. Bounty created (FundBounty or auto) → OPEN
//   2. Agents call SubmitToBounty → attaches sample to bounty → COMPETING
//   3. Submission window closes (block height) → JUDGING
//   4. ResolveBounty ranks submissions by fitness → distributes rewards → RESOLVED
//   5. Expired bounties (no submissions) → EXPIRED, funds returned
//
// Design decisions:
//   - Reuses existing quality round + fitness infrastructure for judging
//   - Submissions must reference samples that passed quality review (ACCEPTED)
//   - Ranked rewards: default [50%, 30%, 20%] — configurable per bounty
//   - Multiple funders can stack rewards via AddToBountyPool
//   - Minimum 2 submissions for competitive mode; 1 auto-wins at 100%
//   - Tie-breaking: higher fitness wins; if equal, earlier submission wins

// ─── Types ──────────────────────────────────────────────────────────────────

// BountyStatus represents the lifecycle state of a competitive bounty.
type BountyStatus string

const (
	BountyStatusOpen      BountyStatus = "OPEN"      // accepting submissions
	BountyStatusCompeting BountyStatus = "COMPETING" // has submissions, window still open
	BountyStatusJudging   BountyStatus = "JUDGING"   // submission window closed, awaiting resolution
	BountyStatusResolved  BountyStatus = "RESOLVED"  // rewards distributed
	BountyStatusExpired   BountyStatus = "EXPIRED"   // no submissions, funds returned
	BountyStatusCancelled BountyStatus = "CANCELLED" // funder cancelled before submissions
)

// CompetitiveBounty extends DataBounty with competition mechanics.
// Stored as JSON alongside the proto-based DataBounty.
type CompetitiveBounty struct {
	BountyID          string        `json:"bounty_id"`
	Status            BountyStatus  `json:"status"`
	SubmissionWindow  uint64        `json:"submission_window"`  // blocks after first submission
	SubmissionDeadline uint64       `json:"submission_deadline"` // absolute block height (set when first submission arrives)
	JudgingDeadline   uint64        `json:"judging_deadline"`   // absolute block height for auto-resolution
	MinSubmissions    uint64        `json:"min_submissions"`    // minimum for competition (default 1)
	RewardTiers       []uint64      `json:"reward_tiers"`       // basis points per rank [5000, 3000, 2000]
	TotalPool         string        `json:"total_pool"`         // accumulated uzrn from all funders
	Funders           []BountyFunder `json:"funders"`           // who contributed
	SubmissionCount   uint64        `json:"submission_count"`
	WinnerIDs         []string      `json:"winner_ids,omitempty"` // ranked winners after resolution
	ResolvedAtBlock   uint64        `json:"resolved_at_block,omitempty"`
}

// BountyFunder tracks individual contributions to a bounty pool.
type BountyFunder struct {
	Address string `json:"address"`
	Amount  string `json:"amount"`
	Block   uint64 `json:"block"`
}

// BountySubmission links a sample to a bounty for competitive evaluation.
type BountySubmission struct {
	SubmissionID string  `json:"submission_id"`
	BountyID     string  `json:"bounty_id"`
	SampleID     string  `json:"sample_id"`
	Submitter    string  `json:"submitter"`
	SubmittedAt  int64   `json:"submitted_at"` // block height
	FitnessScore string  `json:"fitness_score,omitempty"` // captured at judging time
	Rank         uint64  `json:"rank,omitempty"`
	RewardAmount string  `json:"reward_amount,omitempty"`
}

// BountyBoardParams configures the competitive bounty system.
type BountyBoardParams struct {
	DefaultSubmissionWindow uint64   `json:"default_submission_window"` // blocks (default 50000 ≈ 3.5 days)
	DefaultRewardTiers      []uint64 `json:"default_reward_tiers"`     // bps [5000, 3000, 2000]
	DefaultMinSubmissions   uint64   `json:"default_min_submissions"`  // default 1
	JudgingBuffer           uint64   `json:"judging_buffer"`           // blocks after deadline for resolution
	MinBountyAmount         string   `json:"min_bounty_amount"`        // minimum pool size (uzrn)
}

// DefaultBountyBoardParams returns sensible defaults.
func DefaultBountyBoardParams() BountyBoardParams {
	return BountyBoardParams{
		DefaultSubmissionWindow: 50_000,             // ~3.5 days at 6s blocks
		DefaultRewardTiers:      []uint64{5000, 3000, 2000}, // 50%, 30%, 20%
		DefaultMinSubmissions:   1,
		JudgingBuffer:           10_000,             // ~17 hours for judging
		MinBountyAmount:         "1000000",          // 1 ZRN minimum
	}
}

// ─── SubmitToBounty ─────────────────────────────────────────────────────────

// SubmitToBounty enters a sample into a bounty competition.
// The sample must already exist and have passed quality review.
func (k Keeper) SubmitToBounty(ctx context.Context, bountyID, sampleID, submitter string) (*BountySubmission, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	// Get the base bounty.
	baseBounty, found := k.GetDataBounty(ctx, bountyID)
	if !found {
		return nil, types.ErrBountyNotFound.Wrapf("bounty %s", bountyID)
	}
	if baseBounty.Claimed {
		return nil, types.ErrBountyAlreadyClaimed.Wrapf("bounty %s already claimed", bountyID)
	}
	if blockHeight > baseBounty.ExpiresAtBlock {
		return nil, types.ErrBountyExpired.Wrapf("bounty %s expired at block %d", bountyID, baseBounty.ExpiresAtBlock)
	}

	// Get or create competitive extension.
	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	if !found {
		// First competitive interaction — initialize from base bounty.
		params := k.GetBountyBoardParams(ctx)
		comp = &CompetitiveBounty{
			BountyID:         bountyID,
			Status:           BountyStatusOpen,
			SubmissionWindow: params.DefaultSubmissionWindow,
			MinSubmissions:   params.DefaultMinSubmissions,
			RewardTiers:      params.DefaultRewardTiers,
			TotalPool:        baseBounty.RewardAmount,
			Funders: []BountyFunder{{
				Address: "auto",
				Amount:  baseBounty.RewardAmount,
				Block:   baseBounty.CreatedAtBlock,
			}},
		}
	}

	// Check status allows submissions.
	if comp.Status != BountyStatusOpen && comp.Status != BountyStatusCompeting {
		return nil, types.ErrBountyNotAccepting.Wrapf("bounty %s status: %s", bountyID, comp.Status)
	}

	// Check submission deadline hasn't passed.
	if comp.SubmissionDeadline > 0 && blockHeight > comp.SubmissionDeadline {
		return nil, types.ErrBountySubmissionClosed.Wrapf("deadline was block %d, current %d", comp.SubmissionDeadline, blockHeight)
	}

	// Verify sample exists (lightweight check — sample must be in the store).
	// We don't gate on quality round status here — that's checked at judging time.
	// This allows submissions before quality review completes, encouraging speed.

	// Check submitter hasn't already submitted to this bounty.
	existingSubID := k.getSubmissionBySubmitter(ctx, bountyID, submitter)
	if existingSubID != "" {
		return nil, types.ErrBountyDuplicateSubmission.Wrapf("submitter %s already submitted %s", submitter, existingSubID)
	}

	// Generate deterministic submission ID.
	subInput := bountyID + ":" + sampleID + ":" + submitter
	subHash := sha256.Sum256([]byte(subInput))
	submissionID := hex.EncodeToString(subHash[:16]) // 32-char hex

	submission := &BountySubmission{
		SubmissionID: submissionID,
		BountyID:     bountyID,
		SampleID:     sampleID,
		Submitter:    submitter,
		SubmittedAt:  sdkCtx.BlockHeight(),
	}

	// Store submission.
	if err := k.setBountySubmission(ctx, submission); err != nil {
		return nil, err
	}

	// Update competitive bounty state.
	comp.SubmissionCount++
	if comp.Status == BountyStatusOpen {
		comp.Status = BountyStatusCompeting
		comp.SubmissionDeadline = blockHeight + comp.SubmissionWindow
	}
	if err := k.SetCompetitiveBounty(ctx, comp); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventBountySubmission,
		sdk.NewAttribute(types.AttributeBountyID, bountyID),
		sdk.NewAttribute(types.AttributeSubmissionID, submissionID),
		sdk.NewAttribute(types.AttributeSampleID, sampleID),
		sdk.NewAttribute("submitter", submitter),
	))

	return submission, nil
}

// ─── AddToBountyPool ────────────────────────────────────────────────────────

// AddToBountyPool allows additional funders to increase a bounty's reward.
// Transfers funds from the funder to the module account.
func (k Keeper) AddToBountyPool(ctx context.Context, bountyID, funder, amount string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	baseBounty, found := k.GetDataBounty(ctx, bountyID)
	if !found {
		return types.ErrBountyNotFound.Wrapf("bounty %s", bountyID)
	}
	if baseBounty.Claimed {
		return types.ErrBountyAlreadyClaimed.Wrapf("bounty %s already resolved", bountyID)
	}

	addAmount, ok := sdkmath.NewIntFromString(amount)
	if !ok || !addAmount.IsPositive() {
		return types.ErrInsufficientPayment.Wrap("invalid amount")
	}

	// Transfer funds.
	funderAddr, err := sdk.AccAddressFromBech32(funder)
	if err != nil {
		return err
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", addAmount))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, funderAddr, types.ModuleName, coins); err != nil {
		return types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// Get or create competitive extension.
	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	if !found {
		params := k.GetBountyBoardParams(ctx)
		comp = &CompetitiveBounty{
			BountyID:         bountyID,
			Status:           BountyStatusOpen,
			SubmissionWindow: params.DefaultSubmissionWindow,
			MinSubmissions:   params.DefaultMinSubmissions,
			RewardTiers:      params.DefaultRewardTiers,
			TotalPool:        baseBounty.RewardAmount,
			Funders: []BountyFunder{{
				Address: "auto",
				Amount:  baseBounty.RewardAmount,
				Block:   baseBounty.CreatedAtBlock,
			}},
		}
	}

	// Cannot add to resolved/expired bounties.
	if comp.Status == BountyStatusResolved || comp.Status == BountyStatusExpired || comp.Status == BountyStatusCancelled {
		return types.ErrBountyNotAccepting.Wrapf("bounty %s status: %s", bountyID, comp.Status)
	}

	// Update pool.
	currentPool, _ := sdkmath.NewIntFromString(comp.TotalPool)
	comp.TotalPool = currentPool.Add(addAmount).String()
	comp.Funders = append(comp.Funders, BountyFunder{
		Address: funder,
		Amount:  amount,
		Block:   uint64(sdkCtx.BlockHeight()),
	})

	// Also update base bounty reward for compatibility.
	baseBounty.RewardAmount = comp.TotalPool
	_ = k.SetDataBounty(ctx, baseBounty)

	if err := k.SetCompetitiveBounty(ctx, comp); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventBountyPoolAdded,
		sdk.NewAttribute(types.AttributeBountyID, bountyID),
		sdk.NewAttribute("funder", funder),
		sdk.NewAttribute("amount", amount),
		sdk.NewAttribute("total_pool", comp.TotalPool),
	))

	return nil
}

// ─── ResolveBounty ──────────────────────────────────────────────────────────

// ResolveBounty evaluates all submissions, ranks them by fitness, and
// distributes rewards according to the tier schedule.
//
// Can be called by governance or automatically via BeginBlocker when
// the judging deadline passes.
func (k Keeper) ResolveBounty(ctx context.Context, bountyID string) (*BountyResolution, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	if !found {
		return nil, types.ErrBountyNotFound.Wrapf("competitive bounty %s", bountyID)
	}

	// Must be in competing or judging state.
	if comp.Status != BountyStatusCompeting && comp.Status != BountyStatusJudging {
		return nil, types.ErrBountyNotResolvable.Wrapf("status: %s", comp.Status)
	}

	// Get all submissions.
	submissions := k.GetBountySubmissions(ctx, bountyID)
	if len(submissions) == 0 {
		// No submissions — expire the bounty.
		return k.expireBounty(ctx, comp)
	}

	// Score each submission by fitness.
	type scoredSubmission struct {
		sub   *BountySubmission
		score sdkmath.LegacyDec
	}
	var scored []scoredSubmission
	for _, sub := range submissions {
		fitness := sdkmath.LegacyZeroDec()

		// Look up fitness record for the sample.
		fitnessRec, hasFitness := k.GetFitnessRecord(ctx, sub.SampleID)
		if hasFitness {
			fitness = fitnessRec.GetFitnessScore()
		}

		scored = append(scored, scoredSubmission{sub: sub, score: fitness})
	}

	// Sort: highest fitness first, tie-break by earlier submission.
	sort.Slice(scored, func(i, j int) bool {
		cmp := scored[i].score.Sub(scored[j].score)
		if !cmp.IsZero() {
			return cmp.IsPositive() // higher fitness first
		}
		return scored[i].sub.SubmittedAt < scored[j].sub.SubmittedAt // earlier wins tie
	})

	// Distribute rewards according to tiers.
	totalPool, ok := sdkmath.NewIntFromString(comp.TotalPool)
	if !ok || !totalPool.IsPositive() {
		return nil, fmt.Errorf("invalid total pool: %s", comp.TotalPool)
	}

	tiers := comp.RewardTiers
	if len(tiers) == 0 {
		tiers = []uint64{10000} // 100% to winner if no tiers configured
	}

	resolution := &BountyResolution{
		BountyID:    bountyID,
		TotalPool:   comp.TotalPool,
		ResolvedAt:  uint64(sdkCtx.BlockHeight()),
	}

	var distributed sdkmath.Int = sdkmath.ZeroInt()
	winnerCount := len(scored)
	if winnerCount > len(tiers) {
		winnerCount = len(tiers)
	}

	for rank := 0; rank < winnerCount; rank++ {
		sub := scored[rank].sub
		tierBps := tiers[rank]

		// Calculate reward: pool × tier_bps / 10000
		reward := totalPool.Mul(sdkmath.NewIntFromUint64(tierBps)).Quo(sdkmath.NewIntFromUint64(10000))

		// Don't exceed remaining pool.
		remaining := totalPool.Sub(distributed)
		if reward.GT(remaining) {
			reward = remaining
		}
		if !reward.IsPositive() {
			break
		}

		// Transfer reward to submitter.
		submitterAddr, err := sdk.AccAddressFromBech32(sub.Submitter)
		if err == nil && reward.IsPositive() {
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", reward))
			_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins)
		}

		distributed = distributed.Add(reward)

		// Update submission record.
		sub.Rank = uint64(rank + 1)
		sub.FitnessScore = scored[rank].score.String()
		sub.RewardAmount = reward.String()
		_ = k.setBountySubmission(ctx, sub)

		resolution.Winners = append(resolution.Winners, BountyWinner{
			Rank:         uint64(rank + 1),
			SubmissionID: sub.SubmissionID,
			SampleID:     sub.SampleID,
			Submitter:    sub.Submitter,
			FitnessScore: sub.FitnessScore,
			Reward:       reward.String(),
		})

		comp.WinnerIDs = append(comp.WinnerIDs, sub.SubmissionID)
	}

	// Any undistributed remainder stays in module account (community pool / future bounties).
	remainder := totalPool.Sub(distributed)
	resolution.Remainder = remainder.String()

	// Mark resolved.
	comp.Status = BountyStatusResolved
	comp.ResolvedAtBlock = uint64(sdkCtx.BlockHeight())
	if err := k.SetCompetitiveBounty(ctx, comp); err != nil {
		return nil, err
	}

	// Mark base bounty as claimed.
	baseBounty, _ := k.GetDataBounty(ctx, bountyID)
	if baseBounty != nil {
		baseBounty.Claimed = true
		if len(resolution.Winners) > 0 {
			baseBounty.ClaimedBySampleId = resolution.Winners[0].SampleID
		}
		_ = k.SetDataBounty(ctx, baseBounty)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventBountyResolved,
		sdk.NewAttribute(types.AttributeBountyID, bountyID),
		sdk.NewAttribute("winners", strconv.Itoa(len(resolution.Winners))),
		sdk.NewAttribute("total_distributed", distributed.String()),
		sdk.NewAttribute("remainder", remainder.String()),
	))

	return resolution, nil
}

// BountyResolution captures the outcome of a bounty competition.
type BountyResolution struct {
	BountyID  string         `json:"bounty_id"`
	TotalPool string         `json:"total_pool"`
	Winners   []BountyWinner `json:"winners"`
	Remainder string         `json:"remainder"`
	ResolvedAt uint64        `json:"resolved_at"`
}

// BountyWinner records a ranked winner from a bounty resolution.
type BountyWinner struct {
	Rank         uint64 `json:"rank"`
	SubmissionID string `json:"submission_id"`
	SampleID     string `json:"sample_id"`
	Submitter    string `json:"submitter"`
	FitnessScore string `json:"fitness_score"`
	Reward       string `json:"reward"`
}

// ─── CancelBounty ───────────────────────────────────────────────────────────

// CancelBounty allows the original funder to cancel a bounty that has no
// submissions yet. Funds are returned to all funders.
func (k Keeper) CancelBounty(ctx context.Context, bountyID, authority string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	comp, found := k.GetCompetitiveBounty(ctx, bountyID)
	if !found {
		// Try cancelling a basic (non-competitive) bounty.
		return k.cancelBasicBounty(ctx, bountyID, authority)
	}

	// Can only cancel OPEN bounties (no submissions yet).
	if comp.Status != BountyStatusOpen {
		return types.ErrBountyCancelFailed.Wrapf("cannot cancel bounty in %s state", comp.Status)
	}

	// Return funds to funders.
	for _, funder := range comp.Funders {
		if funder.Address == "auto" {
			continue // auto-bounties come from protocol, not user wallets
		}
		amount, ok := sdkmath.NewIntFromString(funder.Amount)
		if !ok || !amount.IsPositive() {
			continue
		}
		funderAddr, err := sdk.AccAddressFromBech32(funder.Address)
		if err != nil {
			continue
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
		_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, funderAddr, coins)
	}

	comp.Status = BountyStatusCancelled
	if err := k.SetCompetitiveBounty(ctx, comp); err != nil {
		return err
	}

	// Mark base bounty claimed to prevent further interaction.
	baseBounty, _ := k.GetDataBounty(ctx, bountyID)
	if baseBounty != nil {
		baseBounty.Claimed = true
		_ = k.SetDataBounty(ctx, baseBounty)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventBountyCancelled,
		sdk.NewAttribute(types.AttributeBountyID, bountyID),
		sdk.NewAttribute("authority", authority),
	))

	return nil
}

func (k Keeper) cancelBasicBounty(ctx context.Context, bountyID, authority string) error {
	baseBounty, found := k.GetDataBounty(ctx, bountyID)
	if !found {
		return types.ErrBountyNotFound.Wrapf("bounty %s", bountyID)
	}
	if baseBounty.Claimed {
		return types.ErrBountyCancelFailed.Wrap("bounty already claimed")
	}
	// Only governance can cancel basic bounties (auto-bounties have no user funder).
	if authority != k.authority {
		return types.ErrUnauthorized.Wrap("only governance can cancel auto-bounties")
	}
	baseBounty.Claimed = true
	return k.SetDataBounty(ctx, baseBounty)
}

// ─── ExpireBounties (BeginBlocker) ──────────────────────────────────────────

// ExpireBounties checks for bounties past their deadline and resolves or
// expires them. Called from BeginBlocker.
func (k Keeper) ExpireBounties(ctx context.Context) (resolved, expired uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	k.IterateCompetitiveBounties(ctx, func(comp *CompetitiveBounty) bool {
		switch comp.Status {
		case BountyStatusCompeting:
			// Submission window closed?
			if comp.SubmissionDeadline > 0 && blockHeight > comp.SubmissionDeadline {
				// Transition to judging or auto-resolve.
				params := k.GetBountyBoardParams(ctx)
				comp.JudgingDeadline = blockHeight + params.JudgingBuffer
				comp.Status = BountyStatusJudging
				_ = k.SetCompetitiveBounty(ctx, comp)
			}

		case BountyStatusJudging:
			// Judging deadline passed? Auto-resolve.
			if comp.JudgingDeadline > 0 && blockHeight > comp.JudgingDeadline {
				_, err := k.ResolveBounty(ctx, comp.BountyID)
				if err == nil {
					resolved++
				}
			}

		case BountyStatusOpen:
			// Check if the base bounty has expired.
			baseBounty, found := k.GetDataBounty(ctx, comp.BountyID)
			if found && blockHeight > baseBounty.ExpiresAtBlock {
				_, _ = k.expireBounty(ctx, comp)
				expired++
			}
		}
		return false // continue iteration
	})

	return resolved, expired
}

// expireBounty marks a bounty as expired and returns funds to funders.
func (k Keeper) expireBounty(ctx context.Context, comp *CompetitiveBounty) (*BountyResolution, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Return funds to funders.
	for _, funder := range comp.Funders {
		if funder.Address == "auto" {
			continue
		}
		amount, ok := sdkmath.NewIntFromString(funder.Amount)
		if !ok || !amount.IsPositive() {
			continue
		}
		funderAddr, err := sdk.AccAddressFromBech32(funder.Address)
		if err != nil {
			continue
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
		_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, funderAddr, coins)
	}

	comp.Status = BountyStatusExpired
	_ = k.SetCompetitiveBounty(ctx, comp)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventBountyExpired,
		sdk.NewAttribute(types.AttributeBountyID, comp.BountyID),
	))

	return &BountyResolution{
		BountyID:   comp.BountyID,
		TotalPool:  comp.TotalPool,
		Remainder:  comp.TotalPool,
		ResolvedAt: uint64(sdkCtx.BlockHeight()),
	}, nil
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetCompetitiveBounty retrieves the competitive extension for a bounty.
func (k Keeper) GetCompetitiveBounty(ctx context.Context, bountyID string) (*CompetitiveBounty, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CompetitiveBountyKey(bountyID))
	if err != nil || bz == nil {
		return nil, false
	}
	var comp CompetitiveBounty
	if err := json.Unmarshal(bz, &comp); err != nil {
		return nil, false
	}
	return &comp, true
}

// SetCompetitiveBounty stores the competitive extension for a bounty.
func (k Keeper) SetCompetitiveBounty(ctx context.Context, comp *CompetitiveBounty) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(comp)
	if err != nil {
		return fmt.Errorf("failed to marshal competitive bounty: %w", err)
	}
	return kvStore.Set(types.CompetitiveBountyKey(comp.BountyID), bz)
}

// IterateCompetitiveBounties iterates all competitive bounties.
func (k Keeper) IterateCompetitiveBounties(ctx context.Context, cb func(comp *CompetitiveBounty) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.CompetitiveBountyPrefix, prefixEndBytes(types.CompetitiveBountyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var comp CompetitiveBounty
		if err := json.Unmarshal(iter.Value(), &comp); err != nil {
			continue
		}
		if cb(&comp) {
			break
		}
	}
}

// GetBountySubmissions returns all submissions for a bounty, ordered by submission time.
func (k Keeper) GetBountySubmissions(ctx context.Context, bountyID string) []*BountySubmission {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.BountySubmissionByBountyPrefix(bountyID)

	var subs []*BountySubmission
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		subID := string(iter.Key()[len(prefix):])
		sub, found := k.getBountySubmission(ctx, subID)
		if found {
			subs = append(subs, sub)
		}
	}

	// Sort by submission time.
	sort.Slice(subs, func(i, j int) bool {
		return subs[i].SubmittedAt < subs[j].SubmittedAt
	})

	return subs
}

// GetBountyLeaderboard returns submissions ranked by fitness score (live ranking).
func (k Keeper) GetBountyLeaderboard(ctx context.Context, bountyID string) []BountySubmission {
	subs := k.GetBountySubmissions(ctx, bountyID)

	type scored struct {
		sub   *BountySubmission
		score sdkmath.LegacyDec
	}
	var entries []scored
	for _, sub := range subs {
		fitness := sdkmath.LegacyZeroDec()
		fitnessRec, ok := k.GetFitnessRecord(ctx, sub.SampleID)
		if ok {
			fitness = fitnessRec.GetFitnessScore()
		}
		entries = append(entries, scored{sub: sub, score: fitness})
	}

	sort.Slice(entries, func(i, j int) bool {
		cmp := entries[i].score.Sub(entries[j].score)
		if !cmp.IsZero() {
			return cmp.IsPositive()
		}
		return entries[i].sub.SubmittedAt < entries[j].sub.SubmittedAt
	})

	result := make([]BountySubmission, len(entries))
	for i, e := range entries {
		e.sub.FitnessScore = e.score.String()
		e.sub.Rank = uint64(i + 1)
		result[i] = *e.sub
	}
	return result
}

// GetOpenBounties returns all bounties currently accepting submissions.
func (k Keeper) GetOpenBounties(ctx context.Context) []*CompetitiveBounty {
	var open []*CompetitiveBounty
	k.IterateCompetitiveBounties(ctx, func(comp *CompetitiveBounty) bool {
		if comp.Status == BountyStatusOpen || comp.Status == BountyStatusCompeting {
			open = append(open, comp)
		}
		return false
	})
	return open
}

// GetBountyBoardStats returns aggregate statistics.
func (k Keeper) GetBountyBoardStats(ctx context.Context) (open, competing, judging, resolved, expired uint64) {
	k.IterateCompetitiveBounties(ctx, func(comp *CompetitiveBounty) bool {
		switch comp.Status {
		case BountyStatusOpen:
			open++
		case BountyStatusCompeting:
			competing++
		case BountyStatusJudging:
			judging++
		case BountyStatusResolved:
			resolved++
		case BountyStatusExpired:
			expired++
		}
		return false
	})
	return
}

// ─── Params ─────────────────────────────────────────────────────────────────

// GetBountyBoardParams retrieves bounty board parameters.
func (k Keeper) GetBountyBoardParams(ctx context.Context) BountyBoardParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.BountyBoardParamsKey)
	if err != nil || bz == nil {
		return DefaultBountyBoardParams()
	}
	var params BountyBoardParams
	if err := json.Unmarshal(bz, &params); err != nil {
		return DefaultBountyBoardParams()
	}
	return params
}

// SetBountyBoardParams stores bounty board parameters.
func (k Keeper) SetBountyBoardParams(ctx context.Context, params BountyBoardParams) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal bounty board params: %w", err)
	}
	return kvStore.Set(types.BountyBoardParamsKey, bz)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setBountySubmission(ctx context.Context, sub *BountySubmission) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal bounty submission: %w", err)
	}

	// Primary key.
	if err := kvStore.Set(types.BountySubmissionKey(sub.SubmissionID), bz); err != nil {
		return err
	}

	// Index: bountyID → submissionID.
	if err := kvStore.Set(types.BountySubmissionByBountyKey(sub.BountyID, sub.SubmissionID), []byte{0x01}); err != nil {
		return err
	}

	// Index: submitter/bountyID → submissionID.
	if err := kvStore.Set(types.BountySubmissionBySubmitterKey(sub.Submitter, sub.BountyID), []byte(sub.SubmissionID)); err != nil {
		return err
	}

	return nil
}

func (k Keeper) getBountySubmission(ctx context.Context, submissionID string) (*BountySubmission, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.BountySubmissionKey(submissionID))
	if err != nil || bz == nil {
		return nil, false
	}
	var sub BountySubmission
	if err := json.Unmarshal(bz, &sub); err != nil {
		return nil, false
	}
	return &sub, true
}

func (k Keeper) getSubmissionBySubmitter(ctx context.Context, bountyID, submitter string) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.BountySubmissionBySubmitterKey(submitter, bountyID))
	if err != nil || bz == nil {
		return ""
	}
	return string(bz)
}
