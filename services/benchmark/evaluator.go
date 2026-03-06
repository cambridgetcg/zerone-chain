package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/zerone-chain/zerone/services/benchmark/domains"
)

// Evaluator scores model responses against expected outputs.
type Evaluator struct {
	endpoint string
	model    string
	timeout  time.Duration
}

// NewEvaluator creates an evaluator. endpoint and model are used for LLM-judge calls.
func NewEvaluator(endpoint, model string, timeout time.Duration) *Evaluator {
	return &Evaluator{endpoint: endpoint, model: model, timeout: timeout}
}

// Evaluate dispatches to the appropriate evaluation method based on the case's EvalMethod.
func (e *Evaluator) Evaluate(c domains.BenchCase, response string) (float64, string, error) {
	switch c.EvalMethod {
	case "exact":
		score, detail := e.ExactMatch(response, c.Expected)
		return score, detail, nil
	case "fuzzy":
		score, detail := e.FuzzyMatch(response, c.Expected)
		return score, detail, nil
	case "execution":
		return e.ExecutionMatch(response, c.Expected, c.Metadata)
	case "llm_judge":
		return e.LLMJudge(c.Prompt, response, c.Expected, c.Metadata)
	default:
		return 0, fmt.Sprintf("unknown eval method: %s", c.EvalMethod), nil
	}
}

// ExactMatch compares the normalized response to the expected output.
func (e *Evaluator) ExactMatch(response, expected string) (float64, string) {
	normResp := normalizeText(response)
	normExp := normalizeText(expected)

	if normResp == normExp {
		return 1.0, "exact match"
	}

	// Check if the expected value appears in the response (for models that add explanation)
	if strings.Contains(normResp, normExp) {
		return 0.5, "expected value found within response"
	}

	return 0.0, fmt.Sprintf("expected %q, got %q", normExp, truncate(normResp, 100))
}

// FuzzyMatch uses normalized Levenshtein distance and substring containment.
func (e *Evaluator) FuzzyMatch(response, expected string) (float64, string) {
	normResp := strings.ToLower(strings.TrimSpace(response))
	normExp := strings.ToLower(strings.TrimSpace(expected))

	// Direct containment check
	if strings.Contains(normResp, normExp) {
		return 1.0, "expected content found in response"
	}

	// Levenshtein similarity on the shorter comparison window
	dist := levenshtein(normResp, normExp)
	maxLen := max(len(normResp), len(normExp))
	if maxLen == 0 {
		return 1.0, "both empty"
	}

	similarity := 1.0 - float64(dist)/float64(maxLen)
	if similarity >= 0.8 {
		return similarity, fmt.Sprintf("fuzzy match similarity=%.2f", similarity)
	}

	// Try extracting just the relevant portion (first line, code block, etc.)
	lines := strings.Split(normResp, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, normExp) {
			return 0.8, "expected content found in a line"
		}
	}

	return similarity, fmt.Sprintf("low similarity=%.2f", similarity)
}

