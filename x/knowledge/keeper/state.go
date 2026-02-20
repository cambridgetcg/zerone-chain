package keeper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// marshalOpts forces deterministic proto encoding for store writes.
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

// ─── Fact CRUD ───────────────────────────────────────────────────────────────

func (k Keeper) SetFact(ctx context.Context, fact *types.Fact) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(fact)
	if err != nil {
		return fmt.Errorf("failed to marshal fact: %w", err)
	}
	if err := store.Set(types.FactKey(fact.Id), bz); err != nil {
		return err
	}
	// Secondary indexes
	if fact.Submitter != "" {
		_ = store.Set(types.FactBySubmitterKey(fact.Submitter, fact.Id), []byte{0x01})
	}
	if fact.Domain != "" {
		_ = store.Set(types.FactByDomainKey(fact.Domain, fact.Id), []byte{0x01})
	}
	return nil
}

func (k Keeper) GetFact(ctx context.Context, id string) (*types.Fact, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FactKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var fact types.Fact
	if err := proto.Unmarshal(bz, &fact); err != nil {
		return nil, false
	}
	return &fact, true
}

func (k Keeper) DeleteFact(ctx context.Context, id string) error {
	fact, found := k.GetFact(ctx, id)
	if !found {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.FactKey(id))
	if fact.Submitter != "" {
		_ = store.Delete(types.FactBySubmitterKey(fact.Submitter, id))
	}
	if fact.Domain != "" {
		_ = store.Delete(types.FactByDomainKey(fact.Domain, id))
	}
	return nil
}

func (k Keeper) IterateFacts(ctx context.Context, cb func(fact *types.Fact) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.FactKeyPrefix, prefixEndBytes(types.FactKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var fact types.Fact
		if err := proto.Unmarshal(iter.Value(), &fact); err != nil {
			continue
		}
		if cb(&fact) {
			break
		}
	}
}

func (k Keeper) IterateFactsByDomain(ctx context.Context, domain string, cb func(factID string) bool) {
	store := k.storeService.OpenKVStore(ctx)
	pfx := append(append([]byte{}, types.DomainFactIndexPrefix...), []byte(domain+"/")...)
	iter, err := store.Iterator(pfx, prefixEndBytes(pfx))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		// Key is prefix+domain+"/"+factID — extract factID from after prefix
		key := iter.Key()
		factID := string(key[len(pfx):])
		if cb(factID) {
			break
		}
	}
}

func (k Keeper) IterateFactsBySubmitter(ctx context.Context, submitter string, cb func(factID string) bool) {
	store := k.storeService.OpenKVStore(ctx)
	pfx := append(append([]byte{}, types.FactBySubmitterIndexPrefix...), []byte(submitter+"/")...)
	iter, err := store.Iterator(pfx, prefixEndBytes(pfx))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		factID := string(key[len(pfx):])
		if cb(factID) {
			break
		}
	}
}

// ─── Claim CRUD ──────────────────────────────────────────────────────────────

func (k Keeper) SetClaim(ctx context.Context, claim *types.Claim) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(claim)
	if err != nil {
		return fmt.Errorf("failed to marshal claim: %w", err)
	}
	if err := store.Set(types.ClaimKey(claim.Id), bz); err != nil {
		return err
	}
	// Content hash dedup index
	if claim.ContentHash != "" {
		_ = store.Set(types.ContentHashKey(claim.ContentHash), []byte(claim.Id))
	}
	return nil
}

func (k Keeper) GetClaim(ctx context.Context, id string) (*types.Claim, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ClaimKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var claim types.Claim
	if err := proto.Unmarshal(bz, &claim); err != nil {
		return nil, false
	}
	return &claim, true
}

func (k Keeper) DeleteClaim(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.ClaimKey(id))
}

