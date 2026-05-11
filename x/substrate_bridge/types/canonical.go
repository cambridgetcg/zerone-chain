package types

import (
	"crypto/sha256"
	"encoding/hex"
)

// PendingClaimCanonicalHash returns the canonical sha256 of a pending
// claim used for dedup against existing x/knowledge claims (spec §2
// idempotency). Two pending claims with identical
// (domain, methodology_id, claim_content) produce the same hash.
func PendingClaimCanonicalHash(p *PendingClaim) string {
	h := sha256.New()
	h.Write([]byte(p.Domain))
	h.Write([]byte{0x00})
	h.Write([]byte(p.MethodologyId))
	h.Write([]byte{0x00})
	h.Write([]byte(p.ClaimContent))
	return hex.EncodeToString(h.Sum(nil))
}
