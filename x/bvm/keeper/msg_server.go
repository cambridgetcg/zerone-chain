package keeper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/bvm/types"
	"github.com/zerone-chain/zerone/x/bvm/vm"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns a MsgServer implementation.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = &msgServer{}

// safeExecute wraps interpreter execution with panic recovery.
func safeExecute(
	interp *vm.Interpreter,
	bytecode []byte,
	execCtx *vm.ExecutionContext,
	stateDB vm.StateDB,
	host vm.HostFunctions,
) (result *vm.ExecutionResult, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			result = &vm.ExecutionResult{
				Success: false,
				GasUsed: execCtx.GasLimit,
			}
		}
	}()
	return interp.Execute(bytecode, execCtx, stateDB, host), false
}

// DeployContract deploys a new contract to the BVM.
func (m msgServer) DeployContract(goCtx context.Context, msg *types.MsgDeployContract) (*types.MsgDeployContractResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := m.GetParams(ctx)

	if uint64(len(msg.Bytecode)) > params.MaxBytecodeSize {
		return nil, types.ErrBytecodeTooBig
	}

	// Charge deploy cost
	if params.DeployCost != "" && params.DeployCost != "0" {
		deployCost, ok := sdkmath.NewIntFromString(params.DeployCost)
		if ok {
			deployerAddr, _ := sdk.AccAddressFromBech32(msg.Deployer)
			deployCoins := sdk.NewCoins(sdk.NewCoin("uzrn", deployCost))
			if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, deployerAddr, types.ModuleName, deployCoins); err != nil {
				return nil, fmt.Errorf("insufficient funds for deploy cost: %w", err)
			}
		}
	}

	// Compute code hash
	hash := sha256.Sum256(msg.Bytecode)
	codeHash := fmt.Sprintf("%x", hash)

	// Code deduplication
	existingCode, codeExists := m.GetCode(ctx, codeHash)
	if codeExists {
		existingCode.RefCount++
		m.SetCode(ctx, existingCode)
	} else {
		m.SetCode(ctx, &types.ContractCode{
			CodeHash: codeHash,
			Bytecode: msg.Bytecode,
			RefCount: 1,
		})
	}

	// Generate deterministic contract address
	nonce := m.GetNextContractNonce(ctx)
	blockHeight := uint64(ctx.BlockHeight())
	addrInput := fmt.Sprintf("%s/%d/%d", msg.Deployer, blockHeight, nonce)
	addrHash := sha256.Sum256([]byte(addrInput))
	contractAddress := fmt.Sprintf("zrn1contract%x", addrHash[:20])

	contract := &types.DeployedContract{
		Address:         contractAddress,
		CodeHash:        codeHash,
		Creator:         msg.Deployer,
		DeployedAtBlock: blockHeight,
		BytecodeSize:    uint64(len(msg.Bytecode)),
		BvmVersion:      params.CurrentBvmVersion,
	}
	m.SetContract(ctx, contract)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.bvm.contract_deployed",
		sdk.NewAttribute("address", contractAddress),
		sdk.NewAttribute("deployer", msg.Deployer),
		sdk.NewAttribute("code_hash", codeHash),
	))

	return &types.MsgDeployContractResponse{
		ContractAddress: contractAddress,
	}, nil
}

