package keeper_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	sdkmath "cosmossdk.io/math"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/tokens/keeper"
	"github.com/zerone-chain/zerone/x/tokens/types"
)

// ---------- Mock BankKeeper ----------

type mockBankKeeper struct {
	// balances: addr -> denom -> amount
	balances map[string]map[string]sdkmath.Int
	// moduleBalances: moduleName -> denom -> amount
	moduleBalances map[string]map[string]sdkmath.Int
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances:       make(map[string]map[string]sdkmath.Int),
		moduleBalances: make(map[string]map[string]sdkmath.Int),
	}
}

func (m *mockBankKeeper) getOrInitAddr(addr string) map[string]sdkmath.Int {
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = make(map[string]sdkmath.Int)
	}
	return m.balances[addr]
}

func (m *mockBankKeeper) getOrInitModule(mod string) map[string]sdkmath.Int {
	if _, ok := m.moduleBalances[mod]; !ok {
		m.moduleBalances[mod] = make(map[string]sdkmath.Int)
	}
	return m.moduleBalances[mod]
}

func (m *mockBankKeeper) getBalSafe(bals map[string]sdkmath.Int, denom string) sdkmath.Int {
	v, ok := bals[denom]
	if !ok {
		return sdkmath.ZeroInt()
	}
	return v
}

func (m *mockBankKeeper) MintCoins(_ context.Context, moduleName string, amt sdk.Coins) error {
	modBals := m.getOrInitModule(moduleName)
	for _, c := range amt {
		cur := m.getBalSafe(modBals, c.Denom)
		modBals[c.Denom] = cur.Add(c.Amount)
	}
	return nil
}

func (m *mockBankKeeper) BurnCoins(_ context.Context, moduleName string, amt sdk.Coins) error {
	modBals := m.getOrInitModule(moduleName)
	for _, c := range amt {
		cur := m.getBalSafe(modBals, c.Denom)
		if cur.LT(c.Amount) {
			return types.ErrInsufficientBalance
		}
		modBals[c.Denom] = cur.Sub(c.Amount)
	}
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	modBals := m.getOrInitModule(senderModule)
	addrBals := m.getOrInitAddr(recipientAddr.String())
	for _, c := range amt {
		cur := m.getBalSafe(modBals, c.Denom)
		if cur.LT(c.Amount) {
			return types.ErrInsufficientBalance
		}
		modBals[c.Denom] = cur.Sub(c.Amount)
		addrBals[c.Denom] = m.getBalSafe(addrBals, c.Denom).Add(c.Amount)
	}
	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	addrBals := m.getOrInitAddr(senderAddr.String())
	modBals := m.getOrInitModule(recipientModule)
	for _, c := range amt {
		cur := m.getBalSafe(addrBals, c.Denom)
		if cur.LT(c.Amount) {
			return types.ErrInsufficientBalance
		}
		addrBals[c.Denom] = cur.Sub(c.Amount)
		modBals[c.Denom] = m.getBalSafe(modBals, c.Denom).Add(c.Amount)
	}
	return nil
}

func (m *mockBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	bals := m.getOrInitAddr(addr.String())
	return sdk.NewCoin(denom, m.getBalSafe(bals, denom))
}

func (m *mockBankKeeper) GetSupply(_ context.Context, denom string) sdk.Coin {
	total := sdkmath.ZeroInt()
	for _, bals := range m.balances {
		if v, ok := bals[denom]; ok {
			total = total.Add(v)
		}
	}
	for _, bals := range m.moduleBalances {
		if v, ok := bals[denom]; ok {
			total = total.Add(v)
		}
	}
	return sdk.NewCoin(denom, total)
}

// ---------- Setup Helpers ----------

func setupKeeperWithBank(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatal(err)
	}

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	storeService := runtime.NewKVStoreService(storeKey)

	bk := newMockBankKeeper()
	k := keeper.NewKeeper(cdc, storeService, bk, testAuthority)
	return k, ctx, bk
}

func setupMsgServerWithBank(t *testing.T) (*keeper.Keeper, sdk.Context, types.MsgServer, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	srv := keeper.NewMsgServerImpl(k)
	return &k, ctx, srv, bk
}

// createWrappableToken is a helper to create a token with Wrappable + Burnable features.
func createWrappableToken(t *testing.T, srv types.MsgServer, ctx sdk.Context, creator, symbol string) string {
	t.Helper()
	return createTestToken(t, srv, ctx, creator, symbol, &types.TokenFeatures{
		Mintable:  true,
		Burnable:  true,
		Pausable:  true,
		Wrappable: true,
	})
}

