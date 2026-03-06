package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"os"
	"sort"
	"strings"
	"unicode"
)

// TDURecord represents a training data unit with its content for similarity matching.
type TDURecord struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Domain   string `json:"domain,omitempty"`
	Category string `json:"category,omitempty"`

	// Precomputed fingerprint
	simHash uint64
}

// AttributedSignal ties a raw signal to a specific TDU with a similarity weight.
type AttributedSignal struct {
	TDUID      string  `json:"tdu_id"`
	Signal     float64 `json:"signal"`     // [-1.0, 1.0]
	Similarity float64 `json:"similarity"` // [0.0, 1.0] attribution weight
	Domain     string  `json:"domain,omitempty"`
}

// Attributor maps usage events to TDU IDs via content similarity.
type Attributor struct {
	tdus     []TDURecord
	topK     int
	maxDist  int // max hamming distance for attribution
}

// NewAttributor creates a new Attributor with configured topK and max hamming distance.
func NewAttributor(topK, maxDist int) *Attributor {
	return &Attributor{
		topK:    topK,
		maxDist: maxDist,
	}
}

// LoadTDUIndex reads a JSONL file of TDU records and precomputes SimHash fingerprints.
func (a *Attributor) LoadTDUIndex(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open TDU index: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	var records []TDURecord
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec TDURecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec.ID == "" || rec.Content == "" {
			continue
		}
		rec.simHash = computeSimHash(rec.Content)
		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan TDU index: %w", err)
	}

	a.tdus = records
	return nil
}

// SetTDUs sets TDU records directly (for testing).
func (a *Attributor) SetTDUs(tdus []TDURecord) {
	for i := range tdus {
		tdus[i].simHash = computeSimHash(tdus[i].Content)
	}
	a.tdus = tdus
}

// Attribute maps a usage event to the top-K most similar TDUs and returns
// attributed signals with similarity-decayed weights.
func (a *Attributor) Attribute(event *UsageEvent, rawSignal float64) []AttributedSignal {
	if len(a.tdus) == 0 {
		return nil
	}

	queryHash := computeSimHash(event.Query)

	// Find distances to all TDUs
	type scored struct {
		idx  int
		dist int
	}
	var candidates []scored

	for i, tdu := range a.tdus {
		dist := hammingDistance(queryHash, tdu.simHash)
		if dist <= a.maxDist {
			candidates = append(candidates, scored{idx: i, dist: dist})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by distance (closest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	// Take top-K
	k := a.topK
	if k > len(candidates) {
		k = len(candidates)
	}
	candidates = candidates[:k]

	// Convert hamming distance to similarity score [0.0, 1.0]
	// dist=0 → sim=1.0, dist=maxDist → sim≈0.0
	results := make([]AttributedSignal, 0, k)
	for _, c := range candidates {
		similarity := 1.0 - float64(c.dist)/float64(a.maxDist+1)
		similarity = math.Max(0, similarity)

		results = append(results, AttributedSignal{
			TDUID:      a.tdus[c.idx].ID,
			Signal:     rawSignal * similarity, // decay signal with distance
			Similarity: math.Round(similarity*10000) / 10000,
			Domain:     a.tdus[c.idx].Domain,
		})
	}

	return results
}

// ─── SimHash (replicates R39-2 algorithm from x/knowledge/keeper/integrity.go) ─

// computeSimHash computes a SimHash fingerprint from 3-gram features.
func computeSimHash(content string) uint64 {
	text := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, content)

	words := strings.Fields(text)
	if len(words) < 3 {
		h := sha256.Sum256([]byte(text))
		return binary.BigEndian.Uint64(h[:8])
	}

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
