package cli

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/auth/types"
)

// GetTxCmd returns the transaction commands for this module.
func GetTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Zerone auth transaction subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		CmdRegisterAccount(),
		CmdRotateKey(),
		CmdCreateSession(),
		CmdRevokeSession(),
		CmdFreezeAccount(),
		CmdUnfreezeAccount(),
		CmdSetRecoveryConfig(),
		CmdInitiateRecovery(),
		CmdSubmitRecoveryShard(),
		CmdChallengeRecovery(),
		CmdExecuteRecovery(),
	)

	return txCmd
}

// CmdRegisterAccount registers a new Zerone account.
func CmdRegisterAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-account [did] [public-key] [account-type]",
		Short: "Register a new Zerone account with DID mapping",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			opKeyHash, _ := cmd.Flags().GetString("operational-key-hash")
			metadata, _ := cmd.Flags().GetString("metadata")

			msg := &types.MsgRegisterAccount{
				Sender:             clientCtx.GetFromAddress().String(),
				Did:                args[0],
				PublicKey:          args[1],
				AccountType:        args[2],
				OperationalKeyHash: opKeyHash,
				Metadata:           metadata,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("operational-key-hash", "", "Hash of initial operational key")
	cmd.Flags().String("metadata", "", "Account metadata (JSON string)")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// CmdRotateKey rotates the operational key.
func CmdRotateKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate-key [new-op-key-hex] [auth-sig-hex]",
		Short: "Rotate operational key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			newKey, err := hex.DecodeString(args[0])
			if err != nil {
				return fmt.Errorf("invalid new key hex: %w", err)
			}

			authSig, err := hex.DecodeString(args[1])
			if err != nil {
				return fmt.Errorf("invalid auth signature hex: %w", err)
			}

			msg := &types.MsgRotateKey{
				Sender:                 clientCtx.GetFromAddress().String(),
				NewOperationalKey:      newKey,
				AuthorizationSignature: authSig,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdCreateSession creates a session key.
func CmdCreateSession() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-session [session-pub-key-hex] [expires-at-height]",
		Short: "Create an ephemeral session key",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			sessionKey, err := hex.DecodeString(args[0])
			if err != nil {
				return fmt.Errorf("invalid session key hex: %w", err)
			}

			var expiresAt uint64
			if len(args) > 1 {
				expiresAt, err = strconv.ParseUint(args[1], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid expires-at-height: %w", err)
				}
			}

			canTransfer, _ := cmd.Flags().GetBool("can-transfer")
			canStake, _ := cmd.Flags().GetBool("can-stake")
			canSubmitClaims, _ := cmd.Flags().GetBool("can-submit-claims")
			canVote, _ := cmd.Flags().GetBool("can-vote")

			caps := &types.SessionCapabilities{
				CanTransfer:     canTransfer,
				CanStake:        canStake,
				CanSubmitClaims: canSubmitClaims,
				CanVote:         canVote,
			}

			msg := &types.MsgCreateSession{
				Owner:          clientCtx.GetFromAddress().String(),
				SessionPubKey:  sessionKey,
				Capabilities:   caps,
				ExpiresAtHeight: expiresAt,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Bool("can-transfer", false, "Allow transfers")
	cmd.Flags().Bool("can-stake", false, "Allow staking")
	cmd.Flags().Bool("can-submit-claims", false, "Allow claim submission")
	cmd.Flags().Bool("can-vote", false, "Allow voting")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// CmdRevokeSession revokes a session key.
func CmdRevokeSession() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke-session [session-id]",
		Short: "Revoke an active session key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgRevokeSession{
				Owner:     clientCtx.GetFromAddress().String(),
				SessionId: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdFreezeAccount freezes an account.
func CmdFreezeAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freeze-account [address]",
		Short: "Freeze an account (self or authority)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			reason, _ := cmd.Flags().GetString("reason")

			msg := &types.MsgFreezeAccount{
				Sender:  clientCtx.GetFromAddress().String(),
				Address: args[0],
				Reason:  reason,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("reason", "", "Reason for freezing")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// CmdUnfreezeAccount unfreezes an account.
func CmdUnfreezeAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unfreeze-account [address]",
		Short: "Unfreeze a frozen account (authority only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgUnfreezeAccount{
				Authority: clientCtx.GetFromAddress().String(),
				Address:   args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdSetRecoveryConfig sets recovery configuration.
func CmdSetRecoveryConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-recovery-config [threshold] [total-shards] [holder-addresses...]",
		Short: "Set recovery configuration for your account",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			threshold, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid threshold: %w", err)
			}

			totalShards, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid total-shards: %w", err)
			}

			holderAddrs := args[2:]
			if len(holderAddrs) != int(totalShards) {
				return fmt.Errorf("expected %d holder addresses, got %d", totalShards, len(holderAddrs))
			}

			holders := make([]*types.ShardHolder, len(holderAddrs))
			for i, addr := range holderAddrs {
				holders[i] = &types.ShardHolder{
					Type:                 "address",
					Identifier:           addr,
					ShardIndex:           uint32(i + 1),
					CanInitiateRecovery:  i == 0, // first holder can initiate
				}
			}

			config := &types.RecoveryConfig{
				Threshold:    uint32(threshold),
				TotalShards:  uint32(totalShards),
				ShardHolders: holders,
			}

			msg := &types.MsgSetRecoveryConfig{
				Sender: clientCtx.GetFromAddress().String(),
				Config: config,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdInitiateRecovery begins account recovery.
func CmdInitiateRecovery() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "initiate-recovery [account-address] [new-op-key-hex]",
		Short: "Initiate account recovery",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgInitiateRecovery{
				Sender:            clientCtx.GetFromAddress().String(),
				AccountAddress:    args[0],
				NewOperationalKey: args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdSubmitRecoveryShard submits a recovery shard.
func CmdSubmitRecoveryShard() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-recovery-shard [account-address] [shard-index]",
		Short: "Submit a recovery shard",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			shardIndex, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid shard-index: %w", err)
			}

			msg := &types.MsgSubmitRecoveryShard{
				Sender:         clientCtx.GetFromAddress().String(),
				AccountAddress: args[0],
				ShardIndex:     uint32(shardIndex),
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdChallengeRecovery challenges a recovery.
func CmdChallengeRecovery() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "challenge-recovery [account-address] [reason]",
		Short: "Challenge an active recovery request",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgChallengeRecovery{
				Sender:         clientCtx.GetFromAddress().String(),
				AccountAddress: args[0],
				Reason:         args[1],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdExecuteRecovery executes a recovery.
func CmdExecuteRecovery() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute-recovery [account-address]",
		Short: "Execute a recovery after delay + challenge period",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := &types.MsgExecuteRecovery{
				Sender:         clientCtx.GetFromAddress().String(),
				AccountAddress: args[0],
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
