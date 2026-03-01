# R30-3 — Event Observability

## Problem

R28 and R29 added ~40 new event types across modules. The existing event audit test (`TestEventAudit_AllHandlersEmitEvents`) checks that handlers emit *something*, but doesn't verify:

1. **Event naming consistency** — some use `zerone.module.action`, others use `module_action`, others use `action` alone
2. **Attribute completeness** — no standard for what attributes each event must include
3. **Event documentation** — no single place listing all events, their attributes, and when they fire
4. **Indexability** — events should be queryable by external indexers; inconsistent naming breaks this

## Objective

1. Standardise event naming: `zerone.{module}.{action}` for all module events
2. Document every event type with attributes and trigger conditions
3. Add event schema validation test
4. Create an events reference doc for indexer builders

## Tasks

### Task 1: Event Naming Audit

Scan all `EmitEvent` calls across the codebase. Categorise:

```bash
grep -rn 'sdk.NewEvent(' x/ --include="*.go" | grep -v test | grep -v '.pb.' | \
    sed 's/.*sdk.NewEvent("\([^"]*\)".*/\1/' | sort -u
```

Fix any that don't follow `zerone.{module}.{verb_noun}` pattern:
- `knowledge_quality_observation` → `zerone.alignment.observation_recorded`
- `fact_status_changed` → `zerone.knowledge.fact_status_changed` (already has module prefix? verify)
- etc.

### Task 2: Event Registry

Create `docs/events.md` — a complete registry:

```markdown
## zerone.knowledge.fact_created
**Trigger:** MsgAddFact succeeds
**Attributes:**
| Key | Type | Description |
|-----|------|-------------|
| fact_id | string | Unique fact identifier |
| domain | string | Knowledge domain |
| submitter | string | Account address |
| initial_energy | uint64 | Starting metabolism energy |

## zerone.knowledge.epistemic_temperature_changed
**Trigger:** BeginBlocker temperature update crosses category boundary
**Attributes:**
...
```

### Task 3: Event Schema Test

Create `tests/integration/event_schema_test.go`:

```go
func TestEventNamingConvention(t *testing.T) {
    // Scan all event type strings in the codebase
    // Assert they match pattern: zerone\.{module}\.{snake_case_action}
    // No bare action names, no inconsistent prefixes
}

func TestEventAttributeCompleteness(t *testing.T) {
    // For each event type, verify minimum required attributes:
    // - All events must have at least one identifying attribute
    // - Events involving accounts must include the account address
    // - Events involving domains must include the domain
    // - Events involving heights must include the block height
}
```

### Task 4: Fix Inconsistent Events

Rename events that don't follow the convention. This is a breaking change for any existing indexers — but since we're pre-mainnet on testnet, now is the time.

## Tests

1. All event types match `zerone.{module}.{action}` pattern
2. All events have required minimum attributes
3. Event documentation covers every event type in the codebase
4. Existing event audit test still passes after renames

## Success Criteria

- `docs/events.md` is the authoritative event reference
- Every `sdk.NewEvent()` call uses the standard naming pattern
- External indexers can rely on consistent event structure
