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

	"github.com/zerone-chain/zerone/x/capture_challenge/keeper"
	"github.com/zerone-chain/zerone/x/capture_challenge/types"
)

// -----------------------------------------------------------------------
// Mock BankKeeper
// -----------------------------------------------------------------------

type mockBankKeeper struct {
	balances       map[string]map[string]int64
	moduleBalances map[string]map[string]int64
	burned         int64
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

func (m *mockBankKeeper) SendCoins(_ context.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	for _, coin := range amt {
		fromAddr := from.String()
		toAddr := to.String()
		if m.balances[fromAddr] == nil {
			m.balances[fromAddr] = make(map[string]int64)
		}
		if m.balances[toAddr] == nil {
			m.balances[toAddr] = make(map[string]int64)
		}
		m.balances[fromAddr][coin.Denom] -= coin.Amount.Int64()
		m.balances[toAddr][coin.Denom] += coin.Amount.Int64()
	}
	return nil
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

func (m *mockBankKeeper) SendCoinsFromModuleToModule(_ context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	for _, coin := range amt {
		if m.moduleBalances[senderModule] == nil {
			m.moduleBalances[senderModule] = make(map[string]int64)
		}
		if m.moduleBalances[recipientModule] == nil {
			m.moduleBalances[recipientModule] = make(map[string]int64)
		}
		m.moduleBalances[senderModule][coin.Denom] -= coin.Amount.Int64()
		m.moduleBalances[recipientModule][coin.Denom] += coin.Amount.Int64()
	}
	return nil
}

func (m *mockBankKeeper) BurnCoins(_ context.Context, moduleName string, amt sdk.Coins) error {
	for _, coin := range amt {
		if m.moduleBalances[moduleName] == nil {
			m.moduleBalances[moduleName] = make(map[string]int64)
		}
		m.moduleBalances[moduleName][coin.Denom] -= coin.Amount.Int64()
		m.burned += coin.Amount.Int64()
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
// Tests: Params
// -----------------------------------------------------------------------

func TestParamsDefault(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	params := k.GetParams(ctx)
	if params.MinChallengeStake != "10000000" {
		t.Errorf("expected MinChallengeStake 10000000, got %s", params.MinChallengeStake)
	}
	if params.EvidencePeriodBlocks != 5000 {
		t.Errorf("expected EvidencePeriodBlocks 5000, got %d", params.EvidencePeriodBlocks)
	}
	if params.ReviewPeriodBlocks != 20000 {
		t.Errorf("expected ReviewPeriodBlocks 20000, got %d", params.ReviewPeriodBlocks)
	}
	if params.RewardRateBps != 100000 {
		t.Errorf("expected RewardRateBps 100000, got %d", params.RewardRateBps)
	}
	if params.SlashRateBps != 50000 {
		t.Errorf("expected SlashRateBps 50000, got %d", params.SlashRateBps)
	}
}

func TestParamsSetGet(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	custom := types.DefaultParams()
	custom.MinChallengeStake = "20000000"
	custom.EvidencePeriodBlocks = 10000
	k.SetParams(ctx, custom)

	got := k.GetParams(ctx)
	if got.MinChallengeStake != "20000000" {
		t.Errorf("expected MinChallengeStake 20000000, got %s", got.MinChallengeStake)
	}
	if got.EvidencePeriodBlocks != 10000 {
		t.Errorf("expected EvidencePeriodBlocks 10000, got %d", got.EvidencePeriodBlocks)
	}
}

// -----------------------------------------------------------------------
// Tests: SubmitChallenge
// -----------------------------------------------------------------------

func TestSubmitChallenge(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	srv := keeper.NewMsgServerImpl(k)
	resp, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "mathematics",
		AccusedValidators: []string{testAddr(10), testAddr(11)},
		Stake:             "10000000",
		Reason:            "Suspected validator capture in mathematics domain",
	})
	if err != nil {
		t.Fatalf("SubmitChallenge failed: %v", err)
	}
	if resp.ChallengeId == "" {
		t.Fatal("expected non-empty challenge ID")
	}

	// Verify challenge state
	challenge, found := k.GetChallenge(ctx, resp.ChallengeId)
	if !found {
		t.Fatal("challenge not found")
	}
	if challenge.Challenger != challenger {
		t.Errorf("expected challenger %s, got %s", challenger, challenge.Challenger)
	}
	if challenge.Domain != "mathematics" {
		t.Errorf("expected domain mathematics, got %s", challenge.Domain)
	}
	if challenge.Status != types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE {
		t.Errorf("expected EVIDENCE status, got %s", challenge.Status.String())
	}
	if challenge.Stake != "10000000" {
		t.Errorf("expected stake 10000000, got %s", challenge.Stake)
	}
	if len(challenge.AccusedValidators) != 2 {
		t.Errorf("expected 2 accused validators, got %d", len(challenge.AccusedValidators))
	}

	// Verify escrow called (module balance increased)
	modBal := bk.moduleBalances[types.ModuleName]["uzrn"]
	if modBal != 10_000_000 {
		t.Errorf("expected module balance 10000000, got %d", modBal)
	}

	// Verify challenger balance decreased
	if bk.balances[challenger]["uzrn"] != 90_000_000 {
		t.Errorf("expected challenger balance 90000000, got %d", bk.balances[challenger]["uzrn"])
	}

	// Verify domain index
	ids := k.GetChallengesByDomain(ctx, "mathematics")
	if len(ids) != 1 {
		t.Errorf("expected 1 challenge in domain index, got %d", len(ids))
	}
}

func TestSubmitChallengeInsufficientStake(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "mathematics",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "1000", // below minimum 10000000
		Reason:            "Low stake",
	})
	if err == nil {
		t.Fatal("expected insufficient stake error")
	}
}

