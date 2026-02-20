package keeper

import (
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/auth/types"
)

// Keeper manages Zerone account state with 4-layer key architecture.
type Keeper struct {
	cdc           codec.Codec
	storeService  store.KVStoreService
	accountKeeper types.CosmosAccountKeeper
	bankKeeper    types.BankKeeper
	authority     string
}

// NewKeeper creates a new Keeper instance.
func NewKeeper(
	cdc codec.Codec,
	storeService store.KVStoreService,
	accountKeeper types.CosmosAccountKeeper,
	bankKeeper types.BankKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
		authority:     authority,
	}
}

// prefixEndBytes returns the end key for a prefix scan (exclusive upper bound).
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

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// SetAccount stores a Zerone account.
func (k Keeper) SetAccount(ctx sdk.Context, account *types.Account) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(account)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal account: %v", err))
	}
	if err := kvStore.Set(types.AccountKey(account.Address), bz); err != nil {
		panic(fmt.Sprintf("failed to store account: %v", err))
	}
}

// GetAccount retrieves a Zerone account by bech32 address.
func (k Keeper) GetAccount(ctx sdk.Context, address string) (*types.Account, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AccountKey(address))
	if err != nil || bz == nil {
		return nil, false
	}
	var account types.Account
	if err := proto.Unmarshal(bz, &account); err != nil {
		return nil, false
	}
	return &account, true
}

// GetAccountByDID retrieves a Zerone account by DID.
func (k Keeper) GetAccountByDID(ctx sdk.Context, did string) (*types.Account, bool) {
	address, found := k.GetAddressForDID(ctx, did)
	if !found {
		return nil, false
	}
	return k.GetAccount(ctx, address)
}

// SetDIDMapping stores a DID -> bech32 mapping.
func (k Keeper) SetDIDMapping(ctx sdk.Context, mapping *types.DIDMapping) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(mapping)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal DID mapping: %v", err))
	}
	if err := kvStore.Set(types.DIDMappingKey(mapping.Did), bz); err != nil {
		panic(fmt.Sprintf("failed to store DID mapping: %v", err))
	}
}

// GetDIDMapping retrieves a DID mapping.
func (k Keeper) GetDIDMapping(ctx sdk.Context, did string) (*types.DIDMapping, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.DIDMappingKey(did))
	if err != nil || bz == nil {
		return nil, false
	}
	var mapping types.DIDMapping
	if err := proto.Unmarshal(bz, &mapping); err != nil {
		return nil, false
	}
	return &mapping, true
}

// GetAddressForDID returns the bech32 address for a DID.
func (k Keeper) GetAddressForDID(ctx sdk.Context, did string) (string, bool) {
	mapping, found := k.GetDIDMapping(ctx, did)
	if !found {
		return "", false
	}
	return mapping.Bech32, true
}

// SetSessionKey stores a session key.
func (k Keeper) SetSessionKey(ctx sdk.Context, session *types.SessionKey) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(session)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal session key: %v", err))
	}
	if err := kvStore.Set(types.SessionKeyKey(session.Owner, session.KeyHash), bz); err != nil {
		panic(fmt.Sprintf("failed to store session key: %v", err))
	}
}

// GetSessionKey retrieves a session key.
func (k Keeper) GetSessionKey(ctx sdk.Context, owner, keyHash string) (*types.SessionKey, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SessionKeyKey(owner, keyHash))
	if err != nil || bz == nil {
		return nil, false
	}
	var session types.SessionKey
	if err := proto.Unmarshal(bz, &session); err != nil {
		return nil, false
	}
	return &session, true
}

// DeleteSessionKey removes a session key.
func (k Keeper) DeleteSessionKey(ctx sdk.Context, owner, keyHash string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.SessionKeyKey(owner, keyHash))
}

// GetSessionKeysForOwner returns all session keys for an owner.
func (k Keeper) GetSessionKeysForOwner(ctx sdk.Context, owner string) []*types.SessionKey {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.SessionKeyOwnerPrefix(owner)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var sessions []*types.SessionKey
	for ; iter.Valid(); iter.Next() {
		var session types.SessionKey
		if err := proto.Unmarshal(iter.Value(), &session); err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}
	return sessions
}

