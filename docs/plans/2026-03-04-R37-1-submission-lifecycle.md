# R37-1 Submission Lifecycle Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement keeper methods for submitting training data — SubmitData, SubmitThread, content hashing, duplicate detection, consent validation, stake locking, and sponsored submissions.

**Architecture:** All state operations follow the existing `state.go` pattern (proto marshal with `Deterministic: true`, KVStore via `storeService`). New submission indexes use unused key prefixes 0x11 (domain→submission) and 0x12 (submitter→submission). Sequence counter at 0x81 generates hex submission IDs. Quality round initiation is stubbed (R37-2 scope).

**Tech Stack:** Go 1.24, Cosmos SDK v0.50.15, protobuf, crypto/sha256

---

### Task 1: Add Submission Index Key Constructors

**Files:**
- Modify: `x/knowledge/types/keys.go`

**Step 1: Add submission-specific index prefixes and key constructors**

Add after the existing index prefixes (around line 41):

```go
// ─── Submission indexes ─────────────────────────────────────────────────
SubmissionDomainIndexPrefix    = []byte{0x11} // domain/submissionID → exists
SubmissionSubmitterIndexPrefix = []byte{0x12} // submitter/submissionID → exists
```

Add key constructors after the existing ones (after `ContentHashKey`):

```go
// SubmissionDomainIndexKey returns the index key for a submission within a domain.
func SubmissionDomainIndexKey(domain, submissionID string) []byte {
	key := append(append([]byte{}, SubmissionDomainIndexPrefix...), []byte(domain)...)
	key = append(key, '/')
	return append(key, []byte(submissionID)...)
}

// SubmissionDomainByDomainPrefix returns the prefix for iterating submissions in a domain.
func SubmissionDomainByDomainPrefix(domain string) []byte {
	key := append(append([]byte{}, SubmissionDomainIndexPrefix...), []byte(domain)...)
	return append(key, '/')
}

// SubmissionSubmitterIndexKey returns the index key for a submission by submitter.
func SubmissionSubmitterIndexKey(submitter, submissionID string) []byte {
	key := append(append([]byte{}, SubmissionSubmitterIndexPrefix...), []byte(submitter)...)
	key = append(key, '/')
	return append(key, []byte(submissionID)...)
}

// SubmissionSubmitterBySubmitterPrefix returns the prefix for iterating submissions by submitter.
func SubmissionSubmitterBySubmitterPrefix(submitter string) []byte {
	key := append(append([]byte{}, SubmissionSubmitterIndexPrefix...), []byte(submitter)...)
	return append(key, '/')
}
```

**Step 2: Verify no key prefix collisions**

Run: `cd /Users/yournameisai/Desktop/zerone && go vet ./x/knowledge/types/...`
Expected: PASS (no errors)

**Step 3: Commit**

```bash
git add x/knowledge/types/keys.go
git commit -m "feat(knowledge): add submission index key constructors (R37-1)"
```

---

### Task 2: Implement Submission CRUD in state.go

**Files:**
- Modify: `x/knowledge/keeper/state.go`

**Step 1: Write the failing test**

Create: `x/knowledge/keeper/state_test.go`

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestSubmissionCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	sub := &types.Submission{
		Id:        "sub-1",
		Submitter: testAddr,
		Content:   "test content",
		Domain:    "technology",
		Status:    types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}

	// Set
	err := k.SetSubmission(ctx, sub)
	require.NoError(t, err)

	// Get
	got, found := k.GetSubmission(ctx, "sub-1")
	require.True(t, found)
	require.Equal(t, sub.Id, got.Id)
	require.Equal(t, sub.Content, got.Content)

	// Get missing
	_, found = k.GetSubmission(ctx, "missing")
	require.False(t, found)

	// Delete
	err = k.DeleteSubmission(ctx, "sub-1")
	require.NoError(t, err)
	_, found = k.GetSubmission(ctx, "sub-1")
	require.False(t, found)
}

func TestContentHashIndex(t *testing.T) {
	k, ctx := setupKeeper(t)

	err := k.SetContentHash(ctx, "abc123", "sub-1")
	require.NoError(t, err)

	require.True(t, k.HasContentHash(ctx, "abc123"))
	require.False(t, k.HasContentHash(ctx, "missing"))
}

func TestNextSubmissionID(t *testing.T) {
	k, ctx := setupKeeper(t)

	id1 := k.NextSubmissionID(ctx)
	id2 := k.NextSubmissionID(ctx)
	require.NotEqual(t, id1, id2)
	// IDs should be hex strings
	require.NotEmpty(t, id1)
}

func TestSubmissionIterator(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i := 0; i < 3; i++ {
		sub := &types.Submission{
			Id:      fmt.Sprintf("sub-%d", i),
			Domain:  "technology",
			Content: fmt.Sprintf("content-%d", i),
		}
		require.NoError(t, k.SetSubmission(ctx, sub))
	}

	var count int
	k.IterateSubmissions(ctx, func(s *types.Submission) bool {
		count++
		return false
	})
	require.Equal(t, 3, count)
}

func TestSubmissionsByDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Index two submissions under "science"
	require.NoError(t, k.SetSubmissionDomainIndex(ctx, "science", "sub-1"))
	require.NoError(t, k.SetSubmissionDomainIndex(ctx, "science", "sub-2"))
	require.NoError(t, k.SetSubmissionDomainIndex(ctx, "technology", "sub-3"))

	ids := k.GetSubmissionsByDomain(ctx, "science")
	require.Len(t, ids, 2)
}

