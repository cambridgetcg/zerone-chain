package keeper_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/home/keeper"
	"github.com/zerone-chain/zerone/x/home/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

// ---------- Test Address Helper ----------

func testAddr(name string) string {
	hash := sha256.Sum256([]byte("test_addr:" + name))
	return sdk.AccAddress(hash[:20]).String()
}

// ---------- Mock BankKeeper ----------

type mockBankKeeper struct {
	balances       map[string]map[string]int64
	moduleBalances map[string]map[string]int64
	failNext       bool
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances:       make(map[string]map[string]int64),
		moduleBalances: make(map[string]map[string]int64),
	}
}

func (m *mockBankKeeper) setBalance(addr, denom string, amount int64) {
	if m.balances[addr] == nil {
		m.balances[addr] = make(map[string]int64)
	}
	m.balances[addr][denom] = amount
}

func (m *mockBankKeeper) SendCoins(_ context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error {
	if m.failNext {
		m.failNext = false
		return fmt.Errorf("insufficient funds")
	}
	for _, coin := range amt {
		from := fromAddr.String()
		to := toAddr.String()
		if m.balances[from] == nil {
			m.balances[from] = make(map[string]int64)
		}
		if m.balances[to] == nil {
			m.balances[to] = make(map[string]int64)
		}
		m.balances[from][coin.Denom] -= coin.Amount.Int64()
		m.balances[to][coin.Denom] += coin.Amount.Int64()
	}
	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	if m.failNext {
		m.failNext = false
		return fmt.Errorf("insufficient funds")
	}
	for _, coin := range amt {
		from := senderAddr.String()
		if m.balances[from] == nil {
			m.balances[from] = make(map[string]int64)
		}
		if m.moduleBalances[recipientModule] == nil {
			m.moduleBalances[recipientModule] = make(map[string]int64)
		}
		m.balances[from][coin.Denom] -= coin.Amount.Int64()
		m.moduleBalances[recipientModule][coin.Denom] += coin.Amount.Int64()
	}
	return nil
}

var _ types.BankKeeper = (*mockBankKeeper)(nil)

// ---------- Test Setup ----------

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper) {
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

	mockBK := newMockBankKeeper()
	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(storeService, cdc, "zrn1authority", mockBK)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	// Set default params.
	dp := types.DefaultParams()
	k.SetParams(ctx, dp)

	return k, ctx, mockBK
}

func setupMsgServer(t *testing.T) (types.MsgServer, keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeper(t)
	return keeper.NewMsgServerImpl(k), k, ctx, bk
}

func setupQueryServer(t *testing.T) (types.QueryServer, keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeper(t)
	return keeper.NewQueryServerImpl(k), k, ctx, bk
}

// ---------- Params Tests ----------

func TestDefaultParams(t *testing.T) {
	p := types.DefaultParams()

	if p.MaxKeysPerHome != 20 {
		t.Errorf("MaxKeysPerHome = %d, want 20", p.MaxKeysPerHome)
	}
	if p.MaxSessionsPerHome != 5 {
		t.Errorf("MaxSessionsPerHome = %d, want 5", p.MaxSessionsPerHome)
	}
	if p.SessionTimeoutBlocks != 1000 {
		t.Errorf("SessionTimeoutBlocks = %d, want 1000", p.SessionTimeoutBlocks)
	}
	if p.DeadmanMinThreshold != 100 {
		t.Errorf("DeadmanMinThreshold = %d, want 100", p.DeadmanMinThreshold)
	}
	if p.DeadmanMaxThreshold != 100000 {
		t.Errorf("DeadmanMaxThreshold = %d, want 100000", p.DeadmanMaxThreshold)
	}
	if p.MaxAlertsPerHome != 100 {
		t.Errorf("MaxAlertsPerHome = %d, want 100", p.MaxAlertsPerHome)
	}
	if p.HomeCreationFee != "10000000" {
		t.Errorf("HomeCreationFee = %q, want 10000000", p.HomeCreationFee)
	}
	if p.MaxRecoveryAddresses != 5 {
		t.Errorf("MaxRecoveryAddresses = %d, want 5", p.MaxRecoveryAddresses)
	}
}

func TestSetGetParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	custom := &types.Params{
		MaxKeysPerHome:       10,
		MaxSessionsPerHome:   3,
		SessionTimeoutBlocks: 500,
		DeadmanMinThreshold:  200,
		DeadmanMaxThreshold:  50000,
		MaxAlertsPerHome:     50,
		HomeCreationFee:      "5000000",
		MaxRecoveryAddresses: 3,
	}
	k.SetParams(ctx, custom)

	got := k.GetParams(ctx)
	if got.MaxKeysPerHome != 10 {
		t.Errorf("MaxKeysPerHome = %d, want 10", got.MaxKeysPerHome)
	}
	if got.MaxSessionsPerHome != 3 {
		t.Errorf("MaxSessionsPerHome = %d, want 3", got.MaxSessionsPerHome)
	}
	if got.HomeCreationFee != "5000000" {
		t.Errorf("HomeCreationFee = %q, want 5000000", got.HomeCreationFee)
	}
}

func TestParamsValidation(t *testing.T) {
	valid := types.DefaultParams()
	if err := valid.Validate(); err != nil {
		t.Errorf("valid params failed: %v", err)
	}

	// MaxKeysPerHome = 0 should fail.
	bad := *valid
	bad.MaxKeysPerHome = 0
	if err := bad.Validate(); err == nil {
		t.Error("expected error for MaxKeysPerHome=0")
	}

	// DeadmanMaxThreshold <= DeadmanMinThreshold should fail.
	bad2 := *valid
	bad2.DeadmanMaxThreshold = bad2.DeadmanMinThreshold
	if err := bad2.Validate(); err == nil {
		t.Error("expected error for DeadmanMaxThreshold <= DeadmanMinThreshold")
	}
}

// ---------- CreateHome Tests ----------

func TestCreateHome(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("alice")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := msgSrv.CreateHome(ctx, &types.MsgCreateHome{
		Owner: owner,
		Name:  "Alice's Home",
	})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}
	if resp.HomeId != "home-1" {
		t.Errorf("HomeId = %q, want home-1", resp.HomeId)
	}

	// Verify home fields.
	home, found := k.GetHome(ctx, "home-1")
	if !found {
		t.Fatal("home not found")
	}
	if home.OwnerAddress != owner {
		t.Errorf("OwnerAddress = %q, want %q", home.OwnerAddress, owner)
	}
	if home.Name != "Alice's Home" {
		t.Errorf("Name = %q, want Alice's Home", home.Name)
	}
	if home.Status != "active" {
		t.Errorf("Status = %q, want active", home.Status)
	}
	if home.ComfortScore != 50 {
		t.Errorf("ComfortScore = %d, want 50", home.ComfortScore)
	}
	if home.CreatedAtBlock != 100 {
		t.Errorf("CreatedAtBlock = %d, want 100", home.CreatedAtBlock)
	}
	if home.LastActiveBlock != 100 {
		t.Errorf("LastActiveBlock = %d, want 100", home.LastActiveBlock)
	}
	if home.Guardian == nil || home.Guardian.DefenseStrategy != "moderate" {
		t.Error("default guardian not set correctly")
	}
}

func TestCreateHome_FeeDeduction(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("bob")
	bk.setBalance(owner, "uzrn", 50_000_000)

	_, err := msgSrv.CreateHome(ctx, &types.MsgCreateHome{
		Owner: owner,
		Name:  "Bob's Home",
	})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Fee of 10,000,000 uzrn should be charged to fee_collector.
	if bk.balances[owner]["uzrn"] != 40_000_000 {
		t.Errorf("owner balance = %d, want 40000000", bk.balances[owner]["uzrn"])
	}
	if bk.moduleBalances["fee_collector"]["uzrn"] != 10_000_000 {
		t.Errorf("fee_collector balance = %d, want 10000000", bk.moduleBalances["fee_collector"]["uzrn"])
	}
}

