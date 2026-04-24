package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Route B Wave 4: economic realignment — shared helpers ───────────────
//
// This file implements the alignment-critical primitives that Wave 4 relies
// on: Popper-weighted training-value weight (TVW), the is-ought wall in
// money, verifier-panel verdict finalization, and escrow/disbursement
// bookkeeping against the KnowledgeTrainingFund module account.
//
// Design invariant (audit-worthy): no payout in this file is ever gated on
// the payer's (or the claimant's) judgment. Every payout is driven by a
// signal that the PoT adjudication layer has already produced.

const bps uint64 = 1_000_000

// ─── Training-value weight (TVW) ────────────────────────────────────────

// TrainingValueWeightBreakdown exposes the component factors of TVW so
// callers (query handlers, auditors, tests) can inspect the computation.
type TrainingValueWeightBreakdown struct {
	BaseWeight             uint64 // survived falsification + 1
	MethodologyMultiplier  uint64 // BPS
	VindicationMultiplier  uint64 // BPS (>= bps when vindicated, else bps)
	SubmitterCalibration   uint64 // BPS snapshot at submission
	AxiomProximity         uint64 // BPS (closer to axiom → higher)
	HardeningMultiplier    uint64 // BPS — accelerating return on survived attacks
	Disproven              bool
	BlockedByIsOught       bool
	StatusIneligible       bool   // true if fact status bars training-value accrual
	Final                  uint64 // composed TVW
}

// Hardening parameters: each rate-limited corroboration bumps the
// multiplier by a fixed step, up to a cap. Pairs with the
// CorroborationCount-based BaseWeight to make the reward schedule
// accelerate with survived attacks — Popper's insight that a claim
// which has passed 100 tests is not 100× as credible as one which
// passed 1, it's exponentially more.
//
//	BaseWeight × HardeningMultiplier total contribution from corroboration:
//	   0 attacks   →  1  ×  1.0   =   1
//	  10 attacks   → 11  ×  1.5   =  16.5
//	  20 attacks   → 21  ×  2.0   =  42
//	  40 attacks   → 41  ×  3.0   = 123     (cap)
//	 100 attacks   → 101 ×  3.0   = 303     (cap)
const (
	hardeningPerCorroboration uint64 = 50_000    // +5% per survived attack
	hardeningMaxBps           uint64 = 3_000_000 // 3× cap
)

