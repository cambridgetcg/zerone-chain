# R33-5 — Load & Stress Testing

## Objective

Push ZERONE to its performance limits. Identify bottlenecks in BeginBlock/EndBlock, store iteration, and transaction throughput before mainnet.

## Tasks

### 1. Transaction throughput

- Target: 100 txs/block for 100 blocks (10,000 total txs)
- Mix: 60% knowledge ops, 20% transfers, 10% governance, 10% partnerships
- Measure: block time, mempool backlog, finalization latency
- Identify the throughput ceiling

### 2. BeginBlock/EndBlock profiling

- Enable Go pprof during a 500-block stress run
- Profile which module hooks consume the most time
- Known suspects:
  - `alignment` EndBlock (iterates all sensors)
  - `knowledge` BeginBlock (metabolism decay over all facts)
  - `capture_defense` BeginBlock (domain analysis)
  - `pacing` EndBlock (multiplier recalculation)
- Target: BeginBlock + EndBlock < 200ms total at 1000 active facts

### 3. Store pressure

- Create 10,000 facts across 100 domains
- Create 1,000 partnerships
- Create 500 active proposals
- Measure query response times for:
  - `QueryFact` (single key lookup)
  - `QueryFactsByDomain` (iterator)
  - `QueryDomainPressure` (computed)
  - `QueryAlignmentHealth` (multi-module aggregation)
- Target: all queries < 100ms

### 4. Deep ontology tree

- Create ontology tree with depth 10, branching factor 5 (9,765 domains)
- Verify `GetDepthForDomain` doesn't degrade with tree depth
- Verify carrying capacity calculation doesn't recurse excessively
- Verify stratum-based queries perform acceptably

### 5. Large knowledge graph

- Create a fully connected citation graph with 100 facts (9,900 edges)
- Verify `GetInboundCrossDomainCitationCount` doesn't explode
- Verify graph acyclicity invariant check is bounded
- Identify if graph traversal needs optimization (indexed adjacency vs. iteration)

### 6. Memory profiling

- Run 1000-block simulation with memory profiling
- Check for memory leaks (growing allocations that don't get GC'd)
- Check cache sizes (any unbounded in-memory caches?)
- Verify IAVL tree doesn't grow unbounded in memory

## Performance Targets (Mainnet Readiness)

| Metric | Target | Critical |
|--------|--------|----------|
| Block time | < 5s | < 10s |
| BeginBlock + EndBlock | < 200ms | < 1s |
| Txs per block | ≥ 100 | ≥ 50 |
| Single-key query | < 10ms | < 50ms |
| Iterator query (100 items) | < 50ms | < 200ms |
| Memory per 1000 blocks | < 500MB growth | < 1GB growth |

## Acceptance Criteria

- [ ] 10,000 tx stress test completes without panic
- [ ] BeginBlock + EndBlock profiled and bottlenecks identified
- [ ] Query performance meets targets at 10,000 facts
- [ ] Deep ontology tree doesn't cause performance degradation
- [ ] No memory leaks detected over 1000-block simulation
- [ ] Performance report generated with actual numbers vs. targets
