//go:build tee_mock

package tee_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/zerone-chain/zerone/pkg/tee"
	"github.com/zerone-chain/zerone/pkg/tee/mock"
)

func TestVerifyAttestation_NilAttestation(t *testing.T) {
	provider := mock.NewProvider()
	_, err := tee.VerifyAttestation(provider, nil, nil, tee.DefaultAttestationTTL)
	if err == nil {
		t.Error("VerifyAttestation(nil) should return error")
	}
}

func TestVerifyAttestation_ProviderMismatch(t *testing.T) {
	provider := mock.NewProvider()
	attestation := &tee.Attestation{
		Provider:  tee.ProviderNitro,
		Document:  []byte("doc"),
		Timestamp: time.Now(),
		Measurements: tee.Measurements{
			CodeHash: []byte("code"),
		},
	}

	result, err := tee.VerifyAttestation(provider, attestation, nil, tee.DefaultAttestationTTL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("should reject provider mismatch")
	}
}

func TestVerifyAttestation_ValidWithMatchingMeasurements(t *testing.T) {
	codeHash := sha256.Sum256([]byte("my-enclave"))
	provider := mock.NewProvider(mock.WithMeasurements(tee.Measurements{
		CodeHash: codeHash[:],
	}))

	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	expected := &tee.Measurements{CodeHash: codeHash[:]}
	result, err := tee.VerifyAttestation(provider, attestation, expected, tee.DefaultAttestationTTL)
	if err != nil {
		t.Fatalf("VerifyAttestation() error: %v", err)
	}
	if !result.Valid {
		t.Errorf("should be valid: %s", result.Error)
	}
}

func TestVerifyAttestation_PartialMeasurementMatch(t *testing.T) {
	codeHash := sha256.Sum256([]byte("enclave"))
	configHash := sha256.Sum256([]byte("config"))
	provider := mock.NewProvider(mock.WithMeasurements(tee.Measurements{
		CodeHash:   codeHash[:],
		ConfigHash: configHash[:],
	}))

	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	// Only check CodeHash (partial match).
	expected := &tee.Measurements{CodeHash: codeHash[:]}
	result, err := tee.VerifyAttestation(provider, attestation, expected, tee.DefaultAttestationTTL)
	if err != nil {
		t.Fatalf("VerifyAttestation() error: %v", err)
	}
	if !result.Valid {
		t.Errorf("partial measurement match should pass: %s", result.Error)
	}
}

func TestVerifyAttestation_ExpiredTTL(t *testing.T) {
	past := time.Now().Add(-2 * time.Hour)
	provider := mock.NewProvider(mock.WithFixedTime(past))

	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	// 1h TTL — attestation is 2h old.
	result, err := tee.VerifyAttestation(provider, attestation, nil, 1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("should reject expired attestation")
	}
}

func TestVerifyAttestation_NoExpectedMeasurements(t *testing.T) {
	provider := mock.NewProvider()

	attestation, err := provider.Attest(context.Background())
	if err != nil {
		t.Fatalf("Attest() error: %v", err)
	}

	// nil expected measurements = skip measurement check.
	result, err := tee.VerifyAttestation(provider, attestation, nil, tee.DefaultAttestationTTL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("should be valid without expected measurements: %s", result.Error)
	}
}

func TestAttestation_Hash(t *testing.T) {
	a := &tee.Attestation{
		Document: []byte("test-doc"),
	}
	hash := a.Hash()
	if len(hash) != 32 {
		t.Errorf("Hash() length = %d, want 32", len(hash))
	}
	hexStr := a.HashHex()
	if len(hexStr) != 64 {
		t.Errorf("HashHex() length = %d, want 64", len(hexStr))
	}
}

func TestAttestation_IsExpired(t *testing.T) {
	a := &tee.Attestation{Timestamp: time.Now().Add(-2 * time.Hour)}
	if !a.IsExpired(1 * time.Hour) {
		t.Error("should be expired")
	}
	if a.IsExpired(3 * time.Hour) {
		t.Error("should not be expired")
	}
}

func TestMeasurements_Validate(t *testing.T) {
	m := &tee.Measurements{}
	if err := m.Validate(); err == nil {
		t.Error("empty measurements should fail validation")
	}

	m.CodeHash = []byte("hash")
	if err := m.Validate(); err != nil {
		t.Errorf("valid measurements should pass: %v", err)
	}
}

func TestEnclaveStatus_String(t *testing.T) {
	tests := []struct {
		status tee.EnclaveStatus
		want   string
	}{
		{tee.EnclaveStatusActive, "active"},
		{tee.EnclaveStatusSuspended, "suspended"},
		{tee.EnclaveStatusRevoked, "revoked"},
		{tee.EnclaveStatus(99), "unknown(99)"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("EnclaveStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}