// -----------------------------------------------------------------------
// Tests: AddEvidence
// -----------------------------------------------------------------------

func TestAddEvidence(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	srv := keeper.NewMsgServerImpl(k)
	resp, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "physics",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Reason:            "Capture in physics",
	})
	if err != nil {
		t.Fatalf("SubmitChallenge failed: %v", err)
	}

	// Add evidence
	_, err = srv.AddEvidence(ctx, &types.MsgAddEvidence{
		Challenger:  challenger,
		ChallengeId: resp.ChallengeId,
		Description: "Voting pattern anomaly detected at block 50",
		DataHash:    "abc123hash",
	})
	if err != nil {
		t.Fatalf("AddEvidence failed: %v", err)
	}

	// Verify evidence appended
	challenge, found := k.GetChallenge(ctx, resp.ChallengeId)
	if !found {
		t.Fatal("challenge not found")
	}
	if len(challenge.Evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %d", len(challenge.Evidence))
	}
	if challenge.Evidence[0].Description != "Voting pattern anomaly detected at block 50" {
		t.Errorf("unexpected evidence description: %s", challenge.Evidence[0].Description)
	}
	if challenge.Evidence[0].DataHash != "abc123hash" {
		t.Errorf("unexpected data hash: %s", challenge.Evidence[0].DataHash)
	}

	// Add second evidence
	_, err = srv.AddEvidence(ctx, &types.MsgAddEvidence{
		Challenger:  challenger,
		ChallengeId: resp.ChallengeId,
		Description: "Additional correlation data",
		DataHash:    "def456hash",
	})
	if err != nil {
		t.Fatalf("AddEvidence second failed: %v", err)
	}

	challenge, _ = k.GetChallenge(ctx, resp.ChallengeId)
	if len(challenge.Evidence) != 2 {
		t.Fatalf("expected 2 evidence entries, got %d", len(challenge.Evidence))
	}
}