// -----------------------------------------------------------------------
// DelegatePower Tests
// -----------------------------------------------------------------------

func TestDelegatePower_Success(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "DELEG", &types.TokenFeatures{})

	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator,
		TokenId:   tokenId,
		Delegate:  testUser1,
		Amount:    "500",
	})
	if err != nil {
		t.Fatalf("DelegatePower failed: %v", err)
	}
}

func TestDelegatePower_IncreaseThenDecrease(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	tokenId := createTestToken(t, srv, ctx, testCreator, "DELID", &types.TokenFeatures{})

	// Delegate 300
	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator,
		TokenId:   tokenId,
		Delegate:  testUser1,
		Amount:    "300",
	})
	if err != nil {
		t.Fatalf("initial delegation failed: %v", err)
	}

	del := k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected 300 delegation, got %s", del.String())
	}

	total := k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected 300 total, got %s", total.String())
	}

	// Increase to 600
	_, err = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator,
		TokenId:   tokenId,
		Delegate:  testUser1,
		Amount:    "600",
	})
	if err != nil {
		t.Fatalf("increase failed: %v", err)
	}

	del = k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Cmp(big.NewInt(600)) != 0 {
		t.Fatalf("expected 600, got %s", del.String())
	}

	// Decrease to 200
	_, err = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator,
		TokenId:   tokenId,
		Delegate:  testUser1,
		Amount:    "200",
	})
	if err != nil {
		t.Fatalf("decrease failed: %v", err)
	}

	del = k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Cmp(big.NewInt(200)) != 0 {
		t.Fatalf("expected 200, got %s", del.String())
	}

	total = k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Cmp(big.NewInt(200)) != 0 {
		t.Fatalf("expected total 200, got %s", total.String())
	}
}

func TestDelegatePower_RevokeWithZero(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "REVOK", &types.TokenFeatures{})

	// Delegate 500
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})

	// Revoke
	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "0",
	})
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	del := k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Sign() != 0 {
		t.Fatalf("expected 0 after revoke, got %s", del.String())
	}

	total := k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Sign() != 0 {
		t.Fatalf("expected total 0 after revoke, got %s", total.String())
	}
}

func TestDelegatePower_SelfDelegationRejected(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "SELFD", &types.TokenFeatures{})

	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testCreator, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected error for self-delegation")
	}
}

func TestDelegatePower_TokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: "nonexistent", Delegate: testUser1, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected error for non-existent token")
	}
}

func TestDelegatePower_TokenPaused(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "PAUSE", &types.TokenFeatures{Pausable: true})

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected error for paused token")
	}
}

func TestDelegatePower_InsufficientUndelegatedBalance(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	// Initial supply 1000000
	tokenId := createTestToken(t, srv, ctx, testCreator, "INSUF", &types.TokenFeatures{})

	// Delegate more than balance
	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "2000000",
	})
	if err == nil {
		t.Fatal("expected insufficient undelegated balance error")
	}
}

func TestDelegatePower_MultipleDelegates(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "MULTI", &types.TokenFeatures{})

	// Delegate to user1 and user2
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "300",
	})
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser2, Amount: "200",
	})

	total := k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected total 500, got %s", total.String())
	}

	undel := k.GetUndelegatedBalance(ctx, tokenId, testCreator)
	if undel.Cmp(big.NewInt(999500)) != 0 {
		t.Fatalf("expected undelegated 999500, got %s", undel.String())
	}
}

// -----------------------------------------------------------------------
// UndelegatePower Tests (Zerone-specific)
// -----------------------------------------------------------------------

func TestUndelegatePower_Success(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "UNDEL", &types.TokenFeatures{})

	// Delegate 500
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})

	// Undelegate 200
	_, err := srv.UndelegatePower(ctx, &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "200",
	})
	if err != nil {
		t.Fatalf("UndelegatePower failed: %v", err)
	}

	del := k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected 300 delegation after undelegate, got %s", del.String())
	}

	total := k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected total 300, got %s", total.String())
	}
}

func TestUndelegatePower_Full(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "UFULL", &types.TokenFeatures{})

	// Delegate 500
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})

	// Undelegate all 500
	_, err := srv.UndelegatePower(ctx, &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})
	if err != nil {
		t.Fatalf("full undelegate failed: %v", err)
	}

	del := k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Sign() != 0 {
		t.Fatalf("expected 0 delegation after full undelegate, got %s", del.String())
	}

	total := k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Sign() != 0 {
		t.Fatalf("expected total 0, got %s", total.String())
	}
}

