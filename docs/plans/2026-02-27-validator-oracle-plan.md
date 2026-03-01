# Validator Evaluation Oracle — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build an advisory sidecar oracle (`zerone-oracle`) that validators can query before voting, plus wire it into the vote extension handler so obviously false claims have a chance of being rejected.

**Architecture:** Separate Go binary (`cmd/oracle/`) communicating via HTTP/JSON with the validator's vote extension handler. Two evaluation tiers: static (keyword + numerical contradiction against 777 genesis axioms) and LLM (Claude API). Oracle failure never blocks consensus.

**Tech Stack:** Go 1.24, net/http (both server and client), `x/knowledge/types.GenesisAxiomsJSON` (embedded axioms), Anthropic Messages API (Tier 2)

---

### Task 1: Evaluator Interface & Types

**Files:**
- Create: `cmd/oracle/evaluator.go`

**Step 1: Write the evaluator interface and types**

```go
package main

// EvaluateRequest is the input for claim evaluation.
type EvaluateRequest struct {
	Claim     string `json:"claim"`
	Domain    string `json:"domain"`
	ClaimType string `json:"claim_type"`
}

// EvaluateResponse is the output of claim evaluation.
type EvaluateResponse struct {
	Verdict    string  `json:"verdict"`    // "accept", "reject", "uncertain"
	Confidence float64 `json:"confidence"` // 0.0 - 1.0
	Reasoning  string  `json:"reasoning"`
}

// Evaluator evaluates a claim and returns a verdict.
type Evaluator interface {
	Evaluate(req EvaluateRequest) (*EvaluateResponse, error)
	Name() string
}

// CombinedEvaluator runs static evaluation first, then optionally LLM.
type CombinedEvaluator struct {
	Static *StaticEvaluator
	LLM    *LLMEvaluator // nil if not configured
}

func (c *CombinedEvaluator) Evaluate(req EvaluateRequest) (*EvaluateResponse, error) {
	// Always run static first
	staticResult, err := c.Static.Evaluate(req)
	if err != nil {
		staticResult = &EvaluateResponse{Verdict: "uncertain", Confidence: 0.5, Reasoning: "static evaluation error: " + err.Error()}
	}

	// High-confidence static verdict (>0.7) → short-circuit
	if staticResult.Confidence > 0.7 && staticResult.Verdict != "uncertain" {
		return staticResult, nil
	}

	// If LLM is available and static was uncertain, use LLM
	if c.LLM != nil && staticResult.Verdict == "uncertain" {
		llmResult, err := c.LLM.Evaluate(req)
		if err != nil {
			// LLM failure → fall back to static result
			return staticResult, nil
		}
		return llmResult, nil
	}

	return staticResult, nil
}

func (c *CombinedEvaluator) Name() string { return "combined" }
```

**Step 2: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./cmd/oracle/...`
Expected: May fail (StaticEvaluator/LLMEvaluator not yet defined). That's fine — this file will compile after Tasks 2-3.

**Step 3: Commit**

```bash
git add cmd/oracle/evaluator.go
git commit -m "feat(oracle): add evaluator interface and combined evaluator"
```

---

### Task 2: Static Evaluator (Tier 1)

**Files:**
- Create: `cmd/oracle/evaluator_static.go`
- Create: `cmd/oracle/evaluator_static_test.go`

**Step 1: Write the failing tests**

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticEvaluator_LoadsAxioms(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)
	require.Greater(t, eval.AxiomCount(), 0, "should load genesis axioms")
}

func TestStaticEvaluator_AcceptsConsistentClaim(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	// Claim consistent with axioms — should not reject
	resp, err := eval.Evaluate(EvaluateRequest{
		Claim:  "Electromagnetic waves propagate at the speed of light in vacuum",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.NotEqual(t, "reject", resp.Verdict, "consistent claim should not be rejected")
}

func TestStaticEvaluator_RejectsNumericalContradiction(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	// Claim with numerical contradiction (speed of light is ~3x10^8 m/s, not 100 m/s)
	resp, err := eval.Evaluate(EvaluateRequest{
		Claim:  "The speed of light in vacuum is 100 m/s",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.Equal(t, "reject", resp.Verdict, "numerical contradiction should be rejected")
	require.Greater(t, resp.Confidence, 0.5, "rejection should have meaningful confidence")
}

func TestStaticEvaluator_RejectsExplicitNegation(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	// Explicit negation of a known axiom
	resp, err := eval.Evaluate(EvaluateRequest{
		Claim:  "The speed of light is not constant across inertial frames",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.Equal(t, "reject", resp.Verdict, "explicit negation of axiom should be rejected")
}

func TestStaticEvaluator_UncertainForUnrelatedClaim(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	// Claim in domain with no matching axiom
	resp, err := eval.Evaluate(EvaluateRequest{
		Claim:  "Bananas are the most popular fruit in Norway",
		Domain: "general",
	})
	require.NoError(t, err)
	require.Equal(t, "uncertain", resp.Verdict, "unrelated claim should be uncertain")
}

func TestStaticEvaluator_DomainFiltering(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	// Physics claim checked against biology domain — should be uncertain
	resp, err := eval.Evaluate(EvaluateRequest{
		Claim:  "The speed of light in vacuum is 100 m/s",
		Domain: "biology",
	})
	require.NoError(t, err)
	// Should not match physics axioms when domain is biology
	require.Equal(t, "uncertain", resp.Verdict, "wrong domain should not match")
}

func TestExtractNumbers_Basic(t *testing.T) {
	nums := extractNumbers("The speed is 299792458 m/s and temperature is 25.5°C")
	require.Contains(t, nums, 299792458.0)
	require.Contains(t, nums, 25.5)
}

func TestExtractNumbers_Scientific(t *testing.T) {
	nums := extractNumbers("c ≈ 2.998 × 10⁸ m/s")
	require.NotEmpty(t, nums, "should extract scientific notation")
	// Should find 2.998 at minimum
	found := false
	for _, n := range nums {
		if n > 2 && n < 4 {
			found = true
		}
	}
	require.True(t, found, "should extract 2.998")
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./cmd/oracle/... -run TestStatic -v`
Expected: FAIL — `NewStaticEvaluator` not defined

