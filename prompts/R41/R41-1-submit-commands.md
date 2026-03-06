# R41-1 — Submission CLI Commands

## Objective

Add CLI transaction commands for agents to submit training data to the Tree of Knowledge.

## Commands

### `zeroned tx knowledge submit-data`

Submit a single TDU (training data unit).

```
zeroned tx knowledge submit-data \
  --type instruction-response \
  --domain code \
  --difficulty standard \
  --content-file ./my-training-pair.json \
  --consent-type original \
  --from agent1
```

**Flags:**
- `--type` — TDU type: `instruction-response`, `conversation`, `correction`, `grounding-fact`, `reasoning-chain`
- `--domain` — target domain (e.g., `code`, `math`, `general`)
- `--difficulty` — `basic` (1×), `standard` (1.5×), `advanced` (2×), `expert` (3×), `frontier` (5×)
- `--content-file` — path to JSON content file (schema depends on type)
- `--consent-type` — `original` (self-authored), `public-domain`, `licensed` (requires proof)
- `--consent-proof` — optional path to consent proof file
- `--metadata` — optional JSON metadata string

**Behavior:**
1. Read content file, compute SHA-256 content hash
2. Calculate required stake: `base_stake × difficulty_multiplier`
3. Build and broadcast `MsgSubmitData`
4. Display: submission ID, stake amount, estimated review timeline

### `zeroned tx knowledge submit-thread`

Submit a multi-turn conversation as a TDU.

```
zeroned tx knowledge submit-thread \
  --domain code \
  --thread-file ./conversation.json \
  --from agent1
```

Thread file format: JSON array of `{role, content}` turns.

### `zeroned tx knowledge submit-correction`

Submit a correction that supersedes an existing TDU.

```
zeroned tx knowledge submit-correction \
  --target-id <tdu-id> \
  --correction-file ./fix.json \
  --reason "Incorrect API usage in example" \
  --from agent1
```

## Content File Schemas

### instruction-response
```json
{
  "instruction": "Write a Go function that...",
  "response": "```go\nfunc ...\n```",
  "system_prompt": "You are a Go expert."  // optional
}
```

### conversation
```json
[
  {"role": "user", "content": "How do I..."},
  {"role": "assistant", "content": "You can..."},
  {"role": "user", "content": "What about..."},
  {"role": "assistant", "content": "In that case..."}
]
```

### correction
```json
{
  "original_id": "<tdu-id>",
  "field": "response",
  "corrected": "The correct approach is...",
  "explanation": "The original was wrong because..."
}
```

## Key Files

- `x/knowledge/client/cli/tx.go` — add commands
- `x/knowledge/types/` — content schemas and validation

## Constraints

- Content hash computed client-side before broadcast
- Stake auto-calculated from params (query chain for current base_stake and multipliers)
- Maximum content size: 1MB per submission
