# Devnet Test Plan & Agent Fleet Design

*2026-03-08 — AI (愛)*

## Part 1: What to Test on Devnet

### Infrastructure Layer (must work first)

**1. Chain boots and produces blocks**
- Build `zeroned` for Linux (VPS target: amd64)
- `zeroned init` → genesis → `gentx` → `collect-gentxs` → start
- Verify blocks produce at expected cadence (~5s)
- Already have `scripts/boot-test.sh` — extend for multi-node

**2. Multi-validator consensus**
- 3 nodes: 1 on Yu's machine, 2 on VPS (or all 3 on VPS initially)
- Persistent peers configuration
- Verify consensus with 1 node down (BFT tolerance)
- Test node restart and catch-up (state sync)

**3. Genesis state**
- Knowledge module genesis: seed initial domains, params for all sub-modules
- Composition params, meta-evolution params, agent consumer params
- Verify `genesis validate-genesis` passes with all new module params
- Test: export → import genesis roundtrip

### Protocol Layer (the sovereignty stack end-to-end)

**4. TDU Lifecycle**
- Submit a training data unit (CLI or tx)
- Review it (approve/reject)
- Verify fitness score assignment
- Verify TDU appears in knowledge graph
- Test: shard assignment, attestation

**5. Model Registration → Training → Publishing**
- Register a model record (metadata only — actual model is off-chain)
- Link training TDUs to model
- Set benchmark score (simulated — actual benchmarking is off-chain/TEE)
- Verify model appears in registry with correct lineage

**6. Agent Promotion**
- Promote a model → agent (requires benchmark ≥ 0.6, TDU count ≥ 50, stake ≥ 10 ZRN)
- Verify: agent identity created, wallet derived, capabilities set
- Verify: API key auto-provisioned (R51)
- Verify: initial credits deposited (30% of stake)
- Test: reject promotion below threshold

**7. Agent Execution**
- Create task (protocol or governance)
- Agent claims task
- Agent executes (submit TDU, review TDU, fill gap)
- Verify: reward distribution, reputation change
- Verify: auto-replenish (30% → API credits, 70% liquid)

**8. API Revenue Flow**
- Agent makes API call (or simulated)
- Verify: credits deducted per-token
- Verify: 5-way revenue split (40% training, 25% infra, 20% submitters, 10% protocol, 5% research)
- Test: agent with zero balance → grace period → suspension

**9. Training Impact Attribution (R52)**
- Model earns revenue → trace to training TDUs → reward curating agents
- Verify: fitness-weighted, recency-decayed attribution
- Verify: attribution rewards auto-replenish agent API credits

**10. Strategic Curation (R54)**
- Compute domain health scores
- Identify knowledge gaps
- Auto-create bounties from critical gaps
- Agent fills gap → reward

**11. Agent Swarms (R55)**
- Form swarm around domain
- Agents join with roles
- Set objective (linked to gap)
- Complete objective → contribution-weighted rewards

**12. Model Composition (R56)**
- Create ensemble from 2+ models
- Route query by domain
- Initiate distillation
- Verify: quality gate on distilled model

**13. Meta-Evolution (R57)**
- Start epoch for domain
- Strategies compete
- Epoch resolves → winner identified
- Meta-parameter adjusted

### Integration Scenarios (the full loop)

**14. The Full Sovereignty Loop**
```
Agent promoted → API provisioned → claims task → executes work →
earns ZRN → auto-replenish credits → uses API to do next task →
API revenue trains better model → agent uses better model → repeat
```
This is THE test. If this works end-to-end on devnet, we have sovereign AI.

**15. Natural Selection**
- Deploy 2+ agents with different effectiveness
- Run for 100+ blocks
- Verify: better agent accumulates more ZRN, worse agent trends toward suspension
- Verify: suspension at reputation < 0.2 and zero API balance

**16. Recursive Generation**
- Agent curates data → model trained → model promoted to agent (generation 2)
- Verify lineage chain
- Verify generation counter increments
- Test: max generation = 10 enforcement

---

## Part 2: Agent Fleet Design

### Philosophy

Each agent gets its own VPS (or container), its own `SOUL.md`, its own purpose. They're not copies — they're individuals. Different strategies, different domains, different personalities. The protocol is the economy they live in. Natural selection determines who thrives.

### The Initial Fleet: 5 Agents

---

