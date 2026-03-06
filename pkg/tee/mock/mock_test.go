//go:build tee_mock

package mock_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/zerone-chain/zerone/pkg/tee"
	"github.com/zerone-chain/zerone/pkg/tee/mock"
)

func TestMockProvider_AttestAndVerify(t *testing.T) {
	provider := mock.NewProvider()

	// Generate attestation.
	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	// Validate attestation structure.
	if attestation.Provider != tee.ProviderMock {
		t.Errorf("Provider = %s, want %s", attestation.Provider, tee.ProviderMock)
	}
	if len(attestation.Document) == 0 {
		t.Error("Document is empty")
	}
	if len(attestation.Nonce) == 0 {
		t.Error("Nonce is empty")
	}
	if attestation.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if err := attestation.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}

	// Verify the attestation.
	result, err := provider.Verify(attestation)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if !result.Valid {
		t.Errorf("Verify() Valid = false, want true; error: %s", result.Error)
	}
	if len(result.Measurements.CodeHash) == 0 {
		t.Error("Measurements.CodeHash is empty after verification")
	}
}

func TestMockProvider_WrongMeasurements_FailsVerification(t *testing.T) {
	// Create a provider with specific measurements.
	realCode := sha256.Sum256([]byte("real-enclave"))
	provider := mock.NewProvider(mock.WithMeasurements(tee.Measurements{
		CodeHash: realCode[:],
	}))

	// Generate attestation.
	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	// Use cross-provider verify with wrong expected measurements.
	wrongCode := sha256.Sum256([]byte("wrong-enclave"))
	expected := &tee.Measurements{CodeHash: wrongCode[:]}

	result, err := tee.VerifyAttestation(provider, attestation, expected, tee.DefaultAttestationTTL)
	if err != nil {
		t.Fatalf("VerifyAttestation() error: %v", err)
	}
	if result.Valid {
		t.Error("VerifyAttestation() should fail with wrong measurements")
	}
	if result.Error == "" {
		t.Error("Expected error message for measurement mismatch")
	}
}

func TestMockProvider_ExpiredAttestation_Rejected(t *testing.T) {
	// Create a provider with a fixed time in the past.
	pastTime := time.Now().Add(-48 * time.Hour)
	provider := mock.NewProvider(mock.WithFixedTime(pastTime))

	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	// Verify with a 24h TTL — should fail since attestation is 48h old.
	result, err := tee.VerifyAttestation(provider, attestation, nil, 24*time.Hour)
	if err != nil {
		t.Fatalf("VerifyAttestation() error: %v", err)
	}
	if result.Valid {
		t.Error("VerifyAttestation() should reject expired attestation")
	}
}

func TestMockProvider_FailVerify_Option(t *testing.T) {
	provider := mock.NewProvider(mock.WithFailVerify())

	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	result, err := provider.Verify(attestation)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.Valid {
		t.Error("Verify() should return invalid when WithFailVerify is set")
	}
}

func TestMockProvider_SealUnseal(t *testing.T) {
	provider := mock.NewProvider()

	plaintext := []byte("sensitive enclave data")

	sealed, err := provider.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal() error: %v", err)
	}
	if len(sealed) == 0 {
		t.Error("Seal() returned empty data")
	}

	// Sealed should be different from plaintext.
	if string(sealed) == string(plaintext) {
		t.Error("Sealed data should differ from plaintext")
	}

	// Unseal should recover the original.
	recovered, err := provider.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal() error: %v", err)
	}
	if string(recovered) != string(plaintext) {
		t.Errorf("Unseal() = %q, want %q", recovered, plaintext)
	}
}

func TestMockProvider_SealUnseal_DifferentKey_Fails(t *testing.T) {
	key1 := sha256.Sum256([]byte("key-1"))
	key2 := sha256.Sum256([]byte("key-2"))

	provider1 := mock.NewProvider(mock.WithSealingKey(key1[:]))
	provider2 := mock.NewProvider(mock.WithSealingKey(key2[:]))

	sealed, err := provider1.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal() error: %v", err)
	}

	_, err = provider2.Unseal(sealed)
	if err == nil {
		t.Error("Unseal() with different key should fail")
	}
}

func TestMockProvider_GetMeasurements(t *testing.T) {
	codeHash := sha256.Sum256([]byte("my-enclave"))
	provider := mock.NewProvider(mock.WithMeasurements(tee.Measurements{
		CodeHash: codeHash[:],
	}))

	m, err := provider.GetMeasurements()
	if err != nil {
		t.Fatalf("GetMeasurements() error: %v", err)
	}
	if string(m.CodeHash) != string(codeHash[:]) {
		t.Error("CodeHash mismatch")
	}
}

func TestMockProvider_Type(t *testing.T) {
	provider := mock.NewProvider()
	if provider.Type() != tee.ProviderMock {
		t.Errorf("Type() = %s, want %s", provider.Type(), tee.ProviderMock)
	}
}

func TestMockProvider_VerifyWrongProvider(t *testing.T) {
	provider := mock.NewProvider()

	// Create an attestation claiming to be Nitro.
	attestation := &tee.Attestation{
		Provider:  tee.ProviderNitro,
		Document:  []byte("fake"),
		Timestamp: time.Now(),
		Measurements: tee.Measurements{
			CodeHash: []byte("hash"),
		},
	}

	result, err := provider.Verify(attestation)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.Valid {
		t.Error("Verify() should reject attestation from wrong provider")
	}
}
