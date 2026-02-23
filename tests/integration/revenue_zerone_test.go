package integration_test

import (
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

	toolboxkeeper "github.com/zerone-chain/zerone/x/toolbox/keeper"
	toolboxtypes "github.com/zerone-chain/zerone/x/toolbox/types"
	treekeeper "github.com/zerone-chain/zerone/x/tree/keeper"
	treetypes "github.com/zerone-chain/zerone/x/tree/types"
	vestingkeeper "github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
	vestingtypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// ---------- Test 11: Toolbox Revenue Cascade ----------

func TestToolboxRevenueCascade(t *testing.T) {
	bk := newMockBankKeeper()

	// Create vesting keeper as ResearchFundDepositor
	vestingStoreKey := storetypes.NewKVStoreKey(vestingtypes.StoreKey)
	toolboxStoreKey := storetypes.NewKVStoreKey(toolboxtypes.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(vestingStoreKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(toolboxStoreKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	founderAddr := sdk.AccAddress("founder_____________")

	vk := vestingkeeper.NewKeeper(cdc, runtime.NewKVStoreService(vestingStoreKey), bk, nil, "authority")
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1000}, false, log.NewNopLogger())

	gs := vestingtypes.DefaultGenesis()
	gs.Params.FounderAddress = founderAddr.String()
	vk.InitGenesis(ctx, gs)

	// Toolbox keeper wired to mock bank + vesting keeper as RFD (via adapter)
	rfdAdapter := vestingkeeper.NewResearchFundDepositorAdapter(vk)
	tk := toolboxkeeper.NewKeeper(runtime.NewKVStoreService(toolboxStoreKey), cdc, "authority", bk, rfdAdapter)
	tk.InitGenesis(ctx, toolboxtypes.DefaultGenesis())

	// Create contributor addresses
	contrib1Addr := sdk.AccAddress("contrib1____________")
	contrib2Addr := sdk.AccAddress("contrib2____________")
	deployerAddr := sdk.AccAddress("deployer____________")

	// Create tool with 2 accepted contributors (50/50)
	tool := &toolboxtypes.Tool{
		Id:       "tool-1",
		Deployer: deployerAddr.String(),
		Contributors: []*toolboxtypes.ContributorShare{
			{Address: contrib1Addr.String(), ShareBps: 500000, Accepted: true, TotalEarned: "0"},
			{Address: contrib2Addr.String(), ShareBps: 500000, Accepted: true, TotalEarned: "0"},
		},
	}

	total := sdk.NewCoin("uzrn", sdkmath.NewInt(1000000))

	err := tk.DistributeRevenue(ctx, tool, total)
	if err != nil {
		t.Fatalf("toolbox DistributeRevenue failed: %v", err)
	}

	// Default toolbox params: ToolRevenueBps=550000, ProtocolBps=220000, ResearchBps=130000, DevelopmentBps=100000
	totalAmt := uint64(1000000)

	// Contributor share: 55% of 1M = 550,000
	// Each contributor (50/50): 275,000
	expectedContrib := totalAmt * 550000 / 1000000 / 2
	contrib1Got := bk.totalSentToAddr(contrib1Addr.String())
	if !contrib1Got.Equal(sdkmath.NewInt(int64(expectedContrib))) {
		t.Errorf("contrib1 got %s, want %d", contrib1Got, expectedContrib)
	}
	contrib2Got := bk.totalSentToAddr(contrib2Addr.String())
	if !contrib2Got.Equal(sdkmath.NewInt(int64(expectedContrib))) {
		t.Errorf("contrib2 got %s, want %d", contrib2Got, expectedContrib)
	}

	// Research fund: 13% of 1M = 130,000 → DepositToResearchFund (with 7% founder split)
	researchTotal := totalAmt * 130000 / 1000000 // 130,000
	expectedFounder := researchTotal * 70000 / 1000000
	expectedResearchNet := researchTotal - expectedFounder
	researchGot := bk.totalSentToModule("research_fund")
	if !researchGot.Equal(sdkmath.NewInt(int64(expectedResearchNet))) {
		t.Errorf("research_fund got %s, want %d", researchGot, expectedResearchNet)
	}
	founderGot := bk.totalSentToAddr(founderAddr.String())
	if !founderGot.Equal(sdkmath.NewInt(int64(expectedFounder))) {
		t.Errorf("founder got %s, want %d", founderGot, expectedFounder)
	}

	// Burn: 10% of 1M = 100,000
	expectedBurn := totalAmt * 100000 / 1000000
	burnGot := bk.totalBurned()
	if !burnGot.Equal(sdkmath.NewInt(int64(expectedBurn))) {
		t.Errorf("burn got %s, want %d", burnGot, expectedBurn)
	}

	// Protocol: 22% of 1M = 220,000
	// Protocol sub-split: citation 50%=110,000, verification 30%=66,000, treasury 20%=44,000
	protocolAmt := totalAmt * 220000 / 1000000
	citationAmt := protocolAmt * 500000 / 1000000
	verificationAmt := protocolAmt * 300000 / 1000000
	treasuryAmt := protocolAmt - citationAmt - verificationAmt

	citationGot := bk.totalSentToModule("knowledge")
	if !citationGot.Equal(sdkmath.NewInt(int64(citationAmt))) {
		t.Errorf("citation pool got %s, want %d", citationGot, citationAmt)
	}

	// vesting_rewards receives BOTH verification pool (66k) AND research escrow (130k)
	// because DepositToResearchFund escrows from toolbox→vesting_rewards first.
	expectedVestingTotal := verificationAmt + researchTotal
	verificationGot := bk.totalSentToModule("vesting_rewards")
	if !verificationGot.Equal(sdkmath.NewInt(int64(expectedVestingTotal))) {
		t.Errorf("vesting_rewards got %s, want %d (verification %d + research escrow %d)",
			verificationGot, expectedVestingTotal, verificationAmt, researchTotal)
	}

	treasuryGot := bk.totalSentToModule("protocol_treasury")
	if !treasuryGot.Equal(sdkmath.NewInt(int64(treasuryAmt))) {
		t.Errorf("protocol treasury got %s, want %d", treasuryGot, treasuryAmt)
	}

	// Conservation invariant: outflows from toolbox module == total.
	// The toolbox module sends: contributors + citation + vesting_rewards (verification+research escrow) + treasury + burn
	// The vesting_rewards module then forwards research to research_fund+founder, but those are internal.
	toolboxOut := contrib1Got.Add(contrib2Got).
		Add(citationGot).
		Add(verificationGot).
		Add(treasuryGot).
		Add(burnGot)
	if !toolboxOut.Equal(sdkmath.NewInt(int64(totalAmt))) {
		t.Errorf("conservation violated: toolbox outflows %s != total %d (dust: %s)",
			toolboxOut, totalAmt, sdkmath.NewInt(int64(totalAmt)).Sub(toolboxOut))
	}
}

// ---------- Test 12: Tree Revenue Routing ----------

func TestTreeRevenueRouting(t *testing.T) {
	total := int64(1000000)

	t.Run("proportional_split", func(t *testing.T) {
		dist := treekeeper.CalculateRevenue(
			total,
			550000, 100000, 130000, 100000,
			[]*treetypes.ContributorRecord{
				{Did: "did:zrn:alice", TasksCompleted: 10},
				{Did: "did:zrn:bob", TasksCompleted: 20},
				{Did: "did:zrn:carol", TasksCompleted: 10},
			},
		)

		// Total tasks = 40; alice=10/40=25%, bob=20/40=50%, carol=10/40=25%
		contribPool := dist.ContributorPool // 550,000
		if contribPool != 550000 {
			t.Errorf("contributor pool: got %d, want 550000", contribPool)
		}

		// alice and carol should get equal shares; bob should get double
		var aliceAmt, bobAmt, carolAmt int64
		for _, cs := range dist.ContributorShares {
			switch cs.Address {
			case "did:zrn:alice":
				aliceAmt = cs.Amount
			case "did:zrn:bob":
				bobAmt = cs.Amount
			case "did:zrn:carol":
				carolAmt = cs.Amount
			}
		}
		if aliceAmt != carolAmt {
			t.Errorf("alice %d != carol %d (expected equal)", aliceAmt, carolAmt)
		}
		// Bob should get ~2x alice (may vary by 1 due to rounding)
		ratio := float64(bobAmt) / float64(aliceAmt)
		if ratio < 1.9 || ratio > 2.1 {
			t.Errorf("bob/alice ratio: got %.2f, want ~2.0", ratio)
		}

		shareSum := aliceAmt + bobAmt + carolAmt
		if shareSum != contribPool {
			t.Errorf("contributor shares sum %d != pool %d", shareSum, contribPool)
		}
	})

	t.Run("no_contributors_redirect", func(t *testing.T) {
		dist := treekeeper.CalculateRevenue(
			total,
			550000, 100000, 130000, 100000,
			[]*treetypes.ContributorRecord{},
		)

		// With no contributors, contributor pool is redirected to treasury
		if dist.ContributorPool != 0 {
			t.Errorf("contributor pool should be 0, got %d", dist.ContributorPool)
		}

		// Verify all sums to total
		sum := dist.ContributorPool + dist.ResearchFund + dist.ProtocolTreasury + dist.VerificationPool + dist.Burn
		if sum != total {
			t.Errorf("sum %d != total %d", sum, total)
		}
	})

	t.Run("zero_tasks_equal_split", func(t *testing.T) {
		dist := treekeeper.CalculateRevenue(
			total,
			550000, 100000, 130000, 100000,
			[]*treetypes.ContributorRecord{
				{Did: "did:zrn:x", TasksCompleted: 0},
				{Did: "did:zrn:y", TasksCompleted: 0},
				{Did: "did:zrn:z", TasksCompleted: 0},
			},
		)

		// With zero tasks, each gets equal share
		contribPool := dist.ContributorPool
		perContrib := contribPool / 3
		for _, cs := range dist.ContributorShares {
			// Last contributor gets remainder (no dust), others get perContrib
			if cs.Amount < perContrib-1 || cs.Amount > perContrib+1 {
				t.Errorf("contributor %s got %d, expected ~%d", cs.Address, cs.Amount, perContrib)
			}
		}

		shareSum := int64(0)
		for _, cs := range dist.ContributorShares {
			shareSum += cs.Amount
		}
		if shareSum != contribPool {
			t.Errorf("shares sum %d != pool %d", shareSum, contribPool)
		}
	})

	t.Run("conservation_invariant", func(t *testing.T) {
		for _, amount := range []int64{1, 100, 999999, 1000000, 7777777} {
			dist := treekeeper.CalculateRevenue(
				amount,
				550000, 100000, 130000, 100000,
				[]*treetypes.ContributorRecord{
					{Did: "did:zrn:solo", TasksCompleted: 5},
				},
			)
			sum := dist.ContributorPool + dist.ResearchFund + dist.ProtocolTreasury + dist.VerificationPool + dist.Burn
			if sum != amount {
				t.Errorf("conservation violated for amount %d: sum %d", amount, sum)
			}
		}
	})
}

// ---------- Test 13: Validator Participation Scales Reward ----------

func TestValidatorParticipationScalesReward(t *testing.T) {
	tests := []struct {
		name             string
		activeValidators uint32
		hasTransactions  bool
		expectedMin      int64
		expectedMax      int64
	}{
		{
			name:             "full_validators",
			activeValidators: 22,
			hasTransactions:  true,
			expectedMin:      10000000,
			expectedMax:      10000000,
		},
		{
			name:             "half_validators",
			activeValidators: 11,
			hasTransactions:  true,
			expectedMin:      5000000,
			expectedMax:      5000000,
		},
		{
			name:             "one_validator",
			activeValidators: 1,
			hasTransactions:  true,
			expectedMin:      454545,
			expectedMax:      454546,
		},
		{
			name:             "over_target",
			activeValidators: 30,
			hasTransactions:  true,
			expectedMin:      10000000,
			expectedMax:      10000000,
		},
		{
			name:             "empty_block_zero_reward",
			activeValidators: 22,
			hasTransactions:  false,
			expectedMin:      0,
			expectedMax:      0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bk := newMockBankKeeper()

			vestingStoreKey := storetypes.NewKVStoreKey(vestingtypes.StoreKey)
			db := dbm.NewMemDB()
			stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
			stateStore.MountStoreWithDB(vestingStoreKey, storetypes.StoreTypeIAVL, db)
			if err := stateStore.LoadLatestVersion(); err != nil {
				t.Fatalf("failed to load: %v", err)
			}
			registry := codectypes.NewInterfaceRegistry()
			cdc := codec.NewProtoCodec(registry)

			sk := &mockStakingKeeper{activeCount: tc.activeValidators}
			vk := vestingkeeper.NewKeeper(cdc, runtime.NewKVStoreService(vestingStoreKey), bk, sk, "authority")
			ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1000}, false, log.NewNopLogger())

			gs := vestingtypes.DefaultGenesis()
			gs.Params.FounderAddress = ""
			vk.InitGenesis(ctx, gs)

			producerAddr := sdk.AccAddress("producer____________")
			dist, err := vk.DistributeBlockReward(ctx, producerAddr.String(), tc.activeValidators, tc.hasTransactions)
			if err != nil {
				t.Fatalf("DistributeBlockReward failed: %v", err)
			}

			totalMinted := new(big.Int)
			totalMinted.SetString(dist.TotalMinted, 10)

			if totalMinted.Int64() < tc.expectedMin || totalMinted.Int64() > tc.expectedMax {
				t.Errorf("total minted %s outside expected [%d, %d]",
					totalMinted, tc.expectedMin, tc.expectedMax)
			}
		})
	}
}

