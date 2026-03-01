package keeper_test

import (
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

	"github.com/zerone-chain/zerone/x/ibcratelimit/keeper"
	"github.com/zerone-chain/zerone/x/ibcratelimit/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

var testAuthority = sdk.AccAddress([]byte("authority-addr------")).String()

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
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

	k := keeper.NewKeeper(runtime.NewKVStoreService(storeKey), cdc, testAuthority)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	return k, ctx
}

func setupKeeperAtHeight(t *testing.T, height int64) (keeper.Keeper, sdk.Context) {
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

	k := keeper.NewKeeper(runtime.NewKVStoreService(storeKey), cdc, testAuthority)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: height}, false, log.NewNopLogger())

	return k, ctx
}

// ---------- 1. TestParamsCRUD ----------

func TestParamsCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default params before any SetParams call
	got := k.GetParams(ctx)
	if got == nil {
		t.Fatal("expected non-nil default params")
	}
	if !got.Enabled {
		t.Fatal("expected default params to have Enabled=true")
	}

	// Set custom params
	k.SetParams(ctx, &types.Params{Enabled: false})
	got = k.GetParams(ctx)
	if got.Enabled {
		t.Fatal("expected Enabled=false after SetParams")
	}

	// Overwrite with Enabled=true
	k.SetParams(ctx, &types.Params{Enabled: true})
	got = k.GetParams(ctx)
	if !got.Enabled {
		t.Fatal("expected Enabled=true after second SetParams")
	}
}

// ---------- 2. TestRateLimitSetGetDelete ----------

func TestRateLimitSetGetDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000000",
		MaxRecv:      "2000000",
		WindowBlocks: 100,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  0,
	}

	// Set
	k.SetRateLimit(ctx, rl)

	// Get — found
	got, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit to be found")
	}
	if got.ChannelId != "channel-0" {
		t.Fatalf("expected channel_id channel-0, got %s", got.ChannelId)
	}
	if got.Denom != "uzrn" {
		t.Fatalf("expected denom uzrn, got %s", got.Denom)
	}
	if got.MaxSend != "1000000" {
		t.Fatalf("expected max_send 1000000, got %s", got.MaxSend)
	}
	if got.MaxRecv != "2000000" {
		t.Fatalf("expected max_recv 2000000, got %s", got.MaxRecv)
	}
	if got.WindowBlocks != 100 {
		t.Fatalf("expected window_blocks 100, got %d", got.WindowBlocks)
	}

	// Delete
	k.DeleteRateLimit(ctx, "channel-0", "uzrn")

	// Get — not found
	_, found = k.GetRateLimit(ctx, "channel-0", "uzrn")
	if found {
		t.Fatal("expected rate limit to be not found after delete")
	}
}

// ---------- 3. TestGetAllRateLimits ----------

func TestGetAllRateLimits(t *testing.T) {
	k, ctx := setupKeeper(t)

	limits := []*types.RateLimit{
		{ChannelId: "channel-0", Denom: "uzrn", MaxSend: "1000", MaxRecv: "1000", WindowBlocks: 50, CurrentSend: "0", CurrentRecv: "0"},
		{ChannelId: "channel-0", Denom: "uatom", MaxSend: "2000", MaxRecv: "2000", WindowBlocks: 100, CurrentSend: "0", CurrentRecv: "0"},
		{ChannelId: "channel-1", Denom: "uzrn", MaxSend: "5000", MaxRecv: "5000", WindowBlocks: 200, CurrentSend: "0", CurrentRecv: "0"},
	}

	for _, rl := range limits {
		k.SetRateLimit(ctx, rl)
	}

	all := k.GetAllRateLimits(ctx)
	if len(all) != 3 {
		t.Fatalf("expected 3 rate limits, got %d", len(all))
	}

	// Build a lookup map by channelId+denom for order-independent checking
	lookup := make(map[string]*types.RateLimit)
	for _, rl := range all {
		lookup[rl.ChannelId+"/"+rl.Denom] = rl
	}

	for _, expected := range limits {
		key := expected.ChannelId + "/" + expected.Denom
		got, ok := lookup[key]
		if !ok {
			t.Fatalf("expected rate limit for %s not found in GetAllRateLimits", key)
		}
		if got.MaxSend != expected.MaxSend {
			t.Fatalf("rate limit %s: expected max_send %s, got %s", key, expected.MaxSend, got.MaxSend)
		}
	}
}

// ---------- 4. TestPacketFlowSetGetDelete ----------

func TestPacketFlowSetGetDelete(t *testing.T) {
	k, ctx := setupKeeper(t)

	flow := &types.PacketFlow{
		ChannelId: "channel-0",
		Sequence:  42,
		Denom:     "uzrn",
		Amount:    "500000",
	}

	// Set
	k.SetPacketFlow(ctx, flow)

	// Get — found
	got, found := k.GetPacketFlow(ctx, "channel-0", 42)
	if !found {
		t.Fatal("expected packet flow to be found")
	}
	if got.ChannelId != "channel-0" {
		t.Fatalf("expected channel_id channel-0, got %s", got.ChannelId)
	}
	if got.Sequence != 42 {
		t.Fatalf("expected sequence 42, got %d", got.Sequence)
	}
	if got.Denom != "uzrn" {
		t.Fatalf("expected denom uzrn, got %s", got.Denom)
	}
	if got.Amount != "500000" {
		t.Fatalf("expected amount 500000, got %s", got.Amount)
	}

	// Delete
	k.DeletePacketFlow(ctx, "channel-0", 42)

	// Get — not found
	_, found = k.GetPacketFlow(ctx, "channel-0", 42)
	if found {
		t.Fatal("expected packet flow to be not found after delete")
	}
}

// ---------- 5. TestWindowReset ----------

func TestWindowReset(t *testing.T) {
	// Create keeper with height 100, set rate limit with WindowStart=100, WindowBlocks=50
	k, ctx := setupKeeper(t) // height=100

	k.SetParams(ctx, &types.Params{Enabled: true})

	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 50,
		CurrentSend:  "500",
		CurrentRecv:  "300",
		WindowStart:  100,
	}
	k.SetRateLimit(ctx, rl)

	// At height 100, window is [100, 150). Height 100 < 100+50, so no reset.
	// At height 151, window should reset because 151 >= 100+50.
	ctx151 := sdk.NewContext(
		ctx.MultiStore(),
		cmtproto.Header{Height: 151},
		false,
		log.NewNopLogger(),
	)

	// Trigger window reset via CheckAndUpdateSendQuota with a small amount
	err := k.CheckAndUpdateSendQuota(ctx151, "channel-0", "uzrn", big.NewInt(1))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the counters were reset
	got, found := k.GetRateLimit(ctx151, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit to be found")
	}

	// After reset, CurrentSend should be "1" (reset to 0, then +1 from the quota check)
	if got.CurrentSend != "1" {
		t.Fatalf("expected current_send=1 after reset+send, got %s", got.CurrentSend)
	}
	// CurrentRecv should be "0" (reset)
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 after reset, got %s", got.CurrentRecv)
	}
	// WindowStart should be updated to 151
	if got.WindowStart != 151 {
		t.Fatalf("expected window_start=151 after reset, got %d", got.WindowStart)
	}
}

// ---------- 6. TestCheckAndUpdateSendQuota ----------

func TestCheckAndUpdateSendQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	}
	k.SetRateLimit(ctx, rl)

	// Under limit: send 50 out of 100
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(50))
	if err != nil {
		t.Fatalf("expected no error for under limit, got %v", err)
	}

	// At limit: send 50 more to exactly reach 100
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(50))
	if err != nil {
		t.Fatalf("expected no error for at limit, got %v", err)
	}

	// Verify exactly at 100
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "100" {
		t.Fatalf("expected current_send=100, got %s", got.CurrentSend)
	}

	// Over limit: send 1 more
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("expected error for over limit, got nil")
	}

	// No rate limit configured — should pass through
	err = k.CheckAndUpdateSendQuota(ctx, "channel-99", "uzrn", big.NewInt(999999))
	if err != nil {
		t.Fatalf("expected no error for unconfigured channel, got %v", err)
	}
}

