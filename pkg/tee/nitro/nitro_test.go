package nitro_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/zerone-chain/zerone/pkg/tee"
	"github.com/zerone-chain/zerone/pkg/tee/nitro"
)

func TestNitroProvider_Type(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}
	if p.Type() != tee.ProviderNitro {
		t.Errorf("Type() = %s, want %s", p.Type(), tee.ProviderNitro)
	}
}

func TestNitroProvider_AttestOutsideEnclave(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}

	// Outside a Nitro Enclave, Attest should fail.
	_, err = p.Attest(context.Background())
	if err == nil {
		t.Error("Attest() should fail outside a Nitro Enclave")
	}
}

func TestNitroProvider_GetMeasurementsOutsideEnclave(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}

	_, err = p.GetMeasurements()
	if err == nil {
		t.Error("GetMeasurements() should fail outside a Nitro Enclave")
	}
}

func TestNitroProvider_VerifyValidDocument(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}

	// Create a valid Nitro document structure.
	doc := nitro.NitroDocument{
		PCR0:      []byte("enclave-image-hash-000000000000"),
		PCR1:      []byte("kernel-boot-hash-0000000000000"),
		PCR8:      []byte("signer-cert-hash-0000000000000"),
		Timestamp: time.Now(),
	}
	docBytes, _ := json.Marshal(doc)

	attestation := &tee.Attestation{
		Provider:  tee.ProviderNitro,
		Document:  docBytes,
		Timestamp: time.Now(),
		Measurements: tee.Measurements{
			CodeHash: doc.PCR0,
		},
	}

	result, err := p.Verify(attestation)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if !result.Valid {
		t.Errorf("Verify() should pass for valid document: %s", result.Error)
	}
	if string(result.Measurements.CodeHash) != string(doc.PCR0) {
		t.Error("CodeHash mismatch in result")
	}
}

func TestNitroProvider_VerifyEmptyPCR0(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}

	doc := nitro.NitroDocument{
		PCR0:      nil, // Empty PCR0
		Timestamp: time.Now(),
	}
	docBytes, _ := json.Marshal(doc)

	attestation := &tee.Attestation{
		Provider:  tee.ProviderNitro,
		Document:  docBytes,
		Timestamp: time.Now(),
		Measurements: tee.Measurements{
			CodeHash: []byte("something"),
		},
	}

	result, err := p.Verify(attestation)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.Valid {
		t.Error("Verify() should fail with empty PCR0")
	}
}

func TestNitroProvider_VerifyWrongProvider(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}

	attestation := &tee.Attestation{
		Provider:  tee.ProviderMock, // Wrong provider
		Document:  []byte("{}"),
		Timestamp: time.Now(),
		Measurements: tee.Measurements{
			CodeHash: []byte("hash"),
		},
	}

	result, err := p.Verify(attestation)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.Valid {
		t.Error("Verify() should reject wrong provider")
	}
}

func TestNitroProvider_VerifyInvalidJSON(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}

	attestation := &tee.Attestation{
		Provider:  tee.ProviderNitro,
		Document:  []byte("not-json"),
		Timestamp: time.Now(),
		Measurements: tee.Measurements{
			CodeHash: []byte("hash"),
		},
	}

	result, err := p.Verify(attestation)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.Valid {
		t.Error("Verify() should reject invalid JSON")
	}
}

func TestNitroProvider_SealUnseal(t *testing.T) {
	p, err := nitro.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider() error: %v", err)
	}

	plaintext := []byte("nitro sealed data")
	sealed, err := p.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal() error: %v", err)
	}

	recovered, err := p.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal() error: %v", err)
	}
	if string(recovered) != string(plaintext) {
		t.Errorf("Unseal() = %q, want %q", recovered, plaintext)
	}
}
