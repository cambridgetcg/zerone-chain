package cross_stack_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// Route B — Adversarial Audit
//
// Each test targets a named valuable structure with a concrete attack and
// asserts the chain's defensive behaviour. These are the tests that
// demonstrate how the system *absorbs adversarial input*, not basic
// state-machine wiring (covered by wave-specific suites).
//
// Taxonomy:
//   - Manifest / Merkle layer (reproducibility promise)
//   - Economic layer (escrow, bounties, clawback)
//   - Is-ought wall + TVW (epistemic integrity)
//   - Verifier panel (adjudication integrity)
//   - Heartbeat (automation safety)

// ─── ATTACK 1: Composition-depth bomb ────────────────────────────────────
//
// A chain of parent manifests deeper than the bound must reject.
// Without this, a malicious operator could craft a DAG so deep that
// bundle assembly becomes a DoS vector.
func TestRouteB_Adversarial_CompositionDepthBomb(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	operator := testAddr("adv_depth_op").String()

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-9d", OperatorAddress: operator, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-9D", Content: "depth target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: operator,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 3,
	}))

	const maxAllowed = 8
	prev := ""
	for i := 0; i < maxAllowed; i++ {
		id := fmt.Sprintf("m-depth-%d", i)
		msg := &knowledgetypes.MsgCreateTrainingManifest{
			Creator: operator, Id: id, PipelineId: "pipe-9d",
			ParentManifestId: prev,
			CorpusSelector: &knowledgetypes.CorpusSelector{
				MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3,
			},
		}
		_, err := ms.CreateTrainingManifest(h.Ctx, msg)
		require.NoError(t, err, "depth=%d should succeed", i)
		_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
			Creator: operator, ManifestId: id,
		})
		require.NoError(t, err)
		prev = id
	}

	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "m-depth-bomb", PipelineId: "pipe-9d",
		ParentManifestId: prev,
		CorpusSelector:   &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical},
	})
	require.Error(t, err, "composition beyond max depth must be rejected")
	require.Contains(t, err.Error(), "max depth")
}

// ─── ATTACK 2: Parent must be FINALIZED or ATTESTED ──────────────────────
//
// A DRAFT parent cannot be referenced. Prevents cycle construction: a
// cycle would require a DRAFT pointing back into the tree, which isn't
// possible because parents must be sealed first.
func TestRouteB_Adversarial_ParentMustBeSealed(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	operator := testAddr("adv_cycle_op").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-cycle", OperatorAddress: operator, TokenizerVersion: 1,
	}))

	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "draft-parent", PipelineId: "pipe-cycle",
		CorpusSelector: &knowledgetypes.CorpusSelector{},
	})
	require.NoError(t, err)

	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "child-of-draft", PipelineId: "pipe-cycle",
		ParentManifestId: "draft-parent",
		CorpusSelector:   &knowledgetypes.CorpusSelector{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "FINALIZED or ATTESTED")
}

// ─── ATTACK 3: Composed-Merkle collision across (parent, delta) pairs ────
//
// Different parents plus/or swapped delta content must yield distinct
// composed roots. Domain separation between PARENT: and DELTA: segments +
// length prefixes guarantee this.
func TestRouteB_Adversarial_ComposedMerkleCollisionResistance(t *testing.T) {
	ids1 := knowledgekeeper.SelectedManifestIDs{FactIDs: []string{"A", "B"}}
	ids2 := knowledgekeeper.SelectedManifestIDs{FactIDs: []string{"C"}}

	parentRoot1 := knowledgekeeper.ComputeManifestMerkleRoot(knowledgekeeper.SelectedManifestIDs{
		FactIDs: []string{"P1"},
	})
	parentRoot2 := knowledgekeeper.ComputeManifestMerkleRoot(knowledgekeeper.SelectedManifestIDs{
		FactIDs: []string{"P2"},
	})

	c1 := knowledgekeeper.ComputeComposedManifestMerkleRoot(parentRoot1, ids1)
	c2 := knowledgekeeper.ComputeComposedManifestMerkleRoot(parentRoot2, ids1)
	c3 := knowledgekeeper.ComputeComposedManifestMerkleRoot(parentRoot1, ids2)

	require.NotEqual(t, c1, c2, "different parents must yield different composed roots")
	require.NotEqual(t, c1, c3, "same parent + different delta must yield different composed roots")
	require.NotEqual(t, c2, c3, "both dims different must yield different composed roots")

	flat := knowledgekeeper.ComputeManifestMerkleRoot(ids1)
	require.NotEqual(t, c1, flat,
		"composed commitment must be distinguishable from a flat commitment over the same delta")
}

