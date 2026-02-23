package keeper_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"

	"github.com/zerone-chain/zerone/x/disputes/keeper"
	"github.com/zerone-chain/zerone/x/disputes/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
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
	}
	return nil
}

// ---------- Mock StakingKeeper ----------

type mockStakingKeeper struct {
	validators []string
}

func newMockStakingKeeper(validators []string) *mockStakingKeeper {
	return &mockStakingKeeper{validators: validators}
}

func (m *mockStakingKeeper) GetQualifiedValidators(_ context.Context, _ string, count int) ([]string, error) {
	if count > len(m.validators) {
		return m.validators, nil
	}
	return m.validators[:count], nil
}

// ---------- Mock KnowledgeKeeper ----------

type mockKnowledgeKeeper struct {
	facts map[string]*knowledgetypes.Fact
}

func newMockKnowledgeKeeper() *mockKnowledgeKeeper {
	return &mockKnowledgeKeeper{
		facts: make(map[string]*knowledgetypes.Fact),
	}
}

func (m *mockKnowledgeKeeper) addFact(id, submitter string) {
	m.facts[id] = &knowledgetypes.Fact{
		Id:        id,
		Submitter: submitter,
	}
}

func (m *mockKnowledgeKeeper) GetFact(_ context.Context, factID string) (*knowledgetypes.Fact, bool) {
	f, ok := m.facts[factID]
	return f, ok
}

// ---------- Test Addresses ----------

func testAddr(name string) string {
	h := sha256.Sum256([]byte("test_seed:" + name))
	return sdk.AccAddress(h[:20]).String()
}

// ---------- Test Setup ----------

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper, *mockStakingKeeper, *mockKnowledgeKeeper) {
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

	// Create validators for arbiter selection (need enough to not include challenger/defender)
	validators := []string{
		testAddr("validator1"),
		testAddr("validator2"),
		testAddr("validator3"),
		testAddr("validator4"),
		testAddr("validator5"),
		testAddr("validator6"),
		testAddr("validator7"),
		testAddr("validator8"),
		testAddr("validator9"),
		testAddr("validator10"),
	}
	mockSK := newMockStakingKeeper(validators)
	mockKK := newMockKnowledgeKeeper()

	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(storeService, cdc, "zrn1authority", mockBK, mockSK, mockKK)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, ChainID: "zerone-test-1"}, false, log.NewNopLogger())

	return k, ctx, mockBK, mockSK, mockKK
}

func setupMsgServer(t *testing.T) (types.MsgServer, keeper.Keeper, sdk.Context, *mockBankKeeper, *mockStakingKeeper, *mockKnowledgeKeeper) {
	t.Helper()
	k, ctx, bk, sk, kk := setupKeeper(t)
	return keeper.NewMsgServerImpl(k), k, ctx, bk, sk, kk
}

// ---------- Params Tests ----------

func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()
	if len(params.TierConfigs) != 4 {
		t.Errorf("expected 4 tier configs, got %d", len(params.TierConfigs))
	}
	if params.MaxActiveDisputes != 100 {
		t.Errorf("expected max active disputes 100, got %d", params.MaxActiveDisputes)
	}
	if params.SlashRateLoserBps != 500000 {
		t.Errorf("expected slash rate 500000, got %d", params.SlashRateLoserBps)
	}
}

func TestSetGetParams(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	params := &types.Params{
		TierConfigs:        types.DefaultTierConfigs(),
		MaxActiveDisputes:  50,
		EscalationDelay:    1000,
		SlashRateLoserBps:  600000,
		RewardRateWinnerBps: 300000,
		ArbiterRewardBps:   100000,
	}
	k.SetParams(ctx, params)

	got := k.GetParams(ctx)
	if got.MaxActiveDisputes != 50 {
		t.Errorf("expected max active disputes 50, got %d", got.MaxActiveDisputes)
	}
	if got.SlashRateLoserBps != 600000 {
		t.Errorf("expected slash rate 600000, got %d", got.SlashRateLoserBps)
	}
}

// ---------- Dispute CRUD Tests ----------

func TestSetGetDispute(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	dispute := &types.Dispute{
		Id:         "dispute-1",
		TargetId:   "fact-1",
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		Challenger: testAddr("challenger"),
		Defender:   testAddr("defender"),
		Bond:       "1000000",
		Tier:       1,
		Phase:      types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
	}
	k.SetDispute(ctx, dispute)

	got, found := k.GetDispute(ctx, "dispute-1")
	if !found {
		t.Fatal("dispute not found")
	}
	if got.Challenger != dispute.Challenger {
		t.Errorf("expected challenger %s, got %s", dispute.Challenger, got.Challenger)
	}
	if got.Bond != "1000000" {
		t.Errorf("expected bond 1000000, got %s", got.Bond)
	}
}

func TestGetDisputeNotFound(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	_, found := k.GetDispute(ctx, "nonexistent")
	if found {
		t.Fatal("expected dispute not found")
	}
}

func TestGetDisputesByTarget(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	for i := 0; i < 3; i++ {
		d := &types.Dispute{
			Id:         fmt.Sprintf("dispute-%d", i),
			TargetId:   "fact-1",
			TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
			Challenger: testAddr("challenger"),
			Phase:      types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
		}
		k.SetDispute(ctx, d)
	}

	disputes := k.GetDisputesByTarget(ctx, "fact-1")
	if len(disputes) != 3 {
		t.Errorf("expected 3 disputes, got %d", len(disputes))
	}
}

func TestGetActiveDisputes(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	// Active dispute
	k.SetDispute(ctx, &types.Dispute{
		Id:       "active-1",
		TargetId: "fact-1",
		Phase:    types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
	})
	// Settled dispute
	k.SetDispute(ctx, &types.Dispute{
		Id:       "settled-1",
		TargetId: "fact-2",
		Phase:    types.DisputePhase_DISPUTE_PHASE_SETTLED,
	})

	active := k.GetActiveDisputes(ctx)
	if len(active) != 1 {
		t.Errorf("expected 1 active dispute, got %d", len(active))
	}
	if active[0].Id != "active-1" {
		t.Errorf("expected active-1, got %s", active[0].Id)
	}
}

// ---------- Genesis Tests ----------

