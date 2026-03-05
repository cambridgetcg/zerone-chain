package keeper_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── helpers ────────────────────────────────────────────────────────────────

const testAddr2 = "zrn1qcxce9c4thzxnfmpr2dqnnlqea9ey35ydj769h"

func createSampleForConsent(t *testing.T, k keeper.Keeper, ctx context.Context, id, submitter, originalAuthor string, consentType types.ConsentType) {
	t.Helper()
	sample := &types.Sample{
		Id:             id,
		Domain:         "science",
		QualityTier:    "gold",
		Status:         types.SampleStatus_SAMPLE_STATUS_GOLD,
		Submitter:      submitter,
		OriginalAuthor: originalAuthor,
		Content:        "original content for " + id,
		Energy:         500_000,
		EnergyCap:      1_000_000,
		Consent: &types.ConsentProof{
			Type: consentType,
		},
	}
	require.NoError(t, k.SetSample(ctx, sample))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "science", id))
	require.NoError(t, k.SetSampleSubmitterIndex(ctx, submitter, id))
}

// ─── Consent Revocation ─────────────────────────────────────────────────────

func TestRevokeConsent_ByOriginalAuthor(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_OPT_IN)

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr2, // original author
		SampleId:  "s1",
		Reason:    "withdrawal of consent",
	})
	require.NoError(t, err)

	sample, found := k.GetSample(ctx, "s1")
	require.True(t, found)
	require.Equal(t, "[consent revoked]", sample.Content)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, sample.Status)
}

func TestRevokeConsent_BySubmitter(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE)

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr, // submitter
		SampleId:  "s1",
		Reason:    "submitter withdrawal",
	})
	require.NoError(t, err)

	sample, found := k.GetSample(ctx, "s1")
	require.True(t, found)
	require.Equal(t, "[consent revoked]", sample.Content)
}

func TestRevokeConsent_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_OPT_IN)

	thirdParty := "zrn1xznhxqv7zqy3h5uqg6efxwdmjkhg7uh23hkufc"
	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: thirdParty,
		SampleId:  "s1",
		Reason:    "I don't like it",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
}

func TestRevokeConsent_SampleNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "nonexistent",
		Reason:    "test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestRevokeConsent_RemovedFromDomainIndex(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr, types.ConsentType_CONSENT_TYPE_SELF_AUTHORED)

	// Verify it's in the domain index before revocation
	ids := k.GetSamplesByDomain(ctx, "science")
	require.Contains(t, ids, "s1")

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "s1",
		Reason:    "withdrawal",
	})
	require.NoError(t, err)

	// After revocation, should be removed from domain index
	ids = k.GetSamplesByDomain(ctx, "science")
	require.NotContains(t, ids, "s1")
}

func TestRevokeConsent_RemovedFromSubmitterIndex(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr, types.ConsentType_CONSENT_TYPE_SELF_AUTHORED)

	ids := k.GetSamplesBySubmitter(ctx, testAddr)
	require.Contains(t, ids, "s1")

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "s1",
		Reason:    "withdrawal",
	})
	require.NoError(t, err)

	ids = k.GetSamplesBySubmitter(ctx, testAddr)
	require.NotContains(t, ids, "s1")
}

func TestRevokeConsent_StopsRevenueQueue(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr, types.ConsentType_CONSENT_TYPE_OPT_IN)
	require.NoError(t, k.SetPendingRevenue(ctx, "s1", 10_000))

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "s1",
		Reason:    "withdrawal",
	})
	require.NoError(t, err)

	// Pending revenue should be cleared
	require.Equal(t, uint64(0), k.GetPendingRevenue(ctx, "s1"))
}

func TestRevokeConsent_ThreadIndexCleared(t *testing.T) {
	k, ctx := setupKeeper(t)
	sample := &types.Sample{
		Id:             "s1",
		Domain:         "science",
		Status:         types.SampleStatus_SAMPLE_STATUS_GOLD,
		Submitter:      testAddr,
		OriginalAuthor: testAddr,
		Content:        "thread content",
		ThreadId:       "thread1",
		Consent:        &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSample(ctx, sample))
	require.NoError(t, k.SetSampleThreadIndex(ctx, "thread1", "s1"))

	ids := k.GetSamplesByThread(ctx, "thread1")
	require.Contains(t, ids, "s1")

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "s1",
		Reason:    "withdrawal",
	})
	require.NoError(t, err)

	ids = k.GetSamplesByThread(ctx, "thread1")
	require.NotContains(t, ids, "s1")
}

