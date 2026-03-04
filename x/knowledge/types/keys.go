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
var (
	// ─── Core state ──────────────────────────────────────────────────────────
	SampleKeyPrefix     = []byte{0x01} // sampleID → Sample
	SubmissionKeyPrefix = []byte{0x02} // submissionID → Submission
	QualityRoundPrefix  = []byte{0x03} // roundID → QualityRound
	DomainKeyPrefix     = []byte{0x04} // domainName → Domain
	DatasetKeyPrefix    = []byte{0x05} // datasetID → Dataset
	TrainingDemandKey   = []byte{0x06} // domain/subject → TrainingDemand
	DataBountyKeyPrefix = []byte{0x07} // bountyID → DataBounty
	ScrapedSourceKey    = []byte{0x08} // sourceID → ScrapedSourceEntry
	ValidatorInfoKey    = []byte{0x09} // address → ValidatorInfo
	ParamsKey           = []byte{0x0F} // singleton Params

	// ─── Indexes ─────────────────────────────────────────────────────────────
	ThreadIndexPrefix       = []byte{0x0A} // thread_id/sample_id → exists
	DomainSampleIndexPrefix = []byte{0x0B} // domain/sample_id → exists
	SubmitterIndexPrefix    = []byte{0x0C} // submitter/sample_id → exists
	NicheIndexPrefix        = []byte{0x0D} // niche_key/sample_id → exists
	ContentHashIndexPrefix  = []byte{0x0E} // content_hash → submission_id (dedup)

	SubmissionDomainIndexPrefix    = []byte{0x11} // domain/submissionID → exists
	SubmissionSubmitterIndexPrefix = []byte{0x12} // submitter/submissionID → exists

	// ─── Sequences ───────────────────────────────────────────────────────────
	SampleSeqKey     = []byte{0x80} // uint64 next sample ID
	SubmissionSeqKey = []byte{0x81} // uint64 next submission ID
	RoundSeqKey      = []byte{0x82} // uint64 next round ID
	DatasetSeqKey    = []byte{0x83} // uint64 next dataset ID

	// ─── Submission → round mapping ──────────────────────────────────────────
	SubmissionRoundIndexPrefix = []byte{0x10} // submissionID → roundID
	ActiveRoundIndexPrefix     = []byte{0x21} // roundID → exists

	// ─── Research fund governance ────────────────────────────────────────────
	ResearchProposalPrefix  = []byte{0x29}
	ResearchVotePrefix      = []byte{0x2a}
	ResearchFundStatsPrefix = []byte{0x2b}

	// ─── Partnership citation stats ──────────────────────────────────────────
	PartnershipCitationStatsPrefix = []byte{0x2c}

	// ─── Niche competition ──────────────────────────────────────────────────
	NicheMembersPrefix = []byte{0x3d} // niche_key → exists

	// ─── Query satisfaction ─────────────────────────────────────────────────
	QueryReceiptPrefix = []byte{0x3e} // rater/sample_id → block height

	// ─── Consensus diversity (R28-2) ────────────────────────────────────────
	RoundDiversityPrefix         = []byte{0x40} // roundID → RoundDiversity (JSON)
	DomainDiversityPrefix        = []byte{0x41} // domain/epoch_bytes → DomainDiversityScore (JSON)
	ValidatorIndependencePrefix  = []byte{0x42} // validatorAddr → ValidatorIndependence (JSON)
	ConformityStreakPrefix       = []byte{0x43} // domain → ConformityStreak (JSON)
	DomainEpochRoundIndexPrefix = []byte{0x44} // domain/epoch_bytes/roundID → 0x01

	// ─── Retroactive vindication (R28-1) ────────────────────────────────────
	VindicationPendingPrefix = []byte{0x50} // sampleID → []VindicationEntry (JSON)
	VindicationRecordPrefix  = []byte{0x51} // sampleID/verifier → VindicationRecord (JSON)

	// ─── Capture defense overrides (R28-8) ──────────────────────────────────
	VerificationThresholdOverrideKeyPrefix = []byte{0x52} // domain → threshold override

	// ─── Epistemic temperature (R29-2) ─────────────────────────────────────
	EpistemicStatePrefix = []byte{0x53} // domain → DomainEpistemicState (JSON)

	// ─── Domain carrying capacity (R29-1) ──────────────────────────────────
	DomainStatsPrefix = []byte{0x54} // domain → DomainStats (JSON)

	// ─── Domain role elasticity (R29-3) ────────────────────────────────────
	DomainRoleRecordPrefix = []byte{0x55} // domain → DomainRoleRecord (JSON)

	// ─── Adaptive pacing (R29-6) ───────────────────────────────────────────
	LastSubmissionHeightKeyPrefix = []byte{0x56} // submitter → uint64 (last submission block)

	// ─── Completion index (R31-2) ──────────────────────────────────────────
	CompletedRoundIndexPrefix = []byte{0x57} // verdictBlock(8)/roundID → CompletedRoundMeta

	// ─── Deprecated aliases (used by keeper until migration) ───────────────
	// TODO(R36-5): remove after keeper migration
	FactKeyPrefix              = SampleKeyPrefix
	ClaimKeyPrefix             = SubmissionKeyPrefix
	VerificationRoundKeyPrefix = QualityRoundPrefix
	ClaimRoundIndexPrefix      = SubmissionRoundIndexPrefix
	FactBySubmitterIndexPrefix = SubmitterIndexPrefix
	DomainFactIndexPrefix      = DomainSampleIndexPrefix
	LastClaimHeightKeyPrefix   = LastSubmissionHeightKeyPrefix
	EquivocationKeyPrefix      = []byte{0x90} // legacy, moved out of core range
	FactReferenceKeyPrefix     = []byte{0x91} // legacy
	IncomingRefIndexPrefix     = []byte{0x92} // legacy
	FactRelationPrefix         = []byte{0x30} // legacy semantic relations
	FactRelationReversePrefix  = []byte{0x31} // legacy reverse index
	FactSubjectPrefix          = []byte{0x32} // legacy structured claim index
	FactTagPrefix              = []byte{0x33} // legacy tag index
	CanonicalHashPrefix        = []byte{0x34} // legacy canonical form dedup
	BountyPrefix               = DataBountyKeyPrefix
	BountyByDomainSubjectPrefix = []byte{0x3b}
	DemandSignalPrefix          = TrainingDemandKey
	CommonKnowledgePrefix       = ScrapedSourceKey
	BootstrapClaimCountPrefix   = []byte{0x35}
	BootstrapEpochCountPrefix   = []byte{0x36}
	NewCitationsEpochPrefix     = []byte{0x37}
	CitationSourcePrefix        = []byte{0x27}
	DomainStratumPrefix         = []byte{0x28}
	ProvisionalChallengeKeyPrefix = []byte{0x93}
	ChallengerCooldownKeyPrefix   = []byte{0x94}
	PendingEvalClaimIndexPrefix   = []byte{0x95}
	SubmitterCalibrationPrefix    = []byte{0x96}
	CounterFactKeyPrefix             = []byte{0x97}
	CounterFactByFactIndexPrefix     = []byte{0x98}
	CounterFactByDomainIndexPrefix   = []byte{0x99}
	CounterFactByHeightIndexPrefix   = []byte{0x9a}
	FactNegationLinkPrefix           = []byte{0x9b}
	ContradictionCooldownPrefix      = []byte{0x9c}
	FalsificationEpochPaidPrefix     = []byte{0x9d}
	FalsificationCarryForwardPrefix  = []byte{0x9e}
	CounterFactChallengeKeyPrefix    = []byte{0x9f}
	CounterFactChallengeWindowPrefix = []byte{0xa0}
	ExtendedParamsKey                = []byte{0xa1}
	PatronageRecordPrefix            = []byte{0xa2}
	PruningQueuePrefix               = []byte{0xa3}
	VerifierConformityPrefix         = []byte{0xa4}
	ValidatorParticipationPrefix     = []byte{0xa5}
	TopicSaturationPrefix            = []byte{0xa6} // domain/topic → uint64 count
	AtRiskSampleIndexPrefix          = []byte{0xa7} // sampleID → exists (at-risk samples)

	// ─── Contest ────────────────────────────────────────────────────────
	ContestIndexPrefix = []byte{0xa8} // sampleID → contestRoundID (active contest)
)

