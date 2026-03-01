# ZERONE Production Infrastructure

## Overview

Multi-provider cloud architecture for ZERONE mainnet. Designed for resilience, cost efficiency, and progressive scaling from testnet to production.

---

## Tier 1 — Validator Nodes (Consensus)

The validator is the heart of the network. It must never be directly exposed to the internet.

### Primary Validator
- **Spec:** 8 vCPU, 32GB RAM, 1TB NVMe
- **Provider:** Hetzner (Finland, AX41)
- **Cost:** ~€50/mo
- **Config:** No public IP. Communicates only with sentry nodes via private network.
- **Signing:** Local key or Horcrux threshold signing (2-of-3 across machines)

### Backup Validator
- **Spec:** 8 vCPU, 32GB RAM, 1TB NVMe
- **Provider:** OVH (France)
- **Cost:** ~€45/mo
- **Config:** Standby mode. Syncs blocks but does not sign unless primary fails.
- **Failover:** Manual promotion to avoid double-signing. Horcrux automates this if configured.

### Sentry Nodes (2-3)
- **Spec:** 4 vCPU, 16GB RAM, 500GB SSD each
- **Providers:** AWS eu-west-1, Hetzner DE, OVH (one per provider)
- **Cost:** ~€30/mo total (Hetzner), ~$75/mo (AWS)
- **Role:** Public-facing P2P relay. Shields validator from direct exposure and DDoS.
- **Config:** `private_peer_ids` includes validator node ID. `persistent_peers` includes each other + validator.

### Sentry Architecture

```
Public Internet (P2P port 26656)
       │
  ┌────┼────┐
  │    │    │
Sentry1  Sentry2  Sentry3
  │    │    │
  └────┼────┘
       │
   Private Network
       │
   Validator (no public IP)
```

---

## Tier 2 — Public Infrastructure

### RPC Nodes (2)
- **Spec:** 4 vCPU, 16GB RAM, 1TB SSD
- **Providers:** AWS eu-west-1 + AWS us-east-1
- **Cost:** ~$150/mo (reserved instances)
- **Ports exposed:** 26657 (CometBFT RPC), 1317 (REST API), 9090 (gRPC)
- **Pruning:** Default (keep recent 100 blocks + every 500th)
- **DNS:** `rpc.zerone.money` via Cloudflare load balancer

### Archive Node
- **Spec:** 8 vCPU, 64GB RAM, 4TB NVMe
- **Provider:** Hetzner (AX101)
- **Cost:** ~€90/mo
- **Config:** `pruning = "nothing"` — full history from genesis
- **Use:** Block explorer backend, historical queries, auditing
- **DNS:** `archive.zerone.money` (not public-facing by default)

### Load Balancer & CDN
- **Provider:** Cloudflare Pro ($20/mo)
- **Routes:**
  - `rpc.zerone.money` → RPC Node 1 + RPC Node 2 (round-robin)
  - `api.zerone.money` → REST/gRPC endpoint
  - `explorer.zerone.money` → Block explorer
- **Features:** DDoS protection, rate limiting, SSL termination

---

## Tier 3 — Supporting Services

### Block Explorer
- **Software:** Ping.pub (lightweight) or BigDipper (full-featured)
- **Hosting:** Same machine as archive node or separate small VPS
- **DNS:** `explorer.zerone.money`
- **Backend:** Points to archive node RPC

### Faucet (Testnet Only)
- **Software:** Custom Go service or cosmos-faucet
- **Hosting:** Colocated on RPC node
- **Rate limit:** 10 ZRN per address per 24h
- **DNS:** `faucet.zerone.money`

### Monitoring & Alerting
- **Metrics:** Prometheus (node_exporter + CometBFT /metrics endpoint)
- **Dashboards:** Grafana Cloud (free tier covers small deployments)
- **Key metrics:**
  - Block height + block time
  - Missed blocks (validator liveness)
  - Peer count
  - Disk usage, CPU, memory
  - Transaction throughput
- **Alerting:** Telegram bot or PagerDuty
  - Validator jailed → immediate alert
  - Node down > 2 min → alert
  - Disk > 80% → warning
  - Missed > 5 blocks in 100 → warning

### Snapshots
- **Provider:** Hetzner storage box (~€5/mo)
- **Schedule:** Daily state export + compressed archive
- **Purpose:** Fast bootstrap for new nodes joining the network
- **DNS:** `snapshots.zerone.money`

---

## Tier 4 — IBC Relayers

### Primary Relayer
- **Software:** Hermes (ibc-rs)
- **Spec:** 2 vCPU, 8GB RAM
- **Provider:** Hetzner
- **Channels:** Cosmos Hub (ATOM), Osmosis (OSMO, USDC)
- **Config:** Auto-clear packet backlog, gas price tuning per chain

