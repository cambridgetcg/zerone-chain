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

// ─── SubmitData tests ───────────────────────────────────────────────────────

func TestSubmitData_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "This is valid training data content",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	resp, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.SubmissionId)

	// Verify submission stored
	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.Equal(t, msg.Content, sub.Content)
	require.Equal(t, msg.Domain, sub.Domain)
	require.Equal(t, msg.Submitter, sub.Submitter)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING, sub.Status)
	require.NotEmpty(t, sub.ContentHash)
	require.Equal(t, uint64(100), sub.SubmittedAtBlock)

	// Verify content hash indexed
	require.True(t, k.HasContentHash(ctx, sub.ContentHash))

	// Verify domain index
	ids := k.GetSubmissionsByDomain(ctx, "technology")
	require.Contains(t, ids, resp.SubmissionId)

	// Verify submitter index
	ids = k.GetSubmissionsBySubmitter(ctx, testAddr)
	require.Contains(t, ids, resp.SubmissionId)
}

func TestSubmitData_ContentTooLarge(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	bigContent := make([]byte, 50_001)
	for i := range bigContent {
		bigContent[i] = 'a'
	}

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    string(bigContent),
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrContentTooLarge)
}

func TestSubmitData_DuplicateContent(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "duplicate content test",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)

	_, err = k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrDuplicateContent)
}

func TestSubmitData_InvalidConsent(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    nil,
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrConsentRequired)
}

func TestSubmitData_InvalidDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "nonexistent",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestSubmitData_InsufficientStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test content",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "100",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestSubmitData_Sponsored(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "sponsored content",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
		Sponsored:  true,
	}

	resp, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.SubmissionId)

	// Verify module-to-module transfer was called
	require.Len(t, bk.moduleToModuleCalls, 1)
	require.Equal(t, types.BootstrapFundModuleName, bk.moduleToModuleCalls[0].from)
	require.Equal(t, types.ModuleName, bk.moduleToModuleCalls[0].to)

	// Verify submission is marked as sponsored
	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.True(t, sub.Sponsored)
}

func TestSubmitData_SponsoredInsufficientFunds(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	bk.failNextSend = true

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "sponsored content fail",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
		Sponsored:  true,
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestSubmitData_SelfFundedInsufficientFunds(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	bk.failNextSend = true

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "self funded fail",
		SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
		Sponsored:  false,
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}
