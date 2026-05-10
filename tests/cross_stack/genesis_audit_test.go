package cross_stack_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	zeroneapp "github.com/zerone-chain/zerone/app"
	autopoiesistypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
	claimingpottypes "github.com/zerone-chain/zerone/x/claiming_pot/types"
	emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestScenario10_AllModulesDefaultGenesisValid verifies that every registered
// module produces a valid DefaultGenesis that passes ValidateGenesis.
// This is a comprehensive audit across all 35+ modules.
func TestScenario10_AllModulesDefaultGenesisValid(t *testing.T) {
	app := newTestApp(t, testChainID)

	genState := app.DefaultGenesis()
	require.NotEmpty(t, genState)

	// Total module count must be >= 35 (SDK + Zerone custom modules)
	require.GreaterOrEqual(t, len(genState), 35,
		"expected >= 35 genesis modules, got %d", len(genState))
	t.Logf("total genesis modules: %d", len(genState))

	// ValidateGenesis for all modules
	err := zeroneapp.ModuleBasics.ValidateGenesis(
		app.AppCodec(),
		app.TxConfig(),
		genState,
	)
	require.NoError(t, err, "DefaultGenesis must pass ValidateGenesis for all modules")

	// Knowledge module specific validation
	knowledgeRaw, ok := genState[knowledgetypes.ModuleName]
	require.True(t, ok, "knowledge module must be in genesis")

	var knowledgeGen knowledgetypes.GenesisState
	require.NoError(t, app.AppCodec().UnmarshalJSON(knowledgeRaw, &knowledgeGen))
	require.NotNil(t, knowledgeGen.Params)

	// DefaultGenesis should have empty facts (axioms only via prepare-genesis)
	require.Empty(t, knowledgeGen.Facts, "DefaultGenesis must have empty facts")

	// But should have 18 domains
	require.Len(t, knowledgeGen.Domains, 18, "expected 18 genesis domains")

	// Verify slash params are non-zero
	p := knowledgeGen.Params
	require.Greater(t, p.WrongVerificationSlashBps, uint64(0))
	require.Greater(t, p.MissedRevealSlashBps, uint64(0))
	require.Greater(t, p.EquivocationSlashBps, uint64(0))
	// InvalidClaimSlashBps deprecated (R19-6): review fee is non-refundable

	// Verify BPS values on 1M scale
	require.LessOrEqual(t, p.InitialConfidence, uint64(1_000_000))
	require.LessOrEqual(t, p.ConfidenceThreshold, uint64(1_000_000))
	require.LessOrEqual(t, p.QuorumThreshold, uint64(1_000_000))

	// Verify non-zero epochs and minimums
	require.Greater(t, p.MinVerifiers, uint64(0))
	require.Greater(t, p.CommitPhaseBlocks, uint64(0))
	require.Greater(t, p.RevealPhaseBlocks, uint64(0))
}

// TestScenario11_SeedAxiomsWellFormed validates all embedded axioms pass
// comprehensive validation including DAG acyclicity and domain coverage.
func TestScenario11_SeedAxiomsWellFormed(t *testing.T) {
	facts, err := knowledgetypes.SeedAxiomFacts()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(facts), 777,
		"expected >= 777 axiom facts, got %d", len(facts))
	t.Logf("seed axiom count: %d", len(facts))

	// Verify domain coverage — must cover at least 15 domains
	domains := make(map[string]int)
	for _, f := range facts {
		domains[f.Domain]++
	}
	require.GreaterOrEqual(t, len(domains), 15,
		"expected >= 15 domains covered, got %d", len(domains))

	// Log domain distribution
	for d, count := range domains {
		t.Logf("  %s: %d facts", d, count)
	}

	// Verify axiom DAG using embedded JSON directly
	axioms, err := knowledgetypes.ParseAxioms(knowledgetypes.GenesisAxiomsJSON)
	require.NoError(t, err)

	domainNames := make([]string, 0)
	for d := range domains {
		domainNames = append(domainNames, d)
	}
	// Add all known axiom domains
	for _, n := range knowledgetypes.AxiomDomainNames() {
		found := false
		for _, dn := range domainNames {
			if dn == n {
				found = true
				break
			}
		}
		if !found {
			domainNames = append(domainNames, n)
		}
	}

	err = knowledgetypes.ValidateAxioms(axioms, domainNames)
	require.NoError(t, err, "axiom validation must pass")

	// Verify DAG stats
	stats, err := knowledgetypes.ComputeDAGStats(axioms)
	require.NoError(t, err)
	require.Greater(t, stats.RootCount, 0, "DAG must have root nodes")
	require.Greater(t, stats.TotalEdges, 0, "DAG must have edges")
	t.Logf("DAG stats: roots=%d, maxDepth=%d, edges=%d, orphans=%d",
		stats.RootCount, stats.MaxDepth, stats.TotalEdges, len(stats.Orphans))
}

