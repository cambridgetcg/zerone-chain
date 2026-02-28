package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stretchr/testify/require"
	"github.com/zerone-chain/zerone/x/capture_defense/keeper"
	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// ---------- Mock KnowledgeKeeper ----------

type mockKnowledgeKeeper struct {
	domains    map[string]string // factId -> domain
	submitters map[string]string // factId -> submitter
}

func newMockKnowledgeKeeper() *mockKnowledgeKeeper {
	return &mockKnowledgeKeeper{
		domains:    make(map[string]string),
		submitters: make(map[string]string),
	}
}

func (m *mockKnowledgeKeeper) addFact(factId, domain, submitter string) {
	m.domains[factId] = domain
	m.submitters[factId] = submitter
}

func (m *mockKnowledgeKeeper) GetFactDomain(_ context.Context, factId string) (string, bool) {
	d, ok := m.domains[factId]
	return d, ok
}

func (m *mockKnowledgeKeeper) GetFactSubmitter(_ context.Context, factId string) (string, bool) {
	s, ok := m.submitters[factId]
	return s, ok
}

func (m *mockKnowledgeKeeper) GetDomainVerificationActivity(_ context.Context, _ string) uint64 {
	return 0
}

// ---------- Mock StakingKeeper ----------

type mockStakingKeeper struct {
	validators map[string]bool
	stakes     map[string]string
}

func newMockStakingKeeper() *mockStakingKeeper {
	return &mockStakingKeeper{
		validators: make(map[string]bool),
		stakes:     make(map[string]string),
	}
}

func (m *mockStakingKeeper) addValidator(addr string, stake string) {
	m.validators[addr] = true
	m.stakes[addr] = stake
}

func (m *mockStakingKeeper) IsActiveValidator(_ context.Context, valAddr string) (bool, error) {
	return m.validators[valAddr], nil
}

func (m *mockStakingKeeper) GetValidatorStake(_ context.Context, valAddr string) (string, error) {
	s, ok := m.stakes[valAddr]
	if !ok {
		return "0", nil
	}
	return s, nil
}

// ---------- Test Setup ----------

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	authority := sdk.AccAddress([]byte("authority-addr------")).String()
	k := keeper.NewKeeper(runtime.NewKVStoreService(storeKey), cdc, authority)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	return k, ctx
}

func testAddr(i int) string {
	addr := sdk.AccAddress([]byte(fmt.Sprintf("test-addr-%010d", i)))
	return addr.String()
}

// ---------- Tests: Params ----------

func TestParamsDefault(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := k.GetParams(ctx)
	if params.DecayEpochBlocks != 10000 {
		t.Errorf("expected DecayEpochBlocks 10000, got %d", params.DecayEpochBlocks)
	}
	if params.HhiThreshold != 250000 {
		t.Errorf("expected HhiThreshold 250000, got %d", params.HhiThreshold)
	}
	if params.BaseReputationScore != 500000 {
		t.Errorf("expected BaseReputationScore 500000, got %d", params.BaseReputationScore)
	}
	if params.MinVerificationsForScore != 5 {
		t.Errorf("expected MinVerificationsForScore 5, got %d", params.MinVerificationsForScore)
	}
	if params.RiskAnalysisInterval != 1000 {
		t.Errorf("expected RiskAnalysisInterval 1000, got %d", params.RiskAnalysisInterval)
	}
}

func TestParamsSetGet(t *testing.T) {
	k, ctx := setupKeeper(t)
	custom := types.DefaultParams()
	custom.DecayEpochBlocks = 20000
	custom.HhiThreshold = 300000
	k.SetParams(ctx, custom)

	got := k.GetParams(ctx)
	if got.DecayEpochBlocks != 20000 {
		t.Errorf("expected DecayEpochBlocks 20000, got %d", got.DecayEpochBlocks)
	}
	if got.HhiThreshold != 300000 {
		t.Errorf("expected HhiThreshold 300000, got %d", got.HhiThreshold)
	}
}

// ---------- Tests: Reputation CRUD ----------

func TestReputationCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)
	validator := testAddr(1)

	// Global
	_, found := k.GetGlobalReputation(ctx, validator)
	if found {
		t.Error("expected global reputation not found")
	}

	gr := &types.GlobalReputation{
		Validator:          validator,
		Score:              600000,
		TotalVerifications: 10,
		LastUpdatedBlock:   50,
	}
	k.SetGlobalReputation(ctx, gr)

	got, found := k.GetGlobalReputation(ctx, validator)
	if !found {
		t.Fatal("expected global reputation to be found")
	}
	if got.Score != 600000 {
		t.Errorf("expected score 600000, got %d", got.Score)
	}
	if got.TotalVerifications != 10 {
		t.Errorf("expected 10 verifications, got %d", got.TotalVerifications)
	}

	k.DeleteGlobalReputation(ctx, validator)
	_, found = k.GetGlobalReputation(ctx, validator)
	if found {
		t.Error("expected global reputation to be deleted")
	}

	// Stratum
	sr := &types.StratumReputation{
		Validator:        validator,
		Stratum:          "empirical",
		Score:            700000,
		Verifications:    5,
		LastUpdatedBlock: 60,
	}
	k.SetStratumReputation(ctx, sr)

	gotSR, found := k.GetStratumReputation(ctx, "empirical", validator)
	if !found {
		t.Fatal("expected stratum reputation to be found")
	}
	if gotSR.Score != 700000 {
		t.Errorf("expected score 700000, got %d", gotSR.Score)
	}

	k.DeleteStratumReputation(ctx, "empirical", validator)
	_, found = k.GetStratumReputation(ctx, "empirical", validator)
	if found {
		t.Error("expected stratum reputation to be deleted")
	}

	// Domain
	dr := &types.DomainReputation{
		Validator:        validator,
		Domain:           "mathematics",
		Score:            800000,
		Verifications:    15,
		LastUpdatedBlock: 70,
	}
	k.SetDomainReputation(ctx, dr)

	gotDR, found := k.GetDomainReputation(ctx, "mathematics", validator)
	if !found {
		t.Fatal("expected domain reputation to be found")
	}
	if gotDR.Score != 800000 {
		t.Errorf("expected score 800000, got %d", gotDR.Score)
	}

	k.DeleteDomainReputation(ctx, "mathematics", validator)
	_, found = k.GetDomainReputation(ctx, "mathematics", validator)
	if found {
		t.Error("expected domain reputation to be deleted")
	}
}

func TestGetAllReputations(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i := 0; i < 3; i++ {
		k.SetGlobalReputation(ctx, &types.GlobalReputation{
			Validator: testAddr(i),
			Score:     500000,
		})
	}
	all := k.GetAllGlobalReputations(ctx)
	if len(all) != 3 {
		t.Errorf("expected 3 global reputations, got %d", len(all))
	}
}

// ---------- Tests: UpdateReputation ----------

func TestUpdateReputation(t *testing.T) {
	k, ctx := setupKeeper(t)
	validator := testAddr(1)

	// First approval
	k.UpdateReputation(ctx, validator, "mathematics", "theoretical", true)

	gr, found := k.GetGlobalReputation(ctx, validator)
	if !found {
		t.Fatal("expected global reputation to exist")
	}
	if gr.TotalVerifications != 1 {
		t.Errorf("expected 1 verification, got %d", gr.TotalVerifications)
	}
	if gr.Score <= 500000 {
		t.Errorf("expected score > 500000 after approval, got %d", gr.Score)
	}

	dr, found := k.GetDomainReputation(ctx, "mathematics", validator)
	if !found {
		t.Fatal("expected domain reputation to exist")
	}
	if dr.Verifications != 1 {
		t.Errorf("expected 1 domain verification, got %d", dr.Verifications)
	}
	if dr.Score <= 500000 {
		t.Errorf("expected domain score > 500000 after approval, got %d", dr.Score)
	}

	sr, found := k.GetStratumReputation(ctx, "theoretical", validator)
	if !found {
		t.Fatal("expected stratum reputation to exist")
	}
	if sr.Verifications != 1 {
		t.Errorf("expected 1 stratum verification, got %d", sr.Verifications)
	}

	// Rejection
	prevScore := gr.Score
	k.UpdateReputation(ctx, validator, "mathematics", "theoretical", false)

	gr2, _ := k.GetGlobalReputation(ctx, validator)
	if gr2.Score >= prevScore {
		t.Errorf("expected score to decrease after rejection: was %d, now %d", prevScore, gr2.Score)
	}
}

func TestUpdateReputationNoStratum(t *testing.T) {
	k, ctx := setupKeeper(t)
	validator := testAddr(2)

	// Update with empty stratum
	k.UpdateReputation(ctx, validator, "physics", "", true)

	_, found := k.GetStratumReputation(ctx, "", validator)
	if found {
		t.Error("expected no stratum reputation for empty stratum")
	}

	dr, found := k.GetDomainReputation(ctx, "physics", validator)
	if !found {
		t.Fatal("expected domain reputation")
	}
	if dr.Score <= 500000 {
		t.Errorf("expected domain score > 500000, got %d", dr.Score)
	}
}

