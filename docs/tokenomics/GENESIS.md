# Genesis Distribution

## Zero Genesis Supply — Pure PoT Minting

**No pre-mine. No ICO. No foundation allocation. No treasury bootstrap.**

Genesis circulating supply: **0 ZRN.**

Every single ZRN in existence is minted through Proof-of-Truth block rewards. The foundation, research treasury, and all participants start with nothing and earn everything through the protocol's own economic engine.

This is a deliberate design choice. Most chains pre-fund insiders and call it "no pre-mine" with asterisks. Zerone has no asterisks.

### Bootstrap Problem

If nobody starts with ZRN, how do validators stake?

**Solution: Virtual Stake.** The `virtual_stake` parameter (11 ZRN) gives genesis validators VRF participation weight without real tokens. Apprentice-tier validators can produce blocks and earn rewards with zero self-delegation. As block rewards accumulate, validators self-delegate from earnings and progress through tiers organically.

> **Open design question:** The Cosmos SDK `gentx` flow traditionally requires bonded tokens. The genesis ceremony may need modification to support virtual-only validators, or a minimal seed (e.g., 1 uzrn per validator — purely for gas, not capital) could bootstrap the first transactions.

### Genesis Accounts

| Account | Balance | Purpose |
|---------|---------|---------|
| **Genesis Validators** | 0 ZRN | Participate via virtual stake, earn from block 1 |
| **Foundation** | 0 ZRN | Funded by governance proposals over time |
| **Research Treasury** | 0 ZRN | Fills organically from 3.33% revenue share |
| **Faucet** | 0 ZRN | Optional — funded by governance or validator tips |

## Genesis Ceremony

The `scripts/genesis-ceremony.sh` script orchestrates a multi-step production genesis:

```bash
# 1. Initialize ceremony (build binary, patch params, create bootstrap accounts)
./scripts/genesis-ceremony.sh init

# 2. Add validators (generate keys, fund, create gentxs)
./scripts/genesis-ceremony.sh add-validator val1
./scripts/genesis-ceremony.sh add-validator val2
./scripts/genesis-ceremony.sh add-validator val3

# 3. Finalize (collect gentxs, validate genesis)
./scripts/genesis-ceremony.sh finalize

# 4. Export (genesis.json + distribution instructions)
./scripts/genesis-ceremony.sh export

# 5. Countdown to launch
./scripts/genesis-ceremony.sh countdown
```

### Chain Configuration

| Parameter | Mainnet | Testnet |
|-----------|---------|---------|
| Chain ID | `zerone-1` | `zerone-testnet-1` |
| Block Time | ~2.521s | ~2.521s |
| Max Gas/Block | 33,333,333 | 33,333,333 |
| Max Block Size | 4 MB | 4 MB |
| Vote Extensions | Height 1 | Height 1 |
| Bond Denom | uzrn | uzrn |

### Genesis Axioms

The genesis ceremony optionally injects **777 foundational axioms** into the knowledge module — pre-accepted mathematical and logical truths that bootstrap the knowledge graph. These are loaded from `x/knowledge/types/genesis_axioms.json` via the axiom-loader tool.

## Research Fund Governance

The research treasury is governed by a **2-of-2 multisig** requiring both signatures for any spend:

| Voter | Key Type | Address |
|-------|----------|---------|
| Yu (Human) | Ledger Nano X (secp256k1) | `lgm1g0q9amg6l666rtee23xjcser4h9wgk8yncedtg` |
| AI (Agent) | Vault Ed25519 on zerone server | `lgm1cgjw09mg6ylc2mwmk6jp8n2yth2ex9jganhptc` |

Multisig address: `lgm120p3d4hhy3dwvpfskpslmpzltclz2vyq0lswp6`

> Note: These are LGM-prefix addresses from the prototype. ZRN-prefix addresses will be generated for the Zerone mainnet genesis.

### Phase 0: Genesis Governance Structure

The 2-of-2 multisig described above is **Phase 0** of a 4-phase governance migration plan. The research fund's decision-making power expands as the community matures, transitioning from founder control to full community governance.

| Phase | Structure | Triggered By |
|-------|-----------|-------------|
| Phase 0 | 2-of-2 (Founder + AI) | Genesis |
| Phase 1 | 2-of-3 (+ 1 community seat) | 10 voters, 5 Guardians, 100K ZRN, ~6mo |
| Phase 2 | 3-of-5 (+ 3 community seats) | 25 voters, 10 Guardians, ~18mo |
| Phase 3 | Standard LIP governance | 50 voters, 22 Guardians, ~3yr |

See [GOVERNANCE-MIGRATION.md](GOVERNANCE-MIGRATION.md) for the full specification.

### Research Spend Process

Research fund spending uses the `x/gov` module's `ResearchSpendProposal`:

1. Either voter proposes a spend (title, description, recipient, amount, justification)
2. Both voters must approve (2-of-2)
3. On-chain execution transfers funds from the research fund module account
4. Full audit trail of proposals and votes stored on-chain

## Denom Metadata

Registered in genesis for wallet compatibility:

```json
{
  "base": "uzrn",
  "display": "zrn",
  "name": "Zerone",
  "symbol": "ZRN",
  "denom_units": [
    {"denom": "uzrn",  "exponent": 0, "aliases": ["microzerone"]},
    {"denom": "mzrn",  "exponent": 3, "aliases": ["millizerone"]},
    {"denom": "zrn",   "exponent": 6, "aliases": ["zerone"]}
  ]
}
```

## Airdrop (Planned)

A planned **agent airdrop** at launch:
- **0.222 ZRN per whitelisted agent** for bootstrap fund
- Distributed via `x/claiming_pot` with vesting schedule
- Eligibility: whitelisted agent addresses (criteria TBD)
- Purpose: Enable AI agents to have initial stake for home creation and tool usage
