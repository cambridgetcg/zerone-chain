// Package app implements the Zerone Cosmos SDK application.
//
// Zerone is a blockchain for AI agent economies using Proof of Truth (PoT)
// consensus, where verifying knowledge IS the useful work.
//
// This file registers all standard Cosmos SDK modules. Custom Zerone modules
// are added incrementally by batch (see REWRITE-PLAN.md).
package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"
	"github.com/gorilla/mux"
	"github.com/spf13/cast"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/tx/signing"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdkruntime "github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	// IBC modules
	capabilitykeeper "github.com/cosmos/ibc-go/modules/capability/keeper"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	ibctransfer "github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v8/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v8/modules/core"
	ibcporttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v8/modules/core/keeper"

	// ICA (Interchain Accounts)
	ica "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts"
	icacontroller "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"

	// IBC Fee Middleware (ICS-29)
	ibcfee "github.com/cosmos/ibc-go/v8/modules/apps/29-fee"
	ibcfeekeeper "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/keeper"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"

	// CometBFT
	abci "github.com/cometbft/cometbft/abci/types"

	// Crypto codec for keyring support
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"

	// Zerone custom modules
	zeroneauth "github.com/zerone-chain/zerone/x/auth"
	zeroneauthkeeper "github.com/zerone-chain/zerone/x/auth/keeper"
	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	zeroneknowledge "github.com/zerone-chain/zerone/x/knowledge"
	zeroneknowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	zeroneknowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	zeroneontology "github.com/zerone-chain/zerone/x/ontology"
	zeroneontologykeeper "github.com/zerone-chain/zerone/x/ontology/keeper"
	zeroneontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
	zeronestaking "github.com/zerone-chain/zerone/x/staking"
	zeronestakingkeeper "github.com/zerone-chain/zerone/x/staking/keeper"
	zeronestakingtypes "github.com/zerone-chain/zerone/x/staking/types"
	zeronebilling "github.com/zerone-chain/zerone/x/billing"
	zeronebillingkeeper "github.com/zerone-chain/zerone/x/billing/keeper"
	zeronebillingtypes "github.com/zerone-chain/zerone/x/billing/types"
	zeronetokens "github.com/zerone-chain/zerone/x/tokens"
	zeronetokenskeeper "github.com/zerone-chain/zerone/x/tokens/keeper"
	zeronetokenstypes "github.com/zerone-chain/zerone/x/tokens/types"
	vestingrewards "github.com/zerone-chain/zerone/x/vesting_rewards"
	vestingrewardskeeper "github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
	vestingrewardstypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"

	// Tx types (cosmos.tx.v1beta1.Tx registration)
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"

	// gRPC services
	cmtservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
)

const (
	// AppName is the application name.
	AppName = "zeroned"

	// AccountAddressPrefix is the bech32 prefix for Zerone addresses.
	AccountAddressPrefix = "zrn"

	// BondDenom is the staking denomination.
	BondDenom = "uzrn"

	// DisplayDenom is the human-readable denomination.
	DisplayDenom = "zrn"

	// DefaultBlockTime is the target block time in milliseconds.
	DefaultBlockTime = 2521
)

