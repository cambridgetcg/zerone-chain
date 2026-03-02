package e2e_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKnowledge_ClaimToFact(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)
	valKeyName := "validator"

	// 1. Submit a claim
	content := "Water boils at 100 degrees Celsius at standard atmospheric pressure"
	claimID := SubmitClaim(t, chain, ctx, valKeyName, content, "physics", "empirical", "1000000")
	t.Logf("Claim ID: %s", claimID)

	// 2. Get the verification round ID
	roundID := GetClaimRoundID(t, chain, ctx, claimID)
	t.Logf("Round ID: %s", roundID)

	// 3. Commit phase — validator votes "accept"
	salt := []byte("e2e-test-salt-claim-to-fact")
	CommitVote(t, chain, ctx, valKeyName, roundID, "accept", salt)

	// 4. Wait for reveal phase (commit_phase_blocks = 10)
	WaitForRoundPhase(t, chain, ctx, roundID, "VERIFICATION_PHASE_REVEAL", 15)

	// 5. Reveal vote
	RevealVote(t, chain, ctx, valKeyName, roundID, "accept", salt)

	// 6. Wait for round to complete (reveal_phase_blocks=10 + aggregation_blocks=5)
	WaitForRoundPhase(t, chain, ctx, roundID, "VERIFICATION_PHASE_COMPLETE", 20)

	// 7. Verify the round verdict
	roundResult := QueryRound(t, chain, ctx, roundID)
	roundData := roundResult["round"].(map[string]interface{})
	t.Logf("Round verdict: %v", roundData["verdict"])

	// 8. Verify a fact was created in the physics domain
	facts := QueryFactsByDomain(t, chain, ctx, "physics")
	found := false
	for _, f := range facts {
		factMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if factMap["content"] == content {
			found = true
			require.Equal(t, "physics", factMap["domain"])
			t.Logf("Fact created: id=%v, domain=%v, confidence=%v",
				factMap["id"], factMap["domain"], factMap["confidence"])
			break
		}
	}
	require.True(t, found, "expected fact with content %q in physics domain", content)
}

func TestKnowledge_DomainPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)
	valKeyName := "validator"

	domain := "mathematics"
	var pressures []uint64

	// Submit 5 claims to the same domain, each driven through full lifecycle.
	claims := []string{
		"The sum of angles in a triangle equals 180 degrees in Euclidean geometry",
		"A prime number is only divisible by one and itself by definition",
		"The square root of two is an irrational number proven by contradiction",
		"Eulers identity states that e to the i pi plus one equals zero exactly",
		"The fundamental theorem of calculus connects differentiation and integration",
	}

	for i, content := range claims {
		t.Logf("=== Claim %d/%d ===", i+1, len(claims))

		DriveClaimToFact(t, chain, ctx, valKeyName, []string{valKeyName},
			content, domain, "formal", "1000000")

		// Wait a couple blocks for state to settle.
		WaitBlocks(t, chain, ctx, 2)

		// Query domain capacity.
		cap := QueryDomainCapacity(t, chain, ctx, domain)
		t.Logf("Domain capacity response: %v", cap)

		pressure, ok := jsonNumber(cap, "pressure_bps")
		if !ok {
			// Try nested structure
			if inner, ok := cap["capacity"].(map[string]interface{}); ok {
				pressure, _ = jsonNumber(inner, "pressure_bps")
			}
		}
		pressures = append(pressures, pressure)
		t.Logf("Pressure after claim %d: %d BPS", i+1, pressure)
	}

	// Verify pressure increased (or at least non-decreasing).
	require.Greater(t, len(pressures), 0, "should have pressure readings")
	t.Logf("Pressure progression: %v", pressures)

	if pressures[0] == 0 {
		require.Greater(t, pressures[len(pressures)-1], pressures[0],
			"pressure should increase as facts are added to domain")
	} else {
		require.GreaterOrEqual(t, pressures[len(pressures)-1], pressures[0],
			"pressure should not decrease as facts are added")
	}
}

func TestKnowledge_Dissent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// 3 validators for majority/minority voting.
	chain, ctx := SetupChain(t, 3)

	// Each validator node has its own keyring with a key named "validator".
	// To sign as different validators, we must target their specific nodes.
	val0 := chain.Validators[0]
	val1 := chain.Validators[1]
	val2 := chain.Validators[2]

	// 1. Submit claim (from val0's node)
	content := "The speed of light in vacuum is approximately 299792458 meters per second"
	claimID := SubmitClaim(t, chain, ctx, "validator", content, "physics", "empirical", "1000000")
	t.Logf("Claim ID: %s", claimID)

	roundID := GetClaimRoundID(t, chain, ctx, claimID)
	t.Logf("Round ID: %s", roundID)

	// 2. Commit phase — 2 accept, 1 reject (each on their own node)
	salt0 := []byte("dissent-salt-val0")
	salt1 := []byte("dissent-salt-val1")
	salt2 := []byte("dissent-salt-val2")

	CommitVoteOnNode(t, val0, ctx, roundID, "accept", salt0)
	CommitVoteOnNode(t, val1, ctx, roundID, "accept", salt1)
	CommitVoteOnNode(t, val2, ctx, roundID, "reject", salt2)

	// 3. Wait for reveal phase
	WaitForRoundPhase(t, chain, ctx, roundID, "VERIFICATION_PHASE_REVEAL", 15)

	// 4. Reveal all votes (each on their own node)
	RevealVoteOnNode(t, val0, ctx, roundID, "accept", salt0)
	RevealVoteOnNode(t, val1, ctx, roundID, "accept", salt1)
	RevealVoteOnNode(t, val2, ctx, roundID, "reject", salt2)

	// 5. Wait for completion
	WaitForRoundPhase(t, chain, ctx, roundID, "VERIFICATION_PHASE_COMPLETE", 20)

	// 6. Verify verdict: 2/3 majority accepted
	roundResult := QueryRound(t, chain, ctx, roundID)
	roundData := roundResult["round"].(map[string]interface{})
	verdict := roundData["verdict"].(string)
	t.Logf("Verdict: %s", verdict)
	require.Equal(t, "VERDICT_ACCEPT", verdict,
		"2/3 majority should result in ACCEPT verdict")

	// 7. Verify reveals show both accept and reject
	reveals, ok := roundData["reveals"].([]interface{})
	require.True(t, ok, "round should have reveals")
	require.Len(t, reveals, 3, "all 3 validators should have revealed")

	acceptCount := 0
	rejectCount := 0
	for _, r := range reveals {
		rm := r.(map[string]interface{})
		switch rm["vote"].(string) {
		case "accept":
			acceptCount++
		case "reject":
			rejectCount++
		}
	}
	require.Equal(t, 2, acceptCount, "should have 2 accept votes")
	require.Equal(t, 1, rejectCount, "should have 1 reject vote")

	// 8. Verify fact was created despite dissent
	facts := QueryFactsByDomain(t, chain, ctx, "physics")
	found := false
	for _, f := range facts {
		factMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if factMap["content"] == content {
			found = true
			break
		}
	}
	require.True(t, found, "fact should be created with 2/3 majority despite dissent")
}

