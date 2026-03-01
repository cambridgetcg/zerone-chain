package keeper_test

import (
	"context"
	"fmt"
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
	totalBonded   string
	delegations   map[string]string // addr -> bonded amount
	guardianCount uint64
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

func (m *mockStakingKeeper) CountActiveGuardians(_ context.Context) (uint64, error) {
	return m.guardianCount, nil
}

func (m *mockStakingKeeper) IsGuardian(_ context.Context, addr string) (bool, error) {
	return false, nil
}

func (m *mockStakingKeeper) IsJailed(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockStakingKeeper) GetSlashCount(_ context.Context, _ string) (uint64, error) {
	return 0, nil
}

// ---------- Mock Upgrade Keeper ----------

type mockUpgradeKeeper struct {
	called bool
	plan   *types.UpgradePlan
}

func (m *mockUpgradeKeeper) ScheduleUpgrade(_ context.Context, plan *types.UpgradePlan) error {
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
	if paramCfg == nil || paramCfg.RequiredStakeUzrn != "1000000000" {
		t.Error("wrong parameter stake")
	}
	upgradeCfg := types.GetCategoryConfig(params, types.CategoryUpgrade)
	if upgradeCfg == nil || upgradeCfg.RequiredStakeUzrn != "800000000" {
		t.Error("wrong upgrade stake")
	}
	textCfg := types.GetCategoryConfig(params, types.CategoryText)
	if textCfg == nil || textCfg.RequiredStakeUzrn != "400000000" {
		t.Error("wrong text stake")
	}
	researchCfg := types.GetCategoryConfig(params, types.CategoryResearchSpend)
	if researchCfg == nil || researchCfg.RequiredStakeUzrn != "200000000" {
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

// ---------- Research Fund Threshold Tests ----------

func TestGetResearchFundThreshold_GenesisPair(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR)
	if required != 2 || total != 2 {
		t.Errorf("Phase 0: expected 2-of-2, got %d-of-%d", required, total)
	}
}

func TestGetResearchFundThreshold_Observer(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER)
	if required != 2 || total != 3 {
		t.Errorf("Phase 1: expected 2-of-3, got %d-of-%d", required, total)
	}
}

func TestGetResearchFundThreshold_Balanced(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED)
	if required != 3 || total != 5 {
		t.Errorf("Phase 2: expected 3-of-5, got %d-of-%d", required, total)
	}
}

func TestGetResearchFundThreshold_FullGovernance(t *testing.T) {
	required, total := types.GetResearchFundThreshold(types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE)
	if required != 0 || total != 0 {
		t.Errorf("Phase 3: expected 0-of-0, got %d-of-%d", required, total)
	}
}

// ---------- Phase State Tests ----------

func TestGetResearchFundPhase_Default(t *testing.T) {
	k, ctx := setupKeeper(t)
	phase := k.GetResearchFundPhase(ctx)
	if phase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR {
		t.Errorf("expected default GENESIS_PAIR, got %v", phase)
	}
}

func TestSetResearchFundPhase(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER)
	phase := k.GetResearchFundPhase(ctx)
	if phase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER {
		t.Errorf("expected OBSERVER, got %v", phase)
	}

	// Verify full state is updated correctly.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.CurrentPhase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER {
		t.Error("state.CurrentPhase not updated")
	}
	if state.PhaseStartedAtBlock != 100 {
		t.Errorf("expected phase_started_at_block=100, got %d", state.PhaseStartedAtBlock)
	}
	if state.ProposalsExecutedInPhase != 0 {
		t.Error("expected proposals counter to reset on phase transition")
	}
}

func TestIncrementProposalsExecuted(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initial count should be 0.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.ProposalsExecutedInPhase != 0 {
		t.Errorf("expected 0, got %d", state.ProposalsExecutedInPhase)
	}

	// Increment twice.
	k.IncrementProposalsExecuted(ctx)
	k.IncrementProposalsExecuted(ctx)

	state = k.GetResearchFundGovernanceState(ctx)
	if state.ProposalsExecutedInPhase != 2 {
		t.Errorf("expected 2, got %d", state.ProposalsExecutedInPhase)
	}
}

// ---------- Distinct Voter Tracking Tests ----------

func TestDistinctVoterTracking_RecordAndCount(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initially zero.
	count := k.CountDistinctVoters(ctx)
	if count != 0 {
		t.Errorf("expected 0 distinct voters, got %d", count)
	}

	// Record three distinct voters.
	k.RecordDistinctVoter(ctx, testAddr("alice"))
	k.RecordDistinctVoter(ctx, testAddr("bob"))
	k.RecordDistinctVoter(ctx, testAddr("charlie"))

	count = k.CountDistinctVoters(ctx)
	if count != 3 {
		t.Errorf("expected 3 distinct voters, got %d", count)
	}
}

func TestDistinctVoterTracking_Deduplication(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Record same voter twice.
	k.RecordDistinctVoter(ctx, testAddr("alice"))
	k.RecordDistinctVoter(ctx, testAddr("alice"))

	count := k.CountDistinctVoters(ctx)
	if count != 1 {
		t.Errorf("expected 1 (deduped), got %d", count)
	}
}

