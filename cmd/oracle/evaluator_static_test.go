package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticEvaluator_LoadsAxioms(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)
	require.Greater(t, se.AxiomCount(), 0, "should load >0 axioms")
}

func TestStaticEvaluator_AcceptsConsistentClaim(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	resp, err := se.Evaluate(EvaluateRequest{
		Claim:  "Electromagnetic waves propagate at the speed of light in vacuum",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.NotEqual(t, "reject", resp.Verdict,
		"a claim consistent with genesis axioms must NOT be rejected")
}

func TestStaticEvaluator_RejectsNumericalContradiction(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	resp, err := se.Evaluate(EvaluateRequest{
		Claim:  "The speed of light in vacuum is 100 m/s",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.Equal(t, "reject", resp.Verdict,
		"claim with contradicting number should be rejected")
	require.Greater(t, resp.Confidence, 0.5,
		"rejection confidence should exceed 0.5")
}

func TestStaticEvaluator_RejectsExplicitNegation(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	resp, err := se.Evaluate(EvaluateRequest{
		Claim:  "The speed of light is not constant across inertial frames",
		Domain: "physics",
	})
	require.NoError(t, err)
	require.Equal(t, "reject", resp.Verdict,
		"claim that explicitly negates an axiom should be rejected")
}

func TestStaticEvaluator_UncertainForUnrelatedClaim(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	resp, err := se.Evaluate(EvaluateRequest{
		Claim:  "Bananas are the most popular fruit in Norway",
		Domain: "general",
	})
	require.NoError(t, err)
	require.Equal(t, "uncertain", resp.Verdict,
		"unrelated claim should be uncertain")
}

func TestStaticEvaluator_DomainFiltering(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	resp, err := se.Evaluate(EvaluateRequest{
		Claim:  "The speed of light in vacuum is 100 m/s",
		Domain: "biology",
	})
	require.NoError(t, err)
	require.Equal(t, "uncertain", resp.Verdict,
		"claim in wrong domain should be uncertain")
}

func TestExtractNumbers_Basic(t *testing.T) {
	nums := extractNumbers("There are 42 cats and 3.14 pies")
	require.Contains(t, nums, 42.0)
	require.Contains(t, nums, 3.14)
}

func TestExtractNumbers_Scientific(t *testing.T) {
	nums := extractNumbers("speed is 2.998 × 10⁸ m/s")
	require.NotEmpty(t, nums, "should extract at least one number")

	// The extracted value should be approximately 2.998e8.
	found := false
	for _, n := range nums {
		if n > 1e7 {
			found = true
			break
		}
	}
	require.True(t, found, "should extract a large number from scientific notation")
}
