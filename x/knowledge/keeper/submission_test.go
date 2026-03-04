package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW, sub.Status)
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

// ─── SubmitThread tests ─────────────────────────────────────────────────────

func TestSubmitThread_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-1",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "First message", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "Second message", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "Third message", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	}

	resp, err := k.SubmitThread(ctx, msg)
	require.NoError(t, err)
	require.Len(t, resp.SubmissionIds, 3)
	require.Equal(t, "thread-1", resp.ThreadId)

	// Verify parent chain linking
	for i, id := range resp.SubmissionIds {
		sub, found := k.GetSubmission(ctx, id)
		require.True(t, found)
		require.Equal(t, "thread-1", sub.ThreadId)
		require.Equal(t, "technology", sub.Domain)
		require.Equal(t, testAddr, sub.Submitter)
		if i > 0 {
			require.Equal(t, resp.SubmissionIds[i-1], sub.ParentSubmissionId)
		} else {
			require.Empty(t, sub.ParentSubmissionId)
		}
	}
}

func TestSubmitThread_TooLarge(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	items := make([]*types.MsgSubmitData, 21)
	for i := range items {
		items[i] = &types.MsgSubmitData{
			Content:    fmt.Sprintf("item %d", i),
			SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION,
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		}
	}

	_, err := k.SubmitThread(ctx, &types.MsgSubmitThread{
		Submitter: testAddr, ThreadId: "thread-big", Domain: "technology", Stake: "1000000", Items: items,
	})
	require.ErrorIs(t, err, types.ErrThreadTooLarge)
}

func TestSubmitThread_DuplicateInThread(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.SubmitThread(ctx, &types.MsgSubmitThread{
		Submitter: testAddr, ThreadId: "thread-dup", Domain: "technology", Stake: "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "same content", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "same content", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	})
	require.ErrorIs(t, err, types.ErrDuplicateContent)
}

func TestSubmitThread_InvalidDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.SubmitThread(ctx, &types.MsgSubmitThread{
		Submitter: testAddr, ThreadId: "t", Domain: "nonexistent", Stake: "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "a", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "b", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestSubmitThread_InsufficientStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.SubmitThread(ctx, &types.MsgSubmitThread{
		Submitter: testAddr, ThreadId: "t-low", Domain: "technology", Stake: "100",
		Items: []*types.MsgSubmitData{
			{Content: "a", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "b", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	})
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestSubmitThread_ItemConsentInvalid(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.SubmitThread(ctx, &types.MsgSubmitThread{
		Submitter: testAddr, ThreadId: "t-consent", Domain: "technology", Stake: "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "ok", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "bad", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN}},
		},
	})
	require.ErrorIs(t, err, types.ErrInvalidConsent)
}

func TestSubmitThread_ItemContentTooLarge(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	big := make([]byte, 50_001)
	for i := range big {
		big[i] = 'x'
	}

	_, err := k.SubmitThread(ctx, &types.MsgSubmitThread{
		Submitter: testAddr, ThreadId: "t-big", Domain: "technology", Stake: "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "small", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: string(big), SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	})
	require.ErrorIs(t, err, types.ErrContentTooLarge)
}

// ─── Edge-case & event tests ────────────────────────────────────────────────

func TestSubmitData_EventEmitted(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "event test content",
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	resp, err := k.SubmitData(sdkCtx, msg)
	require.NoError(t, err)

	events := sdkCtx.EventManager().Events()
	var found bool
	for _, e := range events {
		if e.Type == "submit_data" {
			found = true
			for _, attr := range e.Attributes {
				if attr.Key == "submission_id" {
					require.Equal(t, resp.SubmissionId, attr.Value)
				}
			}
		}
	}
	require.True(t, found, "submit_data event not emitted")
}

func TestSubmitThread_EventEmitted(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-event",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "e1", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "e2", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	}

	_, err := k.SubmitThread(sdkCtx, msg)
	require.NoError(t, err)

	events := sdkCtx.EventManager().Events()
	var found bool
	for _, e := range events {
		if e.Type == "submit_thread" {
			found = true
		}
	}
	require.True(t, found, "submit_thread event not emitted")
}

func TestSubmitData_AllConsentTypes(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	consentCases := []struct {
		name    string
		consent *types.ConsentProof
	}{
		{"self_authored", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		{"opt_in_sig", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN, AuthorSignature: "sig"}},
		{"opt_in_uri", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN, ProofUri: "https://x.com"}},
		{"public_license", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, ProofUri: "https://mit.edu"}},
		{"platform_tos", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PLATFORM_TOS, ProofUri: "https://tos.com"}},
		{"fair_use", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_FAIR_USE}},
	}

	for i, tc := range consentCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := &types.MsgSubmitData{
				Submitter:  testAddr,
				Content:    fmt.Sprintf("consent test %d %s", i, tc.name),
				SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
				Domain:     "technology",
				Consent:    tc.consent,
				Stake:      "1000000",
			}
			_, err := k.SubmitData(ctx, msg)
			require.NoError(t, err)
		})
	}
}