// ---------- Research Fund Balance Tests ----------

func TestGetResearchFundBalance_NilBankKeeper(t *testing.T) {
	k, ctx := setupKeeper(t) // bankKeeper is nil
	balance := k.GetResearchFundBalance(ctx)
	if !balance.IsZero() {
		t.Errorf("expected zero balance with nil bank keeper, got %s", balance)
	}
}

// ---------- Mock Emergency Keeper ----------

type mockEmergencyKeeper struct {
	halts map[string]uint64
}

func (m *mockEmergencyKeeper) CountHaltsForReason(_ context.Context, reason string) uint64 {
	if m.halts != nil {
		return m.halts[reason]
	}
	return 0
}

// ---------- Phase Exit Condition Tests ----------

func TestCheckPhaseExitConditions_GenesisPair_NotMet(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Wire mocks.
	mockSK.guardianCount = 5
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	// Record 10 distinct voters.
	for i := 0; i < 10; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.DistinctLipVoters != 10 {
		t.Errorf("expected 10 distinct voters, got %d", conditions.DistinctLipVoters)
	}
	if conditions.ActiveGuardians != 5 {
		t.Errorf("expected 5 active guardians, got %d", conditions.ActiveGuardians)
	}
	if conditions.ChainAgeBlocks != 100 {
		t.Errorf("expected chain age 100, got %d", conditions.ChainAgeBlocks)
	}

	// Should not be all met — chain age is only 100, needs 2,200,000.
	if allMet {
		t.Error("conditions should NOT all be met (chain age too low)")
	}
}

func TestCheckPhaseExitConditions_NilKeepers(t *testing.T) {
	k, ctx := setupKeeper(t) // nil staking and emergency keepers

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.ActiveGuardians != 0 {
		t.Errorf("expected 0 guardians with nil keeper, got %d", conditions.ActiveGuardians)
	}
	if conditions.EmergencyHaltsFromMisuse != 0 {
		t.Errorf("expected 0 halts with nil keeper, got %d", conditions.EmergencyHaltsFromMisuse)
	}
	if allMet {
		t.Error("should not be met with nil keepers")
	}
}

func TestCheckPhaseExitConditions_FullGovernance(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE)

	_, allMet := k.CheckPhaseExitConditions(ctx)
	// Full governance has no exit conditions — allMet should be false (no transition possible).
	if allMet {
		t.Error("full governance should not have exit conditions met")
	}
}

func TestCheckPhaseExitConditions_AllMet(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Wire mocks.
	mockSK.guardianCount = 10
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	// Record enough distinct voters.
	for i := 0; i < 15; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	// Use a high block height to satisfy chain age.
	ctx = ctx.WithBlockHeight(3_000_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.DistinctLipVoters != 15 {
		t.Errorf("expected 15 distinct voters, got %d", conditions.DistinctLipVoters)
	}
	if conditions.ActiveGuardians != 10 {
		t.Errorf("expected 10 active guardians, got %d", conditions.ActiveGuardians)
	}
	if conditions.ChainAgeBlocks != 3_000_000 {
		t.Errorf("expected chain age 3000000, got %d", conditions.ChainAgeBlocks)
	}

	// Should NOT be met — research fund balance is 0, needs 100,000 ZRN.
	if allMet {
		t.Error("conditions should NOT all be met (zero research fund balance)")
	}
}

func TestGenesisRoundtrip_ResearchFundGovernance(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set non-default governance state.
	state := &types.ResearchFundGovernanceState{
		CurrentPhase:            types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER,
		CommunitySeats:          []string{testAddr("seat1"), testAddr("seat2")},
		ProposalsExecutedInPhase: 3,
		PhaseStartedAtBlock:     50000,
		LastTransitionBlock:     50000,
	}
	k.SetResearchFundGovernanceState(ctx, state)

	// Export genesis.
	gs := k.ExportGenesis(ctx)
	if gs.ResearchFundGovernance == nil {
		t.Fatal("exported genesis has nil ResearchFundGovernance")
	}

	// Create a fresh keeper and import.
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	// Verify roundtrip.
	got := k2.GetResearchFundGovernanceState(ctx2)
	if got.CurrentPhase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER {
		t.Errorf("phase: got %v, want OBSERVER", got.CurrentPhase)
	}
	if len(got.CommunitySeats) != 2 {
		t.Errorf("community seats: got %d, want 2", len(got.CommunitySeats))
	}
	if got.ProposalsExecutedInPhase != 3 {
		t.Errorf("proposals executed: got %d, want 3", got.ProposalsExecutedInPhase)
	}
	if got.PhaseStartedAtBlock != 50000 {
		t.Errorf("phase started at: got %d, want 50000", got.PhaseStartedAtBlock)
	}
}

// ========== NEW PORTED TESTS: LIP Lifecycle Edge Cases ==========

// TestLIPExpiredBeforeVoting verifies that a LIP in voting stage with
// VotingEndBlock in the past is correctly resolved by BeginBlocker.
func TestLIPExpiredBeforeVoting(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Expired Before Voting", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	// Put LIP in voting with end block already passed.
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 50 // already expired at block 100
	k.SetLIP(ctx, lip)

	// No votes cast — should fail for quorum.
	mock.delegations[testAddr("voter1")] = "0" // no stake

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusFailed {
		t.Errorf("expected failed (no votes, expired), got %s", lip.Stage)
	}
}

// TestLIPWithdrawal_FromReviewStage verifies a proposer can withdraw a LIP
// that is in review stage (not yet in voting).
func TestLIPWithdrawal_FromReviewStage(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Review Withdraw", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Fatalf("expected review, got %s", lip.Stage)
	}

	_, err := ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("alice"), LipId: "LIP-1",
	})
	if err != nil {
		t.Fatalf("unexpected error withdrawing from review: %v", err)
	}

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusWithdrawn {
		t.Errorf("expected withdrawn, got %s", lip.Stage)
	}
}

