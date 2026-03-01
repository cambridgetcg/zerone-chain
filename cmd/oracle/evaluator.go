package main

// EvaluateRequest is the input to an Evaluator.
type EvaluateRequest struct {
	Claim     string `json:"claim"`
	Domain    string `json:"domain"`
	ClaimType string `json:"claim_type"`
}

// EvaluateResponse is the output from an Evaluator.
type EvaluateResponse struct {
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// Evaluator is the interface every evaluation strategy must implement.
type Evaluator interface {
	Evaluate(req EvaluateRequest) (*EvaluateResponse, error)
	Name() string
}

// CombinedEvaluator orchestrates static and LLM evaluation.
// Static evaluation runs first; LLM is used only when the static
// result is uncertain and an LLM evaluator is available.
type CombinedEvaluator struct {
	Static *StaticEvaluator
	LLM    *LLMEvaluator
}

// Evaluate implements the combined evaluation strategy:
//  1. Always run static first.
//  2. On static error, treat as uncertain (0.5 confidence).
//  3. High-confidence static verdict (>0.7, accept OR reject) → short-circuit.
//  4. Static uncertain + LLM available → use LLM (on LLM error, fall back to static).
//  5. Static uncertain + no LLM → return static result.
func (c *CombinedEvaluator) Evaluate(req EvaluateRequest) (*EvaluateResponse, error) {
	// Step 1: always run static first.
	staticResp, staticErr := c.Static.Evaluate(req)

	// Step 2: on static error, synthesize an uncertain result.
	if staticErr != nil {
		staticResp = &EvaluateResponse{
			Verdict:    "uncertain",
			Confidence: 0.5,
			Reasoning:  "static evaluation failed: " + staticErr.Error(),
		}
	}

	// Step 3: high-confidence static verdict → short-circuit.
	if staticResp.Confidence > 0.7 && (staticResp.Verdict == "accept" || staticResp.Verdict == "reject") {
		return staticResp, nil
	}

	// Step 4: static uncertain + LLM available → delegate to LLM.
	if c.LLM != nil {
		llmResp, llmErr := c.LLM.Evaluate(req)
		if llmErr == nil {
			return llmResp, nil
		}
		// LLM error → fall back to static result (step 4 fallback).
	}

	// Step 5: static uncertain + no LLM (or LLM failed) → return static.
	return staticResp, nil
}

// Name returns the evaluator name.
func (c *CombinedEvaluator) Name() string {
	return "combined"
}
