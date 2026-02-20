package keeper_test

import (
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/tokens/keeper"
	"github.com/zerone-chain/zerone/x/tokens/types"
)

// -----------------------------------------------------------------------
// Emission ID determinism
// -----------------------------------------------------------------------

func TestGenerateEmissionID_Deterministic(t *testing.T) {
	id1 := keeper.GenerateEmissionID(testAuthority, 100, 200)
	id2 := keeper.GenerateEmissionID(testAuthority, 100, 200)
	id3 := keeper.GenerateEmissionID(testAuthority, 100, 300)

	if id1 != id2 {
		t.Fatal("same inputs should produce same emission ID")
	}
	if id1 == id3 {
		t.Fatal("different inputs should produce different emission IDs")
	}
	if len(id1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(id1))
	}
}

// -----------------------------------------------------------------------
// Emission Period CRUD
// -----------------------------------------------------------------------

func TestEmissionPeriod_SetGetRoundTrip(t *testing.T) {
	k, ctx := setupKeeper(t)

	emission := &types.EmissionPeriod{
		Id:             "em123",
		StartBlock:     100,
		EndBlock:       200,
		AmountPerBlock: "5000",
		Recipient:      testCreator,
		Active:         true,
		TotalEmitted:   "0",
		Creator:        testAuthority,
	}
	k.SetEmissionPeriod(ctx, emission)

	got := k.GetEmissionPeriod(ctx, "em123")
	if got == nil {
		t.Fatal("expected emission period, got nil")
	}
	if got.AmountPerBlock != "5000" {
		t.Fatalf("expected amount_per_block 5000, got %s", got.AmountPerBlock)
	}
	if !got.Active {
		t.Fatal("expected active emission")
	}
}

func TestEmissionPeriod_NotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	if k.GetEmissionPeriod(ctx, "nonexistent") != nil {
		t.Fatal("expected nil for nonexistent emission period")
	}
}

func TestEmissionPeriod_Iterate(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i, id := range []string{"em1", "em2", "em3"} {
		k.SetEmissionPeriod(ctx, &types.EmissionPeriod{
			Id:             id,
			StartBlock:     uint64(100 + i*100),
			EndBlock:       uint64(200 + i*100),
			AmountPerBlock: "1000",
			Active:         true,
		})
	}

	count := 0
	k.IterateEmissionPeriods(ctx, func(emission *types.EmissionPeriod) bool {
		count++
		return false
	})
	if count != 3 {
		t.Fatalf("expected 3 emission periods, got %d", count)
	}
}

// -----------------------------------------------------------------------
// CreateEmissionPeriod
// -----------------------------------------------------------------------

func TestCreateEmissionPeriod_Success(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	resp, err := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})
	if err != nil {
		t.Fatalf("CreateEmissionPeriod failed: %v", err)
	}
	if resp.EmissionId == "" {
		t.Fatal("expected non-empty emission ID")
	}
}

func TestCreateEmissionPeriod_Unauthorized(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testCreator, // not governance
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestCreateEmissionPeriod_InvalidRange(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	// EndBlock <= StartBlock
	_, err := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     500,
		EndBlock:       200,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})
	if err == nil {
		t.Fatal("expected invalid range error")
	}

	// EndBlock == StartBlock
	_, err = srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       200,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})
	if err == nil {
		t.Fatal("expected invalid range error for equal blocks")
	}
}

func TestCreateEmissionPeriod_InvalidAmount(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "0",
		Recipient:      testCreator,
	})
	if err == nil {
		t.Fatal("expected invalid amount error for zero")
	}

	_, err = srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "-100",
		Recipient:      testCreator,
	})
	if err == nil {
		t.Fatal("expected invalid amount error for negative")
	}
}

// -----------------------------------------------------------------------
// CancelEmissionPeriod
// -----------------------------------------------------------------------

func TestCancelEmissionPeriod_Success(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	resp, _ := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})

	_, err := srv.CancelEmissionPeriod(ctx, &types.MsgCancelEmissionPeriod{
		Authority:  testAuthority,
		EmissionId: resp.EmissionId,
	})
	if err != nil {
		t.Fatalf("CancelEmissionPeriod failed: %v", err)
	}

	emission := k.GetEmissionPeriod(ctx, resp.EmissionId)
	if emission.Active {
		t.Fatal("expected emission to be inactive after cancel")
	}
}

