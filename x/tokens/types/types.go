package types

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// -----------------------------------------------------------------------
// ValidateBasic / GetSigners for proto-generated messages
// -----------------------------------------------------------------------

// --- MsgCreateToken ---

func (msg *MsgCreateToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Creator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgCreateToken) ValidateBasic() error {
	if msg.Creator == "" {
		return fmt.Errorf("creator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Creator); err != nil {
		return fmt.Errorf("invalid creator address: %w", err)
	}
	if msg.Symbol == "" {
		return fmt.Errorf("symbol cannot be empty")
	}
	if msg.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if msg.InitialSupply != "" {
		amt := new(big.Int)
		if _, ok := amt.SetString(msg.InitialSupply, 10); !ok || amt.Sign() < 0 {
			return fmt.Errorf("initial_supply must be a non-negative integer")
		}
	}
	if msg.MaxSupply != "" {
		amt := new(big.Int)
		if _, ok := amt.SetString(msg.MaxSupply, 10); !ok || amt.Sign() < 0 {
			return fmt.Errorf("max_supply must be a non-negative integer")
		}
	}
	return nil
}

// --- MsgMintToken ---

func (msg *MsgMintToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgMintToken) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	if msg.To == "" {
		return fmt.Errorf("to cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.To); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount must be a positive integer")
	}
	return nil
}

// --- MsgBurnToken ---

func (msg *MsgBurnToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Burner)
	return []sdk.AccAddress{addr}
}

func (msg *MsgBurnToken) ValidateBasic() error {
	if msg.Burner == "" {
		return fmt.Errorf("burner cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Burner); err != nil {
		return fmt.Errorf("invalid burner address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount must be a positive integer")
	}
	return nil
}

// --- MsgTransferToken ---

func (msg *MsgTransferToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Sender)
	return []sdk.AccAddress{addr}
}

func (msg *MsgTransferToken) ValidateBasic() error {
	if msg.Sender == "" {
		return fmt.Errorf("sender cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	if msg.To == "" {
		return fmt.Errorf("to cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.To); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	if msg.Sender == msg.To {
		return fmt.Errorf("sender and to cannot be the same")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount must be a positive integer")
	}
	return nil
}

// --- MsgApproveToken ---

func (msg *MsgApproveToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Owner)
	return []sdk.AccAddress{addr}
}

func (msg *MsgApproveToken) ValidateBasic() error {
	if msg.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Owner); err != nil {
		return fmt.Errorf("invalid owner address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	if msg.Spender == "" {
		return fmt.Errorf("spender cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Spender); err != nil {
		return fmt.Errorf("invalid spender address: %w", err)
	}
	if msg.Owner == msg.Spender {
		return fmt.Errorf("owner and spender cannot be the same")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() < 0 {
		return fmt.Errorf("amount must be a non-negative integer")
	}
	return nil
}

// --- MsgTransferFrom ---

func (msg *MsgTransferFrom) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Spender)
	return []sdk.AccAddress{addr}
}

func (msg *MsgTransferFrom) ValidateBasic() error {
	if msg.Spender == "" {
		return fmt.Errorf("spender cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Spender); err != nil {
		return fmt.Errorf("invalid spender address: %w", err)
	}
	if msg.From == "" {
		return fmt.Errorf("from cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if msg.To == "" {
		return fmt.Errorf("to cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.To); err != nil {
		return fmt.Errorf("invalid to address: %w", err)
	}
	if msg.From == msg.To {
		return fmt.Errorf("from and to cannot be the same")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount must be a positive integer")
	}
	return nil
}

// --- MsgPauseToken ---

func (msg *MsgPauseToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgPauseToken) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	return nil
}

// --- MsgUnpauseToken ---

func (msg *MsgUnpauseToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUnpauseToken) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	return nil
}

// --- MsgDelegatePower ---

func (msg *MsgDelegatePower) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Delegator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgDelegatePower) ValidateBasic() error {
	if msg.Delegator == "" {
		return fmt.Errorf("delegator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Delegator); err != nil {
		return fmt.Errorf("invalid delegator address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	if msg.Delegate == "" {
		return fmt.Errorf("delegate cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Delegate); err != nil {
		return fmt.Errorf("invalid delegate address: %w", err)
	}
	if msg.Delegator == msg.Delegate {
		return fmt.Errorf("delegator and delegate cannot be the same")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() < 0 {
		return fmt.Errorf("amount must be a non-negative integer")
	}
	return nil
}

// --- MsgUndelegatePower ---

func (msg *MsgUndelegatePower) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Delegator)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUndelegatePower) ValidateBasic() error {
	if msg.Delegator == "" {
		return fmt.Errorf("delegator cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Delegator); err != nil {
		return fmt.Errorf("invalid delegator address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	if msg.Delegate == "" {
		return fmt.Errorf("delegate cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Delegate); err != nil {
		return fmt.Errorf("invalid delegate address: %w", err)
	}
	if msg.Delegator == msg.Delegate {
		return fmt.Errorf("delegator and delegate cannot be the same")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount must be a positive integer")
	}
	return nil
}

// --- MsgWrapToken ---

func (msg *MsgWrapToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Sender)
	return []sdk.AccAddress{addr}
}

func (msg *MsgWrapToken) ValidateBasic() error {
	if msg.Sender == "" {
		return fmt.Errorf("sender cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}
	if msg.TokenId == "" {
		return fmt.Errorf("token_id cannot be empty")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount must be a positive integer")
	}
	return nil
}

// --- MsgUnwrapToken ---

func (msg *MsgUnwrapToken) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Sender)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUnwrapToken) ValidateBasic() error {
	if msg.Sender == "" {
		return fmt.Errorf("sender cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Sender); err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}
	if msg.WrappedDenom == "" {
		return fmt.Errorf("wrapped_denom cannot be empty")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.Amount, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount must be a positive integer")
	}
	return nil
}

// --- MsgCreateEmissionPeriod ---

func (msg *MsgCreateEmissionPeriod) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgCreateEmissionPeriod) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.EndBlock <= msg.StartBlock {
		return fmt.Errorf("end_block must be greater than start_block")
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(msg.AmountPerBlock, 10); !ok || amt.Sign() <= 0 {
		return fmt.Errorf("amount_per_block must be a positive integer")
	}
	if msg.Recipient == "" {
		return fmt.Errorf("recipient cannot be empty")
	}
	return nil
}

// --- MsgCancelEmissionPeriod ---

func (msg *MsgCancelEmissionPeriod) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgCancelEmissionPeriod) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.EmissionId == "" {
		return fmt.Errorf("emission_id cannot be empty")
	}
	return nil
}

// --- MsgUpdateParams ---

func (msg *MsgUpdateParams) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(msg.Authority)
	return []sdk.AccAddress{addr}
}

func (msg *MsgUpdateParams) ValidateBasic() error {
	if msg.Authority == "" {
		return fmt.Errorf("authority cannot be empty")
	}
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if msg.Params != nil {
		if err := msg.Params.Validate(); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}
	return nil
}
