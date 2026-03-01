package keeper_test

import (
	"context"
	"crypto/ed25519"
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

	"github.com/zerone-chain/zerone/x/channels/keeper"
	"github.com/zerone-chain/zerone/x/channels/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

const testChainID = "zerone-test-1"

// ---------- Ed25519 Test Key Infrastructure ----------

type testParty struct {
	Name    string
	PubKey  ed25519.PublicKey
	PrivKey ed25519.PrivateKey
	Addr    string
}

func newTestParty(name string) testParty {
	seed := sha256.Sum256([]byte("test_seed:" + name))
	privKey := ed25519.NewKeyFromSeed(seed[:])
	pubKey := privKey.Public().(ed25519.PublicKey)
	addrHash := sha256.Sum256(pubKey)
	addr := sdk.AccAddress(addrHash[:20]).String()
	return testParty{Name: name, PubKey: pubKey, PrivKey: privKey, Addr: addr}
}

func signPacked(party testParty, operation, channelId string, nonce uint64, spent string) []byte {
	payload := types.ChannelSigningPayload(operation, testChainID, channelId, nonce, spent)
	sig := ed25519.Sign(party.PrivKey, payload)
	return types.PackSignature(party.PubKey, sig)
}

// ---------- Mock BankKeeper ----------

type mockBankKeeper struct {
	balances       map[string]map[string]int64
	moduleBalances map[string]map[string]int64
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

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
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

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	for _, coin := range amt {
		to := recipientAddr.String()
		if m.moduleBalances[senderModule] == nil {
			m.moduleBalances[senderModule] = make(map[string]int64)
		}
		if m.balances[to] == nil {
			m.balances[to] = make(map[string]int64)
		}
		m.moduleBalances[senderModule][coin.Denom] -= coin.Amount.Int64()
		m.balances[to][coin.Denom] += coin.Amount.Int64()
	}
	return nil
}

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

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, ChainID: testChainID}, false, log.NewNopLogger())

	return k, ctx, mockBK
}

func setupMsgServer(t *testing.T) (types.MsgServer, keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeper(t)
	return keeper.NewMsgServerImpl(k), k, ctx, bk
}

// ---------- Params Tests ----------

func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()
	if params.MinDeposit != "1000000" {
		t.Errorf("expected min deposit 1000000, got %s", params.MinDeposit)
	}
	if params.DisputeWindowBlocks != 500 {
		t.Errorf("expected dispute window 500, got %d", params.DisputeWindowBlocks)
	}
	if params.MaxChannelsPerPair != 10 {
		t.Errorf("expected max channels per pair 10, got %d", params.MaxChannelsPerPair)
	}
}

func TestSetGetParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := &types.Params{
		MinDeposit:           "5000000",
		MinTimeoutBlocks:     200,
		MaxTimeoutBlocks:     500000,
		DisputeWindowBlocks:  1000,
		DefaultSettlementFreq: 50,
		MaxChannelsPerPair:   5,
		ChannelOpenFee:       "200000",
	}
	k.SetParams(ctx, params)

	got := k.GetParams(ctx)
	if got.MinDeposit != "5000000" {
		t.Errorf("expected min deposit 5000000, got %s", got.MinDeposit)
	}
	if got.MaxChannelsPerPair != 5 {
		t.Errorf("expected max channels per pair 5, got %d", got.MaxChannelsPerPair)
	}
}

// ---------- Channel CRUD Tests ----------

func TestSetGetChannel(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	ch := &types.PaymentChannel{
		ChannelId: "pc-100-1",
		Payer:     "zrn1payer",
		Receiver:  "zrn1receiver",
		Deposited: "1000000",
		Spent:     "0",
		Available: "1000000",
		Status:    types.ChannelStatusOpen,
	}

	k.SetChannel(ctx, ch)

	got, found := k.GetChannel(ctx, "pc-100-1")
	if !found {
		t.Fatal("channel not found")
	}
	if got.Payer != "zrn1payer" {
		t.Errorf("expected payer zrn1payer, got %s", got.Payer)
	}
	if got.Deposited != "1000000" {
		t.Errorf("expected deposited 1000000, got %s", got.Deposited)
	}
}

func TestGetChannelNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	_, found := k.GetChannel(ctx, "nonexistent")
	if found {
		t.Fatal("expected channel not found")
	}
}

func TestGetChannelsByPayer(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	for i := 0; i < 3; i++ {
		ch := &types.PaymentChannel{
			ChannelId: "pc-100-" + string(rune('1'+i)),
			Payer:     "zrn1payer",
			Receiver:  "zrn1receiver",
			Deposited: "1000000",
			Status:    types.ChannelStatusOpen,
		}
		k.SetChannel(ctx, ch)
	}

	channels := k.GetChannelsByPayer(ctx, "zrn1payer")
	if len(channels) != 3 {
		t.Errorf("expected 3 channels, got %d", len(channels))
	}
}

func TestGetChannelsByReceiver(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	for i := 0; i < 2; i++ {
		ch := &types.PaymentChannel{
			ChannelId: "pc-100-" + string(rune('1'+i)),
			Payer:     "zrn1payer",
			Receiver:  "zrn1receiver",
			Deposited: "1000000",
			Status:    types.ChannelStatusOpen,
		}
		k.SetChannel(ctx, ch)
	}

	channels := k.GetChannelsByReceiver(ctx, "zrn1receiver")
	if len(channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(channels))
	}
}

func TestDeleteChannel(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	ch := &types.PaymentChannel{
		ChannelId: "pc-100-1",
		Payer:     "zrn1payer",
		Receiver:  "zrn1receiver",
		Deposited: "1000000",
		Status:    types.ChannelStatusOpen,
	}
	k.SetChannel(ctx, ch)

	// Verify it exists
	_, found := k.GetChannel(ctx, "pc-100-1")
	if !found {
		t.Fatal("channel should exist before delete")
	}

	k.DeleteChannel(ctx, ch)

	// Verify it's gone
	_, found = k.GetChannel(ctx, "pc-100-1")
	if found {
		t.Fatal("channel should not exist after delete")
	}

	// Verify indexes are cleaned up
	channels := k.GetChannelsByPayer(ctx, "zrn1payer")
	if len(channels) != 0 {
		t.Errorf("expected 0 channels after delete, got %d", len(channels))
	}
}

func TestIterateChannels(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	for i := 0; i < 5; i++ {
		ch := &types.PaymentChannel{
			ChannelId: "pc-100-" + string(rune('a'+i)),
			Payer:     "zrn1payer",
			Receiver:  "zrn1receiver",
			Deposited: "1000000",
			Status:    types.ChannelStatusOpen,
		}
		k.SetChannel(ctx, ch)
	}

	var count int
	k.IterateChannels(ctx, func(ch *types.PaymentChannel) bool {
		count++
		return false
	})
	if count != 5 {
		t.Errorf("expected 5 channels during iteration, got %d", count)
	}
}

// ---------- Dispute CRUD Tests ----------

func TestSetGetDispute(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	d := &types.ChannelDispute{
		ChannelId:     "pc-100-1",
		Disputer:      "zrn1payer",
		DisputedNonce: 5,
		DisputedSpent: "500000",
		DeadlineBlock: 600,
	}
	k.SetDispute(ctx, d)

	got, found := k.GetDispute(ctx, "pc-100-1")
	if !found {
		t.Fatal("dispute not found")
	}
	if got.DisputedNonce != 5 {
		t.Errorf("expected nonce 5, got %d", got.DisputedNonce)
	}
}

func TestDeleteDispute(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	d := &types.ChannelDispute{ChannelId: "pc-100-1", Disputer: "zrn1payer"}
	k.SetDispute(ctx, d)

	k.DeleteDispute(ctx, "pc-100-1")

	_, found := k.GetDispute(ctx, "pc-100-1")
	if found {
		t.Fatal("dispute should not exist after delete")
	}
}

// ---------- Channel Counter Tests ----------

func TestGetNextChannelId(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	id1 := k.GetNextChannelId(ctx)
	id2 := k.GetNextChannelId(ctx)
	id3 := k.GetNextChannelId(ctx)

	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Errorf("expected sequential IDs 1,2,3 - got %d,%d,%d", id1, id2, id3)
	}
}

// ---------- Genesis Tests ----------

func TestInitExportGenesis(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	genState := &types.GenesisState{
		Params: &types.Params{
			MinDeposit:           "2000000",
			MinTimeoutBlocks:     50,
			MaxTimeoutBlocks:     100000,
			DisputeWindowBlocks:  200,
			DefaultSettlementFreq: 50,
			MaxChannelsPerPair:   20,
			ChannelOpenFee:       "50000",
		},
		Channels: []*types.PaymentChannel{
			{
				ChannelId: "pc-1-1",
				Payer:     "zrn1payer",
				Receiver:  "zrn1receiver",
				Deposited: "5000000",
				Spent:     "1000000",
				Available: "4000000",
				Status:    types.ChannelStatusOpen,
			},
		},
	}

	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)
	if exported.Params.MinDeposit != "2000000" {
		t.Errorf("expected min deposit 2000000, got %s", exported.Params.MinDeposit)
	}
	if len(exported.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(exported.Channels))
	}
	if exported.Channels[0].ChannelId != "pc-1-1" {
		t.Errorf("expected channel pc-1-1, got %s", exported.Channels[0].ChannelId)
	}
}

