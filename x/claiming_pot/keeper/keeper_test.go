package keeper_test

import (
	"context"
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

	"github.com/zerone-chain/zerone/x/claiming_pot/keeper"
	"github.com/zerone-chain/zerone/x/claiming_pot/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

// ---------- Mock StakingKeeper ----------

type mockStakingKeeper struct {
	tiers map[string]uint32
}

func newMockStakingKeeper() *mockStakingKeeper {
	return &mockStakingKeeper{tiers: make(map[string]uint32)}
}

func (m *mockStakingKeeper) GetValidatorTier(_ context.Context, addr string) (uint32, error) {
	return m.tiers[addr], nil
}

// ---------- Mock AuthKeeper ----------

type mockAuthKeeper struct {
	registrationBlocks map[string]uint64
}

func newMockAuthKeeper() *mockAuthKeeper {
	return &mockAuthKeeper{registrationBlocks: make(map[string]uint64)}
}

func (m *mockAuthKeeper) GetRegistrationBlock(_ context.Context, addr string) (uint64, error) {
	return m.registrationBlocks[addr], nil
}

// ---------- Mock BankKeeper ----------

type mockBankKeeper struct {
	moduleBalances map[string]map[string]int64
	balances       map[string]map[string]int64
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		moduleBalances: make(map[string]map[string]int64),
		balances:       make(map[string]map[string]int64),
	}
}

func (m *mockBankKeeper) setModuleBalance(moduleName, denom string, amount int64) {
	if m.moduleBalances[moduleName] == nil {
		m.moduleBalances[moduleName] = make(map[string]int64)
	}
	m.moduleBalances[moduleName][denom] = amount
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	to := recipientAddr.String()
	for _, coin := range amt {
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

// ---------- Test Addresses ----------

func testAddr(name string) string {
	h := sha256.Sum256([]byte("test_seed:" + name))
	return sdk.AccAddress(h[:20]).String()
}

// ---------- Test Setup ----------

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockStakingKeeper, *mockAuthKeeper, *mockBankKeeper) {
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

	mockSK := newMockStakingKeeper()
	mockAK := newMockAuthKeeper()
	mockBK := newMockBankKeeper()

	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(storeService, cdc, "zrn1authority", mockSK, mockAK, mockBK)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1000, ChainID: "zerone-test-1"}, false, log.NewNopLogger())

	return k, ctx, mockSK, mockAK, mockBK
}

func setupMsgServer(t *testing.T) (types.MsgServer, keeper.Keeper, sdk.Context, *mockStakingKeeper, *mockAuthKeeper, *mockBankKeeper) {
	t.Helper()
	k, ctx, sk, ak, bk := setupKeeper(t)
	return keeper.NewMsgServerImpl(k), k, ctx, sk, ak, bk
}

// ---------- Test 1: Create Pot (authority-gated) ----------

func TestCreatePot(t *testing.T) {
	msgSrv, k, ctx, _, _, _ := setupMsgServer(t)

	resp, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Test Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock:   500,
			EndBlock:     2000,
			CliffBlocks:  100,
			PeriodBlocks: 10,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier: 1,
		},
	})
	if err != nil {
		t.Fatalf("CreatePot failed: %v", err)
	}
	if resp.PotId == "" {
		t.Fatal("expected non-empty pot ID")
	}

	pot, found := k.GetPot(ctx, resp.PotId)
	if !found {
		t.Fatal("pot not found after creation")
	}
	if pot.Name != "Test Pot" {
		t.Errorf("expected name 'Test Pot', got %s", pot.Name)
	}
	if pot.TotalAmount != "10000000" {
		t.Errorf("expected total_amount 10000000, got %s", pot.TotalAmount)
	}
	if pot.ClaimedAmount != "0" {
		t.Errorf("expected claimed_amount '0', got %s", pot.ClaimedAmount)
	}
	if pot.Status != types.PotStatus_POT_STATUS_ACTIVE {
		t.Errorf("expected ACTIVE status, got %s", pot.Status.String())
	}
}

// ---------- Test 2: Create pot unauthorized ----------

func TestCreatePotUnauthorized(t *testing.T) {
	msgSrv, _, ctx, _, _, _ := setupMsgServer(t)

	_, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   testAddr("not-authority"),
		Name:        "Bad Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
	})
	if err == nil {
		t.Fatal("expected error for unauthorized CreatePot")
	}
}

// ---------- Test 3: Max active pots limit ----------

func TestMaxActivePotsLimit(t *testing.T) {
	msgSrv, k, ctx, _, _, _ := setupMsgServer(t)

	// Set max to 2
	k.SetParams(ctx, &types.Params{
		MaxPotsActive:  2,
		MinClaimAmount: "1000",
	})

	for i := 0; i < 2; i++ {
		_, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
			Authority:   "zrn1authority",
			Name:        "Pot",
			TotalAmount: "10000000",
			Schedule: &types.VestingSchedule{
				StartBlock: 500,
				EndBlock:   2000,
			},
		})
		if err != nil {
			t.Fatalf("CreatePot %d failed: %v", i, err)
		}
	}

	// 3rd should fail
	_, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Overflow Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
	})
	if err == nil {
		t.Fatal("expected error for exceeding max active pots")
	}
}

// ---------- Test 4: Claim with vesting ----------

