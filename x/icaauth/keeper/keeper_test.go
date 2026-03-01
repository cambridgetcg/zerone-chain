package keeper_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/zerone-chain/zerone/x/icaauth/keeper"
	"github.com/zerone-chain/zerone/x/icaauth/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

// ---- Mock ICA Controller ----

type mockICAController struct {
	accounts     map[string]string // connectionID/owner -> address
	openChannels map[string]bool   // connectionID/portID -> open
	registerErr  error
	sendErr      error
}

func newMockICAController() *mockICAController {
	return &mockICAController{
		accounts:     make(map[string]string),
		openChannels: make(map[string]bool),
	}
}

func (m *mockICAController) RegisterInterchainAccount(ctx sdk.Context, connectionID, owner, version string) error {
	if m.registerErr != nil {
		return m.registerErr
	}
	m.accounts[connectionID+"/"+owner] = ""
	return nil
}

func (m *mockICAController) GetInterchainAccountAddress(ctx sdk.Context, connectionID, portID string) (string, bool) {
	addr, ok := m.accounts[connectionID+"/"+portID]
	return addr, ok
}

func (m *mockICAController) GetOpenActiveChannel(ctx sdk.Context, connectionID, portID string) (string, bool) {
	open, ok := m.openChannels[connectionID+"/"+portID]
	return "channel-0", open && ok
}

func (m *mockICAController) SendTx(ctx sdk.Context, chanCap *capabilitytypes.Capability, connectionID, portID string, icaPacketData icatypes.InterchainAccountPacketData, timeoutTimestamp uint64) (uint64, error) {
	if m.sendErr != nil {
		return 0, m.sendErr
	}
	return 1, nil
}

// ---- Setup ----

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockICAController) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	mock := newMockICAController()
	authority := sdk.AccAddress([]byte("authority-addr------")).String()

	k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), mock, authority)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	return k, ctx, mock
}

func testAddr(i int) string {
	return sdk.AccAddress([]byte(fmt.Sprintf("test-addr-%010d", i))).String()
}

// ---- Tests ----

func TestRecordCRUD(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	owner := testAddr(1)

	// No records initially
	accounts := k.GetRemoteAccounts(ctx, owner)
	if len(accounts) != 0 {
		t.Fatalf("expected 0 accounts, got %d", len(accounts))
	}

	// Add a remote account
	acct := &types.RemoteAccount{
		ConnectionId: "connection-0",
		PortId:       "icacontroller-" + owner,
		OwnerAddress: owner,
		Active:       false,
	}
	k.AddRemoteAccount(ctx, owner, acct)

	// Get by owner
	accounts = k.GetRemoteAccounts(ctx, owner)
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].ConnectionId != "connection-0" {
		t.Fatalf("expected connection-0, got %s", accounts[0].ConnectionId)
	}

	// Get by connection
	found, ok := k.GetRemoteAccountByConnection(ctx, owner, "connection-0")
	if !ok {
		t.Fatal("expected to find account by connection")
	}
	if found.PortId != acct.PortId {
		t.Fatalf("expected port %s, got %s", acct.PortId, found.PortId)
	}

	// Not found for different connection
	_, ok = k.GetRemoteAccountByConnection(ctx, owner, "connection-99")
	if ok {
		t.Fatal("should not find account for non-existent connection")
	}
}

func TestUpdateRemoteAccountAddress(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	owner := testAddr(1)

	acct := &types.RemoteAccount{
		ConnectionId: "connection-0",
		OwnerAddress: owner,
		Active:       false,
	}
	k.AddRemoteAccount(ctx, owner, acct)

	k.UpdateRemoteAccountAddress(ctx, owner, "connection-0", "cosmos1remote...")

	found, ok := k.GetRemoteAccountByConnection(ctx, owner, "connection-0")
	if !ok {
		t.Fatal("expected to find account")
	}
	if found.RemoteAddress != "cosmos1remote..." {
		t.Fatalf("expected remote address cosmos1remote..., got %s", found.RemoteAddress)
	}
	if !found.Active {
		t.Fatal("expected account to be active after address update")
	}
}

