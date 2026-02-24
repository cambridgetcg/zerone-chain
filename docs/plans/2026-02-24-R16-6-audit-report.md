# R16-6 Audit Report

## Date: 2026-02-24

### Results

| # | Check | Hits | Status | Notes |
|---|-------|------|--------|-------|
| 1 | burn_bps/BurnBps | 0 | PASS | No hits in Go or proto files |
| 2 | BurnCoins in revenue paths | 0 revenue | PASS | Only in x/tokens (user burns) and x/liquiditypool (LP burns). Removed from x/vesting_rewards expected_keepers. |
| 3 | BurnTokens function | 0 | PASS | Fully removed; replaced by DisburseFromDevelopmentFund |
| 4 | Old research default (130000) | 3 fixed | PASS | Fixed: x/tree/types/genesis.go, tests/cross_stack, tests/simulation. Knowledge module's `research_fund_share_bps=130000` is a separate parameter (see false positives). |
| 5 | Old burn default (100000) | 2 fixed | PASS | Fixed: x/tree/types/genesis.go BurnBp default 100000->196700, simulation test constants |
| 6 | GovernanceActivationHeight active logic | 0 | PASS | Only in: proto fields (DEPRECATED comment), default value (0), gRPC query response, test assertions |
| 7 | Revenue split sum validation | 3/3 correct | PASS | All use DevelopmentBps: billing/types, simulation/invariants, vesting_rewards/types |
| 8 | development_fund module account | registered | PASS | app.go registration, keys.go constant, multi-module keeper usage confirmed |
| 9 | Documentation consistency | 5 fixed | PASS | Fixed stale burn/13% refs in SINKS-AND-FLOWS, SUPPLY, REVIEW, GENESIS, PARAMETERS |
| 10 | Genesis config | 0 burn_bps | PASS | development_bps present in all 4 revenue split sections of testnet-genesis-config.json |

### Fixes Applied

**Code:**
- `x/tree/types/genesis.go` — ResearchFundBp: 130000 -> 33300, BurnBp: 100000 -> 196700
- `x/vesting_rewards/types/expected_keepers.go` — removed unused BurnCoins from BankKeeper interface
- `x/vesting_rewards/keeper/keeper_test.go` — removed burnedCoins field, BurnCoins mock method, and "no coins burned" assertions
- `tests/cross_stack/tree_revenue_test.go` — updated to new split values; dist.Burn -> dist.DevelopmentFund
- `tests/simulation/attack_helpers_test.go` — treeBurnBp -> treeDevelopmentBp, struct field burn -> developmentFund, values updated
- `tests/simulation/adversarial_sim_test.go` — updated comments, log messages, and research fund income calculation
- `app/app.go` — updated stale "burn" comments for billing, tree, partnerships, toolbox, capture_challenge modules

**Documentation:**
- `docs/tokenomics/SINKS-AND-FLOWS.md` — removed "burn mechanism" reference
- `docs/tokenomics/SUPPLY.md` — removed "burned tokens free headroom" claim
- `docs/tokenomics/REVIEW.md` — "13% revenue share" -> "3.33%", removed "burns" from economic mechanisms list
- `docs/tokenomics/GENESIS.md` — "13% revenue share" -> "3.33%"
- `docs/PARAMETERS.md` — added clarification that knowledge module's 130000 is a separate parameter from global revenue split

### False Positives Documented

| Hit | Reason |
|-----|--------|
| `x/knowledge/types/genesis.go:76` ResearchFundShareBps=130000 | Knowledge module's own parameter — share of knowledge rewards to research fund, NOT the global revenue split research share |
| `scripts/testnet-genesis-config.json:110` research_fund_share_bps=130000 | Same knowledge module parameter in genesis config |
| `scripts/testnet-genesis.sh:207` research_fund_share_bps=130000 | Same knowledge module parameter in genesis script |
| `x/toolbox/types/genesis.pb.go:104` comment "130000 (13%)" | Proto-generated file; comment reflects proto source. Will update when proto field is regenerated. |
| `docs/plans/2026-02-23-testnet-genesis.md` burn_bps references | Historical plan document — records pre-R16 state |
| `docs/plans/2026-02-23-knowledge-toolbox-tests.md` "13% research, 10% burn" | Historical plan document |
| `x/tree/types/genesis.pb.go:114` BurnBp proto field | Proto field rename pending (documented in genesis.go comment). Semantically repurposed to development fund share. |
| `x/tree/types/types.go:165` BurnBp in validation sum | Uses proto field name; sum validation is correct (fields total 1000000) |
| `x/tree/keeper/msg_server.go:980` params.BurnBp | Passes proto field to CalculateRevenue's developmentBp parameter; routing goes to development_fund |
| `authtypes.Burner` permissions in app.go | Cosmos SDK module-level permission; modules retain Burner capability for non-revenue operations (slashing, etc.) |
| `docs/tokenomics/*.md` "no burn" / "without burn" phrasing | Correct — these explain the no-burn policy |

### Remaining Items (Not Blocking)

1. **Tree proto field rename** — `burn_bp` -> `development_bp` in `proto/zerone/tree/v1/genesis.proto`. Requires proto regeneration. Currently documented with comment "proto field rename pending" in genesis.go.
2. **app.go authtypes.Burner audit** — Several modules (billing, tree, capture_challenge, partnerships) retain Burner permission but no longer call BurnCoins in their revenue paths. A follow-up could remove unnecessary Burner permissions where modules only route to development_fund.

### Verification

All affected packages compile clean:
```
go build ./x/vesting_rewards/... ✓
go build ./x/tree/... ✓
go build ./tests/cross_stack/... ✓
go build ./tests/simulation/... ✓
go build ./app/... ✓
go build ./tests/integration/... ✓
```

Key test suites pass:
```
TestScenario4_TreeRevenueDistribution ✓
TestScenario4_TreeRevenueEqualSplit ✓
TestScenario4_TreeRevenueNoContributors ✓
TestDistributeBlockReward_DepositsToDevelopmentFund ✓
TestRouteFees_SweepsFeeCollector ✓
TestCalculateRevenue ✓
```
