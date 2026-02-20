// Package crypto provides VRF (Verifiable Random Function) for fair validator selection.
//
// Based on ECVRF using Ed25519 (RFC 9381 simplified).
// Properties:
// - Uniqueness: For any input, only one valid output per key
// - Unpredictability: Output cannot be predicted without private key
// - Verifiability: Anyone can verify output with public key
package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"filippo.io/edwards25519"
)

var (
	ErrInvalidProofLength = errors.New("VRF proof must be 96 bytes (32 Gamma + 32 c + 32 s)")
	ErrInvalidPublicKey   = errors.New("invalid Ed25519 public key")
	ErrInvalidProof       = errors.New("VRF proof verification failed")
	ErrHashToCurveFailed  = errors.New("failed to hash to curve after 256 attempts")

	// curveOrder is the Ed25519 group order (ℓ).
	curveOrder, _ = new(big.Int).SetString("7237005577332262213973186563042994240857116359379907606001950938285454250989", 10)
)

// GenerateVRF produces a VRF output and proof from a seed and Ed25519 private key.
// The private key should be a 64-byte Ed25519 private key (seed + public key)
// or a 32-byte seed.
func GenerateVRF(seed, privateKey []byte) (output, proof []byte, err error) {
	// Normalize private key to 32-byte seed
	var skSeed []byte
	switch len(privateKey) {
	case 64:
		skSeed = privateKey[:32]
	case 32:
		skSeed = privateKey
	default:
		return nil, nil, fmt.Errorf("private key must be 32 or 64 bytes, got %d", len(privateKey))
	}

	// Domain-separate the seed
	domainSeparated := domainHash("ZRN.vrf.v1", seed)

	// H = hash_to_curve(domain_separated_input)
	H, err := hashToCurve(domainSeparated)
	if err != nil {
		return nil, nil, err
	}

	// Derive scalar from Ed25519 seed (SHA-512 + clamp)
	skScalar := deriveScalarFromSeed(skSeed)

	// Gamma = sk * H
	Gamma := new(edwards25519.Point).ScalarMult(skScalar, H)

	// Generate deterministic nonce: k = hash(sk_seed || H_bytes) mod ℓ
	// Must NOT use append(skSeed, ...) — skSeed may alias privateKey[:32] with cap=64,
	// and append would overwrite the public key portion of the caller's key.
	hBytes := H.Bytes()
	nonceInput := make([]byte, len(skSeed)+len(hBytes))
	copy(nonceInput, skSeed)
	copy(nonceInput[len(skSeed):], hBytes)
	kHash := sha512.Sum512(nonceInput)
	k := scalarFromBytes(kHash[:32])

	// U = k * G
	G := edwards25519.NewGeneratorPoint()
	U := new(edwards25519.Point).ScalarMult(k, G)

	// V = k * H
	V := new(edwards25519.Point).ScalarMult(k, H)

	// Derive public key for challenge computation
	pk := ed25519.NewKeyFromSeed(skSeed).Public().(ed25519.PublicKey)

	// c = hash(H, pk, Gamma, U, V) mod ℓ
	c := computeChallenge(H.Bytes(), pk, Gamma.Bytes(), U.Bytes(), V.Bytes())

	// s = k - c * sk (mod ℓ)
	cTimesSk := new(edwards25519.Scalar).Multiply(c, skScalar)
	s := new(edwards25519.Scalar).Subtract(k, cTimesSk)

	// Output = SHA-256(Gamma)
	outputHash := sha256.Sum256(Gamma.Bytes())
	output = outputHash[:]

	// Proof = (Gamma || c || s) = 96 bytes
	proof = make([]byte, 96)
	copy(proof[0:32], Gamma.Bytes())
	copy(proof[32:64], c.Bytes())
	copy(proof[64:96], s.Bytes())

	return output, proof, nil
}

