package types

import "encoding/binary"

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

	// ─── Novelty detection ──────────────────────────────────────────────────
	CommonKnowledgePrefix = []byte{0x38} // 0x38 | domain / subject_hash → CommonKnowledgeEntry

	// ─── Agent demand tracking ─────────────────────────────────────────
	DemandSignalPrefix          = []byte{0x39} // domain / subject_hash → DemandSignal
	BountyPrefix                = []byte{0x3a} // bounty_id → KnowledgeBounty
	BountyByDomainSubjectPrefix = []byte{0x3b} // domain / subject_hash → bounty_id (active index)

	// ─── Niche competition ──────────────────────────────────────────────
	NicheIndexPrefix   = []byte{0x3c} // 0x3c | niche_key | fact_id → []byte{1}
	NicheMembersPrefix = []byte{0x3d} // 0x3d | niche_key → []byte{1} (niche existence)

	// ─── Query satisfaction ─────────────────────────────────────────────
	QueryReceiptPrefix = []byte{0x3e} // 0x3e | rater / fact_id → block height (query receipt)

	// ─── Consensus diversity (R28-2) ────────────────────────────────────
	RoundDiversityPrefix         = []byte{0x40} // 0x40 | roundID → RoundDiversity (JSON)
	DomainDiversityPrefix        = []byte{0x41} // 0x41 | domain / epoch_bytes → DomainDiversityScore (JSON)
	ValidatorIndependencePrefix  = []byte{0x42} // 0x42 | validatorAddr → ValidatorIndependence (JSON)
	ConformityStreakPrefix       = []byte{0x43} // 0x43 | domain → ConformityStreak (JSON)
	DomainEpochRoundIndexPrefix = []byte{0x44} // 0x44 | domain / epoch_bytes / roundID → 0x01

	// ─── Retroactive vindication (R28-1) ────────────────────────────────
	VindicationPendingPrefix = []byte{0x50} // 0x50 | factID → []VindicationEntry (JSON)
	VindicationRecordPrefix  = []byte{0x51} // 0x51 | factID / verifier → VindicationRecord (JSON)

	// ─── Capture defense overrides (R28-8) ──────────────────────────────
	VerificationThresholdOverrideKeyPrefix = []byte{0x52} // 0x52 | domain → threshold override (binary)

	// ─── Epistemic temperature (R29-2) ─────────────────────────────────
	EpistemicStatePrefix = []byte{0x53} // 0x53 | domain → DomainEpistemicState (JSON)

	// ─── Domain carrying capacity (R29-1) ──────────────────────────────
	DomainStatsPrefix = []byte{0x54} // 0x54 | domain → DomainStats (JSON)

	// ─── Domain role elasticity (R29-3) ────────────────────────────────
	DomainRoleRecordPrefix = []byte{0x55} // 0x55 | domain → DomainRoleRecord (JSON)

	// ─── Adaptive pacing (R29-6) ───────────────────────────────────────
	LastClaimHeightKeyPrefix = []byte{0x56} // 0x56 | submitter → uint64 (last claim block height)

	// ─── Completion index (R31-2: Fire activity metrics) ──────────────
	CompletedRoundIndexPrefix = []byte{0x57} // 0x57 | verdictBlock(8) | roundID → CompletedRoundMeta (proto)
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

// CommonKnowledgeKey returns the store key for a common knowledge entry by domain and subject hash.
func CommonKnowledgeKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, CommonKnowledgePrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// CommonKnowledgeByDomainPrefix returns the prefix for iterating all common knowledge entries in a domain.
func CommonKnowledgeByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, CommonKnowledgePrefix...), []byte(domain)...)
	return append(key, '/')
}

// DemandSignalKey returns the store key for a demand signal.
func DemandSignalKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, DemandSignalPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// BountyKey returns the store key for a bounty.
func BountyKey(id string) []byte {
	return append(append([]byte{}, BountyPrefix...), []byte(id)...)
}

// BountyByDomainSubjectKey returns the index key for active bounties by domain/subject.
func BountyByDomainSubjectKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, BountyByDomainSubjectPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// NicheIndexKey returns the composite index key for a fact within a niche.
func NicheIndexKey(nicheKey, factID string) []byte {
	key := append(append([]byte{}, NicheIndexPrefix...), []byte(nicheKey)...)
	key = append(key, '/')
	return append(key, []byte(factID)...)
}

// NicheIndexByNichePrefix returns the prefix for iterating all facts in a niche.
func NicheIndexByNichePrefix(nicheKey string) []byte {
	key := append(append([]byte{}, NicheIndexPrefix...), []byte(nicheKey)...)
	return append(key, '/')
}

// NicheMembersKey returns the key for registering a niche's existence.
func NicheMembersKey(nicheKey string) []byte {
	return append(append([]byte{}, NicheMembersPrefix...), []byte(nicheKey)...)
}