// ─── New key constructors ───────────────────────────────────────────────────

// SampleKey returns the store key for a sample.
func SampleKey(id string) []byte {
	return append(append([]byte{}, SampleKeyPrefix...), []byte(id)...)
}

// SubmissionKey returns the store key for a submission.
func SubmissionKey(id string) []byte {
	return append(append([]byte{}, SubmissionKeyPrefix...), []byte(id)...)
}

// QualityRoundKey returns the store key for a quality round.
func QualityRoundKey(id string) []byte {
	return append(append([]byte{}, QualityRoundPrefix...), []byte(id)...)
}

// DomainKey returns the store key for a domain.
func DomainKey(name string) []byte {
	return append(append([]byte{}, DomainKeyPrefix...), []byte(name)...)
}

// DatasetKey returns the store key for a dataset.
func DatasetKey(id string) []byte {
	return append(append([]byte{}, DatasetKeyPrefix...), []byte(id)...)
}

// TrainingDemandKeyFn returns the store key for a training demand signal.
func TrainingDemandKeyFn(domain, subject string) []byte {
	key := append(append([]byte{}, TrainingDemandKey...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subject)...)
}

// DataBountyKey returns the store key for a data bounty.
func DataBountyKey(id string) []byte {
	return append(append([]byte{}, DataBountyKeyPrefix...), []byte(id)...)
}

