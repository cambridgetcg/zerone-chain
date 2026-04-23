package app

import (
	"context"
	"fmt"
	"sort"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// ─── Wave 10: Module Version Registry ────────────────────────────────────
//
// A single, introspectable surface that reports every module's current
// ConsensusVersion along with the upgrade names this app registers handlers
// for. The registry is read-only — it reflects the module manager's live
// state — and is the primary observability tool for chain operators
// preparing an upgrade.
//
// Design: the module manager already tracks per-module ConsensusVersion
// via GetVersionMap. This file exposes that alongside the registered
// upgrade-handler lineage so a single query answers "where is this chain
// in its upgrade history, and what upgrades is it prepared to run?"

// ModuleVersionReport is a snapshot of one module's upgrade state.
type ModuleVersionReport struct {
	ModuleName       string `json:"module_name"`
	ConsensusVersion uint64 `json:"consensus_version"`
}

// UpgradeLineageEntry names an upgrade this app knows how to run.
// Included whether or not the upgrade has actually been applied.
type UpgradeLineageEntry struct {
	UpgradeName string `json:"upgrade_name"`
	Description string `json:"description"`
}

// ChainVersionReport is the complete upgrade snapshot returned to callers.
// Stable JSON schema so CLIs and dashboards can parse without proto changes.
type ChainVersionReport struct {
	// Per-module current ConsensusVersion, sorted by module name for
	// deterministic ordering.
	Modules []ModuleVersionReport `json:"modules"`
	// Every upgrade name for which this binary has a handler registered.
	// Replay of this list tells an operator "these are the upgrades I can
	// execute against this chain; everything before is already applied".
	KnownUpgrades []UpgradeLineageEntry `json:"known_upgrades"`
}

// BuildChainVersionReport returns the current upgrade snapshot. Safe to
// call at any time; pure read, no state mutation.
func (app *ZeroneApp) BuildChainVersionReport() ChainVersionReport {
	vm := app.ModuleManager.GetVersionMap()
	modules := make([]ModuleVersionReport, 0, len(vm))
	for name, ver := range vm {
		modules = append(modules, ModuleVersionReport{
			ModuleName:       name,
			ConsensusVersion: ver,
		})
	}
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].ModuleName < modules[j].ModuleName
	})

	// Upgrade lineage — hard-coded list that must be updated when a new
	// RegisterUpgradeHandler call lands. Kept adjacent to the handlers for
	// coherence; the test in Wave 10.2 asserts parity between this list
	// and the registered handlers so drift gets caught immediately.
	known := []UpgradeLineageEntry{
		{
			UpgradeName: UpgradeNameTestnet,
			Description: "v1.0.0-testnet — initial testnet launch; runs all module migrations from v1→v2.",
		},
		{
			UpgradeName: UpgradeNameTestnetV2,
			Description: "v1.0.1-testnet — simulated migration upgrade; writes verifiable markers to confirm pipeline ran.",
		},
		{
			UpgradeName: UpgradeNameTestnetV3,
			Description: "v1.0.2-testnet — Wave 10 reference upgrade exercising knowledge v3→v4 (TraceSchema backfill + marker).",
		},
	}

	return ChainVersionReport{
		Modules:       modules,
		KnownUpgrades: known,
	}
}

// KnownUpgradeNames returns just the upgrade names — convenient for tests
// that assert registration parity.
func (app *ZeroneApp) KnownUpgradeNames() []string {
	report := app.BuildChainVersionReport()
	names := make([]string, len(report.KnownUpgrades))
	for i, e := range report.KnownUpgrades {
		names[i] = e.UpgradeName
	}
	return names
}

// RunUpgradeHandlerForTests synchronously applies a scheduled upgrade.
// It seeds the module-version map (so ModuleManager.RunMigrations sees
// older versions for every module named in fromVM), writes the plan,
// invokes the upgrade's registered handler via ApplyUpgrade, and returns
// the post-upgrade VersionMap.
//
// Intended for end-to-end upgrade tests that want to exercise the full
// migration pipeline without spinning up the upgrade module's block-
// height plan machinery. Production flow on a live chain goes through
// governance + the upgrade module's BeginBlocker, not this function.
func (app *ZeroneApp) RunUpgradeHandlerForTests(ctx context.Context, name string, fromVM module.VersionMap, height int64) (module.VersionMap, error) {
	if !app.UpgradeKeeper.HasHandler(name) {
		return nil, fmt.Errorf("no upgrade handler registered for %q", name)
	}
	// Seed the on-chain module-version map to the pre-upgrade state so
	// RunMigrations detects the correct delta per module.
	if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, fromVM); err != nil {
		return nil, fmt.Errorf("seed pre-upgrade vm: %w", err)
	}
	plan := upgradetypes.Plan{Name: name, Height: height, Info: "e2e test"}
	if err := app.UpgradeKeeper.ApplyUpgrade(ctx, plan); err != nil {
		return nil, fmt.Errorf("apply upgrade: %w", err)
	}
	return app.UpgradeKeeper.GetModuleVersionMap(ctx)
}

// CurrentModuleVersionMap returns the module manager's current
// ConsensusVersion map. The "from" side of an upgrade test starts from
// a synthetic map representing older versions; this helper returns the
// current "target" for assertions.
func (app *ZeroneApp) CurrentModuleVersionMap() module.VersionMap {
	return app.ModuleManager.GetVersionMap()
}
