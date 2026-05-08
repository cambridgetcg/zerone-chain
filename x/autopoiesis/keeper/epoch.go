package keeper

import (
	"context"
	"fmt"
	"math/big"
	"math/bits"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/autopoiesis/types"
)

// CollectAndAdapt is called from EndBlocker. It checks if an epoch boundary
// has been reached and, if so, collects cross-module metrics, computes SSI,
// and adjusts multipliers toward their targets.
func (k Keeper) CollectAndAdapt(ctx context.Context) {
	if !k.IsActive(ctx) {
		return
	}

	params := k.GetParams(ctx)
	if !params.Enabled {
		return
	}

	// Check emergency halt — skip adjustment during halts.
	if k.emergencyKeeper != nil && k.emergencyKeeper.IsHalted(ctx) {
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	blockHeight := uint64(sdkCtx.BlockHeight())

	state := k.GetState(ctx)
	if state.LastEpochHeight == 0 {
		// First block after activation — set baseline.
		state.LastEpochHeight = blockHeight
		k.SetState(ctx, state)
		return
	}

	// Check epoch boundary.
	if blockHeight-state.LastEpochHeight < params.EpochLengthBlocks {
		return
	}

	// ---- Epoch boundary reached ----
	state.CurrentEpoch++
	state.LastEpochHeight = blockHeight

	// Collect metrics.
	stakingParticipation := k.collectStakingParticipation(ctx)
	verificationRate := k.collectVerificationRate(ctx)
	isHalted := k.emergencyKeeper != nil && k.emergencyKeeper.IsHalted(ctx)

	// Compute raw SSI and apply damping (T8).
	rawSsi := types.ComputeSSI(stakingParticipation, verificationRate, isHalted)
	smoothedSsi := applyEwmaSmoothing(rawSsi, state.SmoothedSsi, params.SsiSmoothingAlphaBps)

	// Oscillation detection: track sign of (raw - lastRaw) over a rolling window.
	oscillating := false
	if state.LastRawSsi != 0 || state.CurrentEpoch > 1 {
		positiveDelta := uint64(0)
		if rawSsi > state.LastRawSsi {
			positiveDelta = 1
		}
		state.DeltaSignBitmap = (state.DeltaSignBitmap << 1) | positiveDelta
		flips := countFlipsInWindow(state.DeltaSignBitmap, params.OscillationWindowEpochs)
		if params.OscillationSignFlipThreshold > 0 && flips >= params.OscillationSignFlipThreshold {
			oscillating = true
			state.OscillationFrozenUntilEpoch = state.CurrentEpoch + params.OscillationFreezeEpochs
			// Commitment 12 (the chain pays for its own audit): the
			// audit-budget regulator protects itself from instability —
			// detecting oscillation triggers a temporary freeze so the
			// budget cannot whipsaw against itself, preserving the
			// chain's ability to fund scrutiny across stress regimes.
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"zerone.autopoiesis.oscillation_detected",
				sdk.NewAttribute("epoch", fmt.Sprintf("%d", state.CurrentEpoch)),
				sdk.NewAttribute("flips", fmt.Sprintf("%d", flips)),
				sdk.NewAttribute("window_epochs", fmt.Sprintf("%d", params.OscillationWindowEpochs)),
				sdk.NewAttribute("frozen_until_epoch", fmt.Sprintf("%d", state.OscillationFrozenUntilEpoch)),
				sdk.NewAttribute("creed_commitment", "12"),
			))
		}
	}
	state.LastRawSsi = rawSsi
	state.SmoothedSsi = smoothedSsi

	category := types.ClassifySSI(smoothedSsi, params)
	k.SetSSI(ctx, smoothedSsi)

	// Determine whether we're in an oscillation freeze window.
	frozenByOscillation := state.CurrentEpoch < state.OscillationFrozenUntilEpoch

	// Adjust each multiplier using the SMOOTHED SSI + dead-zone.
	// First pass: compute desired deltas per multiplier. Second pass: enforce
	// the cross-module change budget (L7) by scaling if the total exceeds it.
	multipliers := k.GetAllMultipliers(ctx)
	type desired struct {
		ms     *types.MultiplierState
		target uint64
		delta  int64 // signed delta after dead-zone + per-multiplier clamp
	}
	pending := make([]desired, 0, len(multipliers))
	var snapshotMultipliers []*types.MultiplierState
	var totalAbsDelta uint64

	for _, ms := range multipliers {
		if k.IsMultiplierFrozen(ctx, ms.Path) || frozenByOscillation {
			ms.Frozen = true
			snapshotMultipliers = append(snapshotMultipliers, ms)
			continue
		}

		target := types.ComputeTarget(smoothedSsi, ms.Path)
		newCurrent := adjustMultiplierWithDeadZone(
			ms.CurrentBps, target,
			params.MaxChangePerEpochBps,
			params.TargetDeadZoneBps,
			ms.MinBps, ms.MaxBps,
		)
		var d int64
		if newCurrent >= ms.CurrentBps {
			d = int64(newCurrent - ms.CurrentBps)
		} else {
			d = -int64(ms.CurrentBps - newCurrent)
		}
		absD := uint64(d)
		if d < 0 {
			absD = uint64(-d)
		}
		totalAbsDelta += absD
		pending = append(pending, desired{ms: ms, target: target, delta: d})
	}

	// If the total absolute delta exceeds the change budget, scale all
	// deltas proportionally (integer division; drops any sub-bps residual).
	budget := params.MaxTotalChangeBpsPerEpoch
	if budget > 0 && totalAbsDelta > budget {
		for i := range pending {
			// scaled = d × budget / totalAbsDelta
			scaled := pending[i].delta * int64(budget) / int64(totalAbsDelta)
			pending[i].delta = scaled
		}
		// Commitment 12 (the chain pays for its own audit): cross-module
		// change-budget conservation. The autonomic regulator cannot
		// dump unbounded delta into the audit budgets in a single epoch
		// — change is rate-limited so the chain's funding posture moves
		// at a pace observers can follow.
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.autopoiesis.change_budget_scaled",
			sdk.NewAttribute("epoch", fmt.Sprintf("%d", state.CurrentEpoch)),
			sdk.NewAttribute("requested_total_bps", fmt.Sprintf("%d", totalAbsDelta)),
			sdk.NewAttribute("budget_bps", fmt.Sprintf("%d", budget)),
			sdk.NewAttribute("creed_commitment", "12"),
		))
	}

	// Apply scaled deltas.
	for _, p := range pending {
		ms := p.ms
		if p.delta >= 0 {
			ms.CurrentBps += uint64(p.delta)
		} else {
			ms.CurrentBps -= uint64(-p.delta)
		}
		if ms.CurrentBps < ms.MinBps {
			ms.CurrentBps = ms.MinBps
		}
		if ms.CurrentBps > ms.MaxBps {
			ms.CurrentBps = ms.MaxBps
		}
		ms.TargetBps = p.target
		ms.LastUpdated = blockHeight
		k.SetMultiplierState(ctx, ms)
		snapshotMultipliers = append(snapshotMultipliers, ms)
	}
	_ = oscillating // flag is observable via the oscillation_detected event

	// Store snapshot.
	snapshot := &types.EpochSnapshot{
		Epoch:       state.CurrentEpoch,
		BlockHeight: blockHeight,
		Multipliers: snapshotMultipliers,
		SsiScore:    smoothedSsi,
		SsiCategory: category,
	}
	k.SetEpochSnapshot(ctx, snapshot)

	k.SetState(ctx, state)

	// Commitment 5 (the chain manufactures probe demand) AND
	// commitment 12 (the chain pays for its own audit): one epoch of
	// autonomic regulation is the heartbeat of both — the SSI signal
	// adjusts multipliers that scale probe-demand and audit-budget
	// allocations. Without this event the chain's self-regulation is
	// invisible to off-chain observers; with it, the audit posture is
	// fully legible epoch by epoch.
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.autopoiesis.epoch_processed",
			sdk.NewAttribute("epoch", fmt.Sprintf("%d", state.CurrentEpoch)),
			sdk.NewAttribute("ssi_score", fmt.Sprintf("%d", smoothedSsi)),
			sdk.NewAttribute("raw_ssi_score", fmt.Sprintf("%d", rawSsi)),
			sdk.NewAttribute("ssi_category", category),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", blockHeight)),
			sdk.NewAttribute("multiplier_count", fmt.Sprintf("%d", len(snapshotMultipliers))),
			sdk.NewAttribute("oscillation_frozen", fmt.Sprintf("%t", frozenByOscillation)),
			sdk.NewAttribute("creed_commitment", "5,12"),
		),
	)

	k.Logger(ctx).Info("epoch processed",
		"epoch", state.CurrentEpoch,
		"raw_ssi", rawSsi,
		"smoothed_ssi", smoothedSsi,
		"category", category,
		"block", blockHeight,
	)
}

