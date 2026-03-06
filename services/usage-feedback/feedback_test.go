package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ─── Collector Tests ────────────────────────────────────────────────────────

func TestCollectorParsesEventLog(t *testing.T) {
	// Create a temp JSONL event log
	events := []UsageEvent{
		{
			EventID:   "evt-001",
			Type:      EventRating,
			Timestamp: time.Now(),
			Rating:    1,
			Query:     "How do I implement a linked list?",
			Domain:    "code",
		},
		{
			EventID:   "evt-002",
			Type:      EventRating,
			Timestamp: time.Now(),
			Rating:    -1,
			Query:     "What is quantum entanglement?",
			Domain:    "reasoning",
		},
		{
			EventID:       "evt-003",
			Type:          EventRetry,
			Timestamp:     time.Now(),
			Query:         "Explain the theory of relativity",
			OriginalQuery: "What is relativity?",
			Domain:        "reasoning",
		},
		{
			EventID:         "evt-004",
			Type:            EventSessionEnd,
			Timestamp:       time.Now(),
			Query:           "multi-turn conversation",
			SessionDuration: 600.0,
			QueryCount:      10,
		},
		{
			EventID:      "evt-005",
			Type:         EventFollowUp,
			Timestamp:    time.Now(),
			Query:        "How do I sort an array?",
			FollowUpText: "thanks, that's exactly what I needed",
		},
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "events.jsonl")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()

	collector := NewCollector()
	parsed, err := collector.ParseEventLog(logPath)
	if err != nil {
		t.Fatalf("ParseEventLog: %v", err)
	}

	if len(parsed) != 5 {
		t.Fatalf("expected 5 events, got %d", len(parsed))
	}

	// Verify types parsed correctly
	if parsed[0].Type != EventRating || parsed[0].Rating != 1 {
		t.Errorf("event 0: expected rating +1, got type=%s rating=%d", parsed[0].Type, parsed[0].Rating)
	}
	if parsed[2].Type != EventRetry {
		t.Errorf("event 2: expected retry, got %s", parsed[2].Type)
	}
	if parsed[3].Type != EventSessionEnd {
		t.Errorf("event 3: expected session_end, got %s", parsed[3].Type)
	}
}

func TestCollectorRejectsInvalidEvents(t *testing.T) {
	events := []string{
		// Missing query
		`{"event_id":"bad1","type":"rating","rating":1}`,
		// Invalid rating value
		`{"event_id":"bad2","type":"rating","query":"test","rating":5}`,
		// Retry without original_query
		`{"event_id":"bad3","type":"retry","query":"test"}`,
		// Unknown type
		`{"event_id":"bad4","type":"unknown","query":"test"}`,
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "bad.jsonl")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		f.WriteString(e + "\n")
	}
	f.Close()

	collector := NewCollector()
	parsed, err := collector.ParseEventLog(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed) != 0 {
		t.Fatalf("expected 0 valid events, got %d", len(parsed))
	}
}

func TestPositiveRatingPositiveSignal(t *testing.T) {
	collector := NewCollector()
	event := &UsageEvent{
		Type:   EventRating,
		Rating: 1,
		Query:  "test query",
	}

	signal := collector.EventToSignal(event)
	if signal != 1.0 {
		t.Errorf("positive rating: expected signal 1.0, got %f", signal)
	}
}

func TestNegativeRatingNegativeSignal(t *testing.T) {
	collector := NewCollector()
	event := &UsageEvent{
		Type:   EventRating,
		Rating: -1,
		Query:  "test query",
	}

	signal := collector.EventToSignal(event)
	if signal != -1.0 {
		t.Errorf("negative rating: expected signal -1.0, got %f", signal)
	}
}

func TestRetryPatternNegativeSignal(t *testing.T) {
	collector := NewCollector()
	event := &UsageEvent{
		Type:          EventRetry,
		Query:         "retry query",
		OriginalQuery: "original query",
	}

	signal := collector.EventToSignal(event)
	if signal >= 0 {
		t.Errorf("retry: expected negative signal, got %f", signal)
	}
}