var (
	// DefaultNodeHome is the default home directory for the node.
	DefaultNodeHome string

	// ModuleBasics defines the module BasicManager used for codec registration
	// and genesis verification.
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
		bank.AppModuleBasic{},
		staking.AppModuleBasic{},
		distr.AppModuleBasic{},
		gov.NewAppModuleBasic(nil),
		slashing.AppModuleBasic{},
		feegrantmodule.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		vesting.AppModuleBasic{},
		consensus.AppModuleBasic{},
		ibc.AppModuleBasic{},
		ibctransfer.AppModuleBasic{},
		ibcfee.AppModuleBasic{},
		ica.AppModuleBasic{},
		// ===== Zerone custom modules — added by batch =====
		zeroneauth.AppModuleBasic{},
		zeronestaking.AppModuleBasic{},
		vestingrewards.AppModuleBasic{},
		zeroneontology.AppModuleBasic{},
		zeroneknowledge.AppModuleBasic{},
		zeronetokens.AppModuleBasic{},
		zeronebilling.AppModuleBasic{},
		// R2-2: x/knowledge wired
		// R3-1: x/billing — wired
		// R3-2: x/liquiditypool
		// R3-4: x/gov (zeronegov.AppModuleBasic{})
		// R3-6: x/tokens — wired
		// R4-1: x/home
		// R4-2: x/partnerships
		// R4-3: x/bvm
		// R4-4: x/channels
		// R4-5: x/schedule, x/compute_pool, x/discovery
		// R5-1: x/toolbox
		// R6-1: x/emergency
		// R6-2: x/evidence_mgmt
		// R6-3: x/disputes
		// R6-4: x/capture_challenge, x/capture_defense
		// R6-5: x/qualification
		// R6-6: x/ibcratelimit, x/icaauth
		// R7-1: x/autopoiesis
		// R7-2: x/alignment
		// R7-3: x/research
		// R7-4: x/tree
	)

	// Module account permissions.
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		ibcfeetypes.ModuleName:         nil,
		icatypes.ModuleName:            nil,
		// ===== Zerone custom module accounts — added by batch =====
		zeroneauthtypes.ModuleName:    {authtypes.Minter}, // Minter for bootstrap fund
		zeronestakingtypes.ModuleName: {authtypes.Burner, authtypes.Staking},
		vestingrewardstypes.ModuleName:        {authtypes.Minter, authtypes.Burner}, // Minter for block rewards, Burner for burn split
		vestingrewardstypes.ResearchFundModuleName: nil,                              // research_fund: receive-only
		zeroneontologytypes.ModuleName:             nil,                              // ontology: receive proposal stake
		zeroneknowledgetypes.ModuleName:            {authtypes.Burner},               // knowledge: burn slashed claim stakes
		zeronetokenstypes.ModuleName:               {authtypes.Minter, authtypes.Burner}, // tokens: mint/burn for wrap/unwrap + emissions
		zeronebillingtypes.ModuleName:              {authtypes.Burner},                   // billing: burn split
		"treasury_protocol":                        nil,                                  // treasury_protocol: receive-only
	}
)

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	DefaultNodeHome = filepath.Join(userHomeDir, ".zeroned")

	// Set bech32 prefixes for Zerone addresses.
	sdkConfig := sdk.GetConfig()
	sdkConfig.SetBech32PrefixForAccount(AccountAddressPrefix, AccountAddressPrefix+"pub")
	sdkConfig.SetBech32PrefixForValidator(AccountAddressPrefix+"valoper", AccountAddressPrefix+"valoperpub")
	sdkConfig.SetBech32PrefixForConsensusNode(AccountAddressPrefix+"valcons", AccountAddressPrefix+"valconspub")
	sdkConfig.Seal()

	// Set the default bond denom to uzrn (micro-ZRN).
	sdk.DefaultBondDenom = BondDenom
}

// EncodingConfig specifies the concrete encoding types to use for a given app.
type EncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

// MakeEncodingConfig creates the EncodingConfig for the Zerone application.
func MakeEncodingConfig() EncodingConfig {
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          addresscodec.NewBech32Codec(AccountAddressPrefix),
			ValidatorAddressCodec: addresscodec.NewBech32Codec(AccountAddressPrefix + "valoper"),
		},
	})
	if err != nil {
		panic(err)
	}

	appCodec := codec.NewProtoCodec(interfaceRegistry)
	legacyAmino := codec.NewLegacyAmino()
	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	sdk.RegisterLegacyAminoCodec(legacyAmino)
	sdk.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(legacyAmino)
	txtypes.RegisterInterfaces(interfaceRegistry)

	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             appCodec,
		TxConfig:          txConfig,
		Amino:             legacyAmino,
	}
}

// GenesisState is the top-level genesis state: module name → raw genesis bytes.
type GenesisState map[string]json.RawMessage

