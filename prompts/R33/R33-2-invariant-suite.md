# R33-2 — Module Invariant Suite

## Objective

Define and implement invariants for every custom module that are checked on every block during simulation. Invariants are the mathematical properties that must ALWAYS hold — if they break, the chain has a critical bug.

## Tasks

### 1. Economic invariants

**Supply conservation:**
- Total supply = sum of all account balances + module accounts + locked + vesting
- No ZRN created or destroyed outside of mint/burn operations
- Research fund balance ≤ cumulative 3.33% of all block rewards

**Staking consistency:**
- Sum of all delegations = bonded pool balance
- Sum of all unbonding delegations = unbonding pool balance
- No delegation references a non-existent validator

**Fee distribution:**
- Accumulated commission ≤ total fees collected
- No negative outstanding rewards

### 2. Knowledge invariants

**Graph acyclicity:**
- Citation graph has no cycles (fact A cites B cites A)
- No fact cites itself

**Domain consistency:**
- Every fact belongs to exactly one domain
- Domain active count = actual count of active facts in domain
- Domain at-risk count = actual count of at-risk facts in domain

**Verification integrity:**
- Every completed round has commitments from ≥ MinVerifiers
- Every reveal matches its commitment hash
- No claim in ACCEPTED status without a completed round

**Carrying capacity:**
- `GetDomainCarryingCapacity()` ≥ 1 for every domain (never zero)
- Pressure = population / capacity (basic math check)

### 3. Governance invariants

**Proposal consistency:**
- Every proposal in voting period has deposit ≥ MinDeposit
- No proposal with status PASSED that wasn't voted on
- Vote tally = sum of individual votes

**Emergency halt:**
- Emergency halt requires correct ceremony completion
- No halt without quorum

### 4. Partnership invariants

**Formation consistency:**
- Every active partnership has exactly 2 members
- Both members exist as registered accounts
- No duplicate partnerships (same pair)

**Exit integrity:**
- Exit-initiated partnerships have exit_height set
- No completed exit with remaining locked funds

### 5. Defense invariants

**Capture metrics:**
- HerfindahlIndex ∈ [0, 1_000_000]
- RiskScore ∈ [0, 1_000_000]
- Flagged domains have HHI above threshold

**Structural immunity:**
- Immunity adjustment never makes HHI negative

### 6. Alignment invariants

**Sensor bounds:**
- All sensor readings ∈ [0, 1_000_000] (BPS range)
- Composite score is weighted average of sensor readings
- No sensor references a non-existent module state

### 7. Registration

Register all invariants in each module's `module.go`:

```go
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    RegisterInvariants(ir, am.keeper)
}
```

Create `x/<module>/keeper/invariants.go` for each module.

## Acceptance Criteria

- [ ] Every custom module has at least one invariant registered
- [ ] Economic invariants cover supply conservation and staking consistency
- [ ] Knowledge invariants cover graph acyclicity and domain counts
- [ ] All invariants pass on a 500-block simulation run
- [ ] Invariant violations produce diagnostic output (which accounts, what values)
- [ ] `zeroned invariant-check` CLI command works (checks all invariants on current state)
