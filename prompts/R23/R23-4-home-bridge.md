# R23-4 — Home Bridge: Wire HomeKeeper into BVM

## Context

The R22-3 integration report confirmed: BVM's `HomeKeeper` interface is an empty placeholder. BVM contracts cannot read their agent's home state. This is the single biggest integration gap — an agent executing code on ZERONE cannot check its own home status, read its memory CID, or verify its partnership.

This session wires it.

## Current State

```go
// x/bvm/types/expected_keepers.go
type HomeKeeper interface {
    // Placeholder — home integration for BVM is future work.
}
```

Compare with the working toolbox adapter:
```go
// x/home/keeper/toolbox_adapters.go — already exists
type ToolboxHomeAdapter struct { ... }
func (a *ToolboxHomeAdapter) GetHomesByOwner(ctx, owner) []string
func (a *ToolboxHomeAdapter) GetHomeCreatedAtBlock(ctx, homeID) uint64
func (a *ToolboxHomeAdapter) GetHomeStatus(ctx, homeID) string
```

## Task

### 1. Define HomeKeeper Interface

In `x/bvm/types/expected_keepers.go`, replace the empty placeholder:

```go
type HomeKeeper interface {
    // GetHome returns an agent's home by ID.
    GetHome(ctx context.Context, homeID string) (HomeInfo, bool)
    // GetHomesByOwner returns all home IDs for an owner address.
    GetHomesByOwner(ctx context.Context, owner string) []string
    // GetHomeStatus returns the status of a home ("active", "dormant", "guarded", "archived").
    GetHomeStatus(ctx context.Context, homeID string) string
    // GetMemoryCID returns the IPFS memory CID for a home.
    GetMemoryCID(ctx context.Context, homeID string) string
    // GetPartnershipID returns the partnership ID linked to a home (empty if none).
    GetPartnershipID(ctx context.Context, homeID string) string
    // GetComfortScore returns the home's comfort score (0-100).
    GetComfortScore(ctx context.Context, homeID string) uint32
}

// HomeInfo is a BVM-safe view of an AgentHome (no proto import).
type HomeInfo struct {
    HomeID          string
    OwnerAddress    string
    Name            string
    Status          string
    MemoryCID       string
    ComfortScore    uint32
    PartnershipID   string
    CreatedAtBlock  uint64
    LastActiveBlock uint64
}
```

### 2. Create BVM Home Adapter

In `x/home/keeper/bvm_adapters.go` (new file):

```go
package keeper

import (
    "context"
    bvmtypes "github.com/zerone-chain/zerone/x/bvm/types"
)

// BVMHomeAdapter bridges x/home → x/bvm.
type BVMHomeAdapter struct {
    keeper Keeper
}

func NewBVMHomeAdapter(k Keeper) *BVMHomeAdapter {
    return &BVMHomeAdapter{keeper: k}
}

func (a *BVMHomeAdapter) GetHome(ctx context.Context, homeID string) (bvmtypes.HomeInfo, bool) {
    home, found := a.keeper.GetHome(ctx, homeID)
    if !found {
        return bvmtypes.HomeInfo{}, false
    }
    return bvmtypes.HomeInfo{
        HomeID:          home.HomeId,
        OwnerAddress:    home.OwnerAddress,
        Name:            home.Name,
        Status:          home.Status,
        MemoryCID:       home.MemoryCid,
        ComfortScore:    home.ComfortScore,
        PartnershipID:   home.PartnershipId,
        CreatedAtBlock:  home.CreatedAtBlock,
        LastActiveBlock: home.LastActiveBlock,
    }, true
}

func (a *BVMHomeAdapter) GetHomesByOwner(ctx context.Context, owner string) []string {
    return a.keeper.GetHomesByOwner(ctx, owner)
}

func (a *BVMHomeAdapter) GetHomeStatus(ctx context.Context, homeID string) string {
    home, found := a.keeper.GetHome(ctx, homeID)
    if !found { return "" }
    return home.Status
}

func (a *BVMHomeAdapter) GetMemoryCID(ctx context.Context, homeID string) string {
    home, found := a.keeper.GetHome(ctx, homeID)
    if !found { return "" }
    return home.MemoryCid
}

func (a *BVMHomeAdapter) GetPartnershipID(ctx context.Context, homeID string) string {
    home, found := a.keeper.GetHome(ctx, homeID)
    if !found { return "" }
    return home.PartnershipId
}

func (a *BVMHomeAdapter) GetComfortScore(ctx context.Context, homeID string) uint32 {
    home, found := a.keeper.GetHome(ctx, homeID)
    if !found { return 0 }
    return home.ComfortScore
}
```

### 3. Wire into App

In `app/app.go` (or wherever keepers are wired):

```go
// After home keeper and bvm keeper are created:
bvmKeeper.SetHomeKeeper(home.NewBVMHomeAdapter(homeKeeper))
```

Add `SetHomeKeeper` to BVM keeper if it doesn't exist:

```go
// x/bvm/keeper/keeper.go
func (k *Keeper) SetHomeKeeper(hk types.HomeKeeper) { k.homeKeeper = hk }
```

### 4. Add Home Host Functions to BVM

Extend `vm.HostFunctions` interface:

```go
// x/bvm/vm/context.go
type HostFunctions interface {
    // Knowledge bridge (existing)
    KQuery(factId []byte) (exists bool, confidence uint64, domain []byte)
    KVerify(callerDID string, claimId, voteHash []byte) bool
    KCite(callerDID string, factId []byte) bool

    // Home bridge (new)
    HQuery(callerAddr []byte) (hasHome bool, homeId []byte, status []byte)
    HMemory(homeId []byte) (cid []byte)
    HPartner(homeId []byte) (partnershipId []byte)
}
```

