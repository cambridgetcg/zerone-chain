package keeper_test

import (
	"context"
	"crypto/sha256"
	"testing"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/counterexamples/keeper"
	"github.com/zerone-chain/zerone/x/counterexamples/types"
)

func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("zrn", "zrnpub")
	cfg.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

func testAddr(name string) string {
	h := sha256.Sum256([]byte("counterex_test:" + name))
	return sdk.AccAddress(h[:20]).String()
}

func setup(t *testing.T) (keeper.Keeper, types.MsgServer, types.QueryServer, sdk.Context) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("load store: %v", err)
	}
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(storeService, cdc, testAddr("authority"))
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, ChainID: "zerone-test"}, false, log.NewNopLogger())
	return k, keeper.NewMsgServerImpl(k), keeper.NewQueryServerImpl(k), ctx
}

func TestPropose_AssignsMonotonicIDs(t *testing.T) {
	_, ms, _, ctx := setup(t)
	a := testAddr("author")
	r1, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: a, FactId: "fact-1", WrongClaim: "x", Reasoning: "y",
		ErrorType: types.ErrorType_ERROR_TYPE_REASONING,
	})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: a, FactId: "fact-1", WrongClaim: "x2", Reasoning: "y2",
		ErrorType: types.ErrorType_ERROR_TYPE_CATEGORICAL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if r1.CounterexampleId == r2.CounterexampleId {
		t.Fatalf("ids must be unique: %s == %s", r1.CounterexampleId, r2.CounterexampleId)
	}
}

func TestValidate_AutoResolvesAtThreshold(t *testing.T) {
	k, ms, _, ctx := setup(t)
	a := testAddr("author")
	resp, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: a, FactId: "fact-1", WrongClaim: "x", Reasoning: "y",
		ErrorType: types.ErrorType_ERROR_TYPE_FACTUAL,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Three affirmations from distinct validators meets default
	// min_votes=3 with 100% affirm — must resolve VALIDATED on the
	// 3rd vote.
	for i, name := range []string{"v1", "v2", "v3"} {
		resolved := i == 2
		validateResp, err := ms.Validate(ctx, &types.MsgValidate{
			Validator: testAddr(name), CounterexampleId: resp.CounterexampleId,
			Affirm: true, Reason: "ok",
		})
		if err != nil {
			t.Fatal(err)
		}
		if validateResp.Resolved != resolved {
			t.Fatalf("vote %d: Resolved=%v want %v", i+1, validateResp.Resolved, resolved)
		}
	}
	got, ok := k.GetCounterexample(ctx, resp.CounterexampleId)
	if !ok {
		t.Fatal("counterexample missing after validation")
	}
	if got.Status != types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_VALIDATED {
		t.Fatalf("expected VALIDATED, got %s", got.Status)
	}
	if !k.HasValidatedCounterexample(ctx, "fact-1") {
		t.Fatalf("HasValidatedCounterexample should return true")
	}
}

func TestValidate_RejectsBelowThreshold(t *testing.T) {
	k, ms, _, ctx := setup(t)
	a := testAddr("author")
	resp, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: a, FactId: "fact-1", WrongClaim: "x", Reasoning: "y",
		ErrorType: types.ErrorType_ERROR_TYPE_OMISSION,
	})
	if err != nil {
		t.Fatal(err)
	}
	// 1 affirm + 2 rejects = 33% affirm ratio, below 66.6% threshold.
	_, err = ms.Validate(ctx, &types.MsgValidate{
		Validator: testAddr("v1"), CounterexampleId: resp.CounterexampleId, Affirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ms.Validate(ctx, &types.MsgValidate{
		Validator: testAddr("v2"), CounterexampleId: resp.CounterexampleId, Affirm: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ms.Validate(ctx, &types.MsgValidate{
		Validator: testAddr("v3"), CounterexampleId: resp.CounterexampleId, Affirm: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := k.GetCounterexample(ctx, resp.CounterexampleId)
	if !ok {
		t.Fatal("counterexample missing")
	}
	if got.Status != types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_REJECTED {
		t.Fatalf("expected REJECTED, got %s", got.Status)
	}
	if k.HasValidatedCounterexample(ctx, "fact-1") {
		t.Fatalf("HasValidatedCounterexample should return false for rejected")
	}
}

func TestValidate_DoubleVoteRejected(t *testing.T) {
	_, ms, _, ctx := setup(t)
	a := testAddr("author")
	resp, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: a, FactId: "fact-1", WrongClaim: "x", Reasoning: "y",
		ErrorType: types.ErrorType_ERROR_TYPE_REASONING,
	})
	if err != nil {
		t.Fatal(err)
	}
	v := testAddr("v1")
	if _, err := ms.Validate(ctx, &types.MsgValidate{
		Validator: v, CounterexampleId: resp.CounterexampleId, Affirm: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Validate(ctx, &types.MsgValidate{
		Validator: v, CounterexampleId: resp.CounterexampleId, Affirm: true,
	}); err == nil {
		t.Fatalf("expected double-vote rejection")
	}
}

func TestPropose_RejectedWhenFactKeeperSaysFactDoesNotExist(t *testing.T) {
	k, _, _, ctx := setup(t)
	k.SetFactKeeper(stubFactKeeper{exists: false})
	ms := keeper.NewMsgServerImpl(k)
	_, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: testAddr("author"), FactId: "missing", WrongClaim: "x", Reasoning: "y",
		ErrorType: types.ErrorType_ERROR_TYPE_FACTUAL,
	})
	if err == nil {
		t.Fatalf("expected proposal to fail when fact does not exist")
	}
}

func TestPropose_AcceptedWhenFactKeeperConfirms(t *testing.T) {
	k, _, _, ctx := setup(t)
	k.SetFactKeeper(stubFactKeeper{exists: true})
	ms := keeper.NewMsgServerImpl(k)
	_, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: testAddr("author"), FactId: "fact-1", WrongClaim: "x", Reasoning: "y",
		ErrorType: types.ErrorType_ERROR_TYPE_FACTUAL,
	})
	if err != nil {
		t.Fatalf("proposal should succeed when fact exists: %v", err)
	}
}

func TestQueryCounterexamplesByFact(t *testing.T) {
	_, ms, qs, ctx := setup(t)
	a := testAddr("author")
	for i := 0; i < 3; i++ {
		_, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
			Author: a, FactId: "fact-A", WrongClaim: "x", Reasoning: "y",
			ErrorType: types.ErrorType_ERROR_TYPE_REASONING,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// One unrelated counterexample on a different fact.
	if _, err := ms.ProposeCounterexample(ctx, &types.MsgProposeCounterexample{
		Author: a, FactId: "fact-B", WrongClaim: "x", Reasoning: "y",
		ErrorType: types.ErrorType_ERROR_TYPE_REASONING,
	}); err != nil {
		t.Fatal(err)
	}
	resp, err := qs.CounterexamplesByFact(ctx, &types.QueryByFactRequest{FactId: "fact-A"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Counterexamples) != 3 {
		t.Fatalf("expected 3 counterexamples for fact-A, got %d", len(resp.Counterexamples))
	}
	for _, c := range resp.Counterexamples {
		if c.FactId != "fact-A" {
			t.Fatalf("counterexample leaked from another fact: %s", c.Id)
		}
	}
}

type stubFactKeeper struct{ exists bool }

func (s stubFactKeeper) FactExists(_ context.Context, _ string) bool { return s.exists }