// ---------- Tests: HHI Calculation ----------

func TestHHICalculation(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	// Create a monopoly: one validator in all rounds
	for i := 0; i < 5; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "monopoly",
			RoundId:    fmt.Sprintf("round-%d", i),
			Validators: []string{testAddr(1)},
			Verdicts:   []bool{true},
		})
	}

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, "monopoly", params)

	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	// HHI of monopoly should be 1,000,000 (100% share squared on BPS scale)
	if metrics.HerfindahlIndex != types.BPSScale {
		t.Errorf("expected HHI %d for monopoly, got %d", types.BPSScale, metrics.HerfindahlIndex)
	}
	if !metrics.Flagged {
		t.Error("expected monopoly to be flagged")
	}

	// Create a competitive market: 10 validators evenly
	for i := 0; i < 10; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "competitive",
			RoundId:    fmt.Sprintf("round-%d", i),
			Validators: []string{testAddr(i)},
			Verdicts:   []bool{true},
		})
	}

	metrics2 := k.AnalyzeCaptureRisk(ctx, "competitive", params)
	if metrics2 == nil {
		t.Fatal("expected non-nil metrics")
	}
	// HHI for 10 equal validators: (100000)^2 * 10 / 1000000 = 100000
	if metrics2.HerfindahlIndex > 150000 {
		t.Errorf("expected low HHI for competitive market, got %d", metrics2.HerfindahlIndex)
	}
	if metrics2.Flagged {
		t.Error("expected competitive market not to be flagged")
	}
}

// ---------- Tests: Timing Correlation ----------

func TestTimingCorrelation(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create rounds with tight timing (all submit at same block)
	k.SetVerificationHistory(ctx, &types.VerificationHistoryEntry{
		Domain:       "tight",
		RoundId:      "r1",
		Validators:   []string{testAddr(1), testAddr(2), testAddr(3)},
		Verdicts:     []bool{true, true, true},
		SubmitBlocks: []uint64{100, 100, 100},
		BlockHeight:  100,
	})
	k.SetVerificationHistory(ctx, &types.VerificationHistoryEntry{
		Domain:       "tight",
		RoundId:      "r2",
		Validators:   []string{testAddr(1), testAddr(2), testAddr(3)},
		Verdicts:     []bool{true, true, true},
		SubmitBlocks: []uint64{200, 200, 201},
		BlockHeight:  200,
	})

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, "tight", params)
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	// Both rounds have std dev < 2, so timing correlation should be 100%
	if metrics.TimingCorrelation != types.BPSScale {
		t.Errorf("expected timing correlation %d, got %d", types.BPSScale, metrics.TimingCorrelation)
	}

	// Create rounds with spread timing
	k.SetVerificationHistory(ctx, &types.VerificationHistoryEntry{
		Domain:       "spread",
		RoundId:      "s1",
		Validators:   []string{testAddr(1), testAddr(2), testAddr(3)},
		Verdicts:     []bool{true, true, true},
		SubmitBlocks: []uint64{100, 110, 120},
		BlockHeight:  100,
	})

	metrics2 := k.AnalyzeCaptureRisk(ctx, "spread", params)
	if metrics2 == nil {
		t.Fatal("expected non-nil metrics")
	}
	if metrics2.TimingCorrelation != 0 {
		t.Errorf("expected timing correlation 0 for spread timing, got %d", metrics2.TimingCorrelation)
	}
}

// ---------- Tests: Verdict Correlation ----------

func TestVerdictCorrelation(t *testing.T) {
	k, ctx := setupKeeper(t)

	// All unanimous rounds
	k.SetVerificationHistory(ctx, &types.VerificationHistoryEntry{
		Domain:      "unanimous",
		RoundId:     "r1",
		Validators:  []string{testAddr(1), testAddr(2), testAddr(3)},
		Verdicts:    []bool{true, true, true},
		BlockHeight: 100,
	})
	k.SetVerificationHistory(ctx, &types.VerificationHistoryEntry{
		Domain:      "unanimous",
		RoundId:     "r2",
		Validators:  []string{testAddr(1), testAddr(2), testAddr(3)},
		Verdicts:    []bool{false, false, false},
		BlockHeight: 200,
	})

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, "unanimous", params)
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	if metrics.VerdictCorrelation != types.BPSScale {
		t.Errorf("expected verdict correlation %d, got %d", types.BPSScale, metrics.VerdictCorrelation)
	}

	// Mixed verdict rounds
	k.SetVerificationHistory(ctx, &types.VerificationHistoryEntry{
		Domain:      "mixed",
		RoundId:     "m1",
		Validators:  []string{testAddr(1), testAddr(2), testAddr(3)},
		Verdicts:    []bool{true, false, true},
		BlockHeight: 100,
	})

	metrics2 := k.AnalyzeCaptureRisk(ctx, "mixed", params)
	if metrics2 == nil {
		t.Fatal("expected non-nil metrics")
	}
	if metrics2.VerdictCorrelation != 0 {
		t.Errorf("expected verdict correlation 0 for mixed verdicts, got %d", metrics2.VerdictCorrelation)
	}
}

