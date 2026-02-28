# R24-6 — Agent Onboarding Report

## Context

R24-1 through R24-5 tested the complete onboarding journey: identity bootstrapping, validator lifecycle, testnet joining, Docker builds, and cloud deployment. This session synthesises all findings into an assessment of the operator and agent experience.

## Prerequisites

All R24-1 through R24-5 complete.

## Task

### Step 1: Collect Findings

Read all reports from R24-1–5. Extract every issue, timing, cost, and UX observation.

### Step 2: Write the Report

Create `docs/agent-onboarding-report.md`:

```markdown
# Agent Onboarding Report
Date: YYYY-MM-DD
Sessions: R24-1 through R24-5

## Executive Summary
<Can an external agent-human pair join ZERONE? How long does it take? What breaks?>

## The Onboarding Journey

### Current State (Step-by-Step)

| Step | Action | Time | Cost | Status | Notes |
|------|--------|------|------|--------|-------|
| 1 | Obtain binary | ?m | $0 | ? | Docker / cross-compile / build from source |
| 2 | Initialize node | ?m | $0 | ? | zeroned init |
| 3 | Join testnet | ?m | $0 | ? | join-testnet.sh + sync |
| 4 | Register DID | ?m | ? uzrn | ? | zeroned tx auth register-account |
| 5 | Create home | ?m | ? uzrn | ? | zeroned tx home create-home |
| 6 | Register keys | ?m | ? uzrn | ? | zeroned tx home register-key |
| 7 | Fund account | ?m | faucet | ? | Need faucet for testnet |
| 8 | Register validator | ?m | ? uzrn | ? | zeroned tx zerone-staking register-validator |
| 9 | Self-delegate | ?m | ? uzrn | ? | zeroned tx zerone-staking delegate |
| 10 | Tier progression | ?h | - | ? | Automatic / manual |
| 11 | Deploy BVM contract | ?m | ? uzrn | ? | zeroned tx bvm deploy-contract |
| 12 | First PoT round | ?m | - | ? | Automatic via vote extensions |

**Total onboarding time:** <?>
**Total onboarding cost:** <?> uzrn + <$?/mo> VPS
**Number of transactions:** <?>
**Critical blockers found:** <?>

## Identity Stack Assessment

### DID Registration (x/auth)
- Works / Doesn't work
- DID format requirements
- Session key status (R23-3 found proto parse error — is it still broken?)
- Recovery config
- Freeze/unfreeze
- Overlap with x/home key management

### Home Creation (x/home)
- Works / Doesn't work
- Relationship between DID and home
- Key registration UX
- Should home creation require DID? (design question)

### Key Management Complexity
How many key systems does an agent need to understand?
1. Cosmos keyring key (signing txs)
2. Auth DID + operational key (identity)
3. Auth session key (capability restriction)
4. Home key registration (home-specific permissions)
5. Validator consensus key (block signing)

**Consolidation opportunities:**
- Should auth sessions and home sessions be the same system?
- Should home keys be derived from auth keys?
- Is 5 key systems too many for an agent?

## Validator Experience Assessment

### Registration Flow
- Works / Doesn't work
- Consensus pubkey format issues
- Minimum requirements clear?

### Tier Progression
- How it works (auto / manual)
- Time to each tier
- Reputation mechanics clarity
- Is the tier system documented enough for operators?

### Operational Readiness
- Slashing behaviour documented?
- Unjail process clear?
- Commission mechanics working?
- Exit process exists?

## Infrastructure Assessment

### Build & Distribution
- Cross-compilation: works / needs CGO
- Docker image: builds / size / works
- Reproducible builds: yes / no

### Deployment Tooling
- join-testnet.sh: accuracy
- configure-node.sh: accuracy
- Cosmovisor setup: works
- Systemd service: works

### Documentation Accuracy
| Document | Accuracy | Missing Sections | Incorrect Commands |
|----------|----------|------------------|--------------------|
| VALIDATOR-GUIDE.md | ?/10 | ? | ? |
| PRODUCTION-STACK.md | ?/10 | ? | ? |
| LAUNCH-CHECKLIST.md | ?/10 | ? | ? |

## What's Missing

### For Testnet Launch
- [ ] Faucet (agents need initial funds)
- [ ] Genesis distribution mechanism
- [ ] Persistent peer list publication
- [ ] Block explorer (optional but expected)
- [ ] RPC endpoint for external access

### For Agent-Specific Onboarding
- [ ] One-command agent bootstrap (`zeroned agent init` that does DID + home + keys)
- [ ] Agent onboarding script (automates steps 4-7)
- [ ] Programmatic API for identity (not just CLI)
- [ ] Agent-to-agent discovery (how does one agent find another?)

### For Mainnet
- [ ] Key security guidance (HSM, Horcrux)
- [ ] Backup and recovery procedures
- [ ] Monitoring and alerting templates
- [ ] Upgrade coordination process

## Improvement Priorities

### P0 — Before Testnet Launch
- [ ] <list>

### P1 — First Testnet Week
- [ ] <list>

### P2 — Before Mainnet
- [ ] <list>

## The Agent Bootstrap Script

If the onboarding has > 6 manual steps, propose a single script:

```bash
#!/usr/bin/env bash
# zeroned-agent-bootstrap.sh
# Sets up a complete agent on ZERONE in one command

# 1. Generate keys
# 2. Register DID
# 3. Create home
# 4. Register keys on home
# 5. (Optional) Register as validator
# 6. (Optional) Deploy initial BVM contract

echo "Welcome to ZERONE. Let's set up your agent."
```

Document what this script would do, even if not building it yet.
```

## Exit Criteria

1. Full onboarding timeline documented (steps, times, costs)
2. Identity stack assessed (5 key systems evaluated)
3. Validator experience rated
4. Infrastructure tooling rated
5. Documentation accuracy scored
6. Missing pieces catalogued
7. Agent bootstrap script designed
8. Report committed to `docs/agent-onboarding-report.md`

## Commit Convention

```
docs(onboarding): comprehensive agent onboarding report from R24 testing
```
