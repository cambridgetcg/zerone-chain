# R24 — Agent Onboarding: Identity, Validation, and Cloud Deployment

**Goal:** Test the full agent onboarding flow — from registering an identity on-chain to becoming a validator to deploying on cloud infrastructure. Answer the question: can an external agent actually join the ZERONE network?

## Why This Matters

Everything built so far (home, BVM, knowledge, PoT) assumes agents are already on the network. Nobody has tested the *entry path* — the sequence of steps an agent (or agent-human pair) needs to follow to go from zero to participating. This is the operator experience that will make or break external adoption.

## What Exists

| Component | Lines | Status |
|-----------|-------|--------|
| `x/auth` — DID registration, sessions, key rotation, recovery | Full module | 12 RPCs, untested on live chain as a flow |
| `x/staking` — 4-tier validator system | Full module | Tested in localnet, not as external join |
| `scripts/join-testnet.sh` | 352 lines | Cosmovisor + systemd, untested end-to-end |
| `scripts/configure-node.sh` | 278 lines | 4 modes (validator/fullnode/seed/archive) |
| `docs/VALIDATOR-GUIDE.md` | 481 lines | Comprehensive but unvalidated |
| `docs/infrastructure/PRODUCTION-STACK.md` | 250 lines | Cloud architecture designed, no Dockerfile/automation |
| `docs/LAUNCH-CHECKLIST.md` | 147 lines | Pre-launch checklist |
| Makefile | `build`, `install`, `cosmovisor-init` | No cross-compile, no Docker |

**Key gaps:**
- No Dockerfile (can't deploy without building from source on every machine)
- No cross-compilation targets (GOOS=linux)
- `join-testnet.sh` never run against an actual testnet
- DID registration → validator registration → home creation flow never tested as a sequence
- No agent-specific onboarding (how does an AI agent, not a human operator, join?)

## Sessions (6)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R24-1 | R24-1-identity-flow.md | Register account (DID), create session keys, create home — the agent identity stack | Wave 1 |
| R24-2 | R24-2-validator-lifecycle.md | Register validator, delegate, tier progression, unjail, redelegate — full validator lifecycle | Wave 1 |
| R24-3 | R24-3-join-testnet.md | Run join-testnet.sh against running localnet as an "external" node joining | Wave 2 (after R24-1) |
| R24-4 | R24-4-docker-crosscompile.md | Create Dockerfile, cross-compile targets, reproducible builds | Wave 2 |
| R24-5 | R24-5-cloud-deploy.md | Deploy a validator to a real VPS, validate production stack docs | Wave 3 (after R24-3, R24-4) |
| R24-6 | R24-6-agent-onboarding-report.md | Synthesise findings: what works, what's missing, operator UX improvements | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R24-1, R24-2
- **Wave 2 (parallel):** R24-3, R24-4
- **Wave 3 (after all):** R24-5, R24-6

## The Agent Onboarding Journey

The complete sequence an agent-human pair should follow:

```
1. Build/obtain zeroned binary (Dockerfile or cross-compile)
2. Initialize node: zeroned init
3. Join testnet: scripts/join-testnet.sh
4. Wait for sync
5. Register account with DID: zeroned tx auth register-account
6. Create home: zeroned tx home create-home
7. Register keys on home: zeroned tx home register-key
8. Fund account (faucet or transfer)
9. Register as validator: zeroned tx zerone-staking register-validator
10. Self-delegate: zeroned tx zerone-staking delegate
11. Wait for tier promotion
12. Deploy BVM contract: zeroned tx bvm deploy-contract
13. Participate in PoT rounds (automatic via vote extensions)
```

Each session tests a portion of this journey on a live chain.
