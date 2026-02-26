package keeper

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/home/types"
)

// Keeper manages the home module's state.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.BinaryCodec
	authority    string
	bankKeeper   types.BankKeeper
}

// NewKeeper creates a new home module Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
	bk types.BankKeeper,
) Keeper {
	return Keeper{
		storeService: storeService,
		cdc:          cdc,
		authority:    authority,
		bankKeeper:   bk,
	}
}

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetAuthority returns the module authority address.
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

// SetParams sets module parameters.
func (k Keeper) SetParams(ctx context.Context, params *types.Params) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal params: %v", err))
	}
	_ = kvStore.Set(types.ParamsKey, bz)
}

// GetParams returns module parameters.
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

// ---------- Home CRUD ----------

// SetHome stores a home.
func (k Keeper) SetHome(ctx context.Context, home *types.AgentHome) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(home)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal home: %v", err))
	}
	_ = kvStore.Set(types.HomeKey(home.HomeId), bz)
}

// GetHome returns a home by ID.
func (k Keeper) GetHome(ctx context.Context, homeID string) (*types.AgentHome, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.HomeKey(homeID))
	if err != nil || bz == nil {
		return nil, false
	}
	var home types.AgentHome
	if err := proto.Unmarshal(bz, &home); err != nil {
		return nil, false
	}
	return &home, true
}

// IterateHomes iterates over all homes and invokes the callback.
func (k Keeper) IterateHomes(ctx context.Context, cb func(*types.AgentHome) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.HomeKeyPrefix, prefixEndBytes(types.HomeKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var home types.AgentHome
		if err := proto.Unmarshal(iter.Value(), &home); err != nil {
			continue
		}
		if cb(&home) {
			break
		}
	}
}

// GetAllHomes returns all homes.
func (k Keeper) GetAllHomes(ctx context.Context) []*types.AgentHome {
	var homes []*types.AgentHome
	k.IterateHomes(ctx, func(h *types.AgentHome) bool {
		homes = append(homes, h)
		return false
	})
	return homes
}

// ---------- Owner Index ----------

// AddHomeToOwnerIndex adds a home ID to the owner's index.
func (k Keeper) AddHomeToOwnerIndex(ctx context.Context, owner, homeID string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.HomeByOwnerKey(owner)
	var ids []string
	bz, err := kvStore.Get(key)
	if err == nil && bz != nil {
		_ = json.Unmarshal(bz, &ids)
	}
	ids = append(ids, homeID)
	data, _ := json.Marshal(ids)
	_ = kvStore.Set(key, data)
}

// GetHomesByOwner returns all home IDs for an owner.
func (k Keeper) GetHomesByOwner(ctx context.Context, owner string) []string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.HomeByOwnerKey(owner))
	if err != nil || bz == nil {
		return nil
	}
	var ids []string
	_ = json.Unmarshal(bz, &ids)
	return ids
}

// ---------- Home ID Counter ----------

// GetNextHomeID generates the next home ID and increments the counter.
func (k Keeper) GetNextHomeID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	var counter uint64
	bz, err := kvStore.Get(types.HomeCounterKey)
	if err == nil && bz != nil && len(bz) == 8 {
		counter = binary.BigEndian.Uint64(bz)
	}
	counter++
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	_ = kvStore.Set(types.HomeCounterKey, buf)
	return fmt.Sprintf("home-%d", counter)
}

// ---------- Partnership Link ----------

// SetPartnershipOnHome sets the partnership ID on a home.
func (k Keeper) SetPartnershipOnHome(ctx context.Context, homeID, partnershipID string) {
	home, found := k.GetHome(ctx, homeID)
	if !found {
		return
	}
	home.PartnershipId = partnershipID
	k.SetHome(ctx, home)
}

// ---------- Session CRUD ----------

// SetSession stores a session.
func (k Keeper) SetSession(ctx context.Context, session *types.ActiveSession) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(session)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal session: %v", err))
	}
	_ = kvStore.Set(types.SessionKey(session.HomeId, session.SessionId), bz)
}

