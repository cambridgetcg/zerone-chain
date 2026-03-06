package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestRegisterEnclave_Success(t *testing.T) {
	k, ctx := setupKeeper(t)

	enclaveID, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("attestation-doc"), []byte("code-hash"))
	require.NoError(t, err)
	require.NotEmpty(t, enclaveID)

	// Verify enclave is stored.
	record, err := k.GetEnclave(ctx, testAddr)
	require.NoError(t, err)
	require.Equal(t, testAddr, record.Operator)
	require.Equal(t, "nitro", record.Provider)
	require.Equal(t, types.EnclaveStatusActiveStr, record.Status)
	require.Equal(t, int64(100), record.RegisteredAt)
	require.Equal(t, int64(100), record.LastVerified)
	require.NotEmpty(t, record.AttestationHash)
	require.Equal(t, []byte("code-hash"), record.Measurements)
}

func TestRegisterEnclave_DuplicateOperator(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc1"), []byte("hash1"))
	require.NoError(t, err)

	// Second registration should fail.
	_, err = k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc2"), []byte("hash2"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveAlreadyRegistered)
}

func TestRegisterEnclave_InvalidProvider(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "unknown-tee", []byte("doc"), []byte("hash"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidTEEProvider)
}

func TestRegisterEnclave_EmptyAttestation(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", nil, []byte("hash"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidAttestation)
}

func TestRegisterEnclave_EmptyMeasurements(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), nil)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrTEEMeasurementMismatch)
}

func TestRegisterEnclave_AllProviders(t *testing.T) {
	for _, provider := range []string{"nitro", "sgx", "sev"} {
		t.Run(provider, func(t *testing.T) {
			k, ctx := setupKeeper(t)
			_, err := k.RegisterEnclave(ctx, testAddr, provider, []byte("doc"), []byte("hash"))
			require.NoError(t, err)
		})
	}
}

func TestGetEnclave_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.GetEnclave(ctx, testAddr)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveNotFound)
}

func TestVerifyEnclave_Success(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc1"), []byte("hash"))
	require.NoError(t, err)

	// Advance block height.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(200)

	valid, err := k.VerifyEnclave(ctx, testAddr, []byte("fresh-attestation"))
	require.NoError(t, err)
	require.True(t, valid)

	// Verify last_verified was updated.
	record, err := k.GetEnclave(ctx, testAddr)
	require.NoError(t, err)
	require.Equal(t, int64(200), record.LastVerified)
}

func TestVerifyEnclave_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.VerifyEnclave(ctx, testAddr, []byte("doc"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveNotFound)
}

func TestVerifyEnclave_SuspendedEnclave(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	err = k.SuspendEnclave(ctx, testAddr)
	require.NoError(t, err)

	_, err = k.VerifyEnclave(ctx, testAddr, []byte("fresh-doc"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveNotActive)
}

func TestVerifyEnclave_EmptyAttestation(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	_, err = k.VerifyEnclave(ctx, testAddr, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidAttestation)
}

func TestSuspendEnclave_Success(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "sgx", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	err = k.SuspendEnclave(ctx, testAddr)
	require.NoError(t, err)

	record, err := k.GetEnclave(ctx, testAddr)
	require.NoError(t, err)
	require.Equal(t, types.EnclaveStatusSuspendedStr, record.Status)
}

func TestRevokeEnclave_Success(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.RegisterEnclave(ctx, testAddr, "sev", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	err = k.RevokeEnclave(ctx, testAddr)
	require.NoError(t, err)

	record, err := k.GetEnclave(ctx, testAddr)
	require.NoError(t, err)
	require.Equal(t, types.EnclaveStatusRevokedStr, record.Status)
}

func TestEnclaveStatusTransitions(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Register → Active
	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	record, _ := k.GetEnclave(ctx, testAddr)
	require.Equal(t, types.EnclaveStatusActiveStr, record.Status)

	// Active → Suspended
	err = k.SuspendEnclave(ctx, testAddr)
	require.NoError(t, err)
	record, _ = k.GetEnclave(ctx, testAddr)
	require.Equal(t, types.EnclaveStatusSuspendedStr, record.Status)

	// Suspended → Revoked
	err = k.RevokeEnclave(ctx, testAddr)
	require.NoError(t, err)
	record, _ = k.GetEnclave(ctx, testAddr)
	require.Equal(t, types.EnclaveStatusRevokedStr, record.Status)

	// Revoked → cannot change
	err = k.SuspendEnclave(ctx, testAddr)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveNotActive)

	err = k.RevokeEnclave(ctx, testAddr)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrEnclaveNotActive)
}