func TestUndelegatePower_ExceedsDelegation(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "UEXCD", &types.TokenFeatures{})

	// Delegate 500
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})

	// Try to undelegate 600
	_, err := srv.UndelegatePower(ctx, &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "600",
	})
	if err == nil {
		t.Fatal("expected error when undelegating more than delegated")
	}
}

func TestUndelegatePower_TokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.UndelegatePower(ctx, &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: "nonexistent", Delegate: testUser1, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected error for non-existent token")
	}
}

func TestUndelegatePower_SelfDelegationRejected(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "USELF", &types.TokenFeatures{})

	_, err := srv.UndelegatePower(ctx, &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testCreator, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected error for self-delegation")
	}
}

func TestUndelegatePower_ZeroAmountRejected(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "UZAMT", &types.TokenFeatures{})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})

	_, err := srv.UndelegatePower(ctx, &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "0",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

// -----------------------------------------------------------------------
// Delegation-Aware Transfer/Burn/TransferFrom Tests
// -----------------------------------------------------------------------

func TestTransfer_BlockedByDelegation(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TBLK", &types.TokenFeatures{})

	// Delegate almost everything
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "999999",
	})

	// Transfer 2 (only 1 undelegated)
	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser2, Amount: "2",
	})
	if err == nil {
		t.Fatal("expected transfer blocked by delegation")
	}
}

func TestTransfer_SucceedsAfterDelegationReduction(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TRED", &types.TokenFeatures{})

	// Delegate 999999
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "999999",
	})

	// Reduce delegation to 999000
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "999000",
	})

	// Transfer 500 (now 1000 undelegated)
	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser2, Amount: "500",
	})
	if err != nil {
		t.Fatalf("transfer should succeed after delegation reduction: %v", err)
	}
}

func TestBurn_BlockedByDelegation(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "BBLK", &types.TokenFeatures{Burnable: true})

	// Delegate all
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "1000000",
	})

	_, err := srv.BurnToken(ctx, &types.MsgBurnToken{
		Burner: testCreator, TokenId: tokenId, Amount: "1",
	})
	if err == nil {
		t.Fatal("expected burn blocked by delegation")
	}
}

func TestTransfer_Delegate80Pct_Transfer50Pct_Fails(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "D80T", &types.TokenFeatures{})

	// Delegate 80% = 800,000
	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "800000",
	})
	if err != nil {
		t.Fatalf("delegate failed: %v", err)
	}

	// Try transfer 50% = 500,000 (only 200,000 undelegated)
	_, err = srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser2, Amount: "500000",
	})
	if err == nil {
		t.Fatal("expected transfer to fail: 500K exceeds 200K undelegated")
	}
}

func TestBurn_Delegate80Pct_Burn50Pct_Fails(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "D80B", &types.TokenFeatures{Burnable: true})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "800000",
	})

	_, err := srv.BurnToken(ctx, &types.MsgBurnToken{
		Burner: testCreator, TokenId: tokenId, Amount: "500000",
	})
	if err == nil {
		t.Fatal("expected burn to fail: 500K exceeds 200K undelegated")
	}
}

func TestTransfer_Delegate80Pct_Transfer20Pct_Succeeds(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "D80S", &types.TokenFeatures{})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "800000",
	})

	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser2, Amount: "200000",
	})
	if err != nil {
		t.Fatalf("expected transfer to succeed with exactly 200K undelegated: %v", err)
	}

	creatorBal := k.GetBalance(ctx, tokenId, testCreator)
	if creatorBal.Cmp(big.NewInt(800000)) != 0 {
		t.Fatalf("expected creator balance 800000, got %s", creatorBal.String())
	}
	user2Bal := k.GetBalance(ctx, tokenId, testUser2)
	if user2Bal.Cmp(big.NewInt(200000)) != 0 {
		t.Fatalf("expected user2 balance 200000, got %s", user2Bal.String())
	}
}

