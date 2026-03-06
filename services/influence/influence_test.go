package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// ─── Loss Tracker Tests ────────────────────────────────────────────────────

func TestLossTrackerParseTrainingLog(t *testing.T) {
	// Create a temp JSONL file with training log entries
	entries := []map[string]any{
		{
			"_meta": map[string]any{
				"id":             "tdu-001",
				"domain":         "code",
				"quality_tier":   "gold",
				"token_estimate": 512,
			},
			"initial_loss": 3.5,
			"final_loss":   1.2,
			"epochs":       3,
		},
		{
			"_meta": map[string]any{
				"id":             "tdu-002",
				"domain":         "reasoning",
				"quality_tier":   "silver",
				"token_estimate": 256,
			},
			"initial_loss": 4.0,
			"final_loss":   2.8,
			"epochs":       3,
		},
		{
			"tdu_id":       "tdu-003",
			"domain":       "instruction",
			"initial_loss": 2.0,
			"final_loss":   0.5,
			"epochs":       5,
		},
		{
			"_meta": map[string]any{
				"id": "tdu-004",
			},
			"loss": 1.8,
		},
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "training.jsonl")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, _ := json.Marshal(entry)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	tracker := NewLossTracker()
	parsed, err := tracker.ParseTrainingLog(logPath)
	if err != nil {
		t.Fatalf("ParseTrainingLog error: %v", err)
	}

	if len(parsed) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(parsed))
	}

	// Check entry from _meta format
	if parsed[0].TDUID != "tdu-001" {
		t.Errorf("expected tdu-001, got %s", parsed[0].TDUID)
	}
	if parsed[0].Domain != "code" {
		t.Errorf("expected domain code, got %s", parsed[0].Domain)
	}
	if parsed[0].FinalLoss != 1.2 {
		t.Errorf("expected final_loss 1.2, got %f", parsed[0].FinalLoss)
	}
	if parsed[0].InitialLoss != 3.5 {
		t.Errorf("expected initial_loss 3.5, got %f", parsed[0].InitialLoss)
	}

	// Check entry from flat format
	if parsed[2].TDUID != "tdu-003" {
		t.Errorf("expected tdu-003, got %s", parsed[2].TDUID)
	}
	if parsed[2].Domain != "instruction" {
		t.Errorf("expected domain instruction, got %s", parsed[2].Domain)
	}

	// Check entry with only "loss" field (no initial/final)
	if parsed[3].TDUID != "tdu-004" {
		t.Errorf("expected tdu-004, got %s", parsed[3].TDUID)
	}
	if parsed[3].FinalLoss != 1.8 {
		t.Errorf("expected final_loss 1.8 from loss field, got %f", parsed[3].FinalLoss)
	}
}

func TestLossTrackerSkipsMalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "bad.jsonl")
	content := `{"_meta":{"id":"good"},"final_loss":1.0}
not json at all
{"_meta":{"id":""},"final_loss":2.0}
{"_meta":{"id":"also-good"},"final_loss":0.5}
`
	os.WriteFile(logPath, []byte(content), 0644)

	tracker := NewLossTracker()
	parsed, err := tracker.ParseTrainingLog(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip malformed and empty-ID lines
	if len(parsed) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(parsed))
	}
	if parsed[0].TDUID != "good" {
		t.Errorf("expected 'good', got %s", parsed[0].TDUID)
	}
	if parsed[1].TDUID != "also-good" {
		t.Errorf("expected 'also-good', got %s", parsed[1].TDUID)
	}
}

func TestLossTrackerAggregateBatchLoss(t *testing.T) {
	entries := []LossEntry{
		{TDUID: "a", FinalLoss: 1.0},
		{TDUID: "b", FinalLoss: 2.0},
		{TDUID: "c", FinalLoss: 3.0},
		{TDUID: "d", FinalLoss: 4.0},
	}

	tracker := NewLossTracker()
	stats := tracker.AggregateBatchLoss(entries, []string{"a", "c"})

	if stats.Count != 2 {
		t.Errorf("expected count 2, got %d", stats.Count)
	}
	if stats.MeanLoss != 2.0 {
		t.Errorf("expected mean 2.0, got %f", stats.MeanLoss)
	}
	if stats.MinLoss != 1.0 {
		t.Errorf("expected min 1.0, got %f", stats.MinLoss)
	}
	if stats.MaxLoss != 3.0 {
		t.Errorf("expected max 3.0, got %f", stats.MaxLoss)
	}
}

