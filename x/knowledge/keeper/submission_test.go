package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestComputeContentHash(t *testing.T) {
	k, _ := setupKeeper(t)
	hash := k.ComputeContentHash("hello world")
	// SHA-256 of "hello world"
	require.Equal(t, "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", hash)
	// Same input = same hash
	require.Equal(t, hash, k.ComputeContentHash("hello world"))
	// Different input = different hash
	require.NotEqual(t, hash, k.ComputeContentHash("different"))
}

func TestCheckDuplicate(t *testing.T) {
	k, ctx := setupKeeper(t)

	hash := k.ComputeContentHash("unique content")
	require.NoError(t, k.CheckDuplicate(ctx, hash))

	// Store it
	require.NoError(t, k.SetContentHash(ctx, hash, "sub-1"))

	// Now it's a duplicate
	err := k.CheckDuplicate(ctx, hash)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrDuplicateContent)
}

func TestValidateConsent(t *testing.T) {
	k, _ := setupKeeper(t)

	tests := []struct {
		name    string
		consent *types.ConsentProof
		wantErr error
	}{
		{
			name:    "nil consent",
			consent: nil,
			wantErr: types.ErrConsentRequired,
		},
		{
			name:    "self authored valid",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			wantErr: nil,
		},
		{
			name:    "opt-in with signature",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN, AuthorSignature: "sig123"},
			wantErr: nil,
		},
		{
			name:    "opt-in with proof_uri",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN, ProofUri: "https://example.com/consent"},
			wantErr: nil,
		},
		{
			name:    "opt-in without proof",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN},
			wantErr: types.ErrInvalidConsent,
		},
		{
			name:    "public license with uri",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, ProofUri: "https://example.com/license"},
			wantErr: nil,
		},
		{
			name:    "public license without uri",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE},
			wantErr: types.ErrInvalidConsent,
		},
		{
			name:    "platform TOS with uri",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PLATFORM_TOS, ProofUri: "https://example.com/tos"},
			wantErr: nil,
		},
		{
			name:    "platform TOS without uri",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PLATFORM_TOS},
			wantErr: types.ErrInvalidConsent,
		},
		{
			name:    "fair use valid",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_FAIR_USE},
			wantErr: nil,
		},
		{
			name:    "unspecified type",
			consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_UNSPECIFIED},
			wantErr: types.ErrInvalidConsent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := k.ValidateConsent(tc.consent)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
