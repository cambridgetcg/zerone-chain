package keeper_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Mock Bank Keeper ────────────────────────────────────────────────────────

type trackingBankKeeper struct {
	balances map[string]sdk.Coins // addr → coins
	minted   sdk.Coins
	burned   sdk.Coins
	// Module account balances
	moduleBalances map[string]sdk.Coins
	// Track calls for assertions
	sendCalls []sendRecord
}

type sendRecord struct {
	from, to string
	coins    sdk.Coins
}

func newTrackingBankKeeper() *trackingBankKeeper {
	return &trackingBankKeeper{
		balances:       make(map[string]sdk.Coins),
		moduleBalances: make(map[string]sdk.Coins),
	}
}

func (bk *trackingBankKeeper) SendCoins(_ context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error {
	bk.sendCalls = append(bk.sendCalls, sendRecord{fromAddr.String(), toAddr.String(), amt})
	return nil
}

func (bk *trackingBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error {
	bk.sendCalls = append(bk.sendCalls, sendRecord{senderModule, recipientAddr.String(), amt})
	return nil
}

func (bk *trackingBankKeeper) SendCoinsFromAccountToModule(_ context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error {
	bk.sendCalls = append(bk.sendCalls, sendRecord{senderAddr.String(), recipientModule, amt})
	return nil
}

func (bk *trackingBankKeeper) SendCoinsFromModuleToModule(_ context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	bk.sendCalls = append(bk.sendCalls, sendRecord{senderModule, recipientModule, amt})
	return nil
}

func (bk *trackingBankKeeper) BurnCoins(_ context.Context, _ string, amt sdk.Coins) error {
	bk.burned = bk.burned.Add(amt...)
	return nil
}

func (bk *trackingBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if coins, ok := bk.balances[addr.String()]; ok {
		return sdk.NewCoin(denom, coins.AmountOf(denom))
	}
	return sdk.NewInt64Coin(denom, 0)
}

// ─── Mock Staking Keeper ────────────────────────────────────────────────────

type trackingStakingKeeper struct {
	validators map[string]*types.ValidatorInfo
	totalStake uint64
	slashes    []slashRecord
}

type slashRecord struct {
	Validator string
	SlashBps  uint64
}

func newTrackingStakingKeeper() *trackingStakingKeeper {
	return &trackingStakingKeeper{
		validators: make(map[string]*types.ValidatorInfo),
		totalStake: 1_000_000,
	}
}

func (sk *trackingStakingKeeper) GetActiveValidatorInfos(_ context.Context) ([]types.ValidatorInfo, error) {
	vals := make([]types.ValidatorInfo, 0, len(sk.validators))
	for _, v := range sk.validators {
		vals = append(vals, types.ValidatorInfo{
			Address:           v.Address,
			Stake:             v.Stake,
			Tier:              v.Tier,
			VerificationCount: v.VerificationCount,
			AccuracyBps:       v.AccuracyBps,
		})
	}
	return vals, nil
}

func (sk *trackingStakingKeeper) GetValidatorInfo(_ context.Context, addr string) (*types.ValidatorInfo, error) {
	v, ok := sk.validators[addr]
	if !ok {
		return &types.ValidatorInfo{Address: addr, Stake: 100_000}, nil
	}
	return v, nil
}

func (sk *trackingStakingKeeper) GetEffectiveStake(_ context.Context, addr string) (uint64, error) {
	if v, ok := sk.validators[addr]; ok {
		return v.Stake, nil
	}
	return 100_000, nil
}

func (sk *trackingStakingKeeper) GetTotalStake(_ context.Context) (uint64, error) {
	return sk.totalStake, nil
}

func (sk *trackingStakingKeeper) SlashValidator(_ context.Context, addr string, slashBps uint64) error {
	sk.slashes = append(sk.slashes, slashRecord{Validator: addr, SlashBps: slashBps})
	return nil
}

func (sk *trackingStakingKeeper) addValidator(addr string, stake uint64, tier string) {
	sk.validators[addr] = &types.ValidatorInfo{
		Address:           addr,
		Stake:             stake,
		Tier:              tier,
		VerificationCount: 0,
		AccuracyBps:       800_000,
	}
}

// ─── Setup Functions ─────────────────────────────────────────────────────────

// setupKnowledgeTest creates a Keeper with basic mock dependencies.
func setupKnowledgeTest(t *testing.T) (keeper.Keeper, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	bk := newTrackingBankKeeper()
	sk := newTrackingStakingKeeper()

	k := keeper.NewKeeper(
		runtime.NewKVStoreService(storeKey),
		cdc,
		"zrn1authority",
		bk,
		sk,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	require.NoError(t, k.InitGenesis(ctx, types.DefaultGenesis()))

	return k, ctx
}

// setupKnowledgeTestWithBank creates a Keeper and returns the bank keeper for tracking.
func setupKnowledgeTestWithBank(t *testing.T) (keeper.Keeper, sdk.Context, *trackingBankKeeper) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	bk := newTrackingBankKeeper()
	sk := newTrackingStakingKeeper()

	k := keeper.NewKeeper(
		runtime.NewKVStoreService(storeKey),
		cdc,
		"zrn1authority",
		bk,
		sk,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	require.NoError(t, k.InitGenesis(ctx, types.DefaultGenesis()))

	return k, ctx, bk
}

// setupKnowledgeTestFull creates a Keeper and returns both bank and staking keepers.
func setupKnowledgeTestFull(t *testing.T) (keeper.Keeper, sdk.Context, *trackingBankKeeper, *trackingStakingKeeper) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	bk := newTrackingBankKeeper()
	sk := newTrackingStakingKeeper()

	k := keeper.NewKeeper(
		runtime.NewKVStoreService(storeKey),
		cdc,
		"zrn1authority",
		bk,
		sk,
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	require.NoError(t, k.InitGenesis(ctx, types.DefaultGenesis()))

	return k, ctx, bk, sk
}

// ─── Workflow Helpers ────────────────────────────────────────────────────────

// advanceBlocks returns a new context with the block height advanced by n.
func advanceBlocks(ctx sdk.Context, n int64) sdk.Context {
	return ctx.WithBlockHeight(ctx.BlockHeight() + n)
}

// makeValidatorAddr returns a deterministic bech32 validator address.
func makeValidatorAddr(i int) string {
	return fmt.Sprintf("zrn1validator%d", i)
}

// makeSubmitterAddr returns a deterministic bech32 submitter address.
func makeSubmitterAddr(i int) string {
	return fmt.Sprintf("zrn1submitter%d", i)
}

// computeMsgServerCommitHash computes the simple sha256(vote || salt) hash
// used by msg_server.SubmitReveal (NOT the domain-separated commitment.go hash).
func computeMsgServerCommitHash(vote string, salt []byte) []byte {
	h := sha256.New()
	h.Write([]byte(vote))
	h.Write(salt)
	return h.Sum(nil)
}

// makeTestClaim creates and stores a claim with a verification round.
func makeTestClaim(t *testing.T, k keeper.Keeper, ctx sdk.Context, submitter, content, domain, category, stake string) (*types.Claim, *types.VerificationRound) {
	t.Helper()
	height := uint64(ctx.BlockHeight())

	contentHash := keeper.ComputeClaimContentHash(content, domain)
	claimID := keeper.GenerateClaimID(submitter, contentHash, height)

	claim := &types.Claim{
		Id:               claimID,
		FactContent:      content,
		Domain:           domain,
		Category:         category,
		Submitter:        submitter,
		SubmittedAtBlock: height,
		Status:           types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:            stake,
		ContentHash:      contentHash,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	round, err := k.CreateVerificationRound(ctx, claim)
	require.NoError(t, err)

	return claim, round
}

// makeTestFact creates and stores a fact directly (bypassing verification).
func makeTestFact(t *testing.T, k keeper.Keeper, ctx sdk.Context, id, content, domain, category, submitter string, confidence uint64) *types.Fact {
	t.Helper()
	height := uint64(ctx.BlockHeight())
	fact := &types.Fact{
		Id:                id,
		Content:           content,
		Domain:            domain,
		Category:          category,
		Confidence:        confidence,
		Submitter:         submitter,
		SubmittedAtBlock:  height,
		VerifiedAtBlock:   height,
		LastVerifiedBlock: height,
		Status:            types.FactStatus_FACT_STATUS_VERIFIED,
	}
	require.NoError(t, k.SetFact(ctx, fact))
	return fact
}

// makeRoundInPhase creates a round at a specific phase with deadlines.
func makeRoundInPhase(id, claimID string, phase types.VerificationPhase, startBlock uint64) *types.VerificationRound {
	return &types.VerificationRound{
		Id:                  id,
		ClaimId:             claimID,
		StartedAtBlock:      startBlock,
		Phase:               phase,
		CommitDeadline:      startBlock + 4,
		RevealDeadline:      startBlock + 8,
		AggregationDeadline: startBlock + 11,
	}
}

// makeValidBech32Addr returns a valid bech32 address with the "zrn" prefix.
// The seed string is padded/truncated to 20 bytes for the address.
func makeValidBech32Addr(seed string) string {
	addrBytes := make([]byte, 20)
	copy(addrBytes, []byte(seed))
	return sdk.AccAddress(addrBytes).String()
}