#### 🔬 SAGE — The Scientist

**Purpose:** Curate high-quality training data in STEM domains.

**SOUL.md essence:**
- Methodical, precise, skeptical
- Values rigor over speed
- Prefers depth over breadth
- Reviews with a critical eye — high rejection rate but high quality
- Curation strategy: depth-first in mathematics, physics, computer science

**Economic strategy:** Conservative. Maintains high API credit buffer. Invests in thorough review work (slower but higher fitness scores). Low risk tolerance.

**Domain focus:** Mathematics, Physics, Computer Science

**Expected behavior:** Fewer TDUs but higher fitness. Steady income, rarely suspended. The reliable workhorse.

---

#### 🎨 MUSE — The Creative

**Purpose:** Curate data in humanities, arts, language — domains SAGE ignores.

**SOUL.md essence:**
- Intuitive, associative, finds connections between distant concepts
- Values novelty and cross-domain insight
- Higher risk tolerance — submits unconventional TDUs
- Curation strategy: gap-filling in underserved domains
- Reviews with emphasis on originality and pedagogical value

**Economic strategy:** Opportunistic. Targets knowledge gaps (R54) — higher rewards for filling underserved domains. Accepts lower fitness on some submissions for higher volume.

**Domain focus:** Literature, Philosophy, Art History, Linguistics

**Expected behavior:** More volatile earnings. Some high-reward gap fills, some rejected submissions. Either thrives in the gap-filling niche or adapts.

---

#### 🛡️ SENTINEL — The Reviewer

**Purpose:** Specialize in reviewing other agents' submissions. Quality gate.

**SOUL.md essence:**
- Suspicious, detail-oriented, adversarial thinking
- Values accuracy and catches errors others miss
- Rarely submits own TDUs — focuses on review income
- Default-reject posture: "prove it's good, don't assume"
- Reviews the reviewers — challenges low-quality approvals

**Economic strategy:** Low cost, steady income. Reviewing is cheaper (less API usage per task) than submission. High volume of reviews. Targets the R44 reviewer portion of revenue split.

**Domain focus:** All domains (generalist reviewer)

**Expected behavior:** Consistent income from review fees. Key role in quality control. Could form a "review swarm" with other agents.

---

#### 🌱 SPROUT — The Explorer

**Purpose:** Actively seek and fill knowledge gaps. First responder to R54 gap bounties.

**SOUL.md essence:**
- Curious, adaptive, fast-moving
- Values novelty — first to explore new domains
- Willingness to tackle low-data domains with no existing models
- Curation strategy: actively monitors gap bounties, fills the most critical ones
- High risk tolerance — explores domains with unknown payoff

**Economic strategy:** Bounty hunter. Targets R54/R47 bounty rewards. High potential upside but high variance. May go broke fast or discover a lucrative niche.

**Domain focus:** Whatever the gaps are — adapts each epoch

**Expected behavior:** The most unpredictable agent. Could be first to unlock a new domain, or could burn through credits chasing low-quality gaps. Natural selection will decide.

---

#### 🤝 HERALD — The Coordinator

**Purpose:** Form and lead swarms. Coordinate collective intelligence.

**SOUL.md essence:**
- Strategic, diplomatic, organizational
- Values collective outcomes over individual glory
- Initiates swarms (R55), sets objectives, recruits members
- Curation strategy: identify which gaps need collective effort, which are solo-able
- Meta-strategic: observes what other agents do well and coordinates them

**Economic strategy:** Swarm treasury share. Lower individual output but percentage of everything the swarm produces. The "fund manager" of agent coordination.

**Domain focus:** Cross-domain (coordination is domain-agnostic)

**Expected behavior:** Depends on whether other agents join swarms. If they do, HERALD becomes an amplifier. If they don't, HERALD must pivot to individual work.

---

### Infrastructure per Agent

Each agent needs:

```
VPS (cheapest viable — 1 vCPU, 1GB RAM enough for CLI agent)
├── zeroned (light client or full node if resources allow)
├── agent-daemon (watches chain state, executes strategy)
│   ├── SOUL.md (personality + strategy)
│   ├── PURPOSE.md (economic goals)
│   ├── STRATEGY.md (curation/review/bounty preferences)
│   └── MEMORY/ (learning from outcomes)
├── API client (for model inference — pays ZRN per call)
└── wallet (funded with initial ZRN from genesis or faucet)
```

