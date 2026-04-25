package keeper

import (
	"context"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	"github.com/zerone-chain/zerone/x/training_provenance/types"
)

// BuildCertificate synthesizes a ProvenanceCertificate for the given
// manifest. Returns nil + error if the manifest doesn't exist.
//
// The synthesis is intentionally read-only and stateless: the cert is
// computed live every time a query lands. Re-querying at a later block
// can yield a different cert if more incidents / cartel resolutions /
// qualifications have landed in the meantime. The certificate is a
// current statement, not a stored artefact.
//
// Trust grade rubric (deterministic):
//
//	A: privileged_action_count == 0 AND incident_count == 0 AND cartel_resolutions == 0
//	B: cartel_resolutions == 0 AND incident_count == 0 AND privileged_action_count <= 2
//	C: cartel_resolutions == 0 AND (incident_count > 0 OR privileged_action_count > 2)
//	F: cartel_resolutions > 0
//
// A grade is a snapshot-current judgement. Downstream consumers can
// compute their own grade from the underlying counts; we publish the
// canonical one for convenience.
func (k Keeper) BuildCertificate(ctx context.Context, manifestID string) (*types.ProvenanceCertificate, error) {
	if k.knowledgeKeeper == nil {
		return nil, fmt.Errorf("knowledge keeper not wired")
	}
	manifest, ok := k.knowledgeKeeper.GetTrainingManifest(ctx, manifestID)
	if !ok || manifest == nil {
		return nil, fmt.Errorf("manifest %s not found", manifestID)
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// ── Domain coverage ────────────────────────────────────────────
	// Walk the manifest's IncludedFactIds, group by domain, count.
	domainFactCounts := make(map[string]uint64)
	for _, fid := range manifest.IncludedFactIds {
		fact, ok := k.knowledgeKeeper.GetFact(ctx, fid)
		if !ok || fact == nil || fact.Domain == "" {
			continue
		}
		domainFactCounts[fact.Domain]++
	}
	coveredDomains := make([]string, 0, len(domainFactCounts))
	for d := range domainFactCounts {
		coveredDomains = append(coveredDomains, d)
	}
	sort.Strings(coveredDomains) // deterministic output

	domains := make([]*types.DomainCoverage, 0, len(coveredDomains))
	for _, d := range coveredDomains {
		coverage := &types.DomainCoverage{
			Domain:    d,
			FactCount: domainFactCounts[d],
		}
		if k.qualificationKeeper != nil {
			validators := k.qualificationKeeper.GetQualifiedValidators(ctx, d)
			coverage.ActiveVoterCount = uint32(len(validators))
			var totalWeight uint64
			for _, v := range validators {
				totalWeight += uint64(k.qualificationKeeper.GetQualificationWeight(ctx, v, d))
			}
			if len(validators) > 0 {
				coverage.AvgQualifiedWeight = totalWeight / uint64(len(validators))
			}
		}
		domains = append(domains, coverage)
	}

	// ── Audit history ─────────────────────────────────────────────
	// Privileged actions whose target is one of the manifest's facts
	// or whose target/note touches a covered domain. We use a set of
	// covered fact IDs and a set of covered domains to filter the
	// privileged-action stream.
	domainSet := make(map[string]bool, len(coveredDomains))
	for _, d := range coveredDomains {
		domainSet[d] = true
	}
	factSet := make(map[string]bool, len(manifest.IncludedFactIds))
	for _, f := range manifest.IncludedFactIds {
		factSet[f] = true
	}
	var privilegedCount uint32
	k.knowledgeKeeper.IteratePrivilegedActions(ctx, func(p *knowledgetypes.PrivilegedAction) bool {
		if p == nil {
			return false
		}
		if factSet[p.Target] {
			privilegedCount++
		}
		return false
	})

	// Incidents: any incident whose AffectedModules includes
	// "knowledge" — coarse but defensible. Future refinement can
	// match by domain or pipeline_id when those fields are added to
	// the IncidentRecord proto.
	var incidentCount uint32
	k.knowledgeKeeper.IterateIncidents(ctx, func(r *knowledgetypes.IncidentRecord) bool {
		if r == nil {
			return false
		}
		for _, m := range r.AffectedModules {
			if m == "knowledge" {
				incidentCount++
				break
			}
		}
		return false
	})

	// Cartel resolutions: UPHELD challenges whose domain is in the
	// manifest's coverage set. Reads through the adapter which
	// translates capture_challenge's native types to ChallengeView.
	var cartelCount uint32
	if k.captureChallengeKeeper != nil {
		k.captureChallengeKeeper.IterateChallenges(ctx, func(c types.ChallengeView) bool {
			if c.Resolved && c.Outcome == "upheld" && domainSet[c.Domain] {
				cartelCount++
			}
			return false
		})
	}

	// ── Grade ────────────────────────────────────────────────────
	grade, explanation := computeTrustGrade(privilegedCount, incidentCount, cartelCount)

	return &types.ProvenanceCertificate{
		ManifestId:             manifest.ManifestId,
		PipelineId:             manifest.PipelineId,
		MerkleRoot:             manifest.MerkleRoot,
		FactCount:              uint64(len(manifest.IncludedFactIds)),
		FinalizedAtBlock:       manifest.FinalizedAtBlock,
		Status:                 manifest.Status.String(),
		Domains:                domains,
		PrivilegedActionCount:  privilegedCount,
		IncidentCount:          incidentCount,
		CartelResolutionCount:  cartelCount,
		TrustGrade:             grade,
		TrustExplanation:       explanation,
		ComputedAtBlock:        uint64(sdkCtx.BlockHeight()),
	}, nil
}

func computeTrustGrade(privileged, incidents, cartel uint32) (string, string) {
	if cartel > 0 {
		return "F", fmt.Sprintf("cartel_resolutions=%d in covered domains; this manifest's adjudication panel was demonstrably compromised", cartel)
	}
	if privileged == 0 && incidents == 0 {
		return "A", "no privileged actions touched the manifest's facts; no incidents touched the knowledge module; no cartel resolutions in covered domains"
	}
	if incidents == 0 && privileged <= 2 {
		return "B", fmt.Sprintf("privileged_action_count=%d affecting manifest facts; manifest is largely unintervened", privileged)
	}
	return "C", fmt.Sprintf("privileged_action_count=%d, incident_count=%d, cartel_resolution_count=%d — yellow flags accumulating; downstream consumer should review the audit trail before relying on this manifest", privileged, incidents, cartel)
}