// QueryReceiptKey returns the key for a query receipt: 0x3e | rater / factID.
func QueryReceiptKey(rater, factID string) []byte {
	key := append(append([]byte{}, QueryReceiptPrefix...), []byte(rater)...)
	key = append(key, '/')
	return append(key, []byte(factID)...)
}

// RoundDiversityKey returns the store key for a round's diversity data.
func RoundDiversityKey(roundID string) []byte {
	return append(append([]byte{}, RoundDiversityPrefix...), []byte(roundID)...)
}

// DomainDiversityKey returns the store key for a domain's epoch diversity score.
func DomainDiversityKey(domain string, epoch uint64) []byte {
	key := append(append([]byte{}, DomainDiversityPrefix...), []byte(domain)...)
	key = append(key, '/')
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epoch)
	return append(key, epochBz...)
}

// DomainDiversityByDomainPrefix returns the prefix for iterating all epochs for a domain.
func DomainDiversityByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, DomainDiversityPrefix...), []byte(domain)...)
	return append(key, '/')
}

// ValidatorIndependenceKey returns the store key for a validator's independence score.
func ValidatorIndependenceKey(validator string) []byte {
	return append(append([]byte{}, ValidatorIndependencePrefix...), []byte(validator)...)
}

// ConformityStreakKey returns the store key for a domain's conformity streak.
func ConformityStreakKey(domain string) []byte {
	return append(append([]byte{}, ConformityStreakPrefix...), []byte(domain)...)
}

// DomainEpochRoundKey returns the index key for a round in a domain's epoch.
func DomainEpochRoundKey(domain string, epoch uint64, roundID string) []byte {
	key := append(append([]byte{}, DomainEpochRoundIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epoch)
	key = append(key, epochBz...)
	key = append(key, '/')
	return append(key, []byte(roundID)...)
}

// DomainEpochRoundPrefix returns the prefix for iterating all rounds in a domain's epoch.
func DomainEpochRoundPrefix(domain string, epoch uint64) []byte {
	key := append(append([]byte{}, DomainEpochRoundIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	epochBz := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBz, epoch)
	key = append(key, epochBz...)
	return append(key, '/')
}

// VindicationPendingKey returns the store key for pending vindications for a fact.
func VindicationPendingKey(factId string) []byte {
	return append(append([]byte{}, VindicationPendingPrefix...), []byte(factId)...)
}

// VindicationRecordKey returns the store key for a vindication record.
func VindicationRecordKey(factId, verifier string) []byte {
	key := append([]byte{}, VindicationRecordPrefix...)
	key = append(key, []byte(factId)...)
	key = append(key, '/')
	key = append(key, []byte(verifier)...)
	return key
}

// VindicationRecordPrefixForFact returns the prefix for iterating all records for a fact.
func VindicationRecordPrefixForFact(factId string) []byte {
	key := append([]byte{}, VindicationRecordPrefix...)
	key = append(key, []byte(factId)...)
	key = append(key, '/')
	return key
}

// EpistemicStateKey returns the store key for a domain's epistemic state.
func EpistemicStateKey(domain string) []byte {
	return append(append([]byte{}, EpistemicStatePrefix...), []byte(domain)...)
}

// DomainStatsKey returns the store key for a domain's population stats.
func DomainStatsKey(domain string) []byte {
	return append(append([]byte{}, DomainStatsPrefix...), []byte(domain)...)
}

// DomainRoleRecordKey returns the store key for a domain's role track record.
func DomainRoleRecordKey(domain string) []byte {
	return append(append([]byte{}, DomainRoleRecordPrefix...), []byte(domain)...)
}

// LastClaimHeightKey returns the store key for a submitter's last claim height.
func LastClaimHeightKey(submitter string) []byte {
	return append(append([]byte{}, LastClaimHeightKeyPrefix...), []byte(submitter)...)
}

// CompletedRoundKey returns the index key for a completed round by verdict block.
func CompletedRoundKey(verdictBlock uint64, roundID string) []byte {
	key := make([]byte, 0, 1+8+len(roundID))
	key = append(key, CompletedRoundIndexPrefix...)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, verdictBlock)
	key = append(key, buf...)
	key = append(key, []byte(roundID)...)
	return key
}

// CompletedRoundBlockPrefix returns the prefix for iterating completed rounds starting at a block.
// Use with start=CompletedRoundBlockPrefix(startBlock) and end=CompletedRoundBlockPrefix(endBlock+1).
func CompletedRoundBlockPrefix(block uint64) []byte {
	key := make([]byte, 0, 1+8)
	key = append(key, CompletedRoundIndexPrefix...)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, block)
	key = append(key, buf...)
	return key
}
