# Running the Local Testnet

## Prerequisites
- Go 1.24+
- jq 1.6+

## Quick Start
```bash
scripts/localnet.sh start
scripts/localnet-test.sh
scripts/localnet.sh stop
```

## Validator Configuration
| Validator | Stake | Expected Tier |
|-----------|-------|---------------|
| val0 | 100 ZRN | Apprentice |
| val1 | 1,000 ZRN | Verified |
| val2 | 10,000 ZRN | Guardian |
| val3 | 100,000 ZRN | Architect |

## Endpoints
| Validator | RPC | gRPC | API |
|-----------|-----|------|-----|
| val0 | 127.0.0.1:26601 | 127.0.0.1:9090 | 127.0.0.1:1317 |
| val1 | 127.0.0.1:26611 | 127.0.0.1:9091 | 127.0.0.1:1318 |
| val2 | 127.0.0.1:26621 | 127.0.0.1:9092 | 127.0.0.1:1319 |
| val3 | 127.0.0.1:26631 | 127.0.0.1:9093 | 127.0.0.1:1320 |

## Commands
```bash
scripts/localnet.sh start       # Build, init, and start 4-validator localnet
scripts/localnet.sh stop        # Stop all validators
scripts/localnet.sh clean       # Remove all localnet state
scripts/localnet.sh status      # Show validator heights and voting power
scripts/localnet.sh logs N      # Tail logs for validator N (0-3)
scripts/localnet-test.sh        # Run all 9 integration tests
scripts/localnet-test.sh NAME   # Run a single test by name
```

## Integration Tests
| Test | What it verifies |
|------|------------------|
| genesis_invariants | Cross-module genesis consistency |
| block_production | All 4 validators producing blocks |
| validator_set | 4 validators bonded in active set |
| delegation | Token delegation tx succeeds |
| tier_check | Validators have distinct stake tiers |
| pot_round | Full PoT commit/reveal/verdict cycle |
| slashing | Validator jailed for downtime |
| recovery | Jailed validator unjailed after cooldown |
| governance | LIP lifecycle: submit, stake, advance, vote |

## Genesis Overrides for Local Testing
The localnet uses shortened params for fast testing:
- **Knowledge**: commit=10 blocks, reveal=10 blocks, aggregation=5 blocks, min_verifiers=2
- **Slashing**: signed_blocks_window=100, downtime_jail_duration=60s
- **Governance (SDK)**: voting_period=60s
- **Governance (Zerone)**: voting_period=10 blocks, review=5 blocks, required_stake=1 ZRN
- **Gas**: minimum-gas-prices=1uzrn (enforced by ZRNGasDecorator)

## Known Issues
- `test_slashing` takes ~180s because `signed_blocks_window=100` at 2.5s/block
- `test_recovery` requires 60s wait for `downtime_jail_duration` to expire
- Governance LIP may show "failed" status if quorum threshold isn't met by test votes
- val0 (port 26601) is used as the default RPC endpoint for all test queries — do not stop val0 during tests
