package cross_stack_test

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
// Per-mechanism tests. SL-M1 active; SL-M2..M6 skipped.
// ════════════════════════════════════════════════════════════════════

func TestStrangeLoop_SL_M1_DoctrineImport(t *testing.T) {
	h := NewTestHarness(t)
	// Per SLA-16 finding: harness Ctx doesn't auto-run InitGenesis;
	// the test calls LoadDoctrineFacts explicitly. Loader is idempotent.
	require.NoError(t, h.KnowledgeKeeper.LoadDoctrineFacts(h.Ctx))

	_, found := h.KnowledgeKeeper.GetFact(h.Ctx, "commitment-SL")
	require.True(t, found, "SL-M1: commitment-SL must be queryable post-genesis")
	_, found = h.KnowledgeKeeper.GetFact(h.Ctx, "mechanism-SL-M1")
	require.True(t, found, "SL-M1: mechanism-SL-M1 itself must be queryable")
	_, found = h.KnowledgeKeeper.GetFact(h.Ctx, "commitment-UW")
	require.True(t, found, "SL-M1: commitment-UW must be queryable")
}

func TestStrangeLoop_SL_M2_ProtocolAsSubstrate(t *testing.T) {
	t.Skip("Phase SL-β binding pending — x/authorship will bind SL-M2")
}

func TestStrangeLoop_SL_M3_GovernanceLift(t *testing.T) {
	t.Skip("Phase SL-γ binding pending — LIPs become attestations")
}

func TestStrangeLoop_SL_M4_AuthorLineage(t *testing.T) {
	t.Skip("Phase SL-δ binding pending — depends on SL-M2 + SL-M3")
}

func TestStrangeLoop_SL_M5_SelfVerification(t *testing.T) {
	t.Skip("Phase SL-ε binding pending — validators query ToK at verify time")
}

func TestStrangeLoop_SL_M6_OriginAttestation(t *testing.T) {
	t.Skip("Phase SL-ζ binding pending — genesis as first attestation")
}

// ════════════════════════════════════════════════════════════════════
// Meta-test (active at Phase SL-α).
// ════════════════════════════════════════════════════════════════════

func TestStrangeLoop_DoctrineAndContractStayInSync(t *testing.T) {
	doctrinePath := "../../docs/STRANGE_LOOP.md"
	hashPath := "../../.strange-loop-hash"

	doctrineBytes, err := os.ReadFile(doctrinePath)
	require.NoError(t, err)
	doctrine := string(doctrineBytes)

	// Check 1: hash agreement.
	normalized := strings.ReplaceAll(doctrine, "\r", "")
	sum := sha256.Sum256([]byte(normalized))
	actualHash := hex.EncodeToString(sum[:])

	hashBytes, err := os.ReadFile(hashPath)
	require.NoError(t, err)
	expectedHash := strings.TrimSpace(string(hashBytes))

	require.Equal(t, expectedHash, actualHash,
		"docs/STRANGE_LOOP.md hash drift: .strange-loop-hash says %s but doc hashes to %s",
		expectedHash, actualHash)

	// Check 2: mechanism count.
	mechanismHeaderRe := regexp.MustCompile(`(?m)^### SL-M(\d+)\. `)
	matches := mechanismHeaderRe.FindAllStringSubmatch(doctrine, -1)
	require.Len(t, matches, len(creedtypes.CanonicalStrangeLoopMechanisms),
		"doctrine has %d '### SL-MN.' headers but CanonicalStrangeLoopMechanisms has %d entries",
		len(matches), len(creedtypes.CanonicalStrangeLoopMechanisms))

	// Check 3: mechanism name agreement.
	headerRe := regexp.MustCompile(`(?m)^### SL-M(\d+)\. (.+)$`)
	headerMatches := headerRe.FindAllStringSubmatch(doctrine, -1)
	for _, m := range headerMatches {
		num, convErr := strconv.Atoi(m[1])
		require.NoError(t, convErr)
		require.Greater(t, num, 0)
		require.LessOrEqual(t, num, len(creedtypes.CanonicalStrangeLoopMechanisms))

		expectedName := creedtypes.CanonicalStrangeLoopMechanisms[num-1].Name
		actualName := strings.TrimSpace(m[2])
		require.Equal(t, expectedName, actualName,
			"SL-M%d name drift: doctrine says %q but registry says %q",
			num, actualName, expectedName)
	}

	// Check 4: test-name agreement.
	testFileBytes, err := os.ReadFile("strange_loop_invariants_test.go")
	require.NoError(t, err)
	testContent := string(testFileBytes)

	for _, mech := range creedtypes.CanonicalStrangeLoopMechanisms {
		needle := "func TestStrangeLoop_SL_M" + strconv.Itoa(int(mech.Number)) + "_"
		require.Contains(t, testContent, needle,
			"SL-M%d (%s) has no TestStrangeLoop_SL_M%d_* function",
			mech.Number, mech.Name, mech.Number)
	}

	// Check 5: SL-statement agreement.
	require.Contains(t, doctrine, creedtypes.StrangeLoopStatement,
		"docs/STRANGE_LOOP.md must contain the verbatim SL statement %q",
		creedtypes.StrangeLoopStatement)
}
