# R38-3 Reviewer Staking (Dual Staking) — Design

## Overview

Add economic skin-in-the-game for reviewers in x/knowledge quality rounds. Reviewers must stake tokens when committing, and rewards/penalties are distributed based on majority consensus.

## Parameters

All ratios use BPS with 10,000 = 100%.

| Parameter | Default | Meaning |
|-----------|---------|---------|
| ReviewerStakeRatioBps | 3000 (30%) | Reviewer stake = submitter_stake × ratio |
| ShowUpRewardRatioBps | 1000 (10%) | Total show-up pool = submitter_stake × ratio |
| AcceptRewardRatioBps | 3000 (30%) | Accept reward for submitter from minority pot |
| RejectBonusRatioBps | 5000 (50%) | Challenge bonus for rejectors from submitter stake |
| MaxContestedDeepCount | 3 | Strikes before permanent content rejection |

## Reward Math

### Pools

1. **Submitter stake** — escrowed at submission time (existing)
2. **Reviewer stakes** — `submitterStake × ReviewerStakeRatioBps / 10000` per reviewer, escrowed at commitment
3. **Minority pot** — sum of forfeited minority reviewer stakes

### On ACCEPT (majority votes accept)

```
showUpPool    = submitterStake × ShowUpRewardRatioBps / 10000
acceptReward  = min(submitterStake × AcceptRewardRatioBps / 10000, minorityPot)
remainingPot  = minorityPot - acceptReward

submitter     = submitterStake - showUpPool + acceptReward
majorityEach  = reviewerStake + showUpPool/numMaj + remainingPot/numMaj
minorityEach  = 0 (lost)
protocol      = rounding dust
```

### On REJECT (majority votes reject)

```
showUpPool     = submitterStake × ShowUpRewardRatioBps / 10000
challengeBonus = submitterStake × RejectBonusRatioBps / 10000

submitter      = 0 (lost everything)
majorityEach   = reviewerStake + (showUpPool + challengeBonus)/numMaj + minorityPot/numMaj
minorityEach   = 0 (lost)
protocol       = submitterStake - showUpPool - challengeBonus (remainder)
```

### Deep Contested (no 2/3 supermajority)

- Return ALL stakes (submitter + all reviewers)
- Increment `contested_deep_count[contentHash]`
- At count == MaxContestedDeepCount: permanently reject content hash

## Majority Determination

A reviewer votes "accept" if their `OverallQuality >= BronzeThreshold`. Otherwise "reject". Supermajority = 2/3 of revealed voters.

## Store Keys

| Prefix | Key | Value |
|--------|-----|-------|
| 0xb0 | `roundID + "/" + verifier` | stake amount (string) |
| 0xb1 | `contentHash` | uint64 count |

## Files

| File | Action |
|------|--------|
| `types/params_reviewer.go` | New — param struct, defaults, validation |
| `types/keys.go` | Edit — new prefixes |
| `types/errors.go` | Edit — new errors |
| `keeper/state.go` | Edit — CRUD for reviewer stakes + contested counts |
| `keeper/quality_round.go` | Edit — escrow on commit, distribute on aggregate |
| `keeper/reviewer_staking.go` | New — distribution logic |
| `keeper/reviewer_staking_test.go` | New — table-driven tests |
