package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/private_corpus/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	keeper Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{keeper: k}
}

var _ types.MsgServer = &msgServer{}

func (m *msgServer) RegisterVault(ctx context.Context, msg *types.MsgRegisterVault) (*types.MsgRegisterVaultResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	params := m.keeper.GetParams(ctx)
	if !params.RegistrationEnabled {
		return nil, types.ErrRegistrationDisabled
	}
	if uint32(len(msg.Description)) > params.MaxDescriptionBytes {
		return nil, fmt.Errorf("%w: description %d > max %d", types.ErrDescriptionTooLong, len(msg.Description), params.MaxDescriptionBytes)
	}
	if _, exists := m.keeper.GetVault(ctx, msg.Id); exists {
		return nil, fmt.Errorf("%w: %s", types.ErrVaultExists, msg.Id)
	}

	height := CurrentBlock(ctx)
	v := &types.Vault{
		Id:                msg.Id,
		Operator:          msg.Operator,
		DisplayName:       msg.DisplayName,
		Description:       msg.Description,
		AccessPolicyUrl:   msg.AccessPolicyUrl,
		OperatorPubkey:    msg.OperatorPubkey,
		ServerEndpoint:    msg.ServerEndpoint,
		Status:            types.VaultStatus_VAULT_STATUS_ACTIVE,
		RegisteredAtBlock: height,
		UpdatedAtBlock:    0,
	}
	if err := m.keeper.SetVault(ctx, v); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.private_corpus.vault_registered",
		sdk.NewAttribute("vault_id", v.Id),
		sdk.NewAttribute("operator", v.Operator),
		sdk.NewAttribute("display_name", v.DisplayName),
		sdk.NewAttribute("registered_at_block", fmt.Sprintf("%d", height)),
	))
	return &types.MsgRegisterVaultResponse{VaultId: v.Id}, nil
}

func (m *msgServer) UpdateVault(ctx context.Context, msg *types.MsgUpdateVault) (*types.MsgUpdateVaultResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	params := m.keeper.GetParams(ctx)
	v, ok := m.keeper.GetVault(ctx, msg.VaultId)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrVaultNotFound, msg.VaultId)
	}
	if v.Operator != msg.Operator {
		return nil, types.ErrNotVaultOperator
	}
	if v.Status == types.VaultStatus_VAULT_STATUS_DEPRECATED {
		return nil, fmt.Errorf("%w: vault is deprecated", types.ErrVaultNotActive)
	}

	if msg.DisplayName != "" {
		v.DisplayName = msg.DisplayName
	}
	if msg.Description != "" {
		if uint32(len(msg.Description)) > params.MaxDescriptionBytes {
			return nil, fmt.Errorf("%w: description %d > max %d", types.ErrDescriptionTooLong, len(msg.Description), params.MaxDescriptionBytes)
		}
		v.Description = msg.Description
	}
	if msg.AccessPolicyUrl != "" {
		v.AccessPolicyUrl = msg.AccessPolicyUrl
	}
	if msg.OperatorPubkey != "" {
		v.OperatorPubkey = msg.OperatorPubkey
	}
	if msg.ServerEndpoint != "" {
		v.ServerEndpoint = msg.ServerEndpoint
	}
	if msg.Status != types.VaultStatus_VAULT_STATUS_UNSPECIFIED {
		v.Status = msg.Status
	}
	v.UpdatedAtBlock = CurrentBlock(ctx)
	if err := m.keeper.SetVault(ctx, v); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.private_corpus.vault_updated",
		sdk.NewAttribute("vault_id", v.Id),
		sdk.NewAttribute("status", v.Status.String()),
		sdk.NewAttribute("updated_at_block", fmt.Sprintf("%d", v.UpdatedAtBlock)),
	))
	return &types.MsgUpdateVaultResponse{}, nil
}

