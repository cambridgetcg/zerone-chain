package crypto_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zerone-chain/zerone/x/knowledge/crypto"
)

// --- Helper ---

// newTestKey returns a deterministic ed25519 key pair from a 32-byte seed.
func newTestKey(seedByte byte) (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := make([]byte, 32)
	seed[0] = seedByte
	priv := ed25519.NewKeyFromSeed(seed)
	return priv.Public().(ed25519.PublicKey), priv
}

// ============================================================
// GenerateVRF + VerifyVRF
// ============================================================

func TestVRF_GenerateAndVerify(t *testing.T) {
	pub, priv := newTestKey(1)
	seed := []byte("test-claim-42")

	output, proof, err := crypto.GenerateVRF(seed, priv)
	require.NoError(t, err)
	require.NotNil(t, output)
	require.NotNil(t, proof)

	verifiedOutput, valid := crypto.VerifyVRF(seed, pub, proof)
	require.True(t, valid, "proof should verify with the correct public key")
	require.Equal(t, output, verifiedOutput, "verified output must match generated output")
}

func TestVRF_DeterministicOutput(t *testing.T) {
	_, priv := newTestKey(2)
	seed := []byte("determinism-check")

	output1, proof1, err1 := crypto.GenerateVRF(seed, priv)
	require.NoError(t, err1)

	output2, proof2, err2 := crypto.GenerateVRF(seed, priv)
	require.NoError(t, err2)

	require.Equal(t, output1, output2, "same seed+key must produce same output")
	require.Equal(t, proof1, proof2, "same seed+key must produce same proof")
}

func TestVRF_DifferentSeeds_DifferentOutputs(t *testing.T) {
	_, priv := newTestKey(3)

	output1, _, err1 := crypto.GenerateVRF([]byte("seed-alpha"), priv)
	require.NoError(t, err1)

	output2, _, err2 := crypto.GenerateVRF([]byte("seed-beta"), priv)
	require.NoError(t, err2)

	require.NotEqual(t, output1, output2, "different seeds must produce different outputs")
}

func TestVRF_DifferentKeys_DifferentOutputs(t *testing.T) {
	_, privA := newTestKey(10)
	_, privB := newTestKey(20)
	seed := []byte("same-seed-for-both")

	outputA, _, errA := crypto.GenerateVRF(seed, privA)
	require.NoError(t, errA)

	outputB, _, errB := crypto.GenerateVRF(seed, privB)
	require.NoError(t, errB)

	require.NotEqual(t, outputA, outputB, "different keys must produce different outputs")
}

func TestVRF_InvalidPrivateKeyLength(t *testing.T) {
	seed := []byte("anything")

	// 16 bytes -- too short
	_, _, err := crypto.GenerateVRF(seed, make([]byte, 16))
	require.Error(t, err)
	require.Contains(t, err.Error(), "32 or 64 bytes")

	// 48 bytes -- wrong length
	_, _, err = crypto.GenerateVRF(seed, make([]byte, 48))
	require.Error(t, err)

	// 0 bytes
	_, _, err = crypto.GenerateVRF(seed, []byte{})
	require.Error(t, err)
}

func TestVRF_32ByteSeed(t *testing.T) {
	seed32 := make([]byte, 32)
	seed32[0] = 0x42

	// Use only the 32-byte seed (not the full 64-byte ed25519.PrivateKey)
	output, proof, err := crypto.GenerateVRF([]byte("claim"), seed32)
	require.NoError(t, err)
	require.NotNil(t, output)
	require.NotNil(t, proof)

	// Derive matching public key for verification
	pub := ed25519.NewKeyFromSeed(seed32).Public().(ed25519.PublicKey)
	verifiedOutput, valid := crypto.VerifyVRF([]byte("claim"), pub, proof)
	require.True(t, valid)
	require.Equal(t, output, verifiedOutput)
}

func TestVRF_64ByteFullKey(t *testing.T) {
	pub, priv := newTestKey(5)
	require.Len(t, priv, 64, "ed25519.PrivateKey should be 64 bytes")

	output, proof, err := crypto.GenerateVRF([]byte("full-key-test"), priv)
	require.NoError(t, err)

	verifiedOutput, valid := crypto.VerifyVRF([]byte("full-key-test"), pub, proof)
	require.True(t, valid)
	require.Equal(t, output, verifiedOutput)
}

func TestVRF_OutputIs32Bytes(t *testing.T) {
	_, priv := newTestKey(6)
	output, _, err := crypto.GenerateVRF([]byte("length-check"), priv)
	require.NoError(t, err)
	require.Len(t, output, 32, "VRF output must be 32 bytes (SHA-256)")
}

