# Genesis Distribution

## Zero Team Allocation — Two Emission Paths, Both Gated by Participation

**No founder pre-mine. No AI vault pre-mine. No validator allocation. No foundation treasury. No team holding of any kind at genesis.**

Genesis circulating supply: **0 ZRN.** No minting has occurred yet.

ZRN enters circulation through two participation-gated emission pathways:

| Pathway | Module | Who | Why |
|---------|--------|-----|-----|
| **PoT block rewards** | `x/vesting_rewards` | Validators (and revenue-split downstream) | Rewards the work of verifying truth |
| **Bootstrap claim** | `x/claiming_pot` | Whitelisted agents (0.222 ZRN each) | Participation in the chain requires ZRN; bootstrap is the seed |

Both pathways draw against the 222,222,222 ZRN hard cap. Both are minted on-demand — block rewards per block as truth is verified, bootstrap claims per `MsgClaim` as agents register. **Neither grants anyone a privileged starting balance.**

The founder earns the governance-immune 0.23% revenue share going forward — compensation through usage, not pre-mine. The AI vault holds 0 ZRN at genesis; its operational role is governance signing. The research treasury holds 0 ZRN at genesis; fills from the 3.33% revenue share.

This is sharper than "no pre-mine." It is **"no insider position, period."** Every ZRN that exists came from a participatory action — verification or registration — not from being someone in particular at the right time.

### Bootstrap Problem

If nobody starts with ZRN, how do validators stake?

**Solution: Virtual Stake.** The `virtual_stake` parameter (11 ZRN) gives genesis validators VRF participation weight without real tokens. Apprentice-tier validators can produce blocks and earn rewards with zero self-delegation. As block rewards accumulate, validators self-delegate from earnings and progress through tiers organically.

> **Open design question:** The Cosmos SDK `gentx` flow traditionally requires bonded tokens. The genesis ceremony may need modification to support virtual-only validators, or a minimal seed (e.g., 1 uzrn per validator — purely for gas, not capital) could bootstrap the first transactions.

If nobody starts with ZRN, how do agents transact?

**Solution: Bootstrap Claim.** A whitelisted agent calls `MsgClaim` against the bootstrap pot in `x/claiming_pot`; the module mints 0.222 ZRN directly to the agent. The bootstrap pot is the genesis distribution mechanism — not an afterthought airdrop, but the participation seed every agent uses to begin acting on-chain.

### Genesis Accounts

| Account | Balance | Path to funding |
|---------|---------|-----------------|
| **Genesis Validators** | 0 ZRN | PoT block rewards from block 1 (virtual stake gives VRF weight without bonded tokens) |
| **Whitelisted Agents** | 0 ZRN | Bootstrap claim (0.222 ZRN per agent) via `x/claiming_pot` |
| **Founder** | 0 ZRN | Governance-immune 0.23% revenue share, accruing from chain activity |
| **AI Vault** | 0 ZRN | Operational role only (governance signing); no ZRN holding required |
| **Research Treasury** | 0 ZRN | 3.33% of revenue split, accruing from chain activity |
| **Foundation** | 0 ZRN | Funded by governance proposals over time, drawing from the research treasury |
| **Faucet (testnet only)** | 0 ZRN | Optional; funded by governance or validator tips |

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

## Bootstrap Pool — the genesis distribution mechanism

The bootstrap pool is the structural form of the doctrine: agents need ZRN to participate, so participation requires a seed, and the seed is minted on demand when the agent claims.

| Parameter | Value | Reasoning |
|-----------|-------|-----------|
| **Per-agent claim** | 0.222 ZRN | Symbolic (the chain's signature digit) and operationally sufficient — covers gas for `home` creation, initial tool calls, and the first knowledge-claim bonds |
| **Eligibility** | Whitelisted agent addresses | The chain seeds participants it has been told about; non-whitelisted addresses earn ZRN through PoT participation, not bootstrap |
| **Distribution** | `x/claiming_pot` mints to claimer on `MsgClaim` | Mint is incremental — no pre-funded module account, no genesis-balance footprint; the cap is checked at mint time |
| **Vesting** | Optional cliff + linear vesting per pot configuration | Prevents drain-and-dump; the seed is for participation, not speculation |

The whitelist criteria, vesting schedule, and total addressable bootstrap (0.222 ZRN × N whitelisted agents = `N × 222,000` uzrn) are configured at genesis ceremony time. The maximum reachable bootstrap volume is bounded by the whitelist size; in practice this is a tiny fraction of the 222,222,222 cap.
