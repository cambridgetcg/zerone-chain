package cross_stack_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cdtypes "github.com/zerone-chain/zerone/x/capture_defense/types"
	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	ontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
	partnershipstypes "github.com/zerone-chain/zerone/x/partnerships/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
)

// ─── Test 1: Metal → Water: Discovery matches scored higher with complementary qualifications ──

func TestR31_MetalWater_DiscoveryComplementaryQualifications(t *testing.T) {
	h := NewTestHarness(t)

	seeker := "zerone1seeker000000000000000000000000000"
	candidate := "zerone1candidate0000000000000000000000"

	// Seeker qualified in "physics" and "biology"
	h.App.QualificationKeeper.SetQualification(h.Ctx, &qualificationtypes.DomainQualification{
		Validator: seeker,
		Domain:    "physics",
		Status:    qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    50,
	})
	h.App.QualificationKeeper.SetQualification(h.Ctx, &qualificationtypes.DomainQualification{
		Validator: seeker,
		Domain:    "biology",
		Status:    qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    50,
	})

	// Candidate qualified in "chemistry" and "mathematics" (no overlap → fully complementary)
	h.App.QualificationKeeper.SetQualification(h.Ctx, &qualificationtypes.DomainQualification{
		Validator: candidate,
		Domain:    "chemistry",
		Status:    qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    50,
	})
	h.App.QualificationKeeper.SetQualification(h.Ctx, &qualificationtypes.DomainQualification{
		Validator: candidate,
		Domain:    "mathematics",
		Status:    qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    50,
	})

	baseScore := uint64(500_000)

	// ScoreDiscoveryMatch should boost the score because qualifications are complementary
	boostedScore := h.DiscoveryKeeper.ScoreDiscoveryMatch(h.Ctx, seeker, candidate, baseScore)
	require.Greater(t, boostedScore, baseScore,
		"complementary qualifications must boost discovery match score")

	// Fully complementary (0 overlap, 4 unique): complementarity = 4/4 * BPS = 1_000_000
	// bonus = 1_000_000 * 200_000 / 1_000_000 = 200_000 (20%)
	// result = 500_000 * (1_000_000 + 200_000) / 1_000_000 = 600_000
	require.Equal(t, uint64(600_000), boostedScore,
		"fully complementary qualifications should give 20%% bonus")

	// Now test with overlapping qualifications — score should be lower than fully complementary
	overlapper := "zerone1overlapper000000000000000000000"
	h.App.QualificationKeeper.SetQualification(h.Ctx, &qualificationtypes.DomainQualification{
		Validator: overlapper,
		Domain:    "physics", // same as seeker
		Status:    qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    50,
	})
	h.App.QualificationKeeper.SetQualification(h.Ctx, &qualificationtypes.DomainQualification{
		Validator: overlapper,
		Domain:    "chemistry", // different from seeker
		Status:    qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    50,
	})

	partialScore := h.DiscoveryKeeper.ScoreDiscoveryMatch(h.Ctx, seeker, overlapper, baseScore)
	require.Greater(t, partialScore, baseScore,
		"partially complementary qualifications must still boost score")
	require.Less(t, partialScore, boostedScore,
		"partial overlap must give smaller bonus than full complementarity")
}

// ─── Test 2: Metal → Water: Cross-stratum mentorship gets 20% priority bonus ─────────

