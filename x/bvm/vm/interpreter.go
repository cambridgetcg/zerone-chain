package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"

	"golang.org/x/crypto/sha3"
)

// Interpreter is the BVM bytecode execution engine.
type Interpreter struct {
	stack         *Stack
	memory        *Memory
	gas           *GasMeter
	pc            int
	returnData    []byte
	logs          []Log
	changes       []StateChange
	callDepth     int
	stopped       bool
	accessTracker *AccessTracker
	jumpDests     map[int]bool
}

// NewInterpreter creates a new BVM interpreter.
func NewInterpreter() *Interpreter {
	return &Interpreter{
		stack:         NewStack(),
		memory:        NewMemory(),
		logs:          make([]Log, 0),
		changes:       make([]StateChange, 0),
		accessTracker: NewAccessTracker(),
		jumpDests:     make(map[int]bool),
	}
}

// Execute runs bytecode with the given context and returns the result.
func (interp *Interpreter) Execute(bytecode []byte, ctx *ExecutionContext, state StateDB, host HostFunctions) *ExecutionResult {
	interp.gas = NewGasMeter(ctx.GasLimit)
	interp.pc = 0
	interp.stopped = false
	interp.returnData = nil
	interp.logs = interp.logs[:0]
	interp.changes = interp.changes[:0]

	// Pre-warm caller, contract, origin addresses (EIP-2929)
	interp.accessTracker.MarkAddressWarm(ctx.Caller)
	interp.accessTracker.MarkAddressWarm(ctx.CurrentContract)
	interp.accessTracker.MarkAddressWarm(ctx.Origin)

	// Call depth check
	if interp.callDepth >= MaxCallDepth {
		return errorResult(ErrCodeCallDepthExceeded, "call depth exceeded")
	}

	// Pre-compute valid JUMPDEST locations
	interp.jumpDests = make(map[int]bool)
	for i := 0; i < len(bytecode); i++ {
		op := bytecode[i]
		if op == JUMPDEST {
			interp.jumpDests[i] = true
		}
		// Skip PUSH data bytes
		if op >= PUSH1 && op <= PUSH32 {
			i += int(op - PUSH1 + 1)
		}
	}

	// Main execution loop
	for interp.pc < len(bytecode) && !interp.stopped {
		op := bytecode[interp.pc]

		info, known := OpcodeTable[op]
		if !known {
			return errorResult(ErrCodeInvalidOpcode, "invalid opcode: 0x"+hex.EncodeToString([]byte{op}))
		}

		// Version gate
		if info.MinVersion > ctx.ContractBvmVersion {
			return errorResult(ErrCodeInvalidOpcode, "opcode requires BVM version "+uitoa(uint32ToUint64(info.MinVersion)))
		}

		// Static call guard: reject state modifiers
		if ctx.Mode == ModeStaticCall && info.IsStateModifier {
			return errorResult(ErrCodeStaticStateChange, "state modification in static call: "+info.Mnemonic)
		}

		// Gas cost (base)
		gasCost := info.GasCost
		if ctx.GasSchedule != nil {
			gasCost = ctx.GasSchedule.GasCostFor(op, gasCost)
		}
		if err := interp.gas.Consume(gasCost); err != nil {
			return &ExecutionResult{
				Success: false,
				GasUsed: interp.gas.Used(),
				Error:   &ExecutionError{Code: ErrCodeOutOfGas, Message: "out of gas at " + info.Mnemonic},
			}
		}

		// Dispatch opcode
		result := interp.executeOpcode(op, bytecode, ctx, state, host)
		if result != nil {
			result.GasUsed = interp.gas.Used()
			result.Logs = interp.logs
			result.StateChanges = interp.changes
			return result
		}
	}

	// Normal termination (fell through or STOP)
	return &ExecutionResult{
		Success:      true,
		ReturnData:   interp.returnData,
		GasUsed:      interp.gas.Used(),
		Logs:         interp.logs,
		StateChanges: interp.changes,
	}
}

func uint32ToUint64(v uint32) uint64 { return uint64(v) }

