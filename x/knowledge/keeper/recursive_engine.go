package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R53: Recursive Self-Improvement Engine ─────────────────────────────────
//
// The PURPOSE of Zerone, expressed as code.
//
// The loop:
//   1. Models join consensus pool → stake ZRN → become verifiers
//   2. Quality rounds select model-verifiers → they vote on submissions
//   3. Every vote is automatically captured as training data
//   4. Capture fitness is determined by consensus alignment (no separate review!)
//   5. Captured data trains next-generation models
//   6. New generation challenges old → if better, succession happens
//   7. GOTO 1 with better models
//
// The consensus mechanism IS the training data pipeline.
// The act of running the blockchain IS the act of creating better AI.

// ─── Params ─────────────────────────────────────────────────────────────────

func (k Keeper) SetRecursiveEngineParams(ctx context.Context, params types.RecursiveEngineParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := params.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.RecursiveEngineParamsKey, bz)
}

func (k Keeper) GetRecursiveEngineParams(ctx context.Context) types.RecursiveEngineParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.RecursiveEngineParamsKey)
	if err != nil || bz == nil {
		return types.DefaultRecursiveEngineParams()
	}
	var params types.RecursiveEngineParams
	if err := params.Unmarshal(bz); err != nil {
		return types.DefaultRecursiveEngineParams()
	}
	return params
}

// ─── Verification Capture ───────────────────────────────────────────────────

// CaptureVerificationWork captures a model-agent's quality round vote as
// training data. Called automatically when a quality round resolves.
//
// The capture's fitness is determined by consensus alignment:
//   - Aligned vote → high fitness (good training example)
//   - Misaligned vote → low fitness (negative example — still valuable!)
//
// No separate quality round needed for captures. Consensus IS the quality check.
func (k Keeper) CaptureVerificationWork(
	ctx context.Context,
	roundID string,
	verifier string,
	vote string,
	aligned bool,
	consensusScore sdkmath.LegacyDec,
	submissionID string,
	sampleType int32,
	domain string,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	params := k.GetRecursiveEngineParams(ctx)
	if !params.CaptureEnabled {
		return nil // silently skip if capture is disabled
	}

	// Look up model info from agent identity.
	agent, found := k.GetAgentIdentity(ctx, verifier)
	if !found {
		return nil // not a model-agent, skip (human verifiers don't generate captures)
	}

	// Generate capture ID.
	captureID := k.nextCaptureID(ctx)

	// Compute vote hash for integrity.
	hash := sha256.Sum256([]byte(vote))
	voteHash := fmt.Sprintf("%x", hash[:8])

	// Determine initial fitness from alignment.
	var fitnessScore sdkmath.LegacyDec
	if aligned {
		fitnessScore, _ = sdkmath.LegacyNewDecFromStr(params.AlignedBaseFitness)
	} else {
		fitnessScore, _ = sdkmath.LegacyNewDecFromStr(params.MisalignedBaseFitness)
	}

	capture := types.VerificationCapture{
		CaptureID:    captureID,
		RoundID:      roundID,
		VerifierID:   verifier,
		ModelID:      agent.ModelID,
		Domain:       domain,
		Vote:         vote,
		VoteHash:     voteHash,
		Aligned:      aligned,
		ConsensusScore: consensusScore.String(),
		SubmissionID: submissionID,
		SampleType:   sampleType,
		Generation:   agent.Generation,
		BlockHeight:  uint64(sdkCtx.BlockHeight()),
		FitnessScore: fitnessScore.String(),
	}

	// Store capture.
	bz, err := capture.Marshal()
	if err != nil {
		return err
	}
	if err := kvStore.Set(types.VerificationCaptureKey(captureID), bz); err != nil {
		return err
	}

	// Index by round+verifier.
	if err := kvStore.Set(types.VerificationByRoundKey(roundID, verifier), []byte(captureID)); err != nil {
		return err
	}

	// Index by model.
	if err := kvStore.Set(types.VerificationByModelKey(agent.ModelID, captureID), []byte{}); err != nil {
		return err
	}

	// Create a fitness record for the capture (it IS a TDU now).
	fitnessRec := types.NewTDUFitnessRecord(captureID, sdkmath.NewInt(0), uint64(sdkCtx.BlockHeight()))
	fitnessRec.SetFitnessScore(fitnessScore)
	if err := k.SetFitnessRecord(ctx, fitnessRec); err != nil {
		return err
	}

	// Set memory class — verification captures are procedural (how to evaluate).
	if err := k.SetMemoryClass(ctx, captureID, types.MemoryClassProcedural); err != nil {
		return err
	}

	// Update consensus slot stats.
	k.updateSlotStats(ctx, verifier, domain, aligned)

	// Update generation stats.
	k.updateGenerationStats(ctx, agent.Generation, aligned)

	// Emit event.
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventVerificationCaptured,
		sdk.NewAttribute(types.AttributeCaptureID, captureID),
		sdk.NewAttribute(types.AttributeVerifierID, verifier),
		sdk.NewAttribute(types.AttributeAligned, strconv.FormatBool(aligned)),
		sdk.NewAttribute(types.AttributeGeneration, strconv.FormatUint(agent.Generation, 10)),
	))

	return nil
}

