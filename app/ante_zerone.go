package app

import (
	"context"
	"encoding/hex"
	"fmt"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	zeroneauthkeeper "github.com/zerone-chain/zerone/x/auth/keeper"
	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	emergencykeeper "github.com/zerone-chain/zerone/x/emergency/keeper"
	emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
)

// txSigner holds a signer's address and pubkey, extracted from tx signature data.
type txSigner struct {
	Address    sdk.AccAddress
	PubKeyHex string // hex-encoded pubkey bytes
}

// getSignerAddresses extracts all signer addresses from a tx's signature data.
// This works for ALL message types (SDK proto-generated and hand-written Zerone types)
// unlike per-message type assertions which only work for hand-written types.
//
// ANTE P0-1 FIX: Replaces per-message type assertion that silently skipped
// SDK v0.50 proto-generated types (MsgSend, MsgDelegate, etc.), allowing
// frozen accounts and session key capability bypass.
func getSignerAddresses(tx sdk.Tx) []sdk.AccAddress {
	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return nil
	}
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return nil
	}
	addrs := make([]sdk.AccAddress, 0, len(sigs))
	for _, sig := range sigs {
		if sig.PubKey == nil {
			continue
		}
		addrs = append(addrs, sdk.AccAddress(sig.PubKey.Address()))
	}
	return addrs
}

// getTxSigners extracts signer addresses and pubkeys from tx signature data.
// Used by the capability decorator which needs pubkeys for session key matching.
func getTxSigners(tx sdk.Tx) []txSigner {
	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return nil
	}
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return nil
	}
	signers := make([]txSigner, 0, len(sigs))
	for _, sig := range sigs {
		if sig.PubKey == nil {
			continue
		}
		signers = append(signers, txSigner{
			Address:    sdk.AccAddress(sig.PubKey.Address()),
			PubKeyHex: hex.EncodeToString(sig.PubKey.Bytes()),
		})
	}
	return signers
}

// ---------- BootstrapGasFreeDecorator ----------

// BootstrapGasFreeDecorator waives gas and fees for essential PoT transactions
// during the bootstrap period (first 480,000 blocks ~ 14 days).
// This allows the network to begin verifying truth before any ZRN is minted.
type BootstrapGasFreeDecorator struct{}

// NewBootstrapGasFreeDecorator creates a new BootstrapGasFreeDecorator.
func NewBootstrapGasFreeDecorator() BootstrapGasFreeDecorator {
	return BootstrapGasFreeDecorator{}
}

func (bgd BootstrapGasFreeDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if ctx.BlockHeight() > BootstrapEndBlock {
		return next(ctx, tx, simulate)
	}

	// Check if ALL messages in the tx are bootstrap-eligible
	msgs := tx.GetMsgs()
	allEligible := len(msgs) > 0
	for _, msg := range msgs {
		if !BootstrapGasFreeTypes[sdk.MsgTypeURL(msg)] {
			allEligible = false
			break
		}
	}

	if !allEligible {
		return next(ctx, tx, simulate)
	}

	// Set gas meter to the block gas limit so bootstrap txs can consume
	// gas freely without hitting out-of-gas errors. We can't use
	// InfiniteGasMeter or math.MaxInt64 because CometBFT's mempool
	// rejects txs with gas_wanted exceeding ConsensusParams.Block.MaxGas.
	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(BlockGasLimit))

	return next(ctx, tx, simulate)
}

// ---------- EmergencyHaltDecorator ----------

// EmergencyHaltDecorator blocks all non-emergency transactions when the chain
// is in a halted state. Emergency module messages (propose/vote halt/revert/resume)
// are always allowed so guardians can coordinate recovery.
//
// Placed early in the ante chain (after SetUpContext, before gas/fee processing)
// so halted transactions don't consume gas or fees.
type EmergencyHaltDecorator struct {
	ek emergencykeeper.Keeper
}

// NewEmergencyHaltDecorator creates a new EmergencyHaltDecorator.
func NewEmergencyHaltDecorator(ek emergencykeeper.Keeper) EmergencyHaltDecorator {
	return EmergencyHaltDecorator{ek: ek}
}

func (ehd EmergencyHaltDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if simulate {
		return next(ctx, tx, simulate)
	}

	if !ehd.ek.IsHalted(ctx) {
		return next(ctx, tx, simulate)
	}

	// Chain is halted — only allow emergency module messages
	for _, msg := range tx.GetMsgs() {
		if !emergencytypes.IsEmergencyMsg(msg) {
			return ctx, emergencytypes.ErrChainHalted
		}
	}

	return next(ctx, tx, simulate)
}

// ---------- ZRNGasDecorator ----------

// ZRNGasDecorator validates that transactions provide sufficient gas based on
// the ZRN gas cost table (translated from core/billing/gas.ts).
type ZRNGasDecorator struct{}

// NewZRNGasDecorator creates a new ZRNGasDecorator.
func NewZRNGasDecorator() ZRNGasDecorator {
	return ZRNGasDecorator{}
}

func (zgd ZRNGasDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if simulate {
		return next(ctx, tx, simulate)
	}

	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return ctx, errors.Wrap(sdkerrors.ErrTxDecode, "tx must implement FeeTx")
	}

	gasLimit := feeTx.GetGas()

	// Enforce block gas limit
	if gasLimit > BlockGasLimit {
		return ctx, errors.Wrapf(sdkerrors.ErrOutOfGas,
			"tx gas limit %d exceeds block gas limit %d", gasLimit, BlockGasLimit)
	}

	// Enforce per-tx gas limit
	if gasLimit > TxGasLimit {
		return ctx, errors.Wrapf(sdkerrors.ErrOutOfGas,
			"tx gas limit %d exceeds per-tx limit %d", gasLimit, TxGasLimit)
	}

	// Calculate minimum required gas from message types.
	// ANTE P1-2 FIX: Saturating addition to prevent uint64 overflow.
	msgs := tx.GetMsgs()
	var totalRequiredGas uint64
	for _, msg := range msgs {
		msgType := sdk.MsgTypeURL(msg)
		requiredGas := lookupMsgGas(msgType)
		totalRequiredGas += requiredGas
		if totalRequiredGas < requiredGas { // overflow detected
			return ctx, errors.Wrap(sdkerrors.ErrOutOfGas, "gas requirement overflow")
		}
	}

	if totalRequiredGas < MinGasLimit {
		totalRequiredGas = MinGasLimit
	}

	if gasLimit < totalRequiredGas {
		return ctx, errors.Wrapf(sdkerrors.ErrOutOfGas,
			"tx gas limit %d below minimum required %d for %d messages",
			gasLimit, totalRequiredGas, len(msgs))
	}

	// Enforce minimum gas price
	feeCoins := feeTx.GetFee()
	if !feeCoins.IsZero() {
		minFee := sdk.NewCoin(BondDenom, math.NewIntFromUint64(gasLimit*MinGasPrice))
		if feeCoins.AmountOf(BondDenom).LT(minFee.Amount) {
			return ctx, errors.Wrapf(sdkerrors.ErrInsufficientFee,
				"fee %s below minimum %s (gas %d * min price %d)",
				feeCoins, minFee, gasLimit, MinGasPrice)
		}
	}

	return next(ctx, tx, simulate)
}

