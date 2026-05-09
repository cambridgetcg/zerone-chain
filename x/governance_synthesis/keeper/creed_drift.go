package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	creedtypes "github.com/zerone-chain/zerone/x/creed/types"
	"github.com/zerone-chain/zerone/x/governance_synthesis/types"
)

// ComposeCreedDrift builds the chain's drift signal from x/creed.
// The signal is composed live; the synthesizer holds no state of
// its own. If x/creed is not wired (unit tests, dev chains in a
// pre-anchor state), the response carries zero-state values rather
// than panicking — drift_bps=0 truthfully reports "no observed
// drift" when there is no creed to drift from.
//
// docs/TRUTH_SEEKING.md commitments 11 and 19: the synthesised
// composite makes "the chain's voice has moved by N basis points
// since genesis" a single-query fact rather than a stitching
// exercise across raw pin records.
func (k Keeper) ComposeCreedDrift(ctx context.Context) types.CreedDrift {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	out := types.CreedDrift{
		ComputedAtBlock: uint64(sdkCtx.BlockHeight()),
	}

	if k.creedKeeper == nil {
		return out
	}

	// Council surface. Independent of pin state — councils may be
	// seated even before the first pin lands.
	out.CouncilTotalWeightBps = k.creedKeeper.CouncilTotalActiveWeight(ctx)
	k.creedKeeper.IterateCouncilMembers(ctx, func(m *creedtypes.CreedCouncilMember) bool {
		if m.Active {
			out.CouncilActiveCount++
		}
		return false
	})

	cur, ok := k.creedKeeper.GetCurrentPin(ctx)
	if !ok || cur == nil {
		// Pre-anchor state — no creed to drift from. Council
		// fields above remain populated.
		return out
	}

	out.CurrentVersion = cur.Version
	if cur.Version > 1 {
		out.VersionsSinceGenesis = cur.Version - 1
		out.LastAmendmentHeight = cur.PinnedAtHeight
		out.LastAmendmentLip = cur.PinnedViaLip
	}

	// Current registry shape. Walk the registry once.
	out.CurrentCommitmentCount = uint32(len(cur.Commitments))
	for _, c := range cur.Commitments {
		if c == nil {
			continue
		}
		if c.Archived {
			out.CommitmentsArchived++
		} else {
			out.CurrentActiveCount++
		}
	}

	// Genesis registry. Pin v1 is the genesis pin; a missing v1
	// in chains that started after x/creed shipped is a structural
	// anomaly worth surfacing as zero-baseline drift.
	if genesis, ok := k.creedKeeper.GetPin(ctx, 1); ok && genesis != nil {
		out.GenesisCommitmentCount = uint32(len(genesis.Commitments))
		if out.CurrentCommitmentCount > out.GenesisCommitmentCount {
			out.CommitmentsAdded = out.CurrentCommitmentCount - out.GenesisCommitmentCount
		}
	} else {
		// No genesis pin — treat current as the genesis baseline so
		// drift starts at zero. This is the honest read for chains
		// that started without a Genesis Creed seeded.
		out.GenesisCommitmentCount = out.CurrentCommitmentCount
	}

	out.DriftBps = computeDriftBps(out.VersionsSinceGenesis, out.CommitmentsAdded, out.CommitmentsArchived)
	return out
}

// computeDriftBps is the heuristic composite. Bounded at the BPS
// scale (1_000_000). Consumers preferring a different weighting
// should compose their own from the raw fields.
//
// Weighting rationale:
//   - Each amendment version contributes 100_000 bps (10 amendments → max).
//   - Each added commitment contributes 50_000 bps (20 added → max).
//   - Each archived commitment contributes 100_000 bps (10 archived → max,
//     same weight as a version since archival is structurally
//     equivalent to a version transition that removes a load-bearing
//     piece).
//
// The capping at 1_000_000 means the composite saturates at "the
// chain's voice has substantially moved." Any drift beyond that
// reads as the same color of signal; consumers wanting finer
// granularity past saturation should use the raw fields.
func computeDriftBps(versions, added, archived uint32) uint64 {
	const cap = uint64(1_000_000)
	var raw uint64
	raw += uint64(versions) * 100_000
	raw += uint64(added) * 50_000
	raw += uint64(archived) * 100_000
	if raw > cap {
		return cap
	}
	return raw
}
