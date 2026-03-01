package keeper_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

	"github.com/zerone-chain/zerone/x/bvm/keeper"
	"github.com/zerone-chain/zerone/x/bvm/types"
	"github.com/zerone-chain/zerone/x/bvm/vm"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")

	testAuthority = sdk.AccAddress(bytes.Repeat([]byte{0xAA}, 20)).String()
	testDeployer = sdk.AccAddress(bytes.Repeat([]byte{0x01}, 20)).String()
	testCaller = sdk.AccAddress(bytes.Repeat([]byte{0x02}, 20)).String()
	testUser3 = sdk.AccAddress(bytes.Repeat([]byte{0x03}, 20)).String()
}

const testChainID = "zerone-test-1"

// Addresses are generated in init() after bech32 prefix is set.
var (
	testAuthority string
	testDeployer  string
	testCaller    string
	testUser3     string
)

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
		fromStr := from.String()
		toStr := to.String()
		if m.balances[fromStr] == nil {
			m.balances[fromStr] = make(map[string]int64)
		}
		if m.balances[toStr] == nil {
			m.balances[toStr] = make(map[string]int64)
		}
		m.balances[fromStr][coin.Denom] -= coin.Amount.Int64()
		m.balances[toStr][coin.Denom] += coin.Amount.Int64()
	}
	return nil
}

func (m *mockBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	a := addr.String()
	if m.balances[a] != nil {
		return sdk.NewCoin(denom, sdkmath.NewInt(m.balances[a][denom]))
	}
	return sdk.NewCoin(denom, sdkmath.ZeroInt())
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	for _, coin := range amt {
		from := senderAddr.String()
		if m.balances[from] == nil {
			m.balances[from] = make(map[string]int64)
		}
		if m.balances[from][coin.Denom] < coin.Amount.Int64() {
			return fmt.Errorf("insufficient balance: have %d, need %d", m.balances[from][coin.Denom], coin.Amount.Int64())
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

// ---------- Mock KnowledgeKeeper ----------

type mockKnowledgeKeeper struct {
	facts map[string]uint64 // factId -> confidence
}

func newMockKnowledgeKeeper() *mockKnowledgeKeeper {
	return &mockKnowledgeKeeper{facts: make(map[string]uint64)}
}

func (m *mockKnowledgeKeeper) GetFactConfidence(_ context.Context, factId string) (uint64, bool) {
	conf, found := m.facts[factId]
	return conf, found
}

// ---------- Mock AuthKeeper ----------

type mockAuthKeeper struct {
	dids     map[string]string      // address -> DID
	sessions map[string]mockSession // address -> session capabilities
}

type mockSession struct {
	caps           types.SessionCapabilities
	expiresAtBlock uint64
}

func newMockAuthKeeper() *mockAuthKeeper {
	return &mockAuthKeeper{
		dids:     make(map[string]string),
		sessions: make(map[string]mockSession),
	}
}

func (m *mockAuthKeeper) GetAccountDID(_ context.Context, address string) (string, bool) {
	did, ok := m.dids[address]
	return did, ok
}

func (m *mockAuthKeeper) GetSessionCapabilities(_ context.Context, owner string, blockHeight uint64) (types.SessionCapabilities, bool) {
	sess, ok := m.sessions[owner]
	if !ok || sess.expiresAtBlock <= blockHeight {
		return types.SessionCapabilities{}, false
	}
	return sess.caps, true
}

var _ types.AuthKeeper = (*mockAuthKeeper)(nil)

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
	k := keeper.NewKeeper(cdc, storeService, mockBK, testAuthority)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, ChainID: testChainID}, false, log.NewNopLogger())

	p := types.DefaultParams()
	k.SetParams(ctx, &p)

	return k, ctx, mockBK
}

func setupMsgServer(t *testing.T) (types.MsgServer, keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeper(t)
	return keeper.NewMsgServerImpl(k), k, ctx, bk
}

func setupKeeperWithAuth(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper, *mockAuthKeeper) {
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
	mockAuth := newMockAuthKeeper()

	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(cdc, storeService, mockBK, testAuthority)
	k.SetAuthKeeper(mockAuth)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, ChainID: testChainID}, false, log.NewNopLogger())

	p := types.DefaultParams()
	k.SetParams(ctx, &p)

	// Fund deployer for deploy cost (all auth tests need to deploy contracts)
	mockBK.setBalance(testDeployer, "uzrn", 100000000)

	return k, ctx, mockBK, mockAuth
}

// ---------- Bytecode Helpers ----------

// simpleBytecode returns a bytecode that immediately STOPs.
func simpleBytecode() []byte {
	return []byte{vm.STOP}
}

// returnBytecode returns bytecode: PUSH1 value, PUSH1 0, MSTORE, PUSH1 32, PUSH1 0, RETURN
func returnBytecode(value byte) []byte {
	return []byte{
		vm.PUSH1, value, // push value
		vm.PUSH1, 0,     // push memory offset 0
		vm.MSTORE,       // store to memory
		vm.PUSH1, 32,    // return size 32
		vm.PUSH1, 0,     // return offset 0
		vm.RETURN,        // return
	}
}

// revertBytecode returns bytecode that immediately REVERTs.
func revertBytecode() []byte {
	return []byte{
		vm.PUSH1, 0, // revert size 0
		vm.PUSH1, 0, // revert offset 0
		vm.REVERT,
	}
}

// sstoreBytecode returns bytecode that SSTOREs a value at slot 0: PUSH1 val, PUSH1 0, SSTORE, STOP
func sstoreBytecode(value byte) []byte {
	return []byte{
		vm.PUSH1, value, // value to store
		vm.PUSH1, 0,     // storage slot 0
		vm.SSTORE,       // write to storage
		vm.STOP,
	}
}

// infiniteLoopBytecode returns bytecode for an infinite loop: JUMPDEST, PUSH1 0, JUMP
func infiniteLoopBytecode() []byte {
	return []byte{
		vm.JUMPDEST, // position 0
		vm.PUSH1, 0, // push destination 0
		vm.JUMP,     // jump to 0 (infinite loop)
	}
}

// invalidBytecode returns the INVALID opcode.
func invalidBytecode() []byte {
	return []byte{vm.INVALID}
}

// deployContract is a helper that deploys a contract with the given bytecode.
func deployContract(t *testing.T, srv types.MsgServer, ctx sdk.Context, deployer string, bytecode []byte) string {
	t.Helper()
	resp, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: deployer,
		Bytecode: bytecode,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	return resp.ContractAddress
}

