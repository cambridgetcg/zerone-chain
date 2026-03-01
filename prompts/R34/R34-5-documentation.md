# R34-5 — Documentation & Operator Guide

## Objective

Write the documentation needed for validators, operators, and developers to run ZERONE in production.

## Tasks

### 1. Module specifications

For each of the 32 custom modules, create `x/<module>/README.md`:
- Purpose and design rationale
- State model (key prefixes, stored types)
- Messages (with parameter descriptions)
- Queries (with response formats)
- Events emitted
- Parameters (with valid ranges)
- Invariants registered
- BeginBlock/EndBlock hooks (if any)
- Cross-module dependencies

### 2. Operator guide

Create `docs/OPERATOR.md`:
- Hardware requirements (CPU, RAM, disk, bandwidth)
- Installation (from source, Docker, Cosmovisor)
- Configuration (app.toml, config.toml key settings)
- Genesis ceremony participation
- Validator setup (key management, signing)
- Monitoring setup (Prometheus + Grafana)
- Upgrade procedure (Cosmovisor flow)
- Backup and recovery
- Common troubleshooting

### 3. Genesis ceremony guide

Create `docs/GENESIS_CEREMONY.md`:
- Timeline and phases
- Gentx submission process
- Genesis parameter review
- Chain-id and genesis time coordination
- Peer exchange
- Launch coordination

### 4. API documentation

- OpenAPI/Swagger spec for REST endpoints (auto-generated from proto)
- gRPC endpoint documentation
- WebSocket subscription guide
- Rate limiting and access control recommendations

### 5. Architecture overview

Create `docs/ARCHITECTURE.md`:
- System diagram (all 32 modules and their relationships)
- Wu Xing circulation diagram
- Transaction lifecycle (from submission to finalization)
- Block lifecycle (BeginBlock → DeliverTx → EndBlock → Commit)
- Economic flow diagram
- Upgrade architecture (7-layer upgradability)

### 6. Developer guide

Create `docs/DEVELOPER.md`:
- Local development setup
- Running tests (unit, integration, cross-stack, simulation, E2E)
- Adding a new module
- Proto workflow
- Commit conventions
- Prompt system explanation (for AI-assisted development)

## Acceptance Criteria

- [ ] All 32 modules have README.md with specs
- [ ] Operator guide covers full lifecycle from install to upgrade
- [ ] Genesis ceremony guide is detailed enough to follow without assistance
- [ ] API docs generated and accessible
- [ ] Architecture diagrams created
- [ ] Developer guide enables new contributors to start
