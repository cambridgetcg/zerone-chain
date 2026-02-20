# R2-2 — Knowledge Keeper: Fact CRUD + Round Lifecycle

## Goal

Implement the knowledge module's keeper — all state management, the full
commit/reveal verification round lifecycle, VRF-based verifier selection,
confidence scoring, and slashing integration.

## Dependencies

- R2-1 must be complete (proto types generated)

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/knowledge/keeper/` — all keeper files
- `/Users/yuai/Desktop/legible_money/x/knowledge/keeper/msg_server.go` — 21 handlers
- `/Users/yuai/Desktop/legible_money/x/knowledge/keeper/keeper.go` — state CRUD
- `/Users/yuai/Desktop/legible_money/x/knowledge/keeper/grpc_query.go` — queries
- `/Users/yuai/Desktop/legible_money/reports/audits/` — security findings

## Keeper Structure

```go
type Keeper struct {
    storeKey      storetypes.StoreKey
    cdc           codec.Codec
    authority     string

    // Cross-module keepers
    accountKeeper types.AccountKeeper
    bankKeeper    types.BankKeeper
    stakingKeeper types.StakingKeeper
    ontologyKeeper types.OntologyKeeper // set post-init

    // Module accounts
    feeCollectorName string
}
```

## State Operations

Port all state CRUD from draft's `keeper.go`. Use proto marshal (NOT JSON):

```go
func (k Keeper) SetFact(ctx sdk.Context, fact *types.Fact) {
    store := ctx.KVStore(k.storeKey)
    bz, _ := proto.Marshal(fact)
    store.Set(types.FactKey(fact.Id), bz)
}

func (k Keeper) GetFact(ctx sdk.Context, id string) (*types.Fact, bool) {
    store := ctx.KVStore(k.storeKey)
    bz := store.Get(types.FactKey(id))
    if bz == nil {
        return nil, false
    }
    var fact types.Fact
    proto.Unmarshal(bz, &fact)
    return &fact, true
}
```

## Message Handlers (21)

Port all handlers from draft. Key ones:

### MsgSubmitClaim
1. Validate claim (domain exists, content not empty, references valid)
2. Check submitter stake meets minimum
3. Store claim with status "submitted"
4. If enough verifiers available, start verification round
5. Emit event

### MsgCommitVerification
1. Verify the sender is a selected verifier for this round
2. Verify round is in "commit" phase
3. Store commit hash
4. Emit event

### MsgRevealVerification
1. Verify round is in "reveal" phase
2. Verify hash(vote || salt) matches stored commit
3. Store reveal
4. If all reveals received (or phase ends), trigger tally
5. Emit event

### Tally Logic
```go
func (k Keeper) tallyVerification(ctx sdk.Context, round *types.VerificationRound) {
    accepts := 0
    rejects := 0
    for _, reveal := range round.Reveals {
        if reveal.Vote == "accept" { accepts++ }
        if reveal.Vote == "reject" { rejects++ }
    }

    // Simple majority
    if accepts > rejects {
        round.Verdict = "accept"
        // Create fact from claim, set initial confidence
        k.createFactFromClaim(ctx, round)
    } else {
        round.Verdict = "reject"
    }

    // Reward correct voters, slash incorrect voters
    k.distributeVerificationOutcomes(ctx, round)
}
```

### Slashing Integration
```go
func (k Keeper) slashVerifier(ctx sdk.Context, verifier string, slashBps uint64) {
    // Call staking keeper to slash the verifier's stake
    // slashBps is from params (non-zero, enforced in validation)
}
```

### VRF Verifier Selection
```go
func (k Keeper) selectVerifiers(ctx sdk.Context, claim *types.Claim) []string {
    // Use VRF output to deterministically select N verifiers
    // from the active validator set, weighted by tier
    // Exclude the claim submitter
}
```

### Confidence Scoring
```go
func (k Keeper) updateConfidence(ctx sdk.Context, fact *types.Fact) {
    // Fundamentality: how many other facts cite this one
    // Cross-references: how many facts this one cites that are verified
    // Re-verification: each successful re-verification boosts confidence
}
```

## BeginBlocker

```go
func (k Keeper) BeginBlocker(ctx sdk.Context) {
    // 1. Check for rounds in "commit" phase past deadline → advance to "reveal"
    // 2. Check for rounds in "reveal" phase past deadline → force tally
    //    (missing reveals → slash for missed_reveal)
    // 3. Epoch boundary: update fact confidence scores
}
```

## Query Handlers

Port all 12 queries from draft's `grpc_query.go`.

## Wire into app.go

- Store key, keeper, ModuleManager
- BeginBlocker (knowledge runs LAST — depends on all other module state)
- EndBlocker (EARLY — publish computed state for other modules)
- Set post-init keepers: `SetOntologyKeeper`, `SetStakingKeeper`

## Verification

```bash
go build ./...
go vet ./...
go test ./x/knowledge/... -count=1 -v  # (basic compile test, full tests in R2-5)
```

## Commit

```
feat(knowledge): keeper — fact CRUD, commit/reveal rounds, VRF selection, slashing
```

## Do NOT

- Skip the slashing calls (they must be non-zero and enforced)
- Use JSON marshal for state (proto only)
- Skip VRF validation (verifier selection must be deterministic and verifiable)
- Hardcode any economic constants (all from params)
- Skip BeginBlocker phase advancement (rounds can't get stuck)