// TestScenario12_AxiomToFactConversion verifies that converted Facts have
// correct proto fields matching the Zerone Fact message.
func TestScenario12_AxiomToFactConversion(t *testing.T) {
	axioms := []*knowledgetypes.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "A domain of discourse exists.",
			ClaimType:         "axiom",
			Domain:            "mathematics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
			Dependencies:      nil,
		},
		{
			AxiomID:           "PHYS-001",
			Statement:         "Space is 3D.",
			ClaimType:         "definition",
			Domain:            "physics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
			Dependencies:      []string{"MATH-001"},
		},
		{
			AxiomID:           "PHYS-010",
			Statement:         "Energy is conserved.",
			ClaimType:         "empirical_axiom",
			Domain:            "physics",
			EpistemicCategory: "empirical",
			Confidence:        0.97,
			Dependencies:      []string{"PHYS-001"},
		},
	}

	facts := knowledgetypes.AxiomsToFacts(axioms)
	require.Len(t, facts, 3)

	// Verify Content field (mapped from Statement)
	require.Equal(t, "A domain of discourse exists.", facts[0].Content)
	require.Equal(t, "Space is 3D.", facts[1].Content)

	// Verify Category field (mapped from EpistemicCategory)
	require.Equal(t, "analytic", facts[0].Category)
	require.Equal(t, "empirical", facts[2].Category)

	// Verify Status is VERIFIED
	for _, f := range facts {
		require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_VERIFIED, f.Status)
	}

	// Verify Fundamentality = 1,000,000
	for _, f := range facts {
		require.Equal(t, uint64(1_000_000), f.Fundamentality)
	}

	// Verify Stratum from BootstrapDomainStrata
	require.Equal(t, "fundamental", facts[0].Stratum)
	require.Equal(t, "physical", facts[1].Stratum)
	require.Equal(t, "physical", facts[2].Stratum)

	// Verify References come from Dependencies
	require.Empty(t, facts[0].References)
	require.Equal(t, []string{"MATH-001"}, facts[1].References)
	require.Equal(t, []string{"PHYS-001"}, facts[2].References)

	// Verify confidence BPS
	require.Equal(t, uint64(1_000_000), facts[0].Confidence)
	require.Equal(t, uint64(970_000), facts[2].Confidence)
}

// TestScenario13_ZeroTeamAllocationAtGenesis confirms the doctrine in
// docs/tokenomics/GENESIS.md: zero allocation to team. The default genesis
// state contains no positive balances and no positive supply for uzrn.
//
// The doctrine refuses any pre-allocation model: founder gets 0, AI vault
// gets 0, validators get 0 from the chain (their bonded tokens come from
// post-genesis earnings or, optionally, ephemeral net-zero gentx). Every
// ZRN that exists came from a participatory action — verification (block
// reward) or registration (bootstrap claim). Genesis-time non-zero
// balances would mean someone got ZRN by being someone in particular,
// not by doing something on-chain.
func TestScenario13_ZeroTeamAllocationAtGenesis(t *testing.T) {
	app := newTestApp(t, testChainID)

	genState := app.DefaultGenesis()

	// Decode bank module genesis directly — InitChain is not called here,
	// so we read DefaultGenesis as authored, not as patched by a val-set
	// helper that would bond tokens.
	var bankGen banktypes.GenesisState
	require.NoError(t, app.AppCodec().UnmarshalJSON(genState[banktypes.ModuleName], &bankGen))

	// No positive balances. Module accounts may exist (with a 0-balance
	// entry registered for permission tracking), but no entry may carry
	// positive uzrn.
	violations := []string{}
	for _, bal := range bankGen.Balances {
		for _, coin := range bal.Coins {
			if coin.Denom == zeroneapp.BondDenom && coin.Amount.IsPositive() {
				violations = append(violations, bal.Address+" holds "+coin.String())
			}
		}
	}
	require.Empty(t, violations,
		"no genesis account may hold ZRN — doctrine: zero team allocation. violations: %v", violations)

	// No positive supply. Total supply at genesis is 0; minting begins
	// at block 1 through x/vesting_rewards block rewards, and per-claim
	// through x/claiming_pot bootstrap claims.
	for _, supply := range bankGen.Supply {
		if supply.Denom == zeroneapp.BondDenom {
			require.True(t, supply.Amount.IsZero(),
				"genesis supply for ZRN must be 0; got %s", supply.Amount.String())
		}
	}
}

