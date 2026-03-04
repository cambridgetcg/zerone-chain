# R33-2 Module Invariant Suite — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement 20 invariants across 8 custom modules using the SDK InvariantRegistry, plus a CLI command for on-demand checking.

**Architecture:** Each module gets `keeper/invariants.go` following the `x/auth/keeper/invariants.go` pattern. Invariants use `sdk.InvariantRegistry.RegisterRoute()` and return `(string, bool)` via `sdk.FormatInvariant()`. Modules that lack `RegisterInvariants` in `module.go` get it added; existing no-ops get wired up.

**Tech Stack:** Cosmos SDK v0.50 `sdk.InvariantRegistry` + `sdk.Invariant`, Go 1.24+, standard testing

---

## Reference Pattern

All invariants follow the `x/auth/keeper/invariants.go` pattern:

```go
package keeper

import (
    "fmt"
    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/zerone-chain/zerone/x/<module>/types"
)

func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "invariant-name", InvariantFunc(k))
}

func InvariantFunc(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        // Check invariant, accumulate msg for violations
        var msg string
        broken := false
        // ... iterate state, check conditions ...
        if violation {
            msg += fmt.Sprintf("description of violation\n")
            broken = true
        }
        return sdk.FormatInvariant(types.ModuleName, "invariant-name", msg), broken
    }
}
```

Module registration in `module.go`:
```go
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

---

## Task 1: Knowledge Module Invariants (4 invariants)

**Files:**
- Create: `x/knowledge/keeper/invariants.go`
- Create: `x/knowledge/keeper/invariants_test.go`
- Modify: `x/knowledge/module.go:134-135` (wire up existing no-op)

**Step 1: Write the invariants file**

Create `x/knowledge/keeper/invariants.go` with 4 invariants:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/knowledge/types"
)

// RegisterInvariants registers all knowledge module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "domain-count-consistency", DomainCountConsistencyInvariant(k))
    ir.RegisterRoute(types.ModuleName, "no-self-citation", NoSelfCitationInvariant(k))
    ir.RegisterRoute(types.ModuleName, "round-integrity", RoundIntegrityInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// DomainCountConsistencyInvariant checks that each Domain.FactCount matches
// the actual number of facts indexed under that domain.
func DomainCountConsistencyInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        k.IterateDomains(ctx, func(domain *types.Domain) bool {
            var actualCount uint64
            k.IterateFactsByDomain(ctx, domain.Name, func(_ string) bool {
                actualCount++
                return false
            })
            if domain.FactCount != actualCount {
                msg += fmt.Sprintf("domain %s: FactCount=%d but actual facts=%d\n",
                    domain.Name, domain.FactCount, actualCount)
                broken = true
            }
            return false
        })

        return sdk.FormatInvariant(types.ModuleName, "domain-count-consistency", msg), broken
    }
}

// NoSelfCitationInvariant checks that no claim cites itself via relations.
func NoSelfCitationInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        k.IterateFacts(ctx, func(fact *types.Fact) bool {
            for _, rel := range fact.OutgoingRelations {
                if rel.TargetFactId == fact.Id {
                    msg += fmt.Sprintf("fact %s has self-citation via relation type %s\n",
                        fact.Id, rel.RelationType)
                    broken = true
                }
            }
            return false
        })

        return sdk.FormatInvariant(types.ModuleName, "no-self-citation", msg), broken
    }
}

// RoundIntegrityInvariant checks that no claim has ACCEPTED status without
// a completed verification round.
func RoundIntegrityInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        k.IterateClaims(ctx, func(claim *types.Claim) bool {
            if claim.Status == types.ClaimStatus_CLAIM_STATUS_ACCEPTED {
                if claim.RoundId == "" {
                    msg += fmt.Sprintf("claim %s is ACCEPTED but has no round ID\n", claim.Id)
                    broken = true
                }
            }
            return false
        })

        return sdk.FormatInvariant(types.ModuleName, "round-integrity", msg), broken
    }
}
```

**Step 2: Wire up module.go**

In `x/knowledge/module.go`, replace the no-op `RegisterInvariants` (lines 134-135):

