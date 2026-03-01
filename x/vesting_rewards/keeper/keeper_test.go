package keeper_test

import (
	"context"
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
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	commontypes "github.com/zerone-chain/zerone/x/common/types"
	"github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
	"github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.Seal()
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

	k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), nil, nil, "authority")
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1000}, false, log.NewNopLogger())

	gs := types.DefaultGenesis()
	k.InitGenesis(ctx, gs)

	return k, ctx
}

func setupMsgServer(t *testing.T) (types.MsgServer, keeper.Keeper, sdk.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	return keeper.NewMsgServerImpl(k), k, ctx
}

// ---------- CreateVesting Tests ----------

func TestCreateVesting_Success(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	recipient := sdk.AccAddress("recipient1__________").String()

	resp, err := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  recipient,
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})
	if err != nil {
		t.Fatalf("create vesting failed: %v", err)
	}
	if resp.VestingId == "" {
		t.Fatal("expected non-empty vesting ID")
	}

	schedule, found := k.GetVestingSchedule(ctx, resp.VestingId)
	if !found {
		t.Fatal("schedule not found")
	}
	if schedule.Recipient != recipient {
		t.Errorf("expected recipient %s, got %s", recipient, schedule.Recipient)
	}
	if schedule.Status != string(types.VestingStatusActive) {
		t.Errorf("expected active status, got %s", schedule.Status)
	}
	if schedule.TotalAmount != "1000000000000000000" {
		t.Errorf("expected total amount 1000000000000000000, got %s", schedule.TotalAmount)
	}
}

func TestCreateVesting_Unauthorized(t *testing.T) {
	ms, _, ctx := setupMsgServer(t)

	_, err := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "not-authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestCreateVesting_InvalidCategory(t *testing.T) {
	ms, _, ctx := setupMsgServer(t)

	resp, err := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_UNSPECIFIED,
		LinkedFactId: "fact-1",
	})
	if err != nil {
		t.Fatalf("expected unspecified category to fall back to default, got error: %v", err)
	}
	if resp.VestingId == "" {
		t.Fatal("expected non-empty vesting ID")
	}
}

// ---------- PauseVesting Tests ----------

func TestPauseVesting_Success(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.PauseVesting(ctx, &types.MsgPauseVesting{
		Authority: "authority",
		VestingId: resp.VestingId,
		Reason:    "active challenge",
	})
	if err != nil {
		t.Fatalf("pause vesting failed: %v", err)
	}

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	if schedule.Status != string(types.VestingStatusPaused) {
		t.Errorf("expected paused status, got %s", schedule.Status)
	}
	if schedule.PausedAtBlock != 1000 {
		t.Errorf("expected paused at block 1000, got %d", schedule.PausedAtBlock)
	}
}

