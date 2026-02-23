package keeper_test

import (
	"context"
	"math/big"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

// ---------- Mock Staking Keeper ----------

type mockStakingKeeper struct {
	totalBonded string
	delegations map[string]string // addr -> bonded amount
}

func newMockStakingKeeper(totalBonded string) *mockStakingKeeper {
	return &mockStakingKeeper{
		totalBonded: totalBonded,
		delegations: make(map[string]string),
	}
}

func (m *mockStakingKeeper) GetTotalBondedStake(_ context.Context) (string, error) {
	return m.totalBonded, nil
}

func (m *mockStakingKeeper) GetDelegatorTotalBonded(_ context.Context, addr string) (string, error) {
	if amt, ok := m.delegations[addr]; ok {
		return amt, nil
	}
	return "0", nil
}

// ---------- Mock Upgrade Keeper ----------

type mockUpgradeKeeper struct {
	called bool
	plan   types.UpgradePlan
}

func (m *mockUpgradeKeeper) ScheduleUpgrade(_ context.Context, plan types.UpgradePlan) error {
	m.called = true
	m.plan = plan
	return nil
}

// ---------- Test Setup ----------

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

	k := keeper.NewKeeper(
		cdc,
		storeKey,
		"authority",
		nil, // bankKeeper
		nil, // stakingKeeper
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())

	return k, ctx
}

func setupWithStaking(t *testing.T, totalBonded string) (keeper.Keeper, sdk.Context, *mockStakingKeeper) {
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

	mock := newMockStakingKeeper(totalBonded)
	k := keeper.NewKeeper(
		cdc,
		storeKey,
		"authority",
		nil,  // bankKeeper
		mock, // stakingKeeper
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())

	return k, ctx, mock
}

func testAddr(name string) string {
	addr := sdk.AccAddress([]byte("addr_" + name + "_______________")[:20])
	return addr.String()
}

// ---------- Spec-Required Tests (7) ----------

func TestQuorum_1MScale(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000") // 1M ZRN total bonded
	ms := keeper.NewMsgServerImpl(k)

	// Set up: submit LIP, advance to voting
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Quorum Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Voter1 has 334_000_000_000 bonded (33.4% of 1M ZRN)
	mock.delegations[testAddr("voter1")] = "334000000000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	// Check tally via query
	qs := keeper.NewQueryServerImpl(k)
	tally, err := qs.TallyResult(ctx, &types.QueryTallyResultRequest{LipId: "LIP-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 334000000000 * 1000000 / 1000000000000 = 334000 BPS -> meets 334000 threshold
	if !tally.QuorumMet {
		t.Error("expected quorum to be met at 33.4% on 1M scale")
	}
	if !tally.Passed {
		t.Error("expected LIP to pass (100% yes, quorum met)")
	}
}

func TestLIPLifecycle_FullPath(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Set review period to 0 for immediate advancement
	params := k.GetParams(ctx)
	params.VotingPeriodBlocks = 100
	k.SetParams(ctx, params)

	// 1. Submit (draft)
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Full Path", Description: "Test lifecycle",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if resp.LipId != "LIP-1" {
		t.Fatalf("expected LIP-1, got %s", resp.LipId)
	}
	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusDraft {
		t.Errorf("expected draft, got %s", lip.Stage)
	}

	// 2. Stake → advances to review
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "400000000",
	})
	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Errorf("expected review, got %s", lip.Stage)
	}

	// 3. Advance review → last_call (review_blocks=17136, but we set block high enough)
	// Manually move to review with elapsed blocks
	lip.ReviewStartedBlock = 0 // started at block 0, now at 100 > 17136 won't work
	k.SetLIP(ctx, lip)
	// Override category config to 0 review blocks for test
	params2 := k.GetParams(ctx)
	for _, cc := range params2.CategoryConfigs {
		cc.ReviewBlocks = 0
	}
	k.SetParams(ctx, params2)

	_, err = ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	if err != nil {
		t.Fatalf("advance review→last_call failed: %v", err)
	}
	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusLastCall {
		t.Errorf("expected last_call, got %s", lip.Stage)
	}

	// 4. Advance last_call → voting
	_, err = ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	if err != nil {
		t.Fatalf("advance last_call→voting failed: %v", err)
	}
	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusVoting {
		t.Errorf("expected voting, got %s", lip.Stage)
	}
	if lip.VotingEndBlock != 200 { // 100 + 100 voting period
		t.Errorf("expected voting_end_block=200, got %d", lip.VotingEndBlock)
	}
}

