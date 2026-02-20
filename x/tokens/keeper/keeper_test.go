package keeper_test

import (
	"bytes"
	"math/big"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/tokens/keeper"
	"github.com/zerone-chain/zerone/x/tokens/types"
)

var (
	testAuthority string
	testCreator   string
	testUser1     string
	testUser2     string
	testUser3     string
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")

	testAuthority = sdk.AccAddress(bytes.Repeat([]byte{0x01}, 20)).String()
	testCreator = sdk.AccAddress(bytes.Repeat([]byte{0x02}, 20)).String()
	testUser1 = sdk.AccAddress(bytes.Repeat([]byte{0x03}, 20)).String()
	testUser2 = sdk.AccAddress(bytes.Repeat([]byte{0x04}, 20)).String()
	testUser3 = sdk.AccAddress(bytes.Repeat([]byte{0x05}, 20)).String()
}

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
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

	k := keeper.NewKeeper(cdc, storeService, nil, testAuthority)
	return k, ctx
}

func setupMsgServer(t *testing.T) (*keeper.Keeper, sdk.Context, types.MsgServer) {
	t.Helper()
	k, ctx := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	return &k, ctx, srv
}

// createTestToken is a helper that creates a token and returns its ID.
func createTestToken(t *testing.T, srv types.MsgServer, ctx sdk.Context, creator, symbol string, features *types.TokenFeatures) string {
	t.Helper()
	resp, err := srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator:       creator,
		Name:          "Test " + symbol,
		Symbol:        symbol,
		Decimals:      18,
		InitialSupply: "1000000",
		MaxSupply:     "10000000",
		Features:      features,
	})
	if err != nil {
		t.Fatalf("createTestToken(%s) failed: %v", symbol, err)
	}
	return resp.TokenId
}

// allFeatures returns a TokenFeatures with all features enabled.
func allFeatures() *types.TokenFeatures {
	return &types.TokenFeatures{
		Mintable:  true,
		Burnable:  true,
		Pausable:  true,
		Wrappable: true,
	}
}

// -----------------------------------------------------------------------
// Keeper basics
// -----------------------------------------------------------------------

func TestKeeperCreation(t *testing.T) {
	k, _ := setupKeeper(t)
	if k.GetAuthority() != testAuthority {
		t.Fatalf("expected authority %s, got %s", testAuthority, k.GetAuthority())
	}
}

func TestParamsGetSet(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := k.GetParams(ctx)
	if params.EmissionEpochBlocks != 0 {
		t.Fatalf("expected zeroed EmissionEpochBlocks, got %d", params.EmissionEpochBlocks)
	}

	params.EmissionEpochBlocks = 12345
	k.SetParams(ctx, params)
	got := k.GetParams(ctx)
	if got.EmissionEpochBlocks != 12345 {
		t.Fatalf("expected EmissionEpochBlocks 12345, got %d", got.EmissionEpochBlocks)
	}
}

func TestInitExportGenesis(t *testing.T) {
	k, ctx := setupKeeper(t)

	genState := types.DefaultGenesis()
	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)
	if exported.Params == nil {
		t.Fatal("exported params is nil")
	}
}

func TestMsgServerImpl(t *testing.T) {
	k, _ := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	if srv == nil {
		t.Fatal("NewMsgServerImpl returned nil")
	}
}

func TestQueryServerImpl(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewQueryServerImpl(k)
	resp, err := srv.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("Params query failed: %v", err)
	}
	if resp == nil {
		t.Fatal("Params response is nil")
	}
}

// -----------------------------------------------------------------------
// State CRUD
// -----------------------------------------------------------------------

func TestTokenSetGetDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	token := &types.TokenDefinition{
		Id:          "abc123",
		Creator:     testCreator,
		Name:        "Test Token",
		Symbol:      "TST",
		Decimals:    18,
		TotalSupply: "1000",
		MaxSupply:   "10000",
		Features:    allFeatures(),
	}

	// Set and get
	k.SetToken(ctx, token)
	got := k.GetToken(ctx, "abc123")
	if got == nil {
		t.Fatal("expected token, got nil")
	}
	if got.Symbol != "TST" {
		t.Fatalf("expected symbol TST, got %s", got.Symbol)
	}

	// Get by symbol
	got2 := k.GetTokenBySymbol(ctx, "TST")
	if got2 == nil {
		t.Fatal("expected token by symbol, got nil")
	}
	if got2.Id != "abc123" {
		t.Fatalf("expected id abc123, got %s", got2.Id)
	}

	// Delete
	k.DeleteToken(ctx, token)
	if k.GetToken(ctx, "abc123") != nil {
		t.Fatal("expected nil after delete")
	}
	if k.GetTokenBySymbol(ctx, "TST") != nil {
		t.Fatal("expected nil symbol lookup after delete")
	}
}

func TestTokenNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	if k.GetToken(ctx, "nonexistent") != nil {
		t.Fatal("expected nil for nonexistent token")
	}
	if k.GetTokenBySymbol(ctx, "NOPE") != nil {
		t.Fatal("expected nil for nonexistent symbol")
	}
}

func TestIterateTokens(t *testing.T) {
	k, ctx := setupKeeper(t)

	for _, sym := range []string{"AAA", "BBB", "CCC"} {
		k.SetToken(ctx, &types.TokenDefinition{
			Id:          sym + "_id",
			Symbol:      sym,
			Name:        sym,
			Creator:     testCreator,
			TotalSupply: "0",
			MaxSupply:   "0",
			Features:    &types.TokenFeatures{},
		})
	}

	count := 0
	k.IterateTokens(ctx, func(token *types.TokenDefinition) bool {
		count++
		return false
	})
	if count != 3 {
		t.Fatalf("expected 3 tokens, iterated %d", count)
	}
}

func TestBalanceSetGetZeroDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default is zero
	bal := k.GetBalance(ctx, "tok1", testUser1)
	if bal.Sign() != 0 {
		t.Fatalf("expected zero balance, got %s", bal.String())
	}

	// Set and get
	k.SetBalance(ctx, "tok1", testUser1, big.NewInt(500))
	bal = k.GetBalance(ctx, "tok1", testUser1)
	if bal.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected 500, got %s", bal.String())
	}

	// Set to zero deletes
	k.SetBalance(ctx, "tok1", testUser1, big.NewInt(0))
	bal = k.GetBalance(ctx, "tok1", testUser1)
	if bal.Sign() != 0 {
		t.Fatalf("expected zero after delete, got %s", bal.String())
	}
}

func TestIterateBalancesByToken(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetBalance(ctx, "tok1", testUser1, big.NewInt(100))
	k.SetBalance(ctx, "tok1", testUser2, big.NewInt(200))
	k.SetBalance(ctx, "tok2", testUser1, big.NewInt(999))

	count := 0
	total := new(big.Int)
	k.IterateBalancesByToken(ctx, "tok1", func(ownerAddr string, balance *big.Int) bool {
		count++
		total.Add(total, balance)
		return false
	})
	if count != 2 {
		t.Fatalf("expected 2 balances for tok1, got %d", count)
	}
	if total.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected total 300, got %s", total.String())
	}
}

func TestAllowanceSetGetZeroDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default is zero
	al := k.GetAllowance(ctx, "tok1", testUser1, testUser2)
	if al.Sign() != 0 {
		t.Fatalf("expected zero allowance, got %s", al.String())
	}

	// Set and get
	k.SetAllowance(ctx, "tok1", testUser1, testUser2, big.NewInt(100))
	al = k.GetAllowance(ctx, "tok1", testUser1, testUser2)
	if al.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("expected 100, got %s", al.String())
	}

	// Set to zero deletes
	k.SetAllowance(ctx, "tok1", testUser1, testUser2, big.NewInt(0))
	al = k.GetAllowance(ctx, "tok1", testUser1, testUser2)
	if al.Sign() != 0 {
		t.Fatalf("expected zero after delete, got %s", al.String())
	}
}

func TestGenerateTokenID(t *testing.T) {
	id1 := keeper.GenerateTokenID(testCreator, "TST", 100)
	id2 := keeper.GenerateTokenID(testCreator, "TST", 100)
	id3 := keeper.GenerateTokenID(testCreator, "TST", 101)

	if id1 != id2 {
		t.Fatal("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Fatal("different block height should produce different ID")
	}
	if len(id1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(id1))
	}
}

func TestIsValidSymbol(t *testing.T) {
	// Valid symbols tested via CreateToken
	// Invalid symbols tested via CreateToken error paths
}

// -----------------------------------------------------------------------
// CreateToken
// -----------------------------------------------------------------------

func TestCreateTokenSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	resp, err := srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator:       testCreator,
		Name:          "My Token",
		Symbol:        "MYT",
		Decimals:      18,
		InitialSupply: "1000",
		MaxSupply:     "10000",
		Features:      allFeatures(),
	})
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if resp.TokenId == "" {
		t.Fatal("expected non-empty token ID")
	}

	// Verify token stored
	token := k.GetToken(ctx, resp.TokenId)
	if token == nil {
		t.Fatal("token not found in store")
	}
	if token.Symbol != "MYT" {
		t.Fatalf("expected symbol MYT, got %s", token.Symbol)
	}
	if token.TotalSupply != "1000" {
		t.Fatalf("expected total supply 1000, got %s", token.TotalSupply)
	}
	if token.Paused {
		t.Fatal("token should not be paused")
	}

	// Verify creator balance
	bal := k.GetBalance(ctx, resp.TokenId, testCreator)
	if bal.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("expected creator balance 1000, got %s", bal.String())
	}
}

func TestCreateTokenDuplicateSymbol(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator: testCreator, Name: "First", Symbol: "DUP",
		Features: &types.TokenFeatures{},
	})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator: testUser1, Name: "Second", Symbol: "DUP",
		Features: &types.TokenFeatures{},
	})
	if err == nil {
		t.Fatal("expected error for duplicate symbol")
	}
}

func TestCreateTokenInvalidSymbol(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	cases := []string{"", "abc", "TOO LONG SYMBOL ABCDEF", "A!B", "a1"}
	for _, sym := range cases {
		_, err := srv.CreateToken(ctx, &types.MsgCreateToken{
			Creator: testCreator, Name: "Test", Symbol: sym,
			Features: &types.TokenFeatures{},
		})
		if err == nil {
			t.Fatalf("expected error for symbol %q", sym)
		}
	}
}

func TestCreateTokenInitialExceedsMax(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator:       testCreator,
		Name:          "Over",
		Symbol:        "OVR",
		InitialSupply: "100",
		MaxSupply:     "50",
		Features:      &types.TokenFeatures{},
	})
	if err == nil {
		t.Fatal("expected error when initial > max")
	}
}

func TestCreateTokenZeroInitialSupply(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	resp, err := srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator:  testCreator,
		Name:     "Zero",
		Symbol:   "ZRO",
		Features: &types.TokenFeatures{},
	})
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	bal := k.GetBalance(ctx, resp.TokenId, testCreator)
	if bal.Sign() != 0 {
		t.Fatalf("expected zero balance, got %s", bal.String())
	}
}

func TestCreateTokenUnlimitedMax(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	resp, err := srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator:       testCreator,
		Name:          "Unlimited",
		Symbol:        "UNL",
		InitialSupply: "999999999999999999999999999",
		MaxSupply:     "0",
		Features:      &types.TokenFeatures{},
	})
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}
	if resp.TokenId == "" {
		t.Fatal("expected token ID")
	}
}

// -----------------------------------------------------------------------
// MintToken
// -----------------------------------------------------------------------

func TestMintTokenSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "MINT", allFeatures())

	_, err := srv.MintToken(ctx, &types.MsgMintToken{
		Authority: testCreator,
		TokenId:   tokenId,
		To:        testUser1,
		Amount:    "500",
	})
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}

	bal := k.GetBalance(ctx, tokenId, testUser1)
	if bal.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected 500, got %s", bal.String())
	}

	token := k.GetToken(ctx, tokenId)
	if token.TotalSupply != "1000500" {
		t.Fatalf("expected total supply 1000500, got %s", token.TotalSupply)
	}
}

func TestMintTokenByGovernance(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "MGOV", allFeatures())

	_, err := srv.MintToken(ctx, &types.MsgMintToken{
		Authority: testAuthority,
		TokenId:   tokenId,
		To:        testUser1,
		Amount:    "100",
	})
	if err != nil {
		t.Fatalf("MintToken by governance failed: %v", err)
	}

	bal := k.GetBalance(ctx, tokenId, testUser1)
	if bal.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("expected 100, got %s", bal.String())
	}
}

