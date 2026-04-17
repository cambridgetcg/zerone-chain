package keeper_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Mock DomainQualificationKeeper ──────────────────────────────────────────

type mockDomainQualificationKeeper struct {
	// qualified maps "validator/domain" → bool
	qualified map[string]bool
	// outcomes records RecordVerificationOutcome calls
	outcomes []outcomeRecord
}

type outcomeRecord struct {
	Validator string
	Domain    string
	Accepted  bool
}

func newMockDomainQualificationKeeper() *mockDomainQualificationKeeper {
	return &mockDomainQualificationKeeper{
		qualified: make(map[string]bool),
	}
}

func (dk *mockDomainQualificationKeeper) qualify(validator, domain string) {
	dk.qualified[validator+"/"+domain] = true
}

func (dk *mockDomainQualificationKeeper) IsQualified(_ context.Context, validatorAddr, domain string) (bool, error) {
	return dk.qualified[validatorAddr+"/"+domain], nil
}

func (dk *mockDomainQualificationKeeper) GetQualificationWeight(_ context.Context, validatorAddr, domain string) (uint64, error) {
	if dk.qualified[validatorAddr+"/"+domain] {
		return 100, nil
	}
	return 0, nil
}

func (dk *mockDomainQualificationKeeper) GetQualifiedValidators(_ context.Context, domain string) ([]string, error) {
	var result []string
	for key, q := range dk.qualified {
		if !q {
			continue
		}
		// Parse "validator/domain" — find the last "/" since validator might contain "/"
		for i := len(key) - 1; i >= 0; i-- {
			if key[i] == '/' {
				if key[i+1:] == domain {
					result = append(result, key[:i])
				}
				break
			}
		}
	}
	return result, nil
}

func (dk *mockDomainQualificationKeeper) RecordVerificationOutcome(_ context.Context, validatorAddr, domain string, accepted bool) error {
	dk.outcomes = append(dk.outcomes, outcomeRecord{
		Validator: validatorAddr,
		Domain:    domain,
		Accepted:  accepted,
	})
	return nil
}

// ─── GetEligibleValidators Tests ─────────────────────────────────────────────

func TestGetEligibleValidators_NilKeeper_ReturnsAll(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")
	sk.addValidator("zrn1val2", 300_000, "bonded")

	vals, err := k.GetEligibleValidators(ctx, "general")
	require.NoError(t, err)
	require.Len(t, vals, 2, "nil qualification keeper should return all validators")
}

func TestGetEligibleValidators_FiltersUnqualified(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")
	sk.addValidator("zrn1val2", 300_000, "bonded")
	sk.addValidator("zrn1val3", 200_000, "bonded")
	sk.addValidator("zrn1val4", 100_000, "bonded")
	sk.addValidator("zrn1val5", 50_000, "bonded")

	// Qualify 4 — meets effective min (MinVerifiers=3 + 1 nil-partnership = 4).
	dk := newMockDomainQualificationKeeper()
	dk.qualify("zrn1val1", "general")
	dk.qualify("zrn1val3", "general")
	dk.qualify("zrn1val4", "general")
	dk.qualify("zrn1val5", "general")
	k.SetDomainQualificationKeeper(dk)

	vals, err := k.GetEligibleValidators(ctx, "general")
	require.NoError(t, err)
	require.Len(t, vals, 4, "should filter out unqualified validators")

	addrs := make(map[string]bool)
	for _, v := range vals {
		addrs[v.Address] = true
	}
	require.True(t, addrs["zrn1val1"])
	require.False(t, addrs["zrn1val2"], "unqualified val2 should be filtered")
	require.True(t, addrs["zrn1val3"])
	require.True(t, addrs["zrn1val4"])
	require.True(t, addrs["zrn1val5"])
}

func TestGetEligibleValidators_FallbackWhenInsufficientQualified(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")
	sk.addValidator("zrn1val2", 300_000, "bonded")
	sk.addValidator("zrn1val3", 200_000, "bonded")

	// Only qualify 1 validator — less than MinVerifiers (default 3)
	dk := newMockDomainQualificationKeeper()
	dk.qualify("zrn1val1", "physics")
	k.SetDomainQualificationKeeper(dk)

	vals, err := k.GetEligibleValidators(ctx, "physics")
	require.NoError(t, err)
	require.Len(t, vals, 3, "should fall back to all validators when insufficient qualified")
}

func TestGetEligibleValidators_EmptyDomain_ReturnsAll(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")

	dk := newMockDomainQualificationKeeper()
	k.SetDomainQualificationKeeper(dk)

	vals, err := k.GetEligibleValidators(ctx, "")
	require.NoError(t, err)
	require.Len(t, vals, 1, "empty domain should return all validators")
}

// ─── SubmitCommitment Qualification Gate Tests ───────────────────────────────