func TestInitExportGenesis(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	genState := &types.GenesisState{
		Params: &types.Params{
			TierConfigs:        types.DefaultTierConfigs(),
			MaxActiveDisputes:  50,
			EscalationDelay:    1000,
			SlashRateLoserBps:  500000,
			RewardRateWinnerBps: 400000,
			ArbiterRewardBps:   100000,
		},
		Disputes: []*types.Dispute{
			{
				Id:         "gen-dispute-1",
				TargetId:   "fact-1",
				TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
				Challenger: testAddr("challenger"),
				Bond:       "5000000",
				Phase:      types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
			},
		},
	}

	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)
	if exported.Params.MaxActiveDisputes != 50 {
		t.Errorf("expected max active disputes 50, got %d", exported.Params.MaxActiveDisputes)
	}
	if len(exported.Disputes) != 1 {
		t.Fatalf("expected 1 dispute, got %d", len(exported.Disputes))
	}
	if exported.Disputes[0].Id != "gen-dispute-1" {
		t.Errorf("expected gen-dispute-1, got %s", exported.Disputes[0].Id)
	}
}

// ---------- InitiateDispute Tests ----------

func TestInitiateDispute(t *testing.T) {
	msgSrv, k, ctx, bk, _, kk := setupMsgServer(t)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, err := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed claim",
		Bond:       "1000000",
	})
	if err != nil {
		t.Fatalf("InitiateDispute failed: %v", err)
	}
	if resp.DisputeId == "" {
		t.Fatal("expected non-empty dispute ID")
	}

	dispute, found := k.GetDispute(ctx, resp.DisputeId)
	if !found {
		t.Fatal("dispute not found")
	}
	if dispute.Challenger != challenger {
		t.Errorf("expected challenger %s, got %s", challenger, dispute.Challenger)
	}
	if dispute.Defender != defender {
		t.Errorf("expected defender %s, got %s", defender, dispute.Defender)
	}
	if dispute.Bond != "1000000" {
		t.Errorf("expected bond 1000000, got %s", dispute.Bond)
	}
	if dispute.Tier != 1 {
		t.Errorf("expected tier 1, got %d", dispute.Tier)
	}
	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT {
		t.Errorf("expected EVIDENCE_COMMIT phase, got %s", dispute.Phase.String())
	}
	if len(dispute.Arbiters) != 3 {
		t.Errorf("expected 3 arbiters, got %d", len(dispute.Arbiters))
	}

	// Verify bond was escrowed
	if bk.balances[challenger]["uzrn"] != 9_000_000 {
		t.Errorf("expected challenger balance 9000000, got %d", bk.balances[challenger]["uzrn"])
	}
}

func TestInitiateDisputeInsufficientBond(t *testing.T) {
	msgSrv, _, ctx, bk, _, kk := setupMsgServer(t)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	_, err := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed claim",
		Bond:       "100", // below tier 1 minimum of 1000000
	})
	if err == nil {
		t.Fatal("expected error for insufficient bond")
	}
}

func TestInitiateDisputeTargetNotFound(t *testing.T) {
	msgSrv, _, ctx, bk, _, _ := setupMsgServer(t)

	challenger := testAddr("challenger")
	bk.setBalance(challenger, "uzrn", 10_000_000)

	_, err := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "nonexistent-fact",
		Reason:     "Disputed",
		Bond:       "1000000",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

// ---------- CommitEvidence Tests ----------

func TestCommitEvidence(t *testing.T) {
	msgSrv, k, ctx, bk, _, kk := setupMsgServer(t)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Commit evidence
	content := "This fact is inaccurate because..."
	nonce := "random-nonce-123"
	h := sha256.Sum256([]byte(content + nonce))
	commitHash := hex.EncodeToString(h[:])

	_, err := msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      challenger,
		DisputeId:      resp.DisputeId,
		CommitmentHash: commitHash,
	})
	if err != nil {
		t.Fatalf("CommitEvidence failed: %v", err)
	}

	commitment, found := k.GetCommitment(ctx, resp.DisputeId, challenger)
	if !found {
		t.Fatal("commitment not found")
	}
	if commitment.ContentHash != commitHash {
		t.Errorf("expected hash %s, got %s", commitHash, commitment.ContentHash)
	}
	if commitment.Revealed {
		t.Error("expected commitment to be unrevealed")
	}
}

func TestCommitEvidenceWrongPhase(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	// Create dispute in ARBITRATION phase directly
	k.SetDispute(ctx, &types.Dispute{
		Id:               "dispute-arb",
		TargetId:         "fact-1",
		Challenger:       testAddr("challenger"),
		Defender:         testAddr("defender"),
		Phase:            types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
		EvidenceDeadline: 200,
	})

	_, err := msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      testAddr("challenger"),
		DisputeId:      "dispute-arb",
		CommitmentHash: "abc123",
	})
	if err == nil {
		t.Fatal("expected error for wrong phase")
	}
}

func TestCommitEvidenceNotParty(t *testing.T) {
	msgSrv, _, ctx, bk, _, kk := setupMsgServer(t)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	outsider := testAddr("outsider")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	_, err := msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      outsider,
		DisputeId:      resp.DisputeId,
		CommitmentHash: "abc123",
	})
	if err == nil {
		t.Fatal("expected error for non-party submitter")
	}
}

// ---------- RevealEvidence Tests ----------

func TestRevealEvidence(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Commit evidence
	content := "Evidence content"
	nonce := "nonce123"
	h := sha256.Sum256([]byte(content + nonce))
	commitHash := hex.EncodeToString(h[:])

	_, _ = msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      challenger,
		DisputeId:      resp.DisputeId,
		CommitmentHash: commitHash,
	})

	// Advance to reveal phase
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL
	dispute.EvidenceDeadline = 1000 // far future
	k.SetDispute(ctx, dispute)

	// Reveal
	_, err := msgSrv.RevealEvidence(ctx, &types.MsgRevealEvidence{
		Submitter: challenger,
		DisputeId: resp.DisputeId,
		Content:   content,
		Nonce:     nonce,
	})
	if err != nil {
		t.Fatalf("RevealEvidence failed: %v", err)
	}

	// Verify evidence stored
	evidences := k.GetEvidenceByDispute(ctx, resp.DisputeId)
	if len(evidences) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(evidences))
	}
	if evidences[0].Content != content {
		t.Errorf("expected content %q, got %q", content, evidences[0].Content)
	}

	// Verify commitment marked as revealed
	commitment, _ := k.GetCommitment(ctx, resp.DisputeId, challenger)
	if !commitment.Revealed {
		t.Error("expected commitment to be revealed")
	}

	// Verify evidence count incremented
	dispute, _ = k.GetDispute(ctx, resp.DisputeId)
	if dispute.EvidenceCount != 1 {
		t.Errorf("expected evidence count 1, got %d", dispute.EvidenceCount)
	}
}

