package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/qualification/types"
)

// RunAccuracyDecay scans every qualification with sufficient sample
// size and transitions state based on the current AccuracyBps. The
// transitions form a self-correcting feedback loop on top of the
// per-domain panel's RecordVerificationOutcome writes:
//
//	ACTIVE       + accuracy <  decay_probation_bps  → PROBATIONARY
//	PROBATIONARY + accuracy <  decay_suspension_bps → SUSPENDED
//	PROBATIONARY + accuracy >= decay_recovery_bps   → ACTIVE
//
// The chain stops issuing diplomas (one-time grants) and starts
// running an ongoing competency assessment. A voter who consistently
// votes against verified consensus loses weight; one who recovers
// gets it back.
//
// Skipped qualifications:
//   - TotalVerifications < DecayMinSamples (insufficient signal yet)
//   - Status not in {ACTIVE, PROBATIONARY} (already EXPIRED / SUSPENDED /
//     REVOKED — those have their own paths back)
//
// Cost: O(n) on qualifications. Caller (BeginBlocker) gates the scan
// to once per DecayCheckIntervalBlocks so per-block work stays low.
func (k Keeper) RunAccuracyDecay(ctx context.Context, currentBlock uint64, params *types.Params) {
	if params == nil {
		return
	}
	minSamples := params.DecayMinSamples
	if minSamples == 0 {
		return
	}
	probationBps := params.DecayProbationBps
	suspensionBps := params.DecaySuspensionBps
	recoveryBps := params.DecayRecoveryBps
	if probationBps == 0 && suspensionBps == 0 && recoveryBps == 0 {
		return // decay disabled
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	k.IterateQualifications(ctx, func(q *types.DomainQualification) bool {
		if q == nil || q.Metrics == nil {
			return false
		}
		if q.Metrics.TotalVerifications < minSamples {
			return false
		}
		accuracy := q.Metrics.AccuracyBps

		switch q.Status {
		// All three decay transitions below announce commitment 7:
		// skill is current, not historical. Each event marks a
		// validator's qualification weight changing because their
		// recent accuracy crossed a threshold. See TRUTH_SEEKING.md.

		case types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE:
			if probationBps > 0 && accuracy < probationBps {
				q.Status = types.QualificationStatus_QUALIFICATION_STATUS_PROBATIONARY
				// Set probation deadline: standard probation period from
				// the moment of demotion. If the voter doesn't recover
				// within this window, the existing PROBATIONARY → expiry
				// path handles the rest.
				if params.ProbationPeriod > 0 {
					q.ProbationUntil = currentBlock + params.ProbationPeriod
				}
				k.SetQualification(ctx, q)
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"zerone.qualification.decay_probation",
					sdk.NewAttribute("validator", q.Validator),
					sdk.NewAttribute("domain", q.Domain),
					sdk.NewAttribute("accuracy_bps", fmt.Sprintf("%d", accuracy)),
					sdk.NewAttribute("threshold_bps", fmt.Sprintf("%d", probationBps)),
					sdk.NewAttribute("creed_commitment", "7"),
				))
			}

		case types.QualificationStatus_QUALIFICATION_STATUS_PROBATIONARY:
			if suspensionBps > 0 && accuracy < suspensionBps {
				q.Status = types.QualificationStatus_QUALIFICATION_STATUS_SUSPENDED
				q.ProbationUntil = 0
				k.SetQualification(ctx, q)
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"zerone.qualification.decay_suspension",
					sdk.NewAttribute("validator", q.Validator),
					sdk.NewAttribute("domain", q.Domain),
					sdk.NewAttribute("accuracy_bps", fmt.Sprintf("%d", accuracy)),
					sdk.NewAttribute("threshold_bps", fmt.Sprintf("%d", suspensionBps)),
					sdk.NewAttribute("creed_commitment", "7"),
				))
				return false
			}
			if recoveryBps > 0 && accuracy >= recoveryBps {
				q.Status = types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE
				q.ProbationUntil = 0
				k.SetQualification(ctx, q)
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"zerone.qualification.decay_recovered",
					sdk.NewAttribute("validator", q.Validator),
					sdk.NewAttribute("domain", q.Domain),
					sdk.NewAttribute("accuracy_bps", fmt.Sprintf("%d", accuracy)),
					sdk.NewAttribute("threshold_bps", fmt.Sprintf("%d", recoveryBps)),
					sdk.NewAttribute("creed_commitment", "7"),
				))
			}
		}
		return false
	})
}