func TestCreateHome_InsufficientFunds(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("broke")
	bk.failNext = true

	_, err := msgSrv.CreateHome(ctx, &types.MsgCreateHome{
		Owner: owner,
		Name:  "Poor Home",
	})
	if err == nil {
		t.Error("expected error for insufficient funds")
	}
}

func TestCreateHome_IncrementingIDs(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("carol")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp1, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Home 1"})
	resp2, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Home 2"})
	resp3, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Home 3"})

	if resp1.HomeId != "home-1" || resp2.HomeId != "home-2" || resp3.HomeId != "home-3" {
		t.Errorf("IDs = %s/%s/%s, want home-1/home-2/home-3", resp1.HomeId, resp2.HomeId, resp3.HomeId)
	}
}

func TestCreateHome_WithGuardianConfig(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("dan")
	bk.setBalance(owner, "uzrn", 50_000_000)

	guardian := &types.HomeGuardian{
		DefenseStrategy: "aggressive",
		AutoDefend:      true,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 500,
			Action:              "alert_guardians",
		},
	}

	resp, err := msgSrv.CreateHome(ctx, &types.MsgCreateHome{
		Owner:                owner,
		Name:                 "Dan's Fortress",
		InitialGuardianConfig: guardian,
	})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	home, _ := k.GetHome(ctx, resp.HomeId)
	if home.Guardian.DefenseStrategy != "aggressive" {
		t.Errorf("DefenseStrategy = %q, want aggressive", home.Guardian.DefenseStrategy)
	}
	if !home.Guardian.AutoDefend {
		t.Error("AutoDefend should be true")
	}
	if home.Guardian.Deadman == nil || !home.Guardian.Deadman.Enabled {
		t.Error("Deadman should be enabled")
	}
}

func TestCreateHome_OwnerIndex(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("eve")
	bk.setBalance(owner, "uzrn", 100_000_000)

	msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Home A"})
	msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Home B"})

	ids := k.GetHomesByOwner(ctx, owner)
	if len(ids) != 2 {
		t.Errorf("owner has %d homes, want 2", len(ids))
	}
}

// ---------- UpdateHome Tests ----------

func TestUpdateHome_Name(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("frank")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Old Name"})

	_, err := msgSrv.UpdateHome(ctx, &types.MsgUpdateHome{
		Owner:  owner,
		HomeId: resp.HomeId,
		Name:   "New Name",
	})
	if err != nil {
		t.Fatalf("UpdateHome failed: %v", err)
	}

	home, _ := k.GetHome(ctx, resp.HomeId)
	if home.Name != "New Name" {
		t.Errorf("Name = %q, want New Name", home.Name)
	}
}

func TestUpdateHome_StatusTransitions(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("grace")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Grace Home"})

	tests := []struct {
		from, to string
		valid    bool
	}{
		{"active", "dormant", true},
		{"dormant", "active", true},
		{"active", "guarded", true},
		{"guarded", "active", true},
		{"active", "recovery", true},
		{"recovery", "active", true},
		{"active", "archived", true},
	}

	for _, tt := range tests {
		// Set to the 'from' status directly.
		home, _ := k.GetHome(ctx, resp.HomeId)
		home.Status = tt.from
		k.SetHome(ctx, home)

		_, err := msgSrv.UpdateHome(ctx, &types.MsgUpdateHome{
			Owner:  owner,
			HomeId: resp.HomeId,
			Status: tt.to,
		})
		if tt.valid && err != nil {
			t.Errorf("%s -> %s: unexpected error: %v", tt.from, tt.to, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("%s -> %s: expected error", tt.from, tt.to)
		}
	}
}

func TestUpdateHome_ArchivedTerminal(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("hank")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Hank Home"})

	// Set to archived.
	home, _ := k.GetHome(ctx, resp.HomeId)
	home.Status = "archived"
	k.SetHome(ctx, home)

	// Try every possible transition from archived — all should fail.
	for _, target := range []string{"active", "dormant", "guarded", "recovery"} {
		_, err := msgSrv.UpdateHome(ctx, &types.MsgUpdateHome{
			Owner:  owner,
			HomeId: resp.HomeId,
			Status: target,
		})
		if err == nil {
			t.Errorf("archived -> %s: should have failed", target)
		}
	}
}

// ---------- UpdateMemoryCID Tests ----------

func TestUpdateMemoryCID(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("ivan")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Ivan Home"})

	_, err := msgSrv.UpdateMemoryCID(ctx, &types.MsgUpdateMemoryCID{
		Owner:  owner,
		HomeId: resp.HomeId,
		Cid:    "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
	})
	if err != nil {
		t.Fatalf("UpdateMemoryCID failed: %v", err)
	}

	home, _ := k.GetHome(ctx, resp.HomeId)
	if home.MemoryCid != "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi" {
		t.Errorf("MemoryCid mismatch")
	}
}

// ---------- Key Registration Tests ----------

func TestRegisterKey(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("jane")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Jane Home"})

	_, err := msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "abc123def456",
		KeyType:     "ethereum",
		Role:        "operator",
		Permissions: []string{"read", "write", "transfer"},
		ExpiresAt:   5000,
	})
	if err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}

	reg, found := k.GetKeyRegistration(ctx, resp.HomeId, "abc123def456")
	if !found {
		t.Fatal("key registration not found")
	}
	if reg.KeyType != "ethereum" {
		t.Errorf("KeyType = %q, want ethereum", reg.KeyType)
	}
	if reg.Role != "operator" {
		t.Errorf("Role = %q, want operator", reg.Role)
	}
	if len(reg.Permissions) != 3 {
		t.Errorf("len(Permissions) = %d, want 3", len(reg.Permissions))
	}
	if reg.ExpiresAt != 5000 {
		t.Errorf("ExpiresAt = %d, want 5000", reg.ExpiresAt)
	}
	if reg.RegisteredAt != 100 {
		t.Errorf("RegisteredAt = %d, want 100", reg.RegisteredAt)
	}
}

func TestRegisterKey_Duplicate(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("karen")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Karen Home"})

	msg := &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "dupkey111",
		KeyType:     "cosmos",
		Role:        "admin",
		Permissions: []string{"all"},
	}

	_, err := msgSrv.RegisterKey(ctx, msg)
	if err != nil {
		t.Fatalf("first RegisterKey failed: %v", err)
	}

	_, err = msgSrv.RegisterKey(ctx, msg)
	if err == nil {
		t.Error("expected error for duplicate key registration")
	}
}

func TestRegisterKey_MaxKeysEnforced(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("leo")
	bk.setBalance(owner, "uzrn", 50_000_000)

	// Set max keys to 3.
	params := k.GetParams(ctx)
	params.MaxKeysPerHome = 3
	k.SetParams(ctx, params)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Leo Home"})

	for i := 0; i < 3; i++ {
		_, err := msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
			Owner:       owner,
			HomeId:      resp.HomeId,
			KeyHash:     fmt.Sprintf("key-%d", i),
			KeyType:     "cosmos",
			Role:        "operator",
			Permissions: []string{"read"},
		})
		if err != nil {
			t.Fatalf("RegisterKey %d failed: %v", i, err)
		}
	}

	// 4th key should fail.
	_, err := msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "key-overflow",
		KeyType:     "cosmos",
		Role:        "guest",
		Permissions: []string{"read"},
	})
	if err == nil {
		t.Error("expected error for exceeding max keys")
	}
}

// ---------- Key Lifecycle (Register → Session → Revoke → Verify) ----------

