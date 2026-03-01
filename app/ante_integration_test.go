package app

import (
	"context"
	"encoding/hex"
	"math"
	"testing"

	"cosmossdk.io/errors"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	protov2 "google.golang.org/protobuf/proto"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	zeroneauthkeeper "github.com/zerone-chain/zerone/x/auth/keeper"
	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	emergencykeeper "github.com/zerone-chain/zerone/x/emergency/keeper"
	emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
)

// ---------- Mock tx types ----------

// mockSignedTx implements authsigning.SigVerifiableTx + sdk.FeeTx + sdk.TxWithMemo
// to allow testing decorators that extract signers from tx signature data.
type mockSignedTx struct {
	msgs  []sdk.Msg
	fee   sdk.Coins
	gas   uint64
	memo  string
	sigV2 []signing.SignatureV2
}

func (m mockSignedTx) GetMsgs() []sdk.Msg                              { return m.msgs }
func (m mockSignedTx) GetMsgsV2() ([]protov2.Message, error)            { return nil, nil }
func (m mockSignedTx) ValidateBasic() error                            { return nil }
func (m mockSignedTx) GetGas() uint64                                  { return m.gas }
func (m mockSignedTx) GetFee() sdk.Coins                               { return m.fee }
func (m mockSignedTx) FeePayer() []byte                                { return nil }
func (m mockSignedTx) FeeGranter() []byte                              { return nil }
func (m mockSignedTx) GetMemo() string                                 { return m.memo }
func (m mockSignedTx) GetSignaturesV2() ([]signing.SignatureV2, error) { return m.sigV2, nil }
func (m mockSignedTx) GetSigners() ([][]byte, error)                   { return nil, nil }
func (m mockSignedTx) GetPubKeys() ([]cryptotypes.PubKey, error) {
	pks := make([]cryptotypes.PubKey, len(m.sigV2))
	for i, sig := range m.sigV2 {
		pks[i] = sig.PubKey
	}
	return pks, nil
}

// Compile-time interface checks
var _ sdk.Tx = mockSignedTx{}
var _ sdk.FeeTx = mockSignedTx{}
var _ sdk.TxWithMemo = mockSignedTx{}
var _ authsigning.SigVerifiableTx = mockSignedTx{}

// newMockSignedTx creates a mockSignedTx with proper signature data from Ed25519 private keys.
func newMockSignedTx(keys []*ed25519.PrivKey, msgs []sdk.Msg, fee sdk.Coins, gas uint64) mockSignedTx {
	sigs := make([]signing.SignatureV2, len(keys))
	for i, key := range keys {
		sigs[i] = signing.SignatureV2{
			PubKey: key.PubKey(),
			Data:   &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT},
		}
	}
	return mockSignedTx{
		msgs:  msgs,
		fee:   fee,
		gas:   gas,
		sigV2: sigs,
	}
}

// newMockSignedTxWithMemo creates a mockSignedTx with a memo field.
func newMockSignedTxWithMemo(keys []*ed25519.PrivKey, msgs []sdk.Msg, fee sdk.Coins, gas uint64, memo string) mockSignedTx {
	tx := newMockSignedTx(keys, msgs, fee, gas)
	tx.memo = memo
	return tx
}

// ---------- Mock keepers for ante tests ----------

type mockCosmosAccountKeeperForAnte struct{}

func (m mockCosmosAccountKeeperForAnte) GetAccount(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}

type mockStakingKeeperForEmergency struct{}

func (m mockStakingKeeperForEmergency) GetValidator(_ context.Context, _ string) (*emergencytypes.ValidatorInfo, bool) {
	return nil, false
}
func (m mockStakingKeeperForEmergency) GetGuardianValidators(_ context.Context) ([]emergencytypes.ValidatorInfo, error) {
	return nil, nil
}

type mockCosmosAccountKeeperForAuth struct{}

