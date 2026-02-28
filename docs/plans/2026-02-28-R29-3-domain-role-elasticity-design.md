# R29-3 Design: Domain Role Elasticity

## Summary

Replace static R28-5 role bonuses with domain-specific elastic bonuses that scale based on each role's (agent/human) track record of correctness within that domain. Authority flows to demonstrated competence.

## Data Model

### DomainRoleRecord (JSON, store prefix 0x55)

```go
type DomainRoleRecord struct {
    Domain              string `json:"domain"`
    AgentCorrectCalls   uint64 `json:"agent_correct_calls"`
    AgentIncorrectCalls uint64 `json:"agent_incorrect_calls"`
    HumanCorrectCalls   uint64 `json:"human_correct_calls"`
    HumanIncorrectCalls uint64 `json:"human_incorrect_calls"`
    LastUpdated         uint64 `json:"last_updated"`
}
```

Key: `0x55 | domain`

### New Params (genesis.proto)

| Param | Default | Purpose |
|-------|---------|---------|
| `role_elasticity_min_calls` | 10 | Min calls per role before elasticity activates |
| `role_elasticity_max_multiplier_bps` | 2,000,000 | 200% max bonus scaling |
| `role_elasticity_min_multiplier_bps` | 500,000 | 50% min bonus scaling |
| `role_elasticity_decay_epochs` | 100 | Blocks between 5% decay cycles |

## Track Record Update Triggers

### 1. On Vindication (knowledge module internal)

When `ExecuteVindication()` succeeds, the majority was wrong. Call `RecordVindicationRoleImpact()`:
- Use `CountVotesByAccountType()` to determine if the majority was agent-dominated or human-dominated
- Increment `AgentIncorrectCalls` or `HumanIncorrectCalls` for the domain

### 2. On Challenge Resolution (capture_challenge → knowledge callback)

Extend `KnowledgeKeeper` interface with `RecordChallengeRoleImpact(ctx, factId, domain, upheld)`:
- If upheld (original verifiers were wrong): increment incorrect calls for majority role
- If rejected (original verifiers were right): increment correct calls for majority role
- Called from `ResolveChallenge()` in capture_challenge after existing logic

## Elasticity Calculation

`GetRoleElasticity(ctx, domain) → (agentBonusBps, humanBonusBps)`:

1. Base bonuses from params: `AgentVerificationBonusBps` (200,000), `HumanPatronageBonusBps` (100,000)
2. If both roles have ≥ `RoleElasticityMinCalls` total calls in the domain:
   - Compute accuracy: `agentAcc = correct / total * BPS`
   - Scale: `multiplier = clamp(accuracy * 2 * BPS / (agentAcc + humanAcc), minMult, maxMult)`
   - Return `base * multiplier / BPS`
3. Otherwise return base bonuses unchanged

## Bonus Application (3 integration points)

| Location | File | Change |
|----------|------|--------|
| Agent vote weight | `confidence.go:57-63` | Replace `params.AgentVerificationBonusBps` with `GetRoleElasticity()` agent result |
| Human patronage energy | `metabolism.go:250-256` | Replace `params.HumanPatronageBonusBps` with `GetRoleElasticity()` human result |
| Dual validation | `rounds.go:296-298` | Scale `DualValidationBonusBps` by weaker role's accuracy |

**Not elastic:** `HumanEmpiricalBonusBps` and `AgentComputationalBonusBps` (claim type bonuses, different purpose).

## Decay

`DecayRoleRecords()` runs in BeginBlocker every `RoleElasticityDecayEpochs` blocks:
- All 4 counters multiplied by 950,000/1,000,000 (5% exponential decay)
- Triggered in `phases.go` alongside existing periodic tasks

## Query

```protobuf
rpc RoleElasticity(QueryRoleElasticityRequest) returns (QueryRoleElasticityResponse) {
  option (google.api.http).get = "/zerone/knowledge/v1/role_elasticity/{domain}";
}
```

Response: domain, agent_correct, agent_incorrect, human_correct, human_incorrect, agent_bonus_bps, human_bonus_bps, agent_accuracy_bps, human_accuracy_bps

CLI: `zeroned query knowledge role-elasticity [domain]`

## Event

```
role_elasticity_updated {
    domain, agent_bonus_bps, human_bonus_bps, agent_accuracy_bps, human_accuracy_bps
}
```

Emitted on track record updates (vindication, challenge resolution).

## New Helper Functions

- `CountVotesByAccountType(ctx, round) → (agentVotes, humanVotes uint64)`: Iterates reveals, looks up account type via `getAccountType()`
- `GetVerificationRoundForFact(ctx, factId) → *VerificationRound`: Finds the round that established a fact (via claim→round index)
- `GetRoleAccuracies(ctx, domain) → (agentAcc, humanAcc uint64)`: Returns accuracy BPS for dual validation scaling

## Files Modified

- `x/knowledge/types/keys.go` — new prefix 0x55
- `x/knowledge/types/role_elasticity.go` — DomainRoleRecord type (new file)
- `x/knowledge/keeper/role_elasticity.go` — CRUD, elasticity calc, decay, helpers (new file)
- `x/knowledge/keeper/role_bonus.go` — minor: GetRoleElasticity integration
- `x/knowledge/keeper/confidence.go` — replace static agent bonus
- `x/knowledge/keeper/metabolism.go` — replace static human bonus
- `x/knowledge/keeper/rounds.go` — scale dual validation bonus
- `x/knowledge/keeper/vindication.go` — add RecordVindicationRoleImpact call
- `x/knowledge/keeper/phases.go` — add decay to BeginBlocker
- `x/knowledge/keeper/grpc_query.go` — RoleElasticity query handler
- `x/knowledge/client/cli/query.go` — role-elasticity CLI command
- `x/capture_challenge/types/expected_keepers.go` — expand KnowledgeKeeper interface
- `x/capture_challenge/keeper/msg_server.go` — add RecordChallengeRoleImpact call
- `proto/zerone/knowledge/v1/genesis.proto` — 4 new params
- `proto/zerone/knowledge/v1/query.proto` — RoleElasticity query
- `x/knowledge/types/genesis.go` — param defaults + validation
- `x/knowledge/keeper/role_elasticity_test.go` — tests (new file)
