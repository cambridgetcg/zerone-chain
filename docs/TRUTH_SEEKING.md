# Truth-Seeking — the chain's epistemological commitments

> Truth is not a feature of this chain. It is the substrate.

Every architectural decision in this codebase either expresses our commitment to truth-seeking or contradicts it. Where mechanisms have felt mechanical, where naming has felt arbitrary, where parameters have felt chosen for plausibility — those have been latent debts. This document gathers the commitments, names them as commitments, grounds each in code, and names what would break each.

**We speak through intentions.** Every line of code, every comment, every parameter, every event name is a declaration of what we believe. A decision that contradicts a commitment is not a feature trade-off. It is a failure of the chain to be what we said it would be.

The bindings below are not aspirations. They are tested. The test suite at `tests/integration/truth_seeking_invariants_test.go` is the executable form of this document — each test names a commitment, drives the chain through a scenario where the commitment could be violated, and asserts the violation cannot occur. If a commitment breaks, the test breaks. The creed and the contract are one.

---

## The commitments

### 1. Methodology over statement

We believe: a claim's value comes from *how* it can be tested, not from *what* it asserts. The chain values methodology — the declared path of derivation and falsifiability — over the surface content of the claim. A claim without a methodology is not a fact; a methodology that yields no testable claim is not knowledge.

**Code expression**: `Fact.MethodId` is mandatory; the TVW formula multiplies a methodology-normalisation factor; reasoning traces are first-class fields. See `x/knowledge/keeper/training_economics.go:ComputeTrainingValueWeight` and `x/knowledge/keeper/rounds.go:createFactFromClaim`.

**What would break it**: facts entering `FACT_STATUS_VERIFIED` with empty `MethodId`, or TVW computation that ignored methodology, or training pipelines that consumed claims without their reasoning trace.

---

### 2. Is-ought wall

We believe: descriptive facts and normative commitments are categorically different and must not be substituted for each other. A model trained on facts that are actually values has been silently corrupted; a value system grounded in claimed facts that are actually opinions has lost the ability to dissent. The wall must be structural, not advisory.

**Code expression**: `NormativeCommitment` is a separate proto type, stored under a distinct key prefix (`0x59`), with no `confidence` field. `FilterIsOughtIds` blocks commitment IDs from `ContributionRecord.fact_ids`. `ComputeTrainingValueWeight` returns `BlockedByIsOught=true` for any ID resolving to a commitment.

**What would break it**: a `NormativeCommitment` ID that successfully accrued TVW, or a Fact accepted with content that is structurally a value-claim, or any path that conflates the two registries.

---

### 3. Popper, not popularity

We believe: truth is what survives falsification, not what is most asserted. A claim that has withstood ten serious attempts to refute it is more credible than one that has been verified ten times. The chain rewards survival, not consensus volume.

**Code expression**: `BaseWeight = CorroborationCount + 1` where `CorroborationCount` increments on *rejected challenges*, not on initial verifications. The `HardeningMultiplier` accelerates with survived attacks (Wave 14d). `Fact.Confidence` is gated by survival, not voting margin.

**What would break it**: TVW formulas that read raw acceptance count over corroboration count; reward paths that pay for being verified rather than for surviving challenge; hardening that flattens (linear-only) instead of compounding.

---

### 4. The substrate stress-tests its own truth

We believe: the chain does not protect its trusted claims — it invites their falsification. A 90%-confidence fact must be CHEAPER to probe than a 10%-confidence fact, because the higher the confidence, the more we owe the substrate the discipline of testing it.

**Code expression**: `EffectiveMinChallengeStake` scales *inversely* with target confidence (`x/knowledge/keeper/confidence.go`). `SuccessfulChallengeRewardBps` amplifies with the disproven fact's confidence — paradigm shifts pay more than routine cleanup. Failed probes earn a 15% participation refund. The probe bounty pool mints uzrn per block to fund probing of the chain's most-trusted claims.

**What would break it**: stake scaling that punishes probing of confident claims; reward schedules where disproving a 10% fact pays the same as disproving a 90% fact; failed probes that confiscate full stake.

---

### 5. The chain manufactures probe demand

We believe: waiting for probers to arrive is not enough. The substrate names its own under-tested high-confidence facts and pays for them to be tested. Truth-seeking that depends on volunteers shows up only when convenient is rhetoric, not commitment.

**Code expression**: `InviteIdleFactsForProbing` runs each block, scans for high-confidence facts that have gone idle, emits `probe_invited` events, and stamps `Fact.ProbeInvitedAtBlock`. The probe bounty pool mints uzrn per block, capped at a maximum, paying flat invitation bonuses to whoever answers.