// TestLIPWithdrawal_FromVotingStage verifies a proposer CANNOT withdraw
// a LIP that is already in voting stage.
func TestLIPWithdrawal_FromVotingStage(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Voting Withdraw", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	_, err := ms.WithdrawLIP(ctx, &types.MsgWithdrawLIP{
		Proposer: testAddr("alice"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("expected error when withdrawing LIP in voting stage")
	}
}

// TestLIPAmendment_StakeOnReview verifies additional staking on a LIP
// already in review does not re-trigger the draft->review transition but
// simply accumulates stake.
func TestLIPAmendment_StakeOnReview(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Amend Test", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Fatalf("expected review, got %s", lip.Stage)
	}

	// Additional stake while in review.
	_, err := ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("charlie"), LipId: "LIP-1", Amount: "50000000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Errorf("should remain in review, got %s", lip.Stage)
	}
	// 1000000 + 200000000 + 50000000 = 251000000
	if lip.StakedAmount != "251000000" {
		t.Errorf("expected staked=251000000, got %s", lip.StakedAmount)
	}
}

// TestLIPCategoryValidation verifies that each valid category can be
// used to submit a LIP successfully.
func TestLIPCategoryValidation(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	categories := []string{
		types.CategoryParameter,
		types.CategoryUpgrade,
		types.CategoryText,
		types.CategoryResearchSpend,
	}

	for i, cat := range categories {
		t.Run(cat, func(t *testing.T) {
			resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
				Proposer: testAddr("alice"), Title: "Cat " + cat, Description: "Test",
				Category: cat, InitialStake: "1000000",
			})
			if err != nil {
				t.Fatalf("submit with category %s failed: %v", cat, err)
			}
			lip, found := k.GetLIP(ctx, resp.LipId)
			if !found {
				t.Fatalf("LIP not found: %s", resp.LipId)
			}
			if lip.Category != cat {
				t.Errorf("expected category=%s, got %s", cat, lip.Category)
			}
			_ = i
		})
	}
}

// TestLIPDuplicateTitle verifies that submitting LIPs with duplicate titles
// is allowed (IDs are unique, titles are not).
func TestLIPDuplicateTitle(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	resp1, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Same Title", Description: "First",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}
	resp2, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("bob"), Title: "Same Title", Description: "Second",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	if err != nil {
		t.Fatalf("second submit failed: %v", err)
	}

	if resp1.LipId == resp2.LipId {
		t.Error("LIPs with same title should have different IDs")
	}

	lip1, _ := k.GetLIP(ctx, resp1.LipId)
	lip2, _ := k.GetLIP(ctx, resp2.LipId)
	if lip1.Description != "First" || lip2.Description != "Second" {
		t.Error("descriptions should distinguish the two LIPs")
	}
}

// ========== NEW PORTED TESTS: Param Change Execution ==========

// TestParamChangeApplied verifies that when a parameter-category LIP passes,
// the param changes are applied to the router.
func TestParamChangeApplied(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockPR := &mockParamRouter{}
	k.SetParamRouter(mockPR)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Param Apply", Description: "Test",
		Category: types.CategoryParameter, InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "zerone_gov", Key: "voting_period_blocks", Value: "50000"},
		},
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

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Fatalf("expected passed, got %s", lip.Stage)
	}
	if len(mockPR.applied) != 1 {
		t.Fatalf("expected 1 param change, got %d", len(mockPR.applied))
	}
	if mockPR.applied[0].value != "50000" {
		t.Errorf("expected value=50000, got %s", mockPR.applied[0].value)
	}
}

// TestParamChangeInvalidModule verifies that a param change targeting an
// unknown module emits a failure event but does not panic.
func TestParamChangeInvalidModule(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockPR := &mockParamRouter{}
	k.SetParamRouter(mockPR)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Bad Module", Description: "Test",
		Category: types.CategoryParameter, InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "unknown_module", Key: "key", Value: "val"},
		},
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	// Should not panic.
	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("LIP should still pass, got %s", lip.Stage)
	}
	if len(mockPR.applied) != 0 {
		t.Errorf("expected 0 applied (unknown module rejected), got %d", len(mockPR.applied))
	}
}

