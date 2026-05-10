package cross_stack_test

import (
	"encoding/json"
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/log"

	abci "github.com/cometbft/cometbft/abci/types"
	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	sdkstakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	zeroneapp "github.com/zerone-chain/zerone/app"
	zeroneauthkeeper "github.com/zerone-chain/zerone/x/auth/keeper"
	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	zeronestakingkeeper "github.com/zerone-chain/zerone/x/staking/keeper"
	zeronestakingtypes "github.com/zerone-chain/zerone/x/staking/types"

	// R7 module keepers
	zeroneapkeeper "github.com/zerone-chain/zerone/x/autopoiesis/keeper"
	zeronealignmentkeeper "github.com/zerone-chain/zerone/x/alignment/keeper"
	zeroneresearchkeeper "github.com/zerone-chain/zerone/x/research/keeper"
	zeronetreekeeper "github.com/zerone-chain/zerone/x/tree/keeper"
	zeroneemkeeper "github.com/zerone-chain/zerone/x/evidence_mgmt/keeper"
	zeronecpotkeeper "github.com/zerone-chain/zerone/x/claiming_pot/keeper"
	zeronesponsorshipkeeper "github.com/zerone-chain/zerone/x/sponsorship/keeper"
	zeroneemergencykeeper "github.com/zerone-chain/zerone/x/emergency/keeper"
	vestingrewardskeeper "github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
	zeronedisputeskeeper "github.com/zerone-chain/zerone/x/disputes/keeper"
	zeroneknowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	zeronegovkeeper "github.com/zerone-chain/zerone/x/gov/keeper"
	zeronecdkeeper "github.com/zerone-chain/zerone/x/capture_defense/keeper"
	zeronecckeeper "github.com/zerone-chain/zerone/x/capture_challenge/keeper"
	zeronequalificationkeeper "github.com/zerone-chain/zerone/x/qualification/keeper"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
	zeroneprovenancekeeper "github.com/zerone-chain/zerone/x/training_provenance/keeper"
	zeronecounterexkeeper "github.com/zerone-chain/zerone/x/counterexamples/keeper"
	zeronecreedkeeper "github.com/zerone-chain/zerone/x/creed/keeper"
	zeroneinquirykeeper "github.com/zerone-chain/zerone/x/inquiry/keeper"
	zeroneagentukeeper "github.com/zerone-chain/zerone/x/agent_understanding/keeper"
	zeronedialectickeeper "github.com/zerone-chain/zerone/x/dialectic/keeper"
	zeronetrustscorekeeper "github.com/zerone-chain/zerone/x/trust_score/keeper"
	zeronegovsynthkeeper "github.com/zerone-chain/zerone/x/governance_synthesis/keeper"
	zeronepartnershipskeeper "github.com/zerone-chain/zerone/x/partnerships/keeper"
	zeronediscoverykeeper "github.com/zerone-chain/zerone/x/discovery/keeper"
)

const testChainID = "zerone-test-1"

// Note: bech32 prefix is set and sealed by the app package init().

// TestHarness provides a full app context for cross-module integration tests.
// All keepers are real (not mocked) and share state through the same app instance.
type TestHarness struct {
	T   *testing.T
	App *zeroneapp.ZeroneApp
	Ctx sdk.Context

	// Zerone custom module keepers
	AuthKeeper    zeroneauthkeeper.Keeper
	StakingKeeper zeronestakingkeeper.Keeper

	// Knowledge keeper
	KnowledgeKeeper zeroneknowledgekeeper.Keeper

	// R7 module keepers
	AutopoiesisKeeper    zeroneapkeeper.Keeper
	AlignmentKeeper      zeronealignmentkeeper.Keeper
	ResearchKeeper       zeroneresearchkeeper.Keeper
	TreeKeeper           zeronetreekeeper.Keeper
	EvidenceMgmtKeeper   zeroneemkeeper.Keeper
	ClaimingPotKeeper    zeronecpotkeeper.Keeper
	SponsorshipKeeper    zeronesponsorshipkeeper.Keeper
	EmergencyKeeper      zeroneemergencykeeper.Keeper
	VestingRewardsKeeper vestingrewardskeeper.Keeper
	DisputesKeeper       zeronedisputeskeeper.Keeper

	// Governance keeper
	GovKeeper zeronegovkeeper.Keeper

	// R28/R29 keepers
	CaptureDefenseKeeper     zeronecdkeeper.Keeper
	CaptureChallengeKeeper   zeronecckeeper.Keeper
	QualificationKeeper      zeronequalificationkeeper.Keeper
	PartnershipsKeeper       zeronepartnershipskeeper.Keeper
	DiscoveryKeeper          zeronediscoverykeeper.Keeper
	TrainingProvenanceKeeper zeroneprovenancekeeper.Keeper
	TrustScoreKeeper         zeronetrustscorekeeper.Keeper
	GovernanceSynthesisKeeper zeronegovsynthkeeper.Keeper
	CounterexamplesKeeper     zeronecounterexkeeper.Keeper
	InquiryKeeper             zeroneinquirykeeper.Keeper
	AgentUnderstandingKeeper  zeroneagentukeeper.Keeper
	DialecticKeeper           zeronedialectickeeper.Keeper
	CreedKeeper               zeronecreedkeeper.Keeper

	// Standard Cosmos SDK keepers
	BankKeeper    bankkeeper.Keeper
	AccountKeeper authkeeper.AccountKeeper

	currentHeight int64
}

