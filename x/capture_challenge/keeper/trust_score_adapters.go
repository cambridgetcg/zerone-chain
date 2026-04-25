package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/capture_challenge/types"
)

// TrustScoreAdapter wraps the capture_challenge keeper to satisfy the
// trust_score module's CaptureChallengeKeeper expected interface.
//
// One method: CountUpheldStrikesAgainst — how many UPHELD challenges
// list `addr` in their AccusedValidators slice. This is the
// per-address read trust_score wants; the existing IterateChallenges
// iteration is reused with an addr-filter on top.
type TrustScoreAdapter struct {
	k Keeper
}

func NewTrustScoreAdapter(k Keeper) *TrustScoreAdapter {
	return &TrustScoreAdapter{k: k}
}

// CountUpheldStrikesAgainst returns the number of RESOLVED+UPHELD
// challenges in which addr appears as accused.
func (a *TrustScoreAdapter) CountUpheldStrikesAgainst(ctx context.Context, addr string) uint32 {
	if addr == "" {
		return 0
	}
	var count uint32
	a.k.IterateChallenges(ctx, func(c *types.CaptureChallenge) bool {
		if c == nil ||
			c.Status != types.ChallengeStatus_CHALLENGE_STATUS_RESOLVED ||
			c.Resolution == nil ||
			c.Resolution.Outcome != types.ChallengeOutcome_CHALLENGE_OUTCOME_UPHELD {
			return false
		}
		for _, accused := range c.AccusedValidators {
			if accused == addr {
				count++
				break
			}
		}
		return false
	})
	return count
}