// ComputeTrainingValueWeight returns the composed Popper-weighted TVW for a
// fact. Disproven facts and ids resolving to NormativeCommitments return
// TVW=0 (is-ought wall). All factors are BPS-scaled; final is also BPS.
func (k Keeper) ComputeTrainingValueWeight(ctx context.Context, factID string) TrainingValueWeightBreakdown {
	var out TrainingValueWeightBreakdown

	// Is-ought wall: ids resolving to NormativeCommitments earn nothing.
	if _, ok := k.GetNormativeCommitment(ctx, factID); ok {
		out.BlockedByIsOught = true
		return out
	}

	fact, ok := k.GetFact(ctx, factID)
	if !ok || fact == nil {
		return out
	}

	// Clawback state: disproven → revenue zeroed.
	if fact.Status == types.FactStatus_FACT_STATUS_DISPROVEN || fact.RevenueClawbackBlock > 0 {
		out.Disproven = true
		return out
	}

	// Status gate: only facts whose truth-chain cleared verification (or
	// survive mid-adjudication) can accrue training value. PENDING and
	// PROVISIONAL never passed a round; REVOKED / EXPIRED / PRUNED /
	// SUPERSEDED / AT_RISK have been invalidated or retired. UNSPECIFIED
	// is a malformed record. All earn zero TVW. CONTESTED and CHALLENGED
	// remain payable during their adjudication window — falsification
	// closes the door via the DISPROVEN branch above.
	switch fact.Status {
	case types.FactStatus_FACT_STATUS_VERIFIED,
		types.FactStatus_FACT_STATUS_ACTIVE,
		types.FactStatus_FACT_STATUS_CONTESTED,
		types.FactStatus_FACT_STATUS_CHALLENGED:
		// eligible
	default:
		out.StatusIneligible = true
		return out
	}

	// Base: survived falsification attempts + 1. Popper, not popularity.
	out.BaseWeight = fact.CorroborationCount + 1

	// Methodology normalization.
	out.MethodologyMultiplier = k.getMethodologyNormalizationBps(ctx, fact.MethodId)

	// Vindication multiplier: facts vindicated from minority status.
	out.VindicationMultiplier = bps
	if k.factWasVindicatedFromMinority(ctx, fact.Id) {
		params, _ := k.GetParams(ctx)
		mult := params.VindicationTvwMultiplierBps
		if mult == 0 {
			mult = 2_500_000
		}
		out.VindicationMultiplier = mult
	}

	// Calibration snapshot at submission (frozen at fact acceptance; non-gameable).
	// Facts created via createFactFromClaim always carry a snapshot. Legacy
	// records (pre-snapshot) or facts seeded via authority injection default
	// to neutral so they neither farm retroactive calibration nor punish the
	// submitter for it.
	out.SubmitterCalibration = fact.SubmitterCalibrationSnapshotBps
	if out.SubmitterCalibration == 0 {
		out.SubmitterCalibration = bps / 2
	}

	// Axiom proximity: axioms earn more than deep derivations. Simple
	// linear decay 1.0× at depth 0 → 0.5× at depth 10+.
	out.AxiomProximity = axiomProximityBps(fact.AxiomDistance)

	// Hardening: accelerating return on corroboration. BaseWeight already
	// scales linearly (each survived attack adds 1); hardening adds a
	// multiplicative bonus that tops out at the cap. The combination
	// rewards facts that have run a long gauntlet exponentially more
	// than facts that have barely been tested, encouraging submitters to
	// welcome stress-testing of their own claims.
	out.HardeningMultiplier = bps + fact.CorroborationCount*hardeningPerCorroboration
	if out.HardeningMultiplier > hardeningMaxBps {
		out.HardeningMultiplier = hardeningMaxBps
	}

	// Compose: final = base × meth × vind × cal × proximity × hardening.
	final := uint64(out.BaseWeight) * bps
	final = safeMulDivTVW(final, out.MethodologyMultiplier, bps)
	final = safeMulDivTVW(final, out.VindicationMultiplier, bps)
	final = safeMulDivTVW(final, out.SubmitterCalibration, bps)
	final = safeMulDivTVW(final, out.AxiomProximity, bps)
	final = safeMulDivTVW(final, out.HardeningMultiplier, bps)
	out.Final = final
	return out
}

func (k Keeper) getMethodologyNormalizationBps(ctx context.Context, methodID string) uint64 {
	if methodID == "" {
		methodID = "M-LEGACY"
	}
	params, err := k.GetParams(ctx)
	if err != nil || params.MethodologyNormalizationBps == nil {
		return bps
	}
	if v, ok := params.MethodologyNormalizationBps[methodID]; ok && v > 0 {
		return v
	}
	return bps
}

func axiomProximityBps(axiomDistance uint32) uint64 {
	// Linear: 1.0× at 0, dropping 50,000 BPS (5%) per hop; floor at 0.5×.
	dec := uint64(axiomDistance) * 50_000
	if dec >= 500_000 {
		return 500_000
	}
	return bps - dec
}

// factWasVindicatedFromMinority returns true when a VindicationRecord exists
// for this fact — an artifact that minority voters earned refunds because
// their dissent later proved correct.
func (k Keeper) factWasVindicatedFromMinority(ctx context.Context, factID string) bool {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.VindicationRecordPrefixForFact(factID)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return false
	}
	defer iter.Close()
	return iter.Valid()
}

// safeMulDivTVW computes a * b / denom without overflow for the ranges TVW uses.
func safeMulDivTVW(a, b, denom uint64) uint64 {
	if denom == 0 {
		return 0
	}
	ai := sdkmath.NewIntFromUint64(a)
	bi := sdkmath.NewIntFromUint64(b)
	di := sdkmath.NewIntFromUint64(denom)
	res := ai.Mul(bi).Quo(di)
	if !res.IsUint64() {
		return ^uint64(0)
	}
	return res.Uint64()
}

// ─── Is-ought wall ───────────────────────────────────────────────────────

// FilterIsOughtIds partitions a fact_id slice into {facts, rejected-commitments}.
// Rejected entries are NormativeCommitments — ought-claims. Accepting them
// into a ContributionRecord would launder ought-claims into training revenue.
func (k Keeper) FilterIsOughtIds(ctx context.Context, ids []string) (facts []string, rejectedCommitments []string) {
	seen := make(map[string]struct{})
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, isCommitment := k.GetNormativeCommitment(ctx, id); isCommitment {
			rejectedCommitments = append(rejectedCommitments, id)
			continue
		}
		facts = append(facts, id)
	}
	return facts, rejectedCommitments
}

