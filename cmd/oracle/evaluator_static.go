package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// axiomEntry holds a preprocessed genesis axiom for fast evaluation.
type axiomEntry struct {
	ID       string
	Statement string
	Domain   string
	Keywords []string
	Numbers  []float64
}

// StaticEvaluator checks claims against 777 genesis axioms embedded in the binary.
type StaticEvaluator struct {
	axioms   []axiomEntry
	byDomain map[string][]int // domain → axiom indices
}

// NewStaticEvaluator loads genesis axioms and builds a domain-indexed evaluator.
func NewStaticEvaluator() (*StaticEvaluator, error) {
	raw, err := knowledgetypes.ParseAxioms(knowledgetypes.GenesisAxiomsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse genesis axioms: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("genesis axioms are empty")
	}

	se := &StaticEvaluator{
		axioms:   make([]axiomEntry, len(raw)),
		byDomain: make(map[string][]int),
	}

	for i, a := range raw {
		combined := a.Statement + " " + a.FormalExpression
		entry := axiomEntry{
			ID:        a.AxiomID,
			Statement: a.Statement,
			Domain:    a.Domain,
			Keywords:  tokenize(a.Statement),
			Numbers:   extractNumbers(combined),
		}
		se.axioms[i] = entry
		se.byDomain[a.Domain] = append(se.byDomain[a.Domain], i)
	}

	return se, nil
}

// Name returns the evaluator strategy name.
func (se *StaticEvaluator) Name() string { return "static" }

// AxiomCount returns the number of loaded axioms.
func (se *StaticEvaluator) AxiomCount() int { return len(se.axioms) }

// Evaluate checks a claim against axioms in the same domain.
func (se *StaticEvaluator) Evaluate(req EvaluateRequest) (*EvaluateResponse, error) {
	uncertain := &EvaluateResponse{
		Verdict:    "uncertain",
		Confidence: 0.5,
		Reasoning:  "no strong match found among genesis axioms",
	}

	// 1. Get axioms in the same domain.
	indices, ok := se.byDomain[req.Domain]
	if !ok || len(indices) == 0 {
		uncertain.Reasoning = fmt.Sprintf("no axioms in domain %q", req.Domain)
		return uncertain, nil
	}

	claimKW := tokenize(req.Claim)
	claimNums := extractNumbers(req.Claim)

	// 2. Find best keyword match (Jaccard similarity).
	bestScore := 0.0
	bestIdx := -1
	for _, idx := range indices {
		score := keywordOverlap(claimKW, se.axioms[idx].Keywords)
		if score > bestScore {
			bestScore = score
			bestIdx = idx
		}
	}

	// 3. If best match < 0.15 → uncertain.
	if bestScore < 0.15 || bestIdx < 0 {
		uncertain.Reasoning = fmt.Sprintf("best keyword match %.3f < 0.15", bestScore)
		return uncertain, nil
	}

	bestAxiom := se.axioms[bestIdx]

	// 4. Check for explicit negation.
	if hasNegation(req.Claim, bestAxiom.Statement) {
		return &EvaluateResponse{
			Verdict:    "reject",
			Confidence: 0.75,
			Reasoning: fmt.Sprintf(
				"claim negates axiom %s (%q); keyword overlap %.3f",
				bestAxiom.ID, bestAxiom.Statement, bestScore),
		}, nil
	}

	// 5. Check for numerical contradiction.
	if isContradiction, claimN, axiomN := numericalContradiction(claimNums, bestAxiom.Numbers); isContradiction {
		return &EvaluateResponse{
			Verdict:    "reject",
			Confidence: 0.8,
			Reasoning: fmt.Sprintf(
				"numerical contradiction with axiom %s: claim has %.4g, axiom has %.4g",
				bestAxiom.ID, claimN, axiomN),
		}, nil
	}

	// 6. If best match > 0.4 → accept.
	if bestScore > 0.4 {
		return &EvaluateResponse{
			Verdict:    "accept",
			Confidence: 0.6 + bestScore*0.3,
			Reasoning: fmt.Sprintf(
				"consistent with axiom %s (%q); keyword overlap %.3f",
				bestAxiom.ID, bestAxiom.Statement, bestScore),
		}, nil
	}

	// 7. Else → uncertain.
	uncertain.Reasoning = fmt.Sprintf(
		"moderate keyword overlap %.3f with axiom %s but not conclusive",
		bestScore, bestAxiom.ID)
	return uncertain, nil
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// stopWords contains common English stop words to filter during tokenization.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "from": true,
	"is": true, "it": true, "its": true, "are": true, "was": true,
	"were": true, "be": true, "been": true, "being": true,
	"has": true, "have": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"this": true, "that": true, "these": true, "those": true,
	"he": true, "she": true, "they": true, "we": true, "you": true,
	"all": true, "each": true, "every": true, "both": true,
	"such": true, "than": true, "too": true, "very": true,
	"just": true, "about": true, "above": true, "after": true,
	"before": true, "between": true, "into": true, "through": true,
	"during": true, "same": true, "other": true, "which": true,
	"there": true, "when": true, "where": true, "how": true,
	"what": true, "who": true, "whom": true,
}

