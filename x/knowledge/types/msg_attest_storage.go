package types

// MsgAttestStorage (proto-compatible): validator proof-of-storage attestation.
// Proto definition in proto/zerone/knowledge/v1/tx.proto.
// Manually bridged because proto-gen requires network access (buf.build).
// When proto-gen runs, this file should be removed in favour of the
// generated struct in tx.pb.go.

import (
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

const (
	protoNameMsgAttestStorage         = "zerone.knowledge.v1.MsgAttestStorage"
	protoNameMsgAttestStorageResponse = "zerone.knowledge.v1.MsgAttestStorageResponse"
)

// ---------------------------------------------------------------------------
// MsgAttestStorage
// ---------------------------------------------------------------------------

// MsgAttestStorage is a proto-compatible message for storage attestation.
// Fields follow the proto definition in tx.proto:
//
//	string validator        = 1;
//	int64  snapshot_height  = 2;
//	string attestation_hex  = 3;
type MsgAttestStorage struct {
	Validator      string `protobuf:"bytes,1,opt,name=validator,proto3" json:"validator,omitempty"`
	SnapshotHeight int64  `protobuf:"varint,2,opt,name=snapshot_height,json=snapshotHeight,proto3" json:"snapshot_height,omitempty"`
	AttestationHex string `protobuf:"bytes,3,opt,name=attestation_hex,json=attestationHex,proto3" json:"attestation_hex,omitempty"`
}

func (m *MsgAttestStorage) Reset()         { *m = MsgAttestStorage{} }
func (m *MsgAttestStorage) String() string { return proto.CompactTextString(m) }
func (m *MsgAttestStorage) ProtoMessage()  {}

func (m *MsgAttestStorage) GetValidator() string {
	if m != nil {
		return m.Validator
	}
	return ""
}

func (m *MsgAttestStorage) GetSnapshotHeight() int64 {
	if m != nil {
		return m.SnapshotHeight
	}
	return 0
}

func (m *MsgAttestStorage) GetAttestationHex() string {
	if m != nil {
		return m.AttestationHex
	}
	return ""
}

// ValidateBasic implements sdk.HasValidateBasic.
func (m *MsgAttestStorage) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Validator); err != nil {
		return fmt.Errorf("invalid validator address: %w", err)
	}
	if m.SnapshotHeight <= 0 {
		return ErrInvalidAttestation.Wrap("snapshot_height must be > 0")
	}
	if len(m.AttestationHex) == 0 {
		return ErrInvalidAttestation.Wrap("attestation_hex must not be empty")
	}
	return nil
}

// Marshal implements proto.Marshaler (manual protobuf encoding).
func (m *MsgAttestStorage) Marshal() ([]byte, error) {
	var buf []byte

	// field 1: string validator (tag = 0x0a)
	if len(m.Validator) > 0 {
		buf = append(buf, 0x0a)
		buf = appendLenPrefixed(buf, []byte(m.Validator))
	}

	// field 2: int64 snapshot_height (tag = 0x10)
	if m.SnapshotHeight != 0 {
		buf = append(buf, 0x10)
		buf = appendVarint(buf, uint64(m.SnapshotHeight))
	}

	// field 3: string attestation_hex (tag = 0x1a)
	if len(m.AttestationHex) > 0 {
		buf = append(buf, 0x1a)
		buf = appendLenPrefixed(buf, []byte(m.AttestationHex))
	}

	return buf, nil
}

// MarshalTo implements proto.Marshaler.
func (m *MsgAttestStorage) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

// Size implements proto.Sizer.
func (m *MsgAttestStorage) Size() int {
	var n int
	if len(m.Validator) > 0 {
		n += 1 + sizeVarint(uint64(len(m.Validator))) + len(m.Validator)
	}
	if m.SnapshotHeight != 0 {
		n += 1 + sizeVarint(uint64(m.SnapshotHeight))
	}
	if len(m.AttestationHex) > 0 {
		n += 1 + sizeVarint(uint64(len(m.AttestationHex))) + len(m.AttestationHex)
	}
	return n
}

