package types

const (
	// ModuleName is the alignment module's name.
	ModuleName = "alignment"

	// StoreKey is the store key for the alignment module.
	StoreKey = ModuleName
)

// KV store key prefixes.
var (
	ParamsKey              = []byte{0x01}
	StateKey               = []byte{0x02}
	ObservationKeyPrefix   = []byte{0x03}
	ScoresKeyPrefix        = []byte{0x04}
	HealthIndexKeyPrefix   = []byte{0x05}
	CorrectionKeyPrefix    = []byte{0x06}
	CorrectionCountKey         = []byte{0x07}
	CorrectionOutcomeKeyPrefix = []byte{0x08}
)

// ObservationKey returns the store key for an observation at a given height.
func ObservationKey(height uint64) []byte {
	return append(ObservationKeyPrefix, heightBytes(height)...)
}

// ScoresKey returns the store key for dimension scores at a given height.
func ScoresKey(height uint64) []byte {
	return append(ScoresKeyPrefix, heightBytes(height)...)
}

// HealthIndexKey returns the store key for a health index at a given height.
func HealthIndexKey(height uint64) []byte {
	return append(HealthIndexKeyPrefix, heightBytes(height)...)
}

// CorrectionKey returns the store key for a correction at height + index.
func CorrectionKey(height uint64, index uint32) []byte {
	key := make([]byte, len(CorrectionKeyPrefix)+12)
	copy(key, CorrectionKeyPrefix)
	hb := heightBytes(height)
	copy(key[len(CorrectionKeyPrefix):], hb)
	key[len(CorrectionKeyPrefix)+8] = byte(index >> 24)
	key[len(CorrectionKeyPrefix)+9] = byte(index >> 16)
	key[len(CorrectionKeyPrefix)+10] = byte(index >> 8)
	key[len(CorrectionKeyPrefix)+11] = byte(index)
	return key
}

// CorrectionOutcomeKey returns the store key for a correction outcome at height + dimension.
func CorrectionOutcomeKey(height uint64, dimension string) []byte {
	prefix := append(CorrectionOutcomeKeyPrefix, heightBytes(height)...)
	return append(prefix, []byte(dimension)...)
}

// CorrectionOutcomeHeightPrefix returns the key prefix for all outcomes at a height.
func CorrectionOutcomeHeightPrefix(height uint64) []byte {
	return append(CorrectionOutcomeKeyPrefix, heightBytes(height)...)
}

// heightBytes encodes a uint64 as 8 big-endian bytes for ordered iteration.
func heightBytes(height uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(height >> 56)
	b[1] = byte(height >> 48)
	b[2] = byte(height >> 40)
	b[3] = byte(height >> 32)
	b[4] = byte(height >> 24)
	b[5] = byte(height >> 16)
	b[6] = byte(height >> 8)
	b[7] = byte(height)
	return b
}
