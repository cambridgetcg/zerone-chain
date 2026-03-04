package keeper

import (
	"context"
	"encoding/binary"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// Keeper holds module state for the knowledge module.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.BinaryCodec
	authority    string // governance authority address

	// External keeper dependencies (core — set at construction)
	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper

	// External keeper dependencies (post-init setters to break circular deps)
	ontologyKeeper            types.OntologyKeeper
	vestingRewardsKeeper      types.VestingRewardsKeeper
	domainQualificationKeeper types.DomainQualificationKeeper // nil until R6-5
	autopoiesisKeeper         types.AutopoiesisKeeper         // nil until R7-1
	partnershipKeeper         types.PartnershipKeeper         // nil until R26-4
	zeroneAuthKeeper           types.ZeroneAuthKeeper           // nil until R28-5
	captureDefenseKeeper       types.CaptureDefenseKeeper       // nil until R28-8
	pacingKeeper               types.PacingKeeper               // nil until R29-6
}

// NewKeeper creates a new knowledge Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
) Keeper {
	return Keeper{
		storeService:  storeService,
		cdc:           cdc,
		authority:     authority,
		bankKeeper:    bankKeeper,
		stakingKeeper: stakingKeeper,
	}
}

// GetAuthority returns the module's governance authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// GetStakingKeeper returns the staking keeper dependency.
func (k Keeper) GetStakingKeeper() types.StakingKeeper {
	return k.stakingKeeper
}

// SetOntologyKeeper sets the ontology keeper (post-init to break circular dep).
func (k *Keeper) SetOntologyKeeper(ok types.OntologyKeeper) {
	k.ontologyKeeper = ok
}

// SetVestingRewardsKeeper sets the vesting rewards keeper (post-init).
func (k *Keeper) SetVestingRewardsKeeper(vk types.VestingRewardsKeeper) {
	k.vestingRewardsKeeper = vk
}

// SetDomainQualificationKeeper sets the domain qualification keeper (post-init, R6-5).
func (k *Keeper) SetDomainQualificationKeeper(dk types.DomainQualificationKeeper) {
	k.domainQualificationKeeper = dk
}

// SetAutopoiesisKeeper sets the autopoiesis keeper (post-init, R7-1).
func (k *Keeper) SetAutopoiesisKeeper(ak types.AutopoiesisKeeper) {
	k.autopoiesisKeeper = ak
}

// SetPartnershipKeeper sets the partnership keeper (post-init, R26-4).
func (k *Keeper) SetPartnershipKeeper(pk types.PartnershipKeeper) {
	k.partnershipKeeper = pk
}

// SetZeroneAuthKeeper sets the zerone auth keeper (post-init, R28-5).
func (k *Keeper) SetZeroneAuthKeeper(ak types.ZeroneAuthKeeper) {
	k.zeroneAuthKeeper = ak
}

// SetCaptureDefenseKeeper sets the capture defense keeper post-initialization.
func (k *Keeper) SetCaptureDefenseKeeper(cdk types.CaptureDefenseKeeper) {
	k.captureDefenseKeeper = cdk
}

// SetPacingKeeper sets the pacing keeper for adaptive timing (R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) {
	k.pacingKeeper = pk
}

// TestingSetStoreService allows tests to inject a mock store service. Test-only.
func (k *Keeper) TestingSetStoreService(ss store.KVStoreService) {
	k.storeService = ss
}

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// IncreaseVerificationThreshold temporarily requires more verifiers for a domain.
// The override is stored as 20 bytes: additionalVerifiers (4) + expiryHeight (8) + createdAt (8).
func (k Keeper) IncreaseVerificationThreshold(ctx context.Context, domain string, additionalVerifiers uint32, expiryHeight uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	key := append(append([]byte{}, types.VerificationThresholdOverrideKeyPrefix...), []byte(domain)...)
	buf := make([]byte, 20)
	binary.BigEndian.PutUint32(buf[0:4], additionalVerifiers)
	binary.BigEndian.PutUint64(buf[4:12], expiryHeight)
	binary.BigEndian.PutUint64(buf[12:20], uint64(sdkCtx.BlockHeight()))
	return kvStore.Set(key, buf)
}

// RecordChallengeRoleImpact is a no-op stub (role elasticity removed in training data protocol).
// Satisfies capture_challenge/types.KnowledgeKeeper.
func (k Keeper) RecordChallengeRoleImpact(_ context.Context, _, _ string, _ bool) error {
	return nil
}

// ApplyMentorshipDividend is a no-op stub (mentorship dividends removed in training data protocol).
// Called by partnerships/keeper.KnowledgeDividendAdapter.
func (k Keeper) ApplyMentorshipDividend(_ context.Context, _, _, _ string) {
}
