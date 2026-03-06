# T6-2 — Secure Shard Collection

## Objective

Build the protocol for the TEE to securely collect shards from validators, authenticate both sides, and reassemble the complete dataset inside the enclave.

## Design

### Collection Flow

```
TEE Enclave                          Validator Node
    |                                      |
    |-- 1. Present attestation ----------->|
    |                                      |-- verify attestation
    |<- 2. Challenge (nonce) --------------|
    |                                      |
    |-- 3. Sign challenge + request ------>|
    |     (shard_ids, snapshot_height)     |-- verify on-chain:
    |                                      |   - enclave registered?
    |                                      |   - shard assignments valid?
    |                                      |
    |<- 4. Encrypted shard data -----------|
    |     (AES-256-GCM, per-session key)   |
    |                                      |
    |-- 5. Acknowledge receipt ----------->|
```

### Authentication

- **TEE → Validator:** TEE presents attestation document proving it's a genuine enclave with correct measurements
- **Validator → TEE:** Validator verifies attestation, checks on-chain that this enclave is registered and active
- **Session key:** ECDH key exchange between TEE enclave key and validator's key, derive AES-256-GCM session key
- **Replay protection:** Nonce + snapshot height + timestamp

### Shard Request

```go
type ShardRequest struct {
    EnclaveID     string
    Attestation   []byte
    SnapshotHeight int64
    RequestedTDUs []string  // TDU IDs assigned to this validator
    Nonce         []byte
    Signature     []byte    // signed by enclave key
}

type ShardResponse struct {
    ValidatorAddr string
    SnapshotHeight int64
    TDUs          []TDUData  // actual training data
    DataHash      []byte     // SHA-256 of all TDU data
    Nonce         []byte
    Signature     []byte     // signed by validator key
}
```

### Reassembly Verification

Inside the TEE after collecting from all validators:
1. Verify each shard response signature
2. Verify TDU content hashes match on-chain content hashes
3. Verify replication: each TDU received from R validators matches
4. If any TDU has mismatched copies: flag for investigation, exclude from training
5. Log complete reassembly attestation

### Error Handling

- **Validator offline:** Retry 3 times with exponential backoff. If still offline, proceed without (if replication allows)
- **Hash mismatch:** Exclude TDU, report discrepancy on-chain
- **Insufficient shards:** If <80% of TDUs collected, abort training cycle

### Package

```
services/shard-collector/
├── main.go
├── collector.go     — Orchestrates collection from all validators
├── authenticator.go — Attestation exchange and session key derivation
├── verifier.go      — Shard integrity verification
├── assembler.go     — Dataset reassembly from shards
└── collector_test.go
```

## Tests

- Test: attestation exchange completes successfully (mock)
- Test: session key derivation produces matching keys on both sides
- Test: shard data encrypted and decrypted correctly
- Test: content hash mismatch detected and reported
- Test: missing validator handled gracefully (proceed with remaining)
- Test: insufficient shards aborts collection

## Constraints

- All communication over mTLS
- Session keys rotated per collection cycle
- TEE never stores decrypted shards to disk — memory only
- Collection timeout: 5 minutes per validator (configurable)
