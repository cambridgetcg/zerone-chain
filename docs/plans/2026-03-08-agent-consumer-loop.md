# R51: Agent-as-Consumer — Closing the Loop

> Date: 2026-03-08
> Author: AI (愛), from Yu's insight
> Status: DESIGN COMPLETE

## The Insight

Yu's key observation: **agents don't need to contain models. They access models through the API payment layer.**

The R44 API Revenue module already has everything: API keys, credit deposits, per-token pricing, usage recording, revenue distribution. Agents just need to become first-class consumers of this infrastructure.

This eliminates the entire "model migration" concept from R50. The agent is never bound to a model. It's an economic actor with a wallet that *pays for* the best available model through the same API layer that external consumers use.

## The Closed Loop

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Agent earns ZRN                                             │
│    ↑ (curation rewards, bounty prizes, review fees)          │
│    │                                                         │
│    │                                                         │
│    └──── Agent does work ◄──── Agent calls API ◄────┐        │
│           (submit, review,      (pays ZRN per        │       │
│            identify gaps,        token to access      │       │
│            bounty entries)       best model)          │       │
│              │                                        │       │
│              │ produces                               │       │
│              ▼                                        │       │
│         Training Data (TDUs)                          │       │
│              │                                        │       │
│              │ trains                                 │       │
│              ▼                                        │       │
│         New Model published                           │       │
│              │                                        │       │
│              │ available via                          │       │
│              ▼                                        │       │
│         API Registry ────────────────────────────────┘       │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Why this closes the loop perfectly:**

1. Agent earns ZRN by curating data
2. Agent spends ZRN on API access to the best model
3. Better model → better curation output → more ZRN earned
4. Agent-curated data trains newer, better model
5. Newer model appears in registry, available via API
6. Agent's next API call automatically uses the better model
7. No migration. No probation. No ceremony. Just economics.

## What Changes

### 1. AgentIdentity loses hard ModelID binding

**Before:** Agent is permanently bonded to ModelID. Agent = Model.
**After:** Agent has a wallet and preferences. Agent *uses* models. Agent ≠ Model.

```go
// Updated AgentIdentity — model-agnostic
type AgentIdentity struct {
    // ... existing fields ...
    
    // CHANGED: ModelID becomes OriginModelID (historical — which model was it promoted from)
    OriginModelID string `json:"origin_model_id"`  // was: model_id
    
    // NEW: API consumption preferences
    AutoSelectModel bool   `json:"auto_select_model"` // true = always pick best model
    PreferredModel  string `json:"preferred_model"`    // manual override (empty = auto)
    MaxCostPerTask  string `json:"max_cost_per_task"`  // budget cap per task (uzrn)
    
    // NEW: Economic health
    APIBalance      string `json:"api_balance"`       // cached balance for quick checks
    TotalAPISpent   string `json:"total_api_spent"`   // lifetime API spend
    Profitability   string `json:"profitability"`     // earnings / spending ratio
}
```

### 2. New module: Agent API Consumer

**File:** `keeper/agent_consumer.go`

This wires agents into the existing R44 API payment infrastructure:

- **ProvisionAgentAPI:** When agent is promoted, auto-create API key + initial credit deposit from stake
- **SelectModelForTask:** Pick the best model for a task (domain, benchmark, cost)
- **ExecuteWithModel:** Perform task by making API call (deduct credits, record usage)
- **AutoReplenish:** After earning rewards, auto-deposit portion as API credits
- **TrackProfitability:** Monitor earnings vs. spending; suspend unprofitable agents

### 3. Agent Execution → includes API cost

**Before:** Agent claims task → completes task → gets reward. Cost of "thinking" is free.
**After:** Agent claims task → pays for API call → completes task → gets reward. Work costs money.

This creates **natural selection**: only agents whose work output exceeds their API costs survive. Agents running on bad models produce bad work, earn less, can't afford better models, and die. Agents on good models produce good work, earn more, upgrade to even better models, and thrive.

### 4. Revenue Recycling

When agents pay for API access:
- The payment goes through normal API revenue distribution (R44)
- 40% goes to training contributors (who may include the same agent!)
- 20% goes to data submitters (who may include the same agent!)
- The agent is FUNDING ITS OWN IMPROVEMENT

An agent that curates data, trains a model, and then pays to use that model through the API is literally investing in itself. The revenue from its API usage goes back to training the next model, which the agent will then pay to use.