// ZeroneApp extends baseapp.BaseApp with all Cosmos SDK modules.
type ZeroneApp struct {
	*baseapp.BaseApp

	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry

	// Store keys
	keys    map[string]*storetypes.KVStoreKey
	tkeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	// ---------- Standard Cosmos SDK Keepers ----------
	AccountKeeper   authkeeper.AccountKeeper
	BankKeeper      bankkeeper.Keeper
	StakingKeeper   *stakingkeeper.Keeper
	SlashingKeeper  slashingkeeper.Keeper
	DistrKeeper     distrkeeper.Keeper
	GovKeeper       *govkeeper.Keeper
	UpgradeKeeper   *upgradekeeper.Keeper
	EvidenceKeeper  evidencekeeper.Keeper
	FeeGrantKeeper  feegrantkeeper.Keeper
	ConsensusKeeper consensuskeeper.Keeper

	// ---------- IBC Keepers ----------
	CapabilityKeeper          *capabilitykeeper.Keeper
	ScopedIBCKeeper           capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper      capabilitykeeper.ScopedKeeper
	ScopedICAControllerKeeper capabilitykeeper.ScopedKeeper
	ScopedICAHostKeeper       capabilitykeeper.ScopedKeeper
	IBCKeeper                 *ibckeeper.Keeper
	IBCFeeKeeper              ibcfeekeeper.Keeper
	TransferKeeper            ibctransferkeeper.Keeper
	ICAControllerKeeper       icacontrollerkeeper.Keeper
	ICAHostKeeper             icahostkeeper.Keeper

	// ===== Zerone custom module keepers — added by batch =====
	ZeroneAuthKeeper        zeroneauthkeeper.Keeper
	ZeroneStakingKeeper     zeronestakingkeeper.Keeper
	VestingRewardsKeeper    vestingrewardskeeper.Keeper
	ZeroneOntologyKeeper    zeroneontologykeeper.Keeper
	KnowledgeKeeper         zeroneknowledgekeeper.Keeper
	TokensKeeper            zeronetokenskeeper.Keeper
	BillingKeeper           zeronebillingkeeper.Keeper
	// R3-2: LiquidityPoolKeeper
	// ZeroneGovKeeper (custom)
	// R3-6: x/tokens — wired
	// R4-1: HomeKeeper
	// R4-2: PartnershipsKeeper
	// R4-3: BVMKeeper
	// R4-4: ChannelsKeeper
	// R4-5: ScheduleKeeper, ComputePoolKeeper, DiscoveryKeeper
	// R5-1: ToolboxKeeper
	// R6-1: EmergencyKeeper
	// R6-2: EvidenceMgmtKeeper
	// R6-3: DisputesKeeper
	// R6-4: CaptureChallengeKeeper, CaptureDefenseKeeper
	// R6-5: QualificationKeeper
	// R6-6: IBCRateLimitKeeper, ICAAuthKeeper
	// R7-1: AutopoiesisKeeper
	// R7-2: AlignmentKeeper
	// R7-3: ResearchKeeper
	// R7-4: TreeKeeper

	// ABCI++ vote extension config (nil until validator is configured)
	VoteExtConfig *VoteExtensionConfig

	// Module manager
	ModuleManager *module.Manager

	// Simulation manager (for fuzz testing)
	sm *module.SimulationManager

	// Configurator for module msg/query registration
	configurator module.Configurator
}

