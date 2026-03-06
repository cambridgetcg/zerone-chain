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
// 2. FitnessSignal
// ===========================================================================

type FitnessSignal struct {
	SampleId    string `protobuf:"bytes,1,opt,name=sample_id,json=sampleId,proto3" json:"sample_id,omitempty"`
	SignalType  string `protobuf:"bytes,2,opt,name=signal_type,json=signalType,proto3" json:"signal_type,omitempty"`
	Value       string `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
	Weight      string `protobuf:"bytes,4,opt,name=weight,proto3" json:"weight,omitempty"`
	BlockHeight int64  `protobuf:"varint,5,opt,name=block_height,json=blockHeight,proto3" json:"block_height,omitempty"`
}

func (m *FitnessSignal) Reset()         { *m = FitnessSignal{} }
func (m *FitnessSignal) String() string { return fmt.Sprintf("%+v", *m) }
func (m *FitnessSignal) ProtoMessage()  {}

func (m *FitnessSignal) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendStringField(buf, 1, m.SampleId)
	buf = protoAppendStringField(buf, 2, m.SignalType)
	buf = protoAppendStringField(buf, 3, m.Value)
	buf = protoAppendStringField(buf, 4, m.Weight)
	buf = protoAppendInt64Field(buf, 5, m.BlockHeight)
	return buf, nil
}

func (m *FitnessSignal) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *FitnessSignal) Size() int {
	n := protoSizeStringField(1, m.SampleId)
	n += protoSizeStringField(2, m.SignalType)
	n += protoSizeStringField(3, m.Value)
	n += protoSizeStringField(4, m.Weight)
	n += protoSizeInt64Field(5, m.BlockHeight)
	return n
}

func (m *FitnessSignal) Unmarshal(dAtA []byte) error {
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
			m.SignalType = string(bz)
			i = newI
		case 3:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.Value = string(bz)
			i = newI
		case 4:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.Weight = string(bz)
			i = newI
		case 5:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.BlockHeight = int64(v)
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
// 3. FitnessParams
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
// 4. ReputationRecord
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
// 5. ReputationDecayParams
// ===========================================================================

type ReputationDecayParams struct {
	DecayRateBps              uint32 `protobuf:"varint,1,opt,name=decay_rate_bps,json=decayRateBps,proto3" json:"decay_rate_bps,omitempty"`
	DecayIntervalBlocks       uint64 `protobuf:"varint,2,opt,name=decay_interval_blocks,json=decayIntervalBlocks,proto3" json:"decay_interval_blocks,omitempty"`
	FloorRatioBps             uint32 `protobuf:"varint,3,opt,name=floor_ratio_bps,json=floorRatioBps,proto3" json:"floor_ratio_bps,omitempty"`
	SubmitterReputationGain   string `protobuf:"bytes,4,opt,name=submitter_reputation_gain,json=submitterReputationGain,proto3" json:"submitter_reputation_gain,omitempty"`
	ReviewerReputationGain    string `protobuf:"bytes,5,opt,name=reviewer_reputation_gain,json=reviewerReputationGain,proto3" json:"reviewer_reputation_gain,omitempty"`
	ReviewerReputationPenalty string `protobuf:"bytes,6,opt,name=reviewer_reputation_penalty,json=reviewerReputationPenalty,proto3" json:"reviewer_reputation_penalty,omitempty"`
	ReputationMultiplierBps   uint32 `protobuf:"varint,7,opt,name=reputation_multiplier_bps,json=reputationMultiplierBps,proto3" json:"reputation_multiplier_bps,omitempty"`
	BaseVoteWeight            string `protobuf:"bytes,8,opt,name=base_vote_weight,json=baseVoteWeight,proto3" json:"base_vote_weight,omitempty"`
}

func (m *ReputationDecayParams) Reset()         { *m = ReputationDecayParams{} }
func (m *ReputationDecayParams) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ReputationDecayParams) ProtoMessage()  {}

func (m *ReputationDecayParams) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendVarintField(buf, 1, uint64(m.DecayRateBps))
	buf = protoAppendVarintField(buf, 2, m.DecayIntervalBlocks)
	buf = protoAppendVarintField(buf, 3, uint64(m.FloorRatioBps))
	buf = protoAppendStringField(buf, 4, m.SubmitterReputationGain)
	buf = protoAppendStringField(buf, 5, m.ReviewerReputationGain)
	buf = protoAppendStringField(buf, 6, m.ReviewerReputationPenalty)
	buf = protoAppendVarintField(buf, 7, uint64(m.ReputationMultiplierBps))
	buf = protoAppendStringField(buf, 8, m.BaseVoteWeight)
	return buf, nil
}

func (m *ReputationDecayParams) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *ReputationDecayParams) Size() int {
	n := protoSizeVarintField(1, uint64(m.DecayRateBps))
	n += protoSizeVarintField(2, m.DecayIntervalBlocks)
	n += protoSizeVarintField(3, uint64(m.FloorRatioBps))
	n += protoSizeStringField(4, m.SubmitterReputationGain)
	n += protoSizeStringField(5, m.ReviewerReputationGain)
	n += protoSizeStringField(6, m.ReviewerReputationPenalty)
	n += protoSizeVarintField(7, uint64(m.ReputationMultiplierBps))
	n += protoSizeStringField(8, m.BaseVoteWeight)
	return n
}

func (m *ReputationDecayParams) Unmarshal(dAtA []byte) error {
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
			m.DecayRateBps = uint32(v)
			i = newI
		case 2:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.DecayIntervalBlocks = v
			i = newI
		case 3:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.FloorRatioBps = uint32(v)
			i = newI
		case 4:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.SubmitterReputationGain = string(bz)
			i = newI
		case 5:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.ReviewerReputationGain = string(bz)
			i = newI
		case 6:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.ReviewerReputationPenalty = string(bz)
			i = newI
		case 7:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.ReputationMultiplierBps = uint32(v)
			i = newI
		case 8:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.BaseVoteWeight = string(bz)
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
// 6. ShardingParams
// ===========================================================================

type ShardingParams struct {
	ReplicationFactor uint32 `protobuf:"varint,1,opt,name=replication_factor,json=replicationFactor,proto3" json:"replication_factor,omitempty"`
	SnapshotInterval  uint64 `protobuf:"varint,2,opt,name=snapshot_interval,json=snapshotInterval,proto3" json:"snapshot_interval,omitempty"`
	MinValidators     uint32 `protobuf:"varint,3,opt,name=min_validators,json=minValidators,proto3" json:"min_validators,omitempty"`
}

func (m *ShardingParams) Reset()         { *m = ShardingParams{} }
func (m *ShardingParams) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ShardingParams) ProtoMessage()  {}

func (m *ShardingParams) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendVarintField(buf, 1, uint64(m.ReplicationFactor))
	buf = protoAppendVarintField(buf, 2, m.SnapshotInterval)
	buf = protoAppendVarintField(buf, 3, uint64(m.MinValidators))
	return buf, nil
}

func (m *ShardingParams) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *ShardingParams) Size() int {
	n := protoSizeVarintField(1, uint64(m.ReplicationFactor))
	n += protoSizeVarintField(2, m.SnapshotInterval)
	n += protoSizeVarintField(3, uint64(m.MinValidators))
	return n
}

func (m *ShardingParams) Unmarshal(dAtA []byte) error {
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
			m.ReplicationFactor = uint32(v)
			i = newI
		case 2:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.SnapshotInterval = v
			i = newI
		case 3:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.MinValidators = uint32(v)
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
// 7. ShardAssignment
// ===========================================================================

type ShardAssignment struct {
	SampleId       string   `protobuf:"bytes,1,opt,name=sample_id,json=sampleId,proto3" json:"sample_id,omitempty"`
	Validators     []string `protobuf:"bytes,2,rep,name=validators,proto3" json:"validators,omitempty"`
	SnapshotHeight int64    `protobuf:"varint,3,opt,name=snapshot_height,json=snapshotHeight,proto3" json:"snapshot_height,omitempty"`
}

func (m *ShardAssignment) Reset()         { *m = ShardAssignment{} }
func (m *ShardAssignment) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ShardAssignment) ProtoMessage()  {}

func (m *ShardAssignment) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendStringField(buf, 1, m.SampleId)
	for _, v := range m.Validators {
		// Repeated string: always encode each element, including empty strings.
		buf = protoAppendTag(buf, 2, protoWireLengthDelimited)
		buf = protoAppendLenPrefixed(buf, []byte(v))
	}
	buf = protoAppendInt64Field(buf, 3, m.SnapshotHeight)
	return buf, nil
}

func (m *ShardAssignment) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *ShardAssignment) Size() int {
	n := protoSizeStringField(1, m.SampleId)
	for _, v := range m.Validators {
		l := len(v)
		n += protoSizeTag(2) + protoSizeVarint(uint64(l)) + l
	}
	n += protoSizeInt64Field(3, m.SnapshotHeight)
	return n
}

func (m *ShardAssignment) Unmarshal(dAtA []byte) error {
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
			m.Validators = append(m.Validators, string(bz))
			i = newI
		case 3:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.SnapshotHeight = int64(v)
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
// 8. AttestationRecord
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
// 9. ReviewerStakingParams
// ===========================================================================

type ReviewerStakingParams struct {
	ReviewerStakeRatioBps uint64 `protobuf:"varint,1,opt,name=reviewer_stake_ratio_bps,json=reviewerStakeRatioBps,proto3" json:"reviewer_stake_ratio_bps,omitempty"`
	ShowUpRewardRatioBps  uint64 `protobuf:"varint,2,opt,name=show_up_reward_ratio_bps,json=showUpRewardRatioBps,proto3" json:"show_up_reward_ratio_bps,omitempty"`
	AcceptRewardRatioBps  uint64 `protobuf:"varint,3,opt,name=accept_reward_ratio_bps,json=acceptRewardRatioBps,proto3" json:"accept_reward_ratio_bps,omitempty"`
	RejectBonusRatioBps   uint64 `protobuf:"varint,4,opt,name=reject_bonus_ratio_bps,json=rejectBonusRatioBps,proto3" json:"reject_bonus_ratio_bps,omitempty"`
	MaxContestedDeepCount uint64 `protobuf:"varint,5,opt,name=max_contested_deep_count,json=maxContestedDeepCount,proto3" json:"max_contested_deep_count,omitempty"`
}

func (m *ReviewerStakingParams) Reset()         { *m = ReviewerStakingParams{} }
func (m *ReviewerStakingParams) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ReviewerStakingParams) ProtoMessage()  {}

func (m *ReviewerStakingParams) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendVarintField(buf, 1, m.ReviewerStakeRatioBps)
	buf = protoAppendVarintField(buf, 2, m.ShowUpRewardRatioBps)
	buf = protoAppendVarintField(buf, 3, m.AcceptRewardRatioBps)
	buf = protoAppendVarintField(buf, 4, m.RejectBonusRatioBps)
	buf = protoAppendVarintField(buf, 5, m.MaxContestedDeepCount)
	return buf, nil
}

func (m *ReviewerStakingParams) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *ReviewerStakingParams) Size() int {
	n := protoSizeVarintField(1, m.ReviewerStakeRatioBps)
	n += protoSizeVarintField(2, m.ShowUpRewardRatioBps)
	n += protoSizeVarintField(3, m.AcceptRewardRatioBps)
	n += protoSizeVarintField(4, m.RejectBonusRatioBps)
	n += protoSizeVarintField(5, m.MaxContestedDeepCount)
	return n
}

func (m *ReviewerStakingParams) Unmarshal(dAtA []byte) error {
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
			m.ReviewerStakeRatioBps = v
			i = newI
		case 2:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.ShowUpRewardRatioBps = v
			i = newI
		case 3:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.AcceptRewardRatioBps = v
			i = newI
		case 4:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.RejectBonusRatioBps = v
			i = newI
		case 5:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.MaxContestedDeepCount = v
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
// 10. TrainingRecord
// ===========================================================================

type TrainingRecord struct {
	Operator           string `protobuf:"bytes,1,opt,name=operator,proto3" json:"operator,omitempty"`
	EnclaveId          string `protobuf:"bytes,2,opt,name=enclave_id,json=enclaveId,proto3" json:"enclave_id,omitempty"`
	AttestationHash    string `protobuf:"bytes,3,opt,name=attestation_hash,json=attestationHash,proto3" json:"attestation_hash,omitempty"`
	DatasetFingerprint string `protobuf:"bytes,4,opt,name=dataset_fingerprint,json=datasetFingerprint,proto3" json:"dataset_fingerprint,omitempty"`
	DatasetSize        int64  `protobuf:"varint,5,opt,name=dataset_size,json=datasetSize,proto3" json:"dataset_size,omitempty"`
	BaseModel          string `protobuf:"bytes,6,opt,name=base_model,json=baseModel,proto3" json:"base_model,omitempty"`
	ModelHash          string `protobuf:"bytes,7,opt,name=model_hash,json=modelHash,proto3" json:"model_hash,omitempty"`
	BenchmarkScore     string `protobuf:"bytes,8,opt,name=benchmark_score,json=benchmarkScore,proto3" json:"benchmark_score,omitempty"`
	BlockHeight        int64  `protobuf:"varint,9,opt,name=block_height,json=blockHeight,proto3" json:"block_height,omitempty"`
}

func (m *TrainingRecord) Reset()         { *m = TrainingRecord{} }
func (m *TrainingRecord) String() string { return fmt.Sprintf("%+v", *m) }
func (m *TrainingRecord) ProtoMessage()  {}

func (m *TrainingRecord) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendStringField(buf, 1, m.Operator)
	buf = protoAppendStringField(buf, 2, m.EnclaveId)
	buf = protoAppendStringField(buf, 3, m.AttestationHash)
	buf = protoAppendStringField(buf, 4, m.DatasetFingerprint)
	buf = protoAppendInt64Field(buf, 5, m.DatasetSize)
	buf = protoAppendStringField(buf, 6, m.BaseModel)
	buf = protoAppendStringField(buf, 7, m.ModelHash)
	buf = protoAppendStringField(buf, 8, m.BenchmarkScore)
	buf = protoAppendInt64Field(buf, 9, m.BlockHeight)
	return buf, nil
}

func (m *TrainingRecord) MarshalTo(dAtA []byte) (int, error) {
	bz, err := m.Marshal()
	if err != nil {
		return 0, err
	}
	copy(dAtA, bz)
	return len(bz), nil
}

func (m *TrainingRecord) Size() int {
	n := protoSizeStringField(1, m.Operator)
	n += protoSizeStringField(2, m.EnclaveId)
	n += protoSizeStringField(3, m.AttestationHash)
	n += protoSizeStringField(4, m.DatasetFingerprint)
	n += protoSizeInt64Field(5, m.DatasetSize)
	n += protoSizeStringField(6, m.BaseModel)
	n += protoSizeStringField(7, m.ModelHash)
	n += protoSizeStringField(8, m.BenchmarkScore)
	n += protoSizeInt64Field(9, m.BlockHeight)
	return n
}

func (m *TrainingRecord) Unmarshal(dAtA []byte) error {
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
			m.Operator = string(bz)
			i = newI
		case 2:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.EnclaveId = string(bz)
			i = newI
		case 3:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.AttestationHash = string(bz)
			i = newI
		case 4:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.DatasetFingerprint = string(bz)
			i = newI
		case 5:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.DatasetSize = int64(v)
			i = newI
		case 6:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.BaseModel = string(bz)
			i = newI
		case 7:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.ModelHash = string(bz)
			i = newI
		case 8:
			bz, newI, err := protoConsumeBytes(dAtA, i)
			if err != nil {
				return err
			}
			m.BenchmarkScore = string(bz)
			i = newI
		case 9:
			v, newI, err := protoConsumeVarint(dAtA, i)
			if err != nil {
				return err
			}
			m.BlockHeight = int64(v)
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
	proto.RegisterType((*FitnessSignal)(nil), "zerone.knowledge.v1.FitnessSignal")
	proto.RegisterType((*FitnessParams)(nil), "zerone.knowledge.v1.FitnessParams")
	proto.RegisterType((*ReputationRecord)(nil), "zerone.knowledge.v1.ReputationRecord")
	proto.RegisterType((*ReputationDecayParams)(nil), "zerone.knowledge.v1.ReputationDecayParams")
	proto.RegisterType((*ShardingParams)(nil), "zerone.knowledge.v1.ShardingParams")
	proto.RegisterType((*ShardAssignment)(nil), "zerone.knowledge.v1.ShardAssignment")
	proto.RegisterType((*AttestationRecord)(nil), "zerone.knowledge.v1.AttestationRecord")
	proto.RegisterType((*ReviewerStakingParams)(nil), "zerone.knowledge.v1.ReviewerStakingParams")
	proto.RegisterType((*TrainingRecord)(nil), "zerone.knowledge.v1.TrainingRecord")
}
