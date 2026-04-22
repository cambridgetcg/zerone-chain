package types

// Methodology ID constants. Stable identifiers referenced on-chain by every
// claim and fact submitted under the "methodology over statement" model.
const (
	MethodologyFormal        = "M-FORMAL"
	MethodologyEmpirical     = "M-EMPIRICAL"
	MethodologyComputational = "M-COMPUTATIONAL"
	MethodologyTestimonial   = "M-TESTIMONIAL"
	MethodologyAnalogical    = "M-ANALOGICAL"
	MethodologyDialectical   = "M-DIALECTICAL"
	MethodologyLegacy        = "M-LEGACY" // transitional; sunsetted
)

const BPS uint64 = 1_000_000

// DefaultMethodologies returns the seven bootstrap methodologies seeded at
// genesis. Each names the rule of compliance, what evidence proves compliance,
// and what would falsify a claim made under it. The chain's "bedrock" under
// the methodology-over-statement model.
//
// Cross-method discounts are stated per pair: when a claim under method A
// cites evidence from method B, the cited contribution is capped at
// cross_method_discount_bps[B] of its effective confidence. Missing entries
// default to full strength (BPS).
func DefaultMethodologies() []*Methodology {
	return []*Methodology{
		{
			Id:          MethodologyFormal,
			Name:        "Formal derivation",
			Description: "Claim derives from stated premises via declared sound inference rules in a named formal system (e.g. ZFC, Peano, Euclidean). Each step must name a rule of that system.",
			ComplianceCriteria: []string{
				"The formal system is named explicitly",
				"Each inference step cites a rule of the stated system",
				"No gap in the derivation chain from premises to conclusion",
			},
			FalsificationPaths: []string{
				"An inference step fails to match any rule of the stated system",
				"A contradiction with a previously verified theorem in the same system",
				"An undeclared axiom is required for the derivation to close",
			},
			CrossMethodDiscountBps: map[string]uint64{
				// Formal citing formal: full. Formal citing empirical is a
				// "formal under empirical assumption" — capped at 90%.
				MethodologyEmpirical:     900_000,
				MethodologyComputational: 1_000_000, // algorithmic verification is strict
				MethodologyTestimonial:   500_000,   // testimony cannot prove a formal claim
				MethodologyAnalogical:    500_000,
			},
			MinQualificationWeight: 50,
			Version:                1,
		},
		{
			Id:          MethodologyEmpirical,
			Name:        "Empirical investigation",
			Description: "Hypothesis → prediction → experiment → independent replication. Claim is accepted when the experimental protocol has been replicated by at least N independent parties.",
			ComplianceCriteria: []string{
				"A hypothesis is stated with testable prediction",
				"Experimental protocol is published",
				"At least 3 independent replications with declared methodology",
				"Control conditions documented",
			},
			FalsificationPaths: []string{
				"The prediction fails under the stated protocol",
				"Replication fails under the stated protocol",
				"A confound is identified that invalidates the controls",
			},
			CrossMethodDiscountBps: map[string]uint64{
				MethodologyFormal:        1_000_000, // math as premise is fine
				MethodologyComputational: 1_000_000,
				MethodologyTestimonial:   700_000,
				MethodologyAnalogical:    400_000,
			},
			MinQualificationWeight: 50,
			Version:                1,
		},
		{
			Id:          MethodologyComputational,
			Name:        "Algorithmic verification",
			Description: "Claim is decidable by a stated deterministic program on stated inputs. The program and inputs are submitted; verification re-executes and checks the output.",
			ComplianceCriteria: []string{
				"Program submitted (with source or deterministic bytecode)",
				"Inputs are fully specified",
				"Output matches the stated claim under re-execution",
			},
			FalsificationPaths: []string{
				"Re-execution produces a different output",
				"The program contains a bug relevant to the claim",
				"The program is non-deterministic on the stated inputs",
			},
			CrossMethodDiscountBps: map[string]uint64{
				MethodologyFormal:    1_000_000,
				MethodologyEmpirical: 1_000_000,
			},
			MinQualificationWeight: 40,
			Version:                1,
		},
		{
			Id:          MethodologyTestimonial,
			Name:        "Multi-source corroboration",
			Description: "Claim is supported by at least N independent primary sources with documented lineage. Independence means the sources did not derive from each other.",
			ComplianceCriteria: []string{
				"At least 3 primary sources are cited",
				"Each source's lineage is documented (why it is primary)",
				"Independence statement: sources do not share a common origin",
			},
			FalsificationPaths: []string{
				"A primary source repudiates the claim",
				"Two or more sources are shown to share a common origin (not independent)",
				"A source's primary-status is contested and not defended",
			},
			CrossMethodDiscountBps: map[string]uint64{
				MethodologyFormal:        300_000, // testimony cannot ground math
				MethodologyEmpirical:     500_000,
				MethodologyComputational: 300_000,
				MethodologyAnalogical:    700_000,
			},
			MinQualificationWeight: 30,
			Version:                1,
		},
		{
			Id:          MethodologyAnalogical,
			Name:        "Structural analogy",
			Description: "Claim extends a relationship from one domain to another via an explicit structural mapping. The mapping's preserved invariants must be stated; counterexamples considered.",
			ComplianceCriteria: []string{
				"Both domains are stated explicitly",
				"The mapping between them is specified",
				"Preserved invariants are listed",
				"At least one counterexample-consideration is documented",
			},
			FalsificationPaths: []string{
				"A counterexample where the mapping's stated invariant fails",
				"An unacknowledged disanalogy that breaks the inference",
			},
			CrossMethodDiscountBps: map[string]uint64{
				MethodologyFormal:        300_000, // analogy cannot prove a formal claim
				MethodologyEmpirical:     500_000,
				MethodologyComputational: 300_000,
				MethodologyTestimonial:   600_000,
			},
			MinQualificationWeight: 30,
			Version:                1,
		},
		{
			Id:          MethodologyDialectical,
			Name:        "Challenge-survival",
			Description: "Claim earns dialectical robustness by surviving declared challenges from adversarial reviewers. Robustness accrues over time and distinct challenges.",
			ComplianceCriteria: []string{
				"Claim has survived at least N declared challenges (N set per domain)",
				"Each challenge and its rebuttal is logged on-chain",
				"No unrebutted challenge within the current survival window",
			},
			FalsificationPaths: []string{
				"An unrebutted challenge within the survival window",
				"A successful challenge that causes the claim to be marked DISPROVEN",
			},
			CrossMethodDiscountBps: map[string]uint64{
				// Dialectical is a meta-method that confers robustness on top of
				// another method. Cross-citation is discounted because pure
				// challenge-survival doesn't bear on formal or empirical truth
				// independently.
				MethodologyFormal:        500_000,
				MethodologyEmpirical:     700_000,
				MethodologyComputational: 500_000,
			},
			MinQualificationWeight: 40,
			Version:                1,
		},
		{
			Id:          MethodologyLegacy,
			Name:        "Legacy (transitional)",
			Description: "Pre-Phase-1 claims and claims that did not declare a methodology. Adjudicated under permissive rules. Sunsetted — after the sunset window, new claims cannot use this method.",
			ComplianceCriteria: []string{
				"Any evidence considered — transitional permissiveness",
			},
			FalsificationPaths: []string{
				"Standard challenge mechanism applies",
			},
			CrossMethodDiscountBps: map[string]uint64{
				// Legacy claims discount when cited by any method — don't let
				// pre-methodology facts prop up post-methodology claims at full
				// strength.
				MethodologyFormal:        700_000,
				MethodologyEmpirical:     700_000,
				MethodologyComputational: 700_000,
				MethodologyTestimonial:   800_000,
				MethodologyAnalogical:    800_000,
				MethodologyDialectical:   800_000,
			},
			MinQualificationWeight: 20,
			Version:                1,
			IsTransitional:         true,
		},
	}
}

// IsValidMethodologyID reports whether the given id names a bootstrap
// methodology. Governance-added methodologies are validated via the keeper's
// registry check; this function is a fast-path for the defaults.
func IsValidMethodologyID(id string) bool {
	switch id {
	case MethodologyFormal, MethodologyEmpirical, MethodologyComputational,
		MethodologyTestimonial, MethodologyAnalogical, MethodologyDialectical,
		MethodologyLegacy:
		return true
	}
	return false
}