func TestMintTokenUnauthorized(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "MUNA", allFeatures())

	_, err := srv.MintToken(ctx, &types.MsgMintToken{
		Authority: testUser1,
		TokenId:   tokenId,
		To:        testUser2,
		Amount:    "100",
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestMintTokenNotMintable(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "NMNT", &types.TokenFeatures{
		Burnable: true,
	})

	_, err := srv.MintToken(ctx, &types.MsgMintToken{
		Authority: testCreator,
		TokenId:   tokenId,
		To:        testUser1,
		Amount:    "100",
	})
	if err == nil {
		t.Fatal("expected not mintable error")
	}
}

func TestMintTokenExceedsMax(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "MMAX", allFeatures())

	// Token has initial=1000000, max=10000000. Try to mint 9500000 -> would be 10500000 > 10000000
	_, err := srv.MintToken(ctx, &types.MsgMintToken{
		Authority: testCreator,
		TokenId:   tokenId,
		To:        testUser1,
		Amount:    "9500000",
	})
	if err == nil {
		t.Fatal("expected supply exceeded error")
	}
}

func TestMintTokenPaused(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "MPSD", allFeatures())

	// Pause first
	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.MintToken(ctx, &types.MsgMintToken{
		Authority: testCreator,
		TokenId:   tokenId,
		To:        testUser1,
		Amount:    "100",
	})
	if err == nil {
		t.Fatal("expected paused error")
	}
}

// -----------------------------------------------------------------------
// BurnToken
// -----------------------------------------------------------------------

func TestBurnTokenSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "BURN", allFeatures())

	_, err := srv.BurnToken(ctx, &types.MsgBurnToken{
		Burner:  testCreator,
		TokenId: tokenId,
		Amount:  "300",
	})
	if err != nil {
		t.Fatalf("BurnToken failed: %v", err)
	}

	bal := k.GetBalance(ctx, tokenId, testCreator)
	if bal.Cmp(big.NewInt(999700)) != 0 {
		t.Fatalf("expected 999700, got %s", bal.String())
	}

	token := k.GetToken(ctx, tokenId)
	if token.TotalSupply != "999700" {
		t.Fatalf("expected total supply 999700, got %s", token.TotalSupply)
	}
}

func TestBurnTokenInsufficientBalance(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "BINS", allFeatures())

	_, err := srv.BurnToken(ctx, &types.MsgBurnToken{
		Burner:  testUser1, // user1 has no tokens
		TokenId: tokenId,
		Amount:  "1",
	})
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}
}

func TestBurnTokenNotBurnable(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "NBRN", &types.TokenFeatures{
		Mintable: true,
	})

	_, err := srv.BurnToken(ctx, &types.MsgBurnToken{
		Burner:  testCreator,
		TokenId: tokenId,
		Amount:  "1",
	})
	if err == nil {
		t.Fatal("expected not burnable error")
	}
}

func TestBurnTokenPaused(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "BPSD", allFeatures())

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.BurnToken(ctx, &types.MsgBurnToken{
		Burner:  testCreator,
		TokenId: tokenId,
		Amount:  "1",
	})
	if err == nil {
		t.Fatal("expected paused error")
	}
}

// -----------------------------------------------------------------------
// TransferToken
// -----------------------------------------------------------------------

func TestTransferTokenSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "XFER", allFeatures())

	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender:  testCreator,
		TokenId: tokenId,
		To:      testUser1,
		Amount:  "400",
	})
	if err != nil {
		t.Fatalf("TransferToken failed: %v", err)
	}

	senderBal := k.GetBalance(ctx, tokenId, testCreator)
	if senderBal.Cmp(big.NewInt(999600)) != 0 {
		t.Fatalf("expected sender 999600, got %s", senderBal.String())
	}
	recipientBal := k.GetBalance(ctx, tokenId, testUser1)
	if recipientBal.Cmp(big.NewInt(400)) != 0 {
		t.Fatalf("expected recipient 400, got %s", recipientBal.String())
	}
}

func TestTransferTokenInsufficientBalance(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "XINS", allFeatures())

	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender:  testCreator,
		TokenId: tokenId,
		To:      testUser1,
		Amount:  "2000000", // more than initial supply
	})
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}
}

func TestTransferTokenSelfTransfer(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "XSLF", allFeatures())

	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender:  testCreator,
		TokenId: tokenId,
		To:      testCreator,
		Amount:  "1",
	})
	if err == nil {
		t.Fatal("expected self-transfer error")
	}
}

func TestTransferTokenPaused(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "XPSD", allFeatures())

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender:  testCreator,
		TokenId: tokenId,
		To:      testUser1,
		Amount:  "1",
	})
	if err == nil {
		t.Fatal("expected paused error")
	}
}

func TestTransferTokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender:  testCreator,
		TokenId: "nonexistent",
		To:      testUser1,
		Amount:  "1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