// callContract is a helper that calls a contract.
func callContract(t *testing.T, srv types.MsgServer, ctx sdk.Context, caller, contractAddr string, gas uint64) (*types.MsgCallContractResponse, error) {
	t.Helper()
	return srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          caller,
		ContractAddress: contractAddr,
		InputData:       nil,
		GasLimit:        gas,
	})
}

// =========================================================================
// Section 1: State CRUD Tests
// =========================================================================

func TestDefaultParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	p := k.GetParams(ctx)
	if p.MaxBytecodeSize != 65536 {
		t.Fatalf("expected max_bytecode_size 65536, got %d", p.MaxBytecodeSize)
	}
	if p.MaxGasPerCall != 10000000 {
		t.Fatalf("expected max_gas_per_call 10000000, got %d", p.MaxGasPerCall)
	}
	if p.DeployCost != "5000000" {
		t.Fatalf("expected deploy_cost 5000000, got %s", p.DeployCost)
	}
}

func TestSetGetParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	params := &types.Params{
		MaxBytecodeSize: 32768,
		MaxGasPerCall:   5000000,
		MaxGasPerBlock:  50000000,
		DeployCost:      "1000000",
		MaxStateEntries: 5000,
		CurrentBvmVersion: 2,
		MaxSchedulesPerContract: 50,
	}
	k.SetParams(ctx, params)
	got := k.GetParams(ctx)
	if got.MaxBytecodeSize != 32768 {
		t.Fatalf("expected 32768, got %d", got.MaxBytecodeSize)
	}
	if got.CurrentBvmVersion != 2 {
		t.Fatalf("expected bvm version 2, got %d", got.CurrentBvmVersion)
	}
}

func TestSetGetContract(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	contract := &types.DeployedContract{
		Address:  "zrn1contractabc",
		CodeHash: "deadbeef",
		Creator:  testDeployer,
		DeployedAtBlock: 100,
		BytecodeSize: 10,
		BvmVersion: 1,
	}
	k.SetContract(ctx, contract)

	got, found := k.GetContract(ctx, "zrn1contractabc")
	if !found {
		t.Fatal("expected contract to be found")
	}
	if got.Creator != testDeployer {
		t.Fatalf("expected creator %s, got %s", testDeployer, got.Creator)
	}
	if got.CodeHash != "deadbeef" {
		t.Fatalf("expected code hash deadbeef, got %s", got.CodeHash)
	}
}

func TestGetContractNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	_, found := k.GetContract(ctx, "zrn1nonexistent")
	if found {
		t.Fatal("expected contract not found")
	}
}

func TestGetContractsByCreator(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	for i := 0; i < 3; i++ {
		k.SetContract(ctx, &types.DeployedContract{
			Address:  fmt.Sprintf("zrn1contract%d", i),
			CodeHash: "hash",
			Creator:  testDeployer,
		})
	}
	k.SetContract(ctx, &types.DeployedContract{
		Address:  "zrn1contractother",
		CodeHash: "hash",
		Creator:  testCaller,
	})

	contracts := k.GetContractsByCreator(ctx, testDeployer)
	if len(contracts) != 3 {
		t.Fatalf("expected 3 contracts by deployer, got %d", len(contracts))
	}
	contracts2 := k.GetContractsByCreator(ctx, testCaller)
	if len(contracts2) != 1 {
		t.Fatalf("expected 1 contract by caller, got %d", len(contracts2))
	}
}

func TestSetGetCode(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	code := &types.ContractCode{
		CodeHash: "abc123",
		Bytecode: []byte{0x60, 0x00, 0x00},
		RefCount: 1,
	}
	k.SetCode(ctx, code)

	got, found := k.GetCode(ctx, "abc123")
	if !found {
		t.Fatal("expected code to be found")
	}
	if got.RefCount != 1 {
		t.Fatalf("expected ref count 1, got %d", got.RefCount)
	}
	if !bytes.Equal(got.Bytecode, code.Bytecode) {
		t.Fatal("bytecode mismatch")
	}
}

func TestContractState_SetGet(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	k.SetContractState(ctx, "contract_a", "key1", "value1")
	k.SetContractState(ctx, "contract_a", "key2", "value2")
	k.SetContractState(ctx, "contract_b", "key1", "value_b")

	val, found := k.GetContractState(ctx, "contract_a", "key1")
	if !found || val != "value1" {
		t.Fatalf("expected value1, got %q found=%v", val, found)
	}

	count := k.CountContractState(ctx, "contract_a")
	if count != 2 {
		t.Fatalf("expected 2 state entries for contract_a, got %d", count)
	}

	// Contract B's state is isolated from A
	valB, _ := k.GetContractState(ctx, "contract_b", "key1")
	if valB != "value_b" {
		t.Fatalf("expected value_b, got %q", valB)
	}
	_, found = k.GetContractState(ctx, "contract_b", "key2")
	if found {
		t.Fatal("contract_b should not see contract_a's key2")
	}
}

func TestContractState_Isolation(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetContractState(ctx, "alpha", "secret", "alpha_data")
	k.SetContractState(ctx, "beta", "secret", "beta_data")

	v1, _ := k.GetContractState(ctx, "alpha", "secret")
	v2, _ := k.GetContractState(ctx, "beta", "secret")
	if v1 == v2 {
		t.Fatal("expected different values for same key in different contracts")
	}
	if v1 != "alpha_data" || v2 != "beta_data" {
		t.Fatalf("unexpected values: alpha=%q, beta=%q", v1, v2)
	}
}

func TestSetGetSchedule(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	schedule := &types.ContractSchedule{
		ScheduleId:      "sched-100-0",
		ContractAddress: "zrn1contractabc",
		Caller:          testDeployer,
		ExecuteAtBlock:  200,
		MaxGas:          1000000,
	}
	k.SetSchedule(ctx, schedule)

	got, found := k.GetSchedule(ctx, "sched-100-0")
	if !found {
		t.Fatal("expected schedule to be found")
	}
	if got.ContractAddress != "zrn1contractabc" {
		t.Fatalf("expected contract address zrn1contractabc, got %s", got.ContractAddress)
	}
	if got.ExecuteAtBlock != 200 {
		t.Fatalf("expected execute at block 200, got %d", got.ExecuteAtBlock)
	}
}

func TestGetPendingSchedules(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Schedule at block 150 (should be pending at block 200)
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-a",
		ContractAddress: "c1",
		ExecuteAtBlock:  150,
	})
	// Schedule at block 200 (should be pending at block 200)
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-b",
		ContractAddress: "c1",
		ExecuteAtBlock:  200,
	})
	// Schedule at block 300 (should NOT be pending at block 200)
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-c",
		ContractAddress: "c1",
		ExecuteAtBlock:  300,
	})
	// Cancelled schedule at block 150 (should NOT appear)
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-d",
		ContractAddress: "c1",
		ExecuteAtBlock:  150,
		Cancelled:       true,
	})

	pending := k.GetPendingSchedules(ctx, 200)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending schedules, got %d", len(pending))
	}
}