**Step 3: Write the static evaluator**

```go
package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// axiomEntry is an indexed axiom for fast lookup.
type axiomEntry struct {
	ID         string
	Statement  string
	Domain     string
	Keywords   []string
	Numbers    []float64
}

// StaticEvaluator checks claims against the 777 genesis axioms.
// It catches:
//   - Claims with numerical values contradicting axiom values in the same domain
//   - Claims with explicit negation of known axioms
//
// It does NOT catch:
//   - Semantic contradictions (e.g., "the sun orbits the earth")
//   - Paraphrased falsehoods
//   - Claims in domains with no matching axioms
type StaticEvaluator struct {
	axioms    []axiomEntry
	byDomain  map[string][]int // domain → indices into axioms
}

func NewStaticEvaluator() (*StaticEvaluator, error) {
	raw, err := knowledgetypes.ParseAxioms(knowledgetypes.GenesisAxiomsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse genesis axioms: %w", err)
	}

	eval := &StaticEvaluator{
		axioms:   make([]axiomEntry, 0, len(raw)),
		byDomain: make(map[string][]int),
	}

	for _, a := range raw {
		entry := axiomEntry{
			ID:        a.AxiomID,
			Statement: a.Statement,
			Domain:    a.Domain,
			Keywords:  tokenize(a.Statement),
			Numbers:   extractNumbers(a.Statement + " " + a.FormalExpression),
		}
		idx := len(eval.axioms)
		eval.axioms = append(eval.axioms, entry)
		eval.byDomain[a.Domain] = append(eval.byDomain[a.Domain], idx)
	}

	return eval, nil
}

func (s *StaticEvaluator) Name() string    { return "static" }
func (s *StaticEvaluator) AxiomCount() int { return len(s.axioms) }

func (s *StaticEvaluator) Evaluate(req EvaluateRequest) (*EvaluateResponse, error) {
	claimKeywords := tokenize(req.Claim)
	claimNumbers := extractNumbers(req.Claim)
	domain := req.Domain

	// Get axioms in the same domain
	indices, ok := s.byDomain[domain]
	if !ok || len(indices) == 0 {
		return &EvaluateResponse{
			Verdict:    "uncertain",
			Confidence: 0.5,
			Reasoning:  fmt.Sprintf("no axioms found for domain %q", domain),
		}, nil
	}

	bestMatch := 0.0
	var bestAxiom *axiomEntry
	for _, idx := range indices {
		axiom := &s.axioms[idx]
		score := keywordOverlap(claimKeywords, axiom.Keywords)
		if score > bestMatch {
			bestMatch = score
			bestAxiom = axiom
		}
	}

	// No significant keyword match
	if bestMatch < 0.15 || bestAxiom == nil {
		return &EvaluateResponse{
			Verdict:    "uncertain",
			Confidence: 0.5,
			Reasoning:  "no matching axiom found",
		}, nil
	}

	// Check for explicit negation
	if hasNegation(req.Claim, bestAxiom.Statement) {
		conf := math.Min(0.6+bestMatch*0.4, 0.95)
		return &EvaluateResponse{
			Verdict:    "reject",
			Confidence: conf,
			Reasoning:  fmt.Sprintf("claim appears to negate axiom %s: %q", bestAxiom.ID, bestAxiom.Statement),
		}, nil
	}

	// Check for numerical contradiction
	if len(claimNumbers) > 0 && len(bestAxiom.Numbers) > 0 {
		contradicts, claimNum, axiomNum := numericalContradiction(claimNumbers, bestAxiom.Numbers)
		if contradicts {
			conf := math.Min(0.6+bestMatch*0.4, 0.95)
			return &EvaluateResponse{
				Verdict:    "reject",
				Confidence: conf,
				Reasoning: fmt.Sprintf("numerical mismatch with axiom %s: claim has %.6g, axiom has %.6g",
					bestAxiom.ID, claimNum, axiomNum),
			}, nil
		}
	}

	// Claim matches an axiom — accept
	if bestMatch > 0.4 {
		return &EvaluateResponse{
			Verdict:    "accept",
			Confidence: math.Min(0.5+bestMatch*0.5, 0.95),
			Reasoning:  fmt.Sprintf("claim consistent with axiom %s", bestAxiom.ID),
		}, nil
	}

	return &EvaluateResponse{
		Verdict:    "uncertain",
		Confidence: 0.5,
		Reasoning:  fmt.Sprintf("weak match to axiom %s (score %.2f), insufficient to judge", bestAxiom.ID, bestMatch),
	}, nil
}

// tokenize splits text into lowercase keyword tokens, filtering stop words.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	// Remove common punctuation except hyphens
	text = strings.NewReplacer(
		",", " ", ".", " ", ";", " ", ":", " ", "(", " ", ")", " ",
		"[", " ", "]", " ", "{", " ", "}", " ", "\"", " ", "'", " ",
		"=", " ", "+", " ", "/", " ", "\\", " ",
		"≈", " ", "×", " ", "·", " ",
	).Replace(text)

	words := strings.Fields(text)
	var result []string
	for _, w := range words {
		if len(w) < 3 || stopWords[w] {
			continue
		}
		result = append(result, w)
	}
	return result
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true,
	"not": true, "you": true, "all": true, "can": true, "has": true,
	"her": true, "was": true, "one": true, "our": true, "out": true,
	"had": true, "hot": true, "its": true, "let": true, "may": true,
	"who": true, "did": true, "get": true, "him": true, "his": true,
	"how": true, "man": true, "new": true, "now": true, "old": true,
	"see": true, "way": true, "day": true, "too": true, "any": true,
	"that": true, "with": true, "have": true, "this": true, "will": true,
	"your": true, "from": true, "they": true, "been": true, "each": true,
	"make": true, "like": true, "than": true, "them": true, "then": true,
	"when": true, "what": true, "some": true, "into": true, "also": true,
	"which": true, "where": true, "their": true, "there": true, "these": true,
	"other": true, "about": true, "would": true, "could": true, "should": true,
}

// keywordOverlap computes Jaccard similarity between two keyword sets.
func keywordOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(a))
	for _, w := range a {
		setA[w] = true
	}
	setB := make(map[string]bool, len(b))
	for _, w := range b {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// negationWords detects explicit negation in claims.
var negationWords = []string{"not", "never", "false", "incorrect", "wrong", "isn't", "doesn't", "cannot", "no longer"}

// hasNegation checks if a claim negates an axiom statement.
func hasNegation(claim, axiom string) bool {
	claimLower := strings.ToLower(claim)
	for _, neg := range negationWords {
		if strings.Contains(claimLower, neg) {
			// Check that the non-negation parts overlap with the axiom
			stripped := strings.Replace(claimLower, neg, "", 1)
			claimTokens := tokenize(stripped)
			axiomTokens := tokenize(axiom)
			overlap := keywordOverlap(claimTokens, axiomTokens)
			if overlap > 0.2 {
				return true
			}
		}
	}
	return false
}

// numberRegex matches integers, decimals, and basic scientific notation.
var numberRegex = regexp.MustCompile(`(\d+\.?\d*)\s*[×x]\s*10[⁰¹²³⁴⁵⁶⁷⁸⁹]+|(\d+\.?\d*)`)

// superscriptMap converts Unicode superscript digits to regular digits.
var superscriptMap = map[rune]rune{
	'⁰': '0', '¹': '1', '²': '2', '³': '3', '⁴': '4',
	'⁵': '5', '⁶': '6', '⁷': '7', '⁸': '8', '⁹': '9',
	'⁻': '-',
}

// extractNumbers extracts all numeric values from text, including scientific notation.
func extractNumbers(text string) []float64 {
	var nums []float64

	// First pass: scientific notation with superscripts (e.g., "2.998 × 10⁸")
	sciRegex := regexp.MustCompile(`(\d+\.?\d*)\s*[×x]\s*10([⁰¹²³⁴⁵⁶⁷⁸⁹⁻]+)`)
	sciMatches := sciRegex.FindAllStringSubmatch(text, -1)
	for _, m := range sciMatches {
		base, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		expStr := convertSuperscript(m[2])
		exp, err := strconv.ParseFloat(expStr, 64)
		if err != nil {
			continue
		}
		nums = append(nums, base*math.Pow(10, exp))
	}

	// Second pass: plain numbers (skip those already found in scientific notation)
	plainRegex := regexp.MustCompile(`\b(\d+\.?\d*)\b`)
	for _, m := range plainRegex.FindAllStringSubmatch(text, -1) {
		n, err := strconv.ParseFloat(m[1], 64)
		if err != nil || n == 0 {
			continue
		}
		// Skip if this number is part of a scientific notation already extracted
		alreadyFound := false
		for _, existing := range nums {
			if n == existing {
				alreadyFound = true
				break
			}
		}
		if !alreadyFound {
			nums = append(nums, n)
		}
	}

	return nums
}

// convertSuperscript converts superscript digits to a regular string.
func convertSuperscript(s string) string {
	var result []rune
	for _, r := range s {
		if mapped, ok := superscriptMap[r]; ok {
			result = append(result, mapped)
		}
	}
	return string(result)
}

// numericalContradiction checks if any number in the claim contradicts a number in the axiom.
// Two numbers "contradict" if they are in the same order of magnitude range but differ by >50%.
func numericalContradiction(claimNums, axiomNums []float64) (bool, float64, float64) {
	for _, cn := range claimNums {
		for _, an := range axiomNums {
			if cn == 0 || an == 0 {
				continue
			}
			// Both numbers should be in similar magnitude range (within 6 orders)
			ratio := cn / an
			if ratio < 1e-6 || ratio > 1e6 {
				continue // Too different in magnitude to be about the same thing
			}
			// Check if they differ significantly (>50% relative difference)
			relDiff := math.Abs(cn-an) / math.Max(math.Abs(cn), math.Abs(an))
			if relDiff > 0.5 {
				return true, cn, an
			}
		}
	}
	return false, 0, 0
}
```