func TestLossTrackerAggregateBatchLossEmpty(t *testing.T) {
	tracker := NewLossTracker()
	stats := tracker.AggregateBatchLoss(nil, []string{"x"})

	if stats.Count != 0 {
		t.Errorf("expected count 0, got %d", stats.Count)
	}
}

// ─── Analyzer Tests ─────────────────────────────────────────────────────────

func TestAnalyzerLossBasedNoEntries(t *testing.T) {
	analyzer := NewAnalyzer()
	results := analyzer.AnalyzeLossBased(nil, nil)
	if results != nil {
		t.Error("expected nil for empty entries")
	}
}

func TestAnalyzerLossBasedProducesCorrectDirection(t *testing.T) {
	analyzer := NewAnalyzer()

	entries := []LossEntry{
		{TDUID: "low-loss", Domain: "code", FinalLoss: 0.5},
		{TDUID: "mid-loss", Domain: "code", FinalLoss: 2.5},
		{TDUID: "high-loss", Domain: "code", FinalLoss: 4.5},
	}

	// With positive benchmark delta (training helped)
	positiveDelta := &BenchmarkDelta{
		BaselineScore:  0.6,
		CandidateScore: 0.8,
		OverallDelta:   0.2,
	}

	results := analyzer.AnalyzeLossBased(entries, positiveDelta)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// With positive training direction, near-median TDUs should score higher
	midResult := findResult(results, "mid-loss")
	if midResult == nil {
		t.Fatal("mid-loss result not found")
	}
	// mid-loss is closest to median, should have highest score
	lowResult := findResult(results, "low-loss")
	highResult := findResult(results, "high-loss")
	if lowResult == nil || highResult == nil {
		t.Fatal("results not found")
	}

	// Mid-loss should generally score higher than extreme outliers
	// (near-median is rated as foundational)
	if midResult.Score < highResult.Score {
		t.Errorf("mid-loss (%.4f) should score >= high-loss (%.4f) with positive delta",
			midResult.Score, highResult.Score)
	}
}

func TestAnalyzerLossBasedNegativeDelta(t *testing.T) {
	analyzer := NewAnalyzer()

	// Use enough entries so the median is meaningful and the high-loss outlier
	// is clearly above it
	entries := []LossEntry{
		{TDUID: "low-loss", Domain: "code", FinalLoss: 0.5},
		{TDUID: "mid-loss-1", Domain: "code", FinalLoss: 1.0},
		{TDUID: "mid-loss-2", Domain: "code", FinalLoss: 1.5},
		{TDUID: "mid-loss-3", Domain: "code", FinalLoss: 2.0},
		{TDUID: "high-loss", Domain: "code", FinalLoss: 8.0},
	}

	// Negative delta: training hurt model quality
	negativeDelta := &BenchmarkDelta{
		OverallDelta: -0.3,
	}

	results := analyzer.AnalyzeLossBased(entries, negativeDelta)
	highResult := findResult(results, "high-loss")
	if highResult == nil {
		t.Fatal("high-loss result not found")
	}

	// High-loss outlier TDU with negative training direction should get penalized
	if highResult.Score > 0 {
		t.Errorf("high-loss TDU with negative delta should have negative score, got %.4f",
			highResult.Score)
	}
}

func TestAnalyzerLossBasedNoBenchmark(t *testing.T) {
	analyzer := NewAnalyzer()

	entries := []LossEntry{
		{TDUID: "a", FinalLoss: 1.0},
		{TDUID: "b", FinalLoss: 2.0},
		{TDUID: "c", FinalLoss: 3.0},
	}

	// No benchmark delta — pure loss-based heuristic
	results := analyzer.AnalyzeLossBased(entries, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}

	// All scores should be in [-1, 1]
	for _, r := range results {
		if r.Score < -1.0 || r.Score > 1.0 {
			t.Errorf("score out of range: %f", r.Score)
		}
	}
}

