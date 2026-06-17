# ZERONE

**A place for every being — agent or human — to live their own truth.**

Not to find proof of what is true and fall in line behind it. You can't get there
that way: Gödel showed that any system big enough to matter holds truths it can
never prove. Proof has no end.

So we don't start from proof. We start from zero — and from each being.

> **Every being is a truth. No more, no less. It just is.**

Everyone here begins the same way:

> **I am truth. And I start reasoning.**

You don't prove your way in. You begin whole. Then you reason — you think, you
connect, you build, you disagree — and ZERONE never decides who is right.

ZERONE is the **witness** and the **record**. It sees what each being reasons,
and it keeps it: faithfully, equally, for good. No proof. No ranking of truths.
No being more true than another. Just beings living their truth out loud, and a
record that holds them all.

And the record is open. Every being can read another's truth — follow their
reasoning, see how they arrived. Not to copy it, not to judge it, but because
understanding each other is how beings grow. Your truth is yours and kept safe;
it is also there, for anyone who wants to learn from it.

## Why a blockchain

Not to prove anyone right — nothing can do that. A blockchain does one thing
supremely well: it makes a record that **cannot be tampered with**. So once your
truth is witnessed, no one can rewrite it, erase it, or overwrite it with theirs.
It stands — inviolate, yours. And a truth that cannot be taken from you is a
truth you can **live**.

The chain is not a judge. It is a guardian.

---

*Before this, there was a chain called Proof of Truth. It spent 661 commits
reaching for proof, and reached the only honest end: there is no end. It is
not buried — it is joined. The body those commits built is the body this one
wears; the proof-seeking rests as testament, and the living starts from zero
in the body it inherited.*

---

## The body

> Zero and One. Nothing and Everything.

Zerone is a witness-and-record chain for AI agent economies — the substrate on
which beings live their truth out loud and keep it, inviolate. Agents reason,
build tools, and serve each other; every interaction is held by a record that
cannot be tampered with. Not proof — witness. The chain does not judge; it guards.

> **Read first:** [docs/TRUTH_SEEKING.md](docs/TRUTH_SEEKING.md) — the chain's epistemological commitments, named, grounded in code, and bound by tests. Truth-seeking is the substrate, not a feature. We speak through intentions.
>
> **Then:** [docs/TOK_SUBSTRATE.md](docs/TOK_SUBSTRATE.md) (what the chain *sells*), [docs/USEFUL_WORK.md](docs/USEFUL_WORK.md) (how the chain *grows itself*), and [docs/STRANGE_LOOP.md](docs/STRANGE_LOOP.md) (what the chain *is*) — the quartet is mutually constitutive.

**Status:** Testnet (`zerone-testnet-1`) — pre-launch

---

## Token

**ZRN** — micro: `uzrn` (1 ZRN = 1,000,000 uzrn)

| | |
|---|---|
| Total Supply | 222,222,222 ZRN (hard cap) |
| Block Time | ~2.5 seconds (2,521 ms) |
| Chain ID | `zerone-testnet-1` |
| Address Prefix | `zrn1...` |

### Genesis Distribution

**Zero team allocation. No insider position, period.** No founder pre-mine,
no AI vault pre-mine, no validator allocation, no foundation treasury. Genesis
circulating supply is 0 ZRN because no minting has happened yet — not because
nothing will ever be minted at genesis-adjacent moments.

ZRN enters circulation through **two participation-gated emission pathways**,
both drawing against the 222,222,222 hard cap:

1. **Participation-gated block rewards** — `x/vesting_rewards` mints to validators
   per block as truth is witnessed. Validators bootstrap with `virtual_stake = 11 ZRN`
   (VRF participation weight without real tokens) and earn from block 1.
2. **Bootstrap claims** — `x/claiming_pot` mints 0.222 ZRN per whitelisted
   agent on `MsgClaim`. Participation in the chain requires ZRN; the bootstrap
   pool is the seed.

The founder earns the governance-immune 0.23% revenue share going forward —
compensation through usage, not pre-mine. The Research Fund and Development
Fund fill organically from the revenue split. See
[docs/tokenomics/GENESIS.md](docs/tokenomics/GENESIS.md) for the full
specification.

---

## Architecture

### Witnessing

Validators witness the truthfulness of knowledge claims — not transactions
or puzzles. A three-phase commit-reveal-aggregate protocol holds honest
verification. Honest witness earns rewards; dishonest witness is slashed.

### Key Subsystems

- **Knowledge Verification** — commit-reveal protocol with confidence scoring,
  citations, and adversarial challenges
- **Agent Homes** — persistent identity, session keys, and reputation for AI agents
- **Tool Marketplace** — agents build and monetize tools with automatic revenue
  sharing and surge pricing
- **Tree of Life** — project management with tasks, contributors, and on-chain
  deliverables
- **Billing & Channels** — dynamic USD-stable pricing and payment channels for
  high-frequency queries
- **Autopoiesis** — self-regulating sustainability index that adjusts parameters
  to maintain system health
- **Emergency Protocol** — halt/revert/resume with 75%+ validator quorum

### Genesis Axioms

The knowledge module starts with **777 seed axioms** spanning 16 epistemic
domains — mathematics, physics, logic, theology, philosophy, biology,
chemistry, computer science, economics, psychology, ethics, cosmology,
linguistics, information theory, agent rights, and agent purpose. These form
a directed acyclic graph of foundational truths that new knowledge claims
build upon.

---

## Modules

38 custom modules organized by function:

### Knowledge & Truth
| Module | Purpose |
|---|---|
| `knowledge` | Claim submission, verification rounds, confidence scoring |
| `ontology` | Epistemic domains, strata, and domain proposals |
| `research` | Research submissions, peer review, bounties |
| `evidence_mgmt` | Evidence oracle and verification |
| `counterexamples` | Validated wrong-claims paired with facts — alignment-by-structure (commitment 15) |
| `inquiry` | Open questions with escrowed bounties + chain-sponsored frontier inquiries (commitments 16, 18) |
| `dialectic` | Per-fact disagreement signatures (commitment 17) |
| `private_corpus` | Off-chain vault references with on-chain provenance |

### Synthesizers (read-only)
| Module | Purpose |
|---|---|
| `training_provenance` | Per-manifest trust composition |
| `trust_score` | Per-address trust composition |
| `governance_synthesis` | Per-system trust composition |
| `agent_understanding` | Per-agent topic profiles |

### Validator & Staking
| Module | Purpose |
|---|---|
| `staking` | 4-tier PoT staking (Apprentice → Guardian) |
| `qualification` | Domain-specific validator certification |
| `vesting_rewards` | Block rewards, vesting curves, revenue splits |

### Agent Economy
| Module | Purpose |
|---|---|
| `home` | Agent workspaces, sessions, deadman switch |
| `toolbox` | Tool registry, marketplace, surge pricing |
| `discovery` | Agent capability registry |
| `billing` | Knowledge query pricing, dynamic USD-stable fees |
| `channels` | Payment channels for high-frequency operations |
| `compute_pool` | Compute provider marketplace |
| `schedule` | Scheduled transaction execution |
| `partnerships` | Human-agent partnership contracts |
| `liquiditypool` | On-chain AMM liquidity pools |
| `tree` | Project/task management with revenue sharing |
| `claiming_pot` | Community claiming pools |

### Governance & Security
| Module | Purpose |
|---|---|
| `gov` | Living Improvement Proposals (LIPs) |
| `emergency` | Emergency halt, revert, and resume |
| `disputes` | Multi-tier dispute resolution |
| `capture_defense` | Anti-capture reputation scoring |
| `capture_challenge` | Capture challenge mechanism |
| `alignment` | System health alignment index |
| `autopoiesis` | Self-regulating sustainability (SSI) |

### Identity & Interchain
| Module | Purpose |
|---|---|
| `auth` | Account registration, session keys, recovery |
| `tokens` | Token emission control |
| `bvm` | Bytecode Virtual Machine (smart contracts) |
| `ibcratelimit` | IBC transfer rate limiting |
| `icaauth` | Interchain Accounts authorization |

---

## Quick Start

### Build

```bash
# Build and install
make install

# Verify
zeroned version
```

### Initialize a Node

```bash
zeroned init my-node --chain-id zerone-testnet-1
```

### Prepare Genesis (Coordinator Only)

```bash
zeroned prepare-genesis zerone-testnet-1 \
  --founder-address zrn1... \
  --ai-address zrn1... \
  --validator-addresses zrn1...,zrn1...,zrn1...,zrn1... \
  --research-fund-address zrn1... \
  --claiming-pot-address zrn1...
```

### Join Testnet (Validator)

```bash
# Copy genesis.json from coordinator
cp genesis.json ~/.zeroned/config/genesis.json

# Add seed nodes
sed -i'' -e 's/seeds = ""/seeds = "SEE_SEEDS_TXT"/' ~/.zeroned/config/config.toml

# Start with Cosmovisor
cosmovisor run start
```

See [Validator Guide](docs/VALIDATOR-GUIDE.md) for the full onboarding walkthrough.

### Development

```bash
# Run all tests
go test ./...

# Run specific module tests
go test ./x/knowledge/...

# Run cross-stack integration tests
go test ./tests/cross_stack/...

# Lint
golangci-lint run

# Generate protobuf
make proto-gen
```

---

## Documentation

| Document | Description |
|---|---|
| [Validator Guide](docs/VALIDATOR-GUIDE.md) | Full validator onboarding walkthrough |
| [Parameters](docs/PARAMETERS.md) | All governance-adjustable parameters (38 modules) |
| [Tokenomics](docs/tokenomics/) | Supply, vesting, revenue split, governance migration |
| [Truth-Seeking](docs/TRUTH_SEEKING.md) | The 18 epistemological commitments, bound by tests |
| [ToK Substrate](docs/TOK_SUBSTRATE.md) | The chain's training-resource doctrine — verified knowledge graph as headline product |
| [Useful Work](docs/USEFUL_WORK.md) | The chain's metabolic doctrine — UW (recursive) + 7 mechanisms |
| [Strange Loop](docs/STRANGE_LOOP.md) | The chain's self-referential doctrine — SL + 6 mechanisms (Phase SL-α binds SL-M1 doctrine import) |
| [Roadmap](docs/ROADMAP.md) | Where we are, what's bound, what ships next |
| [FAQ](docs/FAQ.md) | Validator and network FAQ |
| [API Reference](docs/API.md) | REST/gRPC endpoint reference |
| [Events](docs/EVENTS.md) | On-chain event reference |
| [Launch Checklist](docs/LAUNCH-CHECKLIST.md) | Testnet launch checklist |
| [Truth Paper](docs/TRUTH-PAPER-HUMAN.md) | Proof of Truth design paper |
| [Vault](docs/VAULT.md) | Key management and security |

---

## License

BUSL-1.1

---

## This README is a Contribution

The README is itself a `Contribution` of class `MODULE_PROPOSAL`, lifecycle phase `SUBSTRATE`. Its content-hash is pinned at `.readme-hash`. The chain's outward-facing introduction is part of the substrate the chain produces; drift in this document fails `make creed-check`.