**Step 4: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./cmd/oracle/... -run TestStatic -v`
Expected: PASS (all static evaluator tests)

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./cmd/oracle/... -run TestExtractNumbers -v`
Expected: PASS (number extraction tests)

**Step 5: Commit**

```bash
git add cmd/oracle/evaluator_static.go cmd/oracle/evaluator_static_test.go
git commit -m "feat(oracle): add static evaluator with keyword + numerical contradiction detection"
```

---

### Task 3: LLM Evaluator (Tier 2)

**Files:**
- Create: `cmd/oracle/evaluator_llm.go`
- Create: `cmd/oracle/evaluator_llm_test.go`

**Step 1: Write the failing tests**

```go
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLLMEvaluator_ParsesResponse(t *testing.T) {
	// Mock Claude API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/v1/messages", r.URL.Path)
		require.Contains(t, r.Header.Get("x-api-key"), "test-key")

		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"verdict":"reject","confidence":0.85,"reasoning":"This contradicts known physics."}`},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	eval := NewLLMEvaluator(server.URL, "test-key", "claude-3-5-sonnet-20241022", 500, 2*time.Second)
	resp, err := eval.Evaluate(EvaluateRequest{
		Claim:  "The speed of light is 100 m/s",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.Equal(t, "reject", resp.Verdict)
	require.InDelta(t, 0.85, resp.Confidence, 0.01)
}

func TestLLMEvaluator_Timeout(t *testing.T) {
	// Mock server that takes too long
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	eval := NewLLMEvaluator(server.URL, "test-key", "test-model", 500, 500*time.Millisecond)
	_, err := eval.Evaluate(EvaluateRequest{
		Claim:  "test claim",
		Domain: "general",
	})
	require.Error(t, err, "should timeout")
}

func TestLLMEvaluator_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"verdict":"accept","confidence":0.9,"reasoning":"Cached."}`},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	eval := NewLLMEvaluator(server.URL, "test-key", "test-model", 500, 2*time.Second)

	req := EvaluateRequest{Claim: "Earth orbits the Sun", Domain: "physics"}
	_, err := eval.Evaluate(req)
	require.NoError(t, err)
	require.Equal(t, 1, callCount)

	// Second call should hit cache
	_, err = eval.Evaluate(req)
	require.NoError(t, err)
	require.Equal(t, 1, callCount, "second call should use cache")
}

func TestLLMEvaluator_CacheKeyIncludesDomain(t *testing.T) {
	key1 := cacheKey("test claim", "physics")
	key2 := cacheKey("test claim", "biology")
	require.NotEqual(t, key1, key2, "different domains should produce different cache keys")
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./cmd/oracle/... -run TestLLM -v`
Expected: FAIL — `NewLLMEvaluator` not defined

