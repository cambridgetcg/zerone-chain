package keeper

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// ---------- Params ----------

// SetParams sets module parameters.
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

// GetParams returns module parameters.
func (k Keeper) GetParams(ctx sdk.Context) *types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
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

// ---------- Stratum Operations ----------

// SetStratum stores stratum properties.
func (k Keeper) SetStratum(ctx sdk.Context, props *types.StratumProperties) {
	store := k.storeService.OpenKVStore(ctx)
	key := stratumKey(types.Stratum(props.Stratum))
	bz, err := proto.Marshal(props)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal stratum: %v", err))
	}
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set stratum: %v", err))
	}
}

// GetStratum retrieves stratum properties by level.
func (k Keeper) GetStratum(ctx sdk.Context, stratum types.Stratum) (*types.StratumProperties, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := stratumKey(stratum)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var props types.StratumProperties
	if err := proto.Unmarshal(bz, &props); err != nil {
		return nil, false
	}
	return &props, true
}

// GetAllStrata returns all registered strata, sorted by stratum level.
func (k Keeper) GetAllStrata(ctx sdk.Context) []*types.StratumProperties {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.StratumKeyPrefix, prefixEndBytes(types.StratumKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var strata []*types.StratumProperties
	for ; iter.Valid(); iter.Next() {
		var props types.StratumProperties
		if err := proto.Unmarshal(iter.Value(), &props); err != nil {
			continue
		}
		strata = append(strata, &props)
	}
	return strata
}

// ---------- Domain Operations ----------

// SetDomain stores a domain.
func (k Keeper) SetDomain(ctx sdk.Context, domain *types.Domain) {
	store := k.storeService.OpenKVStore(ctx)
	key := domainKey(domain.Name)
	bz, err := proto.Marshal(domain)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal domain: %v", err))
	}
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set domain: %v", err))
	}

	// Maintain stratum->domain index
	indexKey := domainByStratumKey(types.Stratum(domain.Stratum), domain.Name)
	if err := store.Set(indexKey, []byte(domain.Name)); err != nil {
		panic(fmt.Sprintf("failed to set domain index: %v", err))
	}
}

// GetDomain retrieves a domain by name.
func (k Keeper) GetDomain(ctx sdk.Context, name string) (*types.Domain, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := domainKey(name)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var domain types.Domain
	if err := proto.Unmarshal(bz, &domain); err != nil {
		return nil, false
	}
	return &domain, true
}

// GetDomainsByStratum returns all domains belonging to a specific stratum.
func (k Keeper) GetDomainsByStratum(ctx sdk.Context, stratum types.Stratum) []*types.Domain {
	store := k.storeService.OpenKVStore(ctx)
	prefix := domainByStratumPrefix(stratum)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var domains []*types.Domain
	for ; iter.Valid(); iter.Next() {
		domainName := string(iter.Value())
		domain, found := k.GetDomain(ctx, domainName)
		if found {
			domains = append(domains, domain)
		}
	}
	return domains
}

// GetAllDomains returns all registered domains.
func (k Keeper) GetAllDomains(ctx sdk.Context) []*types.Domain {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DomainKeyPrefix, prefixEndBytes(types.DomainKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var domains []*types.Domain
	for ; iter.Valid(); iter.Next() {
		var domain types.Domain
		if err := proto.Unmarshal(iter.Value(), &domain); err != nil {
			continue
		}
		domains = append(domains, &domain)
	}
	return domains
}

// DeleteDomain removes a domain from the store.
func (k Keeper) DeleteDomain(ctx sdk.Context, name string) {
	domain, found := k.GetDomain(ctx, name)
	if !found {
		return
	}
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(domainKey(name))
	_ = store.Delete(domainByStratumKey(types.Stratum(domain.Stratum), name))
}

// ---------- Proposal Operations ----------

// SetProposal stores a domain proposal.
func (k Keeper) SetProposal(ctx sdk.Context, proposal *types.DomainProposal) {
	store := k.storeService.OpenKVStore(ctx)
	key := proposalKey(proposal.Id)
	bz, err := proto.Marshal(proposal)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal proposal: %v", err))
	}
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set proposal: %v", err))
	}
}

// GetProposal retrieves a proposal by ID.
func (k Keeper) GetProposal(ctx sdk.Context, id string) (*types.DomainProposal, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := proposalKey(id)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var proposal types.DomainProposal
	if err := proto.Unmarshal(bz, &proposal); err != nil {
		return nil, false
	}
	return &proposal, true
}

// DeleteProposal removes a proposal from the store.
func (k Keeper) DeleteProposal(ctx sdk.Context, id string) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(proposalKey(id))
}

// IterateProposals iterates over all proposals. Return true from cb to stop.
func (k Keeper) IterateProposals(ctx sdk.Context, cb func(*types.DomainProposal) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ProposalKeyPrefix, prefixEndBytes(types.ProposalKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var proposal types.DomainProposal
		if err := proto.Unmarshal(iter.Value(), &proposal); err != nil {
			continue
		}
		if cb(&proposal) {
			break
		}
	}
}

// ---------- Cross-Stratum Link Operations ----------

// SetLink stores a cross-stratum link.
func (k Keeper) SetLink(ctx sdk.Context, link *types.CrossStratumLink) {
	store := k.storeService.OpenKVStore(ctx)
	key := linkKey(link.SourceDomain, link.TargetDomain)
	bz, err := proto.Marshal(link)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal link: %v", err))
	}
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set link: %v", err))
	}
}

