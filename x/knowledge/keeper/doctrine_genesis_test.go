package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	creedtypes "github.com/zerone-chain/zerone/x/creed/types"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestLoadDoctrineFacts_AllCommitmentsExist(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)
	require.NoError(t, k.LoadDoctrineFacts(ctx))

	// Truth-seeking commitments 1-20.
	for _, c := range creedtypes.CanonicalCommitments {
		id := fmt.Sprintf("commitment-%d", c.Number)
		f, found := k.GetFact(ctx, id)
		require.True(t, found, "TS commitment %s missing", id)
		require.Equal(t, types.FactStatus_FACT_STATUS_VERIFIED, f.Status)
		require.Equal(t, types.DoctrineDomainTruthSeeking, f.Domain)
		require.Equal(t, types.DoctrineConfidence, f.Confidence)
		require.Equal(t, uint32(0), f.AxiomDistance)
	}

	// ToK commitments TC1-TC6.
	for _, c := range creedtypes.CanonicalToKCommitments {
		id := fmt.Sprintf("commitment-%s", c.Number)
		f, found := k.GetFact(ctx, id)
		require.True(t, found, "TC commitment %s missing", id)
		require.Equal(t, types.DoctrineDomainToK, f.Domain)
	}

	// Useful-work UW + mechanisms + axes.
	uwFact, found := k.GetFact(ctx, "commitment-UW")
	require.True(t, found, "UW commitment missing")
	require.Equal(t, creedtypes.UsefulWorkStatement, uwFact.Content)
	require.Equal(t, types.DoctrineDomainUsefulWork, uwFact.Domain)

	for _, m := range creedtypes.CanonicalUsefulWorkMechanisms {
		id := fmt.Sprintf("mechanism-UW-M%d", m.Number)
		_, found := k.GetFact(ctx, id)
		require.True(t, found, "UW mechanism %s missing", id)
	}
	for _, axis := range creedtypes.CanonicalRecursiveAxes {
		id := fmt.Sprintf("axis-%s", axis)
		_, found := k.GetFact(ctx, id)
		require.True(t, found, "UW axis %s missing", id)
	}

	// Strange-loop SL + mechanisms.
	slFact, found := k.GetFact(ctx, "commitment-SL")
	require.True(t, found, "SL commitment missing")
	require.Equal(t, creedtypes.StrangeLoopStatement, slFact.Content)
	require.Equal(t, types.DoctrineDomainStrangeLoop, slFact.Domain)

	for _, m := range creedtypes.CanonicalStrangeLoopMechanisms {
		id := fmt.Sprintf("mechanism-SL-M%d", m.Number)
		_, found := k.GetFact(ctx, id)
		require.True(t, found, "SL mechanism %s missing", id)
	}
}

func TestLoadDoctrineFacts_DomainsCreated(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)
	require.NoError(t, k.LoadDoctrineFacts(ctx))

	for _, dom := range []string{
		types.DoctrineDomainTruthSeeking,
		types.DoctrineDomainToK,
		types.DoctrineDomainUsefulWork,
		types.DoctrineDomainStrangeLoop,
	} {
		d, found := k.GetDomain(ctx, dom)
		require.True(t, found, "doctrine domain %s missing", dom)
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, d.Status)
	}
}

func TestLoadDoctrineFacts_Idempotent(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)
	require.NoError(t, k.LoadDoctrineFacts(ctx))

	// Mutate one Fact.
	original, found := k.GetFact(ctx, "commitment-1")
	require.True(t, found)
	original.Content = "MUTATED CONTENT"
	require.NoError(t, k.SetFact(ctx, original))

	// Second run leaves mutated Fact alone.
	require.NoError(t, k.LoadDoctrineFacts(ctx))

	after, _ := k.GetFact(ctx, "commitment-1")
	require.Equal(t, "MUTATED CONTENT", after.Content,
		"LoadDoctrineFacts must be idempotent")
}

func TestLoadDoctrineFacts_EchoesEdgesCreated(t *testing.T) {
	k, ctx, _, _ := setupKnowledgeTestFull(t)
	require.NoError(t, k.LoadDoctrineFacts(ctx))

	// Sample known echo: UW SUPPORTS commitment-11.
	relations, err := k.GetFactRelations(ctx, "commitment-UW")
	require.NoError(t, err)

	var found bool
	for _, r := range relations {
		if r.TargetFactId == "commitment-11" && r.Relation == types.RelationType_RELATION_TYPE_SUPPORTS {
			found = true
			break
		}
	}
	require.True(t, found, "commitment-UW → commitment-11 SUPPORTS edge missing")
}
