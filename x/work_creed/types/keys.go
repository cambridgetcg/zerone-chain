package types

const (
	// ModuleName is the name of the work_creed module.
	ModuleName = "work_creed"

	// StoreKey is the default store key for work_creed.
	StoreKey = ModuleName

	// RouterKey is the router key for work_creed.
	RouterKey = ModuleName

	// QuerierRoute is the querier route key.
	QuerierRoute = ModuleName
)

// KV-store key prefixes. Phase 0 uses only the latest-pin index;
// historical-pin retrieval is added when amendments need it (Phase 1+).
var (
	// LatestSubCreedPinKey is the prefix for the latest pin per phase.
	// Format: LatestSubCreedPinKey || phase_uint32_be (4 bytes) → PinnedSubCreed bytes.
	LatestSubCreedPinKey = []byte{0x01}
)