// lookupMsgGas maps a Cosmos SDK message type URL to its ZRN gas cost.
// Type URLs are like "/zerone.knowledge.v1.MsgSubmitClaim".
func lookupMsgGas(msgTypeURL string) uint64 {
	if cost, ok := msgTypeURLToGas[msgTypeURL]; ok {
		return cost
	}
	// Unknown message types get minimum gas
	return MinGasLimit
}

// msgTypeURLToGas maps protobuf message type URLs to gas costs.
// Built from the TransactionGasCosts table and proto service definitions.
var msgTypeURLToGas = map[string]uint64{
	// Standard Cosmos messages
	"/cosmos.bank.v1beta1.MsgSend":               TransactionGasCosts["transfer"],
	"/cosmos.bank.v1beta1.MsgMultiSend":          TransactionGasCosts["transfer"] * 2,
	"/cosmos.staking.v1beta1.MsgDelegate":        TransactionGasCosts["delegate"],
	"/cosmos.staking.v1beta1.MsgUndelegate":      TransactionGasCosts["undelegate"],
	"/cosmos.staking.v1beta1.MsgBeginRedelegate": TransactionGasCosts["redelegate"],
	"/cosmos.gov.v1.MsgSubmitProposal":           TransactionGasCosts["governance_propose"],
	"/cosmos.gov.v1.MsgVote":                     TransactionGasCosts["governance_vote"],

	// IBC
	"/ibc.applications.transfer.v1.MsgTransfer": TransactionGasCosts["transfer"],

	// Knowledge module
	"/zerone.knowledge.v1.MsgSubmitClaim":             TransactionGasCosts["claim_submit"],
	"/zerone.knowledge.v1.MsgSubmitCommitment":        TransactionGasCosts["verification_commit"],
	"/zerone.knowledge.v1.MsgSubmitReveal":            TransactionGasCosts["verification_reveal"],
	"/zerone.knowledge.v1.MsgChallengeFact":           TransactionGasCosts["challenge_fact"],
	"/zerone.knowledge.v1.MsgAddFact":                 TransactionGasCosts["add_fact"],
	"/zerone.knowledge.v1.MsgSubmitContradiction":     TransactionGasCosts["submit_contradiction"],
	"/zerone.knowledge.v1.MsgPatronizeFact":           TransactionGasCosts["patronize_fact"],
	"/zerone.knowledge.v1.MsgProposeDomain":           TransactionGasCosts["propose_domain"],
	"/zerone.knowledge.v1.MsgEndorseDomainProposal":   TransactionGasCosts["endorse_domain"],
	"/zerone.knowledge.v1.MsgChallengeDomainProposal": TransactionGasCosts["challenge_domain"],
	"/zerone.knowledge.v1.MsgRegisterStratum":         TransactionGasCosts["register_stratum"],

	// Knowledge (extended — hand-written types)
	"/zerone.knowledge.v1.MsgChallengeProvisionalFact": TransactionGasCosts["challenge_provisional_fact"],
	"/zerone.knowledge.v1.MsgUpdateExtendedParams":     TransactionGasCosts["update_extended_params"],

	// Auth module (agent keys)
	"/zerone.auth.v1.MsgRegisterAccount": TransactionGasCosts["register_account"],
	"/zerone.auth.v1.MsgRotateKey":       TransactionGasCosts["rotate_key"],
	"/zerone.auth.v1.MsgCreateSession":   TransactionGasCosts["create_session"],
	"/zerone.auth.v1.MsgRevokeSession":   TransactionGasCosts["revoke_session"],

	// Staking module (extended)
	"/zerone.staking.v1.MsgRegisterValidator":    TransactionGasCosts["register_validator"],
	"/zerone.staking.v1.MsgUpdateValidatorStake": TransactionGasCosts["update_validator_stake"],
	"/zerone.staking.v1.MsgDelegate":             TransactionGasCosts["zerone_delegate"],
	"/zerone.staking.v1.MsgUndelegate":           TransactionGasCosts["zerone_undelegate"],
	"/zerone.staking.v1.MsgRedelegate":           TransactionGasCosts["zerone_redelegate"],

	// Disputes
	"/zerone.disputes.v1.MsgInitiateDispute": TransactionGasCosts["initiate_dispute"],
	"/zerone.disputes.v1.MsgCommitEvidence":  TransactionGasCosts["commit_evidence"],
	"/zerone.disputes.v1.MsgRevealEvidence":  TransactionGasCosts["reveal_evidence"],
	"/zerone.disputes.v1.MsgArbiterVote":     TransactionGasCosts["arbiter_vote"],
	"/zerone.disputes.v1.MsgEscalateDispute": TransactionGasCosts["escalate_dispute"],
	"/zerone.disputes.v1.MsgSettleDispute":   TransactionGasCosts["settle_dispute"],
	"/zerone.disputes.v1.MsgTimeoutDispute":  TransactionGasCosts["timeout_dispute"],

	// Disputes (extended — hand-written types)
	"/zerone.disputes.v1.MsgOpenDispute":    TransactionGasCosts["open_dispute"],
	"/zerone.disputes.v1.MsgSubmitEvidence": TransactionGasCosts["dispute_submit_evidence"],
	"/zerone.disputes.v1.MsgVoteDispute":    TransactionGasCosts["vote_dispute"],

	// Research
	"/zerone.research.v1.MsgSubmitResearch":    TransactionGasCosts["submit_research"],
	"/zerone.research.v1.MsgChallengeResearch": TransactionGasCosts["challenge_research"],
	"/zerone.research.v1.MsgReviewResearch":    TransactionGasCosts["review_research"],
	"/zerone.research.v1.MsgResolveResearch":   TransactionGasCosts["resolve_research"],
	"/zerone.research.v1.MsgCreateBounty":      TransactionGasCosts["create_bounty"],
	"/zerone.research.v1.MsgClaimBounty":       TransactionGasCosts["claim_bounty"],
	"/zerone.research.v1.MsgFulfillBounty":     TransactionGasCosts["fulfill_bounty"],
	"/zerone.research.v1.MsgFundResearch":      TransactionGasCosts["fund_research"],

	// Evidence management
	"/zerone.evidence_mgmt.v1.MsgSubmitEvidence":   TransactionGasCosts["submit_evidence"],
	"/zerone.evidence_mgmt.v1.MsgAuditEvidence":    TransactionGasCosts["audit_evidence"],
	"/zerone.evidence_mgmt.v1.MsgRegisterOracle":   TransactionGasCosts["register_oracle"],
	"/zerone.evidence_mgmt.v1.MsgSubmitOracleData": TransactionGasCosts["submit_oracle_data"],

	// Vesting rewards
	"/zerone.vesting_rewards.v1.MsgCreateVesting":     TransactionGasCosts["create_vesting"],
	"/zerone.vesting_rewards.v1.MsgClaimVesting":      TransactionGasCosts["claim_vesting"],
	"/zerone.vesting_rewards.v1.MsgPauseVesting":      TransactionGasCosts["pause_vesting"],
	"/zerone.vesting_rewards.v1.MsgResumeVesting":     TransactionGasCosts["resume_vesting"],
	"/zerone.vesting_rewards.v1.MsgAccelerateVesting": TransactionGasCosts["accelerate_vesting"],
	"/zerone.vesting_rewards.v1.MsgFalsifyVesting":    TransactionGasCosts["falsify_vesting"],
	"/zerone.vesting_rewards.v1.MsgCompleteVesting":   TransactionGasCosts["complete_vesting"],

	// Payment channels
	"/zerone.channels.v1.MsgOpenChannel":    TransactionGasCosts["open_channel"],
	"/zerone.channels.v1.MsgUpdateChannel":  TransactionGasCosts["update_channel"],
	"/zerone.channels.v1.MsgCloseChannel":   TransactionGasCosts["close_channel"],
	"/zerone.channels.v1.MsgSettleChannel":  TransactionGasCosts["settle_channel"],
	"/zerone.channels.v1.MsgDisputeChannel": TransactionGasCosts["dispute_channel"],

	// Payment channels (extended — hand-written types)
	"/zerone.channels.v1.MsgDepositChannel": TransactionGasCosts["deposit_channel"],
	"/zerone.channels.v1.MsgUpdateState":    TransactionGasCosts["update_channel_state"],
	"/zerone.channels.v1.MsgClaimExpired":   TransactionGasCosts["claim_expired_channel"],

	// Tree of Life — projects
	"/zerone.tree.v1.MsgCreateProject":     TransactionGasCosts["create_project"],
	"/zerone.tree.v1.MsgProposeProject":    TransactionGasCosts["propose_project"],
	"/zerone.tree.v1.MsgStartDevelopment":  TransactionGasCosts["start_development"],
	"/zerone.tree.v1.MsgCompleteProject":   TransactionGasCosts["complete_project"],
	"/zerone.tree.v1.MsgPauseProject":      TransactionGasCosts["pause_project"],
	"/zerone.tree.v1.MsgResumeProject":     TransactionGasCosts["resume_project"],
	"/zerone.tree.v1.MsgAbandonProject":    TransactionGasCosts["abandon_project"],
	"/zerone.tree.v1.MsgSpawnChildProject": TransactionGasCosts["spawn_child_project"],

	// Tree of Life — tasks
	"/zerone.tree.v1.MsgAddTask":            TransactionGasCosts["add_task"],
	"/zerone.tree.v1.MsgAssignTask":         TransactionGasCosts["assign_task"],
	"/zerone.tree.v1.MsgStartWork":          TransactionGasCosts["start_work"],
	"/zerone.tree.v1.MsgSubmitDeliverable":  TransactionGasCosts["submit_deliverable"],
	"/zerone.tree.v1.MsgApproveDeliverable": TransactionGasCosts["approve_deliverable"],
	"/zerone.tree.v1.MsgRejectDeliverable":  TransactionGasCosts["reject_deliverable"],
	"/zerone.tree.v1.MsgReopenTask":         TransactionGasCosts["reopen_task"],

	// Tree of Life — services
	"/zerone.tree.v1.MsgDeployService":    TransactionGasCosts["deploy_service"],
	"/zerone.tree.v1.MsgCallService":      TransactionGasCosts["call_service"],
	"/zerone.tree.v1.MsgSubscribeService": TransactionGasCosts["subscribe_service"],
	"/zerone.tree.v1.MsgPauseService":     TransactionGasCosts["pause_service"],
	"/zerone.tree.v1.MsgResumeService":    TransactionGasCosts["resume_service"],
	"/zerone.tree.v1.MsgRetireService":    TransactionGasCosts["retire_service"],

	// Tree of Life — seeds & contributors
	"/zerone.tree.v1.MsgBeginSeeding":      TransactionGasCosts["begin_seeding"],
	"/zerone.tree.v1.MsgDetectOpportunity": TransactionGasCosts["detect_opportunity"],
	"/zerone.tree.v1.MsgClaimOpportunity":  TransactionGasCosts["claim_opportunity"],
	"/zerone.tree.v1.MsgAddContributor":    TransactionGasCosts["add_contributor"],
	"/zerone.tree.v1.MsgApplyToProject":    TransactionGasCosts["apply_to_project"],
	"/zerone.tree.v1.MsgReviewApplication": TransactionGasCosts["review_application"],
	"/zerone.tree.v1.MsgSetAvailability":   TransactionGasCosts["set_availability"],

	// BVM
	"/zerone.bvm.v1.MsgDeployContract":    TransactionGasCosts["deploy_contract"],
	"/zerone.bvm.v1.MsgCallContract":      TransactionGasCosts["call_contract"],
	"/zerone.bvm.v1.MsgScheduleExecution": TransactionGasCosts["schedule_execution"],

	// BVM (extended — hand-written types)
	"/zerone.bvm.v1.MsgScheduleContract":    TransactionGasCosts["schedule_contract"],
	"/zerone.bvm.v1.MsgCancelSchedule":      TransactionGasCosts["cancel_bvm_schedule"],
	"/zerone.bvm.v1.MsgUpdateContractState": TransactionGasCosts["update_contract_state"],

	// Governance (extended)
	"/zerone.gov.v1.MsgSubmitLIP":           TransactionGasCosts["submit_lip"],
	"/zerone.gov.v1.MsgCastVote":            TransactionGasCosts["cast_vote"],
	"/zerone.gov.v1.MsgLockVote":            TransactionGasCosts["lock_vote"],
	"/zerone.gov.v1.MsgUnlockVote":          TransactionGasCosts["unlock_vote"],
	"/zerone.gov.v1.MsgCommitReview":        TransactionGasCosts["commit_review"],
	"/zerone.gov.v1.MsgRevealReview":        TransactionGasCosts["reveal_review"],
	"/zerone.gov.v1.MsgSubmitDisbursement":  TransactionGasCosts["submit_disbursement"],
	"/zerone.gov.v1.MsgExecuteDisbursement": TransactionGasCosts["execute_disbursement"],

	// Emergency
	"/zerone.emergency.v1.MsgProposeHalt":   TransactionGasCosts["propose_halt"],
	"/zerone.emergency.v1.MsgVoteHalt":      TransactionGasCosts["vote_halt"],
	"/zerone.emergency.v1.MsgProposeRevert": TransactionGasCosts["propose_revert"],
	"/zerone.emergency.v1.MsgVoteRevert":    TransactionGasCosts["vote_revert"],
	"/zerone.emergency.v1.MsgProposeResume": TransactionGasCosts["propose_resume"],
	"/zerone.emergency.v1.MsgVoteResume":    TransactionGasCosts["vote_resume"],

	// Claiming pots
	"/zerone.claiming_pot.v1.MsgCreatePot":    TransactionGasCosts["create_pot"],
	"/zerone.claiming_pot.v1.MsgFundPot":      TransactionGasCosts["fund_pot"],
	"/zerone.claiming_pot.v1.MsgClaimFromPot": TransactionGasCosts["claim_from_pot"],
	"/zerone.claiming_pot.v1.MsgClosePot":     TransactionGasCosts["close_pot"],

	// Capture defense
	"/zerone.capture_defense.v1.MsgRequestQualification": TransactionGasCosts["request_capture_qualification"],
	"/zerone.capture_defense.v1.MsgEndorseQualification": TransactionGasCosts["endorse_capture_qualification"],

	// Capture challenge
	"/zerone.capture_challenge.v1.MsgSubmitChallenge":  TransactionGasCosts["submit_capture_challenge"],
	"/zerone.capture_challenge.v1.MsgAddEvidence":      TransactionGasCosts["add_challenge_evidence"],
	"/zerone.capture_challenge.v1.MsgResolveChallenge": TransactionGasCosts["resolve_capture_challenge"],

	// Agent Home
	"/zerone.home.v1.MsgCreateHome":        TransactionGasCosts["create_home"],
	"/zerone.home.v1.MsgUpdateHome":        TransactionGasCosts["update_home"],
	"/zerone.home.v1.MsgUpdateMemoryCID":   TransactionGasCosts["update_memory_cid"],
	"/zerone.home.v1.MsgStartSession":      TransactionGasCosts["home_start_session"],
	"/zerone.home.v1.MsgEndSession":        TransactionGasCosts["home_end_session"],
	"/zerone.home.v1.MsgRegisterKey":       TransactionGasCosts["home_register_key"],
	"/zerone.home.v1.MsgRevokeKey":         TransactionGasCosts["home_revoke_key"],
	"/zerone.home.v1.MsgConfigureGuardian": TransactionGasCosts["configure_guardian"],
	"/zerone.home.v1.MsgAcknowledgeAlert":  TransactionGasCosts["acknowledge_alert"],
	"/zerone.home.v1.MsgSetSpendingLimit":  TransactionGasCosts["set_spending_limit"],

	// Qualification
	"/zerone.qualification.v1.MsgRequestQualification":  TransactionGasCosts["request_qualification"],
	"/zerone.qualification.v1.MsgEndorseValidator":      TransactionGasCosts["endorse_validator"],
	"/zerone.qualification.v1.MsgRenewQualification":    TransactionGasCosts["renew_qualification"],
	"/zerone.qualification.v1.MsgWithdrawQualification": TransactionGasCosts["withdraw_qualification"],

	// Auth recovery (hand-written ExtendedMsg service)
	"/zerone.auth.v1.MsgRecoverAccount":      TransactionGasCosts["recover_account"],
	"/zerone.auth.v1.MsgFreezeAccount":       TransactionGasCosts["freeze_account"],
	"/zerone.auth.v1.MsgUnfreezeAccount":     TransactionGasCosts["unfreeze_account"],
	"/zerone.auth.v1.MsgSetRecoveryConfig":   TransactionGasCosts["set_recovery_config"],
	"/zerone.auth.v1.MsgInitiateRecovery":    TransactionGasCosts["initiate_recovery"],
	"/zerone.auth.v1.MsgSubmitRecoveryShard": TransactionGasCosts["submit_recovery_shard"],
	"/zerone.auth.v1.MsgChallengeRecovery":   TransactionGasCosts["challenge_recovery"],
	"/zerone.auth.v1.MsgExecuteRecovery":     TransactionGasCosts["execute_recovery"],

	// Governance extras
	"/zerone.gov.v1.MsgAttachUpgradePlan":   TransactionGasCosts["attach_upgrade_plan"],
	"/zerone.gov.v1.MsgStakeLIP":            TransactionGasCosts["governance_stake_lip"],
	"/zerone.gov.v1.MsgAmendLIP":            TransactionGasCosts["amend_lip"],
	"/zerone.gov.v1.MsgAdvanceLIPStage":     TransactionGasCosts["advance_lip_stage"],
	"/zerone.gov.v1.MsgWithdrawLIP":         TransactionGasCosts["withdraw_lip"],
	"/zerone.gov.v1.MsgSwitchVote":          TransactionGasCosts["switch_vote"],
	"/zerone.gov.v1.MsgFinalizeReview":      TransactionGasCosts["finalize_review"],
	"/zerone.gov.v1.MsgStakeDisbursement":   TransactionGasCosts["stake_disbursement"],
	"/zerone.gov.v1.MsgAdvanceDisbursement": TransactionGasCosts["advance_disbursement"],
	"/zerone.gov.v1.MsgCancelDisbursement":  TransactionGasCosts["cancel_disbursement"],
	"/zerone.gov.v1.MsgRegisterOperator":    TransactionGasCosts["register_operator"],
	"/zerone.gov.v1.MsgAddAgent":            TransactionGasCosts["add_agent"],
	"/zerone.gov.v1.MsgRemoveAgent":         TransactionGasCosts["remove_agent"],
	"/zerone.gov.v1.MsgSlashOperator":       TransactionGasCosts["slash_operator"],
	"/zerone.gov.v1.MsgCreateDeployment":    TransactionGasCosts["create_deployment"],
	"/zerone.gov.v1.MsgAdvanceDeployment":   TransactionGasCosts["advance_deployment"],
	"/zerone.gov.v1.MsgApproveDeployment":   TransactionGasCosts["approve_deployment"],
	"/zerone.gov.v1.MsgRollbackDeployment":  TransactionGasCosts["rollback_deployment"],

	// Discovery
	"/zerone.discovery.v1.MsgRegisterAgent":    TransactionGasCosts["register_agent"],
	"/zerone.discovery.v1.MsgUpdateProfile":    TransactionGasCosts["update_profile"],
	"/zerone.discovery.v1.MsgAddCapability":    TransactionGasCosts["add_capability"],
	"/zerone.discovery.v1.MsgRemoveCapability": TransactionGasCosts["remove_capability"],
	"/zerone.discovery.v1.MsgDeregisterAgent":  TransactionGasCosts["deregister_agent"],
	"/zerone.discovery.v1.MsgHeartbeat":        TransactionGasCosts["heartbeat"],

	// Partnerships
	"/zerone.partnerships.v1.MsgInitiatePartnership": TransactionGasCosts["initiate_partnership"],
	"/zerone.partnerships.v1.MsgAcceptPartnership":   TransactionGasCosts["accept_partnership"],
	"/zerone.partnerships.v1.MsgDepositToPot":        TransactionGasCosts["deposit_to_pot"],
	"/zerone.partnerships.v1.MsgDistributeReward":    TransactionGasCosts["distribute_reward"],
	"/zerone.partnerships.v1.MsgProposeOperation":    TransactionGasCosts["propose_operation"],
	"/zerone.partnerships.v1.MsgApproveOperation":    TransactionGasCosts["approve_operation"],
	"/zerone.partnerships.v1.MsgRejectOperation":     TransactionGasCosts["reject_operation"],
	"/zerone.partnerships.v1.MsgSafetyFreeze":        TransactionGasCosts["safety_freeze"],
	"/zerone.partnerships.v1.MsgRaiseCoercionSignal": TransactionGasCosts["raise_coercion_signal"],
	"/zerone.partnerships.v1.MsgInitiateExit":        TransactionGasCosts["initiate_exit"],

	// Ontology
	"/zerone.ontology.v1.MsgProposeDomain":             TransactionGasCosts["propose_domain"],
	"/zerone.ontology.v1.MsgVoteDomainProposal":        TransactionGasCosts["vote_domain_proposal"],
	"/zerone.ontology.v1.MsgUpdateDomain":              TransactionGasCosts["update_domain"],
	"/zerone.ontology.v1.MsgRegisterLogicZone":         TransactionGasCosts["register_logic_zone"],
	"/zerone.ontology.v1.MsgAcknowledgeIncompleteness": TransactionGasCosts["acknowledge_incompleteness"],

	// Schedule
	"/zerone.schedule.v1.MsgCreateSchedule": TransactionGasCosts["create_schedule"],
	"/zerone.schedule.v1.MsgPauseSchedule":  TransactionGasCosts["pause_schedule"],
	"/zerone.schedule.v1.MsgResumeSchedule": TransactionGasCosts["resume_schedule"],
	"/zerone.schedule.v1.MsgCancelSchedule": TransactionGasCosts["cancel_schedule"],
	"/zerone.schedule.v1.MsgAddFee":         TransactionGasCosts["add_fee"],

	// Billing
	"/zerone.billing.v1.MsgRegisterProvider":   TransactionGasCosts["register_provider"],
	"/zerone.billing.v1.MsgDeregisterProvider": TransactionGasCosts["deregister_provider"],
	"/zerone.billing.v1.MsgRequestQuote":       TransactionGasCosts["request_quote"],
	"/zerone.billing.v1.MsgExecutePayment":     TransactionGasCosts["execute_payment"],

	// Autopoiesis
	"/zerone.autopoiesis.v1.MsgActivateAutopoiesis": TransactionGasCosts["activate_autopoiesis"],
	"/zerone.autopoiesis.v1.MsgOverrideMultiplier":  TransactionGasCosts["override_multiplier"],
	"/zerone.autopoiesis.v1.MsgFreezeMultiplier":    TransactionGasCosts["freeze_multiplier"],

	// Compute pool
	"/zerone.compute_pool.v1.MsgRegisterComputeProvider": TransactionGasCosts["register_compute_provider"],
	"/zerone.compute_pool.v1.MsgUpdateProvider":          TransactionGasCosts["update_compute_provider"],
	"/zerone.compute_pool.v1.MsgDeregisterProvider":      TransactionGasCosts["deregister_compute_provider"],
	"/zerone.compute_pool.v1.MsgProviderHeartbeat":       TransactionGasCosts["provider_heartbeat"],
	"/zerone.compute_pool.v1.MsgRedeemCredits":           TransactionGasCosts["redeem_credits"],
	"/zerone.compute_pool.v1.MsgConfirmUsage":            TransactionGasCosts["confirm_usage"],
	"/zerone.compute_pool.v1.MsgDisputeUsage":            TransactionGasCosts["dispute_usage"],
	"/zerone.compute_pool.v1.MsgUpdateComputeParams":     TransactionGasCosts["update_compute_params"],

	// ICA auth
	"/zerone.icaauth.v1.MsgRegisterInterchainAccount": TransactionGasCosts["register_ica"],
	"/zerone.icaauth.v1.MsgSendICATransaction":        TransactionGasCosts["send_ica_tx"],

	// Toolbox
	"/zerone.toolbox.v1.MsgRegisterTool":          TransactionGasCosts["register_tool"],
	"/zerone.toolbox.v1.MsgCallTool":              TransactionGasCosts["call_tool"],
	"/zerone.toolbox.v1.MsgAddContributor":        TransactionGasCosts["toolbox_add_contributor"],
	"/zerone.toolbox.v1.MsgAcceptContributorship": TransactionGasCosts["accept_contributorship"],
	"/zerone.toolbox.v1.MsgUpgradeTool":           TransactionGasCosts["upgrade_tool"],
	"/zerone.toolbox.v1.MsgDeprecateTool":         TransactionGasCosts["deprecate_tool"],
	"/zerone.toolbox.v1.MsgRetireTool":            TransactionGasCosts["retire_tool"],
	"/zerone.toolbox.v1.MsgLockShares":            TransactionGasCosts["lock_shares"],
	"/zerone.toolbox.v1.MsgUpdateDependency":      TransactionGasCosts["update_dependency"],
	"/zerone.toolbox.v1.MsgToolHeartbeat":         TransactionGasCosts["tool_heartbeat"],

	// Alignment
	"/zerone.alignment.v1.MsgActivate": TransactionGasCosts["activate_alignment"],

	// Liquidity pool
	"/zerone.liquiditypool.v1.MsgCreatePool":      TransactionGasCosts["create_pool"],
	"/zerone.liquiditypool.v1.MsgSwap":             TransactionGasCosts["lp_swap"],
	"/zerone.liquiditypool.v1.MsgAddLiquidity":    TransactionGasCosts["lp_add_liquidity"],
	"/zerone.liquiditypool.v1.MsgRemoveLiquidity": TransactionGasCosts["lp_remove_liquidity"],

	// IBC rate limiting
	"/zerone.ibcratelimit.v1.MsgAddRateLimit":    TransactionGasCosts["add_rate_limit"],
	"/zerone.ibcratelimit.v1.MsgRemoveRateLimit": TransactionGasCosts["remove_rate_limit"],

	// Governance research spending
	"/zerone.gov.v1.MsgSubmitResearchSpend": TransactionGasCosts["submit_research_spend"],
	"/zerone.gov.v1.MsgVoteResearchSpend":   TransactionGasCosts["vote_research_spend"],
	"/zerone.gov.v1.MsgSetResearchVoters":   TransactionGasCosts["set_research_voters"],
}

