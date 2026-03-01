package keeper

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- FormationMatch CRUD ----------

func formationMatchKey(id string) []byte {
	return append(types.FormationMatchKeyPrefix, []byte(id)...)
}

func (k Keeper) SetFormationMatch(ctx sdk.Context, fm *types.FormationMatch) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(fm)
	if err != nil {
		panic("failed to marshal formation match: " + err.Error())
	}
	_ = kvStore.Set(formationMatchKey(fm.Id), bz)
}

func (k Keeper) GetFormationMatch(ctx sdk.Context, id string) (*types.FormationMatch, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(formationMatchKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var fm types.FormationMatch
	if err := proto.Unmarshal(bz, &fm); err != nil {
		return nil, false
	}
	return &fm, true
}

func (k Keeper) DeleteFormationMatch(ctx sdk.Context, id string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(formationMatchKey(id))
}

func (k Keeper) GetAllFormationMatches(ctx sdk.Context) []*types.FormationMatch {
	var matches []*types.FormationMatch
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.FormationMatchKeyPrefix, prefixEndBytes(types.FormationMatchKeyPrefix))
	if err != nil {
		return matches
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var fm types.FormationMatch
		if err := proto.Unmarshal(iter.Value(), &fm); err == nil {
			matches = append(matches, &fm)
		}
	}
	return matches
}

// ---------- Matching Engine ----------

const maxMatchingEntries = 200

func (k Keeper) RunFormationMatching(ctx sdk.Context) {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	// R29-6: Adaptive formation interval — degraded health slows formation.
	effectiveInterval := params.FormationMatchIntervalBlocks
	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing != 1_000_000 {
			effectiveInterval = params.FormationMatchIntervalBlocks * 1_000_000 / creationPacing
		}
	}
	if effectiveInterval == 0 {
		return
	}

	// R31-5: Earth → Water — param changes reset matching cycle.
	lastParamUpdate := k.GetLastParamUpdateHeight(ctx)
	cycleBase := lastParamUpdate // reset cycle origin on param change
	if currentBlock < cycleBase || (currentBlock-cycleBase)%effectiveInterval != 0 {
		return
	}

	allEntries := k.GetAllPoolEntries(ctx)
	var entries []*types.PoolEntry
	for _, pe := range allEntries {
		if pe.Status == "active" && pe.MatchedWith == "" {
			entries = append(entries, pe)
		}
	}

	if len(entries) < 2 {
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Address < entries[j].Address
	})

	if len(entries) > maxMatchingEntries {
		entries = entries[:maxMatchingEntries]
	}

	matched := make(map[string]bool)
	for i := 0; i < len(entries); i++ {
		e1 := entries[i]
		if matched[e1.Address] {
			continue
		}

		bestScore := uint64(0)
		bestIdx := -1
		for j := i + 1; j < len(entries); j++ {
			e2 := entries[j]
			if matched[e2.Address] {
				continue
			}
			score := scoreCompatibility(e1, e2, currentBlock)
			if score > bestScore {
				bestScore = score
				bestIdx = j
			}
		}

		if bestIdx >= 0 && bestScore > 0 {
			e2 := entries[bestIdx]

			// R29-5: Boost match score for domains with active formation bonuses.
			bestScore = k.applyFormationBonus(ctx, e1, e2, bestScore)
			matched[e1.Address] = true
			matched[e2.Address] = true

			seq := k.NextSequence(ctx)
			matchId := fmt.Sprintf("match-%d", seq)

			fm := &types.FormationMatch{
				Id:         matchId,
				Addr1:      e1.Address,
				Addr2:      e2.Address,
				Score:      bestScore,
				ProposedAt: currentBlock,
				ExpiresAt:  currentBlock + params.MatchAcceptanceBlocks,
				Status:     "proposed",
			}
			k.SetFormationMatch(ctx, fm)

			e1.MatchedWith = matchId
			k.SetPoolEntry(ctx, e1)
			e2.MatchedWith = matchId
			k.SetPoolEntry(ctx, e2)

			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.partnerships.formation_match_proposed",
					sdk.NewAttribute("match_id", matchId),
					sdk.NewAttribute("addr1", e1.Address),
					sdk.NewAttribute("addr2", e2.Address),
					sdk.NewAttribute("score", fmt.Sprintf("%d", bestScore)),
				),
			)
		}
	}
}

