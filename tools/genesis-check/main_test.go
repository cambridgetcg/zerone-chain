package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// makeGenesis builds a minimal valid genesis from overrides.
// Overrides are applied via dig-style paths using setPath.
func makeGenesis(overrides map[string]interface{}) map[string]interface{} {
	base := `{
		"chain_id": "zerone-testnet-1",
		"genesis_time": "2026-03-01T00:00:00Z",
		"consensus": {
			"params": {
				"abci": {"vote_extensions_enable_height": "1"}
			}
		},
		"app_state": {
			"vesting_rewards": {
				"params": {
					"founder_share_bps": "0",
					"founder_address": "",
					"revenue_split": {
						"contributor_bps": "550000",
						"protocol_bps": "220000",
						"research_bps": "33300",
						"development_bps": "196700"
					},
					"protocol_sub_split": {
						"citation_bps": "500000",
						"verification_bps": "300000",
						"treasury_bps": "200000"
					}
				}
			},
			"knowledge": {
				"params": {
					"fitness_epoch_blocks": "100",
					"fitness_weight_query_bps": "150000",
					"fitness_weight_citation_bps": "250000",
					"fitness_weight_bridge_bps": "100000",
					"fitness_weight_depth_bps": "100000",
					"fitness_weight_patron_bps": "50000",
					"fitness_weight_unique_bps": "100000",
					"fitness_weight_age_bps": "100000",
					"fitness_weight_satisfaction_bps": "150000",
					"demand_tracking_enabled": false,
					"metabolism_base_cost": "100",
					"metabolism_energy_cap": "10000",
					"metabolism_initial_energy": "5000",
					"competition_redundancy_threshold_bps": "300000",
					"competition_max_niche_size": "50"
				},
				"facts": []
			},
			"zerone_staking": {
				"params": {
					"tier_configs": [
						{"name": "Apprentice", "min_stake": "111000"},
						{"name": "Verified", "min_stake": "1110000"},
						{"name": "Scholar", "min_stake": "1111000000"},
						{"name": "Guardian", "min_stake": "11111000000"}
					]
				}
			},
			"zerone_gov": {
				"params": {
					"voting_period_blocks": "34272",
					"discussion_period_blocks": "17136",
					"quorum_threshold_bps": "334000",
					"support_threshold_bps": "500000"
				}
			},
			"bank": {"balances": []}
		}
	}`

	var g map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(base))
	dec.UseNumber()
	if err := dec.Decode(&g); err != nil {
		panic(err)
	}

	for path, val := range overrides {
		setPath(g, strings.Split(path, "."), val)
	}

	return g
}

// setPath sets a value at a nested path, creating intermediate maps as needed.
func setPath(m map[string]interface{}, path []string, val interface{}) {
	for i, key := range path {
		if i == len(path)-1 {
			m[key] = val
			return
		}
		next, ok := m[key].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			m[key] = next
		}
		m = next
	}
}

func runCheck(t *testing.T, fn func(*checker, map[string]interface{}), g map[string]interface{}) *checker {
	t.Helper()
	c := &checker{profile: "testnet"}
	fn(c, g)
	return c
}

// ────────────────────────────────────────────────────────────────────
// Revenue Split
// ────────────────────────────────────────────────────────────────────

func TestRevenueSplit_Valid(t *testing.T) {
	g := makeGenesis(nil)
	c := runCheck(t, checkRevenueSplit, g)
	if c.failed != 0 {
		t.Fatalf("expected 0 failures, got %d", c.failed)
	}
}

func TestRevenueSplit_BadSum(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.vesting_rewards.params.revenue_split.development_bps": "0",
	})
	c := runCheck(t, checkRevenueSplit, g)
	if c.failed < 1 {
		t.Fatal("expected at least 1 failure for bad revenue split sum")
	}
}

// ────────────────────────────────────────────────────────────────────
// Protocol Sub-Split
// ────────────────────────────────────────────────────────────────────

