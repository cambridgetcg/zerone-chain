package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ValidateBasic / GetSigners for the messages.

func (msg *MsgProposeCounterexample) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Author); err != nil {
		return fmt.Errorf("invalid author: %w", err)
	}
	if msg.FactId == "" {
		return fmt.Errorf("fact_id required")
	}
	if msg.WrongClaim == "" {
		return ErrEmptyWrongClaim
	}
	if msg.Reasoning == "" {
		return ErrEmptyReasoning
	}
	if msg.ErrorType == ErrorType_ERROR_TYPE_UNSPECIFIED {
		return ErrInvalidErrorType
	}
	return nil
}

func (msg *MsgProposeCounterexample) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Author)
	return []sdk.AccAddress{addr}
}

func (msg *MsgValidate) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Validator); err != nil {
		return fmt.Errorf("invalid validator: %w", err)
	}
	if msg.CounterexampleId == "" {
		return fmt.Errorf("counterexample_id required")
	}
	return nil
}

func (msg *MsgValidate) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Validator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority: %w", err)
	}
	if msg.Params != nil {
		if err := msg.Params.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

// ParseBondAmount returns the proposal_bond as a big.Int. Returns
// (0, false) on parse failure.
func ParseBondAmount(s string) (*big.Int, bool) {
	if s == "" {
		return big.NewInt(0), true
	}
	n := new(big.Int)
	_, ok := n.SetString(s, 10)
	return n, ok
}