func TestPauseVesting_Unauthorized(t *testing.T) {
	ms, _, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.PauseVesting(ctx, &types.MsgPauseVesting{
		Authority: "not-authority",
		VestingId: resp.VestingId,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// ---------- ResumeVesting Tests ----------

func TestResumeVesting_Success(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	ms.PauseVesting(ctx, &types.MsgPauseVesting{
		Authority: "authority", VestingId: resp.VestingId, Reason: "challenge",
	})

	ctx = ctx.WithBlockHeight(1100)
	_, err := ms.ResumeVesting(ctx, &types.MsgResumeVesting{
		Authority: "authority", VestingId: resp.VestingId,
	})
	if err != nil {
		t.Fatalf("resume vesting failed: %v", err)
	}

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	if schedule.Status != string(types.VestingStatusActive) {
		t.Errorf("expected active status, got %s", schedule.Status)
	}
	if schedule.TotalPausedBlocks != 100 {
		t.Errorf("expected 100 paused blocks, got %d", schedule.TotalPausedBlocks)
	}
	if schedule.PausedAtBlock != 0 {
		t.Errorf("expected paused_at_block reset to 0, got %d", schedule.PausedAtBlock)
	}
}

func TestResumeVesting_NotPaused(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.ResumeVesting(ctx, &types.MsgResumeVesting{
		Authority: "authority", VestingId: resp.VestingId,
	})
	if err != nil {
		t.Fatalf("resume vesting should not error for non-paused: %v", err)
	}

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	if schedule.Status != string(types.VestingStatusActive) {
		t.Errorf("expected active status unchanged, got %s", schedule.Status)
	}
}

// ---------- AccelerateVesting Tests ----------

func TestAccelerateVesting_Defense(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.AccelerateVesting(ctx, &types.MsgAccelerateVesting{
		Authority:          "authority",
		VestingId:          resp.VestingId,
		AccelerationFactor: 1000000,
	})
	if err != nil {
		t.Fatalf("accelerate vesting failed: %v", err)
	}

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	if schedule.DefenseCount != 1 {
		t.Errorf("expected defense count 1, got %d", schedule.DefenseCount)
	}
}

func TestAccelerateVesting_Replication(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.AccelerateVesting(ctx, &types.MsgAccelerateVesting{
		Authority:          "authority",
		VestingId:          resp.VestingId,
		AccelerationFactor: 600000,
	})
	if err != nil {
		t.Fatalf("accelerate vesting failed: %v", err)
	}

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	if schedule.ReplicationCount != 1 {
		t.Errorf("expected replication count 1, got %d", schedule.ReplicationCount)
	}
}

func TestAccelerateVesting_Unauthorized(t *testing.T) {
	ms, _, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.AccelerateVesting(ctx, &types.MsgAccelerateVesting{
		Authority:          "not-authority",
		VestingId:          resp.VestingId,
		AccelerationFactor: 1000000,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// ---------- CompleteVesting Tests ----------

func TestCompleteVesting_ByRecipient(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	recipient := sdk.AccAddress("recipient1__________").String()
	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  recipient,
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.CompleteVesting(ctx, &types.MsgCompleteVesting{
		Authority: recipient, VestingId: resp.VestingId,
	})
	if err != nil {
		t.Fatalf("complete vesting failed: %v", err)
	}

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	if schedule.Status != string(types.VestingStatusCompleted) {
		t.Errorf("expected completed status, got %s", schedule.Status)
	}
}

func TestCompleteVesting_ByAuthority(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.CompleteVesting(ctx, &types.MsgCompleteVesting{
		Authority: "authority", VestingId: resp.VestingId,
	})
	if err != nil {
		t.Fatalf("complete vesting by authority failed: %v", err)
	}

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	if schedule.Status != string(types.VestingStatusCompleted) {
		t.Errorf("expected completed status, got %s", schedule.Status)
	}
}

func TestCompleteVesting_Unauthorized(t *testing.T) {
	ms, _, ctx := setupMsgServer(t)

	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  sdk.AccAddress("recipient1__________").String(),
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	_, err := ms.CompleteVesting(ctx, &types.MsgCompleteVesting{
		Authority: sdk.AccAddress("random______________").String(),
		VestingId: resp.VestingId,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestCompleteVesting_AlreadyCompleted(t *testing.T) {
	ms, _, ctx := setupMsgServer(t)

	recipient := sdk.AccAddress("recipient1__________").String()
	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  recipient,
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	ms.CompleteVesting(ctx, &types.MsgCompleteVesting{
		Authority: recipient, VestingId: resp.VestingId,
	})

	_, err := ms.CompleteVesting(ctx, &types.MsgCompleteVesting{
		Authority: recipient, VestingId: resp.VestingId,
	})
	if err == nil {
		t.Fatal("expected error for already completed")
	}
}

func TestCompleteVesting_FalsifiedSchedule(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	recipient := sdk.AccAddress("recipient1__________").String()
	resp, _ := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  recipient,
		Amount:       "1000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-1",
	})

	schedule, _ := k.GetVestingSchedule(ctx, resp.VestingId)
	schedule.Status = string(types.VestingStatusFalsified)
	k.SetVestingSchedule(ctx, schedule)

	_, err := ms.CompleteVesting(ctx, &types.MsgCompleteVesting{
		Authority: recipient, VestingId: resp.VestingId,
	})
	if err == nil {
		t.Fatal("expected error for falsified schedule")
	}
}

// ---------- ValidateBasic Tests ----------

func TestMsgCreateVesting_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     *types.MsgCreateVesting
		wantErr bool
	}{
		{"valid", &types.MsgCreateVesting{
			Authority:   "authority",
			Beneficiary: sdk.AccAddress("recipient1__________").String(),
			Amount:      "1000",
		}, false},
		{"missing authority", &types.MsgCreateVesting{
			Beneficiary: sdk.AccAddress("recipient1__________").String(),
			Amount:      "1000",
		}, true},
		{"missing beneficiary", &types.MsgCreateVesting{
			Authority: "authority",
			Amount:    "1000",
		}, true},
		{"zero amount", &types.MsgCreateVesting{
			Authority:   "authority",
			Beneficiary: sdk.AccAddress("recipient1__________").String(),
			Amount:      "0",
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMsgAccelerateVesting_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     *types.MsgAccelerateVesting
		wantErr bool
	}{
		{"valid", &types.MsgAccelerateVesting{
			Authority: "authority", VestingId: "v1", AccelerationFactor: 1000000,
		}, false},
		{"missing vesting id", &types.MsgAccelerateVesting{
			Authority: "authority", AccelerationFactor: 1000000,
		}, true},
		{"missing authority", &types.MsgAccelerateVesting{
			VestingId: "v1", AccelerationFactor: 1000000,
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------- Mock Bank/Staking Keepers ----------

type mockBankKeeper struct {
	mintedCoins   sdk.Coins
	sentToAccount map[string]sdk.Coins
	sentToModule  map[string]sdk.Coins
	balances      map[string]sdk.Coins
	supply        map[string]sdkmath.Int
	mintErr       error
	sendAccErr    error
	sendModErr    error
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		sentToAccount: make(map[string]sdk.Coins),
		sentToModule:  make(map[string]sdk.Coins),
		supply:        make(map[string]sdkmath.Int),
	}
}

func (m *mockBankKeeper) MintCoins(_ context.Context, _ string, amounts sdk.Coins) error {
	if m.mintErr != nil {
		return m.mintErr
	}
	m.mintedCoins = m.mintedCoins.Add(amounts...)
	for _, coin := range amounts {
		cur, ok := m.supply[coin.Denom]
		if !ok {
			cur = sdkmath.ZeroInt()
		}
		m.supply[coin.Denom] = cur.Add(coin.Amount)
	}
	return nil
}



func (m *mockBankKeeper) GetSupply(_ context.Context, denom string) sdk.Coin {
	if amt, ok := m.supply[denom]; ok {
		return sdk.NewCoin(denom, amt)
	}
	return sdk.NewCoin(denom, sdkmath.ZeroInt())
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, _ string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	if m.sendAccErr != nil {
		return m.sendAccErr
	}
	key := recipientAddr.String()
	m.sentToAccount[key] = m.sentToAccount[key].Add(amt...)
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToModule(_ context.Context, _ string, recipientModule string, amt sdk.Coins) error {
	if m.sendModErr != nil {
		return m.sendModErr
	}
	m.sentToModule[recipientModule] = m.sentToModule[recipientModule].Add(amt...)
	return nil
}

func (m *mockBankKeeper) GetAllBalances(_ context.Context, addr sdk.AccAddress) sdk.Coins {
	if m.balances != nil {
		if coins, ok := m.balances[addr.String()]; ok {
			return coins
		}
	}
	return sdk.Coins{}
}

type mockStakingKeeper struct {
	activeCount uint32
}

func (m *mockStakingKeeper) GetActiveValidatorCount(_ context.Context) uint32 {
	return m.activeCount
}

func setupKeeperWithBank(t *testing.T, bk *mockBankKeeper, sk *mockStakingKeeper) (keeper.Keeper, sdk.Context) {
	t.Helper()
	return setupKeeperWithBankAndGenesis(t, bk, sk, types.DefaultGenesis())
}

func setupKeeperWithBankAndGenesis(t *testing.T, bk *mockBankKeeper, sk *mockStakingKeeper, gs *types.GenesisState) (keeper.Keeper, sdk.Context) {
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

	k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), bk, sk, "authority")
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1000}, false, log.NewNopLogger())

	k.InitGenesis(ctx, gs)

	return k, ctx
}

// ---------- Block Reward Distribution Tests ----------

func TestDistributeBlockReward_MintAndDistribute(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	mintedBefore := k.GetTotalMinted(ctx)
	if mintedBefore.Sign() != 0 {
		t.Fatalf("expected 0 total minted at genesis, got %s", mintedBefore.String())
	}

	producer := sdk.AccAddress("producer____________").String()

	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	if dist.TotalMinted == "0" {
		t.Fatal("expected non-zero total minted")
	}
	if dist.ProducerReward == "0" {
		t.Fatal("expected non-zero producer reward")
	}
	if dist.ResearchShare == "0" {
		t.Fatal("expected non-zero research share")
	}

	mintedAfter := k.GetTotalMinted(ctx)
	if mintedAfter.Cmp(mintedBefore) <= 0 {
		t.Fatal("expected total minted to increase")
	}
	if bk.mintedCoins.IsZero() {
		t.Fatal("expected MintCoins to be called")
	}
	if len(bk.sentToAccount) == 0 {
		t.Fatal("expected SendCoinsFromModuleToAccount for producer")
	}
	if _, ok := bk.sentToModule["research_fund"]; !ok {
		t.Fatal("expected SendCoinsFromModuleToModule for research_fund")
	}
}

func TestDistributeBlockReward_ValidatorScaling(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 3}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	producer := sdk.AccAddress("producer____________").String()

	dist, err := k.DistributeBlockReward(ctx, producer, 3, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	if dist.ValidatorCount != 3 {
		t.Errorf("expected validator count 3, got %d", dist.ValidatorCount)
	}
	if dist.TotalMinted == "0" {
		t.Fatal("expected non-zero total minted even with reduced validators")
	}
	// With 3/22 validators, reward < 10000000
	if dist.TotalMinted == "10000000" {
		t.Error("expected scaled reward, got full reward amount")
	}
}

func TestDistributeBlockReward_EmptyBlock(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	mintedBefore := k.GetTotalMinted(ctx)
	producer := sdk.AccAddress("producer____________").String()

	dist, err := k.DistributeBlockReward(ctx, producer, 22, false)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	if dist.TotalMinted != "0" {
		t.Errorf("expected 0 minted for empty block, got %s", dist.TotalMinted)
	}
	mintedAfter := k.GetTotalMinted(ctx)
	if mintedAfter.Cmp(mintedBefore) != 0 {
		t.Errorf("expected total minted unchanged for empty block")
	}
}

func TestDistributeBlockReward_NilBankKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)

	producer := sdk.AccAddress("producer____________").String()

	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	if dist.TotalMinted == "0" {
		t.Fatal("expected non-zero distribution record even without bank")
	}
	total := k.GetTotalMinted(ctx)
	if total.Sign() <= 0 {
		t.Fatal("expected total minted to increase even with nil bank")
	}
}

// ---------- Claim Rewards Tests ----------

func TestClaimRewards_SendsCoins(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	recipient := sdk.AccAddress("recipient1__________").String()

	schedule, err := k.CreateVestingSchedule(ctx, "claim-1", "fact-1", recipient,
		"1000000000000000000", types.CategoryFormalProof, types.SourceVerification)
	if err != nil {
		t.Fatalf("create vesting failed: %v", err)
	}

	ctx = ctx.WithBlockHeight(20000)

	claimed, err := k.ClaimRewards(ctx, recipient, schedule.Id)
	if err != nil {
		t.Fatalf("claim rewards failed: %v", err)
	}

	if claimed == "0" {
		t.Fatal("expected non-zero claimed amount after cliff")
	}

	if _, ok := bk.sentToAccount[recipient]; !ok {
		t.Fatal("expected SendCoinsFromModuleToAccount for recipient")
	}
}

func TestClaimRewards_NilBankKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)

	recipient := sdk.AccAddress("recipient1__________").String()

	schedule, err := k.CreateVestingSchedule(ctx, "claim-1", "fact-1", recipient,
		"1000000000000000000", types.CategoryFormalProof, types.SourceVerification)
	if err != nil {
		t.Fatalf("create vesting failed: %v", err)
	}

	ctx = ctx.WithBlockHeight(20000)

	claimed, err := k.ClaimRewards(ctx, recipient, schedule.Id)
	if err != nil {
		t.Fatalf("claim rewards with nil bank failed: %v", err)
	}
	if claimed == "0" {
		t.Fatal("expected non-zero claimed amount")
	}
}

// ---------- 4-Way Revenue Split Tests ----------

func TestDistributeRevenue_4WaySplit(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default split: contributor 55%, protocol 22%, research 3.33%, development 19.67%
	routing, err := k.DistributeRevenue(ctx, types.SourceBlockProduction, "10000",
		sdk.AccAddress("recipient___________").String(), "")
	if err != nil {
		t.Fatalf("distribute revenue failed: %v", err)
	}

	// 10000 * 550000 / 1000000 = 5500
	if routing.ContributorShare != "5500" {
		t.Errorf("expected contributor share 5500, got %s", routing.ContributorShare)
	}
	// 10000 * 220000 / 1000000 = 2200
	if routing.ProtocolShare != "2200" {
		t.Errorf("expected protocol share 2200, got %s", routing.ProtocolShare)
	}
	// 10000 * 33300 / 1000000 = 333
	if routing.ResearchShare != "333" {
		t.Errorf("expected research share 333, got %s", routing.ResearchShare)
	}
	// development = 10000 - 5500 - 2200 - 333 = 1967
	if routing.DevelopmentAmount != "1967" {
		t.Errorf("expected development amount 1967, got %s", routing.DevelopmentAmount)
	}
}

func TestDistributeRevenue_ProtocolSubSplit(t *testing.T) {
	k, ctx := setupKeeper(t)

	routing, err := k.DistributeRevenue(ctx, types.SourceBlockProduction, "10000000",
		sdk.AccAddress("recipient___________").String(), "")
	if err != nil {
		t.Fatalf("distribute revenue failed: %v", err)
	}

	// Protocol share: 10000000 * 220000 / 1000000 = 2200000
	protocolBig := new(big.Int)
	protocolBig.SetString(routing.ProtocolShare, 10)

	// Citation (50%): 2200000 * 500000 / 1000000 = 1100000
	if routing.CitationPool != "1100000" {
		t.Errorf("expected citation pool 1100000, got %s", routing.CitationPool)
	}
	// Verification (30%): 2200000 * 300000 / 1000000 = 660000
	if routing.VerificationPool != "660000" {
		t.Errorf("expected verification pool 660000, got %s", routing.VerificationPool)
	}
	// Treasury = remainder: 2200000 - 1100000 - 660000 = 440000
	if routing.TreasuryShare != "440000" {
		t.Errorf("expected treasury share 440000, got %s", routing.TreasuryShare)
	}
}

func TestDistributeRevenue_InvalidAmount(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.DistributeRevenue(ctx, types.SourceBlockProduction, "0", "addr", "")
	if err == nil {
		t.Fatal("expected error for zero amount")
	}

	_, err = k.DistributeRevenue(ctx, types.SourceBlockProduction, "notanumber", "addr", "")
	if err == nil {
		t.Fatal("expected error for non-numeric amount")
	}
}

func TestDistributeRevenue_SplitSumsToTotal(t *testing.T) {
	k, ctx := setupKeeper(t)

	routing, err := k.DistributeRevenue(ctx, types.SourceBlockProduction, "999999",
		sdk.AccAddress("recipient___________").String(), "")
	if err != nil {
		t.Fatalf("distribute revenue failed: %v", err)
	}

	contributor := new(big.Int)
	contributor.SetString(routing.ContributorShare, 10)
	protocol := new(big.Int)
	protocol.SetString(routing.ProtocolShare, 10)
	research := new(big.Int)
	research.SetString(routing.ResearchShare, 10)
	development := new(big.Int)
	development.SetString(routing.DevelopmentAmount, 10)

	total := new(big.Int).Add(contributor, protocol)
	total.Add(total, research)
	total.Add(total, development)

	if total.Int64() != 999999 {
		t.Errorf("expected split to sum to 999999, got %s", total.String())
	}
}

// ---------- Development Fund Deposit Tests ----------

func TestDistributeBlockReward_DepositsToDevelopmentFund(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	producer := sdk.AccAddress("producer____________").String()

	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	// 19.67% development: remainder = 10000000 - 5500000 - 2200000 - 333000 = 1967000
	if dist.DevelopmentAmount == "0" {
		t.Fatal("expected non-zero development amount")
	}

	devCoins := bk.sentToModule["development_fund"]
	if devCoins.AmountOf("uzrn").Int64() != 1967000 {
		t.Errorf("expected 1967000 uzrn to development_fund, got %d", devCoins.AmountOf("uzrn").Int64())
	}

}

// ---------- Falsify Claim Tests ----------

func TestFalsifyClaim_ClawbackCalculation(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	recipient := sdk.AccAddress("recipient1__________").String()

	schedule, err := k.CreateVestingSchedule(ctx, "claim-falsify", "fact-1", recipient,
		"1000000000000000000", types.CategoryFormalProof, types.SourceVerification)
	if err != nil {
		t.Fatalf("create vesting failed: %v", err)
	}

	ctx = ctx.WithBlockHeight(20000)
	k.ClaimRewards(ctx, recipient, schedule.Id)

	challenger := sdk.AccAddress("challenger__________").String()
	record, err := k.FalsifyClaim(ctx, "claim-falsify", challenger)
	if err != nil {
		t.Fatalf("falsify claim failed: %v", err)
	}

	if record.VestingId != schedule.Id {
		t.Errorf("expected vesting ID %s, got %s", schedule.Id, record.VestingId)
	}

	updated, _ := k.GetVestingSchedule(ctx, schedule.Id)
	if updated.Status != string(types.VestingStatusFalsified) {
		t.Errorf("expected falsified status, got %s", updated.Status)
	}
}

func TestFalsifyClaim_AlreadyFalsified(t *testing.T) {
	k, ctx := setupKeeper(t)

	recipient := sdk.AccAddress("recipient1__________").String()

	k.CreateVestingSchedule(ctx, "claim-dup", "fact-1", recipient,
		"1000000000000000000", types.CategoryFormalProof, types.SourceVerification)

	_, err := k.FalsifyClaim(ctx, "claim-dup", "challenger")
	if err != nil {
		t.Fatalf("first falsify failed: %v", err)
	}

	_, err = k.FalsifyClaim(ctx, "claim-dup", "challenger")
	if err == nil {
		t.Fatal("expected error for already falsified claim")
	}
}

// ---------- Full Lifecycle Test ----------

func TestVestingFullLifecycle(t *testing.T) {
	ms, k, ctx := setupMsgServer(t)

	recipient := sdk.AccAddress("recipient1__________").String()

	createResp, err := ms.CreateVesting(ctx, &types.MsgCreateVesting{
		Authority:    "authority",
		Beneficiary:  recipient,
		Amount:       "10000000000000000000",
		Category:     types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD,
		LinkedFactId: "fact-lifecycle",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	ms.AccelerateVesting(ctx, &types.MsgAccelerateVesting{
		Authority: "authority", VestingId: createResp.VestingId,
		AccelerationFactor: 1000000,
	})

	ctx = ctx.WithBlockHeight(2000)
	ms.PauseVesting(ctx, &types.MsgPauseVesting{
		Authority: "authority", VestingId: createResp.VestingId, Reason: "dispute",
	})

	schedule, _ := k.GetVestingSchedule(ctx, createResp.VestingId)
	if schedule.Status != string(types.VestingStatusPaused) {
		t.Fatalf("expected paused, got %s", schedule.Status)
	}

	ctx = ctx.WithBlockHeight(3000)
	ms.ResumeVesting(ctx, &types.MsgResumeVesting{
		Authority: "authority", VestingId: createResp.VestingId,
	})

	schedule, _ = k.GetVestingSchedule(ctx, createResp.VestingId)
	if schedule.Status != string(types.VestingStatusActive) {
		t.Fatalf("expected active after resume, got %s", schedule.Status)
	}
	if schedule.TotalPausedBlocks != 1000 {
		t.Errorf("expected 1000 paused blocks, got %d", schedule.TotalPausedBlocks)
	}

	ms.AccelerateVesting(ctx, &types.MsgAccelerateVesting{
		Authority: "authority", VestingId: createResp.VestingId,
		AccelerationFactor: 600000,
	})

	schedule, _ = k.GetVestingSchedule(ctx, createResp.VestingId)
	if schedule.DefenseCount != 1 || schedule.ReplicationCount != 1 {
		t.Errorf("expected 1/1 defense/replication, got %d/%d", schedule.DefenseCount, schedule.ReplicationCount)
	}

	_, err = ms.CompleteVesting(ctx, &types.MsgCompleteVesting{
		Authority: recipient, VestingId: createResp.VestingId,
	})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	schedule, _ = k.GetVestingSchedule(ctx, createResp.VestingId)
	if schedule.Status != string(types.VestingStatusCompleted) {
		t.Errorf("expected completed, got %s", schedule.Status)
	}
}

// ==================== Pure PoT Mint Tests ====================

func setupMintKeeper(t *testing.T, bk *mockBankKeeper, totalMinted string, blockHeight int64) (keeper.Keeper, sdk.Context) {
	t.Helper()
	gs := types.DefaultGenesis()
	gs.Params.InitialFundBalance = totalMinted

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	sk := &mockStakingKeeper{activeCount: 22}

	k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), bk, sk, "authority")
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: blockHeight}, false, log.NewNopLogger())

	k.InitGenesis(ctx, gs)

	if totalMinted != "" && totalMinted != "0" {
		amt, ok := new(big.Int).SetString(totalMinted, 10)
		if ok && amt.Sign() > 0 {
			bk.supply["uzrn"] = sdkmath.NewIntFromBigInt(amt)
		}
	}

	return k, ctx
}

func TestBlockReward_Epoch0(t *testing.T) {
	bk := newMockBankKeeper()
	k, ctx := setupMintKeeper(t, bk, "0", 0)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// At epoch 0, no decay. Reward = 10000000 (10 ZRN).
	if dist.TotalMinted != "10000000" {
		t.Errorf("expected full reward 10000000 at epoch 0, got %s", dist.TotalMinted)
	}
	if bk.mintedCoins.AmountOf("uzrn").Int64() != 10000000 {
		t.Errorf("expected 10000000 uzrn minted, got %s", bk.mintedCoins.AmountOf("uzrn").String())
	}
}

func TestBlockReward_Epoch1(t *testing.T) {
	bk := newMockBankKeeper()
	// Block 100000 = start of epoch 1
	k, ctx := setupMintKeeper(t, bk, "0", 100000)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// At epoch 1: 10000000 * 0.994478 = 9944780
	if dist.TotalMinted != "9944780" {
		t.Errorf("expected decayed reward 9944780 at epoch 1, got %s", dist.TotalMinted)
	}
}

func TestBlockReward_Epoch10(t *testing.T) {
	bk := newMockBankKeeper()
	// Block 1000000 = start of epoch 10
	k, ctx := setupMintKeeper(t, bk, "0", 1000000)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalMinted := dist.TotalMinted
	if totalMinted == "0" || totalMinted == "10000000" {
		t.Errorf("expected decayed reward at epoch 10, got %s", totalMinted)
	}
	// 10000000 * 0.994478^10 ≈ 9461280
	minted := new(big.Int)
	minted.SetString(totalMinted, 10)
	if minted.Cmp(big.NewInt(9400000)) < 0 || minted.Cmp(big.NewInt(9500000)) > 0 {
		t.Errorf("epoch 10 reward %s outside expected range [9400000, 9500000]", totalMinted)
	}
}

func TestBlockReward_FloorReward(t *testing.T) {
	bk := newMockBankKeeper()
	// With 1-year half-life (decay_bps=994478), floor reached at ~epoch 832 (~year 6.6)
	k, ctx := setupMintKeeper(t, bk, "0", 1000*100000)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dist.TotalMinted != "100000" {
		t.Errorf("expected floor reward 100000, got %s", dist.TotalMinted)
	}
}

func TestMintWithCap_SupplyExhausted(t *testing.T) {
	bk := newMockBankKeeper()
	maxSupply := "222222222000000"
	// Set supply to exactly maxSupply so remaining = 0 from the start.
	k, ctx := setupMintKeeper(t, bk, maxSupply, 0)

	producer := sdk.AccAddress("producer____________").String()

	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist.TotalMinted != "0" {
		t.Errorf("expected 0 reward when supply exhausted, got %s", dist.TotalMinted)
	}
}

func TestMintWithCap_EnforcesSupplyLimit(t *testing.T) {
	bk := newMockBankKeeper()
	k, ctx := setupMintKeeper(t, bk, "0", 0)

	maxSupply := new(big.Int)
	maxSupply.SetString(types.MaxSupplyUzrn, 10)
	overMax := new(big.Int).Add(maxSupply, big.NewInt(1))

	actual, err := k.MintWithCap(ctx, overMax)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if actual.Cmp(maxSupply) != 0 {
		t.Errorf("expected mint capped to %s, got %s", maxSupply.String(), actual.String())
	}

	totalMinted := k.GetTotalMinted(ctx)
	if totalMinted.Cmp(maxSupply) != 0 {
		t.Errorf("expected total minted %s, got %s", maxSupply.String(), totalMinted.String())
	}

	actual2, err := k.MintWithCap(ctx, big.NewInt(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual2.Sign() != 0 {
		t.Errorf("expected 0 mint when supply exhausted, got %s", actual2.String())
	}
}

func TestMintWithCap_SupplyMonotonic(t *testing.T) {
	// No burn recycling in the new model: supply monotonically increases to cap.
	// Once the cap is reached, no further minting is possible.
	bk := newMockBankKeeper()
	nearCap := "222222221999500"
	k, ctx := setupMintKeeper(t, bk, nearCap, 0)

	// Mint exactly the remaining 500
	actual, err := k.MintWithCap(ctx, big.NewInt(500))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Int64() != 500 {
		t.Errorf("expected 500 minted, got %s", actual.String())
	}

	// Supply exhausted — no more minting possible
	actual2, err := k.MintWithCap(ctx, big.NewInt(1000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual2.Sign() != 0 {
		t.Errorf("expected 0 mint when supply exhausted, got %s", actual2.String())
	}

	// Repeated attempts also yield zero (no burn recycling to free headroom)
	actual3, err := k.MintWithCap(ctx, big.NewInt(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual3.Sign() != 0 {
		t.Errorf("expected 0 after cap permanently reached, got %s", actual3.String())
	}
}

func TestApplyDecay(t *testing.T) {
	tests := []struct {
		name      string
		epoch     int64
		minReward int64
		maxReward int64
	}{
		{"epoch 0", 0, 10000000, 10000000},
		{"epoch 1", 100000, 9944780, 9944780},
		{"epoch 2", 200000, 9889000, 9890000},
		{"epoch 5", 500000, 9720000, 9750000},
		{"epoch 10", 1000000, 9440000, 9480000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bk := newMockBankKeeper()
			k, ctx := setupMintKeeper(t, bk, "0", tt.epoch)

			producer := sdk.AccAddress("producer____________").String()
			dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			minted := new(big.Int)
			minted.SetString(dist.TotalMinted, 10)
			if minted.Cmp(big.NewInt(tt.minReward)) < 0 || minted.Cmp(big.NewInt(tt.maxReward)) > 0 {
				t.Errorf("epoch reward %s outside expected range [%d, %d]", dist.TotalMinted, tt.minReward, tt.maxReward)
			}
		})
	}
}

func TestTotalMintedGetSet(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetTotalMinted(ctx, big.NewInt(999999))
	total := k.GetTotalMinted(ctx)
	if total.Int64() != 999999 {
		t.Errorf("expected 999999, got %s", total.String())
	}

	k.SetTotalMinted(ctx, big.NewInt(1999999))
	total = k.GetTotalMinted(ctx)
	if total.Int64() != 1999999 {
		t.Errorf("expected 1999999, got %s", total.String())
	}
}

func TestExportGenesis_PreservesTotalMinted(t *testing.T) {
	bk := newMockBankKeeper()
	k, ctx := setupMintKeeper(t, bk, "0", 0)

	producer := sdk.AccAddress("producer____________").String()
	k.DistributeBlockReward(ctx, producer, 22, true)

	exported := k.ExportGenesis(ctx)

	if exported.Params.InitialFundBalance == "0" || exported.Params.InitialFundBalance == "" {
		t.Error("expected non-zero exported total minted after distributing rewards")
	}
	if exported.Params.InitialFundBalance != "10000000" {
		t.Errorf("expected exported total minted 10000000, got %s", exported.Params.InitialFundBalance)
	}
}

// ---------- 4-Way Block Reward Distribution Accounting ----------

func TestDistributeBlockReward_4WayAccounting(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	if dist.TotalMinted != "10000000" {
		t.Errorf("expected total minted 10000000, got %s", dist.TotalMinted)
	}

	// Contributor (55%): 10000000 * 550000 / 1000000 = 5500000
	if dist.ProducerReward != "5500000" {
		t.Errorf("expected producer reward 5500000, got %s", dist.ProducerReward)
	}

	// Research (3.33%): 10000000 * 33300 / 1000000 = 333000
	if dist.ResearchShare != "333000" {
		t.Errorf("expected research share 333000, got %s", dist.ResearchShare)
	}

	// Development (19.67%): remainder = 10000000 - 5500000 - 2200000 - 333000 = 1967000
	if dist.DevelopmentAmount != "1967000" {
		t.Errorf("expected development amount 1967000, got %s", dist.DevelopmentAmount)
	}

	// Protocol (22%): 2200000
	if dist.ProtocolShare != "2200000" {
		t.Errorf("expected protocol share 2200000, got %s", dist.ProtocolShare)
	}

	// Verify bank sends
	producerCoins := bk.sentToAccount[producer]
	if producerCoins.AmountOf("uzrn").Int64() != 5500000 {
		t.Errorf("expected 5500000 to producer, got %d", producerCoins.AmountOf("uzrn").Int64())
	}

	// Development fund receives 1967000 (no burn)
	devCoins := bk.sentToModule["development_fund"]
	if devCoins.AmountOf("uzrn").Int64() != 1967000 {
		t.Errorf("expected 1967000 to development_fund, got %d", devCoins.AmountOf("uzrn").Int64())
	}

	// Verification pool split: protocol 22% = 2200000
	// Verification (30% of protocol): 2200000 * 300000 / 1000000 = 660000
	// Of which: 70% to knowledge, 30% to compute_pool
	// Compute: 660000 * 300000 / 1000000 = 198000
	// Knowledge: 660000 - 198000 = 462000
	knowledgeCoins := bk.sentToModule["knowledge"]
	if knowledgeCoins.AmountOf("uzrn").Int64() != 462000 {
		t.Errorf("expected 462000 to knowledge, got %d", knowledgeCoins.AmountOf("uzrn").Int64())
	}
	computeCoins := bk.sentToModule["compute_pool"]
	if computeCoins.AmountOf("uzrn").Int64() != 198000 {
		t.Errorf("expected 198000 to compute_pool, got %d", computeCoins.AmountOf("uzrn").Int64())
	}

	// Citation pool and treasury stay in the module account (not yet distributed to
	// specific modules). They are not tracked in sentToAccount/sentToModule.
	// Citation: 50% of protocol = 2200000 * 500000 / 1000000 = 1100000
	// Treasury: remainder of protocol = 2200000 - 1100000 - 660000 = 440000
	// These are retained in the vesting_rewards module account.
	retainedCitation := int64(1100000)
	retainedTreasury := int64(440000)

	// Total accounting: distributed + retained = total minted (no burn)
	var totalDistributed int64
	for _, coins := range bk.sentToAccount {
		totalDistributed += coins.AmountOf("uzrn").Int64()
	}
	for _, coins := range bk.sentToModule {
		totalDistributed += coins.AmountOf("uzrn").Int64()
	}
	totalDistributed += retainedCitation + retainedTreasury

	if totalDistributed != 10000000 {
		t.Errorf("ACCOUNTING FAIL: total outflows (%d) != total minted (10000000)", totalDistributed)
	}
}

// ---------- Fee Routing Tests ----------

func TestRouteFees_SweepsFeeCollector(t *testing.T) {
	bk := newMockBankKeeper()
	feeCollectorModuleAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	bk.balances = map[string]sdk.Coins{
		feeCollectorModuleAddr.String(): sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100000))),
	}

	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	err := k.RouteFees(ctx)
	if err != nil {
		t.Fatalf("RouteFees failed: %v", err)
	}

	// Research: 3.33% of 100000 = 3330
	researchCoins := bk.sentToModule["research_fund"]
	if researchCoins.AmountOf("uzrn").Int64() != 3330 {
		t.Errorf("expected 3330 uzrn to research_fund, got %d", researchCoins.AmountOf("uzrn").Int64())
	}

	// Development: 19.67% of 100000 = 19670
	devCoins := bk.sentToModule["development_fund"]
	if devCoins.AmountOf("uzrn").Int64() != 19670 {
		t.Errorf("expected 19670 uzrn to development_fund, got %d", devCoins.AmountOf("uzrn").Int64())
	}

}

func TestRouteFees_EmptyFeeCollector(t *testing.T) {
	bk := newMockBankKeeper()
	bk.balances = map[string]sdk.Coins{}

	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	err := k.RouteFees(ctx)
	if err != nil {
		t.Fatalf("RouteFees failed: %v", err)
	}

	if len(bk.sentToModule) > 0 {
		t.Errorf("expected no module sends, got %v", bk.sentToModule)
	}
}

func TestRouteFees_OnlyProcessesUzrn(t *testing.T) {
	feeCollectorModuleAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	bk := newMockBankKeeper()
	bk.balances = map[string]sdk.Coins{
		feeCollectorModuleAddr.String(): sdk.NewCoins(
			sdk.NewCoin("uzrn", sdkmath.NewInt(100000)),
			sdk.NewCoin("atom", sdkmath.NewInt(50000)),
		),
	}

	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	err := k.RouteFees(ctx)
	if err != nil {
		t.Fatalf("RouteFees failed: %v", err)
	}

	researchCoins := bk.sentToModule["research_fund"]
	if researchCoins.AmountOf("uzrn").Int64() != 3330 {
		t.Errorf("expected 3330 uzrn to research_fund, got %d", researchCoins.AmountOf("uzrn").Int64())
	}
	if researchCoins.AmountOf("atom").Int64() != 0 {
		t.Errorf("expected 0 atom to research_fund, got %d", researchCoins.AmountOf("atom").Int64())
	}
}

// ==================== Founder Auto-Split Tests ====================

func setupFounderKeeper(t *testing.T, bk *mockBankKeeper, founderAddr string, govHeight uint64) (keeper.Keeper, sdk.Context) {
	t.Helper()
	gs := types.DefaultGenesis()
	gs.Params.FounderShareBps = 70000
	gs.Params.FounderAddress = founderAddr
	gs.Params.GovernanceActivationHeight = govHeight
	return setupKeeperWithBankAndGenesis(t, bk, &mockStakingKeeper{activeCount: 22}, gs)
}

func TestFounderAutoSplit(t *testing.T) {
	// Block reward with founder: verify 7% of research goes to founder.
	// Math (epoch 0, full validators):
	//   Total minted:       10,000,000
	//   Contributor 55%:    5,500,000
	//   Protocol 22%:       2,200,000
	//   Research 3.33%:     333,000
	//   Development 19.67%: 1,967,000 (remainder)
	//   Founder (7% of research): 333,000 * 70000 / 1000000 = 23,310
	//   Net research:       333,000 - 23,310 = 309,690
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	if dist.TotalMinted != "10000000" {
		t.Errorf("expected total minted 10000000, got %s", dist.TotalMinted)
	}
	if dist.FounderShare != "23310" {
		t.Errorf("expected founder share 23310, got %s", dist.FounderShare)
	}
	if dist.ResearchShare != "309690" {
		t.Errorf("expected net research share 309690, got %s", dist.ResearchShare)
	}
	if dist.ProducerReward != "5500000" {
		t.Errorf("expected producer reward 5500000, got %s", dist.ProducerReward)
	}

	founderCoins := bk.sentToAccount[founderAddr]
	if founderCoins.AmountOf("uzrn").Int64() != 23310 {
		t.Errorf("expected 23310 uzrn to founder, got %d", founderCoins.AmountOf("uzrn").Int64())
	}
	researchCoins := bk.sentToModule["research_fund"]
	if researchCoins.AmountOf("uzrn").Int64() != 309690 {
		t.Errorf("expected 309690 uzrn to research_fund, got %d", researchCoins.AmountOf("uzrn").Int64())
	}
}

func TestFounderSplitDisabled(t *testing.T) {
	bk := newMockBankKeeper()
	k, ctx := setupFounderKeeper(t, bk, "", 0)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	// Research 3.33% = 333000 (full, no founder deduction)
	if dist.ResearchShare != "333000" {
		t.Errorf("expected full research share 333000, got %s", dist.ResearchShare)
	}
	if dist.FounderShare != "0" {
		t.Errorf("expected founder share 0 when disabled, got %s", dist.FounderShare)
	}
}

func TestFounderSplitPermanent(t *testing.T) {
	// Governance sunset was removed — founder share is permanent and governance-immune.
	// Even with GovernanceActivationHeight set, founder remains active.
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 500)

	producer := sdk.AccAddress("producer____________").String()
	dist, err := k.DistributeBlockReward(ctx, producer, 22, true)
	if err != nil {
		t.Fatalf("distribute block reward failed: %v", err)
	}

	// Founder share is permanent: 333000 * 70000 / 1000000 = 23310
	if dist.FounderShare != "23310" {
		t.Errorf("expected founder share 23310 (permanent), got %s", dist.FounderShare)
	}
	// Net research: 333000 - 23310 = 309690
	if dist.ResearchShare != "309690" {
		t.Errorf("expected net research share 309690, got %s", dist.ResearchShare)
	}
}

// ==================== DepositToResearchFund Tests ====================

func TestDepositToResearchFund_BasicSplit(t *testing.T) {
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	depositCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100000)))
	err := k.DepositToResearchFund(ctx, "knowledge", depositCoins)
	if err != nil {
		t.Fatalf("DepositToResearchFund failed: %v", err)
	}

	researchCoins := bk.sentToModule["research_fund"]
	if researchCoins.AmountOf("uzrn").Int64() != 93000 {
		t.Errorf("expected 93000 to research_fund, got %d", researchCoins.AmountOf("uzrn").Int64())
	}

	founderCoins := bk.sentToAccount[founderAddr]
	if founderCoins.AmountOf("uzrn").Int64() != 7000 {
		t.Errorf("expected 7000 to founder, got %d", founderCoins.AmountOf("uzrn").Int64())
	}
}

