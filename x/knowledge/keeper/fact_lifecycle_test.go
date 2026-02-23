package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Status Transition Tests ─────────────────────────────────────────────────
//
// Since higher-level lifecycle methods (MarkFactChallenged, ResolveFactDispute,
// etc.) are not yet implemented, these tests verify that the data model
// correctly supports fact lifecycle transitions via SetFact/GetFact.
// Each test simulates the transition by mutating the Fact struct and
// persisting it, then verifying the stored state.

const bpsScale uint64 = 1_000_000

func TestFactStatus_VerifiedToChallengeable(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(1)

	fact := makeTestFact(t, k, ctx, "fact-v2c", "Verified fact", "physics", "empirical", submitter, 800_000)
	require.Equal(t, types.FactStatus_FACT_STATUS_VERIFIED, fact.Status)

	// Transition: verified -> challenged
	fact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-v2c")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_CHALLENGED, got.Status)
	require.Equal(t, uint64(800_000), got.Confidence, "confidence unchanged on challenge")
}

func TestFactStatus_ActiveToChallengeable(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(2)

	fact := makeTestFact(t, k, ctx, "fact-a2c", "Active fact", "mathematics", "formal", submitter, 950_000)
	fact.Status = types.FactStatus_FACT_STATUS_ACTIVE
	require.NoError(t, k.SetFact(ctx, fact))

	// Transition: active -> challenged
	fact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-a2c")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_CHALLENGED, got.Status)
	require.Equal(t, uint64(950_000), got.Confidence, "confidence unchanged on challenge")
}

func TestFactStatus_InvalidTransition(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(3)

	// Create a contested fact
	fact := makeTestFact(t, k, ctx, "fact-invalid", "Contested fact", "physics", "empirical", submitter, 600_000)
	fact.Status = types.FactStatus_FACT_STATUS_CONTESTED
	require.NoError(t, k.SetFact(ctx, fact))

	// Demonstrate the data layer permits the invalid transition (no guard yet).
	// When lifecycle guards are implemented, this should become an error case.
	fact.Status = types.FactStatus_FACT_STATUS_CHALLENGED
	require.NoError(t, k.SetFact(ctx, fact)) // succeeds -- no guard

	got, found := k.GetFact(ctx, "fact-invalid")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_CHALLENGED, got.Status,
		"data layer permits contested->challenged; lifecycle guard should reject this")
}

func TestFactStatus_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Attempting to challenge a nonexistent fact: GetFact returns not-found.
	_, found := k.GetFact(ctx, "nonexistent-fact-id")
	require.False(t, found, "challenging a nonexistent fact should fail at lookup")
}

// ─── Dispute Resolution Tests ────────────────────────────────────────────────
//
// These tests simulate dispute resolution outcomes by applying the specified
// formulas to the confidence field and persisting via SetFact.

func TestFactDispute_OverturnedReducesConfidence(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(4)

	oldConf := uint64(800_000)
	fact := makeTestFact(t, k, ctx, "fact-overturn", "Overturned fact", "physics", "empirical", submitter, oldConf)

	// Simulate overturned dispute: penalty formula newConf = oldConf * (1M - penalty) / 1M
	penalty := uint64(200_000) // 20% penalty
	newConf := oldConf * (bpsScale - penalty) / bpsScale
	require.Equal(t, uint64(640_000), newConf, "penalty formula: 800k * 800k / 1M = 640k")

	fact.Confidence = newConf
	fact.Status = types.FactStatus_FACT_STATUS_CONTESTED
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-overturn")
	require.True(t, found)
	require.Equal(t, uint64(640_000), got.Confidence)
	require.Equal(t, types.FactStatus_FACT_STATUS_CONTESTED, got.Status)
}

func TestFactDispute_OverturnedRevokesLowConfidence(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(5)

	// Start with confidence just above 100K threshold
	oldConf := uint64(120_000)
	fact := makeTestFact(t, k, ctx, "fact-revoke", "Low confidence fact", "physics", "empirical", submitter, oldConf)

	// Apply penalty that drops below 100K threshold
	penalty := uint64(250_000) // 25% penalty
	newConf := oldConf * (bpsScale - penalty) / bpsScale
	require.Equal(t, uint64(90_000), newConf, "120k * 750k / 1M = 90k")

	revokeThreshold := uint64(100_000)
	if newConf < revokeThreshold {
		fact.Status = types.FactStatus_FACT_STATUS_REVOKED
		fact.Confidence = newConf
	}
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-revoke")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_REVOKED, got.Status)
	require.Less(t, got.Confidence, revokeThreshold, "confidence below revocation threshold")
}