// =========================================================================
// Section 2: Deploy + Call Happy Path
// =========================================================================

func TestDeployContract_Success(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	resp, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: returnBytecode(42),
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if resp.ContractAddress == "" {
		t.Fatal("expected non-empty contract address")
	}

	// Verify contract is stored
	contract, found := k.GetContract(ctx, resp.ContractAddress)
	if !found {
		t.Fatal("deployed contract not found in state")
	}
	if contract.Creator != testDeployer {
		t.Fatalf("expected creator %s, got %s", testDeployer, contract.Creator)
	}
	if contract.BytecodeSize != uint64(len(returnBytecode(42))) {
		t.Fatalf("expected bytecode size %d, got %d", len(returnBytecode(42)), contract.BytecodeSize)
	}

	// Verify code is stored
	code, codeFound := k.GetCode(ctx, contract.CodeHash)
	if !codeFound {
		t.Fatal("code not found for deployed contract")
	}
	if code.RefCount != 1 {
		t.Fatalf("expected ref count 1, got %d", code.RefCount)
	}

	// Verify deploy cost was charged
	if bk.balances[testDeployer]["uzrn"] != 5000000 {
		t.Fatalf("expected deployer to have 5000000 uzrn after deploy cost, got %d", bk.balances[testDeployer]["uzrn"])
	}
}

func TestDeployContract_DeterministicAddress(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	// Deploy two contracts at the same block
	resp1, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: simpleBytecode(),
	})
	resp2, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: returnBytecode(1),
	})

	if resp1.ContractAddress == resp2.ContractAddress {
		t.Fatal("two contracts at the same block should have different addresses (different nonces)")
	}
}

func TestCallContract_Success(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, returnBytecode(42))

	resp, err := callContract(t, srv, ctx, testCaller, addr, 1000000)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if resp.GasUsed == 0 {
		t.Fatal("expected non-zero gas used")
	}
	if len(resp.ReturnData) != 32 {
		t.Fatalf("expected 32 bytes return data, got %d", len(resp.ReturnData))
	}
	if resp.ReturnData[31] != 42 {
		t.Fatalf("expected return value 42, got %d", resp.ReturnData[31])
	}
}

func TestCallContract_NotFound(t *testing.T) {
	srv, _, ctx, _ := setupMsgServer(t)

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: "zrn1nonexistent",
		GasLimit:        100000,
	})
	if err == nil {
		t.Fatal("expected error for non-existent contract")
	}
}

func TestCallContract_SSTORE_StatePersisted(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(99))

	_, err := callContract(t, srv, ctx, testCaller, addr, 1000000)
	if err != nil {
		t.Fatalf("call with SSTORE failed: %v", err)
	}

	// Verify state was persisted — slot 0 should have value 99
	count := k.CountContractState(ctx, addr)
	if count == 0 {
		t.Fatal("expected at least 1 state entry after SSTORE")
	}
}

// =========================================================================
// Section 3: Bytecode Size Limit
// =========================================================================

func TestDeployContract_BytecodeTooBig(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	oversized := make([]byte, 65537) // 1 byte over default limit
	oversized[0] = vm.STOP

	_, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: oversized,
	})
	if err == nil {
		t.Fatal("expected error for oversized bytecode")
	}
}

func TestDeployContract_ExactMaxSize(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Exactly at the limit should succeed
	exact := make([]byte, 65536)
	exact[0] = vm.STOP

	_, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: exact,
	})
	if err != nil {
		t.Fatalf("deploy at exact max size should succeed: %v", err)
	}
}

// =========================================================================
// Section 4: Code Deduplication
// =========================================================================

func TestCodeDedup_SameBytecode(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	bytecode := returnBytecode(42)
	hash := sha256.Sum256(bytecode)
	codeHash := fmt.Sprintf("%x", hash)

	// Deploy first contract
	srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: bytecode,
	})

	code1, _ := k.GetCode(ctx, codeHash)
	if code1.RefCount != 1 {
		t.Fatalf("expected ref count 1, got %d", code1.RefCount)
	}

	// Deploy second contract with identical bytecode
	srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: bytecode,
	})

	code2, _ := k.GetCode(ctx, codeHash)
	if code2.RefCount != 2 {
		t.Fatalf("expected ref count 2 after dedup, got %d", code2.RefCount)
	}
}

func TestCodeDedup_RefCountLifecycle(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	bytecode := returnBytecode(99)
	hash := sha256.Sum256(bytecode)
	codeHash := fmt.Sprintf("%x", hash)

	// Deploy 3 contracts with same bytecode
	addrs := make([]string, 3)
	for i := 0; i < 3; i++ {
		resp, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
			Deployer: testDeployer,
			Bytecode: bytecode,
		})
		addrs[i] = resp.ContractAddress
	}

	code, _ := k.GetCode(ctx, codeHash)
	if code.RefCount != 3 {
		t.Fatalf("expected ref count 3, got %d", code.RefCount)
	}

	// Delete first contract — ref count should drop to 2
	c1, _ := k.GetContract(ctx, addrs[0])
	k.DeleteContract(ctx, c1)
	code, _ = k.GetCode(ctx, codeHash)
	if code.RefCount != 2 {
		t.Fatalf("expected ref count 2 after delete, got %d", code.RefCount)
	}

	// Delete second — ref count drops to 1
	c2, _ := k.GetContract(ctx, addrs[1])
	k.DeleteContract(ctx, c2)
	code, _ = k.GetCode(ctx, codeHash)
	if code.RefCount != 1 {
		t.Fatalf("expected ref count 1, got %d", code.RefCount)
	}

	// Delete third — code should be garbage collected
	c3, _ := k.GetContract(ctx, addrs[2])
	k.DeleteContract(ctx, c3)
	_, found := k.GetCode(ctx, codeHash)
	if found {
		t.Fatal("expected code to be garbage collected at ref count 0")
	}
}

func TestCodeDedup_DifferentBytecodeNotDeduped(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	bytecodeA := returnBytecode(1)
	bytecodeB := returnBytecode(2)

	srv.DeployContract(ctx, &types.MsgDeployContract{Deployer: testDeployer, Bytecode: bytecodeA})
	srv.DeployContract(ctx, &types.MsgDeployContract{Deployer: testDeployer, Bytecode: bytecodeB})

	hashA := sha256.Sum256(bytecodeA)
	hashB := sha256.Sum256(bytecodeB)

	codeA, _ := k.GetCode(ctx, fmt.Sprintf("%x", hashA))
	codeB, _ := k.GetCode(ctx, fmt.Sprintf("%x", hashB))
	if codeA.RefCount != 1 || codeB.RefCount != 1 {
		t.Fatalf("expected ref count 1 each, got A=%d B=%d", codeA.RefCount, codeB.RefCount)
	}
}

