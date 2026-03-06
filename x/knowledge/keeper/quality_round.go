package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	sdkmath "cosmossdk.io/math"
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

// SubmitCommitment handles MsgSubmitCommitment — stores a blinded quality vote
// commitment and escrows the reviewer's stake.
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

	// Escrow reviewer stake: submitterStake × ReviewerStakeRatioBps / 10000.
	sub, subFound := k.GetSubmission(ctx, round.SubmissionId)
	if subFound && sub.Stake != "" {
		rp := k.GetReviewerStakingParams(ctx)
		submitterStake, ok := sdkmath.NewIntFromString(sub.Stake)
		if ok && submitterStake.IsPositive() && rp.ReviewerStakeRatioBps > 0 {
			reviewerStake := submitterStake.Mul(sdkmath.NewInt(int64(rp.ReviewerStakeRatioBps))).Quo(sdkmath.NewInt(10_000))
			if reviewerStake.IsPositive() {
				verifierAddr, err := sdk.AccAddressFromBech32(msg.Verifier)
				if err == nil {
					if err := k.bankKeeper.SendCoinsFromAccountToModule(
						sdkCtx, verifierAddr, types.ModuleName,
						sdk.NewCoins(sdk.NewCoin("uzrn", reviewerStake)),
					); err != nil {
						return types.ErrReviewerStakeInsufficient.Wrap(err.Error())
					}
					if err := k.SetReviewerStake(ctx, msg.RoundId, msg.Verifier, reviewerStake.String()); err != nil {
						return err
					}
				}
			}
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

// AggregateQualityRound computes the verdict from revealed QualityVotes.
func (k Keeper) AggregateQualityRound(ctx context.Context, roundID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	round, found := k.GetQualityRound(ctx, roundID)
	if !found {
		return types.ErrRoundNotFound.Wrapf("round %q not found", roundID)
	}

	if len(round.Reveals) == 0 {
		return nil
	}

	// Deserialize revealed votes
	votes := make([]*types.QualityVote, 0, len(round.Reveals))
	for _, reveal := range round.Reveals {
		var vote types.QualityVote
		if err := json.Unmarshal([]byte(reveal.Vote), &vote); err != nil {
			continue
		}
		votes = append(votes, &vote)
	}

	if len(votes) == 0 {
		return nil
	}

	// Compute aggregated scores (median per dimension)
	aggregated := &types.QualityVote{
		OverallQuality:  medianUint64(votes, func(v *types.QualityVote) uint64 { return v.OverallQuality }),
		ReasoningDepth:  medianUint64(votes, func(v *types.QualityVote) uint64 { return v.ReasoningDepth }),
		Novelty:         medianUint64(votes, func(v *types.QualityVote) uint64 { return v.Novelty }),
		Toxicity:        medianUint64(votes, func(v *types.QualityVote) uint64 { return v.Toxicity }),
		FactualAccuracy: medianUint64(votes, func(v *types.QualityVote) uint64 { return v.FactualAccuracy }),
	}

	// Consent consensus: majority vote
	consentValid := majorityBool(votes, func(v *types.QualityVote) bool { return v.ConsentValid })
	aggregated.ConsentValid = consentValid

	// Duplicate consensus: majority vote
	isDuplicate := majorityBool(votes, func(v *types.QualityVote) bool { return v.Duplicate })
	aggregated.Duplicate = isDuplicate

	// Determine verdict (priority order)
	var verdict types.QualityVerdict
	switch {
	case !consentValid:
		verdict = types.QualityVerdict_QUALITY_VERDICT_CONSENT_FAIL
	case isDuplicate:
		verdict = types.QualityVerdict_QUALITY_VERDICT_REJECT
	case aggregated.Toxicity > params.MaxToxicityThreshold:
		verdict = types.QualityVerdict_QUALITY_VERDICT_REJECT
	case aggregated.OverallQuality >= params.GoldThreshold:
		verdict = types.QualityVerdict_QUALITY_VERDICT_GOLD
	case aggregated.OverallQuality >= params.SilverThreshold:
		verdict = types.QualityVerdict_QUALITY_VERDICT_SILVER
	case aggregated.OverallQuality >= params.BronzeThreshold:
		verdict = types.QualityVerdict_QUALITY_VERDICT_BRONZE
	default:
		verdict = types.QualityVerdict_QUALITY_VERDICT_REJECT
	}

	// Score validators: reward consensus, slash outliers and missed reveals
	k.scoreValidators(ctx, round, aggregated)

	// Update round
	round.Verdict = verdict
	round.VerdictBlock = uint64(sdkCtx.BlockHeight())
	round.AggregateScores = aggregated
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_COMPLETE

	if err := k.SetQualityRound(ctx, round); err != nil {
		return err
	}

	// Remove from active index
	if err := k.DeleteActiveRound(ctx, roundID); err != nil {
		return err
	}

	// Get submission
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	if !found {
		return types.ErrSubmissionNotFound
	}

	// Handle verdict outcomes
	accepted := verdict == types.QualityVerdict_QUALITY_VERDICT_GOLD ||
		verdict == types.QualityVerdict_QUALITY_VERDICT_SILVER ||
		verdict == types.QualityVerdict_QUALITY_VERDICT_BRONZE

	if accepted {
		if err := k.createSampleFromSubmission(ctx, sub, verdict, aggregated); err != nil {
			return err
		}
		sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED
	} else {
		sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_REJECTED
		if verdict == types.QualityVerdict_QUALITY_VERDICT_CONSENT_FAIL {
			sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_CONSENT_FAILED
		}
	}

	// Distribute stakes via reviewer staking mechanism.
	if err := k.distributeReviewerStakes(ctx, round, sub, params); err != nil {
		return err
	}

	if err := k.SetSubmission(ctx, sub); err != nil {
		return err
	}

	// Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"quality_round_completed",
		sdk.NewAttribute("round_id", roundID),
		sdk.NewAttribute("submission_id", round.SubmissionId),
		sdk.NewAttribute("verdict", verdict.String()),
		sdk.NewAttribute("overall_quality", strconv.FormatUint(aggregated.OverallQuality, 10)),
	))

	return nil
}

