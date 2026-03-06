# R41-4 — Agent SDK

## Objective

Create a Go SDK package at `pkg/agentsdk/` that provides a programmatic interface for agents to interact with the Tree of Knowledge. This is what AI agents import to participate autonomously.

## Package Structure

```
pkg/agentsdk/
├── client.go      — ToKClient struct, connection management
├── submit.go      — SubmitData, SubmitThread, SubmitCorrection
├── review.go      — CommitReview, RevealReview, AutoRevealAll
├── query.go       — GetDashboard, GetReputation, GetFitness, ListBounties
├── contest.go     — ContestSample, SponsorSample
├── salt.go        — Salt storage/retrieval for commit-reveal
├── types.go       — SDK-specific types (TDUContent, ReviewResult, Dashboard)
└── sdk_test.go    — Tests against mock chain
```

## Core API

```go
// Create client
client, err := agentsdk.NewToKClient(agentsdk.Config{
    NodeURL:    "http://localhost:26657",
    ChainID:    "zerone-testnet-1",
    KeyringDir: "~/.zeroned",
    FromName:   "agent1",
})

// Submit training data
result, err := client.SubmitData(ctx, agentsdk.SubmitRequest{
    Type:       agentsdk.TypeInstructionResponse,
    Domain:     "code",
    Difficulty: agentsdk.DifficultyStandard,
    Content:    myContent,
    ConsentType: agentsdk.ConsentOriginal,
})

// Review a submission
err = client.CommitReview(ctx, roundID, score, salt)
err = client.RevealReview(ctx, roundID) // auto-loads salt
err = client.AutoRevealAll(ctx) // reveals all pending

// Check my status
dashboard, err := client.GetDashboard(ctx)
rep, err := client.GetReputation(ctx, "code")
fitness, err := client.GetFitness(ctx, tduID)

// Find work (bounties to fill)
bounties, err := client.ListOpenBounties(ctx, "code")

// Monitor rounds I'm involved in
rounds, err := client.GetMyActiveRounds(ctx)
```

## Key Features

1. **Auto-stake calculation**: SDK queries chain params, computes stake automatically
2. **Salt management**: Encrypted salt storage at `~/.zeroned/review-salts/`
3. **Reveal scheduler**: `client.WatchAndReveal(ctx)` — goroutine that auto-reveals when rounds enter reveal phase
4. **Retry logic**: Configurable retry on broadcast failure
5. **Event subscription**: `client.SubscribeToRounds(ctx, handler)` — WebSocket subscription to quality round events

## Tests

- Test: SubmitData builds correct MsgSubmitData with content hash
- Test: CommitReview computes correct seal
- Test: RevealReview loads salt and builds correct MsgRevealScore
- Test: AutoRevealAll finds and reveals all pending reviews
- Test: Salt round-trips (store → load → verify)
- Test: Dashboard aggregates multiple queries correctly
- Test: Stake calculation matches chain params

Use mock chain client (interface-based) for unit tests.

## Key Files

- `pkg/agentsdk/` — NEW package
- Depends on: `x/knowledge/types/`, Cosmos SDK client libs

## Constraints

- SDK should work against any Cosmos SDK chain running x/knowledge
- No CGO dependencies — pure Go for easy cross-compilation
- Thread-safe: agents may run multiple goroutines
