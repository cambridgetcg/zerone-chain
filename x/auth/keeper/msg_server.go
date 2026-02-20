package keeper

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"

	cosmosed25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/auth/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// RegisterAccount creates a new Zerone account with DID mapping.
func (ms msgServer) RegisterAccount(goCtx context.Context, msg *types.MsgRegisterAccount) (*types.MsgRegisterAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if _, found := ms.GetAccount(ctx, msg.Sender); found {
		return nil, types.ErrAccountAlreadyExists
	}

	if _, found := ms.GetDIDMapping(ctx, msg.Did); found {
		return nil, types.ErrDuplicateDID
	}

	// Validate DID derives from the public key (prevents DID spoofing).
	if err := types.ValidateDIDDerivation(msg.Did, msg.PublicKey); err != nil {
		return nil, fmt.Errorf("DID derivation mismatch: %w", err)
	}

	currentHeight := uint64(ctx.BlockHeight())

	flags := &types.AccountFlags{
		CanSubmitClaims: true,
		CanChallenge:    true,
	}

	switch msg.AccountType {
	case "contract":
		flags.CanSubmitClaims = false
		flags.CanChallenge = false
	case "system":
		flags.Frozen = true
		flags.CanSubmitClaims = false
		flags.CanChallenge = false
	}

	account := types.Account{
		Address:               msg.Sender,
		Did:                   msg.Did,
		PublicKey:             msg.PublicKey,
		AccountType:           msg.AccountType,
		OperationalKeyHash:    msg.OperationalKeyHash,
		OperationalPublicKey:  msg.PublicKey, // identity key is initial operational key
		OperationalKeyVersion: 1,
		SessionKeyCount:       0,
		ReputationScore:       500000, // 0.5 default
		CreatedAtBlock:        currentHeight,
		LastActiveBlock:       currentHeight,
		Flags:                 flags,
		Metadata:              msg.Metadata,
	}
	ms.SetAccount(ctx, &account)

	mapping := types.DIDMapping{
		Did:    msg.Did,
		Bech32: msg.Sender,
		PubKey: msg.PublicKey,
	}
	ms.SetDIDMapping(ctx, &mapping)

	// Sync Ed25519 pubkey to Cosmos BaseAccount for standard sig verification.
	senderAddr, _ := sdk.AccAddressFromBech32(msg.Sender)
	if ms.accountKeeper != nil {
		cosmosAcc := ms.accountKeeper.GetAccount(ctx, senderAddr)
		if cosmosAcc == nil {
			cosmosAcc = ms.accountKeeper.NewAccountWithAddress(ctx, senderAddr)
		}
		if cosmosAcc != nil && cosmosAcc.GetPubKey() == nil {
			keyBytes, err := hex.DecodeString(msg.PublicKey)
			if err == nil && len(keyBytes) == 32 {
				pubKey := &cosmosed25519.PubKey{Key: keyBytes}
				if err := cosmosAcc.SetPubKey(pubKey); err == nil {
					ms.accountKeeper.SetAccount(ctx, cosmosAcc)
				}
			}
		}
	}

	// Bootstrap fund: auto-claim for new human/agent accounts.
	params := ms.GetParams(ctx)
	bootstrapClaimed := false
	if params.BootstrapEnabled && (msg.AccountType == "human" || msg.AccountType == "agent") {
		if !ms.HasBootstrapClaim(ctx, msg.Sender) {
			amount, ok := new(big.Int).SetString(params.BootstrapAmount, 10)
			if ok && amount.Sign() > 0 {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(amount)))
				if err := ms.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderAddr, coins); err != nil {
					ms.Logger(ctx).Debug("bootstrap fund disbursement failed", "address", msg.Sender, "error", err)
				} else {
					ms.SetBootstrapClaim(ctx, msg.Sender)
					bootstrapClaimed = true
				}
			}
		}
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"account_registered",
			sdk.NewAttribute("address", msg.Sender),
			sdk.NewAttribute("did", msg.Did),
			sdk.NewAttribute("account_type", msg.AccountType),
			sdk.NewAttribute("bootstrap_claimed", fmt.Sprintf("%t", bootstrapClaimed)),
		),
	)

	return &types.MsgRegisterAccountResponse{}, nil
}

