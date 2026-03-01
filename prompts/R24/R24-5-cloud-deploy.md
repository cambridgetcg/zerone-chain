# R24-5 — Cloud Deployment: Real VPS Validator

## Context

Everything so far runs on localhost. This session deploys a real validator node to a VPS, validating the production stack documentation and the full remote operator experience.

## Prerequisites

- R24-3 complete (join-testnet flow tested on localnet)
- R24-4 complete (Docker image or cross-compiled binary available)
- Access to a VPS (Hetzner, OVH, DigitalOcean, or similar)

**If no VPS is available:** This session can be done as a dry-run simulation using a local Docker container pretending to be a remote machine. Document the simulation approach and flag it for real-world verification later.

## Task

### 1. Provision VPS

**Recommended: Hetzner Cloud CX22 (~€4/mo) for testnet:**
- 2 vCPU, 4GB RAM, 40GB SSD
- Ubuntu 22.04 LTS
- Location: Finland (eu-central)

```bash
# If using Hetzner CLI:
hcloud server create \
    --name zerone-testnet-val1 \
    --type cx22 \
    --image ubuntu-22.04 \
    --location hel1 \
    --ssh-key <your-key>

# Note the IP address
VPS_IP=<ip>
```

**Or simulate with Docker:**
```bash
docker run -d --name zerone-vps \
    -p 26656:26656 -p 26657:26657 -p 1317:1317 \
    ubuntu:22.04 sleep infinity
docker exec -it zerone-vps bash
```

### 2. Install Dependencies

SSH into the VPS and set up:

```bash
ssh root@$VPS_IP

# System updates
apt-get update && apt-get upgrade -y
apt-get install -y curl jq wget unzip

# Option A: Install pre-built binary (from R24-4)
wget https://example.com/zeroned-linux-amd64 -O /usr/local/bin/zeroned
chmod +x /usr/local/bin/zeroned

# Option B: Install via Docker (from R24-4)
apt-get install -y docker.io
docker pull ghcr.io/zerone-chain/zerone:latest

# Option C: Build from source
apt-get install -y git make gcc
# Install Go
wget https://go.dev/dl/go1.24.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.24.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin:~/go/bin
# Clone and build
git clone https://codeberg.org/zerone-dev/zerone.git
cd zerone && make install
```

**Time each option. Document which is fastest and most reliable.**

### 3. Initialize and Configure

```bash
zeroned init "cloud-validator" --chain-id zerone-testnet-1

# Copy genesis (simulate testnet distribution)
# In real testnet: curl -L https://raw.githubusercontent.com/zerone-chain/networks/main/zerone-testnet-1/genesis.json > ~/.zeroned/config/genesis.json

# Configure node
scripts/configure-node.sh \
    --mode validator \
    --enable-api \
    --enable-grpc \
    --prometheus \
    --external-address "$VPS_IP:26656" \
    --moniker "cloud-validator"
```

**Verify:**
- [ ] Initialisation succeeds
- [ ] Config files generated correctly
- [ ] External address set (important for P2P discovery)

### 4. Set Up Systemd Service

```bash
# Generate service file
scripts/join-testnet.sh --systemd --home ~/.zeroned

# Install
sudo cp ~/.zeroned/zeroned.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable zeroned
sudo systemctl start zeroned
sudo systemctl status zeroned
```

**Verify:**
- [ ] Service starts
- [ ] Survives reboot (`systemctl enable`)
- [ ] Logs accessible via `journalctl -u zeroned -f`
- [ ] Auto-restart on crash

### 5. Set Up Cosmovisor (Alternative)

```bash
scripts/join-testnet.sh --cosmovisor --home ~/.zeroned

# Cosmovisor as systemd service
cat > /etc/systemd/system/zeroned.service <<EOF
[Unit]
Description=Zerone Node (Cosmovisor)
After=network.target

[Service]
User=root
Environment="DAEMON_NAME=zeroned"
Environment="DAEMON_HOME=/root/.zeroned"
Environment="DAEMON_ALLOW_DOWNLOAD_BINARIES=true"
Environment="DAEMON_RESTART_AFTER_UPGRADE=true"
ExecStart=$(which cosmovisor) run start --home /root/.zeroned
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable zeroned
sudo systemctl start zeroned
```

