package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zerone-chain/zerone/x/capture_defense/keeper"
	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// ---------- Mock PacingKeeper ----------

type mockPacingKeeper struct {
	creationBps uint64
	analysisBps uint64
}

func (m *mockPacingKeeper) GetGlobalPacingMultiplier(_ context.Context) (uint64, uint64) {
	return m.creationBps, m.analysisBps
}

// ---------- Tests: Adaptive Pacing (R29-6) ----------

func TestPacingBaseInterval_NoPacingKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	// Set params with RiskAnalysisInterval=1000
	params := types.DefaultParams()
	params.RiskAnalysisInterval = 1000
	params.DecayEpochBlocks = 100000 // large, won't trigger
	k.SetParams(ctx, params)

	// Record some data so RunAutoAnalysis has something to process
	_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority: authority, Domain: "test-pacing", RoundId: "r1",
		Validators: []string{testAddr(1)}, Verdicts: []bool{true},
	})

	// No pacing keeper set — should use base interval 1000
	// Block 999: should NOT trigger analysis
	ctx999 := ctx.WithBlockHeight(999)
	_ = k.BeginBlocker(ctx999)
	_, found := k.GetCaptureMetrics(ctx999, "test-pacing")
	if found {
		t.Error("expected no capture metrics at block 999 (before base interval)")
	}

	// Block 1000: should trigger analysis
	ctx1000 := ctx.WithBlockHeight(1000)
	_ = k.BeginBlocker(ctx1000)
	_, found = k.GetCaptureMetrics(ctx1000, "test-pacing")
	if !found {
		t.Error("expected capture metrics at block 1000 (base interval)")
	}
}

func TestPacingDegraded_AnalysisBps1500000(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.RiskAnalysisInterval = 1000
	params.DecayEpochBlocks = 100000
	k.SetParams(ctx, params)

	_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority: authority, Domain: "test-degraded", RoundId: "r1",
		Validators: []string{testAddr(1)}, Verdicts: []bool{true},
	})

	// Set pacing keeper with analysisBps=1_500_000 (1.5x)
	// effectiveInterval = 1000 * 1_000_000 / 1_500_000 = 666
	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 1_500_000}
	k.SetPacingKeeper(pk)

	// Block 666: should trigger (666 % 666 == 0)
	ctx666 := ctx.WithBlockHeight(666)
	_ = k.BeginBlocker(ctx666)
	_, found := k.GetCaptureMetrics(ctx666, "test-degraded")
	if !found {
		t.Error("expected capture metrics at block 666 (degraded pacing, effective interval=666)")
	}

	// Block 500: should NOT trigger (500 % 666 != 0)
	k2, ctx2 := setupKeeper(t)
	srv2 := keeper.NewMsgServerImpl(k2)
	k2.SetParams(ctx2, params)
	_, _ = srv2.RecordVerification(ctx2, &types.MsgRecordVerification{
		Authority: k2.GetAuthority(), Domain: "test-degraded2", RoundId: "r1",
		Validators: []string{testAddr(1)}, Verdicts: []bool{true},
	})
	k2.SetPacingKeeper(pk)

	ctx500 := ctx2.WithBlockHeight(500)
	_ = k2.BeginBlocker(ctx500)
	_, found = k2.GetCaptureMetrics(ctx500, "test-degraded2")
	if found {
		t.Error(fmt.Sprintf("expected no capture metrics at block 500 (500 %% 666 != 0)"))
	}
}

func TestPacingCritical_AnalysisBps2000000(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.RiskAnalysisInterval = 1000
	params.DecayEpochBlocks = 100000
	k.SetParams(ctx, params)

	_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority: authority, Domain: "test-critical", RoundId: "r1",
		Validators: []string{testAddr(1)}, Verdicts: []bool{true},
	})

	// Set pacing keeper with analysisBps=2_000_000 (2x)
	// effectiveInterval = 1000 * 1_000_000 / 2_000_000 = 500
	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 2_000_000}
	k.SetPacingKeeper(pk)

	// Block 500: should trigger (500 % 500 == 0)
	ctx500 := ctx.WithBlockHeight(500)
	_ = k.BeginBlocker(ctx500)
	_, found := k.GetCaptureMetrics(ctx500, "test-critical")
	if !found {
		t.Error("expected capture metrics at block 500 (critical pacing, effective interval=500)")
	}

	// Block 999: should NOT trigger (999 % 500 != 0)
	k2, ctx2 := setupKeeper(t)
	srv2 := keeper.NewMsgServerImpl(k2)
	k2.SetParams(ctx2, params)
	_, _ = srv2.RecordVerification(ctx2, &types.MsgRecordVerification{
		Authority: k2.GetAuthority(), Domain: "test-critical2", RoundId: "r1",
		Validators: []string{testAddr(1)}, Verdicts: []bool{true},
	})
	k2.SetPacingKeeper(pk)

	ctx999 := ctx2.WithBlockHeight(999)
	_ = k2.BeginBlocker(ctx999)
	_, found = k2.GetCaptureMetrics(ctx999, "test-critical2")
	if found {
		t.Error("expected no capture metrics at block 999 (999 %% 500 != 0)")
	}
}

func TestPacingNeutral_AnalysisBps1000000(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	params := types.DefaultParams()
	params.RiskAnalysisInterval = 1000
	params.DecayEpochBlocks = 100000
	k.SetParams(ctx, params)

	_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority: authority, Domain: "test-neutral", RoundId: "r1",
		Validators: []string{testAddr(1)}, Verdicts: []bool{true},
	})

	// analysisBps=1_000_000 means no adjustment (guard condition: analysisPacing != 1_000_000)
	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Block 1000: should trigger at base interval
	ctx1000 := ctx.WithBlockHeight(1000)
	_ = k.BeginBlocker(ctx1000)
	_, found := k.GetCaptureMetrics(ctx1000, "test-neutral")
	if !found {
		t.Error("expected capture metrics at block 1000 (neutral pacing, base interval)")
	}
}
