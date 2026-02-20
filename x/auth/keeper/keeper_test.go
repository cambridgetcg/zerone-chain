package keeper_test

import (
	"context"
	"testing"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/auth/keeper"
	"github.com/zerone-chain/zerone/x/auth/types"
)

// ---------- Mock Keepers ----------

type mockCosmosAccountKeeper struct{}

func (m mockCosmosAccountKeeper) GetAccount(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m mockCosmosAccountKeeper) SetAccount(_ context.Context, _ sdk.AccountI) {}
func (m mockCosmosAccountKeeper) NewAccountWithAddress(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}

type mockBankKeeper struct{}

func (m mockBankKeeper) GetBalance(_ context.Context, _ sdk.AccAddress, _ string) sdk.Coin {
	return sdk.NewCoin("uzrn", sdkmath.ZeroInt())
}
func (m mockBankKeeper) GetAllBalances(_ context.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins()
}
func (m mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

// ---------- Test Setup ----------

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	err := stateStore.LoadLatestVersion()
	if err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		mockCosmosAccountKeeper{},
		mockBankKeeper{},
		"authority",
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	// Set default params
	defaultParams := types.DefaultParams()
	if err := k.SetParams(ctx, &defaultParams); err != nil {
		t.Fatalf("failed to set params: %v", err)
	}

	return k, ctx
}

const (
	testAddr1 = "zrn1m037n75vk2jhdr56y2ptzjjj02uljwnqwwzr7z"
	testAddr2 = "zrn1ur4eyeuuhrkfpcyhykfjsasftv9hn33smszt58"
	// DIDs must derive from their corresponding public keys (first 32 hex chars)
	testPubKey1 = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	testDID1    = "did:zrn:abcdef0123456789abcdef0123456789" // pubKey1[:32]
	testPubKey2 = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	testDID2    = "did:zrn:1234567890abcdef1234567890abcdef" // pubKey2[:32]
)

// ---------- RegisterAccount Tests ----------

func TestRegisterAccount_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	resp, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:             testAddr1,
		Did:                testDID1,
		PublicKey:          testPubKey1,
		AccountType:        "agent",
		OperationalKeyHash: "opkey1hash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// Verify account stored
	account, found := k.GetAccount(ctx, testAddr1)
	if !found {
		t.Fatal("account not found after registration")
	}
	if account.Did != testDID1 {
		t.Errorf("expected DID %s, got %s", testDID1, account.Did)
	}
	if account.PublicKey != testPubKey1 {
		t.Errorf("expected pubkey %s, got %s", testPubKey1, account.PublicKey)
	}
	if account.AccountType != "agent" {
		t.Errorf("expected type agent, got %s", account.AccountType)
	}
	if account.OperationalKeyHash != "opkey1hash" {
		t.Errorf("expected opkey hash opkey1hash, got %s", account.OperationalKeyHash)
	}
	if account.OperationalKeyVersion != 1 {
		t.Errorf("expected version 1, got %d", account.OperationalKeyVersion)
	}
	if account.ReputationScore != 500000 {
		t.Errorf("expected reputation 500000, got %d", account.ReputationScore)
	}
	if !account.Flags.CanSubmitClaims {
		t.Error("expected CanSubmitClaims true")
	}

	// Verify DID mapping stored
	mapping, found := k.GetDIDMapping(ctx, testDID1)
	if !found {
		t.Fatal("DID mapping not found after registration")
	}
	if mapping.Bech32 != testAddr1 {
		t.Errorf("expected bech32 %s, got %s", testAddr1, mapping.Bech32)
	}
	if mapping.PubKey != testPubKey1 {
		t.Errorf("expected pubkey %s, got %s", testPubKey1, mapping.PubKey)
	}
}

func TestRegisterAccount_DuplicateAddress(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	if err != nil {
		t.Fatalf("unexpected error on first registration: %v", err)
	}

	_, err = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID2,
		PublicKey:   testPubKey2,
		AccountType: "human",
	})
	if err == nil {
		t.Fatal("expected error for duplicate address")
	}
}

func TestRegisterAccount_DuplicateDID(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	if err != nil {
		t.Fatalf("unexpected error on first registration: %v", err)
	}

	_, err = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr2,
		Did:         testDID1,
		PublicKey:   testPubKey2,
		AccountType: "human",
	})
	if err == nil {
		t.Fatal("expected error for duplicate DID")
	}
}

// ---------- RotateKey Tests ----------

func TestRotateKey_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:             testAddr1,
		Did:                testDID1,
		PublicKey:          testPubKey1,
		AccountType:        "agent",
		OperationalKeyHash: "oldkey",
	})
	if err != nil {
		t.Fatalf("failed to register account: %v", err)
	}

	_, err = ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("newkey"),
		AuthorizationSignature: []byte("sig"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	expectedHex := "6e65776b6579" // hex("newkey")
	if account.OperationalPublicKey != expectedHex {
		t.Errorf("expected OperationalPublicKey %s, got %s", expectedHex, account.OperationalPublicKey)
	}
	if account.OperationalKeyVersion != 2 {
		t.Errorf("expected version 2, got %d", account.OperationalKeyVersion)
	}
}

func TestRotateKey_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("newkey"),
		AuthorizationSignature: []byte("sig"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}
}

func TestRotateKey_Cooldown(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("key2"),
		AuthorizationSignature: []byte("sig"),
	})
	if err != nil {
		t.Fatalf("first rotation failed: %v", err)
	}

	// Immediate second rotation should fail (cooldown)
	_, err = ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("key3"),
		AuthorizationSignature: []byte("sig"),
	})
	if err == nil {
		t.Fatal("expected cooldown error")
	}

	// After cooldown passes
	params := k.GetParams(ctx)
	newCtx := ctx.WithBlockHeight(int64(100 + params.KeyRotationCooldown + 1))
	_, err = ms.RotateKey(newCtx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("key3"),
		AuthorizationSignature: []byte("sig"),
	})
	if err != nil {
		t.Fatalf("rotation after cooldown should succeed: %v", err)
	}
}

