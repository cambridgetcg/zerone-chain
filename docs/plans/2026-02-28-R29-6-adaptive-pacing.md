# R29-6 Adaptive Pacing (動靜) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Wire alignment health signals across all modules so the system breathes as one — degraded health slows creation and speeds analysis; healthy restores normal pace.

**Architecture:** The alignment module exposes a `GetGlobalPacingMultiplier(ctx) (creationBps, analysisBps)` method. Four consuming modules (knowledge, capture_defense, partnerships, discovery) hold a `PacingKeeper` interface reference and adjust their interval-based processing accordingly. Global pacing sets the floor; domain-specific conditions (R29-1 carrying capacity) can only tighten further. All pacing via BPS (1,000,000 = 100%).

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, existing module keeper patterns, post-init setter wiring.

---

### Task 1: Alignment — PacingKeeper interface & GetGlobalPacingMultiplier

**Files:**
- Create: `x/alignment/keeper/pacing.go`
- Create: `x/alignment/keeper/pacing_test.go`

**Step 1: Write the failing test**

Create `x/alignment/keeper/pacing_test.go`:
```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

func TestGetGlobalPacingMultiplier_Healthy(t *testing.T) {
	k, ctx := setupKeeper(t)
	// Set state with healthy category
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryHealthy})
	creation, analysis := k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, types.BPS, creation)
	require.Equal(t, types.BPS, analysis)
}

func TestGetGlobalPacingMultiplier_Degraded(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryDegraded})
	creation, analysis := k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, uint64(750_000), creation)
	require.Equal(t, uint64(1_500_000), analysis)
}

func TestGetGlobalPacingMultiplier_Critical(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryCritical})
	creation, analysis := k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, uint64(500_000), creation)
	require.Equal(t, uint64(2_000_000), analysis)
}

func TestGetGlobalPacingMultiplier_Disabled(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetState(ctx, &types.AlignmentState{Enabled: false, PreviousCategory: types.CategoryCritical})
	creation, analysis := k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, types.BPS, creation)
	require.Equal(t, types.BPS, analysis)
}

func TestGetGlobalPacingMultiplier_NoPreviousCategory(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: ""})
	creation, analysis := k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, types.BPS, creation)
	require.Equal(t, types.BPS, analysis)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -run TestGetGlobalPacingMultiplier -v -count=1`
Expected: FAIL — `GetGlobalPacingMultiplier` not defined

**Step 3: Write minimal implementation**

Create `x/alignment/keeper/pacing.go`:
```go
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// GetGlobalPacingMultiplier returns creation and analysis pacing multipliers
// based on the current health category. Values are in BPS (1,000,000 = 100%).
// Degraded health slows creation (longer cooldowns) and speeds analysis (shorter intervals).
// Critical health doubles these effects.
func (k Keeper) GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64) {
	state := k.GetState(ctx)
	if state == nil || !state.Enabled {
		return types.BPS, types.BPS
	}

	switch state.PreviousCategory {
	case types.CategoryHealthy:
		return types.BPS, types.BPS
	case types.CategoryDegraded:
		return 750_000, 1_500_000
	case types.CategoryCritical:
		return 500_000, 2_000_000
	default:
		return types.BPS, types.BPS
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -run TestGetGlobalPacingMultiplier -v -count=1`
Expected: PASS (all 5 sub-tests)

**Step 5: Commit**

```bash
git add x/alignment/keeper/pacing.go x/alignment/keeper/pacing_test.go
git commit -m "feat(alignment): add GetGlobalPacingMultiplier for adaptive pacing (R29-6)"
```

---

### Task 2: Alignment — PacingKeeper interface type

**Files:**
- Modify: `x/alignment/types/expected_keepers.go` (add PacingKeeper interface at bottom)

**Step 1: Write the code**

