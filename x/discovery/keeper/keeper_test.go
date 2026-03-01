package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/discovery/keeper"
	"github.com/zerone-chain/zerone/x/discovery/types"
)

// -----------------------------------------------------------------------
// Mock BankKeeper
// -----------------------------------------------------------------------

type mockBankKeeper struct {
	balances map[string]sdkmath.Int
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances: make(map[string]sdkmath.Int),
	}
}

func (m *mockBankKeeper) setBalance(addr sdk.AccAddress, denom string, amount sdkmath.Int) {
	m.balances[addr.String()+"/"+denom] = amount
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	for _, coin := range amt {
		key := senderAddr.String() + "/" + coin.Denom
		bal, ok := m.balances[key]
		if !ok {
			bal = sdkmath.ZeroInt()
		}
		if bal.LT(coin.Amount) {
			return fmt.Errorf("insufficient balance for send to module")
		}
		m.balances[key] = bal.Sub(coin.Amount)
		modKey := recipientModule + "/" + coin.Denom
		modBal, ok := m.balances[modKey]
		if !ok {
			modBal = sdkmath.ZeroInt()
		}
		m.balances[modKey] = modBal.Add(coin.Amount)
	}
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	for _, coin := range amt {
		modKey := senderModule + "/" + coin.Denom
		modBal, ok := m.balances[modKey]
		if !ok {
			modBal = sdkmath.ZeroInt()
		}
		if modBal.LT(coin.Amount) {
			return fmt.Errorf("insufficient module balance")
		}
		m.balances[modKey] = modBal.Sub(coin.Amount)

		key := recipientAddr.String() + "/" + coin.Denom
		bal, ok := m.balances[key]
		if !ok {
			bal = sdkmath.ZeroInt()
		}
		m.balances[key] = bal.Add(coin.Amount)
	}
	return nil
}

// -----------------------------------------------------------------------
// Test Setup
// -----------------------------------------------------------------------

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

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

	bk := newMockBankKeeper()

	authority := sdk.AccAddress([]byte("authority-addr------")).String()
	k := keeper.NewKeeper(runtime.NewKVStoreService(storeKey), cdc, authority, bk)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	return k, ctx, bk
}

func testAddr(i int) string {
	addr := sdk.AccAddress([]byte(fmt.Sprintf("test-addr-%010d", i)))
	return addr.String()
}

// -----------------------------------------------------------------------
// 1. TestSetGetParams
// -----------------------------------------------------------------------

func TestSetGetParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Default params should be returned when nothing is set.
	params := k.GetParams(ctx)
	if params.MinRegistrationStake != "1000000" {
		t.Errorf("expected MinRegistrationStake 1000000, got %s", params.MinRegistrationStake)
	}
	if params.MaxCapabilitiesPerAgent != 20 {
		t.Errorf("expected MaxCapabilitiesPerAgent 20, got %d", params.MaxCapabilitiesPerAgent)
	}
	if params.ProfileExpiryBlocks != 100000 {
		t.Errorf("expected ProfileExpiryBlocks 100000, got %d", params.ProfileExpiryBlocks)
	}

	// Set custom params, then read them back.
	custom := types.DefaultParams()
	custom.MinRegistrationStake = "5000000"
	custom.MaxCapabilitiesPerAgent = 10
	custom.ProfileExpiryBlocks = 50000
	k.SetParams(ctx, custom)

	got := k.GetParams(ctx)
	if got.MinRegistrationStake != "5000000" {
		t.Errorf("expected MinRegistrationStake 5000000, got %s", got.MinRegistrationStake)
	}
	if got.MaxCapabilitiesPerAgent != 10 {
		t.Errorf("expected MaxCapabilitiesPerAgent 10, got %d", got.MaxCapabilitiesPerAgent)
	}
	if got.ProfileExpiryBlocks != 50000 {
		t.Errorf("expected ProfileExpiryBlocks 50000, got %d", got.ProfileExpiryBlocks)
	}
}

// -----------------------------------------------------------------------
// 2. TestProfileCRUD
// -----------------------------------------------------------------------

func TestProfileCRUD(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	addr := testAddr(1)

	// Get on missing profile should return not-found.
	_, found := k.GetProfile(ctx, addr)
	if found {
		t.Error("expected profile not found")
	}

	// Set a profile.
	profile := &types.AgentProfile{
		Address:     addr,
		DisplayName: "Alice",
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference", Domains: []string{"math"}},
		},
		Domains:           []string{"mathematics", "physics"},
		Status:            "active",
		ReputationScore:   500000,
		RegisteredAtBlock: 100,
		LastActiveBlock:   100,
		Stake:             "1000000",
		Description:       "Agent Alice",
		Metadata:          `{"version":"1"}`,
	}
	k.SetProfile(ctx, profile)

	// Get should succeed.
	got, found := k.GetProfile(ctx, addr)
	if !found {
		t.Fatal("expected profile to be found")
	}
	if got.DisplayName != "Alice" {
		t.Errorf("expected display_name Alice, got %s", got.DisplayName)
	}
	if got.Stake != "1000000" {
		t.Errorf("expected stake 1000000, got %s", got.Stake)
	}
	if len(got.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(got.Domains))
	}
	if len(got.Capabilities) != 1 {
		t.Errorf("expected 1 capability, got %d", len(got.Capabilities))
	}

	// Delete profile.
	k.DeleteProfile(ctx, profile)
	_, found = k.GetProfile(ctx, addr)
	if found {
		t.Error("expected profile to be deleted")
	}

	// IterateProfiles / GetAllProfiles with multiple entries.
	for i := 0; i < 5; i++ {
		k.SetProfile(ctx, &types.AgentProfile{
			Address: testAddr(100 + i),
			Domains: []string{"test"},
			Status:  "active",
			Stake:   "1000000",
		})
	}
	all := k.GetAllProfiles(ctx)
	if len(all) != 5 {
		t.Errorf("expected 5 profiles, got %d", len(all))
	}

	// Iterate with early stop.
	count := 0
	k.IterateProfiles(ctx, func(p *types.AgentProfile) bool {
		count++
		return count >= 3
	})
	if count != 3 {
		t.Errorf("expected iterate to stop at 3, stopped at %d", count)
	}
}

// -----------------------------------------------------------------------
// 3. TestRegisterProfile
// -----------------------------------------------------------------------

