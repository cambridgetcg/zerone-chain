package types

const (
	// ModuleName is the billing module's name.
	ModuleName = "billing"

	// StoreKey is the store key for the billing module.
	StoreKey = ModuleName
)

// KV store key prefixes.
var (
	ParamsKey              = []byte{0x00}
	ProviderKeyPrefix      = []byte{0x01}
	DomainIndexPrefix      = []byte{0x02}
	DynamicPricingConfigKey = []byte{0x03}
	LastEmittedPriceKey    = []byte{0x04}
	EscrowKeyPrefix        = []byte{0x05}
	SettlementKeyPrefix    = []byte{0x06}
	SettlementSeqKey       = []byte{0x07}
)

// EscrowKey returns the KV key for a user's escrow balance.
func EscrowKey(userAddr string) []byte {
	return append(EscrowKeyPrefix, []byte(userAddr)...)
}

// SettlementKey returns the KV key for a settlement record.
func SettlementKey(id string) []byte {
	return append(SettlementKeyPrefix, []byte(id)...)
}
