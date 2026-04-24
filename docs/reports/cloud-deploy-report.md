# Cloud Deployment Simulation Report

**Date:** 2026-02-26
**Chain:** zerone-localnet
**Method:** Docker container (Ubuntu 22.04) as simulated VPS
**Total simulation time:** 33 seconds
**Result:** All phases PASS (8 bugs/discrepancies found)

---

## Executive Summary

Simulated a new operator joining the Zerone testnet from a fresh Ubuntu 22.04 VPS. The Docker container connected to a host-side 4-validator localnet, synced blocks, and validated the full operator workflow including installation, configuration, security hardening, and monitoring setup.

**Key findings:**
- Pre-built binary installation is fast (1s) and works perfectly
- Node syncs and catches up to the network within 6 seconds
- configure-node.sh works but is missing two critical app.toml patches
- VALIDATOR-GUIDE.md has several outdated references
- PRODUCTION-STACK.md is solid for mainnet but missing SDK v0.50-specific notes

---

## Phase Results

### Phase 1: VPS Creation (8s) -- PASS

- Ubuntu 22.04 container created via `docker create`
- Base packages installed: curl, jq, ca-certificates, procps, iproute2
- Container connected to host via `host.docker.internal`

### Phase 2: Installation Methods

| Method | Status | Duration | Notes |
|--------|--------|----------|-------|
| **A: Pre-built binary** | PASS | 1s | Copy + chmod. 95MB statically linked binary. Recommended. |
| **B: Docker image** | SKIPPED | -- | Docker-in-Docker not practical. Validated via host `docker build` separately. |
| **C: Build from source** | SKIPPED | ~5-10 min | Ubuntu 22.04 ships Go 1.18 (too old). Must install Go 1.24+ from go.dev manually. |

**Recommendation:** Pre-built binary is the clear winner for testnet operators. Build-from-source requires manual Go installation since Ubuntu apt packages are too old.

### Phase 3: Node Configuration (1s) -- PASS

| Step | Status | Notes |
|------|--------|-------|
| `zeroned init` | PASS | Creates default config at ~/.zeroned |
| Genesis copy | PASS | Copied from localnet coordinator |
| `configure-node.sh` | PASS | Applied validator mode with API/gRPC/Prometheus |
| Persistent peers | PASS | Set to val0@host.docker.internal:26600 |
| Manual patches | REQUIRED | addr_book_strict, max-txs, iavl-disable-fastnode needed separately |

**Bug found:** configure-node.sh does not set `max-txs` or `iavl-disable-fastnode` -- see Bug #3, #4 below.

### Phase 4: Node Sync (17s) -- PASS

- Node started with `zeroned start --minimum-gas-prices 1uzrn`
- Reached height 112 after 6s (1 peer connected)
- Block advancement confirmed: 112 -> 118 over 10s
- Node caught up to network (catching_up: false by end of simulation)

### Phase 5: Systemd Service (0s) -- PASS (validated only)

Generated and validated service file structure:
- ExecStart present and correct
- Restart=always with RestartSec=3
- LimitNOFILE=65535
- After=network-online.target dependency

**Caveat:** Cannot test systemd start/stop in Docker (no init system). Requires real VPS verification.

### Phase 6: Security Hardening (6s) -- PASS (limited)

| Tool | Install | Functional Test | Notes |
|------|---------|-----------------|-------|
| ufw | PASS | LIMITED | Rules configured but cannot activate in Docker (needs iptables) |
| fail2ban | PASS | SKIPPED | Requires sshd and syslog |

**Listening ports observed:**
- `127.0.0.1:26657` -- RPC (localhost only, correct)
- `127.0.0.1:9090` -- gRPC (localhost only, correct)
- `127.0.0.1:1317` -- REST API (localhost only, correct)
- `0.0.0.0:26656` -- P2P (public, correct)
- `0.0.0.0:26660` -- Prometheus (public, intentional)
- `127.0.0.1:6060` -- pprof debug (localhost only, correct)

**Security observation:** RPC, gRPC, and API correctly bind to localhost only. P2P and Prometheus are the only public-facing ports, which matches PRODUCTION-STACK.md recommendations.

### Phase 7: Monitoring (0s) -- PASS

| Component | Status | Notes |
|-----------|--------|-------|
| Health check script | PASS | Reports height, sync status, peers, last block time |
| Health check JSON | PASS | Valid JSON output with `--json` flag |
| Prometheus metrics | PASS | 468 metric lines available at :26660/metrics |
| Cron setup | DOCUMENTED | `*/5 * * * * zerone-health.sh >> /var/log/zerone-health.log` |

### Phase 8: PRODUCTION-STACK.md Validation

