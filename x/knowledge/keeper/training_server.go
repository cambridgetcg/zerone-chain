// training_server.go — Training message handlers for T6-3.
// These wrap Keeper training methods for on-chain training attestation recording.
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TrainingMsgServer is a temporary server interface for training messages.
// When proto-gen adds RecordTraining to the main MsgServer interface, this
// method should be moved to msg_server.go.
type TrainingMsgServer interface {
	RecordTraining(context.Context, *types.MsgRecordTraining) (*types.MsgRecordTrainingResponse, error)
}

type trainingMsgServer struct {
	keeper Keeper
}

// NewTrainingMsgServerImpl returns a TrainingMsgServer backed by the knowledge Keeper.
func NewTrainingMsgServerImpl(keeper Keeper) TrainingMsgServer {
	return &trainingMsgServer{keeper: keeper}
}

func (s *trainingMsgServer) RecordTraining(ctx context.Context, msg *types.MsgRecordTraining) (*types.MsgRecordTrainingResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return s.keeper.RecordTraining(ctx, msg)
}