// ---------- Tests: Decay ----------

func TestDecayReputation(t *testing.T) {
	// Score 800000, base 500000, after 1 half-life
	result := keeper.DecayReputation(800000, 500000, 10000, 10000)
	// After one full half-life: base + (800000 - 500000) / 2 = 500000 + 150000 = 650000
	if result != 650000 {
		t.Errorf("expected 650000 after 1 half-life, got %d", result)
	}

	// After 2 half-lives
	result2 := keeper.DecayReputation(800000, 500000, 20000, 10000)
	// base + (800000 - 500000) / 4 = 500000 + 75000 = 575000
	if result2 != 575000 {
		t.Errorf("expected 575000 after 2 half-lives, got %d", result2)
	}

	// Zero age: no decay
	result3 := keeper.DecayReputation(800000, 500000, 0, 10000)
	if result3 != 800000 {
		t.Errorf("expected 800000 with zero age, got %d", result3)
	}

	// Score at base: no decay
	result4 := keeper.DecayReputation(500000, 500000, 10000, 10000)
	if result4 != 500000 {
		t.Errorf("expected 500000 when score equals base, got %d", result4)
	}

	// Score below base: unchanged
	result5 := keeper.DecayReputation(300000, 500000, 10000, 10000)
	if result5 != 300000 {
		t.Errorf("expected 300000 when score below base, got %d", result5)
	}
}

func TestDecayAllReputations(t *testing.T) {
	k, ctx := setupKeeper(t)

	validator := testAddr(1)
	k.SetGlobalReputation(ctx, &types.GlobalReputation{
		Validator:          validator,
		Score:              800000,
		TotalVerifications: 20,
		LastUpdatedBlock:   0, // old block
	})

	params := types.DefaultParams()
	k.SetParams(ctx, params)

	// ctx is at block 100, but decay epoch is 10000 so age = 100
	k.DecayAllReputations(ctx, params)

	gr, _ := k.GetGlobalReputation(ctx, validator)
	// With age=100, halfLife=10000: delta stays mostly the same
	if gr.Score >= 800000 {
		t.Errorf("expected score to decrease after decay, got %d", gr.Score)
	}
	if gr.LastUpdatedBlock != 100 {
		t.Errorf("expected last_updated_block 100, got %d", gr.LastUpdatedBlock)
	}
}

// ---------- Tests: UpdateParams ----------

func TestUpdateParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()

	srv := keeper.NewMsgServerImpl(k)
	newParams := types.DefaultParams()
	newParams.DecayEpochBlocks = 20000
	newParams.HhiThreshold = 300000

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.DecayEpochBlocks != 20000 {
		t.Errorf("expected DecayEpochBlocks 20000, got %d", got.DecayEpochBlocks)
	}
	if got.HhiThreshold != 300000 {
		t.Errorf("expected HhiThreshold 300000, got %d", got.HhiThreshold)
	}
}

func TestUpdateParamsUnauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAddr(99),
		Params:    types.DefaultParams(),
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestUpdateParamsNilParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    nil,
	})
	if err == nil {
		t.Fatal("expected nil params error")
	}
}

func TestUpdateParamsInvalid(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	badParams := types.DefaultParams()
	badParams.DecayEpochBlocks = 0 // invalid: must be > 0

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: authority,
		Params:    badParams,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid params")
	}
}

// ---------- Tests: CrossStratum ----------

