package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestProposeDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer:    testAddr,
		Name:        "mathematics",
		Description: "Pure and applied mathematics",
		Stratum:     "empirical",
	})
	require.NoError(t, err)
	require.Equal(t, "mathematics", resp.ProposalId)

	domain, found := k.GetDomain(ctx, "mathematics")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_PROPOSED, domain.Status)
	require.Equal(t, testAddr, domain.Proposer)
	require.Len(t, domain.Endorsers, 1) // proposer auto-endorses
}

func TestProposeDomain_AlreadyExists(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "technology", // already exists
	})
	require.ErrorIs(t, err, types.ErrDomainExists)
}

func TestEndorseDomainProposal(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "mathematics",
	})
	require.NoError(t, err)

	addr2 := "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5z5r7e"
	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   addr2,
		ProposalId: "mathematics",
	})
	require.NoError(t, err)

	domain, _ := k.GetDomain(ctx, "mathematics")
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_PROPOSED, domain.Status)
	require.Len(t, domain.Endorsers, 2)
}

func TestEndorseDomainProposal_ActivatesAt3(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "mathematics",
	})
	require.NoError(t, err)

	addr2 := "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5z5r7e"
	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   addr2,
		ProposalId: "mathematics",
	})
	require.NoError(t, err)

	addr3 := "zrn1verifier1qqqqqqqqqqqqqqqqqqpvxfez"
	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   addr3,
		ProposalId: "mathematics",
	})
	require.NoError(t, err)

	domain, _ := k.GetDomain(ctx, "mathematics")
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, domain.Status)
	require.Len(t, domain.Endorsers, 3)
}

func TestEndorseDomainProposal_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   testAddr,
		ProposalId: "nonexistent",
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestEndorseDomainProposal_DuplicateEndorsement(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ProposeDomain(ctx, &types.MsgProposeDomain{
		Proposer: testAddr,
		Name:     "mathematics",
	})
	require.NoError(t, err)

	_, err = k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   testAddr,
		ProposalId: "mathematics",
	})
	require.ErrorIs(t, err, types.ErrInvalidDomain)
}

func TestEndorseDomainProposal_AlreadyActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.EndorseDomainProposal(ctx, &types.MsgEndorseDomainProposal{
		Endorser:   testAddr,
		ProposalId: "technology",
	})
	require.ErrorIs(t, err, types.ErrInvalidDomain)
}
