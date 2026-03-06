package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"
)

// Transport abstracts the network layer for communicating with validators.
type Transport interface {
	// RequestShards sends a shard request to a validator and returns the response.
	RequestShards(ctx context.Context, validatorAddr string, req *ShardRequest) (*ShardResponse, error)
}

// MockTransport is a test implementation of Transport.
type MockTransport struct {
	Responses map[string]*ShardResponse
}

func (m *MockTransport) RequestShards(_ context.Context, validatorAddr string, _ *ShardRequest) (*ShardResponse, error) {
	resp, ok := m.Responses[validatorAddr]
	if !ok {
		return nil, fmt.Errorf("validator %s offline", validatorAddr)
	}
	return resp, nil
}

// Collector orchestrates shard collection from all validators.
type Collector struct {
	EnclaveID   string
	EnclaveKey  *ecdsa.PrivateKey
	Attestation []byte
	Transport   Transport
	Timeout     time.Duration // per-validator timeout
	MaxRetries  int           // retry count for offline validators
}

// CollectionResult is the final output of a collection cycle.
type CollectionResult struct {
	*AssemblyResult
	FailedValidators []string
	SnapshotHeight   int64
}

// Collect gathers shards from all validators in the assignment map,
// verifies integrity, checks replication, and assembles the dataset.
func (c *Collector) Collect(
	ctx context.Context,
	snapshotHeight int64,
	assignments map[string][]string, // validator → TDU IDs
	expectedHashes map[string][]byte, // TDU ID → on-chain content hash
	replicationFactor int,
) (*CollectionResult, error) {
	var (
		mu               sync.Mutex
		responses        []*ShardResponse
		failedValidators []string
		wg               sync.WaitGroup
	)

	for validatorAddr, tduIDs := range assignments {
		wg.Add(1)
		go func(addr string, ids []string) {
			defer wg.Done()

			nonce, err := GenerateNonce()
			if err != nil {
				mu.Lock()
				failedValidators = append(failedValidators, addr)
				mu.Unlock()
				return
			}

			req := &ShardRequest{
				EnclaveID:      c.EnclaveID,
				Attestation:    c.Attestation,
				SnapshotHeight: snapshotHeight,
				RequestedTDUs:  ids,
				Nonce:          nonce,
			}

			resp, err := c.requestWithRetry(ctx, addr, req)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failedValidators = append(failedValidators, addr)
				return
			}
			responses = append(responses, resp)
		}(validatorAddr, tduIDs)
	}

	wg.Wait()

	// Assemble dataset from collected responses
	assembly, err := Assemble(responses, expectedHashes, replicationFactor)
	if err != nil {
		return nil, err
	}

	return &CollectionResult{
		AssemblyResult:   assembly,
		FailedValidators: failedValidators,
		SnapshotHeight:   snapshotHeight,
	}, nil
}

// requestWithRetry attempts to request shards from a validator with
// exponential backoff retries.
func (c *Collector) requestWithRetry(ctx context.Context, addr string, req *ShardRequest) (*ShardResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
		resp, err := c.Transport.RequestShards(reqCtx, addr, req)
		cancel()

		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("validator %s: all %d retries exhausted: %w", addr, c.MaxRetries+1, lastErr)
}