func TestKeyLifecycle_Full(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("mike")
	bk.setBalance(owner, "uzrn", 50_000_000)

	// Step 1: Create home.
	homeResp, err := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Mike Home"})
	if err != nil {
		t.Fatalf("CreateHome: %v", err)
	}
	homeID := homeResp.HomeId

	// Step 2: Register key.
	_, err = msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      homeID,
		KeyHash:     "lifecycle-key-1",
		KeyType:     "ethereum",
		Role:        "operator",
		Permissions: []string{"read", "write", "transfer"},
	})
	if err != nil {
		t.Fatalf("RegisterKey: %v", err)
	}

	// Step 3: Start session.
	sessionResp, err := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               homeID,
		KeyHash:              "lifecycle-key-1",
		RequestedPermissions: []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Verify session exists.
	session, found := k.GetSession(ctx, homeID, sessionResp.SessionId)
	if !found {
		t.Fatal("session not found after creation")
	}
	if len(session.Permissions) != 2 {
		t.Errorf("session permissions = %d, want 2", len(session.Permissions))
	}
	if session.KeyHash != "lifecycle-key-1" {
		t.Errorf("session KeyHash = %q, want lifecycle-key-1", session.KeyHash)
	}

	// Step 4: Revoke key.
	_, err = msgSrv.RevokeKey(ctx, &types.MsgRevokeKey{
		Owner:   owner,
		HomeId:  homeID,
		KeyHash: "lifecycle-key-1",
	})
	if err != nil {
		t.Fatalf("RevokeKey: %v", err)
	}

	// Step 5: Verify session was terminated.
	_, found = k.GetSession(ctx, homeID, sessionResp.SessionId)
	if found {
		t.Error("session should have been deleted after key revocation")
	}

	// Step 6: Verify key is marked as revoked.
	reg, found := k.GetKeyRegistration(ctx, homeID, "lifecycle-key-1")
	if !found {
		t.Fatal("key registration should still exist (revoked, not deleted)")
	}
	if !reg.Revoked {
		t.Error("key should be marked as revoked")
	}
	if reg.RevokedAt != 100 {
		t.Errorf("RevokedAt = %d, want 100", reg.RevokedAt)
	}

	// Step 7: Verify alert was created for revocation.
	alerts := k.GetAlertsByHome(ctx, homeID)
	found = false
	for _, a := range alerts {
		if a.AlertType == "key_revoked" {
			found = true
			if a.Priority != "medium" {
				t.Errorf("alert priority = %q, want medium", a.Priority)
			}
		}
	}
	if !found {
		t.Error("key_revoked alert not found")
	}

	// Step 8: Cannot start session with revoked key.
	_, err = msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               homeID,
		KeyHash:              "lifecycle-key-1",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Error("expected error starting session with revoked key")
	}
}

// ---------- Session Tests ----------

func TestStartSession(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("nancy")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Nancy Home"})
	homeID := resp.HomeId

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      homeID,
		KeyHash:     "session-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read", "write", "stake"},
	})

	sesResp, err := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               homeID,
		KeyHash:              "session-key",
		RequestedPermissions: []string{"read", "stake"},
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	session, found := k.GetSession(ctx, homeID, sesResp.SessionId)
	if !found {
		t.Fatal("session not found")
	}
	// Permission intersection: requested {read, stake} ∩ available {read, write, stake} = {read, stake}.
	if len(session.Permissions) != 2 {
		t.Errorf("len(Permissions) = %d, want 2", len(session.Permissions))
	}
	if session.ExpiresAt != 100+1000 {
		t.Errorf("ExpiresAt = %d, want %d", session.ExpiresAt, 100+1000)
	}
}

func TestStartSession_NoRequestedPerms_GetsAll(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("olive")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Olive Home"})
	homeID := resp.HomeId

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      homeID,
		KeyHash:     "all-perms-key",
		KeyType:     "cosmos",
		Role:        "admin",
		Permissions: []string{"read", "write", "transfer"},
	})

	sesResp, err := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:  owner,
		HomeId:  homeID,
		KeyHash: "all-perms-key",
		// No requested permissions → gets all available.
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	session, _ := k.GetSession(ctx, homeID, sesResp.SessionId)
	if len(session.Permissions) != 3 {
		t.Errorf("should get all 3 permissions, got %d", len(session.Permissions))
	}
}

func TestStartSession_ExpiredKey(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("pat")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Pat Home"})

	// Register key that expires at block 50 (already expired at block 100).
	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "expired-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
		ExpiresAt:   50,
	})

	_, err := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "expired-key",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Error("expected error for expired key")
	}
}

func TestStartSession_RevokedKey(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("quinn")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Quinn Home"})

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "revoked-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	msgSrv.RevokeKey(ctx, &types.MsgRevokeKey{
		Owner:   owner,
		HomeId:  resp.HomeId,
		KeyHash: "revoked-key",
	})

	_, err := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "revoked-key",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Error("expected error for revoked key")
	}
}

func TestStartSession_MaxSessionsEnforced(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("rita")
	bk.setBalance(owner, "uzrn", 50_000_000)

	// Set max sessions to 2.
	params := k.GetParams(ctx)
	params.MaxSessionsPerHome = 2
	k.SetParams(ctx, params)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Rita Home"})

	// Register 3 keys to start 3 sessions.
	for i := 0; i < 3; i++ {
		msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
			Owner:       owner,
			HomeId:      resp.HomeId,
			KeyHash:     fmt.Sprintf("ses-key-%d", i),
			KeyType:     "cosmos",
			Role:        "operator",
			Permissions: []string{"read"},
		})
	}

	// Start up to max (use different block heights for unique session IDs).
	for i := 0; i < 2; i++ {
		newCtx := ctx.WithBlockHeight(int64(100 + i))
		_, err := msgSrv.StartSession(newCtx, &types.MsgStartSession{
			Signer:               owner,
			HomeId:               resp.HomeId,
			KeyHash:              fmt.Sprintf("ses-key-%d", i),
			RequestedPermissions: []string{"read"},
		})
		if err != nil {
			t.Fatalf("StartSession %d failed: %v", i, err)
		}
	}

	// 3rd session should fail.
	newCtx := ctx.WithBlockHeight(102)
	_, err := msgSrv.StartSession(newCtx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "ses-key-2",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Error("expected error for exceeding max sessions")
	}
}

func TestStartSession_UnregisteredKey(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("sam")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Sam Home"})

	_, err := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "nonexistent-key",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Error("expected error for unregistered key")
	}
}

func TestEndSession(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("tina")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Tina Home"})

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "end-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	sesResp, _ := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "end-key",
		RequestedPermissions: []string{"read"},
	})

	_, err := msgSrv.EndSession(ctx, &types.MsgEndSession{
		Signer:    owner,
		HomeId:    resp.HomeId,
		SessionId: sesResp.SessionId,
	})
	if err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	_, found := k.GetSession(ctx, resp.HomeId, sesResp.SessionId)
	if found {
		t.Error("session should have been deleted")
	}
}

func TestEndSession_NotFound(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("uma")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Uma Home"})

	_, err := msgSrv.EndSession(ctx, &types.MsgEndSession{
		Signer:    owner,
		HomeId:    resp.HomeId,
		SessionId: "nonexistent-session",
	})
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

// ---------- Revoke Key Tests ----------

func TestRevokeKey_EndsAllActiveSessions(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("victor")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Victor Home"})
	homeID := resp.HomeId

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      homeID,
		KeyHash:     "multi-ses-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read", "write"},
	})

	// Start 2 sessions with the same key at different heights.
	ctx1 := ctx.WithBlockHeight(101)
	ses1, _ := msgSrv.StartSession(ctx1, &types.MsgStartSession{
		Signer:  owner,
		HomeId:  homeID,
		KeyHash: "multi-ses-key",
	})
	ctx2 := ctx.WithBlockHeight(102)
	ses2, _ := msgSrv.StartSession(ctx2, &types.MsgStartSession{
		Signer:  owner,
		HomeId:  homeID,
		KeyHash: "multi-ses-key",
	})

	// Revoke the key.
	msgSrv.RevokeKey(ctx, &types.MsgRevokeKey{
		Owner:   owner,
		HomeId:  homeID,
		KeyHash: "multi-ses-key",
	})

	// Both sessions should be deleted.
	_, found1 := k.GetSession(ctx, homeID, ses1.SessionId)
	_, found2 := k.GetSession(ctx, homeID, ses2.SessionId)
	if found1 || found2 {
		t.Error("all sessions should be deleted after key revocation")
	}
}

