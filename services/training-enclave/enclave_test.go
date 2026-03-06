package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"
	"time"
)

// helper: generate a test ECDSA key pair.
func testKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return key
}

// helper: create test TDU records with varying fitness.
func testTDURecords(domain string, count int) []TDURecord {
	records := make([]TDURecord, count)
	for i := range records {
		score := 0.8 // default: Core (trainable)
		d := domain
		switch {
		case i%10 == 9:
			score = 0.05 // pruned
		case i%10 == 8:
			score = 0.2 // dormant
		case i%5 == 0 && i > 0:
			d = "general"
			score = 0.6 // active, general domain
		}
		records[i] = TDURecord{
			ID:           "tdu-" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Content:      []byte("content-" + string(rune('A'+i%26))),
			Domain:       d,
			Hash:         nil,
			FitnessScore: score,
		}
	}
	return records
}

// ─── Lifecycle state machine tests ─────────────────────────────────────────

func TestLifecycleTransitions(t *testing.T) {
	rt := NewRuntime(&TrainingConfig{
		EnclaveID:    "test-enclave",
		TargetDomain: "science",
	})

	if rt.Phase() != PhaseIdle {
		t.Fatalf("expected idle, got %s", rt.Phase())
	}

	// Valid transition: idle → collect.
	rt.mu.Lock()
	err := rt.transition(PhaseCollect)
	rt.mu.Unlock()
	if err != nil {
		t.Fatalf("idle→collect failed: %v", err)
	}
	if rt.Phase() != PhaseCollect {
		t.Fatalf("expected collect, got %s", rt.Phase())
	}

	// Valid transition: collect → prepare.
	rt.mu.Lock()
	err = rt.transition(PhasePrepare)
	rt.mu.Unlock()
	if err != nil {
		t.Fatalf("collect→prepare failed: %v", err)
	}

	// Valid transition: prepare → train.
	rt.mu.Lock()
	err = rt.transition(PhaseTrain)
	rt.mu.Unlock()
	if err != nil {
		t.Fatalf("prepare→train failed: %v", err)
	}

	// Valid transition: train → output.
	rt.mu.Lock()
	err = rt.transition(PhaseOutput)
	rt.mu.Unlock()
	if err != nil {
		t.Fatalf("train→output failed: %v", err)
	}

	// Valid transition: output → destroy.
	rt.mu.Lock()
	err = rt.transition(PhaseDestroy)
	rt.mu.Unlock()
	if err != nil {
		t.Fatalf("output→destroy failed: %v", err)
	}

	// Valid transition: destroy → done.
	rt.mu.Lock()
	err = rt.transition(PhaseDone)
	rt.mu.Unlock()
	if err != nil {
		t.Fatalf("destroy→done failed: %v", err)
	}
}

func TestInvalidTransition(t *testing.T) {
	rt := NewRuntime(&TrainingConfig{
		EnclaveID:    "test-enclave",
		TargetDomain: "science",
	})

	// Invalid: idle → train (must go through collect first).
	rt.mu.Lock()
	err := rt.transition(PhaseTrain)
	rt.mu.Unlock()
	if err == nil {
		t.Fatal("expected error for idle→train, got nil")
	}
}

func TestTransitionToFailed(t *testing.T) {
	rt := NewRuntime(&TrainingConfig{
		EnclaveID:    "test-enclave",
		TargetDomain: "science",
	})

	rt.mu.Lock()
	_ = rt.transition(PhaseCollect)
	err := rt.transition(PhaseFailed)
	rt.mu.Unlock()
	if err != nil {
		t.Fatalf("collect→failed should be valid: %v", err)
	}
	if rt.Phase() != PhaseFailed {
		t.Fatalf("expected failed, got %s", rt.Phase())
	}
}

// ─── Fitness filter tests ──────────────────────────────────────────────────

func TestFitnessFilterExcludesDormantAndPruned(t *testing.T) {
	records := []TDURecord{
		{ID: "core", FitnessScore: 0.8, Domain: "science", Content: []byte("core content")},
		{ID: "active", FitnessScore: 0.5, Domain: "science", Content: []byte("active content")},
		{ID: "dormant", FitnessScore: 0.2, Domain: "science", Content: []byte("dormant content")},
		{ID: "pruned", FitnessScore: 0.05, Domain: "science", Content: []byte("pruned content")},
	}

	prep := &Preparator{
		TargetDomain:   "science",
		DomainMixRatio: 0.7,
	}

	dataset, err := prep.Prepare(records)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	// Should have exactly 2 (core + active), not dormant/pruned.
	if dataset.FilteredCount != 2 {
		t.Errorf("expected 2 filtered, got %d", dataset.FilteredCount)
	}

	// Check that dormant/pruned IDs are not in the dataset.
	for _, s := range dataset.Samples {
		if s.ID == "dormant" || s.ID == "pruned" {
			t.Errorf("sample %s should have been filtered", s.ID)
		}
	}
}

