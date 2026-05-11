package types_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/contribution/types"
)

func TestContributionClass_DenseNumbering(t *testing.T) {
	// 11 named classes, values 1..11 (UNSPECIFIED=0 is the sentinel).
	expected := []types.ContributionClass{
		types.ContributionClass_KNOWLEDGE_CLAIM,
		types.ContributionClass_IDEA,
		types.ContributionClass_TOOL,
		types.ContributionClass_DATASET,
		types.ContributionClass_EVAL_SUITE,
		types.ContributionClass_MODEL_ARTIFACT,
		types.ContributionClass_REASONING_TRACE,
		types.ContributionClass_COUNTEREXAMPLE,
		types.ContributionClass_ORCHESTRATION,
		types.ContributionClass_MODULE_PROPOSAL,
		types.ContributionClass_PIPELINE_IMPROVEMENT,
	}
	for i, c := range expected {
		require.Equal(t, types.ContributionClass(i+1), c, "class index %d should equal %d", i+1, c)
	}
}

func TestLifecyclePhase_NineValues(t *testing.T) {
	// 9 phases, 0..8 dense.
	require.Equal(t, types.LifecyclePhase(0), types.LifecyclePhase_PHASE_FOUNDATION)
	require.Equal(t, types.LifecyclePhase(8), types.LifecyclePhase_PHASE_TOOLS)
}

func TestAdapterRegistry_RegisterAndGet(t *testing.T) {
	r := types.NewAdapterRegistry()
	a := &fakeAdapter{class: types.ContributionClass_KNOWLEDGE_CLAIM}
	r.Register(a)

	got, ok := r.Get(types.ContributionClass_KNOWLEDGE_CLAIM)
	require.True(t, ok)
	require.Same(t, a, got)

	_, ok = r.Get(types.ContributionClass_TOOL)
	require.False(t, ok)
}

func TestAdapterRegistry_DuplicateRegistrationPanics(t *testing.T) {
	r := types.NewAdapterRegistry()
	r.Register(&fakeAdapter{class: types.ContributionClass_KNOWLEDGE_CLAIM})
	require.Panics(t, func() {
		r.Register(&fakeAdapter{class: types.ContributionClass_KNOWLEDGE_CLAIM})
	})
}

func TestGenesisState_DefaultIsValid(t *testing.T) {
	require.NoError(t, types.DefaultGenesis().Validate())
}

func TestGenesisState_RejectsBadIDLength(t *testing.T) {
	gs := &types.GenesisState{
		Contributions: []*types.Contribution{{Id: []byte{0x01}, Status: types.ContributionStatus_STATUS_SUBMITTED}},
	}
	err := gs.Validate()
	require.ErrorContains(t, err, "id must be 32 bytes")
}

func TestGenesisState_RejectsDuplicateID(t *testing.T) {
	id := make([]byte, 32)
	for i := range id {
		id[i] = 0xAB
	}
	gs := &types.GenesisState{
		Contributions: []*types.Contribution{
			{Id: id, Status: types.ContributionStatus_STATUS_SUBMITTED, Class: types.ContributionClass_KNOWLEDGE_CLAIM, Phase: types.LifecyclePhase_PHASE_FOUNDATION},
			{Id: id, Status: types.ContributionStatus_STATUS_SUBMITTED, Class: types.ContributionClass_KNOWLEDGE_CLAIM, Phase: types.LifecyclePhase_PHASE_FOUNDATION},
		},
	}
	err := gs.Validate()
	require.ErrorContains(t, err, "duplicate id")
}

// ── helpers ──

type fakeAdapter struct {
	class types.ContributionClass
}

func (f *fakeAdapter) Class() types.ContributionClass { return f.class }
func (f *fakeAdapter) Classify(_ context.Context, _ *types.Contribution) error { return nil }
func (f *fakeAdapter) SubstrateLink(_ context.Context, _ *types.Contribution) (uint32, error) {
	return 0, nil
}
func (f *fakeAdapter) Verify(_ context.Context, _ *types.Contribution) (uint32, error) {
	return 0, nil
}