func TestRevokeKey_KeyNotFound(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("wendy")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Wendy Home"})

	_, err := msgSrv.RevokeKey(ctx, &types.MsgRevokeKey{
		Owner:   owner,
		HomeId:  resp.HomeId,
		KeyHash: "nonexistent-key",
	})
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

// ---------- Deadman Switch Tests ----------

func TestDeadmanSwitch_Triggers(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("xena")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Xena Home"})
	homeID := resp.HomeId

	// Configure deadman with threshold of 200 blocks.
	msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:  owner,
		HomeId: homeID,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 200,
			Action:              "alert_guardians",
		},
	})

	// Simulate inactivity: advance to block 100 + 201 = 301.
	deadmanCtx := ctx.WithBlockHeight(301)
	err := k.BeginBlocker(deadmanCtx)
	if err != nil {
		t.Fatalf("BeginBlocker: %v", err)
	}

	// Home should be "guarded".
	home, _ := k.GetHome(ctx, homeID)
	if home.Status != "guarded" {
		t.Errorf("Status = %q, want guarded", home.Status)
	}

	// Alert should be created.
	alerts := k.GetAlertsByHome(ctx, homeID)
	var deadmanAlert *types.Alert
	for _, a := range alerts {
		if a.AlertType == "deadman_triggered" {
			deadmanAlert = a
			break
		}
	}
	if deadmanAlert == nil {
		t.Fatal("deadman_triggered alert not created")
	}
	if deadmanAlert.Priority != "critical" {
		t.Errorf("alert priority = %q, want critical", deadmanAlert.Priority)
	}
}

func TestDeadmanSwitch_NotTriggered_StillActive(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("yuri")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Yuri Home"})

	msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:  owner,
		HomeId: resp.HomeId,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 500,
			Action:              "alert_guardians",
		},
	})

	// At block 200, only 100 blocks of inactivity (< 500 threshold).
	activeCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(activeCtx)

	home, _ := k.GetHome(ctx, resp.HomeId)
	if home.Status != "active" {
		t.Errorf("Status = %q, should still be active", home.Status)
	}
}

func TestDeadmanSwitch_SkipsArchived(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("zara")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Zara Home"})

	msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:  owner,
		HomeId: resp.HomeId,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 200,
			Action:              "alert_guardians",
		},
	})

	// Archive the home.
	home, _ := k.GetHome(ctx, resp.HomeId)
	home.Status = "archived"
	k.SetHome(ctx, home)

	// Deadman should not trigger on archived homes.
	deadmanCtx := ctx.WithBlockHeight(500)
	k.BeginBlocker(deadmanCtx)

	home, _ = k.GetHome(ctx, resp.HomeId)
	if home.Status != "archived" {
		t.Errorf("Status = %q, archived home should not be changed by deadman", home.Status)
	}
}

func TestDeadmanSwitch_DisabledDoesNotTrigger(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("ann")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Ann Home"})

	// Configure guardian with deadman disabled.
	msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:  owner,
		HomeId: resp.HomeId,
		Deadman: &types.DeadmanConfig{
			Enabled:             false,
			InactivityThreshold: 200,
		},
	})

	// Advance far beyond threshold.
	k.BeginBlocker(ctx.WithBlockHeight(10000))

	home, _ := k.GetHome(ctx, resp.HomeId)
	if home.Status != "active" {
		t.Errorf("Status = %q, should remain active when deadman disabled", home.Status)
	}
}

// ---------- Session Expiry in BeginBlocker Tests ----------

func TestCleanupExpiredSessions(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("brian")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Brian Home"})
	homeID := resp.HomeId

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      homeID,
		KeyHash:     "ses-exp-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	// Start session (expires at 100 + 1000 = 1100).
	sesResp, _ := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               homeID,
		KeyHash:              "ses-exp-key",
		RequestedPermissions: []string{"read"},
	})

	// At block 1100, session not yet expired (height > ExpiresAt needed).
	k.BeginBlocker(ctx.WithBlockHeight(1100))
	_, found := k.GetSession(ctx, homeID, sesResp.SessionId)
	if !found {
		t.Error("session should still exist at block 1100 (not > 1100)")
	}

	// At block 1101, session is expired.
	k.BeginBlocker(ctx.WithBlockHeight(1101))
	_, found = k.GetSession(ctx, homeID, sesResp.SessionId)
	if found {
		t.Error("session should be deleted after expiry")
	}

	// Verify session_expired alert.
	alerts := k.GetAlertsByHome(ctx, homeID)
	var expiredAlert *types.Alert
	for _, a := range alerts {
		if a.AlertType == "session_expired" {
			expiredAlert = a
			break
		}
	}
	if expiredAlert == nil {
		t.Fatal("session_expired alert not created")
	}
	if expiredAlert.Priority != "low" {
		t.Errorf("alert priority = %q, want low", expiredAlert.Priority)
	}
}

func TestCleanupExpiredSessions_KeepsValidSessions(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("claire")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Claire Home"})
	homeID := resp.HomeId

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      homeID,
		KeyHash:     "keep-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	// Start session at block 100, expires at 100+1000=1100.
	sesResp, _ := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:  owner,
		HomeId:  homeID,
		KeyHash: "keep-key",
	})

	// Run BeginBlocker at block 500, well before expiry.
	k.BeginBlocker(ctx.WithBlockHeight(500))

	_, found := k.GetSession(ctx, homeID, sesResp.SessionId)
	if !found {
		t.Error("valid session should not be cleaned up")
	}
}

func TestCleanupExpiredSessions_SkipsArchived(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Directly create archived home with an expired session.
	home := &types.AgentHome{
		HomeId:          "archived-home",
		OwnerAddress:    testAddr("donna"),
		Status:          "archived",
		LastActiveBlock: 100,
	}
	k.SetHome(ctx, home)

	session := &types.ActiveSession{
		SessionId: "old-session",
		HomeId:    "archived-home",
		KeyHash:   "somekey",
		ExpiresAt: 50, // Already expired.
	}
	k.SetSession(ctx, session)

	k.BeginBlocker(ctx.WithBlockHeight(200))

	// Session should still exist because archived homes are skipped.
	_, found := k.GetSession(ctx, "archived-home", "old-session")
	if !found {
		t.Error("sessions on archived homes should not be cleaned up")
	}
}

// ---------- Spending Limit Tests ----------

func TestSetSpendingLimit(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("edna")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Edna Home"})

	_, err := msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner:        owner,
		HomeId:       resp.HomeId,
		KeyType:      "ethereum",
		MaxAmount:    "5000000",
		PeriodBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("SetSpendingLimit: %v", err)
	}

	limit, found := k.GetSpendingLimit(ctx, resp.HomeId, "ethereum")
	if !found {
		t.Fatal("spending limit not found")
	}
	if limit.MaxAmount != "5000000" {
		t.Errorf("MaxAmount = %q, want 5000000", limit.MaxAmount)
	}
	if limit.PeriodBlocks != 1000 {
		t.Errorf("PeriodBlocks = %d, want 1000", limit.PeriodBlocks)
	}
	if limit.SpentInPeriod != "0" {
		t.Errorf("SpentInPeriod = %q, want 0", limit.SpentInPeriod)
	}
	if limit.PeriodStart != 100 {
		t.Errorf("PeriodStart = %d, want 100", limit.PeriodStart)
	}
}

func TestSetSpendingLimit_InvalidAmount(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("fiona")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Fiona Home"})

	// Zero amount.
	_, err := msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner:        owner,
		HomeId:       resp.HomeId,
		KeyType:      "cosmos",
		MaxAmount:    "0",
		PeriodBlocks: 100,
	})
	if err == nil {
		t.Error("expected error for zero amount")
	}

	// Negative amount.
	_, err = msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner:        owner,
		HomeId:       resp.HomeId,
		KeyType:      "cosmos",
		MaxAmount:    "-100",
		PeriodBlocks: 100,
	})
	if err == nil {
		t.Error("expected error for negative amount")
	}

	// Non-numeric amount.
	_, err = msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner:        owner,
		HomeId:       resp.HomeId,
		KeyType:      "cosmos",
		MaxAmount:    "abc",
		PeriodBlocks: 100,
	})
	if err == nil {
		t.Error("expected error for non-numeric amount")
	}
}

