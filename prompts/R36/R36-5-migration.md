# R36-5 — Migration & Module Registration

## Objective

Write the v4 migration for the knowledge module (from fact-claim state to training data state) and update `module.go` to register new message handlers.

## Tasks

### 1. Create migrations/v4/migrate.go

Since this is a testnet pivot (not a live chain migration), the migration can be aggressive:

```go
package v4

// Migrate performs the v4 migration: fact-claim system → training data protocol.
// For testnet: clears all old state and initializes fresh.
// For mainnet (future): would need to convert existing facts → samples.
func Migrate(ctx context.Context, ps ParamStore) error {
    // 1. Set new default params (quality thresholds, consent multipliers, etc.)
    params := types.DefaultParams()
    return ps.SetParams(ctx, &params)
}
```

For testnet, old facts/claims are simply dropped (they won't exist on a fresh chain anyway). Document what a mainnet migration would look like as comments.

### 2. Update module.go

- Bump consensus version to 4
- Register v4 migrator
- Update `RegisterServices` to route new Msg types to new keeper methods
- Update `RegisterInvariants` for new state (sample count, energy conservation, etc.)

### 3. Update keeper interface

Define the new `Keeper` interface that `module.go` will use. Don't implement yet (R37 does that) — just define the interface:

```go
type KnowledgeKeeper interface {
    // Submissions
    SubmitData(ctx context.Context, msg *MsgSubmitData) (*MsgSubmitDataResponse, error)
    SubmitThread(ctx context.Context, msg *MsgSubmitThread) (*MsgSubmitThreadResponse, error)

    // Quality validation
    SubmitCommitment(ctx context.Context, msg *MsgSubmitCommitment) (*MsgSubmitCommitmentResponse, error)
    SubmitReveal(ctx context.Context, msg *MsgSubmitReveal) (*MsgSubmitRevealResponse, error)

    // etc.
}
```

### 4. Update expected_keepers.go

Review and update the interface that other modules expect from knowledge:
- `toolbox` may query knowledge — update interface
- `tree` links to knowledge domains — verify compatibility

### 5. Update CLI

Rewrite `x/knowledge/client/cli/`:
- `tx.go`: New CLI commands for `submit-data`, `submit-thread`, `contest-sample`, etc.
- `query.go`: New CLI commands for `sample`, `samples`, `submission`, `dataset`, etc.
- Remove old `submit-claim`, `query-fact` commands

### 6. Proto registration bridge

Update `proto_register.go` to register new types with gogoproto:

```go
func init() {
    proto.RegisterType((*Sample)(nil), "zerone.knowledge.v1.Sample")
    proto.RegisterType((*Submission)(nil), "zerone.knowledge.v1.Submission")
    proto.RegisterType((*ConsentProof)(nil), "zerone.knowledge.v1.ConsentProof")
    proto.RegisterType((*QualityVote)(nil), "zerone.knowledge.v1.QualityVote")
    proto.RegisterType((*Dataset)(nil), "zerone.knowledge.v1.Dataset")
    proto.RegisterType((*Params)(nil), "zerone.knowledge.v1.Params")
}
```

## Verification

```bash
go build ./x/knowledge/...    # Full module compiles
go build ./...                 # Whole app compiles (may need stub keeper methods)
```

At this point the type system is complete. Keeper implementation follows in R37.
