# R14-2: BVM Test Porting Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Port ~33 BVM tests from the legible-money prototype to zerone, reaching 106 total test functions.

**Architecture:** Three new category-based test files + small additions to existing keeper_test.go. All files share the existing test harness (setupKeeper, setupMsgServer, bytecode helpers) from keeper_test.go via same-package access.

**Tech Stack:** Go testing, Cosmos SDK (storetypes for gas meters, cmtproto for block headers), zerone BVM keeper/types/vm packages.

---

## Key Adaptations (Prototype → Zerone)

| Aspect | Prototype | Zerone |
|--------|-----------|--------|
| Denom | `ulgm` | `uzrn` |
| Address prefix | `lgm1contract` | `zrn1contract` |
| Test addresses | `creator1`, `creator2`, `caller1` | `testDeployer`, `testUser3`, `testCaller` |
| Authority | `"lgm1authority"` | `testAuthority` |
| Auth keeper | `setupKeeperWithAuth` | N/A — use `setupKeeper` |
| Module balance | `bk.balances["module:bvm"]` | `bk.moduleBalances["bvm"]` |
| Block context | `ctx.WithBlockHeight(N)` | `ctx.WithBlockHeader(cmtproto.Header{Height: N, ChainID: testChainID})` |
| Opcode bytes | Raw hex `0x00`, `0x60` | `vm.STOP`, `vm.PUSH1`, etc. |
| Deploy cost | Free | 5M `uzrn` — fund deployer |
| deployContract helper | Takes `types.ExtendedMsgServer` | Takes `types.MsgServer` |

---

### Task 1: Create `gas_bridge_test.go`

**Files:**
- Create: `x/bvm/keeper/gas_bridge_test.go`

**Step 1: Write the test file**

