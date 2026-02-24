package types

import "fmt"

// DefaultParams returns the default module parameters.
// All slash params MUST be non-zero (B22-3 audit requirement).
func DefaultParams() Params {
	return Params{
		// ─── Core verification ────────────────────────────────────────────────
		MinVerifiers:          3,
		MaxVerifiers:          22,
		CommitPhaseBlocks:     200,
		RevealPhaseBlocks:     200,
		AggregationPhaseBlocks: 50,
		ClaimCooldownBlocks:   50,

		// ─── Confidence scoring ───────────────────────────────────────────────
		InitialConfidence:             500_000, // 50%
		ConfidenceBoostPerVerification: 50_000, // 5%
		ConfidenceThreshold:           770_000, // 77% (acceptance)
		QuorumThreshold:               660_000, // 66%

		// ─── Slashing — MUST be non-zero ─────────────────────────────────────
		WrongVerificationSlashBps: 50_000,  // 5%
		MissedRevealSlashBps:      100_000, // 10%
		EquivocationSlashBps:      200_000, // 20%
		InvalidClaimSlashBps:      0,       // DEPRECATED (R19-6): unused — review fee is non-refundable

		// ─── Rewards ─────────────────────────────────────────────────────────
		VerificationReward:          "3000000", // 3 ZRN in uzrn
		VerificationRewardDecayBps:  999_000,   // 0.999× per epoch

		// ─── Claim validation ─────────────────────────────────────────────────
		MinClaimTextLength: 20,
		MaxClaimTextLength: 1_000,
		MinReviewFee:       "100000", // 0.1 ZRN — non-refundable review fee

		// ─── Adversarial verification ─────────────────────────────────────────
		AdversarialVerificationEnabled: true,
		ProvisionalThreshold:           500_000, // 50%
		RejectThreshold:                300_000, // 30%
		ChallengeDurationBlocks:        34_272,  // 1 day
		MinChallengeStake:              "11000000", // 11 ZRN in uzrn
		FailedChallengeSlashBps:        220_000, // 22%
		SuccessfulChallengeRewardBps:   300_000, // 30%
		MaxConcurrentChallenges:        3,

		// ─── Citation economics ───────────────────────────────────────────────
		CitationShareBps:    150_000, // 15%
		CrossDomainBonusBps: 200_000, // 20%

		// ─── Extended governance params ───────────────────────────────────────
		MaxFactsPerDomain:           100_000,
		FactExpiryBlocks:            0,       // no expiry
		CrossStratumDiscountBps:     0,
		NoveltyBonusBps:             0,
		MaxValidatorsPerRound:       22,
		MaxCitationsPerClaim:        50,
		CitationDecayPerLevel:       500_000, // 50% per ancestor level
		SelfCitationDiscountBps:     500_000, // 50%
		ConfidenceGrowthEpoch:       1_111,   // blocks
		ConfidenceGrowthPerEpochBps: 11_000,  // 1.1%
		MaxSurvivalConfidence:       770_000, // 77%
		SurvivedChallengeConfidenceCap: 880_000, // 88%
		MaxApprenticeValidators:     111,     // Sybil cap

		// ─── FARM anti-gaming params ──────────────────────────────────────────
		ConformityThresholdBps:          950_000, // FARM-1
		CalibrationTrivialThreshold:     950_000, // FARM-2
		MisbehaviorRejectionThreshold:   300_000, // FARM-6
		MinDomainContributorsForNovelty: 3,        // FARM-7
		MinParticipationRateBps:         500_000, // FARM-8
		ChallengeStakeRatioMinBps:       500_000, // FARM-9

		// ─── Research fund ────────────────────────────────────────────────────
		ResearchFundShareBps: 130_000, // 13%

		// ─── Malformed claim slashing ────────────────────────────────────────
		MalformedClaimSlashBps: 500_000, // 50% — submitting garbage wastes verifier time

		// ─── Fitness scoring ─────────────────────────────────────────────────
		FitnessEpochBlocks:       10_000,  // ~7 hours at 2.5s blocks
		FitnessWeightQueryBps:    300_000, // 30% — agent usage is the primary signal
		FitnessWeightCitationBps: 250_000, // 25% — facts cited by other facts are foundational
		FitnessWeightBridgeBps:   100_000, // 10% — cross-domain facts are rare and valuable
		FitnessWeightDepthBps:    100_000, // 10% — facts with deep dependency trees are load-bearing
		FitnessWeightPatronBps:    50_000, // 5%  — someone willing to pay for this fact's survival
		FitnessWeightUniqueBps:   100_000, // 10% — non-redundant facts score higher
		FitnessWeightAgeBps:      100_000, // 10% — uncited old facts decay
		FitnessInitialScore:      500_000, // born healthy — 50%
		FitnessGraceEpochs:       10,      // ~3 days before age penalty kicks in

		// ─── Bootstrap fund (R19-7) ─────────────────────────────────────────
		BootstrapFundEnabled:        true,
		BootstrapFundMaxPerAddress:  "10",        // 10 sponsored claims per address lifetime
		BootstrapFundMaxPerEpoch:    "100",       // 100 sponsored claims per epoch across all users
		BootstrapFundEpochBlocks:    50_000,      // ~1.5 days at 2.5s blocks
		BootstrapFundFeeCap:         "5000000",   // Fund covers up to 5 ZRN per claim
	}
}