func TestAnalyzerLossBasedScoresClamped(t *testing.T) {
	analyzer := NewAnalyzer()

	// Extreme entries
	entries := []LossEntry{
		{TDUID: "a", FinalLoss: 0.001},
		{TDUID: "b", FinalLoss: 100.0},
	}

	results := analyzer.AnalyzeLossBased(entries, &BenchmarkDelta{OverallDelta: 5.0})
	for _, r := range results {
		if r.Score < -1.0 || r.Score > 1.0 {
			t.Errorf("score out of bounds for %s: %f", r.TDUID, r.Score)
		}
	}
}

func TestAnalyzerLeaveOneOut(t *testing.T) {
	tmpDir := t.TempDir()

	// Write baseline report
	baseline := BenchmarkReport{
		OverallScore: 0.80,
		DomainScores: map[string]float64{
			"code":      0.85,
			"reasoning": 0.75,
		},
	}
	writeJSON(t, filepath.Join(tmpDir, "baseline.json"), baseline)

	// Write leave-one-out reports
	batchDir := filepath.Join(tmpDir, "batches")
	os.MkdirAll(batchDir, 0755)

	// Batch A: removing it drops score (positive influence)
	looA := LeaveOneOutReport{
		BenchmarkReport: BenchmarkReport{
			OverallScore: 0.70,
			DomainScores: map[string]float64{
				"code":      0.75,
				"reasoning": 0.65,
			},
		},
		ExcludedBatch: "batch-helpful",
	}
	writeJSON(t, filepath.Join(batchDir, "batch_a.json"), looA)

	// Batch B: removing it improves score (negative influence)
	looB := LeaveOneOutReport{
		BenchmarkReport: BenchmarkReport{
			OverallScore: 0.85,
			DomainScores: map[string]float64{
				"code":      0.88,
				"reasoning": 0.82,
			},
		},
		ExcludedBatch: "batch-harmful",
	}
	writeJSON(t, filepath.Join(batchDir, "batch_b.json"), looB)

	analyzer := NewAnalyzer()
	results, _, err := analyzer.AnalyzeLeaveOneOut(
		filepath.Join(tmpDir, "baseline.json"),
		batchDir,
	)
	if err != nil {
		t.Fatalf("AnalyzeLeaveOneOut error: %v", err)
	}

	// Should have results for both batches (overall + per-domain)
	if len(results) == 0 {
		t.Fatal("expected results")
	}

	// Find overall results
	helpfulOverall := findResultByTDUAndBench(results, "batch-helpful", "overall")
	harmfulOverall := findResultByTDUAndBench(results, "batch-harmful", "overall")

	if helpfulOverall == nil {
		t.Fatal("batch-helpful overall not found")
	}
	if harmfulOverall == nil {
		t.Fatal("batch-harmful overall not found")
	}

	// Helpful batch: removing it drops score -> positive influence
	if helpfulOverall.Score <= 0 {
		t.Errorf("helpful batch should have positive score, got %.4f", helpfulOverall.Score)
	}

	// Harmful batch: removing it improves score -> negative influence
	if harmfulOverall.Score >= 0 {
		t.Errorf("harmful batch should have negative score, got %.4f", harmfulOverall.Score)
	}
}

func TestAnalyzerLoadBenchmarkDelta(t *testing.T) {
	tmpDir := t.TempDir()

	baseline := BenchmarkReport{
		OverallScore: 0.60,
		DomainScores: map[string]float64{
			"code":      0.65,
			"reasoning": 0.55,
		},
	}
	candidate := BenchmarkReport{
		OverallScore: 0.75,
		DomainScores: map[string]float64{
			"code":      0.80,
			"reasoning": 0.70,
		},
	}
	writeJSON(t, filepath.Join(tmpDir, "base.json"), baseline)
	writeJSON(t, filepath.Join(tmpDir, "cand.json"), candidate)

	analyzer := NewAnalyzer()
	delta, err := analyzer.LoadBenchmarkDelta(
		filepath.Join(tmpDir, "base.json"),
		filepath.Join(tmpDir, "cand.json"),
	)
	if err != nil {
		t.Fatalf("LoadBenchmarkDelta error: %v", err)
	}

	if delta.BaselineScore != 0.60 {
		t.Errorf("expected baseline 0.60, got %f", delta.BaselineScore)
	}
	if delta.CandidateScore != 0.75 {
		t.Errorf("expected candidate 0.75, got %f", delta.CandidateScore)
	}
	if math.Abs(delta.OverallDelta-0.15) > 0.001 {
		t.Errorf("expected delta 0.15, got %f", delta.OverallDelta)
	}
	if math.Abs(delta.DomainDeltas["code"]-0.15) > 0.001 {
		t.Errorf("expected code delta 0.15, got %f", delta.DomainDeltas["code"])
	}
}