// ---------- OpenChannel Tests ----------

func TestOpenChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 10_000_000)

	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("OpenChannel failed: %v", err)
	}
	if resp.ChannelId == "" {
		t.Fatal("expected non-empty channel ID")
	}

	ch, found := k.GetChannel(ctx, resp.ChannelId)
	if !found {
		t.Fatal("channel not found after opening")
	}
	if ch.Payer != payer.Addr {
		t.Errorf("expected payer %s, got %s", payer.Addr, ch.Payer)
	}
	if ch.Receiver != receiver.Addr {
		t.Errorf("expected receiver %s, got %s", receiver.Addr, ch.Receiver)
	}
	if ch.Deposited != "5000000" {
		t.Errorf("expected deposited 5000000, got %s", ch.Deposited)
	}
	if ch.Status != types.ChannelStatusOpen {
		t.Errorf("expected status open, got %s", ch.Status)
	}
	if ch.Available != "5000000" {
		t.Errorf("expected available 5000000, got %s", ch.Available)
	}
	if ch.Nonce != 0 {
		t.Errorf("expected nonce 0, got %d", ch.Nonce)
	}
}

func TestOpenChannelInsufficientDeposit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 10_000_000)

	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "100", // Below minimum of 1000000
		TimeoutBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for insufficient deposit")
	}
}

func TestOpenChannelSameAddress(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	bk.setBalance(payer.Addr, "uzrn", 10_000_000)

	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      payer.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for same payer and receiver")
	}
}

func TestOpenChannelTimeoutExceedsMax(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 10_000_000)

	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 2000000, // Exceeds default max of 1000000
	})
	if err == nil {
		t.Fatal("expected error for timeout exceeding max")
	}
}

func TestOpenChannelMaxPerPair(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 1_000_000_000)

	// Open max channels (default 10)
	for i := 0; i < 10; i++ {
		_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
			Payer:         payer.Addr,
			Receiver:      receiver.Addr,
			Deposit:       "1000000",
			TimeoutBlocks: 1000,
		})
		if err != nil {
			t.Fatalf("OpenChannel %d failed: %v", i, err)
		}
	}

	// 11th should fail
	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "1000000",
		TimeoutBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for exceeding max channels per pair")
	}
}

// ---------- DepositChannel Tests ----------

func TestDepositChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	_, err := msgSrv.DepositChannel(ctx, &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: resp.ChannelId,
		Amount:    "3000000",
	})
	if err != nil {
		t.Fatalf("DepositChannel failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Deposited != "8000000" {
		t.Errorf("expected deposited 8000000, got %s", ch.Deposited)
	}
	if ch.Available != "8000000" {
		t.Errorf("expected available 8000000, got %s", ch.Available)
	}
}

func TestDepositChannelNotPayer(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)
	bk.setBalance(receiver.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	_, err := msgSrv.DepositChannel(ctx, &types.MsgDepositChannel{
		Depositor: receiver.Addr,
		ChannelId: resp.ChannelId,
		Amount:    "3000000",
	})
	if err == nil {
		t.Fatal("expected error for non-payer depositing")
	}
}

// ---------- UpdateState Tests ----------

func TestUpdateState(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "1000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "1000000"))

	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			ChannelId:          resp.ChannelId,
			Nonce:              1,
			Spent:              "1000000",
			StateHash:          "abc123",
			PayerSignature:     payerSig,
			ReceiverSignature:  receiverSig,
		},
	})
	if err != nil {
		t.Fatalf("UpdateState failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Nonce != 1 {
		t.Errorf("expected nonce 1, got %d", ch.Nonce)
	}
	if ch.Spent != "1000000" {
		t.Errorf("expected spent 1000000, got %s", ch.Spent)
	}
	if ch.Available != "4000000" {
		t.Errorf("expected available 4000000, got %s", ch.Available)
	}
}

func TestUpdateStateNonceRollback(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// First update with nonce=5
	payerSig5 := string(signPacked(payer, "state", resp.ChannelId, 5, "1000000"))
	receiverSig5 := string(signPacked(receiver, "state", resp.ChannelId, 5, "1000000"))
	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             5,
			Spent:             "1000000",
			PayerSignature:    payerSig5,
			ReceiverSignature: receiverSig5,
		},
	})
	if err != nil {
		t.Fatalf("first UpdateState failed: %v", err)
	}

	// Try nonce=3 (rollback attack) — should fail
	payerSig3 := string(signPacked(payer, "state", resp.ChannelId, 3, "500000"))
	receiverSig3 := string(signPacked(receiver, "state", resp.ChannelId, 3, "500000"))
	_, err = msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             3,
			Spent:             "500000",
			PayerSignature:    payerSig3,
			ReceiverSignature: receiverSig3,
		},
	})
	if err == nil {
		t.Fatal("expected error for nonce rollback")
	}
}

func TestUpdateStateSpentExceedsDeposit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Try spending more than deposited
	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "9999999"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "9999999"))
	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             1,
			Spent:             "9999999",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err == nil {
		t.Fatal("expected error for spent exceeding deposit")
	}
}

func TestUpdateStateNotParty(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	thirdParty := newTestParty("thirdparty")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "1000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "1000000"))
	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    thirdParty.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             1,
			Spent:             "1000000",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err == nil {
		t.Fatal("expected error for non-party sender")
	}
}

// ---------- CloseChannel Tests ----------

func TestCloseChannelCooperative(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Cooperative close: closer is payer, needs receiver's signature
	counterpartySig := signPacked(receiver, "close", resp.ChannelId, 10, "3000000")

	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "3000000",
		FinalNonce:            10,
		CounterpartySignature: counterpartySig,
	})
	if err != nil {
		t.Fatalf("CloseChannel failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected status settled, got %s", ch.Status)
	}
	if ch.Spent != "3000000" {
		t.Errorf("expected spent 3000000, got %s", ch.Spent)
	}

	// Verify fund distribution
	if bk.balances[receiver.Addr]["uzrn"] != 3000000 {
		t.Errorf("expected receiver balance 3000000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
}

func TestCloseChannelNotParty(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	thirdParty := newTestParty("thirdparty")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:    thirdParty.Addr,
		ChannelId: resp.ChannelId,
		FinalSpent: "0",
		FinalNonce: 0,
	})
	if err == nil {
		t.Fatal("expected error for non-party closer")
	}
}

// ---------- DisputeChannel Tests ----------

func TestDisputeChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Payer disputes with receiver's signature
	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 5, "2000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "2000000",
		ClaimedNonce:   5,
		ProofSignature: proofSig,
	})
	if err != nil {
		t.Fatalf("DisputeChannel failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusDisputed {
		t.Errorf("expected status disputed, got %s", ch.Status)
	}
	if ch.DisputeDeadline != uint64(ctx.BlockHeight())+500 {
		t.Errorf("expected deadline %d, got %d", uint64(ctx.BlockHeight())+500, ch.DisputeDeadline)
	}

	dispute, found := k.GetDispute(ctx, resp.ChannelId)
	if !found {
		t.Fatal("dispute not found")
	}
	if dispute.DisputedNonce != 5 {
		t.Errorf("expected disputed nonce 5, got %d", dispute.DisputedNonce)
	}
	if dispute.DisputedSpent != "2000000" {
		t.Errorf("expected disputed spent 2000000, got %s", dispute.DisputedSpent)
	}
}

func TestDisputeHigherNonceWins(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// First dispute with nonce=3
	proofSig3 := signPacked(receiver, "dispute", resp.ChannelId, 3, "1000000")
	_, _ = msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "1000000",
		ClaimedNonce:   3,
		ProofSignature: proofSig3,
	})

	// Counter-dispute with higher nonce=7
	proofSig7 := signPacked(payer, "dispute", resp.ChannelId, 7, "3000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       receiver.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "3000000",
		ClaimedNonce:   7,
		ProofSignature: proofSig7,
	})
	if err != nil {
		t.Fatalf("counter-dispute failed: %v", err)
	}

	dispute, _ := k.GetDispute(ctx, resp.ChannelId)
	if dispute.DisputedNonce != 7 {
		t.Errorf("expected dispute nonce 7 (higher wins), got %d", dispute.DisputedNonce)
	}
	if dispute.DisputedSpent != "3000000" {
		t.Errorf("expected disputed spent 3000000, got %s", dispute.DisputedSpent)
	}
}

func TestDisputeLowerNonceRejected(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Dispute with nonce=5
	proofSig5 := signPacked(receiver, "dispute", resp.ChannelId, 5, "2000000")
	_, _ = msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "2000000",
		ClaimedNonce:   5,
		ProofSignature: proofSig5,
	})

	// Counter-dispute with lower nonce=3 should fail
	proofSig3 := signPacked(payer, "dispute", resp.ChannelId, 3, "1000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       receiver.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "1000000",
		ClaimedNonce:   3,
		ProofSignature: proofSig3,
	})
	if err == nil {
		t.Fatal("expected error for dispute with lower nonce")
	}
}

