package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/zerone-chain/zerone/services/benchmark/domains"
)

// Runner orchestrates benchmark execution against an inference endpoint.
type Runner struct {
	endpoint  string
	model     string
	evaluator *Evaluator
	timeout   time.Duration
	workers   int
}

// NewRunner creates a benchmark runner.
func NewRunner(endpoint, model string, timeout time.Duration, workers int) *Runner {
	return &Runner{
		endpoint:  endpoint,
		model:     model,
		evaluator: NewEvaluator(endpoint, model, timeout),
		timeout:   timeout,
		workers:   workers,
	}
}

// Run executes all benchmark cases and returns a report.
func (r *Runner) Run(cases []domains.BenchCase, datasetVersion string) (*domains.BenchmarkReport, error) {
	results := r.runCases(cases)

	reporter := NewReporter()
	report := reporter.Aggregate(results, r.model, datasetVersion)

	return report, nil
}

// RunDomain executes benchmark cases filtered to a specific domain.
func (r *Runner) RunDomain(domain string, cases []domains.BenchCase) ([]domains.CaseResult, error) {
	filtered := make([]domains.BenchCase, 0)
	for _, c := range cases {
		if c.Domain == domain {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no cases found for domain %q", domain)
	}
	return r.runCases(filtered), nil
}

// runCases executes benchmark cases with concurrent workers.
func (r *Runner) runCases(cases []domains.BenchCase) []domains.CaseResult {
	results := make([]domains.CaseResult, len(cases))

	workers := r.workers
	if workers <= 0 {
		workers = 1
	}
	if workers > len(cases) {
		workers = len(cases)
	}

	// Channel-based work distribution
	work := make(chan int, len(cases))
	for i := range cases {
		work <- i
	}
	close(work)

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for idx := range work {
				results[idx] = r.runSingleCase(cases[idx])
			}
		}()
	}

	wg.Wait()
	return results
}

// runSingleCase executes a single benchmark case.
func (r *Runner) runSingleCase(c domains.BenchCase) domains.CaseResult {
	result := domains.CaseResult{
		CaseID:   c.ID,
		Domain:   c.Domain,
		Category: c.Category,
		Prompt:   c.Prompt,
		Expected: c.Expected,
	}

	// Call the inference endpoint
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	response, err := callEndpoint(ctx, r.endpoint, r.model, c.Prompt)
	elapsed := time.Since(start)
	result.LatencyMs = elapsed.Milliseconds()

	if err != nil {
		result.Score = 0
		result.Pass = false
		result.Details = fmt.Sprintf("endpoint error: %v", err)
		log.Printf("[FAIL] %s: %s", c.ID, result.Details)
		return result
	}

	result.Response = response

	// Evaluate the response
	score, details, err := r.evaluator.Evaluate(c, response)
	if err != nil {
		result.Score = 0
		result.Pass = false
		result.Details = fmt.Sprintf("evaluation error: %v", err)
		log.Printf("[FAIL] %s: %s", c.ID, result.Details)
		return result
	}

	result.Score = score
	result.Pass = score >= 0.5
	result.Details = details

	status := "PASS"
	if !result.Pass {
		status = "FAIL"
	}
	log.Printf("[%s] %s: score=%.2f %s (%dms)", status, c.ID, score, details, result.LatencyMs)

	return result
}
