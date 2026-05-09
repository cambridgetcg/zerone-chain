package types

const (
	ModuleName = "creed"
	StoreKey   = ModuleName
)

var (
	// ParamsKey holds the module Params (single record).
	ParamsKey = []byte{0x00}

	// PinPrefix indexes pinned creed records by version
	// (uint32 BE). Append-only; older versions remain queryable as
	// the chain's history of self-amendment.
	//
	// docs/TRUTH_SEEKING.md commitment 10 (forward-only audit):
	// keying by monotonic version ensures the pin history cannot
	// be reordered or rewritten — at most extended.
	PinPrefix = []byte{0x01}

	// CurrentVersionKey stores the highest pinned version (uint32
	// BE). Read by Pinned() to find the canonical record without
	// iterating the history.
	CurrentVersionKey = []byte{0x02}

	// CommitmentIndexPrefix is a per-commitment-number index into
	// the latest non-archived entry. Allows O(1) lookup of "what
	// does the chain currently say about commitment N." Updated
	// on every AnchorPin transaction; entries point to (version,
	// position-in-pin) so the full record can be fetched.
	//
	// Key: PinPrefix || version_be (so entries are co-located in
	// the store with their parent pin).
	CommitmentIndexPrefix = []byte{0x03}
)
