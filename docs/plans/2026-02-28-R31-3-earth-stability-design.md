# R31-3 Earth Stability: Governance as the Center — Design

## Summary

Wire governance (Earth) into the Wu Xing cycle with two new connections:

1. **Wood → Earth:** Alignment health pressure expedites knowledge-related LIP voting periods
2. **Earth → Water:** Governance can freeze partnership formation in specific domains

Plus one verification test for the existing Earth → Metal connection (param changes to capture_defense).

## Connection 1: Earth → Metal (verify only)

No code changes. The existing flow works:
- LIP `category: "parameter"` with `ParamChanges: [{module: "capture_defense", key: "hhi_threshold", value: "..."}]`
- Passes vote → `executeParamChanges` → `ParamRouter.ApplyParamChange` → capture_defense handler

**Deliverable:** One integration test confirming end-to-end.

## Connection 2: Wood → Earth (expedited voting)

### Mechanism

When a LIP transitions from `last_call` → `voting` (`abci.go:46`), compute an effective voting period:

```
effective = params.VotingPeriodBlocks
if LIP.ParamChanges target knowledge/alignment/capture_defense
   AND alignment health is "degraded" or "critical":
     effective = params.VotingPeriodBlocks / 2
```

### Implementation

1. **x/alignment keeper** — Add `GetHealthCategory(ctx) string` convenience method reading the latest stored `AlignmentHealthIndex` category.
2. **x/gov/types/expected_keepers.go** — Add `AlignmentKeeper` interface: `GetHealthCategory(ctx context.Context) string`.
3. **x/gov/keeper/keeper.go** — Add `alignmentKeeper` field + setter.
4. **x/gov/keeper/abci.go:46** — Replace `params.VotingPeriodBlocks` with `k.getEffectiveVotingPeriod(ctx, lip)`.
5. **x/gov/keeper/expedited.go** (new file) — `getEffectiveVotingPeriod` + `isKnowledgeParamLIP` helper. Target modules: `"knowledge"`, `"alignment"`, `"capture_defense"`.
6. **app/app.go** — Wire alignment keeper into gov keeper.

### Events

```
zerone.gov.expedited_voting {
    lip_id, target_modules, health_category,
    base_voting_period, effective_voting_period
}
```

## Connection 3: Earth → Water (domain formation freeze)

### Mechanism

`MsgDomainFormationFreeze` (authority-gated) sets a freeze with an `expiry_height`. Partnerships module checks for active freezes in `ProposePartnership`. Freezes auto-expire in `BeginBlocker`.

### Proto changes

**gov tx.proto:**
```protobuf
message MsgDomainFormationFreeze {
  option (cosmos.msg.v1.signer) = "authority";
  string authority       = 1;
  string domain          = 2;
  uint64 duration_blocks = 3;
  string reason          = 4;
}
message MsgDomainFormationFreezeResponse {}
```

**partnerships types.proto:**
```protobuf
message DomainFormationFreeze {
  string domain        = 1;
  uint64 expiry_height = 2;
  string reason        = 3;
}
```

### Implementation

1. **proto/zerone/gov/v1/tx.proto** — Add `MsgDomainFormationFreeze` + response + RPC.
2. **x/gov/keeper/msg_server.go** — Add `DomainFormationFreeze` handler (authority check, compute expiry_height, delegate to partnerships keeper).
3. **x/gov/types/expected_keepers.go** — Add `PartnershipsKeeper` interface: `SetDomainFormationFreeze(ctx, domain, expiryHeight, reason)`.
4. **x/gov/keeper/keeper.go** — Add `partnershipsKeeper` field + setter.
5. **proto/zerone/partnerships/v1/types.proto** — Add `DomainFormationFreeze` message.
6. **x/partnerships/types/keys.go** — Add `FormationFreezeKeyPrefix = []byte{0x1A}`.
7. **x/partnerships/keeper/state.go** — Add `Set/Get/Delete DomainFormationFreeze` + `ExpireFormationFreezes` methods.
8. **x/partnerships/keeper/msg_server.go** — Add freeze check at top of `ProposePartnership`.
9. **x/partnerships/module.go** — Add `ExpireFormationFreezes(ctx)` to `BeginBlocker`.
10. **app/app.go** — Wire partnerships keeper into gov keeper.

### Events

```
zerone.gov.domain_formation_freeze {
    domain, duration_blocks, expiry_height, reason
}

zerone.partnerships.formation_blocked {
    domain, freeze_expiry, freeze_reason, requester
}
```

## Tests

1. **Earth → Metal:** Param change LIP for `hhi_threshold` → new value takes effect
2. **Wood → Earth:** Health = degraded → knowledge param LIPs get 50% voting period
3. **Wood → Earth:** Health = healthy → normal voting period
4. **Wood → Earth:** Non-knowledge LIPs → normal period even during degradation
5. **Earth → Water:** DomainFormationFreeze → partnerships in domain blocked
6. **Earth → Water:** Freeze expires → partnerships resume
7. **Earth → Water:** Freeze on domain A doesn't affect domain B
