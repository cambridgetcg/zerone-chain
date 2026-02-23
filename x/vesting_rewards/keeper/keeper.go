package keeper

import (
	"fmt"
	"math/big"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	commontypes "github.com/zerone-chain/zerone/x/common/types"
	"github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// Keeper manages the vesting_rewards module's state.
type Keeper struct {
	cdc          codec.Codec
	storeService store.KVStoreService

	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper

	authority string

	autopoiesisKeeper types.AutopoiesisKeeper

	// blockTxCount is set by PreBlocker each block with the actual transaction count.
	blockTxCount int
}

// NewKeeper creates a new vesting_rewards module Keeper.
func NewKeeper(
	cdc codec.Codec,
	storeService store.KVStoreService,
	bk types.BankKeeper,
	sk types.StakingKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		bankKeeper:   bk,
		stakingKeeper: sk,
		authority:    authority,
	}
}

// prefixEndBytes returns the end key for a prefix scan (exclusive).
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

// SetAutopoiesisKeeper sets the autopoiesis keeper for adaptive reward multipliers.
func (k *Keeper) SetAutopoiesisKeeper(ak types.AutopoiesisKeeper) {
	k.autopoiesisKeeper = ak
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// GetStakingKeeper returns the staking keeper.
func (k Keeper) GetStakingKeeper() types.StakingKeeper {
	return k.stakingKeeper
}

// SetBlockTxCount stores the transaction count for the current block.
func (k *Keeper) SetBlockTxCount(count int) {
	k.blockTxCount = count
}

// GetBlockTxCount returns the transaction count for the current block.
func (k Keeper) GetBlockTxCount() int {
	return k.blockTxCount
}

// GetParams returns the module parameters.
func (k Keeper) GetParams(ctx sdk.Context) *types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		return types.DefaultParams()
	}
	return &params
}

// SetParams sets the module parameters.
func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal params: %v", err))
	}
	if err := store.Set(types.ParamsKey, bz); err != nil {
		panic(fmt.Sprintf("failed to set params: %v", err))
	}
}

// GetRevenueSplit returns the revenue split from params, falling back to defaults.
func (k Keeper) GetRevenueSplit(ctx sdk.Context) *commontypes.RevenueSplit {
	params := k.GetParams(ctx)
	if params.RevenueSplit != nil {
		return params.RevenueSplit
	}
	return types.DefaultRevenueSplit()
}

// GetProtocolSubSplit returns the protocol sub-split from params, falling back to defaults.
func (k Keeper) GetProtocolSubSplit(ctx sdk.Context) *commontypes.ProtocolSubSplit {
	params := k.GetParams(ctx)
	if params.ProtocolSubSplit != nil {
		return params.ProtocolSubSplit
	}
	return types.DefaultProtocolSubSplit()
}

// isFounderShareActive returns whether the founder auto-split is active.
// The founder share is immune to governance — it cannot be disabled via parameter
// changes. It is only inactive if the founder address is not yet set.
// The share percentage (FounderShareBps) is enforced as immutable in UpdateParams.
func (k Keeper) isFounderShareActive(ctx sdk.Context, params *types.Params) bool {
	if params.FounderShareBps == 0 || params.FounderAddress == "" {
		return false
	}
	// NOTE: GovernanceActivationHeight sunset has been removed.
	// The founder share is permanent and governance-immune.
	return true
}

// GetCategoryConfig returns the release curve config for a vesting category.
func (k Keeper) GetCategoryConfig(ctx sdk.Context, category types.VestingCategoryStr) (*types.CategoryConfig, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := append(types.CategoryConfigKeyPrefix, []byte(category)...)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		for _, cfg := range types.DefaultCategoryConfigs() {
			if cfg.Category == string(category) {
				return cfg, true
			}
		}
		return nil, false
	}
	var cfg types.CategoryConfig
	if err := proto.Unmarshal(bz, &cfg); err != nil {
		return nil, false
	}
	return &cfg, true
}

// SetCategoryConfig stores a category config.
func (k Keeper) SetCategoryConfig(ctx sdk.Context, cfg *types.CategoryConfig) {
	store := k.storeService.OpenKVStore(ctx)
	key := append(types.CategoryConfigKeyPrefix, []byte(cfg.Category)...)
	bz, err := proto.Marshal(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal category config: %v", err))
	}
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set category config: %v", err))
	}
}

// GetTotalMinted returns the total ZRN minted so far (in uzrn).
func (k Keeper) GetTotalMinted(ctx sdk.Context) *big.Int {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TotalMintedKey)
	if err != nil || bz == nil {
		return new(big.Int)
	}
	total := new(big.Int)
	if _, ok := total.SetString(string(bz), 10); !ok {
		return new(big.Int)
	}
	return total
}

