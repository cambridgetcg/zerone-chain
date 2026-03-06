// msgs_r43.go — Proto-compatible message types for R43-1 cleanup. When proto-gen runs, these will be superseded by generated code in tx.pb.go.
package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

// ---------------------------------------------------------------------------
// MsgUpdateFitnessBatch
// ---------------------------------------------------------------------------

type MsgUpdateFitnessBatch struct {
	Oracle  string           `protobuf:"bytes,1,opt,name=oracle,proto3" json:"oracle,omitempty"`
	Signals []*FitnessSignal `protobuf:"bytes,2,rep,name=signals,proto3" json:"signals,omitempty"`
}

func (m *MsgUpdateFitnessBatch) Reset()         { *m = MsgUpdateFitnessBatch{} }
func (m *MsgUpdateFitnessBatch) String() string { return fmt.Sprintf("%+v", *m) }
func (m *MsgUpdateFitnessBatch) ProtoMessage()  {}

func (m *MsgUpdateFitnessBatch) Marshal() ([]byte, error) {
	buf := make([]byte, 0, m.Size())
	buf = protoAppendStringField(buf, 1, m.Oracle)
	for _, sig := range m.Signals {
		sigBytes, err := sig.Marshal()
		if err != nil {
			return nil, err
		}
		buf = protoAppendMessageField(buf, 2, sigBytes)
	}
	return buf, nil
}

func (m *MsgUpdateFitnessBatch) MarshalTo(dAtA []byte) (int, error) {
	data, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, data)
	return len(data), nil
}

func (m *MsgUpdateFitnessBatch) Size() int {
	var n int
	n += protoSizeStringField(1, m.Oracle)
	for _, sig := range m.Signals {
		sigBytes, _ := sig.Marshal()
		n += protoSizeMessageField(2, sigBytes)
	}
	return n
}

func (m *MsgUpdateFitnessBatch) Unmarshal(dAtA []byte) error {
	offset := 0
	for offset < len(dAtA) {
		fieldNum, wireType, newOffset, err := protoConsumeTag(dAtA, offset)
		if err != nil {
			return err
		}
		offset = newOffset

		switch fieldNum {
		case 1: // oracle
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.Oracle = string(raw)
			offset = newOff
		case 2: // signals
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			sig := new(FitnessSignal)
			if err := sig.Unmarshal(raw); err != nil {
				return err
			}
			m.Signals = append(m.Signals, sig)
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

func (m *MsgUpdateFitnessBatch) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Oracle)
	if err != nil {
		return nil
	}
	return []sdk.AccAddress{addr}
}

func (m *MsgUpdateFitnessBatch) ValidateBasic() error {
	if m.Oracle == "" {
		return ErrUnauthorized.Wrap("oracle is required")
	}
	if len(m.Signals) == 0 {
		return ErrInvalidFitnessSignal.Wrap("signals must not be empty")
	}
	return nil
}

// ---------------------------------------------------------------------------
// MsgUpdateFitnessBatchResponse
// ---------------------------------------------------------------------------

type MsgUpdateFitnessBatchResponse struct {
	Processed uint64 `protobuf:"varint,1,opt,name=processed,proto3" json:"processed,omitempty"`
}

func (m *MsgUpdateFitnessBatchResponse) Reset()         { *m = MsgUpdateFitnessBatchResponse{} }
func (m *MsgUpdateFitnessBatchResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *MsgUpdateFitnessBatchResponse) ProtoMessage()  {}

func (m *MsgUpdateFitnessBatchResponse) Marshal() ([]byte, error) {
	buf := make([]byte, 0, m.Size())
	buf = protoAppendVarintField(buf, 1, m.Processed)
	return buf, nil
}

func (m *MsgUpdateFitnessBatchResponse) MarshalTo(dAtA []byte) (int, error) {
	data, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, data)
	return len(data), nil
}

func (m *MsgUpdateFitnessBatchResponse) Size() int {
	return protoSizeVarintField(1, m.Processed)
}

func (m *MsgUpdateFitnessBatchResponse) Unmarshal(dAtA []byte) error {
	offset := 0
	for offset < len(dAtA) {
		fieldNum, wireType, newOffset, err := protoConsumeTag(dAtA, offset)
		if err != nil {
			return err
		}
		offset = newOffset

		switch fieldNum {
		case 1: // processed
			v, newOff, err := protoConsumeVarint(dAtA, offset)
			if err != nil {
				return err
			}
			m.Processed = v
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

// ---------------------------------------------------------------------------
// Proto registration
// ---------------------------------------------------------------------------

func init() {
	proto.RegisterType((*MsgUpdateFitnessBatch)(nil), "zerone.knowledge.v1.MsgUpdateFitnessBatch")
	proto.RegisterType((*MsgUpdateFitnessBatchResponse)(nil), "zerone.knowledge.v1.MsgUpdateFitnessBatchResponse")
}