// =========================================================================
// Section 5: Gas Metering
// =========================================================================

func TestGasMetering_BasicCall(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, returnBytecode(42))

	resp, err := callContract(t, srv, ctx, testCaller, addr, 1000000)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	// PUSH1 + PUSH1 + MSTORE + PUSH1 + PUSH1 + RETURN = at least some gas
	if resp.GasUsed < 10 {
		t.Fatalf("expected at least 10 gas used, got %d", resp.GasUsed)
	}
}

func TestGasMetering_OutOfGas_VMLimit(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, infiniteLoopBytecode())

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        100, // very low gas
	})
	if err == nil {
		t.Fatal("expected out-of-gas error for infinite loop with low gas")
	}
}

func TestGasMetering_SSTORE_Expensive(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(1))

	resp, err := callContract(t, srv, ctx, testCaller, addr, 1000000)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	// SSTORE costs 20,000 gas
	if resp.GasUsed < 20000 {
		t.Fatalf("expected >= 20000 gas for SSTORE, got %d", resp.GasUsed)
	}
}

func TestGasMetering_GasLimitClamped(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, returnBytecode(1))

	// Request more gas than max_gas_per_call — should be clamped, not rejected
	resp, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        999999999, // way over max
	})
	if err != nil {
		t.Fatalf("call should succeed with clamped gas: %v", err)
	}
	if resp.GasUsed == 0 {
		t.Fatal("expected non-zero gas")
	}
}

func TestGasMetering_OutOfGas_RevertsState(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Bytecode: SSTORE(0, 42) then infinite loop. With low gas, SSTORE should succeed
	// but the loop will run out of gas, causing the entire call to fail.
	// State changes should NOT be persisted.
	bytecode := []byte{
		vm.PUSH1, 42,   // value
		vm.PUSH1, 0,    // slot
		vm.SSTORE,      // write (20000 gas)
		vm.JUMPDEST,    // position 5
		vm.PUSH1, 5,    // push destination 5
		vm.JUMP,        // infinite loop
	}

	addr := deployContract(t, srv, ctx, testDeployer, bytecode)

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        25000, // enough for SSTORE but not infinite loop
	})
	if err == nil {
		t.Fatal("expected out-of-gas error")
	}

	// State should NOT have been persisted because the execution failed
	count := k.CountContractState(ctx, addr)
	if count != 0 {
		t.Fatalf("expected 0 state entries after failed execution, got %d", count)
	}
}

// =========================================================================
// Section 6: Static Call Enforcement
// =========================================================================

func TestStaticCall_ReadOnly(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Deploy a contract that does SSTORE (state modification)
	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(42))

	// Call with static_call=true — should fail because SSTORE is a state modifier
	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1000000,
		StaticCall:      true,
	})
	if err == nil {
		t.Fatal("expected error for state modification in static call")
	}
}

func TestStaticCall_ReadSucceeds(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Deploy a contract that only reads (RETURN) — should succeed in static call
	addr := deployContract(t, srv, ctx, testDeployer, returnBytecode(42))

	resp, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1000000,
		StaticCall:      true,
	})
	if err != nil {
		t.Fatalf("static call with read-only bytecode should succeed: %v", err)
	}
	if resp.ReturnData[31] != 42 {
		t.Fatalf("expected return value 42 in static call, got %d", resp.ReturnData[31])
	}
}

func TestStaticCall_LOG_Rejected(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Bytecode that emits LOG0 (a state modifier per opcode table)
	logBytecode := []byte{
		vm.PUSH1, 0, // length 0
		vm.PUSH1, 0, // offset 0
		vm.LOG0,     // emit log — state modifier
	}

	addr := deployContract(t, srv, ctx, testDeployer, logBytecode)

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1000000,
		StaticCall:      true,
	})
	if err == nil {
		t.Fatal("expected error for LOG0 in static call")
	}
}

// =========================================================================
// Section 7: Knowledge Bridge Opcodes
// =========================================================================

func TestKnowledgeBridge_KQUERY_FactFound(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	mockKK := newMockKnowledgeKeeper()
	factId := bytes.Repeat([]byte{0xAB}, 32)
	factIdHex := hex.EncodeToString(factId)
	mockKK.facts[factIdHex] = 85

	k.SetKnowledgeKeeper(mockKK)

	srv := keeper.NewMsgServerImpl(k)

	// Deploy contract that uses KQUERY
	// PUSH32 factId, KQUERY, PUSH1 0, MSTORE, PUSH1 0, MSTORE(32), PUSH1 64, PUSH1 0, RETURN
	// KQUERY pops factId, pushes (exists, confidence)
	kqueryBytecode := make([]byte, 0, 50)
	kqueryBytecode = append(kqueryBytecode, vm.PUSH32)
	kqueryBytecode = append(kqueryBytecode, factId...)
	kqueryBytecode = append(kqueryBytecode,
		vm.KQUERY,
		// stack: [exists, confidence] — confidence on top
		vm.PUSH1, 32,   // offset 32
		vm.MSTORE,      // store confidence at offset 32
		vm.PUSH1, 0,    // offset 0
		vm.MSTORE,      // store exists at offset 0
		vm.PUSH1, 64,   // return size
		vm.PUSH1, 0,    // return offset
		vm.RETURN,
	)

	resp, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: kqueryBytecode,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	callResp, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: resp.ContractAddress,
		GasLimit:        1000000,
	})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}

	if len(callResp.ReturnData) < 64 {
		t.Fatalf("expected 64 bytes return data, got %d", len(callResp.ReturnData))
	}

	// exists is stored at offset 0 (32 bytes, big-endian) — should be 1
	if callResp.ReturnData[31] != 1 {
		t.Fatalf("expected exists=1, got %d", callResp.ReturnData[31])
	}
	// confidence is stored at offset 32 (32 bytes, big-endian) — should be 85
	if callResp.ReturnData[63] != 85 {
		t.Fatalf("expected confidence=85, got %d", callResp.ReturnData[63])
	}
}