### 6. Security Hardening

Following `docs/infrastructure/PRODUCTION-STACK.md`:

```bash
# Firewall (ufw)
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 26656/tcp  # P2P
# Do NOT expose RPC (26657) or REST (1317) publicly for validators
ufw enable

# Fail2ban for SSH
apt-get install -y fail2ban
systemctl enable fail2ban

# Disable root SSH (create dedicated user)
adduser zerone
usermod -aG sudo zerone
# Copy SSH keys to zerone user
# Disable root login in /etc/ssh/sshd_config
```

**Verify:**
- [ ] Only port 22 (SSH) and 26656 (P2P) exposed
- [ ] RPC and REST not publicly accessible
- [ ] Node still syncs and participates via P2P

### 7. Monitoring Setup

```bash
# Basic monitoring: node height + peer count
cat > /usr/local/bin/zerone-health.sh <<'EOF'
#!/bin/bash
HEIGHT=$(curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height')
CATCHING_UP=$(curl -s http://localhost:26657/status | jq -r '.result.sync_info.catching_up')
PEERS=$(curl -s http://localhost:26657/net_info | jq -r '.result.n_peers')
echo "$(date -u '+%Y-%m-%d %H:%M:%S UTC') | Height: $HEIGHT | Catching up: $CATCHING_UP | Peers: $PEERS"
EOF
chmod +x /usr/local/bin/zerone-health.sh

# Cron: check every 5 minutes
echo "*/5 * * * * /usr/local/bin/zerone-health.sh >> /var/log/zerone-health.log" | crontab -
```

**If Prometheus is enabled:**
- [ ] Metrics available at `localhost:26660/metrics`
- [ ] Key metrics: `cometbft_consensus_height`, `cometbft_p2p_peers`, `cometbft_consensus_rounds`

### 8. Full Operator Walkthrough

Time the entire process from VPS creation to synced validator:

```
T+0:00  Provision VPS
T+?:??  Install binary
T+?:??  Initialize and configure
T+?:??  Start node
T+?:??  Node synced
T+?:??  Register as validator
T+?:??  First block signed
```

**Document:**
- [ ] Total time from zero to signing validator
- [ ] Total cost (VPS + any tooling)
- [ ] Steps that were confusing or underdocumented
- [ ] Steps that could be automated but aren't

### 9. Validate PRODUCTION-STACK.md

Read through the production stack document. For each recommendation:
- Is it correct for the testnet context?
- What's overkill for testnet but needed for mainnet?
- What's missing?

## Report Template

```markdown
### Phase N: <name>
**Time:** <minutes>
**Status:** PASS / FAIL / ISSUE
**Production Stack Match:** <accurate / inaccurate / missing>
**Observation:** <what happened>
**Issue:** <if any>
```

## Exit Criteria

1. Node deployed on real VPS (or Docker simulation documented)
2. Systemd service running and surviving restarts
3. Firewall configured (P2P only exposed)
4. Monitoring in place (health check script)
5. Total operator onboarding time documented
6. Every VALIDATOR-GUIDE and PRODUCTION-STACK discrepancy recorded
7. Report written to `docs/cloud-deploy-report.md`

## Cleanup

If using a paid VPS for testing:
```bash
# Destroy the test server when done
hcloud server delete zerone-testnet-val1
```

## Commit Convention

```
test(infra): cloud deployment validation — VPS + systemd + firewall
docs(infra): cloud deploy report — operator experience findings
fix(scripts): join-testnet.sh and configure-node.sh fixes from cloud testing
docs(validator): VALIDATOR-GUIDE corrections from real deployment
```