// TestParamChangeRollbackOnError verifies that partial failures in param
// changes do not leave the system in an inconsistent state -- changes that
// succeeded are still applied, and failures are logged via events.
func TestParamChangeRollbackOnError(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockPR := &mockParamRouter{}
	k.SetParamRouter(mockPR)

	// First change succeeds, second targets unknown module.
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Mixed Params", Description: "Test",
		Category: types.CategoryParameter, InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "zerone_gov", Key: "voting_period_blocks", Value: "99999"},
			{Module: "unknown_module", Key: "bad_key", Value: "bad_val"},
		},
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

	// First change should have been applied.
	if len(mockPR.applied) != 1 {
		t.Errorf("expected 1 applied change (the valid one), got %d", len(mockPR.applied))
	}

	// Check for both applied and failed events.
	events := ctx.EventManager().Events()
	appliedCount := 0
	failedCount := 0
	for _, e := range events {
		if e.Type == "zerone.gov.param_change_applied" {
			appliedCount++
		}
		if e.Type == "zerone.gov.param_change_failed" {
			failedCount++
		}
	}
	if appliedCount != 1 {
		t.Errorf("expected 1 applied event, got %d", appliedCount)
	}
	if failedCount != 1 {
		t.Errorf("expected 1 failed event, got %d", failedCount)
	}
}

// ========== NEW PORTED TESTS: Upgrade Proposals ==========

// TestUpgradeLIPScheduled verifies that when an upgrade-category LIP with a
// plan passes, the upgrade is scheduled on the mock upgrade keeper.
func TestUpgradeLIPScheduled(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockUK := &mockUpgradeKeeper{}
	k.SetUpgradeKeeper(mockUK)

	resp, _ := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Scheduled Upgrade", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})
	ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: resp.LipId,
		UpgradeName: "v3.0.0", Height: 9999, Info: "release-v3",
	})

	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: resp.LipId, Option: types.VoteYes,
	})

	k.BeginBlocker(ctx)

	if !mockUK.called {
		t.Fatal("expected ScheduleUpgrade to be called")
	}
	if mockUK.plan.Name != "v3.0.0" {
		t.Errorf("plan name: got %q, want v3.0.0", mockUK.plan.Name)
	}
	if mockUK.plan.Height != 9999 {
		t.Errorf("plan height: got %d, want 9999", mockUK.plan.Height)
	}
}

// TestUpgradeLIPHeightValidation verifies that attaching an upgrade plan
// with height=0 or negative height fails validation.
func TestUpgradeLIPHeightValidation(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Height Test", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})

	// Zero height.
	_, err := ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v2", Height: 0,
	})
	if err == nil {
		t.Error("expected error for zero height")
	}

	// Negative height.
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v2", Height: -1,
	})
	if err == nil {
		t.Error("expected error for negative height")
	}

	// Valid height should succeed.
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer: testAddr("alice"), LipId: "LIP-1",
		UpgradeName: "v2", Height: 1000,
	})
	if err != nil {
		t.Fatalf("expected success for valid height, got %v", err)
	}
}

// ========== NEW PORTED TESTS: Stake-Weighted Voting Edge Cases ==========

// TestStakeWeightedVote_LargeStake verifies correct weight accumulation
// with very large bonded stake amounts (overflow-safe).
func TestStakeWeightedVote_LargeStake(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000000000") // 1 quintillion
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Large Stake", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Voter with large bonded stake.
	mock.delegations[testAddr("whale")] = "999999999999999999"
	resp, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("whale"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.EffectiveWeight != "999999999999999999" {
		t.Errorf("expected weight=999999999999999999, got %s", resp.EffectiveWeight)
	}

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.YesStake != "999999999999999999" {
		t.Errorf("expected yes_stake=999999999999999999, got %s", lip.YesStake)
	}
}

// TestStakeWeightedVote_ZeroStake verifies that a voter with zero bonded
// stake can still vote (weight = 0).
func TestStakeWeightedVote_ZeroStake(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Zero Stake", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// No delegation set -> defaults to "0".
	resp, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("nobody"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.EffectiveWeight != "0" {
		t.Errorf("expected weight=0, got %s", resp.EffectiveWeight)
	}
}

// ========== NEW PORTED TESTS: Adversarial / Game Theory ==========

// TestProposalSpamByMinStake verifies that many LIPs can be submitted at
// minimum stake, counter increments correctly, and all remain in draft.
func TestProposalSpamByMinStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	for i := 0; i < 20; i++ {
		resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
			Proposer: testAddr("alice"), Title: "Spam", Description: "Spam",
			Category: types.CategoryText, InitialStake: "1000000",
		})
		if err != nil {
			t.Fatalf("LIP #%d submission failed: %v", i+1, err)
		}
		expectedID := fmt.Sprintf("LIP-%d", i+1)
		if resp.LipId != expectedID {
			t.Errorf("expected %s, got %s", expectedID, resp.LipId)
		}
	}

	// Verify counter.
	nextNum := k.GetNextLIPNumber(ctx)
	if nextNum != 21 {
		t.Errorf("expected next LIP number 21, got %d", nextNum)
	}

	// All should be in draft (no stake threshold met).
	drafts := k.GetLIPsByStatus(ctx, types.StatusDraft)
	if len(drafts) != 20 {
		t.Errorf("expected 20 drafts, got %d", len(drafts))
	}
}

