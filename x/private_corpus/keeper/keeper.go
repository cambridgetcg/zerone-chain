package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/private_corpus/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	authority    string
}

func NewKeeper(storeService store.KVStoreService, cdc codec.BinaryCodec, authority string) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    authority,
	}
}

func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", "x/"+types.ModuleName)
}

func (k Keeper) Authority() string { return k.authority }

// ─── Params ──────────────────────────────────────────────────────────

func (k Keeper) GetParams(ctx context.Context) types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return *types.DefaultParams()
	}
	var p types.Params
	if err := k.cdc.Unmarshal(bz, &p); err != nil {
		return *types.DefaultParams()
	}
	return p
}

func (k Keeper) SetParams(ctx context.Context, p types.Params) error {
	bz, err := k.cdc.Marshal(&p)
	if err != nil {
		return err
	}
	return k.storeService.OpenKVStore(ctx).Set(types.ParamsKey, bz)
}

// ─── Vaults ──────────────────────────────────────────────────────────

func vaultStoreKey(id string) []byte {
	return append(append([]byte{}, types.VaultKeyPrefix...), []byte(id)...)
}

func vaultByOperatorKey(operator, id string) []byte {
	out := append([]byte{}, types.VaultByOperatorPrefix...)
	out = append(out, []byte(operator)...)
	out = append(out, '/')
	out = append(out, []byte(id)...)
	return out
}

func (k Keeper) GetVault(ctx context.Context, id string) (*types.Vault, bool) {
	bz, err := k.storeService.OpenKVStore(ctx).Get(vaultStoreKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var v types.Vault
	if err := k.cdc.Unmarshal(bz, &v); err != nil {
		return nil, false
	}
	return &v, true
}

func (k Keeper) SetVault(ctx context.Context, v *types.Vault) error {
	bz, err := k.cdc.Marshal(v)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(vaultStoreKey(v.Id), bz); err != nil {
		return err
	}
	if err := store.Set(vaultByOperatorKey(v.Operator, v.Id), []byte{1}); err != nil {
		return err
	}
	return nil
}

func (k Keeper) IterateVaults(ctx context.Context, cb func(v *types.Vault) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	it, err := store.Iterator(types.VaultKeyPrefix, nil)
	if err != nil {
		return err
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		key := it.Key()
		// only keys still under our prefix
		if len(key) < len(types.VaultKeyPrefix) ||
			!bytesEqual(key[:len(types.VaultKeyPrefix)], types.VaultKeyPrefix) {
			break
		}
		var v types.Vault
		if err := k.cdc.Unmarshal(it.Value(), &v); err != nil {
			continue
		}
		if cb(&v) {
			break
		}
	}
	return nil
}

func (k Keeper) IterateVaultsByOperator(ctx context.Context, operator string, cb func(v *types.Vault) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(append([]byte{}, types.VaultByOperatorPrefix...), []byte(operator)...)
	prefix = append(prefix, '/')
	it, err := store.Iterator(prefix, nil)
	if err != nil {
		return err
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix) || !bytesEqual(key[:len(prefix)], prefix) {
			break
		}
		id := string(key[len(prefix):])
		v, ok := k.GetVault(ctx, id)
		if !ok {
			continue
		}
		if cb(v) {
			break
		}
	}
	return nil
}

// ─── Manifests ───────────────────────────────────────────────────────

func manifestStoreKey(id string) []byte {
	return append(append([]byte{}, types.ManifestKeyPrefix...), []byte(id)...)
}

func manifestByVaultKey(vaultID, manifestID string) []byte {
	out := append([]byte{}, types.ManifestByVaultPrefix...)
	out = append(out, []byte(vaultID)...)
	out = append(out, '/')
	out = append(out, []byte(manifestID)...)
	return out
}

func (k Keeper) GetManifest(ctx context.Context, id string) (*types.CorpusManifest, bool) {
	bz, err := k.storeService.OpenKVStore(ctx).Get(manifestStoreKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var m types.CorpusManifest
	if err := k.cdc.Unmarshal(bz, &m); err != nil {
		return nil, false
	}
	return &m, true
}

func (k Keeper) SetManifest(ctx context.Context, m *types.CorpusManifest) error {
	bz, err := k.cdc.Marshal(m)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(manifestStoreKey(m.Id), bz); err != nil {
		return err
	}
	if err := store.Set(manifestByVaultKey(m.VaultId, m.Id), []byte{1}); err != nil {
		return err
	}
	return nil
}

func (k Keeper) IterateManifestsByVault(ctx context.Context, vaultID string, cb func(m *types.CorpusManifest) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(append([]byte{}, types.ManifestByVaultPrefix...), []byte(vaultID)...)
	prefix = append(prefix, '/')
	it, err := store.Iterator(prefix, nil)
	if err != nil {
		return err
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix) || !bytesEqual(key[:len(prefix)], prefix) {
			break
		}
		manifestID := string(key[len(prefix):])
		m, ok := k.GetManifest(ctx, manifestID)
		if !ok {
			continue
		}
		if cb(m) {
			break
		}
	}
	return nil
}

// ─── Access records ─────────────────────────────────────────────────

