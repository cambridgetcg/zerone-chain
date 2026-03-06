# T5-2 — Storage Node Network

## Goal

Build the P2P storage node protocol: nodes register on-chain, receive chunk assignments, store encrypted data, respond to proof-of-storage challenges, and serve chunks to authorized buyers.

## Deliverables

### 1. Storage Node Agent

A Go binary that storage node operators (agents) run:
- **Register**: Stake ZRN on-chain, advertise storage capacity and endpoint
- **Receive chunks**: Accept assigned chunks from the chunk engine
- **Store**: Persist encrypted chunks to local disk
- **Serve**: Respond to authorized chunk requests (verify access ticket)
- **Prove**: Respond to proof-of-storage challenges

### 2. Chunk Assignment Protocol

- On dataset publication, chunk manifest lists target replication (R=3)
- Assignment via VRF (already in x/knowledge): deterministic but unpredictable node selection
- Each chunk assigned to R nodes based on VRF output + node stake weight
- Nodes that fail to accept assignment within deadline: chunk reassigned, minor slash
- Geographic/operator diversity scoring: avoid assigning all replicas to same operator

### 3. Proof of Storage

Periodic challenges (on-chain via EndBlocker or off-chain via challenger service):
1. Challenge: "Prove you hold chunk X by providing merkle proof of bytes at offset Y-Z"
2. Node reads the requested byte range, computes merkle proof, submits response
3. Verifier checks proof against chunk's merkle hash in manifest
4. Pass: node earns storage reward for the period
5. Fail: chunk marked at-risk, reassigned, node slashed

Challenge frequency: configurable (e.g., each chunk challenged once per epoch)

### 4. Access Control

When a buyer requests a chunk:
1. Buyer presents access ticket (signed by payment bridge after ZRN payment verified)
2. Storage node verifies ticket: checks signature, checks chunk index matches ticket scope, checks expiry
3. If valid: serve encrypted chunk
4. If invalid: reject with error

Access tickets include:
- Buyer address
- Dataset ID
- Authorized chunk indices (or "all")
- Expiry block height
- Payment bridge signature

### 5. Node Economics

- **Earning**: Storage reward per chunk per epoch + serving fee per chunk served
- **Staking**: Minimum stake to participate (e.g., 1000 uzrn per TB advertised)
- **Slashing**: Lost chunk = 10% of stake per chunk. Extended downtime = 1% per missed epoch.
- **Unstaking**: Cool-down period. Must ensure chunks are replicated to replacement nodes first.

### 6. Node Discovery

- On-chain registry of active storage nodes with endpoints
- Query: "Which nodes hold chunks for dataset X?" → read manifest + node registry
- Lightweight gossip protocol for node health (or rely on on-chain heartbeats)

## Working Directory

`/Users/yournameisai/Desktop/zerone/services/blind-storage/node/`

## Output

- Storage node binary (Go)
- Proof-of-storage protocol specification
- Access ticket format and verification
- Integration tests: assign chunks → store → challenge → serve → verify
- Dockerfile for storage node deployment