```go
package keeper_test

import (
	"fmt"
	"testing"

	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/bvm/keeper"
	"github.com/zerone-chain/zerone/x/bvm/types"
	"github.com/zerone-chain/zerone/x/bvm/vm"
)

// ---------- Bytecode Helpers (gas bridge specific) ----------

// multiSstoreBytecode writes N values to N storage slots.
// Each SSTORE costs 20000 gas, so total gas ≈ N * 20000 + overhead.
func multiSstoreBytecode(count int) []byte {
	code := make([]byte, 0, count*5+1)
	for i := 0; i < count; i++ {
		code = append(code, vm.PUSH1, byte(i+1)) // value
		code = append(code, vm.PUSH1, byte(i))   // slot
		code = append(code, vm.SSTORE)
	}
	code = append(code, vm.STOP)
	return code
}

// tripleCallBytecode creates bytecode that performs 3 CALL operations.
// Each CALL has base cost ~700 gas, exercising nested gas accounting.
func tripleCallBytecode() []byte {
	code := make([]byte, 0, 60)
	for i := 0; i < 3; i++ {
		code = append(code,
			vm.PUSH1, 0, // retLength
			vm.PUSH1, 0, // retOffset
			vm.PUSH1, 0, // argsLength
			vm.PUSH1, 0, // argsOffset
			vm.PUSH1, 0, // value
			vm.PUSH1, 1, // addr (precompile identity)
			vm.PUSH2, 0x13, 0x88, // gas=5000
			vm.CALL,
			vm.POP, // pop result
		)
	}
	code = append(code, vm.STOP)
	return code
}

// deepCallChainBytecode performs 10 nested CALLs to address 0.
func deepCallChainBytecode() []byte {
	code := make([]byte, 0, 150)
	for i := 0; i < 10; i++ {
		code = append(code,
			vm.PUSH1, 0, // retLength
			vm.PUSH1, 0, // retOffset
			vm.PUSH1, 0, // argsLength
			vm.PUSH1, 0, // argsOffset
			vm.PUSH1, 0, // value
			vm.PUSH1, 0, // addr
			vm.PUSH2, 0x27, 0x10, // gas=10000
			vm.CALL,
			vm.POP,
		)
	}
	code = append(code, vm.STOP)
	return code
}

// =========================================================================
// Gas Bridge: SDK gas meter ↔ BVM gas bridging
// =========================================================================

func TestGasBridge_SimpleContract(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(42))

	gasMeter := storetypes.NewGasMeter(10_000_000)
	ctx = ctx.WithGasMeter(gasMeter)

	gasBefore := gasMeter.GasConsumed()

	resp, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        500_000,
	})
	if err != nil {
		t.Fatalf("call error: %v", err)
	}

	sdkGasConsumed := gasMeter.GasConsumed() - gasBefore

	// SDK gas meter must be charged at least the VM's reported gas
	if sdkGasConsumed < resp.GasUsed {
		t.Fatalf("SDK gas consumed (%d) < VM gas used (%d) — gas bridge broken",
			sdkGasConsumed, resp.GasUsed)
	}
	// SSTORE costs 20000+ gas
	if resp.GasUsed < 20_000 {
		t.Fatalf("expected VM gas >= 20000 (SSTORE), got %d", resp.GasUsed)
	}
}

func TestGasBridge_OutOfGas_SDKLimit(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(42))

	// Set SDK gas limit lower than what the VM will consume
	gasMeter := storetypes.NewGasMeter(100)
	ctx = ctx.WithGasMeter(gasMeter)

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _ = srv.CallContract(ctx, &types.MsgCallContract{
			Caller:          testCaller,
			ContractAddress: addr,
			GasLimit:        500_000,
		})
	}()

	if !panicked {
		t.Fatal("expected out-of-gas panic when SDK gas limit < VM gas consumed")
	}
}

func TestGasBridge_OutOfGas_VMLimit(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, infiniteLoopBytecode())

	gasMeter := storetypes.NewGasMeter(10_000_000)
	ctx = ctx.WithGasMeter(gasMeter)

	gasBefore := gasMeter.GasConsumed()

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1_000, // low VM gas limit
	})
	if err == nil {
		t.Fatal("expected error from VM out-of-gas")
	}

	sdkGasConsumed := gasMeter.GasConsumed() - gasBefore
	if sdkGasConsumed == 0 {
		t.Fatal("SDK gas meter shows 0 — gas bridge not working for failed executions")
	}
	if sdkGasConsumed < 900 {
		t.Fatalf("SDK gas consumed (%d) too low — expected near VM gas limit (1000)", sdkGasConsumed)
	}
}

func TestGasBridge_NestedCalls(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	// Deploy triple-CALL contract
	addrNested := deployContract(t, srv, ctx, testDeployer, tripleCallBytecode())
	// Deploy simple STOP for baseline
	addrSimple := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	// Baseline: simple STOP
	gasMeter1 := storetypes.NewGasMeter(10_000_000)
	ctx1 := ctx.WithGasMeter(gasMeter1)
	_, err := srv.CallContract(ctx1, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addrSimple,
		GasLimit:        500_000,
	})
	if err != nil {
		t.Fatalf("simple call error: %v", err)
	}
	simpleGas := gasMeter1.GasConsumed()

	// Nested CALLs
	gasMeter2 := storetypes.NewGasMeter(10_000_000)
	ctx2 := ctx.WithGasMeter(gasMeter2)
	resp, err := srv.CallContract(ctx2, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addrNested,
		GasLimit:        500_000,
	})
	if err != nil {
		t.Fatalf("nested call error: %v", err)
	}
	nestedGas := gasMeter2.GasConsumed()

	if nestedGas <= simpleGas {
		t.Fatalf("nested call SDK gas (%d) should be > simple SDK gas (%d)", nestedGas, simpleGas)
	}
	if nestedGas < resp.GasUsed {
		t.Fatalf("SDK gas (%d) < VM gas (%d) — bridge broken for nested calls", nestedGas, resp.GasUsed)
	}
}

func TestGasBridge_SSTOREHeavyContract(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, multiSstoreBytecode(5))

	gasMeter := storetypes.NewGasMeter(10_000_000)
	ctx = ctx.WithGasMeter(gasMeter)

	resp, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        500_000,
	})
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	// 5 SSTOREs should cost at least 5 × 20000 = 100000 VM gas
	if resp.GasUsed < 100_000 {
		t.Fatalf("expected VM gas >= 100000 for 5 SSTOREs, got %d", resp.GasUsed)
	}
	sdkGas := gasMeter.GasConsumed()
	if sdkGas < resp.GasUsed {
		t.Fatalf("SDK gas (%d) < VM gas (%d) — bridge broken", sdkGas, resp.GasUsed)
	}
}

func TestGasBridge_PanicChargesFullGas(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, invalidBytecode())

	gasMeter := storetypes.NewGasMeter(10_000_000)
	ctx = ctx.WithGasMeter(gasMeter)

	gasBefore := gasMeter.GasConsumed()

	_, _ = srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        100_000,
	})

	sdkGasConsumed := gasMeter.GasConsumed() - gasBefore
	if sdkGasConsumed == 0 {
		t.Fatal("expected non-zero gas consumption for failed execution")
	}
}

// =========================================================================
// Gas Limit Cap: ValidateBasic rejects over-limit gas
// =========================================================================

func TestGasLimitCap_CallContract(t *testing.T) {
	msg := &types.MsgCallContract{
		Caller:          testDeployer,
		ContractAddress: "zrn1contracttest",
		GasLimit:        types.MaxContractGasLimit + 1,
	}
	err := msg.ValidateBasic()
	if err == nil {
		t.Fatal("expected error for gas limit exceeding MaxContractGasLimit")
	}

	msg.GasLimit = types.MaxContractGasLimit
	err = msg.ValidateBasic()
	if err != nil {
		t.Fatalf("expected no error at exact cap, got: %v", err)
	}
}

func TestGasLimitCap_ScheduleContract(t *testing.T) {
	msg := &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: "zrn1contracttest",
		Method:          "test",
		ExecuteAtBlock:  200,
		MaxGas:          types.MaxContractGasLimit + 1,
	}
	err := msg.ValidateBasic()
	if err == nil {
		t.Fatal("expected error for MaxGas exceeding MaxContractGasLimit")
	}

	msg.MaxGas = types.MaxContractGasLimit
	err = msg.ValidateBasic()
	if err != nil {
		t.Fatalf("expected no error at exact cap, got: %v", err)
	}
}

func TestEffectiveGasCap(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, infiniteLoopBytecode())

	// Schedule with gas far exceeding the per-block budget
	_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "loop",
		ExecuteAtBlock:  200,
		MaxGas:          types.MaxContractGasLimit, // 10M > MaxScheduledGasPerBlock (5M)
	})
	if err != nil {
		t.Fatalf("schedule error: %v", err)
	}

	// Execute at block 200 — should not panic, gas should be capped
	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	// If we get here without panic, the effective gas cap is working
}
```

