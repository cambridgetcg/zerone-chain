package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/capture_challenge/types"
	provtypes "github.com/zerone-chain/zerone/x/training_provenance/types"
)

// TrainingProvenanceAdapter wraps the capture_challenge keeper to satisfy
// the training_provenance module's CaptureChallengeKeeper expected
// interface. Translates this module's native CaptureChallenge type into
// the lean ChallengeView the synthesizer needs, keeping the module
// graph one-directional (training_provenance has no import on
// x/capture_challenge native types).
type TrainingProvenanceAdapter struct {
	k Keeper
}

// NewTrainingProvenanceAdapter returns the adapter.
func NewTrainingProvenanceAdapter(k Keeper) *TrainingProvenanceAdapter {
	return &TrainingProvenanceAdapter{k: k}
}

// IterateChallenges yields each capture challenge translated into the
// minimal view training_provenance needs (id / domain / outcome / resolved).
func (a *TrainingProvenanceAdapter) IterateChallenges(ctx context.Context, cb func(provtypes.ChallengeView) bool) {
	a.k.IterateChallenges(ctx, func(c *types.CaptureChallenge) bool {
		if c == nil {
			return false
		}
		view := provtypes.ChallengeView{
			Id:       c.Id,
			Domain:   c.Domain,
			Resolved: c.Status == types.ChallengeStatus_CHALLENGE_STATUS_RESOLVED,
		}
		if c.Resolution != nil {
			switch c.Resolution.Outcome {
			case types.ChallengeOutcome_CHALLENGE_OUTCOME_UPHELD:
				view.Outcome = "upheld"
			case types.ChallengeOutcome_CHALLENGE_OUTCOME_REJECTED:
				view.Outcome = "rejected"
			case types.ChallengeOutcome_CHALLENGE_OUTCOME_PARTIAL:
				view.Outcome = "partial"
			default:
				view.Outcome = "pending"
			}
		}
		return cb(view)
	})
}