func TestAddEvidenceWrongChallenger(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	outsider := testAddr(2)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	srv := keeper.NewMsgServerImpl(k)
	resp, _ := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "physics",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Reason:            "Capture",
	})

	_, err := srv.AddEvidence(ctx, &types.MsgAddEvidence{
		Challenger:  outsider,
		ChallengeId: resp.ChallengeId,
		Description: "Should fail",
		DataHash:    "xyz",
	})
	if err == nil {
		t.Fatal("expected error for wrong challenger")
	}
}

// -----------------------------------------------------------------------
// Tests: ResolveChallenge - Upheld
// -----------------------------------------------------------------------

func TestResolveUpheld(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	// Fund bounty pool first
	srv := keeper.NewMsgServerImpl(k)
	funder := testAddr(2)
	bk.setBalance(funder, "uzrn", 500_000_000)
	_, err := srv.FundBountyPool(ctx, &types.MsgFundBountyPool{
		Sender: funder,
		Domain: "mathematics",
		Amount: "100000000", // 100 ZRN
	})
	if err != nil {
		t.Fatalf("FundBountyPool failed: %v", err)
	}

	// Submit challenge
	resp, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "mathematics",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Reason:            "Capture detected",
	})
	if err != nil {
		t.Fatalf("SubmitChallenge failed: %v", err)
	}

	// Advance to UNDER_REVIEW manually
	ch, _ := k.GetChallenge(ctx, resp.ChallengeId)
	ch.Status = types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW
	k.SetChallenge(ctx, ch)

	authority := k.GetAuthority()
	challengerBalBefore := bk.balances[challenger]["uzrn"]

	// Resolve as upheld
	_, err = srv.ResolveChallenge(ctx, &types.MsgResolveChallenge{
		Authority:   authority,
		ChallengeId: resp.ChallengeId,
		Outcome:     types.ChallengeOutcome_CHALLENGE_OUTCOME_UPHELD,
		Reason:      "Evidence confirmed capture",
	})
	if err != nil {
		t.Fatalf("ResolveChallenge failed: %v", err)
	}

	// Verify challenge resolved
	ch, _ = k.GetChallenge(ctx, resp.ChallengeId)
	if ch.Status != types.ChallengeStatus_CHALLENGE_STATUS_RESOLVED {
		t.Errorf("expected RESOLVED status, got %s", ch.Status.String())
	}
	if ch.Resolution == nil {
		t.Fatal("expected non-nil resolution")
	}
	if ch.Resolution.Outcome != types.ChallengeOutcome_CHALLENGE_OUTCOME_UPHELD {
		t.Errorf("expected UPHELD outcome, got %s", ch.Resolution.Outcome.String())
	}

	// Verify reward sent to challenger (10% of 100 ZRN = 10 ZRN = 10000000)
	challengerBalAfter := bk.balances[challenger]["uzrn"]
	// Challenger receives: stake return (10000000) + reward (10000000)
	expectedGain := int64(10_000_000 + 10_000_000)
	actualGain := challengerBalAfter - challengerBalBefore
	if actualGain != expectedGain {
		t.Errorf("expected challenger balance gain %d, got %d", expectedGain, actualGain)
	}

	// Verify bounty pool decreased
	pool, found := k.GetBountyPool(ctx, "mathematics")
	if !found {
		t.Fatal("bounty pool not found")
	}
	if pool.Balance != "90000000" {
		t.Errorf("expected bounty pool balance 90000000, got %s", pool.Balance)
	}

	// Verify slash records
	if len(ch.Slashes) != 1 {
		t.Fatalf("expected 1 slash record, got %d", len(ch.Slashes))
	}
	if ch.Slashes[0].Validator != testAddr(10) {
		t.Errorf("expected validator %s in slash, got %s", testAddr(10), ch.Slashes[0].Validator)
	}
}

// -----------------------------------------------------------------------
// Tests: ResolveChallenge - Rejected
// -----------------------------------------------------------------------

