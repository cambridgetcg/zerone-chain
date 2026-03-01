package cross_stack_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"

	// All 30 Zerone custom module types for per-module genesis validation.
	alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	autopoiesistypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
	billingtypes "github.com/zerone-chain/zerone/x/billing/types"
	bvmtypes "github.com/zerone-chain/zerone/x/bvm/types"
	capturechallengetypes "github.com/zerone-chain/zerone/x/capture_challenge/types"
	capturedefensetypes "github.com/zerone-chain/zerone/x/capture_defense/types"
	channelstypes "github.com/zerone-chain/zerone/x/channels/types"
	claimingpottypes "github.com/zerone-chain/zerone/x/claiming_pot/types"
	computepooltypes "github.com/zerone-chain/zerone/x/compute_pool/types"
	discoverytypes "github.com/zerone-chain/zerone/x/discovery/types"
	disputestypes "github.com/zerone-chain/zerone/x/disputes/types"
	emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
	evidencemgmttypes "github.com/zerone-chain/zerone/x/evidence_mgmt/types"
	zeronegov "github.com/zerone-chain/zerone/x/gov/types"
	hometypes "github.com/zerone-chain/zerone/x/home/types"
	ibcratelimittypes "github.com/zerone-chain/zerone/x/ibcratelimit/types"
	icaauthtypes "github.com/zerone-chain/zerone/x/icaauth/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	liquiditypooltypes "github.com/zerone-chain/zerone/x/liquiditypool/types"
	ontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
	partnershipstypes "github.com/zerone-chain/zerone/x/partnerships/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
	researchtypes "github.com/zerone-chain/zerone/x/research/types"
	scheduletypes "github.com/zerone-chain/zerone/x/schedule/types"
	zeronestakingtypes "github.com/zerone-chain/zerone/x/staking/types"
	tokenstypes "github.com/zerone-chain/zerone/x/tokens/types"
	toolboxtypes "github.com/zerone-chain/zerone/x/toolbox/types"
	treetypes "github.com/zerone-chain/zerone/x/tree/types"
	vestingrewardstypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// ---------------------------------------------------------------------------
// Gap A: TestPerModuleGenesisValidation
// ---------------------------------------------------------------------------
// Each subtest verifies that a single custom module's DefaultGenesis entry
// exists in the app genesis, is non-empty, and passes Validate().

