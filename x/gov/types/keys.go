package types

const (
	ModuleName = "zerone_gov"
	StoreKey   = ModuleName
	RouterKey  = ModuleName
)

// Store key prefixes.
var (
	LIPKeyPrefix            = []byte{0x01}
	VoteKeyPrefix           = []byte{0x02}
	VoteDedupePrefix        = []byte{0x03}
	ParamsKey               = []byte{0x04}
	LIPCounterKey           = []byte{0x05}
	ResearchSpendKeyPrefix  = []byte{0x06}
	ResearchVotersKey       = []byte{0x07}
	ResearchSpendCounterKey = []byte{0x08}
	FundingRecordKeyPrefix  = []byte{0x0A}
	SybilParamsKey          = []byte{0x0B}
	UpgradePlanKeyPrefix    = []byte{0x0C}
)

// LIPKey returns the store key for a LIP by id.
func LIPKey(lipID string) []byte {
	return append(LIPKeyPrefix, []byte(lipID)...)
}

// VoteKey returns the store key for a vote by lip_id + voter.
func VoteKey(lipID, voter string) []byte {
	key := append(VoteKeyPrefix, []byte(lipID)...)
	key = append(key, 0x00) // separator
	key = append(key, []byte(voter)...)
	return key
}

// VoteDedupeKey returns the dedupe key for a vote.
func VoteDedupeKey(lipID, voter string) []byte {
	key := append(VoteDedupePrefix, []byte(lipID)...)
	key = append(key, 0x00)
	key = append(key, []byte(voter)...)
	return key
}

// VotePrefixForLIP returns the prefix for all votes on a given LIP.
func VotePrefixForLIP(lipID string) []byte {
	key := append(VoteKeyPrefix, []byte(lipID)...)
	key = append(key, 0x00)
	return key
}

// UpgradePlanKey returns the store key for an upgrade plan by LIP ID.
func UpgradePlanKey(lipID string) []byte {
	return append(UpgradePlanKeyPrefix, []byte(lipID)...)
}

// ResearchSpendKey returns the store key for a research spend proposal by ID.
func ResearchSpendKey(proposalID uint64) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(proposalID >> 56)
	bz[1] = byte(proposalID >> 48)
	bz[2] = byte(proposalID >> 40)
	bz[3] = byte(proposalID >> 32)
	bz[4] = byte(proposalID >> 24)
	bz[5] = byte(proposalID >> 16)
	bz[6] = byte(proposalID >> 8)
	bz[7] = byte(proposalID)
	return append(ResearchSpendKeyPrefix, bz...)
}

// ResearchSpendIterPrefix returns the prefix for iterating all research spend proposals.
func ResearchSpendIterPrefix() []byte {
	return ResearchSpendKeyPrefix
}