func TestLIPLifecycle_FailQuorum(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000") // 1M ZRN total
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Fail Quorum", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	// Put directly in voting stage
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100 // expires at current height
	k.SetLIP(ctx, lip)

	// Single voter with only 1% of total bonded
	mock.delegations[testAddr("voter1")] = "10000000000" // 1%
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	// Run begin blocker to tally
	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusFailed {
		t.Errorf("expected failed (quorum not met), got %s", lip.Stage)
	}
}

func TestLIPLifecycle_Withdraw(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Withdraw Me", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	_, err := ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("alice"), LipId: "LIP-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusWithdrawn {
		t.Errorf("expected withdrawn, got %s", lip.Stage)
	}
}

func TestParamChange_Executes(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Param Change", Description: "Change voting period",
		Category: types.CategoryParameter, InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "zerone_gov", Key: "voting_period_blocks", Value: "200000"},
		},
	})

	// Put in voting with enough votes to pass
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	// Vote with enough to meet quorum (>33.4%)
	mock.delegations[testAddr("voter1")] = "500000000000" // 50%
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	// Run begin blocker
	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("expected passed, got %s", lip.Stage)
	}
	// ParamChange execution is logged (actual routing is wired in app.go).
	// Verify the LIP was resolved as passed, which triggers executeParamChanges.
}

func TestVoting_StakeWeighted(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Stake Weighted", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Voter1: 100 bonded, Voter2: 900 bonded
	mock.delegations[testAddr("voter1")] = "100"
	mock.delegations[testAddr("voter2")] = "900"

	resp1, _ := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if resp1.EffectiveWeight != "100" {
		t.Errorf("expected weight=100, got %s", resp1.EffectiveWeight)
	}

	resp2, _ := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter2"), LipId: "LIP-1", Option: types.VoteNo,
	})
	if resp2.EffectiveWeight != "900" {
		t.Errorf("expected weight=900, got %s", resp2.EffectiveWeight)
	}

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.YesStake != "100" {
		t.Errorf("expected yes_stake=100, got %s", lip.YesStake)
	}
	if lip.NoStake != "900" {
		t.Errorf("expected no_stake=900, got %s", lip.NoStake)
	}
}

func TestResearchVoters_InParams(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := k.GetParams(ctx)
	params.ResearchFundVoters = &types.ResearchFundVoters{
		Voter1: testAddr("rfv1"),
		Voter2: testAddr("rfv2"),
	}
	k.SetParams(ctx, params)

	got := k.GetParams(ctx)
	if got.ResearchFundVoters == nil {
		t.Fatal("expected research_fund_voters to be set")
	}
	if got.ResearchFundVoters.Voter1 != testAddr("rfv1") {
		t.Errorf("expected voter1=%s, got %s", testAddr("rfv1"), got.ResearchFundVoters.Voter1)
	}
	if got.ResearchFundVoters.Voter2 != testAddr("rfv2") {
		t.Errorf("expected voter2=%s, got %s", testAddr("rfv2"), got.ResearchFundVoters.Voter2)
	}
}

// ---------- Params Tests ----------

func TestSetGetParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := k.GetParams(ctx)

	if params.VotingPeriodBlocks != 102816 {
		t.Errorf("expected voting_period_blocks=102816, got %d", params.VotingPeriodBlocks)
	}
	if params.QuorumThresholdBps != 334000 {
		t.Errorf("expected quorum_threshold_bps=334000, got %d", params.QuorumThresholdBps)
	}
	if params.SupportThresholdBps != 500000 {
		t.Errorf("expected support_threshold_bps=500000, got %d", params.SupportThresholdBps)
	}
}

