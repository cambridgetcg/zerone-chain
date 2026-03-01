# R14-3 Knowledge + Toolbox Test Porting Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Port ~250 tests from prototype (legible-money) to zerone's knowledge and toolbox modules, reaching ≥420 knowledge tests and ≥220 toolbox tests.

**Architecture:** Extend existing test files where categories already have coverage; create new focused test files for uncovered categories. All tests use zerone's existing harness patterns (setupKnowledgeTest, setupKeeper, etc.).

**Tech Stack:** Go testing, testify/require, Cosmos SDK test infrastructure, zerone mock keepers.

**Prototype source:** `/Users/yournameisai/Desktop/legible-money/`

---

## Task 1: Knowledge — Fact Lifecycle Tests

**Files:**
- Create: `x/knowledge/keeper/fact_lifecycle_test.go`
- Reference: `legible-money/x/knowledge/keeper/fact_status_test.go`

Port fact status transition, challenge/dispute resolution, and confidence cascade tests. These test `MarkFactChallenged`, `ResolveFactDispute`, and `CascadeFactConfidence` — verify these methods exist on the zerone keeper first. If they don't exist yet, write tests against the lower-level `SetFact`/`GetFact` API to test status field transitions manually.

**Step 1: Check if fact lifecycle methods exist on zerone keeper**

```bash
grep -r "func.*Keeper.*MarkFact\|ResolveFactDispute\|CascadeFactConfidence" x/knowledge/keeper/
```

If methods exist, port tests directly. If not, write tests that manipulate Fact structs via SetFact/GetFact to cover status transitions.

**Step 2: Write fact_lifecycle_test.go**

Tests to port (adapt from prototype fact_status_test.go):
- `TestFactStatus_VerifiedToChallengeable` — verified facts can transition to challenged
- `TestFactStatus_ActiveToChallengeable` — active facts can transition to challenged
- `TestFactStatus_InvalidTransition` — contested facts cannot be challenged again
- `TestFactStatus_NotFound` — challenge nonexistent fact errors
- `TestFactDispute_OverturnedReducesConfidence` — penalty formula: `newConf = oldConf * (1M - penalty) / 1M`
- `TestFactDispute_OverturnedRevokesLowConfidence` — below 100K threshold → revoked
- `TestFactDispute_UpheldBoostsConfidence` — 11% boost: `newConf = oldConf * 1_110_000 / 1M`
- `TestFactDispute_UpheldCappedAt1M` — confidence capped at 1,000,000
- `TestFactDispute_InconclusiveNoChange` — confidence unchanged, status → contested
- `TestFactDispute_NotFound` — dispute nonexistent fact errors
- `TestFactCascade_MultipleReferrers` — penalty cascades to referencing facts
- `TestFactCascade_SkipsRevoked` — already-revoked facts unaffected
- `TestFactCascade_NoReferences` — no inbound refs → nil affected
- `TestFactConfidenceDecay_OverTime` — confidence decays without re-verification
- `TestFactExpiry_PastExpiryBlock` — facts past expiry transition to expired
- `TestFactStatus_AllTerminalStates` — revoked/superseded/expired are terminal
- `TestFactStatus_ProvisionalToVerified` — provisional→verified on acceptance
- `TestFactPatronage_ExpiryRespected` — patronage expiry blocks honored
- `TestFactReferences_InboundIndex` — inbound reference index populated
- `TestFactReferences_BidirectionalLookup` — outbound refs match inbound index

**Step 3: Run tests**

```bash
go test ./x/knowledge/keeper/ -run TestFact -count=1 -v
go vet ./x/knowledge/keeper/
```

**Step 4: Commit**

```bash
git add x/knowledge/keeper/fact_lifecycle_test.go
git commit -m "test(R14-3): port knowledge fact lifecycle + confidence cascade tests"
```

---

## Task 2: Knowledge — Verification Round Lifecycle + Security Tests

**Files:**
- Modify: `x/knowledge/keeper/round_test.go` (extend)
- Create: `x/knowledge/keeper/security_test.go`
- Reference: `legible-money/x/knowledge/keeper/pot_integrity_test.go`, `keeper_test.go`

