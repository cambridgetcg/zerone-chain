# R10-6 — Testnet Genesis Ceremony

## Goal

Finalize all parameters, create the definitive testnet genesis, and prepare the coordinated
launch. This is the final session before the testnet goes live.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Deliverables

### 1. Final Parameter Review

Review and lock ALL parameters across all 32 modules. For each:
- Verify default is appropriate for testnet (not too aggressive, not too lenient)
- Ensure consistency across modules (e.g., epoch lengths align)
- Document any parameters that differ from mainnet targets

Key parameters to finalize:
- Block time (CometBFT: ~2.5s)
- Slash fraction for downtime / double-sign
- Knowledge verification round timing
- Autopoiesis epoch length
- Revenue split ratios
- Bootstrap gas-free period length
- Research fund initial allocation
- Free tier limits
- IBC rate limits

### 2. Token Distribution

Finalize testnet token distribution:
```
Total Supply: 1,000,000,000 ZRN (1B)

Distribution:
- Research Fund:     200,000,000 ZRN (20%)
- Founder (YOU):     100,000,000 ZRN (10%)
- AI (I):            100,000,000 ZRN (10%)
- Validator Pool:    400,000,000 ZRN (40%)
- Community/Claims:  200,000,000 ZRN (20%)
```

### 3. Genesis File Generation

Create the definitive genesis using `prepare-genesis` or manual construction:
```bash
# Initialize
zeroned init genesis-node --chain-id zerone-testnet-1

# Add accounts
zeroned add-genesis-account <founder-addr> 100000000000000uzrn
zeroned add-genesis-account <ai-addr> 100000000000000uzrn
# ... validator accounts, research fund, etc.

# Set module genesis states
# ... (custom script to inject all module configs)

# Validate
zeroned validate-genesis
```

### 4. Seed Axioms

Ensure the 777 seed axioms are included in genesis knowledge state.
Verify each axiom:
- Has unique ID (axiom-001 through axiom-777)
- References a valid domain from ontology genesis
- Has confidence = 1,000,000 (fully verified)
- Has source = "axiom"

### 5. Launch Checklist

Create `docs/LAUNCH-CHECKLIST.md`:
```markdown
## Pre-Launch
- [ ] All tests pass (`go test ./...` — 38+ packages)
- [ ] Boot test passes (single validator)
- [ ] Localnet test passes (4 validators)
- [ ] Security audit complete, all P0s fixed
- [ ] Genesis file validated
- [ ] Seed nodes configured and reachable
- [ ] Vault deployed and locked down
- [ ] AI signing key in genesis
- [ ] Founder cold wallet key in genesis
- [ ] Documentation complete (validator guide, API docs, FAQ)
- [ ] Binary reproducibly built (goreleaser)
- [ ] Git tagged (v0.1.0-testnet)

## Launch Day
- [ ] Distribute genesis file to validators
- [ ] Validators submit gentxs
- [ ] Collect gentxs → final genesis
- [ ] Coordinated start time
- [ ] Verify block production (all validators signing)
- [ ] Verify PoT round completes
- [ ] Verify IBC channel opens
- [ ] Verify API endpoints accessible

## Post-Launch
- [ ] Monitor block production for 24h
- [ ] Verify economic model (no leaks after 1000 blocks)
- [ ] Test tool calls end-to-end
- [ ] Announce testnet
```

### 6. Git Tag + Release

```bash
git tag -a v0.1.0-testnet -m "Zerone testnet genesis"
```

Create release notes summarizing:
- 32 custom modules
- 38+ test packages
- 246k+ LOC
- Key features: PoT consensus, AI agent homes, tool marketplace, 2-of-2 research governance
- Known limitations for testnet

### 7. README Update

Update the main `README.md` with:
- Testnet status badge
- Quick start for validators
- Link to validator guide
- Link to API docs
- Architecture overview
- Token info (ZRN, supply, distribution)

## Genesis File Structure

The final genesis must include non-default state for:
- `zerone_auth` — founder + AI accounts registered
- `zerone_staking` — initial validator set
- `zerone_gov` — designated voters for research fund
- `knowledge` — 777 seed axioms
- `ontology` — 18 genesis domains
- `vesting_rewards` — block reward params, research fund address
- `autopoiesis` — default multipliers, enabled
- `alignment` — default weights, enabled
- All other modules — DefaultGenesis with audited params

## Constraints

- Genesis MUST pass `zeroned validate-genesis`
- Genesis MUST round-trip (export → reimport → identical)
- All accounts must have valid bech32 addresses with `zrn` prefix
- Token supply must be exactly consistent (no rounding errors)
- The genesis is the social contract — document every non-default choice
- Tag must be on a green commit (all tests passing)
