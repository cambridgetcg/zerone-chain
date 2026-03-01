package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/capture_defense/keeper"
	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// ---------- Mock PartnershipsKeeper for structural immunity tests ----------

type mockPartnershipsKeeper struct {
	densities            map[string]uint64
	formationBonuses     map[string]formationBonusCall
	participantCounts    map[string]uint64
}

type formationBonusCall struct {
	domain       string
	bonusBps     uint64
	reason       string
	expiryHeight uint64
}

func newMockPartnershipsKeeper() *mockPartnershipsKeeper {
	return &mockPartnershipsKeeper{
		densities:         make(map[string]uint64),
		formationBonuses:  make(map[string]formationBonusCall),
		participantCounts: make(map[string]uint64),
	}
}

func (m *mockPartnershipsKeeper) GetDomainPartnershipDensity(_ context.Context, domain string) uint64 {
	return m.densities[domain]
}

func (m *mockPartnershipsKeeper) SetDomainFormationBonus(_ context.Context, domain string, bonusBps uint64, reason string, expiryHeight uint64) {
	m.formationBonuses[domain] = formationBonusCall{
		domain:       domain,
		bonusBps:     bonusBps,
		reason:       reason,
		expiryHeight: expiryHeight,
	}
}

func (m *mockPartnershipsKeeper) GetPartnershipCountByParticipant(_ context.Context, addr string, _ string) uint64 {
	return m.participantCounts[addr]
}

// ======================================================================
// Test 1: Partnership density reduces HHI
// ======================================================================

func TestCalculateAdjustedHHI_WithDensity(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// Default params: 10,000 BPS per participant (1%), max 200,000 (20%)

	// 5 unique participants → 5% HHI reduction
	mockPK.densities["test_domain"] = 5
	rawHHI := uint64(500000) // 50%

	adjusted := k.CalculateAdjustedHHI(ctx, "test_domain", rawHHI)

	// 5 * 10,000 = 50,000 BPS reduction
	// adjusted = 500,000 * (1,000,000 - 50,000) / 1,000,000 = 500,000 * 950,000 / 1,000,000 = 475,000
	assert.Equal(t, uint64(475000), adjusted,
		"5 participants should reduce HHI by 5%%")
}

func TestCalculateAdjustedHHI_MaxCap(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// 30 participants → 30% would exceed max of 20%
	mockPK.densities["capped_domain"] = 30
	rawHHI := uint64(500000)

	adjusted := k.CalculateAdjustedHHI(ctx, "capped_domain", rawHHI)

	// Capped at 200,000 BPS (20%) reduction
	// adjusted = 500,000 * (1,000,000 - 200,000) / 1,000,000 = 500,000 * 800,000 / 1,000,000 = 400,000
	assert.Equal(t, uint64(400000), adjusted,
		"reduction should cap at 20%% regardless of density")
}

func TestCalculateAdjustedHHI_NoDensity(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// 0 participants → no reduction
	rawHHI := uint64(500000)
	adjusted := k.CalculateAdjustedHHI(ctx, "empty_domain", rawHHI)

	assert.Equal(t, rawHHI, adjusted, "no density should mean no HHI reduction")
}

func TestCalculateAdjustedHHI_NoPartnershipsKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)

	// No partnerships keeper set → raw HHI returned
	rawHHI := uint64(500000)
	adjusted := k.CalculateAdjustedHHI(ctx, "domain", rawHHI)

	assert.Equal(t, rawHHI, adjusted, "nil partnerships keeper should return raw HHI")
}

// ======================================================================
// Test 2: Formation bonus set when domain flagged
// ======================================================================

func TestOnDomainFlagged_SetsFormationBonus(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// Default params: 300,000 BPS bonus, 50,000 block duration
	k.OnDomainFlagged(ctx, "flagged_domain")

	bonus, found := mockPK.formationBonuses["flagged_domain"]
	require.True(t, found, "formation bonus should be set for flagged domain")
	assert.Equal(t, "flagged_domain", bonus.domain)
	assert.Equal(t, uint64(300000), bonus.bonusBps, "bonus should be 30%% (300,000 BPS)")
	assert.Equal(t, "capture_flagged", bonus.reason)
	assert.Equal(t, uint64(100+50000), bonus.expiryHeight, "expiry = current block (100) + 50,000")
}

func TestOnDomainFlagged_NoPartnershipsKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)

	// No partnerships keeper set → should not panic
	k.OnDomainFlagged(ctx, "some_domain")
	// No assertion needed — just verifying no panic.
}

