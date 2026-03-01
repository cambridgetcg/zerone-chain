package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/discovery/types"
)

// Keeper manages the discovery module's state.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.BinaryCodec
	authority    string

	bankKeeper          types.BankKeeper
	pacingKeeper        types.PacingKeeper              // nil-safe, R29-6
	qualificationKeeper types.DomainQualificationKeeper // nil-safe, R31-4
}

// SetPacingKeeper sets the pacing keeper for adaptive expiry timing (post-init, R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) { k.pacingKeeper = pk }

// SetQualificationKeeper sets the domain qualification keeper (post-init, R31-4).
func (k *Keeper) SetQualificationKeeper(qk types.DomainQualificationKeeper) { k.qualificationKeeper = qk }

// NewKeeper creates a new discovery module Keeper.
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

// ---------- Profile Operations ----------

// SetProfile stores an agent profile and maintains domain/capability indexes.
func (k Keeper) SetProfile(ctx context.Context, profile *types.AgentProfile) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := profileKey(profile.Address)
	bz, err := proto.Marshal(profile)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal profile: %v", err))
	}
	_ = kvStore.Set(key, bz)

	// Maintain domain index.
	addrBytes := []byte(profile.Address)
	for _, domain := range profile.Domains {
		idxKey := domainIndexKey(domain, profile.Address)
		_ = kvStore.Set(idxKey, addrBytes)
	}

	// Maintain capability index.
	for _, cap := range profile.Capabilities {
		idxKey := capabilityIndexKey(cap.CapabilityType, profile.Address)
		_ = kvStore.Set(idxKey, addrBytes)
	}
}

// GetProfile retrieves an agent profile by address.
func (k Keeper) GetProfile(ctx context.Context, address string) (*types.AgentProfile, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := profileKey(address)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var profile types.AgentProfile
	if err := proto.Unmarshal(bz, &profile); err != nil {
		return nil, false
	}
	return &profile, true
}

// DeleteProfile removes a profile and cleans all associated indexes.
func (k Keeper) DeleteProfile(ctx context.Context, profile *types.AgentProfile) {
	kvStore := k.storeService.OpenKVStore(ctx)

	// Remove domain indexes.
	for _, domain := range profile.Domains {
		_ = kvStore.Delete(domainIndexKey(domain, profile.Address))
	}

	// Remove capability indexes.
	for _, cap := range profile.Capabilities {
		_ = kvStore.Delete(capabilityIndexKey(cap.CapabilityType, profile.Address))
	}

	// Remove the profile itself.
	_ = kvStore.Delete(profileKey(profile.Address))
}

// IterateProfiles iterates over all profiles. Return true from cb to stop.
func (k Keeper) IterateProfiles(ctx context.Context, cb func(*types.AgentProfile) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.ProfileKeyPrefix, prefixEndBytes(types.ProfileKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var profile types.AgentProfile
		if err := proto.Unmarshal(iter.Value(), &profile); err != nil {
			continue
		}
		if cb(&profile) {
			break
		}
	}
}

// GetAllProfiles returns all agent profiles.
func (k Keeper) GetAllProfiles(ctx context.Context) []*types.AgentProfile {
	var profiles []*types.AgentProfile
	k.IterateProfiles(ctx, func(p *types.AgentProfile) bool {
		profiles = append(profiles, p)
		return false
	})
	return profiles
}

// GetProfilesByDomain returns active profiles for a given domain.
func (k Keeper) GetProfilesByDomain(ctx context.Context, domain string) []*types.AgentProfile {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := domainPrefix(domain)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var profiles []*types.AgentProfile
	for ; iter.Valid(); iter.Next() {
		address := string(iter.Value())
		profile, found := k.GetProfile(ctx, address)
		if found && profile.Status == "active" {
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

// GetProfilesByCapability returns active profiles that have a given capability type.
func (k Keeper) GetProfilesByCapability(ctx context.Context, capabilityType string) []*types.AgentProfile {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := capabilityPrefix(capabilityType)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var profiles []*types.AgentProfile
	for ; iter.Valid(); iter.Next() {
		address := string(iter.Value())
		profile, found := k.GetProfile(ctx, address)
		if found && profile.Status == "active" {
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

// SearchProfiles combines domain, capability, and reputation filters.
func (k Keeper) SearchProfiles(ctx context.Context, domain string, capabilityType string, minReputation uint64) []*types.AgentProfile {
	// If both domain and capability are specified, intersect the results.
	// If only one is specified, use that index.
	// If neither is specified, iterate all profiles.
	var candidates []*types.AgentProfile

	switch {
	case domain != "" && capabilityType != "":
		// Start with domain index, then filter by capability.
		domainProfiles := k.GetProfilesByDomain(ctx, domain)
		for _, p := range domainProfiles {
			if hasCapability(p, capabilityType) {
				candidates = append(candidates, p)
			}
		}
	case domain != "":
		candidates = k.GetProfilesByDomain(ctx, domain)
	case capabilityType != "":
		candidates = k.GetProfilesByCapability(ctx, capabilityType)
	default:
		// No index filter — iterate all active profiles.
		k.IterateProfiles(ctx, func(p *types.AgentProfile) bool {
			if p.Status == "active" {
				candidates = append(candidates, p)
			}
			return false
		})
	}

	// Apply reputation filter.
	if minReputation == 0 {
		return candidates
	}
	var results []*types.AgentProfile
	for _, p := range candidates {
		if p.ReputationScore >= minReputation {
			results = append(results, p)
		}
	}
	return results
}

// hasCapability checks if a profile has a given capability type.
func hasCapability(p *types.AgentProfile, capType string) bool {
	for _, c := range p.Capabilities {
		if c.CapabilityType == capType {
			return true
		}
	}
	return false
}

// ---------- Genesis ----------

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx context.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	}
	for _, profile := range genState.Profiles {
		k.SetProfile(ctx, profile)
	}
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	profiles := k.GetAllProfiles(ctx)
	return &types.GenesisState{
		Params:   params,
		Profiles: profiles,
	}
}

// ---------- Key Construction Helpers ----------

func profileKey(address string) []byte {
	return append(types.ProfileKeyPrefix, []byte(address)...)
}

func domainPrefix(domain string) []byte {
	// ByDomainIndexPrefix + domain + 0x00
	prefix := append(types.ByDomainIndexPrefix, []byte(domain)...)
	return append(prefix, 0x00)
}

func domainIndexKey(domain string, address string) []byte {
	// ByDomainIndexPrefix + domain + 0x00 + address
	key := append(types.ByDomainIndexPrefix, []byte(domain)...)
	key = append(key, 0x00)
	return append(key, []byte(address)...)
}

func capabilityPrefix(capabilityType string) []byte {
	// ByCapabilityIndexPrefix + capability_type + 0x00
	prefix := append(types.ByCapabilityIndexPrefix, []byte(capabilityType)...)
	return append(prefix, 0x00)
}

func capabilityIndexKey(capabilityType string, address string) []byte {
	// ByCapabilityIndexPrefix + capability_type + 0x00 + address
	key := append(types.ByCapabilityIndexPrefix, []byte(capabilityType)...)
	key = append(key, 0x00)
	return append(key, []byte(address)...)
}
