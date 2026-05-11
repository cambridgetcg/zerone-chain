package types

import (
	"fmt"
)

func DefaultParams() Params {
	return Params{
		MaxPendingClaimsPerAttestation:    100_000,
		PerPendingClaimBondUzrn:           "222",
		AttestationMinBondUzrn:            "222000",
		MaxPendingWindowBlocks:            6_220_800, // ~6 months at 2.5s blocks
		PendingClaimRejectionThresholdBps: 5000,
		MinVerifiedRatioForSettleBps:      1000,
		LineageShareBps:                   3000,
		DecayBpsPerHop:                    3000,
		MaxPropagationDepth:               5,
		MinPropagationUzrn:                "1000",
		SelfCitationCapBps:                5000,
	}
}

func (p Params) Validate() error {
	if p.MaxPendingClaimsPerAttestation == 0 {
		return fmt.Errorf("max_pending_claims_per_attestation must be > 0")
	}
	if p.PendingClaimRejectionThresholdBps == 0 || p.PendingClaimRejectionThresholdBps > 10000 {
		return fmt.Errorf("pending_claim_rejection_threshold_bps must be in (0, 10000]")
	}
	if p.MinVerifiedRatioForSettleBps > 10000 {
		return fmt.Errorf("min_verified_ratio_for_settle_bps must be in [0, 10000]")
	}
	if p.LineageShareBps > 10000 {
		return fmt.Errorf("lineage_share_bps must be in [0, 10000]")
	}
	if p.DecayBpsPerHop > 10000 {
		return fmt.Errorf("decay_bps_per_hop must be in [0, 10000]")
	}
	if p.MaxPropagationDepth == 0 || p.MaxPropagationDepth > 20 {
		return fmt.Errorf("max_propagation_depth must be in [1, 20]")
	}
	if p.SelfCitationCapBps > 10000 {
		return fmt.Errorf("self_citation_cap_bps must be in [0, 10000]")
	}
	return nil
}