// ---------- ClaimExpired Tests ----------

func TestClaimExpired(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 200,
	})

	// Move past expiration (block 100 + 200 = 300, need > 300)
	expiredCtx := ctx.WithBlockHeight(301)

	claimResp, err := msgSrv.ClaimExpired(expiredCtx, &types.MsgClaimExpired{
		Claimer:   payer.Addr,
		ChannelId: resp.ChannelId,
	})
	if err != nil {
		t.Fatalf("ClaimExpired failed: %v", err)
	}
	if claimResp.RefundedAmount != "5000000" {
		t.Errorf("expected refunded 5000000, got %s", claimResp.RefundedAmount)
	}

	ch, _ := k.GetChannel(expiredCtx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected status settled, got %s", ch.Status)
	}
}

func TestClaimExpiredNotExpired(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Try claiming before expiration
	_, err := msgSrv.ClaimExpired(ctx, &types.MsgClaimExpired{
		Claimer:   payer.Addr,
		ChannelId: resp.ChannelId,
	})
	if err == nil {
		t.Fatal("expected error for claiming before expiration")
	}
}

func TestClaimExpiredNotPayer(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 200,
	})

	expiredCtx := ctx.WithBlockHeight(301)

	_, err := msgSrv.ClaimExpired(expiredCtx, &types.MsgClaimExpired{
		Claimer:   receiver.Addr,
		ChannelId: resp.ChannelId,
	})
	if err == nil {
		t.Fatal("expected error for non-payer claiming expired")
	}
}

// ---------- AutoSettle Tests ----------

func TestAutoSettleDisputedChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 10000,
	})

	// Open dispute
	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 5, "2000000")
	_, _ = msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "2000000",
		ClaimedNonce:   5,
		ProofSignature: proofSig,
	})

	// Move past dispute deadline
	pastDeadline := uint64(ctx.BlockHeight()) + 501

	expired := k.GetExpiredChannels(ctx.WithBlockHeight(int64(pastDeadline)), pastDeadline)
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired channel, got %d", len(expired))
	}

	// Auto-settle
	k.AutoSettleChannel(ctx.WithBlockHeight(int64(pastDeadline)), expired[0])

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected status settled, got %s", ch.Status)
	}

	// Verify dispute was resolved
	dispute, _ := k.GetDispute(ctx, resp.ChannelId)
	if !dispute.Resolved {
		t.Error("expected dispute to be resolved")
	}
	if dispute.Resolution != "auto_settled" {
		t.Errorf("expected resolution auto_settled, got %s", dispute.Resolution)
	}
}

// ---------- UpdateParams Tests ----------

func TestUpdateParams(t *testing.T) {
	msgSrv, k, ctx, _ := setupMsgServer(t)

	newParams := &types.Params{
		MinDeposit:           "2000000",
		MinTimeoutBlocks:     50,
		MaxTimeoutBlocks:     500000,
		DisputeWindowBlocks:  1000,
		DefaultSettlementFreq: 200,
		MaxChannelsPerPair:   20,
		ChannelOpenFee:       "500000",
	}

	_, err := msgSrv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "zrn1authority",
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MinDeposit != "2000000" {
		t.Errorf("expected min deposit 2000000, got %s", got.MinDeposit)
	}
	if got.MaxChannelsPerPair != 20 {
		t.Errorf("expected max channels 20, got %d", got.MaxChannelsPerPair)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	msgSrv, _, ctx, _ := setupMsgServer(t)

	_, err := msgSrv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "zrn1unauthorized",
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected error for unauthorized params update")
	}
}

// ---------- Full Lifecycle Test ----------

func TestFullLifecycle_OpenDepositUpdateClose(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	// 1. Open channel
	openResp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("OpenChannel: %v", err)
	}

	// 2. Deposit more
	_, err = msgSrv.DepositChannel(ctx, &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: openResp.ChannelId,
		Amount:    "3000000",
	})
	if err != nil {
		t.Fatalf("DepositChannel: %v", err)
	}

	ch, _ := k.GetChannel(ctx, openResp.ChannelId)
	if ch.Deposited != "8000000" {
		t.Fatalf("expected deposited 8000000 after deposit, got %s", ch.Deposited)
	}

	// 3. Update state (off-chain payments)
	payerSig := string(signPacked(payer, "state", openResp.ChannelId, 1, "2000000"))
	receiverSig := string(signPacked(receiver, "state", openResp.ChannelId, 1, "2000000"))
	_, err = msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: openResp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             1,
			Spent:             "2000000",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err != nil {
		t.Fatalf("UpdateState: %v", err)
	}

	ch, _ = k.GetChannel(ctx, openResp.ChannelId)
	if ch.Spent != "2000000" || ch.Available != "6000000" {
		t.Fatalf("state update failed: spent=%s available=%s", ch.Spent, ch.Available)
	}

	// 4. Cooperative close
	counterpartySig := signPacked(receiver, "close", openResp.ChannelId, 1, "2000000")
	_, err = msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             openResp.ChannelId,
		FinalSpent:            "2000000",
		FinalNonce:            1,
		CounterpartySignature: counterpartySig,
	})
	if err != nil {
		t.Fatalf("CloseChannel: %v", err)
	}

	ch, _ = k.GetChannel(ctx, openResp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected settled, got %s", ch.Status)
	}

	// Verify fund distribution
	// Receiver should have gotten 2000000
	if bk.balances[receiver.Addr]["uzrn"] != 2000000 {
		t.Errorf("expected receiver balance 2000000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
}

// ---------- DepositChannel to Closed Channel ----------

func TestDepositToClosedChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 200,
	})

	// Close it
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusSettled
	k.SetChannel(ctx, ch)

	_, err := msgSrv.DepositChannel(ctx, &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: resp.ChannelId,
		Amount:    "1000000",
	})
	if err == nil {
		t.Fatal("expected error for depositing to closed channel")
	}
}

// ---------- Signature Verification Tests ----------

func TestSignatureVerification(t *testing.T) {
	party := newTestParty("alice")

	payload := types.ChannelSigningPayload("state", testChainID, "pc-100-1", 5, "3000000")
	sig := ed25519.Sign(party.PrivKey, payload)
	packed := types.PackSignature(party.PubKey, sig)

	err := types.VerifyPackedSignature(payload, packed, party.Addr)
	if err != nil {
		t.Fatalf("signature verification should succeed: %v", err)
	}
}

func TestSignatureVerificationWrongAddress(t *testing.T) {
	alice := newTestParty("alice")
	bob := newTestParty("bob")

	payload := types.ChannelSigningPayload("state", testChainID, "pc-100-1", 5, "3000000")
	sig := ed25519.Sign(alice.PrivKey, payload)
	packed := types.PackSignature(alice.PubKey, sig)

	// Verify against bob's address — should fail
	err := types.VerifyPackedSignature(payload, packed, bob.Addr)
	if err == nil {
		t.Fatal("expected error for wrong address")
	}
}

func TestSignatureVerificationTamperedPayload(t *testing.T) {
	party := newTestParty("alice")

	payload := types.ChannelSigningPayload("state", testChainID, "pc-100-1", 5, "3000000")
	sig := ed25519.Sign(party.PrivKey, payload)
	packed := types.PackSignature(party.PubKey, sig)

	// Tamper with payload
	tamperedPayload := types.ChannelSigningPayload("state", testChainID, "pc-100-1", 5, "9999999")
	err := types.VerifyPackedSignature(tamperedPayload, packed, party.Addr)
	if err == nil {
		t.Fatal("expected error for tampered payload")
	}
}

// ---------- Query Server Tests ----------

func TestQueryChannel(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	ch := &types.PaymentChannel{
		ChannelId: "pc-100-1",
		Payer:     "zrn1payer",
		Receiver:  "zrn1receiver",
		Deposited: "5000000",
		Status:    types.ChannelStatusOpen,
	}
	k.SetChannel(ctx, ch)

	resp, err := qs.Channel(ctx, &types.QueryChannelRequest{ChannelId: "pc-100-1"})
	if err != nil {
		t.Fatalf("Query Channel failed: %v", err)
	}
	if resp.Channel.Deposited != "5000000" {
		t.Errorf("expected deposited 5000000, got %s", resp.Channel.Deposited)
	}
}

func TestQueryChannelNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Channel(ctx, &types.QueryChannelRequest{ChannelId: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent channel")
	}
}

func TestQueryParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("Query Params failed: %v", err)
	}
	if resp.Params.MinDeposit != "1000000" {
		t.Errorf("expected default min deposit 1000000, got %s", resp.Params.MinDeposit)
	}
}

// ---------- ValidateBasic Tests ----------