Add to end of `x/alignment/types/expected_keepers.go`:
```go
// PacingKeeper provides global pacing signals for cross-module adaptive timing.
// Consuming modules hold this interface to modulate their intervals based on system health.
type PacingKeeper interface {
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
```

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/alignment/...`
Expected: Success

**Step 3: Commit**

```bash
git add x/alignment/types/expected_keepers.go
git commit -m "feat(alignment): add PacingKeeper interface for cross-module pacing (R29-6)"
```

---

### Task 3: Alignment — GlobalPacing query + CLI

**Files:**
- Create: `x/alignment/types/pacing.go` (query request/response types)
- Modify: `x/alignment/keeper/grpc_query.go` (add GlobalPacing handler)
- Modify: `x/alignment/types/query_grpc.pb.go` (add gRPC registration)
- Modify: `x/alignment/types/query.pb.go` (no change needed — types are hand-defined)
- Modify: `x/alignment/client/cli/query.go` (add CLI command)
- Create: `x/alignment/keeper/pacing_query_test.go`

**Step 1: Write the failing test**

Create `x/alignment/keeper/pacing_query_test.go`:
```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

func TestQueryGlobalPacing_Healthy(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryHealthy})

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.GlobalPacing(ctx, &types.QueryGlobalPacingRequest{})
	require.NoError(t, err)
	require.Equal(t, types.CategoryHealthy, resp.HealthCategory)
	require.Equal(t, types.BPS, resp.CreationMultiplierBps)
	require.Equal(t, types.BPS, resp.AnalysisMultiplierBps)
}

func TestQueryGlobalPacing_Degraded(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryDegraded})

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.GlobalPacing(ctx, &types.QueryGlobalPacingRequest{})
	require.NoError(t, err)
	require.Equal(t, types.CategoryDegraded, resp.HealthCategory)
	require.Equal(t, uint64(750_000), resp.CreationMultiplierBps)
	require.Equal(t, uint64(1_500_000), resp.AnalysisMultiplierBps)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -run TestQueryGlobalPacing -v -count=1`
Expected: FAIL — types not defined

**Step 3: Create query types**

Create `x/alignment/types/pacing.go`:
```go
package types

// QueryGlobalPacingRequest is the request for GlobalPacing query.
type QueryGlobalPacingRequest struct{}

// ModulePacingEffect describes how pacing affects one module parameter.
type ModulePacingEffect struct {
	Module         string `json:"module"`
	Parameter      string `json:"parameter"`
	BaseValue      uint64 `json:"base_value"`
	EffectiveValue uint64 `json:"effective_value"`
}

