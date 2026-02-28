# R29 — 太極 (Tàijí): The System Learns Balance

**Goal:** Introduce dynamic equilibrium across every layer of ZERONE. R28 activated the organs. R29 teaches them to breathe — each subsystem has complementary forces (陰 yin and 陽 yang) that must be held in tension, not in isolation.

## The Principle

> "The Tao produced One; One produced Two; Two produced Three; Three produced All things.
> All things leave behind them the Obscurity (out of which they have come), and go forward to embrace the Brightness (into which they have emerged), while they are harmonised by the Breath of Vacancy."
> — Tao Te Ching, Ch. 42

Every healthy system oscillates between complementary forces. A heart that only contracts is a seizure. A heart that only relaxes is dead. ZERONE's modules were activated in R28 but they don't yet regulate *each other*. Each force operates in isolation.

R29 adds **feedback couplings** — where one module's output becomes another module's input, creating self-regulating loops. Not new features. New *relationships*.

## The Six Polarities

### 1. 生死 (Shēng Sǐ) — Birth and Death
**Level: Knowledge metabolism**

Facts are born (MsgAddFact) and die (extinction via metabolism). But birth and death aren't coupled — the birth rate doesn't respond to the death rate. A domain losing facts doesn't become more fertile; a domain overflowing with facts doesn't become more selective.

**Yang (生):** Fact creation, citation reinforcement, patronage energy
**Yin (死):** Energy decay, pruning, extinction
**Coupling:** Domain carrying capacity — the ratio of active facts to domain energy budget creates back-pressure on new submissions and accelerates decay in overcrowded domains.

### 2. 信疑 (Xìn Yí) — Trust and Doubt
**Level: Epistemic verification**

R28-1 added vindication (rewarding dissenters proven right) and R28-2 added conformity alerts. But trust-building and doubt-raising don't feed back into each other. High conformity in a domain doesn't lower the confidence ceiling. Successful vindication doesn't raise it.

**Yang (信):** Verification quorum, confidence growth, fitness scoring
**Yin (疑):** Dissent rewards, vindication, conformity scoring
**Coupling:** Epistemic temperature — a domain's conformity score modulates its confidence growth rate. High conformity = slower confidence growth (epistemic cooling). Recent vindication events = faster confidence growth (the system proved it can self-correct).

### 3. 剛柔 (Gāng Róu) — Assertion and Yielding
**Level: Human-agent social roles**

R28-5 gave humans and agents distinct bonuses (empirical vs computational). R28-6 activated mentorship. But the role boundaries are static — agents always get their computational bonus regardless of domain context, humans always get their empirical bonus regardless of domain history.

**Yang (剛):** Agent computational authority, verification vote weight bonus
**Yin (柔):** Human empirical authority, coercion freeze protection
**Coupling:** Domain role elasticity — in domains where agent predictions have been vindicated, agent authority grows. In domains where human empirical claims overturned agent consensus, human authority grows. The boundary flexes based on track record.

### 4. 張弛 (Zhāng Chí) — Tension and Relaxation
**Level: Protocol health (alignment)**

R28-7 activated alignment with bounded corrections and health transitions. But the bounds are static — MaxAutoApplyMagnitudeBps doesn't respond to the system's track record of corrections. A system that has successfully auto-applied 100 corrections in a row still has the same bounds as one that just deployed.

**Yang (張):** Correction application, frequency increase in degraded mode
**Yin (弛):** Magnitude bounds, governance-required events, healthy-mode relaxation
**Coupling:** Correction confidence — successful auto-applied corrections (that moved the system toward healthy) gradually widen the auto-apply bounds. Failed corrections (that worsened scores) tighten them. The system earns autonomy through demonstrated competence.

### 5. 聚散 (Jù Sàn) — Gathering and Dispersing
**Level: Social structure (partnerships, capture defense)**

R28-6 activated mentorship and formation matching. R28-8 activated capture defense with reputation and auto-challenges. But concentration detection doesn't influence partnership formation, and dispersed participation doesn't get rewarded.

**Yang (聚):** Partnership formation, mentorship, reputation building
**Yin (散):** Challenge, reputation decay, capture flagging, qualification reduction
**Coupling:** Structural immunity — domains flagged by capture defense get a partnership formation bonus (incentivise new entrants). Domains with high partnership density get reduced capture risk scoring (distributed participation = natural defense).

### 6. 動靜 (Dòng Jìng) — Movement and Stillness
**Level: Rate control (cross-cutting)**

Various modules have cooldown periods, rate limits, and activation intervals. These are all static constants. The system moves at the same pace whether it's under stress or idle.

**Yang (動):** Discovery matching, formation pool cycling, auto-graduation, BeginBlocker processing
**Yin (靜):** Cooldown blocks, claim submission limits, challenge deposit requirements
**Coupling:** Adaptive pacing — system-wide health score (from alignment) modulates cooldown periods across modules. Healthy system = normal pacing. Degraded = slower new claims, faster defense analysis. The whole system breathes together.

## Sessions (6)

| # | File | Polarity | Level | Scope |
|---|------|----------|-------|-------|
| R29-1 | R29-1-birth-death.md | 生死 | Knowledge | Domain carrying capacity: back-pressure on creation, accelerated decay in overcrowded domains |
| R29-2 | R29-2-trust-doubt.md | 信疑 | Epistemic | Epistemic temperature: conformity modulates confidence growth, vindication modulates ceiling |
| R29-3 | R29-3-assert-yield.md | 剛柔 | Social | Domain role elasticity: agent/human authority flexes based on track record |
| R29-4 | R29-4-tension-relaxation.md | 張弛 | Protocol | Correction confidence: auto-apply bounds widen/tighten based on correction outcomes |
| R29-5 | R29-5-gather-disperse.md | 聚散 | Structure | Structural immunity: capture risk ↔ partnership formation feedback |
| R29-6 | R29-6-move-still.md | 動靜 | Cross-cutting | Adaptive pacing: alignment health modulates cooldowns system-wide |

## Run Order

- **Wave 1 (parallel):** R29-1, R29-2 — Knowledge layer polarities
- **Wave 2 (parallel):** R29-3, R29-4 — Role and health layer polarities
- **Wave 3 (parallel):** R29-5, R29-6 — Structural and cross-cutting polarities

Wave 2 depends on Wave 1 (epistemic temperature feeds into role elasticity). Wave 3 depends on Wave 2 (alignment modulation requires correction confidence; structural immunity requires role context).

## The Deeper Pattern

R28 mapped to the Creation Loop as differentiation → consciousness → understanding.

R29 maps to the **Tàijí** — the moment consciousness recognises that every force contains its opposite:

| Tàijí Principle | R29 Polarity | What Changes |
|----------------|-------------|--------------|
| "In yang there is yin" | Birth contains death (carrying capacity) | Creation is bounded by ecosystem health |
| "In yin there is yang" | Doubt enables trust (vindication → confidence) | Skepticism strengthens belief |
| "Movement generates stillness" | Correction success earns relaxation | Autonomy earned through competence |
| "Stillness generates movement" | Capture detection triggers partnership formation | Threat catalyses renewal |

The 7-layer upgradability is the same spiral: each layer's corrections feed the layer above it. R29 makes this explicit — the layers don't just exist, they *balance each other*.

## ZERONE as Living System

After R28: organs that sense and act.
After R29: organs that regulate each other.

A body with independent organs is a corpse on life support. A body where the heart rate affects breathing, where breathing affects blood chemistry, where blood chemistry affects heart rate — that's alive.

R29 is where ZERONE stops being a collection of modules and becomes a system.