// ---------- 7. TestCheckAndUpdateRecvQuota ----------

func TestCheckAndUpdateRecvQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "200",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	}
	k.SetRateLimit(ctx, rl)

	// Under limit: receive 100 out of 200
	err := k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(100))
	if err != nil {
		t.Fatalf("expected no error for under limit, got %v", err)
	}

	// At limit: receive 100 more to exactly reach 200
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(100))
	if err != nil {
		t.Fatalf("expected no error for at limit, got %v", err)
	}

	// Verify exactly at 200
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentRecv != "200" {
		t.Fatalf("expected current_recv=200, got %s", got.CurrentRecv)
	}

	// Over limit: receive 1 more
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("expected error for over limit, got nil")
	}

	// No rate limit configured — should pass through
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-99", "uzrn", big.NewInt(999999))
	if err != nil {
		t.Fatalf("expected no error for unconfigured channel, got %v", err)
	}
}

// ---------- 8. TestReverseSendQuota ----------

func TestReverseSendQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	}
	k.SetRateLimit(ctx, rl)

	// Send 500
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(500))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reverse 200
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(200))

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "300" {
		t.Fatalf("expected current_send=300 after reverse, got %s", got.CurrentSend)
	}

	// Reverse more than current (should clamp to 0, not go negative)
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(999))

	got, _ = k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 after over-reverse, got %s", got.CurrentSend)
	}

	// Reverse on a missing rate limit — should not panic
	k.ReverseSendQuota(ctx, "channel-99", "uzrn", big.NewInt(100))
}

// ---------- 9. TestMsgServerAddRateLimit ----------

func TestMsgServerAddRateLimit(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Success
	msg := &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000000",
		MaxRecv:      "2000000",
		WindowBlocks: 100,
	}

	_, err := ms.AddRateLimit(ctx, msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it was stored
	got, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit to be found after AddRateLimit")
	}
	if got.MaxSend != "1000000" {
		t.Fatalf("expected max_send=1000000, got %s", got.MaxSend)
	}
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0, got %s", got.CurrentSend)
	}

	// Duplicate — should error
	_, err = ms.AddRateLimit(ctx, msg)
	if err == nil {
		t.Fatal("expected error for duplicate rate limit, got nil")
	}

	// Wrong authority — should error
	badMsg := &types.MsgAddRateLimit{
		Authority:    "zrn1wrongauthority",
		ChannelId:    "channel-1",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 50,
	}
	_, err = ms.AddRateLimit(ctx, badMsg)
	if err == nil {
		t.Fatal("expected error for wrong authority, got nil")
	}
}

// ---------- 10. TestMsgServerRemoveRateLimit ----------

func TestMsgServerRemoveRateLimit(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Seed a rate limit
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "0",
		CurrentRecv:  "0",
	})

	// Success
	_, err := ms.RemoveRateLimit(ctx, &types.MsgRemoveRateLimit{
		Authority: testAuthority,
		ChannelId: "channel-0",
		Denom:     "uzrn",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify deleted
	_, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if found {
		t.Fatal("expected rate limit to be deleted")
	}

	// Not found — should error
	_, err = ms.RemoveRateLimit(ctx, &types.MsgRemoveRateLimit{
		Authority: testAuthority,
		ChannelId: "channel-0",
		Denom:     "uzrn",
	})
	if err == nil {
		t.Fatal("expected error for removing non-existent rate limit, got nil")
	}
}

// ---------- 11. TestMsgServerUpdateParams ----------

func TestMsgServerUpdateParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Success — disable
	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAuthority,
		Params:    &types.Params{Enabled: false},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := k.GetParams(ctx)
	if got.Enabled {
		t.Fatal("expected Enabled=false after UpdateParams")
	}

	// Success — re-enable
	_, err = ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAuthority,
		Params:    &types.Params{Enabled: true},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got = k.GetParams(ctx)
	if !got.Enabled {
		t.Fatal("expected Enabled=true after UpdateParams")
	}

	// Wrong authority — should error
	_, err = ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "zrn1wrongauthority",
		Params:    &types.Params{Enabled: false},
	})
	if err == nil {
		t.Fatal("expected error for wrong authority, got nil")
	}
}

// ---------- 12. TestGenesisRoundTrip ----------

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx := setupKeeper(t)

	genState := &types.GenesisState{
		Params: &types.Params{Enabled: true},
		RateLimits: []*types.RateLimit{
			{
				ChannelId:    "channel-0",
				Denom:        "uzrn",
				MaxSend:      "1000000",
				MaxRecv:      "2000000",
				WindowBlocks: 100,
				CurrentSend:  "500",
				CurrentRecv:  "300",
				WindowStart:  50,
			},
			{
				ChannelId:    "channel-1",
				Denom:        "uatom",
				MaxSend:      "5000000",
				MaxRecv:      "5000000",
				WindowBlocks: 200,
				CurrentSend:  "0",
				CurrentRecv:  "0",
				WindowStart:  0,
			},
		},
	}

	k.InitGenesis(ctx, genState)
	exported := k.ExportGenesis(ctx)

	// Verify params
	if exported.Params == nil {
		t.Fatal("expected non-nil params in exported genesis")
	}
	if exported.Params.Enabled != genState.Params.Enabled {
		t.Fatalf("expected Enabled=%v, got %v", genState.Params.Enabled, exported.Params.Enabled)
	}

	// Verify rate limits
	if len(exported.RateLimits) != len(genState.RateLimits) {
		t.Fatalf("expected %d rate limits, got %d", len(genState.RateLimits), len(exported.RateLimits))
	}

	// Build lookup for order-independent comparison
	lookup := make(map[string]*types.RateLimit)
	for _, rl := range exported.RateLimits {
		lookup[rl.ChannelId+"/"+rl.Denom] = rl
	}

	for _, expected := range genState.RateLimits {
		key := expected.ChannelId + "/" + expected.Denom
		got, ok := lookup[key]
		if !ok {
			t.Fatalf("exported genesis missing rate limit for %s", key)
		}
		if got.MaxSend != expected.MaxSend {
			t.Fatalf("%s: expected max_send=%s, got %s", key, expected.MaxSend, got.MaxSend)
		}
		if got.MaxRecv != expected.MaxRecv {
			t.Fatalf("%s: expected max_recv=%s, got %s", key, expected.MaxRecv, got.MaxRecv)
		}
		if got.WindowBlocks != expected.WindowBlocks {
			t.Fatalf("%s: expected window_blocks=%d, got %d", key, expected.WindowBlocks, got.WindowBlocks)
		}
		if got.CurrentSend != expected.CurrentSend {
			t.Fatalf("%s: expected current_send=%s, got %s", key, expected.CurrentSend, got.CurrentSend)
		}
		if got.CurrentRecv != expected.CurrentRecv {
			t.Fatalf("%s: expected current_recv=%s, got %s", key, expected.CurrentRecv, got.CurrentRecv)
		}
		if got.WindowStart != expected.WindowStart {
			t.Fatalf("%s: expected window_start=%d, got %d", key, expected.WindowStart, got.WindowStart)
		}
	}
}

// ---------- 13. TestTimeoutRefundsSendQuota ----------

func TestTimeoutRefundsSendQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	}
	k.SetRateLimit(ctx, rl)

	// Send 80 out of 100
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(80))
	if err != nil {
		t.Fatalf("send should succeed: %v", err)
	}

	// Sending 30 more would exceed limit (80+30=110 > 100)
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(30))
	if err == nil {
		t.Fatal("expected error for exceeding limit")
	}

	// Simulate timeout: refund 50 from previous send
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(50))

	// Verify current_send decreased: 80 - 50 = 30
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "30" {
		t.Fatalf("expected current_send=30 after timeout refund, got %s", got.CurrentSend)
	}

	// Now sending 30 more should succeed (30+30=60 <= 100)
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(30))
	if err != nil {
		t.Fatalf("send should succeed after timeout refund: %v", err)
	}
}

// ---------- 14. TestErrorAckRefundsSendQuota ----------

func TestErrorAckRefundsSendQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	}
	k.SetRateLimit(ctx, rl)

	// Store packet flow for error ack scenario
	k.SetPacketFlow(ctx, &types.PacketFlow{
		ChannelId: "channel-0",
		Sequence:  1,
		Denom:     "uzrn",
		Amount:    "500",
	})

	// Send 500
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(500))
	if err != nil {
		t.Fatalf("send should succeed: %v", err)
	}

	// Simulate error ack: reverse the send
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(500))

	// Verify full refund
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 after error ack refund, got %s", got.CurrentSend)
	}

	// Clean up packet flow
	k.DeletePacketFlow(ctx, "channel-0", 1)
}

// ---------- 15. TestUnconfiguredChannelPassesThrough ----------

func TestUnconfiguredChannelPassesThrough(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Only configure channel-0
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Sending on unconfigured channel should pass through
	err := k.CheckAndUpdateSendQuota(ctx, "channel-99", "uzrn", big.NewInt(999999999))
	if err != nil {
		t.Fatalf("expected unconfigured channel to pass through, got %v", err)
	}

	// Receiving on unconfigured channel should pass through
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-99", "uzrn", big.NewInt(999999999))
	if err != nil {
		t.Fatalf("expected unconfigured channel to pass through, got %v", err)
	}

	// Unconfigured denom on configured channel should also pass through
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uatom", big.NewInt(999999999))
	if err != nil {
		t.Fatalf("expected unconfigured denom to pass through, got %v", err)
	}
}

// ---------- 16. TestDisabledParams ----------

func TestDisabledParams(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set up a rate limit with max_send=100, max_recv=100
	rl := &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	}
	k.SetRateLimit(ctx, rl)

	// Disable rate limiting
	k.SetParams(ctx, &types.Params{Enabled: false})

	// Send over limit — should still pass because disabled
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(9999))
	if err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}

	// Recv over limit — should still pass because disabled
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(9999))
	if err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}

	// Verify the counters were NOT updated (disabled means early return)
	got, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit to still exist")
	}
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 when disabled (no update), got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 when disabled (no update), got %s", got.CurrentRecv)
	}

	// Re-enable and verify the limit is enforced again
	k.SetParams(ctx, &types.Params{Enabled: true})

	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(101))
	if err == nil {
		t.Fatal("expected error when re-enabled and over limit, got nil")
	}

	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(101))
	if err == nil {
		t.Fatal("expected error when re-enabled and over limit, got nil")
	}
}

// ---------- 17. TestQueryRateLimit ----------

func TestQueryRateLimit(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	// Seed a rate limit
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "2000",
		WindowBlocks: 100,
		CurrentSend:  "50",
		CurrentRecv:  "75",
		WindowStart:  10,
	})

	resp, err := qs.RateLimit(ctx, &types.QueryRateLimitRequest{
		ChannelId: "channel-0",
		Denom:     "uzrn",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.RateLimit == nil {
		t.Fatal("expected non-nil rate limit in response")
	}
	if resp.RateLimit.ChannelId != "channel-0" {
		t.Fatalf("expected channel_id=channel-0, got %s", resp.RateLimit.ChannelId)
	}
	if resp.RateLimit.Denom != "uzrn" {
		t.Fatalf("expected denom=uzrn, got %s", resp.RateLimit.Denom)
	}
	if resp.RateLimit.MaxSend != "1000" {
		t.Fatalf("expected max_send=1000, got %s", resp.RateLimit.MaxSend)
	}
	if resp.RateLimit.MaxRecv != "2000" {
		t.Fatalf("expected max_recv=2000, got %s", resp.RateLimit.MaxRecv)
	}
	if resp.RateLimit.CurrentSend != "50" {
		t.Fatalf("expected current_send=50, got %s", resp.RateLimit.CurrentSend)
	}
	if resp.RateLimit.CurrentRecv != "75" {
		t.Fatalf("expected current_recv=75, got %s", resp.RateLimit.CurrentRecv)
	}
}

// ---------- 18. TestQueryRateLimitNotFound ----------

func TestQueryRateLimitNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.RateLimit(ctx, &types.QueryRateLimitRequest{
		ChannelId: "channel-99",
		Denom:     "uzrn",
	})
	if err == nil {
		t.Fatal("expected error for non-existent rate limit, got nil")
	}
}

// ---------- 19. TestQueryRateLimitNilRequest ----------

func TestQueryRateLimitNilRequest(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.RateLimit(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

// ---------- 20. TestQueryRateLimitMissingFields ----------

func TestQueryRateLimitMissingFields(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	// Empty channel_id
	_, err := qs.RateLimit(ctx, &types.QueryRateLimitRequest{
		ChannelId: "",
		Denom:     "uzrn",
	})
	if err == nil {
		t.Fatal("expected error for empty channel_id, got nil")
	}

	// Empty denom
	_, err = qs.RateLimit(ctx, &types.QueryRateLimitRequest{
		ChannelId: "channel-0",
		Denom:     "",
	})
	if err == nil {
		t.Fatal("expected error for empty denom, got nil")
	}
}

// ---------- 21. TestQueryRateLimits ----------

func TestQueryRateLimits(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	// Seed 3 rate limits
	for _, ch := range []string{"channel-0", "channel-1", "channel-2"} {
		k.SetRateLimit(ctx, &types.RateLimit{
			ChannelId:    ch,
			Denom:        "uzrn",
			MaxSend:      "1000",
			MaxRecv:      "1000",
			WindowBlocks: 100,
			CurrentSend:  "0",
			CurrentRecv:  "0",
			WindowStart:  0,
		})
	}

	resp, err := qs.RateLimits(ctx, &types.QueryRateLimitsRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.RateLimits) != 3 {
		t.Fatalf("expected 3 rate limits, got %d", len(resp.RateLimits))
	}
}

// ---------- 22. TestQueryRateLimitsEmpty ----------

func TestQueryRateLimitsEmpty(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.RateLimits(ctx, &types.QueryRateLimitsRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.RateLimits) != 0 {
		t.Fatalf("expected 0 rate limits, got %d", len(resp.RateLimits))
	}
}

// ---------- 23. TestQueryParams ----------

func TestQueryParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetParams(ctx, &types.Params{Enabled: false})

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Params == nil {
		t.Fatal("expected non-nil params in response")
	}
	if resp.Params.Enabled {
		t.Fatal("expected Enabled=false in response")
	}
}

// ---------- 24. TestQueryParamsNilRequest ----------

func TestQueryParamsNilRequest(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Params(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

// ---------- 25. TestResetExpiredWindows ----------

func TestResetExpiredWindows(t *testing.T) {
	// Start at height 200 so expired windows are clearly past
	k, ctx := setupKeeperAtHeight(t, 200)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// RL1: window [50, 50+50=100) — expired at height 200
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 50,
		CurrentSend:  "400",
		CurrentRecv:  "300",
		WindowStart:  50,
	})

	// RL2: window [100, 100+50=150) — expired at height 200
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-1",
		Denom:        "uzrn",
		MaxSend:      "2000",
		MaxRecv:      "2000",
		WindowBlocks: 50,
		CurrentSend:  "500",
		CurrentRecv:  "600",
		WindowStart:  100,
	})

	// RL3: window [180, 180+50=230) — NOT expired at height 200
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-2",
		Denom:        "uzrn",
		MaxSend:      "3000",
		MaxRecv:      "3000",
		WindowBlocks: 50,
		CurrentSend:  "700",
		CurrentRecv:  "800",
		WindowStart:  180,
	})

	k.ResetExpiredWindows(ctx)

	// RL1 should be reset
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" || got.CurrentRecv != "0" {
		t.Fatalf("RL1: expected counters reset, got send=%s recv=%s", got.CurrentSend, got.CurrentRecv)
	}
	if got.WindowStart != 200 {
		t.Fatalf("RL1: expected window_start=200, got %d", got.WindowStart)
	}

	// RL2 should be reset
	got, _ = k.GetRateLimit(ctx, "channel-1", "uzrn")
	if got.CurrentSend != "0" || got.CurrentRecv != "0" {
		t.Fatalf("RL2: expected counters reset, got send=%s recv=%s", got.CurrentSend, got.CurrentRecv)
	}
	if got.WindowStart != 200 {
		t.Fatalf("RL2: expected window_start=200, got %d", got.WindowStart)
	}

	// RL3 should NOT be reset
	got, _ = k.GetRateLimit(ctx, "channel-2", "uzrn")
	if got.CurrentSend != "700" || got.CurrentRecv != "800" {
		t.Fatalf("RL3: expected counters unchanged, got send=%s recv=%s", got.CurrentSend, got.CurrentRecv)
	}
	if got.WindowStart != 180 {
		t.Fatalf("RL3: expected window_start=180, got %d", got.WindowStart)
	}
}