// ─── ATTACK 4: Child remains verifiable after parent supersession ───────
//
// Parent gets superseded by a newer manifest for the same pipeline. The
// child (which snapshotted parent_merkle_root at create time) must still
// re-verify correctly. Supersession is a pointer change; the child's
// commitment is immutable.
func TestRouteB_Adversarial_ChildSurvivesParentSupersession(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	operator := testAddr("adv_super_op").String()

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-super-test", OperatorAddress: operator, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-9S", Content: "p", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: operator,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 3,
	}))

	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "parent", PipelineId: "pipe-super-test",
		CorpusSelector: &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: operator, ManifestId: "parent"})
	require.NoError(t, err)

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-9S-CHILD", Content: "delta", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: operator,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 3,
	}))
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "child", PipelineId: "pipe-super-test",
		ParentManifestId: "parent",
		CorpusSelector:   &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: operator, ManifestId: "child"})
	require.NoError(t, err)

	h.AdvanceBlocks(1)
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "parent-v2", PipelineId: "pipe-super-test",
		CorpusSelector: &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: operator, ManifestId: "parent-v2"})
	require.NoError(t, err)
	h.AdvanceBlocks(1)

	bundle, err := qs.TrainingManifestBundle(h.Ctx, &knowledgetypes.QueryTrainingManifestBundleRequest{Id: "child"})
	require.NoError(t, err)
	require.True(t, bundle.MerkleRootValid,
		"child's composed Merkle root must remain valid even after parent's status changes")
}

// ─── ATTACK 5: Augmentation-of-augmentation blocked ─────────────────────
//
// An Augmentation's original_fact_id must resolve to a Fact, not another
// Augmentation. Recursive variant-of-variant would bypass methodology
// adjudication grounded on the ORIGINAL fact.
func TestRouteB_Adversarial_NoAugmentationOfAugmentation(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	operator := testAddr("adv_augaug").String()

	require.NoError(t, h.KnowledgeKeeper.SetAugmentation(h.Ctx, &knowledgetypes.Augmentation{
		Id: "AUG-9", OriginalFactId: "F-ORIG", VariantContent: "x", Submitter: operator,
	}))

	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: operator, Id: "AUG-NESTED",
		OriginalFactId: "AUG-9",
		VariantContent: "nested variant",
	})
	require.Error(t, err,
		"augmentation-of-augmentation must reject (no Fact with id AUG-9)")
	require.Contains(t, err.Error(), "not found")
}

