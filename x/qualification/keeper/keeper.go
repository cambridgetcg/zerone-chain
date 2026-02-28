package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/qualification/types"
)

// Keeper manages the qualification module's state.
type Keeper struct {
	storeService store.KVStoreService
	cdc          codec.BinaryCodec
	authority    string

	bankKeeper           types.BankKeeper
	stakingKeeper        types.StakingKeeper
	captureDefenseKeeper types.CaptureDefenseKeeper // nil-safe, set post-init
	ontologyKeeper       types.OntologyKeeper       // nil-safe, set post-init
}

// NewKeeper creates a new qualification module Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
	bk types.BankKeeper,
	sk types.StakingKeeper,
) Keeper {
	return Keeper{
		storeService:  storeService,
		cdc:           cdc,
		authority:     authority,
		bankKeeper:    bk,
		stakingKeeper: sk,
	}
}

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// SetCaptureDefenseKeeper sets the capture defense keeper post-initialization.
func (k *Keeper) SetCaptureDefenseKeeper(cdk types.CaptureDefenseKeeper) {
	k.captureDefenseKeeper = cdk
}

// SetOntologyKeeper sets the ontology keeper post-initialization.
func (k *Keeper) SetOntologyKeeper(ok types.OntologyKeeper) {
	k.ontologyKeeper = ok
}

// ---------- Params ----------

// SetParams sets module parameters.
func (k Keeper) SetParams(ctx context.Context, params *types.Params) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal params: %v", err))
	}
	_ = kvStore.Set(types.ParamsKey, bz)
}

// GetParams returns module parameters.
func (k Keeper) GetParams(ctx context.Context) *types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		return types.DefaultParams()
	}
	return &params
}

// ---------- Genesis ----------

// InitGenesis initializes the module's state from genesis.
func (k Keeper) InitGenesis(ctx context.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	}
	for _, q := range genState.Qualifications {
		k.SetQualification(ctx, q)
	}
	for _, e := range genState.Endorsements {
		k.SetEndorsement(ctx, e)
	}
	if genState.NextEndorsementId > 0 {
		k.setEndorsementCounter(ctx, genState.NextEndorsementId)
	}
}

// ExportGenesis exports the module's state.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	qualifications := k.GetAllQualifications(ctx)
	endorsements := k.GetAllEndorsements(ctx)
	nextID := k.getEndorsementCounter(ctx)
	return &types.GenesisState{
		Params:            params,
		Qualifications:    qualifications,
		Endorsements:      endorsements,
		NextEndorsementId: nextID,
	}
}

// ---------- Cross-module interface methods ----------

// IsQualified returns true if the validator holds an active qualification in the domain.
func (k Keeper) IsQualified(ctx context.Context, validator string, domain string) bool {
	q, found := k.GetQualification(ctx, validator, domain)
	if !found {
		return false
	}
	return q.Status == types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE
}

// GetQualificationWeight returns the weight of a validator's qualification in a domain.
func (k Keeper) GetQualificationWeight(ctx context.Context, validator string, domain string) uint32 {
	q, found := k.GetQualification(ctx, validator, domain)
	if !found {
		return 0
	}
	if q.Status != types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE {
		return 0
	}
	return q.Weight
}

// GetQualifiedValidators returns all validators with active qualifications in a domain.
func (k Keeper) GetQualifiedValidators(ctx context.Context, domain string) []string {
	validators := k.GetValidatorsByDomain(ctx, domain)
	var qualified []string
	for _, v := range validators {
		if k.IsQualified(ctx, v, domain) {
			qualified = append(qualified, v)
		}
	}
	return qualified
}

// RecordVerificationOutcome records a verification outcome for a validator in a domain.
func (k Keeper) RecordVerificationOutcome(ctx context.Context, validator string, domain string, correct bool) error {
	q, found := k.GetQualification(ctx, validator, domain)
	if !found {
		return fmt.Errorf("%w: %s/%s", types.ErrQualificationNotFound, validator, domain)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if q.Metrics == nil {
		q.Metrics = &types.QualificationMetrics{}
	}
	q.Metrics.TotalVerifications++
	if correct {
		q.Metrics.CorrectVerifications++
	}
	if q.Metrics.TotalVerifications > 0 {
		q.Metrics.AccuracyBps = (q.Metrics.CorrectVerifications * 1000000) / q.Metrics.TotalVerifications
	}
	q.Metrics.LastVerificationBlock = uint64(sdkCtx.BlockHeight())
	k.SetQualification(ctx, q)
	return nil
}

// prefixEndBytes returns the end key for prefix iteration (exclusive upper bound).
func prefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}