**Step 2: Run tests**

Run: `go test ./x/bvm/... -count=1 -run "TestGasBridge|TestGasLimitCap|TestEffectiveGasCap" -v`
Expected: All 9 tests PASS (some may need opcode constant adjustments — see Step 3)

**Step 3: Fix opcode constants if needed**

Check if `vm.CALL`, `vm.POP`, `vm.PUSH2` exist:
Run: `grep -E "CALL|POP|PUSH2" x/bvm/vm/opcodes.go`

If any constants are missing, use raw hex bytes instead (e.g., `0xF1` for CALL, `0x50` for POP, `0x61` for PUSH2).

**Step 4: Commit**

```bash
git add x/bvm/keeper/gas_bridge_test.go
git commit -m "test(R14-2): port BVM gas bridge tests from prototype"
```

---

### Task 2: Create `security_test.go`

**Files:**
- Create: `x/bvm/keeper/security_test.go`

**Step 1: Write the test file**

```go
package keeper_test

// Adversarial probe tests ported from the prototype's OpenClaw BVM audit suite.
// Function names are self-documenting; audit reference IDs noted in comments only.

import (
	"fmt"
	"testing"

	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/bvm/keeper"
	"github.com/zerone-chain/zerone/x/bvm/types"
	"github.com/zerone-chain/zerone/x/bvm/vm"
)

// =========================================================================
// Adversarial Security Probes
// =========================================================================

// Cross-contract state isolation at the raw state layer. (Audit: OCBVM-5)
func TestSecurity_CrossContractStateIsolation(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	contractA := "zrn1contractAAAAAAAAAAAAAAAAAAAAAAAA"
	contractB := "zrn1contractBBBBBBBBBBBBBBBBBBBBBBBB"

	k.SetContractState(ctx, contractA, "secret", "alice-private-data")
	k.SetContractState(ctx, contractB, "secret", "bob-private-data")

	valA, foundA := k.GetContractState(ctx, contractA, "secret")
	valB, foundB := k.GetContractState(ctx, contractB, "secret")

	if !foundA || valA != "alice-private-data" {
		t.Fatalf("contract A state corrupted: got %q (found=%v)", valA, foundA)
	}
	if !foundB || valB != "bob-private-data" {
		t.Fatalf("contract B state corrupted: got %q (found=%v)", valB, foundB)
	}

	// Cross-read attempt
	_, crossFound := k.GetContractState(ctx, contractB, "alice-private-data")
	if crossFound {
		t.Fatal("cross-contract state leak detected")
	}

	countA := k.CountContractState(ctx, contractA)
	countB := k.CountContractState(ctx, contractB)
	if countA != 1 || countB != 1 {
		t.Fatalf("state count mismatch: A=%d, B=%d", countA, countB)
	}
}

// Successful payable call retains value in module account. (Audit: OCBVM-7)
func TestSecurity_ValueTransfer_SuccessfulCallRetains(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)
	bk.setBalance(testCaller, "uzrn", 100000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        100000,
		Value:           "25000",
	})
	if err != nil {
		t.Fatalf("payable call should succeed: %v", err)
	}

	// Caller debited
	if bk.balances[testCaller]["uzrn"] != 75000 {
		t.Fatalf("expected caller balance 75000, got %d", bk.balances[testCaller]["uzrn"])
	}
	// Module received
	if bk.moduleBalances["bvm"]["uzrn"] != 25000 {
		t.Fatalf("expected BVM module balance 25000, got %d", bk.moduleBalances["bvm"]["uzrn"])
	}
}

// Schedule cancellation authorization: non-owner blocked, double-cancel blocked,
// cancel-after-execute blocked. (Audit: OCBVM-8)
func TestSecurity_ScheduleCancelAuth(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	schedResp, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  300,
		MaxGas:          50000,
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}

	// Attack 1: non-owner cancel
	_, err = srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testUser3,
		ScheduleId: schedResp.ScheduleId,
	})
	if err == nil {
		t.Fatal("non-owner was able to cancel schedule")
	}

	schedule, _ := k.GetSchedule(ctx, schedResp.ScheduleId)
	if schedule.Cancelled {
		t.Fatal("schedule was cancelled by non-owner")
	}

	// Owner cancels successfully
	_, err = srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testDeployer,
		ScheduleId: schedResp.ScheduleId,
	})
	if err != nil {
		t.Fatalf("owner cancel failed: %v", err)
	}

	// Attack 2: double-cancel
	_, err = srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testDeployer,
		ScheduleId: schedResp.ScheduleId,
	})
	if err == nil {
		t.Fatal("double-cancel should be rejected")
	}

	// Attack 3: cancel already-executed schedule
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-executed-test",
		ContractAddress: addr,
		Caller:          testDeployer,
		Method:          "done",
		ExecuteAtBlock:  50,
		Executed:        true,
	})
	_, err = srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testDeployer,
		ScheduleId: "sched-executed-test",
	})
	if err == nil {
		t.Fatal("cancel of executed schedule should be rejected")
	}
}

// Governance state update: only authority can modify state. (Audit: OCBVM-9)
func TestSecurity_GovernanceStateUpdateAuth(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())
	k.SetContractState(ctx, addr, "admin", testDeployer)

	// Attack: creator tries governance path
	_, err := srv.UpdateContractState(ctx, &types.MsgUpdateContractState{
		Authority:       testDeployer,
		ContractAddress: addr,
		Key:             "admin",
		Value:           "hacked-by-creator",
	})
	if err == nil {
		t.Fatal("non-authority was able to update contract state")
	}
	val, _ := k.GetContractState(ctx, addr, "admin")
	if val != testDeployer {
		t.Fatalf("state modified by unauthorized party: got %q", val)
	}

	// Attack: random address
	_, err = srv.UpdateContractState(ctx, &types.MsgUpdateContractState{
		Authority:       testCaller,
		ContractAddress: addr,
		Key:             "admin",
		Value:           "hacked-by-random",
	})
	if err == nil {
		t.Fatal("random address was able to update contract state")
	}

	// Legitimate authority update
	_, err = srv.UpdateContractState(ctx, &types.MsgUpdateContractState{
		Authority:       testAuthority,
		ContractAddress: addr,
		Key:             "admin",
		Value:           "governance-approved-admin",
	})
	if err != nil {
		t.Fatalf("authority update should succeed: %v", err)
	}
	val, _ = k.GetContractState(ctx, addr, "admin")
	if val != "governance-approved-admin" {
		t.Fatalf("authority update didn't persist: got %q", val)
	}
}

// Schedule limit enforcement with cancel-frees-slot. (Audit: OCBVM-11)
func TestSecurity_ScheduleLimitEnforcement(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	params := k.GetParams(ctx)
	params.MaxSchedulesPerContract = 3
	k.SetParams(ctx, params)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	// Fill up to limit
	var scheduleIds []string
	for i := 0; i < 3; i++ {
		resp, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
			Caller:          testDeployer,
			ContractAddress: addr,
			Method:          "tick",
			ExecuteAtBlock:  uint64(200 + i),
			MaxGas:          50000,
		})
		if err != nil {
			t.Fatalf("schedule %d should succeed: %v", i, err)
		}
		scheduleIds = append(scheduleIds, resp.ScheduleId)
	}

	// Attack: 4th should be rejected
	_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  300,
		MaxGas:          50000,
	})
	if err == nil {
		t.Fatal("schedule beyond limit was accepted")
	}

	// Cancel one — should free a slot
	_, err = srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testDeployer,
		ScheduleId: scheduleIds[0],
	})
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	// Now should succeed
	_, err = srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  400,
		MaxGas:          50000,
	})
	if err != nil {
		t.Fatalf("schedule after cancel should succeed: %v", err)
	}
}

// Bytecode size limit with configurable max and huge payload. (Audit: OCBVM-12)
func TestSecurity_BytecodeSizeLimit(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	params := k.GetParams(ctx)
	params.MaxBytecodeSize = 100
	k.SetParams(ctx, params)

	// At limit — should succeed
	atLimit := make([]byte, 100)
	atLimit[0] = vm.STOP
	_, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: atLimit,
	})
	if err != nil {
		t.Fatalf("bytecode at limit should be accepted: %v", err)
	}

	// Over limit — should fail
	_, err = srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: make([]byte, 101),
	})
	if err == nil {
		t.Fatal("bytecode over limit was accepted")
	}

	// Way over limit (10 MB)
	_, err = srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: make([]byte, 10_000_000),
	})
	if err == nil {
		t.Fatal("huge bytecode was accepted")
	}
}

// Gas exhaustion in scheduled execution doesn't block others. (Audit: OCBVM-14)
func TestSecurity_GasExhaustion_ScheduledExecution(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	addrLoop := deployContract(t, srv, ctx, testDeployer, infiniteLoopBytecode())
	addrNormal := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	schedLoop, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addrLoop,
		Method:          "loop",
		ExecuteAtBlock:  200,
		MaxGas:          1_000,
	})
	schedNormal, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addrNormal,
		Method:          "ok",
		ExecuteAtBlock:  200,
		MaxGas:          50_000,
	})

	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	loop, _ := k.GetSchedule(ctx200, schedLoop.ScheduleId)
	normal, _ := k.GetSchedule(ctx200, schedNormal.ScheduleId)

	if !loop.Executed {
		t.Fatal("gas-exhausted schedule not marked as executed")
	}
	if !normal.Executed {
		t.Fatal("normal schedule blocked by gas-exhausted sibling")
	}
}

// Deterministic contract addresses: same deployer/block gets different
// addresses via nonce; different deployer gets different address. (Audit: OCBVM-15)
func TestSecurity_DeterministicContractAddresses(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)
	bk.setBalance(testUser3, "uzrn", 100000000)

	addr1 := deployContract(t, srv, ctx, testDeployer, []byte{vm.STOP})
	addr2 := deployContract(t, srv, ctx, testDeployer, returnBytecode(1))
	addr3 := deployContract(t, srv, ctx, testDeployer, []byte{vm.STOP})

	// All should be different (nonce increments)
	if addr1 == addr2 || addr2 == addr3 || addr1 == addr3 {
		t.Fatalf("address collision: %s, %s, %s", addr1, addr2, addr3)
	}

	// Different deployer should also be different
	addr4 := deployContract(t, srv, ctx, testUser3, []byte{vm.STOP})
	if addr4 == addr1 || addr4 == addr2 || addr4 == addr3 {
		t.Fatalf("cross-deployer address collision: %s", addr4)
	}

	// Verify stored correctly
	contract1, found1 := k.GetContract(ctx, addr1)
	contract4, found4 := k.GetContract(ctx, addr4)
	if !found1 || !found4 {
		t.Fatal("contracts not stored")
	}
	if contract1.Creator != testDeployer || contract4.Creator != testUser3 {
		t.Fatal("creator mismatch")
	}
}

// Overdue schedules (chain halt scenario) all execute on resume. (Audit: OCBVM-16)
func TestSecurity_OverdueScheduleExecution(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	var ids []string
	for _, block := range []uint64{150, 160, 170} {
		resp, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
			Caller:          testDeployer,
			ContractAddress: addr,
			Method:          fmt.Sprintf("block_%d", block),
			ExecuteAtBlock:  block,
			MaxGas:          50000,
		})
		ids = append(ids, resp.ScheduleId)
	}

	// Chain "halts" and resumes at block 200
	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	for i, id := range ids {
		schedule, _ := k.GetSchedule(ctx200, id)
		if !schedule.Executed {
			t.Fatalf("overdue schedule %d (id=%s) was not executed", i, id)
		}
	}
}

// Insufficient funds for payable call: rejected pre-execution, balance unchanged.
// (Audit: OCBVM-18)
func TestSecurity_InsufficientFundsPayableCall(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)
	bk.setBalance(testCaller, "uzrn", 100)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        100000,
		Value:           "1000",
	})
	if err == nil {
		t.Fatal("insufficient funds should be rejected")
	}

	// Balance unchanged
	if bk.balances[testCaller]["uzrn"] != 100 {
		t.Fatalf("caller balance changed despite rejection: %d", bk.balances[testCaller]["uzrn"])
	}
}

// =========================================================================
// Panic Recovery (with gas meter verification)
// =========================================================================

func TestPanicRecovery_INVALID_Opcode_GasBridge(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, invalidBytecode())

	gasMeter := storetypes.NewGasMeter(10_000_000)
	ctx = ctx.WithGasMeter(gasMeter)

	gasBefore := gasMeter.GasConsumed()

	// Should NOT panic — safeExecute catches it
	_, callErr := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        100000,
	})

	sdkGasConsumed := gasMeter.GasConsumed() - gasBefore

	// Error expected (INVALID opcode fails) but no panic
	if callErr == nil {
		t.Log("INVALID opcode handled gracefully by interpreter")
	}
	// Gas should still be bridged
	if sdkGasConsumed == 0 {
		t.Fatal("SDK gas should be consumed even for INVALID opcode")
	}
}

func TestPanicRecovery_StackOverflow(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, deepCallChainBytecode())

	gasMeter := storetypes.NewGasMeter(10_000_000)
	ctx = ctx.WithGasMeter(gasMeter)

	gasBefore := gasMeter.GasConsumed()

	// Should NOT panic even with deep call chain
	_, _ = srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1_000_000,
	})

	if gasMeter.GasConsumed() <= gasBefore {
		t.Fatal("expected gas consumption from deep call chain")
	}
}

func TestPanicRecovery_MultipleSchedules_OnePanics_Integration(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	// Deploy: good, bad, good
	addrGood1 := deployContract(t, srv, ctx, testDeployer, simpleBytecode())
	addrBad := deployContract(t, srv, ctx, testDeployer, invalidBytecode())
	addrGood2 := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	sched1, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller: testDeployer, ContractAddress: addrGood1,
		Method: "ok1", ExecuteAtBlock: 200, MaxGas: 50000,
	})
	_, _ = srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller: testDeployer, ContractAddress: addrBad,
		Method: "crash", ExecuteAtBlock: 200, MaxGas: 50000,
	})
	sched3, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller: testDeployer, ContractAddress: addrGood2,
		Method: "ok2", ExecuteAtBlock: 200, MaxGas: 50000,
	})

	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	s1, _ := k.GetSchedule(ctx200, sched1.ScheduleId)
	s3, _ := k.GetSchedule(ctx200, sched3.ScheduleId)

	if !s1.Executed {
		t.Fatal("good schedule 1 should have executed")
	}
	if !s3.Executed {
		t.Fatal("good schedule 3 should have executed despite schedule 2 panicking")
	}
}
```

