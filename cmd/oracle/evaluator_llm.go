package main

// LLMEvaluator is a stub for the LLM-based evaluator (Tier 2).
// The full implementation will be added in a later task.
type LLMEvaluator struct{}

// Evaluate is a placeholder that always returns uncertain.
func (l *LLMEvaluator) Evaluate(req EvaluateRequest) (*EvaluateResponse, error) {
	return &EvaluateResponse{
		Verdict:    "uncertain",
		Confidence: 0.5,
		Reasoning:  "LLM evaluator not yet implemented",
	}, nil
}

// Name returns the evaluator name.
func (l *LLMEvaluator) Name() string { return "llm" }