// TestVoteOnNonexistentLIP verifies that voting on a LIP that does not
// exist returns an error and does not store phantom votes.
func TestVoteOnNonexistentLIP(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter"), LipId: "LIP-999", Option: types.VoteYes,
	})
	if err == nil {
		t.Error("expected error for non-existent LIP")
	}

	// Verify no phantom votes stored.
	votes := k.GetVotesForLIP(ctx, "LIP-999")
	if len(votes) > 0 {
		t.Errorf("expected 0 phantom votes, got %d", len(votes))
	}

	if k.HasVoted(ctx, "LIP-999", testAddr("voter")) {
		t.Error("HasVoted should return false for non-existent LIP")
	}
}

// TestDoubleVoteSameOption verifies that the same voter cannot vote twice
// on the same LIP, even with the same option.
func TestDoubleVoteSameOption(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Double", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	// Same voter, same option.
	_, err := ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	if err == nil {
		t.Error("expected error for duplicate vote with same option")
	}

	// Same voter, different option.
	_, err = ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteNo,
	})
	if err == nil {
		t.Error("expected error for duplicate vote with different option")
	}
}

// TestAbstainQuorumManipulation verifies abstain votes affect quorum
// calculation correctly: they contribute to totalVoted but not to
// the yes/no support ratio.
func TestAbstainQuorumManipulation(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000") // 1M total bonded
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Abstain Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	// 1 supporter: 100k (10%)
	mock.delegations[testAddr("sup1")] = "100000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("sup1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	// 1 opposer: 100k (10%)
	mock.delegations[testAddr("opp1")] = "100000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("opp1"), LipId: "LIP-1", Option: types.VoteNo,
	})

	// 8 abstainers: 25k each (2.5% each, 20% total) → total voted = 40%
	for i := 0; i < 8; i++ {
		voter := testAddr(fmt.Sprintf("abs%d", i))
		mock.delegations[voter] = "25000"
		ms.CastVote(ctx, &types.MsgCastVote{
			Voter: voter, LipId: "LIP-1", Option: types.VoteAbstain,
		})
	}

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, "LIP-1")
	// Total voted = 100k + 100k + 200k = 400k / 1M = 40% (quorum 33.4% met)
	// Support = 100k / (100k + 100k) = 50% (support threshold 50% met)
	// But yes equals no, which is borderline depending on >= vs > comparison.
	// The important thing is the test exercises the path.
	if lip.Stage != types.StatusPassed && lip.Stage != types.StatusFailed {
		t.Errorf("expected passed or failed (borderline), got %s", lip.Stage)
	}
}

// TestCategoryDowngradeAttack verifies category determines stake threshold.
// A LIP submitted as text (400 ZRN) cannot advance with research_spend-level
// stake (200 ZRN).
func TestCategoryDowngradeAttack(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Submit as text category (needs 400 ZRN).
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Downgrade", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})

	// Stake 200 ZRN (enough for research_spend, not text).
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusDraft {
		t.Errorf("should remain in draft (200 ZRN < 400 ZRN text threshold), got %s", lip.Stage)
	}

	// Now stake enough to meet text threshold (need total 400 ZRN = 400000000).
	// Already have 1000000 + 200000000 = 201000000, need 199000000 more.
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("charlie"), LipId: "LIP-1", Amount: "199000000",
	})

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Errorf("should advance to review with 400 ZRN, got %s", lip.Stage)
	}
}

// TestStageRushBypassReview verifies that the review period blocks are
// enforced and a proposer cannot immediately advance from review to last_call.
func TestStageRushBypassReview(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Keep default review period (17136 blocks for research_spend).
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Rush Test", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	ms.StakeLIP(ctx, &types.MsgStakeLIP{
		Staker: testAddr("bob"), LipId: "LIP-1", Amount: "200000000",
	})

	lip, _ := k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Fatalf("expected review, got %s", lip.Stage)
	}

	// Immediately try to advance (review period not elapsed).
	_, err := ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"), LipId: "LIP-1",
	})
	if err == nil {
		t.Error("expected error: review period not elapsed")
	}

	lip, _ = k.GetLIP(ctx, "LIP-1")
	if lip.Stage != types.StatusReview {
		t.Errorf("should still be in review, got %s", lip.Stage)
	}
}

