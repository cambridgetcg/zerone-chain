package keeper

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/claiming_pot/types"
)

type Keeper struct {
	storeService         store.KVStoreService
	cdc                  codec.BinaryCodec
	authority            string
	stakingKeeper        types.StakingKeeper
	authKeeper           types.AuthKeeper
	bankKeeper           types.BankKeeper
	vestingRewardsKeeper types.VestingRewardsKeeper
}

func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
	sk types.StakingKeeper,
	ak types.AuthKeeper,
	bk types.BankKeeper,
	vrk types.VestingRewardsKeeper,
) Keeper {
	return Keeper{
		storeService:         storeService,
		cdc:                  cdc,
		authority:            authority,
		stakingKeeper:        sk,
		authKeeper:           ak,
		bankKeeper:           bk,
		vestingRewardsKeeper: vrk,
	}
}

func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

func (k Keeper) GetAuthority() string {
	return k.authority
}

// prefixEndBytes returns the end key for prefix iteration (exclusive).
func prefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}

// ---------- Params ----------

func (k Keeper) SetParams(ctx context.Context, params *types.Params) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal params: %v", err))
	}
	_ = kvStore.Set(types.ParamsKey, bz)
}

func (k Keeper) GetParams(ctx context.Context) *types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		return types.DefaultParams()
	}
	return &params
}

// ---------- Counter ----------

func (k Keeper) GetNextPotID(ctx context.Context) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.PotCounterKey)
	if err != nil || bz == nil {
		bz = make([]byte, 8)
	}
	counter := binary.BigEndian.Uint64(bz)
	counter++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, counter)
	_ = kvStore.Set(types.PotCounterKey, newBz)
	return counter
}

// ---------- ClaimingPot CRUD ----------

func (k Keeper) SetPot(ctx context.Context, pot *types.ClaimingPot) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(pot)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal pot: %v", err))
	}
	_ = kvStore.Set(types.PotKey(pot.Id), bz)

	// Active index
	if pot.Status == types.PotStatus_POT_STATUS_ACTIVE {
		_ = kvStore.Set(types.ActivePotKey(pot.Id), []byte{1})
	} else {
		_ = kvStore.Delete(types.ActivePotKey(pot.Id))
	}
}

func (k Keeper) GetPot(ctx context.Context, id string) (*types.ClaimingPot, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.PotKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var pot types.ClaimingPot
	if err := proto.Unmarshal(bz, &pot); err != nil {
		return nil, false
	}
	return &pot, true
}

func (k Keeper) GetAllPots(ctx context.Context) []*types.ClaimingPot {
	var pots []*types.ClaimingPot
	k.IteratePots(ctx, func(p *types.ClaimingPot) bool {
		pots = append(pots, p)
		return false
	})
	return pots
}

func (k Keeper) IteratePots(ctx context.Context, cb func(*types.ClaimingPot) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.PotKeyPrefix, prefixEndBytes(types.PotKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var pot types.ClaimingPot
		if err := proto.Unmarshal(iter.Value(), &pot); err != nil {
			continue
		}
		if cb(&pot) {
			break
		}
	}
}

func (k Keeper) CountActivePots(ctx context.Context) int {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.ActivePotPrefix, prefixEndBytes(types.ActivePotPrefix))
	if err != nil {
		return 0
	}
	defer iter.Close()
	count := 0
	for ; iter.Valid(); iter.Next() {
		count++
	}
	return count
}

// ---------- Claim CRUD ----------

func (k Keeper) SetClaim(ctx context.Context, claim *types.Claim) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(claim)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal claim: %v", err))
	}
	_ = kvStore.Set(types.ClaimKey(claim.PotId, claim.Claimant), bz)
}

func (k Keeper) GetClaim(ctx context.Context, potID, claimant string) (*types.Claim, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ClaimKey(potID, claimant))
	if err != nil || bz == nil {
		return nil, false
	}
	var claim types.Claim
	if err := proto.Unmarshal(bz, &claim); err != nil {
		return nil, false
	}
	return &claim, true
}

