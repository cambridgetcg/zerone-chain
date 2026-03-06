package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Authenticator tests
// ---------------------------------------------------------------------------

func TestDeriveSessionKey_MatchingKeys(t *testing.T) {
	// Both sides derive same session key via ECDH
	privA, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privB, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	keyA, err := DeriveSessionKey(privA, &privB.PublicKey)
	if err != nil {
		t.Fatalf("DeriveSessionKey A->B: %v", err)
	}
	keyB, err := DeriveSessionKey(privB, &privA.PublicKey)
	if err != nil {
		t.Fatalf("DeriveSessionKey B->A: %v", err)
	}

	if !bytes.Equal(keyA, keyB) {
		t.Fatalf("session keys do not match:\n  A: %x\n  B: %x", keyA, keyB)
	}
	if len(keyA) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(keyA))
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("shard data: training samples for TDU-001")

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted does not match plaintext:\n  got:  %q\n  want: %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	plaintext := []byte("secret shard data")
	ciphertext, err := Encrypt(key1, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decrypt(key2, ciphertext)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}

func TestGenerateNonce(t *testing.T) {
	n1, err := GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}
	n2, err := GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}

	if len(n1) != 32 {
		t.Fatalf("expected 32-byte nonce, got %d", len(n1))
	}
	if bytes.Equal(n1, n2) {
		t.Fatal("two nonces should not be equal")
	}
}

func TestSignAndVerify(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("request payload")
	sig, err := Sign(priv, data)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if !Verify(&priv.PublicKey, data, sig) {
		t.Fatal("signature verification failed")
	}

	// tampered data should fail
	tampered := append([]byte{}, data...)
	tampered[0] ^= 0xff
	if Verify(&priv.PublicKey, tampered, sig) {
		t.Fatal("signature verified on tampered data")
	}
}

func TestAttestationExchange(t *testing.T) {
	enclaveKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	validatorKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	// Mock attestation doc
	attestation := []byte("mock-attestation-document-v1")

	auth := &Authenticator{
		EnclaveID:   "enclave-001",
		EnclaveKey:  enclaveKey,
		Attestation: attestation,
	}

	// Step 1: TEE presents attestation
	hello := auth.Hello()
	if hello.EnclaveID != "enclave-001" {
		t.Fatalf("unexpected enclave ID: %s", hello.EnclaveID)
	}
	if !bytes.Equal(hello.Attestation, attestation) {
		t.Fatal("attestation mismatch")
	}

	// Step 2: Validator sends challenge
	challenge, err := GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}

	// Step 3: TEE signs challenge
	signedChallenge, err := auth.RespondToChallenge(challenge, &validatorKey.PublicKey)
	if err != nil {
		t.Fatalf("RespondToChallenge: %v", err)
	}

	// Validator verifies
	if !Verify(&enclaveKey.PublicKey, challenge, signedChallenge.Signature) {
		t.Fatal("challenge signature verification failed")
	}

	// Both sides should derive the same session key
	validatorSessionKey, err := DeriveSessionKey(validatorKey, &enclaveKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(signedChallenge.SessionKey, validatorSessionKey) {
		t.Fatal("session keys do not match after exchange")
	}
}

// ---------------------------------------------------------------------------
// Verifier tests
// ---------------------------------------------------------------------------

func TestVerifyShardResponse_Valid(t *testing.T) {
	content := []byte("training data content")
	hash := sha256.Sum256(content)

	resp := &ShardResponse{
		ValidatorAddr:  "val1",
		SnapshotHeight: 1000,
		TDUs: []TDUData{
			{ID: "tdu-001", Content: content, Hash: hash[:]},
		},
		DataHash: hash[:],
	}

	expectedHashes := map[string][]byte{
		"tdu-001": hash[:],
	}

	err := VerifyShardIntegrity(resp, expectedHashes)
	if err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
}

func TestVerifyShardResponse_HashMismatch(t *testing.T) {
	content := []byte("training data content")
	hash := sha256.Sum256(content)
	badHash := sha256.Sum256([]byte("different content"))

	resp := &ShardResponse{
		ValidatorAddr:  "val1",
		SnapshotHeight: 1000,
		TDUs: []TDUData{
			{ID: "tdu-001", Content: content, Hash: badHash[:]},
		},
		DataHash: badHash[:],
	}

	expectedHashes := map[string][]byte{
		"tdu-001": hash[:],
	}

	err := VerifyShardIntegrity(resp, expectedHashes)
	if err == nil {
		t.Fatal("expected hash mismatch error")
	}
}

func TestVerifyShardResponse_ContentHashMismatch(t *testing.T) {
	content := []byte("actual content")
	claimedHash := sha256.Sum256([]byte("claimed content"))

	resp := &ShardResponse{
		ValidatorAddr:  "val1",
		SnapshotHeight: 1000,
		TDUs: []TDUData{
			{ID: "tdu-001", Content: content, Hash: claimedHash[:]},
		},
		DataHash: claimedHash[:],
	}

	// Even if expectedHashes matches the claimed hash, content should fail
	expectedHashes := map[string][]byte{
		"tdu-001": claimedHash[:],
	}

	err := VerifyShardIntegrity(resp, expectedHashes)
	if err == nil {
		t.Fatal("expected content hash mismatch error")
	}
}

