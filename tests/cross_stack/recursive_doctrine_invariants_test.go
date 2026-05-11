package cross_stack_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRecursiveZerone_TestNamesCitedInDoctrineExist binds the recursion
// doctrine document (docs/RECURSIVE_ZERONE.md) to the codebase: every
// `TestXxx` name the doctrine cites as "closed by" a binding test must
// resolve to a real, discoverable Go test function in the repository.
//
// This is the recursion that audits the recursions: the doctrine claims
// bindings exist; this test enforces that claim. If the doctrine ever
// drifts (a binding test is renamed or deleted without updating the
// doctrine), this test fails — surfacing the drift before merge.
//
// Conceptually this is the same five-layer discipline applied to the
// recursion catalog itself: position (the doctrine names the test),
// voice (the test runs and emits events that bind the recursion),
// refusal (this meta-test fails if the position drifts from the binding),
// graph (every cited test is cross-referenced into the doctrine's
// "closed by" rubric), test (this very test).
func TestRecursiveZerone_TestNamesCitedInDoctrineExist(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "must locate repo root from test executable")

	doctrinePath := filepath.Join(repoRoot, "docs", "RECURSIVE_ZERONE.md")
	content, err := os.ReadFile(doctrinePath)
	require.NoError(t, err, "docs/RECURSIVE_ZERONE.md must exist")

	// Match Go test function names cited in the doctrine. Convention:
	// the doctrine refers to tests in `TestXxx_Yyy` form, inside
	// backticks or in plain prose. The regex captures TestX_Y style
	// (camelCase optionally followed by _Suffix segments).
	pattern := regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]+\b`)
	matches := pattern.FindAllString(string(content), -1)
	require.NotEmpty(t, matches, "doctrine must cite at least one binding test")

	// Deduplicate while preserving order for stable test output.
	seen := map[string]bool{}
	unique := []string{}
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}

	// Build a set of all Go test function names in the repo.
	repoTests, err := discoverTestFuncs(repoRoot)
	require.NoError(t, err)
	require.NotEmpty(t, repoTests, "repo must contain at least one Go test")

	// Every cited test must exist somewhere in the repo.
	missing := []string{}
	for _, cited := range unique {
		if !repoTests[cited] {
			missing = append(missing, cited)
		}
	}
	require.Empty(t, missing,
		"docs/RECURSIVE_ZERONE.md cites tests that do not exist in the codebase: %v\n"+
			"Either add the test, rename the citation, or remove it from the doctrine.",
		missing)

	// Sanity check: log how many bindings the doctrine claims.
	t.Logf("doctrine cites %d distinct binding tests; all resolve to real Go tests", len(unique))
}

// findRepoRoot walks upward from the test file's location until it finds
// a `go.mod`, then returns that directory.
func findRepoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// discoverTestFuncs returns the set of all Go test-function names
// declared in *_test.go files anywhere under repoRoot. We use a grep
// shell-out for speed and portability (no AST parsing needed for a
// name-set lookup).
func discoverTestFuncs(repoRoot string) (map[string]bool, error) {
	// Pattern: line starts with `func Test` followed by capital letter,
	// continues with identifier chars, then `(t *testing.T)`. Excludes
	// vendor and build artifacts.
	cmd := exec.Command("grep", "-rh", "--include=*_test.go",
		"--exclude-dir=vendor", "--exclude-dir=.git", "--exclude-dir=build",
		"-E", `^func Test[A-Z][A-Za-z0-9_]*\(`,
		repoRoot,
	)
	out, err := cmd.Output()
	if err != nil {
		// grep returns exit code 1 when no match — but we expect matches.
		// Any error here is a real problem.
		return nil, err
	}

	funcRE := regexp.MustCompile(`^func (Test[A-Z][A-Za-z0-9_]*)\(`)
	set := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		m := funcRE.FindStringSubmatch(line)
		if m != nil {
			set[m[1]] = true
		}
	}
	return set, nil
}
