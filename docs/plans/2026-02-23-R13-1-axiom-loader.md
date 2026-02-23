# R13-1 Axiom Loader Tool Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a CLI tool at `tools/axiom-loader/main.go` with four subcommands (validate, inject, stats, edges) for working with the 777 genesis axioms.

**Architecture:** Thin CLI wrapper around existing `x/knowledge/types` functions (`LoadAxiomsFromFile`, `ValidateAxioms`, `ComputeDAGStats`, `ComputeCrossDomainMatrix`, `AxiomsToFacts`). The tool reads axiom JSON, delegates to existing library functions, and formats output. The `inject` subcommand reads/writes genesis.json using `encoding/json` raw manipulation (no cosmos dependency needed — just unmarshal `app_state.knowledge.facts`, replace, marshal back).

**Tech Stack:** Go stdlib only (no new dependencies). Imports `github.com/zerone-chain/zerone/x/knowledge/types` for `GenesisAxiom`, `LoadAxiomsFromFile`, `ValidateAxioms`, `ComputeDAGStats`, `ComputeCrossDomainMatrix`, `AxiomsToFacts`, `AxiomDomainNames`.

---

### Task 1: Create axiom-loader main.go with subcommand routing

**Files:**
- Create: `tools/axiom-loader/main.go`

**Step 1: Write the CLI skeleton**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "validate":
		err = runValidate(args)
	case "inject":
		err = runInject(args)
	case "stats":
		err = runStats(args)
	case "edges":
		err = runEdges(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: axiom-loader <command> [args]

Commands:
  validate <axioms.json>                    Validate axiom DAG
  inject   <axioms.json> <genesis.json>     Inject axioms into genesis
  stats    <axioms.json>                    Print axiom statistics
  edges    <axioms.json> [-o output.csv]    Export dependency edges as CSV
`)
}
```

**Step 2: Add stub functions for each subcommand**

Add to the same file (stubs that return `nil` for now):

```go
func runValidate(args []string) error { return fmt.Errorf("not implemented") }
func runInject(args []string) error   { return fmt.Errorf("not implemented") }
func runStats(args []string) error    { return fmt.Errorf("not implemented") }
func runEdges(args []string) error    { return fmt.Errorf("not implemented") }
```

**Step 3: Verify it builds**

Run: `go build ./tools/axiom-loader/...`
Expected: Builds successfully.

**Step 4: Commit**

```bash
git add tools/axiom-loader/main.go
git commit -m "feat(R13-1): scaffold axiom-loader CLI with subcommand routing"
```

---

### Task 2: Implement the `validate` subcommand

**Files:**
- Modify: `tools/axiom-loader/main.go`

**Step 1: Write a failing test for validate**

Create `tools/axiom-loader/main_test.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	ktypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// writeAxiomFile is a test helper that writes axioms to a temp JSON file.
func writeAxiomFile(t *testing.T, dir string, axioms []*ktypes.GenesisAxiom) string {
	t.Helper()
	path := filepath.Join(dir, "axioms.json")
	data, err := json.MarshalIndent(axioms, "", "  ")
	if err != nil {
		t.Fatalf("marshal axioms: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write axioms: %v", err)
	}
	return path
}

func TestValidateReal777(t *testing.T) {
	// Validate the real 777-axiom file.
	err := runValidate([]string{"../../x/knowledge/types/genesis_axioms.json"})
	if err != nil {
		t.Fatalf("validate 777 axioms: %v", err)
	}
}

func TestValidateDuplicateID(t *testing.T) {
	dir := t.TempDir()
	axioms := []*ktypes.GenesisAxiom{
		{AxiomID: "MATH-001", Statement: "Zero is a natural number", ClaimType: "axiom", Domain: "mathematics", EpistemicCategory: "analytic", Confidence: 1.0, Dependencies: []string{}},
		{AxiomID: "MATH-001", Statement: "Duplicate", ClaimType: "axiom", Domain: "mathematics", EpistemicCategory: "analytic", Confidence: 1.0, Dependencies: []string{}},
	}
	path := writeAxiomFile(t, dir, axioms)
	err := runValidate([]string{path})
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
}

func TestValidateMissingDep(t *testing.T) {
	dir := t.TempDir()
	axioms := []*ktypes.GenesisAxiom{
		{AxiomID: "MATH-001", Statement: "Zero is a natural number", ClaimType: "axiom", Domain: "mathematics", EpistemicCategory: "analytic", Confidence: 1.0, Dependencies: []string{"MATH-999"}},
	}
	path := writeAxiomFile(t, dir, axioms)
	err := runValidate([]string{path})
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestValidateCycle(t *testing.T) {
	dir := t.TempDir()
	axioms := []*ktypes.GenesisAxiom{
		{AxiomID: "MATH-001", Statement: "A", ClaimType: "derived_claim", Domain: "mathematics", EpistemicCategory: "formal", Confidence: 0.9, Dependencies: []string{"MATH-002"}},
		{AxiomID: "MATH-002", Statement: "B", ClaimType: "derived_claim", Domain: "mathematics", EpistemicCategory: "formal", Confidence: 0.9, Dependencies: []string{"MATH-001"}},
	}
	path := writeAxiomFile(t, dir, axioms)
	err := runValidate([]string{path})
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./tools/axiom-loader/... -v -run TestValidate`
Expected: FAIL — `runValidate` returns "not implemented".

**Step 3: Implement `runValidate`**

Replace the stub in `main.go`:

```go
import (
	"fmt"
	"os"
	"sort"
	"strings"

	ktypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

func runValidate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: axiom-loader validate <axioms.json>")
	}

	axioms, err := ktypes.LoadAxiomsFromFile(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("✓ %d axioms loaded\n", len(axioms))

	// Check duplicates
	idSet := make(map[string]bool, len(axioms))
	dupes := 0
	for _, a := range axioms {
		if idSet[a.AxiomID] {
			dupes++
			fmt.Fprintf(os.Stderr, "  duplicate: %s\n", a.AxiomID)
		}
		idSet[a.AxiomID] = true
	}
	fmt.Printf("✓ %d duplicate IDs\n", dupes)

	// Check missing deps
	missing := 0
	for _, a := range axioms {
		for _, dep := range a.Dependencies {
			if !idSet[dep] {
				missing++
				fmt.Fprintf(os.Stderr, "  %s → missing %s\n", a.AxiomID, dep)
			}
		}
	}
	fmt.Printf("✓ %d missing dependencies\n", missing)

	// Full validation (includes DAG cycle check)
	domainNames := ktypes.AxiomDomainNames()
	valErr := ktypes.ValidateAxioms(axioms, domainNames)
	if valErr != nil {
		if strings.Contains(valErr.Error(), "cycle") {
			fmt.Printf("✗ Cycle detected\n")
		}
		return valErr
	}
	fmt.Printf("✓ 0 cycles detected\n")
	fmt.Printf("✓ DAG validation passed\n")

	// Summary stats
	dagStats, err := ktypes.ComputeDAGStats(axioms)
	if err != nil {
		return err
	}

	domains := make(map[string]bool)
	typeSet := make(map[string]bool)
	for _, a := range axioms {
		domains[a.Domain] = true
		typeSet[a.ClaimType] = true
	}

	leaves := len(axioms) - dagStats.RootCount
	// Count leaves: axioms with no dependents
	dependedOn := make(map[string]bool)
	for _, a := range axioms {
		for _, dep := range a.Dependencies {
			dependedOn[dep] = true
		}
	}
	leafCount := 0
	for _, a := range axioms {
		if !dependedOn[a.AxiomID] {
			leafCount++
		}
	}
	_ = leaves // unused after recompute

	fmt.Printf("\nDomains:   %d\n", len(domains))
	fmt.Printf("Types:     %d distinct\n", len(typeSet))
	fmt.Printf("Roots:     %d (no dependencies)\n", dagStats.RootCount)
	fmt.Printf("Leaves:    %d (no dependents)\n", leafCount)
	fmt.Printf("Max depth: %d\n", dagStats.MaxDepth)

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./tools/axiom-loader/... -v -run TestValidate`
Expected: All 4 tests PASS.

**Step 5: Commit**

```bash
git add tools/axiom-loader/main.go tools/axiom-loader/main_test.go
git commit -m "feat(R13-1): implement validate subcommand with tests"
```

---

### Task 3: Implement the `stats` subcommand

**Files:**
- Modify: `tools/axiom-loader/main.go`

**Step 1: Implement `runStats`**

Replace the stub:

```go
func runStats(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: axiom-loader stats <axioms.json>")
	}

	axioms, err := ktypes.LoadAxiomsFromFile(args[0])
	if err != nil {
		return err
	}

	dagStats, err := ktypes.ComputeDAGStats(axioms)
	if err != nil {
		return err
	}

	// Group by domain
	type domainInfo struct {
		count    int
		roots    int
		types    map[string]bool
		maxDepth int
	}

	// Build per-axiom depth map
	depthOf := computeDepthMap(axioms)

	domainMap := make(map[string]*domainInfo)
	for _, a := range axioms {
		di, ok := domainMap[a.Domain]
		if !ok {
			di = &domainInfo{types: make(map[string]bool)}
			domainMap[a.Domain] = di
		}
		di.count++
		di.types[a.ClaimType] = true
		if len(a.Dependencies) == 0 {
			di.roots++
		}
		d := depthOf[a.AxiomID]
		if d > di.maxDepth {
			di.maxDepth = d
		}
	}

	// Sort domains by name
	var domainNames []string
	for d := range domainMap {
		domainNames = append(domainNames, d)
	}
	sort.Strings(domainNames)

	// Print table
	fmt.Printf("%-20s %5s %5s %9s   %s\n", "Domain", "Count", "Roots", "Max Depth", "Types")
	fmt.Println(strings.Repeat("─", 78))
	for _, d := range domainNames {
		di := domainMap[d]
		var typeNames []string
		for t := range di.types {
			typeNames = append(typeNames, t)
		}
		sort.Strings(typeNames)
		fmt.Printf("%-20s %5d %5d %9d   %s\n", d, di.count, di.roots, di.maxDepth, strings.Join(typeNames, ", "))
	}

	// Cross-domain deps
	crossMatrix := ktypes.ComputeCrossDomainMatrix(axioms)
	totalCross := 0
	for _, e := range crossMatrix.Entries {
		totalCross += e.Count
	}

	fmt.Printf("\n%-20s %5d %5d %9d\n", "Total:", len(axioms), dagStats.RootCount, dagStats.MaxDepth)
	fmt.Printf("Cross-domain deps: %d\n", totalCross)

	return nil
}

// computeDepthMap returns the DAG depth for each axiom ID.
func computeDepthMap(axioms []*ktypes.GenesisAxiom) map[string]int {
	inDegree := make(map[string]int, len(axioms))
	dependents := make(map[string][]string, len(axioms))

	for _, a := range axioms {
		if _, ok := inDegree[a.AxiomID]; !ok {
			inDegree[a.AxiomID] = 0
		}
		for _, dep := range a.Dependencies {
			inDegree[a.AxiomID]++
			dependents[dep] = append(dependents[dep], a.AxiomID)
		}
	}

	depth := make(map[string]int, len(axioms))
	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
			depth[id] = 0
		}
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, dependent := range dependents[node] {
			candidate := depth[node] + 1
			if candidate > depth[dependent] {
				depth[dependent] = candidate
			}
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return depth
}
```

**Step 2: Verify it builds and runs**

Run: `go build ./tools/axiom-loader/... && go run tools/axiom-loader/main.go stats x/knowledge/types/genesis_axioms.json`
Expected: Table output with domains, counts, types.

**Step 3: Commit**

```bash
git add tools/axiom-loader/main.go
git commit -m "feat(R13-1): implement stats subcommand"
```

---

### Task 4: Implement the `edges` subcommand

**Files:**
- Modify: `tools/axiom-loader/main.go`

**Step 1: Implement `runEdges`**

Replace the stub:

```go
func runEdges(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: axiom-loader edges <axioms.json> [-o output.csv]")
	}

	axiomPath := args[0]
	outputPath := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			outputPath = args[i+1]
			i++
		}
	}

	axioms, err := ktypes.LoadAxiomsFromFile(axiomPath)
	if err != nil {
		return err
	}

	idToDomain := make(map[string]string, len(axioms))
	for _, a := range axioms {
		idToDomain[a.AxiomID] = a.Domain
	}

	out := os.Stdout
	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	fmt.Fprintln(out, "source,target,source_domain,target_domain")
	edgeCount := 0
	for _, a := range axioms {
		for _, dep := range a.Dependencies {
			fmt.Fprintf(out, "%s,%s,%s,%s\n", a.AxiomID, dep, a.Domain, idToDomain[dep])
			edgeCount++
		}
	}

	if outputPath != "" {
		fmt.Printf("✓ Exported %d edges to %s\n", edgeCount, outputPath)
	}

	return nil
}
```

**Step 2: Verify it runs**

Run: `go run tools/axiom-loader/main.go edges x/knowledge/types/genesis_axioms.json | head -5`
Expected: CSV header + edge rows.

**Step 3: Commit**

```bash
git add tools/axiom-loader/main.go
git commit -m "feat(R13-1): implement edges subcommand"
```

---

### Task 5: Implement the `inject` subcommand

**Files:**
- Modify: `tools/axiom-loader/main.go`
- Test: `tools/axiom-loader/main_test.go`

**Step 1: Write the inject test**

Add to `main_test.go`:

```go
func TestInjectIntoGenesis(t *testing.T) {
	dir := t.TempDir()

	// Minimal axiom set
	axioms := []*ktypes.GenesisAxiom{
		{AxiomID: "MATH-001", Statement: "Zero is a natural number", ClaimType: "axiom", Domain: "mathematics", EpistemicCategory: "analytic", Confidence: 1.0, Dependencies: []string{}},
		{AxiomID: "MATH-002", Statement: "Every natural number has a successor", ClaimType: "axiom", Domain: "mathematics", EpistemicCategory: "analytic", Confidence: 1.0, Dependencies: []string{}},
	}
	axiomPath := writeAxiomFile(t, dir, axioms)

	// Minimal genesis.json with empty knowledge state
	genesisPath := filepath.Join(dir, "genesis.json")
	genesis := map[string]interface{}{
		"genesis_time":   "2024-01-01T00:00:00Z",
		"chain_id":       "test-1",
		"initial_height": "1",
		"app_state": map[string]interface{}{
			"knowledge": map[string]interface{}{
				"params": map[string]interface{}{},
				"facts":  []interface{}{},
			},
			"bank": map[string]interface{}{},
		},
	}
	genesisData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		t.Fatalf("marshal genesis: %v", err)
	}
	if err := os.WriteFile(genesisPath, genesisData, 0644); err != nil {
		t.Fatalf("write genesis: %v", err)
	}

	// Run inject
	err = runInject([]string{axiomPath, genesisPath})
	if err != nil {
		t.Fatalf("inject: %v", err)
	}

	// Read back and verify facts were injected
	resultData, err := os.ReadFile(genesisPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	appState := result["app_state"].(map[string]interface{})
	knowledge := appState["knowledge"].(map[string]interface{})
	facts := knowledge["facts"].([]interface{})

	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}

	// Verify bank is still there (other modules preserved)
	if _, ok := appState["bank"]; !ok {
		t.Fatal("bank module was lost during inject")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tools/axiom-loader/... -v -run TestInject`
Expected: FAIL — `runInject` returns "not implemented".

**Step 3: Implement `runInject`**

Replace the stub:

```go
func runInject(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: axiom-loader inject <axioms.json> <genesis.json>")
	}

	axiomPath := args[0]
	genesisPath := args[1]

	axioms, err := ktypes.LoadAxiomsFromFile(axiomPath)
	if err != nil {
		return err
	}

	// Validate before injecting
	domainNames := ktypes.AxiomDomainNames()
	if err := ktypes.ValidateAxioms(axioms, domainNames); err != nil {
		return fmt.Errorf("axiom validation failed: %w", err)
	}

	// Convert axioms to facts
	facts := ktypes.AxiomsToFacts(axioms)

	// Read genesis.json
	genesisData, err := os.ReadFile(genesisPath)
	if err != nil {
		return fmt.Errorf("failed to read genesis: %w", err)
	}

	var genesis map[string]json.RawMessage
	if err := json.Unmarshal(genesisData, &genesis); err != nil {
		return fmt.Errorf("failed to parse genesis: %w", err)
	}

	var appState map[string]json.RawMessage
	if err := json.Unmarshal(genesis["app_state"], &appState); err != nil {
		return fmt.Errorf("failed to parse app_state: %w", err)
	}

	var knowledgeState map[string]json.RawMessage
	if err := json.Unmarshal(appState["knowledge"], &knowledgeState); err != nil {
		return fmt.Errorf("failed to parse knowledge state: %w", err)
	}

	// Marshal facts and inject
	factsJSON, err := json.Marshal(facts)
	if err != nil {
		return fmt.Errorf("failed to marshal facts: %w", err)
	}
	knowledgeState["facts"] = factsJSON

	// Reassemble
	knowledgeJSON, err := json.Marshal(knowledgeState)
	if err != nil {
		return fmt.Errorf("failed to marshal knowledge: %w", err)
	}
	appState["knowledge"] = knowledgeJSON

	appStateJSON, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("failed to marshal app_state: %w", err)
	}
	genesis["app_state"] = appStateJSON

	result, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, result, 0644); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}

	fmt.Printf("✓ Injected %d axioms into %s\n", len(facts), genesisPath)
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./tools/axiom-loader/... -v -run TestInject`
Expected: PASS.

**Step 5: Commit**

```bash
git add tools/axiom-loader/main.go tools/axiom-loader/main_test.go
git commit -m "feat(R13-1): implement inject subcommand with test"
```

---

### Task 6: Run full verification

**Step 1: Run all tests**

Run: `go test ./tools/axiom-loader/... -v`
Expected: All tests pass.

**Step 2: Run validate against real axioms**

Run: `go run tools/axiom-loader/main.go validate x/knowledge/types/genesis_axioms.json`
Expected: Output showing `✓ 777 axioms loaded`, `✓ 0 duplicate IDs`, `✓ DAG validation passed`.

**Step 3: Run stats against real axioms**

Run: `go run tools/axiom-loader/main.go stats x/knowledge/types/genesis_axioms.json`
Expected: Domain table with counts, roots, max depth, types.

**Step 4: Run edges against real axioms**

Run: `go run tools/axiom-loader/main.go edges x/knowledge/types/genesis_axioms.json | head -10`
Expected: CSV with header + edge rows.

**Step 5: Commit (if any fix-ups needed)**

```bash
git add tools/axiom-loader/
git commit -m "test(R13-1): verify axiom-loader against 777 real axioms"
```
