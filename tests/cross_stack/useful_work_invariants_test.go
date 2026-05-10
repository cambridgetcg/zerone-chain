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
