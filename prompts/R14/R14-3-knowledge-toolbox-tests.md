# R14-3 — Port Knowledge + Toolbox Test Coverage

## Context

Two platform-critical modules are under-tested:

| Module | Zerone | Prototype | Gap |
|--------|--------|-----------|-----|
| knowledge | 304 | 482 | 178 |
| toolbox | 128 | 268 | 140 |

**Knowledge** is the PoT core — fact management, claim lifecycle, verification rounds, commit/reveal, VRF selection, domain scoring. Under-testing here risks consensus failures.

**Toolbox** is the tool platform — registration, revenue distribution, trust scoring, composability DAG, dynamic pricing, Purpose Prompter integration. Under-testing risks economic exploits.

## Task

### Knowledge (target: ≥ 420 tests)

Port from `legible-money/x/knowledge/keeper/` and `legible-money/x/knowledge/types/`:

1. **Verification round lifecycle** — commit/reveal timing, round finalization, slashing for no-shows
2. **VRF validator selection** — weight calculation, selection fairness, edge cases
3. **Fact management** — creation, update, expiry, confidence decay
4. **Claim types** — all 7 claim types with varying confidence models
5. **Domain scoring** — activity tracking, reputation accumulation
6. **Axiom handling** — genesis axiom injection, axiom validation, DAG integrity
7. **Security** — double-commit, reveal before commit, replay attacks

### Toolbox (target: ≥ 220 tests)

Port from `legible-money/x/toolbox/keeper/`:

1. **Tool registration** — register, update, deprecate, reactivate
2. **Revenue distribution** — creator shares, contributor splits, cascade through dependency chains
3. **Trust engine** — 5-component scoring, decay, boosting
4. **Composability** — DAG validation, circular dependency detection, depth limits
5. **Dynamic pricing** — demand tracking, surge pricing, free tier, USD-stable base
6. **Purpose Prompter** — seed knowledge, scout, analyzer (if ported to Go)
7. **Security** — fee manipulation, trust gaming, revenue theft

### Adapting Tests

- Module path: `github.com/zerone-chain/zerone/x/knowledge` / `x/toolbox`
- Denom: `uzrn`
- BPS scale: 1,000,000
- Proto-generated types
- Read existing zerone test patterns first — match the harness

## Verification

```bash
go test ./x/knowledge/... -count=1 -v
go test ./x/toolbox/... -count=1 -v
go vet ./x/knowledge/... ./x/toolbox/...
```

## Commit Convention

```
test(R14-3): port knowledge verification round tests
test(R14-3): port knowledge VRF + security tests
test(R14-3): port toolbox revenue distribution tests
test(R14-3): port toolbox trust engine + composability tests
```
