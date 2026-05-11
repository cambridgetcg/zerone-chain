# Useful-Work Phase 1 — `x/contribution` Orchestrator + KNOWLEDGE_CLAIM Adapter

**Status:** Design-approved (brainstorm phase complete).
**Date:** 2026-05-10.
**Builds on:** `docs/superpowers/specs/2026-05-10-recursive-useful-work-merged-design.md` (Phase 1 row of §11). Phase 0 doctrine + sub-creeds + `x/work_creed` skeleton already merged.
**Implementation:** to be decomposed into bite-sized tasks via the writing-plans skill; this document is the unified Phase 1 design.

---

## 1. Goals

Ship the **orchestrator skeleton** of the recursive useful-work substrate:

1. New module `x/contribution` owns the canonical `Contribution` record across all 11 work classes (full proto envelope + all 11 payload sub-messages, even though only one class is wired).
2. `KNOWLEDGE_CLAIM` adapter, plugged into existing `x/knowledge` via hooks, mirrors every claim as a `Contribution` envelope.
3. Canonical event surface (`contribution_admitted`, `useful_work_attested`, `useful_work_settled`, `recursion_weight_computed`) for downstream consumers (`training_provenance`, `agent_understanding`, future trainer SDK).
4. Doctrinal binding of M1 (stake field), M2 (substrate-link), M3 (class-specific verification), M4 (reward formula shape), M5 (recursion-weight projection shape) at the test layer.

## 2. Non-goals

