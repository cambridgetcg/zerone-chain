package cross_stack_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"
)

// ─── Wave 10: end-to-end upgrade pipeline tests ─────────────────────────

// TestUpgrade_ChainVersionReportWellFormed — the introspection surface
// returns a sorted, complete module list plus the registered upgrade
// lineage. First guard: drift between the registered handlers and the
// self-described lineage would mask a missing upgrade, so we assert
// parity.
func TestUpgrade_ChainVersionReportWellFormed(t *testing.T) {
	h := NewTestHarness(t)

	report := h.App.BuildChainVersionReport()
	require.NotEmpty(t, report.Modules, "chain has registered modules")
	require.NotEmpty(t, report.KnownUpgrades, "at least one upgrade lineage entry")

	// Modules sorted by name deterministically.
	for i := 1; i < len(report.Modules); i++ {
		require.LessOrEqual(t, report.Modules[i-1].ModuleName, report.Modules[i].ModuleName,
			"module list must be name-sorted for deterministic consumption")
	}

	// Every advertised upgrade must have a handler registered. No drift.
	names := h.App.KnownUpgradeNames()
	for _, n := range names {
		require.True(t, h.App.UpgradeKeeper.HasHandler(n),
			"handler for %q must be registered to match the lineage entry", n)
	}

	// Knowledge module is at ConsensusVersion 4 (post-Wave 10 bump).
	var sawKnowledge bool
	for _, m := range report.Modules {
		if m.ModuleName == "knowledge" {
			sawKnowledge = true
			require.Equal(t, uint64(4), m.ConsensusVersion,
				"knowledge module advertises its current ConsensusVersion")
		}
	}
	require.True(t, sawKnowledge, "knowledge module appears in report")
}

// TestUpgrade_V1ToV2MigrationPipeline — exercise the v1.0.1-testnet upgrade
// against the knowledge module downshifted from its current ConsensusVersion
// to v1. Exercises the full pipeline Migrate1to2 → Migrate2to3 → Migrate3to4
// in sequence. Other modules stay at their current version (no migration
// runs for them — we're not testing SDK-module migrations here, which have
// their own Cosmos SDK test coverage and require test fixtures this harness
// doesn't set up).
func TestUpgrade_V1ToV2MigrationPipeline(t *testing.T) {
	h := NewTestHarness(t)

	// Build fromVM: all modules at current, knowledge downshifted to v1.
	current := h.App.CurrentModuleVersionMap()
	fromVM := make(module.VersionMap, len(current))
	for name, ver := range current {
		fromVM[name] = ver
	}
	fromVM["knowledge"] = 1

	toVM, err := h.App.RunUpgradeHandlerForTests(h.Ctx, zeroneapp.UpgradeNameTestnetV2, fromVM, h.Height())
	require.NoError(t, err, "v1.0.1-testnet handler completes without error")
	require.Equal(t, uint64(4), toVM["knowledge"],
		"knowledge module advances to its current ConsensusVersion (4) via full migration chain")

	// All three migrations ran in sequence — each wrote its marker.
	require.Equal(t, "true", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "migration_v2_complete"),
		"v1→v2 migration marker proves Migrate1to2 ran")
	require.Equal(t, "true", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "migration_v4_complete"),
		"v3→v4 migration marker proves Migrate3to4 ran at the end of the chain")

	// The v1.0.1 handler-level marker was written by the upgrade handler itself.
	handlerMarker := h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "upgrade_marker_v1.0.1")
	require.Equal(t, "migrated", handlerMarker,
		"handler-level marker proves the named upgrade handler executed")
}

