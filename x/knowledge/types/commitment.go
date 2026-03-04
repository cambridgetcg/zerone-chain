package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ComputeCommitmentHash generates a commitment hash for a verification verdict.
// The hash is SHA-256("ZRN.commit.v1:" + roundID + ":" + vote + ":" + confidence + ":" + hex(salt)).
// Domain-separated to prevent cross-protocol replay.
func ComputeCommitmentHash(roundID, vote string, confidence uint64, salt []byte) []byte {
	h := sha256.New()
	h.Write([]byte("ZRN.commit.v1:"))
	h.Write([]byte(roundID))
	h.Write([]byte(":"))
	h.Write([]byte(vote))
	h.Write([]byte(":"))
	h.Write([]byte(fmt.Sprint(confidence)))
	h.Write([]byte(":"))
	h.Write([]byte(hex.EncodeToString(salt)))
	return h.Sum(nil)
}

// VerifyCommitmentHash checks that a reveal matches its prior commitment hash.
func VerifyCommitmentHash(hash []byte, roundID, vote string, confidence uint64, salt []byte) bool {
	expected := ComputeCommitmentHash(roundID, vote, confidence, salt)
	return bytes.Equal(hash, expected)
}

// ComputeQualityCommitHash generates a commitment hash for a QualityVote.
// Hash = SHA-256("ZRN.quality.commit.v1:" + roundID + ":" + deterministic_fields + ":" + hex(salt))
func ComputeQualityCommitHash(roundID string, vote *QualityVote, salt []byte) []byte {
	h := sha256.New()
	h.Write([]byte("ZRN.quality.commit.v1:"))
	h.Write([]byte(roundID))
	h.Write([]byte(":"))
	// Deterministic serialization via field concatenation (not JSON — JSON field order not guaranteed)
	h.Write([]byte(fmt.Sprintf("%d:%d:%d:%d:%d:%t:%t",
		vote.OverallQuality,
		vote.ReasoningDepth,
		vote.Novelty,
		vote.Toxicity,
		vote.FactualAccuracy,
		vote.ConsentValid,
		vote.Duplicate,
	)))
	h.Write([]byte(":"))
	h.Write([]byte(hex.EncodeToString(salt)))
	return h.Sum(nil)
}

// VerifyQualityCommitHash checks that a reveal matches its prior quality commitment hash.
func VerifyQualityCommitHash(hash []byte, roundID string, vote *QualityVote, salt []byte) bool {
	expected := ComputeQualityCommitHash(roundID, vote, salt)
	return bytes.Equal(hash, expected)
}
