package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Reconsolidation Engine (R51) ───────────────────────────────────────────
//
// When a memory is retrieved and produces a prediction error, it enters a
// labile state. This is how the brain keeps memories accurate.
//
// In ToK: training retrieval → negative model outcome → reconsolidation
// window opens → corrections facilitated → data re-stabilizes.
//
// The chain literally learns from its mistakes.

// ─── TriggerReconsolidation ─────────────────────────────────────────────────

// TriggerReconsolidation opens a reconsolidation window for a TDU after a
// negative training outcome. The TDU becomes temporarily labile — corrections
// are facilitated, and the data can be updated.
func (k Keeper) TriggerReconsolidation(ctx context.Context, sampleID string, modelDelta sdkmath.LegacyDec, currentCycle uint64) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	params := k.GetReconsolidationParams(ctx)
	if !params.Enabled {
		return "", types.ErrReconsolidationDisabled
	}

	// Check if already has an open window.
	history := k.getOrCreateHistory(ctx, sampleID)
	if history.HasActiveWindow() {
		return "", types.ErrReconsolidationAlreadyOpen.Wrapf("TDU %s", sampleID)
	}

	// Get memory tier for window duration calculation.
	tier := types.MemoryTierWorking
	activationRec, found := k.GetActivationRecord(ctx, sampleID)
	if found {
		tier = types.MemoryTier(activationRec.MemoryTier)
	}

	// Canonical data requires multiple negative outcomes before reconsolidation.
	if tier == types.MemoryTierCanonical {
		// Count recent negative outcomes from activation record.
		if found && activationRec.NegativeOutcomes < uint64(params.CanonicalMinNegativeOutcomes) {
			return "", types.ErrCanonicalNotEnoughNegatives.Wrapf(
				"need %d negative outcomes, have %d",
				params.CanonicalMinNegativeOutcomes, activationRec.NegativeOutcomes,
			)
		}
	}

	// Get current fitness.
	fitnessRec, fitnessFound := k.GetFitnessRecord(ctx, sampleID)
	originalFitness := sdkmath.LegacyNewDecWithPrec(5, 1) // default 0.5
	if fitnessFound {
		originalFitness = fitnessRec.GetFitnessScore()
	}

	// Calculate window duration based on tier.
	windowDuration := types.WindowDurationForTier(tier)
	blockHeight := sdkCtx.BlockHeight()

	// Create window.
	windowID := k.nextWindowID(ctx)
	window := &types.ReconsolidationWindow{
		WindowID:        windowID,
		SampleID:        sampleID,
		TriggeredAt:     blockHeight,
		ExpiresAt:       blockHeight + windowDuration,
		TriggerCycle:    currentCycle,
		ModelDelta:      modelDelta.String(),
		OriginalFitness: originalFitness.String(),
		MemoryTierAtOpen: int(tier),
		Status:          types.ReconsolidationOpen,
	}

	if err := k.setWindow(ctx, window); err != nil {
		return "", err
	}

	// Update history.
	history.TotalWindows++
	history.LastWindowAt = blockHeight
	history.ActiveWindowID = windowID
	if err := k.setHistory(ctx, history); err != nil {
		return "", err
	}

	// Index: open windows for BeginBlocker expiration check.
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.OpenWindowKey(windowID), []byte(sampleID))
	_ = kvStore.Set(types.ReconsolidationBySampleKey(sampleID, windowID), []byte{0x01})

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventReconsolidationOpened,
		sdk.NewAttribute(types.AttributeWindowID, windowID),
		sdk.NewAttribute(types.AttributeSampleID, sampleID),
		sdk.NewAttribute(types.AttributeModelDelta, modelDelta.String()),
		sdk.NewAttribute(types.AttributeMemoryTier, tier.String()),
		sdk.NewAttribute(types.AttributeExpiresAt, fmt.Sprintf("%d", window.ExpiresAt)),
	))

	return windowID, nil
}

// ─── SubmitCorrection ───────────────────────────────────────────────────────