func TestFitnessFilterAllFiltered(t *testing.T) {
	records := []TDURecord{
		{ID: "dormant1", FitnessScore: 0.2, Domain: "science", Content: []byte("d")},
		{ID: "pruned1", FitnessScore: 0.05, Domain: "science", Content: []byte("p")},
	}

	prep := &Preparator{
		TargetDomain:   "science",
		DomainMixRatio: 0.7,
	}

	_, err := prep.Prepare(records)
	if err == nil {
		t.Fatal("expected error when all TDUs filtered, got nil")
	}
}

// ─── Domain mix ratio tests ────────────────────────────────────────────────

func TestDomainMixRatio(t *testing.T) {
	records := make([]TDURecord, 0, 20)
	// 14 domain records.
	for i := 0; i < 14; i++ {
		records = append(records, TDURecord{
			ID:           fmt.Sprintf("domain-%d", i),
			FitnessScore: 0.8,
			Domain:       "science",
			Content:      []byte(fmt.Sprintf("domain-content-%d", i)),
		})
	}
	// 6 general records.
	for i := 0; i < 6; i++ {
		records = append(records, TDURecord{
			ID:           fmt.Sprintf("general-%d", i),
			FitnessScore: 0.8,
			Domain:       "general",
			Content:      []byte(fmt.Sprintf("general-content-%d", i)),
		})
	}

	prep := &Preparator{
		TargetDomain:   "science",
		DomainMixRatio: 0.7,
	}

	dataset, err := prep.Prepare(records)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	// 70% of 20 = 14 domain, 30% of 20 = 6 general.
	if dataset.DomainSampleCount != 14 {
		t.Errorf("expected 14 domain samples, got %d", dataset.DomainSampleCount)
	}
	if dataset.GeneralSampleCount != 6 {
		t.Errorf("expected 6 general samples, got %d", dataset.GeneralSampleCount)
	}
}

func TestDomainMixOnlyDomain(t *testing.T) {
	records := []TDURecord{
		{ID: "d1", FitnessScore: 0.8, Domain: "science", Content: []byte("c1")},
		{ID: "d2", FitnessScore: 0.8, Domain: "science", Content: []byte("c2")},
	}

	prep := &Preparator{TargetDomain: "science", DomainMixRatio: 0.7}
	dataset, err := prep.Prepare(records)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	// All domain, no general — should use all available.
	if len(dataset.Samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(dataset.Samples))
	}
}

// ─── Output signing tests ──────────────────────────────────────────────────

func TestOutputsSigned(t *testing.T) {
	key := testKey(t)

	result := &TrainingResult{
		ModelWeights:    []byte("fake-model-weights"),
		ModelHash:       []byte("fake-model-hash-32-bytes-padding!"),
		TDULosses:       map[string]float64{"tdu1": 0.5, "tdu2": 0.3},
		FinalLoss:       0.4,
		BenchmarkScore:  0.75,
		BenchmarkDetail: map[string]float64{"overall": 0.75},
		ConfigHash:      []byte("config-hash-32-bytes-of-padding!"),
	}

	outputter := &Outputter{
		EnclaveID:  "test-enclave",
		EnclaveKey: key,
	}

	signed, err := outputter.SignOutputs(result)
	if err != nil {
		t.Fatalf("SignOutputs failed: %v", err)
	}

	// Verify all signatures are present and correct length (64 bytes r||s).
	if len(signed.ModelWeightsSignature) != 64 {
		t.Errorf("model weights signature: expected 64 bytes, got %d", len(signed.ModelWeightsSignature))
	}
	if len(signed.MetricsSignature) != 64 {
		t.Errorf("metrics signature: expected 64 bytes, got %d", len(signed.MetricsSignature))
	}
	if len(signed.BenchmarkSignature) != 64 {
		t.Errorf("benchmark signature: expected 64 bytes, got %d", len(signed.BenchmarkSignature))
	}

	// Verify model weights signature.
	if !VerifyOutputSignature(&key.PublicKey, result.ModelWeights, signed.ModelWeightsSignature) {
		t.Error("model weights signature verification failed")
	}
}

func TestOutputSigningNilKey(t *testing.T) {
	outputter := &Outputter{EnclaveID: "test", EnclaveKey: nil}
	result := &TrainingResult{ModelWeights: []byte("data")}
	_, err := outputter.SignOutputs(result)
	if err == nil {
		t.Fatal("expected error with nil key, got nil")
	}
}

// ─── Destruction tests ─────────────────────────────────────────────────────