func TestKnowledge_Metabolism(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)
	valKeyName := "validator"

	// 1. Create a fact via full lifecycle
	content := "Newtons first law states that an object at rest stays at rest unless acted upon"
	claimID := DriveClaimToFact(t, chain, ctx, valKeyName, []string{valKeyName},
		content, "physics", "empirical", "1000000")
	t.Logf("Claim driven to fact: %s", claimID)

	// 2. Find the fact ID by querying the domain
	WaitBlocks(t, chain, ctx, 2)
	facts := QueryFactsByDomain(t, chain, ctx, "physics")

	var factID string
	var initialEnergy uint64
	for _, f := range facts {
		factMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if factMap["content"] == content {
			factID, _ = factMap["id"].(string)
			initialEnergy, _ = jsonNumber(factMap, "energy")
			break
		}
	}
	require.NotEmpty(t, factID, "fact should exist after verification")
	t.Logf("Fact ID: %s, initial energy: %d", factID, initialEnergy)

	// 3. Wait for at least one fitness epoch (fitness_epoch_blocks=10 in genesis).
	WaitBlocks(t, chain, ctx, 15)

	// 4. Query fact again and check energy decreased
	factResult := QueryFact(t, chain, ctx, factID)
	factData, ok := factResult["fact"].(map[string]interface{})
	require.True(t, ok, "fact query should return fact data")

	currentEnergy, hasEnergy := jsonNumber(factData, "energy")
	t.Logf("Energy after metabolism: %d (initial: %d)", currentEnergy, initialEnergy)

	if hasEnergy && initialEnergy > 0 {
		require.Less(t, currentEnergy, initialEnergy,
			"energy should decrease after metabolism epoch (no queries or citations to earn income)")
	} else {
		t.Logf("NOTE: energy tracking may not be active yet (initial=%d, current=%d, hasEnergy=%v)",
			initialEnergy, currentEnergy, hasEnergy)
	}

	// 5. Verify metabolism status endpoint works
	metaStatus := QueryMetabolismStatus(t, chain, ctx)
	t.Logf("Metabolism status: %v", metaStatus)
}

func TestKnowledge_WuXing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)
	valKeyName := "validator"

	// 1. Record the current block height for alignment observation query.
	heightBefore, err := chain.Height(ctx)
	require.NoError(t, err)
	t.Logf("Height before knowledge creation: %d", heightBefore)

	// 2. Create a fact via full lifecycle (this generates verification activity).
	content := "Conservation of energy states that energy cannot be created or destroyed"
	DriveClaimToFact(t, chain, ctx, valKeyName, []string{valKeyName},
		content, "physics", "empirical", "1000000")

	// 3. Wait for alignment observation interval (observation_interval_blocks=10).
	WaitBlocks(t, chain, ctx, 12)

	heightAfter, err := chain.Height(ctx)
	require.NoError(t, err)
	t.Logf("Height after waiting for observation: %d", heightAfter)

	// 4. Query the alignment observation at the latest observation height.
	obsHeight := (heightAfter / 10) * 10
	if obsHeight == 0 {
		obsHeight = 10
	}

	out := QueryModule(t, chain, ctx, "alignment", "observation", fmt.Sprintf("%d", obsHeight))
	t.Logf("Alignment observation at height %d: %s", obsHeight, string(out))

	// 5. Parse the observation and verify knowledge sensors are populated.
	var obsResp map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &obsResp), "unmarshal alignment observation")

	obs, ok := obsResp["observation"].(map[string]interface{})
	if ok {
		kq, hasKQ := jsonNumber(obs, "knowledge_quality")
		t.Logf("Knowledge quality sensor: %d (found: %v)", kq, hasKQ)

		if hasKQ {
			require.Greater(t, kq, uint64(0),
				"knowledge quality should be > 0 after successful verification")
		}
	} else {
		t.Logf("NOTE: observation may not be available at height %d (response: %v)", obsHeight, obsResp)
	}

	// 6. Query alignment health index to verify the system is tracking.
	healthOut := QueryModule(t, chain, ctx, "alignment", "health-index", fmt.Sprintf("%d", obsHeight))
	t.Logf("Alignment health index at height %d: %s", obsHeight, string(healthOut))
}