func TestRevealEvidenceHashMismatch(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Commit with one hash
	h := sha256.Sum256([]byte("correct content" + "correct nonce"))
	commitHash := hex.EncodeToString(h[:])
	_, _ = msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      challenger,
		DisputeId:      resp.DisputeId,
		CommitmentHash: commitHash,
	})

	// Advance to reveal phase
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL
	dispute.EvidenceDeadline = 1000
	k.SetDispute(ctx, dispute)

	// Try to reveal with different content
	_, err := msgSrv.RevealEvidence(ctx, &types.MsgRevealEvidence{
		Submitter: challenger,
		DisputeId: resp.DisputeId,
		Content:   "wrong content",
		Nonce:     "wrong nonce",
	})
	if err == nil {
		t.Fatal("expected error for hash mismatch")
	}
}

// ---------- ArbiterVote Tests ----------

func TestArbiterVote(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Advance to arbitration phase
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// Vote as an assigned arbiter
	arbiter := dispute.Arbiters[0]
	_, err := msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
		Arbiter:   arbiter,
		DisputeId: resp.DisputeId,
		Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
		Reasoning: "Evidence supports challenger",
	})
	if err != nil {
		t.Fatalf("ArbiterVote failed: %v", err)
	}

	vote, found := k.GetVote(ctx, resp.DisputeId, arbiter)
	if !found {
		t.Fatal("vote not found")
	}
	if vote.Vote != types.ArbiterDecision_ARBITER_DECISION_CHALLENGER {
		t.Errorf("expected CHALLENGER vote, got %s", vote.Vote.String())
	}
}

func TestArbiterVoteNotArbiter(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// Try voting as non-arbiter
	outsider := testAddr("outsider")
	_, err := msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
		Arbiter:   outsider,
		DisputeId: resp.DisputeId,
		Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
		Reasoning: "Should fail",
	})
	if err == nil {
		t.Fatal("expected error for non-arbiter voter")
	}
}

func TestArbiterVoteAlreadyVoted(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	arbiter := dispute.Arbiters[0]
	_, _ = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
		Arbiter:   arbiter,
		DisputeId: resp.DisputeId,
		Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
		Reasoning: "First vote",
	})

	// Try to vote again
	_, err := msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
		Arbiter:   arbiter,
		DisputeId: resp.DisputeId,
		Vote:      types.ArbiterDecision_ARBITER_DECISION_DEFENDER,
		Reasoning: "Change mind",
	})
	if err == nil {
		t.Fatal("expected error for double voting")
	}
}

// ---------- Escalation Tests ----------

func TestEscalateDispute(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 100_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Set dispute as past escalation delay
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.CreatedAt = 1 // set to very early block so escalation delay is met
	k.SetDispute(ctx, dispute)

	// Move to a block past escalation delay
	escCtx := ctx.WithBlockHeight(600)

	escResp, err := msgSrv.EscalateDispute(escCtx, &types.MsgEscalateDispute{
		Requester:      challenger,
		DisputeId:      resp.DisputeId,
		AdditionalBond: "10000000",
	})
	if err != nil {
		t.Fatalf("EscalateDispute failed: %v", err)
	}
	if escResp.NewTier != 2 {
		t.Errorf("expected tier 2, got %d", escResp.NewTier)
	}

	// Verify dispute updated
	dispute, _ = k.GetDispute(ctx, resp.DisputeId)
	if dispute.Tier != 2 {
		t.Errorf("expected tier 2, got %d", dispute.Tier)
	}
	if dispute.Bond != "11000000" {
		t.Errorf("expected total bond 11000000, got %s", dispute.Bond)
	}
	if len(dispute.Arbiters) != 7 {
		t.Errorf("expected 7 arbiters for tier 2, got %d", len(dispute.Arbiters))
	}
}

func TestEscalateDisputeMaxTier(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 1_000_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Set to max tier
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Tier = 4
	dispute.CreatedAt = 1
	k.SetDispute(ctx, dispute)

	escCtx := ctx.WithBlockHeight(600)
	_, err := msgSrv.EscalateDispute(escCtx, &types.MsgEscalateDispute{
		Requester:      challenger,
		DisputeId:      resp.DisputeId,
		AdditionalBond: "100000000",
	})
	if err == nil {
		t.Fatal("expected error for max tier escalation")
	}
}

// ---------- Settlement Tests ----------

func TestSettleDisputeChallengerWins(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Advance to arbitration and set all arbiters voting for challenger
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// All 3 arbiters vote for challenger (>66.7% majority)
	for _, arb := range dispute.Arbiters {
		_, _ = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
			Arbiter:   arb,
			DisputeId: resp.DisputeId,
			Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
			Reasoning: "Challenger is right",
		})
	}

	// Settle
	settleResp, err := msgSrv.SettleDispute(ctx, &types.MsgSettleDispute{
		Authority: "zrn1authority",
		DisputeId: resp.DisputeId,
	})
	if err != nil {
		t.Fatalf("SettleDispute failed: %v", err)
	}
	if settleResp.Outcome != types.DisputeOutcome_DISPUTE_OUTCOME_CHALLENGER_WINS {
		t.Errorf("expected CHALLENGER_WINS, got %s", settleResp.Outcome.String())
	}

	// Verify dispute settled
	dispute, _ = k.GetDispute(ctx, resp.DisputeId)
	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_SETTLED {
		t.Errorf("expected SETTLED phase, got %s", dispute.Phase.String())
	}
}

func TestSettleDisputeDefenderWins(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// All 3 arbiters vote for defender
	for _, arb := range dispute.Arbiters {
		_, _ = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
			Arbiter:   arb,
			DisputeId: resp.DisputeId,
			Vote:      types.ArbiterDecision_ARBITER_DECISION_DEFENDER,
			Reasoning: "Defender is right",
		})
	}

	settleResp, err := msgSrv.SettleDispute(ctx, &types.MsgSettleDispute{
		Authority: "zrn1authority",
		DisputeId: resp.DisputeId,
	})
	if err != nil {
		t.Fatalf("SettleDispute failed: %v", err)
	}
	if settleResp.Outcome != types.DisputeOutcome_DISPUTE_OUTCOME_DEFENDER_WINS {
		t.Errorf("expected DEFENDER_WINS, got %s", settleResp.Outcome.String())
	}
}

