package keeper_test

import (
	"encoding/json"
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ========== Sybil Detection Unit Tests ==========

// TestRecordFunding_NewRecord verifies that a new funding record is created
// with correct fields when no prior record exists.
func TestRecordFunding_NewRecord(t *testing.T) {
	k, ctx := setupKeeper(t)

	sender := testAddr("whale__")
	recipient := testAddr("sybil1_")

	k.RecordFunding(ctx, sender, recipient, "1000000", 100)

	sources := k.GetFundingSources(ctx, recipient, 200, 480000)
	if len(sources) != 1 {
		t.Fatalf("expected 1 funding source, got %d", len(sources))
	}
	if sources[0].Sender != sender {
		t.Errorf("sender: got %s, want %s", sources[0].Sender, sender)
	}
	if sources[0].TotalAmount != "1000000" {
		t.Errorf("amount: got %s, want 1000000", sources[0].TotalAmount)
	}
	if sources[0].TransferCount != 1 {
		t.Errorf("count: got %d, want 1", sources[0].TransferCount)
	}
	if sources[0].FirstBlock != 100 {
		t.Errorf("first_block: got %d, want 100", sources[0].FirstBlock)
	}
}

// TestRecordFunding_UpdateExisting verifies that repeated funding from the
// same sender to the same recipient accumulates amount and count.
func TestRecordFunding_UpdateExisting(t *testing.T) {
	k, ctx := setupKeeper(t)

	sender := testAddr("whale__")
	recipient := testAddr("sybil1_")

	k.RecordFunding(ctx, sender, recipient, "1000000", 100)
	k.RecordFunding(ctx, sender, recipient, "2000000", 200)

	sources := k.GetFundingSources(ctx, recipient, 300, 480000)
	if len(sources) != 1 {
		t.Fatalf("expected 1 funding source (merged), got %d", len(sources))
	}
	if sources[0].TotalAmount != "3000000" {
		t.Errorf("amount: got %s, want 3000000 (accumulated)", sources[0].TotalAmount)
	}
	if sources[0].TransferCount != 2 {
		t.Errorf("count: got %d, want 2", sources[0].TransferCount)
	}
	if sources[0].FirstBlock != 100 {
		t.Errorf("first_block should be original: got %d, want 100", sources[0].FirstBlock)
	}
	if sources[0].LastBlock != 200 {
		t.Errorf("last_block should be updated: got %d, want 200", sources[0].LastBlock)
	}
}

// TestRecordFunding_MultipleSenders verifies that funding from different
// senders creates separate records.
func TestRecordFunding_MultipleSenders(t *testing.T) {
	k, ctx := setupKeeper(t)

	whale1 := testAddr("whale1_")
	whale2 := testAddr("whale2_")
	recipient := testAddr("sybil1_")

	k.RecordFunding(ctx, whale1, recipient, "1000000", 100)
	k.RecordFunding(ctx, whale2, recipient, "2000000", 100)

	sources := k.GetFundingSources(ctx, recipient, 200, 480000)
	if len(sources) != 2 {
		t.Fatalf("expected 2 funding sources, got %d", len(sources))
	}
}

// TestGetFundingSources_WindowFiltering verifies that only funding records
// within the correlation window are returned.
func TestGetFundingSources_WindowFiltering(t *testing.T) {
	k, ctx := setupKeeper(t)

	sender := testAddr("whale__")
	recipient := testAddr("sybil1_")

	// Record at block 100 with window of 100 blocks.
	k.RecordFunding(ctx, sender, recipient, "1000000", 100)

	// Within window (current=150, window=100, cutoff=50, record at 100 passes).
	sources := k.GetFundingSources(ctx, recipient, 150, 100)
	if len(sources) != 1 {
		t.Errorf("expected 1 source within window, got %d", len(sources))
	}

	// Outside window (current=250, window=100, cutoff=150, record at 100 fails).
	sources = k.GetFundingSources(ctx, recipient, 250, 100)
	if len(sources) != 0 {
		t.Errorf("expected 0 sources outside window, got %d", len(sources))
	}
}

// TestSybilParams_DefaultAndCustom verifies default sybil params and
// custom param persistence.
func TestSybilParams_DefaultAndCustom(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Defaults.
	params := k.GetSybilParams(ctx)
	if !params.Enabled {
		t.Error("expected enabled by default")
	}
	if params.CorrelationWindow != 480000 {
		t.Errorf("window: got %d, want 480000", params.CorrelationWindow)
	}
	if params.DecayPerSourceBPS != 2000 {
		t.Errorf("decay: got %d, want 2000", params.DecayPerSourceBPS)
	}
	if params.MinPowerBPS != 1000 {
		t.Errorf("min: got %d, want 1000", params.MinPowerBPS)
	}

	// Custom.
	custom := keeper.SybilParams{
		Enabled:           false,
		CorrelationWindow: 100000,
		DecayPerSourceBPS: 3000,
		MinPowerBPS:       500,
	}
	k.SetSybilParams(ctx, custom)
	got := k.GetSybilParams(ctx)
	if got.Enabled {
		t.Error("expected disabled")
	}
	if got.CorrelationWindow != 100000 {
		t.Errorf("window: got %d, want 100000", got.CorrelationWindow)
	}
}

// TestComputeSybilDecayBPS_NoSources verifies no decay when the voter has
// no funding sources.
func TestComputeSybilDecayBPS_NoSources(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create a LIP in voting stage for the decay check.
	ms := keeper.NewMsgServerImpl(k)
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "Decay Test", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	decay := k.ComputeSybilDecayBPS(ctx, testAddr("clean__"), "LIP-1")
	if decay != 10000 {
		t.Errorf("expected 10000 (no decay), got %d", decay)
	}
}

// TestComputeSybilDecayBPS_NoExistingVotes verifies no decay when the voter
// is the first to vote (no one to correlate with).
func TestComputeSybilDecayBPS_NoExistingVotes(t *testing.T) {
	k, ctx := setupKeeper(t)

	ms := keeper.NewMsgServerImpl(k)
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("alice"), Title: "First Vote", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	whale := testAddr("whale__")
	voter := testAddr("voter1_")

	// Fund voter from whale.
	k.RecordFunding(ctx, whale, voter, "1000000", 50)

	// First voter on proposal — no one to correlate with — no decay.
	decay := k.ComputeSybilDecayBPS(ctx, voter, "LIP-1")
	if decay != 10000 {
		t.Errorf("expected 10000 (first voter), got %d", decay)
	}
}

// TestGetAllFundingRecords verifies iterating all stored funding records.
func TestGetAllFundingRecords(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.RecordFunding(ctx, testAddr("sender1"), testAddr("recip1_"), "100", 10)
	k.RecordFunding(ctx, testAddr("sender2"), testAddr("recip2_"), "200", 20)
	k.RecordFunding(ctx, testAddr("sender1"), testAddr("recip2_"), "300", 30)

	all := k.GetAllFundingRecords(ctx)
	if len(all) != 3 {
		t.Errorf("expected 3 records, got %d", len(all))
	}
}

// TestSybilGenesis_ExportImport verifies sybil genesis roundtrip: export
// funding records and params, reimport, and verify they survived.
func TestSybilGenesis_ExportImport(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set custom sybil params.
	customParams := keeper.SybilParams{
		Enabled:           true,
		CorrelationWindow: 100000,
		DecayPerSourceBPS: 3000,
		MinPowerBPS:       500,
	}
	k.SetSybilParams(ctx, customParams)

	// Create funding records.
	k.RecordFunding(ctx, testAddr("sender1"), testAddr("recip1_"), "1000000", 100)
	k.RecordFunding(ctx, testAddr("sender2"), testAddr("recip1_"), "2000000", 150)
	k.RecordFunding(ctx, testAddr("sender1"), testAddr("recip2_"), "3000000", 200)

	// Export.
	exported := k.ExportSybilGenesis(ctx)

	// Verify valid JSON.
	var sg keeper.SybilGenesis
	if err := json.Unmarshal(exported, &sg); err != nil {
		t.Fatalf("exported sybil genesis is not valid JSON: %v", err)
	}
	if len(sg.FundingRecords) != 3 {
		t.Errorf("expected 3 funding records in export, got %d", len(sg.FundingRecords))
	}
	if sg.Params.DecayPerSourceBPS != 3000 {
		t.Errorf("exported params.decay should be 3000, got %d", sg.Params.DecayPerSourceBPS)
	}

	// Import into fresh keeper.
	k2, ctx2 := setupKeeper(t)
	k2.InitSybilGenesis(ctx2, exported)

	// Verify params survived.
	gotParams := k2.GetSybilParams(ctx2)
	if gotParams.CorrelationWindow != 100000 {
		t.Errorf("window: got %d, want 100000", gotParams.CorrelationWindow)
	}
	if gotParams.DecayPerSourceBPS != 3000 {
		t.Errorf("decay: got %d, want 3000", gotParams.DecayPerSourceBPS)
	}
	if gotParams.MinPowerBPS != 500 {
		t.Errorf("min: got %d, want 500", gotParams.MinPowerBPS)
	}

	// Verify funding records survived.
	allRecords := k2.GetAllFundingRecords(ctx2)
	if len(allRecords) != 3 {
		t.Fatalf("expected 3 records after import, got %d", len(allRecords))
	}

	// Verify specific record details.
	sources := k2.GetFundingSources(ctx2, testAddr("recip1_"), 300, 480000)
	if len(sources) != 2 {
		t.Errorf("expected 2 sources for recip1, got %d", len(sources))
	}
	sources2 := k2.GetFundingSources(ctx2, testAddr("recip2_"), 300, 480000)
	if len(sources2) != 1 {
		t.Errorf("expected 1 source for recip2, got %d", len(sources2))
	}
}

// TestInitSybilGenesis_NilInput verifies that nil input sets defaults.
func TestInitSybilGenesis_NilInput(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.InitSybilGenesis(ctx, nil)

	params := k.GetSybilParams(ctx)
	if !params.Enabled {
		t.Error("expected defaults with nil input")
	}
	if params.CorrelationWindow != 480000 {
		t.Errorf("expected default window, got %d", params.CorrelationWindow)
	}
}

// TestInitSybilGenesis_CorruptInput verifies that corrupt JSON falls back to defaults.
func TestInitSybilGenesis_CorruptInput(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.InitSybilGenesis(ctx, []byte("not-json"))

	params := k.GetSybilParams(ctx)
	if !params.Enabled {
		t.Error("expected defaults with corrupt input")
	}
}

// TestSybilFeatureFlag_Disabled verifies that when sybil detection is disabled,
// ComputeSybilDecayBPS returns full power (10000).
func TestSybilFeatureFlag_Disabled(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetSybilParams(ctx, keeper.SybilParams{
		Enabled:           false,
		CorrelationWindow: 480000,
		DecayPerSourceBPS: 2000,
		MinPowerBPS:       1000,
	})

	// Even with funding sources and existing votes, decay should be 10000.
	decay := k.ComputeSybilDecayBPS(ctx, testAddr("voter1_"), "LIP-1")
	if decay != 10000 {
		t.Errorf("expected 10000 (disabled), got %d", decay)
	}
}
