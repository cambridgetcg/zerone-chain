package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	vrfcrypto "github.com/zerone-chain/zerone/x/knowledge/crypto"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// VoteExtensionConfig holds per-validator configuration for vote extensions.
type VoteExtensionConfig struct {
	// ValidatorAddress is the validator's bech32 operator address.
	ValidatorAddress string

	// ValidatorPrivateKey is the Ed25519 private key for VRF generation.
	// This is the 32-byte seed or 64-byte full private key.
	ValidatorPrivateKey []byte

	// LocalStore holds commitment salts between commit and reveal phases.
	LocalStore *LocalCommitmentStore
}

// ExtendVoteHandler creates the ABCI++ ExtendVote handler that attaches
// PoT verification data to validator votes.
//
// For each active verification round:
//   - Commit phase: Generate VRF, check selection, create commitment hash
//   - Reveal phase: Retrieve local salt, create reveal
func (app *ZeroneApp) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (resp *abci.ResponseExtendVote, err error) {
		logger := ctx.Logger().With("module", "abci", "handler", "ExtendVote")

		// Panic recovery: a panic in ExtendVote must not crash the validator.
		defer func() {
			if r := recover(); r != nil {
				logger.Error("PANIC in ExtendVote — returning empty extension",
					"height", req.Height, "panic", fmt.Sprintf("%v", r))
				resp = &abci.ResponseExtendVote{}
				err = nil
			}
		}()

		// Skip if not configured (e.g., non-validator node)
		if app.VoteExtConfig == nil ||
			app.VoteExtConfig.ValidatorAddress == "" ||
			len(app.VoteExtConfig.ValidatorPrivateKey) == 0 {
			return emptyVoteExtension()
		}

		config := app.VoteExtConfig
		ext := VoteExtension{
			ValidatorAddress: config.ValidatorAddress,
		}

		currentHeight := uint64(ctx.BlockHeight())

		// Get active verification rounds
		activeRounds := app.KnowledgeKeeper.GetActiveRounds(ctx)

		for _, round := range activeRounds {
			switch round.Phase {
			case knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMMIT:
				commitment, err := app.handleCommitPhase(ctx, config, round)
				if err != nil {
					logger.Debug("skipping commit for round",
						"round_id", round.Id, "reason", err.Error())
					continue
				}
				if commitment != nil {
					ext.Commitments = append(ext.Commitments, *commitment)
				}

			case knowledgetypes.VerificationPhase_VERIFICATION_PHASE_REVEAL:
				reveal, err := handleRevealPhase(config, round)
				if err != nil {
					logger.Debug("skipping reveal for round",
						"round_id", round.Id, "reason", err.Error())
					continue
				}
				if reveal != nil {
					ext.Reveals = append(ext.Reveals, *reveal)
				}
			}
		}

		// Clean up expired local commitments
		config.LocalStore.CleanupExpired(currentHeight, 100)

		// Serialize
		bz, err := json.Marshal(ext)
		if err != nil {
			logger.Error("failed to marshal vote extension", "err", err)
			return emptyVoteExtension()
		}

		logger.Debug("extended vote",
			"height", currentHeight,
			"commitments", len(ext.Commitments),
			"reveals", len(ext.Reveals),
		)

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}