// CallContract executes contract bytecode in the BVM interpreter.
func (m msgServer) CallContract(goCtx context.Context, msg *types.MsgCallContract) (*types.MsgCallContractResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	contract, found := m.GetContract(ctx, msg.ContractAddress)
	if !found {
		return nil, types.ErrContractNotFound
	}

	code, codeFound := m.GetCode(ctx, contract.CodeHash)
	if !codeFound {
		return nil, types.ErrCodeNotFound
	}

	params := m.GetParams(ctx)
	if msg.GasLimit > params.MaxGasPerCall {
		msg.GasLimit = params.MaxGasPerCall
	}

	callerAddr, _ := sdk.AccAddressFromBech32(msg.Caller)
	callerBytes := make([]byte, 20)
	copy(callerBytes, callerAddr)

	// Parse and transfer Value (payable contract support)
	callValue := new(big.Int)
	if msg.Value != "" && msg.Value != "0" {
		if v, ok := new(big.Int).SetString(msg.Value, 10); ok && v.Sign() > 0 {
			callValue = v
			valueCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(callValue)))
			if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, callerAddr, types.ModuleName, valueCoins); err != nil {
				return nil, fmt.Errorf("insufficient funds for contract value: %w", err)
			}
		}
	}

	contractBytes := []byte(msg.ContractAddress)

	// Resolve caller DID and session capabilities
	var callerDID string
	var caps *vm.SessionCapabilities
	if m.authKeeper != nil {
		if did, found := m.authKeeper.GetAccountDID(ctx, msg.Caller); found {
			callerDID = did
		}
	}
	if m.authKeeper != nil && callerDID != "" {
		if sc, found := m.authKeeper.GetSessionCapabilities(ctx, msg.Caller, uint64(ctx.BlockHeight())); found {
			// Session key: use restricted capabilities
			caps = &vm.SessionCapabilities{
				CanTransfer:     sc.CanTransfer,
				CanStake:        sc.CanStake,
				CanSubmitClaims: sc.CanSubmitClaims,
				CanVote:         sc.CanVote,
			}
		} else {
			// Identity/operational key with no session key → full access
			caps = &vm.SessionCapabilities{
				CanTransfer: true, CanStake: true,
				CanSubmitClaims: true, CanVote: true,
			}
		}
	}
	// Anonymous caller (no DID): caps stays nil → all agent ops denied (C-1 secure default)

	// Execution mode
	mode := vm.ModeCall
	if msg.StaticCall {
		mode = vm.ModeStaticCall
	}

	execCtx := &vm.ExecutionContext{
		Caller:             callerBytes,
		Origin:             callerBytes,
		CurrentContract:    contractBytes,
		BlockNumber:        uint64(ctx.BlockHeight()),
		Timestamp:          uint64(ctx.BlockTime().Unix()),
		GasPrice:           new(big.Int),
		GasLimit:           msg.GasLimit,
		Value:              callValue,
		CallData:           msg.InputData,
		Mode:               mode,
		Bytecode:           code.Bytecode,
		ChainID:            big.NewInt(2521),
		ContractBvmVersion: contract.BvmVersion,
		CallerDID:          callerDID,
		Capabilities:       caps,
	}

	stateDB := vm.NewMemoryStateDB()
	// Pre-load contract state into memory state DB
	m.IterateContractState(ctx, msg.ContractAddress, func(key, value string) bool {
		keyBytes, _ := hex.DecodeString(key)
		valBytes, _ := hex.DecodeString(value)
		if keyBytes != nil {
			stateDB.SetStorage(contractBytes, keyBytes, valBytes)
		}
		return false
	})

	// Build host functions
	var host vm.HostFunctions
	if m.knowledgeKeeper != nil || m.homeKeeper != nil {
		host = &zeroneHost{kk: m.knowledgeKeeper, hk: m.homeKeeper, ctx: ctx}
	}

	// Execute with panic recovery
	interp := vm.NewInterpreter()
	result, panicked := safeExecute(interp, code.Bytecode, execCtx, stateDB, host)
	if panicked {
		m.Logger(ctx).Error("BVM CallContract execution panicked",
			"contract", msg.ContractAddress,
			"caller", msg.Caller,
		)
	}

	// Bridge VM gas to SDK gas meter
	ctx.GasMeter().ConsumeGas(result.GasUsed, "bvm_execution")

	// Apply state changes on success
	if result.Success {
		for _, change := range result.StateChanges {
			if change.Type == vm.StateChangeStorage {
				valBytes := ""
				if change.NewValue != nil {
					b32 := vm.WordToBytes32(change.NewValue)
					valBytes = hex.EncodeToString(b32[:])
				}
				m.SetContractState(ctx, msg.ContractAddress, change.Key, valBytes)
			}
		}
	}

	// Refund value on failure
	if !result.Success && callValue.Sign() > 0 {
		refundCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(callValue)))
		_ = m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, callerAddr, refundCoins)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.bvm.contract_called",
		sdk.NewAttribute("contract", msg.ContractAddress),
		sdk.NewAttribute("caller", msg.Caller),
		sdk.NewAttribute("gas_used", fmt.Sprintf("%d", result.GasUsed)),
		sdk.NewAttribute("success", fmt.Sprintf("%v", result.Success)),
	))

	if !result.Success {
		errMsg := "contract execution failed"
		if result.Error != nil {
			errMsg = result.Error.Message
		}
		return nil, fmt.Errorf("%s: gas_used=%d", errMsg, result.GasUsed)
	}

	return &types.MsgCallContractResponse{
		ReturnData: result.ReturnData,
		GasUsed:    result.GasUsed,
	}, nil
}