// collectStakingParticipation computes staking participation as a BPS value.
// Returns the fraction of total supply that is bonded (simplified: uses
// bonded stake as a proportion of a reference supply).
func (k Keeper) collectStakingParticipation(ctx context.Context) uint64 {
	if k.stakingKeeper == nil {
		return types.BPSScale // assume healthy if no staking data
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	totalBonded := k.stakingKeeper.GetTotalBondedStake(sdkCtx)
	if totalBonded == nil || totalBonded.Sign() == 0 {
		return 0
	}

	activeCount := k.stakingKeeper.GetActiveValidatorCount(sdkCtx)
	if activeCount == 0 {
		return 0
	}

	// Heuristic: participation = min(1.0, activeValidators / 21) * BPSScale
	// This is simplified — in production, use bonded/total supply ratio.
	target := uint64(21) // target validator set size
	participation := uint64(activeCount) * types.BPSScale / target
	if participation > types.BPSScale {
		participation = types.BPSScale
	}
	return participation
}

// collectVerificationRate gets the knowledge verification rate.
func (k Keeper) collectVerificationRate(ctx context.Context) uint64 {
	if k.knowledgeKeeper == nil {
		return types.BPSScale // assume healthy if no knowledge data
	}
	rate := k.knowledgeKeeper.GetVerificationRate(ctx)
	if rate > types.BPSScale {
		rate = types.BPSScale
	}
	return rate
}

// adjustMultiplier moves current toward target by at most maxChange per step,
// clamped to [min, max]. Kept for backward compatibility; new code uses
// adjustMultiplierWithDeadZone (T8 damping).
func adjustMultiplier(current, target, maxChange, min, max uint64) uint64 {
	return adjustMultiplierWithDeadZone(current, target, maxChange, 0, min, max)
}

// adjustMultiplierWithDeadZone is adjustMultiplier with an additional dead-zone:
// if the absolute delta between target and current is <= deadZone, no change.
// Prevents micro-oscillation around the target (T8).
func adjustMultiplierWithDeadZone(current, target, maxChange, deadZone, min, max uint64) uint64 {
	var delta uint64
	var result uint64

	switch {
	case target > current:
		delta = target - current
		if delta <= deadZone {
			result = current // inside dead-zone — no adjustment
		} else {
			if delta > maxChange {
				delta = maxChange
			}
			result = current + delta
		}
	case target < current:
		delta = current - target
		if delta <= deadZone {
			result = current
		} else {
			if delta > maxChange {
				delta = maxChange
			}
			result = current - delta
		}
	default:
		result = current
	}

	if result < min {
		result = min
	}
	if result > max {
		result = max
	}
	return result
}

// applyEwmaSmoothing returns the EWMA of rawValue given the previous smoothed
// value and an alpha coefficient expressed in BPS (T8). When previous == 0 we
// treat it as an uninitialised state and return rawValue to seed the filter.
func applyEwmaSmoothing(rawValue, previousSmoothed, alphaBps uint64) uint64 {
	if previousSmoothed == 0 || alphaBps == 0 {
		return rawValue
	}
	if alphaBps >= types.BPSScale {
		return rawValue
	}
	// smoothed = (alpha*raw + (BPS-alpha)*prev) / BPS
	num := new(big.Int).Add(
		new(big.Int).Mul(new(big.Int).SetUint64(alphaBps), new(big.Int).SetUint64(rawValue)),
		new(big.Int).Mul(new(big.Int).SetUint64(types.BPSScale-alphaBps), new(big.Int).SetUint64(previousSmoothed)),
	)
	result := num.Div(num, new(big.Int).SetUint64(types.BPSScale))
	if !result.IsUint64() {
		return rawValue
	}
	return result.Uint64()
}

// countFlipsInWindow counts the number of sign changes in the last `window`
// bits of the bitmap. Each bit represents the sign of one epoch's delta.
// A flip is a transition between adjacent bits.
func countFlipsInWindow(bitmap, window uint64) uint64 {
	if window == 0 || window > 64 {
		window = 64
	}
	mask := uint64(1)<<window - 1
	if window == 64 {
		mask = ^uint64(0)
	}
	// Windowed bitmap and its 1-shifted neighbour — XOR is 1 at flip positions.
	w := bitmap & mask
	shifted := (bitmap >> 1) & mask
	flips := w ^ shifted
	// We have `window` bits but only `window-1` adjacent pairs; the lowest
	// bit of the shift produces an artifact. Mask to window-1 adjacencies.
	adjacencyMask := (uint64(1) << (window - 1)) - 1
	return uint64(bits.OnesCount64(flips & adjacencyMask))
}

// bigIntToBPS converts a big.Int ratio to BPS (used internally, exported for tests).
func bigIntToBPS(numerator, denominator *big.Int) uint64 {
	if denominator == nil || denominator.Sign() == 0 {
		return 0
	}
	// (numerator * BPSScale) / denominator
	scaled := new(big.Int).Mul(numerator, new(big.Int).SetUint64(types.BPSScale))
	result := new(big.Int).Div(scaled, denominator)
	if !result.IsUint64() {
		return types.BPSScale
	}
	bps := result.Uint64()
	if bps > types.BPSScale {
		bps = types.BPSScale
	}
	return bps
}