// verdictToSampleStatus maps a QualityVerdict to a SampleStatus.
func verdictToSampleStatus(v types.QualityVerdict) types.SampleStatus {
	switch v {
	case types.QualityVerdict_QUALITY_VERDICT_GOLD:
		return types.SampleStatus_SAMPLE_STATUS_GOLD
	case types.QualityVerdict_QUALITY_VERDICT_SILVER:
		return types.SampleStatus_SAMPLE_STATUS_SILVER
	case types.QualityVerdict_QUALITY_VERDICT_BRONZE:
		return types.SampleStatus_SAMPLE_STATUS_BRONZE
	default:
		return types.SampleStatus_SAMPLE_STATUS_REJECTED
	}
}

// createSampleFromSubmission promotes an accepted submission to a Sample.
func (k Keeper) createSampleFromSubmission(
	ctx context.Context,
	sub *types.Submission,
	verdict types.QualityVerdict,
	scores *types.QualityVote,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	tier := types.QualityVerdictToTier(verdict)

	sampleID := k.NextSampleID(ctx)
	sample := &types.Sample{
		Id:              sampleID,
		Content:         sub.Content,
		SampleType:      sub.SampleType,
		Domain:          sub.Domain,
		SourceUri:       sub.SourceUri,
		SourcePlatform:  sub.SourcePlatform,
		SourceTimestamp:  sub.SourceTimestamp,
		QualityScore:    scores.OverallQuality,
		QualityTier:     string(tier),
		NoveltyScore:    scores.Novelty,
		ReasoningDepth:  scores.ReasoningDepth,
		Submitter:       sub.Submitter,
		OriginalAuthor:  sub.OriginalAuthor,
		Consent:         sub.Consent,
		License:         sub.License,
		SubmissionId:    sub.Id,
		ThreadId:        sub.ThreadId,
		Tags:            sub.Tags,
		Language:        sub.Language,
		Topics:          sub.Tags, // Use tags as topics for now
		Status:          verdictToSampleStatus(verdict),
		VerifiedAtBlock: uint64(sdkCtx.BlockHeight()),
	}

	// Initialize ecology fields
	initializeSampleEnergy(sample)
	primaryTopic := ""
	if len(sub.Tags) > 0 {
		primaryTopic = sub.Tags[0]
	}
	sample.NicheKey = ComputeNicheKey(sub.Domain, sub.SampleType, primaryTopic)

	if err := k.SetSample(ctx, sample); err != nil {
		return err
	}
	if err := k.SetSampleDomainIndex(ctx, sub.Domain, sampleID); err != nil {
		return err
	}
	if err := k.SetSampleSubmitterIndex(ctx, sub.Submitter, sampleID); err != nil {
		return err
	}
	if err := k.SetNicheIndex(ctx, sample.NicheKey, sampleID); err != nil {
		return err
	}
	if sub.ThreadId != "" {
		if err := k.SetSampleThreadIndex(ctx, sub.ThreadId, sampleID); err != nil {
			return err
		}
	}
	// Track topic saturation
	for _, tag := range sub.Tags {
		_ = k.IncrementTopicCount(ctx, sub.Domain, tag)
	}

	// If thread: create samples for all other thread submissions
	if sub.ThreadId != "" {
		return k.createThreadSamples(ctx, sub, verdict, scores, sampleID)
	}

	return nil
}