// -----------------------------------------------------------------------
// ApproveToken
// -----------------------------------------------------------------------

func TestApproveTokenSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "APRV", allFeatures())

	_, err := srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner:   testCreator,
		TokenId: tokenId,
		Spender: testUser1,
		Amount:  "500",
	})
	if err != nil {
		t.Fatalf("ApproveToken failed: %v", err)
	}

	al := k.GetAllowance(ctx, tokenId, testCreator, testUser1)
	if al.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected allowance 500, got %s", al.String())
	}
}

func TestApproveTokenRevoke(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "AREV", allFeatures())

	// Set allowance
	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "500",
	})

	// Revoke (set to 0)
	_, err := srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "0",
	})
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	al := k.GetAllowance(ctx, tokenId, testCreator, testUser1)
	if al.Sign() != 0 {
		t.Fatalf("expected zero allowance after revoke, got %s", al.String())
	}
}

func TestApproveTokenSelfApprove(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "ASLF", allFeatures())

	_, err := srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner:   testCreator,
		TokenId: tokenId,
		Spender: testCreator,
		Amount:  "100",
	})
	if err == nil {
		t.Fatal("expected self-approve error")
	}
}

// -----------------------------------------------------------------------
// TransferFrom
// -----------------------------------------------------------------------

func TestTransferFromSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TFOK", allFeatures())

	// Approve user1 as spender
	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "500",
	})

	// User1 transfers from creator to user2
	_, err := srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1,
		TokenId: tokenId,
		From:    testCreator,
		To:      testUser2,
		Amount:  "200",
	})
	if err != nil {
		t.Fatalf("TransferFrom failed: %v", err)
	}

	// Check balances
	creatorBal := k.GetBalance(ctx, tokenId, testCreator)
	if creatorBal.Cmp(big.NewInt(999800)) != 0 {
		t.Fatalf("expected creator 999800, got %s", creatorBal.String())
	}
	user2Bal := k.GetBalance(ctx, tokenId, testUser2)
	if user2Bal.Cmp(big.NewInt(200)) != 0 {
		t.Fatalf("expected user2 200, got %s", user2Bal.String())
	}

	// Check remaining allowance
	al := k.GetAllowance(ctx, tokenId, testCreator, testUser1)
	if al.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected remaining allowance 300, got %s", al.String())
	}
}

func TestTransferFromExceedsAllowance(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TFEA", allFeatures())

	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "100",
	})

	_, err := srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1, TokenId: tokenId, From: testCreator, To: testUser2, Amount: "200",
	})
	if err == nil {
		t.Fatal("expected allowance exceeded error")
	}
}

func TestTransferFromInsufficientBalance(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TFIB", allFeatures())

	// Give user1 a huge allowance but they have no tokens
	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testUser1, TokenId: tokenId, Spender: testUser2, Amount: "999999",
	})

	_, err := srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser2, TokenId: tokenId, From: testUser1, To: testUser3, Amount: "1",
	})
	if err == nil {
		t.Fatal("expected insufficient balance error")
	}
}

func TestTransferFromPaused(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TFPS", allFeatures())

	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "500",
	})

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1, TokenId: tokenId, From: testCreator, To: testUser2, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected paused error")
	}
}

func TestTransferFromAllowanceDeduction(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "TFAD", allFeatures())

	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "1000",
	})

	// Transfer 600
	_, _ = srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1, TokenId: tokenId, From: testCreator, To: testUser2, Amount: "600",
	})

	// Remaining should be 400
	al := k.GetAllowance(ctx, tokenId, testCreator, testUser1)
	if al.Cmp(big.NewInt(400)) != 0 {
		t.Fatalf("expected 400 remaining, got %s", al.String())
	}

	// Transfer 400 more — should succeed and allowance should be zero (deleted)
	_, err := srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1, TokenId: tokenId, From: testCreator, To: testUser2, Amount: "400",
	})
	if err != nil {
		t.Fatalf("second transfer failed: %v", err)
	}

	al = k.GetAllowance(ctx, tokenId, testCreator, testUser1)
	if al.Sign() != 0 {
		t.Fatalf("expected zero allowance, got %s", al.String())
	}
}

// -----------------------------------------------------------------------
// PauseToken / UnpauseToken
// -----------------------------------------------------------------------

func TestPauseTokenCreatorSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "PCRT", allFeatures())

	_, err := srv.PauseToken(ctx, &types.MsgPauseToken{
		Authority: testCreator,
		TokenId:   tokenId,
	})
	if err != nil {
		t.Fatalf("PauseToken failed: %v", err)
	}

	token := k.GetToken(ctx, tokenId)
	if !token.Paused {
		t.Fatal("expected token to be paused")
	}
}

func TestPauseTokenGovernanceSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "PGOV", allFeatures())

	_, err := srv.PauseToken(ctx, &types.MsgPauseToken{
		Authority: testAuthority,
		TokenId:   tokenId,
	})
	if err != nil {
		t.Fatalf("PauseToken by governance failed: %v", err)
	}

	token := k.GetToken(ctx, tokenId)
	if !token.Paused {
		t.Fatal("expected token to be paused")
	}
}

func TestPauseTokenAlreadyPaused(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "PALR", allFeatures())

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})
	if err == nil {
		t.Fatal("expected already paused error")
	}
}

func TestPauseTokenNotPausable(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "PNOP", &types.TokenFeatures{
		Mintable: true,
		Burnable: true,
	})

	_, err := srv.PauseToken(ctx, &types.MsgPauseToken{
		Authority: testCreator,
		TokenId:   tokenId,
	})
	if err == nil {
		t.Fatal("expected not pausable error")
	}
}

func TestPauseTokenUnauthorized(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "PUNA", allFeatures())

	_, err := srv.PauseToken(ctx, &types.MsgPauseToken{
		Authority: testUser1,
		TokenId:   tokenId,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestUnpauseTokenSuccess(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "UPOK", allFeatures())

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})
	_, err := srv.UnpauseToken(ctx, &types.MsgUnpauseToken{Authority: testCreator, TokenId: tokenId})
	if err != nil {
		t.Fatalf("UnpauseToken failed: %v", err)
	}

	token := k.GetToken(ctx, tokenId)
	if token.Paused {
		t.Fatal("expected token to be unpaused")
	}
}

func TestUnpauseTokenNotPaused(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "UPNP", allFeatures())

	_, err := srv.UnpauseToken(ctx, &types.MsgUnpauseToken{Authority: testCreator, TokenId: tokenId})
	if err == nil {
		t.Fatal("expected not paused error")
	}
}

func TestUnpauseTokenUnauthorized(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)
	tokenId := createTestToken(t, srv, ctx, testCreator, "UPUA", allFeatures())

	_, _ = srv.PauseToken(ctx, &types.MsgPauseToken{Authority: testCreator, TokenId: tokenId})

	_, err := srv.UnpauseToken(ctx, &types.MsgUnpauseToken{Authority: testUser1, TokenId: tokenId})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// -----------------------------------------------------------------------
// Genesis round-trip
// -----------------------------------------------------------------------

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	// Create a token with some state
	tokenId := createTestToken(t, srv, ctx, testCreator, "GEN", allFeatures())

	// Transfer some tokens
	_, _ = srv.TransferToken(ctx, &types.MsgTransferToken{
		Sender: testCreator, TokenId: tokenId, To: testUser1, Amount: "100",
	})

	// Set an allowance
	_, _ = srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: tokenId, Spender: testUser1, Amount: "50",
	})

	// Export
	exported := k.ExportGenesisJSON(ctx)
	if exported == nil {
		t.Fatal("exported genesis is nil")
	}

	// Create a new keeper and import
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesisTokens(ctx2, exported)

	// Verify token restored
	token := k2.GetToken(ctx2, tokenId)
	if token == nil {
		t.Fatal("token not found after genesis import")
	}
	if token.Symbol != "GEN" {
		t.Fatalf("expected symbol GEN, got %s", token.Symbol)
	}

	// Verify balances
	creatorBal := k2.GetBalance(ctx2, tokenId, testCreator)
	if creatorBal.Cmp(big.NewInt(999900)) != 0 {
		t.Fatalf("expected creator balance 999900, got %s", creatorBal.String())
	}
	user1Bal := k2.GetBalance(ctx2, tokenId, testUser1)
	if user1Bal.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("expected user1 balance 100, got %s", user1Bal.String())
	}

	// Verify allowance
	al := k2.GetAllowance(ctx2, tokenId, testCreator, testUser1)
	if al.Cmp(big.NewInt(50)) != 0 {
		t.Fatalf("expected allowance 50, got %s", al.String())
	}
}

