package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Route B Wave 3 msg handlers ───────────────────────────────────────────

// AmendTokenizerSpec (Route B Wave 3a) — authority-gated bump of the
// tokenizer contract. The submitted spec's Version field is ignored; the
// handler auto-assigns current+1 so version monotonicity is guaranteed.
func (m *msgServer) AmendTokenizerSpec(ctx context.Context, msg *types.MsgAmendTokenizerSpec) (*types.MsgAmendTokenizerSpecResponse, error) {
	if msg == nil || msg.Spec == nil {
		return nil, fmt.Errorf("tokenizer spec required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: only governance authority may amend tokenizer spec")
	}
	current, found := m.keeper.GetTokenizerSpec(ctx)
	var nextVersion uint64 = 1
	if found {
		nextVersion = current.Version + 1
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	newSpec := msg.Spec
	newSpec.Version = nextVersion
	newSpec.RatifiedAtBlock = uint64(sdkCtx.BlockHeight())
	if err := m.keeper.SetTokenizerSpec(ctx, newSpec); err != nil {
		return nil, err
	}
	m.keeper.RecordPrivilegedAction(ctx,
		types.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_SCHEMA_AMEND_TOKENIZER,
		msg.Authority, fmt.Sprintf("TokenizerSpec@v%d", newSpec.Version), "", "")
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.tokenizer_spec_amended",
		sdk.NewAttribute("new_version", fmt.Sprintf("%d", newSpec.Version)),
		sdk.NewAttribute("canonical_serialisation_version", fmt.Sprintf("%d", newSpec.CanonicalSerialisationVersion)),
		sdk.NewAttribute("authority", msg.Authority),
	))
	return &types.MsgAmendTokenizerSpecResponse{NewVersion: newSpec.Version}, nil
}

// AttributeContributions (Route B Wave 3b; Wave 4 realignment) — the model's
// owner posts the fact_ids consumed by training. Wave 4 adds:
//   - Is-ought wall: ids resolving to NormativeCommitments are REJECTED and
//     reported in rejected_commitment_count. Normative commitments must not
//     generate training revenue; they are exported via NormativeCorpus.
//   - Popper-weighted TVW: computed_tvw is the canonical revenue signal.
//     total_weight is retained for audit but no longer drives payouts.
//   - Per-fact calibration snapshot for audit trail.
func (m *msgServer) AttributeContributions(ctx context.Context, msg *types.MsgAttributeContributions) (*types.MsgAttributeContributionsResponse, error) {
	if msg == nil || msg.ModelId == "" {
		return nil, fmt.Errorf("model_id required")
	}
	card, ok := m.keeper.GetModelCard(ctx, msg.ModelId)
	if !ok {
		return nil, fmt.Errorf("model card %s not found", msg.ModelId)
	}
	if card.OwnerAddress != msg.Owner {
		return nil, fmt.Errorf("only the model owner may attribute contributions")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Is-ought wall: partition ids. Normative commitments are rejected.
	facts, rejected := m.keeper.FilterIsOughtIds(ctx, msg.FactIds)

	// Wave 3 total_weight (raw, for audit); Wave 4 computed TVW (Popper-weighted).
	var totalWeight uint64
	var computedTVW uint64
	perFactCal := make([]uint64, 0, len(facts))
	for _, f := range facts {
		if fact, ok := m.keeper.GetFact(ctx, f); ok {
			totalWeight += fact.CorroborationCount + 1
			perFactCal = append(perFactCal, fact.SubmitterCalibrationSnapshotBps)
		} else {
			totalWeight += 1
			perFactCal = append(perFactCal, 0)
		}
		tvw := m.keeper.ComputeTrainingValueWeight(ctx, f)
		computedTVW += tvw.Final
	}
	if msg.TotalWeight != 0 {
		totalWeight = msg.TotalWeight // raw override; computed_tvw is NOT overridable
	}

	record := &types.ContributionRecord{
		ModelId:                  msg.ModelId,
		FactIds:                  facts,
		AttributedBy:             msg.Owner,
		AttributedAtBlock:        height,
		TotalWeight:              totalWeight,
		ComputedTvw:              computedTVW,
		RejectedCommitmentCount:  uint32(len(rejected)),
		PerFactCalibrationBps:    perFactCal,
	}
	if err := m.keeper.SetContributionRecord(ctx, record); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.contributions_attributed",
		sdk.NewAttribute("model_id", msg.ModelId),
		sdk.NewAttribute("attributed_by", msg.Owner),
		sdk.NewAttribute("fact_count", fmt.Sprintf("%d", len(facts))),
		sdk.NewAttribute("total_weight", fmt.Sprintf("%d", totalWeight)),
		sdk.NewAttribute("computed_tvw", fmt.Sprintf("%d", computedTVW)),
		sdk.NewAttribute("rejected_commitments", fmt.Sprintf("%d", len(rejected))),
	))
	return &types.MsgAttributeContributionsResponse{Recorded: uint32(len(facts))}, nil
}

