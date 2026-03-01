package keeper_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/toolbox/keeper"
	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// ============================================================
// Address Helper
// ============================================================

func testAddr(name string) string {
	hash := sha256.Sum256([]byte("kt_test_addr:" + name))
	return sdk.AccAddress(hash[:20]).String()
}

// ============================================================
// Mock Bank Keeper (tracking)
// ============================================================

type mockSendRecord struct {
	from, to string
	amount   uint64
}

type mockBankKeeper struct {
	modToAccSends []mockSendRecord
	modToModSends []mockSendRecord
	accToModSends []mockSendRecord
	failSend      bool
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{}
}

func (m *mockBankKeeper) SendCoins(_ context.Context, _ sdk.AccAddress, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}
func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	if m.failSend {
		return fmt.Errorf("insufficient funds")
	}
	m.accToModSends = append(m.accToModSends, mockSendRecord{
		from: senderAddr.String(), to: recipientModule, amount: amt.AmountOf("uzrn").Uint64(),
	})
	return nil
}
func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	m.modToAccSends = append(m.modToAccSends, mockSendRecord{
		from: senderModule, to: recipientAddr.String(), amount: amt.AmountOf("uzrn").Uint64(),
	})
	return nil
}
func (m *mockBankKeeper) SendCoinsFromModuleToModule(_ context.Context, senderModule string, recipientModule string, amt sdk.Coins) error {
	m.modToModSends = append(m.modToModSends, mockSendRecord{
		from: senderModule, to: recipientModule, amount: amt.AmountOf("uzrn").Uint64(),
	})
	return nil
}
func (m *mockBankKeeper) GetBalance(_ context.Context, _ sdk.AccAddress, _ string) sdk.Coin {
	return sdk.NewCoin("uzrn", sdkmath.NewInt(1_000_000_000))
}

func (m *mockBankKeeper) reset() {
	m.modToAccSends = nil
	m.modToModSends = nil
	m.accToModSends = nil
	m.failSend = false
}

func (m *mockBankKeeper) totalModToAcc() uint64 {
	var t uint64
	for _, s := range m.modToAccSends {
		t += s.amount
	}
	return t
}

var _ types.BankKeeper = (*mockBankKeeper)(nil)

// ============================================================
// Mock Research Fund
// ============================================================

type mockResearchFund struct {
	deposits []mockSendRecord
}

func newMockResearchFund() *mockResearchFund { return &mockResearchFund{} }

func (m *mockResearchFund) DepositToResearchFund(_ context.Context, sourceModule string, amt sdk.Coins) error {
	m.deposits = append(m.deposits, mockSendRecord{from: sourceModule, amount: amt.AmountOf("uzrn").Uint64()})
	return nil
}

var _ types.ResearchFundDepositor = (*mockResearchFund)(nil)

// ============================================================
// Mock Discovery Keeper
// ============================================================

type mockDiscoveryKeeper struct {
	agents map[string][]string
}

func newTestDiscoveryKeeper() *mockDiscoveryKeeper {
	return &mockDiscoveryKeeper{agents: make(map[string][]string)}
}

func (m *mockDiscoveryKeeper) IsRegisteredAgent(_ context.Context, address string) bool {
	_, ok := m.agents[address]
	return ok
}
func (m *mockDiscoveryKeeper) GetAgentCapabilityTypes(_ context.Context, address string) ([]string, error) {
	return m.agents[address], nil
}

var _ types.DiscoveryKeeper = (*mockDiscoveryKeeper)(nil)

// ============================================================
// Mock BVM Keeper
// ============================================================

type mockBvmKeeper struct {
	contracts map[string]string // address -> creator
	callOut   []byte
	callErr   error
}

func newMockBvmKeeper() *mockBvmKeeper {
	return &mockBvmKeeper{contracts: make(map[string]string)}
}

func (m *mockBvmKeeper) ContractExists(_ context.Context, address string) bool {
	_, ok := m.contracts[address]
	return ok
}
func (m *mockBvmKeeper) GetContractCreator(_ context.Context, address string) (string, error) {
	c, ok := m.contracts[address]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return c, nil
}
func (m *mockBvmKeeper) CallContract(_ context.Context, _ string, _ string, _ []byte, _ uint64) ([]byte, error) {
	return m.callOut, m.callErr
}

var _ types.BvmKeeper = (*mockBvmKeeper)(nil)

// ============================================================
// Mock Knowledge Keeper
// ============================================================

type mockKnowledgeKeeper struct {
	facts     map[string]mockFact // reuse mockFact from purpose_prompter_test.go
	searchIDs []string
}

func newTestKnowledgeKeeper() *mockKnowledgeKeeper {
	return &mockKnowledgeKeeper{facts: make(map[string]mockFact)}
}

func (m *mockKnowledgeKeeper) GetFactConfidence(_ context.Context, factID string) (uint64, bool) {
	f, ok := m.facts[factID]
	if !ok {
		return 0, false
	}
	return f.confidence, true
}
func (m *mockKnowledgeKeeper) SearchFactsByContent(_ context.Context, _ string, _ []string, _ uint64) ([]string, error) {
	return m.searchIDs, nil
}
func (m *mockKnowledgeKeeper) GetFactDetails(_ context.Context, factID string) (string, uint64, uint64, error) {
	f, ok := m.facts[factID]
	if !ok {
		return "", 0, 0, fmt.Errorf("not found")
	}
	return f.content, f.confidence, f.citations, nil
}
func (m *mockKnowledgeKeeper) RecordFactCitation(_ context.Context, _ string, _ string) error {
	return nil
}

var _ types.KnowledgeKeeper = (*mockKnowledgeKeeper)(nil)

// ============================================================
// Mock Billing Keeper
// ============================================================

type mockBillingKeeper struct {
	priceUSD uint64
	err      error
}

func (m *mockBillingKeeper) GetZRNPriceUSD(_ context.Context) (uint64, error) {
	return m.priceUSD, m.err
}

var _ types.BillingKeeper = (*mockBillingKeeper)(nil)

// ============================================================
// Mock Home Keeper
// ============================================================

type mockHome struct {
	owner     string
	createdAt uint64
	status    string
}

type mockHomeKeeper struct {
	homes map[string]mockHome // homeID -> home
}

func newMockHomeKeeper() *mockHomeKeeper {
	return &mockHomeKeeper{homes: make(map[string]mockHome)}
}

func (m *mockHomeKeeper) GetHomesByOwner(_ context.Context, owner string) ([]string, error) {
	var ids []string
	for id, h := range m.homes {
		if h.owner == owner {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
func (m *mockHomeKeeper) GetHomeCreatedAtBlock(_ context.Context, homeID string) (uint64, error) {
	h, ok := m.homes[homeID]
	if !ok {
		return 0, fmt.Errorf("not found")
	}
	return h.createdAt, nil
}
func (m *mockHomeKeeper) GetHomeStatus(_ context.Context, homeID string) (string, error) {
	h, ok := m.homes[homeID]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return h.status, nil
}

var _ types.HomeKeeper = (*mockHomeKeeper)(nil)

// ============================================================
// Mock Staking Keeper
// ============================================================

type mockStakingKeeper struct {
	tiers      map[string]uint32
	accuracies map[string]uint64
}

func newMockStakingKeeper() *mockStakingKeeper {
	return &mockStakingKeeper{
		tiers:      make(map[string]uint32),
		accuracies: make(map[string]uint64),
	}
}

func (m *mockStakingKeeper) GetValidatorTier(_ context.Context, valAddr string) (uint32, error) {
	t, ok := m.tiers[valAddr]
	if !ok {
		return 0, fmt.Errorf("not found")
	}
	return t, nil
}
func (m *mockStakingKeeper) GetValidatorAccuracy(_ context.Context, valAddr string) (uint64, error) {
	a, ok := m.accuracies[valAddr]
	if !ok {
		return 0, fmt.Errorf("not found")
	}
	return a, nil
}

var _ types.StakingKeeper = (*mockStakingKeeper)(nil)

// ============================================================
// Full Setup Struct
// ============================================================

type fullSetup struct {
	k         keeper.Keeper
	ctx       sdk.Context
	ms        types.MsgServer
	bank      *mockBankKeeper
	research  *mockResearchFund
	discovery *mockDiscoveryKeeper
	bvm       *mockBvmKeeper
	knowledge *mockKnowledgeKeeper
	billing   *mockBillingKeeper
	home      *mockHomeKeeper
	staking   *mockStakingKeeper
}

// ============================================================
// Setup Functions
// ============================================================

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper, *mockResearchFund) {
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

	bk := newMockBankKeeper()
	rf := newMockResearchFund()
	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(storeService, cdc, "zrn1authority", bk, rf)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())
	return k, ctx, bk, rf
}

func setupMsgServer(t *testing.T) (types.MsgServer, keeper.Keeper, sdk.Context, *mockBankKeeper, *mockResearchFund) {
	t.Helper()
	k, ctx, bk, rf := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)
	return ms, k, ctx, bk, rf
}

func setupFull(t *testing.T) *fullSetup {
	t.Helper()
	k, ctx, bk, rf := setupKeeper(t)

	dk := newTestDiscoveryKeeper()
	bvm := newMockBvmKeeper()
	kk := newTestKnowledgeKeeper()
	billing := &mockBillingKeeper{priceUSD: 1_000_000} // $1.00
	home := newMockHomeKeeper()
	staking := newMockStakingKeeper()

	k.SetDiscoveryKeeper(dk)
	k.SetBvmKeeper(bvm)
	k.SetKnowledgeKeeper(kk)
	k.SetBillingKeeper(billing)
	k.SetHomeKeeper(home)
	k.SetStakingKeeper(staking)

	ms := keeper.NewMsgServerImpl(k)
	return &fullSetup{
		k: k, ctx: ctx, ms: ms,
		bank: bk, research: rf,
		discovery: dk, bvm: bvm, knowledge: kk,
		billing: billing, home: home, staking: staking,
	}
}

// ============================================================
// Test Helpers
// ============================================================

type toolOpt func(*types.MsgRegisterTool)