func TestSettleDisputeDraw(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// Split votes: 1 challenger, 1 defender, 1 abstain → no majority
	_, _ = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
		Arbiter:   dispute.Arbiters[0],
		DisputeId: resp.DisputeId,
		Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
		Reasoning: "For challenger",
	})
	_, _ = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
		Arbiter:   dispute.Arbiters[1],
		DisputeId: resp.DisputeId,
		Vote:      types.ArbiterDecision_ARBITER_DECISION_DEFENDER,
		Reasoning: "For defender",
	})
	_, _ = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
		Arbiter:   dispute.Arbiters[2],
		DisputeId: resp.DisputeId,
		Vote:      types.ArbiterDecision_ARBITER_DECISION_ABSTAIN,
		Reasoning: "Abstaining",
	})

	settleResp, err := msgSrv.SettleDispute(ctx, &types.MsgSettleDispute{
		Authority: "zrn1authority",
		DisputeId: resp.DisputeId,
	})
	if err != nil {
		t.Fatalf("SettleDispute failed: %v", err)
	}
	if settleResp.Outcome != types.DisputeOutcome_DISPUTE_OUTCOME_DRAW {
		t.Errorf("expected DRAW, got %s", settleResp.Outcome.String())
	}
}

func TestSettleDisputeTimeout(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// No votes cast → timeout
	settleResp, err := msgSrv.SettleDispute(ctx, &types.MsgSettleDispute{
		Authority: "zrn1authority",
		DisputeId: resp.DisputeId,
	})
	if err != nil {
		t.Fatalf("SettleDispute failed: %v", err)
	}
	if settleResp.Outcome != types.DisputeOutcome_DISPUTE_OUTCOME_TIMED_OUT {
		t.Errorf("expected TIMED_OUT, got %s", settleResp.Outcome.String())
	}

	// Verify bond returned to challenger on timeout
	dispute, _ = k.GetDispute(ctx, resp.DisputeId)
	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_TIMED_OUT {
		t.Errorf("expected TIMED_OUT phase, got %s", dispute.Phase.String())
	}
}

func TestSettleDisputeUnauthorized(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	k.SetDispute(ctx, dispute)

	_, err := msgSrv.SettleDispute(ctx, &types.MsgSettleDispute{
		Authority: testAddr("not-authority"),
		DisputeId: resp.DisputeId,
	})
	if err == nil {
		t.Fatal("expected error for unauthorized settle")
	}
}

// ---------- Bond Distribution Tests ----------

func TestBondDistributionChallengerWins(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// All arbiters vote challenger
	for _, arb := range dispute.Arbiters {
		_, _ = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
			Arbiter: arb, DisputeId: resp.DisputeId,
			Vote: types.ArbiterDecision_ARBITER_DECISION_CHALLENGER, Reasoning: "yes",
		})
	}

	// Note balances before settlement
	challengerBefore := bk.balances[challenger]["uzrn"]

	_, _ = msgSrv.SettleDispute(ctx, &types.MsgSettleDispute{
		Authority: "zrn1authority",
		DisputeId: resp.DisputeId,
	})

	challengerAfter := bk.balances[challenger]["uzrn"]

	// Challenger should have received: winner reward (40% of 50% slash) + refund (50% of bond)
	// Bond=1000000, slash=500000 (50%), winner=200000 (40% of slash), refund=500000
	// So challenger net gain = 200000 + 500000 = 700000 from module
	if challengerAfter <= challengerBefore {
		t.Errorf("expected challenger balance to increase: before=%d, after=%d", challengerBefore, challengerAfter)
	}
}

// ---------- Max Active Disputes Test ----------

func TestMaxActiveDisputes(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	bk.setBalance(challenger, "uzrn", 1_000_000_000)

	// Set max to 2
	params := types.DefaultParams()
	params.MaxActiveDisputes = 2
	k.SetParams(ctx, params)

	for i := 0; i < 2; i++ {
		factID := fmt.Sprintf("fact-%d", i)
		kk.addFact(factID, testAddr(fmt.Sprintf("defender-%d", i)))
		_, err := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
			Challenger: challenger,
			TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
			TargetId:   factID,
			Reason:     "Disputed",
			Bond:       "1000000",
		})
		if err != nil {
			t.Fatalf("InitiateDispute %d failed: %v", i, err)
		}
	}

	// 3rd should fail
	kk.addFact("fact-2", testAddr("defender-2"))
	_, err := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-2",
		Reason:     "Disputed",
		Bond:       "1000000",
	})
	if err == nil {
		t.Fatal("expected error for exceeding max active disputes")
	}
}

// ---------- Phase Transition Tests (BeginBlocker) ----------

func TestPhaseTransitionCommitToReveal(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	// Create dispute with commit phase deadline at block 200
	k.SetDispute(ctx, &types.Dispute{
		Id:               "dispute-1",
		TargetId:         "fact-1",
		Phase:            types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
		Tier:             1,
		EvidenceDeadline: 200,
		VotingDeadline:   1700,
	})

	// Process at block 201 → should advance to reveal
	k.ProcessPhaseTransitions(ctx, 201)

	dispute, _ := k.GetDispute(ctx, "dispute-1")
	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL {
		t.Errorf("expected EVIDENCE_REVEAL, got %s", dispute.Phase.String())
	}
}

func TestPhaseTransitionRevealToArbitration(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	k.SetDispute(ctx, &types.Dispute{
		Id:               "dispute-1",
		TargetId:         "fact-1",
		Phase:            types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL,
		Tier:             1,
		EvidenceDeadline: 300,
		VotingDeadline:   1800,
	})

	k.ProcessPhaseTransitions(ctx, 301)

	dispute, _ := k.GetDispute(ctx, "dispute-1")
	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_ARBITRATION {
		t.Errorf("expected ARBITRATION, got %s", dispute.Phase.String())
	}
}

func TestPhaseTransitionArbitrationToTimeout(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	k.SetDispute(ctx, &types.Dispute{
		Id:             "dispute-1",
		TargetId:       "fact-1",
		Phase:          types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
		Tier:           1,
		Challenger:     testAddr("challenger"),
		Bond:           "1000000",
		VotingDeadline: 500,
	})

	// Process past voting deadline with no votes → timeout
	k.ProcessPhaseTransitions(ctx, 501)

	dispute, _ := k.GetDispute(ctx, "dispute-1")
	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_TIMED_OUT {
		t.Errorf("expected TIMED_OUT, got %s", dispute.Phase.String())
	}
}

// ---------- Full Lifecycle Test ----------

