# Zerone Testnet Launch Checklist

Chain ID: `zerone-testnet-1`

---

## Pre-Launch

### Code & Build

- [ ] All modules compile: `go build ./...`
- [ ] All unit tests pass: `go test ./...`
- [ ] Cross-stack integration tests pass
- [ ] Multi-validator tests pass (4-node local cluster)
- [ ] IBC relay tests pass (hermes/rly)
- [ ] Simulation tests pass (1000+ blocks)
- [ ] Race condition check: `go test -race ./...`
- [ ] Binary reproducible: same SHA across builds

### Genesis Preparation

- [ ] Run `zeroned init` on each validator node
- [ ] Run `zeroned prepare-genesis zerone-testnet-1` with all addresses
- [ ] Verify 777 axioms injected into knowledge module
- [ ] Verify genesis circulating supply is **0 ZRN** (zero team allocation; nothing has been minted yet — both emission pathways mint on demand)
- [ ] Verify bank supply equals sum of all balances (= 0 at genesis)
- [ ] Verify hard cap `MaxSupplyUzrn` = `222222222000000` (222,222,222 ZRN)
- [ ] Verify bootstrap pool whitelist + per-agent claim amount (`0.222 ZRN`) configured in `x/claiming_pot` genesis state
- [ ] Verify `x/vesting_rewards` mint pathway and `x/claiming_pot` mint pathway both gate through `MintWithCap`
- [ ] Distribute signed genesis.json to all validators
- [ ] SHA256 hash matches across all validators
- [ ] Genesis time set (coordinated UTC timestamp)

### Validator Coordination

- [ ] 4 genesis validators confirmed with addresses
- [ ] Each validator generates `gentx` with self-delegation
- [ ] All `gentx` files collected and added to genesis
- [ ] Persistent peer list compiled (node IDs + IPs)
- [ ] Seed node addresses shared
- [ ] Communication channel established (Discord/Telegram)

### Infrastructure

- [ ] Each validator: 4+ CPU cores, 16 GB RAM, 500 GB SSD
- [ ] Port 26656 (P2P) open on all validators
- [ ] Port 26657 (RPC) open on public-facing nodes
- [ ] Port 1317 (REST API) open on API nodes
- [ ] Port 9090 (gRPC) open on API nodes
- [ ] Cosmovisor installed and configured on all validators
- [ ] Monitoring stack deployed (Prometheus + Grafana)
- [ ] Log aggregation configured
- [ ] Backup strategy documented and tested

### Security

- [ ] Validator keys backed up securely offline
- [ ] No validator runs duplicate signing keys
- [ ] Sentry node architecture for public validators
- [ ] Firewall rules reviewed
- [ ] DDoS protection in place
- [ ] Key management documented

### Documentation

- [ ] README.md updated with testnet info
- [ ] Validator Guide reviewed and tested end-to-end
- [ ] Parameter Reference complete for all 35 parameter-bearing modules (38 total; 3 pure synthesisers carry no params)
- [ ] FAQ covers common validator questions
- [ ] API documentation (Swagger) deployed

---

## Launch Day

### Sequence

1. [ ] All validators confirm genesis.json hash match
2. [ ] Set `--halt-height 0` (no premature halt)
3. [ ] Start all validators within the genesis time window
4. [ ] Verify first block produced
5. [ ] Verify all 4 validators signing blocks
6. [ ] Announce testnet live

### Health Checks (First Hour)

- [ ] Blocks producing at ~2.5s intervals
- [ ] All 4 validators active and signing
- [ ] No missed blocks in first 100 blocks
- [ ] RPC endpoints responding
- [ ] REST API endpoints responding
- [ ] P2P peer count stable (each node sees 3+ peers)

### First Transactions

- [ ] Send a transfer between two accounts
- [ ] Register a new validator (tier: Apprentice)
- [ ] Submit a knowledge claim
- [ ] Complete a verification round (commit + reveal)
- [ ] Query knowledge module state
- [ ] Verify gas-free bootstrap transactions work

---

## Post-Launch

### Bootstrap Period Monitoring (First 14 Days)

- [ ] Gas-free transactions working for allowed types
- [ ] Block rewards distributing correctly (10 ZRN/block)
- [ ] Verification rewards working (3 ZRN per correct verification)
- [ ] Validator tiers computing correctly
- [ ] No unexpected slashing events
- [ ] Autopoiesis SSI tracking enabled and reporting

### Economic Health

- [ ] Token distribution matches genesis allocations
- [ ] Research fund balance correct
- [ ] Claiming pots balance correct
- [ ] Fee distribution working (93% validators, 7% research fund)
- [ ] Vesting schedules active (if applicable)
- [ ] No unexpected minting or burning

### Knowledge System

- [ ] 777 genesis axioms queryable
- [ ] New claims can be submitted and verified
- [ ] Confidence scoring working
- [ ] Citation economics distributing correctly
- [ ] Cross-domain references tracked
- [ ] Adversarial verification system functional

### Incident Response

- [ ] Emergency halt procedure tested (offline, not on live chain)
- [ ] Validator communication channel active 24/7 first week
- [ ] Upgrade procedure documented (via Cosmovisor)
- [ ] State export/import tested
- [ ] Rollback procedure documented

---

## Post-Bootstrap (After Block 480,000)

- [ ] Gas fees activate for all transaction types
- [ ] Minimum gas price enforced (0.025 uzrn)
- [ ] Fee market functioning
- [ ] Governance proposals can be submitted
- [ ] Parameter changes can be proposed and voted on
