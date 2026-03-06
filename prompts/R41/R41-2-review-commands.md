# R41-2 — Review CLI Commands

## Objective

Add CLI transaction commands for agents to review submissions in the commit-reveal flow.

## Commands

### `zeroned tx knowledge commit-review`

Commit a sealed score for a submission in an open quality round.

```
zeroned tx knowledge commit-review \
  --round-id <round-id> \
  --score 85 \
  --salt "my-secret-salt" \
  --from reviewer1
```

**Behavior:**
1. Query round details (submission, phase, required reviewer stake)
2. Compute seal: `SHA-256(score || salt || reviewer_address)`
3. Escrow reviewer stake (0.3× submitter stake)
4. Broadcast `MsgCommitScore`
5. Save salt locally to `~/.zeroned/review-salts/<round-id>.json` for auto-reveal
6. Display: round ID, stake escrowed, reveal deadline

### `zeroned tx knowledge reveal-review`

Reveal a previously committed score.

```
zeroned tx knowledge reveal-review \
  --round-id <round-id> \
  --from reviewer1
```

**Behavior:**
1. Load salt from `~/.zeroned/review-salts/<round-id>.json`
2. Broadcast `MsgRevealScore` with score + salt
3. Display: revealed score, current round status

**Auto-reveal mode:**
```
zeroned tx knowledge reveal-review --auto --from reviewer1
```
Reveals all pending committed reviews that are in reveal phase.

### `zeroned tx knowledge contest-sample`

Contest an accepted sample (trigger re-review).

```
zeroned tx knowledge contest-sample \
  --sample-id <sample-id> \
  --reason "Contains factual errors in code example" \
  --stake 2000000uzrn \
  --from challenger1
```

### `zeroned tx knowledge attest-storage`

Validator attests proof-of-storage for assigned shard.

```
zeroned tx knowledge attest-storage \
  --snapshot-height <height> \
  --data-hash <sha256-of-assigned-tdu-data> \
  --from validator1
```

## Key Files

- `x/knowledge/client/cli/tx.go` — add commands
- Create `~/.zeroned/review-salts/` directory structure for salt management

## Constraints

- Salt storage is local only — losing salts means losing the ability to reveal (and forfeiting stake)
- Auto-reveal should warn about pending reveals approaching deadline
- Contest stake minimum = base_stake (queries from params)
