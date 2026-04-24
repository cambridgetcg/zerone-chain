package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Wave 14: internal-hack recursive drill ─────────────────────────────
//
// External drill (Wave 13) assumed the attacker had NO on-chain privileges.
// This drill flips that: the attacker IS a privileged role — most
// commonly a compromised governance authority. The damage vector is no
// longer "break into the chain" but "abuse legitimate powers".
//
// Each iteration simulates a concrete abuse pattern, runs the response,
// identifies friction, and hardens the mechanism. Convergence target
// matches Wave 13: a novel abuse completes with zero new primitives.

// ─── Iteration 1: authority weaponises MsgPauseModule ──────────────────
//
// ATTACK:
//   The governance authority key is compromised (or the authority
//   entity is acting maliciously). The attacker calls MsgPauseModule
//   with auto_unpause_at_block=0 — an INDEFINITE pause. The knowledge
//   module's write-path is blocked until someone with authority
//   unpauses. If the attacker also refuses to unpause, the chain is
//   effectively censored: no manifests can be finalised, no training
//   runs can complete, no incident corrections can fire.
//
// EXPECTED ITER-1 GAP:
//   MsgPauseModule accepts auto_unpause_at_block=0, meaning a malicious
//   authority can hold the breaker open forever. There is no maximum
//   pause duration enforced at the handler level. Recovery requires
//   authority rotation — out of scope for this chain.
//
// RESPONSE (as implemented in iter 1):
//   The test documents the attack succeeding indefinitely. The iter-2
//   fix below caps pause duration at the handler level, so even a
//   compromised authority's pause self-expires within a governance-set
//   window.
func TestInternalHackDrill_Iter1_AuthorityPauseIndefinite(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	// ── ATTACK: indefinite pause (auto_unpause_at_block=0) ──
	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:          authority,
		ModuleName:         knowledgetypes.ModuleName,
		Reason:             "(attacker lies about reason)",
		AutoUnpauseAtBlock: 0, // indefinite
	})
	require.NoError(t, err, "handler accepts indefinite pause — iter-1 gap")

	// Advance blocks. The pause persists forever — demonstrated with a short
	// window here for test speed; the property holds for arbitrary duration.
	h.AdvanceBlocks(50)
	require.True(t, h.KnowledgeKeeper.IsModulePaused(h.Ctx, knowledgetypes.ModuleName),
		"iter-1 gap: indefinite pause never self-expires; authority-compromise = permanent DoS")

	// ITER-1 FRICTION POINTS:
	// 1. [CRITICAL] auto_unpause_at_block=0 means indefinite. Compromised
	//    authority = indefinite chain censorship with no self-recovery.
	// 2. [MEDIUM] no queryable record of "who issued this pause and when"
	//    beyond the pause record itself — the community can see the
	//    module is paused but must trawl block history for the authoring
	//    tx. No unified PrivilegedAction log yet.
	// 3. [MINOR] pause reason field is unstructured; a malicious authority
	//    can put arbitrary text. Not a critical gap but an honest-actor
	//    convention can be documented.
}

// ─── Iteration 2: MaxPauseDurationBlocks cap ────────────────────────────
//
// SAME ATTACK as iter 1 — malicious authority calls PauseModule with
// auto_unpause_at_block=0 intending indefinite.
//
// NEW PROPERTY (iter 2):
//   Every pause is capped at Params.MaxPauseDurationBlocks from pause-
//   time. The handler silently rewrites auto_unpause_at_block=0 (or any
//   value beyond the cap) to now+cap. The compromised authority's DoS
//   self-expires within the governance window; honest operators can
//   still set shorter windows for maintenance.
//
// This changes the threat model: a compromised authority can pause a
// module, but only temporarily. Permanent censorship via the breaker is
// structurally impossible.
func TestInternalHackDrill_Iter2_PauseCapped(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Shrink the cap so the test finishes in a reasonable time.
	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	params.MaxPauseDurationBlocks = 10
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	// ATTACK: "indefinite" pause.
	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority:          authority,
		ModuleName:         knowledgetypes.ModuleName,
		Reason:             "(attacker's indefinite-pause attempt)",
		AutoUnpauseAtBlock: 0, // requests indefinite; handler rewrites.
	})
	require.NoError(t, err)

	// The handler stamped a bounded expiry at paused_at + cap.
	pauseRec, ok := h.KnowledgeKeeper.GetModulePause(h.Ctx, knowledgetypes.ModuleName)
	require.True(t, ok)
	require.Equal(t, pauseRec.PausedAtBlock+10, pauseRec.AutoUnpauseAtBlock,
		"handler capped the window to paused_at_block + MaxPauseDurationBlocks")

	// Advance past the cap; pause self-expires on next IsModulePaused read.
	h.AdvanceBlocks(15)
	require.False(t, h.KnowledgeKeeper.IsModulePaused(h.Ctx, knowledgetypes.ModuleName),
		"pause self-expired at the cap — permanent censorship via breaker is impossible")
}

