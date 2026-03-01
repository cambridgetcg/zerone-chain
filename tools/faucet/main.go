// Package main implements an HTTP faucet server for the ZERONE chain.
//
// It distributes testnet tokens to valid zrn1... addresses with per-address
// rate limiting, atomic state persistence, and soft-cap enforcement.
//
// Configuration is via environment variables:
//
//	FAUCET_HOME       (required) zeroned home directory
//	FAUCET_CHAIN_ID   (required) chain ID for tx signing
//	FAUCET_AMOUNT     tokens per request in uzrn (default 100000000)
//	FAUCET_COOLDOWN   cooldown hours per address (default 24)
//	FAUCET_PORT       listen port (default 8080)
//	FAUCET_KEYRING_BACKEND keyring backend (default test)
//	FAUCET_FROM       signing key name (default faucet)
//	FAUCET_NODE       CometBFT RPC endpoint (default tcp://localhost:26657)
//	FAUCET_STATE_FILE state persistence path (default faucet-state.json)
//	FAUCET_MAX_TOTAL  lifetime distribution cap in uzrn (default 10000000000000)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// Config holds all faucet runtime settings, populated from environment variables.
type Config struct {
	Amount         int64
	CooldownHours  int
	Port           string
	KeyringBackend string
	From           string
	Node           string
	Home           string
	ChainID        string
	StateFile      string
	MaxTotal       int64
}

// loadConfig reads configuration from environment variables. FAUCET_HOME and
// FAUCET_CHAIN_ID are required; all others have sensible defaults.
func loadConfig() Config {
	home := os.Getenv("FAUCET_HOME")
	if home == "" {
		log.Fatal("FAUCET_HOME is required")
	}
	chainID := os.Getenv("FAUCET_CHAIN_ID")
	if chainID == "" {
		log.Fatal("FAUCET_CHAIN_ID is required")
	}

	cfg := Config{
		Amount:         100000000,
		CooldownHours:  24,
		Port:           "8080",
		KeyringBackend: "test",
		From:           "faucet",
		Node:           "tcp://localhost:26657",
		Home:           home,
		ChainID:        chainID,
		StateFile:      "faucet-state.json",
		MaxTotal:       10000000000000,
	}

	if v := os.Getenv("FAUCET_AMOUNT"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil && n > 0 {
			cfg.Amount = n
		}
	}
	if v := os.Getenv("FAUCET_COOLDOWN"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			cfg.CooldownHours = n
		}
	}
	if v := os.Getenv("FAUCET_PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("FAUCET_KEYRING_BACKEND"); v != "" {
		cfg.KeyringBackend = v
	}
	if v := os.Getenv("FAUCET_FROM"); v != "" {
		cfg.From = v
	}
	if v := os.Getenv("FAUCET_NODE"); v != "" {
		cfg.Node = v
	}
	if v := os.Getenv("FAUCET_STATE_FILE"); v != "" {
		cfg.StateFile = v
	}
	if v := os.Getenv("FAUCET_MAX_TOTAL"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil && n > 0 {
			cfg.MaxTotal = n
		}
	}

	return cfg
}

// ---------------------------------------------------------------------------
// Persistent state
// ---------------------------------------------------------------------------

// State tracks lifetime distribution totals and per-address request timestamps
// for rate-limiting. It is serialised to JSON on disk after every successful send.
type State struct {
	TotalDistributed int64             `json:"total_distributed"`
	Requests         map[string]string `json:"requests"` // address → RFC 3339 timestamp
}

// loadState reads persisted state from path. If the file is missing or corrupt
// it returns a fresh empty state.
func loadState(path string) State {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{Requests: make(map[string]string)}
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{Requests: make(map[string]string)}
	}
	if s.Requests == nil {
		s.Requests = make(map[string]string)
	}
	return s
}

// ---------------------------------------------------------------------------
// Faucet core
// ---------------------------------------------------------------------------

// Faucet ties together configuration, mutable state, and a mutex that
// serialises token sends to prevent double-spends.
type Faucet struct {
	cfg   Config
	state State
	mu    sync.Mutex
}

// saveState atomically persists the current state to disk by writing to a
// temporary file first, then renaming.
func (f *Faucet) saveState() error {
	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp := f.cfg.StateFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp state: %w", err)
	}
	if err := os.Rename(tmp, f.cfg.StateFile); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}

// sendTokens shells out to zeroned to broadcast a bank send transaction.
// It returns the tx hash on success.
func (f *Faucet) sendTokens(toAddr string, amount int64) (string, error) {
	amountStr := fmt.Sprintf("%duzrn", amount)

	// #nosec G204 — arguments are validated before reaching here
	cmd := exec.Command("zeroned", "tx", "bank", "send",
		f.cfg.From, toAddr, amountStr,
		"--keyring-backend", f.cfg.KeyringBackend,
		"--home", f.cfg.Home,
		"--chain-id", f.cfg.ChainID,
		"--node", f.cfg.Node,
		"--broadcast-mode", "sync",
		"--output", "json",
		"--yes",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("zeroned tx failed: %w: %s", err, string(out))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("parse tx output: %w: %s", err, string(out))
	}

	// Check for non-zero code (SDK-level error).
	if code, ok := result["code"].(float64); ok && code != 0 {
		rawLog, _ := result["raw_log"].(string)
		return "", fmt.Errorf("tx error code %d: %s", int(code), rawLog)
	}

	txHash, _ := result["txhash"].(string)
	if txHash == "" {
		return "", fmt.Errorf("no txhash in response: %s", string(out))
	}
	return txHash, nil
}

