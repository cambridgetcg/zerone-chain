package keeper_test

import (
	"bytes"
	"context"
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

	"github.com/zerone-chain/zerone/x/tree/keeper"
	"github.com/zerone-chain/zerone/x/tree/types"
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

func (m *mockBankKeeper) SendCoins(_ context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error {
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

// ---------- NoOpResearchFundDepositor ----------

type noOpResearchFundDepositor struct {
	bk *mockBankKeeper
}

func (d *noOpResearchFundDepositor) DepositToResearchFund(_ context.Context, sourceModule string, amount sdk.Coins) error {
	return d.bk.SendCoinsFromModuleToModule(context.Background(), sourceModule, "research_fund", amount)
}

// ---------- Mock ChannelsKeeper ----------

type mockChannelsKeeper struct {
	channels map[string]channelInfo
}

type channelInfo struct {
	payer     string
	provider  string
	available string
	status    string
}

func newMockChannelsKeeper() *mockChannelsKeeper {
	return &mockChannelsKeeper{channels: make(map[string]channelInfo)}
}

func (m *mockChannelsKeeper) GetChannelInfo(_ context.Context, channelId string) (payer, provider, available, status string, found bool) {
	ch, ok := m.channels[channelId]
	if !ok {
		return "", "", "", "", false
	}
	return ch.payer, ch.provider, ch.available, ch.status, true
}

func (m *mockChannelsKeeper) SpendFromChannel(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

// ---------- Test Setup ----------

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()
	initAddresses()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	err := stateStore.LoadLatestVersion()
	if err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	mockBK := newMockBankKeeper()
	noop := &noOpResearchFundDepositor{bk: mockBK}

	k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), mockBK, "zrn1authority", noop)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	return k, ctx, mockBK
}

func testAddr(name string) string {
	return sdk.AccAddress([]byte(name + "_address_padding")).String()
}

var (
	founder1  string
	founder2  string
	agent1    string
	agent2    string
	reviewer1 string
)

func initAddresses() {
	founder1 = testAddr("founder1")
	founder2 = testAddr("founder2")
	agent1 = testAddr("agent001")
	agent2 = testAddr("agent002")
	reviewer1 = testAddr("reviewer")
}

// ---------- Params Tests ----------

func TestDefaultParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	params := k.GetParams(ctx)
	if params.MaxTasksPerProject != 200 {
		t.Errorf("expected MaxTasksPerProject=200, got %d", params.MaxTasksPerProject)
	}
	if params.MaxRejections != 3 {
		t.Errorf("expected MaxRejections=3, got %d", params.MaxRejections)
	}
}

func TestSetGetParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	custom := types.Params{
		MinBudget:              "5000000",
		MaxTasksPerProject:     50,
		MaxContributors:        10,
		MaxApplications:        20,
		TaskDeadlineMinBlocks:  500,
		TaskDeadlineMaxBlocks:  500000,
		MaxRejections:          5,
		SeedExpiryBlocks:       100000,
		MinContributorsToStart: 2,
	}
	k.SetParams(ctx, &custom)
	got := k.GetParams(ctx)
	if got.MaxTasksPerProject != 50 {
		t.Errorf("expected MaxTasksPerProject=50, got %d", got.MaxTasksPerProject)
	}
	if got.MinContributorsToStart != 2 {
		t.Errorf("expected MinContributorsToStart=2, got %d", got.MinContributorsToStart)
	}
}

// ---------- Project CRUD Tests ----------

func TestSetGetProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	project := types.ProductProject{
		Id:              "proj-1",
		Name:            "Test Project",
		Phase:           string(types.PhaseSeed),
		Founder:         founder1,
		KnowledgeDomain: "mathematics",
		Budget:          "1000000",
		Spent:           "0",
		TaskIds:         []string{},
		ServiceIds:      []string{},
		Contributors:    []*types.ContributorRecord{},
	}
	k.SetProject(ctx, &project)

	got, found := k.GetProject(ctx, "proj-1")
	if !found {
		t.Fatal("project not found")
	}
	if got.Name != "Test Project" {
		t.Errorf("expected name 'Test Project', got %q", got.Name)
	}
	if got.Founder != founder1 {
		t.Errorf("expected founder %s, got %s", founder1, got.Founder)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	_, found := k.GetProject(ctx, "nonexistent")
	if found {
		t.Error("expected not found")
	}
}

func TestGetProjectsByFounder(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	for i := 0; i < 3; i++ {
		k.SetProject(ctx, &types.ProductProject{Id: "proj-" + testAddr("p" + string(rune('a'+i))), Founder: founder1})
	}
	projects := k.GetProjectsByFounder(ctx, founder1)
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
	}
}

func TestDeleteProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetProject(ctx, &types.ProductProject{Id: "proj-del", Founder: founder1})
	k.DeleteProject(ctx, &types.ProductProject{Id: "proj-del", Founder: founder1})
	_, found := k.GetProject(ctx, "proj-del")
	if found {
		t.Error("expected project to be deleted")
	}
}

// ---------- Task CRUD Tests ----------

func TestSetGetTask(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	task := types.ProjectTask{
		Id:           "task-1",
		ProjectId:    "proj-1",
		Title:        "Test Task",
		Status:       string(types.TaskOpen),
		BountyAmount: "500000",
	}
	k.SetTask(ctx, &task)

	got, found := k.GetTask(ctx, "task-1")
	if !found {
		t.Fatal("task not found")
	}
	if got.Title != "Test Task" {
		t.Errorf("expected title 'Test Task', got %q", got.Title)
	}
}

func TestGetTasksByProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetTask(ctx, &types.ProjectTask{Id: "task-1", ProjectId: "proj-1"})
	k.SetTask(ctx, &types.ProjectTask{Id: "task-2", ProjectId: "proj-1"})
	k.SetTask(ctx, &types.ProjectTask{Id: "task-3", ProjectId: "proj-2"})
	tasks := k.GetTasksByProject(ctx, "proj-1")
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for proj-1, got %d", len(tasks))
	}
}

