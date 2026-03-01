package types

const (
	ModuleName = "qualification"
	StoreKey   = ModuleName
)

// KV store key prefixes.
var (
	ParamsKey               = []byte{0x00}
	QualificationKeyPrefix  = []byte{0x01}
	EndorsementKeyPrefix    = []byte{0x02}
	DomainValidatorPrefix   = []byte{0x10} // domain → validator index
	EndorserIndexPrefix     = []byte{0x11} // endorser index
	TargetIndexPrefix       = []byte{0x12} // target (validator+domain) → endorsement index
	EndorsementCounterKey        = []byte{0x20}
	QualificationPenaltyKeyPrefix = []byte{0x30} // validator/domain → QualificationPenalty (JSON)
)