func TestGenesisEmptyState(t *testing.T) {
	k, ctx := setupKeeper(t)

	exported := k.ExportGenesisJSON(ctx)
	if exported == nil {
		t.Fatal("exported genesis is nil")
	}

	k2, ctx2 := setupKeeper(t)
	k2.InitGenesisTokens(ctx2, exported)

	count := 0
	k2.IterateTokens(ctx2, func(token *types.TokenDefinition) bool {
		count++
		return false
	})
	if count != 0 {
		t.Fatalf("expected 0 tokens in empty genesis, got %d", count)
	}
}

func TestGenesisRoundTripWithEmissions(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	// Create a token and an emission period
	_ = createTestToken(t, srv, ctx, testCreator, "GENA", allFeatures())

	_, err := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})
	if err != nil {
		t.Fatalf("CreateEmissionPeriod failed: %v", err)
	}

	// Export
	exported := k.ExportGenesisJSON(ctx)

	// Import into fresh keeper
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesisTokens(ctx2, exported)
	k2.InitGenesisEmissions(ctx2, exported)

	// Verify emission period survived
	count := 0
	k2.IterateEmissionPeriods(ctx2, func(emission *types.EmissionPeriod) bool {
		count++
		if !emission.Active {
			t.Fatal("expected active emission")
		}
		if emission.AmountPerBlock != "1000" {
			t.Fatalf("expected amount_per_block 1000, got %s", emission.AmountPerBlock)
		}
		return false
	})
	if count != 1 {
		t.Fatalf("expected 1 emission period, got %d", count)
	}
}

// -----------------------------------------------------------------------
// ValidateBasic
// -----------------------------------------------------------------------

func TestValidateBasicCreateToken(t *testing.T) {
	msg := &types.MsgCreateToken{Creator: testCreator, Name: "T", Symbol: "T"}
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}

	msg2 := &types.MsgCreateToken{Creator: "", Name: "T", Symbol: "T"}
	if err := msg2.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty creator")
	}
}

func TestValidateBasicTransferToken(t *testing.T) {
	msg := &types.MsgTransferToken{
		Sender: testCreator, TokenId: "tok1", To: testUser1, Amount: "100",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	// Self-transfer
	msg2 := &types.MsgTransferToken{
		Sender: testCreator, TokenId: "tok1", To: testCreator, Amount: "100",
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Fatal("expected error for self-transfer")
	}
}

func TestValidateBasicApproveToken(t *testing.T) {
	msg := &types.MsgApproveToken{
		Owner: testCreator, TokenId: "tok1", Spender: testUser1, Amount: "0",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("expected valid (zero = revoke): %v", err)
	}

	msg2 := &types.MsgApproveToken{
		Owner: testCreator, TokenId: "tok1", Spender: testUser1, Amount: "-1",
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Fatal("expected error for negative amount")
	}
}

// -----------------------------------------------------------------------
// Edge cases
// -----------------------------------------------------------------------

func TestCreateTokenDefaultFeatures(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	resp, err := srv.CreateToken(ctx, &types.MsgCreateToken{
		Creator:  testCreator,
		Name:     "No Features",
		Symbol:   "NOFT",
		Features: nil, // nil features
	})
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	token := k.GetToken(ctx, resp.TokenId)
	if token.Features == nil {
		t.Fatal("expected default features, got nil")
	}
	if token.Features.Mintable || token.Features.Burnable || token.Features.Pausable {
		t.Fatal("expected all features false by default")
	}
}

func TestMintTokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.MintToken(ctx, &types.MsgMintToken{
		Authority: testCreator, TokenId: "nonexistent", To: testUser1, Amount: "1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestBurnTokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.BurnToken(ctx, &types.MsgBurnToken{
		Burner: testCreator, TokenId: "nonexistent", Amount: "1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestApproveTokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.ApproveToken(ctx, &types.MsgApproveToken{
		Owner: testCreator, TokenId: "nonexistent", Spender: testUser1, Amount: "100",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestTransferFromNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.TransferFrom(ctx, &types.MsgTransferFrom{
		Spender: testUser1, TokenId: "nonexistent", From: testCreator, To: testUser2, Amount: "1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestPauseTokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.PauseToken(ctx, &types.MsgPauseToken{
		Authority: testCreator, TokenId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestUnpauseTokenNotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.UnpauseToken(ctx, &types.MsgUnpauseToken{
		Authority: testCreator, TokenId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}