// tokenize lowercases text, strips punctuation, and returns non-stop words of length >= 3.
func tokenize(text string) []string {
	lower := strings.ToLower(text)

	// Replace punctuation with spaces.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			return r
		}
		return ' '
	}, lower)

	words := strings.Fields(cleaned)
	var result []string
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		if stopWords[w] {
			continue
		}
		if seen[w] {
			continue
		}
		seen[w] = true
		result = append(result, w)
	}
	return result
}

// keywordOverlap computes the Jaccard similarity between two keyword sets.
func keywordOverlap(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(a))
	for _, w := range a {
		setA[w] = true
	}
	setB := make(map[string]bool, len(b))
	for _, w := range b {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}

	union := len(setA)
	for w := range setB {
		if !setA[w] {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// negationWords are terms that signal a claim is negating an axiom.
// negationRe matches negation words at word boundaries, avoiding false
// positives on substrings (e.g., "notation" should not match "not").
var negationRe = regexp.MustCompile(`\b(not|never|false|incorrect|wrong|isn't|doesn't|cannot|no longer)\b`)

// hasNegation returns true when the claim contains a negation word AND
// the stripped-claim keywords overlap with axiom keywords by > 0.2.
func hasNegation(claim, axiom string) bool {
	lowerClaim := strings.ToLower(claim)

	if !negationRe.MatchString(lowerClaim) {
		return false
	}

	// Strip negation words from claim, then tokenize the remainder.
	stripped := negationRe.ReplaceAllString(lowerClaim, " ")
	strippedKW := tokenize(stripped)
	axiomKW := tokenize(axiom)

	overlap := keywordOverlap(strippedKW, axiomKW)
	return overlap > 0.2
}

// superscriptMap maps Unicode superscript characters to their regular equivalents.
var superscriptMap = map[rune]rune{
	'⁰': '0', '¹': '1', '²': '2', '³': '3', '⁴': '4',
	'⁵': '5', '⁶': '6', '⁷': '7', '⁸': '8', '⁹': '9',
	'⁻': '-',
}

// convertSuperscript replaces Unicode superscript digits with regular digits.
func convertSuperscript(s string) string {
	var b strings.Builder
	for _, r := range s {
		if mapped, ok := superscriptMap[r]; ok {
			b.WriteRune(mapped)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sciNotationRe matches patterns like "2.998 × 10⁸" or "3 × 10⁻²".
var sciNotationRe = regexp.MustCompile(`(\d+\.?\d*)\s*[×x]\s*10([⁰¹²³⁴⁵⁶⁷⁸⁹⁻]+)`)

// plainNumberRe matches integers and decimals.
var plainNumberRe = regexp.MustCompile(`\d+\.?\d*`)

// extractNumbers extracts numbers from text, including scientific notation with
// Unicode superscripts (e.g., "2.998 × 10⁸").
func extractNumbers(text string) []float64 {
	var nums []float64
	seen := make(map[float64]bool)

	addNum := func(f float64) {
		if !seen[f] && !math.IsNaN(f) && !math.IsInf(f, 0) {
			seen[f] = true
			nums = append(nums, f)
		}
	}

	// First pass: extract scientific notation (and mark their positions to skip later).
	sciMatches := sciNotationRe.FindAllStringSubmatchIndex(text, -1)
	skipRanges := make([][2]int, len(sciMatches))

	for i, loc := range sciMatches {
		skipRanges[i] = [2]int{loc[0], loc[1]}

		mantissa := text[loc[2]:loc[3]]
		exponentSuperscript := text[loc[4]:loc[5]]
		exponentStr := convertSuperscript(exponentSuperscript)

		m, err1 := strconv.ParseFloat(mantissa, 64)
		e, err2 := strconv.ParseFloat(exponentStr, 64)
		if err1 == nil && err2 == nil {
			addNum(m * math.Pow(10, e))
		}
	}

	// Second pass: extract plain numbers, skipping ranges already captured.
	plainMatches := plainNumberRe.FindAllStringIndex(text, -1)
	for _, loc := range plainMatches {
		skip := false
		for _, sr := range skipRanges {
			if loc[0] >= sr[0] && loc[1] <= sr[1] {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		numStr := text[loc[0]:loc[1]]
		f, err := strconv.ParseFloat(numStr, 64)
		if err == nil {
			addNum(f)
		}
	}

	return nums
}

// numericalContradiction returns true when two number sets contain a pair
// whose ratio is within [1e-6, 1e6] (same order-of-magnitude ballpark)
// but whose relative difference exceeds 50%.
func numericalContradiction(claimNums, axiomNums []float64) (bool, float64, float64) {
	for _, cn := range claimNums {
		if cn == 0 {
			continue
		}
		for _, an := range axiomNums {
			if an == 0 {
				continue
			}
			ratio := cn / an
			if ratio < 0 {
				ratio = -ratio
			}

			// Must be within 6 orders of magnitude.
			if ratio < 1e-6 || ratio > 1e6 {
				continue
			}

			// Relative difference > 50%.
			diff := math.Abs(cn - an)
			maxVal := math.Max(math.Abs(cn), math.Abs(an))
			if maxVal == 0 {
				continue
			}
			relDiff := diff / maxVal
			if relDiff > 0.5 {
				return true, cn, an
			}
		}
	}
	return false, 0, 0
}