// ScheduleExecution schedules a future contract execution.
func (m msgServer) ScheduleExecution(goCtx context.Context, msg *types.MsgScheduleExecution) (*types.MsgScheduleExecutionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := m.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	contract, found := m.GetContract(ctx, msg.ContractAddress)
	if !found {
		return nil, types.ErrContractNotFound
	}
	if msg.Scheduler != contract.Creator {
		return nil, types.ErrNotContractCreator
	}

	// Determine target block from condition
	var executeAtBlock uint64
	if cond := msg.Condition; cond != nil {
		if bi := cond.GetBlockInterval(); bi != nil {
			executeAtBlock = bi.StartHeight
			if executeAtBlock == 0 {
				executeAtBlock = currentBlock + bi.EveryNBlocks
			}
		}
	}
	if executeAtBlock == 0 {
		executeAtBlock = currentBlock + 1
	}
	if executeAtBlock <= currentBlock {
		return nil, types.ErrScheduleInPast
	}

	// Check schedule limits
	scheduleCount := m.CountContractSchedules(ctx, msg.ContractAddress)
	if scheduleCount >= params.MaxSchedulesPerContract {
		return nil, types.ErrScheduleLimitReached
	}

	counter := m.GetNextScheduleId(ctx)
	scheduleId := fmt.Sprintf("sched-%d-%d", currentBlock, counter)

	schedule := &types.ContractSchedule{
		ScheduleId:      scheduleId,
		ContractAddress: msg.ContractAddress,
		Caller:          msg.Scheduler,
		Payload:         string(msg.InputData),
		ExecuteAtBlock:  executeAtBlock,
		MaxGas:          params.MaxScheduleGas,
		Executed:        false,
		Cancelled:       false,
	}
	m.SetSchedule(ctx, schedule)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.bvm.execution_scheduled",
		sdk.NewAttribute("schedule_id", scheduleId),
		sdk.NewAttribute("contract", msg.ContractAddress),
		sdk.NewAttribute("execute_at_block", fmt.Sprintf("%d", executeAtBlock)),
	))

	return &types.MsgScheduleExecutionResponse{ScheduleId: scheduleId}, nil
}

// ScheduleContract schedules a future contract execution (hand-written message type).
func (m msgServer) ScheduleContract(goCtx context.Context, msg *types.MsgScheduleContract) (*types.MsgScheduleContractResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := m.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	contract, found := m.GetContract(ctx, msg.ContractAddress)
	if !found {
		return nil, types.ErrContractNotFound
	}
	if msg.Caller != contract.Creator {
		return nil, types.ErrNotContractCreator
	}
	if msg.ExecuteAtBlock <= currentBlock {
		return nil, types.ErrScheduleInPast
	}

	scheduleCount := m.CountContractSchedules(ctx, msg.ContractAddress)
	if scheduleCount >= params.MaxSchedulesPerContract {
		return nil, types.ErrScheduleLimitReached
	}

	counter := m.GetNextScheduleId(ctx)
	scheduleId := fmt.Sprintf("sched-%d-%d", currentBlock, counter)

	schedule := &types.ContractSchedule{
		ScheduleId:      scheduleId,
		ContractAddress: msg.ContractAddress,
		Caller:          msg.Caller,
		Method:          msg.Method,
		Payload:         msg.Payload,
		ExecuteAtBlock:  msg.ExecuteAtBlock,
		MaxGas:          msg.MaxGas,
		Executed:        false,
		Cancelled:       false,
	}
	m.SetSchedule(ctx, schedule)

	// Snapshot caller's capabilities at schedule creation time (P1-2).
	if m.authKeeper != nil {
		var callerDID string
		if did, didFound := m.authKeeper.GetAccountDID(ctx, msg.Caller); didFound {
			callerDID = did
		}
		if callerDID != "" {
			var caps types.SessionCapabilities
			if sc, scFound := m.authKeeper.GetSessionCapabilities(ctx, msg.Caller, currentBlock); scFound {
				caps = sc
			} else {
				// Identity/operational key → full access
				caps = types.SessionCapabilities{
					CanTransfer: true, CanStake: true,
					CanSubmitClaims: true, CanVote: true,
				}
			}
			m.SetScheduleCapabilities(ctx, scheduleId, caps)
		}
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.bvm.contract_scheduled",
		sdk.NewAttribute("schedule_id", scheduleId),
		sdk.NewAttribute("contract", msg.ContractAddress),
		sdk.NewAttribute("execute_at_block", fmt.Sprintf("%d", msg.ExecuteAtBlock)),
	))

	return &types.MsgScheduleContractResponse{ScheduleId: scheduleId}, nil
}

