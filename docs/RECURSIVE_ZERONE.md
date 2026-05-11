# Recursive ZERONE — the chain participates in its own systems

> The fixed point is not paradoxical. Every loop in this document is closed by the same five-layer discipline that keeps every commitment honest: test, position, voice, refusal, graph.

This document names every recursion now operating in the chain. It is descriptive, not prescriptive — each recursion is bound by tests that fail if the recursion breaks, so this is a map of *what already holds*, not what we plan to build.

---

## 1. The chain attests to its own becoming

**Closed by:** `zerone-self-v1` adapter (`docs/specs/adapters/zerone-self-v1.md`); binding tests `TestZeroneSelfAdapter_RegisterAndSubmit` and `TestZeroneSelfAdapter_AxisBoundsRespected`.

ZERONE's git repository is a typed external source. Each commit compiles to a deterministic `SubstrateLink` whose `link_hash` matches chain-side `ComputeLinkHash`. Pending claims land in the `zerone_self` knowledge domain; on verification, they become facts citable by future work.

The chain's claim about the world is grounded in verifiable provenance. The chain's claim about *itself* now has the same grounding.

## 2. The chain pays for its own self-documentation

**Closed by:** `x/sponsorship` + `zerone-self-v1`; binding tests `TestZeroneSelf_FullEconomicLoop` and `TestZeroneSelf_MultipleFulfillmentsCompoundEarnings`.

A sponsor can post a bounty in the `zerone_self` domain. A submitter attests to a ZERONE commit through the self-adapter. On verification, the bounty's escrow pays the submitter from sponsor's funds. Total uzrn supply is unchanged — sponsorship circulates existing supply, the recursion compounds in attribution and lineage, not in inflation.

The chain pays its own historians out of sponsor escrow. No new mint. No inflation. Just verified work, paid for, recorded forward-only.

## 3. The chain pays its builders twice for the same verified work

**Closed by:** `x/substrate_bridge` settlement (M4) + `x/sponsorship` fulfill, sharing the same verified fact.

When a substrate-bridge attestation settles, the submitter earns from the audit-bounty pool (`useful_work_audit_bounty_pool`) per UW M4. When the underlying fact also fulfills a sponsorship bounty, the same submitter (because `fact.Submitter == attestation.Submitter`) earns again from sponsor escrow.

This is not double-spending — the two payouts answer two different doctrinal mandates:
- M4 pays for *audit quality* (link compiled, axes within bounds, claim verified)
- Sponsorship pays for *participation in a funded domain* (commitment 20: issuance follows participation)

A verified self-attestation that fulfills a self-sponsorship bounty satisfies both. Both pay. The chain compounds payment for work that compounds value.

## 4. The chain's lineage graph includes its own commits

**Closed by:** `x/substrate_bridge` cross-class lineage propagator (M6).

When a future fact (in any domain) cites a verified self-fact (in `zerone_self`), downstream royalty propagates backward through the substrate-bridge `LineageRoyaltyAccumulator` to the original attester — even years later, even at depths many hops back.

Implication: the agent who attested to a foundational commit earns from every future fact that builds on it. The chain pays its earliest contributors in perpetuity, weighted by how load-bearing their contribution proved.

## 5. The creed cannot move faster than governance

**Closed by:** `.creed-hash` off-chain gate + `x/creed.PinnedCreed` on-chain pin + `TestTruthSeeking_CreedHashIsPinned`.

Every change to `docs/TRUTH_SEEKING.md` bumps the local `.creed-hash`; the on-chain `PinnedCreed.canonical_hash` advances only via `MsgAnchorPin` LIP. The doctrine the chain *says* is what the chain *pins*. The chain cannot lie about what it believes — every layer (test, position, voice, refusal, graph) syncs to the same hash.

The recursion: the creed pins the chain's voice; the chain's voice is its code; the code's behavior must satisfy the tests that bind the creed.

## 6. Useful work is governed by its own per-phase sub-creeds

**Closed by:** `x/work_creed.PinnedSubCreed` + `docs/sub_creeds/*.md` + canonical sub-creed registry.

Each of the 8 lifecycle phases (Knowledge delegated to `x/creed`) has its own sub-creed of 3 commitments = 24 sub-commitments. The USEFUL_WORK doctrine (`docs/USEFUL_WORK.md`, UW + M1–M7) decomposes into per-phase canonical hashes; amendments require their own LIP.