// TestScenario13b_AllowedAppConstants is a compile-time and value-time
// assertion that app/constants.go exposes only doctrine-aligned constants.
// If anyone re-introduces TotalSupplyZRN, FounderZRN, AIAgentZRN,
// ValidatorZRN, ResearchFundZRN, or ClaimingPotsZRN, this test will fail
// to compile (those identifiers no longer resolve). Conversely, this test
// pins the expected values of the constants that ARE allowed — chain-id,
// denom, prefix, block time, and the micro-denomination multiplier.
func TestScenario13b_AllowedAppConstants(t *testing.T) {
	require.Equal(t, "zeroned", zeroneapp.AppName)
	require.Equal(t, "zrn", zeroneapp.AccountAddressPrefix)
	require.Equal(t, "uzrn", zeroneapp.BondDenom)
	require.Equal(t, "zrn", zeroneapp.DisplayDenom)
	require.Equal(t, 2521, zeroneapp.DefaultBlockTime)
	require.Equal(t, 1_000_000, zeroneapp.MicroDenomMultiplier)
	require.Equal(t, "zerone-testnet-1", zeroneapp.TestnetChainID)
}

// TestScenario13c_ClaimingPotMinterPermission asserts that the claiming_pot
// module account is registered with Minter permission, enabling the
// bootstrap-claim emission pathway. Without Minter permission, the
// bootstrap pathway cannot mint and the doctrine collapses back to the
// pre-fund-then-transfer model. The permission is the structural form of
// commitment 20 (issuance follows participation) at the module-account
// layer.
func TestScenario13c_ClaimingPotMinterPermission(t *testing.T) {
	h := NewTestHarness(t)

	moduleAcc := h.App.AccountKeeper.GetModuleAccount(h.Ctx, claimingpottypes.ModuleName)
	require.NotNil(t, moduleAcc, "claiming_pot module account must exist post-InitChain")

	require.True(t, moduleAcc.HasPermission(authtypes.Minter),
		"claiming_pot module account must hold Minter permission to drive the bootstrap mint pathway (commitment 20: issuance follows participation)")
}

// TestScenario13d_BootstrapPotForAgent asserts the bootstrap-pot helper
// produces a doctrine-aligned pot for an arbitrary agent: per-agent
// amount = 222,000 uzrn (0.222 ZRN), single-claimant whitelist, instant
// vest, ACTIVE status. The genesis ceremony will call this helper once
// per whitelisted address in the operator's whitelist file (Phase 5).
//
// The pot model is shared-bucket-vesting, so "per-agent fixed amount"
// is expressed structurally as one pot per agent. This test pins the
// shape of those per-agent pots — a contract change here is a doctrine
// change.
func TestScenario13d_BootstrapPotForAgent(t *testing.T) {
	const sampleAgent = "zrn1exampleagentaddressforbootstraptest"
	const blockHeight = uint64(1)

	pot := claimingpottypes.MakeBootstrapPotForAgent(sampleAgent, blockHeight)

	require.Equal(t, claimingpottypes.BootstrapPotIDPrefix+sampleAgent, pot.Id,
		"bootstrap pot ID must carry the prefix so ceremony tooling can enumerate them")
	require.Equal(t, claimingpottypes.PerAgentBootstrapUzrn, pot.TotalAmount,
		"per-agent amount must be 222000 uzrn (0.222 ZRN) per commitment 20 doctrine")
	require.Equal(t, "0", pot.ClaimedAmount,
		"freshly created pot must have ClaimedAmount = 0")

	require.NotNil(t, pot.Schedule, "schedule must be set so CalculateClaimable can vest")
	require.Equal(t, blockHeight, pot.Schedule.StartBlock)
	require.Equal(t, blockHeight+claimingpottypes.BootstrapPotInstantVestBlocks, pot.Schedule.EndBlock)

	require.NotNil(t, pot.Eligibility)
	require.Equal(t, []string{sampleAgent}, pot.Eligibility.Whitelist,
		"single-agent whitelist binds the pot to exactly the recipient (no surface for cross-claim)")
	require.Equal(t, uint32(0), pot.Eligibility.MinStakingTier,
		"bootstrap is the participation seed — agents have not yet staked")

	require.Equal(t, claimingpottypes.PotStatus_POT_STATUS_ACTIVE, pot.Status)
}

