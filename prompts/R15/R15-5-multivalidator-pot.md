# R15-5 — Multi-Validator PoT Verification

## Context

Zerone's Proof of Truth consensus has never been verified across multiple validators with the new binary. The individual pieces exist — vote extensions (ABCI++), VRF validator selection, commit/reveal rounds, knowledge keeper — but the full cycle hasn't been run end-to-end.

## Prerequisites

- R15-1 merged (auth→BVM bridge, so identity-aware execution works)
- R14-1 merged (`cmd/zeroned/` binary exists)
- `scripts/localnet.sh` available

## Task

### Step 1: Boot 4-Validator Localnet

```bash
# Use existing localnet script or adapt it
bash scripts/localnet.sh

# Or manual setup if localnet.sh needs the binary in specific location:
make build
# Copy binary to expected location
```

Verify: 4 nodes start, all peered, blocks producing.

### Step 2: Verify Vote Extensions Active

Vote extensions must be enabled from block 1 (set in genesis `abci.vote_extensions_enable_height`).

```bash
# Check that ExtendVote is being called
# Look for VRF data in block headers or events
curl -s http://localhost:26657/block?height=5 | jq '.result.block.last_commit'
```

Verify: vote extensions present in commits from block 2+.

### Step 3: Submit a Knowledge Claim

Use CLI to submit a claim that triggers a PoT verification round:

```bash
# Register a validator as a knowledge agent (if not auto-registered)
# Submit a claim
./build/zeroned tx knowledge submit-claim \
  --domain "test-domain" \
  --content-hash "$(echo -n 'test-claim-content' | sha256sum | cut -d' ' -f1)" \
  --logic-zone "propositional" \
  --from validator1 --keyring-backend test \
  --chain-id zerone-localnet-1 --fees 100uzrn -y
```

### Step 4: Observe PoT Round Lifecycle

A complete round should cycle through:

1. **Claim submitted** → round created
2. **Commit phase** → validators commit vote hashes
3. **Reveal phase** → validators reveal votes
4. **Verdict** → consensus reached, fact created or claim rejected

```bash
# Query active rounds
./build/zeroned query knowledge rounds --status active

# Watch for round events
./build/zeroned query txs --events 'zerone.knowledge.round_created' --limit 5

# Check fact creation
./build/zeroned query knowledge facts --domain test-domain
```

### Step 5: Verify Tier Progression

If the test runs long enough (or use shorter params):

```bash
# Check validator tiers
./build/zeroned query zerone-staking validators

# Verify tier 0 (Apprentice) → tier 1 (Verified) progression
# Requires: min stake + reputation threshold
```

### Step 6: Test Slashing (Optional)

If feasible in localnet:
- Stop one validator
- Wait for downtime threshold
- Verify slashing event
- Verify tier demotion if applicable

### Step 7: Verify VRF Selection

```bash
# Check VRF randomness in vote extensions
# Verify validator selection is weight-proportional over multiple rounds
```

### Step 8: Write Integration Test

Create or extend `tests/multivalidator/multivalidator_test.go`:

```go
func TestMultiValidator_PoTRoundComplete(t *testing.T) {
    // Setup 4-validator testnet (in-process or via exec)
    // Submit claim
    // Wait for round lifecycle (commit → reveal → verdict)
    // Assert: fact created with expected confidence
    // Assert: validators who participated got reputation
}

func TestMultiValidator_VoteExtensionsActive(t *testing.T) {
    // Verify vote extensions in block commits
}

func TestMultiValidator_VRFSelection(t *testing.T) {
    // Submit multiple claims
    // Verify different validators selected across rounds
}
```

### Report

```markdown
## Multi-Validator PoT Report

| Check | Status |
|-------|--------|
| 4 validators booted | ✅/❌ |
| All validators peered | ✅/❌ |
| Vote extensions active | ✅/❌ |
| Claim submitted | ✅/❌ |
| PoT round created | ✅/❌ |
| Commit phase observed | ✅/❌ |
| Reveal phase observed | ✅/❌ |
| Verdict reached | ✅/❌ |
| Fact created | ✅/❌ |
| VRF selection observed | ✅/❌ |
| Validator reputation updated | ✅/❌ |
```

## Verification

```bash
go test ./tests/multivalidator/... -count=1 -v -timeout 120s
```

## Commit Convention

```
test(R15-5): multi-validator PoT round lifecycle verification
test(R15-5): add VRF selection + vote extension integration tests
```
