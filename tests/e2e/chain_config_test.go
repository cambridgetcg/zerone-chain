package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"

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

// ZeroneGovChainSpec returns a chain spec configured for governance and
// emergency E2E tests. It injects fast ceremony timings and auto-populates
// the emergency genesis council with validator delegator addresses.
func ZeroneGovChainSpec(numValidators int) *interchaintest.ChainSpec {
	numFullNodes := 0
	return &interchaintest.ChainSpec{
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
			ModifyGenesis:  modifyGenesisWithCouncil(govGenesisKV()),
			Images: []ibc.DockerImage{{
				Repository: "zerone",
				Version:    "local",
				UIDGID:     "0:0",
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

// govGenesisKV extends testGenesisKV with governance-specific overrides
// for fast LIP lifecycle and emergency ceremony testing.
func govGenesisKV() []cosmos.GenesisKV {
	kvs := testGenesisKV()

	// ── Governance: low stakes and short review for fast LIP tests ──
	// Category indices: 0=parameter, 1=upgrade, 2=text, 3=research_spend,
	// 4=research_seat_election, 5=research_phase_transition, 6=research_phase_rollback
	for i := 0; i < 7; i++ {
		kvs = append(kvs,
			cosmos.NewGenesisKV(fmt.Sprintf("app_state.zerone_gov.params.category_configs.%d.review_blocks", i), 3),
			cosmos.NewGenesisKV(fmt.Sprintf("app_state.zerone_gov.params.category_configs.%d.required_stake_uzrn", i), "1000000"),
		)
	}

	kvs = append(kvs,
		// ── Emergency: fast ceremony timing ──
		cosmos.NewGenesisKV("app_state.emergency.params.halt_prevote_blocks", 5),
		cosmos.NewGenesisKV("app_state.emergency.params.halt_precommit_blocks", 5),
		cosmos.NewGenesisKV("app_state.emergency.params.halt_timeout_blocks", 30),
		cosmos.NewGenesisKV("app_state.emergency.params.resume_prevote_blocks", 5),
		cosmos.NewGenesisKV("app_state.emergency.params.resume_precommit_blocks", 5),
		cosmos.NewGenesisKV("app_state.emergency.params.resume_timeout_blocks", 30),
		cosmos.NewGenesisKV("app_state.emergency.params.min_distinct_voters", 1),
		cosmos.NewGenesisKV("app_state.emergency.params.min_guardian_stake", "0"),
		cosmos.NewGenesisKV("app_state.emergency.params.cooldown_blocks", 0),
		cosmos.NewGenesisKV("app_state.emergency.params.max_proposals_per_epoch", 100),
		cosmos.NewGenesisKV("app_state.emergency.params.max_proposals_per_guardian_per_epoch", 100),
		cosmos.NewGenesisKV("app_state.emergency.params.council_expiry_block", 1000000),
		cosmos.NewGenesisKV("app_state.emergency.params.max_halt_duration_blocks", 100),
	)

	return kvs
}

// modifyGenesisWithCouncil wraps the standard KV-based genesis modifier with
// an additional step that extracts validator delegator addresses from gentxs
// and sets them as the emergency genesis council.
func modifyGenesisWithCouncil(kvs []cosmos.GenesisKV) func(ibc.ChainConfig, []byte) ([]byte, error) {
	baseModify := cosmos.ModifyGenesis(kvs)
	return func(cfg ibc.ChainConfig, genbz []byte) ([]byte, error) {
		genbz, err := baseModify(cfg, genbz)
		if err != nil {
			return nil, err
		}

		// Extract validator delegator addresses from gentxs.
		council := extractDelegatorAddresses(genbz)
		if len(council) == 0 {
			return genbz, nil
		}

		// Parse the full genesis preserving number precision.
		var doc map[string]interface{}
		dec := json.NewDecoder(bytes.NewReader(genbz))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			return nil, fmt.Errorf("unmarshal genesis for council injection: %w", err)
		}

		// Navigate: app_state → emergency → params → genesis_council
		appState, _ := doc["app_state"].(map[string]interface{})
		if appState == nil {
			return genbz, nil
		}
		emergency, _ := appState["emergency"].(map[string]interface{})
		if emergency == nil {
			return genbz, nil
		}
		params, _ := emergency["params"].(map[string]interface{})
		if params == nil {
			return genbz, nil
		}
		params["genesis_council"] = council

		return json.Marshal(doc)
	}
}

// extractDelegatorAddresses parses genesis gentxs to find validator
// delegator (account) addresses for use as emergency council members.
func extractDelegatorAddresses(genbz []byte) []string {
	var doc struct {
		AppState struct {
			Genutil struct {
				GenTxs []json.RawMessage `json:"gen_txs"`
			} `json:"genutil"`
		} `json:"app_state"`
	}
	if err := json.Unmarshal(genbz, &doc); err != nil {
		return nil
	}

	var addrs []string
	for _, genTx := range doc.AppState.Genutil.GenTxs {
		var tx struct {
			Body struct {
				Messages []struct {
					DelegatorAddress string `json:"delegator_address"`
				} `json:"messages"`
			} `json:"body"`
		}
		if err := json.Unmarshal(genTx, &tx); err != nil {
			continue
		}
		for _, msg := range tx.Body.Messages {
			if msg.DelegatorAddress != "" {
				addrs = append(addrs, msg.DelegatorAddress)
			}
		}
	}
	return addrs
}