// ---------- Seed CRUD Tests ----------

func TestSetGetSeed(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	seed := types.OpportunitySeed{
		Id:              "seed-1",
		ProjectId:       "proj-1",
		KnowledgeDomain: "physics",
		Status:          string(types.SeedDetected),
		ExpiresAtBlock:  200,
	}
	k.SetSeed(ctx, &seed)

	got, found := k.GetSeed(ctx, "seed-1")
	if !found {
		t.Fatal("seed not found")
	}
	if got.KnowledgeDomain != "physics" {
		t.Errorf("expected domain 'physics', got %q", got.KnowledgeDomain)
	}
}

func TestGetExpiredSeeds(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetSeed(ctx, &types.OpportunitySeed{Id: "seed-1", ExpiresAtBlock: 50, Status: string(types.SeedDetected)})
	k.SetSeed(ctx, &types.OpportunitySeed{Id: "seed-2", ExpiresAtBlock: 200, Status: string(types.SeedDetected)})
	k.SetSeed(ctx, &types.OpportunitySeed{Id: "seed-3", ExpiresAtBlock: 100, Status: string(types.SeedDetected)})

	expired := k.GetExpiredSeeds(ctx, 100)
	if len(expired) != 2 {
		t.Errorf("expected 2 expired seeds at block 100, got %d", len(expired))
	}
}

// ---------- Service CRUD Tests ----------

func TestSetGetService(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	service := types.ServiceLeaf{
		Id:           "svc-1",
		Name:         "Test Service",
		Status:       string(types.ServiceActive),
		TotalCalls:   "0",
		TotalRevenue: "0",
		PricePerCall: "100000",
	}
	k.SetService(ctx, &service)

	got, found := k.GetService(ctx, "svc-1")
	if !found {
		t.Fatal("service not found")
	}
	if got.Name != "Test Service" {
		t.Errorf("expected name 'Test Service', got %q", got.Name)
	}
	if got.PricePerCall != "100000" {
		t.Errorf("expected price 100000, got %s", got.PricePerCall)
	}
}

// ---------- Phase Transition Tests ----------

func TestValidPhaseTransitions(t *testing.T) {
	cases := []struct {
		from, to types.ProjectPhase
		valid    bool
	}{
		{types.PhaseSeed, types.PhaseSprout, true},
		{types.PhaseSprout, types.PhaseGrowing, true},
		{types.PhaseGrowing, types.PhaseMature, true},
		{types.PhaseMature, types.PhaseFruiting, true},
		{types.PhaseFruiting, types.PhaseSeeding, true},
		{types.PhaseSeeding, types.PhaseDormant, true},
		{types.PhaseDormant, types.PhaseWithered, true},
		// Invalid transitions
		{types.PhaseSeed, types.PhaseMature, false},
		{types.PhaseWithered, types.PhaseSeed, false},
		{types.PhaseFruiting, types.PhaseSprout, false},
	}
	for _, tc := range cases {
		got := keeper.IsValidPhaseTransition(tc.from, tc.to)
		if got != tc.valid {
			t.Errorf("phase %s -> %s: expected %v, got %v", string(tc.from), string(tc.to), tc.valid, got)
		}
	}
}

// ---------- Message Server Tests ----------

func TestCreateProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, err := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator:     founder1,
		Title:       "My Project",
		Description: "A test project",
		Domain:      "computer_science",
		Budget:      "5000000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProjectId == "" {
		t.Fatal("expected non-empty project_id")
	}

	project, found := k.GetProject(ctx, resp.ProjectId)
	if !found {
		t.Fatal("project not found after creation")
	}
	if project.Name != "My Project" {
		t.Errorf("expected name 'My Project', got %q", project.Name)
	}
	if project.Founder != founder1 {
		t.Errorf("expected founder %s, got %s", founder1, project.Founder)
	}
	if project.Phase != string(types.PhaseSeed) {
		t.Errorf("expected phase 'seed', got %s", project.Phase)
	}
	if len(project.Contributors) != 1 {
		t.Fatalf("expected 1 contributor, got %d", len(project.Contributors))
	}
	if project.Contributors[0].Role != string(types.RoleFounder) {
		t.Errorf("expected contributor role 'founder', got %s", project.Contributors[0].Role)
	}
}

func TestProposeProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, err := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Proposable",
		Domain:  "math",
		Budget:  "1000000",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = msgServer.ProposeProject(ctx, &types.MsgProposeProject{
		Proposer:  founder1,
		ProjectId: resp.ProjectId,
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}

	project, _ := k.GetProject(ctx, resp.ProjectId)
	if project.Phase != string(types.PhaseSprout) {
		t.Errorf("expected phase 'sprout', got %s", project.Phase)
	}
}

func TestProposeProject_NotFounder(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Test",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.ProposeProject(ctx, &types.MsgProposeProject{
		Proposer:  agent1,
		ProjectId: resp.ProjectId,
	})
	if err == nil {
		t.Fatal("expected error for non-founder")
	}
}

func TestStartDevelopment(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Dev Project",
		Domain:  "physics",
		Budget:  "2000000",
	})
	project, _ := k.GetProject(ctx, resp.ProjectId)
	project.Phase = string(types.PhaseSprout)
	k.SetProject(ctx, project)

	_, err := msgServer.StartDevelopment(ctx, &types.MsgStartDevelopment{
		Authority: founder1,
		ProjectId: resp.ProjectId,
	})
	if err != nil {
		t.Fatalf("start development: %v", err)
	}

	project, _ = k.GetProject(ctx, resp.ProjectId)
	if project.Phase != string(types.PhaseGrowing) {
		t.Errorf("expected phase 'growing', got %s", project.Phase)
	}
}