func (k Keeper) IterateClaims(ctx context.Context, cb func(claim *types.Claim) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ClaimKeyPrefix, prefixEndBytes(types.ClaimKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var claim types.Claim
		if err := proto.Unmarshal(iter.Value(), &claim); err != nil {
			continue
		}
		if cb(&claim) {
			break
		}
	}
}

// GetClaimByContentHash looks up a claim ID by its content hash (dedup).
func (k Keeper) GetClaimByContentHash(ctx context.Context, hash string) (string, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContentHashKey(hash))
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

// ─── VerificationRound CRUD ─────────────────────────────────────────────────

func (k Keeper) SetVerificationRound(ctx context.Context, round *types.VerificationRound) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(round)
	if err != nil {
		return fmt.Errorf("failed to marshal round: %w", err)
	}
	if err := store.Set(types.RoundKey(round.Id), bz); err != nil {
		return err
	}
	// Claim→round index
	if round.ClaimId != "" {
		_ = store.Set(types.ClaimRoundIndexKey(round.ClaimId), []byte(round.Id))
	}
	// Active round index (for BeginBlocker iteration)
	activeKey := activeRoundKey(round.Id)
	if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_COMPLETE &&
		round.Phase != types.VerificationPhase_VERIFICATION_PHASE_EXPIRED {
		_ = store.Set(activeKey, []byte{0x01})
	} else {
		_ = store.Delete(activeKey)
	}
	return nil
}

func (k Keeper) GetVerificationRound(ctx context.Context, id string) (*types.VerificationRound, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.RoundKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var round types.VerificationRound
	if err := proto.Unmarshal(bz, &round); err != nil {
		return nil, false
	}
	return &round, true
}

func (k Keeper) DeleteVerificationRound(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(activeRoundKey(id))
	return store.Delete(types.RoundKey(id))
}

func (k Keeper) GetRoundByClaimID(ctx context.Context, claimID string) (*types.VerificationRound, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ClaimRoundIndexKey(claimID))
	if err != nil || bz == nil {
		return nil, false
	}
	return k.GetVerificationRound(ctx, string(bz))
}

