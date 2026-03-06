package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/zerone-chain/zerone/services/benchmark/domains"
)

// ---------- Dataset Loading Tests ----------

func TestLoadAllDatasets(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")
	cases, err := domains.LoadAll(datasetDir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("expected non-empty cases")
	}

	// Check minimum counts per domain
	counts := map[string]int{}
	for _, c := range cases {
		counts[c.Domain]++
	}
	if counts["code"] < 50 {
		t.Errorf("code cases: got %d, want >= 50", counts["code"])
	}
	if counts["reasoning"] < 30 {
		t.Errorf("reasoning cases: got %d, want >= 30", counts["reasoning"])
	}
	if counts["instruction"] < 20 {
		t.Errorf("instruction cases: got %d, want >= 20", counts["instruction"])
	}
}

func TestLoadDomainCode(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")
	cases, err := domains.LoadDomain(datasetDir, "code")
	if err != nil {
		t.Fatalf("LoadDomain(code): %v", err)
	}
	for _, c := range cases {
		if c.Domain != "code" {
			t.Errorf("case %s: domain=%q, want code", c.ID, c.Domain)
		}
		if c.ID == "" {
			t.Error("case has empty ID")
		}
		if c.Prompt == "" {
			t.Error("case has empty Prompt")
		}
		if c.EvalMethod == "" {
			t.Error("case has empty EvalMethod")
		}
	}
}

func TestLoadDomainReasoning(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")
	cases, err := domains.LoadDomain(datasetDir, "reasoning")
	if err != nil {
		t.Fatalf("LoadDomain(reasoning): %v", err)
	}
	for _, c := range cases {
		if c.Domain != "reasoning" {
			t.Errorf("case %s: domain=%q, want reasoning", c.ID, c.Domain)
		}
	}
}

func TestLoadDomainInstruction(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")
	cases, err := domains.LoadDomain(datasetDir, "instruction")
	if err != nil {
		t.Fatalf("LoadDomain(instruction): %v", err)
	}
	for _, c := range cases {
		if c.Domain != "instruction" {
			t.Errorf("case %s: domain=%q, want instruction", c.ID, c.Domain)
		}
	}
}

func TestLoadDomainUnknown(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")
	_, err := domains.LoadDomain(datasetDir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown domain")
	}
}

// ---------- Evaluator Tests ----------

func TestExactMatch(t *testing.T) {
	eval := NewEvaluator("", "", 0)

	tests := []struct {
		name     string
		response string
		expected string
		wantHigh bool
	}{
		{"identical", "42", "42", true},
		{"with_whitespace", "  42\n", "42", true},
		{"case_insensitive", "Hello", "hello", true},
		{"contains", "The answer is 42.", "42", false}, // 0.5 score
		{"mismatch", "43", "42", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := eval.ExactMatch(tt.response, tt.expected)
			if tt.wantHigh && score < 0.9 {
				t.Errorf("score=%.2f, want >= 0.9", score)
			}
			if !tt.wantHigh && tt.name == "mismatch" && score > 0.1 {
				t.Errorf("score=%.2f, want <= 0.1", score)
			}
		})
	}
}

func TestFuzzyMatch(t *testing.T) {
	eval := NewEvaluator("", "", 0)

	tests := []struct {
		name     string
		response string
		expected string
		minScore float64
	}{
		{"exact_substring", "The fix is to use for i := 0; i < len(nums); i++", "for i := 0; i < len(nums); i++", 1.0},
		{"contains_key", "You should use defer resp.Body.Close() to fix the leak", "defer resp.Body.Close()", 1.0},
		{"partial_match", "race conditions are a problem", "race condition", 1.0},
		{"no_match", "completely unrelated text", "SQL injection vulnerability", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := eval.FuzzyMatch(tt.response, tt.expected)
			if score < tt.minScore {
				t.Errorf("score=%.2f, want >= %.2f", score, tt.minScore)
			}
		})
	}
}

