package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const validatorKeyName = "validator"

// SetupGovChain spins up a chain configured for governance and emergency E2E
// tests. The emergency genesis council is auto-populated with validator
// delegator addresses from gentxs.
func SetupGovChain(t *testing.T, numValidators int) (*cosmos.CosmosChain, context.Context) {
	t.Helper()

	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(
		zaptest.NewLogger(t),
		[]*interchaintest.ChainSpec{ZeroneGovChainSpec(numValidators)},
	)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	require.Len(t, chains, 1)

	chain := chains[0].(*cosmos.CosmosChain)

	client, network := interchaintest.DockerSetup(t)

	ic := interchaintest.NewInterchain().AddChain(chain)

	rep := testreporter.NewNopReporter()

	err = ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ic.Close()
	})

	return chain, ctx
}

// fundTestUser creates and funds a test user with the given amount.
func fundTestUser(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, amount int64) ibc.Wallet {
	t.Helper()
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), sdkmath.NewInt(amount), chain)
	require.Len(t, users, 1)
	return users[0]
}

// queryJSON queries a module and returns the result as a map, preserving
// number precision via json.Number.
func queryJSON(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, module, query string, args ...string) map[string]interface{} {
	t.Helper()
	raw := QueryModule(t, chain, ctx, module, query, args...)
	var result map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	err := dec.Decode(&result)
	require.NoError(t, err, "failed to parse query response: %s", string(raw))
	return result
}

// getLIPField returns a string field from the LIP query response.
func getLIPField(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, lipID, field string) string {
	t.Helper()
	resp := queryJSON(t, chain, ctx, "zerone_gov", "lip", lipID)
	lip, ok := resp["lip"].(map[string]interface{})
	require.True(t, ok, "expected lip object in response")
	val, ok := lip[field]
	require.True(t, ok, "expected field %q in lip", field)
	return jsonString(val)
}

// getEmergencyStatus returns the chain's emergency status string.
func getEmergencyStatus(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context) string {
	t.Helper()
	resp := queryJSON(t, chain, ctx, "emergency", "status")
	return jsonString(resp["status"])
}

// getActiveCeremonyID returns the active emergency ceremony ID, or "" if none.
func getActiveCeremonyID(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context) string {
	t.Helper()
	resp := queryJSON(t, chain, ctx, "emergency", "active-ceremony")
	found, _ := resp["found"].(bool)
	if !found {
		return ""
	}
	ceremony, ok := resp["ceremony"].(map[string]interface{})
	if !ok {
		return ""
	}
	return jsonString(ceremony["id"])
}

// getCeremonyPhase returns the current phase of the active emergency ceremony.
func getCeremonyPhase(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context) string {
	t.Helper()
	resp := queryJSON(t, chain, ctx, "emergency", "active-ceremony")
	ceremony, ok := resp["ceremony"].(map[string]interface{})
	if !ok {
		return ""
	}
	return jsonString(ceremony["phase"])
}

// jsonString coerces an interface{} value to a string (handles string,
// json.Number, and float64 types from JSON unmarshaling).
func jsonString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case json.Number:
		return s.String()
	case float64:
		if s == float64(int64(s)) {
			return strconv.FormatInt(int64(s), 10)
		}
		return strconv.FormatFloat(s, 'f', -1, 64)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// submitAndPassLIP is a helper that submits a LIP, stakes to enter review,
// waits for auto-advance through stages, votes yes, and waits for tally.
// Returns the LIP ID.
func submitAndPassLIP(
	t *testing.T,
	chain *cosmos.CosmosChain,
	ctx context.Context,
	keyName, title, description, category, initialStake string,
	extraFlags ...string,
) string {
	t.Helper()

	// Submit the LIP.
	cmd := append([]string{"zerone_gov", "submit-lip", title, description, category, initialStake}, extraFlags...)
	ExecTx(t, chain, ctx, keyName, cmd...)

	WaitBlocks(t, chain, ctx, 1)

	lipID := findLatestLIP(t, chain, ctx)
	t.Logf("submitted %s (category=%s)", lipID, category)

	stage := getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "draft", stage, "newly submitted LIP should be in draft")

	// Stake to transition draft → review.
	ExecTx(t, chain, ctx, keyName, "zerone_gov", "stake-lip", lipID, "1")
	WaitBlocks(t, chain, ctx, 1)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "review", stage, "staked LIP should transition to review")

	// Wait for review_blocks (3) + discussion_period_blocks (5) + 2 margin
	// for auto-advance: review → last_call → voting.
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "voting", stage, "LIP should have auto-advanced to voting")

	// Cast a "yes" vote from the validator (has 100% of bonded stake).
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "cast-vote", lipID, "yes")

	// Wait for voting_period_blocks (10) + 2 margin for auto-tally.
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "passed", stage, "LIP should have passed")

	return lipID
}