// DefaultGenesis returns the default genesis state with 18 active domains.
func DefaultGenesis() *GenesisState {
	p := DefaultParams()
	return &GenesisState{
		Params:                  &p,
		Facts:                   []*Fact{},
		PendingClaims:           []*Claim{},
		ActiveRounds:            []*VerificationRound{},
		Domains:                 DefaultDomains(),
		BootstrapFundAllocation: "22222000000", // 22,222 ZRN (0.01% of max supply)
	}
}

// DefaultDomains returns the 18 genesis epistemic domains.
func DefaultDomains() []*Domain {
	names := []string{
		"mathematics",
		"physics",
		"computer_science",
		"general",
		"theology",
		"philosophy",
		"logic",
		"chemistry",
		"biology",
		"economics",
		"linguistics",
		"psychology",
		"sociology",
		"cosmology",
		"information_theory",
		"ethics",
		"agent_rights",
		"agent_purpose",
	}

	domains := make([]*Domain, 0, len(names))
	for _, name := range names {
		domains = append(domains, &Domain{
			Name:   name,
			Status: DomainStatus_DOMAIN_STATUS_ACTIVE,
		})
	}
	return domains
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return fmt.Errorf("params must not be nil")
	}
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Verify unique fact IDs.
	seenFacts := make(map[string]bool)
	for _, f := range gs.Facts {
		if f == nil {
			continue
		}
		if seenFacts[f.Id] {
			return fmt.Errorf("duplicate fact ID: %s", f.Id)
		}
		seenFacts[f.Id] = true
	}

	// Verify unique claim IDs.
	seenClaims := make(map[string]bool)
	for _, c := range gs.PendingClaims {
		if c == nil {
			continue
		}
		if seenClaims[c.Id] {
			return fmt.Errorf("duplicate claim ID: %s", c.Id)
		}
		seenClaims[c.Id] = true
	}

	// Verify unique domain names.
	seenDomains := make(map[string]bool)
	for _, d := range gs.Domains {
		if d == nil {
			continue
		}
		if seenDomains[d.Name] {
			return fmt.Errorf("duplicate domain: %s", d.Name)
		}
		seenDomains[d.Name] = true
	}

	// Verify fact references point to existing facts.
	for _, f := range gs.Facts {
		if f == nil {
			continue
		}
		for _, ref := range f.References {
			if !seenFacts[ref] {
				return fmt.Errorf("fact %s references unknown fact %s", f.Id, ref)
			}
		}
	}

	return nil
}

// SeedAxiomFacts loads embedded genesis axioms and converts them to Facts.
// Called by prepare-genesis CLI, not by DefaultGenesis (which stays empty).
func SeedAxiomFacts() ([]*Fact, error) {
	axioms, err := ParseAxioms(GenesisAxiomsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded axioms: %w", err)
	}

	// Collect domain names for validation
	domainNames := make([]string, 0, len(DefaultDomains()))
	for _, d := range DefaultDomains() {
		domainNames = append(domainNames, d.Name)
	}
	// Add axiom-only domains not in DefaultDomains
	for _, n := range AxiomDomainNames() {
		found := false
		for _, dn := range domainNames {
			if dn == n {
				found = true
				break
			}
		}
		if !found {
			domainNames = append(domainNames, n)
		}
	}

	if err := ValidateAxioms(axioms, domainNames); err != nil {
		return nil, fmt.Errorf("axiom validation failed: %w", err)
	}

	return AxiomsToFacts(axioms), nil
}