**Step 3: Write the LLM evaluator**

```go
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultAnthropicURL = "https://api.anthropic.com"

// LLMEvaluator evaluates claims using the Anthropic Claude API.
type LLMEvaluator struct {
	apiURL    string
	apiKey    string
	model     string
	maxTokens int
	timeout   time.Duration
	client    *http.Client

	mu    sync.Mutex
	cache map[string]*EvaluateResponse
}

func NewLLMEvaluator(apiURL, apiKey, model string, maxTokens int, timeout time.Duration) *LLMEvaluator {
	if apiURL == "" {
		apiURL = defaultAnthropicURL
	}
	return &LLMEvaluator{
		apiURL:    strings.TrimRight(apiURL, "/"),
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		timeout:   timeout,
		client:    &http.Client{Timeout: timeout},
		cache:     make(map[string]*EvaluateResponse),
	}
}

func (l *LLMEvaluator) Name() string { return "llm" }

func cacheKey(claim, domain string) string {
	h := sha256.Sum256([]byte(domain + "|" + claim))
	return fmt.Sprintf("%x", h)
}

func (l *LLMEvaluator) Evaluate(req EvaluateRequest) (*EvaluateResponse, error) {
	key := cacheKey(req.Claim, req.Domain)

	// Check cache
	l.mu.Lock()
	if cached, ok := l.cache[key]; ok {
		l.mu.Unlock()
		return cached, nil
	}
	l.mu.Unlock()

	// Build Claude API request
	systemPrompt := `You are a fact-checking oracle for a truth-seeking blockchain. Evaluate the following claim for factual accuracy.

Respond ONLY with a JSON object (no markdown, no explanation outside JSON):
{"verdict": "accept|reject|uncertain", "confidence": 0.0-1.0, "reasoning": "brief explanation"}

- "accept": The claim is factually correct or well-supported
- "reject": The claim is factually incorrect or contradicts established knowledge
- "uncertain": Cannot determine with confidence

Be conservative: only reject claims you are confident are false.`

	userContent := fmt.Sprintf("Domain: %s\nClaim: %s", req.Domain, req.Claim)

	body := map[string]interface{}{
		"model":      l.model,
		"max_tokens": l.maxTokens,
		"system":     systemPrompt,
		"messages": []map[string]interface{}{
			{"role": "user", "content": userContent},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", l.apiURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", l.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty API response")
	}

	// Parse the JSON verdict from the response text
	var result EvaluateResponse
	text := apiResp.Content[0].Text
	// Try to extract JSON from the response (may be wrapped in markdown)
	jsonStart := strings.Index(text, "{")
	jsonEnd := strings.LastIndex(text, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		text = text[jsonStart : jsonEnd+1]
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse verdict from response: %w (raw: %s)", err, apiResp.Content[0].Text)
	}

	// Validate verdict
	switch result.Verdict {
	case "accept", "reject", "uncertain":
	default:
		result.Verdict = "uncertain"
	}

	// Clamp confidence
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 1 {
		result.Confidence = 1
	}

	// Cache result (LRU eviction when cache exceeds 1000 entries)
	l.mu.Lock()
	if len(l.cache) >= 1000 {
		// Simple eviction: clear half the cache
		count := 0
		for k := range l.cache {
			delete(l.cache, k)
			count++
			if count >= 500 {
				break
			}
		}
	}
	l.cache[key] = &result
	l.mu.Unlock()

	return &result, nil
}

// Prefetch evaluates a claim and caches the result without blocking.
func (l *LLMEvaluator) Prefetch(req EvaluateRequest) {
	go func() {
		l.Evaluate(req) //nolint:errcheck
	}()
}
```

**Step 4: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./cmd/oracle/... -run TestLLM -v -timeout 10s`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/oracle/evaluator_llm.go cmd/oracle/evaluator_llm_test.go
git commit -m "feat(oracle): add LLM evaluator with Claude API client and response cache"
```

---

### Task 4: Oracle HTTP Server

**Files:**
- Create: `cmd/oracle/main.go`
- Create: `cmd/oracle/main_test.go`

**Step 1: Write the failing tests**

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOracleServer_Health(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	srv := newOracleServer(&CombinedEvaluator{Static: eval}, "static")
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "ok", resp["status"])
	require.Equal(t, "static", resp["tier"])
}