func TestCancelEmissionPeriod_Unauthorized(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	resp, _ := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})

	_, err := srv.CancelEmissionPeriod(ctx, &types.MsgCancelEmissionPeriod{
		Authority:  testCreator, // not governance
		EmissionId: resp.EmissionId,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestCancelEmissionPeriod_NotFound(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.CancelEmissionPeriod(ctx, &types.MsgCancelEmissionPeriod{
		Authority:  testAuthority,
		EmissionId: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestCancelEmissionPeriod_AlreadyInactive(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	resp, _ := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})

	// Cancel once
	_, _ = srv.CancelEmissionPeriod(ctx, &types.MsgCancelEmissionPeriod{
		Authority: testAuthority, EmissionId: resp.EmissionId,
	})

	// Cancel again
	_, err := srv.CancelEmissionPeriod(ctx, &types.MsgCancelEmissionPeriod{
		Authority: testAuthority, EmissionId: resp.EmissionId,
	})
	if err == nil {
		t.Fatal("expected already inactive error")
	}
}

// -----------------------------------------------------------------------
// BeginBlocker — Emission Processing
// -----------------------------------------------------------------------

func TestBeginBlocker_EmissionProcessing(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	// Create an emission period: blocks 100-200, 500 uzrn per block
	emission := &types.EmissionPeriod{
		Id:             "em_test",
		StartBlock:     100,
		EndBlock:       200,
		AmountPerBlock: "500",
		Recipient:      testCreator,
		Active:         true,
		TotalEmitted:   "0",
		Creator:        testAuthority,
	}
	k.SetEmissionPeriod(ctx, emission)

	// Run BeginBlocker at block 100 (within range)
	k.BeginBlocker(ctx)

	// Check that module minted coins
	recipientAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin := bk.GetBalance(ctx, recipientAddr, "uzrn")
	if !coin.Amount.Equal(sdkmath.NewInt(500)) {
		t.Fatalf("expected 500 uzrn minted, got %s", coin.Amount.String())
	}

	// Verify total emitted updated
	em := k.GetEmissionPeriod(ctx, "em_test")
	total := new(big.Int)
	total.SetString(em.TotalEmitted, 10)
	if total.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("expected total emitted 500, got %s", total.String())
	}
}

func TestBeginBlocker_EmissionBeforeStart(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	emission := &types.EmissionPeriod{
		Id:             "em_before",
		StartBlock:     200, // starts at 200, current block is 100
		EndBlock:       300,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
		Active:         true,
		TotalEmitted:   "0",
	}
	k.SetEmissionPeriod(ctx, emission)

	k.BeginBlocker(ctx) // block 100

	// Should not mint anything
	recipientAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin := bk.GetBalance(ctx, recipientAddr, "uzrn")
	if !coin.Amount.IsZero() {
		t.Fatalf("expected 0 uzrn before start, got %s", coin.Amount.String())
	}
}

func TestBeginBlocker_EmissionAfterEnd(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	emission := &types.EmissionPeriod{
		Id:             "em_after",
		StartBlock:     10,
		EndBlock:       50, // already ended (current block is 100)
		AmountPerBlock: "1000",
		Recipient:      testCreator,
		Active:         true,
		TotalEmitted:   "0",
	}
	k.SetEmissionPeriod(ctx, emission)

	k.BeginBlocker(ctx) // block 100

	// Should not mint
	recipientAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin := bk.GetBalance(ctx, recipientAddr, "uzrn")
	if !coin.Amount.IsZero() {
		t.Fatalf("expected 0 uzrn after end, got %s", coin.Amount.String())
	}

	// Emission should be deactivated
	em := k.GetEmissionPeriod(ctx, "em_after")
	if em.Active {
		t.Fatal("expected emission to be deactivated after end")
	}
}

func TestBeginBlocker_InactiveEmission(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	emission := &types.EmissionPeriod{
		Id:             "em_inactive",
		StartBlock:     50,
		EndBlock:       200,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
		Active:         false, // inactive
		TotalEmitted:   "0",
	}
	k.SetEmissionPeriod(ctx, emission)

	k.BeginBlocker(ctx)

	recipientAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin := bk.GetBalance(ctx, recipientAddr, "uzrn")
	if !coin.Amount.IsZero() {
		t.Fatalf("expected 0 uzrn for inactive emission, got %s", coin.Amount.String())
	}
}

func TestBeginBlocker_MultipleEmissions(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	// Two active emissions at block 100
	k.SetEmissionPeriod(ctx, &types.EmissionPeriod{
		Id:             "em_multi_1",
		StartBlock:     50,
		EndBlock:       200,
		AmountPerBlock: "100",
		Recipient:      testCreator,
		Active:         true,
		TotalEmitted:   "0",
	})
	k.SetEmissionPeriod(ctx, &types.EmissionPeriod{
		Id:             "em_multi_2",
		StartBlock:     80,
		EndBlock:       150,
		AmountPerBlock: "200",
		Recipient:      testUser1,
		Active:         true,
		TotalEmitted:   "0",
	})

	k.BeginBlocker(ctx) // block 100

	// testCreator should get 100
	creatorAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin1 := bk.GetBalance(ctx, creatorAddr, "uzrn")
	if !coin1.Amount.Equal(sdkmath.NewInt(100)) {
		t.Fatalf("expected 100 uzrn for creator, got %s", coin1.Amount.String())
	}

	// testUser1 should get 200
	user1Addr, _ := sdk.AccAddressFromBech32(testUser1)
	coin2 := bk.GetBalance(ctx, user1Addr, "uzrn")
	if !coin2.Amount.Equal(sdkmath.NewInt(200)) {
		t.Fatalf("expected 200 uzrn for user1, got %s", coin2.Amount.String())
	}
}

func TestBeginBlocker_MultipleBlocks(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)

	k.SetEmissionPeriod(ctx, &types.EmissionPeriod{
		Id:             "em_blocks",
		StartBlock:     100,
		EndBlock:       200,
		AmountPerBlock: "50",
		Recipient:      testCreator,
		Active:         true,
		TotalEmitted:   "0",
	})

	// Block 100
	k.BeginBlocker(ctx)

	// Block 101
	ctx101 := ctx.WithBlockHeader(cmtproto.Header{Height: 101})
	k.BeginBlocker(ctx101)

	// Block 102
	ctx102 := ctx.WithBlockHeader(cmtproto.Header{Height: 102})
	k.BeginBlocker(ctx102)

	// Should have emitted 3 * 50 = 150
	recipientAddr, _ := sdk.AccAddressFromBech32(testCreator)
	coin := bk.GetBalance(ctx102, recipientAddr, "uzrn")
	if !coin.Amount.Equal(sdkmath.NewInt(150)) {
		t.Fatalf("expected 150 uzrn after 3 blocks, got %s", coin.Amount.String())
	}

	em := k.GetEmissionPeriod(ctx102, "em_blocks")
	totalEmitted := new(big.Int)
	totalEmitted.SetString(em.TotalEmitted, 10)
	if totalEmitted.Cmp(big.NewInt(150)) != 0 {
		t.Fatalf("expected total emitted 150, got %s", totalEmitted.String())
	}
}

// -----------------------------------------------------------------------
// Query Tests
// -----------------------------------------------------------------------

func TestEmissionPeriod_Query(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	qsrv := keeper.NewQueryServerImpl(*k)

	resp, _ := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority:      testAuthority,
		StartBlock:     200,
		EndBlock:       500,
		AmountPerBlock: "1000",
		Recipient:      testCreator,
	})

	qResp, err := qsrv.EmissionPeriod(ctx, &types.QueryEmissionPeriodRequest{
		EmissionId: resp.EmissionId,
	})
	if err != nil {
		t.Fatalf("EmissionPeriod query failed: %v", err)
	}
	if qResp.Emission == nil {
		t.Fatal("expected non-nil emission")
	}
	if qResp.Emission.AmountPerBlock != "1000" {
		t.Fatalf("expected amount_per_block 1000, got %s", qResp.Emission.AmountPerBlock)
	}
}