// SubmitCorrection records a correction TDU submitted during a reconsolidation
// window. The correction gets expedited review and the original TDU's fitness
// is adjusted.
func (k Keeper) SubmitCorrection(ctx context.Context, windowID, correctionTDUID, correctorAddress string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	window, found := k.GetWindow(ctx, windowID)
	if !found {
		return types.ErrReconsolidationWindowNotFound.Wrapf("window %s", windowID)
	}
	if window.Status != types.ReconsolidationOpen {
		return types.ErrReconsolidationWindowClosed.Wrapf("window %s status: %s", windowID, window.Status)
	}
	if sdkCtx.BlockHeight() > window.ExpiresAt {
		return types.ErrReconsolidationWindowClosed.Wrap("window has expired")
	}

	// Add correction to window.
	window.CorrectionIDs = append(window.CorrectionIDs, correctionTDUID)
	if err := k.setWindow(ctx, window); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventCorrectionSubmitted,
		sdk.NewAttribute(types.AttributeWindowID, windowID),
		sdk.NewAttribute(types.AttributeCorrectionID, correctionTDUID),
		sdk.NewAttribute(types.AttributeSampleID, window.SampleID),
	))

	return nil
}

// ─── ResolveWindow ──────────────────────────────────────────────────────────

// ResolveWindow closes a reconsolidation window after a correction is accepted.
// The correction inherits partial activation history from the original TDU.
func (k Keeper) ResolveWindow(ctx context.Context, windowID, acceptedCorrectionID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	window, found := k.GetWindow(ctx, windowID)
	if !found {
		return types.ErrReconsolidationWindowNotFound.Wrapf("window %s", windowID)
	}
	if window.Status != types.ReconsolidationOpen {
		return types.ErrReconsolidationWindowClosed.Wrapf("already %s", window.Status)
	}

	params := k.GetReconsolidationParams(ctx)

	// Reduce original TDU's fitness.
	fitnessRec, fitnessFound := k.GetFitnessRecord(ctx, window.SampleID)
	if fitnessFound {
		score := fitnessRec.GetFitnessScore()
		// Drop fitness proportional to model degradation (capped at -0.3).
		modelDelta, _ := sdkmath.LegacyNewDecFromStr(window.ModelDelta)
		drop := modelDelta.Abs()
		maxDrop := sdkmath.LegacyNewDecWithPrec(3, 1) // 0.3 cap
		if drop.GT(maxDrop) {
			drop = maxDrop
		}
		fitnessRec.SetFitnessScore(score.Sub(drop))
		_ = k.SetFitnessRecord(ctx, fitnessRec)
		window.FitnessAfter = fitnessRec.GetFitnessScore().String()
	}

	// Inherit activation history to correction TDU.
	originalActivation, hasActivation := k.GetActivationRecord(ctx, window.SampleID)
	if hasActivation {
		inheritRatio, _ := sdkmath.LegacyNewDecFromStr(params.ActivationInheritanceRatio)

		inheritedActivations := inheritRatio.MulInt64(int64(originalActivation.TotalActivations)).TruncateInt64()
		inheritedCycles := inheritRatio.MulInt64(int64(originalActivation.UniqueCycles)).TruncateInt64()
		inheritedPositive := inheritRatio.MulInt64(int64(originalActivation.PositiveOutcomes)).TruncateInt64()

		correctionActivation := &types.ActivationRecord{
			SampleID:         acceptedCorrectionID,
			TotalActivations: uint64(inheritedActivations),
			UniqueCycles:     uint64(inheritedCycles),
			FirstActivation:  originalActivation.FirstActivation,
			LastActivation:   originalActivation.LastActivation,
			PositiveOutcomes: uint64(inheritedPositive),
			MemoryTier:       int(types.MemoryTierActive), // corrections start Active
		}
		_ = k.SetActivationRecord(ctx, correctionActivation)
	}

	// Create knowledge graph edge: correction → corrects → original.
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  "protocol",
		SourceID: acceptedCorrectionID,
		TargetID: window.SampleID,
		EdgeType: types.EdgeTypeCorrects,
	})

	// Update window.
	window.Status = types.ReconsolidationResolved
	window.ResolvedAt = sdkCtx.BlockHeight()
	if err := k.setWindow(ctx, window); err != nil {
		return err
	}

	// Update history.
	history := k.getOrCreateHistory(ctx, window.SampleID)
	history.CorrectedCount++
	history.ActiveWindowID = ""
	history.CorrectionChain = append(history.CorrectionChain, acceptedCorrectionID)
	_ = k.setHistory(ctx, history)

	// Remove from open windows index.
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.OpenWindowKey(windowID))

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventReconsolidationResolved,
		sdk.NewAttribute(types.AttributeWindowID, windowID),
		sdk.NewAttribute(types.AttributeSampleID, window.SampleID),
		sdk.NewAttribute(types.AttributeCorrectionID, acceptedCorrectionID),
	))

	return nil
}