func TestSubmissionsBySubmitter(t *testing.T) {
	k, ctx := setupKeeper(t)

	require.NoError(t, k.SetSubmissionSubmitterIndex(ctx, testAddr, "sub-1"))
	require.NoError(t, k.SetSubmissionSubmitterIndex(ctx, testAddr, "sub-2"))

	ids := k.GetSubmissionsBySubmitter(ctx, testAddr)
	require.Len(t, ids, 2)
}
```

**Step 2: Write test helper setup**

Create: `x/knowledge/keeper/keeper_test.go`

```go
package keeper_test

import (
	"fmt"
	"os"
	"testing"

	"cosmossdk.io/core/store"
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const testAddr = "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqulc3kt"
const testAddr2 = "zrn1qyqs2220qlqgjnvnscpzgjqene5yxqag4mvuq"

func TestMain(m *testing.M) {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
	os.Exit(m.Run())
}

// mockStoreService implements store.KVStoreService with an in-memory map.
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
	return newMockIterator(m.data, start, end), nil
}

func (m *mockKVStore) ReverseIterator(start, end []byte) (store.Iterator, error) {
	return newMockIterator(m.data, start, end), nil
}

// mockIterator iterates over sorted keys in range [start, end).
type mockIterator struct {
	keys   []string
	values [][]byte
	pos    int
}

func newMockIterator(data map[string][]byte, start, end []byte) *mockIterator {
	var keys []string
	var values [][]byte
	for k, v := range data {
		if (start == nil || k >= string(start)) && (end == nil || k < string(end)) {
			keys = append(keys, k)
			values = append(values, v)
		}
	}
	// Sort keys
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
				values[i], values[j] = values[j], values[i]
			}
		}
	}
	return &mockIterator{keys: keys, values: values, pos: 0}
}

func (it *mockIterator) Domain() []byte  { return nil }
func (it *mockIterator) Valid() bool      { return it.pos < len(it.keys) }
func (it *mockIterator) Next()            { it.pos++ }
func (it *mockIterator) Key() []byte      { return []byte(it.keys[it.pos]) }
func (it *mockIterator) Value() []byte    { return it.values[it.pos] }
func (it *mockIterator) Error() error     { return nil }
func (it *mockIterator) Close() error     { return nil }

type mockStoreService struct {
	store *mockKVStore
}

func (m *mockStoreService) OpenKVStore(_ interface{}) store.KVStore {
	return m.store
}

// mockBankKeeper implements types.BankKeeper for tests.
type mockBankKeeper struct {
	balances map[string]sdk.Coins
	modules  map[string]sdk.Coins
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{
		balances: make(map[string]sdk.Coins),
		modules:  make(map[string]sdk.Coins),
	}
}

func (m *mockBankKeeper) SendCoins(_ sdk.Context, from, to sdk.AccAddress, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) SendCoinsFromAccountToModule(_ sdk.Context, sender sdk.AccAddress, module string, amt sdk.Coins) error {
	addr := sender.String()
	bal := m.balances[addr]
	if !bal.IsAllGTE(amt) {
		return fmt.Errorf("insufficient funds")
	}
	m.balances[addr] = bal.Sub(amt...)
	m.modules[module] = m.modules[module].Add(amt...)
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ sdk.Context, module string, recipient sdk.AccAddress, amt sdk.Coins) error {
	bal := m.modules[module]
	if !bal.IsAllGTE(amt) {
		return fmt.Errorf("insufficient module funds")
	}
	m.modules[module] = bal.Sub(amt...)
	addr := recipient.String()
	m.balances[addr] = m.balances[addr].Add(amt...)
	return nil
}

func (m *mockBankKeeper) SendCoinsFromModuleToModule(_ sdk.Context, from, to string, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) MintCoins(_ sdk.Context, module string, amt sdk.Coins) error {
	return nil
}

func (m *mockBankKeeper) GetBalance(_ sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	bal := m.balances[addr.String()]
	return sdk.Coin{Denom: denom, Amount: bal.AmountOf(denom)}
}

func (m *mockBankKeeper) setBalance(addr string, coins sdk.Coins) {
	m.balances[addr] = coins
}

func (m *mockBankKeeper) setModuleBalance(module string, coins sdk.Coins) {
	m.modules[module] = coins
}

// setupKeeper creates a Keeper with mock dependencies for unit tests.
func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
	t.Helper()

	ss := &mockStoreService{store: newMockKVStore()}
	ir := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(ir)

	bk := newMockBankKeeper()
	k := keeper.NewKeeper(cdc, runtime.NewKVStoreService(nil), "authority", bk, nil)
	// Override store service for tests
	k.SetStoreServiceForTest(ss)

	ctx := sdk.Context{}.WithBlockHeight(100)
	return k, ctx
}
```

**Important note:** We need a `SetStoreServiceForTest` method since `storeService` is unexported. Add to `keeper.go`:

```go
// SetStoreServiceForTest allows tests to inject a mock store service.
func (k *Keeper) SetStoreServiceForTest(ss interface{ OpenKVStore(interface{}) store.KVStore }) {
	// This is a test-only escape hatch. We cast to our internal interface.
	k.storeService = ss.(store.KVStoreService)
}
```

Actually, the better approach is to check how the existing keeper constructor works and use the same pattern. Let me reconsider — we should use `cosmossdk.io/store/prefix` and the standard test approach. The mock store approach above will work but we need to handle the interface properly.

**Revised approach:** Use the store service interface directly. The `mockStoreService` must implement `store.KVStoreService`. Looking at the Cosmos SDK, `store.KVStoreService` has method `OpenKVStore(ctx context.Context) store.KVStore`. So our mock must accept `context.Context`.

```go
func (m *mockStoreService) OpenKVStore(_ context.Context) store.KVStore {
	return m.store
}
```

And `setupKeeper` passes `mockStoreService` as the `store.KVStoreService` to `keeper.NewKeeper`. We'll need to check the exact NewKeeper signature.

**Step 3: Implement Submission CRUD in state.go**

Add to `x/knowledge/keeper/state.go`:

```go
// ─── Submission CRUD ────────────────────────────────────────────────────────

