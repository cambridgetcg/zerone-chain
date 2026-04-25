package types

const (
	ModuleName = "counterexamples"
	StoreKey   = ModuleName
)

// KV store key prefixes.
var (
	ParamsKey                  = []byte{0x00}
	CounterexampleKeyPrefix    = []byte{0x01} // id → Counterexample
	ValidationKeyPrefix        = []byte{0x02} // id (uint64 BE) → Validation
	NextCounterexampleSeqKey   = []byte{0x03}
	NextValidationIDKey        = []byte{0x04}

	// Indexes.
	ByFactPrefix               = []byte{0x10} // fact_id/counterexample_id → 1
	ValidationsByCEPrefix      = []byte{0x11} // counterexample_id/validation_id → 1
	ValidatorVotedPrefix       = []byte{0x12} // counterexample_id/validator → 1 (one vote per validator per CE)
)
