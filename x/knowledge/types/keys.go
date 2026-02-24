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
	// BootstrapFundModuleName is the module account that holds the knowledge bootstrap fund.
	BootstrapFundModuleName = "knowledge_bootstrap_fund"
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

	// ─── Semantic relations (knowledge graph edges) ──────────────────────────
	FactRelationPrefix        = []byte{0x30} // 0x30 | source_fact_id / target_fact_id → FactRelation
	FactRelationReversePrefix = []byte{0x31} // 0x31 | target_fact_id / source_fact_id → FactRelation (reverse index)

	// ─── Structured claim indexes ────────────────────────────────────────────
	FactSubjectPrefix = []byte{0x32} // 0x32 | domain / subject_hash → fact_id
	FactTagPrefix     = []byte{0x33} // 0x33 | tag / fact_id → []byte{1}

	// ─── Canonical form dedup ────────────────────────────────────────────────
	CanonicalHashPrefix = []byte{0x34} // 0x34 | canonical_hash → claim_id/fact_id

	// ─── Bootstrap fund tracking ─────────────────────────────────────────────
	BootstrapClaimCountPrefix = []byte{0x35} // 0x35 | address → uint64 (lifetime sponsored count)
	BootstrapEpochCountPrefix = []byte{0x36} // 0x36 | epoch_number → uint64 (epoch-wide count)

	// ─── Metabolism tracking ────────────────────────────────────────────────
	NewCitationsEpochPrefix = []byte{0x37} // 0x37 | fact_id → uint64 (new citations this epoch)
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

// FactRelationKey returns the forward index key for a fact relation.
func FactRelationKey(sourceFactID, targetFactID string) []byte {
	key := append(append([]byte{}, FactRelationPrefix...), []byte(sourceFactID)...)
	key = append(key, '/')
	return append(key, []byte(targetFactID)...)
}

// FactRelationReverseKey returns the reverse index key for a fact relation.
func FactRelationReverseKey(targetFactID, sourceFactID string) []byte {
	key := append(append([]byte{}, FactRelationReversePrefix...), []byte(targetFactID)...)
	key = append(key, '/')
	return append(key, []byte(sourceFactID)...)
}

// FactRelationsBySourcePrefix returns the prefix for iterating all relations from a source fact.
func FactRelationsBySourcePrefix(sourceFactID string) []byte {
	key := append(append([]byte{}, FactRelationPrefix...), []byte(sourceFactID)...)
	return append(key, '/')
}

// FactRelationsByTargetPrefix returns the prefix for iterating all relations to a target fact.
func FactRelationsByTargetPrefix(targetFactID string) []byte {
	key := append(append([]byte{}, FactRelationReversePrefix...), []byte(targetFactID)...)
	return append(key, '/')
}

// FactSubjectKey returns the index key for a fact's subject within a domain.
// Subject is stored as a SHA-256 hash to normalize and bound key length.
func FactSubjectKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, FactSubjectPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// FactTagKey returns the index key for a fact tagged with a given tag.
func FactTagKey(tag, factID string) []byte {
	key := append(append([]byte{}, FactTagPrefix...), []byte(tag)...)
	key = append(key, '/')
	return append(key, []byte(factID)...)
}

// FactTagsByTagPrefix returns the prefix for iterating all facts with a given tag.
func FactTagsByTagPrefix(tag string) []byte {
	key := append(append([]byte{}, FactTagPrefix...), []byte(tag)...)
	return append(key, '/')
}

// CanonicalHashKey returns the index key for a canonical form hash.
func CanonicalHashKey(hash string) []byte {
	return append(append([]byte{}, CanonicalHashPrefix...), []byte(hash)...)
}