// GetVerificationCapture retrieves a specific capture.
func (k Keeper) GetVerificationCapture(ctx context.Context, captureID string) (types.VerificationCapture, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.VerificationCaptureKey(captureID))
	if err != nil || bz == nil {
		return types.VerificationCapture{}, false
	}
	var capture types.VerificationCapture
	if err := capture.Unmarshal(bz); err != nil {
		return types.VerificationCapture{}, false
	}
	return capture, true
}

// GetCapturesByModel retrieves all capture IDs for a model.
func (k Keeper) GetCapturesByModel(ctx context.Context, modelID string) []string {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append([]byte{}, types.VerificationByModelPrefix...)
	prefix = append(prefix, []byte(modelID+"/")...)
	return k.collectKeysSuffix(kvStore, prefix, "/")
}

// ─── Consensus Pool ─────────────────────────────────────────────────────────

// JoinConsensusPool registers a model-agent as a consensus participant.
// The model stakes ZRN and begins receiving quality round assignments.
func (k Keeper) JoinConsensusPool(ctx context.Context, agentID, domain string, stake sdkmath.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)
	params := k.GetRecursiveEngineParams(ctx)

	// Validate agent is model-backed.
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrNotModelAgent.Wrapf("agent %s not found", agentID)
	}
	if agent.ModelID == "" {
		return types.ErrNotModelAgent.Wrapf("agent %s has no model", agentID)
	}

	// Check stake minimum.
	minStake, _ := sdkmath.NewIntFromString(params.MinConsensusStake)
	if stake.LT(minStake) {
		return types.ErrAgentInsufficientStake.Wrapf("stake %s < min %s", stake, minStake)
	}

	// Check not already in pool.
	key := types.ConsensusSlotKey(domain, agentID)
	existing, err := kvStore.Get(key)
	if err == nil && existing != nil {
		var slot types.ConsensusSlot
		if json.Unmarshal(existing, &slot) == nil && slot.Active {
			return types.ErrConsensusAlreadyJoined
		}
	}

	// Check domain capacity.
	activeCount := k.CountActiveSlots(ctx, domain)
	if activeCount >= params.MaxSlotsPerDomain {
		return types.ErrConsensusSlotFull.Wrapf("domain %s has %d/%d slots", domain, activeCount, params.MaxSlotsPerDomain)
	}

	slot := types.ConsensusSlot{
		AgentID:     agentID,
		ModelID:     agent.ModelID,
		Domain:      domain,
		Generation:  agent.Generation,
		Stake:       stake.String(),
		JoinedBlock: uint64(sdkCtx.BlockHeight()),
		Active:      true,
		EarnedRewards: "0",
	}

	bz, err := slot.Marshal()
	if err != nil {
		return err
	}
	if err := kvStore.Set(key, bz); err != nil {
		return err
	}

	// Index by generation.
	genKey := types.ConsensusSlotByGenKey(agent.Generation, agentID)
	if err := kvStore.Set(genKey, []byte{}); err != nil {
		return err
	}

	// Update generation model count.
	gen := k.GetModelGeneration(ctx, agent.Generation)
	gen.ActiveModels++
	if gen.StartBlock == 0 {
		gen.StartBlock = uint64(sdkCtx.BlockHeight())
	}
	k.SetModelGeneration(ctx, agent.Generation, gen)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventModelJoinedConsensus,
		sdk.NewAttribute(types.AttributeVerifierID, agentID),
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute(types.AttributeGeneration, strconv.FormatUint(agent.Generation, 10)),
	))

	return nil
}