func TestKnowledgeBridge_KQUERY_FactNotFound(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	mockKK := newMockKnowledgeKeeper()
	// No facts registered
	k.SetKnowledgeKeeper(mockKK)

	srv := keeper.NewMsgServerImpl(k)

	factId := bytes.Repeat([]byte{0xCC}, 32)
	kqueryBytecode := make([]byte, 0, 50)
	kqueryBytecode = append(kqueryBytecode, vm.PUSH32)
	kqueryBytecode = append(kqueryBytecode, factId...)
	kqueryBytecode = append(kqueryBytecode,
		vm.KQUERY,
		vm.PUSH1, 32, vm.MSTORE,
		vm.PUSH1, 0, vm.MSTORE,
		vm.PUSH1, 64, vm.PUSH1, 0, vm.RETURN,
	)

	resp, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: kqueryBytecode,
	})

	callResp, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: resp.ContractAddress,
		GasLimit:        1000000,
	})
	if err != nil {
		t.Fatalf("call should succeed even if fact not found: %v", err)
	}

	// exists=0
	if callResp.ReturnData[31] != 0 {
		t.Fatalf("expected exists=0, got %d", callResp.ReturnData[31])
	}
	// confidence=0
	if callResp.ReturnData[63] != 0 {
		t.Fatalf("expected confidence=0, got %d", callResp.ReturnData[63])
	}
}

func TestKnowledgeBridge_NoKeeper(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Without knowledge keeper set, KQUERY should return (0, 0) gracefully
	factId := bytes.Repeat([]byte{0xDD}, 32)
	kqueryBytecode := make([]byte, 0, 50)
	kqueryBytecode = append(kqueryBytecode, vm.PUSH32)
	kqueryBytecode = append(kqueryBytecode, factId...)
	kqueryBytecode = append(kqueryBytecode,
		vm.KQUERY,
		vm.PUSH1, 32, vm.MSTORE,
		vm.PUSH1, 0, vm.MSTORE,
		vm.PUSH1, 64, vm.PUSH1, 0, vm.RETURN,
	)

	resp, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: kqueryBytecode,
	})

	callResp, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: resp.ContractAddress,
		GasLimit:        1000000,
	})
	if err != nil {
		t.Fatalf("KQUERY without knowledge keeper should succeed gracefully: %v", err)
	}
	if callResp.ReturnData[31] != 0 || callResp.ReturnData[63] != 0 {
		t.Fatal("expected (0,0) without knowledge keeper")
	}
}

// =========================================================================
// Section 8: Scheduled Execution in BeginBlocker
// =========================================================================

func TestScheduleContract_Success(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	resp, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  200,
		MaxGas:          1000000,
	})
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if resp.ScheduleId == "" {
		t.Fatal("expected non-empty schedule ID")
	}

	schedule, found := k.GetSchedule(ctx, resp.ScheduleId)
	if !found {
		t.Fatal("schedule not found in state")
	}
	if schedule.ExecuteAtBlock != 200 {
		t.Fatalf("expected execute at block 200, got %d", schedule.ExecuteAtBlock)
	}
}

func TestScheduleContract_InPast(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  50, // ctx is at height 100
		MaxGas:          1000000,
	})
	if err == nil {
		t.Fatal("expected error for schedule in the past")
	}
}

func TestScheduleContract_NotCreator(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testCaller, // not the creator
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  200,
		MaxGas:          1000000,
	})
	if err == nil {
		t.Fatal("expected error: only creator can schedule")
	}
}

func TestScheduleContract_LimitReached(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Set limit to 2
	params := k.GetParams(ctx)
	params.MaxSchedulesPerContract = 2
	k.SetParams(ctx, params)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	for i := 0; i < 2; i++ {
		_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
			Caller:          testDeployer,
			ContractAddress: addr,
			Method:          "tick",
			ExecuteAtBlock:  uint64(200 + i),
			MaxGas:          1000000,
		})
		if err != nil {
			t.Fatalf("schedule %d should succeed: %v", i, err)
		}
	}

	_, err := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  300,
		MaxGas:          1000000,
	})
	if err == nil {
		t.Fatal("expected error: schedule limit reached")
	}
}

func TestCancelSchedule_Success(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	schedResp, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  200,
		MaxGas:          1000000,
	})

	_, err := srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testDeployer,
		ScheduleId: schedResp.ScheduleId,
	})
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	schedule, _ := k.GetSchedule(ctx, schedResp.ScheduleId)
	if !schedule.Cancelled {
		t.Fatal("expected schedule to be cancelled")
	}
}

func TestCancelSchedule_NotOwner(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	schedResp, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  200,
		MaxGas:          1000000,
	})

	_, err := srv.CancelSchedule(ctx, &types.MsgCancelSchedule{
		Caller:     testCaller, // not the owner
		ScheduleId: schedResp.ScheduleId,
	})
	if err == nil {
		t.Fatal("expected error: not schedule owner")
	}
}

func TestBeginBlocker_ExecutesSchedules(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	// Deploy contract that SSTOREs value 77 at slot 0
	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(77))

	// Schedule for block 150
	srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  150,
		MaxGas:          1000000,
	})

	// Advance to block 150
	ctx150 := ctx.WithBlockHeader(cmtproto.Header{Height: 150, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx150)

	// The contract should have written to state
	count := k.CountContractState(ctx150, addr)
	if count == 0 {
		t.Fatal("expected state to be written by scheduled execution")
	}
}

func TestBeginBlocker_ScheduleNotYetDue(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(77))

	srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  300,
		MaxGas:          1000000,
	})

	// Execute at block 150 — schedule at 300 should NOT run
	ctx150 := ctx.WithBlockHeader(cmtproto.Header{Height: 150, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx150)

	count := k.CountContractState(ctx150, addr)
	if count != 0 {
		t.Fatal("expected no state changes — schedule not yet due")
	}
}

func TestBeginBlocker_OverdueScheduleExecutes(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(55))

	// Schedule for block 120 (already overdue at block 150)
	srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  120,
		MaxGas:          1000000,
	})

	// Execute at block 150 — should pick up overdue schedule
	ctx150 := ctx.WithBlockHeader(cmtproto.Header{Height: 150, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx150)

	count := k.CountContractState(ctx150, addr)
	if count == 0 {
		t.Fatal("expected overdue schedule to execute")
	}
}

func TestBeginBlocker_PanicInScheduleDoesntBlockOthers(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	// First contract: INVALID opcode (will panic)
	addrBad := deployContract(t, srv, ctx, testDeployer, invalidBytecode())

	// Second contract: writes state
	addrGood := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(33))

	// Schedule both for block 150
	srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addrBad,
		Method:          "bad",
		ExecuteAtBlock:  150,
		MaxGas:          100000,
	})
	srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addrGood,
		Method:          "good",
		ExecuteAtBlock:  150,
		MaxGas:          100000,
	})

	ctx150 := ctx.WithBlockHeader(cmtproto.Header{Height: 150, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx150) // should not panic

	// Good contract should still execute
	count := k.CountContractState(ctx150, addrGood)
	if count == 0 {
		t.Fatal("expected good contract to write state despite bad contract panic")
	}
}