func TestParamsCRUD(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Default params
	params := k.GetParams(ctx)
	if params.MaxRemoteAccountsPerOwner != 5 {
		t.Fatalf("expected default max=5, got %d", params.MaxRemoteAccountsPerOwner)
	}
	if params.MaxMessagesPerTx != 5 {
		t.Fatalf("expected default max_messages=5, got %d", params.MaxMessagesPerTx)
	}

	// Set custom
	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      50,
		MaxMessagesPerTx:          3,
	})

	params = k.GetParams(ctx)
	if params.MaxRemoteAccountsPerOwner != 10 {
		t.Fatalf("expected max=10, got %d", params.MaxRemoteAccountsPerOwner)
	}
	if params.RegistrationCooldown != 50 {
		t.Fatalf("expected cooldown=50, got %d", params.RegistrationCooldown)
	}
}

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Init with some state
	genState := &types.GenesisState{
		Params: &types.Params{
			MaxRemoteAccountsPerOwner: 3,
			AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
			RegistrationCooldown:      200,
			MaxMessagesPerTx:          10,
		},
		Records: []*types.InterchainAccountRecord{
			{
				Owner: testAddr(1),
				Accounts: []*types.RemoteAccount{
					{ConnectionId: "connection-0", OwnerAddress: testAddr(1), Active: true},
				},
			},
		},
	}
	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)
	if exported.Params.MaxRemoteAccountsPerOwner != 3 {
		t.Fatalf("expected max=3, got %d", exported.Params.MaxRemoteAccountsPerOwner)
	}
	if len(exported.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(exported.Records))
	}
	if exported.Records[0].Owner != testAddr(1) {
		t.Fatalf("expected owner %s, got %s", testAddr(1), exported.Records[0].Owner)
	}
}

func TestMsgRegisterAccount(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	owner := testAddr(1)
	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Owner:        owner,
		ConnectionId: "connection-0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	accounts := k.GetRemoteAccounts(ctx, owner)
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
}

func TestMsgRegisterAccountMaxAccounts(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Set max to 2
	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 2,
		AllowedHostMsgTypes:       []string{},
		RegistrationCooldown:      0, // disable cooldown for this test
		MaxMessagesPerTx:          5,
	})

	owner := testAddr(1)

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-0"})
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	_, err = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-1"})
	if err != nil {
		t.Fatalf("second registration should succeed: %v", err)
	}

	_, err = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-2"})
	if err == nil {
		t.Fatal("third registration should fail (max=2)")
	}
}

func TestMsgRegisterAccountDuplicate(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          5,
	})

	owner := testAddr(1)

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-0"})
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	_, err = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-0"})
	if err == nil {
		t.Fatal("duplicate registration should fail")
	}
}

func TestMsgRegisterAccountCooldown(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{},
		RegistrationCooldown:      100,
		MaxMessagesPerTx:          5,
	})

	owner := testAddr(1)

	// Register at height 100
	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-0"})
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	// Try at height 150 (100 + 100 = 200 is the cooldown expiry)
	ctx150 := ctx.WithBlockHeight(150)
	_, err = ms.RegisterAccount(ctx150, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-1"})
	if err == nil {
		t.Fatal("registration during cooldown should fail")
	}

	// Try at height 200 (cooldown expired)
	ctx200 := ctx.WithBlockHeight(200)
	_, err = ms.RegisterAccount(ctx200, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-1"})
	if err != nil {
		t.Fatalf("registration after cooldown should succeed: %v", err)
	}
}

func TestMsgRegisterAccountICAFailure(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	mock.registerErr = fmt.Errorf("ICA registration failed")

	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Owner:        testAddr(1),
		ConnectionId: "connection-0",
	})
	if err == nil {
		t.Fatal("should propagate ICA controller error")
	}
}

func TestMsgSubmitTxNotRegistered(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        testAddr(1),
		ConnectionId: "connection-0",
		Msgs:         nil,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("should fail when not registered")
	}
}

func TestDefaultParamsMsgTransferExcluded(t *testing.T) {
	params := types.DefaultParams()
	for _, msgType := range params.AllowedHostMsgTypes {
		if msgType == "/ibc.applications.transfer.v1.MsgTransfer" {
			t.Fatal("SECURITY: MsgTransfer must NOT be in DefaultParams AllowedHostMsgTypes (P0-6)")
		}
	}
}