func (m mockCosmosAccountKeeperForAuth) GetAccount(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m mockCosmosAccountKeeperForAuth) SetAccount(_ context.Context, _ sdk.AccountI) {}
func (m mockCosmosAccountKeeperForAuth) NewAccountWithAddress(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}

type mockBankKeeperForAnte struct{}

func (m mockBankKeeperForAnte) GetBalance(_ context.Context, _ sdk.AccAddress, _ string) sdk.Coin {
	return sdk.NewCoin("uzrn", sdkmath.ZeroInt())
}
func (m mockBankKeeperForAnte) GetAllBalances(_ context.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins()
}
func (m mockBankKeeperForAnte) SendCoinsFromModuleToAccount(_ context.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

// ---------- Test infrastructure ----------

// setupAuthKeeper creates a real zerone auth keeper with in-memory store.
func setupAuthKeeper(t *testing.T, stateStore store.CommitMultiStore) (zeroneauthkeeper.Keeper, storetypes.StoreKey) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(zeroneauthtypes.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, nil)

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := zeroneauthkeeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		mockCosmosAccountKeeperForAuth{},
		mockBankKeeperForAnte{},
		"authority",
	)
	return k, storeKey
}

// setupEmergencyKeeper creates a real emergency keeper with in-memory store.
func setupEmergencyKeeper(t *testing.T, stateStore store.CommitMultiStore) (emergencykeeper.Keeper, storetypes.StoreKey) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(emergencytypes.StoreKey)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, nil)

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := emergencykeeper.NewKeeper(
		runtime.NewKVStoreService(storeKey),
		cdc,
		"authority",
		mockStakingKeeperForEmergency{},
	)
	return k, storeKey
}

// setupBothKeepers creates both auth and emergency keepers on shared multi-store.
func setupBothKeepers(t *testing.T) (zeroneauthkeeper.Keeper, emergencykeeper.Keeper, sdk.Context) {
	t.Helper()

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())

	authK, _ := setupAuthKeeper(t, stateStore)
	emergencyK, _ := setupEmergencyKeeper(t, stateStore)

	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 500_000, ChainID: "zerone-test-1"}, false, log.NewNopLogger())

	// Set default params for auth keeper
	defaultParams := zeroneauthtypes.DefaultParams()
	if err := authK.SetParams(ctx, &defaultParams); err != nil {
		t.Fatalf("failed to set auth params: %v", err)
	}

	return authK, emergencyK, ctx
}

// registerZeroneAccount creates a Zerone Account entry (optionally frozen).
func registerZeroneAccount(t *testing.T, k zeroneauthkeeper.Keeper, ctx sdk.Context, address string, frozen bool) {
	t.Helper()
	registerZeroneAccountWithType(t, k, ctx, address, "agent", &zeroneauthtypes.AccountFlags{Frozen: frozen})
}

// registerZeroneAccountWithType creates a Zerone Account with specified type and flags.
func registerZeroneAccountWithType(t *testing.T, k zeroneauthkeeper.Keeper, ctx sdk.Context, address, accountType string, flags *zeroneauthtypes.AccountFlags) {
	t.Helper()
	account := &zeroneauthtypes.Account{
		Address:         address,
		Did:             "did:zrn:abcdef0123456789abcdef0123456789",
		AccountType:     accountType,
		Flags:           flags,
		LastActiveBlock: 0,
	}
	k.SetAccount(ctx, account)
}

// ---------- Test 1: Emergency halt blocks MsgSend ----------

func TestAnteIntegration_EmergencyHaltBlocksMsgSend(t *testing.T) {
	_, ek, ctx := setupBothKeepers(t)
	ek.SetEmergencyStatus(ctx, emergencytypes.StatusHalted)

	decorator := NewEmergencyHaltDecorator(ek)

	key := ed25519.GenPrivKey()
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{key},
		[]sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
	)

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Fatal("expected ErrChainHalted, got nil")
	}
	if !emergencytypes.ErrChainHalted.Is(err) {
		t.Fatalf("expected ErrChainHalted, got: %v", err)
	}
}