func TestTransfer_DelegateThenRevoke_TransferFull_Succeeds(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "DRVK", &types.TokenFeatures{})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "800000",
	})

	_, err := srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "0",
	})
	if err != nil {
		t.Fatalf("revoke delegation failed: %v", err)
	}

	_, err = srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser2, Amount: "1000000",
	})
	if err != nil {
		t.Fatalf("expected full transfer after revoke: %v", err)
	}

	user2Bal := k.GetBalance(ctx, tokenId, testUser2)
	if user2Bal.Cmp(big.NewInt(1000000)) != 0 {
		t.Fatalf("expected user2 balance 1000000, got %s", user2Bal.String())
	}
}

func TestTransferFrom_BlockedByDelegation(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TFDB", &types.TokenFeatures{})

	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "500000",
	})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser2, Amount: "900000",
	})

	// TransferFrom 200,000 (within allowance but exceeds undelegated)
	_, err := srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1, TokenId: tokenId, From: testCreator, To: testUser3, Amount: "200000",
	})
	if err == nil {
		t.Fatal("expected TransferFrom to fail: 200K exceeds 100K undelegated")
	}

	// TransferFrom 100,000 (within undelegated and allowance)
	_, err = srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1, TokenId: tokenId, From: testCreator, To: testUser3, Amount: "100000",
	})
	if err != nil {
		t.Fatalf("expected TransferFrom 100K to succeed: %v", err)
	}
}

// -----------------------------------------------------------------------
// WrapToken Tests
// -----------------------------------------------------------------------

func TestWrapToken_Success(t *testing.T) {
	_, ctx, srv, bk := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "WRAP")

	resp, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender:  testCreator,
		TokenId: tokenId,
		Amount:  "1000",
	})
	if err != nil {
		t.Fatalf("WrapToken failed: %v", err)
	}

	expectedDenom := "zrn20/" + tokenId
	if resp.WrappedDenom != expectedDenom {
		t.Fatalf("expected denom %s, got %s", expectedDenom, resp.WrappedDenom)
	}

	// Check sender received wrapped coins
	senderAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin := bk.GetBalance(ctx, senderAddr, expectedDenom)
	if !coin.Amount.Equal(sdkmath.NewInt(1000)) {
		t.Fatalf("expected 1000 wrapped coins, got %s", coin.Amount.String())
	}
}

func TestWrapToken_NotWrappable(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "NWRAP", &types.TokenFeatures{
		Mintable: true,
		Burnable: true,
	})

	_, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected not wrappable error")
	}
}

func TestWrapToken_TokenNotFound(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)

	_, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: "nonexistent", Amount: "100",
	})
	if err == nil {
		t.Fatal("expected token not found")
	}
}

func TestWrapToken_TokenPaused(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "WPAUS")

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected paused error")
	}
}

func TestWrapToken_InsufficientBalance(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "WINSF")

	_, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "99999999",
	})
	if err == nil {
		t.Fatal("expected insufficient balance")
	}
}

func TestWrapToken_InsufficientUndelegatedBalance(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "WDINS")

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "999999",
	})

	// Try to wrap 2 (only 1 undelegated)
	_, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "2",
	})
	if err == nil {
		t.Fatal("expected insufficient undelegated balance")
	}
}

func TestWrapToken_IdempotentWrapRecord(t *testing.T) {
	k, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "WIDEM")

	resp1, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})
	resp2, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})

	if resp1.WrappedDenom != resp2.WrappedDenom {
		t.Fatal("wrap record should be idempotent")
	}

	resolved := k.GetTokenIdByWrappedDenom(ctx, resp1.WrappedDenom)
	if resolved != tokenId {
		t.Fatalf("expected token ID %s, got %s", tokenId, resolved)
	}
}

func TestWrapToken_DenomFormat(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "WFMT")

	resp, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})

	expected := "zrn20/" + tokenId
	if resp.WrappedDenom != expected {
		t.Fatalf("expected %s, got %s", expected, resp.WrappedDenom)
	}
}

// -----------------------------------------------------------------------
// UnwrapToken Tests
// -----------------------------------------------------------------------