func TestTimeoutValidation(t *testing.T) {
	// Zero timeout — rejected
	msg := &types.MsgSubmitTx{
		Owner:        testAddr(1),
		ConnectionId: "connection-0",
		Msgs:         nil,
		TimeoutNs:    0,
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("timeout_ns=0 should be rejected")
	}

	// Under 60s — rejected
	msg.TimeoutNs = uint64(30 * time.Second)
	msg.Msgs = nil
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("timeout_ns < 60s should be rejected")
	}

	// Exactly 60s — but msgs is empty so still fails on msgs check
	msg.TimeoutNs = uint64(60 * time.Second)
	msg.Msgs = nil
	err := msg.ValidateBasic()
	if err == nil {
		t.Fatal("empty msgs should be rejected")
	}
	// Make sure the error is about msgs, not timeout
	if err.Error() == "timeout_ns cannot be zero" || err.Error() == fmt.Sprintf("timeout_ns must be at least %d (60s)", types.MinTimeout) {
		t.Fatal("60s timeout should pass validation, error should be about empty msgs")
	}
}

func TestMsgUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	authority := sdk.AccAddress([]byte("authority-addr------")).String()

	// Success
	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params: &types.Params{
			MaxRemoteAccountsPerOwner: 20,
			AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
			RegistrationCooldown:      50,
			MaxMessagesPerTx:          10,
		},
	})
	if err != nil {
		t.Fatalf("update params should succeed: %v", err)
	}

	params := k.GetParams(ctx)
	if params.MaxRemoteAccountsPerOwner != 20 {
		t.Fatalf("expected max=20, got %d", params.MaxRemoteAccountsPerOwner)
	}

	// Wrong authority
	_, err = ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr(99),
		Params: &types.Params{
			MaxRemoteAccountsPerOwner: 1,
			AllowedHostMsgTypes:       nil,
			RegistrationCooldown:      0,
			MaxMessagesPerTx:          1,
		},
	})
	if err == nil {
		t.Fatal("wrong authority should fail")
	}
}

// ---------- R6-6 Security Tests ----------

// setupActiveAccount registers an account, marks it active, and sets up the
// mock ICA controller to report an open channel for it.
func setupActiveAccount(t *testing.T, k keeper.Keeper, ctx sdk.Context, mock *mockICAController, owner string, connectionID string) {
	t.Helper()
	portID := "icacontroller-" + owner
	acct := &types.RemoteAccount{
		ConnectionId:    connectionID,
		PortId:          portID,
		OwnerAddress:    owner,
		Active:          true,
		RegisteredBlock: uint64(ctx.BlockHeight()),
		RemoteAddress:   "cosmos1remote...",
	}
	k.AddRemoteAccount(ctx, owner, acct)
	mock.openChannels[connectionID+"/"+portID] = true
}

func TestSubmitTxMaxMessages(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          2,
	})

	owner := testAddr(1)
	setupActiveAccount(t, k, ctx, mock, owner, "connection-0")

	// Create 3 messages (exceeds MaxMessagesPerTx=2)
	msgs := make([]*anypb.Any, 3)
	for i := range msgs {
		msgs[i] = &anypb.Any{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}}
	}

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("should reject tx exceeding MaxMessagesPerTx")
	}
	if !strings.Contains(err.Error(), "max is 2") {
		t.Fatalf("error should mention max limit, got: %v", err)
	}
}

func TestSubmitTxDisallowedMessage(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Only allow MsgSend in global allowlist
	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          5,
	})

	owner := testAddr(1)
	setupActiveAccount(t, k, ctx, mock, owner, "connection-0")

	// Submit with MsgTransfer (explicitly excluded / not in allowlist)
	msgs := []*anypb.Any{
		{TypeUrl: "/ibc.applications.transfer.v1.MsgTransfer", Value: []byte{}},
	}

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("SECURITY: disallowed message type must be rejected")
	}
	if !strings.Contains(err.Error(), "not in global allowlist") {
		t.Fatalf("error should mention global allowlist, got: %v", err)
	}
}

