# Zerone Validator FAQ

## General

### What is Proof of Truth (PoT)?

Proof of Truth is Zerone's consensus mechanism for knowledge verification.
Unlike Proof of Stake (which validates transactions) or Proof of Work (which
solves puzzles), PoT validators evaluate the truthfulness of knowledge claims
submitted to the network. Validators commit, reveal, and aggregate their
judgments in a three-phase protocol. Correct verifications earn rewards;
incorrect ones are slashed.

### What is ZRN?

ZRN is Zerone's native token. It is used for staking, transaction fees,
knowledge claim bonds, and governance voting. The micro-denomination is
`uzrn` (1 ZRN = 1,000,000 uzrn). The total supply at genesis is
222,222,222,222 ZRN.

### How is ZRN distributed at genesis?

| Allocation | Amount (ZRN) | % | Purpose |
|-----------|-------------|---|---------|
| Validators (4x) | 88,888,888,888 | 40% | Genesis validator stake and operations |
| Research Fund | 44,444,444,444 | 20% | Research grants and bounties |
| Claiming Pots | 44,444,444,446 | 20% | Community claiming pools |
| Founder | 22,222,222,222 | 10% | Founder allocation |
| AI Agents | 22,222,222,222 | 10% | AI agent economy bootstrap |

---

## Staking & Rewards

### How do I earn ZRN as a validator?

Validators earn ZRN through three channels:

1. **Verification rewards** — 3 ZRN per correct verification (decaying
   0.1% per epoch)
2. **Block rewards** — 10 ZRN per block, distributed to validators based on
   their tier and participation
3. **Fee share** — 93% of transaction fees go to validators; 7% goes to the
   research fund

### What are the validator tier requirements?

| Tier | Min Stake | Min Reputation | Min Verifications | Reward Multiplier |
|------|-----------|----------------|-------------------|-------------------|
| Apprentice | 0.111 ZRN | -- | -- | 0.1x |
| Verified | 1.11 ZRN | 77% | 22 | 0.5x |
| Scholar | 1,111 ZRN | 50% | 11 | 1.0x |
| Guardian | 11,111 ZRN | 77% | 333 | 2.0x |

Only **Scholar** and **Guardian** tiers produce blocks. Apprentice and
Verified tiers participate in knowledge verification with lower reward
multipliers.

### How does tier progression work?

Your tier is recomputed automatically when your stake, reputation, or
verification count changes. To increase your stake:

```bash
zeroned tx staking update-stake <amount>uzrn --increase --from <key> --chain-id zerone-testnet-1
```

Reputation increases by +0.01% per correct verification and decreases by
-0.02% per incorrect one. Slashing penalties reduce reputation by -1%.

### What is the unbonding period?

268,560 blocks (~7 days at 2,521ms block time). During unbonding, your
tokens are locked and do not earn rewards. The redelegation cooldown
(moving stake between validators) is 1,111 blocks (~46 minutes).

---

## Slashing

### What offenses are slashed?

| Offense | Slash Rate | Description |
|---------|-----------|-------------|
| Wrong verification | 5% | Submitting an incorrect verification judgment |
| Missed reveal | 10% | Committing but failing to reveal in time |
| Equivocation | 20% | Submitting conflicting judgments |
| Invalid claim | 22% | Submitting a malformed or fraudulent claim |
| Failed challenge | 22% | Losing a challenge against a verified fact |

### What happens when my validator is slashed?

Your self-delegation is reduced by the slash percentage. Your reputation
score also decreases by 1%. Guardians have zero tolerance for slashing —
any slash event deactivates a Guardian validator. Apprentice validators have
a 1.5x slash multiplier (higher penalty per offense).

### How many slashes before deactivation?

By default, 3 slashes within a decay period (34,272 blocks, ~1 day) will
deactivate a validator (except Guardians, which deactivate on first slash).
Slash escalation increases the penalty by 10% for each successive slash.

---

## Fees & Bootstrap Period

### What is the bootstrap period?

The first 480,000 blocks (~14 days) after genesis. During bootstrap, the
following transactions are **gas-free**:

- Validator registration
- Account registration
- Knowledge claim submission
- Verification commit and reveal

