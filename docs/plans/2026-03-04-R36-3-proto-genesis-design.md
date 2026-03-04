# R36-3 Proto Genesis & Params — Design

## Overview

Rewrite genesis.proto Params (132 → 39 fields) and GenesisState (7 → 14 fields) for the training data protocol. Replace 777 axioms with 25 curated seed samples. Update DefaultParams/DefaultGenesis in Go.

## Changes

### genesis.proto
- **Params**: Replace 132 epistemic fields with 39 training-data-focused fields (submission limits, quality thresholds, consent multipliers, ecology, bounties, research fund)
- **GenesisState**: Add datasets, demands, bounties, validators, 4 sequence counters. Remove bootstrap_fund_allocation.

### genesis_seeds.json (NEW)
25 seed samples: 5 discussion, 5 troubleshooting, 5 debate, 5 explanation, 5 Q&A. All gold-tier, self-authored.

### genesis.go
- New DefaultParams() with 39 sensible defaults
- New DefaultGenesis() loading seeds
- New Validate() for 39 params
- New DefaultDomains() with 9 training-data domains

### seed_embed.go (replaces axiom_embed.go)
`//go:embed genesis_seeds.json` → `GenesisSeedsJSON []byte`