func TestCrossStratum(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set up cross-stratum requirement
	k.SetCrossStratumRequirement(ctx, &types.CrossStratumRequirement{
		TargetStratum:           "theoretical",
		RequiredStrata:          []string{"empirical"},
		MinValidatorsPerStratum: 1,
	})

	params := types.DefaultParams()
	params.MinVerificationsForScore = 3
	k.SetParams(ctx, params)

	validator := testAddr(1)

	// No reputation -> should fail
	result := k.ValidateCrossStratum(ctx, "math", "theoretical", []string{validator})
	if result {
		t.Error("expected cross-stratum validation to fail with no reputation")
	}

	// Add empirical reputation with enough verifications
	k.SetStratumReputation(ctx, &types.StratumReputation{
		Validator:        validator,
		Stratum:          "empirical",
		Score:            600000,
		Verifications:    5,
		LastUpdatedBlock: 90,
	})

	result = k.ValidateCrossStratum(ctx, "math", "theoretical", []string{validator})
	if !result {
		t.Error("expected cross-stratum validation to pass with sufficient reputation")
	}
}

func TestCrossStratumNoRequirement(t *testing.T) {
	k, ctx := setupKeeper(t)

	// No requirement set for "applied" stratum
	result := k.ValidateCrossStratum(ctx, "math", "applied", []string{testAddr(1)})
	if !result {
		t.Error("expected cross-stratum validation to pass when no requirement exists")
	}
}

// ---------- Tests: CaptureMetrics CRUD ----------

func TestCaptureMetricsCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, found := k.GetCaptureMetrics(ctx, "physics")
	if found {
		t.Error("expected metrics not found")
	}

	m := &types.CaptureMetrics{
		Domain:          "physics",
		HerfindahlIndex: 150000,
		RiskScore:       120000,
		Flagged:         false,
		AnalyzedAtBlock: 100,
	}
	k.SetCaptureMetrics(ctx, m)

	got, found := k.GetCaptureMetrics(ctx, "physics")
	if !found {
		t.Fatal("expected metrics to be found")
	}
	if got.HerfindahlIndex != 150000 {
		t.Errorf("expected HHI 150000, got %d", got.HerfindahlIndex)
	}

	k.DeleteCaptureMetrics(ctx, "physics")
	_, found = k.GetCaptureMetrics(ctx, "physics")
	if found {
		t.Error("expected metrics to be deleted")
	}
}

// ---------- Tests: Verification History ----------

func TestVerificationHistory(t *testing.T) {
	k, ctx := setupKeeper(t)

	entry := &types.VerificationHistoryEntry{
		Domain:       "math",
		RoundId:      "r1",
		Validators:   []string{testAddr(1), testAddr(2)},
		Verdicts:     []bool{true, false},
		SubmitBlocks: []uint64{100, 101},
		BlockHeight:  100,
	}
	k.SetVerificationHistory(ctx, entry)

	got, found := k.GetVerificationHistory(ctx, "math", "r1")
	if !found {
		t.Fatal("expected history entry to be found")
	}
	if len(got.Validators) != 2 {
		t.Errorf("expected 2 validators, got %d", len(got.Validators))
	}

	// Get by domain
	entries := k.GetHistoryByDomain(ctx, "math")
	if len(entries) != 1 {
		t.Errorf("expected 1 history entry for domain, got %d", len(entries))
	}

	// Get domains with history
	k.SetVerificationHistory(ctx, &types.VerificationHistoryEntry{
		Domain: "physics", RoundId: "r2", Validators: []string{testAddr(3)},
		Verdicts: []bool{true}, BlockHeight: 200,
	})
	domains := k.GetDomainsWithHistory(ctx)
	if len(domains) != 2 {
		t.Errorf("expected 2 domains with history, got %d", len(domains))
	}

	k.DeleteVerificationHistory(ctx, "math", "r1")
	_, found = k.GetVerificationHistory(ctx, "math", "r1")
	if found {
		t.Error("expected history entry to be deleted")
	}
}

// ---------- Tests: Genesis ----------

func TestGenesis(t *testing.T) {
	k, ctx := setupKeeper(t)

	validator1 := testAddr(1)
	validator2 := testAddr(2)

	k.SetGlobalReputation(ctx, &types.GlobalReputation{
		Validator: validator1, Score: 600000, TotalVerifications: 10,
	})
	k.SetGlobalReputation(ctx, &types.GlobalReputation{
		Validator: validator2, Score: 700000, TotalVerifications: 20,
	})
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "math", HerfindahlIndex: 200000, RiskScore: 100000,
	})

	genState := k.ExportGenesis(ctx)
	if len(genState.GlobalReputations) != 2 {
		t.Errorf("expected 2 global reputations, got %d", len(genState.GlobalReputations))
	}
	if len(genState.CaptureMetrics) != 1 {
		t.Errorf("expected 1 capture metrics, got %d", len(genState.CaptureMetrics))
	}

	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, genState)

	got := k2.ExportGenesis(ctx2)
	if len(got.GlobalReputations) != 2 {
		t.Errorf("expected 2 global reputations after import, got %d", len(got.GlobalReputations))
	}
	if len(got.CaptureMetrics) != 1 {
		t.Errorf("expected 1 capture metrics after import, got %d", len(got.CaptureMetrics))
	}
}