// ScrapedSourceKeyFn returns the store key for a scraped source.
func ScrapedSourceKeyFn(id string) []byte {
	return append(append([]byte{}, ScrapedSourceKey...), []byte(id)...)
}

// ValidatorInfoKeyFn returns the store key for a validator info entry.
func ValidatorInfoKeyFn(addr string) []byte {
	return append(append([]byte{}, ValidatorInfoKey...), []byte(addr)...)
}

// ThreadIndexKey returns the index key for a sample within a thread.
func ThreadIndexKey(threadID, sampleID string) []byte {
	key := append(append([]byte{}, ThreadIndexPrefix...), []byte(threadID)...)
	key = append(key, '/')
	return append(key, []byte(sampleID)...)
}

// ThreadIndexByThreadPrefix returns the prefix for iterating all samples in a thread.
func ThreadIndexByThreadPrefix(threadID string) []byte {
	key := append(append([]byte{}, ThreadIndexPrefix...), []byte(threadID)...)
	return append(key, '/')
}

// DomainSampleIndexKey returns the index key for a sample within a domain.
func DomainSampleIndexKey(domain, sampleID string) []byte {
	key := append(append([]byte{}, DomainSampleIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(sampleID)...)
}

// DomainSampleByDomainPrefix returns the prefix for iterating samples in a domain.
func DomainSampleByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, DomainSampleIndexPrefix...), []byte(domain)...)
	return append(key, '/')
}

// SubmitterIndexKey returns the index key for a sample by submitter.
func SubmitterIndexKey(submitter, sampleID string) []byte {
	key := append(append([]byte{}, SubmitterIndexPrefix...), []byte(submitter)...)
	key = append(key, '/')
	return append(key, []byte(sampleID)...)
}

// SubmitterIndexBySubmitterPrefix returns the prefix for iterating samples by submitter.
func SubmitterIndexBySubmitterPrefix(submitter string) []byte {
	key := append(append([]byte{}, SubmitterIndexPrefix...), []byte(submitter)...)
	return append(key, '/')
}

// NicheIndexKey returns the index key for a sample within a niche.
func NicheIndexKey(nicheKey, sampleID string) []byte {
	key := append(append([]byte{}, NicheIndexPrefix...), []byte(nicheKey)...)
	key = append(key, '/')
	return append(key, []byte(sampleID)...)
}

// NicheIndexByNichePrefix returns the prefix for iterating samples in a niche.
func NicheIndexByNichePrefix(nicheKey string) []byte {
	key := append(append([]byte{}, NicheIndexPrefix...), []byte(nicheKey)...)
	return append(key, '/')
}

// ContentHashKey returns the index key for content hash dedup.
func ContentHashKey(hash string) []byte {
	return append(append([]byte{}, ContentHashIndexPrefix...), []byte(hash)...)
}