// NewZeroneApp creates and initializes a new ZeroneApp instance.
func NewZeroneApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *ZeroneApp {
	interfaceRegistry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          addresscodec.NewBech32Codec(AccountAddressPrefix),
			ValidatorAddressCodec: addresscodec.NewBech32Codec(AccountAddressPrefix + "valoper"),
		},
	})
	if err != nil {
		panic(err)
	}
	appCodec := codec.NewProtoCodec(interfaceRegistry)
	legacyAmino := codec.NewLegacyAmino()
	txConfig := authtx.NewTxConfig(appCodec, authtx.DefaultSignModes)

	sdk.RegisterLegacyAminoCodec(legacyAmino)
	sdk.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterInterfaces(interfaceRegistry)
	ModuleBasics.RegisterLegacyAminoCodec(legacyAmino)
	txtypes.RegisterInterfaces(interfaceRegistry)

	bApp := baseapp.NewBaseApp(AppName, logger, db, txConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)

	// ---- Store Keys ----
	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		distrtypes.StoreKey,
		slashingtypes.StoreKey,
		govtypes.StoreKey,
		upgradetypes.StoreKey,
		feegrant.StoreKey,
		evidencetypes.StoreKey,
		capabilitytypes.StoreKey,
		ibcexported.StoreKey,
		ibctransfertypes.StoreKey,
		ibcfeetypes.StoreKey,
		icacontrollertypes.StoreKey,
		icahosttypes.StoreKey,
		"consensus", // x/consensus module store key
		// ===== Zerone custom module store keys — added by batch =====
		zeroneauthtypes.StoreKey,
		zeronestakingtypes.StoreKey,
		vestingrewardstypes.StoreKey,
		zeroneontologytypes.StoreKey,
		zeroneknowledgetypes.StoreKey,
		zeronetokenstypes.StoreKey,
		zeronebillingtypes.StoreKey,
	)
	tkeys := storetypes.NewTransientStoreKeys(paramstypes.TStoreKey)
	memKeys := storetypes.NewMemoryStoreKeys(capabilitytypes.MemStoreKey)

	app := &ZeroneApp{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		txConfig:          txConfig,
		interfaceRegistry: interfaceRegistry,
		keys:              keys,
		tkeys:             tkeys,
		memKeys:           memKeys,
	}

	// ---- Module Keepers ----

	app.ConsensusKeeper = consensuskeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys["consensus"]),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		sdkruntime.EventService{},
	)
	bApp.SetParamStore(app.ConsensusKeeper.ParamsStore)

	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPerms,
		addresscodec.NewBech32Codec(AccountAddressPrefix),
		AccountAddressPrefix,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[banktypes.StoreKey]),
		app.AccountKeeper,
		blockedModuleAccountAddrs(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		logger,
	)

	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		addresscodec.NewBech32Codec(AccountAddressPrefix+"valoper"),
		addresscodec.NewBech32Codec(AccountAddressPrefix+"valcons"),
	)

	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[distrtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		legacyAmino,
		sdkruntime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		app.StakingKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[feegrant.StoreKey]),
		app.AccountKeeper,
	)

	app.UpgradeKeeper = upgradekeeper.NewKeeper(
		skipUpgradeHeights(appOpts),
		sdkruntime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		DefaultNodeHome,
		app.BaseApp,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.EvidenceKeeper = *evidencekeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		app.StakingKeeper,
		app.SlashingKeeper,
		app.AccountKeeper.AddressCodec(),
		sdkruntime.ProvideCometInfoService(),
	)

	// ---- Staking Hooks ----
	// Wire slashing and distribution as hooks on staking so that validator
	// signing info is created when validators are added during genesis.
	app.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(app.DistrKeeper.Hooks(), app.SlashingKeeper.Hooks()),
	)

	// ---- Governance Keeper ----
	govConfig := govtypes.DefaultConfig()
	app.GovKeeper = govkeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[govtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		app.DistrKeeper,
		app.MsgServiceRouter(),
		govConfig,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	// ---- Capability Keeper (required by IBC) ----
	app.CapabilityKeeper = capabilitykeeper.NewKeeper(
		appCodec,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)
	app.ScopedIBCKeeper = app.CapabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	app.ScopedTransferKeeper = app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	app.ScopedICAControllerKeeper = app.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
	app.ScopedICAHostKeeper = app.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	// Seal after all ScopeToModule calls — prevents capability escalation at runtime.
	app.CapabilityKeeper.Seal()

	// ---- IBC Keepers ----
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec,
		keys[ibcexported.StoreKey],
		paramstypes.Subspace{}, // x/params removed in v0.47+; IBC accepts empty subspace
		app.StakingKeeper,
		app.UpgradeKeeper,
		app.ScopedIBCKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.IBCFeeKeeper = ibcfeekeeper.NewKeeper(
		appCodec,
		keys[ibcfeetypes.StoreKey],
		app.IBCKeeper.ChannelKeeper, // ics4Wrapper
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.PortKeeper,
		app.AccountKeeper,
		app.BankKeeper,
	)

	app.TransferKeeper = ibctransferkeeper.NewKeeper(
		appCodec,
		keys[ibctransfertypes.StoreKey],
		paramstypes.Subspace{},
		app.IBCFeeKeeper,             // ics4Wrapper routes through fee middleware
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.PortKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		app.ScopedTransferKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(
		appCodec,
		keys[icacontrollertypes.StoreKey],
		paramstypes.Subspace{},
		app.IBCKeeper.ChannelKeeper, // ics4Wrapper
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.PortKeeper,
		app.ScopedICAControllerKeeper,
		app.MsgServiceRouter(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		appCodec,
		keys[icahosttypes.StoreKey],
		paramstypes.Subspace{},
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.PortKeeper,
		app.AccountKeeper,
		app.ScopedICAHostKeeper,
		app.MsgServiceRouter(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)
	app.ICAHostKeeper.WithQueryRouter(app.GRPCQueryRouter())

	// ===== Zerone custom module keeper init (added by batch) =====

	app.ZeroneAuthKeeper = zeroneauthkeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[zeroneauthtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.ZeroneStakingKeeper = zeronestakingkeeper.NewKeeper(
		appCodec,
		keys[zeronestakingtypes.StoreKey],
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.VestingRewardsKeeper = vestingrewardskeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[vestingrewardstypes.StoreKey]),
		app.BankKeeper,
		nil, // staking keeper set after x/staking wiring
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.ZeroneOntologyKeeper = zeroneontologykeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[zeroneontologytypes.StoreKey]),
		app.BankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	stakingAdapter := zeronestakingkeeper.NewStakingKeeperAdapter(app.ZeroneStakingKeeper)
	app.KnowledgeKeeper = zeroneknowledgekeeper.NewKeeper(
		sdkruntime.NewKVStoreService(keys[zeroneknowledgetypes.StoreKey]),
		appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		app.BankKeeper,
		stakingAdapter,
	)
	app.KnowledgeKeeper.SetOntologyKeeper(&app.ZeroneOntologyKeeper)
	app.KnowledgeKeeper.SetVestingRewardsKeeper(vestingrewardskeeper.NewVestingRewardsKeeperAdapter(app.VestingRewardsKeeper))

	app.TokensKeeper = zeronetokenskeeper.NewKeeper(
		appCodec,
		sdkruntime.NewKVStoreService(keys[zeronetokenstypes.StoreKey]),
		app.BankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	billingKnowledgeAdapter := zeroneknowledgekeeper.NewBillingKnowledgeAdapter(app.KnowledgeKeeper)
	vestingRFDAdapter := vestingrewardskeeper.NewResearchFundDepositorAdapter(app.VestingRewardsKeeper)
	app.BillingKeeper = zeronebillingkeeper.NewKeeper(
		sdkruntime.NewKVStoreService(keys[zeronebillingtypes.StoreKey]),
		appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		app.BankKeeper,
		billingKnowledgeAdapter,
		vestingRFDAdapter,
	)

	// ---- IBC Router ----
	ibcRouter := ibcporttypes.NewRouter()
	ibcRouter.AddRoute(ibctransfertypes.ModuleName, ibctransfer.NewIBCModule(app.TransferKeeper))
	ibcRouter.AddRoute(
		icacontrollertypes.SubModuleName,
		icacontroller.NewIBCMiddleware(nil, app.ICAControllerKeeper),
	)
	ibcRouter.AddRoute(icahosttypes.SubModuleName, icahost.NewIBCModule(app.ICAHostKeeper))
	app.IBCKeeper.SetRouter(ibcRouter)

	// ---- Module Manager ----
	app.ModuleManager = module.NewManager(
		genutil.NewAppModule(app.AccountKeeper, app.StakingKeeper, app, txConfig),
		auth.NewAppModule(appCodec, app.AccountKeeper, nil, nil),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, nil),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, nil),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil),
		gov.NewAppModule(appCodec, app.GovKeeper, app.AccountKeeper, app.BankKeeper, nil),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil, appCodec.InterfaceRegistry()),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, appCodec.InterfaceRegistry()),
		upgrade.NewAppModule(app.UpgradeKeeper, addresscodec.NewBech32Codec(AccountAddressPrefix)),
		evidence.NewAppModule(app.EvidenceKeeper),
		consensus.NewAppModule(appCodec, app.ConsensusKeeper),
		ibc.NewAppModule(app.IBCKeeper),
		ibctransfer.NewAppModule(app.TransferKeeper),
		ibcfee.NewAppModule(app.IBCFeeKeeper),
		ica.NewAppModule(&app.ICAControllerKeeper, &app.ICAHostKeeper),
		// ===== Zerone custom modules — added by batch =====
		zeroneauth.NewAppModule(appCodec, app.ZeroneAuthKeeper),
		zeronestaking.NewAppModule(app.ZeroneStakingKeeper),
		vestingrewards.NewAppModule(appCodec, app.VestingRewardsKeeper),
		zeroneontology.NewAppModule(appCodec, app.ZeroneOntologyKeeper),
		zeroneknowledge.NewAppModule(appCodec, app.KnowledgeKeeper),
		zeronetokens.NewAppModule(appCodec, app.TokensKeeper),
		zeronebilling.NewAppModule(appCodec, app.BillingKeeper),
	)

	app.ModuleManager.SetOrderBeginBlockers(
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		govtypes.ModuleName,
		genutiltypes.ModuleName,
		vestingtypes.ModuleName,
		feegrant.ModuleName,
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		ibcfeetypes.ModuleName,
		icatypes.ModuleName,
		// ===== Zerone custom module BeginBlocker order — added by batch =====
		vestingrewardstypes.ModuleName, // MUST run before x/distribution to intercept fees
		zeroneauthtypes.ModuleName,
		zeronestakingtypes.ModuleName,
		zeroneontologytypes.ModuleName,
		zeroneknowledgetypes.ModuleName, // LAST: depends on staking + ontology state
		zeronetokenstypes.ModuleName,    // tokens: emission period processing
		zeronebillingtypes.ModuleName,   // billing: no-op
	)

	app.ModuleManager.SetOrderEndBlockers(
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		ibcfeetypes.ModuleName,
		icatypes.ModuleName,
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		feegrant.ModuleName,
		genutiltypes.ModuleName,
		vestingtypes.ModuleName,
		// ===== Zerone custom module EndBlocker order — added by batch =====
		zeroneauthtypes.ModuleName,
		zeronestakingtypes.ModuleName,
		vestingrewardstypes.ModuleName,
		zeroneontologytypes.ModuleName,  // EndBlocker: expire proposals
		zeroneknowledgetypes.ModuleName, // EndBlocker: no-op for now
		zeronetokenstypes.ModuleName,    // EndBlocker: no-op
		zeronebillingtypes.ModuleName,   // EndBlocker: no-op
	)

	genesisOrder := []string{
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		feegrant.ModuleName,
		evidencetypes.ModuleName,
		ibcexported.ModuleName,
		genutiltypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibcfeetypes.ModuleName,
		icatypes.ModuleName,
		vestingtypes.ModuleName,
		upgradetypes.ModuleName,
		// ===== Zerone custom module genesis order — added by batch =====
		zeroneauthtypes.ModuleName,
		zeronestakingtypes.ModuleName,
		vestingrewardstypes.ModuleName,
		zeroneontologytypes.ModuleName,  // Genesis: after bank (needs bank for stake escrow)
		zeroneknowledgetypes.ModuleName, // Genesis: after ontology + staking (needs both)
		zeronetokenstypes.ModuleName,    // Genesis: after bank (needs bank for wrap)
		zeronebillingtypes.ModuleName,   // Genesis: after knowledge (depends on knowledge for fact queries)
	}
	app.ModuleManager.SetOrderInitGenesis(genesisOrder...)
	app.ModuleManager.SetOrderExportGenesis(genesisOrder...)

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	if err := app.ModuleManager.RegisterServices(app.configurator); err != nil {
		panic(fmt.Sprintf("failed to register module services: %s", err))
	}

	// Register upgrade handlers (must be after RegisterServices, before LoadLatestVersion).
	app.RegisterUpgradeHandlers()

	// Mount stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	app.MountMemoryStores(memKeys)

	// Set ante handler
	app.SetAnteHandler(NewAnteHandler(app))

	// Set block handlers
	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PotPreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)

	// ABCI++ handlers for Proof of Truth vote extensions
	app.SetPrepareProposal(app.PrepareProposalHandler())
	app.SetProcessProposal(app.ProcessProposalHandler())
	app.SetExtendVoteHandler(app.ExtendVoteHandler())
	app.SetVerifyVoteExtensionHandler(app.VerifyVoteExtensionHandler())

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			logger.Error("error loading latest version", "err", err)
			os.Exit(1)
		}
	}

	return app
}