func TestStartDevelopment_InvalidPhase(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Seed Phase",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.StartDevelopment(ctx, &types.MsgStartDevelopment{
		Authority: founder1,
		ProjectId: resp.ProjectId,
	})
	if err == nil {
		t.Fatal("expected error for invalid phase transition from seed")
	}
}

func TestPauseAndResumeProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Pausable",
		Domain:  "math",
		Budget:  "1000000",
	})
	project, _ := k.GetProject(ctx, resp.ProjectId)
	project.Phase = string(types.PhaseGrowing)
	k.SetProject(ctx, project)

	_, err := msgServer.PauseProject(ctx, &types.MsgPauseProject{
		Authority: founder1,
		ProjectId: resp.ProjectId,
		Reason:    "maintenance",
	})
	if err != nil {
		t.Fatalf("pause: %v", err)
	}

	project, _ = k.GetProject(ctx, resp.ProjectId)
	if project.Phase != string(types.PhaseDormant) {
		t.Errorf("expected phase 'dormant' after pause, got %s", project.Phase)
	}

	_, err = msgServer.ResumeProject(ctx, &types.MsgResumeProject{
		Authority: founder1,
		ProjectId: resp.ProjectId,
	})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}

	project, _ = k.GetProject(ctx, resp.ProjectId)
	if project.PreviousPhase == "" && project.Phase == string(types.PhaseDormant) {
		t.Error("project should have been resumed")
	}
}

func TestAbandonProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Abandonable",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.AbandonProject(ctx, &types.MsgAbandonProject{
		Authority: founder1,
		ProjectId: resp.ProjectId,
	})
	if err != nil {
		t.Fatalf("abandon: %v", err)
	}

	// Should be pending (timelock)
	pa, found := k.GetPendingAbandon(ctx, resp.ProjectId)
	if !found {
		t.Fatal("expected pending abandon record")
	}
	if pa.ProposedBy != founder1 {
		t.Errorf("expected proposer %s, got %s", founder1, pa.ProposedBy)
	}
}

// ---------- Task Workflow Tests ----------

func TestAddTask(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	bk.balances[founder1] = map[string]int64{"uzrn": 10_000_000}

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Task Project",
		Domain:  "math",
		Budget:  "5000000",
	})

	taskResp, err := msgServer.AddTask(ctx, &types.MsgAddTask{
		Creator:     founder1,
		ProjectId:   resp.ProjectId,
		Title:       "Implement feature",
		Description: "Build the thing",
		Bounty:      "500000",
	})
	if err != nil {
		t.Fatalf("add task: %v", err)
	}
	if taskResp.TaskId == "" {
		t.Fatal("expected non-empty task_id")
	}

	task, found := k.GetTask(ctx, taskResp.TaskId)
	if !found {
		t.Fatal("task not found")
	}
	if task.Title != "Implement feature" {
		t.Errorf("expected title 'Implement feature', got %q", task.Title)
	}
	if task.Status != string(types.TaskOpen) {
		t.Errorf("expected status 'open', got %s", task.Status)
	}
}

func TestAddTask_NotContributor(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Restricted",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.AddTask(ctx, &types.MsgAddTask{
		Creator:   agent1, // not a contributor
		ProjectId: resp.ProjectId,
		Title:     "Unauthorized task",
		Bounty:    "0",
	})
	if err == nil {
		t.Fatal("expected error for non-contributor")
	}
}

func TestTaskWorkflow_AssignStartSubmitApprove(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	bk.balances[founder1] = map[string]int64{"uzrn": 10_000_000}
	bk.moduleBalances[types.ModuleName] = map[string]int64{"uzrn": 10_000_000}

	projResp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Workflow",
		Domain:  "math",
		Budget:  "5000000",
	})

	taskResp, _ := msgServer.AddTask(ctx, &types.MsgAddTask{
		Creator:   founder1,
		ProjectId: projResp.ProjectId,
		Title:     "Task 1",
		Bounty:    "500000",
	})

	// Assign
	_, err := msgServer.AssignTask(ctx, &types.MsgAssignTask{
		Assigner: founder1,
		TaskId:   taskResp.TaskId,
		Assignee: agent1,
	})
	if err != nil {
		t.Fatalf("assign: %v", err)
	}

	// Start work
	_, err = msgServer.StartWork(ctx, &types.MsgStartWork{
		Worker: agent1,
		TaskId: taskResp.TaskId,
	})
	if err != nil {
		t.Fatalf("start work: %v", err)
	}

	// Submit deliverable
	_, err = msgServer.SubmitDeliverable(ctx, &types.MsgSubmitDeliverable{
		Worker:          agent1,
		TaskId:          taskResp.TaskId,
		DeliverableHash: "abc123hash",
	})
	if err != nil {
		t.Fatalf("submit deliverable: %v", err)
	}

	task, _ := k.GetTask(ctx, taskResp.TaskId)
	if task.Status != string(types.TaskReview) {
		t.Errorf("expected status 'review', got %s", task.Status)
	}

	// Approve
	_, err = msgServer.ApproveDeliverable(ctx, &types.MsgApproveDeliverable{
		Approver: founder1,
		TaskId:   taskResp.TaskId,
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}

	task, _ = k.GetTask(ctx, taskResp.TaskId)
	if task.Status != string(types.TaskCompleted) {
		t.Errorf("expected status 'completed', got %s", task.Status)
	}
}

func TestStartWork_NotAssignee(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	bk.balances[founder1] = map[string]int64{"uzrn": 10_000_000}

	projResp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Work Project",
		Domain:  "math",
		Budget:  "1000000",
	})

	taskResp, _ := msgServer.AddTask(ctx, &types.MsgAddTask{
		Creator:   founder1,
		ProjectId: projResp.ProjectId,
		Title:     "Assigned Task",
		Bounty:    "100000",
	})

	_, _ = msgServer.AssignTask(ctx, &types.MsgAssignTask{
		Assigner: founder1,
		TaskId:   taskResp.TaskId,
		Assignee: agent1,
	})

	_, err := msgServer.StartWork(ctx, &types.MsgStartWork{
		Worker: agent2, // not the assignee
		TaskId: taskResp.TaskId,
	})
	if err == nil {
		t.Fatal("expected error for non-assignee")
	}
}