// ---------------------------------------------------------------------------
// Address validation
// ---------------------------------------------------------------------------

// isValidAddress checks that addr looks like a valid bech32 zrn1... address.
// It does not perform full bech32 checksum verification.
func isValidAddress(addr string) bool {
	if !strings.HasPrefix(addr, "zrn1") {
		return false
	}
	if len(addr) < 39 || len(addr) > 59 {
		return false
	}
	for _, c := range addr[len("zrn1"):] {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// HTTP types
// ---------------------------------------------------------------------------

// FaucetRequest is the JSON body expected by POST /faucet.
type FaucetRequest struct {
	Address string `json:"address"`
}

// FaucetResponse is the JSON body returned by POST /faucet.
type FaucetResponse struct {
	Status     string `json:"status"`
	TxHash     string `json:"tx_hash,omitempty"`
	Amount     string `json:"amount,omitempty"`
	Error      string `json:"error,omitempty"`
	RetryAfter string `json:"retry_after,omitempty"`
}

// ---------------------------------------------------------------------------
// CORS middleware
// ---------------------------------------------------------------------------

// cors wraps an http.HandlerFunc with permissive CORS headers suitable for a
// public testnet faucet.
func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (f *Faucet) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (f *Faucet) handleStats(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	totalDist := f.state.TotalDistributed
	uniqueAddrs := len(f.state.Requests)
	f.mu.Unlock()

	remaining := f.cfg.MaxTotal - totalDist
	if remaining < 0 {
		remaining = 0
	}

	resp := map[string]interface{}{
		"total_distributed_uzrn":  totalDist,
		"unique_addresses":        uniqueAddrs,
		"remaining_uzrn":          remaining,
		"amount_per_request_uzrn": f.cfg.Amount,
		"cooldown_hours":          f.cfg.CooldownHours,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (f *Faucet) handleFaucet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Reject non-POST.
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(FaucetResponse{
			Status: "error",
			Error:  "method not allowed",
		})
		return
	}

	// 2. Parse JSON body (limit to 4KB to prevent abuse).
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req FaucetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(FaucetResponse{
			Status: "error",
			Error:  "invalid JSON body",
		})
		return
	}

	// 3. Trim and validate address.
	req.Address = strings.TrimSpace(req.Address)
	if !isValidAddress(req.Address) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(FaucetResponse{
			Status: "error",
			Error:  "invalid address: must be zrn1... bech32",
		})
		return
	}

	// 4. Acquire mutex — cap check, rate limit, send, and persist are all atomic.
	f.mu.Lock()
	defer f.mu.Unlock()

	// 5. Cap check (inside mutex — no data race).
	if f.state.TotalDistributed >= f.cfg.MaxTotal {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(FaucetResponse{
			Status: "error",
			Error:  "faucet supply exhausted",
		})
		return
	}

	// 6. Rate-limit check (inside mutex — TOCTOU prevention).
	if lastStr, ok := f.state.Requests[req.Address]; ok {
		lastTime, err := time.Parse(time.RFC3339, lastStr)
		if err == nil {
			cooldown := time.Duration(f.cfg.CooldownHours) * time.Hour
			retryAfter := lastTime.Add(cooldown)
			if time.Now().Before(retryAfter) {
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(FaucetResponse{
					Status:     "error",
					Error:      "rate limited",
					RetryAfter: retryAfter.UTC().Format(time.RFC3339),
				})
				return
			}
		}
	}

	// 7. Send tokens.
	txHash, err := f.sendTokens(req.Address, f.cfg.Amount)
	if err != nil {
		log.Printf("sendTokens failed for %s: %v", req.Address, err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(FaucetResponse{
			Status: "error",
			Error:  fmt.Sprintf("tx broadcast failed: %v", err),
		})
		return
	}

	// 8. Update state.
	f.state.TotalDistributed += f.cfg.Amount
	f.state.Requests[req.Address] = time.Now().UTC().Format(time.RFC3339)

	// 9. Persist state (atomic write).
	if err := f.saveState(); err != nil {
		log.Printf("WARNING: failed to persist state: %v", err)
	}

	// 10 & 11. Log and return success (mutex released by defer).
	amountStr := fmt.Sprintf("%duzrn", f.cfg.Amount)
	log.Printf("Sent %s to %s (tx=%s, total=%d uzrn)", amountStr, req.Address, txHash, f.state.TotalDistributed)
	_ = json.NewEncoder(w).Encode(FaucetResponse{
		Status: "ok",
		TxHash: txHash,
		Amount: amountStr,
	})
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	cfg := loadConfig()
	state := loadState(cfg.StateFile)

	f := &Faucet{
		cfg:   cfg,
		state: state,
	}

	http.HandleFunc("/health", cors(f.handleHealth))
	http.HandleFunc("/stats", cors(f.handleStats))
	http.HandleFunc("/faucet", cors(f.handleFaucet))

	log.Printf("ZERONE Faucet starting on :%s", cfg.Port)
	log.Printf("  chain-id=%s home=%s from=%s node=%s", cfg.ChainID, cfg.Home, cfg.From, cfg.Node)
	log.Printf("  amount=%d uzrn  cooldown=%dh  max_total=%d uzrn", cfg.Amount, cfg.CooldownHours, cfg.MaxTotal)

	addr := ":" + cfg.Port
	log.Fatal(http.ListenAndServe(addr, nil))
}
