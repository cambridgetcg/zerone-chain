# Useful Work Phase 0 — Doctrine Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adopt the Useful Work doctrine (`docs/superpowers/specs/2026-05-10-useful-work-doctrine-design.md`) as a chain-pinned document — `docs/USEFUL_WORK.md` exists, its hash is anchored, the `x/creed` module names UW + M1–M7 in Go-side canonical data, the cross-stack invariant skeleton is in place, and the meta-test enforces no drift between doc hash and registry. Phase 0 ships **zero behavioral bindings**: the doctrine pins UW so Phase 1+ have a contract.

**Architecture:** Mirrors the existing two-doctrine pattern (`TRUTH_SEEKING.md` + `TOK_SUBSTRATE.md`). Each doctrine has (a) a markdown file under `docs/`, (b) an off-chain hash file plus a verification script wired into `make creed-check`, (c) a Go-side canonical structure in `x/creed/types/` that names the commitments at build-time, (d) a cross-stack invariant test file that holds the binding harness. Phase 0 instantiates this pattern for USEFUL_WORK alongside the existing TRUTH_SEEKING infrastructure, leaving TC1–TC6 (ToK substrate) registration to a future Plan 5 of the ToK series.

**Tech Stack:** Go 1.24, Cosmos SDK v0.50, bash 3.2+ (macOS portable), sha256 via `shasum`/`sha256sum`/`openssl`.

**Spec:** `docs/superpowers/specs/2026-05-10-useful-work-doctrine-design.md` (commit `f829201`).

**Phase series:**
- Phase 0: **Doctrine adoption** — *this doc*
- Phase 1: `x/work` primitive (work-class registry, attestation lifecycle, reward formula, audit bounty pool)
- Phase 2..N: per-class registration plans (knowledge migration, counterexamples migration, training-run attestation, eval-suite execution, dataset curation, alignment artifacts, RL traces, synthetic data, kernel optimization)

---

## File Structure

**New files:**
- `docs/USEFUL_WORK.md` — the doctrine (promoted from spec, sans Status header & Appendices)
- `.useful-work-hash` — single sha256 of the normalized doctrine content
- `scripts/check_useful_work_hash.sh` — verification script, mirrors `scripts/check_creed_hash.sh`
- `x/creed/types/useful_work_creed.go` — Go-side canonical UW commitment + M1–M7 mechanism list
- `x/creed/types/useful_work_creed_test.go` — sanity tests for the canonical structure
- `tests/cross_stack/useful_work_invariants_test.go` — invariant harness with 7 skipped per-mechanism skeletons + 1 active meta-test

**Modified files:**
- `Makefile` — `creed-check` target runs both `check_creed_hash.sh` and `check_useful_work_hash.sh`
- `x/creed/doc.go` — adds the third-doctrine declaration
- `README.md` — extends the documentation table to name the third doctrine

**Out of scope (deferred to later phases):**
- `x/creed/types/genesis_creed.go` (`CanonicalCommitments` extension) — TC1–TC6 + UW will be added to this when Plan 5 of the ToK series unifies cross-doctrine registration. Phase 0 keeps UW in a *parallel* Go structure (`useful_work_creed.go`) to avoid touching the existing `PinnedCreed` proto/keeper.
- `x/work` module — Phase 1.
- Per-mechanism behavioral tests (M1–M7) — Phase 1+ as bindings land.

---

## Pre-Tasks: Read Before Starting

Skim these in order to absorb the patterns this plan mirrors:

- `docs/TRUTH_SEEKING.md` — the first doctrine. Note its 5-layer enforcement structure.
- `docs/TOK_SUBSTRATE.md` — the second doctrine.
- `docs/superpowers/specs/2026-05-10-useful-work-doctrine-design.md` — this plan's spec. Sections 1–8 are what becomes `docs/USEFUL_WORK.md`; the Status header and Appendices A/B are dropped on adoption.
- `scripts/check_creed_hash.sh` — the existing hash-check script. Read end-to-end (under 80 lines).
- `x/creed/types/genesis_creed.go` — `CanonicalCommitments` Go structure. Pattern for `UsefulWorkCommitment` + `CanonicalUsefulWorkMechanisms`.
- `tests/cross_stack/truth_seeking_invariants_test.go` lines 2080–2380 — `TestTruthSeeking_CreedAndContractStayInSync` meta-test. Pattern for `TestUsefulWork_DoctrineAndContractStayInSync`.
- `tests/cross_stack/tok_substrate_invariants_test.go` — TC1/TC2/TC3/TC5 invariant tests. Pattern for skeleton TestUW_M tests.
- `CLAUDE.md` — *Proto-Go Consistency Rule* (not load-bearing here since Phase 0 changes no protos), commit-directly-to-main convention.

---

## Tasks

### Task 1: Adopt spec as `docs/USEFUL_WORK.md`

**Files:**
- Create: `docs/USEFUL_WORK.md`
- Read for source: `docs/superpowers/specs/2026-05-10-useful-work-doctrine-design.md`

The new doctrine doc is the spec content with editorial scaffolding removed (Status header, Inception, deferred-context Appendix A and Appendix B). Sections 1–8 are the doctrine body; Appendix C ("Worked example") is illustrative and stays.

The procedure is a deterministic transform of the spec, performed at adoption time so spec-author decisions are preserved without plan/spec drift.

- [ ] **Step 1: Open the spec and copy the body**

Read `docs/superpowers/specs/2026-05-10-useful-work-doctrine-design.md`. Identify these section boundaries:

- **Drop**: the front matter (lines starting `# Useful Work Doctrine — Design Spec`, `**Status:**`, `**Inception:**`, and `**Doctrine series:**`) — replaced with a doctrine-style header (Step 2)
- **Keep**: `## 1. Doctrine identity` through `## 8. The discipline` (the doctrine body)
- **Drop**: `## Appendix A — Phase 0 implementation checklist` (deferred to this plan)
- **Drop**: `## Appendix B — Open questions deferred to Phase 1+ design` (deferred to Phase 1 spec)
- **Keep**: `## Appendix C — Worked example: a TOOL contribution end-to-end` (illustrative; helps readers see UW + M1–M7 end-to-end)

- [ ] **Step 2: Compose the new file**