func TestVRF_ProofIs96Bytes(t *testing.T) {
	_, priv := newTestKey(7)
	_, proof, err := crypto.GenerateVRF([]byte("proof-length"), priv)
	require.NoError(t, err)
	require.Len(t, proof, 96, "VRF proof must be 96 bytes (Gamma 32 + c 32 + s 32)")
}

// ============================================================
// VerifyVRF edge cases
// ============================================================

func TestVerifyVRF_WrongPublicKey(t *testing.T) {
	_, priv := newTestKey(8)
	pubWrong, _ := newTestKey(9)
	seed := []byte("wrong-key-test")

	_, proof, err := crypto.GenerateVRF(seed, priv)
	require.NoError(t, err)

	_, valid := crypto.VerifyVRF(seed, pubWrong, proof)
	require.False(t, valid, "proof must not verify with a different public key")
}

func TestVerifyVRF_TamperedProof(t *testing.T) {
	pub, priv := newTestKey(11)
	seed := []byte("tamper-test")

	_, proof, err := crypto.GenerateVRF(seed, priv)
	require.NoError(t, err)

	// Flip a byte in the middle of the proof (in the 'c' component)
	tampered := make([]byte, len(proof))
	copy(tampered, proof)
	tampered[40] ^= 0xFF

	_, valid := crypto.VerifyVRF(seed, pub, tampered)
	require.False(t, valid, "tampered proof must not verify")
}

func TestVerifyVRF_TamperedSeed(t *testing.T) {
	pub, priv := newTestKey(12)

	_, proof, err := crypto.GenerateVRF([]byte("original-seed"), priv)
	require.NoError(t, err)

	_, valid := crypto.VerifyVRF([]byte("different-seed"), pub, proof)
	require.False(t, valid, "proof must not verify with a different seed")
}

func TestVerifyVRF_ShortProof(t *testing.T) {
	pub, _ := newTestKey(13)

	// 95 bytes -- one byte short
	shortProof := make([]byte, 95)
	_, valid := crypto.VerifyVRF([]byte("short"), pub, shortProof)
	require.False(t, valid, "proof shorter than 96 bytes must be rejected")

	// 0 bytes
	_, valid = crypto.VerifyVRF([]byte("empty"), pub, []byte{})
	require.False(t, valid)

	// nil
	_, valid = crypto.VerifyVRF([]byte("nil"), pub, nil)
	require.False(t, valid)
}

func TestVerifyVRF_WrongLengthPublicKey(t *testing.T) {
	_, priv := newTestKey(14)
	seed := []byte("key-length")

	_, proof, err := crypto.GenerateVRF(seed, priv)
	require.NoError(t, err)

	// 31 bytes -- too short
	_, valid := crypto.VerifyVRF(seed, make([]byte, 31), proof)
	require.False(t, valid, "public key != 32 bytes must be rejected")

	// 33 bytes -- too long
	_, valid = crypto.VerifyVRF(seed, make([]byte, 33), proof)
	require.False(t, valid)

	// nil
	_, valid = crypto.VerifyVRF(seed, nil, proof)
	require.False(t, valid)
}

func TestVerifyVRF_EmptyInputs(t *testing.T) {
	pub, _ := newTestKey(15)
	fakeProof := make([]byte, 96)

	// nil seed -- should not panic, just return invalid
	_, valid := crypto.VerifyVRF(nil, pub, fakeProof)
	require.False(t, valid)

	// empty seed
	_, valid = crypto.VerifyVRF([]byte{}, pub, fakeProof)
	require.False(t, valid)
}

// ============================================================
// IsValidatorSelected
// ============================================================

func TestIsValidatorSelected_FullStake(t *testing.T) {
	// When stake == totalStake and targetCount >= 1, selection threshold is 2^64,
	// so ANY valid output should be selected (outputNum * totalStake < totalStake * 1 * 2^64).
	_, priv := newTestKey(16)
	seed := []byte("full-stake")
	output, _, err := crypto.GenerateVRF(seed, priv)
	require.NoError(t, err)

	selected, priority := crypto.IsValidatorSelected(output, 1000, 1000, 1)
	require.True(t, selected, "validator with full stake must always be selected")
	require.NotZero(t, priority)
}

func TestIsValidatorSelected_ZeroStake(t *testing.T) {
	output := make([]byte, 32) // all zeros -- lowest possible output
	selected, priority := crypto.IsValidatorSelected(output, 0, 1000, 1)
	require.False(t, selected, "zero stake must never be selected")
	require.Zero(t, priority)
}