func TestProtocolSubSplit_BadSum(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.vesting_rewards.params.protocol_sub_split.treasury_bps": "0",
	})
	c := runCheck(t, checkProtocolSubSplit, g)
	if c.failed != 1 {
		t.Fatalf("expected 1 failure, got %d", c.failed)
	}
}

// ────────────────────────────────────────────────────────────────────
// Founder Share
// ────────────────────────────────────────────────────────────────────

func TestFounderShare_ShareWithoutAddress(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.vesting_rewards.params.founder_share_bps": "70000",
	})
	c := &checker{profile: "production"}
	checkFounderShare(c, g)
	if c.failed < 1 {
		t.Fatal("expected failure: founder_share_bps > 0 but no address (production)")
	}
}

// ────────────────────────────────────────────────────────────────────
// Fitness Weights
// ────────────────────────────────────────────────────────────────────

func TestFitnessWeights_BadSum(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.params.fitness_weight_satisfaction_bps": "0",
	})
	c := runCheck(t, checkKnowledgeFitnessWeights, g)
	if c.failed != 1 {
		t.Fatalf("expected 1 failure for bad fitness weight sum, got %d", c.failed)
	}
}

// ────────────────────────────────────────────────────────────────────
// Demand-Fitness Coupling
// ────────────────────────────────────────────────────────────────────

func TestDemandFitnessCoupling_ZeroWeights(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.params.demand_tracking_enabled":          true,
		"app_state.knowledge.params.fitness_weight_satisfaction_bps":  "0",
		"app_state.knowledge.params.fitness_weight_query_bps":         "0",
	})
	c := runCheck(t, checkDemandFitnessCoupling, g)
	if c.warnings != 2 {
		t.Fatalf("expected 2 warnings for zero coupling weights, got %d", c.warnings)
	}
}

// ────────────────────────────────────────────────────────────────────
// Metabolism Consistency
// ────────────────────────────────────────────────────────────────────

func TestMetabolism_ZeroCostWithFitness(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.params.metabolism_base_cost": "0",
	})
	c := runCheck(t, checkMetabolismConsistency, g)
	if c.failed != 1 {
		t.Fatalf("expected 1 failure: metabolism_base_cost=0 with fitness enabled, got %d", c.failed)
	}
}

func TestMetabolism_FitnessDisabled(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.params.fitness_epoch_blocks":  "0",
		"app_state.knowledge.params.metabolism_base_cost": "0",
	})
	c := runCheck(t, checkMetabolismConsistency, g)
	if c.failed != 0 {
		t.Fatal("expected no failure when fitness disabled")
	}
}

// ────────────────────────────────────────────────────────────────────
// Energy Cap
// ────────────────────────────────────────────────────────────────────

func TestEnergyCap_LessThanBaseCost(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.params.metabolism_energy_cap":  "50",
		"app_state.knowledge.params.metabolism_base_cost": "100",
	})
	c := runCheck(t, checkEnergyCap, g)
	if c.failed != 1 {
		t.Fatalf("expected 1 failure: energy_cap < base_cost, got %d", c.failed)
	}
}

// ────────────────────────────────────────────────────────────────────
// Competition Params
// ────────────────────────────────────────────────────────────────────

func TestCompetition_MaxedOut(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.params.competition_redundancy_threshold_bps": "1000000",
		"app_state.knowledge.params.competition_max_niche_size":           "0",
	})
	c := runCheck(t, checkCompetitionParams, g)
	if c.failed != 2 {
		t.Fatalf("expected 2 failures for maxed competition params, got %d", c.failed)
	}
}

// ────────────────────────────────────────────────────────────────────
// Staking Tiers
// ────────────────────────────────────────────────────────────────────

