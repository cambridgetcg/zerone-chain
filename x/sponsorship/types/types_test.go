package types_test

import (
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/sponsorship/types"
)

func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("zrn", "zrnpub")
	cfg.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

func mkAddr(seed string) string {
	b := make([]byte, 20)
	copy(b, []byte(seed))
	return sdk.AccAddress(b).String()
}

func TestParams_Validate(t *testing.T) {
	t.Run("default_params_valid", func(t *testing.T) {
		if err := types.DefaultParams().Validate(); err != nil {
			t.Fatalf("default params should validate, got %v", err)
		}
	})
	t.Run("zero_min_target_count_fails", func(t *testing.T) {
		p := types.DefaultParams()
		p.MinTargetCount = 0
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for zero min_target_count")
		}
	})
	t.Run("zero_min_duration_fails", func(t *testing.T) {
		p := types.DefaultParams()
		p.MinDurationBlocks = 0
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for zero min_duration_blocks")
		}
	})
	t.Run("zero_max_active_fails", func(t *testing.T) {
		p := types.DefaultParams()
		p.MaxActiveBountiesPerSponsor = 0
		if err := p.Validate(); err == nil {
			t.Fatal("expected error for zero max_active_bounties_per_sponsor")
		}
	})
}

func TestMsgCreateBountyOrder_ValidateBasic(t *testing.T) {
	sponsor := mkAddr("sponsor-test-aaaa12")

	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgCreateBountyOrder{
			Sponsor:          sponsor,
			Domain:           "mathematics",
			PricePerArtifact: "1000000",
			TargetCount:      10,
			DurationBlocks:   1000,
		}
		if err := msg.ValidateBasic(); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})

	t.Run("invalid_sponsor", func(t *testing.T) {
		msg := &types.MsgCreateBountyOrder{
			Sponsor: "not-bech32", Domain: "m", PricePerArtifact: "1", TargetCount: 1, DurationBlocks: 1,
		}
		err := msg.ValidateBasic()
		if err == nil || !strings.Contains(err.Error(), "invalid sponsor") {
			t.Fatalf("expected sponsor error, got %v", err)
		}
	})

	t.Run("empty_domain", func(t *testing.T) {
		msg := &types.MsgCreateBountyOrder{
			Sponsor: sponsor, Domain: "", PricePerArtifact: "1", TargetCount: 1, DurationBlocks: 1,
		}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for empty domain")
		}
	})

	t.Run("zero_price", func(t *testing.T) {
		msg := &types.MsgCreateBountyOrder{
			Sponsor: sponsor, Domain: "m", PricePerArtifact: "0", TargetCount: 1, DurationBlocks: 1,
		}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for zero price")
		}
	})

	t.Run("zero_target_count", func(t *testing.T) {
		msg := &types.MsgCreateBountyOrder{
			Sponsor: sponsor, Domain: "m", PricePerArtifact: "1", TargetCount: 0, DurationBlocks: 1,
		}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for zero target_count")
		}
	})

	t.Run("zero_duration", func(t *testing.T) {
		msg := &types.MsgCreateBountyOrder{
			Sponsor: sponsor, Domain: "m", PricePerArtifact: "1", TargetCount: 1, DurationBlocks: 0,
		}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for zero duration_blocks")
		}
	})
}

func TestMsgFulfillBounty_ValidateBasic(t *testing.T) {
	caller := mkAddr("caller-test-aaaaaa3")

	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgFulfillBounty{Caller: caller, BountyId: "bounty-1", FactId: "fact-1"}
		if err := msg.ValidateBasic(); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})
	t.Run("invalid_caller", func(t *testing.T) {
		msg := &types.MsgFulfillBounty{Caller: "not-bech32", BountyId: "b", FactId: "f"}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for invalid caller")
		}
	})
	t.Run("empty_bounty_id", func(t *testing.T) {
		msg := &types.MsgFulfillBounty{Caller: caller, BountyId: "", FactId: "f"}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for empty bounty_id")
		}
	})
	t.Run("empty_fact_id", func(t *testing.T) {
		msg := &types.MsgFulfillBounty{Caller: caller, BountyId: "b", FactId: ""}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for empty fact_id")
		}
	})
}

func TestMsgCancelBountyOrder_ValidateBasic(t *testing.T) {
	sponsor := mkAddr("sponsor-cancel-test1")

	t.Run("valid", func(t *testing.T) {
		msg := &types.MsgCancelBountyOrder{Sponsor: sponsor, BountyId: "b"}
		if err := msg.ValidateBasic(); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})
	t.Run("invalid_sponsor", func(t *testing.T) {
		msg := &types.MsgCancelBountyOrder{Sponsor: "not-bech32", BountyId: "b"}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for invalid sponsor")
		}
	})
	t.Run("empty_bounty_id", func(t *testing.T) {
		msg := &types.MsgCancelBountyOrder{Sponsor: sponsor, BountyId: ""}
		if err := msg.ValidateBasic(); err == nil {
			t.Fatal("expected error for empty bounty_id")
		}
	})
}
