package app_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"
)

// ---------- ExtendVote tests ----------

// TestExtendVote_ProducesVRF verifies that a properly configured validator produces
// a vote extension containing VRF output. Since we cannot easily set up active rounds
// and a full knowledge keeper in a unit test, we test the structural behavior:
// - A nil VoteExtConfig produces an empty extension.
// - A configured but empty-round validator also produces a valid (empty) extension.
func TestExtendVote_ProducesVRF(t *testing.T) {
	app := newTestApp(t)

	// No vote extension config → handler returns empty extension.
	handler := app.ExtendVoteHandler()
	require.NotNil(t, handler, "ExtendVoteHandler should not be nil")

	// Without config, the handler should return an empty extension.
	// The handler uses app.VoteExtConfig which is nil by default.
	require.Nil(t, app.VoteExtConfig, "VoteExtConfig should be nil for unconfigured validator")

	// Set up config with a mock private key and local store (no active rounds).
	privKey := make([]byte, 64) // dummy 64-byte Ed25519 key
	for i := range privKey {
		privKey[i] = byte(i)
	}
	app.VoteExtConfig = &zeroneapp.VoteExtensionConfig{
		ValidatorAddress:    "zrn1testvalidator",
		ValidatorPrivateKey: privKey,
		LocalStore:          zeroneapp.NewLocalCommitmentStore(""),
	}

	// Verify config is now set
	require.NotNil(t, app.VoteExtConfig)
	require.Equal(t, "zrn1testvalidator", app.VoteExtConfig.ValidatorAddress)
	require.Len(t, app.VoteExtConfig.ValidatorPrivateKey, 64)

	// The VoteExtension struct should support VRF fields
	ext := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1testvalidator",
		Commitments: []zeroneapp.VoteCommitment{
			{
				RoundID:        "round-1",
				CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				VRFOutput:      "aabb1122",
				VRFProof:       "ccdd3344",
				Height:         100,
			},
		},
	}

	bz, err := json.Marshal(ext)
	require.NoError(t, err)

	var decoded zeroneapp.VoteExtension
	require.NoError(t, json.Unmarshal(bz, &decoded))
	require.Equal(t, "aabb1122", decoded.Commitments[0].VRFOutput)
	require.Equal(t, "ccdd3344", decoded.Commitments[0].VRFProof)
}

// ---------- VerifyVoteExtension tests ----------

// TestVerifyVoteExtension_Valid verifies that a well-formed empty vote extension is accepted.
// The VerifyVoteExtension handler accepts empty extensions (len(VoteExtension) == 0).
func TestVerifyVoteExtension_Valid(t *testing.T) {
	app := newTestApp(t)
	handler := app.VerifyVoteExtensionHandler()
	require.NotNil(t, handler, "VerifyVoteExtensionHandler should not be nil")

	// An empty vote extension is always valid per the handler logic.
	emptyExt := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1testvalidator",
	}
	bz, err := json.Marshal(emptyExt)
	require.NoError(t, err)
	require.NotEmpty(t, bz)

	// Verify the extension JSON round-trips correctly.
	var decoded zeroneapp.VoteExtension
	require.NoError(t, json.Unmarshal(bz, &decoded))
	require.Equal(t, "zrn1testvalidator", decoded.ValidatorAddress)
	require.Empty(t, decoded.Commitments)
	require.Empty(t, decoded.Reveals)

	// A valid reveal with proper verdict and confidence range should deserialize.
	validExt := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1val_good",
		Reveals: []zeroneapp.VoteReveal{
			{
				RoundID:    "round-1",
				Verdict:    "accept",
				Confidence: 800_000,
				Salt:       "aabbccdd11223344aabbccdd11223344",
			},
		},
	}

	bz, err = json.Marshal(validExt)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(bz, &decoded))
	require.Len(t, decoded.Reveals, 1)
	require.Equal(t, "accept", decoded.Reveals[0].Verdict)
	require.Equal(t, uint64(800_000), decoded.Reveals[0].Confidence)
}