// ---------- Test 14: Research Fund Deposit and Disburse ----------

func TestResearchFundDepositAndDisburse(t *testing.T) {
	bk := newMockBankKeeper()

	vestingStoreKey := storetypes.NewKVStoreKey(vestingtypes.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(vestingStoreKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	founderAddr := sdk.AccAddress("founder_____________")
	recipientAddr := sdk.AccAddress("recipient___________")

	vk := vestingkeeper.NewKeeper(cdc, runtime.NewKVStoreService(vestingStoreKey), bk, nil, "authority")
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1000}, false, log.NewNopLogger())

	gs := vestingtypes.DefaultGenesis()
	gs.Params.FounderAddress = founderAddr.String()
	vk.InitGenesis(ctx, gs)

	// Step 1: Deposit 1,000,000 uzrn to research fund
	depositCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(1000000)))
	err := vk.DepositToResearchFund(ctx, "billing", depositCoins)
	if err != nil {
		t.Fatalf("deposit failed: %v", err)
	}

	// Verify 7% founder split on deposit
	expectedFounderDeposit := sdkmath.NewInt(70000)
	expectedResearchDeposit := sdkmath.NewInt(930000)

	founderGot := bk.totalSentToAddr(founderAddr.String())
	if !founderGot.Equal(expectedFounderDeposit) {
		t.Errorf("founder got %s on deposit, want %s", founderGot, expectedFounderDeposit)
	}

	researchGot := bk.totalSentToModule("research_fund")
	if !researchGot.Equal(expectedResearchDeposit) {
		t.Errorf("research_fund got %s on deposit, want %s", researchGot, expectedResearchDeposit)
	}

	// Step 2: Disburse from research fund — NO second founder split
	bk.sentToAccount = make(map[string]sdk.Coins) // reset to isolate disburse
	disburseCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(500000)))
	err = vk.DisburseFromResearchFund(ctx, recipientAddr, disburseCoins)
	if err != nil {
		t.Fatalf("disburse failed: %v", err)
	}

	// Recipient should get full amount (no founder split on disburse path)
	recipientGot := bk.totalSentToAddr(recipientAddr.String())
	if !recipientGot.Equal(sdkmath.NewInt(500000)) {
		t.Errorf("recipient got %s, want 500000", recipientGot)
	}

	// Founder should get ZERO on disburse (no double-taxation)
	founderDisburse := bk.totalSentToAddr(founderAddr.String())
	if founderDisburse.IsPositive() {
		t.Errorf("founder got %s on disburse — should be zero (no double-taxation)", founderDisburse)
	}
}