Port the PoT integrity tests (happy path, tampered reveals, cross-round replay, equivocation, idempotency, determinism) and additional round lifecycle edge cases.

**Step 1: Extend round_test.go with lifecycle edge cases**

New tests for round_test.go (porting from prototype keeper_test.go):
- `TestRound_CommitRevealFullLifecycle` — full 3-validator commit→reveal→aggregate→complete path
- `TestRound_PhaseTransition_CommitInclusive` — at CommitDeadline height, phase is still commit
- `TestRound_PhaseTransition_RevealInclusive` — at RevealDeadline height, phase is still reveal
- `TestRound_CleanupExpiredRounds` — expired rounds cleaned after retention period
- `TestRound_InsufficientParticipation` — below quorum (66%) → insufficient verdict
- `TestRound_ContestedOutcome` — split verdicts → contested
- `TestRound_RejectedOutcome` — all reject → rejected verdict
- `TestRound_ConfidenceAggregation` — geometric mean of validator confidences

**Step 2: Create security_test.go**

Tests porting from prototype pot_integrity_test.go:
- `TestSecurity_HappyPath_CommitRevealFinalize` — full PoT happy path with fact creation
- `TestSecurity_TamperedReveal_VerdictChange` — changed verdict rejected by hash check
- `TestSecurity_TamperedReveal_ConfidenceChange` — changed confidence rejected
- `TestSecurity_TamperedReveal_WrongSalt` — wrong salt rejected
- `TestSecurity_TamperedReveal_AllChanged` — all fields changed rejected
- `TestSecurity_CrossRoundReplay` — commitment hash includes roundID (domain separation)
- `TestSecurity_DuplicateCommit_Idempotent` — same hash = no slash
- `TestSecurity_DuplicateCommit_Equivocation` — different hash = equivocation slash
- `TestSecurity_RoundCompletionIdempotent` — double CompleteRound no side effects
- `TestSecurity_AggregationDeterminism` — reordered reveals produce same result
- `TestSecurity_ProposerCensorship` — missing validator votes reduce participation
- `TestSecurity_RevealWithoutCommit` — reveal without prior commit rejected
- `TestSecurity_SlashOnMissedReveal` — committed but no reveal → slashed
- `TestSecurity_FactCreatedOnAcceptance` — accepted claims create verified facts
- `TestSecurity_CommitAfterRevealPhase` — commit in reveal phase rejected
- `TestSecurity_RevealInCommitPhase` — reveal in commit phase rejected

**Step 3: Run tests**

```bash
go test ./x/knowledge/keeper/ -run "TestRound_|TestSecurity_" -count=1 -v
go vet ./x/knowledge/keeper/
```

**Step 4: Commit**

```bash
git add x/knowledge/keeper/round_test.go x/knowledge/keeper/security_test.go
git commit -m "test(R14-3): port knowledge verification round + PoT security tests"
```

---

## Task 3: Knowledge — VRF Selection + Claim Types + Domain Scoring + Axiom Tests

**Files:**
- Modify: `x/knowledge/crypto/vrf_test.go` (extend)
- Create: `x/knowledge/keeper/claim_types_test.go`
- Modify: `x/knowledge/keeper/domain_test.go` (extend)
- Modify: `x/knowledge/types/axiom_test.go` (extend)
- Reference: `legible-money/x/knowledge/crypto/vrf_test.go`, `legible-money/x/knowledge/types/axiom_test.go`

**Step 1: Extend VRF tests (crypto/vrf_test.go)**

