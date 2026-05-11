package types

const (
	ModuleName   = "contribution"
	StoreKey     = ModuleName
	RouterKey    = ModuleName
	QuerierRoute = ModuleName
)

// KV-store key prefixes. All multi-byte keys are big-endian for
// sort-friendly iteration.
var (
	// Primary record: 0x01 || contribution_id (32 bytes) → Contribution
	ContributionKey = []byte{0x01}

	// Secondary indexes — values are presence-only (empty bytes);
	// callers look up the primary record by ID.
	// 0x02 || contributor_addr_len (uvarint) || contributor_addr || contribution_id
	ByContributorKey = []byte{0x02}
	// 0x03 || class_uint32_be (4 bytes) || contribution_id
	ByClassKey = []byte{0x03}
	// 0x04 || phase_uint32_be (4 bytes) || contribution_id
	ByPhaseKey = []byte{0x04}
	// 0x05 || status_uint32_be (4 bytes) || contribution_id
	ByStatusKey = []byte{0x05}

	// Reverse-lookup index for hooks: back_ref (e.g., x/knowledge claim_id)
	// → contribution_id. Used by KnowledgeHooksAdapter to find the
	// mirror Contribution when a claim transitions.
	// 0x06 || back_ref_len (uvarint) || back_ref → contribution_id (32 bytes)
	ByBackRefKey = []byte{0x06}
)
