# R32-5 — Economic Flow E2E

## Objective

Test the complete economic flow: block rewards → vesting → revenue split → research fund → staking rewards. Verify that ZRN flows correctly through all economic modules on a real chain.

## Tasks

### 1. Block reward distribution

- Start chain, wait 50 blocks
- Query vesting_rewards module account balance
- Verify rewards accumulating per block
- Check revenue split: 55% contributors, 22% protocol, 19.67% dev fund, 3.33% research

### 2. Staking reward flow

- Delegate tokens to validator
- Wait for reward accumulation (10+ blocks)
- Query delegation rewards
- Withdraw rewards
- Verify balance increase matches expected rewards

### 3. Research fund accumulation

- Verify research fund address receives 3.33% of block rewards
- Check multisig address balance increases over time
- Verify founder share (7% of research = 0.23% total) goes to correct address

### 4. Fee distribution

- Submit several transactions with fees
- Verify fees distributed to validators proportional to stake
- Test with multiple validators (2+)

### 5. Zero-supply genesis verification

- Start from genesis with zero ZRN supply
- Verify first block mints tokens via PoT
- Track supply growth over 100 blocks
- Verify total supply matches expected mint schedule

### 6. Vesting schedule

- Create vesting account
- Verify locked tokens cannot be transferred
- Wait for vesting cliff
- Verify partial unlock
- Verify full unlock after vesting period

## Acceptance Criteria

- [ ] Revenue split percentages match within 0.01% tolerance
- [ ] Research fund multisig receives correct share
- [ ] Staking rewards withdrawable and correct
- [ ] Fee distribution proportional to stake
- [ ] Zero-supply genesis → PoT minting works
- [ ] Vesting schedules enforce lockup correctly