func TestDepositToResearchFund_NoFounderAddress(t *testing.T) {
	bk := newMockBankKeeper()
	k, ctx := setupFounderKeeper(t, bk, "", 0)

	depositCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100000)))
	err := k.DepositToResearchFund(ctx, "knowledge", depositCoins)
	if err != nil {
		t.Fatalf("DepositToResearchFund failed: %v", err)
	}

	researchCoins := bk.sentToModule["research_fund"]
	if researchCoins.AmountOf("uzrn").Int64() != 100000 {
		t.Errorf("expected 100000 to research_fund (no founder), got %d", researchCoins.AmountOf("uzrn").Int64())
	}

	if len(bk.sentToAccount) > 0 {
		t.Errorf("expected no account sends with empty founder, got %v", bk.sentToAccount)
	}
}

func TestDepositToResearchFund_FounderPermanent(t *testing.T) {
	// Governance sunset removed — founder share is permanent.
	// Even with GovernanceActivationHeight set, founder split still applies.
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 500)

	depositCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100000)))
	err := k.DepositToResearchFund(ctx, "knowledge", depositCoins)
	if err != nil {
		t.Fatalf("DepositToResearchFund failed: %v", err)
	}

	// 7% founder: 100000 * 70000 / 1000000 = 7000; research: 93000
	researchCoins := bk.sentToModule["research_fund"]
	if researchCoins.AmountOf("uzrn").Int64() != 93000 {
		t.Errorf("expected 93000 to research_fund (founder permanent), got %d", researchCoins.AmountOf("uzrn").Int64())
	}

	founderCoins := bk.sentToAccount[founderAddr]
	if founderCoins.AmountOf("uzrn").Int64() != 7000 {
		t.Errorf("expected 7000 to founder (permanent), got %d", founderCoins.AmountOf("uzrn").Int64())
	}
}