func TestFullLifecycle_InitiateCommitRevealVoteSettle(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	// 1. Initiate dispute
	initResp, err := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "The fact is wrong",
		Bond:       "1000000",
	})
	if err != nil {
		t.Fatalf("InitiateDispute: %v", err)
	}

	// 2. Commit evidence
	content := "My evidence that the fact is wrong"
	nonce := "secret-nonce-42"
	h := sha256.Sum256([]byte(content + nonce))
	commitHash := hex.EncodeToString(h[:])

	_, err = msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      challenger,
		DisputeId:      initResp.DisputeId,
		CommitmentHash: commitHash,
	})
	if err != nil {
		t.Fatalf("CommitEvidence: %v", err)
	}

	// 3. Advance to reveal phase
	dispute, _ := k.GetDispute(ctx, initResp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL
	dispute.EvidenceDeadline = 1000
	k.SetDispute(ctx, dispute)

	// 4. Reveal evidence
	_, err = msgSrv.RevealEvidence(ctx, &types.MsgRevealEvidence{
		Submitter: challenger,
		DisputeId: initResp.DisputeId,
		Content:   content,
		Nonce:     nonce,
	})
	if err != nil {
		t.Fatalf("RevealEvidence: %v", err)
	}

	// 5. Advance to arbitration
	dispute, _ = k.GetDispute(ctx, initResp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_ARBITRATION
	dispute.VotingDeadline = 2000
	k.SetDispute(ctx, dispute)

	// 6. All arbiters vote challenger
	dispute, _ = k.GetDispute(ctx, initResp.DisputeId)
	for _, arb := range dispute.Arbiters {
		_, err = msgSrv.ArbiterVote(ctx, &types.MsgArbiterVote{
			Arbiter:   arb,
			DisputeId: initResp.DisputeId,
			Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
			Reasoning: "Evidence is convincing",
		})
		if err != nil {
			t.Fatalf("ArbiterVote by %s: %v", arb, err)
		}
	}

	// 7. Settle
	settleResp, err := msgSrv.SettleDispute(ctx, &types.MsgSettleDispute{
		Authority: "zrn1authority",
		DisputeId: initResp.DisputeId,
	})
	if err != nil {
		t.Fatalf("SettleDispute: %v", err)
	}
	if settleResp.Outcome != types.DisputeOutcome_DISPUTE_OUTCOME_CHALLENGER_WINS {
		t.Errorf("expected CHALLENGER_WINS, got %s", settleResp.Outcome.String())
	}

	// 8. Verify final state
	dispute, _ = k.GetDispute(ctx, initResp.DisputeId)
	if dispute.Phase != types.DisputePhase_DISPUTE_PHASE_SETTLED {
		t.Errorf("expected SETTLED, got %s", dispute.Phase.String())
	}

	// Verify evidence count
	if dispute.EvidenceCount != 1 {
		t.Errorf("expected evidence count 1, got %d", dispute.EvidenceCount)
	}

	// Verify dispute no longer active
	active := k.GetActiveDisputes(ctx)
	for _, a := range active {
		if a.Id == initResp.DisputeId {
			t.Error("settled dispute should not be in active list")
		}
	}
}

// ---------- Query Server Tests ----------

func TestQueryDispute(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetDispute(ctx, &types.Dispute{
		Id:       "q-dispute-1",
		TargetId: "fact-1",
		Bond:     "5000000",
		Phase:    types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
	})

	resp, err := qs.Dispute(ctx, &types.QueryDisputeRequest{Id: "q-dispute-1"})
	if err != nil {
		t.Fatalf("Query Dispute failed: %v", err)
	}
	if resp.Dispute.Bond != "5000000" {
		t.Errorf("expected bond 5000000, got %s", resp.Dispute.Bond)
	}
}

func TestQueryDisputeNotFound(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Dispute(ctx, &types.QueryDisputeRequest{Id: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent dispute")
	}
}

func TestQueryParams(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("Query Params failed: %v", err)
	}
	if resp.Params.MaxActiveDisputes != 100 {
		t.Errorf("expected max active disputes 100, got %d", resp.Params.MaxActiveDisputes)
	}
}

// ---------- ValidateBasic Tests ----------

func TestMsgInitiateDisputeValidateBasic(t *testing.T) {
	msg := &types.MsgInitiateDispute{
		Challenger: testAddr("challenger"),
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Invalid fact",
		Bond:       "1000000",
	}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("ValidateBasic should pass: %v", err)
	}

	// Empty reason
	msg2 := &types.MsgInitiateDispute{
		Challenger: testAddr("challenger"),
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "",
		Bond:       "1000000",
	}
	if err := msg2.ValidateBasic(); err == nil {
		t.Error("ValidateBasic should fail for empty reason")
	}

	// Unspecified target type
	msg3 := &types.MsgInitiateDispute{
		Challenger: testAddr("challenger"),
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_UNSPECIFIED,
		TargetId:   "fact-1",
		Reason:     "Invalid",
		Bond:       "1000000",
	}
	if err := msg3.ValidateBasic(); err == nil {
		t.Error("ValidateBasic should fail for unspecified target type")
	}
}

func TestGenesisValidation(t *testing.T) {
	gs := types.DefaultGenesis()
	if err := gs.Validate(); err != nil {
		t.Errorf("default genesis should be valid: %v", err)
	}

	// Duplicate dispute IDs
	gs2 := &types.GenesisState{
		Params: types.DefaultParams(),
		Disputes: []*types.Dispute{
			{Id: "dup"},
			{Id: "dup"},
		},
	}
	if err := gs2.Validate(); err == nil {
		t.Error("genesis with duplicate dispute IDs should be invalid")
	}
}

// ---------- Arbiter Selection Tests ----------

func TestRevealWithoutCommit(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Advance to reveal phase without committing
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL
	dispute.EvidenceDeadline = 1000
	k.SetDispute(ctx, dispute)

	// Try to reveal without having committed
	_, err := msgSrv.RevealEvidence(ctx, &types.MsgRevealEvidence{
		Submitter: challenger,
		DisputeId: resp.DisputeId,
		Content:   "some evidence",
		Nonce:     "some-nonce",
	})
	if err == nil {
		t.Fatal("expected error for reveal without commit")
	}
}

func TestRevealAfterDeadline(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Commit evidence
	content := "Evidence content"
	nonce := "nonce123"
	h := sha256.Sum256([]byte(content + nonce))
	commitHash := hex.EncodeToString(h[:])
	_, _ = msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      challenger,
		DisputeId:      resp.DisputeId,
		CommitmentHash: commitHash,
	})

	// Advance to reveal phase with deadline already passed
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.Phase = types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL
	dispute.EvidenceDeadline = 50 // deadline in the past (current block = 100)
	k.SetDispute(ctx, dispute)

	// Try to reveal after deadline
	_, err := msgSrv.RevealEvidence(ctx, &types.MsgRevealEvidence{
		Submitter: challenger,
		DisputeId: resp.DisputeId,
		Content:   content,
		Nonce:     nonce,
	})
	if err == nil {
		t.Fatal("expected error for reveal after deadline")
	}
}

