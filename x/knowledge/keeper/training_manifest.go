package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Route B Wave 7: training-manifest foundation ────────────────────────
//
// The load-bearing infrastructure of Route B. A TrainingManifest is the
// atomic, externally-verifiable unit a training pipeline operator actually
// downloads and trains on.
//
// Three load-bearing operations live in this file:
//   1. SelectIncludedIds — applies a CorpusSelector to current chain state
//      and returns the canonical, sorted, deterministic set of IDs.
//   2. ComputeManifestMerkleRoot — canonical Merkle commitment over the
//      included-ID sets. Domain-separated per set so swaps between sets
//      cannot collide.
//   3. AssembleBundle — materialises the downloadable bundle by resolving
//      every ID into its full trace/pair/entry, and re-derives the Merkle
//      root for client-side verification.
//
// Invariant: Merkle root is computed from IDs alone, never from payloads.
// This means a client can verify membership against the on-chain root
// without needing to trust any RPC's ability to faithfully serialise
// payloads. Reproducibility is a property of the ID set, not the encoding.

// ─── CRUD ────────────────────────────────────────────────────────────────

// SetTrainingManifest writes the manifest and maintains all reverse indexes.
// If an existing manifest has the same ID with a different status, the old
// status index entry is purged.
func (k Keeper) SetTrainingManifest(ctx context.Context, m *types.TrainingManifest) error {
	if m == nil || m.ManifestId == "" {
		return fmt.Errorf("invalid manifest")
	}
	store := k.storeService.OpenKVStore(ctx)

	// Purge old status-index entry if this is an update with status change.
	if prev, ok := k.GetTrainingManifest(ctx, m.ManifestId); ok && prev != nil && prev.Status != m.Status {
		_ = store.Delete(types.TrainingManifestByStatusKey(byte(prev.Status), m.ManifestId))
	}

	bz, err := marshalOpts.Marshal(m)
	if err != nil {
		return err
	}
	if err := store.Set(types.TrainingManifestKey(m.ManifestId), bz); err != nil {
		return err
	}
	if m.PipelineId != "" {
		if err := store.Set(types.TrainingManifestByPipelineKey(m.PipelineId, m.ManifestId), []byte{1}); err != nil {
			return err
		}
	}
	if m.Creator != "" {
		if err := store.Set(types.TrainingManifestByCreatorKey(m.Creator, m.ManifestId), []byte{1}); err != nil {
			return err
		}
	}
	return store.Set(types.TrainingManifestByStatusKey(byte(m.Status), m.ManifestId), []byte{1})
}

// GetTrainingManifest fetches a manifest by id.
func (k Keeper) GetTrainingManifest(ctx context.Context, id string) (*types.TrainingManifest, bool) {
	if id == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TrainingManifestKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var m types.TrainingManifest
	if err := proto.Unmarshal(bz, &m); err != nil {
		return nil, false
	}
	return &m, true
}

// IterateTrainingManifests yields every manifest.
func (k Keeper) IterateTrainingManifests(ctx context.Context, cb func(*types.TrainingManifest) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.TrainingManifestKeyPrefix, prefixEndBytes(types.TrainingManifestKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var m types.TrainingManifest
		if err := proto.Unmarshal(iter.Value(), &m); err != nil {
			continue
		}
		if cb(&m) {
			return
		}
	}
}

// CountFinalizedManifests returns the number of manifests in FINALIZED or
// ATTESTED status. Used by RouteBCapabilities.
func (k Keeper) CountFinalizedManifests(ctx context.Context) uint64 {
	var n uint64
	k.IterateTrainingManifests(ctx, func(m *types.TrainingManifest) bool {
		if m.Status == types.ManifestStatus_MANIFEST_STATUS_FINALIZED ||
			m.Status == types.ManifestStatus_MANIFEST_STATUS_ATTESTED {
			n++
		}
		return false
	})
	return n
}

// ─── Selection — applying the CorpusSelector ────────────────────────────

// SelectedManifestIDs is the result of applying a selector to chain state.
type SelectedManifestIDs struct {
	FactIDs                   []string
	TraceIDs                  []string
	PairIDs                   []string
	DriftAugmentationIDs      []string
	NormativeCommitmentIDs    []string
}

// Total returns the sum across all sets.
func (s SelectedManifestIDs) Total() uint32 {
	return uint32(len(s.FactIDs)) +
		uint32(len(s.TraceIDs)) +
		uint32(len(s.PairIDs)) +
		uint32(len(s.DriftAugmentationIDs)) +
		uint32(len(s.NormativeCommitmentIDs))
}