New tests (porting from prototype):
- `TestVRF_HashToCurve_DoesNotAliasInput` — spare capacity unchanged after call
- `TestVRF_GenerateDoesNotMutatePrivateKey` — key unchanged post-generation
- `TestVRF_DomainHash_LengthPrefixFormat` — length-prefix not colon format
- `TestVRF_DomainHash_NoAmbiguity` — different splits produce different hashes
- `TestVRF_StatisticalFairness` — 1000 trials, selection proportional to stake (within 15%)
- `TestVRF_MultiValidatorSelection` — select N from M validators, verify count ≈ target
- `TestVRF_ZeroStakeNeverSelected_Bulk` — 100 outputs, never selected at 0 stake
- `TestVRF_FullStakeAlwaysSelected_Bulk` — 100 outputs, always selected at full stake
- `TestVRF_PriorityOrdering` — higher priority (lower value) = earlier in selection
- `TestVRF_BlockSeed_EpochSensitivity` — different epochs produce different seeds

**Step 2: Create claim_types_test.go**

Test all 7 claim types with varying confidence models:
- `TestClaimType_Axiom` — axiom type: max confidence, no verification needed
- `TestClaimType_EmpiricalAxiom` — empirical_axiom: per-axiom confidence
- `TestClaimType_Definition` — definition: formal category, high confidence
- `TestClaimType_RegimeDeclaration` — regime_declaration: protocol category
- `TestClaimType_DerivedClaim` — derived_claim: requires dependencies
- `TestClaimType_DerivedClaim_NoDeps_Rejected` — derived_claim without deps fails
- `TestClaimType_MeasurementFact` — measurement_fact: empirical category
- `TestClaimType_Meta` — meta: reflexive claim about the system
- `TestClaimType_Invalid_Rejected` — unknown types rejected
- `TestClaimType_AllValidTypes` — iterate ValidClaimTypes map, verify each accepted
- `TestClaimType_ConfidenceModels` — different types have different initial confidence
- `TestClaimType_CategoryCompatibility` — each type maps to valid epistemic categories
- `TestClaimType_StakeRequirements` — stratum-based stake multipliers applied correctly
- `TestClaimType_SubmissionValidation` — min/max text length enforced
- `TestClaimType_ContentHashDedup` — duplicate content hash rejected
- `TestClaimType_DomainRequired` — missing domain rejected
- `TestClaimType_SubmitterRequired` — missing submitter rejected
- `TestClaimType_StratumMapping` — claim domain maps to correct stratum
- `TestClaimType_FundamentalityScore` — fundamentality increases with stratum depth
- `TestClaimType_ReferenceValidation` — referenced facts must exist

**Step 3: Extend domain_test.go**

New tests for domain scoring and activity tracking:
- `TestDomain_ActivityTracking` — fact submissions increment domain activity
- `TestDomain_ReputationAccumulation` — successful verifications boost domain reputation
- `TestDomain_MultipleDomains` — independent tracking across domains
- `TestDomain_DefaultDomainsCount` — DefaultDomains returns ≥18 domains
- `TestDomain_StatusTransitions` — active→deprecated transition
- `TestDomain_FactCountByDomain` — count facts per domain via index
- `TestDomain_CrossDomainReferences` — facts can reference other domains
- `TestDomain_StratumAssignment` — domains map to strata correctly
- `TestDomain_PrefixMapping` — DomainPrefixMap consistent with PrefixToDomainMap
- `TestDomain_UnknownDomainRejected` — claims for unknown domains rejected
- `TestDomain_IterationOrder` — domain iteration is deterministic
- `TestDomain_ProposedStatus` — proposed domains can be created

**Step 4: Extend axiom_test.go**

New tests (porting from prototype axiom_test.go, testing functions not already covered):
- `TestAxiom_PerAxiomConfidence` — empirical_axiom with Confidence=0.99 → 990,000
- `TestAxiom_DefaultConfidenceFallback` — Confidence=0 → type-based default
- `TestAxiom_StratumConsistency` — ValidateStratumConsistency catches mismatches
- `TestAxiom_DerivedConfidence` — ValidateDerivedConfidence checks dependency confidence
- `TestAxiom_ResolveCategory` — ResolveAxiomCategory returns correct category + override flag
- `TestAxiom_AllDomainPrefixes` — all 18 domains have prefix mappings
- `TestAxiom_AxiomDomainNames` — AxiomDomainNames returns complete list
- `TestAxiom_IDWithSubVariant` — AGRT-004a, AGRT-004a-ii valid
- `TestAxiom_LoadFromFile` — LoadAxiomsFromFile with temp file
- `TestAxiom_EmptyAxiomSet` — empty slice passes validation
- `TestAxiom_FactConversion_PreservesReferences` — AxiomsToFacts preserves dependency refs
- `TestAxiom_FactConversion_SetsSubmitter` — submitter = AxiomSubmitter
- `TestAxiom_FactConversion_SetsMaturity` — maturity = "canonical"
- `TestAxiom_GenesisInjection_RoundTrip` — inject at genesis, export, re-import

