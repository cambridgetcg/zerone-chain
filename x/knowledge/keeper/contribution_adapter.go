package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GetClaimVerificationScore returns the PoT verification score (0..1_000_000 BPS)
// for a claim by id, plus a found flag. Implements the
// x/contribution/adapter/knowledgeclaim.KnowledgeKeeperReader interface so
// x/contribution can wire this keeper as its KNOWLEDGE_CLAIM data source.
//
// Semantics:
//   - claim absent → (0, false)
//   - claim present and ACCEPTED → (fact.Confidence, true) by locating the
//     Fact whose ClaimId matches the claim id
//   - claim present in any other terminal/intermediate state → (0, true)
//     (the score is genuinely zero for rejected/malformed/insufficient claims;
//     for SUBMITTED/IN_VERIFICATION the score is not yet computed)
func (k Keeper) GetClaimVerificationScore(ctx context.Context, claimID string) (uint32, bool) {
	claim, found := k.GetClaim(ctx, claimID)
	if !found {
		return 0, false
	}
	if claim.Status != types.ClaimStatus_CLAIM_STATUS_ACCEPTED {
		return 0, true
	}
	// Find the Fact created from this claim. Phase 1 uses a linear scan;
	// a claim_id → fact_id index can be added in a later phase if hot-pathed.
	var score uint32
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.ClaimId == claimID {
			conf := f.Confidence
			if conf > 1_000_000 {
				conf = 1_000_000
			}
			score = uint32(conf)
			return true // stop iteration
		}
		return false
	})
	return score, true
}
