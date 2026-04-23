package cross_stack_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"
	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Wave 12: resilience drill — every primitive in one exercise ────────
//
// The crown-jewel test. Simulates a full incident-response cycle exercising
// EVERY resilience primitive: circuit breaker, incident record, named
// upgrade, migration marker, event audit trail. If this passes, the chain
// has a working damage-containment + recovery pipeline end-to-end.

// TestResilience_FullDrillP0 — end-to-end simulation:
//   1. Bug discovered: a future build of CreateTrainingManifest would
//      corrupt child merkle roots. Open P0 incident.
//   2. Pause the knowledge module (circuit breaker on — writes refuse).
//   3. Verify the write-path rejects with a clear error.
//   4. Record a PauseModule remediation on the incident.
//   5. Register the named upgrade (already registered in app.go).
//   6. Apply the upgrade via RunUpgradeHandlerForTests.
//   7. Verify the v4 migration marker present (fix "deployed").
//   8. Record a NAMED_UPGRADE remediation on the incident.
//   9. Unpause the knowledge module.
//   10. Verify the write-path succeeds again.
//   11. Record DOCUMENTATION remediation.
//   12. Resolve the incident + close.
//   13. Dashboard queries confirm zero open incidents, zero paused modules.
func TestResilience_FullDrillP0(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	operator := testAddr("resilience_op").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-drill", OperatorAddress: operator, TokenizerVersion: 1,
	}))

	// ── STEP 1: open incident ─────────────────────────────────────────
	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority:       authority,
		Id:              "ZR-DRILL-001",
		Severity:        knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P0,
		Title:           "Manifest child-root corruption in edge case",
		Description:     "A specific path in CreateTrainingManifest produces a Merkle root that cannot be re-verified by external consumers. Fix ships as v1.0.2 knowledge v3→v4 migration.",
		AffectedModules: []string{"knowledge"},
	})
	require.NoError(t, err)

	// ── STEP 2: pause the knowledge module ────────────────────────────
	pauseResp, err := ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:          authority,
		ModuleName:         knowledgetypes.ModuleName,
		Reason:             "ZR-DRILL-001: stopping writes while migration is deployed",
		IncidentId:         "ZR-DRILL-001",
		AutoUnpauseAtBlock: 0, // manual unpause only
	})
	require.NoError(t, err)
	require.Greater(t, pauseResp.PausedAtBlock, uint64(0))

	// Sanity-check: dashboard surfaces the paused module.
	paused, err := qs.PausedModules(h.Ctx, &knowledgetypes.QueryPausedModulesRequest{})
	require.NoError(t, err)
	require.Len(t, paused.Paused, 1)
	require.Equal(t, knowledgetypes.ModuleName, paused.Paused[0].ModuleName)
	require.Equal(t, "ZR-DRILL-001", paused.Paused[0].IncidentId)

	// ── STEP 3: verify write-path rejects ─────────────────────────────
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator:        operator,
		Id:             "m-blocked",
		PipelineId:     "pipe-drill",
		CorpusSelector: &knowledgetypes.CorpusSelector{},
	})
	require.Error(t, err, "paused module must reject writes")
	require.Contains(t, err.Error(), "paused")
	require.Contains(t, err.Error(), "ZR-DRILL-001",
		"pause reason referencing the incident should surface in the error")

	// ── STEP 4: record pause as a remediation ─────────────────────────
	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority:  authority,
		IncidentId: "ZR-DRILL-001",
		Type:       knowledgetypes.RemediationType_REMEDIATION_TYPE_STATE_CORRECTION,
		Reference:  "ModulePause:knowledge",
		Note:       "circuit breaker engaged; writes suspended",
	})
	require.NoError(t, err)

	// ── STEP 5 + 6: apply the named upgrade ───────────────────────────
	current := h.App.CurrentModuleVersionMap()
	fromVM := make(module.VersionMap, len(current))
	for name, ver := range current {
		fromVM[name] = ver
	}
	fromVM["knowledge"] = 3 // simulate chain at pre-upgrade state
	toVM, err := h.App.RunUpgradeHandlerForTests(h.Ctx, zeroneapp.UpgradeNameTestnetV3, fromVM, h.Height())
	require.NoError(t, err, "named upgrade runs despite the module being paused (handlers, not migrations, gate writes)")
	require.Equal(t, uint64(4), toVM["knowledge"])

	// ── STEP 7: migration marker present — fix "deployed" ─────────────
	require.Equal(t, "true", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "migration_v4_complete"))

	// ── STEP 8: record the named-upgrade remediation ──────────────────
	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority:  authority,
		IncidentId: "ZR-DRILL-001",
		Type:       knowledgetypes.RemediationType_REMEDIATION_TYPE_NAMED_UPGRADE,
		Reference:  zeroneapp.UpgradeNameTestnetV3,
		Note:       "fix migrated knowledge v3→v4",
	})
	require.NoError(t, err)

	// ── STEP 9: unpause the module ────────────────────────────────────
	_, err = ms.UnpauseModule(h.Ctx, &knowledgetypes.MsgUnpauseModule{
		Authority:  authority,
		ModuleName: knowledgetypes.ModuleName,
		Note:       "ZR-DRILL-001 remediated; writes resume",
	})
	require.NoError(t, err)
	require.False(t, h.KnowledgeKeeper.IsModulePaused(h.Ctx, knowledgetypes.ModuleName))

	// ── STEP 10: write-path succeeds again ────────────────────────────
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator:        operator,
		Id:             "m-resume",
		PipelineId:     "pipe-drill",
		CorpusSelector: &knowledgetypes.CorpusSelector{},
	})
	require.NoError(t, err, "post-unpause write must succeed")

	// ── STEP 11: documentation remediation ────────────────────────────
	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority:  authority,
		IncidentId: "ZR-DRILL-001",
		Type:       knowledgetypes.RemediationType_REMEDIATION_TYPE_DOCUMENTATION,
		Reference:  "ipfs://Qm.../ZR-DRILL-001.md",
		Note:       "post-mortem published",
	})
	require.NoError(t, err)

	// ── STEP 12: resolve + close ──────────────────────────────────────
	_, err = ms.ResolveIncident(h.Ctx, &knowledgetypes.MsgResolveIncident{
		Authority:     authority,
		IncidentId:    "ZR-DRILL-001",
		PostMortemUri: "ipfs://Qm.../ZR-DRILL-001.md",
	})
	require.NoError(t, err)
	_, err = ms.CloseIncident(h.Ctx, &knowledgetypes.MsgCloseIncident{
		Authority:  authority,
		IncidentId: "ZR-DRILL-001",
	})
	require.NoError(t, err)

	// ── STEP 13: dashboards show clean state ──────────────────────────
	openInc, err := qs.OpenIncidents(h.Ctx, &knowledgetypes.QueryOpenIncidentsRequest{})
	require.NoError(t, err)
	require.Empty(t, openInc.Incidents, "no open incidents after drill")

	pausedAfter, err := qs.PausedModules(h.Ctx, &knowledgetypes.QueryPausedModulesRequest{})
	require.NoError(t, err)
	require.Empty(t, pausedAfter.Paused, "no paused modules after drill")

	// ── Audit: incident record shows the full remediation lineage ─────
	rec, _ := h.KnowledgeKeeper.GetIncidentRecord(h.Ctx, "ZR-DRILL-001")
	require.Equal(t, knowledgetypes.IncidentStatus_INCIDENT_STATUS_CLOSED, rec.Status)
	require.Len(t, rec.Remediations, 3,
		"drill records 3 remediations: STATE_CORRECTION (pause) + NAMED_UPGRADE + DOCUMENTATION")
	require.Equal(t, knowledgetypes.RemediationType_REMEDIATION_TYPE_STATE_CORRECTION,
		rec.Remediations[0].Type)
	require.Equal(t, knowledgetypes.RemediationType_REMEDIATION_TYPE_NAMED_UPGRADE,
		rec.Remediations[1].Type)
	require.Equal(t, knowledgetypes.RemediationType_REMEDIATION_TYPE_DOCUMENTATION,
		rec.Remediations[2].Type)
}

