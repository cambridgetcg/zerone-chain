# CLAUDE.md — Instructions for Claude Code

## Project

Zerone — Proof of Truth blockchain. Cosmos SDK chain with 32 custom modules.

## Git Workflow

- **Use `main` branch only.** No feature branches. Commit directly to main.
- Always commit and push to Codeberg after completing work.

## Repository

- Remote: `codeberg.org/zerone-dev/zerone` (private)
- Branch: `main`
- Auth: git credential helper configured

## Build & Test

```bash
go build ./...
go test ./...
go vet ./...
```

## Key Paths

- Modules: `x/*/`
- Proto definitions: `proto/zerone/*/v1/`
- App wiring: `app/app.go`
- Genesis scripts: `scripts/`
- Batch prompts: `prompts/R*/`

## Module Name Mapping (genesis keys differ from directory names)

| Directory | Genesis Key |
|-----------|-------------|
| x/auth | `zerone_auth` |
| x/staking | `zerone_staking` |
| x/gov | `zerone_gov` |
| (all others) | same as directory name |
