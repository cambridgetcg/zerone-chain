# R6-6 — Security Module Tests

## Goal

Write comprehensive tests for all R6 security modules. Focus on adversarial
scenarios and edge cases — these are security-critical modules.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/emergency/keeper/*_test.go` — 1465 LOC
- `/Users/yuai/Desktop/legible_money/x/disputes/keeper/*_test.go` — 2718 LOC
- `/Users/yuai/Desktop/legible_money/x/capture_challenge/keeper/*_test.go` — 988 LOC
- `/Users/yuai/Desktop/legible_money/x/capture_defense/keeper/*_test.go` — 1703 LOC
- `/Users/yuai/Desktop/legible_money/x/qualification/keeper/*_test.go` — 1476 LOC

**Depends on R6-1 through R6-5.**

## Test Categories

### 1. Emergency Module
- Full halt ceremony: propose → 75% vote → halted
- Revert ceremony (80% quorum) while halted
- Resume ceremony (80% quorum) restores normal
- Non-guardian proposer → rejected
- Below-quorum vote → ceremony times out
- Anti-abuse: max proposals per epoch exceeded → rejected
- Cooldown between proposals enforced
- Min distinct voters enforcement
- Genesis council can propose during bootstrap
- Council expires after council_expiry_block
- Auto-resume after max_halt_duration_blocks
- Double vote by same voter → ignored/replaced
- Propose revert when NOT halted → rejected
- Max revert depth enforcement

### 2. Disputes Module
- Full lifecycle: initiate → commit → reveal → vote → challenger wins
- Full lifecycle: initiate → commit → reveal → vote → defender wins
- Draw outcome when votes split evenly
- Timeout when no quorum → bonds returned
- Commit/reveal: hash mismatch → reveal rejected
- Commit/reveal: reveal without commit → rejected
- Commit/reveal: reveal after deadline → rejected
- Escalation: tier 1 → tier 2, more arbiters, higher bond
- Escalation: insufficient additional bond → rejected
- Bond slashing: 50% to winner, 10% to arbiters
- Arbiter not in selected list → vote rejected
- Challenger is also arbiter → excluded from selection
- Max active disputes enforcement
- Dispute against non-existent fact → rejected

### 3. Capture Challenge
- Submit challenge → domain paused → evidence added → upheld → slash accused
- Submit challenge → resolved rejected → challenger slashed
- Bounty pool funding from fact fees
- Min evidence pieces enforcement
- Evidence after deadline → rejected
- Risk analysis: Herfindahl index calculation with known inputs
- Partial resolution: some accused slashed, some not

### 4. Capture Defense
- Record verification → global + stratum + domain reputation updated
- Reputation decay over time (BeginBlocker)
- Herfindahl concentration index: 1 validator = 1M, 4 equal = 250k
- Timing correlation detection
- Cross-stratum requirements enforcement
- Accuracy tracking: correct/incorrect → accuracy_bps updated
- Consecutive incorrect → score drops

### 5. Qualification
- Stake commitment pathway: lock stake → qualified → unlock after period
- Track record pathway: sufficient verifications + accuracy → qualified
- Cross-reference pathway: 3 endorsements → qualified
- Cross-reference: only 2 endorsements → rejected
- Stratum inheritance: parent domain qualification → child at discount
- Status transitions: active → probationary (5 consecutive incorrect)
- Probationary → active (10 consecutive correct)
- Revocation: 3 lost challenges → revoked
- Expiry in BeginBlocker
- Renewal extends expiry
- Max qualifications per validator enforcement
- Endorsement stake lock and return

### 6. IBC Rate Limiting
- Send within limit → passes
- Send exceeding limit → blocked
- Window reset → fresh quota
- Timeout refunds send quota
- Error ack refunds send quota
- Receive within limit → passes
- Receive exceeding limit → error ack
- Wildcard rate limits (channel="*", denom="*")
- Disabled module → all traffic passes

### 7. ICA Auth
- Register account happy path
- Submit tx with allowed message → passes
- Submit tx with disallowed message → rejected
- Max messages per tx enforcement
- Disabled module → submissions rejected

### 8. Adversarial Scenarios (Cross-Module)
- **Emergency abuse:** Colluding guardians spam halt proposals → max_proposals_per_epoch blocks it
- **Dispute gaming:** Initiate dispute, refuse to reveal evidence → timeout, bond returned (not slashed)
- **Qualification farming:** Create many endorsement accounts → min_endorser_stake makes it expensive
- **Capture evasion:** Validator uses multiple addresses → Herfindahl catches concentration
- **IBC drain:** Attempt to drain chain via rapid IBC sends → rate limit blocks it

## Test Utilities

Reuse test patterns from existing modules (e.g., x/channels/keeper/keeper_test.go).
Create shared test helpers where needed for mock keepers.

## Conventions
- Token: uzrn. Module path: github.com/zerone-chain/zerone
- BPS: 1,000,000 scale
- Run `go build ./...` and `go test ./x/emergency/... ./x/disputes/... ./x/capture_challenge/... ./x/capture_defense/... ./x/qualification/... ./x/ibc_ratelimit/... ./x/icaauth/...` before finishing
- Target: ALL tests passing, zero failures
