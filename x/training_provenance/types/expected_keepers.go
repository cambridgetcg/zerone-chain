package types

import (
	"context"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// KnowledgeKeeper is the read surface this module needs from
// x/knowledge to assemble a ProvenanceCertificate. We only need
// LOOKUPS: the manifest record, the facts it includes, and the
// privileged-action / incident logs touching its domains.
//
// All reads, no writes. The synthesizer never mutates upstream state.
type KnowledgeKeeper interface {
	GetTrainingManifest(ctx context.Context, id string) (*knowledgetypes.TrainingManifest, bool)
	GetFact(ctx context.Context, id string) (*knowledgetypes.Fact, bool)
	IteratePrivilegedActions(ctx context.Context, cb func(*knowledgetypes.PrivilegedAction) bool)
	IterateIncidents(ctx context.Context, cb func(*knowledgetypes.IncidentRecord) bool)
}

// QualificationKeeper is the read surface this module needs from
// x/qualification to compute per-domain coverage stats. We need to
// list ACTIVE qualifications by domain and read each one's
// (penalty-adjusted) effective weight.
type QualificationKeeper interface {
	GetQualifiedValidators(ctx context.Context, domain string) []string
	GetQualificationWeight(ctx context.Context, validator, domain string) uint32
}

// CaptureChallengeKeeper exposes the resolution log for cartel
// allegations. Provenance flags how many UPHELD resolutions touch
// any of the manifest's coverage domains — a high count is a yellow
// flag for a downstream model trainer.
type CaptureChallengeKeeper interface {
	IterateChallenges(ctx context.Context, cb func(challenge ChallengeView) bool)
}

// ChallengeView is the minimum we need from a capture_challenge record.
// Defined here so x/training_provenance does not import x/capture_challenge
// types directly — keeps the module-graph one-directional.
type ChallengeView struct {
	Id       string
	Domain   string
	Outcome  string // "upheld" / "rejected" / "partial" / "pending"
	Resolved bool
}