// TestResilience_AutoUnpause — pauses can self-expire if
// auto_unpause_at_block is set. Useful for pre-planned maintenance windows
// where the pause should lift regardless of whether governance remembers.
func TestResilience_AutoUnpause(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	autoUnpauseBlock := uint64(h.Height() + 3)
	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:          authority,
		ModuleName:         "knowledge",
		Reason:             "maintenance-window test",
		AutoUnpauseAtBlock: autoUnpauseBlock,
	})
	require.NoError(t, err)
	require.True(t, h.KnowledgeKeeper.IsModulePaused(h.Ctx, "knowledge"))

	// Advance past auto-unpause block — the check self-clears.
	h.AdvanceBlocks(5)
	require.False(t, h.KnowledgeKeeper.IsModulePaused(h.Ctx, "knowledge"),
		"auto_unpause_at_block honoured on next IsModulePaused check")
}

// TestResilience_PauseAuthorityGate — non-authority attempts are rejected.
func TestResilience_PauseAuthorityGate(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	intruder := testAddr("intruder_resilience").String()

	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:  intruder,
		ModuleName: "knowledge",
		Reason:     "I want to halt the chain",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
}

// TestResilience_UnpauseRequiresActivePause — unpausing a non-paused
// module rejects with a clear error.
func TestResilience_UnpauseRequiresActivePause(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	_, err = ms.UnpauseModule(h.Ctx, &knowledgetypes.MsgUnpauseModule{
		Authority:  h.KnowledgeKeeper.GetAuthority(),
		ModuleName: "knowledge",
	})
	require.Error(t, err, "cannot unpause a module that isn't paused")
	require.Contains(t, err.Error(), "not paused")
}

// TestResilience_PausedModuleReadsStillWork — the breaker gates writes,
// not reads. Queries remain available so operators can inspect the state
// that's being protected.
func TestResilience_PausedModuleReadsStillWork(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority: authority, ModuleName: "knowledge", Reason: "read test",
	})
	require.NoError(t, err)

	// Reads still function.
	caps, err := qs.RouteBCapabilities(h.Ctx, &knowledgetypes.QueryRouteBCapabilitiesRequest{})
	require.NoError(t, err, "read-path queries remain available under pause")
	require.NotNil(t, caps.Capabilities)

	paused, err := qs.PausedModules(h.Ctx, &knowledgetypes.QueryPausedModulesRequest{})
	require.NoError(t, err)
	require.Len(t, paused.Paused, 1)
}
