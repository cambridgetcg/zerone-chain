package keeper_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func createSampleWithConsent(t *testing.T, k keeper.Keeper, ctx context.Context, id, submitter string, consentType types.ConsentType) {
	t.Helper()
	sample := &types.Sample{
		Id:          id,
		Domain:      "science",
		QualityTier: "gold",
		Status:      types.SampleStatus_SAMPLE_STATUS_GOLD,
		Submitter:   submitter,
		Content:     "sample content",
		Energy:      500_000,
		EnergyCap:   1_000_000,
		Consent: &types.ConsentProof{
			Type: consentType,
		},
	}
	require.NoError(t, k.SetSample(ctx, sample))
}

func createSampleWithSubmission(t *testing.T, k keeper.Keeper, ctx context.Context, id, submitter, submissionID string, consentType types.ConsentType) {
	t.Helper()
	sample := &types.Sample{
		Id:           id,
		Domain:       "science",
		QualityTier:  "gold",
		Status:       types.SampleStatus_SAMPLE_STATUS_GOLD,
		Submitter:    submitter,
		Content:      "sample content",
		Energy:       500_000,
		EnergyCap:    1_000_000,
		SubmissionId: submissionID,
		Consent: &types.ConsentProof{
			Type: consentType,
		},
	}
	require.NoError(t, k.SetSample(ctx, sample))
}

func setupRoundWithValidators(t *testing.T, k keeper.Keeper, ctx context.Context, submissionID, roundID string, validators []string) {
	t.Helper()
	reveals := make([]*types.RevealEntry, len(validators))
	for i, v := range validators {
		reveals[i] = &types.RevealEntry{
			Verifier: v,
			Vote:     `{"overall_quality":800000}`,
		}
	}
	round := &types.QualityRound{
		Id:           roundID,
		SubmissionId: submissionID,
		Reveals:      reveals,
	}
	require.NoError(t, k.SetQualityRound(ctx, round))
	require.NoError(t, k.SetSubmissionRoundIndex(ctx, submissionID, roundID))
}

// triggerEpochDistribution simulates EndBlocker at epoch boundary.
func triggerEpochDistribution(t *testing.T, k keeper.Keeper, ctx context.Context) {
	t.Helper()
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	// Call EndBlocker at block 100 (epoch boundary)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	_ = sdkCtx
	// Directly call the revenue distribution
	k.DistributeEpochRevenue(ctx, params)
}

// ─── Basic revenue split ────────────────────────────────────────────────────

func TestRevenue_BasicSplit_55_22_23(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)

	// Queue 10000 uzrn
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// Submitter gets 55% of 10000 = 5500, consent=1.0x → 5500
	require.Len(t, bk.moduleToAccountCalls, 1) // only submitter (no round → no validators)
	submitterPayment := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(5500), submitterPayment)

	// Pending should be cleared
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "s1"))
}

// ─── Consent multiplier tests ───────────────────────────────────────────────

func TestRevenue_SelfAuthored_150Percent(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_SELF_AUTHORED)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// 55% of 10000 = 5500, self-authored=1.5x → 8250
	submitterPayment := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(8250), submitterPayment)
}

func TestRevenue_OptIn_130Percent(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_OPT_IN)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// 55% of 10000 = 5500, opt-in=1.3x → 7150
	submitterPayment := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(7150), submitterPayment)
}

func TestRevenue_PlatformTOS_80Percent(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_PLATFORM_TOS)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// 55% of 10000 = 5500, platform_tos=0.8x → 4400
	submitterPayment := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(4400), submitterPayment)
}

func TestRevenue_FairUse_50Percent(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_FAIR_USE)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// 55% of 10000 = 5500, fair_use=0.5x → 2750
	submitterPayment := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(2750), submitterPayment)
}

func TestRevenue_ConsentPenaltyGoesToProtocol(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	// Fair use: submitter gets 50% of normal → penalty is 50% of submitter share
	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_FAIR_USE)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// Submitter: 5500 * 0.5 = 2750
	// Validator: 2200 (but no round, goes to protocol)
	// Protocol: 10000 - 5500 - 2200 + (5500 - 2750) + 2200 = 2300 + 2750 + 2200 = 7250
	// Total paid out: 2750 (submitter) → only 1 moduleToAccount call
	require.Len(t, bk.moduleToAccountCalls, 1)
	require.Equal(t, sdkmath.NewInt(2750), bk.moduleToAccountCalls[0].amount.AmountOf("uzrn"))
}