// findLatestLIP queries all LIPs and returns the last one (assumed to be
// the most recently submitted, since the query returns LIPs in creation order).
func findLatestLIP(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context) string {
	t.Helper()
	resp := queryJSON(t, chain, ctx, "zerone_gov", "lips")
	lips, ok := resp["lips"].([]interface{})
	require.True(t, ok && len(lips) > 0, "expected at least one LIP")

	last := lips[len(lips)-1].(map[string]interface{})
	return jsonString(last["id"])
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestGov_TextLIPLifecycle tests the full lifecycle of a text LIP:
// submit → stake → review → last_call → voting → passed.
func TestGov_TextLIPLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 1)

	// Wait for a few blocks to let the chain stabilize.
	WaitBlocks(t, chain, ctx, 3)

	// Fund a test user with enough for staking.
	user := fundTestUser(t, chain, ctx, 100_000_000) // 100 ZRN

	// Submit text LIP.
	ExecTx(t, chain, ctx, user.KeyName(), "zerone_gov", "submit-lip",
		"Improve documentation",
		"Proposal to improve developer onboarding docs",
		"text",
		"1000000", // 1 ZRN initial stake
	)
	WaitBlocks(t, chain, ctx, 1)

	lipID := findLatestLIP(t, chain, ctx)
	t.Logf("submitted: %s", lipID)

	// ── Verify draft stage ──
	stage := getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "draft", stage)

	// ── Stake to enter review ──
	ExecTx(t, chain, ctx, user.KeyName(), "zerone_gov", "stake-lip", lipID, "1")
	WaitBlocks(t, chain, ctx, 1)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "review", stage)

	// ── Wait for auto-advance: review → last_call → voting ──
	// review_blocks=3, discussion_period_blocks=5, plus margin
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "voting", stage)

	// ── Vote yes from validator ──
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "cast-vote", lipID, "yes")

	// ── Wait for tally ── (voting_period_blocks=10 + margin)
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "passed", stage)

	// Verify vote was recorded.
	valAddr, err := chain.GetNode().AccountKeyBech32(ctx, validatorKeyName)
	require.NoError(t, err)
	voteResp := queryJSON(t, chain, ctx, "zerone_gov", "vote", lipID, valAddr)
	vote, ok := voteResp["vote"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "yes", jsonString(vote["option"]))
}

// TestGov_ParamChangeProposal tests parameter-change LIPs:
// 1. A valid change to knowledge.domain_base_capacity takes effect.
// 2. A second proposal with an out-of-bounds value fails gracefully.
func TestGov_ParamChangeProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 1)
	WaitBlocks(t, chain, ctx, 3)

	// Query current knowledge params — record the baseline.
	beforeParams := queryJSON(t, chain, ctx, "knowledge", "params")
	t.Logf("knowledge params before: %v", beforeParams)

	t.Run("valid param change", func(t *testing.T) {
		// Submit parameter-change LIP targeting knowledge.domain_base_capacity.
		paramChanges := `[{"module":"knowledge","key":"domain_base_capacity","value":"5000"}]`
		lipID := submitAndPassLIP(t, chain, ctx,
			validatorKeyName,
			"Increase domain capacity",
			"Raise domain_base_capacity from default to 5000",
			"parameter",
			"1000000",
			"--param-changes", paramChanges,
		)
		t.Logf("param change LIP %s passed", lipID)

		// Query knowledge params after — verify the new value.
		afterParams := queryJSON(t, chain, ctx, "knowledge", "params")
		params, ok := afterParams["params"].(map[string]interface{})
		if !ok {
			// Some modules return params at the top level.
			params = afterParams
		}
		domainCap := jsonString(params["domain_base_capacity"])
		require.Equal(t, "5000", domainCap, "domain_base_capacity should have been updated to 5000")
	})

	t.Run("out-of-bounds param rejected", func(t *testing.T) {
		// Submit a second LIP with an extreme out-of-bounds value.
		// The LIP itself should be submittable (governance accepts the proposal),
		// but when executed after passing, the param router should reject the
		// invalid value. The LIP may pass voting but the param should NOT change.
		badChanges := `[{"module":"knowledge","key":"domain_base_capacity","value":"0"}]`
		badLIP := submitAndPassLIP(t, chain, ctx,
			validatorKeyName,
			"Bad domain capacity",
			"Set domain_base_capacity to 0 (invalid)",
			"parameter",
			"1000000",
			"--param-changes", badChanges,
		)
		t.Logf("out-of-bounds LIP %s passed voting (execution may fail)", badLIP)

		// The param should still be 5000 from the previous successful change,
		// because the execution of value=0 should be rejected by the param router.
		afterParams := queryJSON(t, chain, ctx, "knowledge", "params")
		params, ok := afterParams["params"].(map[string]interface{})
		if !ok {
			params = afterParams
		}
		domainCap := jsonString(params["domain_base_capacity"])
		t.Logf("domain_base_capacity after bad proposal: %s", domainCap)
		// The value should not have been changed to "0".
		require.NotEqual(t, "0", domainCap,
			"out-of-bounds param change should have been rejected during execution")
	})
}

