package types

import (
	"encoding/json"
	"fmt"
)

// DefaultParams returns the default module parameters for the training data protocol.
// All BPS values use a 1,000,000 scale (1,000,000 = 100%).
func DefaultParams() Params {
	return Params{
		// ─── Submission ──────────────────────────────────────────────────────
		MinSubmissionStake: "1000000",  // 1 ZRN in uzrn
		MaxContentBytes:    50_000,     // 50KB per submission
		MaxThreadSize:      20,         // Max submissions per thread batch

		// ─── Quality validation ──────────────────────────────────────────────
		CommitPeriodBlocks:    4,
		RevealPeriodBlocks:    4,
		MinValidatorsPerRound: 3,
		MaxValidatorsPerRound: 22,
		GoldThreshold:         800_000, // 80%
		SilverThreshold:       600_000, // 60%
		BronzeThreshold:       400_000, // 40%
		MaxToxicityThreshold:  200_000, // 20% — above this auto-rejects
		ConsentRequiredWeight: 200_000, // 20%

		// ─── Slashing ────────────────────────────────────────────────────────
		WrongValidationSlashBps: 50_000,  // 5%
		MissedRevealSlashBps:    100_000, // 10%
		EquivocationSlashBps:    200_000, // 20%

		// ─── Economics ───────────────────────────────────────────────────────
		SubmitterRevenueShareBps: "5500",  // 55%
		ValidatorRevenueShareBps: "2200",  // 22%
		GoldQualityMultiplier:    30_000,  // 3x (in BPS/10000)
		SilverQualityMultiplier:  20_000,  // 2x
		BronzeQualityMultiplier:  10_000,  // 1x
		AccessFeePerSample:       "100000", // 0.1 ZRN in uzrn

		// ─── Consent multipliers ─────────────────────────────────────────────
		SelfAuthoredMultiplier:  15_000, // 1.5x
		OptInMultiplier:         13_000, // 1.3x
		PublicLicenseMultiplier: 10_000, // 1.0x
		PlatformTosMultiplier:   8_000,  // 0.8x
		FairUseMultiplier:       5_000,  // 0.5x

		// ─── Ecology ─────────────────────────────────────────────────────────
		EnergyDecayRate:          50_000, // 5% per epoch
		EnergyPerAccess:          1_000,
		PruneGraceEpochs:        10,
		NicheSaturationThreshold: 50,
		NoveltyBonusBps:         100_000, // 10%

		// ─── Bounties ────────────────────────────────────────────────────────
		AutoBountyThreshold: 100,
		AutoBountyAmount:    "10000000", // 10 ZRN in uzrn

		// ─── Research fund ───────────────────────────────────────────────────
		ResearchTaxBps:       70_000, // 7%
		ResearchFundAddress:  "",
		FounderShareBps:      80_000, // 8%
		FounderAddress:       "",
		AiOperationsShareBps: 30_000, // 3%
		AiOperationsAddress: "",
	}
}

// DefaultGenesis returns the default genesis state with 9 training-data domains.
func DefaultGenesis() *GenesisState {
	p := DefaultParams()
	return &GenesisState{
		Params:            &p,
		Samples:           []*Sample{},
		Submissions:       []*Submission{},
		QualityRounds:     []*QualityRound{},
		Domains:           DefaultDomains(),
		Datasets:          []*Dataset{},
		Demands:           []*TrainingDemand{},
		Bounties:          []*DataBounty{},
		ScrapedSources:    []*ScrapedSourceEntry{},
		Validators:        []*ValidatorInfo{},
		NextSampleSeq:     1,
		NextSubmissionSeq: 1,
		NextRoundSeq:      1,
		NextDatasetSeq:    1,
	}
}