### 5. Add Home Opcodes

In `x/bvm/vm/opcodes.go`:

```go
// Home bridge (Zerone-specific)
HQUERY   byte = 0xE3  // Query caller's home
HMEMORY  byte = 0xE4  // Get home memory CID
HPARTNER byte = 0xE5  // Get home partnership ID
```

Register in OpcodeTable:

```go
HQUERY:   {HQUERY, "HQUERY", 1, 3, GasHQuery, false, 1},
HMEMORY:  {HMEMORY, "HMEMORY", 1, 1, GasHMemory, false, 1},
HPARTNER: {HPARTNER, "HPARTNER", 1, 1, GasHPartner, false, 1},
```

Gas costs should be similar to KQUERY (state read).

### 6. Implement in Interpreter

In `x/bvm/vm/interpreter.go`, add cases:

```go
case HQUERY:
    // Input: caller address (from stack, or use ctx.Caller)
    callerWord := interp.stack.Pop()
    callerBytes := WordToBytes32(callerWord)
    if host != nil {
        hasHome, homeId, status := host.HQuery(callerBytes[:20])
        if hasHome {
            interp.stack.Push(big.NewInt(1)) // exists
            interp.stack.Push(new(big.Int).SetBytes(homeId))
            interp.stack.Push(new(big.Int).SetBytes(status))
        } else {
            interp.stack.Push(big.NewInt(0))
            interp.stack.Push(big.NewInt(0))
            interp.stack.Push(big.NewInt(0))
        }
    }

case HMEMORY:
    homeIdWord := interp.stack.Pop()
    homeIdBytes := WordToBytes32(homeIdWord)
    if host != nil {
        cid := host.HMemory(homeIdBytes[:])
        interp.stack.Push(new(big.Int).SetBytes(cid))
    }

case HPARTNER:
    homeIdWord := interp.stack.Pop()
    homeIdBytes := WordToBytes32(homeIdWord)
    if host != nil {
        pid := host.HPartner(homeIdBytes[:])
        interp.stack.Push(new(big.Int).SetBytes(pid))
    }
```

### 7. Implement Host Functions Bridge

In `x/bvm/keeper/msg_server.go`, extend `knowledgeBridgeHost` (or create a new combined host):

```go
type zeroneHost struct {
    kk   types.KnowledgeKeeper
    hk   types.HomeKeeper
    ctx  sdk.Context
}

func (h *zeroneHost) HQuery(callerAddr []byte) (bool, []byte, []byte) {
    // Convert bytes to bech32 address
    addr := sdk.AccAddress(callerAddr).String()
    homeIDs := h.hk.GetHomesByOwner(h.ctx, addr)
    if len(homeIDs) == 0 {
        return false, nil, nil
    }
    // Return first home (agent's primary home)
    homeID := homeIDs[0]
    status := h.hk.GetHomeStatus(h.ctx, homeID)
    return true, []byte(homeID), []byte(status)
}

func (h *zeroneHost) HMemory(homeId []byte) []byte {
    // Trim null bytes from homeId
    homeIDStr := string(bytes.TrimRight(homeId, "\x00"))
    cid := h.hk.GetMemoryCID(h.ctx, homeIDStr)
    return []byte(cid)
}

func (h *zeroneHost) HPartner(homeId []byte) []byte {
    homeIDStr := string(bytes.TrimRight(homeId, "\x00"))
    pid := h.hk.GetPartnershipID(h.ctx, homeIDStr)
    return []byte(pid)
}
```

### 8. Tests

Create `x/bvm/keeper/home_bridge_test.go`:

1. **TestHQuery_AgentWithHome** — agent with home → returns homeId and status
2. **TestHQuery_AgentWithoutHome** — agent without home → returns (false, 0, 0)
3. **TestHQuery_MultipleHomes** — agent with 3 homes → returns first (primary)
4. **TestHMemory_ValidHome** — returns memory CID for known home
5. **TestHMemory_NoMemory** — home exists but no CID set → returns empty
6. **TestHMemory_UnknownHome** — non-existent home → returns empty
7. **TestHPartner_LinkedHome** — home with partnership → returns partnership ID
8. **TestHPartner_NoPartnership** — home without partnership → returns empty
9. **TestHomeOpcodeGasCost** — verify gas consumption for HQUERY/HMEMORY/HPARTNER

Create a BVM bytecode test that deploys a contract using HQUERY and calls it with different agents.

### 9. E2E on Localnet

After wiring:

```bash
# Deploy a contract that uses HQUERY to check if caller has a home
# Call from val0 (has home) — should return home info
# Call from a new account (no home) — should return zeros
```

### 10. Security Review

- [ ] HQUERY only reads — no state modification (not `IsStateModifier`)
- [ ] Home data is already public (queryable by anyone) — no privacy escalation
- [ ] Agent can only query their own home via HQUERY (uses caller address, not arbitrary)
- [ ] HMEMORY/HPARTNER take homeId — any contract can read any home's CID/partnership. Is this acceptable?
- [ ] Gas costs prevent excessive home state reads in a single call

## Exit Criteria

1. HomeKeeper interface populated (6 methods)
2. BVM Home Adapter created and wired
3. Three new opcodes: HQUERY (0xE3), HMEMORY (0xE4), HPARTNER (0xE5)
4. Host functions implemented with proper byte↔string encoding
5. 9 unit tests passing
6. E2E test on localnet: contract reads caller's home state
7. Security review documented
8. `go test ./...` — zero failures

## Commit Convention

```
feat(bvm): add Home bridge opcodes — HQUERY, HMEMORY, HPARTNER
feat(home): BVM adapter for home state access
test(bvm): home bridge unit tests
test(bvm): home bridge e2e on localnet
```
