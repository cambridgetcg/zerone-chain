# Albedo: Domain Depth Hierarchy — Design

## Problem

Every domain has the same epistemic standing. Qualification cross-reference (20%) and inheritance (30%) discounts are flat — stratum distance is meaningless because all domains sit at the same level.

## Design

Two independent dimensions per domain:

1. **Epistemic stratum** (existing, unchanged) — axiomatic/formal/empirical/etc. Controls confidence ceilings and decay rates.
2. **Depth** (new) — tree position via `parent_domain` field. Controls qualification discounts and capture defense sensitivity.

### Data Model

Add to ontology and knowledge Domain protos:
- `string parent_domain` — empty = root domain
- `uint32 depth` — auto-computed: root=1, child=parent.depth+1, max=5

### Depth Computation

- `parent_domain == ""` → depth = 1
- Parent exists → depth = parent.depth + 1
- depth > 5 → reject

### Qualification Discounts

**Inheritance**: `discount = InheritanceDiscountBps * depth_diff`. Blocked if depth_diff > 3.
**Cross-reference**: `discount = CrossRefDiscountBps * depth_diff`. No max distance.
depth_diff == 0 → no discount.

### Capture Defense

`adjustedThreshold = HhiThreshold + (depth - 1) * 50000`
- Depth 1 (broad): 250,000 (25%) — strictest
- Depth 5 (specific): 450,000 (45%) — most lenient

### Genesis Domains

All 18 existing domains: parent_domain = "", depth = 1. Tree grows via governance.

### Unchanged

- Epistemic strata, GetStratum, CrossStratumLink, confidence/decay system.