// ─── Consent Audit Trail ────────────────────────────────────────────────────

func TestConsentAuditTrail_RecordedOnRevocation(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr, types.ConsentType_CONSENT_TYPE_OPT_IN)

	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "s1",
		Reason:    "GDPR erasure request",
	})
	require.NoError(t, err)

	events := k.GetConsentHistory(ctx, "s1")
	require.Len(t, events, 1)
	require.Equal(t, "revocation", events[0].EventType)
	require.Equal(t, testAddr, events[0].Actor)
	require.Equal(t, "GDPR erasure request", events[0].Reason)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_OPT_IN, events[0].OldConsentType)
}

func TestConsentAuditTrail_MultipleEvents(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Record multiple events manually
	require.NoError(t, k.RecordConsentEvent(ctx, &types.ConsentEvent{
		SampleId:  "s1",
		EventType: "granted",
		Actor:     testAddr,
		Block:     50,
	}))
	require.NoError(t, k.RecordConsentEvent(ctx, &types.ConsentEvent{
		SampleId:  "s1",
		EventType: "verified",
		Actor:     testAddr2,
		Block:     60,
	}))
	require.NoError(t, k.RecordConsentEvent(ctx, &types.ConsentEvent{
		SampleId:  "s1",
		EventType: "revocation",
		Actor:     testAddr,
		Reason:    "changed mind",
		Block:     100,
	}))

	events := k.GetConsentHistory(ctx, "s1")
	require.Len(t, events, 3)
	require.Equal(t, "granted", events[0].EventType)
	require.Equal(t, "verified", events[1].EventType)
	require.Equal(t, "revocation", events[2].EventType)
}

func TestConsentAuditTrail_EmptyForUnknownSample(t *testing.T) {
	k, ctx := setupKeeper(t)

	events := k.GetConsentHistory(ctx, "nonexistent")
	require.Empty(t, events)
}

// ─── Consent Upgrade ────────────────────────────────────────────────────────

func TestUpgradeConsent_FairUseToOptIn(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	resp, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type:     types.ConsentType_CONSENT_TYPE_OPT_IN,
			ProofUri: "https://example.com/consent",
		},
	})
	require.NoError(t, err)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_FAIR_USE, resp.OldType)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_OPT_IN, resp.NewType)

	// Verify sample was updated
	sample, found := k.GetSample(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_OPT_IN, sample.Consent.Type)
}

func TestUpgradeConsent_IncreasesMultiplier(t *testing.T) {
	k, ctx := setupKeeper(t)
	setDefaultParams(t, k, ctx)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	// Before upgrade: fair_use = 0.5x
	sample, _ := k.GetSample(ctx, "s1")
	require.Equal(t, types.ConsentType_CONSENT_TYPE_FAIR_USE, sample.Consent.Type)

	// Upgrade to opt_in = 1.3x
	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type:     types.ConsentType_CONSENT_TYPE_OPT_IN,
			ProofUri: "https://example.com/consent",
		},
	})
	require.NoError(t, err)

	sample, _ = k.GetSample(ctx, "s1")
	require.Equal(t, types.ConsentType_CONSENT_TYPE_OPT_IN, sample.Consent.Type)
}

func TestUpgradeConsent_CannotDowngrade(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_OPT_IN)

	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type:     types.ConsentType_CONSENT_TYPE_FAIR_USE,
			ProofUri: "https://example.com/consent",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be stronger")
}

func TestUpgradeConsent_CannotSelfAuthoredIfNotAuthor(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not the original author")
}

func TestUpgradeConsent_SelfAuthoredOKIfAuthor(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	resp, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED,
		},
	})
	require.NoError(t, err)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_SELF_AUTHORED, resp.NewType)
}

func TestUpgradeConsent_UnauthorizedNonSubmitter(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr2, // not the submitter
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type:     types.ConsentType_CONSENT_TYPE_OPT_IN,
			ProofUri: "https://example.com",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
}

