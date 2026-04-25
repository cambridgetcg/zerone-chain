package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/zerone-chain/zerone/x/trust_score/types"
)

// Keeper for x/trust_score. Pure consumer: no state, no msg server.
// Same shape as x/training_provenance — a permanent home for the
// cross-module wiring that makes a per-address trust signal queryable.
type Keeper struct {
	cdc codec.BinaryCodec

	knowledgeKeeper        types.KnowledgeKeeper
	qualificationKeeper    types.QualificationKeeper
	captureChallengeKeeper types.CaptureChallengeKeeper
}

func NewKeeper(cdc codec.BinaryCodec) Keeper {
	return Keeper{cdc: cdc}
}

func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper)              { k.knowledgeKeeper = kk }
func (k *Keeper) SetQualificationKeeper(qk types.QualificationKeeper)      { k.qualificationKeeper = qk }
func (k *Keeper) SetCaptureChallengeKeeper(cck types.CaptureChallengeKeeper) {
	k.captureChallengeKeeper = cck
}