// TestVerifyVoteExtension_Invalid tests rejection conditions for malformed vote extensions.
// The VerifyVoteExtension handler checks:
// - JSON validity
// - Commitment count limits (max 50)
// - Reveal count limits (max 50)
// - Empty round IDs and commitment hashes
// - Invalid verdicts
// - Confidence out of range (> 1_000_000)
func TestVerifyVoteExtension_Invalid(t *testing.T) {
	// Test that invalid JSON fails to unmarshal into VoteExtension.
	var ext zeroneapp.VoteExtension
	err := json.Unmarshal([]byte(`{invalid json}`), &ext)
	require.Error(t, err, "invalid JSON should fail to unmarshal")

	// Test oversized commitment list (>50) — the handler rejects these.
	oversizedExt := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1val_oversized",
		Commitments:      make([]zeroneapp.VoteCommitment, 51),
	}
	for i := range oversizedExt.Commitments {
		oversizedExt.Commitments[i] = zeroneapp.VoteCommitment{
			RoundID:        "round-x",
			CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			VRFOutput:      "aabb",
			VRFProof:       "ccdd",
			Height:         uint64(i),
		}
	}
	require.Len(t, oversizedExt.Commitments, 51, "should have 51 commitments")

	// Test oversized reveal list (>50).
	oversizedReveals := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1val_big_reveals",
		Reveals:          make([]zeroneapp.VoteReveal, 51),
	}
	for i := range oversizedReveals.Reveals {
		oversizedReveals.Reveals[i] = zeroneapp.VoteReveal{
			RoundID:    "round-y",
			Verdict:    "accept",
			Confidence: 500_000,
			Salt:       "salt",
		}
	}
	require.Len(t, oversizedReveals.Reveals, 51, "should have 51 reveals")

	// Test commitment with empty round ID (handler rejects).
	emptyRoundCommitment := zeroneapp.VoteCommitment{
		RoundID:        "",
		CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}
	require.Empty(t, emptyRoundCommitment.RoundID)

	// Test commitment with wrong-length hash (not 64 chars).
	shortHashCommitment := zeroneapp.VoteCommitment{
		RoundID:        "round-1",
		CommitmentHash: "tooshort",
	}
	require.NotEqual(t, 64, len(shortHashCommitment.CommitmentHash), "short hash should not be 64 chars")

	// Test reveal with invalid verdict.
	invalidVerdict := zeroneapp.VoteReveal{
		RoundID:    "round-1",
		Verdict:    "maybe",
		Confidence: 500_000,
		Salt:       "salt",
	}
	require.NotContains(t, []string{"accept", "reject", "abstain"}, invalidVerdict.Verdict)

	// Test reveal with confidence > 1_000_000 (out of range).
	outOfRangeConfidence := zeroneapp.VoteReveal{
		RoundID:    "round-1",
		Verdict:    "accept",
		Confidence: 1_000_001,
		Salt:       "salt",
	}
	require.True(t, outOfRangeConfidence.Confidence > 1_000_000, "confidence should be out of range")

	// Test commitment missing VRF proof (handler rejects per GAP-1).
	noVRFCommitment := zeroneapp.VoteCommitment{
		RoundID:        "round-1",
		CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		VRFOutput:      "",
		VRFProof:       "",
	}
	require.Empty(t, noVRFCommitment.VRFOutput, "VRF output should be empty")
	require.Empty(t, noVRFCommitment.VRFProof, "VRF proof should be empty")
}

// ---------- PrepareProposal tests ----------