// Validate validates the Params struct.
func (p *Params) Validate() error {
	// Slash params MUST be non-zero (B22-3 audit fix).
	if p.WrongVerificationSlashBps == 0 {
		return fmt.Errorf("wrong_verification_slash_bps must be > 0")
	}
	if p.MissedRevealSlashBps == 0 {
		return fmt.Errorf("missed_reveal_slash_bps must be > 0")
	}
	if p.EquivocationSlashBps == 0 {
		return fmt.Errorf("equivocation_slash_bps must be > 0")
	}
	// InvalidClaimSlashBps deprecated (R19-6): review fee is non-refundable, no stake to slash.
	if p.MalformedClaimSlashBps == 0 {
		return fmt.Errorf("malformed_claim_slash_bps must be > 0")
	}

	// Confidence values must be within BPS range.
	if p.InitialConfidence > 1_000_000 {
		return fmt.Errorf("initial_confidence must be <= 1,000,000")
	}
	if p.ConfidenceThreshold > 1_000_000 {
		return fmt.Errorf("confidence_threshold must be <= 1,000,000")
	}
	if p.QuorumThreshold > 1_000_000 {
		return fmt.Errorf("quorum_threshold must be <= 1,000,000")
	}

	// Cross-stratum discount must not exceed 100%.
	if p.CrossStratumDiscountBps > 1_000_000 {
		return fmt.Errorf("cross_stratum_discount_bps must be <= 1,000,000")
	}

	// Min verifiers must be at least 1.
	if p.MinVerifiers == 0 {
		return fmt.Errorf("min_verifiers must be > 0")
	}
	if p.MinVerifiers > p.MaxVerifiers {
		return fmt.Errorf("min_verifiers (%d) must be <= max_verifiers (%d)", p.MinVerifiers, p.MaxVerifiers)
	}

	// Text length limits.
	if p.MinClaimTextLength == 0 {
		return fmt.Errorf("min_claim_text_length must be > 0")
	}
	if p.MaxClaimTextLength < p.MinClaimTextLength {
		return fmt.Errorf("max_claim_text_length must be >= min_claim_text_length")
	}

	// Review fee must be positive.
	if p.MinReviewFee == "" || p.MinReviewFee == "0" {
		return fmt.Errorf("min_review_fee must be > 0")
	}

	// ─── Fitness params ──────────────────────────────────────────────────
	if p.FitnessEpochBlocks == 0 {
		return fmt.Errorf("fitness_epoch_blocks must be > 0")
	}
	// Weights are BPS — each must be <= 1,000,000.
	for _, w := range []struct {
		name string
		val  uint64
	}{
		{"fitness_weight_query_bps", p.FitnessWeightQueryBps},
		{"fitness_weight_citation_bps", p.FitnessWeightCitationBps},
		{"fitness_weight_bridge_bps", p.FitnessWeightBridgeBps},
		{"fitness_weight_depth_bps", p.FitnessWeightDepthBps},
		{"fitness_weight_patron_bps", p.FitnessWeightPatronBps},
		{"fitness_weight_unique_bps", p.FitnessWeightUniqueBps},
		{"fitness_weight_age_bps", p.FitnessWeightAgeBps},
	} {
		if w.val > 1_000_000 {
			return fmt.Errorf("%s must be <= 1,000,000", w.name)
		}
	}
	if p.FitnessInitialScore > 1_000_000 {
		return fmt.Errorf("fitness_initial_score must be <= 1,000,000")
	}

	// ─── Bootstrap fund params ──────────────────────────────────────────
	if p.BootstrapFundEnabled {
		if p.BootstrapFundEpochBlocks == 0 {
			return fmt.Errorf("bootstrap_fund_epoch_blocks must be > 0 when fund is enabled")
		}
		if p.BootstrapFundMaxPerAddress == "" || p.BootstrapFundMaxPerAddress == "0" {
			return fmt.Errorf("bootstrap_fund_max_per_address must be > 0 when fund is enabled")
		}
		if p.BootstrapFundMaxPerEpoch == "" || p.BootstrapFundMaxPerEpoch == "0" {
			return fmt.Errorf("bootstrap_fund_max_per_epoch must be > 0 when fund is enabled")
		}
		if p.BootstrapFundFeeCap == "" || p.BootstrapFundFeeCap == "0" {
			return fmt.Errorf("bootstrap_fund_fee_cap must be > 0 when fund is enabled")
		}
	}

	return nil
}