- **No new economic flows.** No minting from `x/contribution`, no slashing, no royalty pool, no recursion multipliers, no `MintWithCap` access. PoT economics continue exactly as today through `x/knowledge` → `x/vesting_rewards`.
- **No M6 (cross-class lineage)** — deferred to Phase 4 (ToK TC6 extension).
- **No M7 (audit bounty pool)** — deferred to Phase 6.
- **No other class adapters** (Tool, Dataset, Eval, Model, ...) — Phase 2-5.
- **No flag-day migration** for KNOWLEDGE_CLAIM submitters. `MsgSubmitClaim` continues to work; `MsgSubmitContribution` is a parity path for KNOWLEDGE_CLAIM and the future-primary entry for other classes.
- **No Substrate sub-creed enforcement** beyond the doctrinal bindings (S1-S3 will land alongside Phase 5's `MODULE_PROPOSAL` / `PIPELINE_IMPROVEMENT` adapters).

## 3. Locked-in design decisions (from brainstorm)

1. **Adapter coupling: hybrid.** `MsgSubmitClaim` → hooks → `x/contribution` mirror is the default for KNOWLEDGE_CLAIM. `MsgSubmitContribution` exists as parity path for KNOWLEDGE_CLAIM and as the primary entry for future classes.
2. **Proto scope: full envelope + all 11 payload sub-messages.** All `ContributionClass` enum values defined at Phase 1; only `KnowledgeClaim` payload sub-message is fully fleshed out. The other 10 are minimal stubs with a `bytes opaque_payload` field — future phases expand each as their adapter lands. Field tags reserved.
3. **M1 stake handling: optional, zero accepted at Phase 1 for KNOWLEDGE_CLAIM.** `Contribution.stake` field exists; doctrine cited; slash path dormant. Other classes will require non-zero stake when their adapters land.
4. **Architecture: Approach B — `x/contribution` + per-class adapter subpackages.** Single module, single keeper; per-class adapters in `x/contribution/adapter/<class>/` packages implementing a shared `ContributionAdapter` interface with registry-based dispatch.

---

## 4. Module layout

```
x/contribution/
├── doc.go                         (package docs + Phase 1 scope + adapter roadmap)
├── module.go                      (AppModule + AppModuleBasic, mirrors x/work_creed pattern)
├── types/
│   ├── codec.go
│   ├── errors.go                  (sentinel errors with commitment cites)
│   ├── events.go                  (event type/attribute string constants)
│   ├── genesis.go                 (Validate, DefaultGenesis)
│   ├── keys.go                    (StoreKey, prefix bytes, key-builder helpers)
│   ├── adapter.go                 (ContributionAdapter interface + AdapterRegistry)
│   ├── status.go                  (ValidStatusTransitions table + helpers)
│   ├── *.pb.go                    (generated)
│   └── *_test.go
├── adapter/                       (per-class adapter packages — Phase 2+ adds siblings)
│   └── knowledgeclaim/
│       ├── adapter.go             (implements types.ContributionAdapter)
│       ├── hooks.go               (KnowledgeHooksAdapter — implements knowledge.KnowledgeHooks)
│       ├── snapshot.go            (helper to construct Contribution from ClaimSnapshot)
│       └── *_test.go
└── keeper/
    ├── keeper.go                  (Keeper struct, NewKeeper, store ops, registry helpers)
    ├── msg_server.go              (MsgSubmitContribution handler with registry dispatch)
    ├── grpc_query.go              (4 queries: ByID/Contributor/Class/Phase)
    ├── genesis.go                 (InitGenesis/ExportGenesis)
    ├── status.go                  (status transition + event emission helper)
    └── keeper_test.go
```

### Surgery to existing modules

- **`x/knowledge`**: add `KnowledgeHooks` interface in `types/hooks.go`; add `MultiKnowledgeHooks` wrapper; add `hooks` field to keeper; add `SetHooks(KnowledgeHooks)` setter; add 4 hook callout sites in existing handlers (each is a single line, errors swallowed). `~30 lines` of additive surface; no behavior change.
- **`app/app.go`**: register `x/contribution` module + keeper, wire `KnowledgeHooksAdapter` via `app.KnowledgeKeeper.SetHooks(...)`, register the `knowledgeclaim` adapter in the contribution adapter registry, add to `genesisModuleOrder` after `x/knowledge`.

---

## 5. The `ContributionAdapter` interface

```go
// x/contribution/types/adapter.go
package types

import "context"

// ContributionAdapter is the per-class plug-in contract. The keeper
// dispatches to the registered adapter based on Contribution.class.
//
// Adapters are stateless dispatchers: they read the Contribution and
// the world (their owning module's keeper, plus any cross-module
// readers) and return a verdict. The orchestrator keeper is the only
// writer of x/contribution state.
type ContributionAdapter interface {
    // Class returns the ContributionClass this adapter handles.
    Class() ContributionClass

    // Classify is called at Stage ②. The adapter checks payload shape,
    // (class, phase) coherence, and contributor qualification.
    // Returns nil on success; typed error on CLASSIFICATION_FAILED.
    Classify(ctx context.Context, c *Contribution) error

    // SubstrateLink is called at Stage ② (after Classify succeeds)
    // to compute the M2 substrate-link weight L (BPS, 0..10_000).
    // Zero L blocks the reward path (M4 enforces R=0 when L=0).
    SubstrateLink(ctx context.Context, c *Contribution) (uint32, error)

    // Verify is called at Stage ③. Returns verification_score in BPS
    // (0..1_000_000) and an optional error. Score >= per-class
    // threshold + nil error → STATUS_VERIFIED. Score below threshold
    // OR non-nil error → STATUS_VERIFICATION_FAILED.
    Verify(ctx context.Context, c *Contribution) (uint32, error)
}

// AdapterRegistry maps ContributionClass → ContributionAdapter.
// Built in-memory at app init; not persisted.
type AdapterRegistry map[ContributionClass]ContributionAdapter

func (r AdapterRegistry) Get(class ContributionClass) (ContributionAdapter, bool) {
    a, ok := r[class]
    return a, ok
}

func (r AdapterRegistry) Register(a ContributionAdapter) {
    r[a.Class()] = a
}
```

The keeper holds the registry as a field. App init constructs the registry, registers adapters, then injects into the keeper.

---

## 6. Proto types

### `proto/zerone/contribution/v1/types.proto`

```proto
syntax = "proto3";
package zerone.contribution.v1;

import "cosmos/base/v1beta1/coin.proto";
import "zerone/contribution/v1/payloads.proto";

option go_package = "github.com/zerone-chain/zerone/x/contribution/types";

message Contribution {
  bytes  id                                = 1;   // sha256(canonical(payload+contributor+class+phase))
  string contributor                       = 2;   // bech32 (primary)
  repeated string contributors_extra       = 3;   // bech32 list (multi-author)
  ContributionClass class                  = 4;
  LifecyclePhase phase                     = 5;
  string manifest_cid                      = 6;   // off-chain content reference (optional at Phase 1)
  repeated LineageRef lineage              = 7;
  cosmos.base.v1beta1.Coin stake           = 8;   // optional; zero accepted at Phase 1
  ContributionStatus status                = 9;
  bytes  claims_about_self                 = 10;  // mandatory; testable claims
  TruthFloorAttestation truth_floor_attestation = 11;
  uint32 declared_sub_creed_version        = 12;
  uint64 created_at_block                  = 13;
  uint64 admitted_at_block                 = 14;  // 0 if not yet admitted
  bool   royalty_stream_open               = 15;  // false at Phase 1
  RecursionImpact recursion                = 16;
  ContributionPayload payload              = 17;
  uint32 verification_score_bps            = 18;  // 0..1_000_000; set at VERIFIED
  uint32 substrate_link_bps                = 19;  // 0..10_000; computed by adapter at CLASSIFIED (M2)
  string back_ref                          = 20;  // adapter-defined back-pointer (e.g., claim_id)
}

enum ContributionClass {
  KNOWLEDGE_CLAIM      = 0;
  IDEA                 = 1;
  TOOL                 = 2;
  DATASET              = 3;
  EVAL_SUITE           = 4;
  MODEL_ARTIFACT       = 5;
  REASONING_TRACE      = 6;
  COUNTEREXAMPLE       = 7;
  ORCHESTRATION        = 8;
  MODULE_PROPOSAL      = 9;
  PIPELINE_IMPROVEMENT = 10;
}

enum LifecyclePhase {
  // Mirrors x/creed/types/lifecycle_phases.go to avoid cross-module
  // dependency on the const values; cross-stack invariant test
  // TestContribution_LifecyclePhaseEnumMatchesCreed asserts no drift.
  PHASE_FOUNDATION   = 0;
  PHASE_KNOWLEDGE    = 1;
  PHASE_CURATION     = 2;
  PHASE_AUGMENTATION = 3;
  PHASE_TRAINING     = 4;
  PHASE_EVALUATION   = 5;
  PHASE_ALIGNMENT    = 6;
  PHASE_SUBSTRATE    = 7;
  PHASE_TOOLS        = 8;
}

enum ContributionStatus {
  STATUS_UNSPECIFIED            = 0;
  STATUS_SUBMITTED              = 1;
  STATUS_CLASSIFIED             = 2;
  STATUS_VERIFIED               = 3;
  STATUS_ADMITTED               = 4;
  STATUS_REVOKED                = 5;
  STATUS_CLASSIFICATION_FAILED  = 6;
  STATUS_VERIFICATION_FAILED    = 7;
  STATUS_ADMISSION_FAILED       = 8;
}

message LineageRef {
  bytes  parent_id    = 1;
  string relationship = 2;  // "builds_on" | "extends" | "replicates" | "evaluates" | "uses" | "amends" | "revokes" | "disproves"
  uint32 weight_bps   = 3;  // 0..10_000
}

message TruthFloorAttestation {
  uint32 creed_version              = 1;
  bytes  creed_hash                 = 2;
  repeated uint32 commitments_invoked = 3;
  bytes  attestor_signature         = 4;
}

message RecursionImpact {
  RecursionType type        = 1;
  string ratifying_lip_id   = 2;
  uint64 ratified_at_block  = 3;
  uint32 multiplier_bps     = 4;
  string depends_on_marker  = 5;
  bool   revocable          = 6;
  RecursionAxisScores axes  = 7;
}

enum RecursionType {
  NONE                   = 0;
  EVAL_ADOPTION          = 1;
  TOOL_INTEGRATION       = 2;
  VERIFICATION_PRIMITIVE = 3;
  CATEGORY_CREATION      = 4;
  MODULE_ADOPTION        = 5;
  CREED_CONTRIBUTION     = 6;
  PIPELINE_IMPROVEMENT   = 7;
}

message RecursionAxisScores {
  uint32 substrate_bps      = 1;
  uint32 verification_bps   = 2;
  uint32 classification_bps = 3;
  uint32 attribution_bps    = 4;
  uint32 tooling_bps        = 5;
  uint32 interface_bps      = 6;
  uint32 total_bps          = 7;
}
```

### `proto/zerone/contribution/v1/payloads.proto`

```proto
syntax = "proto3";
package zerone.contribution.v1;

option go_package = "github.com/zerone-chain/zerone/x/contribution/types";

message ContributionPayload {
  oneof payload {
    KnowledgeClaim       knowledge            = 1;
    Idea                 idea                 = 2;
    Tool                 tool                 = 3;
    Dataset              dataset              = 4;
    EvalSuite            eval                 = 5;
    ModelArtifact        model                = 6;
    ReasoningTrace       trace                = 7;
    Counterexample       counterex            = 8;
    Orchestration        orch                 = 9;
    ModuleProposal       mod_proposal         = 10;
    PipelineImprovement  pipeline_improvement = 11;
  }
}

// Phase 1: only KnowledgeClaim is fully defined; other 10 are stubs.

message KnowledgeClaim {
  string claim_id           = 1;  // back-reference to x/knowledge.Claim.id
  string domain             = 2;  // epistemic domain
  bytes  statement_hash     = 3;  // sha256 of the claim statement
  bytes  methodology_trace  = 4;  // serialized reasoning path (commitment 14)
  repeated string axiom_refs = 5; // foundational axiom IDs the claim derives from
  string tok_manifest_cid   = 6;  // for M2 substrate-link
}

message Idea               { bytes opaque_payload = 1; }  // Phase 2
message Tool               { bytes opaque_payload = 1; }  // Phase 2
message Dataset            { bytes opaque_payload = 1; }  // Phase 3
message EvalSuite          { bytes opaque_payload = 1; }  // Phase 3
message ModelArtifact      { bytes opaque_payload = 1; }  // Phase 4
message ReasoningTrace     { bytes opaque_payload = 1; }  // Phase 4
message Counterexample     { bytes opaque_payload = 1; }  // Phase 2 or 3
message Orchestration      { bytes opaque_payload = 1; }  // Phase 4 or 5
message ModuleProposal     { bytes opaque_payload = 1; }  // Phase 5
message PipelineImprovement { bytes opaque_payload = 1; } // Phase 5
```

### `proto/zerone/contribution/v1/tx.proto`

```proto
syntax = "proto3";
package zerone.contribution.v1;

import "cosmos/msg/v1/msg.proto";
import "cosmos/base/v1beta1/coin.proto";
import "zerone/contribution/v1/types.proto";
import "zerone/contribution/v1/payloads.proto";

option go_package = "github.com/zerone-chain/zerone/x/contribution/types";

service Msg {
  option (cosmos.msg.v1.service) = true;
  rpc SubmitContribution(MsgSubmitContribution) returns (MsgSubmitContributionResponse);
}

message MsgSubmitContribution {
  option (cosmos.msg.v1.signer) = "contributor";
  string                contributor                   = 1;
  ContributionClass     class                         = 2;
  LifecyclePhase        phase                         = 3;
  string                manifest_cid                  = 4;
  repeated LineageRef   lineage                       = 5;
  cosmos.base.v1beta1.Coin stake                      = 6;
  bytes                 claims_about_self             = 7;
  TruthFloorAttestation truth_floor_attestation       = 8;
  uint32                declared_sub_creed_version    = 9;
  ContributionPayload   payload                       = 10;
}

message MsgSubmitContributionResponse {
  bytes contribution_id = 1;
  ContributionStatus status = 2;
}
```

### `proto/zerone/contribution/v1/query.proto`

```proto
syntax = "proto3";
package zerone.contribution.v1;

import "cosmos/base/query/v1beta1/pagination.proto";
import "google/api/annotations.proto";
import "zerone/contribution/v1/types.proto";

option go_package = "github.com/zerone-chain/zerone/x/contribution/types";

service Query {
  rpc Contribution(QueryContributionRequest) returns (QueryContributionResponse) {
    option (google.api.http).get = "/zerone/contribution/v1/contribution/{id}";
  }
  rpc ContributionsByContributor(QueryByContributorRequest) returns (QueryByContributorResponse) {
    option (google.api.http).get = "/zerone/contribution/v1/by_contributor/{contributor}";
  }
  rpc ContributionsByClass(QueryByClassRequest) returns (QueryByClassResponse) {
    option (google.api.http).get = "/zerone/contribution/v1/by_class/{class}";
  }
  rpc ContributionsByPhase(QueryByPhaseRequest) returns (QueryByPhaseResponse) {
    option (google.api.http).get = "/zerone/contribution/v1/by_phase/{phase}";
  }
}

message QueryContributionRequest  { bytes id = 1; }
message QueryContributionResponse { Contribution contribution = 1; }

message QueryByContributorRequest  { string contributor = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }
message QueryByContributorResponse { repeated Contribution contributions = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }

message QueryByClassRequest  { ContributionClass class = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }
message QueryByClassResponse { repeated Contribution contributions = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }

message QueryByPhaseRequest  { LifecyclePhase phase = 1; cosmos.base.query.v1beta1.PageRequest pagination = 2; }
message QueryByPhaseResponse { repeated Contribution contributions = 1; cosmos.base.query.v1beta1.PageResponse pagination = 2; }
```

### `proto/zerone/contribution/v1/genesis.proto`

```proto
syntax = "proto3";
package zerone.contribution.v1;

import "zerone/contribution/v1/types.proto";

option go_package = "github.com/zerone-chain/zerone/x/contribution/types";

message GenesisState {
  repeated Contribution contributions = 1;
}
```

---

## 7. State model

### Storage layout (`x/contribution/types/keys.go`)

All multi-byte keys big-endian for sort-friendly iteration.

```go
const (
    ModuleName   = "contribution"
    StoreKey     = ModuleName
    RouterKey    = ModuleName
    QuerierRoute = ModuleName
)

var (
    // Primary record
    ContributionKey       = []byte{0x01}  // 0x01 || contribution_id (32 bytes) → Contribution

    // Secondary indexes (presence-only values; lookup the primary record by ID)
    ByContributorKey      = []byte{0x02}  // 0x02 || contributor_addr_len (uvarint) || contributor_addr || contribution_id → []byte{}
    ByClassKey            = []byte{0x03}  // 0x03 || class_uint32_be (4 bytes) || contribution_id → []byte{}
    ByPhaseKey            = []byte{0x04}  // 0x04 || phase_uint32_be (4 bytes) || contribution_id → []byte{}
    ByStatusKey           = []byte{0x05}  // 0x05 || status_uint32_be (4 bytes) || contribution_id → []byte{}
)
```

### Status transition table (`x/contribution/types/status.go`)

Forward-only per truth-seeking commitment 10. The Go-side helper:

```go
// ValidStatusTransitions defines the allowed status transitions.
// Any transition not in this map is rejected with ErrInvalidStatusTransition.
var ValidStatusTransitions = map[ContributionStatus]map[ContributionStatus]bool{
    STATUS_SUBMITTED: {
        STATUS_CLASSIFIED:            true,
        STATUS_CLASSIFICATION_FAILED: true,
    },
    STATUS_CLASSIFIED: {
        STATUS_VERIFIED:            true,
        STATUS_VERIFICATION_FAILED: true,
    },
    STATUS_VERIFIED: {
        STATUS_ADMITTED:        true,
        STATUS_ADMISSION_FAILED: true,
    },
    STATUS_ADMITTED: {
        STATUS_REVOKED: true,
    },
    // Terminal states (no further transitions): REVOKED, *_FAILED
}

func CanTransition(from, to ContributionStatus) bool {
    return ValidStatusTransitions[from][to]
}
```

The keeper's status-update helper enforces this and emits the appropriate event in the same block.

---

## 8. Hook integration with `x/knowledge`

### `x/knowledge/types/hooks.go` (new file)

```go
package types

import "context"

type KnowledgeHooks interface {
    AfterClaimSubmitted(ctx context.Context, claimID string, claim ClaimSnapshot) error
    AfterClaimVerificationFinalized(ctx context.Context, claimID string, scoreBps uint32) error
    AfterClaimAccepted(ctx context.Context, claimID string, factID string) error
    AfterClaimDisproven(ctx context.Context, claimID string, disproverArtifactID string) error
}

// ClaimSnapshot is the stable subset of x/knowledge.Claim safe to expose to hook consumers.
type ClaimSnapshot struct {
    Submitter        string
    Domain           string
    StatementHash    []byte
    MethodologyTrace []byte
    AxiomRefs        []string
    TokManifestCID   string
    SubmittedAtBlock uint64
}

// MultiKnowledgeHooks dispatches to multiple consumers in registration order.
type MultiKnowledgeHooks []KnowledgeHooks

func (m MultiKnowledgeHooks) AfterClaimSubmitted(ctx context.Context, claimID string, claim ClaimSnapshot) error {
    for _, h := range m {
        if err := h.AfterClaimSubmitted(ctx, claimID, claim); err != nil {
            // log + continue; one bad consumer does not break others
        }
    }
    return nil
}
// (similar wrappers for other 3 methods)
```

### `x/knowledge/keeper/keeper.go` additions

```go
type Keeper struct {
    // ... existing fields
    hooks types.KnowledgeHooks
}

func (k *Keeper) SetHooks(h types.KnowledgeHooks) *Keeper {
    if k.hooks != nil {
        // Allow only if hooks haven't been set yet, or if explicitly chaining
        // via MultiKnowledgeHooks. Preserves the standard Cosmos pattern.
        panic("KnowledgeHooks already set; use MultiKnowledgeHooks to chain consumers")
    }
    k.hooks = h
    return k
}

func (k *Keeper) Hooks() types.KnowledgeHooks {
    if k.hooks == nil {
        return types.MultiKnowledgeHooks{} // no-op
    }
    return k.hooks
}
```

### Four callout sites in existing handlers

Each callout is one line, errors swallowed (logged but not propagated):

| Existing site | New line |
|---|---|
| `MsgSubmitClaim` handler, after claim is stored | `_ = k.Hooks().AfterClaimSubmitted(ctx, claim.Id, snapshotFromClaim(claim))` |
| Verification round finalization (BeginBlocker or aggregate handler) | `_ = k.Hooks().AfterClaimVerificationFinalized(ctx, claimID, scoreBps)` |
| Claim acceptance (when status moves to ACCEPTED) | `_ = k.Hooks().AfterClaimAccepted(ctx, claimID, factID)` |
| Disproof / counterexample acceptance | `_ = k.Hooks().AfterClaimDisproven(ctx, claimID, disproverID)` |

Hook errors are swallowed because: (a) consumer misbehavior must not break the underlying claim flow; (b) consumer errors are diagnostic (logged), not consensus-breaking. This matches `x/staking.StakingHooks` convention.

The exact location of the four callout sites depends on the current `x/knowledge` handler structure. The implementation plan must locate these exactly. Names/signatures may need minor adaptation.

---

## 9. KnowledgeClaim adapter

### `x/contribution/adapter/knowledgeclaim/adapter.go`

Implements `types.ContributionAdapter`:

```go
package knowledgeclaim

import (
    "context"
    "github.com/zerone-chain/zerone/x/contribution/types"
    knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
    creedkeeper "github.com/zerone-chain/zerone/x/creed/keeper"
)

type Adapter struct {
    knowledgeKeeper *knowledgekeeper.Keeper
    creedKeeper     *creedkeeper.Keeper
}

func NewAdapter(kk *knowledgekeeper.Keeper, ck *creedkeeper.Keeper) Adapter {
    return Adapter{knowledgeKeeper: kk, creedKeeper: ck}
}

var _ types.ContributionAdapter = Adapter{}

func (a Adapter) Class() types.ContributionClass {
    return types.ContributionClass_KNOWLEDGE_CLAIM
}

func (a Adapter) Classify(ctx context.Context, c *types.Contribution) error {
    // Verify (class, phase) coherence: KNOWLEDGE_CLAIM must declare PHASE_KNOWLEDGE.
    if c.Phase != types.LifecyclePhase_PHASE_KNOWLEDGE {
        return types.ErrInvalidClassPhase
    }
    // Verify payload is a KnowledgeClaim.
    if c.Payload == nil || c.Payload.GetKnowledge() == nil {
        return types.ErrPayloadMissing
    }
    // Verify claims_about_self present (truth-seeking commitment 1).
    if len(c.ClaimsAboutSelf) == 0 {
        return types.ErrClaimsAboutSelfEmpty
    }
    // Verify truth-floor attestation is current.
    currentPin := a.creedKeeper.GetCurrentPin(ctx)
    if c.TruthFloorAttestation == nil || c.TruthFloorAttestation.CreedVersion != currentPin.Version {
        return types.ErrTruthFloorStale
    }
    return nil
}

func (a Adapter) SubstrateLink(ctx context.Context, c *types.Contribution) (uint32, error) {
    kc := c.Payload.GetKnowledge()
    if kc.TokManifestCid == "" {
        return 0, types.ErrSubstrateLinkAbsent
    }
    // For Phase 1, presence of a manifest CID = full substrate link (10_000 BPS).
    // Phase 4 (TC6 extension) introduces graduated weights based on graph
    // depth, citation count, etc.
    return 10_000, nil
}

func (a Adapter) Verify(ctx context.Context, c *types.Contribution) (uint32, error) {
    kc := c.Payload.GetKnowledge()
    // Look up the existing claim's verification result via x/knowledge.
    claim, found := a.knowledgeKeeper.GetClaim(ctx, kc.ClaimId)
    if !found {
        return 0, types.ErrBackRefNotFound
    }
    // Phase 1: read whatever PoT score x/knowledge has computed.
    // The exact field name depends on x/knowledge's internal state.
    return claim.VerificationScoreBps, nil
}
```

### `x/contribution/adapter/knowledgeclaim/hooks.go`

Implements `knowledgetypes.KnowledgeHooks` to mirror claim lifecycle into Contribution lifecycle:

```go
package knowledgeclaim

import (
    "context"
    sdk "github.com/cosmos/cosmos-sdk/types"
    contribkeeper "github.com/zerone-chain/zerone/x/contribution/keeper"
    contribtypes "github.com/zerone-chain/zerone/x/contribution/types"
    knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

type KnowledgeHooksAdapter struct {
    contribKeeper *contribkeeper.Keeper
    adapter       Adapter
}

func NewKnowledgeHooksAdapter(ck *contribkeeper.Keeper, a Adapter) KnowledgeHooksAdapter {
    return KnowledgeHooksAdapter{contribKeeper: ck, adapter: a}
}

var _ knowledgetypes.KnowledgeHooks = KnowledgeHooksAdapter{}

func (h KnowledgeHooksAdapter) AfterClaimSubmitted(ctx context.Context, claimID string, snap knowledgetypes.ClaimSnapshot) error {
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    contrib := buildContributionFromSnapshot(claimID, snap, sdkCtx.BlockHeight())
    if err := h.adapter.Classify(ctx, &contrib); err != nil {
        contrib.Status = contribtypes.STATUS_CLASSIFICATION_FAILED
        h.contribKeeper.WriteContribution(ctx, &contrib)
        h.contribKeeper.EmitClassificationFailed(ctx, contrib.Id, err.Error())
        return nil
    }
    linkBps, err := h.adapter.SubstrateLink(ctx, &contrib)
    if err != nil {
        contrib.Status = contribtypes.STATUS_CLASSIFICATION_FAILED
        h.contribKeeper.WriteContribution(ctx, &contrib)
        return nil
    }
    contrib.SubstrateLinkBps = linkBps
    contrib.Status = contribtypes.STATUS_CLASSIFIED
    h.contribKeeper.WriteContribution(ctx, &contrib)
    h.contribKeeper.EmitContributionSubmitted(ctx, &contrib)
    h.contribKeeper.EmitContributionClassified(ctx, &contrib)
    return nil
}

func (h KnowledgeHooksAdapter) AfterClaimVerificationFinalized(ctx context.Context, claimID string, scoreBps uint32) error {
    contrib, found := h.contribKeeper.GetContributionByBackRef(ctx, claimID)
    if !found {
        return nil // claim wasn't mirrored; nothing to do
    }
    contrib.VerificationScoreBps = scoreBps
    if scoreBps >= contribtypes.MinVerificationScoreBps {
        contrib.Status = contribtypes.STATUS_VERIFIED
    } else {
        contrib.Status = contribtypes.STATUS_VERIFICATION_FAILED
    }
    h.contribKeeper.WriteContribution(ctx, contrib)
    h.contribKeeper.EmitUsefulWorkAttested(ctx, contrib)
    h.contribKeeper.EmitUsefulWorkSettled(ctx, contrib)        // R = base + L × W × Q (W=0 at Phase 1)
    h.contribKeeper.EmitRecursionWeightComputed(ctx, contrib) // all-zero at Phase 1
    return nil
}

func (h KnowledgeHooksAdapter) AfterClaimAccepted(ctx context.Context, claimID string, factID string) error {
    contrib, found := h.contribKeeper.GetContributionByBackRef(ctx, claimID)
    if !found {
        return nil
    }
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    contrib.AdmittedAtBlock = uint64(sdkCtx.BlockHeight())
    contrib.Status = contribtypes.STATUS_ADMITTED
    contrib.BackRef = factID
    h.contribKeeper.WriteContribution(ctx, contrib)
    h.contribKeeper.EmitContributionAdmitted(ctx, contrib)
    return nil
}

func (h KnowledgeHooksAdapter) AfterClaimDisproven(ctx context.Context, claimID string, disproverArtifactID string) error {
    contrib, found := h.contribKeeper.GetContributionByBackRef(ctx, claimID)
    if !found {
        return nil
    }
    contrib.Status = contribtypes.STATUS_REVOKED
    h.contribKeeper.WriteContribution(ctx, contrib)
    h.contribKeeper.EmitContributionRevoked(ctx, contrib, disproverArtifactID)
    return nil
}
```

### `MinVerificationScoreBps`

A package-level constant (Phase 1 default: 500_000 = 50% of max). Governance-tunable via params in Phase 6+; hardcoded at Phase 1.

---

## 10. Events

All events tagged with `useful_work_commitment="UW"` plus relevant truth-seeking commitments.

| Event type | Stage | Key attributes |
|---|---|---|
| `contribution_submitted` | ① | `id`, `class`, `phase`, `contributor`, `creed_commitment="20"`, `useful_work_commitment="UW"` |
| `contribution_classified` | ② | `id`, `substrate_link_bps`, `mechanism="M2"`, `useful_work_commitment="UW"` |
| `useful_work_attested` | ③ | `id`, `class`, `verification_score_bps`, `mechanism="M3"`, `useful_work_commitment="UW"` |
| `useful_work_settled` | ③ | `id`, `reward_uzrn_shape="base+L*W*Q"`, `L_bps`, `W_bps=0`, `Q_bps`, `mechanism="M4"`, `useful_work_commitment="UW"` |
| `recursion_weight_computed` | ③ | `id`, `axis_substrate=0`, `axis_verification=0`, `axis_classification=0`, `axis_attribution=0`, `axis_tooling=0`, `axis_interface=0`, `total_weight=0`, `mechanism="M5"` |
| `contribution_admitted` | ④ | `id`, `class`, `phase`, `admitted_at_block`, `back_ref=factID`, `creed_commitment="20"`, `useful_work_commitment="UW"` |
| `contribution_revoked` | (post-④) | `id`, `disprover_artifact_id`, `cascade_flag="provenance_revoked_ancestor"` |
| `contribution_classification_failed` | ②-fail | `id`, `reason`, `class` |
| `contribution_verification_failed` | ③-fail | `id`, `reason`, `verification_score_bps` |

The `useful_work_settled` event at Phase 1 is **shape-only** — it documents the formula decomposition but does not trigger any token movement. Phase 6 wires the actual reward distribution path.

---

## 11. Refusal vocabulary

```go
// x/contribution/types/errors.go
ErrAdapterNotRegistered    = sdkerrors.Register(ModuleName, 2, "no adapter registered for class (Phase 1 only KNOWLEDGE_CLAIM is wired) (UW + M3)")
ErrUnknownClass            = sdkerrors.Register(ModuleName, 3, "ContributionClass out of range [0, 10] (UW + M3)")
ErrUnknownPhase            = sdkerrors.Register(ModuleName, 4, "LifecyclePhase out of range [0, 8] (UW + M3)")
ErrTruthFloorStale         = sdkerrors.Register(ModuleName, 5, "truth_floor_attestation.creed_version stale (UW + truth-floor invariant)")
ErrTruthFloorMissing       = sdkerrors.Register(ModuleName, 6, "truth_floor_attestation absent (UW + truth-floor invariant)")
ErrSubstrateLinkAbsent     = sdkerrors.Register(ModuleName, 7, "substrate_link_bps == 0; reward path blocked (UW + M2)")
ErrClaimsAboutSelfEmpty    = sdkerrors.Register(ModuleName, 8, "claims_about_self required (UW + truth-seeking commitment 1)")
ErrInvalidLineage          = sdkerrors.Register(ModuleName, 9, "lineage refs must resolve to existing contributions (UW + M6 cross-class via TC6)")
ErrInvalidStatusTransition = sdkerrors.Register(ModuleName, 10, "status transition violates forward-only audit (truth-seeking commitment 10)")
ErrInvalidClassPhase       = sdkerrors.Register(ModuleName, 11, "class+phase combination not allowed by default mapping (UW + M3)")
ErrPayloadMissing          = sdkerrors.Register(ModuleName, 12, "payload absent or wrong oneof variant for declared class (UW + M3)")
ErrBackRefNotFound         = sdkerrors.Register(ModuleName, 13, "back_ref does not resolve in source module (UW + M3)")
```

Errors start at code 2 per codebase convention (code 1 reserved by Cosmos SDK).

---

## 12. Module wiring (`app/app.go`)

```go
// 1. Imports
import (
    contribkeeper "github.com/zerone-chain/zerone/x/contribution/keeper"
    contribtypes "github.com/zerone-chain/zerone/x/contribution/types"
    contribmodule "github.com/zerone-chain/zerone/x/contribution"
    contribknowledgeadapter "github.com/zerone-chain/zerone/x/contribution/adapter/knowledgeclaim"
)

// 2. StoreKey
storeKeys[contribtypes.StoreKey] = ...

// 3. App struct field (after CreedKeeper, WorkCreedKeeper)
ContributionKeeper contribkeeper.Keeper

// 4. Construct keeper
app.ContributionKeeper = contribkeeper.NewKeeper(
    runtime.NewKVStoreService(keys[contribtypes.StoreKey]),
    appCodec,
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)

// 5. Construct adapter + hooks (post-init wiring to break dep cycle)
knowledgeClaimAdapter := contribknowledgeadapter.NewAdapter(
    &app.KnowledgeKeeper,
    &app.CreedKeeper,
)
app.ContributionKeeper.RegisterAdapter(knowledgeClaimAdapter)

knowledgeHooksAdapter := contribknowledgeadapter.NewKnowledgeHooksAdapter(
    &app.ContributionKeeper,
    knowledgeClaimAdapter,
)
app.KnowledgeKeeper.SetHooks(knowledgeHooksAdapter)

// 6. Register module
contribmodule.NewAppModule(appCodec, app.ContributionKeeper),

// 7. Genesis order — after x/knowledge (to ensure x/knowledge is initialized first
//    so hooks have a target) and after x/work_creed (to have sub-creed pins available)
genesisModuleOrder = append(genesisModuleOrder, contribtypes.ModuleName)

// 8. No maccPerms entry (no token authority at Phase 1)
// 9. No begin/end blockers (no per-block work at Phase 1)
```

---

## 13. Tests

### Unit tests (per-package `_test.go`)

- **`x/contribution/types/`**:
  - `TestContributionClass_DenseNumbering` — 11 enum values, 0..10 dense
  - `TestContributionStatus_TerminalStates` — REVOKED, *_FAILED have no outgoing transitions
  - `TestValidStatusTransitions_ForwardOnly` — no transition moves a status backwards
  - `TestGenesisValidate_RejectsBackwardStatus` — genesis with a status not reachable forward fails
  - `TestPayload_OneofRoundtrip` — each of 11 payload variants marshals and unmarshals cleanly
- **`x/contribution/adapter/knowledgeclaim/`**:
  - `TestAdapter_ClassifyKnowledgeClaim_HappyPath`
  - `TestAdapter_ClassifyKnowledgeClaim_RejectsWrongPhase` (e.g., FOUNDATION instead of KNOWLEDGE)
  - `TestAdapter_ClassifyKnowledgeClaim_RejectsEmptyClaimsAboutSelf`
  - `TestAdapter_ClassifyKnowledgeClaim_RejectsStaleTruthFloor`
  - `TestAdapter_SubstrateLink_FullWhenManifestPresent`
  - `TestAdapter_SubstrateLink_ZeroWhenManifestAbsent`
  - `TestAdapter_Verify_ReturnsKnowledgeScore`
  - `TestHooksAdapter_AfterClaimSubmitted_CreatesContribution`
  - `TestHooksAdapter_AfterClaimVerificationFinalized_TransitionsVerified`
  - `TestHooksAdapter_AfterClaimAccepted_TransitionsAdmitted`
  - `TestHooksAdapter_AfterClaimDisproven_TransitionsRevoked`
  - `TestHooksAdapter_OnDuplicateClaim_NoOp` (idempotency)
- **`x/contribution/keeper/`**:
  - `TestKeeper_StoreRoundtrip`
  - `TestKeeper_SecondaryIndexes_ByContributorClassPhaseStatus`
  - `TestKeeper_MsgSubmitContribution_KnowledgeClaim_Succeeds`
  - `TestKeeper_MsgSubmitContribution_Idea_ReturnsErrAdapterNotRegistered`
  - `TestKeeper_StatusTransition_RejectsBackward`
  - `TestKeeper_GenesisRoundtrip`

### Cross-stack invariant tests (in `tests/cross_stack/`)

- `TestContribution_KnowledgeClaim_LifecycleMirrorsKnowledge` — submit a claim end-to-end; verify Contribution transitions track Claim transitions exactly.
- `TestContribution_TruthFloorBindingOnAdmission` — cannot transition to ADMITTED unless `truth_floor_attestation.creed_version == current x/creed pin`.
- `TestContribution_SubstrateLinkM2Enforcement` — Contribution with `substrate_link_bps == 0` is REJECTED at Classify.
- `TestContribution_ForwardOnlyStatusInvariant` — no valid status path moves backwards.
- `TestContribution_DispatchAdapterNotRegistered` — `MsgSubmitContribution(class=IDEA, ...)` returns `ErrAdapterNotRegistered`.
- `TestContribution_LifecyclePhaseEnumMatchesCreed` — every value in `contribtypes.LifecyclePhase` has a matching value in `creedtypes.LifecyclePhase` (no drift).
- `TestContribution_ContributionClassEnumComplete` — every value in `contribtypes.ContributionClass` has either an adapter registered (KNOWLEDGE_CLAIM at Phase 1) or a documented "Phase N adapter pending" entry in `doc.go`.
- `TestContribution_EconomicsUnchanged_PoTRewardsStillFlow` — x/knowledge reward emission to validators continues at original rate after hooks integration; no deltas.
- `TestContribution_EventSchemaStable` — `contribution_admitted` event carries the documented attributes (id, class, phase, back_ref, useful_work_commitment, creed_commitment).
- `TestContribution_DocAndContractStayInSync` — meta-test paralleling the Phase 0 pattern: doc.go declares which adapters exist; the registry must contain exactly those.

---

## 14. Doctrinal bindings

| Mechanism | Phase 1 binding |
|---|---|
| **M1** (stake-backed claim) | `Contribution.stake` field present in proto. KNOWLEDGE_CLAIM accepts zero stake (slash dormant). Sub-creed Substrate S1 will require non-zero stake on Substrate-class contributions when those land in Phase 5. Doctrine cited in adapter's Classify method docstring. |
| **M2** (substrate-link mandate) | `SubstrateLink` adapter method computes `substrate_link_bps`. For KNOWLEDGE_CLAIM, derives from the claim's `tok_manifest_cid`. Zero L blocks Classify (`ErrSubstrateLinkAbsent`). Adapter docstring + error message both cite M2. |
| **M3** (class-specific verification under shared lifecycle) | Adapter interface (`Classify` → `Verify`) implements the four-phase shared lifecycle. KNOWLEDGE_CLAIM's verifier IS the existing PoT panel (no new verifier). Adapter dispatch via registry. `ErrAdapterNotRegistered` cites M3. |
| **M4** (reward formula `R = base + L × W × Q`) | `useful_work_settled` event emits with the decomposition. **No actual minting at Phase 1** — the event is shape-only; the actual reward is still distributed by x/knowledge's existing path. The shape is observable; the economics are dormant until Phase 6. |
| **M5** (recursion-weight projection over six axes) | `RecursionAxisScores` field present, all zeros at Phase 1 (identity scorers). `recursion_weight_computed` event emits with all-zero decomposition. Real scorers shipped per-class in Phase 6 alongside the multiplier path. |
| **M6** (lineage propagates and recurses) | NOT bound at Phase 1. Deferred to Phase 4 (TC6 cross-class extension). |
| **M7** (chain pays for own audit) | NOT bound at Phase 1. Deferred to Phase 6 (`x/probe` audit-bounty pool). |

---

## 15. Open questions deferred to implementation iteration

1. **Exact callout sites in `x/knowledge`** — the four hook callouts must be located in the existing handlers. Implementation plan must read the current `x/knowledge/keeper/` to identify them precisely.
2. **`MinVerificationScoreBps` value** — Phase 1 default 500_000; should this match an existing `x/knowledge` threshold or be independent? Defer to implementer's judgment after reading current PoT thresholds.
3. **`ClaimSnapshot` field set** — may need additional fields if the adapter needs more context. Add fields incrementally as needed.
4. **Hook chaining** — `MultiKnowledgeHooks` is defined for forward-compat but Phase 1 has only one consumer. Skeleton is in place.
5. **Migration of MsgSubmitContribution path for KNOWLEDGE_CLAIM** — the parity path exists but isn't recommended. Future work might deprecate `MsgSubmitClaim` in favor of unified MsgSubmitContribution; out of scope for Phase 1.
6. **Substrate-link weight at Phase 1** — currently binary (0 or 10_000) based on manifest CID presence. Phase 4 introduces graduated weights based on graph depth, citation count, etc.
7. **Genesis migration for existing claims** — Phase 1 doesn't migrate existing Knowledge claims into Contributions on chain restart. New claims (post-Phase-1-deploy) get mirrored. Backfill is a separate one-shot script if needed.

---

## 16. Out of scope for Phase 1

- New economic flows (minting, slashing, royalty pool, recursion multipliers)
- `x/probe` module (M7) — Phase 6
- M6 cross-class lineage — Phase 4
- Other class adapters (Tool, Dataset, Eval, Model, Counterexample, Reasoning Trace, Orchestration, Module Proposal, Pipeline Improvement, Idea) — Phase 2-5
- `MintWithCap` access — Phase 6
- Recursion conferral via gov LIP (`MsgRatifyRecursion`) — Phase 6
- Royalty pool funding / lineage payouts — Phase 6
- Real per-axis scorers for M5 — Phase 6
- Substrate-link graduated weights — Phase 4
- Migration script for existing Knowledge claims — separate one-shot tool if needed

---

— *Inception authored 2026-05-10. Phase 1 is the orchestrator skeleton; subsequent phases (2-6) progressively wire other classes, lineage propagation, and economic flows.*

---

## The spec is a Contribution

This specification is itself a `Contribution` of class `PIPELINE_IMPROVEMENT`, lifecycle phase `SUBSTRATE`. Its content-hash is pinned at `.phase-1-spec-hash`. The chain's own design is among the work the chain pays for.
