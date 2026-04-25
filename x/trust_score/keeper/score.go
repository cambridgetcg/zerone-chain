package keeper

import (
	"context"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
	"github.com/zerone-chain/zerone/x/trust_score/types"
)

// BPS scale for all percentages.
const bps = uint64(1_000_000)

// neutralCalibrationBps is the fallback when an address has no
// AgentCalibration record yet — neither rewarded nor penalised. Same
// value the panel tally uses internally.
const neutralCalibrationBps = uint64(500_000)

// BuildScore is the synthesizer. Reads from each upstream keeper,
// composes a TrustScore. Stateless and deterministic given current
// chain state. The returned score is a current statement.
//
// Composition (BPS-scaled throughout):
//
//	composite = (submission_accuracy + verification_accuracy) / 2
//	          ─ cartel_strikes × 250_000   (heavy hit per confirmed strike)
//	          ─ active_penalties × 50_000  (lighter, transient)
//	composite is clamped to [0, 1_000_000]
//
// Band:
//
//	F: cartel_strikes > 0  OR  composite < 400_000
//	C: composite < 600_000
//	B: composite < 800_000
//	A: composite ≥ 800_000  AND  no cartel strikes  AND  no active penalties
func (k Keeper) BuildScore(ctx context.Context, address string) (*types.TrustScore, error) {
	if address == "" {
		return nil, fmt.Errorf("address required")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	out := &types.TrustScore{
		Address:        address,
		ComputedAtBlock: uint64(sdkCtx.BlockHeight()),
	}

	// ── Submission accuracy: from AgentCalibration ─────────────────
	out.SubmissionAccuracyBps = neutralCalibrationBps
	if k.knowledgeKeeper != nil {
		if cal, ok := k.knowledgeKeeper.GetAgentCalibration(ctx, address); ok && cal != nil {
			if cal.CalibrationScoreBps > 0 {
				out.SubmissionAccuracyBps = cal.CalibrationScoreBps
			}
		}
	}

	// ── Verification accuracy: weighted average across this address's
	// qualifications. Each domain's AccuracyBps is weighted by its
	// effective qualification weight (penalty-adjusted), so a primary
	// domain with strong record dominates secondary cross-references.
	var perDomain []*types.DomainTrust
	var totalWeight uint64
	var weightedAccuracy uint64
	var activePenalties uint32
	if k.qualificationKeeper != nil {
		k.qualificationKeeper.IterateQualifications(ctx, func(q *qualificationtypes.DomainQualification) bool {
			if q == nil || q.Validator != address {
				return false
			}
			effective := uint32(0)
			if q.Status == qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE {
				effective = k.qualificationKeeper.GetQualificationWeight(ctx, address, q.Domain)
			}
			accBps := uint64(0)
			var totalVer uint64
			if q.Metrics != nil {
				accBps = q.Metrics.AccuracyBps
				totalVer = q.Metrics.TotalVerifications
			}
			pen, hasPen := k.qualificationKeeper.GetActiveQualificationPenalty(ctx, address, q.Domain)
			if hasPen && pen != nil {
				activePenalties++
			}
			perDomain = append(perDomain, &types.DomainTrust{
				Domain:             q.Domain,
				Status:             uint32(q.Status),
				EffectiveWeight:    effective,
				AccuracyBps:        accBps,
				TotalVerifications: totalVer,
				HasActivePenalty:   hasPen,
			})
			if effective > 0 {
				w := uint64(effective)
				weightedAccuracy += accBps * w
				totalWeight += w
			}
			return false
		})
	}
	sort.Slice(perDomain, func(i, j int) bool { return perDomain[i].Domain < perDomain[j].Domain })
	out.PerDomain = perDomain
	out.ActivePenalties = activePenalties

	if totalWeight > 0 {
		out.VerificationAccuracyBps = weightedAccuracy / totalWeight
	} else {
		out.VerificationAccuracyBps = neutralCalibrationBps
	}

	// ── Cartel strikes: from capture_challenge adapter ─────────────
	if k.captureChallengeKeeper != nil {
		out.CartelStrikes = k.captureChallengeKeeper.CountUpheldStrikesAgainst(ctx, address)
	}

	// ── Composite ──────────────────────────────────────────────────
	base := (out.SubmissionAccuracyBps + out.VerificationAccuracyBps) / 2
	cartelHit := uint64(out.CartelStrikes) * 250_000
	penaltyHit := uint64(out.ActivePenalties) * 50_000
	if cartelHit+penaltyHit >= base {
		out.CompositeBps = 0
	} else {
		out.CompositeBps = base - cartelHit - penaltyHit
	}

	// ── Band ───────────────────────────────────────────────────────
	out.Band, out.Explanation = computeBand(out.CompositeBps, out.CartelStrikes, out.ActivePenalties)

	return out, nil
}

func computeBand(composite uint64, strikes uint32, penalties uint32) (string, string) {
	if strikes > 0 {
		return "F", fmt.Sprintf("%d cartel strike(s) confirmed against this address; trust is structurally compromised regardless of other signals", strikes)
	}
	if composite < 400_000 {
		return "F", fmt.Sprintf("composite %d below 40%%; address has not earned consensus-aligned reputation", composite)
	}
	if composite < 600_000 {
		return "C", fmt.Sprintf("composite %d in 40-60%% band; mediocre track record", composite)
	}
	if composite < 800_000 {
		return "B", fmt.Sprintf("composite %d in 60-80%% band; solid track record with some friction", composite)
	}
	if penalties > 0 {
		return "B", fmt.Sprintf("composite %d would qualify for A but %d active penalty/penalties prevent top band", composite, penalties)
	}
	return "A", fmt.Sprintf("composite %d ≥ 80%%, no strikes, no active penalties — top trust band", composite)
}