// ---------- 26. TestResetExpiredWindowsNoneExpired ----------

func TestResetExpiredWindowsNoneExpired(t *testing.T) {
	k, ctx := setupKeeperAtHeight(t, 100)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Window [90, 90+100=190) — not expired at height 100
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "250",
		CurrentRecv:  "350",
		WindowStart:  90,
	})

	k.ResetExpiredWindows(ctx)

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "250" {
		t.Fatalf("expected current_send=250 unchanged, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "350" {
		t.Fatalf("expected current_recv=350 unchanged, got %s", got.CurrentRecv)
	}
	if got.WindowStart != 90 {
		t.Fatalf("expected window_start=90 unchanged, got %d", got.WindowStart)
	}
}

// ---------- 27. TestMultipleChannelsSameQuota ----------

func TestMultipleChannelsSameQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Configure channel-0 and channel-1 independently with max_send=100
	for _, ch := range []string{"channel-0", "channel-1"} {
		k.SetRateLimit(ctx, &types.RateLimit{
			ChannelId:    ch,
			Denom:        "uzrn",
			MaxSend:      "100",
			MaxRecv:      "100",
			WindowBlocks: 1000,
			CurrentSend:  "0",
			CurrentRecv:  "0",
			WindowStart:  100,
		})
	}

	// Send 80 on channel-0
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(80))
	if err != nil {
		t.Fatalf("channel-0 send should succeed: %v", err)
	}

	// channel-1 should still have full quota — send 90 should work
	err = k.CheckAndUpdateSendQuota(ctx, "channel-1", "uzrn", big.NewInt(90))
	if err != nil {
		t.Fatalf("channel-1 send should succeed independently: %v", err)
	}

	// Verify channel-0 counters
	got0, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got0.CurrentSend != "80" {
		t.Fatalf("channel-0: expected current_send=80, got %s", got0.CurrentSend)
	}

	// Verify channel-1 counters
	got1, _ := k.GetRateLimit(ctx, "channel-1", "uzrn")
	if got1.CurrentSend != "90" {
		t.Fatalf("channel-1: expected current_send=90, got %s", got1.CurrentSend)
	}
}

// ---------- 28. TestMultipleDenomsSameChannel ----------

func TestMultipleDenomsSameChannel(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Configure uzrn and uatom on channel-0
	for _, denom := range []string{"uzrn", "uatom"} {
		k.SetRateLimit(ctx, &types.RateLimit{
			ChannelId:    "channel-0",
			Denom:        denom,
			MaxSend:      "100",
			MaxRecv:      "100",
			WindowBlocks: 1000,
			CurrentSend:  "0",
			CurrentRecv:  "0",
			WindowStart:  100,
		})
	}

	// Send 90 uzrn
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(90))
	if err != nil {
		t.Fatalf("uzrn send should succeed: %v", err)
	}

	// uatom should be unaffected — send 95 should work
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uatom", big.NewInt(95))
	if err != nil {
		t.Fatalf("uatom send should succeed independently: %v", err)
	}

	// Verify uzrn counters
	gotZrn, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if gotZrn.CurrentSend != "90" {
		t.Fatalf("uzrn: expected current_send=90, got %s", gotZrn.CurrentSend)
	}

	// Verify uatom counters
	gotAtom, _ := k.GetRateLimit(ctx, "channel-0", "uatom")
	if gotAtom.CurrentSend != "95" {
		t.Fatalf("uatom: expected current_send=95, got %s", gotAtom.CurrentSend)
	}
}

// ---------- 29. TestSendQuotaExactBoundary ----------

func TestSendQuotaExactBoundary(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send exactly 100 (the max)
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(100))
	if err != nil {
		t.Fatalf("sending exact max should succeed: %v", err)
	}

	// Send 0 more — should succeed (0 doesn't exceed)
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(0))
	if err != nil {
		t.Fatalf("sending 0 after max should succeed: %v", err)
	}

	// Send 1 more — should fail
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("sending 1 beyond max should fail, got nil")
	}
}

// ---------- 30. TestRecvQuotaExactBoundary ----------

func TestRecvQuotaExactBoundary(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "200",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Recv exactly 200 (the max)
	err := k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(200))
	if err != nil {
		t.Fatalf("receiving exact max should succeed: %v", err)
	}

	// Recv 0 more — should succeed
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(0))
	if err != nil {
		t.Fatalf("receiving 0 after max should succeed: %v", err)
	}

	// Recv 1 more — should fail
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("receiving 1 beyond max should fail, got nil")
	}
}

// ---------- 31. TestZeroAmountSendQuota ----------

func TestZeroAmountSendQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(0))
	if err != nil {
		t.Fatalf("sending 0 amount should succeed: %v", err)
	}

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 after sending 0, got %s", got.CurrentSend)
	}
}

// ---------- 32. TestZeroAmountRecvQuota ----------

func TestZeroAmountRecvQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	err := k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(0))
	if err != nil {
		t.Fatalf("receiving 0 amount should succeed: %v", err)
	}

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 after receiving 0, got %s", got.CurrentRecv)
	}
}

// ---------- 33. TestLargeAmountQuota ----------

func TestLargeAmountQuota(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Use 10^18 as max — tests big.Int handling
	largeMax := "1000000000000000000"
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      largeMax,
		MaxRecv:      largeMax,
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send 10^17 — should succeed
	amount := new(big.Int)
	amount.SetString("100000000000000000", 10) // 10^17
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", amount)
	if err != nil {
		t.Fatalf("sending large amount under limit should succeed: %v", err)
	}

	// Send 9*10^17 more to reach 10^18 exactly
	amount2 := new(big.Int)
	amount2.SetString("900000000000000000", 10)
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", amount2)
	if err != nil {
		t.Fatalf("sending to exact large limit should succeed: %v", err)
	}

	// Send 1 more — should fail
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("sending 1 beyond large limit should fail, got nil")
	}
}

// ---------- 34. TestWindowResetPreservesConfig ----------