// ─── ATTACK 6: Is-ought wall smuggling ──────────────────────────────────
//
// A model owner lists a NormativeCommitment ID as a trained-on fact_id.
// The chain must reject it (not smuggle it as a quiet ought-claim into
// training revenue). Duplicates in the input are deduped in the rejection
// count — the count reflects unique commitments, not repeat attempts.
func TestRouteB_Adversarial_IsOughtSmuggling(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	owner := testAddr("adv_smuggle").String()

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-smug", OperatorAddress: owner, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, &knowledgetypes.ModelCard{
		Id: "model-smug", PipelineId: "pipe-smug", OwnerAddress: owner,
		Route: "from_scratch", Active: true,
	}))

	commitments := h.KnowledgeKeeper.GetAllNormativeCommitments(h.Ctx)
	require.NotEmpty(t, commitments)
	commitID := commitments[0].Id

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-REAL", Content: "real", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: owner,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))

	resp, err := ms.AttributeContributions(h.Ctx, &knowledgetypes.MsgAttributeContributions{
		Owner: owner, ModelId: "model-smug",
		FactIds: []string{"F-REAL", commitID, commitID, "F-REAL"},
	})
	require.NoError(t, err, "handler succeeds but filters out commitments")
	require.Equal(t, uint32(1), resp.Recorded,
		"only the real fact is recorded; 2 commitment attempts filtered")

	rec, _ := h.KnowledgeKeeper.GetContributionRecord(h.Ctx, "model-smug")
	require.NotNil(t, rec)
	require.NotContains(t, rec.FactIds, commitID,
		"commitment ID must not appear in recorded fact_ids")
	require.Equal(t, uint32(1), rec.RejectedCommitmentCount,
		"rejected count reflects ONE unique commitment, not two duplicates")
}

// ─── ATTACK 7: Clawback stickiness ──────────────────────────────────────
//
// Once a fact's revenue is clawed back (disproved), TVW stays 0 even if
// the status flips back to ACTIVE. The revenue_clawback_block stamp is
// sticky; a second clawback is idempotent.
func TestRouteB_Adversarial_ClawbackStickyAndIdempotent(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	f := &knowledgetypes.Fact{
		Id: "F-STICKY", Content: "p", Domain: "sciences",
		Status:    knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: testAddr("adv_stick").String(),
		MethodId:  knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000, CorroborationCount: 5,
		SubmitterCalibrationSnapshotBps: 800_000,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, f))

	pre, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: "F-STICKY"})
	require.NoError(t, err)
	require.Greater(t, pre.TvwBps, uint64(0))

	require.NoError(t, h.KnowledgeKeeper.ClawbackOnDisproval(h.Ctx, "F-STICKY"))
	first, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-STICKY")
	firstBlock := first.RevenueClawbackBlock
	require.Greater(t, firstBlock, uint64(0))

	// Status flip back to ACTIVE must not reset TVW.
	recovered, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-STICKY")
	recovered.Status = knowledgetypes.FactStatus_FACT_STATUS_ACTIVE
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, recovered))

	post, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: "F-STICKY"})
	require.NoError(t, err)
	require.Equal(t, uint64(0), post.TvwBps,
		"clawback stays sticky: revenue_clawback_block survives status flip")
	require.True(t, post.Disproven)

	// Second clawback — idempotent, block stamp unchanged.
	require.NoError(t, h.KnowledgeKeeper.ClawbackOnDisproval(h.Ctx, "F-STICKY"))
	second, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-STICKY")
	require.Equal(t, firstBlock, second.RevenueClawbackBlock,
		"double-clawback is idempotent")
}

// ─── ATTACK 8: Sponsor cannot self-finalize via AcceptAugmentation ─────
//
// Wave 4 invariant: the ONLY bounty-acceptance path is a finalized passing
// verdict from the verifier panel. A sponsor calling AcceptAugmentation
// on a pre-verdict bounty variant must be rejected — otherwise sponsors
// could accept their own pet variants without verifier scrutiny.
func TestRouteB_Adversarial_SponsorSelfFinalizeBlocked(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	sponsorAddr := testAddr("adv_sponsor")
	sponsor := sponsorAddr.String()
	submitter := testAddr("adv_sub").String()
	require.NoError(t, h.FundAccount(sponsorAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-SPON", Content: "p", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))

	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "b-spon", TargetFactId: "F-SPON",
		RewardPerVariant: 1_000_000, MaxVariants: 1,
	})
	require.NoError(t, err)
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-spon", BountyId: "b-spon",
		OriginalFactId: "F-SPON", VariantContent: "v",
	})
	require.NoError(t, err)

	_, err = ms.AcceptAugmentation(h.Ctx, &knowledgetypes.MsgAcceptAugmentation{
		Acceptor: sponsor, AugmentationId: "aug-spon",
	})
	require.Error(t, err, "sponsor may not self-accept bounty variants under Wave 4")
	require.Contains(t, err.Error(), "verifier-panel")
}

