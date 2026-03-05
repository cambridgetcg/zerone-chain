package marketplace

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

// Watermarker applies buyer-specific fingerprinting to dataset copies.
// Each buyer gets a unique sample ordering and minor formatting variations,
// enabling identification of the source if a dataset leaks.
type Watermarker struct{}

// NewWatermarker creates a watermarker.
func NewWatermarker() *Watermarker {
	return &Watermarker{}
}

// Permutation generates a deterministic sample ordering for a buyer.
// Given the same seed and count, always produces the same permutation.
func (w *Watermarker) Permutation(seed string, sampleCount int) []int {
	indices := make([]int, sampleCount)
	for i := range indices {
		indices[i] = i
	}

	// Derive per-buyer ordering from seed
	h := sha256.Sum256([]byte(seed))
	rng := newDeterministicRNG(h[:])

	// Fisher-Yates shuffle
	for i := sampleCount - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
	}

	return indices
}

// Fingerprint generates a unique fingerprint for a buyer's copy.
// This can be embedded in metadata or used for verification.
func (w *Watermarker) Fingerprint(seed, datasetID string) string {
	h := sha256.Sum256([]byte(seed + ":" + datasetID))
	return string(encodeHex(h[:8]))
}

// IdentifySource attempts to identify the buyer from a leaked dataset
// by comparing sample ordering against known buyer seeds.
func (w *Watermarker) IdentifySource(sampleOrder []int, candidates []string, sampleCount int) (string, float64) {
	bestSeed := ""
	bestScore := 0.0

	for _, seed := range candidates {
		expected := w.Permutation(seed, sampleCount)
		score := orderCorrelation(sampleOrder, expected)
		if score > bestScore {
			bestScore = score
			bestSeed = seed
		}
	}

	return bestSeed, bestScore
}

// orderCorrelation computes the Spearman rank correlation between two orderings.
func orderCorrelation(a, b []int) float64 {
	n := len(a)
	if n != len(b) || n == 0 {
		return 0
	}

	// Build rank maps
	rankA := rankMap(a)
	rankB := rankMap(b)

	// Spearman's rho: 1 - (6 * sum(d^2)) / (n * (n^2 - 1))
	sumD2 := 0.0
	for i := 0; i < n; i++ {
		d := float64(rankA[i] - rankB[i])
		sumD2 += d * d
	}

	return 1 - (6*sumD2)/(float64(n)*(float64(n)*float64(n)-1))
}

func rankMap(order []int) map[int]int {
	ranks := make(map[int]int, len(order))
	for rank, val := range order {
		ranks[val] = rank
	}
	return ranks
}

// deterministicRNG is a simple deterministic PRNG seeded from a hash.
type deterministicRNG struct {
	state []byte
	pos   int
}

func newDeterministicRNG(seed []byte) *deterministicRNG {
	s := make([]byte, len(seed))
	copy(s, seed)
	return &deterministicRNG{state: s}
}

func (r *deterministicRNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	// Generate more bytes when needed
	if r.pos+4 > len(r.state) {
		h := sha256.Sum256(r.state)
		r.state = h[:]
		r.pos = 0
	}
	val := binary.BigEndian.Uint32(r.state[r.pos : r.pos+4])
	r.pos += 4
	return int(val % uint32(n))
}

func encodeHex(b []byte) []byte {
	const hexChars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexChars[v>>4]
		out[i*2+1] = hexChars[v&0x0f]
	}
	return out
}

// SortedIndices returns sorted copy of indices (utility).
func SortedIndices(indices []int) []int {
	sorted := make([]int, len(indices))
	copy(sorted, indices)
	sort.Ints(sorted)
	return sorted
}
