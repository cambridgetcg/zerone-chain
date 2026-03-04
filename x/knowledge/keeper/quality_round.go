package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// InitiateQualityRound creates a new quality round for a submission (or thread).
func (k Keeper) InitiateQualityRound(
	ctx context.Context,
	submissionID string,
	threadID string,
	selectedVerifiers []string,
) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return "", err
	}

	sub, found := k.GetSubmission(ctx, submissionID)
	if !found {
		return "", types.ErrSubmissionNotFound.Wrapf("submission %q not found", submissionID)
	}

	block := uint64(sdkCtx.BlockHeight())
	commitDeadline := block + params.CommitPeriodBlocks
	revealDeadline := commitDeadline + params.RevealPeriodBlocks

	roundID := k.NextRoundID(ctx)
	round := &types.QualityRound{
		Id:                roundID,
		SubmissionId:      submissionID,
		StartedAtBlock:    block,
		Phase:             types.VerificationPhase_VERIFICATION_PHASE_COMMIT,
		SelectedVerifiers: selectedVerifiers,
		CommitDeadline:    commitDeadline,
		RevealDeadline:    revealDeadline,
	}

	if err := k.SetQualityRound(ctx, round); err != nil {
		return "", err
	}
	if err := k.SetActiveRound(ctx, roundID); err != nil {
		return "", err
	}
	if err := k.SetSubmissionRoundIndex(ctx, submissionID, roundID); err != nil {
		return "", err
	}

	sub.QualityRoundId = roundID
	sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW
	if err := k.SetSubmission(ctx, sub); err != nil {
		return "", err
	}

	// If thread: link all thread submissions to this round
	if threadID != "" {
		k.IterateSubmissions(ctx, func(s *types.Submission) bool {
			if s.ThreadId == threadID && s.Id != submissionID {
				_ = k.SetSubmissionRoundIndex(ctx, s.Id, roundID)
				s.QualityRoundId = roundID
				s.Status = types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW
				_ = k.SetSubmission(ctx, s)
			}
			return false
		})
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"quality_round_started",
		sdk.NewAttribute("round_id", roundID),
		sdk.NewAttribute("submission_id", submissionID),
		sdk.NewAttribute("thread_id", threadID),
		sdk.NewAttribute("verifier_count", strconv.Itoa(len(selectedVerifiers))),
		sdk.NewAttribute("commit_deadline", strconv.FormatUint(commitDeadline, 10)),
		sdk.NewAttribute("reveal_deadline", strconv.FormatUint(revealDeadline, 10)),
	))

	return roundID, nil
}

// SubmitCommitment handles MsgSubmitCommitment — stores a blinded quality vote commitment.
func (k Keeper) SubmitCommitment(ctx context.Context, msg *types.MsgSubmitCommitment) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	round, found := k.GetQualityRound(ctx, msg.RoundId)
	if !found {
		return types.ErrRoundNotFound.Wrapf("round %q not found", msg.RoundId)
	}

	if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_COMMIT {
		return types.ErrWrongPhase.Wrap("round is not in commit phase")
	}

	if uint64(sdkCtx.BlockHeight()) > round.CommitDeadline {
		return types.ErrDeadlinePassed.Wrap("commit deadline has passed")
	}

	selected := false
	for _, v := range round.SelectedVerifiers {
		if v == msg.Verifier {
			selected = true
			break
		}
	}
	if !selected {
		return types.ErrNotSelectedValidator.Wrapf("verifier %s not selected", msg.Verifier)
	}

	for _, c := range round.Commits {
		if c.Verifier == msg.Verifier {
			return types.ErrAlreadyCommitted.Wrapf("verifier %s already committed", msg.Verifier)
		}
	}

	round.Commits = append(round.Commits, &types.CommitEntry{
		Verifier:         msg.Verifier,
		CommitHash:       msg.CommitHash,
		CommittedAtBlock: uint64(sdkCtx.BlockHeight()),
	})

	return k.SetQualityRound(ctx, round)
}

const maxScoreBPS = 1_000_000

// SubmitReveal handles MsgSubmitReveal — verifies commitment hash and stores revealed vote.
func (k Keeper) SubmitReveal(ctx context.Context, msg *types.MsgSubmitReveal) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	round, found := k.GetQualityRound(ctx, msg.RoundId)
	if !found {
		return types.ErrRoundNotFound.Wrapf("round %q not found", msg.RoundId)
	}

	if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_REVEAL {
		return types.ErrWrongPhase.Wrap("round is not in reveal phase")
	}

	if uint64(sdkCtx.BlockHeight()) > round.RevealDeadline {
		return types.ErrDeadlinePassed.Wrap("reveal deadline has passed")
	}

	selected := false
	for _, v := range round.SelectedVerifiers {
		if v == msg.Verifier {
			selected = true
			break
		}
	}
	if !selected {
		return types.ErrNotSelectedValidator.Wrapf("verifier %s not selected", msg.Verifier)
	}

	// Find commitment
	var commitHash []byte
	for _, c := range round.Commits {
		if c.Verifier == msg.Verifier {
			commitHash = c.CommitHash
			break
		}
	}
	if commitHash == nil {
		return types.ErrNoCommitment.Wrapf("no commitment from verifier %s", msg.Verifier)
	}

	// Check duplicate reveal
	for _, r := range round.Reveals {
		if r.Verifier == msg.Verifier {
			return types.ErrAlreadyRevealed.Wrapf("verifier %s already revealed", msg.Verifier)
		}
	}

	// Validate score range (defense-in-depth)
	if msg.Scores.OverallQuality > maxScoreBPS ||
		msg.Scores.ReasoningDepth > maxScoreBPS ||
		msg.Scores.Novelty > maxScoreBPS ||
		msg.Scores.Toxicity > maxScoreBPS ||
		msg.Scores.FactualAccuracy > maxScoreBPS {
		return types.ErrInvalidQualityScore.Wrap("score exceeds 1,000,000 BPS maximum")
	}

	// Verify commitment hash
	if !types.VerifyQualityCommitHash(commitHash, msg.RoundId, msg.Scores, msg.Salt) {
		return types.ErrRevealMismatch.Wrap("revealed scores do not match commitment")
	}

	// Serialize vote as JSON for storage in RevealEntry.Vote
	voteJSON, err := json.Marshal(msg.Scores)
	if err != nil {
		return fmt.Errorf("failed to marshal quality vote: %w", err)
	}

	round.Reveals = append(round.Reveals, &types.RevealEntry{
		Verifier:        msg.Verifier,
		Vote:            string(voteJSON),
		Salt:            msg.Salt,
		RevealedAtBlock: uint64(sdkCtx.BlockHeight()),
	})

	return k.SetQualityRound(ctx, round)
}
