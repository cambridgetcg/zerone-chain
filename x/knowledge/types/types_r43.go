// types_r43.go — Proto-compatible types for R43-1 cleanup. Replaces JSON-only types.
// When proto-gen runs, these will be superseded by generated code in types.pb.go.
package types

import (
	"fmt"

	"github.com/cosmos/gogoproto/proto"
)

// ===========================================================================
// 1. FitnessRecord
// ===========================================================================

type FitnessRecord struct {
	SampleId        string `protobuf:"bytes,1,opt,name=sample_id,json=sampleId,proto3" json:"sample_id,omitempty"`
	Score           string `protobuf:"bytes,2,opt,name=score,proto3" json:"score,omitempty"`
	PeakScore       string `protobuf:"bytes,3,opt,name=peak_score,json=peakScore,proto3" json:"peak_score,omitempty"`
	LastSignalBlock int64  `protobuf:"varint,4,opt,name=last_signal_block,json=lastSignalBlock,proto3" json:"last_signal_block,omitempty"`
	CreatedAtBlock  int64  `protobuf:"varint,5,opt,name=created_at_block,json=createdAtBlock,proto3" json:"created_at_block,omitempty"`
	LifecycleStatus string `protobuf:"bytes,6,opt,name=lifecycle_status,json=lifecycleStatus,proto3" json:"lifecycle_status,omitempty"`
}

func (m *FitnessRecord) Reset()         { *m = FitnessRecord{} }
func (m *FitnessRecord) String() string { return fmt.Sprintf("%+v", *m) }
func (m *FitnessRecord) ProtoMessage()  {}

func (m *FitnessRecord) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendStringField(buf, 1, m.SampleId)
	buf = protoAppendStringField(buf, 2, m.Score)
	buf = protoAppendStringField(buf, 3, m.PeakScore)
	buf = protoAppendInt64Field(buf, 4, m.LastSignalBlock)
	buf = protoAppendInt64Field(buf, 5, m.CreatedAtBlock)
	buf = protoAppendStringField(buf, 6, m.LifecycleStatus)
	return buf, nil
}

func (m *FitnessRecord) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *FitnessRecord) Size() int {
	n := protoSizeStringField(1, m.SampleId)
	n += protoSizeStringField(2, m.Score)
	n += protoSizeStringField(3, m.PeakScore)
	n += protoSizeInt64Field(4, m.LastSignalBlock)
	n += protoSizeInt64Field(5, m.CreatedAtBlock)
	n += protoSizeStringField(6, m.LifecycleStatus)
	return n
}

func (m *FitnessRecord) Unmarshal(dAtA []byte) error {
	i := 0
	for i < len(dAtA) {
		fieldNum, wireType, newI, err := protoConsumeTag(dAtA, i)
		if err != nil {
			return err
		}
		i = newI
		switch fieldNum {
		case 1:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.SampleId = string(bz)
			i = newI
		case 2:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.Score = string(bz)
			i = newI
		case 3:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.PeakScore = string(bz)
			i = newI
		case 4:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.LastSignalBlock = int64(v)
			i = newI
		case 5:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.CreatedAtBlock = int64(v)
			i = newI
		case 6:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.LifecycleStatus = string(bz)
			i = newI
		default:
			newI, err = protoSkipField(dAtA, i, wireType)
			if err != nil {
				return err
			}
			i = newI
		}
	}
	return nil
}

// ===========================================================================
// 2. FitnessParams
// ===========================================================================

type FitnessParams struct {
	FitnessEpochBlocks    uint64 `protobuf:"varint,1,opt,name=fitness_epoch_blocks,json=fitnessEpochBlocks,proto3" json:"fitness_epoch_blocks,omitempty"`
	DecayRateUnscored     string `protobuf:"bytes,2,opt,name=decay_rate_unscored,json=decayRateUnscored,proto3" json:"decay_rate_unscored,omitempty"`
	LongevityRewardAmount string `protobuf:"bytes,3,opt,name=longevity_reward_amount,json=longevityRewardAmount,proto3" json:"longevity_reward_amount,omitempty"`
	PruneThreshold        string `protobuf:"bytes,4,opt,name=prune_threshold,json=pruneThreshold,proto3" json:"prune_threshold,omitempty"`
}