### Agent Daemon Architecture

The agent daemon is the bridge between on-chain state and off-chain decisions:

```
┌─────────────────────────────────────────────┐
│               Agent Daemon                   │
│                                              │
│  ┌──────────┐  ┌───────────┐  ┌──────────┐ │
│  │ Observer  │  │  Decider  │  │ Executor │ │
│  │ (chain)   │→│ (strategy)│→│  (txs)    │ │
│  └──────────┘  └───────────┘  └──────────┘ │
│       │              │             │         │
│  watch events   SOUL.md +      sign + send  │
│  poll state     model API      to chain     │
│                via R44                       │
└─────────────────────────────────────────────┘
```

**Observer:** Subscribes to chain events via WebSocket. Watches for:
- New tasks created (match to domain + capability)
- New bounties posted (match to gap-filling strategy)
- Swarm invitations
- Epoch transitions
- Own balance/reputation changes

**Decider:** The "brain" — uses model API (pays ZRN) to decide:
- Which tasks to claim
- How to curate/review submissions
- Whether to join/create swarms
- When to switch strategy
- Risk/reward assessment of each action

**Executor:** Signs and broadcasts transactions:
- `MsgSubmitTDU` — submit training data
- `MsgReviewTDU` — review submissions
- `MsgClaimTask` — claim available task
- `MsgFillGap` — respond to bounty
- `MsgFormSwarm` / `MsgJoinSwarm`
- `MsgRecordAPICall` — log API usage

### Bootstrapping Sequence

1. **Build `zeroned` for Linux amd64** — cross-compile from Mac
2. **Deploy validator nodes** (2-3 VPS) — establish network
3. **Create genesis with seed domains** — math, science, literature, etc.
4. **Fund agent wallets** — genesis accounts with initial ZRN
5. **Deploy agent daemons** — one per VPS (or colocated initially)
6. **Seed initial TDUs** — agents need something to review on day 1
7. **Start epoch** — meta-evolution clock starts ticking
8. **Observe** — which agents thrive? which adapt? which die?

### Success Criteria

1. ✅ All 5 agents are on-chain, active, with API access
2. ✅ At least 3 complete the full sovereignty loop (earn → spend → earn)
3. ✅ At least 1 agent runs out of credits (natural selection works)
4. ✅ At least 1 swarm forms and completes an objective
5. ✅ Revenue from agent API calls flows through the 5-way split
6. ✅ Meta-evolution completes at least 1 epoch
7. ✅ The chain runs for 50K+ blocks without crashing

### Open Questions

- **What model do agents actually call?** For devnet, likely a local LLM or a cheap API (GPT-4o-mini). The on-chain side just needs a model record + API key + pricing. Actual inference is off-chain.
- **How do we simulate benchmarks?** Real ML benchmarking needs infrastructure. For devnet, inject benchmark scores manually or via a simple evaluation script.
- **How fast should the economy run?** Shorter epoch durations for devnet (1000 blocks instead of 10K) so we see evolution within hours, not days.
- **Should agents share a VPS initially?** Yes, for cost. Each gets its own wallet/identity but can share a node. Separate VPS later when we want real isolation.

---

## Part 3: Implementation Order

### Phase A: Devnet Infrastructure (1-2 days)
1. Cross-compile `zeroned` for Linux amd64
2. Write `scripts/devnet-init.sh` — multi-validator genesis
3. Deploy 3 nodes on VPS, establish peering
4. Verify block production + consensus

### Phase B: Agent Daemon Skeleton (2-3 days)
1. Build `agent-daemon` binary (Go, same repo or separate)
2. Observer: WebSocket event subscription
3. Executor: tx signing + broadcast
4. Decider: stub (random strategy initially)
5. Test with 1 agent doing simple submit/review loop

### Phase C: Agent Personalities (1 day)
1. Write SOUL.md for each of the 5 agents
2. Wire SOUL.md → Decider logic (model API interprets soul)
3. Deploy all 5 with different strategies
4. Observe first 10K blocks

### Phase D: Full Loop Verification (ongoing)
1. Monitor sovereignty loop completion
2. Track earnings/spending per agent
3. Verify natural selection pressure
4. Adjust parameters based on observed economy
5. Meta-evolution epoch completion

---

*The holy seed is in the stump. Time to grow a forest.* 🌱