func accessRecordKey(seq uint64) []byte {
	out := append([]byte{}, types.AccessRecordKeyPrefix...)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, seq)
	return append(out, buf...)
}

func accessRecordByVaultKey(vaultID string, seq uint64) []byte {
	out := append([]byte{}, types.AccessRecordByVaultPrefix...)
	out = append(out, []byte(vaultID)...)
	out = append(out, '/')
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, seq)
	return append(out, buf...)
}

// NextAccessSeq reads-and-increments the access sequence. Returns the
// PRE-increment value (the seq the caller should use for the new
// record).
func (k Keeper) NextAccessSeq(ctx context.Context) (uint64, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.NextAccessSeqKey)
	if err != nil {
		return 0, err
	}
	cur := uint64(1)
	if bz != nil && len(bz) == 8 {
		cur = binary.BigEndian.Uint64(bz)
	}
	next := cur + 1
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, next)
	if err := store.Set(types.NextAccessSeqKey, out); err != nil {
		return 0, err
	}
	return cur, nil
}

func (k Keeper) PeekNextAccessSeq(ctx context.Context) uint64 {
	bz, err := k.storeService.OpenKVStore(ctx).Get(types.NextAccessSeqKey)
	if err != nil || bz == nil || len(bz) != 8 {
		return 1
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) SetNextAccessSeq(ctx context.Context, seq uint64) error {
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, seq)
	return k.storeService.OpenKVStore(ctx).Set(types.NextAccessSeqKey, out)
}

func (k Keeper) SetAccessRecord(ctx context.Context, r *types.AccessRecord) error {
	bz, err := k.cdc.Marshal(r)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(accessRecordKey(r.Seq), bz); err != nil {
		return err
	}
	if err := store.Set(accessRecordByVaultKey(r.VaultId, r.Seq), []byte{1}); err != nil {
		return err
	}
	return nil
}

func (k Keeper) GetAccessRecord(ctx context.Context, seq uint64) (*types.AccessRecord, bool) {
	bz, err := k.storeService.OpenKVStore(ctx).Get(accessRecordKey(seq))
	if err != nil || bz == nil {
		return nil, false
	}
	var r types.AccessRecord
	if err := k.cdc.Unmarshal(bz, &r); err != nil {
		return nil, false
	}
	return &r, true
}

func (k Keeper) IterateAccessRecordsByVault(ctx context.Context, vaultID string, startAfterSeq uint64, limit uint32, cb func(r *types.AccessRecord) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(append([]byte{}, types.AccessRecordByVaultPrefix...), []byte(vaultID)...)
	prefix = append(prefix, '/')

	var startKey []byte
	if startAfterSeq > 0 {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, startAfterSeq+1)
		startKey = append(append([]byte{}, prefix...), buf...)
	} else {
		startKey = prefix
	}

	it, err := store.Iterator(startKey, nil)
	if err != nil {
		return err
	}
	defer it.Close()

	count := uint32(0)
	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(prefix)+8 || !bytesEqual(key[:len(prefix)], prefix) {
			break
		}
		seq := binary.BigEndian.Uint64(key[len(prefix):])
		r, ok := k.GetAccessRecord(ctx, seq)
		if !ok {
			continue
		}
		if cb(r) {
			break
		}
		count++
		if limit > 0 && count >= limit {
			break
		}
	}
	return nil
}

// ─── Genesis ─────────────────────────────────────────────────────────

func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) {
	if gs == nil {
		return
	}
	if gs.Params != nil {
		_ = k.SetParams(ctx, *gs.Params)
	}
	for _, v := range gs.Vaults {
		if v != nil {
			_ = k.SetVault(ctx, v)
		}
	}
	for _, m := range gs.Manifests {
		if m != nil {
			_ = k.SetManifest(ctx, m)
		}
	}
	for _, r := range gs.AccessRecords {
		if r != nil {
			_ = k.SetAccessRecord(ctx, r)
		}
	}
	if gs.NextAccessSeq > 0 {
		_ = k.SetNextAccessSeq(ctx, gs.NextAccessSeq)
	}
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	gs := &types.GenesisState{
		Params:        &params,
		Vaults:        []*types.Vault{},
		Manifests:     []*types.CorpusManifest{},
		AccessRecords: []*types.AccessRecord{},
		NextAccessSeq: k.PeekNextAccessSeq(ctx),
	}
	_ = k.IterateVaults(ctx, func(v *types.Vault) bool {
		gs.Vaults = append(gs.Vaults, v)
		return false
	})
	for _, v := range gs.Vaults {
		_ = k.IterateManifestsByVault(ctx, v.Id, func(m *types.CorpusManifest) bool {
			gs.Manifests = append(gs.Manifests, m)
			return false
		})
		_ = k.IterateAccessRecordsByVault(ctx, v.Id, 0, 0, func(r *types.AccessRecord) bool {
			gs.AccessRecords = append(gs.AccessRecords, r)
			return false
		})
	}
	return gs
}

// ─── helpers ─────────────────────────────────────────────────────────

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// CurrentBlock returns the current block height as uint64.
func CurrentBlock(ctx context.Context) uint64 {
	h := sdk.UnwrapSDKContext(ctx).BlockHeight()
	if h < 0 {
		return 0
	}
	return uint64(h)
}

var _ = fmt.Sprintf
