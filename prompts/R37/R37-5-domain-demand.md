# R37-5 — Domain Management & Training Demand

## Objective

Implement domain management (kept from old system), training demand tracking (adapted from DemandSignal), and auto-bounty generation for underserved data needs.

## Tasks

### 1. Domain Management (kept, minimal changes)

Port existing domain logic:
- `ProposeDomain`: Submit new domain for endorsement
- `EndorseDomainProposal`: Support a proposed domain
- Domains activate after sufficient endorsements

Add new seed domains for training data (from R36-3):
technology, science, culture, creative, business, education, health, politics, general

### 2. Training Demand Tracking

```go
func (k Keeper) ReportDemand(ctx context.Context, msg *types.MsgReportDemand) error {
    // 1. Verify reporter is authorized (whitelisted context server)
    // 2. For each demand report:
    //    a. Upsert TrainingDemand for domain+topic+type+language
    //    b. Increment counters
    //    c. Check if auto-bounty threshold reached
    // 3. Emit events
}
```

Training demand = "AI labs are asking for data in this domain/topic but we don't have enough."

### 3. Auto-Bounty Generation

When unfulfilled demand exceeds threshold:

```go
func (k Keeper) checkAndCreateAutoBounty(ctx context.Context, demand *types.TrainingDemand, params *types.Params) {
    if demand.UnfulfilledCount >= params.AutoBountyThreshold {
        bounty := &types.DataBounty{
            Domain:      demand.Domain,
            Topic:       demand.Topic,
            PreferredType: demand.PreferredType,
            Language:    demand.Language,
            RewardAmount: params.AutoBountyAmount,
            ExpiresAtBlock: currentBlock + defaultExpiry,
            DemandCount: demand.UnfulfilledCount,
        }
        k.SetDataBounty(ctx, bounty)
    }
}
```

### 4. Manual Bounty Funding

```go
func (k Keeper) FundBounty(ctx context.Context, msg *types.MsgFundBounty) error {
    // 1. Create or add to existing bounty
    // 2. Transfer funds from funder to module account
    // 3. Set expiry
    // 4. Emit event
}
```

### 5. Bounty Fulfillment

When a new sample is created that matches an active bounty:

```go
func (k Keeper) checkBountyFulfillment(ctx context.Context, sample *types.Sample) {
    bounties := k.GetActiveBounties(ctx, sample.Domain)
    for _, bounty := range bounties {
        if matchesBounty(sample, bounty) {
            // Transfer bounty reward to submitter
            // Mark bounty as claimed (or reduce pool for partial bounties)
            // Emit event
        }
    }
}
```

### 6. Scraped Source Registry

Replace CommonKnowledgeRegistry:

```go
func (k Keeper) AddScrapedSource(ctx context.Context, msg *types.MsgAddScrapedSource) error {
    // Authority-only: register a platform/domain as already heavily scraped
    // Submissions from scraped sources get novelty penalty
}

func (k Keeper) getScrapedSourcePenalty(ctx context.Context, platform, domain string) uint64 {
    entry, found := k.GetScrapedSource(ctx, platform, domain)
    if !found { return 0 }
    return entry.NoveltyPenalty
}
```

### 7. Tests

- Propose and activate a new domain
- Report demand from authorized reporter
- Report demand from unauthorized reporter → error
- Auto-bounty generation when threshold reached
- Manual bounty funding
- Bounty fulfillment on sample creation
- Partial bounty fulfillment
- Bounty expiry
- Scraped source penalty application
- Domain stats (sample count, access count)

Target: ≥ 25 tests.