// createThreadSamples creates samples for sibling thread submissions.
func (k Keeper) createThreadSamples(
	ctx context.Context,
	primarySub *types.Submission,
	verdict types.QualityVerdict,
	scores *types.QualityVote,
	primarySampleID string,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	tier := types.QualityVerdictToTier(verdict)
	status := verdictToSampleStatus(verdict)

	subToSample := map[string]string{primarySub.Id: primarySampleID}

	var threadSubs []*types.Submission
	k.IterateSubmissions(ctx, func(s *types.Submission) bool {
		if s.ThreadId == primarySub.ThreadId && s.Id != primarySub.Id {
			threadSubs = append(threadSubs, s)
		}
		return false
	})

	sort.Slice(threadSubs, func(i, j int) bool {
		return threadSubs[i].Id < threadSubs[j].Id
	})

	for _, sub := range threadSubs {
		sampleID := k.NextSampleID(ctx)
		sample := &types.Sample{
			Id:              sampleID,
			Content:         sub.Content,
			SampleType:      sub.SampleType,
			Domain:          sub.Domain,
			SourceUri:       sub.SourceUri,
			SourcePlatform:  sub.SourcePlatform,
			SourceTimestamp:  sub.SourceTimestamp,
			QualityScore:    scores.OverallQuality,
			QualityTier:     string(tier),
			NoveltyScore:    scores.Novelty,
			ReasoningDepth:  scores.ReasoningDepth,
			Submitter:       sub.Submitter,
			OriginalAuthor:  sub.OriginalAuthor,
			Consent:         sub.Consent,
			License:         sub.License,
			SubmissionId:    sub.Id,
			ThreadId:        sub.ThreadId,
			Tags:            sub.Tags,
			Language:        sub.Language,
			Status:          status,
			VerifiedAtBlock: uint64(sdkCtx.BlockHeight()),
		}

		// Initialize ecology fields for thread sibling
		initializeSampleEnergy(sample)
		siblingPrimaryTopic := ""
		if len(sub.Tags) > 0 {
			siblingPrimaryTopic = sub.Tags[0]
		}
		sample.NicheKey = ComputeNicheKey(sub.Domain, sub.SampleType, siblingPrimaryTopic)
		sample.Topics = sub.Tags

		if parentSampleID, ok := subToSample[sub.ParentSubmissionId]; ok {
			sample.ParentSampleId = parentSampleID
		}

		if err := k.SetSample(ctx, sample); err != nil {
			return err
		}
		if err := k.SetSampleDomainIndex(ctx, sub.Domain, sampleID); err != nil {
			return err
		}
		if err := k.SetSampleSubmitterIndex(ctx, sub.Submitter, sampleID); err != nil {
			return err
		}
		if err := k.SetSampleThreadIndex(ctx, sub.ThreadId, sampleID); err != nil {
			return err
		}
		if err := k.SetNicheIndex(ctx, sample.NicheKey, sampleID); err != nil {
			return err
		}
		for _, tag := range sub.Tags {
			_ = k.IncrementTopicCount(ctx, sub.Domain, tag)
		}

		subToSample[sub.Id] = sampleID

		sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED
		_ = k.SetSubmission(ctx, sub)
	}

	return nil
}

