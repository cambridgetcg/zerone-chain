# R34-3 — Monitoring & Observability

## Objective

Add Prometheus metrics, Grafana dashboard templates, and alerting rules for ZERONE-specific modules. Operators need visibility into chain health beyond standard CometBFT/SDK metrics.

## Tasks

### 1. Custom Prometheus metrics

Add to each priority module's keeper:

**Knowledge:**
- `zerone_knowledge_facts_total{domain,status}` — fact count by domain and status
- `zerone_knowledge_claims_pending` — pending verification claims
- `zerone_knowledge_domain_pressure{domain}` — carrying capacity pressure
- `zerone_knowledge_verification_rounds_active` — active verification rounds

**Alignment:**
- `zerone_alignment_composite_health` — composite health score (0-1M)
- `zerone_alignment_sensor{sensor_name}` — individual sensor readings
- `zerone_alignment_corrections_total` — corrections applied

**Capture Defense:**
- `zerone_capture_flagged_domains_total` — number of flagged domains
- `zerone_capture_hhi{domain}` — HerfindahlIndex per domain

**Partnerships:**
- `zerone_partnerships_active_total` — active partnerships
- `zerone_partnerships_formation_rate` — formations per epoch

**Governance:**
- `zerone_governance_proposals_active{status}` — proposals by status
- `zerone_governance_emergency_active` — 1 if emergency halt active

**Vesting Rewards:**
- `zerone_vesting_total_minted` — cumulative ZRN minted
- `zerone_vesting_research_fund_balance` — research fund balance

### 2. Grafana dashboard

Create `monitoring/grafana/zerone-overview.json`:
- System health panel (alignment composite)
- Knowledge activity (claims, facts, rounds)
- Economic flow (minting, distribution, staking)
- Governance activity
- Capture defense alerts
- Node performance (block time, tx throughput)

### 3. Alerting rules

Create `monitoring/prometheus/alerts.yml`:
- `ZeroneAlignmentCritical` — composite health < 300,000 for 5min
- `ZeroneCaptureFlagged` — new domain flagged
- `ZeroneEmergencyHalt` — emergency halt activated
- `ZeroneBlockTimeSlow` — block time > 10s for 3 blocks
- `ZeroneSupplyAnomaly` — supply change > 2σ from expected

### 4. Docker compose with monitoring

Extend `docker-compose.yml` with Prometheus + Grafana:

```yaml
services:
  prometheus:
    image: prom/prometheus
    volumes:
      - ./monitoring/prometheus:/etc/prometheus
    ports:
      - "9091:9090"
  
  grafana:
    image: grafana/grafana
    volumes:
      - ./monitoring/grafana:/var/lib/grafana/dashboards
    ports:
      - "3000:3000"
```

## Acceptance Criteria

- [ ] All priority modules export Prometheus metrics
- [ ] Grafana dashboard shows all key metrics
- [ ] Alert rules fire correctly in test scenarios
- [ ] `docker-compose up` starts node + monitoring stack
- [ ] Metrics documentation in `docs/MONITORING.md`
