package types

import (
	"strings"
	"testing"
)

func TestBuildCanonicalForm_Assertion(t *testing.T) {
	s := &ClaimStructure{
		Subject:   "Water",
		Predicate: "boils at 100°C",
		Scope:     "at sea level",
	}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_ASSERTION, s, "physics")
	if !strings.HasPrefix(form, "assert(") {
		t.Fatalf("expected assert() prefix, got: %s", form)
	}
	if !strings.Contains(form, "physics") {
		t.Fatalf("expected domain in form, got: %s", form)
	}
	if !strings.Contains(form, "water") {
		t.Fatalf("expected lowercased subject, got: %s", form)
	}
	if !strings.Contains(form, "at sea level") {
		t.Fatalf("expected scope in form, got: %s", form)
	}
}

func TestBuildCanonicalForm_Relation(t *testing.T) {
	s := &ClaimStructure{
		Subject:   "Earth",
		Predicate: "orbits",
		Object:    "Sun",
	}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_RELATION, s, "astronomy")
	if !strings.HasPrefix(form, "relate(") {
		t.Fatalf("expected relate() prefix, got: %s", form)
	}
	if !strings.Contains(form, "sun") {
		t.Fatalf("expected object in form, got: %s", form)
	}
}

func TestBuildCanonicalForm_Definition(t *testing.T) {
	s := &ClaimStructure{
		Subject:   "Photon",
		Predicate: "a quantum of electromagnetic radiation",
	}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_DEFINITION, s, "physics")
	if !strings.HasPrefix(form, "define(") {
		t.Fatalf("expected define() prefix, got: %s", form)
	}
}

func TestBuildCanonicalForm_Constraint(t *testing.T) {
	s := &ClaimStructure{
		Subject:   "Speed",
		Predicate: "cannot exceed c",
		Scope:     "in vacuum",
	}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_CONSTRAINT, s, "physics")
	if !strings.HasPrefix(form, "constrain(") {
		t.Fatalf("expected constrain() prefix, got: %s", form)
	}
}

func TestBuildCanonicalForm_Negation(t *testing.T) {
	s := &ClaimStructure{
		Subject:   "Perpetual motion",
		Predicate: "is possible",
	}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_NEGATION, s, "physics")
	if !strings.HasPrefix(form, "negate(") {
		t.Fatalf("expected negate() prefix, got: %s", form)
	}
}

func TestBuildCanonicalForm_Observation(t *testing.T) {
	s := &ClaimStructure{
		Subject:       "Global temperature",
		Predicate:     "increased by 1.1°C",
		TemporalScope: "since pre-industrial",
	}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_OBSERVATION, s, "climate")
	if !strings.HasPrefix(form, "observe(") {
		t.Fatalf("expected observe() prefix, got: %s", form)
	}
	if !strings.Contains(form, "since pre-industrial") {
		t.Fatalf("expected temporal_scope in form, got: %s", form)
	}
}

func TestBuildCanonicalForm_NilStructure(t *testing.T) {
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_ASSERTION, nil, "physics")
	if form != "" {
		t.Fatalf("expected empty string for nil structure, got: %s", form)
	}
}

func TestBuildCanonicalForm_EmptySubject(t *testing.T) {
	s := &ClaimStructure{Predicate: "something"}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_ASSERTION, s, "physics")
	if form != "" {
		t.Fatalf("expected empty string for empty subject, got: %s", form)
	}
}

func TestBuildCanonicalForm_UnspecifiedType(t *testing.T) {
	s := &ClaimStructure{Subject: "Test", Predicate: "works"}
	form := BuildCanonicalForm(ClaimType_CLAIM_TYPE_UNSPECIFIED, s, "test")
	if !strings.HasPrefix(form, "assert(") {
		t.Fatalf("expected assert() for unspecified type, got: %s", form)
	}
}

func TestNormalizeCanonicalForm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "ASSERT(\"PHYSICS\")", `assert("physics")`},
		{"trim", "  assert(\"physics\")  ", `assert("physics")`},
		{"collapse whitespace", "assert( \"physics\" ,  \"water\" )", `assert( "physics" , "water" )`},
		{"idempotent", `assert("physics", "water")`, `assert("physics", "water")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCanonicalForm(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeCanonicalForm(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHashCanonicalForm(t *testing.T) {
	form := `assert("physics", "water", "boils at 100°c")`
	hash := HashCanonicalForm(form)

	// SHA-256 produces 64 hex chars
	if len(hash) != 64 {
		t.Fatalf("expected 64-char hex hash, got %d chars: %s", len(hash), hash)
	}

	// Deterministic
	hash2 := HashCanonicalForm(form)
	if hash != hash2 {
		t.Fatalf("hash not deterministic: %s != %s", hash, hash2)
	}

	// Different input → different hash
	hash3 := HashCanonicalForm(`assert("physics", "ice", "melts at 0°c")`)
	if hash == hash3 {
		t.Fatal("different inputs produced same hash")
	}
}

func TestSemanticDedup(t *testing.T) {
	// Two claims with the same semantic meaning but different whitespace/casing
	s1 := &ClaimStructure{Subject: "Water", Predicate: "boils at 100°C"}
	s2 := &ClaimStructure{Subject: "water", Predicate: "Boils at 100°C"}

	form1 := BuildCanonicalForm(ClaimType_CLAIM_TYPE_ASSERTION, s1, "Physics")
	form2 := BuildCanonicalForm(ClaimType_CLAIM_TYPE_ASSERTION, s2, "physics")

	hash1 := HashCanonicalForm(form1)
	hash2 := HashCanonicalForm(form2)

	if hash1 != hash2 {
		t.Fatalf("semantically equivalent claims produced different hashes:\n  form1=%s (hash=%s)\n  form2=%s (hash=%s)",
			form1, hash1, form2, hash2)
	}
}

func TestCollapseWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no change", "hello world", "hello world"},
		{"multiple spaces", "hello   world", "hello world"},
		{"tabs and newlines", "hello\t\n  world", "hello world"},
		{"preserves quoted", `hello  "foo  bar"  world`, `hello "foo  bar" world`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collapseWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("collapseWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
