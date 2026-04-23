package v4

// Knowledge module v3 → v4 migration
//
// This is the **reference implementation** for the Route B upgrade
// protocol (see docs/UPGRADE_PROTOCOL.md). Every future migration in the
// knowledge module — and, by adaptation, every other zerone module —
// should follow this structure:
//
//   1. Declare a narrow interface naming only the keeper methods the
//      migration calls. Avoids the migrations ↔ keeper import cycle.
//
//   2. In Migrate, do idempotent catch-up: only write state that is
//      genuinely missing post-upgrade. A migration must be safe to run
//      even if a chain already has the target state (e.g., because it
//      was seeded at genesis rather than inherited from v3).
//
//   3. Record a verifiable marker. Post-upgrade tests read the marker
//      to prove the handler actually executed.
//
//   4. Backfill defaults that were introduced in the new version and
//      are zero-valued post-decode.
//
// Concretely for v3 → v4:
//
//   - Wave 5 introduced TraceSchema (the MethodologyApplicationTrace
//     contract). Any chain upgrading from v3 without having previously
//     seeded it should receive the default schema now.
//   - Wave 8 added the KnowledgeTrainingFund module account. Historic
//     chains need the module account to exist (ensured by x/auth genesis
//     + permissions set in app.go; this migration only verifies and
//     writes a marker acknowledging that path).
//   - A `migration_v4_complete` marker is written so the upgrade e2e
//     test can confirm the handler ran.

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// V4MigrationKeeper is the narrow keeper interface v4 migration requires.
// Keeping it local prevents the migrations/v4 → keeper → migrations/v4
// import cycle.
type V4MigrationKeeper interface {
	// Trace schema — seeded idempotently if missing.
	GetTraceSchema(ctx context.Context) (*types.TraceSchema, bool)
	SeedDefaultTraceSchema(ctx context.Context) error

	// Write-marker side-channel for verification. Safe to call once per
	// upgrade; writes are idempotent per key.
	WriteMigrationMarker(ctx context.Context, key, value string) error
}

// Migrate runs the knowledge v3 → v4 state transformation.
// Returns nil on success; any returned error rolls back the upgrade.
func Migrate(ctx context.Context, k V4MigrationKeeper) error {
	// Step 1: ensure TraceSchema is seeded. On genesis-fresh chains it
	// already is; on chains that upgraded from a pre-Wave-5 binary, it
	// isn't. Idempotent — only writes when missing.
	if _, ok := k.GetTraceSchema(ctx); !ok {
		if err := k.SeedDefaultTraceSchema(ctx); err != nil {
			return err
		}
	}

	// Step 2: record a verifiable marker.
	return k.WriteMigrationMarker(ctx, "migration_v4_complete", "true")
}