func TestRotateKey_FrozenAccount(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	account, _ := k.GetAccount(ctx, testAddr1)
	account.Flags.Frozen = true
	k.SetAccount(ctx, account)

	_, err := ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("newkey"),
		AuthorizationSignature: []byte("sig"),
	})
	if err == nil {
		t.Fatal("expected error for frozen account")
	}
}

func TestRotateKey_InvalidKeyType(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      nil,
		AuthorizationSignature: []byte("sig"),
	})
	if err == nil {
		t.Fatal("expected error for empty operational key")
	}
}

// ---------- CreateSession Tests ----------

func TestCreateSession_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	resp, err := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:         testAddr1,
		SessionPubKey: []byte("session1"),
		Capabilities: &types.SessionCapabilities{
			CanTransfer:     true,
			CanSubmitClaims: true,
		},
		ExpiresAtHeight: 1100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SessionId == "" {
		t.Error("expected non-empty SessionId")
	}

	keyHash := resp.SessionId
	session, found := k.GetSessionKey(ctx, testAddr1, keyHash)
	if !found {
		t.Fatal("session not found")
	}
	if session.Capabilities == nil || !session.Capabilities.CanTransfer {
		t.Error("expected CanTransfer true")
	}
	if session.Capabilities == nil || !session.Capabilities.CanSubmitClaims {
		t.Error("expected CanSubmitClaims true")
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.SessionKeyCount != 1 {
		t.Errorf("expected session count 1, got %d", account.SessionKeyCount)
	}
}

func TestCreateSession_MaxSessionKeys(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	params := k.GetParams(ctx)

	for i := uint32(0); i < params.MaxSessionKeys; i++ {
		_, err := ms.CreateSession(ctx, &types.MsgCreateSession{
			Owner:           testAddr1,
			SessionPubKey:   []byte(string(rune('a' + int(i)))),
			Capabilities:    &types.SessionCapabilities{},
			ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
		})
		if err != nil {
			t.Fatalf("unexpected error creating session %d: %v", i, err)
		}
	}

	_, err := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("overflow"),
		Capabilities:    &types.SessionCapabilities{},
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})
	if err == nil {
		t.Fatal("expected error for exceeding max session keys")
	}
}

func TestCreateSession_DurationExceeded(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	params := k.GetParams(ctx)

	_, err := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("session1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + params.MaxSessionDuration + 1,
	})
	if err == nil {
		t.Fatal("expected error for exceeding max duration")
	}
}

// ---------- RevokeSession Tests ----------

func TestRevokeSession_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	createResp, _ := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("session1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})

	_, err := ms.RevokeSession(ctx, &types.MsgRevokeSession{
		Owner:     testAddr1,
		SessionId: createResp.SessionId,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, found := k.GetSessionKey(ctx, testAddr1, createResp.SessionId)
	if found {
		t.Fatal("session should be deleted after revocation")
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.SessionKeyCount != 0 {
		t.Errorf("expected session count 0, got %d", account.SessionKeyCount)
	}
}

func TestRevokeSession_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.RevokeSession(ctx, &types.MsgRevokeSession{
		Owner:     testAddr1,
		SessionId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

// ---------- Query Tests ----------

func TestQueryAccount(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	resp, err := qs.Account(ctx, &types.QueryAccountRequest{Address: testAddr1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Account.Did != testDID1 {
		t.Errorf("expected DID %s, got %s", testDID1, resp.Account.Did)
	}
}

func TestQueryAccountByDID(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "human",
	})

	resp, err := qs.AccountByDID(ctx, &types.QueryAccountByDIDRequest{Did: testDID1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Account.Address != testAddr1 {
		t.Errorf("expected address %s, got %s", testAddr1, resp.Account.Address)
	}
	if resp.Account.AccountType != "human" {
		t.Errorf("expected type human, got %s", resp.Account.AccountType)
	}
}

func TestQueryAccount_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Account(ctx, &types.QueryAccountRequest{Address: testAddr1})
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}
}

func TestQuerySessionKeys(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})
	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s2"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 2000,
	})

	resp, err := qs.SessionKeys(ctx, &types.QuerySessionKeysRequest{Owner: testAddr1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.SessionKeys) != 2 {
		t.Errorf("expected 2 session keys, got %d", len(resp.SessionKeys))
	}
}

func TestQueryParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Params.MaxSessionKeys != 5 {
		t.Errorf("expected max session keys 5, got %d", resp.Params.MaxSessionKeys)
	}
	if resp.Params.MaxSessionDuration != 34272 {
		t.Errorf("expected max session duration 34272, got %d", resp.Params.MaxSessionDuration)
	}
	if resp.Params.KeyRotationCooldown != 111 {
		t.Errorf("expected key rotation cooldown 111, got %d", resp.Params.KeyRotationCooldown)
	}
}

func TestQueryFrozenAccounts(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr2,
		Did:         testDID2,
		PublicKey:   testPubKey2,
		AccountType: "human",
	})

	// Freeze first account
	_, _ = ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "test freeze",
	})

	resp, err := qs.FrozenAccounts(ctx, &types.QueryFrozenAccountsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Accounts) != 1 {
		t.Errorf("expected 1 frozen account, got %d", len(resp.Accounts))
	}
	if resp.Accounts[0].Address != testAddr1 {
		t.Errorf("expected frozen account %s, got %s", testAddr1, resp.Accounts[0].Address)
	}
}