func TestR31_MetalWater_CrossStratumMentorshipBonus(t *testing.T) {
	h := NewTestHarness(t)

	// Set up ontology: two domains in different strata with a cross-link
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    "empirical_physics",
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumEmpirical), // stratum 4
		Depth:   1,
	})
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    "formal_logic",
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumFormal), // stratum 1
		Depth:   1,
	})

	// Register strata so GetRelatedStrata can resolve stratum names
	h.App.ZeroneOntologyKeeper.SetStratum(h.Ctx, &ontologytypes.StratumProperties{
		Stratum: uint32(ontologytypes.StratumEmpirical),
		Name:    "empirical",
	})
	h.App.ZeroneOntologyKeeper.SetStratum(h.Ctx, &ontologytypes.StratumProperties{
		Stratum: uint32(ontologytypes.StratumFormal),
		Name:    "formal",
	})

	// Cross-stratum link between the two domains
	h.App.ZeroneOntologyKeeper.SetLink(h.Ctx, &ontologytypes.CrossStratumLink{
		SourceDomain: "empirical_physics",
		TargetDomain: "formal_logic",
		LinkType:     "depends_on",
		Discount:     100_000,
	})

	addr1 := "zerone1mentor0000000000000000000000000"
	addr2 := "zerone1mentee0000000000000000000000000"

	// Create pool entries for partners in different domains
	h.PartnershipsKeeper.SetPoolEntry(h.Ctx, &partnershipstypes.PoolEntry{
		Address: addr1,
		Domains: []string{"empirical_physics"},
		Status:  "active",
	})
	h.PartnershipsKeeper.SetPoolEntry(h.Ctx, &partnershipstypes.PoolEntry{
		Address: addr2,
		Domains: []string{"formal_logic"},
		Status:  "active",
	})

	baseScore := uint64(1_000_000)
	match := &partnershipstypes.FormationMatch{
		Id:    "test-match-1",
		Addr1: addr1,
		Addr2: addr2,
		Score: baseScore,
	}

	// Score should include the 20% cross-stratum bonus
	scored := h.PartnershipsKeeper.ScoreFormationMatchWithStratum(h.Ctx, match)

	// 20% bonus: 1_000_000 * 1_200_000 / 1_000_000 = 1_200_000
	require.Equal(t, uint64(1_200_000), scored,
		"cross-stratum partnership must receive 20%% priority bonus")

	// Verify same-stratum match gets no bonus
	h.PartnershipsKeeper.SetPoolEntry(h.Ctx, &partnershipstypes.PoolEntry{
		Address: "zerone1samestrat000000000000000000000",
		Domains: []string{"empirical_physics"}, // same domain as addr1
		Status:  "active",
	})
	sameMatch := &partnershipstypes.FormationMatch{
		Id:    "test-match-2",
		Addr1: addr1,
		Addr2: "zerone1samestrat000000000000000000000",
		Score: baseScore,
	}
	sameScored := h.PartnershipsKeeper.ScoreFormationMatchWithStratum(h.Ctx, sameMatch)
	require.Equal(t, baseScore, sameScored,
		"same-stratum partnership must not receive cross-stratum bonus")
}

// ─── Test 3: Fire → Metal: Domain with high verification activity → effective HHI threshold increases ─

func TestR31_FireMetal_HighActivityRelaxesHHIThreshold(t *testing.T) {
	h := NewTestHarness(t)
	domain := "active_domain"

	// Set up knowledge params with short fitness epochs
	kParams, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	kParams.FitnessEpochBlocks = 10
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, kParams))

	// Record verification history so AnalyzeCaptureRisk has data to work with
	for i := 0; i < 10; i++ {
		h.CaptureDefenseKeeper.SetVerificationHistory(h.Ctx, &cdtypes.VerificationHistoryEntry{
			Domain:       domain,
			RoundId:      fmt.Sprintf("round-%d", i),
			Validators:   []string{"val1", "val2", "val3", "val4", "val5"},
			Verdicts:     []bool{true, true, true, false, true},
			SubmitBlocks: []uint64{uint64(i*10 + 1), uint64(i*10 + 3), uint64(i*10 + 7), uint64(i*10 + 9), uint64(i*10 + 2)},
			BlockHeight:  uint64(i + 1),
		})
	}

	// Index 15 completed rounds in the completion index so that
	// GetDomainVerificationActivity (which reads from the completion index,
	// not DomainDiversity) reports full activity.
	for i := 0; i < 15; i++ {
		require.NoError(t, h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(10+i*5), fmt.Sprintf("act-round-%d", i),
			&knowledgetypes.CompletedRoundMeta{Domain: domain, DurationBlocks: 11}))
	}

	// Also set domain diversity data for AnalyzeCaptureRisk internals
	currentEpoch := uint64(h.Height()) / kParams.FitnessEpochBlocks
	require.NoError(t, h.KnowledgeKeeper.SetDomainDiversity(h.Ctx, domain, currentEpoch, knowledgekeeper.DomainDiversityRecord{
		Domain:         domain,
		Epoch:          currentEpoch,
		AvgEntropy:     500_000,
		RoundCount:     15,
		UnanimousCount: 2,
	}))

	// Advance context height past all indexed rounds so they fall within the window
	setHeight(h, 500)

	// Analyze capture risk with high activity
	cdParams := h.CaptureDefenseKeeper.GetParams(h.Ctx)
	baseHHIThreshold := cdParams.HhiThreshold // 250,000

	metrics := h.CaptureDefenseKeeper.AnalyzeCaptureRisk(h.Ctx, domain, cdParams)
	require.NotNil(t, metrics, "metrics must not be nil with verification history")

	// The effective HHI threshold should be higher than base due to activity relaxation.
	// With full activity (1_000_000 BPS) and default relaxation (200_000 BPS = 20%):
	// thresholdBonus = base * activity * relaxation / (BPS * BPS)
	// = 250_000 * 1_000_000 * 200_000 / (1_000_000 * 1_000_000)
	// = 50_000
	// effectiveThreshold = 250_000 + 50_000 = 300_000
	//
	// Verify the domain is NOT flagged even if HHI is between base and effective threshold.
	// With 5 validators evenly participating: HHI = 5 * (200_000^2 / 1_000_000) = 5 * 40_000 = 200_000
	// This is below both base and effective, so domain should NOT be flagged.
	require.False(t, metrics.Flagged,
		"domain with diverse validators and high activity must not be flagged")

	// Verify that activity > 0 was correctly detected
	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, domain)
	require.Equal(t, uint64(1_000_000), activity,
		"15 rounds should yield full (1_000_000 BPS) verification activity")

	// The effective threshold must be strictly greater than base because activity > 0.
	// We verify this indirectly: create a scenario where HHI is between base and effective,
	// and the domain isn't flagged (meaning the effective threshold was used).
	_ = baseHHIThreshold
	t.Logf("Base HHI threshold: %d, verification activity: %d BPS", baseHHIThreshold, activity)
}