func (k Keeper) SetSubmission(ctx context.Context, sub *types.Submission) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal submission: %w", err)
	}
	return store.Set(types.SubmissionKey(sub.Id), bz)
}

func (k Keeper) GetSubmission(ctx context.Context, id string) (*types.Submission, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SubmissionKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var sub types.Submission
	if err := proto.Unmarshal(bz, &sub); err != nil {
		return nil, false
	}
	return &sub, true
}

func (k Keeper) DeleteSubmission(ctx context.Context, id string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.SubmissionKey(id))
}

func (k Keeper) IterateSubmissions(ctx context.Context, cb func(sub *types.Submission) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.SubmissionKeyPrefix, prefixEndBytes(types.SubmissionKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var sub types.Submission
		if err := proto.Unmarshal(iter.Value(), &sub); err != nil {
			continue
		}
		if cb(&sub) {
			break
		}
	}
}

// ─── Content hash index ─────────────────────────────────────────────────────

func (k Keeper) SetContentHash(ctx context.Context, hash, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.ContentHashKey(hash), []byte(submissionID))
}

func (k Keeper) HasContentHash(ctx context.Context, hash string) bool {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContentHashKey(hash))
	return err == nil && bz != nil
}

// ─── Submission indexes ─────────────────────────────────────────────────────

func (k Keeper) SetSubmissionDomainIndex(ctx context.Context, domain, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.SubmissionDomainIndexKey(domain, submissionID), []byte{0x01})
}

func (k Keeper) SetSubmissionSubmitterIndex(ctx context.Context, submitter, submissionID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.SubmissionSubmitterIndexKey(submitter, submissionID), []byte{0x01})
}

func (k Keeper) GetSubmissionsByDomain(ctx context.Context, domain string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.SubmissionDomainByDomainPrefix(domain)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var ids []string
	for ; iter.Valid(); iter.Next() {
		// Key is prefix + domain + "/" + submissionID — extract submissionID
		key := iter.Key()
		id := string(key[len(prefix):])
		ids = append(ids, id)
	}
	return ids
}

func (k Keeper) GetSubmissionsBySubmitter(ctx context.Context, submitter string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.SubmissionSubmitterBySubmitterPrefix(submitter)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var ids []string
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		id := string(key[len(prefix):])
		ids = append(ids, id)
	}
	return ids
}

// ─── Sequences ──────────────────────────────────────────────────────────────

func (k Keeper) NextSubmissionID(ctx context.Context) string {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SubmissionSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	id := fmt.Sprintf("%x", seq)
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = store.Set(types.SubmissionSeqKey, next)
	return id
}
```

Add `"encoding/binary"` to imports in state.go.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestSubmission|TestContentHash|TestNextSubmission" -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/state.go x/knowledge/keeper/state_test.go x/knowledge/keeper/keeper_test.go
git commit -m "feat(knowledge): implement Submission CRUD, indexes, sequences (R37-1)"
```

---

### Task 3: Implement Content Hashing & Consent Validation Helpers

**Files:**
- Create: `x/knowledge/keeper/submission.go`
- Create: `x/knowledge/keeper/submission_test.go`

**Step 1: Write the failing tests**

Create `x/knowledge/keeper/submission_test.go`:

```go
package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestComputeContentHash(t *testing.T) {
	k, _ := setupKeeper(t)
	hash := k.ComputeContentHash("hello world")
	// SHA-256 of "hello world" is known
	require.Equal(t, "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", hash)
	// Same input = same hash
	require.Equal(t, hash, k.ComputeContentHash("hello world"))
	// Different input = different hash
	require.NotEqual(t, hash, k.ComputeContentHash("different"))
}

func TestCheckDuplicate(t *testing.T) {
	k, ctx := setupKeeper(t)

	hash := k.ComputeContentHash("unique content")
	require.NoError(t, k.CheckDuplicate(ctx, hash))

	// Store it
	require.NoError(t, k.SetContentHash(ctx, hash, "sub-1"))

	// Now it's a duplicate
	err := k.CheckDuplicate(ctx, hash)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrDuplicateContent)
}

func TestValidateConsent(t *testing.T) {
	k, _ := setupKeeper(t)

	tests := []struct {
		name    string
		consent *types.ConsentProof
		wantErr error
	}{
		{
			name:    "nil consent",
			consent: nil,
			wantErr: types.ErrConsentRequired,
		},
		{
			name: "self authored — valid",
			consent: &types.ConsentProof{
				Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED,
			},
			wantErr: nil,
		},
		{
			name: "opt-in with signature — valid",
			consent: &types.ConsentProof{
				Type:            types.ConsentType_CONSENT_TYPE_OPT_IN,
				AuthorSignature: "sig123",
			},
			wantErr: nil,
		},
		{
			name: "opt-in with proof_uri — valid",
			consent: &types.ConsentProof{
				Type:     types.ConsentType_CONSENT_TYPE_OPT_IN,
				ProofUri: "https://example.com/consent",
			},
			wantErr: nil,
		},
		{
			name: "opt-in without proof — invalid",
			consent: &types.ConsentProof{
				Type: types.ConsentType_CONSENT_TYPE_OPT_IN,
			},
			wantErr: types.ErrInvalidConsent,
		},
		{
			name: "public license with uri — valid",
			consent: &types.ConsentProof{
				Type:     types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE,
				ProofUri: "https://example.com/license",
			},
			wantErr: nil,
		},
		{
			name: "public license without uri — invalid",
			consent: &types.ConsentProof{
				Type: types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE,
			},
			wantErr: types.ErrInvalidConsent,
		},
		{
			name: "platform TOS with uri — valid",
			consent: &types.ConsentProof{
				Type:     types.ConsentType_CONSENT_TYPE_PLATFORM_TOS,
				ProofUri: "https://example.com/tos",
			},
			wantErr: nil,
		},
		{
			name: "platform TOS without uri — invalid",
			consent: &types.ConsentProof{
				Type: types.ConsentType_CONSENT_TYPE_PLATFORM_TOS,
			},
			wantErr: types.ErrInvalidConsent,
		},
		{
			name: "fair use — valid",
			consent: &types.ConsentProof{
				Type: types.ConsentType_CONSENT_TYPE_FAIR_USE,
			},
			wantErr: nil,
		},
		{
			name: "unspecified type — invalid",
			consent: &types.ConsentProof{
				Type: types.ConsentType_CONSENT_TYPE_UNSPECIFIED,
			},
			wantErr: types.ErrInvalidConsent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := k.ValidateConsent(tc.consent)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestComputeContentHash|TestCheckDuplicate|TestValidateConsent" -v -count=1`
