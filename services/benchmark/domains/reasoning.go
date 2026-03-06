package domains

import (
	"strings"
	"unicode"
)

// NormalizeAnswer strips whitespace, punctuation, and lowercases for comparison.
func NormalizeAnswer(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	// Remove trailing punctuation
	s = strings.TrimRightFunc(s, func(r rune) bool {
		return unicode.IsPunct(r)
	})

	return s
}

// ExtractNumber tries to extract a numeric answer from a response.
// It looks for the last standalone number in the text.
func ExtractNumber(response string) string {
	response = strings.TrimSpace(response)

	// If the response is just a number, return it
	cleaned := strings.TrimRightFunc(response, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSpace(r)
	})
	if isNumeric(cleaned) {
		return cleaned
	}

	// Look for the last number in the response
	words := strings.Fields(response)
	for i := len(words) - 1; i >= 0; i-- {
		w := strings.TrimRightFunc(words[i], func(r rune) bool {
			return unicode.IsPunct(r)
		})
		if isNumeric(w) {
			return w
		}
	}

	return response
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}
	hasDigit := false
	hasDot := false
	for i := start; i < len(s); i++ {
		if s[i] == '.' && !hasDot {
			hasDot = true
		} else if s[i] >= '0' && s[i] <= '9' {
			hasDigit = true
		} else {
			return false
		}
	}
	return hasDigit
}

// ReasoningCategories returns the benchmark categories for reasoning.
func ReasoningCategories() []string {
	return []string{
		"math",
		"logic",
		"cause_effect",
	}
}