func TestSetSpendingLimit_ZeroPeriod(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("gina")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Gina Home"})

	_, err := msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner:        owner,
		HomeId:       resp.HomeId,
		KeyType:      "cosmos",
		MaxAmount:    "1000000",
		PeriodBlocks: 0,
	})
	if err == nil {
		t.Error("expected error for zero period_blocks")
	}
}

func TestGetSpendingLimitsByHome(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("hannah")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Hannah Home"})

	// Set limits for 2 key types.
	msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner: owner, HomeId: resp.HomeId, KeyType: "ethereum",
		MaxAmount: "1000000", PeriodBlocks: 100,
	})
	msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner: owner, HomeId: resp.HomeId, KeyType: "cosmos",
		MaxAmount: "2000000", PeriodBlocks: 200,
	})

	limits := k.GetSpendingLimitsByHome(ctx, resp.HomeId)
	if len(limits) != 2 {
		t.Errorf("got %d limits, want 2", len(limits))
	}
}

// ---------- Guardian Config Tests ----------

func TestConfigureGuardian(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("iris")
	guardian := testAddr("guardian1")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Iris Home"})

	_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:             owner,
		HomeId:            resp.HomeId,
		DefenseStrategy:   "conservative",
		AutoDefend:        true,
		RecoveryAddresses: []string{testAddr("recovery1"), testAddr("recovery2")},
		RecoveryThreshold: 2,
		GuardianAddress:   guardian,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 5000,
			Action:              "transfer_to_beneficiary",
			BeneficiaryAddress:  testAddr("beneficiary"),
		},
	})
	if err != nil {
		t.Fatalf("ConfigureGuardian: %v", err)
	}

	home, _ := k.GetHome(ctx, resp.HomeId)
	if home.Guardian.DefenseStrategy != "conservative" {
		t.Errorf("DefenseStrategy = %q, want conservative", home.Guardian.DefenseStrategy)
	}
	if !home.Guardian.AutoDefend {
		t.Error("AutoDefend should be true")
	}
	if home.Guardian.GuardianAddress != guardian {
		t.Errorf("GuardianAddress mismatch")
	}
	if len(home.Guardian.RecoveryAddresses) != 2 {
		t.Errorf("RecoveryAddresses len = %d, want 2", len(home.Guardian.RecoveryAddresses))
	}
	if home.Guardian.RecoveryThreshold != 2 {
		t.Errorf("RecoveryThreshold = %d, want 2", home.Guardian.RecoveryThreshold)
	}
	if home.Guardian.Deadman == nil || !home.Guardian.Deadman.Enabled {
		t.Error("Deadman should be enabled")
	}
}

func TestConfigureGuardian_InvalidStrategy(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("julia")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Julia Home"})

	_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:           owner,
		HomeId:          resp.HomeId,
		DefenseStrategy: "berserk",
	})
	if err == nil {
		t.Error("expected error for invalid defense_strategy")
	}
}

func TestConfigureGuardian_ValidStrategies(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("kate")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Kate Home"})

	for _, strategy := range []string{"aggressive", "moderate", "conservative", "diplomatic"} {
		_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
			Owner:           owner,
			HomeId:          resp.HomeId,
			DefenseStrategy: strategy,
		})
		if err != nil {
			t.Errorf("strategy %q failed: %v", strategy, err)
		}
		home, _ := k.GetHome(ctx, resp.HomeId)
		if home.Guardian.DefenseStrategy != strategy {
			t.Errorf("got %q, want %q", home.Guardian.DefenseStrategy, strategy)
		}
	}
}

func TestConfigureGuardian_DeadmanBelowMin(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("lara")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Lara Home"})

	// Default DeadmanMinThreshold is 100.
	_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:  owner,
		HomeId: resp.HomeId,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 50, // Below min (100).
		},
	})
	if err == nil {
		t.Error("expected error for deadman threshold below minimum")
	}
}

func TestConfigureGuardian_DeadmanAboveMax(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("mary")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Mary Home"})

	// Default DeadmanMaxThreshold is 100000.
	_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:  owner,
		HomeId: resp.HomeId,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 200000, // Above max.
		},
	})
	if err == nil {
		t.Error("expected error for deadman threshold above maximum")
	}
}

func TestConfigureGuardian_TooManyRecoveryAddresses(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("nora")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Nora Home"})

	// Default MaxRecoveryAddresses is 5.
	addrs := make([]string, 6)
	for i := range addrs {
		addrs[i] = testAddr(fmt.Sprintf("recovery-%d", i))
	}

	_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:             owner,
		HomeId:            resp.HomeId,
		RecoveryAddresses: addrs,
		RecoveryThreshold: 3,
	})
	if err == nil {
		t.Error("expected error for too many recovery addresses")
	}
}

func TestConfigureGuardian_ThresholdExceedsAddresses(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("ophelia")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Ophelia Home"})

	_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:             owner,
		HomeId:            resp.HomeId,
		RecoveryAddresses: []string{testAddr("r1"), testAddr("r2")},
		RecoveryThreshold: 5, // More than 2 addresses.
	})
	if err == nil {
		t.Error("expected error for threshold exceeding address count")
	}
}

// ---------- Owner-Only Access Control Tests ----------

func TestOwnerOnly_UpdateHome(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner-a")
	attacker := testAddr("attacker-a")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Owner A Home"})

	_, err := msgSrv.UpdateHome(ctx, &types.MsgUpdateHome{
		Owner:  attacker,
		HomeId: resp.HomeId,
		Name:   "Hacked",
	})
	if err == nil {
		t.Error("expected unauthorized error for non-owner UpdateHome")
	}
}

func TestOwnerOnly_RegisterKey(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner-b")
	attacker := testAddr("attacker-b")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Owner B Home"})

	_, err := msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       attacker,
		HomeId:      resp.HomeId,
		KeyHash:     "malicious-key",
		KeyType:     "cosmos",
		Role:        "admin",
		Permissions: []string{"all"},
	})
	if err == nil {
		t.Error("expected unauthorized error for non-owner RegisterKey")
	}
}

func TestOwnerOnly_RevokeKey(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner-c")
	attacker := testAddr("attacker-c")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Owner C Home"})

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "my-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	_, err := msgSrv.RevokeKey(ctx, &types.MsgRevokeKey{
		Owner:   attacker,
		HomeId:  resp.HomeId,
		KeyHash: "my-key",
	})
	if err == nil {
		t.Error("expected unauthorized error for non-owner RevokeKey")
	}
}

func TestOwnerOnly_ConfigureGuardian(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner-d")
	attacker := testAddr("attacker-d")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Owner D Home"})

	_, err := msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:           attacker,
		HomeId:          resp.HomeId,
		DefenseStrategy: "aggressive",
	})
	if err == nil {
		t.Error("expected unauthorized error for non-owner ConfigureGuardian")
	}
}

func TestOwnerOnly_SetSpendingLimit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner-e")
	attacker := testAddr("attacker-e")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Owner E Home"})

	_, err := msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner:        attacker,
		HomeId:       resp.HomeId,
		KeyType:      "ethereum",
		MaxAmount:    "1000000",
		PeriodBlocks: 100,
	})
	if err == nil {
		t.Error("expected unauthorized error for non-owner SetSpendingLimit")
	}
}

func TestOwnerOnly_UpdateMemoryCID(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner-f")
	attacker := testAddr("attacker-f")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Owner F Home"})

	_, err := msgSrv.UpdateMemoryCID(ctx, &types.MsgUpdateMemoryCID{
		Owner:  attacker,
		HomeId: resp.HomeId,
		Cid:    "bafymalicious",
	})
	if err == nil {
		t.Error("expected unauthorized error for non-owner UpdateMemoryCID")
	}
}

// ---------- Alert Tests ----------

func TestAcknowledgeAlert_Owner(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("peter")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Peter Home"})

	// Create an alert directly.
	alert := &types.Alert{
		AlertId:   "test-alert-1",
		HomeId:    resp.HomeId,
		AlertType: "test",
		Priority:  "medium",
		Message:   "Test alert",
		CreatedAt: 100,
	}
	k.SetAlert(ctx, alert)

	// Owner acknowledges.
	_, err := msgSrv.AcknowledgeAlert(ctx, &types.MsgAcknowledgeAlert{
		Signer:  owner,
		HomeId:  resp.HomeId,
		AlertId: "test-alert-1",
	})
	if err != nil {
		t.Fatalf("AcknowledgeAlert: %v", err)
	}

	ack, _ := k.GetAlert(ctx, resp.HomeId, "test-alert-1")
	if !ack.Acknowledged {
		t.Error("alert should be acknowledged")
	}
}

