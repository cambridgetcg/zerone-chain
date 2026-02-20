package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/ontology/keeper"
	"github.com/zerone-chain/zerone/x/ontology/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

// ---------- Mock Bank Keeper ----------

type mockBankKeeper struct {
	balances map[string]map[string]sdkmath.Int // addr -> denom -> amount
	module   map[string]map[string]sdkmath.Int // module -> denom -> amount
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances: make(map[string]map[string]sdkmath.Int),
		module:   make(map[string]map[string]sdkmath.Int),
	}
}

func (bk *mockBankKeeper) fundAccount(addr string, amount sdkmath.Int) {
	if bk.balances[addr] == nil {
		bk.balances[addr] = make(map[string]sdkmath.Int)
	}
	bk.balances[addr]["uzrn"] = amount
}

func (bk *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	addr := senderAddr.String()
	for _, coin := range amt {
		bal, ok := bk.balances[addr]
		if !ok {
			return fmt.Errorf("account %s has no balance", addr)
		}
		current, exists := bal[coin.Denom]
		if !exists || current.LT(coin.Amount) {
			return fmt.Errorf("insufficient balance: have %s, need %s", current, coin.Amount)
		}
		bk.balances[addr][coin.Denom] = current.Sub(coin.Amount)

		if bk.module[recipientModule] == nil {
			bk.module[recipientModule] = make(map[string]sdkmath.Int)
		}
		existing := bk.module[recipientModule][coin.Denom]
		if existing.IsNil() {
			existing = sdkmath.ZeroInt()
		}
		bk.module[recipientModule][coin.Denom] = existing.Add(coin.Amount)
	}
	return nil
}

func (bk *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	addr := recipientAddr.String()
	for _, coin := range amt {
		modBal, ok := bk.module[senderModule]
		if !ok {
			return fmt.Errorf("module %s has no balance", senderModule)
		}
		current, exists := modBal[coin.Denom]
		if !exists || current.LT(coin.Amount) {
			return fmt.Errorf("module %s insufficient balance", senderModule)
		}
		bk.module[senderModule][coin.Denom] = current.Sub(coin.Amount)

		if bk.balances[addr] == nil {
			bk.balances[addr] = make(map[string]sdkmath.Int)
		}
		existing := bk.balances[addr][coin.Denom]
		if existing.IsNil() {
			existing = sdkmath.ZeroInt()
		}
		bk.balances[addr][coin.Denom] = existing.Add(coin.Amount)
	}
	return nil
}

// ---------- Test Setup ----------

var largeBalance = sdkmath.NewIntWithDecimal(1, 24) // 10^24 uzrn = 1M ZRN

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	err := stateStore.LoadLatestVersion()
	if err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	bk := newMockBankKeeper()

	k := keeper.NewKeeper(
		cdc,
		runtime.NewKVStoreService(storeKey),
		bk,
		"authority",
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())

	// Initialize genesis
	k.InitGenesis(ctx, types.DefaultGenesis())

	return k, ctx, bk
}

func testAddr(name string) string {
	addr := sdk.AccAddress([]byte("addr_" + name + "_______________")[:20])
	return addr.String()
}

// ---------- Params Tests ----------

func TestSetGetParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	params := k.GetParams(ctx)

	if params.ProposalVotingPeriod != 34272 {
		t.Errorf("expected proposal_voting_period=34272, got %d", params.ProposalVotingPeriod)
	}
	if params.MinEndorsements != 3 {
		t.Errorf("expected min_endorsements=3, got %d", params.MinEndorsements)
	}
	if params.MaxDomainsPerStratum != 100 {
		t.Errorf("expected max_domains_per_stratum=100, got %d", params.MaxDomainsPerStratum)
	}
	if params.CrossStratumDiscount != 50000 {
		t.Errorf("expected cross_stratum_discount=50000, got %d", params.CrossStratumDiscount)
	}
}

func TestUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	newParams := types.Params{
		MinProposalStake:     "5000000",
		ProposalVotingPeriod: 50000,
		MinEndorsements:      5,
		CrossStratumDiscount: 100000,
		MaxDomainsPerStratum: 50,
		AllowNewStrata:       true,
	}
	k.SetParams(ctx, &newParams)

	got := k.GetParams(ctx)
	if got.MinProposalStake != "5000000" {
		t.Errorf("expected min_proposal_stake=5000000, got %s", got.MinProposalStake)
	}
	if !got.AllowNewStrata {
		t.Errorf("expected allow_new_strata=true, got false")
	}
}

// ---------- Strata Tests ----------

func TestDefaultStrataInitialized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	strata := k.GetAllStrata(ctx)
	if len(strata) != 7 {
		t.Fatalf("expected 7 strata, got %d", len(strata))
	}
}

func TestGetStratum(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	axiomatic, found := k.GetStratum(ctx, types.StratumAxiomatic)
	if !found {
		t.Fatal("axiomatic stratum not found")
	}
	if axiomatic.Name != "axiomatic" {
		t.Errorf("expected name=axiomatic, got %s", axiomatic.Name)
	}
	if !axiomatic.Complete || !axiomatic.Decidable {
		t.Error("axiomatic stratum should be complete and decidable")
	}
	if axiomatic.GoedelApplies {
		t.Error("Goedel should not apply to axiomatic stratum")
	}
	if axiomatic.MaxConfidence != 1000000 {
		t.Errorf("expected axiomatic max_confidence=1000000, got %d", axiomatic.MaxConfidence)
	}
	if axiomatic.DecayRate != 0 {
		t.Errorf("expected axiomatic decay_rate=0, got %d", axiomatic.DecayRate)
	}

	comp, found := k.GetStratum(ctx, types.StratumComputational)
	if !found {
		t.Fatal("computational stratum not found")
	}
	if !comp.GoedelApplies {
		t.Error("Goedel should apply to computational stratum")
	}
	if comp.Decidable {
		t.Error("computational stratum should not be decidable")
	}

	test, found := k.GetStratum(ctx, types.StratumTestimonial)
	if !found {
		t.Fatal("testimonial stratum not found")
	}
	if test.MaxConfidence != 800000 {
		t.Errorf("expected testimonial max_confidence=800000, got %d", test.MaxConfidence)
	}
	if test.DecayRate != 200 {
		t.Errorf("expected testimonial decay_rate=200, got %d", test.DecayRate)
	}
}

func TestGetStratumNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	_, found := k.GetStratum(ctx, types.Stratum(99))
	if found {
		t.Error("expected stratum 99 to not be found")
	}
}

// ---------- Domain Tests ----------

func TestDefaultDomainsInitialized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	domains := k.GetAllDomains(ctx)
	if len(domains) != 7 {
		t.Fatalf("expected 7 default domains, got %d", len(domains))
	}
}

func TestGetDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	domain, found := k.GetDomain(ctx, "mathematics")
	if !found {
		t.Fatal("mathematics domain not found")
	}
	if domain.DisplayName != "Mathematics" {
		t.Errorf("expected display_name=Mathematics, got %s", domain.DisplayName)
	}
	if domain.Stratum != uint32(types.StratumFormal) {
		t.Errorf("expected stratum=Formal(1), got %d", domain.Stratum)
	}
	if domain.Status != "active" {
		t.Errorf("expected status=active, got %s", domain.Status)
	}
}

func TestGetDomainNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	_, found := k.GetDomain(ctx, "nonexistent")
	if found {
		t.Error("expected nonexistent domain to not be found")
	}
}

func TestGetDomainsByStratum(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	empirical := k.GetDomainsByStratum(ctx, types.StratumEmpirical)
	if len(empirical) != 2 {
		t.Errorf("expected 2 empirical domains, got %d", len(empirical))
	}

	axiomatic := k.GetDomainsByStratum(ctx, types.StratumAxiomatic)
	if len(axiomatic) != 1 {
		t.Errorf("expected 1 axiomatic domain, got %d", len(axiomatic))
	}
}

func TestDeleteDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.DeleteDomain(ctx, "history")

	_, found := k.GetDomain(ctx, "history")
	if found {
		t.Error("expected history domain to be deleted")
	}

	historical := k.GetDomainsByStratum(ctx, types.StratumHistorical)
	if len(historical) != 0 {
		t.Errorf("expected 0 historical domains after delete, got %d", len(historical))
	}
}

// ---------- Domain Operations Tests ----------

func TestIncrementClaimCount(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.IncrementClaimCount(ctx, "physics")
	if err != nil {
		t.Fatalf("failed to increment claim count: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "physics")
	if domain.ClaimCount != 1 {
		t.Errorf("expected claim_count=1, got %d", domain.ClaimCount)
	}

	err = k.IncrementClaimCount(ctx, "physics")
	if err != nil {
		t.Fatalf("failed to increment claim count: %v", err)
	}

	domain, _ = k.GetDomain(ctx, "physics")
	if domain.ClaimCount != 2 {
		t.Errorf("expected claim_count=2, got %d", domain.ClaimCount)
	}
}

func TestIncrementClaimCountNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.IncrementClaimCount(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent domain")
	}
}

func TestIncrementFactCount(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.IncrementFactCount(ctx, "mathematics")
	if err != nil {
		t.Fatalf("failed to increment fact count: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "mathematics")
	if domain.FactCount != 1 {
		t.Errorf("expected fact_count=1, got %d", domain.FactCount)
	}
}

func TestValidateDomainForClaim(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.ValidateDomainForClaim(ctx, "physics")
	if err != nil {
		t.Errorf("expected active domain to be valid: %v", err)
	}

	err = k.DeprecateDomain(ctx, "physics")
	if err != nil {
		t.Fatalf("failed to deprecate domain: %v", err)
	}

	err = k.ValidateDomainForClaim(ctx, "physics")
	if err == nil {
		t.Error("expected deprecated domain to be invalid for claims")
	}

	err = k.ValidateDomainForClaim(ctx, "nonexistent")
	if err == nil {
		t.Error("expected nonexistent domain to fail validation")
	}
}

func TestGetDomainConfidenceCeiling(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	maxConf, decayRate, err := k.GetDomainConfidenceCeiling(ctx, "mathematics")
	if err != nil {
		t.Fatalf("failed to get confidence ceiling: %v", err)
	}
	if maxConf != 1000000 {
		t.Errorf("expected max_confidence=1000000, got %d", maxConf)
	}
	if decayRate != 1 {
		t.Errorf("expected decay_rate=1, got %d", decayRate)
	}

	maxConf, decayRate, err = k.GetDomainConfidenceCeiling(ctx, "history")
	if err != nil {
		t.Fatalf("failed to get confidence ceiling for history: %v", err)
	}
	if maxConf != 900000 {
		t.Errorf("expected max_confidence=900000, got %d", maxConf)
	}
	if decayRate != 100 {
		t.Errorf("expected decay_rate=100, got %d", decayRate)
	}
}

func TestIsGoedelConstrained(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	constrained, err := k.IsGoedelConstrained(ctx, "mathematics")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if constrained {
		t.Error("formal stratum should not be Goedel constrained")
	}

	constrained, err = k.IsGoedelConstrained(ctx, "computer_science")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !constrained {
		t.Error("computational stratum should be Goedel constrained")
	}
}

func TestDeprecateDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.DeprecateDomain(ctx, "general")
	if err != nil {
		t.Fatalf("failed to deprecate: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "general")
	if domain.Status != "deprecated" {
		t.Errorf("expected status=deprecated, got %s", domain.Status)
	}
	if domain.UpdatedAt != 100 {
		t.Errorf("expected updated_at=100, got %d", domain.UpdatedAt)
	}
}

func TestActivateDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetDomain(ctx, &types.Domain{
		Name:    "biology",
		Stratum: uint32(types.StratumEmpirical),
		Status:  "proposed",
	})

	err := k.ActivateDomain(ctx, "biology")
	if err != nil {
		t.Fatalf("failed to activate: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "biology")
	if domain.Status != "active" {
		t.Errorf("expected status=active, got %s", domain.Status)
	}
}

func TestCountDomainsInStratum(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	count := k.CountDomainsInStratum(ctx, types.StratumEmpirical)
	if count != 2 {
		t.Errorf("expected 2 empirical domains, got %d", count)
	}

	count = k.CountDomainsInStratum(ctx, types.StratumFormal)
	if count != 1 {
		t.Errorf("expected 1 formal domain, got %d", count)
	}
}

// ---------- Proposal Lifecycle Tests ----------

func TestCreateProposal(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "test-proposal-1",
		Domain: &types.Domain{
			Name:        "chemistry",
			DisplayName: "Chemistry",
			Stratum:     uint32(types.StratumEmpirical),
		},
		Proposer:     testAddr("alice"),
		ProposalType: "add",
		Status:       "active",
		CreatedAt:    100,
		ExpiresAt:    100 + 34272,
	}

	err := k.CreateProposal(ctx, &proposal)
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	got, found := k.GetProposal(ctx, "test-proposal-1")
	if !found {
		t.Fatal("proposal not found")
	}
	if got.Domain.Name != "chemistry" {
		t.Errorf("expected domain=chemistry, got %s", got.Domain.Name)
	}

	domain, found := k.GetDomain(ctx, "chemistry")
	if !found {
		t.Fatal("proposed domain not found")
	}
	if domain.Status != "proposed" {
		t.Errorf("expected status=proposed, got %s", domain.Status)
	}
}

func TestCreateProposalDuplicateDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "dup-proposal",
		Domain: &types.Domain{
			Name:    "mathematics",
			Stratum: uint32(types.StratumFormal),
		},
		ProposalType: "add",
	}

	err := k.CreateProposal(ctx, &proposal)
	if err == nil {
		t.Error("expected error for duplicate domain")
	}
}

func TestCreateProposalInvalidStratum(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "bad-stratum",
		Domain: &types.Domain{
			Name:    "bad_domain",
			Stratum: uint32(99),
		},
		ProposalType: "add",
	}

	err := k.CreateProposal(ctx, &proposal)
	if err == nil {
		t.Error("expected error for invalid stratum")
	}
}

func TestVoteProposal(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "vote-test",
		Domain: &types.Domain{
			Name:    "biology",
			Stratum: uint32(types.StratumEmpirical),
		},
		Proposer:     testAddr("alice"),
		ProposalType: "add",
		Status:       "active",
		Voters:       []string{},
		CreatedAt:    100,
		ExpiresAt:    100 + 34272,
	}
	err := k.CreateProposal(ctx, &proposal)
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	err = k.VoteProposal(ctx, "vote-test", testAddr("bob"), true)
	if err != nil {
		t.Fatalf("vote 1 failed: %v", err)
	}

	got, _ := k.GetProposal(ctx, "vote-test")
	if got.VotesFor != 1 {
		t.Errorf("expected votes_for=1, got %d", got.VotesFor)
	}

	err = k.VoteProposal(ctx, "vote-test", testAddr("carol"), false)
	if err != nil {
		t.Fatalf("vote 2 failed: %v", err)
	}

	got, _ = k.GetProposal(ctx, "vote-test")
	if got.VotesAgainst != 1 {
		t.Errorf("expected votes_against=1, got %d", got.VotesAgainst)
	}
}

