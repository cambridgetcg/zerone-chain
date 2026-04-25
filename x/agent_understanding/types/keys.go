package types

const (
	ModuleName = "agent_understanding"
	StoreKey   = ModuleName
)

// The synthesizer holds no persistent state beyond Params.
var (
	ParamsKey = []byte{0x00}
)
