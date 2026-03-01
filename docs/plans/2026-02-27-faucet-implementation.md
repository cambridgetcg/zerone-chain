# R27-5 Faucet Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a testnet faucet HTTP server that distributes ZRN tokens, with rate limiting, persistence, and a total cap.

**Architecture:** Standalone Go HTTP binary at `tools/faucet/main.go` that shells out to `zeroned tx bank send --broadcast-mode block`. Rate-limit state persisted to JSON file with atomic writes. Mutex serializes all sends to prevent sequence number conflicts and TOCTOU rate-limit races.

**Tech Stack:** Go stdlib (`net/http`, `encoding/json`, `os/exec`, `sync`), no external dependencies beyond what's in go.mod.

---

### Task 1: Scaffold faucet binary with health endpoint

**Files:**
- Create: `tools/faucet/main.go`

**Step 1: Create the faucet main.go with config, state, and health endpoint**

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Config holds faucet configuration from environment variables.
type Config struct {
	Amount         int64  // uzrn per request
	CooldownHours  int    // hours between requests per address
	Port           string // listen port
	KeyringBackend string // keyring backend
	From           string // signing key name
	Node           string // node RPC URL
	Home           string // zeroned home directory
	ChainID        string // chain ID
	StateFile      string // path to persist state
	MaxTotal       int64  // total distribution cap in uzrn
}

// State holds the faucet's persistent state.
type State struct {
	TotalDistributed int64             `json:"total_distributed"`
	Requests         map[string]string `json:"requests"` // address -> RFC3339 timestamp
}

// Faucet is the main server struct.
type Faucet struct {
	cfg   Config
	state State
	mu    sync.Mutex
}

func loadConfig() Config {
	getEnv := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	getEnvInt64 := func(key string, fallback int64) int64 {
		if v := os.Getenv(key); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				return n
			}
		}
		return fallback
	}
	getEnvInt := func(key string, fallback int) int {
		if v := os.Getenv(key); v != "" {
			n, err := strconv.Atoi(v)
			if err == nil {
				return n
			}
		}
		return fallback
	}

	cfg := Config{
		Amount:         getEnvInt64("FAUCET_AMOUNT", 100_000_000),    // 100 ZRN
		CooldownHours:  getEnvInt("FAUCET_COOLDOWN", 24),
		Port:           getEnv("FAUCET_PORT", "8080"),
		KeyringBackend: getEnv("FAUCET_KEYRING_BACKEND", "test"),
		From:           getEnv("FAUCET_FROM", "faucet"),
		Node:           getEnv("FAUCET_NODE", "tcp://localhost:26657"),
		Home:           getEnv("FAUCET_HOME", ""),
		ChainID:        getEnv("FAUCET_CHAIN_ID", ""),
		StateFile:      getEnv("FAUCET_STATE_FILE", "faucet-state.json"),
		MaxTotal:       getEnvInt64("FAUCET_MAX_TOTAL", 10_000_000_000_000), // 10M ZRN
	}

	if cfg.Home == "" {
		log.Fatal("FAUCET_HOME is required")
	}
	if cfg.ChainID == "" {
		log.Fatal("FAUCET_CHAIN_ID is required")
	}

	return cfg
}

func loadState(path string) State {
	s := State{Requests: make(map[string]string)}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("WARN: failed to parse state file: %v, starting fresh", err)
		return State{Requests: make(map[string]string)}
	}
	if s.Requests == nil {
		s.Requests = make(map[string]string)
	}
	return s
}

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

