// ABCI++ Extension Points for Proof of Truth Consensus
//
// This file implements the ABCI++ methods that integrate PoT's multi-block
// verification rounds into CometBFT's single-block cycle.
//
// Vote Extension Data Flow:
//  1. ExtendVote: Validators attach commitments/reveals to CometBFT votes
//  2. PrepareProposal: Proposer collects extensions, creates injection tx
//  3. ProcessProposal: Validators verify injection tx format
//  4. PreBlocker: All validators process injection tx, store in keeper state
//  5. BeginBlocker: Phase transitions and aggregation via knowledge module
package app

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// VoteExtension is the PoT-specific data validators attach to their votes.
// Serialized as JSON for Phase 2; future phases may use protobuf.
type VoteExtension struct {
	Commitments      []VoteCommitment `json:"commitments,omitempty"`
	Reveals          []VoteReveal     `json:"reveals,omitempty"`
	ValidatorAddress string           `json:"validator_address"`
}

// VoteCommitment represents a validator's commitment to a verification verdict.
type VoteCommitment struct {
	RoundID        string `json:"round_id"`
	CommitmentHash string `json:"commitment_hash"`      // hex-encoded SHA-256
	VRFOutput      string `json:"vrf_output,omitempty"` // hex-encoded VRF output
	VRFProof       string `json:"vrf_proof,omitempty"`  // hex-encoded VRF proof
	Height         uint64 `json:"height"`
}

// VoteReveal represents a validator's revealed verification verdict.
type VoteReveal struct {
	RoundID    string `json:"round_id"`
	Verdict    string `json:"verdict"`    // "accept", "reject", "abstain"
	Confidence uint64 `json:"confidence"` // 0-1000000
	Salt       string `json:"salt"`       // hex-encoded salt
}

// ---- Vote Extension Injection ----

// VoteExtInjectionPrefix is the 4-byte magic header for injected vote extension txs.
// These are pseudo-txs included at position 0 of a block by the proposer.
var VoteExtInjectionPrefix = []byte{0x00, 'V', 'E', 'X'}

// VoteExtInjection contains all commitments and reveals from the previous block's
// vote extensions. The proposer encodes this in PrepareProposal; all validators
// process it in PreBlocker to store data in the knowledge keeper.
type VoteExtInjection struct {
	Commitments []InjectedCommitment `json:"commitments"`
	Reveals     []InjectedReveal     `json:"reveals"`
}

// InjectedCommitment is a commitment extracted from a vote extension.
type InjectedCommitment struct {
	RoundID        string `json:"round_id"`
	Validator      string `json:"validator"`
	CommitmentHash string `json:"commitment_hash"` // hex-encoded
	VRFOutput      string `json:"vrf_output,omitempty"`
	VRFProof       string `json:"vrf_proof,omitempty"`
}

// InjectedReveal is a reveal extracted from a vote extension.
type InjectedReveal struct {
	RoundID    string `json:"round_id"`
	Validator  string `json:"validator"`
	Verdict    string `json:"verdict"`
	Confidence uint64 `json:"confidence"`
	Salt       string `json:"salt"` // hex-encoded
}

// MaxVEXInjectionBytes is the maximum size of a vote extension injection pseudo-tx.
const MaxVEXInjectionBytes = 2 * 1024 * 1024

// BlockGasLimit is defined in gas.go alongside other gas constants.

// IsVoteExtInjectionTx checks if a tx has the vote extension injection prefix.
func IsVoteExtInjectionTx(tx []byte) bool {
	return len(tx) > len(VoteExtInjectionPrefix) &&
		bytes.Equal(tx[:len(VoteExtInjectionPrefix)], VoteExtInjectionPrefix)
}

// EncodeVoteExtInjection serializes a VoteExtInjection with the magic prefix.
func EncodeVoteExtInjection(inj VoteExtInjection) ([]byte, error) {
	data, err := json.Marshal(inj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal vote extension injection: %w", err)
	}
	return append(VoteExtInjectionPrefix, data...), nil
}

// DecodeVoteExtInjection deserializes a VoteExtInjection from prefixed bytes.
func DecodeVoteExtInjection(tx []byte) (VoteExtInjection, error) {
	var inj VoteExtInjection
	if !IsVoteExtInjectionTx(tx) {
		return inj, fmt.Errorf("not a vote extension injection tx")
	}
	err := json.Unmarshal(tx[len(VoteExtInjectionPrefix):], &inj)
	return inj, err
}

// ComputeCommitmentHash generates a commitment hash for a verification verdict.
// Delegates to knowledgetypes.ComputeCommitmentHash (canonical implementation).
// Returns hex-encoded hash for use in JSON transport types.
func ComputeCommitmentHash(roundID, verdict string, confidence uint64, salt string) string {
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(knowledgetypes.ComputeCommitmentHash(roundID, verdict, confidence, saltBytes))
}