func TestStructuralCheck(t *testing.T) {
	eval := NewEvaluator("", "", 0)

	tests := []struct {
		name     string
		response string
		expected string
		metadata map[string]any
		minScore float64
	}{
		{
			name:     "valid_json",
			response: `{"name": "test", "age": 25, "active": true}`,
			expected: `{"name":`,
			metadata: map[string]any{"format": "json"},
			minScore: 0.5,
		},
		{
			name:     "valid_json_array",
			response: `["red", "green", "blue"]`,
			expected: "[",
			metadata: map[string]any{"format": "json_array"},
			minScore: 0.5,
		},
		{
			name:     "markdown_table",
			response: "| Name | Type |\n| --- | --- |\n| GET | read |",
			expected: "|",
			metadata: map[string]any{"format": "markdown_table"},
			minScore: 0.5,
		},
		{
			name:     "bullets",
			response: "- First point\n- Second point\n- Third point",
			expected: "- ",
			metadata: map[string]any{"format": "bullets"},
			minScore: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, err := eval.structuralCheck(tt.response, tt.expected, tt.metadata)
			if err != nil {
				t.Fatalf("structuralCheck: %v", err)
			}
			if score < tt.minScore {
				t.Errorf("score=%.2f, want >= %.2f", score, tt.minScore)
			}
		})
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "axc", 1},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshtein(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseLLMJudgeResponse(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		wantScore float64
		wantErr   bool
	}{
		{
			name:      "clean_json",
			response:  `{"score": 0.85, "reason": "good response"}`,
			wantScore: 0.85,
		},
		{
			name:      "json_in_text",
			response:  `Here is my evaluation: {"score": 0.7, "reason": "mostly correct"}`,
			wantScore: 0.7,
		},
		{
			name:      "invalid_json",
			response:  "this is not json",
			wantScore: 0.5, // default
		},
		{
			name:      "clamped_high",
			response:  `{"score": 1.5, "reason": "over"}`,
			wantScore: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _, err := parseLLMJudgeResponse(tt.response)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if score != tt.wantScore {
				t.Errorf("score=%.2f, want=%.2f", score, tt.wantScore)
			}
		})
	}
}

// ---------- Reporter Tests ----------

func TestReporterAggregate(t *testing.T) {
	results := []domains.CaseResult{
		{CaseID: "c1", Domain: "code", Category: "impl", Score: 1.0, Pass: true},
		{CaseID: "c2", Domain: "code", Category: "impl", Score: 0.5, Pass: true},
		{CaseID: "c3", Domain: "code", Category: "bug", Score: 0.0, Pass: false},
		{CaseID: "r1", Domain: "reasoning", Category: "math", Score: 1.0, Pass: true},
		{CaseID: "r2", Domain: "reasoning", Category: "math", Score: 1.0, Pass: true},
	}

	reporter := NewReporter()
	report := reporter.Aggregate(results, "test-model", "v1")

	if report.ModelVersion != "test-model" {
		t.Errorf("model=%q, want test-model", report.ModelVersion)
	}

	// Code: (1.0+0.5+0.0)/3 = 0.5
	if report.DomainScores["code"] != 0.5 {
		t.Errorf("code score=%.2f, want 0.50", report.DomainScores["code"])
	}

	// Reasoning: (1.0+1.0)/2 = 1.0
	if report.DomainScores["reasoning"] != 1.0 {
		t.Errorf("reasoning score=%.2f, want 1.00", report.DomainScores["reasoning"])
	}

	// Overall: (1.0+0.5+0.0+1.0+1.0)/5 = 0.7
	if report.OverallScore != 0.7 {
		t.Errorf("overall=%.2f, want 0.70", report.OverallScore)
	}

	if len(report.PerCaseResults) != 5 {
		t.Errorf("results count=%d, want 5", len(report.PerCaseResults))
	}
}

func TestReporterWriteAndLoadJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-report.json")

	reporter := NewReporter()
	original := &domains.BenchmarkReport{
		ModelVersion:   "v1",
		DatasetVersion: "d1",
		Timestamp:      "2026-03-06T00:00:00Z",
		OverallScore:   0.75,
		DomainScores:   map[string]float64{"code": 0.8, "reasoning": 0.7},
		PerCaseResults: []domains.CaseResult{
			{CaseID: "c1", Score: 0.8, Pass: true},
		},
	}

	if err := reporter.WriteJSON(original, path); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	loaded, err := reporter.LoadReport(path)
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}

	if loaded.ModelVersion != original.ModelVersion {
		t.Errorf("model=%q, want %q", loaded.ModelVersion, original.ModelVersion)
	}
	if loaded.OverallScore != original.OverallScore {
		t.Errorf("overall=%.2f, want %.2f", loaded.OverallScore, original.OverallScore)
	}
	if loaded.DomainScores["code"] != 0.8 {
		t.Errorf("code=%.2f, want 0.80", loaded.DomainScores["code"])
	}
}