func TestBeginBlocker_GasBudgetEnforced(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	// Deploy a contract that uses >1M gas (infinite loop with high gas limit)
	addr := deployContract(t, srv, ctx, testDeployer, infiniteLoopBytecode())

	// Schedule 10 infinite-loop executions at block 150, each with 1M gas
	for i := 0; i < 10; i++ {
		srv.ScheduleContract(ctx, &types.MsgScheduleContract{
			Caller:          testDeployer,
			ContractAddress: addr,
			Method:          fmt.Sprintf("tick%d", i),
			ExecuteAtBlock:  150,
			MaxGas:          1000000,
		})
	}

	ctx150 := ctx.WithBlockHeader(cmtproto.Header{Height: 150, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx150) // should not run forever

	// MaxScheduledGasPerBlock is 5M, each schedule uses 1M of gas -> at most 5 should execute
	executedCount := 0
	k.IterateSchedules(ctx150, func(s *types.ContractSchedule) bool {
		if s.Executed {
			executedCount++
		}
		return false
	})
	if executedCount > 6 {
		t.Fatalf("expected at most ~5-6 schedules to execute under gas budget, got %d", executedCount)
	}
}

func TestBeginBlocker_MarksScheduleExecuted(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	schedResp, _ := srv.ScheduleContract(ctx, &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: addr,
		Method:          "tick",
		ExecuteAtBlock:  150,
		MaxGas:          100000,
	})

	ctx150 := ctx.WithBlockHeader(cmtproto.Header{Height: 150, ChainID: testChainID})
	k.ExecutePendingSchedules(ctx150)

	schedule, _ := k.GetSchedule(ctx150, schedResp.ScheduleId)
	if !schedule.Executed {
		t.Fatal("expected schedule to be marked as executed")
	}
}

// =========================================================================
// Section 9: Contract State Isolation
// =========================================================================

func TestContractState_CrossContractIsolation(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	// Deploy two contracts with SSTORE — they write the same slot (0) with different values
	addrA := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(11))
	addrB := deployContract(t, srv, ctx, testDeployer, sstoreBytecode(22))

	callContract(t, srv, ctx, testCaller, addrA, 1000000)
	callContract(t, srv, ctx, testCaller, addrB, 1000000)

	// Each contract should have its own state namespace
	countA := k.CountContractState(ctx, addrA)
	countB := k.CountContractState(ctx, addrB)

	if countA == 0 || countB == 0 {
		t.Fatalf("expected both contracts to have state: A=%d B=%d", countA, countB)
	}

	// The values should be different since they're in different namespaces
	var stateA, stateB string
	k.IterateContractState(ctx, addrA, func(key, value string) bool {
		stateA = value
		return true
	})
	k.IterateContractState(ctx, addrB, func(key, value string) bool {
		stateB = value
		return true
	})
	if stateA == stateB {
		t.Fatal("expected different state values for different contracts writing different values")
	}
}

// =========================================================================
// Section 10: Panic Recovery
// =========================================================================

func TestPanicRecovery_InvalidOpcode(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, invalidBytecode())

	_, err := callContract(t, srv, ctx, testCaller, addr, 1000000)
	if err == nil {
		t.Fatal("expected error for INVALID opcode")
	}
}

func TestPanicRecovery_DoesNotCrashModule(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	// Deploy INVALID opcode contract
	addrBad := deployContract(t, srv, ctx, testDeployer, invalidBytecode())
	// Deploy valid contract
	addrGood := deployContract(t, srv, ctx, testDeployer, returnBytecode(99))

	// Call bad contract — should fail but not crash
	callContract(t, srv, ctx, testCaller, addrBad, 1000000)

	// Call good contract — should still work
	resp, err := callContract(t, srv, ctx, testCaller, addrGood, 1000000)
	if err != nil {
		t.Fatalf("good contract should work after bad contract: %v", err)
	}
	if resp.ReturnData[31] != 99 {
		t.Fatalf("expected 99, got %d", resp.ReturnData[31])
	}
}

func TestPanicRecovery_RevertRefundsValue(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)
	bk.setBalance(testCaller, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, revertBytecode())

	balanceBefore := bk.balances[testCaller]["uzrn"]

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1000000,
		Value:           "1000000",
	})
	if err == nil {
		t.Fatal("expected error from REVERT")
	}

	balanceAfter := bk.balances[testCaller]["uzrn"]
	if balanceAfter != balanceBefore {
		t.Fatalf("expected value to be refunded on revert: before=%d after=%d", balanceBefore, balanceAfter)
	}
}

// =========================================================================
// Section 11: Governance Handlers
// =========================================================================

func TestUpdateContractState_Success(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.UpdateContractState(ctx, &types.MsgUpdateContractState{
		Authority:       testAuthority,
		ContractAddress: addr,
		Key:             "emergency_key",
		Value:           "emergency_value",
	})
	if err != nil {
		t.Fatalf("governance state update failed: %v", err)
	}

	val, found := k.GetContractState(ctx, addr, "emergency_key")
	if !found || val != "emergency_value" {
		t.Fatalf("expected emergency_value, got %q found=%v", val, found)
	}
}

func TestUpdateContractState_NotAuthority(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.UpdateContractState(ctx, &types.MsgUpdateContractState{
		Authority:       testCaller, // not authority
		ContractAddress: addr,
		Key:             "key",
		Value:           "val",
	})
	if err == nil {
		t.Fatal("expected error for unauthorized state update")
	}
}

func TestUpdateParams_Success(t *testing.T) {
	srv, k, ctx, _ := setupMsgServer(t)

	newParams := types.DefaultParams()
	newParams.MaxBytecodeSize = 131072
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAuthority,
		Params:    &newParams,
	})
	if err != nil {
		t.Fatalf("update params failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MaxBytecodeSize != 131072 {
		t.Fatalf("expected 131072, got %d", got.MaxBytecodeSize)
	}
}

func TestUpdateParams_Unauthorized(t *testing.T) {
	srv, _, ctx, _ := setupMsgServer(t)

	p := types.DefaultParams()
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testCaller,
		Params:    &p,
	})
	if err == nil {
		t.Fatal("expected error for unauthorized params update")
	}
}

// =========================================================================
// Section 12: Query Server
// =========================================================================

func TestQuery_Contract(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	srv := keeper.NewMsgServerImpl(k)
	qsrv := keeper.NewQueryServerImpl(k)

	resp, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: simpleBytecode(),
	})

	qResp, err := qsrv.Contract(ctx, &types.QueryContractRequest{
		Address: resp.ContractAddress,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if qResp.Contract.Creator != testDeployer {
		t.Fatalf("expected creator %s, got %s", testDeployer, qResp.Contract.Creator)
	}
}

func TestQuery_ContractNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qsrv := keeper.NewQueryServerImpl(k)

	_, err := qsrv.Contract(ctx, &types.QueryContractRequest{
		Address: "zrn1nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent contract query")
	}
}