// IsSessionValid checks if a session key is valid at the given height.
func (k Keeper) IsSessionValid(ctx sdk.Context, owner, keyHash string, height uint64) bool {
	session, found := k.GetSessionKey(ctx, owner, keyHash)
	if !found {
		return false
	}
	return height < session.ExpiresAtBlock
}

// CountSessionKeys returns the number of active session keys for an owner.
func (k Keeper) CountSessionKeys(ctx sdk.Context, owner string) uint32 {
	sessions := k.GetSessionKeysForOwner(ctx, owner)
	height := uint64(ctx.BlockHeight())
	count := uint32(0)
	for _, session := range sessions {
		if session.ExpiresAtBlock > height {
			count++
		}
	}
	return count
}

// SetParams sets module parameters.
func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}
	if err := kvStore.Set(types.ParamsKey, bz); err != nil {
		return fmt.Errorf("failed to store params: %w", err)
	}
	return nil
}

// GetParams retrieves module parameters.
func (k Keeper) GetParams(ctx sdk.Context) *types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ParamsKey)
	if err != nil || bz == nil {
		p := types.DefaultParams()
		return &p
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		p := types.DefaultParams()
		return &p
	}
	return &params
}

// SetLastRotation stores the block height of last key rotation.
func (k Keeper) SetLastRotation(ctx sdk.Context, address string, height uint64) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.LastRotationKey(address), types.Uint64ToBytes(height))
}

// GetLastRotation retrieves the block height of last key rotation.
func (k Keeper) GetLastRotation(ctx sdk.Context, address string) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.LastRotationKey(address))
	if err != nil || bz == nil {
		return 0
	}
	return types.BytesToUint64(bz)
}

// SetRecoveryConfig stores recovery configuration for an account.
func (k Keeper) SetRecoveryConfig(ctx sdk.Context, address string, config *types.RecoveryConfig) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(config)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal recovery config: %v", err))
	}
	if err := kvStore.Set(types.RecoveryConfigKey(address), bz); err != nil {
		panic(fmt.Sprintf("failed to store recovery config: %v", err))
	}
}

// GetRecoveryConfig retrieves recovery configuration for an account.
func (k Keeper) GetRecoveryConfig(ctx sdk.Context, address string) (*types.RecoveryConfig, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.RecoveryConfigKey(address))
	if err != nil || bz == nil {
		return nil, false
	}
	var config types.RecoveryConfig
	if err := proto.Unmarshal(bz, &config); err != nil {
		return nil, false
	}
	return &config, true
}

// SetRecoveryRequest stores a recovery request.
func (k Keeper) SetRecoveryRequest(ctx sdk.Context, req *types.RecoveryRequest) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(req)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal recovery request: %v", err))
	}
	if err := kvStore.Set(types.RecoveryRequestKey(req.AccountAddress), bz); err != nil {
		panic(fmt.Sprintf("failed to store recovery request: %v", err))
	}
}

// GetRecoveryRequest retrieves a recovery request by account address.
func (k Keeper) GetRecoveryRequest(ctx sdk.Context, address string) (*types.RecoveryRequest, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.RecoveryRequestKey(address))
	if err != nil || bz == nil {
		return nil, false
	}
	var req types.RecoveryRequest
	if err := proto.Unmarshal(bz, &req); err != nil {
		return nil, false
	}
	return &req, true
}

// DeleteRecoveryRequest removes a recovery request.
func (k Keeper) DeleteRecoveryRequest(ctx sdk.Context, address string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.RecoveryRequestKey(address))
}

// IterateRecoveryRequests iterates over all recovery requests.
func (k Keeper) IterateRecoveryRequests(ctx sdk.Context, cb func(*types.RecoveryRequest) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.RecoveryRequestPrefix, prefixEndBytes(types.RecoveryRequestPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var req types.RecoveryRequest
		if err := proto.Unmarshal(iter.Value(), &req); err != nil {
			continue
		}
		if cb(&req) {
			break
		}
	}
}

