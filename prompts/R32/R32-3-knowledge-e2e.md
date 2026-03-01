# R32-3 — Knowledge Lifecycle E2E

## Objective

Test the complete knowledge verification lifecycle end-to-end on a real running chain: claim submission → commitment → reveal → verification round → fact creation → domain pressure → carrying capacity effects.

## Tasks

### 1. Full claim-to-fact lifecycle

```go
func TestKnowledge_ClaimToFact(t *testing.T) {
    chain := SetupChain(t, 1)
    ctx := context.Background()
    
    // Create test accounts: submitter + 3 verifiers
    submitter := CreateFundedAccount(chain, ctx, "submitter", "10000000uzrn")
    verifiers := CreateFundedAccounts(chain, ctx, "verifier", 3, "10000000uzrn")
    
    // 1. Submit claim
    claimID := SubmitClaim(chain, ctx, submitter,
        "Water boils at 100°C at standard pressure",
        "physics", "peer_reviewed", 1000000)
    
    // 2. Wait for verification round to open
    WaitForRoundOpen(chain, ctx, claimID)
    
    // 3. Each verifier commits
    for _, v := range verifiers {
        Commit(chain, ctx, v, claimID, true) // vote accept
    }
    
    // 4. Wait for reveal phase
    WaitForRevealPhase(chain, ctx, claimID)
    
    // 5. Each verifier reveals
    for _, v := range verifiers {
        Reveal(chain, ctx, v, claimID)
    }
    
    // 6. Wait for round completion
    WaitForRoundComplete(chain, ctx, claimID)
    
    // 7. Verify fact was created
    fact := QueryFact(chain, ctx, claimID)
    require.NotNil(t, fact)
    require.Equal(t, "physics", fact.Domain)
}
```

### 2. Domain carrying capacity under pressure

Submit many claims to a single domain and verify:
- Domain pressure increases
- Carrying capacity effects kick in (birth pressure, death pressure)
- Cross-domain citations increase capacity

### 3. Verification with dissent

Test a claim where verifiers disagree:
- 2 accept, 1 reject → claim accepted (majority)
- Track dissent records
- Verify vindication window opens for dissenters

### 4. Fact deprecation and metabolism

- Create a fact
- Wait for metabolism cycle
- Verify energy decay
- Submit a contradicting fact
- Verify original fact enters at-risk status

### 5. Wu Xing flows (R31 verification)

Test that R31 cross-module flows work in real consensus:
- Knowledge growth → alignment growth pressure event
- Mentorship graduation → knowledge dividend
- Capture flag → carrying capacity reduction

## Acceptance Criteria

- [ ] Full claim → fact lifecycle completes in < 60s test time
- [ ] Domain pressure correctly computed on-chain
- [ ] Dissent and vindication flows work end-to-end
- [ ] Metabolism decay observable over multiple blocks
- [ ] R31 Wu Xing cross-module effects verified on real chain