func TestUnwrapToken_Success(t *testing.T) {
	k, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "UNWRP")

	balBefore := k.GetBalance(ctx, tokenId, testCreator)

	// Wrap 500
	resp, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "500",
	})

	balAfterWrap := k.GetBalance(ctx, tokenId, testCreator)
	expectedAfterWrap := new(big.Int).Sub(balBefore, big.NewInt(500))
	if balAfterWrap.Cmp(expectedAfterWrap) != 0 {
		t.Fatalf("expected %s after wrap, got %s", expectedAfterWrap, balAfterWrap)
	}

	// Unwrap 300
	unwrapResp, err := srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender:       testCreator,
		WrappedDenom: resp.WrappedDenom,
		Amount:       "300",
	})
	if err != nil {
		t.Fatalf("UnwrapToken failed: %v", err)
	}
	if unwrapResp.TokenId != tokenId {
		t.Fatalf("expected token ID %s, got %s", tokenId, unwrapResp.TokenId)
	}

	balAfterUnwrap := k.GetBalance(ctx, tokenId, testCreator)
	expectedAfterUnwrap := new(big.Int).Add(expectedAfterWrap, big.NewInt(300))
	if balAfterUnwrap.Cmp(expectedAfterUnwrap) != 0 {
		t.Fatalf("expected %s after unwrap, got %s", expectedAfterUnwrap, balAfterUnwrap)
	}
}

func TestUnwrapToken_UnknownDenom(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)

	_, err := srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: "zrn20/nonexistent", Amount: "100",
	})
	if err == nil {
		t.Fatal("expected wrap record not found")
	}
}

func TestUnwrapToken_TokenPaused(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "UPAUS")

	resp, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "500",
	})

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: resp.WrappedDenom, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected paused error")
	}
}

func TestUnwrapToken_InsufficientWrappedCoins(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "UINSF")

	resp, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})

	_, err := srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: resp.WrappedDenom, Amount: "200",
	})
	if err == nil {
		t.Fatal("expected insufficient wrapped coins")
	}
}

func TestWrapUnwrap_RoundTrip(t *testing.T) {
	k, ctx, srv, bk := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "ROUND")

	balBefore := k.GetBalance(ctx, tokenId, testCreator)

	// Wrap 1000
	resp, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "1000",
	})

	// Unwrap all 1000
	_, err := srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: resp.WrappedDenom, Amount: "1000",
	})
	if err != nil {
		t.Fatalf("round-trip unwrap failed: %v", err)
	}

	balAfter := k.GetBalance(ctx, tokenId, testCreator)
	if balAfter.Cmp(balBefore) != 0 {
		t.Fatalf("expected balance %s after round-trip, got %s", balBefore, balAfter)
	}

	// Wrapped coin balance should be 0
	senderAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin := bk.GetBalance(ctx, senderAddr, resp.WrappedDenom)
	if !coin.Amount.IsZero() {
		t.Fatalf("expected 0 wrapped coins after round-trip, got %s", coin.Amount.String())
	}
}

func TestUnwrapToken_ZeroAmountRejected(t *testing.T) {
	_, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "UZERO")

	resp, _ := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})

	_, err := srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: resp.WrappedDenom, Amount: "0",
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

// -----------------------------------------------------------------------
// Hardening: Multi-Step Wrap/Unwrap Round-Trip
// -----------------------------------------------------------------------

func TestWrapUnwrap_MultiStepRoundTrip(t *testing.T) {
	k, ctx, srv, bk := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "MSRT")

	bal := k.GetBalance(ctx, tokenId, testCreator)
	if bal.Cmp(big.NewInt(1000000)) != 0 {
		t.Fatalf("expected initial balance 1000000, got %s", bal.String())
	}
	senderAddr, _ := sdk.AccAddressFromBech32(testCreator)

	// Step 1: Wrap 600,000
	resp, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "600000",
	})
	if err != nil {
		t.Fatalf("wrap 600K failed: %v", err)
	}

	// Verify: ZRN-20 = 400,000, sdk.Coin = 600,000
	zrn20Bal := k.GetBalance(ctx, tokenId, testCreator)
	if zrn20Bal.Cmp(big.NewInt(400000)) != 0 {
		t.Fatalf("after wrap 600K: expected ZRN-20 balance 400000, got %s", zrn20Bal.String())
	}
	coinBal := bk.GetBalance(ctx, senderAddr, resp.WrappedDenom)
	if !coinBal.Amount.Equal(sdkmath.NewInt(600000)) {
		t.Fatalf("after wrap 600K: expected sdk.Coin 600000, got %s", coinBal.Amount.String())
	}

	// Step 2: Unwrap 300,000
	_, err = srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: resp.WrappedDenom, Amount: "300000",
	})
	if err != nil {
		t.Fatalf("unwrap 300K failed: %v", err)
	}

	// Verify: ZRN-20 = 700,000, sdk.Coin = 300,000
	zrn20Bal = k.GetBalance(ctx, tokenId, testCreator)
	if zrn20Bal.Cmp(big.NewInt(700000)) != 0 {
		t.Fatalf("after unwrap 300K: expected ZRN-20 balance 700000, got %s", zrn20Bal.String())
	}
	coinBal = bk.GetBalance(ctx, senderAddr, resp.WrappedDenom)
	if !coinBal.Amount.Equal(sdkmath.NewInt(300000)) {
		t.Fatalf("after unwrap 300K: expected sdk.Coin 300000, got %s", coinBal.Amount.String())
	}

	// Step 3: Unwrap remaining 300,000
	_, err = srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: resp.WrappedDenom, Amount: "300000",
	})
	if err != nil {
		t.Fatalf("unwrap remaining 300K failed: %v", err)
	}

	// Verify: ZRN-20 = 1,000,000, sdk.Coin = 0
	zrn20Bal = k.GetBalance(ctx, tokenId, testCreator)
	if zrn20Bal.Cmp(big.NewInt(1000000)) != 0 {
		t.Fatalf("after full unwrap: expected ZRN-20 balance 1000000, got %s", zrn20Bal.String())
	}
	coinBal = bk.GetBalance(ctx, senderAddr, resp.WrappedDenom)
	if !coinBal.Amount.IsZero() {
		t.Fatalf("after full unwrap: expected sdk.Coin 0, got %s", coinBal.Amount.String())
	}
}