// ─── ExpireWindows ──────────────────────────────────────────────────────────

// ExpireWindows closes all reconsolidation windows that have exceeded their
// duration. Uncorrected windows apply a fitness penalty — data that keeps
// producing errors without correction gradually fades.
// Called from BeginBlocker.
func (k Keeper) ExpireWindows(ctx context.Context) (expired uint64, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := sdkCtx.BlockHeight()
	params := k.GetReconsolidationParams(ctx)

	expirationPenalty, _ := sdkmath.LegacyNewDecFromStr(params.ExpirationPenalty)

	kvStore := k.storeService.OpenKVStore(ctx)
	iter, iterErr := kvStore.Iterator(types.OpenWindowPrefix, prefixEndBytes(types.OpenWindowPrefix))
	if iterErr != nil {
		return 0, iterErr
	}

	// Collect windows to expire (can't modify store during iteration).
	type expireEntry struct {
		windowID string
		sampleID string
	}
	var toExpire []expireEntry

	for ; iter.Valid(); iter.Next() {
		windowID := string(iter.Key()[len(types.OpenWindowPrefix):])
		sampleID := string(iter.Value())

		window, found := k.GetWindow(ctx, windowID)
		if !found {
			toExpire = append(toExpire, expireEntry{windowID, sampleID})
			continue
		}
		if blockHeight > window.ExpiresAt {
			toExpire = append(toExpire, expireEntry{windowID, sampleID})
		}
	}
	iter.Close()

	// Process expirations.
	for _, entry := range toExpire {
		window, found := k.GetWindow(ctx, entry.windowID)
		if found && window.Status == types.ReconsolidationOpen {
			// Apply expiration penalty to fitness.
			fitnessRec, fitnessFound := k.GetFitnessRecord(ctx, entry.sampleID)
			if fitnessFound {
				score := fitnessRec.GetFitnessScore()
				fitnessRec.SetFitnessScore(score.Sub(expirationPenalty))
				_ = k.SetFitnessRecord(ctx, fitnessRec)
				window.FitnessAfter = fitnessRec.GetFitnessScore().String()
			}

			window.Status = types.ReconsolidationExpired
			window.ResolvedAt = blockHeight
			_ = k.setWindow(ctx, window)

			// Update history.
			history := k.getOrCreateHistory(ctx, entry.sampleID)
			history.UncorrectedCount++
			history.ActiveWindowID = ""
			_ = k.setHistory(ctx, history)

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				types.EventReconsolidationExpired,
				sdk.NewAttribute(types.AttributeWindowID, entry.windowID),
				sdk.NewAttribute(types.AttributeSampleID, entry.sampleID),
				sdk.NewAttribute(types.AttributeUncorrectedCount, fmt.Sprintf("%d", history.UncorrectedCount)),
			))

			expired++
		}

		// Remove from open windows index.
		_ = kvStore.Delete(types.OpenWindowKey(entry.windowID))
	}

	return expired, nil
}

// ─── GetEffectiveDecayRate ──────────────────────────────────────────────────

// GetEffectiveDecayRate computes the full effective decay rate for a TDU,
// combining the base decay, memory tier modifier (R50), and reconsolidation
// penalty (R51).
//
// effective_decay = base_decay × tier_modifier × reconsolidation_penalty
func (k Keeper) GetEffectiveDecayRate(ctx context.Context, sampleID string, baseDecay sdkmath.LegacyDec) sdkmath.LegacyDec {
	// Apply tier modifier (R50).
	tierModified := k.GetDecayModifiedRate(ctx, sampleID, baseDecay)

	// Apply reconsolidation penalty (R51).
	history := k.getOrCreateHistory(ctx, sampleID)
	penalty := history.GetReconsolidationPenalty()

	return tierModified.Mul(penalty)
}

