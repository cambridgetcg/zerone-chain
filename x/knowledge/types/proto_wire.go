// proto_wire.go — Manual protobuf wire format helpers for types that cannot be proto-gen'd.
package types

import (
	"encoding/binary"
	"errors"
	"math/bits"
)

// ---------------------------------------------------------------------------
// Wire type constants
// ---------------------------------------------------------------------------

const (
	protoWireVarint          uint8 = 0
	protoWireLengthDelimited uint8 = 2
	protoWireFixed32         uint8 = 5
	protoWireFixed64         uint8 = 1
)

// ---------------------------------------------------------------------------
// Encoding helpers
// ---------------------------------------------------------------------------

// protoAppendVarint appends a varint-encoded uint64 to buf.
func protoAppendVarint(buf []byte, v uint64) []byte {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], v)
	return append(buf, tmp[:n]...)
}

// protoAppendLenPrefixed appends length-prefixed bytes (varint length + raw data) to buf.
func protoAppendLenPrefixed(buf []byte, data []byte) []byte {
	buf = protoAppendVarint(buf, uint64(len(data)))
	return append(buf, data...)
}

// protoAppendTag appends a protobuf tag (field_number << 3 | wire_type) to buf.
func protoAppendTag(buf []byte, fieldNum uint32, wireType uint8) []byte {
	return protoAppendVarint(buf, uint64(fieldNum)<<3|uint64(wireType))
}

// protoAppendStringField appends a string field (wire type 2). Skipped if s is empty.
func protoAppendStringField(buf []byte, fieldNum uint32, s string) []byte {
	if s == "" {
		return buf
	}
	buf = protoAppendTag(buf, fieldNum, protoWireLengthDelimited)
	buf = protoAppendLenPrefixed(buf, []byte(s))
	return buf
}

// protoAppendVarintField appends a varint field (wire type 0). Skipped if v is zero.
func protoAppendVarintField(buf []byte, fieldNum uint32, v uint64) []byte {
	if v == 0 {
		return buf
	}
	buf = protoAppendTag(buf, fieldNum, protoWireVarint)
	buf = protoAppendVarint(buf, v)
	return buf
}

// protoAppendInt64Field appends an int64 field as a varint (wire type 0). Skipped if v is zero.
func protoAppendInt64Field(buf []byte, fieldNum uint32, v int64) []byte {
	if v == 0 {
		return buf
	}
	buf = protoAppendTag(buf, fieldNum, protoWireVarint)
	buf = protoAppendVarint(buf, uint64(v))
	return buf
}

// protoAppendBoolField appends a bool field as varint 1 (wire type 0). Skipped if v is false.
func protoAppendBoolField(buf []byte, fieldNum uint32, v bool) []byte {
	if !v {
		return buf
	}
	buf = protoAppendTag(buf, fieldNum, protoWireVarint)
	buf = protoAppendVarint(buf, 1)
	return buf
}

// protoAppendBytesField appends a bytes field (wire type 2). Skipped if data is nil or empty.
func protoAppendBytesField(buf []byte, fieldNum uint32, data []byte) []byte {
	if len(data) == 0 {
		return buf
	}
	buf = protoAppendTag(buf, fieldNum, protoWireLengthDelimited)
	buf = protoAppendLenPrefixed(buf, data)
	return buf
}

// protoAppendMessageField appends an embedded message field (wire type 2). Skipped if data is nil or empty.
func protoAppendMessageField(buf []byte, fieldNum uint32, data []byte) []byte {
	if len(data) == 0 {
		return buf
	}
	buf = protoAppendTag(buf, fieldNum, protoWireLengthDelimited)
	buf = protoAppendLenPrefixed(buf, data)
	return buf
}

// ---------------------------------------------------------------------------
// Size helpers
// ---------------------------------------------------------------------------

// protoSizeVarint returns the number of bytes needed to varint-encode v.
func protoSizeVarint(v uint64) int {
	if v == 0 {
		return 1
	}
	// Each varint byte encodes 7 bits.
	return (bits.Len64(v) + 6) / 7
}

// protoSizeTag returns the number of bytes needed to encode the tag for fieldNum.
func protoSizeTag(fieldNum uint32) int {
	return protoSizeVarint(uint64(fieldNum) << 3)
}

// protoSizeStringField returns the total encoded size of a string field (0 if empty).
func protoSizeStringField(fieldNum uint32, s string) int {
	if s == "" {
		return 0
	}
	l := len(s)
	return protoSizeTag(fieldNum) + protoSizeVarint(uint64(l)) + l
}