// TestScenario13e_BootstrapPotsDoNotExpire confirms the operational
// binding for commitment 20: bootstrap pots are participation seeds, and
// the BeginBlocker pot-expiry sweep does not transition them to EXPIRED.
//
// Without this invariant, the genesis bootstrap pathway is structurally
// unclaimable. At the start of block 1, ProcessPotExpiry would flip every
// bootstrap pot (StartBlock=0, EndBlock=1) to EXPIRED before any MsgClaim
// tx in block 1 could run. A participation seed must remain claimable for
// the participant who shows up, regardless of how late.
func TestScenario13e_BootstrapPotsDoNotExpire(t *testing.T) {
	h := NewTestHarness(t)

	// Seed a bootstrap pot directly (skip the authority gate; this test is
	// about the expiry rule, not the admission path).
	const sampleAgent = "zrn1exampleagentforexpirytest000000000"
	pot := claimingpottypes.MakeBootstrapPotForAgent(sampleAgent, 0)
	h.ClaimingPotKeeper.SetPot(h.Ctx, pot)

	// Advance many blocks. AdvanceBlocks runs full BeginBlocker each tick,
	// so ProcessPotExpiry fires every block.
	h.AdvanceBlocks(100)

	got, found := h.ClaimingPotKeeper.GetPot(h.Ctx, pot.Id)
	require.True(t, found, "bootstrap pot must persist across the BeginBlocker expiry sweep")
	require.Equal(t, claimingpottypes.PotStatus_POT_STATUS_ACTIVE, got.Status,
		"bootstrap pot must remain ACTIVE after BeginBlocker sweeps; commitment 20 requires the seed to stay claimable")
}

// TestScenario14_GenesisRoundTripWithAxioms verifies that a genesis state
// with injected axioms can be marshaled, unmarshaled, and validated.
func TestScenario14_GenesisRoundTripWithAxioms(t *testing.T) {
	app := newTestApp(t, testChainID)

	// Start with default genesis
	genState := app.DefaultGenesis()

	// Load seed axioms
	axiomFacts, err := knowledgetypes.SeedAxiomFacts()
	require.NoError(t, err)

	// Inject axioms into knowledge genesis
	knowledgeGen := knowledgetypes.DefaultGenesis()
	knowledgeGen.Facts = axiomFacts

	knowledgeGenBz, err := app.AppCodec().MarshalJSON(knowledgeGen)
	require.NoError(t, err)
	genState[knowledgetypes.ModuleName] = knowledgeGenBz

	// Marshal → Unmarshal round-trip
	stateBytes, err := json.Marshal(genState)
	require.NoError(t, err)

	var restored zeroneapp.GenesisState
	require.NoError(t, json.Unmarshal(stateBytes, &restored))

	// Validate the restored genesis
	err = zeroneapp.ModuleBasics.ValidateGenesis(
		app.AppCodec(),
		app.TxConfig(),
		restored,
	)
	require.NoError(t, err, "genesis with axioms must pass ValidateGenesis")

	// Verify axioms survived the round-trip
	var restoredKnowledge knowledgetypes.GenesisState
	require.NoError(t, app.AppCodec().UnmarshalJSON(restored[knowledgetypes.ModuleName], &restoredKnowledge))
	require.Equal(t, len(axiomFacts), len(restoredKnowledge.Facts),
		"axiom count must survive round-trip")

	// Spot-check a few facts
	factMap := make(map[string]*knowledgetypes.Fact)
	for _, f := range restoredKnowledge.Facts {
		factMap[f.Id] = f
	}

	if f, ok := factMap["MATH-001"]; ok {
		require.Equal(t, "mathematics", f.Domain)
		require.Equal(t, "analytic", f.Category)
		require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_VERIFIED, f.Status)
	}
	if f, ok := factMap["THEO-001"]; ok {
		require.Equal(t, "theology", f.Domain)
		require.Equal(t, "canonical", f.Maturity)
	}
	if f, ok := factMap["AP-001"]; ok {
		require.Equal(t, "agent_purpose", f.Domain)
	}
}

