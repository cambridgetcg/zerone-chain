package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---- Params ----

func DefaultParams() *Params {
	return &Params{
		MinTargetCount:              1,
		MinDurationBlocks:           100,
		MaxActiveBountiesPerSponsor: 16,
	}
}

func (p *Params) Validate() error {
	if p.MinTargetCount == 0 {
		return fmt.Errorf("min_target_count must be positive")
	}
	if p.MinDurationBlocks == 0 {
		return fmt.Errorf("min_duration_blocks must be positive")
	}
	if p.MaxActiveBountiesPerSponsor == 0 {
		return fmt.Errorf("max_active_bounties_per_sponsor must be positive")
	}
	return nil
}

// ---- Genesis ----

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:       DefaultParams(),
		Orders:       []*BountyOrder{},
		Fulfillments: []*BountyFulfillment{},
		NextBountyId: 1,
	}
}

func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := gs.Params.Validate(); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}
	seenOrders := make(map[string]bool, len(gs.Orders))
	for _, o := range gs.Orders {
		if seenOrders[o.Id] {
			return fmt.Errorf("duplicate bounty order id: %s", o.Id)
		}
		seenOrders[o.Id] = true
	}
	return nil
}

// ---- MsgCreateBountyOrder ----

func (msg *MsgCreateBountyOrder) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Sponsor)
	return []sdk.AccAddress{addr}
}

func (msg *MsgCreateBountyOrder) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Sponsor); err != nil {
		return fmt.Errorf("invalid sponsor address: %w", err)
	}
	if msg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	price := new(big.Int)
	if _, ok := price.SetString(msg.PricePerArtifact, 10); !ok || price.Sign() <= 0 {
		return fmt.Errorf("price_per_artifact must be a positive integer in uzrn")
	}
	if msg.TargetCount == 0 {
		return fmt.Errorf("target_count must be positive")
	}
	if msg.DurationBlocks == 0 {
		return fmt.Errorf("duration_blocks must be positive")
	}
	return nil
}

// ---- MsgFulfillBounty ----

func (msg *MsgFulfillBounty) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Caller)
	return []sdk.AccAddress{addr}
}

func (msg *MsgFulfillBounty) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Caller); err != nil {
		return fmt.Errorf("invalid caller address: %w", err)
	}
	if msg.BountyId == "" {
		return fmt.Errorf("bounty_id cannot be empty")
	}
	if msg.FactId == "" {
		return fmt.Errorf("fact_id cannot be empty")
	}
	return nil
}

// ---- MsgCancelBountyOrder ----

func (msg *MsgCancelBountyOrder) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Sponsor)
	return []sdk.AccAddress{addr}
}

func (msg *MsgCancelBountyOrder) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Sponsor); err != nil {
		return fmt.Errorf("invalid sponsor address: %w", err)
	}
	if msg.BountyId == "" {
		return fmt.Errorf("bounty_id cannot be empty")
	}
	return nil
}
