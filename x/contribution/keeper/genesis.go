package keeper

import (
	"context"

	corestoretypes "cosmossdk.io/core/store"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/contribution/types"
)

// InitGenesis writes all Contribution records from genesis state.
//
// At the end, emits one Substrate-class Contribution per registered
// adapter declaring the registry's own constitution. This is Layer 2
// of the UW recursion stack (runtime self-application) lifted to the
// genesis boundary: the chain treats its own initial wiring as useful
// work — adapters are not external scaffolding but Contributions the
// chain has admitted about its own dispatch table.
//
// Phase 1: synthetic claims_about_self carries the adapter class name.
// Phase 6: the adapter is bound to a real ModuleProposal Contribution
// whose lineage flows through governance.
func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) {
	for _, c := range gs.Contributions {
		if err := k.WriteContribution(ctx, c); err != nil {
			panic(err)
		}
	}

	// Emit one Substrate Contribution per registered adapter. The
	// registry was populated during NewApp before InitGenesis runs;
	// the actor is the gov authority (the entity that, by the time
	// Phase 6 lands, will be the only legitimate author of adapter
	// registrations).
	for class := range k.adapters {
		desc := []byte("adapter registered: " + class.String())
		// Best-effort: do not panic on a failed wrap during InitGenesis.
		// The pipeline is observational at this layer.
		_, _ = k.WrapAsSubstrateContribution(
			ctx,
			"code",
			k.authority,
			desc,
			nil,
		)
	}
}

// ExportGenesis dumps all Contributions by iterating the primary store.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	store := k.storeService.OpenKVStore(ctx)
	gs := types.DefaultGenesis()

	end := prefixEndBytes(types.ContributionKey)
	iter, err := openIterator(store, types.ContributionKey, end)
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var c types.Contribution
		// proto.Unmarshal — Contribution carries a oneof Payload; the
		// gogoproto table marshaler used by codec.BinaryCodec nil-derefs
		// on oneof closures for modern protoc-gen-go messages. Wire format
		// is identical, so this is safe alongside WriteContribution's
		// proto.Marshal.
		if err := proto.Unmarshal(iter.Value(), &c); err != nil {
			panic(err)
		}
		gs.Contributions = append(gs.Contributions, &c)
	}
	return gs
}

// openIterator wraps the store's Iterator method behind a typed alias.
// Kept private to the package so we can swap iteration implementations
// if needed.
type kvIterator interface {
	Valid() bool
	Next()
	Key() []byte
	Value() []byte
	Close() error
}

func openIterator(store corestoretypes.KVStore, start, end []byte) (kvIterator, error) {
	return store.Iterator(start, end)
}