// ─── Revenue clawback (disproval hook) ───────────────────────────────────

// ClawbackOnDisproval zeroes future revenue for a fact and returns the
// amount removed from the submitter's calibration (BPS, clamped).
// Called from the disproval path — invariant: only runs once per fact.
func (k Keeper) ClawbackOnDisproval(ctx context.Context, factID string) error {
	fact, ok := k.GetFact(ctx, factID)
	if !ok || fact == nil {
		return nil
	}
	if fact.RevenueClawbackBlock > 0 {
		return nil // already clawed
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	fact.RevenueClawbackBlock = uint64(sdkCtx.BlockHeight())
	if err := k.SetFact(ctx, fact); err != nil {
		return err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.training_revenue_clawed",
		sdk.NewAttribute("fact_id", factID),
		sdk.NewAttribute("submitter", fact.Submitter),
		sdk.NewAttribute("cleared_recent", fact.TrainingRevenueEarnedRecent),
	))
	return nil
}

// ─── Training fund bookkeeping ────────────────────────────────────────────

// EscrowAugmentationBounty transfers escrow coins from sponsor into the
// KnowledgeTrainingFund module account and stamps the bookkeeping key.
func (k Keeper) EscrowAugmentationBounty(ctx context.Context, sponsor string, bountyID string, amount sdkmath.Int) error {
	if amount.IsZero() {
		return nil
	}
	addr, err := sdk.AccAddressFromBech32(sponsor)
	if err != nil {
		return fmt.Errorf("invalid sponsor address: %w", err)
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, addr, types.TrainingFundModuleName, coins); err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.TrainingFundEscrowLockedKey(bountyID), []byte(amount.String()))
}

// ReleaseAugmentationPayout debits from the locked escrow and pays out.
func (k Keeper) ReleaseAugmentationPayout(ctx context.Context, bountyID, recipient string, amount sdkmath.Int) error {
	if amount.IsZero() {
		return nil
	}
	recipAddr, err := sdk.AccAddressFromBech32(recipient)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.TrainingFundModuleName, recipAddr, coins); err != nil {
		return err
	}
	return k.reduceEscrowLocked(ctx, bountyID, amount)
}

// ForfeitAugmentationDeposit moves a failed-sponsor deposit into the
// vesting_rewards research fund (keeps research-fund accounting consistent).
func (k Keeper) ForfeitAugmentationDeposit(ctx context.Context, bountyID string, amount sdkmath.Int) error {
	if amount.IsZero() {
		return nil
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	// Send from training fund → module-level deposit; reuse research fund
	// when the vesting_rewards keeper is present.
	if k.vestingRewardsKeeper != nil {
		if err := k.vestingRewardsKeeper.DepositToResearchFund(ctx, types.TrainingFundModuleName, coins); err != nil {
			return err
		}
	} else {
		// Fallback: keep in training fund as accumulated reserve.
	}
	return k.reduceEscrowLocked(ctx, bountyID, amount)
}

// ReturnEscrowToSponsor refunds any unused escrow on bounty expiry, minus
// the kept-market-open fee.
//
// Atomicity note: the sponsor refund is the primary obligation; the fee
// garnishment is secondary and failure-tolerant. If the fee deposit fails
// (e.g. the research-fund module isn't wired in a test harness), we log
// and continue — the sponsor is made whole and the bookkeeping stays
// consistent. This prevents bounties from being re-processed in an
// infinite loop across heartbeats.
func (k Keeper) ReturnEscrowToSponsor(ctx context.Context, bountyID, sponsor string, amount sdkmath.Int, feeBps uint64) error {
	if amount.IsZero() {
		return nil
	}
	fee := amount.Mul(sdkmath.NewIntFromUint64(feeBps)).Quo(sdkmath.NewIntFromUint64(bps))
	refund := amount.Sub(fee)
	if !refund.IsNegative() && !refund.IsZero() {
		addr, err := sdk.AccAddressFromBech32(sponsor)
		if err != nil {
			return fmt.Errorf("invalid sponsor address: %w", err)
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.TrainingFundModuleName, addr, sdk.NewCoins(sdk.NewCoin("uzrn", refund))); err != nil {
			return err
		}
	}
	if !fee.IsZero() && k.vestingRewardsKeeper != nil {
		// Non-blocking: fee deposit is a garnishment, not a primary obligation.
		// A failure here (e.g. research fund not wired) must not invalidate
		// the refund that already succeeded.
		if err := k.vestingRewardsKeeper.DepositToResearchFund(ctx, types.TrainingFundModuleName, sdk.NewCoins(sdk.NewCoin("uzrn", fee))); err != nil {
			k.Logger(ctx).Warn("augmentation expiry fee forfeiture skipped",
				"bounty_id", bountyID, "fee", fee.String(), "error", err.Error())
		}
	}
	return k.reduceEscrowLocked(ctx, bountyID, amount)
}

func (k Keeper) reduceEscrowLocked(ctx context.Context, bountyID string, amount sdkmath.Int) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.TrainingFundEscrowLockedKey(bountyID)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil
	}
	cur, ok := sdkmath.NewIntFromString(string(bz))
	if !ok {
		return nil
	}
	next := cur.Sub(amount)
	if next.IsNegative() {
		next = sdkmath.ZeroInt()
	}
	if next.IsZero() {
		return store.Delete(key)
	}
	return store.Set(key, []byte(next.String()))
}