func TestSubmitCommitment_QualifiedVerifier_Accepted(t *testing.T) {
	k, ctx, bk, sk := setupKnowledgeTestFull(t)
	_ = sk

	verifier := makeValidBech32Addr("verifier1")
	bk.balances[verifier] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 200_000_000))

	// Create domain
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "general",
		Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Set up qualification keeper — qualify the verifier
	dk := newMockDomainQualificationKeeper()
	dk.qualify(verifier, "general")
	// Also qualify enough validators to meet MinVerifiers threshold
	dk.qualify("zrn1other1", "general")
	dk.qualify("zrn1other2", "general")
	k.SetDomainQualificationKeeper(dk)

	claim, round := makeTestClaim(t, k, ctx, makeValidBech32Addr("submitter1"),
		"test claim content", "general", "computational", "1000000")
	_ = claim

	// Submit commitment from qualified verifier
	msgServer := keeper.NewMsgServerImpl(k)
	commitHash := sha256.Sum256([]byte("accept" + "salt123"))
	_, err := msgServer.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		RoundId:    round.Id,
		Verifier:   verifier,
		CommitHash: commitHash[:],
	})
	require.NoError(t, err, "qualified verifier should be accepted")
}

func TestSubmitCommitment_UnqualifiedVerifier_Rejected(t *testing.T) {
	k, ctx, bk, sk := setupKnowledgeTestFull(t)
	_ = sk

	verifier := makeValidBech32Addr("verifier1")
	bk.balances[verifier] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 200_000_000))

	// Create domain
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "general",
		Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Set up qualification keeper — qualify enough OTHER validators but NOT the verifier.
	// Need >= effective min (MinVerifiers=3 + 1 nil-partnership = 4) to trigger rejection.
	dk := newMockDomainQualificationKeeper()
	dk.qualify("zrn1qualified1", "general")
	dk.qualify("zrn1qualified2", "general")
	dk.qualify("zrn1qualified3", "general")
	dk.qualify("zrn1qualified4", "general")
	k.SetDomainQualificationKeeper(dk)

	claim, round := makeTestClaim(t, k, ctx, makeValidBech32Addr("submitter1"),
		"test claim content for reject", "general", "computational", "1000000")
	_ = claim

	// Submit commitment from unqualified verifier
	msgServer := keeper.NewMsgServerImpl(k)
	commitHash := sha256.Sum256([]byte("accept" + "salt123"))
	_, err := msgServer.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		RoundId:    round.Id,
		Verifier:   verifier,
		CommitHash: commitHash[:],
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not qualified")
}

func TestSubmitCommitment_UnqualifiedVerifier_FallbackAllowed(t *testing.T) {
	k, ctx, bk, sk := setupKnowledgeTestFull(t)
	_ = sk

	verifier := makeValidBech32Addr("verifier1")
	bk.balances[verifier] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 200_000_000))

	// Create domain
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "niche",
		Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Set up qualification keeper — qualify only 1 validator (< MinVerifiers=3)
	dk := newMockDomainQualificationKeeper()
	dk.qualify("zrn1onlyone", "niche")
	k.SetDomainQualificationKeeper(dk)

	claim, round := makeTestClaim(t, k, ctx, makeValidBech32Addr("submitter1"),
		"test claim niche content", "niche", "computational", "1000000")
	_ = claim

	// Submit commitment from unqualified verifier — should be allowed due to fallback
	msgServer := keeper.NewMsgServerImpl(k)
	commitHash := sha256.Sum256([]byte("accept" + "salt123"))
	_, err := msgServer.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		RoundId:    round.Id,
		Verifier:   verifier,
		CommitHash: commitHash[:],
	})
	require.NoError(t, err, "unqualified verifier should be allowed when insufficient qualified validators")
}

func TestSubmitCommitment_NilQualificationKeeper_Allowed(t *testing.T) {
	k, ctx, bk, sk := setupKnowledgeTestFull(t)
	_ = sk

	verifier := makeValidBech32Addr("verifier1")
	bk.balances[verifier] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 200_000_000))

	// Create domain
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "general",
		Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// No qualification keeper set — nil path
	claim, round := makeTestClaim(t, k, ctx, makeValidBech32Addr("submitter1"),
		"test claim nil keeper", "general", "computational", "1000000")
	_ = claim

	msgServer := keeper.NewMsgServerImpl(k)
	commitHash := sha256.Sum256([]byte("accept" + "salt123"))
	_, err := msgServer.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		RoundId:    round.Id,
		Verifier:   verifier,
		CommitHash: commitHash[:],
	})
	require.NoError(t, err, "nil qualification keeper should allow all verifiers")
}

// ─── RecordVerificationOutcome Tests ─────────────────────────────────────────