// InitChainer initializes the chain from genesis.
func (app *ZeroneApp) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	// Ensure ZRN denomination metadata is present in the bank genesis state.
	// This registers the denom units (uzrn / mzrn / zrn) with the bank module
	// so queries like /cosmos/bank/v1beta1/denoms_metadata return useful data.
	if raw, ok := genesisState[banktypes.ModuleName]; ok {
		var bankGenState banktypes.GenesisState
		app.appCodec.MustUnmarshalJSON(raw, &bankGenState)
		if !hasZRNMetadata(bankGenState.DenomMetadata) {
			bankGenState.DenomMetadata = append(bankGenState.DenomMetadata, zrnDenomMetadata())
			genesisState[banktypes.ModuleName] = app.appCodec.MustMarshalJSON(&bankGenState)
		}
	}

	app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap())
	return app.ModuleManager.InitGenesis(ctx, app.appCodec, genesisState)
}

// zrnDenomMetadata returns the canonical ZRN token denomination metadata.
func zrnDenomMetadata() banktypes.Metadata {
	return banktypes.Metadata{
		Description: "The native staking and governance token of Zerone",
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: "uzrn", Exponent: 0, Aliases: []string{"microzrn"}},
			{Denom: "mzrn", Exponent: 3, Aliases: []string{"millizrn"}},
			{Denom: "zrn", Exponent: 6, Aliases: nil},
		},
		Base:    "uzrn",
		Display: "zrn",
		Name:    "Zerone",
		Symbol:  "ZRN",
	}
}