func TestMsgOpenChannelValidateBasic(t *testing.T) {
	payer := newTestParty("payer")
	receiver := newTestParty("receiver")

	msg := &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("ValidateBasic should pass: %v", err)
	}

	// Empty deposit
	msg2 := &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "",
		TimeoutBlocks: 1000,
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Error("ValidateBasic should fail for empty deposit")
	}

	// Same payer and receiver
	msg3 := &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      payer.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	}
	if err := msg3.ValidateBasic(); err == nil {
		t.Error("ValidateBasic should fail for same payer/receiver")
	}
}

func TestGenesisValidation(t *testing.T) {
	// Valid genesis
	gs := types.DefaultGenesis()
	if err := gs.Validate(); err != nil {
		t.Errorf("default genesis should be valid: %v", err)
	}

	// Nil params
	gs2 := &types.GenesisState{Params: nil}
	if err := gs2.Validate(); err == nil {
		t.Error("genesis with nil params should be invalid")
	}
}

// ---------- Periodic Settlement Tests ----------

func TestPeriodicSettlement(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("OpenChannel failed: %v", err)
	}

	// Update state to spend some
	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "2000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "2000000"))
	_, err = msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			ChannelId:         resp.ChannelId,
			Nonce:             1,
			Spent:             "2000000",
			StateHash:         "hash1",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err != nil {
		t.Fatalf("UpdateState failed: %v", err)
	}

	// Seed module balance so transfer works
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 5000000}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	k.PeriodicSettlement(ctx, ch, 200)

	ch, _ = k.GetChannel(ctx, resp.ChannelId)
	if ch.Deposited != "3000000" {
		t.Errorf("expected deposited 3000000, got %s", ch.Deposited)
	}
	if ch.Spent != "0" {
		t.Errorf("expected spent reset to 0, got %s", ch.Spent)
	}
	if ch.Available != "3000000" {
		t.Errorf("expected available 3000000, got %s", ch.Available)
	}
	if bk.balances[receiver.Addr]["uzrn"] != 2000000 {
		t.Errorf("expected receiver balance 2000000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
}

func TestPeriodicSettlementNoSpent(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("OpenChannel failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	k.PeriodicSettlement(ctx, ch, 200)

	// Channel should be unchanged since spent is 0
	ch, _ = k.GetChannel(ctx, resp.ChannelId)
	if ch.Deposited != "5000000" {
		t.Errorf("expected deposited unchanged at 5000000, got %s", ch.Deposited)
	}
	if ch.Spent != "0" {
		t.Errorf("expected spent still 0, got %s", ch.Spent)
	}
}

func TestGetChannelsForAutoSettlement(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Set params with settlement freq 50
	params := types.DefaultParams()
	params.DefaultSettlementFreq = 50
	k.SetParams(ctx, params)

	ch := &types.PaymentChannel{
		ChannelId:           "pc-100-1",
		Payer:               "zrn1payer",
		Receiver:            "zrn1receiver",
		Deposited:           "5000000",
		Spent:               "1000000",
		Available:           "4000000",
		Status:              types.ChannelStatusOpen,
		OpenedAtBlock:       100,
		ExpiresAtBlock:      1100,
		SettlementFrequency: 50,
		LastSettlementBlock: 100,
	}
	k.SetChannel(ctx, ch)

	// At block 151 (100 + 50 = 150, need >= 150)
	channels := k.GetChannelsForAutoSettlement(ctx, 151)
	if len(channels) != 1 {
		t.Errorf("expected 1 channel due for settlement, got %d", len(channels))
	}
}

func TestGetChannelsForAutoSettlementNotDue(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	ch := &types.PaymentChannel{
		ChannelId:           "pc-100-1",
		Payer:               "zrn1payer",
		Receiver:            "zrn1receiver",
		Deposited:           "5000000",
		Spent:               "1000000",
		Available:           "4000000",
		Status:              types.ChannelStatusOpen,
		OpenedAtBlock:       100,
		ExpiresAtBlock:      1100,
		SettlementFrequency: 50,
		LastSettlementBlock: 100,
	}
	k.SetChannel(ctx, ch)

	// At block 120 (100 + 50 = 150, 120 < 150, not due)
	channels := k.GetChannelsForAutoSettlement(ctx, 120)
	if len(channels) != 0 {
		t.Errorf("expected 0 channels (not yet due), got %d", len(channels))
	}
}

// ---------- Auto-Settle Tests ----------

func TestAutoSettleOpenChannel(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")

	// Seed module balance
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 5000000}

	ch := &types.PaymentChannel{
		ChannelId:       "pc-100-1",
		Payer:           payer.Addr,
		Receiver:        receiver.Addr,
		Deposited:       "5000000",
		Spent:           "2000000",
		Available:       "3000000",
		Status:          types.ChannelStatusClosing,
		DisputeDeadline: 200,
	}
	k.SetChannel(ctx, ch)

	// Move past deadline
	settleCtx := ctx.WithBlockHeight(201)
	expired := k.GetExpiredChannels(settleCtx, 201)
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired channel, got %d", len(expired))
	}

	k.AutoSettleChannel(settleCtx, expired[0])

	ch, _ = k.GetChannel(ctx, "pc-100-1")
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected status settled, got %s", ch.Status)
	}
	if bk.balances[receiver.Addr]["uzrn"] != 2000000 {
		t.Errorf("expected receiver balance 2000000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
	if bk.balances[payer.Addr]["uzrn"] != 3000000 {
		t.Errorf("expected payer refund 3000000, got %d", bk.balances[payer.Addr]["uzrn"])
	}
}

func TestGetExpiredChannelsNone(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// No channels at all
	expired := k.GetExpiredChannels(ctx, 500)
	if len(expired) != 0 {
		t.Errorf("expected 0 expired channels, got %d", len(expired))
	}

	// Add an open channel (not closing/disputed) with no deadline
	ch := &types.PaymentChannel{
		ChannelId:  "pc-100-1",
		Payer:      "zrn1payer",
		Receiver:   "zrn1receiver",
		Deposited:  "5000000",
		Status:     types.ChannelStatusOpen,
	}
	k.SetChannel(ctx, ch)

	expired = k.GetExpiredChannels(ctx, 500)
	if len(expired) != 0 {
		t.Errorf("expected 0 expired channels for open channel, got %d", len(expired))
	}
}

// ---------- Close Channel Edge Cases ----------

func TestCloseChannelByReceiver(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("OpenChannel failed: %v", err)
	}

	// Receiver initiates close, needs payer's counterparty signature
	counterpartySig := signPacked(payer, "close", resp.ChannelId, 5, "1000000")

	_, err = msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                receiver.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "1000000",
		FinalNonce:            5,
		CounterpartySignature: counterpartySig,
	})
	if err != nil {
		t.Fatalf("CloseChannel by receiver failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected status settled, got %s", ch.Status)
	}
}

func TestCloseChannelAlreadyClosed(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Manually set channel to settled
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusSettled
	k.SetChannel(ctx, ch)

	counterpartySig := signPacked(receiver, "close", resp.ChannelId, 0, "0")
	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "0",
		FinalNonce:            0,
		CounterpartySignature: counterpartySig,
	})
	if err == nil {
		t.Fatal("expected error for closing already settled channel")
	}
}

func TestCloseChannelZeroSpent(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	counterpartySig := signPacked(receiver, "close", resp.ChannelId, 0, "0")
	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "0",
		FinalNonce:            0,
		CounterpartySignature: counterpartySig,
	})
	if err != nil {
		t.Fatalf("CloseChannel with zero spent failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected status settled, got %s", ch.Status)
	}
	if ch.Spent != "0" {
		t.Errorf("expected spent 0, got %s", ch.Spent)
	}
	// Receiver should have gotten nothing
	if bk.balances[receiver.Addr]["uzrn"] != 0 {
		t.Errorf("expected receiver balance 0, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
}

// ---------- Deposit Edge Cases ----------

func TestDepositChannelNonexistent(t *testing.T) {
	msgSrv, _, ctx, _ := setupMsgServer(t)

	payer := newTestParty("payer")
	_, err := msgSrv.DepositChannel(ctx, &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: "nonexistent-channel",
		Amount:    "1000000",
	})
	if err == nil {
		t.Fatal("expected error for depositing to nonexistent channel")
	}
}

func TestDepositAfterPartialSpend(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Update state to spend 2000000
	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "2000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "2000000"))
	_, _ = msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			ChannelId:         resp.ChannelId,
			Nonce:             1,
			Spent:             "2000000",
			StateHash:         "hash1",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})

	// Deposit more
	_, err := msgSrv.DepositChannel(ctx, &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: resp.ChannelId,
		Amount:    "3000000",
	})
	if err != nil {
		t.Fatalf("DepositChannel failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	// Deposited = 5000000 + 3000000 = 8000000
	if ch.Deposited != "8000000" {
		t.Errorf("expected deposited 8000000, got %s", ch.Deposited)
	}
	// Available = (5000000 - 2000000) + 3000000 = 6000000
	if ch.Available != "6000000" {
		t.Errorf("expected available 6000000, got %s", ch.Available)
	}
}

// ---------- State Update Edge Cases ----------

func TestUpdateStateClosedChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Manually set to settled
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusSettled
	k.SetChannel(ctx, ch)

	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "1000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "1000000"))
	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			ChannelId:         resp.ChannelId,
			Nonce:             1,
			Spent:             "1000000",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err == nil {
		t.Fatal("expected error for updating state on settled channel")
	}
}