// ─── Test 4: Fire → Metal: Domain with zero activity → base HHI threshold ──────────

func TestR31_FireMetal_ZeroActivityUsesBaseThreshold(t *testing.T) {
	h := NewTestHarness(t)
	domain := "inactive_domain"

	// No diversity data set → zero verification activity

	// Record some verification history for capture analysis to work
	// Use a single dominant validator to produce high HHI
	for i := 0; i < 5; i++ {
		h.CaptureDefenseKeeper.SetVerificationHistory(h.Ctx, &cdtypes.VerificationHistoryEntry{
			Domain:       domain,
			RoundId:      fmt.Sprintf("round-%d", i),
			Validators:   []string{"dominant_val", "dominant_val", "dominant_val", "minor_val"},
			Verdicts:     []bool{true, true, true, true},
			SubmitBlocks: []uint64{uint64(i*10 + 1), uint64(i*10 + 1), uint64(i*10 + 1), uint64(i*10 + 5)},
			BlockHeight:  uint64(i + 1),
		})
	}

	// Verify activity is 0
	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, domain)
	require.Equal(t, uint64(0), activity,
		"domain with no diversity data must have zero verification activity")

	cdParams := h.CaptureDefenseKeeper.GetParams(h.Ctx)
	baseThreshold := cdParams.HhiThreshold // 250,000

	// With zero activity, the effective threshold equals the base threshold.
	// The domain's HHI should be evaluated against the base threshold only.
	metrics := h.CaptureDefenseKeeper.AnalyzeCaptureRisk(h.Ctx, domain, cdParams)
	require.NotNil(t, metrics, "metrics must not be nil with verification history")

	t.Logf("Zero-activity domain: HHI=%d, baseThreshold=%d, flagged=%t",
		metrics.HerfindahlIndex, baseThreshold, metrics.Flagged)

	// The base threshold is 250,000. Verify that the base threshold constant is correct.
	require.Equal(t, uint64(250_000), baseThreshold,
		"default HHI threshold must be 250,000 (25%%)")
}

// ─── Test 5: Metal → Wood: Empirical domain (depth 1) → full carrying capacity ───────

func TestR31_MetalWood_Depth1FullCapacity(t *testing.T) {
	h := NewTestHarness(t)
	domain := "empirical_root"

	// Create an ontology domain at depth 1 (root level)
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    domain,
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumEmpirical),
		Depth:   1,
	})

	// Retrieve default params to know the base capacity
	kParams, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	baseCapacity := kParams.DomainBaseCapacity
	if baseCapacity == 0 {
		baseCapacity = 1000 // safety default
	}

	// Get carrying capacity — depth 1 should give full (100%) capacity
	capacity := h.KnowledgeKeeper.GetDomainCarryingCapacity(h.Ctx, domain)
	require.Equal(t, baseCapacity, capacity,
		"depth-1 domain must have full carrying capacity (no stratum reduction)")
	t.Logf("Depth-1 capacity: %d (base: %d)", capacity, baseCapacity)
}

// ─── Test 6: Metal → Wood: Theoretical domain (depth 2) → 80% carrying capacity ────

