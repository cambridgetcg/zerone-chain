package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ─── TEE message types (T6-1) ───────────────────────────────────────────────
// Manually implemented pending proto-gen integration.

// MsgRegisterEnclave registers a TEE enclave on-chain.
type MsgRegisterEnclave struct {
	Operator     string `json:"operator"`
	Provider     string `json:"provider"`
	Attestation  []byte `json:"attestation"`
	Measurements []byte `json:"measurements"`
}

func (m *MsgRegisterEnclave) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if m.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if !ValidTEEProviders[m.Provider] {
		return fmt.Errorf("unsupported TEE provider: %s", m.Provider)
	}
	if len(m.Attestation) == 0 {
		return fmt.Errorf("attestation is required")
	}
	if len(m.Measurements) == 0 {
		return fmt.Errorf("measurements are required")
	}
	return nil
}

func (m *MsgRegisterEnclave) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Operator)
	return []sdk.AccAddress{addr}
}

func (*MsgRegisterEnclave) ProtoMessage()           {}
func (*MsgRegisterEnclave) Reset()                  {}
func (*MsgRegisterEnclave) String() string          { return "MsgRegisterEnclave" }
func (*MsgRegisterEnclave) XXX_MessageName() string { return "zerone.knowledge.v1.MsgRegisterEnclave" }

type MsgRegisterEnclaveResponse struct {
	EnclaveId string `json:"enclave_id"`
}

func (*MsgRegisterEnclaveResponse) ProtoMessage()  {}
func (*MsgRegisterEnclaveResponse) Reset()         {}
func (*MsgRegisterEnclaveResponse) String() string { return "MsgRegisterEnclaveResponse" }

// MsgVerifyAttestation verifies a TEE enclave's attestation.
type MsgVerifyAttestation struct {
	Operator    string `json:"operator"`
	Attestation []byte `json:"attestation"`
}

func (m *MsgVerifyAttestation) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	if len(m.Attestation) == 0 {
		return fmt.Errorf("attestation is required")
	}
	return nil
}

func (m *MsgVerifyAttestation) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Operator)
	return []sdk.AccAddress{addr}
}

func (*MsgVerifyAttestation) ProtoMessage()           {}
func (*MsgVerifyAttestation) Reset()                  {}
func (*MsgVerifyAttestation) String() string          { return "MsgVerifyAttestation" }
func (*MsgVerifyAttestation) XXX_MessageName() string { return "zerone.knowledge.v1.MsgVerifyAttestation" }

type MsgVerifyAttestationResponse struct {
	Valid bool `json:"valid"`
}

func (*MsgVerifyAttestationResponse) ProtoMessage()  {}
func (*MsgVerifyAttestationResponse) Reset()         {}
func (*MsgVerifyAttestationResponse) String() string { return "MsgVerifyAttestationResponse" }

// MsgSuspendEnclave suspends a TEE enclave (governance only).
type MsgSuspendEnclave struct {
	Authority string `json:"authority"`
	Operator  string `json:"operator"`
}

func (m *MsgSuspendEnclave) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	return nil
}

func (m *MsgSuspendEnclave) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (*MsgSuspendEnclave) ProtoMessage()           {}
func (*MsgSuspendEnclave) Reset()                  {}
func (*MsgSuspendEnclave) String() string          { return "MsgSuspendEnclave" }
func (*MsgSuspendEnclave) XXX_MessageName() string { return "zerone.knowledge.v1.MsgSuspendEnclave" }

type MsgSuspendEnclaveResponse struct{}

func (*MsgSuspendEnclaveResponse) ProtoMessage()  {}
func (*MsgSuspendEnclaveResponse) Reset()         {}
func (*MsgSuspendEnclaveResponse) String() string { return "MsgSuspendEnclaveResponse" }

// MsgRevokeEnclave permanently revokes a TEE enclave (governance only).
type MsgRevokeEnclave struct {
	Authority string `json:"authority"`
	Operator  string `json:"operator"`
}

func (m *MsgRevokeEnclave) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return fmt.Errorf("invalid authority address: %w", err)
	}
	if _, err := sdk.AccAddressFromBech32(m.Operator); err != nil {
		return fmt.Errorf("invalid operator address: %w", err)
	}
	return nil
}

func (m *MsgRevokeEnclave) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Authority)
	return []sdk.AccAddress{addr}
}

func (*MsgRevokeEnclave) ProtoMessage()           {}
func (*MsgRevokeEnclave) Reset()                  {}
func (*MsgRevokeEnclave) String() string          { return "MsgRevokeEnclave" }
func (*MsgRevokeEnclave) XXX_MessageName() string { return "zerone.knowledge.v1.MsgRevokeEnclave" }

type MsgRevokeEnclaveResponse struct{}

func (*MsgRevokeEnclaveResponse) ProtoMessage()  {}
func (*MsgRevokeEnclaveResponse) Reset()         {}
func (*MsgRevokeEnclaveResponse) String() string { return "MsgRevokeEnclaveResponse" }