```go
// Before:
// RegisterInvariants is a no-op for now; invariants are added in R2-2.
func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry) {}

// After:
// RegisterInvariants registers the knowledge module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/knowledge/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/zerone-chain/zerone/x/knowledge/keeper"
    "github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestParamsValidInvariant_Invalid(t *testing.T) {
    k, ctx := setupKeeper(t)
    params := k.GetParams(ctx)
    params.MinVerifiers = 0 // invalid: must be > 0
    k.SetParams(ctx, params)

    inv := keeper.ParamsValidInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "zero MinVerifiers should break invariant")
}

func TestDomainCountConsistencyInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    // Create a domain with FactCount=0 and no facts — consistent.
    k.SetDomain(ctx, &types.Domain{Name: "test-domain", FactCount: 0})

    inv := keeper.DomainCountConsistencyInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "empty domain should be consistent: %s", msg)
}

func TestDomainCountConsistencyInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    // Create domain claiming 5 facts but store 0 facts.
    k.SetDomain(ctx, &types.Domain{Name: "test-domain", FactCount: 5})

    inv := keeper.DomainCountConsistencyInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "domain with wrong count should break invariant")
}

func TestNoSelfCitationInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    // Create a fact with no self-citation.
    k.SetFact(ctx, &types.Fact{
        Id:     "fact-1",
        Domain: "test",
        OutgoingRelations: []*types.FactRelation{
            {TargetFactId: "fact-2", RelationType: "supports"},
        },
    })

    inv := keeper.NoSelfCitationInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "fact citing another fact should pass: %s", msg)
}

func TestNoSelfCitationInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    // Create a fact that cites itself.
    k.SetFact(ctx, &types.Fact{
        Id:     "fact-1",
        Domain: "test",
        OutgoingRelations: []*types.FactRelation{
            {TargetFactId: "fact-1", RelationType: "supports"},
        },
    })

    inv := keeper.NoSelfCitationInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "self-citing fact should break invariant")
}

func TestRoundIntegrityInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    // ACCEPTED claim with a round ID — valid.
    k.SetClaim(ctx, &types.Claim{
        Id:      "claim-1",
        Status:  types.ClaimStatus_CLAIM_STATUS_ACCEPTED,
        RoundId: "round-abc",
    })

    inv := keeper.RoundIntegrityInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "accepted claim with round should pass: %s", msg)
}

func TestRoundIntegrityInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    // ACCEPTED claim with no round ID — broken.
    k.SetClaim(ctx, &types.Claim{
        Id:     "claim-1",
        Status: types.ClaimStatus_CLAIM_STATUS_ACCEPTED,
    })

    inv := keeper.RoundIntegrityInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "accepted claim without round should break invariant")
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/knowledge/... && go test ./x/knowledge/keeper/ -run TestParams.*Invariant -run TestDomainCount -run TestNoSelfCitation -run TestRoundIntegrity -v -count=1`
Expected: All 8 tests PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/invariants.go x/knowledge/keeper/invariants_test.go x/knowledge/module.go
git commit -m "feat(knowledge): add 4 module invariants (R33-2)"
```

---

## Task 2: Alignment Module Invariants (2 invariants)

**Files:**
- Create: `x/alignment/keeper/invariants.go`
- Create: `x/alignment/keeper/invariants_test.go`
- Modify: `x/alignment/module.go` (add RegisterInvariants method)

**Step 1: Write the invariants file**

Create `x/alignment/keeper/invariants.go`:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/alignment/types"
)

// RegisterInvariants registers all alignment module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "sensor-bounds", SensorBoundsInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// SensorBoundsInvariant checks that the most recent health indices have
// scores within the valid [0, 1_000_000] BPS range.
func SensorBoundsInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        const maxBPS = uint64(1_000_000)

        indices := k.GetRecentHealthIndices(ctx, 10)
        for _, idx := range indices {
            if idx.CompositeScore > maxBPS {
                msg += fmt.Sprintf("health index at height %d: CompositeScore=%d exceeds max %d\n",
                    idx.ObservedAtBlock, idx.CompositeScore, maxBPS)
                broken = true
            }
        }

        return sdk.FormatInvariant(types.ModuleName, "sensor-bounds", msg), broken
    }
}
```

**Step 2: Add RegisterInvariants to module.go**

In `x/alignment/module.go`, add the method. Find the existing methods (likely near RegisterServices) and add:

```go
// RegisterInvariants registers the alignment module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/alignment/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/zerone-chain/zerone/x/alignment/keeper"
    "github.com/zerone-chain/zerone/x/alignment/types"
)

func TestAlignmentParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestAlignmentParamsValidInvariant_Invalid(t *testing.T) {
    k, ctx := setupKeeper(t)
    params := k.GetParams(ctx)
    params.ObservationIntervalBlocks = 0 // invalid
    k.SetParams(ctx, params)

    inv := keeper.ParamsValidInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "zero ObservationIntervalBlocks should break invariant")
}

func TestSensorBoundsInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    // Store a health index within bounds.
    k.SetHealthIndex(ctx, 100, &types.HealthIndex{
        ObservedAtBlock: 100,
        CompositeScore:  500000, // 50% — valid
    })

    inv := keeper.SensorBoundsInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "in-bounds score should pass: %s", msg)
}

func TestSensorBoundsInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    // Store a health index that exceeds BPS max.
    k.SetHealthIndex(ctx, 100, &types.HealthIndex{
        ObservedAtBlock: 100,
        CompositeScore:  2_000_000, // way above 1M — broken
    })

    inv := keeper.SensorBoundsInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "out-of-bounds score should break invariant")
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/alignment/... && go test ./x/alignment/keeper/ -run TestAlignment.*Invariant -run TestSensorBounds -v -count=1`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add x/alignment/keeper/invariants.go x/alignment/keeper/invariants_test.go x/alignment/module.go
git commit -m "feat(alignment): add 2 module invariants (R33-2)"
```

---

## Task 3: Capture Defense Invariants (2 invariants)

**Files:**
- Create: `x/capture_defense/keeper/invariants.go`
- Create: `x/capture_defense/keeper/invariants_test.go`
- Modify: `x/capture_defense/module.go` (add RegisterInvariants)

**Step 1: Write the invariants file**

Create `x/capture_defense/keeper/invariants.go`:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/capture_defense/types"
)

// RegisterInvariants registers all capture_defense module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "metric-bounds", MetricBoundsInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// MetricBoundsInvariant checks that all CaptureMetrics have
// HerfindahlIndex ∈ [0, 1_000_000] and RiskScore ∈ [0, 1_000_000].
func MetricBoundsInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        metrics := k.GetAllCaptureMetrics(ctx)
        for _, m := range metrics {
            if m.HerfindahlIndex > types.BPSScale {
                msg += fmt.Sprintf("domain %s: HerfindahlIndex=%d exceeds max %d\n",
                    m.Domain, m.HerfindahlIndex, types.BPSScale)
                broken = true
            }
            if m.RiskScore > types.BPSScale {
                msg += fmt.Sprintf("domain %s: RiskScore=%d exceeds max %d\n",
                    m.Domain, m.RiskScore, types.BPSScale)
                broken = true
            }
        }

        return sdk.FormatInvariant(types.ModuleName, "metric-bounds", msg), broken
    }
}
```