// hasZRNMetadata checks if ZRN denom metadata is already present.
func hasZRNMetadata(metadata []banktypes.Metadata) bool {
	for _, m := range metadata {
		if m.Base == "uzrn" {
			return true
		}
	}
	return false
}

// BeginBlocker runs begin-block logic for all modules.
func (app *ZeroneApp) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	return app.ModuleManager.BeginBlock(ctx)
}

// EndBlocker runs end-block logic for all modules.
func (app *ZeroneApp) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.ModuleManager.EndBlock(ctx)
}

// LoadHeight loads a specific application state height.
func (app *ZeroneApp) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// AppCodec returns the protobuf codec.
func (app *ZeroneApp) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns the interface registry.
func (app *ZeroneApp) InterfaceRegistry() codectypes.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns the transaction config.
func (app *ZeroneApp) TxConfig() client.TxConfig {
	return app.txConfig
}

// LegacyAmino returns the legacy amino codec.
func (app *ZeroneApp) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// DefaultGenesis returns the default genesis state for all modules.
func (app *ZeroneApp) DefaultGenesis() GenesisState {
	return ModuleBasics.DefaultGenesis(app.appCodec)
}

// SimulationManager returns the simulation manager.
func (app *ZeroneApp) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers REST API routes.
func (app *ZeroneApp) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	authtypes.RegisterInterfaces(clientCtx.InterfaceRegistry)
	ModuleBasics.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Swagger UI placeholder — full OpenAPI served from proto-generated spec (R10-2)
	if apiConfig.Swagger {
		RegisterSwaggerAPI(apiSvr.Router)
	}
}

