package cli

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/staking/types"
)

// GetTxCmd returns the parent transaction command for the staking module.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Zerone staking transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		CmdRegisterValidator(),
		CmdUpdateStake(),
		CmdDelegate(),
		CmdUndelegate(),
		CmdRedelegate(),
	)
	return cmd
}

// CmdRegisterValidator creates a new validator.
func CmdRegisterValidator() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-validator [pubkey-hex] [self-delegation]",
		Short: "Register a new validator",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			commission, _ := cmd.Flags().GetUint64("commission")
			moniker, _ := cmd.Flags().GetString("moniker")
			identity, _ := cmd.Flags().GetString("identity")
			website, _ := cmd.Flags().GetString("website")
			details, _ := cmd.Flags().GetString("details")

			msg := &types.MsgRegisterValidator{
				Operator:        clientCtx.GetFromAddress().String(),
				ConsensusPubkey: args[0],
				SelfDelegation:  args[1],
				CommissionBps:   commission,
				Moniker:         moniker,
				Did:             identity,
				Website:         website,
				Details:         details,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64("commission", 0, "Commission rate in BPS (max 10000)")
	cmd.Flags().String("moniker", "", "Validator moniker")
	cmd.Flags().String("identity", "", "DID for validator identity")
	cmd.Flags().String("website", "", "Website URL")
	cmd.Flags().String("details", "", "Validator details")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdUpdateStake increases or decreases self-delegation.
func CmdUpdateStake() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-stake [amount]",
		Short: "Increase or decrease self-delegation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			increase, _ := cmd.Flags().GetBool("increase")

			msg := &types.MsgUpdateValidatorStake{
				Operator: clientCtx.GetFromAddress().String(),
				Amount:   args[0],
				Increase: increase,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Bool("increase", true, "Whether to increase (true) or decrease (false)")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdDelegate delegates tokens to a validator.
func CmdDelegate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delegate [validator] [amount]",
		Short: "Delegate tokens to a validator",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgDelegate{
				Delegator: clientCtx.GetFromAddress().String(),
				Validator: args[0],
				Amount:    args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdUndelegate initiates unbonding from a validator.
func CmdUndelegate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "undelegate [validator] [amount]",
		Short: "Initiate unbonding from a validator",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgUndelegate{
				Delegator: clientCtx.GetFromAddress().String(),
				Validator: args[0],
				Amount:    args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdRedelegate moves a delegation between validators.
func CmdRedelegate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redelegate [src-validator] [dst-validator] [amount]",
		Short: fmt.Sprintf("Move delegation (cooldown: %d blocks)", 1111),
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgRedelegate{
				Delegator:    clientCtx.GetFromAddress().String(),
				SrcValidator: args[0],
				DstValidator: args[1],
				Amount:       args[2],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
