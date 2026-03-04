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

// ─── QualityRound CRUD ──────────────────────────────────────────────────────

func (k Keeper) SetQualityRound(ctx context.Context, round *types.QualityRound) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(round)
	if err != nil {
		return fmt.Errorf("failed to marshal quality round: %w", err)
	}
	return store.Set(types.QualityRoundKey(round.Id), bz)
}

func (k Keeper) GetQualityRound(ctx context.Context, id string) (*types.QualityRound, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.QualityRoundKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var round types.QualityRound
	if err := proto.Unmarshal(bz, &round); err != nil {
		return nil, false
	}
	return &round, true
}

func (k Keeper) DeleteQualityRound(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.QualityRoundKey(id))
}

// ─── Sample CRUD ────────────────────────────────────────────────────────────

func (k Keeper) SetSample(ctx context.Context, sample *types.Sample) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(sample)
	if err != nil {
		return fmt.Errorf("failed to marshal sample: %w", err)
	}
	return store.Set(types.SampleKey(sample.Id), bz)
}

func (k Keeper) GetSample(ctx context.Context, id string) (*types.Sample, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SampleKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var sample types.Sample
	if err := proto.Unmarshal(bz, &sample); err != nil {
		return nil, false
	}
	return &sample, true
}

func (k Keeper) DeleteSample(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.SampleKey(id))
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

func (k Keeper) NextRoundID(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.RoundSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	id := fmt.Sprintf("%x", seq)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = store.Set(types.RoundSeqKey, next)
	return id
}

func (k Keeper) NextSampleID(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SampleSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	id := fmt.Sprintf("%x", seq)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = store.Set(types.SampleSeqKey, next)
	return id
}

// ─── Active round index ─────────────────────────────────────────────────────

func (k Keeper) SetActiveRound(ctx context.Context, roundID string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := append(append([]byte{}, types.ActiveRoundIndexPrefix...), []byte(roundID)...)
	return store.Set(key, []byte{0x01})
}

func (k Keeper) DeleteActiveRound(ctx context.Context, roundID string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := append(append([]byte{}, types.ActiveRoundIndexPrefix...), []byte(roundID)...)
	return store.Delete(key)
}

func (k Keeper) GetActiveRounds(ctx context.Context) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.ActiveRoundIndexPrefix
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

// ─── Submission → Round index ───────────────────────────────────────────────

func (k Keeper) SetSubmissionRoundIndex(ctx context.Context, submissionID, roundID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.SubmissionRoundIndexKey(submissionID), []byte(roundID))
}

func (k Keeper) GetRoundBySubmission(ctx context.Context, submissionID string) (string, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SubmissionRoundIndexKey(submissionID))
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

// ─── Sample indexes ─────────────────────────────────────────────────────────

func (k Keeper) SetSampleDomainIndex(ctx context.Context, domain, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.DomainSampleIndexKey(domain, sampleID), []byte{0x01})
}

func (k Keeper) SetSampleSubmitterIndex(ctx context.Context, submitter, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.SubmitterIndexKey(submitter, sampleID), []byte{0x01})
}

func (k Keeper) SetSampleThreadIndex(ctx context.Context, threadID, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.ThreadIndexKey(threadID, sampleID), []byte{0x01})
}

func (k Keeper) GetSamplesByDomain(ctx context.Context, domain string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.DomainSampleByDomainPrefix(domain)
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

func (k Keeper) GetSamplesBySubmitter(ctx context.Context, submitter string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.SubmitterIndexBySubmitterPrefix(submitter)
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

func (k Keeper) GetSamplesByThread(ctx context.Context, threadID string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.ThreadIndexByThreadPrefix(threadID)
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

// ─── Sample iteration ───────────────────────────────────────────────────────

func (k Keeper) IterateSamples(ctx context.Context, cb func(sample *types.Sample) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.SampleKeyPrefix, prefixEndBytes(types.SampleKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var sample types.Sample
		if err := proto.Unmarshal(iter.Value(), &sample); err != nil {
			continue
		}
		if cb(&sample) {
			break
		}
	}
}

// ─── Niche index ────────────────────────────────────────────────────────────

func (k Keeper) SetNicheIndex(ctx context.Context, nicheKey, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.NicheIndexKey(nicheKey, sampleID), []byte{0x01})
}

func (k Keeper) DeleteNicheIndex(ctx context.Context, nicheKey, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.NicheIndexKey(nicheKey, sampleID))
}

func (k Keeper) GetSamplesByNiche(ctx context.Context, nicheKey string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.NicheIndexByNichePrefix(nicheKey)
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

// ─── At-risk sample index ───────────────────────────────────────────────────

func (k Keeper) SetAtRiskIndex(ctx context.Context, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.AtRiskSampleKey(sampleID), []byte{0x01})
}

func (k Keeper) DeleteAtRiskIndex(ctx context.Context, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.AtRiskSampleKey(sampleID))
}

func (k Keeper) IterateAtRiskSamples(ctx context.Context, cb func(sampleID string) bool) {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.AtRiskSampleIndexPrefix
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Key()[len(prefix):])
		if cb(id) {
			break
		}
	}
}


// ─── Contest index ──────────────────────────────────────────────────────────

