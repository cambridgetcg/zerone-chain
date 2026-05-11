package types

// CitationTypeWeight returns the multiplier for a citation type per
// the doctrine: CITES=1, SUPPORTS=2, EXTENDS=3, REFINES=3, GENERALIZES=4.
// Unspecified or unknown returns 0 (no weight, no propagation).
func CitationTypeWeight(t CitationType) uint32 {
	switch t {
	case CitationType_CITATION_TYPE_CITES:
		return 1
	case CitationType_CITATION_TYPE_SUPPORTS:
		return 2
	case CitationType_CITATION_TYPE_EXTENDS,
		CitationType_CITATION_TYPE_REFINES:
		return 3
	case CitationType_CITATION_TYPE_GENERALIZES:
		return 4
	default:
		return 0
	}
}
