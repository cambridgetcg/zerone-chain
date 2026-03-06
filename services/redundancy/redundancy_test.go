package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// ─── Embedder Tests ──────────────────────────────────────────────────────────

func TestEmbedderLoadAndEmbed(t *testing.T) {
	tmpDir := t.TempDir()
	tdus := []map[string]any{
		{"id": "tdu-001", "content": "how to write a function in go that parses json data", "domain": "code", "fitness_score": 0.8},
		{"id": "tdu-002", "content": "explain the concept of goroutines and concurrency in golang", "domain": "code", "fitness_score": 0.6},
		{"id": "tdu-003", "content": "how to write a function in go that parses json data structures", "domain": "code", "fitness_score": 0.5},
	}

	path := filepath.Join(tmpDir, "tdus.jsonl")
	f, _ := os.Create(path)
	for _, tdu := range tdus {
		data, _ := json.Marshal(tdu)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	embedder := NewEmbedder(100)
	records, err := embedder.LoadAndEmbed(path)
	if err != nil {
		t.Fatalf("LoadAndEmbed error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// All should have non-zero simHash
	for _, r := range records {
		if r.simHash == 0 {
			t.Errorf("TDU %s has zero simHash", r.ID)
		}
	}
}

func TestEmbedderSkipsMalformed(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tdus.jsonl")
	content := `{"id":"good","content":"valid content here for testing purposes"}
not json
{"id":"","content":"no id"}
{"id":"no-content"}
{"id":"also-good","content":"another valid entry for testing purposes"}
`
	os.WriteFile(path, []byte(content), 0644)

	embedder := NewEmbedder(100)
	records, err := embedder.LoadAndEmbed(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestEmbedBatchProcessesInBatches(t *testing.T) {
	records := make([]TDURecord, 250)
	for i := range records {
		records[i] = TDURecord{
			ID:      "tdu-batch",
			Content: "this is some content for batch processing test number whatever",
		}
	}

	embedder := NewEmbedder(100) // batch size 100
	embedder.EmbedBatch(records)

	for i, r := range records {
		if r.simHash == 0 {
			t.Errorf("record %d has zero simHash", i)
		}
	}
}

func TestSimilarityIdentical(t *testing.T) {
	a := TDURecord{Content: "how to write a function in go that parses json data"}
	b := TDURecord{Content: "how to write a function in go that parses json data"}
	embedder := NewEmbedder(1)
	embedder.EmbedBatch([]TDURecord{a, b})
	// Re-embed to set simHash (EmbedBatch modifies in-place)
	a.simHash = computeSimHash(a.Content)
	b.simHash = computeSimHash(b.Content)

	sim := Similarity(&a, &b)
	if sim != 1.0 {
		t.Errorf("identical content should have similarity 1.0, got %f", sim)
	}
}

// ─── Clusterer Tests ─────────────────────────────────────────────────────────

func TestTwoNearIdenticalTDUsClustered(t *testing.T) {
	records := []TDURecord{
		{ID: "a", Content: "how to write a function in go that parses json data"},
		{ID: "b", Content: "how to write a function in go that parses json data structures"},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	clusterer := NewClusterer(0.85)
	clusters := clusterer.ClusterTDUs(records)

	// Should be clustered together
	foundTogether := false
	for _, c := range clusters {
		if len(c.Members) >= 2 {
			hasA, hasB := false, false
			for _, m := range c.Members {
				if m.ID == "a" {
					hasA = true
				}
				if m.ID == "b" {
					hasB = true
				}
			}
			if hasA && hasB {
				foundTogether = true
			}
		}
	}
	if !foundTogether {
		t.Error("near-identical TDUs should be clustered together")
	}
}

func TestDissimilarTDUsNotClustered(t *testing.T) {
	records := []TDURecord{
		{ID: "code", Content: "how to write a function in go that parses json data from http requests"},
		{ID: "cooking", Content: "the best recipe for chocolate cake with vanilla frosting and sprinkles"},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	clusterer := NewClusterer(0.85)
	clusters := clusterer.ClusterTDUs(records)

	// Each should be in its own cluster
	for _, c := range clusters {
		if len(c.Members) > 1 {
			t.Errorf("dissimilar TDUs should NOT be in the same cluster, got cluster with %d members", len(c.Members))
		}
	}
}

func TestSingleTDUNoCluster(t *testing.T) {
	records := []TDURecord{
		{ID: "alone", Content: "a unique standalone training data unit about quantum physics"},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	clusterer := NewClusterer(0.85)
	clusters := clusterer.ClusterTDUs(records)

	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if len(clusters[0].Members) != 1 {
		t.Errorf("single TDU cluster should have 1 member, got %d", len(clusters[0].Members))
	}
}

// ─── Ranker Tests ────────────────────────────────────────────────────────────

func TestCanonicalIsHighestFitness(t *testing.T) {
	records := []TDURecord{
		{ID: "low", Content: "how to write a function in go that parses json data", FitnessScore: 0.3},
		{ID: "high", Content: "how to write a function in go that parses json data structures", FitnessScore: 0.9},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	cluster := Cluster{ID: 0, Members: []*TDURecord{&records[0], &records[1]}}
	ranker := NewRanker()
	ranked := ranker.RankClusters([]Cluster{cluster})

	if len(ranked) != 1 {
		t.Fatalf("expected 1 ranked cluster, got %d", len(ranked))
	}
	if ranked[0].Canonical.ID != "high" {
		t.Errorf("canonical should be 'high' (fitness=0.9), got %s", ranked[0].Canonical.ID)
	}
	if len(ranked[0].Redundant) != 1 {
		t.Fatalf("expected 1 redundant, got %d", len(ranked[0].Redundant))
	}
	if ranked[0].Redundant[0].ID != "low" {
		t.Errorf("redundant should be 'low', got %s", ranked[0].Redundant[0].ID)
	}
}

func TestCorrectionBecomesCanonicalRegardlessOfFitness(t *testing.T) {
	records := []TDURecord{
		{ID: "high-fitness", Content: "some content", FitnessScore: 0.95},
		{ID: "correction", Content: "corrected content", FitnessScore: 0.3, IsCorrection: true},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	cluster := Cluster{ID: 0, Members: []*TDURecord{&records[0], &records[1]}}
	ranker := NewRanker()
	ranked := ranker.RankClusters([]Cluster{cluster})

	if ranked[0].Canonical.ID != "correction" {
		t.Errorf("correction TDU should be canonical regardless of fitness, got %s", ranked[0].Canonical.ID)
	}
}

func TestOlderTDUWinsTies(t *testing.T) {
	records := []TDURecord{
		{ID: "newer", Content: "content a", FitnessScore: 0.7, CreatedAt: 200},
		{ID: "older", Content: "content b", FitnessScore: 0.7, CreatedAt: 100},
	}

	cluster := Cluster{ID: 0, Members: []*TDURecord{&records[0], &records[1]}}
	ranker := NewRanker()
	ranked := ranker.RankClusters([]Cluster{cluster})

	if ranked[0].Canonical.ID != "older" {
		t.Errorf("older TDU should win ties, got %s", ranked[0].Canonical.ID)
	}
}

func TestSingleMemberClusterNoRedundancy(t *testing.T) {
	records := []TDURecord{
		{ID: "solo", Content: "unique", FitnessScore: 0.5},
	}

	cluster := Cluster{ID: 0, Members: []*TDURecord{&records[0]}}
	ranker := NewRanker()
	ranked := ranker.RankClusters([]Cluster{cluster})

	if ranked[0].Canonical.ID != "solo" {
		t.Errorf("canonical should be the only member")
	}
	if len(ranked[0].Redundant) != 0 {
		t.Errorf("single member should have no redundant, got %d", len(ranked[0].Redundant))
	}
}

// ─── Signal Generator Tests ─────────────────────────────────────────────────

func TestCanonicalGetsPositiveSignal(t *testing.T) {
	records := []TDURecord{
		{ID: "canonical", Content: "how to write go functions for json parsing", FitnessScore: 0.9},
		{ID: "redundant", Content: "how to write go functions for json parsing and data", FitnessScore: 0.4},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	ranked := []RankedCluster{
		{
			Cluster:   Cluster{Members: []*TDURecord{&records[0], &records[1]}},
			Canonical: &records[0],
			Redundant: []*TDURecord{&records[1]},
		},
	}

	sigGen := NewSignalGenerator(0.85)
	signals := sigGen.GenerateSignals(ranked)

	var canonicalSignal *FitnessSignalEntry
	for i := range signals {
		if signals[i].SampleID == "canonical" {
			canonicalSignal = &signals[i]
		}
	}

	if canonicalSignal == nil {
		t.Fatal("canonical signal not found")
	}
	expected := 0.5 + canonicalBoost // 0.55
	if math.Abs(canonicalSignal.Redundancy-expected) > 0.001 {
		t.Errorf("canonical Redundancy should be %.4f, got %.4f", expected, canonicalSignal.Redundancy)
	}
}

func TestRedundancySignalProportionalToSimilarity(t *testing.T) {
	// Create two TDUs with known similarity relationships
	// Use identical content so similarity = 1.0 (hamming distance = 0)
	records := []TDURecord{
		{ID: "canonical", Content: "how to write a function in go that parses json data from api", FitnessScore: 0.9},
		{ID: "very-similar", Content: "how to write a function in go that parses json data from api", FitnessScore: 0.4},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	ranked := []RankedCluster{
		{
			Cluster:   Cluster{Members: []*TDURecord{&records[0], &records[1]}},
			Canonical: &records[0],
			Redundant: []*TDURecord{&records[1]},
		},
	}

	sigGen := NewSignalGenerator(0.85)
	signals := sigGen.GenerateSignals(ranked)

	var redundantSignal *FitnessSignalEntry
	for i := range signals {
		if signals[i].SampleID == "very-similar" {
			redundantSignal = &signals[i]
		}
	}

	if redundantSignal == nil {
		t.Fatal("redundant signal not found")
	}

	// similarity=1.0, threshold=0.85, decay = 1.0 - 0.85 = 0.15
	// Redundancy = 0.5 - 0.15 = 0.35
	expectedDecay := 1.0 - 0.85
	expectedRedundancy := 0.5 - expectedDecay
	if math.Abs(redundantSignal.Redundancy-expectedRedundancy) > 0.01 {
		t.Errorf("redundant Redundancy should be ~%.4f, got %.4f", expectedRedundancy, redundantSignal.Redundancy)
	}

	// Verify it's less than neutral
	if redundantSignal.Redundancy >= 0.5 {
		t.Errorf("redundant signal should be below neutral 0.5, got %.4f", redundantSignal.Redundancy)
	}
}

func TestSingleTDUNoRedundancySignal(t *testing.T) {
	records := []TDURecord{
		{ID: "solo", Content: "unique standalone content", FitnessScore: 0.7},
	}

	ranked := []RankedCluster{
		{
			Cluster:   Cluster{Members: []*TDURecord{&records[0]}},
			Canonical: &records[0],
			Redundant: nil,
		},
	}

	sigGen := NewSignalGenerator(0.85)
	signals := sigGen.GenerateSignals(ranked)

	if len(signals) != 0 {
		t.Errorf("single TDU should produce no signals, got %d", len(signals))
	}
}

func TestSignalBatchBuilds(t *testing.T) {
	sigGen := NewSignalGenerator(0.85)

	signals := []FitnessSignalEntry{
		{SampleID: "a", TrainingInfluence: 0.5, UsageCorrelation: 0.5, Redundancy: 0.55},
		{SampleID: "b", TrainingInfluence: 0.5, UsageCorrelation: 0.5, Redundancy: 0.35},
	}

	batch := sigGen.BuildBatch(signals)
	if batch.Count != 2 {
		t.Errorf("expected count 2, got %d", batch.Count)
	}
	if batch.Method != "redundancy" {
		t.Errorf("expected method redundancy, got %s", batch.Method)
	}
	if batch.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
}

func TestSignalsAllInRange(t *testing.T) {
	records := []TDURecord{
		{ID: "c1", Content: "how to write a function in go that parses json data from http", FitnessScore: 0.9},
		{ID: "r1", Content: "how to write a function in go that parses json data from http requests", FitnessScore: 0.3},
		{ID: "r2", Content: "how to write a function in go that parses json data from http api", FitnessScore: 0.2},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	ranked := []RankedCluster{
		{
			Cluster:   Cluster{Members: []*TDURecord{&records[0], &records[1], &records[2]}},
			Canonical: &records[0],
			Redundant: []*TDURecord{&records[1], &records[2]},
		},
	}

	sigGen := NewSignalGenerator(0.85)
	signals := sigGen.GenerateSignals(ranked)

	for _, s := range signals {
		if s.Redundancy < 0.0 || s.Redundancy > 1.0 {
			t.Errorf("Redundancy out of [0,1]: %f for %s", s.Redundancy, s.SampleID)
		}
		if s.TrainingInfluence < 0.0 || s.TrainingInfluence > 1.0 {
			t.Errorf("TrainingInfluence out of [0,1]: %f for %s", s.TrainingInfluence, s.SampleID)
		}
		if s.UsageCorrelation < 0.0 || s.UsageCorrelation > 1.0 {
			t.Errorf("UsageCorrelation out of [0,1]: %f for %s", s.UsageCorrelation, s.SampleID)
		}
	}
}

// ─── Integration / End-to-End Tests ─────────────────────────────────────────

func TestEndToEndPipeline(t *testing.T) {
	tmpDir := t.TempDir()

	// Create TDU corpus with some duplicates
	tdus := []map[string]any{
		// Cluster 1: two near-identical code TDUs
		{"id": "code-001", "content": "how to write a function in go that parses json data from http endpoints", "domain": "code", "fitness_score": 0.8},
		{"id": "code-002", "content": "how to write a function in go that parses json data from http api endpoints", "domain": "code", "fitness_score": 0.5},
		// Cluster 2: unique TDU (no redundancy)
		{"id": "reasoning-001", "content": "explain the trolley problem in ethics and its philosophical implications for autonomous vehicles", "domain": "reasoning", "fitness_score": 0.7},
		// Cluster 3: unique cooking content
		{"id": "instruction-001", "content": "the best recipe for chocolate cake with vanilla frosting and rainbow sprinkles", "domain": "instruction", "fitness_score": 0.6},
	}

	path := filepath.Join(tmpDir, "corpus.jsonl")
	f, _ := os.Create(path)
	for _, tdu := range tdus {
		data, _ := json.Marshal(tdu)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	// 1. Embed
	embedder := NewEmbedder(100)
	records, err := embedder.LoadAndEmbed(path)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("expected 4, got %d", len(records))
	}

	// 2. Cluster
	clusterer := NewClusterer(0.85)
	clusters := clusterer.ClusterTDUs(records)

	// 3. Rank
	ranker := NewRanker()
	ranked := ranker.RankClusters(clusters)

	// 4. Generate signals
	sigGen := NewSignalGenerator(0.85)
	signals := sigGen.GenerateSignals(ranked)

	// 5. Build batch
	batch := sigGen.BuildBatch(signals)

	// Verify batch serializes correctly
	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	outPath := filepath.Join(tmpDir, "signals.json")
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read back and verify
	readBack, _ := os.ReadFile(outPath)
	var decoded FitnessSignalBatch
	if err := json.Unmarshal(readBack, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Method != "redundancy" {
		t.Errorf("expected method redundancy, got %s", decoded.Method)
	}

	// All signals should be in valid range
	for _, s := range decoded.Signals {
		if s.Redundancy < 0 || s.Redundancy > 1 {
			t.Errorf("Redundancy out of range: %f for %s", s.Redundancy, s.SampleID)
		}
	}
}

func TestEndToEndWithCorrection(t *testing.T) {
	// Correction TDU with low fitness should still become canonical
	records := []TDURecord{
		{ID: "original", Content: "how to write a function in go that parses json data from endpoints", FitnessScore: 0.9},
		{ID: "correction", Content: "how to write a function in go that parses json data from endpoints correctly", FitnessScore: 0.3, IsCorrection: true},
	}
	embedder := NewEmbedder(100)
	embedder.EmbedBatch(records)

	clusterer := NewClusterer(0.85)
	clusters := clusterer.ClusterTDUs(records)

	ranker := NewRanker()
	ranked := ranker.RankClusters(clusters)

	// Find the cluster containing both
	for _, rc := range ranked {
		if len(rc.Redundant) > 0 {
			if rc.Canonical.ID != "correction" {
				t.Errorf("correction should be canonical, got %s", rc.Canonical.ID)
			}
		}
	}

	sigGen := NewSignalGenerator(0.85)
	signals := sigGen.GenerateSignals(ranked)

	// Canonical (correction) should have boosted signal
	for _, s := range signals {
		if s.SampleID == "correction" {
			if s.Redundancy < 0.5 {
				t.Errorf("canonical correction should have Redundancy >= 0.5, got %f", s.Redundancy)
			}
		}
		if s.SampleID == "original" {
			if s.Redundancy >= 0.5 {
				t.Errorf("redundant original should have Redundancy < 0.5, got %f", s.Redundancy)
			}
		}
	}
}

func TestClampSignal(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-0.5, 0.0},
		{0.0, 0.0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}

	for _, tt := range tests {
		got := clampSignal(tt.input)
		if got != tt.expected {
			t.Errorf("clampSignal(%f) = %f, want %f", tt.input, got, tt.expected)
		}
	}
}