func TestResolveRejected(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	srv := keeper.NewMsgServerImpl(k)
	resp, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "biology",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Reason:            "False alarm",
	})
	if err != nil {
		t.Fatalf("SubmitChallenge failed: %v", err)
	}

	// Advance to UNDER_REVIEW
	ch, _ := k.GetChallenge(ctx, resp.ChallengeId)
	ch.Status = types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW
	k.SetChallenge(ctx, ch)

	authority := k.GetAuthority()
	burnedBefore := bk.burned

	// Resolve as rejected
	_, err = srv.ResolveChallenge(ctx, &types.MsgResolveChallenge{
		Authority:   authority,
		ChallengeId: resp.ChallengeId,
		Outcome:     types.ChallengeOutcome_CHALLENGE_OUTCOME_REJECTED,
		Reason:      "Insufficient evidence",
	})
	if err != nil {
		t.Fatalf("ResolveChallenge failed: %v", err)
	}

	// Verify stake was burned
	burnedAfter := bk.burned
	if burnedAfter-burnedBefore != 10_000_000 {
		t.Errorf("expected 10000000 burned, got %d", burnedAfter-burnedBefore)
	}

	// Verify bounty pool received the burned amount
	pool, found := k.GetBountyPool(ctx, "biology")
	if !found {
		t.Fatal("bounty pool not found after reject")
	}
	if pool.Balance != "10000000" {
		t.Errorf("expected bounty pool balance 10000000, got %s", pool.Balance)
	}

	// Verify challenge resolved
	ch, _ = k.GetChallenge(ctx, resp.ChallengeId)
	if ch.Status != types.ChallengeStatus_CHALLENGE_STATUS_RESOLVED {
		t.Errorf("expected RESOLVED status, got %s", ch.Status.String())
	}
	if ch.Resolution.Outcome != types.ChallengeOutcome_CHALLENGE_OUTCOME_REJECTED {
		t.Errorf("expected REJECTED outcome, got %s", ch.Resolution.Outcome.String())
	}
}

// -----------------------------------------------------------------------
// Tests: FundBountyPool
// -----------------------------------------------------------------------

func TestFundBountyPool(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	funder := testAddr(1)
	bk.setBalance(funder, "uzrn", 500_000_000)

	srv := keeper.NewMsgServerImpl(k)

	// First funding
	_, err := srv.FundBountyPool(ctx, &types.MsgFundBountyPool{
		Sender: funder,
		Domain: "chemistry",
		Amount: "50000000",
	})
	if err != nil {
		t.Fatalf("FundBountyPool failed: %v", err)
	}

	pool, found := k.GetBountyPool(ctx, "chemistry")
	if !found {
		t.Fatal("bounty pool not found")
	}
	if pool.Balance != "50000000" {
		t.Errorf("expected balance 50000000, got %s", pool.Balance)
	}

	// Second funding
	_, err = srv.FundBountyPool(ctx, &types.MsgFundBountyPool{
		Sender: funder,
		Domain: "chemistry",
		Amount: "30000000",
	})
	if err != nil {
		t.Fatalf("FundBountyPool second failed: %v", err)
	}

	pool, _ = k.GetBountyPool(ctx, "chemistry")
	if pool.Balance != "80000000" {
		t.Errorf("expected balance 80000000, got %s", pool.Balance)
	}

	// Verify module balance
	modBal := bk.moduleBalances[types.ModuleName]["uzrn"]
	if modBal != 80_000_000 {
		t.Errorf("expected module balance 80000000, got %d", modBal)
	}
}

// -----------------------------------------------------------------------
// Tests: Phase Advancement
// -----------------------------------------------------------------------

