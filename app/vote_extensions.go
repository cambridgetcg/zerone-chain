package app

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// VoteExtensionConfig holds per-validator configuration for vote extensions.
type VoteExtensionConfig struct {
	// ValidatorAddress is the validator's bech32 operator address.
	ValidatorAddress string

	// ValidatorPrivateKey is the Ed25519 private key for VRF generation.
	ValidatorPrivateKey []byte

	// LocalStore holds commitment salts between commit and reveal phases.
	LocalStore *LocalCommitmentStore

	// OracleClient is an optional client for querying the oracle sidecar.
	OracleClient OracleClient
}

// ExtendVoteHandler creates the ABCI++ ExtendVote handler.
// Stubbed: old PoT commit/reveal protocol removed in training data pivot (R36-5).
func (app *ZeroneApp) ExtendVoteHandler() sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (resp *abci.ResponseExtendVote, err error) {
		logger := ctx.Logger().With("module", "abci", "handler", "ExtendVote")

		defer func() {
			if r := recover(); r != nil {
				logger.Error("PANIC in ExtendVote — returning empty extension",
					"height", req.Height, "panic", fmt.Sprintf("%v", r))
				resp = &abci.ResponseExtendVote{}
				err = nil
			}
		}()

		// Skip if not configured
		if app.VoteExtConfig == nil ||
			app.VoteExtConfig.ValidatorAddress == "" ||
			len(app.VoteExtConfig.ValidatorPrivateKey) == 0 {
			return emptyVoteExtension()
		}

		ext := VoteExtension{
			ValidatorAddress: app.VoteExtConfig.ValidatorAddress,
		}

		bz, err := json.Marshal(ext)
		if err != nil {
			logger.Error("failed to marshal vote extension", "err", err)
			return emptyVoteExtension()
		}

		return &abci.ResponseExtendVote{VoteExtension: bz}, nil
	}
}

// VerifyVoteExtensionHandler creates the ABCI++ VerifyVoteExtension handler.
// Stubbed: old PoT commit/reveal protocol removed in training data pivot (R36-5).
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

		// Accept all well-formed extensions (no rounds to verify against)
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
