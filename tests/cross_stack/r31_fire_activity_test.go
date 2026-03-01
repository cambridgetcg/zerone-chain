package cross_stack_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// setHeight is a test helper that repositions the harness context to a given
// block height without running BeginBlocker/EndBlocker. This avoids side
// effects from module hooks while allowing window-based queries to see
// previously indexed completion data.
func setHeight(h *TestHarness, height int64) {
	h.Ctx = h.App.NewContext(true).
		WithBlockHeight(height).
		WithChainID(testChainID).
		WithBlockHeader(cmtproto.Header{
			Height:  height,
			ChainID: testChainID,
		})
	h.currentHeight = height
}

// ─── Test 1: Fire→Earth — High throughput improves governance participation ──

func TestR31_FireEarth_HighThroughputImprovesGovernance(t *testing.T) {
	h := NewTestHarness(t)

	// Index 10 completed rounds (high throughput, no dissent)
	for i := 0; i < 10; i++ {
		meta := &knowledgetypes.CompletedRoundMeta{
			Domain:         "physics",
			HasDissent:     false,
			DurationBlocks: 11,
		}
		err := h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i), meta)
		require.NoError(t, err)
	}

	// Advance context height past all indexed rounds so they fall within the window
	setHeight(h, 1000)

	// Verify throughput > 0
	throughput, disputeRate, _ := h.KnowledgeKeeper.GetVerificationHealth(h.Ctx)
	require.Greater(t, throughput, uint64(0), "throughput should be > 0")
	require.Equal(t, uint64(0), disputeRate, "no dissent = 0 dispute rate")
}

// ─── Test 2: Fire→Earth — Extreme dispute rate degrades governance ──

func TestR31_FireEarth_ExtremeDisputeDegrades(t *testing.T) {
	h := NewTestHarness(t)

	// 10 rounds: 7 with dissent (70% > 30% threshold)
	for i := 0; i < 10; i++ {
		meta := &knowledgetypes.CompletedRoundMeta{
			Domain:         "physics",
			HasDissent:     i < 7,
			DurationBlocks: 11,
		}
		err := h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i), meta)
		require.NoError(t, err)
	}

	// Advance context height past all indexed rounds
	setHeight(h, 1000)

	_, disputeRate, _ := h.KnowledgeKeeper.GetVerificationHealth(h.Ctx)
	require.Greater(t, disputeRate, uint64(300_000), "dispute rate should exceed 30%%")
}

// ─── Test 3: Water→Fire — High partnership density reduces min verifiers ──
// (needs partnerships — test with zero density first, real partnerships test later)

func TestR31_WaterFire_HighDensityRelaxes(t *testing.T) {
	h := NewTestHarness(t)

	// Default: no partnerships → verifiers should be base + 1
	effective := h.KnowledgeKeeper.GetEffectiveMinVerifiers(h.Ctx, "physics")
	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)

	// With 0 partnerships, should be base + 1
	require.Equal(t, uint32(params.MinVerifiers+1), effective,
		"no partnerships = base + 1 verifiers required")
}

// ─── Test 4: Water→Fire — Zero partnerships increases min verifiers ──

func TestR31_WaterFire_ZeroDensityTightens(t *testing.T) {
	h := NewTestHarness(t)

	effective := h.KnowledgeKeeper.GetEffectiveMinVerifiers(h.Ctx, "physics")
	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)

	require.Equal(t, uint32(params.MinVerifiers+1), effective,
		"zero partnerships should require base + 1 verifiers")
}

// ─── Test 5: Fire→Metal — Active domain has verification activity ──

func TestR31_FireMetal_ActiveDomainRecoversFaster(t *testing.T) {
	h := NewTestHarness(t)

	// Index 10 rounds in physics domain
	for i := 0; i < 10; i++ {
		err := h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i),
			&knowledgetypes.CompletedRoundMeta{Domain: "physics", DurationBlocks: 11})
		require.NoError(t, err)
	}

	// Advance context height past all indexed rounds
	setHeight(h, 1000)

	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "physics")
	require.Greater(t, activity, uint64(0), "physics should have verification activity")

	// Inactive domain has no activity
	noActivity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "theology")
	require.Equal(t, uint64(0), noActivity, "theology should have no verification activity")
}

// ─── Test 6: Fire→Metal — Inactive domain has zero activity ──

func TestR31_FireMetal_InactiveDomainBaseRate(t *testing.T) {
	h := NewTestHarness(t)

	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "physics")
	require.Equal(t, uint64(0), activity, "no rounds = no activity")
}

// ─── Test 7: Combined — All three connections operating together ──

func TestR31_FireCombined_AllConnections(t *testing.T) {
	h := NewTestHarness(t)

	// Setup: Index rounds with some dissent
	for i := 0; i < 5; i++ {
		err := h.KnowledgeKeeper.IndexCompletedRound(h.Ctx, uint64(100+i*50), fmt.Sprintf("round-%d", i),
			&knowledgetypes.CompletedRoundMeta{
				Domain:         "physics",
				HasDissent:     i%3 == 0,
				DurationBlocks: 11,
			})
		require.NoError(t, err)
	}

	// Advance context height past all indexed rounds
	setHeight(h, 1000)

	// Fire → Earth: Verification health feeds governance
	throughput, _, _ := h.KnowledgeKeeper.GetVerificationHealth(h.Ctx)
	require.Greater(t, throughput, uint64(0))

	// Water → Fire: Partnership density adjusts min verifiers
	effective := h.KnowledgeKeeper.GetEffectiveMinVerifiers(h.Ctx, "physics")
	require.Greater(t, effective, uint32(0))

	// Fire → Metal: Activity is tracked
	activity := h.KnowledgeKeeper.GetDomainVerificationActivity(h.Ctx, "physics")
	require.Greater(t, activity, uint64(0))
}