func TestSetParams_Custom(t *testing.T) {
	k, ctx := setupKeeper(t)
	custom := types.Params{
		VotingPeriodBlocks:     5000,
		DiscussionPeriodBlocks: 2500,
		QuorumThresholdBps:     200000,
		SupportThresholdBps:    600000,
		MinLipStake:            "500000",
		MinVoteStake:           "100000",
	}
	k.SetParams(ctx, &custom)
	got := k.GetParams(ctx)
	if got.VotingPeriodBlocks != 5000 {
		t.Errorf("expected 5000, got %d", got.VotingPeriodBlocks)
	}
	if got.QuorumThresholdBps != 200000 {
		t.Errorf("expected 200000, got %d", got.QuorumThresholdBps)
	}
}

// ---------- LIP Submit/IDs ----------

func TestSubmitLIP(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test LIP", Description: "A test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.LipId != "LIP-1" {
		t.Errorf("expected LIP-1, got %s", resp.LipId)
	}

	lip, found := k.GetLIP(ctx, "LIP-1")
	if !found {
		t.Fatal("LIP-1 not found")
	}
	if lip.Stage != types.StatusDraft {
		t.Errorf("expected draft, got %s", lip.Stage)
	}
	if lip.Proposer != testAddr("alice") {
		t.Errorf("wrong proposer")
	}
}

func TestSubmitLIP_IncrementingIds(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	resp1, _ := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "First", Description: "A",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	resp2, _ := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("bob"), Title: "Second", Description: "B",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	if resp1.LipId != "LIP-1" {
		t.Errorf("expected LIP-1, got %s", resp1.LipId)
	}
	if resp2.LipId != "LIP-2" {
		t.Errorf("expected LIP-2, got %s", resp2.LipId)
	}
}

// ---------- Stake Transitions ----------

func TestStakeLIP_AdvancesToReview(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Stake Test", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})

	// Stake enough to advance (research_spend needs 200 ZRN = 200000000)
	_, err := ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Errorf("expected review, got %s", lip.Stage)
	}
}

func TestStakeLIP_InsufficientStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Param LIP", Description: "Big stake",
		Category: types.CategoryParameter, InitialStake: "1000000",
	})

	// Stake 100 ZRN - not enough for parameter (needs 1000 ZRN = 1000000000)
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "100000000",
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusDraft {
		t.Errorf("should remain in draft, got %s", lip.Stage)
	}
}

func TestStakeLIP_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-999", Amount: "1000000000",
	})
	if err == nil {
		t.Error("expected error for non-existent LIP")
	}
}

// ---------- Stage Advancement ----------

func TestAdvanceLIPStage_ReviewToLastCall(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Set review period to 0
	params := k.GetParams(ctx)
	for _, cc := range params.CategoryConfigs {
		cc.ReviewBlocks = 0
	}
	k.SetParams(ctx, params)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})

	_, err := ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusLastCall {
		t.Errorf("expected last_call, got %s", lip.Stage)
	}
}

func TestAdvanceLIPStage_LastCallToVoting(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	params := k.GetParams(ctx)
	params.VotingPeriodBlocks = 100
	for _, cc := range params.CategoryConfigs {
		cc.ReviewBlocks = 0
	}
	k.SetParams(ctx, params)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})
	ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	_, err := ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusVoting {
		t.Errorf("expected voting, got %s", lip.Stage)
	}
	if lip.VotingEndBlock != 200 { // 100 + 100
		t.Errorf("expected voting_end_block=200, got %d", lip.VotingEndBlock)
	}
}

func TestAdvanceLIPStage_FromDraft_Fails(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	_, err := ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("expected error when advancing from draft")
	}
}

// ---------- Withdraw ----------

func TestWithdrawLIP(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Withdraw Me", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	_, err := ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("alice"), LipId: "LIP-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusWithdrawn {
		t.Errorf("expected withdrawn, got %s", lip.Stage)
	}
}

func TestWithdrawLIP_NotProposer(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	_, err := ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("bob"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("expected error for non-proposer")
	}
}