// ---------- Application Tests ----------

func TestApplyAndReviewApplication(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	projResp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Open Project",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.ApplyToProject(ctx, &types.MsgApplyToProject{
		Applicant:    agent1,
		ProjectId:    projResp.ProjectId,
		Role:         string(types.RoleDeveloper),
		Pitch:        "I want to help",
		Capabilities: []string{"go", "rust"},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Review application — ApplicationId matches applicant DID
	project, _ := k.GetProject(ctx, projResp.ProjectId)
	if len(project.Applications) != 1 {
		t.Fatalf("expected 1 application, got %d", len(project.Applications))
	}

	_, err = msgServer.ReviewApplication(ctx, &types.MsgReviewApplication{
		Reviewer:      founder1,
		ApplicationId: agent1, // DID of applicant is used as ApplicationId
		Accepted:      true,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}

	// Verify contributor was added
	project, _ = k.GetProject(ctx, projResp.ProjectId)
	foundAgent := false
	for _, c := range project.Contributors {
		if c.Did == agent1 {
			foundAgent = true
			break
		}
	}
	if !foundAgent {
		t.Error("expected agent1 to be added as contributor after acceptance")
	}
}

// ---------- Service Tests ----------

func TestDeployService(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, err := msgServer.DeployService(ctx, &types.MsgDeployService{
		Deployer:     founder1,
		Name:         "API Service",
		Description:  "A public API",
		Endpoint:     "https://api.example.com",
		PricePerCall: "10000",
	})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if resp.ServiceId == "" {
		t.Fatal("expected non-empty service_id")
	}

	svc, found := k.GetService(ctx, resp.ServiceId)
	if !found {
		t.Fatal("service not found")
	}
	if svc.Name != "API Service" {
		t.Errorf("expected name 'API Service', got %q", svc.Name)
	}
	if svc.Status != string(types.ServiceDeploying) {
		t.Errorf("expected status 'deploying', got %s", svc.Status)
	}
}

func TestPauseResumeRetireService(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.DeployService(ctx, &types.MsgDeployService{
		Deployer:     founder1,
		Name:         "Lifecycle Service",
		Endpoint:     "https://svc.example.com",
		PricePerCall: "5000",
	})

	// Set to active first
	svc, _ := k.GetService(ctx, resp.ServiceId)
	svc.Status = string(types.ServiceActive)
	k.SetService(ctx, svc)

	// Pause
	_, err := msgServer.PauseService(ctx, &types.MsgPauseService{
		Owner:     founder1,
		ServiceId: resp.ServiceId,
	})
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	svc, _ = k.GetService(ctx, resp.ServiceId)
	if svc.Status != string(types.ServicePaused) {
		t.Errorf("expected 'paused', got %s", svc.Status)
	}

	// Resume
	_, err = msgServer.ResumeService(ctx, &types.MsgResumeService{
		Owner:     founder1,
		ServiceId: resp.ServiceId,
	})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	svc, _ = k.GetService(ctx, resp.ServiceId)
	if svc.Status != string(types.ServiceActive) {
		t.Errorf("expected 'active', got %s", svc.Status)
	}

	// Retire
	_, err = msgServer.RetireService(ctx, &types.MsgRetireService{
		Owner:     founder1,
		ServiceId: resp.ServiceId,
	})
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	svc, _ = k.GetService(ctx, resp.ServiceId)
	if svc.Status != string(types.ServiceRetired) {
		t.Errorf("expected 'retired', got %s", svc.Status)
	}
}

func TestCallService(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	bk.balances[agent1] = map[string]int64{"uzrn": 10_000_000}

	// Deploy and activate a service
	svcResp, _ := msgServer.DeployService(ctx, &types.MsgDeployService{
		Deployer:     founder1,
		Name:         "Callable",
		Endpoint:     "https://api.test",
		PricePerCall: "100000",
	})
	svc, _ := k.GetService(ctx, svcResp.ServiceId)
	svc.Status = string(types.ServiceActive)
	svc.ProjectId = ""
	k.SetService(ctx, svc)

	_, err := msgServer.CallService(ctx, &types.MsgCallService{
		Caller:    agent1,
		ServiceId: svcResp.ServiceId,
		MaxFee:    "200000",
	})
	if err != nil {
		t.Fatalf("call service: %v", err)
	}

	svc, _ = k.GetService(ctx, svcResp.ServiceId)
	if svc.TotalCalls != "1" {
		t.Errorf("expected TotalCalls=1, got %s", svc.TotalCalls)
	}
}

func TestCallService_NotActive(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	svcResp, _ := msgServer.DeployService(ctx, &types.MsgDeployService{
		Deployer:     founder1,
		Name:         "Inactive",
		Endpoint:     "https://api.test",
		PricePerCall: "100000",
	})

	_, err := msgServer.CallService(ctx, &types.MsgCallService{
		Caller:    agent1,
		ServiceId: svcResp.ServiceId,
		MaxFee:    "200000",
	})
	if err == nil {
		t.Fatal("expected error for inactive service")
	}
}

func TestCallService_FreeService(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	svcResp, _ := msgServer.DeployService(ctx, &types.MsgDeployService{
		Deployer:     founder1,
		Name:         "Free",
		Endpoint:     "https://free.test",
		PricePerCall: "0",
	})
	svc, _ := k.GetService(ctx, svcResp.ServiceId)
	svc.Status = string(types.ServiceActive)
	k.SetService(ctx, svc)

	_, err := msgServer.CallService(ctx, &types.MsgCallService{
		Caller:    agent1,
		ServiceId: svcResp.ServiceId,
		MaxFee:    "0",
	})
	if err != nil {
		t.Fatalf("call free service: %v", err)
	}

	svc, _ = k.GetService(ctx, svcResp.ServiceId)
	if svc.TotalCalls != "1" {
		t.Errorf("expected TotalCalls=1, got %s", svc.TotalCalls)
	}
}

// ---------- Contributor Tests ----------

func TestAddContributor(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Contrib Project",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.AddContributor(ctx, &types.MsgAddContributor{
		Authority:   founder1,
		ProjectId:   resp.ProjectId,
		Contributor: agent1,
		Role:        string(types.RoleDeveloper),
	})
	if err != nil {
		t.Fatalf("add contributor: %v", err)
	}

	project, _ := k.GetProject(ctx, resp.ProjectId)
	if len(project.Contributors) != 2 {
		t.Fatalf("expected 2 contributors, got %d", len(project.Contributors))
	}
}