// RotateKey handles operational key rotation.
func (ms msgServer) RotateKey(goCtx context.Context, msg *types.MsgRotateKey) (*types.MsgRotateKeyResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	account, found := ms.GetAccount(ctx, msg.Sender)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	if account.Flags != nil && account.Flags.Frozen {
		return nil, types.ErrAccountFrozen
	}

	if len(msg.NewOperationalKey) == 0 {
		return nil, types.ErrInvalidKeyType
	}

	params := ms.GetParams(ctx)
	lastRotation := ms.GetLastRotation(ctx, msg.Sender)
	currentHeight := uint64(ctx.BlockHeight())

	if lastRotation > 0 && currentHeight < lastRotation+params.KeyRotationCooldown {
		return nil, types.ErrKeyRotationCooldown
	}

	newPubKeyHex := hex.EncodeToString(msg.NewOperationalKey)
	account.OperationalPublicKey = newPubKeyHex
	account.OperationalKeyVersion++
	ms.SetLastRotation(ctx, msg.Sender, currentHeight)

	// Sync Ed25519 pubkey to Cosmos BaseAccount.
	if ms.accountKeeper != nil && len(msg.NewOperationalKey) == 32 {
		senderAddr, _ := sdk.AccAddressFromBech32(msg.Sender)
		if cosmosAcc := ms.accountKeeper.GetAccount(ctx, senderAddr); cosmosAcc != nil {
			pubKey := &cosmosed25519.PubKey{Key: msg.NewOperationalKey}
			if setErr := cosmosAcc.SetPubKey(pubKey); setErr == nil {
				ms.accountKeeper.SetAccount(ctx, cosmosAcc)
			}
		}
	}

	account.LastActiveBlock = currentHeight
	ms.SetAccount(ctx, account)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"key_rotated",
			sdk.NewAttribute("sender", msg.Sender),
			sdk.NewAttribute("key_type", "operational"),
			sdk.NewAttribute("version", fmt.Sprintf("%d", account.OperationalKeyVersion)),
		),
	)

	return &types.MsgRotateKeyResponse{
		NewKeyVersion: account.OperationalKeyVersion,
	}, nil
}

// CreateSession creates a new session key with limited capabilities.
func (ms msgServer) CreateSession(goCtx context.Context, msg *types.MsgCreateSession) (*types.MsgCreateSessionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	account, found := ms.GetAccount(ctx, msg.Owner)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	if account.Flags != nil && account.Flags.Frozen {
		return nil, types.ErrAccountFrozen
	}

	params := ms.GetParams(ctx)
	currentHeight := uint64(ctx.BlockHeight())
	expiresAt := msg.ExpiresAtHeight
	if expiresAt == 0 {
		expiresAt = currentHeight + params.MaxSessionDuration
	}
	duration := expiresAt - currentHeight
	if duration > params.MaxSessionDuration {
		return nil, types.ErrSessionDurationExceeded
	}

	activeCount := ms.CountSessionKeys(ctx, msg.Owner)
	if activeCount >= params.MaxSessionKeys {
		return nil, types.ErrMaxSessionKeys
	}

	keyHash := hex.EncodeToString(msg.SessionPubKey)
	pubKeyHex := hex.EncodeToString(msg.SessionPubKey)

	session := types.SessionKey{
		KeyHash:        keyHash,
		PublicKey:      pubKeyHex,
		Owner:          msg.Owner,
		Capabilities:   msg.Capabilities,
		ExpiresAtBlock: expiresAt,
		CreatedAtBlock: currentHeight,
	}

	ms.SetSessionKey(ctx, &session)

	account.SessionKeyCount++
	account.LastActiveBlock = currentHeight
	ms.SetAccount(ctx, account)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"session_created",
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("key_hash", keyHash),
			sdk.NewAttribute("expires_at", fmt.Sprintf("%d", expiresAt)),
		),
	)

	return &types.MsgCreateSessionResponse{
		SessionId: keyHash,
	}, nil
}

// RevokeSession revokes an existing session key.
func (ms msgServer) RevokeSession(goCtx context.Context, msg *types.MsgRevokeSession) (*types.MsgRevokeSessionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	account, found := ms.GetAccount(ctx, msg.Owner)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	_, found = ms.GetSessionKey(ctx, msg.Owner, msg.SessionId)
	if !found {
		return nil, types.ErrSessionNotFound
	}

	ms.DeleteSessionKey(ctx, msg.Owner, msg.SessionId)

	if account.SessionKeyCount > 0 {
		account.SessionKeyCount--
	}
	account.LastActiveBlock = uint64(ctx.BlockHeight())
	ms.SetAccount(ctx, account)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"session_revoked",
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("key_hash", msg.SessionId),
		),
	)

	return &types.MsgRevokeSessionResponse{}, nil
}

