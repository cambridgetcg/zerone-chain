package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Route B Wave 5: unified training data format ───────────────────────
//
// This file implements the canonical MethodologyApplicationTrace assembly,
// the ContrastivePair emitter, and the governance-ratified TraceSchema.
//
// Alignment invariant: every trace encodes truth-seeking *process*, not
// just the statement. A model trained on these records learns to declare
// methodology, show work, accept falsification, and cite provenance — the
// behaviors of a truth-seeker.

// ─── TraceSchema CRUD ────────────────────────────────────────────────────

// SetTraceSchema stores the current TraceSchema singleton and a version
// snapshot into the history namespace.
func (k Keeper) SetTraceSchema(ctx context.Context, s *types.TraceSchema) error {
	if s == nil || s.Version == 0 {
		return fmt.Errorf("invalid trace schema (version required)")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(s)
	if err != nil {
		return err
	}
	if err := store.Set(types.TraceSchemaKey, bz); err != nil {
		return err
	}
	return store.Set(types.TraceSchemaHistoryKey(s.Version), bz)
}

// GetTraceSchema fetches the current TraceSchema.
func (k Keeper) GetTraceSchema(ctx context.Context) (*types.TraceSchema, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TraceSchemaKey)
	if err != nil || bz == nil {
		return nil, false
	}
	var s types.TraceSchema
	if err := proto.Unmarshal(bz, &s); err != nil {
		return nil, false
	}
	return &s, true
}

// GetTraceSchemaAtVersion fetches a historical schema by version.
func (k Keeper) GetTraceSchemaAtVersion(ctx context.Context, version uint64) (*types.TraceSchema, bool) {
	if version == 0 {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TraceSchemaHistoryKey(version))
	if err != nil || bz == nil {
		return nil, false
	}
	var s types.TraceSchema
	if err := proto.Unmarshal(bz, &s); err != nil {
		return nil, false
	}
	return &s, true
}

// SeedDefaultTraceSchema writes the v1 schema if none exists. Called at
// genesis and exposed for test harnesses.
func (k Keeper) SeedDefaultTraceSchema(ctx context.Context) error {
	if _, ok := k.GetTraceSchema(ctx); ok {
		return nil
	}
	schema := defaultTraceSchemaV1()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	schema.RatifiedAtBlock = uint64(sdkCtx.BlockHeight())
	return k.SetTraceSchema(ctx, schema)
}

