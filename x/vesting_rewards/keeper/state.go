package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// SetVestingSchedule stores a vesting schedule with all indexes.
func (k Keeper) SetVestingSchedule(ctx sdk.Context, schedule *types.VestingSchedule) {
	store := k.storeService.OpenKVStore(ctx)

	bz, err := proto.Marshal(schedule)
	if err != nil {
		panic("failed to marshal vesting schedule: " + err.Error())
	}

	key := append(types.VestingScheduleKeyPrefix, []byte(schedule.Id)...)
	if err := store.Set(key, bz); err != nil {
		panic("failed to set vesting schedule: " + err.Error())
	}

	if schedule.ClaimId != "" {
		claimKey := append(types.ClaimRecordKeyPrefix, []byte(schedule.ClaimId)...)
		if err := store.Set(claimKey, []byte(schedule.Id)); err != nil {
			panic("failed to set claim index: " + err.Error())
		}
	}

	recipientKey := append(types.VestingByRecipientPrefix, []byte(schedule.Recipient+"/"+schedule.Id)...)
	if err := store.Set(recipientKey, []byte{1}); err != nil {
		panic("failed to set recipient index: " + err.Error())
	}

	if schedule.Status == string(types.VestingStatusActive) || schedule.Status == string(types.VestingStatusPaused) {
		activeKey := append(types.ActiveVestingPrefix, []byte(schedule.Id)...)
		if err := store.Set(activeKey, []byte{1}); err != nil {
			panic("failed to set active index: " + err.Error())
		}
	} else {
		activeKey := append(types.ActiveVestingPrefix, []byte(schedule.Id)...)
		if err := store.Delete(activeKey); err != nil {
			panic("failed to delete active index: " + err.Error())
		}
	}
}

// GetVestingSchedule retrieves a vesting schedule by ID.
func (k Keeper) GetVestingSchedule(ctx sdk.Context, vestingId string) (*types.VestingSchedule, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := append(types.VestingScheduleKeyPrefix, []byte(vestingId)...)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var schedule types.VestingSchedule
	if err := proto.Unmarshal(bz, &schedule); err != nil {
		return nil, false
	}
	return &schedule, true
}

// GetVestingByClaimId retrieves the vesting schedule for a claim.
func (k Keeper) GetVestingByClaimId(ctx sdk.Context, claimId string) (*types.VestingSchedule, bool) {
	store := k.storeService.OpenKVStore(ctx)
	claimKey := append(types.ClaimRecordKeyPrefix, []byte(claimId)...)
	vestingIdBz, err := store.Get(claimKey)
	if err != nil || vestingIdBz == nil {
		return nil, false
	}
	return k.GetVestingSchedule(ctx, string(vestingIdBz))
}

// GetVestingSchedulesByRecipient returns all vesting schedules for a recipient.
func (k Keeper) GetVestingSchedulesByRecipient(ctx sdk.Context, recipient string) []*types.VestingSchedule {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(types.VestingByRecipientPrefix, []byte(recipient+"/")...)

	var schedules []*types.VestingSchedule
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return schedules
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		keyStr := string(iter.Key())
		vestingId := keyStr[len(prefix):]
		schedule, found := k.GetVestingSchedule(ctx, vestingId)
		if found {
			schedules = append(schedules, schedule)
		}
	}
	return schedules
}

// GetAllActiveVestingSchedules returns all active/paused vesting schedules.
func (k Keeper) GetAllActiveVestingSchedules(ctx sdk.Context) []*types.VestingSchedule {
	store := k.storeService.OpenKVStore(ctx)

	var schedules []*types.VestingSchedule
	iter, err := store.Iterator(types.ActiveVestingPrefix, prefixEndBytes(types.ActiveVestingPrefix))
	if err != nil {
		return schedules
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		vestingId := string(iter.Key()[len(types.ActiveVestingPrefix):])
		schedule, found := k.GetVestingSchedule(ctx, vestingId)
		if found {
			schedules = append(schedules, schedule)
		}
	}
	return schedules
}

// SetClawbackRecord stores a clawback record.
func (k Keeper) SetClawbackRecord(ctx sdk.Context, record *types.ClawbackRecord) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(record)
	if err != nil {
		panic("failed to marshal clawback record: " + err.Error())
	}
	key := append(types.FalsificationKeyPrefix, []byte(record.Id)...)
	if err := store.Set(key, bz); err != nil {
		panic("failed to set clawback record: " + err.Error())
	}
}

// GetClawbackRecord retrieves a clawback record by ID.
func (k Keeper) GetClawbackRecord(ctx sdk.Context, id string) (*types.ClawbackRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := append(types.FalsificationKeyPrefix, []byte(id)...)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var record types.ClawbackRecord
	if err := proto.Unmarshal(bz, &record); err != nil {
		return nil, false
	}
	return &record, true
}

// SetBlockRewardDistribution stores a block reward distribution record.
func (k Keeper) SetBlockRewardDistribution(ctx sdk.Context, dist *types.BlockRewardDistribution) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(dist)
	if err != nil {
		return
	}
	key := append(types.BlockRewardKeyPrefix, sdk.Uint64ToBigEndian(dist.BlockHeight)...)
	if err := store.Set(key, bz); err != nil {
		return
	}
}
