# R15-1 — Wire Auth/DID → BVM Bridge

## Context

Zerone's BVM is identity-blind. The prototype's BVM has full auth integration:

- `authKeeper` on the BVM keeper — resolves addresses to DIDs, queries session capabilities
- `CallerDID` in execution context — BVM contracts know who's calling
- `SessionCapabilities` — restricts what session keys can do (transfer, stake, submit claims, vote)
- Capability inheritance — scheduled BVM executions inherit the scheduler's capabilities
- Secure defaults — anonymous callers denied all agent opcodes (C-1 pattern)

Zerone's BVM has `HostFunctions` that accept `callerDID string` as a parameter but nothing wires that from x/auth. The `expected_keepers.go` has no `AuthKeeper` interface.

## Task

### 1. Add AuthKeeper Interface to BVM

In `x/bvm/types/expected_keepers.go`, add:

```go
// SessionCapabilities defines what a session key is allowed to do within BVM.
// BVM-local copy to avoid cross-module type import from x/auth/types.
type SessionCapabilities struct {
    CanTransfer     bool
    CanStake        bool
    CanSubmitClaims bool
    CanVote         bool
}

// AuthKeeper defines the expected auth module keeper interface for BVM.
type AuthKeeper interface {
    // GetAccountDID resolves a bech32 address to its DID. Returns ("", false) if unknown.
    GetAccountDID(ctx context.Context, address string) (string, bool)
    // GetSessionCapabilities returns active session capabilities for an owner at a block height.
    GetSessionCapabilities(ctx context.Context, owner string, blockHeight uint64) (SessionCapabilities, bool)
}
```

### 2. Add authKeeper to BVM Keeper

In `x/bvm/keeper/keeper.go`:

```go
type Keeper struct {
    // ... existing fields ...
    authKeeper types.AuthKeeper
}

func (k *Keeper) SetAuthKeeper(ak types.AuthKeeper) { k.authKeeper = ak }
func (k Keeper) GetAuthKeeper() types.AuthKeeper { return k.authKeeper }
```

### 3. Add SessionCapabilities to VM ExecutionContext

In `x/bvm/vm/context.go`, add to `ExecutionContext`:

```go
type SessionCapabilities struct {
    CanTransfer     bool
    CanStake        bool
    CanSubmitClaims bool
    CanVote         bool
}

// In ExecutionContext:
CallerDID          string
Capabilities       *SessionCapabilities // nil = deny all agent opcodes (secure default)
```

### 4. Wire CallerDID Resolution in msg_server.go

In `CallContract` / `ExecuteContract` handlers:

```go
// Resolve caller DID
var callerDID string
if m.authKeeper != nil {
    if did, found := m.authKeeper.GetAccountDID(ctx, msg.Caller); found {
        callerDID = did
    }
}

// Load session capabilities
var caps *vm.SessionCapabilities
if m.authKeeper != nil && callerDID != "" {
    if sc, found := m.authKeeper.GetSessionCapabilities(ctx, msg.Caller, uint64(ctx.BlockHeight())); found {
        // Session key: use restricted capabilities
        caps = &vm.SessionCapabilities{
            CanTransfer:     sc.CanTransfer,
            CanStake:        sc.CanStake,
            CanSubmitClaims: sc.CanSubmitClaims,
            CanVote:         sc.CanVote,
        }
    } else {
        // Identity/operational key with no session key → full access
        caps = &vm.SessionCapabilities{
            CanTransfer: true, CanStake: true,
            CanSubmitClaims: true, CanVote: true,
        }
    }
}
// Anonymous caller (no DID): caps stays nil → all agent ops denied (C-1 secure default)
```

### 5. Wire Capability Inheritance for Scheduled Execution

In the schedule execution path (EndBlocker or schedule handler), when a scheduled BVM call fires:

- Look up the original scheduler's DID
- Look up their capabilities at current block height
- If session key expired: deny (capabilities expire with the key)
- Pass inherited capabilities to the VM execution context

### 6. Wire in app.go

Add the auth→BVM bridge in `app.go` keeper wiring:

```go
// After auth keeper and BVM keeper are both initialized:
app.BVMKeeper.SetAuthKeeper(authBVMAdapter)
```

Create adapter if needed (the x/auth keeper may not directly satisfy the BVM AuthKeeper interface — check the method signatures). Pattern: `zeronestakingkeeper.NewBVMStakingAdapter(...)` style.

### 7. Add Auth Adapter for BVM

Create the adapter that bridges x/auth keeper to BVM's expected interface. Check if `x/auth/keeper` already has:
- `GetAccountDID(ctx, address) (string, bool)` — maps to `GetAddressForDID` reverse lookup
- `GetSessionCapabilities(ctx, owner, height) (caps, bool)` — may need to be assembled from `IsSessionValid` + session key data

If the auth keeper doesn't have `GetAccountDID` directly, implement it:
```go
func (k Keeper) GetAccountDID(ctx sdk.Context, address string) (string, bool) {
    // Reverse lookup: iterate DID mappings to find one pointing to this address
    // Or maintain a reverse index address→DID
}
```

## Verification

```bash
go build ./...
go test ./x/bvm/... -count=1
go test ./app/... -count=1
go vet ./...
```

The bridge must compile and pass existing BVM tests (no regressions). New tests come in R15-2.

## Commit Convention

```
feat(R15-1): add AuthKeeper interface to BVM expected keepers
feat(R15-1): wire CallerDID resolution + SessionCapabilities in BVM execution
feat(R15-1): add capability inheritance for scheduled BVM execution
feat(R15-1): wire auth→BVM adapter in app.go
```