// AttestTraining (Route B Wave 3c) — pipeline operator posts a signed
// attestation of training completion (FLOPs, wallclock, eval hash).
func (m *msgServer) AttestTraining(ctx context.Context, msg *types.MsgAttestTraining) (*types.MsgAttestTrainingResponse, error) {
	if msg == nil || msg.PipelineId == "" {
		return nil, fmt.Errorf("pipeline_id required")
	}
	pipeline, ok := m.keeper.GetTrainingPipeline(ctx, msg.PipelineId)
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", msg.PipelineId)
	}
	if pipeline.OperatorAddress != msg.Attester {
		return nil, fmt.Errorf("only the pipeline operator may attest")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	attestation := &types.TrainingAttestation{
		PipelineId:       msg.PipelineId,
		AttesterAddress:  msg.Attester,
		FlopsEstimate:    msg.FlopsEstimate,
		WallclockSeconds: msg.WallclockSeconds,
		CompletedAtBlock: height,
		EvalHash:         msg.EvalHash,
		Signature:        msg.Signature,
		Notes:            msg.Notes,
	}
	if err := m.keeper.SetTrainingAttestation(ctx, attestation); err != nil {
		return nil, err
	}
	m.keeper.EmitTrainingAttestationEvent(ctx, attestation)
	return &types.MsgAttestTrainingResponse{}, nil
}

