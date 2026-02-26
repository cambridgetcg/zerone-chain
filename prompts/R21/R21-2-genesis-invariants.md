# R21-2 — Genesis Invariant Checker

## Context

Zerone's genesis.json is the product of 20 rounds of development across 32 modules. Revenue splits must sum to 1,000,000 BPS. Founder share must be immutable. Research fund must point to the correct multisig. Bootstrap fund must have the right epoch config. Fitness weights must sum to 1,000,000. Demand tracking params must be consistent.

No tool currently validates these invariants. A misconfigured genesis means a broken chain at block 0 — or worse, a chain that boots but silently has wrong economics.

## Task

### Create `tools/genesis-check/main.go`

A standalone CLI tool that reads a genesis.json and validates every cross-module invariant.

```bash
go run tools/genesis-check/main.go --genesis path/to/genesis.json
```

Output: a pass/fail checklist with clear error messages for failures.

### Invariants to Check

#### 1. Revenue Split (x/vesting_rewards)

```go
// All BPS fields must sum to exactly 1,000,000
sum := params.ContributorBps + params.ProtocolBps + params.DevelopmentBps + params.ResearchBps
assert(sum == 1_000_000, "revenue split sums to %d, expected 1,000,000", sum)

// No burn
assert(params.BurnBps == 0 || !fieldExists("burn_bps"), "burn_bps must be 0 or absent")

// Development fund > 0
assert(params.DevelopmentBps > 0, "development_bps must be > 0")
```

#### 2. Founder Share Immutability (x/vesting_rewards)

```go
// Founder share and address must both be set or both unset
if params.FounderShareBps > 0 {
    assert(params.FounderAddress != "", "founder_address required when founder_share_bps > 0")
    assert(validBech32(params.FounderAddress), "founder_address is not valid bech32")
}
// Founder share must be within expected range (0.23% = 2,300 BPS of research, or ~77 BPS of total)
// This is a soft check — warn if outside expected range
if params.FounderShareBps > 0 && params.FounderShareBps > 500 {
    warn("founder_share_bps=%d seems high (expected ~77)", params.FounderShareBps)
}
```

#### 3. Research Fund Addresses (x/research)

```go
// Research fund address must be set
assert(params.ResearchFundAddress != "", "research_fund_address required")
assert(validBech32(params.ResearchFundAddress), "research_fund_address invalid")

// If multisig threshold is set, verify it
if params.MultisigThreshold > 0 {
    assert(params.MultisigThreshold == 2, "expected 2-of-2 multisig, got %d-of-N", params.MultisigThreshold)
}
```

#### 4. Knowledge Fitness Weights (x/knowledge)

```go
// Fitness weights must sum to 1,000,000 BPS
sum := params.FitnessWeightQueryBps +
    params.FitnessWeightCitationBps +
    params.FitnessWeightBridgeBps +
    params.FitnessWeightDepthBps +
    params.FitnessWeightPatronBps +
    params.FitnessWeightUniqueBps +
    params.FitnessWeightAgeBps +
    params.FitnessWeightSatisfactionBps
assert(sum == 1_000_000, "fitness weights sum to %d, expected 1,000,000", sum)
```

#### 5. Knowledge Demand Tracking (x/knowledge)

```go
// If demand tracking enabled, bounty params must be set
if params.DemandTrackingEnabled {
    assert(params.DemandBountyThreshold > 0, "demand_bounty_threshold must be > 0 when tracking enabled")
    assert(params.DemandBountyExpiryEpochs > 0, "demand_bounty_expiry_epochs required")
    assert(params.DemandBountyBaseReward != "" && params.DemandBountyBaseReward != "0",
        "demand_bounty_base_reward required")
}
```

#### 6. Bootstrap Fund (x/knowledge)

