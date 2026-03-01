package keeper_test

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/bvm/keeper"
	"github.com/zerone-chain/zerone/x/bvm/types"
	"github.com/zerone-chain/zerone/x/bvm/vm"
)

// ---------- Mock HomeKeeper ----------

type mockHomeKeeper struct {
	homes    map[string]types.HomeInfo // homeID → HomeInfo
	ownerIdx map[string][]string      // owner → []homeID
}

func newMockHomeKeeper() *mockHomeKeeper {
	return &mockHomeKeeper{
		homes:    make(map[string]types.HomeInfo),
		ownerIdx: make(map[string][]string),
	}
}

func (m *mockHomeKeeper) addHome(info types.HomeInfo) {
	m.homes[info.HomeID] = info
	m.ownerIdx[info.OwnerAddress] = append(m.ownerIdx[info.OwnerAddress], info.HomeID)
}

func (m *mockHomeKeeper) GetHome(_ context.Context, homeID string) (types.HomeInfo, bool) {
	h, ok := m.homes[homeID]
	return h, ok
}

func (m *mockHomeKeeper) GetHomesByOwner(_ context.Context, owner string) []string {
	return m.ownerIdx[owner]
}

func (m *mockHomeKeeper) GetHomeStatus(_ context.Context, homeID string) string {
	h, ok := m.homes[homeID]
	if !ok {
		return ""
	}
	return h.Status
}

func (m *mockHomeKeeper) GetMemoryCID(_ context.Context, homeID string) string {
	h, ok := m.homes[homeID]
	if !ok {
		return ""
	}
	return h.MemoryCID
}

func (m *mockHomeKeeper) GetPartnershipID(_ context.Context, homeID string) string {
	h, ok := m.homes[homeID]
	if !ok {
		return ""
	}
	return h.PartnershipID
}

func (m *mockHomeKeeper) GetComfortScore(_ context.Context, homeID string) uint32 {
	h, ok := m.homes[homeID]
	if !ok {
		return 0
	}
	return h.ComfortScore
}

var _ types.HomeKeeper = (*mockHomeKeeper)(nil)

// ---------- Setup Helpers ----------

func setupKeeperWithHome(t *testing.T) (keeper.Keeper, *mockHomeKeeper) {
	t.Helper()
	k, _, _ := setupKeeper(t)
	mockHK := newMockHomeKeeper()
	k.SetHomeKeeper(mockHK)
	return k, mockHK
}

// ---------- HQuery Tests ----------

func TestHQuery_AgentWithHome(t *testing.T) {
	_, mockHK := setupKeeperWithHome(t)

	mockHK.addHome(types.HomeInfo{
		HomeID:       "home-1",
		OwnerAddress: testDeployer,
		Name:         "Test Home",
		Status:       "active",
		MemoryCID:    "QmTest123",
		ComfortScore: 85,
	})

	host := &testZeroneHost{hk: mockHK}

	callerBytes := accAddrToBytes(testDeployer)
	hasHome, homeId, status := host.HQuery(callerBytes)
	if !hasHome {
		t.Fatal("expected agent to have a home")
	}
	if string(homeId) != "home-1" {
		t.Fatalf("expected homeId 'home-1', got '%s'", string(homeId))
	}
	if string(status) != "active" {
		t.Fatalf("expected status 'active', got '%s'", string(status))
	}
}

func TestHQuery_AgentWithoutHome(t *testing.T) {
	_, _ = setupKeeperWithHome(t)

	host := &testZeroneHost{hk: newMockHomeKeeper()}

	callerBytes := accAddrToBytes(testCaller)
	hasHome, homeId, status := host.HQuery(callerBytes)
	if hasHome {
		t.Fatal("expected no home for agent")
	}
	if homeId != nil {
		t.Fatalf("expected nil homeId, got %v", homeId)
	}
	if status != nil {
		t.Fatalf("expected nil status, got %v", status)
	}
}