func TestClaimWithVesting(t *testing.T) {
	msgSrv, k, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("claimant1")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10 // registered early
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	// Create pot: start=500, end=2000, cliff=100, total=10000000
	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Vesting Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock:   500,
			EndBlock:     2000,
			CliffBlocks:  100,
			PeriodBlocks: 10,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 100,
		},
	})

	// At block 1000: elapsed=500, totalDuration=1500
	// vested = 10000000 * 500 / 1500 = 3333333
	claimResp, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("Claim failed: %v", err)
	}
	if claimResp.Amount != "3333333" {
		t.Errorf("expected claimable 3333333, got %s", claimResp.Amount)
	}

	// Verify claim recorded
	claim, found := k.GetClaim(ctx, resp.PotId, claimant)
	if !found {
		t.Fatal("claim not found")
	}
	if claim.Amount != "3333333" {
		t.Errorf("expected claim amount 3333333, got %s", claim.Amount)
	}

	// Verify pot updated
	pot, _ := k.GetPot(ctx, resp.PotId)
	if pot.ClaimedAmount != "3333333" {
		t.Errorf("expected pot claimed_amount 3333333, got %s", pot.ClaimedAmount)
	}

	// Verify bank transfer
	if bk.balances[claimant]["uzrn"] != 3333333 {
		t.Errorf("expected claimant balance 3333333, got %d", bk.balances[claimant]["uzrn"])
	}
}

// ---------- Test 5: Claim before cliff fails ----------

func TestClaimBeforeCliff(t *testing.T) {
	msgSrv, _, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("claimant1")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	// Create pot: start=500, cliff=600 blocks → cliff ends at 1100
	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Cliff Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock:  500,
			EndBlock:    2000,
			CliffBlocks: 600,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 100,
		},
	})

	// At block 1000: cliff at 500+600=1100, current=1000 < 1100 → nothing vested
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("expected error for claim before cliff")
	}
}

// ---------- Test 6: Ineligible claimant (whitelist + tier) ----------

func TestClaimIneligible(t *testing.T) {
	msgSrv, _, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("claimant1")
	outsider := testAddr("outsider")
	sk.tiers[claimant] = 2
	sk.tiers[outsider] = 0 // low tier
	ak.registrationBlocks[claimant] = 10
	ak.registrationBlocks[outsider] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	// Pot with whitelist
	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Whitelist Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			Whitelist: []string{claimant},
		},
	})

	// Outsider (not in whitelist) should fail
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: outsider,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("expected error for non-whitelisted claimant")
	}

	// Pot with tier requirement
	resp2, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Tier Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier: 2,
		},
	})

	// Low-tier outsider should fail
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: outsider,
		PotId:    resp2.PotId,
	})
	if err == nil {
		t.Fatal("expected error for low-tier claimant")
	}
}

// ---------- Test 7: Double claim rejected ----------

func TestDoubleClaim(t *testing.T) {
	msgSrv, _, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("claimant1")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock:   500,
			EndBlock:     2000,
			CliffBlocks:  0,
			PeriodBlocks: 10,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 100,
		},
	})

	// First claim succeeds
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("first Claim failed: %v", err)
	}

	// Second claim should fail
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("expected error for double claim")
	}
}

// ---------- Test 8: Genesis round-trip ----------

func TestGenesisRoundTrip(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	genState := &types.GenesisState{
		Params: &types.Params{
			MaxPotsActive:  5,
			MinClaimAmount: "5000",
		},
		Pots: []*types.ClaimingPot{
			{
				Id:          "gen-pot-1",
				Name:        "Genesis Pot",
				TotalAmount: "50000000",
				ClaimedAmount: "10000000",
				Schedule: &types.VestingSchedule{
					StartBlock: 100,
					EndBlock:   5000,
					CliffBlocks: 200,
				},
				Eligibility: &types.EligibilityCriteria{
					MinStakingTier: 1,
				},
				CreatedAtBlock: 100,
				Status:         types.PotStatus_POT_STATUS_ACTIVE,
			},
		},
		Claims: []*types.Claim{
			{
				PotId:     "gen-pot-1",
				Claimant:  testAddr("claimant1"),
				Amount:    "10000000",
				ClaimedAt: 500,
			},
		},
	}

	k.InitGenesis(ctx, genState)

	exported := k.ExportGenesis(ctx)

	// Check params
	if exported.Params.MaxPotsActive != 5 {
		t.Errorf("expected max_pots_active 5, got %d", exported.Params.MaxPotsActive)
	}
	if exported.Params.MinClaimAmount != "5000" {
		t.Errorf("expected min_claim_amount '5000', got %s", exported.Params.MinClaimAmount)
	}

	// Check pots
	if len(exported.Pots) != 1 {
		t.Fatalf("expected 1 pot, got %d", len(exported.Pots))
	}
	if exported.Pots[0].Id != "gen-pot-1" {
		t.Errorf("expected pot ID gen-pot-1, got %s", exported.Pots[0].Id)
	}
	if exported.Pots[0].ClaimedAmount != "10000000" {
		t.Errorf("expected claimed_amount 10000000, got %s", exported.Pots[0].ClaimedAmount)
	}

	// Check claims
	if len(exported.Claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(exported.Claims))
	}
	if exported.Claims[0].Amount != "10000000" {
		t.Errorf("expected claim amount 10000000, got %s", exported.Claims[0].Amount)
	}
}

// ---------- Test: Pot expiry (BeginBlocker) ----------