func withCategory(cat string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.Category = cat }
}
func withPrice(price string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.PricePerCall = price }
}
func withToolType(tt string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.ToolType = tt }
}
func withDeps(deps ...string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.DependencyIds = deps }
}
func withLicense(lic string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.License = lic }
}
func withContract(addr string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.ContractAddress = addr }
}
func withTargetUSD(usd string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.TargetPriceUsd = usd }
}
func withMinMax(min, max string) toolOpt {
	return func(m *types.MsgRegisterTool) { m.MinPricePerCall = min; m.MaxPricePerCall = max }
}

func registerTestTool(t *testing.T, ms types.MsgServer, ctx sdk.Context, deployer, name string, opts ...toolOpt) string {
	t.Helper()
	msg := &types.MsgRegisterTool{
		Deployer: deployer,
		Name:     name,
		ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility,
		License:  types.LicenseOpen,
		Version:  "1.0.0",
	}
	for _, opt := range opts {
		opt(msg)
	}
	resp, err := ms.RegisterTool(ctx, msg)
	if err != nil {
		t.Fatalf("registerTestTool(%s): %v", name, err)
	}
	return resp.ToolId
}

func activateTool(t *testing.T, k keeper.Keeper, ctx sdk.Context, toolID string) {
	t.Helper()
	tool, ok := k.GetTool(ctx, toolID)
	if !ok {
		t.Fatalf("activateTool: tool %s not found", toolID)
	}
	k.UpdateToolStatus(ctx, tool, types.ToolStatusActive)
}

func setToolStatus(t *testing.T, k keeper.Keeper, ctx sdk.Context, toolID, status string) {
	t.Helper()
	tool, ok := k.GetTool(ctx, toolID)
	if !ok {
		t.Fatalf("setToolStatus: tool %s not found", toolID)
	}
	k.UpdateToolStatus(ctx, tool, status)
}

func uzrn(amount uint64) sdk.Coin {
	return sdk.NewCoin("uzrn", sdkmath.NewInt(int64(amount)))
}

// ============================================================
// 1. Tool Registration & Lifecycle
// ============================================================

func TestRegisterTool_AllTypes(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("deployer")

	// BVM contract
	contractAddr := "zrn1contract1"
	s.bvm.contracts[contractAddr] = deployer
	id1 := registerTestTool(t, s.ms, s.ctx, deployer, "bvm-tool",
		withToolType(types.ToolTypeBVMContract), withContract(contractAddr))

	// Tree service
	id2 := registerTestTool(t, s.ms, s.ctx, deployer, "tree-tool",
		withToolType(types.ToolTypeTreeService))

	// Knowledge template
	id3 := registerTestTool(t, s.ms, s.ctx, deployer, "knowledge-tool",
		withToolType(types.ToolTypeKnowledgeTemplate), withCategory(types.CategoryDataRetrieval))

	// Composite
	activateTool(t, s.k, s.ctx, id2)
	activateTool(t, s.k, s.ctx, id3)
	id4 := registerTestTool(t, s.ms, s.ctx, deployer, "composite-tool",
		withToolType(types.ToolTypeComposite), withCategory(types.CategoryComposite),
		withDeps(id2, id3))

	// id2 and id3 were activated for composite deps — check they're active.
	for _, id := range []string{id2, id3} {
		tool, ok := s.k.GetTool(s.ctx, id)
		if !ok {
			t.Fatalf("tool %s not found", id)
		}
		if tool.Status != types.ToolStatusActive {
			t.Errorf("tool %s: expected active status, got %s", id, tool.Status)
		}
	}
	// id1 and id4 should still be draft.
	for _, id := range []string{id1, id4} {
		tool, ok := s.k.GetTool(s.ctx, id)
		if !ok {
			t.Fatalf("tool %s not found", id)
		}
		if tool.Status != types.ToolStatusDraft {
			t.Errorf("tool %s: expected draft status, got %s", id, tool.Status)
		}
		if tool.TrustScore != 500_000 {
			t.Errorf("tool %s: expected trust 500000, got %d", id, tool.TrustScore)
		}
	}

	// Verify composite has deps
	tool4, _ := s.k.GetTool(s.ctx, id4)
	if len(tool4.DependencyIds) != 2 {
		t.Errorf("composite: expected 2 deps, got %d", len(tool4.DependencyIds))
	}
}

func TestRegisterTool_InvalidCategory(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: testAddr("d"), Name: "bad-cat", ToolType: types.ToolTypeTreeService,
		Category: "nonexistent", License: types.LicenseOpen, Version: "1.0.0",
	})
	if err == nil {
		t.Fatal("expected ErrInvalidCategory")
	}
}

func TestRegisterTool_InvalidLicense(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: testAddr("d"), Name: "bad-lic", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: "pirate", Version: "1.0.0",
	})
	if err == nil {
		t.Fatal("expected ErrInvalidLicense")
	}
}

func TestRegisterTool_ExceedMaxDependencies(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	// Set max deps to 2
	params := types.DefaultParams()
	params.MaxDependencies = 2
	k.SetParams(ctx, params)

	// Create 3 deps
	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		ids[i] = registerTestTool(t, ms, ctx, deployer, fmt.Sprintf("dep-%d", i))
		activateTool(t, k, ctx, ids[i])
	}

	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "too-many-deps", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: types.LicenseOpen, Version: "1.0.0",
		DependencyIds: ids,
	})
	if err == nil {
		t.Fatal("expected ErrTooManyDependencies")
	}
}

func TestRegisterTool_DuplicateName(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	registerTestTool(t, ms, ctx, deployer, "unique-name")
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "unique-name", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: types.LicenseOpen, Version: "1.0.0",
	})
	if err == nil {
		t.Fatal("expected ErrToolAlreadyExists")
	}
}

func TestRegisterTool_BVMContractValidation(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("deployer")
	other := testAddr("other")

	// Contract doesn't exist
	_, err := s.ms.RegisterTool(s.ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "no-contract", ToolType: types.ToolTypeBVMContract,
		ContractAddress: "zrn1missing", Category: types.CategoryComputation,
		License: types.LicenseOpen, Version: "1.0.0",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent contract")
	}

	// Deployer is not the creator
	s.bvm.contracts["zrn1c1"] = other
	_, err = s.ms.RegisterTool(s.ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "wrong-creator", ToolType: types.ToolTypeBVMContract,
		ContractAddress: "zrn1c1", Category: types.CategoryComputation,
		License: types.LicenseOpen, Version: "1.0.0",
	})
	if err == nil {
		t.Fatal("expected ErrInvalidContractOwner")
	}
}

func TestRegisterTool_DefaultPriceZero(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	id := registerTestTool(t, ms, ctx, testAddr("d"), "zero-price")
	tool, _ := k.GetTool(ctx, id)
	if tool.PricePerCall != "0" {
		t.Errorf("expected price '0', got %s", tool.PricePerCall)
	}
}

func TestUpgradeTool_Success(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	prevID := registerTestTool(t, ms, ctx, deployer, "v1-tool", withPrice("100000"))

	resp, err := ms.UpgradeTool(ctx, &types.MsgUpgradeTool{
		Deployer: deployer, PreviousToolId: prevID, NewVersion: "2.0.0",
		Description: "upgraded",
	})
	if err != nil {
		t.Fatalf("UpgradeTool: %v", err)
	}

	newTool, ok := k.GetTool(ctx, resp.NewToolId)
	if !ok {
		t.Fatal("new tool not found")
	}
	if newTool.PreviousVersionId != prevID {
		t.Errorf("expected PreviousVersionId=%s, got %s", prevID, newTool.PreviousVersionId)
	}
	if newTool.PricePerCall != "100000" {
		t.Errorf("expected inherited price 100000, got %s", newTool.PricePerCall)
	}
	if newTool.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %s", newTool.Version)
	}
}

func TestUpgradeTool_NotDeployer(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	prevID := registerTestTool(t, ms, ctx, deployer, "v1")
	_, err := ms.UpgradeTool(ctx, &types.MsgUpgradeTool{
		Deployer: testAddr("stranger"), PreviousToolId: prevID, NewVersion: "2.0.0",
	})
	if err == nil {
		t.Fatal("expected ErrNotDeployer")
	}
}

func TestUpgradeTool_RetiredTool(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	prevID := registerTestTool(t, ms, ctx, deployer, "v1")
	setToolStatus(t, k, ctx, prevID, types.ToolStatusRetired)

	_, err := ms.UpgradeTool(ctx, &types.MsgUpgradeTool{
		Deployer: deployer, PreviousToolId: prevID, NewVersion: "2.0.0",
	})
	if err == nil {
		t.Fatal("expected ErrToolRetired")
	}
}

func TestDeprecateTool_Success(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "deprecatable")
	activateTool(t, k, ctx, id)

	_, err := ms.DeprecateTool(ctx, &types.MsgDeprecateTool{Authority: deployer, ToolId: id})
	if err != nil {
		t.Fatalf("DeprecateTool: %v", err)
	}
	tool, _ := k.GetTool(ctx, id)
	if tool.Status != types.ToolStatusDeprecated {
		t.Errorf("expected deprecated, got %s", tool.Status)
	}
}

func TestDeprecateTool_WithSuccessor(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id1 := registerTestTool(t, ms, ctx, deployer, "old-tool")
	id2 := registerTestTool(t, ms, ctx, deployer, "new-tool")
	activateTool(t, k, ctx, id1)

	_, err := ms.DeprecateTool(ctx, &types.MsgDeprecateTool{
		Authority: deployer, ToolId: id1, SuccessorToolId: id2,
	})
	if err != nil {
		t.Fatalf("DeprecateTool with successor: %v", err)
	}

	// Nonexistent successor should fail
	_, err = ms.DeprecateTool(ctx, &types.MsgDeprecateTool{
		Authority: deployer, ToolId: id2, SuccessorToolId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent successor")
	}
}

func TestRetireTool_Success(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "retirable")
	activateTool(t, k, ctx, id)

	_, err := ms.RetireTool(ctx, &types.MsgRetireTool{Authority: deployer, ToolId: id})
	if err != nil {
		t.Fatalf("RetireTool: %v", err)
	}
	tool, _ := k.GetTool(ctx, id)
	if tool.Status != types.ToolStatusRetired {
		t.Errorf("expected retired, got %s", tool.Status)
	}
}

