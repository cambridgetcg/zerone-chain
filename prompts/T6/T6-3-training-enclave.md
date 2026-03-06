# T6-3 — Training Enclave Runtime

## Objective

Build the enclave runtime that manages the complete training lifecycle inside the TEE: dataset reassembly → fine-tuning → model output → dataset destruction → attestation.

## Design

### Lifecycle

```
┌─────────────────────────────────────────────────────────────┐
│                    TEE Training Enclave                      │
│                                                             │
│  Phase 1: COLLECT                                           │
│  ├── Authenticate with validators                           │
│  ├── Collect encrypted shards                               │
│  ├── Decrypt in enclave memory                              │
│  └── Verify integrity (content hashes)                      │
│                                                             │
│  Phase 2: PREPARE                                           │
│  ├── Reassemble complete dataset                            │
│  ├── Apply fitness filter (exclude Dormant/Pruned)          │
│  ├── Apply domain mix (70% domain / 30% general)            │
│  ├── Format for training (HuggingFace format)               │
│  └── Compute dataset fingerprint                            │
│                                                             │
│  Phase 3: TRAIN                                             │
│  ├── Load base model                                        │
│  ├── Run LoRA/QLoRA fine-tuning                             │
│  ├── Track per-TDU training loss (for influence analysis)   │
│  ├── Run benchmark suite on trained model                   │
│  └── Compute model quality metrics                          │
│                                                             │
│  Phase 4: OUTPUT                                            │
│  ├── Export model weights (LoRA adapter only)               │
│  ├── Export training metrics + per-TDU loss values          │
│  ├── Export benchmark results                               │
│  ├── Sign all outputs with enclave key                      │
│  └── Generate training attestation                          │
│                                                             │
│  Phase 5: DESTROY                                           │
│  ├── Zero all dataset memory                                │
│  ├── Zero all intermediate training state                   │
│  ├── Verify destruction (memory scan)                       │
│  └── Attest destruction (signed certificate)                │
│                                                             │
│  OUTPUTS (only things that leave the enclave):              │
│  ├── LoRA adapter weights (signed)                          │
│  ├── Training metrics + TDU losses (signed)                 │
│  ├── Benchmark results (signed)                             │
│  └── Training attestation (proves correct process)          │
│                                                             │
│  NEVER LEAVES: raw dataset, intermediate activations,       │
│  gradients, optimizer state                                 │
└─────────────────────────────────────────────────────────────┘
```

### Training Attestation

```go
type TrainingAttestation struct {
    EnclaveID        string
    DatasetFingerprint []byte  // hash of complete assembled dataset
    DatasetSize      int64     // number of TDUs used
    BaseModel        string    // e.g., "llama-3-8b"
    TrainingConfig   string    // hyperparameters hash
    ModelHash        []byte    // hash of output LoRA weights
    BenchmarkScore   float64   // overall benchmark score
    StartTime        time.Time
    EndTime          time.Time
    DestructionProof []byte    // signed proof that dataset was zeroed
    Signature        []byte    // signed by enclave key
}
```

### On-Chain Recording

After training completes:
```
zeroned tx knowledge record-training \
  --attestation-file ./training-attestation.json \
  --model-hash <hash> \
  --benchmark-score 0.73 \
  --from enclave-operator
```

Add `MsgRecordTraining` to msg_server — stores attestation hash, makes model version available for API serving.

### Enclave Runtime Package

```
services/training-enclave/
├── main.go
├── runtime.go        — Lifecycle state machine (collect→prepare→train→output→destroy)
├── preparator.go     — Dataset assembly and formatting
├── trainer.go        — Fine-tuning execution (calls T2 pipeline)
├── outputter.go      — Model export and signing
├── destroyer.go      — Secure memory zeroing and verification
├── attestation.go    — Generate and sign training attestation
└── enclave_test.go
```

## Tests

- Test: lifecycle state machine transitions correctly
- Test: fitness filter excludes Dormant/Pruned TDUs
- Test: domain mix ratio applied correctly (70/30)
- Test: all outputs signed with enclave key
- Test: destruction zeros all dataset memory
- Test: attestation contains correct dataset fingerprint + model hash
- Test: incomplete collection → reduced dataset (not failure)

## Constraints

- Dataset NEVER touches disk inside enclave — memory only
- If enclave crashes mid-training: dataset is automatically destroyed (memory freed)
- Model weights are the ONLY training artifact that exits
- Training attestation is publicly verifiable against on-chain enclave registration
- First implementation: single enclave. Distributed training = future work.