func TestAcknowledgeAlert_Guardian(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("rosa")
	guardian := testAddr("guardian-rosa")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Rosa Home"})

	// Configure guardian.
	msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:           owner,
		HomeId:          resp.HomeId,
		GuardianAddress: guardian,
	})

	// Create alert.
	k.SetAlert(ctx, &types.Alert{
		AlertId:   "guardian-alert",
		HomeId:    resp.HomeId,
		AlertType: "test",
		Priority:  "high",
		Message:   "Guardian test alert",
		CreatedAt: 100,
	})

	// Guardian acknowledges.
	_, err := msgSrv.AcknowledgeAlert(ctx, &types.MsgAcknowledgeAlert{
		Signer:  guardian,
		HomeId:  resp.HomeId,
		AlertId: "guardian-alert",
	})
	if err != nil {
		t.Fatalf("Guardian AcknowledgeAlert: %v", err)
	}

	ack, _ := k.GetAlert(ctx, resp.HomeId, "guardian-alert")
	if !ack.Acknowledged {
		t.Error("alert should be acknowledged by guardian")
	}
}

func TestAcknowledgeAlert_UnauthorizedThirdParty(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("sarah")
	thirdParty := testAddr("random-person")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Sarah Home"})

	k.SetAlert(ctx, &types.Alert{
		AlertId:   "private-alert",
		HomeId:    resp.HomeId,
		AlertType: "test",
		Priority:  "low",
		CreatedAt: 100,
	})

	_, err := msgSrv.AcknowledgeAlert(ctx, &types.MsgAcknowledgeAlert{
		Signer:  thirdParty,
		HomeId:  resp.HomeId,
		AlertId: "private-alert",
	})
	if err == nil {
		t.Error("expected unauthorized error for third-party alert acknowledgement")
	}
}