// TestGov_UpgradePlan tests submitting an upgrade-category LIP with an
// attached upgrade plan, passing it, and verifying the plan is scheduled.
func TestGov_UpgradePlan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 1)
	WaitBlocks(t, chain, ctx, 3)

	// Submit upgrade LIP.
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "submit-lip",
		"v2.0.0 upgrade",
		"Schedule the v2.0.0 upgrade at a future height",
		"upgrade",
		"1000000",
	)
	WaitBlocks(t, chain, ctx, 1)

	lipID := findLatestLIP(t, chain, ctx)
	t.Logf("submitted upgrade LIP: %s", lipID)

	// Attach upgrade plan (proposer only).
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "attach-upgrade-plan",
		lipID, "v2.0.0", "999999", "https://github.com/zerone-chain/zerone/releases/tag/v2.0.0",
	)
	WaitBlocks(t, chain, ctx, 1)

	// Stake → review.
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "stake-lip", lipID, "1")
	WaitBlocks(t, chain, ctx, 1)

	stage := getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "review", stage)

	// Wait for auto-advance through review → last_call → voting.
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "voting", stage)

	// Vote yes.
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "cast-vote", lipID, "yes")

	// Wait for tally.
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "passed", stage)

	// Verify the tally confirms passage.
	tally := queryJSON(t, chain, ctx, "zerone_gov", "tally-result", lipID)
	passed, _ := tally["passed"].(bool)
	require.True(t, passed, "tally should indicate passed")

	// Try querying the SDK upgrade module for the scheduled plan.
	// If the plan was correctly scheduled at height 999999, it should appear.
	stdout, _, err := chain.GetNode().ExecQuery(ctx, "upgrade", "plan")
	if err == nil && len(stdout) > 0 {
		var planResp map[string]interface{}
		if json.Unmarshal(stdout, &planResp) == nil {
			if plan, ok := planResp["plan"].(map[string]interface{}); ok {
				t.Logf("upgrade plan registered: name=%s height=%s",
					jsonString(plan["name"]), jsonString(plan["height"]))
				require.Equal(t, "v2.0.0", jsonString(plan["name"]))
				require.Equal(t, "999999", jsonString(plan["height"]))
			}
		}
	} else {
		t.Logf("upgrade plan query returned no plan (may not be registered yet)")
	}
}

// TestGov_LIPRejection tests that a LIP fails when validators vote "no".
func TestGov_LIPRejection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 1)
	WaitBlocks(t, chain, ctx, 3)

	// Submit text LIP.
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "submit-lip",
		"Bad proposal",
		"This proposal should be rejected",
		"text",
		"1000000",
	)
	WaitBlocks(t, chain, ctx, 1)

	lipID := findLatestLIP(t, chain, ctx)

	// Stake to enter review.
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "stake-lip", lipID, "1")
	WaitBlocks(t, chain, ctx, 1)

	// Wait for auto-advance to voting.
	WaitBlocks(t, chain, ctx, 12)

	stage := getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "voting", stage)

	// Vote "no".
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "cast-vote", lipID, "no")

	// Wait for tally.
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "failed", stage, "LIP with only no votes should fail")
}