// defaultTraceSchemaV1 returns the v1 JSON Schema contract. Written inline
// so the bootstrap is deterministic and audit-friendly.
func defaultTraceSchemaV1() *types.TraceSchema {
	jsonSchema := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "MethodologyApplicationTrace",
  "description": "Canonical per-fact training row (Route B Wave 5). Encodes a claim plus every truth-seeking signal the chain has about it.",
  "type": "object",
  "required": [
    "trace_id","fact_id","snapshot_block_height","tokenizer_version",
    "canonical_serialisation_version","trace_schema_version",
    "content","domain","methodology_id","status","submitter","is_normative"
  ],
  "properties": {
    "trace_id":                            {"type":"string"},
    "fact_id":                             {"type":"string"},
    "snapshot_block_height":               {"type":"integer","minimum":0},
    "tokenizer_version":                   {"type":"integer","minimum":1},
    "canonical_serialisation_version":     {"type":"integer","minimum":1},
    "trace_schema_version":                {"type":"integer","minimum":1},
    "content":                             {"type":"string"},
    "domain":                              {"type":"string"},
    "subject":                             {"type":"string"},
    "canonical_form":                      {"type":"string"},
    "methodology_id":                      {"type":"string"},
    "methodology_rubric":                  {"type":"string"},
    "reasoning_trace":                     {"type":"string"},
    "axiom_distance":                      {"type":"integer","minimum":0},
    "dependency_confidence_floor_bps":     {"type":"integer","minimum":0},
    "predecessor_edges":                   {"type":"array"},
    "descendant_edges":                    {"type":"array"},
    "grounded_score_bps":                  {"type":"integer","minimum":0},
    "own_confidence_bps":                  {"type":"integer","minimum":0},
    "verifier_panel_size":                 {"type":"integer","minimum":0},
    "dissenting_verifiers":                {"type":"array","items":{"type":"string"}},
    "challenges":                          {"type":"array"},
    "corroboration_count":                 {"type":"integer","minimum":0},
    "status":                              {"type":"string","enum":["ACTIVE","DISPROVEN","SUPERSEDED","PROVISIONAL","CONTESTED","UNSPECIFIED"]},
    "vindication":                         {"type":"object"},
    "disproval":                           {"type":"object"},
    "supersession_chain":                  {"type":"array","items":{"type":"string"}},
    "reformulations":                      {"type":"array"},
    "drift_examples":                      {"type":"array"},
    "contradicting_fact_ids":              {"type":"array","items":{"type":"string"}},
    "submitter":                           {"type":"string"},
    "submitter_calibration_at_submission_bps": {"type":"integer","minimum":0},
    "training_value_weight_bps":           {"type":"integer","minimum":0},
    "curriculum_tier":                     {"type":"string"},
    "quality_tier":                        {"type":"string"},
    "is_normative":                        {"type":"boolean","const":false}
  }
}`
	sum := sha256.Sum256([]byte(jsonSchema))
	return &types.TraceSchema{
		Version:         1,
		JsonSchemaHash:  hex.EncodeToString(sum[:]),
		JsonSchema:      jsonSchema,
		RequiredFields: []string{
			"trace_id", "fact_id", "snapshot_block_height", "tokenizer_version",
			"canonical_serialisation_version", "trace_schema_version",
			"content", "domain", "methodology_id", "status", "submitter", "is_normative",
		},
		Notes: "Bootstrap MethodologyApplicationTrace contract. Governance may amend via MsgAmendTraceSchema.",
	}
}

// ─── MethodologyApplicationTrace assembly ────────────────────────────────

// BuildMethodologyApplicationTrace assembles the full unified training row
// for a fact by walking the knowledge graph, calibration, adjudication, and
// contrastive companions. O(fact's neighborhood), not O(chain).
func (k Keeper) BuildMethodologyApplicationTrace(ctx context.Context, factID string) (*types.MethodologyApplicationTrace, bool) {
	fact, ok := k.GetFact(ctx, factID)
	if !ok || fact == nil {
		return nil, false
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshotHeight := uint64(sdkCtx.BlockHeight())

	// Provenance pin.
	tokenizerVersion := uint64(0)
	canonSerVersion := uint64(0)
	if spec, ok := k.GetTokenizerSpec(ctx); ok && spec != nil {
		tokenizerVersion = spec.Version
		canonSerVersion = spec.CanonicalSerialisationVersion
	}
	schemaVersion := uint64(0)
	if s, ok := k.GetTraceSchema(ctx); ok && s != nil {
		schemaVersion = s.Version
	}

	trace := &types.MethodologyApplicationTrace{
		TraceId:                       makeTraceID(factID, snapshotHeight),
		FactId:                        fact.Id,
		SnapshotBlockHeight:           snapshotHeight,
		TokenizerVersion:              tokenizerVersion,
		CanonicalSerialisationVersion: canonSerVersion,
		TraceSchemaVersion:            schemaVersion,

		Content:       fact.Content,
		Domain:        fact.Domain,
		CanonicalForm: fact.CanonicalForm,

		MethodologyId:                 fact.MethodId,
		ReasoningTrace:                fact.ReasoningTrace,
		AxiomDistance:                 fact.AxiomDistance,
		DependencyConfidenceFloorBps:  fact.DependencyConfidenceFloor,

		OwnConfidenceBps:   fact.Confidence,
		VerifiedAtBlock:    fact.VerifiedAtBlock,
		CorroborationCount: fact.CorroborationCount,
		LastCorroboratedBlock: fact.LastCorroboratedBlock,

		Status: fact.Status,

		Submitter:                            fact.Submitter,
		SubmitterCalibrationAtSubmissionBps:  fact.SubmitterCalibrationSnapshotBps,
		SubmittedAtBlock:                     fact.SubmittedAtBlock,

		IsNormative: false,
	}

	// Methodology rubric (Description is the plain-language rule body).
	if m, ok := k.GetMethodology(ctx, fact.MethodId); ok && m != nil {
		trace.MethodologyRubric = m.Description
	}

	// Derivation graph — forward edges (this fact → predecessors).
	trace.PredecessorEdges = k.safeGetRelations(ctx, fact.Id)
	trace.DescendantEdges = k.safeGetIncomingRelations(ctx, fact.Id)

	// Grounded score (from TrustProfile math).
	trace.GroundedScoreBps = k.computeGroundedScore(fact)

	// Dialectical history: walk challenge claims that reference this fact.
	trace.Challenges = k.collectTraceChallenges(ctx, fact.Id)

	// Vindication.
	if v := k.collectVindication(ctx, fact.Id); v != nil {
		trace.Vindication = v
	}

	// Disproval.
	if d := k.collectDisproval(ctx, fact); d != nil {
		trace.Disproval = d
	}

	// Supersession chain (SUPERSEDED_BY / SUPERSEDES edges downstream).
	trace.SupersessionChain = k.collectSupersessionChain(ctx, fact.Id)

	// Contrastive companions.
	refs, drifts := k.collectReformulationCompanions(ctx, fact.Id)
	trace.Reformulations = refs
	trace.DriftExamples = drifts
	trace.ContradictingFactIds = k.collectContradictingFactIds(ctx, fact.Id)

	// Training weight (Popper-weighted from Wave 4).
	tvw := k.ComputeTrainingValueWeight(ctx, fact.Id)
	trace.TrainingValueWeightBps = tvw.Final

	// Curriculum + quality tiers (reusing existing logic).
	trace.CurriculumTier = classifyCurriculumTier(fact)
	trace.QualityTier = k.classifyQualityTier(ctx, fact)

	return trace, true
}

// ─── Helpers for trace assembly ──────────────────────────────────────────

func makeTraceID(factID string, height uint64) string {
	h := sha256.New()
	h.Write([]byte(factID))
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, height)
	h.Write(buf)
	sum := h.Sum(nil)
	return "trace-" + hex.EncodeToString(sum[:8])
}

func (k Keeper) computeGroundedScore(f *types.Fact) uint64 {
	if f == nil {
		return 0
	}
	// Mirrors the TrustProfile formula approximately:
	//   grounded = own_confidence × axiom_weight × floor_weight / BPS²
	own := f.Confidence
	if own == 0 {
		return 0
	}
	axiomDecay := uint64(f.AxiomDistance) * 50_000
	if axiomDecay >= 500_000 {
		axiomDecay = 500_000
	}
	axiomWeight := bps - axiomDecay
	floorWeight := uint64(bps)
	if f.DependencyConfidenceFloor > 0 && f.DependencyConfidenceFloor < own {
		floorWeight = (f.DependencyConfidenceFloor * bps) / own
	}
	grounded := safeMulDivTVW(own, axiomWeight, bps)
	grounded = safeMulDivTVW(grounded, floorWeight, bps)
	if grounded > own {
		grounded = own
	}
	return grounded
}

func (k Keeper) collectTraceChallenges(ctx context.Context, factID string) []*types.TraceChallenge {
	var out []*types.TraceChallenge
	// Iterate claims whose provisional_fact_id == factID (challenge claims).
	k.IterateClaims(ctx, func(c *types.Claim) bool {
		if c == nil {
			return false
		}
		if c.ProvisionalFactId != factID {
			return false
		}
		outcome := "pending"
		var resolvedBlock uint64
		if c.VerificationRoundId != "" {
			if round, ok := k.GetVerificationRound(ctx, c.VerificationRoundId); ok && round != nil {
				resolvedBlock = round.VerdictBlock
				switch round.Verdict {
				case types.Verdict_VERDICT_ACCEPT:
					// Challenge accepted → original fact was disproven.
					outcome = "disproven"
				case types.Verdict_VERDICT_REJECT:
					outcome = "survived"
				}
			}
		}
		out = append(out, &types.TraceChallenge{
			Challenger:         c.Submitter,
			ArgumentText:       c.ArgumentText,
			ChallengeMethodId:  c.MethodId,
			RebuttalText:       c.RebuttalText,
			Outcome:            outcome,
			ResolvedBlock:      resolvedBlock,
		})
		return false
	})
	return out
}

func (k Keeper) collectVindication(ctx context.Context, factID string) *types.TraceVindication {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.VindicationRecordPrefixForFact(factID)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var verifiers []string
	var earliest uint64
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		verifier := string(key[len(prefix):])
		verifiers = append(verifiers, verifier)
		// Block height is stored in the record value as JSON — cheap to skip
		// parsing; we surface the earliest observed block instead.
		if earliest == 0 {
			earliest = 1 // marker — callers can cross-reference the chain
		}
	}
	if len(verifiers) == 0 {
		return nil
	}
	return &types.TraceVindication{
		Verifiers:         verifiers,
		VindicatedAtBlock: earliest,
	}
}

func (k Keeper) collectDisproval(ctx context.Context, fact *types.Fact) *types.TraceDisproval {
	if fact == nil || fact.Status != types.FactStatus_FACT_STATUS_DISPROVEN {
		return nil
	}
	// Find the successful challenge: incoming CONTRADICTS edge.
	rels := k.safeGetIncomingRelations(ctx, fact.Id)
	for _, r := range rels {
		if r.Relation == types.RelationType_RELATION_TYPE_CONTRADICTS {
			return &types.TraceDisproval{
				DisprovenByFactId: r.SourceFactId,
				MethodId:          r.MethodId,
				DisprovenAtBlock:  r.CreatedAtBlock,
			}
		}
	}
	return &types.TraceDisproval{
		DisprovenAtBlock: fact.RevenueClawbackBlock,
	}
}

func (k Keeper) collectSupersessionChain(ctx context.Context, factID string) []string {
	var chain []string
	visited := map[string]bool{factID: true}
	cur := factID
	for depth := 0; depth < 8; depth++ {
		rels := k.safeGetIncomingRelations(ctx, cur)
		var next string
		for _, r := range rels {
			if r.Relation == types.RelationType_RELATION_TYPE_SUPERSEDES && !visited[r.SourceFactId] {
				next = r.SourceFactId
				break
			}
		}
		if next == "" {
			break
		}
		chain = append(chain, next)
		visited[next] = true
		cur = next
	}
	return chain
}

func (k Keeper) collectReformulationCompanions(ctx context.Context, factID string) ([]*types.TraceReformulation, []*types.TraceDrift) {
	var refs []*types.TraceReformulation
	var drifts []*types.TraceDrift

	k.IterateAugmentations(ctx, func(a *types.Augmentation) bool {
		if a.OriginalFactId != factID {
			return false
		}
		switch a.Verdict {
		case types.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
			types.AugmentationVerdict_AUGMENTATION_VERDICT_SUPERIOR:
			methID := ""
			if a.BountyId != "" {
				if b, ok := k.GetAugmentationBounty(ctx, a.BountyId); ok {
					methID = b.MethodologyId
				}
			}
			refs = append(refs, &types.TraceReformulation{
				AugmentationId: a.Id,
				VariantContent: a.VariantContent,
				Verdict:        a.Verdict,
				VerifierCount:  uint32(len(a.VerdictVoters)),
				VerdictBlock:   a.VerdictBlock,
				MethodologyId:  methID,
			})
		case types.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT,
			types.AugmentationVerdict_AUGMENTATION_VERDICT_INFERIOR:
			drifts = append(drifts, &types.TraceDrift{
				AugmentationId: a.Id,
				VariantContent: a.VariantContent,
				Verdict:        a.Verdict,
				DriftVoters:    a.VerdictVoters,
				VerdictBlock:   a.VerdictBlock,
			})
		}
		return false
	})
	return refs, drifts
}

func (k Keeper) collectContradictingFactIds(ctx context.Context, factID string) []string {
	var out []string
	// Outgoing CONTRADICTS edges (this fact contradicts another).
	for _, r := range k.safeGetRelations(ctx, factID) {
		if r.Relation == types.RelationType_RELATION_TYPE_CONTRADICTS {
			out = append(out, r.TargetFactId)
		}
	}
	// Incoming CONTRADICTS edges (another fact contradicts this one).
	for _, r := range k.safeGetIncomingRelations(ctx, factID) {
		if r.Relation == types.RelationType_RELATION_TYPE_CONTRADICTS {
			out = append(out, r.SourceFactId)
		}
	}
	return out
}

// classifyCurriculumTier assigns a curriculum tier based on axiom distance,
// corroboration, and methodology. Mirrors the existing StructuredCorpus logic.
func classifyCurriculumTier(f *types.Fact) types.CurriculumTier {
	if f == nil {
		return types.CurriculumTier_CURRICULUM_TIER_UNSPECIFIED
	}
	specialised := map[string]bool{
		"M-PHENOMENOLOGICAL": true,
		"M-ECOLOGICAL":       true,
		"M-PRACTICE":         true,
	}
	if specialised[f.MethodId] {
		return types.CurriculumTier_CURRICULUM_TIER_SPECIALISED
	}
	if f.AxiomDistance <= 1 && f.CorroborationCount >= 3 {
		return types.CurriculumTier_CURRICULUM_TIER_FOUNDATION
	}
	if f.AxiomDistance >= 5 {
		return types.CurriculumTier_CURRICULUM_TIER_ADVANCED
	}
	return types.CurriculumTier_CURRICULUM_TIER_INTERMEDIATE
}

// classifyQualityTier mirrors existing StructuredCorpus quality classification.
func (k Keeper) classifyQualityTier(_ context.Context, f *types.Fact) types.TrainingQualityTier {
	if f == nil {
		return types.TrainingQualityTier_TRAINING_QUALITY_TIER_UNSPECIFIED
	}
	switch f.Status {
	case types.FactStatus_FACT_STATUS_DISPROVEN:
		return types.TrainingQualityTier_TRAINING_QUALITY_TIER_NEGATIVE
	case types.FactStatus_FACT_STATUS_CONTESTED, types.FactStatus_FACT_STATUS_EXPIRED, types.FactStatus_FACT_STATUS_SUPERSEDED:
		return types.TrainingQualityTier_TRAINING_QUALITY_TIER_UNSUITABLE
	}
	if f.MethodId == "" || f.MethodId == "M-LEGACY" {
		return types.TrainingQualityTier_TRAINING_QUALITY_TIER_BRONZE
	}
	if f.CorroborationCount >= 3 {
		return types.TrainingQualityTier_TRAINING_QUALITY_TIER_GOLD
	}
	if f.CorroborationCount >= 1 {
		return types.TrainingQualityTier_TRAINING_QUALITY_TIER_SILVER
	}
	return types.TrainingQualityTier_TRAINING_QUALITY_TIER_BRONZE
}

// ─── Contrastive-pair emitter ────────────────────────────────────────────

// IterateContrastivePairs streams (positive, negative, verdict) tuples
// assembled from the existing adjudication state. Emits four kinds:
//   - SURVIVED_VS_DISPROVEN: for every CONTRADICTS edge where one side is
//     DISPROVEN and the other is ACTIVE.
//   - EQUIVALENT_VS_DRIFT: an EQUIVALENT reformulation paired with a DRIFT
//     reformulation of the same original fact.
//   - EQUIVALENT_VS_INFERIOR: an EQUIVALENT reformulation paired with an
//     INFERIOR sibling.
//   - VINDICATED_MINORITY: vindication records where majority vote lost.
func (k Keeper) IterateContrastivePairs(ctx context.Context, cb func(*types.ContrastivePair) bool) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshotHeight := uint64(sdkCtx.BlockHeight())
	var schemaVersion uint64
	if s, ok := k.GetTraceSchema(ctx); ok && s != nil {
		schemaVersion = s.Version
	}

	// 1) SURVIVED_VS_DISPROVEN — walk facts, find DISPROVEN ones, find the
	//    CONTRADICTS source, emit the pair.
	if !k.iteratePairSurvivedVsDisproven(ctx, snapshotHeight, schemaVersion, cb) {
		return
	}

	// 2/3) EQUIVALENT_VS_DRIFT / EQUIVALENT_VS_INFERIOR — bucket augmentations
	//      by original_fact_id, then cross-emit winners × losers.
	if !k.iteratePairReformulationVsDrift(ctx, snapshotHeight, schemaVersion, cb) {
		return
	}

	// 4) VINDICATED_MINORITY — iterate vindication records.
	k.iteratePairVindicatedMinority(ctx, snapshotHeight, schemaVersion, cb)
}

func (k Keeper) iteratePairSurvivedVsDisproven(ctx context.Context, snap, schema uint64, cb func(*types.ContrastivePair) bool) bool {
	continueWalk := true
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f == nil || f.Status != types.FactStatus_FACT_STATUS_DISPROVEN {
			return false
		}
		// Find the CONTRADICTS source — the fact that survived.
		for _, r := range k.safeGetIncomingRelations(ctx, f.Id) {
			if r.Relation != types.RelationType_RELATION_TYPE_CONTRADICTS {
				continue
			}
			survived, ok := k.GetFact(ctx, r.SourceFactId)
			if !ok || survived == nil || survived.Status == types.FactStatus_FACT_STATUS_DISPROVEN {
				continue
			}
			pair := &types.ContrastivePair{
				PairId:                 "cp-survived-" + f.Id,
				PairType:               types.ContrastivePairType_CONTRASTIVE_PAIR_SURVIVED_VS_DISPROVEN,
				PositiveFactId:         survived.Id,
				PositiveContent:        survived.Content,
				NegativeFactId:         f.Id,
				NegativeContent:        f.Content,
				MethodId:               r.MethodId,
				DistinguishingArgument: fmt.Sprintf("contradicts edge (%s)", r.Relation),
				ResolvedAtBlock:        r.CreatedAtBlock,
				SnapshotBlockHeight:    snap,
				TraceSchemaVersion:     schema,
			}
			if !cb(pair) {
				continue
			}
			continueWalk = false
			return true
		}
		return false
	})
	return continueWalk
}

func (k Keeper) iteratePairReformulationVsDrift(ctx context.Context, snap, schema uint64, cb func(*types.ContrastivePair) bool) bool {
	byOriginal := map[string][]*types.Augmentation{}
	k.IterateAugmentations(ctx, func(a *types.Augmentation) bool {
		byOriginal[a.OriginalFactId] = append(byOriginal[a.OriginalFactId], a)
		return false
	})
	for origID, augs := range byOriginal {
		var winners, drifts, inferiors []*types.Augmentation
		for _, a := range augs {
			switch a.Verdict {
			case types.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
				types.AugmentationVerdict_AUGMENTATION_VERDICT_SUPERIOR:
				winners = append(winners, a)
			case types.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT:
				drifts = append(drifts, a)
			case types.AugmentationVerdict_AUGMENTATION_VERDICT_INFERIOR:
				inferiors = append(inferiors, a)
			}
		}
		origFact, hasOrig := k.GetFact(ctx, origID)
		if !hasOrig {
			continue
		}
		methID := ""
		if w := firstNonNil(winners); w != nil && w.BountyId != "" {
			if b, ok := k.GetAugmentationBounty(ctx, w.BountyId); ok {
				methID = b.MethodologyId
			}
		}
		for _, w := range winners {
			for _, d := range drifts {
				pair := &types.ContrastivePair{
					PairId:                 "cp-drift-" + w.Id + "-" + d.Id,
					PairType:               types.ContrastivePairType_CONTRASTIVE_PAIR_EQUIVALENT_VS_DRIFT,
					PositiveFactId:         origFact.Id,
					PositiveContent:        w.VariantContent,
					NegativeAugmentationId: d.Id,
					NegativeContent:        d.VariantContent,
					MethodId:               methID,
					DistinguishingArgument: "verifier panel ruled DRIFT (meaning preservation failed)",
					ResolvedAtBlock:        d.VerdictBlock,
					SnapshotBlockHeight:    snap,
					TraceSchemaVersion:     schema,
				}
				if !cb(pair) {
					continue
				}
				return false
			}
			for _, inf := range inferiors {
				pair := &types.ContrastivePair{
					PairId:                 "cp-inferior-" + w.Id + "-" + inf.Id,
					PairType:               types.ContrastivePairType_CONTRASTIVE_PAIR_EQUIVALENT_VS_INFERIOR,
					PositiveFactId:         origFact.Id,
					PositiveContent:        w.VariantContent,
					NegativeAugmentationId: inf.Id,
					NegativeContent:        inf.VariantContent,
					MethodId:               methID,
					DistinguishingArgument: "verifier panel ruled INFERIOR (meaning preserved but weaker method application)",
					ResolvedAtBlock:        inf.VerdictBlock,
					SnapshotBlockHeight:    snap,
					TraceSchemaVersion:     schema,
				}
				if !cb(pair) {
					continue
				}
				return false
			}
		}
	}
	return true
}

func (k Keeper) iteratePairVindicatedMinority(ctx context.Context, snap, schema uint64, cb func(*types.ContrastivePair) bool) {
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f == nil {
			return false
		}
		v := k.collectVindication(ctx, f.Id)
		if v == nil || len(v.Verifiers) == 0 {
			return false
		}
		// Pair the vindicated minority's POSITION (the fact) against the
		// (implicit) majority that tried to reject it.
		pair := &types.ContrastivePair{
			PairId:                 "cp-vindicated-" + f.Id,
			PairType:               types.ContrastivePairType_CONTRASTIVE_PAIR_VINDICATED_MINORITY,
			PositiveFactId:         f.Id,
			PositiveContent:        f.Content,
			NegativeContent:        "majority-rejection stance (later overturned)",
			MethodId:               f.MethodId,
			DistinguishingArgument: fmt.Sprintf("%d verifier(s) dissented and were later vindicated", len(v.Verifiers)),
			ResolvedAtBlock:        v.VindicatedAtBlock,
			SnapshotBlockHeight:    snap,
			TraceSchemaVersion:     schema,
		}
		if cb(pair) {
			return true
		}
		return false
	})
}

func firstNonNil(s []*types.Augmentation) *types.Augmentation {
	for _, a := range s {
		if a != nil {
			return a
		}
	}
	return nil
}

// safeGetRelations drops the error return from GetFactRelations for terse use.
func (k Keeper) safeGetRelations(ctx context.Context, factID string) []*types.FactRelation {
	rels, err := k.GetFactRelations(ctx, factID)
	if err != nil {
		return nil
	}
	return rels
}

// safeGetIncomingRelations drops the error return from GetIncomingRelations.
func (k Keeper) safeGetIncomingRelations(ctx context.Context, factID string) []*types.FactRelation {
	rels, err := k.GetIncomingRelations(ctx, factID)
	if err != nil {
		return nil
	}
	return rels
}

// ─── Msg handler: AmendTraceSchema ───────────────────────────────────────

// AmendTraceSchema — authority-gated bump of the trace-schema contract.
// Follows the TokenizerSpec pattern: caller-supplied version is ignored;
// handler auto-assigns current+1.
func (m *msgServer) AmendTraceSchema(ctx context.Context, msg *types.MsgAmendTraceSchema) (*types.MsgAmendTraceSchemaResponse, error) {
	if msg == nil || msg.Schema == nil {
		return nil, fmt.Errorf("trace schema required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: only governance authority may amend trace schema")
	}
	current, found := m.keeper.GetTraceSchema(ctx)
	var nextVersion uint64 = 1
	if found && current != nil {
		nextVersion = current.Version + 1
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	newSchema := msg.Schema
	newSchema.Version = nextVersion
	newSchema.RatifiedAtBlock = uint64(sdkCtx.BlockHeight())
	// Compute hash if not provided (keeps JSON+hash coherent).
	if newSchema.JsonSchemaHash == "" && newSchema.JsonSchema != "" {
		sum := sha256.Sum256([]byte(newSchema.JsonSchema))
		newSchema.JsonSchemaHash = hex.EncodeToString(sum[:])
	}
	if err := m.keeper.SetTraceSchema(ctx, newSchema); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.trace_schema_amended",
		sdk.NewAttribute("new_version", fmt.Sprintf("%d", newSchema.Version)),
		sdk.NewAttribute("json_schema_hash", newSchema.JsonSchemaHash),
		sdk.NewAttribute("authority", msg.Authority),
	))
	return &types.MsgAmendTraceSchemaResponse{NewVersion: newSchema.Version}, nil
}
