package keeper_test

import (
	"crypto/sha256"
	"strings"
	"testing"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/private_corpus/keeper"
	"github.com/zerone-chain/zerone/x/private_corpus/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

func testAddr(name string) string {
	h := sha256.Sum256([]byte("private_corpus_test:" + name))
	return sdk.AccAddress(h[:20]).String()
}

func setupKeeper(t *testing.T) (keeper.Keeper, types.MsgServer, types.QueryServer, sdk.Context) {
	t.Helper()
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load store: %v", err)
	}
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(storeService, cdc, testAddr("authority"))
	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100, ChainID: "zerone-test-1"}, false, log.NewNopLogger())
	return k, keeper.NewMsgServerImpl(k), keeper.NewQueryServerImpl(k), ctx
}

func validHash() string {
	// 32-byte all-zero hash, hex-encoded, valid for the chain's check.
	return strings.Repeat("00", 32)
}

func TestRegisterVault_Roundtrip(t *testing.T) {
	k, ms, qs, ctx := setupKeeper(t)
	op := testAddr("operator1")

	resp, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator:        op,
		Id:              "love-corpus",
		DisplayName:     "Love Corpus",
		Description:     "Curated corpus for the model.",
		AccessPolicyUrl: "https://example.org/policy",
		OperatorPubkey:  "ed25519:base64key",
		ServerEndpoint:  "https://vault.example.org",
	})
	if err != nil {
		t.Fatalf("RegisterVault: %v", err)
	}
	if resp.VaultId != "love-corpus" {
		t.Fatalf("expected vault id love-corpus, got %s", resp.VaultId)
	}

	// Direct keeper read.
	v, ok := k.GetVault(ctx, "love-corpus")
	if !ok {
		t.Fatalf("vault not stored")
	}
	if v.Operator != op {
		t.Fatalf("operator mismatch: %q vs %q", v.Operator, op)
	}
	if v.Status != types.VaultStatus_VAULT_STATUS_ACTIVE {
		t.Fatalf("expected ACTIVE status, got %s", v.Status)
	}

	// Query server read.
	qResp, err := qs.Vault(ctx, &types.QueryVaultRequest{Id: "love-corpus"})
	if err != nil {
		t.Fatalf("Query.Vault: %v", err)
	}
	if qResp.Vault == nil || qResp.Vault.Id != "love-corpus" {
		t.Fatalf("query did not return vault")
	}
}

func TestRegisterVault_DuplicateRejected(t *testing.T) {
	_, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	msg := &types.MsgRegisterVault{
		Operator: op, Id: "love-corpus", DisplayName: "x",
		OperatorPubkey: "k",
	}
	if _, err := ms.RegisterVault(ctx, msg); err != nil {
		t.Fatalf("first RegisterVault: %v", err)
	}
	if _, err := ms.RegisterVault(ctx, msg); err == nil {
		t.Fatalf("expected duplicate vault rejection")
	}
}

func TestUpdateVault_OnlyOperator(t *testing.T) {
	_, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	other := testAddr("operator2")
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault1", DisplayName: "a", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.UpdateVault(ctx, &types.MsgUpdateVault{
		Operator: other, VaultId: "vault1", DisplayName: "hijack",
	}); err == nil {
		t.Fatalf("expected non-operator update to be rejected")
	}
	if _, err := ms.UpdateVault(ctx, &types.MsgUpdateVault{
		Operator: op, VaultId: "vault1", DisplayName: "renamed",
	}); err != nil {
		t.Fatalf("operator update: %v", err)
	}
}

func TestPublishManifest_RequiresActiveVault(t *testing.T) {
	_, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault1", DisplayName: "a", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	// Pause the vault.
	if _, err := ms.UpdateVault(ctx, &types.MsgUpdateVault{
		Operator: op, VaultId: "vault1", Status: types.VaultStatus_VAULT_STATUS_PAUSED,
	}); err != nil {
		t.Fatal(err)
	}
	// Publish should be rejected.
	if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
		Operator:    op, VaultId: "vault1", ManifestId: "vault1#1",
		Version: "1.0", ContentHash: validHash(),
	}); err == nil {
		t.Fatalf("expected publish to be rejected when vault paused")
	}
	// Reactivate, publish should succeed.
	if _, err := ms.UpdateVault(ctx, &types.MsgUpdateVault{
		Operator: op, VaultId: "vault1", Status: types.VaultStatus_VAULT_STATUS_ACTIVE,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
		Operator:    op, VaultId: "vault1", ManifestId: "vault1#1",
		Version: "1.0", ContentHash: validHash(),
	}); err != nil {
		t.Fatalf("publish after reactivate: %v", err)
	}
}