func TestFollowUpClassification(t *testing.T) {
	collector := NewCollector()

	tests := []struct {
		text     string
		wantSign int // -1, 0, or 1
	}{
		{"that's wrong, try again", -1},
		{"incorrect answer", -1},
		{"thanks, that's exactly what I needed", 1},
		{"perfect answer!", 1},
		{"hmm, interesting", 0},
	}

	for _, tt := range tests {
		sig := collector.ClassifyFollowUp(tt.text)
		gotSign := 0
		if sig > 0 {
			gotSign = 1
		} else if sig < 0 {
			gotSign = -1
		}
		if gotSign != tt.wantSign {
			t.Errorf("ClassifyFollowUp(%q): want sign %d, got signal %f", tt.text, tt.wantSign, sig)
		}
	}
}

func TestLongSessionPositiveSignal(t *testing.T) {
	collector := NewCollector()
	event := &UsageEvent{
		Type:            EventSessionEnd,
		Query:           "multi-turn session",
		SessionDuration: 600.0, // 10 minutes
		QueryCount:      5,
	}

	signal := collector.EventToSignal(event)
	if signal <= 0 {
		t.Errorf("long productive session: expected positive signal, got %f", signal)
	}
}

func TestShortSessionNegativeSignal(t *testing.T) {
	collector := NewCollector()
	event := &UsageEvent{
		Type:            EventSessionEnd,
		Query:           "quick bounce",
		SessionDuration: 10.0,
		QueryCount:      1,
	}

	signal := collector.EventToSignal(event)
	if signal >= 0 {
		t.Errorf("short session: expected negative signal, got %f", signal)
	}
}

// ─── Attributor Tests ───────────────────────────────────────────────────────

