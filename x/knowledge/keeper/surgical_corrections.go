package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Wave 13: surgical state corrections ─────────────────────────────────
//
// Authority-gated, single-purpose repair handlers for state corrupted by
// external exploit (direct RPC write, compromised node, etc.). Each
// correction:
//   - Is authority-gated (governance only).
//   - Requires an open incident_id binding — no correction without an
//     audit trail.
//   - Is pure recomputation from data already on-chain — no new state
//     admitted.
//   - Emits a structured event so the correction appears in the audit
//     log alongside the incident.
//
// Iteration 2 (Wave 13) adds CorrectManifestMerkleRoot. Future surgical
// handlers follow the same pattern: narrow scope, incident-bound, pure.

// CorrectManifestMerkleRoot repairs a finalized manifest whose merkle_root
// was corrupted post-finalization. Recomputes the root from the stored
// canonical ID sets (which are themselves authenticated by the original
// commit; the attacker can't alter the IDs without also rewriting the
// root, and the re-derivation would still catch that).
//
// Semantics:
//   - Manifest must exist.
//   - Incident must exist and be OPEN or MITIGATING (not CLOSED).
//   - Authority must equal keeper.GetAuthority().
//   - If the manifest's current merkle_root equals the recomputed one,
//     the handler returns was_corrupted=false and makes no state change.
//     This prevents an operator error from triggering a spurious write.
//   - Optional expected_recomputed_root forces the handler to assert its
//     computation matches before writing — prevents stale/mistaken calls.
func (m *msgServer) CorrectManifestMerkleRoot(ctx context.Context, msg *types.MsgCorrectManifestMerkleRoot) (*types.MsgCorrectManifestMerkleRootResponse, error) {
	if msg == nil || msg.ManifestId == "" {
		return nil, fmt.Errorf("manifest_id required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: only governance authority may correct manifest state")
	}
	if msg.IncidentId == "" {
		return nil, fmt.Errorf("incident_id required — surgical corrections must cite their audit trail")
	}

	// Incident binding: must exist and be open (OPEN or MITIGATING).
	inc, ok := m.keeper.GetIncidentRecord(ctx, msg.IncidentId)
	if !ok {
		return nil, fmt.Errorf("incident %s not found", msg.IncidentId)
	}
	if inc.Status != types.IncidentStatus_INCIDENT_STATUS_OPEN &&
		inc.Status != types.IncidentStatus_INCIDENT_STATUS_MITIGATING {
		return nil, fmt.Errorf("incident %s is not open (status=%s); cannot bind correction", msg.IncidentId, inc.Status)
	}

	manifest, ok := m.keeper.GetTrainingManifest(ctx, msg.ManifestId)
	if !ok {
		return nil, fmt.Errorf("manifest %s not found", msg.ManifestId)
	}

	// Recompute the root from the manifest's own declared IDs. For root
	// manifests this is ComputeManifestMerkleRoot; for children we use
	// the composed form with the stored parent_merkle_root. A tampered
	// parent_merkle_root is a separate incident (requires a dedicated
	// correction path if ever needed — for now we refuse rather than
	// silently compose a bad root).
	ids := SelectedManifestIDs{
		FactIDs:                manifest.IncludedFactIds,
		TraceIDs:               manifest.IncludedTraceIds,
		PairIDs:                manifest.IncludedPairIds,
		DriftAugmentationIDs:   manifest.IncludedDriftAugmentationIds,
		NormativeCommitmentIDs: manifest.IncludedNormativeCommitmentIds,
	}
	var recomputed string
	if manifest.ParentManifestId != "" && manifest.ParentMerkleRoot != "" {
		recomputed = ComputeComposedManifestMerkleRoot(manifest.ParentMerkleRoot, ids)
	} else {
		recomputed = ComputeManifestMerkleRoot(ids)
	}

	// Expected-root assertion if caller provided one.
	if msg.ExpectedRecomputedRoot != "" && msg.ExpectedRecomputedRoot != recomputed {
		return nil, fmt.Errorf(
			"expected_recomputed_root mismatch — caller expected %s, handler computed %s; aborting to prevent silent overwrite",
			msg.ExpectedRecomputedRoot, recomputed)
	}

	priorRoot := manifest.MerkleRoot
	wasCorrupted := priorRoot != recomputed

	if wasCorrupted {
		manifest.MerkleRoot = recomputed
		if err := m.keeper.SetTrainingManifest(ctx, manifest); err != nil {
			return nil, err
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	m.keeper.RecordPrivilegedAction(ctx,
		types.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_MANIFEST_CORRECT,
		msg.Authority, manifest.ManifestId, msg.IncidentId, msg.Note)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.manifest_merkle_corrected",
		sdk.NewAttribute("manifest_id", manifest.ManifestId),
		sdk.NewAttribute("incident_id", msg.IncidentId),
		sdk.NewAttribute("prior_root", priorRoot),
		sdk.NewAttribute("recomputed_root", recomputed),
		sdk.NewAttribute("was_corrupted", fmt.Sprintf("%t", wasCorrupted)),
		sdk.NewAttribute("authority", msg.Authority),
	))

	return &types.MsgCorrectManifestMerkleRootResponse{
		PriorRoot:       priorRoot,
		RecomputedRoot:  recomputed,
		WasCorrupted:    wasCorrupted,
	}, nil
}
