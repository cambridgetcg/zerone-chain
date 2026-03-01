package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/zerone-chain/zerone/x/capture_challenge/types"
)

// Keeper manages the capture_challenge module's state.
type Keeper struct {
	storeService         store.KVStoreService
	cdc                  codec.BinaryCodec
	authority            string
	bankKeeper           types.BankKeeper
	captureDefenseKeeper types.CaptureDefenseKeeper
	qualificationKeeper  types.DomainQualificationKeeper // nil-safe, set post-init
	knowledgeKeeper      types.KnowledgeKeeper           // nil-safe, set post-init
}

// NewKeeper creates a new capture_challenge module Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
	bankKeeper types.BankKeeper,
) Keeper {
	return Keeper{
		storeService: storeService,
		cdc:          cdc,
		authority:    authority,
		bankKeeper:   bankKeeper,
	}
}

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string { return k.authority }

// SetCaptureDefenseKeeper sets the capture defense keeper post-initialization.
func (k *Keeper) SetCaptureDefenseKeeper(cdk types.CaptureDefenseKeeper) {
	k.captureDefenseKeeper = cdk
}

// SetQualificationKeeper sets the domain qualification keeper post-initialization.
func (k *Keeper) SetQualificationKeeper(qk types.DomainQualificationKeeper) {
	k.qualificationKeeper = qk
}

// SetKnowledgeKeeper sets the knowledge keeper post-initialization.
func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper) {
	k.knowledgeKeeper = kk
}

// GenerateChallengeID creates a deterministic ID from challenger + domain + block.
func GenerateChallengeID(challenger, domain string, blockHeight int64) string {
	data := fmt.Sprintf("%s:%s:%d", challenger, domain, blockHeight)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 32-char hex
}

// ---------- Genesis ----------

// InitGenesis sets initial state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, gs *types.GenesisState) {
	if gs.Params != nil {
		k.SetParams(ctx, gs.Params)
	}
	for _, ch := range gs.Challenges {
		k.SetChallenge(ctx, ch)
	}
	for _, bp := range gs.BountyPools {
		k.SetBountyPool(ctx, bp)
	}
}

// ExportGenesis exports the current state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:      k.GetParams(ctx),
		Challenges:  k.GetAllChallenges(ctx),
		BountyPools: k.GetAllBountyPools(ctx),
	}
}

// ---------- BeginBlocker ----------

// BeginBlocker handles phase advancement, auto-fund, and risk analysis.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(sdkCtx)
	height := uint64(sdkCtx.BlockHeight())

	// Advance challenge phases
	k.AdvanceChallengePhases(sdkCtx, height)

	// Periodic risk analysis
	if height > 0 && params.RiskAnalysisInterval > 0 && height%params.RiskAnalysisInterval == 0 {
		k.RunRiskAnalysis(sdkCtx)
	}

	return nil
}

// AdvanceChallengePhases transitions challenges based on deadlines.
func (k Keeper) AdvanceChallengePhases(ctx sdk.Context, height uint64) {
	k.IterateChallenges(ctx, func(ch *types.CaptureChallenge) bool {
		switch ch.Status {
		case types.ChallengeStatus_CHALLENGE_STATUS_OPEN,
			types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE:
			// Transition to under_review at evidence deadline
			if height >= ch.EvidenceDeadline {
				ch.Status = types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW
				k.SetChallenge(ctx, ch)
			}
		case types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW:
			// Auto-expire at review deadline
			if height >= ch.ReviewDeadline {
				ch.Status = types.ChallengeStatus_CHALLENGE_STATUS_EXPIRED
				// Return stake on expiry
				challengerAddr, err := sdk.AccAddressFromBech32(ch.Challenger)
				if err == nil {
					stakeAmt, ok := new(big.Int).SetString(ch.Stake, 10)
					if ok && stakeAmt.Sign() > 0 {
						coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
						_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, coins)
					}
				}
				k.SetChallenge(ctx, ch)
			}
		}
		return false
	})
}

// RunRiskAnalysis queries capture defense keeper for flagged domains.
func (k Keeper) RunRiskAnalysis(ctx sdk.Context) {
	// No-op if defense keeper not wired
	if k.captureDefenseKeeper == nil {
		return
	}
	// Risk analysis is primarily done by capture_defense BeginBlocker.
	// This is a hook for challenge module to take action on flagged domains.
}