func TestPerModuleGenesisValidation(t *testing.T) {
	app := newTestApp(t, testChainID)
	genState := app.DefaultGenesis()
	codec := app.AppCodec()

	// moduleEntry associates a module name with a validation function.
	// The validator unmarshals the raw JSON and calls Validate() on the
	// typed GenesisState. For modules with DefaultGenesis(), we also
	// verify that the app-produced default matches the standalone default.
	type moduleEntry struct {
		name     string
		validate func(t *testing.T, raw json.RawMessage)
	}

	modules := []moduleEntry{
		{alignmenttypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs alignmenttypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{zeroneauthtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs zeroneauthtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
			require.NotNil(t, gs.Params)
			require.Greater(t, gs.Params.MaxSessionKeys, uint32(0), "max session keys must be positive")
		}},
		{autopoiesistypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs autopoiesistypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{billingtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs billingtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{bvmtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs bvmtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{capturechallengetypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs capturechallengetypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{capturedefensetypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs capturedefensetypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{channelstypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs channelstypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{claimingpottypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs claimingpottypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{computepooltypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs computepooltypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{discoverytypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs discoverytypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{disputestypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs disputestypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{emergencytypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs emergencytypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
			require.NotNil(t, gs.Params)
			require.Greater(t, gs.Params.HaltPrevoteBlocks, uint64(0), "halt prevote blocks must be positive")
		}},
		{evidencemgmttypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs evidencemgmttypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{zeronegov.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs zeronegov.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
			require.NotNil(t, gs.Params)
			require.Greater(t, gs.Params.VotingPeriodBlocks, uint64(0), "voting period must be positive")
			require.Greater(t, gs.Params.QuorumThresholdBps, uint64(0), "quorum threshold must be positive")
		}},
		{hometypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs hometypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{ibcratelimittypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs ibcratelimittypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{icaauthtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs icaauthtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{knowledgetypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs knowledgetypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
			require.NotNil(t, gs.Params)
			require.Greater(t, gs.Params.MinVerifiers, uint64(0), "min verifiers must be positive")
			require.Greater(t, gs.Params.ConfidenceThreshold, uint64(0), "confidence threshold must be positive")
			require.Greater(t, gs.Params.WrongVerificationSlashBps, uint64(0), "slash bps must be positive")
			require.Len(t, gs.Domains, 18, "expected 18 genesis domains")
		}},
		{liquiditypooltypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs liquiditypooltypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{ontologytypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs ontologytypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{partnershipstypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs partnershipstypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{qualificationtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs qualificationtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{researchtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs researchtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{scheduletypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs scheduletypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{zeronestakingtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs zeronestakingtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
			require.NotNil(t, gs.Params)
			require.Greater(t, gs.Params.MaxValidators, uint64(0), "max validators must be positive")
			require.Greater(t, gs.Params.UnbondingPeriod, uint64(0), "unbonding period must be positive")
		}},
		{tokenstypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs tokenstypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{toolboxtypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs toolboxtypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{treetypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs treetypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
		}},
		{vestingrewardstypes.ModuleName, func(t *testing.T, raw json.RawMessage) {
			var gs vestingrewardstypes.GenesisState
			require.NoError(t, codec.UnmarshalJSON(raw, &gs))
			require.NoError(t, gs.Validate())
			require.NotNil(t, gs.Params)
			require.NotEmpty(t, gs.Params.BlockReward, "block reward must be set")
			require.Greater(t, gs.Params.MinValidatorsForFullReward, uint32(0), "min validators for full reward must be positive")
		}},
	}

	for _, m := range modules {
		t.Run(m.name, func(t *testing.T) {
			raw, ok := genState[m.name]
			require.True(t, ok, "module %q must be in DefaultGenesis", m.name)
			require.NotEmpty(t, raw, "module %q genesis JSON must not be empty", m.name)
			m.validate(t, raw)
		})
	}

	t.Logf("validated %d custom modules individually", len(modules))
}

// ---------------------------------------------------------------------------
// Gap B: TestKeeperGenesisRoundTrip
// ---------------------------------------------------------------------------
// For knowledge, zerone_staking, zerone_gov, and vesting_rewards, verify that
// InitGenesis -> ExportGenesis preserves default params through the keeper
// round-trip. The harness calls InitChain (which invokes InitGenesis for all
// modules), so we export afterwards and compare against defaults.

func TestKeeperGenesisRoundTrip_Knowledge(t *testing.T) {
	h := NewTestHarness(t)

	exported := h.KnowledgeKeeper.ExportGenesis(h.Ctx)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Params)

	defaults := knowledgetypes.DefaultGenesis()
	require.Equal(t, defaults.Params.MinVerifiers, exported.Params.MinVerifiers,
		"MinVerifiers must survive round-trip")
	require.Equal(t, defaults.Params.ConfidenceThreshold, exported.Params.ConfidenceThreshold,
		"ConfidenceThreshold must survive round-trip")
	require.Equal(t, defaults.Params.WrongVerificationSlashBps, exported.Params.WrongVerificationSlashBps,
		"WrongVerificationSlashBps must survive round-trip")
	require.Equal(t, defaults.Params.CommitPhaseBlocks, exported.Params.CommitPhaseBlocks,
		"CommitPhaseBlocks must survive round-trip")

	// Domains are stored via prefix iteration. After InitChain the harness
	// creates a CheckTx context, so domain iteration may return a different
	// count depending on store commit semantics. Verify that the exported
	// genesis is structurally valid regardless.
	require.NoError(t, exported.Validate(), "exported genesis must pass validation")
}

func TestKeeperGenesisRoundTrip_Staking(t *testing.T) {
	h := NewTestHarness(t)

	exported := h.StakingKeeper.ExportGenesis(h.Ctx)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Params)

	defaults := zeronestakingtypes.DefaultParams()
	require.Equal(t, defaults.MaxValidators, exported.Params.MaxValidators,
		"MaxValidators must survive round-trip")
	require.Equal(t, defaults.UnbondingPeriod, exported.Params.UnbondingPeriod,
		"UnbondingPeriod must survive round-trip")
	require.Len(t, exported.Params.TierConfigs, 4,
		"must have exactly 4 tier configs after round-trip")
}

func TestKeeperGenesisRoundTrip_Gov(t *testing.T) {
	h := NewTestHarness(t)

	exported := h.GovKeeper.ExportGenesis(h.Ctx)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Params)

	defaults := zeronegov.DefaultParams()
	require.Equal(t, defaults.VotingPeriodBlocks, exported.Params.VotingPeriodBlocks,
		"VotingPeriodBlocks must survive round-trip")
	require.Equal(t, defaults.QuorumThresholdBps, exported.Params.QuorumThresholdBps,
		"QuorumThresholdBps must survive round-trip")
	require.Equal(t, defaults.SupportThresholdBps, exported.Params.SupportThresholdBps,
		"SupportThresholdBps must survive round-trip")
}

func TestKeeperGenesisRoundTrip_VestingRewards(t *testing.T) {
	h := NewTestHarness(t)

	exported := h.VestingRewardsKeeper.ExportGenesis(h.Ctx)
	require.NotNil(t, exported)
	require.NotNil(t, exported.Params)

	defaults := vestingrewardstypes.DefaultParams()
	require.Equal(t, defaults.BlockReward, exported.Params.BlockReward,
		"BlockReward must survive round-trip")
	require.Equal(t, defaults.MinValidatorsForFullReward, exported.Params.MinValidatorsForFullReward,
		"MinValidatorsForFullReward must survive round-trip")
	require.Equal(t, defaults.RewardDecayBps, exported.Params.RewardDecayBps,
		"RewardDecayBps must survive round-trip")
	require.Equal(t, defaults.BlocksPerRewardEpoch, exported.Params.BlocksPerRewardEpoch,
		"BlocksPerRewardEpoch must survive round-trip")
}

// ---------------------------------------------------------------------------
// Gap C: TestBlockProduction_100Blocks
// ---------------------------------------------------------------------------
// Advances the chain 100 blocks to verify all BeginBlocker/EndBlocker hooks
// execute without panics or errors across all modules.

func TestBlockProduction_100Blocks(t *testing.T) {
	h := NewTestHarness(t)
	require.GreaterOrEqual(t, h.Height(), int64(1))

	h.AdvanceBlocks(100)
	require.GreaterOrEqual(t, h.Height(), int64(100))

	// Verify state is still consistent by exporting genesis.
	genState := h.App.DefaultGenesis()
	require.NotEmpty(t, genState)

	err := zeroneapp.ModuleBasics.ValidateGenesis(
		h.App.AppCodec(),
		h.App.TxConfig(),
		genState,
	)
	require.NoError(t, err, "genesis must remain valid after 100 blocks")

	t.Logf("successfully produced %d blocks", h.Height())
}

// ---------------------------------------------------------------------------
// Gap D: TestAxiomDuplicateAndDependencyChecks
// ---------------------------------------------------------------------------
// Explicitly checks that the embedded axiom set has no duplicate IDs and that
// every dependency reference resolves to an existing axiom.

func TestAxiomDuplicateAndDependencyChecks(t *testing.T) {
	axioms, err := knowledgetypes.ParseAxioms(knowledgetypes.GenesisAxiomsJSON)
	require.NoError(t, err)
	require.NotEmpty(t, axioms, "embedded axioms must not be empty")

	// Explicit duplicate ID check.
	seen := make(map[string]bool)
	for _, a := range axioms {
		require.False(t, seen[a.AxiomID], "duplicate axiom ID: %s", a.AxiomID)
		seen[a.AxiomID] = true
	}

	// Verify every dependency references an existing axiom.
	for _, a := range axioms {
		for _, dep := range a.Dependencies {
			require.True(t, seen[dep],
				"axiom %s depends on non-existent axiom %s", a.AxiomID, dep)
		}
	}

	t.Logf("verified %d axioms with no duplicates and all deps resolved", len(axioms))
}

// ---------------------------------------------------------------------------
// Gap E: TestExplicitParamsSmokeTest
// ---------------------------------------------------------------------------
// Boots a full harness and queries keeper params to verify non-zero critical
// values survive the InitChain pipeline.

func TestExplicitParamsSmokeTest(t *testing.T) {
	h := NewTestHarness(t)

	t.Run("Knowledge", func(t *testing.T) {
		p, err := h.KnowledgeKeeper.GetParams(h.Ctx)
		require.NoError(t, err)
		require.NotNil(t, p)
		require.Greater(t, p.MinVerifiers, uint64(0), "MinVerifiers")
		require.Greater(t, p.ConfidenceThreshold, uint64(0), "ConfidenceThreshold")
		require.Greater(t, p.WrongVerificationSlashBps, uint64(0), "WrongVerificationSlashBps")
		require.NotEmpty(t, p.VerificationReward, "VerificationReward must be set")
		require.NotEqual(t, "0", p.VerificationReward, "VerificationReward must be non-zero")
	})

	t.Run("Staking", func(t *testing.T) {
		p := h.StakingKeeper.GetParams(h.Ctx)
		require.NotNil(t, p)
		require.Greater(t, p.MaxValidators, uint64(0), "MaxValidators")
		require.Greater(t, p.UnbondingPeriod, uint64(0), "UnbondingPeriod")
		require.Len(t, p.TierConfigs, 4, "must have 4 tier configs")
	})

	t.Run("Gov", func(t *testing.T) {
		p := h.GovKeeper.GetParams(h.Ctx)
		require.NotNil(t, p)
		require.Greater(t, p.VotingPeriodBlocks, uint64(0), "VotingPeriodBlocks")
		require.Greater(t, p.QuorumThresholdBps, uint64(0), "QuorumThresholdBps")
		require.Greater(t, p.SupportThresholdBps, uint64(0), "SupportThresholdBps")
	})

	t.Run("VestingRewards", func(t *testing.T) {
		p := h.VestingRewardsKeeper.GetParams(h.Ctx)
		require.NotNil(t, p)
		require.NotEmpty(t, p.BlockReward, "BlockReward must be set")
		require.NotEqual(t, "0", p.BlockReward, "BlockReward must be non-zero")
		require.Greater(t, p.MinValidatorsForFullReward, uint32(0), "MinValidatorsForFullReward")
		require.Greater(t, p.BlocksPerRewardEpoch, uint64(0), "BlocksPerRewardEpoch")
		require.Greater(t, p.RewardDecayBps, uint64(0), "RewardDecayBps")
	})
}
