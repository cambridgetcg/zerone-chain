package types

// TEE provider constants matching proto EnclaveStatus enum values.
const (
	TEEProviderNitro = "nitro"
	TEEProviderSGX   = "sgx"
	TEEProviderSEV   = "sev"

	EnclaveStatusActiveStr    = "active"
	EnclaveStatusSuspendedStr = "suspended"
	EnclaveStatusRevokedStr   = "revoked"
)

// ValidTEEProviders is the set of recognized TEE provider types.
var ValidTEEProviders = map[string]bool{
	TEEProviderNitro: true,
	TEEProviderSGX:   true,
	TEEProviderSEV:   true,
}

// EnclaveRecord is the on-chain representation of a registered TEE enclave.
// Stored as JSON under EnclaveKeyPrefix + operator.
type EnclaveRecord struct {
	Operator        string `json:"operator"`
	Provider        string `json:"provider"`
	Measurements    []byte `json:"measurements"`
	AttestationHash []byte `json:"attestation_hash"`
	RegisteredAt    int64  `json:"registered_at"`
	LastVerified    int64  `json:"last_verified"`
	Status          string `json:"status"`
}

// TEE event types.
const (
	EventEnclaveRegistered = "enclave_registered"
	EventEnclaveVerified   = "enclave_verified"
	EventEnclaveSuspended  = "enclave_suspended"
	EventEnclaveRevoked    = "enclave_revoked"

	AttributeEnclaveOperator = "enclave_operator"
	AttributeEnclaveProvider = "enclave_provider"
	AttributeEnclaveStatus   = "enclave_status"
)
