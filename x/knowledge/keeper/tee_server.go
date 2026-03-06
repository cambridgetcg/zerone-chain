// tee_server.go — TEE message handlers for T6-1.
// These wrap Keeper TEE methods and will be integrated into the main MsgServer
// when proto-gen regenerates the MsgServer interface with TEE RPCs.
package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TEEMsgServer is a temporary server interface for TEE messages.
// When proto-gen adds RegisterEnclave/VerifyAttestation/SuspendEnclave/RevokeEnclave
// to the main MsgServer interface, these methods should be moved to msg_server.go.
type TEEMsgServer interface {
	RegisterEnclave(context.Context, *types.MsgRegisterEnclave) (*types.MsgRegisterEnclaveResponse, error)
	VerifyAttestation(context.Context, *types.MsgVerifyAttestation) (*types.MsgVerifyAttestationResponse, error)
	SuspendEnclave(context.Context, *types.MsgSuspendEnclave) (*types.MsgSuspendEnclaveResponse, error)
	RevokeEnclave(context.Context, *types.MsgRevokeEnclave) (*types.MsgRevokeEnclaveResponse, error)
}

type teeMsgServer struct {
	keeper Keeper
}

// NewTEEMsgServerImpl returns a TEEMsgServer backed by the knowledge Keeper.
func NewTEEMsgServerImpl(keeper Keeper) TEEMsgServer {
	return &teeMsgServer{keeper: keeper}
}

func (s *teeMsgServer) RegisterEnclave(ctx context.Context, msg *types.MsgRegisterEnclave) (*types.MsgRegisterEnclaveResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	enclaveID, err := s.keeper.RegisterEnclave(ctx, msg.Operator, msg.Provider, msg.Attestation, msg.Measurements)
	if err != nil {
		return nil, err
	}
	return &types.MsgRegisterEnclaveResponse{EnclaveId: enclaveID}, nil
}

func (s *teeMsgServer) VerifyAttestation(ctx context.Context, msg *types.MsgVerifyAttestation) (*types.MsgVerifyAttestationResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	valid, err := s.keeper.VerifyEnclave(ctx, msg.Operator, msg.Attestation)
	if err != nil {
		return nil, err
	}
	return &types.MsgVerifyAttestationResponse{Valid: valid}, nil
}

func (s *teeMsgServer) SuspendEnclave(ctx context.Context, msg *types.MsgSuspendEnclave) (*types.MsgSuspendEnclaveResponse, error) {
	if msg.Authority != s.keeper.GetAuthority() {
		return nil, types.ErrUnauthorized.Wrap("only governance can suspend enclaves")
	}
	if err := s.keeper.SuspendEnclave(ctx, msg.Operator); err != nil {
		return nil, err
	}
	return &types.MsgSuspendEnclaveResponse{}, nil
}

func (s *teeMsgServer) RevokeEnclave(ctx context.Context, msg *types.MsgRevokeEnclave) (*types.MsgRevokeEnclaveResponse, error) {
	if msg.Authority != s.keeper.GetAuthority() {
		return nil, types.ErrUnauthorized.Wrap("only governance can revoke enclaves")
	}
	if err := s.keeper.RevokeEnclave(ctx, msg.Operator); err != nil {
		return nil, err
	}
	return &types.MsgRevokeEnclaveResponse{}, nil
}
