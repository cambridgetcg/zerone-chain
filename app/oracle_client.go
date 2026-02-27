package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OracleEvaluation represents the evaluation result from the oracle sidecar.
type OracleEvaluation struct {
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// OracleClient defines the interface for evaluating claims via an oracle.
type OracleClient interface {
	Evaluate(claim, domain, claimType string) (*OracleEvaluation, error)
}

// HTTPOracleClient implements OracleClient by calling an HTTP oracle sidecar.
type HTTPOracleClient struct {
	endpoint      string
	timeout       time.Duration
	minConfidence float64
	client        *http.Client
}

// NewHTTPOracleClient creates a new HTTPOracleClient with the given endpoint,
// timeout, and minimum confidence threshold.
func NewHTTPOracleClient(endpoint string, timeout time.Duration, minConfidence float64) *HTTPOracleClient {
	return &HTTPOracleClient{
		endpoint:      endpoint,
		timeout:       timeout,
		minConfidence: minConfidence,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Evaluate sends a claim to the oracle sidecar for evaluation.
//
// The method POSTs a JSON payload to {endpoint}/evaluate and decodes the
// response into an OracleEvaluation. If the oracle's confidence falls below
// the configured minConfidence threshold and the verdict is not already
// "uncertain", the verdict is overridden to "uncertain" with a note appended
// to the reasoning.
func (c *HTTPOracleClient) Evaluate(claim, domain, claimType string) (*OracleEvaluation, error) {
	reqBody := map[string]string{
		"claim":      claim,
		"domain":     domain,
		"claim_type": claimType,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("oracle: failed to marshal request: %w", err)
	}

	resp, err := c.client.Post(c.endpoint+"/evaluate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("oracle: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oracle: non-200 status: %d", resp.StatusCode)
	}

	var result OracleEvaluation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("oracle: failed to decode response: %w", err)
	}

	// Confidence threshold: override verdict to "uncertain" if confidence is
	// below the minimum and the verdict is not already "uncertain".
	if result.Confidence < c.minConfidence && result.Verdict != "uncertain" {
		result.Verdict = "uncertain"
		result.Reasoning = fmt.Sprintf("%s (below minimum confidence threshold %.2f)", result.Reasoning, c.minConfidence)
	}

	return &result, nil
}

// EvaluateWithOracle calls the oracle client and maps the result to a verdict
// string and uint64 confidence value suitable for vote extensions.
//
// If the client is nil or an error occurs, safe defaults are returned:
// verdict="accept", confidence=600_000.
//
// The "uncertain" verdict is mapped to "accept" (with the oracle's confidence).
// Confidence is scaled from float64 [0,1] to uint64 [0,1_000_000] and capped.
func EvaluateWithOracle(client OracleClient, claim, domain, claimType string) (string, uint64) {
	const (
		defaultVerdict    = "accept"
		defaultConfidence = uint64(600_000)
	)

	if client == nil {
		return defaultVerdict, defaultConfidence
	}

	result, err := client.Evaluate(claim, domain, claimType)
	if err != nil {
		return defaultVerdict, defaultConfidence
	}

	// Map confidence: float64 [0,1] -> uint64 [0,1_000_000], capped at 1_000_000.
	conf := uint64(result.Confidence * 1_000_000)
	if conf > 1_000_000 {
		conf = 1_000_000
	}

	// Map verdict: "uncertain" -> "accept" (with oracle's confidence).
	verdict := result.Verdict
	if verdict == "uncertain" {
		verdict = "accept"
	}

	return verdict, conf
}