// GetSession returns a session by home ID and session ID.
func (k Keeper) GetSession(ctx context.Context, homeID, sessionID string) (*types.ActiveSession, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SessionKey(homeID, sessionID))
	if err != nil || bz == nil {
		return nil, false
	}
	var session types.ActiveSession
	if err := proto.Unmarshal(bz, &session); err != nil {
		return nil, false
	}
	return &session, true
}

// DeleteSession removes a session.
func (k Keeper) DeleteSession(ctx context.Context, homeID, sessionID string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.SessionKey(homeID, sessionID))
}

// IterateSessions iterates over all sessions for a home.
func (k Keeper) IterateSessions(ctx context.Context, homeID string, cb func(*types.ActiveSession) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.SessionPrefixKey(homeID)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var session types.ActiveSession
		if err := proto.Unmarshal(iter.Value(), &session); err != nil {
			continue
		}
		if cb(&session) {
			break
		}
	}
}

// CountSessions counts the number of active sessions for a home.
func (k Keeper) CountSessions(ctx context.Context, homeID string) uint64 {
	var count uint64
	k.IterateSessions(ctx, homeID, func(_ *types.ActiveSession) bool {
		count++
		return false
	})
	return count
}

// GetSessionsByHome returns all sessions for a home.
func (k Keeper) GetSessionsByHome(ctx context.Context, homeID string) []*types.ActiveSession {
	var sessions []*types.ActiveSession
	k.IterateSessions(ctx, homeID, func(s *types.ActiveSession) bool {
		sessions = append(sessions, s)
		return false
	})
	return sessions
}

// ---------- Key Registration CRUD ----------

// SetKeyRegistration stores a key registration.
func (k Keeper) SetKeyRegistration(ctx context.Context, homeID string, reg *types.KeyRegistration) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(reg)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal key registration: %v", err))
	}
	_ = kvStore.Set(types.KeyRegKey(homeID, reg.KeyHash), bz)
}

// GetKeyRegistration returns a key registration.
func (k Keeper) GetKeyRegistration(ctx context.Context, homeID, keyHash string) (*types.KeyRegistration, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyRegKey(homeID, keyHash))
	if err != nil || bz == nil {
		return nil, false
	}
	var reg types.KeyRegistration
	if err := proto.Unmarshal(bz, &reg); err != nil {
		return nil, false
	}
	return &reg, true
}

// IterateKeyRegistrations iterates over all key registrations for a home.
func (k Keeper) IterateKeyRegistrations(ctx context.Context, homeID string, cb func(*types.KeyRegistration) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.KeyRegPrefixKey(homeID)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var reg types.KeyRegistration
		if err := proto.Unmarshal(iter.Value(), &reg); err != nil {
			continue
		}
		if cb(&reg) {
			break
		}
	}
}

// CountActiveKeys counts non-revoked keys for a home.
func (k Keeper) CountActiveKeys(ctx context.Context, homeID string) uint64 {
	var count uint64
	k.IterateKeyRegistrations(ctx, homeID, func(reg *types.KeyRegistration) bool {
		if !reg.Revoked {
			count++
		}
		return false
	})
	return count
}

// GetKeysByHome returns all key registrations for a home.
func (k Keeper) GetKeysByHome(ctx context.Context, homeID string) []*types.KeyRegistration {
	var keys []*types.KeyRegistration
	k.IterateKeyRegistrations(ctx, homeID, func(reg *types.KeyRegistration) bool {
		keys = append(keys, reg)
		return false
	})
	return keys
}

// ---------- Alert CRUD ----------

// SetAlert stores an alert.
func (k Keeper) SetAlert(ctx context.Context, alert *types.Alert) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(alert)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal alert: %v", err))
	}
	_ = kvStore.Set(types.AlertKey(alert.HomeId, alert.AlertId), bz)
}