Write `docs/USEFUL_WORK.md` with the following structure:

1. **Doctrine header** (replacing the spec's front matter):

```markdown
# Useful Work — the chain's metabolic identity

> Useful work is how ZERONE grows itself. This document pins one commitment, and everything that follows is mechanism in service of it.

Truth-seeking is what the chain *believes* (`docs/TRUTH_SEEKING.md`). ToK substrate is what the chain *sells* outward (`docs/TOK_SUBSTRATE.md`). **Useful work is how ZERONE grows itself.** The three doctrines bind through the same five-layer enforcement (test, position, voice, refusal, graph) and are mutually constitutive: truth-seeking produces the verified knowledge graph; ToK names that graph as the headline product; useful work pays for the compute that makes the graph richer, the verifications stronger, the reward attribution sharper, the chain itself more capable.

**We speak through intentions.** Every reward path either expresses UW or contradicts it. A trainer asking "what does this chain pay for?" should get one answer, in one voice, from every layer.

---

## Inception

This doctrine is declared at inception, 2026-05-10. Phase 0 ships zero behavioral bindings; the Go-side canonical structure (`x/creed/types/useful_work_creed.go`) and the cross-stack invariant harness (`tests/cross_stack/useful_work_invariants_test.go`) exist as the contract that subsequent phases must satisfy.

---
```

2. **Doctrine body** — copy spec sections **2 through 8 verbatim**, but renumber them so they replace the old section numbers (1 was "Doctrine identity" in the spec, which we've replaced with the header above):
   - Spec `## 2. The single commitment — UW` → output `## The single commitment — UW`
   - Spec `## 3. The six recursive axes` → output `## The six recursive axes`
   - Spec `## 4. The seven mechanisms (M1–M7)` → output `## The seven mechanisms`
     - Sub-headers `### M1.` through `### M7.` stay as-is.
   - Spec `## 5. Five-layer enforcement` → output `## How the commitment echoes` (rename to match the voice of `TOK_SUBSTRATE.md`'s analogous section)
     - Sub-headers `### Test layer`, `### Position layer`, etc. stay.
   - Spec `## 6. Phase mapping` → **DROP** (this is implementation roadmap, lives in this plan and in the spec; doctrine doesn't need it. The doctrine binds *what* the mechanisms are, not *which phase* delivers them.)
   - Spec `## 7. What this is not` → output `## What this is not`
   - Spec `## 8. The discipline` → output `## The discipline`

3. **Worked example** — copy spec `## Appendix C — Worked example: a TOOL contribution end-to-end` verbatim as `## Worked example — a TOOL contribution end-to-end` (drop the "Appendix C —" prefix; in the doctrine, it's a regular section).

4. **Footer** — preserve the inception line at the very bottom:

```markdown
— *Inception authored 2026-05-10. Free to evolve through bound mechanisms only. UW is indivisible.*
```

- [ ] **Step 3: Verify the file structure**

Run:
```bash
grep -nE "^# |^## " docs/USEFUL_WORK.md
```
Expected output (in this order):
```
1:# Useful Work — the chain's metabolic identity
N:## Inception
N:## The single commitment — UW
N:## The six recursive axes
N:## The seven mechanisms
N:## How the commitment echoes
N:## What this is not
N:## The discipline
N:## Worked example — a TOOL contribution end-to-end
```

(Exact line numbers vary; section count and order are what matter.)

- [ ] **Step 4: Verify mechanism headers**

Run:
```bash
grep -E "^### M[0-9]+\. " docs/USEFUL_WORK.md
```
Expected: 7 lines, M1 through M7, with names matching `CanonicalUsefulWorkMechanisms` from Task 5.

- [ ] **Step 5: Verify the file content sanity**

Run:
```bash
wc -l docs/USEFUL_WORK.md
grep -c "ZERONE is recursive" docs/USEFUL_WORK.md
```
Expected: roughly 100–200 lines (depending on Appendix C length); at least 1 occurrence of "ZERONE is recursive" (the UW statement that `creedtypes.UsefulWorkStatement` matches verbatim).

- [ ] **Step 6: Commit**

```bash
git add docs/USEFUL_WORK.md
git commit -m "$(cat <<'EOF'
docs(useful-work): adopt doctrine — UW (recursive) + M1-M7

Promotes spec content from docs/superpowers/specs/2026-05-10-
useful-work-doctrine-design.md (commit f829201) to the canonical
location. Drops Status header + Appendices A/B; promotes sections
1-8 to top-level. Hash will be pinned in .useful-work-hash in the
next task; cross-stack meta-test enforces no drift.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Compute and pin hash in `.useful-work-hash`

**Files:**
- Create: `.useful-work-hash`

The hash is sha256 of the file with carriage returns stripped (matching the existing `check_creed_hash.sh` normalization).

- [ ] **Step 1: Compute the hash**

Run:
```bash
tr -d '\r' < docs/USEFUL_WORK.md | shasum -a 256 | awk '{print $1}'
```
Expected: a 64-character hex string. Save it as the value to write in step 2.

- [ ] **Step 2: Write the hash file**

Write the 64-character hex string from step 1 to `.useful-work-hash` (no trailing newline necessary; existing `.creed-hash` has a single newline at end which `tr -d '[:space:]'` strips during read — the format is permissive).

```bash
tr -d '\r' < docs/USEFUL_WORK.md | shasum -a 256 | awk '{print $1}' > .useful-work-hash
```

- [ ] **Step 3: Verify the hash file**

Run: `cat .useful-work-hash | wc -c`
Expected: 65 (64 hex chars + 1 newline).

- [ ] **Step 4: Commit**

```bash
git add .useful-work-hash
git commit -m "$(cat <<'EOF'
docs(useful-work): pin doctrine hash in .useful-work-hash

Anchors docs/USEFUL_WORK.md content. Verification script
scripts/check_useful_work_hash.sh (next task) and the cross-stack
meta-test will fail if doc and hash drift apart.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Add `scripts/check_useful_work_hash.sh`

**Files:**
- Create: `scripts/check_useful_work_hash.sh`

Mirror of `scripts/check_creed_hash.sh`, pointed at `docs/USEFUL_WORK.md` and `.useful-work-hash`.

- [ ] **Step 1: Create the script**

```bash
#!/usr/bin/env bash
#
# check_useful_work_hash.sh — verify USEFUL_WORK.md has not drifted from
# the pinned hash in .useful-work-hash.
#
# Mirror of check_creed_hash.sh applied to the third doctrine. The on-
# chain pin lives in x/creed/types/useful_work_creed.go (Go-side build-
# time canonical structure); this script catches accidental drift in PRs
# even before the chain has the canonical record on-chain. The cross-
# stack meta-test TestUsefulWork_DoctrineAndContractStayInSync also
# enforces this in CI; the script provides the same enforcement on local
# dev machines via `make creed-check`.
#
# To intentionally amend the doctrine:
#   1. Edit docs/USEFUL_WORK.md.
#   2. Run this script — it will print the new computed hash.
#   3. Update .useful-work-hash with the new value.
#   4. Update x/creed/types/useful_work_creed.go if the mechanism count
#      changed (UW is indivisible — UW itself never changes).
#   5. Update tests/cross_stack/useful_work_invariants_test.go if any
#      TestUW_M* function names need to match new mechanism numbers.
#   6. Commit all five files together with a message naming what changed.
#
# Reviewers see the .useful-work-hash bump as the visible signal that
# the doctrine text has changed, prompting full diff review.

set -euo pipefail

DOCTRINE_FILE="docs/USEFUL_WORK.md"
HASH_FILE=".useful-work-hash"

if [ ! -f "$DOCTRINE_FILE" ]; then
  echo "error: $DOCTRINE_FILE not found"
  exit 1
fi
if [ ! -f "$HASH_FILE" ]; then
  echo "error: $HASH_FILE not found"
  exit 1
fi

hash_cmd() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 | awk '{print $NF}'
  else
    echo "error: need shasum, sha256sum, or openssl" >&2
    exit 1
  fi
}

ACTUAL=$(tr -d '\r' < "$DOCTRINE_FILE" | hash_cmd | awk '{print $1}')
EXPECTED=$(tr -d '[:space:]' < "$HASH_FILE")

if [ "$ACTUAL" != "$EXPECTED" ]; then
  cat <<EOF >&2
USEFUL_WORK.md hash check failed.

Expected (from $HASH_FILE): $EXPECTED
Actual (computed):          $ACTUAL

If you intentionally changed the doctrine, update $HASH_FILE to:
  $ACTUAL

The change should be visible in your PR diff so reviewers see both
the doctrine text change AND the hash bump in the same commit. UW is
indivisible — only mechanism content evolves.
EOF
  exit 1
fi

echo "useful-work hash check ok ($ACTUAL)"
```

- [ ] **Step 2: Make the script executable**

Run: `chmod +x scripts/check_useful_work_hash.sh`

- [ ] **Step 3: Run the script to verify it passes**

Run: `bash scripts/check_useful_work_hash.sh`
Expected: `useful-work hash check ok (<64-hex>)`.

- [ ] **Step 4: Commit**

```bash
git add scripts/check_useful_work_hash.sh
git commit -m "$(cat <<'EOF'
scripts(useful-work): add hash verification script

Mirror of check_creed_hash.sh for the third doctrine. Catches
USEFUL_WORK.md drift in PRs and via make creed-check.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Wire `make creed-check` to also verify useful-work hash

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Update the `creed-check` target**

Find the existing target at `Makefile:38-39`:

```makefile
creed-check:
	@bash scripts/check_creed_hash.sh
```

Replace with:

```makefile
creed-check:
	@bash scripts/check_creed_hash.sh
	@bash scripts/check_useful_work_hash.sh
```

- [ ] **Step 2: Verify the target runs both checks**

Run: `make creed-check`
Expected:
```
creed hash check ok (<64-hex>)
useful-work hash check ok (<64-hex>)
```

- [ ] **Step 3: Verify `pr-check` covers this** (no edit needed)

Run: `grep "pr-check:" Makefile`
Expected: `pr-check: lint test proto-check creed-check build` — confirms `creed-check` is invoked from `pr-check`, so the new doctrine hash is now PR-gated.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "$(cat <<'EOF'
build(makefile): creed-check verifies useful-work hash

The third doctrine's hash is now part of the same gate as the
truth-seeking creed hash. make creed-check (and therefore make
pr-check) catches drift in either doctrine.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Add `x/creed/types/useful_work_creed.go`

**Files:**
- Create: `x/creed/types/useful_work_creed.go`

A parallel Go canonical structure. Does NOT modify the existing `CanonicalCommitments` (truth-seeking only) or `PinnedCreed` proto. Phase 0 keeps UW data as build-time Go constants; on-chain pinning of UW is deferred to a future plan that unifies multi-doctrine registration on `PinnedCreed`.

- [ ] **Step 1: Create the file**

```go
package types

// UsefulWorkCommitment is the chain-built-in registration of the
// USEFUL_WORK doctrine's single commitment. Parallel to
// CanonicalCommitments (which holds truth-seeking commitments 1-20),
// kept separate because UW is structurally different — one commitment
// + N mechanisms, not N co-equal commitments.
//
// UW is fixed and indivisible: this constant never changes its value
// or its statement once shipped. Only mechanisms (M1-M7) extend.
const UsefulWorkCommitment = "UW"

// UsefulWorkStatement is the canonical short statement of UW.
// It must match the heading in docs/USEFUL_WORK.md exactly. The
// cross-stack meta-test TestUsefulWork_DoctrineAndContractStayInSync
// enforces this match.
const UsefulWorkStatement = "ZERONE is recursive"

// UsefulWorkMechanism describes one of the M1-M7 mechanisms that
// enforce UW. Number is the mechanism index (1..7); Name is the
// short label that must match the corresponding "### MN. <Name>"
// header in docs/USEFUL_WORK.md.
type UsefulWorkMechanism struct {
	Number uint32
	Name   string
}

// CanonicalUsefulWorkMechanisms is the canonical name-by-number
// registry of the seven mechanisms that enforce UW at the time this
// binary was built.
//
// To add a mechanism (NEVER to add a second co-equal commitment —
// that would dilute UW's indivisibility):
//  1. Add the "### MN. <Name>" section to docs/USEFUL_WORK.md.
//  2. Bump .useful-work-hash to the new sha256 of the normalized file.
//  3. Append the (Number, Name) pair to the slice below.
//  4. Add a binding TestUW_MN test in
//     tests/cross_stack/useful_work_invariants_test.go.
//  5. Wire the mechanism's voice (event attribute), refusal (error
//     message), and position (x/<module>/doc.go declaration).
//
// The TestUsefulWork_DoctrineAndContractStayInSync meta-test catches a
// step omitted from this list.
//
// Mechanism removal is a doctrine amendment requiring full governance
// passage (LIP class-registration revocation under M3). Mechanisms
// shipped at inception are load-bearing and do not retire.
var CanonicalUsefulWorkMechanisms = []UsefulWorkMechanism{
	{1, "Stake-backed claim"},
	{2, "Substrate-link mandate"},
	{3, "Class-specific verification under shared lifecycle"},
	{4, "Reward formula"},
	{5, "Recursion-weight projection over six axes"},
	{6, "Lineage propagates AND recurses"},
	{7, "The chain pays for the audit of its own useful work"},
}

// CanonicalRecursiveAxes is the canonical name-by-number registry of
// the six recursive axes that compose recursion-weight (M5). Axes are
// fixed by the doctrine — adding/removing an axis is a doctrine
// amendment requiring full governance passage and a hash bump.
//
// Per-axis weights and per-axis scoring formulas are governance-tunable
// at the parameter layer; only the axis identity is doctrinally fixed.
var CanonicalRecursiveAxes = []string{
	"substrate",
	"verification",
	"classification",
	"attribution",
	"tooling",
	"interface",
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/creed/types/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add x/creed/types/useful_work_creed.go
git commit -m "$(cat <<'EOF'
feat(creed): canonical UW commitment + M1-M7 mechanism registry

Go-side build-time registration of the USEFUL_WORK doctrine. Parallel
to CanonicalCommitments (truth-seeking, 1-20); UW kept separate
because its structure is one commitment + N mechanisms, not N
co-equal commitments. UW is indivisible — UsefulWorkCommitment and
UsefulWorkStatement constants never change. Mechanisms extend via
M1-M7+ as bound deliverables ship.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Add `x/creed/types/useful_work_creed_test.go`

**Files:**
- Create: `x/creed/types/useful_work_creed_test.go`

Sanity tests at the package level — no harness needed, no chain state. Catches obvious shape regressions.

- [ ] **Step 1: Write the tests**

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/creed/types"
)

func TestUsefulWorkCommitment_IsIndivisible(t *testing.T) {
	require.Equal(t, "UW", types.UsefulWorkCommitment,
		"UW commitment identifier must not change once shipped")
	require.Equal(t, "ZERONE is recursive", types.UsefulWorkStatement,
		"UW statement is doctrinally fixed")
}

func TestCanonicalUsefulWorkMechanisms_Count(t *testing.T) {
	require.Len(t, types.CanonicalUsefulWorkMechanisms, 7,
		"Phase 0 ships M1-M7; later phases add via M3 governance gate")
}

func TestCanonicalUsefulWorkMechanisms_NumberingDense(t *testing.T) {
	for i, m := range types.CanonicalUsefulWorkMechanisms {
		require.Equal(t, uint32(i+1), m.Number,
			"mechanism numbering must be dense and monotonic; index %d must hold M%d", i, i+1)
	}
}

func TestCanonicalUsefulWorkMechanisms_NamesNonEmpty(t *testing.T) {
	for _, m := range types.CanonicalUsefulWorkMechanisms {
		require.NotEmpty(t, m.Name, "mechanism M%d must have a non-empty name", m.Number)
	}
}

func TestCanonicalRecursiveAxes_SixAndOrdered(t *testing.T) {
	require.Equal(t, []string{
		"substrate",
		"verification",
		"classification",
		"attribution",
		"tooling",
		"interface",
	}, types.CanonicalRecursiveAxes,
		"the six recursive axes are doctrinally fixed in this exact order")
}
```

- [ ] **Step 2: Run tests to verify PASS**

Run: `go test ./x/creed/types/ -run "TestUsefulWork|TestCanonicalUsefulWork|TestCanonicalRecursiveAxes" -v`
Expected: PASS (5 tests).

- [ ] **Step 3: Commit**

```bash
git add x/creed/types/useful_work_creed_test.go
git commit -m "$(cat <<'EOF'
test(creed): sanity coverage on UW canonical structure

Five package-level tests catch obvious shape regressions: UW
identifier and statement are fixed; mechanism count is exactly 7
at inception; numbering is dense and monotonic; names non-empty;
axes are the canonical six in fixed order. Cross-stack meta-test
covers doc/hash consistency separately.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Update `x/creed/doc.go` to declare the third doctrine

**Files:**
- Modify: `x/creed/doc.go`

- [ ] **Step 1: Read the existing file**

Run: `cat x/creed/doc.go`
Note its current structure (length, voice). The Phase 0 update appends a USEFUL_WORK pointer alongside whatever existing TRUTH_SEEKING pointer it has.

- [ ] **Step 2: Append the USEFUL_WORK paragraph**

Add this paragraph to the package doc comment (above `package creed`), immediately after the existing TRUTH_SEEKING and TOK_SUBSTRATE references (or at the bottom of the comment block if those references don't yet exist):

```go
// USEFUL_WORK doctrine (docs/USEFUL_WORK.md) — the third in the trio.
// One commitment (UW: ZERONE is recursive) + seven mechanisms (M1-M7)
// + six recursive axes (substrate / verification / classification /
// attribution / tooling / interface). Canonical Go-side registration
// in x/creed/types/useful_work_creed.go; cross-stack invariant harness
// in tests/cross_stack/useful_work_invariants_test.go.
//
// Phase 0 (this commit's vintage) ships zero behavioral bindings.
// Phase 1 introduces the x/work module that binds M1-M4, M5 shape, M7;
// Phase 2+ adds per-class registrations (knowledge migration,
// counterexamples, training-run attestation, eval-suite execution,
// dataset curation, alignment artifacts, RL traces, synthetic data,
// kernel optimization). M6 (recursion-amplified lineage) extends
// TC6 (Plan 4 of ToK series) cross-class.
```

- [ ] **Step 3: Verify build**

Run: `go build ./x/creed/...`
Expected: clean.

- [ ] **Step 4: Verify go doc surface**

Run: `go doc ./x/creed | head -40`
Expected: the USEFUL_WORK paragraph appears in the package's go-doc output.

- [ ] **Step 5: Commit**

```bash
git add x/creed/doc.go
git commit -m "$(cat <<'EOF'
docs(creed): declare USEFUL_WORK as third doctrine in trio

Position-layer pointer to the new doctrine + canonical registration
file + invariant harness. No behavioral changes; Phase 0 is doctrine-
adoption only.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Create cross-stack invariant harness skeleton

**Files:**
- Create: `tests/cross_stack/useful_work_invariants_test.go`

Skeleton with the file-level docstring, the imports, the seven `TestUW_M{1..7}` skipped tests, and the `TestUsefulWork_DoctrineAndContractStayInSync` meta-test (active).

- [ ] **Step 1: Create the file**

```go
package cross_stack_test

// Useful-work invariants. Each TestUW_MN test in this file binds one
// mechanism from docs/USEFUL_WORK.md. The file's meta-test
// TestUsefulWork_DoctrineAndContractStayInSync enforces no drift
// between the doctrine (markdown), the canonical Go registration
// (x/creed/types/useful_work_creed.go), the on-disk hash
// (.useful-work-hash), and the test scaffold (this file).
//
// Phase 0 (this commit's vintage) ships:
//   - The meta-test (active; passes if doctrine + registry + hash + tests stay aligned)
//   - Seven skeleton TestUW_M1..M7 tests, each calling t.Skip("Phase 1 binding pending")
//
// Phase 1+ replaces the t.Skip body with real bindings as the x/work
// primitive and per-class plans land.
//
// Cross-doctrine integrity (USEFUL_WORK + TRUTH_SEEKING + TOK_SUBSTRATE
// staying mutually consistent) is enforced by Plan 5 of the ToK series
// when it adds TestToKSubstrate_DoctrineAndContractStayInSync; that
// future test will read this file and the truth-seeking invariant file
// to confirm cross-doctrine echoes match.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	creedtypes "github.com/zerone-chain/zerone/x/creed/types"
)

// ════════════════════════════════════════════════════════════════════
// Per-mechanism skeleton tests. Each is skipped at inception; Phase 1+
// replaces t.Skip with the actual binding. Test-name format MUST be
// TestUW_M<N>_<ShortName> where N matches the mechanism number in
// CanonicalUsefulWorkMechanisms; the meta-test below uses this format
// to verify every mechanism has a binding test.
// ════════════════════════════════════════════════════════════════════

// M1: Stake-backed claim.
// Phase 1 binding: x/work primitive enforces claim-stake invariants —
// agents staking ZRN against work claims; correctness pays the stake
// back plus reward; fraud slashes the stake.
func TestUW_M1_StakeBackedClaim(t *testing.T) {
	t.Skip("Phase 1 binding pending — x/work primitive will bind M1")
}

// M2: Substrate-link mandate.
// Phase 1 binding: x/work attestation refuses settlement when the
// claim's substrate-link is missing or fails re-derivation; reward
// stays zero regardless of recursion-weight claimed.
func TestUW_M2_SubstrateLinkMandate(t *testing.T) {
	t.Skip("Phase 1 binding pending — x/work substrate-link gate will bind M2")
}

// M3: Class-specific verification under shared lifecycle.
// Phase 1 binding: work-class registry enforces commit→reveal→verify
// →settle lifecycle across all classes; class-specific judgment
// localized to verify phase. Class registration is governance-gated.
func TestUW_M3_ClassSpecificVerificationSharedLifecycle(t *testing.T) {
	t.Skip("Phase 1 binding pending — x/work class registry will bind M3")
}

// M4: Reward formula R = base + L × W × Q.
// Phase 1 binding: reward-accounting layer applies the formula;
// non-recursive verified work receives base only; substrate-link zero
// produces total zero; recursion-weight scales the dominant share.
func TestUW_M4_RewardFormula(t *testing.T) {
	t.Skip("Phase 1 binding pending — x/work reward-accounting will bind M4")
}

// M5: Recursion-weight projection over six axes.
// Phase 1 binding: per-axis decomposition stored on attestation
// record forward-only; W = Σ axis_weight_i × axis_score_i; identity
// scorers at Phase 1, real scorers in Phase 2+ pathway plans.
func TestUW_M5_RecursionWeightProjection(t *testing.T) {
	t.Skip("Phase 1 binding pending — x/work recursion-weight projector will bind M5 shape; per-axis scorers in Phase 2+")
}

// M6: Lineage propagates AND recurses.
// Phase 4 (ToK series TC6) extended cross-class binding: a dataset
// trained-on by a model that helps verify substrate contributes to
// both the dataset's royalties AND back to the original facts.
func TestUW_M6_LineagePropagatesAndRecurses(t *testing.T) {
	t.Skip("Phase 4 (ToK TC6 extension) binding pending — cross-class lineage will bind M6")
}

// M7: The chain pays for the audit of its own useful work.
// Phase 1 binding: useful_work_audit_bounty_pool module account
// mints uzrn per block (Minter-permissioned, rate-capped); challenge
// stakers pay-out from the pool on successful challenge.
func TestUW_M7_AuditBountyPool(t *testing.T) {
	t.Skip("Phase 1 binding pending — useful_work_audit_bounty_pool will bind M7")
}

// ════════════════════════════════════════════════════════════════════
// Meta-test (active at Phase 0). Verifies the doctrine, the Go
// registration, the on-disk hash, and the test scaffold stay in sync.
// ════════════════════════════════════════════════════════════════════

// TestUsefulWork_DoctrineAndContractStayInSync is the binding meta-test
// for Phase 0 of the USEFUL_WORK doctrine. It enforces:
//
//  1. Hash agreement: the sha256 of docs/USEFUL_WORK.md (stripped of
//     CRs to match the script's normalization) matches the value in
//     .useful-work-hash.
//
//  2. Mechanism count agreement: the number of "### MN." headers in
//     the doctrine equals len(CanonicalUsefulWorkMechanisms).
//
//  3. Mechanism name agreement: each "### MN. <Name>" header in the
//     doctrine matches CanonicalUsefulWorkMechanisms[N-1].Name
//     (modulo trailing punctuation / whitespace).
//
//  4. Test-name agreement: this file contains a TestUW_M<N>_* function
//     for every mechanism number 1..len(CanonicalUsefulWorkMechanisms).
//
//  5. UW-statement agreement: the doctrine contains the exact
//     UsefulWorkStatement string verbatim.
//
// If any of these fail, the doctrine and the contract have drifted.
// Either the doctrine was edited without updating the registry/tests,
// or the registry/tests were edited without updating the doctrine.
// Both must move together.
//
// Phase 1+ extends this test to also verify position-layer (x/*/doc.go),
// voice-layer (event attributes useful_work_commitment="UW" and
// mechanism="MN"), and refusal-layer (error messages naming UW + MN).
// At Phase 0 those layers don't exist yet; the meta-test only checks
// what's been bound.
func TestUsefulWork_DoctrineAndContractStayInSync(t *testing.T) {
	doctrinePath := "../../docs/USEFUL_WORK.md"
	hashPath := "../../.useful-work-hash"

	doctrineBytes, err := os.ReadFile(doctrinePath)
	require.NoError(t, err, "doctrine must exist; if you renamed or moved it, update this test")
	doctrine := string(doctrineBytes)

	// ─── Check 1: hash agreement ─────────────────────────────────────
	// Match scripts/check_useful_work_hash.sh: strip CR before hashing.
	normalized := strings.ReplaceAll(doctrine, "\r", "")
	sum := sha256.Sum256([]byte(normalized))
	actualHash := hex.EncodeToString(sum[:])

	hashBytes, err := os.ReadFile(hashPath)
	require.NoError(t, err, ".useful-work-hash must exist; run scripts/check_useful_work_hash.sh to bootstrap")
	expectedHash := strings.TrimSpace(string(hashBytes))

	require.Equal(t, expectedHash, actualHash,
		"docs/USEFUL_WORK.md hash drift: .useful-work-hash says %s but normalized doc hashes to %s. "+
			"Update .useful-work-hash if the doctrine change is intentional.",
		expectedHash, actualHash)

	// ─── Check 2: mechanism count agreement ──────────────────────────
	mechanismHeaderRe := regexp.MustCompile(`(?m)^### M(\d+)\. `)
	matches := mechanismHeaderRe.FindAllStringSubmatch(doctrine, -1)
	require.Len(t, matches, len(creedtypes.CanonicalUsefulWorkMechanisms),
		"doctrine has %d '### MN.' headers but CanonicalUsefulWorkMechanisms has %d entries; "+
			"add or remove the mechanism in BOTH places",
		len(matches), len(creedtypes.CanonicalUsefulWorkMechanisms))

	// ─── Check 3: mechanism name agreement ───────────────────────────
	// Extract each "### MN. <Name>" header with full name segment up to
	// end-of-line, then compare against CanonicalUsefulWorkMechanisms[N-1].Name.
	headerRe := regexp.MustCompile(`(?m)^### M(\d+)\. (.+)$`)
	headerMatches := headerRe.FindAllStringSubmatch(doctrine, -1)
	require.Len(t, headerMatches, len(creedtypes.CanonicalUsefulWorkMechanisms),
		"doctrine '### MN. <Name>' header parse mismatch")

	for _, m := range headerMatches {
		num, convErr := strconv.Atoi(m[1])
		require.NoError(t, convErr, "non-numeric mechanism index in doctrine: %q", m[1])
		require.Greater(t, num, 0, "mechanism number must be ≥ 1")
		require.LessOrEqual(t, num, len(creedtypes.CanonicalUsefulWorkMechanisms),
			"doctrine cites M%d but CanonicalUsefulWorkMechanisms only has %d entries",
			num, len(creedtypes.CanonicalUsefulWorkMechanisms))

		expectedName := creedtypes.CanonicalUsefulWorkMechanisms[num-1].Name
		actualName := strings.TrimSpace(m[2])
		require.Equal(t, expectedName, actualName,
			"M%d name drift: doctrine says %q but CanonicalUsefulWorkMechanisms says %q",
			num, actualName, expectedName)
	}

	// ─── Check 4: test-name agreement ────────────────────────────────
	testFileBytes, err := os.ReadFile("useful_work_invariants_test.go")
	require.NoError(t, err)
	testContent := string(testFileBytes)

	for _, mech := range creedtypes.CanonicalUsefulWorkMechanisms {
		needle := "func TestUW_M" + strconv.Itoa(int(mech.Number)) + "_"
		require.Contains(t, testContent, needle,
			"M%d (%s) has no TestUW_M%d_* function in this file; add a binding test or remove the mechanism",
			mech.Number, mech.Name, mech.Number)
	}

	// ─── Check 5: UW-statement agreement ─────────────────────────────
	require.Contains(t, doctrine, creedtypes.UsefulWorkStatement,
		"docs/USEFUL_WORK.md must contain the verbatim UW statement %q; "+
			"if the statement has been amended, update both the doctrine and "+
			"creedtypes.UsefulWorkStatement (UW is doctrinally indivisible — "+
			"this should require a governance-gated doctrine amendment)",
		creedtypes.UsefulWorkStatement)
}
```

- [ ] **Step 2: Run the meta-test**

Run: `go test ./tests/cross_stack/ -run TestUsefulWork_DoctrineAndContractStayInSync -v`
Expected: PASS.

- [ ] **Step 3: Run the skipped tests to confirm they're correctly skipped (not failing)**

Run: `go test ./tests/cross_stack/ -run "TestUW_M[1-7]_" -v`
Expected: 7 tests, all SKIP with "Phase N binding pending" reasons.

- [ ] **Step 4: Commit**

```bash
git add tests/cross_stack/useful_work_invariants_test.go
git commit -m "$(cat <<'EOF'
test(cross_stack): useful-work invariant skeleton + meta-test

Seven TestUW_M{1..7} skeleton tests (skipped, "Phase N binding
pending") + one active meta-test TestUsefulWork_DoctrineAndContract
StayInSync. Meta-test enforces hash + mechanism count + name +
test-name + UW-statement agreement. Phase 0 ships zero behavioral
bindings; Phase 1+ replaces t.Skip bodies with real bindings.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Update `README.md` to mention the third doctrine

**Files:**
- Modify: `README.md`

The current README has a Documentation table including TRUTH_SEEKING.md and TOK_SUBSTRATE.md. Add USEFUL_WORK.md as the third row.

- [ ] **Step 1: Find the documentation table**

Run: `grep -n "TRUTH_SEEKING\|TOK_SUBSTRATE\|Documentation" README.md`
Expected: a table near the bottom of the README listing markdown docs in `docs/`.

- [ ] **Step 2: Add the USEFUL_WORK row**

Insert immediately after the `[ToK Substrate](docs/TOK_SUBSTRATE.md)` row:

```markdown
| [Useful Work](docs/USEFUL_WORK.md) | The chain's metabolic doctrine — UW (recursive) + 7 mechanisms |
```

- [ ] **Step 3: Update the "Read first" callout**

Find the line near the top of the README:
```markdown
> **Read first:** [docs/TRUTH_SEEKING.md](docs/TRUTH_SEEKING.md) — the chain's epistemological commitments...
```

Append a sibling pointer (do NOT replace — TRUTH_SEEKING stays the primary first-read since it's the substrate):

```markdown
> **Read first:** [docs/TRUTH_SEEKING.md](docs/TRUTH_SEEKING.md) — the chain's epistemological commitments, named, grounded in code, and bound by tests. Truth-seeking is the substrate, not a feature. We speak through intentions.
>
> **Then:** [docs/TOK_SUBSTRATE.md](docs/TOK_SUBSTRATE.md) (what the chain *sells*) and [docs/USEFUL_WORK.md](docs/USEFUL_WORK.md) (how the chain *grows itself*) — the trio is mutually constitutive.
```

- [ ] **Step 4: Verify the file renders**

Run: `head -20 README.md`
Expected: the "Read first" + "Then" callout block at the top.

Run: `grep -A 2 "Useful Work" README.md`
Expected: the Documentation table row appears.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs(readme): introduce USEFUL_WORK as third doctrine

Documentation table gains a USEFUL_WORK.md row. Top-of-README
"Read first" callout extends to point readers to the trio: truth-
seeking (substrate), ToK (what's sold), useful work (how it grows).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Add Plan-5 follow-up marker

**Files:**
- Modify: `tests/cross_stack/tok_substrate_invariants_test.go`

A comment block at the top of the file naming what Plan 5 of the ToK series will need to add when it lands. This isn't a code change — it's a marker for the future plan author.

- [ ] **Step 1: Find the top of the file**

Run: `head -10 tests/cross_stack/tok_substrate_invariants_test.go`
Expected: package declaration and existing imports.

- [ ] **Step 2: Append a top-of-file marker comment**

Add this comment block immediately after the package declaration (at the very top of the file, before any imports or type declarations):

```go
// ════════════════════════════════════════════════════════════════════
// FOLLOW-UP NOTE for Plan 5 of the ToK series
// ════════════════════════════════════════════════════════════════════
//
// When Plan 5 lands TestToKSubstrate_DoctrineAndContractStayInSync, it
// must extend its scope to ALSO check the third doctrine
// (docs/USEFUL_WORK.md) for the same five-layer integrity it enforces
// on TOK_SUBSTRATE.md. The two doctrines are mutually constitutive
// (TRUTH_SEEKING, TOK_SUBSTRATE, USEFUL_WORK) and should not drift
// independently.
//
// Specifically, Plan 5 should add (or coordinate with the existing
// TestUsefulWork_DoctrineAndContractStayInSync in
// useful_work_invariants_test.go):
//
//   - Cross-doctrine echo verification: every "Echoes:" reference in
//     USEFUL_WORK.md UW (currently citing commitments 11, 12, TC1,
//     TC6) must point to a real commitment in the cited doctrine. If
//     UW.Echoes mentions TC6 but TOK_SUBSTRATE.md no longer has TC6,
//     fail.
//
//   - Hash-bundle integrity: a single make target (or extension of
//     make creed-check) that fails fast on ANY of the three hashes
//     drifting. Today make creed-check covers .creed-hash and
//     .useful-work-hash; Plan 5 should add .tok-substrate-hash and
//     verify all three together.
//
//   - Position-layer cross-coverage: every commitment in the unified
//     registry (truth-seeking 1-20 + TC1-TC6 + UW + M1-M7) is
//     declared in at least one x/*/doc.go.
//
// This marker is not a TODO comment in the conventional sense — Phase 0
// of USEFUL_WORK does not ship Plan 5; Plan 5 is the closure plan for
// the ToK series and will be authored separately. This marker exists so
// the Plan 5 author finds the cross-doctrine integration requirement
// without having to grep across plans.
//
// Reference: docs/superpowers/specs/2026-05-10-useful-work-doctrine-
// design.md, section 5 ("Graph layer"), and docs/USEFUL_WORK.md
// "How the commitment echoes" section.
// ════════════════════════════════════════════════════════════════════
```

- [ ] **Step 3: Verify build**

Run: `go build ./tests/cross_stack/...`
Expected: clean (comments don't affect build).

- [ ] **Step 4: Run the existing TC tests to confirm no regression**

Run: `go test ./tests/cross_stack/ -run "TestToKSubstrate" -v`
Expected: existing TC1, TC2, TC3, TC5 tests still PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/cross_stack/tok_substrate_invariants_test.go
git commit -m "$(cat <<'EOF'
test(cross_stack): mark Plan 5 follow-up — extend doctrine sync to USEFUL_WORK

Top-of-file comment for Plan 5 of the ToK series, naming the cross-
doctrine integrity requirements when TestToKSubstrate_DoctrineAnd
ContractStayInSync lands. Future Plan 5 author will see this marker
and pick up the unified registry / cross-echo verification work
without grepping across plans.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Final integration sweep

- [ ] **Step 1: Run all creed/useful-work-related tests**

Run: `go test ./x/creed/... ./tests/cross_stack/ -run "TestUsefulWork|TestUW_|TestCanonicalUsefulWork|TestRecursive|TestTruthSeeking_Creed|TestToKSubstrate" -v -count=1`
Expected: PASS (with TestUW_M{1..7} marked SKIP). No FAILures.

- [ ] **Step 2: Run hash check**

Run: `make creed-check`
Expected:
```
creed hash check ok (<truth-seeking-hash>)
useful-work hash check ok (<useful-work-hash>)
```

- [ ] **Step 3: Run the full pre-PR check**

Run: `make pr-check`
Expected: PASS — covers lint, test, proto-check, creed-check, build.

If `make test` (300s timeout) is too slow on local hardware, narrow it:
Run: `go test -timeout 120s ./x/creed/... ./tests/cross_stack/...`

- [ ] **Step 4: Verify the full set of new files**

Run:
```bash
git log --oneline --since="2026-05-10 00:00:00" -- docs/USEFUL_WORK.md .useful-work-hash scripts/check_useful_work_hash.sh Makefile x/creed/types/useful_work_creed.go x/creed/types/useful_work_creed_test.go x/creed/doc.go tests/cross_stack/useful_work_invariants_test.go tests/cross_stack/tok_substrate_invariants_test.go README.md
```
Expected: 10 commits (one per task 1-10), in chronological order, each with a clear scope tag.

- [ ] **Step 5: Push (only if user authorizes)**

Phase 0 commits land directly on `main` per CLAUDE.md convention. Do NOT push without explicit user authorization — this is the project's rule (see system prompt: confirm pushes).

- [ ] **Step 6: Hand off to Phase 1**

Phase 0 is complete. The chain now:
- Has the third doctrine pinned at `docs/USEFUL_WORK.md`
- Has the hash anchored in `.useful-work-hash`, verified by `make creed-check`
- Has the Go-side canonical structure at `x/creed/types/useful_work_creed.go`
- Has the cross-stack invariant harness at `tests/cross_stack/useful_work_invariants_test.go` with 7 skeleton TestUW_M{1..7} tests skipped + 1 active meta-test
- Has README.md and `x/creed/doc.go` declarations of the third doctrine

The next plan in the series is **Phase 1: x/work primitive**, which binds M1, M2, M3, M4, M5 (shape), and M7. Phase 1 should be brainstormed → spec'd → planned in a separate cycle. Phase 4 of the ToK series (TC6 lineage) is a prerequisite of binding M6 (cross-class lineage extension).

---

## Self-Review

After implementing all tasks, verify:

1. **Spec coverage:** Each item in the spec's Appendix A is covered:
   - "Adopt this content as `docs/USEFUL_WORK.md`" → Task 1
   - "Compute hash and update `.creed-hash`" → Tasks 2-4 (note: spec said `.creed-hash` but Phase 0 uses `.useful-work-hash` to keep doctrines independently versioned; the meta-test and `make creed-check` enforce both)
   - "Register UW in `x/creed` module's commitment registry" → Tasks 5-7
   - "Create skeleton `tests/cross_stack/useful_work_invariants_test.go`" → Task 8
   - "Update `README.md`" → Task 9
   - "Extend Plan 5 of ToK series" follow-up marker → Task 10

2. **Position layer present:** `x/creed/doc.go` and `tests/cross_stack/useful_work_invariants_test.go` declare the doctrine. `x/work/doc.go` is Phase 1; not in scope here.

3. **Voice layer present:** Phase 0 ships zero events (no behavioral bindings yet). The doctrine NAMES the events Phase 1 must emit (`useful_work_attested`, `useful_work_settled`, `recursion_weight_computed`); Phase 1 binds them.

4. **Refusal layer present:** Phase 0 ships zero error sites. The doctrine NAMES the refusal vocabulary Phase 1 must use; Phase 1 binds it.

5. **Graph layer present:** UW.Echoes references commitments 11, 12, TC1, TC6 (in the doctrine doc). The Plan 5 marker (Task 10) flags that cross-doctrine echo verification is needed when Plan 5 lands.

6. **All tests green:** TestUsefulWork_DoctrineAndContractStayInSync passes; TestUW_M{1..7} skipped (correct for Phase 0); existing TruthSeeking + ToK tests unaffected.

7. **`make pr-check` PASS:** lint + test + proto-check + creed-check (now covers both hashes) + build all green.

---

## What This Plan Does Not Do

- **No `x/work` module.** Phase 1 spec/plan introduces the work-class registry, attestation lifecycle, reward-accounting layer, and useful_work_audit_bounty_pool.
- **No M1–M7 behavioral bindings.** All seven TestUW_M{1..7} tests are skipped at Phase 0. Phase 1 binds M1, M2, M3, M4, M5 (shape), M7; Phase 4 of ToK series binds M6.
- **No per-class registrations.** Knowledge migration, counterexamples migration, training-run attestation, eval-suite execution, dataset curation, alignment artifacts, RL traces, synthetic data, kernel optimization — each is a Phase 2+ plan.
- **No on-chain `PinnedCreed` extension.** Phase 0 keeps UW data in a parallel Go structure (`x/creed/types/useful_work_creed.go`) instead of extending `CanonicalCommitments` or modifying the `PinnedCreed` proto. A future plan unifies multi-doctrine on-chain registration; Phase 0 minimally avoids touching the existing keeper/proto.
- **No `.creed-hash` format change.** `.creed-hash` continues to hold the truth-seeking-only hash; `.useful-work-hash` is parallel. A future plan may unify into a single multi-line hash file; Phase 0 minimally avoids touching the existing script's `tr -d '[:space:]'` collapse logic.
- **No `docs/TOK_SUBSTRATE.md` hash.** TOK_SUBSTRATE remains unhashed at Phase 0. A future plan (likely Plan 5 of the ToK series) extends `make creed-check` with `.tok-substrate-hash`.

— *Plan authored 2026-05-10. Phase 0 is the smallest of the phases; subsequent phases ship the actual bindings UW commits to.*

---

## This document is a Contribution

This plan is itself a `Contribution` of class `PIPELINE_IMPROVEMENT`, lifecycle phase `SUBSTRATE`. Its content-hash is pinned at `.phase-0-plan-hash`. The chain pays for its own design and execution; this document is among the work.