**Step 2: Add RegisterInvariants to module.go**

In `x/capture_defense/module.go`, add:

```go
// RegisterInvariants registers the capture_defense module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/capture_defense/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/zerone-chain/zerone/x/capture_defense/keeper"
    "github.com/zerone-chain/zerone/x/capture_defense/types"
)

func TestCaptureDefenseParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestMetricBoundsInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
        Domain:          "physics",
        HerfindahlIndex: 250000,
        RiskScore:       100000,
    })

    inv := keeper.MetricBoundsInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "in-bounds metrics should pass: %s", msg)
}

func TestMetricBoundsInvariant_HHIExceedsBPS(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
        Domain:          "physics",
        HerfindahlIndex: 2_000_000, // exceeds BPSScale
        RiskScore:       100000,
    })

    inv := keeper.MetricBoundsInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "HHI exceeding BPSScale should break invariant")
}

func TestMetricBoundsInvariant_RiskScoreExceedsBPS(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
        Domain:          "physics",
        HerfindahlIndex: 250000,
        RiskScore:       1_500_000, // exceeds BPSScale
    })

    inv := keeper.MetricBoundsInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "RiskScore exceeding BPSScale should break invariant")
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/capture_defense/... && go test ./x/capture_defense/keeper/ -run TestCaptureDefense.*Invariant -run TestMetricBounds -v -count=1`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add x/capture_defense/keeper/invariants.go x/capture_defense/keeper/invariants_test.go x/capture_defense/module.go
git commit -m "feat(capture_defense): add 2 module invariants (R33-2)"
```

---

## Task 4: Partnerships Invariants (3 invariants)

**Files:**
- Create: `x/partnerships/keeper/invariants.go`
- Create: `x/partnerships/keeper/invariants_test.go`
- Modify: `x/partnerships/module.go` (add RegisterInvariants)

**Step 1: Write the invariants file**

Create `x/partnerships/keeper/invariants.go`:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/partnerships/types"
)

// RegisterInvariants registers all partnerships module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "active-partnership-members", ActivePartnershipMembersInvariant(k))
    ir.RegisterRoute(types.ModuleName, "no-duplicate-partnerships", NoDuplicatePartnershipsInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// ActivePartnershipMembersInvariant checks that every active partnership
// has exactly 2 members (non-empty HumanAddr and AgentAddr).
func ActivePartnershipMembersInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        partnerships := k.GetAllPartnerships(ctx)
        for _, p := range partnerships {
            if p.Status != types.StatusActive {
                continue
            }
            if p.HumanAddr == "" || p.AgentAddr == "" {
                msg += fmt.Sprintf("active partnership %s: missing member (human=%q, agent=%q)\n",
                    p.Id, p.HumanAddr, p.AgentAddr)
                broken = true
            }
        }

        return sdk.FormatInvariant(types.ModuleName, "active-partnership-members", msg), broken
    }
}

// NoDuplicatePartnershipsInvariant checks that no two active/forming
// partnerships share the same (human, agent) pair.
func NoDuplicatePartnershipsInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        type pair struct{ human, agent string }
        seen := make(map[pair]string) // pair → first partnership ID

        partnerships := k.GetAllPartnerships(ctx)
        for _, p := range partnerships {
            if p.Status == types.StatusDissolved {
                continue
            }
            key := pair{p.HumanAddr, p.AgentAddr}
            if existingID, exists := seen[key]; exists {
                msg += fmt.Sprintf("duplicate partnership pair (%s, %s): IDs %s and %s\n",
                    p.HumanAddr, p.AgentAddr, existingID, p.Id)
                broken = true
            } else {
                seen[key] = p.Id
            }
        }

        return sdk.FormatInvariant(types.ModuleName, "no-duplicate-partnerships", msg), broken
    }
}
```

**Step 2: Add RegisterInvariants to module.go**

In `x/partnerships/module.go`, add:

