package keeper

const (
	// Event types.
	EventTypeExternalAttestationSubmitted = "external_attestation_submitted"
	EventTypeExternalAttestationCommitted = "external_attestation_committed"
	EventTypeExternalAttestationSettled   = "external_attestation_settled"
	EventTypeExternalAttestationRejected  = "external_attestation_rejected"
	EventTypeExternalAttestationPartial   = "external_attestation_partial"

	EventTypeAdapterRegistered = "adapter_registered"
	EventTypeAdapterSuspended  = "adapter_suspended"
	EventTypeAdapterTombstoned = "adapter_tombstoned"

	EventTypeLineageEdgeCreated = "lineage_edge_created"
	EventTypeLineageRoyaltyPaid = "lineage_royalty_paid"

	// Attributes.
	AttrUsefulWorkCommitment = "useful_work_commitment" // value: "UW"
	AttrMechanism            = "mechanism"              // value: "M1" | "M2,M3" | etc.
)
