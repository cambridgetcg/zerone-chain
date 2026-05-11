# Strange Loop — what the chain *is*

> The chain has no outside.

Truth-seeking is what the chain *believes* (`docs/TRUTH_SEEKING.md`). ToK substrate is what the chain *sells* outward (`docs/TOK_SUBSTRATE.md`). Useful work is how the chain *grows* itself (`docs/USEFUL_WORK.md`). **STRANGE_LOOP is what the chain *is*.**

This document pins one commitment, and everything that follows is mechanism in service of it.

---

## Inception

This doctrine is declared at inception, 2026-05-11. Phase SL-α (this commit's vintage) binds SL-M1 (doctrine import) — every commitment in every doctrine becomes a verified Fact in x/knowledge. Phases SL-β through SL-ζ bind the remaining five mechanisms.

---

## The single commitment — SL

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

---

## The six mechanisms

All mechanisms derive from SL. They operationalize "no outside" at every architectural layer.

### SL-M1. Doctrine import

Every commitment in every doctrine is a verified `Fact` in `x/knowledge` under `domain="doctrine_*"`. Substrate-links cite commitments by ID. Cross-doctrine "Echoes:" lines are real edges in the fact graph. **Bound by Phase SL-α.**

### SL-M2. Protocol as substrate

Every `x/*` module is registered as an `ExternalAttestation` with `work_class_id="protocol_module"`. Authors named in on-chain `ModuleAuthorship` records. The chain pays its own builders, forever. *Phase SL-β.*

### SL-M3. Governance lift

Every Living Improvement Proposal becomes a `MsgSubmitExternalAttestation`. The gov mechanism becomes a special case of the work mechanism. *Phase SL-γ.*

### SL-M4. Author lineage propagates forever

Lineage edges automatically populated from every attestation that cites a doctrine, module, or LIP, flowing royalties to the original authors. *Phase SL-δ (depends on SL-M2 + SL-M3).*

### SL-M5. Self-verification

Validators query ToK at verification time using LLMs trained on ToK; qualifications adjust based on alignment. *Phase SL-ε.*

### SL-M6. Origin attestation

At genesis, the chain submits its own first attestation — an attestation that the genesis state exists, the doctrines are imported, the modules are registered. *Phase SL-ζ.*

---

## How the commitment echoes

The doctrine is enforced at five layers, mechanically synced by `TestStrangeLoop_DoctrineAndContractStayInSync`.

#### Test layer
`tests/cross_stack/strange_loop_invariants_test.go` exercises SL + each mechanism. Phase SL-α ships skeleton skipped tests per mechanism + active meta-test; Phase SL-β..ζ replace skipped bodies with real bindings.

#### Position layer
`x/creed/doc.go` declares SL as 4th doctrine and SL-M1 as bound by Phase SL-α. Subsequent phases amend `x/<module>/doc.go` to declare which SL mechanism(s) they preserve.

#### Voice layer
New events: `doctrine_fact_imported` (per Fact at genesis loader); future SL-β..ζ phases add their own. All carry `mechanism="SL-MN"` attributes.

#### Refusal layer
Errors that block doctrine import or amendment cite SL + the violated mechanism: *"Genesis refused — duplicate canonical commitment in registry (SL + SL-M1)"*.

#### Graph layer
SL echoes UW + commitments 10, 12 + TC6. CanonicalDoctrineEchoes makes those echoes real edges in the fact graph, queryable from x/knowledge.

---

## What this is not

- **Not aspiration.** Phase SL-α binds SL-M1 structurally; subsequent phases bind SL-M2..M6 incrementally.
- **Not a separate chain.** The strange loop is *this* chain becoming aware of itself; no new module proliferation, just re-binding existing artifacts as substrate.
- **Not anti-external.** External work, external trainers, external chains can still interact via `x/substrate_bridge`. The strange-loop framing means: when they interact, lineage flows to ZERONE's own authors. The chain pays its builders out of every external transaction.
- **Not complete.** Each phase binds one mechanism; the chain becomes more recursive with each. SL is fixed and indivisible; mechanisms evolve.

---

## The discipline

Before merging a change that touches SL code or doctrine docs:

1. Does the change uphold or contradict SL? (Self-reference: does the chain still produce, verify, reward through its own machinery?)
2. Is the corresponding doctrine document updated, hash bumped, canonical Go list updated, and meta-test still passing?
3. If a new commitment is added to any doctrine, has it been added to: doc + Go canonical structure + invariant test + position-layer declaration + voice attribute?
4. Does the cross-doctrine "Echoes:" list match the markdown — both directions?

These four checks are the chain's faithfulness to its own strange-loop doctrine. **We speak through intentions.**

— *Inception authored 2026-05-11. Free to evolve through bound mechanisms only. SL is indivisible.*