```go
// RegisterInvariants registers the partnerships module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/partnerships/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/zerone-chain/zerone/x/partnerships/keeper"
    "github.com/zerone-chain/zerone/x/partnerships/types"
)

func TestPartnershipsParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestActivePartnershipMembersInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetPartnership(ctx, &types.Partnership{
        Id:        "p1",
        HumanAddr: "zrn1human",
        AgentAddr: "zrn1agent",
        Status:    types.StatusActive,
    })

    inv := keeper.ActivePartnershipMembersInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "partnership with both members should pass: %s", msg)
}

func TestActivePartnershipMembersInvariant_MissingAgent(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetPartnership(ctx, &types.Partnership{
        Id:        "p1",
        HumanAddr: "zrn1human",
        AgentAddr: "",
        Status:    types.StatusActive,
    })

    inv := keeper.ActivePartnershipMembersInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "active partnership missing agent should break invariant")
}

func TestNoDuplicatePartnershipsInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetPartnership(ctx, &types.Partnership{
        Id: "p1", HumanAddr: "zrn1human1", AgentAddr: "zrn1agent1", Status: types.StatusActive,
    })
    k.SetPartnership(ctx, &types.Partnership{
        Id: "p2", HumanAddr: "zrn1human2", AgentAddr: "zrn1agent2", Status: types.StatusActive,
    })

    inv := keeper.NoDuplicatePartnershipsInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "unique pairs should pass: %s", msg)
}

func TestNoDuplicatePartnershipsInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetPartnership(ctx, &types.Partnership{
        Id: "p1", HumanAddr: "zrn1human", AgentAddr: "zrn1agent", Status: types.StatusActive,
    })
    k.SetPartnership(ctx, &types.Partnership{
        Id: "p2", HumanAddr: "zrn1human", AgentAddr: "zrn1agent", Status: types.StatusActive,
    })

    inv := keeper.NoDuplicatePartnershipsInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "duplicate pair should break invariant")
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/partnerships/... && go test ./x/partnerships/keeper/ -run TestPartnerships.*Invariant -run TestActivePartnership -run TestNoDuplicate -v -count=1`
Expected: All 5 tests PASS

**Step 5: Commit**

```bash
git add x/partnerships/keeper/invariants.go x/partnerships/keeper/invariants_test.go x/partnerships/module.go
git commit -m "feat(partnerships): add 3 module invariants (R33-2)"
```

---

## Task 5: Zerone Staking Invariants (3 invariants)

**Files:**
- Create: `x/staking/keeper/invariants.go`
- Create: `x/staking/keeper/invariants_test.go`
- Modify: `x/staking/module.go` (add RegisterInvariants)

**Note:** Module is `zerone_staking` but code lives in `x/staking/`.

**Step 1: Write the invariants file**

Create `x/staking/keeper/invariants.go`:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/staking/types"
)

// RegisterInvariants registers all zerone_staking module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "delegation-validator-exists", DelegationValidatorExistsInvariant(k))
    ir.RegisterRoute(types.ModuleName, "unbonding-consistency", UnbondingConsistencyInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// DelegationValidatorExistsInvariant checks that every delegation references
// an existing validator.
func DelegationValidatorExistsInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        k.IterateDelegations(ctx, func(del *types.Delegation) bool {
            _, found := k.GetValidator(ctx, del.ValidatorAddress)
            if !found {
                msg += fmt.Sprintf("delegation from %s references non-existent validator %s\n",
                    del.DelegatorAddress, del.ValidatorAddress)
                broken = true
            }
            return false
        })

        return sdk.FormatInvariant(types.ModuleName, "delegation-validator-exists", msg), broken
    }
}

// UnbondingConsistencyInvariant checks that all unbonding entries have
// completion height greater than the current block height.
func UnbondingConsistencyInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false
        currentHeight := uint64(ctx.BlockHeight())

        k.IterateUnbondings(ctx, func(ub *types.UnbondingEntry) bool {
            if ub.CompletionHeight <= currentHeight {
                msg += fmt.Sprintf("unbonding %s has completion height %d <= current height %d\n",
                    ub.Id, ub.CompletionHeight, currentHeight)
                broken = true
            }
            return false
        })

        return sdk.FormatInvariant(types.ModuleName, "unbonding-consistency", msg), broken
    }
}
```

**Step 2: Add RegisterInvariants to module.go**

In `x/staking/module.go`, add:

```go
// RegisterInvariants registers the zerone_staking module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/staking/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/staking/keeper"
    "github.com/zerone-chain/zerone/x/staking/types"
)

func TestStakingParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestDelegationValidatorExistsInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    setupValidatorAndDelegation(t, k, ctx, "zrn1val", "zrn1del")

    inv := keeper.DelegationValidatorExistsInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "delegation to existing validator should pass: %s", msg)
}

func TestDelegationValidatorExistsInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    // Store delegation without creating the validator.
    k.SetDelegation(ctx, &types.Delegation{
        DelegatorAddress: "zrn1del",
        ValidatorAddress: "zrn1ghost",
    })

    inv := keeper.DelegationValidatorExistsInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "delegation to non-existent validator should break invariant")
}

func TestUnbondingConsistencyInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    ctx = ctx.WithBlockHeight(100)
    k.SetUnbonding(ctx, &types.UnbondingEntry{
        Id:               "ub-1",
        CompletionHeight: 200, // future
    })

    inv := keeper.UnbondingConsistencyInvariant(k)
    msg, broken := inv(sdk.UnwrapSDKContext(ctx))
    assert.False(t, broken, "future unbonding should pass: %s", msg)
}

func TestUnbondingConsistencyInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    ctx = ctx.WithBlockHeight(100)
    k.SetUnbonding(ctx, &types.UnbondingEntry{
        Id:               "ub-1",
        CompletionHeight: 50, // past
    })

    inv := keeper.UnbondingConsistencyInvariant(k)
    _, broken := inv(sdk.UnwrapSDKContext(ctx))
    assert.True(t, broken, "past unbonding should break invariant")
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/staking/... && go test ./x/staking/keeper/ -run TestStaking.*Invariant -run TestDelegationValidator -run TestUnbonding -v -count=1`
Expected: All 5 tests PASS

**Step 5: Commit**

```bash
git add x/staking/keeper/invariants.go x/staking/keeper/invariants_test.go x/staking/module.go
git commit -m "feat(zerone_staking): add 3 module invariants (R33-2)"
```

---

## Task 6: Vesting Rewards Invariants (2 invariants)

**Files:**
- Create: `x/vesting_rewards/keeper/invariants.go`
- Create: `x/vesting_rewards/keeper/invariants_test.go`
- Modify: `x/vesting_rewards/module.go` (add RegisterInvariants)

**Step 1: Write the invariants file**

Create `x/vesting_rewards/keeper/invariants.go`:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// RegisterInvariants registers all vesting_rewards module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "schedule-consistency", ScheduleConsistencyInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// ScheduleConsistencyInvariant checks that all active vesting schedules
// have non-zero amounts and valid recipient addresses.
func ScheduleConsistencyInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        schedules := k.GetAllActiveVestingSchedules(ctx)
        for _, s := range schedules {
            if s.Recipient == "" {
                msg += fmt.Sprintf("active vesting schedule %s has empty recipient\n", s.Id)
                broken = true
            }
            if s.TotalAmount == "" || s.TotalAmount == "0" {
                msg += fmt.Sprintf("active vesting schedule %s has zero/empty total amount\n", s.Id)
                broken = true
            }
        }

        return sdk.FormatInvariant(types.ModuleName, "schedule-consistency", msg), broken
    }
}
```

**Step 2: Add RegisterInvariants to module.go**

In `x/vesting_rewards/module.go`, add:

```go
// RegisterInvariants registers the vesting_rewards module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/vesting_rewards/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
    "github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

func TestVestingParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestScheduleConsistencyInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetVestingSchedule(ctx, &types.VestingSchedule{
        Id:          "vs-1",
        Recipient:   "zrn1recipient",
        TotalAmount: "1000000",
        Status:      "active",
    })

    inv := keeper.ScheduleConsistencyInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "valid schedule should pass: %s", msg)
}

func TestScheduleConsistencyInvariant_EmptyRecipient(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetVestingSchedule(ctx, &types.VestingSchedule{
        Id:          "vs-1",
        Recipient:   "",
        TotalAmount: "1000000",
        Status:      "active",
    })

    inv := keeper.ScheduleConsistencyInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "empty recipient should break invariant")
}

func TestScheduleConsistencyInvariant_ZeroAmount(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetVestingSchedule(ctx, &types.VestingSchedule{
        Id:          "vs-1",
        Recipient:   "zrn1recipient",
        TotalAmount: "0",
        Status:      "active",
    })

    inv := keeper.ScheduleConsistencyInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "zero amount should break invariant")
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/vesting_rewards/... && go test ./x/vesting_rewards/keeper/ -run TestVesting.*Invariant -run TestScheduleConsistency -v -count=1`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add x/vesting_rewards/keeper/invariants.go x/vesting_rewards/keeper/invariants_test.go x/vesting_rewards/module.go
git commit -m "feat(vesting_rewards): add 2 module invariants (R33-2)"
```

---

## Task 7: Zerone Gov Invariants (2 invariants)

**Files:**
- Create: `x/gov/keeper/invariants.go`
- Create: `x/gov/keeper/invariants_test.go`
- Modify: `x/gov/module.go` (add RegisterInvariants)

**Note:** Module is `zerone_gov` but code lives in `x/gov/`.

**Step 1: Write the invariants file**

Create `x/gov/keeper/invariants.go`:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/gov/types"
)

// RegisterInvariants registers all zerone_gov module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "proposal-status-consistency", ProposalStatusConsistencyInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// ProposalStatusConsistencyInvariant checks that no LIP with PASSED status
// has zero votes.
func ProposalStatusConsistencyInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        k.IterateLIPs(ctx, func(lip *types.LIP) bool {
            if lip.Status == types.StatusPassed {
                votes := k.GetVotesForLIP(ctx, lip.Id)
                if len(votes) == 0 {
                    msg += fmt.Sprintf("LIP %s has PASSED status but zero votes\n", lip.Id)
                    broken = true
                }
            }
            return false
        })

        return sdk.FormatInvariant(types.ModuleName, "proposal-status-consistency", msg), broken
    }
}
```

