// msg_attest_storage.go — Proto-compatible MsgAttestStorage type.
// Validators submit proof-of-storage attestations for their assigned TDU shards.
package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

// MsgAttestStorage is the message for submitting proof-of-storage attestations.
type MsgAttestStorage struct {
	Validator      string `protobuf:"bytes,1,opt,name=validator,proto3" json:"validator,omitempty"`
	AttestationHex string `protobuf:"bytes,2,opt,name=attestation_hex,json=attestationHex,proto3" json:"attestation_hex,omitempty"`
	SnapshotHeight int64  `protobuf:"varint,3,opt,name=snapshot_height,json=snapshotHeight,proto3" json:"snapshot_height,omitempty"`
}

func (m *MsgAttestStorage) Reset()         { *m = MsgAttestStorage{} }
func (m *MsgAttestStorage) String() string { return fmt.Sprintf("%+v", *m) }
func (m *MsgAttestStorage) ProtoMessage()  {}

func (m *MsgAttestStorage) Marshal() ([]byte, error) {
	buf := make([]byte, 0, m.Size())
	buf = protoAppendStringField(buf, 1, m.Validator)
	buf = protoAppendStringField(buf, 2, m.AttestationHex)
	buf = protoAppendInt64Field(buf, 3, m.SnapshotHeight)
	return buf, nil
}

func (m *MsgAttestStorage) MarshalTo(dAtA []byte) (int, error) {
	data, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, data)
	return len(data), nil
}

func (m *MsgAttestStorage) Size() int {
	var n int
	n += protoSizeStringField(1, m.Validator)
	n += protoSizeStringField(2, m.AttestationHex)
	n += protoSizeInt64Field(3, m.SnapshotHeight)
	return n
}

func (m *MsgAttestStorage) Unmarshal(dAtA []byte) error {
	offset := 0
	for offset < len(dAtA) {
		fieldNum, wireType, newOffset, err := protoConsumeTag(dAtA, offset)
		if err != nil {
			return err
		}
		offset = newOffset

		switch fieldNum {
		case 1: // validator
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.Validator = string(raw)
			offset = newOff
		case 2: // attestation_hex
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.AttestationHex = string(raw)
			offset = newOff
		case 3: // snapshot_height
			v, newOff, err := protoConsumeVarint(dAtA, offset)
			if err != nil {
				return err
			}
			m.SnapshotHeight = int64(v)
			offset = newOff
		default:
			newOff, err := protoSkipField(dAtA, offset, wireType)
			if err != nil {
				return err
			}
			offset = newOff
		}
	}
	return nil
}

func (m *MsgAttestStorage) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Validator)
	if err != nil {
		return nil
	}
	return []sdk.AccAddress{addr}
}

func (m *MsgAttestStorage) ValidateBasic() error {
	if m.Validator == "" {
		return ErrInvalidAttestation.Wrap("validator is required")
	}
	if m.AttestationHex == "" {
		return ErrInvalidAttestation.Wrap("attestation hex is required")
	}
	if m.SnapshotHeight <= 0 {
		return ErrInvalidAttestation.Wrap("snapshot height must be positive")
	}
	return nil
}

// MsgAttestStorageResponse is the response for MsgAttestStorage.
type MsgAttestStorageResponse struct{}

func (m *MsgAttestStorageResponse) Reset()         { *m = MsgAttestStorageResponse{} }
func (m *MsgAttestStorageResponse) String() string { return "MsgAttestStorageResponse{}" }
func (m *MsgAttestStorageResponse) ProtoMessage()  {}

// RegisterMsgAttestStorageProto registers MsgAttestStorage with the gogo proto registry.
// Called from proto_register.go init().
func RegisterMsgAttestStorageProto() {
	proto.RegisterType((*MsgAttestStorage)(nil), "zerone.knowledge.v1.MsgAttestStorage")
	proto.RegisterType((*MsgAttestStorageResponse)(nil), "zerone.knowledge.v1.MsgAttestStorageResponse")
}
