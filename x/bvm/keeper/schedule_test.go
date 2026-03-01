package keeper_test

import (
	"fmt"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/bvm/keeper"
	"github.com/zerone-chain/zerone/x/bvm/types"
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
