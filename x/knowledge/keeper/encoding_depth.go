package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Encoding Depth & Type-Specific Decay (R52) ────────────────────────────
//
// Completes the STM → LTM transfer model:
//
// 1. InitializeFitnessWithDepth: replaces flat 0.5 initial score with
//    encoding-depth-based score (0.3-0.8) computed from quality round.
//
// 2. GetTypeSpecificDecayRate: applies memory class modifier so facts
//    persist, events fade, and skills endure.

// ─── InitializeFitnessWithDepth ─────────────────────────────────────────────

// InitializeFitnessWithDepth creates a fitness record with initial score
// based on encoding depth from the quality round. Deeper encoding = higher
// initial fitness = stronger initial memory trace.
//
// This replaces the flat 0.5 for TDUs that go through quality rounds.
func (k Keeper) InitializeFitnessWithDepth(
	ctx context.Context,
	sampleID string,
	originalStake sdkmath.Int,
	currentCycle uint64,
	consensusStrength sdkmath.LegacyDec,
	avgReviewerRep sdkmath.LegacyDec,
	stakeRatio sdkmath.LegacyDec,
	sampleType types.SampleType,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	memoryClass := types.ClassifyFromSampleType(sampleType)

	initialFitness := types.EncodingDepth(
		consensusStrength,
		avgReviewerRep,
		stakeRatio,
		memoryClass,
	)

	record := types.TDUFitnessRecord{
		SampleID:        sampleID,
		FitnessScore:    initialFitness.String(),
		OriginalStake:   originalStake.String(),
		LastSignalCycle: currentCycle,
		CycleCount:      0,
	}

	if err := k.SetFitnessRecord(ctx, record); err != nil {
		return err
	}

	// Store memory class for type-specific decay.
	if err := k.SetMemoryClass(ctx, sampleID, memoryClass); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEncodingDepthComputed,
		sdk.NewAttribute(types.AttributeSampleID, sampleID),
		sdk.NewAttribute(types.AttributeEncodingDepth, initialFitness.String()),
		sdk.NewAttribute(types.AttributeMemoryClass, memoryClass.String()),
		sdk.NewAttribute(types.AttributeInitialFitness, initialFitness.String()),
	))

	return nil
}

// ─── GetFullEffectiveDecayRate ───────────────────────────────────────────────

// GetFullEffectiveDecayRate computes the complete effective decay rate for a TDU,
// combining ALL memory system modifiers:
//
//   effective_decay = base_decay × tier_modifier × reconsolidation_penalty × type_modifier
//
// This is the final, complete decay rate formula incorporating:
//   - R50: Memory tier (Canonical=0×, Consolidated=0.2×, Active=0.7×, Working=1×)
//   - R51: Reconsolidation penalty (1.0 + 0.1 × uncorrected_count)
//   - R52: Memory class (Semantic=0.8×, Episodic=1.2×, Procedural=0.6×)
func (k Keeper) GetFullEffectiveDecayRate(ctx context.Context, sampleID string, baseDecay sdkmath.LegacyDec) sdkmath.LegacyDec {
	// Get R50 + R51 combined rate.
	tierAndReconRate := k.GetEffectiveDecayRate(ctx, sampleID, baseDecay)

	// Apply R52 type-specific modifier.
	memoryClass := k.GetMemoryClass(ctx, sampleID)
	typeModifier := memoryClass.DecayModifier()

	return tierAndReconRate.Mul(typeModifier)
}

// ─── Memory Class Storage ───────────────────────────────────────────────────

// SetMemoryClass stores the memory class for a TDU.
func (k Keeper) SetMemoryClass(ctx context.Context, sampleID string, class types.MemoryClass) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(types.MemoryClassKey(sampleID), []byte{byte(class)})
}