func TestDestructionZerosDatasetMemory(t *testing.T) {
	dataset := &PreparedDataset{
		Samples: []TrainingSample{
			{ID: "s1", Text: "secret content", Domain: "science"},
			{ID: "s2", Text: "more secrets", Domain: "general"},
		},
		DomainSampleCount:  1,
		GeneralSampleCount: 1,
		TotalTDUs:          2,
		Fingerprint:        []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	destroyer := &Destroyer{}
	err := destroyer.DestroyDataset(dataset)
	if err != nil {
		t.Fatalf("DestroyDataset failed: %v", err)
	}

	// Verify all fields are zeroed.
	if dataset.Samples != nil {
		t.Error("samples should be nil after destruction")
	}
	if dataset.Fingerprint != nil {
		t.Error("fingerprint should be nil after destruction")
	}
	if dataset.DomainSampleCount != 0 || dataset.GeneralSampleCount != 0 {
		t.Error("sample counts should be 0 after destruction")
	}
}

func TestDestructionZerosTrainingState(t *testing.T) {
	result := &TrainingResult{
		ModelWeights:   []byte("model"),
		gradients:      [][]byte{{0x01, 0x02}, {0x03, 0x04}},
		optimizerState: [][]byte{{0x05, 0x06}},
		activations:    [][]byte{{0x07, 0x08}},
	}

	destroyer := &Destroyer{}
	err := destroyer.DestroyTrainingState(result)
	if err != nil {
		t.Fatalf("DestroyTrainingState failed: %v", err)
	}

	if result.gradients != nil {
		t.Error("gradients should be nil after destruction")
	}
	if result.optimizerState != nil {
		t.Error("optimizer state should be nil after destruction")
	}
	if result.activations != nil {
		t.Error("activations should be nil after destruction")
	}
	// Model weights should survive (they were already exported).
	if len(result.ModelWeights) == 0 {
		t.Error("model weights should survive destruction (already exported)")
	}
}

func TestDestructionVerification(t *testing.T) {
	dataset := &PreparedDataset{}
	result := &TrainingResult{}

	destroyer := &Destroyer{}
	_ = destroyer.DestroyDataset(dataset)
	_ = destroyer.DestroyTrainingState(result)

	proof, err := destroyer.VerifyDestruction(dataset, result)
	if err != nil {
		t.Fatalf("VerifyDestruction failed: %v", err)
	}
	if len(proof) == 0 {
		t.Error("destruction proof should not be empty")
	}
}

func TestDestructionVerificationFailsWithResidual(t *testing.T) {
	// Dataset still has samples — verification should fail.
	dataset := &PreparedDataset{
		Samples: []TrainingSample{{ID: "residual"}},
	}
	result := &TrainingResult{}

	destroyer := &Destroyer{}
	_, err := destroyer.VerifyDestruction(dataset, result)
	if err == nil {
		t.Fatal("expected verification failure with residual samples")
	}
}

// ─── Attestation tests ─────────────────────────────────────────────────────

func TestAttestationContainsCorrectData(t *testing.T) {
	key := testKey(t)
	fingerprint := []byte{0x01, 0x02, 0x03}
	modelHash := []byte{0xAA, 0xBB, 0xCC}
	configHash := []byte{0xDD, 0xEE, 0xFF}

	dataset := &PreparedDataset{
		Samples:     make([]TrainingSample, 100),
		Fingerprint: fingerprint,
	}

	result := &TrainingResult{
		ModelHash:      modelHash,
		ConfigHash:     configHash,
		BenchmarkScore: 0.73,
	}

	gen := &AttestationGenerator{
		EnclaveID:  "test-enclave",
		EnclaveKey: key,
	}

	phaseTimings := map[Phase]time.Time{
		PhaseCollect: time.Now().Add(-1 * time.Hour),
	}

	att, err := gen.GenerateWithBaseModel(dataset, result, "llama-3-8b", phaseTimings)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if att.EnclaveID != "test-enclave" {
		t.Errorf("expected enclave-id test-enclave, got %s", att.EnclaveID)
	}
	if att.DatasetSize != 100 {
		t.Errorf("expected dataset size 100, got %d", att.DatasetSize)
	}
	if att.BenchmarkScore != 0.73 {
		t.Errorf("expected benchmark 0.73, got %f", att.BenchmarkScore)
	}
	if att.BaseModel != "llama-3-8b" {
		t.Errorf("expected base model llama-3-8b, got %s", att.BaseModel)
	}

	// Sign and verify.
	err = gen.Sign(att)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	valid, err := VerifyAttestation(att, &key.PublicKey)
	if err != nil {
		t.Fatalf("VerifyAttestation failed: %v", err)
	}
	if !valid {
		t.Error("attestation signature verification failed")
	}
}

// ─── Incomplete collection → reduced dataset test ──────────────────────────

func TestIncompleteCollectionReducedDataset(t *testing.T) {
	// With partial collection (some TDUs missing), training should proceed
	// with the available data, not fail.
	records := []TDURecord{
		{ID: "tdu1", FitnessScore: 0.8, Domain: "science", Content: []byte("c1")},
		{ID: "tdu2", FitnessScore: 0.6, Domain: "science", Content: []byte("c2")},
		// tdu3, tdu4 missing (incomplete collection)
	}

	prep := &Preparator{TargetDomain: "science", DomainMixRatio: 0.7}
	dataset, err := prep.Prepare(records)
	if err != nil {
		t.Fatalf("Prepare with partial collection should succeed: %v", err)
	}

	if len(dataset.Samples) != 2 {
		t.Errorf("expected 2 samples from partial collection, got %d", len(dataset.Samples))
	}
}

// ─── Full lifecycle integration test ───────────────────────────────────────

func TestFullLifecycleIntegration(t *testing.T) {
	key := testKey(t)

	config := &TrainingConfig{
		EnclaveID:      "integration-test-enclave",
		EnclaveKey:     key,
		Attestation:    []byte("test-attestation"),
		BaseModel:      "llama-3-8b",
		TargetDomain:   "science",
		DomainMixRatio: 0.7,
		LoRAConfig:     DefaultLoRAConfig(),
		Assignments: map[string][]string{
			"validator1": {"tdu1", "tdu2", "tdu3"},
			"validator2": {"tdu4", "tdu5"},
		},
	}

	rt := NewRuntime(config)

	if rt.Phase() != PhaseIdle {
		t.Fatalf("expected idle, got %s", rt.Phase())
	}

	// Note: Run() will use simulated collection, so the TDU records
	// won't have content. This tests the lifecycle orchestration.
	// A full integration test with real shard collection requires
	// the chain to be running.
	att, err := rt.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if att == nil {
		t.Fatal("expected attestation, got nil")
	}

	if rt.Phase() != PhaseDone {
		t.Errorf("expected done phase, got %s", rt.Phase())
	}
}

// ─── Phase string tests ───────────────────────────────────────────────────

func TestPhaseString(t *testing.T) {
	tests := []struct {
		phase    Phase
		expected string
	}{
		{PhaseIdle, "idle"},
		{PhaseCollect, "collect"},
		{PhasePrepare, "prepare"},
		{PhaseTrain, "train"},
		{PhaseOutput, "output"},
		{PhaseDestroy, "destroy"},
		{PhaseDone, "done"},
		{PhaseFailed, "failed"},
		{Phase(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.expected {
			t.Errorf("Phase(%d).String() = %s, want %s", tt.phase, got, tt.expected)
		}
	}
}

// ─── TDU trainability tests ──────────────────────────────────────────────

func TestTDUTrainability(t *testing.T) {
	tests := []struct {
		score     float64
		trainable bool
		dormant   bool
		pruned    bool
	}{
		{0.8, true, false, false},   // core
		{0.5, true, false, false},   // active
		{0.3, true, false, false},   // boundary: active
		{0.29, false, true, false},  // dormant
		{0.1, false, true, false},   // boundary: dormant
		{0.09, false, false, true},  // pruned
		{0.0, false, false, true},   // pruned
	}

	for _, tt := range tests {
		tdu := &TDURecord{FitnessScore: tt.score}
		if tdu.IsTrainable() != tt.trainable {
			t.Errorf("score %.2f: IsTrainable() = %v, want %v", tt.score, tdu.IsTrainable(), tt.trainable)
		}
		if tdu.IsDormant() != tt.dormant {
			t.Errorf("score %.2f: IsDormant() = %v, want %v", tt.score, tdu.IsDormant(), tt.dormant)
		}
		if tdu.IsPruned() != tt.pruned {
			t.Errorf("score %.2f: IsPruned() = %v, want %v", tt.score, tdu.IsPruned(), tt.pruned)
		}
	}
}

// ─── Dataset fingerprint determinism ──────────────────────────────────────

func TestDatasetFingerprintDeterministic(t *testing.T) {
	records := []TDURecord{
		{ID: "b", FitnessScore: 0.8, Domain: "science", Content: []byte("content-b")},
		{ID: "a", FitnessScore: 0.8, Domain: "science", Content: []byte("content-a")},
	}

	prep := &Preparator{TargetDomain: "science", DomainMixRatio: 0.7}

	ds1, _ := prep.Prepare(records)

	// Reverse order.
	records[0], records[1] = records[1], records[0]
	ds2, _ := prep.Prepare(records)

	// Fingerprints should be identical regardless of input order.
	if string(ds1.Fingerprint) != string(ds2.Fingerprint) {
		t.Error("fingerprints differ for same dataset in different order")
	}
}