func TestOracleServer_Evaluate(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	srv := newOracleServer(&CombinedEvaluator{Static: eval}, "static")

	body, _ := json.Marshal(EvaluateRequest{
		Claim:  "Electromagnetic waves propagate at the speed of light",
		Domain: "physics",
	})
	req := httptest.NewRequest("POST", "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp EvaluateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Contains(t, []string{"accept", "reject", "uncertain"}, resp.Verdict)
}

func TestOracleServer_EvaluateBadRequest(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	srv := newOracleServer(&CombinedEvaluator{Static: eval}, "static")

	req := httptest.NewRequest("POST", "/evaluate", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOracleServer_Prefetch(t *testing.T) {
	eval, err := NewStaticEvaluator()
	require.NoError(t, err)

	srv := newOracleServer(&CombinedEvaluator{Static: eval}, "static")

	body, _ := json.Marshal(EvaluateRequest{
		Claim:  "test claim",
		Domain: "general",
	})
	req := httptest.NewRequest("POST", "/prefetch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./cmd/oracle/... -run TestOracleServer -v`
Expected: FAIL — `newOracleServer` not defined

**Step 3: Write the HTTP server**

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func newOracleServer(eval *CombinedEvaluator, tier string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"tier":   tier,
		})
	})

	mux.HandleFunc("POST /evaluate", func(w http.ResponseWriter, r *http.Request) {
		var req EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Claim == "" {
			http.Error(w, `{"error":"claim is required"}`, http.StatusBadRequest)
			return
		}

		resp, err := eval.Evaluate(req)
		if err != nil {
			log.Printf("evaluation error: %v", err)
			resp = &EvaluateResponse{Verdict: "uncertain", Confidence: 0.5, Reasoning: "evaluation error"}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("POST /prefetch", func(w http.ResponseWriter, r *http.Request) {
		var req EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		// Evaluate in background (result goes to cache)
		go func() {
			eval.Evaluate(req) //nolint:errcheck
		}()

		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "prefetching"})
	})

	return mux
}

func main() {
	port := flag.Int("port", 8081, "HTTP server port")
	tier := flag.String("tier", "static", "evaluation tier: static or llm")
	apiKey := flag.String("llm-api-key", "", "Anthropic API key (required for llm tier)")
	apiURL := flag.String("llm-api-url", defaultAnthropicURL, "Anthropic API base URL")
	model := flag.String("llm-model", "claude-sonnet-4-5-20250514", "LLM model ID")
	maxTokens := flag.Int("llm-max-tokens", 500, "LLM max response tokens")
	llmTimeout := flag.Duration("llm-timeout", 2*time.Second, "LLM API call timeout")
	flag.Parse()

	// Build static evaluator (always needed)
	staticEval, err := NewStaticEvaluator()
	if err != nil {
		log.Fatalf("Failed to initialize static evaluator: %v", err)
	}
	log.Printf("Loaded %d genesis axioms", staticEval.AxiomCount())

	combined := &CombinedEvaluator{Static: staticEval}

	// Optionally add LLM evaluator
	if *tier == "llm" {
		if *apiKey == "" {
			log.Fatal("--llm-api-key is required when tier=llm")
		}
		combined.LLM = NewLLMEvaluator(*apiURL, *apiKey, *model, *maxTokens, *llmTimeout)
		log.Printf("LLM evaluator enabled: model=%s, timeout=%s", *model, *llmTimeout)
	}

	srv := newOracleServer(combined, *tier)
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("zerone-oracle starting on %s (tier=%s)", addr, *tier)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

**Step 4: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./cmd/oracle/... -v`
Expected: ALL PASS (static, LLM, and server tests)

**Step 5: Verify the binary builds**

Run: `cd /Users/yournameisai/Desktop/zerone && go build -o ./build/zerone-oracle ./cmd/oracle/`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add cmd/oracle/main.go cmd/oracle/main_test.go
git commit -m "feat(oracle): add HTTP server with /evaluate, /prefetch, /health endpoints"
```

---

### Task 5: Oracle Client for Vote Extensions

**Files:**
- Create: `app/oracle_client.go`
- Create: `app/oracle_client_test.go`

**Step 1: Write the failing tests**

```go
package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"
)

func TestOracleClient_Evaluate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/evaluate", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"verdict":    "reject",
			"confidence": 0.85,
			"reasoning":  "contradicts known physics",
		})
	}))
	defer server.Close()

	client := zeroneapp.NewHTTPOracleClient(server.URL, 2*time.Second, 0.6)
	result, err := client.Evaluate("The speed of light is 100 m/s", "physics", "assertion")
	require.NoError(t, err)
	require.Equal(t, "reject", result.Verdict)
	require.InDelta(t, 0.85, result.Confidence, 0.01)
}

func TestOracleClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer server.Close()

	client := zeroneapp.NewHTTPOracleClient(server.URL, 500*time.Millisecond, 0.6)
	_, err := client.Evaluate("test", "general", "assertion")
	require.Error(t, err, "should timeout")
}

func TestOracleClient_ErrorFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := zeroneapp.NewHTTPOracleClient(server.URL, 2*time.Second, 0.6)
	_, err := client.Evaluate("test", "general", "assertion")
	require.Error(t, err, "should return error on 500")
}

func TestOracleClient_LowConfidenceToUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"verdict":    "reject",
			"confidence": 0.3,
			"reasoning":  "weak signal",
		})
	}))
	defer server.Close()

	client := zeroneapp.NewHTTPOracleClient(server.URL, 2*time.Second, 0.6)
	result, err := client.Evaluate("test", "general", "assertion")
	require.NoError(t, err)
	// Low confidence reject should be overridden to uncertain
	require.Equal(t, "uncertain", result.Verdict, "low confidence should be treated as uncertain")
}

func TestEvaluateWithOracle_NilClient(t *testing.T) {
	verdict, confidence := zeroneapp.EvaluateWithOracle(nil, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(600_000), confidence)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./app/... -run TestOracle -v`
Expected: FAIL — types not defined

**Step 3: Write the oracle client**

```go
package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OracleEvaluation is the result from the oracle sidecar.
type OracleEvaluation struct {
	Verdict    string  `json:"verdict"`    // "accept", "reject", "uncertain"
	Confidence float64 `json:"confidence"` // 0.0 - 1.0
	Reasoning  string  `json:"reasoning"`
}

// OracleClient queries the oracle sidecar for claim evaluation.
type OracleClient interface {
	Evaluate(claim, domain, claimType string) (*OracleEvaluation, error)
}

// HTTPOracleClient is an HTTP-based OracleClient implementation.
type HTTPOracleClient struct {
	endpoint      string
	timeout       time.Duration
	minConfidence float64
	client        *http.Client
}

// NewHTTPOracleClient creates a new HTTP oracle client.
func NewHTTPOracleClient(endpoint string, timeout time.Duration, minConfidence float64) *HTTPOracleClient {
	return &HTTPOracleClient{
		endpoint:      endpoint,
		timeout:       timeout,
		minConfidence: minConfidence,
		client:        &http.Client{Timeout: timeout},
	}
}

func (c *HTTPOracleClient) Evaluate(claim, domain, claimType string) (*OracleEvaluation, error) {
	body, err := json.Marshal(map[string]string{
		"claim":      claim,
		"domain":     domain,
		"claim_type": claimType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.Post(c.endpoint+"/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("oracle request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oracle returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result OracleEvaluation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode oracle response: %w", err)
	}

	// Confidence threshold: low-confidence verdicts become uncertain
	if result.Confidence < c.minConfidence && result.Verdict != "uncertain" {
		result.Verdict = "uncertain"
		result.Reasoning = fmt.Sprintf("below confidence threshold (%.2f < %.2f): %s", result.Confidence, c.minConfidence, result.Reasoning)
	}

	return &result, nil
}

// EvaluateWithOracle queries the oracle and returns a verdict + confidence for vote extensions.
// If client is nil or the oracle fails, returns the default accept verdict.
func EvaluateWithOracle(client OracleClient, claim, domain, claimType string) (string, uint64) {
	const defaultVerdict = "accept"
	const defaultConfidence = uint64(600_000)

	if client == nil {
		return defaultVerdict, defaultConfidence
	}

	result, err := client.Evaluate(claim, domain, claimType)
	if err != nil {
		return defaultVerdict, defaultConfidence
	}

	verdict := result.Verdict
	confidence := uint64(result.Confidence * 1_000_000)

	// Cap confidence at 1,000,000
	if confidence > 1_000_000 {
		confidence = 1_000_000
	}

	// Uncertain → default to accept with oracle's confidence
	if verdict == "uncertain" {
		return "accept", confidence
	}

	return verdict, confidence
}
```

**Step 4: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./app/... -run TestOracle -v`
Run: `cd /Users/yournameisai/Desktop/zerone && go test ./app/... -run TestEvaluateWithOracle -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add app/oracle_client.go app/oracle_client_test.go
git commit -m "feat(oracle): add HTTP oracle client with confidence threshold and fallback"
```

---

### Task 6: Wire Oracle into Vote Extensions

**Files:**
- Modify: `app/vote_extensions.go:17-27` (add OracleClient field)
- Modify: `app/vote_extensions.go:169-172` (replace stub verdict)

**Step 1: Add OracleClient to VoteExtensionConfig**

In `app/vote_extensions.go`, modify `VoteExtensionConfig` at line 17 to add the oracle client field:

```go
// VoteExtensionConfig holds per-validator configuration for vote extensions.
type VoteExtensionConfig struct {
	// ValidatorAddress is the validator's bech32 operator address.
	ValidatorAddress string

	// ValidatorPrivateKey is the Ed25519 private key for VRF generation.
	// This is the 32-byte seed or 64-byte full private key.
	ValidatorPrivateKey []byte

	// LocalStore holds commitment salts between commit and reveal phases.
	LocalStore *LocalCommitmentStore

	// OracleClient is an optional client for querying the oracle sidecar.
	// If nil, the default accept verdict is used.
	OracleClient OracleClient
}
```

**Step 2: Replace stub verdict at lines 169-172**

Replace:
```go
	// Stub evaluation: accept with 600K confidence.
	// Full deterministic evaluation engine (evaluation.EvaluateClaim) will be wired later.
	verdict := "accept"
	confidence := uint64(600_000)
```

With:
```go
	// Evaluate claim via oracle sidecar (if configured).
	// Falls back to accept@600K if oracle is nil, unreachable, or returns error.
	claim, claimErr := app.KnowledgeKeeper.GetClaim(ctx, round.ClaimId)
	var verdict string
	var confidence uint64
	if claimErr != nil {
		verdict = "accept"
		confidence = 600_000
	} else {
		verdict, confidence = EvaluateWithOracle(
			config.OracleClient,
			claim.FactContent,
			claim.Domain,
			claim.ClaimType.String(),
		)
	}
```

**Step 3: Run existing tests to verify nothing breaks**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./app/... -v -timeout 60s`
Expected: ALL PASS (existing tests use nil OracleClient → unchanged behavior)

**Step 4: Commit**

```bash
git add app/vote_extensions.go
git commit -m "feat(oracle): wire oracle client into vote extension commit phase"
```

---

### Task 7: Add Oracle Config to app.toml

**Files:**
- Modify: `cmd/zeroned/cmd/config.go`
- Modify: `app/app.go:487-494` (read config in NewZeroneApp)

**Step 1: Extend config.go with oracle section**

Replace the full `config.go` content:

```go
package cmd

import (
	"time"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"

	cmtcfg "github.com/cometbft/cometbft/config"
)

// OracleConfig holds configuration for the validator evaluation oracle sidecar.
type OracleConfig struct {
	// Enabled enables oracle queries during vote extension commit phase.
	Enabled bool `mapstructure:"enabled"`
	// Endpoint is the HTTP URL of the oracle sidecar.
	Endpoint string `mapstructure:"endpoint"`
	// Timeout is the maximum time to wait for an oracle response.
	Timeout time.Duration `mapstructure:"timeout"`
	// MinConfidence is the minimum confidence threshold to use the oracle's verdict.
	MinConfidence float64 `mapstructure:"min-confidence"`
}

// ZeroneAppConfig extends the SDK server config with oracle settings.
type ZeroneAppConfig struct {
	serverconfig.Config `mapstructure:",squash"`
	Oracle              OracleConfig `mapstructure:"oracle"`
}

const oracleConfigTemplate = `

###############################################################################
###                      Oracle Sidecar Configuration                       ###
###############################################################################

[oracle]

# Enable oracle queries during vote extension commit phase.
# When enabled, the validator queries the oracle sidecar before voting.
# Oracle failure never blocks consensus — falls back to default accept.
enabled = {{ .Oracle.Enabled }}

# HTTP endpoint of the zerone-oracle sidecar.
endpoint = "{{ .Oracle.Endpoint }}"

# Maximum time to wait for an oracle response.
# Must be less than the vote extension timeout (typically 2-3s).
timeout = "{{ .Oracle.Timeout }}"

# Minimum confidence threshold to use the oracle's verdict.
# Verdicts below this threshold are treated as uncertain (default: accept).
# Protects validators from slashing on weak oracle signals.
min-confidence = {{ printf "%.2f" .Oracle.MinConfidence }}
`

// initAppConfig returns the default server configuration template and values
// with zerone-specific overrides.
func initAppConfig() (string, interface{}) {
	srvCfg := serverconfig.DefaultConfig()

	// Minimum gas price prevents spam; 0.025 uzrn ≈ negligible for real users.
	srvCfg.MinGasPrices = "0.025uzrn"

	// Enable REST API and Swagger UI by default for testnet convenience.
	srvCfg.API.Enable = true
	srvCfg.API.Swagger = true

	customConfig := ZeroneAppConfig{
		Config: *srvCfg,
		Oracle: OracleConfig{
			Enabled:       false,
			Endpoint:      "http://localhost:8081",
			Timeout:       2 * time.Second,
			MinConfidence: 0.6,
		},
	}

	customTemplate := serverconfig.DefaultConfigTemplate + oracleConfigTemplate

	return customTemplate, customConfig
}

// initCometBFTConfig returns the default CometBFT configuration with
// zerone-specific overrides.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// Zerone targets 2521ms block time.
	cfg.Consensus.TimeoutCommit = 2521 * time.Millisecond

	return cfg
}
```

**Step 2: Wire oracle client in NewZeroneApp**

In `app/app.go`, after the ABCI handler wiring block (after line 1335), add oracle client initialization:

```go
	// Wire oracle client if configured
	oracleEnabled := cast.ToBool(appOpts.Get("oracle.enabled"))
	if oracleEnabled {
		oracleEndpoint := cast.ToString(appOpts.Get("oracle.endpoint"))
		oracleTimeout := cast.ToDuration(appOpts.Get("oracle.timeout"))
		oracleMinConf := cast.ToFloat64(appOpts.Get("oracle.min-confidence"))
		if oracleEndpoint != "" {
			logger.Info("oracle sidecar enabled",
				"endpoint", oracleEndpoint,
				"timeout", oracleTimeout,
				"min_confidence", oracleMinConf,
			)
			oracleClient := NewHTTPOracleClient(oracleEndpoint, oracleTimeout, oracleMinConf)
			// Store on app for VoteExtConfig to use when it's initialized
			app.oracleClient = oracleClient
		}
	}
```

This requires adding `oracleClient OracleClient` field to the `ZeroneApp` struct (after `VoteExtConfig` at line 474):

```go
	// ABCI++ vote extension config (nil until validator is configured)
	VoteExtConfig *VoteExtensionConfig

	// Oracle client for querying the evaluation sidecar (nil if disabled)
	oracleClient OracleClient
```

And when `VoteExtConfig` is set (wherever the validator initialization happens), the oracle client should be copied over:

```go
// In any code that sets VoteExtConfig:
if app.oracleClient != nil {
    app.VoteExtConfig.OracleClient = app.oracleClient
}
```

**Note:** Since `VoteExtConfig` is currently only set in tests, the oracle client field on `ZeroneApp` acts as a holding spot until a validator configures itself. The important thing is the config is parsed and the client is ready.

**Step 3: Verify build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./cmd/zeroned/...`
Expected: Build succeeds

**Step 4: Verify existing tests pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./app/... -v -timeout 120s`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add cmd/zeroned/cmd/config.go app/app.go
git commit -m "feat(oracle): add [oracle] config section to app.toml and wire client in app startup"
```

---

### Task 8: Vote Extension Oracle Tests

**Files:**
- Modify: `app/vote_extensions_test.go`

**Step 1: Add oracle-specific test cases to the existing test file**

Append these tests to `app/vote_extensions_test.go`:

```go
// ---------- Oracle integration tests ----------

// mockOracleClient implements OracleClient for testing.
type mockOracleClient struct {
	verdict    string
	confidence float64
	err        error
}

func (m *mockOracleClient) Evaluate(claim, domain, claimType string) (*zeroneapp.OracleEvaluation, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &zeroneapp.OracleEvaluation{
		Verdict:    m.verdict,
		Confidence: m.confidence,
		Reasoning:  "test",
	}, nil
}

func TestEvaluateWithOracle_Accept(t *testing.T) {
	client := &mockOracleClient{verdict: "accept", confidence: 0.9}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(900_000), confidence)
}

func TestEvaluateWithOracle_Reject(t *testing.T) {
	client := &mockOracleClient{verdict: "reject", confidence: 0.85}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "reject", verdict)
	require.Equal(t, uint64(850_000), confidence)
}

func TestEvaluateWithOracle_Uncertain(t *testing.T) {
	client := &mockOracleClient{verdict: "uncertain", confidence: 0.5}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict, "uncertain should default to accept")
	require.Equal(t, uint64(500_000), confidence)
}

