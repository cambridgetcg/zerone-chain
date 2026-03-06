# T6-1 — TEE Provider Bridge

## Objective

Build a provider-agnostic TEE abstraction that supports multiple trusted execution environments and enables on-chain attestation verification.

## Design

### Provider Abstraction

```go
// TEE provider interface
type TEEProvider interface {
    // Generate attestation document proving enclave identity
    Attest(ctx context.Context) (*Attestation, error)
    
    // Verify a remote attestation document
    Verify(attestation *Attestation) (*AttestationResult, error)
    
    // Get enclave measurements (code hash, configuration)
    GetMeasurements() (*Measurements, error)
    
    // Seal data to enclave (encrypt with enclave key)
    Seal(data []byte) ([]byte, error)
    
    // Unseal data (decrypt with enclave key)
    Unseal(sealed []byte) ([]byte, error)
}
```

### Supported Providers

1. **AWS Nitro Enclaves** (primary — most accessible)
   - PCRS (Platform Configuration Registers) for code identity
   - Nitro Security Module for attestation documents
   - KMS integration for key management

2. **Intel SGX** (future)
   - DCAP attestation
   - Enclave quote verification

3. **AMD SEV-SNP** (future)
   - Versioned Chip Endorsement Key
   - Attestation report verification

4. **Mock/Development** (for testing)
   - Simulates attestation flow without hardware
   - Allows local development and CI testing

### On-Chain Attestation

Add to x/knowledge:

```go
// MsgRegisterEnclave — TEE operator registers their enclave
type MsgRegisterEnclave struct {
    Operator    string
    Attestation []byte  // serialized attestation document
    Provider    string  // "nitro" | "sgx" | "sev"
    Measurements []byte // expected code hash
}

// MsgVerifyAttestation — anyone can verify an enclave is genuine
// Keeper verifies attestation against known measurements
```

Store registered enclaves in state:
```go
type RegisteredEnclave struct {
    Operator     sdk.AccAddress
    Provider     string
    Measurements []byte
    AttestationHash []byte
    RegisteredAt int64
    LastVerified int64
    Status       EnclaveStatus // Active, Suspended, Revoked
}
```

### Enclave Package

```
pkg/tee/
├── provider.go      — TEEProvider interface
├── nitro/
│   ├── provider.go  — AWS Nitro implementation
│   ├── attestation.go
│   └── nitro_test.go
├── mock/
│   ├── provider.go  — Mock implementation for testing
│   └── mock_test.go
├── verify.go        — Cross-provider attestation verification
└── types.go         — Attestation, Measurements, AttestationResult
```

## Tests

- Test: mock provider generates and verifies attestation
- Test: attestation with wrong measurements fails verification
- Test: MsgRegisterEnclave stores enclave in state
- Test: expired attestation rejected
- Test: enclave status transitions (Active → Suspended → Revoked)

## Constraints

- Start with Nitro Enclaves (AWS) as primary — most documentation, easiest to deploy
- Mock provider MUST be feature-flag gated — never available in production builds
- Attestation documents expire (configurable, default 24h)
- On-chain storage: only attestation hash + measurements, not full document
