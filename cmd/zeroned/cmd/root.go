package cmd

import (
	"io"
	"os"

	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/log"
	sdkevidencecli "cosmossdk.io/x/evidence/client/cli"
	feegrantcli "cosmossdk.io/x/feegrant/client/cli"
	upgradecli "cosmossdk.io/x/upgrade/client/cli"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdkauthcli "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdkbankcli "github.com/cosmos/cosmos-sdk/x/bank/client/cli"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	sdkdistrcli "github.com/cosmos/cosmos-sdk/x/distribution/client/cli"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	sdkgovcli "github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	sdkstakingcli "github.com/cosmos/cosmos-sdk/x/staking/client/cli"

	"github.com/zerone-chain/zerone/app"
	alignmentcli "github.com/zerone-chain/zerone/x/alignment/client/cli"
	zeroneauthcli "github.com/zerone-chain/zerone/x/auth/client/cli"
	autopoiesiscli "github.com/zerone-chain/zerone/x/autopoiesis/client/cli"
	billingcli "github.com/zerone-chain/zerone/x/billing/client/cli"
	bvmcli "github.com/zerone-chain/zerone/x/bvm/client/cli"
	capturechallengecli "github.com/zerone-chain/zerone/x/capture_challenge/client/cli"
	capturedefensecli "github.com/zerone-chain/zerone/x/capture_defense/client/cli"
	channelscli "github.com/zerone-chain/zerone/x/channels/client/cli"
	claimingpotcli "github.com/zerone-chain/zerone/x/claiming_pot/client/cli"
	computepoolcli "github.com/zerone-chain/zerone/x/compute_pool/client/cli"
	discoverycli "github.com/zerone-chain/zerone/x/discovery/client/cli"
	disputescli "github.com/zerone-chain/zerone/x/disputes/client/cli"
	emergencycli "github.com/zerone-chain/zerone/x/emergency/client/cli"
	evidencemgmtcli "github.com/zerone-chain/zerone/x/evidence_mgmt/client/cli"
	govcli "github.com/zerone-chain/zerone/x/gov/client/cli"
	homecli "github.com/zerone-chain/zerone/x/home/client/cli"
	ibcratelimitcli "github.com/zerone-chain/zerone/x/ibcratelimit/client/cli"
	icaauthcli "github.com/zerone-chain/zerone/x/icaauth/client/cli"
	knowledgecli "github.com/zerone-chain/zerone/x/knowledge/client/cli"
	liquiditypoolcli "github.com/zerone-chain/zerone/x/liquiditypool/client/cli"
	ontologycli "github.com/zerone-chain/zerone/x/ontology/client/cli"
	partnershipscli "github.com/zerone-chain/zerone/x/partnerships/client/cli"
	qualificationcli "github.com/zerone-chain/zerone/x/qualification/client/cli"
	researchcli "github.com/zerone-chain/zerone/x/research/client/cli"
	schedulecli "github.com/zerone-chain/zerone/x/schedule/client/cli"
	stakingcli "github.com/zerone-chain/zerone/x/staking/client/cli"
	tokenscli "github.com/zerone-chain/zerone/x/tokens/client/cli"
	toolboxcli "github.com/zerone-chain/zerone/x/toolbox/client/cli"
	treecli "github.com/zerone-chain/zerone/x/tree/client/cli"
	vestingrewardscli "github.com/zerone-chain/zerone/x/vesting_rewards/client/cli"
)

// NewRootCmd creates the root command for the zeroned daemon.
func NewRootCmd() *cobra.Command {
	encodingConfig := app.MakeEncodingConfig()

	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("")

	rootCmd := &cobra.Command{
		Use:   app.AppName,
		Short: "Zerone blockchain node — Proof of Truth consensus",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			customAppTemplate, customAppConfig := initAppConfig()
			customCMTConfig := initCometBFTConfig()

			return server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customCMTConfig)
		},
	}

	initRootCmd(rootCmd, encodingConfig)

	// AutoCLI: generate query/tx commands for SDK modules (bank, auth,
	// staking, distribution, gov, slashing, etc.) from protobuf service
	// definitions. Only adds commands that are not already registered
	// manually, so existing custom module commands are preserved.
	tempApp := app.NewZeroneApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		false,
		emptyAppOptions{},
	)

	moduleMap := make(map[string]appmodule.AppModule)
	for name, mod := range tempApp.ModuleManager.Modules {
		if m, ok := mod.(appmodule.AppModule); ok {
			moduleMap[name] = m
		}
	}

	autoCliOpts := autocli.AppOptions{
		Modules:               moduleMap,
		AddressCodec:          addresscodec.NewBech32Codec(app.AccountAddressPrefix),
		ValidatorAddressCodec: runtime.ValidatorAddressCodec(addresscodec.NewBech32Codec(app.AccountAddressPrefix + "valoper")),
		ConsensusAddressCodec: runtime.ConsensusAddressCodec(addresscodec.NewBech32Codec(app.AccountAddressPrefix + "valcons")),
		ClientCtx:             initClientCtx,
	}

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd
}