func TestEmissionPeriods_Query(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)
	qsrv := keeper.NewQueryServerImpl(*k)

	// Create two emissions
	_, _ = srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority: testAuthority, StartBlock: 200, EndBlock: 500,
		AmountPerBlock: "1000", Recipient: testCreator,
	})
	resp2, _ := srv.CreateEmissionPeriod(ctx, &types.MsgCreateEmissionPeriod{
		Authority: testAuthority, StartBlock: 300, EndBlock: 600,
		AmountPerBlock: "2000", Recipient: testUser1,
	})

	// Cancel the second one
	_, _ = srv.CancelEmissionPeriod(ctx, &types.MsgCancelEmissionPeriod{
		Authority: testAuthority, EmissionId: resp2.EmissionId,
	})

	// Query all
	allResp, err := qsrv.EmissionPeriods(ctx, &types.QueryEmissionPeriodsRequest{
		ActiveOnly: false,
	})
	if err != nil {
		t.Fatalf("EmissionPeriods query failed: %v", err)
	}
	if len(allResp.Emissions) != 2 {
		t.Fatalf("expected 2 emissions, got %d", len(allResp.Emissions))
	}

	// Query active only
	activeResp, err := qsrv.EmissionPeriods(ctx, &types.QueryEmissionPeriodsRequest{
		ActiveOnly: true,
	})
	if err != nil {
		t.Fatalf("active-only query failed: %v", err)
	}
	if len(activeResp.Emissions) != 1 {
		t.Fatalf("expected 1 active emission, got %d", len(activeResp.Emissions))
	}
}

