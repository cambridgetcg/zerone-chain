# R35-1 — Full Test Pipeline Run

## Objective

Run every test in the codebase. Fix every failure. This is the first time ALL test layers run together as a qualification gate.

## Tasks

### 1. Create the pipeline script

Create `scripts/full-pipeline.sh`:

```bash
#!/bin/bash
set -euo pipefail

echo "═══ ZERONE Test Pipeline ═══"
FAILURES=0

echo "─── 1/10 Unit Tests ───"
go test ./x/... ./app/... -short -count=1 || ((FAILURES++))

echo "─── 2/10 Integration Tests ───"
go test ./tests/integration/... -count=1 || ((FAILURES++))

echo "─── 3/10 Cross-Stack Tests ───"
go test ./tests/cross_stack/... -count=1 || ((FAILURES++))

echo "─── 4/10 Simulation (200 blocks) ───"
go test -run TestFullAppSimulation -NumBlocks=200 -BlockSize=30 -timeout 30m ./app/... || ((FAILURES++))

echo "─── 5/10 E2E Tests ───"
go test -timeout 20m ./tests/e2e/... || ((FAILURES++))

echo "─── 6/10 Adversarial Simulation ───"
go test -timeout 15m ./tests/simulation/... || ((FAILURES++))

echo "─── 7/10 Stress Tests ───"
go test -run TestStress -timeout 15m ./tests/e2e/... || ((FAILURES++))

echo "─── 8/10 IBC Tests ───"
go test -run TestIBC -timeout 15m ./tests/e2e/... || ((FAILURES++))

echo "─── 9/10 Upgrade Test ───"
go test -run TestUpgrade -timeout 15m ./tests/e2e/... || ((FAILURES++))

echo "─── 10/10 Genesis Round-Trip ───"
go test -run TestGenesis_ExportImport -timeout 10m ./tests/e2e/... || ((FAILURES++))

echo ""
if [ $FAILURES -eq 0 ]; then
    echo "✅ ALL 10 STAGES PASSED — ready for testnet"
else
    echo "❌ $FAILURES STAGES FAILED — not ready"
    exit 1
fi
```

### 2. Fix every failure

This session's primary work is fixing whatever breaks. Expected issues:
- Test interdependencies (state leaking between tests)
- Docker resource limits (E2E tests need cleanup)
- Timing-sensitive tests (block production is non-deterministic)
- Mock drift (unit test mocks out of date with R31 changes)

### 3. Makefile target

```makefile
pipeline: docker-build-local
	@./scripts/full-pipeline.sh

pipeline-fast:
	@go test ./... -short -count=1
```

### 4. CI integration

```yaml
  pipeline:
    name: "Full Pipeline"
    runs-on: ubuntu-latest
    timeout-minutes: 90
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - name: Run full pipeline
        run: make pipeline
```

## Acceptance Criteria

- [ ] All 10 pipeline stages pass
- [ ] Zero test failures
- [ ] Pipeline completes in < 60 minutes
- [ ] Script is idempotent (can run multiple times)
- [ ] CI job configured for main branch pushes
