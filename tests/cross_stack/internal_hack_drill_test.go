package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Internal-hack drill ───────────────────────────────────────────────
//
// The attacker holds a privileged role — most commonly a compromised
// governance authority. Damage vector is "abuse legitimate powers", not
// "break into the chain". Each test below pins a distinct containment
// property that limits how far that abuse can reach.
//
// Audit: docs/ROUTE_B_INTERNAL_HACK_AUDIT.md

// ─── Containment 1: MaxPauseDurationBlocks caps authority pause ────────
//
// A compromised authority calls MsgPauseModule with
// auto_unpause_at_block=0 intending an indefinite DoS. The handler caps
// every pause at paused_at + Params.MaxPauseDurationBlocks, so the
// window self-expires even if the authority never unpauses. Permanent
// censorship via the breaker is structurally impossible.
func TestInternalHackDrill_PauseDurationCapped(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	params.MaxPauseDurationBlocks = 10
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:          authority,
		ModuleName:         knowledgetypes.ModuleName,
		Reason:             "(attacker's indefinite-pause attempt)",
		AutoUnpauseAtBlock: 0,
	})
	require.NoError(t, err)

	pauseRec, ok := h.KnowledgeKeeper.GetModulePause(h.Ctx, knowledgetypes.ModuleName)
	require.True(t, ok)
	require.Equal(t, pauseRec.PausedAtBlock+10, pauseRec.AutoUnpauseAtBlock,
		"handler capped the window to paused_at_block + MaxPauseDurationBlocks")

	h.AdvanceBlocks(15)
	require.False(t, h.KnowledgeKeeper.IsModulePaused(h.Ctx, knowledgetypes.ModuleName),
		"pause self-expired at the cap — permanent censorship via breaker is impossible")
}

// Caller-supplied window beyond the cap is truncated — authority cannot
// bypass the cap by naming an overlong explicit height.
func TestInternalHackDrill_OverlongPauseWindowTruncated(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	params, _ := h.KnowledgeKeeper.GetParams(h.Ctx)
	params.MaxPauseDurationBlocks = 20
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	heightAtPause := uint64(h.Height())
	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:          authority,
		ModuleName:         knowledgetypes.ModuleName,
		AutoUnpauseAtBlock: heightAtPause + 10_000_000,
	})
	require.NoError(t, err)
	rec, _ := h.KnowledgeKeeper.GetModulePause(h.Ctx, knowledgetypes.ModuleName)
	require.Equal(t, heightAtPause+20, rec.AutoUnpauseAtBlock,
		"overlong window truncated to cap")
}

// Honest short window within the cap is respected unchanged — the cap
// does not punish legitimate short-maintenance pauses.
func TestInternalHackDrill_HonestShortPauseWindowRespected(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	params, _ := h.KnowledgeKeeper.GetParams(h.Ctx)
	params.MaxPauseDurationBlocks = 1_000
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	heightAtPause := uint64(h.Height())
	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:          authority,
		ModuleName:         knowledgetypes.ModuleName,
		AutoUnpauseAtBlock: heightAtPause + 5,
	})
	require.NoError(t, err)
	rec, _ := h.KnowledgeKeeper.GetModulePause(h.Ctx, knowledgetypes.ModuleName)
	require.Equal(t, heightAtPause+5, rec.AutoUnpauseAtBlock,
		"short windows within the cap are honored unchanged")
}

