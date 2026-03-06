package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestRecordTraining_Success(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Register an enclave first.
	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("attestation-doc"), []byte("code-hash"))
	require.NoError(t, err)

	msg := &types.MsgRecordTraining{
		Operator:           testAddr,
		AttestationHash:    "abc123",
		DatasetFingerprint: "def456",
		DatasetSize:        1000,
		BaseModel:          "llama-3-8b",
		ModelHash:          "model-hash-001",
		BenchmarkScore:     0.73,
	}

	resp, err := k.RecordTraining(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, "abc123", resp.AttestationHash)

	// Verify the record is stored.
	record, err := k.GetTrainingRecord(ctx, "abc123")
	require.NoError(t, err)
	require.Equal(t, testAddr, record.Operator)
	require.Equal(t, "llama-3-8b", record.BaseModel)
	require.Equal(t, "model-hash-001", record.ModelHash)
	require.Equal(t, int64(1000), record.DatasetSize)
	require.Equal(t, 0.73, record.BenchmarkScore)
	require.Equal(t, int64(100), record.BlockHeight)
}

func TestRecordTraining_NoEnclave(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgRecordTraining{
		Operator:           testAddr,
		AttestationHash:    "abc123",
		DatasetFingerprint: "def456",
		DatasetSize:        1000,
		BaseModel:          "llama-3-8b",
		ModelHash:          "model-hash-001",
		BenchmarkScore:     0.73,
	}

	_, err := k.RecordTraining(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveNotRegistered)
}

func TestRecordTraining_SuspendedEnclave(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	err = k.SuspendEnclave(ctx, testAddr)
	require.NoError(t, err)

	msg := &types.MsgRecordTraining{
		Operator:           testAddr,
		AttestationHash:    "abc123",
		DatasetFingerprint: "def456",
		DatasetSize:        1000,
		BaseModel:          "llama-3-8b",
		ModelHash:          "hash",
		BenchmarkScore:     0.5,
	}

	_, err = k.RecordTraining(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveNotActive)
}

func TestRecordTraining_DuplicateAttestation(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	msg := &types.MsgRecordTraining{
		Operator:           testAddr,
		AttestationHash:    "abc123",
		DatasetFingerprint: "def456",
		DatasetSize:        1000,
		BaseModel:          "llama-3-8b",
		ModelHash:          "hash",
		BenchmarkScore:     0.5,
	}

	_, err = k.RecordTraining(ctx, msg)
	require.NoError(t, err)

	// Second recording with same attestation hash should fail.
	_, err = k.RecordTraining(ctx, msg)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTrainingRecordExists)
}

func TestGetTrainingRecord_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.GetTrainingRecord(ctx, "nonexistent")
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTrainingRecordNotFound)
}

func TestMsgRecordTraining_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgRecordTraining
		wantErr bool
	}{
		{
			name: "valid",
			msg: types.MsgRecordTraining{
				Operator:           testAddr,
				AttestationHash:    "hash",
				DatasetFingerprint: "fp",
				DatasetSize:        100,
				BaseModel:          "llama-3-8b",
				ModelHash:          "model",
				BenchmarkScore:     0.5,
			},
			wantErr: false,
		},
		{
			name: "empty operator",
			msg: types.MsgRecordTraining{
				Operator:        "",
				AttestationHash: "hash",
				DatasetSize:     100,
				BaseModel:       "model",
				ModelHash:       "mh",
				BenchmarkScore:  0.5,
			},
			wantErr: true,
		},
		{
			name: "empty attestation hash",
			msg: types.MsgRecordTraining{
				Operator:       testAddr,
				DatasetSize:    100,
				BaseModel:      "model",
				ModelHash:      "mh",
				BenchmarkScore: 0.5,
			},
			wantErr: true,
		},
		{
			name: "empty model hash",
			msg: types.MsgRecordTraining{
				Operator:        testAddr,
				AttestationHash: "hash",
				DatasetSize:     100,
				BaseModel:       "model",
				BenchmarkScore:  0.5,
			},
			wantErr: true,
		},
		{
			name: "zero dataset size",
			msg: types.MsgRecordTraining{
				Operator:        testAddr,
				AttestationHash: "hash",
				DatasetSize:     0,
				BaseModel:       "model",
				ModelHash:       "mh",
				BenchmarkScore:  0.5,
			},
			wantErr: true,
		},
		{
			name: "benchmark out of range",
			msg: types.MsgRecordTraining{
				Operator:        testAddr,
				AttestationHash: "hash",
				DatasetSize:     100,
				BaseModel:       "model",
				ModelHash:       "mh",
				BenchmarkScore:  1.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTrainingMsgServer_RecordTraining(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTrainingMsgServerImpl(k)

	// Register enclave.
	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	resp, err := srv.RecordTraining(ctx, &types.MsgRecordTraining{
		Operator:           testAddr,
		AttestationHash:    "att-hash-001",
		DatasetFingerprint: "fp-001",
		DatasetSize:        500,
		BaseModel:          "llama-3-8b",
		ModelHash:          "model-001",
		BenchmarkScore:     0.8,
	})
	require.NoError(t, err)
	require.Equal(t, "att-hash-001", resp.AttestationHash)
}

func TestTrainingMsgServer_RecordTraining_InvalidMsg(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTrainingMsgServerImpl(k)

	_, err := srv.RecordTraining(ctx, &types.MsgRecordTraining{
		Operator: "", // invalid
	})
	require.Error(t, err)
}