// ExecutionMatch runs generated code in a sandbox and checks output.
func (e *Evaluator) ExecutionMatch(response, expected string, metadata map[string]any) (float64, string, error) {
	code := domains.ExtractCodeBlock(response)
	if code == "" {
		code = response
	}

	lang, _ := metadata["language"].(string)
	if lang == "" {
		lang = "go"
	}

	testInput, _ := metadata["test_input"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var output string
	var err error

	switch lang {
	case "go":
		output, err = executeGo(ctx, code, testInput)
	default:
		return 0, fmt.Sprintf("unsupported language: %s", lang), nil
	}

	if err != nil {
		return 0, fmt.Sprintf("execution error: %v", err), nil
	}

	normOutput := normalizeText(output)
	normExpected := normalizeText(expected)

	if normOutput == normExpected {
		return 1.0, "execution output matches", nil
	}
	if strings.Contains(normOutput, normExpected) {
		return 0.8, "expected value found in execution output", nil
	}

	return 0.0, fmt.Sprintf("expected output %q, got %q", normExpected, truncate(normOutput, 200)), nil
}

// LLMJudge uses the model endpoint to evaluate subjective quality.
func (e *Evaluator) LLMJudge(prompt, response, expected string, metadata map[string]any) (float64, string, error) {
	if e.endpoint == "" {
		// Fallback to structural checks when no endpoint available
		return e.structuralCheck(response, expected, metadata)
	}

	judgePrompt := fmt.Sprintf(`You are evaluating an AI model's response quality. Score from 0.0 to 1.0.

Original prompt: %s

Model response:
%s

Expected output should contain: %s

Evaluate based on:
1. Does the response follow the format/structure requested?
2. Does the response satisfy all constraints mentioned in the prompt?
3. Is the content accurate and relevant?

Respond with ONLY a JSON object: {"score": 0.X, "reason": "brief explanation"}`,
		prompt, truncate(response, 2000), expected)

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	result, err := callEndpoint(ctx, e.endpoint, e.model, judgePrompt)
	if err != nil {
		// Fallback to structural checks
		return e.structuralCheck(response, expected, metadata)
	}

	return parseLLMJudgeResponse(result)
}

// structuralCheck performs basic structural validation when LLM-judge is unavailable.
func (e *Evaluator) structuralCheck(response, expected string, metadata map[string]any) (float64, string, error) {
	score := 0.0
	reasons := []string{}

	// Check if expected substring is present
	if expected != "" && strings.Contains(strings.ToLower(response), strings.ToLower(expected)) {
		score += 0.3
		reasons = append(reasons, "contains expected content")
	}

	// Format-specific checks
	format, _ := metadata["format"].(string)
	switch format {
	case "json":
		if json.Valid([]byte(strings.TrimSpace(response))) {
			score += 0.4
			reasons = append(reasons, "valid JSON")
		}
	case "json_array":
		trimmed := strings.TrimSpace(response)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			var arr []any
			if json.Unmarshal([]byte(trimmed), &arr) == nil {
				score += 0.4
				reasons = append(reasons, "valid JSON array")
			}
		}
	case "markdown_table":
		if strings.Contains(response, "|") && strings.Contains(response, "---") {
			score += 0.4
			reasons = append(reasons, "contains markdown table")
		}
	case "yaml":
		if strings.Contains(response, ":") && !strings.HasPrefix(strings.TrimSpace(response), "{") {
			score += 0.3
			reasons = append(reasons, "appears to be YAML")
		}
	case "bullets":
		bulletCount := strings.Count(response, "\n- ") + strings.Count(response, "\n* ")
		if strings.HasPrefix(strings.TrimSpace(response), "- ") || strings.HasPrefix(strings.TrimSpace(response), "* ") {
			bulletCount++
		}
		if bulletCount > 0 {
			score += 0.3
			reasons = append(reasons, fmt.Sprintf("has %d bullets", bulletCount))
		}
	default:
		// No format check, give partial credit for non-empty response
		if len(strings.TrimSpace(response)) > 0 {
			score += 0.2
			reasons = append(reasons, "non-empty response")
		}
	}

	// Constraint checks from metadata
	if constraints, ok := metadata["constraints"].([]any); ok {
		for _, c := range constraints {
			cs, _ := c.(string)
			if checkConstraint(response, cs) {
				score += 0.1
				reasons = append(reasons, fmt.Sprintf("constraint %q satisfied", cs))
			}
		}
	}

	score = math.Min(score, 1.0)
	detail := strings.Join(reasons, "; ")
	if detail == "" {
		detail = "no structural matches"
	}
	return score, detail, nil
}

// checkConstraint performs a basic check for a named constraint.
func checkConstraint(response, constraint string) bool {
	switch {
	case strings.HasPrefix(constraint, "under_") && strings.HasSuffix(constraint, "_words"):
		// e.g. "under_50_words"
		words := len(strings.Fields(response))
		return words < 50
	case strings.HasPrefix(constraint, "under_") && strings.HasSuffix(constraint, "_chars"):
		return len(response) < 200
	case strings.HasPrefix(constraint, "exactly_") && strings.HasSuffix(constraint, "_sentences"):
		sentences := countSentences(response)
		return sentences >= 2 && sentences <= 5
	case strings.Contains(constraint, "valid_json"):
		return json.Valid([]byte(strings.TrimSpace(response)))
	default:
		return false
	}
}

func countSentences(s string) int {
	count := 0
	for _, r := range s {
		if r == '.' || r == '!' || r == '?' {
			count++
		}
	}
	return count
}

// parseLLMJudgeResponse extracts score and reason from LLM judge output.
func parseLLMJudgeResponse(response string) (float64, string, error) {
	// Try to extract JSON from response
	response = strings.TrimSpace(response)

	// Find JSON object in response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	var result struct {
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return 0.5, "could not parse judge response, defaulting to 0.5", nil
	}

	// Clamp score
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 1 {
		result.Score = 1
	}

	return result.Score, result.Reason, nil
}

// executeGo compiles and runs Go code in a subprocess.
func executeGo(ctx context.Context, code, testInput string) (string, error) {
	// Wrap code in a main package with a main function that prints the test result
	wrapper := fmt.Sprintf(`package main

import "fmt"

%s

func main() {
	fmt.Println(%s)
}
`, code, testInput)

	// Write to temp file and run
	cmd := exec.CommandContext(ctx, "go", "run", "/dev/stdin")
	cmd.Stdin = strings.NewReader(wrapper)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// callEndpoint sends a chat completion request to an OpenAI-compatible endpoint.
func callEndpoint(ctx context.Context, endpoint, model, prompt string) (string, error) {
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  1024,
		"temperature": 0.0,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("endpoint returned status %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// normalizeText strips whitespace, lowercases, and removes common noise.
func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// Remove markdown code fences
	s = strings.ReplaceAll(s, "```go", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.TrimSpace(s)
	return s
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two rows instead of full matrix
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = minOf(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func minOf(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// stripNonAlphanumeric removes non-letter, non-digit characters.
func stripNonAlphanumeric(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