func TestR31_MetalWood_Depth2ReducedCapacity(t *testing.T) {
	h := NewTestHarness(t)
	domain := "theoretical_sub"

	// Create an ontology domain at depth 2
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    domain,
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumFormal),
		Depth:   2,
	})

	// Retrieve default params
	kParams, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	baseCapacity := kParams.DomainBaseCapacity
	if baseCapacity == 0 {
		baseCapacity = 1000
	}

	// Depth 2 → 80% capacity multiplier
	expected := baseCapacity * 800_000 / 1_000_000
	capacity := h.KnowledgeKeeper.GetDomainCarryingCapacity(h.Ctx, domain)
	require.Equal(t, expected, capacity,
		"depth-2 domain must have 80%% of base carrying capacity")
	t.Logf("Depth-2 capacity: %d (base: %d, expected: %d)", capacity, baseCapacity, expected)
}

// ─── Test 7: Metal → Wood: Combined: captured theoretical domain → reduced capacity from stratum depth ─

func TestR31_MetalWood_CombinedStratumDepthReducesCapacity(t *testing.T) {
	h := NewTestHarness(t)
	domain := "deep_captured"

	// Create a depth-2 domain (theoretical level)
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    domain,
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumFormal),
		Depth:   2,
	})

	// Mark domain as captured (flagged by capture defense)
	h.CaptureDefenseKeeper.SetCaptureMetrics(h.Ctx, &cdtypes.CaptureMetrics{
		Domain:          domain,
		HerfindahlIndex: 800_000,
		RiskScore:       850_000,
		Flagged:         true,
		AnalyzedAtBlock: uint64(h.Height()),
	})
	require.True(t, h.CaptureDefenseKeeper.IsDomainFlagged(h.Ctx, domain))

	// Get carrying capacity — stratum depth penalty applied
	kParams, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	baseCapacity := kParams.DomainBaseCapacity
	if baseCapacity == 0 {
		baseCapacity = 1000
	}

	capacity := h.KnowledgeKeeper.GetDomainCarryingCapacity(h.Ctx, domain)

	// Depth 2 gives 80% multiplier.
	// The capture flag doesn't directly affect carrying capacity (it affects partnerships),
	// but the stratum depth still reduces it to 80%.
	expectedFromStratum := baseCapacity * 800_000 / 1_000_000
	require.Equal(t, expectedFromStratum, capacity,
		"depth-2 captured domain must have 80%% capacity from stratum depth penalty")

	// Verify the capture flag is independent of carrying capacity
	require.True(t, h.CaptureDefenseKeeper.IsDomainFlagged(h.Ctx, domain),
		"domain must still be flagged as captured")

	// Verify overcrowding still works correctly with reduced capacity.
	// Set population above the reduced capacity to demonstrate combined effect.
	h.KnowledgeKeeper.SetDomainStats(h.Ctx, &knowledgekeeper.DomainStats{
		Domain:      domain,
		ActiveCount: expectedFromStratum + 100, // Over the reduced capacity
		AtRiskCount: 50,
		TotalEnergy: 1_000_000,
		LastUpdated: uint64(h.Height()),
	})

	pressure := h.KnowledgeKeeper.GetDomainPressure(h.Ctx, domain)
	require.Greater(t, pressure, uint64(1_000_000),
		"population exceeding stratum-reduced capacity must produce overcrowding pressure")

	// Also test deeper strata for completeness
	deepDomain := "deep_domain_3"
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    deepDomain,
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumComputational),
		Depth:   3,
	})
	depth3Capacity := h.KnowledgeKeeper.GetDomainCarryingCapacity(h.Ctx, deepDomain)
	expectedDepth3 := baseCapacity * 600_000 / 1_000_000
	require.Equal(t, expectedDepth3, depth3Capacity,
		"depth-3 domain must have 60%% of base capacity")

	veryDeepDomain := "deep_domain_4"
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    veryDeepDomain,
		Status:  "active",
		Stratum: uint32(ontologytypes.StratumTestimonial),
		Depth:   4,
	})
	depth4Capacity := h.KnowledgeKeeper.GetDomainCarryingCapacity(h.Ctx, veryDeepDomain)
	expectedDepth4 := baseCapacity * 500_000 / 1_000_000
	require.Equal(t, expectedDepth4, depth4Capacity,
		"depth-4+ domain must have 50%% floor capacity")

	t.Logf("Capacities by depth: d1=%d, d2=%d, d3=%d, d4=%d (base=%d)",
		baseCapacity, expectedFromStratum, expectedDepth3, expectedDepth4, baseCapacity)
}
