package keeper_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/bvm/types"
	"github.com/zerone-chain/zerone/x/bvm/vm"
)

// ---------- Bytecode Helpers (gas bridge specific) ----------

// multiSstoreBytecode writes N values to N storage slots.
// Each SSTORE costs 20000 gas, so total gas ~ N * 20000 + overhead.
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
// Gas Bridge: SDK gas meter <-> BVM gas bridging
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
	// 5 SSTOREs should cost at least 5 x 20000 = 100000 VM gas
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