// ---------- DID Lookup Tests ----------

func TestGetAccountByDID(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	account, found := k.GetAccountByDID(ctx, testDID1)
	if !found {
		t.Fatal("expected to find account by DID")
	}
	if account.Address != testAddr1 {
		t.Errorf("expected address %s, got %s", testAddr1, account.Address)
	}

	addr, found := k.GetAddressForDID(ctx, testDID1)
	if !found {
		t.Fatal("expected to find address for DID")
	}
	if addr != testAddr1 {
		t.Errorf("expected %s, got %s", testAddr1, addr)
	}
}

// ---------- Genesis Tests ----------

func TestInitExportGenesis(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr2,
		Did:         testDID2,
		PublicKey:   testPubKey2,
		AccountType: "human",
	})

	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})

	gs := k.ExportGenesis(ctx)
	if len(gs.Accounts) != 2 {
		t.Errorf("expected 2 accounts in genesis, got %d", len(gs.Accounts))
	}
	if len(gs.DidMappings) != 2 {
		t.Errorf("expected 2 DID mappings in genesis, got %d", len(gs.DidMappings))
	}
	if len(gs.SessionKeys) != 1 {
		t.Errorf("expected 1 session key in genesis, got %d", len(gs.SessionKeys))
	}

	if err := gs.Validate(); err != nil {
		t.Fatalf("genesis validation failed: %v", err)
	}
}

// ---------- ValidateBasic Tests ----------

func TestMsgRegisterAccount_ValidateBasic(t *testing.T) {
	msg := types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.AccountType = "invalid"
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for invalid account type")
	}

	msg.AccountType = "agent"
	msg.PublicKey = ""
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty public key")
	}

	msg.PublicKey = "short"
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for short public key")
	}

	msg.PublicKey = testPubKey1
	msg.Did = "invalid"
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for invalid DID")
	}
}

func TestValidateDID(t *testing.T) {
	if err := types.ValidateDID(testDID1); err != nil {
		t.Errorf("expected valid DID: %v", err)
	}

	if err := types.ValidateDID("0000000000000000000000000000000000000000000000000000000000000001"); err == nil {
		t.Error("expected error for missing did:zrn: prefix")
	}

	if err := types.ValidateDID("did:zrn:short"); err == nil {
		t.Error("expected error for short suffix")
	}
}

// ---------- Phase 2: OperationalPublicKey + Ed25519 Sync Tests ----------

func TestRegisterAccount_SetsOperationalPublicKey(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:             testAddr1,
		Did:                testDID1,
		PublicKey:          testPubKey1,
		AccountType:        "agent",
		OperationalKeyHash: "opkey1hash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, found := k.GetAccount(ctx, testAddr1)
	if !found {
		t.Fatal("account not found")
	}
	if account.OperationalPublicKey != testPubKey1 {
		t.Errorf("expected OperationalPublicKey %s, got %s", testPubKey1, account.OperationalPublicKey)
	}
}

func TestRotateKey_StoresNewPublicKey(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:             testAddr1,
		Did:                testDID1,
		PublicKey:          testPubKey1,
		AccountType:        "agent",
		OperationalKeyHash: "oldkey",
	})

	_, err := ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("newkey"),
		AuthorizationSignature: []byte("sig"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	expectedHex := "6e65776b6579" // hex("newkey")
	if account.OperationalPublicKey != expectedHex {
		t.Errorf("expected OperationalPublicKey %s, got %s", expectedHex, account.OperationalPublicKey)
	}
	if account.OperationalKeyVersion != 2 {
		t.Errorf("expected version 2, got %d", account.OperationalKeyVersion)
	}
}

func TestRotateKey_WithoutNewPublicKey_PreservesExisting(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:             testAddr1,
		Did:                testDID1,
		PublicKey:          testPubKey1,
		AccountType:        "agent",
		OperationalKeyHash: "oldkey",
	})

	_, err := ms.RotateKey(ctx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      nil,
		AuthorizationSignature: []byte("sig"),
	})
	if err == nil {
		t.Fatal("expected error when NewOperationalKey is nil")
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.OperationalPublicKey != testPubKey1 {
		t.Errorf("expected OperationalPublicKey preserved as %s, got %s", testPubKey1, account.OperationalPublicKey)
	}
}

// ---------- Phase 2-3: Session Key PublicKey Tests ----------

