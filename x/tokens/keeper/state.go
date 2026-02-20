package keeper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/tokens/types"
)

var symbolRegex = regexp.MustCompile(`^[A-Z0-9]{1,16}$`)

// isValidSymbol checks that a symbol is 1-16 uppercase alphanumeric characters.
func isValidSymbol(symbol string) bool {
	return symbolRegex.MatchString(symbol)
}

// GenerateTokenID produces a deterministic token ID from creator, symbol, and block height.
// Formula: hex(SHA256("ZRN.token.id.v1:" + creator + ":" + symbol + ":" + blockHeight))
func GenerateTokenID(creator, symbol string, blockHeight int64) string {
	data := fmt.Sprintf("ZRN.token.id.v1:%s:%s:%d", creator, symbol, blockHeight)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GenerateEmissionID produces a deterministic emission period ID.
func GenerateEmissionID(creator string, startBlock, endBlock uint64) string {
	data := fmt.Sprintf("ZRN.emission.v1:%s:%d:%d", creator, startBlock, endBlock)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// ---------- Token CRUD ----------

// SetToken stores a token definition and maintains the symbol index.
func (k Keeper) SetToken(ctx sdk.Context, token *types.TokenDefinition) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(token)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal token: %v", err))
	}
	_ = kvStore.Set(types.TokenDefKey(token.Id), bz)
	_ = kvStore.Set(types.SymbolIndexKey(token.Symbol), []byte(token.Id))
}

// GetToken retrieves a token definition by ID. Returns nil if not found.
func (k Keeper) GetToken(ctx sdk.Context, tokenId string) *types.TokenDefinition {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.TokenDefKey(tokenId))
	if err != nil || bz == nil {
		return nil
	}
	var token types.TokenDefinition
	if err := proto.Unmarshal(bz, &token); err != nil {
		return nil
	}
	return &token
}

// GetTokenBySymbol retrieves a token definition by symbol using the symbol index.
func (k Keeper) GetTokenBySymbol(ctx sdk.Context, symbol string) *types.TokenDefinition {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SymbolIndexKey(symbol))
	if err != nil || bz == nil {
		return nil
	}
	return k.GetToken(ctx, string(bz))
}

// DeleteToken removes a token definition and its symbol index entry.
func (k Keeper) DeleteToken(ctx sdk.Context, token *types.TokenDefinition) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.TokenDefKey(token.Id))
	_ = kvStore.Delete(types.SymbolIndexKey(token.Symbol))
}

// IterateTokens iterates over all token definitions. Return true from cb to stop.
func (k Keeper) IterateTokens(ctx sdk.Context, cb func(token *types.TokenDefinition) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.TokenDefKeyPrefix, prefixEndBytes(types.TokenDefKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var token types.TokenDefinition
		if err := proto.Unmarshal(iter.Value(), &token); err != nil {
			continue
		}
		if cb(&token) {
			break
		}
	}
}

// ---------- Balance CRUD ----------

// GetBalance returns the balance of an owner for a token. Returns zero if not found.
func (k Keeper) GetBalance(ctx sdk.Context, tokenId, ownerAddr string) *big.Int {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.BalanceKey(tokenId, ownerAddr))
	if err != nil || bz == nil {
		return new(big.Int)
	}
	bal := new(big.Int)
	if _, ok := bal.SetString(string(bz), 10); !ok {
		return new(big.Int)
	}
	return bal
}

// SetBalance sets the balance of an owner for a token.
// If the balance is zero, deletes the entry and the owner-token index.
func (k Keeper) SetBalance(ctx sdk.Context, tokenId, ownerAddr string, balance *big.Int) {
	kvStore := k.storeService.OpenKVStore(ctx)
	if balance.Sign() == 0 {
		_ = kvStore.Delete(types.BalanceKey(tokenId, ownerAddr))
		_ = kvStore.Delete(types.OwnerTokenIndexKey(ownerAddr, tokenId))
		return
	}
	_ = kvStore.Set(types.BalanceKey(tokenId, ownerAddr), []byte(balance.String()))
	_ = kvStore.Set(types.OwnerTokenIndexKey(ownerAddr, tokenId), []byte{1})
}

// IterateBalancesByToken iterates over all balances for a given token.
func (k Keeper) IterateBalancesByToken(ctx sdk.Context, tokenId string, cb func(ownerAddr string, balance *big.Int) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.BalancesByTokenPrefix(tokenId)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		ownerAddr := string(key[prefixLen:])
		bal := new(big.Int)
		if _, ok := bal.SetString(string(iter.Value()), 10); !ok {
			continue
		}
		if cb(ownerAddr, bal) {
			break
		}
	}
}