func TestWithdrawManifest_StatusTransition(t *testing.T) {
	k, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault1", DisplayName: "a", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
		Operator: op, VaultId: "vault1", ManifestId: "vault1#1",
		Version: "1.0", ContentHash: validHash(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.WithdrawManifest(ctx, &types.MsgWithdrawManifest{
		Operator: op, ManifestId: "vault1#1", Reason: "data corruption found",
	}); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	mf, ok := k.GetManifest(ctx, "vault1#1")
	if !ok || mf.Status != types.ManifestStatus_MANIFEST_STATUS_WITHDRAWN {
		t.Fatalf("expected manifest WITHDRAWN, got %v", mf)
	}
	// Double-withdraw should fail.
	if _, err := ms.WithdrawManifest(ctx, &types.MsgWithdrawManifest{
		Operator: op, ManifestId: "vault1#1", Reason: "x",
	}); err == nil {
		t.Fatalf("expected double-withdraw to fail")
	}
}

func TestRecordAccess_MonotonicSeq(t *testing.T) {
	_, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault1", DisplayName: "a", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	r1, err := ms.RecordAccess(ctx, &types.MsgRecordAccess{Operator: op, VaultId: "vault1", Note: "n1"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := ms.RecordAccess(ctx, &types.MsgRecordAccess{Operator: op, VaultId: "vault1", Note: "n2"})
	if err != nil {
		t.Fatal(err)
	}
	r3, err := ms.RecordAccess(ctx, &types.MsgRecordAccess{Operator: op, VaultId: "vault1", Note: "n3"})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Seq >= r2.Seq || r2.Seq >= r3.Seq {
		t.Fatalf("seq not monotonic: %d %d %d", r1.Seq, r2.Seq, r3.Seq)
	}
}

func TestRecordAccess_RejectsManifestVaultMismatch(t *testing.T) {
	_, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	for _, id := range []string{"vault1", "vault2"} {
		if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
			Operator: op, Id: id, DisplayName: id, OperatorPubkey: "k",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
			Operator: op, VaultId: id, ManifestId: id + "#1",
			Version: "1.0", ContentHash: validHash(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Try to log access in vault1 referring to vault2's manifest.
	if _, err := ms.RecordAccess(ctx, &types.MsgRecordAccess{
		Operator: op, VaultId: "vault1", ManifestId: "vault2#1",
	}); err == nil {
		t.Fatalf("expected manifest-vault mismatch rejection")
	}
}

func TestQueryManifestsByVault_PaginatesAndFilters(t *testing.T) {
	_, ms, qs, ctx := setupKeeper(t)
	op := testAddr("operator1")
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault1", DisplayName: "a", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault2", DisplayName: "b", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
			Operator: op, VaultId: "vault1", ManifestId: vaultManifestID("vault1", i),
			Version: "1.0", ContentHash: validHash(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Should not appear in vault1's list.
	if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
		Operator: op, VaultId: "vault2", ManifestId: "vault2#1",
		Version: "1.0", ContentHash: validHash(),
	}); err != nil {
		t.Fatal(err)
	}
	resp, err := qs.ManifestsByVault(ctx, &types.QueryManifestsByVaultRequest{VaultId: "vault1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Manifests) != 3 {
		t.Fatalf("expected 3 manifests in vault1, got %d", len(resp.Manifests))
	}
	for _, m := range resp.Manifests {
		if m.VaultId != "vault1" {
			t.Fatalf("manifest leaked from another vault: %s", m.Id)
		}
	}
}

func TestInvalidContentHashRejected(t *testing.T) {
	_, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault1", DisplayName: "a", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
		Operator: op, VaultId: "vault1", ManifestId: "vault1#1",
		Version: "1.0", ContentHash: "not-hex",
	}); err == nil {
		t.Fatalf("expected non-hex content hash to be rejected")
	}
	if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
		Operator: op, VaultId: "vault1", ManifestId: "vault1#1",
		Version: "1.0", ContentHash: "deadbeef", // 4 bytes — too short
	}); err == nil {
		t.Fatalf("expected too-short content hash to be rejected")
	}
}

func TestGenesisRoundTrip(t *testing.T) {
	k, ms, _, ctx := setupKeeper(t)
	op := testAddr("operator1")
	if _, err := ms.RegisterVault(ctx, &types.MsgRegisterVault{
		Operator: op, Id: "vault1", DisplayName: "a", OperatorPubkey: "k",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.PublishManifest(ctx, &types.MsgPublishManifest{
		Operator: op, VaultId: "vault1", ManifestId: "vault1#1",
		Version: "1.0", ContentHash: validHash(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.RecordAccess(ctx, &types.MsgRecordAccess{
		Operator: op, VaultId: "vault1", Note: "n",
	}); err != nil {
		t.Fatal(err)
	}
	gs := k.ExportGenesis(ctx)
	if err := gs.Validate(); err != nil {
		t.Fatalf("exported genesis invalid: %v", err)
	}
	if len(gs.Vaults) != 1 || len(gs.Manifests) != 1 || len(gs.AccessRecords) != 1 {
		t.Fatalf("unexpected genesis content: %+v", gs)
	}
	if gs.NextAccessSeq <= 1 {
		t.Fatalf("next_access_seq did not advance: %d", gs.NextAccessSeq)
	}
}

func vaultManifestID(vault string, n int) string {
	return vault + "#" + intToStr(n)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	x := n
	for x > 0 {
		digits = append([]byte{byte('0' + x%10)}, digits...)
		x /= 10
	}
	return string(digits)
}