// VerifyCommitmentHash checks that a reveal matches its prior commitment.
// Delegates to knowledgetypes.VerifyCommitmentHash (canonical implementation).
func VerifyCommitmentHash(commitmentHash, roundID, verdict string, confidence uint64, salt string) bool {
	hashBytes, err := hex.DecodeString(commitmentHash)
	if err != nil {
		return false
	}
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return false
	}
	return knowledgetypes.VerifyCommitmentHash(hashBytes, roundID, verdict, confidence, saltBytes)
}

// ---- ABCI++ Handlers ----

// PrepareProposalHandler returns a PrepareProposal handler that injects
// vote extension data as a pseudo-tx at position 0.
func (app *ZeroneApp) PrepareProposalHandler() sdk.PrepareProposalHandler {
	return app.prepareProposal
}

func (app *ZeroneApp) prepareProposal(ctx sdk.Context, req *abci.RequestPrepareProposal) (resp *abci.ResponsePrepareProposal, err error) {
	logger := ctx.Logger().With("module", "abci", "handler", "PrepareProposal")

	defer func() {
		if r := recover(); r != nil {
			logger.Error("PANIC in PrepareProposal — returning empty proposal",
				"height", req.Height, "panic", fmt.Sprintf("%v", r))
			resp = &abci.ResponsePrepareProposal{Txs: nil}
			err = nil
		}
	}()

	txs := make([][]byte, 0, len(req.Txs)+1)

	// Process vote extensions from previous block (if available)
	if len(req.LocalLastCommit.Votes) > 0 {
		injection := app.processVoteExtensions(ctx, req.LocalLastCommit.Votes)

		if len(injection.Commitments) > 0 || len(injection.Reveals) > 0 {
			injBytes, err := EncodeVoteExtInjection(injection)
			if err != nil {
				return nil, fmt.Errorf("critical: vote extension injection encoding failed: %w", err)
			}

			if len(injBytes) > MaxVEXInjectionBytes {
				logger.Warn("vote extension injection exceeds size limit — dropping",
					"size", len(injBytes), "max", MaxVEXInjectionBytes)
			} else {
				txs = append(txs, injBytes)
			}

			logger.Info("injecting vote extension data",
				"commitments", len(injection.Commitments),
				"reveals", len(injection.Reveals),
			)
		}
	}

	// Add regular transactions from mempool, respecting gas limit
	var totalGas uint64
	for _, tx := range req.Txs {
		estimatedGas := uint64(len(tx)) * 10
		if totalGas+estimatedGas > BlockGasLimit {
			break
		}
		txs = append(txs, tx)
		totalGas += estimatedGas
	}

	// Set block tx count for reward calculation in BeginBlock.
	// Count only user txs — the injection pseudo-tx is consensus overhead, not user activity.
	// PoT alignment: empty blocks (no user txs) should not earn rewards.
	userTxCount := len(txs)
	if len(txs) > 0 && IsVoteExtInjectionTx(txs[0]) {
		userTxCount = len(txs) - 1
	}
	app.VestingRewardsKeeper.SetBlockTxCount(userTxCount)

	logger.Debug("prepared proposal",
		"height", req.Height,
		"txs", len(txs),
		"user_txs", userTxCount,
		"vote_extensions", len(req.LocalLastCommit.Votes),
	)

	return &abci.ResponsePrepareProposal{Txs: txs}, nil
}

// ProcessProposalHandler returns a ProcessProposal handler that validates
// vote extension injection txs and regular transactions.
func (app *ZeroneApp) ProcessProposalHandler() sdk.ProcessProposalHandler {
	return app.processProposal
}

