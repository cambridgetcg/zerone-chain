package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// BeginBlocker scans AWAITING_RESOLUTION attestations and:
//   - times out any older than Params.MaxPendingWindowBlocks (settle-as-PARTIAL or REJECTED)
//   - drains READY attestations into SETTLED (called from settlement engine,
//     but BeginBlocker also pulls them in case OnClaimResolved didn't fire
//     for the last pending claim).
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := uint64(sdkCtx.BlockHeight())
	params := k.GetParams(ctx)

	// Timeout scan.
	var timedOut []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, func(id string) bool {
		att, found := k.GetAttestation(ctx, id)
		if !found {
			return false
		}
		if currentHeight-att.SubmittedAtBlock >= params.MaxPendingWindowBlocks {
			timedOut = append(timedOut, id)
		}
		return false
	})
	for _, id := range timedOut {
		if err := k.SettleAttestation(ctx, id); err != nil {
			k.Logger(sdkCtx).Error("timeout-settle failed", "attestation_id", id, "err", err)
		}
	}

	// READY drain.
	var ready []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_READY, func(id string) bool {
		ready = append(ready, id)
		return false
	})
	for _, id := range ready {
		if err := k.SettleAttestation(ctx, id); err != nil {
			k.Logger(sdkCtx).Error("ready-settle failed", "attestation_id", id, "err", err)
		}
	}

	return nil
}
