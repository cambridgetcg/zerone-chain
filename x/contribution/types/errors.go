package types

import sdkerrors "cosmossdk.io/errors"

var (
	ErrAdapterNotRegistered    = sdkerrors.Register(ModuleName, 2, "no adapter registered for class (Phase 1 only KNOWLEDGE_CLAIM is wired) (UW + M3)")
	ErrUnknownClass            = sdkerrors.Register(ModuleName, 3, "ContributionClass out of range [1, 11] (UW + M3)")
	ErrUnknownPhase            = sdkerrors.Register(ModuleName, 4, "LifecyclePhase out of range [0, 8] (UW + M3)")
	ErrTruthFloorStale         = sdkerrors.Register(ModuleName, 5, "truth_floor_attestation.creed_version stale (UW + truth-floor invariant)")
	ErrTruthFloorMissing       = sdkerrors.Register(ModuleName, 6, "truth_floor_attestation absent (UW + truth-floor invariant)")
	ErrSubstrateLinkAbsent     = sdkerrors.Register(ModuleName, 7, "substrate_link_bps == 0; reward path blocked (UW + M2)")
	ErrClaimsAboutSelfEmpty    = sdkerrors.Register(ModuleName, 8, "claims_about_self required (UW + truth-seeking commitment 1)")
	ErrInvalidLineage          = sdkerrors.Register(ModuleName, 9, "lineage refs must resolve to existing contributions (UW + M6 cross-class via TC6)")
	ErrInvalidStatusTransition = sdkerrors.Register(ModuleName, 10, "status transition violates forward-only audit (truth-seeking commitment 10)")
	ErrInvalidClassPhase       = sdkerrors.Register(ModuleName, 11, "class+phase combination not allowed by default mapping (UW + M3)")
	ErrPayloadMissing          = sdkerrors.Register(ModuleName, 12, "payload absent or wrong oneof variant for declared class (UW + M3)")
	ErrBackRefNotFound         = sdkerrors.Register(ModuleName, 13, "back_ref does not resolve in source module (UW + M3)")
)