func TestVoteProposalDuplicate(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "dup-vote-test",
		Domain: &types.Domain{
			Name:    "ecology",
			Stratum: uint32(types.StratumEmpirical),
		},
		Proposer:     testAddr("alice"),
		ProposalType: "add",
		Status:       "active",
		Voters:       []string{},
		ExpiresAt:    200,
	}
	_ = k.CreateProposal(ctx, &proposal)

	_ = k.VoteProposal(ctx, "dup-vote-test", testAddr("bob"), true)

	err := k.VoteProposal(ctx, "dup-vote-test", testAddr("bob"), true)
	if err == nil {
		t.Error("expected error for duplicate vote")
	}
}

func TestVoteProposalReachesQuorum(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "quorum-test",
		Domain: &types.Domain{
			Name:    "astronomy",
			Stratum: uint32(types.StratumEmpirical),
		},
		Proposer:     testAddr("alice"),
		ProposalType: "add",
		Status:       "active",
		Voters:       []string{},
		ExpiresAt:    200,
	}
	_ = k.CreateProposal(ctx, &proposal)

	_ = k.VoteProposal(ctx, "quorum-test", testAddr("v1"), true)
	_ = k.VoteProposal(ctx, "quorum-test", testAddr("v2"), true)
	_ = k.VoteProposal(ctx, "quorum-test", testAddr("v3"), true)

	got, _ := k.GetProposal(ctx, "quorum-test")
	if got.Status != "passed" {
		t.Errorf("expected status=passed, got %s", got.Status)
	}

	domain, found := k.GetDomain(ctx, "astronomy")
	if !found {
		t.Fatal("astronomy domain not found")
	}
	if domain.Status != "active" {
		t.Errorf("expected domain status=active, got %s", domain.Status)
	}
}

func TestVoteProposalExpired(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "expired-test",
		Domain: &types.Domain{
			Name:    "geology",
			Stratum: uint32(types.StratumEmpirical),
		},
		Proposer:     testAddr("alice"),
		ProposalType: "add",
		Status:       "active",
		Voters:       []string{},
		ExpiresAt:    50,
	}
	_ = k.CreateProposal(ctx, &proposal)

	err := k.VoteProposal(ctx, "expired-test", testAddr("bob"), true)
	if err == nil {
		t.Error("expected error for expired proposal")
	}
}

func TestProcessExpiredProposals(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Id: "will-expire",
		Domain: &types.Domain{
			Name:    "alchemy",
			Stratum: uint32(types.StratumTestimonial),
		},
		Proposer:     testAddr("alice"),
		ProposalType: "add",
		Status:       "active",
		Voters:       []string{},
		CreatedAt:    50,
		ExpiresAt:    90,
	}
	_ = k.CreateProposal(ctx, &proposal)

	err := k.ProcessExpiredProposals(ctx)
	if err != nil {
		t.Fatalf("failed to process expired proposals: %v", err)
	}

	got, _ := k.GetProposal(ctx, "will-expire")
	if got.Status != "expired" {
		t.Errorf("expected status=expired, got %s", got.Status)
	}

	_, found := k.GetDomain(ctx, "alchemy")
	if found {
		t.Error("expected proposed domain to be removed after proposal expiry")
	}
}