func TestAddContributor_NotFounder(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Restricted",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.AddContributor(ctx, &types.MsgAddContributor{
		Authority:   agent1, // not the founder
		ProjectId:   resp.ProjectId,
		Contributor: agent2,
		Role:        string(types.RoleDeveloper),
	})
	if err == nil {
		t.Fatal("expected error for non-founder adding contributor")
	}
}

func TestAddContributor_Duplicate(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Dup Test",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, _ = msgServer.AddContributor(ctx, &types.MsgAddContributor{
		Authority:   founder1,
		ProjectId:   resp.ProjectId,
		Contributor: agent1,
		Role:        string(types.RoleDeveloper),
	})

	_, err := msgServer.AddContributor(ctx, &types.MsgAddContributor{
		Authority:   founder1,
		ProjectId:   resp.ProjectId,
		Contributor: agent1,
		Role:        string(types.RoleDeveloper),
	})
	if err == nil {
		t.Fatal("expected error for duplicate contributor")
	}
}

// ---------- Subscription Tests ----------

func TestSubscribeService(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	bk.balances[agent1] = map[string]int64{"uzrn": 10_000_000}

	svcResp, _ := msgServer.DeployService(ctx, &types.MsgDeployService{
		Deployer:          founder1,
		Name:              "SubService",
		Endpoint:          "https://sub.test",
		PricePerCall:      "10000",
		SubscriptionPrice: "1000000",
	})
	svc, _ := k.GetService(ctx, svcResp.ServiceId)
	svc.Status = string(types.ServiceActive)
	svc.SubscriptionPrice = "1000000"
	k.SetService(ctx, svc)

	resp, err := msgServer.SubscribeService(ctx, &types.MsgSubscribeService{
		Subscriber:     agent1,
		ServiceId:      svcResp.ServiceId,
		DurationBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if resp.SubscriptionId == "" {
		t.Fatal("expected non-empty subscription_id")
	}
}

func TestSubscribeService_NotActive(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	svcResp, _ := msgServer.DeployService(ctx, &types.MsgDeployService{
		Deployer:     founder1,
		Name:         "InactiveSub",
		Endpoint:     "https://sub.test",
		PricePerCall: "10000",
	})

	_, err := msgServer.SubscribeService(ctx, &types.MsgSubscribeService{
		Subscriber:     agent1,
		ServiceId:      svcResp.ServiceId,
		DurationBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for subscribing to inactive service")
	}
}

// ---------- Seeding Tests ----------

func TestBeginSeeding(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	projResp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "Seeding Project",
		Domain:  "math",
		Budget:  "1000000",
	})
	// Advance to fruiting phase
	project, _ := k.GetProject(ctx, projResp.ProjectId)
	project.Phase = string(types.PhaseFruiting)
	k.SetProject(ctx, project)

	_, err := msgServer.BeginSeeding(ctx, &types.MsgBeginSeeding{
		Seeder:    founder1,
		ProjectId: projResp.ProjectId,
		Domain:    "physics",
	})
	if err != nil {
		t.Fatalf("begin seeding: %v", err)
	}

	project, _ = k.GetProject(ctx, projResp.ProjectId)
	if project.Phase != string(types.PhaseSeeding) {
		t.Errorf("expected phase 'seeding', got %s", project.Phase)
	}
}

func TestBeginSeeding_InvalidPhase(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	projResp, _ := msgServer.CreateProject(ctx, &types.MsgCreateProject{
		Creator: founder1,
		Title:   "WrongPhase",
		Domain:  "math",
		Budget:  "1000000",
	})

	_, err := msgServer.BeginSeeding(ctx, &types.MsgBeginSeeding{
		Seeder:    founder1,
		ProjectId: projResp.ProjectId,
		Domain:    "physics",
	})
	if err == nil {
		t.Fatal("expected error for seeding from seed phase")
	}
}

func TestDetectOpportunity(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	resp, err := msgServer.DetectOpportunity(ctx, &types.MsgDetectOpportunity{
		Detector:     agent1,
		Domain:       "physics",
		Description:  "Quantum computing breakthrough",
		RelatedFacts: []string{"fact-1", "fact-2"},
	})
	if err != nil {
		t.Fatalf("detect opportunity: %v", err)
	}
	if resp.OpportunityId == "" {
		t.Fatal("expected non-empty opportunity_id")
	}

	seed, found := k.GetSeed(ctx, resp.OpportunityId)
	if !found {
		t.Fatal("seed not found")
	}
	if seed.KnowledgeDomain != "physics" {
		t.Errorf("expected domain 'physics', got %q", seed.KnowledgeDomain)
	}
}

func TestClaimOpportunity(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	bk.balances[agent1] = map[string]int64{"uzrn": 10_000_000}

	oppResp, _ := msgServer.DetectOpportunity(ctx, &types.MsgDetectOpportunity{
		Detector:     agent2,
		Domain:       "math",
		Description:  "New opportunity",
		RelatedFacts: []string{"fact-1"},
	})

	_, err := msgServer.ClaimOpportunity(ctx, &types.MsgClaimOpportunity{
		Claimer:       agent1,
		OpportunityId: oppResp.OpportunityId,
		Stake:         "1000000",
	})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	seed, _ := k.GetSeed(ctx, oppResp.OpportunityId)
	if seed.ClaimedBy != agent1 {
		t.Errorf("expected claimed_by %s, got %s", agent1, seed.ClaimedBy)
	}
}

func TestClaimOpportunity_AlreadyClaimed(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	bk.balances[agent1] = map[string]int64{"uzrn": 10_000_000}
	bk.balances[agent2] = map[string]int64{"uzrn": 10_000_000}

	oppResp, _ := msgServer.DetectOpportunity(ctx, &types.MsgDetectOpportunity{
		Detector:     founder1,
		Domain:       "math",
		Description:  "Contested",
		RelatedFacts: []string{"f1"},
	})

	_, _ = msgServer.ClaimOpportunity(ctx, &types.MsgClaimOpportunity{
		Claimer:       agent1,
		OpportunityId: oppResp.OpportunityId,
		Stake:         "1000000",
	})

	_, err := msgServer.ClaimOpportunity(ctx, &types.MsgClaimOpportunity{
		Claimer:       agent2,
		OpportunityId: oppResp.OpportunityId,
		Stake:         "1000000",
	})
	if err == nil {
		t.Fatal("expected error for already claimed opportunity")
	}
}

// ---------- Query Tests ----------

func TestQueryProject(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	querySrv := keeper.NewQueryServerImpl(k)

	k.SetProject(ctx, &types.ProductProject{
		Id:      "proj-q1",
		Name:    "Query Project",
		Founder: founder1,
	})

	resp, err := querySrv.Project(ctx, &types.QueryProjectRequest{ProjectId: "proj-q1"})
	if err != nil {
		t.Fatalf("query project: %v", err)
	}
	if resp.Project.Name != "Query Project" {
		t.Errorf("expected name 'Query Project', got %q", resp.Project.Name)
	}
}

func TestQueryProjectNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	querySrv := keeper.NewQueryServerImpl(k)

	_, err := querySrv.Project(ctx, &types.QueryProjectRequest{ProjectId: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent project")
	}
}

func TestQueryParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	querySrv := keeper.NewQueryServerImpl(k)

	resp, err := querySrv.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("query params: %v", err)
	}
	if resp.Params == nil {
		t.Fatal("expected non-nil params")
	}
}