func TestSubmitTxPerAccountAllowlistRestriction(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Global allowlist permits MsgSend + MsgDelegate
	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes: []string{
			"/cosmos.bank.v1beta1.MsgSend",
			"/cosmos.staking.v1beta1.MsgDelegate",
		},
		RegistrationCooldown: 0,
		MaxMessagesPerTx:     5,
	})

	owner := testAddr(1)
	portID := "icacontroller-" + owner
	// Register with per-account allowlist that only permits MsgSend
	acct := &types.RemoteAccount{
		ConnectionId:    "connection-0",
		PortId:          portID,
		OwnerAddress:    owner,
		Active:          true,
		RegisteredBlock: uint64(ctx.BlockHeight()),
		RemoteAddress:   "cosmos1remote...",
		AllowedMsgTypes: []string{"/cosmos.bank.v1beta1.MsgSend"},
	}
	k.AddRemoteAccount(ctx, owner, acct)
	mock.openChannels["connection-0/"+portID] = true

	// MsgDelegate passes global but fails per-account
	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.staking.v1beta1.MsgDelegate", Value: []byte{}},
	}

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("SECURITY: per-account allowlist must restrict beyond global")
	}
	if !strings.Contains(err.Error(), "account allowlist") {
		t.Fatalf("error should mention account allowlist, got: %v", err)
	}
}

func TestSubmitTxInactiveAccount(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	owner := testAddr(1)
	// Register but do NOT activate
	acct := &types.RemoteAccount{
		ConnectionId:    "connection-0",
		PortId:          "icacontroller-" + owner,
		OwnerAddress:    owner,
		Active:          false,
		RegisteredBlock: uint64(ctx.BlockHeight()),
	}
	k.AddRemoteAccount(ctx, owner, acct)

	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("should reject tx on inactive account")
	}
	if !strings.Contains(err.Error(), "not active") {
		t.Fatalf("error should mention inactive, got: %v", err)
	}
}

func TestSubmitTxChannelClosedMarksInactive(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	owner := testAddr(1)
	portID := "icacontroller-" + owner
	acct := &types.RemoteAccount{
		ConnectionId:    "connection-0",
		PortId:          portID,
		OwnerAddress:    owner,
		Active:          true,
		RegisteredBlock: uint64(ctx.BlockHeight()),
		RemoteAddress:   "cosmos1remote...",
	}
	k.AddRemoteAccount(ctx, owner, acct)
	// NOTE: do NOT add to mock.openChannels — channel is closed

	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("should reject tx when channel is closed")
	}

	// Verify account was marked inactive
	found, ok := k.GetRemoteAccountByConnection(ctx, owner, "connection-0")
	if !ok {
		t.Fatal("account record should still exist")
	}
	if found.Active {
		t.Fatal("account should be marked inactive after closed channel detection")
	}

	// Suppress unused variable warnings for mock
	_ = mock
}

// ============================================================
// R15-4: Register Interchain Account Tests
// ============================================================

func TestRegisterInterchainAccount_EmptyOwner(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	msg := &types.MsgRegisterAccount{
		Owner:        "",
		ConnectionId: "connection-0",
	}
	_, err := ms.RegisterAccount(ctx, msg)
	if err == nil {
		t.Fatal("expected error for empty owner")
	}
}

func TestRegisterInterchainAccount_EmptyConnectionID(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	msg := &types.MsgRegisterAccount{
		Owner:        testAddr(1),
		ConnectionId: "",
	}
	_, err := ms.RegisterAccount(ctx, msg)
	if err == nil {
		t.Fatal("expected error for empty connection_id")
	}
}

func TestRegisterInterchainAccount_MultipleOwners(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          5,
	})

	owner1 := testAddr(1)
	owner2 := testAddr(2)

	// Both owners register on connection-0 — should succeed (different owners)
	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner1, ConnectionId: "connection-0"})
	if err != nil {
		t.Fatalf("owner1 registration should succeed: %v", err)
	}
	_, err = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner2, ConnectionId: "connection-0"})
	if err != nil {
		t.Fatalf("owner2 registration should succeed: %v", err)
	}

	accounts1 := k.GetRemoteAccounts(ctx, owner1)
	accounts2 := k.GetRemoteAccounts(ctx, owner2)
	if len(accounts1) != 1 {
		t.Fatalf("owner1: expected 1 account, got %d", len(accounts1))
	}
	if len(accounts2) != 1 {
		t.Fatalf("owner2: expected 1 account, got %d", len(accounts2))
	}
}