func TestDepositToResearchFund_ZeroAmount(t *testing.T) {
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	err := k.DepositToResearchFund(ctx, "knowledge", sdk.Coins{})
	if err != nil {
		t.Fatalf("DepositToResearchFund with zero amount failed: %v", err)
	}

	if len(bk.sentToModule) > 0 {
		t.Errorf("expected no module sends for zero amount, got %v", bk.sentToModule)
	}
}

func TestDepositToResearchFund_EmitsEvent(t *testing.T) {
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	depositCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100000)))
	err := k.DepositToResearchFund(ctx, "billing", depositCoins)
	if err != nil {
		t.Fatalf("DepositToResearchFund failed: %v", err)
	}

	events := ctx.EventManager().Events()
	found := false
	for _, event := range events {
		if event.Type == "zerone.vesting_rewards.research_fund_deposit" {
			found = true
			attrs := make(map[string]string)
			for _, attr := range event.Attributes {
				attrs[attr.Key] = attr.Value
			}
			if attrs["source_module"] != "billing" {
				t.Errorf("expected source_module=billing, got %s", attrs["source_module"])
			}
			if attrs["total"] != "100000" {
				t.Errorf("expected total=100000, got %s", attrs["total"])
			}
			if attrs["research"] != "93000" {
				t.Errorf("expected research=93000, got %s", attrs["research"])
			}
			if attrs["founder"] != "7000" {
				t.Errorf("expected founder=7000, got %s", attrs["founder"])
			}
		}
	}
	if !found {
		t.Error("expected research_fund_deposit event to be emitted")
	}
}