// CleanupExpiredSessions removes expired session keys and decrements account counters.
func (k Keeper) CleanupExpiredSessions(ctx sdk.Context) {
	currentHeight := uint64(ctx.BlockHeight())
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.SessionKeyPrefix, prefixEndBytes(types.SessionKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	type deletion struct {
		key   []byte
		owner string
	}
	var toDelete []deletion

	for ; iter.Valid(); iter.Next() {
		var session types.SessionKey
		if err := proto.Unmarshal(iter.Value(), &session); err != nil {
			continue
		}
		if session.ExpiresAtBlock <= currentHeight {
			keyCopy := make([]byte, len(iter.Key()))
			copy(keyCopy, iter.Key())
			toDelete = append(toDelete, deletion{key: keyCopy, owner: session.Owner})
		}
	}

	for _, d := range toDelete {
		_ = kvStore.Delete(d.key)
		if account, found := k.GetAccount(ctx, d.owner); found {
			if account.SessionKeyCount > 0 {
				account.SessionKeyCount--
				k.SetAccount(ctx, account)
			}
		}
	}
}

// ProcessRecoveryTimeouts advances recovery request statuses based on block height.
func (k Keeper) ProcessRecoveryTimeouts(ctx sdk.Context) {
	currentHeight := uint64(ctx.BlockHeight())

	k.IterateRecoveryRequests(ctx, func(req *types.RecoveryRequest) bool {
		changed := false

		switch req.Status {
		case "delayed":
			if currentHeight >= req.DelayExpiresAt {
				req.Status = "challengeable"
				changed = true
			}
		case "challengeable":
			if currentHeight >= req.ChallengeExpiresAt {
				req.Status = "executable"
				changed = true
			}
		}

		if changed {
			k.SetRecoveryRequest(ctx, req)
		}
		return false
	})
}

// StoreRecoveryShard persists an encrypted recovery shard on-chain.
func (k Keeper) StoreRecoveryShard(ctx sdk.Context, shard *types.RecoveryShard) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(shard)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal recovery shard: %v", err))
	}
	if err := kvStore.Set(types.RecoveryShardKey(shard.OwnerAddress, shard.ShardIndex), bz); err != nil {
		panic(fmt.Sprintf("failed to store recovery shard: %v", err))
	}
}

// GetRecoveryShard retrieves a single recovery shard by owner and index.
func (k Keeper) GetRecoveryShard(ctx sdk.Context, ownerAddress string, shardIndex uint32) (*types.RecoveryShard, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.RecoveryShardKey(ownerAddress, shardIndex))
	if err != nil || bz == nil {
		return nil, false
	}
	var shard types.RecoveryShard
	if err := proto.Unmarshal(bz, &shard); err != nil {
		return nil, false
	}
	return &shard, true
}

// GetRecoveryShards retrieves all submitted shards for an owner.
func (k Keeper) GetRecoveryShards(ctx sdk.Context, ownerAddress string) []*types.RecoveryShard {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.RecoveryShardOwnerPrefix(ownerAddress)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var shards []*types.RecoveryShard
	for ; iter.Valid(); iter.Next() {
		var shard types.RecoveryShard
		if err := proto.Unmarshal(iter.Value(), &shard); err != nil {
			continue
		}
		shards = append(shards, &shard)
	}
	return shards
}

// DeleteRecoveryShards removes all stored shards for an owner.
func (k Keeper) DeleteRecoveryShards(ctx sdk.Context, ownerAddress string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.RecoveryShardOwnerPrefix(ownerAddress)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keyCopy := make([]byte, len(iter.Key()))
		copy(keyCopy, iter.Key())
		keys = append(keys, keyCopy)
	}
	for _, key := range keys {
		_ = kvStore.Delete(key)
	}
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// HasBootstrapClaim checks if an address has already claimed the bootstrap fund.
func (k Keeper) HasBootstrapClaim(ctx sdk.Context, address string) bool {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.BootstrapClaimKey(address))
	return err == nil && bz != nil
}

