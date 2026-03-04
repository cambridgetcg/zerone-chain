package keeper_test

import (
	"bytes"
	"context"
	"os"
	"sort"
	"testing"

	"cosmossdk.io/core/store"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
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

// ─── setupKeeper ────────────────────────────────────────────────────────────

func setupKeeper(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	ss := newMockStoreService()
	k := keeper.NewKeeper(ss, nil, "authority", nil, nil)
	ctx := sdk.Context{}.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	return k, ctx
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
