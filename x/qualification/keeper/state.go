package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/qualification/types"
)

// ---------- Qualification CRUD ----------

// SetQualification stores a domain qualification and updates the domain index.
func (k Keeper) SetQualification(ctx context.Context, q *types.DomainQualification) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := qualificationKey(q.Validator, q.Domain)
	bz, err := proto.Marshal(q)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal qualification: %v", err))
	}
	_ = kvStore.Set(key, bz)

	// Update domain→validator index.
	idxKey := domainValidatorIndexKey(q.Domain, q.Validator)
	_ = kvStore.Set(idxKey, []byte(q.Validator))
}

// GetQualification retrieves a qualification by validator and domain.
func (k Keeper) GetQualification(ctx context.Context, validator string, domain string) (*types.DomainQualification, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := qualificationKey(validator, domain)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var q types.DomainQualification
	if err := proto.Unmarshal(bz, &q); err != nil {
		return nil, false
	}
	return &q, true
}

// DeleteQualification removes a qualification and its domain index entry.
func (k Keeper) DeleteQualification(ctx context.Context, validator string, domain string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(qualificationKey(validator, domain))
	_ = kvStore.Delete(domainValidatorIndexKey(domain, validator))
}

// GetAllQualifications returns all qualifications.
func (k Keeper) GetAllQualifications(ctx context.Context) []*types.DomainQualification {
	var qualifications []*types.DomainQualification
	k.IterateQualifications(ctx, func(q *types.DomainQualification) bool {
		qualifications = append(qualifications, q)
		return false
	})
	return qualifications
}

// IterateQualifications iterates over all qualifications. Return true from cb to stop.
func (k Keeper) IterateQualifications(ctx context.Context, cb func(*types.DomainQualification) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.QualificationKeyPrefix, prefixEndBytes(types.QualificationKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var q types.DomainQualification
		if err := proto.Unmarshal(iter.Value(), &q); err != nil {
			continue
		}
		if cb(&q) {
			break
		}
	}
}

// GetValidatorsByDomain returns validator addresses for a given domain from the index.
func (k Keeper) GetValidatorsByDomain(ctx context.Context, domain string) []string {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := domainValidatorPrefix(domain)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var validators []string
	for ; iter.Valid(); iter.Next() {
		validators = append(validators, string(iter.Value()))
	}
	return validators
}

// GetQualifiedDomains returns all domains where the given account holds an active qualification (R31-4).
func (k Keeper) GetQualifiedDomains(ctx context.Context, account string) []string {
	var domains []string
	k.IterateQualifications(ctx, func(q *types.DomainQualification) bool {
		if q.Validator == account && q.Status == types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE {
			domains = append(domains, q.Domain)
		}
		return false
	})
	return domains
}

// ---------- Endorsement CRUD ----------

// SetEndorsement stores an endorsement and updates indexes.
func (k Keeper) SetEndorsement(ctx context.Context, e *types.QualificationEndorsement) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := endorsementKey(e.Id)
	bz, err := proto.Marshal(e)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal endorsement: %v", err))
	}
	_ = kvStore.Set(key, bz)

	// Endorser index: endorser → endorsement ID.
	endorserKey := endorserIndexKey(e.Endorser, e.Id)
	_ = kvStore.Set(endorserKey, uint64ToBytes(e.Id))

	// Target index: (validator+domain) → endorsement ID.
	targetKey := targetIndexKey(e.QualificationValidator, e.QualificationDomain, e.Id)
	_ = kvStore.Set(targetKey, uint64ToBytes(e.Id))
}

// GetEndorsement retrieves an endorsement by ID.
func (k Keeper) GetEndorsement(ctx context.Context, id uint64) (*types.QualificationEndorsement, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := endorsementKey(id)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var e types.QualificationEndorsement
	if err := proto.Unmarshal(bz, &e); err != nil {
		return nil, false
	}
	return &e, true
}

// DeleteEndorsement removes an endorsement and its index entries.
func (k Keeper) DeleteEndorsement(ctx context.Context, e *types.QualificationEndorsement) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(endorsementKey(e.Id))
	_ = kvStore.Delete(endorserIndexKey(e.Endorser, e.Id))
	_ = kvStore.Delete(targetIndexKey(e.QualificationValidator, e.QualificationDomain, e.Id))
}

// GetAllEndorsements returns all endorsements.
func (k Keeper) GetAllEndorsements(ctx context.Context) []*types.QualificationEndorsement {
	var endorsements []*types.QualificationEndorsement
	k.IterateEndorsements(ctx, func(e *types.QualificationEndorsement) bool {
		endorsements = append(endorsements, e)
		return false
	})
	return endorsements
}

// IterateEndorsements iterates over all endorsements. Return true from cb to stop.
func (k Keeper) IterateEndorsements(ctx context.Context, cb func(*types.QualificationEndorsement) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.EndorsementKeyPrefix, prefixEndBytes(types.EndorsementKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var e types.QualificationEndorsement
		if err := proto.Unmarshal(iter.Value(), &e); err != nil {
			continue
		}
		if cb(&e) {
			break
		}
	}
}

// GetEndorsementsByTarget returns endorsements for a specific validator+domain.
func (k Keeper) GetEndorsementsByTarget(ctx context.Context, validator string, domain string) []*types.QualificationEndorsement {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := targetPrefix(validator, domain)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var endorsements []*types.QualificationEndorsement
	for ; iter.Valid(); iter.Next() {
		id := bytesToUint64(iter.Value())
		e, found := k.GetEndorsement(ctx, id)
		if found {
			endorsements = append(endorsements, e)
		}
	}
	return endorsements
}

// ---------- Endorsement Counter ----------

// GetNextEndorsementID returns the next endorsement ID and increments the counter.
func (k Keeper) GetNextEndorsementID(ctx context.Context) uint64 {
	id := k.getEndorsementCounter(ctx)
	k.setEndorsementCounter(ctx, id+1)
	return id
}

func (k Keeper) getEndorsementCounter(ctx context.Context) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.EndorsementCounterKey)
	if err != nil || bz == nil {
		return 1
	}
	return bytesToUint64(bz)
}

func (k Keeper) setEndorsementCounter(ctx context.Context, id uint64) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.EndorsementCounterKey, uint64ToBytes(id))
}

// ---------- Key Construction Helpers ----------

func qualificationKey(validator string, domain string) []byte {
	return append(types.QualificationKeyPrefix, []byte(validator+"/"+domain)...)
}

func domainValidatorPrefix(domain string) []byte {
	return append(types.DomainValidatorPrefix, []byte(domain+"/")...)
}

func domainValidatorIndexKey(domain string, validator string) []byte {
	return append(types.DomainValidatorPrefix, []byte(domain+"/"+validator)...)
}

func endorsementKey(id uint64) []byte {
	return append(types.EndorsementKeyPrefix, uint64ToBytes(id)...)
}

func endorserIndexKey(endorser string, id uint64) []byte {
	key := append(types.EndorserIndexPrefix, []byte(endorser+"/")...)
	return append(key, uint64ToBytes(id)...)
}

func targetPrefix(validator string, domain string) []byte {
	return append(types.TargetIndexPrefix, []byte(validator+"/"+domain+"/")...)
}

func targetIndexKey(validator string, domain string, id uint64) []byte {
	key := append(types.TargetIndexPrefix, []byte(validator+"/"+domain+"/")...)
	return append(key, uint64ToBytes(id)...)
}

func uint64ToBytes(v uint64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, v)
	return bz
}

func bytesToUint64(bz []byte) uint64 {
	if len(bz) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}