// SetBootstrapClaim marks an address as having claimed the bootstrap fund.
func (k Keeper) SetBootstrapClaim(ctx sdk.Context, address string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.BootstrapClaimKey(address), []byte{0x01})
}

// IterateAccounts iterates over all accounts.
func (k Keeper) IterateAccounts(ctx sdk.Context, cb func(*types.Account) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.AccountKeyPrefix, prefixEndBytes(types.AccountKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var account types.Account
		if err := proto.Unmarshal(iter.Value(), &account); err != nil {
			continue
		}
		if cb(&account) {
			break
		}
	}
}

// InitGenesis initializes the module state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, data *types.GenesisState) error {
	if data.Params != nil {
		if err := k.SetParams(ctx, data.Params); err != nil {
			return fmt.Errorf("failed to set params: %w", err)
		}
	} else {
		p := types.DefaultParams()
		if err := k.SetParams(ctx, &p); err != nil {
			return fmt.Errorf("failed to set default params: %w", err)
		}
	}

	for _, account := range data.Accounts {
		if account != nil {
			k.SetAccount(ctx, account)
		}
	}

	for _, mapping := range data.DidMappings {
		if mapping != nil {
			k.SetDIDMapping(ctx, mapping)
		}
	}

	for _, session := range data.SessionKeys {
		if session != nil {
			k.SetSessionKey(ctx, session)
		}
	}

	for _, rc := range data.RecoveryConfigs {
		if rc != nil && rc.AccountAddress != "" {
			k.SetRecoveryConfig(ctx, rc.AccountAddress, rc)
		}
	}

	return nil
}

// ExportGenesis exports the module state for genesis.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	params := k.GetParams(ctx)

	var accounts []*types.Account
	k.IterateAccounts(ctx, func(account *types.Account) bool {
		accounts = append(accounts, account)
		return false
	})

	var mappings []*types.DIDMapping
	kvStore := k.storeService.OpenKVStore(ctx)
	didIter, err := kvStore.Iterator(types.DIDMappingPrefix, prefixEndBytes(types.DIDMappingPrefix))
	if err == nil {
		defer didIter.Close()
		for ; didIter.Valid(); didIter.Next() {
			mapping := new(types.DIDMapping)
			if err := proto.Unmarshal(didIter.Value(), mapping); err != nil {
				continue
			}
			mappings = append(mappings, mapping)
		}
	}

	var sessions []*types.SessionKey
	sessionIter, err := kvStore.Iterator(types.SessionKeyPrefix, prefixEndBytes(types.SessionKeyPrefix))
	if err == nil {
		defer sessionIter.Close()
		for ; sessionIter.Valid(); sessionIter.Next() {
			session := new(types.SessionKey)
			if err := proto.Unmarshal(sessionIter.Value(), session); err != nil {
				continue
			}
			sessions = append(sessions, session)
		}
	}

	var recoveryConfigs []*types.RecoveryConfig
	k.IterateRecoveryConfigs(ctx, func(address string, rc *types.RecoveryConfig) bool {
		rc.AccountAddress = address
		recoveryConfigs = append(recoveryConfigs, rc)
		return false
	})

	return &types.GenesisState{
		Params:          params,
		Accounts:        accounts,
		DidMappings:     mappings,
		SessionKeys:     sessions,
		RecoveryConfigs: recoveryConfigs,
	}
}

// IterateRecoveryConfigs iterates over all recovery configurations.
func (k Keeper) IterateRecoveryConfigs(ctx sdk.Context, cb func(address string, config *types.RecoveryConfig) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.RecoveryConfigPrefix, prefixEndBytes(types.RecoveryConfigPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		config := new(types.RecoveryConfig)
		if err := proto.Unmarshal(iter.Value(), config); err != nil {
			continue
		}
		key := iter.Key()
		address := string(key[len(types.RecoveryConfigPrefix):])
		if cb(address, config) {
			break
		}
	}
}