// ---------- Allowance CRUD ----------

// GetAllowance returns the allowance granted by owner to spender for a token.
func (k Keeper) GetAllowance(ctx sdk.Context, tokenId, ownerAddr, spenderAddr string) *big.Int {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AllowanceKey(tokenId, ownerAddr, spenderAddr))
	if err != nil || bz == nil {
		return new(big.Int)
	}
	al := new(big.Int)
	if _, ok := al.SetString(string(bz), 10); !ok {
		return new(big.Int)
	}
	return al
}

// SetAllowance sets the allowance for a spender. Zero deletes the entry.
func (k Keeper) SetAllowance(ctx sdk.Context, tokenId, ownerAddr, spenderAddr string, allowance *big.Int) {
	kvStore := k.storeService.OpenKVStore(ctx)
	if allowance.Sign() == 0 {
		_ = kvStore.Delete(types.AllowanceKey(tokenId, ownerAddr, spenderAddr))
		return
	}
	_ = kvStore.Set(types.AllowanceKey(tokenId, ownerAddr, spenderAddr), []byte(allowance.String()))
}

// IterateAllowancesByOwner iterates over all allowances granted by an owner for a token.
func (k Keeper) IterateAllowancesByOwner(ctx sdk.Context, tokenId, ownerAddr string, cb func(spenderAddr string, allowance *big.Int) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AllowancesByOwnerPrefix(tokenId, ownerAddr)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		spenderAddr := string(key[prefixLen:])
		al := new(big.Int)
		if _, ok := al.SetString(string(iter.Value()), 10); !ok {
			continue
		}
		if cb(spenderAddr, al) {
			break
		}
	}
}

// ---------- Delegation CRUD ----------

// GetDelegation returns the delegation amount from delegator to delegate for a token.
func (k Keeper) GetDelegation(ctx sdk.Context, tokenId, delegator, delegate string) *big.Int {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.DelegationKey(tokenId, delegator, delegate))
	if err != nil || bz == nil {
		return new(big.Int)
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(string(bz), 10); !ok {
		return new(big.Int)
	}
	return amt
}

// SetDelegation sets the delegation amount. Zero deletes the entry.
func (k Keeper) SetDelegation(ctx sdk.Context, tokenId, delegator, delegate string, amount *big.Int) {
	kvStore := k.storeService.OpenKVStore(ctx)
	if amount.Sign() == 0 {
		_ = kvStore.Delete(types.DelegationKey(tokenId, delegator, delegate))
		return
	}
	_ = kvStore.Set(types.DelegationKey(tokenId, delegator, delegate), []byte(amount.String()))
}

// GetDelegatorTotal returns the total amount delegated by a delegator for a token.
func (k Keeper) GetDelegatorTotal(ctx sdk.Context, tokenId, delegator string) *big.Int {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.DelegatorTotalKey(tokenId, delegator))
	if err != nil || bz == nil {
		return new(big.Int)
	}
	amt := new(big.Int)
	if _, ok := amt.SetString(string(bz), 10); !ok {
		return new(big.Int)
	}
	return amt
}

// SetDelegatorTotal sets the total delegated amount. Zero deletes the entry.
func (k Keeper) SetDelegatorTotal(ctx sdk.Context, tokenId, delegator string, total *big.Int) {
	kvStore := k.storeService.OpenKVStore(ctx)
	if total.Sign() == 0 {
		_ = kvStore.Delete(types.DelegatorTotalKey(tokenId, delegator))
		return
	}
	_ = kvStore.Set(types.DelegatorTotalKey(tokenId, delegator), []byte(total.String()))
}

// GetUndelegatedBalance returns balance minus total delegated for a token holder.
func (k Keeper) GetUndelegatedBalance(ctx sdk.Context, tokenId, addr string) *big.Int {
	bal := k.GetBalance(ctx, tokenId, addr)
	total := k.GetDelegatorTotal(ctx, tokenId, addr)
	result := new(big.Int).Sub(bal, total)
	if result.Sign() < 0 {
		return new(big.Int)
	}
	return result
}

// IterateDelegationsByDelegator iterates over all delegations by a delegator for a token.
func (k Keeper) IterateDelegationsByDelegator(ctx sdk.Context, tokenId, delegator string, cb func(delegate string, amount *big.Int) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.DelegationsByDelegatorPrefix(tokenId, delegator)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		delegate := string(iter.Key()[prefixLen:])
		amt := new(big.Int)
		if _, ok := amt.SetString(string(iter.Value()), 10); !ok {
			continue
		}
		if cb(delegate, amt) {
			break
		}
	}
}

