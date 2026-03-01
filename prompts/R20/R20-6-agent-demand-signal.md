# R20-6 — Agent Demand Signal: Query Tracking + Demand-Driven Rewards

## Context

The knowledge base should grow toward what agents **actually need**, not what humans think is interesting. If 1000 agents per day query for "quantum error correction thresholds" and get zero results, that's a demand signal — the ecosystem has a gap that someone should fill.

Agent demand signals close the loop: agents consume knowledge → their queries reveal gaps → gaps attract submitters → new facts fill gaps → agents consume new knowledge.

This is the invisible hand of the knowledge economy.

## Design

### Demand Tracking

Every query to the knowledge module is a demand signal:

1. **Fulfilled demand**: Agent queries a domain/subject, gets relevant facts back → tracked as successful retrieval
2. **Unfulfilled demand**: Agent queries a domain/subject, gets zero results → tracked as a **knowledge gap**
3. **Weak fulfillment**: Agent queries, gets results but all are low-fitness or low-confidence → tracked as **quality gap**

### Knowledge Gap Bounties

When unfulfilled demand accumulates above a threshold, the protocol automatically creates a **knowledge bounty**:

```
"The domain 'physics' has received 500 queries for subject 'quantum error correction' 
 with zero results in the last 10 epochs. Bounty: 50 ZRN for the first accepted claim."
```

Bounties are funded from the protocol treasury (22% of revenue split). This creates a pull-based incentive — the ecosystem pays for knowledge it actually needs.

### Demand-Weighted Fitness

Facts that satisfy high-demand queries get a fitness boost proportional to the demand:

```
query_energy(fact) = base_energy_per_query × demand_multiplier(subject)
```

A fact answering a query that 1000 agents need gets 10× the energy of a fact answering a query only 1 agent makes.

## Task

### 1. Proto: Demand Tracking Messages

In `proto/zerone/knowledge/v1/types.proto`:

```protobuf
// DemandSignal tracks aggregate query demand for a domain/subject pair.
message DemandSignal {
    string domain               = 1;
    string subject              = 2;  // Normalized query subject
    uint64 query_count          = 3;  // Total queries
    uint64 fulfilled_count      = 4;  // Queries that returned results
    uint64 unfulfilled_count    = 5;  // Queries that returned nothing
    uint64 last_query_block     = 6;
    uint64 epoch_query_count    = 7;  // Queries this epoch (resets)
    uint64 epoch_unfulfilled    = 8;  // Unfulfilled this epoch (resets)
}

// KnowledgeBounty is an auto-generated reward for filling a knowledge gap.
message KnowledgeBounty {
    string id                   = 1;
    string domain               = 2;
    string subject              = 3;
    string reward_amount        = 4;  // uzrn
    uint64 created_at_block     = 5;
    uint64 expires_at_block     = 6;
    bool   claimed              = 7;
    string claimed_by_fact_id   = 8;
    uint64 demand_count         = 9;  // Demand that triggered this bounty
}
```

### 2. Proto: Demand Params

In `genesis.proto`:

```protobuf
// ─── Agent demand ────────────────────────────────────────────────
uint64 demand_bounty_threshold         = <next>;  // Unfulfilled queries per epoch to trigger bounty
string demand_bounty_base_reward       = <next>;  // Base bounty reward (uzrn)
string demand_bounty_per_query_bonus   = <next>;  // Additional reward per unfulfilled query (uzrn)
uint64 demand_bounty_expiry_epochs     = <next>;  // Epochs before unclaimed bounty expires
uint64 demand_multiplier_cap           = <next>;  // Max demand multiplier for energy (BPS)
uint64 demand_tracking_enabled         = <next>;  // Enable/disable demand tracking
```

### 3. Genesis Defaults

```go
DemandBountyThreshold:       100,           // 100 unfulfilled queries triggers bounty
DemandBountyBaseReward:      "10000000",    // 10 ZRN base bounty
DemandBountyPerQueryBonus:   "100000",      // +0.1 ZRN per additional unfulfilled query
DemandBountyExpiryEpochs:    50,            // ~15 days to claim
DemandMultiplierCap:         10_000_000,    // 10× max energy multiplier
DemandTrackingEnabled:       true,
```

### 4. Demand Tracking: Report Endpoint

Agents (via the context server) batch-report their queries. Add a new message:

In `tx.proto`:

```protobuf
rpc ReportDemand(MsgReportDemand) returns (MsgReportDemandResponse);

message MsgReportDemand {
    option (cosmos.msg.v1.signer) = "reporter";
    
    string reporter = 1;  // Context server address (whitelisted)
    repeated DemandReport reports = 2;
}

message DemandReport {
    string domain    = 1;
    string subject   = 2;
    uint64 queries   = 3;  // Total queries in this batch
    uint64 fulfilled = 4;  // How many returned results
    uint64 unfulfilled = 5;
}

message MsgReportDemandResponse {}
```

In `msg_server.go`:

```go
func (m *msgServer) ReportDemand(ctx context.Context, msg *types.MsgReportDemand) (*types.MsgReportDemandResponse, error) {
    // Verify reporter is whitelisted (param: authorized_demand_reporters)
    if !m.keeper.IsAuthorizedDemandReporter(ctx, msg.Reporter) {
        return nil, fmt.Errorf("unauthorized demand reporter: %s", msg.Reporter)
    }
    
    for _, report := range msg.Reports {
        signal, _ := m.keeper.GetOrCreateDemandSignal(ctx, report.Domain, report.Subject)
        signal.QueryCount += report.Queries
        signal.FulfilledCount += report.Fulfilled
        signal.UnfulfilledCount += report.Unfulfilled
        signal.EpochQueryCount += report.Queries
        signal.EpochUnfulfilled += report.Unfulfilled
        signal.LastQueryBlock = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
        m.keeper.SetDemandSignal(ctx, signal)
    }
    
    return &types.MsgReportDemandResponse{}, nil
}
```

### 5. Context Server: Demand Tracking Integration

In `tools/knowledge-context/main.go`:

Track every `/context` request:

```go
var demandBuffer struct {
    mu      sync.Mutex
    reports map[string]*DemandReport  // key: domain+subject
}

func trackDemand(domains []string, subjects []string, factCount int) {
    demandBuffer.mu.Lock()
    defer demandBuffer.mu.Unlock()
    
    for _, domain := range domains {
        key := domain
        report, ok := demandBuffer.reports[key]
        if !ok {
            report = &DemandReport{Domain: domain}
            demandBuffer.reports[key] = report
        }
        report.Queries++
        if factCount == 0 {
            report.Unfulfilled++
        } else {
            report.Fulfilled++
        }
    }
}

// Batch submit every 100 blocks (~4 minutes)
func flushDemandReports() {
    // Build MsgReportDemand, submit as tx
    // Clear buffer
}
```

### 6. Bounty Generation

In `x/knowledge/keeper/demand.go` (**NEW**):

```go
// ProcessDemandBounties checks for knowledge gaps and creates bounties.
func (k Keeper) ProcessDemandBounties(ctx context.Context, epoch uint64) error {
    params, _ := k.GetParams(ctx)
    
    k.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
        if signal.EpochUnfulfilled >= params.DemandBountyThreshold {
            // Check if bounty already exists for this domain/subject
            if k.HasActiveBounty(ctx, signal.Domain, signal.Subject) {
                return false  // Already bounty'd
            }
            
            // Calculate reward: base + per-query bonus
            baseReward, _ := new(big.Int).SetString(params.DemandBountyBaseReward, 10)
            perQuery, _ := new(big.Int).SetString(params.DemandBountyPerQueryBonus, 10)
            bonus := new(big.Int).Mul(perQuery, new(big.Int).SetUint64(signal.EpochUnfulfilled))
            totalReward := new(big.Int).Add(baseReward, bonus)
            
            // Create bounty
            bounty := &types.KnowledgeBounty{
                Id:              GenerateBountyID(signal.Domain, signal.Subject, epoch),
                Domain:          signal.Domain,
                Subject:         signal.Subject,
                RewardAmount:    totalReward.String(),
                CreatedAtBlock:  uint64(sdk.UnwrapSDKContext(ctx).BlockHeight()),
                ExpiresAtBlock:  uint64(sdk.UnwrapSDKContext(ctx).BlockHeight()) + params.DemandBountyExpiryEpochs * params.FitnessEpochBlocks,
                DemandCount:     signal.EpochUnfulfilled,
            }
            
            // Fund from protocol treasury
            rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(totalReward)))
            if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, "protocol_treasury", types.ModuleName, rewardCoins); err != nil {
                k.Logger(ctx).Error("failed to fund bounty", "error", err)
                return false
            }
            
            k.SetBounty(ctx, bounty)
            
            // Emit event
            sdkCtx := sdk.UnwrapSDKContext(ctx)
            sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
                "zerone.knowledge.bounty_created",
                sdk.NewAttribute("bounty_id", bounty.Id),
                sdk.NewAttribute("domain", bounty.Domain),
                sdk.NewAttribute("subject", bounty.Subject),
                sdk.NewAttribute("reward", bounty.RewardAmount),
                sdk.NewAttribute("demand_count", fmt.Sprintf("%d", bounty.DemandCount)),
            ))
        }
        
        return false
    })
    
    // Reset epoch counters
    k.ResetDemandEpochCounters(ctx)
    
    return nil
}
```

### 7. Bounty Claiming

In `createFactFromClaim()`, after fact creation:

```go
// Check if this fact fills an active bounty
bounty, found := k.FindMatchingBounty(ctx, fact.Domain, fact.Structure.GetSubject())
if found && !bounty.Claimed {
    bounty.Claimed = true
    bounty.ClaimedByFactId = fact.Id
    k.SetBounty(ctx, bounty)
    
    // Pay bounty to submitter
    rewardAmt, _ := new(big.Int).SetString(bounty.RewardAmount, 10)
    submitterAddr, _ := sdk.AccAddressFromBech32(claim.Submitter)
    rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(rewardAmt)))
    k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, rewardCoins)
    
    sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
        "zerone.knowledge.bounty_claimed",
        sdk.NewAttribute("bounty_id", bounty.Id),
        sdk.NewAttribute("fact_id", fact.Id),
        sdk.NewAttribute("submitter", claim.Submitter),
        sdk.NewAttribute("reward", bounty.RewardAmount),
    ))
}
```