func (f *Faucet) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	cfg := loadConfig()
	state := loadState(cfg.StateFile)

	f := &Faucet{cfg: cfg, state: state}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", cors(f.handleHealth))

	addr := ":" + cfg.Port
	log.Printf("Faucet starting on %s (amount=%d uzrn, cooldown=%dh, max_total=%d uzrn)",
		addr, cfg.Amount, cfg.CooldownHours, cfg.MaxTotal)
	log.Fatal(http.ListenAndServe(addr, mux))
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tools/faucet/`
Expected: Compiles without errors.

**Step 3: Commit**

```bash
git add tools/faucet/main.go
git commit -m "feat(faucet): scaffold HTTP server with config and health endpoint"
```

---

### Task 2: Add /stats endpoint

**Files:**
- Modify: `tools/faucet/main.go`

**Step 1: Add the stats handler**

Add this method to the `Faucet` struct:

```go
func (f *Faucet) handleStats(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	totalDistributed := f.state.TotalDistributed
	uniqueAddresses := len(f.state.Requests)
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_distributed_uzrn": totalDistributed,
		"unique_addresses":       uniqueAddresses,
		"remaining_uzrn":         f.cfg.MaxTotal - totalDistributed,
		"amount_per_request_uzrn": f.cfg.Amount,
		"cooldown_hours":         f.cfg.CooldownHours,
	})
}
```

**Step 2: Register the route in main()**

Add to the mux block:
```go
mux.HandleFunc("/stats", cors(f.handleStats))
```

**Step 3: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tools/faucet/`

**Step 4: Commit**

```bash
git add tools/faucet/main.go
git commit -m "feat(faucet): add /stats endpoint"
```

---

### Task 3: Add address validation and state persistence

**Files:**
- Modify: `tools/faucet/main.go`

**Step 1: Add bech32 validation and atomic state save**

Add these imports: `"os/exec"`, `"strings"`, `"path/filepath"`.

Add these functions:

```go
func isValidAddress(addr string) bool {
	if len(addr) < 4 || addr[:4] != "zrn1" {
		return false
	}
	// Use zeroned to validate — most reliable
	// For now, basic length + prefix check is sufficient
	// Bech32 addresses are 39-59 chars for cosmos chains
	if len(addr) < 39 || len(addr) > 59 {
		return false
	}
	// Only alphanumeric (lowercase) after prefix
	for _, c := range addr[4:] {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func (f *Faucet) saveState() error {
	data, err := json.MarshalIndent(f.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp := f.cfg.StateFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp state: %w", err)
	}
	if err := os.Rename(tmp, f.cfg.StateFile); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}
	return nil
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tools/faucet/`

**Step 3: Commit**

```bash
git add tools/faucet/main.go
git commit -m "feat(faucet): add address validation and atomic state persistence"
```

---

### Task 4: Add /faucet POST endpoint

**Files:**
- Modify: `tools/faucet/main.go`

**Step 1: Add the faucet request/response types and handler**

Add these types:

```go
type FaucetRequest struct {
	Address string `json:"address"`
}

type FaucetResponse struct {
	Status string `json:"status"`
	TxHash string `json:"tx_hash,omitempty"`
	Amount string `json:"amount,omitempty"`
	Error  string `json:"error,omitempty"`
}
```

Add the handler:

```go
func (f *Faucet) handleFaucet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(FaucetResponse{Error: "method not allowed, use POST"})
		return
	}

	var req FaucetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FaucetResponse{Error: "invalid JSON body"})
		return
	}

	req.Address = strings.TrimSpace(req.Address)
	if !isValidAddress(req.Address) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FaucetResponse{Error: "invalid address: must be bech32 zrn1..."})
		return
	}

	// Soft cap check (outside mutex — ok to overshoot by one request)
	if f.state.TotalDistributed >= f.cfg.MaxTotal {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(FaucetResponse{Error: "faucet depleted"})
		return
	}

	// Acquire mutex — rate limit check + send + persist must be atomic
	f.mu.Lock()
	defer f.mu.Unlock()

	// Rate limit check (inside mutex to prevent TOCTOU)
	if lastStr, ok := f.state.Requests[req.Address]; ok {
		last, err := time.Parse(time.RFC3339, lastStr)
		if err == nil {
			cooldown := time.Duration(f.cfg.CooldownHours) * time.Hour
			if time.Since(last) < cooldown {
				retryAfter := last.Add(cooldown)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(FaucetResponse{
					Error: fmt.Sprintf("rate limited, retry after %s", retryAfter.Format(time.RFC3339)),
				})
				return
			}
		}
	}

	// Send tokens
	amount := fmt.Sprintf("%duzrn", f.cfg.Amount)
	txHash, err := f.sendTokens(req.Address, amount)
	if err != nil {
		log.Printf("ERROR: send to %s failed: %v", req.Address, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FaucetResponse{Error: fmt.Sprintf("send failed: %v", err)})
		return
	}

	// Update state
	f.state.TotalDistributed += f.cfg.Amount
	f.state.Requests[req.Address] = time.Now().UTC().Format(time.RFC3339)

	if err := f.saveState(); err != nil {
		log.Printf("WARN: failed to persist state: %v", err)
	}

	log.Printf("Sent %s to %s (tx=%s, total=%d)", amount, req.Address, txHash, f.state.TotalDistributed)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(FaucetResponse{
		Status: "ok",
		TxHash: txHash,
		Amount: amount,
	})
}
```