// SelectIncludedIds applies the CorpusSelector against current chain state
// and returns the canonical sorted ID sets. Deterministic under a fixed
// snapshot. Pure: no mutation, no event emission.
func (k Keeper) SelectIncludedIds(ctx context.Context, sel *types.CorpusSelector) SelectedManifestIDs {
	if sel == nil {
		sel = &types.CorpusSelector{}
	}
	out := SelectedManifestIDs{}

	// Build a whitelist/blacklist set for domain filtering.
	allow := toSet(sel.DomainWhitelist)
	deny := toSet(sel.DomainBlacklist)

	// 1) Facts + Traces — walk facts, filter, emit one fact_id + one trace_id per kept fact.
	k.IterateFacts(ctx, func(f *types.Fact) bool {
		if f == nil {
			return false
		}
		if !factMatchesSelector(f, sel, allow, deny, k) {
			return false
		}
		out.FactIDs = append(out.FactIDs, f.Id)
		// Trace id is deterministic from (fact_id, snapshot_block).
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		out.TraceIDs = append(out.TraceIDs, makeTraceID(f.Id, uint64(sdkCtx.BlockHeight())))
		return false
	})

	// 2) Contrastive pairs.
	if sel.IncludeContrastivePairs {
		k.IterateContrastivePairs(ctx, func(p *types.ContrastivePair) bool {
			if sel.PairTypeFilter != types.ContrastivePairType_CONTRASTIVE_PAIR_UNSPECIFIED &&
				p.PairType != sel.PairTypeFilter {
				return false
			}
			if sel.MethodId != "" && p.MethodId != sel.MethodId {
				return false
			}
			out.PairIDs = append(out.PairIDs, p.PairId)
			return false
		})
	}

	// 3) Drift entries.
	if sel.IncludeDrift {
		k.IterateDriftAugmentations(ctx, func(a *types.Augmentation) bool {
			out.DriftAugmentationIDs = append(out.DriftAugmentationIDs, a.Id)
			return false
		})
	}

	// 4) Normative commitments.
	if sel.IncludeNormative {
		for _, c := range k.GetAllNormativeCommitments(ctx) {
			if c != nil {
				out.NormativeCommitmentIDs = append(out.NormativeCommitmentIDs, c.Id)
			}
		}
	}

	// Sort all sets for determinism.
	sort.Strings(out.FactIDs)
	sort.Strings(out.TraceIDs)
	sort.Strings(out.PairIDs)
	sort.Strings(out.DriftAugmentationIDs)
	sort.Strings(out.NormativeCommitmentIDs)
	return out
}

func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		if v != "" {
			m[v] = true
		}
	}
	return m
}

func factMatchesSelector(f *types.Fact, sel *types.CorpusSelector, allow, deny map[string]bool, k Keeper) bool {
	if sel.MethodId != "" && f.MethodId != sel.MethodId {
		return false
	}
	if f.CorroborationCount < sel.MinCorroboration {
		return false
	}
	if !sel.IncludeDisproven && f.Status == types.FactStatus_FACT_STATUS_DISPROVEN {
		return false
	}
	if len(allow) > 0 && !allow[f.Domain] {
		return false
	}
	if deny[f.Domain] {
		return false
	}
	if sel.MinSubmitterCalibrationBps > 0 &&
		f.SubmitterCalibrationSnapshotBps < sel.MinSubmitterCalibrationBps {
		return false
	}
	// Quality tier filter.
	if sel.MinQualityTier != types.TrainingQualityTier_TRAINING_QUALITY_TIER_UNSPECIFIED {
		q := k.classifyQualityTier(nil, f)
		if qualityRank(q) < qualityRank(sel.MinQualityTier) {
			return false
		}
	}
	// Curriculum tier filter.
	if sel.MinCurriculumTier != types.CurriculumTier_CURRICULUM_TIER_UNSPECIFIED {
		c := classifyCurriculumTier(f)
		if curriculumRank(c) < curriculumRank(sel.MinCurriculumTier) {
			return false
		}
	}
	return true
}

func curriculumRank(t types.CurriculumTier) int {
	switch t {
	case types.CurriculumTier_CURRICULUM_TIER_FOUNDATION:
		return 4
	case types.CurriculumTier_CURRICULUM_TIER_INTERMEDIATE:
		return 3
	case types.CurriculumTier_CURRICULUM_TIER_ADVANCED:
		return 2
	case types.CurriculumTier_CURRICULUM_TIER_SPECIALISED:
		return 1
	default:
		return 0
	}
}

// ─── Merkle commitment ──────────────────────────────────────────────────
//
// The commitment is a single SHA-256 over a domain-separated, canonically-
// sorted concatenation of every included ID. The shape:
//
//   H( "ZERONE/KNOWLEDGE/MANIFEST/v1" |
//      "FACTS:"       || len(facts)       || facts[0]      || ... ||
//      "TRACES:"      || len(traces)      || traces[0]     || ... ||
//      "PAIRS:"       || len(pairs)       || pairs[0]      || ... ||
//      "DRIFT:"       || len(drifts)      || drifts[0]     || ... ||
//      "NORMATIVE:"   || len(commitments) || commitments[0]|| ... )
//
// Domain separators ("FACTS:", "TRACES:", etc.) prevent swaps between sets
// from colliding. Length prefixes prevent length-extension. Sorted inputs
// make the commitment independent of iteration order.
//
// A client can re-derive this without trusting the chain RPC: they just
// need the ID lists (authenticated once via this root).

const manifestMerkleDomainTag = "ZERONE/KNOWLEDGE/MANIFEST/v1"

