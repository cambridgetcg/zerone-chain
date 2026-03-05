package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/bits"
	"regexp"
	"sort"
	"strings"
	"unicode"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Normalized hash (Layer 2) ──────────────────────────────────────────────

var nonAlphaNumRe = regexp.MustCompile(`[^a-z0-9\s]+`)

// normalizeContent produces a canonical form of content for near-duplicate detection.
// Steps: lowercase → strip punctuation → collapse whitespace → sort words.
func normalizeContent(content string) string {
	// Lowercase
	s := strings.ToLower(content)
	// Strip punctuation
	s = nonAlphaNumRe.ReplaceAllString(s, "")
	// Split into words, sort, and rejoin
	words := strings.Fields(s)
	sort.Strings(words)
	return strings.Join(words, " ")
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// SetNormalizedHash stores a normalized content hash mapping to a submission ID.
func (k Keeper) SetNormalizedHash(ctx context.Context, hash, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.NormalizedHashKey(hash), []byte(submissionID))
}

// GetNormalizedHash retrieves the submission ID associated with a normalized hash.
func (k Keeper) GetNormalizedHash(ctx context.Context, hash string) (string, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.NormalizedHashKey(hash))
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

// checkNormalizedDuplicate checks if content matches an existing submission after normalization.
func (k Keeper) checkNormalizedDuplicate(ctx context.Context, content string) (string, bool) {
	normalizedHash := sha256Hex(normalizeContent(content))
	return k.GetNormalizedHash(ctx, normalizedHash)
}

// ─── SimHash (Layer 3) ──────────────────────────────────────────────────────

// computeSimHash computes a SimHash fingerprint from 3-gram features of the content.
func computeSimHash(content string) uint64 {
	// Normalize to lowercase for consistent fingerprinting
	text := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, content)

	words := strings.Fields(text)
	if len(words) < 3 {
		// For very short content, use word-level hash directly
		h := sha256.Sum256([]byte(text))
		return binary.BigEndian.Uint64(h[:8])
	}

	// Build 3-grams
	var vectors [64]int
	for i := 0; i <= len(words)-3; i++ {
		gram := words[i] + " " + words[i+1] + " " + words[i+2]
		h := sha256.Sum256([]byte(gram))
		hash := binary.BigEndian.Uint64(h[:8])
		for j := 0; j < 64; j++ {
			if hash&(1<<uint(j)) != 0 {
				vectors[j]++
			} else {
				vectors[j]--
			}
		}
	}

	var result uint64
	for j := 0; j < 64; j++ {
		if vectors[j] > 0 {
			result |= 1 << uint(j)
		}
	}
	return result
}

// hammingDistance returns the number of differing bits between two SimHash values.
func hammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// SimHashThreshold is the maximum hamming distance for two documents to be considered near-duplicates.
const SimHashThreshold = 6

// SetSimHash stores a SimHash mapping to a submission ID.
func (k Keeper) SetSimHash(ctx context.Context, hash uint64, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.SimHashKey(hash), []byte(submissionID))
}

// checkSimHashDuplicate checks if content is a near-duplicate using SimHash hamming distance.
func (k Keeper) checkSimHashDuplicate(ctx context.Context, content string) (string, bool) {
	hash := computeSimHash(content)
	store := k.storeService.OpenKVStore(ctx)

	// Check exact SimHash first
	bz, err := store.Get(types.SimHashKey(hash))
	if err == nil && bz != nil {
		return string(bz), true
	}

	// Check within hamming distance by iterating all SimHash entries
	prefix := types.SimHashPrefix
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return "", false
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) < len(prefix)+8 {
			continue
		}
		storedHash := binary.BigEndian.Uint64(key[len(prefix):])
		if hammingDistance(hash, storedHash) <= SimHashThreshold {
			return string(iter.Value()), true
		}
	}

	return "", false
}

// ─── Full duplicate check ───────────────────────────────────────────────────