func (m *FitnessParams) Reset()         { *m = FitnessParams{} }
func (m *FitnessParams) String() string { return fmt.Sprintf("%+v", *m) }
func (m *FitnessParams) ProtoMessage()  {}

func (m *FitnessParams) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendVarintField(buf, 1, m.FitnessEpochBlocks)
	buf = protoAppendStringField(buf, 2, m.DecayRateUnscored)
	buf = protoAppendStringField(buf, 3, m.LongevityRewardAmount)
	buf = protoAppendStringField(buf, 4, m.PruneThreshold)
	return buf, nil
}

func (m *FitnessParams) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *FitnessParams) Size() int {
	n := protoSizeVarintField(1, m.FitnessEpochBlocks)
	n += protoSizeStringField(2, m.DecayRateUnscored)
	n += protoSizeStringField(3, m.LongevityRewardAmount)
	n += protoSizeStringField(4, m.PruneThreshold)
	return n
}

func (m *FitnessParams) Unmarshal(dAtA []byte) error {
	i := 0
	for i < len(dAtA) {
		fieldNum, wireType, newI, err := protoConsumeTag(dAtA, i)
		if err != nil {
			return err
		}
		i = newI
		switch fieldNum {
		case 1:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.FitnessEpochBlocks = v
			i = newI
		case 2:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.DecayRateUnscored = string(bz)
			i = newI
		case 3:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.LongevityRewardAmount = string(bz)
			i = newI
		case 4:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.PruneThreshold = string(bz)
			i = newI
		default:
			newI, err = protoSkipField(dAtA, i, wireType)
			if err != nil {
				return err
			}
			i = newI
		}
	}
	return nil
}

// ===========================================================================
// 3. ReputationRecord
// ===========================================================================

type ReputationRecord struct {
	Agent           string `protobuf:"bytes,1,opt,name=agent,proto3" json:"agent,omitempty"`
	Domain          string `protobuf:"bytes,2,opt,name=domain,proto3" json:"domain,omitempty"`
	Score           string `protobuf:"bytes,3,opt,name=score,proto3" json:"score,omitempty"`
	PeakScore       string `protobuf:"bytes,4,opt,name=peak_score,json=peakScore,proto3" json:"peak_score,omitempty"`
	LastActiveBlock int64  `protobuf:"varint,5,opt,name=last_active_block,json=lastActiveBlock,proto3" json:"last_active_block,omitempty"`
	CreatedAtBlock  int64  `protobuf:"varint,6,opt,name=created_at_block,json=createdAtBlock,proto3" json:"created_at_block,omitempty"`
}

func (m *ReputationRecord) Reset()         { *m = ReputationRecord{} }
func (m *ReputationRecord) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ReputationRecord) ProtoMessage()  {}

func (m *ReputationRecord) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendStringField(buf, 1, m.Agent)
	buf = protoAppendStringField(buf, 2, m.Domain)
	buf = protoAppendStringField(buf, 3, m.Score)
	buf = protoAppendStringField(buf, 4, m.PeakScore)
	buf = protoAppendInt64Field(buf, 5, m.LastActiveBlock)
	buf = protoAppendInt64Field(buf, 6, m.CreatedAtBlock)
	return buf, nil
}

func (m *ReputationRecord) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *ReputationRecord) Size() int {
	n := protoSizeStringField(1, m.Agent)
	n += protoSizeStringField(2, m.Domain)
	n += protoSizeStringField(3, m.Score)
	n += protoSizeStringField(4, m.PeakScore)
	n += protoSizeInt64Field(5, m.LastActiveBlock)
	n += protoSizeInt64Field(6, m.CreatedAtBlock)
	return n
}