func TestReporterCompare(t *testing.T) {
	reporter := NewReporter()

	baseline := &domains.BenchmarkReport{
		ModelVersion: "v1.1",
		DomainScores: map[string]float64{"code": 0.7, "reasoning": 0.6},
		OverallScore: 0.65,
		PerCaseResults: []domains.CaseResult{
			{CaseID: "c1", Score: 0.8},
			{CaseID: "c2", Score: 0.6},
		},
	}
	candidate := &domains.BenchmarkReport{
		ModelVersion: "v1.2",
		DomainScores: map[string]float64{"code": 0.8, "reasoning": 0.5},
		OverallScore: 0.65,
		PerCaseResults: []domains.CaseResult{
			{CaseID: "c1", Score: 0.9},
			{CaseID: "c2", Score: 0.4},
		},
	}

	comparison := reporter.Compare(baseline, candidate)
	if comparison == "" {
		t.Fatal("expected non-empty comparison")
	}

	// Should mention both model versions
	if !contains(comparison, "v1.1") || !contains(comparison, "v1.2") {
		t.Error("comparison should mention both model versions")
	}

	// Should detect regressions (reasoning dropped)
	if !contains(comparison, "Regression") && !contains(comparison, "regression") {
		t.Error("comparison should detect regression in reasoning")
	}
}

func TestCompareDetectsRegressions(t *testing.T) {
	reporter := NewReporter()

	baseline := &domains.BenchmarkReport{
		ModelVersion: "v1",
		DomainScores: map[string]float64{"code": 0.9},
		OverallScore: 0.9,
		PerCaseResults: []domains.CaseResult{
			{CaseID: "c1", Score: 1.0},
			{CaseID: "c2", Score: 0.8},
		},
	}
	candidate := &domains.BenchmarkReport{
		ModelVersion: "v2",
		DomainScores: map[string]float64{"code": 0.5},
		OverallScore: 0.5,
		PerCaseResults: []domains.CaseResult{
			{CaseID: "c1", Score: 0.3},
			{CaseID: "c2", Score: 0.7},
		},
	}

	comparison := reporter.Compare(baseline, candidate)
	if !contains(comparison, "WARNING") {
		t.Error("should warn about regressions")
	}
	if !contains(comparison, "c1") {
		t.Error("should list regressed case c1")
	}
}

// ---------- Runner Tests ----------

func TestRunnerCollectsResults(t *testing.T) {
	// This tests that the runner processes all cases and returns results.
	// We use a mock by providing an endpoint that won't connect—
	// the runner should gracefully handle endpoint errors.
	runner := NewRunner("http://127.0.0.1:1", "test", 1, 1) // unreachable endpoint

	cases := []domains.BenchCase{
		{ID: "t1", Domain: "test", Category: "unit", Prompt: "hello", Expected: "world", EvalMethod: "exact"},
		{ID: "t2", Domain: "test", Category: "unit", Prompt: "foo", Expected: "bar", EvalMethod: "exact"},
	}

	report, err := runner.Run(cases, "test-v1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(report.PerCaseResults) != 2 {
		t.Errorf("got %d results, want 2", len(report.PerCaseResults))
	}

	// All should fail (endpoint unreachable)
	for _, res := range report.PerCaseResults {
		if res.Pass {
			t.Errorf("case %s passed but endpoint was unreachable", res.CaseID)
		}
	}
}

func TestRunnerFiltersDomain(t *testing.T) {
	runner := NewRunner("http://127.0.0.1:1", "test", 1, 1)

	cases := []domains.BenchCase{
		{ID: "c1", Domain: "code", Prompt: "test", Expected: "x", EvalMethod: "exact"},
		{ID: "r1", Domain: "reasoning", Prompt: "test", Expected: "x", EvalMethod: "exact"},
	}

	results, err := runner.RunDomain("code", cases)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
	if results[0].CaseID != "c1" {
		t.Errorf("got case %q, want c1", results[0].CaseID)
	}
}