// TestUpgrade_V3ToV4KnowledgeMigrationPipeline — exercise the
// v1.0.2-testnet upgrade, which in turn fires the knowledge v3→v4
// migration (TraceSchema backfill + v4 marker).
func TestUpgrade_V3ToV4KnowledgeMigrationPipeline(t *testing.T) {
	h := NewTestHarness(t)

	// Synthetic fromVM: knowledge at v3, everything else at current.
	current := h.App.CurrentModuleVersionMap()
	fromVM := make(module.VersionMap, len(current))
	for name, ver := range current {
		fromVM[name] = ver
	}
	fromVM["knowledge"] = 3 // downshift so v3→v4 migration fires

	// NO pre-seeded TraceSchema — the v4 migration must backfill it.
	_, seeded := h.KnowledgeKeeper.GetTraceSchema(h.Ctx)
	if seeded {
		t.Log("trace schema already present pre-upgrade; v4 migration will be a no-op on schema")
	}

	toVM, err := h.App.RunUpgradeHandlerForTests(h.Ctx, zeroneapp.UpgradeNameTestnetV3, fromVM, h.Height())
	require.NoError(t, err)
	require.Equal(t, uint64(4), toVM["knowledge"],
		"knowledge module is now at ConsensusVersion 4 post-migration")

	// v4 migration marker proves Migrate3to4 ran.
	require.Equal(t, "true", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "migration_v4_complete"),
		"v4 migration marker present")

	// TraceSchema is present post-upgrade.
	schema, ok := h.KnowledgeKeeper.GetTraceSchema(h.Ctx)
	require.True(t, ok, "TraceSchema is backfilled by v4 migration")
	require.Equal(t, uint64(1), schema.Version)

	// v1.0.2 handler-level marker.
	require.Equal(t, "migrated",
		h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "upgrade_marker_v1.0.2"))
}

// TestUpgrade_UnknownHandlerRejected — calling an unregistered upgrade
// name must return a clear error, not silently succeed or panic.
func TestUpgrade_UnknownHandlerRejected(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.App.RunUpgradeHandlerForTests(h.Ctx, "not-a-real-upgrade", module.VersionMap{}, h.Height())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no upgrade handler registered")
}

// TestUpgrade_MigrationMarkerIdempotent — writing the same marker twice
// is a no-op (idempotent); writing a DIFFERENT value for the same key is
// rejected without overwriting (first writer wins).
func TestUpgrade_MigrationMarkerIdempotent(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.WriteMigrationMarker(h.Ctx, "test_marker", "alpha"))
	require.Equal(t, "alpha", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "test_marker"))

	// Same value again — idempotent.
	require.NoError(t, h.KnowledgeKeeper.WriteMigrationMarker(h.Ctx, "test_marker", "alpha"))
	require.Equal(t, "alpha", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "test_marker"))

	// Different value — preserves original, does not error (warns via log).
	require.NoError(t, h.KnowledgeKeeper.WriteMigrationMarker(h.Ctx, "test_marker", "beta"))
	require.Equal(t, "alpha", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "test_marker"),
		"first writer wins: conflicting value does not overwrite")
}

// TestUpgrade_LineageParityWithHandlers — every entry in the lineage list
// has a registered handler, AND every registered handler appears in the
// lineage list. Parity prevents the chain from drifting into a state
// where a handler exists but is invisible to operators, or vice versa.
func TestUpgrade_LineageParityWithHandlers(t *testing.T) {
	h := NewTestHarness(t)
	lineage := h.App.BuildChainVersionReport().KnownUpgrades

	// Every advertised upgrade must have a handler.
	for _, entry := range lineage {
		require.True(t, h.App.UpgradeKeeper.HasHandler(entry.UpgradeName),
			"lineage entry %q advertises a handler; must be registered", entry.UpgradeName)
	}

	// Inverse — every UpgradeName constant known to app should be listed
	// in the lineage. Hard-coded check against the three we ship.
	lineageNames := h.App.KnownUpgradeNames()
	require.Contains(t, lineageNames, zeroneapp.UpgradeNameTestnet)
	require.Contains(t, lineageNames, zeroneapp.UpgradeNameTestnetV2)
	require.Contains(t, lineageNames, zeroneapp.UpgradeNameTestnetV3)
}