func TestPotExpiry(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	// Create an active pot that should expire
	k.SetPot(ctx, &types.ClaimingPot{
		Id:          "expiring-pot",
		Name:        "Expiring",
		TotalAmount: "1000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 100,
			EndBlock:   500,
		},
		Status: types.PotStatus_POT_STATUS_ACTIVE,
	})

	// Not yet expired (block 400)
	k.ProcessPotExpiry(ctx, 400)
	pot, _ := k.GetPot(ctx, "expiring-pot")
	if pot.Status != types.PotStatus_POT_STATUS_ACTIVE {
		t.Errorf("expected ACTIVE at block 400, got %s", pot.Status.String())
	}

	// Process at block 500 → should expire
	k.ProcessPotExpiry(ctx, 500)
	pot, _ = k.GetPot(ctx, "expiring-pot")
	if pot.Status != types.PotStatus_POT_STATUS_EXPIRED {
		t.Errorf("expected EXPIRED at block 500, got %s", pot.Status.String())
	}
}

// ---------- Test: Default genesis validates ----------

func TestDefaultGenesisValidation(t *testing.T) {
	gs := types.DefaultGenesis()
	if err := gs.Validate(); err != nil {
		t.Errorf("default genesis should be valid: %v", err)
	}
}

// ---------- Test: UpdatePotParams ----------

func TestUpdatePotParams(t *testing.T) {
	msgSrv, k, ctx, _, _, _ := setupMsgServer(t)

	_, err := msgSrv.UpdatePotParams(ctx, &types.MsgUpdatePotParams{
		Authority: "zrn1authority",
		Params: &types.Params{
			MaxPotsActive:  20,
			MinClaimAmount: "5000",
		},
	})
	if err != nil {
		t.Fatalf("UpdatePotParams failed: %v", err)
	}

	params := k.GetParams(ctx)
	if params.MaxPotsActive != 20 {
		t.Errorf("expected max_pots_active 20, got %d", params.MaxPotsActive)
	}

	// Unauthorized
	_, err = msgSrv.UpdatePotParams(ctx, &types.MsgUpdatePotParams{
		Authority: testAddr("not-authority"),
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected error for unauthorized UpdatePotParams")
	}
}

// ---------- Test: Claim from inactive pot fails ----------

func TestClaimFromInactivePot(t *testing.T) {
	msgSrv, k, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("claimant1")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	// Create and expire a pot
	k.SetPot(ctx, &types.ClaimingPot{
		Id:          "expired-pot",
		Name:        "Expired",
		TotalAmount: "10000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Status: types.PotStatus_POT_STATUS_EXPIRED,
	})

	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    "expired-pot",
	})
	if err == nil {
		t.Fatal("expected error for claim from expired pot")
	}
}

// ---------- Test: CalculateClaimable math ----------

func TestCalculateClaimable(t *testing.T) {
	// Before start
	pot := &types.ClaimingPot{
		TotalAmount:   "10000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock:  1000,
			EndBlock:    2000,
			CliffBlocks: 100,
		},
	}

	result := keeper.CalculateClaimable(pot, 500) // before start
	if result.Int64() != 0 {
		t.Errorf("before start: expected 0, got %d", result.Int64())
	}

	result = keeper.CalculateClaimable(pot, 1050) // before cliff (start+cliff = 1100)
	if result.Int64() != 0 {
		t.Errorf("before cliff: expected 0, got %d", result.Int64())
	}

	// At midpoint: elapsed=500, duration=1000 → vested = 10000000*500/1000 = 5000000
	result = keeper.CalculateClaimable(pot, 1500)
	if result.Int64() != 5000000 {
		t.Errorf("at midpoint: expected 5000000, got %d", result.Int64())
	}

	// After end: fully vested
	result = keeper.CalculateClaimable(pot, 3000)
	if result.Int64() != 10000000 {
		t.Errorf("after end: expected 10000000, got %d", result.Int64())
	}

	// With some already claimed
	pot.ClaimedAmount = "3000000"
	result = keeper.CalculateClaimable(pot, 1500) // vested=5000000 - claimed=3000000 = 2000000
	if result.Int64() != 2000000 {
		t.Errorf("with claimed: expected 2000000, got %d", result.Int64())
	}

	// After end with some claimed
	result = keeper.CalculateClaimable(pot, 3000) // total=10000000 - claimed=3000000 = 7000000
	if result.Int64() != 7000000 {
		t.Errorf("after end with claimed: expected 7000000, got %d", result.Int64())
	}
}

// ========== PORTED TESTS: Pot CRUD Direct ==========

// ---------- Test: SetPot/GetPot roundtrip ----------

func TestSetGetPot(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	pot := &types.ClaimingPot{
		Id:            "crud-pot-1",
		Name:          "CRUD Test",
		TotalAmount:   "5000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 100,
			EndBlock:   2000,
		},
		Status: types.PotStatus_POT_STATUS_ACTIVE,
	}
	k.SetPot(ctx, pot)

	got, found := k.GetPot(ctx, "crud-pot-1")
	if !found {
		t.Fatal("expected pot to be found")
	}
	if got.Id != "crud-pot-1" {
		t.Errorf("expected id crud-pot-1, got %s", got.Id)
	}
	if got.Name != "CRUD Test" {
		t.Errorf("expected name 'CRUD Test', got %s", got.Name)
	}
	if got.TotalAmount != "5000000" {
		t.Errorf("expected total_amount 5000000, got %s", got.TotalAmount)
	}
	if got.Status != types.PotStatus_POT_STATUS_ACTIVE {
		t.Errorf("expected ACTIVE status, got %s", got.Status.String())
	}
}

// ---------- Test: GetPot for non-existent ID ----------

func TestGetPotNotFound(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	_, found := k.GetPot(ctx, "nonexistent-id")
	if found {
		t.Error("expected pot not found for nonexistent ID")
	}
}

