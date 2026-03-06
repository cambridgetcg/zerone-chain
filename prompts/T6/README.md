# T6 — 殿 (Tono): The Sanctuary — TEE Training Enclave

**Goal:** Design and implement the Trusted Execution Environment bridge that enables secure model training — the only place the complete dataset is reassembled.

殿 (Tono) means "palace" or "sanctuary" — the sacred space where the dataset becomes a model.

## Context

From the ToK spec (section 6.4): "The complete dataset is ONLY reassembled inside a secure training enclave (TEE). Complete dataset NEVER exists outside this enclave."

The TEE is where:
1. Shards are collected from all validators (encrypted channels)
2. Complete dataset is reassembled
3. Fine-tuning runs
4. Model weights are output
5. Reassembled dataset is destroyed

This is the most security-critical component of the entire system.

## Sessions (3)

| # | File | Scope |
|---|------|-------|
| T6-1 | T6-1-tee-bridge.md | TEE provider abstraction: support Intel SGX, AMD SEV, and AWS Nitro Enclaves. Attestation verification on-chain. |
| T6-2 | T6-2-shard-collection.md | Secure shard collection protocol: authenticated requests to validators, encrypted transport, reassembly verification inside TEE. |
| T6-3 | T6-3-training-enclave.md | Training enclave runtime: dataset reassembly, fine-tune execution, model output, dataset destruction, attestation of process. |

## Run Order

Sequential: T6-1 → T6-2 → T6-3

## Prerequisites

- T2 (training pipeline — runs inside TEE)
- R40-3 (sharding lifecycle — knows which validator has which shard)
- T5-2 (storage nodes — validators serving shard data)

## Exit Criteria

1. TEE attestation can be verified on-chain (validator confirms enclave is genuine)
2. Shard collection retrieves all shards via encrypted channels
3. Dataset reassembly inside TEE produces correct complete dataset
4. Training runs to completion inside enclave
5. Only model weights exit the enclave — dataset is provably destroyed
6. Full audit trail: attestation → collection → training → output → destruction