// executeOpcode dispatches a single opcode. Returns nil to continue, or a result to stop.
func (interp *Interpreter) executeOpcode(op byte, bytecode []byte, ctx *ExecutionContext, state StateDB, host HostFunctions) *ExecutionResult {
	switch op {
	case STOP:
		interp.stopped = true
		return nil

	// ─── Arithmetic ─────────────────────────────────────────────────────
	case ADD:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		result := Mod256(new(big.Int).Add(a, b))
		interp.stack.Push(result)
		interp.pc++

	case MUL:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		result := Mod256(new(big.Int).Mul(a, b))
		interp.stack.Push(result)
		interp.pc++

	case SUB:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		result := Mod256(new(big.Int).Sub(a, b))
		interp.stack.Push(result)
		interp.pc++

	case DIV:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if b.Sign() == 0 {
			interp.stack.Push(new(big.Int))
		} else {
			interp.stack.Push(new(big.Int).Div(a, b))
		}
		interp.pc++

	case SDIV:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if b.Sign() == 0 {
			interp.stack.Push(new(big.Int))
		} else {
			sa := ToSigned(a)
			sb := ToSigned(b)
			result := new(big.Int).Quo(sa, sb)
			interp.stack.Push(FromSigned(result))
		}
		interp.pc++

	case MOD:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if b.Sign() == 0 {
			interp.stack.Push(new(big.Int))
		} else {
			interp.stack.Push(new(big.Int).Mod(a, b))
		}
		interp.pc++

	case SMOD:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if b.Sign() == 0 {
			interp.stack.Push(new(big.Int))
		} else {
			sa := ToSigned(a)
			sb := ToSigned(b)
			result := new(big.Int).Rem(sa, sb)
			interp.stack.Push(FromSigned(result))
		}
		interp.pc++

	case ADDMOD:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		n, _ := interp.stack.Pop()
		if n.Sign() == 0 {
			interp.stack.Push(new(big.Int))
		} else {
			sum := new(big.Int).Add(a, b)
			interp.stack.Push(new(big.Int).Mod(sum, n))
		}
		interp.pc++

	case MULMOD:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		n, _ := interp.stack.Pop()
		if n.Sign() == 0 {
			interp.stack.Push(new(big.Int))
		} else {
			prod := new(big.Int).Mul(a, b)
			interp.stack.Push(new(big.Int).Mod(prod, n))
		}
		interp.pc++

	case EXP:
		base, _ := interp.stack.Pop()
		exp, _ := interp.stack.Pop()
		// Dynamic gas: 50 per byte of exponent
		expBytes := exp.Bytes()
		if len(expBytes) > 0 {
			interp.gas.Consume(GasExpPerByte * uint64(len(expBytes)))
		}
		result := new(big.Int).Exp(base, exp, WordMod)
		interp.stack.Push(result)
		interp.pc++

	case SIGNEXTEND:
		b, _ := interp.stack.Pop()
		x, _ := interp.stack.Pop()
		if b.Cmp(big.NewInt(31)) < 0 {
			bit := uint(b.Uint64()*8 + 7)
			mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), bit), big.NewInt(1))
			if x.Bit(int(bit)) == 1 {
				x.Or(x, new(big.Int).Xor(MaxUint256, mask))
			} else {
				x.And(x, mask)
			}
		}
		interp.stack.Push(Mod256(x))
		interp.pc++

	// ─── Comparison ─────────────────────────────────────────────────────
	case LT:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if a.Cmp(b) < 0 {
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case GT:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if a.Cmp(b) > 0 {
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case SLT:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if ToSigned(a).Cmp(ToSigned(b)) < 0 {
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case SGT:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if ToSigned(a).Cmp(ToSigned(b)) > 0 {
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case EQ:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		if a.Cmp(b) == 0 {
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case ISZERO:
		a, _ := interp.stack.Pop()
		if a.Sign() == 0 {
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	// ─── Bitwise ────────────────────────────────────────────────────────
	case AND:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		interp.stack.Push(new(big.Int).And(a, b))
		interp.pc++

	case OR:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		interp.stack.Push(new(big.Int).Or(a, b))
		interp.pc++

	case XOR:
		a, _ := interp.stack.Pop()
		b, _ := interp.stack.Pop()
		interp.stack.Push(new(big.Int).Xor(a, b))
		interp.pc++

	case NOT:
		a, _ := interp.stack.Pop()
		interp.stack.Push(new(big.Int).And(new(big.Int).Not(a), MaxUint256))
		interp.pc++

	case BYTE:
		i, _ := interp.stack.Pop()
		x, _ := interp.stack.Pop()
		if i.Cmp(big.NewInt(32)) >= 0 {
			interp.stack.Push(new(big.Int))
		} else {
			b32 := WordToBytes32(x)
			interp.stack.Push(new(big.Int).SetUint64(uint64(b32[i.Uint64()])))
		}
		interp.pc++

	case SHL:
		shift, _ := interp.stack.Pop()
		val, _ := interp.stack.Pop()
		if shift.Cmp(big.NewInt(256)) >= 0 {
			interp.stack.Push(new(big.Int))
		} else {
			result := new(big.Int).Lsh(val, uint(shift.Uint64()))
			interp.stack.Push(Mod256(result))
		}
		interp.pc++

	case SHR:
		shift, _ := interp.stack.Pop()
		val, _ := interp.stack.Pop()
		if shift.Cmp(big.NewInt(256)) >= 0 {
			interp.stack.Push(new(big.Int))
		} else {
			interp.stack.Push(new(big.Int).Rsh(val, uint(shift.Uint64())))
		}
		interp.pc++

	case SAR:
		shift, _ := interp.stack.Pop()
		val, _ := interp.stack.Pop()
		signed := ToSigned(val)
		if shift.Cmp(big.NewInt(256)) >= 0 {
			if signed.Sign() < 0 {
				interp.stack.Push(new(big.Int).Set(MaxUint256))
			} else {
				interp.stack.Push(new(big.Int))
			}
		} else {
			result := new(big.Int).Rsh(signed, uint(shift.Uint64()))
			interp.stack.Push(FromSigned(result))
		}
		interp.pc++

	// ─── SHA3 ───────────────────────────────────────────────────────────
	case SHA3:
		offset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		off := int(offset.Uint64())
		ln := int(length.Uint64())
		// Memory expansion gas
		if ln > 0 {
			memGas, err := interp.memory.Expand(off + ln)
			if err != nil {
				return errorResult(ErrCodeExecutionFailed, "memory error: "+err.Error())
			}
			interp.gas.Consume(memGas)
			interp.gas.Consume(GasSHA3PerWord * toWordCount(ln))
		}
		data := interp.memory.Read(off, ln)
		h := sha3.NewLegacyKeccak256()
		h.Write(data)
		hash := h.Sum(nil)
		interp.stack.Push(new(big.Int).SetBytes(hash))
		interp.pc++

	// ─── Environmental ──────────────────────────────────────────────────
	case ADDRESS:
		interp.stack.Push(new(big.Int).SetBytes(ctx.CurrentContract))
		interp.pc++

	case BALANCE:
		addr, _ := interp.stack.Pop()
		addrBytes := WordToBytes20(addr)
		interp.gas.Consume(interp.accessTracker.AddressGas(addrBytes))
		bal := state.GetBalance(addrBytes)
		interp.stack.Push(bal)
		interp.pc++

	case ORIGIN:
		interp.stack.Push(new(big.Int).SetBytes(ctx.Origin))
		interp.pc++

	case CALLER:
		interp.stack.Push(new(big.Int).SetBytes(ctx.Caller))
		interp.pc++

	case CALLVALUE:
		val := new(big.Int)
		if ctx.Value != nil {
			val.Set(ctx.Value)
		}
		interp.stack.Push(val)
		interp.pc++

	case CALLDATALOAD:
		i, _ := interp.stack.Pop()
		offset := int(i.Uint64())
		var data [32]byte
		if offset < len(ctx.CallData) {
			end := offset + 32
			if end > len(ctx.CallData) {
				end = len(ctx.CallData)
			}
			copy(data[0:end-offset], ctx.CallData[offset:end])
		}
		interp.stack.Push(new(big.Int).SetBytes(data[:]))
		interp.pc++

	case CALLDATASIZE:
		interp.stack.Push(big.NewInt(int64(len(ctx.CallData))))
		interp.pc++

	case CALLDATACOPY:
		destOffset, _ := interp.stack.Pop()
		dataOffset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		dOff := int(destOffset.Uint64())
		sOff := int(dataOffset.Uint64())
		ln := int(length.Uint64())
		if ln > 0 {
			memGas, err := interp.memory.Expand(dOff + ln)
			if err != nil {
				return errorResult(ErrCodeExecutionFailed, "memory error: "+err.Error())
			}
			interp.gas.Consume(memGas)
			interp.gas.Consume(GasCopy * toWordCount(ln))
			data := make([]byte, ln)
			if sOff < len(ctx.CallData) {
				end := sOff + ln
				if end > len(ctx.CallData) {
					end = len(ctx.CallData)
				}
				copy(data, ctx.CallData[sOff:end])
			}
			interp.memory.Write(dOff, data)
		}
		interp.pc++

	case CODESIZE:
		interp.stack.Push(big.NewInt(int64(len(bytecode))))
		interp.pc++

	case CODECOPY:
		destOffset, _ := interp.stack.Pop()
		codeOffset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		dOff := int(destOffset.Uint64())
		cOff := int(codeOffset.Uint64())
		ln := int(length.Uint64())
		if ln > 0 {
			memGas, err := interp.memory.Expand(dOff + ln)
			if err != nil {
				return errorResult(ErrCodeExecutionFailed, "memory error: "+err.Error())
			}
			interp.gas.Consume(memGas)
			interp.gas.Consume(GasCopy * toWordCount(ln))
			data := make([]byte, ln)
			if cOff < len(bytecode) {
				end := cOff + ln
				if end > len(bytecode) {
					end = len(bytecode)
				}
				copy(data, bytecode[cOff:end])
			}
			interp.memory.Write(dOff, data)
		}
		interp.pc++

	case GASPRICE:
		gp := new(big.Int)
		if ctx.GasPrice != nil {
			gp.Set(ctx.GasPrice)
		}
		interp.stack.Push(gp)
		interp.pc++

	case RETURNDATASIZE:
		interp.stack.Push(big.NewInt(int64(len(interp.returnData))))
		interp.pc++

	case RETURNDATACOPY:
		destOffset, _ := interp.stack.Pop()
		dataOffset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		dOff := int(destOffset.Uint64())
		sOff := int(dataOffset.Uint64())
		ln := int(length.Uint64())
		if sOff+ln > len(interp.returnData) {
			return errorResult(ErrCodeReturnDataOutOfBounds, "return data out of bounds")
		}
		if ln > 0 {
			memGas, _ := interp.memory.Expand(dOff + ln)
			interp.gas.Consume(memGas)
			interp.gas.Consume(GasCopy * toWordCount(ln))
			interp.memory.Write(dOff, interp.returnData[sOff:sOff+ln])
		}
		interp.pc++

	// ─── Block Information ──────────────────────────────────────────────
	case BLOCKHASH:
		interp.stack.Pop() // block number (not available in deterministic BVM)
		interp.stack.Push(new(big.Int))
		interp.pc++

	case COINBASE:
		cb := new(big.Int)
		if ctx.Coinbase != nil {
			cb.SetBytes(ctx.Coinbase)
		}
		interp.stack.Push(cb)
		interp.pc++

	case TIMESTAMP:
		interp.stack.Push(new(big.Int).SetUint64(ctx.Timestamp))
		interp.pc++

	case NUMBER:
		interp.stack.Push(new(big.Int).SetUint64(ctx.BlockNumber))
		interp.pc++

	case PREVRANDAO:
		pr := new(big.Int)
		if ctx.PrevRandao != nil {
			pr.Set(ctx.PrevRandao)
		}
		interp.stack.Push(pr)
		interp.pc++

	case GASLIMIT:
		interp.stack.Push(new(big.Int).SetUint64(ctx.GasLimit))
		interp.pc++

	case CHAINID:
		cid := new(big.Int)
		if ctx.ChainID != nil {
			cid.Set(ctx.ChainID)
		}
		interp.stack.Push(cid)
		interp.pc++

	case SELFBALANCE:
		bal := state.GetBalance(ctx.CurrentContract)
		interp.stack.Push(bal)
		interp.pc++

	case BASEFEE:
		bf := new(big.Int)
		if ctx.BaseFee != nil {
			bf.Set(ctx.BaseFee)
		}
		interp.stack.Push(bf)
		interp.pc++

	// ─── Stack, Memory, Storage ─────────────────────────────────────────
	case POP:
		interp.stack.Pop()
		interp.pc++

	case MLOAD:
		offset, _ := interp.stack.Pop()
		off := int(offset.Uint64())
		memGas, err := interp.memory.Expand(off + 32)
		if err != nil {
			return errorResult(ErrCodeExecutionFailed, "memory error: "+err.Error())
		}
		interp.gas.Consume(memGas)
		val := interp.memory.MLoad(off)
		interp.stack.Push(val)
		interp.pc++

	case MSTORE:
		offset, _ := interp.stack.Pop()
		val, _ := interp.stack.Pop()
		off := int(offset.Uint64())
		memGas, err := interp.memory.Expand(off + 32)
		if err != nil {
			return errorResult(ErrCodeExecutionFailed, "memory error: "+err.Error())
		}
		interp.gas.Consume(memGas)
		interp.memory.MStore(off, val)
		interp.pc++

	case MSTORE8:
		offset, _ := interp.stack.Pop()
		val, _ := interp.stack.Pop()
		off := int(offset.Uint64())
		memGas, err := interp.memory.Expand(off + 1)
		if err != nil {
			return errorResult(ErrCodeExecutionFailed, "memory error: "+err.Error())
		}
		interp.gas.Consume(memGas)
		interp.memory.SetByte(off, byte(val.Uint64()&0xFF))
		interp.pc++

	case SLOAD:
		keyWord, _ := interp.stack.Pop()
		keyBytes := WordToBytes32(keyWord)
		// EIP-2929 cold/warm gas
		interp.gas.Consume(interp.accessTracker.StorageGas(ctx.CurrentContract, keyBytes[:]))
		valBytes := state.GetStorage(ctx.CurrentContract, keyBytes[:])
		if valBytes == nil {
			interp.stack.Push(new(big.Int))
		} else {
			interp.stack.Push(new(big.Int).SetBytes(valBytes))
		}
		interp.pc++

	case SSTORE:
		keyWord, _ := interp.stack.Pop()
		valWord, _ := interp.stack.Pop()
		keyBytes := WordToBytes32(keyWord)
		valBytes := WordToBytes32(valWord)
		keyHex := hex.EncodeToString(keyBytes[:])
		// Track state change
		oldVal := state.GetStorage(ctx.CurrentContract, keyBytes[:])
		state.SetStorage(ctx.CurrentContract, keyBytes[:], valBytes[:])
		var oldBigInt *big.Int
		if oldVal != nil {
			oldBigInt = new(big.Int).SetBytes(oldVal)
		}
		interp.changes = append(interp.changes, StateChange{
			Type:     StateChangeStorage,
			Address:  ctx.CurrentContract,
			Key:      keyHex,
			OldValue: oldBigInt,
			NewValue: new(big.Int).Set(valWord),
		})
		interp.pc++

	// ─── Control Flow ───────────────────────────────────────────────────
	case JUMP:
		dest, _ := interp.stack.Pop()
		destInt := int(dest.Uint64())
		if !interp.jumpDests[destInt] {
			return errorResult(ErrCodeInvalidJump, "invalid jump destination")
		}
		interp.pc = destInt

	case JUMPI:
		dest, _ := interp.stack.Pop()
		cond, _ := interp.stack.Pop()
		if cond.Sign() != 0 {
			destInt := int(dest.Uint64())
			if !interp.jumpDests[destInt] {
				return errorResult(ErrCodeInvalidJump, "invalid jump destination")
			}
			interp.pc = destInt
		} else {
			interp.pc++
		}

	case PC:
		interp.stack.Push(big.NewInt(int64(interp.pc)))
		interp.pc++

	case MSIZE:
		interp.stack.Push(big.NewInt(int64(interp.memory.Size())))
		interp.pc++

	case GAS:
		interp.stack.Push(new(big.Int).SetUint64(interp.gas.Remaining()))
		interp.pc++

	case JUMPDEST:
		interp.pc++

	// ─── Push ───────────────────────────────────────────────────────────
	case PUSH0:
		interp.stack.Push(new(big.Int))
		interp.pc++

	// ─── Return / Revert ────────────────────────────────────────────────
	case RETURN:
		offset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		off := int(offset.Uint64())
		ln := int(length.Uint64())
		if ln > 0 {
			memGas, _ := interp.memory.Expand(off + ln)
			interp.gas.Consume(memGas)
		}
		data := interp.memory.Read(off, ln)
		return &ExecutionResult{
			Success:    true,
			ReturnData: data,
		}

	case REVERT:
		offset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		off := int(offset.Uint64())
		ln := int(length.Uint64())
		if ln > 0 {
			memGas, _ := interp.memory.Expand(off + ln)
			interp.gas.Consume(memGas)
		}
		data := interp.memory.Read(off, ln)
		return &ExecutionResult{
			Success:    false,
			ReturnData: data,
			Error:      &ExecutionError{Code: ErrCodeRevert, Message: "execution reverted"},
		}

	case INVALID:
		return errorResult(ErrCodeInvalidOpcode, "invalid instruction")

	// ─── Logging ────────────────────────────────────────────────────────
	case LOG0, LOG1, LOG2, LOG3, LOG4:
		numTopics := int(op - LOG0)
		offset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		off := int(offset.Uint64())
		ln := int(length.Uint64())
		if ln > 0 {
			memGas, _ := interp.memory.Expand(off + ln)
			interp.gas.Consume(memGas)
			interp.gas.Consume(GasLogData * uint64(ln))
		}
		topics := make([][]byte, numTopics)
		for i := 0; i < numTopics; i++ {
			t, _ := interp.stack.Pop()
			b := WordToBytes32(t)
			topics[i] = b[:]
		}
		data := interp.memory.Read(off, ln)
		interp.logs = append(interp.logs, Log{
			Address: ctx.CurrentContract,
			Topics:  topics,
			Data:    data,
		})
		interp.pc++

	// ─── Knowledge Bridge ───────────────────────────────────────────────
	case KQUERY:
		factIdWord, _ := interp.stack.Pop()
		factIdBytes := WordToBytes32(factIdWord)
		if host != nil {
			exists, confidence, _ := host.KQuery(factIdBytes[:])
			if exists {
				interp.stack.Push(big.NewInt(1))
			} else {
				interp.stack.Push(new(big.Int))
			}
			interp.stack.Push(new(big.Int).SetUint64(confidence))
		} else {
			interp.stack.Push(new(big.Int))
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case KVERIFY:
		claimIdWord, _ := interp.stack.Pop()
		voteHashWord, _ := interp.stack.Pop()
		claimIdBytes := WordToBytes32(claimIdWord)
		voteHashBytes := WordToBytes32(voteHashWord)
		if host != nil {
			ok := host.KVerify("", claimIdBytes[:], voteHashBytes[:])
			if ok {
				interp.stack.Push(big.NewInt(1))
			} else {
				interp.stack.Push(new(big.Int))
			}
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case KCITE:
		factIdWord, _ := interp.stack.Pop()
		factIdBytes := WordToBytes32(factIdWord)
		if host != nil {
			ok := host.KCite("", factIdBytes[:])
			if ok {
				interp.stack.Push(big.NewInt(1))
			} else {
				interp.stack.Push(new(big.Int))
			}
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	// ─── Home Bridge ───────────────────────────────────────────────────
	case HQUERY:
		callerWord, _ := interp.stack.Pop()
		callerBytes := WordToBytes20(callerWord)
		if host != nil {
			hasHome, homeId, status := host.HQuery(callerBytes)
			if hasHome {
				interp.stack.Push(big.NewInt(1))
				interp.stack.Push(new(big.Int).SetBytes(homeId))
				interp.stack.Push(new(big.Int).SetBytes(status))
			} else {
				interp.stack.Push(new(big.Int))
				interp.stack.Push(new(big.Int))
				interp.stack.Push(new(big.Int))
			}
		} else {
			interp.stack.Push(new(big.Int))
			interp.stack.Push(new(big.Int))
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case HMEMORY:
		homeIdWord, _ := interp.stack.Pop()
		homeIdBytes := WordToBytes32(homeIdWord)
		if host != nil {
			cid := host.HMemory(homeIdBytes[:])
			interp.stack.Push(new(big.Int).SetBytes(cid))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case HPARTNER:
		homeIdWord, _ := interp.stack.Pop()
		homeIdBytes := WordToBytes32(homeIdWord)
		if host != nil {
			pid := host.HPartner(homeIdBytes[:])
			interp.stack.Push(new(big.Int).SetBytes(pid))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	// ─── System (CALL, CREATE, etc.) ────────────────────────────────────
	case CALL:
		gasWord, _ := interp.stack.Pop()
		addrWord, _ := interp.stack.Pop()
		valueWord, _ := interp.stack.Pop()
		argsOffset, _ := interp.stack.Pop()
		argsLength, _ := interp.stack.Pop()
		retOffset, _ := interp.stack.Pop()
		retLength, _ := interp.stack.Pop()

		targetAddr := WordToBytes20(addrWord)
		callData := interp.memory.Read(int(argsOffset.Uint64()), int(argsLength.Uint64()))
		gasForward := CalculateForwardGas(interp.gas.Remaining(), gasWord.Uint64())

		result := ExecuteCall(ModeCall, ctx, targetAddr, callData, gasForward, valueWord, state, host, interp.callDepth)
		interp.returnData = result.ReturnData
		interp.gas.Consume(result.GasUsed)

		// Copy return data to memory
		if result.ReturnData != nil && retLength.Sign() > 0 {
			rOff := int(retOffset.Uint64())
			rLen := int(retLength.Uint64())
			memGas, _ := interp.memory.Expand(rOff + rLen)
			interp.gas.Consume(memGas)
			cpLen := rLen
			if cpLen > len(result.ReturnData) {
				cpLen = len(result.ReturnData)
			}
			interp.memory.Write(rOff, result.ReturnData[:cpLen])
		}

		// Collect sub-call state changes
		if result.Success {
			interp.changes = append(interp.changes, result.StateChanges...)
			interp.logs = append(interp.logs, result.Logs...)
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case STATICCALL:
		gasWord, _ := interp.stack.Pop()
		addrWord, _ := interp.stack.Pop()
		argsOffset, _ := interp.stack.Pop()
		argsLength, _ := interp.stack.Pop()
		retOffset, _ := interp.stack.Pop()
		retLength, _ := interp.stack.Pop()

		targetAddr := WordToBytes20(addrWord)
		callData := interp.memory.Read(int(argsOffset.Uint64()), int(argsLength.Uint64()))
		gasForward := CalculateForwardGas(interp.gas.Remaining(), gasWord.Uint64())

		result := ExecuteCall(ModeStaticCall, ctx, targetAddr, callData, gasForward, new(big.Int), state, host, interp.callDepth)
		interp.returnData = result.ReturnData
		interp.gas.Consume(result.GasUsed)

		if result.ReturnData != nil && retLength.Sign() > 0 {
			rOff := int(retOffset.Uint64())
			rLen := int(retLength.Uint64())
			memGas, _ := interp.memory.Expand(rOff + rLen)
			interp.gas.Consume(memGas)
			cpLen := rLen
			if cpLen > len(result.ReturnData) {
				cpLen = len(result.ReturnData)
			}
			interp.memory.Write(rOff, result.ReturnData[:cpLen])
		}

		if result.Success {
			interp.logs = append(interp.logs, result.Logs...)
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case DELEGATECALL:
		gasWord, _ := interp.stack.Pop()
		addrWord, _ := interp.stack.Pop()
		argsOffset, _ := interp.stack.Pop()
		argsLength, _ := interp.stack.Pop()
		retOffset, _ := interp.stack.Pop()
		retLength, _ := interp.stack.Pop()

		targetAddr := WordToBytes20(addrWord)
		callData := interp.memory.Read(int(argsOffset.Uint64()), int(argsLength.Uint64()))
		gasForward := CalculateForwardGas(interp.gas.Remaining(), gasWord.Uint64())

		result := ExecuteCall(ModeDelegateCall, ctx, targetAddr, callData, gasForward, ctx.Value, state, host, interp.callDepth)
		interp.returnData = result.ReturnData
		interp.gas.Consume(result.GasUsed)

		if result.ReturnData != nil && retLength.Sign() > 0 {
			rOff := int(retOffset.Uint64())
			rLen := int(retLength.Uint64())
			memGas, _ := interp.memory.Expand(rOff + rLen)
			interp.gas.Consume(memGas)
			cpLen := rLen
			if cpLen > len(result.ReturnData) {
				cpLen = len(result.ReturnData)
			}
			interp.memory.Write(rOff, result.ReturnData[:cpLen])
		}

		if result.Success {
			interp.changes = append(interp.changes, result.StateChanges...)
			interp.logs = append(interp.logs, result.Logs...)
			interp.stack.Push(big.NewInt(1))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case CREATE:
		valueWord, _ := interp.stack.Pop()
		offset, _ := interp.stack.Pop()
		length, _ := interp.stack.Pop()
		initCode := interp.memory.Read(int(offset.Uint64()), int(length.Uint64()))
		nonce := uint64(0)
		contractAddr := createAddress(ctx.CurrentContract, nonce)
		result := ExecuteCall(ModeCreate, ctx, contractAddr, initCode, interp.gas.Remaining()/2, valueWord, state, host, interp.callDepth)
		interp.gas.Consume(result.GasUsed)
		if result.Success {
			interp.stack.Push(new(big.Int).SetBytes(contractAddr))
		} else {
			interp.stack.Push(new(big.Int))
		}
		interp.pc++

	case SELFDESTRUCT:
		interp.stack.Pop() // beneficiary
		interp.stopped = true

	default:
		// Handle PUSH1-PUSH32
		if op >= PUSH1 && op <= PUSH32 {
			numBytes := int(op - PUSH1 + 1)
			val := new(big.Int)
			if interp.pc+1+numBytes <= len(bytecode) {
				val.SetBytes(bytecode[interp.pc+1 : interp.pc+1+numBytes])
			}
			interp.stack.Push(val)
			interp.pc += 1 + numBytes
			return nil
		}

		// Handle DUP1-DUP16
		if op >= DUP1 && op <= DUP16 {
			depth := int(op - DUP1)
			if err := interp.stack.Dup(depth); err != nil {
				return errorResult(ErrCodeStackUnderflow, "stack underflow in "+OpcodeTable[op].Mnemonic)
			}
			interp.pc++
			return nil
		}

		// Handle SWAP1-SWAP16
		if op >= SWAP1 && op <= SWAP16 {
			depth := int(op-SWAP1) + 1
			if err := interp.stack.Swap(depth); err != nil {
				return errorResult(ErrCodeStackUnderflow, "stack underflow in "+OpcodeTable[op].Mnemonic)
			}
			interp.pc++
			return nil
		}

		return errorResult(ErrCodeInvalidOpcode, "unhandled opcode: 0x"+hex.EncodeToString([]byte{op}))
	}

	return nil
}

func createAddress(sender []byte, nonce uint64) []byte {
	data := make([]byte, len(sender)+8)
	copy(data, sender)
	for i := 7; i >= 0; i-- {
		data[len(sender)+i] = byte(nonce)
		nonce >>= 8
	}
	h := sha256.Sum256(data)
	return h[:20]
}

func errorResult(code ErrorCode, msg string) *ExecutionResult {
	return &ExecutionResult{
		Success: false,
		Error:   &ExecutionError{Code: code, Message: msg},
	}
}

// ExecuteCall performs a sub-call with a new interpreter instance.
func ExecuteCall(
	mode ExecutionMode,
	callerCtx *ExecutionContext,
	targetAddr []byte,
	callData []byte,
	gasForward uint64,
	value *big.Int,
	state StateDB,
	host HostFunctions,
	callDepth int,
) *ExecutionResult {
	if callDepth >= MaxCallDepth {
		return errorResult(ErrCodeCallDepthExceeded, "call depth exceeded")
	}

	var bytecodeToRun []byte
	var currentContract []byte
	var caller []byte

	switch mode {
	case ModeCall:
		bytecodeToRun = state.GetCode(targetAddr)
		currentContract = targetAddr
		caller = callerCtx.CurrentContract
	case ModeStaticCall:
		bytecodeToRun = state.GetCode(targetAddr)
		currentContract = targetAddr
		caller = callerCtx.CurrentContract
	case ModeDelegateCall:
		bytecodeToRun = state.GetCode(targetAddr)
		currentContract = callerCtx.CurrentContract
		caller = callerCtx.Caller
	case ModeCallCode:
		bytecodeToRun = state.GetCode(targetAddr)
		currentContract = callerCtx.CurrentContract
		caller = callerCtx.CurrentContract
	case ModeCreate:
		bytecodeToRun = callData
		currentContract = targetAddr
		caller = callerCtx.CurrentContract
		callData = nil
	}

	if len(bytecodeToRun) == 0 && mode != ModeCreate {
		return &ExecutionResult{Success: true, ReturnData: nil, GasUsed: 0}
	}

	subCtx := &ExecutionContext{
		Caller:             caller,
		Origin:             callerCtx.Origin,
		CurrentContract:    currentContract,
		BlockNumber:        callerCtx.BlockNumber,
		Timestamp:          callerCtx.Timestamp,
		GasPrice:           callerCtx.GasPrice,
		GasLimit:           gasForward,
		Value:              value,
		CallData:           callData,
		Mode:               mode,
		Bytecode:           bytecodeToRun,
		ChainID:            callerCtx.ChainID,
		Coinbase:           callerCtx.Coinbase,
		PrevRandao:         callerCtx.PrevRandao,
		BaseFee:            callerCtx.BaseFee,
		ContractBvmVersion: callerCtx.ContractBvmVersion,
		GasSchedule:        callerCtx.GasSchedule,
	}

	subInterp := NewInterpreter()
	subInterp.callDepth = callDepth + 1
	return subInterp.Execute(bytecodeToRun, subCtx, state, host)
}

// CalculateForwardGas computes gas to forward per 63/64 rule (EIP-150).
func CalculateForwardGas(available uint64, requested uint64) uint64 {
	maxForward := available - available/64
	if requested == 0 || requested > maxForward {
		return maxForward
	}
	return requested
}
