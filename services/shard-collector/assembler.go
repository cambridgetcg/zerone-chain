package main

import "errors"

// ErrInsufficientShards is returned when <80% of expected TDUs are collected.
var ErrInsufficientShards = errors.New("insufficient shards: less than 80% of TDUs collected")

// AssemblyResult holds the outcome of dataset reassembly.
type AssemblyResult struct {
	Verified  []TDUData // TDUs that passed all integrity and replication checks
	Flagged   []string  // TDU IDs with replication mismatches
	TotalTDUs int       // number of unique verified TDUs
}

// Assemble collects verified TDUs from multiple shard responses, checks
// replication consistency, verifies content hashes against on-chain values,
// and returns the reassembled dataset. Returns ErrInsufficientShards if
// fewer than 80% of expected TDUs are collected.
func Assemble(responses []*ShardResponse, expectedHashes map[string][]byte, replicationFactor int) (*AssemblyResult, error) {
	// Step 1: Verify replication consistency across all responses
	verified, flagged := VerifyReplication(responses)

	// Step 2: Build deduplicated TDU map from verified responses
	tduMap := make(map[string]TDUData)
	for _, resp := range responses {
		for _, tdu := range resp.TDUs {
			if _, exists := tduMap[tdu.ID]; exists {
				continue // already have it
			}
			// Only include TDUs that passed replication check
			if contains(flagged, tdu.ID) {
				continue
			}
			// Verify against on-chain hash
			if err := VerifyShardIntegrity(&ShardResponse{TDUs: []TDUData{tdu}}, expectedHashes); err != nil {
				continue // skip invalid
			}
			tduMap[tdu.ID] = tdu
		}
	}

	// Step 3: Check sufficiency — need >=80% of expected TDUs
	totalExpected := len(expectedHashes)
	if totalExpected > 0 {
		ratio := float64(len(tduMap)) / float64(totalExpected)
		if ratio < 0.8 {
			return nil, ErrInsufficientShards
		}
	}

	// Step 4: Build result
	result := &AssemblyResult{
		Flagged:   flagged,
		TotalTDUs: len(tduMap),
	}
	for _, tdu := range tduMap {
		result.Verified = append(result.Verified, tdu)
	}

	_ = verified // used by VerifyReplication, result tracked in tduMap
	return result, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