// handleCommitPhase generates a commitment for a round in commit phase.
// Returns nil if the validator is not selected via VRF for this round.
//
// P0-1: If a commitment already exists in the local store for this round,
// re-use it to prevent false equivocation slashing.
func (app *ZeroneApp) handleCommitPhase(
	ctx sdk.Context,
	config *VoteExtensionConfig,
	round *knowledgetypes.VerificationRound,
) (*VoteCommitment, error) {
	// Generate VRF to check if this validator is selected
	vrfSeed := vrfcrypto.GenerateVRFSeed(round.ClaimId, round.StartedAtBlock, nil)
	vrfOutput, vrfProof, err := vrfcrypto.GenerateVRF(vrfSeed, config.ValidatorPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("VRF generation failed: %w", err)
	}

	// Get validator's effective stake
	effectiveStake, err := app.KnowledgeKeeper.GetStakingKeeper().GetEffectiveStake(ctx, config.ValidatorAddress)
	if err != nil || effectiveStake == 0 {
		return nil, fmt.Errorf("validator %s not found or has zero stake", config.ValidatorAddress)
	}

	totalStake, err := app.KnowledgeKeeper.GetStakingKeeper().GetTotalStake(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total stake: %w", err)
	}

	params, err := app.KnowledgeKeeper.GetParams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get params: %w", err)
	}

	// Check if we're selected
	selected, _ := vrfcrypto.IsValidatorSelected(
		vrfOutput,
		effectiveStake,
		totalStake,
		uint32(params.MaxVerifiers),
	)

	if !selected {
		return nil, fmt.Errorf("not selected for round %s", round.Id)
	}

	// P0-1: Re-use existing commitment to prevent equivocation.
	if existing, found := config.LocalStore.Get(round.Id); found {
		commitmentHash := ComputeCommitmentHash(round.Id, existing.Verdict, existing.Confidence, existing.Salt)
		return &VoteCommitment{
			RoundID:        round.Id,
			CommitmentHash: commitmentHash,
			VRFOutput:      hex.EncodeToString(vrfOutput),
			VRFProof:       hex.EncodeToString(vrfProof),
			Height:         uint64(ctx.BlockHeight()),
		}, nil
	}

	// Stub evaluation: accept with 600K confidence.
	// Full deterministic evaluation engine (evaluation.EvaluateClaim) will be wired later.
	verdict := "accept"
	confidence := uint64(600_000)

	// Generate random salt
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	salt := hex.EncodeToString(saltBytes)

	// Compute commitment hash
	commitmentHash := ComputeCommitmentHash(round.Id, verdict, confidence, salt)

	// Store locally for reveal phase
	config.LocalStore.Store(LocalCommitment{
		RoundID:    round.Id,
		Verdict:    verdict,
		Confidence: confidence,
		Salt:       salt,
		Height:     uint64(ctx.BlockHeight()),
	})

	return &VoteCommitment{
		RoundID:        round.Id,
		CommitmentHash: commitmentHash,
		VRFOutput:      hex.EncodeToString(vrfOutput),
		VRFProof:       hex.EncodeToString(vrfProof),
		Height:         uint64(ctx.BlockHeight()),
	}, nil
}

// handleRevealPhase generates a reveal for a round in reveal phase.
// Returns nil if no local commitment exists (validator didn't commit).
func handleRevealPhase(
	config *VoteExtensionConfig,
	round *knowledgetypes.VerificationRound,
) (*VoteReveal, error) {
	localCommit, found := config.LocalStore.Get(round.Id)
	if !found {
		return nil, fmt.Errorf("no local commitment for round %s", round.Id)
	}

	return &VoteReveal{
		RoundID:    round.Id,
		Verdict:    localCommit.Verdict,
		Confidence: localCommit.Confidence,
		Salt:       localCommit.Salt,
	}, nil
}