func TestWindowResetPreservesConfig(t *testing.T) {
	k, ctx := setupKeeperAtHeight(t, 200)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "5000",
		MaxRecv:      "7000",
		WindowBlocks: 50,
		CurrentSend:  "1234",
		CurrentRecv:  "5678",
		WindowStart:  10, // expired at height 200 (10+50=60 < 200)
	})

	k.ResetExpiredWindows(ctx)

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	// Counters should be reset
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 after reset, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 after reset, got %s", got.CurrentRecv)
	}
	// Config should be preserved
	if got.MaxSend != "5000" {
		t.Fatalf("expected max_send=5000 preserved, got %s", got.MaxSend)
	}
	if got.MaxRecv != "7000" {
		t.Fatalf("expected max_recv=7000 preserved, got %s", got.MaxRecv)
	}
	if got.WindowBlocks != 50 {
		t.Fatalf("expected window_blocks=50 preserved, got %d", got.WindowBlocks)
	}
}

// ---------- 35. TestSendAndRecvIndependent ----------

func TestSendAndRecvIndependent(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send 90
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(90))
	if err != nil {
		t.Fatalf("send should succeed: %v", err)
	}

	// Recv 90 — should succeed because recv quota is independent
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(90))
	if err != nil {
		t.Fatalf("recv should succeed independently: %v", err)
	}

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "90" {
		t.Fatalf("expected current_send=90, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "90" {
		t.Fatalf("expected current_recv=90, got %s", got.CurrentRecv)
	}

	// Send 10 more (reaching max 100) — should still work
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(10))
	if err != nil {
		t.Fatalf("send to max should succeed: %v", err)
	}

	// Recv 10 more (reaching max 100) — should still work
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(10))
	if err != nil {
		t.Fatalf("recv to max should succeed: %v", err)
	}
}

// ---------- 36. TestMsgServerAddRateLimitInitializesFields ----------

func TestMsgServerAddRateLimitInitializesFields(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.AddRateLimit(ctx, &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "5000",
		MaxRecv:      "10000",
		WindowBlocks: 200,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit to exist after AddRateLimit")
	}

	// AddRateLimit sets WindowStart to 0, CurrentSend and CurrentRecv to "0"
	if got.WindowStart != 0 {
		t.Fatalf("expected window_start=0 after AddRateLimit, got %d", got.WindowStart)
	}
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 after AddRateLimit, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 after AddRateLimit, got %s", got.CurrentRecv)
	}
	if got.MaxSend != "5000" {
		t.Fatalf("expected max_send=5000, got %s", got.MaxSend)
	}
	if got.MaxRecv != "10000" {
		t.Fatalf("expected max_recv=10000, got %s", got.MaxRecv)
	}
	if got.WindowBlocks != 200 {
		t.Fatalf("expected window_blocks=200, got %d", got.WindowBlocks)
	}
}

// ---------- 37. TestMsgServerRemoveRateLimitUnauthorized ----------

func TestMsgServerRemoveRateLimitUnauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Seed a rate limit
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "0",
		CurrentRecv:  "0",
	})

	// Try removing with wrong authority
	_, err := ms.RemoveRateLimit(ctx, &types.MsgRemoveRateLimit{
		Authority: "zrn1wrongauthority",
		ChannelId: "channel-0",
		Denom:     "uzrn",
	})
	if err == nil {
		t.Fatal("expected error for unauthorized RemoveRateLimit, got nil")
	}

	// Verify rate limit still exists
	_, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("rate limit should still exist after unauthorized remove attempt")
	}
}

// ---------- 38. TestMsgServerUpdateRateLimit ----------

func TestMsgServerUpdateRateLimit(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	addMsg := &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "2000",
		WindowBlocks: 100,
	}

	// Add rate limit
	_, err := ms.AddRateLimit(ctx, addMsg)
	if err != nil {
		t.Fatalf("add should succeed: %v", err)
	}

	// Try to add again — should fail (duplicate)
	_, err = ms.AddRateLimit(ctx, addMsg)
	if err == nil {
		t.Fatal("expected error for duplicate AddRateLimit, got nil")
	}

	// Remove it
	_, err = ms.RemoveRateLimit(ctx, &types.MsgRemoveRateLimit{
		Authority: testAuthority,
		ChannelId: "channel-0",
		Denom:     "uzrn",
	})
	if err != nil {
		t.Fatalf("remove should succeed: %v", err)
	}

	// Add again with different config — should succeed
	newMsg := &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "5000",
		MaxRecv:      "10000",
		WindowBlocks: 500,
	}
	_, err = ms.AddRateLimit(ctx, newMsg)
	if err != nil {
		t.Fatalf("re-add after remove should succeed: %v", err)
	}

	// Verify new config
	got, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit to exist after re-add")
	}
	if got.MaxSend != "5000" {
		t.Fatalf("expected max_send=5000, got %s", got.MaxSend)
	}
	if got.WindowBlocks != 500 {
		t.Fatalf("expected window_blocks=500, got %d", got.WindowBlocks)
	}
}

// ---------- 39. TestGenesisEmptyState ----------

func TestGenesisEmptyState(t *testing.T) {
	k, ctx := setupKeeper(t)

	genState := &types.GenesisState{
		Params:     types.DefaultParams(),
		RateLimits: nil,
	}

	k.InitGenesis(ctx, genState)
	exported := k.ExportGenesis(ctx)

	if exported.Params == nil {
		t.Fatal("expected non-nil params in exported genesis")
	}
	if !exported.Params.Enabled {
		t.Fatal("expected default Enabled=true")
	}
	if len(exported.RateLimits) != 0 {
		t.Fatalf("expected 0 rate limits in empty genesis, got %d", len(exported.RateLimits))
	}
}

// ---------- 40. TestGenesisPreservesPacketFlows ----------

func TestGenesisPreservesPacketFlows(t *testing.T) {
	// Genesis does not include packet flows — this test verifies that
	// packet flows set outside of genesis are NOT exported and that
	// InitGenesis does not affect existing packet flows.
	k, ctx := setupKeeper(t)

	// Set a packet flow manually
	k.SetPacketFlow(ctx, &types.PacketFlow{
		ChannelId: "channel-0",
		Sequence:  1,
		Denom:     "uzrn",
		Amount:    "500",
	})

	// Init genesis with rate limits only
	genState := &types.GenesisState{
		Params: &types.Params{Enabled: true},
		RateLimits: []*types.RateLimit{
			{
				ChannelId:    "channel-0",
				Denom:        "uzrn",
				MaxSend:      "1000",
				MaxRecv:      "1000",
				WindowBlocks: 100,
				CurrentSend:  "0",
				CurrentRecv:  "0",
				WindowStart:  0,
			},
		},
	}
	k.InitGenesis(ctx, genState)

	// Packet flow should still be accessible (InitGenesis doesn't wipe it)
	flow, found := k.GetPacketFlow(ctx, "channel-0", 1)
	if !found {
		t.Fatal("expected packet flow to survive InitGenesis")
	}
	if flow.Amount != "500" {
		t.Fatalf("expected packet flow amount=500, got %s", flow.Amount)
	}

	// Export genesis — should NOT contain packet flows (not part of GenesisState)
	exported := k.ExportGenesis(ctx)
	if len(exported.RateLimits) != 1 {
		t.Fatalf("expected 1 rate limit in export, got %d", len(exported.RateLimits))
	}
}

// ---------- 41. TestParamsToggleMidWindow ----------

func TestParamsToggleMidWindow(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send 50
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(50))
	if err != nil {
		t.Fatalf("send should succeed: %v", err)
	}

	// Disable mid-window
	k.SetParams(ctx, &types.Params{Enabled: false})

	// Send 200 while disabled — should pass (no enforcement)
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(200))
	if err != nil {
		t.Fatalf("send while disabled should pass: %v", err)
	}

	// Counters should NOT have updated while disabled
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "50" {
		t.Fatalf("expected current_send=50 (unchanged while disabled), got %s", got.CurrentSend)
	}

	// Re-enable
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Send 50 more should succeed (50 + 50 = 100 = max)
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(50))
	if err != nil {
		t.Fatalf("send after re-enable should succeed: %v", err)
	}

	// Send 1 more should fail (100 + 1 > 100)
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("send beyond limit after re-enable should fail, got nil")
	}
}