// initRootCmd registers all sub-commands on the root command.
func initRootCmd(rootCmd *cobra.Command, encodingConfig app.EncodingConfig) {
	rootCmd.AddCommand(
		genutilcli.InitCmd(app.ModuleBasics, app.DefaultNodeHome),
		debug.Cmd(),
		pruning.Cmd(newApp, app.DefaultNodeHome),
		snapshot.Cmd(newApp),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, appExport, addModuleInitFlags)

	// Key management
	rootCmd.AddCommand(keys.Commands())

	// Genesis subcommand
	rootCmd.AddCommand(genesisCommand(encodingConfig))

	// Top-level add-genesis-account (expected by boot-test.sh)
	rootCmd.AddCommand(AddGenesisAccountCmd(app.DefaultNodeHome))

	// Status / query / tx
	rootCmd.AddCommand(server.StatusCommand())
	rootCmd.AddCommand(queryCommand(encodingConfig))
	rootCmd.AddCommand(txCommand(encodingConfig))
}

// addModuleInitFlags adds module-specific init flags to the start command.
func addModuleInitFlags(startCmd *cobra.Command) {
	// Module-specific flags can be added here if needed.
}

// genesisCommand returns the genesis subcommand tree.
func genesisCommand(encodingConfig app.EncodingConfig) *cobra.Command {
	gentxModule := app.ModuleBasics[genutiltypes.ModuleName].(genutil.AppModuleBasic)

	cmd := &cobra.Command{
		Use:                        "genesis",
		Short:                      "Application genesis-related subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		genutilcli.GenTxCmd(
			app.ModuleBasics,
			encodingConfig.TxConfig,
			banktypes.GenesisBalancesIterator{},
			app.DefaultNodeHome,
			encodingConfig.InterfaceRegistry.SigningContext().ValidatorAddressCodec(),
		),
		genutilcli.CollectGenTxsCmd(
			banktypes.GenesisBalancesIterator{},
			app.DefaultNodeHome,
			gentxModule.GenTxValidator,
			encodingConfig.InterfaceRegistry.SigningContext().ValidatorAddressCodec(),
		),
		genutilcli.ValidateGenesisCmd(app.ModuleBasics),
	)

	return cmd
}

// queryCommand returns the root query command.
func queryCommand(_ app.EncodingConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		server.QueryBlockCmd(),
		server.QueryBlocksCmd(),
		server.QueryBlockResultsCmd(),
	)

	// SDK auth utility query commands
	cmd.AddCommand(
		sdkauthcli.QueryTxsByEventsCmd(),
		sdkauthcli.QueryTxCmd(),
	)

	// Zerone custom module query commands
	cmd.AddCommand(
		alignmentcli.NewQueryCmd(),
		zeroneauthcli.GetQueryCmd(),
		autopoiesiscli.NewQueryCmd(),
		billingcli.NewQueryCmd(),
		bvmcli.NewQueryCmd(),
		capturechallengecli.NewQueryCmd(),
		capturedefensecli.NewQueryCmd(),
		channelscli.NewQueryCmd(),
		claimingpotcli.NewQueryCmd(),
		computepoolcli.NewQueryCmd(),
		discoverycli.NewQueryCmd(),
		disputescli.NewQueryCmd(),
		emergencycli.NewQueryCmd(),
		evidencemgmtcli.NewQueryCmd(),
		govcli.NewQueryCmd(),
		homecli.NewQueryCmd(),
		ibcratelimitcli.NewQueryCmd(),
		icaauthcli.NewQueryCmd(),
		knowledgecli.GetQueryCmd(),
		liquiditypoolcli.NewQueryCmd(),
		ontologycli.NewQueryCmd(),
		partnershipscli.NewQueryCmd(),
		qualificationcli.NewQueryCmd(),
		researchcli.NewQueryCmd(),
		schedulecli.NewQueryCmd(),
		stakingcli.GetQueryCmd(),
		tokenscli.NewQueryCmd(),
		toolboxcli.NewQueryCmd(),
		treecli.NewQueryCmd(),
		vestingrewardscli.NewQueryCmd(),
	)

	return cmd
}