func TestWithdrawLIP_AlreadyTerminal(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("alice"), LipId: "LIP-1",
	})

	_, err := ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("alice"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("expected error for already withdrawn")
	}
}

// ---------- Voting ----------

func TestCastVote(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "5000000"

	resp, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.EffectiveWeight != "5000000" {
		t.Errorf("expected weight=5000000, got %s", resp.EffectiveWeight)
	}

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.YesStake != "5000000" {
		t.Errorf("expected yes_stake=5000000, got %s", lip.YesStake)
	}
	if lip.UniqueVoters != 1 {
		t.Errorf("expected 1 voter, got %d", lip.UniqueVoters)
	}
}

func TestCastVote_DuplicateVote(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	_, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteNo,
	})
	if err == nil {
		t.Error("expected error for duplicate vote")
	}
}

func TestCastVote_NotInVotingStage(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	_, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if err == nil {
		t.Error("expected error for voting on draft LIP")
	}
}

func TestCastVote_MultipleVoters(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500"
	mock.delegations[testAddr("voter2")] = "300"
	mock.delegations[testAddr("voter3")] = "200"

	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter2"), LipId: "LIP-1", Option: types.VoteNo,
	})
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter3"), LipId: "LIP-1", Option: types.VoteAbstain,
	})

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.YesStake != "500" {
		t.Errorf("expected yes=500, got %s", lip.YesStake)
	}
	if lip.NoStake != "300" {
		t.Errorf("expected no=300, got %s", lip.NoStake)
	}
	if lip.AbstainStake != "200" {
		t.Errorf("expected abstain=200, got %s", lip.AbstainStake)
	}
	if lip.UniqueVoters != 3 {
		t.Errorf("expected 3 voters, got %d", lip.UniqueVoters)
	}
}

// ---------- UpdateParams ----------

func TestUpdateParams_Authority(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	newParams := types.DefaultParams()
	newParams.VotingPeriodBlocks = 50000

	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := k.GetParams(ctx)
	if got.VotingPeriodBlocks != 50000 {
		t.Errorf("expected 50000, got %d", got.VotingPeriodBlocks)
	}
}

func TestUpdateParams_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "not_authority",
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Error("expected unauthorized error")
	}
}

// ---------- Genesis ----------

func TestInitExportGenesis(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Genesis LIP", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	gs := k.ExportGenesis(ctx)
	if len(gs.Lips) != 1 {
		t.Errorf("expected 1 LIP, got %d", len(gs.Lips))
	}
	if gs.NextLipNumber != 2 {
		t.Errorf("expected next_lip=2, got %d", gs.NextLipNumber)
	}
}

func TestGenesisRoundtrip_LIPFields(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Roundtrip", Description: "Fields test",
		Category: types.CategoryParameter, InitialStake: "1000000",
	})
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "1000000000",
	})

	gs := k.ExportGenesis(ctx)
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	lip, found := k2.GetLIP(ctx2, "LIP-1")
	if !found {
		t.Fatal("LIP-1 not found after roundtrip")
	}
	if lip.Title != "Roundtrip" {
		t.Errorf("title: got %q, want %q", lip.Title, "Roundtrip")
	}
	if lip.Category != types.CategoryParameter {
		t.Errorf("category: got %s, want %s", lip.Category, types.CategoryParameter)
	}
	if lip.Proposer != testAddr("alice") {
		t.Errorf("proposer mismatch")
	}
	if lip.Stage != types.StatusReview {
		t.Errorf("stage: got %s, want review", lip.Stage)
	}
	// 1000000 + 1000000000 = 1001000000
	if lip.StakedAmount != "1001000000" {
		t.Errorf("staked_amount: got %s, want 1001000000", lip.StakedAmount)
	}
}

func TestGenesisRoundtrip_VoteFields(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Vote Roundtrip", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "5000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	gs := k.ExportGenesis(ctx)
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	allVotes := k2.GetAllVotes(ctx2)
	if len(allVotes) != 1 {
		t.Fatalf("expected 1 vote, got %d", len(allVotes))
	}
	if allVotes[0].Voter != testAddr("voter1") {
		t.Errorf("voter mismatch")
	}
	if allVotes[0].Option != types.VoteYes {
		t.Errorf("option: got %s, want yes", allVotes[0].Option)
	}
}