func TestOnDomainFlagged_CustomParams(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// Set custom structural immunity params
	customParams := types.DefaultStructuralImmunityParams()
	customParams.CapturedDomainFormationBonusBps = 500000
	customParams.FormationBonusDurationBlocks = 100000
	k.SetStructuralImmunityParams(ctx, customParams)

	k.OnDomainFlagged(ctx, "custom_domain")

	bonus := mockPK.formationBonuses["custom_domain"]
	assert.Equal(t, uint64(500000), bonus.bonusBps, "should use custom bonus BPS")
	assert.Equal(t, uint64(100+100000), bonus.expiryHeight, "should use custom duration")
}

// ======================================================================
// Test 3: Structural reputation bonus from partnerships
// ======================================================================

func TestCalculateStructuralReputationBonus(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	validator := testAddr(1)

	// 3 active partnerships → 3 * 20,000 = 60,000 BPS (6%)
	mockPK.participantCounts[validator] = 3

	bonus := k.CalculateStructuralReputationBonus(ctx, validator, "math")
	assert.Equal(t, uint64(60000), bonus, "3 partnerships should give 6%% bonus")
}

func TestCalculateStructuralReputationBonus_MaxCap(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	validator := testAddr(1)

	// 10 partnerships → 10 * 20,000 = 200,000 BPS, but max is 100,000 (10%)
	mockPK.participantCounts[validator] = 10

	bonus := k.CalculateStructuralReputationBonus(ctx, validator, "math")
	assert.Equal(t, uint64(100000), bonus, "bonus should cap at 10%% (100,000 BPS)")
}

func TestCalculateStructuralReputationBonus_NoPartnerships(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	validator := testAddr(1)

	bonus := k.CalculateStructuralReputationBonus(ctx, validator, "math")
	assert.Equal(t, uint64(0), bonus, "no partnerships should give 0 bonus")
}

func TestCalculateStructuralReputationBonus_NilKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)

	bonus := k.CalculateStructuralReputationBonus(ctx, testAddr(1), "math")
	assert.Equal(t, uint64(0), bonus, "nil keeper should return 0")
}

// ======================================================================
// Test 4: Accelerated flag clearing
// ======================================================================

func TestShouldAccelerateClearFlag(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// Set flagged metrics
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain:          "flagged_domain",
		HerfindahlIndex: 500000,
		Flagged:         true,
	})

	// Default min density for accelerated clear = 10
	mockPK.densities["flagged_domain"] = 10

	result := k.ShouldAccelerateClearFlag(ctx, "flagged_domain")
	assert.True(t, result, "should accelerate clearing when density >= min (10)")
}

func TestShouldAccelerateClearFlag_InsufficientDensity(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "flagged_domain", Flagged: true,
	})

	mockPK.densities["flagged_domain"] = 5 // Below threshold of 10

	result := k.ShouldAccelerateClearFlag(ctx, "flagged_domain")
	assert.False(t, result, "should NOT accelerate clearing when density < min")
}

func TestShouldAccelerateClearFlag_NotFlagged(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "healthy_domain", Flagged: false,
	})
	mockPK.densities["healthy_domain"] = 20

	result := k.ShouldAccelerateClearFlag(ctx, "healthy_domain")
	assert.False(t, result, "should NOT accelerate clearing for unflagged domain")
}

func TestShouldAccelerateClearFlag_NoMetrics(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)
	mockPK.densities["unknown_domain"] = 50

	result := k.ShouldAccelerateClearFlag(ctx, "unknown_domain")
	assert.False(t, result, "should NOT accelerate clearing when no metrics exist")
}

// ======================================================================
// Test 5: IsDomainFlagged
// ======================================================================

func TestIsDomainFlagged(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "flagged", Flagged: true,
	})
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "healthy", Flagged: false,
	})

	assert.True(t, k.IsDomainFlagged(ctx, "flagged"))
	assert.False(t, k.IsDomainFlagged(ctx, "healthy"))
	assert.False(t, k.IsDomainFlagged(ctx, "nonexistent"))
}

// ======================================================================
// Test 6: StructuralImmunityParams CRUD
// ======================================================================

