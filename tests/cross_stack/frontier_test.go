package cross_stack_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	inquirytypes "github.com/zerone-chain/zerone/x/inquiry/types"
	inquirykeeper "github.com/zerone-chain/zerone/x/inquiry/keeper"
	ontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
)

// TestFrontier_OpenInquiriesRaiseSparsity verifies that an open
// inquiry in a domain raises that domain's sparsity score above a
// quiet domain. This binds the inquiry → frontier composition: open
// questions are demand for exploration, and the frontier signal
// reflects them.
func TestFrontier_OpenInquiriesRaiseSparsity(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// The harness's default genesis does not auto-seed ontology
	// domains in cross-stack tests; seed two by hand so the frontier
	// synthesizer has territory to walk.
	for _, name := range []string{"philosophy", "physics"} {
		h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
			Name:    name,
			Stratum: uint32(ontologytypes.StratumEmpirical),
			Status:  "active",
			Depth:   1,
		})
	}

	asker := testAddr("frontier_asker")
	require.NoError(t, h.FundAccount(asker, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(10_000_000)))))

	// Submit an inquiry in "philosophy" — a domain that exists in the
	// default ontology and starts sparse.
	inquiryMs := inquirykeeper.NewMsgServerImpl(h.InquiryKeeper)
	_, err = inquiryMs.SubmitInquiry(h.Ctx, &inquirytypes.MsgSubmitInquiry{
		Asker:    asker.String(),
		Question: "Test inquiry that should raise philosophy's sparsity",
		Domain:   "philosophy",
		Bounty:   "2000000",
	})
	require.NoError(t, err)

	// Compose the frontier and find philosophy's row.
	frontier := h.GovernanceSynthesisKeeper.ComposeFrontier(h.Ctx, 0)
	require.NotEmpty(t, frontier.Domains, "frontier must include at least the seeded domains")

	var philosophy, physics *struct {
		open     uint64
		sparsity uint64
	}
	for _, row := range frontier.Domains {
		switch row.Domain {
		case "philosophy":
			philosophy = &struct {
				open     uint64
				sparsity uint64
			}{open: row.OpenInquiries, sparsity: row.SparsityScoreBps}
		case "physics":
			physics = &struct {
				open     uint64
				sparsity uint64
			}{open: row.OpenInquiries, sparsity: row.SparsityScoreBps}
		}
	}
	require.NotNil(t, philosophy, "philosophy must appear in frontier")
	require.NotNil(t, physics, "physics must appear in frontier")

	require.Equal(t, uint64(1), philosophy.open,
		"philosophy must reflect the open inquiry")
	require.Equal(t, uint64(0), physics.open,
		"physics has no inquiry, must show 0 open")

	require.Greater(t, philosophy.sparsity, physics.sparsity,
		"a domain with an open inquiry must rank as sparser than an otherwise-equal quiet domain — the frontier signal must reflect demand")
}