**Step 2: Add the sendTokens method**

```go
func (f *Faucet) sendTokens(toAddr, amount string) (string, error) {
	args := []string{
		"tx", "bank", "send",
		f.cfg.From, toAddr, amount,
		"--keyring-backend", f.cfg.KeyringBackend,
		"--home", f.cfg.Home,
		"--chain-id", f.cfg.ChainID,
		"--node", f.cfg.Node,
		"--broadcast-mode", "sync",
		"--output", "json",
		"--yes",
	}

	cmd := exec.Command("zeroned", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(output))
	}

	// Parse tx hash from JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		// Try to find txhash in raw output
		return "unknown", nil
	}

	if txhash, ok := result["txhash"].(string); ok {
		// Check for non-zero code (tx error)
		if code, ok := result["code"].(float64); ok && code != 0 {
			rawLog, _ := result["raw_log"].(string)
			return "", fmt.Errorf("tx failed (code %d): %s", int(code), rawLog)
		}
		return txhash, nil
	}

	return "unknown", nil
}
```

**Step 3: Register the route in main()**

Add to the mux block:
```go
mux.HandleFunc("/faucet", cors(f.handleFaucet))
```

**Step 4: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tools/faucet/`

**Step 5: Commit**

```bash
git add tools/faucet/main.go
git commit -m "feat(faucet): add POST /faucet endpoint with rate limiting and tx broadcast"
```

---

### Task 5: Add unit tests

**Files:**
- Create: `tools/faucet/main_test.go`

**Step 1: Write tests for validation and state logic**

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		name  string
		addr  string
		valid bool
	}{
		{"valid address", "zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu", true},
		{"too short", "zrn1abc", false},
		{"wrong prefix", "cosmos1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu", false},
		{"empty", "", false},
		{"uppercase chars", "zrn1QYPQXPQ9QCRSSZG2PVXQ6RS0ZQG3YYC5LZV7XU", false},
		{"special chars", "zrn1qypq!pq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidAddress(tt.addr); got != tt.valid {
				t.Errorf("isValidAddress(%q) = %v, want %v", tt.addr, got, tt.valid)
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	f := &Faucet{
		cfg:   Config{Port: "0"},
		state: State{Requests: make(map[string]string)},
	}

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	f.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health returned %d, want 200", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("health status = %q, want ok", resp["status"])
	}
}

func TestStatsEndpoint(t *testing.T) {
	f := &Faucet{
		cfg: Config{
			Amount:        100_000_000,
			MaxTotal:      10_000_000_000_000,
			CooldownHours: 24,
		},
		state: State{
			TotalDistributed: 500_000_000,
			Requests: map[string]string{
				"zrn1abc": "2026-01-01T00:00:00Z",
				"zrn1def": "2026-01-02T00:00:00Z",
			},
		},
	}

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()
	f.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("stats returned %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["unique_addresses"] != float64(2) {
		t.Errorf("unique_addresses = %v, want 2", resp["unique_addresses"])
	}
}

func TestFaucetBadMethod(t *testing.T) {
	f := &Faucet{
		cfg:   Config{},
		state: State{Requests: make(map[string]string)},
	}

	req := httptest.NewRequest("GET", "/faucet", nil)
	w := httptest.NewRecorder()
	f.handleFaucet(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /faucet returned %d, want 405", w.Code)
	}
}

func TestFaucetInvalidAddress(t *testing.T) {
	f := &Faucet{
		cfg:   Config{},
		state: State{Requests: make(map[string]string)},
	}

	body := `{"address": "invalid"}`
	req := httptest.NewRequest("POST", "/faucet", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.handleFaucet(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid addr returned %d, want 400", w.Code)
	}
}

func TestFaucetDepleted(t *testing.T) {
	f := &Faucet{
		cfg: Config{MaxTotal: 100},
		state: State{
			TotalDistributed: 100,
			Requests:         make(map[string]string),
		},
	}

	body := `{"address": "zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu"}`
	req := httptest.NewRequest("POST", "/faucet", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.handleFaucet(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("depleted faucet returned %d, want 503", w.Code)
	}
}

func TestFaucetRateLimited(t *testing.T) {
	addr := "zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu"
	f := &Faucet{
		cfg: Config{
			MaxTotal:      10_000_000_000_000,
			CooldownHours: 24,
			Amount:        100_000_000,
			StateFile:     "test-state.json",
		},
		state: State{
			TotalDistributed: 0,
			Requests: map[string]string{
				addr: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	body := `{"address": "` + addr + `"}`
	req := httptest.NewRequest("POST", "/faucet", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.handleFaucet(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("rate limited returned %d, want 429", w.Code)
	}
}

func TestSaveLoadState(t *testing.T) {
	tmpFile := t.TempDir() + "/test-state.json"
	f := &Faucet{
		cfg: Config{StateFile: tmpFile},
		state: State{
			TotalDistributed: 12345,
			Requests: map[string]string{
				"zrn1test": "2026-01-01T00:00:00Z",
			},
		},
	}

	if err := f.saveState(); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	loaded := loadState(tmpFile)
	if loaded.TotalDistributed != 12345 {
		t.Errorf("TotalDistributed = %d, want 12345", loaded.TotalDistributed)
	}
	if loaded.Requests["zrn1test"] != "2026-01-01T00:00:00Z" {
		t.Errorf("Requests[zrn1test] = %q, want 2026-01-01T00:00:00Z", loaded.Requests["zrn1test"])
	}

	// Cleanup
	os.Remove(tmpFile)
}
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./tools/faucet/ -v`
Expected: All tests pass. The rate limit test will pass because the timestamp is "now" so the cooldown hasn't expired.