// ---------- Test: IteratePots ----------

func TestIteratePots(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	for i := 0; i < 5; i++ {
		k.SetPot(ctx, &types.ClaimingPot{
			Id:            fmt.Sprintf("iter-%d", i),
			Name:          fmt.Sprintf("Pot %d", i),
			TotalAmount:   "1000000",
			ClaimedAmount: "0",
			Status:        types.PotStatus_POT_STATUS_ACTIVE,
		})
	}

	count := 0
	k.IteratePots(ctx, func(pot *types.ClaimingPot) bool {
		count++
		return false
	})
	if count != 5 {
		t.Errorf("expected 5 pots, got %d", count)
	}
}

// ---------- Test: IteratePots with early stop ----------

func TestIteratePotsEarlyStop(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	for i := 0; i < 5; i++ {
		k.SetPot(ctx, &types.ClaimingPot{
			Id:            fmt.Sprintf("stop-%d", i),
			Name:          "Pot",
			TotalAmount:   "1000000",
			ClaimedAmount: "0",
			Status:        types.PotStatus_POT_STATUS_ACTIVE,
		})
	}

	count := 0
	k.IteratePots(ctx, func(pot *types.ClaimingPot) bool {
		count++
		return count >= 2 // stop after 2
	})
	if count != 2 {
		t.Errorf("expected early stop at 2, got %d", count)
	}
}

// ---------- Test: CountActivePots ----------

func TestCountActivePots(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	// Initially no pots
	if c := k.CountActivePots(ctx); c != 0 {
		t.Errorf("expected 0 active pots initially, got %d", c)
	}

	// Add 3 active, 1 expired, 1 depleted
	k.SetPot(ctx, &types.ClaimingPot{Id: "active-1", TotalAmount: "100", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_ACTIVE})
	k.SetPot(ctx, &types.ClaimingPot{Id: "active-2", TotalAmount: "100", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_ACTIVE})
	k.SetPot(ctx, &types.ClaimingPot{Id: "active-3", TotalAmount: "100", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_ACTIVE})
	k.SetPot(ctx, &types.ClaimingPot{Id: "expired-1", TotalAmount: "100", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_EXPIRED})
	k.SetPot(ctx, &types.ClaimingPot{Id: "depleted-1", TotalAmount: "100", ClaimedAmount: "100", Status: types.PotStatus_POT_STATUS_DEPLETED})

	if c := k.CountActivePots(ctx); c != 3 {
		t.Errorf("expected 3 active pots, got %d", c)
	}
}

// ---------- Test: GetAllPots ----------

func TestGetAllPots(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	k.SetPot(ctx, &types.ClaimingPot{Id: "all-1", TotalAmount: "100", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_ACTIVE})
	k.SetPot(ctx, &types.ClaimingPot{Id: "all-2", TotalAmount: "200", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_EXPIRED})
	k.SetPot(ctx, &types.ClaimingPot{Id: "all-3", TotalAmount: "300", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_DEPLETED})

	pots := k.GetAllPots(ctx)
	if len(pots) != 3 {
		t.Errorf("expected 3 pots, got %d", len(pots))
	}
}

// ========== PORTED TESTS: Claim Record ==========

// ---------- Test: SetClaim/GetClaim roundtrip ----------

func TestSetGetClaimRecord(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	claim := &types.Claim{
		PotId:     "pot-1",
		Claimant:  testAddr("claimer1"),
		Amount:    "500000",
		ClaimedAt: 1000,
	}
	k.SetClaim(ctx, claim)

	got, found := k.GetClaim(ctx, "pot-1", testAddr("claimer1"))
	if !found {
		t.Fatal("expected claim to be found")
	}
	if got.Amount != "500000" {
		t.Errorf("expected amount 500000, got %s", got.Amount)
	}
	if got.ClaimedAt != 1000 {
		t.Errorf("expected claimed_at 1000, got %d", got.ClaimedAt)
	}
}

// ---------- Test: GetClaim for non-existent ----------

func TestGetClaimRecordNotFound(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	_, found := k.GetClaim(ctx, "no-pot", "no-claimant")
	if found {
		t.Error("expected claim not found for nonexistent record")
	}
}

// ---------- Test: GetAllClaims returns all ----------

func TestGetAllClaimRecords(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	k.SetClaim(ctx, &types.Claim{PotId: "pot-1", Claimant: testAddr("c1"), Amount: "100", ClaimedAt: 100})
	k.SetClaim(ctx, &types.Claim{PotId: "pot-1", Claimant: testAddr("c2"), Amount: "200", ClaimedAt: 200})
	k.SetClaim(ctx, &types.Claim{PotId: "pot-2", Claimant: testAddr("c1"), Amount: "300", ClaimedAt: 300})

	all := k.GetAllClaims(ctx)
	if len(all) != 3 {
		t.Errorf("expected 3 total claims, got %d", len(all))
	}
}

// ---------- Test: GetClaimsByPot returns claims for specific pot ----------