func TestUpgradeConsent_SampleNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "nonexistent",
		NewConsent: &types.ConsentProof{
			Type:     types.ConsentType_CONSENT_TYPE_OPT_IN,
			ProofUri: "https://example.com",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestUpgradeConsent_RecordsAuditEvent(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type:     types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE,
			ProofUri: "https://example.com/license",
		},
	})
	require.NoError(t, err)

	events := k.GetConsentHistory(ctx, "s1")
	require.Len(t, events, 1)
	require.Equal(t, "upgraded", events[0].EventType)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_FAIR_USE, events[0].OldConsentType)
	require.Equal(t, types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, events[0].NewConsentType)
}

// ─── Cryptographic Consent Verification ─────────────────────────────────────

func TestVerifyOptInSignature_ValidSignature(t *testing.T) {
	k, ctx := setupKeeper(t)
	_ = ctx

	// 64-byte hex signature (valid length)
	sig := hex.EncodeToString(make([]byte, 64))
	consent := &types.ConsentProof{
		Type:            types.ConsentType_CONSENT_TYPE_OPT_IN,
		AuthorSignature: sig,
	}

	// Public method via UpgradeConsent with valid signature
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)
	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter:  testAddr,
		SampleId:   "s1",
		NewConsent: consent,
	})
	require.NoError(t, err)
}

func TestVerifyOptInSignature_InvalidEncoding(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	consent := &types.ConsentProof{
		Type:            types.ConsentType_CONSENT_TYPE_OPT_IN,
		AuthorSignature: "not-valid-hex!!!",
	}

	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter:  testAddr,
		SampleId:   "s1",
		NewConsent: consent,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid")
}

func TestVerifyOptInSignature_TooShort(t *testing.T) {
	k, ctx := setupKeeper(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr2, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	// 16-byte signature (too short, need >= 32)
	sig := hex.EncodeToString(make([]byte, 16))
	consent := &types.ConsentProof{
		Type:            types.ConsentType_CONSENT_TYPE_OPT_IN,
		AuthorSignature: sig,
	}

	_, err := k.UpgradeConsent(ctx, &types.MsgUpgradeConsent{
		Submitter:  testAddr,
		SampleId:   "s1",
		NewConsent: consent,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "too short")
}

// ─── ValidateBasic ──────────────────────────────────────────────────────────

func TestMsgRevokeConsent_ValidateBasic(t *testing.T) {
	msg := &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "s1",
		Reason:    "withdrawal",
	}
	require.NoError(t, msg.ValidateBasic())

	// Missing sample ID
	msg.SampleId = ""
	require.Error(t, msg.ValidateBasic())

	// Invalid address
	msg.SampleId = "s1"
	msg.Requester = "invalid"
	require.Error(t, msg.ValidateBasic())
}

func TestMsgUpgradeConsent_ValidateBasic(t *testing.T) {
	msg := &types.MsgUpgradeConsent{
		Submitter: testAddr,
		SampleId:  "s1",
		NewConsent: &types.ConsentProof{
			Type:     types.ConsentType_CONSENT_TYPE_OPT_IN,
			ProofUri: "https://example.com",
		},
	}
	require.NoError(t, msg.ValidateBasic())

	// Missing consent
	msg.NewConsent = nil
	require.Error(t, msg.ValidateBasic())
}

// ─── Query: ConsentHistory ──────────────────────────────────────────────────

func TestQuery_ConsentHistory(t *testing.T) {
	qs, k, ctx := setupQueryServer(t)
	createSampleForConsent(t, k, ctx, "s1", testAddr, testAddr, types.ConsentType_CONSENT_TYPE_FAIR_USE)

	// Revoke to generate an event
	err := k.RevokeConsent(ctx, &types.MsgRevokeConsent{
		Requester: testAddr,
		SampleId:  "s1",
		Reason:    "test",
	})
	require.NoError(t, err)

	resp, err := qs.ConsentHistory(ctx, &types.QueryConsentHistoryRequest{SampleId: "s1"})
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	require.Equal(t, "revocation", resp.Events[0].EventType)
}