// ==================== Founder Share Immutability Tests ====================

func TestFounderShareImmutability_RejectBpsChange(t *testing.T) {
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	ms := keeper.NewMsgServerImpl(k)

	// Current: FounderShareBps=70000. Attempt to change to 50000.
	newParams := types.DefaultParams()
	newParams.FounderShareBps = 50000
	newParams.FounderAddress = founderAddr

	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    newParams,
	})
	if err == nil {
		t.Fatal("expected error when changing FounderShareBps")
	}
	if err != types.ErrFounderShareImmutable {
		t.Errorf("expected ErrFounderShareImmutable, got %v", err)
	}
}

func TestFounderShareImmutability_RejectAddressChange(t *testing.T) {
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	ms := keeper.NewMsgServerImpl(k)

	// Current: FounderAddress is set. Attempt to change to a different address.
	newParams := types.DefaultParams()
	newParams.FounderShareBps = 70000
	newParams.FounderAddress = sdk.AccAddress("another_founder_____").String()

	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    newParams,
	})
	if err == nil {
		t.Fatal("expected error when changing FounderAddress")
	}
	if err != types.ErrFounderShareImmutable {
		t.Errorf("expected ErrFounderShareImmutable, got %v", err)
	}
}

func TestFounderShareImmutability_AllowInitialSet(t *testing.T) {
	// When founder params are empty/zero, setting them for the first time should succeed.
	bk := newMockBankKeeper()
	k, ctx := setupFounderKeeper(t, bk, "", 0) // empty founder

	ms := keeper.NewMsgServerImpl(k)

	newParams := types.DefaultParams()
	newParams.FounderShareBps = 70000
	newParams.FounderAddress = sdk.AccAddress("new_founder_________").String()

	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("expected initial set to succeed, got: %v", err)
	}

	// Verify params were set
	params := k.GetParams(ctx)
	if params.FounderShareBps != 70000 {
		t.Errorf("expected FounderShareBps 70000, got %d", params.FounderShareBps)
	}
	if params.FounderAddress != newParams.FounderAddress {
		t.Errorf("expected FounderAddress %s, got %s", newParams.FounderAddress, params.FounderAddress)
	}
}