func TestUpdateStateNilUpdate(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update:    nil,
	})
	if err == nil {
		t.Fatal("expected error for nil state update")
	}
}

func TestUpdateStateMultipleUpdates(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Three sequential updates with increasing nonce
	for i := uint64(1); i <= 3; i++ {
		spent := fmt.Sprintf("%d", i*500000)
		payerSig := string(signPacked(payer, "state", resp.ChannelId, i, spent))
		receiverSig := string(signPacked(receiver, "state", resp.ChannelId, i, spent))
		_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
			Sender:    payer.Addr,
			ChannelId: resp.ChannelId,
			Update: &types.ChannelStateUpdate{
				ChannelId:         resp.ChannelId,
				Nonce:             i,
				Spent:             spent,
				StateHash:         fmt.Sprintf("hash%d", i),
				PayerSignature:    payerSig,
				ReceiverSignature: receiverSig,
			},
		})
		if err != nil {
			t.Fatalf("UpdateState nonce=%d failed: %v", i, err)
		}
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Nonce != 3 {
		t.Errorf("expected nonce 3, got %d", ch.Nonce)
	}
	if ch.Spent != "1500000" {
		t.Errorf("expected spent 1500000, got %s", ch.Spent)
	}
	if ch.Available != "3500000" {
		t.Errorf("expected available 3500000, got %s", ch.Available)
	}
}

func TestUpdateStateReceiverSubmits(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "1500000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "1500000"))
	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    receiver.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			ChannelId:         resp.ChannelId,
			Nonce:             1,
			Spent:             "1500000",
			StateHash:         "hash1",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err != nil {
		t.Fatalf("UpdateState by receiver failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Nonce != 1 {
		t.Errorf("expected nonce 1, got %d", ch.Nonce)
	}
	if ch.Spent != "1500000" {
		t.Errorf("expected spent 1500000, got %s", ch.Spent)
	}
}

// ---------- Dispute Edge Cases ----------

func TestDisputeChannelNotParty(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	thirdParty := newTestParty("thirdparty")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 3, "1000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       thirdParty.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "1000000",
		ClaimedNonce:   3,
		ProofSignature: proofSig,
	})
	if err == nil {
		t.Fatal("expected error for third party dispute")
	}
}

func TestDisputeSettledChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Manually settle
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusSettled
	k.SetChannel(ctx, ch)

	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 3, "1000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "1000000",
		ClaimedNonce:   3,
		ProofSignature: proofSig,
	})
	if err == nil {
		t.Fatal("expected error for disputing settled channel")
	}
}

func TestDisputeSpentExceedsDeposit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 3, "9999999")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "9999999",
		ClaimedNonce:   3,
		ProofSignature: proofSig,
	})
	if err == nil {
		t.Fatal("expected error for dispute spent exceeding deposit")
	}
}

// ---------- Query Tests ----------

func TestQueryChannelsByPayer(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	qs := keeper.NewQueryServerImpl(k)

	payer := newTestParty("payer")
	receiver1 := newTestParty("receiver1")
	receiver2 := newTestParty("receiver2")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	_, _ = msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver1.Addr,
		Deposit:       "2000000",
		TimeoutBlocks: 1000,
	})
	_, _ = msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver2.Addr,
		Deposit:       "2000000",
		TimeoutBlocks: 1000,
	})

	resp, err := qs.ChannelsByPayer(ctx, &types.QueryByPayerRequest{Payer: payer.Addr})
	if err != nil {
		t.Fatalf("ChannelsByPayer failed: %v", err)
	}
	if len(resp.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(resp.Channels))
	}
}

func TestQueryChannelsByPayerEmpty(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.ChannelsByPayer(ctx, &types.QueryByPayerRequest{Payer: "zrn1nobody"})
	if err != nil {
		t.Fatalf("ChannelsByPayer failed: %v", err)
	}
	if len(resp.Channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(resp.Channels))
	}
}

func TestQueryChannelsByReceiver(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	qs := keeper.NewQueryServerImpl(k)

	payer1 := newTestParty("payer1")
	payer2 := newTestParty("payer2")
	receiver := newTestParty("receiver")
	bk.setBalance(payer1.Addr, "uzrn", 100_000_000)
	bk.setBalance(payer2.Addr, "uzrn", 100_000_000)

	_, _ = msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer1.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "2000000",
		TimeoutBlocks: 1000,
	})
	_, _ = msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer2.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "2000000",
		TimeoutBlocks: 1000,
	})

	resp, err := qs.ChannelsByReceiver(ctx, &types.QueryByReceiverRequest{Receiver: receiver.Addr})
	if err != nil {
		t.Fatalf("ChannelsByReceiver failed: %v", err)
	}
	if len(resp.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(resp.Channels))
	}
}

func TestQueryDispute(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)
	qs := keeper.NewQueryServerImpl(k)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 5, "2000000")
	_, _ = msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "2000000",
		ClaimedNonce:   5,
		ProofSignature: proofSig,
	})

	qResp, err := qs.Dispute(ctx, &types.QueryDisputeRequest{ChannelId: resp.ChannelId})
	if err != nil {
		t.Fatalf("Query Dispute failed: %v", err)
	}
	if qResp.Dispute == nil {
		t.Fatal("expected dispute to be returned")
	}
	if qResp.Dispute.DisputedNonce != 5 {
		t.Errorf("expected disputed nonce 5, got %d", qResp.Dispute.DisputedNonce)
	}
	if qResp.Dispute.DisputedSpent != "2000000" {
		t.Errorf("expected disputed spent 2000000, got %s", qResp.Dispute.DisputedSpent)
	}
}

func TestQueryDisputeNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Dispute(ctx, &types.QueryDisputeRequest{ChannelId: "nonexistent"})
	if err != nil {
		t.Fatalf("Query Dispute should not error for nonexistent, got: %v", err)
	}
	// The query server returns an empty response (no dispute) rather than an error
	if resp.Dispute != nil {
		t.Error("expected nil dispute for nonexistent channel")
	}
}

// ---------- Genesis Tests ----------

func TestInitExportGenesisWithDisputes(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	genState := &types.GenesisState{
		Params: types.DefaultParams(),
		Channels: []*types.PaymentChannel{
			{
				ChannelId: "pc-1-1",
				Payer:     "zrn1payer",
				Receiver:  "zrn1receiver",
				Deposited: "5000000",
				Spent:     "1000000",
				Available: "4000000",
				Status:    types.ChannelStatusDisputed,
			},
		},
		Disputes: []*types.ChannelDispute{
			{
				ChannelId:     "pc-1-1",
				Disputer:      "zrn1payer",
				DisputedNonce: 3,
				DisputedSpent: "1000000",
				DeadlineBlock: 600,
				Resolved:      false,
			},
		},
	}

	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)
	if len(exported.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(exported.Channels))
	}
	if len(exported.Disputes) != 1 {
		t.Fatalf("expected 1 dispute, got %d", len(exported.Disputes))
	}
	if exported.Disputes[0].DisputedNonce != 3 {
		t.Errorf("expected disputed nonce 3, got %d", exported.Disputes[0].DisputedNonce)
	}
	if exported.Disputes[0].DisputedSpent != "1000000" {
		t.Errorf("expected disputed spent 1000000, got %s", exported.Disputes[0].DisputedSpent)
	}
	if exported.Channels[0].Status != types.ChannelStatusDisputed {
		t.Errorf("expected channel status disputed, got %s", exported.Channels[0].Status)
	}
}

func TestInitGenesisEmptyState(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	genState := types.DefaultGenesis()
	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)
	if exported.Params == nil {
		t.Fatal("expected params to be set")
	}
	if exported.Params.MinDeposit != "1000000" {
		t.Errorf("expected default min deposit 1000000, got %s", exported.Params.MinDeposit)
	}
	if len(exported.Channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(exported.Channels))
	}
	if len(exported.Disputes) != 0 {
		t.Errorf("expected 0 disputes, got %d", len(exported.Disputes))
	}
}

// ---------- Open Channel Edge Cases ----------

func TestOpenChannelMinTimeout(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	// Default MinTimeoutBlocks is 100; try with 10
	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 10,
	})
	if err == nil {
		t.Fatal("expected error for timeout below minimum")
	}
}

func TestOpenChannelZeroDeposit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "0",
		TimeoutBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for zero deposit")
	}
}

func TestOpenChannelNegativeDeposit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "-100",
		TimeoutBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for negative deposit")
	}
}

// ---------- Fund Distribution Tests ----------

