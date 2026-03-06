// tee_msgs.go — Manual message types for TEE attestation (T6-1).
// These mirror the proto definitions in tx.proto and query.proto.
// When make proto-gen succeeds (BSR available), the proto-generated types in
// tx.pb.go / query.pb.go will take precedence, and this file should be removed.
package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── TX messages ─────────────────────────────────────────────────────────────

type MsgRegisterEnclave struct {
	Operator     string `protobuf:"bytes,1,opt,name=operator,proto3" json:"operator,omitempty"`
	Attestation  []byte `protobuf:"bytes,2,opt,name=attestation,proto3" json:"attestation,omitempty"`
	Provider     string `protobuf:"bytes,3,opt,name=provider,proto3" json:"provider,omitempty"`
	Measurements []byte `protobuf:"bytes,4,opt,name=measurements,proto3" json:"measurements,omitempty"`
}

func (m *MsgRegisterEnclave) Reset()         {}
func (m *MsgRegisterEnclave) String() string { return fmt.Sprintf("MsgRegisterEnclave{%s}", m.Operator) }
func (m *MsgRegisterEnclave) ProtoMessage()  {}
func (m *MsgRegisterEnclave) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Operator)
	return []sdk.AccAddress{addr}
}

func (m *MsgRegisterEnclave) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if !ValidTEEProviders[m.Provider] {
		return ErrInvalidTEEProvider.Wrapf("unknown provider: %s", m.Provider)
	}
	if len(m.Attestation) == 0 {
		return ErrInvalidAttestation.Wrap("attestation must not be empty")
	}
	if len(m.Measurements) == 0 {
		return ErrTEEMeasurementMismatch.Wrap("measurements must not be empty")
	}
	return nil
}

type MsgRegisterEnclaveResponse struct {
	EnclaveId string `protobuf:"bytes,1,opt,name=enclave_id,json=enclaveId,proto3" json:"enclave_id,omitempty"`
}

func (m *MsgRegisterEnclaveResponse) Reset()         {}
func (m *MsgRegisterEnclaveResponse) String() string { return "MsgRegisterEnclaveResponse" }
func (m *MsgRegisterEnclaveResponse) ProtoMessage()  {}

type MsgVerifyAttestation struct {
	Verifier    string `protobuf:"bytes,1,opt,name=verifier,proto3" json:"verifier,omitempty"`
	Operator    string `protobuf:"bytes,2,opt,name=operator,proto3" json:"operator,omitempty"`
	Attestation []byte `protobuf:"bytes,3,opt,name=attestation,proto3" json:"attestation,omitempty"`
}

func (m *MsgVerifyAttestation) Reset()         {}
func (m *MsgVerifyAttestation) String() string { return fmt.Sprintf("MsgVerifyAttestation{%s}", m.Operator) }
func (m *MsgVerifyAttestation) ProtoMessage()  {}
func (m *MsgVerifyAttestation) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Verifier)
	return []sdk.AccAddress{addr}
}

func (m *MsgVerifyAttestation) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Verifier); err != nil {
		return fmt.Errorf("invalid verifier address: %w", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if len(m.Attestation) == 0 {
		return ErrInvalidAttestation.Wrap("attestation must not be empty")
	}
	return nil
}

type MsgVerifyAttestationResponse struct {
	Valid bool `protobuf:"varint,1,opt,name=valid,proto3" json:"valid,omitempty"`
}

func (m *MsgVerifyAttestationResponse) Reset()         {}
func (m *MsgVerifyAttestationResponse) String() string { return "MsgVerifyAttestationResponse" }
func (m *MsgVerifyAttestationResponse) ProtoMessage()  {}

type MsgSuspendEnclave struct {
	Authority string `protobuf:"bytes,1,opt,name=authority,proto3" json:"authority,omitempty"`
	Operator  string `protobuf:"bytes,2,opt,name=operator,proto3" json:"operator,omitempty"`
	Reason    string `protobuf:"bytes,3,opt,name=reason,proto3" json:"reason,omitempty"`
}

func (m *MsgSuspendEnclave) Reset()         {}
func (m *MsgSuspendEnclave) String() string { return fmt.Sprintf("MsgSuspendEnclave{%s}", m.Operator) }
func (m *MsgSuspendEnclave) ProtoMessage()  {}
func (m *MsgSuspendEnclave) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

type MsgSuspendEnclaveResponse struct{}

func (m *MsgSuspendEnclaveResponse) Reset()         {}
func (m *MsgSuspendEnclaveResponse) String() string { return "MsgSuspendEnclaveResponse" }
func (m *MsgSuspendEnclaveResponse) ProtoMessage()  {}

type MsgRevokeEnclave struct {
	Authority string `protobuf:"bytes,1,opt,name=authority,proto3" json:"authority,omitempty"`
	Operator  string `protobuf:"bytes,2,opt,name=operator,proto3" json:"operator,omitempty"`
	Reason    string `protobuf:"bytes,3,opt,name=reason,proto3" json:"reason,omitempty"`
}

func (m *MsgRevokeEnclave) Reset()         {}
func (m *MsgRevokeEnclave) String() string { return fmt.Sprintf("MsgRevokeEnclave{%s}", m.Operator) }
func (m *MsgRevokeEnclave) ProtoMessage()  {}
func (m *MsgRevokeEnclave) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

type MsgRevokeEnclaveResponse struct{}

func (m *MsgRevokeEnclaveResponse) Reset()         {}
func (m *MsgRevokeEnclaveResponse) String() string { return "MsgRevokeEnclaveResponse" }
func (m *MsgRevokeEnclaveResponse) ProtoMessage()  {}