// -----------------------------------------------------------------------
// Hardening: Unwrap Without Wrappable Flag
// -----------------------------------------------------------------------

func TestUnwrapToken_AfterWrappableDisabled(t *testing.T) {
	k, ctx, srv, bk := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "UWDF")

	resp, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "500",
	})
	if err != nil {
		t.Fatalf("wrap failed: %v", err)
	}

	// "Governance disables Wrappable" — directly modify token features
	token := k.GetToken(ctx, tokenId)
	token.Features.Wrappable = false
	k.SetToken(ctx, token)

	// Verify wrapping is now blocked
	_, err = srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected wrap to fail after Wrappable disabled")
	}

	// Unwrap should still work — users can always get their tokens back
	_, err = srv.UnwrapToken(ctx, &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: resp.WrappedDenom, Amount: "500",
	})
	if err != nil {
		t.Fatalf("unwrap should succeed even after Wrappable disabled: %v", err)
	}

	zrn20Bal := k.GetBalance(ctx, tokenId, testCreator)
	if zrn20Bal.Cmp(big.NewInt(1000000)) != 0 {
		t.Fatalf("expected full balance 1000000 after unwrap, got %s", zrn20Bal.String())
	}
	senderAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coinBal := bk.GetBalance(ctx, senderAddr, resp.WrappedDenom)
	if !coinBal.Amount.IsZero() {
		t.Fatalf("expected 0 wrapped coins after unwrap, got %s", coinBal.Amount.String())
	}
}

// -----------------------------------------------------------------------
// Hardening: Edge Cases
// -----------------------------------------------------------------------

func TestDelegation_ReceiveTokens_UndelegatedIncreases(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "RCVD", &types.TokenFeatures{})

	_, _ = srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser1, Amount: "200000",
	})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testUser1, TokenId: tokenId, Delegate: testUser2, Amount: "150000",
	})

	undel := k.GetUndelegatedBalance(ctx, tokenId, testUser1)
	if undel.Cmp(big.NewInt(50000)) != 0 {
		t.Fatalf("expected undelegated 50000, got %s", undel.String())
	}

	_, _ = srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser1, Amount: "100000",
	})

	undel = k.GetUndelegatedBalance(ctx, tokenId, testUser1)
	if undel.Cmp(big.NewInt(150000)) != 0 {
		t.Fatalf("expected undelegated 150000 after receiving, got %s", undel.String())
	}
}

func TestWrap_OnlyUndelegatedPortion(t *testing.T) {
	k, ctx, srv, bk := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "WUDL")

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "700000",
	})

	// Wrap 300,000 (exact undelegated amount)
	resp, err := srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "300000",
	})
	if err != nil {
		t.Fatalf("expected wrap of undelegated portion to succeed: %v", err)
	}

	zrn20Bal := k.GetBalance(ctx, tokenId, testCreator)
	if zrn20Bal.Cmp(big.NewInt(700000)) != 0 {
		t.Fatalf("expected ZRN-20 balance 700000, got %s", zrn20Bal.String())
	}
	senderAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coinBal := bk.GetBalance(ctx, senderAddr, resp.WrappedDenom)
	if !coinBal.Amount.Equal(sdkmath.NewInt(300000)) {
		t.Fatalf("expected 300000 wrapped coins, got %s", coinBal.Amount.String())
	}

	// Try to wrap 1 more — must fail (0 undelegated)
	_, err = srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "1",
	})
	if err == nil {
		t.Fatal("expected wrap to fail: 0 undelegated remaining")
	}
}