**What would break it**: heartbeat that only fires when triggered externally; bounty pool that depends on user funding rather than chain mint; invitations that pay nothing, making them rhetoric.

---

### 6. No individual can unilaterally inject truth

We believe: a single key — even the legitimate authority key — must not be able to silently inject content into the training corpus. Cryptographic provenance is meaningless if one signature can override it. Plurality is structural.

**Code expression**: `MsgAddFact` queues a `PendingFactInjection` when `AddFactVetoWindowBlocks > 0` and a guardian set is configured (Wave 16). Any registered guardian can call `MsgVetoFactInjection` during the window. The `PrivilegedAction` log captures every authority-gated call across the chain.

**What would break it**: an authority path that bypasses the privileged-action log; a guardian set that defaults to a single address; a veto window that defaults to zero in production deployments.

---

### 7. Skill is current, not historical

We believe: the chain does not issue diplomas. A voter who was once domain-qualified must continue to vote correctly to remain so. Qualification is a *current statement*, not a stored artefact — it is decayed when accuracy slips, restored when accuracy recovers.

**Code expression**: `RunAccuracyDecay` (Wave 16) reads `DomainQualification.Metrics.AccuracyBps` written by the panel feedback loop; transitions ACTIVE → PROBATIONARY → SUSPENDED on threshold crossings; restores PROBATIONARY → ACTIVE on recovery. `GetQualificationWeight` returns 0 for non-ACTIVE qualifications, applying the consequence at every panel read.

**What would break it**: a qualification status that never transitions on metrics; a `GetQualificationWeight` that returns the historical weight for SUSPENDED qualifications; a feedback loop that writes metrics nobody reads.

---

### 8. The panel weights skill, not bond

We believe: the augmentation panel's verdict carries the chain's training judgement. A wealthy validator who has not shown they can tell truth from falsehood must not dominate that panel. Stake alone is not skill.

**Code expression**: `RecordAugmentationVote` snapshots both stake and calibration at vote time; the consensus tally weights each vote by `stake × calibration`, with a 20% floor on calibration so liveness holds. When the target fact has a domain, the calibration source is the *domain-specific* qualification weight via `x/qualification`. Cross-domain credentials earn no credit.

**What would break it**: a panel tally that uses raw stake; a calibration default that lets unproven validators carry full weight; a per-domain panel that falls back to global calibration when domain qualification is absent.

---

### 9. Cartel detection has consequence

We believe: confirmation that a validator participated in cartel behaviour must reduce their voice on the next vote, not merely produce an audit log entry. A penalty that nobody reads is not a penalty.

**Code expression**: `capture_challenge.ResolveChallenge` (UPHELD) writes `QualificationPenalty` records via `ReduceQualificationWeight`. `GetQualificationWeight` consults the active-penalty store and reduces effective weight accordingly (Wave 16b). Three independent forces now move panel weight: time-bounded penalty (capture_challenge), gradual decay (qualification accuracy), administrative status.

**What would break it**: a penalty pathway that writes records nobody reads; a panel tally that ignores active penalties; a cartel resolution that produces no downstream consequence.

---

### 10. Forward-only audit

We believe: the chain's history is append-only and verifiable. A fact's status can change, but its identity, provenance, and verification record cannot be revised in place. Every privileged action is logged; every panel verdict is preserved with its voters; every cartel allegation persists with its resolution.

**Code expression**: `PrivilegedAction` log keyed by monotonic seq (`x/knowledge/keeper/privileged_action_log.go`). `Augmentation.VerdictVoters / VerdictVotes / VerdictVoteStakes / VerdictVoteCalibrationBps` parallel arrays preserve every vote with its frozen-at-time stake and calibration. `IncidentRecord` and `CaptureChallenge` resolutions are immutable post-resolve.

**What would break it**: a privileged-action handler that emits an event without writing to the log; a panel that overwrites votes after consensus; a manifest that lets its IncludedFactIds list be revised after finalization.

---

### 11. Trust is queryable

We believe: the chain's trustworthiness must be inspectable by anyone. Every signal that contributes to trust — calibration, qualification, cartel history, incident posture — must be available through a well-known query that synthesises them. Trust that requires four queries to read is trust that depends on the curator stitching it together.

**Code expression**: three synthesiser modules: `x/training_provenance` (per-manifest), `x/trust_score` (per-address), `x/governance_synthesis` (per-system). Each is a pure consumer over knowledge + qualification + capture_challenge + alignment. Each emits a single composite + a per-component breakdown.