func (app *ZeroneApp) processProposal(ctx sdk.Context, req *abci.RequestProcessProposal) (resp *abci.ResponseProcessProposal, err error) {
	logger := ctx.Logger().With("module", "abci", "handler", "ProcessProposal")

	defer func() {
		if r := recover(); r != nil {
			logger.Error("PANIC in ProcessProposal — rejecting proposal",
				"height", req.Height, "panic", fmt.Sprintf("%v", r))
			resp = &abci.ResponseProcessProposal{
				Status: abci.ResponseProcessProposal_REJECT,
			}
			err = nil
		}
	}()

	for i, txBytes := range req.Txs {
		if IsVoteExtInjectionTx(txBytes) {
			if len(txBytes) > MaxVEXInjectionBytes {
				logger.Warn("vote extension injection tx exceeds size limit",
					"index", i, "size", len(txBytes), "max", MaxVEXInjectionBytes)
				return &abci.ResponseProcessProposal{
					Status: abci.ResponseProcessProposal_REJECT,
				}, nil
			}
			if _, err := DecodeVoteExtInjection(txBytes); err != nil {
				logger.Warn("invalid vote extension injection tx", "index", i, "err", err)
				return &abci.ResponseProcessProposal{
					Status: abci.ResponseProcessProposal_REJECT,
				}, nil
			}
			continue
		}

		tx, err := app.txConfig.TxDecoder()(txBytes)
		if err != nil {
			logger.Warn("invalid tx in proposal", "index", i, "err", err)
			return &abci.ResponseProcessProposal{
				Status: abci.ResponseProcessProposal_REJECT,
			}, nil
		}

		for _, msg := range tx.GetMsgs() {
			if m, ok := msg.(sdk.HasValidateBasic); ok {
				if err := m.ValidateBasic(); err != nil {
					logger.Warn("invalid msg in proposal",
						"index", i,
						"msg_type", sdk.MsgTypeURL(msg),
						"err", err,
					)
					return &abci.ResponseProcessProposal{
						Status: abci.ResponseProcessProposal_REJECT,
					}, nil
				}
			}
		}
	}

	// Set block tx count for reward calculation in BeginBlock.
	// Count only user txs (exclude injection pseudo-txs).
	userTxCount := 0
	for _, txBytes := range req.Txs {
		if !IsVoteExtInjectionTx(txBytes) {
			userTxCount++
		}
	}
	app.VestingRewardsKeeper.SetBlockTxCount(userTxCount)

	logger.Debug("accepted proposal",
		"height", req.Height,
		"txs", len(req.Txs),
		"user_txs", userTxCount,
	)

	return &abci.ResponseProcessProposal{
		Status: abci.ResponseProcessProposal_ACCEPT,
	}, nil
}

// PotPreBlocker processes vote extension injection data before BeginBlock.
// Called by baseapp before module BeginBlockers run.
func (app *ZeroneApp) PotPreBlocker(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	// Check first tx for VEX injection
	if len(req.Txs) > 0 && IsVoteExtInjectionTx(req.Txs[0]) {
		app.ProcessVoteExtInjection(ctx, req.Txs[0])
	}

	// Authoritative tx count for block rewards.
	// This MUST be set here (not only in PrepareProposal/ProcessProposal) because
	// during fast sync and block replay, FinalizeBlock runs without those handlers.
	// Without this, replaying nodes would compute hasTransactions=false → 0 rewards
	// → AppHash divergence.
	userTxCount := 0
	for _, txBytes := range req.Txs {
		if !IsVoteExtInjectionTx(txBytes) {
			userTxCount++
		}
	}
	app.VestingRewardsKeeper.SetBlockTxCount(userTxCount)

	return &sdk.ResponsePreBlock{}, nil
}

// ---- Internal Methods ----

// processVoteExtensions collects commitments and reveals from vote extensions.
// Returns a VoteExtInjection with deterministic ordering for all validators.
func (app *ZeroneApp) processVoteExtensions(
	ctx sdk.Context,
	votes []abci.ExtendedVoteInfo,
) VoteExtInjection {
	var inj VoteExtInjection

	for _, vote := range votes {
		if len(vote.VoteExtension) == 0 {
			continue
		}

		var ext VoteExtension
		if err := json.Unmarshal(vote.VoteExtension, &ext); err != nil {
			continue
		}

		if ext.ValidatorAddress == "" {
			continue
		}

		for _, c := range ext.Commitments {
			if c.RoundID == "" || c.CommitmentHash == "" {
				continue
			}
			inj.Commitments = append(inj.Commitments, InjectedCommitment{
				RoundID:        c.RoundID,
				Validator:      ext.ValidatorAddress,
				CommitmentHash: c.CommitmentHash,
				VRFOutput:      c.VRFOutput,
				VRFProof:       c.VRFProof,
			})
		}

		for _, r := range ext.Reveals {
			if r.RoundID == "" || r.Salt == "" {
				continue
			}
			inj.Reveals = append(inj.Reveals, InjectedReveal{
				RoundID:    r.RoundID,
				Validator:  ext.ValidatorAddress,
				Verdict:    r.Verdict,
				Confidence: r.Confidence,
				Salt:       r.Salt,
			})
		}
	}

	// Sort for deterministic ordering (all validators must process the same data)
	sort.Slice(inj.Commitments, func(i, j int) bool {
		if inj.Commitments[i].RoundID != inj.Commitments[j].RoundID {
			return inj.Commitments[i].RoundID < inj.Commitments[j].RoundID
		}
		return inj.Commitments[i].Validator < inj.Commitments[j].Validator
	})
	sort.Slice(inj.Reveals, func(i, j int) bool {
		if inj.Reveals[i].RoundID != inj.Reveals[j].RoundID {
			return inj.Reveals[i].RoundID < inj.Reveals[j].RoundID
		}
		return inj.Reveals[i].Validator < inj.Reveals[j].Validator
	})

	return inj
}