// SetTotalMinted stores the total ZRN minted so far (in uzrn).
func (k Keeper) SetTotalMinted(ctx sdk.Context, amount *big.Int) {
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(types.TotalMintedKey, []byte(amount.String())); err != nil {
		panic(fmt.Sprintf("failed to set total minted: %v", err))
	}
}

// MintWithCap mints new ZRN tokens up to the supply cap (222,222,222 ZRN).
// Burn recycling: the cap is enforced against current bank supply (not cumulative
// totalMinted). Burned tokens free headroom for future block rewards.
func (k Keeper) MintWithCap(ctx sdk.Context, amount *big.Int) (*big.Int, error) {
	if amount.Sign() <= 0 {
		return new(big.Int), nil
	}

	maxSupply := new(big.Int)
	maxSupply.SetString(types.MaxSupplyUzrn, 10)

	var currentSupply *big.Int
	if k.bankKeeper != nil {
		supply := k.bankKeeper.GetSupply(ctx, "uzrn")
		currentSupply = supply.Amount.BigInt()
	} else {
		currentSupply = k.GetTotalMinted(ctx)
	}

	remaining := new(big.Int).Sub(maxSupply, currentSupply)
	if remaining.Sign() <= 0 {
		return new(big.Int), nil
	}

	actual := new(big.Int).Set(amount)
	if actual.Cmp(remaining) > 0 {
		actual.Set(remaining)
	}

	if k.bankKeeper != nil {
		mintCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(actual)))
		if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, mintCoins); err != nil {
			return nil, fmt.Errorf("failed to mint block reward: %w", err)
		}
	}

	totalMinted := k.GetTotalMinted(ctx)
	newTotal := new(big.Int).Add(totalMinted, actual)
	k.SetTotalMinted(ctx, newTotal)

	return actual, nil
}

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, gs *types.GenesisState) {
	if gs.Params != nil {
		k.SetParams(ctx, gs.Params)
	}

	for _, cfg := range gs.CategoryConfigs {
		if cfg != nil {
			k.SetCategoryConfig(ctx, cfg)
		}
	}

	for _, schedule := range gs.VestingSchedules {
		if schedule != nil {
			k.SetVestingSchedule(ctx, schedule)
		}
	}

	totalMinted := new(big.Int)
	if gs.Params != nil && gs.Params.InitialFundBalance != "" && gs.Params.InitialFundBalance != "0" {
		if _, ok := totalMinted.SetString(gs.Params.InitialFundBalance, 10); !ok {
			totalMinted = new(big.Int)
		}
	}
	k.SetTotalMinted(ctx, totalMinted)
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	totalMinted := k.GetTotalMinted(ctx)
	params.InitialFundBalance = totalMinted.String()
	return &types.GenesisState{
		Params:           params,
		CategoryConfigs:  k.GetAllCategoryConfigs(ctx),
		VestingSchedules: k.GetAllActiveVestingSchedules(ctx),
	}
}

// GetAllCategoryConfigs returns all stored category configs, falling back to defaults.
func (k Keeper) GetAllCategoryConfigs(ctx sdk.Context) []*types.CategoryConfig {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.CategoryConfigKeyPrefix, prefixEndBytes(types.CategoryConfigKeyPrefix))
	if err != nil {
		return types.DefaultCategoryConfigs()
	}
	defer iter.Close()

	var configs []*types.CategoryConfig
	for ; iter.Valid(); iter.Next() {
		var cfg types.CategoryConfig
		if err := proto.Unmarshal(iter.Value(), &cfg); err != nil {
			continue
		}
		configs = append(configs, &cfg)
	}

	if len(configs) == 0 {
		return types.DefaultCategoryConfigs()
	}
	return configs
}

// GetDecaySchedule returns the epoch-based decay parameters.
func (k Keeper) GetDecaySchedule(ctx sdk.Context) (uint64, uint64, string) {
	params := k.GetParams(ctx)
	return params.BlocksPerRewardEpoch, params.RewardDecayBps, params.FloorReward
}

// GetBlockRewardDistribution retrieves the block reward distribution for a specific height.
func (k Keeper) GetBlockRewardDistribution(ctx sdk.Context, blockHeight uint64) (*types.BlockRewardDistribution, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := append(types.BlockRewardKeyPrefix, sdk.Uint64ToBigEndian(blockHeight)...)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var dist types.BlockRewardDistribution
	if err := proto.Unmarshal(bz, &dist); err != nil {
		return nil, false
	}
	return &dist, true
}
