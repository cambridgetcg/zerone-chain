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

	// Escrow reviewer stake.
	sub, subFound := k.GetSubmission(ctx, round.SubmissionId)
	if subFound && sub.Stake != "" {
		submitterStake, ok := sdkmath.NewIntFromString(sub.Stake)
		if ok && submitterStake.IsPositive() {
			if err := k.EscrowReviewerStake(ctx, msg.RoundId, msg.Verifier, submitterStake); err != nil {
				return err
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
// Votes are weighted by each reviewer's domain reputation. Reputation is
// updated for submitters and reviewers based on the aggregation outcome.
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

	// Get submission early for domain resolution and reputation wiring.
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	if !found {
		return types.ErrSubmissionNotFound
	}
	domainID := resolveDomain(sub.Domain)

	// Deserialize revealed votes with verifier tracking.
	votes := make([]*types.QualityVote, 0, len(round.Reveals))
	verifiers := make([]string, 0, len(round.Reveals))
	for _, reveal := range round.Reveals {
		var vote types.QualityVote
		if err := json.Unmarshal([]byte(reveal.Vote), &vote); err != nil {
			continue
		}
		votes = append(votes, &vote)
		verifiers = append(verifiers, reveal.Verifier)
	}

	if len(votes) == 0 {
		return nil
	}

	// Compute reputation-weighted vote weights.
	weights := k.computeReviewerWeights(ctx, verifiers, domainID)

	// Compute aggregated scores (reputation-weighted median per dimension).
	aggregated := &types.QualityVote{
		OverallQuality:  weightedMedianUint64(votes, weights, func(v *types.QualityVote) uint64 { return v.OverallQuality }),
		ReasoningDepth:  weightedMedianUint64(votes, weights, func(v *types.QualityVote) uint64 { return v.ReasoningDepth }),
		Novelty:         weightedMedianUint64(votes, weights, func(v *types.QualityVote) uint64 { return v.Novelty }),
		Toxicity:        weightedMedianUint64(votes, weights, func(v *types.QualityVote) uint64 { return v.Toxicity }),
		FactualAccuracy: weightedMedianUint64(votes, weights, func(v *types.QualityVote) uint64 { return v.FactualAccuracy }),
	}

	// Consent consensus: reputation-weighted majority vote.
	consentValid := weightedMajorityBool(votes, weights, func(v *types.QualityVote) bool { return v.ConsentValid })
	aggregated.ConsentValid = consentValid

	// Duplicate consensus: reputation-weighted majority vote.
	isDuplicate := weightedMajorityBool(votes, weights, func(v *types.QualityVote) bool { return v.Duplicate })
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

	// Handle verdict outcomes
	accepted := verdict == types.QualityVerdict_QUALITY_VERDICT_GOLD ||
		verdict == types.QualityVerdict_QUALITY_VERDICT_SILVER ||
		verdict == types.QualityVerdict_QUALITY_VERDICT_BRONZE

	if accepted {
		strength := ConsensusStrength(len(round.Reveals), len(round.SelectedVerifiers))
		if err := k.createSampleFromSubmission(ctx, sub, verdict, aggregated, strength); err != nil {
			return err
		}
		sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED
	} else {
		sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_REJECTED
		if verdict == types.QualityVerdict_QUALITY_VERDICT_CONSENT_FAIL {
			sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_CONSENT_FAILED
		}
	}

	// Distribute stakes via reviewer staking mechanism (atomic via CacheContext).
	cacheCtx, write := sdkCtx.CacheContext()
	if err := k.distributeReviewerStakes(cacheCtx, round, sub, params); err != nil {
		return err
	}
	write()

	if err := k.SetSubmission(ctx, sub); err != nil {
		return err
	}

	// ─── Reputation wiring ──────────────────────────────────────────────────
	currentHeight := sdkCtx.BlockHeight()
	repParams := k.GetReputationDecayParams(ctx)

	// Submitter gains reputation on accepted submissions, scaled by tier.
	if accepted {
		submitterGain := repParams.GetSubmitterGain()
		switch verdict {
		case types.QualityVerdict_QUALITY_VERDICT_GOLD:
			submitterGain = submitterGain.MulInt64(3)
		case types.QualityVerdict_QUALITY_VERDICT_SILVER:
			submitterGain = submitterGain.MulInt64(2)
		default:
			// Bronze: keep base gain
		}
		_ = k.UpdateReputation(ctx, sub.Submitter, domainID, submitterGain, currentHeight)
		k.ResetInactivityTimer(ctx, sub.Submitter, domainID, currentHeight)
	}

	// Classify reviewers and update reputation: majority gains, minority loses.
	sides := classifyVoters(round, params)
	for verifier, side := range sides {
		isMajority := (accepted && side == sideAccept) || (!accepted && side == sideReject)
		if isMajority {
			gain := repParams.GetReviewerGain()
			_ = k.UpdateReputation(ctx, verifier, domainID, gain, currentHeight)
			k.ResetInactivityTimer(ctx, verifier, domainID, currentHeight)
		} else {
			// Preserve old timer — bad reviews don't count as activity.
			oldHeight := int64(0)
			if oldRep, found := k.GetAgentDomainReputation(ctx, verifier, domainID); found {
				oldHeight = oldRep.LastActiveHeight
			}
			penalty := repParams.GetReviewerPenalty()
			_ = k.UpdateReputation(ctx, verifier, domainID, penalty.Neg(), oldHeight)
		}
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
	consensusStrength sdkmath.LegacyDec,
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

	// Initialize TDU fitness record
	stake := sdkmath.ZeroInt()
	if sub.Stake != "" {
		if s, ok := sdkmath.NewIntFromString(sub.Stake); ok {
			stake = s
		}
	}
	fitnessParams := k.GetFitnessDecayParams(ctx)
	currentCycle := uint64(sdkCtx.BlockHeight()) / fitnessParams.GetFitnessEpochBlocks()
	_ = k.InitializeFitnessRecord(ctx, sampleID, stake, currentCycle)

	// Send training influence signal based on consensus strength
	signal := types.FitnessSignal{
		TrainingInfluence: consensusStrength,
		UsageCorrelation:  sdkmath.LegacyZeroDec(),
		Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1), // 0.5 neutral
	}
	_ = k.UpdateFitnessScoreWithEvent(ctx, sampleID, signal, currentCycle)

	// If thread: create samples for all other thread submissions
	if sub.ThreadId != "" {
		return k.createThreadSamples(ctx, sub, verdict, scores, sampleID, consensusStrength)
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
	consensusStrength sdkmath.LegacyDec,
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

		// Initialize TDU fitness record for thread sibling
		siblingStake := sdkmath.ZeroInt()
		if sub.Stake != "" {
			if s, ok := sdkmath.NewIntFromString(sub.Stake); ok {
				siblingStake = s
			}
		}
		fitnessParams := k.GetFitnessDecayParams(ctx)
		currentCycle := uint64(sdkCtx.BlockHeight()) / fitnessParams.GetFitnessEpochBlocks()
		_ = k.InitializeFitnessRecord(ctx, sampleID, siblingStake, currentCycle)

		signal := types.FitnessSignal{
			TrainingInfluence: consensusStrength,
			UsageCorrelation:  sdkmath.LegacyZeroDec(),
			Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1),
		}
		_ = k.UpdateFitnessScoreWithEvent(ctx, sampleID, signal, currentCycle)

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

// ─── Reputation-Weighted Voting ──────────────────────────────────────────────

// reputationNormCap is the reputation score at which the weight bonus reaches maximum.
const reputationNormCap = 100

// resolveDomain returns the domain ID from submission metadata, defaulting to "general".
func resolveDomain(domain string) string {
	if domain == "" {
		return "general"
	}
	return domain
}

// computeVoteWeight returns the vote weight for a reviewer based on their domain reputation.
// weight = baseWeight + min(repScore/100, 1.0) * repMultiplier
// This caps the max weight at baseWeight + repMultiplier (e.g. 1.0 + 2.0 = 3.0x).
func computeVoteWeight(repScore, baseWeight, repMultiplier sdkmath.LegacyDec) sdkmath.LegacyDec {
	normCap := sdkmath.LegacyNewDec(reputationNormCap)
	normalized := repScore.Quo(normCap)
	if normalized.GT(sdkmath.LegacyOneDec()) {
		normalized = sdkmath.LegacyOneDec()
	}
	if normalized.IsNegative() {
		normalized = sdkmath.LegacyZeroDec()
	}
	return baseWeight.Add(normalized.Mul(repMultiplier))
}

// computeReviewerWeights returns vote weights for the given verifiers based on their
// domain reputation scores. Verifiers with no reputation get base_weight only.
func (k Keeper) computeReviewerWeights(ctx context.Context, verifiers []string, domainID string) []sdkmath.LegacyDec {
	repParams := k.GetReputationDecayParams(ctx)
	baseWeight := repParams.GetBaseVoteWeight()
	repMultiplier := repParams.GetReputationMultiplier()

	weights := make([]sdkmath.LegacyDec, len(verifiers))
	for i, v := range verifiers {
		rep, found := k.GetAgentDomainReputation(ctx, v, domainID)
		if !found {
			weights[i] = baseWeight
			continue
		}
		weights[i] = computeVoteWeight(rep.GetScore(), baseWeight, repMultiplier)
	}
	return weights
}

type weightedValue struct {
	value  uint64
	weight sdkmath.LegacyDec
}

// weightedMedianUint64 computes the weighted median of a uint64 field across votes.
func weightedMedianUint64(votes []*types.QualityVote, weights []sdkmath.LegacyDec, fn func(*types.QualityVote) uint64) uint64 {
	if len(votes) == 0 {
		return 0
	}

	pairs := make([]weightedValue, len(votes))
	totalWeight := sdkmath.LegacyZeroDec()
	for i, v := range votes {
		w := sdkmath.LegacyOneDec()
		if i < len(weights) {
			w = weights[i]
		}
		pairs[i] = weightedValue{value: fn(v), weight: w}
		totalWeight = totalWeight.Add(w)
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].value < pairs[j].value })

	halfWeight := totalWeight.Quo(sdkmath.LegacyNewDec(2))
	cumulative := sdkmath.LegacyZeroDec()
	for _, p := range pairs {
		cumulative = cumulative.Add(p.weight)
		if cumulative.GT(halfWeight) {
			return p.value
		}
	}

	return pairs[len(pairs)-1].value
}

// weightedMajorityBool returns true if the weighted sum of true votes exceeds half.
func weightedMajorityBool(votes []*types.QualityVote, weights []sdkmath.LegacyDec, fn func(*types.QualityVote) bool) bool {
	trueWeight := sdkmath.LegacyZeroDec()
	totalWeight := sdkmath.LegacyZeroDec()
	for i, v := range votes {
		w := sdkmath.LegacyOneDec()
		if i < len(weights) {
			w = weights[i]
		}
		totalWeight = totalWeight.Add(w)
		if fn(v) {
			trueWeight = trueWeight.Add(w)
		}
	}
	return trueWeight.GT(totalWeight.Quo(sdkmath.LegacyNewDec(2)))
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

// computeParticipationScores returns per-verifier quality scores in range [0, 1_000_000].
// Score = 1_000_000 - deviation; a verifier whose vote exactly matches the aggregate
// gets the maximum score, while one with maximum deviation gets zero.
// Verifiers whose vote cannot be parsed receive a score of zero.
// Returns an empty map if the round has no AggregateScores yet.
func computeParticipationScores(round *types.QualityRound) map[string]uint64 {
	scores := make(map[string]uint64, len(round.Reveals))
	if round.AggregateScores == nil {
		return scores
	}
	for _, reveal := range round.Reveals {
		var vote types.QualityVote
		if err := json.Unmarshal([]byte(reveal.Vote), &vote); err != nil {
			scores[reveal.Verifier] = 0
			continue
		}
		deviation := computeDeviation(&vote, round.AggregateScores)
		if deviation >= 1_000_000 {
			scores[reveal.Verifier] = 0
		} else {
			scores[reveal.Verifier] = 1_000_000 - deviation
		}
	}
	return scores
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
