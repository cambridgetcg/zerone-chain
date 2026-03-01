package keeper_test

// Adversarial probe tests ported from the prototype's OpenClaw BVM audit suite.
// Function names are self-documenting; audit reference IDs noted in comments only.

import (
	"fmt"
	"testing"

	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

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
	// Module received (deploy cost 5M + value transfer 25000)
	expectedModule := int64(5000000 + 25000)
	if bk.moduleBalances["bvm"]["uzrn"] != expectedModule {
		t.Fatalf("expected BVM module balance %d, got %d", expectedModule, bk.moduleBalances["bvm"]["uzrn"])
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