| Recommendation | Testnet Verdict | Notes |
|---|---|---|
| Validator behind sentry nodes | OVERKILL | Direct P2P is fine for testnet |
| No public IP on validator | OVERKILL | Public IP needed for direct peering |
| 8 vCPU / 32GB RAM | OVERKILL | 2 vCPU / 4GB RAM sufficient for testnet |
| 1TB NVMe | OVERKILL | 50GB SSD is plenty for months |
| Horcrux threshold signing | OVERKILL | Local key signing is fine |
| Cosmovisor | CORRECT | Useful even on testnet for upgrade testing |
| Prometheus monitoring | CORRECT | Essential for debugging |
| systemd service | CORRECT | Production must-have |
| ufw firewall (22, 26656) | CORRECT | Basic security even on testnet |
| fail2ban | NICE TO HAVE | Protects SSH but not critical |
| RPC/API behind Cloudflare | OVERKILL | Direct access fine for testnet |
| minimum-gas-prices = 0.025uzrn | CORRECT | Matches docs |
| Block time 2521ms | CORRECT | Matches configure-node.sh defaults |
| LimitNOFILE=65535 | CORRECT | Prevents fd exhaustion |

**Missing from PRODUCTION-STACK.md:**
1. No mention of `max-txs` mempool fix (SDK v0.50 defaults to NoOpMempool)
2. No mention of `iavl-disable-fastnode` setting
3. No mention of `addr_book_strict` for initial bootstrapping
4. No mention of `vote_extensions_enable_height` requirement
5. No quick-start section for testnet (only mainnet architecture)
6. Security checklist mentions disk encryption but gives no instructions

---

## Bugs & Discrepancies Found

### Bug 1: join-testnet.sh hardcodes chain-id
- **File:** `scripts/join-testnet.sh:29`
- **Issue:** `CHAIN_ID="zerone-testnet-1"` is hardcoded. Cannot be used for localnet or other chain IDs.
- **Impact:** Operators testing locally must modify script. Not critical for production.
- **Fix:** Add `--chain-id` flag. Deferred -- join-testnet.sh is specifically for the testnet.

### Bug 2: join-testnet.sh uses deprecated validate-genesis
- **File:** `scripts/join-testnet.sh:150`
- **Issue:** `zeroned validate-genesis` is deprecated in SDK v0.50. Should be `zeroned genesis validate`.
- **Impact:** Script has fallback but primary command fails, causing confusing stderr output.
- **Fix:** Swap primary and fallback commands. **FIXED.**

### Bug 3: configure-node.sh missing max-txs patch
- **File:** `scripts/configure-node.sh`
- **Issue:** Does not set `max-txs = 5000` in app.toml. SDK v0.50 defaults `max-txs = -1` (NoOpMempool), which silently drops all transactions.
- **Impact:** CRITICAL -- new operators' nodes will not process any transactions.
- **Fix:** Add `max-txs` patch to app.toml section. **FIXED.**

### Bug 4: configure-node.sh missing iavl-disable-fastnode
- **File:** `scripts/configure-node.sh`
- **Issue:** Does not set `iavl-disable-fastnode = true`. Default causes "version does not exist" query errors.
- **Impact:** MODERATE -- queries fail intermittently after node restart.
- **Fix:** Add `iavl-disable-fastnode` patch. **FIXED.**

### Bug 5: VALIDATOR-GUIDE.md wrong Go version
- **File:** `docs/VALIDATOR-GUIDE.md:32`
- **Issue:** Says "Go 1.22+" but go.mod requires Go 1.24+.
- **Impact:** Operators install wrong Go version, build fails.
- **Fix:** Update to "Go 1.24+". **FIXED.**

### Bug 6: VALIDATOR-GUIDE.md deprecated validate-genesis
- **File:** `docs/VALIDATOR-GUIDE.md:142`
- **Issue:** Uses `zeroned validate-genesis` instead of `zeroned genesis validate`.
- **Impact:** Command fails, confusing new operators.
- **Fix:** Update command. **FIXED.**

### Bug 7: VALIDATOR-GUIDE.md missing mempool warning
- **File:** `docs/VALIDATOR-GUIDE.md`
- **Issue:** No mention of the `max-txs = -1` (NoOpMempool) default or how to fix it.
- **Impact:** Operators set up nodes that silently drop all transactions.
- **Fix:** Add troubleshooting entry. **FIXED.**

### Bug 8: VALIDATOR-GUIDE.md knowledge params mismatch
- **File:** `docs/VALIDATOR-GUIDE.md:327-330`
- **Issue:** Says "Commit phase (4 blocks)" and "Reveal phase (4 blocks)" but testnet genesis uses 10/10/5.
- **Impact:** Misleading but not functionally harmful (params come from genesis, not docs).
- **Fix:** Update to match genesis defaults. **FIXED.**

---

## Simulation Caveats

The following need real VPS verification:

1. **systemd** -- service start/stop/restart behavior
2. **ufw** -- actually blocking traffic (Docker lacks iptables permissions)
3. **fail2ban** -- banning IPs after SSH brute force
4. **Disk I/O** -- NVMe vs SSD performance under sustained block production
5. **Network latency** -- cross-region peer sync times
6. **Cosmovisor** -- actual upgrade mechanics with governance proposal
7. **Memory usage** -- long-running stability with growing state

---

## Recommendations

1. **Pre-built binary** is the recommended installation method for testnet operators
2. **configure-node.sh** should be the single source of truth for node configuration
3. **Testnet quick-start guide** should be extracted from PRODUCTION-STACK.md as a separate section
4. All scripts should be tested on real Ubuntu 22.04 VPS before public testnet launch
5. Consider adding a `--dry-run` mode to scripts for validation without side effects