func (k Keeper) IterateActiveRounds(ctx context.Context, cb func(round *types.VerificationRound) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ActiveRoundIndexPrefix, prefixEndBytes(types.ActiveRoundIndexPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		roundID := string(iter.Key()[len(types.ActiveRoundIndexPrefix):])
		round, found := k.GetVerificationRound(ctx, roundID)
		if !found {
			continue
		}
		if cb(round) {
			break
		}
	}
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

// ─── ID Generation ───────────────────────────────────────────────────────────

// ComputeClaimContentHash returns the SHA-256 hex of domain-tagged claim content.
func ComputeClaimContentHash(content, domain string) string {
	h := sha256.New()
	h.Write([]byte("ZRN.claim.v1:"))
	h.Write([]byte(domain))
	h.Write([]byte(":"))
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateClaimID generates a deterministic claim ID from submitter, content hash, and block.
func GenerateClaimID(submitter, contentHash string, height uint64) string {
	h := sha256.New()
	h.Write([]byte("ZRN.claim.id.v1:"))
	h.Write([]byte(submitter))
	h.Write([]byte(":"))
	h.Write([]byte(contentHash))
	h.Write([]byte(fmt.Sprintf(":%d", height)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// GenerateFactID generates a deterministic fact ID from claim ID and block.
func GenerateFactID(claimID string, height uint64) string {
	h := sha256.New()
	h.Write([]byte("ZRN.fact.id.v1:"))
	h.Write([]byte(claimID))
	h.Write([]byte(fmt.Sprintf(":%d", height)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// GenerateRoundID generates a deterministic round ID from claim ID and block.
func GenerateRoundID(claimID string, height uint64) string {
	h := sha256.New()
	h.Write([]byte("ZRN.round.id.v1:"))
	h.Write([]byte(claimID))
	h.Write([]byte(fmt.Sprintf(":%d", height)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// ─── Commit/Reveal helpers ───────────────────────────────────────────────────

func findCommitByVerifier(commits []*types.CommitEntry, verifier string) *types.CommitEntry {
	for _, c := range commits {
		if c.Verifier == verifier {
			return c
		}
	}
	return nil
}

func findRevealByVerifier(reveals []*types.RevealEntry, verifier string) *types.RevealEntry {
	for _, r := range reveals {
		if r.Verifier == verifier {
			return r
		}
	}
	return nil
}

// ─── ABCI commit/reveal storage ──────────────────────────────────────────

// StoreCommitmentInRound stores a commitment entry in a verification round.
// Idempotent for duplicate commitments (same hash); returns ErrEquivocation
// for conflicting commitments from the same verifier.
func (k Keeper) StoreCommitmentInRound(ctx context.Context, roundID string, commit *types.CommitEntry) error {
	round, found := k.GetVerificationRound(ctx, roundID)
	if !found {
		return types.ErrRoundNotFound
	}
	if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_COMMIT {
		return types.ErrRoundNotInCommitPhase
	}

	existing := findCommitByVerifier(round.Commits, commit.Verifier)
	if existing != nil {
		if bytes.Equal(existing.CommitHash, commit.CommitHash) {
			return types.ErrDuplicateCommitment
		}
		return types.ErrEquivocation
	}

	round.Commits = append(round.Commits, commit)

	// Add to selected verifiers if new
	isNew := true
	for _, v := range round.SelectedVerifiers {
		if v == commit.Verifier {
			isNew = false
			break
		}
	}
	if isNew {
		round.SelectedVerifiers = append(round.SelectedVerifiers, commit.Verifier)
	}

	return k.SetVerificationRound(ctx, round)
}

// StoreRevealInRound stores a reveal entry in a verification round.
// Verifies the reveal matches the prior commitment hash using the provided confidence.
// The confidence parameter is needed because RevealEntry (proto) does not carry it,
// but it is part of the commitment hash preimage.
func (k Keeper) StoreRevealInRound(ctx context.Context, roundID string, reveal *types.RevealEntry, confidence uint64) error {
	round, found := k.GetVerificationRound(ctx, roundID)
	if !found {
		return types.ErrRoundNotFound
	}
	if round.Phase != types.VerificationPhase_VERIFICATION_PHASE_REVEAL {
		return types.ErrRoundNotInRevealPhase
	}

	// Find matching commit
	commit := findCommitByVerifier(round.Commits, reveal.Verifier)
	if commit == nil {
		return types.ErrNoCommitment
	}

	// Verify reveal matches commitment hash
	if !types.VerifyCommitmentHash(commit.CommitHash, roundID, reveal.Vote, confidence, reveal.Salt) {
		return types.ErrRevealMismatch
	}

	// Check for existing reveal
	existing := findRevealByVerifier(round.Reveals, reveal.Verifier)
	if existing != nil {
		if existing.Vote == reveal.Vote && bytes.Equal(existing.Salt, reveal.Salt) {
			return types.ErrDuplicateReveal
		}
		return types.ErrEquivocation
	}

	round.Reveals = append(round.Reveals, reveal)
	return k.SetVerificationRound(ctx, round)
}

// GetActiveRounds returns all active (non-complete/expired) verification rounds.
func (k Keeper) GetActiveRounds(ctx context.Context) []*types.VerificationRound {
	var rounds []*types.VerificationRound
	k.IterateActiveRounds(ctx, func(round *types.VerificationRound) bool {
		rounds = append(rounds, round)
		return false
	})
	return rounds
}

// ─── Store helpers ───────────────────────────────────────────────────────────

func activeRoundKey(roundID string) []byte {
	return append(append([]byte{}, types.ActiveRoundIndexPrefix...), []byte(roundID)...)
}

// prefixEndBytes returns the exclusive end key for prefix iteration.
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
	return nil // overflow: 0xFF...FF → nil means no upper bound
}