func TestExecuteProposalDeprecate(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Domain: &types.Domain{
			Name: "general",
		},
		ProposalType: "deprecate",
	}

	err := k.ExecuteProposal(ctx, &proposal)
	if err != nil {
		t.Fatalf("failed to execute deprecate: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "general")
	if domain.Status != "deprecated" {
		t.Errorf("expected status=deprecated, got %s", domain.Status)
	}
}

func TestExecuteProposalModify(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Domain: &types.Domain{
			Name:        "physics",
			DisplayName: "Physics & Cosmology",
			Description: "Updated description",
		},
		ProposalType: "modify",
	}

	err := k.ExecuteProposal(ctx, &proposal)
	if err != nil {
		t.Fatalf("failed to execute modify: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "physics")
	if domain.DisplayName != "Physics & Cosmology" {
		t.Errorf("expected display_name=Physics & Cosmology, got %s", domain.DisplayName)
	}
}

func TestGenerateProposalID(t *testing.T) {
	id1 := keeper.GenerateProposalID(testAddr("alice"), "chemistry", 100)
	id2 := keeper.GenerateProposalID(testAddr("alice"), "chemistry", 100)
	id3 := keeper.GenerateProposalID(testAddr("alice"), "chemistry", 101)

	if id1 != id2 {
		t.Error("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Error("different height should produce different ID")
	}
	if len(id1) != 32 {
		t.Errorf("expected ID length=32, got %d", len(id1))
	}
}

// ---------- Cross-Stratum Link Tests ----------

func TestSetGetLink(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	link := types.CrossStratumLink{
		SourceDomain: "physics",
		TargetDomain: "mathematics",
		LinkType:     "depends_on",
		Discount:     50000,
	}

	k.SetLink(ctx, &link)

	got, found := k.GetLink(ctx, "physics", "mathematics")
	if !found {
		t.Fatal("link not found")
	}
	if got.LinkType != "depends_on" {
		t.Errorf("expected link_type=depends_on, got %s", got.LinkType)
	}
	if got.Discount != 50000 {
		t.Errorf("expected discount=50000, got %d", got.Discount)
	}
}

func TestGetAllLinks(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetLink(ctx, &types.CrossStratumLink{
		SourceDomain: "physics",
		TargetDomain: "mathematics",
		LinkType:     "depends_on",
		Discount:     50000,
	})
	k.SetLink(ctx, &types.CrossStratumLink{
		SourceDomain: "computer_science",
		TargetDomain: "mathematics",
		LinkType:     "generalizes",
		Discount:     30000,
	})

	links := k.GetAllLinks(ctx)
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}
}

// ---------- MsgServer Tests ----------

func TestMsgProposeDomain(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	proposer := testAddr("alice")
	bk.fundAccount(proposer, largeBalance)

	resp, err := msgSrv.ProposeDomain(sdk.WrapSDKContext(ctx), &types.MsgProposeDomain{
		Name:        "biology",
		DisplayName: "Biology",
		Description: "The study of living organisms",
		Stratum:     uint32(types.StratumEmpirical),
		Proposer:    proposer,
		Stake:       "1000000",
	})
	if err != nil {
		t.Fatalf("failed to propose domain: %v", err)
	}
	if resp.ProposalId == "" {
		t.Error("expected non-empty proposal ID")
	}

	proposal, found := k.GetProposal(ctx, resp.ProposalId)
	if !found {
		t.Fatal("proposal not found after creation")
	}
	if proposal.Domain.Name != "biology" {
		t.Errorf("expected domain=biology, got %s", proposal.Domain.Name)
	}

	remaining := bk.balances[proposer]["uzrn"]
	expected := largeBalance.Sub(sdkmath.NewInt(1000000))
	if !remaining.Equal(expected) {
		t.Errorf("expected balance=%s, got %s", expected, remaining)
	}
}

func TestMsgProposeDomainInsufficientStake(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	proposer := testAddr("poor")
	bk.fundAccount(proposer, sdkmath.NewInt(100))

	_, err := msgSrv.ProposeDomain(sdk.WrapSDKContext(ctx), &types.MsgProposeDomain{
		Name:        "biology",
		DisplayName: "Biology",
		Stratum:     uint32(types.StratumEmpirical),
		Proposer:    proposer,
		Stake:       "1000000",
	})
	if err == nil {
		t.Error("expected error for insufficient stake")
	}
}

func TestMsgProposeDomainBelowMinStake(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	proposer := testAddr("alice")
	bk.fundAccount(proposer, largeBalance)

	_, err := msgSrv.ProposeDomain(sdk.WrapSDKContext(ctx), &types.MsgProposeDomain{
		Name:        "biology",
		DisplayName: "Biology",
		Stratum:     uint32(types.StratumEmpirical),
		Proposer:    proposer,
		Stake:       "100",
	})
	if err == nil {
		t.Error("expected error for stake below minimum")
	}
}

func TestMsgVoteDomainProposal(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	proposer := testAddr("alice")
	bk.fundAccount(proposer, largeBalance)

	resp, _ := msgSrv.ProposeDomain(sdk.WrapSDKContext(ctx), &types.MsgProposeDomain{
		Name:        "biology",
		DisplayName: "Biology",
		Stratum:     uint32(types.StratumEmpirical),
		Proposer:    proposer,
		Stake:       "1000000",
	})

	voter := testAddr("bob")
	_, err := msgSrv.VoteDomainProposal(sdk.WrapSDKContext(ctx), &types.MsgVoteDomainProposal{
		ProposalId: resp.ProposalId,
		Voter:      voter,
		Approve:    true,
	})
	if err != nil {
		t.Fatalf("failed to vote: %v", err)
	}

	proposal, _ := k.GetProposal(ctx, resp.ProposalId)
	if proposal.VotesFor != 1 {
		t.Errorf("expected votes_for=1, got %d", proposal.VotesFor)
	}
}

func TestMsgUpdateDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.UpdateDomain(sdk.WrapSDKContext(ctx), &types.MsgUpdateDomain{
		Authority:   "authority",
		DomainName:  "physics",
		DisplayName: "Physics (updated)",
	})
	if err != nil {
		t.Fatalf("failed to update domain: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "physics")
	if domain.DisplayName != "Physics (updated)" {
		t.Errorf("expected display_name=Physics (updated), got %s", domain.DisplayName)
	}
}

func TestMsgUpdateDomainUnauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.UpdateDomain(sdk.WrapSDKContext(ctx), &types.MsgUpdateDomain{
		Authority:  testAddr("hacker"),
		DomainName: "physics",
		Status:     "deprecated",
	})
	if err == nil {
		t.Error("expected error for unauthorized update")
	}
}

func TestMsgUpdateParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	newParams := types.Params{
		MinProposalStake:     "5000000",
		ProposalVotingPeriod: 50000,
		MinEndorsements:      5,
		CrossStratumDiscount: 80000,
		MaxDomainsPerStratum: 200,
		AllowNewStrata:       true,
	}

	_, err := msgSrv.UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
		Authority: "authority",
		Params:    &newParams,
	})
	if err != nil {
		t.Fatalf("failed to update params: %v", err)
	}

	got := k.GetParams(ctx)
	if got.MinEndorsements != 5 {
		t.Errorf("expected min_endorsements=5, got %d", got.MinEndorsements)
	}
	if !got.AllowNewStrata {
		t.Error("expected allow_new_strata=true")
	}
}

func TestMsgUpdateParamsUnauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	defaultParams := types.DefaultParams()
	_, err := msgSrv.UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
		Authority: testAddr("hacker"),
		Params:    &defaultParams,
	})
	if err == nil {
		t.Error("expected error for unauthorized param update")
	}
}

// ---------- Query Tests ----------

func TestQueryParams(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.Params(sdk.WrapSDKContext(ctx), &types.QueryParamsRequest{})
	if err != nil {
		t.Fatalf("query params failed: %v", err)
	}
	if resp.Params.MinEndorsements != 3 {
		t.Errorf("expected min_endorsements=3, got %d", resp.Params.MinEndorsements)
	}
}

func TestQueryStratum(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.Stratum(sdk.WrapSDKContext(ctx), &types.QueryStratumRequest{
		Stratum: uint32(types.StratumEmpirical),
	})
	if err != nil {
		t.Fatalf("query stratum failed: %v", err)
	}
	if resp.Properties.Name != "empirical" {
		t.Errorf("expected name=empirical, got %s", resp.Properties.Name)
	}
}

func TestQueryStratumInvalid(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	_, err := qSrv.Stratum(sdk.WrapSDKContext(ctx), &types.QueryStratumRequest{
		Stratum: uint32(types.Stratum(99)),
	})
	if err == nil {
		t.Error("expected error for invalid stratum query")
	}
}

func TestQueryAllStrata(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.AllStrata(sdk.WrapSDKContext(ctx), &types.QueryAllStrataRequest{})
	if err != nil {
		t.Fatalf("query all strata failed: %v", err)
	}
	if len(resp.Strata) != 7 {
		t.Errorf("expected 7 strata, got %d", len(resp.Strata))
	}
}

func TestQueryDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.Domain(sdk.WrapSDKContext(ctx), &types.QueryDomainRequest{
		Name: "logic",
	})
	if err != nil {
		t.Fatalf("query domain failed: %v", err)
	}
	if resp.Domain.DisplayName != "Logic & Foundations" {
		t.Errorf("expected display_name=Logic & Foundations, got %s", resp.Domain.DisplayName)
	}
}

func TestQueryDomainNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	_, err := qSrv.Domain(sdk.WrapSDKContext(ctx), &types.QueryDomainRequest{
		Name: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent domain query")
	}
}

func TestQueryDomainsByStratum(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.DomainsByStratum(sdk.WrapSDKContext(ctx), &types.QueryDomainsByStratumRequest{
		Stratum: uint32(types.StratumEmpirical),
	})
	if err != nil {
		t.Fatalf("query domains by stratum failed: %v", err)
	}
	if len(resp.Domains) != 2 {
		t.Errorf("expected 2 empirical domains, got %d", len(resp.Domains))
	}
}

func TestQueryAllDomains(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.AllDomains(sdk.WrapSDKContext(ctx), &types.QueryAllDomainsRequest{})
	if err != nil {
		t.Fatalf("query all domains failed: %v", err)
	}
	if len(resp.Domains) != 7 {
		t.Errorf("expected 7 domains, got %d", len(resp.Domains))
	}
}

func TestQueryProposal(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)
	qSrv := keeper.NewQueryServerImpl(k)

	proposer := testAddr("alice")
	bk.fundAccount(proposer, largeBalance)

	createResp, _ := msgSrv.ProposeDomain(sdk.WrapSDKContext(ctx), &types.MsgProposeDomain{
		Name:        "biology",
		DisplayName: "Biology",
		Stratum:     uint32(types.StratumEmpirical),
		Proposer:    proposer,
		Stake:       "1000000",
	})

	resp, err := qSrv.Proposal(sdk.WrapSDKContext(ctx), &types.QueryProposalRequest{
		ProposalId: createResp.ProposalId,
	})
	if err != nil {
		t.Fatalf("query proposal failed: %v", err)
	}
	if resp.Proposal.Domain.Name != "biology" {
		t.Errorf("expected domain=biology, got %s", resp.Proposal.Domain.Name)
	}
}

func TestQueryConfidenceCeiling(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.ConfidenceCeiling(sdk.WrapSDKContext(ctx), &types.QueryConfidenceCeilingRequest{
		DomainName: "logic",
	})
	if err != nil {
		t.Fatalf("query confidence ceiling failed: %v", err)
	}
	if resp.MaxConfidence != 1000000 {
		t.Errorf("expected max_confidence=1000000, got %d", resp.MaxConfidence)
	}
	if resp.StratumName != "axiomatic" {
		t.Errorf("expected stratum_name=axiomatic, got %s", resp.StratumName)
	}
}

// ---------- Genesis Tests ----------

func TestExportImportGenesis(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetLink(ctx, &types.CrossStratumLink{
		SourceDomain: "physics",
		TargetDomain: "mathematics",
		LinkType:     "depends_on",
		Discount:     50000,
	})

	exported := k.ExportGenesis(ctx)
	if len(exported.Strata) != 7 {
		t.Errorf("exported strata count: expected 7, got %d", len(exported.Strata))
	}
	if len(exported.CrossLinks) != 1 {
		t.Errorf("exported links count: expected 1, got %d", len(exported.CrossLinks))
	}

	storeKey := storetypes.NewKVStoreKey("test_reimport")
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	_ = stateStore.LoadLatestVersion()

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	k2 := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), nil, "authority")
	ctx2 := sdk.NewContext(stateStore, cmtproto.Header{Height: 200}, false, log.NewNopLogger())

	k2.InitGenesis(ctx2, exported)

	domains := k2.GetAllDomains(ctx2)
	if len(domains) != 7 {
		t.Errorf("re-imported domains: expected 7, got %d", len(domains))
	}

	links := k2.GetAllLinks(ctx2)
	if len(links) != 1 {
		t.Errorf("re-imported links: expected 1, got %d", len(links))
	}

	strata := k2.GetAllStrata(ctx2)
	if len(strata) != 7 {
		t.Errorf("re-imported strata: expected 7, got %d", len(strata))
	}
}

func TestGenesisValidation(t *testing.T) {
	gs := types.DefaultGenesis()
	if err := gs.Validate(); err != nil {
		t.Errorf("default genesis should be valid: %v", err)
	}

	badStrata := types.DefaultGenesis()
	badStrata.Strata = append(badStrata.Strata, &types.StratumProperties{
		Stratum:       uint32(types.StratumAxiomatic),
		Name:          "duplicate",
		MaxConfidence: 500000,
	})
	if err := badStrata.Validate(); err == nil {
		t.Error("expected error for duplicate strata")
	}

	badDomains := types.DefaultGenesis()
	badDomains.Domains = append(badDomains.Domains, &types.Domain{
		Name:    "physics",
		Stratum: uint32(types.StratumEmpirical),
	})
	if err := badDomains.Validate(); err == nil {
		t.Error("expected error for duplicate domain names")
	}

	badParams := types.DefaultGenesis()
	badParams.Params.MinEndorsements = 0
	if err := badParams.Validate(); err == nil {
		t.Error("expected error for zero min endorsements")
	}
}

// ---------- Full Lifecycle Test ----------