```go
// If bootstrap fund enabled, epoch blocks and amount must be set
if params.BootstrapFundEnabled {
    assert(params.BootstrapFundEpochBlocks > 0, "bootstrap_fund_epoch_blocks required")
    assert(params.BootstrapFundAmountPerEpoch != "" && params.BootstrapFundAmountPerEpoch != "0",
        "bootstrap_fund_amount_per_epoch required")
}
```

#### 7. Staking Tier Boundaries (x/staking)

```go
// Tier thresholds must be strictly increasing
for i := 1; i < len(params.TierThresholds); i++ {
    assert(params.TierThresholds[i] > params.TierThresholds[i-1],
        "tier thresholds must be strictly increasing: tier %d (%d) <= tier %d (%d)",
        i, params.TierThresholds[i], i-1, params.TierThresholds[i-1])
}
```

#### 8. Governance Periods (x/gov)

```go
// Voting period must be > deposit period
assert(params.VotingPeriod > params.DepositPeriod,
    "voting_period (%s) must exceed deposit_period (%s)", params.VotingPeriod, params.DepositPeriod)

// Quorum must be > 0 and <= 1
assert(params.Quorum > 0 && params.Quorum <= 1,
    "quorum must be in (0, 1], got %f", params.Quorum)
```

#### 9. Vote Extensions (consensus params)

```go
// Vote extensions must be enabled from block 1 for PoT
abci := genesis.ConsensusParams.Abci
assert(abci.VoteExtensionsEnableHeight == 1,
    "vote_extensions_enable_height must be 1 for PoT, got %d", abci.VoteExtensionsEnableHeight)
```

#### 10. Module Account Balances (bank genesis)

```go
// Protocol treasury, development fund, knowledge module should have expected genesis balances
// (or zero if minted via inflation). Warn if any module account has unexpected large balances.
for _, balance := range genesis.AppState.Bank.Balances {
    if isModuleAccount(balance.Address) {
        if totalCoins(balance) > threshold {
            warn("module account %s has %s at genesis — is this intentional?", balance.Address, totalCoins(balance))
        }
    }
}
```

#### 11. Axiom Seeds (x/knowledge genesis)

```go
// If knowledge genesis has pre-loaded facts, verify they have valid structure
for _, fact := range genesis.AppState.Knowledge.Facts {
    assert(fact.Domain != "", "genesis fact %s has empty domain", fact.Id)
    assert(fact.Structure != nil, "genesis fact %s has nil structure", fact.Id)
    assert(fact.Structure.Subject != "", "genesis fact %s has empty subject", fact.Id)
    assert(fact.Status == "VERIFIED" || fact.Status == "AXIOM",
        "genesis fact %s has unexpected status %s", fact.Id, fact.Status)
}
```

#### 12. Chain Metadata

```go
// Chain ID format
assert(strings.HasPrefix(genesis.ChainID, "zerone-"), "chain_id should start with 'zerone-'")

// Genesis time is in the future (for production) or past (for testnet)
// Just warn, don't fail
if genesis.GenesisTime.Before(time.Now()) {
    warn("genesis_time is in the past — acceptable for testnet, not for mainnet")
}
```

### Output Format

```
═══════════════════════════════════════════════
  ZERONE GENESIS INVARIANT CHECK
  Chain ID: zerone-testnet-1
  Genesis Time: 2026-03-01T00:00:00Z
═══════════════════════════════════════════════

Revenue Split
  ✅ BPS sum = 1,000,000
  ✅ No burn configured
  ✅ Development fund = 196,700 BPS

Founder Share
  ✅ Founder address valid (lgm1g0q9amg6...)
  ✅ Share = 77 BPS (0.23% of research)

Research Fund
  ✅ Address valid (lgm120p3d4h...)
  ✅ 2-of-2 multisig threshold

Knowledge Fitness
  ✅ Weights sum = 1,000,000

Knowledge Demand
  ✅ Demand tracking enabled
  ✅ Bounty threshold = 5
  ✅ Bounty base reward = 10,000,000 uzrn

Bootstrap Fund
  ✅ Enabled, epoch = 1000 blocks

Staking Tiers
  ✅ 4 tiers, strictly increasing

Governance
  ✅ Voting period > deposit period
  ✅ Quorum = 0.334

Vote Extensions
  ✅ Enabled from block 1

Module Accounts
  ⚠️  protocol_treasury has 0 uzrn at genesis (expected — minted via inflation)

Axiom Seeds
  ✅ 777 genesis facts loaded
  ✅ All have valid structure

Chain Metadata
  ✅ Chain ID: zerone-testnet-1
  ⚠️  Genesis time is in the past (testnet OK)

═══════════════════════════════════════════════
  RESULT: 18 passed, 0 failed, 2 warnings
═══════════════════════════════════════════════
```

