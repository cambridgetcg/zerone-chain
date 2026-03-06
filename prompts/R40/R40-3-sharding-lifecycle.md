# R40-3 — Sharding Lifecycle

## Objective

Wire `sharding.go` into the BeginBlocker, genesis, and validator set change hooks so shard assignments are computed, rotated, and persisted as part of the chain lifecycle.

## Context

- `sharding.go` has: `ComputeShardAssignments()`, `ApplyShardAssignments()`, `GetValidatorShard()`, `AttestProofOfStorage()`
- Sharding params: `ReplicationFactor` (default 3), `SnapshotInterval` (blocks between reshuffles), `MinValidators`
- Assignment is deterministic from `SHA-256(tdu_hash || snapshot_block_hash)`
- Validators must attest proof-of-storage each cycle

## Tasks

1. **In BeginBlocker** (every `SnapshotInterval` blocks):
   - Get current block hash from `ctx.BlockHeader().LastBlockId.Hash`
   - Get all active validator addresses from staking keeper
   - Get all accepted TDU/Sample IDs (fitness ≥ 0.1, not Pruned)
   - Call `ComputeShardAssignments()` and `ApplyShardAssignments()` to persist new assignments
   - Emit `EventShardReshuffle` with snapshot height and validator count

2. **Validator set changes**:
   - On new validator joining (hook from staking module): no immediate reshuffle, wait for next snapshot
   - On validator leaving/jailed: mark their shard assignments as orphaned, redistribute at next snapshot
   - If active validators < `MinValidators`: skip reshuffling, emit warning event

3. **Proof-of-storage attestation**:
   - Add `MsgAttestStorage` handler in `msg_server.go`
   - Validators submit attestation (signed hash of their assigned TDU data) each cycle
   - Track attestation status per validator per snapshot
   - Missing attestation after grace period (2× SnapshotInterval): slash event (integrate with slashing keeper if available, otherwise emit event for governance)

4. **Genesis export/import**:
   - Export: all shard assignments, attestation records, sharding params
   - Import: restore state, validate assignments against current validator set

5. **Pruned TDU handling**:
   - When fitness.go marks a TDU as Pruned, remove it from next shard assignment computation
   - Existing assignments for pruned TDUs cleaned up at next snapshot

## Tests

- Test: BeginBlocker triggers reshuffle at correct interval
- Test: shard assignments are deterministic (same inputs → same outputs)
- Test: different block hash → different assignments
- Test: each TDU assigned to exactly R validators
- Test: genesis export → import round-trips all state
- Test: pruned TDU excluded from next reshuffle
- Test: fewer than MinValidators → skip reshuffle with warning

## Key Files

- `x/knowledge/keeper/sharding.go` — call into
- `x/knowledge/keeper/ecology.go` or `phases.go` — add to BeginBlocker
- `x/knowledge/keeper/msg_server.go` — add MsgAttestStorage
- `x/knowledge/keeper/genesis.go` — export/import
- `x/knowledge/types/sharding.go` — types

## Constraints

- Do NOT modify proto files — add Go types only
- SnapshotInterval default: 1000 blocks (~1 hour on testnet)
- Use existing staking keeper interface for validator list