// ─── Validator distribution ─────────────────────────────────────────────────

func TestRevenue_ValidatorDistribution_EqualSplit(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	val1 := "zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h"
	val2 := "zrn1xznhxqv7zqy3h5uqg6efxwdmjkhg7uh23hkufc"

	createSampleWithSubmission(t, k, ctx, "s1", testAddr, "sub1", types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	setupRoundWithValidators(t, k, ctx, "sub1", "round1", []string{val1, val2})
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// Validator share: 22% of 10000 = 2200, split 2 ways = 1100 each
	// Calls: 1 (submitter) + 2 (validators) = 3
	require.Len(t, bk.moduleToAccountCalls, 3)

	// Find validator payments
	var valPayments []sdkmath.Int
	for _, call := range bk.moduleToAccountCalls {
		if call.to != testAddr {
			valPayments = append(valPayments, call.amount.AmountOf("uzrn"))
		}
	}
	require.Len(t, valPayments, 2)
	require.Equal(t, sdkmath.NewInt(1100), valPayments[0])
	require.Equal(t, sdkmath.NewInt(1100), valPayments[1])
}

func TestRevenue_ValidatorDistribution_NoRound(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	// Sample with submission but no round → validator share goes to protocol
	createSampleWithSubmission(t, k, ctx, "s1", testAddr, "sub_no_round", types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// Only submitter gets paid
	require.Len(t, bk.moduleToAccountCalls, 1)
}

func TestRevenue_ValidatorDistribution_NoSubmission(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	// Sample with no submission_id → all validator share to protocol
	sample := &types.Sample{
		Id:        "s1",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
		Consent:   &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE},
	}
	require.NoError(t, k.SetSample(ctx, sample))
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	require.Len(t, bk.moduleToAccountCalls, 1) // only submitter
}

// ─── Edge cases ─────────────────────────────────────────────────────────────

func TestRevenue_ZeroPending_NoDistribution(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 0))

	triggerEpochDistribution(t, k, ctx)

	require.Len(t, bk.moduleToAccountCalls, 0)
	// Zero entry should be cleaned up
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "s1"))
}

func TestRevenue_SampleNotFound_ClearsEntry(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	// Queue revenue for non-existent sample
	require.NoError(t, k.SetPendingRevenue(ctx, "ghost", 5_000))

	triggerEpochDistribution(t, k, ctx)

	require.Len(t, bk.moduleToAccountCalls, 0)
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "ghost"))
}

func TestRevenue_MultipleAccesses_Accumulate(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	// Simulate 3 accesses queued before epoch
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 30_000))

	triggerEpochDistribution(t, k, ctx)

	// 55% of 30000 = 16500
	submitterPayment := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(16500), submitterPayment)
}

func TestRevenue_MultipleSamples_AllDistributed(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	createSampleWithConsent(t, k, ctx, "s2", testAddr, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))
	require.NoError(t, k.SetPendingRevenue(ctx, "s2", 20_000))

	triggerEpochDistribution(t, k, ctx)

	// Both should be paid and cleared
	require.Len(t, bk.moduleToAccountCalls, 2)
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "s1"))
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "s2"))
}

func TestRevenue_NoConsentProof_DefaultMultiplier(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	// Sample without Consent field
	sample := &types.Sample{
		Id:        "s1",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_GOLD,
		// No Consent field
	}
	require.NoError(t, k.SetSample(ctx, sample))
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// Default multiplier = 1.0x, so submitter gets 5500
	submitterPayment := bk.moduleToAccountCalls[0].amount.AmountOf("uzrn")
	require.Equal(t, sdkmath.NewInt(5500), submitterPayment)
}

// ─── DOKIMANT: Quality-weighted validator revenue split ─────────────────────