// CreateAugmentationBounty (Route B Wave 3e; Wave 4 realignment) — sponsor
// opens a bounty pool for variant formulations of a target fact. Wave 4:
// the sponsor LOCKS escrow (reward_per_variant × max_variants) into the
// KnowledgeTrainingFund at creation. Payouts are released only by verifier
// panel verdicts, never by the sponsor directly.
func (m *msgServer) CreateAugmentationBounty(ctx context.Context, msg *types.MsgCreateAugmentationBounty) (*types.MsgCreateAugmentationBountyResponse, error) {
	if msg == nil || msg.Id == "" {
		return nil, fmt.Errorf("bounty id required")
	}
	if msg.Sponsor == "" {
		return nil, fmt.Errorf("sponsor required")
	}
	if msg.TargetFactId == "" {
		return nil, fmt.Errorf("target_fact_id required")
	}
	targetFact, ok := m.keeper.GetFact(ctx, msg.TargetFactId)
	if !ok {
		return nil, fmt.Errorf("target fact %s not found", msg.TargetFactId)
	}
	if _, exists := m.keeper.GetAugmentationBounty(ctx, msg.Id); exists {
		return nil, fmt.Errorf("bounty %s already exists", msg.Id)
	}
	if msg.MaxVariants == 0 {
		return nil, fmt.Errorf("max_variants must be > 0")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Wave 4: escrow = reward_per_variant × max_variants × (1 + SUPERIOR bonus cap).
	// Simpler: lock base × max and charge the bonus against the reserve at
	// payout time. This keeps the basic case cheap for sponsors.
	escrowAmount := sdkmath.NewIntFromUint64(msg.RewardPerVariant).Mul(sdkmath.NewIntFromUint64(uint64(msg.MaxVariants)))
	// Pad for SUPERIOR bonus.
	params, _ := m.keeper.GetParams(ctx)
	bonusBps := params.ReformulationSuperiorBonusBps
	if bonusBps == 0 {
		bonusBps = 500_000
	}
	padding := escrowAmount.Mul(sdkmath.NewIntFromUint64(bonusBps)).Quo(sdkmath.NewIntFromUint64(1_000_000))
	escrowTotal := escrowAmount.Add(padding)

	if !escrowTotal.IsZero() {
		if err := m.keeper.EscrowAugmentationBounty(ctx, msg.Sponsor, msg.Id, escrowTotal); err != nil {
			return nil, fmt.Errorf("escrow failed: %w", err)
		}
	}

	bounty := &types.AugmentationBounty{
		Id:                msg.Id,
		SponsorAddress:    msg.Sponsor,
		TargetFactId:      msg.TargetFactId,
		RewardPerVariant:  msg.RewardPerVariant,
		MaxVariants:       msg.MaxVariants,
		AcceptedVariants:  0,
		CreatedAtBlock:    height,
		ExpiresAtBlock:    msg.ExpiresAtBlock,
		Active:            true,
		Description:       msg.Description,
		EscrowLocked:      escrowTotal.String(),
		MethodologyId:     targetFact.MethodId,
	}
	if err := m.keeper.SetAugmentationBounty(ctx, bounty); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.augmentation_bounty_created",
		sdk.NewAttribute("bounty_id", bounty.Id),
		sdk.NewAttribute("sponsor", bounty.SponsorAddress),
		sdk.NewAttribute("target_fact_id", bounty.TargetFactId),
		sdk.NewAttribute("reward_per_variant", fmt.Sprintf("%d", bounty.RewardPerVariant)),
		sdk.NewAttribute("max_variants", fmt.Sprintf("%d", bounty.MaxVariants)),
		sdk.NewAttribute("escrow_locked", bounty.EscrowLocked),
		sdk.NewAttribute("methodology_id", bounty.MethodologyId),
	))
	return &types.MsgCreateAugmentationBountyResponse{}, nil
}

// SubmitAugmentation — anyone may submit a variant. If bounty_id is set
// the bounty must be active and not yet saturated; if empty the variant
// is volunteer (no payment, but still queryable as training augmentation).
func (m *msgServer) SubmitAugmentation(ctx context.Context, msg *types.MsgSubmitAugmentation) (*types.MsgSubmitAugmentationResponse, error) {
	if msg == nil || msg.Id == "" {
		return nil, fmt.Errorf("augmentation id required")
	}
	if msg.OriginalFactId == "" {
		return nil, fmt.Errorf("original_fact_id required")
	}
	if _, ok := m.keeper.GetFact(ctx, msg.OriginalFactId); !ok {
		return nil, fmt.Errorf("original fact %s not found", msg.OriginalFactId)
	}
	if _, exists := m.keeper.GetAugmentation(ctx, msg.Id); exists {
		return nil, fmt.Errorf("augmentation %s already exists", msg.Id)
	}
	if msg.VariantContent == "" {
		return nil, fmt.Errorf("variant_content required")
	}

	if msg.BountyId != "" {
		bounty, ok := m.keeper.GetAugmentationBounty(ctx, msg.BountyId)
		if !ok {
			return nil, fmt.Errorf("bounty %s not found", msg.BountyId)
		}
		if !bounty.Active {
			return nil, fmt.Errorf("bounty %s is not active", msg.BountyId)
		}
		if bounty.AcceptedVariants >= bounty.MaxVariants {
			return nil, fmt.Errorf("bounty %s is saturated", msg.BountyId)
		}
		if bounty.TargetFactId != msg.OriginalFactId {
			return nil, fmt.Errorf("bounty target does not match original_fact_id")
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	aug := &types.Augmentation{
		Id:                    msg.Id,
		BountyId:              msg.BountyId,
		OriginalFactId:        msg.OriginalFactId,
		VariantContent:        msg.VariantContent,
		VariantReasoningTrace: msg.VariantReasoningTrace,
		Submitter:             msg.Submitter,
		CreatedAtBlock:        height,
		Accepted:              false,
	}
	if err := m.keeper.SetAugmentation(ctx, aug); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.augmentation_submitted",
		sdk.NewAttribute("augmentation_id", aug.Id),
		sdk.NewAttribute("original_fact_id", aug.OriginalFactId),
		sdk.NewAttribute("bounty_id", aug.BountyId),
		sdk.NewAttribute("submitter", aug.Submitter),
	))
	return &types.MsgSubmitAugmentationResponse{}, nil
}

