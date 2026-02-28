# R28-3 — Albedo: Domain Stratum Hierarchy

_Purifying knowledge by giving it depth._

## The Problem

From R25 assessment: "stratum hierarchy returns 1 for all domains (cosmetic)." Every domain has the same depth, the same weight, the same epistemic standing. Physics and "miscellaneous" are treated identically. The qualification cross-reference and inheritance pathways use stratum discounts (20% and 30% respectively), but since stratum is always 1, these discounts are meaningless.

Knowledge without hierarchy is noise. Albedo gives it structure.

## The Fix: Living Stratum Tree

Domains should form a tree with meaningful strata:

```
stratum 0: ROOT (meta-domain, not directly claimable)
├── stratum 1: FORMAL (mathematics, logic, computation)
├── stratum 1: EMPIRICAL (physics, chemistry, biology)
│   ├── stratum 2: PHYSICS
│   │   ├── stratum 3: MECHANICS
│   │   ├── stratum 3: THERMODYNAMICS
│   │   └── stratum 3: QUANTUM
│   ├── stratum 2: CHEMISTRY
│   └── stratum 2: BIOLOGY
├── stratum 1: SOCIAL (economics, governance, ethics)
└── stratum 1: APPLIED (engineering, medicine, technology)
```

**Stratum affects:**
1. **Qualification inheritance discount**: inheriting from parent domain costs `30% × stratum_diff` (deeper = harder to inherit)
2. **Cross-reference discount**: qualifying via related domain costs `20% × stratum_diff`
3. **Claim confidence weighting**: claims in deeper (more specific) domains get a confidence multiplier for specificity
4. **Capture defense sensitivity**: higher-stratum (broader) domains have stricter capture thresholds

## Task

### 1. Fix GetStratum to Return Real Values

Find where stratum is computed and replace the hardcoded `return 1`:

```bash
grep -rn "GetStratum\|stratum" --include="*.go" x/knowledge/ x/qualification/ x/ontology/ | head -20
```

Stratum should be derived from the domain's position in the tree:
- Root domains (no parent): stratum = 1
- Child domains: stratum = parent.stratum + 1
- Max stratum: 5 (prevent infinite nesting)

### 2. Add Parent Field to Domain

If domains don't already have a `parent_domain` field, add one:

```protobuf
message Domain {
    string name = 1;
    string parent_domain = 2;  // empty = root domain
    uint64 stratum = 3;        // computed from tree depth
    // ... existing fields
}
```

When a domain is proposed (`MsgProposeDomain`), allow specifying a parent. Stratum is auto-computed.

### 3. Update Genesis Axiom Domains

The 777 genesis axioms have domains. Verify they map to the tree structure:
- `general` → stratum 1 (root)
- `computational` → stratum 2 under `formal`
- Any others → assign appropriate parents and strata

### 4. Wire Stratum into Qualification

In `x/qualification/keeper/`:
- Cross-reference pathway: actual discount = `CrossRefDiscountBps × stratum_diff`
- Inheritance pathway: actual discount = `InheritanceDiscountBps × stratum_diff`
- If stratum_diff = 0 (same stratum): no discount (full weight)
- If stratum_diff > 3: disallow (too distant to transfer qualification)

### 5. Wire Stratum into Knowledge

- Claims in deeper domains (higher stratum) could receive a specificity bonus to initial confidence
- Or: claims in stratum 1 (broad) require more verifiers than stratum 3 (specific)
- Design decision — document which approach and why

### 6. Wire Stratum into Capture Defense

- Higher-stratum (broader) domains: lower HHI threshold (easier to flag capture)
- Rationale: capturing "physics" is more dangerous than capturing "thermodynamics"
- Update `AnalyzeCaptureRisk` to adjust thresholds by stratum

### 7. Tests

- Domain with parent → stratum = parent.stratum + 1
- Root domain → stratum = 1
- Max stratum enforcement (can't nest beyond 5)
- Qualification inheritance discount scales with stratum diff
- Cross-reference discount scales with stratum diff
- Stratum diff > 3 blocks qualification transfer
- Genesis domains have correct strata
- Capture defense thresholds adjusted by stratum

## Files to Modify

- `x/knowledge/types/` — Add parent_domain to Domain proto if needed
- `x/knowledge/keeper/` — Compute stratum from tree, store/query it
- `x/qualification/keeper/` — Scale discounts by stratum diff
- `x/capture_defense/keeper/detection.go` — Adjust thresholds by stratum
- Genesis axiom data — Update domain assignments

## Success Criteria

- [ ] Domains form a tree with real strata (not all 1)
- [ ] Stratum auto-computed from parent chain
- [ ] Qualification discounts scale meaningfully with stratum distance
- [ ] Genesis domains have correct hierarchy
- [ ] Capture defense sensitivity varies by stratum
- [ ] Knowledge has depth, not just breadth