// Unmarshal implements proto.Unmarshaler (manual protobuf decoding).
func (m *MsgAttestStorage) Unmarshal(dAtA []byte) error {
	i := 0
	for i < len(dAtA) {
		if i >= len(dAtA) {
			return fmt.Errorf("unexpected end of data")
		}
		tag := dAtA[i]
		i++
		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch fieldNum {
		case 1: // validator (string, wire type 2)
			if wireType != 2 {
				return fmt.Errorf("field 1: expected wire type 2, got %d", wireType)
			}
			length, n := binary.Uvarint(dAtA[i:])
			if n <= 0 {
				return fmt.Errorf("field 1: invalid length")
			}
			i += n
			if i+int(length) > len(dAtA) {
				return fmt.Errorf("field 1: data too short")
			}
			m.Validator = string(dAtA[i : i+int(length)])
			i += int(length)

		case 2: // snapshot_height (int64, wire type 0)
			if wireType != 0 {
				return fmt.Errorf("field 2: expected wire type 0, got %d", wireType)
			}
			v, n := binary.Uvarint(dAtA[i:])
			if n <= 0 {
				return fmt.Errorf("field 2: invalid varint")
			}
			m.SnapshotHeight = int64(v)
			i += n

		case 3: // attestation_hex (string, wire type 2)
			if wireType != 2 {
				return fmt.Errorf("field 3: expected wire type 2, got %d", wireType)
			}
			length, n := binary.Uvarint(dAtA[i:])
			if n <= 0 {
				return fmt.Errorf("field 3: invalid length")
			}
			i += n
			if i+int(length) > len(dAtA) {
				return fmt.Errorf("field 3: data too short")
			}
			m.AttestationHex = string(dAtA[i : i+int(length)])
			i += int(length)

		default:
			// skip unknown fields
			if wireType == 0 {
				_, n := binary.Uvarint(dAtA[i:])
				if n <= 0 {
					return fmt.Errorf("skip varint: invalid")
				}
				i += n
			} else if wireType == 2 {
				length, n := binary.Uvarint(dAtA[i:])
				if n <= 0 {
					return fmt.Errorf("skip len-prefixed: invalid length")
				}
				i += n + int(length)
			} else {
				return fmt.Errorf("unknown wire type %d for field %d", wireType, fieldNum)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// MsgAttestStorageResponse
// ---------------------------------------------------------------------------

type MsgAttestStorageResponse struct{}

func (m *MsgAttestStorageResponse) Reset()         { *m = MsgAttestStorageResponse{} }
func (m *MsgAttestStorageResponse) String() string { return "MsgAttestStorageResponse{}" }
func (m *MsgAttestStorageResponse) ProtoMessage()  {}

func (m *MsgAttestStorageResponse) Marshal() ([]byte, error) { return nil, nil }
func (m *MsgAttestStorageResponse) MarshalTo(dAtA []byte) (int, error) {
	return 0, nil
}
func (m *MsgAttestStorageResponse) Size() int              { return 0 }
func (m *MsgAttestStorageResponse) Unmarshal([]byte) error { return nil }

// ---------------------------------------------------------------------------
// Protobuf encoding helpers
// ---------------------------------------------------------------------------

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func appendLenPrefixed(buf, data []byte) []byte {
	buf = appendVarint(buf, uint64(len(data)))
	return append(buf, data...)
}

func sizeVarint(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}

// ---------------------------------------------------------------------------
// Registration (called from proto_register.go init)
// ---------------------------------------------------------------------------

func RegisterMsgAttestStorageProto() {
	proto.RegisterType((*MsgAttestStorage)(nil), protoNameMsgAttestStorage)
	proto.RegisterType((*MsgAttestStorageResponse)(nil), protoNameMsgAttestStorageResponse)
}