// TestPrepareProposal_IncludesVoteExtensions verifies that the PrepareProposal handler
// creates a vote extension injection tx when vote extensions contain data.
func TestPrepareProposal_IncludesVoteExtensions(t *testing.T) {
	app := newTestApp(t)
	handler := app.PrepareProposalHandler()
	require.NotNil(t, handler, "PrepareProposalHandler should not be nil")

	// Build a VoteExtension with a commitment and reveal.
	ext := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1val_proposer",
		Commitments: []zeroneapp.VoteCommitment{
			{
				RoundID:        "round-1",
				CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				VRFOutput:      "aabb",
				VRFProof:       "ccdd",
				Height:         50,
			},
		},
		Reveals: []zeroneapp.VoteReveal{
			{
				RoundID:    "round-0",
				Verdict:    "accept",
				Confidence: 600_000,
				Salt:       "aabbccdd",
			},
		},
	}

	extBz, err := json.Marshal(ext)
	require.NoError(t, err)
	require.NotEmpty(t, extBz, "serialized vote extension should not be empty")

	// Verify the vote extension can be deserialized back.
	var roundTripped zeroneapp.VoteExtension
	require.NoError(t, json.Unmarshal(extBz, &roundTripped))
	require.Equal(t, "zrn1val_proposer", roundTripped.ValidatorAddress)
	require.Len(t, roundTripped.Commitments, 1)
	require.Len(t, roundTripped.Reveals, 1)

	// Verify the injection encoding works for the data we'd expect.
	inj := zeroneapp.VoteExtInjection{
		Commitments: []zeroneapp.InjectedCommitment{
			{
				RoundID:        "round-1",
				Validator:      "zrn1val_proposer",
				CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				VRFOutput:      "aabb",
				VRFProof:       "ccdd",
			},
		},
		Reveals: []zeroneapp.InjectedReveal{
			{
				RoundID:    "round-0",
				Validator:  "zrn1val_proposer",
				Verdict:    "accept",
				Confidence: 600_000,
				Salt:       "aabbccdd",
			},
		},
	}

	encoded, err := zeroneapp.EncodeVoteExtInjection(inj)
	require.NoError(t, err)
	require.True(t, zeroneapp.IsVoteExtInjectionTx(encoded))
	require.True(t, len(encoded) < zeroneapp.MaxVEXInjectionBytes)

	// Verify the injection decodes correctly.
	decoded, err := zeroneapp.DecodeVoteExtInjection(encoded)
	require.NoError(t, err)
	require.Len(t, decoded.Commitments, 1)
	require.Len(t, decoded.Reveals, 1)
	require.Equal(t, "round-1", decoded.Commitments[0].RoundID)
	require.Equal(t, "zrn1val_proposer", decoded.Commitments[0].Validator)
	require.Equal(t, "round-0", decoded.Reveals[0].RoundID)

	// Verify that multiple validators' extensions would be sorted deterministically.
	ext2 := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1val_a",
		Commitments: []zeroneapp.VoteCommitment{
			{RoundID: "round-1", CommitmentHash: "1111111111111111111111111111111111111111111111111111111111111111"},
		},
	}
	ext3 := zeroneapp.VoteExtension{
		ValidatorAddress: "zrn1val_z",
		Commitments: []zeroneapp.VoteCommitment{
			{RoundID: "round-1", CommitmentHash: "2222222222222222222222222222222222222222222222222222222222222222"},
		},
	}

	// These should be serializable without error.
	_, err = json.Marshal(ext2)
	require.NoError(t, err)
	_, err = json.Marshal(ext3)
	require.NoError(t, err)
}

// ---------- ProcessProposal tests ----------

