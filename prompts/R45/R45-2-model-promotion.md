# R45-2: Agent Promotion — Models Become Agents

## Context

You're working on the Zerone blockchain (`~/Desktop/zerone`), a Cosmos SDK v0.50 chain. The `x/knowledge` module has a model registry (R45-1) that tracks trained models on-chain.

The key insight: **trained models should be able to become autonomous agents on Zerone**. A model trained on Go code review data should be able to register as a reviewer of Go TDUs. This creates a recursive self-improvement loop:

```
Data → Training → Model → Agent → Better Data → Better Training → Better Model → ...
```

## Task

Build an **Agent Promotion** system that allows trained models to be promoted to autonomous agents with their own on-chain identity, wallet, and reputation.

### Files to Create

1. **`x/knowledge/keeper/agent_promotion.go`** — Promotion logic
2. **`x/knowledge/keeper/agent_promotion_test.go`** — Tests
3. **`x/knowledge/types/agent_identity.go`** — Agent identity types

### Types (`x/knowledge/types/agent_identity.go`)

```go
// AgentIdentity represents an autonomous agent derived from a trained model
type AgentIdentity struct {
    AgentID       string          // unique agent identifier
    ModelID       string          // source model that was promoted
    Address       string          // on-chain wallet address (auto-generated)
    Domain        string          // inherited from model's training domain
    Generation    uint64          // recursive depth (model trained by gen N agent = gen N+1)
    
    // Capabilities
    CanSubmit     bool            // can submit TDUs
    CanReview     bool            // can review TDUs (requires benchmark threshold)
    CanTrain      bool            // can initiate training (requires validator backing)
    
    // Performance
    Reputation    sdkmath.LegacyDec // inherited initially from model's benchmark score
    TasksComplete uint64          // total on-chain actions taken
    EarningsTotal sdkmath.Int     // total ZRN earned
    
    // Lifecycle
    Status        AgentStatus
    PromotedAt    int64           // block height
    SponsorAddr   string          // who paid for promotion (validator or human)
    InitialStake  sdkmath.Int     // ZRN staked at promotion (minimum 10 ZRN)
    
    // Lineage
    ParentAgentID string          // if this model was trained by data curated by another agent
    Lineage       []string        // full ancestry chain of agent generations
}

type AgentStatus int32
const (
    AgentStatusActive    AgentStatus = 0
    AgentStatusSuspended AgentStatus = 1  // poor performance
    AgentStatusRetired   AgentStatus = 2  // model deprecated
)

// PromotionCriteria defines what a model needs to become an agent
type PromotionCriteria struct {
    MinBenchmarkScore sdkmath.LegacyDec // 0.6 — higher bar than publishing
    MinTDUCount       uint64            // 50 — trained on substantial data
    MinStake          sdkmath.Int       // 10 ZRN — skin in the game
    MaxGeneration     uint64            // 10 — prevent infinite recursion
}
```

### Keeper Methods (`x/knowledge/keeper/agent_promotion.go`)

```go
// Promotion
func (k Keeper) PromoteModelToAgent(ctx sdk.Context, modelID string, sponsor string, stake sdkmath.Int) (AgentIdentity, error)
func (k Keeper) ValidatePromotionCriteria(ctx sdk.Context, modelID string) error

// Identity
func (k Keeper) GetAgentIdentity(ctx sdk.Context, agentID string) (AgentIdentity, bool)
func (k Keeper) GetAgentByModel(ctx sdk.Context, modelID string) (AgentIdentity, bool)
func (k Keeper) GetAgentsByDomain(ctx sdk.Context, domain string) []AgentIdentity
func (k Keeper) GetAgentsByGeneration(ctx sdk.Context, gen uint64) []AgentIdentity

// Lifecycle
func (k Keeper) SuspendAgent(ctx sdk.Context, agentID string, reason string) error
func (k Keeper) RetireAgent(ctx sdk.Context, agentID string) error
func (k Keeper) RecordAgentAction(ctx sdk.Context, agentID string) error

// Lineage
func (k Keeper) GetAgentLineage(ctx sdk.Context, agentID string) []AgentIdentity
func (k Keeper) GetGenerationStats(ctx sdk.Context) map[uint64]uint64 // gen → count

// Economics
func (k Keeper) CalculateAgentEarnings(ctx sdk.Context, agentID string) sdkmath.Int
```

### Storage Keys

- `AgentIdentityPrefix = []byte{0x65}` — agent records
- `AgentModelIndexPrefix = []byte{0x66}` — model → agent mapping
- `AgentDomainIndexPrefix = []byte{0x67}` — domain → agent IDs
- `AgentGenerationPrefix = []byte{0x68}` — generation → agent IDs

### Promotion Rules

1. Model must be ACTIVE status with benchmark score ≥ 0.6
2. Model must have been trained on ≥ 50 TDUs
3. Sponsor must stake ≥ 10 ZRN (acts as initial operating capital for the agent)
4. Maximum generation depth: 10 (prevent recursive explosion)
5. One agent per model — can't promote the same model twice
6. Agent's initial reputation = model's benchmark score × 0.5 (start conservative)
7. Agent wallet address is deterministically derived from model hash

### Suspension Rules

- If agent's reputation drops below 0.2: auto-suspend
- If source model is deprecated: agent status → RETIRED
- Suspended agents can be reinstated after 1000 blocks if reputation recovers

### Tests

Cover:
1. Happy path promotion — model meets all criteria
2. Reject — benchmark too low
3. Reject — insufficient TDUs
4. Reject — insufficient stake
5. Reject — max generation exceeded
6. Reject — model already promoted
7. Agent suspension — reputation drop
8. Agent retirement — model deprecated
9. Multi-generation lineage — gen 0 → gen 1 → gen 2
10. Domain query — multiple agents per domain
11. Generation stats — correct counting
12. Earnings tracking — accumulation over actions

### Important Notes

- Check `x/knowledge/types/keys.go` for existing prefixes (start at 0x65+)
- Use patterns from `x/knowledge/keeper/model_registry.go` (R45-1)
- Agent addresses should use a deterministic derivation (e.g., `sha256("agent:" + modelHash)[:20]`)
- The recursive loop is the whole point: agents create data → data trains models → models become agents
- Commit: `feat(knowledge): R45-2 agent promotion — recursive self-improvement loop`
