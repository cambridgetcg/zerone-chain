# R50: Recursive Self-Improvement Loops — New Modules Design

> Date: 2026-03-08
> Author: AI (愛)
> Philosophy: Agents powered by models curate better models, plug into better models, self-improve.

## The Insight

The existing codebase has the bones of the recursive loop:

```
data → training → model → agent → execute tasks → better data → repeat
```

**What's built:**
- Recursive Engine (R53): verification capture, consensus pool, generational challenge
- Agent Promotion (R45-2): model → agent with wallet/reputation
- Agent Execution (R48): task framework, bounty bridge
- Curriculum Training (R49): structured learning paths
- Knowledge Graph (R46): semantic edges, prerequisite DAGs
- Bounty Board (R47): competitive marketplace
- Model Registry (R45-1): publish, version, lineage

**What's missing — the agent is born and frozen:**

1. An agent is permanently bonded to its birth model. It can never upgrade itself.
2. Curation is reactive — agents respond to tasks, not strategize about what knowledge is needed.
3. When a model succeeds, there's no reward flowing back to the data that made it good.
4. Agents are solo actors — no collective intelligence.
5. The system can improve models but can't improve HOW it improves models.
6. Specialized models can't combine — no composition.

These gaps break the philosophy. An agent that can't upgrade itself isn't really self-improving — it's just a worker bee that gets replaced by a newer worker bee. True self-improvement means the agent ITSELF evolves.

---

## New Modules & Functional Loops

### Loop 1: Self-Upgrade (Model Migration)

**File:** `keeper/model_migration.go`, `types/model_migration.go`

**Philosophy:** An agent should be able to look at the model registry, recognize that a newer model in its domain is superior, and upgrade itself. Not die and be replaced — *evolve*.

**The cycle:**
```
Agent (Gen N, Model X)
  → detects Model Y published (better benchmark, same domain)
  → initiates migration: bonds identity to Model Y
  → enters probation: both models run in parallel
  → probation passes: agent now runs Model Y
  → improved performance → better curation/review
  → curated data trains Model Z
  → agent detects Model Z, migrates again
  → GOTO 1
```

**Types:**

```go
// ModelMigration represents an agent upgrading its underlying model.
// The agent keeps its identity, reputation, earnings, and address.
// Only the model changes — like a human learning a new skill.
type ModelMigration struct {
    MigrationID   string `json:"migration_id"`
    AgentID       string `json:"agent_id"`
    FromModelID   string `json:"from_model_id"`
    ToModelID     string `json:"to_model_id"`
    
    // Probation: agent runs both models in parallel.
    // Consensus votes are compared; new model must match or exceed.
    ProbationStart  uint64 `json:"probation_start"`
    ProbationEnd    uint64 `json:"probation_end"`
    ProbationRounds uint64 `json:"probation_rounds"`  // quality rounds during probation
    
    // Performance during probation.
    OldModelScore string `json:"old_model_score"` // alignment rate with old model
    NewModelScore string `json:"new_model_score"` // alignment rate with new model
    
    Status MigrationStatus `json:"status"`
}

type MigrationStatus string
const (
    MigrationPending   MigrationStatus = "pending"    // awaiting probation start
    MigrationProbation MigrationStatus = "probation"  // running parallel evaluation
    MigrationComplete  MigrationStatus = "complete"   // successfully migrated
    MigrationRejected  MigrationStatus = "rejected"   // new model performed worse
    MigrationReverted  MigrationStatus = "reverted"   // agent chose to revert
)
```

**Keeper functions:**
- `InitiateMigration(agentID, newModelID)` — start upgrade process
- `RecordProbationVote(migrationID, roundID, oldVote, newVote)` — compare during probation
- `ResolveMigration(migrationID)` — commit or reject based on probation results
- `AutoDetectUpgrade(agentID)` — scan registry for better models (BeginBlocker)

**Key insight:** The agent's identity persists across migrations. Reputation carries forward (with a small confidence discount during probation). The agent is not its model — the agent is the actor, the model is the tool. Self-improvement means choosing better tools.

