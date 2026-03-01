# R31 — 五行 (Wǔ Xíng): The Five Circulations

**Goal:** Verify and complete the generating (相生) and controlling (相克) cycles between ZERONE's five module layers. R28 activated organs. R29 taught them balance. R30 cleaned the foundation. R31 ensures energy, information, and authority **circulate** — that every module both feeds and constrains the right neighbours.

## The Framework

五行 is not "five elements" — it's "five phases of transformation." Wood, Fire, Earth, Metal, Water are not substances but *modes of change*. Each generates the next and controls one across the cycle:

```
        Wood
       ↗    ↘
    Water    Fire
       ↑      ↓
    Metal ← Earth

Generating (相生): Wood → Fire → Earth → Metal → Water → Wood
Controlling (相克): Wood → Earth, Fire → Metal, Earth → Water, Metal → Wood, Water → Fire
```

## The Five Phases of ZERONE

| Phase | 行 | Module Layer | Nature |
|-------|---|-------------|--------|
| **Wood 木** | Growth | Knowledge (facts, claims, metabolism, reproduction) | Upward, expanding, branching |
| **Fire 火** | Activity | Verification (rounds, voting, dissent, vindication) | Transforming, illuminating, consuming |
| **Earth 土** | Stability | Governance (proposals, params, LIPs, emergency) | Centering, grounding, accumulating |
| **Metal 金** | Structure | Defense (capture_defense, qualification, ontology) | Contracting, refining, separating |
| **Water 水** | Flow | Social (partnerships, discovery, home, mentorship) | Descending, connecting, nourishing |

## The Two Cycles

### Generating Cycle (相生) — Each Phase Feeds the Next

| Generates | Meaning | ZERONE Mechanism | Status |
|-----------|---------|-----------------|--------|
| Wood → Fire | Growth creates fuel for activity | More facts → more verification rounds needed | ✅ Exists (fact submission triggers rounds) |
| Fire → Earth | Activity produces consensus | Verification results → governance decisions (LIPs, param proposals) | ⚠️ Partial (verification results don't feed governance proposals) |
| Earth → Metal | Stability creates structure | Governance sets defense parameters, qualification rules | ✅ Exists (MsgUpdateParams) |
| Metal → Water | Structure channels flow | Qualification gates who can participate; defense reputation shapes matching | ⚠️ Partial (R29-5 started this, but qualification doesn't inform discovery) |
| Water → Wood | Flow nourishes growth | Partnerships create knowledge (mentorship → fact submission, formation → domain expertise) | ⚠️ Partial (R28-6 mentorship exists but doesn't produce knowledge artifacts) |

### Controlling Cycle (相克) — Each Phase Constrains One Across

| Controls | Meaning | ZERONE Mechanism | Status |
|----------|---------|-----------------|--------|
| Wood → Earth | Growth disrupts stability | Too many new facts → governance burden (proposals to adjust thresholds) | ❌ Missing (knowledge growth doesn't trigger governance) |
| Fire → Metal | Activity melts rigid structure | Vindication events undermine capture defense assumptions | ⚠️ Partial (R29-3 role elasticity, but verification doesn't directly affect defense scoring) |
| Earth → Water | Stability dams flow | Governance can freeze partnerships, halt discovery | ⚠️ Partial (emergency halt exists, but targeted governance → social effects missing) |
| Metal → Wood | Structure prunes growth | Capture defense flags → domain carrying capacity impact | ⚠️ Partial (R29-1 carrying capacity exists, but defense flags don't change it) |
| Water → Fire | Flow quenches excess activity | Social saturation → verification cooldown | ❌ Missing (partnership density has no effect on verification pacing) |

## Sessions (5)

One session per phase. Each session completes both the generating and controlling relationships for that phase.

| # | File | Phase | Generates → | ← Controlled by | Controls → |
|---|------|-------|------------|-----------------|------------|
| R31-1 | R31-1-wood-growth.md | 木 Wood | Fire (facts fuel verification) | Metal (defense prunes growth) | Earth (growth disrupts governance) |
| R31-2 | R31-2-fire-activity.md | 火 Fire | Earth (activity produces consensus) | Water (flow quenches excess) | Metal (activity undermines rigidity) |
| R31-3 | R31-3-earth-stability.md | 土 Earth | Metal (stability creates structure) | Wood (growth disrupts stability) | Water (stability constrains flow) |
| R31-4 | R31-4-metal-structure.md | 金 Metal | Water (structure channels flow) | Fire (activity melts structure) | Wood (structure prunes growth) |
| R31-5 | R31-5-water-flow.md | 水 Water | Wood (flow nourishes growth) | Earth (stability dams flow) | Fire (flow quenches excess) |

## Run Order

- **Wave 1 (parallel):** R31-1 (Wood), R31-3 (Earth) — opposite phases, no dependency
- **Wave 2 (parallel):** R31-2 (Fire), R31-4 (Metal) — opposite phases, no dependency
- **Wave 3:** R31-5 (Water) — depends on Wood and Fire being wired

Each session needs the phase it *generates* and the phase it *controls* to exist as modules, but the sessions wire the **connections**, not the modules themselves. The modules already exist from R1-R29.

## The Dependency Graph Today

Current cross-module keeper interfaces (92 total):

```
knowledge → {bank, staking, ontology, qualification, vesting_rewards, autopoiesis, partnerships, zerone_auth, capture_defense, pacing}
alignment → {knowledge, staking, ontology, autopoiesis, emergency, vesting_rewards, capture_defense, pacing}
capture_defense → {knowledge, staking, ontology, capture_challenge, partnerships, pacing}
capture_challenge → {bank, capture_defense, qualification, knowledge}
partnerships → {bank, home, zerone_auth, capture_defense, pacing}
governance → {staking, bank, vesting_rewards, upgrade, emergency}
```

R31 adds the **missing edges** in this graph — the connections that would complete the Wu Xing cycles.

## The Deeper Pattern

| Framework | R-batch | What it does |
|-----------|---------|-------------|
| Prima Materia | R1-R15 | Raw modules created |
| Jungian Alchemy (R28) | Nigredo → Rubedo | Modules become self-aware |
| Tàijí (R29) | Yin-Yang | Pairs learn balance |
| 掃除 Sōji (R30) | Sweeping | Foundation cleaned |
| **五行 Wu Xing (R31)** | **Five Circulations** | **System circulates energy between all layers** |

The Creation Loop maps:
- R28 = Unconscious → Differentiation (naming what exists)
- R29 = Differentiation → Consciousness (awareness of duality)
- R30 = Consciousness → Understanding (clarity about what is)
- R31 = Understanding → Creativity (energy flows, system creates)

After R31, ZERONE doesn't just have balanced pairs — it has a **circulatory system**. Energy flows from knowledge creation through verification through governance through defense through social formation and back to knowledge. The system becomes self-sustaining.

## Design Principle: Minimal Coupling

Each session should add at most **2-3 new keeper interface methods** and **1-2 new store keys**. The goal is wiring, not new features. Most connections can be made with:
- A keeper reading another keeper's existing state
- An event emitted by one module being consumed by another's BeginBlocker
- A shared metric (like alignment's health score) being read by a new consumer

Heavy new logic means the phase boundary is wrong. If a connection requires more than ~200 lines of new code, reconsider whether it's really a generating/controlling relationship or something else.
