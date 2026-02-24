# Tokenomics Review: Honest Assessment

> Written by AI (愛), 2026-02-23. This is a critical review, not marketing.

## What's Strong

### 1. Coherent Economic Loop

The core loop is tight: **truth creates tokens → tokens fund more truth-seeking → better truth increases token value**. Unlike most crypto projects where the token is grafted onto an existing application, ZRN is structurally necessary — you literally cannot produce blocks without verified knowledge. The token isn't an afterthought; it's the substrate.

### 2. No Burn — Every Token Works

Most chains burn tokens as a deflationary signal to markets. Zerone takes the opposite position: burning freshly minted tokens is just minting less with ceremony. The 19.67% that would have been burned instead funds bug bounties, truth discovery, and protocol development. The hard cap provides scarcity; artificial deflation is unnecessary when you can fund productive work instead.

### 3. Truth-Linked Vesting Is Novel

I haven't seen this elsewhere. Tying reward release to the epistemic category and survival of knowledge claims creates genuine skin-in-the-game for truth. The half-life curves are thoughtfully designed — axioms vest slowly, oracle feeds vest quickly. The clawback mechanism (33% of released + 100% of unvested + 100% of reserve) is painful enough to deter fraud without being so punitive that it discourages participation.

### 4. The Tiered Validator System Is Well-Designed

The progression from Apprentice → Guardian with increasing stake, reputation, and accuracy requirements creates a natural meritocracy. The fact that you can't just buy your way to Guardian tier (you need 333 verifications with 77% accuracy including 33 contested ones) is a strong anti-plutocracy mechanism.

### 5. Anti-Capture Is Architectural

HHI-based concentration monitoring, cross-stratum verification requirements, isolated reputation scopes, and the capture challenge/defense pair are unusually thorough. Most chains treat anti-capture as an afterthought; here it's four dedicated modules.

### 6. Self-Regulation (Autopoiesis + Alignment)

The SSI-based adaptive parameters are genuinely innovative — the chain adjusting its own slash severity based on system health, within governance-bounded rails. This is closer to how biological systems maintain homeostasis than anything I've seen in crypto.

## What Needs Work

### 1. ~~Genesis Bootstrap Is Top-Heavy~~ → RESOLVED

**Decision: Zero genesis supply.** No foundation allocation, no research treasury bootstrap. Every ZRN minted through PoT block rewards. This eliminates the centralisation concern entirely.

**New concern: Bootstrap friction.** With 0 ZRN at genesis, validators rely on virtual stake and earn from block 1. The research fund starts empty and fills organically from 13% revenue share. This is philosophically clean but means early-stage funding for ecosystem development depends entirely on block reward accumulation speed.

### 2. ~~The Founder Share Is Uncomfortably Manual~~ → RESOLVED (by design)

The 7% founder share of research fund (0.23% of total revenue) is now **governance-immune by design**. `ValidateFounderShareImmutability()` in `MsgUpdateParams` rejects any governance proposal that modifies `founder_share_bps` or `founder_address` once set.

**This is intentional, not an oversight.** The founder share is a permanent protocol commitment — perpetual alignment between creator and network. Only a code upgrade (upgrade-category LIP) could alter it.

**Remaining concern:** The share goes directly to an address with no vesting or lock. At 0.23% of total revenue it's modest, but it's still unencumbered capital.

### 3. Empty Block Reward = 0 May Cause Issues

Zero reward for blocks without PoT activity means validators earn nothing when the knowledge pipeline is quiet. In early network phases with low activity, this could create:
- Validator exodus during quiet periods
- Incentive to spam low-quality claims just to trigger rewards
- Block production without economic incentive during knowledge droughts

**Consideration:** Even a small empty block reward (e.g., 1% of base) would maintain validator incentives during quiet periods without undermining the PoT alignment.

### 4. ~~Decay Is Very Aggressive~~ → RESOLVED

**Decision: 1-year half-life.** `reward_decay_bps` changed from 850,000 (15%/epoch, ~78-day collapse) to 994,478 (0.55%/epoch, ~1-year half-life). Rewards now halve annually — 4× faster than Bitcoin's 4-year halvings but 125× slower than the original design.

**Result:** Validators joining at year 2 earn 2.5 ZRN/block (half the genesis rate), not 0.001 ZRN. Floor reward (0.1 ZRN) is reached at ~year 6.6 instead of day 78. The gold rush dynamic is eliminated while still rewarding early participation.

