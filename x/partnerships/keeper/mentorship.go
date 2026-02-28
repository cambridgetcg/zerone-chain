package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- Mentorship CRUD ----------

func mentorshipKey(id string) []byte {
	return append(types.MentorshipKeyPrefix, []byte(id)...)
}

func byMentorKey(mentorAddr, id string) []byte {
	return append(types.ByMentorIndexPrefix, []byte(mentorAddr+"/"+id)...)
}

func byMenteeKey(menteeAddr, id string) []byte {
	return append(types.ByMenteeIndexPrefix, []byte(menteeAddr+"/"+id)...)
}

func (k Keeper) SetMentorship(ctx sdk.Context, m *types.Mentorship) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(m)
	if err != nil {
		panic("failed to marshal mentorship: " + err.Error())
	}
	_ = kvStore.Set(mentorshipKey(m.Id), bz)
	_ = kvStore.Set(byMentorKey(m.MentorAddr, m.Id), []byte(m.Id))
	_ = kvStore.Set(byMenteeKey(m.MenteeAddr, m.Id), []byte(m.Id))
}

func (k Keeper) GetMentorship(ctx sdk.Context, id string) (*types.Mentorship, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(mentorshipKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var m types.Mentorship
	if err := proto.Unmarshal(bz, &m); err != nil {
		return nil, false
	}
	return &m, true
}

func (k Keeper) DeleteMentorship(ctx sdk.Context, m *types.Mentorship) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(mentorshipKey(m.Id))
	_ = kvStore.Delete(byMentorKey(m.MentorAddr, m.Id))
	_ = kvStore.Delete(byMenteeKey(m.MenteeAddr, m.Id))
}

func (k Keeper) GetAllMentorships(ctx sdk.Context) []*types.Mentorship {
	var mentorships []*types.Mentorship
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.MentorshipKeyPrefix, prefixEndBytes(types.MentorshipKeyPrefix))
	if err != nil {
		return mentorships
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var m types.Mentorship
		if err := proto.Unmarshal(iter.Value(), &m); err == nil {
			mentorships = append(mentorships, &m)
		}
	}
	return mentorships
}

func (k Keeper) GetMentorshipsByMentor(ctx sdk.Context, mentorAddr string) []*types.Mentorship {
	var result []*types.Mentorship
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append(types.ByMentorIndexPrefix, []byte(mentorAddr+"/")...)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return result
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Value())
		if m, found := k.GetMentorship(ctx, id); found {
			result = append(result, m)
		}
	}
	return result
}

func (k Keeper) GetMentorshipsByMentee(ctx sdk.Context, menteeAddr string) []*types.Mentorship {
	var result []*types.Mentorship
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append(types.ByMenteeIndexPrefix, []byte(menteeAddr+"/")...)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return result
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Value())
		if m, found := k.GetMentorship(ctx, id); found {
			result = append(result, m)
		}
	}
	return result
}

func (k Keeper) CountActiveMentorshipsForMentor(ctx sdk.Context, mentorAddr string) int {
	mentorships := k.GetMentorshipsByMentor(ctx, mentorAddr)
	count := 0
	for _, m := range mentorships {
		if m.Status == "active" {
			count++
		}
	}
	return count
}

func (k Keeper) GetActiveMentorshipForMentee(ctx sdk.Context, menteeAddr string) (*types.Mentorship, bool) {
	mentorships := k.GetMentorshipsByMentee(ctx, menteeAddr)
	for _, m := range mentorships {
		if m.Status == "active" {
			return m, true
		}
	}
	return nil, false
}

// graduateMentorship transitions a mentorship to graduated status
// and optionally proposes a partnership.
func (k Keeper) graduateMentorship(ctx sdk.Context, m *types.Mentorship) {
	m.Status = "graduated"
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_graduated",
			sdk.NewAttribute("mentorship_id", m.Id),
			sdk.NewAttribute("mentor", m.MentorAddr),
			sdk.NewAttribute("mentee", m.MenteeAddr),
			sdk.NewAttribute("domain", m.Domain),
		),
	)

	params := k.GetParams(ctx)
	if params.AutoProposePartnershipOnGraduation {
		seq := k.NextSequence(ctx)
		partnershipId := fmt.Sprintf("partnership-%d", seq)
		currentBlock := uint64(ctx.BlockHeight())

		partnership := &types.Partnership{
			Id:               partnershipId,
			HumanAddr:        m.MentorAddr,
			AgentAddr:        m.MenteeAddr,
			Status:           types.StatusPending,
			Tier:             0,
			LockTier:         0,
			LockExpiresAt:    currentBlock + types.LockTiers[0].MinBlocks,
			SplitHumanBps:    params.DefaultHumanSplitBps,
			SplitAgentBps:    params.DefaultAgentSplitBps,
			CommonPotBalance: "0",
			TotalEarned:      "0",
			CooperationScore: 500000,
			FormedAtBlock:    currentBlock,
		}
		k.SetPartnership(ctx, partnership)

		kvStore := k.storeService.OpenKVStore(ctx)
		formationExpiry := currentBlock + params.FormationWindowBlocks
		_ = kvStore.Set(
			append(types.FormationKeyPrefix, []byte(partnershipId)...),
			[]byte(fmt.Sprintf("%d", formationExpiry)),
		)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.partnerships.partnership_proposed",
				sdk.NewAttribute("partnership_id", partnershipId),
				sdk.NewAttribute("proposer", m.MentorAddr),
				sdk.NewAttribute("partner", m.MenteeAddr),
				sdk.NewAttribute("source", "mentorship_graduation"),
			),
		)
	}
}

// AutoGraduateMentorships checks active mentorships for duration expiry
// and graduates them automatically.
func (k Keeper) AutoGraduateMentorships(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	mentorships := k.GetAllMentorships(ctx)

	for _, m := range mentorships {
		if m.Status != "active" {
			continue
		}
		if m.StartBlock > 0 && m.DurationBlocks > 0 && currentBlock >= m.StartBlock+m.DurationBlocks {
			k.graduateMentorship(ctx, m)
		}
	}
}