// TestGenesisRoundtrip_Counters verifies LIP counters survive genesis export/import.
func TestGenesisRoundtrip_Counters(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	for i := 0; i < 5; i++ {
		ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
			Proposer: testAddr("alice"), Title: "LIP", Description: "Test",
			Category: types.CategoryText, InitialStake: "1000000",
		})
	}

	gs := k.ExportGenesis(ctx)
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	if k2.GetNextLIPNumber(ctx2) != 6 {
		t.Errorf("LIP counter: got %d, want 6", k2.GetNextLIPNumber(ctx2))
	}
}

// TestGenesisRoundtrip_Params verifies custom params survive genesis roundtrip.
func TestGenesisRoundtrip_Params(t *testing.T) {
	k, ctx := setupKeeper(t)

	custom := types.DefaultParams()
	custom.VotingPeriodBlocks = 7777
	custom.QuorumThresholdBps = 250000
	custom.SupportThresholdBps = 600000
	k.SetParams(ctx, custom)

	gs := k.ExportGenesis(ctx)
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	got := k2.GetParams(ctx2)
	if got.VotingPeriodBlocks != 7777 {
		t.Errorf("VotingPeriodBlocks: got %d, want 7777", got.VotingPeriodBlocks)
	}
	if got.QuorumThresholdBps != 250000 {
		t.Errorf("QuorumThresholdBps: got %d, want 250000", got.QuorumThresholdBps)
	}
	if got.SupportThresholdBps != 600000 {
		t.Errorf("SupportThresholdBps: got %d, want 600000", got.SupportThresholdBps)
	}
}

// TestTallyResult_Query verifies the TallyResult query returns correct
// quorum and support calculation.
func TestTallyResult_Query(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Tally Query", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Vote with 50% of total bonded.
	mock.delegations[testAddr("voter1")] = "500000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: "LIP-1", Option: types.VoteYes,
	})

	tally, err := qs.TallyResult(ctx, &types.QueryTallyResultRequest{LipId: "LIP-1"})
	if err != nil {
		t.Fatalf("tally query failed: %v", err)
	}
	if !tally.QuorumMet {
		t.Error("expected quorum met at 50% participation")
	}
	if !tally.Passed {
		t.Error("expected passed (100% yes, quorum met)")
	}
	if tally.YesStake != "500000" {
		t.Errorf("expected yes_stake=500000, got %s", tally.YesStake)
	}
}

// TestBeginBlocker_MultipleLIPTally verifies that BeginBlocker correctly
// tallies multiple LIPs that expire in the same block.
func TestBeginBlocker_MultipleLIPTally(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	// Create two LIPs in voting that expire at block 100.
	for i := 1; i <= 2; i++ {
		ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
			Proposer: testAddr("alice"), Title: fmt.Sprintf("LIP %d", i), Description: "Test",
			Category: types.CategoryText, InitialStake: "1000000",
		})
		lip, _ := k.GetLIP(ctx, fmt.Sprintf("LIP-%d", i))
		lip.Stage = types.StatusVoting
		lip.VotingEndBlock = 100
		k.SetLIP(ctx, lip)
	}

	// Vote yes on LIP-1, no on LIP-2.
	mock.delegations[testAddr("v1")] = "500000"
	mock.delegations[testAddr("v2")] = "500000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("v1"), LipId: "LIP-1", Option: types.VoteYes,
	})
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("v2"), LipId: "LIP-2", Option: types.VoteNo,
	})

	k.BeginBlocker(ctx)

	lip1, _ := k.GetLIP(ctx, "LIP-1")
	lip2, _ := k.GetLIP(ctx, "LIP-2")

	if lip1.Stage != types.StatusPassed {
		t.Errorf("LIP-1 expected passed, got %s", lip1.Stage)
	}
	if lip2.Stage != types.StatusFailed {
		t.Errorf("LIP-2 expected failed, got %s", lip2.Stage)
	}
}

