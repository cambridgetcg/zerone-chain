package types

const (
	ModuleName = "capture_defense"
	StoreKey   = ModuleName
)

var (
	ParamsKey                       = []byte{0x00}
	GlobalReputationKeyPrefix       = []byte{0x01}
	StratumReputationKeyPrefix      = []byte{0x02}
	DomainReputationKeyPrefix       = []byte{0x03}
	CaptureMetricsKeyPrefix         = []byte{0x04}
	VerificationHistoryKeyPrefix    = []byte{0x05}
	CrossStratumKeyPrefix           = []byte{0x06}
	StructuralImmunityParamsKey     = []byte{0x07} // R29-5: structural immunity params
)