// FullDuplicateCheck runs all dedup layers against the content.
// Returns: matchID (if found), isDuplicate, isNearDuplicate (for validator flagging).
func (k Keeper) FullDuplicateCheck(ctx context.Context, content string) (matchID string, isDuplicate bool, isNearDuplicate bool) {
	// Layer 1: exact hash
	exactHash := k.ComputeContentHash(content)
	if k.HasContentHash(ctx, exactHash) {
		return exactHash, true, false
	}

	// Layer 2: normalized hash
	if id, found := k.checkNormalizedDuplicate(ctx, content); found {
		return id, true, false
	}

	// Layer 3: SimHash (flag, don't reject)
	if id, found := k.checkSimHashDuplicate(ctx, content); found {
		return id, false, true
	}

	return "", false, false
}

// IndexContentForDedup stores all dedup indexes for a piece of content.
func (k Keeper) IndexContentForDedup(ctx context.Context, content, submissionID string) {
	// Normalized hash index
	normalizedHash := sha256Hex(normalizeContent(content))
	_ = k.SetNormalizedHash(ctx, normalizedHash, submissionID)

	// SimHash index
	simHash := computeSimHash(content)
	_ = k.SetSimHash(ctx, simHash, submissionID)
}

// ─── Content Integrity Invariants ───────────────────────────────────────────

// ContentIntegrityInvariant verifies that active samples have valid content hashes and tier/score alignment.
func ContentIntegrityInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		broken := false

		k.IterateSamples(ctx, func(sample *types.Sample) bool {
			// Skip non-active samples
			if sample.Status == types.SampleStatus_SAMPLE_STATUS_PRUNED ||
				sample.Status == types.SampleStatus_SAMPLE_STATUS_EXPIRED {
				return false
			}

			// 1. Thread references valid
			if sample.ParentSampleId != "" {
				if _, found := k.GetSample(ctx, sample.ParentSampleId); !found {
					msg += fmt.Sprintf("sample %s: broken parent ref %s\n", sample.Id, sample.ParentSampleId)
					broken = true
				}
			}

			// 2. Quality score / tier alignment
			if sample.QualityScore > 0 && sample.QualityTier != "" {
				expectedTier := qualityTierFromScore(sample.QualityScore)
				if expectedTier != sample.QualityTier {
					msg += fmt.Sprintf("sample %s: tier/score mismatch (score=%d, tier=%s, expected=%s)\n",
						sample.Id, sample.QualityScore, sample.QualityTier, expectedTier)
					broken = true
				}
			}

			return false
		})

		return msg, broken
	}
}

// EnergyConservationInvariant verifies no sample's energy exceeds its energy_cap.
func EnergyConservationInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		broken := false

		k.IterateSamples(ctx, func(sample *types.Sample) bool {
			if sample.EnergyCap > 0 && sample.Energy > sample.EnergyCap {
				msg += fmt.Sprintf("sample %s: energy %d exceeds cap %d\n",
					sample.Id, sample.Energy, sample.EnergyCap)
				broken = true
			}
			return false
		})

		return msg, broken
	}
}

// DuplicateHashInvariant verifies no two active samples share a content hash.
func DuplicateHashInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		broken := false
		seen := make(map[string]string) // hash → sampleID

		k.IterateSamples(ctx, func(sample *types.Sample) bool {
			if sample.Status == types.SampleStatus_SAMPLE_STATUS_PRUNED ||
				sample.Status == types.SampleStatus_SAMPLE_STATUS_EXPIRED {
				return false
			}
			if sample.Content == "" || sample.Content == "[consent revoked]" {
				return false
			}

			h := sha256.Sum256([]byte(sample.Content))
			hash := hex.EncodeToString(h[:])
			if prev, exists := seen[hash]; exists {
				msg += fmt.Sprintf("duplicate content hash: samples %s and %s\n", prev, sample.Id)
				broken = true
			}
			seen[hash] = sample.Id
			return false
		})

		return msg, broken
	}
}

// qualityTierFromScore derives the expected quality tier from a score.
func qualityTierFromScore(score uint64) string {
	switch {
	case score >= 800_000:
		return "gold"
	case score >= 500_000:
		return "silver"
	case score >= 200_000:
		return "bronze"
	default:
		return ""
	}
}