// CancelSchedule cancels a pending scheduled execution.
func (m msgServer) CancelSchedule(goCtx context.Context, msg *types.MsgCancelSchedule) (*types.MsgCancelScheduleResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	schedule, found := m.GetSchedule(ctx, msg.ScheduleId)
	if !found {
		return nil, types.ErrScheduleNotFound
	}
	if schedule.Executed || schedule.Cancelled {
		return nil, types.ErrScheduleNotFound
	}
	if msg.Caller != schedule.Caller {
		return nil, types.ErrScheduleNotOwner
	}

	// Remove block index entry
	store := m.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.ScheduleBlockIndexKey(schedule.ExecuteAtBlock, schedule.ScheduleId))

	schedule.Cancelled = true
	m.SetSchedule(ctx, schedule)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.bvm.schedule_cancelled",
		sdk.NewAttribute("schedule_id", msg.ScheduleId),
		sdk.NewAttribute("caller", msg.Caller),
	))

	return &types.MsgCancelScheduleResponse{}, nil
}

// UpdateContractState updates contract state (governance only).
func (m msgServer) UpdateContractState(goCtx context.Context, msg *types.MsgUpdateContractState) (*types.MsgUpdateContractStateResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Authority != m.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.GetAuthority(), msg.Authority)
	}
	_, found := m.GetContract(ctx, msg.ContractAddress)
	if !found {
		return nil, types.ErrContractNotFound
	}
	m.SetContractState(ctx, msg.ContractAddress, msg.Key, msg.Value)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.bvm.update_contract_state",
			sdk.NewAttribute("contract_address", msg.ContractAddress),
			sdk.NewAttribute("caller", msg.Authority),
		),
	)

	return &types.MsgUpdateContractStateResponse{}, nil
}

// UpdateParams updates module params (governance only).
func (m msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.GetAuthority(), msg.Authority)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	m.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.bvm.update_params",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// ExecutePendingSchedules executes schedules due at the current block.
// Called from BeginBlock.
func (k Keeper) ExecutePendingSchedules(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	schedules := k.GetPendingSchedules(ctx, currentBlock)

	var totalGasUsed uint64

	for _, schedule := range schedules {
		if totalGasUsed >= types.MaxScheduledGasPerBlock {
			break
		}

		remainingBudget := types.MaxScheduledGasPerBlock - totalGasUsed
		effectiveMaxGas := schedule.MaxGas
		if effectiveMaxGas > remainingBudget {
			effectiveMaxGas = remainingBudget
		}

		// Remove block index entry
		kvStore := k.storeService.OpenKVStore(ctx)
		_ = kvStore.Delete(types.ScheduleBlockIndexKey(schedule.ExecuteAtBlock, schedule.ScheduleId))

		success := false
		var gasUsed uint64

		contract, found := k.GetContract(ctx, schedule.ContractAddress)
		if found {
			code, codeFound := k.GetCode(ctx, contract.CodeHash)
			if codeFound {
				callData := []byte(schedule.Payload)
				callerAddr, _ := sdk.AccAddressFromBech32(schedule.Caller)
				callerBytes := make([]byte, 20)
				copy(callerBytes, callerAddr)

				// Resolve scheduler's DID and use stored capabilities (P1-2).
				// Stored caps take priority (snapshot at creation time).
				// Fallback to full access for backwards compat (pre-P1-2 schedules).
				var schedCallerDID string
				var schedCaps *vm.SessionCapabilities
				if k.authKeeper != nil {
					if did, found := k.authKeeper.GetAccountDID(ctx, schedule.Caller); found {
						schedCallerDID = did
					}
				}
				if storedCaps, hasCaps := k.GetScheduleCapabilities(ctx, schedule.ScheduleId); hasCaps {
					// Use capabilities stored at schedule creation time
					schedCaps = &vm.SessionCapabilities{
						CanTransfer:     storedCaps.CanTransfer,
						CanStake:        storedCaps.CanStake,
						CanSubmitClaims: storedCaps.CanSubmitClaims,
						CanVote:         storedCaps.CanVote,
					}
				} else if k.authKeeper != nil && schedCallerDID != "" {
					// Backwards compat: no stored caps → full access (pre-P1-2 schedule)
					schedCaps = &vm.SessionCapabilities{
						CanTransfer: true, CanStake: true,
						CanSubmitClaims: true, CanVote: true,
					}
				}

				execCtx := &vm.ExecutionContext{
					Caller:             callerBytes,
					Origin:             callerBytes,
					CurrentContract:    []byte(schedule.ContractAddress),
					BlockNumber:        currentBlock,
					Timestamp:          uint64(ctx.BlockTime().Unix()),
					GasPrice:           new(big.Int),
					GasLimit:           effectiveMaxGas,
					Value:              new(big.Int),
					CallData:           callData,
					Mode:               vm.ModeCall,
					Bytecode:           code.Bytecode,
					ChainID:            big.NewInt(2521),
					ContractBvmVersion: contract.BvmVersion,
					CallerDID:          schedCallerDID,
					Capabilities:       schedCaps,
				}

				stateDB := vm.NewMemoryStateDB()
				var host vm.HostFunctions
				if k.knowledgeKeeper != nil || k.homeKeeper != nil {
					host = &zeroneHost{kk: k.knowledgeKeeper, hk: k.homeKeeper, ctx: ctx}
				}

				interp := vm.NewInterpreter()
				result, panicked := safeExecute(interp, code.Bytecode, execCtx, stateDB, host)
				if panicked {
					k.Logger(ctx).Error("BVM scheduled execution panicked",
						"schedule_id", schedule.ScheduleId,
						"contract", schedule.ContractAddress,
					)
				}

				success = result.Success
				gasUsed = result.GasUsed

				if result.Success {
					for _, change := range result.StateChanges {
						if change.Type == vm.StateChangeStorage {
							valBytes := ""
							if change.NewValue != nil {
								b32 := vm.WordToBytes32(change.NewValue)
								valBytes = hex.EncodeToString(b32[:])
							}
							k.SetContractState(ctx, schedule.ContractAddress, change.Key, valBytes)
						}
					}
				}
			}
		}

		totalGasUsed += gasUsed

		schedule.Executed = true
		k.SetSchedule(ctx, schedule)

		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.bvm.schedule_executed",
			sdk.NewAttribute("schedule_id", schedule.ScheduleId),
			sdk.NewAttribute("contract", schedule.ContractAddress),
			sdk.NewAttribute("block", fmt.Sprintf("%d", currentBlock)),
			sdk.NewAttribute("success", fmt.Sprintf("%v", success)),
			sdk.NewAttribute("gas_used", fmt.Sprintf("%d", gasUsed)),
		))
	}
}