// genesisStateWithValSet injects a CometBFT validator into the default genesis
// so that InitChain succeeds (Cosmos SDK requires at least one bonded validator).
func genesisStateWithValSet(
	t *testing.T,
	app *zeroneapp.ZeroneApp,
	genState zeroneapp.GenesisState,
) zeroneapp.GenesisState {
	t.Helper()

	privVal := cmted25519.GenPrivKey()
	pubKey := privVal.PubKey()
	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	// Create genesis account for the validator.
	senderPrivKey := cmted25519.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), nil, 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100_000_000_000))),
	}

	var err error
	genState, err = genesisStateWithValSetHelper(
		app, genState, valSet, []authtypes.GenesisAccount{acc}, balance,
	)
	require.NoError(t, err)
	return genState
}

// genesisStateWithValSetHelper patches the genesis state to include
// the given validator set, accounts, and balances.
func genesisStateWithValSetHelper(
	app *zeroneapp.ZeroneApp,
	genesisState map[string]json.RawMessage,
	valSet *cmttypes.ValidatorSet,
	genAccs []authtypes.GenesisAccount,
	balances ...banktypes.Balance,
) (map[string]json.RawMessage, error) {
	codec := app.AppCodec()

	// set genesis accounts
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccs)
	genesisState[authtypes.ModuleName] = codec.MustMarshalJSON(authGenesis)

	validators := make([]sdkstakingtypes.Validator, 0, len(valSet.Validators))
	delegations := make([]sdkstakingtypes.Delegation, 0, len(valSet.Validators))

	bondAmt := sdk.DefaultPowerReduction

	for _, val := range valSet.Validators {
		pk, err := cryptocodec.FromCmtPubKeyInterface(val.PubKey)
		if err != nil {
			return nil, err
		}
		pkAny, err := codectypes.NewAnyWithValue(pk)
		if err != nil {
			return nil, err
		}
		validator := sdkstakingtypes.Validator{
			OperatorAddress:   sdk.ValAddress(val.Address).String(),
			ConsensusPubkey:   pkAny,
			Jailed:            false,
			Status:            sdkstakingtypes.Bonded,
			Tokens:            bondAmt,
			DelegatorShares:   sdkmath.LegacyOneDec(),
			Description:       sdkstakingtypes.Description{},
			UnbondingHeight:   0,
			Commission:        sdkstakingtypes.NewCommission(sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec()),
			MinSelfDelegation: sdkmath.ZeroInt(),
		}
		validators = append(validators, validator)
		delegations = append(delegations, sdkstakingtypes.NewDelegation(
			genAccs[0].GetAddress().String(),
			sdk.ValAddress(val.Address).String(),
			sdkmath.LegacyOneDec(),
		))
	}

	// set validators and delegations in the SDK staking genesis
	stakingGenesis := sdkstakingtypes.NewGenesisState(sdkstakingtypes.DefaultParams(), validators, delegations)
	genesisState[sdkstakingtypes.ModuleName] = codec.MustMarshalJSON(stakingGenesis)

	totalSupply := sdk.NewCoins()
	for _, b := range balances {
		totalSupply = totalSupply.Add(b.Coins...)
	}
	for range delegations {
		totalSupply = totalSupply.Add(sdk.NewCoin(sdk.DefaultBondDenom, bondAmt))
	}

	// add bonded amount to bonded pool module account
	balances = append(balances, banktypes.Balance{
		Address: authtypes.NewModuleAddress(sdkstakingtypes.BondedPoolName).String(),
		Coins:   sdk.Coins{sdk.NewCoin(sdk.DefaultBondDenom, bondAmt)},
	})

	bankGenesis := banktypes.NewGenesisState(
		banktypes.DefaultGenesisState().Params,
		balances,
		totalSupply,
		[]banktypes.Metadata{},
		[]banktypes.SendEnabled{},
	)
	genesisState[banktypes.ModuleName] = codec.MustMarshalJSON(bankGenesis)

	return genesisState, nil
}