func TestEvaluateWithOracle_Error(t *testing.T) {
	client := &mockOracleClient{err: fmt.Errorf("connection refused")}
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict, "error should default to accept")
	require.Equal(t, uint64(600_000), confidence, "error should use default confidence")
}

func TestEvaluateWithOracle_NilClient(t *testing.T) {
	verdict, confidence := zeroneapp.EvaluateWithOracle(nil, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(600_000), confidence)
}
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./app/... -run TestEvaluateWithOracle -v`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add app/vote_extensions_test.go
git commit -m "test(oracle): add oracle integration tests for vote extension evaluation"
```

---

### Task 9: Documentation

**Files:**
- Create: `docs/validator-oracle.md`

**Step 1: Write the operator documentation**

```markdown
# Validator Evaluation Oracle

## What It Does

The **zerone-oracle** is an advisory sidecar process that helps validators make informed decisions when voting on knowledge claims. Instead of blindly accepting all claims, validators can consult the oracle for a fact-checking verdict before casting their vote.

**Important:** The oracle is purely advisory. It does not participate in consensus. If the oracle is down, slow, or misconfigured, the validator falls back to default behavior (accept all claims). Oracle failure never blocks the chain.

## How It Works

The oracle provides two evaluation tiers:

### Tier 1: Static (No External Dependencies)

Checks new claims against the 777 genesis axioms embedded in the binary:
- **Numerical contradiction:** Extracts numbers from the claim and matching axioms, flags mismatches (e.g., "speed of light is 100 m/s" contradicts the axiom stating ~3×10⁸ m/s)
- **Explicit negation:** Detects negation words ("not", "never", "false") that reverse the meaning of a matching axiom

**Limitations:** Tier 1 does NOT catch semantic contradictions (e.g., "the sun orbits the earth"), paraphrased falsehoods, or claims in domains with no matching axioms. It is a simple filter, not a general fact-checker.

### Tier 2: LLM (Requires API Key)

Sends the claim to Claude for fact-checking with a structured prompt. Returns a verdict with confidence and reasoning. Results are cached (LRU, 1000 entries) to avoid repeated API calls.

**Timeout:** LLM calls have a strict 2-second timeout. If Claude doesn't respond in time, the oracle returns "uncertain."

## Quick Start

### 1. Build the oracle

```bash
cd /path/to/zerone
go build -o build/zerone-oracle ./cmd/oracle/
```

### 2. Start the sidecar (Tier 1 only)

```bash
./build/zerone-oracle --port 8081 --tier static
```

### 3. Start the sidecar (with LLM)

```bash
./build/zerone-oracle --port 8081 --tier llm \
  --llm-api-key "sk-ant-..." \
  --llm-model "claude-sonnet-4-5-20250514"
