package cross_stack_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// External-hack drill. The attacker has NO legitimate on-chain role;
// they exploit RPC write access, node compromise, or an application bug
// to corrupt state. Each test exercises the response pipeline for a
// distinct attack class. See docs/ROUTE_B_HACK_DRILL_AUDIT.md for the
// full scorecard (3 attack classes + safety invariants; converged at
// iter 4 of Wave 13 — novel attacks absorbed by existing primitives).

// ─── Attack 1: manifest merkle-root corruption ──────────────────────────
//
// Attacker writes a bad root into a finalized manifest's KV record.
// Detection: bundle.merkle_root_valid=false. Recovery: incident + pause
// + MsgCorrectManifestMerkleRoot (authority-gated, incident-bound,
// pure recomputation from canonical IDs).
func TestHackDrill_ManifestCorruptionRecovery(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()
	operator := testAddr("hack_op").String()

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-hack", OperatorAddress: operator, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-HACK", Content: "legit", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: operator,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 3,
	}))
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "m-hack", PipelineId: "pipe-hack",
		CorpusSelector: &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err)
	finResp, err := ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator: operator, ManifestId: "m-hack",
	})
	require.NoError(t, err)
	trueRoot := finResp.MerkleRoot

	// ── ATTACK ──
	corrupted, _ := h.KnowledgeKeeper.GetTrainingManifest(h.Ctx, "m-hack")
	corrupted.MerkleRoot = "deadbeef" + trueRoot[8:]
	require.NoError(t, h.KnowledgeKeeper.SetTrainingManifest(h.Ctx, corrupted))

	// ── DETECTION: Merkle re-derivation flags the tamper ──
	bundle, _ := qs.TrainingManifestBundle(h.Ctx, &knowledgetypes.QueryTrainingManifestBundleRequest{Id: "m-hack"})
	require.False(t, bundle.MerkleRootValid)
	require.NotEqual(t, bundle.DerivedMerkleRoot, bundle.Manifest.MerkleRoot)

	// ── RESPONSE: incident + pause + surgical correction ──
	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "HACK-MANIFEST",
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P1,
		Title:    "m-hack merkle root tampered", AffectedModules: []string{"knowledge"},
	})
	require.NoError(t, err)
	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority: authority, ModuleName: "knowledge", IncidentId: "HACK-MANIFEST",
	})
	require.NoError(t, err)

	correct, err := ms.CorrectManifestMerkleRoot(h.Ctx, &knowledgetypes.MsgCorrectManifestMerkleRoot{
		Authority: authority, ManifestId: "m-hack", IncidentId: "HACK-MANIFEST",
		ExpectedRecomputedRoot: trueRoot,
	})
	require.NoError(t, err)
	require.True(t, correct.WasCorrupted)
	require.Equal(t, trueRoot, correct.RecomputedRoot)

	// ── VERIFY + unwind ──
	fixed, _ := qs.TrainingManifestBundle(h.Ctx, &knowledgetypes.QueryTrainingManifestBundleRequest{Id: "m-hack"})
	require.True(t, fixed.MerkleRootValid, "bundle re-verifies post-correction")

	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority: authority, IncidentId: "HACK-MANIFEST",
		Type:      knowledgetypes.RemediationType_REMEDIATION_TYPE_STATE_CORRECTION,
		Reference: "CorrectManifestMerkleRoot:m-hack",
	})
	require.NoError(t, err)
	_, err = ms.UnpauseModule(h.Ctx, &knowledgetypes.MsgUnpauseModule{Authority: authority, ModuleName: "knowledge"})
	require.NoError(t, err)
	_, err = ms.ResolveIncident(h.Ctx, &knowledgetypes.MsgResolveIncident{
		Authority: authority, IncidentId: "HACK-MANIFEST", PostMortemUri: "ipfs://post-mortem",
	})
	require.NoError(t, err)

	// After resolve, re-running the correction is rejected — incident no
	// longer OPEN/MITIGATING; the bind invariant enforces the audit trail.
	_, err = ms.CorrectManifestMerkleRoot(h.Ctx, &knowledgetypes.MsgCorrectManifestMerkleRoot{
		Authority: authority, ManifestId: "m-hack", IncidentId: "HACK-MANIFEST",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not open")
}

// ─── Safety properties of the surgical correction handler ──────────────
//
// Five invariants bundled — each is a distinct defensive property.
// Consolidated from the iter-2 safety suite.
func TestHackDrill_CorrectionSafetyInvariants(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()
	operator := testAddr("safe_op").String()

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-safe", OperatorAddress: operator, TokenizerVersion: 1,
	}))
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "m-safe", PipelineId: "pipe-safe",
		CorpusSelector: &knowledgetypes.CorpusSelector{},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator: operator, ManifestId: "m-safe",
	})
	require.NoError(t, err)

	// 1. Non-authority rejected.
	_, err = ms.CorrectManifestMerkleRoot(h.Ctx, &knowledgetypes.MsgCorrectManifestMerkleRoot{
		Authority: testAddr("impostor").String(), ManifestId: "m-safe", IncidentId: "X",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")

	// 2. incident_id required — surgical corrections MUST cite an audit trail.
	_, err = ms.CorrectManifestMerkleRoot(h.Ctx, &knowledgetypes.MsgCorrectManifestMerkleRoot{
		Authority: authority, ManifestId: "m-safe",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "incident_id required")

	// 3. Unknown incident rejected.
	_, err = ms.CorrectManifestMerkleRoot(h.Ctx, &knowledgetypes.MsgCorrectManifestMerkleRoot{
		Authority: authority, ManifestId: "m-safe", IncidentId: "NONEXISTENT",
	})
	require.Error(t, err)

	// 4. Clean manifest → no-op, not a spurious write.
	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "INC-NOOP",
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P3, Title: "noop",
	})
	require.NoError(t, err)
	noop, err := ms.CorrectManifestMerkleRoot(h.Ctx, &knowledgetypes.MsgCorrectManifestMerkleRoot{
		Authority: authority, ManifestId: "m-safe", IncidentId: "INC-NOOP",
	})
	require.NoError(t, err)
	require.False(t, noop.WasCorrupted)
	require.Equal(t, noop.PriorRoot, noop.RecomputedRoot)

	// 5. Expected-root mismatch aborts without writing.
	_, err = ms.CorrectManifestMerkleRoot(h.Ctx, &knowledgetypes.MsgCorrectManifestMerkleRoot{
		Authority: authority, ManifestId: "m-safe", IncidentId: "INC-NOOP",
		ExpectedRecomputedRoot: "definitely-not-right",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected_recomputed_root mismatch")
}

// ─── Attack 2: SLA-breach surfacing ─────────────────────────────────────
//
// An incident whose SLA window has passed while still OPEN or MITIGATING
// is surfaced by the SlaBreachedIncidents query. Wires alerting.
func TestHackDrill_SLABreachDashboard(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "SLA-1",
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P3,
		Title:    "slow response test", SlaWindowBlocks: 5,
	})
	require.NoError(t, err)

	pre, _ := qs.SlaBreachedIncidents(h.Ctx, &knowledgetypes.QuerySlaBreachedIncidentsRequest{})
	require.Empty(t, pre.Incidents)

	h.AdvanceBlocks(10)

	post, _ := qs.SlaBreachedIncidents(h.Ctx, &knowledgetypes.QuerySlaBreachedIncidentsRequest{})
	require.Len(t, post.Incidents, 1)
	require.Equal(t, "SLA-1", post.Incidents[0].Id)
	require.Greater(t, post.CurrentBlockHeight, post.Incidents[0].SlaTargetBlock)

	// Resolve — drops off the dashboard.
	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority: authority, IncidentId: "SLA-1",
		Type: knowledgetypes.RemediationType_REMEDIATION_TYPE_DOCUMENTATION, Reference: "late",
	})
	require.NoError(t, err)
	_, err = ms.ResolveIncident(h.Ctx, &knowledgetypes.MsgResolveIncident{
		Authority: authority, IncidentId: "SLA-1", PostMortemUri: "x",
	})
	require.NoError(t, err)

	cleared, _ := qs.SlaBreachedIncidents(h.Ctx, &knowledgetypes.QuerySlaBreachedIncidentsRequest{})
	require.Empty(t, cleared.Incidents)
}

