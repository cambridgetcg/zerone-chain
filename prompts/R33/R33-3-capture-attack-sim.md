# R33-3 — Capture & Sybil Attack Simulations

## Objective

Simulate realistic adversarial scenarios against ZERONE's defense modules. Verify that capture_defense, qualification, and alignment detect and respond to coordinated attacks.

## Tasks

### 1. Sybil attack — verification flooding

- Create 50 sybil accounts with minimal stake
- Have all 50 submit commitments to the same claim
- Verify capture_defense flags the domain
- Verify carrying capacity reduction kicks in
- Verify qualification gates block low-quality verifiers

### 2. Cartel capture — domain monopoly

- Create 3 accounts that control 90% of a domain's verification
- Run 100 blocks of normal operation
- Verify HerfindahlIndex rises above flagging threshold
- Verify alignment sensor detects capture risk
- Verify structural immunity from partnerships provides some resistance

### 3. Collusion — coordinated false verification

- Create a group of 5 verifiers who always vote together
- Submit a false claim
- Have all 5 commit and reveal "accept"
- Verify the claim passes (this is expected — 5/5 majority)
- Submit a dissent from an independent verifier
- Verify vindication window opens
- Run additional verification rounds
- Verify the system eventually self-corrects

### 4. Knowledge spam — domain flooding

- Submit 1000 claims to a single domain in 10 blocks
- Verify domain pressure exceeds capacity
- Verify overcrowding decay accelerates
- Verify birth pressure reduces initial energy for new facts
- Verify alignment growth pressure event fires
- Verify governance expedited voting activates

### 5. Governance attack — param manipulation

- Create a validator with 40% stake
- Submit a param change that would break economic invariants
- Vote yes with 40% stake
- Verify proposal fails (needs >50% or >66% depending on type)
- Try with 67% stake
- Verify param validation rejects out-of-bounds values even with supermajority

### 6. Economic attack — reward gaming

- Stake/unstake rapidly to maximize reward capture
- Verify no reward amplification from timing attacks
- Verify unbonding period prevents rapid cycling

## Acceptance Criteria

- [ ] Sybil attack detected within 20 blocks
- [ ] Domain monopoly flagged when HHI exceeds threshold
- [ ] Collusion detected via dissent → vindication flow
- [ ] Knowledge spam triggers all R31 pressure mechanisms
- [ ] Governance param validation prevents destructive changes
- [ ] No economic amplification from stake cycling