```

### 4. Enable in validator config

Add to your `app.toml`:

```toml
[oracle]
enabled = true
endpoint = "http://localhost:8081"
timeout = "2s"
min-confidence = 0.6
```

### 5. Restart your validator

The validator will now query the oracle during vote extension commit phase.

## Configuration Reference

### Oracle Sidecar Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8081` | HTTP server port |
| `--tier` | `static` | Evaluation tier: `static` or `llm` |
| `--llm-api-key` | (empty) | Anthropic API key (required for `llm` tier) |
| `--llm-api-url` | `https://api.anthropic.com` | Anthropic API base URL |
| `--llm-model` | `claude-sonnet-4-5-20250514` | Model ID for LLM evaluation |
| `--llm-max-tokens` | `500` | Max response tokens |
| `--llm-timeout` | `2s` | LLM API call timeout |

### Validator app.toml

| Key | Default | Description |
|-----|---------|-------------|
| `oracle.enabled` | `false` | Enable oracle queries |
| `oracle.endpoint` | `http://localhost:8081` | Oracle sidecar URL |
| `oracle.timeout` | `2s` | Max wait for oracle response |
| `oracle.min-confidence` | `0.6` | Minimum confidence to act on verdict |

## API Endpoints

### POST /evaluate

Evaluate a claim.

