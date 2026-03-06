package domains

import (
	"strings"
)

// ExtractCodeBlock extracts code from markdown fenced code blocks.
// If no code block is found, returns the original string.
func ExtractCodeBlock(response string) string {
	// Try to find ```go ... ``` or ``` ... ```
	markers := []string{"```go\n", "```golang\n", "```\n"}

	for _, marker := range markers {
		start := strings.Index(response, marker)
		if start < 0 {
			continue
		}
		codeStart := start + len(marker)
		end := strings.Index(response[codeStart:], "```")
		if end < 0 {
			continue
		}
		return strings.TrimSpace(response[codeStart : codeStart+end])
	}

	return ""
}

// ValidateGoSyntax does a quick check that Go code has basic structure.
func ValidateGoSyntax(code string) bool {
	// Basic checks: has func keyword, balanced braces
	if !strings.Contains(code, "func") {
		return false
	}

	braces := 0
	for _, r := range code {
		switch r {
		case '{':
			braces++
		case '}':
			braces--
		}
		if braces < 0 {
			return false
		}
	}

	return braces == 0
}

// CodeCategories returns the benchmark categories for code generation.
func CodeCategories() []string {
	return []string{
		"function_implementation",
		"bug_fixing",
		"code_review",
		"test_generation",
	}
}