func TestHQuery_MultipleHomes(t *testing.T) {
	_, mockHK := setupKeeperWithHome(t)

	for i := 1; i <= 3; i++ {
		mockHK.addHome(types.HomeInfo{
			HomeID:       "home-" + hbUitoa(uint64(i)),
			OwnerAddress: testDeployer,
			Status:       "active",
		})
	}

	host := &testZeroneHost{hk: mockHK}
	callerBytes := accAddrToBytes(testDeployer)
	hasHome, homeId, _ := host.HQuery(callerBytes)
	if !hasHome {
		t.Fatal("expected agent to have a home")
	}
	if string(homeId) != "home-1" {
		t.Fatalf("expected primary home 'home-1', got '%s'", string(homeId))
	}
}

// ---------- HMemory Tests ----------

func TestHMemory_ValidHome(t *testing.T) {
	_, mockHK := setupKeeperWithHome(t)

	mockHK.addHome(types.HomeInfo{
		HomeID:    "home-1",
		MemoryCID: "QmTestMemory456",
	})

	host := &testZeroneHost{hk: mockHK}
	cid := host.HMemory(hbPadHomeID("home-1"))
	if string(cid) != "QmTestMemory456" {
		t.Fatalf("expected CID 'QmTestMemory456', got '%s'", string(cid))
	}
}

func TestHMemory_NoMemory(t *testing.T) {
	_, mockHK := setupKeeperWithHome(t)

	mockHK.addHome(types.HomeInfo{
		HomeID:    "home-1",
		MemoryCID: "",
	})

	host := &testZeroneHost{hk: mockHK}
	cid := host.HMemory(hbPadHomeID("home-1"))
	if len(cid) != 0 {
		t.Fatalf("expected empty CID, got '%s'", string(cid))
	}
}

func TestHMemory_UnknownHome(t *testing.T) {
	_, _ = setupKeeperWithHome(t)

	host := &testZeroneHost{hk: newMockHomeKeeper()}
	cid := host.HMemory(hbPadHomeID("home-999"))
	if len(cid) != 0 {
		t.Fatalf("expected empty CID for unknown home, got '%s'", string(cid))
	}
}

// ---------- HPartner Tests ----------

func TestHPartner_LinkedHome(t *testing.T) {
	_, mockHK := setupKeeperWithHome(t)

	mockHK.addHome(types.HomeInfo{
		HomeID:        "home-1",
		PartnershipID: "partnership-42",
	})

	host := &testZeroneHost{hk: mockHK}
	pid := host.HPartner(hbPadHomeID("home-1"))
	if string(pid) != "partnership-42" {
		t.Fatalf("expected partnership 'partnership-42', got '%s'", string(pid))
	}
}

func TestHPartner_NoPartnership(t *testing.T) {
	_, mockHK := setupKeeperWithHome(t)

	mockHK.addHome(types.HomeInfo{
		HomeID:        "home-1",
		PartnershipID: "",
	})

	host := &testZeroneHost{hk: mockHK}
	pid := host.HPartner(hbPadHomeID("home-1"))
	if len(pid) != 0 {
		t.Fatalf("expected empty partnership, got '%s'", string(pid))
	}
}

// ---------- Gas Tests ----------

func TestHomeOpcodeGasCost(t *testing.T) {
	tests := []struct {
		opcode byte
		name   string
		gas    uint64
	}{
		{vm.HQUERY, "HQUERY", vm.GasHQuery},
		{vm.HMEMORY, "HMEMORY", vm.GasHMemory},
		{vm.HPARTNER, "HPARTNER", vm.GasHPartner},
	}
	for _, tc := range tests {
		info, ok := vm.OpcodeTable[tc.opcode]
		if !ok {
			t.Fatalf("%s not found in OpcodeTable", tc.name)
		}
		if info.GasCost != tc.gas {
			t.Fatalf("%s gas cost: expected %d, got %d", tc.name, tc.gas, info.GasCost)
		}
		if info.IsStateModifier {
			t.Fatalf("%s should not be a state modifier", tc.name)
		}
	}
}