// ---------- Test 2: Emergency halt allows MsgProposeResume ----------

func TestAnteIntegration_EmergencyHaltAllowsMsgProposeResume(t *testing.T) {
	_, ek, ctx := setupBothKeepers(t)
	ek.SetEmergencyStatus(ctx, emergencytypes.StatusHalted)

	decorator := NewEmergencyHaltDecorator(ek)

	key := ed25519.GenPrivKey()
	addr := sdk.AccAddress(key.PubKey().Address())
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{key},
		[]sdk.Msg{&emergencytypes.MsgProposeResume{Proposer: addr.String()}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
	)

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err != nil {
		t.Fatalf("emergency message should pass during halt, got: %v", err)
	}
}

// ---------- Test 3: Frozen account MsgSend rejected ----------

func TestAnteIntegration_FrozenAccountRejected(t *testing.T) {
	ak, _, ctx := setupBothKeepers(t)

	key := ed25519.GenPrivKey()
	addr := sdk.AccAddress(key.PubKey().Address())

	registerZeroneAccount(t, ak, ctx, addr.String(), true)

	decorator := NewZeroneAccountDecorator(ak)
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{key},
		[]sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
	)

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Fatal("expected ErrAccountFrozen, got nil")
	}
	if !zeroneauthtypes.ErrAccountFrozen.Is(err) {
		t.Fatalf("expected ErrAccountFrozen, got: %v", err)
	}
}

// ---------- Test 4: Unfreeze + retry succeeds ----------

func TestAnteIntegration_UnfreezeAndRetrySucceeds(t *testing.T) {
	ak, _, ctx := setupBothKeepers(t)

	key := ed25519.GenPrivKey()
	addr := sdk.AccAddress(key.PubKey().Address())

	// Register as frozen
	registerZeroneAccount(t, ak, ctx, addr.String(), true)

	decorator := NewZeroneAccountDecorator(ak)
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{key},
		[]sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
	)

	// First attempt: should fail
	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil || !zeroneauthtypes.ErrAccountFrozen.Is(err) {
		t.Fatalf("frozen account should be rejected, got: %v", err)
	}

	// Unfreeze
	account, found := ak.GetAccount(ctx, addr.String())
	if !found {
		t.Fatal("account should exist")
	}
	account.Flags.Frozen = false
	ak.SetAccount(ctx, account)

	// Retry: should succeed
	_, err = decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err != nil {
		t.Fatalf("unfrozen account should pass, got: %v", err)
	}
}

// ---------- Test 5: Session key CanTransfer → MsgVote rejected ----------

func TestAnteIntegration_SessionKeyTransferOnlyRejectsVote(t *testing.T) {
	ak, _, ctx := setupBothKeepers(t)

	// Create session key — the decorator looks up sessions by the signer's
	// derived address (sdk.AccAddress from pubkey), so Owner must match that.
	sessionKey := ed25519.GenPrivKey()
	sessionAddr := sdk.AccAddress(sessionKey.PubKey().Address())
	pubKeyHex := hex.EncodeToString(sessionKey.PubKey().Bytes())

	// Register session key with CanTransfer only, stored under session key's address
	ak.SetSessionKey(ctx, &zeroneauthtypes.SessionKey{
		Owner:          sessionAddr.String(),
		KeyHash:        "session1",
		PublicKey:      pubKeyHex,
		ExpiresAtBlock: 1_000_000,
		Capabilities: &zeroneauthtypes.SessionCapabilities{
			CanTransfer: true,
		},
	})

	decorator := NewZeroneCapabilityDecorator(ak, mockCosmosAccountKeeperForAnte{})

	// Sign with session key, try MsgVote → should be rejected
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{sessionKey},
		[]sdk.Msg{mockMsg{typeURL: "/cosmos.gov.v1.MsgVote"}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
	)

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Fatal("expected ErrSessionCapabilityDenied, got nil")
	}
	if !zeroneauthtypes.ErrSessionCapabilityDenied.Is(err) {
		t.Fatalf("expected ErrSessionCapabilityDenied, got: %v", err)
	}
}

