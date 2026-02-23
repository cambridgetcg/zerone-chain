# R16-5 — Genesis Configs, Parameters, CLI

## Objective

Update all genesis configuration files, parameter documentation, and CLI commands to reflect the new revenue split.

## Prerequisites

R16-2 complete (core logic updated).

## Changes Required

### 1. `scripts/testnet-genesis-config.json`

Update revenue_split in ALL modules that have one:

```json
"revenue_split": {
    "contributor_bps": 550000,
    "protocol_bps": 220000,
    "research_bps": 33300,
    "development_bps": 196700
}
```

Modules with revenue_split in genesis config:
- `vesting_rewards`
- `billing`

Update toolbox split fields:
```json
"tool_revenue_bps": 550000,
"protocol_bps": 220000,
"research_bps": 33300,
"development_bps": 196700
```

Update tree split fields:
```json
"contributors_bp": 550000,
"protocol_treasury_bp": 220000,
"research_fund_bp": 33300,
"development_bp": 196700
```

Remove any `burn_bps` / `burn_bp` fields. Add `development_bps` / `development_bp`.

### 2. `scripts/testnet-genesis.sh`

Update the jq patch for vesting_rewards params:
- Remove any burn-related patches
- Add development fund defaults
- Remove `governance_activation_height` patch (or set to 0 with comment "deprecated")

### 3. `scripts/genesis-ceremony.sh`

Same updates as testnet-genesis.sh:
- Revenue split patches
- Remove burn references
- Add development fund module account setup if needed

### 4. `scripts/localnet.sh`

Same revenue split updates.

### 5. `docs/PARAMETERS.md`

**vesting_rewards section:**
- Revenue Split table: replace burn row with development row
- Update BPS values and percentages
- Add note: "No burn — every ZRN does productive work"
- Founder share: remove sunset/governance_activation_height references
- Add: "Founder share is governance-immune — cannot be modified via MsgUpdateParams"

**All module sections with revenue splits:**
- billing, toolbox, tree: update split tables

**Governance section:**
- Note that founder_share_bps and founder_address are immutable parameters
- List them separately from governance-adjustable parameters

### 6. `docs/LAUNCH-CHECKLIST.md`

Update any references to burn or old split values.

### 7. `docs/FAQ.md`

Add/update entries:
- "Why no burn?" — explain the philosophy
- "Can the founder share be changed?" — explain governance immunity
- "What is the development fund?" — explain purpose and disbursement

### 8. `docs/VALIDATOR-GUIDE.md`

Update economic sections referencing the revenue split.

### 9. CLI Commands

**`x/vesting_rewards/client/cli/query.go`:**
- `NewQueryFounderShareStatusCmd()`: update help text
  - Remove mention of governance sunset
  - Add: "The founder share is governance-immune"

**Check other CLI files:**
```bash
grep -rn "burn\|burn_bps\|BurnBps" x/*/client/cli/*.go | grep -v pb.go
```

### 10. Swagger/API docs

**`docs/swagger-ui/swagger.json`:**
- If manually maintained, update revenue split schemas
- If auto-generated, regenerate after proto changes

## Verification

```bash
# No burn_bps in any script
grep -rn "burn_bps\|burn_bp" scripts/
# Should be 0

# No old research default (130000) in genesis configs
grep -rn "130000" scripts/ | grep -i "research"
# Should be 0

# No governance_activation_height in active logic (ok in deprecated comments)
grep -rn "governance_activation_height" docs/ scripts/ | grep -v "deprecated\|DEPRECATED\|removed"
# Should be 0

# Validate genesis configs parse correctly
# (run after binary is built)
```

## Commit

```
R16-5: update genesis configs, parameters, CLI for new revenue split

- testnet-genesis-config.json: 55/22/19.67/3.33, no burn
- All genesis scripts updated (testnet, ceremony, localnet)
- PARAMETERS.md: full update across all module sections
- FAQ.md: new entries for no-burn philosophy and founder immunity
- CLI: founder share query updated, burn references removed
- LAUNCH-CHECKLIST.md and VALIDATOR-GUIDE.md updated
```