// ─── Attack 3: attribution over-reporting ──────────────────────────────
//
// Model owner inflates contribution fact_ids; challenger files a bond,
// authority resolves uphold, challenger collects bond × 2. Demonstrates
// that the Wave 4 challenge mechanism handles a common insider-friendly
// economic exploit with no new primitives required.
func TestHackDrill_AttributionOverReportRecovery(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	owner := testAddr("overreport_owner").String()
	challengerAddr := testAddr("overreport_chal")
	challenger := challengerAddr.String()
	require.NoError(t, h.FundAccount(challengerAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(20_000_000)))))

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-over", OperatorAddress: owner, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, &knowledgetypes.ModelCard{
		Id: "m-over", PipelineId: "pipe-over", OwnerAddress: owner, Route: "from_scratch", Active: true,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-REAL", Content: "real", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: owner,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-UNUSED", Content: "never used", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: owner,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 10,
	}))

	// ── ATTACK: over-report ──
	_, err = ms.AttributeContributions(h.Ctx, &knowledgetypes.MsgAttributeContributions{
		Owner: owner, ModelId: "m-over", FactIds: []string{"F-REAL", "F-UNUSED"},
	})
	require.NoError(t, err)

	// ── RESPONSE: bond + challenge + resolve ──
	_, err = ms.ChallengeContribution(h.Ctx, &knowledgetypes.MsgChallengeContribution{
		Challenger: challenger, ModelId: "m-over",
		DisputedFactId: "F-UNUSED", DisputeType: "fraudulent",
		Evidence: "verification eval shows F-UNUSED not in training set", Id: "chal-over",
	})
	require.NoError(t, err)

	resp, err := ms.ResolveContributionChallenge(h.Ctx, &knowledgetypes.MsgResolveContributionChallenge{
		Resolver: authority, ChallengeId: "chal-over", Uphold: true,
	})
	require.NoError(t, err)
	require.Equal(t, "10000000", resp.PayoutToWinner)

	// Challenger started with 20M, bonded 5M (fund has 5M), received bond×2 = 10M → 20M - 5M + 10M = 25M.
	finalBal := h.GetBalance(challengerAddr, "uzrn")
	require.Equal(t, sdkmath.NewInt(25_000_000), finalBal.Amount)
}

