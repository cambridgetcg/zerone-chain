package types

const (
	ModuleName = "private_corpus"
	StoreKey   = ModuleName
)

// KV store key prefixes.
var (
	ParamsKey               = []byte{0x00}
	VaultKeyPrefix          = []byte{0x01} // vault_id → Vault
	ManifestKeyPrefix       = []byte{0x02} // manifest_id → CorpusManifest
	AccessRecordKeyPrefix   = []byte{0x03} // seq (big-endian) → AccessRecord
	NextAccessSeqKey        = []byte{0x04}

	// Indexes.
	VaultByOperatorPrefix    = []byte{0x10} // operator/vault_id → 1
	ManifestByVaultPrefix    = []byte{0x11} // vault_id/manifest_id → 1
	AccessRecordByVaultPrefix = []byte{0x12} // vault_id/seq → 1
)
