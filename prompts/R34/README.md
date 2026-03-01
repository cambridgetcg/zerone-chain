# R34 — 橋 (Hashi): Bridge — IBC, Upgrades & Operational Readiness

**Goal:** Wire ZERONE to the interchain, prove the upgrade path works, and build the operational tooling needed to run a production network.

橋 (Hashi) means "bridge" — connecting what was isolated to the wider world. A blockchain that can't upgrade, can't connect to others, and can't be monitored isn't ready for mainnet.

## Sessions (5)

| # | File | Scope |
|---|------|-------|
| R34-1 | R34-1-ibc-integration.md | IBC transfer tests with interchaintest — real relayer, real counterparty chain |
| R34-2 | R34-2-upgrade-path.md | Cosmovisor upgrade test — binary swap at scheduled height, state migration |
| R34-3 | R34-3-monitoring.md | Prometheus metrics, Grafana dashboards, alerting rules for ZERONE-specific modules |
| R34-4 | R34-4-cli-completeness.md | CLI audit — every msg and query has a working CLI command with tests |
| R34-5 | R34-5-documentation.md | API docs, module specs, operator guide, genesis ceremony guide |

## Run Order

All 5 can run in parallel — different concerns, no dependencies.

## The Deeper Pattern

| Framework | R-batch | What it does |
|-----------|---------|-------------|
| 鍛 Tan (R32) | Forging | Proved the system works |
| 試 Shi (R33) | Trial | Proved the system survives |
| **橋 Hashi (R34)** | **Bridge** | **Connects the system to the world** |
