package keeper

// ─── Route B named constants ─────────────────────────────────────────────
//
// Consolidated magic numbers used across the Route B training infrastructure.
// The values live here so that amendments affect every call site at once,
// and so a reader can discover the full numeric contract in one place.
//
// Guardrail: ANY change to a value below is a consensus-breaking change.
// Route them through governance when the chain is live.

// NOTE: `BPS` exists in diversity.go; we reuse that spelling. The lowercase
// `bps` in training_economics.go is a local copy used only within that
// file. Both refer to 1_000_000.

// ─── Training-value weight (TVW, Wave 4b) ───────────────────────────────

// TVWAxiomDecayPerHopBps is the per-hop axiom-proximity decay used by
// ComputeTrainingValueWeight. 50,000 BPS = 5% decay per hop, linear,
// floored at 500,000 BPS (0.5×).
const TVWAxiomDecayPerHopBps uint64 = 50_000

// TVWAxiomProximityFloorBps is the floor for axiom-distance weighting —
// no fact earns less than 0.5× from depth alone.
const TVWAxiomProximityFloorBps uint64 = 500_000

// TVWNeutralCalibrationBps is the default calibration used for legacy facts
// whose submitter had no snapshot at submission (0.5×, neutral).
const TVWNeutralCalibrationBps uint64 = 500_000

// ─── Belief-revision chain (Wave 6.4) ───────────────────────────────────

// BeliefRevisionCorroborationStepBps is the per-corroboration upward nudge
// applied during belief-revision chain synthesis.
const BeliefRevisionCorroborationStepBps uint64 = 50_000

// BeliefRevisionContradictionStepBps is the upward nudge applied when a
// fact survives an incoming contradiction (the contradiction itself is a
// failed falsification attempt → Popperian survival).
const BeliefRevisionContradictionStepBps uint64 = 25_000

// BeliefRevisionInitialPriorBps is the neutral prior stamped on the
// initial-submission row of a belief-revision chain.
const BeliefRevisionInitialPriorBps uint64 = 500_000

// ─── Curriculum tiers (Wave 5, Wave 6.1) ─────────────────────────────────

// CurriculumFoundationAxiomMaxDistance is the maximum axiom_distance a
// fact may have to qualify for CURRICULUM_TIER_FOUNDATION.
const CurriculumFoundationAxiomMaxDistance uint32 = 1

// CurriculumFoundationMinCorroboration is the minimum corroboration count
// a fact must have earned to qualify for FOUNDATION tier.
const CurriculumFoundationMinCorroboration uint64 = 3

// CurriculumAdvancedMinAxiomDistance — at this depth or deeper, facts are
// ADVANCED regardless of corroboration (derivation chain is long).
const CurriculumAdvancedMinAxiomDistance uint32 = 5

// ─── Quality tiers (classification) ──────────────────────────────────────

// QualityGoldMinCorroboration — a non-legacy fact with this many survived
// falsifications qualifies for GOLD.
const QualityGoldMinCorroboration uint64 = 3

// QualitySilverMinCorroboration — minimum survived falsifications for SILVER.
const QualitySilverMinCorroboration uint64 = 1

// ─── Reformulation-round consensus (Wave 4d) ────────────────────────────

// ReformulationDefaultMinPanelVotes — fallback when Params.ReformulationMinPanelVotes
// is unset.
const ReformulationDefaultMinPanelVotes uint64 = 3

// ReformulationDefaultConsensusBps — fallback consensus threshold when the
// param is unset (66.6%).
const ReformulationDefaultConsensusBps uint64 = 666_000

// ReformulationDefaultSuperiorBonusBps — fallback SUPERIOR bonus fraction
// (50% on top of base reward_per_variant).
const ReformulationDefaultSuperiorBonusBps uint64 = 500_000

// ─── Training-fund disbursement (Wave 4f) ────────────────────────────────

// TrainingFundDefaultCalibrationFloorBps — minimum calibration score
// (50%) required for post-hoc disbursement when the param is unset.
const TrainingFundDefaultCalibrationFloorBps uint64 = 500_000

// TrainingFundDefaultVestingEpochs — vesting window fallback (60 epochs).
const TrainingFundDefaultVestingEpochs uint64 = 60

// TrainingFundDefaultFitnessEpochBlocks — fitness-epoch fallback.
const TrainingFundDefaultFitnessEpochBlocks uint64 = 1_111

// TrainingFundDefaultBaseRewardUzrn — base reward fallback (1000 ZRN in uzrn).
const TrainingFundDefaultBaseRewardUzrn = "1000000000"

// TrainingFundCalibrationScalingCeilingBps — upper bound on the linear
// calibration multiplier (2.0× at BPS=1.0 vs floor).
const TrainingFundCalibrationScalingCeilingBps uint64 = 2_000_000

// TrainingFundReleasedFraction — portion released immediately on claim
// (the remainder goes to vesting). Expressed as a divisor: 2 = 50/50 split.
const TrainingFundReleasedDivisor int64 = 2

// ─── Attribution challenge (Wave 4e) ────────────────────────────────────

// ContributionChallengeDefaultBondUzrn — fallback bond amount (5 ZRN in uzrn).
const ContributionChallengeDefaultBondUzrn = "5000000"

// ContributionChallengeDefaultRewardMultiplierBps — fallback multiplier
// (2× → winner takes bond × 2).
const ContributionChallengeDefaultRewardMultiplierBps uint64 = 2_000_000

// ─── Vindication premium (Wave 4b) ──────────────────────────────────────

// VindicationTVWMultiplierDefaultBps — fallback multiplier applied to TVW
// for facts vindicated from minority status (2.5×).
const VindicationTVWMultiplierDefaultBps uint64 = 2_500_000

// ─── Model lineage walk (Wave 3d) ───────────────────────────────────────

// ModelAncestryDefaultMaxDepth — fallback max-depth for WalkModelAncestry.
const ModelAncestryDefaultMaxDepth uint32 = 10

// ─── Methodology normalization defaults (Wave 4b) ──────────────────────
//
// These ship as seed defaults in genesis.go; they're duplicated here for
// visibility and for tests that want to assert the shape of the table.
// Amendments must go through MsgUpdateParams, not through this file.

// MethodologyNormalizationDefaults returns the seed table. Keys are
// methodology IDs in the Methodology registry; values are BPS multipliers.
func MethodologyNormalizationDefaults() map[string]uint64 {
	return map[string]uint64{
		"M-PHENOMENOLOGICAL": 2_000_000, // 2.0×
		"M-PRACTICE":         1_750_000, // 1.75×
		"M-ECOLOGICAL":       1_500_000, // 1.5×
		"M-TESTIMONIAL":      1_250_000, // 1.25×
		"M-LEGACY":           500_000,   // 0.5× — disincentivise legacy-method farming
	}
}

// ─── Route B version pins (Wave 7) ──────────────────────────────────────

// RouteBModuleVersion is the semantic version exposed by RouteBCapabilities.
// Bump when the Route B training surface gains or loses a capability.
const RouteBModuleVersion = "route_b.v7"

// AvailableCorpora enumerates the training corpora the current chain exposes.
// Order matters for determinism — appended at the end when new corpora land.
func AvailableCorpora() []string {
	return []string{
		"StructuredCorpus",
		"DisputationCorpus",
		"MethodCorpus",
		"DisprovenCorpus",
		"VindicationCorpus",
		"DriftCorpus",
		"NormativeCorpus",
		"MethodologyApplicationTrace",
		"ContrastivePair",
	}
}
