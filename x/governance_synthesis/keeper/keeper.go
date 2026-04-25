package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/zerone-chain/zerone/x/governance_synthesis/types"
)

// Keeper for x/governance_synthesis. Pure consumer; same shape as
// x/training_provenance and x/trust_score.
type Keeper struct {
	cdc codec.BinaryCodec

	knowledgeKeeper        types.KnowledgeKeeper
	captureChallengeKeeper types.CaptureChallengeKeeper
	alignmentKeeper        types.AlignmentKeeper

	// Frontier-query upstreams. Optional; if any are nil, Frontier
	// returns an empty list rather than panicking — the Frontier
	// query is opt-in.
	ontologyKeeper           types.OntologyKeeper
	frontierKnowledgeKeeper  types.FrontierKnowledgeKeeper
	frontierInquiryKeeper    types.FrontierInquiryKeeper
	frontierCounterexamples  types.FrontierCounterexamplesKeeper
}

func NewKeeper(cdc codec.BinaryCodec) Keeper {
	return Keeper{cdc: cdc}
}

func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper)              { k.knowledgeKeeper = kk }
func (k *Keeper) SetCaptureChallengeKeeper(cck types.CaptureChallengeKeeper) {
	k.captureChallengeKeeper = cck
}
func (k *Keeper) SetAlignmentKeeper(ak types.AlignmentKeeper) { k.alignmentKeeper = ak }

// Frontier-query setters. All are optional — Frontier returns an
// empty list when any upstream is missing, so this synthesizer can
// stay correct in unit tests without the full app wiring.
func (k *Keeper) SetOntologyKeeper(ok types.OntologyKeeper)               { k.ontologyKeeper = ok }
func (k *Keeper) SetFrontierKnowledgeKeeper(fk types.FrontierKnowledgeKeeper) { k.frontierKnowledgeKeeper = fk }
func (k *Keeper) SetFrontierInquiryKeeper(fk types.FrontierInquiryKeeper)     { k.frontierInquiryKeeper = fk }
func (k *Keeper) SetFrontierCounterexamplesKeeper(fk types.FrontierCounterexamplesKeeper) {
	k.frontierCounterexamples = fk
}
