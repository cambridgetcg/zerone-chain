package app_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"
)

// TestOracleClient_Evaluate verifies that HTTPOracleClient correctly calls
// the /evaluate endpoint and parses a reject@0.85 response.
func TestOracleClient_Evaluate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/evaluate", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		// Decode the request body to verify it.
		var reqBody map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&reqBody))
		require.Equal(t, "the earth is round", reqBody["claim"])
		require.Equal(t, "science", reqBody["domain"])
		require.Equal(t, "factual", reqBody["claim_type"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(zeroneapp.OracleEvaluation{
			Verdict:    "reject",
			Confidence: 0.85,
			Reasoning:  "claim is incorrect",
		})
	}))
	defer srv.Close()

	client := zeroneapp.NewHTTPOracleClient(srv.URL, 5*time.Second, 0.5)
	eval, err := client.Evaluate("the earth is round", "science", "factual")
	require.NoError(t, err)
	require.Equal(t, "reject", eval.Verdict)
	require.InDelta(t, 0.85, eval.Confidence, 0.001)
	require.Equal(t, "claim is incorrect", eval.Reasoning)
}

// TestOracleClient_Timeout verifies that the client returns an error when
// the oracle sidecar takes longer than the configured timeout.
func TestOracleClient_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := zeroneapp.NewHTTPOracleClient(srv.URL, 500*time.Millisecond, 0.5)
	_, err := client.Evaluate("test claim", "test", "factual")
	require.Error(t, err, "should timeout before receiving response")
}

// TestOracleClient_ErrorFallback verifies that a non-200 HTTP status from the
// oracle sidecar is surfaced as an error.
func TestOracleClient_ErrorFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "internal error")
	}))
	defer srv.Close()

	client := zeroneapp.NewHTTPOracleClient(srv.URL, 5*time.Second, 0.5)
	_, err := client.Evaluate("test claim", "test", "factual")
	require.Error(t, err, "should return error on HTTP 500")
}

// TestOracleClient_LowConfidenceToUncertain verifies that when the oracle
// returns a verdict with confidence below the minConfidence threshold, the
// verdict is overridden to "uncertain" (unless it was already "uncertain").
func TestOracleClient_LowConfidenceToUncertain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(zeroneapp.OracleEvaluation{
			Verdict:    "reject",
			Confidence: 0.3,
			Reasoning:  "low confidence rejection",
		})
	}))
	defer srv.Close()

	client := zeroneapp.NewHTTPOracleClient(srv.URL, 5*time.Second, 0.6)
	eval, err := client.Evaluate("test claim", "test", "factual")
	require.NoError(t, err)
	require.Equal(t, "uncertain", eval.Verdict, "low confidence should override verdict to uncertain")
	require.InDelta(t, 0.3, eval.Confidence, 0.001)
	require.Contains(t, eval.Reasoning, "below minimum confidence threshold")
}

// TestEvaluateWithOracle_NilClient verifies that EvaluateWithOracle returns
// safe defaults when the oracle client is nil.
func TestEvaluateWithOracle_NilClient(t *testing.T) {
	verdict, confidence := zeroneapp.EvaluateWithOracle(nil, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(600_000), confidence)
}

// TestEvaluateWithOracle_UncertainMapsToAccept verifies that an "uncertain"
// verdict from the oracle maps to "accept" with the oracle's confidence value.
func TestEvaluateWithOracle_UncertainMapsToAccept(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(zeroneapp.OracleEvaluation{
			Verdict:    "uncertain",
			Confidence: 0.45,
			Reasoning:  "not sure",
		})
	}))
	defer srv.Close()

	client := zeroneapp.NewHTTPOracleClient(srv.URL, 5*time.Second, 0.0)
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict, "uncertain should map to accept")
	require.Equal(t, uint64(450_000), confidence)
}

// TestEvaluateWithOracle_ConfidenceCap verifies that confidence is capped at 1_000_000.
func TestEvaluateWithOracle_ConfidenceCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(zeroneapp.OracleEvaluation{
			Verdict:    "accept",
			Confidence: 1.5, // above 1.0
			Reasoning:  "very confident",
		})
	}))
	defer srv.Close()

	client := zeroneapp.NewHTTPOracleClient(srv.URL, 5*time.Second, 0.0)
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(1_000_000), confidence, "confidence should be capped at 1_000_000")
}

// TestEvaluateWithOracle_ErrorReturnsDefaults verifies that an oracle error
// causes EvaluateWithOracle to return the safe defaults.
func TestEvaluateWithOracle_ErrorReturnsDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := zeroneapp.NewHTTPOracleClient(srv.URL, 5*time.Second, 0.5)
	verdict, confidence := zeroneapp.EvaluateWithOracle(client, "claim", "domain", "type")
	require.Equal(t, "accept", verdict)
	require.Equal(t, uint64(600_000), confidence)
}