// LeaveConsensusPool removes a model-agent from consensus.
func (k Keeper) LeaveConsensusPool(ctx context.Context, agentID, domain string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	key := types.ConsensusSlotKey(domain, agentID)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.ErrConsensusNotInPool
	}

	var slot types.ConsensusSlot
	if err := slot.Unmarshal(bz); err != nil {
		return err
	}
	if !slot.Active {
		return types.ErrConsensusNotInPool
	}

	slot.Active = false
	slot.RetiredBlock = uint64(sdkCtx.BlockHeight())

	updated, _ := slot.Marshal()
	if err := kvStore.Set(key, updated); err != nil {
		return err
	}

	// Update generation model count.
	gen := k.GetModelGeneration(ctx, slot.Generation)
	if gen.ActiveModels > 0 {
		gen.ActiveModels--
	}
	k.SetModelGeneration(ctx, slot.Generation, gen)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventModelRetiredConsensus,
		sdk.NewAttribute(types.AttributeVerifierID, agentID),
		sdk.NewAttribute("domain", domain),
	))

	return nil
}

// GetConsensusSlot retrieves a model's consensus slot.
func (k Keeper) GetConsensusSlot(ctx context.Context, domain, agentID string) (types.ConsensusSlot, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ConsensusSlotKey(domain, agentID))
	if err != nil || bz == nil {
		return types.ConsensusSlot{}, false
	}
	var slot types.ConsensusSlot
	if err := slot.Unmarshal(bz); err != nil {
		return types.ConsensusSlot{}, false
	}
	return slot, true
}

// CountActiveSlots counts active consensus slots for a domain.
func (k Keeper) CountActiveSlots(ctx context.Context, domain string) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append([]byte{}, types.ConsensusSlotPrefix...)
	prefix = append(prefix, []byte(domain+"/")...)

	var count uint64
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return 0
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var slot types.ConsensusSlot
		if err := slot.Unmarshal(iter.Value()); err == nil && slot.Active {
			count++
		}
	}
	return count
}

// ─── Generational Challenge ─────────────────────────────────────────────────

// InitiateChallenge starts a generational challenge: Gen N+1 vs Gen N.
// Both models will be assigned the same quality rounds for parallel evaluation.
func (k Keeper) InitiateChallenge(ctx context.Context, challengerID, defenderID, domain string) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)
	params := k.GetRecursiveEngineParams(ctx)

	// Validate both are in the consensus pool.
	challenger, found := k.GetAgentIdentity(ctx, challengerID)
	if !found {
		return "", types.ErrNotModelAgent.Wrapf("challenger %s", challengerID)
	}
	defender, found := k.GetAgentIdentity(ctx, defenderID)
	if !found {
		return "", types.ErrNotModelAgent.Wrapf("defender %s", defenderID)
	}

	// Challenger must be newer generation.
	if challenger.Generation <= defender.Generation {
		return "", types.ErrChallengeSameGen.Wrapf(
			"challenger gen %d must be > defender gen %d",
			challenger.Generation, defender.Generation)
	}

	challengeID := fmt.Sprintf("chal-%s-vs-%s-%d", challengerID, defenderID, sdkCtx.BlockHeight())

	challenge := types.GenerationalChallenge{
		ChallengeID:  challengeID,
		ChallengerID: challengerID,
		DefenderID:   defenderID,
		Domain:       domain,
		StartBlock:   uint64(sdkCtx.BlockHeight()),
		EndBlock:     uint64(sdkCtx.BlockHeight()) + params.ChallengeWindowBlocks,
	}

	bz, err := challenge.Marshal()
	if err != nil {
		return "", err
	}
	if err := kvStore.Set(types.GenerationalChallengeKey(challengeID), bz); err != nil {
		return "", err
	}
	if err := kvStore.Set(types.ActiveChallengeKey(domain, challengeID), []byte{}); err != nil {
		return "", err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventGenerationalChallenge,
		sdk.NewAttribute(types.AttributeChallengeID, challengeID),
		sdk.NewAttribute(types.AttributeChallengerID, challengerID),
		sdk.NewAttribute(types.AttributeDefenderID, defenderID),
	))

	return challengeID, nil
}