func TestFactDispute_UpheldBoostsConfidence(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(6)

	oldConf := uint64(700_000)
	fact := makeTestFact(t, k, ctx, "fact-upheld", "Upheld fact", "physics", "empirical", submitter, oldConf)

	// 11% boost: newConf = oldConf * 1_110_000 / 1M
	newConf := oldConf * 1_110_000 / bpsScale
	require.Equal(t, uint64(777_000), newConf, "700k * 1.11M / 1M = 777k")

	fact.Confidence = newConf
	fact.Status = types.FactStatus_FACT_STATUS_VERIFIED // restored after successful defense
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-upheld")
	require.True(t, found)
	require.Equal(t, uint64(777_000), got.Confidence)
	require.Equal(t, types.FactStatus_FACT_STATUS_VERIFIED, got.Status)
}

func TestFactDispute_UpheldCappedAt1M(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(7)

	oldConf := uint64(950_000) // close to max
	fact := makeTestFact(t, k, ctx, "fact-capped", "High confidence fact", "physics", "empirical", submitter, oldConf)

	// 11% boost would produce 1,054,500 — must cap at 1M
	newConf := oldConf * 1_110_000 / bpsScale
	require.Equal(t, uint64(1_054_500), newConf, "uncapped: 950k * 1.11M / 1M = 1,054,500")

	if newConf > bpsScale {
		newConf = bpsScale
	}
	require.Equal(t, bpsScale, newConf)

	fact.Confidence = newConf
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-capped")
	require.True(t, found)
	require.Equal(t, bpsScale, got.Confidence, "confidence must be capped at 1,000,000")
}

func TestFactDispute_InconclusiveNoChange(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(8)

	origConf := uint64(750_000)
	fact := makeTestFact(t, k, ctx, "fact-inconclusive", "Inconclusive dispute fact", "physics", "empirical", submitter, origConf)

	// Inconclusive: confidence unchanged, status transitions to contested
	fact.Status = types.FactStatus_FACT_STATUS_CONTESTED
	// Confidence stays the same
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-inconclusive")
	require.True(t, found)
	require.Equal(t, origConf, got.Confidence, "confidence unchanged on inconclusive dispute")
	require.Equal(t, types.FactStatus_FACT_STATUS_CONTESTED, got.Status)
}

func TestFactDispute_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Attempting to resolve a dispute on a nonexistent fact should fail at lookup
	_, found := k.GetFact(ctx, "dispute-nonexistent")
	require.False(t, found, "dispute on nonexistent fact should fail at lookup")
}

// ─── Cascade Tests ───────────────────────────────────────────────────────────

func TestFactCascade_MultipleReferrers(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(9)

	// Create a base fact that will be penalized
	baseFact := makeTestFact(t, k, ctx, "base-fact", "Base fact", "physics", "empirical", submitter, 800_000)

	// Create multiple facts that reference the base fact
	ref1 := makeTestFact(t, k, ctx, "ref-1", "References base", "physics", "derived", submitter, 700_000)
	ref1.References = []string{"base-fact"}
	require.NoError(t, k.SetFact(ctx, ref1))

	ref2 := makeTestFact(t, k, ctx, "ref-2", "Also references base", "physics", "derived", submitter, 600_000)
	ref2.References = []string{"base-fact"}
	require.NoError(t, k.SetFact(ctx, ref2))

	// Simulate base fact penalty and cascade
	penalty := uint64(300_000) // 30%
	baseFact.Confidence = baseFact.Confidence * (bpsScale - penalty) / bpsScale
	baseFact.Status = types.FactStatus_FACT_STATUS_CONTESTED
	require.NoError(t, k.SetFact(ctx, baseFact))

	// Cascade: apply proportional penalty to referencing facts
	// Find all facts that reference base-fact and apply penalty
	cascadePenalty := penalty / 2 // cascade at 50% of original penalty
	var affected []string

	k.IterateFacts(ctx, func(f *types.Fact) bool {
		for _, refID := range f.References {
			if refID == "base-fact" {
				f.Confidence = f.Confidence * (bpsScale - cascadePenalty) / bpsScale
				require.NoError(t, k.SetFact(ctx, f))
				affected = append(affected, f.Id)
				break
			}
		}
		return false
	})

	require.Len(t, affected, 2)
	require.Contains(t, affected, "ref-1")
	require.Contains(t, affected, "ref-2")

	// Verify cascaded confidence values
	got1, _ := k.GetFact(ctx, "ref-1")
	// ref-1: 700k * (1M - 150k) / 1M = 700k * 850k / 1M = 595k
	require.Equal(t, uint64(595_000), got1.Confidence)

	got2, _ := k.GetFact(ctx, "ref-2")
	// ref-2: 600k * (1M - 150k) / 1M = 600k * 850k / 1M = 510k
	require.Equal(t, uint64(510_000), got2.Confidence)
}

