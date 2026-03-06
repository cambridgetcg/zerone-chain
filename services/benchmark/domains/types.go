package domains

// BenchCase represents a single benchmark test case.
type BenchCase struct {
	ID         string         `json:"id"`
	Domain     string         `json:"domain"`
	Category   string         `json:"category"`
	Prompt     string         `json:"prompt"`
	Expected   string         `json:"expected"`
	EvalMethod string         `json:"eval_method"` // exact, fuzzy, execution, llm_judge
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// CaseResult holds the evaluation result for a single case.
type CaseResult struct {
	CaseID    string  `json:"case_id"`
	Domain    string  `json:"domain"`
	Category  string  `json:"category"`
	Prompt    string  `json:"prompt"`
	Response  string  `json:"response"`
	Expected  string  `json:"expected"`
	Score     float64 `json:"score"`
	Pass      bool    `json:"pass"`
	Details   string  `json:"details"`
	LatencyMs int64   `json:"latency_ms"`
}

// BenchmarkReport is the full output report for a benchmark run.
type BenchmarkReport struct {
	ModelVersion   string             `json:"model_version"`
	DatasetVersion string             `json:"dataset_version"`
	Timestamp      string             `json:"timestamp"`
	OverallScore   float64            `json:"overall_score"`
	DomainScores   map[string]float64 `json:"domain_scores"`
	PerCaseResults []CaseResult       `json:"per_case_results"`
}