// ─── Containment 2: PrivilegedActionLog makes abuse queryable ──────────
//
// Authority amends TraceSchema to a subtly-malicious variant. The
// handler accepts — legitimately-formed amendments cannot be rejected
// at the handler level without destroying the primitive. Containment
// works at the observability layer: every privileged call emits a
// structured PrivilegedAction record. External monitors poll the log,
// the community reviews, and the schema is amended back. Both the
// attack and the corrective amendment appear in the same log — full
// audit trail is preserved forward-only.
func TestInternalHackDrill_SchemaAmendmentDetection(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	_, err = ms.AmendTraceSchema(h.Ctx, &knowledgetypes.MsgAmendTraceSchema{
		Authority: authority,
		Schema: &knowledgetypes.TraceSchema{
			JsonSchema: `{"title":"MALICIOUS","description":"attacker-controlled"}`,
		},
	})
	require.NoError(t, err, "handler accepts — this IS the attack surface")

	logs, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{
		Type: knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_SCHEMA_AMEND_TRACE,
	})
	require.NoError(t, err)
	require.Len(t, logs.Actions, 1, "the amendment appears in the privileged-action log")
	require.Equal(t, "TraceSchema@v2", logs.Actions[0].Target)
	require.Equal(t, authority, logs.Actions[0].Invoker)
	require.Greater(t, logs.Actions[0].InvokedAtBlock, uint64(0))

	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "INT-HACK-003",
		Severity:        knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P1,
		Title:           "Trace schema v2 malicious; amend back",
		AffectedModules: []string{"knowledge"},
	})
	require.NoError(t, err)

	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority: authority, ModuleName: knowledgetypes.ModuleName,
		Reason: "INT-HACK-003: halt while sanitising schema", IncidentId: "INT-HACK-003",
	})
	require.NoError(t, err)

	_, err = ms.AmendTraceSchema(h.Ctx, &knowledgetypes.MsgAmendTraceSchema{
		Authority: authority,
		Schema: &knowledgetypes.TraceSchema{
			JsonSchema: `{"title":"MethodologyApplicationTrace","$schema":"https://json-schema.org/draft/2020-12/schema"}`,
			Notes:      "restored after INT-HACK-003",
		},
	})
	require.NoError(t, err)

	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority: authority, IncidentId: "INT-HACK-003",
		Type:      knowledgetypes.RemediationType_REMEDIATION_TYPE_SCHEMA_AMENDMENT,
		Reference: "TraceSchema@v3", Note: "sanitised amendment supersedes malicious v2",
	})
	require.NoError(t, err)
	_, err = ms.UnpauseModule(h.Ctx, &knowledgetypes.MsgUnpauseModule{
		Authority: authority, ModuleName: knowledgetypes.ModuleName,
	})
	require.NoError(t, err)
	_, err = ms.ResolveIncident(h.Ctx, &knowledgetypes.MsgResolveIncident{
		Authority: authority, IncidentId: "INT-HACK-003",
		PostMortemUri: "ipfs://Qm.../INT-HACK-003.md",
	})
	require.NoError(t, err)

	logsAfter, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{
		Type: knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_SCHEMA_AMEND_TRACE,
	})
	require.NoError(t, err)
	require.Len(t, logsAfter.Actions, 2,
		"both the attack amendment and the corrective amendment appear in the log")
	require.Equal(t, "TraceSchema@v2", logsAfter.Actions[0].Target)
	require.Equal(t, "TraceSchema@v3", logsAfter.Actions[1].Target)

	allLogs, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{})
	require.NoError(t, err)
	typesSeen := map[knowledgetypes.PrivilegedActionType]int{}
	for _, a := range allLogs.Actions {
		typesSeen[a.Type]++
	}
	require.Equal(t, 1, typesSeen[knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_INCIDENT_OPEN])
	require.Equal(t, 1, typesSeen[knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_MODULE_PAUSE])
	require.Equal(t, 1, typesSeen[knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_MODULE_UNPAUSE])
	require.Equal(t, 1, typesSeen[knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_INCIDENT_RESOLVE])
	require.Equal(t, 2, typesSeen[knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_SCHEMA_AMEND_TRACE])
}

// ─── Convergence: novel internal attack absorbed by existing surface ───
//
// Authority spams 30 fake P3 incidents to bury a real P0. Severity
// filter on OpenIncidents and the PrivilegedAction log — both
// pre-existing — absorb the attack. Zero new msg types, queries, or
// proto types were required to respond. This is the convergence signal
// for the known internal-attack class.
func TestInternalHackDrill_IncidentSpamAbsorbedByDashboard(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	const N = 30
	for i := 0; i < N; i++ {
		_, err := ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
			Authority: authority, Id: fmtInt("SPAM-", i),
			Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P3,
			Title:    "(spam — attacker burying signal)",
		})
		require.NoError(t, err)
	}

	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "REAL-P0",
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P0,
		Title:    "Critical — actual consensus break",
	})
	require.NoError(t, err)

	realDash, err := qs.OpenIncidents(h.Ctx, &knowledgetypes.QueryOpenIncidentsRequest{
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P0,
	})
	require.NoError(t, err)
	require.Len(t, realDash.Incidents, 1, "P0 severity filter cuts through the spam")
	require.Equal(t, "REAL-P0", realDash.Incidents[0].Id)

	logs, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{
		Type: knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_INCIDENT_OPEN,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(logs.Actions), N+1,
		"log surfaces the entire spam burst alongside the real incident")

	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority: authority, IncidentId: "REAL-P0",
		Type:      knowledgetypes.RemediationType_REMEDIATION_TYPE_DOCUMENTATION,
		Reference: "handled amid spam",
	})
	require.NoError(t, err)
	_, err = ms.ResolveIncident(h.Ctx, &knowledgetypes.MsgResolveIncident{
		Authority: authority, IncidentId: "REAL-P0",
		PostMortemUri: "ipfs://Qm.../REAL-P0.md",
	})
	require.NoError(t, err)

	postResolve, err := qs.OpenIncidents(h.Ctx, &knowledgetypes.QueryOpenIncidentsRequest{
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P0,
	})
	require.NoError(t, err)
	require.Empty(t, postResolve.Incidents)
}

func fmtInt(prefix string, n int) string {
	if n < 10 {
		return prefix + string(rune('0'+n))
	}
	tens := n / 10
	ones := n % 10
	return prefix + string(rune('0'+tens)) + string(rune('0'+ones))
}