func TestFullProposalLifecycle(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	proposer := testAddr("alice")
	bk.fundAccount(proposer, largeBalance)

	resp, err := msgSrv.ProposeDomain(sdk.WrapSDKContext(ctx), &types.MsgProposeDomain{
		Name:        "chemistry",
		DisplayName: "Chemistry",
		Description: "Chemical sciences",
		Stratum:     uint32(types.StratumEmpirical),
		Proposer:    proposer,
		Stake:       "1000000",
	})
	if err != nil {
		t.Fatalf("step 1 (propose) failed: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "chemistry")
	if domain.Status != "proposed" {
		t.Errorf("step 1: expected status=proposed, got %s", domain.Status)
	}

	for i := 1; i <= 3; i++ {
		_, err = msgSrv.VoteDomainProposal(sdk.WrapSDKContext(ctx), &types.MsgVoteDomainProposal{
			ProposalId: resp.ProposalId,
			Voter:      testAddr(fmt.Sprintf("voter%d", i)),
			Approve:    true,
		})
		if err != nil {
			t.Fatalf("step 2 (vote %d) failed: %v", i, err)
		}
	}

	proposal, _ := k.GetProposal(ctx, resp.ProposalId)
	if proposal.Status != "passed" {
		t.Errorf("step 3: expected status=passed, got %s", proposal.Status)
	}

	domain, _ = k.GetDomain(ctx, "chemistry")
	if domain.Status != "active" {
		t.Errorf("step 4: expected domain status=active, got %s", domain.Status)
	}

	err = k.ValidateDomainForClaim(ctx, "chemistry")
	if err != nil {
		t.Errorf("step 5: expected valid domain for claims: %v", err)
	}

	maxConf, decayRate, err := k.GetDomainConfidenceCeiling(ctx, "chemistry")
	if err != nil {
		t.Fatalf("step 6: failed to get confidence ceiling: %v", err)
	}
	if maxConf != 950000 {
		t.Errorf("step 6: expected max_confidence=950000, got %d", maxConf)
	}
	if decayRate != 50 {
		t.Errorf("step 6: expected decay_rate=50, got %d", decayRate)
	}
}

func TestMaxDomainsPerStratumEnforced(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := k.GetParams(ctx)
	params.MaxDomainsPerStratum = 3
	k.SetParams(ctx, params)

	err := k.CreateProposal(ctx, &types.DomainProposal{
		Id: "add-biology",
		Domain: &types.Domain{
			Name:    "biology",
			Stratum: uint32(types.StratumEmpirical),
		},
		ProposalType: "add",
	})
	if err != nil {
		t.Fatalf("expected to add third empirical domain: %v", err)
	}

	err = k.CreateProposal(ctx, &types.DomainProposal{
		Id: "add-chemistry",
		Domain: &types.Domain{
			Name:    "chemistry",
			Stratum: uint32(types.StratumEmpirical),
		},
		ProposalType: "add",
	})
	if err == nil {
		t.Error("expected error when exceeding max domains per stratum")
	}
}

// ---------- Logic Zone Tests ----------

func TestDefaultLogicZonesInitialized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	zones := k.GetAllLogicZones(ctx)
	if len(zones) != 5 {
		t.Fatalf("expected 5 default logic zones, got %d", len(zones))
	}
}

func TestGetLogicZone(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	prop, found := k.GetLogicZone(ctx, types.ZonePropositional)
	if !found {
		t.Fatal("propositional zone not found")
	}
	if !prop.Complete || !prop.Decidable {
		t.Error("propositional zone should be complete and decidable")
	}
	if prop.GoedelApplies {
		t.Error("Goedel should not apply to propositional zone")
	}
	if prop.MaxConfidenceBps != 1000000 {
		t.Errorf("expected propositional max_confidence=1000000, got %d", prop.MaxConfidenceBps)
	}

	peano, found := k.GetLogicZone(ctx, types.ZonePeano)
	if !found {
		t.Fatal("peano zone not found")
	}
	if peano.Complete {
		t.Error("peano zone should NOT be complete")
	}
	if !peano.GoedelApplies {
		t.Error("Goedel should apply to peano zone")
	}
	if peano.MaxConfidenceBps != 850000 {
		t.Errorf("expected peano max_confidence=850000, got %d", peano.MaxConfidenceBps)
	}

	emp, found := k.GetLogicZone(ctx, types.ZoneEmpirical)
	if !found {
		t.Fatal("empirical zone not found")
	}
	if emp.GoedelApplies {
		t.Error("Goedel should not apply to empirical zone")
	}
	if emp.MaxConfidenceBps != 700000 {
		t.Errorf("expected empirical max_confidence=700000, got %d", emp.MaxConfidenceBps)
	}
}

func TestGetLogicZoneNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	_, found := k.GetLogicZone(ctx, types.LogicZone("nonexistent"))
	if found {
		t.Error("expected nonexistent zone to not be found")
	}
}

func TestValidateClaimLogicZone(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	if err := k.ValidateClaimLogicZone(ctx, "", 1000000); err != nil {
		t.Errorf("empty zone should pass: %v", err)
	}
	if err := k.ValidateClaimLogicZone(ctx, "propositional", 1000000); err != nil {
		t.Errorf("propositional 100%% should pass: %v", err)
	}
	if err := k.ValidateClaimLogicZone(ctx, "peano", 850000); err != nil {
		t.Errorf("peano at ceiling should pass: %v", err)
	}
	if err := k.ValidateClaimLogicZone(ctx, "peano", 860000); err == nil {
		t.Error("peano above ceiling should fail")
	}
	if err := k.ValidateClaimLogicZone(ctx, "set_theory", 800000); err != nil {
		t.Errorf("set_theory at ceiling should pass: %v", err)
	}
	if err := k.ValidateClaimLogicZone(ctx, "set_theory", 810000); err == nil {
		t.Error("set_theory above ceiling should fail")
	}
	if err := k.ValidateClaimLogicZone(ctx, "empirical", 700000); err != nil {
		t.Errorf("empirical at ceiling should pass: %v", err)
	}
	if err := k.ValidateClaimLogicZone(ctx, "empirical", 710000); err == nil {
		t.Error("empirical above ceiling should fail")
	}
	if err := k.ValidateClaimLogicZone(ctx, "unknown_zone", 500000); err == nil {
		t.Error("unknown zone should fail")
	}
}

func TestRequiresIncompletenessAck(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	if k.RequiresIncompletenessAck(ctx, "propositional") {
		t.Error("propositional should not require incompleteness ack")
	}
	if k.RequiresIncompletenessAck(ctx, "presburger") {
		t.Error("presburger should not require incompleteness ack")
	}
	if !k.RequiresIncompletenessAck(ctx, "peano") {
		t.Error("peano should require incompleteness ack")
	}
	if !k.RequiresIncompletenessAck(ctx, "set_theory") {
		t.Error("set_theory should require incompleteness ack")
	}
	if k.RequiresIncompletenessAck(ctx, "empirical") {
		t.Error("empirical should not require incompleteness ack")
	}
	if k.RequiresIncompletenessAck(ctx, "") {
		t.Error("empty zone should not require ack")
	}
}

func TestGetZoneConfidenceCeiling(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	if c := k.GetZoneConfidenceCeiling(ctx, "propositional"); c != 1000000 {
		t.Errorf("expected propositional ceiling=1000000, got %d", c)
	}
	if c := k.GetZoneConfidenceCeiling(ctx, "peano"); c != 850000 {
		t.Errorf("expected peano ceiling=850000, got %d", c)
	}
	if c := k.GetZoneConfidenceCeiling(ctx, "empirical"); c != 700000 {
		t.Errorf("expected empirical ceiling=700000, got %d", c)
	}
	if c := k.GetZoneConfidenceCeiling(ctx, ""); c != 1000000 {
		t.Errorf("expected empty zone ceiling=1000000, got %d", c)
	}
	if c := k.GetZoneConfidenceCeiling(ctx, "nonexistent"); c != 1000000 {
		t.Errorf("expected unknown zone ceiling=1000000, got %d", c)
	}
}

// ---------- Logic Zone Message Handler Tests ----------

