package keeper

import (
	"context"
	"math/big"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// VerificationResult holds the aggregated outcome of a verification round.
// This is a local struct (not proto) used only within the keeper.
type VerificationResult struct {
	Verdict    types.Verdict
	Confidence uint64 // 0-1,000,000
	Rewards    []VerifierReward
	Slashes    []VerifierSlash
}

// VerifierReward records a reward amount for a correct verifier.
type VerifierReward struct {
	Verifier string
	Amount   uint64
}

// VerifierSlash records a slash for an incorrect or absent verifier.
type VerifierSlash struct {
	Verifier string
	SlashBps uint64
}

// AggregateVerificationResult performs stake-weighted vote aggregation.
func (k Keeper) AggregateVerificationResult(ctx context.Context, round *types.VerificationRound) (*VerificationResult, error) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// Count stake-weighted accept/reject votes from reveals
	var acceptStake, rejectStake, totalVoteStake uint64

	for _, reveal := range round.Reveals {
		var stake uint64
		if k.stakingKeeper != nil {
			s, err := k.stakingKeeper.GetEffectiveStake(ctx, reveal.Verifier)
			if err == nil {
				stake = s
			}
		}
		if stake == 0 {
			stake = 1 // minimum weight for unknown validators
		}

		totalVoteStake += stake
		switch reveal.Vote {
		case "accept":
			acceptStake += stake
		case "reject":
			rejectStake += stake
		}
	}

	result := &VerificationResult{}

	// Check quorum: total reveals must be >= minVerifiers
	if uint64(len(round.Reveals)) < params.MinVerifiers {
		result.Verdict = types.Verdict_VERDICT_INCONCLUSIVE
		result.Confidence = 0
		return result, nil
	}

	// Calculate vote ratios (BPS scale)
	var acceptRatio, rejectRatio uint64
	if totalVoteStake > 0 {
		acceptRatio = safeMulDiv(acceptStake, 1_000_000, totalVoteStake)
		rejectRatio = safeMulDiv(rejectStake, 1_000_000, totalVoteStake)
	}

	// Determine verdict
	if acceptRatio >= params.ConfidenceThreshold {
		result.Verdict = types.Verdict_VERDICT_ACCEPT
		result.Confidence = acceptRatio
	} else if rejectRatio >= params.ConfidenceThreshold {
		result.Verdict = types.Verdict_VERDICT_REJECT
		result.Confidence = rejectRatio
	} else {
		result.Verdict = types.Verdict_VERDICT_INCONCLUSIVE
		result.Confidence = 0
	}

	// Apply stratum ceiling to confidence
	if k.ontologyKeeper != nil && result.Verdict == types.Verdict_VERDICT_ACCEPT {
		claim, found := k.GetClaim(ctx, round.ClaimId)
		if found && claim.Domain != "" {
			stratum, err := k.ontologyKeeper.GetStratumForDomain(ctx, claim.Domain)
			if err == nil && stratum != "" {
				ceiling, err := k.ontologyKeeper.GetConfidenceCeiling(ctx, stratum)
				if err == nil && ceiling > 0 && result.Confidence > ceiling {
					result.Confidence = ceiling
				}
			}
		}
	}

	// Calculate rewards and slashes
	k.calculateRewardsAndSlashes(ctx, round, result, params)

	return result, nil
}

// calculateRewardsAndSlashes determines rewards for correct voters and slashes for incorrect ones.
func (k Keeper) calculateRewardsAndSlashes(ctx context.Context, round *types.VerificationRound, result *VerificationResult, params *types.Params) {
	// Determine which vote is "correct" based on verdict
	var correctVote string
	switch result.Verdict {
	case types.Verdict_VERDICT_ACCEPT:
		correctVote = "accept"
	case types.Verdict_VERDICT_REJECT:
		correctVote = "reject"
	default:
		// Inconclusive — no rewards or slashes
		return
	}

	// Parse base reward
	baseReward := new(big.Int)
	if params.VerificationReward != "" {
		baseReward.SetString(params.VerificationReward, 10)
	}
	if baseReward.Sign() <= 0 {
		return
	}

	// Build reveal map for lookup
	revealMap := make(map[string]string) // verifier → vote
	for _, reveal := range round.Reveals {
		revealMap[reveal.Verifier] = reveal.Vote
	}

	// Process all committed verifiers
	for _, commit := range round.Commits {
		vote, revealed := revealMap[commit.Verifier]

		if !revealed {
			// Committed but did not reveal — slash for missed reveal
			result.Slashes = append(result.Slashes, VerifierSlash{
				Verifier: commit.Verifier,
				SlashBps: params.MissedRevealSlashBps,
			})
			continue
		}

		if vote == correctVote {
			// Correct vote — reward
			if baseReward.IsUint64() {
				result.Rewards = append(result.Rewards, VerifierReward{
					Verifier: commit.Verifier,
					Amount:   baseReward.Uint64(),
				})
			}
		} else {
			// Incorrect vote — slash
			result.Slashes = append(result.Slashes, VerifierSlash{
				Verifier: commit.Verifier,
				SlashBps: params.WrongVerificationSlashBps,
			})
		}
	}
}

// safeMulDiv computes (a * b / c) using big.Int to prevent overflow.
func safeMulDiv(a, b, c uint64) uint64 {
	if c == 0 {
		return 0
	}
	result := new(big.Int).SetUint64(a)
	result.Mul(result, new(big.Int).SetUint64(b))
	result.Div(result, new(big.Int).SetUint64(c))
	if !result.IsUint64() {
		return ^uint64(0)
	}
	return result.Uint64()
}
