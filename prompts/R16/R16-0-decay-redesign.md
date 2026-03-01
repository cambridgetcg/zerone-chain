# R16-0 — Block Reward Decay Redesign

## Problem

The current decay rate is **117× faster than Bitcoin's halving** — rewards collapse from 10 ZRN to 0.1 ZRN (floor) in 78 days. This creates an extreme first-mover advantage where validators joining 3 months after launch earn 100× less than genesis validators. It's a gold rush, not a sustainable economy.

## Bitcoin Reference

Bitcoin's design:
- **Initial reward:** 50 BTC/block
- **Halving:** Every 210,000 blocks (~4 years)
- **Half-life:** 4 years
- **Floor:** 0 (eventually, ~2140)
- **50% of supply minted:** ~4 years
- **Result:** Distinct "eras" where participation at any point in the first decade was meaningful

Bitcoin's 4-year cadence works because it:
1. Gives early adopters an advantage **without excluding late joiners**
2. Creates predictable long-term schedules that institutions can plan around
3. Allows the fee economy time to develop before block rewards become negligible

## Zerone Is Not Bitcoin

Key differences that affect decay design:
- Zerone has a **permanent floor reward** (0.1 ZRN) — Bitcoin goes to 0
- Zerone mints only for **blocks with PoT activity** — empty blocks earn nothing
- Zerone has **no burn** — supply monotonically increases toward cap
- Zerone's cap (222,222,222) is **~10.6× Bitcoin's** (21,000,000) relative to base unit scale
- Zerone uses **continuous exponential decay** — Bitcoin uses step-function halvings
- Zerone needs to attract **knowledge contributors**, not just hardware operators

## Analysis: Half-Life Options

All options keep `blocks_per_reward_epoch = 100,000` (~2.9 days per epoch, ~125 epochs/year).

| Half-Life | `decay_bps` | Year 1 Reward | Year 2 Reward | Year 5 Reward | Year 1 Total | Floor Reached | % Cap Year 5 |
|-----------|------------|---------------|---------------|---------------|-------------|--------------|-------------|
| **12 days (current)** | 850,000 | 0.0 ZRN | 0.1 ZRN | 0.1 ZRN | ~8M | Day 78 | 3.9% |
| **6 months** | 988,987 | 2.5 ZRN | 0.63 ZRN | 0.01 ZRN | 68M (30.6%) | Year 3.3 | 40.8% |
| **1 year** | 994,478 | 5.0 ZRN | 2.5 ZRN | 0.31 ZRN | 90M (40.7%) | Year 6.6 | 78.9% |
| **2 years** | 997,235 | 7.07 ZRN | 5.0 ZRN | 1.77 ZRN | 106M (47.6%) | Year 13.3 | cap binds |
| **4 years (Bitcoin)** | 998,617 | 8.41 ZRN | 7.07 ZRN | 4.20 ZRN | 115M (51.7%) | Year 26.6 | cap binds |

## Recommendation: 1-Year Half-Life

**`reward_decay_bps = 994,478`** (0.994478× per epoch)

### Why 1 Year

1. **Bootstrap without excluding.** 40.7% of cap minted in year 1 — enough to fund the research treasury (3.33%), development fund (19.67%), and establish a validator economy. But someone joining at year 2 still earns 2.5 ZRN/block — half the genesis rate, not 1/100th.

2. **Predictable cadence.** "Rewards halve every year" is simple to communicate. Bitcoin's "every 4 years" created cultural events (halvings). Zerone's annual halving is faster but still on a human timescale.

3. **Floor transition is gradual.** The floor (0.1 ZRN) kicks in around year 6.6 — giving ~6.5 years for the fee-based economy to develop. The current design gives 78 days.

4. **Cap never binds from minting alone.** At floor rate (~1.26M ZRN/year), reaching 222M from the year-5 level (~175M) would take ~37 more years. The economy has decades to mature.

5. **4× faster than Bitcoin, 30× slower than current.** Faster than Bitcoin because Zerone's floor provides a permanent safety net that Bitcoin doesn't have. Slower than current because 78 days is absurd.

### Emission Schedule