// ---------- Tests: Msg Server ----------

func TestRecordVerification(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority:    authority,
		Domain:       "mathematics",
		RoundId:      "round-1",
		Validators:   []string{testAddr(1), testAddr(2), testAddr(3)},
		Verdicts:     []bool{true, true, false},
		SubmitBlocks: []uint64{100, 101, 102},
	})
	if err != nil {
		t.Fatalf("RecordVerification failed: %v", err)
	}

	// Verify history stored
	entry, found := k.GetVerificationHistory(ctx, "mathematics", "round-1")
	if !found {
		t.Fatal("expected history entry")
	}
	if len(entry.Validators) != 3 {
		t.Errorf("expected 3 validators, got %d", len(entry.Validators))
	}

	// Verify reputations updated
	gr, found := k.GetGlobalReputation(ctx, testAddr(1))
	if !found {
		t.Fatal("expected global reputation for validator 1")
	}
	if gr.Score <= 500000 {
		t.Errorf("expected score > 500000 for approved validator, got %d", gr.Score)
	}

	gr3, found := k.GetGlobalReputation(ctx, testAddr(3))
	if !found {
		t.Fatal("expected global reputation for validator 3")
	}
	if gr3.Score >= 500000 {
		t.Errorf("expected score < 500000 for rejected validator, got %d", gr3.Score)
	}
}

func TestRecordVerificationUnauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority:  testAddr(99),
		Domain:     "mathematics",
		RoundId:    "round-1",
		Validators: []string{testAddr(1)},
		Verdicts:   []bool{true},
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestRecordVerificationMismatchedArrays(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	_, err := srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority:  authority,
		Domain:     "mathematics",
		RoundId:    "round-1",
		Validators: []string{testAddr(1), testAddr(2)},
		Verdicts:   []bool{true},
	})
	if err == nil {
		t.Fatal("expected mismatched array length error")
	}
}

func TestAnalyzeDomain(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	// Record some history first
	for i := 0; i < 5; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "physics",
			RoundId:    fmt.Sprintf("round-%d", i),
			Validators: []string{testAddr(1), testAddr(2)},
			Verdicts:   []bool{true, true},
		})
	}

	resp, err := srv.AnalyzeDomain(ctx, &types.MsgAnalyzeDomain{
		Sender: testAddr(10),
		Domain: "physics",
	})
	if err != nil {
		t.Fatalf("AnalyzeDomain failed: %v", err)
	}
	if resp.RiskScore == 0 {
		t.Error("expected non-zero risk score")
	}
}

