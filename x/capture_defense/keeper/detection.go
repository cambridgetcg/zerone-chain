package keeper

import (
	"math"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// AnalyzeCaptureRisk computes capture metrics for a domain from its verification history.
func (k Keeper) AnalyzeCaptureRisk(ctx sdk.Context, domain string, params *types.Params) *types.CaptureMetrics {
	history := k.GetHistoryByDomain(ctx, domain)
	if len(history) == 0 {
		return nil
	}

	// 1. Count participations per validator
	participations := make(map[string]uint64)
	var totalParticipations uint64
	for _, entry := range history {
		for _, v := range entry.Validators {
			participations[v]++
			totalParticipations++
		}
	}

	if totalParticipations == 0 {
		return nil
	}

	// 2. HHI = sum of (share_i)^2 on BPS scale
	var hhi uint64
	for _, count := range participations {
		share := count * types.BPSScale / totalParticipations
		hhi += share * share / types.BPSScale
	}

	// 3. Top-3 share
	top3 := top3Share(participations, totalParticipations)

	// 4. Timing correlation
	timing := timingCorrelation(history)

	// 5. Verdict correlation
	verdict := verdictCorrelation(history)

	// 6. Composite risk score: weighted average
	riskScore := (hhi*40 + timing*20 + verdict*20 + top3*20) / 100

	// Adjust HHI threshold by domain depth: deeper (more specific) domains
	// get a more lenient threshold. +50,000 BPS per depth level above 1.
	adjustedThreshold := params.HhiThreshold
	if k.ontologyKeeper != nil {
		if depth, err := k.ontologyKeeper.GetDepthForDomain(ctx, domain); err == nil && depth > 1 {
			adjustedThreshold += uint64(depth-1) * 50000
		}
	}

	flagged := hhi > adjustedThreshold

	metrics := &types.CaptureMetrics{
		Domain:              domain,
		HerfindahlIndex:     hhi,
		TimingCorrelation:   timing,
		VerdictCorrelation:  verdict,
		Top3Share:           top3,
		RiskScore:           riskScore,
		TotalParticipations: totalParticipations,
		AnalyzedAtBlock:     uint64(ctx.BlockHeight()),
		Flagged:             flagged,
	}
	k.SetCaptureMetrics(ctx, metrics)
	return metrics
}

// RunCaptureDetection is a convenience wrapper that calls AnalyzeCaptureRisk.
func (k Keeper) RunCaptureDetection(ctx sdk.Context, domain string, params *types.Params) *types.CaptureMetrics {
	return k.AnalyzeCaptureRisk(ctx, domain, params)
}

// top3Share returns the BPS fraction of the top 3 validators' combined share.
func top3Share(participations map[string]uint64, total uint64) uint64 {
	if total == 0 {
		return 0
	}

	// Collect counts and sort descending
	counts := make([]uint64, 0, len(participations))
	for _, c := range participations {
		counts = append(counts, c)
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i] > counts[j]
	})

	// Sum top 3
	var topSum uint64
	for i := 0; i < 3 && i < len(counts); i++ {
		topSum += counts[i]
	}

	return topSum * types.BPSScale / total
}

// timingCorrelation measures the fraction of rounds where validators submitted
// within a tight window (std dev < 2 blocks). Returns BPS fraction.
func timingCorrelation(history []*types.VerificationHistoryEntry) uint64 {
	if len(history) == 0 {
		return 0
	}

	var correlated uint64
	var total uint64

	for _, entry := range history {
		if len(entry.SubmitBlocks) < 2 {
			continue
		}
		total++

		// Compute standard deviation
		var sum uint64
		for _, b := range entry.SubmitBlocks {
			sum += b
		}
		mean := float64(sum) / float64(len(entry.SubmitBlocks))

		var varianceSum float64
		for _, b := range entry.SubmitBlocks {
			diff := float64(b) - mean
			varianceSum += diff * diff
		}
		stdDev := math.Sqrt(varianceSum / float64(len(entry.SubmitBlocks)))

		if stdDev < 2.0 {
			correlated++
		}
	}

	if total == 0 {
		return 0
	}
	return correlated * types.BPSScale / total
}

// verdictCorrelation measures the fraction of rounds where all validators
// gave identical verdicts. Returns BPS fraction.
func verdictCorrelation(history []*types.VerificationHistoryEntry) uint64 {
	if len(history) == 0 {
		return 0
	}

	var unanimous uint64
	var total uint64

	for _, entry := range history {
		if len(entry.Verdicts) < 2 {
			continue
		}
		total++

		allSame := true
		first := entry.Verdicts[0]
		for _, v := range entry.Verdicts[1:] {
			if v != first {
				allSame = false
				break
			}
		}
		if allSame {
			unanimous++
		}
	}

	if total == 0 {
		return 0
	}
	return unanimous * types.BPSScale / total
}