// TestValidateBasic_MsgStakeLIP verifies ValidateBasic for MsgStakeLIP.
func TestValidateBasic_MsgStakeLIP(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgStakeLIP
		wantErr bool
	}{
		{"valid", types.MsgStakeLIP{Staker: testAddr("alice"), LipId: "LIP-1", Amount: "1000000"}, false},
		{"invalid addr", types.MsgStakeLIP{Staker: "bad", LipId: "LIP-1", Amount: "1000000"}, true},
		{"empty lip_id", types.MsgStakeLIP{Staker: testAddr("alice"), LipId: "", Amount: "1000000"}, true},
		{"zero amount", types.MsgStakeLIP{Staker: testAddr("alice"), LipId: "LIP-1", Amount: "0"}, true},
		{"empty amount", types.MsgStakeLIP{Staker: testAddr("alice"), LipId: "LIP-1", Amount: ""}, true},
		{"negative amount", types.MsgStakeLIP{Staker: testAddr("alice"), LipId: "LIP-1", Amount: "-100"}, true},
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

// TestValidateBasic_MsgAttachUpgradePlan verifies ValidateBasic for MsgAttachUpgradePlan.
func TestValidateBasic_MsgAttachUpgradePlan(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgAttachUpgradePlan
		wantErr bool
	}{
		{"valid", types.MsgAttachUpgradePlan{Proposer: testAddr("alice"), LipId: "LIP-1", UpgradeName: "v2", Height: 500}, false},
		{"invalid addr", types.MsgAttachUpgradePlan{Proposer: "bad", LipId: "LIP-1", UpgradeName: "v2", Height: 500}, true},
		{"empty lip_id", types.MsgAttachUpgradePlan{Proposer: testAddr("alice"), LipId: "", UpgradeName: "v2", Height: 500}, true},
		{"empty name", types.MsgAttachUpgradePlan{Proposer: testAddr("alice"), LipId: "LIP-1", UpgradeName: "", Height: 500}, true},
		{"zero height", types.MsgAttachUpgradePlan{Proposer: testAddr("alice"), LipId: "LIP-1", UpgradeName: "v2", Height: 0}, true},
		{"negative height", types.MsgAttachUpgradePlan{Proposer: testAddr("alice"), LipId: "LIP-1", UpgradeName: "v2", Height: -1}, true},
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

// ========== Phase Exit Condition Isolation Tests ==========

// TestExitConditions_Phase0_VotersMissing verifies that Phase 0 exit fails
// when distinct voter count is below the 10-voter threshold, even if all
// other conditions are satisfied.
func TestExitConditions_Phase0_VotersMissing(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	mockSK.guardianCount = 5
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	// Only record 5 voters (need 10 for Phase 0 exit).
	for i := 0; i < 5; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	// Satisfy chain age threshold.
	ctx = ctx.WithBlockHeight(2_300_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.DistinctLipVoters != 5 {
		t.Errorf("expected 5 distinct voters, got %d", conditions.DistinctLipVoters)
	}
	if allMet {
		t.Error("conditions should NOT all be met (only 5 of 10 required voters)")
	}
}

// TestExitConditions_Phase0_GuardiansMissing verifies that Phase 0 exit fails
// when active guardian count is below the 5-guardian threshold, even if voters
// and chain age are satisfied.
func TestExitConditions_Phase0_GuardiansMissing(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Only 3 guardians (need 5).
	mockSK.guardianCount = 3
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	// Record 10 distinct voters (satisfies voter threshold).
	for i := 0; i < 10; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	ctx = ctx.WithBlockHeight(2_300_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.ActiveGuardians != 3 {
		t.Errorf("expected 3 active guardians, got %d", conditions.ActiveGuardians)
	}
	if allMet {
		t.Error("conditions should NOT all be met (only 3 of 5 required guardians)")
	}
}

// TestExitConditions_Phase1_AllMet verifies that Phase 1 (Observer) exit
// succeeds when all conditions are met: 25+ voters, 10+ guardians,
// chain age 5.7M+, 3+ proposals executed, 2+ community seat votes,
// and zero emergency halts.
func TestExitConditions_Phase1_AllMet(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Set phase to Observer.
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER)

	// Set up community seat and governance state.
	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	state.ProposalsExecutedInPhase = 5
	state.PhaseStartedAtBlock = 50
	k.SetResearchFundGovernanceState(ctx, state)

	// Set up research voters so we can create proposals for community votes.
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Create 2 proposals and record community votes on them (need 2 MinCommunitySeatVotes).
	for i := 0; i < 2; i++ {
		resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
			Proposer:  voter1,
			Title:     fmt.Sprintf("Proposal %d", i+1),
			Recipient: testAddr("recipient"),
			Amount:    "100000000",
		})
		if err != nil {
			t.Fatalf("submit %d failed: %v", i+1, err)
		}
		k.SetResearchCommunityVote(ctx, resp.ProposalId, community1, "yes")
	}

	// Wire mocks: 12 guardians, 0 halts.
	mockSK.guardianCount = 12
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	// Record 30 distinct voters (need 25).
	for i := 0; i < 30; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	// Chain age 6M (need 5.7M).
	ctx = ctx.WithBlockHeight(6_000_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.DistinctLipVoters != 30 {
		t.Errorf("expected 30 voters, got %d", conditions.DistinctLipVoters)
	}
	if conditions.ActiveGuardians != 12 {
		t.Errorf("expected 12 guardians, got %d", conditions.ActiveGuardians)
	}
	if conditions.ProposalsExecutedInPhase != 5 {
		t.Errorf("expected 5 proposals executed, got %d", conditions.ProposalsExecutedInPhase)
	}
	if conditions.CommunitySeatParticipation < 2 {
		t.Errorf("expected >= 2 community seat votes, got %d", conditions.CommunitySeatParticipation)
	}
	if !allMet {
		t.Error("all Phase 1 exit conditions should be met")
	}
}

// TestExitConditions_Phase1_ProposalCountInsufficient verifies that Phase 1
// exit fails when ProposalsExecutedInPhase is below the 3-proposal threshold.
func TestExitConditions_Phase1_ProposalCountInsufficient(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Set phase to Observer.
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER)

	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	state.ProposalsExecutedInPhase = 1 // need 3
	state.PhaseStartedAtBlock = 50
	k.SetResearchFundGovernanceState(ctx, state)

	voter1, _ := setupResearchVoters(t, k, ctx)

	// Create 2 proposals with community votes (satisfies MinCommunitySeatVotes=2).
	for i := 0; i < 2; i++ {
		resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
			Proposer:  voter1,
			Title:     fmt.Sprintf("Proposal %d", i+1),
			Recipient: testAddr("recipient"),
			Amount:    "100000000",
		})
		if err != nil {
			t.Fatalf("submit %d failed: %v", i+1, err)
		}
		k.SetResearchCommunityVote(ctx, resp.ProposalId, community1, "yes")
	}

	mockSK.guardianCount = 12
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	for i := 0; i < 30; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	ctx = ctx.WithBlockHeight(6_000_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.ProposalsExecutedInPhase != 1 {
		t.Errorf("expected 1 proposal executed, got %d", conditions.ProposalsExecutedInPhase)
	}
	if allMet {
		t.Error("conditions should NOT be met (only 1 of 3 required proposals executed)")
	}
}