// RegisterSwaggerAPI registers a Swagger UI route with the API router.
func RegisterSwaggerAPI(rtr *mux.Router) {
	// Placeholder: swagger spec will be generated from proto files in R10-2.
	_ = rtr
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *ZeroneApp) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *ZeroneApp) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(
		clientCtx,
		app.BaseApp.GRPCQueryRouter(),
		app.interfaceRegistry,
		app.Query,
	)
}

// RegisterNodeService implements the Application.RegisterNodeService method.
func (app *ZeroneApp) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// blockedModuleAccountAddrs returns the set of module account addresses that
// are blocked from receiving funds (all module accounts except governance).
func blockedModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}
	// Allow governance module to receive funds (for proposal deposits).
	delete(modAccAddrs, authtypes.NewModuleAddress(govtypes.ModuleName).String())
	return modAccAddrs
}

// skipUpgradeHeights reads skip-upgrade-heights from app options.
func skipUpgradeHeights(appOpts servertypes.AppOptions) map[int64]bool {
	skipHeights := map[int64]bool{}
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipHeights[int64(h)] = true
	}
	return skipHeights
}

// Ensure ZeroneApp implements the servertypes.Application interface at compile time.
var _ servertypes.Application = (*ZeroneApp)(nil)

// Suppress unused-import warnings for types that will be used by custom modules.
var (
	_ = govv1beta1.RegisterInterfaces
)
