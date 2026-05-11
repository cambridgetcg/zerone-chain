package knowledgeclaim

import (
	"context"

	contribtypes "github.com/zerone-chain/zerone/x/contribution/types"
)

// KnowledgeKeeperReader is the subset of x/knowledge.Keeper that the
// adapter needs. Defined locally to avoid importing the concrete keeper
// (avoids circular import / heavy dependency).
type KnowledgeKeeperReader interface {
	// GetClaimVerificationScore returns the PoT verification score
	// (in BPS, 0..1_000_000) for a claim by id, plus a found flag.
	// Implementation reads x/knowledge state.
	GetClaimVerificationScore(ctx context.Context, claimID string) (uint32, bool)
}

// CreedKeeperReader is the subset of x/creed.Keeper the adapter needs.
type CreedKeeperReader interface {
	// GetCurrentPinVersion returns the latest creed pin version
	// (per x/creed.PinnedCreed history). Used for truth-floor check.
	GetCurrentPinVersion(ctx context.Context) uint32
}

// Adapter implements contribtypes.ContributionAdapter for KNOWLEDGE_CLAIM.
type Adapter struct {
	knowledgeKeeper KnowledgeKeeperReader
	creedKeeper     CreedKeeperReader
}

// NewAdapter constructs a KNOWLEDGE_CLAIM adapter.
func NewAdapter(kk KnowledgeKeeperReader, ck CreedKeeperReader) Adapter {
	return Adapter{knowledgeKeeper: kk, creedKeeper: ck}
}

var _ contribtypes.ContributionAdapter = Adapter{}

// Class returns KNOWLEDGE_CLAIM.
func (a Adapter) Class() contribtypes.ContributionClass {
	return contribtypes.ContributionClass_KNOWLEDGE_CLAIM
}

// Classify validates payload shape, (class, phase) coherence,
// claims_about_self presence, and truth-floor freshness.
// Cites M3 in error returns.
func (a Adapter) Classify(ctx context.Context, c *contribtypes.Contribution) error {
	// (class, phase) coherence: KNOWLEDGE_CLAIM must declare PHASE_KNOWLEDGE.
	if c.Phase != contribtypes.LifecyclePhase_PHASE_KNOWLEDGE {
		return contribtypes.ErrInvalidClassPhase.Wrapf("KNOWLEDGE_CLAIM requires PHASE_KNOWLEDGE, got %s", c.Phase)
	}
	// Payload must be a KnowledgeClaim variant.
	if c.Payload == nil || c.Payload.GetKnowledge() == nil {
		return contribtypes.ErrPayloadMissing.Wrap("KNOWLEDGE_CLAIM payload missing")
	}
	// claims_about_self required (truth-seeking commitment 1).
	if len(c.ClaimsAboutSelf) == 0 {
		return contribtypes.ErrClaimsAboutSelfEmpty
	}
	// Truth-floor attestation must be present and reference current creed pin.
	if c.TruthFloorAttestation == nil {
		return contribtypes.ErrTruthFloorMissing
	}
	currentVersion := a.creedKeeper.GetCurrentPinVersion(ctx)
	if c.TruthFloorAttestation.CreedVersion != currentVersion {
		return contribtypes.ErrTruthFloorStale.Wrapf("attested=%d current=%d", c.TruthFloorAttestation.CreedVersion, currentVersion)
	}
	return nil
}

// SubstrateLink returns 10_000 (full link) when a tok_manifest_cid is
// present on the payload. Returns 0 when absent (M2 enforcement: zero
// link blocks the reward path). Phase 4 introduces graduated weights.
func (a Adapter) SubstrateLink(ctx context.Context, c *contribtypes.Contribution) (uint32, error) {
	kc := c.Payload.GetKnowledge()
	if kc == nil || kc.TokManifestCid == "" {
		return 0, contribtypes.ErrSubstrateLinkAbsent
	}
	return 10_000, nil
}

// Verify reads the existing PoT panel score from x/knowledge.
// Returns the score and any lookup error.
func (a Adapter) Verify(ctx context.Context, c *contribtypes.Contribution) (uint32, error) {
	kc := c.Payload.GetKnowledge()
	if kc == nil {
		return 0, contribtypes.ErrPayloadMissing
	}
	score, found := a.knowledgeKeeper.GetClaimVerificationScore(ctx, kc.ClaimId)
	if !found {
		return 0, contribtypes.ErrBackRefNotFound.Wrapf("claim_id=%s", kc.ClaimId)
	}
	return score, nil
}