// ---------- Test 6: Session key CanTransfer → MsgSend accepted ----------

func TestAnteIntegration_SessionKeyTransferAllowsMsgSend(t *testing.T) {
	ak, _, ctx := setupBothKeepers(t)

	sessionKey := ed25519.GenPrivKey()
	sessionAddr := sdk.AccAddress(sessionKey.PubKey().Address())
	pubKeyHex := hex.EncodeToString(sessionKey.PubKey().Bytes())

	ak.SetSessionKey(ctx, &zeroneauthtypes.SessionKey{
		Owner:          sessionAddr.String(),
		KeyHash:        "session1",
		PublicKey:      pubKeyHex,
		ExpiresAtBlock: 1_000_000,
		Capabilities: &zeroneauthtypes.SessionCapabilities{
			CanTransfer: true,
		},
	})

	decorator := NewZeroneCapabilityDecorator(ak, mockCosmosAccountKeeperForAnte{})

	tx := newMockSignedTx(
		[]*ed25519.PrivKey{sessionKey},
		[]sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
	)

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err != nil {
		t.Fatalf("session key with CanTransfer should allow MsgSend, got: %v", err)
	}
}

// ---------- Test 7: Bootstrap gas-free MsgSubmitClaim at height 1 ----------

func TestAnteIntegration_BootstrapGasFreeAtHeight1(t *testing.T) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1}, false, log.NewNopLogger())
	decorator := NewBootstrapGasFreeDecorator()

	key := ed25519.GenPrivKey()
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{key},
		[]sdk.Msg{mockMsg{typeURL: "/zerone.knowledge.v1.MsgSubmitClaim"}},
		sdk.Coins{},
		0,
	)

	// The decorator should set gas meter to BlockGasLimit so bootstrap txs
	// can consume gas freely. We use BlockGasLimit (not infinite) because
	// CometBFT's mempool rejects txs with gas_wanted > ConsensusParams.Block.MaxGas.
	var receivedCtx sdk.Context
	handler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		receivedCtx = ctx
		return ctx, nil
	}

	_, err := decorator.AnteHandle(ctx, tx, false, handler)
	if err != nil {
		t.Fatalf("bootstrap gas-free should pass at height 1, got: %v", err)
	}

	// Check for BlockGasLimit gas meter
	if receivedCtx.GasMeter().Limit() != BlockGasLimit {
		t.Errorf("expected BlockGasLimit gas meter (limit=%d), got limit=%d",
			BlockGasLimit, receivedCtx.GasMeter().Limit())
	}
}

// ---------- Test 8: Bootstrap gas-free expires after BootstrapEndBlock ----------

func TestAnteIntegration_BootstrapGasFreeExpiresAfterEndBlock(t *testing.T) {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	// Set a finite gas meter to verify the decorator does NOT replace it
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: BootstrapEndBlock + 1}, false, log.NewNopLogger())
	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(1_000_000))

	decorator := NewBootstrapGasFreeDecorator()

	key := ed25519.GenPrivKey()
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{key},
		[]sdk.Msg{mockMsg{typeURL: "/zerone.knowledge.v1.MsgSubmitClaim"}},
		sdk.Coins{},
		0,
	)

	var receivedCtx sdk.Context
	handler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		receivedCtx = ctx
		return ctx, nil
	}

	_, err := decorator.AnteHandle(ctx, tx, false, handler)
	if err != nil {
		t.Fatalf("decorator should pass through after bootstrap end, got: %v", err)
	}

	// Gas meter should be finite — the decorator should NOT have replaced it
	if receivedCtx.GasMeter().Limit() == math.MaxUint64 {
		t.Error("gas meter should NOT be infinite after BootstrapEndBlock")
	}
	if receivedCtx.GasMeter().Limit() != 1_000_000 {
		t.Errorf("gas meter limit should remain 1,000,000, got %d", receivedCtx.GasMeter().Limit())
	}
}

