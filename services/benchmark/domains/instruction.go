package domains

import (
	"encoding/json"
	"strings"
)

// CheckJSON validates that a response is valid JSON.
func CheckJSON(response string) bool {
	return json.Valid([]byte(strings.TrimSpace(response)))
}

// CheckJSONKeys validates that a JSON response contains all required keys.
func CheckJSONKeys(response string, keys []string) bool {
	response = strings.TrimSpace(response)
	var obj map[string]any
	if err := json.Unmarshal([]byte(response), &obj); err != nil {
		return false
	}
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			return false
		}
	}
	return true
}

// CheckWordCount returns true if the response word count is within the given range.
func CheckWordCount(response string, min, max int) bool {
	words := len(strings.Fields(response))
	return words >= min && words <= max
}

// CheckBulletCount counts bullet points in a response.
func CheckBulletCount(response string) int {
	count := 0
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			count++
		}
	}
	return count
}

// CheckContainsAll returns true if response contains all the given substrings.
func CheckContainsAll(response string, substrings []string) bool {
	lower := strings.ToLower(response)
	for _, s := range substrings {
		if !strings.Contains(lower, strings.ToLower(s)) {
			return false
		}
	}
	return true
}

// CheckExcludesAll returns true if response does not contain any of the given substrings.
func CheckExcludesAll(response string, substrings []string) bool {
	lower := strings.ToLower(response)
	for _, s := range substrings {
		if strings.Contains(lower, strings.ToLower(s)) {
			return false
		}
	}
	return true
}

// InstructionCategories returns the benchmark categories for instruction following.
func InstructionCategories() []string {
	return []string{
		"format_compliance",
		"constraint_satisfaction",
		"multi_constraint",
	}
}