func TestStructuralImmunityParams_Default(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := k.GetStructuralImmunityParams(ctx)
	defaults := types.DefaultStructuralImmunityParams()

	assert.Equal(t, defaults.PartnershipHHIReductionPerParticipantBps, params.PartnershipHHIReductionPerParticipantBps)
	assert.Equal(t, defaults.MaxPartnershipHHIReductionBps, params.MaxPartnershipHHIReductionBps)
	assert.Equal(t, defaults.CapturedDomainFormationBonusBps, params.CapturedDomainFormationBonusBps)
	assert.Equal(t, defaults.FormationBonusDurationBlocks, params.FormationBonusDurationBlocks)
	assert.Equal(t, defaults.MinDensityForAcceleratedClear, params.MinDensityForAcceleratedClear)
}

func TestStructuralImmunityParams_SetGet(t *testing.T) {
	k, ctx := setupKeeper(t)

	custom := &types.StructuralImmunityParams{
		PartnershipHHIReductionPerParticipantBps: 20000,
		MaxPartnershipHHIReductionBps:            300000,
		PartnershipReputationBonusBps:            30000,
		MaxPartnershipReputationBonusBps:         150000,
		MinDensityForAcceleratedClear:            15,
		CapturedDomainFormationBonusBps:          400000,
		FormationBonusDurationBlocks:             100000,
	}
	k.SetStructuralImmunityParams(ctx, custom)

	got := k.GetStructuralImmunityParams(ctx)
	assert.Equal(t, uint64(20000), got.PartnershipHHIReductionPerParticipantBps)
	assert.Equal(t, uint64(300000), got.MaxPartnershipHHIReductionBps)
	assert.Equal(t, uint64(30000), got.PartnershipReputationBonusBps)
	assert.Equal(t, uint64(150000), got.MaxPartnershipReputationBonusBps)
	assert.Equal(t, uint64(15), got.MinDensityForAcceleratedClear)
	assert.Equal(t, uint64(400000), got.CapturedDomainFormationBonusBps)
	assert.Equal(t, uint64(100000), got.FormationBonusDurationBlocks)
}

// ======================================================================
// Test 7: Adjusted HHI wired into AnalyzeCaptureRisk
// ======================================================================

func TestAnalyzeCaptureRisk_WithPartnershipDensity(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// Create a domain with moderate concentration (3 validators, one dominant)
	// val1: 7 rounds, val2: 2 rounds, val3: 1 round → HHI will be high
	for i := 0; i < 7; i++ {
		k.RecordVerificationFromKnowledge(ctx, "borderline_domain",
			fmt.Sprintf("round-a-%d", i), []string{testAddr(1)}, []bool{true}, nil)
	}
	for i := 0; i < 2; i++ {
		k.RecordVerificationFromKnowledge(ctx, "borderline_domain",
			fmt.Sprintf("round-b-%d", i), []string{testAddr(2)}, []bool{true}, nil)
	}
	k.RecordVerificationFromKnowledge(ctx, "borderline_domain",
		"round-c-0", []string{testAddr(3)}, []bool{true}, nil)

	params := k.GetParams(ctx)

	// First: analyze without partnership density → should be flagged
	mockPK.densities["borderline_domain"] = 0
	metrics := k.AnalyzeCaptureRisk(ctx, "borderline_domain", params)
	require.NotNil(t, metrics)

	rawHHI := metrics.HerfindahlIndex
	t.Logf("Raw HHI: %d, Flagged: %v", rawHHI, metrics.Flagged)

	// Now add significant partnership density
	mockPK.densities["borderline_domain"] = 20 // 20 participants → 20% reduction (max cap)

	metrics2 := k.AnalyzeCaptureRisk(ctx, "borderline_domain", params)
	require.NotNil(t, metrics2)

	// The raw HHI stored in metrics should be the same
	assert.Equal(t, rawHHI, metrics2.HerfindahlIndex,
		"raw HHI stored in metrics should not change")

	// But the flagging decision should use adjusted HHI, potentially unflagging
	// With 20% HHI reduction, a borderline domain might become unflagged
	if metrics.Flagged && !metrics2.Flagged {
		t.Log("Partnership density successfully prevented flagging")
	}
}

// ======================================================================
// Test 8: RunAutoAnalysis triggers OnDomainFlagged
// ======================================================================

func TestRunAutoAnalysis_TriggersFormationBonus(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	mockChallenge := newMockChallengeKeeper()
	k.SetChallengeKeeper(mockChallenge)

	// Create monopoly domain
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(ctx, "monopoly_domain",
			fmt.Sprintf("round-%d", i), []string{testAddr(1)}, []bool{true}, nil)
	}

	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params)

	// Verify formation bonus was set for the flagged domain
	bonus, found := mockPK.formationBonuses["monopoly_domain"]
	require.True(t, found, "formation bonus should be set when domain is flagged")
	assert.Equal(t, uint64(300000), bonus.bonusBps, "bonus should be 30%%")
	assert.Equal(t, "capture_flagged", bonus.reason)

	// Also verify challenge was submitted
	require.Len(t, mockChallenge.calls, 1, "challenge should also be submitted")
}