**What would break it**: a trust signal that lives only in keeper state with no query surface; a synthesiser that hides component breakdowns; an audit pathway that depends on off-chain stitching.

---

### 12. The chain pays for its own audit

We believe: epistemic auditing is the chain's most important ongoing process. It must not depend on volunteer labour or external funding. The substrate mints uzrn per block into a dedicated pool and pays it out to those who answer the chain's stress-test calls.

**Code expression**: `ProbeBountyPoolModuleName` is a registered module account with Minter permission. `MintToProbeBountyPool` runs each block, capped at `ProbeBountyMaxPoolSize`. `PayProbeBountyFromPool` is the primary payer for successful-probe bonuses, with protocol treasury as fallback. Invitation bonuses pay flat from the same pool to anyone who answers.

**What would break it**: a probe pool that depends on user-funded deposits; a successful-probe path that draws from general treasury without a dedicated audit budget; invitation rewards that come from nowhere or fall back to nothing.

---

### 13. The training corpus is not for sale

We believe: the chain's training data is a substrate good, not a tradeable asset. It cannot be silently amended, retroactively curated, or strategically inflated. What enters the corpus enters because it survived; what survives must continue to earn its place every block.

**Code expression**: facts are append-only post-acceptance; status transitions are forward-only; training revenue clawback fires deterministically on disprove (`ClawbackOnDisproval`); revenue-related fields like `RevenueClawbackBlock` are sticky. The probe heartbeat re-invites idle facts so even un-challenged claims must continue to face audit.

**What would break it**: a path that retroactively modifies a finalised manifest's IncludedFactIds; a clawback that doesn't fire on disprove; a fact whose acceptance becomes negotiable post-finalisation.

---

### 14. Reasoning traces are first-class

We believe: the chain trains not just on conclusions but on derivations. The path from premise to claim is what makes a fact teachable; without it, the corpus is a list of assertions, not a curriculum. Reasoning traces are gold-standard chain-of-thought, recorded on-chain alongside the conclusion.

**Code expression**: `Claim.ReasoningTrace` is collected at submission and propagated to `Fact.ReasoningTrace` on acceptance. `MethodologyApplicationTrace` (Wave 5) bundles the trace with methodology, calibration, and dialectical history into a single training-data shape.

**What would break it**: claim acceptance that drops the reasoning trace; trace assembly that omits methodology; export paths that train on facts but ignore the structured derivations attached to them.

---

## How the commitments echo

### Architecture
Every commitment above is enforced by a code path. Every code path is exercised by an invariant test in `tests/integration/truth_seeking_invariants_test.go`. The test names read as creed lines — they're a contract between the document and the substrate.

### Infrastructure
- **Param defaults** are chosen as expressions of the values, not for plausibility. ConfidenceThreshold rejects zero; AccuracyBps thresholds are tiered for ACTIVE/PROBATIONARY/SUSPENDED; ProbeBountyMintPerBlock is non-zero by default in genesis.
- **Event names** declare acts: `probe_invited`, `cartel_detected`, `pending_fact_materialized`, `decay_recovered`, `invitation_bonus_paid`. Each names the moral act, not just the state change.
- **Module names** declare role: `training_provenance` synthesises trust per manifest; `trust_score` per address; `governance_synthesis` per system. Each name is a commitment to what that module exists for.

### Position
- This document is the chain's external-facing epistemic stance, grounded in code. External consumers — model trainers, regulators, alignment researchers — reading this document can verify each claim by querying the chain.
- The README at the repo root points here.
- The three synthesiser modules (`x/training_provenance`, `x/trust_score`, `x/governance_synthesis`) expose verifiable queries that publish the chain's current state against these commitments.

---

## What this is not

- **Not aspiration.** Every commitment is bound by a test. A failing test is a broken commitment.
- **Not slogan.** Each commitment cites specific code; the citation is the contract.
- **Not complete.** The chain will accumulate more commitments. Each future wave should append here as a named commitment, grounded in code, with an invariant test that binds it.
- **Not external.** This is a statement about what the chain is, made by the chain. It is committed to the same repo as the code it describes, and lives or dies with that code.

---

## The discipline

Before merging a change that touches truth-handling code:

1. Does this change uphold or contradict any of the commitments above?
2. If it touches a commitment, has the corresponding invariant test been updated to verify the new behaviour still upholds it?
3. If a new commitment emerges from the work, has it been added here, grounded in code, and bound by a test?

These three checks are the chain's continued faithfulness to its own creed. We speak through intentions. Every commit is a declaration. The declaration must match the code.