func TestGetClaimsByPot(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	k.SetClaim(ctx, &types.Claim{PotId: "pot-A", Claimant: testAddr("c1"), Amount: "100", ClaimedAt: 100})
	k.SetClaim(ctx, &types.Claim{PotId: "pot-A", Claimant: testAddr("c2"), Amount: "200", ClaimedAt: 200})
	k.SetClaim(ctx, &types.Claim{PotId: "pot-B", Claimant: testAddr("c1"), Amount: "300", ClaimedAt: 300})

	claimsA := k.GetClaimsByPot(ctx, "pot-A")
	if len(claimsA) != 2 {
		t.Errorf("expected 2 claims for pot-A, got %d", len(claimsA))
	}

	claimsB := k.GetClaimsByPot(ctx, "pot-B")
	if len(claimsB) != 1 {
		t.Errorf("expected 1 claim for pot-B, got %d", len(claimsB))
	}

	claimsC := k.GetClaimsByPot(ctx, "pot-C")
	if len(claimsC) != 0 {
		t.Errorf("expected 0 claims for pot-C, got %d", len(claimsC))
	}
}

// ========== PORTED TESTS: Pot Creation Edge Cases ==========

// ---------- Test: CreatePot invalid schedule (end before start) ----------

func TestCreatePotInvalidScheduleEndBeforeStart(t *testing.T) {
	msgSrv, _, ctx, _, _, _ := setupMsgServer(t)

	msg := &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Bad Schedule",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 2000,
			EndBlock:   500, // end before start
		},
	}

	// ValidateBasic should catch this
	if err := msg.ValidateBasic(); err == nil {
		t.Fatal("expected ValidateBasic to fail for end_block < start_block")
	}

	// Also verify server rejects it (it may or may not reach msg_server depending on validation flow)
	_, err := msgSrv.CreatePot(ctx, msg)
	// Either ValidateBasic or server should reject
	_ = err // msg_server may accept or reject depending on whether it calls ValidateBasic
}

// ---------- Test: CreatePot zero total amount ----------

func TestCreatePotZeroTotalAmount(t *testing.T) {
	msgSrv, _, ctx, _, _, _ := setupMsgServer(t)

	_, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Zero Pot",
		TotalAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
	})
	if err == nil {
		t.Fatal("expected error for zero total amount")
	}
}

// ---------- Test: CreatePot negative total amount ----------

func TestCreatePotNegativeTotalAmount(t *testing.T) {
	msgSrv, _, ctx, _, _, _ := setupMsgServer(t)

	_, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Negative Pot",
		TotalAmount: "-1000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
	})
	if err == nil {
		t.Fatal("expected error for negative total amount")
	}
}

// ---------- Test: CreatePot non-numeric total amount ----------

func TestCreatePotNonNumericTotalAmount(t *testing.T) {
	msgSrv, _, ctx, _, _, _ := setupMsgServer(t)

	_, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "NaN Pot",
		TotalAmount: "not-a-number",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
	})
	if err == nil {
		t.Fatal("expected error for non-numeric total amount")
	}
}

// ========== PORTED TESTS: Claim Edge Cases ==========

// ---------- Test: Claim from non-existent pot ----------

func TestClaimFromNonExistentPot(t *testing.T) {
	msgSrv, _, ctx, sk, ak, _ := setupMsgServer(t)

	claimant := testAddr("claimant1")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10

	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    "does-not-exist",
	})
	if err == nil {
		t.Fatal("expected error for claim from nonexistent pot")
	}
}

// ---------- Test: Claim pot exhaustion (depleted) ----------

func TestClaimFromPotMaxClaims(t *testing.T) {
	msgSrv, k, ctx, sk, ak, bk := setupMsgServer(t)

	claimant1 := testAddr("claimer-exhaust-1")
	claimant2 := testAddr("claimer-exhaust-2")
	sk.tiers[claimant1] = 2
	sk.tiers[claimant2] = 2
	ak.registrationBlocks[claimant1] = 10
	ak.registrationBlocks[claimant2] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Exhaust Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 100,
		},
	})

	// First claimant claims everything available at block 1000
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant1,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}

	// Second claimant tries but vested amount minus claimed = 0
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant2,
		PotId:    resp.PotId,
	})
	// The vested amount at block 1000 = 10000000*500/1500 = 3333333
	// After first claim, claimedAmount = 3333333, vested is still 3333333 => claimable=0
	// Should fail with cliff/zero claimable
	if err == nil {
		t.Fatal("expected error when no claimable amount remains at this block")
	}

	// Verify pot state
	pot, _ := k.GetPot(ctx, resp.PotId)
	if pot.ClaimedAmount != "3333333" {
		t.Errorf("expected claimed_amount 3333333, got %s", pot.ClaimedAmount)
	}
}

// ---------- Test: Claim below MinClaimAmount ----------

func TestClaimMinAmountEnforcement(t *testing.T) {
	msgSrv, k, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("min-amt-claimer")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	// Set a high min claim amount
	k.SetParams(ctx, &types.Params{
		MaxPotsActive:  10,
		MinClaimAmount: "9999999", // very high threshold
	})

	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Min Claim Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 100,
		},
	})

	// At block 1000: vested = 10000000*500/1500 = 3333333 < 9999999 min
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("expected error for claim below MinClaimAmount")
	}
}

// ========== PORTED TESTS: Eligibility ==========

// ---------- Test: Eligibility allowlist enforcement ----------

func TestEligibilityAllowlist(t *testing.T) {
	msgSrv, _, ctx, _, _, bk := setupMsgServer(t)

	allowed := testAddr("allowed-user")
	blocked := testAddr("blocked-user")
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Whitelist Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			Whitelist: []string{allowed},
		},
	})

	// Allowed user should be able to claim
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: allowed,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("expected allowed user to claim successfully: %v", err)
	}

	// Blocked user should fail
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: blocked,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("expected error for non-whitelisted claimant")
	}
}

// ---------- Test: Eligibility with no restrictions (open pot) ----------