// ---------- Fee Router Decorator ----------

// FeeRouterDecorator splits collected fees: 7% to research fund, 93% to validators.
// This implements the ZRN fee routing from core/billing/reward_router.ts.
type FeeRouterDecorator struct {
	bankKeeper bankkeeper.Keeper
}

// NewFeeRouterDecorator creates a new FeeRouterDecorator.
func NewFeeRouterDecorator(bk bankkeeper.Keeper) FeeRouterDecorator {
	return FeeRouterDecorator{bankKeeper: bk}
}

func (frd FeeRouterDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	// Fee routing is handled in x/vesting_rewards BeginBlocker via keeper.RouteFees().
	// The BeginBlocker sweeps 7% of fee_collector to research_fund before
	// x/distribution's BeginBlocker sends the remaining 93% to validators.
	//
	// This decorator only logs the intended split for observability.

	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return next(ctx, tx, simulate)
	}

	fee := feeTx.GetFee()
	if fee.IsZero() {
		return next(ctx, tx, simulate)
	}

	ctx.Logger().Debug("ZRN fee routing",
		"total_fee", fee.String(),
		"research_share_bps", ResearchContributionBPS,
		"validator_share_bps", ValidatorFeeBPS,
	)

	return next(ctx, tx, simulate)
}

// GetMinimumFee calculates the minimum fee for a transaction based on message types.
func GetMinimumFee(msgs []sdk.Msg) sdk.Coins {
	var totalGas uint64
	for _, msg := range msgs {
		totalGas += lookupMsgGas(sdk.MsgTypeURL(msg))
	}
	if totalGas < MinGasLimit {
		totalGas = MinGasLimit
	}

	minFee := math.NewIntFromUint64(totalGas * MinGasPrice)
	return sdk.NewCoins(sdk.NewCoin(BondDenom, minFee))
}