// NewTestHarness creates a fully initialized Zerone app with default genesis
// and a single bonded validator. Uses an in-memory database so each test gets
// a clean, isolated state.
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()

	db := dbm.NewMemDB()
	app := zeroneapp.NewZeroneApp(
		log.NewNopLogger(),
		db,
		nil,  // traceStore
		true, // loadLatest
		simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
		baseapp.SetChainID(testChainID),
	)

	genState := app.DefaultGenesis()
	genState = genesisStateWithValSet(t, app, genState)
	stateBytes, err := json.Marshal(genState)
	require.NoError(t, err)

	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         testChainID,
		AppStateBytes:   stateBytes,
		ConsensusParams: simtestutil.DefaultConsensusParams,
	})
	require.NoError(t, err)

	_, err = app.Commit()
	require.NoError(t, err)

	h := &TestHarness{
		T:             t,
		App:           app,
		AuthKeeper:    app.ZeroneAuthKeeper,
		StakingKeeper: app.ZeroneStakingKeeper,
		BankKeeper:      app.BankKeeper,
		AccountKeeper:   app.AccountKeeper,
		KnowledgeKeeper: app.KnowledgeKeeper,
		GovKeeper:       app.ZeroneGovKeeper,

		// R28/R29 keepers
		CaptureDefenseKeeper:     app.CaptureDefenseKeeper,
		CaptureChallengeKeeper:   app.CaptureChallengeKeeper,
		QualificationKeeper:      app.QualificationKeeper,
		PartnershipsKeeper:       app.PartnershipsKeeper,
		DiscoveryKeeper:          app.DiscoveryKeeper,
		TrainingProvenanceKeeper: app.TrainingProvenanceKeeper,
		TrustScoreKeeper:         app.TrustScoreKeeper,
		GovernanceSynthesisKeeper: app.GovernanceSynthesisKeeper,
		CounterexamplesKeeper:     app.CounterexamplesKeeper,
		InquiryKeeper:             app.InquiryKeeper,
		AgentUnderstandingKeeper:  app.AgentUnderstandingKeeper,
		DialecticKeeper:           app.DialecticKeeper,
		CreedKeeper:               app.CreedKeeper,

		// R7 keepers
		AutopoiesisKeeper:    app.AutopoiesisKeeper,
		AlignmentKeeper:      app.AlignmentKeeper,
		ResearchKeeper:       app.ResearchKeeper,
		TreeKeeper:           app.TreeKeeper,
		EvidenceMgmtKeeper:   app.EvidenceMgmtKeeper,
		ClaimingPotKeeper:    app.ClaimingPotKeeper,
		SponsorshipKeeper:    app.SponsorshipKeeper,
		EmergencyKeeper:      app.EmergencyKeeper,
		VestingRewardsKeeper: app.VestingRewardsKeeper,
		DisputesKeeper:       app.DisputesKeeper,

		currentHeight: 1,
	}

	h.Ctx = app.NewContext(true).
		WithBlockHeight(h.currentHeight).
		WithChainID(testChainID).
		WithBlockHeader(cmtproto.Header{
			Height:  h.currentHeight,
			ChainID: testChainID,
		})

	return h
}

// newTestApp creates a ZeroneApp with SetChainID but does NOT call InitChain.
// Used by genesis tests that need to control InitChain themselves.
func newTestApp(t *testing.T, chainID string) *zeroneapp.ZeroneApp {
	t.Helper()
	db := dbm.NewMemDB()
	return zeroneapp.NewZeroneApp(
		log.NewNopLogger(),
		db,
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
		baseapp.SetChainID(chainID),
	)
}

