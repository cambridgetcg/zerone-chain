# R14-2 — Port BVM Test Coverage

## Context

The BVM (Belief Virtual Machine) is the highest-risk module — it executes arbitrary agent-submitted programs. The prototype has 438 tests; zerone has 72. Gap: **366 tests**.

The BVM handles: contract lifecycle, opcode execution, gas bridging, scheduled execution, host functions, and the interpreter loop. Under-testing here means potential consensus-breaking bugs.

## Task

Port BVM tests from `legible-money/x/bvm/keeper/` to `zerone/x/bvm/keeper/`, adapting for:

- Module path: `github.com/zerone-chain/zerone/x/bvm`
- Denom: `uzrn` (not `ulgm`)
- Address prefix: `zrn` (not `lgm`)
- BPS scale: 1,000,000 everywhere
- Proto-generated types (not hand-written JSON)

### Categories to Port (priority order)

1. **Interpreter tests** — opcode execution, stack operations, memory, control flow
2. **Contract lifecycle** — deploy, execute, pause, resume, destroy
3. **Gas bridge** — Cosmos gas ↔ BVM gas conversion, metering, limits
4. **Host functions** — knowledge queries, bank operations, staking queries from BVM
5. **Scheduled execution** — cron-like BVM program scheduling
6. **Security tests** — stack overflow, infinite loops, memory bombs, reentrancy guards
7. **Edge cases** — empty programs, max program size, invalid opcodes

### Source Files (prototype)

Check all `*_test.go` files in:
- `legible-money/x/bvm/keeper/`
- `legible-money/x/bvm/types/`

### Target

BVM test count ≥ 350 (currently 72, need ~280 new tests).

## Approach

- Read existing zerone BVM tests first — understand the test harness pattern
- Port test-by-test, adapting types and assertions
- Do NOT copy-paste blindly — zerone's BVM may have structural differences from prototype
- Run `go test ./x/bvm/... -count=1 -v` after each batch of ports

## Verification

```bash
go test ./x/bvm/... -count=1 -v
go vet ./x/bvm/...
# Count: grep -c "func Test" x/bvm/keeper/*_test.go x/bvm/types/*_test.go
```

## Commit Convention

```
test(R14-2): port BVM interpreter tests from prototype
test(R14-2): port BVM contract lifecycle tests
test(R14-2): port BVM gas bridge + security tests
```