func TestEscalateInsufficientBond(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 100_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Set up for escalation
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.CreatedAt = 1
	k.SetDispute(ctx, dispute)

	escCtx := ctx.WithBlockHeight(600)

	// Try to escalate with zero additional bond
	_, err := msgSrv.EscalateDispute(escCtx, &types.MsgEscalateDispute{
		Requester:      challenger,
		DisputeId:      resp.DisputeId,
		AdditionalBond: "0",
	})
	if err == nil {
		t.Fatal("expected error for zero additional bond")
	}
}

func TestCommitEvidenceAfterDeadline(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	bk.setBalance(challenger, "uzrn", 10_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	// Set evidence deadline to past
	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.EvidenceDeadline = 50 // past (current block = 100)
	k.SetDispute(ctx, dispute)

	_, err := msgSrv.CommitEvidence(ctx, &types.MsgCommitEvidence{
		Submitter:      challenger,
		DisputeId:      resp.DisputeId,
		CommitmentHash: "abc123",
	})
	if err == nil {
		t.Fatal("expected error for commit after deadline")
	}
}

func TestEscalateDisputeNonParty(t *testing.T) {
	k, ctx, bk, _, kk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	challenger := testAddr("challenger")
	defender := testAddr("defender")
	outsider := testAddr("outsider")
	bk.setBalance(challenger, "uzrn", 100_000_000)
	bk.setBalance(outsider, "uzrn", 100_000_000)
	kk.addFact("fact-1", defender)

	resp, _ := msgSrv.InitiateDispute(ctx, &types.MsgInitiateDispute{
		Challenger: challenger,
		TargetType: types.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		TargetId:   "fact-1",
		Reason:     "Disputed",
		Bond:       "1000000",
	})

	dispute, _ := k.GetDispute(ctx, resp.DisputeId)
	dispute.CreatedAt = 1
	k.SetDispute(ctx, dispute)

	escCtx := ctx.WithBlockHeight(600)

	// Outsider tries to escalate
	_, err := msgSrv.EscalateDispute(escCtx, &types.MsgEscalateDispute{
		Requester:      outsider,
		DisputeId:      resp.DisputeId,
		AdditionalBond: "10000000",
	})
	if err == nil {
		t.Fatal("expected error for non-party escalation")
	}
}

func TestArbiterSelectionExcludesParties(t *testing.T) {
	k, ctx, _, sk, _ := setupKeeper(t)

	challenger := testAddr("challenger")
	defender := testAddr("defender")

	// Add challenger and defender to validators
	sk.validators = append(sk.validators, challenger, defender)

	arbiters, err := k.SelectArbiters(ctx, 3, challenger, defender, 100)
	if err != nil {
		t.Fatalf("SelectArbiters failed: %v", err)
	}
	if len(arbiters) != 3 {
		t.Fatalf("expected 3 arbiters, got %d", len(arbiters))
	}

	for _, arb := range arbiters {
		if arb == challenger {
			t.Error("challenger should not be selected as arbiter")
		}
		if arb == defender {
			t.Error("defender should not be selected as arbiter")
		}
	}
}

// ---------- Tests: UpdateParams ----------

func TestUpdateParams(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.MaxActiveDisputes = 200
	newParams.EscalationDelay = 1000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MaxActiveDisputes != 200 {
		t.Errorf("expected MaxActiveDisputes 200, got %d", got.MaxActiveDisputes)
	}
	if got.EscalationDelay != 1000 {
		t.Errorf("expected EscalationDelay 1000, got %d", got.EscalationDelay)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
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
	k, ctx, _, _, _ := setupKeeper(t)
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
	k, ctx, _, _, _ := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	badParams := types.DefaultParams()
	badParams.TierConfigs = nil // invalid: at least one tier required

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}

// ---------- Tally/Settlement Edge Cases ----------

func TestTallyVotesNoVotes(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	dispute := &types.Dispute{
		Id:       "tally-no-votes",
		TargetId: "fact-1",
		Tier:     1,
		Phase:    types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
		Arbiters: []string{testAddr("arb1"), testAddr("arb2"), testAddr("arb3")},
	}
	k.SetDispute(ctx, dispute)

	// No votes cast at all
	outcome := k.TallyVotes(ctx, dispute)
	if outcome != types.DisputeOutcome_DISPUTE_OUTCOME_TIMED_OUT {
		t.Errorf("expected TIMED_OUT with no votes, got %s", outcome.String())
	}
}

func TestTallyVotesAllAbstain(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	dispute := &types.Dispute{
		Id:       "tally-all-abstain",
		TargetId: "fact-1",
		Tier:     1,
		Phase:    types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
		Arbiters: []string{testAddr("arb1"), testAddr("arb2"), testAddr("arb3")},
	}
	k.SetDispute(ctx, dispute)

	// All 3 arbiters abstain
	for i, arb := range dispute.Arbiters {
		k.SetVote(ctx, &types.DisputeVote{
			DisputeId: dispute.Id,
			Arbiter:   arb,
			Vote:      types.ArbiterDecision_ARBITER_DECISION_ABSTAIN,
			Stake:     "1",
			Rationale: fmt.Sprintf("abstain-%d", i),
		})
	}

	outcome := k.TallyVotes(ctx, dispute)
	if outcome != types.DisputeOutcome_DISPUTE_OUTCOME_DRAW {
		t.Errorf("expected DRAW when all abstain (totalVotingWeight=0), got %s", outcome.String())
	}
}

func TestTallyVotesTiedVotes(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	dispute := &types.Dispute{
		Id:       "tally-tied",
		TargetId: "fact-1",
		Tier:     1,
		Phase:    types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
		Arbiters: []string{testAddr("arb1"), testAddr("arb2")},
	}
	k.SetDispute(ctx, dispute)

	// 1 vote challenger, 1 vote defender with equal stake
	k.SetVote(ctx, &types.DisputeVote{
		DisputeId: dispute.Id,
		Arbiter:   dispute.Arbiters[0],
		Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
		Stake:     "1000",
	})
	k.SetVote(ctx, &types.DisputeVote{
		DisputeId: dispute.Id,
		Arbiter:   dispute.Arbiters[1],
		Vote:      types.ArbiterDecision_ARBITER_DECISION_DEFENDER,
		Stake:     "1000",
	})

	// 50/50 split: neither reaches 66.67% majority → DRAW
	outcome := k.TallyVotes(ctx, dispute)
	if outcome != types.DisputeOutcome_DISPUTE_OUTCOME_DRAW {
		t.Errorf("expected DRAW for tied votes, got %s", outcome.String())
	}
}

func TestDistributeSettlementZeroBond(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	dispute := &types.Dispute{
		Id:         "zero-bond",
		TargetId:   "fact-1",
		Challenger: testAddr("challenger"),
		Defender:   testAddr("defender"),
		Bond:       "0",
		Outcome:    types.DisputeOutcome_DISPUTE_OUTCOME_CHALLENGER_WINS,
		Phase:      types.DisputePhase_DISPUTE_PHASE_SETTLED,
	}
	k.SetDispute(ctx, dispute)

	err := k.DistributeSettlement(ctx, dispute)
	if err != nil {
		t.Fatalf("expected nil error for zero bond, got %v", err)
	}
}

func TestDistributeSettlementDefenderWins(t *testing.T) {
	k, ctx, bk, _, _ := setupKeeper(t)

	defender := testAddr("defender")
	challenger := testAddr("challenger")

	dispute := &types.Dispute{
		Id:         "def-wins",
		TargetId:   "fact-1",
		Challenger: challenger,
		Defender:   defender,
		Bond:       "1000000",
		Outcome:    types.DisputeOutcome_DISPUTE_OUTCOME_DEFENDER_WINS,
		Phase:      types.DisputePhase_DISPUTE_PHASE_SETTLED,
		Tier:       1,
		Arbiters:   []string{testAddr("arb1"), testAddr("arb2"), testAddr("arb3")},
	}
	k.SetDispute(ctx, dispute)

	// Set up votes for the arbiters (defender wins)
	for _, arb := range dispute.Arbiters {
		k.SetVote(ctx, &types.DisputeVote{
			DisputeId: dispute.Id,
			Arbiter:   arb,
			Vote:      types.ArbiterDecision_ARBITER_DECISION_DEFENDER,
			Stake:     "1",
		})
	}

	// Fund the module so distribution can happen
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 2_000_000}

	defenderBefore := bk.balances[defender]["uzrn"]
	err := k.DistributeSettlement(ctx, dispute)
	if err != nil {
		t.Fatalf("DistributeSettlement failed: %v", err)
	}

	defenderAfter := bk.balances[defender]["uzrn"]
	if defenderAfter <= defenderBefore {
		t.Errorf("expected defender balance to increase: before=%d, after=%d", defenderBefore, defenderAfter)
	}
}

// ---------- Phase Transition Tests (Extended) ----------

func TestProcessPhaseTransitions_CommitToReveal(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	k.SetDispute(ctx, &types.Dispute{
		Id:               "pt-commit-reveal",
		TargetId:         "fact-1",
		Phase:            types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
		Tier:             1,
		EvidenceDeadline: 150,
		VotingDeadline:   2000,
	})

	// Block 150: should NOT transition (not past deadline)
	k.ProcessPhaseTransitions(ctx, 150)
	d, _ := k.GetDispute(ctx, "pt-commit-reveal")
	if d.Phase != types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT {
		t.Errorf("should still be EVIDENCE_COMMIT at deadline block, got %s", d.Phase.String())
	}

	// Block 151: should transition to EVIDENCE_REVEAL
	k.ProcessPhaseTransitions(ctx, 151)
	d, _ = k.GetDispute(ctx, "pt-commit-reveal")
	if d.Phase != types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL {
		t.Errorf("expected EVIDENCE_REVEAL after deadline, got %s", d.Phase.String())
	}
}

func TestProcessPhaseTransitions_RevealToArbitration(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	k.SetDispute(ctx, &types.Dispute{
		Id:               "pt-reveal-arb",
		TargetId:         "fact-1",
		Phase:            types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL,
		Tier:             1,
		EvidenceDeadline: 400,
		VotingDeadline:   2000,
	})

	// Block 400: should NOT transition
	k.ProcessPhaseTransitions(ctx, 400)
	d, _ := k.GetDispute(ctx, "pt-reveal-arb")
	if d.Phase != types.DisputePhase_DISPUTE_PHASE_EVIDENCE_REVEAL {
		t.Errorf("should still be EVIDENCE_REVEAL at deadline block, got %s", d.Phase.String())
	}

	// Block 401: should transition to ARBITRATION
	k.ProcessPhaseTransitions(ctx, 401)
	d, _ = k.GetDispute(ctx, "pt-reveal-arb")
	if d.Phase != types.DisputePhase_DISPUTE_PHASE_ARBITRATION {
		t.Errorf("expected ARBITRATION after reveal deadline, got %s", d.Phase.String())
	}
}

func TestProcessPhaseTransitions_ArbitrationAutoSettle(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	arbiters := []string{testAddr("arb1"), testAddr("arb2"), testAddr("arb3")}
	challenger := testAddr("challenger")

	k.SetDispute(ctx, &types.Dispute{
		Id:             "pt-arb-settle",
		TargetId:       "fact-1",
		Phase:          types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
		Tier:           1,
		Challenger:     challenger,
		Defender:       testAddr("defender"),
		Bond:           "1000000",
		VotingDeadline: 600,
		Arbiters:       arbiters,
	})

	// All arbiters vote for challenger
	for _, arb := range arbiters {
		k.SetVote(ctx, &types.DisputeVote{
			DisputeId: "pt-arb-settle",
			Arbiter:   arb,
			Vote:      types.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
			Stake:     "1",
		})
	}

	// Fund the module account for settlement distribution
	k.ProcessPhaseTransitions(ctx, 601)

	d, _ := k.GetDispute(ctx, "pt-arb-settle")
	if d.Phase != types.DisputePhase_DISPUTE_PHASE_SETTLED {
		t.Errorf("expected SETTLED after auto-settle, got %s", d.Phase.String())
	}
	if d.Outcome != types.DisputeOutcome_DISPUTE_OUTCOME_CHALLENGER_WINS {
		t.Errorf("expected CHALLENGER_WINS outcome, got %s", d.Outcome.String())
	}
}

// ---------- Evidence/Vote CRUD Tests ----------

func TestSetGetMultipleEvidence(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	for i := 0; i < 3; i++ {
		k.SetEvidence(ctx, &types.DisputeEvidence{
			Id:          fmt.Sprintf("ev-%d", i),
			DisputeId:   "dispute-multi-ev",
			Submitter:   testAddr(fmt.Sprintf("submitter-%d", i)),
			Side:        "challenger",
			Content:     fmt.Sprintf("evidence content %d", i),
			SubmittedAt: uint64(100 + i),
		})
	}

	evidence := k.GetEvidenceByDispute(ctx, "dispute-multi-ev")
	if len(evidence) != 3 {
		t.Fatalf("expected 3 evidence items, got %d", len(evidence))
	}

	// Verify each evidence is present
	ids := map[string]bool{}
	for _, ev := range evidence {
		ids[ev.Id] = true
	}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("ev-%d", i)
		if !ids[id] {
			t.Errorf("evidence %s not found in results", id)
		}
	}
}

