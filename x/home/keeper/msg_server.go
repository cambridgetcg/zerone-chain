package keeper

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/home/types"
)

const (
	MaxNameLength    = 128
	MaxKeyHashLength = 128
	MaxCIDLength     = 256
)

// validateName rejects empty, too-long, or null-byte-containing names.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty", types.ErrInvalidInput)
	}
	if len(name) > MaxNameLength {
		return fmt.Errorf("%w: name exceeds max length %d", types.ErrInvalidInput, MaxNameLength)
	}
	if strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("%w: name contains null bytes", types.ErrInvalidInput)
	}
	return nil
}

// validateKeyHash rejects empty or too-long key hashes.
func validateKeyHash(keyHash string) error {
	if keyHash == "" {
		return fmt.Errorf("%w: key_hash cannot be empty", types.ErrInvalidInput)
	}
	if len(keyHash) > MaxKeyHashLength {
		return fmt.Errorf("%w: key_hash exceeds max length %d", types.ErrInvalidInput, MaxKeyHashLength)
	}
	return nil
}

// validateCID rejects empty or too-long CIDs.
func validateCID(cid string) error {
	if cid == "" {
		return fmt.Errorf("%w: cid cannot be empty", types.ErrInvalidInput)
	}
	if len(cid) > MaxCIDLength {
		return fmt.Errorf("%w: cid exceeds max length %d", types.ErrInvalidInput, MaxCIDLength)
	}
	return nil
}

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// CreateHome creates a new agent home.
func (k msgServer) CreateHome(goCtx context.Context, msg *types.MsgCreateHome) (*types.MsgCreateHomeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := validateName(msg.Name); err != nil {
		return nil, err
	}

	ownerAddr, err := sdk.AccAddressFromBech32(msg.Owner)
	if err != nil {
		return nil, fmt.Errorf("invalid owner address: %w", err)
	}

	// Charge creation fee.
	params := k.GetParams(ctx)
	fee := new(big.Int)
	if _, ok := fee.SetString(params.HomeCreationFee, 10); !ok || fee.Sign() <= 0 {
		fee.SetInt64(10000000)
	}
	feeCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(fee)))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, ownerAddr, "fee_collector", feeCoins); err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrInsufficientFunds, err)
	}

	homeID := k.GetNextHomeID(ctx)
	height := uint64(ctx.BlockHeight())

	guardian := msg.InitialGuardianConfig
	if guardian == nil {
		guardian = &types.HomeGuardian{
			DefenseStrategy: "moderate",
			AutoDefend:      false,
		}
	}

	home := &types.AgentHome{
		HomeId:          homeID,
		OwnerAddress:    msg.Owner,
		Name:            msg.Name,
		Status:          "active",
		ComfortScore:    50,
		Treasury:        &types.HomeTreasury{ReservedBalance: "0"},
		Guardian:        guardian,
		CreatedAtBlock:  height,
		LastActiveBlock: height,
	}

	k.SetHome(ctx, home)
	k.AddHomeToOwnerIndex(ctx, msg.Owner, homeID)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.home_created",
			sdk.NewAttribute("home_id", homeID),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("name", msg.Name),
		),
	)

	return &types.MsgCreateHomeResponse{HomeId: homeID}, nil
}

// UpdateHome updates an existing home's name and/or status.
func (k msgServer) UpdateHome(goCtx context.Context, msg *types.MsgUpdateHome) (*types.MsgUpdateHomeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}
	if home.OwnerAddress != msg.Owner {
		return nil, fmt.Errorf("%w: not the home owner", types.ErrUnauthorized)
	}

	if msg.Name != "" {
		if err := validateName(msg.Name); err != nil {
			return nil, err
		}
		home.Name = msg.Name
	}
	if msg.Status != "" {
		if !isValidStatusTransition(home.Status, msg.Status) {
			return nil, fmt.Errorf("%w: %s -> %s", types.ErrInvalidStatus, home.Status, msg.Status)
		}
		home.Status = msg.Status
	}

	home.LastActiveBlock = uint64(ctx.BlockHeight())
	k.SetHome(ctx, home)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.home_updated",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("status", home.Status),
		),
	)

	return &types.MsgUpdateHomeResponse{}, nil
}

// isValidStatusTransition checks if a status transition is allowed.
func isValidStatusTransition(from, to string) bool {
	transitions := map[string]map[string]bool{
		"active": {
			"dormant":  true,
			"guarded":  true,
			"recovery": true,
			"archived": true,
		},
		"dormant": {
			"active":   true,
			"archived": true,
		},
		"guarded": {
			"active":   true,
			"archived": true,
		},
		"recovery": {
			"active":   true,
			"archived": true,
		},
	}
	allowed, ok := transitions[from]
	if !ok {
		return false // archived is terminal
	}
	return allowed[to]
}