Expected: FAIL (methods not defined)

**Step 3: Implement the helpers**

Create `x/knowledge/keeper/submission.go`:

```go
package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ComputeContentHash returns the lowercase hex SHA-256 of content.
func (k Keeper) ComputeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// CheckDuplicate returns ErrDuplicateContent if the hash already exists.
func (k Keeper) CheckDuplicate(ctx context.Context, contentHash string) error {
	if k.HasContentHash(ctx, contentHash) {
		return types.ErrDuplicateContent
	}
	return nil
}

// ValidateConsent checks that the consent proof is present and has the required fields.
func (k Keeper) ValidateConsent(consent *types.ConsentProof) error {
	if consent == nil {
		return types.ErrConsentRequired
	}
	switch consent.Type {
	case types.ConsentType_CONSENT_TYPE_SELF_AUTHORED:
		return nil
	case types.ConsentType_CONSENT_TYPE_OPT_IN:
		if consent.AuthorSignature == "" && consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE:
		if consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_PLATFORM_TOS:
		if consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_FAIR_USE:
		return nil
	default:
		return types.ErrInvalidConsent
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestComputeContentHash|TestCheckDuplicate|TestValidateConsent" -v -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/submission.go x/knowledge/keeper/submission_test.go
git commit -m "feat(knowledge): add content hashing, dedup check, consent validation (R37-1)"
```

---

### Task 4: Implement SubmitData Handler

**Files:**
- Modify: `x/knowledge/keeper/submission.go`
- Modify: `x/knowledge/keeper/msg_server.go`

**Step 1: Write the failing tests**

Add to `x/knowledge/keeper/submission_test.go`:

```go
func TestSubmitData_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "This is valid training data content",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent: &types.ConsentProof{
			Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED,
		},
		Stake: "1000000",
	}

	resp, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.SubmissionId)

	// Verify submission was stored
	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.Equal(t, msg.Content, sub.Content)
	require.Equal(t, msg.Domain, sub.Domain)
	require.Equal(t, msg.Submitter, sub.Submitter)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING, sub.Status)
	require.NotEmpty(t, sub.ContentHash)

	// Verify content hash indexed
	require.True(t, k.HasContentHash(ctx, sub.ContentHash))

	// Verify domain index
	ids := k.GetSubmissionsByDomain(ctx, "technology")
	require.Contains(t, ids, resp.SubmissionId)

	// Verify submitter index
	ids = k.GetSubmissionsBySubmitter(ctx, testAddr)
	require.Contains(t, ids, resp.SubmissionId)
}

func TestSubmitData_ContentTooLarge(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	// MaxContentBytes default is 50_000
	bigContent := make([]byte, 50_001)
	for i := range bigContent {
		bigContent[i] = 'a'
	}

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    string(bigContent),
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrContentTooLarge)
}

func TestSubmitData_DuplicateContent(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "duplicate content test",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)

	// Submit same content again
	_, err = k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrDuplicateContent)
}

func TestSubmitData_InvalidConsent(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    nil,
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrConsentRequired)
}

func TestSubmitData_InvalidDomain(t *testing.T) {
	k, ctx := setupKeeper(t)
	// No domains set up

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "nonexistent",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestSubmitData_InsufficientStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test content",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "100", // Below minimum of 1000000
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}
```

Add `setupDefaultDomains` helper to `keeper_test.go`:

```go
func setupDefaultDomains(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
	t.Helper()
	for _, name := range []string{"technology", "science", "culture", "creative"} {
		require.NoError(t, k.SetDomain(ctx, &types.Domain{
			Name:   name,
			Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		}))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestSubmitData" -v -count=1`
Expected: FAIL

**Step 3: Implement SubmitData on the keeper**

Add to `x/knowledge/keeper/submission.go`:

```go
import (
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SubmitData handles MsgSubmitData — validates, deduplicates, stores, and indexes a submission.
func (k Keeper) SubmitData(ctx context.Context, msg *types.MsgSubmitData) (*types.MsgSubmitDataResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Validate content size
	if uint64(len(msg.Content)) > params.MaxContentBytes {
		return nil, types.ErrContentTooLarge.Wrapf("content %d bytes exceeds max %d", len(msg.Content), params.MaxContentBytes)
	}

	// 2. Compute content hash
	contentHash := k.ComputeContentHash(msg.Content)

	// 3. Check duplicates
	if err := k.CheckDuplicate(ctx, contentHash); err != nil {
		return nil, err
	}

	// 4. Validate consent
	if err := k.ValidateConsent(msg.Consent); err != nil {
		return nil, err
	}

	// 5. Validate domain exists and is active
	domain, found := k.GetDomain(ctx, msg.Domain)
	if !found {
		return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", msg.Domain)
	}
	if domain.Status != types.DomainStatus_DOMAIN_STATUS_ACTIVE {
		return nil, types.ErrInvalidDomain.Wrapf("domain %q is not active", msg.Domain)
	}

	// 6. Validate and lock stake
	stakeAmt, ok := sdkmath.NewIntFromString(msg.Stake)
	if !ok || stakeAmt.IsNegative() {
		return nil, types.ErrInsufficientStake.Wrap("invalid stake amount")
	}
	minStake, _ := sdkmath.NewIntFromString(params.MinSubmissionStake)
	if stakeAmt.LT(minStake) {
		return nil, types.ErrInsufficientStake.Wrapf("stake %s < minimum %s", msg.Stake, params.MinSubmissionStake)
	}

	// Handle sponsored submissions
	sponsored := msg.Sponsored
	if !sponsored {
		// Lock stake from submitter
		submitterAddr, _ := sdk.AccAddressFromBech32(msg.Submitter)
		stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
		if err := k.bankKeeper.SendCoinsFromAccountToModule(
			sdkCtx, submitterAddr, types.ModuleName, sdk.NewCoins(stakeCoin),
		); err != nil {
			// If submitter can't pay, check if sponsored is possible
			return nil, types.ErrInsufficientStake.Wrap(err.Error())
		}
	} else {
		// Sponsored: draw from bootstrap fund
		stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			sdkCtx, types.BootstrapFundModuleName, types.ModuleName, sdk.NewCoins(stakeCoin),
		); err != nil {
			return nil, types.ErrInsufficientStake.Wrap("bootstrap fund insufficient")
		}
	}

	// 7. Create Submission
	submissionID := k.NextSubmissionID(ctx)
	submission := &types.Submission{
		Id:                 submissionID,
		Submitter:          msg.Submitter,
		Content:            msg.Content,
		SampleType:         msg.SampleType,
		Domain:             msg.Domain,
		SourceUri:          msg.SourceUri,
		SourcePlatform:     msg.SourcePlatform,
		SourceTimestamp:     msg.SourceTimestamp,
		ParentSubmissionId: msg.ParentSubmissionId,
		ContextIds:         msg.ContextIds,
		ThreadId:           msg.ThreadId,
		Consent:            msg.Consent,
		OriginalAuthor:     msg.OriginalAuthor,
		License:            msg.License,
		Tags:               msg.Tags,
		Language:            msg.Language,
		Stake:              msg.Stake,
		SubmittedAtBlock:   uint64(sdkCtx.BlockHeight()),
		Status:             types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		ContentHash:        contentHash,
		Sponsored:          sponsored,
	}

	// 8. Store + indexes
	if err := k.SetSubmission(ctx, submission); err != nil {
		return nil, err
	}
	if err := k.SetContentHash(ctx, contentHash, submissionID); err != nil {
		return nil, err
	}
	if err := k.SetSubmissionDomainIndex(ctx, msg.Domain, submissionID); err != nil {
		return nil, err
	}
	if err := k.SetSubmissionSubmitterIndex(ctx, msg.Submitter, submissionID); err != nil {
		return nil, err
	}

	// 9. TODO(R37-2): Check DataBounty matches
	// 10. TODO(R37-2): Initiate quality round (VRF select validators, start commit phase)

	// 11. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"submit_data",
		sdk.NewAttribute("submission_id", submissionID),
		sdk.NewAttribute("submitter", msg.Submitter),
		sdk.NewAttribute("domain", msg.Domain),
		sdk.NewAttribute("content_hash", contentHash),
		sdk.NewAttribute("sponsored", strconv.FormatBool(sponsored)),
	))

	return &types.MsgSubmitDataResponse{SubmissionId: submissionID}, nil
}
```

Add `sdkmath "cosmossdk.io/math"` to imports.

**Step 4: Wire SubmitData into msg_server.go**

Replace the stub in `msg_server.go`:

```go
func (m msgServer) SubmitData(ctx context.Context, msg *types.MsgSubmitData) (*types.MsgSubmitDataResponse, error) {
	return m.keeper.SubmitData(ctx, msg)
}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestSubmitData" -v -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/submission.go x/knowledge/keeper/submission_test.go x/knowledge/keeper/msg_server.go x/knowledge/keeper/keeper_test.go
git commit -m "feat(knowledge): implement SubmitData handler with validation and indexing (R37-1)"
```

---

### Task 5: Implement SubmitThread Handler

**Files:**
- Modify: `x/knowledge/keeper/submission.go`
- Modify: `x/knowledge/keeper/msg_server.go`

**Step 1: Write the failing tests**

Add to `x/knowledge/keeper/submission_test.go`:

```go
func TestSubmitThread_Success(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-1",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{
				Content:    "First message in thread",
				SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION,
				Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			},
			{
				Content:    "Second message in thread",
				SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION,
				Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			},
			{
				Content:    "Third message in thread",
				SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION,
				Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			},
		},
	}

	resp, err := k.SubmitThread(ctx, msg)
	require.NoError(t, err)
	require.Len(t, resp.SubmissionIds, 3)
	require.Equal(t, "thread-1", resp.ThreadId)

	// Verify all submissions stored and linked
	for i, id := range resp.SubmissionIds {
		sub, found := k.GetSubmission(ctx, id)
		require.True(t, found, "submission %d not found", i)
		require.Equal(t, "thread-1", sub.ThreadId)
		require.Equal(t, "technology", sub.Domain)
		require.Equal(t, testAddr, sub.Submitter)

		// Verify parent chain
		if i > 0 {
			require.Equal(t, resp.SubmissionIds[i-1], sub.ParentSubmissionId)
		}
	}
}

func TestSubmitThread_TooLarge(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	// MaxThreadSize default is 20, create 21 items
	items := make([]*types.MsgSubmitData, 21)
	for i := range items {
		items[i] = &types.MsgSubmitData{
			Content:    fmt.Sprintf("item %d", i),
			SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION,
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		}
	}

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-big",
		Domain:    "technology",
		Stake:     "1000000",
		Items:     items,
	}

	_, err := k.SubmitThread(ctx, msg)
	require.ErrorIs(t, err, types.ErrThreadTooLarge)
}

func TestSubmitThread_DuplicateInThread(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-dup",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{
				Content:    "same content",
				SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION,
				Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			},
			{
				Content:    "same content",
				SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION,
				Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			},
		},
	}

	_, err := k.SubmitThread(ctx, msg)
	require.ErrorIs(t, err, types.ErrDuplicateContent)
}

func TestSubmitThread_InvalidDomain(t *testing.T) {
	k, ctx := setupKeeper(t)
	// No domains

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-bad",
		Domain:    "nonexistent",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "a", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "b", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	}

	_, err := k.SubmitThread(ctx, msg)
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestSubmitThread" -v -count=1`
Expected: FAIL