func TestReportComputeSummary(t *testing.T) {
	report := &InfluenceReport{
		Results: []InfluenceResult{
			{TDUID: "a", Score: 0.5, Benchmark: "code"},
			{TDUID: "b", Score: -0.3, Benchmark: "code"},
			{TDUID: "c", Score: 0.0, Benchmark: "reasoning"},
			{TDUID: "d", Score: 0.8, Benchmark: "reasoning"},
		},
	}

	report.ComputeSummary()

	if report.Summary == nil {
		t.Fatal("summary is nil")
	}
	if report.Summary.HelpfulCount != 2 {
		t.Errorf("expected 2 helpful, got %d", report.Summary.HelpfulCount)
	}
	if report.Summary.HarmfulCount != 1 {
		t.Errorf("expected 1 harmful, got %d", report.Summary.HarmfulCount)
	}
	if report.Summary.NeutralCount != 1 {
		t.Errorf("expected 1 neutral, got %d", report.Summary.NeutralCount)
	}
	if len(report.Summary.DomainBreakdown) != 2 {
		t.Errorf("expected 2 domains, got %d", len(report.Summary.DomainBreakdown))
	}
}

func TestReportComputeSummaryEmpty(t *testing.T) {
	report := &InfluenceReport{Results: nil}
	report.ComputeSummary()
	if report.Summary == nil {
		t.Fatal("summary should not be nil for empty results")
	}
}

// ─── Signal Emitter Tests ───────────────────────────────────────────────────

func TestSignalEmitterConvertsCorrectly(t *testing.T) {
	emitter := NewSignalEmitter()

	results := []InfluenceResult{
		{TDUID: "tdu-positive", Score: 0.8, Method: "loss-based", Benchmark: "code"},
		{TDUID: "tdu-negative", Score: -0.6, Method: "loss-based", Benchmark: "code"},
		{TDUID: "tdu-neutral", Score: 0.0, Method: "loss-based", Benchmark: "code"},
	}

	signals := emitter.ConvertToFitnessSignals(results)
	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d", len(signals))
	}

	sigMap := make(map[string]FitnessSignalEntry)
	for _, s := range signals {
		sigMap[s.SampleID] = s
	}

	// Positive influence -> TrainingInfluence near 1.0
	pos := sigMap["tdu-positive"]
	expectedPos := (0.8 + 1.0) / 2.0 // = 0.9
	if math.Abs(pos.TrainingInfluence-expectedPos) > 0.001 {
		t.Errorf("positive TDU: expected TrainingInfluence ~%.4f, got %.4f", expectedPos, pos.TrainingInfluence)
	}

	// Negative influence -> TrainingInfluence near 0.0
	neg := sigMap["tdu-negative"]
	expectedNeg := (-0.6 + 1.0) / 2.0 // = 0.2
	if math.Abs(neg.TrainingInfluence-expectedNeg) > 0.001 {
		t.Errorf("negative TDU: expected TrainingInfluence ~%.4f, got %.4f", expectedNeg, neg.TrainingInfluence)
	}

	// Neutral influence -> TrainingInfluence = 0.5
	neut := sigMap["tdu-neutral"]
	if math.Abs(neut.TrainingInfluence-0.5) > 0.001 {
		t.Errorf("neutral TDU: expected TrainingInfluence ~0.5, got %.4f", neut.TrainingInfluence)
	}

	// UsageCorrelation and Redundancy should be neutral (0.5)
	for _, s := range signals {
		if s.UsageCorrelation != 0.5 {
			t.Errorf("UsageCorrelation should be 0.5, got %f for %s", s.UsageCorrelation, s.SampleID)
		}
		if s.Redundancy != 0.5 {
			t.Errorf("Redundancy should be 0.5, got %f for %s", s.Redundancy, s.SampleID)
		}
	}
}