// UpdateMemoryCID updates the IPFS memory CID for a home.
func (k msgServer) UpdateMemoryCID(goCtx context.Context, msg *types.MsgUpdateMemoryCID) (*types.MsgUpdateMemoryCIDResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := validateCID(msg.Cid); err != nil {
		return nil, err
	}

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}
	if home.OwnerAddress != msg.Owner {
		return nil, fmt.Errorf("%w: not the home owner", types.ErrUnauthorized)
	}

	home.MemoryCid = msg.Cid
	home.LastActiveBlock = uint64(ctx.BlockHeight())
	k.SetHome(ctx, home)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.memory_cid_updated",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("cid", msg.Cid),
		),
	)

	return &types.MsgUpdateMemoryCIDResponse{}, nil
}

// StartSession starts a new session for a registered key.
func (k msgServer) StartSession(goCtx context.Context, msg *types.MsgStartSession) (*types.MsgStartSessionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	height := uint64(ctx.BlockHeight())

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}

	// Verify key is registered, not revoked, not expired.
	reg, found := k.GetKeyRegistration(ctx, msg.HomeId, msg.KeyHash)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrKeyNotFound, msg.KeyHash)
	}
	if reg.Revoked {
		return nil, fmt.Errorf("%w: %s", types.ErrKeyRevoked, msg.KeyHash)
	}
	if reg.ExpiresAt > 0 && height > reg.ExpiresAt {
		return nil, fmt.Errorf("%w: %s", types.ErrKeyExpired, msg.KeyHash)
	}

	// Enforce max sessions.
	params := k.GetParams(ctx)
	if k.CountSessions(ctx, msg.HomeId) >= params.MaxSessionsPerHome {
		return nil, fmt.Errorf("%w: limit %d", types.ErrMaxSessionsReached, params.MaxSessionsPerHome)
	}

	// Compute granted permissions (intersection).
	granted := intersectPermissions(reg.Permissions, msg.RequestedPermissions)

	sessionID := fmt.Sprintf("ses-%s-%d", msg.KeyHash[:min(8, len(msg.KeyHash))], height)
	session := &types.ActiveSession{
		SessionId:   sessionID,
		HomeId:      msg.HomeId,
		KeyHash:     msg.KeyHash,
		Permissions: granted,
		StartedAt:   height,
		ExpiresAt:   height + params.SessionTimeoutBlocks,
	}

	k.SetSession(ctx, session)

	// Update key's LastUsedAt.
	reg.LastUsedAt = height
	k.SetKeyRegistration(ctx, msg.HomeId, reg)

	// Update home's last active block.
	home.LastActiveBlock = height
	k.SetHome(ctx, home)

	// Commitment 7 (skill is current, not historical): session start
	// marks the moment the agent is currently authorised. Past
	// sessions are part of the queryable record (commitment 10), but
	// the panel-weight / spending-power surface reads from the active
	// session's current state, not from the agent's history of having
	// once been in good standing.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.session_started",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("session_id", sessionID),
			sdk.NewAttribute("key_hash", msg.KeyHash),
			sdk.NewAttribute("creed_commitment", "7"),
		),
	)

	return &types.MsgStartSessionResponse{SessionId: sessionID}, nil
}

// intersectPermissions returns the intersection of available and requested permissions.
func intersectPermissions(available, requested []string) []string {
	if len(requested) == 0 {
		return available
	}
	avail := make(map[string]bool, len(available))
	for _, p := range available {
		avail[p] = true
	}
	var result []string
	for _, p := range requested {
		if avail[p] {
			result = append(result, p)
		}
	}
	return result
}

// EndSession ends an active session.
func (k msgServer) EndSession(goCtx context.Context, msg *types.MsgEndSession) (*types.MsgEndSessionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	_, found := k.GetSession(ctx, msg.HomeId, msg.SessionId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrSessionNotFound, msg.SessionId)
	}

	k.DeleteSession(ctx, msg.HomeId, msg.SessionId)

	// Commitment 7 (skill is current, not historical): session end
	// is the chain dropping the agent's currently-authorised status.
	// The audit record of the session remains, but the agent now
	// reads as "not currently live" — past activity does not extend
	// authorisation forward.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.session_ended",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("session_id", msg.SessionId),
			sdk.NewAttribute("creed_commitment", "7"),
		),
	)

	return &types.MsgEndSessionResponse{}, nil
}