// FreezeAccount freezes an account. Owner can self-freeze; authority can freeze anyone.
func (ms msgServer) FreezeAccount(goCtx context.Context, msg *types.MsgFreezeAccount) (*types.MsgFreezeAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	account, found := ms.GetAccount(ctx, msg.Address)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	if account.Flags == nil {
		account.Flags = &types.AccountFlags{}
	}
	if account.Flags.Frozen {
		return nil, types.ErrAccountFrozen
	}

	if msg.Sender != msg.Address && msg.Sender != ms.Keeper.GetAuthority() {
		return nil, fmt.Errorf("%w: only account owner or authority can freeze", types.ErrUnauthorized)
	}

	account.Flags.Frozen = true
	account.Flags.FreezeReason = msg.Reason
	ms.SetAccount(ctx, account)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"account_frozen",
			sdk.NewAttribute("address", msg.Address),
			sdk.NewAttribute("frozen_by", msg.Sender),
			sdk.NewAttribute("reason", msg.Reason),
		),
	)

	return &types.MsgFreezeAccountResponse{}, nil
}

// UnfreezeAccount unfreezes a frozen account. Authority-only.
func (ms msgServer) UnfreezeAccount(goCtx context.Context, msg *types.MsgUnfreezeAccount) (*types.MsgUnfreezeAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Authority != ms.Keeper.GetAuthority() {
		return nil, fmt.Errorf("%w: only authority can unfreeze accounts", types.ErrUnauthorized)
	}

	account, found := ms.GetAccount(ctx, msg.Address)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	if account.Flags == nil || !account.Flags.Frozen {
		return nil, types.ErrAccountNotFrozen
	}

	account.Flags.Frozen = false
	account.Flags.FreezeReason = ""
	ms.SetAccount(ctx, account)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"account_unfrozen",
			sdk.NewAttribute("address", msg.Address),
			sdk.NewAttribute("unfrozen_by", msg.Authority),
		),
	)

	return &types.MsgUnfreezeAccountResponse{}, nil
}

// SetRecoveryConfig stores recovery configuration for the sender's account.
func (ms msgServer) SetRecoveryConfig(goCtx context.Context, msg *types.MsgSetRecoveryConfig) (*types.MsgSetRecoveryConfigResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if _, found := ms.GetAccount(ctx, msg.Sender); !found {
		return nil, types.ErrAccountNotFound
	}

	ms.Keeper.SetRecoveryConfig(ctx, msg.Sender, msg.Config)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"recovery_config_set",
			sdk.NewAttribute("address", msg.Sender),
			sdk.NewAttribute("threshold", fmt.Sprintf("%d", msg.Config.Threshold)),
			sdk.NewAttribute("total_shards", fmt.Sprintf("%d", msg.Config.TotalShards)),
		),
	)

	return &types.MsgSetRecoveryConfigResponse{}, nil
}