func TestFounderShareImmutability_AllowIdenticalValues(t *testing.T) {
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	ms := keeper.NewMsgServerImpl(k)

	// Setting the same values should succeed (no change = no violation).
	newParams := types.DefaultParams()
	newParams.FounderShareBps = 70000
	newParams.FounderAddress = founderAddr

	_, err := ms.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: "authority",
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("expected identical values to succeed, got: %v", err)
	}
}

func TestNoGovernanceSunset(t *testing.T) {
	// Governance sunset was removed. isFounderShareActive returns true regardless
	// of block height when founder address is set.
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()

	// Set GovernanceActivationHeight to 500 — should be ignored.
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 500)

	// Test at block 1 (before activation height)
	ctx1 := ctx.WithBlockHeight(1)
	producer := sdk.AccAddress("producer____________").String()
	dist1, err := k.DistributeBlockReward(ctx1, producer, 22, true)
	if err != nil {
		t.Fatalf("block 1 reward failed: %v", err)
	}
	if dist1.FounderShare == "0" {
		t.Error("expected founder share active at block 1 (no sunset)")
	}

	// Test at block 10000 (well after activation height)
	bk2 := newMockBankKeeper()
	k2, ctx2 := setupFounderKeeper(t, bk2, founderAddr, 500)
	ctx2 = ctx2.WithBlockHeight(10000)
	dist2, err := k2.DistributeBlockReward(ctx2, producer, 22, true)
	if err != nil {
		t.Fatalf("block 10000 reward failed: %v", err)
	}
	if dist2.FounderShare == "0" {
		t.Error("expected founder share active at block 10000 (no sunset)")
	}

	// Both should yield the same founder share amount
	if dist1.FounderShare != dist2.FounderShare {
		t.Errorf("founder share should be consistent regardless of block height: block1=%s, block10000=%s",
			dist1.FounderShare, dist2.FounderShare)
	}
}

