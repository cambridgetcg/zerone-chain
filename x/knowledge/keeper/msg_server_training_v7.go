package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Route B Wave 7 msg handlers ─────────────────────────────────────────
//
// Three handlers, one lifecycle. Create → Finalize → Bind. Each step is
// idempotent on failure; each emits a structured event so the chain's
// history is fully reconstructible from the event log alone.

// CreateTrainingManifest materialises a DRAFT manifest from a selector.
// The handler applies the selector against current chain state, computes
// all five included-ID sets, and stamps every version pin. Merkle root
// is NOT yet locked — only Finalize commits the root.
func (m *msgServer) CreateTrainingManifest(ctx context.Context, msg *types.MsgCreateTrainingManifest) (*types.MsgCreateTrainingManifestResponse, error) {
	// Wave 12: circuit breaker. When knowledge is paused, write-path
	// handlers reject immediately. Read-path queries remain available.
	if err := m.keeper.RequireNotPaused(ctx, types.ModuleName); err != nil {
		return nil, err
	}
	if msg == nil || msg.Id == "" {
		return nil, fmt.Errorf("manifest id required")
	}
	if msg.Creator == "" {
		return nil, fmt.Errorf("creator required")
	}
	if msg.PipelineId == "" {
		return nil, fmt.Errorf("pipeline_id required")
	}
	if _, exists := m.keeper.GetTrainingManifest(ctx, msg.Id); exists {
		return nil, fmt.Errorf("manifest %s already exists", msg.Id)
	}
	pipeline, ok := m.keeper.GetTrainingPipeline(ctx, msg.PipelineId)
	if !ok {
		return nil, fmt.Errorf("pipeline %s not found", msg.PipelineId)
	}
	if pipeline.OperatorAddress != msg.Creator {
		return nil, fmt.Errorf("only the pipeline operator may create a manifest")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Stamp version pins.
	var tokenizerVersion, canonSerVersion, traceSchemaVersion uint64
	if spec, ok := m.keeper.GetTokenizerSpec(ctx); ok && spec != nil {
		tokenizerVersion = spec.Version
		canonSerVersion = spec.CanonicalSerialisationVersion
	}
	if sch, ok := m.keeper.GetTraceSchema(ctx); ok && sch != nil {
		traceSchemaVersion = sch.Version
	}

	// Apply selector.
	sel := msg.CorpusSelector
	if sel == nil {
		sel = &types.CorpusSelector{}
	}
	ids := m.keeper.SelectIncludedIds(ctx, sel)

	// Wave 8: composable manifest support. If a parent is declared, the
	// child carries only the delta — IDs present in the parent are
	// SUBTRACTED from the child's selected set, and the parent's
	// merkle_root is snapshotted into the child.
	var parentMerkleRoot string
	var compositionDepth uint32
	if msg.ParentManifestId != "" {
		parent, ok := m.keeper.GetTrainingManifest(ctx, msg.ParentManifestId)
		if !ok {
			return nil, fmt.Errorf("parent manifest %s not found", msg.ParentManifestId)
		}
		if parent.Status != types.ManifestStatus_MANIFEST_STATUS_FINALIZED &&
			parent.Status != types.ManifestStatus_MANIFEST_STATUS_ATTESTED {
			return nil, fmt.Errorf("parent manifest must be FINALIZED or ATTESTED; got %s", parent.Status)
		}
		// Cap composition depth to prevent DAG-bomb DoS. MaxDepth is the
		// hard cap; allowed child depths are 1..MaxDepth-1 (inclusive), so
		// the deepest admissible chain has MaxDepth levels (root at depth
		// 0 plus MaxDepth-1 children).
		const maxDepth uint32 = 8
		if parent.CompositionDepth+1 >= maxDepth {
			return nil, fmt.Errorf("composition chain would exceed max depth %d", maxDepth)
		}
		parentMerkleRoot = parent.MerkleRoot
		compositionDepth = parent.CompositionDepth + 1
		ids = subtractParentIds(ids, parent)
	}

	manifest := &types.TrainingManifest{
		ManifestId:                    msg.Id,
		PipelineId:                    msg.PipelineId,
		Creator:                       msg.Creator,
		CreatedAtBlock:                height,
		Description:                   msg.Description,
		TokenizerVersion:              tokenizerVersion,
		CanonicalSerialisationVersion: canonSerVersion,
		TraceSchemaVersion:            traceSchemaVersion,
		MethodologySetVersion:         pipeline.MethodologySetVersion,
		SnapshotBlockHeight:           height,
		ChainId:                       sdkCtx.ChainID(),
		CorpusSelector:                sel,

		IncludedFactIds:                ids.FactIDs,
		IncludedTraceIds:               ids.TraceIDs,
		IncludedPairIds:                ids.PairIDs,
		IncludedDriftAugmentationIds:   ids.DriftAugmentationIDs,
		IncludedNormativeCommitmentIds: ids.NormativeCommitmentIDs,

		FactCount:      uint32(len(ids.FactIDs)),
		TraceCount:     uint32(len(ids.TraceIDs)),
		PairCount:      uint32(len(ids.PairIDs)),
		DriftCount:     uint32(len(ids.DriftAugmentationIDs)),
		NormativeCount: uint32(len(ids.NormativeCommitmentIDs)),
		TotalIncluded:  ids.Total(),

		Status: types.ManifestStatus_MANIFEST_STATUS_DRAFT,

		// Wave 8: composition metadata.
		ParentManifestId:  msg.ParentManifestId,
		ParentMerkleRoot:  parentMerkleRoot,
		CompositionDepth:  compositionDepth,
	}
	if err := m.keeper.SetTrainingManifest(ctx, manifest); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.training_manifest_created",
		sdk.NewAttribute("manifest_id", manifest.ManifestId),
		sdk.NewAttribute("pipeline_id", manifest.PipelineId),
		sdk.NewAttribute("creator", manifest.Creator),
		sdk.NewAttribute("total_included", fmt.Sprintf("%d", manifest.TotalIncluded)),
		sdk.NewAttribute("tokenizer_version", fmt.Sprintf("%d", manifest.TokenizerVersion)),
		sdk.NewAttribute("trace_schema_version", fmt.Sprintf("%d", manifest.TraceSchemaVersion)),
	))

	return &types.MsgCreateTrainingManifestResponse{
		TotalIncluded:  manifest.TotalIncluded,
		FactCount:      manifest.FactCount,
		TraceCount:     manifest.TraceCount,
		PairCount:      manifest.PairCount,
		DriftCount:     manifest.DriftCount,
		NormativeCount: manifest.NormativeCount,
	}, nil
}

