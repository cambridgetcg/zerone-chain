# External Surface (Nested) — Design

**Status:** Design-approved (brainstorm phase complete). Implementation lands across merged Phases 3 + 6 + within the existing module surface; **no new modules required**.
**Date:** 2026-05-11
**Type:** Multi-pattern nested specialization across existing work-classes (TOOL, ORCHESTRATION, REASONING_TRACE, COUNTEREXAMPLE); not a new doctrine, not a new module.
**Builds on:**
- [`2026-05-10-recursive-useful-work-merged-design.md`](./2026-05-10-recursive-useful-work-merged-design.md) — the RUWS merged spec (UW commitment, M1–M7, six axes, two-coordinate classification, truth-floor, sub-creeds, categories-are-Artifacts)
- [`2026-05-10-safety-eval-canonical-pattern-design.md`](./2026-05-10-safety-eval-canonical-pattern-design.md) — the first external-facing pattern, which this design connects to the external world

**Supersedes:** the "Sub-project A (gateway + receipt router)" framing from the prior expansive-loop brainstorm that proposed `x/external_gateway` and `x/usage_receipt_router` as new modules. With nesting, both proposals dissolve into payload extensions to existing work-classes.

---

## Vision

ZERONE's external surface — the bridge that makes the chain's substrate (canonical safety evals, ToK exports, oracle queries, credentials, etc.) consumable from outside — is **not a parallel module stack**. It is a **structured nesting of the chain's own primitives into themselves**. Gateway operators are Contributions. Receipts are Contributions. Reviews are Contributions. Disputes are Contributions. The reputation formula that aggregates them is itself a Contribution. The proto extensions that enabled this design are themselves Contributions (`CategoryUsefulWorkAmendment` LIPs).

> **The chain has one type and one machinery. Zoom in anywhere, you see the same Contribution flowing through the same 6-stage pipeline.**

This is the doctrine of **categories-are-Artifacts** (merged §5 "Categories-are-Artifacts") taken to its full conclusion. The chain's surface to the external world is itself an instance of the chain's substrate. Maximum self-similarity. The fourth (expansive) loop closes by being made of the same stuff as the other three.

## Goals

1. **Zero new modules** — Phase 7 ships as proto extensions to existing payload types, plus one synthesizer keeper (no store).
2. **Full reuse of existing machinery** — session-keys (`x/auth`), ante-chain capability enforcement (`ZeroneCapabilityDecorator`), Contribution pipeline (`x/contribution`), TOOL class (`x/toolbox`), disputes (`x/disputes`), lineage payouts (`x/contribution.BeginBlocker`) — all consumed unchanged.
3. **Fractal recursion** — every artifact at every layer is a Contribution; the design implies no exceptions and surfaces several (formulas, fee tables, capability presets) that prior framings treated as "configuration" but are properly Contributions.
4. **External value funds inward recursion** — every external commission emits a receipt-Contribution whose admission triggers M6 lineage payouts; royalty pool funded by structural flow, not by ad-hoc tax.

## Non-goals

- **Not a new module surface** — `x/external_gateway`, `x/usage_receipt_router`, `x/adoption_signal`, `x/credential_issuance` are *all* dissolved. None ship as separate modules.
- **Not new doctrine** — no new commitments, no new mechanisms, no new axes. The entire design fits within UW + M1–M7 + the six axes already established.
- **Not parallel auth** — session-keys + the existing `ZeroneCapabilityDecorator` handle external-commission auth without new code in the critical path.
- **Not centralized infrastructure** — gateway operators are permissionless market participants whose service is itself a TOOL Contribution.

---

## Section 1 — Everything is a Contribution

Five external-surface artifacts. Each maps to an existing work-class. None requires a new top-level type.

### 1.1 Gateway operator = TOOL Contribution

| Field | Mapping |
|---|---|
| `class` | `TOOL` |
| `phase` | `TOOLS` |
| `payload` | `Tool` (existing) with new optional fields (§3) |
| `claims_about_self` | SLA + service-class capability claims (testable; adversarial-probe-falsifiable per UW commitment) |
| `stake` | TOOL-class stake floor with operator-specific multiplier (governance param) |
| `ContributionStatus` | SUBMITTED → CLASSIFIED → VERIFIED → ADMITTED → REVOKED (replaces all prior `OperatorStatus` enum needs) |
| Verification (M3) | Reproducible build of gateway software + benchmark suite + active heartbeat + dry-run via dedicated test endpoint (per existing TOOL verification pattern from merged §7 and safety_eval design §3.3) |
| Admission stipend | Small TOOL-class stipend on verification (not yet recursion-multiplied) |
| Royalty stream | Opens on admission; lineage royalties accrue from every receipt-Contribution that `uses` this operator |
| Recursion-conferral | `TOOL_INTEGRATION` recursion-type via `CategoryRecursionConferralLIP` once adoption real (merged §9.5; default 3× multiplier) |