func (m *msgServer) PublishManifest(ctx context.Context, msg *types.MsgPublishManifest) (*types.MsgPublishManifestResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	params := m.keeper.GetParams(ctx)
	v, ok := m.keeper.GetVault(ctx, msg.VaultId)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrVaultNotFound, msg.VaultId)
	}
	if v.Operator != msg.Operator {
		return nil, types.ErrNotVaultOperator
	}
	if v.Status != types.VaultStatus_VAULT_STATUS_ACTIVE {
		return nil, fmt.Errorf("%w: status=%s", types.ErrVaultNotActive, v.Status.String())
	}
	if uint32(len(msg.Description)) > params.MaxManifestDescriptionBytes {
		return nil, fmt.Errorf("%w: description %d > max %d", types.ErrDescriptionTooLong, len(msg.Description), params.MaxManifestDescriptionBytes)
	}
	if _, exists := m.keeper.GetManifest(ctx, msg.ManifestId); exists {
		return nil, fmt.Errorf("%w: %s", types.ErrManifestExists, msg.ManifestId)
	}

	height := CurrentBlock(ctx)
	mf := &types.CorpusManifest{
		Id:               msg.ManifestId,
		VaultId:          msg.VaultId,
		Version:          msg.Version,
		ContentHash:      msg.ContentHash,
		ItemCount:        msg.ItemCount,
		SizeBytes:        msg.SizeBytes,
		Description:      msg.Description,
		Status:           types.ManifestStatus_MANIFEST_STATUS_PUBLISHED,
		PublishedAtBlock: height,
		PublishedBy:      msg.Operator,
	}
	if err := m.keeper.SetManifest(ctx, mf); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.private_corpus.manifest_published",
		sdk.NewAttribute("manifest_id", mf.Id),
		sdk.NewAttribute("vault_id", mf.VaultId),
		sdk.NewAttribute("version", mf.Version),
		sdk.NewAttribute("content_hash", mf.ContentHash),
		sdk.NewAttribute("published_at_block", fmt.Sprintf("%d", height)),
	))
	return &types.MsgPublishManifestResponse{ManifestId: mf.Id}, nil
}

func (m *msgServer) WithdrawManifest(ctx context.Context, msg *types.MsgWithdrawManifest) (*types.MsgWithdrawManifestResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	mf, ok := m.keeper.GetManifest(ctx, msg.ManifestId)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrManifestNotFound, msg.ManifestId)
	}
	v, vaultExists := m.keeper.GetVault(ctx, mf.VaultId)
	if !vaultExists {
		return nil, fmt.Errorf("%w: vault %s", types.ErrVaultNotFound, mf.VaultId)
	}
	if v.Operator != msg.Operator {
		return nil, types.ErrNotVaultOperator
	}
	if mf.Status == types.ManifestStatus_MANIFEST_STATUS_WITHDRAWN {
		return nil, types.ErrManifestAlreadyWithdrawn
	}

	height := CurrentBlock(ctx)
	mf.Status = types.ManifestStatus_MANIFEST_STATUS_WITHDRAWN
	mf.WithdrawnAtBlock = height
	mf.WithdrawnReason = msg.Reason
	if err := m.keeper.SetManifest(ctx, mf); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.private_corpus.manifest_withdrawn",
		sdk.NewAttribute("manifest_id", mf.Id),
		sdk.NewAttribute("vault_id", mf.VaultId),
		sdk.NewAttribute("reason", msg.Reason),
		sdk.NewAttribute("withdrawn_at_block", fmt.Sprintf("%d", height)),
	))
	return &types.MsgWithdrawManifestResponse{}, nil
}

func (m *msgServer) RecordAccess(ctx context.Context, msg *types.MsgRecordAccess) (*types.MsgRecordAccessResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	params := m.keeper.GetParams(ctx)
	v, ok := m.keeper.GetVault(ctx, msg.VaultId)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrVaultNotFound, msg.VaultId)
	}
	if v.Operator != msg.Operator {
		return nil, types.ErrNotVaultOperator
	}
	if uint32(len(msg.Note)) > params.MaxAccessNoteBytes {
		return nil, fmt.Errorf("%w: note %d > max %d", types.ErrNoteTooLong, len(msg.Note), params.MaxAccessNoteBytes)
	}
	if msg.ManifestId != "" {
		mf, manifestExists := m.keeper.GetManifest(ctx, msg.ManifestId)
		if !manifestExists {
			return nil, fmt.Errorf("%w: %s", types.ErrManifestNotFound, msg.ManifestId)
		}
		if mf.VaultId != msg.VaultId {
			return nil, types.ErrManifestVaultMismatch
		}
	}

	seq, err := m.keeper.NextAccessSeq(ctx)
	if err != nil {
		return nil, err
	}
	height := CurrentBlock(ctx)
	rec := &types.AccessRecord{
		Seq:             seq,
		VaultId:         msg.VaultId,
		ManifestId:      msg.ManifestId,
		Accessor:        msg.Accessor,
		RecordedAtBlock: height,
		Note:            msg.Note,
	}
	if err := m.keeper.SetAccessRecord(ctx, rec); err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.private_corpus.access_recorded",
		sdk.NewAttribute("seq", fmt.Sprintf("%d", seq)),
		sdk.NewAttribute("vault_id", rec.VaultId),
		sdk.NewAttribute("manifest_id", rec.ManifestId),
		sdk.NewAttribute("accessor", rec.Accessor),
		sdk.NewAttribute("recorded_at_block", fmt.Sprintf("%d", height)),
	))
	return &types.MsgRecordAccessResponse{Seq: seq}, nil
}

func (m *msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if msg.Authority != m.keeper.Authority() {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrInvalidAuthority, m.keeper.Authority(), msg.Authority)
	}
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := m.keeper.SetParams(ctx, *msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}