func (k Keeper) GetClaimsByPot(ctx context.Context, potID string) []*types.Claim {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.ClaimByPotPrefix(potID)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var claims []*types.Claim
	for ; iter.Valid(); iter.Next() {
		var claim types.Claim
		if err := proto.Unmarshal(iter.Value(), &claim); err != nil {
			continue
		}
		claims = append(claims, &claim)
	}
	return claims
}

func (k Keeper) GetAllClaims(ctx context.Context) []*types.Claim {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.ClaimKeyPrefix, prefixEndBytes(types.ClaimKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var claims []*types.Claim
	for ; iter.Valid(); iter.Next() {
		var claim types.Claim
		if err := proto.Unmarshal(iter.Value(), &claim); err != nil {
			continue
		}
		claims = append(claims, &claim)
	}
	return claims
}

// ---------- Vesting Math ----------

// CalculateClaimable computes the vested-but-unclaimed amount for a pot at a given block.
func CalculateClaimable(pot *types.ClaimingPot, currentBlock uint64) *big.Int {
	if pot.Schedule == nil {
		return new(big.Int)
	}

	schedule := pot.Schedule

	// Before start: nothing vested
	if currentBlock < schedule.StartBlock {
		return new(big.Int)
	}

	// Before cliff: nothing vested
	cliffBlock := schedule.StartBlock + schedule.CliffBlocks
	if currentBlock < cliffBlock {
		return new(big.Int)
	}

	totalAmount := new(big.Int)
	totalAmount.SetString(pot.TotalAmount, 10)
	if totalAmount.Sign() <= 0 {
		return new(big.Int)
	}

	claimedAmount := new(big.Int)
	claimedAmount.SetString(pot.ClaimedAmount, 10)

	// After end: fully vested
	if currentBlock >= schedule.EndBlock {
		remaining := new(big.Int).Sub(totalAmount, claimedAmount)
		if remaining.Sign() < 0 {
			return new(big.Int)
		}
		return remaining
	}

	// Linear vesting between cliff and end
	elapsed := currentBlock - schedule.StartBlock
	totalDuration := schedule.EndBlock - schedule.StartBlock
	if totalDuration == 0 {
		return new(big.Int)
	}

	// vested = totalAmount * elapsed / totalDuration
	vested := new(big.Int).Mul(totalAmount, new(big.Int).SetUint64(elapsed))
	vested.Div(vested, new(big.Int).SetUint64(totalDuration))

	// claimable = vested - already claimed
	claimable := new(big.Int).Sub(vested, claimedAmount)
	if claimable.Sign() < 0 {
		return new(big.Int)
	}
	return claimable
}

// ---------- BeginBlocker: Pot Expiry ----------

// ProcessPotExpiry checks active pots and marks expired ones.
func (k Keeper) ProcessPotExpiry(ctx context.Context, currentBlock uint64) {
	k.IteratePots(ctx, func(pot *types.ClaimingPot) bool {
		if pot.Status == types.PotStatus_POT_STATUS_ACTIVE && pot.Schedule != nil {
			if currentBlock >= pot.Schedule.EndBlock {
				pot.Status = types.PotStatus_POT_STATUS_EXPIRED
				k.SetPot(ctx, pot)
			}
		}
		return false
	})
}

// ---------- Genesis ----------

func (k Keeper) InitGenesis(ctx context.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	}
	for _, pot := range genState.Pots {
		k.SetPot(ctx, pot)
	}
	for _, claim := range genState.Claims {
		k.SetClaim(ctx, claim)
	}
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	pots := k.GetAllPots(ctx)
	claims := k.GetAllClaims(ctx)
	return &types.GenesisState{
		Params: params,
		Pots:   pots,
		Claims: claims,
	}
}
