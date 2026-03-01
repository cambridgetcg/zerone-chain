package keeper

import (
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/bvm/types"
)

// --- Contract CRUD ---

func (k Keeper) SetContract(ctx sdk.Context, contract *types.DeployedContract) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(contract)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.ContractKey(contract.Address), bz); err != nil {
		panic(fmt.Sprintf("failed to set contract: %v", err))
	}
	if err := store.Set(types.ContractCreatorIndexKey(contract.Creator, contract.Address), []byte{1}); err != nil {
		panic(fmt.Sprintf("failed to set contract creator index: %v", err))
	}
}

func (k Keeper) GetContract(ctx sdk.Context, address string) (*types.DeployedContract, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContractKey(address))
	if err != nil || bz == nil {
		return nil, false
	}
	var contract types.DeployedContract
	if err := proto.Unmarshal(bz, &contract); err != nil {
		panic(err)
	}
	return &contract, true
}

func (k Keeper) DeleteContract(ctx sdk.Context, contract *types.DeployedContract) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.ContractKey(contract.Address))
	_ = store.Delete(types.ContractCreatorIndexKey(contract.Creator, contract.Address))

	if contract.CodeHash != "" {
		code, found := k.GetCode(ctx, contract.CodeHash)
		if found {
			if code.RefCount <= 1 {
				k.DeleteCode(ctx, contract.CodeHash)
			} else {
				code.RefCount--
				k.SetCode(ctx, code)
			}
		}
	}
}

func (k Keeper) IterateContracts(ctx sdk.Context, cb func(*types.DeployedContract) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ContractKeyPrefix, prefixEndBytes(types.ContractKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var contract types.DeployedContract
		if err := proto.Unmarshal(iter.Value(), &contract); err != nil {
			panic(err)
		}
		if cb(&contract) {
			break
		}
	}
}

func (k Keeper) GetContractsByCreator(ctx sdk.Context, creator string) []*types.DeployedContract {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.ContractCreatorPrefix(creator)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var contracts []*types.DeployedContract
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		address := string(key[len(prefix):])
		contract, found := k.GetContract(ctx, address)
		if found {
			contracts = append(contracts, contract)
		}
	}
	return contracts
}

func (k Keeper) GetNextContractNonce(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContractCounterKey)
	var counter uint64
	if err == nil && bz != nil {
		counter = binary.BigEndian.Uint64(bz)
	}
	next := counter + 1
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, next)
	_ = store.Set(types.ContractCounterKey, buf)
	return counter
}

// --- Code CRUD ---

func (k Keeper) SetCode(ctx sdk.Context, code *types.ContractCode) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(code)
	if err != nil {
		panic(err)
	}
	_ = store.Set(types.CodeKey(code.CodeHash), bz)
}

func (k Keeper) GetCode(ctx sdk.Context, codeHash string) (*types.ContractCode, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.CodeKey(codeHash))
	if err != nil || bz == nil {
		return nil, false
	}
	var code types.ContractCode
	if err := proto.Unmarshal(bz, &code); err != nil {
		panic(err)
	}
	return &code, true
}

func (k Keeper) DeleteCode(ctx sdk.Context, codeHash string) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.CodeKey(codeHash))
}

func (k Keeper) IterateCode(ctx sdk.Context, cb func(*types.ContractCode) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.CodeKeyPrefix, prefixEndBytes(types.CodeKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var code types.ContractCode
		if err := proto.Unmarshal(iter.Value(), &code); err != nil {
			panic(err)
		}
		if cb(&code) {
			break
		}
	}
}

// --- Contract State CRUD ---

func (k Keeper) SetContractState(ctx sdk.Context, contractAddress, key, value string) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Set(types.ContractStateKey(contractAddress, key), []byte(value))
}

func (k Keeper) GetContractState(ctx sdk.Context, contractAddress, key string) (string, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContractStateKey(contractAddress, key))
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

func (k Keeper) IterateContractState(ctx sdk.Context, contractAddress string, cb func(key, value string) bool) {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.ContractStatePrefix(contractAddress)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := string(iter.Key()[len(prefix):])
		value := string(iter.Value())
		if cb(key, value) {
			break
		}
	}
}

func (k Keeper) CountContractState(ctx sdk.Context, contractAddress string) uint64 {
	var count uint64
	k.IterateContractState(ctx, contractAddress, func(_, _ string) bool {
		count++
		return false
	})
	return count
}

// --- Schedule CRUD ---

func (k Keeper) SetSchedule(ctx sdk.Context, schedule *types.ContractSchedule) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(schedule)
	if err != nil {
		panic(err)
	}
	_ = store.Set(types.ScheduleKey(schedule.ScheduleId), bz)
	if !schedule.Executed && !schedule.Cancelled {
		_ = store.Set(types.ScheduleBlockIndexKey(schedule.ExecuteAtBlock, schedule.ScheduleId), []byte{1})
	}
}

