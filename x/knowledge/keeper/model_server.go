// model_server.go — Model registry + agent promotion message handlers (R45).
// Wraps Keeper methods for on-chain model lifecycle and agent promotion.
// When proto-gen adds these to the main MsgServer, move to msg_server.go.
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ModelMsgServer handles model registry and agent promotion messages.
type ModelMsgServer interface {
	PublishModel(context.Context, *types.MsgPublishModel) (*types.MsgPublishModelResponse, error)
	DeprecateModel(context.Context, *types.MsgDeprecateModel) (*types.MsgDeprecateModelResponse, error)
	PromoteModelToAgent(context.Context, *types.MsgPromoteModel) (*types.MsgPromoteModelResponse, error)
}

type modelMsgServer struct {
	keeper Keeper
}

// NewModelMsgServerImpl returns a ModelMsgServer backed by the knowledge Keeper.
func NewModelMsgServerImpl(keeper Keeper) ModelMsgServer {
	return &modelMsgServer{keeper: keeper}
}

func (s *modelMsgServer) PublishModel(ctx context.Context, msg *types.MsgPublishModel) (*types.MsgPublishModelResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return s.keeper.PublishModel(ctx, msg)
}

func (s *modelMsgServer) DeprecateModel(ctx context.Context, msg *types.MsgDeprecateModel) (*types.MsgDeprecateModelResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if err := s.keeper.DeprecateModel(ctx, msg.ModelID, msg.Authority, msg.Reason); err != nil {
		return nil, err
	}
	return &types.MsgDeprecateModelResponse{}, nil
}

func (s *modelMsgServer) PromoteModelToAgent(ctx context.Context, msg *types.MsgPromoteModel) (*types.MsgPromoteModelResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return s.keeper.PromoteModelToAgent(ctx, msg)
}