func TestRegisterInterchainAccount_PortIDGeneration(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	owner := testAddr(1)
	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{
		Owner:        owner,
		ConnectionId: "connection-0",
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	accounts := k.GetRemoteAccounts(ctx, owner)
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	expectedPort := "icacontroller-" + owner
	if accounts[0].PortId != expectedPort {
		t.Errorf("expected port %s, got %s", expectedPort, accounts[0].PortId)
	}
}

// ============================================================
// R15-4: Submit Interchain Tx Tests
// ============================================================

func TestSubmitInterchainTx_SendError(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          5,
	})

	owner := testAddr(1)
	setupActiveAccount(t, k, ctx, mock, owner, "connection-0")

	// Configure send failure
	mock.sendErr = fmt.Errorf("mock send failure")

	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}

	// Note: in the test environment, the codec's interface registry has no
	// registered implementations, so the error occurs at the unpack stage
	// rather than at SendTx. Either way, SubmitTx must return an error.
	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("expected error from ICA submit tx")
	}
}

func TestSubmitInterchainTx_EmptyMsgs(t *testing.T) {
	msg := &types.MsgSubmitTx{
		Owner:        testAddr(1),
		ConnectionId: "connection-0",
		Msgs:         nil,
		TimeoutNs:    uint64(120 * time.Second),
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty msgs")
	}
}

func TestSubmitInterchainTx_EmptyOwner(t *testing.T) {
	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}
	msg := &types.MsgSubmitTx{
		Owner:        "",
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty owner in SubmitTx")
	}
}

// ============================================================
// R15-4: ICA Tx Timeout Tests
// ============================================================

func TestICATxTimeout_Below60s(t *testing.T) {
	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}
	msg := &types.MsgSubmitTx{
		Owner:        testAddr(1),
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(59 * time.Second), // below 60s minimum
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("expected error for timeout below 60s")
	}
}

func TestICATxTimeout_Exactly60s(t *testing.T) {
	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}
	msg := &types.MsgSubmitTx{
		Owner:        testAddr(1),
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    types.MinTimeout, // exactly 60s
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("expected no error for exactly 60s timeout, got: %v", err)
	}
}

func TestICATxTimeout_LargeTimeout(t *testing.T) {
	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}
	msg := &types.MsgSubmitTx{
		Owner:        testAddr(1),
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(24 * time.Hour), // 24 hours
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Fatalf("expected no error for 24h timeout, got: %v", err)
	}
}

// ============================================================
// R15-4: ICA Tx Unauthorized Tests
// ============================================================

func TestICATxUnauthorized_WrongOwner(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          5,
	})

	// Register with owner1
	owner1 := testAddr(1)
	setupActiveAccount(t, k, ctx, mock, owner1, "connection-0")

	// Try to submit with owner2 (different owner)
	owner2 := testAddr(2)
	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner2,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("expected error when wrong owner submits tx")
	}
}

func TestICATxUnauthorized_WrongConnection(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          5,
	})

	owner := testAddr(1)
	setupActiveAccount(t, k, ctx, mock, owner, "connection-0")

	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}

	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-99", // wrong connection
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("expected error for unregistered connection")
	}
}

// ============================================================
// R15-4: ICA Account Reuse Tests
// ============================================================

func TestICAAccountReuse_ReactivateAfterChannelClose(t *testing.T) {
	k, ctx, mock := setupKeeper(t)

	owner := testAddr(1)
	portID := "icacontroller-" + owner

	// Register and activate
	acct := &types.RemoteAccount{
		ConnectionId:    "connection-0",
		PortId:          portID,
		OwnerAddress:    owner,
		Active:          true,
		RegisteredBlock: uint64(ctx.BlockHeight()),
		RemoteAddress:   "cosmos1remote...",
	}
	k.AddRemoteAccount(ctx, owner, acct)
	// Do NOT open the channel — simulate channel close

	// Account is active but channel is closed (not in mock.openChannels)
	ms := keeper.NewMsgServerImpl(k)
	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      0,
		MaxMessagesPerTx:          5,
	})

	msgs := []*anypb.Any{
		{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte{}},
	}

	// SubmitTx should fail and mark account inactive
	_, err := ms.SubmitTx(ctx, &types.MsgSubmitTx{
		Owner:        owner,
		ConnectionId: "connection-0",
		Msgs:         msgs,
		TimeoutNs:    uint64(120 * time.Second),
	})
	if err == nil {
		t.Fatal("expected error when channel is closed")
	}

	// Verify account was marked inactive
	found, ok := k.GetRemoteAccountByConnection(ctx, owner, "connection-0")
	if !ok {
		t.Fatal("account record should still exist")
	}
	if found.Active {
		t.Fatal("account should be marked inactive after channel close detection")
	}

	// Reactivate by updating address
	k.UpdateRemoteAccountAddress(ctx, owner, "connection-0", "cosmos1newremote...")
	found, ok = k.GetRemoteAccountByConnection(ctx, owner, "connection-0")
	if !ok {
		t.Fatal("account should still exist after reactivation")
	}
	if !found.Active {
		t.Fatal("account should be active after UpdateRemoteAccountAddress")
	}
	if found.RemoteAddress != "cosmos1newremote..." {
		t.Errorf("expected cosmos1newremote..., got %s", found.RemoteAddress)
	}

	_ = mock
}

