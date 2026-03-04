# R39-3 — Cross-Module Integration

## Objective

Wire the pivoted knowledge module into the rest of the ZERONE chain: tree (data collection campaigns), vesting_rewards (revenue flow), toolbox (agent data queries).

## Tasks

### 1. knowledge ↔ tree: Data Collection Campaigns

The tree module manages projects. A new project type: **data collection campaign**.

```go
// In x/tree/types — add or extend:
const ProjectTypeDataCollection = "data_collection"

// A data collection project specifies:
// - Target domain/topic
// - Target sample count
// - Quality threshold
// - Budget (from project funds)
// - Deadline
```

When a tree project is type `data_collection`:
- It creates a DataBounty in the knowledge module
- Task completion = submission acceptance
- Project revenue = access fees from collected data

Update `x/tree/keeper/` to call knowledge keeper:
```go
func (k Keeper) linkDataCollectionProject(ctx context.Context, project *types.ProductProject) error {
    // Create corresponding DataBounty via knowledge keeper
    return k.knowledgeKeeper.CreateProjectBounty(ctx, bounty)
}
```

Update expected_keepers in tree to include knowledge methods it needs.

### 2. knowledge ↔ vesting_rewards: Revenue Integration

The protocol's share of data access revenue should flow through the existing vesting_rewards distribution:

```go
// In knowledge EndBlocker, when distributing protocol share:
func (k Keeper) sendProtocolRevenue(ctx context.Context, amount sdk.Coins) error {
    // Send to vesting_rewards fee collector
    // This gets split: 55% contributors, 22% protocol, etc.
    // But here "contributors" = data submitters (already paid directly)
    // So protocol share goes to: validators + development fund + research fund
    return k.vestingKeeper.DepositFees(ctx, amount)
}
```

Verify the revenue split in vesting_rewards handles knowledge module deposits correctly. The knowledge module already pays submitters directly — so the protocol share sent to vesting_rewards should NOT double-pay submitters.

### 3. knowledge ↔ toolbox: Agent Data Queries

The toolbox module provides agent-facing APIs. Add training data query capabilities:

```go
// In x/toolbox — add query bridge:
func (k Keeper) QueryTrainingData(ctx context.Context, req *QueryTrainingDataRequest) (*QueryTrainingDataResponse, error) {
    // Proxy to knowledge module's sample queries
    // Add agent-specific formatting (ready for prompt injection)
    return k.knowledgeKeeper.QuerySamples(ctx, convertRequest(req))
}
```

### 4. Update App Wiring

In `app/app.go` or wherever modules are wired:
- Pass knowledge keeper to tree module
- Pass vesting_rewards keeper to knowledge module
- Update any interface assertions

### 5. Update Simulation Registration

Register new knowledge Msg types in the app's simulation manager with appropriate weights:

```go
// In app/sim.go or equivalent
weightMsgSubmitData      = 100  // Most common operation
weightMsgSubmitThread    = 30   // Less common (batch)
weightMsgSubmitCommitment = 80
weightMsgSubmitReveal    = 80
weightMsgContestSample   = 10
weightMsgAccessSample    = 50
weightMsgAccessDataset   = 20
weightMsgCreateDataset   = 5
weightMsgFundBounty      = 10
```

### 6. Tests

- Tree data collection project creates knowledge bounty
- Knowledge bounty fulfilled by submission → tree task completed
- Revenue flows from knowledge → vesting_rewards correctly
- No double-payment to submitters
- Toolbox can query training data
- App wiring compiles with all cross-references
- Simulation runs with new message types

Target: ≥ 15 tests.