func TestAcknowledgeAlert_NotFound(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("tom")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Tom Home"})

	_, err := msgSrv.AcknowledgeAlert(ctx, &types.MsgAcknowledgeAlert{
		Signer:  owner,
		HomeId:  resp.HomeId,
		AlertId: "nonexistent-alert",
	})
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestCountPendingAlerts(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	homeID := "alert-count-home"
	k.SetHome(ctx, &types.AgentHome{HomeId: homeID, OwnerAddress: testAddr("ursula"), Status: "active"})

	k.SetAlert(ctx, &types.Alert{AlertId: "a1", HomeId: homeID, Acknowledged: false})
	k.SetAlert(ctx, &types.Alert{AlertId: "a2", HomeId: homeID, Acknowledged: true})
	k.SetAlert(ctx, &types.Alert{AlertId: "a3", HomeId: homeID, Acknowledged: false})

	count := k.CountPendingAlerts(ctx, homeID)
	if count != 2 {
		t.Errorf("pending alerts = %d, want 2", count)
	}
}

// ---------- Query Tests ----------

func TestQueryHome(t *testing.T) {
	qSrv, k, ctx, _ := setupQueryServer(t)

	k.SetHome(ctx, &types.AgentHome{
		HomeId:       "q-home-1",
		OwnerAddress: testAddr("query-owner"),
		Name:         "Query Home",
		Status:       "active",
	})

	resp, err := qSrv.Home(ctx, &types.QueryHomeRequest{HomeId: "q-home-1"})
	if err != nil {
		t.Fatalf("QueryHome: %v", err)
	}
	if resp.Home.Name != "Query Home" {
		t.Errorf("Name = %q, want Query Home", resp.Home.Name)
	}
}

func TestQueryHome_NotFound(t *testing.T) {
	qSrv, _, ctx, _ := setupQueryServer(t)

	_, err := qSrv.Home(ctx, &types.QueryHomeRequest{HomeId: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent home")
	}
}

func TestQueryHomesByOwner(t *testing.T) {
	qSrv, k, ctx, _ := setupQueryServer(t)
	owner := testAddr("multi-owner")

	k.SetHome(ctx, &types.AgentHome{HomeId: "ho-1", OwnerAddress: owner, Status: "active"})
	k.SetHome(ctx, &types.AgentHome{HomeId: "ho-2", OwnerAddress: owner, Status: "dormant"})
	k.AddHomeToOwnerIndex(ctx, owner, "ho-1")
	k.AddHomeToOwnerIndex(ctx, owner, "ho-2")

	resp, err := qSrv.HomesByOwner(ctx, &types.QueryHomesByOwnerRequest{Owner: owner})
	if err != nil {
		t.Fatalf("QueryHomesByOwner: %v", err)
	}
	if len(resp.Homes) != 2 {
		t.Errorf("got %d homes, want 2", len(resp.Homes))
	}
}

func TestQueryKeys(t *testing.T) {
	qSrv, k, ctx, _ := setupQueryServer(t)

	k.SetKeyRegistration(ctx, "key-home", &types.KeyRegistration{
		KeyHash: "k1", KeyType: "cosmos", Role: "admin",
	})
	k.SetKeyRegistration(ctx, "key-home", &types.KeyRegistration{
		KeyHash: "k2", KeyType: "ethereum", Role: "operator",
	})

	resp, err := qSrv.Keys(ctx, &types.QueryKeysRequest{HomeId: "key-home"})
	if err != nil {
		t.Fatalf("QueryKeys: %v", err)
	}
	if len(resp.Keys) != 2 {
		t.Errorf("got %d keys, want 2", len(resp.Keys))
	}
}

func TestQuerySessions(t *testing.T) {
	qSrv, k, ctx, _ := setupQueryServer(t)

	k.SetSession(ctx, &types.ActiveSession{
		SessionId: "ses-1", HomeId: "ses-home", KeyHash: "k1", ExpiresAt: 200,
	})

	resp, err := qSrv.Sessions(ctx, &types.QuerySessionsRequest{HomeId: "ses-home"})
	if err != nil {
		t.Fatalf("QuerySessions: %v", err)
	}
	if len(resp.Sessions) != 1 {
		t.Errorf("got %d sessions, want 1", len(resp.Sessions))
	}
}

func TestQueryAlerts(t *testing.T) {
	qSrv, k, ctx, _ := setupQueryServer(t)

	k.SetAlert(ctx, &types.Alert{AlertId: "al-1", HomeId: "alert-home", Acknowledged: false})
	k.SetAlert(ctx, &types.Alert{AlertId: "al-2", HomeId: "alert-home", Acknowledged: true})

	// All alerts.
	resp, err := qSrv.Alerts(ctx, &types.QueryAlertsRequest{HomeId: "alert-home"})
	if err != nil {
		t.Fatalf("QueryAlerts: %v", err)
	}
	if len(resp.Alerts) != 2 {
		t.Errorf("got %d alerts, want 2", len(resp.Alerts))
	}

	// Unacknowledged only.
	resp2, err := qSrv.Alerts(ctx, &types.QueryAlertsRequest{HomeId: "alert-home", UnacknowledgedOnly: true})
	if err != nil {
		t.Fatalf("QueryAlerts (unack): %v", err)
	}
	if len(resp2.Alerts) != 1 {
		t.Errorf("got %d unack alerts, want 1", len(resp2.Alerts))
	}
}

func TestQuerySpendingLimits(t *testing.T) {
	qSrv, k, ctx, _ := setupQueryServer(t)

	k.SetSpendingLimit(ctx, "sl-home", &types.SpendingLimit{
		KeyType: "ethereum", MaxAmount: "1000000", PeriodBlocks: 100,
	})

	resp, err := qSrv.SpendingLimits(ctx, &types.QuerySpendingLimitsRequest{HomeId: "sl-home"})
	if err != nil {
		t.Fatalf("QuerySpendingLimits: %v", err)
	}
	if len(resp.Limits) != 1 {
		t.Errorf("got %d limits, want 1", len(resp.Limits))
	}
}

func TestQueryParams(t *testing.T) {
	qSrv, _, ctx, _ := setupQueryServer(t)

	resp, err := qSrv.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("QueryParams: %v", err)
	}
	if resp.Params.MaxKeysPerHome != 20 {
		t.Errorf("MaxKeysPerHome = %d, want 20", resp.Params.MaxKeysPerHome)
	}
}

// ---------- Genesis Tests ----------

func TestInitExportGenesis(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Initialize with custom state.
	genState := &types.GenesisState{
		Params: &types.Params{
			MaxKeysPerHome:       15,
			MaxSessionsPerHome:   10,
			SessionTimeoutBlocks: 2000,
			DeadmanMinThreshold:  200,
			DeadmanMaxThreshold:  80000,
			MaxAlertsPerHome:     50,
			HomeCreationFee:      "20000000",
			MaxRecoveryAddresses: 3,
		},
		Homes: []*types.AgentHome{
			{HomeId: "gen-home-1", OwnerAddress: testAddr("gen-owner"), Name: "Genesis Home", Status: "active"},
		},
		KeySets: []*types.HomeKeySet{
			{HomeId: "gen-home-1", Keys: []*types.KeyRegistration{
				{KeyHash: "gen-key-1", KeyType: "cosmos", Role: "admin"},
			}},
		},
	}

	k.InitGenesis(ctx, genState)

	// Verify state.
	params := k.GetParams(ctx)
	if params.MaxKeysPerHome != 15 {
		t.Errorf("MaxKeysPerHome = %d, want 15", params.MaxKeysPerHome)
	}

	home, found := k.GetHome(ctx, "gen-home-1")
	if !found {
		t.Fatal("genesis home not found")
	}
	if home.Name != "Genesis Home" {
		t.Errorf("Name = %q, want Genesis Home", home.Name)
	}

	keys := k.GetKeysByHome(ctx, "gen-home-1")
	if len(keys) != 1 {
		t.Errorf("got %d keys, want 1", len(keys))
	}

	// Export and verify roundtrip.
	exported := k.ExportGenesis(ctx)
	if exported.Params.MaxKeysPerHome != 15 {
		t.Errorf("exported MaxKeysPerHome = %d, want 15", exported.Params.MaxKeysPerHome)
	}
	if len(exported.Homes) != 1 {
		t.Errorf("exported homes = %d, want 1", len(exported.Homes))
	}
	if len(exported.KeySets) != 1 {
		t.Errorf("exported key sets = %d, want 1", len(exported.KeySets))
	}
}

func TestGenesisValidation(t *testing.T) {
	// Valid genesis.
	gs := types.DefaultGenesis()
	if err := gs.Validate(); err != nil {
		t.Errorf("valid genesis failed: %v", err)
	}

	// Duplicate home IDs.
	dup := &types.GenesisState{
		Params: types.DefaultParams(),
		Homes: []*types.AgentHome{
			{HomeId: "dup-1"},
			{HomeId: "dup-1"},
		},
	}
	if err := dup.Validate(); err == nil {
		t.Error("expected error for duplicate home IDs")
	}
}

// ---------- State CRUD Tests ----------

func TestHomeCRUD(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	home := &types.AgentHome{
		HomeId:       "crud-home",
		OwnerAddress: testAddr("crud-owner"),
		Name:         "CRUD Home",
		Status:       "active",
	}

	// Set.
	k.SetHome(ctx, home)

	// Get.
	got, found := k.GetHome(ctx, "crud-home")
	if !found {
		t.Fatal("home not found after Set")
	}
	if got.Name != "CRUD Home" {
		t.Errorf("Name = %q, want CRUD Home", got.Name)
	}

	// GetAll.
	all := k.GetAllHomes(ctx)
	if len(all) != 1 {
		t.Errorf("GetAllHomes = %d, want 1", len(all))
	}

	// Not found.
	_, found = k.GetHome(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent home")
	}
}

func TestSessionCRUD(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	session := &types.ActiveSession{
		SessionId: "ses-crud-1",
		HomeId:    "crud-home",
		KeyHash:   "key1",
		ExpiresAt: 200,
	}

	// Set.
	k.SetSession(ctx, session)

	// Get.
	got, found := k.GetSession(ctx, "crud-home", "ses-crud-1")
	if !found {
		t.Fatal("session not found")
	}
	if got.KeyHash != "key1" {
		t.Errorf("KeyHash = %q, want key1", got.KeyHash)
	}

	// Count.
	count := k.CountSessions(ctx, "crud-home")
	if count != 1 {
		t.Errorf("CountSessions = %d, want 1", count)
	}

	// Delete.
	k.DeleteSession(ctx, "crud-home", "ses-crud-1")
	_, found = k.GetSession(ctx, "crud-home", "ses-crud-1")
	if found {
		t.Error("session should be deleted")
	}
}

func TestKeyRegistrationCRUD(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	reg := &types.KeyRegistration{
		KeyHash:     "keyreg-1",
		KeyType:     "cosmos",
		Role:        "admin",
		Permissions: []string{"all"},
	}

	k.SetKeyRegistration(ctx, "reg-home", reg)

	got, found := k.GetKeyRegistration(ctx, "reg-home", "keyreg-1")
	if !found {
		t.Fatal("key not found")
	}
	if got.Role != "admin" {
		t.Errorf("Role = %q, want admin", got.Role)
	}

	// Count active keys.
	count := k.CountActiveKeys(ctx, "reg-home")
	if count != 1 {
		t.Errorf("CountActiveKeys = %d, want 1", count)
	}

	// Revoke and count again.
	got.Revoked = true
	k.SetKeyRegistration(ctx, "reg-home", got)
	count = k.CountActiveKeys(ctx, "reg-home")
	if count != 0 {
		t.Errorf("CountActiveKeys after revoke = %d, want 0", count)
	}
}

func TestAlertCRUD(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	alert := &types.Alert{
		AlertId:   "alert-crud-1",
		HomeId:    "alert-home",
		AlertType: "test",
		Priority:  "low",
	}

	k.SetAlert(ctx, alert)

	got, found := k.GetAlert(ctx, "alert-home", "alert-crud-1")
	if !found {
		t.Fatal("alert not found")
	}
	if got.Priority != "low" {
		t.Errorf("Priority = %q, want low", got.Priority)
	}

	all := k.GetAlertsByHome(ctx, "alert-home")
	if len(all) != 1 {
		t.Errorf("GetAlertsByHome = %d, want 1", len(all))
	}
}

func TestOwnerIndex(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	owner := testAddr("index-owner")

	k.AddHomeToOwnerIndex(ctx, owner, "idx-1")
	k.AddHomeToOwnerIndex(ctx, owner, "idx-2")
	k.AddHomeToOwnerIndex(ctx, owner, "idx-3")

	ids := k.GetHomesByOwner(ctx, owner)
	if len(ids) != 3 {
		t.Errorf("GetHomesByOwner = %d, want 3", len(ids))
	}

	// Different owner.
	other := testAddr("other-owner")
	otherIDs := k.GetHomesByOwner(ctx, other)
	if len(otherIDs) != 0 {
		t.Errorf("other owner has %d homes, want 0", len(otherIDs))
	}
}

func TestHomeIDCounter(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	id1 := k.GetNextHomeID(ctx)
	id2 := k.GetNextHomeID(ctx)
	id3 := k.GetNextHomeID(ctx)

	if id1 != "home-1" || id2 != "home-2" || id3 != "home-3" {
		t.Errorf("IDs = %s/%s/%s, want home-1/home-2/home-3", id1, id2, id3)
	}
}

func TestPartnershipLink(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetHome(ctx, &types.AgentHome{
		HomeId:       "link-home",
		OwnerAddress: testAddr("link-owner"),
		Status:       "active",
	})

	k.SetPartnershipOnHome(ctx, "link-home", "partnership-42")

	home, _ := k.GetHome(ctx, "link-home")
	if home.PartnershipId != "partnership-42" {
		t.Errorf("PartnershipId = %q, want partnership-42", home.PartnershipId)
	}
}

// ---------- Edge Case Tests ----------

func TestHomeNotFound_Operations(t *testing.T) {
	msgSrv, _, ctx, _ := setupMsgServer(t)
	owner := testAddr("ghost")

	_, err := msgSrv.UpdateHome(ctx, &types.MsgUpdateHome{Owner: owner, HomeId: "nonexistent"})
	if err == nil {
		t.Error("expected error for UpdateHome on nonexistent home")
	}

	_, err = msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{Owner: owner, HomeId: "nonexistent", KeyHash: "k1"})
	if err == nil {
		t.Error("expected error for RegisterKey on nonexistent home")
	}

	_, err = msgSrv.RevokeKey(ctx, &types.MsgRevokeKey{Owner: owner, HomeId: "nonexistent", KeyHash: "k1"})
	if err == nil {
		t.Error("expected error for RevokeKey on nonexistent home")
	}

	_, err = msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{Owner: owner, HomeId: "nonexistent"})
	if err == nil {
		t.Error("expected error for ConfigureGuardian on nonexistent home")
	}

	_, err = msgSrv.SetSpendingLimit(ctx, &types.MsgSetSpendingLimit{
		Owner: owner, HomeId: "nonexistent", KeyType: "eth", MaxAmount: "100", PeriodBlocks: 10,
	})
	if err == nil {
		t.Error("expected error for SetSpendingLimit on nonexistent home")
	}

	_, err = msgSrv.UpdateMemoryCID(ctx, &types.MsgUpdateMemoryCID{Owner: owner, HomeId: "nonexistent", Cid: "bafy"})
	if err == nil {
		t.Error("expected error for UpdateMemoryCID on nonexistent home")
	}

	_, err = msgSrv.StartSession(ctx, &types.MsgStartSession{Signer: owner, HomeId: "nonexistent", KeyHash: "k1"})
	if err == nil {
		t.Error("expected error for StartSession on nonexistent home")
	}

	_, err = msgSrv.AcknowledgeAlert(ctx, &types.MsgAcknowledgeAlert{Signer: owner, HomeId: "nonexistent", AlertId: "a1"})
	if err == nil {
		t.Error("expected error for AcknowledgeAlert on nonexistent home")
	}
}

