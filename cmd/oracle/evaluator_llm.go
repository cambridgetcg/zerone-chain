package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	mu        sync.Mutex
	cache     map[string]*EvaluateResponse
}

// NewLLMEvaluator creates a new LLM evaluator backed by the Claude Messages API.
// If apiURL is empty, the default Anthropic URL is used.
func NewLLMEvaluator(apiURL, apiKey, model string, maxTokens int, timeout time.Duration) *LLMEvaluator {
	if apiURL == "" {
		apiURL = defaultAnthropicURL
	}
	return &LLMEvaluator{
		apiURL:    apiURL,
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		timeout:   timeout,
		client:    &http.Client{Timeout: timeout},
		cache:     make(map[string]*EvaluateResponse),
	}
}

// Name returns the evaluator name.
func (l *LLMEvaluator) Name() string { return "llm" }

// claudeRequest is the request body sent to the Claude Messages API.
type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

// claudeMessage represents a single message in the Claude conversation.
type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeResponse is the response from the Claude Messages API.
type claudeResponse struct {
	Content []claudeContent `json:"content"`
}

// claudeContent represents one content block in a Claude response.
type claudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// llmVerdict is the structured JSON we ask the LLM to return.
type llmVerdict struct {
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

const systemPrompt = `You are a fact-checking oracle for the Zerone knowledge network. Your job is to evaluate whether a claim is factually accurate.

You MUST respond with a single JSON object (no markdown, no extra text):
{"verdict":"accept|reject|uncertain","confidence":0.0-1.0,"reasoning":"..."}

Rules:
- "accept" means the claim is factually accurate.
- "reject" means the claim is factually inaccurate.
- "uncertain" means you cannot determine accuracy with confidence.
- confidence is a float between 0.0 and 1.0 indicating your certainty.
- reasoning is a brief explanation of your verdict.`

// Evaluate sends the claim to the Claude API and returns a parsed verdict.
func (l *LLMEvaluator) Evaluate(req EvaluateRequest) (*EvaluateResponse, error) {
	// 1. Check cache.
	key := cacheKey(req.Claim, req.Domain)
	l.mu.Lock()
	if cached, ok := l.cache[key]; ok {
		l.mu.Unlock()
		return cached, nil
	}
	l.mu.Unlock()

	// 2. Build the API request.
	apiReq := claudeRequest{
		Model:     l.model,
		MaxTokens: l.maxTokens,
		System:    systemPrompt,
		Messages: []claudeMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf("Domain: %s\nClaim: %s", req.Domain, req.Claim),
			},
		},
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}

	// 3. Make HTTP request.
	httpReq, err := http.NewRequest(http.MethodPost, l.apiURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", l.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm: read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm: API returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// 4. Parse Claude response.
	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("llm: unmarshal response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("llm: empty content in response")
	}

	text := claudeResp.Content[0].Text

	// 5. Extract JSON from response text (handle potential markdown wrapping).
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		return nil, fmt.Errorf("llm: no JSON found in response: %s", text)
	}

	var verdict llmVerdict
	if err := json.Unmarshal([]byte(jsonStr), &verdict); err != nil {
		return nil, fmt.Errorf("llm: parse verdict JSON: %w", err)
	}

	// 6. Validate and normalize verdict.
	switch verdict.Verdict {
	case "accept", "reject", "uncertain":
		// valid
	default:
		verdict.Verdict = "uncertain"
	}

	// 7. Clamp confidence to [0, 1].
	if verdict.Confidence < 0 {
		verdict.Confidence = 0
	}
	if verdict.Confidence > 1 {
		verdict.Confidence = 1
	}

	result := &EvaluateResponse{
		Verdict:    verdict.Verdict,
		Confidence: verdict.Confidence,
		Reasoning:  verdict.Reasoning,
	}

	// 8. Cache result with simple eviction.
	l.mu.Lock()
	if len(l.cache) > 1000 {
		// Evict roughly half the cache.
		count := 0
		for k := range l.cache {
			delete(l.cache, k)
			count++
			if count >= len(l.cache)/2+1 {
				break
			}
		}
	}
	l.cache[key] = result
	l.mu.Unlock()

	return result, nil
}

// Prefetch starts an asynchronous evaluation to pre-warm the cache.
func (l *LLMEvaluator) Prefetch(req EvaluateRequest) {
	go func() {
		_, _ = l.Evaluate(req)
	}()
}

// cacheKey returns a hex-encoded SHA-256 hash of "domain|claim".
func cacheKey(claim, domain string) string {
	h := sha256.Sum256([]byte(domain + "|" + claim))
	return hex.EncodeToString(h[:])
}

// extractJSON finds the first JSON object in text by locating the first '{'
// and the last '}', handling potential markdown code block wrapping.
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(text, "}")
	if end < start {
		return ""
	}
	return text[start : end+1]
}
