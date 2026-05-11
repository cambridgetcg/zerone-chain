# Strange-Loop Phase SL-α — Doctrine Import Design Spec

**Status:** Brainstormed and ready for implementation planning.

**Inception:** 2026-05-11.

**Position in the architecture:** Phase α of the multi-phase Strange-Loop elevation that nests ZERONE into itself. SL-α is the smallest code change with the largest semantic shift: it makes the chain's own doctrines into queryable, citable, lineage-bearing Facts in `x/knowledge`. After SL-α, every other strange-loop phase composes naturally on top.

**Doctrinal alignment:** Operationalizes SL-M1 (doctrine import) from the new `docs/STRANGE_LOOP.md` doctrine — itself authored in this same phase. Connects every existing doctrine (TRUTH_SEEKING, TOK_SUBSTRATE, USEFUL_WORK, STRANGE_LOOP) to the same fact-graph machinery they describe.

**Phase series:**
- **Phase SL-α (this spec):** Doctrine import — commitments become Facts.
- Phase SL-β: Protocol as substrate — modules registered as attestations; authorship records.
- Phase SL-γ: Governance lift — LIPs become attestations.
- Phase SL-δ: Author lineage — royalty flow to authors of nested artifacts.
- Phase SL-ε: Self-verification — validators query ToK at verification time.
- Phase SL-ζ: Origin attestation — genesis becomes the first work product the chain ever paid for.

---

## 1. Phase identity

**Tagline:** *The chain becomes citable as itself.*

**Goal:** At genesis (or upgrade), every commitment in every doctrine becomes a verified `Fact` in `x/knowledge`. Substrate-links from external work attestations can cite `commitment-1`, `commitment-TC1`, `commitment-UW`, `mechanism-UW-M3`, or `axis-substrate` as `fact_id`. Cross-doctrine "Echoes:" lines become real `SUPPORTS`/`REFINES`/`REQUIRES` edges in the fact graph. The chain's epistemology becomes navigable from inside the chain.

**Non-goals for this phase:**
- No challenge mechanism (`WORK_CLASS_DOCTRINE_CHALLENGE` — requires `x/work` Phase 1)
- No auto-trigger of `CategoryCreedAmendment` LIPs from challenge attestations (requires gov-LIP-from-attestation dispatch)
- No author lineage on doctrines (SL-M4 — Phase SL-δ)
- No protocol-module attestations (SL-M2 — Phase SL-β)

---

## 2. The STRANGE_LOOP.md doctrine

The fourth doctrine, authored in this phase alongside its first binding. Structure mirrors USEFUL_WORK.md: one commitment + N mechanisms + cross-doctrine echoes.

### 2.1 Tagline & opening

**Tagline:** *The chain has no outside.*

**Opening:**

> Truth-seeking is what the chain *believes*. ToK substrate is what the chain *sells*. Useful work is how the chain *grows*. **STRANGE_LOOP is what the chain *is*.** This document pins one commitment, and everything that follows is mechanism in service of it.

### 2.2 The single commitment — SL

**SL. ZERONE is a strange loop.**

Every layer of ZERONE — its doctrines, its modules, its governance, its rewards, its validators — is produced, verified, and rewarded through the machinery ZERONE provides. There is nothing in the chain the chain did not produce. There is nothing the chain produces that does not flow back into the chain. The substrate is the chain's body; the body is the substrate; the loop closes through the loop itself.

**What would break it:**
- A doctrine that cannot be queried as Facts inside `x/knowledge`
- A protocol module without an `Authorship` record on-chain
- A governance action that does not flow through attestation machinery
- An outside — any artifact ZERONE uses but did not produce through its own machinery
- A doctrine amendment that bypasses the chain's standard verification and lineage