// protoSizeVarintField returns the total encoded size of a varint field (0 if zero).
func protoSizeVarintField(fieldNum uint32, v uint64) int {
	if v == 0 {
		return 0
	}
	return protoSizeTag(fieldNum) + protoSizeVarint(v)
}

// protoSizeInt64Field returns the total encoded size of an int64 varint field (0 if zero).
func protoSizeInt64Field(fieldNum uint32, v int64) int {
	if v == 0 {
		return 0
	}
	return protoSizeTag(fieldNum) + protoSizeVarint(uint64(v))
}

// protoSizeBoolField returns the total encoded size of a bool field (0 if false).
func protoSizeBoolField(fieldNum uint32, v bool) int {
	if !v {
		return 0
	}
	// bool true is varint(1) which is always 1 byte.
	return protoSizeTag(fieldNum) + 1
}

// protoSizeBytesField returns the total encoded size of a bytes field (0 if empty).
func protoSizeBytesField(fieldNum uint32, data []byte) int {
	if len(data) == 0 {
		return 0
	}
	l := len(data)
	return protoSizeTag(fieldNum) + protoSizeVarint(uint64(l)) + l
}

// protoSizeMessageField returns the total encoded size of an embedded message field (0 if empty).
func protoSizeMessageField(fieldNum uint32, data []byte) int {
	if len(data) == 0 {
		return 0
	}
	l := len(data)
	return protoSizeTag(fieldNum) + protoSizeVarint(uint64(l)) + l
}

// ---------------------------------------------------------------------------
// Decoding helpers
// ---------------------------------------------------------------------------

var (
	errProtoShortBuffer   = errors.New("proto: short buffer")
	errProtoOverflow      = errors.New("proto: varint overflow")
	errProtoNegativeLength = errors.New("proto: negative length")
	errProtoUnknownWire   = errors.New("proto: unknown wire type")
)

// protoConsumeVarint reads a varint from data starting at offset.
// Returns (value, newOffset, error).
func protoConsumeVarint(data []byte, offset int) (uint64, int, error) {
	var v uint64
	for shift := uint(0); ; shift += 7 {
		if offset >= len(data) {
			return 0, offset, errProtoShortBuffer
		}
		if shift >= 64 {
			return 0, offset, errProtoOverflow
		}
		b := data[offset]
		offset++
		v |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return v, offset, nil
		}
	}
}

// protoConsumeBytes reads a length-delimited field from data starting at offset.
// It reads the varint length prefix and then returns the raw bytes.
// Returns (bytes, newOffset, error).
func protoConsumeBytes(data []byte, offset int) ([]byte, int, error) {
	length, newOffset, err := protoConsumeVarint(data, offset)
	if err != nil {
		return nil, offset, err
	}
	l := int(length)
	if l < 0 {
		return nil, offset, errProtoNegativeLength
	}
	if newOffset+l > len(data) {
		return nil, offset, errProtoShortBuffer
	}
	return data[newOffset : newOffset+l], newOffset + l, nil
}

// protoConsumeTag reads a protobuf tag from data starting at offset.
// Returns (fieldNum, wireType, newOffset, error).
func protoConsumeTag(data []byte, offset int) (uint32, uint8, int, error) {
	v, newOffset, err := protoConsumeVarint(data, offset)
	if err != nil {
		return 0, 0, offset, err
	}
	fieldNum := uint32(v >> 3)
	wireType := uint8(v & 0x07)
	return fieldNum, wireType, newOffset, nil
}

// protoSkipField advances offset past an unknown field of the given wire type.
// Returns (newOffset, error).
func protoSkipField(data []byte, offset int, wireType uint8) (int, error) {
	switch wireType {
	case protoWireVarint:
		// Consume the varint value.
		_, newOffset, err := protoConsumeVarint(data, offset)
		return newOffset, err

	case protoWireLengthDelimited:
		// Read length, then skip that many bytes.
		length, newOffset, err := protoConsumeVarint(data, offset)
		if err != nil {
			return offset, err
		}
		l := int(length)
		if l < 0 {
			return offset, errProtoNegativeLength
		}
		if newOffset+l > len(data) {
			return offset, errProtoShortBuffer
		}
		return newOffset + l, nil

	case protoWireFixed32:
		if offset+4 > len(data) {
			return offset, errProtoShortBuffer
		}
		return offset + 4, nil

	case protoWireFixed64:
		if offset+8 > len(data) {
			return offset, errProtoShortBuffer
		}
		return offset + 8, nil

	default:
		return offset, errProtoUnknownWire
	}
}