// GetEscrowLocked returns the remaining locked escrow for a bounty (uzrn as
// sdkmath.Int). Zero if untracked.
func (k Keeper) GetEscrowLocked(ctx context.Context, bountyID string) sdkmath.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TrainingFundEscrowLockedKey(bountyID))
	if err != nil || bz == nil {
		return sdkmath.ZeroInt()
	}
	v, ok := sdkmath.NewIntFromString(string(bz))
	if !ok {
		return sdkmath.ZeroInt()
	}
	return v
}

// ─── Contribution challenge CRUD ────────────────────────────────────────

// SetContributionChallenge stores a challenge and indexes it by model and
// by open-status.
func (k Keeper) SetContributionChallenge(ctx context.Context, c *types.ContributionChallenge) error {
	if c == nil || c.Id == "" {
		return fmt.Errorf("invalid challenge")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(c)
	if err != nil {
		return err
	}
	if err := store.Set(types.ContributionChallengeKey(c.Id), bz); err != nil {
		return err
	}
	if err := store.Set(types.ContributionChallengeByModelKey(c.ModelId, c.Id), []byte{1}); err != nil {
		return err
	}
	if c.Status == "" || c.Status == "open" {
		return store.Set(types.OpenContributionChallengeKey(c.Id), []byte{1})
	}
	return store.Delete(types.OpenContributionChallengeKey(c.Id))
}

// GetContributionChallenge fetches a challenge by id.
func (k Keeper) GetContributionChallenge(ctx context.Context, id string) (*types.ContributionChallenge, bool) {
	if id == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContributionChallengeKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var c types.ContributionChallenge
	if err := proto.Unmarshal(bz, &c); err != nil {
		return nil, false
	}
	return &c, true
}

// IterateOpenContributionChallenges yields every open challenge.
func (k Keeper) IterateOpenContributionChallenges(ctx context.Context, cb func(*types.ContributionChallenge) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.OpenContributionChallengeKeyPrefix, prefixEndBytes(types.OpenContributionChallengeKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		id := string(key[len(types.OpenContributionChallengeKeyPrefix):])
		c, ok := k.GetContributionChallenge(ctx, id)
		if !ok {
			continue
		}
		if cb(c) {
			return
		}
	}
}

// ─── Training-fund disbursements ────────────────────────────────────────

// SetTrainingFundDisbursement stores (and indexes) a disbursement.
func (k Keeper) SetTrainingFundDisbursement(ctx context.Context, d *types.TrainingFundDisbursement) error {
	if d == nil || d.Id == "" {
		return fmt.Errorf("invalid disbursement")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(d)
	if err != nil {
		return err
	}
	if err := store.Set(types.TrainingFundDisbursementKey(d.Id), bz); err != nil {
		return err
	}
	if err := store.Set(types.TrainingFundDisbursementByModelKey(d.ModelId, d.Id), []byte{1}); err != nil {
		return err
	}
	// Bookkeeping: vesting bucket.
	if d.VestingAmount != "" && d.VestingAmount != "0" {
		return store.Set(types.TrainingFundVestingKey(d.Id), []byte(d.VestingAmount))
	}
	return store.Delete(types.TrainingFundVestingKey(d.Id))
}

// GetTrainingFundDisbursement fetches a disbursement by id.
func (k Keeper) GetTrainingFundDisbursement(ctx context.Context, id string) (*types.TrainingFundDisbursement, bool) {
	if id == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TrainingFundDisbursementKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var d types.TrainingFundDisbursement
	if err := proto.Unmarshal(bz, &d); err != nil {
		return nil, false
	}
	return &d, true
}

// IterateTrainingFundDisbursements yields all disbursements.
func (k Keeper) IterateTrainingFundDisbursements(ctx context.Context, cb func(*types.TrainingFundDisbursement) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.TrainingFundDisbursementKeyPrefix, prefixEndBytes(types.TrainingFundDisbursementKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var d types.TrainingFundDisbursement
		if err := proto.Unmarshal(iter.Value(), &d); err != nil {
			continue
		}
		if cb(&d) {
			return
		}
	}
}

// ─── Verifier-panel verdict finalization ─────────────────────────────────

// RecordAugmentationVote adds a verifier's vote and — if consensus is
// reached — finalizes the verdict. Returns (finalized, verdict).
// Never called by the sponsor or the submitter.
//
// Wave 10 Sybil fix: consensus is STAKE-WEIGHTED, not headcount.
// Each verifier's vote is weighted by the bonded stake they control at
// the moment of voting (frozen on the augmentation record so bond/unbond
// between vote and tally can't manipulate the outcome). A Sybil ring
// running three zero-stake addresses has zero weight and cannot finalize
// a verdict regardless of their agreement. To carry consensus, an
// attacker must now control bonded stake proportional to the consensus
// threshold — economically equivalent to a validator-set attack on the
// underlying chain, which dwarfs the cost of a multi-address spoof.
//
// Non-validator voters are not rejected (dissent is always allowed) but
// their votes carry zero weight and do not advance the consensus tally.
// A minimum total-stake quorum ensures a lone stake-bearing voter can't
// finalize alone.
func (k Keeper) RecordAugmentationVote(ctx context.Context, augID, verifier string, vote types.AugmentationVerdict) (bool, types.AugmentationVerdict, error) {
	aug, ok := k.GetAugmentation(ctx, augID)
	if !ok {
		return false, types.AugmentationVerdict_AUGMENTATION_VERDICT_PENDING, fmt.Errorf("augmentation %s not found", augID)
	}
	if aug.Verdict != types.AugmentationVerdict_AUGMENTATION_VERDICT_PENDING {
		return false, aug.Verdict, fmt.Errorf("augmentation verdict is already final")
	}
	if vote == types.AugmentationVerdict_AUGMENTATION_VERDICT_PENDING {
		return false, aug.Verdict, fmt.Errorf("pending is not a valid vote")
	}

	// Guard: sponsor and submitter may not vote.
	if aug.Submitter == verifier {
		return false, aug.Verdict, fmt.Errorf("submitter may not vote on their own variant")
	}
	if aug.BountyId != "" {
		if bounty, ok := k.GetAugmentationBounty(ctx, aug.BountyId); ok && bounty.SponsorAddress == verifier {
			return false, aug.Verdict, fmt.Errorf("sponsor may not vote as verifier")
		}
	}

	// Dedup: one vote per verifier.
	for _, v := range aug.VerdictVoters {
		if v == verifier {
			return false, aug.Verdict, fmt.Errorf("verifier already voted")
		}
	}

	// Snapshot the verifier's effective stake at vote time. Non-validator
	// addresses return stake=0 (and nil error for not-found via the keeper
	// contract); zero-stake votes are still recorded for audit but carry
	// no weight in the consensus tally.
	var stakeAtVote uint64
	if k.stakingKeeper != nil {
		if s, err := k.stakingKeeper.GetEffectiveStake(ctx, verifier); err == nil {
			stakeAtVote = s
		}
	}

	// Snapshot the verifier's skill signal at vote time. Wave 15c: when
	// the target fact has a domain, use DOMAIN-SPECIFIC qualification as
	// the primary panel weight — a physics-domain augmentation should
	// be adjudicated by physics-qualified voters, and cross-domain
	// credentials earn no credit regardless of how strong they are
	// globally. Voters unqualified in the target domain are floored at
	// the tally's panelCalibrationFloorBps, preserving liveness (they
	// still vote, still carry some weight, but cannot dominate).
	//
	// Global calibration is used only as a fallback when:
	//   - the target fact has no domain (legacy / migration records), OR
	//   - no DomainQualificationKeeper is wired (genesis / early bootstrap).
	var calibrationAtVote uint64
	domain := ""
	if aug.OriginalFactId != "" {
		if origFact, ok := k.GetFact(ctx, aug.OriginalFactId); ok && origFact != nil {
			domain = origFact.Domain
		}
	}
	if domain != "" && k.domainQualificationKeeper != nil {
		// Domain path: ONLY domain qualification counts. Unqualified
		// voters get weight 0, which the tally floors to 20%.
		if w, err := k.domainQualificationKeeper.GetQualificationWeight(ctx, verifier, domain); err == nil && w > 0 {
			// x/qualification reports weight in the 1..100 range; scale
			// to BPS (0..1_000_000). Cap at BPS in case governance ever
			// raises the qualification scale past 100.
			scaled := w * 10_000
			if scaled > bps {
				scaled = bps
			}
			calibrationAtVote = scaled
		}
	} else {
		// Fallback path: no domain on target fact, or no qualification
		// adapter wired. Use global calibration.
		if cal, ok := k.GetAgentCalibration(ctx, verifier); ok && cal != nil {
			calibrationAtVote = cal.CalibrationScoreBps
		}
	}

	aug.VerdictVoters = append(aug.VerdictVoters, verifier)
	aug.VerdictVotes = append(aug.VerdictVotes, vote)
	aug.VerdictVoteStakes = append(aug.VerdictVoteStakes, stakeAtVote)
	aug.VerdictVoteCalibrationBps = append(aug.VerdictVoteCalibrationBps, calibrationAtVote)
	if err := k.SetAugmentation(ctx, aug); err != nil {
		return false, aug.Verdict, err
	}

	// Consensus check.
	params, err := k.GetParams(ctx)
	if err != nil {
		return false, aug.Verdict, err
	}
	minVotes := params.ReformulationMinPanelVotes
	if minVotes == 0 {
		minVotes = 3
	}
	if uint64(len(aug.VerdictVoters)) < minVotes {
		return false, aug.Verdict, nil
	}
	consensusBps := params.ReformulationConsensusBps
	if consensusBps == 0 {
		consensusBps = 666_000
	}

	// Reputation-weighted tally (Wave 15). Each voter's effective panel
	// weight is stake × max(floor, calibration) / BPS. Stake alone isn't
	// enough to dominate — a verifier who has shown they can't tell
	// truth from falsehood carries reduced weight regardless of bond.
	// The calibration floor prevents lockup in the early chain state
	// before anyone has built up a record; it also means NO stake-
	// bearing verifier ever drops to zero weight (stake × floor > 0).
	const panelCalibrationFloorBps uint64 = 200_000 // 20% — bare minimum signal credibility
	stakeTally := make(map[types.AugmentationVerdict]uint64)
	var totalStake uint64
	for i, v := range aug.VerdictVotes {
		stake := uint64(0)
		if i < len(aug.VerdictVoteStakes) {
			stake = aug.VerdictVoteStakes[i]
		}
		cal := uint64(0)
		if i < len(aug.VerdictVoteCalibrationBps) {
			cal = aug.VerdictVoteCalibrationBps[i]
		}
		if cal < panelCalibrationFloorBps {
			cal = panelCalibrationFloorBps
		}
		// Effective weight = stake × calibration / BPS. A 100M stake at
		// 50% calibration weighs the same as a 50M stake at 100%.
		effectiveWeight := safeMulDiv(stake, cal, bps)
		stakeTally[v] += effectiveWeight
		totalStake += effectiveWeight
	}
	if totalStake == 0 {
		// No stake-bearing votes yet; wait for more.
		return false, aug.Verdict, nil
	}
	var winner types.AugmentationVerdict
	var winnerStake uint64
	for v, s := range stakeTally {
		if s*bps/totalStake >= consensusBps && s > winnerStake {
			winner = v
			winnerStake = s
		}
	}
	if winner == types.AugmentationVerdict_AUGMENTATION_VERDICT_PENDING {
		return false, aug.Verdict, nil
	}
	// Finalize.
	return true, winner, nil
}

// ApplyFinalizedAugmentationVerdict persists the verdict, updates augmentation
// state, and — for passing verdicts — releases the escrow payout. Drift and
// inferior verdicts archive but do not pay.
func (k Keeper) ApplyFinalizedAugmentationVerdict(ctx context.Context, augID string, verdict types.AugmentationVerdict) error {
	aug, ok := k.GetAugmentation(ctx, augID)
	if !ok {
		return fmt.Errorf("augmentation %s not found", augID)
	}
	aug.Verdict = verdict
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	aug.VerdictBlock = uint64(sdkCtx.BlockHeight())

	passing := verdict == types.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT ||
		verdict == types.AugmentationVerdict_AUGMENTATION_VERDICT_SUPERIOR

	if passing && aug.BountyId != "" {
		bounty, ok := k.GetAugmentationBounty(ctx, aug.BountyId)
		if !ok {
			return fmt.Errorf("bounty %s missing", aug.BountyId)
		}
		if bounty.AcceptedVariants >= bounty.MaxVariants {
			// Saturated after the fact — archive without payout.
			aug.AcceptanceNote = "saturated before finalize"
		} else if !aug.SponsorVetoed {
			amount := sdkmath.NewIntFromUint64(bounty.RewardPerVariant)
			if verdict == types.AugmentationVerdict_AUGMENTATION_VERDICT_SUPERIOR {
				params, _ := k.GetParams(ctx)
				bonusBps := params.ReformulationSuperiorBonusBps
				if bonusBps == 0 {
					bonusBps = 500_000
				}
				bonus := amount.Mul(sdkmath.NewIntFromUint64(bonusBps)).Quo(sdkmath.NewIntFromUint64(bps))
				amount = amount.Add(bonus)
			}
			if err := k.ReleaseAugmentationPayout(ctx, aug.BountyId, aug.Submitter, amount); err != nil {
				return err
			}
			aug.PayoutAmount = amount.String()
			aug.Accepted = true
			aug.AcceptedAtBlock = aug.VerdictBlock
			bounty.AcceptedVariants++
			if bounty.AcceptedVariants >= bounty.MaxVariants {
				bounty.Active = false
			}
			if err := k.SetAugmentationBounty(ctx, bounty); err != nil {
				return err
			}
		}
	} else if passing && aug.BountyId == "" {
		// Volunteer passing — no payout, but mark accepted.
		aug.Accepted = true
		aug.AcceptedAtBlock = aug.VerdictBlock
	}

	// For passing verdicts, also write a REFORMULATES edge from variant →
	// original fact (provides queryable knowledge-graph evidence).
	if passing {
		rel := &types.FactRelation{
			SourceFactId:   aug.Id, // variant-side
			TargetFactId:   aug.OriginalFactId,
			Relation:       types.RelationType_RELATION_TYPE_REFORMULATES,
			CreatedAtBlock: aug.VerdictBlock,
		}
		// The relation writer tolerates missing source-fact (the variant is an
		// Augmentation, not a Fact). We still record it in the typed-edge index
		// for downstream corpus export.
		_ = k.SetFactRelation(ctx, rel)
	}

	if err := k.SetAugmentation(ctx, aug); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.augmentation_verdict_finalized",
		sdk.NewAttribute("augmentation_id", aug.Id),
		sdk.NewAttribute("original_fact_id", aug.OriginalFactId),
		sdk.NewAttribute("verdict", verdict.String()),
		sdk.NewAttribute("payout", aug.PayoutAmount),
	))
	return nil
}

// ─── Drift corpus iteration ──────────────────────────────────────────────

// IterateDriftAugmentations yields every augmentation with a DRIFT or
// INFERIOR verdict — the negative-training-signal corpus for meaning
// preservation learning.
func (k Keeper) IterateDriftAugmentations(ctx context.Context, cb func(*types.Augmentation) bool) {
	k.IterateAugmentations(ctx, func(a *types.Augmentation) bool {
		if a.Verdict == types.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT ||
			a.Verdict == types.AugmentationVerdict_AUGMENTATION_VERDICT_INFERIOR {
			return cb(a)
		}
		return false
	})
}

// IterateAugmentations — helper used by drift corpus; walks every augmentation.
func (k Keeper) IterateAugmentations(ctx context.Context, cb func(*types.Augmentation) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AugmentationKeyPrefix, prefixEndBytes(types.AugmentationKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var a types.Augmentation
		if err := proto.Unmarshal(iter.Value(), &a); err != nil {
			continue
		}
		if cb(&a) {
			return
		}
	}
}