// zeroneHost implements vm.HostFunctions by delegating to knowledge and home keepers.
type zeroneHost struct {
	kk  types.KnowledgeKeeper
	hk  types.HomeKeeper
	ctx sdk.Context
}

func (h *zeroneHost) KQuery(factId []byte) (bool, uint64, []byte) {
	if h.kk == nil {
		return false, 0, nil
	}
	factIdStr := hex.EncodeToString(factId)
	confidence, found := h.kk.GetFactConfidence(h.ctx, factIdStr)
	if !found {
		return false, 0, nil
	}
	return true, confidence, []byte(factIdStr)
}

func (h *zeroneHost) KVerify(_ string, _ []byte, _ []byte) bool {
	return false // Stub: verification voting requires full round integration
}

func (h *zeroneHost) KCite(_ string, _ []byte) bool {
	return true // Citation recording is fire-and-forget
}

func (h *zeroneHost) HQuery(callerAddr []byte) (bool, []byte, []byte) {
	if h.hk == nil {
		return false, nil, nil
	}
	addr := sdk.AccAddress(callerAddr).String()
	homeIDs := h.hk.GetHomesByOwner(h.ctx, addr)
	if len(homeIDs) == 0 {
		return false, nil, nil
	}
	homeID := homeIDs[0]
	status := h.hk.GetHomeStatus(h.ctx, homeID)
	return true, []byte(homeID), []byte(status)
}

func (h *zeroneHost) HMemory(homeId []byte) []byte {
	if h.hk == nil {
		return nil
	}
	homeIDStr := string(bytes.TrimRight(homeId, "\x00"))
	return []byte(h.hk.GetMemoryCID(h.ctx, homeIDStr))
}

func (h *zeroneHost) HPartner(homeId []byte) []byte {
	if h.hk == nil {
		return nil
	}
	homeIDStr := string(bytes.TrimRight(homeId, "\x00"))
	return []byte(h.hk.GetPartnershipID(h.ctx, homeIDStr))
}