**Step 2: Run tests**

Run: `go test ./x/bvm/... -count=1 -run "TestSecurity_|TestPanicRecovery_.*_GasBridge|TestPanicRecovery_StackOverflow|TestPanicRecovery_MultipleSchedules_OnePanics_Integration" -v`
Expected: All 13 tests PASS

**Step 3: Commit**

```bash
git add x/bvm/keeper/security_test.go
git commit -m "test(R14-2): port BVM security probes + panic recovery tests"
```

---

### Task 3: Create `schedule_test.go`

**Files:**
- Create: `x/bvm/keeper/schedule_test.go`

**Step 1: Write the test file**

```go
package keeper_test

import (
	"fmt"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/bvm/keeper"
	"github.com/zerone-chain/zerone/x/bvm/types"
	"github.com/zerone-chain/zerone/x/bvm/vm"
)

// =========================================================================
// Schedule Edge Cases
// =========================================================================

func TestScheduleContract_ContractNotFound(t *testing.T) {
	srv, _, ctx, _ := setupMsgServer(t)

	_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: "zrn1nonexistent",
		Method:          "tick",
		ExecuteAtBlock:  200,
		MaxGas:          50000,
	})
	if err == nil {
		t.Fatal("expected error for scheduling non-existent contract")
	}
}

func TestCancelSchedule_AlreadyExecuted(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-exec",
		ContractAddress: "contract1",
		Caller:          testDeployer,
		Method:          "tick",
		ExecuteAtBlock:  50,
		Executed:        true,
	})

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testDeployer,
		ScheduleId: "sched-exec",
	})
	if err == nil {
		t.Fatal("expected error for cancelling already-executed schedule")
	}
}

func TestGetPendingSchedules_IncludesOverdue(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Overdue at block 150
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-overdue",
		ContractAddress: "c1",
		Caller:          testDeployer,
		Method:          "a",
		ExecuteAtBlock:  150,
	})
	// On-time at block 200
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-ontime",
		ContractAddress: "c1",
		Caller:          testDeployer,
		Method:          "b",
		ExecuteAtBlock:  200,
	})
	// Future at block 300 (should NOT appear)
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-future",
		ContractAddress: "c1",
		Caller:          testDeployer,
		Method:          "c",
		ExecuteAtBlock:  300,
	})

	pending := k.GetPendingSchedules(ctx, 200)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending schedules (overdue + on-time), got %d", len(pending))
	}
}

// =========================================================================
// Per-Block Gas Budget Enforcement
// =========================================================================

func TestScheduleBudget_SingleSchedule_FitsInBudget(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	schedResp, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  200,
		MaxGas:          50000,
	})
	if err != nil {
		t.Fatalf("schedule error: %v", err)
	}

	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	schedule, found := k.GetSchedule(ctx200, schedResp.ScheduleId)
	if !found {
		t.Fatal("schedule not found")
	}
	if !schedule.Executed {
		t.Fatal("single schedule should have executed within budget")
	}
}

func TestScheduleBudget_MultipleSchedules_ExceedsBudget(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, infiniteLoopBytecode())

	// Schedule 3 contracts each requesting 3M gas
	// MaxScheduledGasPerBlock = 5M, so not all fit
	schedIds := make([]string, 3)
	for i := 0; i < 3; i++ {
		resp, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
			Caller:          testDeployer,
			ContractAddress: addr,
			Method:          fmt.Sprintf("loop%d", i),
			ExecuteAtBlock:  200,
			MaxGas:          3_000_000,
		})
		if err != nil {
			t.Fatalf("schedule %d error: %v", i, err)
		}
		schedIds[i] = resp.ScheduleId
	}

	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	var executed, deferred int
	for _, id := range schedIds {
		sched, found := k.GetSchedule(ctx200, id)
		if !found {
			t.Fatalf("schedule %s not found", id)
		}
		if sched.Executed {
			executed++
		} else {
			deferred++
		}
	}

	if executed == 0 {
		t.Fatal("expected at least 1 schedule to execute")
	}
	if deferred == 0 {
		t.Fatal("expected at least 1 schedule to be deferred (budget exceeded)")
	}
}

func TestScheduleBudget_DeferredScheduleRunsNextBlock(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, infiniteLoopBytecode())

	schedIds := make([]string, 3)
	for i := 0; i < 3; i++ {
		resp, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
			Caller:          testDeployer,
			ContractAddress: addr,
			Method:          fmt.Sprintf("loop%d", i),
			ExecuteAtBlock:  200,
			MaxGas:          3_000_000,
		})
		if err != nil {
			t.Fatalf("schedule %d error: %v", i, err)
		}
		schedIds[i] = resp.ScheduleId
	}

	// Block 200
	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	var executedBlock200 int
	for _, id := range schedIds {
		sched, _ := k.GetSchedule(ctx200, id)
		if sched.Executed {
			executedBlock200++
		}
	}

	// Block 201 — deferred should run
	ctx201 := ctx.WithBlockHeader(cmtproto.Header{Height: 201, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx201)

	var executedBlock201 int
	for _, id := range schedIds {
		sched, _ := k.GetSchedule(ctx201, id)
		if sched.Executed {
			executedBlock201++
		}
	}

	if executedBlock201 <= executedBlock200 {
		t.Fatalf("deferred schedules didn't run in next block: block200=%d, block201=%d",
			executedBlock200, executedBlock201)
	}
}

func TestScheduleBudget_PanicInSchedule_DoesntBlockOthers(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	addrPanic := deployContract(t, srv, ctx, testDeployer, invalidBytecode())
	addrNormal := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, _ = srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addrPanic,
		Method:          "crash",
		ExecuteAtBlock:  200,
		MaxGas:          50000,
	})
	normalSchedResp, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addrNormal,
		Method:          "ok",
		ExecuteAtBlock:  200,
		MaxGas:          50000,
	})

	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	normalSched, found := k.GetSchedule(ctx200, normalSchedResp.ScheduleId)
	if !found {
		t.Fatal("normal schedule not found")
	}
	if !normalSched.Executed {
		t.Fatal("normal schedule should execute despite prior panic")
	}
}

func TestScheduledGasBudget(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	// Schedule 3 contracts each requesting near-max gas
	maxGasPerSchedule := uint64(3_000_000)
	for i := 0; i < 3; i++ {
		_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
			Caller:          testDeployer,
			ContractAddress: addr,
			Method:          fmt.Sprintf("run%d", i),
			ExecuteAtBlock:  200,
			MaxGas:          maxGasPerSchedule,
		})
		if err != nil {
			t.Fatalf("schedule error: %v", err)
		}
	}

	ctx200 := ctx.WithBlockHeader(cmtproto.Header{Height: 200, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx200)

	var executed int
	k.IterateSchedules(ctx200, func(s *types.ContractSchedule) bool {
		if s.Executed {
			executed++
		}
		return false
	})

	if executed == 0 {
		t.Fatal("no schedules executed")
	}
}
```