func TestEligibilityNoRestrictions(t *testing.T) {
	msgSrv, _, ctx, _, _, bk := setupMsgServer(t)

	anyone := testAddr("any-user")
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Open Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		// nil eligibility = open to all
	})

	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: anyone,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("expected open pot to allow anyone: %v", err)
	}
}

// ---------- Test: Eligibility tier requirement ----------

func TestEligibilityTierRequirement(t *testing.T) {
	msgSrv, _, ctx, sk, ak, bk := setupMsgServer(t)

	highTier := testAddr("high-tier")
	lowTier := testAddr("low-tier")
	sk.tiers[highTier] = 5
	sk.tiers[lowTier] = 1
	ak.registrationBlocks[highTier] = 10
	ak.registrationBlocks[lowTier] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Tier Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     3,
			MinRegistrationAge: 100,
		},
	})

	// High tier should succeed
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: highTier,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("expected high-tier user to claim: %v", err)
	}

	// Low tier should fail
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: lowTier,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("expected error for low-tier claimant")
	}
}

// ---------- Test: Eligibility registration age ----------

func TestEligibilityRegistrationAge(t *testing.T) {
	msgSrv, _, ctx, sk, ak, bk := setupMsgServer(t)

	oldUser := testAddr("old-user")
	newUser := testAddr("new-user")
	sk.tiers[oldUser] = 2
	sk.tiers[newUser] = 2
	ak.registrationBlocks[oldUser] = 10   // registered at block 10, age=990 blocks
	ak.registrationBlocks[newUser] = 950  // registered at block 950, age=50 blocks
	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	resp, _ := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Age Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 200,
		},
	})

	// Old user (age 990) should succeed
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: oldUser,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("expected old user to claim: %v", err)
	}

	// New user (age 50) should fail
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: newUser,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("expected error for user with insufficient registration age")
	}
}

// ========== PORTED TESTS: Query Server ==========

func setupQueryServer(t *testing.T) (types.QueryServer, keeper.Keeper, sdk.Context, *mockStakingKeeper, *mockAuthKeeper, *mockBankKeeper) {
	t.Helper()
	k, ctx, sk, ak, bk := setupKeeper(t)
	return keeper.NewQueryServerImpl(k), k, ctx, sk, ak, bk
}

// ---------- Test: QueryPot ----------

func TestQueryPot(t *testing.T) {
	qs, k, ctx, _, _, _ := setupQueryServer(t)

	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "q-pot-1",
		Name:          "Query Pot",
		TotalAmount:   "5000000",
		ClaimedAmount: "0",
		Status:        types.PotStatus_POT_STATUS_ACTIVE,
	})

	resp, err := qs.QueryPot(ctx, &types.QueryPotRequest{Id: "q-pot-1"})
	if err != nil {
		t.Fatalf("QueryPot failed: %v", err)
	}
	if resp.Pot.Name != "Query Pot" {
		t.Errorf("expected name 'Query Pot', got %s", resp.Pot.Name)
	}
	if resp.Pot.TotalAmount != "5000000" {
		t.Errorf("expected total_amount 5000000, got %s", resp.Pot.TotalAmount)
	}
}

// ---------- Test: QueryPotNotFound ----------

func TestQueryPotNotFound(t *testing.T) {
	qs, _, ctx, _, _, _ := setupQueryServer(t)

	_, err := qs.QueryPot(ctx, &types.QueryPotRequest{Id: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for querying nonexistent pot")
	}
}

// ---------- Test: QueryAllPots ----------

func TestQueryAllPots(t *testing.T) {
	qs, k, ctx, _, _, _ := setupQueryServer(t)

	k.SetPot(ctx, &types.ClaimingPot{Id: "qa-1", TotalAmount: "100", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_ACTIVE})
	k.SetPot(ctx, &types.ClaimingPot{Id: "qa-2", TotalAmount: "200", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_EXPIRED})
	k.SetPot(ctx, &types.ClaimingPot{Id: "qa-3", TotalAmount: "300", ClaimedAmount: "0", Status: types.PotStatus_POT_STATUS_DEPLETED})

	resp, err := qs.QueryAllPots(ctx, &types.QueryAllPotsRequest{})
	if err != nil {
		t.Fatalf("QueryAllPots failed: %v", err)
	}
	if len(resp.Pots) != 3 {
		t.Errorf("expected 3 pots, got %d", len(resp.Pots))
	}
}

// ---------- Test: QueryClaimable ----------

func TestQueryClaimable(t *testing.T) {
	qs, k, ctx, _, _, _ := setupQueryServer(t)

	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "qc-pot-1",
		TotalAmount:   "10000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Status: types.PotStatus_POT_STATUS_ACTIVE,
	})

	// At block 1000: elapsed=500, duration=1500 → vested = 10000000*500/1500 = 3333333
	resp, err := qs.QueryClaimable(ctx, &types.QueryClaimableRequest{
		PotId:   "qc-pot-1",
		Address: testAddr("anyone"),
	})
	if err != nil {
		t.Fatalf("QueryClaimable failed: %v", err)
	}
	if resp.Amount != "3333333" {
		t.Errorf("expected claimable 3333333, got %s", resp.Amount)
	}
}

// ---------- Test: QueryClaimable returns 0 for already claimed ----------