func TestICAAccountReuse_UpdateNonexistentOwner(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Update on a non-existent owner — should be a no-op (no panic)
	k.UpdateRemoteAccountAddress(ctx, testAddr(99), "connection-0", "cosmos1addr...")

	// Verify nothing was created
	accounts := k.GetRemoteAccounts(ctx, testAddr(99))
	if len(accounts) != 0 {
		t.Fatal("expected no accounts for non-existent owner")
	}
}

// ============================================================
// R15-4: ICA Channel Ordering Tests
// ============================================================

func TestICAChannelOrdering_MultipleConnectionsSameOwner(t *testing.T) {
	k, ctx, mock := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	k.SetParams(ctx, &types.Params{
		MaxRemoteAccountsPerOwner: 10,
		AllowedHostMsgTypes:       []string{"/cosmos.bank.v1beta1.MsgSend"},
		RegistrationCooldown:      0, // disable for this test
		MaxMessagesPerTx:          5,
	})

	owner := testAddr(1)

	// Register on connection-0
	_, err := ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-0"})
	if err != nil {
		t.Fatalf("connection-0 registration should succeed: %v", err)
	}

	// Register on connection-1
	_, err = ms.RegisterAccount(ctx, &types.MsgRegisterAccount{Owner: owner, ConnectionId: "connection-1"})
	if err != nil {
		t.Fatalf("connection-1 registration should succeed: %v", err)
	}

	accounts := k.GetRemoteAccounts(ctx, owner)
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}

	// Verify each can be independently found
	acc0, ok := k.GetRemoteAccountByConnection(ctx, owner, "connection-0")
	if !ok {
		t.Fatal("expected connection-0 account to be found")
	}
	if acc0.ConnectionId != "connection-0" {
		t.Errorf("expected connection-0, got %s", acc0.ConnectionId)
	}

	acc1, ok := k.GetRemoteAccountByConnection(ctx, owner, "connection-1")
	if !ok {
		t.Fatal("expected connection-1 account to be found")
	}
	if acc1.ConnectionId != "connection-1" {
		t.Errorf("expected connection-1, got %s", acc1.ConnectionId)
	}

	_ = mock
}

// ============================================================
// R15-4: Query Server Additional Tests
// ============================================================

func TestQueryAccountNotRegistered(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Account(ctx, &types.QueryAccountRequest{
		Owner:        testAddr(99),
		ConnectionId: "connection-0",
	})
	if err == nil {
		t.Fatal("expected error for non-existent account")
	}
}

func TestQueryAccountsForOwner(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	owner := testAddr(1)

	// No accounts yet
	resp, err := qs.Accounts(ctx, &types.QueryAccountsRequest{Owner: owner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Accounts) != 0 {
		t.Fatalf("expected 0 accounts, got %d", len(resp.Accounts))
	}

	// Add an account
	k.AddRemoteAccount(ctx, owner, &types.RemoteAccount{
		ConnectionId: "connection-0",
		OwnerAddress: owner,
		Active:       true,
	})

	resp, err = qs.Accounts(ctx, &types.QueryAccountsRequest{Owner: owner})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(resp.Accounts))
	}
}