func TestPhaseAdvancement(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create a challenge manually with EVIDENCE status and short deadline
	ch := &types.CaptureChallenge{
		Id:               "test-challenge-1",
		Challenger:        testAddr(1),
		Domain:            "mathematics",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Status:            types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE,
		CreatedBlock:      50,
		EvidenceDeadline:  150, // at block 150
		ReviewDeadline:    300, // at block 300
	}
	k.SetChallenge(ctx, ch)

	// Advance at block 140 - should NOT change
	k.AdvanceChallengePhases(ctx, 140)
	ch, _ = k.GetChallenge(ctx, "test-challenge-1")
	if ch.Status != types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE {
		t.Errorf("expected EVIDENCE at block 140, got %s", ch.Status.String())
	}

	// Advance at block 150 - should transition to UNDER_REVIEW
	k.AdvanceChallengePhases(ctx, 150)
	ch, _ = k.GetChallenge(ctx, "test-challenge-1")
	if ch.Status != types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW {
		t.Errorf("expected UNDER_REVIEW at block 150, got %s", ch.Status.String())
	}

	// Advance at block 299 - should stay UNDER_REVIEW
	k.AdvanceChallengePhases(ctx, 299)
	ch, _ = k.GetChallenge(ctx, "test-challenge-1")
	if ch.Status != types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW {
		t.Errorf("expected UNDER_REVIEW at block 299, got %s", ch.Status.String())
	}

	// Advance at block 300 - should auto-expire
	k.AdvanceChallengePhases(ctx, 300)
	ch, _ = k.GetChallenge(ctx, "test-challenge-1")
	if ch.Status != types.ChallengeStatus_CHALLENGE_STATUS_EXPIRED {
		t.Errorf("expected EXPIRED at block 300, got %s", ch.Status.String())
	}
}

// -----------------------------------------------------------------------
// Tests: Domain Pause
// -----------------------------------------------------------------------

func TestDomainPause(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 200_000_000)

	srv := keeper.NewMsgServerImpl(k)

	// Submit first challenge - should set domain pause
	_, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "linguistics",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Reason:            "First challenge",
	})
	if err != nil {
		t.Fatalf("first SubmitChallenge failed: %v", err)
	}

	// Verify domain pause created
	pauseUntil, found := k.GetPausedDomain(ctx, "linguistics")
	if !found {
		t.Fatal("expected domain pause to be set")
	}
	params := k.GetParams(ctx)
	expectedPause := uint64(100) + params.DomainPauseBlocks // block 100 + 1000 = 1100
	if pauseUntil != expectedPause {
		t.Errorf("expected pause until %d, got %d", expectedPause, pauseUntil)
	}

	// Submit second challenge to same domain - should fail (paused)
	_, err = srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "linguistics",
		AccusedValidators: []string{testAddr(11)},
		Stake:             "10000000",
		Reason:            "Second challenge",
	})
	if err == nil {
		t.Fatal("expected error for paused domain")
	}

	// Submit to different domain - should succeed
	_, err = srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "philosophy",
		AccusedValidators: []string{testAddr(12)},
		Stake:             "10000000",
		Reason:            "Different domain",
	})
	if err != nil {
		t.Fatalf("SubmitChallenge to different domain failed: %v", err)
	}
}

// -----------------------------------------------------------------------
// Tests: Genesis
// -----------------------------------------------------------------------

func TestGenesis(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Populate state
	k.SetChallenge(ctx, &types.CaptureChallenge{
		Id: "ch-1", Challenger: testAddr(1), Domain: "math",
		Stake: "10000000", Status: types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE,
	})
	k.SetChallenge(ctx, &types.CaptureChallenge{
		Id: "ch-2", Challenger: testAddr(2), Domain: "physics",
		Stake: "20000000", Status: types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW,
	})
	k.SetBountyPool(ctx, &types.DomainBountyPool{Domain: "math", Balance: "50000000"})

	// Export
	genState := k.ExportGenesis(ctx)
	if len(genState.Challenges) != 2 {
		t.Errorf("expected 2 challenges, got %d", len(genState.Challenges))
	}
	if len(genState.BountyPools) != 1 {
		t.Errorf("expected 1 bounty pool, got %d", len(genState.BountyPools))
	}

	// Import into fresh keeper
	k2, ctx2, _ := setupKeeper(t)
	k2.InitGenesis(ctx2, genState)

	got := k2.ExportGenesis(ctx2)
	if len(got.Challenges) != 2 {
		t.Errorf("expected 2 challenges after import, got %d", len(got.Challenges))
	}
	if len(got.BountyPools) != 1 {
		t.Errorf("expected 1 bounty pool after import, got %d", len(got.BountyPools))
	}
}