### Integration

Add to `Makefile`:

```makefile
genesis-check:
	@go run tools/genesis-check/main.go --genesis $(GENESIS)
```

Add to `scripts/localnet-test.sh` as test 0 (runs before all others):

```bash
test_genesis_invariants() {
    info "Checking genesis invariants..."
    go run tools/genesis-check/main.go \
        --genesis "${COORDINATOR_HOME}/config/genesis.json"
}
```

### Profiles

Support two profiles via `--profile` flag:

- `testnet` (default) — relaxed: allows past genesis time, shorter governance periods, low stakes
- `production` — strict: future genesis time required, minimum governance periods, founder address must match known value, research fund must be the known multisig address

```bash
# Testnet (relaxed)
go run tools/genesis-check/main.go --genesis genesis.json --profile testnet

# Production (strict)
go run tools/genesis-check/main.go --genesis genesis.json --profile production
```

## Exit Criteria

1. `tools/genesis-check/main.go` builds and runs
2. Passes on localnet genesis (after R21-1 fixes)
3. Catches at least 3 intentionally broken invariants (test with bad genesis)
4. Integrated into localnet-test.sh
5. Production profile documented

## Spec-to-Reality Corrections

The original spec assumed field names that differ from actual proto definitions. Key mappings:

| Spec Assumption | Actual Codebase |
|---|---|
| Flat `burn_bps` field in revenue_split | Field doesn't exist — revenue_split has only contributor/protocol/research/development |
| `research_fund_address` + `multisig_threshold` in x/research | `research_fund_voters.voter1`/`voter2` in **x/zerone_gov** params — 2-of-2 is implicit |
| `TierThresholds[]` integer array | `tier_configs[]` with `min_stake` as string (big.Int), in **x/zerone_staking** |
| SDK gov `VotingPeriod`/`DepositPeriod` (time.Duration) | Custom zerone_gov: `voting_period_blocks`/`discussion_period_blocks` (uint64) |
| `Quorum` as 0-1 float | `quorum_threshold_bps` (uint64, 0-1,000,000 BPS) |
| `bootstrap_fund_amount_per_epoch` | `bootstrap_fund_max_per_epoch` |
| Fact status `"VERIFIED"` or `"AXIOM"` | Enum: `FACT_STATUS_VERIFIED`, `FACT_STATUS_ACTIVE`, `FACT_STATUS_PROVISIONAL` — no AXIOM status |
| `fact.Structure.Subject` | `fact.structure.subject` (ClaimStructure nested message) |
| Address prefix `lgm1` | Address prefix is `zrn` (app/constants.go) |
| `consensus_params.abci.vote_extensions_enable_height` | `consensus.params.abci.vote_extensions_enable_height` (SDK v0.50 module path) |
| Proto uint64 as JSON numbers | Proto3 encodes uint64 as JSON **strings** — parser must handle both |
| `founder_share_bps` expected ~77 (of total) | Actually 70,000 BPS (of research allocation, not total) — 7% of research ≈ 0.23% of total |

## Commit Convention

```
feat(tools): genesis invariant checker
test(tools): genesis-check edge cases — bad splits, missing addresses, wrong weights
docs(tools): genesis-check usage and profiles
```