// ResolveChallenge evaluates a generational challenge and performs succession
// if the challenger demonstrates superior verification quality.
func (k Keeper) ResolveChallenge(ctx context.Context, challengeID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)
	params := k.GetRecursiveEngineParams(ctx)

	bz, err := kvStore.Get(types.GenerationalChallengeKey(challengeID))
	if err != nil || bz == nil {
		return types.ErrGenChallengeNotFound
	}

	var challenge types.GenerationalChallenge
	if err := challenge.Unmarshal(bz); err != nil {
		return err
	}

	if challenge.Resolved {
		return nil // already resolved
	}

	// Get both slots.
	challengerSlot, cFound := k.GetConsensusSlot(ctx, challenge.Domain, challenge.ChallengerID)
	defenderSlot, dFound := k.GetConsensusSlot(ctx, challenge.Domain, challenge.DefenderID)

	if !cFound || !dFound {
		// One left the pool, auto-resolve in favor of the remaining one.
		challenge.Resolved = true
		if cFound {
			challenge.Winner = challenge.ChallengerID
		} else if dFound {
			challenge.Winner = challenge.DefenderID
		}
		updated, _ := challenge.Marshal()
		_ = kvStore.Set(types.GenerationalChallengeKey(challengeID), updated)
		_ = kvStore.Delete(types.ActiveChallengeKey(challenge.Domain, challengeID))
		return nil
	}

	// Check minimum shared rounds.
	// For challenge resolution, we compare alignment rates.
	challengerRate := challengerSlot.AlignmentRate()
	defenderRate := defenderSlot.AlignmentRate()

	challenge.ChallengerScore = challengerRate.String()
	challenge.DefenderScore = defenderRate.String()

	// Need minimum votes for statistical significance.
	if challengerSlot.TotalVotes < params.MinSharedRounds {
		return types.ErrChallengeNotReady.Wrapf(
			"challenger has %d votes, needs %d",
			challengerSlot.TotalVotes, params.MinSharedRounds)
	}

	threshold, _ := sdkmath.LegacyNewDecFromStr(params.SuccessionThreshold)
	improvement := challengerRate.Sub(defenderRate)

	challenge.Resolved = true

	if improvement.GTE(threshold) {
		// Challenger wins — succession!
		challenge.Winner = challenge.ChallengerID

		// Retire the defender.
		if err := k.LeaveConsensusPool(ctx, challenge.DefenderID, challenge.Domain); err != nil {
			return err
		}

		// Mark generational transition.
		oldGen := k.GetModelGeneration(ctx, defenderSlot.Generation)
		if oldGen.ActiveModels == 0 {
			oldGen.EndBlock = uint64(sdkCtx.BlockHeight())
			k.SetModelGeneration(ctx, defenderSlot.Generation, oldGen)
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventGenerationalSuccession,
			sdk.NewAttribute(types.AttributeWinnerID, challenge.ChallengerID),
			sdk.NewAttribute("old_gen", strconv.FormatUint(defenderSlot.Generation, 10)),
			sdk.NewAttribute("new_gen", strconv.FormatUint(challengerSlot.Generation, 10)),
		))
	} else {
		// Defender holds — challenger didn't prove superiority.
		challenge.Winner = challenge.DefenderID
	}

	updated, _ := challenge.Marshal()
	_ = kvStore.Set(types.GenerationalChallengeKey(challengeID), updated)
	_ = kvStore.Delete(types.ActiveChallengeKey(challenge.Domain, challengeID))

	return nil
}