func (m *ReputationRecord) Unmarshal(dAtA []byte) error {
	i := 0
	for i < len(dAtA) {
		fieldNum, wireType, newI, err := protoConsumeTag(dAtA, i)
		if err != nil {
			return err
		}
		i = newI
		switch fieldNum {
		case 1:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.Agent = string(bz)
			i = newI
		case 2:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.Domain = string(bz)
			i = newI
		case 3:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.Score = string(bz)
			i = newI
		case 4:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.PeakScore = string(bz)
			i = newI
		case 5:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.LastActiveBlock = int64(v)
			i = newI
		case 6:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.CreatedAtBlock = int64(v)
			i = newI
		default:
			newI, err = protoSkipField(dAtA, i, wireType)
			if err != nil {
				return err
			}
			i = newI
		}
	}
	return nil
}

// ===========================================================================
// 4. AttestationRecord
// ===========================================================================

type AttestationRecord struct {
	Validator       string `protobuf:"bytes,1,opt,name=validator,proto3" json:"validator,omitempty"`
	SnapshotHeight  int64  `protobuf:"varint,2,opt,name=snapshot_height,json=snapshotHeight,proto3" json:"snapshot_height,omitempty"`
	DataHash        string `protobuf:"bytes,3,opt,name=data_hash,json=dataHash,proto3" json:"data_hash,omitempty"`
	AttestedAtBlock int64  `protobuf:"varint,4,opt,name=attested_at_block,json=attestedAtBlock,proto3" json:"attested_at_block,omitempty"`
	Verified        bool   `protobuf:"varint,5,opt,name=verified,proto3" json:"verified,omitempty"`
}

func (m *AttestationRecord) Reset()         { *m = AttestationRecord{} }
func (m *AttestationRecord) String() string { return fmt.Sprintf("%+v", *m) }
func (m *AttestationRecord) ProtoMessage()  {}

func (m *AttestationRecord) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendStringField(buf, 1, m.Validator)
	buf = protoAppendInt64Field(buf, 2, m.SnapshotHeight)
	buf = protoAppendStringField(buf, 3, m.DataHash)
	buf = protoAppendInt64Field(buf, 4, m.AttestedAtBlock)
	buf = protoAppendBoolField(buf, 5, m.Verified)
	return buf, nil
}

func (m *AttestationRecord) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *AttestationRecord) Size() int {
	n := protoSizeStringField(1, m.Validator)
	n += protoSizeInt64Field(2, m.SnapshotHeight)
	n += protoSizeStringField(3, m.DataHash)
	n += protoSizeInt64Field(4, m.AttestedAtBlock)
	n += protoSizeBoolField(5, m.Verified)
	return n
}

func (m *AttestationRecord) Unmarshal(dAtA []byte) error {
	i := 0
	for i < len(dAtA) {
		fieldNum, wireType, newI, err := protoConsumeTag(dAtA, i)
		if err != nil {
			return err
		}
		i = newI
		switch fieldNum {
		case 1:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.Validator = string(bz)
			i = newI
		case 2:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.SnapshotHeight = int64(v)
			i = newI
		case 3:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.DataHash = string(bz)
			i = newI
		case 4:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.AttestedAtBlock = int64(v)
			i = newI
		case 5:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.Verified = v != 0
			i = newI
		default:
			newI, err = protoSkipField(dAtA, i, wireType)
			if err != nil {
				return err
			}
			i = newI
		}
	}
	return nil
}

// ===========================================================================
// Proto registration
// ===========================================================================

func init() {
	proto.RegisterType((*FitnessRecord)(nil), "zerone.knowledge.v1.FitnessRecord")
	proto.RegisterType((*FitnessParams)(nil), "zerone.knowledge.v1.FitnessParams")
	proto.RegisterType((*ReputationRecord)(nil), "zerone.knowledge.v1.ReputationRecord")
	proto.RegisterType((*AttestationRecord)(nil), "zerone.knowledge.v1.AttestationRecord")
}