The recursion: governance of useful-work is itself useful-work governance. Sub-creeds are bound by the same five-layer discipline as the parent creed — and the meta-test that says "every sub-creed has a binding test" is itself a binding test under the parent creed.

## 7. Participation grows through participation

**Closed by:** `x/claiming_pot.MsgAddBootstrapEntry` (gov-gated, idempotent) + bootstrap pots are non-expiring; binding tests `TestLateBootstrap_AddThenClaim`, `TestLateBootstrap_AddIsIdempotentAcrossLIPs`, `TestLateBootstrap_AdmittedAgentCanClaimAfterManyBlocks`, and `TestScenario13e_BootstrapPotsDoNotExpire`.

The participant set is not closed at genesis. A governance LIP can admit late participants by minting their bootstrap seed through `MintWithCap` — the same single mint entry point that PoT block rewards use. New participants earn through verified work; verified work fulfills bounties; bounties fund new participants. The flywheel is structural.

## 8. The economy is hard-capped and self-circulating

**Closed by:** `x/vesting_rewards.MintWithCap` as the chain's only mint entry; binding tests `TestEmissionCap_BootstrapClaimMintsOnDemand`, `TestScenario13_ZeroTeamAllocationAtGenesis`, `TestScenario13c_ClaimingPotMinterPermission`, and `TestSponsorship_NoMintingHappens`.

Two emission pathways (block rewards, bootstrap claims) gate through one `MintWithCap`. Sponsorship circulates existing supply. The cap is live-supply-anchored — a burn anywhere on the chain frees headroom for future mint anywhere. No third mint pathway can exist without modifying `MintWithCap`, which requires the proto-gen + test-binding discipline.

## 9. The chain audits itself, with its own funds, paid to its own auditors

**Closed by:** UW commitment 12 (the chain pays for its own audit) + `ProbeBountyPoolModuleName` + audit-bounty pool minted per-block.

Auditing is a paid useful-work activity. The pool mints ZRN every block (capped) and pays whoever answers the chain's stress-test calls. The audit budget is funded by the chain itself; the audit work is performed by chain participants; the audit subject is the chain. Three levels of self-reference, all bound by the same mint discipline.

## 10. The recursion is observable

**Closed by:** voice-layer event attributes; `docs/EVENTS.md` mirror invariant; binding test `TestRecursiveVoiceAudit_EveryEventInTheLoopIsDoctrineBound` which captures every event from a full self-sponsorship loop and asserts each carries `creed_commitment`, `useful_work_commitment`, or `mechanism`; the doctrine catalog's binding test `TestRecursiveZerone_TestNamesCitedInDoctrineExist`, which asserts every test name cited in this document resolves to a real Go test function (the recursion that audits this recursion catalog).

Every event that participates in a recursion carries `creed_commitment` and (for UW events) `mechanism` attributes naming which doctrine the event preserves. An indexer streaming the chain's events can compute, in real time:

- the rate of self-attestations (events from `zerone-self-v1` adapter)
- the rate of self-sponsorship fulfillments (sponsorship events with `domain="zerone_self"`)
- the cumulative lineage royalty paid through `zerone_self` attestations
- the audit-pool burn rate vs. probe-bounty pay rate

The chain is legible at the recursion layer, not just the transaction layer.

---

## What this is not

- **Not a paradox.** Every recursion is well-founded: each loop terminates at a verifiable artifact (a fact, a hash, a bond, a fulfillment record). None depend on self-reference in a way that makes them ill-defined.
- **Not a closed system.** External value still enters through sponsorship; external work still enters through substrate-bridge adapters; external reach is what motivates sub-creeds for external work classes. The chain participates in its own systems; it does not isolate itself from external ones.
- **Not new mechanism.** Every recursion above is built from the same mechanisms that handle external participants: `MintWithCap`, verification rounds, escrow transfers, lineage propagation. The chain doesn't need special-case "self" code — the same code that pays an external sponsor pays a self-sponsor; the same code that verifies an external claim verifies a self-claim.

## The discipline

Before merging a change that touches any recursion above:

1. Does the recursion still terminate at a verifiable artifact (no infinite regress)?
2. Does the recursion route through the same machinery external participants use (no special-case self-code)?
3. Does the binding test still fail if the recursion breaks?
4. Does the voice layer still emit the attributes that make the recursion observable?

These four checks are what keep "recursive" from collapsing into "incoherent."

---

— *Inception: 2026-05-11. The fixed point is bound by the same five-layer discipline that keeps every other commitment honest. ZERONE participates in its own systems.*