// VerifyVRF verifies a VRF proof and returns the output if valid.
func VerifyVRF(seed, publicKey, proof []byte) (output []byte, valid bool) {
	if len(proof) != 96 {
		return nil, false
	}
	if len(publicKey) != 32 {
		return nil, false
	}

	// Parse proof components
	Gamma, err := new(edwards25519.Point).SetBytes(proof[0:32])
	if err != nil {
		return nil, false
	}
	c, err := new(edwards25519.Scalar).SetCanonicalBytes(proof[32:64])
	if err != nil {
		return nil, false
	}
	s, err := new(edwards25519.Scalar).SetCanonicalBytes(proof[64:96])
	if err != nil {
		return nil, false
	}

	// Domain-separate the seed
	domainSeparated := domainHash("ZRN.vrf.v1", seed)

	// H = hash_to_curve(domain_separated_input)
	H, err := hashToCurve(domainSeparated)
	if err != nil {
		return nil, false
	}

	// Parse public key as point
	PK, err := new(edwards25519.Point).SetBytes(publicKey)
	if err != nil {
		return nil, false
	}

	// U' = s*G + c*PK
	G := edwards25519.NewGeneratorPoint()
	sG := new(edwards25519.Point).ScalarMult(s, G)
	cPK := new(edwards25519.Point).ScalarMult(c, PK)
	Uprime := new(edwards25519.Point).Add(sG, cPK)

	// V' = s*H + c*Gamma
	sH := new(edwards25519.Point).ScalarMult(s, H)
	cGamma := new(edwards25519.Point).ScalarMult(c, Gamma)
	Vprime := new(edwards25519.Point).Add(sH, cGamma)

	// c' = hash(H, pk, Gamma, U', V')
	cPrime := computeChallenge(H.Bytes(), publicKey, Gamma.Bytes(), Uprime.Bytes(), Vprime.Bytes())

	// Verify c == c'
	if c.Equal(cPrime) != 1 {
		return nil, false
	}

	// Output = SHA-256(Gamma)
	outputHash := sha256.Sum256(Gamma.Bytes())
	return outputHash[:], true
}

// IsValidatorSelected checks if a validator is selected based on VRF output and stake.
// Selection probability = (stake / totalStake) * targetCount.
// Returns selected status and priority (lower = higher priority).
//
// All intermediate arithmetic uses math/big.Int to prevent overflow:
// the comparison outputNum * totalStake < stake * targetCount * 2^64
// can exceed uint64 range when totalStake or stake are large.
func IsValidatorSelected(vrfOutput []byte, stake, totalStake uint64, targetCount uint32) (selected bool, priority uint64) {
	if len(vrfOutput) < 8 || totalStake == 0 || stake == 0 {
		return false, 0
	}

	// Clamp stake to totalStake (defensive — should never exceed)
	if stake > totalStake {
		stake = totalStake
	}

	// Use first 8 bytes as priority value
	outputNum := binary.BigEndian.Uint64(vrfOutput[:8])

	// 2^64 as big.Int
	maxNum := new(big.Int).Lsh(big.NewInt(1), 64)

	// Selection: outputNum * totalStake < stake * targetCount * 2^64
	// All operations in big.Int to prevent overflow
	left := new(big.Int).SetUint64(outputNum)
	left.Mul(left, new(big.Int).SetUint64(totalStake))

	right := new(big.Int).SetUint64(stake)
	right.Mul(right, new(big.Int).SetUint64(uint64(targetCount)))
	right.Mul(right, maxNum)

	selected = left.Cmp(right) < 0
	priority = outputNum

	return selected, priority
}

// GenerateVRFSeed creates a deterministic seed for verification round VRF.
func GenerateVRFSeed(claimID string, blockNumber uint64, prevBlockHash []byte) []byte {
	claimBytes := []byte(claimID)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, blockNumber)

	// Build payload: uint32BE(len(claimID)) || claimID || blockNumber(8) || prevBlockHash
	payload := make([]byte, 4+len(claimBytes)+8+len(prevBlockHash))
	binary.BigEndian.PutUint32(payload[0:4], uint32(len(claimBytes)))
	copy(payload[4:], claimBytes)
	copy(payload[4+len(claimBytes):], buf)
	copy(payload[4+len(claimBytes)+8:], prevBlockHash)

	return domainHash("ZRN.vrf.seed.v1", payload)
}

