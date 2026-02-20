package types

const (
	// ModuleName is the name of the knowledge module.
	ModuleName = "knowledge"
	// StoreKey is the store key for the knowledge module.
	StoreKey = ModuleName
	// MemStoreKey is the in-memory store key.
	MemStoreKey = "mem_knowledge"
	// PortID is the IBC port ID.
	PortID = "knowledge"
	// Version is the IBC channel version.
	Version = "zrn-knowledge-1"
	// RouterKey is the message routing key.
	RouterKey = ModuleName
)

// Store key prefixes — one byte per sub-namespace.
// KV prefix ranges for knowledge module state.
var (
	// ─── Core state ──────────────────────────────────────────────────────────
	FactKeyPrefix              = []byte{0x01} // factID → Fact
	ClaimKeyPrefix             = []byte{0x02} // claimID → Claim
	VerificationRoundKeyPrefix = []byte{0x03} // roundID → VerificationRound
	FactReferenceKeyPrefix     = []byte{0x04} // factID:refID → exists
	DomainFactIndexPrefix      = []byte{0x05} // domain/factID → exists
	ParamsKey                  = []byte{0x06} // singleton Params
	DomainKeyPrefix            = []byte{0x07} // domainName → Domain
	IncomingRefIndexPrefix     = []byte{0x08} // toFactID:fromFactID → exists
	ContentHashIndexPrefix     = []byte{0x09} // contentHash → claimID (dedup)
	ClaimRoundIndexPrefix      = []byte{0x0a} // claimID → roundID
	EquivocationKeyPrefix      = []byte{0x0b} // roundID:validator → evidence

	// ─── Adversarial verification ────────────────────────────────────────────
	ProvisionalChallengeKeyPrefix = []byte{0x0c}
	ChallengerCooldownKeyPrefix   = []byte{0x0d}
	PendingEvalClaimIndexPrefix   = []byte{0x0e}
	SubmitterCalibrationPrefix    = []byte{0x0f}

	// ─── Negative knowledge ──────────────────────────────────────────────────
	CounterFactKeyPrefix             = []byte{0x10}
	CounterFactByFactIndexPrefix     = []byte{0x11}
	CounterFactByDomainIndexPrefix   = []byte{0x12}
	CounterFactByHeightIndexPrefix   = []byte{0x13}
	FactNegationLinkPrefix           = []byte{0x14}
	ContradictionCooldownPrefix      = []byte{0x15}
	FalsificationEpochPaidPrefix     = []byte{0x16}
	FalsificationCarryForwardPrefix  = []byte{0x17}
	CounterFactChallengeKeyPrefix    = []byte{0x18}
	CounterFactChallengeWindowPrefix = []byte{0x19}
	ExtendedParamsKey                = []byte{0x1a} // singleton JSON ExtendedParams
	PatronageRecordPrefix            = []byte{0x1b}
	PruningQueuePrefix               = []byte{0x1c}
	VerifierConformityPrefix         = []byte{0x1d} // FARM-1
	ValidatorParticipationPrefix     = []byte{0x1e} // FARM-8

	// ─── Secondary query indexes ─────────────────────────────────────────────
	FactBySubmitterIndexPrefix = []byte{0x1f} // submitter/factID → exists
	FactByDomainIndexPrefix    = []byte{0x20} // domain/factID → exists (mirror of 0x05)
	ActiveRoundIndexPrefix     = []byte{0x21} // roundID → exists

	// ─── Citation and domain strata ──────────────────────────────────────────
	CitationSourcePrefix = []byte{0x27} // FARM-11 citation-source tracking
	DomainStratumPrefix  = []byte{0x28} // FARM-12 domain-to-stratum mapping

	// ─── Research fund governance ────────────────────────────────────────────
	ResearchProposalPrefix  = []byte{0x29}
	ResearchVotePrefix      = []byte{0x2a}
	ResearchFundStatsPrefix = []byte{0x2b}

	// ─── Partnership citation stats ──────────────────────────────────────────
	PartnershipCitationStatsPrefix = []byte{0x2c}
)

// ─── Key constructors ─────────────────────────────────────────────────────────

// FactKey returns the store key for a fact.
func FactKey(id string) []byte {
	return append(append([]byte{}, FactKeyPrefix...), []byte(id)...)
}

// ClaimKey returns the store key for a claim.
func ClaimKey(id string) []byte {
	return append(append([]byte{}, ClaimKeyPrefix...), []byte(id)...)
}

// RoundKey returns the store key for a verification round.
func RoundKey(id string) []byte {
	return append(append([]byte{}, VerificationRoundKeyPrefix...), []byte(id)...)
}

// DomainKey returns the store key for a domain.
func DomainKey(name string) []byte {
	return append(append([]byte{}, DomainKeyPrefix...), []byte(name)...)
}

// ContentHashKey returns the index key for a content hash.
func ContentHashKey(hash string) []byte {
	return append(append([]byte{}, ContentHashIndexPrefix...), []byte(hash)...)
}

// ClaimRoundIndexKey returns the index key mapping a claim to its round.
func ClaimRoundIndexKey(claimID string) []byte {
	return append(append([]byte{}, ClaimRoundIndexPrefix...), []byte(claimID)...)
}

// FactBySubmitterKey returns the composite index key for facts by submitter.
func FactBySubmitterKey(submitter, factID string) []byte {
	key := append(append([]byte{}, FactBySubmitterIndexPrefix...), []byte(submitter)...)
	key = append(key, '/')
	return append(key, []byte(factID)...)
}

// FactByDomainKey returns the composite index key for facts by domain.
func FactByDomainKey(domain, factID string) []byte {
	key := append(append([]byte{}, DomainFactIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(factID)...)
}