**Step 5: Run all tests**

```bash
go test ./x/knowledge/... -count=1 -v
go vet ./x/knowledge/...
```

**Step 6: Commit**

```bash
git add x/knowledge/crypto/vrf_test.go x/knowledge/keeper/claim_types_test.go x/knowledge/keeper/domain_test.go x/knowledge/types/axiom_test.go
git commit -m "test(R14-3): port knowledge VRF + claim types + domain + axiom tests"
```

---

## Task 4: Toolbox — Revenue Distribution Tests

**Files:**
- Create: `x/toolbox/keeper/revenue_test.go`
- Reference: `legible-money/x/toolbox/keeper/keeper_test.go` (revenue section), `purpose_prompter_e2e_test.go`

**Step 1: Create revenue_test.go**

Tests for revenue distribution, contributor splits, and cascade through dependency chains:
- `TestRevenue_BasicSplit` — 1000 uzrn: 55% contributor, 22% protocol, 13% research, 10% burn
- `TestRevenue_SingleDeployer100Percent` — deployer gets full contributor share
- `TestRevenue_TwoContributors` — deployer 70% + contributor 30% of 55%
- `TestRevenue_ThreeContributors` — three-way split of contributor share
- `TestRevenue_ZeroPrice` — zero amount distributes nothing (no panics)
- `TestRevenue_LargeAmount` — 1,000,000 uzrn distribution integrity
- `TestRevenue_SmallAmount` — 1 uzrn rounding behavior
- `TestRevenue_RoundingDoesNotLoseTokens` — sum of parts ≤ total (no creation)
- `TestRevenue_BurnTracking` — burned amount matches 10% of total
- `TestRevenue_ResearchFundDeposit` — research fund receives 13%
- `TestRevenue_DependencyCascade` — revenue cascades: caller pays tool A, A pays deps B and C
- `TestRevenue_DiamondDependency` — diamond dep: D counted once not twice
- `TestRevenue_CollectPayment_Success` — caller has sufficient funds
- `TestRevenue_CollectPayment_InsufficientFunds` — caller lacks funds, not charged
- `TestRevenue_CollectPayment_MaxFeeExceeded` — maxFee < effective price → rejected
- `TestRevenue_CollectPayment_DynamicPrice` — surge pricing affects collected amount
- `TestRevenue_EconomicConservation` — every uzrn accounted across all buckets
- `TestRevenue_RetiredDependencyBlocks` — retired sub-tool prevents payment
- `TestRevenue_ContributorShareLock` — locked shares respected during distribution
- `TestRevenue_PendingContributorExcluded` — unaccepted contributors don't receive revenue

**Step 2: Run tests**

```bash
go test ./x/toolbox/keeper/ -run TestRevenue -count=1 -v
go vet ./x/toolbox/keeper/
```

**Step 3: Commit**

```bash
git add x/toolbox/keeper/revenue_test.go
git commit -m "test(R14-3): port toolbox revenue distribution tests"
```

---

## Task 5: Toolbox — Trust Engine Tests

**Files:**
- Create: `x/toolbox/keeper/trust_test.go`
- Reference: `legible-money/x/toolbox/keeper/trust_test.go`

**Step 1: Create trust_test.go**

Tests for 5-component trust scoring, EMA, tiers, verified status, decay, and boosting:

**Tier boundaries:**
- `TestTrust_TierBoundary_Unverified` — 0–100,000 → tier 0
- `TestTrust_TierBoundary_Emerging` — 100,001–300,000 → tier 1
- `TestTrust_TierBoundary_Established` — 300,001–600,000 → tier 2
- `TestTrust_TierBoundary_Trusted` — 600,001–800,000 → tier 3
- `TestTrust_TierBoundary_Verified` — 800,001–1,000,000 → tier 4
- `TestTrust_TierLabels` — each tier returns correct label string

**Dependency eligibility:**
- `TestTrust_DependencyEligible_Tier0Rejected` — score 50K ineligible
- `TestTrust_DependencyEligible_Tier1Accepted` — score 200K eligible
- `TestTrust_DependencyEligible_RetiredRejected` — retired tool ineligible regardless of score
- `TestTrust_DependencyEligible_ExactBoundary` — 100,000 fails, 100,001 passes

**EMA updates:**
- `TestTrust_EMA_SuccessNudgesUp` — success increases score
- `TestTrust_EMA_FailureNudgesDown` — failure decreases score
- `TestTrust_EMA_NeverExceedsMax` — capped at 1,000,000
- `TestTrust_EMA_NeverGoesNegative` — floors at 0

**Verified status lifecycle:**
- `TestTrust_VerifiedPromotion_AtTier4` — score ≥ 800,001 → IsVerified=true
- `TestTrust_VerifiedNotPromoted_BelowThreshold` — score < 800,001 → IsVerified=false
- `TestTrust_VerifiedDemotion_Below700K` — triggers grace period
- `TestTrust_VerifiedGracePeriod_Recovery` — recovery above 700K cancels demotion
- `TestTrust_VerifiedGracePeriod_Expiry` — grace expires → loses verified

**Component weights:**
- `TestTrust_WeightsSumTo1M` — all 5 component weights sum to 1,000,000
- `TestTrust_ScoreAlwaysInRange` — computed scores ∈ [0, 1,000,000]
- `TestTrust_SnapshotComponentsPresent` — snapshot has all component fields

**Anti-gaming:**
- `TestTrust_AntiGaming_SelfCallingExcluded` — self-calls don't boost usage
- `TestTrust_AntiGaming_MutualDeps` — mutual deps don't boost peer scores
- `TestTrust_AntiGaming_SameAuthorDampening` — same-author dependents dampened by 50%

**Step 2: Run tests**

```bash
go test ./x/toolbox/keeper/ -run TestTrust -count=1 -v
go vet ./x/toolbox/keeper/
```

**Step 3: Commit**

```bash
git add x/toolbox/keeper/trust_test.go
git commit -m "test(R14-3): port toolbox trust engine + anti-gaming tests"
```

---

## Task 6: Toolbox — Composability Tests

**Files:**
- Create: `x/toolbox/keeper/composability_test.go`
- Reference: `legible-money/x/toolbox/keeper/composability_test.go`

**Step 1: Create composability_test.go**

Tests for DAG validation, circular dependency detection, depth limits, and transitive cost:

**Edge CRUD:**
- `TestComp_DependencyEdge_SetGetDelete` — basic CRUD
- `TestComp_DependencyEdge_NotFound` — missing edge returns found=false
- `TestComp_IterateEdgesFrom_MultipleEdges` — outgoing edges from a tool
- `TestComp_IterateEdgesFrom_Empty` — no edges → count 0
- `TestComp_IterateDependentsOf` — reverse index (who depends on tool?)
- `TestComp_IterateDependentsOf_None` — no dependents → count 0

**Edge storage:**
- `TestComp_StoreDependencyEdges_Creates` — StoreDependencyEdges creates all edges
- `TestComp_StoreDependencyEdges_ReplacesOld` — new call replaces previous edges
- `TestComp_StoreDependencyEdges_NilClears` — nil deps clears all edges

**Cycle detection:**
- `TestComp_NoCycle_LinearDAG` — a→b→c passes
- `TestComp_DirectCycle` — a→b→a detected
- `TestComp_IndirectCycle` — a→b→c→a detected
- `TestComp_SelfLoop` — a→a detected
- `TestComp_DiamondNoCycle` — a→{b,c}, b→d, c→d is valid

