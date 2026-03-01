# Zerone Tokenomics

> **Status:** Pre-testnet review (2026-02-23)  
> **Token:** ZRN (micro-denomination: uzrn, 1 ZRN = 1,000,000 uzrn)  
> **Chain:** Cosmos SDK v0.50, CometBFT consensus

## Overview

Zerone is a **Proof-of-Truth** (PoT) blockchain where all tokens are minted through verified knowledge contribution — not through proof-of-work or proof-of-stake inflation. There is no pre-mine, no ICO, and no token sale. All ZRN enters circulation as block rewards for producing blocks that contain verified truth claims.

## Documents in This Directory

| File | Description |
|------|-------------|
| [SUPPLY.md](SUPPLY.md) | Supply cap, emission schedule, decay curve, and long-term projections |
| [REVENUE-SPLIT.md](REVENUE-SPLIT.md) | The 4-way revenue split, protocol sub-split, and founder sunset |
| [VESTING.md](VESTING.md) | Truth-linked vesting: epistemic categories, release curves, clawback |
| [STAKING.md](STAKING.md) | Tiered validator system, staking economics, slashing |
| [GENESIS.md](GENESIS.md) | Genesis distribution, bootstrap accounts, and ceremony |
| [SINKS-AND-FLOWS.md](SINKS-AND-FLOWS.md) | Complete map of where ZRN is created, destroyed, and moves |
| [GOVERNANCE-MIGRATION.md](GOVERNANCE-MIGRATION.md) | 4-phase research fund governance: from founder pair to full community |
| [REVIEW.md](REVIEW.md) | Honest assessment: strengths, risks, open questions |

## Quick Numbers

| Metric | Value |
|--------|-------|
| **Max Supply** | 222,222,222 ZRN (hard cap, enforced in code) |
| **Genesis Circulating** | 0 ZRN (pure PoT — everything minted from block 1) |
| **Initial Block Reward** | 10 ZRN/block |
| **Block Time** | ~2.521 seconds |
| **Epoch Length** | 100,000 blocks (~2.9 days) |
| **Decay Rate** | 0.85× per epoch |
| **Floor Reward** | 0.1 ZRN/block |
| **Revenue to Contributors** | 55% |
| **Revenue to Protocol** | 22% |
| **Revenue to Development** | 19.67% (bug bounties, truth discovery, protocol dev) |
| **Revenue to Research Fund** | 3.33% |
| **Burn** | 0% — every ZRN does productive work |

## Design Philosophy

1. **Truth creates value, not computation.** Block rewards flow only when the chain produces verified knowledge. Empty blocks earn nothing.

2. **Every token works.** No burn — all revenue goes to productive purposes. The 222M supply cap provides natural scarcity; artificial deflation through burning is unnecessary when you can fund bug bounties and truth discovery instead.

3. **Knowledge has memory.** Rewards vest according to epistemic category — mathematical proofs vest slowly (because they should last forever), oracle feeds vest quickly (because they expire). If a claim is falsified, rewards are clawed back.

4. **Anti-capture as infrastructure.** HHI-based concentration monitoring, tiered validators with reputation requirements, and cross-stratum verification make knowledge monopolies structurally expensive to maintain.

5. **The chain can heal itself.** The autopoiesis module adjusts slash severity and economic parameters based on a System Stability Index (SSI), within governance-bounded rails. The alignment module monitors five health dimensions. The chain doesn't just enforce rules — it adapts them.