// ─── ATTACK 9: Verifier Sybil on augmentation panel — CLOSED ──────────
//
// The Wave 9 audit documented this as a known gap: a single actor
// controlling three zero-stake addresses could push DRIFT as EQUIVALENT
// because consensus was raw headcount. Wave 10 closed it by making
// consensus stake-weighted. Sybil addresses with no bonded stake now
// carry zero weight and cannot finalize a verdict regardless of how
// many of them agree. This test confirms the gap stays closed —
// three zero-stake votes do NOT reach consensus.
func TestRouteB_Adversarial_ZeroStakeSybilCannotFinalize(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	sponsorAddr := testAddr("adv_sybil_s")
	sponsor := sponsorAddr.String()
	submitter := testAddr("adv_sybil_sub").String()
	require.NoError(t, h.FundAccount(sponsorAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-SYBIL", Content: "truth target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "b-sybil", TargetFactId: "F-SYBIL",
		RewardPerVariant: 1_000_000, MaxVariants: 1,
	})
	require.NoError(t, err)
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-sybil", BountyId: "b-sybil",
		OriginalFactId: "F-SYBIL", VariantContent: "a variant that actually drifts meaning",
	})
	require.NoError(t, err)

	// Three zero-stake Sybil addresses each cast a vote. The chain
	// records the votes (dissent is always allowed) but they carry zero
	// weight in the stake-weighted tally, so no verdict finalizes.
	for _, v := range []string{"adv_sybil_v1", "adv_sybil_v2", "adv_sybil_v3"} {
		resp, err := ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
			Verifier: testAddr(v).String(), AugmentationId: "aug-sybil",
			Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
		})
		require.NoError(t, err, "the vote is recorded even though it carries no weight")
		require.False(t, resp.VerdictFinalized,
			"zero-stake Sybil votes must not finalize a verdict")
	}

	aug, found := h.KnowledgeKeeper.GetAugmentation(h.Ctx, "aug-sybil")
	require.True(t, found)
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_PENDING, aug.Verdict,
		"verdict must stay PENDING — Sybil carries no stake, no consensus")
	require.False(t, aug.Accepted, "payout does not fire without stake-weighted consensus")
	require.Len(t, aug.VerdictVoteStakes, 3, "each vote recorded with its zero stake at vote time")
	for _, w := range aug.VerdictVoteStakes {
		require.Equal(t, uint64(0), w, "zero-stake Sybil addresses weigh zero")
	}
}

// ─── ATTACK 10: Heartbeat withstands many expiring bounties ─────────────
//
// Create N bounties that all expire in the same block; verify the
// heartbeat processes all of them without failing or stalling. Probes the
// unbounded-scan risk.
func TestRouteB_Adversarial_HeartbeatScalesToManyBounties(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	sponsorAddr := testAddr("adv_mass_sponsor")
	sponsor := sponsorAddr.String()
	require.NoError(t, h.FundAccount(sponsorAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(10_000_000_000)))))

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-MASS", Content: "target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))

	const N = 50
	expiryBlock := uint64(h.Height() + 3)
	for i := 0; i < N; i++ {
		_, err := ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
			Sponsor: sponsor, Id: fmt.Sprintf("b-mass-%d", i), TargetFactId: "F-MASS",
			RewardPerVariant: 100_000, MaxVariants: 1, ExpiresAtBlock: expiryBlock,
		})
		require.NoError(t, err)
	}

	h.AdvanceBlocks(5)

	activeCount := 0
	h.KnowledgeKeeper.IterateAugmentationBounties(h.Ctx, func(b *knowledgetypes.AugmentationBounty) bool {
		if b.Active {
			activeCount++
		}
		return false
	})
	require.Equal(t, 0, activeCount, "heartbeat processed all N bounties without stalling")
}