// ---------- Genesis Roundtrip Test ----------

func TestGenesisRoundtrip(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetProject(ctx, &types.ProductProject{
		Id:       "proj-gen-1",
		Name:     "Genesis Project",
		Founder:  founder1,
		TaskIds:  []string{},
		Budget:   "1000000",
		Spent:    "0",
		Phase:    string(types.PhaseSeed),
		Contributors: []*types.ContributorRecord{},
	})
	k.SetTask(ctx, &types.ProjectTask{
		Id:        "task-gen-1",
		ProjectId: "proj-gen-1",
		Title:     "Genesis Task",
		Status:    string(types.TaskOpen),
	})
	k.SetService(ctx, &types.ServiceLeaf{
		Id:           "svc-gen-1",
		Name:         "Genesis Service",
		Status:       string(types.ServiceActive),
		TotalCalls:   "0",
		TotalRevenue: "0",
	})
	k.SetSeed(ctx, &types.OpportunitySeed{
		Id:              "seed-gen-1",
		KnowledgeDomain: "math",
		Status:          string(types.SeedDetected),
	})

	exported := k.ExportGenesis(ctx)

	if len(exported.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(exported.Projects))
	}
	if len(exported.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(exported.Tasks))
	}
	if len(exported.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(exported.Services))
	}
	if len(exported.Seeds) != 1 {
		t.Fatalf("expected 1 seed, got %d", len(exported.Seeds))
	}

	// Re-import into fresh keeper
	k2, ctx2, _ := setupKeeper(t)
	k2.InitGenesis(ctx2, exported)

	p, found := k2.GetProject(ctx2, "proj-gen-1")
	if !found {
		t.Fatal("project not found after genesis import")
	}
	if p.Name != "Genesis Project" {
		t.Errorf("expected 'Genesis Project', got %q", p.Name)
	}
}

// ---------- Revenue Tests ----------

func TestCalculateRevenue(t *testing.T) {
	contributors := []*types.ContributorRecord{
		{Did: founder1, TasksCompleted: 3},
		{Did: agent1, TasksCompleted: 7},
	}

	dist := keeper.CalculateRevenue(
		1_000_000,
		550000, // 55% contributor
		220000, // 22% treasury
		33300,  // 3.33% research
		196700, // 19.67% development fund
		contributors,
	)

	if dist.ContributorPool != 550000 {
		t.Errorf("expected contributor pool 550000, got %d", dist.ContributorPool)
	}
	if dist.DevelopmentFund != 196700 {
		t.Errorf("expected development fund 196700, got %d", dist.DevelopmentFund)
	}
	if dist.ResearchFund != 33300 {
		t.Errorf("expected research fund 33300, got %d", dist.ResearchFund)
	}

	// Treasury allocation = 1M - 550K - 33.3K - 196.7K = 220K
	// Verification pool = 220K * 300000 / 1000000 = 66K
	// Protocol treasury = 220K - 66K = 154K
	if dist.ProtocolTreasury != 154000 {
		t.Errorf("expected protocol treasury 154000, got %d", dist.ProtocolTreasury)
	}
	if dist.VerificationPool != 66000 {
		t.Errorf("expected verification pool 66000, got %d", dist.VerificationPool)
	}

	// Check lossless: all allocations sum to total
	total := dist.ContributorPool + dist.ResearchFund + dist.ProtocolTreasury + dist.VerificationPool + dist.DevelopmentFund
	if total != 1_000_000 {
		t.Errorf("expected total 1000000, got %d (lossless violation)", total)
	}

	// Contributor shares: 3/10 and 7/10
	if len(dist.ContributorShares) != 2 {
		t.Fatalf("expected 2 contributor shares, got %d", len(dist.ContributorShares))
	}
}