func TestQuery_ContractsByCreator(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)

	srv := keeper.NewMsgServerImpl(k)
	qsrv := keeper.NewQueryServerImpl(k)

	for i := 0; i < 3; i++ {
		srv.DeployContract(ctx, &types.MsgDeployContract{
			Deployer: testDeployer,
			Bytecode: returnBytecode(byte(i)),
		})
	}

	qResp, err := qsrv.ContractsByCreator(ctx, &types.QueryByCreatorRequest{
		Creator: testDeployer,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(qResp.Contracts) != 3 {
		t.Fatalf("expected 3 contracts, got %d", len(qResp.Contracts))
	}
}

func TestQuery_ContractState(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qsrv := keeper.NewQueryServerImpl(k)

	k.SetContractState(ctx, "addr1", "mykey", "myvalue")
	qResp, err := qsrv.ContractState(ctx, &types.QueryContractStateRequest{
		Address: "addr1",
		Key:     "mykey",
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if qResp.Value != "myvalue" {
		t.Fatalf("expected myvalue, got %q", qResp.Value)
	}
}

func TestQuery_Params(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qsrv := keeper.NewQueryServerImpl(k)

	qResp, err := qsrv.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if qResp.Params.MaxBytecodeSize != 65536 {
		t.Fatalf("expected 65536, got %d", qResp.Params.MaxBytecodeSize)
	}
}

// =========================================================================
// Section 13: Genesis Round-Trip
// =========================================================================

func TestGenesis_RoundTrip(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Populate state
	k.SetContract(ctx, &types.DeployedContract{
		Address:  "zrn1c1",
		CodeHash: "hash1",
		Creator:  testDeployer,
		BvmVersion: 1,
	})
	k.SetCode(ctx, &types.ContractCode{
		CodeHash: "hash1",
		Bytecode: simpleBytecode(),
		RefCount: 1,
	})
	k.SetContractState(ctx, "zrn1c1", "key1", "val1")
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "sched-1",
		ContractAddress: "zrn1c1",
		ExecuteAtBlock:  500,
	})

	// Export
	gs := k.ExportGenesis(ctx)
	if len(gs.Contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(gs.Contracts))
	}
	if len(gs.Codes) != 1 {
		t.Fatalf("expected 1 code, got %d", len(gs.Codes))
	}
	if len(gs.State) != 1 {
		t.Fatalf("expected 1 state entry, got %d", len(gs.State))
	}
	if len(gs.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(gs.Schedules))
	}

	// Import into fresh keeper
	k2, ctx2, _ := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	c, found := k2.GetContract(ctx2, "zrn1c1")
	if !found {
		t.Fatal("contract not found after genesis import")
	}
	if c.Creator != testDeployer {
		t.Fatalf("expected creator %s, got %s", testDeployer, c.Creator)
	}

	val, _ := k2.GetContractState(ctx2, "zrn1c1", "key1")
	if val != "val1" {
		t.Fatalf("expected val1 after genesis import, got %q", val)
	}
}

func TestGenesis_EmptyState(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	gs := k.ExportGenesis(ctx)
	if len(gs.Contracts) != 0 {
		t.Fatalf("expected 0 contracts, got %d", len(gs.Contracts))
	}
	if gs.Params == nil {
		t.Fatal("expected params to be non-nil")
	}
}

// =========================================================================
// Section 14: ValidateBasic
// =========================================================================

func TestValidateBasic_DeployContract(t *testing.T) {
	// Empty bytecode
	msg := &types.MsgDeployContract{Deployer: testDeployer, Bytecode: nil}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for empty bytecode")
	}

	// Invalid deployer address
	msg = &types.MsgDeployContract{Deployer: "invalid", Bytecode: simpleBytecode()}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for invalid deployer")
	}

	// Oversized bytecode (stateless limit)
	msg = &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: make([]byte, types.MaxDeployBytecodeSize+1),
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for oversized bytecode (stateless)")
	}
}

func TestValidateBasic_CallContract(t *testing.T) {
	// Zero gas limit
	msg := &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: "addr",
		GasLimit:        0,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for zero gas limit")
	}

	// Gas exceeds static limit
	msg = &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: "addr",
		GasLimit:        types.MaxContractGasLimit + 1,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for gas exceeding static limit")
	}

	// Empty contract address
	msg = &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: "",
		GasLimit:        100000,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for empty contract address")
	}
}

func TestValidateBasic_ScheduleContract(t *testing.T) {
	msg := &types.MsgScheduleContract{
		Caller:          "invalid",
		ContractAddress: "addr",
		ExecuteAtBlock:  200,
		MaxGas:          100000,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for invalid caller")
	}

	msg = &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: "addr",
		ExecuteAtBlock:  0,
		MaxGas:          100000,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for zero execute_at_block")
	}

	msg = &types.MsgScheduleContract{
		Caller:          testDeployer,
		ContractAddress: "addr",
		ExecuteAtBlock:  200,
		MaxGas:          0,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for zero max_gas")
	}
}

func TestValidateBasic_CancelSchedule(t *testing.T) {
	msg := &types.MsgCancelSchedule{
		Caller:     testDeployer,
		ScheduleId: "",
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for empty schedule_id")
	}
}

func TestValidateBasic_UpdateParams(t *testing.T) {
	msg := &types.MsgUpdateParams{
		Authority: "",
		Params:    nil,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for empty authority")
	}

	p := types.Params{MaxBytecodeSize: 0, MaxStateEntries: 1, MaxGasPerCall: 1}
	msg = &types.MsgUpdateParams{
		Authority: testAuthority,
		Params:    &p,
	}
	if msg.ValidateBasic() == nil {
		t.Fatal("expected error for invalid params (zero max_bytecode_size)")
	}
}

// =========================================================================
// Section 15: Deploy Cost Handling
// =========================================================================

func TestDeployContract_InsufficientFunds(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 1000) // not enough for 5 ZRN deploy cost

	_, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: simpleBytecode(),
	})
	if err == nil {
		t.Fatal("expected error for insufficient deploy funds")
	}
}

func TestDeployContract_ZeroDeployCost(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)

	// Set deploy cost to 0
	params := k.GetParams(ctx)
	params.DeployCost = "0"
	k.SetParams(ctx, params)

	// Should succeed without any balance
	bk.setBalance(testDeployer, "uzrn", 0)
	_, err := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: simpleBytecode(),
	})
	if err != nil {
		t.Fatalf("deploy with zero cost should succeed: %v", err)
	}
}