func TestRegisterProfile(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(1)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(200000000))

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "TestAgent",
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference", Domains: []string{"mathematics"}},
			{CapabilityType: "verification", Domains: []string{"physics"}},
		},
		Domains:     []string{"mathematics", "physics"},
		Stake:       "10000000",
		Description: "A test agent",
		Metadata:    `{"version":"1"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify profile was stored correctly.
	profile, found := k.GetProfile(ctx, addr)
	if !found {
		t.Fatal("expected profile to be found")
	}
	if profile.Status != "active" {
		t.Errorf("expected status active, got %s", profile.Status)
	}
	if profile.ReputationScore != 500000 {
		t.Errorf("expected reputation 500000, got %d", profile.ReputationScore)
	}
	if profile.Stake != "10000000" {
		t.Errorf("expected stake 10000000, got %s", profile.Stake)
	}
	if profile.DisplayName != "TestAgent" {
		t.Errorf("expected display_name TestAgent, got %s", profile.DisplayName)
	}
	if profile.RegisteredAtBlock != 100 {
		t.Errorf("expected registered_at_block 100, got %d", profile.RegisteredAtBlock)
	}
	if profile.LastActiveBlock != 100 {
		t.Errorf("expected last_active_block 100, got %d", profile.LastActiveBlock)
	}
	if len(profile.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(profile.Capabilities))
	}

	// Verify domain index works.
	mathProfiles := k.GetProfilesByDomain(ctx, "mathematics")
	if len(mathProfiles) != 1 {
		t.Errorf("expected 1 math profile, got %d", len(mathProfiles))
	}
	physicsProfiles := k.GetProfilesByDomain(ctx, "physics")
	if len(physicsProfiles) != 1 {
		t.Errorf("expected 1 physics profile, got %d", len(physicsProfiles))
	}

	// Verify capability index works.
	inferenceProfiles := k.GetProfilesByCapability(ctx, "inference")
	if len(inferenceProfiles) != 1 {
		t.Errorf("expected 1 inference profile, got %d", len(inferenceProfiles))
	}

	// Verify stake was deducted from the sender.
	remainingBal := bk.balances[accAddr.String()+"/uzrn"]
	expected := sdkmath.NewInt(190000000)
	if !remainingBal.Equal(expected) {
		t.Errorf("expected sender balance %s, got %s", expected, remainingBal)
	}

	// Verify module account received the stake.
	moduleBal := bk.balances[types.ModuleName+"/uzrn"]
	expectedModule := sdkmath.NewInt(10000000)
	if !moduleBal.Equal(expectedModule) {
		t.Errorf("expected module balance %s, got %s", expectedModule, moduleBal)
	}
}

// -----------------------------------------------------------------------
// 4. TestRegisterProfileInsufficientStake
// -----------------------------------------------------------------------

func TestRegisterProfileInsufficientStake(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(1)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(200000000))

	srv := keeper.NewMsgServerImpl(k)

	// Default MinRegistrationStake is "1000000", try with less.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr,
		Domains: []string{"mathematics"},
		Stake:   "500",
	})
	if err == nil {
		t.Error("expected insufficient stake error")
	}

	// Verify the profile was NOT created.
	_, found := k.GetProfile(ctx, addr)
	if found {
		t.Error("expected profile not to be created with insufficient stake")
	}
}

// -----------------------------------------------------------------------
// 5. TestRegisterProfileAlreadyExists
// -----------------------------------------------------------------------

func TestRegisterProfileAlreadyExists(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(1)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	// First registration succeeds.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "Agent1",
		Domains:     []string{"mathematics"},
		Stake:       "1000000",
	})
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration with same address should fail.
	_, err = srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "Agent1Duplicate",
		Domains:     []string{"physics"},
		Stake:       "1000000",
	})
	if err == nil {
		t.Error("expected duplicate agent error")
	}

	// Original profile should be unchanged.
	profile, found := k.GetProfile(ctx, addr)
	if !found {
		t.Fatal("expected original profile to still exist")
	}
	if profile.DisplayName != "Agent1" {
		t.Errorf("expected original display_name Agent1, got %s", profile.DisplayName)
	}
}

// -----------------------------------------------------------------------
// 6. TestUpdateProfile
// -----------------------------------------------------------------------

func TestUpdateProfile(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(1)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(200000000))

	srv := keeper.NewMsgServerImpl(k)

	// Register first.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "OriginalName",
		Domains:     []string{"mathematics"},
		Stake:       "1000000",
		Description: "Original description",
		Metadata:    `{"v":"1"}`,
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Update display_name only (other fields empty => unchanged).
	_, err = srv.UpdateProfile(ctx, &types.MsgUpdateProfile{
		Sender:      addr,
		DisplayName: "UpdatedName",
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	profile, _ := k.GetProfile(ctx, addr)
	if profile.DisplayName != "UpdatedName" {
		t.Errorf("expected display_name UpdatedName, got %s", profile.DisplayName)
	}
	if profile.Description != "Original description" {
		t.Errorf("expected description unchanged, got %s", profile.Description)
	}
	if profile.Metadata != `{"v":"1"}` {
		t.Errorf("expected metadata unchanged, got %s", profile.Metadata)
	}

	// Update description and metadata.
	_, err = srv.UpdateProfile(ctx, &types.MsgUpdateProfile{
		Sender:      addr,
		Description: "Updated description",
		Metadata:    `{"v":"2"}`,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	profile, _ = k.GetProfile(ctx, addr)
	if profile.DisplayName != "UpdatedName" {
		t.Errorf("expected display_name UpdatedName (unchanged), got %s", profile.DisplayName)
	}
	if profile.Description != "Updated description" {
		t.Errorf("expected Updated description, got %s", profile.Description)
	}
	if profile.Metadata != `{"v":"2"}` {
		t.Errorf("expected metadata v2, got %s", profile.Metadata)
	}

	// Update for non-existent profile should fail.
	_, err = srv.UpdateProfile(ctx, &types.MsgUpdateProfile{
		Sender:      testAddr(99),
		DisplayName: "Ghost",
	})
	if err == nil {
		t.Error("expected error updating non-existent profile")
	}
}

// -----------------------------------------------------------------------
// 7. TestHeartbeat
// -----------------------------------------------------------------------

func TestHeartbeat(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(1)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(200000000))

	srv := keeper.NewMsgServerImpl(k)

	// Register at block 100.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr,
		Domains: []string{"mathematics"},
		Stake:   "1000000",
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	profile, _ := k.GetProfile(ctx, addr)
	if profile.LastActiveBlock != 100 {
		t.Errorf("expected last_active_block 100, got %d", profile.LastActiveBlock)
	}

	// Advance to block 200 and send heartbeat.
	ctx200 := ctx.WithBlockHeight(200)
	_, err = srv.Heartbeat(ctx200, &types.MsgHeartbeat{
		Sender: addr,
	})
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}

	profile, _ = k.GetProfile(ctx200, addr)
	if profile.LastActiveBlock != 200 {
		t.Errorf("expected last_active_block 200, got %d", profile.LastActiveBlock)
	}

	// Manually expire the profile, then heartbeat to reactivate.
	profile.Status = "expired"
	k.SetProfile(ctx200, profile)

	ctx300 := ctx.WithBlockHeight(300)
	_, err = srv.Heartbeat(ctx300, &types.MsgHeartbeat{
		Sender: addr,
	})
	if err != nil {
		t.Fatalf("heartbeat reactivation failed: %v", err)
	}

	profile, _ = k.GetProfile(ctx300, addr)
	if profile.Status != "active" {
		t.Errorf("expected status active after heartbeat reactivation, got %s", profile.Status)
	}
	if profile.LastActiveBlock != 300 {
		t.Errorf("expected last_active_block 300, got %d", profile.LastActiveBlock)
	}

	// Heartbeat for non-existent profile should fail.
	_, err = srv.Heartbeat(ctx300, &types.MsgHeartbeat{
		Sender: testAddr(99),
	})
	if err == nil {
		t.Error("expected error for heartbeat on non-existent profile")
	}
}

// -----------------------------------------------------------------------
// 8. TestDeregisterProfile
// -----------------------------------------------------------------------

func TestDeregisterProfile(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(1)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(200000000))

	srv := keeper.NewMsgServerImpl(k)

	// Register.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr,
		Domains: []string{"mathematics", "physics"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference"},
		},
		Stake: "10000000",
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Verify pre-deregister state.
	balBefore := bk.balances[accAddr.String()+"/uzrn"]
	expectedBefore := sdkmath.NewInt(190000000)
	if !balBefore.Equal(expectedBefore) {
		t.Errorf("expected sender balance %s before deregister, got %s", expectedBefore, balBefore)
	}

	// Deregister.
	resp, err := srv.DeregisterProfile(ctx, &types.MsgDeregisterProfile{
		Sender: addr,
	})
	if err != nil {
		t.Fatalf("deregistration failed: %v", err)
	}
	if resp.RefundedAmount != "10000000" {
		t.Errorf("expected refunded amount 10000000, got %s", resp.RefundedAmount)
	}

	// Verify profile is deleted.
	_, found := k.GetProfile(ctx, addr)
	if found {
		t.Error("expected profile to be deleted")
	}

	// Verify domain index was cleaned.
	mathProfiles := k.GetProfilesByDomain(ctx, "mathematics")
	if len(mathProfiles) != 0 {
		t.Errorf("expected 0 math profiles after deregister, got %d", len(mathProfiles))
	}
	physicsProfiles := k.GetProfilesByDomain(ctx, "physics")
	if len(physicsProfiles) != 0 {
		t.Errorf("expected 0 physics profiles after deregister, got %d", len(physicsProfiles))
	}

	// Verify capability index was cleaned.
	inferenceProfiles := k.GetProfilesByCapability(ctx, "inference")
	if len(inferenceProfiles) != 0 {
		t.Errorf("expected 0 inference profiles after deregister, got %d", len(inferenceProfiles))
	}

	// Verify stake was refunded.
	balAfter := bk.balances[accAddr.String()+"/uzrn"]
	expectedAfter := sdkmath.NewInt(200000000)
	if !balAfter.Equal(expectedAfter) {
		t.Errorf("expected sender balance %s after refund, got %s", expectedAfter, balAfter)
	}

	// Module account should be empty.
	moduleBal, ok := bk.balances[types.ModuleName+"/uzrn"]
	if ok && moduleBal.IsPositive() {
		t.Errorf("expected module balance 0 after deregister, got %s", moduleBal)
	}

	// Deregister non-existent profile should fail.
	_, err = srv.DeregisterProfile(ctx, &types.MsgDeregisterProfile{
		Sender: testAddr(99),
	})
	if err == nil {
		t.Error("expected error deregistering non-existent profile")
	}
}

// -----------------------------------------------------------------------
// 9. TestExpireStaleProfiles
// -----------------------------------------------------------------------

func TestExpireStaleProfiles(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Set a short expiry for testing purposes.
	params := types.DefaultParams()
	params.ProfileExpiryBlocks = 500
	k.SetParams(ctx, params)

	// Create 3 profiles at block 100 (current block).
	for i := 0; i < 3; i++ {
		k.SetProfile(ctx, &types.AgentProfile{
			Address:         testAddr(i),
			Domains:         []string{"test"},
			Status:          "active",
			Stake:           "1000000",
			LastActiveBlock: 100,
		})
	}

	// At block 200 (not a multiple of 100 boundary-hit yet), run BeginBlocker.
	// block 200 is a multiple of 100, but 200 < 100 + 500, so no expiry.
	ctx200 := ctx.WithBlockHeight(200)
	err := k.BeginBlocker(ctx200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All profiles should still be active.
	all := k.GetAllProfiles(ctx200)
	for _, p := range all {
		if p.Status != "active" {
			t.Errorf("expected active at block 200, got %s for %s", p.Status, p.Address)
		}
	}

	// At block 700 (700 % 100 == 0 and 100 + 500 < 700), profiles should expire.
	ctx700 := ctx.WithBlockHeight(700)
	err = k.BeginBlocker(ctx700)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all = k.GetAllProfiles(ctx700)
	for _, p := range all {
		if p.Status != "expired" {
			t.Errorf("expected expired at block 700, got %s for %s", p.Status, p.Address)
		}
	}

	// A profile with recent activity should NOT be expired.
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(10),
		Domains:         []string{"fresh"},
		Status:          "active",
		Stake:           "1000000",
		LastActiveBlock: 650, // 650 + 500 = 1150 > 700
	})

	// Re-run at block 700 to check the fresh profile.
	err = k.BeginBlocker(ctx700)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fresh, found := k.GetProfile(ctx700, testAddr(10))
	if !found {
		t.Fatal("expected fresh profile to exist")
	}
	if fresh.Status != "active" {
		t.Errorf("expected fresh profile to be active, got %s", fresh.Status)
	}

	// BeginBlocker at a non-100 block should be a no-op.
	ctx701 := ctx.WithBlockHeight(701)
	// Manually re-activate a profile to check that no expiry happens.
	p := all[0]
	p.Status = "active"
	k.SetProfile(ctx701, p)

	err = k.BeginBlocker(ctx701)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	check, _ := k.GetProfile(ctx701, p.Address)
	if check.Status != "active" {
		t.Errorf("expected no expiry at non-100 block, but profile was %s", check.Status)
	}
}

// -----------------------------------------------------------------------
// 10. TestSearchByDomain
// -----------------------------------------------------------------------

func TestSearchByDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create agents in various domains.
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(1),
		Domains: []string{"mathematics"},
		Status:  "active",
		Stake:   "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(2),
		Domains: []string{"mathematics", "physics"},
		Status:  "active",
		Stake:   "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(3),
		Domains: []string{"physics"},
		Status:  "active",
		Stake:   "1000000",
	})
	// Inactive profile should NOT appear in domain search.
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(4),
		Domains: []string{"mathematics"},
		Status:  "expired",
		Stake:   "1000000",
	})

	// Search by domain "mathematics".
	results := k.SearchProfiles(ctx, "mathematics", "", 0)
	if len(results) != 2 {
		t.Errorf("expected 2 mathematics profiles, got %d", len(results))
	}

	// Search by domain "physics".
	results = k.SearchProfiles(ctx, "physics", "", 0)
	if len(results) != 2 {
		t.Errorf("expected 2 physics profiles, got %d", len(results))
	}

	// Search by non-existent domain.
	results = k.SearchProfiles(ctx, "chemistry", "", 0)
	if len(results) != 0 {
		t.Errorf("expected 0 chemistry profiles, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// 11. TestSearchByCapability
// -----------------------------------------------------------------------

func TestSearchByCapability(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create agents with various capabilities.
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(1),
		Domains: []string{"math"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference"},
			{CapabilityType: "reasoning"},
		},
		Status: "active",
		Stake:  "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(2),
		Domains: []string{"physics"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference"},
		},
		Status: "active",
		Stake:  "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(3),
		Domains: []string{"biology"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "verification"},
		},
		Status: "active",
		Stake:  "1000000",
	})
	// Inactive profile should be filtered out.
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(4),
		Domains: []string{"math"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference"},
		},
		Status: "expired",
		Stake:  "1000000",
	})

	// Search by capability "inference".
	results := k.SearchProfiles(ctx, "", "inference", 0)
	if len(results) != 2 {
		t.Errorf("expected 2 inference profiles, got %d", len(results))
	}

	// Search by capability "reasoning".
	results = k.SearchProfiles(ctx, "", "reasoning", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 reasoning profile, got %d", len(results))
	}

	// Search by capability "verification".
	results = k.SearchProfiles(ctx, "", "verification", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 verification profile, got %d", len(results))
	}

	// Combined: domain "math" + capability "inference".
	results = k.SearchProfiles(ctx, "math", "inference", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 math+inference profile, got %d", len(results))
	}

	// Combined: domain "physics" + capability "reasoning" (no match).
	results = k.SearchProfiles(ctx, "physics", "reasoning", 0)
	if len(results) != 0 {
		t.Errorf("expected 0 physics+reasoning profiles, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// 12. TestSearchWithMinReputation
// -----------------------------------------------------------------------

func TestSearchWithMinReputation(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create agents with varying reputations.
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(1),
		Domains:         []string{"math"},
		Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
		Status:          "active",
		ReputationScore: 800000, // high
		Stake:           "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(2),
		Domains:         []string{"math"},
		Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
		Status:          "active",
		ReputationScore: 500000, // medium
		Stake:           "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(3),
		Domains:         []string{"math"},
		Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
		Status:          "active",
		ReputationScore: 200000, // low
		Stake:           "1000000",
	})

	// No min reputation: all active profiles returned.
	results := k.SearchProfiles(ctx, "math", "", 0)
	if len(results) != 3 {
		t.Errorf("expected 3 profiles with no min reputation, got %d", len(results))
	}

	// Min reputation 400000: only 2 profiles pass.
	results = k.SearchProfiles(ctx, "math", "", 400000)
	if len(results) != 2 {
		t.Errorf("expected 2 profiles with min reputation 400000, got %d", len(results))
	}

	// Min reputation 700000: only 1 profile passes.
	results = k.SearchProfiles(ctx, "math", "", 700000)
	if len(results) != 1 {
		t.Errorf("expected 1 profile with min reputation 700000, got %d", len(results))
	}

	// Min reputation 900000: none pass.
	results = k.SearchProfiles(ctx, "math", "", 900000)
	if len(results) != 0 {
		t.Errorf("expected 0 profiles with min reputation 900000, got %d", len(results))
	}

	// Capability filter + reputation filter combined.
	results = k.SearchProfiles(ctx, "", "inference", 500000)
	if len(results) != 2 {
		t.Errorf("expected 2 inference profiles with min reputation 500000, got %d", len(results))
	}

	// All active profiles with reputation filter (no domain/capability filter).
	results = k.SearchProfiles(ctx, "", "", 500000)
	if len(results) != 2 {
		t.Errorf("expected 2 profiles with min reputation 500000 (no index filter), got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// Additional: TestMaxCapabilitiesValidation
// -----------------------------------------------------------------------

func TestMaxCapabilitiesValidation(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	// Set max capabilities to 2.
	params := types.DefaultParams()
	params.MaxCapabilitiesPerAgent = 2
	k.SetParams(ctx, params)

	addr := testAddr(1)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(200000000))

	srv := keeper.NewMsgServerImpl(k)

	// Try to register with 3 capabilities (exceeds max of 2).
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr,
		Domains: []string{"math"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference"},
			{CapabilityType: "reasoning"},
			{CapabilityType: "verification"},
		},
		Stake: "1000000",
	})
	if err == nil {
		t.Error("expected max capabilities error")
	}

	// With 2 capabilities should succeed.
	_, err = srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr,
		Domains: []string{"math"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference"},
			{CapabilityType: "reasoning"},
		},
		Stake: "1000000",
	})
	if err != nil {
		t.Fatalf("registration with 2 capabilities should succeed: %v", err)
	}
}

// -----------------------------------------------------------------------
// Additional: TestGenesisExportImport
// -----------------------------------------------------------------------

func TestGenesisExportImport(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Set custom params.
	params := types.DefaultParams()
	params.MinRegistrationStake = "5000000"
	params.MaxCapabilitiesPerAgent = 15
	k.SetParams(ctx, params)

	// Create profiles.
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(1),
		DisplayName:     "Agent1",
		Domains:         []string{"math", "physics"},
		Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
		Status:          "active",
		ReputationScore: 700000,
		Stake:           "5000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(2),
		DisplayName:     "Agent2",
		Domains:         []string{"biology"},
		Status:          "expired",
		ReputationScore: 300000,
		Stake:           "5000000",
	})

	// Export genesis.
	genState := k.ExportGenesis(ctx)
	if len(genState.Profiles) != 2 {
		t.Errorf("expected 2 profiles in genesis export, got %d", len(genState.Profiles))
	}
	if genState.Params.MinRegistrationStake != "5000000" {
		t.Errorf("expected exported MinRegistrationStake 5000000, got %s", genState.Params.MinRegistrationStake)
	}

	// Import into a fresh keeper.
	k2, ctx2, _ := setupKeeper(t)
	k2.InitGenesis(ctx2, genState)

	// Verify params.
	gotParams := k2.GetParams(ctx2)
	if gotParams.MinRegistrationStake != "5000000" {
		t.Errorf("expected imported MinRegistrationStake 5000000, got %s", gotParams.MinRegistrationStake)
	}

	// Verify profiles.
	allProfiles := k2.GetAllProfiles(ctx2)
	if len(allProfiles) != 2 {
		t.Errorf("expected 2 profiles after import, got %d", len(allProfiles))
	}

	// Verify domain indexes were rebuilt.
	mathProfiles := k2.GetProfilesByDomain(ctx2, "math")
	if len(mathProfiles) != 1 {
		t.Errorf("expected 1 math profile after import, got %d", len(mathProfiles))
	}
}

// -----------------------------------------------------------------------
// Additional: TestUpdateParamsGovernance
// -----------------------------------------------------------------------

func TestUpdateParamsGovernance(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.MinRegistrationStake = "10000000"
	newParams.MaxCapabilitiesPerAgent = 50
	newParams.ProfileExpiryBlocks = 200000

	// Authority can update params.
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MinRegistrationStake != "10000000" {
		t.Errorf("expected 10000000, got %s", got.MinRegistrationStake)
	}
	if got.MaxCapabilitiesPerAgent != 50 {
		t.Errorf("expected 50, got %d", got.MaxCapabilitiesPerAgent)
	}
	if got.ProfileExpiryBlocks != 200000 {
		t.Errorf("expected 200000, got %d", got.ProfileExpiryBlocks)
	}

	// Non-authority should be rejected.
	_, err = srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr(99),
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Error("expected unauthorized error")
	}

	// Nil params should be rejected.
	_, err = srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    nil,
	})
	if err == nil {
		t.Error("expected nil params error")
	}

	// Invalid params should be rejected.
	invalidParams := types.DefaultParams()
	invalidParams.MaxCapabilitiesPerAgent = 0 // invalid
	_, err = srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    invalidParams,
	})
	if err == nil {
		t.Error("expected invalid params error")
	}
}

// =======================================================================
// Ported from legible-money prototype: Discovery module test coverage
// =======================================================================

// -----------------------------------------------------------------------
// 16. TestDiscoveryRegistration
// Full registration lifecycle: register with all fields, verify every
// stored field including block height, default reputation, and indexes.
// Ported from: OC-DISC-1 / TestRegisterAgentHandler
// -----------------------------------------------------------------------

func TestDiscoveryRegistration(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(200)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "DiscoveryAgent",
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "verification", Domains: []string{"mathematics"}, ConfidenceBps: 8000},
			{CapabilityType: "research", Domains: []string{"physics"}, ConfidenceBps: 7000},
		},
		Domains:     []string{"mathematics", "physics"},
		Stake:       "5000000",
		Description: "Full registration test agent",
		Metadata:    `{"tier":"premium"}`,
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	profile, found := k.GetProfile(ctx, addr)
	if !found {
		t.Fatal("registered profile not found")
	}
	if profile.Status != "active" {
		t.Errorf("expected status active, got %s", profile.Status)
	}
	if profile.ReputationScore != 500000 {
		t.Errorf("expected initial reputation 500000, got %d", profile.ReputationScore)
	}
	if profile.DisplayName != "DiscoveryAgent" {
		t.Errorf("expected display_name DiscoveryAgent, got %s", profile.DisplayName)
	}
	if profile.Description != "Full registration test agent" {
		t.Errorf("expected description preserved, got %s", profile.Description)
	}
	if profile.Metadata != `{"tier":"premium"}` {
		t.Errorf("expected metadata preserved, got %s", profile.Metadata)
	}
	if profile.RegisteredAtBlock != 100 {
		t.Errorf("expected registered_at_block 100, got %d", profile.RegisteredAtBlock)
	}
	if profile.LastActiveBlock != 100 {
		t.Errorf("expected last_active_block 100, got %d", profile.LastActiveBlock)
	}
	if len(profile.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(profile.Capabilities))
	}
	if len(profile.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(profile.Domains))
	}
	if profile.Stake != "5000000" {
		t.Errorf("expected stake 5000000, got %s", profile.Stake)
	}

	// Verify capability confidence was stored.
	for _, cap := range profile.Capabilities {
		if cap.CapabilityType == "verification" && cap.ConfidenceBps != 8000 {
			t.Errorf("expected verification confidence_bps 8000, got %d", cap.ConfidenceBps)
		}
		if cap.CapabilityType == "research" && cap.ConfidenceBps != 7000 {
			t.Errorf("expected research confidence_bps 7000, got %d", cap.ConfidenceBps)
		}
	}

	// Verify both domain indexes were populated.
	mathProfiles := k.GetProfilesByDomain(ctx, "mathematics")
	if len(mathProfiles) != 1 {
		t.Errorf("expected 1 math profile, got %d", len(mathProfiles))
	}
	physicsProfiles := k.GetProfilesByDomain(ctx, "physics")
	if len(physicsProfiles) != 1 {
		t.Errorf("expected 1 physics profile, got %d", len(physicsProfiles))
	}

	// Verify both capability indexes were populated.
	verProfiles := k.GetProfilesByCapability(ctx, "verification")
	if len(verProfiles) != 1 {
		t.Errorf("expected 1 verification profile, got %d", len(verProfiles))
	}
	resProfiles := k.GetProfilesByCapability(ctx, "research")
	if len(resProfiles) != 1 {
		t.Errorf("expected 1 research profile, got %d", len(resProfiles))
	}
}

// -----------------------------------------------------------------------
// 17. TestDiscoveryRegistrationStakeBoundary
// Boundary test: register at exactly min stake, just below, and above.
// Ported from: OC-DISC-1 (InsufficientStakeRegistration boundary)
// -----------------------------------------------------------------------

func TestDiscoveryRegistrationStakeBoundary(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	// Default MinRegistrationStake is "1000000".
	srv := keeper.NewMsgServerImpl(k)

	// Test: stake 1 unit below minimum should fail.
	addr1 := testAddr(201)
	accAddr1, _ := sdk.AccAddressFromBech32(addr1)
	bk.setBalance(accAddr1, "uzrn", sdkmath.NewInt(500000000))

	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr1,
		Domains: []string{"math"},
		Stake:   "999999", // 1 below 1000000
	})
	if err == nil {
		t.Error("expected error for stake 1 unit below minimum")
	}

	// Verify no profile stored.
	_, found := k.GetProfile(ctx, addr1)
	if found {
		t.Error("profile persisted despite insufficient stake rejection")
	}

	// Test: exactly at minimum should succeed.
	_, err = srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr1,
		Domains: []string{"math"},
		Stake:   "1000000", // exactly minimum
	})
	if err != nil {
		t.Fatalf("registration at exact minimum should succeed: %v", err)
	}

	_, found = k.GetProfile(ctx, addr1)
	if !found {
		t.Error("profile not found after valid registration at exact minimum")
	}

	// Test: above minimum should succeed.
	addr2 := testAddr(202)
	accAddr2, _ := sdk.AccAddressFromBech32(addr2)
	bk.setBalance(accAddr2, "uzrn", sdkmath.NewInt(500000000))

	_, err = srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr2,
		Domains: []string{"math"},
		Stake:   "2000000",
	})
	if err != nil {
		t.Fatalf("registration above minimum should succeed: %v", err)
	}
}

// -----------------------------------------------------------------------
// 18. TestDiscoveryUpdate
// Update preserves capabilities and domains when only mutable fields change.
// Ported from: OC-DISC-7 (ProfileUpdatePreservation)
// -----------------------------------------------------------------------

func TestDiscoveryUpdate(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(203)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	// Register with capabilities and multiple domains.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "Original Name",
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "verification", ConfidenceBps: 8000},
			{CapabilityType: "research", ConfidenceBps: 7000},
		},
		Domains:     []string{"mathematics", "physics"},
		Stake:       "1000000",
		Description: "Original description",
		Metadata:    `{"v":"1"}`,
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Update ONLY display name.
	_, err = srv.UpdateProfile(ctx, &types.MsgUpdateProfile{
		Sender:      addr,
		DisplayName: "Updated Name",
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	profile, _ := k.GetProfile(ctx, addr)
	if profile.DisplayName != "Updated Name" {
		t.Errorf("expected 'Updated Name', got '%s'", profile.DisplayName)
	}

	// Verify domains NOT clobbered.
	if len(profile.Domains) != 2 {
		t.Errorf("domains clobbered by display name update; expected 2, got %d", len(profile.Domains))
	}

	// Verify capabilities NOT clobbered.
	if len(profile.Capabilities) != 2 {
		t.Errorf("capabilities clobbered by display name update; expected 2, got %d", len(profile.Capabilities))
	}

	// Verify description NOT clobbered.
	if profile.Description != "Original description" {
		t.Errorf("description clobbered; expected 'Original description', got '%s'", profile.Description)
	}

	// Verify metadata NOT clobbered.
	if profile.Metadata != `{"v":"1"}` {
		t.Errorf("metadata clobbered; expected {\"v\":\"1\"}, got '%s'", profile.Metadata)
	}

	// Verify stake and reputation unchanged.
	if profile.Stake != "1000000" {
		t.Errorf("stake changed after update; expected 1000000, got %s", profile.Stake)
	}
	if profile.ReputationScore != 500000 {
		t.Errorf("reputation changed after update; expected 500000, got %d", profile.ReputationScore)
	}
}

// -----------------------------------------------------------------------
// 19. TestDiscoveryDeregistration
// Full deregistration lifecycle: register, verify indexes, deregister,
// verify profile deleted, indexes cleaned, stake refunded.
// Ported from: OC-DISC-4 (DeregisterStakeRefund)
// -----------------------------------------------------------------------

func TestDiscoveryDeregistration(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(204)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "Deregister Test",
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "verification"},
			{CapabilityType: "research"},
		},
		Domains: []string{"mathematics", "physics", "chemistry"},
		Stake:   "10000000",
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Verify indexes populated before deregistration.
	if len(k.GetProfilesByDomain(ctx, "mathematics")) != 1 {
		t.Fatal("expected 1 profile in mathematics domain before deregister")
	}
	if len(k.GetProfilesByDomain(ctx, "chemistry")) != 1 {
		t.Fatal("expected 1 profile in chemistry domain before deregister")
	}
	if len(k.GetProfilesByCapability(ctx, "verification")) != 1 {
		t.Fatal("expected 1 profile with verification capability before deregister")
	}

	// Deregister.
	resp, err := srv.DeregisterProfile(ctx, &types.MsgDeregisterProfile{
		Sender: addr,
	})
	if err != nil {
		t.Fatalf("deregistration failed: %v", err)
	}
	if resp.RefundedAmount != "10000000" {
		t.Errorf("expected refund 10000000, got %s", resp.RefundedAmount)
	}

	// Verify profile deleted.
	_, found := k.GetProfile(ctx, addr)
	if found {
		t.Error("profile still exists after deregistration")
	}

	// Verify all domain indexes cleaned.
	for _, domain := range []string{"mathematics", "physics", "chemistry"} {
		profiles := k.GetProfilesByDomain(ctx, domain)
		for _, p := range profiles {
			if p.Address == addr {
				t.Errorf("deregistered agent still appears in %s domain index", domain)
			}
		}
	}

	// Verify all capability indexes cleaned.
	for _, capType := range []string{"verification", "research"} {
		profiles := k.GetProfilesByCapability(ctx, capType)
		for _, p := range profiles {
			if p.Address == addr {
				t.Errorf("deregistered agent still appears in %s capability index", capType)
			}
		}
	}

	// Verify stake refunded to sender.
	expectedBal := sdkmath.NewInt(500000000)
	actualBal := bk.balances[accAddr.String()+"/uzrn"]
	if !actualBal.Equal(expectedBal) {
		t.Errorf("expected sender balance %s after refund, got %s", expectedBal, actualBal)
	}
}

// -----------------------------------------------------------------------
// 20. TestDiscoveryDeregistrationNonExistent
// Deregistering a never-registered address must fail gracefully.
// Ported from: OC-DISC-10 (DeregisterNonExistent)
// -----------------------------------------------------------------------

func TestDiscoveryDeregistrationNonExistent(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.DeregisterProfile(ctx, &types.MsgDeregisterProfile{
		Sender: testAddr(999),
	})
	if err == nil {
		t.Error("expected error for deregistering non-existent agent")
	}

	// Verify no profile was created as a side effect.
	_, found := k.GetProfile(ctx, testAddr(999))
	if found {
		t.Error("profile appeared after failed deregister")
	}

	_ = k // suppress unused
}

// -----------------------------------------------------------------------
// 21. TestDiscoverySearch
// Combined domain+capability search with multiple agents.
// Ported from: TestGetProfilesByDomain / TestGetProfilesByCapability
// -----------------------------------------------------------------------

func TestDiscoverySearch(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create a diverse set of agents.
	k.SetProfile(ctx, &types.AgentProfile{
		Address:      testAddr(300),
		Domains:      []string{"mathematics"},
		Capabilities: []*types.AgentCapability{{CapabilityType: "inference"}},
		Status:       "active",
		Stake:        "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address:      testAddr(301),
		Domains:      []string{"mathematics", "physics"},
		Capabilities: []*types.AgentCapability{{CapabilityType: "inference"}, {CapabilityType: "verification"}},
		Status:       "active",
		Stake:        "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address:      testAddr(302),
		Domains:      []string{"physics"},
		Capabilities: []*types.AgentCapability{{CapabilityType: "verification"}},
		Status:       "active",
		Stake:        "1000000",
	})

	// domain=mathematics, capability=inference => addr 300 and 301
	results := k.SearchProfiles(ctx, "mathematics", "inference", 0)
	if len(results) != 2 {
		t.Errorf("expected 2 for math+inference, got %d", len(results))
	}

	// domain=physics, capability=verification => addr 301 and 302
	results = k.SearchProfiles(ctx, "physics", "verification", 0)
	if len(results) != 2 {
		t.Errorf("expected 2 for physics+verification, got %d", len(results))
	}

	// domain=mathematics, capability=verification => only addr 301
	results = k.SearchProfiles(ctx, "mathematics", "verification", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 for math+verification, got %d", len(results))
	}

	// domain=physics, capability=inference => only addr 301
	results = k.SearchProfiles(ctx, "physics", "inference", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 for physics+inference, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// 22. TestDiscoverySearchByCategory
// Search profiles across multiple distinct categories/domains.
// Ported from: TestGetProfilesByDomain multi-domain
// -----------------------------------------------------------------------

func TestDiscoverySearchByCategory(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	categories := []string{"mathematics", "physics", "biology", "chemistry", "economics"}
	for i, cat := range categories {
		k.SetProfile(ctx, &types.AgentProfile{
			Address: testAddr(400 + i),
			Domains: []string{cat},
			Status:  "active",
			Stake:   "1000000",
		})
	}
	// One agent covers multiple categories.
	k.SetProfile(ctx, &types.AgentProfile{
		Address: testAddr(410),
		Domains: categories,
		Status:  "active",
		Stake:   "1000000",
	})

	for _, cat := range categories {
		results := k.SearchProfiles(ctx, cat, "", 0)
		if len(results) != 2 {
			t.Errorf("expected 2 profiles in %s (dedicated + multi), got %d", cat, len(results))
		}
	}

	// All active profiles via empty search.
	all := k.SearchProfiles(ctx, "", "", 0)
	if len(all) != 6 {
		t.Errorf("expected 6 total active profiles, got %d", len(all))
	}
}

// -----------------------------------------------------------------------
// 23. TestDiscoverySearchPagination
// Search with a large result set to verify iteration correctness.
// -----------------------------------------------------------------------

func TestDiscoverySearchPagination(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create 20 profiles in the "ai" domain.
	for i := 0; i < 20; i++ {
		k.SetProfile(ctx, &types.AgentProfile{
			Address:         testAddr(500 + i),
			Domains:         []string{"ai"},
			Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
			Status:          "active",
			ReputationScore: uint64(100000 + i*10000),
			Stake:           "1000000",
		})
	}

	// Search all "ai" domain profiles.
	results := k.SearchProfiles(ctx, "ai", "", 0)
	if len(results) != 20 {
		t.Errorf("expected 20 ai profiles, got %d", len(results))
	}

	// Search with capability filter.
	results = k.SearchProfiles(ctx, "ai", "inference", 0)
	if len(results) != 20 {
		t.Errorf("expected 20 ai+inference profiles, got %d", len(results))
	}

	// Search with min reputation filter (only some pass).
	// Scores range from 100000 to 290000.
	results = k.SearchProfiles(ctx, "ai", "", 200000)
	if len(results) != 10 {
		t.Errorf("expected 10 profiles with reputation >= 200000, got %d", len(results))
	}

	// Verify GetAllProfiles returns all 20.
	all := k.GetAllProfiles(ctx)
	if len(all) != 20 {
		t.Errorf("expected 20 total profiles, got %d", len(all))
	}
}

// -----------------------------------------------------------------------
// 24. TestDiscoverySearchEmpty
// Ghost domain/capability queries: no panic on empty results.
// Ported from: OC-DISC-6 (GhostDomainQuery)
// -----------------------------------------------------------------------

func TestDiscoverySearchEmpty(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Query on empty store must return nil/empty, not panic.
	profiles := k.GetProfilesByDomain(ctx, "nonexistent_domain_xyz")
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles for nonexistent domain, got %d", len(profiles))
	}

	profiles = k.GetProfilesByCapability(ctx, "nonexistent_capability")
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles for nonexistent capability, got %d", len(profiles))
	}

	results := k.SearchProfiles(ctx, "ghost_domain", "ghost_capability", 0)
	if len(results) != 0 {
		t.Errorf("expected 0 profiles for ghost combined search, got %d", len(results))
	}

	results = k.SearchProfiles(ctx, "", "", 999999)
	if len(results) != 0 {
		t.Errorf("expected 0 profiles for high min reputation on empty store, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// 25. TestDiscoveryRanking
// Reputation-based filtering across diverse reputation scores.
// Ported from: reputation filtering in prototype queries
// -----------------------------------------------------------------------

func TestDiscoveryRanking(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create agents with distinct reputation scores.
	scores := []uint64{900000, 700000, 500000, 300000, 100000}
	for i, score := range scores {
		k.SetProfile(ctx, &types.AgentProfile{
			Address:         testAddr(600 + i),
			Domains:         []string{"ranking"},
			Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
			Status:          "active",
			ReputationScore: score,
			Stake:           "1000000",
		})
	}

	// All agents returned with no reputation filter.
	results := k.SearchProfiles(ctx, "ranking", "", 0)
	if len(results) != 5 {
		t.Errorf("expected 5 ranking profiles, got %d", len(results))
	}

	// Progressive reputation thresholds.
	thresholds := []struct {
		minRep   uint64
		expected int
	}{
		{100000, 5},
		{300000, 4},
		{500000, 3},
		{700000, 2},
		{900000, 1},
		{1000000, 0},
	}
	for _, tc := range thresholds {
		results = k.SearchProfiles(ctx, "ranking", "", tc.minRep)
		if len(results) != tc.expected {
			t.Errorf("minRep %d: expected %d profiles, got %d", tc.minRep, tc.expected, len(results))
		}
	}
}

// -----------------------------------------------------------------------
// 26. TestDiscoveryRankingDecay
// Expired profiles should not appear in reputation-filtered searches.
// Ported from: OC-DISC-9 (ExpiredAgentNotInQueries)
// -----------------------------------------------------------------------

func TestDiscoveryRankingDecay(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create two agents: one active with high reputation, one expired with high reputation.
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(700),
		Domains:         []string{"decay"},
		Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
		Status:          "active",
		ReputationScore: 800000,
		Stake:           "1000000",
	})
	k.SetProfile(ctx, &types.AgentProfile{
		Address:         testAddr(701),
		Domains:         []string{"decay"},
		Capabilities:    []*types.AgentCapability{{CapabilityType: "inference"}},
		Status:          "expired",
		ReputationScore: 900000, // higher reputation but expired
		Stake:           "1000000",
	})

	// Domain search should only return the active agent.
	results := k.GetProfilesByDomain(ctx, "decay")
	if len(results) != 1 {
		t.Errorf("expected 1 active profile in decay domain, got %d", len(results))
	}
	if len(results) > 0 && results[0].Address != testAddr(700) {
		t.Errorf("expected active agent %s, got %s", testAddr(700), results[0].Address)
	}

	// Capability search should only return the active agent.
	results = k.GetProfilesByCapability(ctx, "inference")
	if len(results) != 1 {
		t.Errorf("expected 1 active inference profile, got %d", len(results))
	}

	// SearchProfiles (combined) should only return the active agent.
	results = k.SearchProfiles(ctx, "decay", "inference", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 active profile in combined search, got %d", len(results))
	}

	// Even with low reputation threshold, expired agent should not appear.
	results = k.SearchProfiles(ctx, "decay", "", 100000)
	if len(results) != 1 {
		t.Errorf("expected 1 active profile with min reputation, got %d", len(results))
	}
	for _, p := range results {
		if p.Status == "expired" {
			t.Error("expired agent appeared in search results despite status filtering")
		}
	}
}

// -----------------------------------------------------------------------
// 27. TestDiscoveryMetadata
// Full metadata round-trip: store JSON metadata, update it, verify
// persistence and no corruption of other fields.
// Ported from: TestUpdateProfile_EmptyFieldsPreserved / OC-DISC-7
// -----------------------------------------------------------------------

func TestDiscoveryMetadata(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(800)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	// Register with rich metadata.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "MetadataAgent",
		Domains:     []string{"metadata"},
		Stake:       "1000000",
		Description: "Tests metadata round-trip",
		Metadata:    `{"version":"1","endpoints":["https://api.example.com"],"tags":["ai","ml"]}`,
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Verify metadata stored correctly.
	profile, _ := k.GetProfile(ctx, addr)
	if profile.Metadata != `{"version":"1","endpoints":["https://api.example.com"],"tags":["ai","ml"]}` {
		t.Errorf("metadata not preserved after registration: %s", profile.Metadata)
	}

	// Update only metadata.
	_, err = srv.UpdateProfile(ctx, &types.MsgUpdateProfile{
		Sender:   addr,
		Metadata: `{"version":"2","endpoints":["https://api-v2.example.com"],"tags":["ai","ml","nlp"]}`,
	})
	if err != nil {
		t.Fatalf("metadata update failed: %v", err)
	}

	profile, _ = k.GetProfile(ctx, addr)
	if profile.Metadata != `{"version":"2","endpoints":["https://api-v2.example.com"],"tags":["ai","ml","nlp"]}` {
		t.Errorf("metadata not updated correctly: %s", profile.Metadata)
	}

	// Verify other fields unchanged.
	if profile.DisplayName != "MetadataAgent" {
		t.Errorf("display_name changed after metadata update: %s", profile.DisplayName)
	}
	if profile.Description != "Tests metadata round-trip" {
		t.Errorf("description changed after metadata update: %s", profile.Description)
	}
	if profile.Status != "active" {
		t.Errorf("status changed after metadata update: %s", profile.Status)
	}

	// Update only description, verify metadata unchanged.
	_, err = srv.UpdateProfile(ctx, &types.MsgUpdateProfile{
		Sender:      addr,
		Description: "Updated description",
	})
	if err != nil {
		t.Fatalf("description update failed: %v", err)
	}

	profile, _ = k.GetProfile(ctx, addr)
	if profile.Metadata != `{"version":"2","endpoints":["https://api-v2.example.com"],"tags":["ai","ml","nlp"]}` {
		t.Errorf("metadata changed after description-only update: %s", profile.Metadata)
	}
	if profile.Description != "Updated description" {
		t.Errorf("description not updated: %s", profile.Description)
	}
}

// -----------------------------------------------------------------------
// 28. TestDiscoveryVerification
// Capability verified_by_count field is preserved through storage.
// Ported from: capability confidence/verification fields in prototype
// -----------------------------------------------------------------------

func TestDiscoveryVerification(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	addr := testAddr(900)

	// Store profile with verified capabilities.
	k.SetProfile(ctx, &types.AgentProfile{
		Address:     addr,
		DisplayName: "VerifiedAgent",
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference", Domains: []string{"math"}, ConfidenceBps: 9500, VerifiedByCount: 10},
			{CapabilityType: "verification", Domains: []string{"physics"}, ConfidenceBps: 8000, VerifiedByCount: 5},
			{CapabilityType: "research", Domains: []string{"biology"}, ConfidenceBps: 6000, VerifiedByCount: 0},
		},
		Domains:         []string{"math", "physics", "biology"},
		Status:          "active",
		ReputationScore: 750000,
		Stake:           "1000000",
	})

	profile, found := k.GetProfile(ctx, addr)
	if !found {
		t.Fatal("verified profile not found")
	}

	if len(profile.Capabilities) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(profile.Capabilities))
	}

	// Verify each capability's fields are preserved.
	for _, cap := range profile.Capabilities {
		switch cap.CapabilityType {
		case "inference":
			if cap.ConfidenceBps != 9500 {
				t.Errorf("inference: expected confidence_bps 9500, got %d", cap.ConfidenceBps)
			}
			if cap.VerifiedByCount != 10 {
				t.Errorf("inference: expected verified_by_count 10, got %d", cap.VerifiedByCount)
			}
			if len(cap.Domains) != 1 || cap.Domains[0] != "math" {
				t.Errorf("inference: expected domains [math], got %v", cap.Domains)
			}
		case "verification":
			if cap.ConfidenceBps != 8000 {
				t.Errorf("verification: expected confidence_bps 8000, got %d", cap.ConfidenceBps)
			}
			if cap.VerifiedByCount != 5 {
				t.Errorf("verification: expected verified_by_count 5, got %d", cap.VerifiedByCount)
			}
		case "research":
			if cap.ConfidenceBps != 6000 {
				t.Errorf("research: expected confidence_bps 6000, got %d", cap.ConfidenceBps)
			}
			if cap.VerifiedByCount != 0 {
				t.Errorf("research: expected verified_by_count 0, got %d", cap.VerifiedByCount)
			}
		default:
			t.Errorf("unexpected capability type: %s", cap.CapabilityType)
		}
	}

	// Update a verified capability's count by replacing the profile.
	profile.Capabilities[0].VerifiedByCount = 15
	profile.Capabilities[0].ConfidenceBps = 9800
	k.SetProfile(ctx, profile)

	updated, _ := k.GetProfile(ctx, addr)
	for _, cap := range updated.Capabilities {
		if cap.CapabilityType == "inference" {
			if cap.VerifiedByCount != 15 {
				t.Errorf("expected updated verified_by_count 15, got %d", cap.VerifiedByCount)
			}
			if cap.ConfidenceBps != 9800 {
				t.Errorf("expected updated confidence_bps 9800, got %d", cap.ConfidenceBps)
			}
		}
	}
}

// -----------------------------------------------------------------------
// 29. TestDiscoveryExpiry
// Full expiry lifecycle via BeginBlocker.
// Ported from: OC-DISC-5 / TestExpireStaleProfiles
// -----------------------------------------------------------------------

func TestDiscoveryExpiry(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	// Use short expiry for testing.
	params := types.DefaultParams()
	params.ProfileExpiryBlocks = 200
	k.SetParams(ctx, params)

	addr := testAddr(1000)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	// Register at block 100 (ctx default).
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr,
		Domains: []string{"expiry"},
		Stake:   "1000000",
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// At block 200 (200 % 100 == 0, but 100 + 200 = 300 > 200): not expired yet.
	ctx200 := ctx.WithBlockHeight(200)
	if err := k.BeginBlocker(ctx200); err != nil {
		t.Fatalf("BeginBlocker at 200 failed: %v", err)
	}
	profile, _ := k.GetProfile(ctx200, addr)
	if profile.Status != "active" {
		t.Errorf("expected active at block 200, got %s", profile.Status)
	}

	// At block 400 (400 % 100 == 0, and 100 + 200 = 300 < 400): should expire.
	ctx400 := ctx.WithBlockHeight(400)
	if err := k.BeginBlocker(ctx400); err != nil {
		t.Fatalf("BeginBlocker at 400 failed: %v", err)
	}
	profile, _ = k.GetProfile(ctx400, addr)
	if profile.Status != "expired" {
		t.Errorf("expected expired at block 400, got %s", profile.Status)
	}

	// Expired agent should NOT appear in domain search.
	results := k.GetProfilesByDomain(ctx400, "expiry")
	if len(results) != 0 {
		t.Errorf("expected 0 active profiles in expiry domain after expiration, got %d", len(results))
	}

	// At a non-100 block (401): no expiry processing.
	// Re-activate the profile and verify it stays active.
	profile.Status = "active"
	profile.LastActiveBlock = 100 // still old
	k.SetProfile(ctx400, profile)

	ctx401 := ctx.WithBlockHeight(401)
	if err := k.BeginBlocker(ctx401); err != nil {
		t.Fatalf("BeginBlocker at 401 failed: %v", err)
	}
	check, _ := k.GetProfile(ctx401, addr)
	if check.Status != "active" {
		t.Errorf("expected no expiry at non-100 block, but profile was %s", check.Status)
	}
}

// -----------------------------------------------------------------------
// 30. TestDiscoveryReactivation
// Heartbeat reactivation from expired state with full verification.
// Ported from: OC-DISC-5 (HeartbeatReactivation)
// -----------------------------------------------------------------------

func TestDiscoveryReactivation(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	// Short expiry.
	params := types.DefaultParams()
	params.ProfileExpiryBlocks = 200
	k.SetParams(ctx, params)

	addr := testAddr(1100)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	// Register at block 100.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:  addr,
		Domains: []string{"reactivation"},
		Capabilities: []*types.AgentCapability{
			{CapabilityType: "inference"},
		},
		Stake: "1000000",
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Expire at block 400.
	ctx400 := ctx.WithBlockHeight(400)
	if err := k.BeginBlocker(ctx400); err != nil {
		t.Fatalf("BeginBlocker failed: %v", err)
	}
	profile, _ := k.GetProfile(ctx400, addr)
	if profile.Status != "expired" {
		t.Fatalf("expected expired at block 400, got %s", profile.Status)
	}

	// Expired profile should not appear in searches.
	results := k.SearchProfiles(ctx400, "reactivation", "", 0)
	if len(results) != 0 {
		t.Errorf("expected 0 active profiles when expired, got %d", len(results))
	}

	// Send heartbeat to reactivate at block 500.
	ctx500 := ctx.WithBlockHeight(500)
	_, err = srv.Heartbeat(ctx500, &types.MsgHeartbeat{
		Sender: addr,
	})
	if err != nil {
		t.Fatalf("heartbeat reactivation failed: %v", err)
	}

	profile, _ = k.GetProfile(ctx500, addr)
	if profile.Status != "active" {
		t.Errorf("expected active after heartbeat, got %s", profile.Status)
	}
	if profile.LastActiveBlock != 500 {
		t.Errorf("expected last_active_block 500, got %d", profile.LastActiveBlock)
	}

	// Reactivated profile should now appear in searches.
	results = k.SearchProfiles(ctx500, "reactivation", "", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 active profile after reactivation, got %d", len(results))
	}

	// Verify capability index still works after reactivation.
	capResults := k.GetProfilesByCapability(ctx500, "inference")
	if len(capResults) != 1 {
		t.Errorf("expected 1 inference profile after reactivation, got %d", len(capResults))
	}
}

// -----------------------------------------------------------------------
// 31. TestDiscoveryExpiryNotInSearch
// Verify expired profiles are excluded from all search pathways.
// Ported from: OC-DISC-9 (ExpiredAgentNotInQueries)
// -----------------------------------------------------------------------

func TestDiscoveryExpiryNotInSearch(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	srv := keeper.NewMsgServerImpl(k)

	// Register two agents in same domain.
	addr1 := testAddr(1200)
	accAddr1, _ := sdk.AccAddressFromBech32(addr1)
	bk.setBalance(accAddr1, "uzrn", sdkmath.NewInt(500000000))

	addr2 := testAddr(1201)
	accAddr2, _ := sdk.AccAddressFromBech32(addr2)
	bk.setBalance(accAddr2, "uzrn", sdkmath.NewInt(500000000))

	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:       addr1,
		Domains:      []string{"shared"},
		Capabilities: []*types.AgentCapability{{CapabilityType: "inference"}},
		Stake:        "1000000",
	})
	if err != nil {
		t.Fatalf("registration 1 failed: %v", err)
	}

	_, err = srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:       addr2,
		Domains:      []string{"shared"},
		Capabilities: []*types.AgentCapability{{CapabilityType: "inference"}},
		Stake:        "1000000",
	})
	if err != nil {
		t.Fatalf("registration 2 failed: %v", err)
	}

	// Both should appear in search.
	results := k.SearchProfiles(ctx, "shared", "", 0)
	if len(results) != 2 {
		t.Errorf("expected 2 active profiles, got %d", len(results))
	}

	// Manually expire agent 1.
	profile1, _ := k.GetProfile(ctx, addr1)
	profile1.Status = "expired"
	k.SetProfile(ctx, profile1)

	// Domain search: only agent 2 should appear.
	results = k.GetProfilesByDomain(ctx, "shared")
	if len(results) != 1 {
		t.Errorf("expected 1 active domain profile, got %d", len(results))
	}
	for _, p := range results {
		if p.Address == addr1 {
			t.Error("expired agent1 appears in domain query results")
		}
	}

	// Capability search: only agent 2 should appear.
	results = k.GetProfilesByCapability(ctx, "inference")
	if len(results) != 1 {
		t.Errorf("expected 1 active capability profile, got %d", len(results))
	}
	for _, p := range results {
		if p.Address == addr1 {
			t.Error("expired agent1 appears in capability query results")
		}
	}

	// SearchProfiles (combined): only agent 2 should appear.
	results = k.SearchProfiles(ctx, "shared", "inference", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 active combined search profile, got %d", len(results))
	}

	// SearchProfiles (no filter): only agent 2 should appear.
	results = k.SearchProfiles(ctx, "", "", 0)
	if len(results) != 1 {
		t.Errorf("expected 1 active profile in unfiltered search, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// 32. TestDiscoveryDoubleRegistration
// Double registration must fail and preserve original profile data.
// Ported from: OC-DISC-8 (DoubleRegistration)
// -----------------------------------------------------------------------

func TestDiscoveryDoubleRegistration(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	addr := testAddr(1300)
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	bk.setBalance(accAddr, "uzrn", sdkmath.NewInt(500000000))

	srv := keeper.NewMsgServerImpl(k)

	// First registration.
	_, err := srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "Original",
		Domains:     []string{"math"},
		Stake:       "5000000",
		Description: "First registration",
	})
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration must fail.
	_, err = srv.RegisterProfile(ctx, &types.MsgRegisterProfile{
		Sender:      addr,
		DisplayName: "Duplicate",
		Domains:     []string{"physics"},
		Stake:       "10000000",
		Description: "Second registration attempt",
	})
	if err == nil {
		t.Error("double registration accepted; allows overwriting existing profile")
	}

	// Verify original profile data is intact.
	profile, found := k.GetProfile(ctx, addr)
	if !found {
		t.Fatal("original profile disappeared after failed second registration")
	}
	if profile.DisplayName != "Original" {
		t.Errorf("expected display_name 'Original', got '%s'", profile.DisplayName)
	}
	if profile.Description != "First registration" {
		t.Errorf("expected description 'First registration', got '%s'", profile.Description)
	}
	if profile.ReputationScore != 500000 {
		t.Errorf("expected initial reputation 500000, got %d; profile may have been overwritten", profile.ReputationScore)
	}
	if profile.Stake != "5000000" {
		t.Errorf("expected stake 5000000, got %s; stake may have been overwritten", profile.Stake)
	}

	// Verify only the first registration's stake was deducted.
	expectedBal := sdkmath.NewInt(495000000) // 500000000 - 5000000
	actualBal := bk.balances[accAddr.String()+"/uzrn"]
	if !actualBal.Equal(expectedBal) {
		t.Errorf("expected balance %s, got %s; second registration may have deducted stake", expectedBal, actualBal)
	}
}