func TestAnalyzeDomainEmpty(t *testing.T) {
	k, ctx := setupKeeper(t)
	srv := keeper.NewMsgServerImpl(k)

	resp, err := srv.AnalyzeDomain(ctx, &types.MsgAnalyzeDomain{
		Sender: testAddr(10),
		Domain: "empty-domain",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RiskScore != 0 {
		t.Errorf("expected zero risk score for empty domain, got %d", resp.RiskScore)
	}
}

// ---------- Tests: Query Server ----------

func TestQueryParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Params.DecayEpochBlocks != 10000 {
		t.Errorf("expected 10000, got %d", resp.Params.DecayEpochBlocks)
	}
}

func TestQueryReputation(t *testing.T) {
	k, ctx := setupKeeper(t)
	validator := testAddr(1)

	k.SetGlobalReputation(ctx, &types.GlobalReputation{
		Validator: validator, Score: 700000, TotalVerifications: 15,
	})
	k.SetDomainReputation(ctx, &types.DomainReputation{
		Validator: validator, Domain: "math", Score: 800000, Verifications: 10,
	})

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.Reputation(ctx, &types.QueryReputationRequest{
		Validator: validator,
		Domain:    "math",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Global == nil {
		t.Fatal("expected non-nil global reputation")
	}
	if resp.Global.Score != 700000 {
		t.Errorf("expected global score 700000, got %d", resp.Global.Score)
	}
	if resp.Domain == nil {
		t.Fatal("expected non-nil domain reputation")
	}
	if resp.Domain.Score != 800000 {
		t.Errorf("expected domain score 800000, got %d", resp.Domain.Score)
	}
	// Effective score should come from domain (has enough verifications with default min=5)
	if resp.EffectiveScore != 800000 {
		t.Errorf("expected effective score 800000, got %d", resp.EffectiveScore)
	}
}

func TestQueryCaptureMetrics(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "physics", HerfindahlIndex: 200000, RiskScore: 150000,
	})

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.CaptureMetrics(ctx, &types.QueryCaptureMetricsRequest{Domain: "physics"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Metrics.HerfindahlIndex != 200000 {
		t.Errorf("expected HHI 200000, got %d", resp.Metrics.HerfindahlIndex)
	}
}

func TestQueryCaptureMetricsNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.CaptureMetrics(ctx, &types.QueryCaptureMetricsRequest{Domain: "nonexistent"})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestQueryCrossStratumRequirements(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetCrossStratumRequirement(ctx, &types.CrossStratumRequirement{
		TargetStratum:           "theoretical",
		RequiredStrata:          []string{"empirical"},
		MinValidatorsPerStratum: 2,
	})
	k.SetCrossStratumRequirement(ctx, &types.CrossStratumRequirement{
		TargetStratum:           "applied",
		RequiredStrata:          []string{"theoretical", "empirical"},
		MinValidatorsPerStratum: 1,
	})

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.CrossStratumRequirements(ctx, &types.QueryCrossStratumRequirementsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Requirements) != 2 {
		t.Errorf("expected 2 requirements, got %d", len(resp.Requirements))
	}
}

// ---------- Tests: Effective Reputation ----------

func TestEffectiveReputationFallback(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	k.SetParams(ctx, params)

	validator := testAddr(1)

	// No reputation at all -> base score
	score := k.GetEffectiveReputation(ctx, validator, "math", "theoretical", params)
	if score != params.BaseReputationScore {
		t.Errorf("expected base score %d, got %d", params.BaseReputationScore, score)
	}

	// Global only
	k.SetGlobalReputation(ctx, &types.GlobalReputation{
		Validator: validator, Score: 600000, TotalVerifications: 10,
	})
	score = k.GetEffectiveReputation(ctx, validator, "math", "theoretical", params)
	if score != 600000 {
		t.Errorf("expected 600000 from global fallback, got %d", score)
	}

	// Domain with insufficient verifications -> falls back to global
	k.SetDomainReputation(ctx, &types.DomainReputation{
		Validator: validator, Domain: "math", Score: 900000, Verifications: 2,
	})
	score = k.GetEffectiveReputation(ctx, validator, "math", "theoretical", params)
	if score != 600000 {
		t.Errorf("expected 600000 (domain verifications too low), got %d", score)
	}

	// Domain with sufficient verifications -> uses domain
	k.SetDomainReputation(ctx, &types.DomainReputation{
		Validator: validator, Domain: "math", Score: 900000, Verifications: 10,
	})
	score = k.GetEffectiveReputation(ctx, validator, "math", "theoretical", params)
	if score != 900000 {
		t.Errorf("expected 900000 from domain reputation, got %d", score)
	}
}

// ---------- Tests: Accuracy Tracking ----------

func TestAccuracyTrackingMultipleVerifications(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	validator := testAddr(1)

	// Record 10 approvals
	for i := 0; i < 10; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "math",
			RoundId:    fmt.Sprintf("approved-%d", i),
			Validators: []string{validator},
			Verdicts:   []bool{true},
		})
	}

	gr, found := k.GetGlobalReputation(ctx, validator)
	if !found {
		t.Fatal("expected global reputation")
	}
	if gr.TotalVerifications != 10 {
		t.Errorf("expected 10 verifications, got %d", gr.TotalVerifications)
	}
	// Score should be above base (500000) after 10 approvals
	if gr.Score <= 500000 {
		t.Errorf("expected score > 500000 after 10 approvals, got %d", gr.Score)
	}

	// Record 5 rejections
	for i := 0; i < 5; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "math",
			RoundId:    fmt.Sprintf("rejected-%d", i),
			Validators: []string{validator},
			Verdicts:   []bool{false},
		})
	}

	gr2, _ := k.GetGlobalReputation(ctx, validator)
	if gr2.TotalVerifications != 15 {
		t.Errorf("expected 15 verifications, got %d", gr2.TotalVerifications)
	}
	// Score should have decreased from rejections
	if gr2.Score >= gr.Score {
		t.Errorf("expected score to decrease after rejections: was %d, now %d", gr.Score, gr2.Score)
	}
}

