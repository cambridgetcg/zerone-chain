package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

// ShardRequest represents a TEE's request for shards from a validator.
type ShardRequest struct {
	EnclaveID      string
	Attestation    []byte
	SnapshotHeight int64
	RequestedTDUs  []string
	Nonce          []byte
	Signature      []byte
}

// ShardResponse represents a validator's response containing shard data.
type ShardResponse struct {
	ValidatorAddr  string
	SnapshotHeight int64
	TDUs           []TDUData
	DataHash       []byte
	Nonce          []byte
	Signature      []byte
}

// TDUData is a single training data unit with content and integrity hash.
type TDUData struct {
	ID      string
	Content []byte
	Hash    []byte
}

// VerifyShardIntegrity checks that all TDUs in a response match their
// expected on-chain content hashes, and that the actual content matches
// the claimed hash.
func VerifyShardIntegrity(resp *ShardResponse, expectedHashes map[string][]byte) error {
	for _, tdu := range resp.TDUs {
		// Verify content matches its own hash
		computed := sha256.Sum256(tdu.Content)
		if !bytes.Equal(computed[:], tdu.Hash) {
			return fmt.Errorf("TDU %s: content hash mismatch (computed %x, claimed %x)",
				tdu.ID, computed[:8], tdu.Hash[:8])
		}

		// Verify hash matches on-chain expected hash
		expected, ok := expectedHashes[tdu.ID]
		if !ok {
			return fmt.Errorf("TDU %s: not in expected set", tdu.ID)
		}
		if !bytes.Equal(tdu.Hash, expected) {
			return fmt.Errorf("TDU %s: hash does not match on-chain hash (got %x, want %x)",
				tdu.ID, tdu.Hash[:8], expected[:8])
		}
	}
	return nil
}

// VerifyReplication checks that all copies of each TDU across multiple
// validator responses have matching content hashes. Returns lists of
// verified (consistent) and flagged (inconsistent) TDU IDs.
func VerifyReplication(responses []*ShardResponse) (verified, flagged []string) {
	// Collect all hashes per TDU ID
	tduHashes := make(map[string][][]byte)
	for _, resp := range responses {
		for _, tdu := range resp.TDUs {
			tduHashes[tdu.ID] = append(tduHashes[tdu.ID], tdu.Hash)
		}
	}

	for id, hashes := range tduHashes {
		consistent := true
		for i := 1; i < len(hashes); i++ {
			if !bytes.Equal(hashes[0], hashes[i]) {
				consistent = false
				break
			}
		}
		if consistent {
			verified = append(verified, id)
		} else {
			flagged = append(flagged, id)
		}
	}
	return verified, flagged
}
