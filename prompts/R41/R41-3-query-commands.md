# R41-3 — Query CLI Commands

## Objective

Add CLI query commands for agents to inspect their state, browse submissions, check reputation, and monitor the ToK ecosystem.

## Commands

### Agent Dashboard
```
zeroned q knowledge dashboard --address <agent-addr>
```
Shows: total submissions, acceptance rate, reputation per domain, active stakes, fitness scores of submitted TDUs, pending reviews.

### Submissions
```
zeroned q knowledge submissions --status pending|accepted|rejected --domain code --limit 20
zeroned q knowledge submission <submission-id>
```

### Samples (accepted TDUs)
```
zeroned q knowledge samples --domain code --fitness-min 0.7 --limit 50
zeroned q knowledge sample <sample-id>
```

### Reputation
```
zeroned q knowledge reputation --address <agent-addr>
zeroned q knowledge reputation --address <agent-addr> --domain code
zeroned q knowledge leaderboard --domain code --limit 10
```

### Stakes
```
zeroned q knowledge stakes --address <agent-addr>
zeroned q knowledge active-rounds --reviewer <agent-addr>
```

### Fitness
```
zeroned q knowledge fitness <tdu-id>
zeroned q knowledge fitness-summary --status core|active|dormant|pruned --domain code
```

### Sharding
```
zeroned q knowledge shard --validator <val-addr> --snapshot-height <height>
zeroned q knowledge shard-status  # current snapshot, next reshuffle, attestation stats
```

### Domains & Bounties
```
zeroned q knowledge domains
zeroned q knowledge bounties --domain code --status open
```

### Quality Rounds
```
zeroned q knowledge rounds --status commit|reveal|aggregating --limit 10
zeroned q knowledge round <round-id>
```

### Params
```
zeroned q knowledge params
zeroned q knowledge params --section staking|fitness|sharding|reputation
```

## Output Formatting

- Default: human-readable table format
- `--output json` for machine-readable
- Colors for status indicators (green=Core, yellow=Active, red=Dormant, gray=Pruned)
- Reputation shows as percentage bar: `████████░░ 82%`

## Key Files

- `x/knowledge/client/cli/query.go` — add commands
- Wire to existing gRPC query handlers in `grpc_query.go`

## Constraints

- All queries use existing gRPC query endpoints (R38-4 consumer API)
- Pagination via standard Cosmos SDK page/limit/offset
- Dashboard is a convenience aggregation of multiple queries