func TestVerifyReplication_AllMatch(t *testing.T) {
	content := []byte("shared content")
	hash := sha256.Sum256(content)

	responses := []*ShardResponse{
		{
			ValidatorAddr: "val1",
			TDUs:          []TDUData{{ID: "tdu-001", Content: content, Hash: hash[:]}},
		},
		{
			ValidatorAddr: "val2",
			TDUs:          []TDUData{{ID: "tdu-001", Content: content, Hash: hash[:]}},
		},
		{
			ValidatorAddr: "val3",
			TDUs:          []TDUData{{ID: "tdu-001", Content: content, Hash: hash[:]}},
		},
	}

	verified, flagged := VerifyReplication(responses)
	if len(flagged) != 0 {
		t.Fatalf("expected no flagged TDUs, got %v", flagged)
	}
	if len(verified) != 1 || verified[0] != "tdu-001" {
		t.Fatalf("expected [tdu-001] verified, got %v", verified)
	}
}

func TestVerifyReplication_Mismatch(t *testing.T) {
	content1 := []byte("content v1")
	content2 := []byte("content v2")
	hash1 := sha256.Sum256(content1)
	hash2 := sha256.Sum256(content2)

	responses := []*ShardResponse{
		{
			ValidatorAddr: "val1",
			TDUs:          []TDUData{{ID: "tdu-001", Content: content1, Hash: hash1[:]}},
		},
		{
			ValidatorAddr: "val2",
			TDUs:          []TDUData{{ID: "tdu-001", Content: content2, Hash: hash2[:]}},
		},
	}

	_, flagged := VerifyReplication(responses)
	if len(flagged) != 1 || flagged[0] != "tdu-001" {
		t.Fatalf("expected [tdu-001] flagged, got %v", flagged)
	}
}

// ---------------------------------------------------------------------------
// Assembler tests
// ---------------------------------------------------------------------------

func TestAssemble_Success(t *testing.T) {
	c1 := []byte("tdu-001 data")
	c2 := []byte("tdu-002 data")
	h1 := sha256.Sum256(c1)
	h2 := sha256.Sum256(c2)

	responses := []*ShardResponse{
		{
			ValidatorAddr: "val1",
			TDUs: []TDUData{
				{ID: "tdu-001", Content: c1, Hash: h1[:]},
			},
		},
		{
			ValidatorAddr: "val2",
			TDUs: []TDUData{
				{ID: "tdu-001", Content: c1, Hash: h1[:]},
				{ID: "tdu-002", Content: c2, Hash: h2[:]},
			},
		},
		{
			ValidatorAddr: "val3",
			TDUs: []TDUData{
				{ID: "tdu-002", Content: c2, Hash: h2[:]},
			},
		},
	}

	expectedHashes := map[string][]byte{
		"tdu-001": h1[:],
		"tdu-002": h2[:],
	}

	result, err := Assemble(responses, expectedHashes, 2)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if len(result.Verified) != 2 {
		t.Fatalf("expected 2 verified TDUs, got %d", len(result.Verified))
	}
	if len(result.Flagged) != 0 {
		t.Fatalf("expected 0 flagged, got %d", len(result.Flagged))
	}
	if result.TotalTDUs != 2 {
		t.Fatalf("expected TotalTDUs=2, got %d", result.TotalTDUs)
	}
}

