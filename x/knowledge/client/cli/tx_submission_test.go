package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Helper function tests ──────────────────────────────────────────────────

func TestParseTDUType(t *testing.T) {
	tests := []struct {
		input   string
		want    types.SampleType
		wantErr bool
	}{
		{"instruction-response", types.SampleType_SAMPLE_TYPE_Q_AND_A, false},
		{"instruction_response", types.SampleType_SAMPLE_TYPE_Q_AND_A, false},
		{"conversation", types.SampleType_SAMPLE_TYPE_DISCUSSION, false},
		{"correction", types.SampleType_SAMPLE_TYPE_CORRECTION, false},
		{"grounding-fact", types.SampleType_SAMPLE_TYPE_ANNOTATION, false},
		{"grounding_fact", types.SampleType_SAMPLE_TYPE_ANNOTATION, false},
		{"reasoning-chain", types.SampleType_SAMPLE_TYPE_EXPLANATION, false},
		{"reasoning_chain", types.SampleType_SAMPLE_TYPE_EXPLANATION, false},
		{"INSTRUCTION-RESPONSE", types.SampleType_SAMPLE_TYPE_Q_AND_A, false},
		{"unknown", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseTDUType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTDUType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseTDUType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDifficultyMultiplier(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"basic", 10, false},
		{"", 10, false},
		{"standard", 15, false},
		{"advanced", 20, false},
		{"expert", 30, false},
		{"frontier", 50, false},
		{"STANDARD", 15, false},
		{"unknown", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := difficultyMultiplier(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("difficultyMultiplier(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("difficultyMultiplier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCalculateStake(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		mult    int64
		want    string
		wantErr bool
	}{
		{"basic 1x", "1000000", 10, "1000000", false},
		{"standard 1.5x", "1000000", 15, "1500000", false},
		{"advanced 2x", "1000000", 20, "2000000", false},
		{"expert 3x", "1000000", 30, "3000000", false},
		{"frontier 5x", "1000000", 50, "5000000", false},
		{"large base", "10000000", 15, "15000000", false},
		{"invalid base", "abc", 10, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateStake(tt.base, tt.mult)
			if (err != nil) != tt.wantErr {
				t.Errorf("calculateStake(%q, %d) error = %v, wantErr %v", tt.base, tt.mult, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("calculateStake(%q, %d) = %q, want %q", tt.base, tt.mult, got, tt.want)
			}
		})
	}
}

func TestParseR41ConsentType(t *testing.T) {
	tests := []struct {
		input   string
		want    types.ConsentType
		wantErr bool
	}{
		{"original", types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, false},
		{"self", types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, false},
		{"public-domain", types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, false},
		{"public_domain", types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, false},
		{"licensed", types.ConsentType_CONSENT_TYPE_OPT_IN, false},
		// Falls through to legacy parser
		{"optin", types.ConsentType_CONSENT_TYPE_OPT_IN, false},
		{"fairuse", types.ConsentType_CONSENT_TYPE_FAIR_USE, false},
		{"unknown_junk", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseR41ConsentType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseR41ConsentType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseR41ConsentType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestContentHashHex(t *testing.T) {
	data := []byte(`{"instruction":"test","response":"hello"}`)
	h := sha256.Sum256(data)
	want := hex.EncodeToString(h[:])
	got := contentHashHex(data)
	if got != want {
		t.Errorf("contentHashHex() = %q, want %q", got, want)
	}
}

// ─── Content file schema tests ──────────────────────────────────────────────

func TestInstructionResponseSchema(t *testing.T) {
	content := map[string]string{
		"instruction":   "Write a Go function that reverses a string.",
		"response":      "```go\nfunc Reverse(s string) string { ... }\n```",
		"system_prompt": "You are a Go expert.",
	}
	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("instruction-response schema produced invalid JSON")
	}
	// Verify round-trip
	var parsed map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["instruction"] != content["instruction"] {
		t.Errorf("instruction mismatch")
	}
}

func TestConversationSchema(t *testing.T) {
	turns := []threadTurn{
		{Role: "user", Content: "How do I sort a slice in Go?"},
		{Role: "assistant", Content: "Use sort.Slice(s, less) from the sort package."},
		{Role: "user", Content: "Can you show an example?"},
		{Role: "assistant", Content: "sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })"},
	}
	data, err := json.Marshal(turns)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("conversation schema produced invalid JSON")
	}
	var parsed []threadTurn
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed) != 4 {
		t.Errorf("expected 4 turns, got %d", len(parsed))
	}
}

func TestCorrectionSchema(t *testing.T) {
	correction := map[string]string{
		"original_id": "abc123",
		"field":       "response",
		"corrected":   "The correct approach is to use sort.SliceStable.",
		"explanation": "sort.Slice is not stable; use sort.SliceStable for deterministic ordering.",
	}
	data, err := json.Marshal(correction)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("correction schema produced invalid JSON")
	}
}

// ─── CLI command construction tests ─────────────────────────────────────────

func TestSubmitDataCmdExists(t *testing.T) {
	cmd := NewSubmitDataCmd()
	if cmd.Use != "submit-data" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	// Verify R41 flags exist
	for _, flag := range []string{"type", "domain", "difficulty", "content-file", "consent-proof", "metadata", "stake"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing flag --%s", flag)
		}
	}
	// Verify legacy flags still exist
	for _, flag := range []string{"sample-type", "source-uri", "consent-type", "tags", "language"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing legacy flag --%s", flag)
		}
	}
}

func TestSubmitThreadCmdExists(t *testing.T) {
	cmd := NewSubmitThreadCmd()
	if cmd.Use != "submit-thread" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	for _, flag := range []string{"thread-file", "domain", "difficulty", "stake", "consent-type", "thread-id"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing flag --%s", flag)
		}
	}
}

func TestSubmitCorrectionCmdExists(t *testing.T) {
	cmd := NewSubmitCorrectionCmd()
	if cmd.Use != "submit-correction" {
		t.Errorf("unexpected Use: %s", cmd.Use)
	}

	for _, flag := range []string{"target-id", "correction-file", "reason", "domain", "difficulty", "stake", "consent-type"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing flag --%s", flag)
		}
	}
}

func TestGetTxCmdHasSubmitCorrection(t *testing.T) {
	txCmd := GetTxCmd()
	found := false
	for _, sub := range txCmd.Commands() {
		if sub.Use == "submit-correction" {
			found = true
			break
		}
	}
	if !found {
		t.Error("submit-correction not found in GetTxCmd")
	}
}

// ─── File I/O integration tests ─────────────────────────────────────────────

func TestContentFileSizeLimit(t *testing.T) {
	// Create a temp file larger than 1MB
	dir := t.TempDir()
	bigFile := filepath.Join(dir, "big.json")
	data := make([]byte, maxContentFileSize+1)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(bigFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Verify the size check constant
	if maxContentFileSize != 1_048_576 {
		t.Errorf("maxContentFileSize = %d, want 1048576", maxContentFileSize)
	}
}

func TestThreadFileParsingRoundTrip(t *testing.T) {
	turns := []threadTurn{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	dir := t.TempDir()
	f := filepath.Join(dir, "thread.json")
	data, _ := json.Marshal(turns)
	if err := os.WriteFile(f, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Read back and verify
	readBack, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	var parsed []threadTurn
	if err := json.Unmarshal(readBack, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("expected 2 turns, got %d", len(parsed))
	}
	if parsed[0].Role != "user" || parsed[1].Role != "assistant" {
		t.Error("roles don't match")
	}
}