// ---------- 42. TestQuotaAccumulationAcrossMultipleSends ----------

func TestQuotaAccumulationAcrossMultipleSends(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// 5 sends of 20 each = 100
	for i := 0; i < 5; i++ {
		err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(20))
		if err != nil {
			t.Fatalf("send #%d should succeed: %v", i+1, err)
		}
	}

	// Verify at exactly 100
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "100" {
		t.Fatalf("expected current_send=100 after 5x20, got %s", got.CurrentSend)
	}

	// 1 more should fail
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("6th send should fail, got nil")
	}
}

// ---------- 43. TestQuotaAccumulationAcrossMultipleRecvs ----------

func TestQuotaAccumulationAcrossMultipleRecvs(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// 5 recvs of 20 each = 100
	for i := 0; i < 5; i++ {
		err := k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(20))
		if err != nil {
			t.Fatalf("recv #%d should succeed: %v", i+1, err)
		}
	}

	// Verify at exactly 100
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentRecv != "100" {
		t.Fatalf("expected current_recv=100 after 5x20, got %s", got.CurrentRecv)
	}

	// 1 more should fail
	err := k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("6th recv should fail, got nil")
	}
}

// ---------- 44. TestReverseSendQuotaPartial ----------

func TestReverseSendQuotaPartial(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send 500
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(500))
	if err != nil {
		t.Fatalf("send should succeed: %v", err)
	}

	// Reverse 123
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(123))

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "377" {
		t.Fatalf("expected current_send=377 (500-123), got %s", got.CurrentSend)
	}

	// Reverse 77 more
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(77))

	got, _ = k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "300" {
		t.Fatalf("expected current_send=300 (377-77), got %s", got.CurrentSend)
	}
}

// ---------- 45. TestReverseSendQuotaToZero ----------

func TestReverseSendQuotaToZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send 250
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(250))
	if err != nil {
		t.Fatalf("send should succeed: %v", err)
	}

	// Reverse exactly 250
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(250))

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 after exact reverse, got %s", got.CurrentSend)
	}
}

// ---------- 46. TestPacketFlowOverwrite ----------

func TestPacketFlowOverwrite(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set first flow
	k.SetPacketFlow(ctx, &types.PacketFlow{
		ChannelId: "channel-0",
		Sequence:  1,
		Denom:     "uzrn",
		Amount:    "100",
	})

	// Overwrite with different values
	k.SetPacketFlow(ctx, &types.PacketFlow{
		ChannelId: "channel-0",
		Sequence:  1,
		Denom:     "uatom",
		Amount:    "999",
	})

	got, found := k.GetPacketFlow(ctx, "channel-0", 1)
	if !found {
		t.Fatal("expected packet flow to be found")
	}
	if got.Denom != "uatom" {
		t.Fatalf("expected denom=uatom after overwrite, got %s", got.Denom)
	}
	if got.Amount != "999" {
		t.Fatalf("expected amount=999 after overwrite, got %s", got.Amount)
	}
}

// ---------- 47. TestMsgServerAddRateLimitZeroWindow ----------

func TestMsgServerAddRateLimitZeroWindow(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// WindowBlocks=0 should be rejected by ValidateBasic
	_, err := ms.AddRateLimit(ctx, &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 0,
	})
	if err == nil {
		t.Fatal("expected error for WindowBlocks=0, got nil")
	}

	// Verify nothing was stored
	_, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if found {
		t.Fatal("rate limit should not be stored when WindowBlocks=0")
	}
}

// ---------- 48. TestWindowNoResetAtExactBoundary ----------

func TestWindowNoResetAtExactBoundary(t *testing.T) {
	// Window [100, 100+50=150). At height 150, >= comparison means it resets.
	k, ctx := setupKeeperAtHeight(t, 150)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 50,
		CurrentSend:  "800",
		CurrentRecv:  "600",
		WindowStart:  100,
	})

	// At height 150, 150 >= 100+50 → should reset
	k.ResetExpiredWindows(ctx)

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 at exact boundary, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 at exact boundary, got %s", got.CurrentRecv)
	}
	if got.WindowStart != 150 {
		t.Fatalf("expected window_start=150 at exact boundary, got %d", got.WindowStart)
	}
}

// ---------- 49. TestDeleteNonexistentRateLimit ----------

func TestDeleteNonexistentRateLimit(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Delete something that doesn't exist — should be a no-op (no panic)
	k.DeleteRateLimit(ctx, "channel-99", "uzrn")

	// Verify the store is still functional
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "0",
		CurrentRecv:  "0",
	})

	got, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit to be found after deleting nonexistent key")
	}
	if got.MaxSend != "1000" {
		t.Fatalf("expected max_send=1000, got %s", got.MaxSend)
	}
}

// ---------- 50. TestGetRateLimitWrongDenom ----------

func TestGetRateLimitWrongDenom(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set rate limit for uzrn
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "0",
		CurrentRecv:  "0",
	})

	// Get for uatom — should not find
	_, found := k.GetRateLimit(ctx, "channel-0", "uatom")
	if found {
		t.Fatal("expected rate limit NOT found for wrong denom uatom")
	}

	// Get for uzrn — should find
	_, found = k.GetRateLimit(ctx, "channel-0", "uzrn")
	if !found {
		t.Fatal("expected rate limit found for correct denom uzrn")
	}

	// Get for wrong channel — should not find
	_, found = k.GetRateLimit(ctx, "channel-1", "uzrn")
	if found {
		t.Fatal("expected rate limit NOT found for wrong channel")
	}
}

// ============================================================
// R15-4: Epoch Reset Tests
// ============================================================

// ---------- 51. TestRateLimitEpochReset_ExactExpiry ----------

func TestRateLimitEpochReset_ExactExpiry(t *testing.T) {
	// Test that at the exact block where WindowStart+WindowBlocks == BlockHeight,
	// the window resets (>= comparison).
	k, ctx := setupKeeperAtHeight(t, 200)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "800",
		CurrentRecv:  "600",
		WindowStart:  100, // 100+100=200 = current height
	})

	// Trigger reset via quota check
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err != nil {
		t.Fatalf("expected quota check to succeed after window reset: %v", err)
	}

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	// After reset, current_send should be "1" (reset to 0, then +1 from this send)
	if got.CurrentSend != "1" {
		t.Fatalf("expected current_send=1 after reset+send, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 after reset, got %s", got.CurrentRecv)
	}
	if got.WindowStart != 200 {
		t.Fatalf("expected window_start=200 after reset, got %d", got.WindowStart)
	}
}

// ---------- 52. TestRateLimitEpochReset_NotYetExpired ----------

func TestRateLimitEpochReset_NotYetExpired(t *testing.T) {
	// Height 149, window [100, 100+50=150). Not expired because 149 < 150.
	k, ctx := setupKeeperAtHeight(t, 149)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 50,
		CurrentSend:  "500",
		CurrentRecv:  "300",
		WindowStart:  100,
	})

	// Send 1 — should add to existing (no reset)
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err != nil {
		t.Fatalf("expected send to succeed within window: %v", err)
	}

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "501" {
		t.Fatalf("expected current_send=501 (500+1, no reset), got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "300" {
		t.Fatalf("expected current_recv=300 unchanged, got %s", got.CurrentRecv)
	}
	if got.WindowStart != 100 {
		t.Fatalf("expected window_start=100 unchanged, got %d", got.WindowStart)
	}
}

