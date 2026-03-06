// msgs_r43.go — Proto-compatible message types for R43-1 cleanup. When proto-gen runs, these will be superseded by generated code in tx.pb.go.
package types

import (
	"fmt"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

// ---------------------------------------------------------------------------
// MsgRecordTraining
// ---------------------------------------------------------------------------

type MsgRecordTraining struct {
	Operator           string `protobuf:"bytes,1,opt,name=operator,proto3" json:"operator,omitempty"`
	AttestationHash    string `protobuf:"bytes,2,opt,name=attestation_hash,json=attestationHash,proto3" json:"attestation_hash,omitempty"`
	DatasetFingerprint string `protobuf:"bytes,3,opt,name=dataset_fingerprint,json=datasetFingerprint,proto3" json:"dataset_fingerprint,omitempty"`
	DatasetSize        int64  `protobuf:"varint,4,opt,name=dataset_size,json=datasetSize,proto3" json:"dataset_size,omitempty"`
	BaseModel          string `protobuf:"bytes,5,opt,name=base_model,json=baseModel,proto3" json:"base_model,omitempty"`
	ModelHash          string `protobuf:"bytes,6,opt,name=model_hash,json=modelHash,proto3" json:"model_hash,omitempty"`
	BenchmarkScore     string `protobuf:"bytes,7,opt,name=benchmark_score,json=benchmarkScore,proto3" json:"benchmark_score,omitempty"`
}

func (m *MsgRecordTraining) Reset()         { *m = MsgRecordTraining{} }
func (m *MsgRecordTraining) String() string { return fmt.Sprintf("%+v", *m) }
func (m *MsgRecordTraining) ProtoMessage()  {}

func (m *MsgRecordTraining) Marshal() ([]byte, error) {
	buf := make([]byte, 0, m.Size())
	buf = protoAppendStringField(buf, 1, m.Operator)
	buf = protoAppendStringField(buf, 2, m.AttestationHash)
	buf = protoAppendStringField(buf, 3, m.DatasetFingerprint)
	buf = protoAppendInt64Field(buf, 4, m.DatasetSize)
	buf = protoAppendStringField(buf, 5, m.BaseModel)
	buf = protoAppendStringField(buf, 6, m.ModelHash)
	buf = protoAppendStringField(buf, 7, m.BenchmarkScore)
	return buf, nil
}

func (m *MsgRecordTraining) MarshalTo(dAtA []byte) (int, error) {
	data, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, data)
	return len(data), nil
}

func (m *MsgRecordTraining) Size() int {
	var n int
	n += protoSizeStringField(1, m.Operator)
	n += protoSizeStringField(2, m.AttestationHash)
	n += protoSizeStringField(3, m.DatasetFingerprint)
	n += protoSizeInt64Field(4, m.DatasetSize)
	n += protoSizeStringField(5, m.BaseModel)
	n += protoSizeStringField(6, m.ModelHash)
	n += protoSizeStringField(7, m.BenchmarkScore)
	return n
}

func (m *MsgRecordTraining) Unmarshal(dAtA []byte) error {
	offset := 0
	for offset < len(dAtA) {
		fieldNum, wireType, newOffset, err := protoConsumeTag(dAtA, offset)
		if err != nil {
			return err
		}
		offset = newOffset

		switch fieldNum {
		case 1: // operator
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.Operator = string(raw)
			offset = newOff
		case 2: // attestation_hash
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.AttestationHash = string(raw)
			offset = newOff
		case 3: // dataset_fingerprint
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.DatasetFingerprint = string(raw)
			offset = newOff
		case 4: // dataset_size
			v, newOff, err := protoConsumeVarint(dAtA, offset)
			if err != nil {
				return err
			}
			m.DatasetSize = int64(v)
			offset = newOff
		case 5: // base_model
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.BaseModel = string(raw)
			offset = newOff
		case 6: // model_hash
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.ModelHash = string(raw)
			offset = newOff
		case 7: // benchmark_score
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.BenchmarkScore = string(raw)
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

func (m *MsgRecordTraining) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(m.Operator)
	if err != nil {
		return nil
	}
	return []sdk.AccAddress{addr}
}

func (m *MsgRecordTraining) ValidateBasic() error {
	if m.Operator == "" {
		return ErrUnauthorized.Wrap("operator is required")
	}
	if m.AttestationHash == "" {
		return ErrInvalidTrainingAttestation.Wrap("attestation hash is required")
	}
	if m.ModelHash == "" {
		return ErrInvalidTrainingAttestation.Wrap("model hash is required")
	}
	if m.BaseModel == "" {
		return ErrInvalidTrainingAttestation.Wrap("base model is required")
	}
	if m.DatasetSize <= 0 {
		return ErrInvalidTrainingAttestation.Wrap("dataset size must be positive")
	}
	score, err := strconv.ParseFloat(m.BenchmarkScore, 64)
	if err != nil || score < 0 || score > 1 {
		return ErrInvalidTrainingAttestation.Wrap("benchmark score must be a valid decimal in [0, 1]")
	}
	return nil
}

// ---------------------------------------------------------------------------
// MsgRecordTrainingResponse
// ---------------------------------------------------------------------------

type MsgRecordTrainingResponse struct {
	AttestationHash string `protobuf:"bytes,1,opt,name=attestation_hash,json=attestationHash,proto3" json:"attestation_hash,omitempty"`
}

func (m *MsgRecordTrainingResponse) Reset()         { *m = MsgRecordTrainingResponse{} }
func (m *MsgRecordTrainingResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *MsgRecordTrainingResponse) ProtoMessage()  {}

func (m *MsgRecordTrainingResponse) Marshal() ([]byte, error) {
	buf := make([]byte, 0, m.Size())
	buf = protoAppendStringField(buf, 1, m.AttestationHash)
	return buf, nil
}

func (m *MsgRecordTrainingResponse) MarshalTo(dAtA []byte) (int, error) {
	data, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, data)
	return len(data), nil
}

func (m *MsgRecordTrainingResponse) Size() int {
	return protoSizeStringField(1, m.AttestationHash)
}

func (m *MsgRecordTrainingResponse) Unmarshal(dAtA []byte) error {
	offset := 0
	for offset < len(dAtA) {
		fieldNum, wireType, newOffset, err := protoConsumeTag(dAtA, offset)
		if err != nil {
			return err
		}
		offset = newOffset

		switch fieldNum {
		case 1: // attestation_hash
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			m.AttestationHash = string(raw)
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
	proto.RegisterType((*MsgRecordTraining)(nil), "zerone.knowledge.v1.MsgRecordTraining")
	proto.RegisterType((*MsgRecordTrainingResponse)(nil), "zerone.knowledge.v1.MsgRecordTrainingResponse")
	proto.RegisterType((*MsgUpdateFitnessBatch)(nil), "zerone.knowledge.v1.MsgUpdateFitnessBatch")
	proto.RegisterType((*MsgUpdateFitnessBatchResponse)(nil), "zerone.knowledge.v1.MsgUpdateFitnessBatchResponse")
}
