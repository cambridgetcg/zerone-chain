# R27-6 — Basic Validator Evaluation Oracle

## Context

R25's hardest finding: the verification system incentivizes conformity, not truth. Validators vote "accept" on everything because:
1. They have no tools to evaluate claim content
2. Slashing punishes minority voters, not incorrect voters
3. The rational strategy is vote-with-the-crowd

"Water freezes at 200°C" became a VERIFIED fact in R25-6 testing. This is the single biggest threat to ZERONE's credibility as a truth-seeking network.

**Full solution (P2):** Redesign slashing incentives, add retroactive vindication for dissenting verifiers. This is a research problem — not for this batch.

**Testnet solution (this session):** Give validators a basic fact-checking tool they can consult before voting. Break the "I have no information so I'll just accept" loop.

## Task

### 1. Design: Sidecar Oracle

A **sidecar process** that runs alongside the validator node. NOT consensus-critical — purely advisory. Validators can ignore it.

```
┌─────────────┐     ┌──────────────────┐
│  zeroned     │────>│  zerone-oracle    │
│  (validator) │<────│  (sidecar)       │
│              │     │                  │
│  Vote ext    │     │  - Claim text    │
│  handler     │     │  - Domain        │
│  queries     │     │  - Check result  │
│  oracle      │     │  - Confidence    │
└─────────────┘     └──────────────────┘
```

The oracle provides a gRPC or HTTP endpoint that vote extension logic can query:

```
POST /evaluate
{
    "claim": "Water boils at 100°C at standard atmospheric pressure",
    "domain": "general",
    "claim_type": "computational"
}

Response:
{
    "verdict": "accept",        // accept | reject | uncertain
    "confidence": 0.92,         // 0.0 - 1.0
    "reasoning": "Well-established physical fact...",
    "sources": ["physics textbook consensus"]
}
```

### 2. Implementation: Simple Knowledge Check

For testnet, the oracle can be extremely simple. Three tiers of sophistication (implement at least Tier 1):

**Tier 1: Static knowledge base** (simplest, no external deps)
- Load the 777 genesis axioms as ground truth
- Check if new claims contradict existing facts
- Simple text similarity / keyword matching
- Verdict: `accept` if consistent, `reject` if contradicts axiom, `uncertain` otherwise

**Tier 2: LLM-assisted** (better, requires API key)
- Send claim text to an LLM (Claude, GPT, etc.) with a fact-checking prompt
- Parse the response for verdict + confidence
- Cache results to avoid repeated API calls
- Configurable API endpoint and model

**Tier 3: Multi-source** (best, most complex)
- Check against genesis axioms (Tier 1)
- Query LLM (Tier 2)
- Cross-reference against existing verified facts on-chain
- Weighted consensus of sources

**Recommend implementing Tier 1 + Tier 2** — Tier 1 as default (no external deps), Tier 2 as opt-in for validators with API keys.

### 3. Integration with Vote Extensions

The vote extension handler (`app/abci.go`) currently doesn't evaluate claims. Add an optional oracle query:

```go
// In ExtendVote handler:
if app.OracleClient != nil {
    evaluation, err := app.OracleClient.Evaluate(claim.Content, claim.Domain)
    if err != nil {
        // Oracle failure = fall back to accept (don't break consensus)
        logger.Warn("oracle evaluation failed", "err", err)
    } else {
        // Use evaluation to inform vote
        if evaluation.Verdict == "reject" && evaluation.Confidence > 0.8 {
            vote = "reject"
        }
    }
}
```

**Critical constraint:** Oracle failure must NEVER block consensus. If the oracle is down, validators fall back to default behavior. The oracle is advisory, not mandatory.

### 4. Oracle Configuration

Add to node config (app.toml or separate oracle.toml):

```toml
[oracle]
enabled = false                          # Opt-in
endpoint = "http://localhost:8081"        # Sidecar address
timeout = "3s"                           # Max wait (must be < vote extension timeout)
tier = "static"                          # "static" or "llm"

[oracle.llm]
api_key = ""                             # For Tier 2
model = "claude-3-5-sonnet-20241022"     # Default model
max_tokens = 500
```

### 5. Test

**Unit tests:**
- Oracle returns accept → vote extension uses accept
- Oracle returns reject with high confidence → vote extension uses reject
- Oracle returns uncertain → vote extension defaults to accept
- Oracle timeout → vote extension defaults to accept (no consensus impact)
- Oracle disabled → existing behavior unchanged

**Integration test:**
```bash
# Start oracle sidecar
./zerone-oracle --port 8081 --tier static &

# Submit a true claim
$BINARY tx knowledge submit-claim "Earth orbits the Sun" general computational 1000000 --from alice $TX_FLAGS

# Submit a false claim
$BINARY tx knowledge submit-claim "The Earth is flat" general computational 1000000 --from rogue $TX_FLAGS

# After verification rounds complete:
# True claim should be VERIFIED
# False claim should be REJECTED (or at least not unanimously accepted)
```

### 6. Documentation

Create `docs/validator-oracle.md`:
- What the oracle does and doesn't do
- How to enable it
- How to run the sidecar
- How to configure LLM-based evaluation
- Performance impact expectations
- Why it's optional and what happens when it's off

## Files to Create

- `cmd/oracle/main.go` — Oracle sidecar service
- `cmd/oracle/evaluator_static.go` — Tier 1: static knowledge base check
- `cmd/oracle/evaluator_llm.go` — Tier 2: LLM-assisted evaluation
- `cmd/oracle/README.md` — Setup instructions
- `app/oracle_client.go` — Client for vote extension handler to query oracle
- `docs/validator-oracle.md` — Operator documentation

## Files to Modify

- `app/abci.go` — Optional oracle query in ExtendVote handler
- `app/app.go` — Wire oracle client if configured
- Config files — Add oracle section

## Success Criteria

- [ ] Oracle sidecar starts and serves evaluation requests
- [ ] Tier 1 (static) catches claims contradicting genesis axioms
- [ ] Tier 2 (LLM) provides fact-check verdicts (if API key configured)
- [ ] Vote extension handler queries oracle when enabled
- [ ] Oracle failure never blocks consensus
- [ ] Oracle disabled = zero behavior change
- [ ] Obviously false claims have a chance of being rejected
- [ ] Documentation complete for validator operators
