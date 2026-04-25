package cross_stack_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
	trustscorekeeper "github.com/zerone-chain/zerone/x/trust_score/keeper"
	trustscoretypes "github.com/zerone-chain/zerone/x/trust_score/types"
)

// x/trust_score is the second module created from the integration
// pattern: a per-address synthesizer that bundles signals from
// x/knowledge (calibration), x/qualification (metrics, penalties,
// status), and x/capture_challenge (cartel strikes) into a single
// composite trust band. Pure consumer; no state; deterministic
// synthesis.
//
// These tests drive the synthesizer end-to-end against the production
// keepers wired in app.go. Three bands exercised: A (clean record),
// C (mid composite), F (cartel strike).

// Grade A: high calibration + strong domain accuracy + no strikes,
// no penalties.
func TestTrustScore_GradeAOnCleanRecord(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	addr := testAddr("trust_clean").String()

	// Strong global submission record: 90 accepted of 100.
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: addr, CalibrationScoreBps: 900_000,
		Accepted: 90, TotalSubmissions: 100,
	}))

	// Strong domain qualification with strong accuracy.
	h.SetDomainQualification(addr, "mathematics", 90)
	q, _ := h.QualificationKeeper.GetQualification(h.Ctx, addr, "mathematics")
	q.Metrics = &qualificationtypes.QualificationMetrics{
		TotalVerifications:   200,
		CorrectVerifications: 180,
		AccuracyBps:          900_000,
	}
	h.QualificationKeeper.SetQualification(h.Ctx, q)

	qs := trustscorekeeper.NewQueryServerImpl(h.TrustScoreKeeper)
	resp, err := qs.TrustScore(h.Ctx, &trustscoretypes.QueryTrustScoreRequest{Address: addr})
	require.NoError(t, err)
	score := resp.Score
	require.NotNil(t, score)
	require.Equal(t, addr, score.Address)
	require.Equal(t, "A", score.Band, "clean high-accuracy record = Grade A")
	require.Equal(t, uint32(0), score.CartelStrikes)
	require.Equal(t, uint32(0), score.ActivePenalties)
	require.GreaterOrEqual(t, score.CompositeBps, uint64(800_000))
	require.Equal(t, uint64(900_000), score.SubmissionAccuracyBps)
	require.Equal(t, uint64(900_000), score.VerificationAccuracyBps)
	require.Len(t, score.PerDomain, 1)
	require.Equal(t, "mathematics", score.PerDomain[0].Domain)
}

// Grade F via cartel strike: even with otherwise-strong record, a
// single UPHELD challenge against this address drops band to F.
// Cartel strikes are structural — they override composite arithmetic.
func TestTrustScore_GradeFOnCartelStrike(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	addr := testAddr("trust_struck").String()

	// Otherwise-strong record.
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: addr, CalibrationScoreBps: 900_000,
		Accepted: 90, TotalSubmissions: 100,
	}))
	h.SetDomainQualification(addr, "mathematics", 90)
	q, _ := h.QualificationKeeper.GetQualification(h.Ctx, addr, "mathematics")
	q.Metrics = &qualificationtypes.QualificationMetrics{
		TotalVerifications: 100, CorrectVerifications: 90, AccuracyBps: 900_000,
	}
	h.QualificationKeeper.SetQualification(h.Ctx, q)

	// Drive a cartel UPHELD against this address.
	whistleblower := testAddr("trust_whistle")
	require.NoError(t, h.FundAccount(whistleblower, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))
	driveCartelUpheld(t, h, whistleblower.String(), "mathematics", []string{addr})

	qs := trustscorekeeper.NewQueryServerImpl(h.TrustScoreKeeper)
	resp, err := qs.TrustScore(h.Ctx, &trustscoretypes.QueryTrustScoreRequest{Address: addr})
	require.NoError(t, err)
	score := resp.Score
	require.GreaterOrEqual(t, score.CartelStrikes, uint32(1),
		"cartel UPHELD against this address must register as a strike")
	require.Equal(t, "F", score.Band,
		"any cartel strike drops band to F regardless of other signals")
}

// Grade C: mediocre record (50% submission, 55% verification, no strikes
// or penalties) lands mid-band.
func TestTrustScore_GradeCOnMediocreRecord(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	addr := testAddr("trust_mid").String()
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: addr, CalibrationScoreBps: 500_000,
		Accepted: 50, TotalSubmissions: 100,
	}))
	h.SetDomainQualification(addr, "biology", 50)
	q, _ := h.QualificationKeeper.GetQualification(h.Ctx, addr, "biology")
	q.Metrics = &qualificationtypes.QualificationMetrics{
		TotalVerifications: 100, CorrectVerifications: 55, AccuracyBps: 550_000,
	}
	h.QualificationKeeper.SetQualification(h.Ctx, q)

	qs := trustscorekeeper.NewQueryServerImpl(h.TrustScoreKeeper)
	resp, err := qs.TrustScore(h.Ctx, &trustscoretypes.QueryTrustScoreRequest{Address: addr})
	require.NoError(t, err)
	score := resp.Score
	require.Equal(t, "C", score.Band,
		"mediocre composite (40-60%) = Grade C")
	require.Equal(t, uint32(0), score.CartelStrikes)
}

// Active qualification penalty (from a recent capture_challenge UPHELD)
// keeps a high-record address out of the top band even before the
// strike is technically fully resolved as a "strike".
//
// Tests the multi-signal compose: same address has high accuracy AND
// an active penalty AND a strike — it stays F because of the strike;
// but if we strip the strike and keep the penalty, it should drop to
// B (composite high enough but penalty prevents A).
func TestTrustScore_PenaltyKeepsAddressOutOfTopBand(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	addr := testAddr("trust_penalised").String()
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: addr, CalibrationScoreBps: 900_000,
		Accepted: 90, TotalSubmissions: 100,
	}))
	h.SetDomainQualification(addr, "mathematics", 90)
	q, _ := h.QualificationKeeper.GetQualification(h.Ctx, addr, "mathematics")
	q.Metrics = &qualificationtypes.QualificationMetrics{
		TotalVerifications: 100, CorrectVerifications: 90, AccuracyBps: 900_000,
	}
	h.QualificationKeeper.SetQualification(h.Ctx, q)

	// Apply a 30% penalty directly (simulating R28-8 reduction without
	// driving the full capture_challenge pipeline — keeps the test
	// focused on penalty-band interaction).
	require.NoError(t, h.QualificationKeeper.ReduceQualificationWeight(
		h.Ctx, addr, "mathematics", 300_000, uint64(h.Height())+10_000,
	))

	qs := trustscorekeeper.NewQueryServerImpl(h.TrustScoreKeeper)
	resp, err := qs.TrustScore(h.Ctx, &trustscoretypes.QueryTrustScoreRequest{Address: addr})
	require.NoError(t, err)
	score := resp.Score
	require.Equal(t, uint32(0), score.CartelStrikes)
	require.Equal(t, uint32(1), score.ActivePenalties,
		"active penalty must be counted")
	require.NotEqual(t, "A", score.Band,
		"active penalty must keep a strong-record address out of the A band")
	require.Contains(t, []string{"B", "C"}, score.Band)
}