// txCommand returns the root tx command.
func txCommand(encodingConfig app.EncodingConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		flags.LineBreak,
	)

	// SDK auth utility tx commands
	cmd.AddCommand(
		sdkauthcli.GetEncodeCommand(),
		sdkauthcli.GetDecodeCommand(),
		sdkauthcli.GetBroadcastCommand(),
		sdkauthcli.GetSignCommand(),
		sdkauthcli.GetSignBatchCommand(),
		sdkauthcli.GetMultiSignCommand(),
		sdkauthcli.GetMultiSignBatchCmd(),
		sdkauthcli.GetValidateSignaturesCommand(),
		sdkauthcli.GetSimulateCmd(),
	)

	// SDK module tx commands
	ac := encodingConfig.InterfaceRegistry.SigningContext().AddressCodec()
	valAc := encodingConfig.InterfaceRegistry.SigningContext().ValidatorAddressCodec()
	cmd.AddCommand(
		sdkbankcli.NewTxCmd(ac),
		sdkdistrcli.NewTxCmd(valAc, ac),
		sdkgovcli.NewTxCmd(nil),
		sdkstakingcli.NewTxCmd(valAc, ac),
		sdkevidencecli.GetTxCmd(nil),
		feegrantcli.GetTxCmd(ac),
		upgradecli.GetTxCmd(ac),
	)

	// Zerone custom module tx commands
	cmd.AddCommand(
		alignmentcli.NewTxCmd(),
		zeroneauthcli.GetTxCmd(),
		autopoiesiscli.NewTxCmd(),
		billingcli.NewTxCmd(),
		bvmcli.NewTxCmd(),
		capturechallengecli.NewTxCmd(),
		capturedefensecli.NewTxCmd(),
		channelscli.NewTxCmd(),
		claimingpotcli.NewTxCmd(),
		computepoolcli.NewTxCmd(),
		discoverycli.NewTxCmd(),
		disputescli.NewTxCmd(),
		emergencycli.NewTxCmd(),
		evidencemgmtcli.NewTxCmd(),
		govcli.NewTxCmd(),
		homecli.NewTxCmd(),
		ibcratelimitcli.NewTxCmd(),
		icaauthcli.NewTxCmd(),
		knowledgecli.GetTxCmd(),
		liquiditypoolcli.NewTxCmd(),
		ontologycli.NewTxCmd(),
		partnershipscli.NewTxCmd(),
		qualificationcli.NewTxCmd(),
		researchcli.NewTxCmd(),
		schedulecli.NewTxCmd(),
		stakingcli.GetTxCmd(),
		tokenscli.NewTxCmd(),
		toolboxcli.NewTxCmd(),
		treecli.NewTxCmd(),
		vestingrewardscli.NewTxCmd(),
	)

	return cmd
}

// newApp creates a new ZeroneApp with the given options.
func newApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	appOpts servertypes.AppOptions,
) servertypes.Application {
	baseappOptions := server.DefaultBaseappOptions(appOpts)

	return app.NewZeroneApp(
		logger, db, traceStore, true, appOpts,
		baseappOptions...,
	)
}

// appExport creates a new ZeroneApp and exports its state.
func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, nil
	}

	zeroneApp := app.NewZeroneApp(
		logger, db, traceStore, height == -1, appOpts,
	)

	if height != -1 {
		if err := zeroneApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	}

	return zeroneApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// emptyAppOptions satisfies servertypes.AppOptions for creating a temporary
// app instance used only to extract module metadata for AutoCLI.
type emptyAppOptions struct{}

func (o emptyAppOptions) Get(_ string) interface{} { return nil }

// SDK bech32 config is initialized in app.init() — no duplicate needed here.