**Step 2: Run tests**

Run: `go test ./x/bvm/... -count=1 -run "TestScheduleContract_ContractNotFound|TestCancelSchedule_AlreadyExecuted|TestGetPendingSchedules_IncludesOverdue|TestScheduleBudget_|TestScheduledGasBudget" -v`
Expected: All 8 tests PASS

**Step 3: Commit**

```bash
git add x/bvm/keeper/schedule_test.go
git commit -m "test(R14-2): port BVM schedule edge cases + budget tests"
```

---

### Task 4: Additions to `keeper_test.go`

**Files:**
- Modify: `x/bvm/keeper/keeper_test.go`

**Step 1: Add tests at end of file**

Append the following after `TestCountContractSchedules`:

```go
// =========================================================================
// Section 18: Ported from Prototype — Genesis + Lifecycle Edge Cases
// =========================================================================

func TestGenesisRoundtrip_IncludesState(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	srv := keeper.NewMsgServerImpl(k)
	deployResp, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: simpleBytecode(),
	})

	k.SetContractState(ctx, deployResp.ContractAddress, "counter", "42")
	k.SetContractState(ctx, deployResp.ContractAddress, "owner", testDeployer)

	gs := k.ExportGenesis(ctx)
	if len(gs.State) != 2 {
		t.Fatalf("expected 2 state entries in genesis, got %d", len(gs.State))
	}

	k2, ctx2, _ := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	val, found := k2.GetContractState(ctx2, deployResp.ContractAddress, "counter")
	if !found || val != "42" {
		t.Fatalf("expected counter=42, got %s (found=%v)", val, found)
	}
	val, found = k2.GetContractState(ctx2, deployResp.ContractAddress, "owner")
	if !found || val != testDeployer {
		t.Fatalf("expected owner=%s, got %s (found=%v)", testDeployer, val, found)
	}
}

func TestDeleteContract_DecrRefCount(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)
	bk.setBalance(testUser3, "uzrn", 100000000)

	bytecode := returnBytecode(42)

	resp1, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: bytecode,
	})
	resp2, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testUser3,
		Bytecode: bytecode,
	})

	contract1, _ := k.GetContract(ctx, resp1.ContractAddress)
	code, _ := k.GetCode(ctx, contract1.CodeHash)
	if code.RefCount != 2 {
		t.Fatalf("expected refcount 2, got %d", code.RefCount)
	}

	// Delete first — refcount should drop to 1
	k.DeleteContract(ctx, contract1)
	code, found := k.GetCode(ctx, contract1.CodeHash)
	if !found {
		t.Fatal("code should still exist with refcount 1")
	}
	if code.RefCount != 1 {
		t.Fatalf("expected refcount 1 after delete, got %d", code.RefCount)
	}

	// Delete second — code should be garbage collected
	contract2, _ := k.GetContract(ctx, resp2.ContractAddress)
	k.DeleteContract(ctx, contract2)
	_, found = k.GetCode(ctx, contract1.CodeHash)
	if found {
		t.Fatal("code should be garbage collected when refcount reaches 0")
	}
}

func TestCallContract_ZeroValueNoTransfer(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)
	bk.setBalance(testCaller, "uzrn", 50000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        100000,
		Value:           "0",
	})
	if err != nil {
		t.Fatalf("zero value call: %v", err)
	}

	if bk.balances[testCaller]["uzrn"] != 50000 {
		t.Fatalf("expected no change in balance, got %d", bk.balances[testCaller]["uzrn"])
	}
}
```