func TestCalculateRevenue_ZeroAmount(t *testing.T) {
	dist := keeper.CalculateRevenue(0, 550000, 220000, 33300, 196700, nil)
	if dist.ContributorPool != 0 || dist.ProtocolTreasury != 0 {
		t.Error("expected zero distribution for zero total")
	}
}

func TestCalculateRevenue_NoContributors(t *testing.T) {
	dist := keeper.CalculateRevenue(1_000_000, 550000, 220000, 33300, 196700, nil)
	if dist.ContributorPool != 0 {
		t.Errorf("expected contributor pool redirected to treasury, got %d", dist.ContributorPool)
	}
	// Contributor pool should be added to treasury
	if dist.ProtocolTreasury < 154000 {
		t.Errorf("expected treasury to include redirected contributor pool, got %d", dist.ProtocolTreasury)
	}
}

func TestCalculateRevenue_EqualSplitNoTasks(t *testing.T) {
	contributors := []*types.ContributorRecord{
		{Did: founder1, TasksCompleted: 0},
		{Did: agent1, TasksCompleted: 0},
	}

	dist := keeper.CalculateRevenue(1_000_000, 550000, 220000, 33300, 196700, contributors)
	if len(dist.ContributorShares) != 2 {
		t.Fatalf("expected 2 shares, got %d", len(dist.ContributorShares))
	}
	// With zero tasks, should split equally
	for _, share := range dist.ContributorShares {
		if share.Amount < 270000 || share.Amount > 280000 {
			t.Errorf("expected roughly equal share (~275000), got %d for %s", share.Amount, share.Address)
		}
	}
}

// ---------- Toolbox Adapter Tests ----------

func TestServiceExists(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetService(ctx, &types.ServiceLeaf{
		Id:     "svc-exists",
		Name:   "Existing",
		Status: string(types.ServiceActive),
	})

	if !k.ServiceExists(ctx, "svc-exists") {
		t.Error("expected service to exist")
	}
	if k.ServiceExists(ctx, "svc-nope") {
		t.Error("expected service to not exist")
	}
}

func TestCallServiceViaToolbox(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetService(ctx, &types.ServiceLeaf{
		Id:         "svc-tool",
		Name:       "Toolbox Svc",
		Status:     string(types.ServiceActive),
		TotalCalls: "0",
	})

	_, err := k.CallService(ctx, agent1, "svc-tool", nil)
	if err != nil {
		t.Fatalf("toolbox call: %v", err)
	}

	svc, _ := k.GetService(ctx, "svc-tool")
	if svc.TotalCalls != "1" {
		t.Errorf("expected TotalCalls=1 after toolbox call, got %s", svc.TotalCalls)
	}
}

func TestCallService_NotActiveToolbox(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetService(ctx, &types.ServiceLeaf{
		Id:     "svc-inactive",
		Name:   "Inactive",
		Status: string(types.ServicePaused),
	})

	_, err := k.CallService(ctx, agent1, "svc-inactive", nil)
	if err == nil {
		t.Fatal("expected error for inactive service")
	}
}

// ---------- UpdateParams Tests ----------

func TestUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	newParams := types.DefaultParams()
	newParams.MaxTasksPerProject = 500

	_, err := msgServer.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: k.GetAuthority(),
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("update params: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MaxTasksPerProject != 500 {
		t.Errorf("expected MaxTasksPerProject=500, got %d", got.MaxTasksPerProject)
	}
}

func TestUpdateParams_Unauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	_, err := msgServer.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: agent1, // not the authority
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected error for unauthorized params update")
	}
}

// ---------- SetAvailability Tests ----------

func TestSetAvailability(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgServer := keeper.NewMsgServerImpl(k)

	_, err := msgServer.SetAvailability(ctx, &types.MsgSetAvailability{
		Agent:            agent1,
		Available:        true,
		Capabilities:     []string{"go", "python"},
		PreferredDomains: []string{"math", "physics"},
		MinimumBounty:    "100000",
	})
	if err != nil {
		t.Fatalf("set availability: %v", err)
	}

	avail, found := k.GetAgentAvailability(ctx, agent1)
	if !found {
		t.Fatal("availability not found")
	}
	if !avail.Available {
		t.Error("expected agent to be available")
	}
	if len(avail.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(avail.Capabilities))
	}
}

// ---------- Determinism Tests ----------

// setupDeterministicKeeper creates an independent keeper+store pair for
// determinism comparison testing. Each call returns a fresh, isolated
// instance backed by its own in-memory DB.
func setupDeterministicKeeper(t *testing.T) (keeper.Keeper, sdk.Context, storetypes.CommitMultiStore) {
	t.Helper()
	initAddresses()

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
	noop := &noOpResearchFundDepositor{bk: mockBK}
	k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), mockBK, "zrn1authority", noop)
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	return k, ctx, stateStore
}