// FinalizeTrainingManifest computes and commits the Merkle root. After
// finalization the manifest is immutable.
func (m *msgServer) FinalizeTrainingManifest(ctx context.Context, msg *types.MsgFinalizeTrainingManifest) (*types.MsgFinalizeTrainingManifestResponse, error) {
	if msg == nil || msg.ManifestId == "" {
		return nil, fmt.Errorf("manifest_id required")
	}
	manifest, ok := m.keeper.GetTrainingManifest(ctx, msg.ManifestId)
	if !ok {
		return nil, fmt.Errorf("manifest %s not found", msg.ManifestId)
	}
	if manifest.Creator != msg.Creator {
		return nil, fmt.Errorf("only the manifest creator may finalize")
	}
	if manifest.Status != types.ManifestStatus_MANIFEST_STATUS_DRAFT {
		return nil, fmt.Errorf("manifest is not DRAFT; status=%s", manifest.Status)
	}

	ids := SelectedManifestIDs{
		FactIDs:                manifest.IncludedFactIds,
		TraceIDs:               manifest.IncludedTraceIds,
		PairIDs:                manifest.IncludedPairIds,
		DriftAugmentationIDs:   manifest.IncludedDriftAugmentationIds,
		NormativeCommitmentIDs: manifest.IncludedNormativeCommitmentIds,
	}
	var root string
	if manifest.ParentManifestId != "" && manifest.ParentMerkleRoot != "" {
		root = ComputeComposedManifestMerkleRoot(manifest.ParentMerkleRoot, ids)
	} else {
		root = ComputeManifestMerkleRoot(ids)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	manifest.MerkleRoot = root
	manifest.Status = types.ManifestStatus_MANIFEST_STATUS_FINALIZED
	manifest.FinalizedAtBlock = uint64(sdkCtx.BlockHeight())
	if err := m.keeper.SetTrainingManifest(ctx, manifest); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.training_manifest_finalized",
		sdk.NewAttribute("manifest_id", manifest.ManifestId),
		sdk.NewAttribute("merkle_root", manifest.MerkleRoot),
		sdk.NewAttribute("total_included", fmt.Sprintf("%d", manifest.TotalIncluded)),
	))

	return &types.MsgFinalizeTrainingManifestResponse{MerkleRoot: root}, nil
}

// BindManifestToAttestation links a FINALIZED manifest to an existing
// TrainingAttestation. The attestation is keyed by pipeline_id (one per
// pipeline) — we accept an attestation_id that must equal the manifest's
// pipeline_id to bind.
func (m *msgServer) BindManifestToAttestation(ctx context.Context, msg *types.MsgBindManifestToAttestation) (*types.MsgBindManifestToAttestationResponse, error) {
	if msg == nil || msg.ManifestId == "" || msg.AttestationId == "" {
		return nil, fmt.Errorf("manifest_id and attestation_id required")
	}
	manifest, ok := m.keeper.GetTrainingManifest(ctx, msg.ManifestId)
	if !ok {
		return nil, fmt.Errorf("manifest %s not found", msg.ManifestId)
	}
	if manifest.Creator != msg.Creator {
		return nil, fmt.Errorf("only the manifest creator may bind")
	}
	if manifest.Status != types.ManifestStatus_MANIFEST_STATUS_FINALIZED {
		return nil, fmt.Errorf("manifest must be FINALIZED before binding; status=%s", manifest.Status)
	}
	if msg.AttestationId != manifest.PipelineId {
		// TrainingAttestation is keyed by pipeline_id; attestation_id must
		// match. This invariant keeps the attestation↔manifest binding 1:1
		// per pipeline (one run = one attestation = one manifest).
		return nil, fmt.Errorf("attestation_id must equal manifest.pipeline_id (current binding model)")
	}
	att, ok := m.keeper.GetTrainingAttestation(ctx, msg.AttestationId)
	if !ok {
		return nil, fmt.Errorf("attestation %s not found", msg.AttestationId)
	}
	_ = att // reserved — future: record manifest_id on the attestation too

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	manifest.Status = types.ManifestStatus_MANIFEST_STATUS_ATTESTED
	manifest.AttestationId = msg.AttestationId
	manifest.AttestedAtBlock = uint64(sdkCtx.BlockHeight())
	if err := m.keeper.SetTrainingManifest(ctx, manifest); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.training_manifest_attested",
		sdk.NewAttribute("manifest_id", manifest.ManifestId),
		sdk.NewAttribute("attestation_id", manifest.AttestationId),
		sdk.NewAttribute("creator", manifest.Creator),
	))
	return &types.MsgBindManifestToAttestationResponse{}, nil
}