func TestDelegation_ZeroRevoke_CleansUpStoreKeys(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "ZCLR", &types.TokenFeatures{})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})

	del := k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected delegation 500, got %s", del.String())
	}

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "0",
	})

	del = k.GetDelegation(ctx, tokenId, testCreator, testUser1)
	if del.Sign() != 0 {
		t.Fatalf("expected delegation 0 after revoke, got %s", del.String())
	}

	total := k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Sign() != 0 {
		t.Fatalf("expected total 0 after revoke, got %s", total.String())
	}

	count := 0
	k.IterateDelegationsByDelegator(ctx, tokenId, testCreator, func(delegate string, amount *big.Int) bool {
		count++
		return false
	})
	if count != 0 {
		t.Fatalf("expected 0 delegations after revoke, got %d", count)
	}
}

func TestDelegation_MultipleDelegates_TotalTracking(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "MDTK", &types.TokenFeatures{})

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "100000",
	})
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser2, Amount: "200000",
	})
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser3, Amount: "300000",
	})

	total := k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Cmp(big.NewInt(600000)) != 0 {
		t.Fatalf("expected total 600000, got %s", total.String())
	}

	undel := k.GetUndelegatedBalance(ctx, tokenId, testCreator)
	if undel.Cmp(big.NewInt(400000)) != 0 {
		t.Fatalf("expected undelegated 400000, got %s", undel.String())
	}

	// Revoke one delegate
	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser2, Amount: "0",
	})

	total = k.GetDelegatorTotal(ctx, tokenId, testCreator)
	if total.Cmp(big.NewInt(400000)) != 0 {
		t.Fatalf("expected total 400000 after revoke, got %s", total.String())
	}

	undel = k.GetUndelegatedBalance(ctx, tokenId, testCreator)
	if undel.Cmp(big.NewInt(600000)) != 0 {
		t.Fatalf("expected undelegated 600000 after revoke, got %s", undel.String())
	}
}

// -----------------------------------------------------------------------
// ValidateBasic Tests
// -----------------------------------------------------------------------

func TestMsgDelegatePower_ValidateBasic(t *testing.T) {
	valid := &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: "tok1", Delegate: testUser1, Amount: "100",
	}
	if err := valid.ValidateBasic(); err != nil {
		t.Fatalf("valid msg failed: %v", err)
	}

	selfDel := &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: "tok1", Delegate: testCreator, Amount: "100",
	}
	if err := selfDel.ValidateBasic(); err == nil {
		t.Fatal("expected self-delegation error")
	}

	empty := &types.MsgDelegatePower{}
	if err := empty.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty msg")
	}
}

func TestMsgUndelegatePower_ValidateBasic(t *testing.T) {
	valid := &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: "tok1", Delegate: testUser1, Amount: "100",
	}
	if err := valid.ValidateBasic(); err != nil {
		t.Fatalf("valid msg failed: %v", err)
	}

	selfUndel := &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: "tok1", Delegate: testCreator, Amount: "100",
	}
	if err := selfUndel.ValidateBasic(); err == nil {
		t.Fatal("expected self-delegation error")
	}

	zeroAmt := &types.MsgUndelegatePower{
		Delegator: testCreator, TokenId: "tok1", Delegate: testUser1, Amount: "0",
	}
	if err := zeroAmt.ValidateBasic(); err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestMsgWrapToken_ValidateBasic(t *testing.T) {
	valid := &types.MsgWrapToken{
		Sender: testCreator, TokenId: "tok1", Amount: "100",
	}
	if err := valid.ValidateBasic(); err != nil {
		t.Fatalf("valid msg failed: %v", err)
	}

	empty := &types.MsgWrapToken{}
	if err := empty.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty msg")
	}

	zeroAmt := &types.MsgWrapToken{
		Sender: testCreator, TokenId: "tok1", Amount: "0",
	}
	if err := zeroAmt.ValidateBasic(); err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestMsgUnwrapToken_ValidateBasic(t *testing.T) {
	valid := &types.MsgUnwrapToken{
		Sender: testCreator, WrappedDenom: "zrn20/abc", Amount: "100",
	}
	if err := valid.ValidateBasic(); err != nil {
		t.Fatalf("valid msg failed: %v", err)
	}

	empty := &types.MsgUnwrapToken{}
	if err := empty.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty msg")
	}
}

