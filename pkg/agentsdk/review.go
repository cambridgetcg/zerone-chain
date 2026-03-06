package agentsdk

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// CommitReview commits a blinded quality score for a round.
// Generates a random salt, stores it and the score for later reveal, and broadcasts the commitment.
func (c *ToKClient) CommitReview(ctx context.Context, roundID string, score ReviewScore) (*ReviewResult, error) {
	if roundID == "" {
		return nil, fmt.Errorf("round ID is required")
	}

	// Generate and store salt
	salt, err := c.salts.GenerateAndStore(roundID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Store the score alongside the salt for AutoRevealAll
	if err := c.salts.StoreScore(roundID, score); err != nil {
		_ = c.salts.Delete(roundID)
		return nil, fmt.Errorf("failed to store score: %w", err)
	}

	// Build QualityVote for commitment hash
	vote := scoreToVote(score)

	// Compute commitment hash
	commitHash := types.ComputeQualityCommitHash(roundID, vote, salt)

	msg := &types.MsgSubmitCommitment{
		Verifier:   c.chain.GetAddress(),
		RoundId:    roundID,
		CommitHash: commitHash,
	}

	txHash, err := c.broadcastWithRetry(ctx, msg)
	if err != nil {
		// Clean up salt and score on broadcast failure
		_ = c.salts.Delete(roundID)
		return nil, fmt.Errorf("failed to commit review: %w", err)
	}

	return &ReviewResult{TxHash: txHash}, nil
}

// RevealReview reveals a previously committed quality score.
// Auto-loads the salt and score from storage.
func (c *ToKClient) RevealReview(ctx context.Context, roundID string, score ReviewScore) (*ReviewResult, error) {
	if roundID == "" {
		return nil, fmt.Errorf("round ID is required")
	}

	// Load stored salt
	salt, err := c.salts.Load(roundID)
	if err != nil {
		return nil, fmt.Errorf("no salt found for round %s (was commit called?): %w", roundID, err)
	}

	vote := scoreToVote(score)

	msg := &types.MsgSubmitReveal{
		Verifier: c.chain.GetAddress(),
		RoundId:  roundID,
		Scores:   vote,
		Salt:     salt,
	}

	txHash, err := c.broadcastWithRetry(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to reveal review: %w", err)
	}

	// Clean up salt and score after successful reveal
	_ = c.salts.Delete(roundID)

	return &ReviewResult{TxHash: txHash}, nil
}

// AutoRevealAll finds all pending salts, loads stored scores, and reveals them
// for rounds that are in the reveal phase. Returns the number of successful
// reveals and any errors encountered.
func (c *ToKClient) AutoRevealAll(ctx context.Context) (revealed int, errors []error) {
	pendingRounds, err := c.salts.ListPending()
	if err != nil {
		return 0, []error{fmt.Errorf("failed to list pending salts: %w", err)}
	}

	for _, roundID := range pendingRounds {
		// Query the round to check if it's in reveal phase
		round, err := c.chain.QueryQualityRound(ctx, roundID)
		if err != nil {
			errors = append(errors, fmt.Errorf("round %s: failed to query: %w", roundID, err))
			continue
		}

		// Only reveal if the round is in reveal phase
		if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_REVEAL {
			continue
		}

		// Load the stored score
		score, err := c.salts.LoadScore(roundID)
		if err != nil {
			errors = append(errors, fmt.Errorf("round %s: no stored score found: %w", roundID, err))
			continue
		}

		// Reveal
		_, err = c.RevealReview(ctx, roundID, *score)
		if err != nil {
			errors = append(errors, fmt.Errorf("round %s: reveal failed: %w", roundID, err))
			continue
		}

		revealed++
	}

	return revealed, errors
}

// scoreToVote converts a ReviewScore to a proto QualityVote.
func scoreToVote(score ReviewScore) *types.QualityVote {
	return &types.QualityVote{
		OverallQuality:  score.OverallQuality,
		ReasoningDepth:  score.ReasoningDepth,
		Novelty:         score.Novelty,
		Toxicity:        score.Toxicity,
		FactualAccuracy: score.FactualAccuracy,
		ConsentValid:    score.ConsentValid,
		Duplicate:       score.Duplicate,
		Notes:           score.Notes,
	}
}