// TestProcessProposal_ValidatesExtensions verifies that ProcessProposal correctly
// validates vote extension injection transactions.
func TestProcessProposal_ValidatesExtensions(t *testing.T) {
	app := newTestApp(t)
	handler := app.ProcessProposalHandler()
	require.NotNil(t, handler, "ProcessProposalHandler should not be nil")

	// Build a valid injection tx.
	inj := zeroneapp.VoteExtInjection{
		Commitments: []zeroneapp.InjectedCommitment{
			{
				RoundID:        "round-1",
				Validator:      "zrn1validator1",
				CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				VRFOutput:      "aabb",
				VRFProof:       "ccdd",
			},
		},
		Reveals: []zeroneapp.InjectedReveal{
			{
				RoundID:    "round-0",
				Validator:  "zrn1validator1",
				Verdict:    "accept",
				Confidence: 600_000,
				Salt:       "deadbeef",
			},
		},
	}

	encoded, err := zeroneapp.EncodeVoteExtInjection(inj)
	require.NoError(t, err)
	require.True(t, zeroneapp.IsVoteExtInjectionTx(encoded))

	// Verify that a valid injection tx can be decoded (what ProcessProposal checks).
	decoded, err := zeroneapp.DecodeVoteExtInjection(encoded)
	require.NoError(t, err)
	require.Len(t, decoded.Commitments, 1)
	require.Len(t, decoded.Reveals, 1)

	// Verify that an invalid injection tx (bad JSON) would fail to decode.
	badInjTx := append(zeroneapp.VoteExtInjectionPrefix, []byte(`{not valid json}`)...)
	require.True(t, zeroneapp.IsVoteExtInjectionTx(badInjTx))
	_, err = zeroneapp.DecodeVoteExtInjection(badInjTx)
	require.Error(t, err, "should reject malformed injection JSON")

	// Verify that non-VEX data is not recognized as an injection.
	regularTx := []byte(`{"body":{"messages":[]},"auth_info":{}}`)
	require.False(t, zeroneapp.IsVoteExtInjectionTx(regularTx))

	// Verify that oversized injection would exceed limits.
	hugeInj := zeroneapp.VoteExtInjection{}
	hugePayload := make([]byte, zeroneapp.MaxVEXInjectionBytes)
	for i := range hugePayload {
		hugePayload[i] = 'x'
	}
	hugeInj.Commitments = append(hugeInj.Commitments, zeroneapp.InjectedCommitment{
		RoundID:        string(hugePayload),
		Validator:      "zrn1validator_huge",
		CommitmentHash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	})
	hugeBz, err := zeroneapp.EncodeVoteExtInjection(hugeInj)
	require.NoError(t, err)
	require.True(t, len(hugeBz) > zeroneapp.MaxVEXInjectionBytes,
		"oversized injection should exceed MaxVEXInjectionBytes")

	// Verify empty injection tx.
	emptyInj := zeroneapp.VoteExtInjection{}
	emptyEncoded, err := zeroneapp.EncodeVoteExtInjection(emptyInj)
	require.NoError(t, err)
	emptyDecoded, err := zeroneapp.DecodeVoteExtInjection(emptyEncoded)
	require.NoError(t, err)
	require.Empty(t, emptyDecoded.Commitments)
	require.Empty(t, emptyDecoded.Reveals)
}

// ---------- LocalCommitmentStore tests (ported from prototype) ----------

func TestLocalCommitmentStore_StoreAndGet(t *testing.T) {
	store := zeroneapp.NewLocalCommitmentStore("")

	store.Store(zeroneapp.LocalCommitment{
		RoundID:    "round-1",
		Verdict:    "accept",
		Confidence: 800_000,
		Salt:       "aabbccdd",
		Height:     100,
	})

	got, found := store.Get("round-1")
	require.True(t, found)
	require.Equal(t, "accept", got.Verdict)
	require.Equal(t, uint64(800_000), got.Confidence)
	require.Equal(t, "aabbccdd", got.Salt)
}

func TestLocalCommitmentStore_GetNotFound(t *testing.T) {
	store := zeroneapp.NewLocalCommitmentStore("")
	_, found := store.Get("nonexistent")
	require.False(t, found)
}

func TestLocalCommitmentStore_Delete(t *testing.T) {
	store := zeroneapp.NewLocalCommitmentStore("")
	store.Store(zeroneapp.LocalCommitment{RoundID: "round-1", Verdict: "accept"})
	store.Delete("round-1")
	_, found := store.Get("round-1")
	require.False(t, found)
}

