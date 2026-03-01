# R32-4 — Governance & Emergency E2E

## Objective

Test governance proposal lifecycle, LIP submission, parameter changes, emergency halt ceremony, and formation freeze — all on a running chain with real voting.

## Tasks

### 1. Standard proposal lifecycle

- Submit text proposal
- Deposit minimum
- Vote (yes from validator)
- Wait for voting period end
- Verify proposal passes and executes

### 2. Parameter change proposal

- Submit param change for `knowledge.DomainBaseCapacity`
- Vote and pass
- Query params after execution — verify new value
- Submit a second proposal with out-of-bounds value — verify rejection

### 3. LIP submission and execution

- Submit a LIP (Legible Improvement Proposal)
- Attach upgrade plan
- Vote and pass
- Verify upgrade plan scheduled at target height
- (Don't actually upgrade — just verify the plan is registered)

### 4. Emergency halt ceremony

- Submit emergency halt proposal
- Advance ceremony stages
- Verify chain halts (or enters halt state)
- Resume chain
- Verify normal operation continues

### 5. Domain formation freeze (R31-3)

- Submit MsgDomainFormationFreeze via governance
- Verify partnership formation blocked for target domain
- Wait for freeze expiry
- Verify formation unblocked

### 6. Expedited voting under knowledge pressure (R31-1)

- Create high knowledge growth pressure (many pending claims)
- Submit a knowledge-related LIP
- Verify voting period is shortened
- Pass the proposal in the expedited window

## Acceptance Criteria

- [ ] Standard proposal lifecycle: submit → deposit → vote → execute
- [ ] Param changes take effect on-chain after proposal passes
- [ ] Emergency halt ceremony completes without panics
- [ ] Formation freeze blocks and unblocks correctly
- [ ] Expedited voting activates under growth pressure
- [ ] Rejected proposals (insufficient votes, out-of-bounds params) fail gracefully