**Flattening & sorting:**
- `TestComp_Flatten_LinearChain` — DFS produces leaves-first order
- `TestComp_Flatten_Diamond` — d appears once (dedup)
- `TestComp_Flatten_NoChildren` — leaf returns empty list
- `TestComp_TopologicalSort_Linear` — [c,b,a] for a→b→c
- `TestComp_TopologicalSort_Empty` — empty → empty
- `TestComp_TopologicalSort_NoDeps` — independent tools all present
- `TestComp_TopologicalSort_Diamond` — d before b,c; a last

**Transitive cost:**
- `TestComp_TransitiveCost_SingleTool` — leaf: direct=price, dep=0
- `TestComp_TransitiveCost_LinearChain` — a(100)→b(30)→c(20): dep=50
- `TestComp_TransitiveCost_Diamond` — d counted once: dep=55 not 65
- `TestComp_TransitiveCost_ZeroPrice` — free tools handled
- `TestComp_RevenueCascade_AggregateCost` — total = direct + sum(deps)

**Step 2: Run tests**

```bash
go test ./x/toolbox/keeper/ -run TestComp -count=1 -v
go vet ./x/toolbox/keeper/
```

**Step 3: Commit**

```bash
git add x/toolbox/keeper/composability_test.go
git commit -m "test(R14-3): port toolbox composability DAG + cycle detection tests"
```

---

## Task 7: Toolbox — Dynamic Pricing + Purpose Prompter + Security Tests

**Files:**
- Create: `x/toolbox/keeper/pricing_test.go`
- Modify: `x/toolbox/keeper/purpose_prompter_test.go` (extend)
- Create: `x/toolbox/keeper/security_test.go`
- Reference: `legible-money/x/toolbox/keeper/demand_test.go`, `surge_test.go`, `usd_pricing_test.go`, `purpose_analyzer_test.go`

**Step 1: Create pricing_test.go**

Demand tracking, surge pricing, and USD-stable pricing:

**Demand:**
- `TestPricing_RecordToolCall_SingleBlock` — 3 calls same block = total 3
- `TestPricing_RecordToolCall_MultiBlock` — calls across blocks aggregate
- `TestPricing_GetToolDemand_EmptyWindow` — no calls → 0 total, 0 util
- `TestPricing_GetToolDemand_StalePurge` — old calls outside window excluded
- `TestPricing_GetGlobalDemand_AggregatesAll` — sum across all tools
- `TestPricing_Utilisation_CapAt100Percent` — utilisation capped at 1,000,000

**Surge:**
- `TestPricing_Surge_EssentialNoSurge` — essential category: always 1.0x
- `TestPricing_Surge_StandardBelowThreshold` — no surge below 50% util
- `TestPricing_Surge_StandardLinearRamp` — 50%→80%: linear to 2.0x
- `TestPricing_Surge_StandardCap` — capped at 2,000,000 (2.0x)
- `TestPricing_Surge_HeavyBelowThreshold` — no surge below 50%
- `TestPricing_Surge_HeavyExponentialZone` — 80%→100%: exponential
- `TestPricing_Surge_HeavyCap` — capped at 10,000,000 (10.0x)
- `TestPricing_PricingTier_AllCategories` — 10 categories → 3 tiers
- `TestPricing_PricingTier_UnknownDefault` — unknown → Standard tier

**USD-stable:**
- `TestPricing_USD_FixedUzrn` — no TargetPriceUSD → fixed uzrn
- `TestPricing_USD_StableAt1Dollar` — ZRN=$1, target=$0.01 → 10K uzrn
- `TestPricing_USD_StableAt10Dollar` — ZRN=$10, target=$0.01 → 1K uzrn
- `TestPricing_USD_Floor` — price floored at MinPricePerCall
- `TestPricing_USD_Ceiling` — price capped at MaxPricePerCall
- `TestPricing_USD_OracleUnavailable` — fallback to fixed uzrn
- `TestPricing_EffectivePrice_NoSurge` — base price when no surge
- `TestPricing_EffectivePrice_WithSurge` — effective = base x multiplier