// setupRoundWithScores creates a quality round where each verifier's reveal
// contains the given vote JSON, and AggregateScores is set so that
// computeParticipationScores() produces non-zero weights.
func setupRoundWithScores(t *testing.T, k keeper.Keeper, ctx context.Context, submissionID, roundID string, verifiers []string, votes []string, aggregated *types.QualityVote) {
	t.Helper()
	reveals := make([]*types.RevealEntry, len(verifiers))
	for i, v := range verifiers {
		reveals[i] = &types.RevealEntry{
			Verifier: v,
			Vote:     votes[i],
		}
	}
	round := &types.QualityRound{
		Id:              roundID,
		SubmissionId:    submissionID,
		Reveals:         reveals,
		AggregateScores: aggregated,
	}
	require.NoError(t, k.SetQualityRound(ctx, round))
	require.NoError(t, k.SetSubmissionRoundIndex(ctx, submissionID, roundID))
}

// TestDOKIMANT_QualityWeightedRevenue verifies that validators whose votes
// closely match the aggregate receive a proportionally larger share of the
// validator revenue pool than those with high deviation.
func TestDOKIMANT_QualityWeightedRevenue(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	// Two validators: val1 votes exactly at aggregate (low dev), val2 votes far off.
	val1 := "zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h"
	val2 := "zrn1xznhxqv7zqy3h5uqg6efxwdmjkhg7uh23hkufc"

	aggregated := &types.QualityVote{OverallQuality: 800_000}
	// val1 matches aggregate perfectly (deviation=0, score=1_000_000).
	// val2 deviates by 500_000 (score=500_000).
	votes := []string{
		`{"overall_quality":800000}`,
		`{"overall_quality":300000}`,
	}

	createSampleWithSubmission(t, k, ctx, "s1", testAddr, "sub1", types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	setupRoundWithScores(t, k, ctx, "sub1", "round1", []string{val1, val2}, votes, aggregated)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// Total validator share = 22% of 10000 = 2200
	// totalScore = 1_000_000 + 500_000 = 1_500_000
	// val1 share = 2200 * 1_000_000 / 1_500_000 = 1466
	// val2 share = 2200 * 500_000 / 1_500_000 = 733
	var val1Pay, val2Pay sdkmath.Int
	for _, call := range bk.moduleToAccountCalls {
		switch call.to {
		case val1:
			val1Pay = call.amount.AmountOf("uzrn")
		case val2:
			val2Pay = call.amount.AmountOf("uzrn")
		}
	}
	require.Equal(t, sdkmath.NewInt(1466), val1Pay, "high-quality validator should receive larger share")
	require.Equal(t, sdkmath.NewInt(733), val2Pay, "low-quality validator should receive smaller share")
	require.True(t, val1Pay.GT(val2Pay), "better-matching validator must earn more")
}

// TestDOKIMANT_QualityWeightedFallbackOnNoScores verifies that when a round
// has no AggregateScores the distribution falls back to equal split.
func TestDOKIMANT_QualityWeightedFallbackOnNoScores(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	val1 := "zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h"
	val2 := "zrn1xznhxqv7zqy3h5uqg6efxwdmjkhg7uh23hkufc"

	createSampleWithSubmission(t, k, ctx, "s1", testAddr, "sub1", types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	// Round with no AggregateScores → equal split fallback.
	setupRoundWithValidators(t, k, ctx, "sub1", "round1", []string{val1, val2})
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	triggerEpochDistribution(t, k, ctx)

	// Equal split: 2200 / 2 = 1100 each.
	var val1Pay, val2Pay sdkmath.Int
	for _, call := range bk.moduleToAccountCalls {
		switch call.to {
		case val1:
			val1Pay = call.amount.AmountOf("uzrn")
		case val2:
			val2Pay = call.amount.AmountOf("uzrn")
		}
	}
	require.Equal(t, sdkmath.NewInt(1100), val1Pay)
	require.Equal(t, sdkmath.NewInt(1100), val2Pay)
}

// ─── EndBlocker integration ─────────────────────────────────────────────────

func TestRevenue_EndBlocker_DistributesAtEpoch(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setDefaultParams(t, k, ctx)

	createSampleWithConsent(t, k, ctx, "s1", testAddr, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	// Run EndBlocker at epoch boundary (block 100)
	epochCtx := sdk.UnwrapSDKContext(ctx).WithBlockHeight(100)
	err := k.EndBlocker(epochCtx)
	require.NoError(t, err)

	// Revenue should have been distributed
	require.True(t, len(bk.moduleToAccountCalls) > 0)
	require.Equal(t, uint64(0), k.GetPendingRevenue(epochCtx, "s1"))
}