// initChainWithValSet is a convenience for genesis tests: patches the genesis
// state with a validator set and calls InitChain + Commit.
func initChainWithValSet(t *testing.T, app *zeroneapp.ZeroneApp, chainID string) {
	t.Helper()
	genState := app.DefaultGenesis()
	genState = genesisStateWithValSet(t, app, genState)
	stateBytes, err := json.Marshal(genState)
	require.NoError(t, err)

	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         chainID,
		AppStateBytes:   stateBytes,
		ConsensusParams: simtestutil.DefaultConsensusParams,
	})
	require.NoError(t, err)
	_, err = app.Commit()
	require.NoError(t, err)
}

// FundAccount mints coins and sends them to the given address.
// Uses the zerone_auth module account which has Minter permission.
func (h *TestHarness) FundAccount(addr sdk.AccAddress, amount sdk.Coins) error {
	moduleName := zeroneauthtypes.ModuleName // has Minter permission
	if err := h.App.BankKeeper.MintCoins(h.Ctx, moduleName, amount); err != nil {
		return err
	}
	return h.App.BankKeeper.SendCoinsFromModuleToAccount(h.Ctx, moduleName, addr, amount)
}

// AdvanceBlocks simulates advancing the chain by n blocks.
// Runs module-level BeginBlocker and EndBlocker at each block height.
func (h *TestHarness) AdvanceBlocks(n int) {
	for i := 0; i < n; i++ {
		h.currentHeight++
		h.Ctx = h.App.NewContext(true).
			WithBlockHeight(h.currentHeight).
			WithChainID(testChainID).
			WithBlockHeader(cmtproto.Header{
				Height:  h.currentHeight,
				ChainID: testChainID,
			})

		h.App.BeginBlocker(h.Ctx)
		// Knowledge module BeginBlocker isn't picked up by the test harness's
		// module-manager invocation in some setups (Route B Wave 8 heartbeat
		// depends on it running deterministically). Call it explicitly so
		// lifecycle tests pass in-harness as they would on a live chain.
		_ = h.KnowledgeKeeper.BeginBlocker(h.Ctx)
		// Qualification module BeginBlocker — same harness limitation;
		// Wave 16 accuracy-decay tests depend on it running deterministically.
		_ = h.QualificationKeeper.BeginBlocker(h.Ctx)
		h.App.EndBlocker(h.Ctx)
	}
}

// GetBalance returns the balance of a specific denom for an address.
func (h *TestHarness) GetBalance(addr sdk.AccAddress, denom string) sdk.Coin {
	return h.App.BankKeeper.GetBalance(h.Ctx, addr, denom)
}

// SetDomainQualification grants the given validator an ACTIVE
// qualification in the named domain with the provided weight (1..100,
// mapped to BPS by x/knowledge). Used by tests that exercise the
// per-domain panel weighting (Wave 15c).
func (h *TestHarness) SetDomainQualification(addr, domain string, weight uint32) {
	q := &qualificationtypes.DomainQualification{
		Validator: addr,
		Domain:    domain,
		Status:    qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    weight,
		Pathway:   qualificationtypes.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
	}
	h.QualificationKeeper.SetQualification(h.Ctx, q)
}

// BondTestValidator creates a bonded Scholar-tier validator record for
// the given address and self-delegation amount (in uzrn). Used by tests
// that need stake-weighted voting on the augmentation verifier panel
// (Wave 10+ Sybil fix). Scholar tier uses REAL total stake for effective
// selection weight (apprentice tiers use a virtual-stake constant), so
// this addr has exactly `selfDel` weight on the stake-weighted tally.
func (h *TestHarness) BondTestValidator(addr string, selfDel uint64) {
	selfDelStr := fmt.Sprintf("%d", selfDel)
	val := &zeronestakingtypes.Validator{
		OperatorAddress: addr,
		ConsensusPubkey: "pk_" + addr[:min(8, len(addr))],
		Did:             "did:zrn:test:" + addr[:min(8, len(addr))],
		Moniker:         "val_" + addr[:min(8, len(addr))],
		Tier:            zeronestakingtypes.TierScholar,
		SelfDelegation:  selfDelStr,
		DelegatedStake:  "0",
		TotalStake:      selfDelStr,
		ReputationScore: 500_000,
		IsActive:        true,
	}
	sdkCtx := sdk.UnwrapSDKContext(h.Ctx)
	h.StakingKeeper.SetValidator(sdkCtx, val)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Height returns the current block height.
func (h *TestHarness) Height() int64 {
	return h.currentHeight
}