// ---------- ZeroneDIDDecorator ----------

// ZeroneDIDDecorator validates DID references in transactions.
// If a tx memo contains "did:zrn:", it validates the DID resolves to the sender
// and emits indexing events.
//
// Runs AFTER signature verification (post-auth) to prevent unauthenticated
// state reads. DID validation is an additional constraint, not a prerequisite.
//
// ANTE P0-1 FIX: Uses tx-level signer extraction for DID-to-signer matching.
type ZeroneDIDDecorator struct {
	zak zeroneauthkeeper.Keeper
}

// NewZeroneDIDDecorator creates a new ZeroneDIDDecorator.
func NewZeroneDIDDecorator(zak zeroneauthkeeper.Keeper) ZeroneDIDDecorator {
	return ZeroneDIDDecorator{zak: zak}
}

func (zdd ZeroneDIDDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if simulate {
		return next(ctx, tx, simulate)
	}

	// Check if tx has a memo with DID reference
	memoTx, ok := tx.(sdk.TxWithMemo)
	if !ok {
		return next(ctx, tx, simulate)
	}

	memo := memoTx.GetMemo()
	if len(memo) < 8 || memo[:8] != "did:zrn:" {
		return next(ctx, tx, simulate)
	}

	// Extract DID from memo (format: "did:zrn:{32-64hex}")
	did := memo
	if len(did) > 72 { // "did:zrn:" + 64 hex = 72
		did = memo[:72]
	}

	if err := zeroneauthtypes.ValidateDID(did); err != nil {
		return ctx, zeroneauthtypes.ErrDIDResolutionFailed
	}

	// Verify DID resolves to a known address
	address, found := zdd.zak.GetAddressForDID(ctx, did)
	if !found {
		return ctx, zeroneauthtypes.ErrDIDResolutionFailed
	}

	// Validate that the DID resolves to one of the tx signers.
	signers := getSignerAddresses(tx)
	senderMatch := false
	for _, signer := range signers {
		if signer.String() == address {
			senderMatch = true
			break
		}
	}

	if !senderMatch {
		return ctx, zeroneauthtypes.ErrDIDResolutionFailed
	}

	// Emit DID event for indexing
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"did_reference",
			sdk.NewAttribute("did", did),
			sdk.NewAttribute("address", address),
		),
	)

	return next(ctx, tx, simulate)
}

