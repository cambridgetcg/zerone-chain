# R10-5 — Security Audit Pass

## Goal

Systematic security review of the entire codebase. Re-run all audit checklists from the
draft, verify every P0/P1 fix was properly ported, and identify any new vulnerabilities
introduced during the clean rewrite.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- Draft audit findings from B14 (security audit) and B22-3 (audit fixes)
- `/Users/yuai/Desktop/legible_money/` — original audit findings in test files

## Audit Checklists

### 1. Ante Handler Security
- [ ] Frozen account check covers ALL message types (not just hand-written)
- [ ] Signer extraction uses `getSignerAddresses()` from tx sigs (P0-1 fix)
- [ ] Session key capabilities enforced for ALL messages
- [ ] Gas overflow protection (saturating addition, P1-2 fix)
- [ ] Emergency halt blocks all non-emergency messages
- [ ] Bootstrap gas-free only applies to whitelisted message types
- [ ] Fee router validates denomination
- [ ] No bypass possible via multi-message transactions

### 2. Knowledge Module Security
- [ ] Commit/reveal: commitment cannot be front-run
- [ ] VRF selection: proposer cannot choose favorable verifiers
- [ ] Slashing: misbehaving verifiers lose stake
- [ ] Confidence scoring: cannot be manipulated by single verifier
- [ ] Fact deduplication: no duplicate facts with different IDs
- [ ] Domain validation: claims reference valid domains

### 3. Economic Security
- [ ] Revenue splits sum to exactly 1M BPS at every junction
- [ ] No token creation except through authorized mint (vesting rewards, tokens module)
- [ ] No token destruction except through authorized burn
- [ ] Module accounts cannot go negative
- [ ] Research fund cannot be overdrawn
- [ ] Fee collection and routing is atomic (no partial fee states)

### 4. Governance Security
- [ ] Only designated voters can vote on research fund disbursements
- [ ] 2-of-2 quorum strictly enforced (no way to bypass)
- [ ] Proposal expiry works correctly
- [ ] Double-voting prevented
- [ ] Governance parameter changes have bounds validation
- [ ] Sybil vote decay works (funding tracker)

### 5. Staking Security
- [ ] Tier transitions have cooldown periods
- [ ] Slash parameters are non-zero
- [ ] Delegation/undelegation accounting is correct
- [ ] Validator jailing works for missed blocks
- [ ] No stake inflation through rapid delegate/undelegate cycles

### 6. IBC Security
- [ ] Rate limiting enforced on all channels
- [ ] Rate limit windows reset correctly
- [ ] ICA auth validates interchain account ownership
- [ ] No token duplication through IBC round-trip

### 7. Module Account Permissions
- [ ] Each module account has correct permissions (mint, burn, staking)
- [ ] No module account has excessive permissions
- [ ] Module accounts cannot be controlled by external transactions

### 8. Input Validation
- [ ] All message handlers validate inputs (non-empty strings, valid addresses, positive amounts)
- [ ] Proto fields have appropriate constraints
- [ ] No integer overflow in amount calculations
- [ ] No division by zero possible
- [ ] String lengths bounded (no memory exhaustion via large inputs)

## Process

### Step 1: Automated Checks
```bash
# Static analysis
go vet ./...
staticcheck ./... 2>/dev/null || true

# Check for common vulnerabilities
grep -rn "unsafe\." --include="*.go" . | grep -v vendor | grep -v "_test.go"
grep -rn "math/rand" --include="*.go" . | grep -v vendor | grep -v "_test.go"
grep -rn "panic(" --include="*.go" . | grep -v vendor | grep -v "_test.go" | grep -v "// panic"
```

### Step 2: Manual Review
For each checklist item:
1. Find the relevant code
2. Verify the check/fix exists
3. Write a test if one doesn't exist
4. Mark as pass/fail

### Step 3: Fix Findings
For any issues found:
1. Classify as P0 (critical), P1 (important), P2 (minor)
2. Fix P0s immediately
3. Fix P1s in this session
4. Document P2s for future (create GitHub issues)

## Deliverables

1. `docs/SECURITY-AUDIT.md` — full audit report with findings, fixes, and remaining items
2. Any code fixes for P0/P1 findings
3. New tests for any untested security-critical paths
4. Updated `PARAMETERS.md` with security-relevant parameter bounds

## Constraints

- ALL P0 findings must be fixed before testnet launch
- P1 findings should be fixed; document any deferred with justification
- Audit report must be committed to the repo
- No security-through-obscurity — document the security model clearly
- Every fix must have a corresponding test
