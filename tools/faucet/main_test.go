package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// TestIsValidAddress — table-driven
// ---------------------------------------------------------------------------

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{
			name: "valid address",
			addr: "zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu",
			want: true,
		},
		{
			name: "too short",
			addr: "zrn1abc",
			want: false,
		},
		{
			name: "wrong prefix",
			addr: "cosmos1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu",
			want: false,
		},
		{
			name: "empty",
			addr: "",
			want: false,
		},
		{
			name: "uppercase chars",
			addr: "zrn1QYPQXPQ9QCRSSZG2PVXQ6RS0ZQG3YYC5LZV7XU",
			want: false,
		},
		{
			name: "special chars",
			addr: "zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5!@#$xu",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidAddress(tc.addr)
			if got != tc.want {
				t.Errorf("isValidAddress(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helper: create a Faucet with sensible test defaults (no real chain needed).
// ---------------------------------------------------------------------------

func newTestFaucet() *Faucet {
	return &Faucet{
		cfg: Config{
			Amount:        100000000,
			CooldownHours: 24,
			MaxTotal:      10000000000000,
		},
		state: State{
			Requests: make(map[string]string),
		},
	}
}

// ---------------------------------------------------------------------------
// TestHealthEndpoint
// ---------------------------------------------------------------------------

func TestHealthEndpoint(t *testing.T) {
	f := newTestFaucet()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	f.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

// ---------------------------------------------------------------------------
// TestStatsEndpoint
// ---------------------------------------------------------------------------

func TestStatsEndpoint(t *testing.T) {
	f := newTestFaucet()
	// Seed some state so the numbers are non-trivial.
	f.state.TotalDistributed = 500000000
	f.state.Requests["zrn1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0000"] = time.Now().UTC().Format(time.RFC3339)
	f.state.Requests["zrn1bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb1111"] = time.Now().UTC().Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	f.handleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	assertJSONFloat := func(key string, want float64) {
		t.Helper()
		got, ok := stats[key].(float64)
		if !ok {
			t.Fatalf("key %q missing or not a number", key)
		}
		if got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	assertJSONFloat("total_distributed_uzrn", 500000000)
	assertJSONFloat("unique_addresses", 2)
	assertJSONFloat("remaining_uzrn", float64(f.cfg.MaxTotal-500000000))
	assertJSONFloat("amount_per_request_uzrn", float64(f.cfg.Amount))
	assertJSONFloat("cooldown_hours", float64(f.cfg.CooldownHours))
}

// ---------------------------------------------------------------------------
// TestFaucetBadMethod — GET /faucet → 405
// ---------------------------------------------------------------------------

func TestFaucetBadMethod(t *testing.T) {
	f := newTestFaucet()

	req := httptest.NewRequest(http.MethodGet, "/faucet", nil)
	rec := httptest.NewRecorder()

	f.handleFaucet(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}

	var resp FaucetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
}

// ---------------------------------------------------------------------------
// TestFaucetInvalidAddress — POST with bad address → 400
// ---------------------------------------------------------------------------

func TestFaucetInvalidAddress(t *testing.T) {
	f := newTestFaucet()

	body := `{"address":"cosmos1invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	f.handleFaucet(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var resp FaucetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if !strings.Contains(resp.Error, "invalid address") {
		t.Errorf("expected error to mention 'invalid address', got %q", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// TestFaucetDepleted — TotalDistributed >= MaxTotal → 503
// ---------------------------------------------------------------------------

func TestFaucetDepleted(t *testing.T) {
	f := newTestFaucet()
	f.state.TotalDistributed = f.cfg.MaxTotal // exhausted

	body := `{"address":"zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu"}`
	req := httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	f.handleFaucet(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var resp FaucetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if !strings.Contains(resp.Error, "exhausted") {
		t.Errorf("expected error to mention 'exhausted', got %q", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// TestFaucetRateLimited — address served recently → 429
// ---------------------------------------------------------------------------

func TestFaucetRateLimited(t *testing.T) {
	f := newTestFaucet()

	addr := "zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu"
	// Record a request from 1 minute ago — well within the 24h cooldown.
	f.state.Requests[addr] = time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339)

	body := `{"address":"` + addr + `"}`
	req := httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	f.handleFaucet(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", rec.Code)
	}

	var resp FaucetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got %q", resp.Status)
	}
	if !strings.Contains(resp.Error, "rate limited") {
		t.Errorf("expected error to mention 'rate limited', got %q", resp.Error)
	}
	if resp.RetryAfter == "" {
		t.Error("expected retry_after to be set")
	}
}

// ---------------------------------------------------------------------------
// TestSaveLoadState — round-trip persistence
// ---------------------------------------------------------------------------

func TestSaveLoadState(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")

	f := &Faucet{
		cfg: Config{
			StateFile: stateFile,
		},
		state: State{
			TotalDistributed: 42000000,
			Requests: map[string]string{
				"zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu": "2026-02-27T12:00:00Z",
				"zrn1aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0000": "2026-02-27T13:00:00Z",
			},
		},
	}

	// Save.
	if err := f.saveState(); err != nil {
		t.Fatalf("saveState failed: %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("state file not found: %v", err)
	}

	// Load back.
	loaded := loadState(stateFile)

	if loaded.TotalDistributed != f.state.TotalDistributed {
		t.Errorf("TotalDistributed = %d, want %d", loaded.TotalDistributed, f.state.TotalDistributed)
	}
	if len(loaded.Requests) != len(f.state.Requests) {
		t.Fatalf("Requests count = %d, want %d", len(loaded.Requests), len(f.state.Requests))
	}
	for addr, ts := range f.state.Requests {
		got, ok := loaded.Requests[addr]
		if !ok {
			t.Errorf("address %q missing in loaded state", addr)
			continue
		}
		if got != ts {
			t.Errorf("timestamp for %q = %q, want %q", addr, got, ts)
		}
	}
}