// -----------------------------------------------------------------------
// Genesis Round-Trip Tests
// -----------------------------------------------------------------------

func TestGenesis_DelegationsAndWraps_RoundTrip(t *testing.T) {
	k, ctx, srv, _ := setupMsgServerWithBank(t)
	tokenId := createWrappableToken(t, srv, ctx, testCreator, "GENWP")

	_, _ = srv.DelegatePower(ctx, &types.MsgDelegatePower{
		Delegator: testCreator, TokenId: tokenId, Delegate: testUser1, Amount: "500",
	})

	_, _ = srv.WrapToken(ctx, &types.MsgWrapToken{
		Sender: testCreator, TokenId: tokenId, Amount: "200",
	})

	// Export
	exportedJSON := k.ExportGenesisJSON(ctx)

	// Verify exported JSON contains delegation and wrap entries
	var g struct {
		DelegationEntries []struct {
			TokenId     string            `json:"token_id"`
			Delegations map[string]string `json:"delegations,omitempty"`
			Totals      map[string]string `json:"totals,omitempty"`
		} `json:"delegation_entries"`
		WrapEntries []struct {
			TokenId      string `json:"token_id"`
			WrappedDenom string `json:"wrapped_denom"`
		} `json:"wrap_entries"`
	}
	if err := json.Unmarshal(exportedJSON, &g); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(g.DelegationEntries) != 1 {
		t.Fatalf("expected 1 delegation entry, got %d", len(g.DelegationEntries))
	}
	if g.DelegationEntries[0].TokenId != tokenId {
		t.Fatalf("expected token ID %s", tokenId)
	}
	if len(g.DelegationEntries[0].Delegations) != 1 {
		t.Fatalf("expected 1 delegation, got %d", len(g.DelegationEntries[0].Delegations))
	}

	if len(g.WrapEntries) != 1 {
		t.Fatalf("expected 1 wrap entry, got %d", len(g.WrapEntries))
	}
	if g.WrapEntries[0].TokenId != tokenId {
		t.Fatalf("expected token ID %s", tokenId)
	}

	// Import into fresh keeper
	k2, ctx2, _ := setupKeeperWithBank(t)
	k2.InitGenesisTokens(ctx2, exportedJSON)
	k2.InitGenesisDelegations(ctx2, exportedJSON)
	k2.InitGenesisWraps(ctx2, exportedJSON)

	// Verify delegation survived
	del := k2.GetDelegation(ctx2, tokenId, testCreator, testUser1)
	if del.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected delegation 500 after import, got %s", del.String())
	}

	total := k2.GetDelegatorTotal(ctx2, tokenId, testCreator)
	if total.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected total 500 after import, got %s", total.String())
	}

	// Verify wrap record survived
	wrappedDenom := k2.GetWrappedDenom(ctx2, tokenId)
	expectedDenom := "zrn20/" + tokenId
	if wrappedDenom != expectedDenom {
		t.Fatalf("expected denom %s, got %s", expectedDenom, wrappedDenom)
	}

	resolvedId := k2.GetTokenIdByWrappedDenom(ctx2, expectedDenom)
	if resolvedId != tokenId {
		t.Fatalf("expected reverse lookup %s, got %s", tokenId, resolvedId)
	}
}

func TestGenesis_EmptyDelegationsAndWraps(t *testing.T) {
	k, ctx, srv, _ := setupMsgServerWithBank(t)
	_ = createTestToken(t, srv, ctx, testCreator, "EMPTY", &types.TokenFeatures{})

	exportedJSON := k.ExportGenesisJSON(ctx)

	var g struct {
		DelegationEntries []json.RawMessage `json:"delegation_entries"`
		WrapEntries       []json.RawMessage `json:"wrap_entries"`
	}
	if err := json.Unmarshal(exportedJSON, &g); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(g.DelegationEntries) != 0 {
		t.Fatalf("expected 0 delegation entries, got %d", len(g.DelegationEntries))
	}
	if len(g.WrapEntries) != 0 {
		t.Fatalf("expected 0 wrap entries, got %d", len(g.WrapEntries))
	}
}