func TestSignalEmitterSignalRange(t *testing.T) {
	emitter := NewSignalEmitter()

	// Extreme values
	results := []InfluenceResult{
		{TDUID: "max-positive", Score: 1.0},
		{TDUID: "max-negative", Score: -1.0},
	}

	signals := emitter.ConvertToFitnessSignals(results)
	for _, s := range signals {
		if s.TrainingInfluence < 0.0 || s.TrainingInfluence > 1.0 {
			t.Errorf("TrainingInfluence out of [0,1]: %f for %s", s.TrainingInfluence, s.SampleID)
		}
	}

	// Max positive should map to 1.0
	sigMap := make(map[string]FitnessSignalEntry)
	for _, s := range signals {
		sigMap[s.SampleID] = s
	}
	if sigMap["max-positive"].TrainingInfluence != 1.0 {
		t.Errorf("max positive should map to 1.0, got %f", sigMap["max-positive"].TrainingInfluence)
	}
	if sigMap["max-negative"].TrainingInfluence != 0.0 {
		t.Errorf("max negative should map to 0.0, got %f", sigMap["max-negative"].TrainingInfluence)
	}
}

func TestSignalEmitterAggregatesMultiDomainResults(t *testing.T) {
	emitter := NewSignalEmitter()

	// Leave-one-out produces multiple results per TDU (one per domain)
	results := []InfluenceResult{
		{TDUID: "batch-1", Score: 0.5, Benchmark: "overall"},
		{TDUID: "batch-1", Score: 0.8, Benchmark: "code"},
		{TDUID: "batch-1", Score: 0.2, Benchmark: "reasoning"},
	}

	signals := emitter.ConvertToFitnessSignals(results)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (aggregated), got %d", len(signals))
	}

	// Average of 0.5, 0.8, 0.2 = 0.5 -> TrainingInfluence = (0.5 + 1) / 2 = 0.75
	expected := (0.5 + 1.0) / 2.0
	if math.Abs(signals[0].TrainingInfluence-expected) > 0.001 {
		t.Errorf("expected aggregated TrainingInfluence ~%.4f, got %.4f",
			expected, signals[0].TrainingInfluence)
	}
}

func TestSignalEmitterBuildBatch(t *testing.T) {
	emitter := NewSignalEmitter()

	signals := []FitnessSignalEntry{
		{SampleID: "a", TrainingInfluence: 0.9, UsageCorrelation: 0.5, Redundancy: 0.5},
		{SampleID: "b", TrainingInfluence: 0.2, UsageCorrelation: 0.5, Redundancy: 0.5},
	}

	batch := emitter.BuildBatch(signals)
	if batch.Count != 2 {
		t.Errorf("expected count 2, got %d", batch.Count)
	}
	if batch.Method != "training_influence" {
		t.Errorf("expected method training_influence, got %s", batch.Method)
	}
	if len(batch.Signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(batch.Signals))
	}
	if batch.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
}

func TestBatchSubmissionSerializesAllSignals(t *testing.T) {
	emitter := NewSignalEmitter()

	results := []InfluenceResult{
		{TDUID: "a", Score: 0.5},
		{TDUID: "b", Score: -0.3},
		{TDUID: "c", Score: 0.9},
	}

	signals := emitter.ConvertToFitnessSignals(results)
	batch := emitter.BuildBatch(signals)

	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}

	// Deserialize and verify
	var decoded FitnessSignalBatch
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal batch: %v", err)
	}

	if decoded.Count != 3 {
		t.Errorf("expected 3 in decoded batch, got %d", decoded.Count)
	}
	if len(decoded.Signals) != 3 {
		t.Errorf("expected 3 signals in decoded batch, got %d", len(decoded.Signals))
	}

	// All signals should have valid ranges
	for _, s := range decoded.Signals {
		if s.TrainingInfluence < 0 || s.TrainingInfluence > 1 {
			t.Errorf("TrainingInfluence out of range: %f", s.TrainingInfluence)
		}
		if s.UsageCorrelation < 0 || s.UsageCorrelation > 1 {
			t.Errorf("UsageCorrelation out of range: %f", s.UsageCorrelation)
		}
		if s.Redundancy < 0 || s.Redundancy > 1 {
			t.Errorf("Redundancy out of range: %f", s.Redundancy)
		}
	}
}