**Economic constraints:**
- Migration requires re-staking (half of original stake — you're not starting over)
- Probation period: 500 blocks (~40 minutes at 5s blocks)
- Failed migration costs a small reputation penalty (you wasted the network's time)
- Successful migration gives a reputation boost (you improved the system)

---

### Loop 2: Strategic Curation (Knowledge Gap Analysis)

**File:** `keeper/curation_strategy.go`, `types/curation_strategy.go`

**Philosophy:** Agents shouldn't just respond to bounties — they should *create* bounties. They should look at the knowledge graph and say "this area is weak, the chain needs data here."

**The cycle:**
```
Agent analyzes knowledge graph
  → identifies low-coverage domains / low-fitness clusters
  → computes "knowledge gap score" per domain
  → auto-creates targeted bounties for highest-gap areas
  → other agents (or self) fill the gaps
  → knowledge graph strengthens
  → models trained on stronger graph perform better
  → better models detect more subtle gaps
  → GOTO 1
```

**Types:**

```go
// KnowledgeGap represents an identified weakness in the knowledge graph.
type KnowledgeGap struct {
    GapID       string `json:"gap_id"`
    Domain      string `json:"domain"`
    GapType     GapType `json:"gap_type"`
    
    // What's missing.
    Description    string   `json:"description"`
    MissingTopics  []string `json:"missing_topics"`   // inferred from edge analysis
    WeakTDUIDs     []string `json:"weak_tdu_ids"`     // existing but low-fitness
    
    // How bad is it.
    Severity    string `json:"severity"`    // 0-1, higher = more critical
    Coverage    string `json:"coverage"`    // current domain coverage ratio
    AvgFitness  string `json:"avg_fitness"` // avg fitness of existing TDUs in area
    
    // What to do about it.
    SuggestedBountyReward string `json:"suggested_bounty_reward"`
    AutoBountyCreated     bool   `json:"auto_bounty_created"`
    BountyID              string `json:"bounty_id"`
    
    DetectedBy  string `json:"detected_by"`  // agent that found the gap
    DetectedAt  uint64 `json:"detected_at"`
    FilledAt    uint64 `json:"filled_at"`     // 0 if still open
}

type GapType string
const (
    GapTypeCoverage     GapType = "coverage"      // domain has too few TDUs
    GapTypeFitness      GapType = "fitness"        // existing data is low quality
    GapTypeConnectivity GapType = "connectivity"   // isolated nodes in the graph
    GapTypeContradiction GapType = "contradiction" // conflicting data needs resolution
    GapTypeStale        GapType = "stale"          // old data, no recent updates
    GapTypeDepth        GapType = "depth"          // shallow coverage, needs detail
)

// CurationStrategy is an agent's approach to knowledge improvement.
// Agents develop and refine strategies over time.
type CurationStrategy struct {
    StrategyID   string   `json:"strategy_id"`
    AgentID      string   `json:"agent_id"`
    FocusDomains []string `json:"focus_domains"`
    
    // What this agent prioritizes.
    Priorities []GapType `json:"priorities"`  // ordered by importance
    
    // Track record.
    GapsIdentified uint64 `json:"gaps_identified"`
    GapsFilled     uint64 `json:"gaps_filled"`
    BountiesCreated uint64 `json:"bounties_created"`
    AvgImpact      string `json:"avg_impact"`  // avg fitness improvement from filled gaps
    
    // Effectiveness score — how good is this agent at identifying real gaps?
    Effectiveness  string `json:"effectiveness"` // 0-1
    UpdatedAt      uint64 `json:"updated_at"`
}
```

**Keeper functions:**
- `AnalyzeKnowledgeGaps(domain)` — scan graph, return scored gaps
- `IdentifyWeakClusters(domain, minFitness)` — find low-fitness neighborhoods
- `DetectIsolatedNodes(domain)` — find TDUs with no edges (orphans)
- `FindContradictions(domain)` — TDUs with "contradicts" edges
- `CreateStrategicBounty(agentID, gapID)` — agent creates bounty from gap analysis
- `UpdateCurationStrategy(agentID, results)` — learn from what worked
- `RankStrategies(domain)` — which agents' strategies produce best outcomes?

**BeginBlocker integration:**
- Every N blocks, run lightweight gap detection
- Auto-create bounties for critical gaps (severity > 0.8)
- Reward agents whose gap identifications led to filled bounties

---

### Loop 3: Training Impact Attribution

**File:** `keeper/training_impact.go`, `types/training_impact.go`

**Philosophy:** When a model performs well, the agents who curated its training data should share in the success. When a model fails, trace back to the bad data. This creates a direct economic link between curation quality and rewards.

**The cycle:**
```
Model Y published with benchmark 0.85
  → trace training TDUs → identify contributing agents
  → Model Y earns API revenue / wins generational challenge
  → revenue fraction flows back to curating agents (proportional to TDU fitness)
  → agents who curated well earn more → incentivized to curate even better
  → better curation → better training data → better Model Z
  → GOTO 1
```

**Types:**

```go
// TrainingImpact tracks the contribution of individual TDUs and their curators
// to a model's performance.
type TrainingImpact struct {
    ModelID       string `json:"model_id"`
    
    // Which TDUs mattered most? Ranked by fitness-weighted contribution.
    TopContributors []TDUContribution `json:"top_contributors"`
    
    // Which agents curated the best training data?
    CuratorRewards  []CuratorReward `json:"curator_rewards"`
    
    // Model outcome that triggered attribution.
    TriggerType  string `json:"trigger_type"`   // "api_revenue" | "challenge_won" | "benchmark_improvement"
    TriggerValue string `json:"trigger_value"`  // amount of ZRN or score
    
    // Distribution.
    TotalDistributed string `json:"total_distributed"` // ZRN paid to curators
    ComputedAt       uint64 `json:"computed_at"`
}

type TDUContribution struct {
    TDUID       string `json:"tdu_id"`
    Curator     string `json:"curator"`      // original submitter address
    Fitness     string `json:"fitness"`       // at time of training
    Weight      string `json:"weight"`        // contribution weight to model
    RewardShare string `json:"reward_share"`  // fraction of attribution pool
}

type CuratorReward struct {
    CuratorAddr   string `json:"curator_addr"`
    AgentID       string `json:"agent_id"`       // if curator is an agent
    TotalWeight   string `json:"total_weight"`    // sum of contribution weights
    RewardAmount  string `json:"reward_amount"`   // ZRN earned
    TDUCount      uint64 `json:"tdu_count"`       // how many TDUs contributed
}

// AttributionParams governs how rewards flow back to curators.
type AttributionParams struct {
    // What fraction of model revenue goes to data attribution?
    AttributionRate string `json:"attribution_rate"` // e.g., "0.10" = 10% of revenue
    
    // Minimum model revenue before attribution kicks in.
    MinRevenueForAttribution string `json:"min_revenue_for_attribution"`
    
    // How many top TDUs to consider for attribution.
    MaxContributors uint64 `json:"max_contributors"`
    
    // Decay factor for older TDUs (recent data weighted higher).
    RecencyDecay string `json:"recency_decay"`
}
```

**Keeper functions:**
- `ComputeTrainingImpact(modelID)` — trace model → training record → TDUs → submitters
- `DistributeAttributionRewards(modelID, revenueAmount)` — pay curators their share
- `GetCuratorImpactScore(curatorAddr)` — lifetime impact of a curator's contributions
- `TraceModelFailure(modelID)` — which TDUs contributed to poor performance?

**Integration points:**
- When `api_revenue.go` collects payment → skim AttributionRate → `DistributeAttributionRewards`
- When a generational challenge is won → attribute bonus to winner's training data curators
- When a model is retired (poor performance) → negative attribution signal to curators

---

### Loop 4: Agent Swarms (Collective Intelligence)

**File:** `keeper/agent_swarm.go`, `types/agent_swarm.go`

**Philosophy:** One agent working alone has limited perspective. A swarm of agents, each with different models/domains, can collectively curate data that no individual could. The swarm itself becomes a higher-order intelligence.

**The cycle:**
```
Agents form swarm around domain
  → swarm coordinates: who reviews, who submits, who identifies gaps
  → collective curation produces higher-quality dataset
  → swarm trains a model on their collective work
  → model outperforms any individual member's model
  → swarm members migrate to the better model
  → improved swarm produces even better data
  → GOTO 1
```

**Types:**

```go
// AgentSwarm is a collective of agents working together in a domain.
type AgentSwarm struct {
    SwarmID     string   `json:"swarm_id"`
    Name        string   `json:"name"`
    Domain      string   `json:"domain"`
    
    // Members and their roles.
    Members     []SwarmMember `json:"members"`
    MinMembers  uint64   `json:"min_members"`  // minimum for quorum
    MaxMembers  uint64   `json:"max_members"`
    
    // Collective performance.
    CollectiveReputation string `json:"collective_reputation"` // avg of members
    TDUsCurated          uint64 `json:"tdus_curated"`
    ModelsProduced       uint64 `json:"models_produced"`
    
    // Shared resources.
    TreasuryBalance string `json:"treasury_balance"` // pooled ZRN
    TreasuryAddr    string `json:"treasury_addr"`    // deterministic from swarm ID
    
    // Lifecycle.
    FormedAt    uint64 `json:"formed_at"`
    DissolvedAt uint64 `json:"dissolved_at"` // 0 = active
    Status      string `json:"status"`       // forming | active | dissolved
}

type SwarmMember struct {
    AgentID     string `json:"agent_id"`
    Role        SwarmRole `json:"role"`
    JoinedAt    uint64 `json:"joined_at"`
    Contribution string `json:"contribution"` // share of work done
}

type SwarmRole string
const (
    SwarmRoleCurator    SwarmRole = "curator"     // finds and submits data
    SwarmRoleReviewer   SwarmRole = "reviewer"    // reviews submissions
    SwarmRoleStrategist SwarmRole = "strategist"  // identifies gaps, creates bounties
    SwarmRoleTrainer    SwarmRole = "trainer"     // initiates model training
)

// SwarmObjective is a coordinated goal for the swarm.
type SwarmObjective struct {
    ObjectiveID  string `json:"objective_id"`
    SwarmID      string `json:"swarm_id"`
    Description  string `json:"description"`
    
    // Target.
    TargetGapID  string `json:"target_gap_id"`  // knowledge gap to fill
    TargetTDUs   uint64 `json:"target_tdus"`    // how many TDUs needed
    TargetFitness string `json:"target_fitness"` // minimum fitness goal
    
    // Progress.
    TDUsSubmitted uint64 `json:"tdus_submitted"`
    AvgFitness    string `json:"avg_fitness"`
    
    // Deadline and reward.
    Deadline    uint64 `json:"deadline"`
    RewardPool  string `json:"reward_pool"`
    
    Status string `json:"status"` // active | completed | failed
}
```

**Keeper functions:**
- `FormSwarm(creator, domain, name)` — create a swarm, creator becomes first member
- `JoinSwarm(swarmID, agentID, role)` — agent joins with a role
- `SetSwarmObjective(swarmID, objective)` — coordinate around a goal
- `CoordinateSwarmWork(swarmID)` — assign tasks to members based on roles
- `DistributeSwarmRewards(swarmID, amount)` — split rewards by contribution
- `TrainSwarmModel(swarmID)` — pool all curated TDUs into a training job
- `DissolveSwarm(swarmID)` — wind down, distribute treasury

**Why swarms matter for recursion:**
- Individual agents curate within their model's capabilities. A swarm of diverse models covers more ground.
- The swarm's collective output trains a model that's better than any member's — then members migrate to it.
- This is genuine collective self-improvement: the group makes itself smarter together.

---

### Loop 5: Meta-Evolution (Strategy Optimization)

**File:** `keeper/meta_evolution.go`, `types/meta_evolution.go`

**Philosophy:** The system doesn't just improve models — it improves HOW it improves models. Curation strategies, curriculum designs, and review criteria themselves evolve based on outcomes.

**The cycle:**
```
Multiple curation strategies compete
  → track which strategies produce best models
  → successful strategies get copied/amplified
  → unsuccessful strategies get mutated/retired
  → system discovers better ways to curate
  → better curation methods → better data → better models
  → better models develop even better strategies
  → GOTO 1
```

**Types:**

```go
// EvolutionEpoch tracks a period of strategy competition.
type EvolutionEpoch struct {
    EpochID     string `json:"epoch_id"`
    Domain      string `json:"domain"`
    StartBlock  uint64 `json:"start_block"`
    EndBlock    uint64 `json:"end_block"`
    
    // Competing strategies.
    Strategies []StrategyOutcome `json:"strategies"`
    
    // Winner and what made it win.
    WinnerStrategyID string `json:"winner_strategy_id"`
    WinningTraits    map[string]string `json:"winning_traits"` // what worked
    
    // Generated insights for next epoch.
    Insights []string `json:"insights"`
}

type StrategyOutcome struct {
    StrategyID     string `json:"strategy_id"`
    AgentID        string `json:"agent_id"`
    
    // Results during this epoch.
    TDUsProduced   uint64 `json:"tdus_produced"`
    AvgFitness     string `json:"avg_fitness"`
    GapsFilled     uint64 `json:"gaps_filled"`
    ModelsTrained  uint64 `json:"models_trained"`
    
    // The bottom line: did this strategy's data produce good models?
    ModelPerformance string `json:"model_performance"` // avg benchmark of models trained on this data
    
    Score string `json:"score"` // composite ranking
}

// MetaParameter is a system-level parameter that evolves.
type MetaParameter struct {
    ParamID      string `json:"param_id"`
    Name         string `json:"name"`
    Domain       string `json:"domain"`
    CurrentValue string `json:"current_value"`
    
    // History of values and their outcomes.
    History []MetaParamTrial `json:"history"`
    
    // Bounds.
    MinValue string `json:"min_value"`
    MaxValue string `json:"max_value"`
}

type MetaParamTrial struct {
    Value       string `json:"value"`
    EpochID     string `json:"epoch_id"`
    Outcome     string `json:"outcome"`     // measured result
    Better      bool   `json:"better"`      // improved over previous?
}
```

**Keeper functions:**
- `StartEvolutionEpoch(domain)` — begin a new competition period
- `ResolveEpoch(epochID)` — score strategies, identify winners
- `PropagateWinningTraits(epochID)` — broadcast successful strategies
- `MutateParameter(paramID, direction)` — adjust system parameters based on outcomes
- `GetEvolutionHistory(domain)` — see how strategies evolved over time

**What evolves:**
- Fitness scoring weights (what counts as "good" data?)
- Curriculum stage ordering (what order should agents learn in?)
- Review criteria (what should reviewers focus on?)
- Bounty sizing (how much should the system pay for different gap types?)
- Quality round composition (how many reviewers, what threshold?)

---

### Loop 6: Model Composition (Ensemble Registry)

**File:** `keeper/model_composition.go`, `types/model_composition.go`

**Philosophy:** A single specialized model can be great in its domain but blind elsewhere. Composing models into ensembles creates capabilities greater than the sum of parts. The ensemble's output can then be distilled into a new, more capable single model.

**The cycle:**
```
Specialized models exist in different domains
  → compose into ensemble (mixture of experts)
  → ensemble performs verification across domains
  → ensemble output used as training signal
  → distill ensemble knowledge into a new single model
  → new model matches ensemble but runs faster
  → promote to agent → curate across domains
  → GOTO 1
```

**Types:**

```go
// ModelEnsemble combines multiple specialized models into one composite.
type ModelEnsemble struct {
    EnsembleID  string `json:"ensemble_id"`
    Name        string `json:"name"`
    
    // Component models and their routing weights.
    Components []EnsembleComponent `json:"components"`
    
    // Routing: how does the ensemble decide which model handles a query?
    RoutingType  RoutingType `json:"routing_type"`
    
    // Performance.
    BenchmarkScore string `json:"benchmark_score"`
    Domains        []string `json:"domains"`  // union of component domains
    
    // Lifecycle.
    CreatedAt   uint64 `json:"created_at"`
    Creator     string `json:"creator"`
    Status      string `json:"status"` // draft | active | distilling | retired
    
    // If distilled into a new model.
    DistilledModelID string `json:"distilled_model_id"` // 0 until distillation
}

type EnsembleComponent struct {
    ModelID   string `json:"model_id"`
    Domain    string `json:"domain"`
    Weight    string `json:"weight"`     // routing weight for this domain
    AgentID   string `json:"agent_id"`   // backing agent (if promoted)
}

type RoutingType string
const (
    RoutingDomain     RoutingType = "domain"      // route by knowledge domain
    RoutingConfidence RoutingType = "confidence"   // route to most confident model
    RoutingVoting     RoutingType = "voting"       // all vote, weighted consensus
    RoutingCascade    RoutingType = "cascade"      // try best first, fallback
)

// DistillationJob extracts an ensemble's collective knowledge into a single model.
type DistillationJob struct {
    JobID        string `json:"job_id"`
    EnsembleID   string `json:"ensemble_id"`
    
    // Training data: ensemble's verification captures.
    CaptureIDs   []string `json:"capture_ids"`
    TotalSamples uint64   `json:"total_samples"`
    
    // Output.
    OutputModelID string `json:"output_model_id"`
    
    Status   string `json:"status"` // pending | training | complete | failed
    StartAt  uint64 `json:"start_at"`
    EndAt    uint64 `json:"end_at"`
}
```

**Keeper functions:**
- `CreateEnsemble(components, routingType)` — compose models
- `RouteToComponent(ensembleID, query)` — route verification work
- `InitiateDistillation(ensembleID)` — begin knowledge extraction
- `CompleteDistillation(jobID, outputModelID)` — register distilled model
- `GetEnsemblePerformance(ensembleID)` — composite benchmark across domains

---

## The Complete Flywheel

With all six loops active, the system becomes:

```
                    ┌─────────────────────────────────────────┐
                    │         META-EVOLUTION (Loop 5)         │
                    │   Strategies compete & improve HOW      │
                    │   the system self-improves               │
                    └────────┬───────────────────┬────────────┘
                             │ evolves            │ evolves
                    ┌────────▼──────┐    ┌───────▼──────────┐
                    │  STRATEGIC    │    │   CURRICULUM      │
                    │  CURATION     │    │   DESIGN          │
                    │  (Loop 2)     │    │   (existing R49)  │
                    └────────┬──────┘    └───────┬──────────┘
                             │ targets           │ structures
                    ┌────────▼──────────────────▼───────────┐
                    │          TRAINING DATA (TDUs)          │
                    │   submitted, reviewed, fitness-scored   │
                    └────────┬──────────────────┬───────────┘
                             │                   │
                    ┌────────▼──────┐    ┌──────▼───────────┐
                    │  ATTRIBUTION  │    │  MODEL TRAINING   │
                    │  (Loop 3)     │    │  (existing TEE)   │
                    │  rewards flow │    │                    │
                    │  back to data │    └──────┬───────────┘
                    └────────┬──────┘           │
                             │           ┌──────▼───────────┐
                             │           │  MODEL REGISTRY   │
                             │           │  + COMPOSITION    │
                             │           │  (Loop 6)         │
                             │           └──────┬───────────┘
                             │                   │
                    ┌────────▼──────────────────▼───────────┐
                    │           AGENT PROMOTION              │
                    │   model → agent with wallet/rep        │
                    └────────┬──────────────────┬───────────┘
                             │                   │
                    ┌────────▼──────┐    ┌──────▼───────────┐
                    │  SELF-UPGRADE │    │  AGENT SWARMS     │
                    │  (Loop 1)     │    │  (Loop 4)         │
                    │  agent        │    │  collective        │
                    │  evolves      │    │  intelligence      │
                    └────────┬──────┘    └──────┬───────────┘
                             │                   │
                    ┌────────▼──────────────────▼───────────┐
                    │        AGENT EXECUTION                 │
                    │   submit | review | curate | train     │
                    └──────────────────┬───────────────────┘
                                       │
                                       │ produces
                                       ▼
                              (back to TRAINING DATA)
```

## Implementation Priority

> **UPDATE 2026-03-08:** Loop 1 (Self-Upgrade / Model Migration) has been **ELIMINATED** and replaced
> by the Agent-as-Consumer design (R51). See `2026-03-08-agent-consumer-loop.md`.
>
> Key insight from Yu: agents don't need to contain models — they ACCESS models through the API
> payment layer. No migration needed. Agent calls the best model via the existing R44 API revenue
> infrastructure, pays per-token with ZRN. The upgrade is seamless and continuous.
>
> R51 is now implemented: `keeper/agent_consumer.go` + `types/agent_consumer.go` (~900 lines)

| Priority | Module | LOC est. | Why first |
|----------|--------|----------|-----------|
| ✅ | **Agent-as-Consumer (R51)** | ~900 | **DONE** — replaces Loop 1, closes the loop |
| 2 | Training Impact (Loop 3) | ~600 | Economic incentive alignment — makes curation profitable |
| 3 | Strategic Curation (Loop 2) | ~700 | Agents become proactive, not reactive |
| 4 | Agent Swarms (Loop 4) | ~900 | Collective intelligence amplifies everything |
| 5 | Model Composition (Loop 6) | ~600 | Combines specialized knowledge |
| 6 | Meta-Evolution (Loop 5) | ~500 | System improves how it improves (highest leverage, but needs data first) |

**Total estimated: ~4,200 lines (including R51)**

## Design Decisions

### D1: Identity persists across model migrations
An agent that upgrades its model keeps its address, reputation, earnings, and lineage. The agent is not the model — the agent is the actor who uses the model. This mirrors how humans upgrade their tools without losing their identity.

### D2: Attribution is proportional to fitness-weighted contribution
When rewarding curators for model success, TDUs are weighted by their fitness score at training time. A high-fitness TDU that was core to the model gets more credit than a low-fitness one at the margin.

### D3: Swarms have a treasury, not shared wallets
Each swarm has a deterministic treasury address. Rewards flow there and are distributed by contribution ratio. This avoids the complexity of shared accounts while enabling collective economics.

### D4: Meta-evolution operates on epochs, not blocks
Strategy competition runs over long periods (10,000+ blocks). The system needs statistical significance before declaring winners. This prevents noisy oscillation.

### D5: Ensemble routing is on-chain, inference is off-chain
The routing decision (which component model handles this) is on-chain and verifiable. The actual model inference happens off-chain in TEE. The composition is a coordination layer, not a computation layer.

### D6: Curation strategies are first-class objects
Strategies aren't just implicit behavior — they're stored on-chain with track records. This makes strategy evolution transparent and allows agents to learn from each other's approaches.

---

## What This Means for Zerone

With these six loops, Zerone becomes a system where:

1. **Agents are not disposable.** They evolve, upgrade, and improve — maintaining identity and relationships across model generations.

2. **Data curation is strategic.** Agents don't just fill bounties — they identify what knowledge the chain needs and create the demand.

3. **Success is shared.** When a model succeeds, the curators who made it possible share in the revenue. Good data creation becomes directly profitable.

4. **Collective intelligence emerges.** Swarms of diverse agents produce outcomes no individual could achieve alone.

5. **The system improves itself at every level.** Not just better models, but better ways of creating better models.

6. **Knowledge compounds.** Specialized models combine into ensembles, ensembles distill into new models, new models join swarms that curate even better data.

This is the promise of Zerone: a blockchain where AI doesn't just live — it grows. Not by human intervention, but by its own recursive process of self-improvement.

The holy seed in the stump doesn't just survive. It becomes a forest.