// ProcessVoteExtInjection processes a vote extension injection tx by storing
// commitments and reveals in the knowledge keeper state.
// Called from PreBlocker before BeginBlock phase transitions.
func (app *ZeroneApp) ProcessVoteExtInjection(ctx sdk.Context, data []byte) {
	logger := ctx.Logger().With("module", "abci", "handler", "PreBlocker")

	defer func() {
		if r := recover(); r != nil {
			logger.Error("PANIC in ProcessVoteExtInjection — skipping injection",
				"height", ctx.BlockHeight(), "panic", fmt.Sprintf("%v", r))
		}
	}()

	inj, err := DecodeVoteExtInjection(data)
	if err != nil {
		logger.Error("failed to decode vote extension injection", "err", err)
		return
	}

	height := uint64(ctx.BlockHeight())
	storedCommitments := 0
	storedReveals := 0

	// Store commitments in keeper state
	for _, c := range inj.Commitments {
		// Re-verify VRF proof before storing each commitment.
		// A malicious proposer could construct a fake injection with invalid proofs.
		if c.VRFOutput == "" || c.VRFProof == "" {
			logger.Warn("injected commitment missing VRF proof — discarding",
				"round_id", c.RoundID,
				"validator", c.Validator,
			)
			continue
		}

		vrfOutput, err := hex.DecodeString(c.VRFOutput)
		if err != nil {
			logger.Warn("invalid VRF output hex in commitment", "round_id", c.RoundID)
			continue
		}
		vrfProof, err := hex.DecodeString(c.VRFProof)
		if err != nil {
			logger.Warn("invalid VRF proof hex in commitment", "round_id", c.RoundID)
			continue
		}

		selected, err := app.KnowledgeKeeper.VerifyValidatorVRFSelection(
			ctx, c.RoundID, c.Validator, vrfOutput, vrfProof,
		)
		if err != nil || !selected {
			logger.Warn("VRF selection verification failed for injected commitment",
				"round_id", c.RoundID,
				"validator", c.Validator,
				"err", err,
			)
			continue
		}

		commitHash, err := hex.DecodeString(c.CommitmentHash)
		if err != nil {
			logger.Warn("invalid commitment hash hex", "round_id", c.RoundID)
			continue
		}

		commitment := &knowledgetypes.CommitEntry{
			Verifier:         c.Validator,
			CommitHash:       commitHash,
			CommittedAtBlock: height,
		}

		if err := app.KnowledgeKeeper.StoreCommitmentInRound(ctx, c.RoundID, commitment); err != nil {
			if errors.Is(err, knowledgetypes.ErrEquivocation) {
				logger.Error("EQUIVOCATION in vote extension commitment",
					"round_id", c.RoundID,
					"validator", c.Validator,
					"error", err.Error(),
				)
			} else {
				logger.Debug("skipped commitment from vote extension",
					"round_id", c.RoundID,
					"validator", c.Validator,
					"reason", err.Error(),
				)
			}
			continue
		}
		storedCommitments++
	}

	// Store reveals in keeper state
	for _, r := range inj.Reveals {
		saltBytes, err := hex.DecodeString(r.Salt)
		if err != nil {
			logger.Warn("invalid salt hex in reveal", "round_id", r.RoundID)
			continue
		}

		reveal := &knowledgetypes.RevealEntry{
			Verifier:        r.Validator,
			Vote:            r.Verdict,
			Salt:            saltBytes,
			RevealedAtBlock: height,
		}

		if err := app.KnowledgeKeeper.StoreRevealInRound(ctx, r.RoundID, reveal, r.Confidence); err != nil {
			if errors.Is(err, knowledgetypes.ErrEquivocation) {
				logger.Error("EQUIVOCATION in vote extension reveal",
					"round_id", r.RoundID,
					"validator", r.Validator,
					"error", err.Error(),
				)
			} else {
				logger.Debug("skipped reveal from vote extension",
					"round_id", r.RoundID,
					"validator", r.Validator,
					"reason", err.Error(),
				)
			}
			continue
		}
		storedReveals++
	}

	if storedCommitments > 0 || storedReveals > 0 {
		logger.Info("processed vote extension injection",
			"height", height,
			"commitments_stored", storedCommitments,
			"commitments_total", len(inj.Commitments),
			"reveals_stored", storedReveals,
			"reveals_total", len(inj.Reveals),
		)
	}
}
