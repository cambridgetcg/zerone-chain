package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/counterexamples/types"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	authority    string

	// Optional dependency: confirm the fact_id resolves. nil means
	// "skip the existence check" (unit tests, isolated runs).
	factKeeper types.FactExistenceKeeper
}

func NewKeeper(storeService store.KVStoreService, cdc codec.BinaryCodec, authority string) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    authority,
	}
}

// SetFactKeeper wires the existence-check adapter from x/knowledge.
func (k *Keeper) SetFactKeeper(fk types.FactExistenceKeeper) {
	k.factKeeper = fk
}

func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", "x/"+types.ModuleName)
}

func (k Keeper) Authority() string { return k.authority }

// ─── Params ──────────────────────────────────────────────────────────

func (k Keeper) GetParams(ctx context.Context) types.Params {
	bz, err := k.storeService.OpenKVStore(ctx).Get(types.ParamsKey)
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

// ─── Counterexamples ─────────────────────────────────────────────────

func ceKey(id string) []byte {
	return append(append([]byte{}, types.CounterexampleKeyPrefix...), []byte(id)...)
}

func byFactKey(factID, ceID string) []byte {
	out := append([]byte{}, types.ByFactPrefix...)
	out = append(out, []byte(factID)...)
	out = append(out, '/')
	return append(out, []byte(ceID)...)
}

func (k Keeper) NextCounterexampleID(ctx context.Context) (string, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.NextCounterexampleSeqKey)
	if err != nil {
		return "", err
	}
	cur := uint64(1)
	if bz != nil && len(bz) == 8 {
		cur = binary.BigEndian.Uint64(bz)
	}
	next := cur + 1
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, next)
	if err := store.Set(types.NextCounterexampleSeqKey, out); err != nil {
		return "", err
	}
	return fmt.Sprintf("ce-%d", cur), nil
}

func (k Keeper) GetCounterexample(ctx context.Context, id string) (*types.Counterexample, bool) {
	bz, err := k.storeService.OpenKVStore(ctx).Get(ceKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var c types.Counterexample
	if err := k.cdc.Unmarshal(bz, &c); err != nil {
		return nil, false
	}
	return &c, true
}

func (k Keeper) SetCounterexample(ctx context.Context, c *types.Counterexample) error {
	bz, err := k.cdc.Marshal(c)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(ceKey(c.Id), bz); err != nil {
		return err
	}
	return store.Set(byFactKey(c.FactId, c.Id), []byte{1})
}

func (k Keeper) IterateCounterexamplesByFact(ctx context.Context, factID string, cb func(c *types.Counterexample) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(append([]byte{}, types.ByFactPrefix...), []byte(factID)...)
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
		ceID := string(key[len(prefix):])
		c, ok := k.GetCounterexample(ctx, ceID)
		if !ok {
			continue
		}
		if cb(c) {
			break
		}
	}
	return nil
}

// GetTvwMultiplierBps exposes the configured multiplier so x/knowledge
// can apply it without having to read the counterexamples params via
// gRPC. Returns the param value or the default if unset.
func (k Keeper) GetTvwMultiplierBps(ctx context.Context) uint64 {
	return k.GetParams(ctx).TvwMultiplierBps
}

// HasValidatedCounterexample is the read used by x/knowledge to
// decide whether to apply the TVW multiplier on a fact.
//
// Returns true iff at least one counterexample for fact_id has
// status VALIDATED. Used by ComputeTrainingValueWeight via
// the FactCounterexampleAdapter.
func (k Keeper) HasValidatedCounterexample(ctx context.Context, factID string) bool {
	found := false
	_ = k.IterateCounterexamplesByFact(ctx, factID, func(c *types.Counterexample) bool {
		if c.Status == types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_VALIDATED {
			found = true
			return true
		}
		return false
	})
	return found
}

// ─── Validations ─────────────────────────────────────────────────────

func validationKey(id uint64) []byte {
	out := append([]byte{}, types.ValidationKeyPrefix...)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, id)
	return append(out, buf...)
}

func validationsByCEKey(ceID string, validationID uint64) []byte {
	out := append([]byte{}, types.ValidationsByCEPrefix...)
	out = append(out, []byte(ceID)...)
	out = append(out, '/')
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, validationID)
	return append(out, buf...)
}

func validatorVotedKey(ceID, validator string) []byte {
	out := append([]byte{}, types.ValidatorVotedPrefix...)
	out = append(out, []byte(ceID)...)
	out = append(out, '/')
	return append(out, []byte(validator)...)
}

