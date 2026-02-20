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