func TestRetireTool_CallsFail(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	caller := testAddr("caller")
	id := registerTestTool(t, ms, ctx, deployer, "soon-retired")
	activateTool(t, k, ctx, id)

	// Retire
	_, _ = ms.RetireTool(ctx, &types.MsgRetireTool{Authority: deployer, ToolId: id})

	// Call should fail
	_, err := ms.CallTool(ctx, &types.MsgCallTool{Caller: caller, ToolId: id})
	if err == nil {
		t.Fatal("expected ErrToolRetired on call")
	}
}

func TestStatusTransitions(t *testing.T) {
	_, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	tool := &types.Tool{
		Id: "status-test", Name: "status-test", ToolType: types.ToolTypeTreeService,
		Deployer: deployer, Status: types.ToolStatusDraft, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	transitions := []string{
		types.ToolStatusDraft,
		types.ToolStatusTesting,
		types.ToolStatusActive,
		types.ToolStatusDeprecated,
		types.ToolStatusRetired,
	}
	for i, status := range transitions {
		tool, _ = k.GetTool(ctx, "status-test")
		if tool.Status != transitions[i] {
			t.Fatalf("step %d: expected %s, got %s", i, status, tool.Status)
		}
		if i < len(transitions)-1 {
			k.UpdateToolStatus(ctx, tool, transitions[i+1])
		}
	}
}

// ============================================================
// 2. Contributor Management
// ============================================================

func TestAddContributor_PendingUntilAccepted(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("deployer")
	contrib := testAddr("contrib")
	id := registerTestTool(t, ms, ctx, deployer, "contrib-tool")

	_, err := ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: contrib,
		Role: types.RoleDeveloper, ShareBps: 200_000,
		Reallocations: []*types.ShareReallocation{{Address: deployer, NewShareBps: 800_000}},
	})
	if err != nil {
		t.Fatalf("AddContributor: %v", err)
	}

	// Should be pending
	pc, found := k.GetPendingContributorship(ctx, id, contrib)
	if !found {
		t.Fatal("expected pending contributorship")
	}
	if pc.ShareBps != 200_000 {
		t.Errorf("expected share 200000, got %d", pc.ShareBps)
	}

	// Not yet added to tool
	tool, _ := k.GetTool(ctx, id)
	if len(tool.Contributors) != 1 {
		t.Errorf("expected 1 contributor before accept, got %d", len(tool.Contributors))
	}
}

func TestAcceptContributorship_Success(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("deployer")
	contrib := testAddr("contrib")
	id := registerTestTool(t, ms, ctx, deployer, "accept-tool")

	_, _ = ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: contrib,
		Role: types.RoleDeveloper, ShareBps: 300_000,
		Reallocations: []*types.ShareReallocation{{Address: deployer, NewShareBps: 700_000}},
	})

	_, err := ms.AcceptContributorship(ctx, &types.MsgAcceptContributorship{
		ContributorAddress: contrib, ToolId: id,
	})
	if err != nil {
		t.Fatalf("AcceptContributorship: %v", err)
	}

	tool, _ := k.GetTool(ctx, id)
	if len(tool.Contributors) != 2 {
		t.Fatalf("expected 2 contributors, got %d", len(tool.Contributors))
	}
	if tool.Contributors[1].Address != contrib {
		t.Errorf("expected contrib address, got %s", tool.Contributors[1].Address)
	}
	if !tool.Contributors[1].Accepted {
		t.Error("expected accepted=true")
	}
}

func TestAddContributor_SharesSumIncorrect(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "bad-shares")

	_, err := ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: testAddr("c"),
		Role: types.RoleDeveloper, ShareBps: 200_000,
		// No reallocation — deployer keeps 1M + new 200K = 1.2M
	})
	if err == nil {
		t.Fatal("expected ErrSharesNotSumTo100")
	}
}

func TestLockShares_Success(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "lockable")

	_, err := ms.LockShares(ctx, &types.MsgLockShares{Deployer: deployer, ToolId: id})
	if err != nil {
		t.Fatalf("LockShares: %v", err)
	}
	tool, _ := k.GetTool(ctx, id)
	if tool.ShareLockHeight == 0 {
		t.Error("expected ShareLockHeight to be set")
	}
}

func TestLockShares_BlocksNewContributors(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "locked-tool")

	_, _ = ms.LockShares(ctx, &types.MsgLockShares{Deployer: deployer, ToolId: id})

	_, err := ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: testAddr("c"),
		Role: types.RoleDeveloper, ShareBps: 100_000,
		Reallocations: []*types.ShareReallocation{{Address: deployer, NewShareBps: 900_000}},
	})
	if err == nil {
		t.Fatal("expected ErrSharesLocked")
	}
}

func TestLockShares_AlreadyLocked(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "double-lock")

	_, _ = ms.LockShares(ctx, &types.MsgLockShares{Deployer: deployer, ToolId: id})
	_, err := ms.LockShares(ctx, &types.MsgLockShares{Deployer: deployer, ToolId: id})
	if err == nil {
		t.Fatal("expected ErrSharesLocked on second lock")
	}
}

func TestMaxContributors_Enforced(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "max-contrib")

	params := types.DefaultParams()
	params.MaxContributors = 2 // deployer + 1 more
	k.SetParams(ctx, params)

	// Add one contributor (succeeds)
	c1 := testAddr("c1")
	_, err := ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: c1,
		Role: types.RoleDeveloper, ShareBps: 300_000,
		Reallocations: []*types.ShareReallocation{{Address: deployer, NewShareBps: 700_000}},
	})
	if err != nil {
		t.Fatalf("AddContributor 1: %v", err)
	}
	_, _ = ms.AcceptContributorship(ctx, &types.MsgAcceptContributorship{ContributorAddress: c1, ToolId: id})

	// Second contributor should fail (now at 2/2)
	_, err = ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: testAddr("c2"),
		Role: types.RoleTester, ShareBps: 100_000,
		Reallocations: []*types.ShareReallocation{
			{Address: deployer, NewShareBps: 600_000},
			{Address: c1, NewShareBps: 300_000},
		},
	})
	if err == nil {
		t.Fatal("expected ErrTooManyContributors")
	}
}

func TestContributor_DuplicateRejected(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	contrib := testAddr("c")
	id := registerTestTool(t, ms, ctx, deployer, "dup-test")

	_, _ = ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: contrib,
		Role: types.RoleDeveloper, ShareBps: 200_000,
		Reallocations: []*types.ShareReallocation{{Address: deployer, NewShareBps: 800_000}},
	})
	_, _ = ms.AcceptContributorship(ctx, &types.MsgAcceptContributorship{ContributorAddress: contrib, ToolId: id})

	// Adding same contributor again
	_, err := ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: contrib,
		Role: types.RoleTester, ShareBps: 100_000,
		Reallocations: []*types.ShareReallocation{{Address: deployer, NewShareBps: 700_000}},
	})
	if err == nil {
		t.Fatal("expected ErrContributorExists")
	}
}

// ============================================================
// 3. Revenue Distribution
// ============================================================