// ---------- Test 9: Gas overflow with many messages ----------

func TestAnteIntegration_GasOverflowWithManyMessages(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}

	// Create 1000 messages each requiring 200,000 gas (deploy_contract)
	// 1000 * 200,000 = 200,000,000 which exceeds TxGasLimit (11,111,111)
	msgs := make([]sdk.Msg, 1000)
	for i := range msgs {
		msgs[i] = mockMsg{typeURL: "/zerone.bvm.v1.MsgDeployContract"}
	}

	tx := mockFeeTx{
		gas:  TxGasLimit,
		fee:  sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(999_999_999))),
		msgs: msgs,
	}

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Fatal("expected gas overflow error, got nil")
	}
	if !sdkerrors.ErrOutOfGas.Is(err) {
		t.Fatalf("expected ErrOutOfGas, got: %v", err)
	}
}

// ---------- Test 10: Fee in wrong denom rejected ----------

func TestAnteIntegration_WrongFeeDenomRejected(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}

	tx := mockFeeTx{
		gas:  100_000,
		fee:  sdk.NewCoins(sdk.NewCoin("uatom", sdkmath.NewInt(100_000))),
		msgs: []sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
	}

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Fatal("expected ErrInsufficientFee for wrong denom, got nil")
	}
	if !sdkerrors.ErrInsufficientFee.Is(err) {
		t.Fatalf("expected ErrInsufficientFee, got: %v", err)
	}
}

// ---------- Test 11: ZRN gas table coverage ----------

func TestAnteIntegration_GasTableCoverage(t *testing.T) {
	for name, gas := range TransactionGasCosts {
		if gas == 0 && !IsSystemTransaction(name) {
			t.Errorf("TransactionGasCosts[%q] = 0 but is not a system transaction", name)
		}
		if gas > TxGasLimit {
			t.Errorf("TransactionGasCosts[%q] = %d exceeds TxGasLimit %d", name, gas, TxGasLimit)
		}
	}

	// All msgTypeURLToGas entries must reference non-zero costs
	for url, gas := range msgTypeURLToGas {
		if gas == 0 {
			t.Errorf("msgTypeURLToGas[%q] = 0, all mapped messages should have non-zero gas", url)
		}
		if gas > TxGasLimit {
			t.Errorf("msgTypeURLToGas[%q] = %d exceeds TxGasLimit %d", url, gas, TxGasLimit)
		}
	}
}

// ---------- Test 12: DID resolution from memo ----------

func TestAnteIntegration_DIDResolutionFromMemo(t *testing.T) {
	ak, _, ctx := setupBothKeepers(t)

	key := ed25519.GenPrivKey()
	addr := sdk.AccAddress(key.PubKey().Address())
	pubKeyHex := hex.EncodeToString(key.PubKey().Bytes())

	// Create a DID from the first 32 hex chars of the pubkey
	did := "did:zrn:" + pubKeyHex[:32]

	// Register the DID mapping
	ak.SetDIDMapping(ctx, &zeroneauthtypes.DIDMapping{
		Did:    did,
		Bech32: addr.String(),
	})

	decorator := NewZeroneDIDDecorator(ak)

	tx := newMockSignedTxWithMemo(
		[]*ed25519.PrivKey{key},
		[]sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
		did,
	)

	// Use a handler that captures the context to check for events
	var receivedCtx sdk.Context
	handler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		receivedCtx = ctx
		return ctx, nil
	}

	// Need an event manager on context
	ctx = ctx.WithEventManager(sdk.NewEventManager())

	_, err := decorator.AnteHandle(ctx, tx, false, handler)
	if err != nil {
		t.Fatalf("DID resolution should succeed, got: %v", err)
	}

	// Check that did_reference event was emitted
	events := receivedCtx.EventManager().Events()
	found := false
	for _, event := range events {
		if event.Type == "did_reference" {
			found = true
			for _, attr := range event.Attributes {
				if attr.Key == "did" && attr.Value == did {
					t.Logf("DID event emitted: did=%s", did)
				}
				if attr.Key == "address" && attr.Value == addr.String() {
					t.Logf("DID event emitted: address=%s", addr.String())
				}
			}
		}
	}
	if !found {
		t.Error("expected did_reference event to be emitted")
	}
}

