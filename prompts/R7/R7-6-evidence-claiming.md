# R7-6 — Evidence Management + Claiming Pot (Batched)

## Goal

Port two smaller modules in a single session:
1. **x/evidence_mgmt** — on-chain evidence submission, verification, and chain-of-custody tracking
2. **x/claiming_pot** — token claiming mechanism with vesting schedules and eligibility proofs

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/evidence_mgmt/` — 3760 LOC keeper, 2 test files
- `/Users/yuai/Desktop/legible_money/x/claiming_pot/` — 2219 LOC keeper, 2 test files
- `/Users/yuai/Desktop/legible_money/proto/legible/evidence_mgmt/v1/` — 4 protos
- `/Users/yuai/Desktop/legible_money/proto/legible/claiming_pot/v1/` — 4 protos
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`, `LGM` → `ZRN`

---

## Module 1: Evidence Management

### Proto Files

#### `proto/zerone/evidence_mgmt/v1/state.proto`
Port from draft:
- `Evidence` — id, submitter, evidence_type ("document", "observation", "measurement", "testimony"),
  content_hash, metadata, chain_of_custody entries, status, timestamps
- `ChainOfCustodyEntry` — custodian, action, timestamp, notes
- `VerificationResult` — evidence_id, verifier, outcome, confidence, method

#### `proto/zerone/evidence_mgmt/v1/tx.proto`
- MsgSubmitEvidence: submit with content hash + metadata
- MsgTransferCustody: transfer evidence to new custodian
- MsgVerifyEvidence: submit verification result
- MsgChallengeEvidence: challenge evidence validity
- MsgUpdateParams (authority-gated)

#### `proto/zerone/evidence_mgmt/v1/query.proto`
- QueryEvidence (by ID), QueryEvidenceBySubmitter, QueryCustodyChain, QueryVerifications

#### `proto/zerone/evidence_mgmt/v1/genesis.proto`
- GenesisState: params + evidences + verifications + next IDs

### Keeper Implementation
- Submit evidence with content hash (no on-chain content storage)
- Chain of custody tracking (append-only log)
- Verification by qualified verifiers (staking tier check)
- Challenge mechanism triggers dispute module
- Integration: disputes module references evidence, knowledge module uses verified evidence

### Expected Keepers
```go
type StakingKeeper interface {
    GetValidatorTier(ctx sdk.Context, addr sdk.AccAddress) (uint32, error)
}
type DisputesKeeper interface {
    CreateDispute(ctx sdk.Context, ...) error
}
```

### Default Params
- MinVerifierTier: 2
- VerificationQuorum: 3
- ChallengeBond: 500000 uzrn (0.5 ZRN)
- ChallengeWindowBlocks: 50000

---

## Module 2: Claiming Pot

### Proto Files

#### `proto/zerone/claiming_pot/v1/state.proto`
Port from draft:
- `ClaimingPot` — id, name, total_amount, claimed_amount, schedule, eligibility_criteria
- `VestingSchedule` — start_block, end_block, cliff_blocks, period_blocks
- `Claim` — pot_id, claimant, amount, claimed_at
- `EligibilityCriteria` — min_staking_tier, min_registration_age, whitelist

#### `proto/zerone/claiming_pot/v1/tx.proto`
- MsgCreatePot: create claiming pot with funded amount + schedule (authority-gated)
- MsgClaim: claim available tokens from a pot
- MsgUpdatePotParams: update pot parameters (authority-gated)

#### `proto/zerone/claiming_pot/v1/query.proto`
- QueryPot (by ID), QueryAllPots, QueryClaimable (amount available for address), QueryClaims

#### `proto/zerone/claiming_pot/v1/genesis.proto`
- GenesisState: params + pots + claims

### Keeper Implementation
- Create pots funded from module account or governance
- Vesting schedule: linear vesting with cliff, calculate claimable at any block
- Eligibility check: tier, registration age, whitelist
- Claim: verify eligible, calculate vested-but-unclaimed, transfer, record claim
- Anti-double-claim: track total claimed per address per pot

### Expected Keepers
```go
type StakingKeeper interface {
    GetValidatorTier(ctx sdk.Context, addr sdk.AccAddress) (uint32, error)
}
type AuthKeeper interface {
    GetRegistrationBlock(ctx sdk.Context, addr sdk.AccAddress) (uint64, error)
}
type BankKeeper interface {
    SendCoinsFromModuleToAccount(ctx, moduleName, addr, coins) error
}
```

### Default Params
- MaxPotsActive: 10
- MinClaimAmount: 1000 uzrn

---

## Tests (Both Modules)

### Evidence Management
1. Submit evidence, verify storage and custody chain
2. Transfer custody appends entry
3. Verification by qualified verifier
4. Verification rejected for low-tier verifier
5. Challenge creates dispute
6. Genesis round-trip

### Claiming Pot
1. Create pot with vesting schedule
2. Claim before cliff returns 0
3. Claim after cliff returns correct vested amount
4. Claim after full vesting returns remaining
5. Ineligible claimant rejected
6. Double-claim prevented (only unclaimed portion)
7. Genesis round-trip

## Constraints

- Proto-first, gogoproto generation for both modules
- 1M BPS scale for any percentage fields
- Both modules register in app.go
- Evidence module has no EndBlocker (event-driven)
- Claiming pot may need EndBlocker for pot expiry (check draft)
- Keep both modules small and focused — they're utility modules