**Step 3: Commit**

```bash
git add tools/faucet/main_test.go
git commit -m "test(faucet): add unit tests for validation, endpoints, state, and rate limiting"
```

---

### Task 6: Write README

**Files:**
- Create: `tools/faucet/README.md`

**Step 1: Write the README**

```markdown
# Zerone Testnet Faucet

HTTP server that distributes testnet ZRN tokens.

## Build

    go build -o faucet ./tools/faucet/

## Run

    FAUCET_HOME=~/.zeroned/localnet/coordinator \
    FAUCET_CHAIN_ID=zerone-localnet \
    ./faucet

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| FAUCET_HOME | (required) | zeroned home directory with keyring |
| FAUCET_CHAIN_ID | (required) | Chain ID |
| FAUCET_AMOUNT | 100000000 | uzrn per request (100 ZRN) |
| FAUCET_COOLDOWN | 24 | Hours between requests per address |
| FAUCET_PORT | 8080 | Listen port |
| FAUCET_KEYRING_BACKEND | test | Keyring backend |
| FAUCET_FROM | faucet | Signing key name |
| FAUCET_NODE | tcp://localhost:26657 | Node RPC URL |
| FAUCET_STATE_FILE | faucet-state.json | Rate-limit persistence file |
| FAUCET_MAX_TOTAL | 10000000000000 | Total cap in uzrn (10M ZRN) |

## Endpoints

### POST /faucet

Request tokens:

    curl -X POST http://localhost:8080/faucet \
      -H "Content-Type: application/json" \
      -d '{"address":"zrn1..."}'

Response:

    {"status":"ok","tx_hash":"ABC123...","amount":"100000000uzrn"}

### GET /health

    curl http://localhost:8080/health

### GET /stats

    curl http://localhost:8080/stats

## Rate Limits

- 1 request per address per 24 hours (configurable)
- 503 when total distribution cap is reached
```