// InitiateRecovery begins account recovery. The sender must be an authorized shard holder.
func (ms msgServer) InitiateRecovery(goCtx context.Context, msg *types.MsgInitiateRecovery) (*types.MsgInitiateRecoveryResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	account, found := ms.GetAccount(ctx, msg.AccountAddress)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	if _, exists := ms.Keeper.GetRecoveryRequest(ctx, msg.AccountAddress); exists {
		return nil, types.ErrRecoveryAlreadyActive
	}

	config, found := ms.Keeper.GetRecoveryConfig(ctx, msg.AccountAddress)
	if !found {
		return nil, types.ErrRecoveryConfigNotFound
	}

	authorized := false
	for _, holder := range config.ShardHolders {
		if holder.Identifier == msg.Sender && holder.CanInitiateRecovery {
			authorized = true
			break
		}
	}
	if !authorized {
		return nil, fmt.Errorf("%w: sender is not an authorized recovery initiator", types.ErrUnauthorized)
	}

	currentHeight := uint64(ctx.BlockHeight())
	delayBlocks := config.RecoveryDelayBlocks
	if delayBlocks == 0 {
		delayBlocks = ms.Keeper.GetParams(ctx).RecoveryDelayBlocks
	}
	challengeBlocks := config.ChallengePeriodBlocks
	if challengeBlocks == 0 {
		challengeBlocks = ms.Keeper.GetParams(ctx).ChallengePeriodBlocks
	}

	req := types.RecoveryRequest{
		AccountAddress:     msg.AccountAddress,
		InitiatedBy:        msg.Sender,
		NewOperationalKey:  msg.NewOperationalKey,
		ShardsProvided:     []uint32{},
		ShardsRequired:     config.Threshold,
		InitiatedAtBlock:   currentHeight,
		DelayExpiresAt:     currentHeight + delayBlocks,
		ChallengeExpiresAt: currentHeight + delayBlocks + challengeBlocks,
		Status:             "pending",
	}

	ms.Keeper.SetRecoveryRequest(ctx, &req)

	if account.Flags == nil {
		account.Flags = &types.AccountFlags{}
	}
	account.Flags.InRecovery = true
	ms.SetAccount(ctx, account)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"recovery_initiated",
			sdk.NewAttribute("account", msg.AccountAddress),
			sdk.NewAttribute("initiated_by", msg.Sender),
			sdk.NewAttribute("delay_expires_at", fmt.Sprintf("%d", req.DelayExpiresAt)),
		),
	)

	return &types.MsgInitiateRecoveryResponse{}, nil
}

// SubmitRecoveryShard provides a shard to an active recovery request.
func (ms msgServer) SubmitRecoveryShard(goCtx context.Context, msg *types.MsgSubmitRecoveryShard) (*types.MsgSubmitRecoveryShardResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	req, found := ms.Keeper.GetRecoveryRequest(ctx, msg.AccountAddress)
	if !found {
		return nil, types.ErrRecoveryNotFound
	}

	if req.Status != "pending" {
		return nil, fmt.Errorf("recovery is in '%s' status, shards can only be submitted while 'pending'", req.Status)
	}

	config, found := ms.Keeper.GetRecoveryConfig(ctx, msg.AccountAddress)
	if !found {
		return nil, types.ErrRecoveryConfigNotFound
	}

	shardAuthorized := false
	for _, holder := range config.ShardHolders {
		if holder.ShardIndex == msg.ShardIndex && holder.Identifier == msg.Sender {
			shardAuthorized = true
			break
		}
	}
	if !shardAuthorized {
		return nil, fmt.Errorf("%w: sender does not hold shard index %d", types.ErrInvalidShard, msg.ShardIndex)
	}

	for _, idx := range req.ShardsProvided {
		if idx == msg.ShardIndex {
			return nil, fmt.Errorf("%w: shard %d already submitted", types.ErrInvalidShard, msg.ShardIndex)
		}
	}

	req.ShardsProvided = append(req.ShardsProvided, msg.ShardIndex)

	// Store encrypted shard data on-chain if provided.
	if len(msg.EncryptedShard) > 0 {
		shard := &types.RecoveryShard{
			OwnerAddress:     msg.AccountAddress,
			ShardIndex:       msg.ShardIndex,
			SubmitterAddress: msg.Sender,
			EncryptedData:    msg.EncryptedShard,
			SubmittedBlock:   uint64(ctx.BlockHeight()),
		}
		ms.Keeper.StoreRecoveryShard(ctx, shard)
	}

	if uint32(len(req.ShardsProvided)) >= req.ShardsRequired {
		req.Status = "delayed"
	}

	ms.Keeper.SetRecoveryRequest(ctx, req)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"recovery_shard_submitted",
			sdk.NewAttribute("account", msg.AccountAddress),
			sdk.NewAttribute("shard_index", fmt.Sprintf("%d", msg.ShardIndex)),
			sdk.NewAttribute("shards_count", fmt.Sprintf("%d/%d", len(req.ShardsProvided), req.ShardsRequired)),
			sdk.NewAttribute("status", req.Status),
		),
	)

	return &types.MsgSubmitRecoveryShardResponse{}, nil
}