// -----------------------------------------------------------------------
// UpdateParams
// -----------------------------------------------------------------------

func TestUpdateParams_Success(t *testing.T) {
	k, ctx, srv := setupMsgServer(t)

	newParams := &types.Params{
		EmissionEpochBlocks: 100,
		DefaultFeeBps:       "30",
	}
	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testAuthority,
		Params:    newParams,
	})
	if err != nil {
		t.Fatalf("UpdateParams failed: %v", err)
	}

	got := k.GetParams(ctx)
	if got.EmissionEpochBlocks != 100 {
		t.Fatalf("expected EmissionEpochBlocks 100, got %d", got.EmissionEpochBlocks)
	}
}

func TestUpdateParams_Unauthorized(t *testing.T) {
	_, ctx, srv := setupMsgServer(t)

	_, err := srv.UpdateParams(ctx, &types.MsgUpdateParams{
		Authority: testCreator,
		Params:    &types.Params{EmissionEpochBlocks: 100},
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// -----------------------------------------------------------------------
// ValidateBasic Tests
// -----------------------------------------------------------------------

func TestValidateBasic_CreateEmissionPeriod(t *testing.T) {
	valid := &types.MsgCreateEmissionPeriod{
		Authority: testAuthority, StartBlock: 100, EndBlock: 200,
		AmountPerBlock: "1000", Recipient: testCreator,
	}
	if err := valid.ValidateBasic(); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	// EndBlock <= StartBlock
	invalid := &types.MsgCreateEmissionPeriod{
		Authority: testAuthority, StartBlock: 200, EndBlock: 100,
		AmountPerBlock: "1000", Recipient: testCreator,
	}
	if err := invalid.ValidateBasic(); err == nil {
		t.Fatal("expected error for end <= start")
	}

	// Zero amount
	zeroAmt := &types.MsgCreateEmissionPeriod{
		Authority: testAuthority, StartBlock: 100, EndBlock: 200,
		AmountPerBlock: "0", Recipient: testCreator,
	}
	if err := zeroAmt.ValidateBasic(); err == nil {
		t.Fatal("expected error for zero amount")
	}

	// Empty authority
	empty := &types.MsgCreateEmissionPeriod{
		Authority: "", StartBlock: 100, EndBlock: 200,
		AmountPerBlock: "1000", Recipient: testCreator,
	}
	if err := empty.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty authority")
	}
}

func TestValidateBasic_CancelEmissionPeriod(t *testing.T) {
	valid := &types.MsgCancelEmissionPeriod{
		Authority: testAuthority, EmissionId: "em1",
	}
	if err := valid.ValidateBasic(); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	empty := &types.MsgCancelEmissionPeriod{
		Authority: testAuthority, EmissionId: "",
	}
	if err := empty.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty emission_id")
	}

	noAuth := &types.MsgCancelEmissionPeriod{
		Authority: "", EmissionId: "em1",
	}
	if err := noAuth.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty authority")
	}
}

func TestValidateBasic_UpdateParams(t *testing.T) {
	valid := &types.MsgUpdateParams{
		Authority: testAuthority,
		Params:    &types.Params{EmissionEpochBlocks: 100},
	}
	if err := valid.ValidateBasic(); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	// Nil params is valid (no-op)
	nilParams := &types.MsgUpdateParams{
		Authority: testAuthority,
		Params:    nil,
	}
	if err := nilParams.ValidateBasic(); err != nil {
		t.Fatalf("nil params should be valid: %v", err)
	}

	empty := &types.MsgUpdateParams{Authority: ""}
	if err := empty.ValidateBasic(); err == nil {
		t.Fatal("expected error for empty authority")
	}
}
