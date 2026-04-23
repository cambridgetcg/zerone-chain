package app

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/types/module"
)

const UpgradeNameTestnet = "v1.0.0-testnet"
const UpgradeNameTestnetV2 = "v1.0.1-testnet"
const UpgradeNameTestnetV3 = "v1.0.2-testnet"

// RegisterUpgradeHandlers registers upgrade handlers for each named software upgrade.
// When a governance upgrade proposal passes, the corresponding handler here runs
// the necessary state migrations before the new binary starts producing blocks.
//
// Call this AFTER RegisterServices but BEFORE LoadLatestVersion.
func (app *ZeroneApp) RegisterUpgradeHandlers() {
	// v1.0.0-testnet — initial testnet launch.
	// Runs all module migrations from ConsensusVersion 1 → 2.
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeNameTestnet,
		func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			app.Logger().Info(fmt.Sprintf("applying upgrade %q at height %d", plan.Name, plan.Height))
			return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		},
	)

	// v1.0.1-testnet — simulated upgrade for testing the migration pipeline.
	// Runs module migrations (knowledge v1→v2 writes a verifiable marker) and
	// writes its own upgrade marker to the knowledge store.
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeNameTestnetV2,
		func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			app.Logger().Info(fmt.Sprintf("applying upgrade %q at height %d", plan.Name, plan.Height))

			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return nil, err
			}

			// Handler-level marker (via the knowledge keeper's marker API)
			// to prove this named upgrade handler executed. Tests read it
			// via ReadMigrationMarker.
			if err := app.KnowledgeKeeper.WriteMigrationMarker(ctx, "upgrade_marker_v1.0.1", "migrated"); err != nil {
				return nil, err
			}

			return toVM, nil
		},
	)

	// v1.0.2-testnet — Wave 10 reference upgrade exercising the v3→v4
	// knowledge migration (TraceSchema backfill + v4 marker). Also used by
	// the end-to-end upgrade test to verify the full pipeline works.
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeNameTestnetV3,
		func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			app.Logger().Info(fmt.Sprintf("applying upgrade %q at height %d", plan.Name, plan.Height))

			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return nil, err
			}

			// Handler-level marker — tests assert both the per-module v4
			// marker (written by the migrator) AND this handler-level marker
			// were recorded, proving both layers ran.
			if err := app.KnowledgeKeeper.WriteMigrationMarker(ctx, "upgrade_marker_v1.0.2", "migrated"); err != nil {
				return nil, err
			}

			return toVM, nil
		},
	)
}

// RegisterStoreUpgrades configures store loaders for upgrades that add or remove
// module store keys. Call this BEFORE LoadLatestVersion.
func (app *ZeroneApp) RegisterStoreUpgrades() {
	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		// No pending upgrade — nothing to do.
		return
	}

	switch upgradeInfo.Name {
	case UpgradeNameTestnet:
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{
				// Add new module store keys here when the upgrade introduces them.
			},
		}
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))

	case UpgradeNameTestnetV2:
		// No new store keys for v1.0.1-testnet — migration-only upgrade.
		storeUpgrades := storetypes.StoreUpgrades{}
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))

	case UpgradeNameTestnetV3:
		// v1.0.2-testnet — Wave 10 reference upgrade. No new store keys;
		// knowledge v3→v4 migration only touches existing prefixes.
		storeUpgrades := storetypes.StoreUpgrades{}
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