func TestFactCascade_SkipsRevoked(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(10)

	baseFact := makeTestFact(t, k, ctx, "base-skip", "Base for skip test", "physics", "empirical", submitter, 800_000)

	// Create a revoked referrer — should not be affected by cascade
	revokedFact := makeTestFact(t, k, ctx, "revoked-ref", "Already revoked", "physics", "derived", submitter, 50_000)
	revokedFact.Status = types.FactStatus_FACT_STATUS_REVOKED
	revokedFact.References = []string{"base-skip"}
	require.NoError(t, k.SetFact(ctx, revokedFact))

	// Also create a live referrer
	liveFact := makeTestFact(t, k, ctx, "live-ref", "Active referrer", "physics", "derived", submitter, 600_000)
	liveFact.References = []string{"base-skip"}
	require.NoError(t, k.SetFact(ctx, liveFact))

	// Penalize base
	penalty := uint64(200_000)
	baseFact.Confidence = baseFact.Confidence * (bpsScale - penalty) / bpsScale
	require.NoError(t, k.SetFact(ctx, baseFact))

	// Cascade — skip revoked facts
	cascadePenalty := penalty / 2
	var affectedCount int

	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.Status == types.FactStatus_FACT_STATUS_REVOKED {
			return false // skip revoked
		}
		for _, refID := range f.References {
			if refID == "base-skip" {
				f.Confidence = f.Confidence * (bpsScale - cascadePenalty) / bpsScale
				require.NoError(t, k.SetFact(ctx, f))
				affectedCount++
				break
			}
		}
		return false
	})

	require.Equal(t, 1, affectedCount, "only the live referrer should be affected")

	// Verify revoked fact is untouched
	gotRevoked, _ := k.GetFact(ctx, "revoked-ref")
	require.Equal(t, uint64(50_000), gotRevoked.Confidence, "revoked fact confidence unchanged")
	require.Equal(t, types.FactStatus_FACT_STATUS_REVOKED, gotRevoked.Status)
}

func TestFactCascade_NoReferences(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(11)

	// Create a fact with no inbound references
	makeTestFact(t, k, ctx, "isolated-fact", "No one references me", "physics", "empirical", submitter, 800_000)

	// Look for facts referencing "isolated-fact"
	var affected []string
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		for _, refID := range f.References {
			if refID == "isolated-fact" {
				affected = append(affected, f.Id)
				break
			}
		}
		return false
	})

	require.Empty(t, affected, "no inbound references means no cascade targets")
}

// ─── Confidence Decay Tests ──────────────────────────────────────────────────

func TestFactConfidenceDecay_OverTime(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(12)

	fact := makeTestFact(t, k, ctx, "fact-decay", "Decaying fact", "physics", "empirical", submitter, 900_000)

	// Simulate confidence decay without re-verification.
	// Decay formula: each period without re-verification reduces confidence.
	// Simple linear decay: 1% per period (10,000 BPS per period)
	decayPerPeriod := uint64(10_000)
	periods := uint64(5)

	expectedConf := fact.Confidence
	for i := uint64(0); i < periods; i++ {
		expectedConf -= decayPerPeriod
	}
	require.Equal(t, uint64(850_000), expectedConf)

	fact.Confidence = expectedConf
	ctx = advanceBlocks(ctx, int64(periods)*100) // advance many blocks
	fact.LastVerifiedBlock = uint64(ctx.BlockHeight())
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-decay")
	require.True(t, found)
	require.Equal(t, uint64(850_000), got.Confidence,
		"confidence should decay without re-verification")
}