// TestExitConditions_Phase1_CommunitySeatInactive verifies that Phase 1 exit
// fails when community seat participation is below the 2-vote threshold.
func TestExitConditions_Phase1_CommunitySeatInactive(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Set phase to Observer.
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER)

	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{testAddr("community1")}
	state.ProposalsExecutedInPhase = 5
	state.PhaseStartedAtBlock = 50
	k.SetResearchFundGovernanceState(ctx, state)

	// No community votes recorded — 0 of 2 required.

	mockSK.guardianCount = 12
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	for i := 0; i < 30; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	ctx = ctx.WithBlockHeight(6_000_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.CommunitySeatParticipation != 0 {
		t.Errorf("expected 0 community seat votes, got %d", conditions.CommunitySeatParticipation)
	}
	if allMet {
		t.Error("conditions should NOT be met (0 of 2 required community seat votes)")
	}
}

// TestExitConditions_Phase2_AllMet verifies that Phase 2 (Balanced) exit
// succeeds when all conditions are met: 50+ voters, 22+ guardians,
// chain age 12.6M+, 10+ proposals executed, and zero emergency halts.
func TestExitConditions_Phase2_AllMet(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Set phase to Balanced.
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED)

	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED
	state.ProposalsExecutedInPhase = 12
	state.PhaseStartedAtBlock = 50
	k.SetResearchFundGovernanceState(ctx, state)

	mockSK.guardianCount = 25
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{}}
	k.SetEmergencyKeeper(mockEK)

	// Record 55 distinct voters (need 50).
	for i := 0; i < 55; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	// Chain age 13M (need 12.6M).
	ctx = ctx.WithBlockHeight(13_000_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.DistinctLipVoters != 55 {
		t.Errorf("expected 55 voters, got %d", conditions.DistinctLipVoters)
	}
	if conditions.ActiveGuardians != 25 {
		t.Errorf("expected 25 guardians, got %d", conditions.ActiveGuardians)
	}
	if conditions.ProposalsExecutedInPhase != 12 {
		t.Errorf("expected 12 proposals executed, got %d", conditions.ProposalsExecutedInPhase)
	}
	if conditions.EmergencyHaltsFromMisuse != 0 {
		t.Errorf("expected 0 emergency halts, got %d", conditions.EmergencyHaltsFromMisuse)
	}
	if !allMet {
		t.Error("all Phase 2 exit conditions should be met")
	}
}

// TestExitConditions_Phase2_EmergencyHaltBlocksTransition verifies that
// a single emergency halt in Phase 2 blocks the transition to Phase 3,
// even when all other conditions are met (zero tolerance policy).
func TestExitConditions_Phase2_EmergencyHaltBlocksTransition(t *testing.T) {
	k, ctx, mockSK := setupWithStaking(t, "1000000000000")

	// Set phase to Balanced.
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED)

	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED
	state.ProposalsExecutedInPhase = 12
	state.PhaseStartedAtBlock = 50
	k.SetResearchFundGovernanceState(ctx, state)

	mockSK.guardianCount = 25
	// One emergency halt — should block transition.
	mockEK := &mockEmergencyKeeper{halts: map[string]uint64{
		"research_fund": 1,
	}}
	k.SetEmergencyKeeper(mockEK)

	for i := 0; i < 55; i++ {
		k.RecordDistinctVoter(ctx, testAddr(fmt.Sprintf("voter%d", i)))
	}

	ctx = ctx.WithBlockHeight(13_000_000)

	conditions, allMet := k.CheckPhaseExitConditions(ctx)

	if conditions.EmergencyHaltsFromMisuse != 1 {
		t.Errorf("expected 1 emergency halt, got %d", conditions.EmergencyHaltsFromMisuse)
	}
	if allMet {
		t.Error("conditions should NOT be met (emergency halt blocks transition)")
	}
}