| Period | Per-Block Reward | Annual Emission | Cumulative | % of Cap |
|--------|-----------------|----------------|------------|----------|
| Year 0 start | 10.0 ZRN | — | — | — |
| Year 1 end | 5.0 ZRN | ~90M | 90M | 40.7% |
| Year 2 end | 2.5 ZRN | ~45M | 136M | 61.1% |
| Year 3 end | 1.25 ZRN | ~23M | 158M | 71.2% |
| Year 4 end | 0.625 ZRN | ~11M | 169M | 76.1% |
| Year 5 end | 0.31 ZRN | ~6M | 175M | 78.9% |
| Year 6.6 | 0.1 ZRN (floor) | ~1.26M | ~180M | ~81% |
| Year 10+ | 0.1 ZRN (floor) | ~1.26M | ~185M+ | ~83%+ |

### Comparison to Current

| Metric | Current | Proposed | Bitcoin |
|--------|---------|----------|--------|
| Half-life | 12.4 days | 1 year | 4 years |
| Year 1 total emission | 8M | 90M | — |
| Floor reached | Day 78 | Year 6.6 | Never (goes to 0) |
| Ratio of year-1 to year-2 reward | 100:1 | 2:1 | 1:1 |
| Sustainable? | No | Yes | Yes |

## Changes Required

### Parameter Changes

```
reward_decay_bps:        850000 → 994478
```

That's it — one parameter. The epoch length, block reward, and floor reward stay the same.

### Files to Update

1. **`x/vesting_rewards/types/genesis.go`** — `DefaultParams()`:
   ```go
   RewardDecayBps: 994478,  // ~1-year half-life (was 850000)
   ```

2. **`scripts/testnet-genesis-config.json`**:
   ```json
   "reward_decay_bps": 994478,
   "reward_decay_bps_note": "1-year half-life: rewards halve annually"
   ```
   Also update the `blocks_per_reward_epoch` testnet override — consider keeping it at 100,000 for testnet too (was 50,000).

3. **`scripts/testnet-genesis.sh`** — jq patch:
   ```
   .app_state.vesting_rewards.params.reward_decay_bps = 994478
   ```

4. **`scripts/genesis-ceremony.sh`** — same patch.

5. **`scripts/localnet.sh`** — same patch.

6. **`docs/PARAMETERS.md`** — vesting_rewards section:
   - `reward_decay_bps`: 994,478 (1-year half-life)
   - Add note: "Rewards halve approximately every year, reaching floor at ~year 6.6"

7. **`docs/tokenomics/SUPPLY.md`** — Update:
   - Decay curve table with new epoch-by-epoch numbers
   - Long-term projections
   - Comparison table

8. **`docs/tokenomics/STAKING.md`** — Update validator economics section with new reward rates.

9. **`docs/tokenomics/REVIEW.md`** — Update/resolve the "decay is very aggressive" risk item.

10. **Tests** — `x/vesting_rewards/keeper/keeper_test.go`:
    - `TestBlockReward_Epoch1`: expected reward changes (was 8.5 ZRN, now ~9.94 ZRN)
    - `TestBlockReward_Epoch10`: expected reward changes significantly
    - `TestApplyDecay`: update expected values
    - Any hardcoded `850000` decay BPS in test fixtures

### Verification

```bash
# Only one occurrence of old decay value should remain (in migration/history comments)
grep -rn "850000" x/vesting_rewards/ --include="*.go" | grep -v pb.go | grep -v "migration\|history\|changelog"
# Should be 0

# New value present
grep -rn "994478" x/vesting_rewards/types/genesis.go
# Should find DefaultParams

# All tests pass with new decay
go test ./x/vesting_rewards/... -v -run "Decay\|Epoch\|BlockReward"
```

## Commit

```
R16-0: reduce reward decay to 1-year half-life

reward_decay_bps: 850000 → 994478 (0.994478× per 100K-block epoch)

Old: 12.4-day half-life, floor in 78 days, 117× faster than Bitcoin
New: 1-year half-life, floor in year 6.6, 4× faster than Bitcoin

Emission: ~90M ZRN year 1 (40.7% of cap), ~136M year 2, floor at ~180M.
Validators joining at year 2 earn 2.5 ZRN/block (not 0.01 ZRN).
Cap (222M) never binds from minting alone — decades of headroom.
```

## Note on Testnet

Consider using a faster decay on testnet (e.g., 6-month half-life, `decay_bps = 988987`) to observe the full emission curve in compressed time. Add a `[TESTNET]` annotation in the genesis config:

```json
"reward_decay_bps": 988987,
"reward_decay_bps_[TESTNET]": "prod=994478 (1-year HL), testnet uses 6-month HL for faster observation"
```
