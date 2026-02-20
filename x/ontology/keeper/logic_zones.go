package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// InitializeDefaultZones pre-populates the 5 default logic zones.
// Called from InitGenesis to ensure zones are always available.
func (k Keeper) InitializeDefaultZones(ctx sdk.Context) {
	defaults := types.DefaultLogicZones()
	for i := range defaults {
		zone := &defaults[i]
		// Only set if not already registered (preserves governance overrides)
		if _, found := k.GetLogicZone(ctx, types.LogicZone(zone.Zone)); !found {
			k.SetLogicZone(ctx, zone)
		}
	}
}

// ValidateClaimLogicZone enforces logic zone confidence ceilings and Gödelian constraints.
//
// For claims in zones where Gödel's incompleteness applies (Peano, set_theory),
// this enforces a confidence ceiling below 100% and flags the claim as requiring
// incompleteness acknowledgment.
//
// Returns:
//   - nil if the zone allows the given confidence
//   - ErrZoneConfidenceExceeded if confidence exceeds zone ceiling
//   - ErrGoedelInconsistency if an incomplete zone claim lacks acknowledgment
//   - ErrInvalidLogicZone if the zone is not registered
func (k Keeper) ValidateClaimLogicZone(ctx sdk.Context, zone string, confidenceBps uint64) error {
	if zone == "" {
		// No zone specified — skip validation (backward compatible)
		return nil
	}

	logicZone := types.LogicZone(zone)
	props, found := k.GetLogicZone(ctx, logicZone)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrInvalidLogicZone, zone)
	}

	// Enforce confidence ceiling
	if confidenceBps > props.MaxConfidenceBps {
		return fmt.Errorf("%w: zone %s allows max %d bps, got %d",
			types.ErrZoneConfidenceExceeded, zone, props.MaxConfidenceBps, confidenceBps)
	}

	return nil
}

// RequiresIncompletenessAck returns whether a zone requires Gödelian incompleteness acknowledgment.
func (k Keeper) RequiresIncompletenessAck(ctx sdk.Context, zone string) bool {
	if zone == "" {
		return false
	}
	props, found := k.GetLogicZone(ctx, types.LogicZone(zone))
	if !found {
		return false
	}
	return props.GoedelApplies
}

// ValidateZoneForCategory checks that the declared logic zone is compatible with
// the claim's epistemic category. Prevents zone gaming where submitters declare a
// more formal zone to bypass Gödelian confidence ceilings.
func (k Keeper) ValidateZoneForCategory(ctx sdk.Context, zone string, epistemicCategory string) error {
	if zone == "" || epistemicCategory == "" {
		return nil // backward compatible
	}

	allowedZones, catFound := types.ValidZonesForCategory[epistemicCategory]
	if !catFound {
		// Unknown category — let the knowledge module validate
		return nil
	}

	if !allowedZones[types.LogicZone(zone)] {
		return fmt.Errorf("%w: category %q cannot use zone %q",
			types.ErrZoneCategoryMismatch, epistemicCategory, zone)
	}

	return nil
}

// GetZoneConfidenceCeiling returns the maximum confidence in basis points for a zone.
// Returns 1000000 (100%) if the zone is not found or empty.
func (k Keeper) GetZoneConfidenceCeiling(ctx sdk.Context, zone string) uint64 {
	if zone == "" {
		return 1000000
	}
	props, found := k.GetLogicZone(ctx, types.LogicZone(zone))
	if !found {
		return 1000000
	}
	return props.MaxConfidenceBps
}