func TestCreateSession_StoresSessionPublicKey(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	sessionPubKeyBytes := []byte{0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	resp, err := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:         testAddr1,
		SessionPubKey: sessionPubKeyBytes,
		Capabilities: &types.SessionCapabilities{
			CanTransfer:     true,
			CanSubmitClaims: true,
		},
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	session, found := k.GetSessionKey(ctx, testAddr1, resp.SessionId)
	if !found {
		t.Fatal("session not found")
	}
	expectedHex := "fedcba9876543210"
	if session.PublicKey != expectedHex {
		t.Errorf("expected session PublicKey %s, got %s", expectedHex, session.PublicKey)
	}
}

func TestGetSessionKeysForOwner_ReturnsAll(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	for i := 0; i < 3; i++ {
		pubKey := []byte{byte('a' + i), 0x01, 0x02, 0x03}
		_, err := ms.CreateSession(ctx, &types.MsgCreateSession{
			Owner:           testAddr1,
			SessionPubKey:   pubKey,
			Capabilities:    &types.SessionCapabilities{CanTransfer: i%2 == 0},
			ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
		})
		if err != nil {
			t.Fatalf("session %d: %v", i, err)
		}
	}

	sessions := k.GetSessionKeysForOwner(ctx, testAddr1)
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestGetSessionKeysForOwner_EmptyForUnknownOwner(t *testing.T) {
	k, ctx := setupKeeper(t)
	sessions := k.GetSessionKeysForOwner(ctx, testAddr2)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for unknown owner, got %d", len(sessions))
	}
}

// ---------- Phase 3: Frozen Account Blocking Tests ----------

func TestCreateSession_FrozenAccount(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	account, _ := k.GetAccount(ctx, testAddr1)
	account.Flags.Frozen = true
	k.SetAccount(ctx, account)

	_, err := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("session1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})
	if err == nil {
		t.Fatal("expected error for frozen account")
	}
}

// ---------- Phase 4: DID Resolution Tests ----------

func TestGetAddressForDID_NotRegistered(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, found := k.GetAddressForDID(ctx, testDID1)
	if found {
		t.Fatal("expected DID not found for unregistered DID")
	}
}

func TestGetAccountByDID_ReturnsFullAccount(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:             testAddr1,
		Did:                testDID1,
		PublicKey:          testPubKey1,
		AccountType:        "agent",
		OperationalKeyHash: "opkey1hash",
	})

	account, found := k.GetAccountByDID(ctx, testDID1)
	if !found {
		t.Fatal("expected to find account by DID")
	}
	if account.Address != testAddr1 {
		t.Errorf("expected address %s, got %s", testAddr1, account.Address)
	}
	if account.OperationalPublicKey != testPubKey1 {
		t.Errorf("expected OperationalPublicKey %s, got %s", testPubKey1, account.OperationalPublicKey)
	}
	if account.OperationalKeyHash != "opkey1hash" {
		t.Errorf("expected opkey hash opkey1hash, got %s", account.OperationalKeyHash)
	}
}

// ---------- ValidateBasic Phase 2-4 Tests ----------

func TestMsgRotateKey_ValidateBasic(t *testing.T) {
	msg := types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("newhash"),
		AuthorizationSignature: []byte("sig"),
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.Sender = "invalid"
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for invalid sender")
	}
}

func TestMsgCreateSession_ValidateBasic(t *testing.T) {
	msg := types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: 1100,
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.Owner = "bad"
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for invalid owner")
	}

	msg.Owner = testAddr1
	msg.SessionPubKey = nil
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty session pub key")
	}
}

func TestMsgRevokeSession_ValidateBasic(t *testing.T) {
	msg := types.MsgRevokeSession{
		Owner:     testAddr1,
		SessionId: "s1",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.SessionId = ""
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty session id")
	}
}

// ---------- LastActiveBlock Update Tests ----------

func TestRegisterAccount_SetsCreatedAndLastActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.CreatedAtBlock != 100 {
		t.Errorf("expected CreatedAtBlock 100, got %d", account.CreatedAtBlock)
	}
	if account.LastActiveBlock != 100 {
		t.Errorf("expected LastActiveBlock 100, got %d", account.LastActiveBlock)
	}
}

func TestRotateKey_UpdatesLastActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	params := k.GetParams(ctx)
	advancedCtx := ctx.WithBlockHeight(int64(100 + params.KeyRotationCooldown + 1))

	_, err := ms.RotateKey(advancedCtx, &types.MsgRotateKey{
		Sender:                 testAddr1,
		NewOperationalKey:      []byte("newkey"),
		AuthorizationSignature: []byte("sig"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(advancedCtx, testAddr1)
	expected := uint64(100 + params.KeyRotationCooldown + 1)
	if account.LastActiveBlock != expected {
		t.Errorf("expected LastActiveBlock %d, got %d", expected, account.LastActiveBlock)
	}
}

func TestCreateSession_UpdatesLastActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	advancedCtx := ctx.WithBlockHeight(200)

	_, err := ms.CreateSession(advancedCtx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(advancedCtx.BlockHeight()) + 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(advancedCtx, testAddr1)
	if account.LastActiveBlock != 200 {
		t.Errorf("expected LastActiveBlock 200, got %d", account.LastActiveBlock)
	}
}

func TestRevokeSession_UpdatesLastActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	createResp, _ := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})

	advancedCtx := ctx.WithBlockHeight(250)
	_, err := ms.RevokeSession(advancedCtx, &types.MsgRevokeSession{
		Owner:     testAddr1,
		SessionId: createResp.SessionId,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(advancedCtx, testAddr1)
	if account.LastActiveBlock != 250 {
		t.Errorf("expected LastActiveBlock 250, got %d", account.LastActiveBlock)
	}
}

// ---------- FreezeAccount Tests ----------

func TestFreezeAccount_SelfFreeze(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "compromised key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if !account.Flags.Frozen {
		t.Fatal("expected account to be frozen")
	}
}

func TestFreezeAccount_AuthorityFreeze(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  "authority",
		Address: testAddr1,
		Reason:  "malicious activity",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if !account.Flags.Frozen {
		t.Fatal("expected account to be frozen by authority")
	}
}

func TestFreezeAccount_UnauthorizedThirdParty(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr2,
		Address: testAddr1,
		Reason:  "attack",
	})
	if err == nil {
		t.Fatal("expected error for unauthorized freeze")
	}
}

func TestFreezeAccount_AlreadyFrozen(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, _ = ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "first freeze",
	})

	_, err := ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "second freeze",
	})
	if err == nil {
		t.Fatal("expected error for already frozen account")
	}
}

func TestFreezeAccount_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "test",
	})
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}
}

func TestFreezeAccount_StoresReason(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "suspected breach",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.Flags.FreezeReason != "suspected breach" {
		t.Errorf("expected freeze reason 'suspected breach', got '%s'", account.Flags.FreezeReason)
	}
}

// ---------- UnfreezeAccount Tests ----------