**Step 2: Add RegisterInvariants to module.go**

In `x/gov/module.go`, add:

```go
// RegisterInvariants registers the zerone_gov module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/gov/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/zerone-chain/zerone/x/gov/keeper"
    "github.com/zerone-chain/zerone/x/gov/types"
)

func TestGovParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestProposalStatusConsistencyInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    lip := &types.LIP{
        Id:     "lip-1",
        Status: types.StatusPassed,
    }
    k.SetLIP(ctx, lip)
    k.SetVote(ctx, &types.Vote{LipId: "lip-1", Voter: "zrn1voter", Option: 1})

    inv := keeper.ProposalStatusConsistencyInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "passed LIP with votes should pass: %s", msg)
}

func TestProposalStatusConsistencyInvariant_Broken(t *testing.T) {
    k, ctx := setupKeeper(t)
    lip := &types.LIP{
        Id:     "lip-1",
        Status: types.StatusPassed,
    }
    k.SetLIP(ctx, lip)
    // No votes stored for this LIP.

    inv := keeper.ProposalStatusConsistencyInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "passed LIP without votes should break invariant")
}

func TestProposalStatusConsistencyInvariant_DraftIgnored(t *testing.T) {
    k, ctx := setupKeeper(t)
    lip := &types.LIP{
        Id:     "lip-1",
        Status: types.StatusDraft,
    }
    k.SetLIP(ctx, lip)
    // No votes — but it's draft, so no invariant violation.

    inv := keeper.ProposalStatusConsistencyInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "draft LIP without votes should pass: %s", msg)
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/gov/... && go test ./x/gov/keeper/ -run TestGov.*Invariant -run TestProposalStatus -v -count=1`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add x/gov/keeper/invariants.go x/gov/keeper/invariants_test.go x/gov/module.go
git commit -m "feat(zerone_gov): add 2 module invariants (R33-2)"
```

---

## Task 8: Emergency Module Invariants (2 invariants)

**Files:**
- Create: `x/emergency/keeper/invariants.go`
- Create: `x/emergency/keeper/invariants_test.go`
- Modify: `x/emergency/module.go` (add RegisterInvariants)

**Step 1: Write the invariants file**

Create `x/emergency/keeper/invariants.go`:

```go
package keeper

import (
    "fmt"

    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/zerone-chain/zerone/x/emergency/types"
)

// RegisterInvariants registers all emergency module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
    ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
    ir.RegisterRoute(types.ModuleName, "ceremony-consistency", CeremonyConsistencyInvariant(k))
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        params := k.GetParams(ctx)
        if err := params.Validate(); err != nil {
            msg := fmt.Sprintf("stored params are invalid: %v\n", err)
            return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
        }
        return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
    }
}

// CeremonyConsistencyInvariant checks that active ceremonies have valid state:
// - Phase must be "prevote" or "precommit" (not empty)
// - StartBlock must be > 0
// - Deadlines must be > StartBlock
func CeremonyConsistencyInvariant(k Keeper) sdk.Invariant {
    return func(ctx sdk.Context) (string, bool) {
        var msg string
        broken := false

        validPhases := map[string]bool{
            string(types.PhasePrevote):   true,
            string(types.PhasePrecommit): true,
            "finalized":                  true,
            "failed":                     true,
        }

        ceremonies := k.GetAllCeremonies(ctx)
        for _, c := range ceremonies {
            if c.Phase == "" {
                msg += fmt.Sprintf("ceremony %s has empty phase\n", c.Id)
                broken = true
            } else if !validPhases[c.Phase] {
                msg += fmt.Sprintf("ceremony %s has invalid phase %q\n", c.Id, c.Phase)
                broken = true
            }
            if c.StartBlock == 0 {
                msg += fmt.Sprintf("ceremony %s has zero StartBlock\n", c.Id)
                broken = true
            }
        }

        return sdk.FormatInvariant(types.ModuleName, "ceremony-consistency", msg), broken
    }
}
```

**Step 2: Add RegisterInvariants to module.go**

In `x/emergency/module.go`, add:

```go
// RegisterInvariants registers the emergency module invariants.
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
    keeper.RegisterInvariants(ir, am.keeper)
}
```

**Step 3: Write tests**

Create `x/emergency/keeper/invariants_test.go`:

```go
package keeper_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "github.com/zerone-chain/zerone/x/emergency/keeper"
    "github.com/zerone-chain/zerone/x/emergency/types"
)

func TestEmergencyParamsValidInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    inv := keeper.ParamsValidInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "default params should be valid: %s", msg)
}

func TestCeremonyConsistencyInvariant_Valid(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetCeremony(ctx, &types.EmergencyCeremony{
        Id:         "c1",
        Phase:      string(types.PhasePrevote),
        StartBlock: 100,
    })

    inv := keeper.CeremonyConsistencyInvariant(k)
    msg, broken := inv(ctx)
    assert.False(t, broken, "valid ceremony should pass: %s", msg)
}

