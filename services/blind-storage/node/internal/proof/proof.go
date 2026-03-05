package proof

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Challenge represents a proof-of-storage challenge.
type Challenge struct {
	DatasetID  string `json:"dataset_id"`
	ChunkIndex int    `json:"chunk_index"`
	Offset     int    `json:"offset"`
	Length     int    `json:"length"`
	Nonce      string `json:"nonce"`
}

// Response is the proof submitted by a storage node.
type Response struct {
	DatasetID  string `json:"dataset_id"`
	ChunkIndex int    `json:"chunk_index"`
	DataHash   string `json:"data_hash"`   // hash of requested byte range
	ChunkHash  string `json:"chunk_hash"`  // hash of full chunk
	NodeAddr   string `json:"node_addr"`
}

// ComputeResponse generates a proof response from chunk data.
func ComputeResponse(challenge *Challenge, chunkData []byte, nodeAddr string) (*Response, error) {
	if challenge.Offset >= len(chunkData) {
		return nil, fmt.Errorf("challenge offset %d beyond chunk size %d", challenge.Offset, len(chunkData))
	}

	end := challenge.Offset + challenge.Length
	if end > len(chunkData) {
		end = len(chunkData)
	}

	rangeData := chunkData[challenge.Offset:end]
	// Hash includes nonce to prevent replay
	rangeWithNonce := append(rangeData, []byte(challenge.Nonce)...)
	rangeHash := sha256.Sum256(rangeWithNonce)

	chunkHash := sha256.Sum256(chunkData)

	return &Response{
		DatasetID:  challenge.DatasetID,
		ChunkIndex: challenge.ChunkIndex,
		DataHash:   hex.EncodeToString(rangeHash[:]),
		ChunkHash:  hex.EncodeToString(chunkHash[:]),
		NodeAddr:   nodeAddr,
	}, nil
}

// VerifyResponse checks a proof response against the expected chunk hash.
func VerifyResponse(challenge *Challenge, response *Response, expectedChunkHash string) bool {
	return response.ChunkHash == expectedChunkHash
}
