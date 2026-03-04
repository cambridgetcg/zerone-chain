package keeper_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"testing"

	"cosmossdk.io/core/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const testAddr = "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqulc3kt"

// ─── mockKVStore ────────────────────────────────────────────────────────────

type mockKVStore struct {
	data map[string][]byte
}

func newMockKVStore() *mockKVStore {
	return &mockKVStore{data: make(map[string][]byte)}
}

func (m *mockKVStore) Get(key []byte) ([]byte, error) {
	v, ok := m.data[string(key)]
	if !ok {
		return nil, nil
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, nil
}

func (m *mockKVStore) Has(key []byte) (bool, error) {
	_, ok := m.data[string(key)]
	return ok, nil
}

func (m *mockKVStore) Set(key, value []byte) error {
	cp := make([]byte, len(value))
	copy(cp, value)
	m.data[string(key)] = cp
	return nil
}

func (m *mockKVStore) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

func (m *mockKVStore) Iterator(start, end []byte) (store.Iterator, error) {
	return newMockIterator(m.data, start, end, false), nil
}

func (m *mockKVStore) ReverseIterator(start, end []byte) (store.Iterator, error) {
	return newMockIterator(m.data, start, end, true), nil
}

// ─── mockIterator ───────────────────────────────────────────────────────────

type mockIterator struct {
	keys   [][]byte
	values [][]byte
	pos    int
}

func newMockIterator(data map[string][]byte, start, end []byte, reverse bool) *mockIterator {
	var keys []string
	for k := range data {
		kb := []byte(k)
		if start != nil && bytes.Compare(kb, start) < 0 {
			continue
		}
		if end != nil && bytes.Compare(kb, end) >= 0 {
			continue
		}
		keys = append(keys, k)
	}
	if reverse {
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	} else {
		sort.Strings(keys)
	}

	it := &mockIterator{
		keys:   make([][]byte, len(keys)),
		values: make([][]byte, len(keys)),
	}
	for i, k := range keys {
		it.keys[i] = []byte(k)
		v := data[k]
		cp := make([]byte, len(v))
		copy(cp, v)
		it.values[i] = cp
	}
	return it
}

func (it *mockIterator) Domain() ([]byte, []byte) { return nil, nil }
func (it *mockIterator) Valid() bool               { return it.pos < len(it.keys) }
func (it *mockIterator) Next()                     { it.pos++ }
func (it *mockIterator) Key() []byte               { return it.keys[it.pos] }
func (it *mockIterator) Value() []byte             { return it.values[it.pos] }
func (it *mockIterator) Error() error              { return nil }
func (it *mockIterator) Close() error              { return nil }

// ─── mockStoreService ───────────────────────────────────────────────────────

type mockStoreService struct {
	kvStore *mockKVStore
}

func newMockStoreService() *mockStoreService {
	return &mockStoreService{kvStore: newMockKVStore()}
}

func (m *mockStoreService) OpenKVStore(_ context.Context) store.KVStore {
	return m.kvStore
}

// ─── mockBankKeeper ─────────────────────────────────────────────────────────

type bankTransfer struct {
	from   string
	to     string
	amount sdk.Coins
}

type mockBankKeeper struct {
	accountToModuleCalls []bankTransfer
	moduleToModuleCalls  []bankTransfer
	failNextSend         bool
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{}
}

func (m *mockBankKeeper) SendCoins(_ context.Context, _, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, sender sdk.AccAddress, module string, amt sdk.Coins) error {
	if m.failNextSend {
		m.failNextSend = false
		return fmt.Errorf("insufficient funds")
	}
	m.accountToModuleCalls = append(m.accountToModuleCalls, bankTransfer{from: sender.String(), to: module, amount: amt})
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, module string, recipient sdk.AccAddress, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToModule(_ context.Context, from, to string, amt sdk.Coins) error {
	if m.failNextSend {
		m.failNextSend = false
		return fmt.Errorf("insufficient module funds")
	}
	m.moduleToModuleCalls = append(m.moduleToModuleCalls, bankTransfer{from: from, to: to, amount: amt})
	return nil
}

func (m *mockBankKeeper) MintCoins(_ context.Context, _ string, _ sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) GetBalance(_ context.Context, _ sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewInt64Coin(denom, 0)
}

// ─── setupKeeper ────────────────────────────────────────────────────────────

func setupKeeper(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	ss := newMockStoreService()
	bk := newMockBankKeeper()
	k := keeper.NewKeeper(ss, nil, "authority", bk, nil)
	ctx := sdk.Context{}.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	return k, ctx
}

// setupKeeperWithBank returns the keeper and the mock bank for tests that need to inspect calls.
func setupKeeperWithBank(t *testing.T) (keeper.Keeper, context.Context, *mockBankKeeper) {
	t.Helper()
	ss := newMockStoreService()
	bk := newMockBankKeeper()
	k := keeper.NewKeeper(ss, nil, "authority", bk, nil)
	ctx := sdk.Context{}.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	return k, ctx, bk
}

// setupDefaultDomains creates active domains for testing.
func setupDefaultDomains(t *testing.T, k keeper.Keeper, ctx context.Context) {
	t.Helper()
	for _, name := range []string{"technology", "science", "culture", "creative"} {
		require.NoError(t, k.SetDomain(ctx, &types.Domain{
			Name:   name,
			Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		}))
	}
}

// ─── TestMain ───────────────────────────────────────────────────────────────

func TestMain(m *testing.M) {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("zrn", "zrnpub")
	cfg.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
	cfg.Seal()

	os.Exit(m.Run())
}
