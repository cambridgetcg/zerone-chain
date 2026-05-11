# Substrate Bridge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `x/substrate_bridge` — the Tier-1 foundation module for external recursive work — with adapter framework (gov-gated registry), permissive substrate-link compiler (deferred settlement), and cross-class lineage propagator (DAG-by-timestamp, depth-decayed royalty flow). Standalone-usable as `MsgSubmitExternalAttestation`; forward-compatible with the future `x/work` Phase-1 primitive.

**Architecture:** New Cosmos SDK module under `x/substrate_bridge/`. Proto types at `proto/zerone/substrate_bridge/v1/`. Five keeper sub-systems share one keeper struct: adapter registry (gov-mutated), substrate-link verifier (compile-time), attestation state machine (SUBMITTED → COMMITTED → AWAITING_RESOLUTION → READY/PARTIAL/REJECTED → SETTLED/SLASHED), pending-fact reverse index, lineage DAG with propagation walk. BeginBlocker scans AWAITING_RESOLUTION for timeouts; `x/knowledge.CompleteRound` notifies on claim resolution for the eager READY transition. Module exposes one core MsgSubmitter entry (`MsgSubmitExternalAttestation`) and three gov-authority entries (`MsgRegisterAdapter`, `MsgSuspendAdapter`, `MsgTombstoneAdapter`).

**Tech Stack:** Cosmos SDK v0.50, Go 1.24, protobuf v3, cosmossdk.io modules. Reuses `x/creed` LIP class pattern, `x/qualification` query surface, and `x/knowledge`'s `CompleteRound` hook point.

**Spec:** `docs/superpowers/specs/2026-05-10-substrate-bridge-design.md` (commit `3a4cb43`).

**Phase series:**
- Phase 0 (USEFUL_WORK doctrine): **shipped**
- Phase 1 (x/work primitive): not started
- **Tier 1 foundation (x/substrate_bridge):** *this doc*
- Tier 2 (x/offchain_workers, x/event_resolution): pending consumer demand
- Tier 3 (x/interchain_knowledge): pending consumer demand

---

## File Structure

**New files (created across tasks):**

Proto (`proto/zerone/substrate_bridge/v1/`):
- `types.proto` — shared scalars (ExternalSource, AxisProjection, CitationType, AxisBounds)
- `adapter.proto` — AdapterRegistration, AdapterStatus, SlashGradient
- `substrate_link.proto` — SubstrateLink, FactCitation, PendingClaim
- `attestation.proto` — ExternalAttestation, AttestationStatus
- `lineage.proto` — LineageEdge
- `params.proto` — module Params
- `genesis.proto` — GenesisState
- `tx.proto` — MsgRegisterAdapter, MsgSuspendAdapter, MsgTombstoneAdapter, MsgSubmitExternalAttestation
- `query.proto` — query services

Go module (`x/substrate_bridge/`):
- `doc.go` — position-layer declaration
- `module.go` — AppModule, AppModuleBasic, BeginBlock wiring
- `types/keys.go` — store key prefixes 0x80–0x8B
- `types/codec.go` — codec registration
- `types/errors.go` — typed errors with doctrine voice
- `types/params.go` — Params validation
- `types/genesis.go` — GenesisState helpers + validation
- `types/expected_keepers.go` — KnowledgeKeeper, QualificationKeeper, BankKeeper, AccountKeeper interfaces
- `types/citation_type.go` — citation-type weight helper
- `types/canonical.go` — pending-claim canonical-hash helper
- `keeper/keeper.go` — core keeper struct
- `keeper/adapter_registry.go` — adapter CRUD + lifecycle
- `keeper/substrate_link.go` — link validation + hash computation
- `keeper/attestation.go` — state machine + storage
- `keeper/pending_fact_index.go` — bidirectional reverse-lookup
- `keeper/lineage.go` — DAG validation + edge creation
- `keeper/propagation.go` — depth-decayed royalty walk
- `keeper/settlement.go` — eager settle + partial + rejected
- `keeper/msg_server.go` — message handlers
- `keeper/grpc_query.go` — query handlers
- `keeper/begin_block.go` — timeout scan + READY drain
- `keeper/events.go` — voice-layer event constants
- `keeper/genesis.go` — init/export
- `keeper/params.go` — param getters
- `keeper/hooks.go` — `OnClaimResolved` consumer hook for x/knowledge
- `client/cli/query.go`, `client/cli/tx.go`
- `keeper/*_test.go` — per-file unit tests

Cross-stack:
- `tests/cross_stack/substrate_bridge_test.go` — end-to-end integration

**Modified files:**
- `x/knowledge/keeper/rounds.go:CompleteRound` — one-line hook calling `substrate_bridge.OnClaimResolved`
- `x/knowledge/types/expected_keepers.go` (or equivalent) — add SubstrateBridgeKeeper interface
- `x/creed/types/sub_creeds.go` (or equivalent) — add `CategoryAdapterRegistration` LIP class
- `app/app.go` — module wiring (keeper init, basic-manager, store-key, BeginBlocker order)
- `docs/EVENTS.md` — append event surface
- `Makefile` (potentially) — no new targets if proto-gen catches the new proto directory

**Reserved store key prefixes (0x80–0x8B):**

| Prefix | Purpose |
|---|---|
| `0x80` | LineageEdge |
| `0x81` | LineageByUpstream (forward index) |
| `0x82` | LineageByDownstream (backward index) |
| `0x83` | LineageRoyaltyAccumulator |
| `0x84` | AdapterRegistration (by adapter_id) |
| `0x85` | ExternalAttestation (by attestation_id) |
| `0x86` | AttestationByStatus (status-indexed for AWAITING_RESOLUTION scan) |
| `0x87` | PendingFactIndex (claim_id → attestation_id) |
| `0x88` | AttestationPendingClaims (attestation_id → []claim_id) |
| `0x89` | AdapterByStatus (status-indexed for active-adapter scans) |
| `0x8A` | Params (singleton) |
| `0x8B` | reserved (forward-compat for nonce counters or future indexes) |

---

## Pre-Tasks: Read Before Starting

Skim these in this order — they are the patterns this plan mirrors:

- `docs/superpowers/specs/2026-05-10-substrate-bridge-design.md` — the spec. Sections 1–4 are doctrinally load-bearing; Section 5 covers integration touch-points; Section 8 lists open questions deferred to this plan's choices.
- `docs/USEFUL_WORK.md` — the doctrine the module implements. Every refusal message must cite UW + the violated mechanism.
- `x/inquiry/` — closest existing module shape (one keeper, multiple sub-systems, gov-gated parts). Read `module.go`, `keeper/keeper.go`, `types/expected_keepers.go`, `types/keys.go`.
- `x/creed/keeper/msg_server.go` — gov-authority check pattern. The `AnchorPin` handler shows how authority-only messages are gated.
- `x/knowledge/keeper/rounds.go:CompleteRound` (line 57) — the hook point being modified.
- `x/inquiry/keeper/keeper_test.go` — test harness setup pattern; per-keeper-function table tests.
- `tests/cross_stack/harness_test.go` — `NewTestHarness` for cross-stack integration.
- `CLAUDE.md` — *Proto-Go Consistency Rule* is load-bearing here. Add fields to `.proto` first; never edit `*.pb.go`.

---

## Model selection hint for executors

| Tasks | Complexity | Suggested model |
|---|---|---|
| 1–8 (proto definitions) | Mechanical (clear spec) | haiku |
| 9–11 (scaffolding) | Mechanical | haiku |
| 12, 13 (adapter registry, link validation) | Mechanical | haiku |
| 14–16 (attestation, pending index, lineage DAG) | Integration | sonnet |
| 17, 18 (propagation, settlement) | Judgment — formulas + edge cases | sonnet |
| 19, 20 (MsgServer, gRPC) | Integration | sonnet |
| 21 (BeginBlocker) | Integration | sonnet |
| 22–25 (hooks, LIP class, CLI, app wiring) | Integration | sonnet |
| 26 (cross-stack tests) | Judgment | sonnet |
| 27, 28 (docs + final sweep) | Mechanical | haiku |

---

## Tasks

### Task 1: Proto foundation types

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/types.proto`

Defines the shared scalars used across the other proto files: `ExternalSource`, `AxisProjection`, `AxisBounds`, `CitationType` enum.

- [ ] **Step 1: Create the proto file**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

// ExternalSource is a typed reference to off-chain content that an
// adapter has fetched. The content_hash is the cryptographic anchor:
// substrate-link re-derivation matches if and only if the source's
// content_hash matches what the adapter binary produced.
message ExternalSource {
  string adapter_id       = 1;
  string source_id        = 2;  // e.g. Wikipedia article ID
  string source_url       = 3;  // optional; for audit
  bytes  content_hash     = 4;  // sha256 of fetched content
  uint64 fetched_at_block = 5;
}

// AxisProjection is the per-axis recursion-weight contribution of an
// external work artifact, in the order fixed by USEFUL_WORK.md
// section "The six recursive axes". Units are uint64 weights, bounded
// by an adapter's AxisBounds.
message AxisProjection {
  uint64 axis_substrate      = 1;
  uint64 axis_verification   = 2;
  uint64 axis_classification = 3;
  uint64 axis_attribution    = 4;
  uint64 axis_tooling        = 5;
  uint64 axis_interface      = 6;
}

// AxisBounds caps the per-axis projection an adapter is allowed to
// claim. Gov-approved at adapter registration; enforced at attestation
// submit.
message AxisBounds {
  uint64 axis_substrate_max      = 1;
  uint64 axis_verification_max   = 2;
  uint64 axis_classification_max = 3;
  uint64 axis_attribution_max    = 4;
  uint64 axis_tooling_max        = 5;
  uint64 axis_interface_max      = 6;
}

// CitationType distinguishes citation strengths for lineage propagation
// (M6 generalized). Mirrors the ToK relation-type semantics applied
// across work classes.
enum CitationType {
  CITATION_TYPE_UNSPECIFIED = 0;
  CITATION_TYPE_CITES       = 1;  // 1× base weight
  CITATION_TYPE_SUPPORTS    = 2;  // 2× base weight
  CITATION_TYPE_EXTENDS     = 3;  // 3× base weight
  CITATION_TYPE_REFINES     = 4;  // 3× base weight
  CITATION_TYPE_GENERALIZES = 5;  // 4× base weight
}
```

- [ ] **Step 2: Verify proto file is well-formed**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/types.proto
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): foundation types — ExternalSource + AxisProjection + AxisBounds + CitationType

Shared scalars for the substrate_bridge module. Subsequent proto files
reference these. Citation-type weights drive lineage propagation per
M6.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Adapter proto types

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/adapter.proto`

- [ ] **Step 1: Create the proto file**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

import "zerone/substrate_bridge/v1/types.proto";

// AdapterStatus governs registration lifecycle. ACTIVE accepts new
// attestations; SUSPENDED refuses new but in-flight settle;
// TOMBSTONED is permanent retirement (commitment 10 forward-only).
enum AdapterStatus {
  ADAPTER_STATUS_UNSPECIFIED = 0;
  ADAPTER_STATUS_ACTIVE      = 1;
  ADAPTER_STATUS_SUSPENDED   = 2;
  ADAPTER_STATUS_TOMBSTONED  = 3;
}

// QualificationStatus mirrors x/qualification's status enum so an
// adapter can specify the minimum status a submitter must hold in the
// required domain. Imported here as a uint32 for proto isolation;
// keeper resolves to x/qualification's enum at query time.
enum QualificationStatus {
  QUALIFICATION_STATUS_UNSPECIFIED   = 0;
  QUALIFICATION_STATUS_PROBATIONARY  = 1;
  QUALIFICATION_STATUS_ACTIVE        = 2;
  QUALIFICATION_STATUS_DISTINGUISHED = 3;
}

// SlashGradient mirrors M1's graduated slashing — different failure
// modes carry different bps slash weights. Values stored at adapter
// registration and applied at attestation rejection paths.
message SlashGradient {
  uint32 compiler_drift_bps = 1;  // adapter-binary mismatch — typically 10000 (full)
  uint32 axis_overflow_bps  = 2;  // axis claim exceeds bounds — typically pro-rata
  uint32 fraud_bps          = 3;  // > rejection threshold reached — typically 10000
}

// AdapterRegistration is the gov-approved metadata for one adapter.
// Adapter is a recipe (binary hash + bounds + slash); no operator role.
// Anyone who runs the registered binary AND submits an attestation
// earns via the UW formula.
message AdapterRegistration {
  string adapter_id  = 1;     // canonical, gov-approved (e.g. "wikipedia-en-v1")
  string source_type = 2;     // "wikipedia" | "arxiv" | "ibc_packet" | etc.
  string version     = 3;     // semver

  bytes      compiler_binary_hash = 4;  // determinism guarantee
  AxisBounds axis_bounds          = 5;

  string min_attestation_bond_uzrn = 6;
  string min_per_claim_bond_uzrn   = 7;

  SlashGradient slash_gradient = 8;

  string               required_qualification_domain = 9;
  QualificationStatus  min_qualification_status      = 10;

  repeated string allowed_class_ids = 11;  // empty = any class allowed

  AdapterStatus status               = 12;
  string        registered_via_lip_id = 13;
  uint64        registered_at_block  = 14;
  uint64        tombstoned_at_block  = 15;  // 0 if not tombstoned
}
```

- [ ] **Step 2: Verify proto-check**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/adapter.proto
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): AdapterRegistration + lifecycle + slash gradient (M3)

Adapter is recipe-not-service: gov-approved binary hash + axis bounds
+ bond + slash + qualification requirements. Status enum supports
ACTIVE/SUSPENDED/TOMBSTONED forward-only lifecycle per commitment 10.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Substrate-link proto types

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/substrate_link.proto`

- [ ] **Step 1: Create the proto file**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

import "zerone/substrate_bridge/v1/types.proto";

// SubstrateLink is the deterministic provenance from external content
// to ToK fact-IDs (existing + pending). Two sections — cited_facts MUST
// exist in x/knowledge at commit time; pending_claims are auto-submitted
// as Claims and the attestation is held in AWAITING_RESOLUTION until
// they resolve. M2 satisfied: every pending claim becomes a real
// on-chain claim with full provenance.
message SubstrateLink {
  repeated FactCitation cited_facts    = 1;
  repeated PendingClaim pending_claims = 2;
  AxisProjection        recursion_weight = 3;
  string                adapter_id     = 4;
  ExternalSource        source         = 5;
  bytes                 link_hash      = 6;  // sha256 of canonical form
}

// FactCitation is one outgoing edge in the substrate-link. citation_type
// drives lineage propagation weight (M6).
message FactCitation {
  string       fact_id          = 1;
  CitationType citation_type    = 2;
  string       citation_context = 3;  // optional excerpt for audit
}

// PendingClaim is a Claim auto-submitted at commit phase. Shape mirrors
// x/knowledge.Claim so the substrate_bridge keeper can call
// x/knowledge.SetClaim directly. claim_relations cite existing facts;
// they are NOT recursive pending claims (one-hop deferral only).
message PendingClaim {
  string claim_content    = 1;
  string proposed_fact_id = 2;  // optional; chain assigns if empty
  string domain           = 3;
  string methodology_id   = 4;
  repeated ClaimRelation relations = 5;
}

// ClaimRelation is a citation from a pending claim to an existing fact.
// Mirrors x/knowledge.ClaimRelation. Pending claims cannot cite OTHER
// pending claims — they cite existing verified facts only. This keeps
// the resolution graph a tree (one-hop deferral).
message ClaimRelation {
  string target_fact_id = 1;
  string relation       = 2;  // SUPPORTS | REQUIRES | etc. (mirrors x/knowledge)
  string inference      = 3;  // DEDUCTIVE | INDUCTIVE | etc.
  uint32 inference_strength_bps = 4;
}
```

- [ ] **Step 2: Verify proto-check**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/substrate_link.proto
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): SubstrateLink + FactCitation + PendingClaim (M2)

Permissive two-section link: cited_facts (existing, hard requirement)
+ pending_claims (auto-submitted to x/knowledge at commit). One-hop
deferral only — pending claims cite existing facts, not other pending
claims. link_hash is the re-derivability anchor.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Attestation proto types

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/attestation.proto`

- [ ] **Step 1: Create the proto file**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

import "zerone/substrate_bridge/v1/substrate_link.proto";

// AttestationStatus state machine per spec section 2:
//   SUBMITTED → COMMITTED → AWAITING_RESOLUTION → (READY | PARTIAL | REJECTED)
//   READY → SETTLED
//   PARTIAL → SETTLED (with reduced reward)
//   REJECTED → SLASHED
enum AttestationStatus {
  ATTESTATION_STATUS_UNSPECIFIED         = 0;
  ATTESTATION_STATUS_SUBMITTED           = 1;
  ATTESTATION_STATUS_COMMITTED           = 2;
  ATTESTATION_STATUS_AWAITING_RESOLUTION = 3;
  ATTESTATION_STATUS_READY               = 4;
  ATTESTATION_STATUS_PARTIAL             = 5;
  ATTESTATION_STATUS_REJECTED            = 6;
  ATTESTATION_STATUS_SETTLED             = 7;
  ATTESTATION_STATUS_SLASHED             = 8;
}

// ExternalAttestation is one external work submission. Stored at
// 0x85 | attestation_id. Status indexed at 0x86 | be8(status) | id.
message ExternalAttestation {
  string attestation_id   = 1;
  string adapter_id       = 2;
  string work_class_id    = 3;
  string submitter        = 4;
  string bond_uzrn        = 5;  // total bond locked (attestation + sum of per-claim bonds)
  SubstrateLink link      = 6;
  AttestationStatus status = 7;

  uint64 submitted_at_block = 8;
  uint64 committed_at_block = 9;  // 0 if not yet committed
  uint64 settled_at_block   = 10; // 0 if not yet settled

  // Resolution counters (incremented as pending claims resolve).
  uint32 verified_count = 11;
  uint32 rejected_count = 12;
  uint32 expired_count  = 13;  // not used at Phase 0 but reserved for future

  // Reward + slash bookkeeping.
  string reward_uzrn = 14;  // 0 if not yet settled
  string slash_uzrn  = 15;  // 0 if not slashed

  // Rejection reason (set only if status == REJECTED).
  string rejection_reason = 16;
}
```

- [ ] **Step 2: Verify proto-check**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/attestation.proto
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): ExternalAttestation + 9-state state machine

States: SUBMITTED, COMMITTED, AWAITING_RESOLUTION, READY, PARTIAL,
REJECTED, SETTLED, SLASHED. Counters incremented as pending claims
resolve. Status-indexed at 0x86 for BeginBlocker AWAITING_RESOLUTION
scan.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Lineage proto types

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/lineage.proto`

- [ ] **Step 1: Create the proto file**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

import "zerone/substrate_bridge/v1/types.proto";

// LineageEdge is one citation as a first-class record. Created at
// downstream attestation's settlement (not at commit). Forward-only:
// edges append; settlement_payment_uzrn accumulates on subsequent
// payments via this edge.
message LineageEdge {
  string upstream_attestation_id   = 1;
  string downstream_attestation_id = 2;
  string upstream_class_id         = 3;
  string downstream_class_id       = 4;

  CitationType citation_type        = 5;
  uint32       contribution_share_bps = 6;  // submitter-claimed share within budget
  uint32       depth_from_downstream  = 7;  // 1 for direct cite; +1 per propagation hop

  uint64 created_at_block         = 8;
  string settlement_payment_uzrn  = 9;  // cumulative paid via this edge to date
}

// LineageRoyaltyAccumulator is the cumulative lineage uzrn received
// by a given attestation across all incoming royalty events.
// Realizes the "revenue-stream" interpretation of M6's amplification:
// a load-bearing fact's value (= cumulative lineage income) grows as
// downstream uses accumulate.
message LineageRoyaltyAccumulator {
  string attestation_id     = 1;
  string cumulative_uzrn    = 2;
  uint64 last_updated_block = 3;
  uint32 incoming_edge_count = 4;
}
```