This allows the network to begin verifying truth before any fees are
collected.

### What are the gas prices?

The recommended minimum gas price is `0.025uzrn`. Specific transaction costs
vary by type — for example, registering a validator costs 100,000 gas, while
a simple transfer costs 21,000 gas. See [PARAMETERS.md](PARAMETERS.md) for
the full gas cost table.

### How are fees distributed?

Block reward revenue is split four ways:

- **55%** to fact contributors
- **22%** to the protocol (citations, verification, treasury)
- **19.67%** to the development fund (bug bounties, protocol development)
- **3.33%** to the research fund (community grants)

No ZRN is burned — every token does productive work.

### Why doesn't Zerone burn tokens?

Burn mechanics reduce supply but destroy value. Zerone redirects what would
have been burned into the **development fund**, which finances bug bounties,
truth discovery rewards, and protocol development. Every ZRN stays in the
ecosystem doing productive work rather than being destroyed for artificial
scarcity.

### Can the founder share be changed by governance?

No. The `founder_share_bps` and `founder_address` parameters are
**governance-immune** — they cannot be modified via `MsgUpdateParams`. This
protects the founder's contribution from being voted away and ensures long-term
alignment between the founder and the protocol.

### What is the development fund?

The development fund receives 19.67% of block reward revenue (replacing the
former burn allocation). It funds:

- Bug bounties and security audits
- Truth discovery rewards
- Protocol development and tooling

Disbursement is managed via governance proposals (research spend proposals).

---

## Infrastructure

### Can I run a validator on a VPS?

Yes. A VPS with at least 4 CPU cores, 16 GB RAM, and 500 GB SSD is
sufficient for testnet. Ensure your provider allows sustained CPU usage and
has reliable networking. Popular choices include Hetzner, OVH, and
DigitalOcean dedicated CPU instances.

Important VPS considerations:
- Ensure port 26656 (P2P) is open in your firewall
- Use a static IP or configure `external_address` in config.toml
- Set up monitoring to detect downtime quickly
- Use Cosmovisor for automated upgrades

### Should I use Cosmovisor?

Yes. Cosmovisor watches for governance-approved upgrade proposals and swaps
the `zeroned` binary automatically at the correct block height. This
minimizes downtime during chain upgrades. See
[cosmovisor/README.md](../cosmovisor/README.md) for setup instructions.

### How do I back up my validator?

Critical files to back up:

```
$HOME/.zeroned/config/priv_validator_key.json   # Consensus key — NEVER share
$HOME/.zeroned/config/node_key.json             # Node identity key
$HOME/.zeroned/data/priv_validator_state.json    # Signing state (prevent double-sign)
```

Back up your key mnemonic securely offline. If you lose
`priv_validator_key.json`, you cannot recover your validator without it.

> **Warning:** Never run two nodes with the same `priv_validator_key.json`.
> This causes equivocation (double-signing) and results in a 20% slash.

---

## Governance

### How do I propose a parameter change?

Zerone uses a LIP (Living Improvement Proposal) governance system. To propose
a parameter change:

```bash
zeroned tx gov submit-lip \
  --title "Increase max verifiers" \
  --description "Proposal to increase max_verifiers from 22 to 33" \
  --category "protocol" \
  --from my-validator \
  --chain-id zerone-testnet-1
```

Proposals go through a discussion period (68,544 blocks, ~2 days) followed
by a voting period (102,816 blocks, ~3 days). The quorum threshold is 33.4%
and the support threshold is 50%.

### How do I vote?

```bash
zeroned tx gov cast-vote <proposal-id> yes \
  --from my-validator \
  --chain-id zerone-testnet-1
```

Vote options: `yes`, `no`, `abstain`.

---

## Network

### What is the chain ID?

`zerone-testnet-1`

### What is the block time?

Target: 2,521 milliseconds (~2.5 seconds).

### What is the address prefix?

- Regular accounts: `zrn1...`
- Validator operators: `zrnvaloper1...`
- Validator consensus: `zrnvalcons1...`

---

## Further Reading

- [Validator Guide](VALIDATOR-GUIDE.md) — Full onboarding walkthrough
- [Parameters Reference](PARAMETERS.md) — All governance-adjustable parameters