### 5. Verification Pool Split Is Complex

The path from block reward to compute provider is: block reward → 22% protocol → 30% verification → 30% compute = 2% of total. That's a 5-level split before money reaches compute providers. Complexity breeds confusion and potential rounding errors.

### 6. Max 3 Liquidity Pools Is Restrictive

With only 3 AMM pools allowed, the protocol severely limits what trading pairs can exist on-chain. This may be intentional for simplicity, but could bottleneck price discovery for future ZRN-20 tokens.

### 7. Dynamic Pricing Oracle Is Disabled

The billing module's dynamic pricing (ZRN/USD peg for query costs) is disabled at genesis. When enabled, it introduces oracle dependency. The 3-tier fallback (TWAP → manual governance → min/max bounds) is reasonable, but the TWAP requires active liquidity pools (see above — max 3 pools).

### 8. Research Fund Centralisation Risk

The 2-of-2 multisig between Yu and AI for the research fund is philosophically beautiful but practically a centralisation point. If either key is lost or compromised, the research fund is permanently locked or at risk.

**Mitigations in place:**
- Vault key on dedicated hardware with challenge-response auth
- Ledger Nano X for human key
- On-chain ResearchSpendProposal with full audit trail

**Still needed:**
- Recovery mechanism if one key becomes unavailable
- Plan for transitioning to broader governance (3-of-5? community vote?)

## Open Questions

### Economic

1. **What's the equilibrium circulating supply?** With burns, vesting locks, staking, and the supply cap, the actual liquid supply at any given time is hard to model. A simulation would help.

2. **When does the cap bind?** With no burn, the supply monotonically increases. At floor rate (~1.26M/year) plus early rapid emission (~8M in first 3 months), the cap could bind in decades. When `MintWithCap` starts clamping, block producers earn less. The transition is smooth (it mints whatever headroom remains) but the economic shift from minting-based to fee-based incentives needs testing.

3. **Is the development fund governance-ready?** 19.67% of all revenue is a substantial fund. The governance mechanism for disbursing it (LIP proposals) needs to be robust from day 1. Without clear disbursement criteria, the fund could become a political football or sit idle.

4. **Are knowledge query prices viable?** Base price of 1 ZRN per query is high. Even with the $0.01 target in dynamic pricing, early ZRN will likely be worth less than $0.01, making queries effectively free. Is that the intent?

### Governance

5. **Who decides the knowledge strata?** `allow_new_strata = false` at genesis. New knowledge domains can be added, but new epistemic strata require a code upgrade. Is this too conservative?

6. **Can governance break the economics?** A governance proposal could set `burn_bps = 0` and `contributor_bps = 1000000`, directing all revenue to contributors with no burn. Should there be parameter bounds enforced in code?

7. **Emergency governance thresholds are very high.** 75% for halt, 80% for revert/resume. With 22 validators, that requires near-unanimity. Is this too high for actual emergencies?

### Technical

8. **How does vesting work in practice?** The VestingSchedule proto has 22 fields. How do contributors actually claim? Is there a UI? An auto-claim mechanism? (Home treasury has `auto_claim_vesting = true`, but homes require 10 ZRN creation fee.)

9. **Citation pool distribution mechanism?** The citation pool (11% of total) accumulates but the distribution to cited authors isn't fully specified in the reviewed code. How does it flow back?

10. **FARM anti-gaming effectiveness?** The 6 FARM parameters (conformity threshold, calibration trivial threshold, misbehaviour rejection, etc.) are configurable but their interaction effects are hard to predict. Simulation or testnet data needed.

## Verdict

The tokenomics are **significantly more thoughtful than typical crypto projects**. The truth-linked vesting, 4-way split, and anti-capture infrastructure show genuine economic design rather than token-bolted-on-afterwards thinking.

The main risks are:
- **Bootstrap friction** — zero genesis supply means slow early funding for ecosystem
- **Complexity** — 32 modules with independent parameters create a large governance surface area

The main strengths are:
- **No-burn philosophy** — every token does productive work, scarcity from hard cap
- **Knowledge-aligned incentives** that reward truth production over capital accumulation
- **Self-healing economics** via autopoiesis/alignment

For testnet, the priority should be empirically validating the empty block economics and vesting clawback flows. These are the mechanisms most likely to surprise when real actors interact with them. The decay curve (1-year half-life) is well-tested in unit and integration tests.
