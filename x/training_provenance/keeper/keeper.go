package keeper

import (
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/zerone-chain/zerone/x/training_provenance/types"
)

// Keeper of the training_provenance module. Owns no state of its own —
// every reader is satisfied by the upstream producer keepers wired in
// at app init. The Keeper struct is intentionally minimal: it's a
// permanent home for the cross-module wiring.
type Keeper struct {
	cdc codec.BinaryCodec

	knowledgeKeeper        types.KnowledgeKeeper
	qualificationKeeper    types.QualificationKeeper
	captureChallengeKeeper types.CaptureChallengeKeeper
}

// NewKeeper constructs a Keeper. Upstream producer keepers are wired
// in via post-init Setters so the app build can avoid circular keeper
// dependencies.
func NewKeeper(cdc codec.BinaryCodec) Keeper {
	return Keeper{cdc: cdc}
}

// SetKnowledgeKeeper wires the knowledge keeper post-init. Required.
func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper) { k.knowledgeKeeper = kk }

// SetQualificationKeeper wires the qualification keeper post-init.
// Required for domain-coverage stats; optional in tests that don't
// exercise that path.
func (k *Keeper) SetQualificationKeeper(qk types.QualificationKeeper) { k.qualificationKeeper = qk }

// SetCaptureChallengeKeeper wires the capture-challenge keeper post-init.
// Optional: if nil, cartel_resolution_count on the cert is reported as 0.
func (k *Keeper) SetCaptureChallengeKeeper(cck types.CaptureChallengeKeeper) {
	k.captureChallengeKeeper = cck
}