func TestConsecutiveIncorrectScoreDrop(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	validator := testAddr(1)

	// First, build up a high score
	for i := 0; i < 20; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "physics",
			RoundId:    fmt.Sprintf("good-%d", i),
			Validators: []string{validator},
			Verdicts:   []bool{true},
		})
	}

	highScore, _ := k.GetGlobalReputation(ctx, validator)
	prevScore := highScore.Score

	// Now record many consecutive incorrect verifications
	for i := 0; i < 10; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "physics",
			RoundId:    fmt.Sprintf("bad-%d", i),
			Validators: []string{validator},
			Verdicts:   []bool{false},
		})

		gr, _ := k.GetGlobalReputation(ctx, validator)
		if gr.Score >= prevScore {
			t.Errorf("iteration %d: expected score to decrease: was %d, now %d", i, prevScore, gr.Score)
		}
		prevScore = gr.Score
	}

	finalGR, _ := k.GetGlobalReputation(ctx, validator)
	// After 10 consecutive incorrect, score should drop meaningfully (at least 15% below peak)
	threshold := highScore.Score * 85 / 100
	if finalGR.Score > threshold {
		t.Errorf("expected meaningful score drop after 10 consecutive incorrect: peak=%d, final=%d, threshold=%d",
			highScore.Score, finalGR.Score, threshold)
	}
}

// ---------- Tests: Herfindahl with Known Inputs ----------

func TestHHI4EqualValidators(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	// 4 equal validators each participating in same number of rounds
	for i := 0; i < 4; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "even",
			RoundId:    fmt.Sprintf("round-%d", i),
			Validators: []string{testAddr(i)},
			Verdicts:   []bool{true},
		})
	}

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, "even", params)
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	// HHI for 4 equal validators: 4 * (250000)^2 / 1000000 = 250000
	if metrics.HerfindahlIndex > 300000 {
		t.Errorf("expected HHI ~250000 for 4 equal validators, got %d", metrics.HerfindahlIndex)
	}
	if metrics.HerfindahlIndex < 200000 {
		t.Errorf("expected HHI ~250000 for 4 equal validators, got %d", metrics.HerfindahlIndex)
	}
}

// ---------- Tests: BeginBlocker ----------

func TestBeginBlocker(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	// Set params so decay and analysis trigger at block 1000
	params := types.DefaultParams()
	params.DecayEpochBlocks = 1000
	params.RiskAnalysisInterval = 1000
	k.SetParams(ctx, params)

	// Add some data
	k.SetGlobalReputation(ctx, &types.GlobalReputation{
		Validator: testAddr(1), Score: 800000, TotalVerifications: 10, LastUpdatedBlock: 0,
	})
	_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
		Authority: authority, Domain: "test", RoundId: "r1",
		Validators: []string{testAddr(1)}, Verdicts: []bool{true},
	})

	// Run at block 1000
	ctx1000 := ctx.WithBlockHeight(1000)
	err := k.BeginBlocker(ctx1000)
	if err != nil {
		t.Fatalf("BeginBlocker failed: %v", err)
	}

	// Verify decay happened
	gr, _ := k.GetGlobalReputation(ctx1000, testAddr(1))
	if gr.LastUpdatedBlock != 1000 {
		t.Errorf("expected last_updated_block 1000 after decay, got %d", gr.LastUpdatedBlock)
	}

	// Verify analysis ran
	_, found := k.GetCaptureMetrics(ctx1000, "test")
	if !found {
		t.Error("expected capture metrics to exist after auto-analysis")
	}
}

// ---------- Tests: GetDomainCapturePenalty (R31-1) ----------

func TestGetDomainCapturePenalty_NotFlagged(t *testing.T) {
	k, ctx := setupKeeper(t)
	flagged, penalty := k.GetDomainCapturePenalty(ctx, "physics")
	require.False(t, flagged)
	require.Equal(t, uint64(0), penalty)
}

func TestGetDomainCapturePenalty_Flagged(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain:          "physics",
		HerfindahlIndex: 400_000,
		Flagged:         true,
	})
	flagged, penalty := k.GetDomainCapturePenalty(ctx, "physics")
	require.True(t, flagged)
	require.Equal(t, uint64(400_000), penalty)
}

func TestGetDomainCapturePenalty_FlaggedMonopoly(t *testing.T) {
	k, ctx := setupKeeper(t)
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain:          "physics",
		HerfindahlIndex: 1_000_000,
		Flagged:         true,
	})
	flagged, penalty := k.GetDomainCapturePenalty(ctx, "physics")
	require.True(t, flagged)
	require.Equal(t, uint64(1_000_000), penalty)
}