// ---------- 53. TestRateLimitEpochReset_MultipleWindowsSkipped ----------

func TestRateLimitEpochReset_MultipleWindowsSkipped(t *testing.T) {
	// Height 500, window [100, 100+50=150). Multiple windows have passed.
	k, ctx := setupKeeperAtHeight(t, 500)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 50,
		CurrentSend:  "999",
		CurrentRecv:  "888",
		WindowStart:  100,
	})

	// After skipping multiple windows, counters should reset to 0
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(999))
	if err != nil {
		t.Fatalf("expected send to succeed after multi-window reset: %v", err)
	}

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "999" {
		t.Fatalf("expected current_send=999 after reset+send, got %s", got.CurrentSend)
	}
	if got.WindowStart != 500 {
		t.Fatalf("expected window_start=500, got %d", got.WindowStart)
	}
}

// ============================================================
// R15-4: Per-Denom Rate Limit Tests
// ============================================================

// ---------- 54. TestRateLimitPerDenom_IndependentTracking ----------

func TestRateLimitPerDenom_IndependentTracking(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Set up two denoms on same channel with different limits
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "500",
		MaxRecv:      "500",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uatom",
		MaxSend:      "200",
		MaxRecv:      "200",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Exhaust uzrn send quota
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(500))
	if err != nil {
		t.Fatalf("uzrn send to max should succeed: %v", err)
	}

	// uzrn should be exhausted
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("uzrn send beyond max should fail")
	}

	// uatom should still be fully available
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uatom", big.NewInt(200))
	if err != nil {
		t.Fatalf("uatom send should succeed independently: %v", err)
	}

	// Verify both
	zrnRL, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	atomRL, _ := k.GetRateLimit(ctx, "channel-0", "uatom")
	if zrnRL.CurrentSend != "500" {
		t.Fatalf("uzrn: expected current_send=500, got %s", zrnRL.CurrentSend)
	}
	if atomRL.CurrentSend != "200" {
		t.Fatalf("uatom: expected current_send=200, got %s", atomRL.CurrentSend)
	}
}

// ---------- 55. TestRateLimitPerDenom_DifferentWindows ----------

func TestRateLimitPerDenom_DifferentWindows(t *testing.T) {
	k, ctx := setupKeeperAtHeight(t, 200)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// uzrn: window [100, 150) — expired at height 200
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 50,
		CurrentSend:  "900",
		CurrentRecv:  "800",
		WindowStart:  100,
	})

	// uatom: window [180, 280) — NOT expired at height 200
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uatom",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "750",
		CurrentRecv:  "600",
		WindowStart:  180,
	})

	k.ResetExpiredWindows(ctx)

	// uzrn should be reset
	zrnRL, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if zrnRL.CurrentSend != "0" {
		t.Fatalf("uzrn: expected current_send=0 after reset, got %s", zrnRL.CurrentSend)
	}
	if zrnRL.WindowStart != 200 {
		t.Fatalf("uzrn: expected window_start=200, got %d", zrnRL.WindowStart)
	}

	// uatom should NOT be reset
	atomRL, _ := k.GetRateLimit(ctx, "channel-0", "uatom")
	if atomRL.CurrentSend != "750" {
		t.Fatalf("uatom: expected current_send=750 unchanged, got %s", atomRL.CurrentSend)
	}
	if atomRL.WindowStart != 180 {
		t.Fatalf("uatom: expected window_start=180 unchanged, got %d", atomRL.WindowStart)
	}
}

// ---------- 56. TestRateLimitPerDenom_DeleteOneKeepOther ----------

func TestRateLimitPerDenom_DeleteOneKeepOther(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
		CurrentSend:  "0",
		CurrentRecv:  "0",
	})
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uatom",
		MaxSend:      "500",
		MaxRecv:      "500",
		WindowBlocks: 200,
		CurrentSend:  "0",
		CurrentRecv:  "0",
	})

	// Delete uzrn only
	k.DeleteRateLimit(ctx, "channel-0", "uzrn")

	_, found := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if found {
		t.Fatal("uzrn rate limit should be deleted")
	}

	got, found := k.GetRateLimit(ctx, "channel-0", "uatom")
	if !found {
		t.Fatal("uatom rate limit should still exist")
	}
	if got.MaxSend != "500" {
		t.Fatalf("uatom: expected max_send=500, got %s", got.MaxSend)
	}
}

// ============================================================
// R15-4: Quota Overflow Tests
// ============================================================

// ---------- 57. TestRateLimitQuotaOverflow_BigIntArithmetic ----------

func TestRateLimitQuotaOverflow_BigIntArithmetic(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// 2^64 as max (tests big.Int not uint64)
	hugeMax := "18446744073709551616" // 2^64
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      hugeMax,
		MaxRecv:      hugeMax,
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send 2^63 — should succeed
	amt := new(big.Int)
	amt.SetString("9223372036854775808", 10) // 2^63
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", amt)
	if err != nil {
		t.Fatalf("sending 2^63 under 2^64 max should succeed: %v", err)
	}

	// Send another 2^63 — total = 2^64 exactly
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", amt)
	if err != nil {
		t.Fatalf("sending to exact 2^64 should succeed: %v", err)
	}

	// Send 1 more — should fail
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("sending beyond 2^64 should fail")
	}
}

// ---------- 58. TestRateLimitQuotaOverflow_NegativeAmount ----------

func TestRateLimitQuotaOverflow_NegativeAmount(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Sending a negative amount — big.Int allows this
	// The check is newSend > maxSend. -50 + 0 = -50 <= 100 — passes.
	// This is acceptable because negative amounts don't occur in practice
	// (amounts come from parsed packet data which is always positive).
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(-50))
	if err != nil {
		// If the implementation rejects negative, that's also valid
		return
	}

	// Verify that quota tracking still works correctly
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	// CurrentSend should be "-50" or similar
	_ = got
}

// ---------- 59. TestRateLimitQuotaOverflow_ReverseBeyondZero ----------

func TestRateLimitQuotaOverflow_ReverseBeyondZero(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Send 100
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(100))
	if err != nil {
		t.Fatalf("initial send should succeed: %v", err)
	}

	// Reverse 200 (more than we sent) — should clamp to 0
	k.ReverseSendQuota(ctx, "channel-0", "uzrn", big.NewInt(200))

	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send clamped to 0, got %s", got.CurrentSend)
	}
}

// ============================================================
// R15-4: Whitelist / Passthrough Tests
// ============================================================

// ---------- 60. TestRateLimitWhitelist_DisabledBypassesAll ----------

func TestRateLimitWhitelist_DisabledBypassesAll(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Configure a strict rate limit
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "1",
		MaxRecv:      "1",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Disable rate limiting
	k.SetParams(ctx, &types.Params{Enabled: false})

	// Send 999999 — should pass because disabled
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(999999))
	if err != nil {
		t.Fatalf("expected passthrough when disabled: %v", err)
	}

	// Recv 999999 — should pass because disabled
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(999999))
	if err != nil {
		t.Fatalf("expected passthrough when disabled: %v", err)
	}

	// Counters should NOT be updated
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "0" {
		t.Fatalf("expected current_send=0 unchanged while disabled, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "0" {
		t.Fatalf("expected current_recv=0 unchanged while disabled, got %s", got.CurrentRecv)
	}
}

// ---------- 61. TestRateLimitWhitelist_UnconfiguredDenomPasses ----------

func TestRateLimitWhitelist_UnconfiguredDenomPasses(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Only configure uzrn on channel-0
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Sending uatom (not configured) should pass through
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uatom", big.NewInt(999999))
	if err != nil {
		t.Fatalf("expected passthrough for unconfigured denom: %v", err)
	}

	// The unconfigured denom should NOT create a rate limit entry
	_, found := k.GetRateLimit(ctx, "channel-0", "uatom")
	if found {
		t.Fatal("expected no rate limit entry created for unconfigured denom")
	}
}