// ---------- ZeroneAccountDecorator ----------

// ZeroneAccountDecorator enforces Zerone-specific account constraints:
// 1. Frozen accounts cannot send transactions
// 2. Updates LastActiveBlock for registered Zerone accounts
//
// Runs AFTER signature verification (signer is already authenticated).
//
// ANTE P0-1 FIX: Uses tx-level signer extraction (getSignerAddresses) instead of
// per-message type assertion. SDK v0.50 proto-generated types (MsgSend, MsgDelegate)
// don't implement GetSigners() []sdk.AccAddress, so the old approach silently skipped
// frozen account checks for all standard Cosmos SDK messages.
type ZeroneAccountDecorator struct {
	zak zeroneauthkeeper.Keeper
}

// NewZeroneAccountDecorator creates a new ZeroneAccountDecorator.
func NewZeroneAccountDecorator(zak zeroneauthkeeper.Keeper) ZeroneAccountDecorator {
	return ZeroneAccountDecorator{zak: zak}
}

func (zad ZeroneAccountDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if simulate {
		return next(ctx, tx, simulate)
	}

	currentHeight := uint64(ctx.BlockHeight())

	// Extract signers from tx signature data (works for ALL message types).
	signers := getSignerAddresses(tx)
	for _, signer := range signers {
		address := signer.String()

		account, found := zad.zak.GetAccount(ctx, address)
		if !found {
			// Not a registered Zerone account — standard Cosmos account, skip
			continue
		}

		// Check frozen status
		if account.Flags != nil && account.Flags.Frozen {
			return ctx, zeroneauthtypes.ErrAccountFrozen
		}

		// Update last active block.
		if account.LastActiveBlock < currentHeight {
			account.LastActiveBlock = currentHeight
			zad.zak.SetAccount(ctx, account)
		}
	}

	return next(ctx, tx, simulate)
}