// RegisterKey registers a new key for a home.
func (k msgServer) RegisterKey(goCtx context.Context, msg *types.MsgRegisterKey) (*types.MsgRegisterKeyResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	height := uint64(ctx.BlockHeight())

	if err := validateKeyHash(msg.KeyHash); err != nil {
		return nil, err
	}

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}
	if home.OwnerAddress != msg.Owner {
		return nil, fmt.Errorf("%w: not the home owner", types.ErrUnauthorized)
	}

	// Check for duplicate key.
	if _, exists := k.GetKeyRegistration(ctx, msg.HomeId, msg.KeyHash); exists {
		return nil, fmt.Errorf("%w: %s", types.ErrKeyAlreadyRegistered, msg.KeyHash)
	}

	// Enforce max keys.
	params := k.GetParams(ctx)
	if k.CountActiveKeys(ctx, msg.HomeId) >= params.MaxKeysPerHome {
		return nil, fmt.Errorf("%w: limit %d", types.ErrMaxKeysReached, params.MaxKeysPerHome)
	}

	reg := &types.KeyRegistration{
		KeyHash:      msg.KeyHash,
		KeyType:      msg.KeyType,
		Role:         msg.Role,
		Permissions:  msg.Permissions,
		RegisteredAt: height,
		ExpiresAt:    msg.ExpiresAt,
	}

	k.SetKeyRegistration(ctx, msg.HomeId, reg)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.key_registered",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("key_hash", msg.KeyHash),
			sdk.NewAttribute("key_type", msg.KeyType),
			sdk.NewAttribute("role", msg.Role),
		),
	)

	return &types.MsgRegisterKeyResponse{}, nil
}

// RevokeKey revokes a registered key and ends its active sessions.
func (k msgServer) RevokeKey(goCtx context.Context, msg *types.MsgRevokeKey) (*types.MsgRevokeKeyResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	height := uint64(ctx.BlockHeight())

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}
	if home.OwnerAddress != msg.Owner {
		return nil, fmt.Errorf("%w: not the home owner", types.ErrUnauthorized)
	}

	reg, found := k.GetKeyRegistration(ctx, msg.HomeId, msg.KeyHash)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrKeyNotFound, msg.KeyHash)
	}

	// Mark as revoked.
	reg.Revoked = true
	reg.RevokedAt = height
	k.SetKeyRegistration(ctx, msg.HomeId, reg)

	// End active sessions using this key.
	k.IterateSessions(ctx, msg.HomeId, func(s *types.ActiveSession) bool {
		if s.KeyHash == msg.KeyHash {
			k.DeleteSession(ctx, msg.HomeId, s.SessionId)
		}
		return false
	})

	// Create alert (silently skip if limit reached — revocation must not be blocked).
	alertID := fmt.Sprintf("key-revoked-%s-%d", msg.KeyHash[:min(8, len(msg.KeyHash))], height)
	alert := &types.Alert{
		AlertId:   alertID,
		HomeId:    msg.HomeId,
		AlertType: "key_revoked",
		Priority:  "medium",
		Message:   fmt.Sprintf("Key %s has been revoked", msg.KeyHash),
		CreatedAt: height,
	}
	k.SetAlertWithLimit(ctx, alert)

	// Commitment 7 (skill is current, not historical): key revocation
	// is the chain refusing the previously-authorised key NOW, even
	// though it was authorised before. Authorisation does not survive
	// revocation by virtue of having existed; current standing is
	// what the chain reads.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.key_revoked",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("key_hash", msg.KeyHash),
			sdk.NewAttribute("creed_commitment", "7"),
		),
	)

	return &types.MsgRevokeKeyResponse{}, nil
}