func (k Keeper) SetContestIndex(ctx context.Context, sampleID, roundID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.ContestIndexKey(sampleID), []byte(roundID))
}

func (k Keeper) GetContestRound(ctx context.Context, sampleID string) (string, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContestIndexKey(sampleID))
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

func (k Keeper) DeleteContestIndex(ctx context.Context, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.ContestIndexKey(sampleID))
}
// ─── Topic saturation counters ──────────────────────────────────────────────

func (k Keeper) IncrementTopicCount(ctx context.Context, domain, topic string) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.TopicSaturationKey(domain, topic)
	current := k.GetTopicCount(ctx, domain, topic)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, current+1)
	return store.Set(key, next)
}

func (k Keeper) GetTopicCount(ctx context.Context, domain, topic string) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TopicSaturationKey(domain, topic))
	if err != nil || len(bz) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

// ─── TrainingDemand CRUD ────────────────────────────────────────────────────

func (k Keeper) SetTrainingDemand(ctx context.Context, demand *types.TrainingDemand) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(demand)
	if err != nil {
		return fmt.Errorf("failed to marshal training demand: %w", err)
	}
	return store.Set(types.TrainingDemandKeyFn(demand.Domain, demand.Subject), bz)
}

func (k Keeper) GetTrainingDemand(ctx context.Context, domain, subject string) (*types.TrainingDemand, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TrainingDemandKeyFn(domain, subject))
	if err != nil || bz == nil {
		return nil, false
	}
	var demand types.TrainingDemand
	if err := proto.Unmarshal(bz, &demand); err != nil {
		return nil, false
	}
	return &demand, true
}

func (k Keeper) IterateTrainingDemands(ctx context.Context, cb func(demand *types.TrainingDemand) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.TrainingDemandKey, prefixEndBytes(types.TrainingDemandKey))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var demand types.TrainingDemand
		if err := proto.Unmarshal(iter.Value(), &demand); err != nil {
			continue
		}
		if cb(&demand) {
			break
		}
	}
}

// ─── DataBounty CRUD ────────────────────────────────────────────────────────

func (k Keeper) SetDataBounty(ctx context.Context, bounty *types.DataBounty) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(bounty)
	if err != nil {
		return fmt.Errorf("failed to marshal data bounty: %w", err)
	}
	return store.Set(types.DataBountyKey(bounty.Id), bz)
}

func (k Keeper) GetDataBounty(ctx context.Context, id string) (*types.DataBounty, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DataBountyKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var bounty types.DataBounty
	if err := proto.Unmarshal(bz, &bounty); err != nil {
		return nil, false
	}
	return &bounty, true
}

func (k Keeper) DeleteDataBounty(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.DataBountyKey(id))
}

func (k Keeper) IterateDataBounties(ctx context.Context, cb func(bounty *types.DataBounty) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DataBountyKeyPrefix, prefixEndBytes(types.DataBountyKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var bounty types.DataBounty
		if err := proto.Unmarshal(iter.Value(), &bounty); err != nil {
			continue
		}
		if cb(&bounty) {
			break
		}
	}
}

func (k Keeper) SetBountyDomainIndex(ctx context.Context, domain, bountyID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.BountyDomainIndexKey(domain, bountyID), []byte{0x01})
}

func (k Keeper) DeleteBountyDomainIndex(ctx context.Context, domain, bountyID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.BountyDomainIndexKey(domain, bountyID))
}

func (k Keeper) GetActiveBounties(ctx context.Context, domain string) []*types.DataBounty {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.BountyDomainByDomainPrefix(domain)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var bounties []*types.DataBounty
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Key()[len(prefix):])
		bounty, found := k.GetDataBounty(ctx, id)
		if found && !bounty.Claimed {
			bounties = append(bounties, bounty)
		}
	}
	return bounties
}

func (k Keeper) NextBountyID(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.BountySeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	id := fmt.Sprintf("%x", seq)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = store.Set(types.BountySeqKey, next)
	return id
}

// ─── ScrapedSource CRUD ─────────────────────────────────────────────────────

func (k Keeper) SetScrapedSource(ctx context.Context, entry *types.ScrapedSourceEntry) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal scraped source: %w", err)
	}
	return store.Set(types.ScrapedSourceKeyFn(entry.Id), bz)
}

func (k Keeper) GetScrapedSource(ctx context.Context, id string) (*types.ScrapedSourceEntry, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ScrapedSourceKeyFn(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var entry types.ScrapedSourceEntry
	if err := proto.Unmarshal(bz, &entry); err != nil {
		return nil, false
	}
	return &entry, true
}

func (k Keeper) DeleteScrapedSource(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.ScrapedSourceKeyFn(id))
}

func (k Keeper) GetScrapedSourcePenalty(ctx context.Context, platform, domain string) uint64 {
	id := platform + "/" + domain
	entry, found := k.GetScrapedSource(ctx, id)
	if !found {
		return 0
	}
	return entry.NoveltyPenalty
}

func (k Keeper) IterateScrapedSources(ctx context.Context, cb func(entry *types.ScrapedSourceEntry) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ScrapedSourceKey, prefixEndBytes(types.ScrapedSourceKey))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var entry types.ScrapedSourceEntry
		if err := proto.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		if cb(&entry) {
			break
		}
	}
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