// ChallengeRecovery challenges an active recovery. Only the account owner or authority can challenge.
func (ms msgServer) ChallengeRecovery(goCtx context.Context, msg *types.MsgChallengeRecovery) (*types.MsgChallengeRecoveryResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	req, found := ms.Keeper.GetRecoveryRequest(ctx, msg.AccountAddress)
	if !found {
		return nil, types.ErrRecoveryNotFound
	}

	if req.Status != "challengeable" {
		return nil, fmt.Errorf("recovery is in '%s' status, can only challenge while 'challengeable'", req.Status)
	}

	if msg.Sender != msg.AccountAddress && msg.Sender != ms.Keeper.GetAuthority() {
		return nil, fmt.Errorf("%w: only account owner or authority can challenge recovery", types.ErrUnauthorized)
	}

	req.Status = "cancelled"
	req.ChallengerAddress = msg.Sender
	req.ChallengeReason = msg.Reason
	ms.Keeper.SetRecoveryRequest(ctx, req)

	if account, accFound := ms.GetAccount(ctx, msg.AccountAddress); accFound {
		if account.Flags != nil {
			account.Flags.InRecovery = false
		}
		ms.SetAccount(ctx, account)
	}

	ms.Keeper.DeleteRecoveryShards(ctx, msg.AccountAddress)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"recovery_challenged",
			sdk.NewAttribute("account", msg.AccountAddress),
			sdk.NewAttribute("challenger", msg.Sender),
			sdk.NewAttribute("reason", msg.Reason),
		),
	)

	return &types.MsgChallengeRecoveryResponse{}, nil
}

// ExecuteRecovery completes a recovery after delay + challenge period have passed.
func (ms msgServer) ExecuteRecovery(goCtx context.Context, msg *types.MsgExecuteRecovery) (*types.MsgExecuteRecoveryResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	req, found := ms.Keeper.GetRecoveryRequest(ctx, msg.AccountAddress)
	if !found {
		return nil, types.ErrRecoveryNotFound
	}

	if req.Status != "executable" {
		return nil, fmt.Errorf("%w: recovery status is '%s', must be 'executable'", types.ErrRecoveryNotExecutable, req.Status)
	}

	account, found := ms.GetAccount(ctx, msg.AccountAddress)
	if !found {
		return nil, types.ErrAccountNotFound
	}

	account.OperationalPublicKey = req.NewOperationalKey
	account.OperationalKeyVersion++
	if account.Flags == nil {
		account.Flags = &types.AccountFlags{}
	}
	account.Flags.InRecovery = false
	account.LastActiveBlock = uint64(ctx.BlockHeight())
	ms.SetAccount(ctx, account)

	// Sync to Cosmos BaseAccount.
	if ms.accountKeeper != nil {
		senderAddr, _ := sdk.AccAddressFromBech32(msg.AccountAddress)
		if cosmosAcc := ms.accountKeeper.GetAccount(ctx, senderAddr); cosmosAcc != nil {
			keyBytes, decErr := hex.DecodeString(req.NewOperationalKey)
			if decErr == nil && len(keyBytes) == 32 {
				pubKey := &cosmosed25519.PubKey{Key: keyBytes}
				if setErr := cosmosAcc.SetPubKey(pubKey); setErr == nil {
					ms.accountKeeper.SetAccount(ctx, cosmosAcc)
				}
			}
		}
	}

	// Revoke all existing session keys (security measure).
	sessions := ms.GetSessionKeysForOwner(ctx, msg.AccountAddress)
	for _, session := range sessions {
		ms.DeleteSessionKey(ctx, msg.AccountAddress, session.KeyHash)
	}
	account.SessionKeyCount = 0
	ms.SetAccount(ctx, account)

	req.Status = "completed"
	ms.Keeper.SetRecoveryRequest(ctx, req)

	ms.Keeper.DeleteRecoveryShards(ctx, msg.AccountAddress)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"recovery_executed",
			sdk.NewAttribute("account", msg.AccountAddress),
			sdk.NewAttribute("executed_by", msg.Sender),
			sdk.NewAttribute("new_key_version", fmt.Sprintf("%d", account.OperationalKeyVersion)),
		),
	)

	return &types.MsgExecuteRecoveryResponse{}, nil
}

// UpdateParams updates auth module parameters. Authority-only.
func (ms msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if ms.GetAuthority() != msg.Authority {
		return nil, types.ErrUnauthorized
	}

	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := ms.Keeper.SetParams(ctx, msg.Params); err != nil {
		return nil, fmt.Errorf("failed to set params: %w", err)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