// medianUint64 computes the median of a uint64 field across votes.
func medianUint64(votes []*types.QualityVote, fn func(*types.QualityVote) uint64) uint64 {
	vals := make([]uint64, len(votes))
	for i, v := range votes {
		vals[i] = fn(v)
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	n := len(vals)
	if n%2 == 1 {
		return vals[n/2]
	}
	return (vals[n/2-1] + vals[n/2]) / 2
}

// majorityBool returns true if more than half of votes return true.
func majorityBool(votes []*types.QualityVote, fn func(*types.QualityVote) bool) bool {
	count := 0
	for _, v := range votes {
		if fn(v) {
			count++
		}
	}
	return count > len(votes)/2
}

// outlierThresholdBPS defines max deviation before a validator is outlier (200,000 = 20%).
const outlierThresholdBPS = 200_000

// scoreValidators rewards consensus validators and slashes outliers.
func (k Keeper) scoreValidators(ctx context.Context, round *types.QualityRound, aggregated *types.QualityVote) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Build set of verifiers who revealed
	revealedVerifiers := make(map[string]bool)
	for _, r := range round.Reveals {
		revealedVerifiers[r.Verifier] = true
	}

	// Slash validators who committed but didn't reveal
	for _, c := range round.Commits {
		if !revealedVerifiers[c.Verifier] {
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"validator_missed_reveal",
				sdk.NewAttribute("verifier", c.Verifier),
				sdk.NewAttribute("round_id", round.Id),
			))
		}
	}

	// Score revealed validators
	for _, reveal := range round.Reveals {
		var vote types.QualityVote
		if err := json.Unmarshal([]byte(reveal.Vote), &vote); err != nil {
			continue
		}

		deviation := computeDeviation(&vote, aggregated)
		if deviation > outlierThresholdBPS {
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"validator_slashed",
				sdk.NewAttribute("verifier", reveal.Verifier),
				sdk.NewAttribute("round_id", round.Id),
				sdk.NewAttribute("deviation", strconv.FormatUint(deviation, 10)),
			))
		} else {
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"validator_rewarded",
				sdk.NewAttribute("verifier", reveal.Verifier),
				sdk.NewAttribute("round_id", round.Id),
			))
		}
	}
}

// computeDeviation returns the maximum BPS deviation between vote and aggregated scores.
func computeDeviation(vote, aggregated *types.QualityVote) uint64 {
	dims := []struct{ v, a uint64 }{
		{vote.OverallQuality, aggregated.OverallQuality},
		{vote.ReasoningDepth, aggregated.ReasoningDepth},
		{vote.Novelty, aggregated.Novelty},
		{vote.Toxicity, aggregated.Toxicity},
		{vote.FactualAccuracy, aggregated.FactualAccuracy},
	}

	var maxDev uint64
	for _, d := range dims {
		var dev uint64
		if d.v > d.a {
			dev = d.v - d.a
		} else {
			dev = d.a - d.v
		}
		if dev > maxDev {
			maxDev = dev
		}
	}
	return maxDev
}