func TestClaimExpiredWithSpent(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 200,
	})

	// Update state to spend some before expiration
	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "2000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "2000000"))
	_, _ = msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			ChannelId:         resp.ChannelId,
			Nonce:             1,
			Spent:             "2000000",
			StateHash:         "hash1",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})

	// Seed module balance
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 5000000}

	// Move past expiration
	expiredCtx := ctx.WithBlockHeight(301)

	claimResp, err := msgSrv.ClaimExpired(expiredCtx, &types.MsgClaimExpired{
		Claimer:   payer.Addr,
		ChannelId: resp.ChannelId,
	})
	if err != nil {
		t.Fatalf("ClaimExpired failed: %v", err)
	}
	// Payer should get deposited - spent = 5000000 - 2000000 = 3000000
	if claimResp.RefundedAmount != "3000000" {
		t.Errorf("expected refunded 3000000, got %s", claimResp.RefundedAmount)
	}

	// Receiver should have gotten the spent amount
	if bk.balances[receiver.Addr]["uzrn"] != 2000000 {
		t.Errorf("expected receiver balance 2000000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}

	ch, _ := k.GetChannel(expiredCtx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected status settled, got %s", ch.Status)
	}
}

func TestCloseChannelFundDistribution(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Seed module balance for distribution
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 5000000}

	// Close with specific spent
	counterpartySig := signPacked(receiver, "close", resp.ChannelId, 10, "3500000")
	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "3500000",
		FinalNonce:            10,
		CounterpartySignature: counterpartySig,
	})
	if err != nil {
		t.Fatalf("CloseChannel failed: %v", err)
	}

	// Receiver should get 3500000
	if bk.balances[receiver.Addr]["uzrn"] != 3500000 {
		t.Errorf("expected receiver balance 3500000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}

	// Payer should get refund of 1500000 from module
	// Note: payer's initial balance was reduced by deposit + fees; the refund is from module
	payerModuleRefund := bk.balances[payer.Addr]["uzrn"]
	// The open channel deducted 5000000 (deposit) + 100000 (fee) from payer
	// Then close refunded 1500000 from module to payer
	expectedPayerBalance := int64(100_000_000 - 5_000_000 - 100_000 + 1_500_000)
	if payerModuleRefund != expectedPayerBalance {
		t.Errorf("expected payer balance %d, got %d", expectedPayerBalance, payerModuleRefund)
	}
}

// ---------- Counter Tests ----------

func TestGetOpenChannelCountForPair(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 1_000_000_000)

	// Open 3 channels for same pair
	var channelIds []string
	for i := 0; i < 3; i++ {
		resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
			Payer:         payer.Addr,
			Receiver:      receiver.Addr,
			Deposit:       "1000000",
			TimeoutBlocks: 1000,
		})
		if err != nil {
			t.Fatalf("OpenChannel %d failed: %v", i, err)
		}
		channelIds = append(channelIds, resp.ChannelId)
	}

	count := k.GetOpenChannelCountForPair(ctx, payer.Addr, receiver.Addr)
	if count != 3 {
		t.Errorf("expected 3 open channels, got %d", count)
	}

	// Close one channel
	ch, _ := k.GetChannel(ctx, channelIds[0])
	ch.Status = types.ChannelStatusSettled
	k.SetChannel(ctx, ch)

	count = k.GetOpenChannelCountForPair(ctx, payer.Addr, receiver.Addr)
	if count != 2 {
		t.Errorf("expected 2 open channels after closing one, got %d", count)
	}
}

// ==========================================================================
// Ported from legible-money prototype — payment channel lifecycle, state
// updates, dispute resolution, and adversarial edge cases.
// ==========================================================================

// ---------- Cooperative Close Tests ----------

func TestCooperativeCloseBalanceConservation(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("OpenChannel failed: %v", err)
	}

	// Seed module balance
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 5000000}

	// Close with half to each
	counterpartySig := signPacked(receiver, "close", resp.ChannelId, 10, "2500000")
	_, err = msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "2500000",
		FinalNonce:            10,
		CounterpartySignature: counterpartySig,
	})
	if err != nil {
		t.Fatalf("CloseChannel failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected settled, got %s", ch.Status)
	}

	// Verify balance conservation: receiver got 2500000, payer got refund 2500000
	if bk.balances[receiver.Addr]["uzrn"] != 2500000 {
		t.Errorf("expected receiver 2500000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
	// Module balance should be 0 after distribution
	if bk.moduleBalances[types.ModuleName]["uzrn"] != 0 {
		t.Errorf("expected module balance 0, got %d", bk.moduleBalances[types.ModuleName]["uzrn"])
	}
}

func TestCooperativeCloseInvalidSig(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	wrongParty := newTestParty("wrongparty")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Sign with wrong party's key instead of receiver
	wrongSig := signPacked(wrongParty, "close", resp.ChannelId, 10, "3000000")
	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "3000000",
		FinalNonce:            10,
		CounterpartySignature: wrongSig,
	})
	if err == nil {
		t.Fatal("expected error for cooperative close with wrong signer")
	}
}

func TestCooperativeCloseMissingSignature(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// No counterparty signature
	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:     payer.Addr,
		ChannelId:  resp.ChannelId,
		FinalSpent: "3000000",
		FinalNonce: 10,
	})
	if err == nil {
		t.Fatal("expected error for cooperative close without signature")
	}
}

func TestCooperativeCloseSpentExceedsDeposit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Try closing with spent exceeding deposit
	counterpartySig := signPacked(receiver, "close", resp.ChannelId, 5, "99999999")
	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "99999999",
		FinalNonce:            5,
		CounterpartySignature: counterpartySig,
	})
	if err == nil {
		t.Fatal("expected error for close spent exceeding deposit")
	}
}

// ---------- State Update Sequence Tests ----------

func TestStateUpdateSequenceNumber(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "10000000",
		TimeoutBlocks: 1000,
	})

	// Update with nonce 1 -> 3 -> 10 (gaps are fine, just must increase)
	nonces := []uint64{1, 3, 10}
	spends := []string{"1000000", "3000000", "5000000"}
	for i, nonce := range nonces {
		payerSig := string(signPacked(payer, "state", resp.ChannelId, nonce, spends[i]))
		receiverSig := string(signPacked(receiver, "state", resp.ChannelId, nonce, spends[i]))
		_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
			Sender:    payer.Addr,
			ChannelId: resp.ChannelId,
			Update: &types.ChannelStateUpdate{
				Nonce:             nonce,
				Spent:             spends[i],
				PayerSignature:    payerSig,
				ReceiverSignature: receiverSig,
			},
		})
		if err != nil {
			t.Fatalf("UpdateState nonce=%d failed: %v", nonce, err)
		}
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Nonce != 10 {
		t.Errorf("expected nonce 10, got %d", ch.Nonce)
	}
	if ch.Spent != "5000000" {
		t.Errorf("expected spent 5000000, got %s", ch.Spent)
	}
	if ch.Available != "5000000" {
		t.Errorf("expected available 5000000, got %s", ch.Available)
	}
}

func TestStateUpdateSameNonceReplay(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// First update with nonce=1
	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "1000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "1000000"))
	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             1,
			Spent:             "1000000",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}

	// Replay same nonce=1 with different spent (state rewrite attack)
	payerSig2 := string(signPacked(payer, "state", resp.ChannelId, 1, "500000"))
	receiverSig2 := string(signPacked(receiver, "state", resp.ChannelId, 1, "500000"))
	_, err = msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             1,
			Spent:             "500000",
			PayerSignature:    payerSig2,
			ReceiverSignature: receiverSig2,
		},
	})
	if err == nil {
		t.Fatal("expected error: same-nonce replay should be rejected")
	}

	// Verify original state is preserved
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Nonce != 1 || ch.Spent != "1000000" {
		t.Errorf("state corrupted: nonce=%d, spent=%s", ch.Nonce, ch.Spent)
	}
}

func TestStateUpdateBalanceConservation(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Each update: deposited = spent + available must hold
	updates := []struct {
		nonce uint64
		spent string
		avail string
	}{
		{1, "1000000", "4000000"},
		{2, "2500000", "2500000"},
		{3, "4999999", "1"},
	}

	for _, u := range updates {
		payerSig := string(signPacked(payer, "state", resp.ChannelId, u.nonce, u.spent))
		receiverSig := string(signPacked(receiver, "state", resp.ChannelId, u.nonce, u.spent))
		_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
			Sender:    payer.Addr,
			ChannelId: resp.ChannelId,
			Update: &types.ChannelStateUpdate{
				Nonce:             u.nonce,
				Spent:             u.spent,
				PayerSignature:    payerSig,
				ReceiverSignature: receiverSig,
			},
		})
		if err != nil {
			t.Fatalf("UpdateState nonce=%d failed: %v", u.nonce, err)
		}

		ch, _ := k.GetChannel(ctx, resp.ChannelId)
		if ch.Available != u.avail {
			t.Errorf("nonce %d: expected available %s, got %s", u.nonce, u.avail, ch.Available)
		}
		// Verify conservation: deposited = spent + available
		if ch.Deposited != "5000000" {
			t.Errorf("nonce %d: deposited changed to %s", u.nonce, ch.Deposited)
		}
	}
}