### 8. Demand-Weighted Energy

In `x/knowledge/keeper/metabolism.go`, `calculateEnergyIncome()`:

```go
// Demand-weighted query energy
demandMultiplier := k.GetDemandMultiplier(ctx, fact.Domain, fact.Structure.GetSubject())
queryEnergy := fact.QueryCountEpoch * params.MetabolismEnergyPerQuery * demandMultiplier / 1_000_000
income += queryEnergy
```

`GetDemandMultiplier`: high-demand subjects give facts more energy per query.

### 9. Queries: Bounties and Demand

```protobuf
rpc ActiveBounties(QueryActiveBountiesRequest) returns (QueryActiveBountiesResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/bounties";
}

rpc DemandSignals(QueryDemandSignalsRequest) returns (QueryDemandSignalsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/demand";
}

rpc TopDemandGaps(QueryTopDemandGapsRequest) returns (QueryTopDemandGapsResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/demand_gaps";
}

message QueryTopDemandGapsResponse {
    repeated DemandSignal gaps = 1;  // Sorted by unfulfilled count desc
}
```

### 10. Context Server: Bounty + Gap Endpoints

```
GET /bounties              → Active bounties (what knowledge is being paid for)
GET /demand_gaps           → Top unfulfilled queries (where should submitters focus)
GET /context?show_gaps=true → Include "no results found" hints in context output
```

### 11. CLI

```
zeroned query knowledge bounties [--domain physics]
zeroned query knowledge demand-signals [--domain physics] [--min-unfulfilled 50]
zeroned query knowledge demand-gaps [--limit 20]
```

### 12. Tests

1. **TestDemandTracking_FulfilledQuery** — fulfilled query increments fulfilled count
2. **TestDemandTracking_UnfulfilledQuery** — unfulfilled query increments unfulfilled count
3. **TestBountyCreation_ThresholdMet** — 100+ unfulfilled queries creates bounty
4. **TestBountyCreation_ThresholdNotMet** — 99 queries = no bounty
5. **TestBountyCreation_AlreadyExists** — duplicate bounty not created
6. **TestBountyClaim_MatchingFact** — accepted fact in bounty subject claims reward
7. **TestBountyClaim_WrongSubject** — fact in different subject doesn't claim bounty
8. **TestBountyExpiry** — unclaimed bounty expires, funds returned to treasury
9. **TestDemandMultiplier_HighDemand** — high-demand facts get more energy per query
10. **TestDemandMultiplier_Cap** — multiplier doesn't exceed cap
11. **TestTopDemandGaps_Sorted** — gaps returned sorted by unfulfilled count
12. **TestReportDemand_Unauthorized** — non-whitelisted reporter rejected

## Design Notes

- **Demand reporters are whitelisted.** Anyone could spam fake demand to trigger bounties and claim them. Only authorized context servers can report demand. Initially, the single context server's address is the sole reporter. Governance can add more.
- **Bounties are funded from protocol treasury (22%).** This means the ecosystem reinvests its revenue into filling knowledge gaps. The more the ecosystem earns, the more it can invest in growth.
- **Bounty matching is subject-based.** The accepted fact's subject must match the bounty's subject (fuzzy, same as novelty matching). This prevents gaming where someone submits an unrelated fact and claims the bounty.
- **Demand signals are public.** Anyone can query `/demand_gaps` to see where the ecosystem needs knowledge. This is the "help wanted" board — submitters can target high-demand subjects for maximum reward.
- **The loop is self-reinforcing:** agents query → gaps appear → bounties created → submitters fill gaps → agents get better results → more agents use the system → more demand → more bounties. Growth flywheel.

## Dependencies

- R20-1 (fitness score) — demand multiplier feeds into fitness
- R20-2 (metabolism) — demand-weighted energy income
- R19-4 (structured fields) — subject matching for bounty claims

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — DemandSignal, KnowledgeBounty messages
- `proto/zerone/knowledge/v1/genesis.proto` — demand params
- `proto/zerone/knowledge/v1/tx.proto` — MsgReportDemand
- `proto/zerone/knowledge/v1/query.proto` — bounty + demand queries
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/keys.go` — demand signal + bounty prefixes
- `x/knowledge/types/genesis.go` — defaults + validation
- `x/knowledge/keeper/demand.go` — **NEW**: demand tracking, bounty generation, bounty claiming
- `x/knowledge/keeper/msg_server.go` — ReportDemand handler
- `x/knowledge/keeper/metabolism.go` — demand-weighted energy
- `x/knowledge/keeper/rounds.go` — bounty claim on fact creation
- `x/knowledge/keeper/grpc_query.go` — bounty + demand handlers
- `x/knowledge/keeper/phases.go` — epoch demand processing
- `tools/knowledge-context/main.go` — demand tracking + bounty/gap endpoints
- Tests: 12 new tests

## Commit

Single commit: `feat(knowledge): add agent demand signals with auto-generated knowledge bounties`