func TestSubmitData_InactiveDomain(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "inactive",
		Status: types.DomainStatus_DOMAIN_STATUS_PROPOSED,
	}))

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
		Domain:     "inactive",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidDomain)
}

func TestSubmitData_ZeroStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "zero stake test",
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "0",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestSubmitData_EmptyStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "empty stake test",
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "",
	}

	_, err := k.SubmitData(ctx, msg)
	require.Error(t, err)
}

func TestSubmitData_WithThreadAndParent(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:          testAddr,
		Content:            "child content with thread",
		SampleType:         types.SampleType_SAMPLE_TYPE_DISCUSSION,
		Domain:             "technology",
		Consent:            &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:              "1000000",
		ThreadId:           "thread-x",
		ParentSubmissionId: "parent-1",
	}

	resp, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)

	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.Equal(t, "thread-x", sub.ThreadId)
	require.Equal(t, "parent-1", sub.ParentSubmissionId)
}

func TestSubmitData_MultipleDomains(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	for _, domain := range []string{"technology", "science"} {
		msg := &types.MsgSubmitData{
			Submitter:  testAddr,
			Content:    "content for " + domain,
			SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
			Domain:     domain,
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			Stake:      "1000000",
		}
		_, err := k.SubmitData(ctx, msg)
		require.NoError(t, err)
	}

	techIds := k.GetSubmissionsByDomain(ctx, "technology")
	sciIds := k.GetSubmissionsByDomain(ctx, "science")
	require.Len(t, techIds, 1)
	require.Len(t, sciIds, 1)
}

func TestContentHash_Deterministic(t *testing.T) {
	k, _ := setupKeeper(t)
	content := "deterministic hash test"
	h1 := k.ComputeContentHash(content)
	h2 := k.ComputeContentHash(content)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 64) // SHA-256 hex = 64 chars
}

func TestSubmitData_StakeLocking(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "stake locking test",
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)

	// Verify bank was called
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, testAddr, bk.accountToModuleCalls[0].from)
	require.Equal(t, "knowledge", bk.accountToModuleCalls[0].to)
}

func TestSubmitThread_StakeLocking(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-stake",
		Domain:    "technology",
		Stake:     "2000000",
		Items: []*types.MsgSubmitData{
			{Content: "s1", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "s2", SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	}

	_, err := k.SubmitThread(ctx, msg)
	require.NoError(t, err)

	// Single stake for entire thread
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, "knowledge", bk.accountToModuleCalls[0].to)
}

// ─── Quality round wiring tests ─────────────────────────────────────────────

func TestSubmitData_InitiatesQualityRound(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.SubmitData(ctx, &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "quality round test",
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	})
	require.NoError(t, err)

	roundID, found := k.GetRoundBySubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.NotEmpty(t, roundID)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, resp.SubmissionId, round.SubmissionId)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMMIT, round.Phase)

	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW, sub.Status)
}

func TestSubmitThread_InitiatesOneQualityRound(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.SubmitThread(ctx, &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-test",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "msg1", Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}, SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION},
			{Content: "msg2", Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}, SampleType: types.SampleType_SAMPLE_TYPE_DISCUSSION},
		},
	})
	require.NoError(t, err)

	var roundIDs []string
	for _, sid := range resp.SubmissionIds {
		rid, found := k.GetRoundBySubmission(ctx, sid)
		require.True(t, found)
		roundIDs = append(roundIDs, rid)
	}
	require.Equal(t, roundIDs[0], roundIDs[1], "all thread submissions should share one round")
}
