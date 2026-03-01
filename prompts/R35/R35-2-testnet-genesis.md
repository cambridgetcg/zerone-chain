# R35-2 — Testnet Genesis Generation

## Objective

Generate a production-quality testnet genesis with tuned parameters, initial validator set, faucet configuration, and explorer compatibility.

## Tasks

### 1. Parameter tuning

Review and set every module's params for testnet:

**Knowledge:**
- DomainBaseCapacity: 100 (low for testing)
- Verification round timing: 30 blocks commit, 30 blocks reveal
- MinVerifiers: 2 (lower than mainnet for easier testing)

**Governance:**
- Voting period: 2 minutes (fast iteration)
- Min deposit: 10 ZRN (accessible for testers)
- Quorum: 33%

**Vesting Rewards:**
- Block reward: 10 ZRN/block (generous for testnet)
- Revenue split: same as mainnet (test the real percentages)

**Partnerships:**
- Formation cooldown: 10 blocks (fast for testing)
- Min stake: 100 ZRN

**All modules:** Document the testnet value AND the planned mainnet value.

### 2. Initial validator set

- Create 3 genesis validators (operated by Yu/AI)
- Set initial stake: 1,000,000 ZRN each
- Generate gentx files
- Configure persistent peers

### 3. Faucet

- Create a faucet account with 100,000,000 ZRN
- Deploy faucet service (simple HTTP endpoint)
- Rate limit: 100 ZRN per request, 1 request per hour per IP

### 4. Explorer configuration

- Ensure genesis compatible with ping.pub / Mintscan
- Generate chain.json for cosmos/chain-registry format
- Configure REST/gRPC endpoints for explorer access

### 5. Genesis validation

- Run `tools/genesis-check` on the generated genesis
- Run genesis round-trip test (import → export → compare)
- Verify all 32 modules initialize from genesis
- Run 100 blocks and verify no panics

### 6. Testnet configuration files

Create `testnet/`:
- `genesis.json` — the genesis file
- `config.toml.template` — CometBFT config
- `app.toml.template` — app config
- `peers.txt` — persistent peer addresses
- `README.md` — how to join the testnet

## Acceptance Criteria

- [ ] All module params documented (testnet vs mainnet values)
- [ ] Genesis validates with `tools/genesis-check`
- [ ] Chain starts from genesis and runs 100+ blocks
- [ ] Faucet service functional
- [ ] Chain registry JSON generated
- [ ] `testnet/` directory complete and documented
