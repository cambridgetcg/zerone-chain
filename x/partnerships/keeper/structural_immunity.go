package keeper

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- Domain Social Benefit Status (R31-5) ----------

// GetDomainSocialBenefitStatus returns true when a domain's social density
// meets or exceeds the SocialSaturationThreshold (R31-5: Water → Fire).
func (k Keeper) GetDomainSocialBenefitStatus(ctx sdk.Context, domain string) bool {
	params := k.GetParams(ctx)
	threshold := params.SocialSaturationThreshold
	if threshold == 0 {
		threshold = 4 // safety default
	}
	density := k.GetDomainPartnershipDensity(ctx, domain)
	return density >= threshold
}

// ---------- Domain Partnership Density (R29-5) ----------

// GetDomainPartnershipDensity counts unique participants in active partnerships
// and mentorships for a domain. Higher density = more distributed participation.
func (k Keeper) GetDomainPartnershipDensity(ctx sdk.Context, domain string) uint64 {
	uniqueParticipants := make(map[string]bool)

	// Count from active partnerships — partnerships don't have a domain field,
	// so we count all active partnerships (domain-agnostic structural density).
	k.IteratePartnerships(ctx, func(p *types.Partnership) bool {
		if p.Status == types.StatusActive {
			uniqueParticipants[p.HumanAddr] = true
			uniqueParticipants[p.AgentAddr] = true
		}
		return false
	})

	// Count from active mentorships in this specific domain.
	mentorships := k.GetAllMentorships(ctx)
	for _, m := range mentorships {
		if m.Status == "active" && m.Domain == domain {
			uniqueParticipants[m.MentorAddr] = true
			uniqueParticipants[m.MenteeAddr] = true
		}
	}

	return uint64(len(uniqueParticipants))
}

// GetPartnershipCountByParticipant returns the count of active partnerships
// for a participant in a domain. Since partnerships don't have domain fields,
// this counts all active partnerships for the address.
func (k Keeper) GetPartnershipCountByParticipant(ctx sdk.Context, addr string, _ string) uint64 {
	partnerships := k.GetPartnershipsByParticipant(ctx, addr)
	count := uint64(0)
	for _, p := range partnerships {
		if p.Status == types.StatusActive {
			count++
		}
	}

	// Also count active mentorships as partnerships
	mentorships := k.GetMentorshipsByMentor(ctx, addr)
	for _, m := range mentorships {
		if m.Status == "active" {
			count++
		}
	}
	mentorships = k.GetMentorshipsByMentee(ctx, addr)
	for _, m := range mentorships {
		if m.Status == "active" {
			count++
		}
	}

	return count
}

// ---------- Formation Bonus CRUD (R29-5) ----------

func formationBonusKey(domain string) []byte {
	return append(types.FormationBonusKeyPrefix, []byte(domain)...)
}

// SetDomainFormationBonus stores a formation bonus for a domain.
func (k Keeper) SetDomainFormationBonus(ctx sdk.Context, domain string, bonusBps uint64, reason string, expiryHeight uint64) {
	bonus := &types.FormationBonus{
		Domain:       domain,
		BonusBps:     bonusBps,
		Reason:       reason,
		ExpiryHeight: expiryHeight,
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(bonus)
	if err != nil {
		panic("failed to marshal formation bonus: " + err.Error())
	}
	_ = kvStore.Set(formationBonusKey(domain), bz)
}

// GetDomainFormationBonus returns the formation bonus for a domain, if any.
func (k Keeper) GetDomainFormationBonus(ctx sdk.Context, domain string) *types.FormationBonus {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(formationBonusKey(domain))
	if err != nil || bz == nil {
		return nil
	}
	var bonus types.FormationBonus
	if err := json.Unmarshal(bz, &bonus); err != nil {
		return nil
	}
	return &bonus
}

// DeleteDomainFormationBonus removes a formation bonus.
func (k Keeper) DeleteDomainFormationBonus(ctx sdk.Context, domain string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(formationBonusKey(domain))
}

// ExpireFormationBonuses removes expired formation bonuses.
func (k Keeper) ExpireFormationBonuses(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.FormationBonusKeyPrefix, prefixEndBytes(types.FormationBonusKeyPrefix))
	if err != nil {
		return
	}

	var toDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		var bonus types.FormationBonus
		if err := json.Unmarshal(iter.Value(), &bonus); err != nil {
			continue
		}
		if bonus.ExpiryHeight > 0 && bonus.ExpiryHeight <= currentBlock {
			toDelete = append(toDelete, append([]byte{}, iter.Key()...))
		}
	}
	iter.Close()

	for _, key := range toDelete {
		_ = kvStore.Delete(key)
	}
}

// ---------- Domain Formation Freeze CRUD (R31-3) ----------

func formationFreezeKey(domain string) []byte {
	return append(types.FormationFreezeKeyPrefix, []byte(domain)...)
}

// SetDomainFormationFreeze stores a formation freeze for a domain.
func (k Keeper) SetDomainFormationFreeze(ctx sdk.Context, domain string, expiryHeight uint64, reason string) {
	freeze := &types.DomainFormationFreeze{
		Domain:       domain,
		ExpiryHeight: expiryHeight,
		Reason:       reason,
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(freeze)
	if err != nil {
		panic("failed to marshal formation freeze: " + err.Error())
	}
	_ = kvStore.Set(formationFreezeKey(domain), bz)
}

// GetDomainFormationFreeze returns the formation freeze for a domain, if any.
func (k Keeper) GetDomainFormationFreeze(ctx sdk.Context, domain string) *types.DomainFormationFreeze {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(formationFreezeKey(domain))
	if err != nil || bz == nil {
		return nil
	}
	var freeze types.DomainFormationFreeze
	if err := json.Unmarshal(bz, &freeze); err != nil {
		return nil
	}
	return &freeze
}

// DeleteDomainFormationFreeze removes a formation freeze.
func (k Keeper) DeleteDomainFormationFreeze(ctx sdk.Context, domain string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(formationFreezeKey(domain))
}

// ExpireFormationFreezes removes expired formation freezes.
func (k Keeper) ExpireFormationFreezes(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.FormationFreezeKeyPrefix, prefixEndBytes(types.FormationFreezeKeyPrefix))
	if err != nil {
		return
	}

	var toDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		var freeze types.DomainFormationFreeze
		if err := json.Unmarshal(iter.Value(), &freeze); err != nil {
			continue
		}
		if freeze.ExpiryHeight > 0 && freeze.ExpiryHeight <= currentBlock {
			toDelete = append(toDelete, append([]byte{}, iter.Key()...))
		}
	}
	iter.Close()

	for _, key := range toDelete {
		_ = kvStore.Delete(key)
	}
}
