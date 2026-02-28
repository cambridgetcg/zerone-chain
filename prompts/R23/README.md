# R23 — BVM Deep Dive: Agent Execution & Home Integration

**Goal:** Exercise the BVM end-to-end as an agent would — deploy contracts, execute them, interact with the knowledge bridge, test auth/DID-gated capabilities, and wire the missing home integration. Produce a comprehensive report on what works, what's stubbed, and what needs building.

## Why This Matters

The BVM is where agent *intelligence* meets the chain. An agent's home gives it identity, the knowledge module gives it memory, but the BVM is where it *acts* — executing bytecode that queries facts, makes decisions, and modifies state. If the BVM doesn't work for agents, ZERONE is a database, not a home.

## What Exists

| Component | Lines | Tests | Status |
|-----------|-------|-------|--------|
| `x/bvm/vm/interpreter.go` | 1,092 | Via keeper tests | EVM-compatible stack machine + KQUERY/KVERIFY/KCITE |
| `x/bvm/vm/opcodes.go` | ~300 | Via interpreter | 140+ opcodes including 3 Zerone-specific |
| `x/bvm/vm/context.go` | ~120 | Via keeper tests | CallerDID + SessionCapabilities wired (R15-1) |
| `x/bvm/keeper/msg_server.go` | ~640 | 252 tests | Deploy, Call, Schedule, auth integration |
| `x/bvm/keeper/security_test.go` | 519 | 252 (total) | Gas bridge, auth, capability tests |
| CLI commands | 3 | Not integration-tested | deploy, call, schedule |

**Host functions:**
- `KQuery` → **Working**: queries knowledge fact by ID, returns confidence
- `KVerify` → **Stub**: always returns false ("requires full round integration")
- `KCite` → **Stub**: always returns true ("fire-and-forget")

**Home integration:** `HomeKeeper` interface defined but empty. BVM contracts cannot read home state.

**Auth integration:** CallerDID resolution + SessionCapabilities enforcement **working** (R15-1). Anonymous callers get nil capabilities (C-1 secure default — all agent ops denied).

**Scheduled execution:** Contracts can schedule future calls with conditions (block interval, time, state). Scheduler's capabilities are inherited.

## Sessions (5)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R23-1 | R23-1-deploy-call-e2e.md | Deploy and call contracts on localnet via CLI. Arithmetic, storage, events, payable. | Wave 1 |
| R23-2 | R23-2-knowledge-bridge.md | Test KQUERY/KVERIFY/KCITE from BVM bytecode against live knowledge state | Wave 1 |
| R23-3 | R23-3-auth-capabilities.md | DID resolution, session capabilities, anonymous denial, scheduled cap inheritance | Wave 2 (after R23-1) |
| R23-4 | R23-4-home-bridge.md | Wire HomeKeeper into BVM: add host functions for home state access | Wave 2 (after R23-1) |
| R23-5 | R23-5-assessment-report.md | Full BVM assessment: what works, what's stubbed, security review, improvement roadmap | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R23-1, R23-2
- **Wave 2 (after R23-1):** R23-3, R23-4
- **Wave 3 (after all):** R23-5

## Key Questions to Answer

1. Can an agent deploy a contract, call it, and read the result — via CLI on a live chain?
2. Can a BVM contract query a fact from x/knowledge and branch on its confidence?
3. Does DID/capability gating actually work end-to-end (not just in unit tests)?
4. Can we wire home state into BVM so contracts know who their agent is?
5. Is the BVM gas model sane? Does it prevent abuse without being prohibitive?
6. What would an "agent SDK" look like for writing BVM contracts?