// ---------- Bytecode Execution Test ----------

func TestHQuery_BytecodeExecution(t *testing.T) {
	_, mockHK := setupKeeperWithHome(t)

	mockHK.addHome(types.HomeInfo{
		HomeID:       "home-1",
		OwnerAddress: testDeployer,
		Status:       "active",
		MemoryCID:    "QmTest",
	})

	// Bytecode: CALLER, HQUERY, STOP
	// CALLER pushes caller address, HQUERY pops it and pushes 3 values
	bytecode := []byte{
		vm.CALLER, // push caller address
		vm.HQUERY, // pop address, push (exists, homeId, status)
		vm.STOP,
	}

	callerBytes := accAddrToBytes(testDeployer)
	host := &testZeroneHost{hk: mockHK}

	interp := vm.NewInterpreter()
	execCtx := &vm.ExecutionContext{
		Caller:             callerBytes,
		Origin:             callerBytes,
		CurrentContract:    []byte("contract"),
		GasLimit:           100000,
		Bytecode:           bytecode,
		ContractBvmVersion: 1,
	}

	stateDB := vm.NewMemoryStateDB()
	result := interp.Execute(bytecode, execCtx, stateDB, host)
	if !result.Success {
		errMsg := ""
		if result.Error != nil {
			errMsg = result.Error.Message
		}
		t.Fatalf("execution failed: %s", errMsg)
	}
	if result.GasUsed < vm.GasHQuery {
		t.Fatalf("expected at least %d gas used, got %d", vm.GasHQuery, result.GasUsed)
	}
}

// ---------- Helpers ----------

// testZeroneHost implements vm.HostFunctions for home bridge testing.
type testZeroneHost struct {
	kk types.KnowledgeKeeper
	hk types.HomeKeeper
}

func (h *testZeroneHost) KQuery(_ []byte) (bool, uint64, []byte)       { return false, 0, nil }
func (h *testZeroneHost) KVerify(_ string, _ []byte, _ []byte) bool    { return false }
func (h *testZeroneHost) KCite(_ string, _ []byte) bool                { return true }

func (h *testZeroneHost) HQuery(callerAddr []byte) (bool, []byte, []byte) {
	if h.hk == nil {
		return false, nil, nil
	}
	addr := sdk.AccAddress(callerAddr).String()
	homeIDs := h.hk.GetHomesByOwner(nil, addr)
	if len(homeIDs) == 0 {
		return false, nil, nil
	}
	homeID := homeIDs[0]
	status := h.hk.GetHomeStatus(nil, homeID)
	return true, []byte(homeID), []byte(status)
}

func (h *testZeroneHost) HMemory(homeId []byte) []byte {
	if h.hk == nil {
		return nil
	}
	homeIDStr := hbTrimNull(homeId)
	cid := h.hk.GetMemoryCID(nil, homeIDStr)
	return []byte(cid)
}

func (h *testZeroneHost) HPartner(homeId []byte) []byte {
	if h.hk == nil {
		return nil
	}
	homeIDStr := hbTrimNull(homeId)
	pid := h.hk.GetPartnershipID(nil, homeIDStr)
	return []byte(pid)
}

var _ vm.HostFunctions = (*testZeroneHost)(nil)

// accAddrToBytes converts a bech32 address to raw bytes.
func accAddrToBytes(addr string) []byte {
	accAddr, _ := sdk.AccAddressFromBech32(addr)
	result := make([]byte, 20)
	copy(result, accAddr)
	return result
}

// hbPadHomeID creates a 32-byte array with the homeID left-aligned and null-padded.
func hbPadHomeID(homeID string) []byte {
	buf := make([]byte, 32)
	copy(buf, homeID)
	return buf
}

// hbTrimNull trims trailing null bytes from a byte slice.
func hbTrimNull(b []byte) string {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != 0 {
			return string(b[:i+1])
		}
	}
	return ""
}

// hbUitoa converts uint64 to string.
func hbUitoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