// SubmissionDomainIndexKey returns the index key for a submission within a domain.
func SubmissionDomainIndexKey(domain, submissionID string) []byte {
	key := append(append([]byte{}, SubmissionDomainIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(submissionID)...)
}

// SubmissionDomainByDomainPrefix returns the prefix for iterating submissions in a domain.
func SubmissionDomainByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, SubmissionDomainIndexPrefix...), []byte(domain)...)
	return append(key, '/')
}

// SubmissionSubmitterIndexKey returns the index key for a submission by submitter.
func SubmissionSubmitterIndexKey(submitter, submissionID string) []byte {
	key := append(append([]byte{}, SubmissionSubmitterIndexPrefix...), []byte(submitter)...)
	key = append(key, '/')
	return append(key, []byte(submissionID)...)
}

// SubmissionSubmitterBySubmitterPrefix returns the prefix for iterating submissions by submitter.
func SubmissionSubmitterBySubmitterPrefix(submitter string) []byte {
	key := append(append([]byte{}, SubmissionSubmitterIndexPrefix...), []byte(submitter)...)
	return append(key, '/')
}

// SubmissionRoundIndexKey returns the index key mapping a submission to its round.
func SubmissionRoundIndexKey(submissionID string) []byte {
	return append(append([]byte{}, SubmissionRoundIndexPrefix...), []byte(submissionID)...)
}

// NicheMembersKey returns the key for a niche's existence marker.
func NicheMembersKey(nicheKey string) []byte {
	return append(append([]byte{}, NicheMembersPrefix...), []byte(nicheKey)...)
}

// QueryReceiptKey returns the key for a query receipt.
func QueryReceiptKey(rater, sampleID string) []byte {
	key := append(append([]byte{}, QueryReceiptPrefix...), []byte(rater)...)
	key = append(key, '/')
	return append(key, []byte(sampleID)...)
}

// TopicSaturationKey returns the key for a topic's sample count.
func TopicSaturationKey(domain, topic string) []byte {
	key := append(append([]byte{}, TopicSaturationPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(topic)...)
}

// AtRiskSampleKey returns the index key for an at-risk sample.
func AtRiskSampleKey(sampleID string) []byte {
	return append(append([]byte{}, AtRiskSampleIndexPrefix...), []byte(sampleID)...)
}

// ContestIndexKey returns the index key mapping a contested sample to its re-validation round.
func ContestIndexKey(sampleID string) []byte {
	return append(append([]byte{}, ContestIndexPrefix...), []byte(sampleID)...)
}

// ─── Deprecated key constructors (keeper migration pending) ─────────────────

// FactKey returns the store key for a fact (deprecated: use SampleKey).
func FactKey(id string) []byte { return SampleKey(id) }

// ClaimKey returns the store key for a claim (deprecated: use SubmissionKey).
func ClaimKey(id string) []byte { return SubmissionKey(id) }

// RoundKey returns the store key for a round (deprecated: use QualityRoundKey).
func RoundKey(id string) []byte { return QualityRoundKey(id) }

// ClaimRoundIndexKey returns the claim→round index key (deprecated: use SubmissionRoundIndexKey).
func ClaimRoundIndexKey(claimID string) []byte { return SubmissionRoundIndexKey(claimID) }

// FactBySubmitterKey returns the fact-by-submitter index key (deprecated: use SubmitterIndexKey).
func FactBySubmitterKey(submitter, factID string) []byte { return SubmitterIndexKey(submitter, factID) }

// FactByDomainKey returns the fact-by-domain index key (deprecated: use DomainSampleIndexKey).
func FactByDomainKey(domain, factID string) []byte { return DomainSampleIndexKey(domain, factID) }

// FactRelationKey returns the forward index key for a fact relation (deprecated).
func FactRelationKey(sourceFactID, targetFactID string) []byte {
	key := append(append([]byte{}, FactRelationPrefix...), []byte(sourceFactID)...)
	key = append(key, '/')
	return append(key, []byte(targetFactID)...)
}

// FactRelationReverseKey returns the reverse index key for a fact relation (deprecated).
func FactRelationReverseKey(targetFactID, sourceFactID string) []byte {
	key := append(append([]byte{}, FactRelationReversePrefix...), []byte(targetFactID)...)
	key = append(key, '/')
	return append(key, []byte(sourceFactID)...)
}

// FactRelationsBySourcePrefix returns the prefix for relations from a source fact (deprecated).
func FactRelationsBySourcePrefix(sourceFactID string) []byte {
	key := append(append([]byte{}, FactRelationPrefix...), []byte(sourceFactID)...)
	return append(key, '/')
}

// FactRelationsByTargetPrefix returns the prefix for relations to a target fact (deprecated).
func FactRelationsByTargetPrefix(targetFactID string) []byte {
	key := append(append([]byte{}, FactRelationReversePrefix...), []byte(targetFactID)...)
	return append(key, '/')
}

// FactSubjectKey returns the subject index key (deprecated).
func FactSubjectKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, FactSubjectPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// FactTagKey returns the tag index key (deprecated).
func FactTagKey(tag, factID string) []byte {
	key := append(append([]byte{}, FactTagPrefix...), []byte(tag)...)
	key = append(key, '/')
	return append(key, []byte(factID)...)
}

// FactTagsByTagPrefix returns the prefix for iterating facts by tag (deprecated).
func FactTagsByTagPrefix(tag string) []byte {
	key := append(append([]byte{}, FactTagPrefix...), []byte(tag)...)
	return append(key, '/')
}

// CanonicalHashKey returns the canonical hash index key (deprecated).
func CanonicalHashKey(hash string) []byte {
	return append(append([]byte{}, CanonicalHashPrefix...), []byte(hash)...)
}

// CommonKnowledgeKey returns the common knowledge key (deprecated: use ScrapedSourceKeyFn).
func CommonKnowledgeKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, CommonKnowledgePrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// CommonKnowledgeByDomainPrefix returns the common knowledge domain prefix (deprecated).
func CommonKnowledgeByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, CommonKnowledgePrefix...), []byte(domain)...)
	return append(key, '/')
}