func TestUnfreezeAccount_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	_, _ = ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "test",
	})

	_, err := ms.UnfreezeAccount(ctx, &types.MsgUnfreezeAccount{
		Authority: "authority",
		Address:   testAddr1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.Flags.Frozen {
		t.Fatal("expected account to be unfrozen")
	}
}

func TestUnfreezeAccount_NonAuthority(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	_, _ = ms.FreezeAccount(ctx, &types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "test",
	})

	_, err := ms.UnfreezeAccount(ctx, &types.MsgUnfreezeAccount{
		Authority: testAddr2,
		Address:   testAddr1,
	})
	if err == nil {
		t.Fatal("expected error for non-authority unfreeze")
	}
}

func TestUnfreezeAccount_NotFrozen(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.UnfreezeAccount(ctx, &types.MsgUnfreezeAccount{
		Authority: "authority",
		Address:   testAddr1,
	})
	if err == nil {
		t.Fatal("expected error for unfreezing non-frozen account")
	}
}

// ---------- SetRecoveryConfig Tests ----------

func TestSetRecoveryConfig_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   2,
		TotalShards: 3,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
			{Type: "agent", Identifier: "zrn1guardian2xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 1, CanInitiateRecovery: false},
			{Type: "agent", Identifier: "zrn1guardian3xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 2, CanInitiateRecovery: false},
		},
		RecoveryDelayBlocks:   500,
		ChallengePeriodBlocks: 250,
	}

	_, err := ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored, found := k.GetRecoveryConfig(ctx, testAddr1)
	if !found {
		t.Fatal("recovery config not found")
	}
	if stored.Threshold != 2 {
		t.Errorf("expected threshold 2, got %d", stored.Threshold)
	}
	if stored.TotalShards != 3 {
		t.Errorf("expected total shards 3, got %d", stored.TotalShards)
	}
	if len(stored.ShardHolders) != 3 {
		t.Errorf("expected 3 shard holders, got %d", len(stored.ShardHolders))
	}
}

func TestSetRecoveryConfig_AccountNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &types.RecoveryConfig{Threshold: 1, TotalShards: 1},
	})
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}
}

// ---------- InitiateRecovery Tests ----------

func TestInitiateRecovery_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   2,
		TotalShards: 3,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
			{Type: "agent", Identifier: "zrn1guardian2xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 1, CanInitiateRecovery: false},
			{Type: "agent", Identifier: "zrn1guardian3xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 2, CanInitiateRecovery: false},
		},
		RecoveryDelayBlocks:   500,
		ChallengePeriodBlocks: 250,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, err := ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, found := k.GetRecoveryRequest(ctx, testAddr1)
	if !found {
		t.Fatal("recovery request not found")
	}
	if req.Status != "pending" {
		t.Errorf("expected status pending, got %s", req.Status)
	}
	if req.InitiatedBy != testAddr2 {
		t.Errorf("expected initiated by %s, got %s", testAddr2, req.InitiatedBy)
	}
	if req.NewOperationalKey != testPubKey2 {
		t.Errorf("expected new key %s, got %s", testPubKey2, req.NewOperationalKey)
	}
	if req.ShardsRequired != 2 {
		t.Errorf("expected shards required 2, got %d", req.ShardsRequired)
	}
	if req.DelayExpiresAt != 600 { // 100 + 500
		t.Errorf("expected delay expires at 600, got %d", req.DelayExpiresAt)
	}
	if req.ChallengeExpiresAt != 850 { // 100 + 500 + 250
		t.Errorf("expected challenge expires at 850, got %d", req.ChallengeExpiresAt)
	}

	account, _ := k.GetAccount(ctx, testAddr1)
	if !account.Flags.InRecovery {
		t.Fatal("expected InRecovery flag to be set")
	}
}

func TestInitiateRecovery_UnauthorizedInitiator(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   2,
		TotalShards: 3,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: false},
		},
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, err := ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})
	if err == nil {
		t.Fatal("expected error for unauthorized initiator")
	}
}

func TestInitiateRecovery_AlreadyActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, err := ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})
	if err == nil {
		t.Fatal("expected error for already active recovery")
	}
}

func TestInitiateRecovery_NoConfig(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, err := ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})
	if err == nil {
		t.Fatal("expected error for missing recovery config")
	}
}

// ---------- SubmitRecoveryShard Tests ----------

func TestSubmitRecoveryShard_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   2,
		TotalShards: 3,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
			{Type: "agent", Identifier: "zrn1guardian2xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 1, CanInitiateRecovery: false},
			{Type: "agent", Identifier: "zrn1guardian3xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 2, CanInitiateRecovery: false},
		},
		RecoveryDelayBlocks:   500,
		ChallengePeriodBlocks: 250,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, err := ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})
	if err != nil {
		t.Fatalf("unexpected error submitting shard 0: %v", err)
	}

	req, _ := k.GetRecoveryRequest(ctx, testAddr1)
	if req.Status != "pending" {
		t.Errorf("expected status pending after 1 shard, got %s", req.Status)
	}
	if len(req.ShardsProvided) != 1 {
		t.Errorf("expected 1 shard provided, got %d", len(req.ShardsProvided))
	}

	_, err = ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         "zrn1guardian2xxxxxxxxxxxxxxxxxxxxxxx6",
		AccountAddress: testAddr1,
		ShardIndex:     1,
	})
	if err != nil {
		t.Fatalf("unexpected error submitting shard 1: %v", err)
	}

	req, _ = k.GetRecoveryRequest(ctx, testAddr1)
	if req.Status != "delayed" {
		t.Errorf("expected status delayed after 2 shards (threshold met), got %s", req.Status)
	}
}

func TestSubmitRecoveryShard_DuplicateShard(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   2,
		TotalShards: 2,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
			{Type: "agent", Identifier: "zrn1guardian2xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 1, CanInitiateRecovery: false},
		},
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, _ = ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})

	_, err := ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})
	if err == nil {
		t.Fatal("expected error for duplicate shard submission")
	}
}