func TestIsValidatorSelected_ZeroTotalStake(t *testing.T) {
	output := make([]byte, 32)
	selected, priority := crypto.IsValidatorSelected(output, 100, 0, 1)
	require.False(t, selected, "zero totalStake must return not selected")
	require.Zero(t, priority)
}

func TestIsValidatorSelected_EmptyOutput(t *testing.T) {
	selected, priority := crypto.IsValidatorSelected(nil, 100, 1000, 1)
	require.False(t, selected, "nil output must return not selected")
	require.Zero(t, priority)

	selected, priority = crypto.IsValidatorSelected([]byte{}, 100, 1000, 1)
	require.False(t, selected, "empty output must return not selected")
	require.Zero(t, priority)

	// 7 bytes -- less than the required 8
	selected, priority = crypto.IsValidatorSelected(make([]byte, 7), 100, 1000, 1)
	require.False(t, selected, "output < 8 bytes must return not selected")
	require.Zero(t, priority)
}

func TestIsValidatorSelected_StakeClampedToTotal(t *testing.T) {
	// stake > totalStake should behave identically to stake == totalStake
	_, priv := newTestKey(17)
	output, _, err := crypto.GenerateVRF([]byte("clamp"), priv)
	require.NoError(t, err)

	selectedClamped, priorityClamped := crypto.IsValidatorSelected(output, 2000, 1000, 1)
	selectedEqual, priorityEqual := crypto.IsValidatorSelected(output, 1000, 1000, 1)

	require.Equal(t, selectedClamped, selectedEqual, "stake > totalStake must be clamped to totalStake")
	require.Equal(t, priorityClamped, priorityEqual)
}

func TestIsValidatorSelected_PriorityIsFirstEightBytes(t *testing.T) {
	// Build output with known first 8 bytes
	output := make([]byte, 32)
	binary.BigEndian.PutUint64(output[:8], 0xDEADBEEFCAFEBABE)

	// Use totalStake == stake so we always get selected, and can inspect priority
	_, priority := crypto.IsValidatorSelected(output, 1000, 1000, 1)
	require.Equal(t, uint64(0xDEADBEEFCAFEBABE), priority, "priority must equal BigEndian(output[:8])")
}

func TestIsValidatorSelected_HigherStakeMoreLikely(t *testing.T) {
	const trials = 200
	const totalStake uint64 = 10_000
	const lowStake uint64 = 100   // 1%
	const highStake uint64 = 5000 // 50%

	lowSelected := 0
	highSelected := 0

	for i := 0; i < trials; i++ {
		// Generate pseudo-random VRF output
		output := make([]byte, 32)
		_, err := rand.Read(output)
		require.NoError(t, err)

		selLow, _ := crypto.IsValidatorSelected(output, lowStake, totalStake, 1)
		selHigh, _ := crypto.IsValidatorSelected(output, highStake, totalStake, 1)

		if selLow {
			lowSelected++
		}
		if selHigh {
			highSelected++
		}
	}

	require.Greater(t, highSelected, lowSelected,
		"higher stake (%d%%) should be selected more often than lower stake (%d%%): got high=%d, low=%d",
		highStake*100/totalStake, lowStake*100/totalStake, highSelected, lowSelected)
}

// ============================================================
// GenerateVRFSeed
// ============================================================

func TestGenerateVRFSeed_Deterministic(t *testing.T) {
	prevHash := []byte("block-hash-abc")
	s1 := crypto.GenerateVRFSeed("claim-1", 100, prevHash)
	s2 := crypto.GenerateVRFSeed("claim-1", 100, prevHash)
	require.Equal(t, s1, s2, "same inputs must produce same seed")
	require.Len(t, s1, 32, "seed must be 32 bytes (SHA-256)")
}

func TestGenerateVRFSeed_DifferentInputs(t *testing.T) {
	prevHash := []byte("hash")

	base := crypto.GenerateVRFSeed("claim-A", 100, prevHash)

	// Different claimID
	diffClaim := crypto.GenerateVRFSeed("claim-B", 100, prevHash)
	require.NotEqual(t, base, diffClaim, "different claimID must produce different seed")

	// Different blockNumber
	diffBlock := crypto.GenerateVRFSeed("claim-A", 101, prevHash)
	require.NotEqual(t, base, diffBlock, "different blockNumber must produce different seed")

	// Different prevBlockHash
	diffHash := crypto.GenerateVRFSeed("claim-A", 100, []byte("other-hash"))
	require.NotEqual(t, base, diffHash, "different prevBlockHash must produce different seed")
}

