package app

import (
	"testing"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---------- lookupMsgGas Tests ----------

func TestLookupMsgGas_KnownTypes(t *testing.T) {
	tests := []struct {
		msgTypeURL string
		wantGas    uint64
	}{
		{"/cosmos.bank.v1beta1.MsgSend", TransactionGasCosts["transfer"]},
		{"/cosmos.bank.v1beta1.MsgMultiSend", TransactionGasCosts["transfer"] * 2},
		{"/zerone.knowledge.v1.MsgSubmitClaim", TransactionGasCosts["claim_submit"]},
		{"/zerone.staking.v1.MsgRegisterValidator", TransactionGasCosts["register_validator"]},
		{"/zerone.bvm.v1.MsgDeployContract", TransactionGasCosts["deploy_contract"]},
		{"/zerone.tree.v1.MsgCreateProject", TransactionGasCosts["create_project"]},
		{"/zerone.tree.v1.MsgPauseProject", TransactionGasCosts["pause_project"]},
		{"/zerone.tree.v1.MsgSubscribeService", TransactionGasCosts["subscribe_service"]},
		{"/zerone.tree.v1.MsgSetAvailability", TransactionGasCosts["set_availability"]},
		{"/zerone.emergency.v1.MsgProposeHalt", TransactionGasCosts["propose_halt"]},
		{"/zerone.toolbox.v1.MsgRegisterTool", TransactionGasCosts["register_tool"]},
		{"/zerone.alignment.v1.MsgActivate", TransactionGasCosts["activate_alignment"]},
		{"/zerone.liquiditypool.v1.MsgCreatePool", TransactionGasCosts["create_pool"]},
	}

	for _, tc := range tests {
		got := lookupMsgGas(tc.msgTypeURL)
		if got != tc.wantGas {
			t.Errorf("lookupMsgGas(%q) = %d, want %d", tc.msgTypeURL, got, tc.wantGas)
		}
	}
}

func TestLookupMsgGas_UnknownTypeFallsBackToMinGas(t *testing.T) {
	unknownTypes := []string{
		"/cosmos.unknown.v1.MsgDoSomething",
		"/zerone.future.v1.MsgNewFeature",
		"",
		"not-a-valid-type-url",
	}

	for _, msgType := range unknownTypes {
		got := lookupMsgGas(msgType)
		if got != MinGasLimit {
			t.Errorf("lookupMsgGas(%q) = %d, want MinGasLimit %d", msgType, got, MinGasLimit)
		}
	}
}

// ---------- Message Type Classification Tests ----------

func TestIsTransferMsg(t *testing.T) {
	trueCases := []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/cosmos.bank.v1beta1.MsgMultiSend",
		"/ibc.applications.transfer.v1.MsgTransfer",
	}
	for _, tc := range trueCases {
		if !isTransferMsg(tc) {
			t.Errorf("isTransferMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/cosmos.staking.v1beta1.MsgDelegate",
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"/cosmos.gov.v1.MsgVote",
		"",
	}
	for _, tc := range falseCases {
		if isTransferMsg(tc) {
			t.Errorf("isTransferMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsStakeMsg(t *testing.T) {
	trueCases := []string{
		"/cosmos.staking.v1beta1.MsgDelegate",
		"/cosmos.staking.v1beta1.MsgUndelegate",
		"/cosmos.staking.v1beta1.MsgBeginRedelegate",
		"/zerone.staking.v1.MsgRegisterValidator",
	}
	for _, tc := range trueCases {
		if !isStakeMsg(tc) {
			t.Errorf("isStakeMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"",
	}
	for _, tc := range falseCases {
		if isStakeMsg(tc) {
			t.Errorf("isStakeMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsClaimMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"/zerone.knowledge.v1.MsgSubmitCommitment",
		"/zerone.knowledge.v1.MsgSubmitReveal",
		"/zerone.knowledge.v1.MsgChallengeFact",
	}
	for _, tc := range trueCases {
		if !isClaimMsg(tc) {
			t.Errorf("isClaimMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/cosmos.gov.v1.MsgVote",
		"",
	}
	for _, tc := range falseCases {
		if isClaimMsg(tc) {
			t.Errorf("isClaimMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsVoteMsg(t *testing.T) {
	trueCases := []string{
		"/cosmos.gov.v1.MsgVote",
		"/zerone.gov.v1.MsgCastVote",
		"/zerone.emergency.v1.MsgVoteHalt",
		"/zerone.emergency.v1.MsgVoteRevert",
		"/zerone.emergency.v1.MsgVoteResume",
	}
	for _, tc := range trueCases {
		if !isVoteMsg(tc) {
			t.Errorf("isVoteMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"",
	}
	for _, tc := range falseCases {
		if isVoteMsg(tc) {
			t.Errorf("isVoteMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsPartnershipMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.partnerships.v1.MsgInitiatePartnership",
		"/zerone.partnerships.v1.MsgDepositToPot",
		"/zerone.partnerships.v1.MsgSafetyFreeze",
	}
	for _, tc := range trueCases {
		if !isPartnershipMsg(tc) {
			t.Errorf("isPartnershipMsg(%q) = false, want true", tc)
		}
	}
}

func TestIsResearchMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.research.v1.MsgSubmitResearch",
		"/zerone.research.v1.MsgCreateBounty",
		"/zerone.research.v1.MsgFundResearch",
	}
	for _, tc := range trueCases {
		if !isResearchMsg(tc) {
			t.Errorf("isResearchMsg(%q) = false, want true", tc)
		}
	}
}

func TestIsDisputeMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.disputes.v1.MsgInitiateDispute",
		"/zerone.disputes.v1.MsgOpenDispute",
		"/zerone.disputes.v1.MsgArbiterVote",
	}
	for _, tc := range trueCases {
		if !isDisputeMsg(tc) {
			t.Errorf("isDisputeMsg(%q) = false, want true", tc)
		}
	}
}

func TestIsICAMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.icaauth.v1.MsgRegisterInterchainAccount",
		"/zerone.icaauth.v1.MsgSendICATransaction",
	}
	for _, tc := range trueCases {
		if !isICAMsg(tc) {
			t.Errorf("isICAMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/zerone.knowledge.v1.MsgSubmitClaim",
	}
	for _, tc := range falseCases {
		if isICAMsg(tc) {
			t.Errorf("isICAMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsClaimSubmissionMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"/zerone.knowledge.v1.MsgSubmitCommitment",
		"/zerone.knowledge.v1.MsgSubmitReveal",
	}
	for _, tc := range trueCases {
		if !isClaimSubmissionMsg(tc) {
			t.Errorf("isClaimSubmissionMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/zerone.knowledge.v1.MsgChallengeFact",
		"/cosmos.bank.v1beta1.MsgSend",
		"/cosmos.gov.v1.MsgVote",
		"",
	}
	for _, tc := range falseCases {
		if isClaimSubmissionMsg(tc) {
			t.Errorf("isClaimSubmissionMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsChallengeMsg(t *testing.T) {
	if !isChallengeMsg("/zerone.knowledge.v1.MsgChallengeFact") {
		t.Error("isChallengeMsg(MsgChallengeFact) = false, want true")
	}

	falseCases := []string{
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"/zerone.knowledge.v1.MsgSubmitCommitment",
		"/cosmos.bank.v1beta1.MsgSend",
		"",
	}
	for _, tc := range falseCases {
		if isChallengeMsg(tc) {
			t.Errorf("isChallengeMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsClaimMsg_BackwardCompatible(t *testing.T) {
	// isClaimMsg must still return true for all 4 original messages
	allClaims := []string{
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"/zerone.knowledge.v1.MsgSubmitCommitment",
		"/zerone.knowledge.v1.MsgSubmitReveal",
		"/zerone.knowledge.v1.MsgChallengeFact",
	}
	for _, tc := range allClaims {
		if !isClaimMsg(tc) {
			t.Errorf("isClaimMsg(%q) = false, want true (backward compat)", tc)
		}
	}
}

func TestIsAuthManagementMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.auth.v1.MsgRegisterAccount",
		"/zerone.auth.v1.MsgRotateKey",
		"/zerone.auth.v1.MsgCreateSession",
		"/zerone.auth.v1.MsgRevokeSession",
		"/zerone.auth.v1.MsgRecoverAccount",
		"/zerone.auth.v1.MsgFreezeAccount",
		"/zerone.auth.v1.MsgUnfreezeAccount",
		"/zerone.auth.v1.MsgSetRecoveryConfig",
		"/zerone.auth.v1.MsgInitiateRecovery",
		"/zerone.auth.v1.MsgSubmitRecoveryShard",
		"/zerone.auth.v1.MsgChallengeRecovery",
		"/zerone.auth.v1.MsgExecuteRecovery",
	}
	for _, tc := range trueCases {
		if !isAuthManagementMsg(tc) {
			t.Errorf("isAuthManagementMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"/cosmos.staking.v1beta1.MsgDelegate",
		"",
	}
	for _, tc := range falseCases {
		if isAuthManagementMsg(tc) {
			t.Errorf("isAuthManagementMsg(%q) = true, want false", tc)
		}
	}
}

func TestIsZeroneSpecificMsg(t *testing.T) {
	trueCases := []string{
		"/zerone.knowledge.v1.MsgSubmitClaim",
		"/zerone.knowledge.v1.MsgChallengeFact",
		"/zerone.partnerships.v1.MsgInitiatePartnership",
		"/zerone.research.v1.MsgSubmitResearch",
		"/zerone.disputes.v1.MsgInitiateDispute",
		"/zerone.icaauth.v1.MsgRegisterInterchainAccount",
	}
	for _, tc := range trueCases {
		if !isZeroneSpecificMsg(tc) {
			t.Errorf("isZeroneSpecificMsg(%q) = false, want true", tc)
		}
	}

	falseCases := []string{
		"/cosmos.bank.v1beta1.MsgSend",
		"/cosmos.staking.v1beta1.MsgDelegate",
		"/cosmos.gov.v1.MsgVote",
		"/zerone.auth.v1.MsgRegisterAccount",
		"",
	}
	for _, tc := range falseCases {
		if isZeroneSpecificMsg(tc) {
			t.Errorf("isZeroneSpecificMsg(%q) = true, want false", tc)
		}
	}
}

// ---------- Gas Constants Validation ----------

func TestGasConstantsInvariant(t *testing.T) {
	// MinGasLimit < TxGasLimit < BlockGasLimit
	if MinGasLimit >= TxGasLimit {
		t.Errorf("MinGasLimit (%d) should be < TxGasLimit (%d)", MinGasLimit, TxGasLimit)
	}
	if TxGasLimit >= BlockGasLimit {
		t.Errorf("TxGasLimit (%d) should be < BlockGasLimit (%d)", TxGasLimit, BlockGasLimit)
	}
}

func TestAllGasCostsBelowTxLimit(t *testing.T) {
	for txType, gas := range TransactionGasCosts {
		if gas > TxGasLimit {
			t.Errorf("TransactionGasCosts[%q] = %d exceeds TxGasLimit %d", txType, gas, TxGasLimit)
		}
	}
}

func TestFeeRoutingBPSSumTo1000000(t *testing.T) {
	if ResearchContributionBPS+ValidatorFeeBPS != 1000000 {
		t.Errorf("ResearchContributionBPS (%d) + ValidatorFeeBPS (%d) = %d, want 1000000",
			ResearchContributionBPS, ValidatorFeeBPS, ResearchContributionBPS+ValidatorFeeBPS)
	}
}

// ---------- EstimateTransactionGas Tests ----------

func TestEstimateTransactionGas_KnownType(t *testing.T) {
	got := EstimateTransactionGas("transfer")
	if got != 21_000 {
		t.Errorf("EstimateTransactionGas(transfer) = %d, want 21000", got)
	}
}

func TestEstimateTransactionGas_UnknownType(t *testing.T) {
	got := EstimateTransactionGas("nonexistent_type")
	if got != MinGasLimit {
		t.Errorf("EstimateTransactionGas(nonexistent_type) = %d, want %d", got, MinGasLimit)
	}
}

func TestIsSystemTransaction(t *testing.T) {
	sysTxs := []string{
		"verification_reward", "slash_validator", "epoch_transition",
		"emergency_halt", "emergency_revert", "emergency_resume", "block_reward",
	}
	for _, tx := range sysTxs {
		if !IsSystemTransaction(tx) {
			t.Errorf("IsSystemTransaction(%q) = false, want true", tx)
		}
	}

	nonSysTxs := []string{"transfer", "stake", "claim_submit", ""}
	for _, tx := range nonSysTxs {
		if IsSystemTransaction(tx) {
			t.Errorf("IsSystemTransaction(%q) = true, want false", tx)
		}
	}
}

// ---------- GetMinimumFee Tests ----------

// mockMsg implements sdk.Msg for testing gas calculations.
type mockMsg struct {
	typeURL string
}

func (m mockMsg) ProtoMessage()        {}
func (m mockMsg) Reset()               {}
func (m mockMsg) String() string       { return m.typeURL }
func (m mockMsg) ValidateBasic() error { return nil }

func (m mockMsg) XXX_MessageName() string { return m.typeURL[1:] } // strip leading /

func TestGetMinimumFee_SingleMessage(t *testing.T) {
	// MsgSend -> transfer -> 21,000 gas -> clamped to MinGasLimit 22,222
	msgs := []sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}}
	fee := GetMinimumFee(msgs)

	expectedGas := uint64(22_222) // MinGasLimit since 21,000 < 22,222
	expectedFee := math.NewIntFromUint64(expectedGas * MinGasPrice)

	if !fee.AmountOf(BondDenom).Equal(expectedFee) {
		t.Errorf("GetMinimumFee for MsgSend = %s, want %s uzrn", fee, expectedFee)
	}
}

func TestGetMinimumFee_MultipleMessages(t *testing.T) {
	// 3 transfers: 21,000 * 3 = 63,000 gas
	msgs := []sdk.Msg{
		mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"},
		mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"},
		mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"},
	}
	fee := GetMinimumFee(msgs)

	expectedGas := uint64(63_000) // 21,000 * 3 > MinGasLimit
	expectedFee := math.NewIntFromUint64(expectedGas * MinGasPrice)

	if !fee.AmountOf(BondDenom).Equal(expectedFee) {
		t.Errorf("GetMinimumFee for 3x MsgSend = %s, want %s uzrn", fee, expectedFee)
	}
}

func TestGetMinimumFee_ExpensiveMessage(t *testing.T) {
	// deploy_contract = 200,000 gas
	msgs := []sdk.Msg{mockMsg{typeURL: "/zerone.bvm.v1.MsgDeployContract"}}
	fee := GetMinimumFee(msgs)

	expectedGas := uint64(200_000)
	expectedFee := math.NewIntFromUint64(expectedGas * MinGasPrice)

	if !fee.AmountOf(BondDenom).Equal(expectedFee) {
		t.Errorf("GetMinimumFee for MsgDeployContract = %s, want %s uzrn", fee, expectedFee)
	}
}

// ---------- Proto URL Coverage Tests ----------

func TestMsgTypeURLToGas_AllEntriesReferenceValidCosts(t *testing.T) {
	for url, gas := range msgTypeURLToGas {
		if gas == 0 {
			t.Errorf("msgTypeURLToGas[%q] = 0, all mapped messages should have non-zero gas", url)
		}
	}
}

func TestMsgTypeURLToGas_TreeModuleComplete(t *testing.T) {
	treeMessages := []string{
		"/zerone.tree.v1.MsgCreateProject",
		"/zerone.tree.v1.MsgProposeProject",
		"/zerone.tree.v1.MsgStartDevelopment",
		"/zerone.tree.v1.MsgCompleteProject",
		"/zerone.tree.v1.MsgPauseProject",
		"/zerone.tree.v1.MsgResumeProject",
		"/zerone.tree.v1.MsgAbandonProject",
		"/zerone.tree.v1.MsgSpawnChildProject",
		"/zerone.tree.v1.MsgAddTask",
		"/zerone.tree.v1.MsgAssignTask",
		"/zerone.tree.v1.MsgStartWork",
		"/zerone.tree.v1.MsgSubmitDeliverable",
		"/zerone.tree.v1.MsgApproveDeliverable",
		"/zerone.tree.v1.MsgRejectDeliverable",
		"/zerone.tree.v1.MsgReopenTask",
		"/zerone.tree.v1.MsgDeployService",
		"/zerone.tree.v1.MsgCallService",
		"/zerone.tree.v1.MsgSubscribeService",
		"/zerone.tree.v1.MsgPauseService",
		"/zerone.tree.v1.MsgResumeService",
		"/zerone.tree.v1.MsgRetireService",
		"/zerone.tree.v1.MsgBeginSeeding",
		"/zerone.tree.v1.MsgDetectOpportunity",
		"/zerone.tree.v1.MsgClaimOpportunity",
		"/zerone.tree.v1.MsgAddContributor",
		"/zerone.tree.v1.MsgApplyToProject",
		"/zerone.tree.v1.MsgReviewApplication",
		"/zerone.tree.v1.MsgSetAvailability",
	}

	for _, msg := range treeMessages {
		if _, ok := msgTypeURLToGas[msg]; !ok {
			t.Errorf("missing gas mapping for tree message %q", msg)
		}
	}
}

func TestMsgTypeURLToGas_ToolboxModuleComplete(t *testing.T) {
	toolboxMessages := []string{
		"/zerone.toolbox.v1.MsgRegisterTool",
		"/zerone.toolbox.v1.MsgCallTool",
		"/zerone.toolbox.v1.MsgAddContributor",
		"/zerone.toolbox.v1.MsgAcceptContributorship",
		"/zerone.toolbox.v1.MsgUpgradeTool",
		"/zerone.toolbox.v1.MsgDeprecateTool",
		"/zerone.toolbox.v1.MsgRetireTool",
		"/zerone.toolbox.v1.MsgLockShares",
		"/zerone.toolbox.v1.MsgUpdateDependency",
		"/zerone.toolbox.v1.MsgToolHeartbeat",
	}

	for _, msg := range toolboxMessages {
		if _, ok := msgTypeURLToGas[msg]; !ok {
			t.Errorf("missing gas mapping for toolbox message %q", msg)
		}
	}
}

// ---------- ZRNGasDecorator Tests ----------

// mockFeeTx implements sdk.FeeTx for testing.
type mockFeeTx struct {
	sdk.Tx
	gas  uint64
	fee  sdk.Coins
	msgs []sdk.Msg
}

func (m mockFeeTx) GetGas() uint64     { return m.gas }
func (m mockFeeTx) GetFee() sdk.Coins  { return m.fee }
func (m mockFeeTx) GetMsgs() []sdk.Msg { return m.msgs }
func (m mockFeeTx) FeeGranter() []byte { return nil }
func (m mockFeeTx) FeePayer() []byte   { return nil }

// passThroughHandler is a no-op next handler for testing decorators.
func passThroughHandler(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
	return ctx, nil
}

func TestZRNGasDecorator_SimulateSkips(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}
	tx := mockFeeTx{gas: 0} // Would fail if not simulating

	_, err := decorator.AnteHandle(ctx, tx, true, passThroughHandler)
	if err != nil {
		t.Errorf("simulate mode should skip validation, got error: %v", err)
	}
}

func TestZRNGasDecorator_ExceedsBlockGasLimit(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}
	tx := mockFeeTx{
		gas:  BlockGasLimit + 1,
		fee:  sdk.NewCoins(sdk.NewCoin(BondDenom, math.NewInt(999999999))),
		msgs: []sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
	}

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Error("expected error for gas exceeding block limit")
	}
}

func TestZRNGasDecorator_ExceedsTxGasLimit(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}
	tx := mockFeeTx{
		gas:  TxGasLimit + 1,
		fee:  sdk.NewCoins(sdk.NewCoin(BondDenom, math.NewInt(999999999))),
		msgs: []sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
	}

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Error("expected error for gas exceeding tx limit")
	}
}

func TestZRNGasDecorator_InsufficientGas(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}
	// deploy_contract requires 200,000 gas, provide only 1000
	tx := mockFeeTx{
		gas:  1000,
		fee:  sdk.NewCoins(sdk.NewCoin(BondDenom, math.NewInt(999999999))),
		msgs: []sdk.Msg{mockMsg{typeURL: "/zerone.bvm.v1.MsgDeployContract"}},
	}

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Error("expected error for insufficient gas")
	}
}

func TestZRNGasDecorator_SufficientGas(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}
	// transfer = 21,000 but MinGasLimit = 22,222; provide 30,000
	tx := mockFeeTx{
		gas:  30_000,
		fee:  sdk.NewCoins(sdk.NewCoin(BondDenom, math.NewInt(30_000))),
		msgs: []sdk.Msg{mockMsg{typeURL: "/cosmos.bank.v1beta1.MsgSend"}},
	}

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err != nil {
		t.Errorf("expected no error for sufficient gas, got: %v", err)
	}
}

func TestZRNGasDecorator_InsufficientFee(t *testing.T) {
	decorator := NewZRNGasDecorator()
	ctx := sdk.Context{}
	// Gas = 100,000, MinGasPrice = 1, so min fee = 100,000 uzrn. Provide only 1.
	tx := mockFeeTx{
		gas:  100_000,
		fee:  sdk.NewCoins(sdk.NewCoin(BondDenom, math.NewInt(1))),
		msgs: []sdk.Msg{mockMsg{typeURL: "/zerone.knowledge.v1.MsgSubmitClaim"}},
	}

	_, err := decorator.AnteHandle(ctx, tx, false, passThroughHandler)
	if err == nil {
		t.Error("expected error for insufficient fee")
	}
}

// ---------- Capability Preset Tests ----------

func TestResolvePreset_Known(t *testing.T) {
	presets := []string{"knowledge-worker", "partnership-operator", "researcher", "autonomous-agent", "dispute-arbiter"}
	for _, name := range presets {
		caps := ResolvePreset(name)
		if caps == nil {
			t.Errorf("ResolvePreset(%q) = nil, want non-nil", name)
		}
	}
}

func TestResolvePreset_Unknown(t *testing.T) {
	caps := ResolvePreset("nonexistent")
	if caps != nil {
		t.Error("ResolvePreset(nonexistent) should return nil")
	}
}

func TestResolvePreset_ReturnsCopy(t *testing.T) {
	caps1 := ResolvePreset("knowledge-worker")
	caps2 := ResolvePreset("knowledge-worker")
	if caps1 == caps2 {
		t.Error("ResolvePreset should return a copy, not the same pointer")
	}
}

func TestResolvePreset_KnowledgeWorker(t *testing.T) {
	caps := ResolvePreset("knowledge-worker")
	if !caps.CanSubmitClaims {
		t.Error("knowledge-worker should have CanSubmitClaims")
	}
	if !caps.CanVote {
		t.Error("knowledge-worker should have CanVote")
	}
	if caps.CanTransfer {
		t.Error("knowledge-worker should not have CanTransfer")
	}
}

func TestResolvePreset_AutonomousAgent(t *testing.T) {
	caps := ResolvePreset("autonomous-agent")
	if !caps.CanSubmitClaims || !caps.CanVote || !caps.CanPartnership || !caps.CanResearch || !caps.CanDispute || !caps.CanTransfer {
		t.Error("autonomous-agent should have full operational capabilities")
	}
	if caps.CanIca || caps.CanStake {
		t.Error("autonomous-agent should not have CanIca or CanStake")
	}
}