func TestSubmitRecoveryShard_WrongSender(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 2,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
			{Type: "agent", Identifier: "zrn1guardian2xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 1, CanInitiateRecovery: false},
		},
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, err := ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     1,
	})
	if err == nil {
		t.Fatal("expected error for wrong shard holder")
	}
}

// ---------- ChallengeRecovery Tests ----------

func TestChallengeRecovery_OwnerCancels(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
		RecoveryDelayBlocks:   100,
		ChallengePeriodBlocks: 100,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, _ = ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})

	challengeCtx := ctx.WithBlockHeight(201)
	k.ProcessRecoveryTimeouts(challengeCtx)

	req, _ := k.GetRecoveryRequest(challengeCtx, testAddr1)
	if req.Status != "challengeable" {
		t.Fatalf("expected challengeable, got %s", req.Status)
	}

	_, err := ms.ChallengeRecovery(challengeCtx, &types.MsgChallengeRecovery{
		Sender:         testAddr1,
		AccountAddress: testAddr1,
		Reason:         "I still have my keys",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ = k.GetRecoveryRequest(challengeCtx, testAddr1)
	if req.Status != "cancelled" {
		t.Errorf("expected status cancelled, got %s", req.Status)
	}
	if req.ChallengerAddress != testAddr1 {
		t.Errorf("expected challenger %s, got %s", testAddr1, req.ChallengerAddress)
	}

	account, _ := k.GetAccount(challengeCtx, testAddr1)
	if account.Flags.InRecovery {
		t.Fatal("expected InRecovery flag cleared after challenge")
	}
}

func TestChallengeRecovery_NotChallengeable(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, err := ms.ChallengeRecovery(ctx, &types.MsgChallengeRecovery{
		Sender:         testAddr1,
		AccountAddress: testAddr1,
		Reason:         "too early",
	})
	if err == nil {
		t.Fatal("expected error for challenging non-challengeable recovery")
	}
}

func TestChallengeRecovery_ThirdPartyBlocked(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
		RecoveryDelayBlocks:   10,
		ChallengePeriodBlocks: 10,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})
	_, _ = ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})

	challengeCtx := ctx.WithBlockHeight(111)
	k.ProcessRecoveryTimeouts(challengeCtx)

	_, err := ms.ChallengeRecovery(challengeCtx, &types.MsgChallengeRecovery{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		Reason:         "I'm not the owner",
	})
	if err == nil {
		t.Fatal("expected error for third party challenge")
	}
}

// ---------- ExecuteRecovery Tests ----------

func TestExecuteRecovery_FullLifecycle(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 10000,
	})
	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s2"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 10000,
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
		RecoveryDelayBlocks:   10,
		ChallengePeriodBlocks: 10,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, _ = ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})

	delayCtx := ctx.WithBlockHeight(111)
	k.ProcessRecoveryTimeouts(delayCtx)
	req, _ := k.GetRecoveryRequest(delayCtx, testAddr1)
	if req.Status != "challengeable" {
		t.Fatalf("expected challengeable, got %s", req.Status)
	}

	execCtx := ctx.WithBlockHeight(121)
	k.ProcessRecoveryTimeouts(execCtx)
	req, _ = k.GetRecoveryRequest(execCtx, testAddr1)
	if req.Status != "executable" {
		t.Fatalf("expected executable, got %s", req.Status)
	}

	_, err := ms.ExecuteRecovery(execCtx, &types.MsgExecuteRecovery{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, _ := k.GetAccount(execCtx, testAddr1)
	if account.OperationalPublicKey != testPubKey2 {
		t.Errorf("expected new operational key %s, got %s", testPubKey2, account.OperationalPublicKey)
	}
	if account.OperationalKeyVersion != 2 {
		t.Errorf("expected key version 2, got %d", account.OperationalKeyVersion)
	}
	if account.Flags.InRecovery {
		t.Fatal("expected InRecovery flag cleared")
	}

	if account.SessionKeyCount != 0 {
		t.Errorf("expected session count 0 after recovery, got %d", account.SessionKeyCount)
	}
	sessions := k.GetSessionKeysForOwner(execCtx, testAddr1)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after recovery, got %d", len(sessions))
	}

	req, _ = k.GetRecoveryRequest(execCtx, testAddr1)
	if req.Status != "completed" {
		t.Errorf("expected status completed, got %s", req.Status)
	}
}

func TestExecuteRecovery_NotExecutable(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})

	_, err := ms.ExecuteRecovery(ctx, &types.MsgExecuteRecovery{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
	})
	if err == nil {
		t.Fatal("expected error for executing non-executable recovery")
	}
}

// ---------- BeginBlock Session Cleanup Tests ----------

func TestCleanupExpiredSessions(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	shortResp, _ := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("short"),
		ExpiresAtHeight: 150,
	})
	longResp, _ := ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("long"),
		ExpiresAtHeight: 1100,
	})

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.SessionKeyCount != 2 {
		t.Fatalf("expected 2 sessions, got %d", account.SessionKeyCount)
	}

	expiredCtx := ctx.WithBlockHeight(151)
	k.CleanupExpiredSessions(expiredCtx)

	_, found := k.GetSessionKey(expiredCtx, testAddr1, shortResp.SessionId)
	if found {
		t.Fatal("expected short session to be cleaned up")
	}

	_, found = k.GetSessionKey(expiredCtx, testAddr1, longResp.SessionId)
	if !found {
		t.Fatal("expected long session to still exist")
	}

	account, _ = k.GetAccount(expiredCtx, testAddr1)
	if account.SessionKeyCount != 1 {
		t.Errorf("expected session count 1 after cleanup, got %d", account.SessionKeyCount)
	}
}