// ==================== New Query Tests ====================

func TestQueryResearchFundBalance(t *testing.T) {
	bk := newMockBankKeeper()
	sk := &mockStakingKeeper{activeCount: 22}
	k, ctx := setupKeeperWithBank(t, bk, sk)

	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.ResearchFundBalance(ctx, &types.QueryResearchFundBalanceRequest{})
	if err != nil {
		t.Fatalf("ResearchFundBalance query failed: %v", err)
	}
	if resp.Balance != "0" {
		t.Errorf("expected balance 0, got %s", resp.Balance)
	}
	if resp.Denom != "uzrn" {
		t.Errorf("expected denom uzrn, got %s", resp.Denom)
	}
}

func TestQueryFounderShareStatus_Active(t *testing.T) {
	bk := newMockBankKeeper()
	founderAddr := sdk.AccAddress("founder_____________").String()
	k, ctx := setupFounderKeeper(t, bk, founderAddr, 0)

	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.FounderShareStatus(ctx, &types.QueryFounderShareStatusRequest{})
	if err != nil {
		t.Fatalf("FounderShareStatus query failed: %v", err)
	}
	if !resp.Active {
		t.Error("expected founder share to be active")
	}
	if resp.FounderShareBps != 70000 {
		t.Errorf("expected 70000 bps, got %d", resp.FounderShareBps)
	}
	if resp.FounderAddress != founderAddr {
		t.Errorf("expected founder address %s, got %s", founderAddr, resp.FounderAddress)
	}
}

