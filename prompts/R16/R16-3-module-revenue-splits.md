# R16-3 — Module-Level Revenue Split Updates

## Objective

Update every module that has its own revenue split or calls burn operations to use the new development fund pattern.

## Prerequisites

R16-1 complete (proto types available).

## Modules to Update

### 1. `x/toolbox` — Tool Revenue Split

**`x/toolbox/types/types.go`:**
```go
// DefaultRevenueSplit — update:
BurnBps → DevelopmentBps
100_000 → 196_700  // development
// Update ResearchBps if toolbox uses its own:
ResearchBps → 33_300
```

**`x/toolbox/types/types.go` — `ValidateParams()`:**
- Change: `revSum := p.ToolRevenueBps + p.ProtocolBps + p.ResearchBps + p.DevelopmentBps`

**`x/toolbox/keeper/revenue.go`:**
- Rename `burnAmount` → `developmentAmount`
- Replace `BurnCoins` call with `SendCoinsFromModuleToModule` to development_fund
- Update all comments referencing "burn"

**`x/toolbox/proto/genesis.proto`:**
- Already renamed in R16-1: `burn_bps` field 23 → `development_bps`

### 2. `x/billing` — Query Billing Revenue Split

**`x/billing/types/types.go`:**
```go
// DefaultRevenueSplit:
BurnBps → DevelopmentBps: 196700
ResearchBps: 33300
```

**`x/billing/types/types.go` — validation:**
- Change sum check: `ContributorBps + ProtocolBps + ResearchBps + DevelopmentBps`

**`x/billing/keeper/distribution.go`:**
- Replace any burn logic with development fund deposit
- Update expected_keepers if BurnCoins interface is used

### 3. `x/tree` — Project Tree Revenue Split

**`x/tree/keeper/msg_server.go`:**
- `burn_bp` → `development_bp` in revenue routing
- Replace burn with development fund deposit

**`x/tree/types/expected_keepers.go`:**
- Remove `BurnCoins` from BankKeeper interface if present
- Add `SendCoinsFromModuleToModule` if not present

### 4. `x/knowledge` — Verification Slashing

**`x/knowledge/keeper/rounds.go`:**
- Any slashed tokens that were burned should go to development fund
- Check: does knowledge burn tokens directly or delegate to vesting_rewards?

**`x/knowledge/types/expected_keepers.go`:**
- Remove `BurnCoins` from expected keepers if present

### 5. `x/disputes` — Dispute Resolution

**`x/disputes/keeper/keeper.go`:**
- Slashed tokens: route to development fund instead of burning
- Update expected_keepers

### 6. `x/staking` — Validator Slashing

**`x/staking/keeper/keeper.go`:**
- Slashing currently may burn tokens. Route to development fund instead.
- Check: does staking module have its own burn logic or use SDK's?

### 7. `x/liquiditypool` — Swap Fees

**`x/liquiditypool/keeper/msg_server.go`:**
- Protocol fee routing: any burn portion → development fund
- Update expected_keepers

### 8. `x/capture_challenge` — Challenge Resolution

**`x/capture_challenge/keeper/msg_server.go`:**
- Failed challenge stake: route to development fund instead of burning

### 9. `x/partnerships` — Exit Penalties

**`x/partnerships/keeper/exit.go`:**
- Exit penalty tokens: route to development fund instead of burning

### 10. `x/tokens` — Token Operations

**`x/tokens/keeper/msg_server.go`:**
- ZRN-20 token burn operations: these are USER-INITIATED burns of secondary tokens, not protocol burns. **Leave these alone** — users can burn their own ZRN-20 tokens if they want.
- Only change: protocol revenue routing if tokens module has its own split

### 11. `x/research` — Research Slashing

**`x/research/keeper/msg_server.go`:**
- Rejected research slash: route to development fund

## Expected Keepers Audit

Every module's `expected_keepers.go` that includes `BurnCoins` in the BankKeeper interface:

```bash
grep -rn "BurnCoins" x/*/types/expected_keepers.go
```

For each hit:
- If the module uses it for protocol revenue burn → remove, use SendCoinsFromModuleToModule
- If the module uses it for user-initiated burns (x/tokens) → keep

## Verification

```bash
# No protocol-level BurnCoins in any module keeper (except x/tokens for user burns)
grep -rn "BurnCoins" x/*/keeper/*.go | grep -v _test.go | grep -v pb.go | grep -v tokens/
# Should be 0 (or only development_fund fallback paths)

# No BurnBps/burn_bps in any module
grep -rn "BurnBps\|burn_bps" x/ --include="*.go" | grep -v pb.go | grep -v _test.go
# Should be 0

# Build
go build ./...
```

## Commit

```
R16-3: update all modules — burn → development fund routing

Modules updated: toolbox, billing, tree, knowledge, disputes, staking,
liquiditypool, capture_challenge, partnerships, research.
All protocol-level BurnCoins calls replaced with development_fund deposits.
User-initiated burns (x/tokens ZRN-20) preserved.
Revenue split defaults updated to 55/22/19.67/3.33 across all modules.
Expected keepers updated: BurnCoins removed where no longer needed.
```