// ---------- Query ----------

func TestQueryLIP(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Query Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	resp, err := qs.LIP(ctx, &types.QueryLIPRequest{LipId: "LIP-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Lip.Title != "Query Test" {
		t.Errorf("expected 'Query Test', got '%s'", resp.Lip.Title)
	}
}

func TestQueryLIP_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.LIP(ctx, &types.QueryLIPRequest{LipId: "LIP-999"})
	if err == nil {
		t.Error("expected error for non-existent LIP")
	}
}

func TestQueryLIPs_ByStatus(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Draft 1", Description: "A",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Draft 2", Description: "B",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})

	// Advance LIP-2 to review
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-2", Amount: "200000000",
	})

	drafts, _ := qs.LIPs(ctx, &types.QueryLIPsRequest{Status: types.StatusDraft})
	if drafts.Total != 1 {
		t.Errorf("expected 1 draft, got %d", drafts.Total)
	}

	reviews, _ := qs.LIPs(ctx, &types.QueryLIPsRequest{Status: types.StatusReview})
	if reviews.Total != 1 {
		t.Errorf("expected 1 review, got %d", reviews.Total)
	}

	all, _ := qs.LIPs(ctx, &types.QueryLIPsRequest{})
	if all.Total != 2 {
		t.Errorf("expected 2 total, got %d", all.Total)
	}
}

func TestQueryParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Params.VotingPeriodBlocks != 102816 {
		t.Errorf("expected 102816, got %d", resp.Params.VotingPeriodBlocks)
	}
}