// DemandSignalKey returns the demand signal key (deprecated: use TrainingDemandKeyFn).
func DemandSignalKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, DemandSignalPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// BountyKey returns the bounty key (deprecated: use DataBountyKey).
func BountyKey(id string) []byte { return DataBountyKey(id) }

// BountyByDomainSubjectKey returns the bounty domain/subject index key (deprecated).
func BountyByDomainSubjectKey(domain, subjectHash string) []byte {
	key := append(append([]byte{}, BountyByDomainSubjectPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(subjectHash)...)
}

// LastClaimHeightKey returns the last claim height key (deprecated: use LastSubmissionHeightKey).
func LastClaimHeightKey(submitter string) []byte {
	return append(append([]byte{}, LastClaimHeightKeyPrefix...), []byte(submitter)...)
}

// ─── Shared key constructors (used by both old and new keeper) ──────────────

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

// VindicationPendingKey returns the store key for pending vindications.
func VindicationPendingKey(sampleId string) []byte {
	return append(append([]byte{}, VindicationPendingPrefix...), []byte(sampleId)...)
}

// VindicationRecordKey returns the store key for a vindication record.
func VindicationRecordKey(sampleId, verifier string) []byte {
	key := append([]byte{}, VindicationRecordPrefix...)
	key = append(key, []byte(sampleId)...)
	key = append(key, '/')
	key = append(key, []byte(verifier)...)
	return key
}

// VindicationRecordPrefixForSample returns the prefix for iterating records for a sample.
func VindicationRecordPrefixForSample(sampleId string) []byte {
	key := append([]byte{}, VindicationRecordPrefix...)
	key = append(key, []byte(sampleId)...)
	key = append(key, '/')
	return key
}

// VindicationRecordPrefixForFact is an alias for VindicationRecordPrefixForSample (deprecated).
func VindicationRecordPrefixForFact(factId string) []byte {
	return VindicationRecordPrefixForSample(factId)
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

// LastSubmissionHeightKey returns the store key for a submitter's last submission height.
func LastSubmissionHeightKey(submitter string) []byte {
	return append(append([]byte{}, LastSubmissionHeightKeyPrefix...), []byte(submitter)...)
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

// CompletedRoundBlockPrefix returns the prefix for iterating completed rounds at a block.
func CompletedRoundBlockPrefix(block uint64) []byte {
	key := make([]byte, 0, 1+8)
	key = append(key, CompletedRoundIndexPrefix...)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, block)
	key = append(key, buf...)
	return key
}