func TestMsgRegisterLogicZone(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.RegisterLogicZone(sdk.WrapSDKContext(ctx), &types.MsgRegisterLogicZone{
		Authority: "authority",
		ZoneProperties: &types.LogicZoneProperties{
			Zone:             "modal_logic",
			Complete:         false,
			Decidable:        true,
			GoedelApplies:    false,
			MaxConfidenceBps: 950000,
			Description:      "Modal logic: decidable in many fragments",
		},
	})
	if err != nil {
		t.Fatalf("RegisterLogicZone failed: %v", err)
	}

	zone, found := k.GetLogicZone(ctx, "modal_logic")
	if !found {
		t.Fatal("registered zone not found")
	}
	if zone.MaxConfidenceBps != 950000 {
		t.Errorf("expected max_confidence=950000, got %d", zone.MaxConfidenceBps)
	}
}

func TestMsgRegisterLogicZoneDuplicate(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.RegisterLogicZone(sdk.WrapSDKContext(ctx), &types.MsgRegisterLogicZone{
		Authority: "authority",
		ZoneProperties: &types.LogicZoneProperties{
			Zone:             string(types.ZonePropositional),
			Complete:         true,
			MaxConfidenceBps: 1000000,
		},
	})
	if err == nil {
		t.Error("expected error for duplicate zone registration")
	}
}

func TestMsgRegisterLogicZoneUnauthorized(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.RegisterLogicZone(sdk.WrapSDKContext(ctx), &types.MsgRegisterLogicZone{
		Authority: testAddr("hacker"),
		ZoneProperties: &types.LogicZoneProperties{
			Zone:             "hacked_zone",
			MaxConfidenceBps: 1000000,
		},
	})
	if err == nil {
		t.Error("expected error for unauthorized zone registration")
	}
}

func TestMsgAcknowledgeIncompleteness(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	submitter := testAddr("alice")
	_, err := msgSrv.AcknowledgeIncompleteness(sdk.WrapSDKContext(ctx), &types.MsgAcknowledgeIncompleteness{
		Submitter: submitter,
		FactId:    "fact-123",
		Zone:      string(types.ZonePeano),
		Reason:    "This theorem is stated within Peano arithmetic and may be unprovable within the system",
	})
	if err != nil {
		t.Fatalf("AcknowledgeIncompleteness failed: %v", err)
	}

	ack, found := k.GetIncompletenessAck(ctx, "fact-123")
	if !found {
		t.Fatal("acknowledgment not found")
	}
	if ack.Zone != types.ZonePeano {
		t.Errorf("expected zone=peano, got %s", ack.Zone)
	}
	if ack.AcknowledgedBy != submitter {
		t.Errorf("expected acknowledged_by=%s, got %s", submitter, ack.AcknowledgedBy)
	}
	if ack.AcknowledgedAt != 100 {
		t.Errorf("expected acknowledged_at=100, got %d", ack.AcknowledgedAt)
	}
}

func TestMsgAcknowledgeIncompletenessCompleteZone(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.AcknowledgeIncompleteness(sdk.WrapSDKContext(ctx), &types.MsgAcknowledgeIncompleteness{
		Submitter: testAddr("alice"),
		FactId:    "fact-456",
		Zone:      string(types.ZonePropositional),
		Reason:    "This shouldn't work",
	})
	if err == nil {
		t.Error("expected error for acknowledging incompleteness in complete zone")
	}
}

func TestMsgAcknowledgeIncompletenessUnknownZone(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	_, err := msgSrv.AcknowledgeIncompleteness(sdk.WrapSDKContext(ctx), &types.MsgAcknowledgeIncompleteness{
		Submitter: testAddr("alice"),
		FactId:    "fact-789",
		Zone:      "nonexistent",
		Reason:    "Unknown zone",
	})
	if err == nil {
		t.Error("expected error for unknown zone")
	}
}

// ---------- Logic Zone Query Tests ----------

func TestQueryLogicZone(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.LogicZone(sdk.WrapSDKContext(ctx), &types.QueryLogicZoneRequest{
		Zone: string(types.ZonePeano),
	})
	if err != nil {
		t.Fatalf("query logic zone failed: %v", err)
	}
	if !resp.Properties.GoedelApplies {
		t.Error("expected Goedel to apply to peano zone")
	}
	if resp.Properties.MaxConfidenceBps != 850000 {
		t.Errorf("expected max_confidence=850000, got %d", resp.Properties.MaxConfidenceBps)
	}
}

func TestQueryLogicZoneNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	_, err := qSrv.LogicZone(sdk.WrapSDKContext(ctx), &types.QueryLogicZoneRequest{
		Zone: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent zone query")
	}
}

func TestQueryAllLogicZones(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	qSrv := keeper.NewQueryServerImpl(k)

	resp, err := qSrv.AllLogicZones(sdk.WrapSDKContext(ctx), &types.QueryAllLogicZonesRequest{})
	if err != nil {
		t.Fatalf("query all logic zones failed: %v", err)
	}
	if len(resp.Zones) != 5 {
		t.Errorf("expected 5 logic zones, got %d", len(resp.Zones))
	}
}

// ---------- Logic Zone Genesis Tests ----------

func TestLogicZonesExportImport(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	zones := k.GetAllLogicZones(ctx)
	if len(zones) != 5 {
		t.Errorf("expected 5 logic zones, got %d", len(zones))
	}

	exported := k.ExportGenesis(ctx)

	storeKey := storetypes.NewKVStoreKey("test_reimport_lz")
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	_ = stateStore.LoadLatestVersion()

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	k2 := keeper.NewKeeper(cdc, runtime.NewKVStoreService(storeKey), nil, "authority")
	ctx2 := sdk.NewContext(stateStore, cmtproto.Header{Height: 200}, false, log.NewNopLogger())

	k2.InitGenesis(ctx2, exported)

	zones = k2.GetAllLogicZones(ctx2)
	if len(zones) != 5 {
		t.Errorf("re-imported logic zones: expected 5, got %d", len(zones))
	}

	peano, found := k2.GetLogicZone(ctx2, types.ZonePeano)
	if !found {
		t.Fatal("peano zone not found after re-import")
	}
	if peano.MaxConfidenceBps != 850000 {
		t.Errorf("expected peano max_confidence=850000, got %d", peano.MaxConfidenceBps)
	}
}

// ---------- Zone-Category Consistency Tests ----------

func TestValidateZoneForCategory_Valid(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	if err := k.ValidateZoneForCategory(ctx, "propositional", "analytic"); err != nil {
		t.Errorf("analytic+propositional should be valid: %v", err)
	}
	if err := k.ValidateZoneForCategory(ctx, "presburger", "analytic"); err != nil {
		t.Errorf("analytic+presburger should be valid: %v", err)
	}
	if err := k.ValidateZoneForCategory(ctx, "peano", "formal"); err != nil {
		t.Errorf("formal+peano should be valid: %v", err)
	}
	if err := k.ValidateZoneForCategory(ctx, "set_theory", "formal"); err != nil {
		t.Errorf("formal+set_theory should be valid: %v", err)
	}
	if err := k.ValidateZoneForCategory(ctx, "empirical", "empirical"); err != nil {
		t.Errorf("empirical+empirical should be valid: %v", err)
	}
}