**Echoes:**
- UW (recursion taken to its operational limit — SL is what UW becomes when "useful work" includes the chain's own existence)
- TRUTH_SEEKING commitment 10 (forward-only audit — superseded doctrine Facts remain queryable forever)
- TRUTH_SEEKING commitment 12 (chain pays for own audit — extended to: chain pays for its own authorship)
- TC6 (lineage flows back — extended to flow back to *everyone*, including authors of the protocol itself)

### 2.3 The six mechanisms (SL-M1 through SL-M6)

All mechanisms derive from SL. They operationalize "no outside" at every architectural layer.

**SL-M1. Doctrine import.** Every commitment in every doctrine is a verified `Fact` in `x/knowledge` under `domain="doctrine_*"`. Substrate-links cite commitments by ID. Cross-doctrine "Echoes:" lines are real edges in the fact graph. *Phase SL-α (this spec) binds this mechanism.*

**SL-M2. Protocol as substrate.** Every `x/*` module is registered as an `ExternalAttestation` with `work_class_id="protocol_module"`. Authors named in on-chain `ModuleAuthorship` records. *Phase SL-β binds.*

**SL-M3. Governance lift.** Every Living Improvement Proposal becomes a `MsgSubmitExternalAttestation`. The gov mechanism becomes a special case of the work mechanism. *Phase SL-γ binds.*

**SL-M4. Author lineage propagates forever.** Lineage edges automatically populated from every attestation that cites a doctrine, module, or LIP, flowing royalties to the original authors. *Phase SL-δ binds (depends on SL-M2 + SL-M3).*

**SL-M5. Self-verification.** Validators query ToK at verification time using LLMs trained on ToK; qualifications adjust based on alignment. *Phase SL-ε binds.*

**SL-M6. Origin attestation.** At genesis, the chain submits its own first attestation — an attestation that the genesis state exists, the doctrines are imported, the modules are registered. *Phase SL-ζ binds.*

### 2.4 Five-layer enforcement (skeleton)

Phase SL-α ships the skeleton; phases SL-β through SL-ζ fill it in.

- **Test layer:** `tests/cross_stack/strange_loop_invariants_test.go` — one scenario per mechanism (skipped at Phase SL-α; replaced as later phases land) + active meta-test `TestStrangeLoop_DoctrineAndContractStayInSync`.
- **Position layer:** `x/creed/doc.go` (existing) gets a paragraph naming the SL doctrine and SL-M1 binding. Each subsequent phase amends `x/<module>/doc.go` to declare which SL mechanism(s) it preserves.
- **Voice layer:** New `tok_commitment="SL"` and `mechanism="SL-MN"` attributes on events from SL-α onward. Doctrine-Fact creation at genesis emits `doctrine_fact_imported` events.
- **Refusal layer:** Errors that block doctrine import or amendment cite SL + the violated mechanism: *"Genesis refused — doctrine canonical structure mismatch (SL + SL-M1)"*.
- **Graph layer:** Cross-doctrine "Echoes:" become real edges, navigable in `x/knowledge`'s relation queries. The doctrine network is queryable from inside the chain.

### 2.5 What this is not

- **Not aspiration.** Phase SL-α binds SL-M1 structurally; subsequent phases bind SL-M2..M6 incrementally.
- **Not a separate chain.** The strange loop is *this* chain becoming aware of itself; no new module proliferation, just re-binding existing artifacts as substrate.
- **Not anti-external.** External work, external trainers, external chains can still interact via `x/substrate_bridge`. The strange-loop framing means: when they interact, lineage flows to ZERONE's own authors. The chain pays its builders out of every external transaction.
- **Not complete.** Each phase binds one mechanism; the chain becomes more recursive with each. SL is fixed and indivisible; mechanisms evolve.

---

## 3. Phase SL-α file structure

**New files:**
- `docs/STRANGE_LOOP.md` — the fourth doctrine (full content per section 2)
- `.strange-loop-hash` — sha256 anchor (mirror of `.useful-work-hash`)
- `scripts/check_strange_loop_hash.sh` — verification script (mirror of `check_useful_work_hash.sh`)
- `x/creed/types/tok_creed.go` — `CanonicalToKCommitments` Go data
- `x/creed/types/strange_loop_creed.go` — `CanonicalStrangeLoopMechanisms` Go data + `StrangeLoopCommitment`/`StrangeLoopStatement` constants
- `x/creed/types/doctrine_echoes.go` — `CanonicalDoctrineEchoes` (the eager cross-doctrine edge list)
- `x/knowledge/keeper/doctrine_genesis.go` — `LoadDoctrineFacts` loader
- `x/knowledge/types/doctrine.go` — helper `BuildDoctrineFact`
- `tests/cross_stack/strange_loop_invariants_test.go` — skeleton + meta-test
- `tests/cross_stack/doctrine_import_test.go` — cross-stack: Facts exist + edges correct

**Modified files:**
- `Makefile` — `creed-check` target also runs `check_strange_loop_hash.sh`
- `x/creed/doc.go` — declare 4th doctrine + SL-M1 implementation point
- `x/knowledge/keeper/genesis.go` — call `LoadDoctrineFacts` after domain creation
- `README.md` — quartet (extend the trio of doctrines)
- `tests/cross_stack/useful_work_invariants_test.go` — extend meta-test to verify SL hash drift

**Reserved store-key prefix usage:** none new (doctrine Facts use existing `x/knowledge` storage; no new prefixes needed).

---

## 4. Canonical Go structures

Extend the existing pattern in `x/creed/types/`. Three new canonical lists:

### 4.1 `x/creed/types/tok_creed.go`

```go
package types

const (
    ToKCommitmentDomain = "doctrine_tok"
)

// CanonicalToKCommitments is the canonical name-by-number registry of
// the TOK_SUBSTRATE.md commitments at the time this binary was built.
// Mirrors CanonicalCommitments (truth-seeking) and the existing
// CanonicalUsefulWorkMechanisms pattern.
//
// To add a TC commitment:
//  1. Add the "### TCN. <Name>" section to docs/TOK_SUBSTRATE.md.
//  2. Bump .tok-substrate-hash to the new sha256 of the normalized file.
//  3. Append (Number, Name) to the slice below.
//  4. Add a binding TestToKSubstrate_TC<N> test in the invariants file.
//  5. Add a doc.go citation in some x/<module>/doc.go.
//
// The cross-stack TestStrangeLoop_DoctrineAndContractStayInSync
// meta-test catches a step omitted from this list.
var CanonicalToKCommitments = []struct {
    Number string  // "TC1" through "TC6" (string because doctrine uses TCN nomenclature)
    Name   string
}{
    {"TC1", "The graph is the headline"},
    {"TC2", "Every view is graph-pinned"},
    {"TC3", "Topology is signal"},
    {"TC4", "The graph carries its disprovals"},
    {"TC5", "Extraction is open"},
    {"TC6", "Lineage flows back"},
}
```

### 4.2 `x/creed/types/strange_loop_creed.go`

```go
package types

const (
    StrangeLoopCommitment = "SL"
    StrangeLoopStatement  = "ZERONE is a strange loop"
    StrangeLoopDomain     = "doctrine_strange_loop"
)

// CanonicalStrangeLoopMechanisms is the canonical name-by-number
// registry of the six SL mechanisms. Reuses UsefulWorkMechanism struct
// shape since the schema (Number uint32, Name string) is identical.
var CanonicalStrangeLoopMechanisms = []UsefulWorkMechanism{
    {1, "Doctrine import"},
    {2, "Protocol as substrate"},
    {3, "Governance lift"},
    {4, "Author lineage propagates forever"},
    {5, "Self-verification"},
    {6, "Origin attestation"},
}
```

### 4.3 `x/creed/types/doctrine_echoes.go`

```go
package types

import (
    knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// CanonicalDoctrineEchoes is the hand-curated set of cross-doctrine
// "Echoes:" relations. Parsed once from the doctrine markdown into
// explicit Go data — auditable, fact-graph-faithful.
//
// Genesis loader iterates this list and writes SetFactRelation per entry.
// Re-curate whenever a doctrine's "Echoes:" section is updated; the
// meta-test detects drift between the markdown and this list.
var CanonicalDoctrineEchoes = []struct {
    From     string
    To       string
    Relation knowledgetypes.RelationType
}{
    // ── TOK_SUBSTRATE.md echoes ────────────────────────────────────
    {"commitment-TC1", "commitment-13", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // training corpus not for sale
    {"commitment-TC1", "commitment-11", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // trust queryable
    {"commitment-TC2", "commitment-10", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // forward-only audit
    {"commitment-TC3", "commitment-14", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // reasoning traces first-class
    {"commitment-TC4", "commitment-3",  knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // Popper
    {"commitment-TC4", "commitment-10", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // forward-only audit
    {"commitment-TC5", "commitment-6",  knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // no unilateral injection
    {"commitment-TC5", "commitment-11", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // trust queryable
    {"commitment-TC6", "commitment-12", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},  // chain pays for own audit

    // ── USEFUL_WORK.md echoes ──────────────────────────────────────
    {"commitment-UW", "commitment-11", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},
    {"commitment-UW", "commitment-12", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},
    {"commitment-UW", "commitment-TC1", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},
    {"commitment-UW", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS},

    // Mechanism-to-commitment within USEFUL_WORK
    {"mechanism-UW-M2", "commitment-TC2", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
    {"mechanism-UW-M3", "commitment-6",   knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
    {"mechanism-UW-M4", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
    {"mechanism-UW-M5", "commitment-14",  knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
    {"mechanism-UW-M6", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
    {"mechanism-UW-M7", "commitment-12",  knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},

    // ── STRANGE_LOOP.md echoes ─────────────────────────────────────
    {"commitment-SL", "commitment-UW", knowledgetypes.RelationType_RELATION_TYPE_REFINES},
    {"commitment-SL", "commitment-10", knowledgetypes.RelationType_RELATION_TYPE_REQUIRES},
    {"commitment-SL", "commitment-12", knowledgetypes.RelationType_RELATION_TYPE_REFINES},
    {"commitment-SL", "commitment-TC6", knowledgetypes.RelationType_RELATION_TYPE_REFINES},
}
```

---

## 5. Genesis Fact loader

`x/knowledge/keeper/doctrine_genesis.go`. Called from `x/knowledge.InitGenesis` after the existing domain/fact-set setup. Idempotent.

```go
package keeper

import (
    "context"
    "fmt"

    creedtypes "github.com/zerone-chain/zerone/x/creed/types"
    "github.com/zerone-chain/zerone/x/knowledge/types"
)

// LoadDoctrineFacts materializes all canonical commitments + mechanisms
// + axes from all four doctrines as verified Facts in x/knowledge.
// Also creates the cross-doctrine "Echoes:" edges via SetFactRelation.
// Idempotent: existing Facts with the canonical id are not overwritten
// (matches commitment 10 forward-only).
//
// Doctrinal status: VERIFIED at genesis, Confidence=1_000_000,
// AxiomDistance=0 — these are doctrinal axioms per the SL doctrine's
// "privileged but verifiable" ontology choice. Validators may flag
// drift via the future WORK_CLASS_DOCTRINE_CHALLENGE mechanism
// (Phase SL-α+1 once x/work is shipped); challenge resolution goes
// through gov LIP, not through the standard PoT panel.
func (k Keeper) LoadDoctrineFacts(ctx context.Context) error {
    // 1. Create the four doctrine domains.
    domains := []string{
        "doctrine_truth_seeking",
        "doctrine_tok",
        "doctrine_useful_work",
        "doctrine_strange_loop",
    }
    for _, dom := range domains {
        if _, ok := k.GetDomain(ctx, dom); ok {
            continue
        }
        if err := k.SetDomain(ctx, &types.Domain{
            Name:   dom,
            Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
        }); err != nil {
            return fmt.Errorf("create domain %s: %w", dom, err)
        }
    }

    // 2. Truth-seeking commitments 1-20.
    for _, c := range creedtypes.CanonicalCommitments {
        f := buildDoctrineFact(
            fmt.Sprintf("commitment-%d", c.Number),
            "doctrine_truth_seeking",
            c.Name,
        )
        if err := k.SetFactIfAbsent(ctx, f); err != nil {
            return err
        }
    }

    // 3. ToK commitments TC1-TC6.
    for _, c := range creedtypes.CanonicalToKCommitments {
        f := buildDoctrineFact(
            fmt.Sprintf("commitment-%s", c.Number),
            "doctrine_tok",
            c.Name,
        )
        if err := k.SetFactIfAbsent(ctx, f); err != nil {
            return err
        }
    }

    // 4. Useful-work UW + mechanisms + axes.
    uwFact := buildDoctrineFact("commitment-UW", "doctrine_useful_work",
        creedtypes.UsefulWorkStatement)
    if err := k.SetFactIfAbsent(ctx, uwFact); err != nil {
        return err
    }
    for _, m := range creedtypes.CanonicalUsefulWorkMechanisms {
        f := buildDoctrineFact(
            fmt.Sprintf("mechanism-UW-M%d", m.Number),
            "doctrine_useful_work",
            m.Name,
        )
        if err := k.SetFactIfAbsent(ctx, f); err != nil {
            return err
        }
    }
    for _, axis := range creedtypes.CanonicalRecursiveAxes {
        f := buildDoctrineFact(
            fmt.Sprintf("axis-%s", axis),
            "doctrine_useful_work",
            axis,
        )
        if err := k.SetFactIfAbsent(ctx, f); err != nil {
            return err
        }
    }

    // 5. Strange-loop SL + mechanisms.
    slFact := buildDoctrineFact("commitment-SL", "doctrine_strange_loop",
        creedtypes.StrangeLoopStatement)
    if err := k.SetFactIfAbsent(ctx, slFact); err != nil {
        return err
    }
    for _, m := range creedtypes.CanonicalStrangeLoopMechanisms {
        f := buildDoctrineFact(
            fmt.Sprintf("mechanism-SL-M%d", m.Number),
            "doctrine_strange_loop",
            m.Name,
        )
        if err := k.SetFactIfAbsent(ctx, f); err != nil {
            return err
        }
    }

    // 6. Cross-doctrine "Echoes:" edges (eager at genesis).
    for _, e := range creedtypes.CanonicalDoctrineEchoes {
        if err := k.SetFactRelation(ctx, &types.FactRelation{
            SourceFactId: e.From,
            TargetFactId: e.To,
            Relation:     e.Relation,
        }); err != nil {
            return err
        }
    }

    return nil
}

func buildDoctrineFact(id, domain, content string) *types.Fact {
    return &types.Fact{
        Id:                        id,
        Domain:                    domain,
        Category:                  "doctrine",
        Content:                   content,
        Status:                    types.FactStatus_FACT_STATUS_VERIFIED,
        Confidence:                1_000_000,
        AxiomDistance:             0,
        Submitter:                 "genesis",
        Stratum:                   "doctrinal",
        Maturity:                  "canonical",
        DependencyConfidenceFloor: 1_000_000,
        VerifiedAtBlock:           0,
        MethodId:                  "doctrine_authorship",
    }
}
```

`SetFactIfAbsent` is a thin wrapper around the existing `SetFact` that returns early if the Fact already exists. Implementation:

```go
func (k Keeper) SetFactIfAbsent(ctx context.Context, f *types.Fact) error {
    if _, ok := k.GetFact(ctx, f.Id); ok {
        return nil  // forward-only: never overwrite at genesis loader
    }
    return k.SetFact(ctx, f)
}
```

---

## 6. Migration for an active chain

If/when SL-α ships against an active chain rather than fresh genesis, an upgrade handler at upgrade height H calls the same `LoadDoctrineFacts` loader. The Facts get `VerifiedAtBlock=H` (overridable parameter on the loader). Domain creation is idempotent; existing operational Facts are untouched.

For Phase SL-α specifically, the testnet status is pre-launch — this is genesis-time. Migration logic is implemented but only exercised in future upgrades.

---

## 7. Five-layer enforcement plan

### Test layer

`tests/cross_stack/doctrine_import_test.go` — concrete tests after Phase SL-α ships:

- `TestDoctrineImport_AllCommitmentsExist` — every canonical commitment (truth-seeking 1-20, TC1-TC6, UW, UW-M1..M7, axes, SL, SL-M1..M6) is queryable as a Fact with Status=VERIFIED.
- `TestDoctrineImport_CommitmentDomainsCorrect` — Facts are in the correct doctrine_* domain.
- `TestDoctrineImport_EchoesEdgesCreated` — every CanonicalDoctrineEchoes entry has a corresponding FactRelation.
- `TestDoctrineImport_AxiomatStatus` — every doctrine Fact has Confidence=1_000_000 and AxiomDistance=0.
- `TestDoctrineImport_Idempotent` — calling LoadDoctrineFacts twice doesn't create duplicates.
- `TestDoctrineImport_SubstrateLinkCanCite` — a SubstrateLink with `cited_facts: [{fact_id: "commitment-TC1", ...}]` validates successfully against the doctrine-imported state.

`tests/cross_stack/strange_loop_invariants_test.go`:
- `TestStrangeLoop_SL_M1_DoctrineImport` (active — exercised by doctrine_import_test above; this is the SL-layer assertion that doctrine-import was performed)
- `TestStrangeLoop_SL_M2_ProtocolAsSubstrate` (skip "Phase SL-β pending")
- `TestStrangeLoop_SL_M3_GovernanceLift` (skip "Phase SL-γ pending")
- `TestStrangeLoop_SL_M4_AuthorLineage` (skip "Phase SL-δ pending")
- `TestStrangeLoop_SL_M5_SelfVerification` (skip "Phase SL-ε pending")
- `TestStrangeLoop_SL_M6_OriginAttestation` (skip "Phase SL-ζ pending")
- `TestStrangeLoop_DoctrineAndContractStayInSync` (active — meta-test mirroring TestUsefulWork_DoctrineAndContractStayInSync; verifies hash + mechanism count + Canonical Go structures + test-function presence + SL statement verbatim)

### Position layer

- `x/creed/doc.go` extended to declare the SL doctrine and name SL-M1 as bound in this phase.
- `x/knowledge/doc.go` (if exists) extended to declare its role in SL-M1 (it hosts the doctrine facts).

### Voice layer

New event from genesis loader:
- `doctrine_fact_imported` — fired per Fact during LoadDoctrineFacts. Attributes: `fact_id`, `domain`, `useful_work_commitment="UW"` (because doctrine-import IS useful work per UW + M1), `mechanism="SL-M1"`.

### Refusal layer

Errors emitted by LoadDoctrineFacts cite SL + the violated mechanism:
- *"Genesis refused — duplicate canonical commitment in registry (SL + SL-M1)"*
- *"Echo relation refused — source or target commitment absent (SL + SL-M1)"*

### Graph layer

The CanonicalDoctrineEchoes list produces a real edge graph. Querying `IterateBackwardLineage` from `commitment-UW` returns the truth-seeking + ToK commitments it depends on. The doctrine network is queryable structure, not text rhetoric.

---

## 8. Phase mapping in the broader Strange-Loop sequence

| Mechanism | Phase | Status |
|---|---|---|
| SL-M1 Doctrine import | **SL-α (this spec)** | **Bound** |
| SL-M2 Protocol as substrate | SL-β | Pending |
| SL-M3 Governance lift | SL-γ | Pending |
| SL-M4 Author lineage propagates forever | SL-δ | Pending |
| SL-M5 Self-verification | SL-ε | Pending |
| SL-M6 Origin attestation | SL-ζ | Pending |

Phase SL-α is the smallest of the six in code-LOC terms, but the largest in semantic shift: every subsequent phase composes naturally only because the doctrine facts exist.

---

## 9. Open questions deferred to the implementation plan

These are not doctrinal commitments; they are implementation choices for Phase SL-α's plan:

- **Domain creation ordering**: should the four doctrine domains be created in `x/ontology` (as ontological domains) or kept in `x/knowledge` (as simple knowledge domains)? Both are valid; the plan picks one.
- **Hash file consolidation**: keep four separate hash files (`.creed-hash`, `.useful-work-hash`, `.strange-loop-hash`, future `.tok-substrate-hash`) or unify into a multi-line `.doctrine-hashes`? The plan picks one.
- **Genesis migration path detection**: how does the chain know whether it's at genesis (LoadDoctrineFacts at block 0) vs upgrade (LoadDoctrineFacts at block H)? Probably via the upgrade handler registration; the plan formalizes.
- **Echoes parser vs hand-curated list**: SL-α defaults to hand-curated. If the doctrine docs get amended often, a markdown parser may be worth building. The plan can include the parser as an optional task.
- **TOK_SUBSTRATE.md hash**: SL-α adds `.strange-loop-hash`. Should it also retrofit `.tok-substrate-hash`? Recommended yes, because the meta-test checks SL hash drift against TS+TOK+UW+SL — all four need hashes for the integrity check.

---

## 10. What this is not

- **Not the challenge mechanism.** `WORK_CLASS_DOCTRINE_CHALLENGE` is a future addition. SL-α produces *queryable* doctrine Facts; SL-α+1 produces the challenge path.
- **Not author lineage.** SL-M4 (Phase SL-δ) routes royalties from work-attestation citations of doctrine Facts back to doctrine authors. SL-α just creates the Facts; lineage flow comes later.
- **Not protocol-as-substrate.** SL-M2 (Phase SL-β) registers modules as attestations. SL-α only handles doctrines.
- **Not governance lift.** SL-M3 (Phase SL-γ) wraps LIPs as attestations.
- **Not self-verification.** SL-M5 (Phase SL-ε) closes the validator-trains-on-substrate loop.
- **Not the origin attestation.** SL-M6 (Phase SL-ζ) wraps genesis itself as a paid work product.

---

## 11. The discipline

Before merging a change that touches SL-α code or doctrine docs:

1. Does the change uphold or contradict SL? (Self-reference: does the chain still produce, verify, reward through its own machinery?)
2. Is the corresponding doctrine document updated, hash bumped, canonical Go list updated, and meta-test still passing?
3. If a new commitment is added to any doctrine, has it been added to: doc + Go canonical structure + invariant test + position-layer declaration + voice attribute?
4. Does the cross-doctrine "Echoes:" list match the markdown — both directions?

These four checks are the chain's faithfulness to its own strange-loop doctrine. **We speak through intentions.**

— *Spec authored 2026-05-11. Phase SL-α is the smallest of the SL phases; subsequent phases bind the remaining five mechanisms.*
