# R14-2: BVM Test Porting Design

## Summary

Port ~40 BVM tests from `legible-money` prototype to `zerone`, organized into category-based files. Excludes auth/DID/session-capability-dependent tests (zerone lacks that bridge).

## Source → Target Mapping

- Module path: `github.com/zerone-chain/zerone/x/bvm`
- Denom: `uzrn` (not `ulgm`)
- Address prefix: `zrn` (not `lgm`)
- BPS scale: 1,000,000

## File Structure

### New: `x/bvm/keeper/gas_bridge_test.go` (~12 tests)

Gas metering, Cosmos ↔ BVM gas bridging, OOG recovery, gas caps.

| Test | Source |
|------|--------|
| TestGasBridge_SimpleContract | integration_test.go |
| TestGasBridge_OutOfGas_SDKLimit | integration_test.go |
| TestGasBridge_OutOfGas_VMLimit | integration_test.go |
| TestGasBridge_NestedCalls | integration_test.go |
| TestGasBridge_SSTOREHeavyContract | integration_test.go |
| TestGasBridge_PanicChargesFullGas | integration_test.go |
| TestGasBridge_CallContract | keeper_test.go (P0_1) |
| TestGasBridge_OutOfGas_Reverts | keeper_test.go (P0_1) |
| TestGasLimitCap_CallContract | keeper_test.go (P1_1) |
| TestGasLimitCap_ScheduleContract | keeper_test.go (P1_1) |
| TestEffectiveGasCap | keeper_test.go (P0_2) |

### New: `x/bvm/keeper/security_test.go` (~15 tests)

Adversarial probes + panic recovery. Function names are self-documenting; OCBVM audit IDs in comments only.

| Test | Source (audit ref) |
|------|--------|
| TestSecurity_CrossContractStateIsolation | openclaw (OCBVM5) |
| TestSecurity_ValueRefundOnRevert | openclaw (OCBVM6) |
| TestSecurity_ValueTransfer_SuccessfulCallRetains | openclaw (OCBVM7) |
| TestSecurity_ScheduleCancelAuth | openclaw (OCBVM8) |
| TestSecurity_GovernanceStateUpdateAuth | openclaw (OCBVM9) |
| TestSecurity_CodeDedup_RefCountLifecycle | openclaw (OCBVM10) |
| TestSecurity_ScheduleLimitEnforcement | openclaw (OCBVM11) |
| TestSecurity_BytecodeSizeLimit | openclaw (OCBVM12) |
| TestSecurity_ScheduleCreatorOnly | openclaw (OCBVM13) |
| TestSecurity_GasExhaustion_ScheduledExecution | openclaw (OCBVM14) |
| TestSecurity_DeterministicContractAddresses | openclaw (OCBVM15) |
| TestSecurity_OverdueScheduleExecution | openclaw (OCBVM16) |
| TestSecurity_InsufficientFundsPayableCall | openclaw (OCBVM18) |
| TestPanicRecovery_INVALID_Opcode | integration_test.go |
| TestPanicRecovery_StackOverflow | integration_test.go |
| TestPanicRecovery_MultipleSchedules_OnePanics | integration_test.go |

### New: `x/bvm/keeper/schedule_test.go` (~8 tests)

Schedule edge cases and per-block budget enforcement.

| Test | Source |
|------|--------|
| TestScheduleContract_ContractNotFound | keeper_test.go |
| TestCancelSchedule_AlreadyExecuted | keeper_test.go |
| TestGetPendingSchedules_IncludesOverdue | keeper_test.go |
| TestScheduleBudget_SingleSchedule_FitsInBudget | integration_test.go |
| TestScheduleBudget_MultipleSchedules_ExceedsBudget | integration_test.go |
| TestScheduleBudget_DeferredScheduleRunsNextBlock | integration_test.go |
| TestScheduleBudget_PanicInSchedule_DoesntBlockOthers | integration_test.go |
| TestScheduledGasBudget | keeper_test.go (P0_2) |

### Additions to existing `keeper_test.go` (~5 tests)

| Test | Source |
|------|--------|
| TestQueryCode | keeper_test.go |
| TestQuerySchedule | keeper_test.go |
| TestGenesisRoundtrip_IncludesState | keeper_test.go |
| TestDeleteContract_DecrRefCount | keeper_test.go |
| TestCallContract_ZeroValueNoTransfer | keeper_test.go |

## Excluded (auth-dependent, ~16 tests)

CallerDID_*, SessionCaps_*, OCBVM1-4, OCBVM17, ScheduleCapability_*, P1_2_ScheduleCapability_*, P1_2_ScheduleExecution_*

These require auth/DID bridge work (R15 territory).

## Shared Infrastructure

All new files reuse from `keeper_test.go` (same package):
- `setupKeeper()`, `setupMsgServer()`
- Bytecode helpers: `simpleBytecode()`, `returnBytecode()`, `sstoreBytecode()`, `revertBytecode()`, `infiniteLoopBytecode()`, `invalidBytecode()`
- `deployContract()`, `callContract()` action helpers
- Mock keepers: `mockBankKeeper`, `mockKnowledgeKeeper`

## Verification

```bash
go test ./x/bvm/... -count=1 -v
go vet ./x/bvm/...
grep -c "func Test" x/bvm/keeper/*_test.go
```

## Impact

| Metric | Before | After |
|--------|--------|-------|
| Test files | 1 | 4 |
| Test functions | 73 | ~113 |
| Categories | 17 | 20 |