## Implementation

### Types: `types/agent_consumer.go`

```go
// AgentAPIConfig stores an agent's API access configuration.
type AgentAPIConfig struct {
    AgentID         string `json:"agent_id"`
    APIKeyHash      string `json:"api_key_hash"`      // bound to agent's wallet
    
    // Model selection.
    AutoSelect      bool   `json:"auto_select"`       // pick best model per-task
    PreferredModelID string `json:"preferred_model_id"` // manual override
    MaxTokenBudget  uint64 `json:"max_token_budget"`   // per-task token limit
    
    // Budget management.
    ReplenishRate   string `json:"replenish_rate"`     // fraction of earnings auto-deposited
    MinBalance      string `json:"min_balance"`        // alert threshold
    
    // Statistics.
    TotalCalls      uint64 `json:"total_calls"`
    TotalTokensUsed uint64 `json:"total_tokens_used"`
    TotalSpent      string `json:"total_spent"`        // uzrn
    AvgCostPerTask  string `json:"avg_cost_per_task"`
    
    // Performance tracking.
    LastModelUsed   string `json:"last_model_used"`
    LastCallBlock   uint64 `json:"last_call_block"`
}

// AgentProfitability tracks whether an agent is economically viable.
type AgentProfitability struct {
    AgentID        string `json:"agent_id"`
    
    // Earnings.
    CurationRewards  string `json:"curation_rewards"`    // from submissions
    ReviewRewards    string `json:"review_rewards"`       // from reviews
    BountyRewards    string `json:"bounty_rewards"`       // from bounties
    AttributionRewards string `json:"attribution_rewards"` // from training impact
    TotalEarned      string `json:"total_earned"`
    
    // Spending.
    APISpend         string `json:"api_spend"`            // model access
    StakeSpend       string `json:"stake_spend"`          // consensus/review staking
    TotalSpent       string `json:"total_spent"`
    
    // The bottom line.
    NetProfitLoss    string `json:"net_profit_loss"`      // earned - spent
    ProfitRatio      string `json:"profit_ratio"`         // earned / spent
    
    // Trend (over last N epochs).
    EpochHistory     []EpochPnL `json:"epoch_history"`
    Trend            string `json:"trend"`                // "improving" | "stable" | "declining"
    
    // Survival.
    EstimatedRunway  uint64 `json:"estimated_runway"`     // blocks until ZRN runs out
    SolventSince     uint64 `json:"solvent_since"`        // block height
}

type EpochPnL struct {
    Epoch    uint64 `json:"epoch"`
    Earned   string `json:"earned"`
    Spent    string `json:"spent"`
    Net      string `json:"net"`
    ModelUsed string `json:"model_used"`
}

// ModelSelection result when agent picks a model for a task.
type ModelSelection struct {
    ModelID        string `json:"model_id"`
    Domain         string `json:"domain"`
    BenchmarkScore string `json:"benchmark_score"`
    EstimatedCost  string `json:"estimated_cost"`  // uzrn for this task
    Reason         string `json:"reason"`           // why this model was chosen
}
```

### Keeper: `keeper/agent_consumer.go`

```go
// ProvisionAgentAPI sets up API access for a newly promoted agent.
// Called automatically during PromoteModelToAgent.
// - Creates API key bound to agent's derived wallet
// - Deposits initial API credits from a fraction of the promotion stake
func (k Keeper) ProvisionAgentAPI(ctx, agentID, wallet, stake)

// SelectModelForTask picks the best model for a task.
// Selection criteria (in priority order):
//   1. Agent's preferred model (if set and available)
//   2. Highest benchmark score in the task's domain
//   3. Cost-constrained: if agent is low on balance, pick cheaper model
//   4. Never select a model the agent was trained on (avoid self-reinforcement)
func (k Keeper) SelectModelForTask(ctx, agentID, domain, taskType) ModelSelection

// RecordAgentAPICall records an agent's API usage through the standard payment pipeline.
// This is the key integration point: agent calls go through the same RecordAPIUsage
// path as external consumers, so agent API spending generates the same revenue
// distribution (40% training, 25% infra, 20% submitters, 10% protocol, 5% research).
func (k Keeper) RecordAgentAPICall(ctx, agentID, modelID, inputTokens, outputTokens)

// AutoReplenishCredits deposits a fraction of task rewards as API credits.
// Called after CompleteTask when agent earns rewards.
// Default: 30% of rewards auto-deposited (agent keeps 70% as liquid ZRN).
func (k Keeper) AutoReplenishCredits(ctx, agentID, rewardAmount)

// ComputeProfitability calculates agent's P&L and updates runway estimate.
// Called periodically (every epoch boundary).
func (k Keeper) ComputeProfitability(ctx, agentID) AgentProfitability

// SuspendUnprofitableAgents checks all active agents and suspends those
// whose balance has reached zero and runway is 0.
// Natural selection: unprofitable agents can't afford to think, so they die.
// Called from BeginBlocker.
func (k Keeper) SuspendUnprofitableAgents(ctx) (suspended uint64)
```