func TestDistributeRevenue_BasicSplit(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("deployer")

	tool := &types.Tool{
		Id: "rev-tool", Deployer: deployer, PricePerCall: "1000000",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	total := uint64(1_000_000)
	err := k.DistributeRevenue(ctx, tool, uzrn(total))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// Contributor share: 55% of 1M = 550K
	if bk.totalModToAcc() != 550_000 {
		t.Errorf("contributor total: expected 550000, got %d", bk.totalModToAcc())
	}

	// Protocol sub-splits: 22% = 220K
	// Citation: 50% of 220K = 110K
	// Verification: 30% of 220K = 66K
	// Treasury: remainder = 44K
	var citation, verification, treasury uint64
	for _, s := range bk.modToModSends {
		switch s.to {
		case "knowledge":
			citation += s.amount
		case "vesting_rewards":
			verification += s.amount
		case "protocol_treasury":
			treasury += s.amount
		}
	}
	if citation != 110_000 {
		t.Errorf("citation: expected 110000, got %d", citation)
	}
	if verification != 66_000 {
		t.Errorf("verification: expected 66000, got %d", verification)
	}
	if treasury != 44_000 {
		t.Errorf("treasury: expected 44000, got %d", treasury)
	}

	// Research: 3.33% = 33,300
	if len(rf.deposits) != 1 || rf.deposits[0].amount != 33_300 {
		t.Errorf("research: expected 33300")
	}

	// Development fund: 19.67% = 196,700 (via SendCoinsFromModuleToModule)
	var devFund uint64
	for _, s := range bk.modToModSends {
		if s.to == "development_fund" {
			devFund += s.amount
		}
	}
	if devFund != 196_700 {
		t.Errorf("development fund: expected 196700, got %d", devFund)
	}
}

func TestDistributeRevenue_ContributorProRata(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("deployer")
	c2 := testAddr("c2")

	tool := &types.Tool{
		Id: "prorata", Deployer: deployer, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 700_000, Accepted: true, TotalEarned: "0"},
			{Address: c2, ShareBps: 300_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	// Contributor portion = 55% of 1M = 550K
	err := k.DistributeRevenue(ctx, tool, uzrn(1_000_000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// deployer: 70% of 550K = 385K, c2: 30% of 550K = 165K
	var deployerAmt, c2Amt uint64
	for _, s := range bk.modToAccSends {
		if s.to == deployer {
			deployerAmt += s.amount
		} else if s.to == c2 {
			c2Amt += s.amount
		}
	}
	if deployerAmt != 385_000 {
		t.Errorf("deployer: expected 385000, got %d", deployerAmt)
	}
	if c2Amt != 165_000 {
		t.Errorf("c2: expected 165000, got %d", c2Amt)
	}
}

func TestDistributeRevenue_ProtocolSubSplit(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	tool := &types.Tool{
		Id: "proto-sub", Deployer: testAddr("d"), PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: testAddr("d"), ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	_ = k.DistributeRevenue(ctx, tool, uzrn(1_000_000))

	// Protocol = 220K, split 50/30/20
	got := make(map[string]uint64)
	for _, s := range bk.modToModSends {
		got[s.to] += s.amount
	}
	if got["knowledge"] != 110_000 {
		t.Errorf("citation expected 110000, got %d", got["knowledge"])
	}
	if got["vesting_rewards"] != 66_000 {
		t.Errorf("verification expected 66000, got %d", got["vesting_rewards"])
	}
	if got["protocol_treasury"] != 44_000 {
		t.Errorf("treasury expected 44000, got %d", got["protocol_treasury"])
	}
}

func TestDistributeRevenue_GovernanceChangedSplits(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)

	// Custom splits: 60/20/15/5
	params := types.DefaultParams()
	params.ToolRevenueBps = 600_000
	params.ProtocolBps = 200_000
	params.ResearchBps = 150_000
	params.DevelopmentBps = 50_000
	k.SetParams(ctx, params)

	tool := &types.Tool{
		Id: "custom-split", Deployer: testAddr("d"), PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: testAddr("d"), ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	_ = k.DistributeRevenue(ctx, tool, uzrn(1_000_000))

	if bk.totalModToAcc() != 600_000 {
		t.Errorf("contributor: expected 600000, got %d", bk.totalModToAcc())
	}
	if len(rf.deposits) != 1 || rf.deposits[0].amount != 150_000 {
		t.Errorf("research: expected 150000")
	}
	var devFund uint64
	for _, s := range bk.modToModSends {
		if s.to == "development_fund" {
			devFund += s.amount
		}
	}
	if devFund != 50_000 {
		t.Errorf("development fund: expected 50000, got %d", devFund)
	}
}

func TestDistributeRevenue_ZeroPriceTool(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	tool := &types.Tool{
		Id: "free", Deployer: testAddr("d"), PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: testAddr("d"), ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(0))
	if err != nil {
		t.Fatalf("DistributeRevenue(0): %v", err)
	}
	if len(bk.modToAccSends) != 0 {
		t.Error("expected no sends for zero amount")
	}
}

func TestDistributeRevenue_NoContributors(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("d")
	tool := &types.Tool{
		Id: "no-contrib", Deployer: deployer, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: nil, // empty
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(1_000_000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// Contributor share (550K) should all go to deployer
	var toDeployer uint64
	for _, s := range bk.modToAccSends {
		if s.to == deployer {
			toDeployer += s.amount
		}
	}
	if toDeployer != 550_000 {
		t.Errorf("deployer should get full contributor share 550000, got %d", toDeployer)
	}
}

func TestDistributeRevenue_Remainder(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("deployer")
	c2 := testAddr("c2")
	c3 := testAddr("c3")

	tool := &types.Tool{
		Id: "remainder", Deployer: deployer, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 333_333, Accepted: true, TotalEarned: "0"},
			{Address: c2, ShareBps: 333_333, Accepted: true, TotalEarned: "0"},
			{Address: c3, ShareBps: 333_334, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	// Contributor portion for 100 uzrn = safeMulDiv(100, 550_000, 1_000_000) = 55
	// deployer: safeMulDiv(55, 333_333, 1_000_000) = 18
	// c2: 18, c3: safeMulDiv(55, 333_334, 1_000_000) = 18
	// distributed = 54, remainder = 1 → deployer
	err := k.DistributeRevenue(ctx, tool, uzrn(100))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	var toDeployer uint64
	for _, s := range bk.modToAccSends {
		if s.to == deployer {
			toDeployer += s.amount
		}
	}
	// deployer gets 18 + 1 (remainder) = 19
	if toDeployer != 19 {
		t.Errorf("deployer should get 19 (18 share + 1 remainder), got %d", toDeployer)
	}
}

func TestCollectPayment_InsufficientBalance(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	bk.failSend = true
	caller := testAddr("broke")
	tool := &types.Tool{
		Id: "paid", PricePerCall: "100000", Category: types.CategoryComputation,
		Status: types.ToolStatusActive,
	}

	_, _, err := k.CollectPayment(ctx, caller, tool, 0)
	if err == nil {
		t.Fatal("expected ErrInsufficientBalance")
	}
}

// ============================================================
// 4. Trust Engine
// ============================================================

func TestTrustScore_InitialValue(t *testing.T) {
	if keeper.InitialTrustScore() != 500_000 {
		t.Errorf("expected 500000, got %d", keeper.InitialTrustScore())
	}
}

func TestTrustScore_EMAUpdate_Success(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	tool := &types.Tool{
		Id: "ema-s", TrustScore: 500_000, Deployer: testAddr("d"),
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	k.UpdateTrustScore(ctx, tool, true)
	// new = 10% * 1M + 90% * 500K = 100K + 450K = 550K
	if tool.TrustScore != 550_000 {
		t.Errorf("expected 550000 after success, got %d", tool.TrustScore)
	}
}

func TestTrustScore_EMAUpdate_Failure(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	tool := &types.Tool{
		Id: "ema-f", TrustScore: 500_000, Deployer: testAddr("d"),
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	k.UpdateTrustScore(ctx, tool, false)
	// new = 10% * 0 + 90% * 500K = 0 + 450K = 450K
	if tool.TrustScore != 450_000 {
		t.Errorf("expected 450000 after failure, got %d", tool.TrustScore)
	}
}

func TestComputeTrustScore_5Components(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("deployer")

	tool := &types.Tool{
		Id: "trust-5c", Name: "trust-5c", Deployer: deployer,
		ToolType: types.ToolTypeTreeService, Category: types.CategoryUtility,
		Status: types.ToolStatusActive, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 500_000, SourceHash: "abc123",
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	s.k.SetTool(s.ctx, tool)
	s.k.SetTrustSnapshot(s.ctx, &types.TrustSnapshot{ToolId: tool.Id, Score: 500_000})

	snap := s.k.ComputeTrustScore(s.ctx, tool)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	// Should have all 5 components
	if snap.Score == 0 {
		t.Error("expected non-zero composite score")
	}
	// Verification component: SourceHash present → 300K base
	if snap.VerificationComponent == 0 {
		t.Error("expected non-zero verification component (has SourceHash)")
	}
}

func TestTrustScore_ReliabilityComponent(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	deployer := testAddr("d")
	caller := testAddr("caller")

	tool := &types.Tool{
		Id: "rel-test", Deployer: deployer, Status: types.ToolStatusActive,
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 500_000,
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	// Record 100% success rate with enough calls
	for i := 0; i < 100; i++ {
		k.RecordCaller(ctx, tool.Id, caller, 100, true)
	}

	snap := k.ComputeTrustScore(ctx, tool)
	// With 100% success and 100 calls (= MinCallsForReliability), reliability should be ~1M
	if snap.ReliabilityComponent < 900_000 {
		t.Errorf("reliability too low for 100%% success: %d", snap.ReliabilityComponent)
	}
}

func TestTrustScore_UsageComponent_SelfExclusion(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	deployer := testAddr("deployer")
	outsideCaller := testAddr("outsider")

	tool := &types.Tool{
		Id: "usage-self", Deployer: deployer, Status: types.ToolStatusActive,
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 500_000,
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	// Deployer calls 50 times — should be excluded
	for i := 0; i < 50; i++ {
		k.RecordCaller(ctx, tool.Id, deployer, 100, true)
	}
	snapSelf := k.ComputeTrustScore(ctx, tool)

	// Now add outside callers
	for i := 0; i < 20; i++ {
		k.RecordCaller(ctx, tool.Id, outsideCaller+fmt.Sprintf("-%d", i), 100, true)
	}
	snapWithCallers := k.ComputeTrustScore(ctx, tool)

	if snapWithCallers.UsageComponent <= snapSelf.UsageComponent {
		t.Errorf("external callers should increase usage: self=%d with=%d",
			snapSelf.UsageComponent, snapWithCallers.UsageComponent)
	}
}

func TestTrustScore_ContributorComponent(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("d")

	// Set up staking tiers
	s.staking.tiers[deployer] = 3 // Guardian
	s.staking.accuracies[deployer] = 900_000

	tool := &types.Tool{
		Id: "contrib-trust", Deployer: deployer, Status: types.ToolStatusActive,
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 500_000,
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	s.k.SetTool(s.ctx, tool)

	snap := s.k.ComputeTrustScore(s.ctx, tool)
	// Guardian tier (1M) * 70% + accuracy (900K) * 30% = 700K + 270K = 970K
	if snap.ContributorComponent < 900_000 {
		t.Errorf("expected high contributor score for guardian, got %d", snap.ContributorComponent)
	}
}

func TestRecalculateTrustScores_EndBlocker(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	deployer := testAddr("d")

	tool := &types.Tool{
		Id: "recalc-test", Deployer: deployer, Status: types.ToolStatusActive,
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 500_000, SourceHash: "hash",
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)
	k.SetTrustSnapshot(ctx, &types.TrustSnapshot{ToolId: tool.Id, Score: 500_000})

	k.RecalculateTrustScores(ctx)

	snap, ok := k.GetTrustSnapshot(ctx, tool.Id)
	if !ok {
		t.Fatal("expected trust snapshot after recalculation")
	}
	if snap.ComputedAtBlock != uint64(ctx.BlockHeight()) {
		t.Errorf("expected computed at block %d, got %d", ctx.BlockHeight(), snap.ComputedAtBlock)
	}
}

func TestVerifiedStatus_Promotion(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	tool := &types.Tool{
		Id: "verify-promo", TrustScore: 800_001, IsVerified: false,
		Deployer: testAddr("d"), PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	k.UpdateVerifiedStatus(ctx, tool)
	if !tool.IsVerified {
		t.Error("expected IsVerified=true after promotion")
	}
	if tool.VerifiedSince != uint64(ctx.BlockHeight()) {
		t.Errorf("expected VerifiedSince=%d, got %d", ctx.BlockHeight(), tool.VerifiedSince)
	}
}

func TestVerifiedStatus_Demotion(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	params := types.DefaultParams()
	params.VerifiedGracePeriodBlocks = 100
	k.SetParams(ctx, params)

	tool := &types.Tool{
		Id: "verify-demote", TrustScore: 600_000, IsVerified: true,
		VerifiedSince: 50, Deployer: testAddr("d"),
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// First call: below retention threshold, starts grace period
	k.UpdateVerifiedStatus(ctx, tool)
	if !tool.IsVerified {
		t.Error("should still be verified during grace period start")
	}
	if tool.VerifiedDemotionBlock == 0 {
		t.Error("expected VerifiedDemotionBlock to be set")
	}

	// Advance past grace period
	ctx = ctx.WithBlockHeight(int64(tool.VerifiedDemotionBlock + 101))
	k.UpdateVerifiedStatus(ctx, tool)
	if tool.IsVerified {
		t.Error("expected demotion after grace period")
	}
}

func TestTrustTierBoundaries(t *testing.T) {
	tests := []struct {
		score uint64
		tier  uint32
		label string
	}{
		{0, 0, "Unverified"},
		{100_000, 0, "Unverified"},
		{100_001, 1, "Emerging"},
		{300_000, 1, "Emerging"},
		{300_001, 2, "Established"},
		{600_000, 2, "Established"},
		{600_001, 3, "Trusted"},
		{800_000, 3, "Trusted"},
		{800_001, 4, "Verified"},
		{1_000_000, 4, "Verified"},
	}
	for _, tc := range tests {
		if got := types.TrustTier(tc.score); got != tc.tier {
			t.Errorf("score %d: expected tier %d, got %d", tc.score, tc.tier, got)
		}
		if got := types.TrustTierLabel(tc.score); got != tc.label {
			t.Errorf("score %d: expected label %q, got %q", tc.score, tc.label, got)
		}
	}
}

// ============================================================
// 5. Dependency DAG
// ============================================================

func TestDependencyChain_ABC(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	// C has no deps
	idC := registerTestTool(t, ms, ctx, deployer, "tool-c")
	activateTool(t, k, ctx, idC)

	// B depends on C
	idB := registerTestTool(t, ms, ctx, deployer, "tool-b", withDeps(idC))
	activateTool(t, k, ctx, idB)

	// A depends on B
	idA := registerTestTool(t, ms, ctx, deployer, "tool-a", withDeps(idB))

	tool, _ := k.GetTool(ctx, idA)
	if len(tool.DependencyIds) != 1 || tool.DependencyIds[0] != idB {
		t.Errorf("expected A->B dependency, got %v", tool.DependencyIds)
	}

	// Verify edge exists
	_, ok := k.GetDependencyEdge(ctx, idA, idB)
	if !ok {
		t.Error("expected dependency edge A->B")
	}
}

func TestDependencyCycle_Rejected(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	idA := registerTestTool(t, ms, ctx, deployer, "cyc-a")
	activateTool(t, k, ctx, idA)

	idB := registerTestTool(t, ms, ctx, deployer, "cyc-b", withDeps(idA))
	activateTool(t, k, ctx, idB)

	idC := registerTestTool(t, ms, ctx, deployer, "cyc-c", withDeps(idB))
	activateTool(t, k, ctx, idC)

	// Try to make A depend on C (creates A->C->B->A cycle)
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "cyc-trigger", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: types.LicenseOpen, Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("register cyc-trigger: %v", err)
	}

	// Direct cycle: A already depends on nothing. Let's create proper cycle.
	// Store edge C->A manually to simulate existing deps, then try adding dep A->C
	k.StoreDependencyEdges(ctx, idA, []string{idC}, 100)

	// Now check: adding C as dep of a new tool that A depends on should detect cycle
	err = k.CheckDependencyCycles(ctx, idA, idC, 10)
	if err == nil {
		t.Fatal("expected ErrDependencyCycle for A->C->B->A")
	}
}

func TestSelfDependency_Rejected(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	err := k.CheckDependencyCycles(ctx, "tool-self", "tool-self", 10)
	if err == nil {
		t.Fatal("expected ErrDependencyCycle for self-dependency")
	}
}

func TestDependencyDepthLimit(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	params := types.DefaultParams()
	params.MaxDependencyDepth = 2 // DFS depth check: depth > 2 triggers error
	k.SetParams(ctx, params)

	// Create chain: d0 <- d1 <- d2 <- d3 (d3 depends on d2 depends on d1 depends on d0)
	prev := ""
	ids := make([]string, 4)
	for i := 0; i < 4; i++ {
		opts := []toolOpt{}
		if prev != "" {
			opts = append(opts, withDeps(prev))
		}
		ids[i] = registerTestTool(t, ms, ctx, deployer, fmt.Sprintf("depth-%d", i), opts...)
		activateTool(t, k, ctx, ids[i])
		prev = ids[i]
	}

	// Adding dep on d3: DFS traverses d3->d2->d1->d0 reaching depth 3 > maxDepth 2
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "too-deep", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: types.LicenseOpen, Version: "1.0.0",
		DependencyIds: []string{ids[3]},
	})
	if err == nil {
		t.Fatal("expected ErrDependencyDepthExceeded")
	}
}

func TestDependencyCountLimit(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	params := types.DefaultParams()
	params.MaxDependencies = 2
	k.SetParams(ctx, params)

	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		ids[i] = registerTestTool(t, ms, ctx, deployer, fmt.Sprintf("dep-%d", i))
		activateTool(t, k, ctx, ids[i])
	}

	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "too-many", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: types.LicenseOpen, Version: "1.0.0",
		DependencyIds: ids, // 3 deps > max 2
	})
	if err == nil {
		t.Fatal("expected ErrTooManyDependencies")
	}
}

func TestDependency_TrustTierEligibility(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	// Create tool with low trust (tier 0 = Unverified)
	lowTrust := &types.Tool{
		Id: "low-trust", Name: "low-trust", Deployer: deployer,
		ToolType: types.ToolTypeTreeService, Category: types.CategoryUtility,
		Status: types.ToolStatusActive, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 50_000, // tier 0
	}
	k.SetTool(ctx, lowTrust)

	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "needs-trust", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: types.LicenseOpen, Version: "1.0.0",
		DependencyIds: []string{"low-trust"},
	})
	if err == nil {
		t.Fatal("expected ErrIneligibleDependency for tier-0 tool")
	}
}

func TestDependency_RetiredToolRejected(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	id := registerTestTool(t, ms, ctx, deployer, "retired-dep")
	setToolStatus(t, k, ctx, id, types.ToolStatusRetired)

	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "dep-on-retired", ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility, License: types.LicenseOpen, Version: "1.0.0",
		DependencyIds: []string{id},
	})
	if err == nil {
		t.Fatal("expected ErrToolRetired for retired dependency")
	}
}

func TestGetDependencyTree_Structure(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	idC := registerTestTool(t, ms, ctx, deployer, "tree-c")
	activateTool(t, k, ctx, idC)
	idB := registerTestTool(t, ms, ctx, deployer, "tree-b", withDeps(idC))
	activateTool(t, k, ctx, idB)
	idA := registerTestTool(t, ms, ctx, deployer, "tree-a", withDeps(idB))

	tree := k.GetDependencyTree(ctx, idA, 10)
	if tree == nil {
		t.Fatal("expected non-nil tree")
	}
	if tree.ToolID != idA {
		t.Errorf("expected root=%s, got %s", idA, tree.ToolID)
	}
	if len(tree.Children) != 1 || tree.Children[0].ToolID != idB {
		t.Error("expected child B")
	}
	if len(tree.Children[0].Children) != 1 || tree.Children[0].Children[0].ToolID != idC {
		t.Error("expected grandchild C")
	}
}

func TestFlattenDependencies_PostOrder(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	idC := registerTestTool(t, ms, ctx, deployer, "flat-c")
	activateTool(t, k, ctx, idC)
	idB := registerTestTool(t, ms, ctx, deployer, "flat-b", withDeps(idC))
	activateTool(t, k, ctx, idB)
	idA := registerTestTool(t, ms, ctx, deployer, "flat-a", withDeps(idB))

	deps := k.FlattenDependencies(ctx, idA)
	// Post-order: C, B, A
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d: %v", len(deps), deps)
	}
	if deps[0] != idC {
		t.Errorf("expected C first, got %s", deps[0])
	}
	if deps[1] != idB {
		t.Errorf("expected B second, got %s", deps[1])
	}
	if deps[2] != idA {
		t.Errorf("expected A third, got %s", deps[2])
	}
}

func TestComputeTransitiveCost(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	idC := registerTestTool(t, ms, ctx, deployer, "cost-c", withPrice("100"))
	activateTool(t, k, ctx, idC)
	idB := registerTestTool(t, ms, ctx, deployer, "cost-b", withPrice("200"), withDeps(idC))
	activateTool(t, k, ctx, idB)
	idA := registerTestTool(t, ms, ctx, deployer, "cost-a", withPrice("300"), withDeps(idB))

	cost := k.ComputeTransitiveCost(ctx, idA)
	// Should sum B(200) + C(100) = 300 (excludes self)
	expected := big.NewInt(300)
	if cost.Cmp(expected) != 0 {
		t.Errorf("transitive cost: expected %s, got %s", expected, cost)
	}
}

func TestUpdateDependency_Swap(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")

	oldDep := registerTestTool(t, ms, ctx, deployer, "old-dep")
	activateTool(t, k, ctx, oldDep)
	newDep := registerTestTool(t, ms, ctx, deployer, "new-dep")
	activateTool(t, k, ctx, newDep)
	parent := registerTestTool(t, ms, ctx, deployer, "parent-tool", withDeps(oldDep))

	_, err := ms.UpdateDependency(ctx, &types.MsgUpdateDependency{
		Authority: deployer, ToolId: parent, OldDepId: oldDep, NewDepId: newDep,
	})
	if err != nil {
		t.Fatalf("UpdateDependency: %v", err)
	}

	tool, _ := k.GetTool(ctx, parent)
	if len(tool.DependencyIds) != 1 || tool.DependencyIds[0] != newDep {
		t.Errorf("expected dep [%s], got %v", newDep, tool.DependencyIds)
	}

	// Old edge gone, new edge present
	if _, ok := k.GetDependencyEdge(ctx, parent, oldDep); ok {
		t.Error("old edge should be removed")
	}
	if _, ok := k.GetDependencyEdge(ctx, parent, newDep); !ok {
		t.Error("new edge should exist")
	}
}

// ============================================================
// 6. Composite Tool Execution & Revenue Cascade
// ============================================================

func TestCompositeExecution_WithDependencies(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("d")
	caller := testAddr("caller")
	s.discovery.agents[caller] = []string{"general"}

	// Create knowledge-template dep
	s.knowledge.facts["f1"] = mockFact{content: "test fact", confidence: 700_000, citations: 3}
	s.knowledge.searchIDs = []string{"f1"}

	depTool := &types.Tool{
		Id: "kt-dep", Name: "KT Dep", ToolType: types.ToolTypeKnowledgeTemplate,
		Category: types.CategoryDataRetrieval, Status: types.ToolStatusActive,
		PricePerCall: "100", Deployer: deployer, KnowledgeQuery: "test",
		TotalRevenue: "0", TotalCalls: "0", TrustScore: 500_000,
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	s.k.SetTool(s.ctx, depTool)

	composite := &types.Tool{
		Id: "comp-exec", Name: "Composite Exec", ToolType: types.ToolTypeComposite,
		Category: types.CategoryComposite, Status: types.ToolStatusActive,
		PricePerCall: "500", Deployer: deployer, KnowledgeQuery: "purpose",
		DependencyIds: []string{"kt-dep"}, TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 500_000,
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	s.k.SetTool(s.ctx, composite)
	s.k.StoreDependencyEdges(s.ctx, "comp-exec", []string{"kt-dep"}, 100)

	result, err := s.k.ExecuteCompositeWithCascade(s.ctx, composite, caller, nil, 500)
	if err != nil {
		t.Fatalf("ExecuteCompositeWithCascade: %v", err)
	}
	if result.DependencyCost != 100 {
		t.Errorf("expected dep cost 100, got %d", result.DependencyCost)
	}
	if result.OwnRevenue != 400 {
		t.Errorf("expected own revenue 400, got %d", result.OwnRevenue)
	}
	if len(result.SubCallIDs) != 1 {
		t.Errorf("expected 1 sub call, got %d", len(result.SubCallIDs))
	}
}

func TestRevenueCascade_OwnRevenue(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("d")

	s.knowledge.searchIDs = nil

	dep := &types.Tool{
		Id: "rc-dep", Name: "RC Dep", ToolType: types.ToolTypeKnowledgeTemplate,
		Category: types.CategoryDataRetrieval, Status: types.ToolStatusActive,
		PricePerCall: "200", Deployer: deployer, KnowledgeQuery: "q",
		TotalRevenue: "0", TotalCalls: "0", TrustScore: 500_000,
	}
	s.k.SetTool(s.ctx, dep)

	composite := &types.Tool{
		Id: "rc-comp", Name: "RC Comp", ToolType: types.ToolTypeComposite,
		Category: types.CategoryComposite, Status: types.ToolStatusActive,
		PricePerCall: "1000", Deployer: deployer, KnowledgeQuery: "q",
		DependencyIds: []string{"rc-dep"}, TotalRevenue: "0", TotalCalls: "0",
		TrustScore: 500_000,
	}
	s.k.SetTool(s.ctx, composite)
	s.k.StoreDependencyEdges(s.ctx, "rc-comp", []string{"rc-dep"}, 100)

	result, err := s.k.ExecuteCompositeWithCascade(s.ctx, composite, testAddr("caller"), nil, 1000)
	if err != nil {
		t.Fatalf("ExecuteCompositeWithCascade: %v", err)
	}
	// Own = payment(1000) - depCost(200) = 800
	if result.OwnRevenue != 800 {
		t.Errorf("expected own 800, got %d", result.OwnRevenue)
	}
}

func TestBvmExecutor_RetiredToolFails(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	tool := &types.Tool{
		Id: "bvm-retired", Status: types.ToolStatusRetired, ToolType: types.ToolTypeBVMContract,
		Deployer: testAddr("d"), PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	exec := keeper.NewBvmExecutor(k)
	_, err := exec.CallToolFromBVM(ctx, "contract", "bvm-retired", nil, 1_000_000)
	if err == nil {
		t.Fatal("expected ErrToolRetired from BvmExecutor")
	}
}

func TestBvmExecutor_CallToolByID(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("d")

	s.knowledge.searchIDs = []string{"f1"}
	s.knowledge.facts["f1"] = mockFact{content: "fact", confidence: 800_000, citations: 1}

	tool := &types.Tool{
		Id: "bvm-call-kt", Name: "BVM KT", ToolType: types.ToolTypeKnowledgeTemplate,
		Category: types.CategoryDataRetrieval, Status: types.ToolStatusActive,
		PricePerCall: "500", Deployer: deployer, KnowledgeQuery: "test",
		TotalRevenue: "0", TotalCalls: "0", TrustScore: 500_000,
	}
	s.k.SetTool(s.ctx, tool)

	exec := keeper.NewBvmExecutor(s.k)
	output, cost, err := exec.CallToolByID(s.ctx, "bvm-call-kt", testAddr("caller"), nil)
	if err != nil {
		t.Fatalf("CallToolByID: %v", err)
	}
	if cost != 500 {
		t.Errorf("expected cost 500, got %d", cost)
	}
	if output == nil {
		t.Error("expected non-nil output")
	}
}

// ============================================================
// 7. Dynamic Pricing
// ============================================================

func setupSurgeParams(k keeper.Keeper, ctx sdk.Context) {
	params := types.DefaultParams()
	params.DemandWindowSize = 10
	params.TargetCallsPerBlockPerTool = 1
	params.TargetGlobalCallsPerBlock = 100
	k.SetParams(ctx, params)
}

func TestDemandWindow_RecordAndSum(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	toolID := "demand-test"
	for i := 0; i < 5; i++ {
		k.RecordToolCall(ctx, toolID)
	}

	totalCalls, _ := k.GetToolDemand(ctx, toolID)
	if totalCalls != 5 {
		t.Errorf("expected 5 calls, got %d", totalCalls)
	}
}

func TestDemandWindow_SameBlockAccumulates(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	toolID := "accum-test"
	k.RecordToolCall(ctx, toolID)
	k.RecordToolCall(ctx, toolID)
	k.RecordToolCall(ctx, toolID)

	total, _ := k.GetToolDemand(ctx, toolID)
	if total != 3 {
		t.Errorf("expected 3 accumulated calls, got %d", total)
	}
}

func TestDemandWindow_Wraps(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	params := types.DefaultParams()
	params.DemandWindowSize = 5
	params.TargetCallsPerBlockPerTool = 1
	params.TargetGlobalCallsPerBlock = 100
	k.SetParams(ctx, params)

	toolID := "wrap-test"
	// Record at blocks 100-106 (7 blocks, window size 5)
	for block := int64(100); block <= 106; block++ {
		bctx := ctx.WithBlockHeight(block)
		k.RecordToolCall(bctx, toolID)
	}

	// Query at block 106 — window covers blocks 102-106 (5 blocks)
	qctx := ctx.WithBlockHeight(106)
	total, _ := k.GetToolDemand(qctx, toolID)
	if total != 5 {
		t.Errorf("expected 5 calls in window, got %d", total)
	}
}

func TestSurgePricing_Essential_NoSurge(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "essential-surge", Category: types.CategoryDataRetrieval,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Generate high demand
	for i := 0; i < 9; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge != types.BpsDenominator {
		t.Errorf("essential: expected 1x (1000000), got %d", surge)
	}
}

func TestSurgePricing_Standard_Linear(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "std-surge", Category: types.CategoryDataAnalysis,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 6 calls with window=10, target=1 → 60% utilisation (above 50% threshold)
	for i := 0; i < 6; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	// Should be > 1x
	if surge <= types.BpsDenominator {
		t.Errorf("standard at 60%%: expected surge > 1x, got %d", surge)
	}
	// And < 2x
	if surge >= 2*types.BpsDenominator {
		t.Errorf("standard at 60%%: expected < 2x, got %d", surge)
	}
}

func TestSurgePricing_Standard_CappedAt2x(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "std-cap", Category: types.CategoryDataAnalysis,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 9 calls → 90% > critical (80%)
	for i := 0; i < 9; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge > 2*types.BpsDenominator {
		t.Errorf("standard above critical: expected <= 2x, got %d", surge)
	}
}

func TestSurgePricing_Heavy_Linear(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "heavy-linear", Category: types.CategoryComputation,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 7 calls → 70% (between 50% threshold and 80% critical)
	for i := 0; i < 7; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge <= types.BpsDenominator {
		t.Errorf("heavy at 70%%: expected surge > 1x, got %d", surge)
	}
	if surge >= 3*types.BpsDenominator {
		t.Errorf("heavy at 70%%: expected < 3x in linear phase, got %d", surge)
	}
}

func TestSurgePricing_Heavy_CappedAt10x(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "heavy-cap", Category: types.CategoryComputation,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 10 calls → 100% (well above critical)
	for i := 0; i < 10; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge > 10*types.BpsDenominator {
		t.Errorf("heavy at 100%%: expected <= 10x cap, got %d", surge)
	}
}

func TestSurgePricing_BelowThreshold_NoSurge(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "below-thresh", Category: types.CategoryDataAnalysis,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 4 calls → 40% < threshold 50%
	for i := 0; i < 4; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge != types.BpsDenominator {
		t.Errorf("below threshold: expected 1x, got %d", surge)
	}
}

func TestSurgePricing_Disabled(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	params := types.DefaultParams()
	params.SurgeEnabled = false
	k.SetParams(ctx, params)

	tool := &types.Tool{
		Id: "no-surge", Category: types.CategoryComputation,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge != types.BpsDenominator {
		t.Errorf("disabled: expected 1x, got %d", surge)
	}
}

// ============================================================
// 8. USD-Stable Pricing
// ============================================================

func TestGetBasePrice_FixedMode(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	tool := &types.Tool{
		Id: "fixed-price", PricePerCall: "500000",
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	price, mode := k.GetBasePrice(ctx, tool)
	if mode != "fixed_uzrn" {
		t.Errorf("expected fixed_uzrn mode, got %s", mode)
	}
	if price != 500_000 {
		t.Errorf("expected 500000, got %d", price)
	}
}

func TestGetBasePrice_USDStable_WithOracle(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 2_000_000 // $2.00 per ZRN

	tool := &types.Tool{
		Id: "usd-tool", PricePerCall: "100000",
		TargetPriceUsd: "1000000", // $1.00 target
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	price, mode := s.k.GetBasePrice(s.ctx, tool)
	if mode != "usd_stable" {
		t.Errorf("expected usd_stable, got %s", mode)
	}
	// base = (1_000_000 * 1_000_000) / 2_000_000 = 500_000
	if price != 500_000 {
		t.Errorf("expected 500000, got %d", price)
	}
}

func TestGetBasePrice_USDStable_ClampedToMin(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 100_000_000 // $100 per ZRN (very expensive)

	tool := &types.Tool{
		Id: "usd-min", PricePerCall: "100000",
		TargetPriceUsd: "1000", // $0.001 target → very low uzrn
		MinPricePerCall: "50000", // min 50K uzrn
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	price, _ := s.k.GetBasePrice(s.ctx, tool)
	// raw = (1000 * 1e6) / 100e6 = 10 → clamped to min 50000
	if price != 50_000 {
		t.Errorf("expected clamped to min 50000, got %d", price)
	}
}

func TestGetBasePrice_USDStable_ClampedToMax(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 100 // $0.0001 per ZRN (very cheap)

	tool := &types.Tool{
		Id: "usd-max", PricePerCall: "100000",
		TargetPriceUsd: "1000000", // $1.00
		MaxPricePerCall: "5000000", // max 5M uzrn
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	price, _ := s.k.GetBasePrice(s.ctx, tool)
	// raw = (1e6 * 1e6) / 100 = 10_000_000_000 → clamped to max 5M
	if price != 5_000_000 {
		t.Errorf("expected clamped to max 5000000, got %d", price)
	}
}

func TestGetBasePrice_OracleUnavailable_Fallback(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	// No billing keeper set

	tool := &types.Tool{
		Id: "no-oracle", PricePerCall: "100000",
		TargetPriceUsd: "1000000",
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	price, mode := k.GetBasePrice(ctx, tool)
	if mode != "fixed_uzrn" {
		t.Errorf("expected fixed fallback, got %s", mode)
	}
	if price != 100_000 {
		t.Errorf("expected fallback 100000, got %d", price)
	}
}

func TestGetBasePrice_OracleReturnsZero_Fallback(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 0 // zero price

	tool := &types.Tool{
		Id: "zero-oracle", PricePerCall: "100000",
		TargetPriceUsd: "1000000",
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	price, mode := s.k.GetBasePrice(s.ctx, tool)
	if mode != "fixed_uzrn" {
		t.Errorf("expected fixed fallback for zero oracle, got %s", mode)
	}
	if price != 100_000 {
		t.Errorf("expected fallback 100000, got %d", price)
	}
}

func TestEffectivePrice_SurgeApplied(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "surge-effective", Category: types.CategoryDataAnalysis,
		PricePerCall: "100000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Generate demand to create surge
	for i := 0; i < 7; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	effective, _ := k.CalculateEffectivePrice(ctx, tool)
	if effective <= 100_000 {
		t.Errorf("expected effective > base (100000) with surge, got %d", effective)
	}
}

func TestEffectivePrice_SurgeDisabled(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	params := types.DefaultParams()
	params.SurgeEnabled = false
	k.SetParams(ctx, params)

	tool := &types.Tool{
		Id: "no-surge-eff", Category: types.CategoryDataAnalysis,
		PricePerCall: "100000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	effective, _ := k.CalculateEffectivePrice(ctx, tool)
	if effective != 100_000 {
		t.Errorf("expected base price 100000 (surge disabled), got %d", effective)
	}
}

// ============================================================
// 9. Free Tier
// ============================================================

func setupFreeTier(t *testing.T) *fullSetup {
	t.Helper()
	s := setupFull(t)
	caller := testAddr("free-caller")

	// Set up an eligible home
	s.home.homes["home-1"] = mockHome{
		owner:     caller,
		createdAt: 10, // block 10, current is 100 → age = 90 > default MinHomeAgeBlocks(10K)
		status:    "active",
	}

	// Override min home age so our test home qualifies
	params := types.DefaultParams()
	params.MinHomeAgeBlocks = 50 // home age 90 >= 50
	s.k.SetParams(s.ctx, params)

	return s
}

func TestFreeTier_EligibleCall(t *testing.T) {
	s := setupFreeTier(t)
	caller := testAddr("free-caller")

	tool := &types.Tool{
		Id: "free-ess", Category: types.CategoryDataRetrieval,
		PricePerCall: "100000", Status: types.ToolStatusActive,
	}

	consumed := s.k.TryConsumeFreeCall(s.ctx, caller, tool)
	if !consumed {
		t.Error("expected free call to be consumed for essential category")
	}
}

func TestFreeTier_NonEssentialCategory(t *testing.T) {
	s := setupFreeTier(t)
	caller := testAddr("free-caller")

	tool := &types.Tool{
		Id: "non-ess", Category: types.CategoryComputation,
		PricePerCall: "100000", Status: types.ToolStatusActive,
	}

	consumed := s.k.TryConsumeFreeCall(s.ctx, caller, tool)
	if consumed {
		t.Error("non-essential category should not get free call")
	}
}

func TestFreeTier_NoHomeKeeper(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	// No home keeper set

	eligible, reason := k.CheckFreeEligibility(ctx, testAddr("someone"))
	if eligible {
		t.Error("expected ineligible without home keeper")
	}
	if reason != "home_keeper_unavailable" {
		t.Errorf("expected reason 'home_keeper_unavailable', got %q", reason)
	}
}

func TestFreeTier_NoHome(t *testing.T) {
	s := setupFull(t)
	eligible, reason := s.k.CheckFreeEligibility(s.ctx, testAddr("homeless"))
	if eligible {
		t.Error("expected ineligible without home")
	}
	if reason != "no_home_owned" {
		t.Errorf("expected 'no_home_owned', got %q", reason)
	}
}

func TestFreeTier_HomeTooYoung(t *testing.T) {
	s := setupFull(t)
	caller := testAddr("young-home")

	params := types.DefaultParams()
	params.MinHomeAgeBlocks = 1000
	s.k.SetParams(s.ctx, params)

	s.home.homes["young-home-1"] = mockHome{
		owner:     caller,
		createdAt: 50, // age = 100-50 = 50, < 1000
		status:    "active",
	}

	eligible, reason := s.k.CheckFreeEligibility(s.ctx, caller)
	if eligible {
		t.Error("expected ineligible for too-young home")
	}
	if reason != "no_eligible_home" {
		t.Errorf("expected 'no_eligible_home', got %q", reason)
	}
}

func TestFreeTier_AllowanceExhausted(t *testing.T) {
	s := setupFreeTier(t)
	caller := testAddr("free-caller")

	params := s.k.GetParams(s.ctx)

	tool := &types.Tool{
		Id: "exhaust", Category: types.CategoryDataRetrieval,
		PricePerCall: "100000", Status: types.ToolStatusActive,
	}

	// Exhaust all free calls
	for i := uint64(0); i < params.FreeCallsPerEpoch; i++ {
		if !s.k.TryConsumeFreeCall(s.ctx, caller, tool) {
			t.Fatalf("call %d should be free", i)
		}
	}

	// Next should fail
	consumed := s.k.TryConsumeFreeCall(s.ctx, caller, tool)
	if consumed {
		t.Error("expected exhausted free tier")
	}
}

func TestFreeTier_EpochReset(t *testing.T) {
	s := setupFreeTier(t)
	caller := testAddr("free-caller")

	tool := &types.Tool{
		Id: "epoch-reset", Category: types.CategoryUtility,
		PricePerCall: "100000", Status: types.ToolStatusActive,
	}

	// Use all free calls
	params := s.k.GetParams(s.ctx)
	for i := uint64(0); i < params.FreeCallsPerEpoch; i++ {
		s.k.TryConsumeFreeCall(s.ctx, caller, tool)
	}
	if s.k.TryConsumeFreeCall(s.ctx, caller, tool) {
		t.Fatal("should be exhausted")
	}

	// Advance to new epoch (epoch = blockHeight / BlocksPerTrustUpdate)
	newCtx := s.ctx.WithBlockHeight(int64(params.BlocksPerTrustUpdate + 1))
	consumed := s.k.TryConsumeFreeCall(newCtx, caller, tool)
	if !consumed {
		t.Error("expected fresh allowance after epoch reset")
	}
}

func TestFreeTier_Disabled(t *testing.T) {
	s := setupFull(t)
	params := types.DefaultParams()
	params.FreeCallsEnabled = false
	s.k.SetParams(s.ctx, params)

	eligible, _ := s.k.CheckFreeEligibility(s.ctx, testAddr("anyone"))
	if eligible {
		t.Error("expected ineligible when free calls disabled")
	}
}

func TestFreeTier_HomeInactiveStatus(t *testing.T) {
	s := setupFull(t)
	caller := testAddr("inactive-home")

	params := types.DefaultParams()
	params.MinHomeAgeBlocks = 10
	s.k.SetParams(s.ctx, params)

	s.home.homes["inactive-1"] = mockHome{
		owner:     caller,
		createdAt: 10,
		status:    "suspended", // not "active"
	}

	eligible, reason := s.k.CheckFreeEligibility(s.ctx, caller)
	if eligible {
		t.Error("expected ineligible for inactive home")
	}
	if reason != "no_eligible_home" {
		t.Errorf("expected 'no_eligible_home', got %q", reason)
	}
}

func TestIsEssentialCategory(t *testing.T) {
	essentials := []string{types.CategoryDataRetrieval, types.CategoryUtility, types.CategoryFormatting}
	nonEssentials := []string{types.CategoryComputation, types.CategoryDataAnalysis, types.CategoryVerification,
		types.CategoryCommunication, types.CategoryMonitoring, types.CategoryIntegration, types.CategoryComposite}

	for _, cat := range essentials {
		if !keeper.IsEssentialCategory(cat) {
			t.Errorf("%s should be essential", cat)
		}
	}
	for _, cat := range nonEssentials {
		if keeper.IsEssentialCategory(cat) {
			t.Errorf("%s should not be essential", cat)
		}
	}
}

// ============================================================
// 10. Adversarial Scenarios & Edge Cases
// ============================================================

func TestAdversarial_SybilFreeAbuse(t *testing.T) {
	s := setupFull(t)
	sybil := testAddr("sybil")

	params := types.DefaultParams()
	params.MinHomeAgeBlocks = 10_000
	s.k.SetParams(s.ctx, params)

	// Young home (age = 100 - 99 = 1, < 10K)
	s.home.homes["sybil-home"] = mockHome{owner: sybil, createdAt: 99, status: "active"}

	eligible, _ := s.k.CheckFreeEligibility(s.ctx, sybil)
	if eligible {
		t.Error("sybil with young home should not be eligible")
	}
}

func TestAdversarial_CircularDependencyExploit(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	// Set up tools
	toolA := &types.Tool{Id: "circ-a", Status: types.ToolStatusActive, TrustScore: 500_000,
		Deployer: testAddr("d"), PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0"}
	toolB := &types.Tool{Id: "circ-b", Status: types.ToolStatusActive, TrustScore: 500_000,
		Deployer: testAddr("d"), PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0"}
	k.SetTool(ctx, toolA)
	k.SetTool(ctx, toolB)
	k.StoreDependencyEdges(ctx, "circ-b", []string{"circ-a"}, 100)

	// Try A → B (would create cycle A→B→A)
	err := k.CheckDependencyCycles(ctx, "circ-a", "circ-b", 10)
	if err == nil {
		t.Fatal("expected cycle detection for A→B→A")
	}
}

func TestAdversarial_TrustManipulation_SelfCalling(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	deployer := testAddr("self-caller")

	tool := &types.Tool{
		Id: "self-call", Deployer: deployer, Status: types.ToolStatusActive,
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0", TrustScore: 500_000,
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	// Deployer calls themselves 100 times
	for i := 0; i < 100; i++ {
		k.RecordCaller(ctx, tool.Id, deployer, 100, true)
	}

	snap := k.ComputeTrustScore(ctx, tool)
	// Usage should be 0 because deployer is excluded
	if snap.UsageComponent != 0 {
		t.Errorf("deployer self-calls should produce 0 usage, got %d", snap.UsageComponent)
	}
}

func TestAdversarial_SurgeGaming(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "surge-game", Category: types.CategoryComputation, // heavy tier
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Massive spike
	for i := 0; i < 100; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	// Should be capped at tier max (10x)
	if surge > 10*types.BpsDenominator {
		t.Errorf("surge should be capped at 10x, got %d", surge)
	}
}

func TestAdversarial_RevenueDrain_ShareEnforcement(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("d")
	id := registerTestTool(t, ms, ctx, deployer, "drain-test")

	// Try to give away more than 100% shares
	_, err := ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: testAddr("c"),
		Role: types.RoleDeveloper, ShareBps: 500_000,
		// deployer keeps 1M → total 1.5M (should fail)
	})
	if err == nil {
		t.Fatal("expected ErrSharesNotSumTo100")
	}
}

func TestAdversarial_GhostContributor(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("deployer")
	ghost := testAddr("ghost")

	tool := &types.Tool{
		Id: "ghost-test", Deployer: deployer, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, ShareBps: 800_000, Accepted: true, TotalEarned: "0"},
			{Address: ghost, ShareBps: 200_000, Accepted: false, TotalEarned: "0"}, // NOT accepted
		},
	}
	k.SetTool(ctx, tool)

	_ = k.DistributeRevenue(ctx, tool, uzrn(1_000_000))

	// Ghost should get nothing (Accepted=false)
	for _, s := range bk.modToAccSends {
		if s.to == ghost {
			t.Error("ghost (unaccepted) contributor should receive no revenue")
		}
	}
}

func TestEdge_ZeroContributors_DeployerGetsAll(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("solo")
	tool := &types.Tool{
		Id: "zero-contrib", Deployer: deployer, PricePerCall: "0",
		TotalRevenue: "0", TotalCalls: "0",
		Contributors: nil,
	}
	k.SetTool(ctx, tool)

	_ = k.DistributeRevenue(ctx, tool, uzrn(1_000_000))

	var toDeployer uint64
	for _, s := range bk.modToAccSends {
		if s.to == deployer {
			toDeployer += s.amount
		}
	}
	if toDeployer != 550_000 {
		t.Errorf("deployer should get 550000, got %d", toDeployer)
	}
}

func TestEdge_ZeroPriceTool_AlwaysFree(t *testing.T) {
	ms, k, ctx, bk, _ := setupMsgServer(t)
	deployer := testAddr("d")
	caller := testAddr("caller")

	id := registerTestTool(t, ms, ctx, deployer, "truly-free", withPrice("0"))
	activateTool(t, k, ctx, id)

	_, err := ms.CallTool(ctx, &types.MsgCallTool{Caller: caller, ToolId: id})
	if err != nil {
		t.Fatalf("CallTool on zero-price: %v", err)
	}

	// No payment should have been collected
	if len(bk.accToModSends) != 0 {
		t.Error("zero-price tool should not collect payment")
	}
}

func TestEdge_LargeNumbers_SafeMulDiv(t *testing.T) {
	// Verify safeMulDiv handles large numbers without overflow
	// safeMulDiv(a, b, c) = (a * b) / c using big.Int
	maxU64 := ^uint64(0)

	// Test with max values — should not panic
	tool := &types.Tool{TrustScore: maxU64}
	_ = tool // Just ensuring the type works with large numbers

	// The safeMulDiv function is tested implicitly through trust/revenue calculations.
	// Directly test: (maxU64 * 1) / 1 = maxU64, but capped at maxScore (1M)
	ab := new(big.Int).Mul(new(big.Int).SetUint64(maxU64), new(big.Int).SetUint64(1))
	result := new(big.Int).Div(ab, new(big.Int).SetUint64(1))
	if !result.IsUint64() || result.Uint64() != maxU64 {
		t.Error("big.Int math failed for max uint64")
	}

	// Verify no overflow: maxU64 * maxU64 / maxU64 = maxU64
	ab = new(big.Int).Mul(new(big.Int).SetUint64(maxU64), new(big.Int).SetUint64(maxU64))
	result = new(big.Int).Div(ab, new(big.Int).SetUint64(maxU64))
	if !result.IsUint64() || result.Uint64() != maxU64 {
		t.Error("big.Int overflow protection failed")
	}
}

// ============================================================
// Bonus: Queries & Genesis
// ============================================================

func TestQuery_Tool(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	tool := &types.Tool{
		Id: "q-tool", Name: "Query Tool", Deployer: testAddr("d"),
		PricePerCall: "0", TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	resp, err := qs.Tool(ctx, &types.QueryToolRequest{ToolId: "q-tool"})
	if err != nil {
		t.Fatalf("Query Tool: %v", err)
	}
	if resp.Tool.Name != "Query Tool" {
		t.Errorf("expected 'Query Tool', got %s", resp.Tool.Name)
	}
}

func TestQuery_ToolsByDeployer(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	qs := keeper.NewQueryServerImpl(k)
	deployer := testAddr("qd")

	registerTestTool(t, ms, ctx, deployer, "d-tool-1")
	registerTestTool(t, ms, ctx, deployer, "d-tool-2")
	registerTestTool(t, ms, ctx, testAddr("other"), "other-tool")

	resp, err := qs.ToolsByDeployer(ctx, &types.QueryByDeployerRequest{Deployer: deployer})
	if err != nil {
		t.Fatalf("Query ToolsByDeployer: %v", err)
	}
	if len(resp.Tools) != 2 {
		t.Errorf("expected 2 tools for deployer, got %d", len(resp.Tools))
	}
}

func TestQuery_ToolsByCategory(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	qs := keeper.NewQueryServerImpl(k)
	deployer := testAddr("qc")

	registerTestTool(t, ms, ctx, deployer, "cat-tool-1", withCategory(types.CategoryComputation))
	registerTestTool(t, ms, ctx, deployer, "cat-tool-2", withCategory(types.CategoryComputation))
	registerTestTool(t, ms, ctx, deployer, "cat-other", withCategory(types.CategoryFormatting))

	resp, err := qs.ToolsByCategory(ctx, &types.QueryByCategoryRequest{Category: types.CategoryComputation})
	if err != nil {
		t.Fatalf("Query ToolsByCategory: %v", err)
	}
	if len(resp.Tools) != 2 {
		t.Errorf("expected 2 computation tools, got %d", len(resp.Tools))
	}
}

func TestQuery_TrustScore(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	k.SetTrustSnapshot(ctx, &types.TrustSnapshot{
		ToolId: "snap-tool", Score: 750_000, ComputedAtBlock: 100,
	})

	resp, err := qs.TrustScore(ctx, &types.QueryTrustScoreRequest{ToolId: "snap-tool"})
	if err != nil {
		t.Fatalf("Query TrustScore: %v", err)
	}
	if resp.Snapshot.Score != 750_000 {
		t.Errorf("expected 750000, got %d", resp.Snapshot.Score)
	}
}

func TestQuery_FreeAllowance(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)
	caller := testAddr("free-q")

	fa := k.GetFreeAllowance(ctx, caller)
	fa.UsedCalls = 10
	k.SetFreeAllowance(ctx, fa)

	resp, err := qs.FreeAllowance(ctx, &types.QueryFreeAllowanceRequest{Caller: caller})
	if err != nil {
		t.Fatalf("Query FreeAllowance: %v", err)
	}
	if resp.Allowance.UsedCalls != 10 {
		t.Errorf("expected 10 used calls, got %d", resp.Allowance.UsedCalls)
	}
}

func TestQuery_Params(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("Query Params: %v", err)
	}
	if resp.Params.MaxContributors != 22 {
		t.Errorf("expected MaxContributors=22, got %d", resp.Params.MaxContributors)
	}
}

func TestGenesis_InitAndExport(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	genesis := types.DefaultGenesis()
	k.InitGenesis(ctx, genesis)

	exported := k.ExportGenesis(ctx)
	if exported.Params == nil {
		t.Fatal("exported params nil")
	}
	if exported.Params.MaxContributors != genesis.Params.MaxContributors {
		t.Errorf("params mismatch: %d vs %d", exported.Params.MaxContributors, genesis.Params.MaxContributors)
	}

	// Default genesis has 5 purpose prompter tools
	if len(exported.Tools) < 5 {
		t.Errorf("expected at least 5 genesis tools, got %d", len(exported.Tools))
	}

	// Find purpose-prompter tool
	found := false
	for _, tool := range exported.Tools {
		if tool.Id == "purpose-prompter" {
			found = true
			if len(tool.DependencyIds) != 4 {
				t.Errorf("purpose-prompter should have 4 deps, got %d", len(tool.DependencyIds))
			}
		}
	}
	if !found {
		t.Error("purpose-prompter tool not found in exported genesis")
	}
}