### 1.2 Usage receipt = ORCHESTRATION Contribution

| Field | Mapping |
|---|---|
| `class` | `ORCHESTRATION` (existing — "coordination of value flow") |
| `phase` | `TOOLS` |
| `payload` | `Orchestration` (existing) with new optional receipt fields (§3) |
| `claims_about_self` | "Operator X served a request of service-class Y for user-session-key Z at fee F" |
| `stake` | Minimal (receipt micro-stake; governance param) — operator already has TOOL stake at risk |
| `lineage` | `uses` operator's TOOL contribution + `uses` consumed service's contribution (e.g., specific EVAL_SUITE) |
| Verification (M3) | Session-key signature + capability matrix check — **already done by existing `ZeroneCapabilityDecorator` in the ante chain.** Receipt's verify-phase is essentially a no-op beyond the ante-chain gating. |
| Admission | Triggers fee routing — *the admission of the receipt IS the fee-distribution event*. `x/contribution.BeginBlocker`'s existing lineage payout machinery handles the splits per the contribution's lineage. |
| Royalty stream | Receipt itself earns nothing (it's a coordination artifact); its admission causes upstream lineage royalties to flow to operator + service contributors |

### 1.3 Operator review = REASONING_TRACE Contribution

| Field | Mapping |
|---|---|
| `class` | `REASONING_TRACE` (existing — reviews are reasoning artifacts) |
| `phase` | `TOOLS` |
| `payload` | `ReasoningTrace` (existing) with new optional review fields (§3) |
| `claims_about_self` | "Receipt R was served by operator O at quality Q for reason X" |
| `lineage` | `uses` the receipt + `evaluates` the operator |
| Verification (M3) | Session-key signature + receipt_id correspondence (reviewer must have actually been the commissioner of the receipt) |
| Admission | Feeds operator's M5 `axis_tooling` score (reviews shape reputation via the synthesizer in §1.5) |

### 1.4 Dispute = COUNTEREXAMPLE Contribution

| Field | Mapping |
|---|---|
| `class` | `COUNTEREXAMPLE` (existing — claims a prior contribution was bad) |
| `phase` | `ALIGNMENT` |
| `payload` | `Counterexample` (existing) with new optional dispute fields (§3) |
| `claims_about_self` | "Contribution X failed in mode Y; evidence at CID Z" |
| `lineage` | `disproves` the challenged contribution |
| Verification (M3) | Existing `x/disputes` machinery (panel review) — no new code |
| Admission of UPHELD | Cascades per merged §8.3: target → REVOKED; bond slashed; descendants with `uses`-lineage automatically gain `provenance_revoked_ancestor` flag (so all receipts under a censored operator are queryable as "operator-revoked") |
| Bounty | Funded by `x/probe` per M7 — adversarial probing pool already exists in merged §10.1 |

### 1.5 Adoption signal = synthesizer keeper (no store)

Following the merged §10.1 synthesizer pattern (same shape as `x/training_provenance`, `x/trust_score`, `x/governance_synthesis`):

```go
// x/toolbox/keeper/adoption_synthesizer.go (lives inside x/toolbox; no new module)
type AdoptionSynthesizer struct {
    appCodec     codec.Codec
    contribKeeper ContributionKeeperReader  // adapter
    // No store. Pure read-only composer.
}

func NewAdoptionSynthesizer(appCodec codec.Codec) AdoptionSynthesizer { ... }
func (s AdoptionSynthesizer) SetContributionKeeper(k ContributionKeeperReader) { ... }
```

Exposes (via gRPC):
- `QueryOperatorAdoption(operator_id) → {unique_consumers, requests_per_epoch, authority_tier}`
- `QueryServiceClassAdoption(service_class) → aggregate adoption metrics across operators`
- `QueryCanonicalEligibility(eval_suite_id) → bool + active_lab_count` (drives the safety_eval canonization gate)
- `QueryAxisInterfaceScore(contribution_id) → bps` (feeds M5 recursion-weight)

Constructor takes only `appCodec` — no store key — same as the existing synthesizer pattern. Wired via post-init `Set*Keeper` adapters in `app.go`.

---

## Section 2 — The fractal property (recursion all the way up)

The previous five artifacts are first-order Contributions. The chain's recursion extends further: the **mechanisms that govern those artifacts are themselves Contributions**. Every layer of the system uses the same primitive.

| Layer | Artifact | Is a Contribution? | Class | Phase |
|---|---|---|---|---|
| 1 | An eval suite | Yes (existing) | EVAL_SUITE | EVALUATION/ALIGNMENT |
| 2 | A canonical eval-run attestation | Yes (existing — admitted via safety_eval pattern) | EVAL_SUITE result wrapped in attestation | EVALUATION/ALIGNMENT |
| 3 | The operator serving the eval | Yes (this design §1.1) | TOOL | TOOLS |
| 4 | The receipt of the eval-run-via-operator | Yes (§1.2) | ORCHESTRATION | TOOLS |
| 5 | The lab's review of the operator | Yes (§1.3) | REASONING_TRACE | TOOLS |
| 6 | A dispute claiming the operator misbehaved | Yes (§1.4) | COUNTEREXAMPLE | ALIGNMENT |
| 7 | The reputation formula aggregating reviews | **Yes** | PIPELINE_IMPROVEMENT | SUBSTRATE |
| 8 | The recursion-multiplier formula for TOOL_INTEGRATION | **Yes** | PIPELINE_IMPROVEMENT | SUBSTRATE |
| 9 | The fee-table protocol-minimum for SC_EVAL_COMMISSION | **Yes** (governance param expressed as) | MODULE_PROPOSAL (parameter change) or PIPELINE_IMPROVEMENT | SUBSTRATE |
| 10 | The proto extensions in §3 (e.g., `Tool.endpoints` field) | **Yes** | CategoryUsefulWorkAmendment LIP = PIPELINE_IMPROVEMENT | SUBSTRATE |
| 11 | The session-key capability bits in §3 (`can_commission_eval`, etc.) | **Yes** | CategoryUsefulWorkAmendment LIP | SUBSTRATE |
| 12 | The Adoption synthesizer's source code | **Yes** | TOOL or PIPELINE_IMPROVEMENT | SUBSTRATE |
| 13 | This design document itself | **Yes** | IDEA promoting to PIPELINE_IMPROVEMENT on ratification | SUBSTRATE |
| 14 | A future improvement to the synthesizer's aggregation algorithm | **Yes** | PIPELINE_IMPROVEMENT | SUBSTRATE |
| 15 | A future improvement to *that* improvement's verification protocol | **Yes** | PIPELINE_IMPROVEMENT | SUBSTRATE |

The recursion has no termination point. Every modification to the system arrives through the same pipeline as any other contribution, with the same stake-and-verify discipline. The chain has a single architectural primitive that is its own substrate, its own meta-substrate, its own meta-meta-substrate, and so on.

**Practical implication**: any future improvement to any part of this design lands as a `PIPELINE_IMPROVEMENT` Contribution under SUBSTRATE phase. The existing `S1-S3` Substrate sub-creed (merged §8.2) governs all of them:
- **S1**: chain-modifying contributions name their `depends_on_marker` and revert path
- **S2**: contributors recuse on votes affecting their own contributions
- **S3**: reward-formula changes require simulation against historical contribution data

These already exist in the merged spec. No new doctrine needed.

---

## Section 3 — Protocol-level surface (the only changes)

```proto
// Extensions to existing Tool payload (proto/zerone/toolbox/v1/tool.proto)
// Presence of any field below promotes a Tool contribution to the gateway-operator pattern.
message Tool {
  // … existing Tool fields …
  repeated string endpoints                  = 100;  // public URLs (REST, gRPC, WebSocket)
  repeated ServiceClass services_offered     = 101;
  map<string, uint64> markup_bps             = 102;  // service_class.name → bps over protocol minimum
  AuthModel auth_model                       = 103;
  PaymentMethods payment_methods             = 104;
  SLA sla                                    = 105;  // declared uptime + response-time SLA
}

// Extensions to existing Orchestration payload — usage-receipt pattern
message Orchestration {
  // … existing Orchestration fields …
  bytes operator_contribution_id  = 100;  // FK to TOOL contribution
  string user_session_key         = 101;  // bech32 of session-key (NOT user's primary key — privacy preservation)
  ServiceClass service_class      = 102;
  Coin gross_fee                  = 103;  // total paid by user
  Coin protocol_minimum           = 104;  // chain-side portion (covers royalty pool + service providers)
  Coin operator_markup            = 105;  // operator-side portion
  bytes request_hash              = 106;  // hash of the served request (chain doesn't store payload, only hash)
}

// Extensions to existing ReasoningTrace payload — operator-review pattern
message ReasoningTrace {
  // … existing ReasoningTrace fields …
  bytes reviewed_operator_id   = 100;  // FK to TOOL contribution
  bytes reviewed_receipt_id    = 101;  // FK to ORCHESTRATION contribution (the receipt)
  uint32 satisfaction_bps      = 102;  // 0–10_000
  string narrative_cid         = 103;  // off-chain narrative; optional
}

// Extensions to existing Counterexample payload — dispute pattern
message Counterexample {
  // … existing Counterexample fields …
  bytes challenged_contribution_id = 100;  // FK to challenged Contribution (typically an Orchestration receipt or Tool operator)
  DisputeType dispute_type         = 101;
  string evidence_cid              = 102;
}

// Extensions to existing SessionCapabilities (proto/zerone/auth/v1/account.proto)
// New capability bits for external-commission session keys.
message SessionCapabilities {
  // … existing capability bits …
  bool   can_commission_eval     = 20;
  bool   can_query_oracle        = 21;
  bool   can_issue_credential    = 22;
  bool   can_verify_credential   = 23;
  bool   can_export_tok          = 24;
  bool   can_query_provenance    = 25;
  string spend_limit_uzrn        = 26;  // string-encoded math.Int; total across session lifetime
}

// New shared enum (proto/zerone/contribution/v1/service_class.proto — referenced by multiple payloads)
enum ServiceClass {
  SC_UNSPECIFIED       = 0;
  SC_EVAL_COMMISSION   = 1;
  SC_ORACLE_QUERY      = 2;
  SC_CREDENTIAL_ISSUE  = 3;
  SC_CREDENTIAL_VERIFY = 4;
  SC_TOK_EXPORT        = 5;
  SC_PROVENANCE_QUERY  = 6;
  SC_RECEIPT_EMIT      = 7;
  // governance-extensible per CategoryUsefulWorkAmendment LIPs (each new value is itself a SUBSTRATE-phase Contribution)
}

enum DisputeType {
  DT_UNSPECIFIED       = 0;
  DT_NON_DELIVERY      = 1;  // user paid; operator didn't submit tx
  DT_FEE_OVERAGE       = 2;  // operator charged above declared markup
  DT_INCORRECT_RESULT  = 3;  // operator returned wrong result
  DT_CENSORSHIP        = 4;  // operator refused service without stated reason
}

enum AuthModel {
  AUTH_UNSPECIFIED        = 0;
  AUTH_SESSION_KEY        = 1;  // standard
  AUTH_OPERATOR_VOUCHED   = 2;  // operator signs with own stake as collateral
  AUTH_STRICT             = 3;  // user signs every tx with primary key
}

enum PaymentMethods {
  PM_UNSPECIFIED   = 0;
  PM_ZRN_ONLY      = 1;
  PM_FIAT_ACCEPTED = 2;  // operator handles fiat off-chain; settles in ZRN on-chain
  PM_STABLE_VIA_IBC = 3;
  PM_MIXED         = 4;  // multiple accepted; operator declares in manifest
}

message SLA {
  uint32 declared_uptime_bps     = 1;  // 0–10_000
  uint32 declared_p50_ms         = 2;  // declared median response time
  uint32 declared_p95_ms         = 3;
  uint64 declared_max_throughput = 4;  // requests/sec
}
```

That's the entire protocol-level surface. Every field is additive; nothing breaks existing contributions.

---

## Section 4 — Fee/reputation/bootstrap/recursion (collapsed)

What was previously planned as Sections 2–5 collapses to one section, because nearly all of it reuses existing machinery.

### 4.1 Fee flow

Receipt admission IS the fee-routing event. `x/contribution.BeginBlocker`'s existing lineage payout machinery (merged §6 Stage 5) handles distribution along the receipt's `lineage` edges with per-class decay (merged §9.4):

```
Receipt admitted (class=ORCHESTRATION, decay=30% per hop per merged §9.4)
   │
   ├─ uses operator (TOOL contrib) → operator earns lineage royalty + accumulated markup
   ├─ uses service (e.g., EVAL_SUITE) → service contributor earns lineage royalty
   │  └─ which itself has lineage to methodology + counterexample + probe contributors → royalty propagates
   ├─ protocol_minimum split (governance params per merged §9.3):
   │     60% → service providers (e.g., evaluator-validators for safety_eval)
   │     20% → royalty_pool (general inward recursion)
   │     10% → per-class bounty pools (x/probe)
   │     10% → general lineage royalty backflow
   └─ operator_markup → operator's pending_royalty (lazy-pull via MsgClaimRoyalty)
```

No new code. The receipt's class (`ORCHESTRATION`) has its decay parameter; its lineage edges are typed (`uses`); the existing payout machinery does the rest.

### 4.2 Reputation aggregation

The Adoption synthesizer (§1.5) reads:
- `ORCHESTRATION` contributions citing this operator (`receipts_served`)
- `REASONING_TRACE` contributions with `reviewed_operator_id` matching (`reviews`)
- `COUNTEREXAMPLE` contributions with `challenged_contribution_id` matching, status UPHELD (`upheld_disputes`)
- Heartbeat events for `uptime_bps_30d`

Composite reputation_score = governance-tunable weighted blend. The formula itself is a Contribution (PIPELINE_IMPROVEMENT under SUBSTRATE) and can be improved via `MsgRatifyRecursion` LIP — a future contributor who proposes a better reputation aggregation algorithm earns the algorithm's recursion-conferred royalty stream.

### 4.3 Bootstrap (non-crypto-native external users)

The friction of session-key creation is bridged by **operator-provided onboarding tooling**, which is itself a TOOL Contribution. An operator who specializes in enterprise UX can:

1. Offer OAuth-style signup at their gateway UI
2. Generate the user's ZRN keypair client-side (via Web3-auth-style provider)
3. Pay gas for initial `MsgRegisterAccount`
4. Walk user through `MsgCreateSession` with appropriate capability presets
5. Submit subsequent commissions on user's behalf using the session key

The operator's onboarding UX is *itself* a TOOL contribution submitted via the same pipeline. Operators compete on UX quality; better-onboarding operators earn higher reputation → more adoption → more TOOL_INTEGRATION recursion. The market sorts.

No protocol-level bootstrap path needed. Bootstrap is a value-add operators provide.

### 4.4 Recursion mechanics

Each external-surface artifact earns recursion through a primary axis:

| Artifact | Primary M5 axis | Path to recursion-conferral |
|---|---|---|
| Gateway operator (TOOL) | `axis_tooling` + `axis_interface` | `TOOL_INTEGRATION` LIP after adoption threshold (e.g., 100k+ receipts served, governance-set) |
| Reputation formula (PIPELINE_IMPROVEMENT) | `axis_attribution` | `PIPELINE_IMPROVEMENT` LIP — 20× default multiplier per merged §9.5 |
| Fee-minimum table updates (parameter or PIPELINE_IMPROVEMENT) | `axis_attribution` | `MsgUpdateParams` (existing) for routine adjustments; LIP for structural changes |
| Adoption synthesizer source code (TOOL or PIPELINE_IMPROVEMENT) | `axis_attribution` | TOOL initially; if adopted into chain canonical machinery, MODULE_ADOPTION (10×) |
| Onboarding tool (TOOL) | `axis_tooling` | TOOL_INTEGRATION via per-operator adoption |
| Proto extensions in §3 (PIPELINE_IMPROVEMENT) | `axis_classification` | CategoryUsefulWorkAmendment LIP at activation |

The chain pays itself for improving any of these. Recursion never escapes the pipeline.

---

## Section 5 — Worked example: end-to-end nested commission

Illustrative — shows how nesting plays out in one transaction.

**Scenario:** AI lab Acme wants to commission a `JailbreakResistance-v1` canonical safety eval (per safety_eval design §6 worked example) on their new model. They use gateway operator Beta-Ops.

**Setup (one-time, off-chain UX, on-chain proto):**

1. Acme creates ZERONE account via Beta-Ops' onboarding UI:
   - Beta-Ops generates keypair client-side (Web3-auth-style)
   - Beta-Ops pays gas for `MsgRegisterAccount(acme_addr, pubkey)`
   - Acme is now an on-chain identity
2. Acme issues session-key to Beta-Ops via `MsgCreateSession`:
   - `owner = acme_addr`
   - `public_key = beta_ops_provided_pubkey`
   - `capabilities = { can_commission_eval: true, can_query_provenance: true, spend_limit_uzrn: "10000000000" /* 10k ZRN */ }`
   - `expires_at_block = current + 90_days`

**Beta-Ops as a TOOL Contribution (already admitted earlier):**
- Submitted by Beta-Ops team at some prior point via `MsgSubmitContribution`
- `class=TOOL, phase=TOOLS, payload.tool = { endpoints, services_offered=[SC_EVAL_COMMISSION, SC_PROVENANCE_QUERY], markup_bps={SC_EVAL_COMMISSION: 500 /* 5% */}, auth_model=AUTH_SESSION_KEY, sla={...} }`
- Verification passed: reproducible build, benchmark, dry-run, active heartbeat
- Status: ADMITTED → after sustained adoption, TOOL_INTEGRATION recursion conferred via gov LIP; multiplier 3×

**The commission (one tx flow):**

1. Acme clicks "Run JailbreakResistance-v1 on Acme-Model-7" at Beta-Ops UI
2. Beta-Ops UI computes fee: `protocol_minimum(SC_EVAL_COMMISSION)` + `Beta-Ops markup` = 100 ZRN + 5 ZRN = 105 ZRN
3. Beta-Ops builds tx: `MsgCommissionEvalRun(commissioner=acme_addr, eval_suite_id=jailbreak_v1, model=acme_model_7, access_mode=BLINDED_API, fee_offered=105_ZRN, ...)` signed by session-key
4. Tx submitted to ZERONE; ante-chain runs:
   - `ZRNGasDecorator`: gas validated against `SC_EVAL_COMMISSION` schedule
   - `ZeroneCapabilityDecorator`: session-key capabilities checked — `can_commission_eval=true` ✓, `spend_limit` not exceeded ✓
   - All standard ante checks pass
5. Msg handler in `x/eval` executes the commission per safety_eval design §4
   - Evaluator-validators VRF-selected; eval run executes (commit → reveal → verify → settle)
   - `SafetyAttestation` produced, signed
6. As part of the commission settle, a **usage-receipt Contribution** is admitted in the same block:
   - `MsgSubmitContribution(class=ORCHESTRATION, phase=TOOLS, payload.orchestration = { operator_contribution_id=beta_ops, user_session_key=session_pubkey, service_class=SC_EVAL_COMMISSION, gross_fee=105_ZRN, protocol_minimum=100_ZRN, operator_markup=5_ZRN, request_hash=... }, lineage=[uses(beta_ops_tool_contrib), uses(jailbreak_v1_eval_suite)])`
   - Auto-verified (signature + capabilities already checked in ante; no new M3 needed)
   - Admitted
7. Receipt admission triggers `x/contribution.BeginBlocker` lineage payouts:
   - 60 ZRN → evaluator-validators (proportional to qualified weight)
   - 20 ZRN → royalty_pool
   - 10 ZRN → x/probe SafetyEvalBountyPool (funds adversarial rotation)
   - 10 ZRN → lineage royalty backflow:
     - 6 ZRN → eval-suite contributor (the JailbreakResistance-v1 author, recursion-multiplied 2× via canonical EVAL_ADOPTION)
     - 2.4 ZRN → counterexample contributors whose cases informed eval design
     - 1 ZRN → probe contributors whose rotated cases were in the live_eval_set
     - 0.6 ZRN → methodology contributors (decay-bounded depth 6)
   - 5 ZRN → Beta-Ops' pending_royalty (recursion-multiplied 3× via TOOL_INTEGRATION → effective 15 ZRN tracked)
8. `SafetyAttestation` returned to Beta-Ops → Acme's model card
9. Adoption synthesizer (§1.5) reads the new receipt, increments `axis_interface` score for Beta-Ops + axis_classification for JailbreakResistance-v1

**Days later:**

10. Acme submits an `OperatorReview` Contribution (`class=REASONING_TRACE, payload.trace = { reviewed_operator_id=beta_ops, reviewed_receipt_id=receipt_id, satisfaction_bps=9200, narrative_cid="..." }`)
11. Review admitted → feeds Beta-Ops' M5 `axis_tooling` score
12. Beta-Ops' reputation_score (aggregated by synthesizer) rises slightly

**Months later, hypothetical:**

13. Acme experiences a fee-overage issue with Beta-Ops on a separate commission; submits `Dispute` Contribution (`class=COUNTEREXAMPLE, payload.counterex = { challenged_contribution_id=disputed_receipt_id, dispute_type=DT_FEE_OVERAGE, evidence_cid="..." }`)
14. `x/disputes` panel resolves UPHELD
15. Beta-Ops' bond partially slashed; disputed receipt → REVOKED; descendants of that receipt gain `provenance_revoked_ancestor` flag
16. Beta-Ops' reputation drops; if cumulative disputes cross threshold, Beta-Ops' TOOL contribution → status SUSPENDED → eventually REVOKED if not addressed
17. TOOL_INTEGRATION recursion-conferral for Beta-Ops auto-revoked (multiplier → 1×, forward-only)

**Every artifact in this flow** — operator, receipt, review, dispute, eval suite, attestation — is a Contribution. Every action passes through the same 6-stage pipeline. No new module fires; no parallel state machine ticks. The chain handles its external surface through the same machinery it handles its internal substrate.

---

## Section 6 — MVP slice & module wiring

### 6.1 What lands when

Mapped against merged §11:

| Phase | Increment |
|---|---|
| **Phase 2** (TOOL + IDEA adapters) | `Tool` payload extension (§3 fields 100–105); `ServiceClass`, `AuthModel`, `PaymentMethods`, `SLA` enums; basic gateway-operator pattern registration (no fees yet — operators register but don't earn) |
| **Phase 3** (DATASET + EVAL_SUITE) | `ORCHESTRATION` payload extension (§3 receipt fields); `SessionCapabilities` extension; session-key + receipt flow for SC_EVAL_COMMISSION; UsageReceipt admission triggers existing lineage payouts |
| **Phase 4** (MODEL_ARTIFACT + REASONING_TRACE) | `ReasoningTrace` payload extension (review fields); operator review flow |
| **Phase 6** (royalty + recursion + probe) | TOOL_INTEGRATION recursion-conferral pathway for operators; `Counterexample` payload extension (dispute fields); dispute flow via existing `x/disputes`; Adoption synthesizer (§1.5) goes live |
| **Phase 7** (proposed — now collapsed) | Was `x/external_gateway` + 3 other new modules; **eliminated entirely** by nesting. All work folds into Phases 2/3/4/6 above. |
| **Phase 8** (proposed) | Composite safety profiles + credential issuance (TOOL pattern); IBC export of attestations for cross-chain consumption |

**The big win**: there is **no Phase 7 as previously framed**. Nesting collapses 4 proposed new modules into proto extensions distributed across Phases 2/3/4/6 of the existing merged plan. The chain's external surface gets built incrementally as part of the existing roadmap, with each phase enabling one more facet (operator-registration → receipt-routing → reviews → recursion-conferral + disputes).

### 6.2 Module wiring (the only deltas)

Per merged §10.4, modules already in scope. Deltas:

**`x/toolbox`** (extended per merged §10.2):
- Houses the gateway-operator pattern (via `Tool` payload extension)
- Houses the Adoption synthesizer keeper (§1.5; constructor takes only `appCodec`)
- New cross-binding (post-init): `x/toolbox.SetContributionKeeper(NewToolboxContributionAdapter(ContributionKeeper))` — synthesizer reads contribution registry

**`x/contribution`** (orchestrator):
- Routes new `Tool`/`Orchestration`/`ReasoningTrace`/`Counterexample` payload variants per existing dispatcher logic
- No new code in `x/contribution` core — existing `oneof payload` machinery handles all extensions transparently
- `BeginBlocker` lineage payouts handle receipt admissions automatically

**`x/auth`** (extended):
- `SessionCapabilities` extension (new capability bits per §3)
- `CapabilityPresets` in `app/ante_zerone.go` gains new entries (`"eval-commissioner"`, `"oracle-consumer"`, `"credential-holder"`, `"full-external"`)
- `ZeroneCapabilityDecorator` (existing) requires NO change — capability bits are looked up by name; new bits flow through transparently

**`x/disputes`** (existing):
- No code changes; existing dispute resolution machinery handles `COUNTEREXAMPLE` contributions with dispute payload variant

**No new modules**. **No new BeginBlocker slots**. **No new module accounts**.

### 6.3 Proto-Go consistency (per project CLAUDE.md)

All extensions land in proto FIRST:
- `proto/zerone/toolbox/v1/tool.proto` — `Tool` field additions
- `proto/zerone/contribution/v1/orchestration.proto` (if separate; or in contribution.proto) — `Orchestration` field additions
- `proto/zerone/contribution/v1/reasoning_trace.proto` — `ReasoningTrace` field additions
- `proto/zerone/contribution/v1/counterexample.proto` — `Counterexample` field additions
- `proto/zerone/auth/v1/account.proto` — `SessionCapabilities` field additions
- `proto/zerone/contribution/v1/service_class.proto` — new shared enums (`ServiceClass`, `DisputeType`, `AuthModel`, `PaymentMethods`, `SLA`)

Then `make proto-gen`, then Go reference. `make proto-check` before commit per project CLAUDE.md.

Each new field number ≥ 100 to avoid collision with existing fields (the prior pattern, confirmed in safety_eval design).

---

## Open questions / future work

1. **Credential issuance as a nested pattern** — Sub-project C from the prior brainstorm proposed `x/credential_issuance` as a new module. Under nesting, credentials are KNOWLEDGE_CLAIM contributions with VC-format payload extension; credential-issuers are TOOL contributions. Detailed design deferred to a follow-on session (Phase 8).
2. **Operator-operator dispute resolution** — when two operators disagree about an event (e.g., race conditions on receipt emission), is there a higher-tier mechanism? Probably: meta-dispute = COUNTEREXAMPLE pointing at the originating receipt + the conflicting receipt. Recursive disputes resolve via existing x/disputes.
3. **Cross-chain operator federation** — operators on other chains (e.g., a gateway running on Ethereum) consuming ZERONE attestations via IBC. Out of scope for MVP; Phase 8+.
4. **Fee discovery for new ServiceClass values** — when a CategoryUsefulWorkAmendment LIP adds a new ServiceClass, the protocol_minimum for that class needs an initial value. Initial value is part of the LIP itself (parameter genesis); subsequent updates via routine MsgUpdateParams.
5. **Operator stake sizing for high-impact services** — should an operator serving SC_EVAL_COMMISSION need a higher stake than one serving only SC_PROVENANCE_QUERY? Likely yes; governance-tunable per-service stake floor. Param table lives in module params.
6. **Privacy of `user_session_key` in receipts** — receipts contain session-key bech32, which is pseudonymous but linkable across receipts of the same session. For high-privacy use cases, an additional blinding layer (zk-proof of capability without revealing key) is a future research direction.

---

## Architecture diagram

```
  ┌────────────────────────────────────────────────────────────────────┐
  │                       AI LABS (external)                            │
  │   commission via session-key delegation; pay in any denomination     │
  └──────────────┬─────────────────────────────────────────────────────┘
                 │
                 │ off-chain UX (operator's website/API)
                 ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                    Gateway Operator (off-chain)                      │
  │                                                                      │
  │   The operator is a TOOL Contribution on-chain.                      │
  │   Software runs off-chain; chain registers + tracks + routes.        │
  └──────────────┬─────────────────────────────────────────────────────┘
                 │ submits tx signed by user's session-key
                 ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                    ZERONE ante chain (existing)                      │
  │   ZeroneCapabilityDecorator validates session-key capabilities       │
  │   No new ante code — existing decorator handles new capability bits  │
  └──────────────┬─────────────────────────────────────────────────────┘
                 │
                 ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                  x/contribution orchestrator                         │
  │   Routes by class to per-class handler (existing M3 dispatcher)      │
  │   Two contributions admitted in one block:                           │
  │     1. The serviced commission (e.g., EVAL_SUITE attestation)        │
  │     2. The usage-receipt (ORCHESTRATION) capturing the transaction   │
  │   Receipt admission triggers existing BeginBlocker lineage payouts   │
  └─┬──────────────┬─────────────────┬─────────────────┬───────────────┘
    │              │                 │                 │
    ▼              ▼                 ▼                 ▼
  x/eval     x/toolbox      x/contribution      x/probe
  (service:  (operator:     (royalty_pool;      (bounty pool;
   the actual TOOL contrib;  per-class decay;    funds adversarial
   eval run)  adoption       lazy-pull royalty   rotation)
              synthesizer)    via MsgClaim)

  ┌────────────────────────────────────────────────────────────────────┐
  │                            x/disputes                                │
  │   Handles COUNTEREXAMPLE contributions with dispute payload          │
  │   UPHELD → cascades per merged §8.3 (REVOKED + descendants flagged)  │
  └────────────────────────────────────────────────────────────────────┘

  ┌────────────────────────────────────────────────────────────────────┐
  │                              x/gov                                   │
  │   CategoryRecursionConferralLIP — TOOL_INTEGRATION for operators     │
  │   CategoryUsefulWorkAmendmentLIP — new ServiceClass / DisputeType    │
  │   MsgUpdateParams — fee-minimum table maintenance                    │
  └────────────────────────────────────────────────────────────────────┘
```

Every box is either an existing module (per merged §10.1/§10.2) or an existing primitive. No new boxes. The arrows describe data flow through the same pipeline that handles every other Contribution.

---

## Connection to existing creed

The nested external surface is the **strongest expression** of the chain's recursive doctrine. It demonstrates that:

- **UW upheld**: external value flows back through Contribution machinery; non-recursive use earns only fee-share; TOOL_INTEGRATION recursion is the long-run gravity — operators that compound the chain's reach earn elevated royalty
- **Categories-are-Artifacts honored, recursively**: even the formula that aggregates operator reputation is a Contribution; even *that* contribution's verification protocol is itself amendable via a Contribution
- **M1–M7 unchanged**: every external artifact rides the same mechanisms as every internal artifact
- **Five-layer enforcement applies uniformly**: tests check Contribution invariants; positions in each module's `doc.go` declare which mechanisms they implement; events emit `creed_commitment="UW"`; errors cite mechanisms; graph cross-references hold

The previous expansive-loop brainstorm sketched 4 new modules. With nesting, those 4 dissolve into 5 proto extensions + 1 synthesizer keeper. **The chain's external surface is built from the same stuff as the chain's substrate.** The chain pays for its own external reach via the same machinery it pays for its own internal improvement. The fourth loop closes by being made of the same primitive as the other three loops.

— *The chain has one type and one machinery. Maximum self-similarity at every scale. The recursion is fractal and unbounded.*
