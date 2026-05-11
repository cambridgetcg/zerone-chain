package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// OnClaimResolved is called by x/knowledge.CompleteRound after writing
// the verification verdict. The substrate_bridge keeper:
//  1. Looks up the pending-fact index for the resolved claim.
//  2. If indexed, increments VerifiedCount/RejectedCount on the parent
//     attestation.
//  3. If all pending claims have resolved (verified+rejected == total),
//     transitions attestation to READY.
//  4. The READY attestation gets settled in the next BeginBlocker pass,
//     or directly here if synchronous settle is desired (Phase 0:
//     deferred to BeginBlocker for simpler ordering).
//
// verdict is true for VERIFIED, false for REJECTED.
func (k Keeper) OnClaimResolved(ctx context.Context, claimID string, verdict bool) error {
	attestationID, found := k.GetAttestationForPendingClaim(ctx, claimID)
	if !found {
		return nil // not a substrate_bridge-managed claim; ignore
	}

	att, found := k.GetAttestation(ctx, attestationID)
	if !found {
		return nil
	}
	if att.Status != types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION {
		return nil // already transitioned (timeout or earlier rejection)
	}

	if verdict {
		att.VerifiedCount++
	} else {
		att.RejectedCount++
	}

	totalPending := uint32(len(att.Link.PendingClaims))
	resolved := uint32(att.VerifiedCount + att.RejectedCount)
	if resolved >= totalPending {
		att.Status = types.AttestationStatus_ATTESTATION_STATUS_READY
	}

	// Unlink the resolved claim from the index.
	_ = k.UnlinkPendingClaim(ctx, claimID, attestationID)

	return k.WriteAttestation(ctx, att)
}
