package simulation

import (
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// Operation weight constants.
const (
	OpWeightMsgSubmitData    = "op_weight_msg_submit_data"
	OpWeightMsgSubmitThread  = "op_weight_msg_submit_thread"
	OpWeightMsgContestSample = "op_weight_msg_contest_sample"
	OpWeightMsgAccessSample  = "op_weight_msg_access_sample"
	OpWeightMsgSponsorSample = "op_weight_msg_sponsor_sample"
	OpWeightMsgRevokeConsent = "op_weight_msg_revoke_consent"
	OpWeightMsgFundBounty    = "op_weight_msg_fund_bounty"

	DefaultWeightMsgSubmitData    = 100
	DefaultWeightMsgSubmitThread  = 30
	DefaultWeightMsgContestSample = 10
	DefaultWeightMsgAccessSample  = 50
	DefaultWeightMsgSponsorSample = 15
	DefaultWeightMsgRevokeConsent = 5
	DefaultWeightMsgFundBounty    = 10
)

var (
	simLanguages   = []string{"en", "es", "zh", "ja", "de", "fr", "ar"}
	simDomains     = []string{"science", "mathematics", "history", "philosophy", "technology", "literature"}
	simConsentTypes = []types.ConsentType{
		types.ConsentType_CONSENT_TYPE_SELF_AUTHORED,
		types.ConsentType_CONSENT_TYPE_OPT_IN,
		types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE,
		types.ConsentType_CONSENT_TYPE_PLATFORM_TOS,
	}
	simSampleTypes = []types.SampleType{
		types.SampleType_SAMPLE_TYPE_DISCUSSION,
		types.SampleType_SAMPLE_TYPE_EXPLANATION,
		types.SampleType_SAMPLE_TYPE_DEBATE,
		types.SampleType_SAMPLE_TYPE_TUTORIAL,
	}
	simContestTypes = []types.ContestType{
		types.ContestType_CONTEST_TYPE_QUALITY,
		types.ContestType_CONTEST_TYPE_CONSENT,
		types.ContestType_CONTEST_TYPE_DUPLICATE,
	}
)

// WeightedOperations returns knowledge module simulation operations.
func WeightedOperations(
	appParams simtypes.AppParams,
	_ codec.JSONCodec,
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simulation.WeightedOperations {
	var (
		weightSubmitData    int
		weightSubmitThread  int
		weightContestSample int
		weightAccessSample  int
		weightSponsorSample int
		weightRevokeConsent int
		weightFundBounty    int
	)

	appParams.GetOrGenerate(OpWeightMsgSubmitData, &weightSubmitData, nil, func(_ *rand.Rand) {
		weightSubmitData = DefaultWeightMsgSubmitData
	})
	appParams.GetOrGenerate(OpWeightMsgSubmitThread, &weightSubmitThread, nil, func(_ *rand.Rand) {
		weightSubmitThread = DefaultWeightMsgSubmitThread
	})
	appParams.GetOrGenerate(OpWeightMsgContestSample, &weightContestSample, nil, func(_ *rand.Rand) {
		weightContestSample = DefaultWeightMsgContestSample
	})
	appParams.GetOrGenerate(OpWeightMsgAccessSample, &weightAccessSample, nil, func(_ *rand.Rand) {
		weightAccessSample = DefaultWeightMsgAccessSample
	})
	appParams.GetOrGenerate(OpWeightMsgSponsorSample, &weightSponsorSample, nil, func(_ *rand.Rand) {
		weightSponsorSample = DefaultWeightMsgSponsorSample
	})
	appParams.GetOrGenerate(OpWeightMsgRevokeConsent, &weightRevokeConsent, nil, func(_ *rand.Rand) {
		weightRevokeConsent = DefaultWeightMsgRevokeConsent
	})
	appParams.GetOrGenerate(OpWeightMsgFundBounty, &weightFundBounty, nil, func(_ *rand.Rand) {
		weightFundBounty = DefaultWeightMsgFundBounty
	})

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(weightSubmitData, SimulateMsgSubmitData(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightSubmitThread, SimulateMsgSubmitThread(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightContestSample, SimulateMsgContestSample(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightAccessSample, SimulateMsgAccessSample(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightSponsorSample, SimulateMsgSponsorSample(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightRevokeConsent, SimulateMsgRevokeConsent(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightFundBounty, SimulateMsgFundBounty(txGen, ak, bk, k)),
	}
}

// randomContent generates random content with 100-5000 chars.
func randomContent(r *rand.Rand) string {
	length := 100 + r.Intn(4901)
	b := make([]byte, length)
	for i := range b {
		b[i] = byte('a' + r.Intn(26))
		if r.Intn(5) == 0 {
			b[i] = ' '
		}
	}
	return string(b)
}

func noOp(moduleName, msgName, reason string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
	return simtypes.NoOpMsg(moduleName, msgName, reason), nil, nil
}

// SimulateMsgSubmitData submits random training data.
func SimulateMsgSubmitData(
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		content := randomContent(r)
		domain := simDomains[r.Intn(len(simDomains))]
		lang := simLanguages[r.Intn(len(simLanguages))]
		consentType := simConsentTypes[r.Intn(len(simConsentTypes))]
		sampleType := simSampleTypes[r.Intn(len(simSampleTypes))]

		msg := &types.MsgSubmitData{
			Submitter:  simAccount.Address.String(),
			Content:    content,
			Domain:     domain,
			Language:   lang,
			SampleType: sampleType,
			Consent: &types.ConsentProof{
				Type: consentType,
			},
		}

		msgURL := sdk.MsgTypeURL(msg)

		// Attempt direct state submission (sim doesn't need full tx delivery)
		_, err := k.SubmitData(ctx, msg)
		if err != nil {
			return noOp(types.ModuleName, msgURL, fmt.Sprintf("submit failed: %v", err))
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

// SimulateMsgSubmitThread submits a random thread of 2-8 messages.
func SimulateMsgSubmitThread(
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		itemCount := 2 + r.Intn(7)
		threadID := fmt.Sprintf("thread-%d-%d", ctx.BlockHeight(), r.Intn(1000))

		items := make([]*types.MsgSubmitData, itemCount)
		for i := range items {
			speaker := accs[r.Intn(len(accs))].Address.String()
			items[i] = &types.MsgSubmitData{
				Submitter: speaker,
				Content:   randomContent(r),
				Domain:    simDomains[r.Intn(len(simDomains))],
				ThreadId:  threadID,
			}
		}

		msg := &types.MsgSubmitThread{
			Submitter: simAccount.Address.String(),
			Domain:    simDomains[r.Intn(len(simDomains))],
			ThreadId:  threadID,
			Items:     items,
		}

		msgURL := sdk.MsgTypeURL(msg)

		_, err := k.SubmitThread(ctx, msg)
		if err != nil {
			return noOp(types.ModuleName, msgURL, fmt.Sprintf("thread submit failed: %v", err))
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

// SimulateMsgContestSample contests a random active sample.
func SimulateMsgContestSample(
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msgURL := sdk.MsgTypeURL(&types.MsgContestSample{})

		// Find a sample to contest
		var sampleID string
		k.IterateSamples(ctx, func(s *types.Sample) bool {
			if s.Status == types.SampleStatus_SAMPLE_STATUS_GOLD ||
				s.Status == types.SampleStatus_SAMPLE_STATUS_SILVER ||
				s.Status == types.SampleStatus_SAMPLE_STATUS_BRONZE {
				if r.Intn(10) == 0 { // 10% chance to pick this one
					sampleID = s.Id
					return true
				}
			}
			return false
		})

		if sampleID == "" {
			return noOp(types.ModuleName, msgURL, "no contestable sample found")
		}

		msg := &types.MsgContestSample{
			Challenger:  simAccount.Address.String(),
			SampleId:    sampleID,
			Reason:      "simulation contest",
			ContestType: simContestTypes[r.Intn(len(simContestTypes))],
		}

		_, err := k.ContestSample(ctx, msg)
		if err != nil {
			return noOp(types.ModuleName, msgURL, fmt.Sprintf("contest failed: %v", err))
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

// SimulateMsgAccessSample accesses a random active sample.
func SimulateMsgAccessSample(
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msgURL := sdk.MsgTypeURL(&types.MsgAccessSample{})

		var sampleID string
		k.IterateSamples(ctx, func(s *types.Sample) bool {
			if s.Status == types.SampleStatus_SAMPLE_STATUS_GOLD ||
				s.Status == types.SampleStatus_SAMPLE_STATUS_SILVER {
				if r.Intn(5) == 0 {
					sampleID = s.Id
					return true
				}
			}
			return false
		})

		if sampleID == "" {
			return noOp(types.ModuleName, msgURL, "no accessible sample found")
		}

		msg := &types.MsgAccessSample{
			Consumer: simAccount.Address.String(),
			SampleId: sampleID,
		}

		_, err := k.AccessSample(ctx, msg)
		if err != nil {
			return noOp(types.ModuleName, msgURL, fmt.Sprintf("access failed: %v", err))
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

// SimulateMsgSponsorSample sponsors a random sample with a random amount.
func SimulateMsgSponsorSample(
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		msgURL := sdk.MsgTypeURL(&types.MsgSponsorSample{})

		var sampleID string
		k.IterateSamples(ctx, func(s *types.Sample) bool {
			if s.Status != types.SampleStatus_SAMPLE_STATUS_PRUNED &&
				s.Status != types.SampleStatus_SAMPLE_STATUS_EXPIRED {
				if r.Intn(5) == 0 {
					sampleID = s.Id
					return true
				}
			}
			return false
		})

		if sampleID == "" {
			return noOp(types.ModuleName, msgURL, "no sponsorable sample found")
		}

		amount := fmt.Sprintf("%d", 1000+r.Intn(1_000_000))

		msg := &types.MsgSponsorSample{
			Sponsor:  simAccount.Address.String(),
			SampleId: sampleID,
			Amount:   amount,
		}

		_, err := k.SponsorSample(ctx, msg)
		if err != nil {
			return noOp(types.ModuleName, msgURL, fmt.Sprintf("sponsor failed: %v", err))
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

// SimulateMsgRevokeConsent revokes consent for a random sample owned by the caller.
func SimulateMsgRevokeConsent(
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		addr := simAccount.Address.String()
		msgURL := sdk.MsgTypeURL(&types.MsgRevokeConsent{})

		var sampleID string
		k.IterateSamples(ctx, func(s *types.Sample) bool {
			if (s.Submitter == addr || s.OriginalAuthor == addr) &&
				s.Status != types.SampleStatus_SAMPLE_STATUS_PRUNED {
				sampleID = s.Id
				return true
			}
			return false
		})

		if sampleID == "" {
			return noOp(types.ModuleName, msgURL, "no owned sample to revoke")
		}

		msg := &types.MsgRevokeConsent{
			Requester: addr,
			SampleId:  sampleID,
			Reason:    "simulation revocation",
		}

		err := k.RevokeConsent(ctx, msg)
		if err != nil {
			return noOp(types.ModuleName, msgURL, fmt.Sprintf("revoke failed: %v", err))
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}

// SimulateMsgFundBounty funds a data bounty in a random domain.
func SimulateMsgFundBounty(
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		domain := simDomains[r.Intn(len(simDomains))]
		amount := fmt.Sprintf("%d", 10000+r.Intn(10_000_000))

		msg := &types.MsgFundBounty{
			Funder: simAccount.Address.String(),
			Domain: domain,
			Amount: amount,
		}

		msgURL := sdk.MsgTypeURL(msg)

		_, err := k.FundBounty(ctx, msg)
		if err != nil {
			return noOp(types.ModuleName, msgURL, fmt.Sprintf("fund bounty failed: %v", err))
		}

		return simtypes.NewOperationMsg(msg, true, ""), nil, nil
	}
}