// IterateDelegationsByToken iterates over all delegations for a token.
func (k Keeper) IterateDelegationsByToken(ctx sdk.Context, tokenId string, cb func(delegator, delegate string, amount *big.Int) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.DelegationsByTokenPrefix(tokenId)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		suffix := string(iter.Key()[prefixLen:])
		parts := strings.SplitN(suffix, "/", 2)
		if len(parts) != 2 {
			continue
		}
		amt := new(big.Int)
		if _, ok := amt.SetString(string(iter.Value()), 10); !ok {
			continue
		}
		if cb(parts[0], parts[1], amt) {
			break
		}
	}
}

// IterateDelegatorTotalsByToken iterates over all delegator totals for a token.
func (k Keeper) IterateDelegatorTotalsByToken(ctx sdk.Context, tokenId string, cb func(delegator string, total *big.Int) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.DelegatorTotalsByTokenPrefix(tokenId)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()

	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		delegator := string(iter.Key()[prefixLen:])
		amt := new(big.Int)
		if _, ok := amt.SetString(string(iter.Value()), 10); !ok {
			continue
		}
		if cb(delegator, amt) {
			break
		}
	}
}

// ---------- Wrap CRUD ----------

// WrappedDenom returns the deterministic wrapped denom for a token ID.
func WrappedDenom(tokenId string) string {
	return "zrn20/" + tokenId
}

// SetWrapRecord stores the wrap record mapping (tokenId <-> wrappedDenom).
func (k Keeper) SetWrapRecord(ctx sdk.Context, tokenId, wrappedDenom string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.WrapRecordKey(tokenId), []byte(wrappedDenom))
	_ = kvStore.Set(types.WrapReverseIndexKey(wrappedDenom), []byte(tokenId))
}

// GetWrappedDenom returns the wrapped denom for a token ID, or empty if not set.
func (k Keeper) GetWrappedDenom(ctx sdk.Context, tokenId string) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.WrapRecordKey(tokenId))
	if err != nil || bz == nil {
		return ""
	}
	return string(bz)
}

// GetTokenIdByWrappedDenom returns the token ID for a wrapped denom, or empty if not found.
func (k Keeper) GetTokenIdByWrappedDenom(ctx sdk.Context, wrappedDenom string) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.WrapReverseIndexKey(wrappedDenom))
	if err != nil || bz == nil {
		return ""
	}
	return string(bz)
}

// DeleteWrapRecord removes a wrap record and its reverse index.
func (k Keeper) DeleteWrapRecord(ctx sdk.Context, tokenId, wrappedDenom string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.WrapRecordKey(tokenId))
	_ = kvStore.Delete(types.WrapReverseIndexKey(wrappedDenom))
}

// IterateWrapRecords iterates over all wrap records.
func (k Keeper) IterateWrapRecords(ctx sdk.Context, cb func(tokenId, wrappedDenom string) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.WrapRecordKeyPrefix, prefixEndBytes(types.WrapRecordKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	prefixLen := len(types.WrapRecordKeyPrefix)
	for ; iter.Valid(); iter.Next() {
		tokenId := string(iter.Key()[prefixLen:])
		wrappedDenom := string(iter.Value())
		if cb(tokenId, wrappedDenom) {
			break
		}
	}
}

// ---------- Emission Period CRUD ----------

// SetEmissionPeriod stores an emission period.
func (k Keeper) SetEmissionPeriod(ctx sdk.Context, emission *types.EmissionPeriod) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(emission)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal emission period: %v", err))
	}
	_ = kvStore.Set(types.EmissionPeriodKey(emission.Id), bz)
}

// GetEmissionPeriod retrieves an emission period by ID. Returns nil if not found.
func (k Keeper) GetEmissionPeriod(ctx sdk.Context, emissionId string) *types.EmissionPeriod {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.EmissionPeriodKey(emissionId))
	if err != nil || bz == nil {
		return nil
	}
	var emission types.EmissionPeriod
	if err := proto.Unmarshal(bz, &emission); err != nil {
		return nil
	}
	return &emission
}

// IterateEmissionPeriods iterates over all emission periods. Return true from cb to stop.
func (k Keeper) IterateEmissionPeriods(ctx sdk.Context, cb func(emission *types.EmissionPeriod) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.EmissionPeriodKeyPrefix, prefixEndBytes(types.EmissionPeriodKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var emission types.EmissionPeriod
		if err := proto.Unmarshal(iter.Value(), &emission); err != nil {
			continue
		}
		if cb(&emission) {
			break
		}
	}
}

// ---------- Helpers ----------

// prefixEndBytes returns the end key for prefix iteration (prefix with last byte incremented).
func prefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}
