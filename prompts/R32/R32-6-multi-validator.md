# R32-6 — Multi-Validator Network E2E

## Objective

Test ZERONE with a realistic 4-validator network: consensus stability, validator set changes, slashing, and coordinated upgrades.

## Tasks

### 1. 4-validator startup

- Start chain with 4 validators, each with equal stake
- Verify all 4 sign blocks
- Verify consensus with 1 validator down (Byzantine fault tolerance)

### 2. Validator set changes

- Add a 5th validator mid-chain
- Verify new validator enters active set
- Remove a validator (unbond)
- Verify set adjusts and chain continues

### 3. Slashing

- Double-sign simulation (if possible via interchaintest)
- Verify slashing penalty applied
- Verify jailed validator excluded from consensus
- Unjail after jail period
- Verify validator re-enters active set

### 4. Network partition simulation

- Stop 1 of 4 validators
- Verify chain continues (3/4 = 75% > 2/3 threshold)
- Stop 2nd validator
- Verify chain halts (2/4 = 50% < 2/3 threshold)
- Restart validators
- Verify chain resumes and catches up

### 5. Coordinated upgrade

- Submit upgrade proposal on 4-validator network
- All validators vote yes
- Verify upgrade plan scheduled
- (Cosmovisor handles the actual binary swap — test the governance flow)

### 6. State sync verification

- Start 4-validator chain, run for 100 blocks
- Start a new full node with state sync
- Verify full node catches up to current height
- Verify full node can serve queries with correct state

## Acceptance Criteria

- [ ] 4-validator network starts and produces blocks
- [ ] Chain survives 1 validator failure
- [ ] Chain halts at 2 validator failures, recovers on restart
- [ ] Validator addition/removal works mid-chain
- [ ] Slashing mechanics work correctly
- [ ] Upgrade proposal flow works on multi-validator network
