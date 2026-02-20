# R2-3 — Knowledge ABCI: Vote Extensions + Proposal Integration

## Goal

Wire Proof of Truth into CometBFT's ABCI++ interface. Validators include
PoT data in vote extensions, and the block proposer aggregates verification
results into the proposal.

## Dependencies

- R2-2 must be complete (keeper with round lifecycle)

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/app/ante.go` — vote extension handling
- `/Users/yuai/Desktop/legible_money/app/app.go` — ABCI++ wiring
- `/Users/yuai/Desktop/legible_money/x/knowledge/` — keeper integration
- CometBFT ABCI++ docs for ExtendVote/VerifyVoteExtension

## ABCI++ Integration Points

### 1. ExtendVote

Called on each validator when casting a vote. Validators attach their
PoT commitments/reveals as vote extensions:

```go
func (app *ZeroneApp) ExtendVote(ctx context.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
    // 1. Check if there are active verification rounds
    // 2. If this validator is a selected verifier with pending action:
    //    - During commit phase: include commit hash
    //    - During reveal phase: include reveal (vote + salt)
    // 3. Marshal the PoT extension data
    // 4. Return as vote extension bytes
}
```

### 2. VerifyVoteExtension

Called on validators to verify other validators' vote extensions:

```go
func (app *ZeroneApp) VerifyVoteExtension(ctx context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
    // 1. Unmarshal the vote extension
    // 2. Verify the extension data is well-formed
    // 3. Verify the commit hash format (if commit phase)
    // 4. Verify the reveal matches previously seen commit (if reveal phase)
    // 5. Return ACCEPT or REJECT
}
```

### 3. PrepareProposal

Block proposer aggregates PoT results:

```go
func (app *ZeroneApp) PrepareProposal(ctx context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
    // 1. Collect vote extensions from previous block
    // 2. Aggregate PoT commits/reveals
    // 3. If a round has enough reveals, include tally transaction
    // 4. Include in proposal alongside normal transactions
}
```

### 4. ProcessProposal

Validates the proposer's work:

```go
func (app *ZeroneApp) ProcessProposal(ctx context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
    // 1. Verify the PoT aggregation is correct
    // 2. Verify tally transactions match the vote extensions
    // 3. Return ACCEPT or REJECT
}
```

## Vote Extension Types

```protobuf
// In proto/zerone/knowledge/v1/abci.proto

message PoTVoteExtension {
  repeated VerificationCommit commits = 1;
  repeated VerificationReveal reveals = 2;
  uint64 block_height = 3;
  string validator = 4;
}

message VerificationCommit {
  string round_id = 1;
  bytes commit_hash = 2;
}

message VerificationReveal {
  string round_id = 1;
  string vote = 2;
  bytes salt = 3;
}
```

## App Configuration

In `app/app.go`, enable vote extensions:

```go
// In PrepareProposalHandler / ProcessProposalHandler setup
app.SetExtendVoteHandler(app.ExtendVote)
app.SetVerifyVoteExtensionHandler(app.VerifyVoteExtension)
app.SetPrepareProposalHandler(app.PrepareProposal)
app.SetProcessProposalHandler(app.ProcessProposal)
```

In genesis config:
```json
{
  "consensus_params": {
    "abci": {
      "vote_extensions_enable_height": 1
    }
  }
}
```

## Security Considerations (from draft audits)

1. **Proposer cannot tamper with PoT verdicts** — ProcessProposal must verify
   the tally matches the collected vote extensions exactly
2. **Vote extensions are signed** — CometBFT handles this, but verify the
   extension format prevents replay
3. **Missing extensions** — validators who don't extend their vote (no pending
   PoT work) send empty extensions, which is valid
4. **Round advancement** — rounds must advance even if some validators are
   offline (BeginBlocker handles timeouts)

## Tests

`x/knowledge/keeper/abci_test.go`:
- TestExtendVote_WithPendingCommit
- TestExtendVote_WithPendingReveal
- TestExtendVote_NoPendingWork
- TestVerifyVoteExtension_ValidCommit
- TestVerifyVoteExtension_InvalidFormat
- TestPrepareProposal_AggregatesExtensions
- TestPrepareProposal_IncludesTallyTx
- TestProcessProposal_VerifiesAggregation
- TestProcessProposal_RejectsTamperedTally

## Verification

```bash
go build ./...
go vet ./...
go test ./x/knowledge/... -count=1 -v
go test ./app/... -count=1 -v
```

## Commit

```
feat(knowledge): ABCI++ integration — vote extensions, PoT proposal handling
```

## Do NOT

- Skip ProcessProposal validation (proposer tampering was a P0 in the draft)
- Hardcode vote extension enable height (use genesis config)
- Assume all validators will have pending PoT work (empty extensions are valid)
- Skip the reveal-matches-commit verification
