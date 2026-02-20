package types

import (
	"encoding/binary"
)

const (
	ModuleName  = "zerone_auth"
	StoreKey    = ModuleName
	RouterKey   = ModuleName
	MemStoreKey = "mem_" + ModuleName
)

var (
	// AccountKeyPrefix is the prefix for account storage.
	AccountKeyPrefix = []byte{0x01}

	// DIDMappingPrefix is the prefix for DID -> bech32 mapping.
	DIDMappingPrefix = []byte{0x02}

	// SessionKeyPrefix is the prefix for session keys.
	SessionKeyPrefix = []byte{0x03}

	// ParamsKey is the key for module parameters.
	ParamsKey = []byte{0x04}

	// LastRotationPrefix is the prefix for tracking last key rotation.
	LastRotationPrefix = []byte{0x05}

	// RecoveryRequestPrefix is the prefix for recovery requests.
	RecoveryRequestPrefix = []byte{0x06}

	// RecoveryConfigPrefix is the prefix for recovery configuration per account.
	RecoveryConfigPrefix = []byte{0x07}

	// BootstrapClaimPrefix tracks which accounts have claimed their bootstrap fund.
	BootstrapClaimPrefix = []byte{0x08}

	// RecoveryShardPrefix stores encrypted recovery shard data.
	RecoveryShardPrefix = []byte{0x09}
)

// AccountKey returns the store key for an account by bech32 address.
func AccountKey(address string) []byte {
	return append(AccountKeyPrefix, []byte(address)...)
}

// DIDMappingKey returns the store key for DID mapping.
func DIDMappingKey(did string) []byte {
	return append(DIDMappingPrefix, []byte(did)...)
}

// SessionKeyKey returns the store key for a session key.
func SessionKeyKey(owner, keyHash string) []byte {
	key := append(SessionKeyPrefix, []byte(owner)...)
	key = append(key, byte('/'))
	key = append(key, []byte(keyHash)...)
	return key
}

// SessionKeyOwnerPrefix returns the prefix for all session keys of an owner.
func SessionKeyOwnerPrefix(owner string) []byte {
	return append(SessionKeyPrefix, []byte(owner)...)
}

// LastRotationKey returns the store key for last rotation timestamp.
func LastRotationKey(address string) []byte {
	return append(LastRotationPrefix, []byte(address)...)
}

// RecoveryRequestKey returns the store key for a recovery request by account address.
func RecoveryRequestKey(address string) []byte {
	return append(RecoveryRequestPrefix, []byte(address)...)
}

// RecoveryConfigKey returns the store key for a recovery config by account address.
func RecoveryConfigKey(address string) []byte {
	return append(RecoveryConfigPrefix, []byte(address)...)
}

// BootstrapClaimKey returns the store key for tracking a bootstrap claim by address.
func BootstrapClaimKey(address string) []byte {
	return append(BootstrapClaimPrefix, []byte(address)...)
}

// RecoveryShardKey returns the store key for an encrypted recovery shard.
func RecoveryShardKey(ownerAddress string, shardIndex uint32) []byte {
	key := append(RecoveryShardPrefix, []byte(ownerAddress)...)
	key = append(key, byte('/'))
	key = append(key, Uint32ToBytes(shardIndex)...)
	return key
}

// RecoveryShardOwnerPrefix returns the prefix for all shards of an owner.
func RecoveryShardOwnerPrefix(ownerAddress string) []byte {
	key := append(RecoveryShardPrefix, []byte(ownerAddress)...)
	key = append(key, byte('/'))
	return key
}

// Uint32ToBytes converts uint32 to 4 bytes (big-endian).
func Uint32ToBytes(n uint32) []byte {
	b := make([]byte, 4)
	b[0] = byte(n >> 24)
	b[1] = byte(n >> 16)
	b[2] = byte(n >> 8)
	b[3] = byte(n)
	return b
}

// Uint64ToBytes converts uint64 to bytes for storage.
func Uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// BytesToUint64 converts bytes to uint64 from storage.
func BytesToUint64(b []byte) uint64 {
	if len(b) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b)
}