func TestAssemble_InsufficientShards(t *testing.T) {
	// Only 1 out of 10 TDUs collected → <80% → abort
	c1 := []byte("tdu-001 data")
	h1 := sha256.Sum256(c1)

	responses := []*ShardResponse{
		{
			ValidatorAddr: "val1",
			TDUs:          []TDUData{{ID: "tdu-001", Content: c1, Hash: h1[:]}},
		},
	}

	// 10 expected TDUs but only 1 collected
	expectedHashes := make(map[string][]byte)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("tdu-%03d", i)
		h := sha256.Sum256([]byte(id))
		expectedHashes[id] = h[:]
	}
	// Override tdu-001 with real hash
	expectedHashes["tdu-001"] = h1[:]

	_, err := Assemble(responses, expectedHashes, 1)
	if err == nil {
		t.Fatal("expected insufficient shards error")
	}
	if err != ErrInsufficientShards {
		t.Fatalf("expected ErrInsufficientShards, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Collector integration tests (with mock transport)
// ---------------------------------------------------------------------------

func TestCollector_SuccessfulCollection(t *testing.T) {
	c1 := []byte("tdu-001 data")
	c2 := []byte("tdu-002 data")
	h1 := sha256.Sum256(c1)
	h2 := sha256.Sum256(c2)

	mockTransport := &MockTransport{
		Responses: map[string]*ShardResponse{
			"val1": {
				ValidatorAddr:  "val1",
				SnapshotHeight: 1000,
				TDUs:           []TDUData{{ID: "tdu-001", Content: c1, Hash: h1[:]}},
			},
			"val2": {
				ValidatorAddr:  "val2",
				SnapshotHeight: 1000,
				TDUs: []TDUData{
					{ID: "tdu-001", Content: c1, Hash: h1[:]},
					{ID: "tdu-002", Content: c2, Hash: h2[:]},
				},
			},
			"val3": {
				ValidatorAddr:  "val3",
				SnapshotHeight: 1000,
				TDUs:           []TDUData{{ID: "tdu-002", Content: c2, Hash: h2[:]}},
			},
		},
	}

	enclaveKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	collector := &Collector{
		EnclaveID:   "enclave-001",
		EnclaveKey:  enclaveKey,
		Attestation: []byte("mock-attestation"),
		Transport:   mockTransport,
		Timeout:     5 * time.Minute,
		MaxRetries:  3,
	}

	assignments := map[string][]string{
		"val1": {"tdu-001"},
		"val2": {"tdu-001", "tdu-002"},
		"val3": {"tdu-002"},
	}

	expectedHashes := map[string][]byte{
		"tdu-001": h1[:],
		"tdu-002": h2[:],
	}

	result, err := collector.Collect(context.Background(), 1000, assignments, expectedHashes, 2)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if result.TotalTDUs != 2 {
		t.Fatalf("expected 2 TDUs, got %d", result.TotalTDUs)
	}
	if len(result.Flagged) != 0 {
		t.Fatalf("expected 0 flagged, got %d", len(result.Flagged))
	}
}

func TestCollector_MissingValidator(t *testing.T) {
	c1 := []byte("tdu-001 data")
	h1 := sha256.Sum256(c1)

	// val2 is offline, but replication is enough
	mockTransport := &MockTransport{
		Responses: map[string]*ShardResponse{
			"val1": {
				ValidatorAddr:  "val1",
				SnapshotHeight: 1000,
				TDUs:           []TDUData{{ID: "tdu-001", Content: c1, Hash: h1[:]}},
			},
			// val2 missing
			"val3": {
				ValidatorAddr:  "val3",
				SnapshotHeight: 1000,
				TDUs:           []TDUData{{ID: "tdu-001", Content: c1, Hash: h1[:]}},
			},
		},
	}

	enclaveKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	collector := &Collector{
		EnclaveID:   "enclave-001",
		EnclaveKey:  enclaveKey,
		Attestation: []byte("mock-attestation"),
		Transport:   mockTransport,
		Timeout:     1 * time.Second,
		MaxRetries:  1, // fast fail for test
	}

	assignments := map[string][]string{
		"val1": {"tdu-001"},
		"val2": {"tdu-001"},
		"val3": {"tdu-001"},
	}

	expectedHashes := map[string][]byte{
		"tdu-001": h1[:],
	}

	result, err := collector.Collect(context.Background(), 1000, assignments, expectedHashes, 1)
	if err != nil {
		t.Fatalf("Collect should succeed despite missing validator: %v", err)
	}
	if result.TotalTDUs != 1 {
		t.Fatalf("expected 1 TDU, got %d", result.TotalTDUs)
	}
	if len(result.FailedValidators) != 1 || result.FailedValidators[0] != "val2" {
		t.Fatalf("expected val2 in failed validators, got %v", result.FailedValidators)
	}
}

func TestCollector_InsufficientShards_Aborts(t *testing.T) {
	// Only 1 out of 5 TDUs available → <80% → abort
	c1 := []byte("tdu-001 data")
	h1 := sha256.Sum256(c1)

	mockTransport := &MockTransport{
		Responses: map[string]*ShardResponse{
			"val1": {
				ValidatorAddr:  "val1",
				SnapshotHeight: 1000,
				TDUs:           []TDUData{{ID: "tdu-001", Content: c1, Hash: h1[:]}},
			},
		},
	}

	enclaveKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	collector := &Collector{
		EnclaveID:   "enclave-001",
		EnclaveKey:  enclaveKey,
		Attestation: []byte("mock-attestation"),
		Transport:   mockTransport,
		Timeout:     1 * time.Second,
		MaxRetries:  1,
	}

	assignments := map[string][]string{
		"val1": {"tdu-001"},
	}

	expectedHashes := make(map[string][]byte)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("tdu-%03d", i)
		h := sha256.Sum256([]byte(id))
		expectedHashes[id] = h[:]
	}
	expectedHashes["tdu-001"] = h1[:]

	_, err := collector.Collect(context.Background(), 1000, assignments, expectedHashes, 1)
	if err != ErrInsufficientShards {
		t.Fatalf("expected ErrInsufficientShards, got: %v", err)
	}
}
