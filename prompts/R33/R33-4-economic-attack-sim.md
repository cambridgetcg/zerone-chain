# R33-4 — Economic Attack Simulations

## Objective

Model and simulate economic attacks against ZERONE's token economics, reward distribution, and liquidity mechanisms. Verify that no rational adversary can extract outsized value.

## Tasks

### 1. Reward gaming simulation

- Model an adversary with 10% of stake who optimally times claim submissions
- Compare their reward rate vs. honest participants
- Verify advantage is bounded (< 5% premium)
- Test with varying adversary budgets: 5%, 10%, 25%, 50% of stake

### 2. MEV extraction potential

- Model a validator who reorders transactions within a block
- Identify which ZERONE-specific transactions could benefit from ordering
  - Claim submission (front-running a verification round)
  - Reveal submission (seeing other reveals before submitting)
- Verify commit-reveal scheme prevents front-running
- Verify no profitable reordering exists for verification transactions

### 3. Fee manipulation

- Submit transactions with varying gas prices
- Verify fee distribution is correct regardless of gas price
- Test with zero-fee transactions (if allowed by mempool config)
- Verify fee floor prevents fee-less spam

### 4. Liquidity pool attacks (if applicable)

- Model sandwich attacks on liquidity pool operations
- Model flash-loan equivalent attacks (instant borrow/use/repay)
- Verify pool invariants hold under all tested scenarios

### 5. Inflation / deflation edge cases

- Run chain for 10,000 blocks
- Verify total supply follows expected curve
- Check for rounding errors in revenue split accumulation
- Verify no dust accumulation in module accounts
- Test with extreme parameters (max mint, min mint, zero mint)

### 6. Research fund isolation

- Verify research fund multisig cannot be drained by single party
- Verify 2-of-2 requirement holds under simulation
- Test failed multisig attempts (only 1 signature)

## Acceptance Criteria

- [ ] No reward gaming strategy yields > 5% premium over honest behavior
- [ ] Commit-reveal prevents transaction ordering attacks
- [ ] Fee distribution correct at all gas price levels
- [ ] Supply curve matches expected inflation schedule over 10K blocks
- [ ] No rounding dust accumulates in module accounts
- [ ] Research fund multisig enforced under all conditions
