# R19-1 — Add "malformed" Vote Option to Verification

## Context

Verifiers currently vote `"accept"` or `"reject"` on knowledge claims. This is insufficient for claims that are **not truth-apt** — self-referential paradoxes, category errors, meaningless statements, or syntactically invalid claims. Today these produce split votes and an INCONCLUSIVE verdict, returning the submitter's stake in full. This means submitting garbage has zero cost if verifiers can't agree.

A `"malformed"` vote lets verifiers explicitly flag incoherent claims. When a malformed supermajority is reached, the submitter is slashed more aggressively (wasting verifier time is an attack), and verifiers who correctly identified the malformation are rewarded.

## Task

### 1. Proto: Add VERDICT_MALFORMED

In `proto/zerone/knowledge/v1/types.proto`, add to the `Verdict` enum:

```protobuf
VERDICT_MALFORMED = 4;  // Claim is not truth-apt (paradox, category error, nonsense)
```

Add to `ClaimStatus` enum:

```protobuf
CLAIM_STATUS_MALFORMED = 12;  // Rejected as not truth-apt
```

Regenerate Go proto files.

### 2. Proto: Add MalformedClaimSlashBps Param

In `proto/zerone/knowledge/v1/genesis.proto`, add to `Params`:

```protobuf
uint64 malformed_claim_slash_bps = <next_field_number>;  // default: 500,000 (50%) — harsher than invalid_claim (22%)
```

### 3. Genesis Defaults

In `x/knowledge/types/genesis.go`, set:

```go
MalformedClaimSlashBps: 500_000, // 50% — submitting garbage wastes verifier time
```

Add validation in `Validate()`:

```go
if p.MalformedClaimSlashBps == 0 {
    return fmt.Errorf("malformed_claim_slash_bps must be > 0")
}
```

### 4. Vote Validation in SubmitReveal

In `x/knowledge/keeper/msg_server.go`, `SubmitReveal()`, update the vote validation:

```go
// Before:
if msg.Vote != "accept" && msg.Vote != "reject" {
    return nil, fmt.Errorf("invalid vote: must be 'accept' or 'reject'")
}

// After:
if msg.Vote != "accept" && msg.Vote != "reject" && msg.Vote != "malformed" {
    return nil, fmt.Errorf("invalid vote: must be 'accept', 'reject', or 'malformed'")
}
```

### 5. Aggregation Logic

In `x/knowledge/keeper/confidence.go`, `AggregateVerificationResult()`:

**Add malformed stake tracking** alongside accept/reject:

```go
var acceptStake, rejectStake, malformedStake, totalVoteStake uint64

// In the reveal loop:
case "malformed":
    malformedStake += stake
```

**Add malformed verdict determination** — check malformed FIRST (before accept/reject), because a malformed claim should never become a fact regardless of accept votes:

```go
malformedRatio := safeMulDiv(malformedStake, 1_000_000, totalVoteStake)

if malformedRatio >= params.ConfidenceThreshold {
    result.Verdict = types.Verdict_VERDICT_MALFORMED
    result.Confidence = malformedRatio
} else if acceptRatio >= params.ConfidenceThreshold {
    result.Verdict = types.Verdict_VERDICT_ACCEPT
    // ...existing logic
} else if rejectRatio >= params.ConfidenceThreshold {
    // ...existing logic
}
```

### 6. Rewards & Slashes for Malformed Verdict

In `calculateRewardsAndSlashes()`:

When verdict is `VERDICT_MALFORMED`:
- `correctVote = "malformed"` — verifiers who voted malformed get rewarded
- Verifiers who voted "accept" get slashed at `WrongVerificationSlashBps` (they tried to accept garbage)
- Verifiers who voted "reject" get a **reduced reward** (half of base) — they correctly opposed the claim but didn't identify it as malformed specifically. This is a soft incentive to use the right label.

```go
case types.Verdict_VERDICT_MALFORMED:
    correctVote = "malformed"
    // "reject" voters get partial reward (saw through it but wrong label)
    partialRewardVote = "reject"
    partialRewardRatio = 500_000 // 50% of base reward
```

### 7. Round Completion for Malformed

In `CompleteRound()`, add a `VERDICT_MALFORMED` case:

```go
case types.Verdict_VERDICT_MALFORMED:
    // Slash submitter harder than invalid claims — they wasted verifier time with nonsense
    if err := k.slashAndBurnClaimStake(ctx, claim, params.MalformedClaimSlashBps); err != nil {
        k.Logger(ctx).Error("failed to slash malformed claim", "claim_id", claim.Id, "error", err)
    }
    claim.Status = types.ClaimStatus_CLAIM_STATUS_MALFORMED
```

No fact is created. No vesting schedule. The submitter eats a 50% slash.

### 8. CLI

Ensure the CLI help text for `submit-reveal` documents the three vote options:

```
--vote string    Verification vote: "accept", "reject", or "malformed"
```

### 9. Tests

Add to `x/knowledge/keeper/confidence_test.go`:

1. **TestAggregate_UnanimousMalformed** — 3 verifiers all vote malformed → VERDICT_MALFORMED, confidence = 1,000,000
2. **TestAggregate_MalformedSupermajority** — 2 malformed + 1 accept → malformed wins if above threshold
3. **TestAggregate_MalformedBelowThreshold** — 1 malformed + 1 accept + 1 reject → INCONCLUSIVE (no supermajority)
4. **TestAggregate_MalformedTrumpsAccept** — malformed checked before accept, so even if accept would pass threshold, malformed wins if it also passes
5. **TestMalformedSlash_SubmitterPenalized** — verify submitter loses 50% stake on malformed verdict
6. **TestMalformedReward_RejectVotersGetPartial** — reject voters get 50% reward, malformed voters get full reward

Add to `x/knowledge/keeper/security_test.go`:

7. **TestMalformed_SybilCostAnalysis** — N sybils voting malformed on a legitimate claim: verify they all get slashed when the legitimate claim passes ACCEPT

## Design Notes

- **Malformed > Accept priority**: A claim that N% of stake-weighted verifiers call malformed should NEVER become a fact, even if another N% voted accept. Check malformed first.
- **Partial reward for reject on malformed**: "reject" is *directionally correct* (the claim shouldn't be accepted) but imprecise. Half reward incentivizes using the right label without harshly punishing close-enough votes.
- **50% slash on submitter**: This is intentionally harsh. `InvalidClaimSlashBps` (22%) covers honest mistakes — wrong facts submitted in good faith. `MalformedClaimSlashBps` (50%) covers claims that should never have been submitted. The distinction: a false claim about physics is invalid; "this statement is false" is malformed.
- **No MALFORMED for challenges**: Challenge-facts ("Challenge of fact X: reason") are meta-claims about existing facts. They shouldn't be votable as malformed — the underlying fact either survives or doesn't. Keep malformed for original claims only.

## Files Modified

- `proto/zerone/knowledge/v1/types.proto` — add VERDICT_MALFORMED, CLAIM_STATUS_MALFORMED
- `proto/zerone/knowledge/v1/genesis.proto` — add malformed_claim_slash_bps param
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/genesis.go` — default + validation
- `x/knowledge/keeper/msg_server.go` — vote validation
- `x/knowledge/keeper/confidence.go` — aggregation + rewards/slashes
- `x/knowledge/keeper/rounds.go` — CompleteRound malformed case
- `x/knowledge/keeper/confidence_test.go` — 6 new tests
- `x/knowledge/keeper/security_test.go` — 1 new test

## Commit

Single commit: `feat(knowledge): add malformed vote for non-truth-apt claims`