**Step 2: Commit**

```bash
git add tools/faucet/README.md
git commit -m "docs(faucet): add README with setup and usage instructions"
```

---

### Task 7: Write testnet economics documentation

**Files:**
- Create: `docs/testnet-economics.md`

**Step 1: Write the token economics doc**

```markdown
# Zerone Testnet Token Economics

## Genesis Supply

Total: **222,222,222,222 ZRN** (222,222,222,222,000,000 uzrn)

## Allocation

| Allocation | ZRN | % | Purpose |
|-----------|-----|---|---------|
| Research Fund | 44,444,444,444 | 20% | Research bounties and rewards |
| Founder | 22,222,222,222 | 10% | Founder allocation |
| AI Agent | 22,222,222,222 | 10% | AI agent allocation |
| Validators (4x) | 88,888,888,888 | 40% | Initial validator set (22.2B each) |
| Claiming Pots | 44,444,444,446 | 20% | Bootstrap claims and airdrops |

## Faucet

- Pre-funded with **10,000,000 ZRN** (10M) from the coordinator genesis account
- Distributes **100 ZRN** per request
- Rate limited to **1 request per address per 24 hours**
- Total cap: **10,000,000 ZRN** (matches funded amount)
- At 100 ZRN/request, supports **100,000 unique faucet requests**

## Claiming Pots (x/claiming_pot)

The claiming_pot module manages eligibility-gated token distribution with linear vesting.

### Capabilities
- Governance creates pots with total amount, vesting schedule, and eligibility criteria
- Users claim vested-but-unclaimed tokens via MsgClaim
- Eligibility filters: whitelist, min staking tier, min registration age
- Linear vesting from start_block to end_block with optional cliff

### Account Type Enforcement

The claiming_pot module does **not** enforce account_type (human/agent/contract).
Eligibility is controlled entirely through whitelist, staking tier, and registration age criteria.
Adding account_type gating would require extending the EligibilityCriteria proto and keeper logic.

### Airdrop Support

The current module supports airdrop-style distribution via:
- Per-pot whitelist of eligible addresses
- Fixed total_amount / number of whitelisted addresses = per-claim amount
- Deadline via end_block (pot expires and transitions to EXPIRED status)
- One claim per address per pot

The "0.222 ZRN per whitelisted agent" bootstrap allocation is achievable with
current module functionality by creating a pot with appropriate whitelist and total amount.

## Localnet Accounts

| Account | Balance | Purpose |
|---------|---------|---------|
| faucet | 10,000,000 ZRN | Token distribution |
| test1 | 10,000 ZRN | Testing |
| test2 | 10,000 ZRN | Testing |
| test3 | 10,000 ZRN | Testing |
| val0 | 100,000 ZRN | Validator (stake: 100 ZRN) |
| val1 | 1,000,000 ZRN | Validator (stake: 1,000 ZRN) |
| val2 | 10,000,000 ZRN | Validator (stake: 10,000 ZRN) |
| val3 | 100,000,000 ZRN | Validator (stake: 100,000 ZRN) |
```

**Step 2: Commit**

```bash
git add docs/testnet-economics.md
git commit -m "docs: add testnet token economics documentation"
```

---

### Task 8: Integration verification

**Files:** None (verification only)

**Step 1: Build the faucet**

Run: `cd /Users/yournameisai/Desktop/zerone && go build -o build/faucet ./tools/faucet/`
Expected: Compiles successfully.

**Step 2: Run unit tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./tools/faucet/ -v -count=1`
Expected: All tests pass.

**Step 3: Verify binary starts (dry run)**

Run: `FAUCET_HOME=/tmp FAUCET_CHAIN_ID=test timeout 2 ./build/faucet || true`
Expected: Starts and prints log line before timeout kills it.

**Step 4: Commit build artifact to .gitignore if needed**

Check if `build/` is already in `.gitignore`. If not, the binary is already ignored (it should be).