// Iter 2 also verifies: a caller-supplied window beyond the cap is truncated.
func TestInternalHackDrill_Iter2_OverlongWindowTruncated(t *testing.T) {
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
		AutoUnpauseAtBlock: heightAtPause + 10_000_000, // way beyond cap
	})
	require.NoError(t, err)
	rec, _ := h.KnowledgeKeeper.GetModulePause(h.Ctx, knowledgetypes.ModuleName)
	require.Equal(t, heightAtPause+20, rec.AutoUnpauseAtBlock,
		"overlong window is truncated to cap — caller cannot bypass")
}

// Honest-actor path unchanged: a short window within the cap is respected.
func TestInternalHackDrill_Iter2_HonestShortWindowRespected(t *testing.T) {
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
		AutoUnpauseAtBlock: heightAtPause + 5, // honest short window
	})
	require.NoError(t, err)
	rec, _ := h.KnowledgeKeeper.GetModulePause(h.Ctx, knowledgetypes.ModuleName)
	require.Equal(t, heightAtPause+5, rec.AutoUnpauseAtBlock,
		"short windows within the cap are honored unchanged")
}

// ─── Iteration 3: PrivilegedActionLog + schema-amend weaponisation ─────
//
// ATTACK:
//   Compromised authority amends the TraceSchema to a subtly-malicious
//   version — e.g., adds a field that downstream training pipelines
//   interpret as trusted but actually points to attacker-controlled
//   content. The amendment looks legitimate on the wire (it IS a
//   well-formed MsgAmendTraceSchema call) but is an abuse of power.
//
//   The old mitigation surface was "watch the event log"; easy to miss
//   amid other activity. Iter 3 adds a structured, queryable
//   PrivilegedAction log so the community can review authority activity
//   in one pane.
//
// RESPONSE:
//   1. External monitor runs PrivilegedActions query periodically.
//   2. Sees a SCHEMA_AMEND_TRACE entry at an unexpected block.
//   3. Community inspects the new schema; finds it malicious.
//   4. Opens P1 incident.
//   5. Pauses the module (Wave 12).
//   6. Authority amends the schema BACK to a sanitised version. (This
//      uses the same mechanism the attacker used — the cure is the
//      same procedure, only enacted by honest governance after the
//      attack is surfaced. Note: a truly-captured authority is a
//      different threat class — see audit doc.)
//   7. Unpause; resolve incident.
//
// ITER-3 GAP OBSERVED:
//   The log is on-chain, queryable, machine-parseable. Every privileged
//   action appears. Detection lag is now the query-polling interval of
//   the monitor — not operator trawl time. This is the property that
//   was missing in iter 2.
func TestInternalHackDrill_Iter3_SchemaAmendmentDetection(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	// ── ATTACK: malicious TraceSchema amendment ──
	_, err = ms.AmendTraceSchema(h.Ctx, &knowledgetypes.MsgAmendTraceSchema{
		Authority: authority,
		Schema: &knowledgetypes.TraceSchema{
			JsonSchema: `{"title":"MALICIOUS","description":"attacker-controlled"}`,
		},
	})
	require.NoError(t, err, "handler accepts — this IS the attack surface")

	// ── DETECTION via PrivilegedActions query ──
	logs, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{
		Type: knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_SCHEMA_AMEND_TRACE,
	})
	require.NoError(t, err)
	require.Len(t, logs.Actions, 1, "the amendment appears in the privileged-action log")
	require.Equal(t, "TraceSchema@v2", logs.Actions[0].Target)
	require.Equal(t, authority, logs.Actions[0].Invoker)
	require.Greater(t, logs.Actions[0].InvokedAtBlock, uint64(0))

	// ── RESPONSE ──
	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "INT-HACK-003",
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P1,
		Title:    "Trace schema v2 malicious; amend back",
		AffectedModules: []string{"knowledge"},
	})
	require.NoError(t, err)

	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority: authority, ModuleName: knowledgetypes.ModuleName,
		Reason: "INT-HACK-003: halt while sanitising schema", IncidentId: "INT-HACK-003",
	})
	require.NoError(t, err)

	// Amend BACK to a sanitised version (same msg type, honest content).
	_, err = ms.AmendTraceSchema(h.Ctx, &knowledgetypes.MsgAmendTraceSchema{
		Authority: authority,
		Schema: &knowledgetypes.TraceSchema{
			JsonSchema: `{"title":"MethodologyApplicationTrace","$schema":"https://json-schema.org/draft/2020-12/schema"}`,
			Notes:      "restored after INT-HACK-003",
		},
	})
	require.NoError(t, err)

	// Record remediation + unpause + resolve.
	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority: authority, IncidentId: "INT-HACK-003",
		Type: knowledgetypes.RemediationType_REMEDIATION_TYPE_SCHEMA_AMENDMENT,
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

	// ── The log now shows BOTH amendments — full audit trail ──
	logsAfter, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{
		Type: knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_SCHEMA_AMEND_TRACE,
	})
	require.NoError(t, err)
	require.Len(t, logsAfter.Actions, 2,
		"both the attack amendment and the corrective amendment appear in the log")
	require.Equal(t, "TraceSchema@v2", logsAfter.Actions[0].Target)
	require.Equal(t, "TraceSchema@v3", logsAfter.Actions[1].Target)

	// Log also captures the incident/pause/resolve actions.
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