func TestStakingTiers_NotIncreasing(t *testing.T) {
	tiers := []interface{}{
		map[string]interface{}{"name": "Apprentice", "min_stake": "1000000"},
		map[string]interface{}{"name": "Verified", "min_stake": "500000"},
	}
	g := makeGenesis(map[string]interface{}{
		"app_state.zerone_staking.params.tier_configs": tiers,
	})
	c := runCheck(t, checkStakingTierBoundaries, g)
	if c.failed < 1 {
		t.Fatal("expected failure for non-increasing tier stakes")
	}
}

// ────────────────────────────────────────────────────────────────────
// Governance
// ────────────────────────────────────────────────────────────────────

func TestGovernance_VotingLessThanDiscussion(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"app_state.zerone_gov.params.voting_period_blocks":     "100",
		"app_state.zerone_gov.params.discussion_period_blocks": "200",
	})
	c := runCheck(t, checkGovernancePeriods, g)
	if c.failed < 1 {
		t.Fatal("expected failure: voting < discussion")
	}
}

// ────────────────────────────────────────────────────────────────────
// Vote Extensions
// ────────────────────────────────────────────────────────────────────

func TestVoteExtensions_Disabled(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"consensus.params.abci.vote_extensions_enable_height": "0",
	})
	c := runCheck(t, checkVoteExtensions, g)
	if c.failed != 1 {
		t.Fatalf("expected 1 failure for disabled vote extensions, got %d", c.failed)
	}
}

// ────────────────────────────────────────────────────────────────────
// Axiom Seeds
// ────────────────────────────────────────────────────────────────────

func TestAxiomSeeds_ZeroEnergy(t *testing.T) {
	facts := []interface{}{
		map[string]interface{}{
			"id":     "fact-1",
			"domain": "physics",
			"status": "FACT_STATUS_VERIFIED",
			"energy": "0",
		},
	}
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.facts": facts,
	})
	c := runCheck(t, checkAxiomSeeds, g)
	if c.warnings < 1 {
		t.Fatal("expected warning for genesis facts with energy=0")
	}
}

func TestAxiomSeeds_InvalidStatus(t *testing.T) {
	facts := []interface{}{
		map[string]interface{}{
			"id":     "fact-1",
			"domain": "physics",
			"status": "FACT_STATUS_PENDING",
			"energy": "5000",
		},
	}
	g := makeGenesis(map[string]interface{}{
		"app_state.knowledge.facts": facts,
	})
	c := runCheck(t, checkAxiomSeeds, g)
	if c.failed < 1 {
		t.Fatal("expected failure for PENDING status in genesis fact")
	}
}

// ────────────────────────────────────────────────────────────────────
// Chain Metadata
// ────────────────────────────────────────────────────────────────────

func TestChainMetadata_BadPrefix(t *testing.T) {
	g := makeGenesis(map[string]interface{}{
		"chain_id": "badchain-1",
	})
	c := runCheck(t, checkChainMetadata, g)
	if c.failed < 1 {
		t.Fatal("expected failure for bad chain_id prefix")
	}
}

// ────────────────────────────────────────────────────────────────────
// dig helpers
// ────────────────────────────────────────────────────────────────────

func TestDig_Missing(t *testing.T) {
	m := map[string]interface{}{"a": "b"}
	if _, ok := dig(m, "x", "y"); ok {
		t.Fatal("expected false for missing path")
	}
}

func TestDigUint64_StringEncoded(t *testing.T) {
	m := map[string]interface{}{"val": "12345"}
	v, ok := digUint64(m, "val")
	if !ok || v != 12345 {
		t.Fatalf("expected 12345, got %d (ok=%v)", v, ok)
	}
}

func TestValidBech32(t *testing.T) {
	if !validBech32("zrn1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5d9pmh") {
		t.Fatal("expected valid bech32")
	}
	if validBech32("") {
		t.Fatal("empty should be invalid")
	}
	if validBech32("notbech32") {
		t.Fatal("no separator should be invalid")
	}
}
