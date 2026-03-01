# R34-4 — CLI Completeness Audit

## Objective

Ensure every message type and query in every module has a working CLI command. Operators and users interact with the chain via CLI — missing commands are missing functionality.

## Tasks

### 1. CLI audit

For each module, compare:
- Proto `Msg` service RPCs → must have a `tx` CLI command
- Proto `Query` service RPCs → must have a `query` CLI command

```bash
# Count proto RPCs vs CLI commands for each module
for mod in $(ls x/); do
    proto_msgs=$(grep "rpc " proto/zerone/$mod/v1/tx.proto 2>/dev/null | wc -l)
    proto_queries=$(grep "rpc " proto/zerone/$mod/v1/query.proto 2>/dev/null | wc -l)
    cli_tx=$(grep "func.*TxCmd\|RunE:" x/$mod/client/cli/tx*.go 2>/dev/null | wc -l)
    cli_query=$(grep "func.*QueryCmd\|RunE:" x/$mod/client/cli/query*.go 2>/dev/null | wc -l)
    echo "$mod: msgs=$proto_msgs/$cli_tx queries=$proto_queries/$cli_query"
done
```

### 2. Missing command implementation

For each missing CLI command:
- Implement the command with proper flags
- Add flag validation
- Add `--output json` support
- Write a basic CLI test

### 3. CLI integration tests

For each module, create `x/<module>/client/cli/cli_test.go`:
- Test each tx command with `--dry-run`
- Test each query command against a test network
- Test flag validation (missing required flags, invalid values)
- Test JSON output formatting

### 4. Help text audit

Every command must have:
- `Short` description (one line)
- `Long` description (usage example)
- `Example` with realistic usage
- Proper flag descriptions

### 5. Auto-complete support

Register shell completion for custom flags:
- `--domain` → list known domains
- `--proposal-id` → list active proposals
- `--partnership-id` → list partnerships

## Acceptance Criteria

- [ ] Every proto Msg RPC has a corresponding tx CLI command
- [ ] Every proto Query RPC has a corresponding query CLI command
- [ ] All commands have help text with examples
- [ ] CLI integration tests pass for all modules
- [ ] `zeroned tx --help` and `zeroned query --help` list all subcommands
