package e2e_test

import (
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func ZeroneChainSpec(numValidators int) *interchaintest.ChainSpec {
	numFullNodes := 0
	return &interchaintest.ChainSpec{
		// No Name — zerone isn't in interchaintest's built-in chain catalog,
		// so we provide a complete ChainConfig instead.
		ChainName: "zerone",
		Version:   "local",
		ChainConfig: ibc.ChainConfig{
			Type:           "cosmos",
			Name:           "zerone",
			ChainID:        "zerone-test-1",
			Bin:            "zeroned",
			Bech32Prefix:   "zrn",
			Denom:          "uzrn",
			GasPrices:      "0.025uzrn",
			GasAdjustment:  1.5,
			TrustingPeriod: "112h",
			NoHostMount:    false,
			ModifyGenesis:  cosmos.ModifyGenesis(testGenesisKV()),
			Images: []ibc.DockerImage{{
				Repository: "zerone",
				Version:    "local",
				UIDGID:     "0:0", // Dockerfile runs as root
			}},
		},
		NumValidators: &numValidators,
		NumFullNodes:  &numFullNodes,
	}
}

// testGenesisKV returns genesis key-value overrides for E2E tests.
// These shorten time-based parameters so tests finish quickly.
func testGenesisKV() []cosmos.GenesisKV {
	return []cosmos.GenesisKV{
		// ── Governance: short periods for fast voting tests ──
		cosmos.NewGenesisKV("app_state.zerone_gov.params.voting_period_blocks", 10),
		cosmos.NewGenesisKV("app_state.zerone_gov.params.discussion_period_blocks", 5),

		// ── Knowledge: short verification lifecycle ──
		cosmos.NewGenesisKV("app_state.knowledge.params.commit_phase_blocks", 10),
		cosmos.NewGenesisKV("app_state.knowledge.params.reveal_phase_blocks", 10),
		cosmos.NewGenesisKV("app_state.knowledge.params.aggregation_blocks", 5),

		// ── Alignment: fast observation cycle ──
		cosmos.NewGenesisKV("app_state.alignment.params.observation_interval_blocks", 10),

		// ── Staking: fast unbonding for delegation tests ──
		cosmos.NewGenesisKV("app_state.zerone_staking.params.unbonding_period_blocks", 50),

		// ── Capture defense: shorter analysis interval ──
		cosmos.NewGenesisKV("app_state.capture_defense.params.risk_analysis_interval", 20),

		// ── Partnerships: shorter formation windows ──
		cosmos.NewGenesisKV("app_state.partnerships.params.formation_window_blocks", 20),
		cosmos.NewGenesisKV("app_state.partnerships.params.cooling_period_blocks", 10),

		// ── Vesting rewards: enable full rewards for E2E testing ──
		cosmos.NewGenesisKV("app_state.vesting_rewards.params.min_validators_for_full_reward", 2),
		cosmos.NewGenesisKV("app_state.vesting_rewards.params.empty_block_reward_rate", 10000),

		// ── Vote extensions: must be enabled from block 1 for PoT ──
		cosmos.NewGenesisKV("consensus.params.abci.vote_extensions_enable_height", "1"),
	}
}