func (k Keeper) GetSchedule(ctx sdk.Context, scheduleId string) (*types.ContractSchedule, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ScheduleKey(scheduleId))
	if err != nil || bz == nil {
		return nil, false
	}
	var schedule types.ContractSchedule
	if err := proto.Unmarshal(bz, &schedule); err != nil {
		panic(err)
	}
	return &schedule, true
}

func (k Keeper) DeleteSchedule(ctx sdk.Context, schedule *types.ContractSchedule) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.ScheduleKey(schedule.ScheduleId))
	_ = store.Delete(types.ScheduleBlockIndexKey(schedule.ExecuteAtBlock, schedule.ScheduleId))
}

func (k Keeper) IterateSchedules(ctx sdk.Context, cb func(*types.ContractSchedule) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ScheduleKeyPrefix, prefixEndBytes(types.ScheduleKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var schedule types.ContractSchedule
		if err := proto.Unmarshal(iter.Value(), &schedule); err != nil {
			panic(err)
		}
		if cb(&schedule) {
			break
		}
	}
}

func (k Keeper) GetPendingSchedules(ctx sdk.Context, upToBlock uint64) []*types.ContractSchedule {
	store := k.storeService.OpenKVStore(ctx)

	start := make([]byte, len(types.ScheduleBlockIndexPrefix))
	copy(start, types.ScheduleBlockIndexPrefix)

	end := make([]byte, 0, len(types.ScheduleBlockIndexPrefix)+8)
	end = append(end, types.ScheduleBlockIndexPrefix...)
	end = append(end, types.Uint64ToBytes(upToBlock+1)...)

	iter, err := store.Iterator(start, end)
	if err != nil {
		return nil
	}
	defer iter.Close()

	prefixLen := len(types.ScheduleBlockIndexPrefix) + 8

	var schedules []*types.ContractSchedule
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) <= prefixLen {
			continue
		}
		scheduleId := string(key[prefixLen:])
		schedule, found := k.GetSchedule(ctx, scheduleId)
		if found && !schedule.Executed && !schedule.Cancelled {
			schedules = append(schedules, schedule)
		}
	}
	return schedules
}

func (k Keeper) GetNextScheduleId(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ScheduleCounterKey)
	var counter uint64
	if err == nil && bz != nil {
		counter = binary.BigEndian.Uint64(bz)
	}
	next := counter + 1
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, next)
	_ = store.Set(types.ScheduleCounterKey, buf)
	return counter
}

func (k Keeper) CountContractSchedules(ctx sdk.Context, contractAddress string) uint64 {
	var count uint64
	k.IterateSchedules(ctx, func(s *types.ContractSchedule) bool {
		if s.ContractAddress == contractAddress && !s.Executed && !s.Cancelled {
			count++
		}
		return false
	})
	return count
}

// --- Schedule Capabilities ---

// SetScheduleCapabilities stores session capabilities snapshot for a schedule.
func (k Keeper) SetScheduleCapabilities(ctx sdk.Context, scheduleId string, caps types.SessionCapabilities) {
	store := k.storeService.OpenKVStore(ctx)
	bz := encodeCapabilities(caps)
	_ = store.Set(types.ScheduleCapabilityKey(scheduleId), bz)
}

// GetScheduleCapabilities retrieves stored session capabilities for a schedule.
func (k Keeper) GetScheduleCapabilities(ctx sdk.Context, scheduleId string) (types.SessionCapabilities, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ScheduleCapabilityKey(scheduleId))
	if err != nil || bz == nil || len(bz) < 4 {
		return types.SessionCapabilities{}, false
	}
	return decodeCapabilities(bz), true
}

func encodeCapabilities(caps types.SessionCapabilities) []byte {
	bz := make([]byte, 4)
	if caps.CanTransfer {
		bz[0] = 1
	}
	if caps.CanStake {
		bz[1] = 1
	}
	if caps.CanSubmitClaims {
		bz[2] = 1
	}
	if caps.CanVote {
		bz[3] = 1
	}
	return bz
}

func decodeCapabilities(bz []byte) types.SessionCapabilities {
	return types.SessionCapabilities{
		CanTransfer:     bz[0] == 1,
		CanStake:        bz[1] == 1,
		CanSubmitClaims: bz[2] == 1,
		CanVote:         bz[3] == 1,
	}
}

// --- Params ---

func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(err)
	}
	_ = store.Set(types.ParamsKey, bz)
}

func (k Keeper) GetParams(ctx sdk.Context) *types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil || bz == nil {
		p := types.DefaultParams()
		return &p
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		panic(err)
	}
	return &params
}