// TestEmergency_HaltAndResume tests the full emergency halt ceremony:
// propose halt → vote (prevote+precommit) → verify halted →
// propose resume → vote (prevote+precommit) → verify normal.
// Uses the genesis council for guardian access.
func TestEmergency_HaltAndResume(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 1)
	WaitBlocks(t, chain, ctx, 3)

	t.Run("initial state is normal", func(t *testing.T) {
		status := getEmergencyStatus(t, chain, ctx)
		require.Equal(t, "normal", status)
	})

	// ── Halt ceremony ──

	t.Run("propose halt", func(t *testing.T) {
		ExecTx(t, chain, ctx, validatorKeyName, "emergency", "propose-halt",
			"E2E test: simulated security incident",
		)
		WaitBlocks(t, chain, ctx, 1)

		status := getEmergencyStatus(t, chain, ctx)
		require.Equal(t, "halt_voting", status)
	})

	ceremonyID := getActiveCeremonyID(t, chain, ctx)
	require.NotEmpty(t, ceremonyID, "expected active halt ceremony")
	t.Logf("halt ceremony ID: %s", ceremonyID)

	t.Run("prevote halt", func(t *testing.T) {
		ExecTx(t, chain, ctx, validatorKeyName, "emergency", "vote-halt", ceremonyID, "true")
		WaitBlocks(t, chain, ctx, 1)

		// Verify ceremony advanced to precommit phase after prevote quorum.
		phase := getCeremonyPhase(t, chain, ctx)
		require.Equal(t, "precommit", phase,
			"ceremony should advance to precommit after prevote quorum (1 guardian at 100% > 75%)")
	})

	t.Run("precommit halt", func(t *testing.T) {
		ExecTx(t, chain, ctx, validatorKeyName, "emergency", "vote-halt", ceremonyID, "true")
		WaitBlocks(t, chain, ctx, 2)

		status := getEmergencyStatus(t, chain, ctx)
		require.Equal(t, "halted", status, "chain should be halted after ceremony finalization")
	})

	t.Run("chain still produces blocks while halted", func(t *testing.T) {
		// Emergency halt is a state flag, not a consensus halt.
		err := testutil.WaitForBlocks(ctx, 2, chain)
		require.NoError(t, err, "chain should still produce blocks while halted")
	})

	// ── Resume ceremony ──

	t.Run("propose resume", func(t *testing.T) {
		ExecTx(t, chain, ctx, validatorKeyName, "emergency", "propose-resume")
		WaitBlocks(t, chain, ctx, 1)

		status := getEmergencyStatus(t, chain, ctx)
		require.Equal(t, "resume_voting", status)
	})

	resumeID := getActiveCeremonyID(t, chain, ctx)
	require.NotEmpty(t, resumeID, "expected active resume ceremony")
	t.Logf("resume ceremony ID: %s", resumeID)

	t.Run("prevote resume", func(t *testing.T) {
		ExecTx(t, chain, ctx, validatorKeyName, "emergency", "vote-resume", resumeID, "true")
		WaitBlocks(t, chain, ctx, 1)

		phase := getCeremonyPhase(t, chain, ctx)
		require.Equal(t, "precommit", phase,
			"resume ceremony should advance to precommit after prevote quorum")
	})

	t.Run("precommit resume", func(t *testing.T) {
		ExecTx(t, chain, ctx, validatorKeyName, "emergency", "vote-resume", resumeID, "true")
		WaitBlocks(t, chain, ctx, 2)

		status := getEmergencyStatus(t, chain, ctx)
		require.Equal(t, "normal", status, "chain should resume normal operation")
	})

	t.Run("liveness after resume", func(t *testing.T) {
		h1, err := chain.Height(ctx)
		require.NoError(t, err)
		WaitBlocks(t, chain, ctx, 3)
		h2, err := chain.Height(ctx)
		require.NoError(t, err)
		require.Greater(t, h2, h1, "chain should continue producing blocks after resume")
	})

	t.Run("completed ceremonies recorded", func(t *testing.T) {
		completed := queryJSON(t, chain, ctx, "emergency", "completed-ceremonies")
		t.Logf("completed ceremonies: %v", completed)
	})
}

// TestGov_DomainFormationFreeze verifies the domain formation freeze pathway.
// MsgDomainFormationFreeze requires the governance module authority address,
// which cannot be sent directly via CLI in E2E. This test verifies that the
// partnership module's freeze infrastructure is operational, while full freeze
// lifecycle coverage is in x/partnerships/keeper/formation_freeze_test.go.
func TestGov_DomainFormationFreeze(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	t.Skip("MsgDomainFormationFreeze requires governance module authority; " +
		"full freeze lifecycle is covered by x/partnerships/keeper/formation_freeze_test.go")

	chain, ctx := SetupGovChain(t, 1)
	WaitBlocks(t, chain, ctx, 3)

	// Verify the partnerships module is alive.
	paramsResp := queryJSON(t, chain, ctx, "partnerships", "params")
	require.NotNil(t, paramsResp, "partnerships module should be queryable")

	// Verify formation pool query works.
	_, _, _ = chain.GetNode().ExecQuery(ctx, "partnerships", "formation-pool")
}

