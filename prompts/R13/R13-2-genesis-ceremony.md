# R13-2 — Production Genesis Ceremony Script

## Context

Zerone needs a multi-step genesis ceremony script for coordinated mainnet/testnet launches. The prototype had `scripts/genesis-ceremony.sh` with subcommands: `init`, `add-validator`, `finalize`, `export`, `countdown`.

Zerone's `init-testnet.sh` is a bare-minimum single-validator setup. `localnet.sh` is for local development only. Neither is suitable for production launches.

## Task

### Create `scripts/genesis-ceremony.sh`

A bash script following the prototype's pattern but adapted for Zerone's binary (`zeroned`), denom (`uzrn`), and chain ID (`zerone-1`).

### Constants

```bash
CHAIN_ID="zerone-1"
GENESIS_TIME="2026-06-01T00:00:00Z"  # placeholder — set at ceremony time
DENOM="uzrn"
BINARY="build/zeroned"
CEREMONY_HOME="${HOME}/.zeroned/genesis-ceremony"

# Economics: NO pre-mine. Bootstrap only
FOUNDATION_BALANCE="10000000000000${DENOM}"   # 10,000,000 ZRN (10M)
RESEARCH_BALANCE="5000000000000${DENOM}"       #  5,000,000 ZRN  (5M)
FAUCET_BALANCE="500000000000${DENOM}"          #    500,000 ZRN (500K)
VALIDATOR_BALANCE="1000000000000${DENOM}"      #  1,000,000 ZRN  (1M)
VALIDATOR_STAKE="100000000000${DENOM}"          #    100,000 ZRN (100K)
```

### Subcommands

#### `init`

1. Build binary if not present
2. Clean previous ceremony state
3. `zeroned init genesis-coordinator --chain-id zerone-1 --default-denom uzrn`
4. Set genesis time
5. **Patch ALL consensus params:**
   - `block.max_gas = "33333333"`
   - `block.max_bytes = "4194304"`
   - `abci.vote_extensions_enable_height = "1"` (mandatory for PoT/VRF)
6. **Patch ALL module params** — defer to `testnet-genesis.sh` pattern (R13-3), but at minimum set critical production values for:
   - knowledge (PoT lifecycle)
   - zerone_gov (voting periods)
   - emergency (halt quorum)
   - zerone_staking (tier configs)
   - vesting_rewards (block rewards, decay)
7. Create bootstrap accounts: foundation, research-treasury, faucet
8. Print summary

#### `add-validator NAME`

1. Validate ceremony home exists
2. Generate unique consensus key in per-validator home
3. Copy coordinator genesis
4. Create validator account key in coordinator keyring
5. Fund validator in coordinator genesis
6. Copy updated genesis + keyring to validator home
7. Generate gentx with:
   - Commission rate: 10%
   - Max commission: 20%
   - Max commission change: 1%
8. Save consensus key + node key for distribution
9. Print address, stake, key locations

#### `finalize`

1. Collect all gentxs into coordinator
2. Run `zeroned validate-genesis` (or `validate`)
3. Print validator count, genesis hash

#### `export`

1. Copy genesis.json to project root
2. Print chain ID, genesis time, account count
3. Extract and print all validator node IDs from saved node keys
4. Print distribution instructions:
   - genesis.json → each validator's `~/.zeroned/config/genesis.json`
   - priv_validator_key.json → each validator
   - node_key.json → each validator
   - persistent_peers configuration

#### `countdown`

1. Parse genesis time from genesis.json
2. Print instructions ("All validators should start their nodes now")
3. Live countdown timer to genesis time
4. At genesis time: print block verification command

### Important Design Decisions

**Use `jq` for genesis patching** (not python3). The prototype used python3, but `jq` is more standard in the Cosmos ecosystem and already required by `localnet.sh`. Simpler, atomic patches:

```bash
patch() {
  jq "$1" "$GENESIS" > "${GENESIS}.tmp" && mv "${GENESIS}.tmp" "$GENESIS"
}
```

**Denom metadata** — include in genesis:
```json
{
  "description": "Zerone - the currency of verified truth",
  "denom_units": [
    {"denom": "uzrn", "exponent": 0, "aliases": ["microzerone"]},
    {"denom": "mzrn", "exponent": 3, "aliases": ["millizerone"]},
    {"denom": "zrn",  "exponent": 6, "aliases": ["zerone"]}
  ],
  "base": "uzrn",
  "display": "zrn",
  "name": "Zerone",
  "symbol": "ZRN"
}
```

**SDK module overrides:**
```bash
.app_state.staking.params.bond_denom = "uzrn"
.app_state.slashing.params.signed_blocks_window = "100"
.app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000"
```

### Error Handling

- `set -euo pipefail` everywhere
- `die()` helper for fatal errors
- Check binary exists before each subcommand
- Validate genesis after every mutation step (not just at the end)

### Axiom Injection Hook

After param patching in `init`, check for `tools/axiom-loader` and `x/knowledge/types/genesis_axioms.json`:

```bash
if command -v go &>/dev/null && [ -f "x/knowledge/types/genesis_axioms.json" ]; then
    info "Injecting 777 genesis axioms..."
    go run tools/axiom-loader/main.go inject \
        x/knowledge/types/genesis_axioms.json "$GENESIS"
fi
```

## Reference

- Prototype: `legible_money/scripts/genesis-ceremony.sh` (complete implementation)
- Zerone localnet: `scripts/localnet.sh` (validator setup pattern)
- Zerone join: `scripts/join-testnet.sh` (cosmovisor/systemd pattern)

## Verification

```bash
# Full ceremony flow
./scripts/genesis-ceremony.sh init
./scripts/genesis-ceremony.sh add-validator val1
./scripts/genesis-ceremony.sh add-validator val2
./scripts/genesis-ceremony.sh add-validator val3
./scripts/genesis-ceremony.sh finalize
./scripts/genesis-ceremony.sh export

# Verify genesis
zeroned validate-genesis --home $HOME/.zeroned/genesis-ceremony
```
