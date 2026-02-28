# R27-7 — Testnet Launch Checklist + Go/No-Go

## Context

This is the final session before testnet launch. It synthesizes all R27 results, verifies everything is in place, and makes the call.

**This session depends on R27-1 through R27-6 being complete.**

## Task

### 1. Verify All R27 Sessions

Review outputs from each session and confirm:

| Session | Deliverable | Status |
|---------|------------|--------|
| R27-1 | Tree CLI complete (29/29 commands) | ? |
| R27-2 | Evidence CLI + hash format fix | ? |
| R27-3 | E2E full loop passes all checkpoints | ? |
| R27-4 | Testnet genesis validates + validator guide | ? |
| R27-5 | Faucet functional + token economics documented | ? |
| R27-6 | Oracle sidecar + vote extension integration | ? |

### 2. Run the Full Test Suite

```bash
cd ~/Desktop/zerone
go test ./... -count=1 2>&1 | tee /tmp/test-results.txt
echo "=== SUMMARY ==="
grep -E "^(ok|FAIL|---)" /tmp/test-results.txt | sort
echo "=== FAILURES ==="
grep "^FAIL" /tmp/test-results.txt
```

**Must be zero failures.**

### 3. Build Release Binaries

```bash
# Build for target platforms
make build-linux-amd64
make build-linux-arm64
make build-darwin-amd64
make build-darwin-arm64

# Or if using the cross-compilation from R24:
scripts/build-release.sh

# Verify binaries work
./build/zeroned version
```

### 4. Genesis File Final Check

```bash
# Validate
zeroned genesis validate networks/zerone-testnet-1/genesis.json

# Start from genesis (single node test)
zeroned init test-launch --chain-id zerone-testnet-1 --home /tmp/launch-test
cp networks/zerone-testnet-1/genesis.json /tmp/launch-test/config/genesis.json
zeroned start --home /tmp/launch-test &
sleep 15
zeroned status --node tcp://localhost:26657
kill %1
```

### 5. Documentation Completeness

Verify these docs exist and are accurate:

- [ ] `docs/testnet-validator-guide.md` — Step-by-step validator onboarding
- [ ] `docs/testnet-economics.md` — Token distribution and economics
- [ ] `docs/validator-oracle.md` — Oracle sidecar setup
- [ ] `networks/zerone-testnet-1/README.md` — Network info, peers, genesis
- [ ] `README.md` — Updated with testnet information

### 6. Infrastructure Readiness

- [ ] Seed node address known and documented
- [ ] Faucet deployable (or deployed)
- [ ] Genesis file hosted (Codeberg or direct download)
- [ ] At least 2 initial validators ready (Yu's machines)

### 7. Security Review (Quick Pass)

Not a full audit, but check:
- [ ] No hardcoded private keys in codebase
- [ ] No test mnemonics in genesis
- [ ] Faucet rate limiting functional
- [ ] AnteHandler chain complete (all decorators in order)
- [ ] Module account permissions correct (only mint where needed)

### 8. Go/No-Go Decision

Fill in this matrix:

| Category | Ready? | Blocker? | Notes |
|----------|--------|----------|-------|
| Core loop (claim→verify→reward) | | | |
| Cross-module wiring | | | |
| CLI completeness | | | |
| Genesis configuration | | | |
| Faucet | | | |
| Validator oracle | | | |
| Documentation | | | |
| Test suite green | | | |
| Release binaries | | | |
| Infrastructure | | | |

**Decision:**
- **LAUNCH** — All critical items ready, no blockers
- **LAUNCH WITH CAVEATS** — Minor issues documented, can proceed
- **NOT YET** — List specific blockers and estimated fix time

### 9. Launch Plan

If GO:

1. **Day 0:** Deploy seed node with genesis, start faucet
2. **Day 0:** Announce on Codeberg + any channels
3. **Day 1:** First external validator joins
4. **Day 1-3:** Monitor chain health, faucet distribution
5. **Day 3-7:** First claims submitted, verification rounds tested
6. **Week 2:** First partnerships formed, research submitted
7. **Ongoing:** Monitor, fix issues, iterate

### 10. Write Launch Report

Create `docs/testnet-launch-report.md`:
- R25-R27 journey summary (assessment → wiring → launch readiness)
- Module status matrix (all 32 modules, readiness score)
- Known limitations for testnet
- What comes after testnet (mainnet blockers)
- The go/no-go decision and reasoning

## Files to Create

- `docs/testnet-launch-report.md` — Comprehensive launch report

## Success Criteria

- [ ] All R27 sessions reviewed
- [ ] Full test suite green
- [ ] Release binaries built
- [ ] Genesis validated
- [ ] Documentation complete
- [ ] Go/no-go decision made with reasoning
- [ ] Launch plan documented (if GO)