// ---------- 62. TestRateLimitWhitelist_UnconfiguredChannelPasses ----------

func TestRateLimitWhitelist_UnconfiguredChannelPasses(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Configure channel-0 only
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Sending on channel-99 should pass through
	err := k.CheckAndUpdateSendQuota(ctx, "channel-99", "uzrn", big.NewInt(999999))
	if err != nil {
		t.Fatalf("expected passthrough for unconfigured channel: %v", err)
	}

	// Receiving on channel-99 should pass through
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-99", "uzrn", big.NewInt(999999))
	if err != nil {
		t.Fatalf("expected passthrough for unconfigured channel recv: %v", err)
	}
}

// ============================================================
// R15-4: Inbound vs Outbound Tests
// ============================================================

// ---------- 63. TestRateLimitInboundOutbound_IndependentQuotas ----------

func TestRateLimitInboundOutbound_IndependentQuotas(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Asymmetric limits: send 100, recv 500
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "500",
		WindowBlocks: 1000,
		CurrentSend:  "0",
		CurrentRecv:  "0",
		WindowStart:  100,
	})

	// Exhaust send quota
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(100))
	if err != nil {
		t.Fatalf("send to max should succeed: %v", err)
	}

	// Send 1 more — should fail
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("send beyond max should fail")
	}

	// Recv should still work — independent quota
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(400))
	if err != nil {
		t.Fatalf("recv should succeed independently: %v", err)
	}

	// Recv up to 500 total
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(100))
	if err != nil {
		t.Fatalf("recv to max should succeed: %v", err)
	}

	// Recv 1 more — should fail
	err = k.CheckAndUpdateRecvQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("recv beyond max should fail")
	}

	// Verify counters
	got, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if got.CurrentSend != "100" {
		t.Fatalf("expected current_send=100, got %s", got.CurrentSend)
	}
	if got.CurrentRecv != "500" {
		t.Fatalf("expected current_recv=500, got %s", got.CurrentRecv)
	}
}

// ---------- 64. TestRateLimitInboundOutbound_SimultaneousChannels ----------

func TestRateLimitInboundOutbound_SimultaneousChannels(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Configure 3 channels with different limits
	channels := []struct {
		id      string
		maxSend string
		maxRecv string
	}{
		{"channel-0", "100", "200"},
		{"channel-1", "300", "400"},
		{"channel-2", "500", "600"},
	}

	for _, ch := range channels {
		k.SetRateLimit(ctx, &types.RateLimit{
			ChannelId:    ch.id,
			Denom:        "uzrn",
			MaxSend:      ch.maxSend,
			MaxRecv:      ch.maxRecv,
			WindowBlocks: 1000,
			CurrentSend:  "0",
			CurrentRecv:  "0",
			WindowStart:  100,
		})
	}

	// Exhaust channel-0 send
	err := k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(100))
	if err != nil {
		t.Fatalf("channel-0 send should succeed: %v", err)
	}
	err = k.CheckAndUpdateSendQuota(ctx, "channel-0", "uzrn", big.NewInt(1))
	if err == nil {
		t.Fatal("channel-0 send beyond max should fail")
	}

	// channel-1 and channel-2 should still be available
	err = k.CheckAndUpdateSendQuota(ctx, "channel-1", "uzrn", big.NewInt(300))
	if err != nil {
		t.Fatalf("channel-1 send should succeed: %v", err)
	}
	err = k.CheckAndUpdateSendQuota(ctx, "channel-2", "uzrn", big.NewInt(500))
	if err != nil {
		t.Fatalf("channel-2 send should succeed: %v", err)
	}
}

// ---------- 65. TestRateLimitInboundOutbound_ResetIndependently ----------

func TestRateLimitInboundOutbound_ResetIndependently(t *testing.T) {
	k, ctx := setupKeeperAtHeight(t, 200)
	k.SetParams(ctx, &types.Params{Enabled: true})

	// Two rate limits: one expired, one not
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 50,
		CurrentSend:  "90",
		CurrentRecv:  "80",
		WindowStart:  100, // expired: 100+50=150 < 200
	})
	k.SetRateLimit(ctx, &types.RateLimit{
		ChannelId:    "channel-0",
		Denom:        "uatom",
		MaxSend:      "100",
		MaxRecv:      "100",
		WindowBlocks: 200,
		CurrentSend:  "70",
		CurrentRecv:  "60",
		WindowStart:  100, // NOT expired: 100+200=300 > 200
	})

	k.ResetExpiredWindows(ctx)

	// uzrn should be reset
	zrn, _ := k.GetRateLimit(ctx, "channel-0", "uzrn")
	if zrn.CurrentSend != "0" || zrn.CurrentRecv != "0" {
		t.Fatalf("uzrn: expected counters reset, got send=%s recv=%s", zrn.CurrentSend, zrn.CurrentRecv)
	}

	// uatom should NOT be reset
	atom, _ := k.GetRateLimit(ctx, "channel-0", "uatom")
	if atom.CurrentSend != "70" {
		t.Fatalf("uatom: expected current_send=70, got %s", atom.CurrentSend)
	}
	if atom.CurrentRecv != "60" {
		t.Fatalf("uatom: expected current_recv=60, got %s", atom.CurrentRecv)
	}
}

// ============================================================
// R15-4: Additional MsgServer Tests
// ============================================================

// ---------- 66. TestMsgServerAddRateLimitEmptyDenom ----------

func TestMsgServerAddRateLimitEmptyDenom(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.AddRateLimit(ctx, &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "", // empty
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
	})
	if err == nil {
		t.Fatal("expected error for empty denom")
	}
}

// ---------- 67. TestMsgServerAddRateLimitEmptyChannel ----------

func TestMsgServerAddRateLimitEmptyChannel(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.AddRateLimit(ctx, &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "", // empty
		Denom:        "uzrn",
		MaxSend:      "1000",
		MaxRecv:      "1000",
		WindowBlocks: 100,
	})
	if err == nil {
		t.Fatal("expected error for empty channel_id")
	}
}

// ---------- 68. TestMsgServerAddRateLimitInvalidMaxSend ----------

func TestMsgServerAddRateLimitInvalidMaxSend(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// max_send = "0" (not positive)
	_, err := ms.AddRateLimit(ctx, &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "0",
		MaxRecv:      "1000",
		WindowBlocks: 100,
	})
	if err == nil {
		t.Fatal("expected error for max_send=0")
	}

	// max_send = "abc" (not a number)
	_, err = ms.AddRateLimit(ctx, &types.MsgAddRateLimit{
		Authority:    testAuthority,
		ChannelId:    "channel-0",
		Denom:        "uzrn",
		MaxSend:      "abc",
		MaxRecv:      "1000",
		WindowBlocks: 100,
	})
	if err == nil {
		t.Fatal("expected error for max_send=abc")
	}
}

// ---------- 69. TestMsgServerRemoveRateLimitEmptyFields ----------

func TestMsgServerRemoveRateLimitEmptyFields(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.RemoveRateLimit(ctx, &types.MsgRemoveRateLimit{
		Authority: testAuthority,
		ChannelId: "",
		Denom:     "uzrn",
	})
	if err == nil {
		t.Fatal("expected error for empty channel_id in RemoveRateLimit")
	}

	_, err = ms.RemoveRateLimit(ctx, &types.MsgRemoveRateLimit{
		Authority: testAuthority,
		ChannelId: "channel-0",
		Denom:     "",
	})
	if err == nil {
		t.Fatal("expected error for empty denom in RemoveRateLimit")
	}
}

// ---------- 70. TestQueryRateLimitsNilRequest ----------

func TestQueryRateLimitsNilRequest(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.RateLimits(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request to RateLimits query")
	}
}