func TestGetCommitmentNotFound(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	_, found := k.GetCommitment(ctx, "nonexistent-dispute", "nonexistent-submitter")
	if found {
		t.Fatal("expected commitment not found for nonexistent dispute+submitter")
	}
}

func TestGetVotesByDisputeEmpty(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	votes := k.GetVotesByDispute(ctx, "no-votes-dispute")
	if len(votes) != 0 {
		t.Errorf("expected 0 votes for nonexistent dispute, got %d", len(votes))
	}
}

// ---------- Query Server Tests (Extended) ----------

func TestQueryDisputesByTarget(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	// Create 2 disputes for the same target
	k.SetDispute(ctx, &types.Dispute{
		Id:       "target-q-1",
		TargetId: "shared-fact",
		Phase:    types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
	})
	k.SetDispute(ctx, &types.Dispute{
		Id:       "target-q-2",
		TargetId: "shared-fact",
		Phase:    types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
	})

	resp, err := qs.DisputesByTarget(ctx, &types.QueryByTargetRequest{TargetId: "shared-fact"})
	if err != nil {
		t.Fatalf("DisputesByTarget failed: %v", err)
	}
	if len(resp.Disputes) != 2 {
		t.Errorf("expected 2 disputes for target, got %d", len(resp.Disputes))
	}
}

