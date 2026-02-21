# R7-4 — Research Module: Funded Investigation

## Goal

Port x/research — on-chain research submissions, peer review, bounties, and funding from the
research fund. Researchers stake ZRN, submit work targeting specific facts, reviewers score it,
accepted research earns rewards from the research fund.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/research/` — full module (2720 LOC keeper, 2 test files)
- `/Users/yuai/Desktop/legible_money/proto/legible/research/v1/` — all 4 protos (state, tx, query, genesis)
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`, `LGM` → `ZRN`

## Proto Files

Port all 4 from draft, converting to zerone namespace:

### `proto/zerone/research/v1/state.proto`
Port `Research`, `Bounty`, `Review`, `ResearchFunding` messages from draft.
Key fields on Research: id, submitter, title, description, domain, research_type
("replication", "fraud_investigation", "methodology_audit", "data_validation"),
target_fact_id, stake, status, review counts, aggregate_score, timestamps.

### `proto/zerone/research/v1/tx.proto`
Messages:
- MsgSubmitResearch: submit research with stake
- MsgReviewResearch: peer review (approve/reject/revise) with score
- MsgCreateBounty: create bounty for specific domain/fact
- MsgClaimBounty: claim bounty with research submission
- MsgFundResearch: direct funding from research fund (authority-gated)
- MsgUpdateParams (authority-gated)

### `proto/zerone/research/v1/query.proto`
- QueryParams, QueryResearch (by ID), QueryResearchByDomain (paginated),
  QueryBounty (by ID), QueryActiveBounties, QueryReviewsByResearch

### `proto/zerone/research/v1/genesis.proto`
- GenesisState: params + researches + bounties + reviews + next IDs

## Module Implementation

### Keeper
Port from draft keeper:
- **Submit**: validate stake ≥ MinStake, check target fact exists (knowledge keeper),
  escrow stake, create research with "submitted" status
- **Review**: only qualified reviewers (staking tier ≥ MinReviewerTier), score 0-100,
  aggregate scores, transition status when review quorum met
- **Bounty lifecycle**: create → fund → claim → complete/expire
- **Funding**: authority can disburse from research fund module account
- **Status transitions**: submitted → under_review → accepted/rejected/challenged
- **Rewards**: accepted research gets stake back + reward from research fund proportional to score

### Expected Keepers
```go
type StakingKeeper interface {
    GetValidatorTier(ctx sdk.Context, addr sdk.AccAddress) (uint32, error)
}
type KnowledgeKeeper interface {
    GetFact(ctx sdk.Context, id string) (*types.Fact, error)
}
type BankKeeper interface {
    SendCoinsFromAccountToModule(ctx, addr, moduleName, coins) error
    SendCoinsFromModuleToAccount(ctx, moduleName, addr, coins) error
}
```

### Default Params
- MinStake: 1000000 uzrn (1 ZRN)
- MinReviewerTier: 2 (Verified tier)
- ReviewQuorum: 3
- AcceptThreshold: 70 (score out of 100)
- BountyExpiryBlocks: 100000
- MaxActiveResearchPerSubmitter: 5

## Tests

Port from draft + add:
1. Submit research with valid stake
2. Submit fails with insufficient stake
3. Review by qualified reviewer updates scores
4. Review by unqualified reviewer rejected
5. Status transitions on quorum
6. Bounty create → claim → complete
7. Bounty expiry
8. Genesis import/export round-trip

## Constraints

- Proto-first, gogoproto generation
- 1M BPS scale for any percentage fields
- Research fund is a module account (reuse from vesting_rewards research fund address)
- Wire in app.go, register all msg/query handlers
- No EndBlocker needed (all event-driven via txs) — unless bounty expiry needs periodic cleanup