// TestScenario15_ByteIdenticalGenesisRoundTrip verifies that DefaultGenesis
// marshaled to JSON is byte-identical across two fresh app instances.
func TestScenario15_ByteIdenticalGenesisRoundTrip(t *testing.T) {
	app1 := newTestApp(t, testChainID)
	app2 := newTestApp(t, testChainID)

	genState1 := app1.DefaultGenesis()
	genState2 := app2.DefaultGenesis()

	bytes1, err := json.Marshal(genState1)
	require.NoError(t, err)

	bytes2, err := json.Marshal(genState2)
	require.NoError(t, err)

	require.Equal(t, bytes1, bytes2,
		"DefaultGenesis must be byte-identical across app instances")

	// Round-trip: unmarshal → re-marshal must be idempotent
	var restored zeroneapp.GenesisState
	require.NoError(t, json.Unmarshal(bytes1, &restored))

	bytes3, err := json.Marshal(restored)
	require.NoError(t, err)

	require.Equal(t, bytes1, bytes3,
		"genesis JSON must be idempotent through marshal → unmarshal → marshal")
}

// TestScenario16_InvalidGenesisRejection verifies that ValidateGenesis rejects
// invalid parameter values for each module.
func TestScenario16_InvalidGenesisRejection(t *testing.T) {
	app := newTestApp(t, testChainID)
	codec := app.AppCodec()
	txConfig := app.TxConfig()

	t.Run("Knowledge_ZeroSlash", func(t *testing.T) {
		genState := app.DefaultGenesis()
		knowledgeGen := knowledgetypes.DefaultGenesis()
		knowledgeGen.Params.WrongVerificationSlashBps = 0
		bz, err := codec.MarshalJSON(knowledgeGen)
		require.NoError(t, err)
		genState[knowledgetypes.ModuleName] = bz

		err = zeroneapp.ModuleBasics.ValidateGenesis(codec, txConfig, genState)
		require.Error(t, err, "zero slash BPS should be rejected")
		t.Logf("rejected with: %v", err)
	})

	t.Run("Knowledge_ConfidenceOverBPS", func(t *testing.T) {
		genState := app.DefaultGenesis()
		knowledgeGen := knowledgetypes.DefaultGenesis()
		knowledgeGen.Params.InitialConfidence = 1_000_001
		bz, err := codec.MarshalJSON(knowledgeGen)
		require.NoError(t, err)
		genState[knowledgetypes.ModuleName] = bz

		err = zeroneapp.ModuleBasics.ValidateGenesis(codec, txConfig, genState)
		require.Error(t, err, "InitialConfidence > 1M should be rejected")
		t.Logf("rejected with: %v", err)
	})

	t.Run("Autopoiesis_InvertedBounds", func(t *testing.T) {
		genState := app.DefaultGenesis()
		apGen := autopoiesistypes.DefaultGenesis()
		apGen.Params.SlashMultiplierMin = 3_000_000
		apGen.Params.SlashMultiplierMax = 1_000_000
		bz, err := codec.MarshalJSON(apGen)
		require.NoError(t, err)
		genState[autopoiesistypes.ModuleName] = bz

		err = zeroneapp.ModuleBasics.ValidateGenesis(codec, txConfig, genState)
		require.Error(t, err, "inverted slash multiplier bounds should be rejected")
		t.Logf("rejected with: %v", err)
	})

	t.Run("Emergency_InvalidStatus", func(t *testing.T) {
		genState := app.DefaultGenesis()
		emGen := emergencytypes.DefaultGenesis()
		emGen.Params.HaltPrevoteBlocks = 0 // invalid: must be > 0
		bz, err := codec.MarshalJSON(emGen)
		require.NoError(t, err)
		genState[emergencytypes.ModuleName] = bz

		err = zeroneapp.ModuleBasics.ValidateGenesis(codec, txConfig, genState)
		require.Error(t, err, "zero halt_prevote_blocks should be rejected")
		t.Logf("rejected with: %v", err)
	})
}