// GetMemoryClass retrieves the memory class for a TDU, defaulting to Semantic.
func (k Keeper) GetMemoryClass(ctx context.Context, sampleID string) types.MemoryClass {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.MemoryClassKey(sampleID))
	if err != nil || bz == nil || len(bz) == 0 {
		return types.MemoryClassSemantic // default
	}
	return types.MemoryClass(bz[0])
}

// ─── Compute Encoding Factors from Quality Round ────────────────────────────

// ComputeEncodingFactors extracts the encoding depth factors from a completed
// quality round. This bridges the quality round system to the memory system.
func (k Keeper) ComputeEncodingFactors(ctx context.Context, roundID string) (
	consensusStrength sdkmath.LegacyDec,
	avgReviewerRep sdkmath.LegacyDec,
	stakeRatio sdkmath.LegacyDec,
	err error,
) {
	round, found := k.GetQualityRound(ctx, roundID)
	if !found {
		return sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyOneDec(), fmt.Errorf("round not found: %s", roundID)
	}

	// 1. Consensus strength: reveals/commits ratio.
	revealCount := len(round.Reveals)
	commitCount := len(round.Commits)
	consensusStrength = ConsensusStrength(revealCount, commitCount)

	// 2. Average reviewer reputation: mean of majority reviewers.
	avgReviewerRep = k.computeAvgReviewerRep(ctx, round)

	// 3. Stake ratio: actual_stake / base_stake.
	stakeRatio = k.computeStakeRatio(ctx, round)

	return consensusStrength, avgReviewerRep, stakeRatio, nil
}

// computeAvgReviewerRep calculates the average reputation of revealing reviewers.
func (k Keeper) computeAvgReviewerRep(ctx context.Context, round *types.QualityRound) sdkmath.LegacyDec {
	if len(round.Reveals) == 0 {
		return sdkmath.LegacyNewDecWithPrec(5, 1) // default 0.5
	}

	// Get domain from the submission for reputation lookup.
	domain := ""
	if sub, found := k.GetSubmission(ctx, round.SubmissionId); found {
		domain = sub.Domain
	}

	total := sdkmath.LegacyZeroDec()
	count := 0

	for _, reveal := range round.Reveals {
		rep := k.GetDomainReputation(ctx, reveal.Verifier, domain)
		total = total.Add(rep)
		count++
	}

	if count == 0 {
		return sdkmath.LegacyNewDecWithPrec(5, 1)
	}
	return total.Quo(sdkmath.LegacyNewDec(int64(count)))
}

// GetDomainReputation retrieves an agent's domain reputation as a decimal.
func (k Keeper) GetDomainReputation(ctx context.Context, agentAddr, domain string) sdkmath.LegacyDec {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.AgentDomainReputationKey(agentAddr, domain)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return sdkmath.LegacyNewDecWithPrec(5, 1) // default 0.5
	}
	var rep types.AgentDomainReputation
	if err := json.Unmarshal(bz, &rep); err != nil {
		return sdkmath.LegacyNewDecWithPrec(5, 1)
	}
	score, err := sdkmath.LegacyNewDecFromStr(rep.Score)
	if err != nil {
		return sdkmath.LegacyNewDecWithPrec(5, 1)
	}
	return score
}

// computeStakeRatio calculates actual_stake / base_stake.
func (k Keeper) computeStakeRatio(ctx context.Context, round *types.QualityRound) sdkmath.LegacyDec {
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	if !found || sub.Stake == "" {
		return sdkmath.LegacyOneDec() // 1.0 = minimum
	}

	actualStake, ok := sdkmath.NewIntFromString(sub.Stake)
	if !ok || actualStake.IsZero() {
		return sdkmath.LegacyOneDec()
	}

	baseStake := sdkmath.NewInt(1000000) // 1 ZRN default (1_000_000 uzrn)

	if baseStake.IsZero() {
		return sdkmath.LegacyOneDec()
	}

	return sdkmath.LegacyNewDecFromInt(actualStake).Quo(sdkmath.LegacyNewDecFromInt(baseStake))
}