// ---------- Dispute Resolution Tests ----------

func TestDisputeTimeoutAutoSettle(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 10000,
	})

	// Open dispute
	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 3, "3000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "3000000",
		ClaimedNonce:   3,
		ProofSignature: proofSig,
	})
	if err != nil {
		t.Fatalf("DisputeChannel failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	deadline := ch.DisputeDeadline

	// Before deadline: channel should still appear as disputed, not expired
	beforeDeadline := deadline - 1
	expired := k.GetExpiredChannels(ctx.WithBlockHeight(int64(beforeDeadline)), beforeDeadline)
	if len(expired) != 0 {
		t.Errorf("expected 0 expired before deadline, got %d", len(expired))
	}

	// After deadline: should be considered expired
	afterDeadline := deadline + 1
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 5000000}
	expired = k.GetExpiredChannels(ctx.WithBlockHeight(int64(afterDeadline)), afterDeadline)
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired channel, got %d", len(expired))
	}

	// Auto-settle should use disputed state
	k.AutoSettleChannel(ctx.WithBlockHeight(int64(afterDeadline)), expired[0])
	ch, _ = k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected settled, got %s", ch.Status)
	}

	// Receiver should get the disputed spent amount (3000000)
	if bk.balances[receiver.Addr]["uzrn"] != 3000000 {
		t.Errorf("expected receiver 3000000, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
	// Payer gets refund of 2000000 from module; their balance is initial minus deposit/fee plus refund
	// initial: 100M, deposit: 5M, fee: 100K, refund from module: 2M
	expectedPayerBal := int64(100_000_000 - 5_000_000 - 100_000 + 2_000_000)
	if bk.balances[payer.Addr]["uzrn"] != expectedPayerBal {
		t.Errorf("expected payer balance %d, got %d", expectedPayerBal, bk.balances[payer.Addr]["uzrn"])
	}
}

func TestDisputeCounterEvidenceHigherNonce(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 10000,
	})

	// Payer disputes claiming receiver owes less (nonce 2, spent 1000000)
	proofSig1 := signPacked(receiver, "dispute", resp.ChannelId, 2, "1000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "1000000",
		ClaimedNonce:   2,
		ProofSignature: proofSig1,
	})
	if err != nil {
		t.Fatalf("first dispute failed: %v", err)
	}

	// Receiver counters with higher nonce (nonce 8, spent 4000000)
	proofSig2 := signPacked(payer, "dispute", resp.ChannelId, 8, "4000000")
	_, err = msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       receiver.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "4000000",
		ClaimedNonce:   8,
		ProofSignature: proofSig2,
	})
	if err != nil {
		t.Fatalf("counter-dispute failed: %v", err)
	}

	// The dispute record should reflect the newer state
	dispute, found := k.GetDispute(ctx, resp.ChannelId)
	if !found {
		t.Fatal("dispute not found")
	}
	if dispute.DisputedNonce != 8 {
		t.Errorf("expected nonce 8 from counter-evidence, got %d", dispute.DisputedNonce)
	}
	if dispute.DisputedSpent != "4000000" {
		t.Errorf("expected spent 4000000 from counter-evidence, got %s", dispute.DisputedSpent)
	}
	if dispute.Disputer != receiver.Addr {
		t.Errorf("expected disputer to be receiver, got %s", dispute.Disputer)
	}
}

func TestDisputeSameNonceRejected(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// First dispute with nonce=5
	proofSig := signPacked(receiver, "dispute", resp.ChannelId, 5, "2000000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       payer.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "2000000",
		ClaimedNonce:   5,
		ProofSignature: proofSig,
	})
	if err != nil {
		t.Fatalf("first dispute failed: %v", err)
	}

	// Second dispute with SAME nonce=5 (should be rejected, needs higher)
	proofSig2 := signPacked(payer, "dispute", resp.ChannelId, 5, "500000")
	_, err = msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       receiver.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "500000",
		ClaimedNonce:   5,
		ProofSignature: proofSig2,
	})
	if err == nil {
		t.Fatal("expected error: dispute with same nonce should be rejected")
	}
}

func TestDisputeMissingSignature(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Dispute without proof signature
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:     payer.Addr,
		ChannelId:    resp.ChannelId,
		ClaimedSpent: "2000000",
		ClaimedNonce: 3,
		// No ProofSignature
	})
	if err == nil {
		t.Fatal("expected error for dispute without proof signature")
	}
}

// ---------- Edge Cases ----------

func TestChannelWithMinimumDeposit(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	// Open with exactly minimum deposit (1000000)
	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "1000000",
		TimeoutBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("OpenChannel with min deposit failed: %v", err)
	}

	ch, found := k.GetChannel(ctx, resp.ChannelId)
	if !found {
		t.Fatal("channel not found")
	}
	if ch.Deposited != "1000000" {
		t.Errorf("expected deposited 1000000, got %s", ch.Deposited)
	}
	if ch.Available != "1000000" {
		t.Errorf("expected available 1000000, got %s", ch.Available)
	}

	// Spend the full amount
	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "1000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "1000000"))
	_, err = msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             1,
			Spent:             "1000000",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err != nil {
		t.Fatalf("UpdateState to spend full deposit failed: %v", err)
	}

	ch, _ = k.GetChannel(ctx, resp.ChannelId)
	if ch.Available != "0" {
		t.Errorf("expected available 0 after full spend, got %s", ch.Available)
	}
}

func TestChannelMaxTimeout(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	// Default MaxTimeoutBlocks is 1000000 — open with exactly that
	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000000,
	})
	if err != nil {
		t.Fatalf("OpenChannel with max timeout failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	expectedExpiry := uint64(ctx.BlockHeight()) + 1000000
	if ch.ExpiresAtBlock != expectedExpiry {
		t.Errorf("expected expires at %d, got %d", expectedExpiry, ch.ExpiresAtBlock)
	}
}

func TestMultipleChannelsBetweenSamePair(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 1_000_000_000)

	// Open multiple channels between the same pair
	var channelIds []string
	for i := 0; i < 5; i++ {
		resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
			Payer:         payer.Addr,
			Receiver:      receiver.Addr,
			Deposit:       "2000000",
			TimeoutBlocks: 1000,
		})
		if err != nil {
			t.Fatalf("OpenChannel %d failed: %v", i, err)
		}
		channelIds = append(channelIds, resp.ChannelId)
	}

	// Verify all channel IDs are unique
	seen := make(map[string]bool)
	for _, id := range channelIds {
		if seen[id] {
			t.Fatalf("duplicate channel ID: %s", id)
		}
		seen[id] = true
	}

	// Verify payer has 5 channels
	channels := k.GetChannelsByPayer(ctx, payer.Addr)
	if len(channels) != 5 {
		t.Errorf("expected 5 channels for payer, got %d", len(channels))
	}

	// Can update state on each independently
	for i, cid := range channelIds {
		spent := fmt.Sprintf("%d", (i+1)*100000)
		nonce := uint64(i + 1)
		payerSig := string(signPacked(payer, "state", cid, nonce, spent))
		receiverSig := string(signPacked(receiver, "state", cid, nonce, spent))
		_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
			Sender:    payer.Addr,
			ChannelId: cid,
			Update: &types.ChannelStateUpdate{
				Nonce:             nonce,
				Spent:             spent,
				PayerSignature:    payerSig,
				ReceiverSignature: receiverSig,
			},
		})
		if err != nil {
			t.Fatalf("UpdateState channel %s failed: %v", cid, err)
		}
	}

	// Verify each channel has independent state
	for i, cid := range channelIds {
		ch, _ := k.GetChannel(ctx, cid)
		expectedSpent := fmt.Sprintf("%d", (i+1)*100000)
		if ch.Spent != expectedSpent {
			t.Errorf("channel %s: expected spent %s, got %s", cid, expectedSpent, ch.Spent)
		}
	}
}

func TestChannelIdDeterminism(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 1_000_000_000)

	ids := make(map[string]bool)
	for i := 0; i < 3; i++ {
		resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
			Payer:         payer.Addr,
			Receiver:      receiver.Addr,
			Deposit:       "1000000",
			TimeoutBlocks: 1000,
		})
		if err != nil {
			t.Fatalf("open channel %d: %v", i+1, err)
		}
		if ids[resp.ChannelId] {
			t.Errorf("channel ID collision: %s", resp.ChannelId)
		}
		ids[resp.ChannelId] = true

		// Verify ID follows pattern pc-{blockHeight}-{counter}
		expectedPrefix := "pc-100-"
		if len(resp.ChannelId) < len(expectedPrefix) || resp.ChannelId[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("channel ID %s does not match expected prefix %s", resp.ChannelId, expectedPrefix)
		}
	}

	if len(ids) != 3 {
		t.Errorf("expected 3 unique IDs, got %d", len(ids))
	}
}

func TestDepositToDisputedChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Manually set to disputed
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusDisputed
	k.SetChannel(ctx, ch)

	_, err := msgSrv.DepositChannel(ctx, &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: resp.ChannelId,
		Amount:    "1000000",
	})
	if err == nil {
		t.Fatal("expected error for depositing to disputed channel")
	}
}

func TestUpdateStateDisputedChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Set to disputed
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusDisputed
	k.SetChannel(ctx, ch)

	payerSig := string(signPacked(payer, "state", resp.ChannelId, 1, "1000000"))
	receiverSig := string(signPacked(receiver, "state", resp.ChannelId, 1, "1000000"))
	_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
		Sender:    payer.Addr,
		ChannelId: resp.ChannelId,
		Update: &types.ChannelStateUpdate{
			Nonce:             1,
			Spent:             "1000000",
			PayerSignature:    payerSig,
			ReceiverSignature: receiverSig,
		},
	})
	if err == nil {
		t.Fatal("expected error for updating state on disputed channel")
	}
}

func TestCloseDisputedChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 1000,
	})

	// Set to disputed
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusDisputed
	k.SetChannel(ctx, ch)

	counterpartySig := signPacked(receiver, "close", resp.ChannelId, 1, "1000000")
	_, err := msgSrv.CloseChannel(ctx, &types.MsgCloseChannel{
		Closer:                payer.Addr,
		ChannelId:             resp.ChannelId,
		FinalSpent:            "1000000",
		FinalNonce:            1,
		CounterpartySignature: counterpartySig,
	})
	if err == nil {
		t.Fatal("expected error for closing disputed channel (cooperative close requires open status)")
	}
}

func TestClaimExpiredDisputedChannel(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 200,
	})

	// Set to disputed (ClaimExpired requires open status)
	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	ch.Status = types.ChannelStatusDisputed
	k.SetChannel(ctx, ch)

	expiredCtx := ctx.WithBlockHeight(301)
	_, err := msgSrv.ClaimExpired(expiredCtx, &types.MsgClaimExpired{
		Claimer:   payer.Addr,
		ChannelId: resp.ChannelId,
	})
	if err == nil {
		t.Fatal("expected error: ClaimExpired on disputed channel should fail (status is not open)")
	}
}

func TestFullLifecycle_OpenDisputeAutoSettle(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	// 1. Open channel
	resp, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 10000,
	})
	if err != nil {
		t.Fatalf("OpenChannel: %v", err)
	}

	// 2. Update state a few times
	for i := uint64(1); i <= 3; i++ {
		spent := fmt.Sprintf("%d", i*1000000)
		payerSig := string(signPacked(payer, "state", resp.ChannelId, i, spent))
		receiverSig := string(signPacked(receiver, "state", resp.ChannelId, i, spent))
		_, err := msgSrv.UpdateState(ctx, &types.MsgUpdateState{
			Sender:    payer.Addr,
			ChannelId: resp.ChannelId,
			Update: &types.ChannelStateUpdate{
				Nonce:             i,
				Spent:             spent,
				PayerSignature:    payerSig,
				ReceiverSignature: receiverSig,
			},
		})
		if err != nil {
			t.Fatalf("UpdateState nonce=%d: %v", i, err)
		}
	}

	// 3. Dispute — receiver shows higher state (nonce 5, spent 4000000)
	proofSig := signPacked(payer, "dispute", resp.ChannelId, 5, "4000000")
	_, err = msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       receiver.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "4000000",
		ClaimedNonce:   5,
		ProofSignature: proofSig,
	})
	if err != nil {
		t.Fatalf("DisputeChannel: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusDisputed {
		t.Fatalf("expected disputed, got %s", ch.Status)
	}

	// 4. Advance past dispute deadline and auto-settle
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 5000000}
	pastDeadline := ch.DisputeDeadline + 1
	settleCtx := ctx.WithBlockHeight(int64(pastDeadline))

	expired := k.GetExpiredChannels(settleCtx, pastDeadline)
	if len(expired) != 1 {
		t.Fatalf("expected 1 expired, got %d", len(expired))
	}

	k.AutoSettleChannel(settleCtx, expired[0])

	ch, _ = k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusSettled {
		t.Errorf("expected settled, got %s", ch.Status)
	}

	// 5. Verify disputed state was used for settlement
	if bk.balances[receiver.Addr]["uzrn"] != 4000000 {
		t.Errorf("expected receiver 4000000 from dispute, got %d", bk.balances[receiver.Addr]["uzrn"])
	}
	// Payer gets refund of 1000000 from module; their balance is initial minus deposit/fee plus refund
	// initial: 100M, deposit: 5M, fee: 100K, refund: 1M
	expectedPayerBal := int64(100_000_000 - 5_000_000 - 100_000 + 1_000_000)
	if bk.balances[payer.Addr]["uzrn"] != expectedPayerBal {
		t.Errorf("expected payer balance %d, got %d", expectedPayerBal, bk.balances[payer.Addr]["uzrn"])
	}

	dispute, _ := k.GetDispute(ctx, resp.ChannelId)
	if !dispute.Resolved {
		t.Error("expected dispute resolved")
	}
}

func TestOpenChannelEmptyDeposit(t *testing.T) {
	msgSrv, _, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	_, err := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "",
		TimeoutBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for empty deposit string")
	}
}

func TestDisputeReceiverInitiated(t *testing.T) {
	msgSrv, k, ctx, bk := setupMsgServer(t)

	payer := newTestParty("payer")
	receiver := newTestParty("receiver")
	bk.setBalance(payer.Addr, "uzrn", 100_000_000)

	resp, _ := msgSrv.OpenChannel(ctx, &types.MsgOpenChannel{
		Payer:         payer.Addr,
		Receiver:      receiver.Addr,
		Deposit:       "5000000",
		TimeoutBlocks: 10000,
	})

	// Receiver disputes, proving payer signed this state
	proofSig := signPacked(payer, "dispute", resp.ChannelId, 4, "3500000")
	_, err := msgSrv.DisputeChannel(ctx, &types.MsgDisputeChannel{
		Disputer:       receiver.Addr,
		ChannelId:      resp.ChannelId,
		ClaimedSpent:   "3500000",
		ClaimedNonce:   4,
		ProofSignature: proofSig,
	})
	if err != nil {
		t.Fatalf("receiver-initiated dispute failed: %v", err)
	}

	ch, _ := k.GetChannel(ctx, resp.ChannelId)
	if ch.Status != types.ChannelStatusDisputed {
		t.Errorf("expected disputed, got %s", ch.Status)
	}

	dispute, found := k.GetDispute(ctx, resp.ChannelId)
	if !found {
		t.Fatal("dispute not found")
	}
	if dispute.Disputer != receiver.Addr {
		t.Errorf("expected disputer to be receiver, got %s", dispute.Disputer)
	}
	if dispute.DisputedSpent != "3500000" {
		t.Errorf("expected disputed spent 3500000, got %s", dispute.DisputedSpent)
	}
}

func TestValidateBasicDepositChannel(t *testing.T) {
	payer := newTestParty("payer")

	// Valid
	msg := &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: "pc-100-1",
		Amount:    "1000000",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid: %v", err)
	}

	// Empty channel ID
	msg2 := &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: "",
		Amount:    "1000000",
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Error("expected error for empty channel ID")
	}

	// Zero amount
	msg3 := &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: "pc-100-1",
		Amount:    "0",
	}
	if err := msg3.ValidateBasic(); err == nil {
		t.Error("expected error for zero amount")
	}

	// Empty amount
	msg4 := &types.MsgDepositChannel{
		Depositor: payer.Addr,
		ChannelId: "pc-100-1",
		Amount:    "",
	}
	if err := msg4.ValidateBasic(); err == nil {
		t.Error("expected error for empty amount")
	}
}

func TestValidateBasicCloseChannel(t *testing.T) {
	payer := newTestParty("payer")

	// Valid
	msg := &types.MsgCloseChannel{
		Closer:    payer.Addr,
		ChannelId: "pc-100-1",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid: %v", err)
	}

	// Empty channel ID
	msg2 := &types.MsgCloseChannel{
		Closer:    payer.Addr,
		ChannelId: "",
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Error("expected error for empty channel ID")
	}
}

func TestValidateBasicDisputeChannel(t *testing.T) {
	payer := newTestParty("payer")

	// Valid
	msg := &types.MsgDisputeChannel{
		Disputer:  payer.Addr,
		ChannelId: "pc-100-1",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid: %v", err)
	}

	// Empty channel ID
	msg2 := &types.MsgDisputeChannel{
		Disputer:  payer.Addr,
		ChannelId: "",
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Error("expected error for empty channel ID in dispute")
	}
}

func TestValidateBasicClaimExpired(t *testing.T) {
	payer := newTestParty("payer")

	// Valid
	msg := &types.MsgClaimExpired{
		Claimer:   payer.Addr,
		ChannelId: "pc-100-1",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid: %v", err)
	}

	// Empty channel ID
	msg2 := &types.MsgClaimExpired{
		Claimer:   payer.Addr,
		ChannelId: "",
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Error("expected error for empty channel ID in claim expired")
	}
}
