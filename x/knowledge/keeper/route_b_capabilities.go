package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Route B Wave 7: capability report + one-shot bootstrap ─────────────
//
// Two primitives live here:
//   - BuildRouteBCapabilities produces the chain's self-description. The
//     first query a new trainer runs. Cheap: counters and pin reads.
//   - SeedRouteB is the one-shot bootstrap — calls every SeedDefault*
//     idempotently and returns the status report.
//
// Both are called from keeper.go (genesis init) and from the grpc query
// server (read side). The two surfaces share nothing except the pure
// assembly logic below.

// BuildRouteBCapabilities assembles the chain's current self-description.
func (k Keeper) BuildRouteBCapabilities(ctx context.Context) *types.RouteBCapabilities {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshot := uint64(sdkCtx.BlockHeight())

	caps := &types.RouteBCapabilities{
		SnapshotBlockHeight: snapshot,
		ChainId:             sdkCtx.ChainID(),
		AvailableCorpora:    AvailableCorpora(),
	}

	// Version pins.
	if spec, ok := k.GetTokenizerSpec(ctx); ok && spec != nil {
		caps.CurrentTokenizerVersion = spec.Version
	}
	if sch, ok := k.GetTraceSchema(ctx); ok && sch != nil {
		caps.CurrentTraceSchemaVersion = sch.Version
	}
	caps.CurrentMethodologySetVersion = k.currentMethodologySetVersion(ctx)

	// Counts.
	caps.MethodologyCount = k.countMethodologies(ctx)
	caps.FactCount = k.countFacts(ctx)
	caps.ActivePipelineCount = k.countActivePipelines(ctx)
	caps.ModelCardCount = k.countModelCards(ctx)
	caps.ActiveBountyCount = k.countActiveBounties(ctx)
	caps.FinalizedManifestCount = k.CountFinalizedManifests(ctx)
	caps.OpenContributionChallengeCount = k.countOpenContributionChallenges(ctx)

	// Financials.
	fundAddr := sdk.AccAddress(authtypes.NewModuleAddress(types.TrainingFundModuleName))
	bal := k.bankKeeper.GetBalance(ctx, fundAddr, "uzrn")
	caps.TrainingFundBalanceUzrn = bal.Amount.String()

	// Escrowed + vesting summaries.
	escrow := sdkmath.ZeroInt()
	k.IterateAugmentationBounties(ctx, func(b *types.AugmentationBounty) bool {
		if b.EscrowLocked != "" {
			if v, ok := sdkmath.NewIntFromString(b.EscrowLocked); ok {
				escrow = escrow.Add(v)
			}
		}
		return false
	})
	caps.TrainingFundEscrowedUzrn = escrow.String()

	vesting := sdkmath.ZeroInt()
	k.IterateTrainingFundDisbursements(ctx, func(d *types.TrainingFundDisbursement) bool {
		if d.VestingAmount != "" && d.ClawedBackAtBlock == 0 {
			if v, ok := sdkmath.NewIntFromString(d.VestingAmount); ok {
				vesting = vesting.Add(v)
			}
		}
		return false
	})
	caps.TrainingFundVestingUzrn = vesting.String()

	// Seed status.
	caps.SeedStatus = &types.SeedStatus{
		MethodologiesSeeded:  caps.MethodologyCount > 0,
		TokenizerSpecSeeded:  caps.CurrentTokenizerVersion > 0,
		TraceSchemaSeeded:    caps.CurrentTraceSchemaVersion > 0,
		CommitmentsSeeded:    len(k.GetAllNormativeCommitments(ctx)) > 0,
	}

	return caps
}

// ─── One-shot bootstrap ─────────────────────────────────────────────────

// SeedRouteBResult reports which seeds actually wrote state vs were already
// present. Idempotent: running twice yields all-false on the second call.
type SeedRouteBResult struct {
	MethodologiesWritten bool
	TokenizerSpecWritten bool
	TraceSchemaWritten   bool
	CommitmentsWritten   bool
}

// SeedRouteB runs every SeedDefault* entry point in order and returns a
// structured report. Callers (genesis, test harnesses, governance bootstrap
// messages) use this instead of remembering the individual seed calls.
func (k Keeper) SeedRouteB(ctx context.Context) (*SeedRouteBResult, error) {
	out := &SeedRouteBResult{}

	// Methodologies first — they're referenced by tokenizer/trace seeds.
	if k.countMethodologies(ctx) == 0 {
		if err := k.SeedDefaultMethodologies(ctx); err != nil {
			return out, err
		}
		out.MethodologiesWritten = true
	}

	// Tokenizer spec — pins the canonical serialisation contract.
	if _, ok := k.GetTokenizerSpec(ctx); !ok {
		if err := k.SeedDefaultTokenizerSpec(ctx); err != nil {
			return out, err
		}
		out.TokenizerSpecWritten = true
	}

	// Trace schema — pins the training-row contract.
	if _, ok := k.GetTraceSchema(ctx); !ok {
		if err := k.SeedDefaultTraceSchema(ctx); err != nil {
			return out, err
		}
		out.TraceSchemaWritten = true
	}

	// Normative commitments — seeds the is-ought-wall corpus.
	if len(k.GetAllNormativeCommitments(ctx)) == 0 {
		if err := k.SeedDefaultCommitments(ctx); err != nil {
			return out, err
		}
		out.CommitmentsWritten = true
	}

	return out, nil
}

// ─── Count helpers ──────────────────────────────────────────────────────

func (k Keeper) countMethodologies(ctx context.Context) uint64 {
	var n uint64
	k.IterateMethodologies(ctx, func(_ *types.Methodology) bool {
		n++
		return false
	})
	return n
}

func (k Keeper) countFacts(ctx context.Context) uint64 {
	var n uint64
	k.IterateFacts(ctx, func(_ *types.Fact) bool {
		n++
		return false
	})
	return n
}

func (k Keeper) countActivePipelines(ctx context.Context) uint64 {
	var n uint64
	k.IterateTrainingPipelines(ctx, func(p *types.TrainingPipeline) bool {
		if p != nil {
			n++
		}
		return false
	})
	return n
}

func (k Keeper) countModelCards(ctx context.Context) uint64 {
	var n uint64
	k.IterateModelCards(ctx, func(c *types.ModelCard) bool {
		if c != nil {
			n++
		}
		return false
	})
	return n
}

func (k Keeper) countActiveBounties(ctx context.Context) uint64 {
	var n uint64
	k.IterateAugmentationBounties(ctx, func(b *types.AugmentationBounty) bool {
		if b != nil && b.Active {
			n++
		}
		return false
	})
	return n
}

func (k Keeper) countOpenContributionChallenges(ctx context.Context) uint64 {
	var n uint64
	k.IterateOpenContributionChallenges(ctx, func(_ *types.ContributionChallenge) bool {
		n++
		return false
	})
	return n
}

// currentMethodologySetVersion returns the version number of the active
// methodology set. Today we don't version the set itself (methodologies
// are individually amendable); return the max Methodology.Version as a
// stand-in until the set becomes governance-versioned.
func (k Keeper) currentMethodologySetVersion(ctx context.Context) uint64 {
	var max uint64
	k.IterateMethodologies(ctx, func(m *types.Methodology) bool {
		if m != nil && m.Version > max {
			max = m.Version
		}
		return false
	})
	return max
}