// ComputeManifestMerkleRoot returns the hex-encoded SHA-256 commitment.
// Deterministic; sorted-set assumed (SelectIncludedIds sorts).
func ComputeManifestMerkleRoot(ids SelectedManifestIDs) string {
	h := sha256.New()
	// Domain tag.
	writeLenString(h, manifestMerkleDomainTag)

	writeLabelledStringSet(h, "FACTS", ids.FactIDs)
	writeLabelledStringSet(h, "TRACES", ids.TraceIDs)
	writeLabelledStringSet(h, "PAIRS", ids.PairIDs)
	writeLabelledStringSet(h, "DRIFT", ids.DriftAugmentationIDs)
	writeLabelledStringSet(h, "NORMATIVE", ids.NormativeCommitmentIDs)

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

func writeLabelledStringSet(h interface{ Write([]byte) (int, error) }, label string, ss []string) {
	writeLenString(h, label+":")
	var lenBuf [8]byte
	// Length prefix (big-endian uint64).
	putUint64(lenBuf[:], uint64(len(ss)))
	_, _ = h.Write(lenBuf[:])
	for _, s := range ss {
		writeLenString(h, s)
	}
}

func writeLenString(h interface{ Write([]byte) (int, error) }, s string) {
	var lenBuf [8]byte
	putUint64(lenBuf[:], uint64(len(s)))
	_, _ = h.Write(lenBuf[:])
	_, _ = h.Write([]byte(s))
}

func putUint64(buf []byte, x uint64) {
	buf[0] = byte(x >> 56)
	buf[1] = byte(x >> 48)
	buf[2] = byte(x >> 40)
	buf[3] = byte(x >> 32)
	buf[4] = byte(x >> 24)
	buf[5] = byte(x >> 16)
	buf[6] = byte(x >> 8)
	buf[7] = byte(x)
}

// ─── Bundle assembly ─────────────────────────────────────────────────────

// AssembleManifestBundle materialises the full downloadable payload for a
// FINALIZED or ATTESTED manifest, and re-derives the Merkle root for
// client-side verification.
func (k Keeper) AssembleManifestBundle(ctx context.Context, manifestID string) (*types.QueryTrainingManifestBundleResponse, error) {
	m, ok := k.GetTrainingManifest(ctx, manifestID)
	if !ok || m == nil {
		return nil, fmt.Errorf("manifest %s not found", manifestID)
	}

	// Resolve traces.
	var traces []*types.MethodologyApplicationTrace
	for _, fid := range m.IncludedFactIds {
		if t, found := k.BuildMethodologyApplicationTrace(ctx, fid); found {
			traces = append(traces, t)
		}
	}

	// Resolve contrastive pairs — walk all, filter by ID.
	wantedPairs := toSet(m.IncludedPairIds)
	var pairs []*types.ContrastivePair
	if len(wantedPairs) > 0 {
		k.IterateContrastivePairs(ctx, func(p *types.ContrastivePair) bool {
			if wantedPairs[p.PairId] {
				pairs = append(pairs, p)
			}
			return false
		})
	}

	// Resolve drift entries.
	wantedDrifts := toSet(m.IncludedDriftAugmentationIds)
	var drifts []*types.DriftCorpusEntry
	if len(wantedDrifts) > 0 {
		k.IterateDriftAugmentations(ctx, func(a *types.Augmentation) bool {
			if !wantedDrifts[a.Id] {
				return false
			}
			drifts = append(drifts, &types.DriftCorpusEntry{
				AugmentationId: a.Id,
				OriginalFactId: a.OriginalFactId,
				VariantContent: a.VariantContent,
				Verdict:        a.Verdict,
				VerdictBlock:   a.VerdictBlock,
			})
			return false
		})
	}

	// Resolve normative commitments.
	wantedCommitments := toSet(m.IncludedNormativeCommitmentIds)
	var normativeEntries []*types.NormativeCorpusEntry
	if len(wantedCommitments) > 0 {
		for _, c := range k.GetAllNormativeCommitments(ctx) {
			if c == nil || !wantedCommitments[c.Id] {
				continue
			}
			normativeEntries = append(normativeEntries, &types.NormativeCorpusEntry{
				CommitmentId: c.Id,
				Content:      c.Statement,
				Domain:       c.Category,
				IsNormative:  true,
			})
		}
	}

	// Re-derive the Merkle root from the manifest's recorded ID sets. This
	// is a pure function of IDs — no RPC trust required for callers.
	ids := SelectedManifestIDs{
		FactIDs:                m.IncludedFactIds,
		TraceIDs:               m.IncludedTraceIds,
		PairIDs:                m.IncludedPairIds,
		DriftAugmentationIDs:   m.IncludedDriftAugmentationIds,
		NormativeCommitmentIDs: m.IncludedNormativeCommitmentIds,
	}
	derived := ComputeManifestMerkleRoot(ids)

	return &types.QueryTrainingManifestBundleResponse{
		Manifest:         m,
		Traces:           traces,
		ContrastivePairs: pairs,
		DriftEntries:     drifts,
		NormativeEntries: normativeEntries,
		DerivedMerkleRoot: derived,
		MerkleRootValid:   derived == m.MerkleRoot,
	}, nil
}