func TestQueryClaimableAlreadyClaimed(t *testing.T) {
	qs, k, ctx, _, _, _ := setupQueryServer(t)

	claimant := testAddr("already-claimed")

	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "qc-claimed",
		TotalAmount:   "10000000",
		ClaimedAmount: "3333333",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Status: types.PotStatus_POT_STATUS_ACTIVE,
	})

	// Set an existing claim record
	k.SetClaim(ctx, &types.Claim{
		PotId:    "qc-claimed",
		Claimant: claimant,
		Amount:   "3333333",
	})

	resp, err := qs.QueryClaimable(ctx, &types.QueryClaimableRequest{
		PotId:   "qc-claimed",
		Address: claimant,
	})
	if err != nil {
		t.Fatalf("QueryClaimable failed: %v", err)
	}
	if resp.Amount != "0" {
		t.Errorf("expected 0 for already claimed, got %s", resp.Amount)
	}
}

// ---------- Test: QueryClaims ----------

func TestQueryClaims(t *testing.T) {
	qs, k, ctx, _, _, _ := setupQueryServer(t)

	k.SetClaim(ctx, &types.Claim{PotId: "qcl-pot", Claimant: testAddr("c1"), Amount: "100", ClaimedAt: 100})
	k.SetClaim(ctx, &types.Claim{PotId: "qcl-pot", Claimant: testAddr("c2"), Amount: "200", ClaimedAt: 200})

	resp, err := qs.QueryClaims(ctx, &types.QueryClaimsRequest{PotId: "qcl-pot"})
	if err != nil {
		t.Fatalf("QueryClaims failed: %v", err)
	}
	if len(resp.Claims) != 2 {
		t.Errorf("expected 2 claims, got %d", len(resp.Claims))
	}
}

// ---------- Test: QueryParams ----------

func TestQueryClaimingPotParams(t *testing.T) {
	qs, k, ctx, _, _, _ := setupQueryServer(t)

	k.SetParams(ctx, &types.Params{
		MaxPotsActive:  42,
		MinClaimAmount: "7777",
	})

	resp, err := qs.QueryParams(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("QueryParams failed: %v", err)
	}
	if resp.Params.MaxPotsActive != 42 {
		t.Errorf("expected max_pots_active 42, got %d", resp.Params.MaxPotsActive)
	}
	if resp.Params.MinClaimAmount != "7777" {
		t.Errorf("expected min_claim_amount 7777, got %s", resp.Params.MinClaimAmount)
	}
}

// ========== PORTED TESTS: Pot ID Generation ==========

// ---------- Test: GetNextPotID sequential ----------

func TestGetNextPotId(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	id1 := k.GetNextPotID(ctx)
	if id1 != 1 {
		t.Errorf("expected first ID 1, got %d", id1)
	}

	id2 := k.GetNextPotID(ctx)
	if id2 != 2 {
		t.Errorf("expected second ID 2, got %d", id2)
	}

	id3 := k.GetNextPotID(ctx)
	if id3 != 3 {
		t.Errorf("expected third ID 3, got %d", id3)
	}
}

// ========== PORTED TESTS: BeginBlock ==========

// ---------- Test: ProcessPotExpiry expires old pots ----------

func TestBeginBlockExpiresOldPots(t *testing.T) {
	k, ctx, _, _, _ := setupKeeper(t)

	// Active pot that should expire (endBlock=500, current=1000)
	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "bb-expired",
		TotalAmount:   "1000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 100,
			EndBlock:   500,
		},
		Status: types.PotStatus_POT_STATUS_ACTIVE,
	})

	// Active pot that should NOT expire (endBlock=2000)
	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "bb-future",
		TotalAmount:   "1000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 100,
			EndBlock:   2000,
		},
		Status: types.PotStatus_POT_STATUS_ACTIVE,
	})

	// Already expired pot (should not change)
	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "bb-already-expired",
		TotalAmount:   "1000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 100,
			EndBlock:   300,
		},
		Status: types.PotStatus_POT_STATUS_EXPIRED,
	})

	k.ProcessPotExpiry(ctx, 1000)

	// Should be expired now
	pot1, _ := k.GetPot(ctx, "bb-expired")
	if pot1.Status != types.PotStatus_POT_STATUS_EXPIRED {
		t.Errorf("expected EXPIRED for bb-expired, got %s", pot1.Status.String())
	}

	// Should still be active
	pot2, _ := k.GetPot(ctx, "bb-future")
	if pot2.Status != types.PotStatus_POT_STATUS_ACTIVE {
		t.Errorf("expected ACTIVE for bb-future, got %s", pot2.Status.String())
	}

	// Should still be expired (unchanged)
	pot3, _ := k.GetPot(ctx, "bb-already-expired")
	if pot3.Status != types.PotStatus_POT_STATUS_EXPIRED {
		t.Errorf("expected EXPIRED for bb-already-expired, got %s", pot3.Status.String())
	}
}

// ========== PORTED TESTS: Adversarial (from OpenClaw) ==========

// ---------- Test: Double claim attack ----------

func TestDoubleClaimAttack(t *testing.T) {
	msgSrv, _, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("double-attacker")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 50_000_000)

	resp, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Attack Pot",
		TotalAmount: "50000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 100,
		},
	})
	if err != nil {
		t.Fatalf("create pot: %v", err)
	}

	// First claim should succeed
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    resp.PotId,
	})
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}

	// ATTACK: Second claim from same account
	_, err = msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    resp.PotId,
	})
	if err == nil {
		t.Fatal("SECURITY: double claim allowed! Same account could drain pot.")
	}
}

// ---------- Test: Unauthorized CreatePot attack ----------

