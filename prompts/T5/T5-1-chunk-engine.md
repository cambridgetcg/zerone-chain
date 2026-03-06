# T5-1 — Chunk Engine: Erasure Coding & Encryption

## Goal

Build the chunk engine that takes a finalized training dataset, applies erasure coding to split it into redundant chunks, encrypts each chunk, and produces the metadata needed for distribution and reconstruction.

## Deliverables

### 1. Erasure Coding

Use Reed-Solomon erasure coding:
- **K** data chunks + **M** parity chunks = **N** total chunks
- Any **K** of **N** chunks sufficient to reconstruct the full dataset
- Recommended starting ratio: K=64, M=32 (N=96). Need 64 of 96 chunks = ~67% threshold.
- Configurable per dataset (larger datasets may want higher K)

Implementation: Use `klauspost/reedsolomon` Go library (battle-tested, fast)

### 2. Dataset Serialization

Before chunking:
- Serialize the full training dataset (JSONL) into a binary blob
- Prepend header: dataset version, sample count, format, checksum (SHA-256)
- Pad to align with chunk size (Reed-Solomon requires equal-sized shards)

### 3. Encryption

Per-chunk encryption:
- Generate master dataset key (AES-256) per dataset version
- Derive per-chunk key via HKDF(master_key, chunk_index, dataset_version)
- Encrypt each chunk with AES-256-GCM (authenticated encryption)
- Store chunk metadata: {chunk_index, encrypted_size, nonce, tag, merkle_hash}

Master key management:
- Split master key using Shamir's Secret Sharing (threshold T of N shares)
- T = payment threshold (e.g., T=64 shares, one per data chunk needed)
- Shares distributed to key holders (can be storage nodes themselves, or separate key escrow nodes)

### 4. Chunk Manifest

Produce a manifest for each dataset version:
```json
{
  "dataset_id": "zerone-technical-v1.0.0",
  "total_chunks": 96,
  "data_chunks": 64,
  "parity_chunks": 32,
  "reconstruction_threshold": 64,
  "chunk_size_bytes": 10485760,
  "total_size_bytes": 671088640,
  "merkle_root": "0xabc...",
  "chunks": [
    {
      "index": 0,
      "merkle_hash": "0x123...",
      "encrypted_size": 10485792,
      "assigned_nodes": ["node1", "node2", "node3"]
    }
  ]
}
```

Manifest is stored on-chain (or IPFS with hash on-chain) for verifiability.

### 5. Integrity Verification

- Merkle tree over all chunk hashes → single merkle_root stored on-chain
- Buyers verify each received chunk against merkle proof
- Storage nodes prove possession via merkle proof of random byte ranges

### 6. CLI

```bash
chunk encode --dataset /data/training/v1.0.0/ --k 64 --m 32 --output /chunks/v1.0.0/
chunk verify --manifest /chunks/v1.0.0/manifest.json
chunk decode --chunks-dir /chunks/v1.0.0/ --output /data/reconstructed/
```

## Working Directory

`/Users/yournameisai/Desktop/zerone/services/blind-storage/`

## Output

- Go package for erasure coding + encryption
- CLI tool for encoding/decoding datasets
- Unit tests: encode → tamper → verify detects corruption
- Integration test: full round-trip (encode → distribute → collect K chunks → decode → verify match)
