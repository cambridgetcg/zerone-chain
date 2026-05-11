package keeper

import (
	"context"
	"encoding/binary"

	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/contribution/types"
)

// Keeper is the x/contribution module keeper.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService corestoretypes.KVStoreService

	// authority is the gov module address (used by Phase 6+ msg
	// handlers; Phase 1 stores it but doesn't enforce it).
	authority string

	// adapters is the per-class registry, populated at app init.
	adapters types.AdapterRegistry
}

// NewKeeper constructs the Keeper.
func NewKeeper(
	storeService corestoretypes.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    authority,
		adapters:     types.NewAdapterRegistry(),
	}
}

// Authority returns the gov authority address.
func (k Keeper) Authority() string { return k.authority }

// Logger returns a sub-logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", "x/"+types.ModuleName)
}

// RegisterAdapter exposes the registry for app-init wiring.
func (k *Keeper) RegisterAdapter(a types.ContributionAdapter) {
	k.adapters.Register(a)
}

// GetAdapter looks up the adapter for a class.
func (k Keeper) GetAdapter(class types.ContributionClass) (types.ContributionAdapter, bool) {
	return k.adapters.Get(class)
}

// ── store ops ──

func contributionKey(id []byte) []byte {
	return append(types.ContributionKey, id...)
}

// WriteContribution stores or updates a Contribution and refreshes secondary indexes.
func (k Keeper) WriteContribution(ctx context.Context, c *types.Contribution) error {
	store := k.storeService.OpenKVStore(ctx)

	// Read prior contribution (if any) so we can clean up stale secondary indexes
	// when status or other indexed fields change.
	priorBytes, err := store.Get(contributionKey(c.Id))
	if err != nil {
		return err
	}
	var prior *types.Contribution
	if priorBytes != nil {
		prior = &types.Contribution{}
		if err := k.cdc.Unmarshal(priorBytes, prior); err != nil {
			return err
		}
	}

	// Write primary record.
	bz, err := k.cdc.Marshal(c)
	if err != nil {
		return err
	}
	if err := store.Set(contributionKey(c.Id), bz); err != nil {
		return err
	}

	// Refresh secondary indexes.
	if prior != nil {
		if err := store.Delete(byContributorIdxKey(prior.Contributor, prior.Id)); err != nil {
			return err
		}
		if err := store.Delete(byClassIdxKey(prior.Class, prior.Id)); err != nil {
			return err
		}
		if err := store.Delete(byPhaseIdxKey(prior.Phase, prior.Id)); err != nil {
			return err
		}
		if err := store.Delete(byStatusIdxKey(prior.Status, prior.Id)); err != nil {
			return err
		}
		if prior.BackRef != "" {
			if err := store.Delete(byBackRefKey(prior.BackRef)); err != nil {
				return err
			}
		}
	}
	if err := store.Set(byContributorIdxKey(c.Contributor, c.Id), []byte{}); err != nil {
		return err
	}
	if err := store.Set(byClassIdxKey(c.Class, c.Id), []byte{}); err != nil {
		return err
	}
	if err := store.Set(byPhaseIdxKey(c.Phase, c.Id), []byte{}); err != nil {
		return err
	}
	if err := store.Set(byStatusIdxKey(c.Status, c.Id), []byte{}); err != nil {
		return err
	}
	if c.BackRef != "" {
		if err := store.Set(byBackRefKey(c.BackRef), c.Id); err != nil {
			return err
		}
	}
	return nil
}

// GetContribution reads a Contribution by id.
func (k Keeper) GetContribution(ctx context.Context, id []byte) (*types.Contribution, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(contributionKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	c := &types.Contribution{}
	if err := k.cdc.Unmarshal(bz, c); err != nil {
		return nil, false
	}
	return c, true
}

// GetContributionByBackRef looks up a Contribution via the back_ref index.
func (k Keeper) GetContributionByBackRef(ctx context.Context, backRef string) (*types.Contribution, bool) {
	if backRef == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	idBz, err := store.Get(byBackRefKey(backRef))
	if err != nil || idBz == nil {
		return nil, false
	}
	return k.GetContribution(ctx, idBz)
}

// ── index key builders ──

func uint32BE(v uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return buf
}

func uvarintBytes(v uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, v)
	return buf[:n]
}

func byContributorIdxKey(contributor string, id []byte) []byte {
	addrBz := []byte(contributor)
	out := append([]byte{}, types.ByContributorKey...)
	out = append(out, uvarintBytes(uint64(len(addrBz)))...)
	out = append(out, addrBz...)
	out = append(out, id...)
	return out
}

func byClassIdxKey(class types.ContributionClass, id []byte) []byte {
	out := append([]byte{}, types.ByClassKey...)
	out = append(out, uint32BE(uint32(class))...)
	out = append(out, id...)
	return out
}

func byPhaseIdxKey(phase types.LifecyclePhase, id []byte) []byte {
	out := append([]byte{}, types.ByPhaseKey...)
	out = append(out, uint32BE(uint32(phase))...)
	out = append(out, id...)
	return out
}

func byStatusIdxKey(status types.ContributionStatus, id []byte) []byte {
	out := append([]byte{}, types.ByStatusKey...)
	out = append(out, uint32BE(uint32(status))...)
	out = append(out, id...)
	return out
}

func byBackRefKey(backRef string) []byte {
	bz := []byte(backRef)
	out := append([]byte{}, types.ByBackRefKey...)
	out = append(out, uvarintBytes(uint64(len(bz)))...)
	out = append(out, bz...)
	return out
}