// ─── IsInReconsolidation ────────────────────────────────────────────────────

// IsInReconsolidation checks if a TDU currently has an open reconsolidation window.
func (k Keeper) IsInReconsolidation(ctx context.Context, sampleID string) (bool, *types.ReconsolidationWindow) {
	history := k.getOrCreateHistory(ctx, sampleID)
	if !history.HasActiveWindow() {
		return false, nil
	}
	window, found := k.GetWindow(ctx, history.ActiveWindowID)
	if !found {
		return false, nil
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if sdkCtx.BlockHeight() > window.ExpiresAt {
		return false, nil // technically expired but not yet processed
	}
	return true, window
}

// ─── GetReconsolidationHistory ──────────────────────────────────────────────

// GetReconsolidationHistory returns a TDU's full reconsolidation history.
func (k Keeper) GetReconsolidationHistory(ctx context.Context, sampleID string) *types.ReconsolidationHistory {
	return k.getOrCreateHistory(ctx, sampleID)
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetWindow retrieves a reconsolidation window by ID.
func (k Keeper) GetWindow(ctx context.Context, windowID string) (*types.ReconsolidationWindow, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ReconsolidationWindowKey(windowID))
	if err != nil || bz == nil {
		return nil, false
	}
	var window types.ReconsolidationWindow
	if err := json.Unmarshal(bz, &window); err != nil {
		return nil, false
	}
	return &window, true
}

// GetOpenWindowCount returns the number of currently open reconsolidation windows.
func (k Keeper) GetOpenWindowCount(ctx context.Context) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.OpenWindowPrefix, prefixEndBytes(types.OpenWindowPrefix))
	if err != nil {
		return 0
	}
	defer iter.Close()
	var count uint64
	for ; iter.Valid(); iter.Next() {
		count++
	}
	return count
}

// GetReconsolidationParams returns the reconsolidation parameters.
func (k Keeper) GetReconsolidationParams(ctx context.Context) types.ReconsolidationParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ReconsolidationParamsKey)
	if err != nil || bz == nil {
		return types.DefaultReconsolidationParams()
	}
	var params types.ReconsolidationParams
	if err := json.Unmarshal(bz, &params); err != nil {
		return types.DefaultReconsolidationParams()
	}
	return params
}

// SetReconsolidationParams stores the reconsolidation parameters.
func (k Keeper) SetReconsolidationParams(ctx context.Context, params types.ReconsolidationParams) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal reconsolidation params: %w", err)
	}
	return kvStore.Set(types.ReconsolidationParamsKey, bz)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setWindow(ctx context.Context, window *types.ReconsolidationWindow) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(window)
	if err != nil {
		return fmt.Errorf("failed to marshal reconsolidation window: %w", err)
	}
	return kvStore.Set(types.ReconsolidationWindowKey(window.WindowID), bz)
}

func (k Keeper) getOrCreateHistory(ctx context.Context, sampleID string) *types.ReconsolidationHistory {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ReconsolidationHistoryKey(sampleID))
	if err == nil && bz != nil {
		var history types.ReconsolidationHistory
		if json.Unmarshal(bz, &history) == nil {
			return &history
		}
	}
	return &types.ReconsolidationHistory{SampleID: sampleID}
}

// SetReconsolidationHistory stores a TDU's reconsolidation history (exported for testing).
func (k Keeper) SetReconsolidationHistory(ctx context.Context, history *types.ReconsolidationHistory) error {
	return k.setHistory(ctx, history)
}

func (k Keeper) setHistory(ctx context.Context, history *types.ReconsolidationHistory) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal reconsolidation history: %w", err)
	}
	return kvStore.Set(types.ReconsolidationHistoryKey(history.SampleID), bz)
}

func (k Keeper) nextWindowID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ReconsolidationSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	input := fmt.Sprintf("reconsolidation:%d", seq)
	hash := sha256.Sum256([]byte(input))
	id := hex.EncodeToString(hash[:16])
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = kvStore.Set(types.ReconsolidationSeqKey, next)
	return id
}
