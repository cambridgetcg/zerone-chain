# R15-2 — Port Auth-Dependent BVM Tests

## Context

R14-2 skipped ~22 tests that depend on the auth/DID → BVM bridge. R15-1 wires that bridge. Now port the tests.

## Prerequisites

R15-1 must be merged — authKeeper wired into BVM keeper, CallerDID resolution working, SessionCapabilities in execution context, capability inheritance for schedules.

## Task

Port these tests from `legible-money/x/bvm/keeper/` to zerone, adapted for zerone naming (uzrn, zrn prefix, proto types, 1M BPS).

### File: `x/bvm/keeper/auth_integration_test.go` (new)

#### CallerDID Tests (~4 tests)

```
TestCallContract_CallerDID_Resolved
  — Register account with DID, call BVM contract, verify CallerDID populated in execution context

TestCallContract_CallerDID_NilAuth
  — Call with no authKeeper set, verify CallerDID is empty string (not panic)

TestCallContract_CallerDID_UnknownAddress
  — Call from address with no DID mapping, verify CallerDID is empty

TestCallContract_CallerDID_PassedToHostFunctions
  — Verify KVerify and KCite receive the resolved CallerDID
```

#### SessionCapability Tests (~4 tests)

```
TestCallContract_SessionCaps_StakeBlocked
  — Session key with CanStake=false, verify staking opcode denied

TestCallContract_SessionCaps_FullAccess
  — Session key with all capabilities true, verify all agent opcodes work

TestCallContract_SessionCaps_AnonymousDenied
  — No DID (anonymous caller), verify all agent opcodes denied (C-1 secure default)

TestCallContract_SessionCaps_IdentityKeyFullAccess
  — Identity key (DID exists, no session key), verify full capabilities granted
```

### File: `x/bvm/keeper/schedule_capability_test.go` (new)

#### Schedule Capability Inheritance (~7 tests)

```
TestScheduleCapability_RestrictedKey_RestrictedExecution
  — Schedule created by session key with CanTransfer=false, scheduled execution inherits restriction

TestScheduleCapability_FullKey_FullExecution
  — Schedule created by full-access key, scheduled execution has all capabilities

TestScheduleCapability_CapabilityExpires
  — Session key expires between schedule creation and execution, verify execution denied

TestP1_2_ScheduleCapabilityStorage
  — Verify capabilities are stored with the schedule, not looked up at execution time

TestP1_2_ScheduleCapability_FullAccess
  — Full-access schedule capability storage + retrieval

TestP1_2_ScheduleExecution_UsesStoredCaps
  — Verify schedule uses stored capabilities, not current caller capabilities

TestP1_2_ScheduleExecution_BackwardsCompat
  — Schedules created before capability system get full access (backwards compatibility)
```

#### OpenClaw Security Tests (~7 tests)

```
TestOCBVM1_SessionCapability_StakeBlocked
  — Adversarial: session key tries staking opcode with CanStake=false

TestOCBVM2_AnonymousCaller_AllAgentOpsDenied
  — Adversarial: no DID, verify all agent opcodes (KVERIFY, KCITE, etc.) denied

TestOCBVM3_IdentityKeyAuth_FullCapabilities
  — Identity key holder gets full BVM capabilities

TestOCBVM4_ScheduledExecution_FullCapabilities
  — Scheduled execution by identity key holder has full capabilities

TestOCBVM17_SessionKeyExpiry
  — Session key expires, subsequent BVM calls denied

TestOCBVM10_CodeDedup_RefCountLifecycle (already ported in R14-2 — verify)
  — If already ported, skip. If not, port here.

TestScheduleCapability_RevokedKey_ExecutionDenied
  — Session key revoked after schedule created, verify execution still uses stored caps (or denies — match prototype behavior)
```

## Approach

- Read R15-1's implementation first — understand how CallerDID and SessionCapabilities flow through
- Set up test helper that creates auth keeper with DID mappings and session keys
- Each test should be self-contained: create account → register DID → optionally create session key → deploy contract → call → assert
- Match the prototype's test assertions exactly — these are security-critical

## Verification

```bash
go test ./x/bvm/... -count=1 -v
go vet ./x/bvm/...
# Count auth-dependent tests
grep -c "func Test" x/bvm/keeper/auth_integration_test.go x/bvm/keeper/schedule_capability_test.go
```

Target: 22 new test functions, all passing.

## Commit Convention

```
test(R15-2): port CallerDID + SessionCapability BVM tests
test(R15-2): port schedule capability inheritance tests
test(R15-2): port OCBVM auth security tests (1-4, 17)
```
