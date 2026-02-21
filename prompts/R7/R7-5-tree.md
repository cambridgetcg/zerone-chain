# R7-5 — Tree Module: Service Registry & Revenue Routing

## Goal

Port x/tree — the project/service registry. Projects register as service nodes in a tree
structure, track contributors, route revenue through the tree, and integrate with billing
for service pricing and channels for payment flows.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/tree/` — full module (5521 LOC keeper, 3 test files)
- `/Users/yuai/Desktop/legible_money/proto/legible/tree/v1/` — all 4 protos
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`, `LGM` → `ZRN`

## Proto Files

Port all from draft, converting to zerone namespace:

### `proto/zerone/tree/v1/state.proto`
Port these messages:
- `ContributorRecord` — DID, role, tasks completed, total earned
- `ProductSpec` — service type, requirements, capabilities, MVP criteria
- `FundingSource` — type, amount, terms
- `Project` — the core entity: id, name, owner, status, contributors, product spec,
  parent project (tree structure), revenue share config, funding sources
- `ServiceRegistration` — links a project to a callable service endpoint
- `RevenueRoute` — defines how revenue flows through the tree (parent takes X%, rest to contributors)

### `proto/zerone/tree/v1/tx.proto`
Messages:
- MsgCreateProject: register a new project node
- MsgUpdateProject: update metadata, status
- MsgAddContributor: add contributor to project
- MsgRemoveContributor: remove contributor
- MsgRegisterService: register callable service for a project
- MsgDeregisterService: remove service registration
- MsgSetRevenueRoute: configure revenue routing (BPS splits)
- MsgSetAvailability: mark service available/unavailable
- MsgUpdateParams (authority-gated)

### `proto/zerone/tree/v1/query.proto`
- QueryParams, QueryProject (by ID), QueryProjectsByOwner, QueryService (by project),
  QueryContributors (by project), QueryRevenueRoute, QueryChildProjects (tree traversal)

### `proto/zerone/tree/v1/genesis.proto`
- GenesisState: params + projects + services + revenue routes + next IDs

## Module Implementation

### Keeper
Port from draft keeper:
- **Project CRUD**: create/update/delete projects. Tree structure via parent_id field
- **Contributor management**: add/remove, track earnings per contributor
- **Service registration**: link project to callable endpoint (type, URL, requirements)
- **Revenue routing**: when a service earns revenue, distribute through the tree:
  - Service project gets (1M - parent_share) BPS
  - Parent project gets parent_share BPS (recursively up the tree)
  - Within a project, split among contributors by configured shares
- **Availability**: services can be marked available/unavailable (used by toolbox for discovery)

### Expected Keepers
```go
type BillingKeeper interface {
    GetServicePrice(ctx sdk.Context, serviceID string) (sdk.Coin, error)
}
type ChannelsKeeper interface {
    GetChannel(ctx sdk.Context, channelID string) (*channeltypes.PaymentChannel, error)
}
type BankKeeper interface {
    SendCoinsFromModuleToAccount(ctx, moduleName, addr, coins) error
    SendCoinsFromAccountToModule(ctx, addr, moduleName, coins) error
}
```

### Default Params
- MaxTreeDepth: 5 (prevent infinite nesting)
- MaxContributorsPerProject: 50
- MinParentShare: 0 BPS (parent can take 0%)
- MaxParentShare: 500000 BPS (parent can take up to 50%)
- DefaultParentShare: 100000 BPS (10%)
- ProjectRegistrationFee: 100000 uzrn (0.1 ZRN)

## Tests

Port from draft + add:
1. Create project, verify storage
2. Tree structure: create child projects, verify parent-child links
3. Add/remove contributors
4. Revenue routing: service earns → splits through tree correctly
5. Max tree depth enforced
6. Service registration and availability toggle
7. Revenue share BPS validation (sum must equal 1M within project)
8. Genesis import/export round-trip

## Constraints

- Proto-first, gogoproto generation
- 1M BPS scale for all revenue shares
- Revenue route validation: contributor shares within a project must sum to 1M
- Tree depth validation: reject projects that would exceed MaxTreeDepth
- Wire in app.go, no EndBlocker needed (event-driven)
- Service availability state used by x/toolbox for discovery (interface only, no direct import)