// GenerateBlockSeed creates a deterministic seed for block proposal VRF.
func GenerateBlockSeed(prevBlockHash []byte, blockNumber, epoch uint64) []byte {
	payload := make([]byte, len(prevBlockHash)+16)
	copy(payload, prevBlockHash)
	binary.BigEndian.PutUint64(payload[len(prevBlockHash):], blockNumber)
	binary.BigEndian.PutUint64(payload[len(prevBlockHash)+8:], epoch)

	return domainHash("ZRN.vrf.blockseed.v1", payload)
}

// ---- Internal helpers ----

// domainHash produces a domain-separated SHA-256 hash.
// Format: SHA-256(domain || uint32BE(len(domain)) || data)
func domainHash(domain string, data []byte) []byte {
	domainBytes := []byte(domain)
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(domainBytes)))

	h := sha256.New()
	h.Write(domainBytes)
	h.Write(lengthBytes)
	h.Write(data)
	return h.Sum(nil)
}

// deriveScalarFromSeed derives the Ed25519 scalar from a 32-byte seed.
func deriveScalarFromSeed(seed []byte) *edwards25519.Scalar {
	h := sha512.Sum512(seed)
	s, err := new(edwards25519.Scalar).SetBytesWithClamping(h[:32])
	if err != nil {
		scalar := make([]byte, 32)
		copy(scalar, h[:32])
		scalar[0] &= 248
		scalar[31] &= 127
		scalar[31] |= 64
		s, _ = new(edwards25519.Scalar).SetCanonicalBytes(scalar)
	}
	return s
}

// hashToCurve implements a simplified hash-to-curve for Ed25519.
func hashToCurve(input []byte) (*edwards25519.Point, error) {
	for attempt := 0; attempt < 256; attempt++ {
		buf := make([]byte, len(input)+1)
		copy(buf, input)
		buf[len(input)] = byte(attempt)
		h := sha512.Sum512(buf)
		point, err := new(edwards25519.Point).SetBytes(h[:32])
		if err != nil {
			continue
		}

		// Multiply by cofactor (8) to clear low-order components
		eight := scalarFromUint64(8)
		cleared := new(edwards25519.Point).ScalarMult(eight, point)

		if cleared.Equal(edwards25519.NewIdentityPoint()) == 1 {
			continue
		}

		return cleared, nil
	}
	return nil, ErrHashToCurveFailed
}

// computeChallenge computes c = SHA-512(H || pk || Gamma || U || V) mod ℓ.
func computeChallenge(h, pk, gamma, u, v []byte) *edwards25519.Scalar {
	combined := make([]byte, 0, len(h)+len(pk)+len(gamma)+len(u)+len(v))
	combined = append(combined, h...)
	combined = append(combined, pk...)
	combined = append(combined, gamma...)
	combined = append(combined, u...)
	combined = append(combined, v...)

	hash := sha512.Sum512(combined)

	num := new(big.Int).SetBytes(hash[:24])
	num.Mod(num, curveOrder)

	b := num.Bytes()
	le := make([]byte, 32)
	for i, v := range b {
		le[len(b)-1-i] = v
	}

	s, err := new(edwards25519.Scalar).SetCanonicalBytes(le)
	if err != nil {
		return new(edwards25519.Scalar)
	}
	return s
}

// scalarFromBytes creates a scalar from 32 bytes by reducing mod ℓ.
func scalarFromBytes(b []byte) *edwards25519.Scalar {
	num := new(big.Int).SetBytes(b)
	num.Mod(num, curveOrder)

	be := num.Bytes()
	le := make([]byte, 32)
	for i, v := range be {
		le[len(be)-1-i] = v
	}

	s, err := new(edwards25519.Scalar).SetCanonicalBytes(le)
	if err != nil {
		return new(edwards25519.Scalar)
	}
	return s
}

// scalarFromUint64 creates a scalar from a uint64 value.
func scalarFromUint64(v uint64) *edwards25519.Scalar {
	b := make([]byte, 32)
	binary.LittleEndian.PutUint64(b, v)
	s, _ := new(edwards25519.Scalar).SetCanonicalBytes(b)
	return s
}