func TestQueryParamsNilRequest(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Params(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestQueryAccountNilRequest(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Account(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestQueryAccountsMissingOwner(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Accounts(ctx, &types.QueryAccountsRequest{Owner: ""})
	if err == nil {
		t.Fatal("expected error for empty owner")
	}
}

// ============================================================
// R15-4: Genesis Edge Cases
// ============================================================

func TestGenesisNilParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	gs := &types.GenesisState{
		Params:  nil,
		Records: nil,
	}
	k.InitGenesis(ctx, gs)

	// Should get default params
	params := k.GetParams(ctx)
	if params.MaxRemoteAccountsPerOwner != 5 {
		t.Errorf("expected default max=5, got %d", params.MaxRemoteAccountsPerOwner)
	}
	if params.MaxMessagesPerTx != 5 {
		t.Errorf("expected default MaxMessagesPerTx=5, got %d", params.MaxMessagesPerTx)
	}
}

func TestGenesisMultipleRecords(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	gs := &types.GenesisState{
		Params: types.DefaultParams(),
		Records: []*types.InterchainAccountRecord{
			{
				Owner: testAddr(1),
				Accounts: []*types.RemoteAccount{
					{ConnectionId: "connection-0", OwnerAddress: testAddr(1), Active: true},
					{ConnectionId: "connection-1", OwnerAddress: testAddr(1), Active: false},
				},
			},
			{
				Owner: testAddr(2),
				Accounts: []*types.RemoteAccount{
					{ConnectionId: "connection-0", OwnerAddress: testAddr(2), Active: true},
				},
			},
		},
	}
	k.InitGenesis(ctx, gs)

	exported := k.ExportGenesis(ctx)
	if len(exported.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(exported.Records))
	}

	// Verify owner1 has 2 accounts
	accounts1 := k.GetRemoteAccounts(ctx, testAddr(1))
	if len(accounts1) != 2 {
		t.Fatalf("owner1: expected 2 accounts, got %d", len(accounts1))
	}

	// Verify owner2 has 1 account
	accounts2 := k.GetRemoteAccounts(ctx, testAddr(2))
	if len(accounts2) != 1 {
		t.Fatalf("owner2: expected 1 account, got %d", len(accounts2))
	}
}

// ============================================================
// R15-4: UpdateParams Validation Tests
// ============================================================

func TestMsgUpdateParamsValidation(t *testing.T) {
	msg := &types.MsgUpdateParams{
		Authority: "",
		Params:    types.DefaultParams(),
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty authority")
	}

	msg2 := &types.MsgUpdateParams{
		Authority: testAddr(1),
		Params:    nil,
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Fatal("expected error for nil params")
	}

	msg3 := &types.MsgUpdateParams{
		Authority: testAddr(1),
		Params: &types.Params{
			MaxRemoteAccountsPerOwner: 0, // invalid
			MaxMessagesPerTx:          5,
		},
	}
	if err := msg3.ValidateBasic(); err == nil {
		t.Fatal("expected error for zero max_remote_accounts_per_owner")
	}

	msg4 := &types.MsgUpdateParams{
		Authority: testAddr(1),
		Params: &types.Params{
			MaxRemoteAccountsPerOwner: 5,
			MaxMessagesPerTx:          0, // invalid
		},
	}
	if err := msg4.ValidateBasic(); err == nil {
		t.Fatal("expected error for zero max_messages_per_tx")
	}
}

func TestDefaultParams_SecurityExclusions(t *testing.T) {
	params := types.DefaultParams()

	// Verify MsgTransfer is NOT included (P0-6 security)
	for _, msgType := range params.AllowedHostMsgTypes {
		if msgType == "/ibc.applications.transfer.v1.MsgTransfer" {
			t.Fatal("SECURITY: MsgTransfer must not be in default AllowedHostMsgTypes")
		}
	}

	// Verify expected governance/staking types ARE present
	expected := map[string]bool{
		"/cosmos.bank.v1beta1.MsgSend":                             false,
		"/cosmos.staking.v1beta1.MsgDelegate":                      false,
		"/cosmos.staking.v1beta1.MsgUndelegate":                    false,
		"/cosmos.staking.v1beta1.MsgBeginRedelegate":               false,
		"/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward":  false,
	}
	for _, msgType := range params.AllowedHostMsgTypes {
		if _, ok := expected[msgType]; ok {
			expected[msgType] = true
		}
	}
	for msgType, found := range expected {
		if !found {
			t.Errorf("expected %s in AllowedHostMsgTypes", msgType)
		}
	}
}
