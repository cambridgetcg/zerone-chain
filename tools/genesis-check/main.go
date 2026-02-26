// tools/genesis-check/main.go — Zerone Genesis Invariant Checker
//
// Validates cross-module invariants in a genesis.json before chain launch.
//
//	go run tools/genesis-check/main.go --genesis path/to/genesis.json [--profile testnet|production]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

// ════════════════════════════════════════════════════════════════════════
//  JSON Navigation Helpers
// ════════════════════════════════════════════════════════════════════════

// dig navigates nested map[string]interface{} by key path.
func dig(m map[string]interface{}, path ...string) (interface{}, bool) {
	var current interface{} = m
	for _, key := range path {
		cm, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = cm[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func digMap(m map[string]interface{}, path ...string) (map[string]interface{}, bool) {
	v, ok := dig(m, path...)
	if !ok {
		return nil, false
	}
	result, ok := v.(map[string]interface{})
	return result, ok
}

// toUint64 converts a JSON value to uint64.
// Handles: json.Number (UseNumber), float64 (default), string (proto3 uint64 encoding).
func toUint64(v interface{}) (uint64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := strconv.ParseUint(string(n), 10, 64)
		if err != nil {
			return 0, false
		}
		return i, true
	case float64:
		return uint64(n), true
	case string:
		i, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func digUint64(m map[string]interface{}, path ...string) (uint64, bool) {
	v, ok := dig(m, path...)
	if !ok {
		return 0, false
	}
	return toUint64(v)
}

func digString(m map[string]interface{}, path ...string) (string, bool) {
	v, ok := dig(m, path...)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func digBool(m map[string]interface{}, path ...string) (bool, bool) {
	v, ok := dig(m, path...)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func digSlice(m map[string]interface{}, path ...string) ([]interface{}, bool) {
	v, ok := dig(m, path...)
	if !ok {
		return nil, false
	}
	s, ok := v.([]interface{})
	return s, ok
}

// validBech32 does a basic bech32 format check (no crypto deps).
func validBech32(addr string) bool {
	idx := strings.LastIndex(addr, "1")
	if idx < 1 || idx+7 > len(addr) {
		return false
	}
	for _, c := range addr[:idx] {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	for _, c := range addr[idx+1:] {
		if !strings.ContainsRune(charset, c) {
			return false
		}
	}
	return len(addr[idx+1:]) >= 6
}

// ════════════════════════════════════════════════════════════════════════
//  Checker
// ════════════════════════════════════════════════════════════════════════

type checker struct {
	profile  string
	passed   int
	failed   int
	warnings int
}

func (c *checker) section(name string) {
	fmt.Printf("\n\033[1m%s\033[0m\n", name)
}

func (c *checker) pass(format string, args ...interface{}) {
	fmt.Printf("  \033[32m✓\033[0m %s\n", fmt.Sprintf(format, args...))
	c.passed++
}

func (c *checker) fail(format string, args ...interface{}) {
	fmt.Printf("  \033[31m✗\033[0m %s\n", fmt.Sprintf(format, args...))
	c.failed++
}

func (c *checker) warn(format string, args ...interface{}) {
	fmt.Printf("  \033[33m⚠\033[0m %s\n", fmt.Sprintf(format, args...))
	c.warnings++
}

// ════════════════════════════════════════════════════════════════════════
//  1. Revenue Split (x/vesting_rewards)
// ════════════════════════════════════════════════════════════════════════

func checkRevenueSplit(c *checker, g map[string]interface{}) {
	c.section("Revenue Split")

	split, ok := digMap(g, "app_state", "vesting_rewards", "params", "revenue_split")
	if !ok {
		c.fail("revenue_split not found in vesting_rewards params")
		return
	}

	contributor, _ := toUint64(split["contributor_bps"])
	protocol, _ := toUint64(split["protocol_bps"])
	research, _ := toUint64(split["research_bps"])
	development, _ := toUint64(split["development_bps"])

	sum := contributor + protocol + research + development
	if sum == 1_000_000 {
		c.pass("BPS sum = %d (contributor=%d protocol=%d research=%d development=%d)",
			sum, contributor, protocol, research, development)
	} else {
		c.fail("BPS sum = %d, expected 1,000,000", sum)
	}

	if development > 0 {
		c.pass("Development fund = %d BPS", development)
	} else {
		c.fail("development_bps must be > 0")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  2. Protocol Sub-Split (x/vesting_rewards)
// ════════════════════════════════════════════════════════════════════════

func checkProtocolSubSplit(c *checker, g map[string]interface{}) {
	c.section("Protocol Sub-Split")

	split, ok := digMap(g, "app_state", "vesting_rewards", "params", "protocol_sub_split")
	if !ok {
		c.fail("protocol_sub_split not found in vesting_rewards params")
		return
	}

	citation, _ := toUint64(split["citation_bps"])
	verification, _ := toUint64(split["verification_bps"])
	treasury, _ := toUint64(split["treasury_bps"])

	sum := citation + verification + treasury
	if sum == 1_000_000 {
		c.pass("Sub-split sum = %d (citation=%d verification=%d treasury=%d)",
			sum, citation, verification, treasury)
	} else {
		c.fail("Sub-split sum = %d, expected 1,000,000", sum)
	}
}

// ════════════════════════════════════════════════════════════════════════
//  3. Founder Share (x/vesting_rewards)
// ════════════════════════════════════════════════════════════════════════

func checkFounderShare(c *checker, g map[string]interface{}) {
	c.section("Founder Share")

	shareBps, _ := digUint64(g, "app_state", "vesting_rewards", "params", "founder_share_bps")
	addr, _ := digString(g, "app_state", "vesting_rewards", "params", "founder_address")

	if shareBps == 0 {
		if addr == "" {
			c.pass("No founder share configured")
		} else {
			c.warn("founder_address set (%s) but founder_share_bps is 0", addr)
		}
		return
	}

	if addr == "" {
		if c.profile == "production" {
			c.fail("founder_address required when founder_share_bps = %d", shareBps)
		} else {
			c.warn("founder_address empty but founder_share_bps = %d", shareBps)
		}
	} else if !validBech32(addr) {
		c.fail("founder_address is not valid bech32: %s", addr)
	} else {
		c.pass("Founder address valid (%s...)", addr[:min(len(addr), 16)])
	}

	// founder_share_bps is BPS of the research allocation (70,000 = 7% of research)
	if shareBps > 100_000 {
		c.warn("founder_share_bps = %d (>10%% of research allocation)", shareBps)
	} else {
		c.pass("Founder share = %d BPS of research", shareBps)
	}
}

// ════════════════════════════════════════════════════════════════════════
//  4. Research Fund Voters (x/zerone_gov)
// ════════════════════════════════════════════════════════════════════════

func checkResearchFundVoters(c *checker, g map[string]interface{}) {
	c.section("Research Fund")

	voters, ok := digMap(g, "app_state", "zerone_gov", "params", "research_fund_voters")
	if !ok {
		if c.profile == "production" {
			c.fail("research_fund_voters not found in zerone_gov params")
		} else {
			c.warn("research_fund_voters not configured")
		}
		return
	}

	v1, _ := voters["voter1"].(string)
	v2, _ := voters["voter2"].(string)

	checkVoter := func(name, addr string) {
		if addr == "" {
			if c.profile == "production" {
				c.fail("%s address required for production", name)
			} else {
				c.warn("%s not set", name)
			}
		} else if !validBech32(addr) {
			c.fail("%s is not valid bech32: %s", name, addr)
		} else {
			c.pass("%s valid (%s...)", name, addr[:min(len(addr), 16)])
		}
	}

	checkVoter("voter1", v1)
	checkVoter("voter2", v2)

	if v1 != "" && v2 != "" {
		c.pass("2-of-2 research fund governance configured")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  5. Knowledge Fitness Weights (x/knowledge)
// ════════════════════════════════════════════════════════════════════════

func checkKnowledgeFitnessWeights(c *checker, g map[string]interface{}) {
	c.section("Knowledge Fitness")

	kp, ok := digMap(g, "app_state", "knowledge", "params")
	if !ok {
		c.fail("knowledge params not found")
		return
	}

	weights := []struct {
		key  string
		name string
	}{
		{"fitness_weight_query_bps", "query"},
		{"fitness_weight_citation_bps", "citation"},
		{"fitness_weight_bridge_bps", "bridge"},
		{"fitness_weight_depth_bps", "depth"},
		{"fitness_weight_patron_bps", "patron"},
		{"fitness_weight_unique_bps", "unique"},
		{"fitness_weight_age_bps", "age"},
		{"fitness_weight_satisfaction_bps", "satisfaction"},
	}

	var sum uint64
	parts := make([]string, 0, len(weights))
	for _, w := range weights {
		v, _ := toUint64(kp[w.key])
		sum += v
		parts = append(parts, fmt.Sprintf("%s=%d", w.name, v))
	}

	if sum == 1_000_000 {
		c.pass("Weights sum = %d (%s)", sum, strings.Join(parts, " "))
	} else {
		c.fail("Weights sum = %d, expected 1,000,000 (%s)", sum, strings.Join(parts, " "))
	}
}

// ════════════════════════════════════════════════════════════════════════
//  6. Demand-Fitness Weight Coupling (x/knowledge)
// ════════════════════════════════════════════════════════════════════════

func checkDemandFitnessCoupling(c *checker, g map[string]interface{}) {
	c.section("Demand-Fitness Coupling")

	kp, ok := digMap(g, "app_state", "knowledge", "params")
	if !ok {
		c.fail("knowledge params not found")
		return
	}

	demandEnabled, _ := kp["demand_tracking_enabled"].(bool)
	if !demandEnabled {
		c.pass("Demand tracking disabled — coupling check skipped")
		return
	}

	satWeight, _ := toUint64(kp["fitness_weight_satisfaction_bps"])
	queryWeight, _ := toUint64(kp["fitness_weight_query_bps"])

	if satWeight == 0 {
		c.warn("demand_tracking_enabled but fitness_weight_satisfaction_bps = 0 — satisfaction feedback not feeding fitness")
	} else {
		c.pass("Satisfaction weight = %d BPS (demand→fitness loop active)", satWeight)
	}

	if queryWeight == 0 {
		c.warn("demand_tracking_enabled but fitness_weight_query_bps = 0 — query demand data goes nowhere")
	} else {
		c.pass("Query weight = %d BPS (demand→fitness loop active)", queryWeight)
	}
}

// ════════════════════════════════════════════════════════════════════════
//  7. Knowledge Demand Tracking (x/knowledge)
// ════════════════════════════════════════════════════════════════════════

func checkKnowledgeDemandTracking(c *checker, g map[string]interface{}) {
	c.section("Knowledge Demand")

	kp, ok := digMap(g, "app_state", "knowledge", "params")
	if !ok {
		c.fail("knowledge params not found")
		return
	}

	enabled, _ := kp["demand_tracking_enabled"].(bool)
	if !enabled {
		c.pass("Demand tracking disabled")
		return
	}

	c.pass("Demand tracking enabled")

	threshold, _ := toUint64(kp["demand_bounty_threshold"])
	if threshold > 0 {
		c.pass("Bounty threshold = %d", threshold)
	} else {
		c.fail("demand_bounty_threshold must be > 0 when tracking enabled")
	}

	expiryEpochs, _ := toUint64(kp["demand_bounty_expiry_epochs"])
	if expiryEpochs > 0 {
		c.pass("Bounty expiry = %d epochs", expiryEpochs)
	} else {
		c.fail("demand_bounty_expiry_epochs must be > 0 when tracking enabled")
	}

	baseReward, _ := kp["demand_bounty_base_reward"].(string)
	if baseReward != "" && baseReward != "0" {
		c.pass("Bounty base reward = %s", baseReward)
	} else {
		c.fail("demand_bounty_base_reward required when tracking enabled")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  8. Metabolism Consistency (x/knowledge)
// ════════════════════════════════════════════════════════════════════════

func checkMetabolismConsistency(c *checker, g map[string]interface{}) {
	c.section("Knowledge Ecology — Metabolism")

	kp, ok := digMap(g, "app_state", "knowledge", "params")
	if !ok {
		c.fail("knowledge params not found")
		return
	}

	fitnessEpoch, _ := toUint64(kp["fitness_epoch_blocks"])
	if fitnessEpoch == 0 {
		c.pass("Fitness disabled (epoch_blocks=0) — metabolism check skipped")
		return
	}

	baseCost, _ := toUint64(kp["metabolism_base_cost"])
	if baseCost > 0 {
		c.pass("Metabolism base cost = %d (facts must earn energy to survive)", baseCost)
	} else {
		c.fail("metabolism_base_cost = 0 but fitness is enabled (epoch=%d) — facts never die, ecology broken", fitnessEpoch)
	}
}

// ════════════════════════════════════════════════════════════════════════
//  9. Energy Cap Sanity (x/knowledge)
// ════════════════════════════════════════════════════════════════════════

func checkEnergyCap(c *checker, g map[string]interface{}) {
	c.section("Knowledge Ecology — Energy Cap")

	kp, ok := digMap(g, "app_state", "knowledge", "params")
	if !ok {
		c.fail("knowledge params not found")
		return
	}

	energyCap, _ := toUint64(kp["metabolism_energy_cap"])
	baseCost, _ := toUint64(kp["metabolism_base_cost"])

	if baseCost == 0 {
		c.pass("Metabolism disabled — energy cap check skipped")
		return
	}

	if energyCap == 0 {
		c.fail("metabolism_energy_cap = 0 — facts can never hold energy")
		return
	}

	if energyCap > baseCost {
		c.pass("Energy cap (%d) > base cost (%d) — facts can survive at least one epoch", energyCap, baseCost)
	} else {
		c.fail("metabolism_energy_cap (%d) <= metabolism_base_cost (%d) — facts born dead, cannot survive one epoch",
			energyCap, baseCost)
	}

	initialEnergy, _ := toUint64(kp["metabolism_initial_energy"])
	if initialEnergy > 0 {
		c.pass("Initial energy = %d", initialEnergy)
	} else {
		c.warn("metabolism_initial_energy = 0 — new facts start with no energy")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  10. Competition Params (x/knowledge)
// ════════════════════════════════════════════════════════════════════════

func checkCompetitionParams(c *checker, g map[string]interface{}) {
	c.section("Knowledge Ecology — Competition")

	kp, ok := digMap(g, "app_state", "knowledge", "params")
	if !ok {
		c.fail("knowledge params not found")
		return
	}

	redundancyThreshold, hasRedundancy := toUint64(kp["competition_redundancy_threshold_bps"])
	maxNiche, hasNiche := toUint64(kp["competition_max_niche_size"])

	if !hasRedundancy && !hasNiche {
		c.pass("Competition params not set (defaults apply)")
		return
	}

	if hasRedundancy {
		if redundancyThreshold < 1_000_000 {
			c.pass("Redundancy threshold = %d BPS (below 1M — pruning can occur)", redundancyThreshold)
		} else {
			c.fail("competition_redundancy_threshold_bps = %d (>= 1,000,000) — nothing is ever redundant", redundancyThreshold)
		}
	}

	if hasNiche {
		if maxNiche > 0 {
			c.pass("Max niche size = %d (forced pruning active)", maxNiche)
		} else {
			c.fail("competition_max_niche_size = 0 — forced pruning disabled, niches grow unbounded")
		}
	}
}

// ════════════════════════════════════════════════════════════════════════
//  11. Bootstrap Fund (x/knowledge)
// ════════════════════════════════════════════════════════════════════════

func checkBootstrapFund(c *checker, g map[string]interface{}) {
	c.section("Bootstrap Fund")

	kp, ok := digMap(g, "app_state", "knowledge", "params")
	if !ok {
		c.fail("knowledge params not found")
		return
	}

	enabled, _ := kp["bootstrap_fund_enabled"].(bool)
	if !enabled {
		c.pass("Bootstrap fund disabled")
		return
	}

	c.pass("Bootstrap fund enabled")

	epochBlocks, _ := toUint64(kp["bootstrap_fund_epoch_blocks"])
	if epochBlocks > 0 {
		c.pass("Epoch = %d blocks", epochBlocks)
	} else {
		c.fail("bootstrap_fund_epoch_blocks must be > 0 when fund is enabled")
	}

	maxPerEpoch, _ := kp["bootstrap_fund_max_per_epoch"].(string)
	if maxPerEpoch != "" && maxPerEpoch != "0" {
		c.pass("Max per epoch = %s", maxPerEpoch)
	} else {
		c.fail("bootstrap_fund_max_per_epoch required when fund is enabled")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  12. Staking Tier Boundaries (x/zerone_staking)
// ════════════════════════════════════════════════════════════════════════

func checkStakingTierBoundaries(c *checker, g map[string]interface{}) {
	c.section("Staking Tiers")

	tiers, ok := digSlice(g, "app_state", "zerone_staking", "params", "tier_configs")
	if !ok || len(tiers) == 0 {
		c.fail("tier_configs not found or empty in zerone_staking params")
		return
	}

	c.pass("%d tiers configured", len(tiers))

	prevStake := new(big.Int)
	prevName := ""
	allIncreasing := true

	for i, raw := range tiers {
		tier, ok := raw.(map[string]interface{})
		if !ok {
			c.fail("tier_configs[%d] is not a valid object", i)
			allIncreasing = false
			continue
		}

		name, _ := tier["name"].(string)
		stakeStr, _ := tier["min_stake"].(string)
		stake := new(big.Int)
		if _, ok := stake.SetString(stakeStr, 10); !ok {
			c.fail("tier %s: min_stake '%s' is not a valid integer", name, stakeStr)
			allIncreasing = false
			continue
		}

		if i > 0 && stake.Cmp(prevStake) <= 0 {
			c.fail("Tier %s min_stake (%s) <= %s min_stake (%s) — must be strictly increasing",
				name, stakeStr, prevName, prevStake.String())
			allIncreasing = false
		}

		prevStake.Set(stake)
		prevName = name
	}

	if allIncreasing {
		c.pass("Tier min_stake strictly increasing")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  13. Governance Periods (x/zerone_gov)
// ════════════════════════════════════════════════════════════════════════

func checkGovernancePeriods(c *checker, g map[string]interface{}) {
	c.section("Governance")

	gp, ok := digMap(g, "app_state", "zerone_gov", "params")
	if !ok {
		c.fail("zerone_gov params not found")
		return
	}

	votingBlocks, _ := toUint64(gp["voting_period_blocks"])
	discussionBlocks, _ := toUint64(gp["discussion_period_blocks"])

	if votingBlocks > discussionBlocks {
		c.pass("Voting period (%d blocks) > discussion period (%d blocks)", votingBlocks, discussionBlocks)
	} else {
		c.fail("voting_period_blocks (%d) must exceed discussion_period_blocks (%d)",
			votingBlocks, discussionBlocks)
	}

	quorumBps, _ := toUint64(gp["quorum_threshold_bps"])
	if quorumBps > 0 && quorumBps <= 1_000_000 {
		c.pass("Quorum threshold = %d BPS (%.1f%%)", quorumBps, float64(quorumBps)/10_000)
	} else {
		c.fail("quorum_threshold_bps must be in (0, 1,000,000], got %d", quorumBps)
	}

	supportBps, _ := toUint64(gp["support_threshold_bps"])
	if supportBps > 0 && supportBps <= 1_000_000 {
		c.pass("Support threshold = %d BPS", supportBps)
	} else {
		c.fail("support_threshold_bps must be in (0, 1,000,000], got %d", supportBps)
	}
}

// ════════════════════════════════════════════════════════════════════════
//  14. Vote Extensions (consensus params)
// ════════════════════════════════════════════════════════════════════════

func checkVoteExtensions(c *checker, g map[string]interface{}) {
	c.section("Vote Extensions")

	// SDK v0.50: consensus params at top-level .consensus.params
	height, ok := digString(g, "consensus", "params", "abci", "vote_extensions_enable_height")
	if !ok {
		// Fallback: older SDK layout at .consensus_params.abci
		height, ok = digString(g, "consensus_params", "abci", "vote_extensions_enable_height")
	}
	if !ok {
		c.fail("vote_extensions_enable_height not found in consensus params")
		return
	}

	if height == "1" {
		c.pass("Vote extensions enabled from block 1 (required for PoT)")
	} else {
		c.fail("vote_extensions_enable_height = %q, must be \"1\" for PoT", height)
	}
}

// ════════════════════════════════════════════════════════════════════════
//  15. Bank Balances (informational)
// ════════════════════════════════════════════════════════════════════════

func checkBankBalances(c *checker, g map[string]interface{}) {
	c.section("Bank & Balances")

	balances, ok := digSlice(g, "app_state", "bank", "balances")
	if !ok {
		c.warn("bank.balances not found or empty")
		return
	}

	c.pass("%d funded accounts at genesis", len(balances))

	totalUzrn := new(big.Int)
	for _, raw := range balances {
		bal, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		coins, ok := bal["coins"].([]interface{})
		if !ok {
			continue
		}
		for _, rawCoin := range coins {
			coin, ok := rawCoin.(map[string]interface{})
			if !ok {
				continue
			}
			denom, _ := coin["denom"].(string)
			if denom != "uzrn" {
				continue
			}
			amtStr, _ := coin["amount"].(string)
			amt := new(big.Int)
			if _, ok := amt.SetString(amtStr, 10); ok {
				totalUzrn.Add(totalUzrn, amt)
			}
		}
	}

	// Convert to ZRN for display (1 ZRN = 1,000,000 uzrn)
	zrn := new(big.Int).Div(totalUzrn, big.NewInt(1_000_000))
	c.pass("Total genesis supply: %s uzrn (%s ZRN)", totalUzrn.String(), zrn.String())
}

// ════════════════════════════════════════════════════════════════════════
//  16. Axiom Seeds (x/knowledge genesis)
// ════════════════════════════════════════════════════════════════════════

func checkAxiomSeeds(c *checker, g map[string]interface{}) {
	c.section("Genesis Facts")

	facts, ok := digSlice(g, "app_state", "knowledge", "facts")
	if !ok || len(facts) == 0 {
		c.pass("No genesis facts loaded")
		return
	}

	c.pass("%d genesis facts loaded", len(facts))

	validStatuses := map[string]bool{
		"FACT_STATUS_VERIFIED":    true,
		"FACT_STATUS_ACTIVE":      true,
		"FACT_STATUS_PROVISIONAL": true,
		// Also handle integer enum values (proto3 allows both)
		"3": true, // VERIFIED
		"4": true, // ACTIVE
		"2": true, // PROVISIONAL
	}

	structureIssues := 0
	statusIssues := 0
	energyIssues := 0
	domainIssues := 0

	for _, raw := range facts {
		fact, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		factID, _ := fact["id"].(string)

		// Domain check
		domain, _ := fact["domain"].(string)
		if domain == "" {
			domainIssues++
			if domainIssues <= 3 {
				c.fail("Genesis fact %s has empty domain", factID)
			}
		}

		// Status check
		status := ""
		switch v := fact["status"].(type) {
		case string:
			status = v
		case json.Number:
			status = string(v)
		}
		if !validStatuses[status] {
			statusIssues++
			if statusIssues <= 3 {
				c.fail("Genesis fact %s has unexpected status %q (expected VERIFIED/ACTIVE/PROVISIONAL)", factID, status)
			}
		}

		// Structure check
		if structure, ok := fact["structure"].(map[string]interface{}); ok {
			subject, _ := structure["subject"].(string)
			if subject == "" {
				structureIssues++
				if structureIssues <= 3 {
					c.fail("Genesis fact %s has structure but empty subject", factID)
				}
			}
		}

		// Energy check — genesis facts with 0 energy will be pruned in first fitness epoch
		energy, _ := toUint64(fact["energy"])
		if energy == 0 {
			energyIssues++
		}
	}

	if domainIssues > 3 {
		c.fail("... and %d more facts with empty domain", domainIssues-3)
	}
	if statusIssues > 3 {
		c.fail("... and %d more facts with invalid status", statusIssues-3)
	}
	if structureIssues > 3 {
		c.fail("... and %d more facts with empty structure.subject", structureIssues-3)
	}

	if domainIssues == 0 {
		c.pass("All genesis facts have valid domain")
	}
	if statusIssues == 0 {
		c.pass("All genesis facts have valid status")
	}
	if energyIssues > 0 {
		c.warn("%d/%d genesis facts have energy=0 — will be pruned in first fitness epoch unless grace period applies",
			energyIssues, len(facts))
	} else {
		c.pass("All genesis facts have energy > 0")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  17. Chain Metadata
// ════════════════════════════════════════════════════════════════════════

func checkChainMetadata(c *checker, g map[string]interface{}) {
	c.section("Chain Metadata")

	chainID, _ := digString(g, "chain_id")
	if strings.HasPrefix(chainID, "zerone-") {
		c.pass("Chain ID: %s", chainID)
	} else {
		c.fail("chain_id %q should start with \"zerone-\"", chainID)
	}

	genesisTimeStr, _ := digString(g, "genesis_time")
	if genesisTimeStr != "" {
		t, err := time.Parse(time.RFC3339, genesisTimeStr)
		if err != nil {
			c.fail("genesis_time %q is not valid RFC3339", genesisTimeStr)
		} else if t.Before(time.Now()) {
			c.warn("Genesis time is in the past — acceptable for testnet")
		} else {
			c.pass("Genesis time: %s", genesisTimeStr)
		}
	} else {
		c.fail("genesis_time not set")
	}
}

// ════════════════════════════════════════════════════════════════════════
//  Main
// ════════════════════════════════════════════════════════════════════════

func main() {
	genesisPath := flag.String("genesis", "", "Path to genesis.json")
	profile := flag.String("profile", "testnet", "Validation profile: testnet or production")
	flag.Parse()

	if *genesisPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run tools/genesis-check/main.go --genesis <path> [--profile testnet|production]")
		os.Exit(1)
	}

	if *profile != "testnet" && *profile != "production" {
		fmt.Fprintf(os.Stderr, "Invalid profile %q: must be 'testnet' or 'production'\n", *profile)
		os.Exit(1)
	}

	data, err := os.ReadFile(*genesisPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading genesis: %v\n", err)
		os.Exit(1)
	}

	var genesis map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&genesis); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing genesis JSON: %v\n", err)
		os.Exit(1)
	}

	chainID, _ := digString(genesis, "chain_id")
	genesisTime, _ := digString(genesis, "genesis_time")

	fmt.Println("═══════════════════════════════════════════════")
	fmt.Println("  ZERONE GENESIS INVARIANT CHECK")
	fmt.Printf("  Chain ID:      %s\n", chainID)
	fmt.Printf("  Genesis Time:  %s\n", genesisTime)
	fmt.Printf("  Profile:       %s\n", *profile)
	fmt.Println("═══════════════════════════════════════════════")

	c := &checker{profile: *profile}

	checkRevenueSplit(c, genesis)
	checkProtocolSubSplit(c, genesis)
	checkFounderShare(c, genesis)
	checkResearchFundVoters(c, genesis)
	checkKnowledgeFitnessWeights(c, genesis)
	checkDemandFitnessCoupling(c, genesis)
	checkKnowledgeDemandTracking(c, genesis)
	checkMetabolismConsistency(c, genesis)
	checkEnergyCap(c, genesis)
	checkCompetitionParams(c, genesis)
	checkBootstrapFund(c, genesis)
	checkStakingTierBoundaries(c, genesis)
	checkGovernancePeriods(c, genesis)
	checkVoteExtensions(c, genesis)
	checkBankBalances(c, genesis)
	checkAxiomSeeds(c, genesis)
	checkChainMetadata(c, genesis)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════")
	fmt.Printf("  RESULT: %d passed, %d failed, %d warnings\n", c.passed, c.failed, c.warnings)
	fmt.Println("═══════════════════════════════════════════════")

	if c.failed > 0 {
		os.Exit(1)
	}
}
