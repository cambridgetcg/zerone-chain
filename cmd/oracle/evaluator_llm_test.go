package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockClaudeResponse builds a minimal Claude Messages API response body
// containing the given text content.
func mockClaudeResponse(text string) []byte {
	resp := map[string]interface{}{
		"id":    "msg_test",
		"type":  "message",
		"role":  "assistant",
		"model": "claude-3-haiku-20240307",
		"content": []map[string]interface{}{
			{"type": "text", "text": text},
		},
		"stop_reason": "end_turn",
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestLLMEvaluator_ParsesResponse(t *testing.T) {
	var receivedAPIKey string
	var receivedVersion string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAPIKey = r.Header.Get("x-api-key")
		receivedVersion = r.Header.Get("anthropic-version")

		// Verify it's a POST to /v1/messages.
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/messages", r.URL.Path)

		// Verify request body is valid JSON with expected fields.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		defer r.Body.Close()

		var reqBody map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &reqBody))
		require.Contains(t, reqBody, "model")
		require.Contains(t, reqBody, "messages")

		verdict := `{"verdict":"accept","confidence":0.92,"reasoning":"The claim is well-supported by evidence."}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockClaudeResponse(verdict))
	}))
	defer srv.Close()

	eval := NewLLMEvaluator(srv.URL, "test-api-key-123", "claude-3-haiku-20240307", 1024, 5*time.Second)

	resp, err := eval.Evaluate(EvaluateRequest{
		Claim:  "Water boils at 100 degrees Celsius at sea level",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "accept", resp.Verdict)
	require.InDelta(t, 0.92, resp.Confidence, 0.001)
	require.Contains(t, resp.Reasoning, "well-supported")

	// Verify API key was sent in header.
	require.Equal(t, "test-api-key-123", receivedAPIKey)
	require.Equal(t, "2023-06-01", receivedVersion)
}

func TestLLMEvaluator_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than evaluator timeout.
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write(mockClaudeResponse(`{"verdict":"accept","confidence":0.9,"reasoning":"ok"}`))
	}))
	defer srv.Close()

	eval := NewLLMEvaluator(srv.URL, "test-key", "claude-3-haiku-20240307", 1024, 500*time.Millisecond)

	_, err := eval.Evaluate(EvaluateRequest{
		Claim:  "The Earth orbits the Sun",
		Domain: "astronomy",
	})
	require.Error(t, err, "should fail due to timeout")
}

func TestLLMEvaluator_CacheHit(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		verdict := `{"verdict":"reject","confidence":0.85,"reasoning":"contradicts known facts"}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockClaudeResponse(verdict))
	}))
	defer srv.Close()

	eval := NewLLMEvaluator(srv.URL, "test-key", "claude-3-haiku-20240307", 1024, 5*time.Second)

	req := EvaluateRequest{
		Claim:  "The moon is made of cheese",
		Domain: "astronomy",
	}

	// First call hits the server.
	resp1, err := eval.Evaluate(req)
	require.NoError(t, err)
	require.Equal(t, "reject", resp1.Verdict)
	require.Equal(t, int32(1), callCount.Load(), "first call should hit server")

	// Second call with same input should use cache.
	resp2, err := eval.Evaluate(req)
	require.NoError(t, err)
	require.Equal(t, "reject", resp2.Verdict)
	require.InDelta(t, 0.85, resp2.Confidence, 0.001)
	require.Equal(t, int32(1), callCount.Load(), "second call should NOT hit server (cache hit)")
}

func TestLLMEvaluator_CacheKeyIncludesDomain(t *testing.T) {
	k1 := cacheKey("claim", "physics")
	k2 := cacheKey("claim", "biology")
	require.NotEqual(t, k1, k2, "cache keys for same claim but different domains must differ")

	// Also verify determinism.
	k3 := cacheKey("claim", "physics")
	require.Equal(t, k1, k3, "cache key must be deterministic")
}