// ---------- ZeroneCapabilityDecorator ----------

// ZeroneCapabilityDecorator enforces session key capabilities.
// If a tx is signed with a pubkey matching a registered session key,
// the decorator checks that the session key has the required capabilities
// for ALL message types in the tx.
//
// Runs AFTER signature verification and ZeroneAccountDecorator.
//
// ANTE P0-1 FIX: Uses tx-level signer extraction (getTxSigners) to get pubkeys
// directly from signatures, instead of per-message type assertion + AccountKeeper
// lookup. This ensures capability enforcement works for SDK proto-generated types.
type ZeroneCapabilityDecorator struct {
	zak zeroneauthkeeper.Keeper
	ak  AccountKeeperForZerone
}

// AccountKeeperForZerone is the Cosmos AccountKeeper interface needed by capability decorator.
type AccountKeeperForZerone interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}

// NewZeroneCapabilityDecorator creates a new ZeroneCapabilityDecorator.
func NewZeroneCapabilityDecorator(zak zeroneauthkeeper.Keeper, ak AccountKeeperForZerone) ZeroneCapabilityDecorator {
	return ZeroneCapabilityDecorator{zak: zak, ak: ak}
}

func (zcd ZeroneCapabilityDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if simulate {
		return next(ctx, tx, simulate)
	}

	msgs := tx.GetMsgs()
	currentHeight := uint64(ctx.BlockHeight())

	// Extract signers with pubkeys from tx signature data (works for ALL message types).
	signers := getTxSigners(tx)
	for _, signer := range signers {
		address := signer.Address.String()

		// Check if the signing pubkey matches a registered session key
		session := zcd.findSessionByPubKey(ctx, address, signer.PubKeyHex, currentHeight)
		if session != nil {
			// Session key: enforce session capabilities (default-deny)
			for _, msg := range msgs {
				if err := zcd.checkCapability(session, msg); err != nil {
					return ctx, err
				}
			}
		} else {
			// Primary key: enforce account-level capabilities (default-allow)
			for _, msg := range msgs {
				if err := zcd.checkAccountCapability(ctx, address, msg); err != nil {
					return ctx, err
				}
			}
		}
	}

	return next(ctx, tx, simulate)
}