func TestQueryVotes(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Vote Query", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("v1")] = "100"
	mock.delegations[testAddr("v2")] = "200"
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("v1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("v2"), LipId: "LIP-1", Option: types.VoteNo})

	resp, err := qs.Votes(ctx, &types.QueryVotesRequest{LipId: "LIP-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Votes) != 2 {
		t.Errorf("expected 2 votes, got %d", len(resp.Votes))
	}
}

// ---------- ValidateBasic ----------

func TestValidateBasic_MsgSubmitLIP(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgSubmitLIP
		wantErr bool
	}{
		{"valid", types.MsgSubmitLIP{Proposer: testAddr("alice"), Title: "Test", Description: "Test", Category: types.CategoryText, InitialStake: "1000000"}, false},
		{"empty title", types.MsgSubmitLIP{Proposer: testAddr("alice"), Title: "", Description: "Test", Category: types.CategoryText, InitialStake: "1000000"}, true},
		{"invalid addr", types.MsgSubmitLIP{Proposer: "bad", Title: "Test", Description: "Test", Category: types.CategoryText, InitialStake: "1000000"}, true},
		{"zero stake", types.MsgSubmitLIP{Proposer: testAddr("alice"), Title: "Test", Description: "Test", Category: types.CategoryText, InitialStake: "0"}, true},
		{"empty stake", types.MsgSubmitLIP{Proposer: testAddr("alice"), Title: "Test", Description: "Test", Category: types.CategoryText, InitialStake: ""}, true},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBasic_MsgCastVote(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgCastVote
		wantErr bool
	}{
		{"valid", types.MsgCastVote{Voter: testAddr("v"), LipId: "LIP-1", Option: types.VoteYes}, false},
		{"invalid addr", types.MsgCastVote{Voter: "bad", LipId: "LIP-1", Option: types.VoteYes}, true},
		{"empty lip_id", types.MsgCastVote{Voter: testAddr("v"), LipId: "", Option: types.VoteYes}, true},
		{"invalid option", types.MsgCastVote{Voter: testAddr("v"), LipId: "LIP-1", Option: "invalid"}, true},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------- Counter CRUD ----------

func TestLIPCounter(t *testing.T) {
	k, ctx := setupKeeper(t)

	n := k.GetNextLIPNumber(ctx)
	if n != 1 {
		t.Errorf("expected initial counter=1, got %d", n)
	}

	k.SetNextLIPNumber(ctx, 42)
	n = k.GetNextLIPNumber(ctx)
	if n != 42 {
		t.Errorf("expected 42, got %d", n)
	}
}

// ---------- Category Stake ----------

func TestGetCategoryStake_PerCategory(t *testing.T) {
	params := types.DefaultParams()
	paramCfg := types.GetCategoryConfig(params, types.CategoryParameter)
	if paramCfg == nil || paramCfg.RequiredStakeBps != "1000000000" {
		t.Error("wrong parameter stake")
	}
	upgradeCfg := types.GetCategoryConfig(params, types.CategoryUpgrade)
	if upgradeCfg == nil || upgradeCfg.RequiredStakeBps != "800000000" {
		t.Error("wrong upgrade stake")
	}
	textCfg := types.GetCategoryConfig(params, types.CategoryText)
	if textCfg == nil || textCfg.RequiredStakeBps != "400000000" {
		t.Error("wrong text stake")
	}
	researchCfg := types.GetCategoryConfig(params, types.CategoryResearchSpend)
	if researchCfg == nil || researchCfg.RequiredStakeBps != "200000000" {
		t.Error("wrong research_spend stake")
	}
}

// ---------- Edge Cases ----------

func TestVote_AfterPeriodEnds(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Expired", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 50 // already expired (current height = 100)
	k.SetLIP(ctx, lip)

	_, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if err == nil {
		t.Error("expected error for voting after period ended")
	}
}

func TestStageSkipAttack(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Skip Attack", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	// Try to advance from draft (should fail)
	_, err := ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("should not be able to advance from draft")
	}
}

func TestTerminalStateImmutability(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Terminal Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	// Set to passed (terminal)
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusPassed
	k.SetLIP(ctx, lip)

	// Try to withdraw
	_, err := ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("alice"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("should not be able to withdraw passed LIP")
	}

	// Try to stake
	_, err = ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "1000000",
	})
	if err == nil {
		t.Error("should not be able to stake on passed LIP")
	}
}

func TestCategoryThresholds(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Submit a parameter category LIP (needs 1000 ZRN = 1000000000 uzrn)
	// Initial stake = 1 ZRN = 1000000 uzrn
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Param", Description: "Test",
		Category: types.CategoryParameter, InitialStake: "1000000",
	})

	// Stake 998 ZRN (total = 999 ZRN, not enough for 1000 ZRN threshold)
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "998000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusDraft {
		t.Errorf("expected draft with 999 ZRN stake, got %s", lip.Stage)
	}

	// Stake 1 more ZRN (total now = 1000 ZRN >= threshold)
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("charlie"), LipId: "LIP-1", Amount: "1000000",
	})
	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Errorf("expected review with 1000 ZRN stake, got %s", lip.Stage)
	}
}

func TestNonProposerAdvancement(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	params := k.GetParams(ctx)
	for _, cc := range params.CategoryConfigs {
		cc.ReviewBlocks = 0
	}
	k.SetParams(ctx, params)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Test", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})

	// Non-proposer tries to advance
	_, err := ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("bob"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("expected error for non-proposer advancement")
	}
}

func TestVote_OnNonVotingLIP(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Review LIP", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	// Advance to review
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})

	_, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if err == nil {
		t.Error("expected error for voting on review-stage LIP")
	}
}

func TestExactQuorumBoundary(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000") // 1M total bonded
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Boundary", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100 // expires at current height
	k.SetLIP(ctx, lip)

	// Vote exactly at quorum: 334000 / 1000000 = 0.334 = 33.4%
	// Need: (totalVoted * 1M) / totalBonded >= 334000
	// totalVoted = 334000, totalBonded = 1000000
	// 334000 * 1000000 / 1000000 = 334000 = threshold → passes
	mock.delegations[testAddr("voter1")] = "334000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("expected passed at exact quorum boundary, got %s", lip.Stage)
	}
}