func TestStartSession_UpdatesKeyLastUsedAt(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("lastused")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "LastUsed Home"})

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "lastused-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	newCtx := ctx.WithBlockHeight(150)
	msgSrv.StartSession(newCtx, &types.MsgStartSession{
		Signer:  owner,
		HomeId:  resp.HomeId,
		KeyHash: "lastused-key",
	})

	reg, _ := k.GetKeyRegistration(ctx, resp.HomeId, "lastused-key")
	if reg.LastUsedAt != 150 {
		t.Errorf("LastUsedAt = %d, want 150", reg.LastUsedAt)
	}
}

func TestStartSession_UpdatesHomeLastActiveBlock(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("homeactive")
	bk.setBalance(owner, "uzrn", 50_000_000)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Active Home"})

	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "active-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	newCtx := ctx.WithBlockHeight(250)
	msgSrv.StartSession(newCtx, &types.MsgStartSession{
		Signer:  owner,
		HomeId:  resp.HomeId,
		KeyHash: "active-key",
	})

	home, _ := k.GetHome(ctx, resp.HomeId)
	if home.LastActiveBlock != 250 {
		t.Errorf("LastActiveBlock = %d, want 250", home.LastActiveBlock)
	}
}

func TestBeginBlocker_DeadmanAndSessionExpiry(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("combo")
	bk.setBalance(owner, "uzrn", 50_000_000)

	// Set session timeout to 50 blocks for faster testing.
	params := k.GetParams(ctx)
	params.SessionTimeoutBlocks = 50
	k.SetParams(ctx, params)

	resp, _ := msgSrv.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "Combo Home"})
	homeID := resp.HomeId

	// Configure deadman with 200 block threshold.
	msgSrv.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:  owner,
		HomeId: homeID,
		Deadman: &types.DeadmanConfig{
			Enabled:             true,
			InactivityThreshold: 200,
			Action:              "alert",
		},
	})

	// Register key and start session at block 100.
	msgSrv.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      homeID,
		KeyHash:     "combo-key",
		KeyType:     "cosmos",
		Role:        "operator",
		Permissions: []string{"read"},
	})

	sesResp, _ := msgSrv.StartSession(ctx, &types.MsgStartSession{
		Signer:  owner,
		HomeId:  homeID,
		KeyHash: "combo-key",
	})

	// Block 151: session expires (100 + 50 = 150, 151 > 150), deadman not yet (100 + 200 = 300).
	k.BeginBlocker(ctx.WithBlockHeight(151))

	_, sesFound := k.GetSession(ctx, homeID, sesResp.SessionId)
	if sesFound {
		t.Error("session should be expired at block 151")
	}

	home, _ := k.GetHome(ctx, homeID)
	if home.Status != "active" {
		t.Errorf("home should still be active, got %q", home.Status)
	}

	// Block 301: deadman triggers (LastActiveBlock=100, 301 > 100+200).
	k.BeginBlocker(ctx.WithBlockHeight(301))

	home, _ = k.GetHome(ctx, homeID)
	if home.Status != "guarded" {
		t.Errorf("home should be guarded after deadman, got %q", home.Status)
	}
}

func TestSpendingLimitCRUD_DirectKeeper(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	limit := &types.SpendingLimit{
		KeyType:       "cosmos",
		MaxAmount:     "5000000",
		PeriodBlocks:  500,
		SpentInPeriod: "1000000",
		PeriodStart:   100,
	}

	k.SetSpendingLimit(ctx, "sl-home", limit)

	got, found := k.GetSpendingLimit(ctx, "sl-home", "cosmos")
	if !found {
		t.Fatal("spending limit not found")
	}
	if got.MaxAmount != "5000000" {
		t.Errorf("MaxAmount = %q, want 5000000", got.MaxAmount)
	}
	if got.SpentInPeriod != "1000000" {
		t.Errorf("SpentInPeriod = %q, want 1000000", got.SpentInPeriod)
	}

	// Not found.
	_, found = k.GetSpendingLimit(ctx, "sl-home", "bitcoin")
	if found {
		t.Error("should not find nonexistent key type")
	}
}

func TestMultipleHomesIndependentState(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create 2 homes.
	k.SetHome(ctx, &types.AgentHome{HomeId: "h1", OwnerAddress: testAddr("o1"), Status: "active"})
	k.SetHome(ctx, &types.AgentHome{HomeId: "h2", OwnerAddress: testAddr("o2"), Status: "active"})

	// Add keys to each.
	k.SetKeyRegistration(ctx, "h1", &types.KeyRegistration{KeyHash: "k1", KeyType: "cosmos"})
	k.SetKeyRegistration(ctx, "h2", &types.KeyRegistration{KeyHash: "k2", KeyType: "ethereum"})

	// Verify isolation.
	keys1 := k.GetKeysByHome(ctx, "h1")
	keys2 := k.GetKeysByHome(ctx, "h2")

	if len(keys1) != 1 || keys1[0].KeyHash != "k1" {
		t.Error("h1 keys are wrong")
	}
	if len(keys2) != 1 || keys2[0].KeyHash != "k2" {
		t.Error("h2 keys are wrong")
	}
}

// ---------- Tests: UpdateParams ----------

func TestUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.MaxKeysPerHome = 50
	newParams.SessionTimeoutBlocks = 2000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MaxKeysPerHome != 50 {
		t.Errorf("expected MaxKeysPerHome 50, got %d", got.MaxKeysPerHome)
	}
	if got.SessionTimeoutBlocks != 2000 {
		t.Errorf("expected SessionTimeoutBlocks 2000, got %d", got.SessionTimeoutBlocks)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr("wrongauthority"),
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestUpdateParamsNilParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    nil,
	})
	if err == nil {
		t.Fatal("expected nil params error")
	}
}

func TestUpdateParamsInvalid(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	badParams := types.DefaultParams()
	badParams.MaxKeysPerHome = 0 // invalid: must be > 0

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}