// ─── Expiry Tests ────────────────────────────────────────────────────────────

func TestFactExpiry_PastExpiryBlock(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(13)

	fact := makeTestFact(t, k, ctx, "fact-expiry", "Expiring fact", "physics", "empirical", submitter, 700_000)
	fact.ReverificationBlock = uint64(ctx.BlockHeight()) + 50 // set expiry window
	require.NoError(t, k.SetFact(ctx, fact))

	// Advance past the expiry block
	ctx = advanceBlocks(ctx, 100)
	currentBlock := uint64(ctx.BlockHeight())

	// Check: fact is past reverification block
	got, found := k.GetFact(ctx, "fact-expiry")
	require.True(t, found)
	require.Greater(t, currentBlock, got.ReverificationBlock,
		"current block should be past reverification block")

	// Simulate EndBlocker expiry logic: mark as expired
	got.Status = types.FactStatus_FACT_STATUS_EXPIRED
	require.NoError(t, k.SetFact(ctx, got))

	expired, _ := k.GetFact(ctx, "fact-expiry")
	require.Equal(t, types.FactStatus_FACT_STATUS_EXPIRED, expired.Status)
}

// ─── Terminal State Tests ────────────────────────────────────────────────────

func TestFactStatus_AllTerminalStates(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(14)

	terminalStatuses := []types.FactStatus{
		types.FactStatus_FACT_STATUS_REVOKED,
		types.FactStatus_FACT_STATUS_SUPERSEDED,
		types.FactStatus_FACT_STATUS_EXPIRED,
	}

	for i, status := range terminalStatuses {
		id := makeFactID("terminal", i)
		fact := makeTestFact(t, k, ctx, id, "Terminal state fact", "physics", "empirical", submitter, 500_000)
		fact.Status = status
		require.NoError(t, k.SetFact(ctx, fact))

		got, found := k.GetFact(ctx, id)
		require.True(t, found)
		require.Equal(t, status, got.Status)

		// Verify terminal states are distinct from challengeable states.
		// A proper lifecycle guard would reject transitions FROM terminal states.
		require.NotEqual(t, types.FactStatus_FACT_STATUS_VERIFIED, got.Status)
		require.NotEqual(t, types.FactStatus_FACT_STATUS_ACTIVE, got.Status)
		require.NotEqual(t, types.FactStatus_FACT_STATUS_CHALLENGED, got.Status)
	}
}

// ─── Provisional to Verified ─────────────────────────────────────────────────

func TestFactStatus_ProvisionalToVerified(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(15)

	fact := &types.Fact{
		Id:               "fact-prov",
		Content:          "Provisional fact awaiting acceptance",
		Domain:           "mathematics",
		Category:         "formal",
		Submitter:        submitter,
		Confidence:       400_000, // initial low confidence
		SubmittedAtBlock: uint64(ctx.BlockHeight()),
		Status:           types.FactStatus_FACT_STATUS_PROVISIONAL,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "fact-prov")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_PROVISIONAL, got.Status)

	// Simulate acceptance: transition to verified with boosted confidence
	got.Status = types.FactStatus_FACT_STATUS_VERIFIED
	got.VerifiedAtBlock = uint64(ctx.BlockHeight()) + 10
	got.Confidence = 800_000 // confidence set by verification result
	require.NoError(t, k.SetFact(ctx, got))

	verified, _ := k.GetFact(ctx, "fact-prov")
	require.Equal(t, types.FactStatus_FACT_STATUS_VERIFIED, verified.Status)
	require.Equal(t, uint64(800_000), verified.Confidence)
	require.Greater(t, verified.VerifiedAtBlock, verified.SubmittedAtBlock)
}

// ─── Patronage Tests ─────────────────────────────────────────────────────────