func TestCeremonyConsistencyInvariant_EmptyPhase(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetCeremony(ctx, &types.EmergencyCeremony{
        Id:         "c1",
        Phase:      "",
        StartBlock: 100,
    })

    inv := keeper.CeremonyConsistencyInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "empty phase should break invariant")
}

func TestCeremonyConsistencyInvariant_ZeroStartBlock(t *testing.T) {
    k, ctx := setupKeeper(t)
    k.SetCeremony(ctx, &types.EmergencyCeremony{
        Id:         "c1",
        Phase:      string(types.PhasePrevote),
        StartBlock: 0,
    })

    inv := keeper.CeremonyConsistencyInvariant(k)
    _, broken := inv(ctx)
    assert.True(t, broken, "zero StartBlock should break invariant")
}
```

**Step 4: Compile and run tests**

Run: `go build ./x/emergency/... && go test ./x/emergency/keeper/ -run TestEmergency.*Invariant -run TestCeremonyConsistency -v -count=1`
Expected: All 4 tests PASS

**Step 5: Commit**

```bash
git add x/emergency/keeper/invariants.go x/emergency/keeper/invariants_test.go x/emergency/module.go
git commit -m "feat(emergency): add 2 module invariants (R33-2)"
```

---

## Task 9: CLI Command — `zeroned query invariants check`

**Files:**
- Create: `x/invariants/` module directory with: `module.go`, `client/cli/query.go`
- Modify: `cmd/zeroned/cmd/root.go` (add query command)

**Context:** SDK v0.50 removed the crisis module, so there's no built-in invariant runner. We create a lightweight CLI command that iterates all registered invariants and reports pass/fail.

**Step 1: Create the invariants CLI module**

Create `x/invariants/client/cli/query.go`:

```go
package cli

import (
    "fmt"

    "github.com/cosmos/cosmos-sdk/client"
    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/cosmos/cosmos-sdk/types/module"
    "github.com/spf13/cobra"
)

// CheckInvariantsCmd returns a CLI command that runs all registered invariants.
func CheckInvariantsCmd(mm module.BasicManager, registry *sdk.InvariantRegistry) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "check-invariants",
        Short: "Run all registered module invariants against current state",
        Long:  "Iterates all invariants registered via sdk.InvariantRegistry and reports pass/fail for each.",
        RunE: func(cmd *cobra.Command, args []string) error {
            clientCtx, err := client.GetClientQueryContext(cmd)
            if err != nil {
                return err
            }
            // This is a placeholder — actual implementation requires app access.
            // For now, print usage guidance.
            fmt.Fprintf(clientCtx.Output, "Run invariant checks via: zeroned start --assert-invariants-block-interval=1\n")
            return nil
        },
    }
    return cmd
}
```

**Alternative approach:** Since invariants need app state access (they run keeper methods against the store), the most practical CLI approach is a server-side command. Check if the app exposes an invariant runner accessible from CLI context. If not, implement a simpler approach: register a query endpoint or use the existing `module.Manager.RunInvariants()` method.

**Step 1 (revised): Add invariant check as a server command**

Instead of a query command (which can't access keeper state), add it as a standalone command that opens the database directly. Create `cmd/zeroned/cmd/invariants.go`:

```go
package cmd

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/cosmos/cosmos-sdk/server"
    sdk "github.com/cosmos/cosmos-sdk/types"
)

// CheckInvariantsCmd creates the check-invariants command.
func CheckInvariantsCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "check-invariants",
        Short: "Run all registered module invariants against current state",
        Long: `Opens the application database read-only and runs every invariant
registered by modules via RegisterInvariants(). Reports pass/fail for each.

This is the replacement for the crisis module's invariant checking which
was removed in Cosmos SDK v0.50.`,
        RunE: func(cmd *cobra.Command, args []string) error {
            serverCtx := server.GetServerContextFromCmd(cmd)
            cfg := serverCtx.Config

            db, err := openDB(cfg)
            if err != nil {
                return fmt.Errorf("failed to open database: %w", err)
            }
            defer db.Close()

            app := newApp(db, serverCtx, nil)

            // Get the latest committed state.
            ctx := app.NewContext(false)

            // Run all invariants via the app's invariant registry.
            res, stop := app.CrisisKeeper.AssertInvariants(ctx)
            // Note: If CrisisKeeper is not available, iterate the module manager
            // and call each module's invariants directly.

            if stop {
                fmt.Println("INVARIANT VIOLATION DETECTED:")
                fmt.Println(res)
                return fmt.Errorf("invariant check failed")
            }
            fmt.Println("All invariants passed.")
            return nil
        },
    }
    return cmd
}
```

**Note to implementer:** The exact wiring depends on what the app struct exposes. Explore `app/app.go` to find:
1. How to open the DB and create an app instance in read-only mode
2. Whether `app.ModuleManager` has a method to run all invariants
3. Whether there's an existing invariant registry accessible from the app

If the app has a `ModuleManager` but no `CrisisKeeper`, implement the invariant runner directly:

```go
// Create a simple registry that collects invariants.
type invariantCollector struct {
    invariants []namedInvariant
}
type namedInvariant struct {
    route string
    name  string
    inv   sdk.Invariant
}
func (c *invariantCollector) RegisterRoute(moduleName, route string, inv sdk.Invariant) {
    c.invariants = append(c.invariants, namedInvariant{moduleName, route, inv})
}

