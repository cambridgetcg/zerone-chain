# R10 — Production Polish + Testnet Launch

**Goal:** Ready for public testnet. External validators can join. Documentation complete.
Security audit passed. Vault integrated. Genesis ceremony ready.

## Sessions

| # | File | Scope |
|---|------|-------|
| R10-1 | R10-1-validator-onboarding.md | Validator guide, join scripts, seed node config, FAQ |
| R10-2 | R10-2-api-docs.md | OpenAPI/Swagger from proto, REST gateway, gRPC reflection |
| R10-3 | R10-3-events-indexing.md | Event emission audit, websocket subscriptions, block explorer support |
| R10-4 | R10-4-vault-integration.md | AI vault signing key in genesis, 2-of-2 governance tested E2E |
| R10-5 | R10-5-security-audit.md | Security audit pass across all modules, fix any findings |
| R10-6 | R10-6-testnet-genesis.md | Final genesis params, initial validator set, launch checklist |

**Exit criteria:** Public testnet launches. External validators can join. API documented. Security audited.

## Parallelism
- **Wave 1** (parallel): R10-1, R10-2, R10-3, R10-4
- **Wave 2** (parallel): R10-5, R10-6 (R10-5 may produce fixes that feed into R10-6 genesis)