### Backup Relayer
- **Software:** Hermes (same config)
- **Provider:** Different provider (OVH or AWS)
- **Purpose:** Redundancy — if primary goes down, packets still relay

---

## Network Topology (Full)

```
Internet
   │
   ├── Cloudflare (DDoS + DNS + SSL)
   │      │
   │      ├── rpc.zerone.money ──────→ RPC Node 1 (AWS eu-west-1)
   │      │                    ──────→ RPC Node 2 (AWS us-east-1)
   │      │
   │      ├── explorer.zerone.money ─→ Block Explorer (Hetzner)
   │      │
   │      ├── api.zerone.money ──────→ REST/gRPC (colocated with RPC)
   │      │
   │      └── faucet.zerone.money ───→ Faucet (testnet, on RPC node)
   │
   ├── P2P Network (port 26656)
   │      │
   │      ├── Sentry 1 (AWS) ──────┐
   │      ├── Sentry 2 (Hetzner) ──┼── Private ── Validator (Hetzner, no public IP)
   │      └── Sentry 3 (OVH) ─────┘
   │
   ├── Archive Node (Hetzner) ── full history, explorer backend
   │
   ├── Hermes Relayer 1 (Hetzner) ── IBC: Cosmos Hub, Osmosis
   └── Hermes Relayer 2 (OVH) ────── IBC backup
```

---

## Cost Estimate (Monthly)

| Item | Provider | Cost |
|------|----------|------|
| Primary validator (AX41) | Hetzner | €50 |
| Backup validator | OVH | €45 |
| Sentry nodes (2× CX31) | Hetzner | €30 |
| Sentry node (1× t3.medium) | AWS | ~$35 |
| RPC nodes (2× t3.xlarge reserved) | AWS | ~$150 |
| Archive node (AX101) | Hetzner | €90 |
| IBC relayers (2× CX21) | Hetzner + OVH | €20 |
| Cloudflare Pro | Cloudflare | $20 |
| Snapshot storage | Hetzner | €5 |
| Monitoring | Grafana Cloud free | $0 |
| **Total** | | **~$400-450/mo** |

---

## Provider Rationale

| Provider | Role | Why |
|----------|------|-----|
| **Hetzner** | Validators, archive, relayers | Best price/performance in EU, dedicated servers with NVMe |
| **AWS** | RPC, sentry | Global presence, reliability, reserved instance pricing |
| **OVH** | Backup validator, sentry | Geographic + provider diversity |
| **Cloudflare** | DNS, CDN, DDoS | Industry standard, free/cheap tier covers most needs |
| **Njalla** | Domain registration | Privacy WHOIS (already in use for zerone.money) |

**Key principle:** No single provider failure should halt the chain or make it unreachable.

---

## Progressive Scaling

### Phase 1 — Testnet (Now)
- 1 node (Mac Studio or cheap VPS)
- Local RPC
- Faucet optional
- **Cost:** $0-20/mo

### Phase 2 — Public Testnet
- 1 validator + 1 sentry + 1 RPC
- Block explorer
- Faucet
- **Cost:** ~$100/mo

### Phase 3 — Mainnet Launch
- Full stack as described above
- **Cost:** ~$400-450/mo

### Phase 4 — Growth
- Additional RPC nodes in Asia-Pacific
- Dedicated indexer (SubQuery or custom)
- WebSocket endpoint for real-time apps
- **Cost:** ~$600-800/mo

---

## Security Checklist

- [ ] Validator key never on a machine with public IP
- [ ] SSH key-only auth on all servers (no passwords)
- [ ] Firewall: only required ports open (26656 P2P, 26657 RPC where needed)
- [ ] Validator: only sentry IPs whitelisted
- [ ] Horcrux or similar threshold signing for validator key
- [ ] Automated security updates (unattended-upgrades)
- [ ] Disk encryption at rest
- [ ] Regular key rotation for SSH and service accounts
- [ ] Monitoring alerts tested monthly

---

## DNS Records (zerone.money)

| Record | Type | Value |
|--------|------|-------|
| `rpc` | A | Cloudflare proxy → RPC nodes |
| `api` | A | Cloudflare proxy → REST/gRPC |
| `explorer` | A | Cloudflare proxy → Explorer |
| `faucet` | A | Cloudflare proxy → Faucet |
| `archive` | A | Direct to archive node (internal use) |
| `snapshots` | A | Direct to snapshot server |
| `vault` | A | 80.78.19.135 (existing, AI signing vault) |

---

*Last updated: 2026-02-24*
*Status: Planning — implement at mainnet readiness*