func TestTEEMsgServer_RegisterEnclave(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTEEMsgServerImpl(k)

	resp, err := srv.RegisterEnclave(ctx, &types.MsgRegisterEnclave{
		Operator:     testAddr,
		Provider:     "nitro",
		Attestation:  []byte("doc"),
		Measurements: []byte("hash"),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.EnclaveId)
}

func TestTEEMsgServer_RegisterEnclave_InvalidAddress(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTEEMsgServerImpl(k)

	_, err := srv.RegisterEnclave(ctx, &types.MsgRegisterEnclave{
		Operator:     "invalid-addr",
		Provider:     "nitro",
		Attestation:  []byte("doc"),
		Measurements: []byte("hash"),
	})
	require.Error(t, err)
}

func TestTEEMsgServer_VerifyAttestation(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTEEMsgServerImpl(k)

	// Register first.
	_, err := srv.RegisterEnclave(ctx, &types.MsgRegisterEnclave{
		Operator:     testAddr,
		Provider:     "nitro",
		Attestation:  []byte("doc"),
		Measurements: []byte("hash"),
	})
	require.NoError(t, err)

	// Verify.
	resp, err := srv.VerifyAttestation(ctx, &types.MsgVerifyAttestation{
		Operator:    testAddr,
		Attestation: []byte("fresh-doc"),
	})
	require.NoError(t, err)
	require.True(t, resp.Valid)
}

func TestTEEMsgServer_SuspendEnclave_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTEEMsgServerImpl(k)

	// Register enclave.
	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	// Non-authority tries to suspend.
	_, err = srv.SuspendEnclave(ctx, &types.MsgSuspendEnclave{
		Authority: testAddr, // Not the governance authority
		Operator:  testAddr,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestTEEMsgServer_SuspendEnclave_Authorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTEEMsgServerImpl(k)

	// Register enclave.
	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	// Authority suspends.
	_, err = srv.SuspendEnclave(ctx, &types.MsgSuspendEnclave{
		Authority: "authority",
		Operator:  testAddr,
	})
	require.NoError(t, err)
}

func TestTEEMsgServer_RevokeEnclave_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTEEMsgServerImpl(k)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	_, err = srv.RevokeEnclave(ctx, &types.MsgRevokeEnclave{
		Authority: testAddr,
		Operator:  testAddr,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestTEEMsgServer_RevokeEnclave_Authorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewTEEMsgServerImpl(k)

	_, err := k.RegisterEnclave(ctx, testAddr, "nitro", []byte("doc"), []byte("hash"))
	require.NoError(t, err)

	_, err = srv.RevokeEnclave(ctx, &types.MsgRevokeEnclave{
		Authority: "authority",
		Operator:  testAddr,
	})
	require.NoError(t, err)

	record, err := k.GetEnclave(ctx, testAddr)
	require.NoError(t, err)
	require.Equal(t, types.EnclaveStatusRevokedStr, record.Status)
}

func TestMsgRegisterEnclave_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgRegisterEnclave
		wantErr bool
	}{
		{
			name: "valid",
			msg: types.MsgRegisterEnclave{
				Operator:     testAddr,
				Provider:     "nitro",
				Attestation:  []byte("doc"),
				Measurements: []byte("hash"),
			},
			wantErr: false,
		},
		{
			name: "invalid operator",
			msg: types.MsgRegisterEnclave{
				Operator:     "bad",
				Provider:     "nitro",
				Attestation:  []byte("doc"),
				Measurements: []byte("hash"),
			},
			wantErr: true,
		},
		{
			name: "invalid provider",
			msg: types.MsgRegisterEnclave{
				Operator:     testAddr,
				Provider:     "unknown",
				Attestation:  []byte("doc"),
				Measurements: []byte("hash"),
			},
			wantErr: true,
		},
		{
			name: "empty attestation",
			msg: types.MsgRegisterEnclave{
				Operator:     testAddr,
				Provider:     "nitro",
				Attestation:  nil,
				Measurements: []byte("hash"),
			},
			wantErr: true,
		},
		{
			name: "empty measurements",
			msg: types.MsgRegisterEnclave{
				Operator:     testAddr,
				Provider:     "nitro",
				Attestation:  []byte("doc"),
				Measurements: nil,
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

func TestMsgVerifyAttestation_ValidateBasic(t *testing.T) {
	tests := []struct {
		name    string
		msg     types.MsgVerifyAttestation
		wantErr bool
	}{
		{
			name: "valid",
			msg: types.MsgVerifyAttestation{
				Operator:    testAddr,
				Attestation: []byte("doc"),
			},
			wantErr: false,
		},
		{
			name: "invalid verifier",
			msg: types.MsgVerifyAttestation{
				Operator:    "bad",
				Attestation: []byte("doc"),
			},
			wantErr: true,
		},
		{
			name: "empty attestation",
			msg: types.MsgVerifyAttestation{
				Operator:    testAddr,
				Attestation: nil,
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