// ConfigureGuardian updates the guardian configuration.
func (k msgServer) ConfigureGuardian(goCtx context.Context, msg *types.MsgConfigureGuardian) (*types.MsgConfigureGuardianResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}
	if home.OwnerAddress != msg.Owner {
		return nil, fmt.Errorf("%w: not the home owner", types.ErrUnauthorized)
	}

	params := k.GetParams(ctx)

	// Validate recovery addresses.
	if uint64(len(msg.RecoveryAddresses)) > params.MaxRecoveryAddresses {
		return nil, fmt.Errorf("%w: too many recovery addresses (max %d)", types.ErrInvalidGuardianConfig, params.MaxRecoveryAddresses)
	}
	for _, addr := range msg.RecoveryAddresses {
		if _, err := sdk.AccAddressFromBech32(addr); err != nil {
			return nil, fmt.Errorf("%w: invalid recovery address %q: %v", types.ErrInvalidInput, addr, err)
		}
	}
	if msg.RecoveryThreshold > uint32(len(msg.RecoveryAddresses)) {
		return nil, fmt.Errorf("%w: recovery_threshold exceeds number of recovery_addresses", types.ErrInvalidGuardianConfig)
	}
	if msg.GuardianAddress != "" {
		if _, err := sdk.AccAddressFromBech32(msg.GuardianAddress); err != nil {
			return nil, fmt.Errorf("%w: invalid guardian address %q: %v", types.ErrInvalidInput, msg.GuardianAddress, err)
		}
	}

	// Validate deadman config.
	if msg.Deadman != nil && msg.Deadman.Enabled {
		if msg.Deadman.InactivityThreshold < params.DeadmanMinThreshold {
			return nil, fmt.Errorf("%w: inactivity_threshold below minimum %d", types.ErrInvalidDeadmanConfig, params.DeadmanMinThreshold)
		}
		if msg.Deadman.InactivityThreshold > params.DeadmanMaxThreshold {
			return nil, fmt.Errorf("%w: inactivity_threshold above maximum %d", types.ErrInvalidDeadmanConfig, params.DeadmanMaxThreshold)
		}
	}

	// Validate defense strategy.
	validStrategies := map[string]bool{
		"aggressive":   true,
		"moderate":     true,
		"conservative": true,
		"diplomatic":   true,
	}
	if msg.DefenseStrategy != "" && !validStrategies[msg.DefenseStrategy] {
		return nil, fmt.Errorf("%w: invalid defense_strategy %q", types.ErrInvalidGuardianConfig, msg.DefenseStrategy)
	}

	if home.Guardian == nil {
		home.Guardian = &types.HomeGuardian{}
	}

	if msg.DefenseStrategy != "" {
		home.Guardian.DefenseStrategy = msg.DefenseStrategy
	}
	home.Guardian.AutoDefend = msg.AutoDefend
	if msg.Deadman != nil {
		home.Guardian.Deadman = msg.Deadman
	}
	if msg.RecoveryAddresses != nil {
		home.Guardian.RecoveryAddresses = msg.RecoveryAddresses
		home.Guardian.RecoveryThreshold = msg.RecoveryThreshold
	}
	if msg.GuardianAddress != "" {
		home.Guardian.GuardianAddress = msg.GuardianAddress
	}

	home.LastActiveBlock = uint64(ctx.BlockHeight())
	k.SetHome(ctx, home)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.guardian_configured",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("defense_strategy", home.Guardian.DefenseStrategy),
			sdk.NewAttribute("auto_defend", fmt.Sprintf("%t", home.Guardian.AutoDefend)),
		),
	)

	return &types.MsgConfigureGuardianResponse{}, nil
}

// AcknowledgeAlert acknowledges an alert.
func (k msgServer) AcknowledgeAlert(goCtx context.Context, msg *types.MsgAcknowledgeAlert) (*types.MsgAcknowledgeAlertResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}

	// Owner or guardian can acknowledge.
	if home.OwnerAddress != msg.Signer {
		if home.Guardian == nil || home.Guardian.GuardianAddress != msg.Signer {
			return nil, fmt.Errorf("%w: only owner or guardian can acknowledge alerts", types.ErrUnauthorized)
		}
	}

	alert, found := k.GetAlert(ctx, msg.HomeId, msg.AlertId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrAlertNotFound, msg.AlertId)
	}

	alert.Acknowledged = true
	k.SetAlert(ctx, alert)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.alert_acknowledged",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("alert_id", msg.AlertId),
			sdk.NewAttribute("signer", msg.Signer),
		),
	)

	return &types.MsgAcknowledgeAlertResponse{}, nil
}

// SetSpendingLimit sets a spending limit for a key type on a home.
func (k msgServer) SetSpendingLimit(goCtx context.Context, msg *types.MsgSetSpendingLimit) (*types.MsgSetSpendingLimitResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	height := uint64(ctx.BlockHeight())

	home, found := k.GetHome(ctx, msg.HomeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrHomeNotFound, msg.HomeId)
	}
	if home.OwnerAddress != msg.Owner {
		return nil, fmt.Errorf("%w: not the home owner", types.ErrUnauthorized)
	}

	// Validate amount.
	maxAmt := new(big.Int)
	if _, ok := maxAmt.SetString(msg.MaxAmount, 10); !ok || maxAmt.Sign() <= 0 {
		return nil, fmt.Errorf("%w: %s", types.ErrInvalidAmount, msg.MaxAmount)
	}

	if msg.PeriodBlocks == 0 {
		return nil, fmt.Errorf("%w: period_blocks must be positive", types.ErrInvalidAmount)
	}

	limit := &types.SpendingLimit{
		KeyType:       msg.KeyType,
		MaxAmount:     msg.MaxAmount,
		PeriodBlocks:  msg.PeriodBlocks,
		SpentInPeriod: "0",
		PeriodStart:   height,
	}

	k.Keeper.SetSpendingLimit(ctx, msg.HomeId, limit)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.home.spending_limit_set",
			sdk.NewAttribute("home_id", msg.HomeId),
			sdk.NewAttribute("owner", msg.Owner),
			sdk.NewAttribute("key_type", msg.KeyType),
			sdk.NewAttribute("max_amount", msg.MaxAmount),
			sdk.NewAttribute("period_blocks", fmt.Sprintf("%d", msg.PeriodBlocks)),
		),
	)

	return &types.MsgSetSpendingLimitResponse{}, nil
}

// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	k.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.home.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