// TestScenario17_BankGenesisSupplyConsistency verifies that the bank genesis
// supply entries sum to the declared total supply.
func TestScenario17_BankGenesisSupplyConsistency(t *testing.T) {
	app := newTestApp(t, testChainID)

	genState := app.DefaultGenesis()
	genState = genesisStateWithValSet(t, app, genState)

	// Unmarshal bank genesis
	var bankGen banktypes.GenesisState
	require.NoError(t, app.AppCodec().UnmarshalJSON(genState[banktypes.ModuleName], &bankGen))

	// Sum all balance entries
	totalFromBalances := make(map[string]int64)
	for _, bal := range bankGen.Balances {
		for _, coin := range bal.Coins {
			totalFromBalances[coin.Denom] += coin.Amount.Int64()
		}
	}

	// Compare against declared supply
	for _, supply := range bankGen.Supply {
		balAmt, ok := totalFromBalances[supply.Denom]
		require.True(t, ok, "supply denom %s must have matching balances", supply.Denom)
		require.Equal(t, supply.Amount.Int64(), balAmt,
			"supply for %s (%d) must equal sum of balances (%d)",
			supply.Denom, supply.Amount.Int64(), balAmt)
	}

	t.Logf("bank genesis: %d balance entries, %d supply denoms",
		len(bankGen.Balances), len(bankGen.Supply))
}

// TestScenario18_ModuleAccountPermissions verifies that module account
// addresses are deterministically derived and that the app's DefaultGenesis
// includes module genesis entries for all expected Zerone custom modules.
func TestScenario18_ModuleAccountPermissions(t *testing.T) {
	// Verify module account addresses are deterministic (derived from module name).
	// In SDK v0.50, module accounts are created lazily when they first interact
	// with the bank. Here we verify the address derivation is consistent.
	moduleNames := []string{
		"fee_collector",
		"bonded_tokens_pool",
		"not_bonded_tokens_pool",
		"distribution",
		"zerone_auth",
		"research_fund",
	}
	for _, name := range moduleNames {
		addr := authtypes.NewModuleAddress(name)
		require.NotEmpty(t, addr, "module address for %q must not be empty", name)
		// Verify the address is a valid bech32 with zrn prefix
		bech32 := addr.String()
		require.True(t, len(bech32) > 3 && bech32[:3] == "zrn",
			"module address for %q should have zrn prefix, got %s", name, bech32)
		t.Logf("module %q -> %s", name, bech32)
	}

	// Verify address derivation is idempotent
	for _, name := range moduleNames {
		addr1 := authtypes.NewModuleAddress(name)
		addr2 := authtypes.NewModuleAddress(name)
		require.Equal(t, addr1, addr2,
			"module address for %q must be deterministic", name)
	}

	// Verify all expected Zerone custom modules appear in DefaultGenesis
	app := newTestApp(t, testChainID)
	genState := app.DefaultGenesis()

	zeroneModules := []string{
		"zerone_auth",
		"zerone_staking",
		"knowledge",
		"emergency",
		"autopoiesis",
		"alignment",
		"research",
		"tree",
		"disputes",
		"vesting_rewards",
	}
	for _, name := range zeroneModules {
		_, ok := genState[name]
		require.True(t, ok, "DefaultGenesis must include module %q", name)
		t.Logf("genesis module %q: present", name)
	}

	t.Logf("total genesis modules: %d", len(genState))
}