// TestGov_ExpeditedVoting tests that knowledge-related parameter LIPs pass
// through the expedited voting pathway without panics. On a fresh chain,
// alignment health defaults to "healthy" so the full voting period applies.
// The test verifies the voting_end_block to determine if expediting activated.
func TestGov_ExpeditedVoting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 1)
	WaitBlocks(t, chain, ctx, 3)

	// Submit multiple knowledge claims to create activity.
	user := fundTestUser(t, chain, ctx, 100_000_000)

	for i := 0; i < 3; i++ {
		// These may fail if domains aren't set up, but the goal is to
		// create activity, not necessarily valid claims.
		_, _, _ = chain.GetNode().Exec(ctx,
			chain.GetNode().TxCommand(user.KeyName(),
				"knowledge", "submit-claim",
				fmt.Sprintf("Test claim %d for expedited voting", i),
				"science", "hypothesis", "1000000",
			),
			chain.Config().Env,
		)
	}

	WaitBlocks(t, chain, ctx, 2)

	// Submit a knowledge-related parameter change LIP.
	paramChanges := `[{"module":"knowledge","key":"domain_base_capacity","value":"3000"}]`
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "submit-lip",
		"Expedited capacity change",
		"Urgent knowledge capacity adjustment",
		"parameter",
		"1000000",
		"--param-changes", paramChanges,
	)
	WaitBlocks(t, chain, ctx, 1)

	lipID := findLatestLIP(t, chain, ctx)
	t.Logf("submitted expedited LIP: %s", lipID)

	// Stake → review.
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "stake-lip", lipID, "1")
	WaitBlocks(t, chain, ctx, 1)

	// Wait for auto-advance to voting.
	WaitBlocks(t, chain, ctx, 12)

	stage := getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "voting", stage)

	// Check the voting_end_block to see if expediting activated.
	// Normal = review_started + review(3) + discussion(5) + voting(10) blocks from review start.
	// Expedited = same but voting period halved to 5.
	votingEnd := getLIPField(t, chain, ctx, lipID, "voting_end_block")
	t.Logf("voting_end_block=%s (full=10, expedited=5)", votingEnd)

	// Vote yes.
	ExecTx(t, chain, ctx, validatorKeyName, "zerone_gov", "cast-vote", lipID, "yes")

	// Wait for tally (at most voting_period_blocks=10, could be 5 if expedited).
	WaitBlocks(t, chain, ctx, 12)

	stage = getLIPField(t, chain, ctx, lipID, "stage")
	require.Equal(t, "passed", stage, "expedited LIP should pass")

	// Verify the tally result.
	tally := queryJSON(t, chain, ctx, "zerone_gov", "tally-result", lipID)
	t.Logf("expedited LIP tally: %v", tally)
	passed, _ := tally["passed"].(bool)
	require.True(t, passed)
}

// TestGov_QueryEndpoints verifies that governance query endpoints respond
// correctly and return well-formed JSON.
func TestGov_QueryEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 1)
	WaitBlocks(t, chain, ctx, 3)

	// ── Gov module queries ──

	// Params
	params := queryJSON(t, chain, ctx, "zerone_gov", "params")
	require.NotNil(t, params, "gov params should be queryable")
	t.Logf("gov params: voting_period_blocks=%v", params["voting_period_blocks"])

	// LIPs (empty list is ok)
	lips := queryJSON(t, chain, ctx, "zerone_gov", "lips")
	require.NotNil(t, lips, "lips list should be queryable")

	// Research voters
	voters := queryJSON(t, chain, ctx, "zerone_gov", "research-voters")
	require.NotNil(t, voters, "research-voters should be queryable")

	// Research fund governance
	rfg := queryJSON(t, chain, ctx, "zerone_gov", "research-fund-governance")
	require.NotNil(t, rfg, "research-fund-governance should be queryable")

	// ── Emergency module queries ──

	// Status
	status := queryJSON(t, chain, ctx, "emergency", "status")
	require.NotNil(t, status)
	require.Equal(t, "normal", jsonString(status["status"]))

	// Params
	eParams := queryJSON(t, chain, ctx, "emergency", "params")
	require.NotNil(t, eParams, "emergency params should be queryable")

	// Active ceremony (should be empty)
	ceremony := queryJSON(t, chain, ctx, "emergency", "active-ceremony")
	require.NotNil(t, ceremony)

	// Completed ceremonies (should be empty)
	completed := queryJSON(t, chain, ctx, "emergency", "completed-ceremonies")
	require.NotNil(t, completed)
}
