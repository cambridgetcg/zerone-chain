// graph_server.go — Knowledge graph message handlers (R46).
// Wraps Keeper methods for on-chain knowledge edge management.
// When proto-gen adds these to the main MsgServer, move to msg_server.go.
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GraphMsgServer handles knowledge graph messages.
type GraphMsgServer interface {
	CreateEdge(context.Context, *types.MsgCreateEdge) (*types.MsgCreateEdgeResponse, error)
	RemoveEdge(context.Context, *types.MsgRemoveEdge) error
}

type graphMsgServer struct {
	keeper Keeper
}

// NewGraphMsgServerImpl returns a GraphMsgServer backed by the knowledge Keeper.
func NewGraphMsgServerImpl(keeper Keeper) GraphMsgServer {
	return &graphMsgServer{keeper: keeper}
}

func (s *graphMsgServer) CreateEdge(ctx context.Context, msg *types.MsgCreateEdge) (*types.MsgCreateEdgeResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	return s.keeper.CreateEdge(ctx, msg)
}

func (s *graphMsgServer) RemoveEdge(ctx context.Context, msg *types.MsgRemoveEdge) error {
	if msg.Authority == "" || msg.EdgeID == "" {
		return types.ErrUnauthorized.Wrap("authority and edge_id required")
	}
	return s.keeper.RemoveEdge(ctx, msg.EdgeID, msg.Authority)
}
