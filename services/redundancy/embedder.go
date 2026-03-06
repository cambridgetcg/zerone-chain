package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"strings"
	"unicode"
)

// TDURecord represents a training data unit with content and metadata for clustering.
type TDURecord struct {
	ID           string  `json:"id"`
	Content      string  `json:"content"`
	Domain       string  `json:"domain,omitempty"`
	Category     string  `json:"category,omitempty"`
	FitnessScore float64 `json:"fitness_score,omitempty"` // current fitness [0.0, 1.0]
	IsCorrection bool    `json:"is_correction,omitempty"` // correction TDUs get canonical priority
	CreatedAt    uint64  `json:"created_at,omitempty"`    // block height or timestamp for tie-breaking

	// Precomputed fingerprint (not serialized)
	simHash uint64
}

// Embedder computes SimHash fingerprints for TDU content in batches.
type Embedder struct {
	batchSize int
}

// NewEmbedder creates a new Embedder with configurable batch size.
func NewEmbedder(batchSize int) *Embedder {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &Embedder{batchSize: batchSize}
}

// LoadAndEmbed reads a JSONL file of TDU records and computes SimHash fingerprints.
func (e *Embedder) LoadAndEmbed(path string) ([]TDURecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open TDU file: %w", err)
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
			continue // skip malformed
		}
		if rec.ID == "" || rec.Content == "" {
			continue
		}
		records = append(records, rec)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan TDU file: %w", err)
	}

	e.EmbedBatch(records)
	return records, nil
}

// EmbedBatch computes SimHash fingerprints for a slice of TDU records in-place.
func (e *Embedder) EmbedBatch(records []TDURecord) {
	for i := 0; i < len(records); i += e.batchSize {
		end := i + e.batchSize
		if end > len(records) {
			end = len(records)
		}
		for j := i; j < end; j++ {
			records[j].simHash = computeSimHash(records[j].Content)
		}
	}
}

// Similarity returns the cosine-like similarity between two TDU records.
// Mapped from hamming distance: similarity = 1.0 - (hamming / 64.0).
func Similarity(a, b *TDURecord) float64 {
	dist := hammingDistance(a.simHash, b.simHash)
	return 1.0 - float64(dist)/64.0
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