// AcceptAugmentation (Route B Wave 3e; Wave 4 realignment) — RETAINED only
// for volunteer augmentations whose original-fact submitter has elected to
// accept, and as a legacy acceptor path where a passing verdict has already
// been reached. Sponsors CANNOT use this to self-judge under Wave 4; the
// only bounty path to acceptance is VoteOnAugmentation reaching consensus.
func (m *msgServer) AcceptAugmentation(ctx context.Context, msg *types.MsgAcceptAugmentation) (*types.MsgAcceptAugmentationResponse, error) {
	if msg == nil || msg.AugmentationId == "" {
		return nil, fmt.Errorf("augmentation_id required")
	}
	aug, ok := m.keeper.GetAugmentation(ctx, msg.AugmentationId)
	if !ok {
		return nil, fmt.Errorf("augmentation %s not found", msg.AugmentationId)
	}
	if aug.Accepted {
		return nil, fmt.Errorf("augmentation %s already accepted", msg.AugmentationId)
	}

	if aug.BountyId != "" {
		// Wave 4: sponsor can no longer self-accept. The only legal way to
		// mark accepted on a bounty is a finalized passing verdict. If the
		// verdict is already final and passing, this call is effectively a
		// no-op that emits the accept event for downstream consumers.
		passing := aug.Verdict == types.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT ||
			aug.Verdict == types.AugmentationVerdict_AUGMENTATION_VERDICT_SUPERIOR
		if !passing {
			return nil, fmt.Errorf("bounty augmentations require a verifier-panel passing verdict; see VoteOnAugmentation")
		}
		bounty, ok := m.keeper.GetAugmentationBounty(ctx, aug.BountyId)
		if !ok {
			return nil, fmt.Errorf("bounty %s vanished", aug.BountyId)
		}
		_ = bounty // defensive: verdict finalizer already processed bounty state
	} else {
		// Volunteer path UNCHANGED: original fact's submitter may accept.
		fact, ok := m.keeper.GetFact(ctx, aug.OriginalFactId)
		if !ok || fact.Submitter != msg.Acceptor {
			return nil, fmt.Errorf("only the original fact's submitter may accept a volunteer augmentation")
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	aug.Accepted = true
	aug.AcceptedAtBlock = uint64(sdkCtx.BlockHeight())
	aug.AcceptanceNote = msg.Note
	if err := m.keeper.SetAugmentation(ctx, aug); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.augmentation_accepted",
		sdk.NewAttribute("augmentation_id", aug.Id),
		sdk.NewAttribute("original_fact_id", aug.OriginalFactId),
		sdk.NewAttribute("bounty_id", aug.BountyId),
		sdk.NewAttribute("acceptor", msg.Acceptor),
	))
	return &types.MsgAcceptAugmentationResponse{}, nil
}