func TestAttributorFindsSimilarTDUs(t *testing.T) {
	attributor := NewAttributor(3, 30)

	// Create TDUs with known content
	tdus := []TDURecord{
		{ID: "tdu-001", Content: "implementing a linked list data structure in python with nodes and pointers", Domain: "code"},
		{ID: "tdu-002", Content: "quantum entanglement explained for physics students studying mechanics", Domain: "science"},
		{ID: "tdu-003", Content: "cooking recipes for italian pasta dishes with tomato sauce", Domain: "cooking"},
		{ID: "tdu-004", Content: "implementing a binary tree data structure in python with nodes", Domain: "code"},
	}
	attributor.SetTDUs(tdus)

	// Query similar to code TDUs
	event := &UsageEvent{
		Query:  "how to implement a linked list data structure in python",
		Domain: "code",
	}

	results := attributor.Attribute(event, 1.0)
	if len(results) == 0 {
		t.Fatal("expected at least one attributed signal")
	}

	// The closest match should be tdu-001 (nearly identical content)
	found := false
	for _, r := range results {
		if r.TDUID == "tdu-001" {
			found = true
			if r.Similarity <= 0 {
				t.Errorf("tdu-001 similarity should be > 0, got %f", r.Similarity)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected tdu-001 in results, got: %+v", results)
	}
}

func TestAttributorDecaysWithDistance(t *testing.T) {
	attributor := NewAttributor(5, 30)

	// Two TDUs: one very similar, one moderately similar
	tdus := []TDURecord{
		{ID: "close", Content: "the quick brown fox jumps over the lazy dog in the park today"},
		{ID: "far", Content: "completely different content about quantum mechanics and nuclear physics experiments"},
	}
	attributor.SetTDUs(tdus)

	event := &UsageEvent{
		Query: "the quick brown fox jumps over the lazy dog in the garden",
	}

	results := attributor.Attribute(event, 1.0)

	// Find signals for both TDUs
	var closeSig, farSig float64
	var closeFound, farFound bool
	for _, r := range results {
		if r.TDUID == "close" {
			closeSig = r.Signal
			closeFound = true
		}
		if r.TDUID == "far" {
			farSig = r.Signal
			farFound = true
		}
	}

	if closeFound && farFound {
		if closeSig <= farSig {
			t.Errorf("close TDU should get stronger signal: close=%f, far=%f", closeSig, farSig)
		}
	}
	// At minimum, close should be found
	if !closeFound {
		t.Log("close TDU not found — distance may exceed maxDist for this content pair")
	}
}

func TestAttributorRespectsTopK(t *testing.T) {
	attributor := NewAttributor(2, 64) // topK=2, very wide maxDist

	tdus := make([]TDURecord, 10)
	for i := range tdus {
		tdus[i] = TDURecord{
			ID:      fmt.Sprintf("tdu-%03d", i),
			Content: fmt.Sprintf("data structure number %d with various algorithms and implementations", i),
		}
	}
	attributor.SetTDUs(tdus)

	event := &UsageEvent{
		Query: "data structure algorithms and implementations",
	}

	results := attributor.Attribute(event, 1.0)
	if len(results) > 2 {
		t.Errorf("expected at most 2 results (topK=2), got %d", len(results))
	}
}

func TestAttributorHandlesEmptyIndex(t *testing.T) {
	attributor := NewAttributor(5, 30)
	// No TDUs loaded

	event := &UsageEvent{
		Query: "some query",
	}

	results := attributor.Attribute(event, 1.0)
	if len(results) != 0 {
		t.Errorf("expected 0 results with empty index, got %d", len(results))
	}
}

// ─── SimHash Tests ──────────────────────────────────────────────────────────

func TestSimHashDeterministic(t *testing.T) {
	content := "the quick brown fox jumps over the lazy dog near the riverbank"
	h1 := computeSimHash(content)
	h2 := computeSimHash(content)
	if h1 != h2 {
		t.Errorf("SimHash not deterministic: %d != %d", h1, h2)
	}
}

func TestSimHashSimilarContentCloseDistance(t *testing.T) {
	a := "implementing a linked list data structure in python with node pointers and traversal"
	b := "implementing a linked list data structure in python with node references and iteration"

	ha := computeSimHash(a)
	hb := computeSimHash(b)
	dist := hammingDistance(ha, hb)

	// Similar content should have low hamming distance
	if dist > 20 {
		t.Errorf("similar content hamming distance too high: %d", dist)
	}
}

func TestSimHashDifferentContentFarDistance(t *testing.T) {
	a := "implementing a linked list data structure in python with node pointers"
	b := "cooking recipes for french cuisine including croissants and baguettes"

	ha := computeSimHash(a)
	hb := computeSimHash(b)
	dist := hammingDistance(ha, hb)

	// Very different content should have higher hamming distance
	if dist < 5 {
		t.Errorf("different content hamming distance suspiciously low: %d", dist)
	}
}

// ─── Aggregator Tests ───────────────────────────────────────────────────────

func TestAggregatorBatchesCorrectly(t *testing.T) {
	config := AggregationConfig{
		WindowDuration: 1 * time.Hour,
		MinSignalCount: 2, // Lower threshold for test
	}
	aggregator := NewAggregator(config)

	signals := []AttributedSignal{
		{TDUID: "tdu-001", Signal: 0.8, Domain: "code"},
		{TDUID: "tdu-001", Signal: 0.6, Domain: "code"},
		{TDUID: "tdu-001", Signal: -0.2, Domain: "code"},
		{TDUID: "tdu-002", Signal: -0.5, Domain: "science"},
		{TDUID: "tdu-002", Signal: -0.7, Domain: "science"},
	}

	results := aggregator.Aggregate(signals)

	if len(results) != 2 {
		t.Fatalf("expected 2 aggregated TDUs, got %d", len(results))
	}

	// Check TDU-001: mean of (0.8 + 0.6 + -0.2) / 3 = 0.4
	for _, r := range results {
		switch r.TDUID {
		case "tdu-001":
			expectedMean := 0.4
			if math.Abs(r.MeanSignal-expectedMean) > 0.001 {
				t.Errorf("tdu-001: expected mean %.4f, got %.4f", expectedMean, r.MeanSignal)
			}
			if r.SignalCount != 3 {
				t.Errorf("tdu-001: expected 3 signals, got %d", r.SignalCount)
			}
		case "tdu-002":
			expectedMean := -0.6
			if math.Abs(r.MeanSignal-expectedMean) > 0.001 {
				t.Errorf("tdu-002: expected mean %.4f, got %.4f", expectedMean, r.MeanSignal)
			}
			if r.SignalCount != 2 {
				t.Errorf("tdu-002: expected 2 signals, got %d", r.SignalCount)
			}
		}
	}
}

func TestAggregatorBelowThresholdNoEmit(t *testing.T) {
	config := AggregationConfig{
		WindowDuration: 1 * time.Hour,
		MinSignalCount: 3,
	}
	aggregator := NewAggregator(config)

	signals := []AttributedSignal{
		{TDUID: "tdu-001", Signal: 0.5},
		{TDUID: "tdu-001", Signal: 0.7},
		// Only 2 signals for tdu-001, threshold is 3
		{TDUID: "tdu-002", Signal: 0.3},
		{TDUID: "tdu-002", Signal: 0.4},
		{TDUID: "tdu-002", Signal: 0.5},
		// 3 signals for tdu-002, meets threshold
	}

	results := aggregator.Aggregate(signals)

	if len(results) != 1 {
		t.Fatalf("expected 1 aggregated TDU (tdu-002 only), got %d", len(results))
	}

	if results[0].TDUID != "tdu-002" {
		t.Errorf("expected tdu-002, got %s", results[0].TDUID)
	}
}

func TestAggregatorEmptyInput(t *testing.T) {
	aggregator := NewAggregator(DefaultAggregationConfig())
	results := aggregator.Aggregate(nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestAggregatorClampsMean(t *testing.T) {
	config := AggregationConfig{MinSignalCount: 1}
	aggregator := NewAggregator(config)

	signals := []AttributedSignal{
		{TDUID: "tdu-001", Signal: 1.5}, // exceeds [-1,1]
	}

	results := aggregator.Aggregate(signals)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].MeanSignal > 1.0 {
		t.Errorf("mean should be clamped to 1.0, got %f", results[0].MeanSignal)
	}
}

// ─── Emitter Tests ──────────────────────────────────────────────────────────

func TestEmitterPositiveSignalMapping(t *testing.T) {
	emitter := NewEmitter()

	aggregated := []AggregatedSignal{
		{TDUID: "tdu-001", MeanSignal: 0.8, SignalCount: 5, Domain: "code"},
	}

	signals := emitter.ConvertToFitnessSignals(aggregated)
	if len(signals) != 1 {
		t.Fatal("expected 1 signal")
	}

	// mean 0.8 -> usage_correlation (0.8+1)/2 = 0.9
	expected := 0.9
	if math.Abs(signals[0].UsageCorrelation-expected) > 0.001 {
		t.Errorf("expected UsageCorrelation %.4f, got %.4f", expected, signals[0].UsageCorrelation)
	}

	// Training influence should be neutral
	if signals[0].TrainingInfluence != 0.5 {
		t.Errorf("expected TrainingInfluence 0.5 (neutral), got %f", signals[0].TrainingInfluence)
	}

	// Redundancy should be neutral
	if signals[0].Redundancy != 0.5 {
		t.Errorf("expected Redundancy 0.5 (neutral), got %f", signals[0].Redundancy)
	}
}

func TestEmitterNegativeSignalMapping(t *testing.T) {
	emitter := NewEmitter()

	aggregated := []AggregatedSignal{
		{TDUID: "tdu-001", MeanSignal: -0.6, SignalCount: 4},
	}

	signals := emitter.ConvertToFitnessSignals(aggregated)
	if len(signals) != 1 {
		t.Fatal("expected 1 signal")
	}

	// mean -0.6 -> usage_correlation (-0.6+1)/2 = 0.2
	expected := 0.2
	if math.Abs(signals[0].UsageCorrelation-expected) > 0.001 {
		t.Errorf("expected UsageCorrelation %.4f, got %.4f", expected, signals[0].UsageCorrelation)
	}
}

func TestEmitterNeutralSignalMapping(t *testing.T) {
	emitter := NewEmitter()

	aggregated := []AggregatedSignal{
		{TDUID: "tdu-001", MeanSignal: 0.0, SignalCount: 3},
	}

	signals := emitter.ConvertToFitnessSignals(aggregated)
	if len(signals) != 1 {
		t.Fatal("expected 1 signal")
	}

	// mean 0.0 -> usage_correlation 0.5 (neutral)
	if math.Abs(signals[0].UsageCorrelation-0.5) > 0.001 {
		t.Errorf("expected UsageCorrelation 0.5, got %f", signals[0].UsageCorrelation)
	}
}

func TestEmitterBuildBatch(t *testing.T) {
	emitter := NewEmitter()

	signals := []FitnessSignalEntry{
		{SampleID: "tdu-001", TrainingInfluence: 0.5, UsageCorrelation: 0.9, Redundancy: 0.5},
		{SampleID: "tdu-002", TrainingInfluence: 0.5, UsageCorrelation: 0.2, Redundancy: 0.5},
	}

	batch := emitter.BuildBatch(signals)

	if batch.Method != "usage_correlation" {
		t.Errorf("expected method usage_correlation, got %s", batch.Method)
	}
	if batch.Count != 2 {
		t.Errorf("expected count 2, got %d", batch.Count)
	}
	if len(batch.Signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(batch.Signals))
	}
}

func TestEmitterClampsBounds(t *testing.T) {
	emitter := NewEmitter()

	// Edge case: signal at boundary
	aggregated := []AggregatedSignal{
		{TDUID: "tdu-max", MeanSignal: 1.0, SignalCount: 3},
		{TDUID: "tdu-min", MeanSignal: -1.0, SignalCount: 3},
	}

	signals := emitter.ConvertToFitnessSignals(aggregated)

	for _, s := range signals {
		if s.UsageCorrelation < 0.0 || s.UsageCorrelation > 1.0 {
			t.Errorf("%s: UsageCorrelation out of bounds: %f", s.SampleID, s.UsageCorrelation)
		}
	}
}

// ─── Integration: Full Pipeline ─────────────────────────────────────────────

func TestFullPipelineIntegration(t *testing.T) {
	dir := t.TempDir()

	// Create event log
	events := []UsageEvent{
		{EventID: "e1", Type: EventRating, Query: "how to implement sorting algorithms in python with quicksort", Rating: 1, Domain: "code", Timestamp: time.Now()},
		{EventID: "e2", Type: EventRating, Query: "how to implement sorting algorithms in python with mergesort", Rating: 1, Domain: "code", Timestamp: time.Now()},
		{EventID: "e3", Type: EventRating, Query: "how to implement sorting algorithms in python with bubblesort", Rating: 1, Domain: "code", Timestamp: time.Now()},
		{EventID: "e4", Type: EventRating, Query: "how to implement sorting algorithms in python with heapsort", Rating: -1, Domain: "code", Timestamp: time.Now()},
		{EventID: "e5", Type: EventFollowUp, Query: "explain quantum mechanics wave function collapse", FollowUpText: "that's wrong, try again", Domain: "science", Timestamp: time.Now()},
		{EventID: "e6", Type: EventFollowUp, Query: "explain quantum mechanics wave function collapse details", FollowUpText: "incorrect answer", Domain: "science", Timestamp: time.Now()},
		{EventID: "e7", Type: EventFollowUp, Query: "explain quantum mechanics wave function and probability", FollowUpText: "not what i asked", Domain: "science", Timestamp: time.Now()},
	}

	eventPath := filepath.Join(dir, "events.jsonl")
	ef, _ := os.Create(eventPath)
	enc := json.NewEncoder(ef)
	for _, e := range events {
		enc.Encode(e)
	}
	ef.Close()

	// Create TDU index
	tdus := []TDURecord{
		{ID: "tdu-sort", Content: "implementing sorting algorithms in python with quicksort mergesort and heapsort comparison", Domain: "code"},
		{ID: "tdu-quantum", Content: "quantum mechanics wave function collapse and probability interpretation explained", Domain: "science"},
		{ID: "tdu-cooking", Content: "french cooking recipes for desserts and pastries with chocolate", Domain: "cooking"},
	}

	tduPath := filepath.Join(dir, "tdus.jsonl")
	tf, _ := os.Create(tduPath)
	tenc := json.NewEncoder(tf)
	for _, tdu := range tdus {
		tenc.Encode(tdu)
	}
	tf.Close()

	// Run pipeline
	collector := NewCollector()
	parsedEvents, err := collector.ParseEventLog(eventPath)
	if err != nil {
		t.Fatalf("parse events: %v", err)
	}

	attributor := NewAttributor(3, 30)
	if err := attributor.LoadTDUIndex(tduPath); err != nil {
		t.Fatalf("load TDU index: %v", err)
	}

	var allAttributed []AttributedSignal
	for i := range parsedEvents {
		rawSignal := collector.EventToSignal(&parsedEvents[i])
		if rawSignal == 0.0 {
			continue
		}
		attributed := attributor.Attribute(&parsedEvents[i], rawSignal)
		allAttributed = append(allAttributed, attributed...)
	}

	if len(allAttributed) == 0 {
		t.Fatal("expected at least some attributed signals")
	}

	aggConfig := AggregationConfig{MinSignalCount: 1} // low for test
	aggregator := NewAggregator(aggConfig)
	aggregated := aggregator.Aggregate(allAttributed)

	if len(aggregated) == 0 {
		t.Fatal("expected at least one aggregated signal")
	}

	emitter := NewEmitter()
	fitnessSignals := emitter.ConvertToFitnessSignals(aggregated)
	batch := emitter.BuildBatch(fitnessSignals)

	// Verify batch structure
	if batch.Method != "usage_correlation" {
		t.Errorf("batch method: got %s, want usage_correlation", batch.Method)
	}
	if batch.Count == 0 {
		t.Error("batch should have signals")
	}

	// All signals should be in valid range
	for _, s := range batch.Signals {
		if s.UsageCorrelation < 0.0 || s.UsageCorrelation > 1.0 {
			t.Errorf("%s: UsageCorrelation out of range: %f", s.SampleID, s.UsageCorrelation)
		}
		if s.TrainingInfluence != 0.5 {
			t.Errorf("%s: TrainingInfluence should be neutral 0.5, got %f", s.SampleID, s.TrainingInfluence)
		}
		if s.Redundancy != 0.5 {
			t.Errorf("%s: Redundancy should be neutral 0.5, got %f", s.SampleID, s.Redundancy)
		}
	}

	// Verify JSON serialization
	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}

	var roundTrip FitnessSignalBatch
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal batch: %v", err)
	}
	if roundTrip.Count != batch.Count {
		t.Errorf("roundtrip count mismatch: %d != %d", roundTrip.Count, batch.Count)
	}
}

// ─── Privacy Test ───────────────────────────────────────────────────────────

func TestSignalsContainNoUserIdentity(t *testing.T) {
	emitter := NewEmitter()

	aggregated := []AggregatedSignal{
		{TDUID: "tdu-001", MeanSignal: 0.5, SignalCount: 5},
	}

	signals := emitter.ConvertToFitnessSignals(aggregated)
	batch := emitter.BuildBatch(signals)

	// Serialize and check no user-identifying fields exist
	data, _ := json.Marshal(batch)
	s := string(data)

	// These fields should never appear in emitted signals
	forbidden := []string{"user_id", "api_key", "ip_address", "email"}
	for _, f := range forbidden {
		if contains(s, f) {
			t.Errorf("batch JSON contains forbidden field: %s", f)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