func TestValidateZoneForCategory_Invalid(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	if err := k.ValidateZoneForCategory(ctx, "propositional", "empirical"); err == nil {
		t.Error("empirical+propositional should be rejected")
	}
	if err := k.ValidateZoneForCategory(ctx, "presburger", "social"); err == nil {
		t.Error("social+presburger should be rejected")
	}
	if err := k.ValidateZoneForCategory(ctx, "peano", "historical"); err == nil {
		t.Error("historical+peano should be rejected")
	}
	if err := k.ValidateZoneForCategory(ctx, "empirical", "analytic"); err == nil {
		t.Error("analytic+empirical should be rejected")
	}
	if err := k.ValidateZoneForCategory(ctx, "peano", "protocol"); err == nil {
		t.Error("protocol+peano should be rejected")
	}
}

func TestValidateZoneForCategory_BackwardCompat(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	if err := k.ValidateZoneForCategory(ctx, "", "empirical"); err != nil {
		t.Errorf("empty zone should be valid (backward compat): %v", err)
	}
	if err := k.ValidateZoneForCategory(ctx, "propositional", ""); err != nil {
		t.Errorf("empty category should be valid (backward compat): %v", err)
	}
	if err := k.ValidateZoneForCategory(ctx, "propositional", "unknown_category"); err != nil {
		t.Errorf("unknown category should pass through: %v", err)
	}
}

// ---------- Archive Domain Tests ----------

func TestArchiveDomain(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.ArchiveDomain(ctx, "general")
	if err != nil {
		t.Fatalf("failed to archive: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "general")
	if domain.Status != "archived" {
		t.Errorf("expected status=archived, got %s", domain.Status)
	}

	err = k.ValidateDomainForClaim(ctx, "general")
	if err == nil {
		t.Error("expected archived domain to reject claims")
	}
}

func TestArchiveDomainNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.ArchiveDomain(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent domain")
	}
}

// ---------- Merge Domain Tests ----------

func TestMergeDomains(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	_ = k.IncrementClaimCount(ctx, "physics")
	_ = k.IncrementClaimCount(ctx, "physics")
	_ = k.IncrementFactCount(ctx, "physics")
	_ = k.IncrementClaimCount(ctx, "general")
	_ = k.IncrementFactCount(ctx, "general")

	proposal := types.DomainProposal{
		Domain: &types.Domain{
			Name:        "general",
			Description: types.FormatMergeDescription("physics"),
		},
		ProposalType: "merge",
	}

	err := k.ExecuteProposal(ctx, &proposal)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	source, _ := k.GetDomain(ctx, "general")
	if source.Status != "archived" {
		t.Errorf("expected source status=archived, got %s", source.Status)
	}

	target, _ := k.GetDomain(ctx, "physics")
	if target.ClaimCount != 3 {
		t.Errorf("expected target claim_count=3, got %d", target.ClaimCount)
	}
	if target.FactCount != 2 {
		t.Errorf("expected target fact_count=2, got %d", target.FactCount)
	}
}

func TestMergeDomainsNoTarget(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Domain: &types.Domain{
			Name:        "general",
			Description: "no merge target here",
		},
		ProposalType: "merge",
	}

	err := k.ExecuteProposal(ctx, &proposal)
	if err == nil {
		t.Error("expected error for merge without target")
	}
}

func TestMergeDomainsTargetNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Domain: &types.Domain{
			Name:        "general",
			Description: types.FormatMergeDescription("nonexistent"),
		},
		ProposalType: "merge",
	}

	err := k.ExecuteProposal(ctx, &proposal)
	if err == nil {
		t.Error("expected error when merge target doesn't exist")
	}
}

// ---------- Execute Proposal Archive Tests ----------

func TestExecuteProposalArchive(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	proposal := types.DomainProposal{
		Domain: &types.Domain{
			Name: "history",
		},
		ProposalType: "archive",
	}

	err := k.ExecuteProposal(ctx, &proposal)
	if err != nil {
		t.Fatalf("failed to execute archive: %v", err)
	}

	domain, _ := k.GetDomain(ctx, "history")
	if domain.Status != "archived" {
		t.Errorf("expected status=archived, got %s", domain.Status)
	}
}

// ---------- Stake Refund Tests ----------

func TestStakeRefundOnPassedProposal(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	msgSrv := keeper.NewMsgServerImpl(k)

	proposer := testAddr("alice")
	bk.fundAccount(proposer, largeBalance)

	resp, err := msgSrv.ProposeDomain(sdk.WrapSDKContext(ctx), &types.MsgProposeDomain{
		Name:        "chemistry",
		DisplayName: "Chemistry",
		Description: "Chemical sciences",
		Stratum:     uint32(types.StratumEmpirical),
		Proposer:    proposer,
		Stake:       "1000000",
	})
	if err != nil {
		t.Fatalf("propose failed: %v", err)
	}

	balanceAfterPropose := bk.balances[proposer]["uzrn"]
	expectedAfterPropose := largeBalance.Sub(sdkmath.NewInt(1000000))
	if !balanceAfterPropose.Equal(expectedAfterPropose) {
		t.Fatalf("expected balance after propose=%s, got %s", expectedAfterPropose, balanceAfterPropose)
	}

	for i := 1; i <= 3; i++ {
		_, err = msgSrv.VoteDomainProposal(sdk.WrapSDKContext(ctx), &types.MsgVoteDomainProposal{
			ProposalId: resp.ProposalId,
			Voter:      testAddr(fmt.Sprintf("voter%d", i)),
			Approve:    true,
		})
		if err != nil {
			t.Fatalf("vote %d failed: %v", i, err)
		}
	}

	balanceAfterPass := bk.balances[proposer]["uzrn"]
	if !balanceAfterPass.Equal(largeBalance) {
		t.Errorf("expected balance after pass=%s (full refund), got %s", largeBalance, balanceAfterPass)
	}
}

// ---------- ParseMergeTarget Tests ----------

func TestParseMergeTarget(t *testing.T) {
	target := types.ParseMergeTarget("merge_into:physics")
	if target != "physics" {
		t.Errorf("expected physics, got %s", target)
	}

	target = types.ParseMergeTarget("no merge here")
	if target != "" {
		t.Errorf("expected empty, got %s", target)
	}

	target = types.ParseMergeTarget("")
	if target != "" {
		t.Errorf("expected empty, got %s", target)
	}

	desc := types.FormatMergeDescription("mathematics")
	target = types.ParseMergeTarget(desc)
	if target != "mathematics" {
		t.Errorf("expected mathematics, got %s", target)
	}
}