// VerifyVoteExtensionHandler creates the ABCI++ VerifyVoteExtension handler
// that validates PoT vote extensions from other validators.
func (app *ZeroneApp) VerifyVoteExtensionHandler() sdk.VerifyVoteExtensionHandler {
	return func(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (resp *abci.ResponseVerifyVoteExtension, err error) {
		logger := ctx.Logger().With("module", "abci", "handler", "VerifyVoteExtension")

		defer func() {
			if r := recover(); r != nil {
				logger.Error("PANIC in VerifyVoteExtension — accepting extension to preserve liveness",
					"height", req.Height, "panic", fmt.Sprintf("%v", r))
				resp = acceptExtension()
				err = nil
			}
		}()

		// Empty extensions are always valid
		if len(req.VoteExtension) == 0 {
			return acceptExtension(), nil
		}

		var ext VoteExtension
		if err := json.Unmarshal(req.VoteExtension, &ext); err != nil {
			logger.Warn("invalid vote extension format", "err", err)
			return rejectExtension(), nil
		}

		// Reject oversized extensions (DoS prevention)
		const MaxCommitmentsPerExtension = 50
		const MaxRevealsPerExtension = 50
		if len(ext.Commitments) > MaxCommitmentsPerExtension {
			logger.Warn("too many commitments in vote extension",
				"count", len(ext.Commitments), "max", MaxCommitmentsPerExtension)
			return rejectExtension(), nil
		}
		if len(ext.Reveals) > MaxRevealsPerExtension {
			logger.Warn("too many reveals in vote extension",
				"count", len(ext.Reveals), "max", MaxRevealsPerExtension)
			return rejectExtension(), nil
		}

		// Validate commitments
		for _, c := range ext.Commitments {
			if c.RoundID == "" || c.CommitmentHash == "" {
				return rejectExtension(), nil
			}
			if len(c.CommitmentHash) != 64 {
				return rejectExtension(), nil
			}

			// VRF proof is REQUIRED for all commitments (GAP-1).
			if c.VRFOutput == "" || c.VRFProof == "" {
				logger.Warn("commitment missing VRF proof — rejecting extension",
					"round_id", c.RoundID,
					"validator", ext.ValidatorAddress,
				)
				return rejectExtension(), nil
			}

			vrfOutput, err := hex.DecodeString(c.VRFOutput)
			if err != nil {
				return rejectExtension(), nil
			}
			vrfProof, err := hex.DecodeString(c.VRFProof)
			if err != nil {
				return rejectExtension(), nil
			}

			selected, err := app.KnowledgeKeeper.VerifyValidatorVRFSelection(
				ctx, c.RoundID, ext.ValidatorAddress, vrfOutput, vrfProof,
			)
			if err != nil {
				logger.Warn("VRF verification failed",
					"round_id", c.RoundID,
					"validator", ext.ValidatorAddress,
					"err", err,
				)
				return rejectExtension(), nil
			}
			if !selected {
				logger.Warn("validator not selected by VRF",
					"round_id", c.RoundID,
					"validator", ext.ValidatorAddress,
				)
				return rejectExtension(), nil
			}
		}

		// Validate reveals
		for _, r := range ext.Reveals {
			if r.RoundID == "" || r.Salt == "" {
				return rejectExtension(), nil
			}

			switch r.Verdict {
			case "accept", "reject", "abstain":
			default:
				return rejectExtension(), nil
			}

			if r.Confidence > 1_000_000 {
				return rejectExtension(), nil
			}

			// Verify reveal matches prior commitment (if round+commit exist in state)
			round, found := app.KnowledgeKeeper.GetVerificationRound(ctx, r.RoundID)
			if found {
				commit := findCommitByVerifier(round.Commits, ext.ValidatorAddress)
				if commit != nil {
					if !VerifyCommitmentHash(
						hex.EncodeToString(commit.CommitHash),
						r.RoundID, r.Verdict, r.Confidence, r.Salt,
					) {
						logger.Warn("reveal does not match commitment",
							"round_id", r.RoundID,
							"validator", ext.ValidatorAddress,
						)
						return rejectExtension(), nil
					}
				}
			}
		}

		logger.Debug("verified vote extension",
			"height", req.Height,
			"commitments", len(ext.Commitments),
			"reveals", len(ext.Reveals),
		)

		return acceptExtension(), nil
	}
}

// ---- Helpers ----

func emptyVoteExtension() (*abci.ResponseExtendVote, error) {
	return &abci.ResponseExtendVote{}, nil
}

func acceptExtension() *abci.ResponseVerifyVoteExtension {
	return &abci.ResponseVerifyVoteExtension{
		Status: abci.ResponseVerifyVoteExtension_ACCEPT,
	}
}

func rejectExtension() *abci.ResponseVerifyVoteExtension {
	return &abci.ResponseVerifyVoteExtension{
		Status: abci.ResponseVerifyVoteExtension_REJECT,
	}
}

// findCommitByVerifier scans a commit slice for a matching verifier address.
// Duplicated from keeper/state.go to avoid circular import (app → keeper).
func findCommitByVerifier(commits []*knowledgetypes.CommitEntry, verifier string) *knowledgetypes.CommitEntry {
	for _, c := range commits {
		if c.Verifier == verifier {
			return c
		}
	}
	return nil
}