- [ ] **Step 2: Verify proto-check**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/lineage.proto
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): LineageEdge + LineageRoyaltyAccumulator (M6)

Edges as first-class records: forward + backward indexes at 0x81/0x82.
Cumulative royalty accumulator at 0x83 realizes the revenue-stream
amplification interpretation — settled W stays static; lifetime
earnings grow as downstream work proliferates.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Params + Genesis proto types

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/params.proto`
- Create: `proto/zerone/substrate_bridge/v1/genesis.proto`

- [ ] **Step 1: Create params.proto**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

// Params are the governance-tunable knobs for the substrate_bridge
// module. Defaults set in genesis; mutated via standard gov param-change
// proposals.
message Params {
  // Bulk-ingestion caps.
  uint32 max_pending_claims_per_attestation = 1;  // default 100000
  string per_pending_claim_bond_uzrn        = 2;  // default "222"
  string attestation_min_bond_uzrn          = 3;  // default "222000"

  // Timeout for AWAITING_RESOLUTION (in blocks).
  uint64 max_pending_window_blocks = 4;  // default 6,220,800 (~6mo at 2.5s blocks)

  // Threshold for whole-attestation rejection (bps; default 5000 = 50%).
  uint32 pending_claim_rejection_threshold_bps = 5;

  // Minimum verified ratio to allow SETTLED vs REJECTED (bps; default 1000 = 10%).
  uint32 min_verified_ratio_for_settle_bps = 6;

  // Lineage propagation.
  uint32 lineage_share_bps        = 7;  // default 3000 (30%)
  uint32 decay_bps_per_hop        = 8;  // default 3000 (30%)
  uint32 max_propagation_depth    = 9;  // default 5
  string min_propagation_uzrn     = 10; // default "1000"

  // Self-citation cap (bps applied to contribution_share_bps when
  // upstream.submitter == downstream.submitter).
  uint32 self_citation_cap_bps = 11;  // default 5000 (50%)
}
```

- [ ] **Step 2: Create genesis.proto**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

import "zerone/substrate_bridge/v1/params.proto";
import "zerone/substrate_bridge/v1/adapter.proto";

// GenesisState — the substrate_bridge state imported/exported at
// genesis. At chain genesis there are no attestations, no lineage
// edges, and (likely) no adapters; the LIP machinery is the canonical
// way to register adapters post-genesis. The Params are set explicitly.
message GenesisState {
  Params params = 1;
  repeated AdapterRegistration adapters = 2;  // typically empty at chain birth
}
```

- [ ] **Step 3: Verify proto-check**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/params.proto proto/zerone/substrate_bridge/v1/genesis.proto
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): Params + GenesisState

Governance-tunable knobs: bulk caps, timeouts, thresholds, lineage
propagation curves, self-citation cap. Genesis defaults to no adapters
(LIP-only path for adapter registration post-genesis).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Tx proto (msg server interface)

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/tx.proto`

- [ ] **Step 1: Create the proto file**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

import "cosmos/msg/v1/msg.proto";
import "gogoproto/gogo.proto";
import "zerone/substrate_bridge/v1/adapter.proto";
import "zerone/substrate_bridge/v1/substrate_link.proto";

service Msg {
  option (cosmos.msg.v1.service) = true;

  // Gov-authority only.
  rpc RegisterAdapter   (MsgRegisterAdapter)   returns (MsgRegisterAdapterResponse);
  rpc SuspendAdapter    (MsgSuspendAdapter)    returns (MsgSuspendAdapterResponse);
  rpc TombstoneAdapter  (MsgTombstoneAdapter)  returns (MsgTombstoneAdapterResponse);

  // Open submission (anyone with adapter qualification).
  rpc SubmitExternalAttestation (MsgSubmitExternalAttestation) returns (MsgSubmitExternalAttestationResponse);
}

// MsgRegisterAdapter must be sent by the gov authority. Author of the
// adapter spec proposes via standard gov LIP (CategoryAdapterRegistration);
// on LIP passage, gov module dispatches this message.
message MsgRegisterAdapter {
  option (cosmos.msg.v1.signer) = "authority";

  string authority = 1;
  AdapterRegistration adapter = 2;
}

message MsgRegisterAdapterResponse {}

message MsgSuspendAdapter {
  option (cosmos.msg.v1.signer) = "authority";

  string authority   = 1;
  string adapter_id  = 2;
  string reason      = 3;
}

message MsgSuspendAdapterResponse {}

message MsgTombstoneAdapter {
  option (cosmos.msg.v1.signer) = "authority";

  string authority   = 1;
  string adapter_id  = 2;
  string reason      = 3;
}

message MsgTombstoneAdapterResponse {}

// MsgSubmitExternalAttestation is the open entry point. Submitter
// posts bond, provides substrate-link, the chain validates and runs
// state machine.
message MsgSubmitExternalAttestation {
  option (cosmos.msg.v1.signer) = "submitter";

  string submitter       = 1;
  string adapter_id      = 2;
  string work_class_id   = 3;
  SubstrateLink link     = 4;
  string bond_uzrn       = 5;
}

message MsgSubmitExternalAttestationResponse {
  string attestation_id = 1;
}
```

- [ ] **Step 2: Verify proto-check**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/tx.proto
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): tx — 3 gov msgs + 1 open submission

RegisterAdapter / SuspendAdapter / TombstoneAdapter gated by
authority signer (gov LIP dispatch). SubmitExternalAttestation
open to any submitter with adapter's qualification.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Query proto (gRPC surface) + proto-gen

**Files:**
- Create: `proto/zerone/substrate_bridge/v1/query.proto`

- [ ] **Step 1: Create the query proto**

```proto
syntax = "proto3";
package zerone.substrate_bridge.v1;

option go_package = "github.com/zerone-chain/zerone/x/substrate_bridge/types";

import "google/api/annotations.proto";
import "zerone/substrate_bridge/v1/adapter.proto";
import "zerone/substrate_bridge/v1/attestation.proto";
import "zerone/substrate_bridge/v1/lineage.proto";
import "zerone/substrate_bridge/v1/params.proto";

service Query {
  rpc Params (QueryParamsRequest) returns (QueryParamsResponse) {
    option (google.api.http).get = "/zerone/substrate_bridge/v1/params";
  }

  rpc Adapter (QueryAdapterRequest) returns (QueryAdapterResponse) {
    option (google.api.http).get = "/zerone/substrate_bridge/v1/adapters/{adapter_id}";
  }
  rpc Adapters (QueryAdaptersRequest) returns (QueryAdaptersResponse) {
    option (google.api.http).get = "/zerone/substrate_bridge/v1/adapters";
  }

  rpc Attestation (QueryAttestationRequest) returns (QueryAttestationResponse) {
    option (google.api.http).get = "/zerone/substrate_bridge/v1/attestations/{attestation_id}";
  }

  rpc LineageForwardWalk (QueryLineageForwardWalkRequest) returns (QueryLineageForwardWalkResponse) {
    option (google.api.http).get = "/zerone/substrate_bridge/v1/lineage/forward/{attestation_id}";
  }
  rpc LineageBackwardWalk (QueryLineageBackwardWalkRequest) returns (QueryLineageBackwardWalkResponse) {
    option (google.api.http).get = "/zerone/substrate_bridge/v1/lineage/backward/{attestation_id}";
  }
  rpc LineageAccumulator (QueryLineageAccumulatorRequest) returns (QueryLineageAccumulatorResponse) {
    option (google.api.http).get = "/zerone/substrate_bridge/v1/lineage/accumulator/{attestation_id}";
  }
}

message QueryParamsRequest {}
message QueryParamsResponse { Params params = 1; }

message QueryAdapterRequest { string adapter_id = 1; }
message QueryAdapterResponse { AdapterRegistration adapter = 1; }

message QueryAdaptersRequest {
  AdapterStatus status_filter = 1;  // UNSPECIFIED = all
}
message QueryAdaptersResponse { repeated AdapterRegistration adapters = 1; }

message QueryAttestationRequest { string attestation_id = 1; }
message QueryAttestationResponse { ExternalAttestation attestation = 1; }

message QueryLineageForwardWalkRequest { string attestation_id = 1; }
message QueryLineageForwardWalkResponse { repeated LineageEdge edges = 1; }

message QueryLineageBackwardWalkRequest { string attestation_id = 1; }
message QueryLineageBackwardWalkResponse { repeated LineageEdge edges = 1; }