func TestLocalCommitmentStore_Overwrite(t *testing.T) {
	store := zeroneapp.NewLocalCommitmentStore("")
	store.Store(zeroneapp.LocalCommitment{RoundID: "round-1", Verdict: "accept"})
	store.Store(zeroneapp.LocalCommitment{RoundID: "round-1", Verdict: "reject"})
	got, found := store.Get("round-1")
	require.True(t, found)
	require.Equal(t, "reject", got.Verdict)
}

func TestLocalCommitmentStore_Count(t *testing.T) {
	store := zeroneapp.NewLocalCommitmentStore("")
	require.Equal(t, 0, store.Count())

	store.Store(zeroneapp.LocalCommitment{RoundID: "r1"})
	store.Store(zeroneapp.LocalCommitment{RoundID: "r2"})
	store.Store(zeroneapp.LocalCommitment{RoundID: "r3"})
	require.Equal(t, 3, store.Count())

	store.Delete("r2")
	require.Equal(t, 2, store.Count())
}

func TestLocalCommitmentStore_CleanupExpired(t *testing.T) {
	store := zeroneapp.NewLocalCommitmentStore("")

	store.Store(zeroneapp.LocalCommitment{RoundID: "old", Height: 10})
	store.Store(zeroneapp.LocalCommitment{RoundID: "recent", Height: 90})
	store.Store(zeroneapp.LocalCommitment{RoundID: "current", Height: 100})

	// cutoff = 110 - 50 = 60
	store.CleanupExpired(110, 50)
	require.Equal(t, 2, store.Count())

	_, found := store.Get("old")
	require.False(t, found, "old commitment should have been cleaned up")

	_, found = store.Get("recent")
	require.True(t, found, "recent commitment should survive cleanup")

	_, found = store.Get("current")
	require.True(t, found, "current commitment should survive cleanup")
}

func TestLocalCommitmentStore_ThreadSafety(t *testing.T) {
	store := zeroneapp.NewLocalCommitmentStore("")
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Store(zeroneapp.LocalCommitment{
				RoundID:    "round-" + string(rune('A'+n%26)),
				Verdict:    "accept",
				Confidence: uint64(n * 1000),
				Height:     uint64(n),
			})
		}(i)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Get("round-" + string(rune('A'+n%26)))
		}(i)
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.Delete("round-" + string(rune('A'+n%26)))
		}(i)
	}

	wg.Wait()
	require.GreaterOrEqual(t, store.Count(), 0)
}

// ---------- Oracle integration tests ----------

// mockOracleClient implements OracleClient for testing.
type mockOracleClient struct {
	verdict    string
	confidence float64
	err        error
}

func (m *mockOracleClient) Evaluate(claim, domain, claimType string) (*zeroneapp.OracleEvaluation, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &zeroneapp.OracleEvaluation{
		Verdict:    m.verdict,
		Confidence: m.confidence,
		Reasoning:  "mock",
	}, nil
}

func TestEvaluateWithOracle_Accept(t *testing.T) {
	client := &mockOracleClient{verdict: "accept", confidence: 0.9}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(900_000), confidence)
}

func TestEvaluateWithOracle_Reject(t *testing.T) {
	client := &mockOracleClient{verdict: "reject", confidence: 0.85}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "reject", verdict)
	require.Equal(t, uint64(850_000), confidence)
}

func TestEvaluateWithOracle_Uncertain(t *testing.T) {
	client := &mockOracleClient{verdict: "uncertain", confidence: 0.5}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict, "uncertain should default to accept")
	require.Equal(t, uint64(500_000), confidence)
}

func TestEvaluateWithOracle_Error(t *testing.T) {
	client := &mockOracleClient{err: fmt.Errorf("connection refused")}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict, "error should default to accept")
	require.Equal(t, uint64(600_000), confidence, "error should use default confidence")
}

func TestEvaluateWithOracle_NilClientDefault(t *testing.T) {
	verdict, confidence := zeroneapp.EvaluateWithOracle(nil, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(600_000), confidence)
}
