package app

// RegisterUpgradeHandlers registers upgrade handlers for each named software upgrade.
// When a governance upgrade proposal passes, the corresponding handler here runs
// the necessary state migrations before the new binary starts producing blocks.
//
// Example (not active yet):
//
//	app.UpgradeKeeper.SetUpgradeHandler("v1.1.0",
//	    func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
//	        return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
//	    },
//	)
func (app *ZeroneApp) RegisterUpgradeHandlers() {
	// Future upgrade handlers are registered here as Zerone batches land.
	// See docs/upgrades/ for migration guides.
}