func TestCleanupExpiredSessions_AllExpired(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: 110,
	})
	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s2"),
		ExpiresAtHeight: 120,
	})

	expiredCtx := ctx.WithBlockHeight(200)
	k.CleanupExpiredSessions(expiredCtx)

	account, _ := k.GetAccount(expiredCtx, testAddr1)
	if account.SessionKeyCount != 0 {
		t.Errorf("expected session count 0, got %d", account.SessionKeyCount)
	}

	sessions := k.GetSessionKeysForOwner(expiredCtx, testAddr1)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestCleanupExpiredSessions_NoneExpired(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})

	k.CleanupExpiredSessions(ctx)

	account, _ := k.GetAccount(ctx, testAddr1)
	if account.SessionKeyCount != 1 {
		t.Errorf("expected session count 1 (no cleanup), got %d", account.SessionKeyCount)
	}
}

// ---------- Recovery Timeout Processing Tests ----------

func TestProcessRecoveryTimeouts_DelayedToChallengeable(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
		RecoveryDelayBlocks:   50,
		ChallengePeriodBlocks: 50,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})
	_, _ = ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})

	req, _ := k.GetRecoveryRequest(ctx, testAddr1)
	if req.Status != "delayed" {
		t.Fatalf("expected delayed, got %s", req.Status)
	}

	beforeCtx := ctx.WithBlockHeight(149)
	k.ProcessRecoveryTimeouts(beforeCtx)
	req, _ = k.GetRecoveryRequest(beforeCtx, testAddr1)
	if req.Status != "delayed" {
		t.Errorf("expected still delayed before expiry, got %s", req.Status)
	}

	afterCtx := ctx.WithBlockHeight(151)
	k.ProcessRecoveryTimeouts(afterCtx)
	req, _ = k.GetRecoveryRequest(afterCtx, testAddr1)
	if req.Status != "challengeable" {
		t.Errorf("expected challengeable after delay, got %s", req.Status)
	}
}

func TestProcessRecoveryTimeouts_ChallengeableToExecutable(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   1,
		TotalShards: 1,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
		},
		RecoveryDelayBlocks:   10,
		ChallengePeriodBlocks: 10,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	_, _ = ms.InitiateRecovery(ctx, &types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	})
	_, _ = ms.SubmitRecoveryShard(ctx, &types.MsgSubmitRecoveryShard{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
		ShardIndex:     0,
	})

	afterDelay := ctx.WithBlockHeight(111)
	k.ProcessRecoveryTimeouts(afterDelay)
	req, _ := k.GetRecoveryRequest(afterDelay, testAddr1)
	if req.Status != "challengeable" {
		t.Fatalf("expected challengeable, got %s", req.Status)
	}

	afterChallenge := ctx.WithBlockHeight(121)
	k.ProcessRecoveryTimeouts(afterChallenge)
	req, _ = k.GetRecoveryRequest(afterChallenge, testAddr1)
	if req.Status != "executable" {
		t.Errorf("expected executable after challenge period, got %s", req.Status)
	}
}

// ---------- ValidateBasic Tests for Msg Types ----------

func TestMsgFreezeAccount_ValidateBasic(t *testing.T) {
	msg := types.MsgFreezeAccount{
		Sender:  testAddr1,
		Address: testAddr1,
		Reason:  "test",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.Sender = ""
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty sender")
	}

	msg.Sender = testAddr1
	msg.Address = ""
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty address")
	}
}

func TestMsgUnfreezeAccount_ValidateBasic(t *testing.T) {
	msg := types.MsgUnfreezeAccount{
		Authority: testAddr2,
		Address:   testAddr1,
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.Authority = ""
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty authority")
	}
}

func TestMsgInitiateRecovery_ValidateBasic(t *testing.T) {
	msg := types.MsgInitiateRecovery{
		Sender:            testAddr2,
		AccountAddress:    testAddr1,
		NewOperationalKey: testPubKey2,
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.NewOperationalKey = ""
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty new key")
	}
}

func TestMsgExecuteRecovery_ValidateBasic(t *testing.T) {
	msg := types.MsgExecuteRecovery{
		Sender:         testAddr2,
		AccountAddress: testAddr1,
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.AccountAddress = ""
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty account address")
	}
}

// ---------- UpdateParams Tests ----------

func TestUpdateParams_AuthoritySuccess(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	newParams := types.Params{
		MaxSessionKeys:                10,
		MaxSessionDuration:            50000,
		KeyRotationCooldown:           222,
		RecoveryDelayBlocks:           2000,
		ChallengePeriodBlocks:         1000,
		BootstrapEnabled:              false,
		BootstrapAmount:               "0",
		MaxMetadataLength:             2048,
		RequireDid:                    true,
		MaxRecoveryShards:             20,
		RecoveryChallengePeriodBlocks: 1000,
		RecoveryExecutionDelayBlocks:  2000,
	}

	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    &newParams,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := k.GetParams(ctx)
	if stored.MaxSessionKeys != 10 {
		t.Errorf("expected MaxSessionKeys 10, got %d", stored.MaxSessionKeys)
	}
	if stored.MaxSessionDuration != 50000 {
		t.Errorf("expected MaxSessionDuration 50000, got %d", stored.MaxSessionDuration)
	}
	if stored.KeyRotationCooldown != 222 {
		t.Errorf("expected KeyRotationCooldown 222, got %d", stored.KeyRotationCooldown)
	}
	if stored.BootstrapEnabled {
		t.Error("expected BootstrapEnabled false")
	}
}