// ---------- Account-Level Capability Enforcement Tests ----------

// capabilityTestCase defines a single capability enforcement test.
type capabilityTestCase struct {
	name        string
	accountType string
	flags       *zeroneauthtypes.AccountFlags
	register    bool // false = unregistered account
	msgType     string
	wantErr     bool
	errType     *errors.Error
}

func TestAnteIntegration_AccountCapabilityEnforcement(t *testing.T) {
	tests := []capabilityTestCase{
		// Contract restrictions
		{
			name:        "ContractBlockedFromClaims",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{CanSubmitClaims: false, CanChallenge: false},
			register:    true,
			msgType:     "/zerone.knowledge.v1.MsgSubmitClaim",
			wantErr:     true,
			errType:     zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		{
			name:        "ContractBlockedFromChallenge",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{CanSubmitClaims: false, CanChallenge: false},
			register:    true,
			msgType:     "/zerone.knowledge.v1.MsgChallengeFact",
			wantErr:     true,
			errType:     zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		{
			name:        "ContractBlockedFromStaking",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{},
			register:    true,
			msgType:     "/cosmos.staking.v1beta1.MsgDelegate",
			wantErr:     true,
			errType:     zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		{
			name:        "ContractBlockedFromVoting",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{},
			register:    true,
			msgType:     "/cosmos.gov.v1.MsgVote",
			wantErr:     true,
			errType:     zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		{
			name:        "ContractBlockedFromResearch",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{},
			register:    true,
			msgType:     "/zerone.research.v1.MsgSubmitResearch",
			wantErr:     true,
			errType:     zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		{
			name:        "ContractAllowsPartnership",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{},
			register:    true,
			msgType:     "/zerone.partnerships.v1.MsgInitiatePartnership",
			wantErr:     false,
		},
		{
			name:        "ContractAllowsTransfer",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{},
			register:    true,
			msgType:     "/cosmos.bank.v1beta1.MsgSend",
			wantErr:     false,
		},
		// Human allows all (with flags enabled)
		{
			name:        "HumanAllowsAll",
			accountType: "human",
			flags:       &zeroneauthtypes.AccountFlags{CanSubmitClaims: true, CanChallenge: true},
			register:    true,
			msgType:     "/zerone.knowledge.v1.MsgSubmitClaim",
			wantErr:     false,
		},
		// Agent allows all (with flags enabled)
		{
			name:        "AgentAllowsAll",
			accountType: "agent",
			flags:       &zeroneauthtypes.AccountFlags{CanSubmitClaims: true, CanChallenge: true},
			register:    true,
			msgType:     "/zerone.knowledge.v1.MsgChallengeFact",
			wantErr:     false,
		},
		// Unregistered account restrictions
		{
			name:     "UnregisteredBlockedFromClaims",
			register: false,
			msgType:  "/zerone.knowledge.v1.MsgSubmitClaim",
			wantErr:  true,
			errType:  zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		{
			name:     "UnregisteredAllowsTransfer",
			register: false,
			msgType:  "/cosmos.bank.v1beta1.MsgSend",
			wantErr:  false,
		},
		{
			name:     "UnregisteredAllowsStaking",
			register: false,
			msgType:  "/cosmos.staking.v1beta1.MsgDelegate",
			wantErr:  false,
		},
		{
			name:     "UnregisteredAllowsRegistration",
			register: false,
			msgType:  "/zerone.auth.v1.MsgRegisterAccount",
			wantErr:  false,
		},
		{
			name:     "UnregisteredBlockedFromPartnership",
			register: false,
			msgType:  "/zerone.partnerships.v1.MsgInitiatePartnership",
			wantErr:  true,
			errType:  zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		// Challenge split from claims
		{
			name:        "ChallengeSplitFromClaims",
			accountType: "contract",
			flags:       &zeroneauthtypes.AccountFlags{CanSubmitClaims: true, CanChallenge: false},
			register:    true,
			msgType:     "/zerone.knowledge.v1.MsgChallengeFact",
			wantErr:     true,
			errType:     zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		// Nil flags blocks claims
		{
			name:        "NilFlagsBlocksClaims",
			accountType: "human",
			flags:       nil,
			register:    true,
			msgType:     "/zerone.knowledge.v1.MsgSubmitClaim",
			wantErr:     true,
			errType:     zeroneauthtypes.ErrAccountCapabilityDenied,
		},
		// Primary key default-allow for unknown msg types
		{
			name:        "PrimaryKeyDefaultAllow",
			accountType: "human",
			flags:       &zeroneauthtypes.AccountFlags{CanSubmitClaims: true, CanChallenge: true},
			register:    true,
			msgType:     "/cosmos.unknown.v1.MsgDoSomething",
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ak, _, ctx := setupBothKeepers(t)

			key := ed25519.GenPrivKey()
			addr := sdk.AccAddress(key.PubKey().Address())

			if tc.register {
				registerZeroneAccountWithType(t, ak, ctx, addr.String(), tc.accountType, tc.flags)
			}

			decorator := NewZeroneCapabilityDecorator(ak, mockCosmosAccountKeeperForAnte{})
			tx := newMockSignedTx(
				[]*ed25519.PrivKey{key},
				[]sdk.Msg{mockMsg{typeURL: tc.msgType}},
				sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
				100_000,
			)

			_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errType != nil && !tc.errType.Is(err) {
					t.Fatalf("expected %v, got: %v", tc.errType, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// Test 18: Session key still uses default-deny for unknown msg types
func TestAnteIntegration_SessionKeyStillDefaultDeny(t *testing.T) {
	ak, _, ctx := setupBothKeepers(t)

	sessionKey := ed25519.GenPrivKey()
	sessionAddr := sdk.AccAddress(sessionKey.PubKey().Address())
	pubKeyHex := hex.EncodeToString(sessionKey.PubKey().Bytes())

	ak.SetSessionKey(ctx, &zeroneauthtypes.SessionKey{
		Owner:          sessionAddr.String(),
		KeyHash:        "session-default-deny",
		PublicKey:      pubKeyHex,
		ExpiresAtBlock: 1_000_000,
		Capabilities: &zeroneauthtypes.SessionCapabilities{
			CanTransfer: true,
		},
	})

	decorator := NewZeroneCapabilityDecorator(ak, mockCosmosAccountKeeperForAnte{})

	// Unknown msg type should be denied for session keys
	tx := newMockSignedTx(
		[]*ed25519.PrivKey{sessionKey},
		[]sdk.Msg{mockMsg{typeURL: "/cosmos.unknown.v1.MsgDoSomething"}},
		sdk.NewCoins(sdk.NewCoin(BondDenom, sdkmath.NewInt(100_000))),
		100_000,
	)

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Fatal("expected ErrSessionCapabilityDenied for unknown msg type with session key, got nil")
	}
	if !zeroneauthtypes.ErrSessionCapabilityDenied.Is(err) {
		t.Fatalf("expected ErrSessionCapabilityDenied, got: %v", err)
	}
}