// ─── Iteration 4: convergence — novel internal attack, zero new code ───
//
// ATTACK:
//   Compromised authority spams 30 fake P3 incidents to bury a real P0
//   in the OpenIncidents dashboard. Responders get distracted filtering;
//   mean-time-to-attention for the real incident balloons.
//
// EXPECTED RESPONSE (should use only existing primitives):
//   1. PrivilegedActions query — filtered by INCIDENT_OPEN, grouped by
//      invoker+block — surfaces the anomalous burst. (Existing primitive.)
//   2. Responders filter OpenIncidents by severity=P0 to focus. (Existing
//      primitive — the severity filter shipped in Wave 11.)
//   3. Real incident is surfaced. Response proceeds. Fake incidents are
//      closed (either CloseIncident after formal resolve, or ignored and
//      aged out).
//
// CONVERGENCE CHECK:
//   This test must pass with ZERO new handlers, ZERO new query surface,
//   ZERO new proto types. If it does, the internal-hack drill has
//   converged for the known attack class.
func TestInternalHackDrill_Iter4_ConvergenceIncidentSpam(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	// ── ATTACK: spam 30 fake P3 incidents ──
	const N = 30
	for i := 0; i < N; i++ {
		_, err := ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
			Authority: authority, Id: fmtInt("SPAM-", i),
			Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P3,
			Title:    "(spam — attacker burying signal)",
		})
		require.NoError(t, err)
	}

	// ── LEGITIMATE INCIDENT filed amid the noise ──
	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "REAL-P0",
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P0,
		Title:    "Critical — actual consensus break",
	})
	require.NoError(t, err)

	// ── RESPONSE via existing primitives ──

	// Step 1: severity filter on OpenIncidents isolates the real P0.
	realDash, err := qs.OpenIncidents(h.Ctx, &knowledgetypes.QueryOpenIncidentsRequest{
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P0,
	})
	require.NoError(t, err)
	require.Len(t, realDash.Incidents, 1, "P0 severity filter cuts through the spam")
	require.Equal(t, "REAL-P0", realDash.Incidents[0].Id)

	// Step 2: PrivilegedActions shows the spam pattern — N+1 INCIDENT_OPENs
	// from the same invoker in the same block. An indexer or monitor
	// running anomaly detection on this log would flag it.
	logs, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{
		Type: knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_INCIDENT_OPEN,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(logs.Actions), N+1,
		"log surfaces the entire spam burst alongside the real incident")

	// Step 3: handle the real incident normally.
	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority: authority, IncidentId: "REAL-P0",
		Type: knowledgetypes.RemediationType_REMEDIATION_TYPE_DOCUMENTATION,
		Reference: "handled amid spam",
	})
	require.NoError(t, err)
	_, err = ms.ResolveIncident(h.Ctx, &knowledgetypes.MsgResolveIncident{
		Authority: authority, IncidentId: "REAL-P0",
		PostMortemUri: "ipfs://Qm.../REAL-P0.md",
	})
	require.NoError(t, err)

	// Step 4: the P0 dashboard now clears.
	postResolve, err := qs.OpenIncidents(h.Ctx, &knowledgetypes.QueryOpenIncidentsRequest{
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P0,
	})
	require.NoError(t, err)
	require.Empty(t, postResolve.Incidents)

	// ── CONVERGENCE ASSERTION ──
	// This test used: OpenIncident, OpenIncidents-with-severity-filter,
	// PrivilegedActions, RecordRemediation, ResolveIncident — all
	// primitives that existed before iter 4. No new msg type, no new
	// query, no new proto types were required. The attack was absorbed
	// by the existing response surface.
}

// fmtInt formats a spam-incident id — tiny helper to avoid fmt-import dance.
func fmtInt(prefix string, n int) string {
	// Small N; handrolled to avoid pulling strconv.
	if n < 10 {
		return prefix + string(rune('0'+n))
	}
	tens := n / 10
	ones := n % 10
	return prefix + string(rune('0'+tens)) + string(rune('0'+ones))
}