// ─── ATTACK 11: Merkle immutability under double-finalize ──────────────
//
// A finalized manifest's Merkle root is the chain's reproducibility
// commitment. Attempting to finalize twice must reject — otherwise a
// subtle change in qualifying fact state between the two calls could
// silently swap the committed root.
func TestRouteB_Adversarial_DoubleFinalizeRejected(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	operator := testAddr("adv_dblfin").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-dblfin", OperatorAddress: operator, TokenizerVersion: 1,
	}))
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "m-dblfin", PipelineId: "pipe-dblfin",
		CorpusSelector: &knowledgetypes.CorpusSelector{},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: operator, ManifestId: "m-dblfin"})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: operator, ManifestId: "m-dblfin"})
	require.Error(t, err, "double-finalize must reject; Merkle root is immutable")
	require.Contains(t, err.Error(), "not DRAFT")
}

// ─── ATTACK 12: Bundle detects parent_merkle_root tampering ─────────────
//
// Simulate a compromised node directly mutating a child's stored
// parent_merkle_root after finalize. The bundle MUST re-derive the
// composed root from component IDs and flag the mismatch. This is how a
// historical-corruption attack gets caught after the fact — tamper
// detection via re-derivation, not trust-the-stored-value.
func TestRouteB_Adversarial_BundleDetectsParentRootTamper(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	operator := testAddr("adv_tamper").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-tamp", OperatorAddress: operator, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-TAMP", Content: "p", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: operator,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 3,
	}))
	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "parent-tamp", PipelineId: "pipe-tamp",
		CorpusSelector: &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: operator, ManifestId: "parent-tamp"})
	require.NoError(t, err)

	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "child-tamp", PipelineId: "pipe-tamp",
		ParentManifestId: "parent-tamp",
		CorpusSelector:   &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: operator, ManifestId: "child-tamp"})
	require.NoError(t, err)

	child, _ := h.KnowledgeKeeper.GetTrainingManifest(h.Ctx, "child-tamp")
	child.ParentMerkleRoot = "deadbeef" + child.ParentMerkleRoot[8:]
	require.NoError(t, h.KnowledgeKeeper.SetTrainingManifest(h.Ctx, child))

	bundle, err := qs.TrainingManifestBundle(h.Ctx, &knowledgetypes.QueryTrainingManifestBundleRequest{Id: "child-tamp"})
	require.NoError(t, err)
	require.False(t, bundle.MerkleRootValid,
		"tampered parent_merkle_root must be detected by re-derivation mismatch")
	require.NotEqual(t, bundle.DerivedMerkleRoot, bundle.Manifest.MerkleRoot)
}

// ─── ATTACK 13: Cross-pipeline parent manifest — DOCUMENTED DECISION ────
//
// Pipeline A's manifest can be used as parent by Pipeline B's manifest.
// This is PERMITTED by design — legitimate lineages cross operators
// (e.g. distillation from a third-party SFT). The audit trail makes any
// abuse observable. Test pins the decision so a future hardener doesn't
// silently restrict it without revisiting the rationale.
func TestRouteB_Adversarial_CrossPipelineParentPermitted(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	opA := testAddr("adv_xp_a").String()
	opB := testAddr("adv_xp_b").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-xp-a", OperatorAddress: opA, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-xp-b", OperatorAddress: opB, TokenizerVersion: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-XP", Content: "shared", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: opA,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 3,
	}))

	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: opA, Id: "m-xp-a", PipelineId: "pipe-xp-a",
		CorpusSelector: &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{Creator: opA, ManifestId: "m-xp-a"})
	require.NoError(t, err)

	_, err = ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: opB, Id: "m-xp-b", PipelineId: "pipe-xp-b",
		ParentManifestId: "m-xp-a",
		CorpusSelector:   &knowledgetypes.CorpusSelector{MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3},
	})
	require.NoError(t, err,
		"cross-pipeline parent is permitted by design — lineage trail makes abuse visible")
}
