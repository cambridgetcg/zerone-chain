# T1-2 — Blind Storage Protocol

## Goal

Design the blind storage protocol that distributes encrypted training data chunks across storage nodes so that:
- No single node holds enough data to reconstruct a useful dataset
- Only agents/users who pay sufficient ZRN can access enough chunks to train a model
- Storage nodes earn ZRN for hosting chunks honestly
- The system is resilient to node failures and adversarial behavior

## Context

The Tree of Knowledge produces validated training data (Samples) on-chain. The full content of approved samples needs to be stored off-chain in a way that:
1. **Protects the dataset** — it's the core product; free access kills the model
2. **Decentralizes storage** — no single point of failure or censorship
3. **Monetizes access** — ZRN payment unlocks chunks; enough chunks = viable dataset
4. **Incentivizes storage** — nodes earn ZRN for hosting and serving

## Deliverables

### 1. Chunking Strategy

- **Chunk granularity**: How do we break training data into chunks?
  - Per-sample (each Sample = 1 chunk)? Too coarse — one chunk could be independently valuable
  - Sub-sample (split each sample into N fragments)? Better distribution but reassembly complexity
  - Cross-sample mixing (interleave bytes from different samples)? Strongest protection but complex
  - **Recommended**: Reed-Solomon erasure coding — split dataset into K data chunks, generate M parity chunks. Need any K of K+M to reconstruct. Adjustable threshold.
- **Chunk size**: Target 1-10MB per chunk (large enough to amortize overhead, small enough for distribution)
- **Dataset versioning**: Each dataset snapshot gets a new set of chunks. Old versions can be pruned.

### 2. Encryption Scheme

- **Symmetric encryption**: Each chunk encrypted with AES-256-GCM
- **Key management**: 
  - Master dataset key derived per dataset version
  - Chunk keys derived from master key + chunk index (HKDF)
  - Master key split via Shamir's Secret Sharing — threshold of T-of-N key holders
  - Payment unlocks key shares proportional to ZRN paid
- **Alternative — onion encryption**: Each chunk encrypted with a unique key. Keys are released individually on payment. Simpler but less flexible threshold control.
- **Recommended**: Shamir's + per-chunk keys. Payment releases key shares. Reaching threshold T unlocks all chunks.

### 3. Distribution Protocol

- **Storage node registration**: Nodes stake ZRN on-chain (via compute_pool or new mechanism) to become storage nodes
- **Chunk assignment**: Chunks randomly assigned to nodes via VRF (knowledge module already has VRF). Each chunk replicated to R nodes (replication factor, e.g., R=3)
- **Proof of storage**: Periodic challenges — node must prove it still holds its assigned chunks
  - Challenge: provide merkle proof of random byte range within chunk
  - Failure: slashing of staked ZRN, chunk reassigned
- **Node discovery**: Storage nodes register endpoints on-chain. Clients query chain for chunk locations.

### 4. Access Control & Payment

- **Two-tier access**:
  1. **API inference** (low tier): Pay per-call, never touch raw data. Cheapest.
  2. **Dataset access** (high tier): Pay ZRN to unlock chunks. Enough payment = reconstruct dataset. Expensive.
- **Payment → chunk access flow**:
  1. Buyer deposits ZRN to dataset escrow on-chain
  2. Smart contract (or module) records payment amount
  3. Payment amount determines how many key shares / chunk access tickets are released
  4. Buyer requests chunks from storage nodes, presenting access tickets
  5. Storage nodes verify ticket validity (on-chain check or signed receipt)
  6. Below critical threshold: chunks are useless fragments
  7. At/above threshold: buyer can reconstruct full dataset
- **Minimum viable payment**: The threshold should be set so that the minimum payment to reconstruct equals the target dataset price
- **Partial access**: Below threshold, the data fragments could still have some value for narrow fine-tuning. Consider whether to allow or prevent this (trade-off: revenue vs protection)

### 5. Incentive Structure

- **Storage rewards**: Nodes earn ZRN proportional to:
  - Number of chunks stored × time stored
  - Number of chunks served to paying customers
  - Passing proof-of-storage challenges
- **Slashing**: Nodes lose staked ZRN for:
  - Failing proof-of-storage (lost data)
  - Serving chunks without valid access tickets (free-riding)
  - Extended downtime

### 6. Threat Model

Address these attacks:
- **Sybil storage nodes**: One entity runs many nodes to collect all chunks → VRF assignment + stake requirement + geographic diversity scoring
- **Collusion**: Storage nodes share chunks to reconstruct dataset → Chunks are encrypted; nodes only hold ciphertext. Keys are separate from chunks.
- **Free-riding**: Someone gets chunks without paying → Access tickets are cryptographically signed, verified by storage nodes
- **Data poisoning**: Malicious storage node serves corrupted chunks → Merkle root of each chunk stored on-chain; buyer verifies integrity
- **Threshold gaming**: Buyer gets just below threshold, tries to infer missing data → Erasure coding makes partial reconstruction computationally infeasible below threshold

### 7. Implementation Considerations

- How does this integrate with IPFS/Arweave? Or is it a custom p2p network?
- What's the minimum number of storage nodes for launch?
- How do we bootstrap the storage network before there are paying customers?
- Can storage nodes also be inference nodes (dual role)?

## Output

Append to `docs/inference-layer/ARCHITECTURE.md` as a "Blind Storage Protocol" section.
Include protocol diagrams and a worked example (dataset of N samples → chunking → distribution → purchase → reconstruction).