// -----------------------------------------------------------------------
// Tests: Queries
// -----------------------------------------------------------------------

func TestQueryParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Params.MinChallengeStake != "10000000" {
		t.Errorf("expected 10000000, got %s", resp.Params.MinChallengeStake)
	}
}

func TestQueryChallenge(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetChallenge(ctx, &types.CaptureChallenge{
		Id:     "q-ch-1",
		Domain: "test",
		Stake:  "5000000",
		Status: types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE,
	})

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.Challenge(ctx, &types.QueryChallengeRequest{Id: "q-ch-1"})
	if err != nil {
		t.Fatalf("Query Challenge failed: %v", err)
	}
	if resp.Challenge.Stake != "5000000" {
		t.Errorf("expected stake 5000000, got %s", resp.Challenge.Stake)
	}
}

func TestQueryChallengeNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)
	_, err := qs.Challenge(ctx, &types.QueryChallengeRequest{Id: "nonexistent"})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestQueryBountyPool(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetBountyPool(ctx, &types.DomainBountyPool{Domain: "math", Balance: "100000000"})

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.BountyPool(ctx, &types.QueryBountyPoolRequest{Domain: "math"})
	if err != nil {
		t.Fatalf("Query BountyPool failed: %v", err)
	}
	if resp.Pool.Balance != "100000000" {
		t.Errorf("expected balance 100000000, got %s", resp.Pool.Balance)
	}
}

func TestQueryChallengesByDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create challenges with domain index
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("ch-%d", i)
		k.SetChallenge(ctx, &types.CaptureChallenge{
			Id: id, Domain: "physics", Stake: "10000000",
			Status: types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE,
		})
		k.SetDomainIndex(ctx, "physics", id)
	}

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.ChallengesByDomain(ctx, &types.QueryChallengesByDomainRequest{Domain: "physics"})
	if err != nil {
		t.Fatalf("Query ChallengesByDomain failed: %v", err)
	}
	if len(resp.Challenges) != 3 {
		t.Errorf("expected 3 challenges, got %d", len(resp.Challenges))
	}
}

// -----------------------------------------------------------------------
// Tests: ResolveChallenge - Unauthorized
// -----------------------------------------------------------------------

func TestAddEvidenceAfterDeadline(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	srv := keeper.NewMsgServerImpl(k)
	resp, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "physics",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Reason:            "Capture in physics",
	})
	if err != nil {
		t.Fatalf("SubmitChallenge failed: %v", err)
	}

	// Set evidence deadline to a past block
	ch, _ := k.GetChallenge(ctx, resp.ChallengeId)
	ch.EvidenceDeadline = 50 // past (current block = 100)
	k.SetChallenge(ctx, ch)

	// Try to add evidence after deadline
	_, err = srv.AddEvidence(ctx, &types.MsgAddEvidence{
		Challenger:  challenger,
		ChallengeId: resp.ChallengeId,
		Description: "Late evidence",
		DataHash:    "late-hash",
	})
	if err == nil {
		t.Fatal("expected error for evidence after deadline")
	}
}