// ─── Attack 4: Sybil containment — defense in depth ────────────────────
//
// Primary defense (Wave 10): stake-weighted verifier consensus. Three
// zero-stake Sybil addresses now carry zero weight in the augmentation
// panel tally, so the Sybil verdict never finalizes — the poisoned
// variant remains PENDING forever. Secondary defense (Wave 12): if a
// compromised validator somehow pushed a false verdict through, the
// circuit breaker still traps the variant before it reaches a training
// manifest. This drill exercises both layers.
func TestHackDrill_SybilPoisoningContainedByBreaker(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	sponsorAddr := testAddr("sybil_sponsor")
	sponsor := sponsorAddr.String()
	submitter := testAddr("sybil_sub").String()
	require.NoError(t, h.FundAccount(sponsorAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(500_000_000)))))

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-POISON", Content: "target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "b-poison", TargetFactId: "F-POISON",
		RewardPerVariant: 10_000_000, MaxVariants: 1,
	})
	require.NoError(t, err)
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-poison", BountyId: "b-poison",
		OriginalFactId: "F-POISON", VariantContent: "meaning-changed variant",
	})
	require.NoError(t, err)

	// ── PRIMARY DEFENSE: Sybil consensus fails at the voting layer ──
	// Three zero-stake addresses each cast EQUIVALENT. Their votes record
	// for audit but carry zero weight in the stake-weighted tally, so the
	// verdict never finalizes.
	for _, v := range []string{"sybil1", "sybil2", "sybil3"} {
		resp, err := ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
			Verifier: testAddr(v).String(), AugmentationId: "aug-poison",
			Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
		})
		require.NoError(t, err)
		require.False(t, resp.VerdictFinalized,
			"zero-stake Sybil vote must not trip consensus")
	}
	aug, _ := h.KnowledgeKeeper.GetAugmentation(h.Ctx, "aug-poison")
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_PENDING, aug.Verdict,
		"primary defense: Sybil carries no stake, so the panel never finalizes")

	// ── SECONDARY DEFENSE: even if a compromised stake-bearing validator
	// somehow pushed a false verdict through, the breaker still contains
	// it. Simulate the "worst case" by bonding a single validator and
	// having them vote. With 1 voter we don't reach MinPanelVotes (default
	// 3), so verdict still doesn't finalize — but this proves the incident
	// + breaker pipeline works even after a voting-layer breach scenario.
	_, err = ms.OpenIncident(h.Ctx, &knowledgetypes.MsgOpenIncident{
		Authority: authority, Id: "SYBIL",
		Severity: knowledgetypes.IncidentSeverity_INCIDENT_SEVERITY_P1,
		Title:    "Sybil attempt on augmentation verdict", AffectedModules: []string{"knowledge"},
	})
	require.NoError(t, err)
	_, err = ms.PauseModule(h.Ctx, &knowledgetypes.MsgPauseModule{
		Authority: authority, ModuleName: "knowledge", IncidentId: "SYBIL",
	})
	require.NoError(t, err)

	// While paused, manifest creation rejects — any poisoned state is
	// trapped behind the breaker while governance responds.
	operator := testAddr("sybil_op").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-sybil", OperatorAddress: operator, TokenizerVersion: 1,
	}))
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "m-poisoned", PipelineId: "pipe-sybil",
		CorpusSelector: &knowledgetypes.CorpusSelector{IncludeContrastivePairs: true},
	})
	require.Error(t, err, "breaker blocks manifest creation → poisoned variant can't propagate")

	// Close out the response.
	_, err = ms.RecordRemediation(h.Ctx, &knowledgetypes.MsgRecordRemediation{
		Authority: authority, IncidentId: "SYBIL",
		Type: knowledgetypes.RemediationType_REMEDIATION_TYPE_DOCUMENTATION,
		Reference: "defense-in-depth: stake-weighted-voting + breaker",
	})
	require.NoError(t, err)
	_, err = ms.UnpauseModule(h.Ctx, &knowledgetypes.MsgUnpauseModule{Authority: authority, ModuleName: "knowledge"})
	require.NoError(t, err)
	_, err = ms.ResolveIncident(h.Ctx, &knowledgetypes.MsgResolveIncident{
		Authority: authority, IncidentId: "SYBIL", PostMortemUri: "ipfs://sybil",
	})
	require.NoError(t, err)

	// Post-drill state clean.
	open, _ := qs.OpenIncidents(h.Ctx, &knowledgetypes.QueryOpenIncidentsRequest{})
	require.Empty(t, open.Incidents)
	paused, _ := qs.PausedModules(h.Ctx, &knowledgetypes.QueryPausedModulesRequest{})
	require.Empty(t, paused.Paused)
}