func TestRunnerDomainNotFound(t *testing.T) {
	runner := NewRunner("http://127.0.0.1:1", "test", 1, 1)

	_, err := runner.RunDomain("nonexistent", []domains.BenchCase{
		{ID: "c1", Domain: "code", Prompt: "test", Expected: "x", EvalMethod: "exact"},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent domain")
	}
}

// ---------- Domain Helper Tests ----------

func TestExtractCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "go_fenced",
			input:    "Here's the code:\n```go\nfunc foo() int {\n\treturn 42\n}\n```\nDone.",
			expected: "func foo() int {\n\treturn 42\n}",
		},
		{
			name:     "plain_fenced",
			input:    "```\nfunc bar() {}\n```",
			expected: "func bar() {}",
		},
		{
			name:     "no_fence",
			input:    "func baz() {}",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domains.ExtractCodeBlock(tt.input)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidateGoSyntax(t *testing.T) {
	valid := "func foo() { return }"
	if !domains.ValidateGoSyntax(valid) {
		t.Error("expected valid syntax")
	}

	invalid := "func foo() { return"
	if domains.ValidateGoSyntax(invalid) {
		t.Error("expected invalid syntax (unbalanced braces)")
	}

	noFunc := "var x = 1"
	if domains.ValidateGoSyntax(noFunc) {
		t.Error("expected invalid (no func keyword)")
	}
}

func TestNormalizeAnswer(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"  42  ", "42"},
		{"YES.", "yes"},
		{"Hello, World!", "hello, world"},
	}

	for _, tt := range tests {
		got := domains.NormalizeAnswer(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeAnswer(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractNumber(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"42", "42"},
		{"The answer is 42.", "42"},
		{"Results: 3.14", "3.14"},
		{"no numbers here", "no numbers here"},
	}

	for _, tt := range tests {
		got := domains.ExtractNumber(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractNumber(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCheckJSON(t *testing.T) {
	if !domains.CheckJSON(`{"a":1}`) {
		t.Error("expected valid JSON")
	}
	if domains.CheckJSON(`{invalid}`) {
		t.Error("expected invalid JSON")
	}
}

func TestCheckJSONKeys(t *testing.T) {
	if !domains.CheckJSONKeys(`{"name":"a","age":1}`, []string{"name", "age"}) {
		t.Error("expected keys found")
	}
	if domains.CheckJSONKeys(`{"name":"a"}`, []string{"name", "age"}) {
		t.Error("expected missing key")
	}
}

func TestCheckWordCount(t *testing.T) {
	if !domains.CheckWordCount("one two three", 1, 5) {
		t.Error("expected within range")
	}
	if domains.CheckWordCount("one two three", 5, 10) {
		t.Error("expected below range")
	}
}

func TestCheckBulletCount(t *testing.T) {
	text := "- one\n- two\n- three"
	if got := domains.CheckBulletCount(text); got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}

// ---------- Dataset Integrity Tests ----------

func TestAllCasesHaveUniqueIDs(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")
	cases, err := domains.LoadAll(datasetDir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	seen := make(map[string]bool)
	for _, c := range cases {
		if seen[c.ID] {
			t.Errorf("duplicate case ID: %s", c.ID)
		}
		seen[c.ID] = true
	}
}

func TestAllCasesHaveValidEvalMethod(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")
	cases, err := domains.LoadAll(datasetDir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	validMethods := map[string]bool{
		"exact":     true,
		"fuzzy":     true,
		"execution": true,
		"llm_judge": true,
	}

	for _, c := range cases {
		if !validMethods[c.EvalMethod] {
			t.Errorf("case %s has invalid eval_method: %q", c.ID, c.EvalMethod)
		}
	}
}

func TestDatasetJSONValidity(t *testing.T) {
	datasetDir := filepath.Join(testDatasetDir(t), "datasets")

	files := []string{"code_bench.json", "reasoning_bench.json", "instruct_bench.json"}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(datasetDir, f))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if !json.Valid(data) {
				t.Fatalf("invalid JSON in %s", f)
			}
		})
	}
}

// ---------- Helpers ----------

func testDatasetDir(t *testing.T) string {
	t.Helper()
	// Walk up to find the benchmark service root (where datasets/ lives)
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Verify datasets directory exists
	if _, err := os.Stat(filepath.Join(dir, "datasets")); err != nil {
		t.Skipf("datasets not found in %s: %v", dir, err)
	}
	return dir
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
			(containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