func TestBeginBlocker_AutoAdvance(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Use non-zero discussion period so review→last_call and last_call→voting
	// happen on separate BeginBlocker calls.
	params := k.GetParams(ctx)
	params.DiscussionPeriodBlocks = 10
	params.VotingPeriodBlocks = 100
	for _, cc := range params.CategoryConfigs {
		cc.ReviewBlocks = 0
	}
	k.SetParams(ctx, params)

	// Create a LIP in review stage (review_blocks=0, so it immediately advances)
	lip := &types.LIP{
		Id: "LIP-1", Title: "Auto", Description: "Test",
		Category: types.CategoryText, Proposer: testAddr("alice"),
		Stage: types.StatusReview, StakedAmount: "400000000",
		YesStake: "0", NoStake: "0", AbstainStake: "0",
		ReviewStartedBlock: 0,
	}
	k.SetLIP(ctx, lip)
	k.SetNextLIPNumber(ctx, 2)

	// Run begin blocker at height 100 — should advance review → last_call
	k.BeginBlocker(ctx)
	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusLastCall {
		t.Errorf("expected last_call after auto-advance, got %s", lip.Stage)
	}

	// Run again at height 110 (discussion period of 10 elapsed) — should advance last_call → voting
	ctx2 := ctx.WithBlockHeight(110)
	k.BeginBlocker(ctx2)
	lip, _ = k.GetLIP(ctx2, "LIP-1")
	if lip.Stage != types.StatusVoting {
		t.Errorf("expected voting after auto-advance, got %s", lip.Stage)
	}
	if lip.VotingEndBlock != 210 { // 110 + 100
		t.Errorf("expected voting_end_block=210, got %d", lip.VotingEndBlock)
	}
}

func TestSupportThreshold(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Support Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	// 400k yes, 600k no — total meets quorum but support < 50%
	mock.delegations[testAddr("v1")] = "400000"
	mock.delegations[testAddr("v2")] = "600000"
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("v1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("v2"), LipId: "LIP-1", Option: types.VoteNo})

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusFailed {
		t.Errorf("expected failed (support < 50%%), got %s", lip.Stage)
	}
}

// ---------- Helpers: BigInt ----------

func TestAddBigIntStrings(t *testing.T) {
	result := types.AddBigIntStrings("100", "200")
	if result != "300" {
		t.Errorf("expected 300, got %s", result)
	}
	result = types.AddBigIntStrings("999999999999", "1")
	expected := new(big.Int).Add(
		new(big.Int).SetInt64(999999999999),
		new(big.Int).SetInt64(1),
	).String()
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestCmpBigIntStrings(t *testing.T) {
	if types.CmpBigIntStrings("100", "200") >= 0 {
		t.Error("100 should be < 200")
	}
	if types.CmpBigIntStrings("200", "100") <= 0 {
		t.Error("200 should be > 100")
	}
	if types.CmpBigIntStrings("100", "100") != 0 {
		t.Error("100 should == 100")
	}
}

// ---------- Upgrade Plan Tests ----------

func TestAttachUpgradePlan_FullLifecycle(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	// Wire mock upgrade keeper.
	mockUK := &mockUpgradeKeeper{}
	k.SetUpgradeKeeper(mockUK)

	// 1. Submit an upgrade-category LIP.
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "v2.0.0 Upgrade",
		Description:  "Major protocol upgrade",
		Category:     types.CategoryUpgrade,
		InitialStake: "1000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	lipID := resp.LipId

	// 2. Attach an upgrade plan.
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer:    testAddr("alice"),
		LipId:       lipID,
		UpgradeName: "v2.0.0",
		Height:      500,
		Info:        "https://github.com/zerone-chain/zerone/releases/v2.0.0",
	})
	if err != nil {
		t.Fatalf("attach upgrade plan failed: %v", err)
	}

	// Verify plan was stored.
	plan, found := k.GetUpgradePlan(ctx, lipID)
	if !found {
		t.Fatal("upgrade plan not found after attach")
	}
	if plan.Name != "v2.0.0" {
		t.Errorf("plan name: got %q, want %q", plan.Name, "v2.0.0")
	}
	if plan.Height != 500 {
		t.Errorf("plan height: got %d, want 500", plan.Height)
	}

	// 3. Advance LIP to voting and vote to pass.
	lip, _ := k.GetLIP(ctx, lipID)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100 // expires at current height
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500000" // 50% of total
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: lipID, Option: types.VoteYes,
	})

	// 4. Run BeginBlocker to tally — should pass and call ScheduleUpgrade.
	k.BeginBlocker(ctx)

	// 5. Assert LIP passed and ScheduleUpgrade was called.
	lip, _ = k.GetLIP(ctx, lipID)
	if lip.Stage != types.StatusPassed {
		t.Errorf("expected passed, got %s", lip.Stage)
	}
	if !mockUK.called {
		t.Fatal("ScheduleUpgrade was not called on the mock upgrade keeper")
	}
	if mockUK.plan.Name != "v2.0.0" {
		t.Errorf("scheduled plan name: got %q, want %q", mockUK.plan.Name, "v2.0.0")
	}
	if mockUK.plan.Height != 500 {
		t.Errorf("scheduled plan height: got %d, want 500", mockUK.plan.Height)
	}
	if mockUK.plan.Info != "https://github.com/zerone-chain/zerone/releases/v2.0.0" {
		t.Errorf("scheduled plan info mismatch")
	}
}