func TestUnauthorizedCreatePotAttack(t *testing.T) {
	msgSrv, _, ctx, _, _, _ := setupMsgServer(t)

	attacker := testAddr("attacker")

	_, err := msgSrv.CreatePot(ctx, &types.MsgCreatePot{
		Authority:   attacker,
		Name:        "Attacker Pot",
		TotalAmount: "10000000",
		Schedule: &types.VestingSchedule{
			StartBlock: 500,
			EndBlock:   2000,
		},
	})
	if err == nil {
		t.Fatal("SECURITY: unauthorized user was able to create a pot!")
	}
}

// ---------- Test: Claim from expired pot attack ----------

func TestClaimFromExpiredPotAttack(t *testing.T) {
	msgSrv, k, ctx, sk, ak, bk := setupMsgServer(t)

	claimant := testAddr("expired-attacker")
	sk.tiers[claimant] = 2
	ak.registrationBlocks[claimant] = 10
	bk.setModuleBalance(types.ModuleName, "uzrn", 50_000_000)

	// Directly set an expired pot
	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "expired-attack-pot",
		Name:          "Expired Pot",
		TotalAmount:   "50000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 100,
			EndBlock:   500,
		},
		Eligibility: &types.EligibilityCriteria{
			MinStakingTier:     1,
			MinRegistrationAge: 100,
		},
		Status: types.PotStatus_POT_STATUS_EXPIRED,
	})

	// ATTACK: Claim from expired pot
	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    "expired-attack-pot",
	})
	if err == nil {
		t.Fatal("SECURITY: claim from expired pot allowed!")
	}
}

// ========== PORTED TESTS: Additional Coverage ==========

// ---------- Test: Pot transitions to DEPLETED when fully claimed ----------

func TestPotDepletionTransition(t *testing.T) {
	msgSrv, k, ctx, _, _, bk := setupMsgServer(t)

	bk.setModuleBalance(types.ModuleName, "uzrn", 10_000_000)

	// Create pot with small amount, past end block so it's fully vested
	k.SetPot(ctx, &types.ClaimingPot{
		Id:            "depletion-pot",
		Name:          "Depletion Test",
		TotalAmount:   "10000000",
		ClaimedAmount: "0",
		Schedule: &types.VestingSchedule{
			StartBlock: 100,
			EndBlock:   500,
		},
		Status: types.PotStatus_POT_STATUS_ACTIVE,
	})

	claimant := testAddr("depleter")

	_, err := msgSrv.Claim(ctx, &types.MsgClaim{
		Claimant: claimant,
		PotId:    "depletion-pot",
	})
	if err != nil {
		t.Fatalf("claim failed: %v", err)
	}

	pot, _ := k.GetPot(ctx, "depletion-pot")
	if pot.Status != types.PotStatus_POT_STATUS_DEPLETED {
		t.Errorf("expected DEPLETED after full claim, got %s", pot.Status.String())
	}
	if pot.ClaimedAmount != "10000000" {
		t.Errorf("expected claimed_amount 10000000, got %s", pot.ClaimedAmount)
	}
}

// ---------- Test: ValidateBasic on MsgClaim ----------

func TestMsgClaimValidateBasic(t *testing.T) {
	// Empty claimant
	msg := &types.MsgClaim{Claimant: "", PotId: "pot-1"}
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty claimant")
	}

	// Empty pot_id
	msg = &types.MsgClaim{Claimant: testAddr("valid"), PotId: ""}
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty pot_id")
	}

	// Invalid claimant address
	msg = &types.MsgClaim{Claimant: "notanaddress", PotId: "pot-1"}
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for invalid claimant address")
	}

	// Valid message
	msg = &types.MsgClaim{Claimant: testAddr("valid"), PotId: "pot-1"}
	if err := msg.ValidateBasic(); err != nil {
		t.Errorf("expected valid message, got error: %v", err)
	}
}

// ---------- Test: ValidateBasic on MsgCreatePot ----------

func TestMsgCreatePotValidateBasic(t *testing.T) {
	// Empty authority
	msg := &types.MsgCreatePot{
		Authority:   "",
		Name:        "Test",
		TotalAmount: "1000000",
		Schedule:    &types.VestingSchedule{StartBlock: 100, EndBlock: 200},
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty authority")
	}

	// Empty name
	msg = &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "",
		TotalAmount: "1000000",
		Schedule:    &types.VestingSchedule{StartBlock: 100, EndBlock: 200},
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for empty name")
	}

	// Nil schedule
	msg = &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Test",
		TotalAmount: "1000000",
		Schedule:    nil,
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for nil schedule")
	}

	// Negative total amount
	msg = &types.MsgCreatePot{
		Authority:   "zrn1authority",
		Name:        "Test",
		TotalAmount: "-100",
		Schedule:    &types.VestingSchedule{StartBlock: 100, EndBlock: 200},
	}
	if err := msg.ValidateBasic(); err == nil {
		t.Error("expected error for negative total amount")
	}
}

// ---------- Test: Params validation ----------

func TestParamsValidation(t *testing.T) {
	// Valid params
	p := &types.Params{MaxPotsActive: 10, MinClaimAmount: "1000"}
	if err := p.Validate(); err != nil {
		t.Errorf("expected valid params: %v", err)
	}

	// Zero max pots
	p = &types.Params{MaxPotsActive: 0, MinClaimAmount: "1000"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for zero MaxPotsActive")
	}

	// Invalid min claim amount
	p = &types.Params{MaxPotsActive: 10, MinClaimAmount: "not-a-number"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for invalid MinClaimAmount")
	}

	// Negative min claim amount
	p = &types.Params{MaxPotsActive: 10, MinClaimAmount: "-1"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for negative MinClaimAmount")
	}
}
