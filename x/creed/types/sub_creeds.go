package types

// SubCreedCommitment is one numbered commitment within a lifecycle
// phase's sub-creed. Number is the phase-local commitment index
// (1-based); Code is the doctrine's short identifier (e.g., "F1",
// "C3"); Name is the short label that must match the corresponding
// "## Code. <Name>" header in docs/sub_creeds/<phase>.md.
type SubCreedCommitment struct {
	Number uint32 // 1-based, dense, monotonic within a phase
	Code   string // doctrine identifier ("F1", "C2", "TL3", ...)
	Name   string // short label matching the markdown H2 header
}

// SubCreedDef is the canonical per-phase commitment list at the time
// this binary was built. Sub-creeds extend by appending new
// commitments via CategoryUsefulWorkAmendment LIPs; mechanism
// removal requires full doctrine amendment.
//
// At inception (2026-05-10), each phase ships exactly 3 commitments.
// Knowledge phase has zero commitments here — it delegates to
// CanonicalCommitments (truth-seeking).
type SubCreedDef struct {
	Phase       LifecyclePhase
	Commitments []SubCreedCommitment
}

// CanonicalSubCreeds is the registry. The order matches
// CanonicalLifecyclePhases. Sub-creed amendment writes a new entry to
// the on-chain x/work_creed PinnedSubCreed history (Phase 1+); this
// constant is the build-time inception baseline.
var CanonicalSubCreeds = []SubCreedDef{
	{
		Phase: LifecyclePhaseFoundation,
		Commitments: []SubCreedCommitment{
			{1, "F1", "Axiom non-contradiction"},
			{2, "F2", "Ontology versioned, never silently re-keyed"},
			{3, "F3", "Methodology primitives publicly derivable"},
		},
	},
	{
		Phase:       LifecyclePhaseKnowledge,
		Commitments: nil, // delegates to CanonicalCommitments (truth-seeking)
	},
	{
		Phase: LifecyclePhaseCuration,
		Commitments: []SubCreedCommitment{
			{1, "C1", "Selectors are deterministic and auditable"},
			{2, "C2", "No claim-of-curation without published filter"},
			{3, "C3", "Corpus snapshots are content-addressed"},
		},
	},
	{
		Phase: LifecyclePhaseAugmentation,
		Commitments: []SubCreedCommitment{
			{1, "A1", "Generation method is declared and reproducible"},
			{2, "A2", "Augmentation cannot inject untruth"},
			{3, "A3", "Contrastive pairs preserve grounding to a real fact"},
		},
	},
	{
		Phase: LifecyclePhaseTraining,
		Commitments: []SubCreedCommitment{
			{1, "T1", "Compute attestations are verifier-spot-checkable"},
			{2, "T2", "Training manifests are graph-pinned (TC2 binding)"},
			{3, "T3", "Model cards declare evaluation lineage"},
		},
	},
	{
		Phase: LifecyclePhaseEvaluation,
		Commitments: []SubCreedCommitment{
			{1, "E1", "Eval sets declare leakage-checking method"},
			{2, "E2", "Evaluation runs are replicable"},
			{3, "E3", "Gameability discovered → eval set status → DEPRECATED"},
		},
	},
	{
		Phase: LifecyclePhaseAlignment,
		Commitments: []SubCreedCommitment{
			{1, "AL1", "Red-team artifacts disclose attack surface"},
			{2, "AL2", "Capture-defense work cannot be self-attested by the captured target"},
			{3, "AL3", "Dispute traces preserve all positions"},
		},
	},
	{
		Phase: LifecyclePhaseSubstrate,
		Commitments: []SubCreedCommitment{
			{1, "S1", "Chain-modifying contributions name their `depends_on_marker` and revert path"},
			{2, "S2", "Contributors recuse on votes affecting their own contributions"},
			{3, "S3", "Reward-formula changes require simulation against historical contribution data"},
		},
	},
	{
		Phase: LifecyclePhaseTools,
		Commitments: []SubCreedCommitment{
			{1, "TL1", "Tools declare deprecation policy"},
			{2, "TL2", "Fee changes >X% require user-notice window"},
			{3, "TL3", "No tool may bypass the truth-floor on outputs it claims as verified"},
		},
	},
}

// SubCreedFor returns the canonical SubCreedDef for a given phase, or
// (zero, false) if not found.
func SubCreedFor(phase LifecyclePhase) (SubCreedDef, bool) {
	for _, sc := range CanonicalSubCreeds {
		if sc.Phase == phase {
			return sc, true
		}
	}
	return SubCreedDef{}, false
}