**Step 3: Implement SubmitThread**

Add to `x/knowledge/keeper/submission.go`:

```go
// SubmitThread handles MsgSubmitThread — validates and stores each item as a linked submission.
func (k Keeper) SubmitThread(ctx context.Context, msg *types.MsgSubmitThread) (*types.MsgSubmitThreadResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Validate thread size
	if uint64(len(msg.Items)) > params.MaxThreadSize {
		return nil, types.ErrThreadTooLarge.Wrapf("thread has %d items, max %d", len(msg.Items), params.MaxThreadSize)
	}

	// 2. Validate domain
	domain, found := k.GetDomain(ctx, msg.Domain)
	if !found {
		return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", msg.Domain)
	}
	if domain.Status != types.DomainStatus_DOMAIN_STATUS_ACTIVE {
		return nil, types.ErrInvalidDomain.Wrapf("domain %q is not active", msg.Domain)
	}

	// 3. Validate stake
	stakeAmt, ok := sdkmath.NewIntFromString(msg.Stake)
	if !ok || stakeAmt.IsNegative() {
		return nil, types.ErrInsufficientStake.Wrap("invalid stake amount")
	}
	minStake, _ := sdkmath.NewIntFromString(params.MinSubmissionStake)
	if stakeAmt.LT(minStake) {
		return nil, types.ErrInsufficientStake.Wrapf("stake %s < minimum %s", msg.Stake, params.MinSubmissionStake)
	}

	// 4. Pre-validate all items (consent + content size + dedup) before locking stake
	type itemPrep struct {
		contentHash string
	}
	preps := make([]itemPrep, len(msg.Items))
	for i, item := range msg.Items {
		if uint64(len(item.Content)) > params.MaxContentBytes {
			return nil, types.ErrContentTooLarge.Wrapf("item[%d]: content %d bytes exceeds max %d", i, len(item.Content), params.MaxContentBytes)
		}
		hash := k.ComputeContentHash(item.Content)
		if err := k.CheckDuplicate(ctx, hash); err != nil {
			return nil, types.ErrDuplicateContent.Wrapf("item[%d]: duplicate content", i)
		}
		if err := k.ValidateConsent(item.Consent); err != nil {
			return nil, err
		}
		// Check for intra-thread duplicates
		for j := 0; j < i; j++ {
			if preps[j].contentHash == hash {
				return nil, types.ErrDuplicateContent.Wrapf("item[%d] duplicates item[%d]", i, j)
			}
		}
		preps[i] = itemPrep{contentHash: hash}
	}

	// 5. Lock stake (single stake covers whole thread)
	submitterAddr, _ := sdk.AccAddressFromBech32(msg.Submitter)
	stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		sdkCtx, submitterAddr, types.ModuleName, sdk.NewCoins(stakeCoin),
	); err != nil {
		return nil, types.ErrInsufficientStake.Wrap(err.Error())
	}

	// 6. Create submissions linked via parent chain
	submissionIDs := make([]string, len(msg.Items))
	var prevID string
	for i, item := range msg.Items {
		subID := k.NextSubmissionID(ctx)
		submission := &types.Submission{
			Id:                 subID,
			Submitter:          msg.Submitter,
			Content:            item.Content,
			SampleType:         item.SampleType,
			Domain:             msg.Domain,
			SourceUri:          item.SourceUri,
			SourcePlatform:     item.SourcePlatform,
			SourceTimestamp:     item.SourceTimestamp,
			ParentSubmissionId: prevID,
			ContextIds:         item.ContextIds,
			ThreadId:           msg.ThreadId,
			Consent:            item.Consent,
			OriginalAuthor:     item.OriginalAuthor,
			License:            item.License,
			Tags:               item.Tags,
			Language:            item.Language,
			Stake:              msg.Stake, // Thread stake recorded on each item
			SubmittedAtBlock:   uint64(sdkCtx.BlockHeight()),
			Status:             types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
			ContentHash:        preps[i].contentHash,
		}

		if err := k.SetSubmission(ctx, submission); err != nil {
			return nil, err
		}
		if err := k.SetContentHash(ctx, preps[i].contentHash, subID); err != nil {
			return nil, err
		}
		if err := k.SetSubmissionDomainIndex(ctx, msg.Domain, subID); err != nil {
			return nil, err
		}
		if err := k.SetSubmissionSubmitterIndex(ctx, msg.Submitter, subID); err != nil {
			return nil, err
		}

		submissionIDs[i] = subID
		prevID = subID
	}

	// 7. TODO(R37-2): Initiate ONE quality round for the entire thread

	// 8. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"submit_thread",
		sdk.NewAttribute("thread_id", msg.ThreadId),
		sdk.NewAttribute("submitter", msg.Submitter),
		sdk.NewAttribute("domain", msg.Domain),
		sdk.NewAttribute("item_count", strconv.Itoa(len(msg.Items))),
	))

	return &types.MsgSubmitThreadResponse{
		SubmissionIds: submissionIDs,
		ThreadId:      msg.ThreadId,
	}, nil
}
```