// ---------- Test 15: Fee Router Split ----------

func TestFeeRouterSplit(t *testing.T) {
	bk := newMockBankKeeper()

	vestingStoreKey := storetypes.NewKVStoreKey(vestingtypes.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(vestingStoreKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	founderAddr := sdk.AccAddress("founder_____________")

	vk := vestingkeeper.NewKeeper(cdc, runtime.NewKVStoreService(vestingStoreKey), bk, nil, "authority")
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 1000}, false, log.NewNopLogger())

	gs := vestingtypes.DefaultGenesis()
	gs.Params.FounderAddress = founderAddr.String()
	vk.InitGenesis(ctx, gs)

	// Seed fee_collector balance using the canonical module address
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	feeBalance := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(1000000)))
	bk.balances[feeCollectorAddr.String()] = feeBalance

	// Call RouteFees
	err := vk.RouteFees(ctx)
	if err != nil {
		t.Fatalf("RouteFees failed: %v", err)
	}

	// RevenueSplit defaults: Research=130000 (13%), Burn=100000 (10%)
	// Research share: 13% of 1M = 130,000 → DepositToResearchFund (with 7% founder split)
	expectedResearchTotal := int64(130000)
	expectedFounderFromFees := expectedResearchTotal * 70000 / 1000000 // 9,100
	expectedResearchNet := expectedResearchTotal - expectedFounderFromFees

	// Verify research fund received net research (after founder split)
	researchGot := bk.totalSentToModule("research_fund")
	if !researchGot.Equal(sdkmath.NewInt(expectedResearchNet)) {
		t.Errorf("research_fund got %s, want %d", researchGot, expectedResearchNet)
	}

	// Verify founder got 7% of research share
	founderGot := bk.totalSentToAddr(founderAddr.String())
	if !founderGot.Equal(sdkmath.NewInt(expectedFounderFromFees)) {
		t.Errorf("founder got %s from fees, want %d", founderGot, expectedFounderFromFees)
	}

	// Verify burn share: 10% of 1M = 100,000
	expectedBurn := int64(100000)
	burnGot := bk.totalBurned()
	if !burnGot.Equal(sdkmath.NewInt(expectedBurn)) {
		t.Errorf("burned %s, want %d", burnGot, expectedBurn)
	}

	// Verify remainder stays in fee_collector for x/distribution
	// RouteFees only extracts research + burn; the rest (77%) stays for validators
	totalExtracted := expectedResearchTotal + expectedBurn // 230,000
	remaining := int64(1000000) - totalExtracted           // 770,000

	// The mock doesn't track fee_collector deductions directly, but we verify
	// the total sent to vesting_rewards module (escrow for research + burn)
	vestingSent := bk.totalSentToModule("vesting_rewards")
	if !vestingSent.Equal(sdkmath.NewInt(totalExtracted)) {
		t.Errorf("vesting_rewards received %s, want %d (research+burn escrow)", vestingSent, totalExtracted)
	}

	_ = remaining // 770,000 stays in fee_collector for validators via x/distribution
}
