package main

// StaticEvaluator checks claims against genesis axioms.
// Stubbed: axioms were removed in the training data protocol pivot (R36-5).
type StaticEvaluator struct{}

// NewStaticEvaluator returns a stubbed evaluator (axioms removed in R36-5).
func NewStaticEvaluator() (*StaticEvaluator, error) {
	return &StaticEvaluator{}, nil
}

// Name returns the evaluator strategy name.
func (se *StaticEvaluator) Name() string { return "static" }

// AxiomCount returns 0 (axioms removed).
func (se *StaticEvaluator) AxiomCount() int { return 0 }

// Evaluate always returns uncertain (no axioms to check against).
func (se *StaticEvaluator) Evaluate(_ EvaluateRequest) (*EvaluateResponse, error) {
	return &EvaluateResponse{
		Verdict:    "uncertain",
		Confidence: 0.5,
		Reasoning:  "static axiom evaluator disabled (training data protocol)",
	}, nil
}