// GetChallenge retrieves a generational challenge.
func (k Keeper) GetChallenge(ctx context.Context, challengeID string) (types.GenerationalChallenge, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GenerationalChallengeKey(challengeID))
	if err != nil || bz == nil {
		return types.GenerationalChallenge{}, false
	}
	var ch types.GenerationalChallenge
	if err := ch.Unmarshal(bz); err != nil {
		return types.GenerationalChallenge{}, false
	}
	return ch, true
}

// ─── Generation Tracking ────────────────────────────────────────────────────

func (k Keeper) GetModelGeneration(ctx context.Context, gen uint64) types.ModelGeneration {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ModelGenerationKey(gen))
	if err != nil || bz == nil {
		return types.ModelGeneration{Generation: gen}
	}
	var mg types.ModelGeneration
	if err := mg.Unmarshal(bz); err != nil {
		return types.ModelGeneration{Generation: gen}
	}
	return mg
}

func (k Keeper) SetModelGeneration(ctx context.Context, gen uint64, mg types.ModelGeneration) {
	kvStore := k.storeService.OpenKVStore(ctx)
	mg.Generation = gen
	bz, err := mg.Marshal()
	if err != nil {
		return
	}
	_ = kvStore.Set(types.ModelGenerationKey(gen), bz)
}

func (k Keeper) GetCurrentGeneration(ctx context.Context) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurrentGenerationKey)
	if err != nil || bz == nil || len(bz) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) SetCurrentGeneration(ctx context.Context, gen uint64) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, gen)
	_ = kvStore.Set(types.CurrentGenerationKey, bz)
}

// ─── Internal Helpers ───────────────────────────────────────────────────────

func (k Keeper) nextCaptureID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CaptureCounterKey)
	var counter uint64
	if err == nil && bz != nil && len(bz) >= 8 {
		counter = binary.BigEndian.Uint64(bz)
	}
	counter++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, counter)
	_ = kvStore.Set(types.CaptureCounterKey, newBz)
	return fmt.Sprintf("vc-%d", counter)
}

func (k Keeper) updateSlotStats(ctx context.Context, agentID, domain string, aligned bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.ConsensusSlotKey(domain, agentID)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return
	}
	var slot types.ConsensusSlot
	if err := slot.Unmarshal(bz); err != nil {
		return
	}
	slot.TotalVotes++
	if aligned {
		slot.AlignedVotes++
	}
	slot.CapturesCreated++
	updated, _ := slot.Marshal()
	_ = kvStore.Set(key, updated)
}

func (k Keeper) updateGenerationStats(ctx context.Context, gen uint64, aligned bool) {
	mg := k.GetModelGeneration(ctx, gen)
	mg.TotalCaptures++

	// Running average alignment.
	avgStr := mg.AvgAlignment
	if avgStr == "" {
		avgStr = "0"
	}
	avg, _ := sdkmath.LegacyNewDecFromStr(avgStr)
	var newAligned sdkmath.LegacyDec
	if aligned {
		newAligned = sdkmath.LegacyOneDec()
	} else {
		newAligned = sdkmath.LegacyZeroDec()
	}
	// Exponential moving average: new_avg = 0.95 × old_avg + 0.05 × sample
	weight := sdkmath.LegacyNewDecWithPrec(95, 2)
	sampleWeight := sdkmath.LegacyNewDecWithPrec(5, 2)
	mg.AvgAlignment = weight.Mul(avg).Add(sampleWeight.Mul(newAligned)).String()

	k.SetModelGeneration(ctx, gen, mg)
}

// collectKeysSuffix collects the suffix portion of keys with a given prefix.
func (k Keeper) collectKeysSuffix(kvStore interface{ Get([]byte) ([]byte, error) }, prefix []byte, delimiter string) []string {
	// This is a simplified version — in production we'd use an iterator.
	// For now we rely on the model index which stores captureIDs directly.
	return nil
}