// GetLink retrieves a cross-stratum link.
func (k Keeper) GetLink(ctx sdk.Context, source, target string) (*types.CrossStratumLink, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := linkKey(source, target)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var link types.CrossStratumLink
	if err := proto.Unmarshal(bz, &link); err != nil {
		return nil, false
	}
	return &link, true
}

// GetLinksBySource returns all cross-stratum links originating from the given source domain.
// Used by partnerships module for cross-stratum matching (R31-4).
func (k Keeper) GetLinksBySource(ctx sdk.Context, sourceDomain string) []*types.CrossStratumLink {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(types.LinkKeyPrefix, []byte(sourceDomain+"/")...)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var links []*types.CrossStratumLink
	for ; iter.Valid(); iter.Next() {
		var link types.CrossStratumLink
		if err := proto.Unmarshal(iter.Value(), &link); err != nil {
			continue
		}
		links = append(links, &link)
	}
	return links
}

// GetAllLinks returns all cross-stratum links.
func (k Keeper) GetAllLinks(ctx sdk.Context) []*types.CrossStratumLink {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.LinkKeyPrefix, prefixEndBytes(types.LinkKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var links []*types.CrossStratumLink
	for ; iter.Valid(); iter.Next() {
		var link types.CrossStratumLink
		if err := proto.Unmarshal(iter.Value(), &link); err != nil {
			continue
		}
		links = append(links, &link)
	}
	return links
}

// ---------- Logic Zone Operations ----------

// SetLogicZone stores logic zone properties.
func (k Keeper) SetLogicZone(ctx sdk.Context, props *types.LogicZoneProperties) {
	store := k.storeService.OpenKVStore(ctx)
	key := logicZoneKey(types.LogicZone(props.Zone))
	bz, err := json.Marshal(props)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal logic zone: %v", err))
	}
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set logic zone: %v", err))
	}
}

// GetLogicZone retrieves logic zone properties by zone name.
func (k Keeper) GetLogicZone(ctx sdk.Context, zone types.LogicZone) (*types.LogicZoneProperties, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := logicZoneKey(zone)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var props types.LogicZoneProperties
	if err := json.Unmarshal(bz, &props); err != nil {
		return nil, false
	}
	return &props, true
}

// GetAllLogicZones returns all registered logic zones.
func (k Keeper) GetAllLogicZones(ctx sdk.Context) []*types.LogicZoneProperties {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.LogicZoneKeyPrefix, prefixEndBytes(types.LogicZoneKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var zones []*types.LogicZoneProperties
	for ; iter.Valid(); iter.Next() {
		var props types.LogicZoneProperties
		if err := json.Unmarshal(iter.Value(), &props); err != nil {
			continue
		}
		zones = append(zones, &props)
	}
	return zones
}

// ---------- Incompleteness Acknowledgment Operations ----------

// SetIncompletenessAck stores an incompleteness acknowledgment.
func (k Keeper) SetIncompletenessAck(ctx sdk.Context, ack types.IncompletenessAcknowledgment) {
	store := k.storeService.OpenKVStore(ctx)
	key := incompletenessAckKey(ack.FactId)
	bz, err := json.Marshal(ack)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal incompleteness ack: %v", err))
	}
	if err := store.Set(key, bz); err != nil {
		panic(fmt.Sprintf("failed to set incompleteness ack: %v", err))
	}
}

// GetIncompletenessAck retrieves an incompleteness acknowledgment by fact ID.
func (k Keeper) GetIncompletenessAck(ctx sdk.Context, factId string) (types.IncompletenessAcknowledgment, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := incompletenessAckKey(factId)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.IncompletenessAcknowledgment{}, false
	}
	var ack types.IncompletenessAcknowledgment
	if err := json.Unmarshal(bz, &ack); err != nil {
		return types.IncompletenessAcknowledgment{}, false
	}
	return ack, true
}

// GetAllIncompletenessAcks returns all incompleteness acknowledgments.
func (k Keeper) GetAllIncompletenessAcks(ctx sdk.Context) []types.IncompletenessAcknowledgment {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.IncompletenessAckKeyPrefix, prefixEndBytes(types.IncompletenessAckKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var acks []types.IncompletenessAcknowledgment
	for ; iter.Valid(); iter.Next() {
		var ack types.IncompletenessAcknowledgment
		if err := json.Unmarshal(iter.Value(), &ack); err != nil {
			continue
		}
		acks = append(acks, ack)
	}
	return acks
}

// ---------- Key Construction Helpers ----------

func stratumKey(stratum types.Stratum) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(stratum))
	return append(types.StratumKeyPrefix, buf...)
}

func domainKey(name string) []byte {
	return append(types.DomainKeyPrefix, []byte(name)...)
}

func proposalKey(id string) []byte {
	return append(types.ProposalKeyPrefix, []byte(id)...)
}

func linkKey(source, target string) []byte {
	return append(types.LinkKeyPrefix, []byte(source+"/"+target)...)
}

func domainByStratumPrefix(stratum types.Stratum) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(stratum))
	return append(types.DomainByStratumPrefix, buf...)
}

func domainByStratumKey(stratum types.Stratum, domainName string) []byte {
	prefix := domainByStratumPrefix(stratum)
	return append(prefix, []byte(domainName)...)
}

func logicZoneKey(zone types.LogicZone) []byte {
	return append(types.LogicZoneKeyPrefix, []byte(zone)...)
}

func incompletenessAckKey(factId string) []byte {
	return append(types.IncompletenessAckKeyPrefix, []byte(factId)...)
}