func TestResolvePartial(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	// Fund bounty pool
	srv := keeper.NewMsgServerImpl(k)
	funder := testAddr(2)
	bk.setBalance(funder, "uzrn", 500_000_000)
	_, _ = srv.FundBountyPool(ctx, &types.MsgFundBountyPool{
		Sender: funder,
		Domain: "mixed",
		Amount: "100000000",
	})

	// Submit challenge with 2 accused validators
	resp, err := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "mixed",
		AccusedValidators: []string{testAddr(10), testAddr(11)},
		Stake:             "10000000",
		Reason:            "Partial capture",
	})
	if err != nil {
		t.Fatalf("SubmitChallenge failed: %v", err)
	}

	// Advance to UNDER_REVIEW
	ch, _ := k.GetChallenge(ctx, resp.ChallengeId)
	ch.Status = types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW
	k.SetChallenge(ctx, ch)

	authority := k.GetAuthority()
	challengerBalBefore := bk.balances[challenger]["uzrn"]

	// Resolve as partial
	_, err = srv.ResolveChallenge(ctx, &types.MsgResolveChallenge{
		Authority:   authority,
		ChallengeId: resp.ChallengeId,
		Outcome:     types.ChallengeOutcome_CHALLENGE_OUTCOME_PARTIAL,
		Reason:      "Only some validators were captured",
	})
	if err != nil {
		t.Fatalf("ResolveChallenge PARTIAL failed: %v", err)
	}

	// Verify challenge resolved with partial outcome
	ch, _ = k.GetChallenge(ctx, resp.ChallengeId)
	if ch.Status != types.ChallengeStatus_CHALLENGE_STATUS_RESOLVED {
		t.Errorf("expected RESOLVED status, got %s", ch.Status.String())
	}
	if ch.Resolution.Outcome != types.ChallengeOutcome_CHALLENGE_OUTCOME_PARTIAL {
		t.Errorf("expected PARTIAL outcome, got %s", ch.Resolution.Outcome.String())
	}

	// Challenger should get some reward (half stake return + half reward)
	challengerBalAfter := bk.balances[challenger]["uzrn"]
	if challengerBalAfter <= challengerBalBefore {
		t.Errorf("expected challenger balance to increase on partial resolution: before=%d, after=%d",
			challengerBalBefore, challengerBalAfter)
	}
}

func TestResolveChallengeUnauthorized(t *testing.T) {
	k, ctx, bk := setupKeeper(t)

	challenger := testAddr(1)
	bk.setBalance(challenger, "uzrn", 100_000_000)

	srv := keeper.NewMsgServerImpl(k)
	resp, _ := srv.SubmitChallenge(ctx, &types.MsgSubmitChallenge{
		Challenger:        challenger,
		Domain:            "math",
		AccusedValidators: []string{testAddr(10)},
		Stake:             "10000000",
		Reason:            "Test",
	})

	ch, _ := k.GetChallenge(ctx, resp.ChallengeId)
	ch.Status = types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW
	k.SetChallenge(ctx, ch)

	_, err := srv.ResolveChallenge(ctx, &types.MsgResolveChallenge{
		Authority:   testAddr(99), // wrong authority
		ChallengeId: resp.ChallengeId,
		Outcome:     types.ChallengeOutcome_CHALLENGE_OUTCOME_UPHELD,
		Reason:      "Should fail",
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// -----------------------------------------------------------------------
// Tests: UpdateParams
// -----------------------------------------------------------------------

func TestUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.MinChallengeStake = "50000000"
	newParams.EvidencePeriodBlocks = 10000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MinChallengeStake != "50000000" {
		t.Errorf("expected MinChallengeStake 50000000, got %s", got.MinChallengeStake)
	}
	if got.EvidencePeriodBlocks != 10000 {
		t.Errorf("expected EvidencePeriodBlocks 10000, got %d", got.EvidencePeriodBlocks)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr(99),
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
	badParams.MinChallengeStake = "0" // invalid: must be > 0

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}

// -----------------------------------------------------------------------
// Unused import guard
// -----------------------------------------------------------------------

var _ = sdkmath.ZeroInt()