func TestUpdateParams_NonAuthorityRejected(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	defaultParams := types.DefaultParams()
	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr1,
		Params:    &defaultParams,
	})
	if err == nil {
		t.Fatal("expected error for non-authority UpdateParams")
	}
}

func TestUpdateParams_InvalidParamsRejected(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	invalidParams := types.Params{
		MaxSessionKeys:     0,
		MaxSessionDuration: 1000,
	}
	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    &invalidParams,
	})
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	dp := types.DefaultParams()
	msg := types.MsgUpdateParams{
		Authority: testAddr1,
		Params:    &dp,
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	msg.Authority = "invalid"
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for invalid authority")
	}
}

// ---------- Genesis Roundtrip with RecoveryConfig Tests ----------

func TestGenesisRoundtrip_WithRecoveryConfigs(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	config := types.RecoveryConfig{
		Threshold:   2,
		TotalShards: 3,
		ShardHolders: []*types.ShardHolder{
			{Type: "agent", Identifier: testAddr2, ShardIndex: 0, CanInitiateRecovery: true},
			{Type: "agent", Identifier: "zrn1guardian2xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 1},
			{Type: "agent", Identifier: "zrn1guardian3xxxxxxxxxxxxxxxxxxxxxxx6", ShardIndex: 2},
		},
		RecoveryDelayBlocks:   500,
		ChallengePeriodBlocks: 250,
	}
	_, _ = ms.SetRecoveryConfig(ctx, &types.MsgSetRecoveryConfig{
		Sender: testAddr1,
		Config: &config,
	})

	gs := k.ExportGenesis(ctx)
	if len(gs.RecoveryConfigs) != 1 {
		t.Fatalf("expected 1 recovery config in export, got %d", len(gs.RecoveryConfigs))
	}
	if gs.RecoveryConfigs[0].AccountAddress != testAddr1 {
		t.Errorf("expected AccountAddress %s, got %s", testAddr1, gs.RecoveryConfigs[0].AccountAddress)
	}
	if gs.RecoveryConfigs[0].Threshold != 2 {
		t.Errorf("expected threshold 2, got %d", gs.RecoveryConfigs[0].Threshold)
	}

	k2, ctx2 := setupKeeper(t)
	if err := k2.InitGenesis(ctx2, gs); err != nil {
		t.Fatalf("InitGenesis failed: %v", err)
	}

	rc, found := k2.GetRecoveryConfig(ctx2, testAddr1)
	if !found {
		t.Fatal("recovery config lost during genesis roundtrip")
	}
	if rc.Threshold != 2 {
		t.Errorf("expected threshold 2 after roundtrip, got %d", rc.Threshold)
	}
	if rc.TotalShards != 3 {
		t.Errorf("expected total shards 3, got %d", rc.TotalShards)
	}
	if len(rc.ShardHolders) != 3 {
		t.Errorf("expected 3 shard holders, got %d", len(rc.ShardHolders))
	}
	if rc.RecoveryDelayBlocks != 500 {
		t.Errorf("expected delay 500, got %d", rc.RecoveryDelayBlocks)
	}

	acc, found := k2.GetAccount(ctx2, testAddr1)
	if !found {
		t.Fatal("account lost during genesis roundtrip")
	}
	if acc.Did != testDID1 {
		t.Errorf("expected DID %s, got %s", testDID1, acc.Did)
	}
}

// ---------- Invariant Tests ----------

func TestAccountDIDParityInvariant_Passes(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	inv := keeper.AccountDIDParityInvariant(k)
	msg, broken := inv(ctx)
	if broken {
		t.Errorf("invariant should pass: %s", msg)
	}
}

func TestAccountDIDParityInvariant_DetectsOrphanedAccount(t *testing.T) {
	k, ctx := setupKeeper(t)

	account := types.Account{
		Address:     testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	}
	k.SetAccount(ctx, &account)

	inv := keeper.AccountDIDParityInvariant(k)
	_, broken := inv(ctx)
	if !broken {
		t.Error("invariant should detect orphaned account without DID mapping")
	}
}

func TestSessionCountConsistencyInvariant_Passes(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})
	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})

	inv := keeper.SessionCountConsistencyInvariant(k)
	msg, broken := inv(ctx)
	if broken {
		t.Errorf("invariant should pass: %s", msg)
	}
}

func TestSessionCountConsistencyInvariant_DetectsMismatch(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, _ = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
	})

	_, _ = ms.CreateSession(ctx, &types.MsgCreateSession{
		Owner:           testAddr1,
		SessionPubKey:   []byte("s1"),
		ExpiresAtHeight: uint64(ctx.BlockHeight()) + 1000,
	})
	account, _ := k.GetAccount(ctx, testAddr1)
	account.SessionKeyCount = 99
	k.SetAccount(ctx, account)

	inv := keeper.SessionCountConsistencyInvariant(k)
	_, broken := inv(ctx)
	if !broken {
		t.Error("invariant should detect session count mismatch")
	}
}

func TestParamsValidInvariant_Passes(t *testing.T) {
	k, ctx := setupKeeper(t)

	inv := keeper.ParamsValidInvariant(k)
	msg, broken := inv(ctx)
	if broken {
		t.Errorf("invariant should pass with default params: %s", msg)
	}
}

// ---------- Metadata Tests (Zerone-specific) ----------

func TestRegisterAccount_WithMetadata(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Sender:      testAddr1,
		Did:         testDID1,
		PublicKey:   testPubKey1,
		AccountType: "agent",
		Metadata:    `{"name":"TestAgent","version":"1.0"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account, found := k.GetAccount(ctx, testAddr1)
	if !found {
		t.Fatal("account not found")
	}
	if account.Metadata != `{"name":"TestAgent","version":"1.0"}` {
		t.Errorf("expected metadata preserved, got %s", account.Metadata)
	}
}