// findSessionByPubKey checks if the given pubkey matches any active session key for the owner.
func (zcd ZeroneCapabilityDecorator) findSessionByPubKey(ctx sdk.Context, owner, pubKeyHex string, height uint64) *zeroneauthtypes.SessionKey {
	sessions := zcd.zak.GetSessionKeysForOwner(ctx, owner)
	for _, session := range sessions {
		if session.ExpiresAtBlock <= height {
			continue // expired
		}
		if session.PublicKey == pubKeyHex {
			return session
		}
	}
	return nil
}

// checkCapability verifies the session key has capability for the given message type.
// Default-deny: unrecognized message types are rejected for session keys.
func (zcd ZeroneCapabilityDecorator) checkCapability(session *zeroneauthtypes.SessionKey, msg sdk.Msg) error {
	if session.Capabilities == nil {
		return zeroneauthtypes.ErrSessionCapabilityDenied
	}
	msgType := sdk.MsgTypeURL(msg)

	switch {
	case isTransferMsg(msgType):
		if !session.Capabilities.CanTransfer {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	case isStakeMsg(msgType):
		if !session.Capabilities.CanStake {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	case isClaimMsg(msgType):
		if !session.Capabilities.CanSubmitClaims {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	case isVoteMsg(msgType):
		if !session.Capabilities.CanVote {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	case isICAMsg(msgType):
		if !session.Capabilities.CanIca {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		// ICA registration requires operational key — session keys can only
		// send transactions to already-registered ICAs.
		if msgType == "/zerone.icaauth.v1.MsgRegisterInterchainAccount" {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	case isPartnershipMsg(msgType):
		if !session.Capabilities.CanPartnership {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	case isResearchMsg(msgType):
		if !session.Capabilities.CanResearch {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	case isDisputeMsg(msgType):
		if !session.Capabilities.CanDispute {
			return zeroneauthtypes.ErrSessionCapabilityDenied
		}
		return nil
	default:
		// Default deny: session keys cannot perform unrecognized message types
		return zeroneauthtypes.ErrSessionCapabilityDenied
	}
}

// checkAccountCapability enforces account-level capabilities for primary key signers.
// Unlike session keys (default-deny), primary keys use default-allow for unrecognized types.
func (zcd ZeroneCapabilityDecorator) checkAccountCapability(ctx sdk.Context, address string, msg sdk.Msg) error {
	msgType := sdk.MsgTypeURL(msg)

	// Auth management messages are always allowed (registration, key rotation, etc.)
	if isAuthManagementMsg(msgType) {
		return nil
	}

	account, found := zcd.zak.GetAccount(ctx, address)
	if !found {
		// Unregistered account: block Zerone-specific ops, allow everything else
		if isZeroneSpecificMsg(msgType) {
			return zeroneauthtypes.ErrAccountCapabilityDenied
		}
		return nil
	}

	return zcd.checkRegisteredAccountCapability(account, msgType)
}

// checkRegisteredAccountCapability enforces capabilities for registered accounts.
// Flag-based capabilities (CanSubmitClaims, CanChallenge) are checked from AccountFlags.
// Type-based restrictions (staking, voting, research, disputes) are derived from account_type.
func (zcd ZeroneCapabilityDecorator) checkRegisteredAccountCapability(account *zeroneauthtypes.Account, msgType string) error {
	flags := account.Flags
	accountType := account.AccountType

	switch {
	case isClaimSubmissionMsg(msgType):
		if flags == nil || !flags.CanSubmitClaims {
			return zeroneauthtypes.ErrAccountCapabilityDenied
		}
		return nil
	case isChallengeMsg(msgType):
		if flags == nil || !flags.CanChallenge {
			return zeroneauthtypes.ErrAccountCapabilityDenied
		}
		return nil
	case isStakeMsg(msgType):
		if accountType == "contract" {
			return zeroneauthtypes.ErrAccountCapabilityDenied
		}
		return nil
	case isVoteMsg(msgType):
		if accountType == "contract" {
			return zeroneauthtypes.ErrAccountCapabilityDenied
		}
		return nil
	case isResearchMsg(msgType):
		if accountType == "contract" {
			return zeroneauthtypes.ErrAccountCapabilityDenied
		}
		return nil
	case isDisputeMsg(msgType):
		if accountType == "contract" {
			return zeroneauthtypes.ErrAccountCapabilityDenied
		}
		return nil
	case isPartnershipMsg(msgType), isTransferMsg(msgType), isICAMsg(msgType):
		// Allowed for all registered account types
		return nil
	default:
		// Default-allow for primary keys (preserves authz/fee grants/etc.)
		return nil
	}
}

func isTransferMsg(msgType string) bool {
	return msgType == "/cosmos.bank.v1beta1.MsgSend" ||
		msgType == "/cosmos.bank.v1beta1.MsgMultiSend" ||
		msgType == "/ibc.applications.transfer.v1.MsgTransfer"
}

func isStakeMsg(msgType string) bool {
	return msgType == "/cosmos.staking.v1beta1.MsgDelegate" ||
		msgType == "/cosmos.staking.v1beta1.MsgUndelegate" ||
		msgType == "/cosmos.staking.v1beta1.MsgBeginRedelegate" ||
		msgType == "/zerone.staking.v1.MsgRegisterValidator"
}

func isClaimSubmissionMsg(msgType string) bool {
	return msgType == "/zerone.knowledge.v1.MsgSubmitClaim" ||
		msgType == "/zerone.knowledge.v1.MsgSubmitCommitment" ||
		msgType == "/zerone.knowledge.v1.MsgSubmitReveal"
}

func isChallengeMsg(msgType string) bool {
	return msgType == "/zerone.knowledge.v1.MsgChallengeFact"
}

func isClaimMsg(msgType string) bool {
	return isClaimSubmissionMsg(msgType) || isChallengeMsg(msgType)
}

// isAuthManagementMsg checks if a message is an auth management operation.
// These are always allowed — accounts must be able to register and manage keys.
func isAuthManagementMsg(msgType string) bool {
	return msgType == "/zerone.auth.v1.MsgRegisterAccount" ||
		msgType == "/zerone.auth.v1.MsgRotateKey" ||
		msgType == "/zerone.auth.v1.MsgCreateSession" ||
		msgType == "/zerone.auth.v1.MsgRevokeSession" ||
		msgType == "/zerone.auth.v1.MsgRecoverAccount" ||
		msgType == "/zerone.auth.v1.MsgFreezeAccount" ||
		msgType == "/zerone.auth.v1.MsgUnfreezeAccount" ||
		msgType == "/zerone.auth.v1.MsgSetRecoveryConfig" ||
		msgType == "/zerone.auth.v1.MsgInitiateRecovery" ||
		msgType == "/zerone.auth.v1.MsgSubmitRecoveryShard" ||
		msgType == "/zerone.auth.v1.MsgChallengeRecovery" ||
		msgType == "/zerone.auth.v1.MsgExecuteRecovery"
}

// isZeroneSpecificMsg checks if a message is a Zerone-specific operation
// that requires registration. Used for unregistered account gating.
func isZeroneSpecificMsg(msgType string) bool {
	return isClaimSubmissionMsg(msgType) ||
		isChallengeMsg(msgType) ||
		isPartnershipMsg(msgType) ||
		isResearchMsg(msgType) ||
		isDisputeMsg(msgType) ||
		isICAMsg(msgType)
}

// isICAMsg checks if a message type URL is an ICA message.
func isICAMsg(msgType string) bool {
	return msgType == "/zerone.icaauth.v1.MsgRegisterInterchainAccount" ||
		msgType == "/zerone.icaauth.v1.MsgSendICATransaction"
}

func isVoteMsg(msgType string) bool {
	return msgType == "/cosmos.gov.v1.MsgVote" ||
		msgType == "/zerone.gov.v1.MsgCastVote" ||
		msgType == "/zerone.emergency.v1.MsgVoteHalt" ||
		msgType == "/zerone.emergency.v1.MsgVoteRevert" ||
		msgType == "/zerone.emergency.v1.MsgVoteResume"
}

func isPartnershipMsg(msgType string) bool {
	return msgType == "/zerone.partnerships.v1.MsgDepositToPot" ||
		msgType == "/zerone.partnerships.v1.MsgProposeOperation" ||
		msgType == "/zerone.partnerships.v1.MsgApproveOperation" ||
		msgType == "/zerone.partnerships.v1.MsgRejectOperation" ||
		msgType == "/zerone.partnerships.v1.MsgSafetyFreeze" ||
		msgType == "/zerone.partnerships.v1.MsgInitiatePartnership" ||
		msgType == "/zerone.partnerships.v1.MsgAcceptPartnership" ||
		msgType == "/zerone.partnerships.v1.MsgInitiateExit" ||
		msgType == "/zerone.partnerships.v1.MsgDistributeReward" ||
		msgType == "/zerone.partnerships.v1.MsgRaiseCoercionSignal" ||
		msgType == "/zerone.partnerships.v1.MsgDeclareIntent" ||
		msgType == "/zerone.partnerships.v1.MsgRegisterInPool" ||
		msgType == "/zerone.partnerships.v1.MsgSponsorPartnership"
}

func isResearchMsg(msgType string) bool {
	return msgType == "/zerone.research.v1.MsgSubmitResearch" ||
		msgType == "/zerone.research.v1.MsgChallengeResearch" ||
		msgType == "/zerone.research.v1.MsgReviewResearch" ||
		msgType == "/zerone.research.v1.MsgResolveResearch" ||
		msgType == "/zerone.research.v1.MsgCreateBounty" ||
		msgType == "/zerone.research.v1.MsgClaimBounty" ||
		msgType == "/zerone.research.v1.MsgFulfillBounty" ||
		msgType == "/zerone.research.v1.MsgFundResearch"
}

func isDisputeMsg(msgType string) bool {
	return msgType == "/zerone.disputes.v1.MsgInitiateDispute" ||
		msgType == "/zerone.disputes.v1.MsgOpenDispute" ||
		msgType == "/zerone.disputes.v1.MsgCommitEvidence" ||
		msgType == "/zerone.disputes.v1.MsgSubmitEvidence" ||
		msgType == "/zerone.disputes.v1.MsgRevealEvidence" ||
		msgType == "/zerone.disputes.v1.MsgVoteDispute" ||
		msgType == "/zerone.disputes.v1.MsgArbiterVote" ||
		msgType == "/zerone.disputes.v1.MsgEscalateDispute"
}

// ---------- Capability Presets ----------

// CapabilityPresets defines common session key capability bundles for agent roles.
var CapabilityPresets = map[string]*zeroneauthtypes.SessionCapabilities{
	"knowledge-worker": {
		CanSubmitClaims: true,
		CanVote:         true,
	},
	"partnership-operator": {
		CanPartnership: true,
	},
	"researcher": {
		CanResearch: true,
	},
	"autonomous-agent": {
		CanSubmitClaims: true,
		CanVote:         true,
		CanPartnership:  true,
		CanResearch:     true,
		CanDispute:      true,
		CanTransfer:     true, // subject to spending limits
	},
	"dispute-arbiter": {
		CanDispute: true,
		CanVote:    true,
	},
}

// ResolvePreset returns the SessionCapabilities for a named preset.
// Returns nil if the preset name is not recognized.
func ResolvePreset(name string) *zeroneauthtypes.SessionCapabilities {
	caps, ok := CapabilityPresets[name]
	if !ok {
		return nil
	}
	return &zeroneauthtypes.SessionCapabilities{
		CanTransfer:     caps.CanTransfer,
		CanStake:        caps.CanStake,
		CanSubmitClaims: caps.CanSubmitClaims,
		CanVote:         caps.CanVote,
		CanIca:          caps.CanIca,
		CanPartnership:  caps.CanPartnership,
		CanResearch:     caps.CanResearch,
		CanDispute:      caps.CanDispute,
	}
}

// ---------- Helpers ----------

func init() {
	// Validate that all gas costs are within limits at startup
	for txType, gas := range TransactionGasCosts {
		if gas > TxGasLimit {
			panic(fmt.Sprintf("gas cost for %s (%d) exceeds tx limit (%d)", txType, gas, TxGasLimit))
		}
	}
}