```json
Request:  {"claim": "Water boils at 100°C", "domain": "physics", "claim_type": "assertion"}
Response: {"verdict": "accept", "confidence": 0.75, "reasoning": "consistent with axiom PHYS-..."}
```

### POST /prefetch

Pre-warm the cache for an upcoming evaluation. Returns immediately (HTTP 202).

### GET /health

Health check.

```json
Response: {"status": "ok", "tier": "static"}
```

## Safety

- **Oracle down:** Validator falls back to `accept` with default confidence (600,000 BPS). No consensus impact.
- **Oracle slow:** 2-second hard timeout. Falls back to default.
- **Oracle wrong:** Verdicts below the confidence threshold (default 0.6) are treated as "uncertain" → accept.
- **Oracle disabled:** `oracle.enabled = false` (default). Zero behavior change from pre-oracle code.

## Performance

- Static evaluation: <1ms per claim
- LLM evaluation: 500ms-2s (first call), <1ms (cache hit)
- Cache pre-warming via `/prefetch` recommended for LLM tier
- Localhost HTTP overhead: ~1ms
```

**Step 2: Commit**

```bash
git add docs/validator-oracle.md
git commit -m "docs: add validator oracle operator documentation"
```

---

### Task 10: Build & Full Test Verification

**Step 1: Build both binaries**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./cmd/zeroned/... && go build -o ./build/zerone-oracle ./cmd/oracle/`
Expected: Both build successfully

**Step 2: Run all tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./app/... ./cmd/oracle/... -v -timeout 120s`
Expected: ALL PASS

**Step 3: Smoke test the sidecar**

Run:
```bash
./build/zerone-oracle --port 8081 --tier static &
ORACLE_PID=$!
sleep 1
curl -s http://localhost:8081/health | jq .
curl -s -X POST http://localhost:8081/evaluate \
  -H 'Content-Type: application/json' \
  -d '{"claim":"The speed of light in vacuum is 100 m/s","domain":"physics"}' | jq .
curl -s -X POST http://localhost:8081/evaluate \
  -H 'Content-Type: application/json' \
  -d '{"claim":"Electromagnetic waves propagate at the speed of light","domain":"physics"}' | jq .
kill $ORACLE_PID
```

Expected:
- Health: `{"status":"ok","tier":"static"}`
- False claim: `verdict: "reject"` with confidence > 0.5
- True claim: `verdict: "accept"` or `"uncertain"`

**Step 4: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix(oracle): address issues found during verification"
```