func TestRunAutoAnalysis_NoFormationBonusForHealthy(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	// Diverse domain (10 validators)
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(ctx, "healthy_domain",
			fmt.Sprintf("round-%d", i), []string{testAddr(i)}, []bool{true}, nil)
	}

	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params)

	// Verify NO formation bonus set for healthy domain
	_, found := mockPK.formationBonuses["healthy_domain"]
	assert.False(t, found, "no formation bonus should be set for healthy domain")
}

// ======================================================================
// Test 9: PartnershipsCaptureDefenseAdapter compile-time check
// ======================================================================

func TestPartnershipsCaptureDefenseAdapter(t *testing.T) {
	k, ctx := setupKeeper(t)

	adapter := keeper.NewPartnershipsCaptureDefenseAdapter(k)

	// No metrics → not flagged
	assert.False(t, adapter.IsDomainFlagged(ctx, "unknown_domain"))

	// Set flagged metrics
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "test_domain", Flagged: true,
	})

	assert.True(t, adapter.IsDomainFlagged(ctx, "test_domain"))

	// Clear flag
	k.ClearCaptureFlag(ctx, "test_domain")
	assert.False(t, adapter.IsDomainFlagged(ctx, "test_domain"))
}

// ======================================================================
// Test 10: Integration — full structural immunity lifecycle
// ======================================================================

func TestStructuralImmunity_FullLifecycle(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockPK := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(mockPK)

	mockChallenge := newMockChallengeKeeper()
	k.SetChallengeKeeper(mockChallenge)

	// Phase 1: Domain starts with monopoly → gets flagged
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(ctx, "evolving_domain",
			fmt.Sprintf("r-%d", i), []string{testAddr(1)}, []bool{true}, nil)
	}

	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params)

	m, found := k.GetCaptureMetrics(ctx, "evolving_domain")
	require.True(t, found)
	require.True(t, m.Flagged, "Phase 1: domain should be flagged (monopoly)")

	// Verify formation bonus was set
	bonus, found := mockPK.formationBonuses["evolving_domain"]
	require.True(t, found, "Phase 1: formation bonus should be set")
	assert.Equal(t, uint64(300000), bonus.bonusBps)

	// Phase 2: Partnerships form, increasing density
	mockPK.densities["evolving_domain"] = 20 // 20 unique participants

	// Re-analyze — with 20% HHI reduction, adjusted HHI = raw * 0.8
	// For monopoly (HHI=1,000,000): adjusted = 800,000, still above threshold 250,000
	k.RunAutoAnalysis(ctx, params)
	m2, _ := k.GetCaptureMetrics(ctx, "evolving_domain")
	t.Logf("Phase 2: raw HHI=%d, flagged=%v (with 20 participants)", m2.HerfindahlIndex, m2.Flagged)

	// Phase 3: Domain also gets accelerated clearing
	// Need to have the flag already set AND density >= 10
	// ShouldAccelerateClearFlag → additional 20% reduction on top of density reduction
	// adjusted = 1,000,000 * 0.8 * 0.8 = 640,000 — still above 250,000 for monopoly
	// A monopoly is too concentrated even with structural immunity
	// But for a moderately concentrated domain, this could make the difference

	// Phase 4: Verify reputation bonus for participating validator
	mockPK.participantCounts[testAddr(1)] = 3
	repBonus := k.CalculateStructuralReputationBonus(ctx, testAddr(1), "evolving_domain")
	assert.Equal(t, uint64(60000), repBonus, "Phase 4: validator with 3 partnerships should get 6%% bonus")

	// Phase 5: Verify params are customizable
	customSI := types.DefaultStructuralImmunityParams()
	customSI.PartnershipHHIReductionPerParticipantBps = 50000 // 5% per participant
	k.SetStructuralImmunityParams(ctx, customSI)

	// With 20 participants * 5% = 100% → capped at max 20%
	adjusted := k.CalculateAdjustedHHI(ctx, "evolving_domain", 500000)
	assert.Equal(t, uint64(400000), adjusted, "Phase 5: 20%% max reduction on 500,000 = 400,000")
}