**Step 2: Run tests**

Run: `go test ./x/bvm/... -count=1 -run "TestGenesisRoundtrip_IncludesState|TestDeleteContract_DecrRefCount|TestCallContract_ZeroValueNoTransfer" -v`
Expected: All 3 tests PASS

**Step 3: Commit**

```bash
git add x/bvm/keeper/keeper_test.go
git commit -m "test(R14-2): port genesis + lifecycle edge case tests"
```

---

### Task 5: Full Test Suite Verification

**Step 1: Run full suite**

Run: `go test ./x/bvm/... -count=1 -v`
Expected: All tests PASS, no compilation errors

**Step 2: Run go vet**

Run: `go vet ./x/bvm/...`
Expected: No warnings

**Step 3: Count test functions**

Run: `grep -c "func Test" x/bvm/keeper/*_test.go x/bvm/types/*_test.go 2>/dev/null`
Expected:
- `keeper_test.go`: 76 (73 + 3 new)
- `gas_bridge_test.go`: 9
- `schedule_test.go`: 8
- `security_test.go`: 13
- **Total: 106**

**Step 4: Final commit (if any fixes were needed)**

```bash
git add x/bvm/keeper/*_test.go
git commit -m "test(R14-2): final BVM test porting — 73→106 test functions"
```

---

## Opcode Constants Reference

If any `vm.CALL`, `vm.POP`, or `vm.PUSH2` constants don't exist, use raw hex:

| Constant | Hex |
|----------|-----|
| `vm.STOP` | `0x00` |
| `vm.PUSH1` | `0x60` |
| `vm.PUSH2` | `0x61` |
| `vm.PUSH32` | `0x7F` |
| `vm.MSTORE` | `0x52` |
| `vm.SSTORE` | `0x55` |
| `vm.RETURN` | `0xF3` |
| `vm.REVERT` | `0xFD` |
| `vm.CALL` | `0xF1` |
| `vm.POP` | `0x50` |
| `vm.JUMP` | `0x56` |
| `vm.JUMPDEST` | `0x5B` |
| `vm.LOG0` | `0xA0` |
| `vm.INVALID` | `0xFE` |
| `vm.KQUERY` | (custom) |