func TestQueryFounderShareStatus_Inactive(t *testing.T) {
	bk := newMockBankKeeper()
	k, ctx := setupFounderKeeper(t, bk, "", 0)

	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.FounderShareStatus(ctx, &types.QueryFounderShareStatusRequest{})
	if err != nil {
		t.Fatalf("FounderShareStatus query failed: %v", err)
	}
	if resp.Active {
		t.Error("expected founder share to be inactive with empty address")
	}
}

// ==================== Custom RevenueSplit Params Test ====================

func TestDistributeRevenue_CustomSplit(t *testing.T) {
	// Test with a custom governance-adjusted split: 40/30/20/10
	gs := types.DefaultGenesis()
	gs.Params.RevenueSplit = &commontypes.RevenueSplit{
		ContributorBps: 400000,
		ProtocolBps:    300000,
		ResearchBps:    200000,
		DevelopmentBps:        100000,
	}
	gs.Params.ProtocolSubSplit = &commontypes.ProtocolSubSplit{
		CitationBps:     600000,
		VerificationBps: 300000,
		TreasuryBps:     100000,
	}

	bk := newMockBankKeeper()
	k, ctx := setupKeeperWithBankAndGenesis(t, bk, &mockStakingKeeper{activeCount: 22}, gs)

	routing, err := k.DistributeRevenue(ctx, types.SourceBlockProduction, "1000000",
		sdk.AccAddress("recipient___________").String(), "")
	if err != nil {
		t.Fatalf("distribute revenue failed: %v", err)
	}

	if routing.ContributorShare != "400000" {
		t.Errorf("expected contributor 400000, got %s", routing.ContributorShare)
	}
	if routing.ProtocolShare != "300000" {
		t.Errorf("expected protocol 300000, got %s", routing.ProtocolShare)
	}
	if routing.ResearchShare != "200000" {
		t.Errorf("expected research 200000, got %s", routing.ResearchShare)
	}
	if routing.DevelopmentAmount != "100000" {
		t.Errorf("expected development 100000, got %s", routing.DevelopmentAmount)
	}
	if routing.CitationPool != "180000" {
		t.Errorf("expected citation pool 180000, got %s", routing.CitationPool)
	}
	if routing.VerificationPool != "90000" {
		t.Errorf("expected verification pool 90000, got %s", routing.VerificationPool)
	}
	if routing.TreasuryShare != "30000" {
		t.Errorf("expected treasury 30000, got %s", routing.TreasuryShare)
	}
}