func scoreCompatibility(e1, e2 *types.PoolEntry, currentBlock uint64) uint64 {
	score := uint64(0)

	// Domain overlap: (shared / max(len1, len2)) * 5000
	shared := 0
	domainSet := make(map[string]bool)
	for _, d := range e1.Domains {
		domainSet[d] = true
	}
	for _, d := range e2.Domains {
		if domainSet[d] {
			shared++
		}
	}
	maxDomains := len(e1.Domains)
	if len(e2.Domains) > maxDomains {
		maxDomains = len(e2.Domains)
	}
	if maxDomains > 0 {
		score += uint64(shared) * 5000 / uint64(maxDomains)
	}

	// Preferred role compatibility
	if isComplementary(e1.PreferredRole, e2.PreferredRole) {
		score += 3000
	} else if e1.PreferredRole == "any" || e2.PreferredRole == "any" || e1.PreferredRole == "" || e2.PreferredRole == "" {
		score += 1500
	}

	// Time in pool: min(avg_time / 1000, 2000)
	avgTime := uint64(0)
	if e1.RegisteredAt > 0 && currentBlock > e1.RegisteredAt {
		avgTime += currentBlock - e1.RegisteredAt
	}
	if e2.RegisteredAt > 0 && currentBlock > e2.RegisteredAt {
		avgTime += currentBlock - e2.RegisteredAt
	}
	avgTime /= 2
	timeScore := avgTime / 1000
	if timeScore > 2000 {
		timeScore = 2000
	}
	score += timeScore

	return score
}

func isComplementary(r1, r2 string) bool {
	return (r1 == "human" && r2 == "agent") || (r1 == "agent" && r2 == "human")
}

// applyFormationBonus boosts match scores when either entry's domains have an active
// formation bonus (R29-5). Flagged domains attract more partnerships.
func (k Keeper) applyFormationBonus(ctx sdk.Context, e1, e2 *types.PoolEntry, baseScore uint64) uint64 {
	currentBlock := uint64(ctx.BlockHeight())
	bestBonusBps := uint64(0)

	// Check all domains from both entries for active bonuses.
	allDomains := make(map[string]bool)
	for _, d := range e1.Domains {
		allDomains[d] = true
	}
	for _, d := range e2.Domains {
		allDomains[d] = true
	}

	for domain := range allDomains {
		bonus := k.GetDomainFormationBonus(ctx, domain)
		if bonus != nil && bonus.ExpiryHeight > currentBlock && bonus.BonusBps > bestBonusBps {
			bestBonusBps = bonus.BonusBps
		}
	}

	if bestBonusBps > 0 {
		return baseScore * (1_000_000 + bestBonusBps) / 1_000_000
	}
	return baseScore
}

// ---------- Expiry ----------

func (k Keeper) ExpireFormationMatches(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	matches := k.GetAllFormationMatches(ctx)

	for _, fm := range matches {
		if fm.Status == "proposed" && fm.ExpiresAt > 0 && fm.ExpiresAt <= currentBlock {
			fm.Status = "expired"
			k.SetFormationMatch(ctx, fm)

			if pe, found := k.GetPoolEntry(ctx, fm.Addr1); found && pe.MatchedWith == fm.Id {
				pe.MatchedWith = ""
				k.SetPoolEntry(ctx, pe)
			}
			if pe, found := k.GetPoolEntry(ctx, fm.Addr2); found && pe.MatchedWith == fm.Id {
				pe.MatchedWith = ""
				k.SetPoolEntry(ctx, pe)
			}
		}
	}
}