// DefaultDomains returns the 9 genesis training-data domains.
func DefaultDomains() []*Domain {
	names := []string{
		"technology",
		"science",
		"culture",
		"creative",
		"business",
		"education",
		"health",
		"politics",
		"general",
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

	// Verify unique sample IDs.
	seenSamples := make(map[string]bool)
	for _, s := range gs.Samples {
		if s == nil {
			continue
		}
		if seenSamples[s.Id] {
			return fmt.Errorf("duplicate sample ID: %s", s.Id)
		}
		seenSamples[s.Id] = true
	}

	// Verify unique submission IDs.
	seenSubmissions := make(map[string]bool)
	for _, s := range gs.Submissions {
		if s == nil {
			continue
		}
		if seenSubmissions[s.Id] {
			return fmt.Errorf("duplicate submission ID: %s", s.Id)
		}
		seenSubmissions[s.Id] = true
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

	return nil
}

// SeedSamples loads embedded genesis seeds and returns Sample objects.
// Called by prepare-genesis CLI, not by DefaultGenesis (which stays empty).
func SeedSamples() ([]*Sample, error) {
	var seeds []*Sample
	if err := json.Unmarshal(GenesisSeedsJSON, &seeds); err != nil {
		return nil, fmt.Errorf("failed to parse embedded seeds: %w", err)
	}
	if len(seeds) == 0 {
		return nil, fmt.Errorf("seed dataset is empty")
	}
	return seeds, nil
}

// Validate validates the Params struct.
func (p *Params) Validate() error {
	// ─── Submission ──────────────────────────────────────────────────────
	if p.MinSubmissionStake == "" || p.MinSubmissionStake == "0" {
		return fmt.Errorf("min_submission_stake must be > 0")
	}
	if p.MaxContentBytes == 0 {
		return fmt.Errorf("max_content_bytes must be > 0")
	}
	if p.MaxThreadSize == 0 {
		return fmt.Errorf("max_thread_size must be > 0")
	}

	// ─── Quality validation ──────────────────────────────────────────────
	if p.CommitPeriodBlocks == 0 {
		return fmt.Errorf("commit_period_blocks must be > 0")
	}
	if p.RevealPeriodBlocks == 0 {
		return fmt.Errorf("reveal_period_blocks must be > 0")
	}
	if p.MinValidatorsPerRound == 0 {
		return fmt.Errorf("min_validators_per_round must be > 0")
	}
	if p.MinValidatorsPerRound > p.MaxValidatorsPerRound {
		return fmt.Errorf("min_validators_per_round (%d) must be <= max_validators_per_round (%d)",
			p.MinValidatorsPerRound, p.MaxValidatorsPerRound)
	}

	// Thresholds must be ordered: gold > silver > bronze
	if p.GoldThreshold > 1_000_000 {
		return fmt.Errorf("gold_threshold must be <= 1,000,000")
	}
	if p.SilverThreshold > 1_000_000 {
		return fmt.Errorf("silver_threshold must be <= 1,000,000")
	}
	if p.BronzeThreshold > 1_000_000 {
		return fmt.Errorf("bronze_threshold must be <= 1,000,000")
	}
	if p.GoldThreshold <= p.SilverThreshold {
		return fmt.Errorf("gold_threshold (%d) must be > silver_threshold (%d)",
			p.GoldThreshold, p.SilverThreshold)
	}
	if p.SilverThreshold <= p.BronzeThreshold {
		return fmt.Errorf("silver_threshold (%d) must be > bronze_threshold (%d)",
			p.SilverThreshold, p.BronzeThreshold)
	}

	if p.MaxToxicityThreshold > 1_000_000 {
		return fmt.Errorf("max_toxicity_threshold must be <= 1,000,000")
	}
	if p.ConsentRequiredWeight > 1_000_000 {
		return fmt.Errorf("consent_required_weight must be <= 1,000,000")
	}

	// ─── Slashing — MUST be non-zero ─────────────────────────────────────
	if p.WrongValidationSlashBps == 0 {
		return fmt.Errorf("wrong_validation_slash_bps must be > 0")
	}
	if p.MissedRevealSlashBps == 0 {
		return fmt.Errorf("missed_reveal_slash_bps must be > 0")
	}
	if p.EquivocationSlashBps == 0 {
		return fmt.Errorf("equivocation_slash_bps must be > 0")
	}

	// ─── Economics ───────────────────────────────────────────────────────
	if p.GoldQualityMultiplier == 0 {
		return fmt.Errorf("gold_quality_multiplier must be > 0")
	}
	if p.SilverQualityMultiplier == 0 {
		return fmt.Errorf("silver_quality_multiplier must be > 0")
	}
	if p.BronzeQualityMultiplier == 0 {
		return fmt.Errorf("bronze_quality_multiplier must be > 0")
	}

	// ─── Consent multipliers — ordered by strength ───────────────────────
	if p.SelfAuthoredMultiplier < p.OptInMultiplier {
		return fmt.Errorf("self_authored_multiplier (%d) must be >= opt_in_multiplier (%d)",
			p.SelfAuthoredMultiplier, p.OptInMultiplier)
	}
	if p.OptInMultiplier < p.PublicLicenseMultiplier {
		return fmt.Errorf("opt_in_multiplier (%d) must be >= public_license_multiplier (%d)",
			p.OptInMultiplier, p.PublicLicenseMultiplier)
	}
	if p.PublicLicenseMultiplier < p.PlatformTosMultiplier {
		return fmt.Errorf("public_license_multiplier (%d) must be >= platform_tos_multiplier (%d)",
			p.PublicLicenseMultiplier, p.PlatformTosMultiplier)
	}
	if p.PlatformTosMultiplier < p.FairUseMultiplier {
		return fmt.Errorf("platform_tos_multiplier (%d) must be >= fair_use_multiplier (%d)",
			p.PlatformTosMultiplier, p.FairUseMultiplier)
	}

	// ─── Ecology ─────────────────────────────────────────────────────────
	if p.EnergyDecayRate > 1_000_000 {
		return fmt.Errorf("energy_decay_rate must be <= 1,000,000")
	}
	if p.NoveltyBonusBps > 1_000_000 {
		return fmt.Errorf("novelty_bonus_bps must be <= 1,000,000")
	}

	// ─── Bounties ────────────────────────────────────────────────────────
	if p.AutoBountyAmount != "" && p.AutoBountyAmount != "0" && p.AutoBountyThreshold == 0 {
		return fmt.Errorf("auto_bounty_threshold must be > 0 when auto_bounty_amount is set")
	}

	// ─── Research fund ───────────────────────────────────────────────────
	if p.ResearchTaxBps > 1_000_000 {
		return fmt.Errorf("research_tax_bps must be <= 1,000,000")
	}
	if p.FounderShareBps > 1_000_000 {
		return fmt.Errorf("founder_share_bps must be <= 1,000,000")
	}
	if p.AiOperationsShareBps > 1_000_000 {
		return fmt.Errorf("ai_operations_share_bps must be <= 1,000,000")
	}

	return nil
}