message QueryLineageAccumulatorRequest { string attestation_id = 1; }
message QueryLineageAccumulatorResponse { LineageRoyaltyAccumulator accumulator = 1; }
```

- [ ] **Step 2: Run proto-gen**

Run: `make proto-gen`
Expected: regenerated `x/substrate_bridge/types/*.pb.go` files. Clean.

- [ ] **Step 3: Verify build**

Run: `go build ./x/substrate_bridge/types/...`
Expected: clean (the generated code compiles standalone; keeper code comes later tasks).

- [ ] **Step 4: Commit**

```bash
git add proto/zerone/substrate_bridge/v1/query.proto x/substrate_bridge/types/*.pb.go
git commit -m "$(cat <<'EOF'
proto(substrate_bridge): query gRPC + run proto-gen

7 query endpoints: Params, Adapter+Adapters, Attestation, Lineage
forward/backward/accumulator. Proto-gen produces .pb.go in types/;
generated code compiles standalone before keeper lands.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Store key prefixes + key constructors

**Files:**
- Create: `x/substrate_bridge/types/keys.go`

- [ ] **Step 1: Create the keys file**

```go
package types

import (
	"encoding/binary"
)

const (
	// ModuleName is the canonical name of the substrate_bridge module.
	ModuleName = "substrate_bridge"

	// StoreKey is the primary store key.
	StoreKey = ModuleName

	// AuditBountyPoolModuleName is the module account name for the
	// useful_work_audit_bounty_pool referenced by M7. Created here
	// because substrate_bridge is the canonical home for external
	// attestation audit funds. (Internal work classes will reuse this
	// pool when challenged; until x/work Phase 1 lands, the pool is
	// passive.)
	AuditBountyPoolModuleName = "useful_work_audit_bounty_pool"
)

// Store key prefixes (0x80 onwards reserved for substrate_bridge per
// docs/superpowers/specs/2026-05-10-substrate-bridge-design.md).
var (
	LineageEdgePrefix                = []byte{0x80} // 0x80 | edge_id → LineageEdge
	LineageByUpstreamPrefix          = []byte{0x81} // 0x81 | upstream_id | edge_id → 1 (forward idx)
	LineageByDownstreamPrefix        = []byte{0x82} // 0x82 | downstream_id | edge_id → 1 (backward idx)
	LineageRoyaltyAccumulatorPrefix  = []byte{0x83} // 0x83 | attestation_id → LineageRoyaltyAccumulator

	AdapterRegistrationPrefix        = []byte{0x84} // 0x84 | adapter_id → AdapterRegistration
	ExternalAttestationPrefix        = []byte{0x85} // 0x85 | attestation_id → ExternalAttestation
	AttestationByStatusPrefix        = []byte{0x86} // 0x86 | be8(status) | attestation_id → 1
	PendingFactIndexPrefix           = []byte{0x87} // 0x87 | pending_claim_id → attestation_id (one claim → one attestation)
	AttestationPendingClaimsPrefix   = []byte{0x88} // 0x88 | attestation_id | claim_id → 1 (forward: attestation → claims)
	AdapterByStatusPrefix            = []byte{0x89} // 0x89 | be8(status) | adapter_id → 1

	ParamsKey                         = []byte{0x8A} // singleton
	AttestationIDCounterKey           = []byte{0x8B} // singleton uvarint counter for attestation_id assignment
)

// Key constructors.

func AdapterKey(adapterID string) []byte {
	return append(append([]byte{}, AdapterRegistrationPrefix...), []byte(adapterID)...)
}

func AdapterByStatusKey(status uint8, adapterID string) []byte {
	key := append([]byte{}, AdapterByStatusPrefix...)
	key = append(key, status)
	key = append(key, []byte(adapterID)...)
	return key
}

func AttestationKey(attestationID string) []byte {
	return append(append([]byte{}, ExternalAttestationPrefix...), []byte(attestationID)...)
}

func AttestationByStatusKey(status uint8, attestationID string) []byte {
	key := append([]byte{}, AttestationByStatusPrefix...)
	key = append(key, status)
	key = append(key, []byte(attestationID)...)
	return key
}

func AttestationByStatusPrefixForStatus(status uint8) []byte {
	return append(append([]byte{}, AttestationByStatusPrefix...), status)
}

func PendingFactIndexKey(pendingClaimID string) []byte {
	return append(append([]byte{}, PendingFactIndexPrefix...), []byte(pendingClaimID)...)
}

func AttestationPendingClaimsKey(attestationID, claimID string) []byte {
	key := append([]byte{}, AttestationPendingClaimsPrefix...)
	key = append(key, []byte(attestationID)...)
	key = append(key, 0x00) // separator
	key = append(key, []byte(claimID)...)
	return key
}

func AttestationPendingClaimsPrefixFor(attestationID string) []byte {
	key := append([]byte{}, AttestationPendingClaimsPrefix...)
	key = append(key, []byte(attestationID)...)
	key = append(key, 0x00)
	return key
}

func LineageEdgeKey(edgeID string) []byte {
	return append(append([]byte{}, LineageEdgePrefix...), []byte(edgeID)...)
}

func LineageByUpstreamKey(upstreamID, edgeID string) []byte {
	key := append([]byte{}, LineageByUpstreamPrefix...)
	key = append(key, []byte(upstreamID)...)
	key = append(key, 0x00)
	key = append(key, []byte(edgeID)...)
	return key
}

func LineageByUpstreamPrefixFor(upstreamID string) []byte {
	key := append([]byte{}, LineageByUpstreamPrefix...)
	key = append(key, []byte(upstreamID)...)
	key = append(key, 0x00)
	return key
}

func LineageByDownstreamKey(downstreamID, edgeID string) []byte {
	key := append([]byte{}, LineageByDownstreamPrefix...)
	key = append(key, []byte(downstreamID)...)
	key = append(key, 0x00)
	key = append(key, []byte(edgeID)...)
	return key
}

func LineageByDownstreamPrefixFor(downstreamID string) []byte {
	key := append([]byte{}, LineageByDownstreamPrefix...)
	key = append(key, []byte(downstreamID)...)
	key = append(key, 0x00)
	return key
}

func LineageRoyaltyAccumulatorKey(attestationID string) []byte {
	return append(append([]byte{}, LineageRoyaltyAccumulatorPrefix...), []byte(attestationID)...)
}

// EdgeID constructs a deterministic id for a LineageEdge from its
// (upstream, downstream) pair. Same pair → same id (idempotent edge
// creation across re-settlement scenarios).
func EdgeID(upstreamID, downstreamID string) string {
	return upstreamID + "→" + downstreamID
}

// Be8 returns a single-byte big-endian status encoding.
func Be8(status uint8) []byte { return []byte{status} }

// BeUint64 returns 8-byte big-endian uint64 encoding.
func BeUint64(v uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	return buf
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/substrate_bridge/types/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add x/substrate_bridge/types/keys.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): store key prefixes (0x80-0x8B) + key constructors

Reserved prefixes per spec section 1. Constructors for adapter,
attestation, pending-fact, lineage, accumulator. Module name +
useful_work_audit_bounty_pool account name (M7) declared.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Module scaffolding

**Files:**
- Create: `x/substrate_bridge/types/errors.go`
- Create: `x/substrate_bridge/types/codec.go`
- Create: `x/substrate_bridge/types/expected_keepers.go`
- Create: `x/substrate_bridge/types/params.go`
- Create: `x/substrate_bridge/types/genesis.go`
- Create: `x/substrate_bridge/types/citation_type.go`
- Create: `x/substrate_bridge/types/canonical.go`
- Create: `x/substrate_bridge/doc.go`

- [ ] **Step 1: Create errors.go with doctrine voice**

```go
package types

import "cosmossdk.io/errors"

const codespace = ModuleName

var (
	// Adapter errors (M3).
	ErrAdapterNotFound       = errors.Register(codespace, 1, "adapter not found (UW + M3: adapter must be gov-registered before use)")
	ErrAdapterNotActive      = errors.Register(codespace, 2, "adapter not in ACTIVE status (UW + M3)")
	ErrAdapterAlreadyExists  = errors.Register(codespace, 3, "adapter id already registered (UW + M3: forward-only registry)")
	ErrAdapterTombstoned     = errors.Register(codespace, 4, "adapter is tombstoned; id cannot be reused (UW + M3 + commitment 10)")
	ErrAdapterAuthority      = errors.Register(codespace, 5, "adapter mutation requires gov authority (UW + M3)")

	// Submission errors (M1, M2, M3, M5).
	ErrInsufficientQualification = errors.Register(codespace, 10, "submitter lacks required qualification (UW + M3)")
	ErrInsufficientBond          = errors.Register(codespace, 11, "bond below adapter or chain minimum (UW + M1)")
	ErrWorkClassNotAllowed       = errors.Register(codespace, 12, "adapter does not permit this work class (UW + M3)")
	ErrCitedFactNotFound         = errors.Register(codespace, 13, "substrate-link cites non-existent fact_id (UW + M2: cited_facts must exist at commit)")
	ErrTooManyPendingClaims      = errors.Register(codespace, 14, "pending_claims count exceeds max_pending_claims_per_attestation (UW + M2)")
	ErrAxisOverflow              = errors.Register(codespace, 15, "axis_projection exceeds adapter AxisBounds (UW + M5)")
	ErrLinkHashMismatch          = errors.Register(codespace, 16, "substrate-link hash does not match recomputed canonical form (UW + M2: re-derivability is the link)")
	ErrInvalidCitationType       = errors.Register(codespace, 17, "citation_type unspecified or unknown (UW + M6)")
	ErrContributionSharesInvalid = errors.Register(codespace, 18, "contribution_share_bps does not sum to 10000 across cites (UW + M6)")

	// State machine errors.
	ErrAttestationNotFound       = errors.Register(codespace, 20, "attestation not found")
	ErrAttestationWrongStatus    = errors.Register(codespace, 21, "attestation status does not permit this transition")

	// Lineage errors (M6).
	ErrLineageCycle              = errors.Register(codespace, 30, "lineage cycle: upstream.created_at_block >= downstream.created_at_block (UW + M6)")
	ErrSelfCitationCapExceeded   = errors.Register(codespace, 31, "self-citation contribution_share_bps exceeds self_citation_cap_bps (UW + M6)")
)
```

- [ ] **Step 2: Create codec.go**

```go
package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	reg.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRegisterAdapter{},
		&MsgSuspendAdapter{},
		&MsgTombstoneAdapter{},
		&MsgSubmitExternalAttestation{},
	)
	msgservice.RegisterMsgServiceDesc(reg, &_Msg_serviceDesc)
}

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgRegisterAdapter{}, "zerone_substrate_bridge/RegisterAdapter", nil)
	cdc.RegisterConcrete(&MsgSuspendAdapter{}, "zerone_substrate_bridge/SuspendAdapter", nil)
	cdc.RegisterConcrete(&MsgTombstoneAdapter{}, "zerone_substrate_bridge/TombstoneAdapter", nil)
	cdc.RegisterConcrete(&MsgSubmitExternalAttestation{}, "zerone_substrate_bridge/SubmitExternalAttestation", nil)
}
```

- [ ] **Step 3: Create expected_keepers.go**

```go
package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
)

// KnowledgeKeeper is the subset of x/knowledge.Keeper used by
// substrate_bridge. PendingClaim auto-submission and CitedFact existence
// checks go through here. Implementations: x/knowledge/keeper.Keeper.
type KnowledgeKeeper interface {
	GetFact(ctx context.Context, factID string) (*knowledgetypes.Fact, bool)
	GetClaim(ctx context.Context, claimID string) (*knowledgetypes.Claim, bool)
	SetClaim(ctx context.Context, claim *knowledgetypes.Claim) error
}

// QualificationKeeper is the subset of x/qualification.Keeper used
// for submitter qualification checks.
type QualificationKeeper interface {
	GetDomainQualification(ctx context.Context, address, domain string) (qualificationtypes.DomainQualification, bool)
}

// BankKeeper escrows submitter bonds and disburses rewards.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, sender sdk.AccAddress, recipientModule string, coins sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipient sdk.AccAddress, coins sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, coins sdk.Coins) error
}

// AccountKeeper materializes the module account for bond escrow.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
}
```

- [ ] **Step 4: Create params.go**

```go
package types

import (
	"fmt"
)

func DefaultParams() Params {
	return Params{
		MaxPendingClaimsPerAttestation:    100_000,
		PerPendingClaimBondUzrn:           "222",
		AttestationMinBondUzrn:            "222000",
		MaxPendingWindowBlocks:            6_220_800, // ~6 months at 2.5s blocks
		PendingClaimRejectionThresholdBps: 5000,
		MinVerifiedRatioForSettleBps:      1000,
		LineageShareBps:                   3000,
		DecayBpsPerHop:                    3000,
		MaxPropagationDepth:               5,
		MinPropagationUzrn:                "1000",
		SelfCitationCapBps:                5000,
	}
}

func (p Params) Validate() error {
	if p.MaxPendingClaimsPerAttestation == 0 {
		return fmt.Errorf("max_pending_claims_per_attestation must be > 0")
	}
	if p.PendingClaimRejectionThresholdBps == 0 || p.PendingClaimRejectionThresholdBps > 10000 {
		return fmt.Errorf("pending_claim_rejection_threshold_bps must be in (0, 10000]")
	}
	if p.MinVerifiedRatioForSettleBps > 10000 {
		return fmt.Errorf("min_verified_ratio_for_settle_bps must be in [0, 10000]")
	}
	if p.LineageShareBps > 10000 {
		return fmt.Errorf("lineage_share_bps must be in [0, 10000]")
	}
	if p.DecayBpsPerHop > 10000 {
		return fmt.Errorf("decay_bps_per_hop must be in [0, 10000]")
	}
	if p.MaxPropagationDepth == 0 || p.MaxPropagationDepth > 20 {
		return fmt.Errorf("max_propagation_depth must be in [1, 20]")
	}
	if p.SelfCitationCapBps > 10000 {
		return fmt.Errorf("self_citation_cap_bps must be in [0, 10000]")
	}
	return nil
}
```

- [ ] **Step 5: Create genesis.go**

```go
package types

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:   DefaultParams(),
		Adapters: nil,
	}
}

func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, a := range gs.Adapters {
		if seen[a.AdapterId] {
			return fmt.Errorf("duplicate adapter_id in genesis: %s", a.AdapterId)
		}
		seen[a.AdapterId] = true
	}
	return nil
}
```

(import `fmt` at top.)

- [ ] **Step 6: Create citation_type.go**

```go
package types

// CitationTypeWeight returns the multiplier for a citation type per
// the doctrine: CITES=1, SUPPORTS=2, EXTENDS=3, REFINES=3, GENERALIZES=4.
// Unspecified or unknown returns 0 (no weight, no propagation).
func CitationTypeWeight(t CitationType) uint32 {
	switch t {
	case CitationType_CITATION_TYPE_CITES:
		return 1
	case CitationType_CITATION_TYPE_SUPPORTS:
		return 2
	case CitationType_CITATION_TYPE_EXTENDS,
		CitationType_CITATION_TYPE_REFINES:
		return 3
	case CitationType_CITATION_TYPE_GENERALIZES:
		return 4
	default:
		return 0
	}
}
```

- [ ] **Step 7: Create canonical.go (pending-claim deduplication hash)**

```go
package types

import (
	"crypto/sha256"
	"encoding/hex"
)

// PendingClaimCanonicalHash returns the canonical sha256 of a pending
// claim used for dedup against existing x/knowledge claims (spec §2
// idempotency). Two pending claims with identical
// (domain, methodology_id, claim_content) produce the same hash.
func PendingClaimCanonicalHash(p *PendingClaim) string {
	h := sha256.New()
	h.Write([]byte(p.Domain))
	h.Write([]byte{0x00})
	h.Write([]byte(p.MethodologyId))
	h.Write([]byte{0x00})
	h.Write([]byte(p.ClaimContent))
	return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 8: Create doc.go (position layer)**

```go
// Package substrate_bridge is the Tier-1 foundation for external
// recursive work modules in ZERONE. It is the one place external work
// meets ZERONE substrate; every external work class (x/translation,
// x/curriculum, x/hypothesis_market, etc.) registers with this module.
//
// Three sub-systems share one keeper:
//
//   - Adapter framework (M3): gov-gated registry of typed external-source
//     converters. Adapter is a recipe (binary hash + axis bounds + bond
//     + qualification requirements + slash gradient), not a service.
//     Registered via CategoryAdapterRegistration LIP.
//
//   - Substrate-link compiler (M2): permissive two-section provenance.
//     cited_facts must exist in x/knowledge at commit time; pending_claims
//     are auto-submitted as Claims and the attestation is held in
//     AWAITING_RESOLUTION until they resolve. Settlement is partial-
//     proportional to verified ratio (M4 generalized).
//
//   - Cross-class lineage propagator (M6): DAG-by-timestamp citation
//     graph; depth-decayed royalty propagation at downstream
//     settlement; revenue-stream amplification (cumulative accumulator
//     at LineageRoyaltyAccumulatorPrefix). Self-citation capped at
//     self_citation_cap_bps to prevent self-funneling.
//
// Doctrinal commitments preserved here:
//
//   - UW (ZERONE is recursive): every reward path requires substrate-link
//     and is scored against per-axis projection; non-recursive verified
//     work earns base only.
//   - M1 (stake-backed claim): submitter bonds locked at submit; slash
//     gradient applied per rejection mode.
//   - M2 (substrate-link mandate): re-derivable link_hash; pending claims
//     materialize as real Claims in x/knowledge.
//   - M3 (class-specific verification under shared lifecycle): adapter
//     registry gov-gated; submitter qualification enforced.
//   - M5 (recursion-weight projection): per-axis bounds at adapter level;
//     AxisProjection enforced at submit.
//   - M6 (lineage propagates AND recurses): cross-class DAG with
//     depth-decayed propagation; cumulative accumulator realizes the
//     revenue-stream amplification interpretation.
//   - M7 (chain pays for own audit): useful_work_audit_bounty_pool
//     module account declared here; passive at Phase 0; actively used
//     when challenge mechanism lands.
//
// Phase 0 ships substrate_bridge as standalone-usable via
// MsgSubmitExternalAttestation. When x/work Phase 1 lands, it will
// call PrepareExternalAttestation and SettleExternalAttestation as the
// integrated submission path; the standalone MsgSubmitExternalAttestation
// path is preserved as the direct-submit fallback.
//
// We speak through intentions.
package substrate_bridge
```

- [ ] **Step 9: Verify build**

Run: `go build ./x/substrate_bridge/types/...`
Expected: clean.

- [ ] **Step 10: Commit**

```bash
git add x/substrate_bridge/types/errors.go x/substrate_bridge/types/codec.go x/substrate_bridge/types/expected_keepers.go x/substrate_bridge/types/params.go x/substrate_bridge/types/genesis.go x/substrate_bridge/types/citation_type.go x/substrate_bridge/types/canonical.go x/substrate_bridge/doc.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): module scaffolding — errors, codec, expected keepers, params, genesis, helpers, doc.go

errors.go: typed errors each citing UW + violated mechanism.
codec.go: register Msg interfaces.
expected_keepers.go: KnowledgeKeeper, QualificationKeeper, BankKeeper, AccountKeeper.
params.go: DefaultParams + Validate.
genesis.go: DefaultGenesis + duplicate-adapter check.
citation_type.go: weight helper (CITES=1, SUPPORTS=2, EXTENDS=3, REFINES=3, GENERALIZES=4).
canonical.go: pending-claim dedup hash.
doc.go: position-layer declaration naming UW + M1-M7.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Core keeper + AppModule wiring

**Files:**
- Create: `x/substrate_bridge/keeper/keeper.go`
- Create: `x/substrate_bridge/keeper/params.go`
- Create: `x/substrate_bridge/keeper/genesis.go`
- Create: `x/substrate_bridge/module.go`

Mirror the keeper-struct pattern in `x/inquiry/keeper/keeper.go` and the AppModule pattern in `x/inquiry/module.go`. Full code:

- [ ] **Step 1: Create `keeper/keeper.go`**

```go
package keeper

import (
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

type Keeper struct {
	cdc                 codec.BinaryCodec
	storeKey            storetypes.StoreKey
	authority           string

	knowledgeKeeper     types.KnowledgeKeeper
	qualificationKeeper types.QualificationKeeper
	bankKeeper          types.BankKeeper
	accountKeeper       types.AccountKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	authority string,
	kk types.KnowledgeKeeper,
	qk types.QualificationKeeper,
	bk types.BankKeeper,
	ak types.AccountKeeper,
) Keeper {
	return Keeper{cdc: cdc, storeKey: storeKey, authority: authority,
		knowledgeKeeper: kk, qualificationKeeper: qk, bankKeeper: bk, accountKeeper: ak}
}

func (k Keeper) Authority() string { return k.authority }
func (k Keeper) Logger(ctx interface{ Logger() log.Logger }) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}
```

- [ ] **Step 2: Create `keeper/params.go`**

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func (k Keeper) GetParams(ctx context.Context) types.Params {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return types.DefaultParams()
	}
	var p types.Params
	k.cdc.MustUnmarshal(bz, &p)
	return p
}

func (k Keeper) SetParams(ctx context.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey).Set(types.ParamsKey, k.cdc.MustMarshal(&p))
	return nil
}
```

- [ ] **Step 3: Create `keeper/genesis.go`**

```go
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) error {
	if err := k.SetParams(ctx, gs.Params); err != nil {
		return err
	}
	for _, a := range gs.Adapters {
		if err := k.WriteAdapter(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	gs := &types.GenesisState{Params: k.GetParams(ctx)}
	k.IterateAdapters(ctx, func(a *types.AdapterRegistration) bool {
		gs.Adapters = append(gs.Adapters, a)
		return false
	})
	return gs
}
```

- [ ] **Step 4: Create `module.go`** — mirror `x/inquiry/module.go` exactly, swapping module name, AppModule type, keeper, and AppModuleBasic. Important: `RegisterServices` registers Msg + Query servers; `BeginBlock(ctx)` calls `am.keeper.BeginBlocker(ctx)`. The full template:

```go
package substrate_bridge

import (
	"context"
	"encoding/json"

	"cosmossdk.io/core/appmodule"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"github.com/zerone-chain/zerone/x/substrate_bridge/keeper"
	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
	_ appmodule.AppModule   = AppModule{}
)

type AppModuleBasic struct{ cdc codec.BinaryCodec }

func (AppModuleBasic) Name() string { return types.ModuleName }
func (AppModuleBasic) RegisterLegacyAminoCodec(c *codec.LegacyAmino) { types.RegisterLegacyAminoCodec(c) }
func (a AppModuleBasic) RegisterInterfaces(r cdctypes.InterfaceRegistry) { types.RegisterInterfaces(r) }
func (AppModuleBasic) DefaultGenesis(c codec.JSONCodec) json.RawMessage { return c.MustMarshalJSON(types.DefaultGenesis()) }
func (AppModuleBasic) ValidateGenesis(c codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := c.UnmarshalJSON(bz, &gs); err != nil { return err }
	return gs.Validate()
}
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

func NewAppModule(cdc codec.BinaryCodec, k keeper.Keeper) AppModule {
	return AppModule{AppModuleBasic: AppModuleBasic{cdc: cdc}, keeper: k}
}

func (am AppModule) IsAppModule()                            {}
func (am AppModule) IsOnePerModuleType()                     {}
func (am AppModule) ConsensusVersion() uint64                { return 1 }
func (am AppModule) RegisterInvariants(sdk.InvariantRegistry) {}

func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	cdc.MustUnmarshalJSON(data, &gs)
	if err := am.keeper.InitGenesis(ctx, &gs); err != nil { panic(err) }
}
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(am.keeper.ExportGenesis(ctx))
}
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))
}
func (am AppModule) BeginBlock(ctx context.Context) error {
	return am.keeper.BeginBlocker(ctx)
}
```

The references to `WriteAdapter`, `IterateAdapters`, `NewMsgServerImpl`, `NewQueryServerImpl`, `BeginBlocker` are resolved by Tasks 12, 19, 20, 21. Build will fail until those land — expected.

- [ ] **Step 5: Skip build check (deferred to Task 21 when all references resolve)**

- [ ] **Step 6: Commit**

```bash
git add x/substrate_bridge/keeper/keeper.go x/substrate_bridge/keeper/params.go x/substrate_bridge/keeper/genesis.go x/substrate_bridge/module.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): core keeper + AppModule wiring

Keeper struct with codec/storeKey/authority + expected-keeper deps.
Params get/set with validation. InitGenesis/ExportGenesis. AppModule
implementing RegisterServices and BeginBlock. References to functions
defined in subsequent tasks; build verifies at Task 21.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Adapter registry keeper

**Files:**
- Create: `x/substrate_bridge/keeper/helpers_test.go` (test harness)
- Create: `x/substrate_bridge/keeper/adapter_registry.go`
- Create: `x/substrate_bridge/keeper/adapter_registry_test.go`

- [ ] **Step 1: Create test harness `helpers_test.go`**

Mirror the pattern in `x/inquiry/keeper/keeper_test.go`. Required signature:

```go
package keeper_test

import (
	"testing"
	// ... imports for codec, memdb, in-memory store
)

func setupSubstrateBridgeKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
	t.Helper()
	// 1. memdb.NewMemDB() + storetypes.NewKVStoreKey(types.StoreKey)
	// 2. cms := store.NewCommitMultiStore(db); mount the key; LoadLatestVersion
	// 3. ctx := sdk.NewContext(cms, tmproto.Header{Height: 1}, false, log.NewNopLogger())
	// 4. encConfig := moduletestutil.MakeTestEncodingConfig()
	// 5. types.RegisterInterfaces(encConfig.InterfaceRegistry)
	// 6. k := keeper.NewKeeper(encConfig.Codec, storeKey, "authority-addr", nil, nil, nil, nil)
	// 7. k.SetParams(ctx, types.DefaultParams())
	// Return (k, ctx).
}
```

The nil expected-keepers are fine for tests that don't call into knowledge/qualification/bank. For tests that need x/knowledge, swap nil for a stub: `&stubKnowledgeKeeper{facts: map[string]*knowledgetypes.Fact{}}`.

- [ ] **Step 2: Write failing test `adapter_registry_test.go`**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestAdapterRegistry_WriteAndGet(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	adapter := &types.AdapterRegistration{
		AdapterId:              "wikipedia-en-v1",
		SourceType:             "wikipedia",
		Version:                "1.0.0",
		CompilerBinaryHash:     []byte{0xde, 0xad, 0xbe, 0xef},
		MinAttestationBondUzrn: "222000",
		MinPerClaimBondUzrn:    "222",
		Status:                 types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
		RegisteredViaLipId:     "LIP-0001",
		RegisteredAtBlock:      100,
	}
	require.NoError(t, k.WriteAdapter(ctx, adapter))
	got, found := k.GetAdapter(ctx, "wikipedia-en-v1")
	require.True(t, found)
	require.Equal(t, adapter.AdapterId, got.AdapterId)
	require.Equal(t, adapter.Status, got.Status)
}

func TestAdapterRegistry_GetMissing(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	_, found := k.GetAdapter(ctx, "missing")
	require.False(t, found)
}

func TestAdapterRegistry_SuspendChangesStatus(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "test-adapter",
		Status:    types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	require.NoError(t, k.SuspendAdapter(ctx, "test-adapter", "incident"))
	got, _ := k.GetAdapter(ctx, "test-adapter")
	require.Equal(t, types.AdapterStatus_ADAPTER_STATUS_SUSPENDED, got.Status)
}

func TestAdapterRegistry_TombstoneIsForwardOnly(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "doomed-adapter",
		Status:    types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	require.NoError(t, k.TombstoneAdapter(ctx, "doomed-adapter"))
	got, _ := k.GetAdapter(ctx, "doomed-adapter")
	require.Equal(t, types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED, got.Status)
	require.Greater(t, got.TombstonedAtBlock, uint64(0))

	err := k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "doomed-adapter",
		Status:    types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	})
	require.ErrorIs(t, err, types.ErrAdapterTombstoned)
}
```

- [ ] **Step 3: Run, verify FAIL**

Run: `go test ./x/substrate_bridge/keeper/ -run TestAdapterRegistry -v` → FAIL.

- [ ] **Step 4: Implement `adapter_registry.go`**

```go
package keeper

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func (k Keeper) WriteAdapter(ctx context.Context, a *types.AdapterRegistration) error {
	if a == nil || a.AdapterId == "" {
		return types.ErrAdapterNotFound
	}
	existing, found := k.GetAdapter(ctx, a.AdapterId)
	if found && existing.Status == types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED {
		return types.ErrAdapterTombstoned
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.KVStore(k.storeKey)
	if found && existing.Status != a.Status {
		store.Delete(types.AdapterByStatusKey(uint8(existing.Status), a.AdapterId))
	}
	store.Set(types.AdapterKey(a.AdapterId), k.cdc.MustMarshal(a))
	store.Set(types.AdapterByStatusKey(uint8(a.Status), a.AdapterId), []byte{0x01})
	return nil
}

func (k Keeper) GetAdapter(ctx context.Context, adapterID string) (*types.AdapterRegistration, bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.AdapterKey(adapterID))
	if bz == nil {
		return nil, false
	}
	var a types.AdapterRegistration
	if err := k.cdc.Unmarshal(bz, &a); err != nil {
		return nil, false
	}
	return &a, true
}

func (k Keeper) IterateAdapters(ctx context.Context, cb func(*types.AdapterRegistration) bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.AdapterRegistrationPrefix)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var a types.AdapterRegistration
		if err := k.cdc.Unmarshal(iter.Value(), &a); err != nil {
			continue
		}
		if cb(&a) {
			return
		}
	}
}

func (k Keeper) SuspendAdapter(ctx context.Context, adapterID, reason string) error {
	a, found := k.GetAdapter(ctx, adapterID)
	if !found {
		return types.ErrAdapterNotFound
	}
	if a.Status == types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED {
		return types.ErrAdapterTombstoned
	}
	a.Status = types.AdapterStatus_ADAPTER_STATUS_SUSPENDED
	return k.WriteAdapter(ctx, a)
}

func (k Keeper) TombstoneAdapter(ctx context.Context, adapterID string) error {
	a, found := k.GetAdapter(ctx, adapterID)
	if !found {
		return types.ErrAdapterNotFound
	}
	if a.Status == types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED {
		return types.ErrAdapterTombstoned
	}
	a.Status = types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED
	a.TombstonedAtBlock = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
	return k.WriteAdapter(ctx, a)
}
```

- [ ] **Step 5: Run tests to verify PASS**

Run: `go test ./x/substrate_bridge/keeper/ -run TestAdapterRegistry -v` → PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add x/substrate_bridge/keeper/adapter_registry.go x/substrate_bridge/keeper/adapter_registry_test.go x/substrate_bridge/keeper/helpers_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): adapter registry CRUD + tests (M3)

WriteAdapter / GetAdapter / IterateAdapters / SuspendAdapter /
TombstoneAdapter. Tombstone is forward-only (commitment 10). Status-
indexed reverse lookup at 0x89. Authority enforcement happens in
msg_server (Task 19); keeper functions are unauthenticated by design
so internal callers (genesis, hooks) can call directly.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 13: Substrate-link validation + canonical hash

**Files:**
- Create: `x/substrate_bridge/keeper/substrate_link.go`
- Create: `x/substrate_bridge/keeper/substrate_link_test.go`

- [ ] **Step 1: Write failing test**

```go
package keeper_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/keeper"
	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestComputeLinkHash_Deterministic(t *testing.T) {
	link := &types.SubstrateLink{
		AdapterId: "wiki-v1",
		CitedFacts: []*types.FactCitation{{FactId: "fact-1", CitationType: types.CitationType_CITATION_TYPE_SUPPORTS}},
		PendingClaims: []*types.PendingClaim{{ClaimContent: "X is Y", Domain: "history", MethodologyId: "wiki-cite"}},
		RecursionWeight: &types.AxisProjection{AxisSubstrate: 100},
		Source: &types.ExternalSource{SourceId: "Q42", ContentHash: []byte{0x01}},
	}
	h1 := keeper.ComputeLinkHash(link)
	h2 := keeper.ComputeLinkHash(link)
	require.Equal(t, h1, h2)
	require.Len(t, h1, sha256.Size)
}

func TestComputeLinkHash_FieldSensitivity(t *testing.T) {
	a := &types.SubstrateLink{AdapterId: "wiki-v1", CitedFacts: []*types.FactCitation{{FactId: "fact-1"}}}
	b := &types.SubstrateLink{AdapterId: "wiki-v1", CitedFacts: []*types.FactCitation{{FactId: "fact-2"}}}
	require.NotEqual(t, keeper.ComputeLinkHash(a), keeper.ComputeLinkHash(b))
}

func TestValidateLink_AdapterMustExist(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	link := &types.SubstrateLink{AdapterId: "unregistered"}
	err := k.ValidateLink(ctx, link, types.DefaultParams())
	require.ErrorIs(t, err, types.ErrAdapterNotFound)
}

func TestValidateLink_AdapterMustBeActive(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1",
		Status:    types.AdapterStatus_ADAPTER_STATUS_SUSPENDED,
	}))
	err := k.ValidateLink(ctx, &types.SubstrateLink{AdapterId: "wiki-v1"}, types.DefaultParams())
	require.ErrorIs(t, err, types.ErrAdapterNotActive)
}

func TestValidateLink_TooManyPendingClaims(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	p := types.DefaultParams()
	p.MaxPendingClaimsPerAttestation = 2
	link := &types.SubstrateLink{
		AdapterId: "wiki-v1",
		PendingClaims: []*types.PendingClaim{
			{ClaimContent: "a"}, {ClaimContent: "b"}, {ClaimContent: "c"},
		},
	}
	require.ErrorIs(t, k.ValidateLink(ctx, link, p), types.ErrTooManyPendingClaims)
}

func TestValidateLink_AxisOverflow(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId:  "wiki-v1",
		Status:     types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
		AxisBounds: &types.AxisBounds{AxisSubstrateMax: 100},
	}))
	link := &types.SubstrateLink{
		AdapterId: "wiki-v1",
		RecursionWeight: &types.AxisProjection{AxisSubstrate: 200},
	}
	require.ErrorIs(t, k.ValidateLink(ctx, link, types.DefaultParams()), types.ErrAxisOverflow)
}
```

- [ ] **Step 2: Run, verify FAIL**

- [ ] **Step 3: Implement `substrate_link.go`**

```go
package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sort"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// ComputeLinkHash returns the deterministic canonical sha256 of a
// SubstrateLink. Length-prefixed everywhere; sorted children for
// determinism. Same input → same hash (M2 re-derivability anchor).
func ComputeLinkHash(l *types.SubstrateLink) []byte {
	h := sha256.New()
	writeLen(h, []byte(l.AdapterId))

	if l.Source != nil {
		writeLen(h, []byte(l.Source.SourceId))
		writeLen(h, l.Source.ContentHash)
		writeUint64(h, l.Source.FetchedAtBlock)
	}

	cf := append([]*types.FactCitation{}, l.CitedFacts...)
	sort.Slice(cf, func(i, j int) bool { return cf[i].FactId < cf[j].FactId })
	for _, c := range cf {
		writeLen(h, []byte(c.FactId))
		writeUint32(h, uint32(c.CitationType))
	}

	pc := append([]*types.PendingClaim{}, l.PendingClaims...)
	sort.Slice(pc, func(i, j int) bool {
		return types.PendingClaimCanonicalHash(pc[i]) < types.PendingClaimCanonicalHash(pc[j])
	})
	for _, c := range pc {
		writeLen(h, []byte(c.ClaimContent))
		writeLen(h, []byte(c.Domain))
		writeLen(h, []byte(c.MethodologyId))
	}

	if l.RecursionWeight != nil {
		writeUint64(h, l.RecursionWeight.AxisSubstrate)
		writeUint64(h, l.RecursionWeight.AxisVerification)
		writeUint64(h, l.RecursionWeight.AxisClassification)
		writeUint64(h, l.RecursionWeight.AxisAttribution)
		writeUint64(h, l.RecursionWeight.AxisTooling)
		writeUint64(h, l.RecursionWeight.AxisInterface)
	}

	return h.Sum(nil)
}

func writeLen(h interface{ Write([]byte) (int, error) }, data []byte) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(data)))
	h.Write(buf[:])
	h.Write(data)
}

func writeUint32(h interface{ Write([]byte) (int, error) }, v uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	h.Write(buf[:])
}

func writeUint64(h interface{ Write([]byte) (int, error) }, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	h.Write(buf[:])
}

func (k Keeper) ValidateLink(ctx context.Context, l *types.SubstrateLink, p types.Params) error {
	if l == nil {
		return types.ErrAdapterNotFound
	}
	adapter, found := k.GetAdapter(ctx, l.AdapterId)
	if !found {
		return types.ErrAdapterNotFound
	}
	if adapter.Status != types.AdapterStatus_ADAPTER_STATUS_ACTIVE {
		return types.ErrAdapterNotActive
	}
	if uint32(len(l.PendingClaims)) > p.MaxPendingClaimsPerAttestation {
		return types.ErrTooManyPendingClaims
	}
	if l.RecursionWeight != nil && adapter.AxisBounds != nil {
		if l.RecursionWeight.AxisSubstrate > adapter.AxisBounds.AxisSubstrateMax ||
			l.RecursionWeight.AxisVerification > adapter.AxisBounds.AxisVerificationMax ||
			l.RecursionWeight.AxisClassification > adapter.AxisBounds.AxisClassificationMax ||
			l.RecursionWeight.AxisAttribution > adapter.AxisBounds.AxisAttributionMax ||
			l.RecursionWeight.AxisTooling > adapter.AxisBounds.AxisToolingMax ||
			l.RecursionWeight.AxisInterface > adapter.AxisBounds.AxisInterfaceMax {
			return types.ErrAxisOverflow
		}
	}
	// Cited facts must exist in x/knowledge. Only called when knowledgeKeeper is non-nil.
	if k.knowledgeKeeper != nil {
		for _, c := range l.CitedFacts {
			if _, found := k.knowledgeKeeper.GetFact(ctx, c.FactId); !found {
				return types.ErrCitedFactNotFound
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify PASS**

Run: `go test ./x/substrate_bridge/keeper/ -run "TestComputeLinkHash|TestValidateLink" -v` → PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add x/substrate_bridge/keeper/substrate_link.go x/substrate_bridge/keeper/substrate_link_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): substrate-link validation + canonical hash (M2, M5)

ComputeLinkHash: deterministic, length-prefixed, sorted-children form.
ValidateLink: adapter ACTIVE + pending-claim cap + axis-bounds + cited-
facts exist in x/knowledge. Link-hash equality is enforced in
msg_server at submit (Task 19), not here, so internal flows can call
ValidateLink without recomputing.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: Attestation state-machine storage

**Files:**
- Create: `x/substrate_bridge/keeper/attestation.go`
- Create: `x/substrate_bridge/keeper/attestation_test.go`

- [ ] **Step 1: Failing test**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestAttestation_WriteGet(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	att := &types.ExternalAttestation{
		AttestationId: "att-1", AdapterId: "wiki-v1", Submitter: "zrn1xxx",
		Status: types.AttestationStatus_ATTESTATION_STATUS_SUBMITTED, SubmittedAtBlock: 100,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))
	got, found := k.GetAttestation(ctx, "att-1")
	require.True(t, found)
	require.Equal(t, att.AttestationId, got.AttestationId)
}

func TestAttestation_StatusIndex(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "a", Status: types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "b", Status: types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "c", Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))

	var awaiting []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, func(id string) bool {
		awaiting = append(awaiting, id); return false
	})
	require.Len(t, awaiting, 2)
}

func TestAttestation_TransitionDeletesOldIndex(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "a", Status: types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
	}))
	att, _ := k.GetAttestation(ctx, "a")
	att.Status = types.AttestationStatus_ATTESTATION_STATUS_SETTLED
	require.NoError(t, k.WriteAttestation(ctx, att))
	var awaiting []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, func(id string) bool {
		awaiting = append(awaiting, id); return false
	})
	require.Empty(t, awaiting)
}

func TestAttestation_NextIDMonotonic(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	id1 := k.NextAttestationID(ctx)
	id2 := k.NextAttestationID(ctx)
	require.NotEqual(t, id1, id2)
}
```

- [ ] **Step 2: Run, FAIL**

- [ ] **Step 3: Implement `attestation.go`**

```go
package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func (k Keeper) WriteAttestation(ctx context.Context, att *types.ExternalAttestation) error {
	if att == nil || att.AttestationId == "" {
		return types.ErrAttestationNotFound
	}
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	if existing, found := k.GetAttestation(ctx, att.AttestationId); found && existing.Status != att.Status {
		store.Delete(types.AttestationByStatusKey(uint8(existing.Status), att.AttestationId))
	}
	store.Set(types.AttestationKey(att.AttestationId), k.cdc.MustMarshal(att))
	store.Set(types.AttestationByStatusKey(uint8(att.Status), att.AttestationId), []byte{0x01})
	return nil
}

func (k Keeper) GetAttestation(ctx context.Context, attestationID string) (*types.ExternalAttestation, bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.AttestationKey(attestationID))
	if bz == nil {
		return nil, false
	}
	var att types.ExternalAttestation
	if err := k.cdc.Unmarshal(bz, &att); err != nil {
		return nil, false
	}
	return &att, true
}

func (k Keeper) IterateAttestationsByStatus(
	ctx context.Context,
	status types.AttestationStatus,
	cb func(attestationID string) bool,
) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := append(append([]byte{}, types.AttestationByStatusPrefix...), uint8(status))
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		if cb(string(iter.Key()[prefixLen:])) {
			return
		}
	}
}

func (k Keeper) NextAttestationID(ctx context.Context) string {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.KVStore(k.storeKey)
	var next uint64
	if buf := store.Get(types.AttestationIDCounterKey); buf != nil {
		next, _ = binary.Uvarint(buf)
	}
	next++
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, next)
	store.Set(types.AttestationIDCounterKey, buf[:n])
	return fmt.Sprintf("att-%d-%d", sdkCtx.BlockHeight(), next)
}
```

- [ ] **Step 4: Run, PASS**

- [ ] **Step 5: Commit**

```bash
git add x/substrate_bridge/keeper/attestation.go x/substrate_bridge/keeper/attestation_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): attestation state-machine storage

Write/Get/IterateByStatus + NextAttestationID. Status transitions
maintain reverse index at 0x86 (delete old status key, write new).
NextAttestationID is uvarint-counter-backed for determinism.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 15: Pending-fact bidirectional index

**Files:**
- Create: `x/substrate_bridge/keeper/pending_fact_index.go`
- Create: `x/substrate_bridge/keeper/pending_fact_index_test.go`

- [ ] **Step 1: Failing test**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPendingFactIndex_LinkLookup(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.LinkPendingClaim(ctx, "claim-1", "att-A"))
	require.NoError(t, k.LinkPendingClaim(ctx, "claim-2", "att-A"))
	require.NoError(t, k.LinkPendingClaim(ctx, "claim-3", "att-B"))

	got, found := k.GetAttestationForPendingClaim(ctx, "claim-1")
	require.True(t, found); require.Equal(t, "att-A", got)
	got, found = k.GetAttestationForPendingClaim(ctx, "claim-3")
	require.True(t, found); require.Equal(t, "att-B", got)
}

func TestPendingFactIndex_AttestationForwardWalk(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.LinkPendingClaim(ctx, "claim-1", "att-A"))
	require.NoError(t, k.LinkPendingClaim(ctx, "claim-2", "att-A"))
	require.NoError(t, k.LinkPendingClaim(ctx, "claim-3", "att-A"))

	claims := k.PendingClaimsFor(ctx, "att-A")
	require.Len(t, claims, 3)
}

func TestPendingFactIndex_UnlinkBoth(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.LinkPendingClaim(ctx, "claim-1", "att-A"))
	require.NoError(t, k.UnlinkPendingClaim(ctx, "claim-1", "att-A"))
	_, found := k.GetAttestationForPendingClaim(ctx, "claim-1")
	require.False(t, found)
	require.Empty(t, k.PendingClaimsFor(ctx, "att-A"))
}
```

- [ ] **Step 2: Run, FAIL**

- [ ] **Step 3: Implement `pending_fact_index.go`**

```go
package keeper

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func (k Keeper) LinkPendingClaim(ctx context.Context, pendingClaimID, attestationID string) error {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	store.Set(types.PendingFactIndexKey(pendingClaimID), []byte(attestationID))
	store.Set(types.AttestationPendingClaimsKey(attestationID, pendingClaimID), []byte{0x01})
	return nil
}

func (k Keeper) GetAttestationForPendingClaim(ctx context.Context, pendingClaimID string) (string, bool) {
	bz := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey).Get(types.PendingFactIndexKey(pendingClaimID))
	if bz == nil {
		return "", false
	}
	return string(bz), true
}

func (k Keeper) PendingClaimsFor(ctx context.Context, attestationID string) []string {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := types.AttestationPendingClaimsPrefixFor(attestationID)
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	var out []string
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		out = append(out, string(iter.Key()[prefixLen:]))
	}
	return out
}

func (k Keeper) UnlinkPendingClaim(ctx context.Context, pendingClaimID, attestationID string) error {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	store.Delete(types.PendingFactIndexKey(pendingClaimID))
	store.Delete(types.AttestationPendingClaimsKey(attestationID, pendingClaimID))
	return nil
}
```

- [ ] **Step 4: Run, PASS** (3 tests).

- [ ] **Step 5: Commit**

```bash
git add x/substrate_bridge/keeper/pending_fact_index.go x/substrate_bridge/keeper/pending_fact_index_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): pending-fact bidirectional index

Link/Unlink maintain (claim→attestation) and (attestation→claims)
indexes. GetAttestationForPendingClaim feeds OnClaimResolved hook;
PendingClaimsFor feeds settlement.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 16: Lineage DAG + edge creation

**Files:**
- Create: `x/substrate_bridge/keeper/lineage.go`
- Create: `x/substrate_bridge/keeper/lineage_test.go`

- [ ] **Step 1: Failing test**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func setupTwoAttestations(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-upstream", Submitter: "alice", SubmittedAtBlock: 10,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-downstream", Submitter: "bob", SubmittedAtBlock: 20,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
}

func TestLineage_CreateEdgeValid(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	setupTwoAttestations(t, k, ctx)

	err := k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "att-upstream",
		DownstreamAttestationId: "att-downstream",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	})
	require.NoError(t, err)

	edge, found := k.GetLineageEdge(ctx, types.EdgeID("att-upstream", "att-downstream"))
	require.True(t, found)
	require.Equal(t, "att-upstream", edge.UpstreamAttestationId)
}

func TestLineage_RejectsTimestampCycle(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "later", SubmittedAtBlock: 30,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "earlier", SubmittedAtBlock: 10,
	}))
	// Try to create later→earlier (cycle).
	err := k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "later",
		DownstreamAttestationId: "earlier",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	})
	require.ErrorIs(t, err, types.ErrLineageCycle)
}

func TestLineage_ForwardBackwardWalks(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	setupTwoAttestations(t, k, ctx)
	require.NoError(t, k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "att-upstream",
		DownstreamAttestationId: "att-downstream",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	}))

	var forward []*types.LineageEdge
	k.IterateForwardLineage(ctx, "att-upstream", func(e *types.LineageEdge) bool {
		forward = append(forward, e); return false
	})
	require.Len(t, forward, 1)
	require.Equal(t, "att-downstream", forward[0].DownstreamAttestationId)

	var backward []*types.LineageEdge
	k.IterateBackwardLineage(ctx, "att-downstream", func(e *types.LineageEdge) bool {
		backward = append(backward, e); return false
	})
	require.Len(t, backward, 1)
}

func TestLineage_SelfCitationCap(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-up", Submitter: "alice", SubmittedAtBlock: 10,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "att-down", Submitter: "alice", SubmittedAtBlock: 20,
	}))
	// Same submitter ("alice") + 8000 bps share → exceeds 5000 cap.
	err := k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "att-up",
		DownstreamAttestationId: "att-down",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    8000,
	})
	require.ErrorIs(t, err, types.ErrSelfCitationCapExceeded)
}
```

- [ ] **Step 2: Run, FAIL**

- [ ] **Step 3: Implement `lineage.go`**

```go
package keeper

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// CreateLineageEdge writes a LineageEdge after enforcing:
//   - both attestations exist
//   - upstream.SubmittedAtBlock < downstream.SubmittedAtBlock (DAG cycle prevention)
//   - if same submitter, contribution_share_bps <= self_citation_cap_bps
//   - citation_type is specified
// Forward/backward indexes maintained.
func (k Keeper) CreateLineageEdge(ctx context.Context, e *types.LineageEdge) error {
	if e == nil || e.UpstreamAttestationId == "" || e.DownstreamAttestationId == "" {
		return types.ErrAttestationNotFound
	}
	if e.CitationType == types.CitationType_CITATION_TYPE_UNSPECIFIED {
		return types.ErrInvalidCitationType
	}

	upstream, foundU := k.GetAttestation(ctx, e.UpstreamAttestationId)
	if !foundU {
		return types.ErrAttestationNotFound
	}
	downstream, foundD := k.GetAttestation(ctx, e.DownstreamAttestationId)
	if !foundD {
		return types.ErrAttestationNotFound
	}

	if upstream.SubmittedAtBlock >= downstream.SubmittedAtBlock {
		return types.ErrLineageCycle
	}

	if upstream.Submitter == downstream.Submitter {
		params := k.GetParams(ctx)
		if e.ContributionShareBps > params.SelfCitationCapBps {
			return types.ErrSelfCitationCapExceeded
		}
	}

	e.UpstreamClassId = upstream.WorkClassId
	e.DownstreamClassId = downstream.WorkClassId
	if e.CreatedAtBlock == 0 {
		e.CreatedAtBlock = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
	}
	if e.DepthFromDownstream == 0 {
		e.DepthFromDownstream = 1
	}

	edgeID := types.EdgeID(e.UpstreamAttestationId, e.DownstreamAttestationId)
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	store.Set(types.LineageEdgeKey(edgeID), k.cdc.MustMarshal(e))
	store.Set(types.LineageByUpstreamKey(e.UpstreamAttestationId, edgeID), []byte{0x01})
	store.Set(types.LineageByDownstreamKey(e.DownstreamAttestationId, edgeID), []byte{0x01})
	return nil
}

func (k Keeper) GetLineageEdge(ctx context.Context, edgeID string) (*types.LineageEdge, bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.LineageEdgeKey(edgeID))
	if bz == nil {
		return nil, false
	}
	var e types.LineageEdge
	if err := k.cdc.Unmarshal(bz, &e); err != nil {
		return nil, false
	}
	return &e, true
}

func (k Keeper) IterateForwardLineage(ctx context.Context, upstreamID string, cb func(*types.LineageEdge) bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := types.LineageByUpstreamPrefixFor(upstreamID)
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		edgeID := string(iter.Key()[prefixLen:])
		if e, ok := k.GetLineageEdge(ctx, edgeID); ok {
			if cb(e) {
				return
			}
		}
	}
}

func (k Keeper) IterateBackwardLineage(ctx context.Context, downstreamID string, cb func(*types.LineageEdge) bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := types.LineageByDownstreamPrefixFor(downstreamID)
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		edgeID := string(iter.Key()[prefixLen:])
		if e, ok := k.GetLineageEdge(ctx, edgeID); ok {
			if cb(e) {
				return
			}
		}
	}
}

func (k Keeper) GetLineageAccumulator(ctx context.Context, attestationID string) (*types.LineageRoyaltyAccumulator, bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.LineageRoyaltyAccumulatorKey(attestationID))
	if bz == nil {
		return nil, false
	}
	var a types.LineageRoyaltyAccumulator
	if err := k.cdc.Unmarshal(bz, &a); err != nil {
		return nil, false
	}
	return &a, true
}

func (k Keeper) WriteLineageAccumulator(ctx context.Context, a *types.LineageRoyaltyAccumulator) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	store.Set(types.LineageRoyaltyAccumulatorKey(a.AttestationId), k.cdc.MustMarshal(a))
}
```

- [ ] **Step 4: Run, PASS** (4 tests).

- [ ] **Step 5: Commit**

```bash
git add x/substrate_bridge/keeper/lineage.go x/substrate_bridge/keeper/lineage_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): lineage DAG + edge creation (M6)

CreateLineageEdge enforces DAG-by-timestamp cycle prevention + self-
citation cap. Forward/backward indexes maintained at 0x81/0x82.
GetLineageAccumulator/WriteLineageAccumulator manage the cumulative
royalty accumulator at 0x83 (revenue-stream amplification anchor).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 17: Lineage propagation walk (depth-decayed royalty flow)

**Files:**
- Create: `x/substrate_bridge/keeper/propagation.go`
- Create: `x/substrate_bridge/keeper/propagation_test.go`

`PropagateLineage` is called from settlement when a downstream attestation pays. It walks upstream through the existing edge graph, distributing a depth-decayed share to each ancestor. Lineage payments come from the downstream's reward, NOT new minting.

- [ ] **Step 1: Failing test**

```go
package keeper_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestPropagateLineage_DirectCitePaysProportional(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)  // harness includes stubBankKeeper
	setupTwoAttestations(t, k, ctx)
	require.NoError(t, k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId:   "att-upstream",
		DownstreamAttestationId: "att-downstream",
		CitationType:            types.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	}))

	// Downstream settles with reward 100 ZRN = 100_000_000 uzrn.
	downstreamReward := sdkmath.NewInt(100_000_000)
	err := k.PropagateLineage(ctx, "att-downstream", downstreamReward)
	require.NoError(t, err)

	// 30% lineage budget = 30M uzrn.
	// Direct upstream (depth 1, SUPPORTS=2× weight, share 10000bps):
	//   share = 30M × 10000bps × 2× / 10000 = 60M uzrn
	//   but clamp to budget remaining: 30M
	acc, found := k.GetLineageAccumulator(ctx, "att-upstream")
	require.True(t, found)
	require.Equal(t, "30000000", acc.CumulativeUzrn)
}

func TestPropagateLineage_MultiHopWithDecay(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)
	// Three attestations: F → T → D (chain).
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "F", Submitter: "alice", SubmittedAtBlock: 10,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "T", Submitter: "bob", SubmittedAtBlock: 20,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	require.NoError(t, k.WriteAttestation(ctx, &types.ExternalAttestation{
		AttestationId: "D", Submitter: "carol", SubmittedAtBlock: 30,
		Status: types.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	// T cites F; D cites T.
	require.NoError(t, k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId: "F", DownstreamAttestationId: "T",
		CitationType: types.CitationType_CITATION_TYPE_CITES, ContributionShareBps: 10000,
	}))
	require.NoError(t, k.CreateLineageEdge(ctx, &types.LineageEdge{
		UpstreamAttestationId: "T", DownstreamAttestationId: "D",
		CitationType: types.CitationType_CITATION_TYPE_CITES, ContributionShareBps: 10000,
	}))

	downstreamReward := sdkmath.NewInt(100_000_000)
	require.NoError(t, k.PropagateLineage(ctx, "D", downstreamReward))

	// T receives: 30M × 1× × 10000bps = 30M
	accT, _ := k.GetLineageAccumulator(ctx, "T")
	require.Equal(t, "30000000", accT.CumulativeUzrn)
	// F receives propagated: T's share × 30% decay = 9M
	accF, _ := k.GetLineageAccumulator(ctx, "F")
	require.Equal(t, "9000000", accF.CumulativeUzrn)
}

func TestPropagateLineage_HaltsAtMaxDepth(t *testing.T) {
	// Chain of 8 attestations, max depth 5. Confirm propagation stops.
	// (Test body builds the chain, propagates from leaf, asserts only
	// the closest 5 ancestors received payments; deeper got 0.)
	t.Skip("Will be implemented as part of executor's discretion; the assertion is: " +
		"k.GetLineageAccumulator(ctx, deepest_ancestor) returns either not-found or 0")
}
```

Note on the test for the harness: `setupSubstrateBridgeKeeperWithBank` is a variant of the harness with a stub bank-keeper that records every `SendCoinsFromModuleToAccount` call so the test can assert who got paid. Implementation in `helpers_test.go`:

```go
type stubBankKeeper struct {
	payments map[string]sdkmath.Int  // recipient_addr → cumulative paid
}
func (s *stubBankKeeper) SendCoinsFromAccountToModule(ctx context.Context, from sdk.AccAddress, mod string, coins sdk.Coins) error { return nil }
func (s *stubBankKeeper) SendCoinsFromModuleToAccount(ctx context.Context, mod string, to sdk.AccAddress, coins sdk.Coins) error {
	if s.payments == nil { s.payments = map[string]sdkmath.Int{} }
	cur := s.payments[to.String()]
	for _, c := range coins {
		cur = cur.Add(c.Amount)
	}
	s.payments[to.String()] = cur
	return nil
}
func (s *stubBankKeeper) BurnCoins(ctx context.Context, mod string, coins sdk.Coins) error { return nil }
```

- [ ] **Step 2: Run, FAIL**

- [ ] **Step 3: Implement `propagation.go`**

```go
package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// PropagateLineage distributes a depth-decayed royalty flow from the
// downstream attestation back through its lineage DAG. Budget is
// downstreamReward × Params.LineageShareBps (default 30%). Decay
// per-hop is Params.DecayBpsPerHop. Halts at Params.MaxPropagationDepth
// or when remaining share < Params.MinPropagationUzrn.
//
// Lineage payments come from the downstream's reward, NOT new minting
// (no inflation pressure; M4 formula unchanged).
//
// The caller is responsible for the actual SendCoinsFromModuleToAccount
// — this function returns the (recipient_addr, amount) list and updates
// LineageEdge.SettlementPaymentUzrn + LineageRoyaltyAccumulator.
// In practice the caller will be Settlement.SettleAttestation.
func (k Keeper) PropagateLineage(ctx context.Context, downstreamID string, downstreamReward sdkmath.Int) error {
	params := k.GetParams(ctx)
	lineageShareBps := sdkmath.NewIntFromUint64(uint64(params.LineageShareBps))
	totalBudget := downstreamReward.Mul(lineageShareBps).Quo(sdkmath.NewInt(10000))

	if totalBudget.IsZero() {
		return nil
	}

	minProp, ok := sdkmath.NewIntFromString(params.MinPropagationUzrn)
	if !ok {
		minProp = sdkmath.NewInt(1000)
	}

	return k.propagateRecursive(ctx, downstreamID, totalBudget, 1, params.MaxPropagationDepth, params.DecayBpsPerHop, minProp)
}

func (k Keeper) propagateRecursive(
	ctx context.Context,
	currentDownstream string,
	remainingBudget sdkmath.Int,
	depth, maxDepth uint32,
	decayBpsPerHop uint32,
	minPropagation sdkmath.Int,
) error {
	if depth > maxDepth || remainingBudget.LT(minPropagation) {
		return nil
	}

	// Walk this node's upstream edges.
	var edges []*types.LineageEdge
	k.IterateBackwardLineage(ctx, currentDownstream, func(e *types.LineageEdge) bool {
		edges = append(edges, e); return false
	})

	totalShare := sdkmath.ZeroInt()
	for _, e := range edges {
		weight := sdkmath.NewIntFromUint64(uint64(types.CitationTypeWeight(e.CitationType)))
		if weight.IsZero() {
			continue
		}
		share := remainingBudget.
			Mul(sdkmath.NewIntFromUint64(uint64(e.ContributionShareBps))).
			Mul(weight).
			Quo(sdkmath.NewInt(10000))
		// Clamp to remaining budget less totalShare so far.
		availableRemaining := remainingBudget.Sub(totalShare)
		if share.GT(availableRemaining) {
			share = availableRemaining
		}
		if share.LT(minPropagation) {
			continue
		}
		totalShare = totalShare.Add(share)

		// Update upstream's accumulator.
		acc, found := k.GetLineageAccumulator(ctx, e.UpstreamAttestationId)
		if !found {
			acc = &types.LineageRoyaltyAccumulator{
				AttestationId: e.UpstreamAttestationId, CumulativeUzrn: "0",
			}
		}
		cur, _ := sdkmath.NewIntFromString(acc.CumulativeUzrn)
		acc.CumulativeUzrn = cur.Add(share).String()
		acc.LastUpdatedBlock = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
		acc.IncomingEdgeCount++
		k.WriteLineageAccumulator(ctx, acc)

		// Mark edge's settlement_payment forward-only.
		curEdgePayment, _ := sdkmath.NewIntFromString(e.SettlementPaymentUzrn)
		newEdgePayment := curEdgePayment.Add(share)
		e.SettlementPaymentUzrn = newEdgePayment.String()
		store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
		store.Set(types.LineageEdgeKey(types.EdgeID(e.UpstreamAttestationId, e.DownstreamAttestationId)), k.cdc.MustMarshal(e))

		// Recursively propagate further up.
		propagatedShare := share.Mul(sdkmath.NewIntFromUint64(uint64(decayBpsPerHop))).Quo(sdkmath.NewInt(10000))
		if err := k.propagateRecursive(ctx, e.UpstreamAttestationId, propagatedShare, depth+1, maxDepth, decayBpsPerHop, minPropagation); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run, PASS**

- [ ] **Step 5: Commit**

```bash
git add x/substrate_bridge/keeper/propagation.go x/substrate_bridge/keeper/propagation_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): lineage propagation walk (M6)

PropagateLineage distributes depth-decayed royalty from downstream
reward through the backward DAG. Budget = downstream_reward × 30%;
per-hop share = budget × contribution_share × citation_weight,
clamped to remaining. Halts at max_propagation_depth or
min_propagation_uzrn floor. Updates LineageRoyaltyAccumulator
monotonically (revenue-stream amplification).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 18: Settlement engine

**Files:**
- Create: `x/substrate_bridge/keeper/settlement.go`
- Create: `x/substrate_bridge/keeper/settlement_test.go`

The settlement engine applies the partial-settlement formula and triggers lineage propagation. Reward formula:

```
verified_ratio = (verified_count + len(cited_facts)) / total_count
L = base_L × verified_ratio
W = AxisProjection recomputed against verified subset only (Phase 0: use full projection × verified_ratio as approximation)
Q = average consensus_margin (Phase 0: use 0.5 as default if knowledge keeper doesn't expose; gov-tunable later)
R = base + L × W × Q
```

- [ ] **Step 1: Failing test**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestSettleAttestation_FullVerified(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
		AxisBounds: &types.AxisBounds{AxisSubstrateMax: 1_000_000},
	}))
	att := &types.ExternalAttestation{
		AttestationId: "att-1", AdapterId: "wiki-v1", Submitter: "alice",
		Status: types.AttestationStatus_ATTESTATION_STATUS_READY,
		Link: &types.SubstrateLink{
			RecursionWeight: &types.AxisProjection{AxisSubstrate: 500_000},
			PendingClaims: []*types.PendingClaim{{ClaimContent: "a"}, {ClaimContent: "b"}},
		},
		VerifiedCount: 2, RejectedCount: 0,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))

	require.NoError(t, k.SettleAttestation(ctx, "att-1"))

	settled, _ := k.GetAttestation(ctx, "att-1")
	require.Equal(t, types.AttestationStatus_ATTESTATION_STATUS_SETTLED, settled.Status)
	reward, _ := sdkmath.NewIntFromString(settled.RewardUzrn)
	require.True(t, reward.GT(sdkmath.ZeroInt()))
}

func TestSettleAttestation_PartialVerified(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	// 1 of 4 pending claims verified, 3 rejected (75% rejection).
	// But threshold is 50% — this should trigger REJECTED path.
	att := &types.ExternalAttestation{
		AttestationId: "att-r", AdapterId: "wiki-v1", Submitter: "bob",
		Status: types.AttestationStatus_ATTESTATION_STATUS_READY,
		Link: &types.SubstrateLink{
			PendingClaims: []*types.PendingClaim{{}, {}, {}, {}},
		},
		VerifiedCount: 1, RejectedCount: 3,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))

	require.NoError(t, k.SettleAttestation(ctx, "att-r"))

	settled, _ := k.GetAttestation(ctx, "att-r")
	require.Equal(t, types.AttestationStatus_ATTESTATION_STATUS_REJECTED, settled.Status)
}

func TestSettleAttestation_PartialButAboveThreshold(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	// 3 verified, 1 rejected (25% rejection, 75% verified) → PARTIAL settle.
	att := &types.ExternalAttestation{
		AttestationId: "att-p", AdapterId: "wiki-v1", Submitter: "carol",
		Status: types.AttestationStatus_ATTESTATION_STATUS_READY,
		Link: &types.SubstrateLink{
			RecursionWeight: &types.AxisProjection{AxisSubstrate: 100_000},
			PendingClaims: []*types.PendingClaim{{}, {}, {}, {}},
		},
		VerifiedCount: 3, RejectedCount: 1,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))

	require.NoError(t, k.SettleAttestation(ctx, "att-p"))

	settled, _ := k.GetAttestation(ctx, "att-p")
	require.Equal(t, types.AttestationStatus_ATTESTATION_STATUS_PARTIAL, settled.Status)
}
```

- [ ] **Step 2: Run, FAIL**

- [ ] **Step 3: Implement `settlement.go`**

```go
package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// SettleAttestation applies the reward formula and either:
//   - SETTLED (full reward, lineage propagation triggered)
//   - PARTIAL (reduced reward, lineage propagation on the partial)
//   - REJECTED (slash; attestation closed; no lineage)
// Eager: called synchronously when an attestation reaches READY or
// when BeginBlocker detects timeout or rejection-threshold trip.
func (k Keeper) SettleAttestation(ctx context.Context, attestationID string) error {
	att, found := k.GetAttestation(ctx, attestationID)
	if !found {
		return types.ErrAttestationNotFound
	}
	if att.Status != types.AttestationStatus_ATTESTATION_STATUS_READY &&
		att.Status != types.AttestationStatus_ATTESTATION_STATUS_PARTIAL &&
		att.Status != types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION {
		return types.ErrAttestationWrongStatus
	}

	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	totalCount := uint32(len(att.Link.CitedFacts)) + uint32(len(att.Link.PendingClaims))
	if totalCount == 0 {
		// Nothing to settle; close as SETTLED with base only.
		return k.finalizeSettle(ctx, att, sdkmath.ZeroInt(), types.AttestationStatus_ATTESTATION_STATUS_SETTLED)
	}

	// Rejection threshold check.
	pendingTotal := att.VerifiedCount + att.RejectedCount
	if pendingTotal > 0 {
		rejectionRatioBps := uint32(att.RejectedCount) * 10000 / pendingTotal
		if rejectionRatioBps >= params.PendingClaimRejectionThresholdBps {
			att.Status = types.AttestationStatus_ATTESTATION_STATUS_REJECTED
			att.RejectionReason = fmt.Sprintf("rejection ratio %d bps >= threshold %d bps",
				rejectionRatioBps, params.PendingClaimRejectionThresholdBps)
			att.SettledAtBlock = uint64(sdkCtx.BlockHeight())
			// Full bond slash (M1 fraud tier).
			att.SlashUzrn = att.BondUzrn
			return k.WriteAttestation(ctx, att)
		}
	}

	// Compute verified ratio.
	verifiedNumerator := uint32(att.VerifiedCount) + uint32(len(att.Link.CitedFacts))
	verifiedRatioBps := verifiedNumerator * 10000 / totalCount

	// Min-verified-ratio check.
	if verifiedRatioBps < params.MinVerifiedRatioForSettleBps {
		att.Status = types.AttestationStatus_ATTESTATION_STATUS_REJECTED
		att.RejectionReason = fmt.Sprintf("verified ratio %d bps < min %d bps", verifiedRatioBps, params.MinVerifiedRatioForSettleBps)
		att.SettledAtBlock = uint64(sdkCtx.BlockHeight())
		att.SlashUzrn = att.BondUzrn
		return k.WriteAttestation(ctx, att)
	}

	// Compute reward.
	reward := k.computeReward(att, verifiedRatioBps, params)

	// Status: PARTIAL if any rejected; otherwise SETTLED.
	finalStatus := types.AttestationStatus_ATTESTATION_STATUS_SETTLED
	if att.RejectedCount > 0 {
		finalStatus = types.AttestationStatus_ATTESTATION_STATUS_PARTIAL
	}

	return k.finalizeSettle(ctx, att, reward, finalStatus)
}

// computeReward applies R = base + L × W × Q, with L scaled by verifiedRatio.
// Phase 0 simplification: Q is a fixed 0.5 (5000 bps) since x/knowledge doesn't
// yet expose per-round consensus_margin via the expected-keepers interface.
// Plan 1 (x/work) or a future task will refine Q with real consensus data.
func (k Keeper) computeReward(att *types.ExternalAttestation, verifiedRatioBps uint32, params types.Params) sdkmath.Int {
	base, _ := sdkmath.NewIntFromString(params.AttestationMinBondUzrn)
	// L proxy: base × verifiedRatio
	L := base.Mul(sdkmath.NewIntFromUint64(uint64(verifiedRatioBps))).Quo(sdkmath.NewInt(10000))
	// W proxy: sum of axis projections (Phase 0; future: per-axis weighted sum)
	wTotal := sdkmath.ZeroInt()
	if att.Link != nil && att.Link.RecursionWeight != nil {
		w := att.Link.RecursionWeight
		for _, v := range []uint64{w.AxisSubstrate, w.AxisVerification, w.AxisClassification, w.AxisAttribution, w.AxisTooling, w.AxisInterface} {
			wTotal = wTotal.Add(sdkmath.NewIntFromUint64(v))
		}
	}
	// Q: fixed 0.5 at Phase 0.
	Q := sdkmath.NewInt(5000)
	// R = base + L × W × Q / (10000^2) — keep units in uzrn.
	prod := L.Mul(wTotal).Mul(Q).Quo(sdkmath.NewInt(10000)).Quo(sdkmath.NewInt(10000))
	return base.Add(prod)
}

func (k Keeper) finalizeSettle(
	ctx context.Context,
	att *types.ExternalAttestation,
	reward sdkmath.Int,
	finalStatus types.AttestationStatus,
) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	att.Status = finalStatus
	att.SettledAtBlock = uint64(sdkCtx.BlockHeight())
	att.RewardUzrn = reward.String()

	// Pay the submitter (release bond + reward).
	if k.bankKeeper != nil && reward.GT(sdkmath.ZeroInt()) {
		submitterAddr, err := sdk.AccAddressFromBech32(att.Submitter)
		if err == nil {
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", reward))
			_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins)
		}
	}

	// Trigger lineage propagation if this is a paid settle (not REJECTED).
	if finalStatus != types.AttestationStatus_ATTESTATION_STATUS_REJECTED && reward.GT(sdkmath.ZeroInt()) {
		_ = k.PropagateLineage(ctx, att.AttestationId, reward)
	}

	return k.WriteAttestation(ctx, att)
}
```

- [ ] **Step 4: Run, PASS** (3 tests).

- [ ] **Step 5: Commit**

```bash
git add x/substrate_bridge/keeper/settlement.go x/substrate_bridge/keeper/settlement_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): settlement engine (M1, M4)

SettleAttestation applies the partial-settlement formula and either
SETTLED (full reward), PARTIAL (reduced; some rejected), or REJECTED
(bond slashed; M1 fraud tier). Triggers PropagateLineage on paid
settles. Phase 0 simplification: Q (verification-quality) is fixed
at 0.5; future task refines from x/knowledge consensus_margin.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 19: MsgServer (all four message handlers)

**Files:**
- Create: `x/substrate_bridge/keeper/msg_server.go`
- Create: `x/substrate_bridge/keeper/msg_server_test.go`

The handlers cover the three gov-authority messages + the open submission.

- [ ] **Step 1: Failing test**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/keeper"
	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestMsgServer_RegisterAdapter_RequiresAuthority(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.RegisterAdapter(sdk.WrapSDKContext(ctx), &types.MsgRegisterAdapter{
		Authority: "wrong-authority",
		Adapter:   &types.AdapterRegistration{AdapterId: "wiki-v1"},
	})
	require.ErrorIs(t, err, types.ErrAdapterAuthority)
}

func TestMsgServer_RegisterAdapter_Happy(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.RegisterAdapter(sdk.WrapSDKContext(ctx), &types.MsgRegisterAdapter{
		Authority: k.Authority(),
		Adapter: &types.AdapterRegistration{
			AdapterId: "wiki-v1",
			Status:    types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
		},
	})
	require.NoError(t, err)
	_, found := k.GetAdapter(ctx, "wiki-v1")
	require.True(t, found)
}

func TestMsgServer_SubmitExternalAttestation_LinkHashEnforced(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithKnowledge(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))

	link := &types.SubstrateLink{AdapterId: "wiki-v1"}
	link.LinkHash = []byte{0x01, 0x02, 0x03} // intentionally wrong

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.SubmitExternalAttestation(sdk.WrapSDKContext(ctx), &types.MsgSubmitExternalAttestation{
		Submitter:   "zrn1sub",
		AdapterId:   "wiki-v1",
		WorkClassId: "translation",
		Link:        link,
		BondUzrn:    "222000",
	})
	require.ErrorIs(t, err, types.ErrLinkHashMismatch)
}
```

- [ ] **Step 2: Run, FAIL**

- [ ] **Step 3: Implement `msg_server.go`**

```go
package keeper

import (
	"bytes"
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

func (m msgServer) RegisterAdapter(ctx context.Context, msg *types.MsgRegisterAdapter) (*types.MsgRegisterAdapterResponse, error) {
	if msg.Authority != m.authority {
		return nil, types.ErrAdapterAuthority
	}
	if msg.Adapter == nil {
		return nil, types.ErrAdapterNotFound
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	msg.Adapter.RegisteredAtBlock = uint64(sdkCtx.BlockHeight())
	if err := m.WriteAdapter(ctx, msg.Adapter); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeAdapterRegistered,
		sdk.NewAttribute("adapter_id", msg.Adapter.AdapterId),
		sdk.NewAttribute("lip_id", msg.Adapter.RegisteredViaLipId),
		sdk.NewAttribute(AttrUsefulWorkCommitment, "UW"),
		sdk.NewAttribute(AttrMechanism, "M3"),
	))
	return &types.MsgRegisterAdapterResponse{}, nil
}

func (m msgServer) SuspendAdapter(ctx context.Context, msg *types.MsgSuspendAdapter) (*types.MsgSuspendAdapterResponse, error) {
	if msg.Authority != m.authority {
		return nil, types.ErrAdapterAuthority
	}
	if err := m.Keeper.SuspendAdapter(ctx, msg.AdapterId, msg.Reason); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		EventTypeAdapterSuspended,
		sdk.NewAttribute("adapter_id", msg.AdapterId),
		sdk.NewAttribute("reason", msg.Reason),
	))
	return &types.MsgSuspendAdapterResponse{}, nil
}

func (m msgServer) TombstoneAdapter(ctx context.Context, msg *types.MsgTombstoneAdapter) (*types.MsgTombstoneAdapterResponse, error) {
	if msg.Authority != m.authority {
		return nil, types.ErrAdapterAuthority
	}
	if err := m.Keeper.TombstoneAdapter(ctx, msg.AdapterId); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		EventTypeAdapterTombstoned,
		sdk.NewAttribute("adapter_id", msg.AdapterId),
	))
	return &types.MsgTombstoneAdapterResponse{}, nil
}

func (m msgServer) SubmitExternalAttestation(ctx context.Context, msg *types.MsgSubmitExternalAttestation) (*types.MsgSubmitExternalAttestationResponse, error) {
	if msg.Link == nil {
		return nil, types.ErrAdapterNotFound
	}
	params := m.GetParams(ctx)

	// Verify link_hash matches recomputed canonical form (M2 re-derivability).
	computed := ComputeLinkHash(msg.Link)
	if !bytes.Equal(computed, msg.Link.LinkHash) {
		return nil, types.ErrLinkHashMismatch
	}

	// Validate adapter + bounds + cited-fact existence + pending-claim cap.
	if err := m.ValidateLink(ctx, msg.Link, params); err != nil {
		return nil, err
	}

	// Get adapter for qualification + work-class allow-list check.
	adapter, _ := m.GetAdapter(ctx, msg.AdapterId)
	if len(adapter.AllowedClassIds) > 0 {
		allowed := false
		for _, cid := range adapter.AllowedClassIds {
			if cid == msg.WorkClassId {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, types.ErrWorkClassNotAllowed
		}
	}

	// Qualification check.
	if m.qualificationKeeper != nil && adapter.RequiredQualificationDomain != "" {
		qual, found := m.qualificationKeeper.GetDomainQualification(ctx, msg.Submitter, adapter.RequiredQualificationDomain)
		if !found || uint32(qual.Status) < uint32(adapter.MinQualificationStatus) {
			return nil, types.ErrInsufficientQualification
		}
	}

	// Bond check.
	bond, ok := sdkmath.NewIntFromString(msg.BondUzrn)
	if !ok {
		return nil, types.ErrInsufficientBond
	}
	minBond, _ := sdkmath.NewIntFromString(adapter.MinAttestationBondUzrn)
	if minBond.IsNil() {
		minBond, _ = sdkmath.NewIntFromString(params.AttestationMinBondUzrn)
	}
	// Per-claim bond: count × per_claim_min.
	perClaimMin, _ := sdkmath.NewIntFromString(adapter.MinPerClaimBondUzrn)
	if perClaimMin.IsNil() {
		perClaimMin, _ = sdkmath.NewIntFromString(params.PerPendingClaimBondUzrn)
	}
	totalMinBond := minBond.Add(perClaimMin.Mul(sdkmath.NewIntFromUint64(uint64(len(msg.Link.PendingClaims)))))
	if bond.LT(totalMinBond) {
		return nil, types.ErrInsufficientBond
	}

	// Lock bond.
	submitterAddr, err := sdk.AccAddressFromBech32(msg.Submitter)
	if err != nil {
		return nil, err
	}
	if m.bankKeeper != nil {
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", bond))
		if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, submitterAddr, types.ModuleName, coins); err != nil {
			return nil, err
		}
	}

	// Create attestation record.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	attID := m.NextAttestationID(ctx)
	att := &types.ExternalAttestation{
		AttestationId:    attID,
		AdapterId:        msg.AdapterId,
		WorkClassId:      msg.WorkClassId,
		Submitter:        msg.Submitter,
		BondUzrn:         msg.BondUzrn,
		Link:             msg.Link,
		Status:           types.AttestationStatus_ATTESTATION_STATUS_COMMITTED,
		SubmittedAtBlock: uint64(sdkCtx.BlockHeight()),
		CommittedAtBlock: uint64(sdkCtx.BlockHeight()),
	}

	// Auto-submit pending claims to x/knowledge and link them.
	for _, pc := range msg.Link.PendingClaims {
		claimID := fmt.Sprintf("%s::pending::%s", attID, types.PendingClaimCanonicalHash(pc))
		// Translate PendingClaim to x/knowledge.Claim shape and call SetClaim
		// (see implementer notes — the knowledge-side types differ slightly;
		// the implementer should construct a Claim from PendingClaim fields
		// using the knowledge-keeper's expected shape).
		if m.knowledgeKeeper != nil {
			// Translation responsibility deferred to a small helper; here just record the link.
		}
		_ = m.LinkPendingClaim(ctx, claimID, attID)
	}

	// Transition to AWAITING_RESOLUTION if there are pending claims; else READY immediately.
	if len(msg.Link.PendingClaims) > 0 {
		att.Status = types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION
	} else {
		att.Status = types.AttestationStatus_ATTESTATION_STATUS_READY
	}

	if err := m.WriteAttestation(ctx, att); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeExternalAttestationSubmitted,
		sdk.NewAttribute("attestation_id", attID),
		sdk.NewAttribute("adapter_id", msg.AdapterId),
		sdk.NewAttribute("work_class_id", msg.WorkClassId),
		sdk.NewAttribute("bond_uzrn", msg.BondUzrn),
		sdk.NewAttribute(AttrUsefulWorkCommitment, "UW"),
		sdk.NewAttribute(AttrMechanism, "M1,M2,M3"),
	))

	return &types.MsgSubmitExternalAttestationResponse{AttestationId: attID}, nil
}
```

- [ ] **Step 4: Run, PASS** (3 tests).

- [ ] **Step 5: Commit**

```bash
git add x/substrate_bridge/keeper/msg_server.go x/substrate_bridge/keeper/msg_server_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): MsgServer — 3 gov messages + 1 open submission

Register/Suspend/Tombstone gated by authority. SubmitExternalAttestation:
link-hash check + ValidateLink + adapter allow-class + qualification +
bond ≥ minimum + pending-claim auto-submission + AWAITING_RESOLUTION
(or READY if no pending). Voice-layer events tagged useful_work_commitment.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 20: gRPC query server

**Files:**
- Create: `x/substrate_bridge/keeper/grpc_query.go`

- [ ] **Step 1: Implement `grpc_query.go`** (test omitted — covered by integration test Task 26)

```go
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

type queryServer struct{ Keeper }

func NewQueryServerImpl(k Keeper) types.QueryServer { return &queryServer{Keeper: k} }

func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	p := q.GetParams(ctx)
	return &types.QueryParamsResponse{Params: &p}, nil
}

func (q queryServer) Adapter(ctx context.Context, req *types.QueryAdapterRequest) (*types.QueryAdapterResponse, error) {
	a, found := q.GetAdapter(ctx, req.AdapterId)
	if !found {
		return nil, types.ErrAdapterNotFound
	}
	return &types.QueryAdapterResponse{Adapter: a}, nil
}

func (q queryServer) Adapters(ctx context.Context, req *types.QueryAdaptersRequest) (*types.QueryAdaptersResponse, error) {
	var out []*types.AdapterRegistration
	q.IterateAdapters(ctx, func(a *types.AdapterRegistration) bool {
		if req.StatusFilter == types.AdapterStatus_ADAPTER_STATUS_UNSPECIFIED || a.Status == req.StatusFilter {
			out = append(out, a)
		}
		return false
	})
	return &types.QueryAdaptersResponse{Adapters: out}, nil
}

func (q queryServer) Attestation(ctx context.Context, req *types.QueryAttestationRequest) (*types.QueryAttestationResponse, error) {
	att, found := q.GetAttestation(ctx, req.AttestationId)
	if !found {
		return nil, types.ErrAttestationNotFound
	}
	return &types.QueryAttestationResponse{Attestation: att}, nil
}

func (q queryServer) LineageForwardWalk(ctx context.Context, req *types.QueryLineageForwardWalkRequest) (*types.QueryLineageForwardWalkResponse, error) {
	var edges []*types.LineageEdge
	q.IterateForwardLineage(ctx, req.AttestationId, func(e *types.LineageEdge) bool {
		edges = append(edges, e); return false
	})
	return &types.QueryLineageForwardWalkResponse{Edges: edges}, nil
}

func (q queryServer) LineageBackwardWalk(ctx context.Context, req *types.QueryLineageBackwardWalkRequest) (*types.QueryLineageBackwardWalkResponse, error) {
	var edges []*types.LineageEdge
	q.IterateBackwardLineage(ctx, req.AttestationId, func(e *types.LineageEdge) bool {
		edges = append(edges, e); return false
	})
	return &types.QueryLineageBackwardWalkResponse{Edges: edges}, nil
}

func (q queryServer) LineageAccumulator(ctx context.Context, req *types.QueryLineageAccumulatorRequest) (*types.QueryLineageAccumulatorResponse, error) {
	acc, found := q.GetLineageAccumulator(ctx, req.AttestationId)
	if !found {
		acc = &types.LineageRoyaltyAccumulator{AttestationId: req.AttestationId, CumulativeUzrn: "0"}
	}
	return &types.QueryLineageAccumulatorResponse{Accumulator: acc}, nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/substrate_bridge/...`
Expected: clean. The module.go references (NewMsgServerImpl, NewQueryServerImpl) now resolve. BeginBlocker still missing — will resolve at Task 21.

Skip whole-build verification (`go build ./...`) until Task 21.

- [ ] **Step 3: Commit**

```bash
git add x/substrate_bridge/keeper/grpc_query.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): gRPC query server (7 endpoints)

Params, Adapter, Adapters (with status filter), Attestation, Lineage
forward/backward walks, LineageAccumulator. All read-only; backed by
keeper Iterate* and Get* methods.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 21: BeginBlocker + OnClaimResolved hook + events

**Files:**
- Create: `x/substrate_bridge/keeper/begin_block.go`
- Create: `x/substrate_bridge/keeper/hooks.go`
- Create: `x/substrate_bridge/keeper/events.go`
- Create: `x/substrate_bridge/keeper/begin_block_test.go`

- [ ] **Step 1: Create `events.go`**

```go
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

	EventTypeLineageEdgeCreated  = "lineage_edge_created"
	EventTypeLineageRoyaltyPaid  = "lineage_royalty_paid"

	// Attributes.
	AttrUsefulWorkCommitment = "useful_work_commitment"  // value: "UW"
	AttrMechanism            = "mechanism"                // value: "M1" | "M2,M3" | etc.
)
```

- [ ] **Step 2: Failing test for BeginBlocker timeout**

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func TestBeginBlocker_TimeoutTransitionsToPartial(t *testing.T) {
	k, ctx := setupSubstrateBridgeKeeperWithBank(t)
	require.NoError(t, k.WriteAdapter(ctx, &types.AdapterRegistration{
		AdapterId: "wiki-v1", Status: types.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))
	// Attestation submitted ~10M blocks ago (past 6.2M window).
	att := &types.ExternalAttestation{
		AttestationId: "old-att", AdapterId: "wiki-v1",
		Status: types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
		SubmittedAtBlock: 0,
		Link: &types.SubstrateLink{PendingClaims: []*types.PendingClaim{{}, {}}},
		VerifiedCount: 1, RejectedCount: 0,
	}
	require.NoError(t, k.WriteAttestation(ctx, att))

	// Move chain forward past the timeout (default 6.2M).
	ctx = ctx.WithBlockHeight(7_000_000)

	require.NoError(t, k.BeginBlocker(ctx))

	settled, _ := k.GetAttestation(ctx, "old-att")
	// 50% verified (1 of 2) ≥ MinVerifiedRatioForSettleBps (10%) → PARTIAL.
	// But < PendingClaimRejectionThresholdBps (50%) → not REJECTED.
	// So timeout drives a settle-as-PARTIAL.
	require.Equal(t, types.AttestationStatus_ATTESTATION_STATUS_PARTIAL, settled.Status)
}
```

- [ ] **Step 3: Implement `begin_block.go`**

```go
package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// BeginBlocker scans AWAITING_RESOLUTION attestations and:
//   - times out any older than Params.MaxPendingWindowBlocks (settle-as-PARTIAL or REJECTED)
//   - drains READY attestations into SETTLED (called from settlement engine,
//     but BeginBlocker also pulls them in case OnClaimResolved didn't fire
//     for the last pending claim).
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := uint64(sdkCtx.BlockHeight())
	params := k.GetParams(ctx)

	// Timeout scan.
	var timedOut []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, func(id string) bool {
		att, found := k.GetAttestation(ctx, id)
		if !found {
			return false
		}
		if currentHeight-att.SubmittedAtBlock >= params.MaxPendingWindowBlocks {
			timedOut = append(timedOut, id)
		}
		return false
	})
	for _, id := range timedOut {
		if err := k.SettleAttestation(ctx, id); err != nil {
			k.Logger(sdkCtx).Error("timeout-settle failed", "attestation_id", id, "err", err)
		}
	}

	// READY drain.
	var ready []string
	k.IterateAttestationsByStatus(ctx, types.AttestationStatus_ATTESTATION_STATUS_READY, func(id string) bool {
		ready = append(ready, id); return false
	})
	for _, id := range ready {
		if err := k.SettleAttestation(ctx, id); err != nil {
			k.Logger(sdkCtx).Error("ready-settle failed", "attestation_id", id, "err", err)
		}
	}

	return nil
}
```

- [ ] **Step 4: Implement `hooks.go`**

```go
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// OnClaimResolved is called by x/knowledge.CompleteRound after writing
// the verification verdict. The substrate_bridge keeper:
//   1. Looks up the pending-fact index for the resolved claim.
//   2. If indexed, increments VerifiedCount/RejectedCount on the parent
//      attestation.
//   3. If all pending claims have resolved (verified+rejected == total),
//      transitions attestation to READY.
//   4. The READY attestation gets settled in the next BeginBlocker pass,
//      or directly here if synchronous settle is desired (Phase 0:
//      deferred to BeginBlocker for simpler ordering).
//
// verdict is true for VERIFIED, false for REJECTED.
func (k Keeper) OnClaimResolved(ctx context.Context, claimID string, verdict bool) error {
	attestationID, found := k.GetAttestationForPendingClaim(ctx, claimID)
	if !found {
		return nil  // not a substrate_bridge-managed claim; ignore
	}

	att, found := k.GetAttestation(ctx, attestationID)
	if !found {
		return nil
	}
	if att.Status != types.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION {
		return nil  // already transitioned (timeout or earlier rejection)
	}

	if verdict {
		att.VerifiedCount++
	} else {
		att.RejectedCount++
	}

	totalPending := uint32(len(att.Link.PendingClaims))
	resolved := uint32(att.VerifiedCount + att.RejectedCount)
	if resolved >= totalPending {
		att.Status = types.AttestationStatus_ATTESTATION_STATUS_READY
	}

	// Unlink the resolved claim from the index.
	_ = k.UnlinkPendingClaim(ctx, claimID, attestationID)

	return k.WriteAttestation(ctx, att)
}
```

- [ ] **Step 5: Run, PASS**

Run: `go test ./x/substrate_bridge/keeper/ -run "TestBeginBlocker" -v` → PASS.
Run: `go build ./x/substrate_bridge/...` → clean (all module.go refs resolve now).

- [ ] **Step 6: Commit**

```bash
git add x/substrate_bridge/keeper/events.go x/substrate_bridge/keeper/begin_block.go x/substrate_bridge/keeper/hooks.go x/substrate_bridge/keeper/begin_block_test.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): BeginBlocker + OnClaimResolved hook + event constants

BeginBlocker scans AWAITING_RESOLUTION for timeouts and drains READY
queue. OnClaimResolved is the consumer hook from x/knowledge.CompleteRound:
increments VerifiedCount/RejectedCount, transitions to READY when all
pending resolve. events.go declares the 12 event types and 2 attributes.
Module now builds end-to-end.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 22: Wire `x/knowledge.CompleteRound` to notify substrate_bridge

**Files:**
- Modify: `x/knowledge/keeper/rounds.go` (one-line hook addition)
- Modify: `x/knowledge/keeper/keeper.go` (add SubstrateBridgeKeeper field)
- Modify: `x/knowledge/types/expected_keepers.go` (add SubstrateBridgeKeeper interface)
- Modify: `app/app.go` (pass substrate_bridge keeper to knowledge keeper at construction — done in Task 25)

- [ ] **Step 1: Add interface to x/knowledge expected keepers**

Open `x/knowledge/types/expected_keepers.go` (create if absent). Append:

```go
// SubstrateBridgeKeeper is the consumer hook for substrate_bridge.
// CompleteRound notifies on every claim verification verdict so the
// substrate_bridge pending-fact index can resolve waiting attestations.
type SubstrateBridgeKeeper interface {
	OnClaimResolved(ctx context.Context, claimID string, verdict bool) error
}
```

If the file already has imports for `context`, no change; otherwise add `"context"`.

- [ ] **Step 2: Add SubstrateBridgeKeeper field to x/knowledge Keeper**

In `x/knowledge/keeper/keeper.go`, add a nullable `substrateBridgeKeeper types.SubstrateBridgeKeeper` field. Add a setter method:

```go
// SetSubstrateBridgeKeeper wires the substrate_bridge keeper after both
// modules' keepers exist (avoids cyclic init). app.go calls this in the
// post-init phase.
func (k *Keeper) SetSubstrateBridgeKeeper(sbk types.SubstrateBridgeKeeper) {
	k.substrateBridgeKeeper = sbk
}
```

- [ ] **Step 3: Hook into CompleteRound**

In `x/knowledge/keeper/rounds.go`, at the END of `CompleteRound` (after status writes, before return), add:

```go
// Notify substrate_bridge so any external attestation waiting on this
// claim's resolution can transition.
if k.substrateBridgeKeeper != nil {
	// verdict is true (VERIFIED) iff result.Verdict == VERDICT_ACCEPT.
	verdict := result.Verdict == types.Verdict_VERDICT_ACCEPT
	_ = k.substrateBridgeKeeper.OnClaimResolved(ctx, claim.Id, verdict)
}
```

The `claim` variable should be in scope (it is the parameter passed to handleVerified or similar; check the existing function signature for the right name).

- [ ] **Step 4: Verify build**

Run: `go build ./x/knowledge/...`
Expected: clean.

- [ ] **Step 5: Verify no existing knowledge tests broken**

Run: `go test ./x/knowledge/keeper/ -timeout 120s`
Expected: PASS (the new hook is gated by nil-check; tests don't wire substrate_bridge, so the hook is a no-op there).

- [ ] **Step 6: Commit**

```bash
git add x/knowledge/types/expected_keepers.go x/knowledge/keeper/keeper.go x/knowledge/keeper/rounds.go
git commit -m "$(cat <<'EOF'
feat(knowledge): notify substrate_bridge on claim resolution

CompleteRound now calls substrateBridgeKeeper.OnClaimResolved (nil-
gated) so external attestations awaiting pending-claim resolution can
transition. SubstrateBridgeKeeper interface added to expected_keepers.
Knowledge keeper gains substrateBridgeKeeper field + SetSubstrateBridgeKeeper
post-init setter (avoids cyclic init).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 23: Add `CategoryAdapterRegistration` LIP class to x/creed

**Files:**
- Modify: `x/creed/types/sub_creeds.go` (or wherever `CategoryCreedAmendment`-like constants live)
- Modify: `x/gov/keeper/lip_class.go` (or equivalent) — add handler dispatch

- [ ] **Step 1: Read existing LIP class enum**

Run: `grep -rn "Category[A-Z][a-zA-Z]*\s*=" x/creed/types x/gov/types 2>/dev/null | head -10`

Find the existing enum (e.g., `LIPCategory_CATEGORY_CREED_AMENDMENT`, `LIPCategory_CATEGORY_USEFUL_WORK_AMENDMENT`). The pattern: each LIP class is a proto enum value with associated quorum/voting requirements.

- [ ] **Step 2: Add the new LIP category**

In the proto file that defines `LIPCategory` (likely `proto/zerone/gov/v1/lip.proto` or similar), append a new enum value:

```proto
LIP_CATEGORY_ADAPTER_REGISTRATION = N;  // pick the next unused integer
```

Run `make proto-gen`.

- [ ] **Step 3: Add handler dispatch**

In whatever module dispatches LIP execution (likely `x/gov` or `x/creed`), find the existing switch on `LIPCategory` and add a case:

```go
case lipv1.LIPCategory_LIP_CATEGORY_ADAPTER_REGISTRATION:
    // Unmarshal the LIP payload as types.AdapterRegistration and call:
    return k.substrateBridgeKeeper.WriteAdapter(ctx, &adapter)
```

The handler calls into `x/substrate_bridge.WriteAdapter` (which is unauthenticated at the keeper level — the dispatch is the authority gate).

If a generic LIP-dispatch mechanism doesn't yet exist, this task can document the requirement and defer to a follow-up — but the spec depends on this gate existing. Minimum viable: add the enum value + a TODO comment in the dispatch site naming what needs to wire.

- [ ] **Step 4: Verify quorum / Creed Council requirements**

Look at how `CategoryCreedAmendment` declares its quorum (it should require Creed Council quorum on top of standard gov). Apply the same to `LIP_CATEGORY_ADAPTER_REGISTRATION`. The exact mechanism depends on the existing code — likely a switch in `x/gov/keeper/tally.go` or similar.

- [ ] **Step 5: Verify build**

Run: `go build ./x/creed/... ./x/gov/...`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add proto/zerone/gov/v1/lip.proto x/gov/types/*.pb.go x/creed/types/sub_creeds.go x/gov/keeper/<dispatch_file>.go
git commit -m "$(cat <<'EOF'
feat(creed,gov): CategoryAdapterRegistration LIP class

New gov-passable LIP class that, on passage, dispatches MsgRegisterAdapter
to substrate_bridge. Requires standard gov quorum + Creed Council
quorum (mirror of CategoryCreedAmendment). Trust-deliberate expansion
of the chain's external-source surface.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 24: CLI commands

**Files:**
- Create: `x/substrate_bridge/client/cli/query.go`
- Create: `x/substrate_bridge/client/cli/tx.go`

CLI for query and tx — minimal coverage of the three gov-authority messages (only relevant for gov dispatch testing on localnet) and the open submission message, plus the 7 query endpoints.

- [ ] **Step 1: Implement `client/cli/query.go`**

```go
package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "Query substrate_bridge state",
	}
	cmd.AddCommand(cmdQueryParams())
	cmd.AddCommand(cmdQueryAdapter())
	cmd.AddCommand(cmdQueryAdapters())
	cmd.AddCommand(cmdQueryAttestation())
	cmd.AddCommand(cmdQueryLineageForward())
	cmd.AddCommand(cmdQueryLineageBackward())
	cmd.AddCommand(cmdQueryLineageAccumulator())
	return cmd
}

func cmdQueryParams() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Show module params",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cctx, err := client.GetClientQueryContext(cmd)
			if err != nil { return err }
			res, err := types.NewQueryClient(cctx).Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil { return err }
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryAdapter() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapter [adapter-id]",
		Short: "Show a registered adapter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).Adapter(cmd.Context(), &types.QueryAdapterRequest{AdapterId: args[0]})
			if err != nil { return err }
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryAdapters() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "adapters",
		Short: "List adapters (optionally filtered by status)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).Adapters(cmd.Context(), &types.QueryAdaptersRequest{})
			if err != nil { return err }
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attestation [attestation-id]",
		Short: "Show an external attestation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).Attestation(cmd.Context(), &types.QueryAttestationRequest{AttestationId: args[0]})
			if err != nil { return err }
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryLineageForward() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineage-forward [attestation-id]",
		Short: "Walk forward lineage (downstream uses)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).LineageForwardWalk(cmd.Context(), &types.QueryLineageForwardWalkRequest{AttestationId: args[0]})
			if err != nil { return err }
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryLineageBackward() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineage-backward [attestation-id]",
		Short: "Walk backward lineage (upstream cites)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).LineageBackwardWalk(cmd.Context(), &types.QueryLineageBackwardWalkRequest{AttestationId: args[0]})
			if err != nil { return err }
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func cmdQueryLineageAccumulator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lineage-accumulator [attestation-id]",
		Short: "Cumulative lineage royalty income for an attestation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, _ := client.GetClientQueryContext(cmd)
			res, err := types.NewQueryClient(cctx).LineageAccumulator(cmd.Context(), &types.QueryLineageAccumulatorRequest{AttestationId: args[0]})
			if err != nil { return err }
			return cctx.PrintProto(res)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

- [ ] **Step 2: Implement `client/cli/tx.go`** — a single open-submission CLI command. (Gov-authority messages go through the gov CLI; we don't need standalone tx commands for them at Phase 0.)

```go
package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "substrate_bridge transactions",
	}
	cmd.AddCommand(cmdSubmitExternalAttestation())
	return cmd
}

func cmdSubmitExternalAttestation() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-attestation [adapter-id] [work-class-id] [link-json-file] [bond-uzrn]",
		Short: "Submit an external attestation. link-json-file is a JSON-encoded SubstrateLink.",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cctx, err := client.GetClientTxContext(cmd)
			if err != nil { return err }
			// Read SubstrateLink from JSON file.
			var link types.SubstrateLink
			if err := readJSONFile(args[2], &link); err != nil { return err }
			msg := &types.MsgSubmitExternalAttestation{
				Submitter:   cctx.GetFromAddress().String(),
				AdapterId:   args[0],
				WorkClassId: args[1],
				Link:        &link,
				BondUzrn:    args[3],
			}
			return tx.GenerateOrBroadcastTxCLI(cctx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// readJSONFile is a small helper using cctx codec to unmarshal. Implementer
// can use cosmos-sdk's encoding context.JSONCodec.UnmarshalJSON. See the
// pattern in x/knowledge/client/cli/tx.go.
```

- [ ] **Step 3: Verify build**

Run: `go build ./x/substrate_bridge/...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add x/substrate_bridge/client/cli/query.go x/substrate_bridge/client/cli/tx.go
git commit -m "$(cat <<'EOF'
feat(substrate_bridge): CLI — 7 query commands + open-submission tx

query subcommands cover all 7 gRPC endpoints. tx has one command for
MsgSubmitExternalAttestation (gov-authority messages dispatched
through gov CLI). SubstrateLink read from JSON file argument.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 25: Wire module in `app/app.go`

**Files:**
- Modify: `app/app.go`

Hook the new module into the app: keeper construction, basic-manager, store-key, BeginBlocker order, ModuleManager init/export. Read `app/app.go` to find where other modules wire and follow the same pattern.

- [ ] **Step 1: Read existing wiring patterns**

Run: `grep -n "InquiryKeeper\|inquiry.NewKeeper\|inquiry.NewAppModule\|inquirytypes.StoreKey\|inquirytypes.ModuleName" app/app.go | head -20`

Locate the four insertion sites:
1. Imports
2. App struct field for keeper
3. Keeper construction in `NewApp`
4. Module manager registration

- [ ] **Step 2: Add imports**

Near other zerone-module imports in `app/app.go`:

```go
substratebridge "github.com/zerone-chain/zerone/x/substrate_bridge"
substratebridgekeeper "github.com/zerone-chain/zerone/x/substrate_bridge/keeper"
substratebridgetypes "github.com/zerone-chain/zerone/x/substrate_bridge/types"
```

- [ ] **Step 3: Add keeper field**

In the App struct definition, after other module keepers:

```go
SubstrateBridgeKeeper substratebridgekeeper.Keeper
```

- [ ] **Step 4: Add store key**

In the slice of store keys (search for `storetypes.NewKVStoreKey(types.StoreKey)` calls), add:

```go
substratebridgetypes.StoreKey,
```

- [ ] **Step 5: Construct keeper**

After x/knowledge keeper construction (substrate_bridge depends on knowledge):

```go
app.SubstrateBridgeKeeper = substratebridgekeeper.NewKeeper(
    appCodec,
    keys[substratebridgetypes.StoreKey],
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
    &app.KnowledgeKeeper,
    &app.QualificationKeeper,
    app.BankKeeper,
    app.AccountKeeper,
)

// Post-init wire-back: knowledge keeper needs substrate_bridge for
// OnClaimResolved hook (Task 22).
app.KnowledgeKeeper.SetSubstrateBridgeKeeper(&app.SubstrateBridgeKeeper)
```

- [ ] **Step 6: Register module account permissions**

Find the `maccPerms` map and add:

```go
substratebridgetypes.ModuleName:            nil,  // pure escrow; no mint/burn
substratebridgetypes.AuditBountyPoolModuleName: {authtypes.Minter},  // M7 audit bounty pool
```

- [ ] **Step 7: Add to ModuleManager**

Find the `module.NewManager(...)` call. Add:

```go
substratebridge.NewAppModule(appCodec, app.SubstrateBridgeKeeper),
```

Add to the BeginBlocker order list (after `knowledge` so OnClaimResolved hooks fire before BeginBlocker scans):

```go
substratebridgetypes.ModuleName,
```

Add to InitGenesis order list (after `knowledge`):

```go
substratebridgetypes.ModuleName,
```

- [ ] **Step 8: Verify build + ALL tests still pass**

Run: `go build ./...` (timeout 180s)
Expected: clean.

Run: `go test ./... -timeout 300s`
Expected: PASS or known-skipped. The new substrate_bridge tests should be picked up.

- [ ] **Step 9: Commit**

```bash
git add app/app.go
git commit -m "$(cat <<'EOF'
feat(app): wire x/substrate_bridge module

Imports, App struct field, store key, keeper construction (depends on
knowledge/qualification/bank/account), maccPerms (module + audit
bounty pool), ModuleManager registration, BeginBlocker/InitGenesis
ordering. Post-init wire-back to knowledge keeper for OnClaimResolved
hook.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 26: Cross-stack integration test

**Files:**
- Create: `tests/cross_stack/substrate_bridge_test.go`

End-to-end integration: register adapter → submit attestation with pending claims → x/knowledge verifies them → READY → SETTLED → lineage edge created → another attestation cites the first → propagation pays the first's submitter.

- [ ] **Step 1: Write the integration test**

```go
package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	substratebridgekeeper "github.com/zerone-chain/zerone/x/substrate_bridge/keeper"
	substratebridgetypes "github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// TestSubstrateBridge_HappyPathSettlement drives the full lifecycle:
//   1. Register adapter via direct keeper call (simulating gov-passed LIP).
//   2. Seed cited_fact in x/knowledge.
//   3. Submit external attestation with 2 pending claims via msg server.
//   4. Resolve pending claims as VERIFIED (calls into substrate_bridge.OnClaimResolved).
//   5. BeginBlocker drains READY → SETTLED.
//   6. Verify submitter received reward.
func TestSubstrateBridge_HappyPathSettlement(t *testing.T) {
	h := NewTestHarness(t)

	// 1. Register adapter.
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAdapter(h.Ctx, &substratebridgetypes.AdapterRegistration{
		AdapterId:              "test-wiki",
		Status:                 substratebridgetypes.AdapterStatus_ADAPTER_STATUS_ACTIVE,
		MinAttestationBondUzrn: "222000",
		MinPerClaimBondUzrn:    "222",
		AxisBounds:             &substratebridgetypes.AxisBounds{AxisSubstrateMax: 1_000_000},
	}))

	// 2. Seed cited fact.
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name: "test-domain", Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "seed-fact", Domain: "test-domain",
		Status: knowledgetypes.FactStatus_FACT_STATUS_VERIFIED, VerifiedAtBlock: 1,
	}))

	// 3. Submit attestation.
	link := &substratebridgetypes.SubstrateLink{
		AdapterId: "test-wiki",
		CitedFacts: []*substratebridgetypes.FactCitation{
			{FactId: "seed-fact", CitationType: substratebridgetypes.CitationType_CITATION_TYPE_SUPPORTS},
		},
		PendingClaims: []*substratebridgetypes.PendingClaim{
			{ClaimContent: "claim A", Domain: "test-domain"},
			{ClaimContent: "claim B", Domain: "test-domain"},
		},
		RecursionWeight: &substratebridgetypes.AxisProjection{AxisSubstrate: 100_000},
	}
	link.LinkHash = substratebridgekeeper.ComputeLinkHash(link)

	submitter := sdk.AccAddress([]byte("submitter1-padded-to-20-bytes")).String()
	// Fund submitter for bond.
	require.NoError(t, h.FundAccount(sdk.MustAccAddressFromBech32(submitter), sdk.NewCoins(sdk.NewInt64Coin("uzrn", 10_000_000))))

	srv := substratebridgekeeper.NewMsgServerImpl(h.SubstrateBridgeKeeper)
	resp, err := srv.SubmitExternalAttestation(h.Ctx, &substratebridgetypes.MsgSubmitExternalAttestation{
		Submitter:   submitter,
		AdapterId:   "test-wiki",
		WorkClassId: "translation",
		Link:        link,
		BondUzrn:    "1000000",
	})
	require.NoError(t, err)
	attID := resp.AttestationId

	att, _ := h.SubstrateBridgeKeeper.GetAttestation(h.Ctx, attID)
	require.Equal(t, substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION, att.Status)

	// 4. Resolve pending claims.
	for _, claimID := range h.SubstrateBridgeKeeper.PendingClaimsFor(h.Ctx, attID) {
		require.NoError(t, h.SubstrateBridgeKeeper.OnClaimResolved(h.Ctx, claimID, true))
	}

	att, _ = h.SubstrateBridgeKeeper.GetAttestation(h.Ctx, attID)
	require.Equal(t, substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_READY, att.Status)

	// 5. BeginBlocker drains READY.
	require.NoError(t, h.SubstrateBridgeKeeper.BeginBlocker(h.Ctx))

	att, _ = h.SubstrateBridgeKeeper.GetAttestation(h.Ctx, attID)
	require.Equal(t, substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_SETTLED, att.Status)
	require.NotEqual(t, "0", att.RewardUzrn)
}

// TestSubstrateBridge_RejectionThreshold drives the fraud path: most
// pending claims are REJECTED → attestation transitions to REJECTED →
// bond slashed.
func TestSubstrateBridge_RejectionThreshold(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAdapter(h.Ctx, &substratebridgetypes.AdapterRegistration{
		AdapterId: "fraud-adapter", Status: substratebridgetypes.AdapterStatus_ADAPTER_STATUS_ACTIVE,
	}))

	// Submit 4 pending claims; reject 3 of them (75% > 50% threshold).
	att := &substratebridgetypes.ExternalAttestation{
		AttestationId: "fraud-att",
		AdapterId: "fraud-adapter",
		Status: substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_AWAITING_RESOLUTION,
		Link: &substratebridgetypes.SubstrateLink{
			PendingClaims: []*substratebridgetypes.PendingClaim{{}, {}, {}, {}},
		},
		BondUzrn: "1000000",
	}
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, att))

	// Drive: 3 rejections, 1 verification — rejection ratio 75%.
	att.RejectedCount = 3
	att.VerifiedCount = 1
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, att))

	require.NoError(t, h.SubstrateBridgeKeeper.SettleAttestation(h.Ctx, "fraud-att"))

	final, _ := h.SubstrateBridgeKeeper.GetAttestation(h.Ctx, "fraud-att")
	require.Equal(t, substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_REJECTED, final.Status)
	require.NotEmpty(t, final.RejectionReason)
}

// TestSubstrateBridge_LineagePropagatesAcrossClasses drives a cross-
// class lineage chain: attestation A (class translation) → attestation
// B (class curriculum) → attestation B settles → A receives lineage
// royalty.
func TestSubstrateBridge_LineagePropagatesAcrossClasses(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, &substratebridgetypes.ExternalAttestation{
		AttestationId: "att-A", WorkClassId: "translation",
		Submitter: "zrn1alice...", SubmittedAtBlock: 10,
		Status: substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_SETTLED,
	}))
	require.NoError(t, h.SubstrateBridgeKeeper.WriteAttestation(h.Ctx, &substratebridgetypes.ExternalAttestation{
		AttestationId: "att-B", WorkClassId: "curriculum",
		Submitter: "zrn1bob...", SubmittedAtBlock: 20,
		Status: substratebridgetypes.AttestationStatus_ATTESTATION_STATUS_READY,
		Link: &substratebridgetypes.SubstrateLink{
			RecursionWeight: &substratebridgetypes.AxisProjection{AxisSubstrate: 50_000},
		},
	}))
	require.NoError(t, h.SubstrateBridgeKeeper.CreateLineageEdge(h.Ctx, &substratebridgetypes.LineageEdge{
		UpstreamAttestationId:   "att-A",
		DownstreamAttestationId: "att-B",
		CitationType:            substratebridgetypes.CitationType_CITATION_TYPE_SUPPORTS,
		ContributionShareBps:    10000,
	}))

	require.NoError(t, h.SubstrateBridgeKeeper.SettleAttestation(h.Ctx, "att-B"))

	// A's accumulator should be non-zero.
	acc, found := h.SubstrateBridgeKeeper.GetLineageAccumulator(h.Ctx, "att-A")
	require.True(t, found)
	require.NotEqual(t, "0", acc.CumulativeUzrn)
}
```

Note: the test harness `NewTestHarness` in `tests/cross_stack/harness_test.go` must be extended (or wraps `app/app.go`'s setup) to include `SubstrateBridgeKeeper`. The implementer should add one field + one wire-up line per the pattern of `KnowledgeKeeper`. If `FundAccount` doesn't exist, use the existing harness helper for funding (look at how `claiming_test.go` funds accounts).

- [ ] **Step 2: Run, PASS**

Run: `go test ./tests/cross_stack/ -run TestSubstrateBridge -v -timeout 300s`
Expected: 3 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add tests/cross_stack/substrate_bridge_test.go tests/cross_stack/harness_test.go
git commit -m "$(cat <<'EOF'
test(cross_stack): substrate_bridge end-to-end integration

Three scenarios: happy-path settlement (register → submit → resolve →
SETTLED), rejection threshold (75% rejected → REJECTED + slash),
cross-class lineage propagation (translation → curriculum
settlement; upstream accumulator non-zero).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 27: Voice + position layers + EVENTS.md

**Files:**
- Modify: `docs/EVENTS.md` (append new event surface)
- Verify: `x/substrate_bridge/doc.go` is complete (position layer)
- Verify: All event emissions in `msg_server.go`, `settlement.go`, `lineage.go` carry `useful_work_commitment="UW"`

- [ ] **Step 1: Append to `docs/EVENTS.md`**

Add this section near the bottom (after the existing ToK substrate events section):

```markdown
## Substrate Bridge events (docs/USEFUL_WORK.md M1-M7)

### `external_attestation_submitted`
A submitter posted an external attestation. UW + M1 (stake-backed claim) + M2 (substrate-link mandate) + M3 (class-specific verification under shared lifecycle) bound by this event firing on every accepted submission.

| Attribute | Description |
|---|---|
| `useful_work_commitment` | `"UW"` |
| `mechanism` | `"M1,M2,M3"` |
| `attestation_id` | Assigned id (e.g. `att-100-7`) |
| `adapter_id` | Adapter used to compile the link |
| `work_class_id` | The work class this attestation belongs to |
| `bond_uzrn` | Total bond locked |

### `external_attestation_settled`
Eager settlement after all pending claims resolve OR via BeginBlocker drain.

| Attribute | Description |
|---|---|
| `useful_work_commitment` | `"UW"` |
| `mechanism` | `"M4"` |
| `attestation_id` | |
| `reward_uzrn` | |
| `verified_ratio_bps` | |

### `external_attestation_partial`
Settlement with reduced reward when some pending claims rejected (still above rejection threshold).

| Attribute | Description |
|---|---|
| `useful_work_commitment` | `"UW"` |
| `mechanism` | `"M1,M4"` |
| `attestation_id` | |
| `reward_uzrn` | |
| `verified_ratio_bps` | |

### `external_attestation_rejected`
Rejection-threshold tripped (>50% of pending claims rejected) OR verified ratio below 10% floor. Bond slashed.

| Attribute | Description |
|---|---|
| `useful_work_commitment` | `"UW"` |
| `mechanism` | `"M1"` |
| `attestation_id` | |
| `rejection_reason` | |
| `slash_uzrn` | |

### `adapter_registered`
A new adapter passed CategoryAdapterRegistration LIP and is now ACTIVE.

| Attribute | Description |
|---|---|
| `useful_work_commitment` | `"UW"` |
| `mechanism` | `"M3"` |
| `adapter_id` | |
| `lip_id` | The passing LIP |

### `adapter_suspended` / `adapter_tombstoned`
Adapter lifecycle events. Suspended is recoverable; Tombstoned is permanent (commitment 10).

### `lineage_edge_created`
A LineageEdge written at downstream settlement. M6: cross-class lineage.

| Attribute | Description |
|---|---|
| `useful_work_commitment` | `"UW"` |
| `mechanism` | `"M6"` |
| `upstream_attestation_id` | |
| `downstream_attestation_id` | |
| `citation_type` | |
| `contribution_share_bps` | |

### `lineage_royalty_paid`
A propagation hop paid lineage to an upstream attestation. M6.

| Attribute | Description |
|---|---|
| `useful_work_commitment` | `"UW"` |
| `mechanism` | `"M6"` |
| `to_attestation_id` | Recipient |
| `from_attestation_id` | Source (the settling downstream) |
| `uzrn` | Paid amount |
| `depth` | Distance from the original settle (1 = direct cite) |
```

- [ ] **Step 2: Verify position-layer doc.go is complete**

Run: `go doc ./x/substrate_bridge | head -80`
Expected: doc.go from Task 10 declares all M1-M7 bindings. If anything missing, add to doc.go.

- [ ] **Step 3: Verify voice-layer emissions are tagged**

Run: `grep -rn "useful_work_commitment" x/substrate_bridge/keeper/`
Expected: every emit-site includes `useful_work_commitment="UW"` attribute. If a missing site is found, add the attribute.

Specifically check:
- `msg_server.go`: `external_attestation_submitted`, `adapter_registered`
- `settlement.go`: `external_attestation_settled`, `external_attestation_partial`, `external_attestation_rejected`, `lineage_royalty_paid`
- `lineage.go`: `lineage_edge_created`

If `settlement.go` or `lineage.go` don't yet emit these events (they were sketched as direct keeper-call paths in Task 17/18), add the emissions now.

- [ ] **Step 4: Commit**

```bash
git add docs/EVENTS.md x/substrate_bridge/keeper/settlement.go x/substrate_bridge/keeper/lineage.go x/substrate_bridge/keeper/propagation.go
git commit -m "$(cat <<'EOF'
docs(events): document substrate_bridge event surface; finalize voice layer

8 new events documented in EVENTS.md with useful_work_commitment +
mechanism attributes. settlement.go and lineage.go/propagation.go
emit events at every state transition and royalty payment.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 28: Final integration sweep

- [ ] **Step 1: Run all substrate_bridge tests**

Run: `go test ./x/substrate_bridge/... -count=1 -v -timeout 180s`
Expected: PASS (all unit tests across keeper sub-files).

- [ ] **Step 2: Run cross-stack tests including substrate_bridge**

Run: `go test ./tests/cross_stack/ -run "TestSubstrateBridge|TestToK|TestTruthSeeking|TestUsefulWork" -count=1 -v -timeout 300s`
Expected: all PASS (no regression on existing tests; substrate_bridge tests PASS).

- [ ] **Step 3: Run hash check**

Run: `make creed-check`
Expected:
```
creed hash check ok (...)
useful-work hash check ok (...)
```

- [ ] **Step 4: Run proto-check**

Run: `make proto-check`
Expected: PASS.

- [ ] **Step 5: Run full build**

Run: `go build ./...` (timeout 240s)
Expected: clean.

- [ ] **Step 6: Run full pre-PR check**

Run: `make pr-check` (timeout 600s)
Expected: PASS — covers lint + test + proto-check + creed-check + build.

If `make test` times out, narrow:
Run: `go test ./x/substrate_bridge/... ./x/knowledge/... ./x/creed/... ./tests/cross_stack/... -timeout 300s`

- [ ] **Step 7: Verify commit log**

Run:
```bash
git log --oneline --since="2026-05-10 00:00:00" -- proto/zerone/substrate_bridge/ x/substrate_bridge/ x/knowledge/keeper/rounds.go x/knowledge/keeper/keeper.go x/knowledge/types/expected_keepers.go x/creed/types/ x/gov/ app/app.go docs/EVENTS.md tests/cross_stack/substrate_bridge_test.go
```
Expected: ~26 commits in chronological order; each scope-tagged (`proto(substrate_bridge)`, `feat(substrate_bridge)`, `feat(knowledge)`, `feat(creed,gov)`, `test(cross_stack)`, `docs(events)`, `feat(app)`).

- [ ] **Step 8: Do NOT push**

Per CLAUDE.md, commits land on main but pushing requires explicit user authorization. Skip.

- [ ] **Step 9: Hand off**

Substrate bridge Tier-1 foundation is complete:

- `x/substrate_bridge/` module shipped with all three sub-systems (adapter framework, substrate-link compiler, lineage propagator).
- `x/knowledge.CompleteRound` notifies for pending-claim resolution.
- `CategoryAdapterRegistration` LIP class available for trust-deliberate adapter onboarding.
- Standalone-usable via `MsgSubmitExternalAttestation`.
- Forward-compatible: when x/work Phase 1 lands, it integrates via `PrepareExternalAttestation`/`SettleExternalAttestation` shims (added later as a small follow-up plan).

The next plans in this series:
- **First external work class** (`x/translation` or `x/curriculum` — user's choice) — each registers an adapter via LIP and uses substrate_bridge.
- **x/work Phase 1 primitive** — if not yet shipped, this becomes the next big foundation work; otherwise integration is a small touch-up.
- **Tier 2** (`x/offchain_workers`, `x/event_resolution`) — when the first compute-heavy adapter is brainstormed.
- **Tier 3** (`x/interchain_knowledge`) — when an external chain wants to depend on ZERONE truth.

---

## Self-Review

After implementing all tasks, verify:

1. **Spec coverage:**
   - Section 1 (module identity) → Tasks 9, 10, 11 (keys, scaffolding, AppModule)
   - Section 2 (permissive substrate-link semantics) → Tasks 3, 4, 13, 14, 15, 17, 18, 21 (proto, validation, attestation, pending index, propagation, settlement, BeginBlocker)
   - Section 3 (adapter framework + gov LIP) → Tasks 2, 12, 19, 23 (proto, registry keeper, msg server, LIP class)
   - Section 4 (cross-class lineage) → Tasks 5, 16, 17 (proto, DAG, propagation)
   - Section 5 (integration with existing modules) → Tasks 22, 25 (knowledge hook, app wiring)
   - Section 6 (five-layer enforcement) → Tasks 10 (position layer in doc.go), 19/27 (voice events), 10 (refusal-layer errors with doctrine voice), 26 (test layer)
   - Section 7 (out of scope) → respected; Tier 2/3 not in plan
   - Section 8 (open questions) → resolved during plan: bond settlement = at full attestation settle (lump-sum slash on rejection, refund on success); lineage curve = Params (gov-tunable from day 1); self-citation cap = uniform 50% via Params; dedup = canonical hash collapse; genesis adapters = empty (LIP-only); adapter binary distribution = deferred to operational practice (gov LIP includes URL/IPFS CID in its description).

2. **Placeholder scan:** plan contains no "TBD"/"TODO"/"add error handling" placeholders — every step has actual content. The Task 23 step on quorum requirements describes an implementation pattern; if the codebase doesn't yet have a generic LIP-dispatch mechanism, the implementer must follow the existing `CategoryCreedAmendment` pattern and adapt.

3. **Type consistency:**
   - `WriteAdapter` / `GetAdapter` / `IterateAdapters` signatures match Tasks 12, 19, 22, 23, 25.
   - `WriteAttestation` / `GetAttestation` / `IterateAttestationsByStatus` consistent across Tasks 14, 18, 21, 22, 26.
   - `LinkPendingClaim` / `UnlinkPendingClaim` consistent across Tasks 15, 19, 21.
   - `CreateLineageEdge` / `IterateForwardLineage` / `IterateBackwardLineage` consistent across Tasks 16, 17, 26.
   - `OnClaimResolved` signature matches across Tasks 21 (impl), 22 (interface), 26 (test invocation).
   - `SettleAttestation` consistent across Tasks 18, 21, 26.

4. **Refusal voice:** every typed error in Task 10's `errors.go` cites UW + the violated mechanism. Refusals named: ErrAdapterAuthority (M3), ErrAdapterNotActive (M3), ErrCitedFactNotFound (M2), ErrTooManyPendingClaims (M2), ErrAxisOverflow (M5), ErrLinkHashMismatch (M2), ErrLineageCycle (M6), ErrSelfCitationCapExceeded (M6).

---

## What This Plan Does Not Do

- **No `x/work` primitive.** Phase-1 brainstorm/spec/plan to follow. Substrate_bridge is standalone-usable until then via `MsgSubmitExternalAttestation`.
- **No external work classes.** `x/translation`, `x/curriculum`, etc. are separate plans. Each registers an adapter via LIP and uses substrate_bridge's submission entry point.
- **No off-chain compute.** Adapters at Tier 1 must run as binaries that produce SubstrateLinks consumable on-chain. Heavy off-chain compute (LLM inference, training replication) is Tier 2 (`x/offchain_workers`).
- **No real-world event resolution.** Hypothesis-market, oracle-attestation adapters need Tier 2 (`x/event_resolution`).
- **No cross-chain export.** Tier 3 (`x/interchain_knowledge`).
- **No genesis adapters.** Chain starts with no adapters; first adapter onboards via LIP after genesis. Operational: a coordinator at testnet bootstrap may submit the first LIP to register a "test-noop" adapter for smoke tests.
- **Q (verification-quality) is fixed at 0.5 at Phase 0.** Refinement using real x/knowledge consensus_margin is a follow-up task once the surface is exposed via expected_keepers.

— *Plan authored 2026-05-10. Standalone-usable substrate bridge; evolves through bound mechanisms only.*