**Step 4: Wire SubmitThread into msg_server.go**

Replace the stub:

```go
func (m msgServer) SubmitThread(ctx context.Context, msg *types.MsgSubmitThread) (*types.MsgSubmitThreadResponse, error) {
	return m.keeper.SubmitThread(ctx, msg)
}
```

**Step 5: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestSubmitThread" -v -count=1`
Expected: All PASS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/submission.go x/knowledge/keeper/submission_test.go x/knowledge/keeper/msg_server.go
git commit -m "feat(knowledge): implement SubmitThread handler with parent chain linking (R37-1)"
```

---

### Task 6: Sponsored Submission Tests

**Files:**
- Modify: `x/knowledge/keeper/submission_test.go`

**Step 1: Write tests for sponsored submissions**

Add to `submission_test.go`:

```go
func TestSubmitData_Sponsored(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	// Fund the bootstrap module
	bk := getBankKeeper(t, k)
	bk.setModuleBalance(types.BootstrapFundModuleName, sdk.NewCoins(sdk.NewInt64Coin("uzrn", 10_000_000)))

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "Sponsored content",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
		Sponsored:  true,
	}

	resp, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.SubmissionId)

	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.True(t, sub.Sponsored)
}

func TestSubmitData_SponsoredInsufficientFund(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	// Don't fund the bootstrap module

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "Sponsored but no fund",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
		Sponsored:  true,
	}

	_, err := k.SubmitData(ctx, msg)
	require.Error(t, err)
}
```

Add `getBankKeeper` helper to `keeper_test.go`:

```go
// getBankKeeper extracts the mock bank keeper from a test keeper.
// This requires the keeper to expose its bankKeeper for testing.
func getBankKeeper(t *testing.T, k keeper.Keeper) *mockBankKeeper {
	t.Helper()
	return k.BankKeeperForTest().(*mockBankKeeper)
}
```

And add to `keeper.go`:

```go
// BankKeeperForTest returns the bank keeper for testing.
func (k Keeper) BankKeeperForTest() types.BankKeeper {
	return k.bankKeeper
}
```

**Step 2: Run tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -run "TestSubmitData_Sponsor" -v -count=1`
Expected: All PASS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/submission_test.go x/knowledge/keeper/keeper_test.go x/knowledge/keeper/keeper.go
git commit -m "test(knowledge): add sponsored submission tests (R37-1)"
```

---

### Task 7: Event Emission & Edge Case Tests

**Files:**
- Modify: `x/knowledge/keeper/submission_test.go`

**Step 1: Write remaining tests to hit ≥30 target**

Add to `submission_test.go`:

```go
func TestSubmitData_EventEmitted(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "event test content",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	resp, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)

	events := ctx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == "submit_data" {
			found = true
			for _, attr := range e.Attributes {
				if attr.Key == "submission_id" {
					require.Equal(t, resp.SubmissionId, attr.Value)
				}
			}
		}
	}
	require.True(t, found, "submit_data event not emitted")
}

func TestSubmitThread_EventEmitted(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "thread-event",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "e1", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "e2", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	}

	_, err := k.SubmitThread(ctx, msg)
	require.NoError(t, err)

	events := ctx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == "submit_thread" {
			found = true
		}
	}
	require.True(t, found, "submit_thread event not emitted")
}

func TestSubmitData_ConsentTypes(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	consentTypes := []struct {
		name    string
		consent *types.ConsentProof
	}{
		{"self_authored", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		{"opt_in_sig", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN, AuthorSignature: "sig"}},
		{"opt_in_uri", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN, ProofUri: "https://x.com"}},
		{"public_license", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE, ProofUri: "https://mit.edu"}},
		{"platform_tos", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_PLATFORM_TOS, ProofUri: "https://tos.com"}},
		{"fair_use", &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_FAIR_USE}},
	}

	for i, tc := range consentTypes {
		t.Run(tc.name, func(t *testing.T) {
			msg := &types.MsgSubmitData{
				Submitter:  testAddr,
				Content:    fmt.Sprintf("consent test content %d", i),
				SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
				Domain:     "technology",
				Consent:    tc.consent,
				Stake:      "1000000",
			}
			_, err := k.SubmitData(ctx, msg)
			require.NoError(t, err)
		})
	}
}

func TestSubmitData_InactiveDomain(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "inactive",
		Status: types.DomainStatus_DOMAIN_STATUS_PROPOSED,
	}))

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "inactive",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidDomain)
}

func TestSubmitData_ZeroStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "0",
	}

	_, err := k.SubmitData(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestSubmitData_EmptyStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "test",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "",
	}

	_, err := k.SubmitData(ctx, msg)
	require.Error(t, err)
}

func TestSubmitData_WithThreadAndParent(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitData{
		Submitter:          testAddr,
		Content:            "child content",
		SampleType:         types.SampleType_SAMPLE_TYPE_CONVERSATION,
		Domain:             "technology",
		Consent:            &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:              "1000000",
		ThreadId:           "thread-x",
		ParentSubmissionId: "parent-1",
	}

	resp, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)

	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.Equal(t, "thread-x", sub.ThreadId)
	require.Equal(t, "parent-1", sub.ParentSubmissionId)
}

func TestSubmitThread_InsufficientStake(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "t-low",
		Domain:    "technology",
		Stake:     "100",
		Items: []*types.MsgSubmitData{
			{Content: "a", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "b", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	}

	_, err := k.SubmitThread(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientStake)
}

func TestSubmitThread_ItemConsentInvalid(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "t-consent",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "ok", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: "bad consent", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_OPT_IN}}, // missing proof
		},
	}

	_, err := k.SubmitThread(ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidConsent)
}

