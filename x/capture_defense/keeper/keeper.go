package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// Keeper manages the capture_defense module's state.
type Keeper struct {
	storeService       store.KVStoreService
	cdc                codec.BinaryCodec
	authority          string
	knowledgeKeeper    types.KnowledgeKeeper
	stakingKeeper      types.StakingKeeper
	ontologyKeeper     types.OntologyKeeper          // nil-safe, set post-init
	challengeKeeper    types.CaptureChallengeKeeper   // nil-safe, set post-init
	partnershipsKeeper types.PartnershipsKeeper       // nil-safe, set post-init (R29-5)
	pacingKeeper       types.PacingKeeper             // nil-safe, set post-init (R29-6)
}

// NewKeeper creates a new capture_defense module Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
) Keeper {
	return Keeper{
		storeService: storeService,
		cdc:          cdc,
		authority:    authority,
	}
}

// Logger returns a module-scoped logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.Logger().With("module", "x/"+types.ModuleName)
}

// GetAuthority returns the module authority address.
func (k Keeper) GetAuthority() string { return k.authority }

// SetKnowledgeKeeper sets the knowledge keeper post-initialization.
func (k *Keeper) SetKnowledgeKeeper(kk types.KnowledgeKeeper) { k.knowledgeKeeper = kk }

// SetStakingKeeper sets the staking keeper post-initialization.
func (k *Keeper) SetStakingKeeper(sk types.StakingKeeper) { k.stakingKeeper = sk }

// SetOntologyKeeper sets the ontology keeper post-initialization.
func (k *Keeper) SetOntologyKeeper(ok types.OntologyKeeper) { k.ontologyKeeper = ok }

// SetChallengeKeeper sets the capture challenge keeper post-initialization.
func (k *Keeper) SetChallengeKeeper(ck types.CaptureChallengeKeeper) { k.challengeKeeper = ck }

// SetPartnershipsKeeper sets the partnerships keeper post-initialization (R29-5).
func (k *Keeper) SetPartnershipsKeeper(pk types.PartnershipsKeeper) { k.partnershipsKeeper = pk }

// SetPacingKeeper sets the pacing keeper post-initialization (R29-6).
func (k *Keeper) SetPacingKeeper(pk types.PacingKeeper) { k.pacingKeeper = pk }

// InitGenesis sets initial state from genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, gs *types.GenesisState) {
	k.SetParams(ctx, gs.Params)
	for _, r := range gs.GlobalReputations {
		k.SetGlobalReputation(ctx, r)
	}
	for _, r := range gs.StratumReputations {
		k.SetStratumReputation(ctx, r)
	}
	for _, r := range gs.DomainReputations {
		k.SetDomainReputation(ctx, r)
	}
	for _, m := range gs.CaptureMetrics {
		k.SetCaptureMetrics(ctx, m)
	}
	for _, c := range gs.CrossStratumRequirements {
		k.SetCrossStratumRequirement(ctx, c)
	}
}

// ExportGenesis exports the current state.
func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:                   k.GetParams(ctx),
		GlobalReputations:        k.GetAllGlobalReputations(ctx),
		StratumReputations:       k.GetAllStratumReputations(ctx),
		DomainReputations:        k.GetAllDomainReputations(ctx),
		CaptureMetrics:           k.GetAllCaptureMetrics(ctx),
		CrossStratumRequirements: k.GetAllCrossStratumRequirements(ctx),
	}
}

// BeginBlocker runs reputation decay and auto-analysis.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetParams(sdkCtx)

	height := uint64(sdkCtx.BlockHeight())

	// Periodic decay
	if height > 0 && params.DecayEpochBlocks > 0 && height%params.DecayEpochBlocks == 0 {
		k.DecayAllReputations(sdkCtx, params)
	}

	// Auto risk analysis — adaptive interval (R29-6)
	effectiveInterval := params.RiskAnalysisInterval
	if k.pacingKeeper != nil {
		_, analysisPacing := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if analysisPacing > 0 && analysisPacing != 1_000_000 {
			effectiveInterval = params.RiskAnalysisInterval * 1_000_000 / analysisPacing
		}
	}
	if height > 0 && effectiveInterval > 0 && height%effectiveInterval == 0 {
		k.RunAutoAnalysis(sdkCtx, params)
	}

	return nil
}

// RunAutoAnalysis runs capture detection on all domains with recent history.
// When a domain is flagged, it auto-submits a challenge to capture_challenge.
func (k Keeper) RunAutoAnalysis(ctx sdk.Context, params *types.Params) {
	domains := k.GetDomainsWithHistory(ctx)
	for _, domain := range domains {
		metrics := k.AnalyzeCaptureRisk(ctx, domain, params)
		if metrics == nil {
			continue
		}
		if metrics.Flagged {
			// R29-5: Signal partnerships module to boost formation in flagged domains.
			k.OnDomainFlagged(ctx, domain)

			if k.challengeKeeper != nil {
				evidence := formatMetricsAsEvidence(metrics)
				if err := k.challengeKeeper.AutoSubmitChallenge(ctx, domain, metrics.RiskScore, metrics.HerfindahlIndex, evidence); err != nil {
					k.Logger(ctx).Error("auto-challenge submission failed", "domain", domain, "err", err)
				}
			}
		}
	}
}

// formatMetricsAsEvidence creates a human-readable evidence string from capture metrics.
func formatMetricsAsEvidence(m *types.CaptureMetrics) string {
	return fmt.Sprintf(
		"Auto-detected capture risk: HHI=%d, timing_correlation=%d, verdict_correlation=%d, top3_share=%d, risk_score=%d, analyzed_at_block=%d",
		m.HerfindahlIndex, m.TimingCorrelation, m.VerdictCorrelation, m.Top3Share, m.RiskScore, m.AnalyzedAtBlock,
	)
}

// RecordVerificationFromKnowledge records verification history from the knowledge module.
// This is the internal method called by the adapter — it writes directly to state
// without requiring a message transaction.
func (k Keeper) RecordVerificationFromKnowledge(ctx sdk.Context, domain, roundId string, validators []string, verdicts []bool, submitBlocks []uint64) {
	entry := &types.VerificationHistoryEntry{
		Domain:       domain,
		RoundId:      roundId,
		Validators:   validators,
		Verdicts:     verdicts,
		SubmitBlocks: submitBlocks,
		BlockHeight:  uint64(ctx.BlockHeight()),
	}
	k.SetVerificationHistory(ctx, entry)
}
