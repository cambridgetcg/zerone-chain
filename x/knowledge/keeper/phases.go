package keeper

import (
	"context"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// BeginBlocker processes active quality rounds, transitioning phases based on block deadlines.
// Also triggers shard reshuffling and attestation checks at SnapshotInterval boundaries.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	block := uint64(sdkCtx.BlockHeight())
	params, _ := k.GetParams(ctx)

	// ── Quality round phase transitions ──────────────────────────────────
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
				minValidators := uint64(3)
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

	// ── Shard lifecycle ──────────────────────────────────────────────────
	k.processShardingLifecycle(ctx, block)

	return nil
}

// processShardingLifecycle handles shard reshuffling and attestation checks at SnapshotInterval.
func (k Keeper) processShardingLifecycle(ctx context.Context, block uint64) {
	shardParams := k.GetShardingParams(ctx)
	if shardParams.SnapshotInterval == 0 || block == 0 {
		return
	}

	if block%shardParams.SnapshotInterval != 0 {
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshotHeight := int64(block)

	// Check attestations from the PREVIOUS snapshot cycle before reshuffling.
	// Grace period = 2× SnapshotInterval. We check the snapshot from 2 intervals ago.
	prevSnapshotHeight := snapshotHeight - int64(shardParams.SnapshotInterval)
	graceCutoffHeight := snapshotHeight - 2*int64(shardParams.SnapshotInterval)
	if graceCutoffHeight > 0 && prevSnapshotHeight > 0 {
		k.checkMissingAttestations(ctx, prevSnapshotHeight, graceCutoffHeight)
	}

	// Get active validators
	validators := k.GetActiveValidatorAddresses(ctx)
	if uint32(len(validators)) < shardParams.MinValidators {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventShardReshuffleSkipped,
			sdk.NewAttribute(types.AttributeSnapshotHeight, strconv.FormatInt(snapshotHeight, 10)),
			sdk.NewAttribute(types.AttributeValidatorCount, strconv.Itoa(len(validators))),
			sdk.NewAttribute(types.AttributeReason, "insufficient validators"),
		))
		return
	}

	// Get all active TDU hashes (fitness >= 0.1, not Pruned)
	tduHashes := k.GetActiveTDUHashes(ctx)

	// Get block hash for deterministic seeding
	blockHash := sdkCtx.BlockHeader().LastBlockId.Hash
	if len(blockHash) == 0 {
		// Fallback: use block height as seed (shouldn't happen in production)
		blockHash = make([]byte, 8)
		for i := 0; i < 8; i++ {
			blockHash[i] = byte(block >> (56 - uint(i)*8))
		}
	}

	// Apply new shard assignments
	if err := k.ApplyShardAssignments(ctx, blockHash, snapshotHeight, tduHashes, validators); err != nil {
		sdkCtx.Logger().Error("failed to apply shard assignments",
			"snapshot_height", snapshotHeight, "error", err)
		return
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventShardReshuffle,
		sdk.NewAttribute(types.AttributeSnapshotHeight, strconv.FormatInt(snapshotHeight, 10)),
		sdk.NewAttribute(types.AttributeValidatorCount, strconv.Itoa(len(validators))),
		sdk.NewAttribute(types.AttributeTDUCount, strconv.Itoa(len(tduHashes))),
	))
}

// checkMissingAttestations emits slash events for validators who have shard assignments
// at prevSnapshotHeight but no attestation submitted by graceCutoffHeight.
func (k Keeper) checkMissingAttestations(ctx context.Context, prevSnapshotHeight, graceCutoffHeight int64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Collect validators with assignments at prevSnapshotHeight
	k.IterateShardAssignments(ctx, func(assignment types.ShardAssignment) bool {
		if assignment.SnapshotHeight != prevSnapshotHeight {
			return false
		}

		// Check if attestation exists
		_, hasAttestation := k.GetStorageAttestation(ctx, assignment.ValidatorAddr, prevSnapshotHeight)
		if hasAttestation {
			return false
		}

		// Missing attestation — emit event (governance/slashing integration)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventMissingStorageAttestation,
			sdk.NewAttribute(types.AttributeValidatorAddr, assignment.ValidatorAddr),
			sdk.NewAttribute(types.AttributeSnapshotHeight, strconv.FormatInt(prevSnapshotHeight, 10)),
		))
		return false
	})
}

// expireRound marks a round as expired, removes from active index, and returns stake to submitter.
func (k Keeper) expireRound(ctx context.Context, round *types.QualityRound) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_EXPIRED
	_ = k.SetQualityRound(ctx, round)
	_ = k.DeleteActiveRound(ctx, round.Id)

	// Return stake to submitter and reset submission status
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	if found {
		if sub.Submitter != "" && sub.Stake != "" {
			submitterAddr, addrErr := sdk.AccAddressFromBech32(sub.Submitter)
			if addrErr == nil {
				stakeAmt, ok := sdkmath.NewIntFromString(sub.Stake)
				if ok && stakeAmt.IsPositive() {
					stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
					if err := k.bankKeeper.SendCoinsFromModuleToAccount(sdkCtx, types.ModuleName, submitterAddr, sdk.NewCoins(stakeCoin)); err != nil {
						sdkCtx.Logger().Error("failed to return stake on round expiry",
							"round_id", round.Id, "submitter", sub.Submitter, "error", err)
					}
				}
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

// EndBlocker processes epoch boundaries, patronage expiry, and bounty expiry.
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
		k.distributeEpochRevenue(ctx, params)
		k.expireBounties(ctx, blockHeight)
	}

	// 2. Fitness epoch processing (independent of ecology epoch)
	fitnessParams := k.GetFitnessDecayParams(ctx)
	fitnessEpoch := fitnessParams.GetFitnessEpochBlocks()
	if fitnessEpoch > 0 && blockHeight > 0 && blockHeight%fitnessEpoch == 0 {
		currentCycle := blockHeight / fitnessEpoch
		k.DecayUnscored(ctx, currentCycle)
		k.DistributeLongevityRewards(ctx)
		k.PruneFitnessBelowThreshold(ctx)
	}

	// 3. Expire patronage (every block)
	k.expirePatronage(ctx, blockHeight)

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
