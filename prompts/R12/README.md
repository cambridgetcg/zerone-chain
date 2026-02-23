# R12 — Upgradability: Migration Wiring, LIP→Upgrade Bridge, Governance Param Updates

**Goal:** Every module can migrate state during chain upgrades. Governance can schedule software upgrades via LIPs. All modules have governance-adjustable parameters. The ParamRouter is wired so parameter-category LIPs execute.

## The Problem

The prototype (Legible Money) had a complete upgrade pipeline:
- Every module registered migration stubs via `RegisterMigration`
- Consensus-category LIPs could carry `UpgradePlan` payloads that bridged to `x/upgrade`
- Every module had `MsgUpdateParams` for governance-adjustable parameters

The Zerone rewrite carried over the _structure_ (migrations dirs, ConsensusVersion, cosmovisor) but not the _wiring_:

1. **25 of 30 modules** have `migrations/v2/migrate.go` stubs but no `RegisterMigration` call in `module.go` and no `keeper/migrator.go`. The SDK's `RunMigrations` won't call them.
2. **LIP→upgrade bridge** is completely absent — no `MsgAttachUpgradePlan`, no `UpgradeKeeper` interface, no `ScheduleUpgrade` call on LIP acceptance.
3. **5 modules** (`capture_challenge`, `capture_defense`, `claiming_pot`, `disputes`, `home`) lack `MsgUpdateParams`, making their parameters immutable without a binary upgrade.
4. **ParamRouter** in `executeParamChanges` is a stub that logs but doesn't execute.

## Sessions (5)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R12-1 | R12-1-migration-wiring.md | Wire RegisterMigration + create migrator.go for 25 modules | Wave 1 |
| R12-2 | R12-2-lip-upgrade-bridge.md | Proto + keeper + app wiring for LIP→x/upgrade pipeline | Wave 1 |
| R12-3 | R12-3-missing-update-params.md | Add MsgUpdateParams to 5 missing modules | Wave 1 |
| R12-4 | R12-4-param-router.md | Wire ParamRouter in app.go so parameter-category LIPs execute | Wave 2 (after R12-3) |
| R12-5 | R12-5-verify.md | Full verification: build, test, upgrade simulation | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R12-1, R12-2, R12-3
- **Wave 2:** R12-4 (depends on R12-3 for MsgUpdateParams endpoints)
- **Wave 3:** R12-5 (depends on all)

## Exit Criteria

1. All 30 custom modules have `RegisterMigration(moduleName, 1, migrator.Migrate1to2)` in `RegisterServices`
2. All 30 custom modules have `keeper/migrator.go` with `Migrate1to2` stub
3. `MsgAttachUpgradePlan` proto-defined, registered, handler implemented
4. Accepted upgrade-category LIPs call `ScheduleUpgrade` on the upgrade keeper
5. All 30 modules have `MsgUpdateParams` (authority-gated)
6. Parameter-category LIPs execute param changes via a wired `ParamRouter`
7. `go build ./...` — clean
8. `go test ./...` — all pass
9. Upgrade simulation test: submit LIP → attach upgrade plan → tally → verify `ScheduleUpgrade` called