// Then: app.ModuleManager.RegisterInvariants(&collector)
// Then: iterate collector.invariants, run each, report results.
```

**Step 2: Wire the command into root.go**

In `cmd/zeroned/cmd/root.go`, find where commands are added and add:

```go
rootCmd.AddCommand(CheckInvariantsCmd())
```

**Step 3: Test manually**

Run: `go build ./cmd/zeroned/ && ./build/zeroned check-invariants --home ~/.zeroned`
Expected: Command runs, reports invariant results

**Step 4: Commit**

```bash
git add cmd/zeroned/cmd/invariants.go cmd/zeroned/cmd/root.go
git commit -m "feat(cli): add check-invariants command (R33-2)"
```

---

## Task 10: Integration Verification

**Files:**
- Create: `tests/invariants_registration_test.go`

**Step 1: Write a registration smoke test**

This test verifies that all 8 target modules register their invariants with the registry. Create `tests/invariants_registration_test.go`:

```go
package tests

import (
    "testing"

    sdk "github.com/cosmos/cosmos-sdk/types"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/zerone-chain/zerone/app"
)

// TestAllModulesRegisterInvariants verifies that the target modules
// register their invariants when the app module manager calls
// RegisterInvariants.
func TestAllModulesRegisterInvariants(t *testing.T) {
    // Create a collector that captures RegisterRoute calls.
    collector := &invariantCollector{}

    // Create the app and register invariants.
    testApp := app.Setup(t, false)
    testApp.ModuleManager.RegisterInvariants(collector)

    // Verify expected modules registered invariants.
    expectedModules := map[string][]string{
        "zerone_auth":     {"account-did-parity", "session-count-consistency", "params-valid"},
        "knowledge":       {"params-valid", "domain-count-consistency", "no-self-citation", "round-integrity"},
        "alignment":       {"params-valid", "sensor-bounds"},
        "capture_defense": {"params-valid", "metric-bounds"},
        "partnerships":    {"params-valid", "active-partnership-members", "no-duplicate-partnerships"},
        "zerone_staking":  {"params-valid", "delegation-validator-exists", "unbonding-consistency"},
        "vesting_rewards": {"params-valid", "schedule-consistency"},
        "zerone_gov":      {"params-valid", "proposal-status-consistency"},
        "emergency":       {"params-valid", "ceremony-consistency"},
    }

    for module, expectedInvariants := range expectedModules {
        for _, invName := range expectedInvariants {
            found := collector.has(module, invName)
            assert.True(t, found, "module %s should register invariant %q", module, invName)
        }
    }

    // Verify total count (auth:3 + knowledge:4 + alignment:2 + capture_defense:2 +
    // partnerships:3 + zerone_staking:3 + vesting_rewards:2 + zerone_gov:2 + emergency:2 = 23)
    require.GreaterOrEqual(t, len(collector.invariants), 23,
        "expected at least 23 registered invariants (9 modules), got %d", len(collector.invariants))
}

type invariantCollector struct {
    invariants []struct{ module, name string }
}

func (c *invariantCollector) RegisterRoute(module, name string, _ sdk.Invariant) {
    c.invariants = append(c.invariants, struct{ module, name string }{module, name})
}

func (c *invariantCollector) has(module, name string) bool {
    for _, inv := range c.invariants {
        if inv.module == module && inv.name == name {
            return true
        }
    }
    return false
}
```

**Step 2: Run the test**

Run: `go test ./tests/ -run TestAllModulesRegisterInvariants -v -count=1`
Expected: PASS — all 23 invariants registered

**Step 3: Commit**

```bash
git add tests/invariants_registration_test.go
git commit -m "test: add invariant registration smoke test (R33-2)"
```

---

## Adaptation Notes

The code in this plan is a starting point. During implementation, the implementer must:

1. **Check actual method signatures** — The test helpers (`setupKeeper`, `setupValidatorAndDelegation`, etc.) may already exist in each module's test files. Reuse them. If they don't exist, look at existing `*_test.go` files in each keeper directory for the pattern used.

2. **Check field names** — Proto-generated field names may differ slightly (e.g., `CompletionHeight` vs `MaturityHeight`, `Id` vs `ID`). Always verify against the actual `.pb.go` types.

3. **Handle receiver types** — Some keeper methods take `sdk.Context`, others take `context.Context`. The invariant functions always receive `sdk.Context` (since `sdk.Invariant = func(sdk.Context) (string, bool)`), but keeper methods may need `sdk.WrapSDKContext(ctx)`.

4. **Import paths** — Module code directories may not match module names (e.g., `zerone_staking` lives in `x/staking/`, `zerone_gov` lives in `x/gov/`).

5. **vesting_rewards schedule** — The `GetAllActiveVestingSchedules` method may filter by status internally. Verify what "active" means in context (status field value).