func TestQueryActiveDisputes(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	// 2 active disputes
	k.SetDispute(ctx, &types.Dispute{
		Id:       "qa-active-1",
		TargetId: "fact-a",
		Phase:    types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT,
	})
	k.SetDispute(ctx, &types.Dispute{
		Id:       "qa-active-2",
		TargetId: "fact-b",
		Phase:    types.DisputePhase_DISPUTE_PHASE_ARBITRATION,
	})
	// 1 settled dispute (not active)
	k.SetDispute(ctx, &types.Dispute{
		Id:       "qa-settled-1",
		TargetId: "fact-c",
		Phase:    types.DisputePhase_DISPUTE_PHASE_SETTLED,
	})

	resp, err := qs.ActiveDisputes(ctx, &types.QueryActiveRequest{})
	if err != nil {
		t.Fatalf("ActiveDisputes query failed: %v", err)
	}
	if len(resp.Disputes) != 2 {
		t.Errorf("expected 2 active disputes, got %d", len(resp.Disputes))
	}

	// Verify none of them are the settled one
	for _, d := range resp.Disputes {
		if d.Id == "qa-settled-1" {
			t.Error("settled dispute should not appear in active query results")
		}
	}
}

func TestQueryParamsNilRequest(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Params(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil Params request")
	}
}

// ---------- Arbiter Selection Tests (Extended) ----------

func TestSelectArbitersDeterministic(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	challenger := testAddr("challenger")
	defender := testAddr("defender")

	arbiters1, err := k.SelectArbiters(ctx, 3, challenger, defender, 100)
	if err != nil {
		t.Fatalf("first SelectArbiters failed: %v", err)
	}

	arbiters2, err := k.SelectArbiters(ctx, 3, challenger, defender, 100)
	if err != nil {
		t.Fatalf("second SelectArbiters failed: %v", err)
	}

	if len(arbiters1) != len(arbiters2) {
		t.Fatalf("deterministic selection returned different lengths: %d vs %d", len(arbiters1), len(arbiters2))
	}
	for i := range arbiters1 {
		if arbiters1[i] != arbiters2[i] {
			t.Errorf("arbiter[%d] differs: %s vs %s", i, arbiters1[i], arbiters2[i])
		}
	}
}

func TestSelectArbitersInsufficientValidators(t *testing.T) {
	k, ctx, _, sk, _ := setupKeeper(t)

	// Override validators to only have 2 (plus challenger and defender makes it even worse)
	sk.validators = []string{testAddr("val1"), testAddr("val2")}

	challenger := testAddr("challenger")
	defender := testAddr("defender")

	// Request 3 arbiters but only 2 validators available (neither is challenger/defender)
	_, err := k.SelectArbiters(ctx, 3, challenger, defender, 100)
	if err == nil {
		t.Fatal("expected error for insufficient validators")
	}
}

// ---------- Counter and ID Tests ----------

func TestGetNextDisputeIDSequential(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	for expected := uint64(1); expected <= 5; expected++ {
		got := k.GetNextDisputeID(ctx)
		if got != expected {
			t.Errorf("expected dispute ID %d, got %d", expected, got)
		}
	}
}

func TestGenerateDisputeIDDeterministic(t *testing.T) {
	// Same inputs produce the same ID
	id1 := keeper.GenerateDisputeID("fact-1", "challenger-addr", 100)
	id2 := keeper.GenerateDisputeID("fact-1", "challenger-addr", 100)
	if id1 != id2 {
		t.Errorf("expected same IDs for same inputs, got %s and %s", id1, id2)
	}

	// Different inputs produce different IDs
	id3 := keeper.GenerateDisputeID("fact-2", "challenger-addr", 100)
	if id1 == id3 {
		t.Errorf("expected different IDs for different inputs, got same: %s", id1)
	}

	id4 := keeper.GenerateDisputeID("fact-1", "challenger-addr", 200)
	if id1 == id4 {
		t.Errorf("expected different IDs for different block heights, got same: %s", id1)
	}
}

// ---------- Delete and Iteration Tests ----------

func TestDeleteDisputeCleansIndices(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	dispute := &types.Dispute{
		Id:       "del-idx-1",
		TargetId: "fact-del",
		Phase:    types.DisputePhase_DISPUTE_PHASE_EVIDENCE_COMMIT, // active
	}
	k.SetDispute(ctx, dispute)

	// Verify it appears in both indices before deletion
	byTarget := k.GetDisputesByTarget(ctx, "fact-del")
	if len(byTarget) != 1 {
		t.Fatalf("expected 1 dispute by target before delete, got %d", len(byTarget))
	}
	active := k.GetActiveDisputes(ctx)
	foundActive := false
	for _, a := range active {
		if a.Id == "del-idx-1" {
			foundActive = true
		}
	}
	if !foundActive {
		t.Fatal("expected dispute in active index before delete")
	}

	// Delete and verify indices are cleaned
	k.DeleteDispute(ctx, dispute)

	byTarget = k.GetDisputesByTarget(ctx, "fact-del")
	if len(byTarget) != 0 {
		t.Errorf("expected 0 disputes by target after delete, got %d", len(byTarget))
	}

	active = k.GetActiveDisputes(ctx)
	for _, a := range active {
		if a.Id == "del-idx-1" {
			t.Error("deleted dispute should not appear in active index")
		}
	}

	_, found := k.GetDispute(ctx, "del-idx-1")
	if found {
		t.Error("deleted dispute should not be found by GetDispute")
	}
}