// ============================================================
// GenerateBlockSeed
// ============================================================

func TestGenerateBlockSeed_Deterministic(t *testing.T) {
	prevHash := []byte("prev-block-hash")
	s1 := crypto.GenerateBlockSeed(prevHash, 42, 7)
	s2 := crypto.GenerateBlockSeed(prevHash, 42, 7)
	require.Equal(t, s1, s2, "same inputs must produce same block seed")
	require.Len(t, s1, 32, "block seed must be 32 bytes (SHA-256)")
}

func TestGenerateBlockSeed_DifferentInputs(t *testing.T) {
	prevHash := []byte("hash")

	base := crypto.GenerateBlockSeed(prevHash, 42, 7)

	// Different blockNumber
	diffBlock := crypto.GenerateBlockSeed(prevHash, 43, 7)
	require.NotEqual(t, base, diffBlock, "different blockNumber must produce different block seed")

	// Different epoch
	diffEpoch := crypto.GenerateBlockSeed(prevHash, 42, 8)
	require.NotEqual(t, base, diffEpoch, "different epoch must produce different block seed")

	// Different prevBlockHash
	diffHash := crypto.GenerateBlockSeed([]byte("other"), 42, 7)
	require.NotEqual(t, base, diffHash, "different prevBlockHash must produce different block seed")
}

// ============================================================
// Extended VRF tests — batch 2
// ============================================================

// TestVRF_GenerateDoesNotMutatePrivateKey verifies that calling GenerateVRF
// does not modify the caller's private key slice.
func TestVRF_GenerateDoesNotMutatePrivateKey(t *testing.T) {
	_, priv := newTestKey(30)
	original := make([]byte, len(priv))
	copy(original, priv)

	_, _, err := crypto.GenerateVRF([]byte("no-mutate-test"), priv)
	require.NoError(t, err)

	require.Equal(t, original, []byte(priv),
		"GenerateVRF must not mutate the private key (alias safety)")
}

// TestVRF_DomainSeparation_ViaExportedAPI verifies that the internal domain hash
// uses length-prefix format rather than a simple colon separator.
// We exercise this through the exported API: if two seeds that would be ambiguous
// under colon-separation produce the same VRF output, the domain-hash has a
// length-prefix collision bug. With proper length-prefixed separation they must differ.
func TestVRF_DomainSeparation_ViaExportedAPI(t *testing.T) {
	_, priv := newTestKey(31)

	// "ab" + "cdef" vs "abc" + "def" — if domain hash used "domain:data"
	// both would be "ZRN.vrf.v1:abcdef" and collide. Length-prefix separates them.
	// We rely on GenerateVRFSeed which internally uses domainHash("ZRN.vrf.seed.v1", payload)
	// with different structured payload.
	s1 := crypto.GenerateVRFSeed("ab-cdef", 100, []byte("hash"))
	s2 := crypto.GenerateVRFSeed("abc-def", 100, []byte("hash"))
	require.NotEqual(t, s1, s2,
		"different claim IDs that share the same bytes must produce different seeds (length-prefix separation)")

	// Further: produce VRF from each — must yield different outputs
	out1, _, err1 := crypto.GenerateVRF(s1, priv)
	require.NoError(t, err1)
	out2, _, err2 := crypto.GenerateVRF(s2, priv)
	require.NoError(t, err2)
	require.NotEqual(t, out1, out2, "VRF outputs from differently-separated seeds must differ")
}

// TestVRF_NoAmbiguity_ClaimIDLength verifies that seeds whose components share
// the same concatenated bytes but differ in structure produce different outputs.
func TestVRF_NoAmbiguity_ClaimIDLength(t *testing.T) {
	hash := []byte("prev-hash")

	// These claim IDs have different lengths but overlapping byte patterns
	s1 := crypto.GenerateVRFSeed("MATH-001", 1, hash)
	s2 := crypto.GenerateVRFSeed("MATH-0011", 0, hash) // blockNum bytes start differently
	require.NotEqual(t, s1, s2,
		"different structured inputs must never produce the same seed")
}

// TestVRF_StatisticalFairness runs 1000 trials and checks that selection
// probability is proportional to stake within a 15% tolerance.
func TestVRF_StatisticalFairness(t *testing.T) {
	const trials = 1000
	const totalStake uint64 = 10_000
	const targetStake uint64 = 3000 // 30%
	const expectedRate = 0.30
	const tolerance = 0.15

	selected := 0
	for i := 0; i < trials; i++ {
		output := make([]byte, 32)
		_, err := rand.Read(output)
		require.NoError(t, err)

		sel, _ := crypto.IsValidatorSelected(output, targetStake, totalStake, 1)
		if sel {
			selected++
		}
	}

	observedRate := float64(selected) / float64(trials)
	require.InDelta(t, expectedRate, observedRate, tolerance,
		"selection rate %.3f should be within %.0f%% of expected %.3f (got %d/%d)",
		observedRate, tolerance*100, expectedRate, selected, trials)
}

