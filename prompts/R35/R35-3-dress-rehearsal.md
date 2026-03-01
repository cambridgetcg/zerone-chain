# R35-3 — Dress Rehearsal

## Objective

Full dress rehearsal of the testnet launch: genesis ceremony simulation, chain start, 1000 blocks of operation, software upgrade, IBC connection, and genesis export. Everything that will happen in production, done once in rehearsal.

## Tasks

### 1. Genesis ceremony simulation

- Generate validator keys for 3 validators
- Each validator creates a gentx
- Collect gentxs into genesis
- Validate combined genesis
- Distribute genesis to all validators

### 2. Coordinated chain start

- Start all 3 validators simultaneously (docker-compose)
- Verify first block produced
- Verify all validators signing
- Record chain-id, genesis hash, first block hash

### 3. Operational lifecycle (1000 blocks)

Over 1000 blocks, execute the full feature set:

| Block range | Activity |
|---|---|
| 1-50 | Chain stabilization, basic transfers |
| 51-100 | Faucet distribution, account creation |
| 101-200 | Knowledge: submit 10 claims, verify 5, create 3 facts |
| 201-300 | Partnerships: form 3 partnerships, 1 mentorship |
| 301-400 | Governance: submit and pass 2 proposals |
| 401-500 | Defense: trigger and resolve capture detection |
| 501-600 | Economic: verify reward distribution, delegation |
| 601-700 | Alignment: observe health metrics, trigger correction |
| 701-800 | IBC: connect to counterparty, transfer ZRN both ways |
| 801-900 | Stress: burst of 50 txs/block |
| 901-1000 | Final: export genesis, verify round-trip |

### 4. Software upgrade rehearsal

- At block 500, submit upgrade proposal
- Vote and pass
- At upgrade height, swap binary via Cosmovisor
- Verify chain continues with new binary
- Verify no state loss

### 5. Emergency halt rehearsal

- At block 750, trigger emergency halt
- Advance ceremony to completion
- Resume chain
- Verify normal operation continues

### 6. Final verification

- Export genesis at block 1000
- Verify all module state present
- Compare economic totals (minted, distributed, staked)
- Generate testnet health report

## Acceptance Criteria

- [ ] Genesis ceremony completes without manual intervention
- [ ] 3-validator network runs 1000 blocks
- [ ] All feature areas exercised (knowledge, governance, partnerships, etc.)
- [ ] Software upgrade succeeds mid-chain
- [ ] Emergency halt and recovery works
- [ ] Genesis export at block 1000 is valid and re-importable
- [ ] Health report generated with all metrics
