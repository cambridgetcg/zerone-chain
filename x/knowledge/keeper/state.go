package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

var marshalOpts = proto.MarshalOptions{Deterministic: true}

// ─── Params ──────────────────────────────────────────────────────────────────

func (k Keeper) SetParams(ctx context.Context, params *types.Params) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}
	return store.Set(types.ParamsKey, bz)
}

func (k Keeper) GetParams(ctx context.Context) (*types.Params, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil {
		return nil, err
	}
	if bz == nil {
		p := types.DefaultParams()
		return &p, nil
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		p := types.DefaultParams()
		return &p, nil
	}
	return &params, nil
}

// ─── Domain CRUD ─────────────────────────────────────────────────────────────

func (k Keeper) SetDomain(ctx context.Context, domain *types.Domain) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(domain)
	if err != nil {
		return fmt.Errorf("failed to marshal domain: %w", err)
	}
	return store.Set(types.DomainKey(domain.Name), bz)
}

func (k Keeper) GetDomain(ctx context.Context, name string) (*types.Domain, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DomainKey(name))
	if err != nil || bz == nil {
		return nil, false
	}
	var domain types.Domain
	if err := proto.Unmarshal(bz, &domain); err != nil {
		return nil, false
	}
	return &domain, true
}

func (k Keeper) IterateDomains(ctx context.Context, cb func(domain *types.Domain) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DomainKeyPrefix, prefixEndBytes(types.DomainKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var domain types.Domain
		if err := proto.Unmarshal(iter.Value(), &domain); err != nil {
			continue
		}
		if cb(&domain) {
			break
		}
	}
}

// ─── Submission CRUD ────────────────────────────────────────────────────────

func (k Keeper) SetSubmission(ctx context.Context, sub *types.Submission) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal submission: %w", err)
	}
	return store.Set(types.SubmissionKey(sub.Id), bz)
}

func (k Keeper) GetSubmission(ctx context.Context, id string) (*types.Submission, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SubmissionKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var sub types.Submission
	if err := proto.Unmarshal(bz, &sub); err != nil {
		return nil, false
	}
	return &sub, true
}

func (k Keeper) DeleteSubmission(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.SubmissionKey(id))
}

func (k Keeper) IterateSubmissions(ctx context.Context, cb func(sub *types.Submission) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.SubmissionKeyPrefix, prefixEndBytes(types.SubmissionKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var sub types.Submission
		if err := proto.Unmarshal(iter.Value(), &sub); err != nil {
			continue
		}
		if cb(&sub) {
			break
		}
	}
}

// ─── Content hash index ─────────────────────────────────────────────────────

func (k Keeper) SetContentHash(ctx context.Context, hash, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.ContentHashKey(hash), []byte(submissionID))
}

func (k Keeper) HasContentHash(ctx context.Context, hash string) bool {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContentHashKey(hash))
	return err == nil && bz != nil
}

// ─── Submission indexes ─────────────────────────────────────────────────────

func (k Keeper) SetSubmissionDomainIndex(ctx context.Context, domain, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.SubmissionDomainIndexKey(domain, submissionID), []byte{0x01})
}

func (k Keeper) SetSubmissionSubmitterIndex(ctx context.Context, submitter, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.SubmissionSubmitterIndexKey(submitter, submissionID), []byte{0x01})
}

func (k Keeper) GetSubmissionsByDomain(ctx context.Context, domain string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.SubmissionDomainByDomainPrefix(domain)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var ids []string
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		id := string(key[len(prefix):])
		ids = append(ids, id)
	}
	return ids
}

func (k Keeper) GetSubmissionsBySubmitter(ctx context.Context, submitter string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.SubmissionSubmitterBySubmitterPrefix(submitter)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var ids []string
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		id := string(key[len(prefix):])
		ids = append(ids, id)
	}
	return ids
}

// ─── Sequences ──────────────────────────────────────────────────────────────

func (k Keeper) NextSubmissionID(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SubmissionSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	id := fmt.Sprintf("%x", seq)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = store.Set(types.SubmissionSeqKey, next)
	return id
}

// ─── Store helpers ───────────────────────────────────────────────────────────

func prefixEndBytes(pfx []byte) []byte {
	if len(pfx) == 0 {
		return nil
	}
	end := make([]byte, len(pfx))
	copy(end, pfx)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}