func TestCompleteRound_RecordsVerificationOutcomes(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1correct", 500_000, "bonded")
	sk.addValidator("zrn1wrong", 300_000, "bonded")

	dk := newMockDomainQualificationKeeper()
	k.SetDomainQualificationKeeper(dk)

	claim, round := makeTestClaim(t, k, ctx, "zrn1submitter1",
		"test outcome tracking", "general", "computational", "1000000")
	_ = claim

	// Complete round with rewards and slashes
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800_000,
		Rewards: []keeper.VerifierReward{
			{Verifier: "zrn1correct", Amount: 100},
		},
		Slashes: []keeper.VerifierSlash{
			{Verifier: "zrn1wrong", SlashBps: 50_000},
		},
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// Verify outcomes were recorded
	require.Len(t, dk.outcomes, 2)

	// Find the correct outcome
	var correctOutcome, wrongOutcome *outcomeRecord
	for i, o := range dk.outcomes {
		if o.Validator == "zrn1correct" {
			correctOutcome = &dk.outcomes[i]
		}
		if o.Validator == "zrn1wrong" {
			wrongOutcome = &dk.outcomes[i]
		}
	}

	require.NotNil(t, correctOutcome)
	require.True(t, correctOutcome.Accepted, "rewarded verifier should be recorded as correct")
	require.Equal(t, "general", correctOutcome.Domain)

	require.NotNil(t, wrongOutcome)
	require.False(t, wrongOutcome.Accepted, "slashed verifier should be recorded as incorrect")
	require.Equal(t, "general", wrongOutcome.Domain)
}

func TestCompleteRound_InconclusiveVerdict_NoOutcomes(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")

	dk := newMockDomainQualificationKeeper()
	k.SetDomainQualificationKeeper(dk)

	claim, round := makeTestClaim(t, k, ctx, "zrn1submitter1",
		"test inconclusive", "general", "computational", "1000000")
	_ = claim

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_INCONCLUSIVE,
		Confidence: 0,
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)
	require.Empty(t, dk.outcomes, "inconclusive verdict should not record outcomes")
}

func TestCompleteRound_NilQualificationKeeper_NoOutcomes(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")

	// No qualification keeper set
	claim, round := makeTestClaim(t, k, ctx, "zrn1submitter1",
		"test nil keeper outcomes", "general", "computational", "1000000")
	_ = claim

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800_000,
		Rewards: []keeper.VerifierReward{
			{Verifier: "zrn1val1", Amount: 100},
		},
	}

	// Should not panic — nil keeper path
	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)
}

func TestCompleteRound_EmptyDomain_NoOutcomes(t *testing.T) {
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")

	dk := newMockDomainQualificationKeeper()
	k.SetDomainQualificationKeeper(dk)

	// Create claim with empty domain
	claim, round := makeTestClaim(t, k, ctx, "zrn1submitter1",
		"test empty domain outcomes", "", "computational", "1000000")
	_ = claim

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800_000,
		Rewards: []keeper.VerifierReward{
			{Verifier: "zrn1val1", Amount: 100},
		},
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)
	require.Empty(t, dk.outcomes, "empty domain should skip outcome recording")
}

// ─── Error type test ─────────────────────────────────────────────────────────

func TestErrUnqualifiedVerifier_Exists(t *testing.T) {
	err := types.ErrUnqualifiedVerifier.Wrapf("test validator for domain test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not qualified")
	require.Contains(t, err.Error(), "test validator")
}

// ─── Adapter mock verification ───────────────────────────────────────────────

func TestMockDomainQualificationKeeper_GetQualifiedValidators(t *testing.T) {
	dk := newMockDomainQualificationKeeper()
	dk.qualify("zrn1val1", "general")
	dk.qualify("zrn1val2", "general")
	dk.qualify("zrn1val3", "physics")

	ctx := context.Background()
	vals, err := dk.GetQualifiedValidators(ctx, "general")
	require.NoError(t, err)
	require.Len(t, vals, 2)

	vals, err = dk.GetQualifiedValidators(ctx, "physics")
	require.NoError(t, err)
	require.Len(t, vals, 1)
	require.Equal(t, "zrn1val3", vals[0])

	vals, err = dk.GetQualifiedValidators(ctx, "empty")
	require.NoError(t, err)
	require.Empty(t, vals)
}

func TestGetEligibleValidators_UsesEffectiveMinVerifiers_FallbackWhenBelowEffective(t *testing.T) {
	// MinVerifiers=3, nil partnershipKeeper → effective=4.
	// With 3 qualified validators (== params.MinVerifiers), old code would NOT fall back.
	// New code: 3 < effective(4) → fallback fires → returns all 5 validators.
	k, ctx, _, sk := setupKnowledgeTestFull(t)
	sk.addValidator("zrn1val1", 500_000, "bonded")
	sk.addValidator("zrn1val2", 400_000, "bonded")
	sk.addValidator("zrn1val3", 300_000, "bonded")
	sk.addValidator("zrn1val4", 200_000, "bonded")
	sk.addValidator("zrn1val5", 100_000, "bonded")

	dk := newMockDomainQualificationKeeper()
	dk.qualify("zrn1val1", "physics")
	dk.qualify("zrn1val2", "physics")
	dk.qualify("zrn1val3", "physics")
	k.SetDomainQualificationKeeper(dk)

	vals, err := k.GetEligibleValidators(ctx, "physics")
	require.NoError(t, err)
	require.Len(t, vals, 5,
		"3 qualified < effective min 4 → fallback returns all validators")
}

// Suppress unused import warning
var _ = fmt.Sprintf