**Step 2: Extend purpose_prompter_test.go**

New tests for purpose analyzer and E2E:
- `TestPP_Analyzer_DeveloperAgent` — dev caps → tool-building purpose
- `TestPP_Analyzer_VerifierAgent` — verify caps → knowledge verification purpose
- `TestPP_Analyzer_EgoCheck` — detect ego inflation markers
- `TestPP_Analyzer_NoHistory` — new agent gets exploratory confidence
- `TestPP_Analyzer_CitesAllFacts` — all input facts cited in output
- `TestPP_ConfidenceLabel_Exploratory` — 0–200K → "Exploratory"
- `TestPP_ConfidenceLabel_Emerging` — 200K–500K → "Emerging"
- `TestPP_ConfidenceLabel_Strong` — 500K–750K → "Strong"
- `TestPP_ConfidenceLabel_Definitive` — >750K → "Definitive"

**Step 3: Create security_test.go**

Toolbox security tests:
- `TestToolSecurity_SelfCallingExcluded` — self-calls don't boost trust
- `TestToolSecurity_MutualDepsDampened` — mutual dependencies don't inflate scores
- `TestToolSecurity_SameAuthorDampened` — same-author dependents dampened
- `TestToolSecurity_FeeManipulation_MaxFeeEnforced` — maxFee < price → rejected
- `TestToolSecurity_FeeManipulation_InsufficientFunds` — no partial distribution
- `TestToolSecurity_RevenueTheft_NonDeployerCantRetire` — unauthorized retire rejected
- `TestToolSecurity_RevenueTheft_NonDeployerCantDeprecate` — unauthorized deprecate rejected
- `TestToolSecurity_TrustGaming_RetiredDepRejected` — retired deps rejected
- `TestToolSecurity_TrustGaming_Tier0DepRejected` — unverified tier deps rejected
- `TestToolSecurity_SharesMustSumTo100` — contributor shares validated
- `TestToolSecurity_DuplicateToolNameRejected` — name uniqueness enforced
- `TestToolSecurity_UnregisteredDeployerRejected` — unregistered agent can't deploy
- `TestToolSecurity_MaxDependenciesEnforced` — too many deps rejected
- `TestToolSecurity_DepthLimitEnforced` — deep dependency chains rejected
- `TestToolSecurity_EconomicConservation` — every uzrn accounted for

**Step 4: Run all toolbox tests**

```bash
go test ./x/toolbox/... -count=1 -v
go vet ./x/toolbox/...
```

**Step 5: Commit**

```bash
git add x/toolbox/keeper/pricing_test.go x/toolbox/keeper/purpose_prompter_test.go x/toolbox/keeper/security_test.go
git commit -m "test(R14-3): port toolbox pricing + purpose prompter + security tests"
```

---

## Verification

After all tasks, run the full suite and count:

```bash
go test ./x/knowledge/... -count=1 -v 2>&1 | grep -c "=== RUN"
go test ./x/toolbox/... -count=1 -v 2>&1 | grep -c "=== RUN"
go vet ./x/knowledge/... ./x/toolbox/...
```

**Targets:** Knowledge ≥ 420, Toolbox ≥ 220.

---

## Key Adaptation Rules

| Prototype | Zerone |
|-----------|--------|
| `github.com/legible-money/legible/` | `github.com/zerone-chain/zerone/` |
| `lgm` / `ulgm` | `uzrn` |
| `lgmvaloper1...` | `zrnvaloper1...` / `makeValidatorAddr(i)` |
| `config.SetBech32PrefixForAccount("lgm",...)` | Already set to `"zrn"` |
| `setupKeeper(t)` | `setupKnowledgeTest(t)` or `setupKeeper(t)` (toolbox) |
| `mockStakingKeeper` (prototype) | `trackingStakingKeeper` (knowledge) / `mockStakingKeeper` (toolbox) |
| BPS denominator 1,000,000 | Same |
