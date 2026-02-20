package types

const (
	ModuleName  = "zerone_ontology"
	StoreKey    = ModuleName
	RouterKey   = ModuleName
	MemStoreKey = "mem_" + ModuleName
	QuerierRoute = ModuleName
)

var (
	ParamsKey                  = []byte{0x00}
	StratumKeyPrefix           = []byte{0x01}
	DomainKeyPrefix            = []byte{0x02}
	ProposalKeyPrefix          = []byte{0x03}
	LinkKeyPrefix              = []byte{0x04}
	DomainByStratumPrefix      = []byte{0x05}
	LogicZoneKeyPrefix         = []byte{0x10}
	IncompletenessAckKeyPrefix = []byte{0x11}
)