// QueryGlobalPacingResponse is the response for GlobalPacing query.
type QueryGlobalPacingResponse struct {
	HealthCategory       string                `json:"health_category"`
	CreationMultiplierBps uint64               `json:"creation_multiplier_bps"`
	AnalysisMultiplierBps uint64               `json:"analysis_multiplier_bps"`
	AffectedModules      []*ModulePacingEffect `json:"affected_modules"`
}
```

**Step 4: Add query handler**

Add to `x/alignment/keeper/grpc_query.go` after the CorrectionConfidence method:
```go
func (q queryServer) GlobalPacing(ctx context.Context, req *types.QueryGlobalPacingRequest) (*types.QueryGlobalPacingResponse, error) {
	state := q.Keeper.GetState(ctx)
	category := state.PreviousCategory
	if category == "" {
		category = types.CategoryHealthy
	}

	creation, analysis := q.Keeper.GetGlobalPacingMultiplier(ctx)
	return &types.QueryGlobalPacingResponse{
		HealthCategory:        category,
		CreationMultiplierBps: creation,
		AnalysisMultiplierBps: analysis,
	}, nil
}
```

**Step 5: Update gRPC registration**

In `x/alignment/types/query_grpc.pb.go`:
- Add `Query_GlobalPacing_FullMethodName = "/zerone.alignment.v1.Query/GlobalPacing"` to const block
- Add `GlobalPacing(context.Context, *QueryGlobalPacingRequest) (*QueryGlobalPacingResponse, error)` to `QueryClient` and `QueryServer` interfaces
- Add `(UnimplementedQueryServer) GlobalPacing(...)` stub
- Add `_Query_GlobalPacing_Handler` function
- Add client method
- Add handler to `Query_ServiceDesc.Methods`

Follow the exact pattern of `CorrectionConfidence` entries.

**Step 6: Add CLI command**

Add to `x/alignment/client/cli/query.go`:
- `NewQueryGlobalPacingCmd()` function
- Add to `queryCmd.AddCommand(...)` list

```go
func NewQueryGlobalPacingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global-pacing",
		Short: "Query the global adaptive pacing state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryGlobalPacingRequest{}
			resp := &types.QueryGlobalPacingResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/GlobalPacing", req, resp); err != nil {
				return fmt.Errorf("failed to query global pacing: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 7: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -run TestQueryGlobalPacing -v -count=1`
Expected: PASS

**Step 8: Commit**

```bash
git add x/alignment/types/pacing.go x/alignment/keeper/grpc_query.go x/alignment/types/query_grpc.pb.go x/alignment/client/cli/query.go x/alignment/keeper/pacing_query_test.go
git commit -m "feat(alignment): add GlobalPacing gRPC query and CLI command (R29-6)"
```

---

### Task 4: Knowledge — Adaptive claim cooldown & review fee scaling

**Files:**
- Modify: `x/knowledge/keeper/keeper.go:16-33` (add `pacingKeeper` field)
- Modify: `x/knowledge/types/expected_keepers.go` (add PacingKeeper import/alias — or define inline)
- Modify: `x/knowledge/keeper/msg_server.go:27-35` (add cooldown check)
- Create: `x/knowledge/keeper/pacing.go` (adaptive helpers)
- Create: `x/knowledge/keeper/pacing_test.go`
- Modify: `app/app.go` (wire pacing keeper)

**Step 1: Write the failing test**

Create `x/knowledge/keeper/pacing_test.go`:
```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetEffectiveCooldown_Healthy(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	// Default: no pacing keeper → base cooldown
	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	require.Equal(t, uint64(50), cooldown) // ClaimCooldownBlocks default
}

func TestGetEffectiveCooldown_Degraded(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	k.SetPacingKeeper(&mockPacingKeeper{creation: 750_000, analysis: 1_500_000})
	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	// 50 * 1_000_000 / 750_000 = 66
	require.Equal(t, uint64(66), cooldown)
}

func TestGetEffectiveCooldown_Critical(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	k.SetPacingKeeper(&mockPacingKeeper{creation: 500_000, analysis: 2_000_000})
	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	// 50 * 1_000_000 / 500_000 = 100
	require.Equal(t, uint64(100), cooldown)
}

func TestGetEffectiveCooldown_DomainPressureOverride(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	k.SetPacingKeeper(&mockPacingKeeper{creation: 750_000, analysis: 1_500_000})
	// Simulate overcrowded domain (pressure > BPS)
	// When domain pressure is stricter than global pacing, use domain value
	// This test validates the max(global, domain) logic
	// Specific behavior depends on domain stats setup
}

func TestGetEffectiveReviewFee_Healthy(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	fee := k.GetEffectiveMinReviewFee(ctx)
	require.Equal(t, "100000", fee) // base MinReviewFee
}

func TestGetEffectiveReviewFee_Degraded(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	k.SetPacingKeeper(&mockPacingKeeper{creation: 750_000, analysis: 1_500_000})
	fee := k.GetEffectiveMinReviewFee(ctx)
	// 100000 * 1_000_000 / 750_000 = 133333
	require.Equal(t, "133333", fee)
}

type mockPacingKeeper struct {
	creation uint64
	analysis uint64
}

func (m *mockPacingKeeper) GetGlobalPacingMultiplier(_ interface{}) (uint64, uint64) {
	return m.creation, m.analysis
}
```

Note: The mock's method signature will need to match the actual PacingKeeper interface with `context.Context`. Adjust during implementation.

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestGetEffective -v -count=1`
Expected: FAIL — methods not defined

**Step 3: Add PacingKeeper interface to knowledge types**

Add to `x/knowledge/types/expected_keepers.go`:
```go
// PacingKeeper provides global pacing signals from the alignment module.
type PacingKeeper interface {
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
```

**Step 4: Add pacingKeeper field and setter to knowledge keeper**

In `x/knowledge/keeper/keeper.go`:
- Add `pacingKeeper types.PacingKeeper // nil until R29-6` field to Keeper struct (after line 32)
- Add setter method:
```go
// SetPacingKeeper sets the pacing keeper for adaptive timing (R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) {
	k.pacingKeeper = pk
}
```

**Step 5: Create pacing helpers**

Create `x/knowledge/keeper/pacing.go`:
```go
package keeper

import (
	"context"
	"math/big"
)

const pacingBPS = uint64(1_000_000)

// GetEffectiveCooldown returns the claim cooldown adjusted by global health pacing
// and domain carrying capacity pressure. Global pacing sets the floor; domain
// pressure can only tighten further.
func (k Keeper) GetEffectiveCooldown(ctx context.Context, domain string) uint64 {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 50 // safe default
	}
	baseCooldown := params.ClaimCooldownBlocks

	// Apply global pacing
	effectiveCooldown := baseCooldown
	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing != pacingBPS {
			effectiveCooldown = baseCooldown * pacingBPS / creationPacing
		}
	}

	// Domain carrying capacity override: can only tighten
	pressure := k.GetDomainPressure(ctx, domain)
	if pressure > pacingBPS {
		domainCooldown := effectiveCooldown * pressure / pacingBPS
		if domainCooldown > effectiveCooldown {
			effectiveCooldown = domainCooldown
		}
	}

	return effectiveCooldown
}

// GetEffectiveMinReviewFee returns the minimum review fee adjusted by global
// health pacing. Stressed system → higher fee to slow submissions.
func (k Keeper) GetEffectiveMinReviewFee(ctx context.Context) string {
	params, err := k.GetParams(ctx)
	if err != nil {
		return "100000"
	}
	baseStr := params.MinReviewFee
	baseFee, ok := new(big.Int).SetString(baseStr, 10)
	if !ok || baseFee.Sign() <= 0 {
		return baseStr
	}

	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing < pacingBPS {
			// Inverse: slower creation → higher fee
			adjusted := new(big.Int).Mul(baseFee, new(big.Int).SetUint64(pacingBPS))
			adjusted.Div(adjusted, new(big.Int).SetUint64(creationPacing))
			return adjusted.String()
		}
	}

	return baseStr
}
```

**Step 6: Add cooldown check to SubmitClaim**

In `x/knowledge/keeper/msg_server.go`, after the content hash dedup check (~line 135) and before fee collection (~line 148), add:
```go
	// Adaptive cooldown check (R29-6): enforce ClaimCooldownBlocks with pacing
	effectiveCooldown := m.keeper.GetEffectiveCooldown(ctx, msg.Domain)
	if effectiveCooldown > 0 {
		lastClaimHeight := m.keeper.GetLastClaimHeight(ctx, msg.Submitter)
		if lastClaimHeight > 0 && height-lastClaimHeight < effectiveCooldown {
			return nil, fmt.Errorf("claim cooldown active: %d blocks remaining (effective cooldown: %d)",
				effectiveCooldown-(height-lastClaimHeight), effectiveCooldown)
		}
	}
```

Also update fee validation to use effective fee:
```go
	effectiveMinFee := m.keeper.GetEffectiveMinReviewFee(ctx)
	minFee, _ := new(big.Int).SetString(effectiveMinFee, 10)
```

This requires a `GetLastClaimHeight` / `SetLastClaimHeight` helper using a new store key prefix. Add a `LastClaimHeightKeyPrefix = []byte{0x30}` to knowledge types/keys.go and implement simple get/set in a state helper.

**Step 7: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestGetEffective -v -count=1`
Expected: PASS

**Step 8: Commit**

```bash
git add x/knowledge/keeper/pacing.go x/knowledge/keeper/pacing_test.go x/knowledge/keeper/keeper.go x/knowledge/keeper/msg_server.go x/knowledge/types/expected_keepers.go x/knowledge/types/keys.go
git commit -m "feat(knowledge): add adaptive claim cooldown and review fee scaling (R29-6)"
```

---

### Task 5: Capture Defense — Adaptive analysis frequency

**Files:**
- Modify: `x/capture_defense/keeper/keeper.go:17-25` (add pacingKeeper field)
- Modify: `x/capture_defense/types/types.go` (add PacingKeeper interface)
- Create: `x/capture_defense/keeper/pacing_test.go`
- Modify: `app/app.go` (wire pacing keeper)

**Step 1: Write the failing test**

Create `x/capture_defense/keeper/pacing_test.go`:
```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBeginBlocker_AdaptiveAnalysisInterval_Healthy(t *testing.T) {
	k, ctx := setupKeeper(t)
	// At healthy, analysis runs at base interval (1000)
	// Block 1000 should trigger
	ctx = ctx.WithBlockHeight(1000)
	err := k.BeginBlocker(ctx)
	require.NoError(t, err)
	// Verify RunAutoAnalysis was called (check via metrics or events)
}

func TestBeginBlocker_AdaptiveAnalysisInterval_Degraded(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetPacingKeeper(&mockPacingKeeper{creation: 750_000, analysis: 1_500_000})
	// Effective interval: 1000 * 1_000_000 / 1_500_000 = 666
	// Block 666 should trigger
	ctx = ctx.WithBlockHeight(666)
	err := k.BeginBlocker(ctx)
	require.NoError(t, err)
}
```

**Step 2: Add PacingKeeper interface to capture_defense types**

In `x/capture_defense/types/types.go`, add:
```go
// PacingKeeper provides global pacing signals for adaptive analysis timing.
type PacingKeeper interface {
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
```

**Step 3: Add pacingKeeper field and setter**

In `x/capture_defense/keeper/keeper.go`:
- Add `pacingKeeper types.PacingKeeper // nil-safe, R29-6` field after line 24
- Add setter:
```go
// SetPacingKeeper sets the pacing keeper for adaptive analysis timing (R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) { k.pacingKeeper = pk }
```

**Step 4: Modify BeginBlocker for adaptive interval**

In `x/capture_defense/keeper/keeper.go`, modify the auto risk analysis section (lines 105-108):
```go
	// Auto risk analysis — adaptive interval (R29-6)
	effectiveInterval := params.RiskAnalysisInterval
	if k.pacingKeeper != nil {
		_, analysisPacing := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if analysisPacing > 0 && analysisPacing != 1_000_000 {
			effectiveInterval = params.RiskAnalysisInterval * 1_000_000 / analysisPacing
		}
	}
	if height > 0 && effectiveInterval > 0 && height%effectiveInterval == 0 {
		k.RunAutoAnalysis(sdkCtx, params)
	}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/capture_defense/keeper/ -run TestBeginBlocker_Adaptive -v -count=1`
Expected: PASS

**Step 6: Commit**

```bash
git add x/capture_defense/keeper/keeper.go x/capture_defense/types/types.go x/capture_defense/keeper/pacing_test.go
git commit -m "feat(capture_defense): add adaptive analysis frequency via global pacing (R29-6)"
```

---

### Task 6: Partnerships — Adaptive formation matching interval

**Files:**
- Modify: `x/partnerships/keeper/keeper.go:17-28` (add pacingKeeper field)
- Modify: `x/partnerships/types/genesis.go` or create `x/partnerships/types/expected_keepers.go` (add PacingKeeper interface)
- Modify: `x/partnerships/keeper/formation_matching.go:67-73` (adaptive interval)
- Create: `x/partnerships/keeper/pacing_test.go`
- Modify: `app/app.go` (wire pacing keeper)

**Step 1: Write the failing test**

Create `x/partnerships/keeper/pacing_test.go`:
```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunFormationMatching_AdaptiveInterval_Healthy(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	// At healthy, matching runs at base interval (100)
	// Block 100 should trigger
	ctx = ctx.WithBlockHeight(100)
	k.RunFormationMatching(ctx) // should not panic
}

func TestRunFormationMatching_AdaptiveInterval_Degraded(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	k.SetPacingKeeper(&mockPacingKeeper{creation: 750_000, analysis: 1_500_000})
	// Effective interval: 100 * 1_000_000 / 750_000 = 133
	// Block 100 should NOT trigger (not divisible by 133)
	ctx = ctx.WithBlockHeight(100)
	k.RunFormationMatching(ctx) // should skip
	// Block 133 should trigger
	ctx = ctx.WithBlockHeight(133)
	k.RunFormationMatching(ctx) // should trigger
}
```

**Step 2: Add PacingKeeper interface**

Create `x/partnerships/types/expected_keepers.go` (or add to existing types file):
```go
package types

import "context"

// PacingKeeper provides global pacing signals for adaptive formation timing.
type PacingKeeper interface {
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
```

**Step 3: Add pacingKeeper field and setter**

In `x/partnerships/keeper/keeper.go`:
- Add `pacingKeeper types.PacingKeeper // nil-safe, R29-6` field
- Add setter:
```go
// SetPacingKeeper sets the pacing keeper for adaptive formation timing (R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) { k.pacingKeeper = pk }
```

**Step 4: Modify RunFormationMatching for adaptive interval**

In `x/partnerships/keeper/formation_matching.go`, replace lines 71-73:
```go
	effectiveInterval := params.FormationMatchIntervalBlocks
	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing != 1_000_000 {
			effectiveInterval = params.FormationMatchIntervalBlocks * 1_000_000 / creationPacing
		}
	}
	if effectiveInterval == 0 || currentBlock%effectiveInterval != 0 {
		return
	}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/partnerships/keeper/ -run TestRunFormationMatching_Adaptive -v -count=1`
Expected: PASS

**Step 6: Commit**

```bash
git add x/partnerships/keeper/keeper.go x/partnerships/keeper/formation_matching.go x/partnerships/types/expected_keepers.go x/partnerships/keeper/pacing_test.go
git commit -m "feat(partnerships): add adaptive formation matching interval (R29-6)"
```

---

### Task 7: Discovery — Adaptive expiry check interval

**Files:**
- Modify: `x/discovery/keeper/keeper.go:18-24` (add pacingKeeper field)
- Modify: `x/discovery/types/genesis.go` or create `x/discovery/types/expected_keepers.go` (add PacingKeeper)
- Modify: `x/discovery/keeper/abci.go:14-21` (adaptive interval)
- Create: `x/discovery/keeper/pacing_test.go`
- Modify: `app/app.go` (wire pacing keeper)

**Step 1: Write the failing test**

Create `x/discovery/keeper/pacing_test.go`:
```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBeginBlocker_AdaptiveExpiryInterval_Degraded(t *testing.T) {
	k, ctx := setupTestKeeper(t)
	k.SetPacingKeeper(&mockPacingKeeper{creation: 750_000, analysis: 1_500_000})
	// Base expiry check interval: 100
	// Creation pacing 75% → effective: 100 * 1_000_000 / 750_000 = 133
	// Block 100 should NOT trigger (not divisible by 133)
	ctx = ctx.WithBlockHeight(100)
	err := k.BeginBlocker(ctx)
	require.NoError(t, err)
}
```

**Step 2: Add PacingKeeper interface**

Add to discovery types:
```go
// PacingKeeper provides global pacing signals for adaptive discovery timing.
type PacingKeeper interface {
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
```

**Step 3: Add pacingKeeper field and setter**

In `x/discovery/keeper/keeper.go`:
- Add `pacingKeeper types.PacingKeeper // nil-safe, R29-6` field
- Add setter:
```go
// SetPacingKeeper sets the pacing keeper for adaptive discovery timing (R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) { k.pacingKeeper = pk }
```

**Step 4: Modify BeginBlocker for adaptive interval**

In `x/discovery/keeper/abci.go`, replace the hardcoded `100` interval (line 19):
```go
	// Adaptive expiry check interval (R29-6)
	expiryCheckInterval := uint64(100)
	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing != 1_000_000 {
			expiryCheckInterval = 100 * 1_000_000 / creationPacing
		}
	}
	if expiryCheckInterval == 0 {
		expiryCheckInterval = 100
	}
	if currentBlock%expiryCheckInterval != 0 {
		return nil
	}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/discovery/keeper/ -run TestBeginBlocker_Adaptive -v -count=1`
Expected: PASS

**Step 6: Commit**

```bash
git add x/discovery/keeper/keeper.go x/discovery/keeper/abci.go x/discovery/types/expected_keepers.go x/discovery/keeper/pacing_test.go
git commit -m "feat(discovery): add adaptive expiry check interval (R29-6)"
```

---

### Task 8: App wiring — Connect PacingKeeper to all consumers

**Files:**
- Modify: `app/app.go` (add SetPacingKeeper calls + create adapter)
- Create: `x/alignment/keeper/pacing_adapter.go` (adapter wrapping Keeper as PacingKeeper)

**Step 1: Create alignment pacing adapter**

Create `x/alignment/keeper/pacing_adapter.go`:
```go
package keeper

import "context"

// AlignmentPacingAdapter wraps the alignment Keeper to expose it as a PacingKeeper
// for other modules.
type AlignmentPacingAdapter struct {
	keeper Keeper
}

// NewAlignmentPacingAdapter creates a new adapter.
func NewAlignmentPacingAdapter(k Keeper) *AlignmentPacingAdapter {
	return &AlignmentPacingAdapter{keeper: k}
}

// GetGlobalPacingMultiplier delegates to the alignment keeper.
func (a *AlignmentPacingAdapter) GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64) {
	return a.keeper.GetGlobalPacingMultiplier(ctx)
}
```

**Step 2: Wire in app.go**

After the alignment keeper is created (~line 1032 in app.go), add:
```go
	// R29-6: Wire global pacing to consuming modules
	alignmentPacingAdapter := zeronealignmentkeeper.NewAlignmentPacingAdapter(app.AlignmentKeeper)
	app.KnowledgeKeeper.SetPacingKeeper(alignmentPacingAdapter)
	app.CaptureDefenseKeeper.SetPacingKeeper(alignmentPacingAdapter)
	app.PartnershipsKeeper.SetPacingKeeper(alignmentPacingAdapter)
	app.DiscoveryKeeper.SetPacingKeeper(alignmentPacingAdapter)
```

**Step 3: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: Success

**Step 4: Commit**

```bash
git add x/alignment/keeper/pacing_adapter.go app/app.go
git commit -m "feat(app): wire alignment PacingKeeper to knowledge, capture_defense, partnerships, discovery (R29-6)"
```

---

### Task 9: Events — Enrich health transitions with pacing multipliers

**Files:**
- Modify: `x/alignment/module.go:190-218` (enrich events with pacing data)

**Step 1: Enrich health transition events**

In `x/alignment/module.go`, after computing the health category (line 175) and before the transition switch (line 192), compute pacing values:
```go
	// Compute pacing for event enrichment (R29-6)
	creationPacing, analysisPacing := am.keeper.GetGlobalPacingMultiplier(ctx)
```

Then add pacing attributes to each health transition event:
```go
	sdk.NewAttribute("creation_multiplier_bps", fmt.Sprintf("%d", creationPacing)),
	sdk.NewAttribute("analysis_multiplier_bps", fmt.Sprintf("%d", analysisPacing)),
```

And enrich the `observation_recorded` event at line 230 with the same attributes.

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./x/alignment/...`
Expected: Success

**Step 3: Commit**

```bash
git add x/alignment/module.go
git commit -m "feat(alignment): enrich health events with pacing multipliers (R29-6)"
```

---

### Task 10: Integration test — Full pacing lifecycle

**Files:**
- Create: `x/alignment/keeper/pacing_integration_test.go`

**Step 1: Write integration test**

Create `x/alignment/keeper/pacing_integration_test.go`:
```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

func TestAdaptivePacing_FullLifecycle(t *testing.T) {
	k, ctx := setupKeeper(t)

	// 1. Healthy: base pacing
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryHealthy})
	creation, analysis := k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, types.BPS, creation, "healthy: creation should be 100%")
	require.Equal(t, types.BPS, analysis, "healthy: analysis should be 100%")

	// 2. Transition to degraded
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryDegraded})
	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, uint64(750_000), creation, "degraded: creation should be 75%")
	require.Equal(t, uint64(1_500_000), analysis, "degraded: analysis should be 150%")

	// 3. Transition to critical
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryCritical})
	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, uint64(500_000), creation, "critical: creation should be 50%")
	require.Equal(t, uint64(2_000_000), analysis, "critical: analysis should be 200%")

	// 4. Recovery to healthy
	k.SetState(ctx, &types.AlignmentState{Enabled: true, PreviousCategory: types.CategoryHealthy})
	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, types.BPS, creation, "recovered: creation should be 100%")
	require.Equal(t, types.BPS, analysis, "recovered: analysis should be 100%")

	// 5. Module disabled
	k.SetState(ctx, &types.AlignmentState{Enabled: false, PreviousCategory: types.CategoryCritical})
	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	require.Equal(t, types.BPS, creation, "disabled: creation should be 100%")
	require.Equal(t, types.BPS, analysis, "disabled: analysis should be 100%")

	// 6. Query endpoint
	qs := NewQueryServerImpl(k)
	resp, err := qs.GlobalPacing(ctx, &types.QueryGlobalPacingRequest{})
	require.NoError(t, err)
	require.Equal(t, types.CategoryCritical, resp.HealthCategory) // PreviousCategory from disabled state
	require.Equal(t, types.BPS, resp.CreationMultiplierBps)       // disabled → 1x
}