// =========================================================================
// Section 16: Payable Contract Value Transfer
// =========================================================================

func TestCallContract_ValueTransfer(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)
	bk.setBalance(testCaller, "uzrn", 5000000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1000000,
		Value:           "1000000", // 1 ZRN
	})
	if err != nil {
		t.Fatalf("payable call failed: %v", err)
	}

	// Caller should have 4 ZRN left (5 - 1)
	if bk.balances[testCaller]["uzrn"] != 4000000 {
		t.Fatalf("expected caller to have 4000000, got %d", bk.balances[testCaller]["uzrn"])
	}
}

func TestCallContract_ValueTransfer_InsufficientFunds(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)
	bk.setBalance(testCaller, "uzrn", 100) // not enough

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        1000000,
		Value:           "1000000",
	})
	if err == nil {
		t.Fatal("expected error for insufficient funds on payable call")
	}
}

// =========================================================================
// Section 17: Edge Cases
// =========================================================================

func TestIterateContracts(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	for i := 0; i < 5; i++ {
		k.SetContract(ctx, &types.DeployedContract{
			Address:  fmt.Sprintf("addr_%d", i),
			CodeHash: "h",
			Creator:  testDeployer,
		})
	}

	count := 0
	k.IterateContracts(ctx, func(_ *types.DeployedContract) bool {
		count++
		return false
	})
	if count != 5 {
		t.Fatalf("expected 5 contracts, iterated %d", count)
	}
}

func TestIterateCode(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	for i := 0; i < 3; i++ {
		k.SetCode(ctx, &types.ContractCode{
			CodeHash: fmt.Sprintf("hash_%d", i),
			Bytecode: []byte{byte(i)},
			RefCount: 1,
		})
	}

	count := 0
	k.IterateCode(ctx, func(_ *types.ContractCode) bool {
		count++
		return false
	})
	if count != 3 {
		t.Fatalf("expected 3 codes, iterated %d", count)
	}
}

func TestNextContractNonce_Increments(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	n0 := k.GetNextContractNonce(ctx)
	n1 := k.GetNextContractNonce(ctx)
	n2 := k.GetNextContractNonce(ctx)

	if n0 != 0 || n1 != 1 || n2 != 2 {
		t.Fatalf("expected nonces 0,1,2, got %d,%d,%d", n0, n1, n2)
	}
}

func TestNextScheduleId_Increments(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	s0 := k.GetNextScheduleId(ctx)
	s1 := k.GetNextScheduleId(ctx)

	if s0 != 0 || s1 != 1 {
		t.Fatalf("expected schedule IDs 0,1, got %d,%d", s0, s1)
	}
}

func TestCountContractSchedules(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "s1",
		ContractAddress: "c1",
		ExecuteAtBlock:  200,
	})
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "s2",
		ContractAddress: "c1",
		ExecuteAtBlock:  300,
	})
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "s3",
		ContractAddress: "c1",
		ExecuteAtBlock:  400,
		Executed:        true, // already executed — should not count
	})
	k.SetSchedule(ctx, &types.ContractSchedule{
		ScheduleId:      "s4",
		ContractAddress: "c2",
		ExecuteAtBlock:  200,
	})

	count := k.CountContractSchedules(ctx, "c1")
	if count != 2 {
		t.Fatalf("expected 2 active schedules for c1, got %d", count)
	}
}

// =========================================================================
// Section 18: Ported from Prototype — Genesis + Lifecycle Edge Cases
// =========================================================================

func TestGenesisRoundtrip_IncludesState(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)

	srv := keeper.NewMsgServerImpl(k)
	deployResp, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: simpleBytecode(),
	})

	k.SetContractState(ctx, deployResp.ContractAddress, "counter", "42")
	k.SetContractState(ctx, deployResp.ContractAddress, "owner", testDeployer)

	gs := k.ExportGenesis(ctx)
	if len(gs.State) != 2 {
		t.Fatalf("expected 2 state entries in genesis, got %d", len(gs.State))
	}

	k2, ctx2, _ := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	val, found := k2.GetContractState(ctx2, deployResp.ContractAddress, "counter")
	if !found || val != "42" {
		t.Fatalf("expected counter=42, got %s (found=%v)", val, found)
	}
	val, found = k2.GetContractState(ctx2, deployResp.ContractAddress, "owner")
	if !found || val != testDeployer {
		t.Fatalf("expected owner=%s, got %s (found=%v)", testDeployer, val, found)
	}
}

func TestDeleteContract_DecrRefCount(t *testing.T) {
	srv, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 100000000)
	bk.setBalance(testUser3, "uzrn", 100000000)

	bytecode := returnBytecode(42)

	resp1, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testDeployer,
		Bytecode: bytecode,
	})
	resp2, _ := srv.DeployContract(ctx, &types.MsgDeployContract{
		Deployer: testUser3,
		Bytecode: bytecode,
	})

	contract1, _ := k.GetContract(ctx, resp1.ContractAddress)
	code, _ := k.GetCode(ctx, contract1.CodeHash)
	if code.RefCount != 2 {
		t.Fatalf("expected refcount 2, got %d", code.RefCount)
	}

	// Delete first — refcount should drop to 1
	k.DeleteContract(ctx, contract1)
	code, found := k.GetCode(ctx, contract1.CodeHash)
	if !found {
		t.Fatal("code should still exist with refcount 1")
	}
	if code.RefCount != 1 {
		t.Fatalf("expected refcount 1 after delete, got %d", code.RefCount)
	}

	// Delete second — code should be garbage collected
	contract2, _ := k.GetContract(ctx, resp2.ContractAddress)
	k.DeleteContract(ctx, contract2)
	_, found = k.GetCode(ctx, contract1.CodeHash)
	if found {
		t.Fatal("code should be garbage collected when refcount reaches 0")
	}
}

func TestCallContract_ZeroValueNoTransfer(t *testing.T) {
	srv, _, ctx, bk := setupMsgServer(t)
	bk.setBalance(testDeployer, "uzrn", 10000000)
	bk.setBalance(testCaller, "uzrn", 50000)

	addr := deployContract(t, srv, ctx, testDeployer, simpleBytecode())

	_, err := srv.CallContract(ctx, &types.MsgCallContract{
		Caller:          testCaller,
		ContractAddress: addr,
		GasLimit:        100000,
		Value:           "0",
	})
	if err != nil {
		t.Fatalf("zero value call: %v", err)
	}

	if bk.balances[testCaller]["uzrn"] != 50000 {
		t.Fatalf("expected no change in balance, got %d", bk.balances[testCaller]["uzrn"])
	}
}