// SetAlertWithLimit stores an alert only if the pending alert count is below MaxAlertsPerHome.
// Returns true if the alert was stored, false if the limit was reached.
func (k Keeper) SetAlertWithLimit(ctx context.Context, alert *types.Alert) bool {
	params := k.GetParams(ctx)
	if k.CountPendingAlerts(ctx, alert.HomeId) >= params.MaxAlertsPerHome {
		return false
	}
	k.SetAlert(ctx, alert)
	return true
}

// GetAlert returns an alert.
func (k Keeper) GetAlert(ctx context.Context, homeID, alertID string) (*types.Alert, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AlertKey(homeID, alertID))
	if err != nil || bz == nil {
		return nil, false
	}
	var alert types.Alert
	if err := proto.Unmarshal(bz, &alert); err != nil {
		return nil, false
	}
	return &alert, true
}

// IterateAlerts iterates over all alerts for a home.
func (k Keeper) IterateAlerts(ctx context.Context, homeID string, cb func(*types.Alert) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AlertPrefixKey(homeID)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var alert types.Alert
		if err := proto.Unmarshal(iter.Value(), &alert); err != nil {
			continue
		}
		if cb(&alert) {
			break
		}
	}
}

// GetAlertsByHome returns all alerts for a home.
func (k Keeper) GetAlertsByHome(ctx context.Context, homeID string) []*types.Alert {
	var alerts []*types.Alert
	k.IterateAlerts(ctx, homeID, func(a *types.Alert) bool {
		alerts = append(alerts, a)
		return false
	})
	return alerts
}

// CountPendingAlerts counts unacknowledged alerts for a home.
func (k Keeper) CountPendingAlerts(ctx context.Context, homeID string) uint64 {
	var count uint64
	k.IterateAlerts(ctx, homeID, func(a *types.Alert) bool {
		if !a.Acknowledged {
			count++
		}
		return false
	})
	return count
}

// ---------- Spending Limit CRUD ----------

// SetSpendingLimit stores a spending limit.
func (k Keeper) SetSpendingLimit(ctx context.Context, homeID string, limit *types.SpendingLimit) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(limit)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal spending limit: %v", err))
	}
	_ = kvStore.Set(types.SpendLimitKey(homeID, limit.KeyType), bz)
}

// GetSpendingLimit returns a spending limit.
func (k Keeper) GetSpendingLimit(ctx context.Context, homeID, keyType string) (*types.SpendingLimit, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SpendLimitKey(homeID, keyType))
	if err != nil || bz == nil {
		return nil, false
	}
	var limit types.SpendingLimit
	if err := proto.Unmarshal(bz, &limit); err != nil {
		return nil, false
	}
	return &limit, true
}

// GetSpendingLimitsByHome returns all spending limits for a home.
func (k Keeper) GetSpendingLimitsByHome(ctx context.Context, homeID string) []*types.SpendingLimit {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.SpendLimitPrefixKey(homeID)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var limits []*types.SpendingLimit
	for ; iter.Valid(); iter.Next() {
		var limit types.SpendingLimit
		if err := proto.Unmarshal(iter.Value(), &limit); err != nil {
			continue
		}
		limits = append(limits, &limit)
	}
	return limits
}

// ---------- Genesis ----------

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx context.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	}
	for _, home := range genState.Homes {
		k.SetHome(ctx, home)
		k.AddHomeToOwnerIndex(ctx, home.OwnerAddress, home.HomeId)
	}
	for _, ks := range genState.KeySets {
		for _, key := range ks.Keys {
			k.SetKeyRegistration(ctx, ks.HomeId, key)
		}
	}
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	homes := k.GetAllHomes(ctx)

	var keySets []*types.HomeKeySet
	for _, home := range homes {
		keys := k.GetKeysByHome(ctx, home.HomeId)
		if len(keys) > 0 {
			keySets = append(keySets, &types.HomeKeySet{
				HomeId: home.HomeId,
				Keys:   keys,
			})
		}
	}

	return &types.GenesisState{
		Params:  params,
		Homes:   homes,
		KeySets: keySets,
	}
}
