package app

import (
	"testing"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	vestingrewardstypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

func TestResearchFundRestriction_BlocksUnauthorizedSpend(t *testing.T) {
	ctx := sdk.Context{}.WithBlockHeight(10)

	billingAddr := authtypes.NewModuleAddress("billing")
	researchAddr := authtypes.NewModuleAddress(ResearchFundName)
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(1000)))

	_, err := ResearchFundRestriction(ctx, billingAddr, researchAddr, coins)
	if err == nil {
		t.Fatal("expected error for unauthorized deposit from billing to research_fund, got nil")
	}

	// Verify error message contains useful information.
	if got := err.Error(); got == "" {
		t.Fatal("error message should not be empty")
	}

	// A random user address should also be blocked.
	userAddr := sdk.AccAddress([]byte("random_user_address_pad"))
	_, err = ResearchFundRestriction(ctx, userAddr, researchAddr, coins)
	if err == nil {
		t.Fatal("expected error for unauthorized deposit from user to research_fund, got nil")
	}

	// The fee_collector module should also be blocked.
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	_, err = ResearchFundRestriction(ctx, feeCollectorAddr, researchAddr, coins)
	if err == nil {
		t.Fatal("expected error for unauthorized deposit from fee_collector to research_fund, got nil")
	}
}

func TestResearchFundRestriction_AllowsGovernanceSpend(t *testing.T) {
	ctx := sdk.Context{}.WithBlockHeight(10)

	// vesting_rewards is the only allowed depositor.
	vestingAddr := authtypes.NewModuleAddress(vestingrewardstypes.ModuleName)
	researchAddr := authtypes.NewModuleAddress(ResearchFundName)
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(1000)))

	retAddr, err := ResearchFundRestriction(ctx, vestingAddr, researchAddr, coins)
	if err != nil {
		t.Fatalf("expected vesting_rewards -> research_fund to be allowed, got error: %v", err)
	}
	if !retAddr.Equals(researchAddr) {
		t.Fatalf("expected returned address to be research_fund, got %s", retAddr.String())
	}
}

func TestResearchFundRestriction_AllowsOutflow(t *testing.T) {
	ctx := sdk.Context{}.WithBlockHeight(10)

	// Sends FROM research_fund to a user should be unrestricted
	// (the restriction only blocks sends TO research_fund).
	researchAddr := authtypes.NewModuleAddress(ResearchFundName)
	userAddr := sdk.AccAddress([]byte("user_address_placeholder1"))
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(500)))

	retAddr, err := ResearchFundRestriction(ctx, researchAddr, userAddr, coins)
	if err != nil {
		t.Fatalf("expected outflow from research_fund to be allowed, got error: %v", err)
	}
	if !retAddr.Equals(userAddr) {
		t.Fatalf("expected returned address to be user address, got %s", retAddr.String())
	}
}

func TestResearchFundRestriction_GenesisBypass(t *testing.T) {
	// At block height 0 (genesis), all deposits to research_fund are allowed.
	ctx := sdk.Context{}.WithBlockHeight(0)

	billingAddr := authtypes.NewModuleAddress("billing")
	researchAddr := authtypes.NewModuleAddress(ResearchFundName)
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(1000)))

	retAddr, err := ResearchFundRestriction(ctx, billingAddr, researchAddr, coins)
	if err != nil {
		t.Fatalf("expected genesis bypass to allow any module -> research_fund, got error: %v", err)
	}
	if !retAddr.Equals(researchAddr) {
		t.Fatalf("expected returned address to be research_fund, got %s", retAddr.String())
	}
}

func TestResearchFundRestriction_UnrelatedSendsPassThrough(t *testing.T) {
	ctx := sdk.Context{}.WithBlockHeight(10)

	// Sends between two unrelated addresses should pass through unchanged.
	fromAddr := sdk.AccAddress([]byte("from_address_paddedok"))
	toAddr := sdk.AccAddress([]byte("to_address_paddedokk"))
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", math.NewInt(100)))

	retAddr, err := ResearchFundRestriction(ctx, fromAddr, toAddr, coins)
	if err != nil {
		t.Fatalf("expected unrelated send to pass through, got error: %v", err)
	}
	if !retAddr.Equals(toAddr) {
		t.Fatalf("expected returned address to be original toAddr, got %s", retAddr.String())
	}
}

func TestResearchFundRestriction_ResearchFundNameMatchesVestingTypes(t *testing.T) {
	// Verify that ResearchFundName in app/gas.go matches the canonical name
	// in vesting_rewards/types to prevent address mismatches.
	if ResearchFundName != vestingrewardstypes.ResearchFundModuleName {
		t.Fatalf("ResearchFundName (%q) != vestingrewardstypes.ResearchFundModuleName (%q)",
			ResearchFundName, vestingrewardstypes.ResearchFundModuleName)
	}
}