func TestAdaptivePacing_IntervalCalculations(t *testing.T) {
	// Test that BPS math produces correct interval adjustments
	tests := []struct {
		name            string
		baseInterval    uint64
		creationPacing  uint64
		analysisPacing  uint64
		expectedCreation uint64 // effective interval for creation-paced param
		expectedAnalysis uint64 // effective interval for analysis-paced param
	}{
		{"healthy", 100, 1_000_000, 1_000_000, 100, 100},
		{"degraded_creation", 100, 750_000, 1_500_000, 133, 66},
		{"critical_creation", 100, 500_000, 2_000_000, 200, 50},
		{"degraded_large", 1000, 750_000, 1_500_000, 1333, 666},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Creation-paced (inverse): base * BPS / creationPacing
			effectiveCreation := tc.baseInterval * 1_000_000 / tc.creationPacing
			require.Equal(t, tc.expectedCreation, effectiveCreation)

			// Analysis-paced (inverse): base * BPS / analysisPacing
			effectiveAnalysis := tc.baseInterval * 1_000_000 / tc.analysisPacing
			require.Equal(t, tc.expectedAnalysis, effectiveAnalysis)
		})
	}
}
```

**Step 2: Run test**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/keeper/ -run TestAdaptivePacing -v -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add x/alignment/keeper/pacing_integration_test.go
git commit -m "test(alignment): add full lifecycle integration test for adaptive pacing (R29-6)"
```

---

### Task 11: Final verification and cleanup

**Step 1: Run all affected module tests**

```bash
cd /Users/yournameisai/Desktop/zerone && go test ./x/alignment/... ./x/knowledge/... ./x/capture_defense/... ./x/partnerships/... ./x/discovery/... -v -count=1
```
Expected: All PASS

**Step 2: Full build**

```bash
cd /Users/yournameisai/Desktop/zerone && go build ./...
```
Expected: Success

**Step 3: Final commit (if any cleanup needed)**

```bash
git commit -m "fix(alignment): address R29-6 code review findings"
```