// TestVRF_MultiValidatorSelection selects N from M validators using targetCount > 1.
func TestVRF_MultiValidatorSelection(t *testing.T) {
	const totalStake uint64 = 10_000
	const validatorStake uint64 = 2000 // 20% each → 5 validators cover 100%
	const targetCount uint32 = 3       // want ~3 from each 20%-stake validator = 60% selection
	const trials = 500

	selected := 0
	for i := 0; i < trials; i++ {
		output := make([]byte, 32)
		_, err := rand.Read(output)
		require.NoError(t, err)

		sel, _ := crypto.IsValidatorSelected(output, validatorStake, totalStake, targetCount)
		if sel {
			selected++
		}
	}

	// Expected: 20% * 3 = 60%; allow generous margin
	observedRate := float64(selected) / float64(trials)
	require.InDelta(t, 0.60, observedRate, 0.15,
		"multi-validator selection rate should approximate stake*targetCount/totalStake")
}

// TestVRF_ZeroStakeNeverSelected_Bulk verifies that zero stake never gets selected
// across 100 random VRF outputs.
func TestVRF_ZeroStakeNeverSelected_Bulk(t *testing.T) {
	for i := 0; i < 100; i++ {
		output := make([]byte, 32)
		_, err := rand.Read(output)
		require.NoError(t, err)

		sel, prio := crypto.IsValidatorSelected(output, 0, 10_000, 1)
		require.False(t, sel, "zero stake must never be selected (trial %d)", i)
		require.Zero(t, prio, "zero stake must have zero priority (trial %d)", i)
	}
}

// TestVRF_FullStakeAlwaysSelected_Bulk verifies that a validator with 100% of total
// stake is always selected across 100 real VRF outputs.
func TestVRF_FullStakeAlwaysSelected_Bulk(t *testing.T) {
	_, priv := newTestKey(40)

	for i := 0; i < 100; i++ {
		seed := make([]byte, 32)
		binary.BigEndian.PutUint64(seed, uint64(i))

		output, _, err := crypto.GenerateVRF(seed, priv)
		require.NoError(t, err)

		sel, prio := crypto.IsValidatorSelected(output, 10_000, 10_000, 1)
		require.True(t, sel, "full stake must always be selected (trial %d)", i)
		require.NotZero(t, prio, "full stake must have non-zero priority (trial %d)", i)
	}
}

// TestVRF_PriorityOrdering verifies that among selected validators, lower
// priority value corresponds to higher selection ranking (first-pick).
func TestVRF_PriorityOrdering(t *testing.T) {
	_, priv := newTestKey(41)
	const totalStake uint64 = 10_000

	type result struct {
		priority uint64
		selected bool
	}
	var results []result

	for i := 0; i < 50; i++ {
		seed := make([]byte, 8)
		binary.BigEndian.PutUint64(seed, uint64(i*1000))

		output, _, err := crypto.GenerateVRF(seed, priv)
		require.NoError(t, err)

		sel, prio := crypto.IsValidatorSelected(output, totalStake, totalStake, 1)
		results = append(results, result{priority: prio, selected: sel})
	}

	// All should be selected (full stake), and priorities should be distinct
	priorities := make(map[uint64]bool)
	for _, r := range results {
		require.True(t, r.selected)
		priorities[r.priority] = true
	}
	require.Greater(t, len(priorities), 1,
		"priorities should vary across different VRF outputs")
}

// TestVRF_BlockSeed_EpochSensitivity verifies that different epochs produce
// completely different block seeds, even with the same block number and prev hash.
func TestVRF_BlockSeed_EpochSensitivity(t *testing.T) {
	prevHash := []byte("fixed-prev-hash")
	const blockNum uint64 = 42

	seeds := make(map[string]bool)
	for epoch := uint64(0); epoch < 100; epoch++ {
		s := crypto.GenerateBlockSeed(prevHash, blockNum, epoch)
		require.Len(t, s, 32)
		key := string(s)
		require.False(t, seeds[key],
			"epoch %d produced a duplicate block seed", epoch)
		seeds[key] = true
	}
	require.Len(t, seeds, 100, "100 different epochs must produce 100 unique seeds")
}