func (k Keeper) HasValidatorVoted(ctx context.Context, ceID, validator string) bool {
	bz, err := k.storeService.OpenKVStore(ctx).Get(validatorVotedKey(ceID, validator))
	return err == nil && bz != nil
}

func (k Keeper) markValidatorVoted(ctx context.Context, ceID, validator string) error {
	return k.storeService.OpenKVStore(ctx).Set(validatorVotedKey(ceID, validator), []byte{1})
}

func (k Keeper) NextValidationID(ctx context.Context) (uint64, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.NextValidationIDKey)
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
	if err := store.Set(types.NextValidationIDKey, out); err != nil {
		return 0, err
	}
	return cur, nil
}

func (k Keeper) SetValidation(ctx context.Context, v *types.Validation) error {
	bz, err := k.cdc.Marshal(v)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(validationKey(v.Id), bz); err != nil {
		return err
	}
	return store.Set(validationsByCEKey(v.CounterexampleId, v.Id), []byte{1})
}

func (k Keeper) GetValidation(ctx context.Context, id uint64) (*types.Validation, bool) {
	bz, err := k.storeService.OpenKVStore(ctx).Get(validationKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var v types.Validation
	if err := k.cdc.Unmarshal(bz, &v); err != nil {
		return nil, false
	}
	return &v, true
}

func (k Keeper) IterateValidationsByCE(ctx context.Context, ceID string, cb func(v *types.Validation) bool) error {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(append([]byte{}, types.ValidationsByCEPrefix...), []byte(ceID)...)
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
		idBytes := key[len(prefix):]
		if len(idBytes) != 8 {
			continue
		}
		id := binary.BigEndian.Uint64(idBytes)
		v, ok := k.GetValidation(ctx, id)
		if !ok {
			continue
		}
		if cb(v) {
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
	for _, c := range gs.Counterexamples {
		if c != nil {
			_ = k.SetCounterexample(ctx, c)
		}
	}
	for _, v := range gs.Validations {
		if v != nil {
			_ = k.SetValidation(ctx, v)
			_ = k.markValidatorVoted(ctx, v.CounterexampleId, v.Validator)
		}
	}
	if gs.NextCounterexampleSeq > 0 {
		out := make([]byte, 8)
		binary.BigEndian.PutUint64(out, gs.NextCounterexampleSeq)
		_ = k.storeService.OpenKVStore(ctx).Set(types.NextCounterexampleSeqKey, out)
	}
	if gs.NextValidationId > 0 {
		out := make([]byte, 8)
		binary.BigEndian.PutUint64(out, gs.NextValidationId)
		_ = k.storeService.OpenKVStore(ctx).Set(types.NextValidationIDKey, out)
	}
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	gs := &types.GenesisState{
		Params: &params,
	}
	store := k.storeService.OpenKVStore(ctx)
	if bz, err := store.Get(types.NextCounterexampleSeqKey); err == nil && len(bz) == 8 {
		gs.NextCounterexampleSeq = binary.BigEndian.Uint64(bz)
	} else {
		gs.NextCounterexampleSeq = 1
	}
	if bz, err := store.Get(types.NextValidationIDKey); err == nil && len(bz) == 8 {
		gs.NextValidationId = binary.BigEndian.Uint64(bz)
	} else {
		gs.NextValidationId = 1
	}

	// Iterate ALL counterexamples directly via the primary key prefix.
	it, err := store.Iterator(types.CounterexampleKeyPrefix, nil)
	if err == nil {
		defer it.Close()
		for ; it.Valid(); it.Next() {
			key := it.Key()
			if len(key) < len(types.CounterexampleKeyPrefix) ||
				!bytesEqual(key[:len(types.CounterexampleKeyPrefix)], types.CounterexampleKeyPrefix) {
				break
			}
			var c types.Counterexample
			if err := k.cdc.Unmarshal(it.Value(), &c); err != nil {
				continue
			}
			gs.Counterexamples = append(gs.Counterexamples, &c)
		}
	}
	for _, c := range gs.Counterexamples {
		_ = k.IterateValidationsByCE(ctx, c.Id, func(v *types.Validation) bool {
			gs.Validations = append(gs.Validations, v)
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

// CurrentBlock returns the current block height.
func CurrentBlock(ctx context.Context) uint64 {
	h := sdk.UnwrapSDKContext(ctx).BlockHeight()
	if h < 0 {
		return 0
	}
	return uint64(h)
}