func TestFactPatronage_ExpiryRespected(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(16)

	expiryBlock := uint64(ctx.BlockHeight()) + 200

	fact := &types.Fact{
		Id:                   "fact-patronage",
		Content:              "Patronized fact",
		Domain:               "general",
		Category:             "empirical",
		Submitter:            submitter,
		Confidence:           750_000,
		SubmittedAtBlock:     uint64(ctx.BlockHeight()),
		VerifiedAtBlock:      uint64(ctx.BlockHeight()),
		LastVerifiedBlock:    uint64(ctx.BlockHeight()),
		Status:               types.FactStatus_FACT_STATUS_VERIFIED,
		PatronageAmount:      "5000000",
		PatronageExpiryBlock: expiryBlock,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	// Before expiry: patronage is active
	got, found := k.GetFact(ctx, "fact-patronage")
	require.True(t, found)
	require.Equal(t, "5000000", got.PatronageAmount)
	require.Greater(t, got.PatronageExpiryBlock, uint64(ctx.BlockHeight()),
		"patronage should still be active before expiry block")

	// After expiry: simulate patronage check
	ctx = advanceBlocks(ctx, 300)
	currentBlock := uint64(ctx.BlockHeight())
	require.Greater(t, currentBlock, expiryBlock,
		"current block should be past patronage expiry")

	// A proper EndBlocker would clear patronage; we verify the data model supports it
	got.PatronageAmount = "0"
	require.NoError(t, k.SetFact(ctx, got))

	cleared, _ := k.GetFact(ctx, "fact-patronage")
	require.Equal(t, "0", cleared.PatronageAmount)
}

// ─── Reference Tracking Tests ────────────────────────────────────────────────

func TestFactReferences_InboundIndex(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(17)

	// Create target fact
	makeTestFact(t, k, ctx, "target-fact", "Referenced target", "physics", "empirical", submitter, 900_000)

	// Create facts that reference the target
	for i := 0; i < 3; i++ {
		id := makeFactID("inbound-ref", i)
		f := makeTestFact(t, k, ctx, id, "References target", "physics", "derived", submitter, 700_000)
		f.References = []string{"target-fact"}
		require.NoError(t, k.SetFact(ctx, f))
	}

	// Build inbound index by iterating all facts
	inboundRefs := make(map[string][]string) // targetID -> []referrerIDs
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		for _, refID := range f.References {
			inboundRefs[refID] = append(inboundRefs[refID], f.Id)
		}
		return false
	})

	require.Len(t, inboundRefs["target-fact"], 3, "target should have 3 inbound references")
}

func TestFactReferences_BidirectionalLookup(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	submitter := makeSubmitterAddr(18)

	// Create a reference graph: A -> B, A -> C, B -> C
	makeTestFact(t, k, ctx, "fact-A", "Fact A", "physics", "empirical", submitter, 800_000)
	makeTestFact(t, k, ctx, "fact-B", "Fact B", "physics", "derived", submitter, 700_000)
	makeTestFact(t, k, ctx, "fact-C", "Fact C", "physics", "derived", submitter, 600_000)

	factA, _ := k.GetFact(ctx, "fact-A")
	factA.References = []string{"fact-B", "fact-C"}
	require.NoError(t, k.SetFact(ctx, factA))

	factB, _ := k.GetFact(ctx, "fact-B")
	factB.References = []string{"fact-C"}
	require.NoError(t, k.SetFact(ctx, factB))

	// Verify outbound references
	gotA, _ := k.GetFact(ctx, "fact-A")
	require.Equal(t, []string{"fact-B", "fact-C"}, gotA.References)

	gotB, _ := k.GetFact(ctx, "fact-B")
	require.Equal(t, []string{"fact-C"}, gotB.References)

	gotC, _ := k.GetFact(ctx, "fact-C")
	require.Empty(t, gotC.References, "fact-C has no outbound references")

	// Build inbound index
	inbound := make(map[string][]string)
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		for _, refID := range f.References {
			inbound[refID] = append(inbound[refID], f.Id)
		}
		return false
	})

	// Verify bidirectional consistency
	// fact-B is referenced by fact-A
	require.Contains(t, inbound["fact-B"], "fact-A")
	// fact-C is referenced by both A and B
	require.Contains(t, inbound["fact-C"], "fact-A")
	require.Contains(t, inbound["fact-C"], "fact-B")
	require.Len(t, inbound["fact-C"], 2)
	// fact-A has no inbound references
	require.Empty(t, inbound["fact-A"], "fact-A should have no inbound references")
}

// ─── Helper ──────────────────────────────────────────────────────────────────

func makeFactID(prefix string, index int) string {
	return fmt.Sprintf("%s-%d", prefix, index)
}