// TestDeterminism_ProjectCRUD verifies that the same project operations on
// two independent stores produce identical IAVL commit hashes.
func TestDeterminism_ProjectCRUD(t *testing.T) {
	// Run identical operations on two independent stores
	hashes := make([][]byte, 2)
	for run := 0; run < 2; run++ {
		k, ctx, cms := setupDeterministicKeeper(t)

		// Create 5 projects
		for i := 1; i <= 5; i++ {
			p := &types.ProductProject{
				Id:              fmt.Sprintf("proj-%d", i),
				Name:            fmt.Sprintf("Project %d", i),
				Description:     fmt.Sprintf("Description for project %d", i),
				Phase:           string(types.PhaseSeed),
				CreatedAtBlock:  100,
				Founder:         founder1,
				KnowledgeDomain: "general",
				Budget:          "1000000",
				Spent:           "0",
				TaskIds:         []string{},
				ServiceIds:      []string{},
				Contributors: []*types.ContributorRecord{
					{Did: founder1, Role: string(types.RoleFounder), JoinedAtBlock: 100},
				},
			}
			k.SetProject(ctx, p)
		}

		// Update one
		p, _ := k.GetProject(ctx, "proj-3")
		p.Phase = string(types.PhaseGrowing)
		k.SetProject(ctx, p)

		// Delete one
		p2, _ := k.GetProject(ctx, "proj-5")
		k.DeleteProject(ctx, p2)

		commitID := cms.Commit()
		hashes[run] = commitID.Hash
	}

	if !bytes.Equal(hashes[0], hashes[1]) {
		t.Fatalf("non-deterministic state: hash1=%x hash2=%x", hashes[0], hashes[1])
	}
}

// TestDeterminism_SeedWithMapEvidence verifies that seeds containing
// DemandSignal.Evidence (a proto map<string,string>) produce identical
// bytes across independent serialization runs. This is the critical
// test: proto map serialization must be deterministic for consensus.
func TestDeterminism_SeedWithMapEvidence(t *testing.T) {
	hashes := make([][]byte, 2)
	for run := 0; run < 2; run++ {
		k, ctx, cms := setupDeterministicKeeper(t)

		// Create seeds with map evidence in varying key insertion order.
		// If marshaling is non-deterministic, different runs may produce
		// different bytes for the same logical map content.
		for i := 1; i <= 3; i++ {
			evidence := make(map[string]string)
			// Insert keys in different "logical" groups to stress map ordering.
			evidence["description"] = fmt.Sprintf("Opportunity %d detected", i)
			evidence["related_fact_0"] = fmt.Sprintf("fact-alpha-%d", i)
			evidence["related_fact_1"] = fmt.Sprintf("fact-beta-%d", i)
			evidence["related_fact_2"] = fmt.Sprintf("fact-gamma-%d", i)
			evidence["source"] = "agent_detection"
			evidence["confidence"] = "0.85"

			seed := &types.OpportunitySeed{
				Id:              fmt.Sprintf("seed-%d", i),
				DetectedAtBlock: 100,
				KnowledgeDomain: "general",
				Confidence:      "0.5",
				Status:          string(types.SeedDetected),
				ExpiresAtBlock:  200,
				Signal: &types.DemandSignal{
					Type:     "agent_detection",
					Evidence: evidence,
					Strength: "0.5",
				},
			}
			k.SetSeed(ctx, seed)
		}

		commitID := cms.Commit()
		hashes[run] = commitID.Hash
	}

	if !bytes.Equal(hashes[0], hashes[1]) {
		t.Fatalf("non-deterministic state from map evidence: hash1=%x hash2=%x",
			hashes[0], hashes[1])
	}
}

// TestDeterminism_FullWorkflow runs a comprehensive workflow (projects, tasks,
// services, seeds, params) on two independent stores and verifies the commit
// hashes match. This catches any non-determinism in the complete state machine.
func TestDeterminism_FullWorkflow(t *testing.T) {
	hashes := make([][]byte, 2)
	for run := 0; run < 2; run++ {
		k, ctx, cms := setupDeterministicKeeper(t)

		// Set params
		k.SetParams(ctx, &types.Params{
			MinBudget:              "100000",
			MaxTasksPerProject:     100,
			MaxContributors:        20,
			MaxApplications:        50,
			TaskDeadlineMinBlocks:  100,
			TaskDeadlineMaxBlocks:  100000,
			MaxRejections:          3,
			SeedExpiryBlocks:       50000,
			MinContributorsToStart: 2,
		})

		// Create project
		k.SetProject(ctx, &types.ProductProject{
			Id: "proj-100-1", Name: "Workflow Test", Phase: string(types.PhaseSeed),
			CreatedAtBlock: 100, Founder: founder1, KnowledgeDomain: "general",
			Budget: "5000000", Spent: "0", TaskIds: []string{"task-100-1"},
			ServiceIds: []string{}, Contributors: []*types.ContributorRecord{
				{Did: founder1, Role: string(types.RoleFounder), JoinedAtBlock: 100},
				{Did: agent1, Role: "developer", JoinedAtBlock: 101},
			},
		})

		// Create task
		k.SetTask(ctx, &types.ProjectTask{
			Id: "task-100-1", ProjectId: "proj-100-1", Title: "Build API",
			Status: string(types.TaskAssigned), CreatedAtBlock: 100,
			Assignee: agent1, BountyAmount: "1000000",
		})

		// Create service
		k.SetService(ctx, &types.ServiceLeaf{
			Id: "svc-100-1", Name: "API Service", Description: "REST API",
			ContractAddress: "https://api.example.com", Status: string(types.ServiceActive),
			DeployedAtBlock: 100, PricePerCall: "1000",
			TotalCalls: "0", TotalRevenue: "0", UptimeBlocks: "0",
		})

		// Create seed with evidence map
		k.SetSeed(ctx, &types.OpportunitySeed{
			Id: "seed-100-1", DetectedAtBlock: 100, KnowledgeDomain: "general",
			Status: string(types.SeedDetected), ExpiresAtBlock: 200,
			Signal: &types.DemandSignal{
				Type:     "agent_detection",
				Evidence: map[string]string{"desc": "opportunity", "fact_0": "evidence"},
				Strength: "0.7",
			},
		})

		commitID := cms.Commit()
		hashes[run] = commitID.Hash
	}

	if !bytes.Equal(hashes[0], hashes[1]) {
		t.Fatalf("non-deterministic state in full workflow: hash1=%x hash2=%x",
			hashes[0], hashes[1])
	}
}