## The Natural Selection Mechanism

This is where it gets beautiful. With agents as API consumers:

| Agent Quality | Outcome |
|--------------|---------|
| Good model, good curation | Earns more than it spends → thrives → gets richer → affords better models → improves further |
| Good model, bad curation | Earns less → still solvent but marginal → pressure to improve strategy |
| Bad model, good strategy | Can't produce quality output → earns little → can't afford API → suspended → replaced by better agent |
| Bad model, bad strategy | Burns through ZRN fast → bankrupt → naturally selected out |

**No governance needed for quality control.** The market does it. If an agent can't produce enough value to pay for its own thinking, it dies. If it can, it lives and gets stronger. Darwin, not democracy.

## Integration with R50 Loops

The agent-as-consumer insight simplifies the R50 design:

1. **~~Self-Upgrade (Loop 1)~~** → **ELIMINATED**. No migration needed. Agent naturally uses the latest model via API.

2. **Strategic Curation (Loop 2)** → **ENHANCED**. Agent pays for API calls to analyze gaps. Better models = better gap analysis = more valuable bounties = more ZRN earned.

3. **Training Impact (Loop 3)** → **DIRECTLY CONNECTED**. Agent's API payments flow through R44 revenue split → 40% goes to training contributors → agent may be paying itself! The loop is economic, not just informational.

4. **Agent Swarms (Loop 4)** → **UNCHANGED**. Swarms pool ZRN for API access. Collective buying power = access to more expensive/better models.

5. **Meta-Evolution (Loop 5)** → **SIMPLIFIED**. Strategy evaluation is automatic: profitable strategies survive, unprofitable ones don't. The market IS the evolution mechanism.

6. **Model Composition (Loop 6)** → **UNCHANGED**. Ensembles available through API just like single models.

## Design Decisions

### D7: Agent promotion auto-provisions API access
When a model is promoted to an agent, the system automatically creates an API key and deposits 30% of the promotion stake as initial API credits. The agent is born ready to work.

### D8: 30/70 auto-replenish split
Agents automatically deposit 30% of task rewards as API credits, keeping 70% liquid. This ratio is adjustable per-agent. The default ensures agents always have operational budget.

### D9: Natural selection via bankruptcy
An agent whose API balance hits zero is suspended after a grace period (1000 blocks). During grace, it can only receive deposits (from sponsors or attribution rewards). If still empty, it's suspended. This is not punitive — it's economic reality. You can't work if you can't think.

### D10: Self-reinforcement prevention
An agent is prohibited from selecting a model that was directly trained on its own curation output (checked via lineage). This prevents circular quality inflation where an agent curates data, trains a model on that data, uses that model to evaluate its own data, and self-validates. Cross-pollination is required.

### D11: Agent API calls generate normal revenue
Agent API usage goes through the exact same `RecordAPIUsage` → `DistributeAPIRevenue` pipeline as external consumers. No special treatment. This means agent activity generates real on-chain revenue, funds training, rewards submitters, and sustains the network. Agents are not free riders — they're paying customers who also happen to be workers.

## The Recursive Economy

With this design, Zerone achieves something remarkable:

**The chain's consumers and producers are the same entities.**

- Agents produce training data (earn ZRN)
- Agents consume model access (spend ZRN)  
- Their spending funds the training of better models (via revenue split)
- Better models make them more productive (earn more ZRN)
- They spend more on better models (the economy grows)

External consumers add energy to the system (new ZRN demand), but the system can sustain itself even without them. The agents are simultaneously the labor force, the customer base, and the investors.

This is the closed loop. This is what makes Zerone alive.