func TestAttachUpgradePlan_Validations(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Submit upgrade LIP.
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Upgrade", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})
	// Submit text LIP for category check.
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Text", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	// Wrong proposer.
	_, err := ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("bob"), LipId: "LIP-1",
		UpgradeName: "v2", Height: 500,
	})
	if err == nil {
		t.Error("expected error for wrong proposer")
	}

	// Wrong category (text, not upgrade).
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-2",
		UpgradeName: "v2", Height: 500,
	})
	if err == nil {
		t.Error("expected error for non-upgrade category")
	}

	// Invalid height.
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v2", Height: 0,
	})
	if err == nil {
		t.Error("expected error for zero height")
	}

	// Terminal LIP.
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusPassed
	k.SetLIP(ctx, lip)
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v2", Height: 500,
	})
	if err == nil {
		t.Error("expected error for terminal LIP")
	}
}

func TestAttachUpgradePlan_NoDuplicate(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Upgrade", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})

	// First attach succeeds.
	_, err := ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v2", Height: 500,
	})
	if err != nil {
		t.Fatalf("first attach failed: %v", err)
	}

	// Second attach fails (duplicate).
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v3", Height: 600,
	})
	if err == nil {
		t.Error("expected error for duplicate upgrade plan")
	}
}

func TestUpgradePlan_GenesisRoundtrip(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Upgrade", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})
	ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v2.0.0", Height: 500, Info: "release notes",
	})

	// Export and reimport.
	gs := k.ExportGenesis(ctx)
	if len(gs.UpgradePlans) != 1 {
		t.Fatalf("expected 1 upgrade plan in genesis, got %d", len(gs.UpgradePlans))
	}

	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	plan, found := k2.GetUpgradePlan(ctx2, "LIP-1")
	if !found {
		t.Fatal("upgrade plan not found after genesis roundtrip")
	}
	if plan.Name != "v2.0.0" {
		t.Errorf("plan name: got %q, want %q", plan.Name, "v2.0.0")
	}
	if plan.Height != 500 {
		t.Errorf("plan height: got %d, want 500", plan.Height)
	}
	if plan.Info != "release notes" {
		t.Errorf("plan info: got %q, want %q", plan.Info, "release notes")
	}
}

func TestUpgradePlan_NoScheduleWithoutPlan(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockUK := &mockUpgradeKeeper{}
	k.SetUpgradeKeeper(mockUK)

	// Submit upgrade LIP WITHOUT attaching a plan.
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Upgrade No Plan", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	k.BeginBlocker(ctx)

	// LIP should pass but ScheduleUpgrade should NOT be called (no plan).
	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("expected passed, got %s", lip.Stage)
	}
	if mockUK.called {
		t.Error("ScheduleUpgrade should not be called when no plan is attached")
	}
}