func TestSubmitThread_ItemContentTooLarge(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	big := make([]byte, 50_001)
	for i := range big {
		big[i] = 'x'
	}

	msg := &types.MsgSubmitThread{
		Submitter: testAddr,
		ThreadId:  "t-big",
		Domain:    "technology",
		Stake:     "1000000",
		Items: []*types.MsgSubmitData{
			{Content: "small", SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
			{Content: string(big), SampleType: types.SampleType_SAMPLE_TYPE_CONVERSATION, Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED}},
		},
	}

	_, err := k.SubmitThread(ctx, msg)
	require.ErrorIs(t, err, types.ErrContentTooLarge)
}

func TestSubmitData_MultipleDomains(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	for _, domain := range []string{"technology", "science"} {
		msg := &types.MsgSubmitData{
			Submitter:  testAddr,
			Content:    "content for " + domain,
			SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
			Domain:     domain,
			Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			Stake:      "1000000",
		}
		_, err := k.SubmitData(ctx, msg)
		require.NoError(t, err)
	}

	techIds := k.GetSubmissionsByDomain(ctx, "technology")
	sciIds := k.GetSubmissionsByDomain(ctx, "science")
	require.Len(t, techIds, 1)
	require.Len(t, sciIds, 1)
}

func TestContentHash_Deterministic(t *testing.T) {
	k, _ := setupKeeper(t)
	content := "deterministic hash test"
	h1 := k.ComputeContentHash(content)
	h2 := k.ComputeContentHash(content)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 64) // SHA-256 hex = 64 chars
}

func TestSubmitData_StakeLocking(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	bk := getBankKeeper(t, k)
	addr, _ := sdk.AccAddressFromBech32(testAddr)
	bk.setBalance(testAddr, sdk.NewCoins(sdk.NewInt64Coin("uzrn", 5_000_000)))

	msg := &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "stake locking test",
		SampleType: types.SampleType_SAMPLE_TYPE_INSTRUCTION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	}

	_, err := k.SubmitData(ctx, msg)
	require.NoError(t, err)

	// Balance should decrease
	bal := bk.GetBalance(sdk.Context{}, addr, "uzrn")
	require.Equal(t, sdkmath.NewInt(4_000_000), bal.Amount)
}
```

**Step 2: Run all tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/... -v -count=1`
Expected: ≥30 tests PASS

**Step 3: Commit**

```bash
git add x/knowledge/keeper/submission_test.go
git commit -m "test(knowledge): comprehensive submission lifecycle tests (R37-1, ≥30 tests)"
```

---

### Task 8: Final Verification & Cleanup

**Step 1: Run full test suite**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -v -count=1`
Expected: All PASS

**Step 2: Run vet and build**

Run: `cd /Users/yournameisai/Desktop/zerone && go vet ./x/knowledge/... && go build ./...`
Expected: No errors

**Step 3: Final commit (if any cleanup needed)**

```bash
git add -A
git commit -m "chore(knowledge): R37-1 submission lifecycle cleanup"
```

---

## Test Count Summary

| Test | Category |
|------|----------|
| TestSubmissionCRUD | Store CRUD |
| TestContentHashIndex | Store index |
| TestNextSubmissionID | Sequence |
| TestSubmissionIterator | Iterator |
| TestSubmissionsByDomain | Index query |
| TestSubmissionsBySubmitter | Index query |
| TestComputeContentHash | Hashing |
| TestCheckDuplicate | Dedup |
| TestValidateConsent (11 subtests) | Consent |
| TestSubmitData_Success | Happy path |
| TestSubmitData_ContentTooLarge | Validation |
| TestSubmitData_DuplicateContent | Dedup |
| TestSubmitData_InvalidConsent | Consent |
| TestSubmitData_InvalidDomain | Domain |
| TestSubmitData_InsufficientStake | Stake |
| TestSubmitData_Sponsored | Sponsor |
| TestSubmitData_SponsoredInsufficientFund | Sponsor fail |
| TestSubmitData_EventEmitted | Events |
| TestSubmitData_ConsentTypes (6 subtests) | Consent variants |
| TestSubmitData_InactiveDomain | Domain status |
| TestSubmitData_ZeroStake | Stake edge |
| TestSubmitData_EmptyStake | Stake edge |
| TestSubmitData_WithThreadAndParent | Threading |
| TestSubmitData_MultipleDomains | Multi-domain |
| TestSubmitData_StakeLocking | Economics |
| TestContentHash_Deterministic | Hash property |
| TestSubmitThread_Success | Thread happy path |
| TestSubmitThread_TooLarge | Thread validation |
| TestSubmitThread_DuplicateInThread | Thread dedup |
| TestSubmitThread_InvalidDomain | Thread domain |
| TestSubmitThread_InsufficientStake | Thread stake |
| TestSubmitThread_ItemConsentInvalid | Thread consent |
| TestSubmitThread_ItemContentTooLarge | Thread content |
| TestSubmitThread_EventEmitted | Thread events |

**Total: 34+ tests (11 consent subtests + 6 consent type subtests counted individually = 45+)**

## Files Created/Modified

| File | Action |
|------|--------|
| `x/knowledge/types/keys.go` | Modify — add 2 index prefixes + 4 key constructors |
| `x/knowledge/keeper/state.go` | Modify — add Submission CRUD, indexes, sequences |
| `x/knowledge/keeper/submission.go` | Create — hash, dedup, consent, SubmitData, SubmitThread |
| `x/knowledge/keeper/msg_server.go` | Modify — wire 2 handlers |
| `x/knowledge/keeper/keeper.go` | Modify — add test helpers |
| `x/knowledge/keeper/keeper_test.go` | Create — test setup, mocks |
| `x/knowledge/keeper/state_test.go` | Create — CRUD tests |
| `x/knowledge/keeper/submission_test.go` | Create — handler tests |