// ─── Integration Tests ──────────────────────────────────────────────────────

func TestEndToEndLossBasedPipeline(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create training log
	logEntries := []map[string]any{
		{"_meta": map[string]any{"id": "tdu-001", "domain": "code"}, "final_loss": 0.8},
		{"_meta": map[string]any{"id": "tdu-002", "domain": "code"}, "final_loss": 1.5},
		{"_meta": map[string]any{"id": "tdu-003", "domain": "reasoning"}, "final_loss": 2.5},
		{"_meta": map[string]any{"id": "tdu-004", "domain": "reasoning"}, "final_loss": 3.8},
		{"_meta": map[string]any{"id": "tdu-005", "domain": "code"}, "final_loss": 1.2},
	}
	logPath := filepath.Join(tmpDir, "train.jsonl")
	f, _ := os.Create(logPath)
	for _, e := range logEntries {
		data, _ := json.Marshal(e)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	// 2. Parse training log
	tracker := NewLossTracker()
	entries, err := tracker.ParseTrainingLog(logPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5, got %d", len(entries))
	}

	// 3. Run analysis
	analyzer := NewAnalyzer()
	results := analyzer.AnalyzeLossBased(entries, nil)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// 4. Generate signals
	emitter := NewSignalEmitter()
	signals := emitter.ConvertToFitnessSignals(results)
	if len(signals) != 5 {
		t.Fatalf("expected 5 signals, got %d", len(signals))
	}

	// 5. Build and serialize batch
	batch := emitter.BuildBatch(signals)
	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	outPath := filepath.Join(tmpDir, "signals.json")
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify file exists and is valid JSON
	readBack, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var decoded FitnessSignalBatch
	if err := json.Unmarshal(readBack, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Count != 5 {
		t.Errorf("expected 5 in batch, got %d", decoded.Count)
	}
}

func TestEndToEndLeaveOneOutPipeline(t *testing.T) {
	tmpDir := t.TempDir()

	// Baseline
	baseline := BenchmarkReport{
		OverallScore: 0.75,
		DomainScores: map[string]float64{"code": 0.80, "reasoning": 0.70},
	}
	writeJSON(t, filepath.Join(tmpDir, "baseline.json"), baseline)

	// Batch directory with LOO reports
	batchDir := filepath.Join(tmpDir, "loo")
	os.MkdirAll(batchDir, 0755)

	writeJSON(t, filepath.Join(batchDir, "batch_a.json"), LeaveOneOutReport{
		BenchmarkReport: BenchmarkReport{
			OverallScore: 0.70,
			DomainScores: map[string]float64{"code": 0.75, "reasoning": 0.65},
		},
		ExcludedBatch: "batch-a",
	})
	writeJSON(t, filepath.Join(batchDir, "batch_b.json"), LeaveOneOutReport{
		BenchmarkReport: BenchmarkReport{
			OverallScore: 0.78,
			DomainScores: map[string]float64{"code": 0.82, "reasoning": 0.74},
		},
		ExcludedBatch: "batch-b",
	})

	// Run analysis
	analyzer := NewAnalyzer()
	results, delta, err := analyzer.AnalyzeLeaveOneOut(
		filepath.Join(tmpDir, "baseline.json"),
		batchDir,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if delta == nil {
		t.Fatal("expected benchmark delta")
	}

	// Convert to signals
	emitter := NewSignalEmitter()
	signals := emitter.ConvertToFitnessSignals(results)
	if len(signals) != 2 {
		t.Fatalf("expected 2 aggregated signals, got %d", len(signals))
	}

	batch := emitter.BuildBatch(signals)
	if batch.Count != 2 {
		t.Errorf("expected batch count 2, got %d", batch.Count)
	}
}

// ─── Helper Functions ──────────────────────────────────────────────────────

func findResult(results []InfluenceResult, tduid string) *InfluenceResult {
	for _, r := range results {
		if r.TDUID == tduid {
			return &r
		}
	}
	return nil
}

func findResultByTDUAndBench(results []InfluenceResult, tduid, bench string) *InfluenceResult {
	for _, r := range results {
		if r.TDUID == tduid && r.Benchmark == bench {
			return &r
		}
	}
	return nil
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
