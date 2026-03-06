package agentsdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ChainClient abstracts the Cosmos SDK chain interaction for testability.
// Production implementations use CometBFT RPC + keyring signing.
type ChainClient interface {
	// BroadcastMsg signs and broadcasts a sdk.Msg, returning the tx hash.
	BroadcastMsg(ctx context.Context, msg interface{}) (txHash string, err error)
	// QueryParams queries the knowledge module parameters.
	QueryParams(ctx context.Context) (*types.Params, error)
	// QuerySamplesBySubmitter returns samples submitted by the given address.
	QuerySamplesBySubmitter(ctx context.Context, submitter string) ([]*types.Sample, error)
	// QuerySample returns a single sample by ID.
	QuerySample(ctx context.Context, id string) (*types.Sample, error)
	// QueryQualityRound returns a quality round by ID.
	QueryQualityRound(ctx context.Context, id string) (*types.QualityRound, error)
	// QueryDomains returns all registered domains.
	QueryDomains(ctx context.Context) ([]*types.Domain, error)
	// QueryDataBounties returns bounties, optionally filtered by domain.
	QueryDataBounties(ctx context.Context, domain string) ([]*types.DataBounty, error)
	// QueryProtocolStats returns aggregate protocol statistics.
	QueryProtocolStats(ctx context.Context) (activeRounds uint64, err error)
	// GetAddress returns the signer's bech32 address.
	GetAddress() string
}

// ToKClient is the main SDK client for agents to interact with the Tree of Knowledge.
// All exported methods are safe for concurrent use.
type ToKClient struct {
	mu     sync.RWMutex
	chain  ChainClient
	salts  *SaltStore
	config Config

	// cachedParams caches chain params to avoid repeated queries.
	cachedParams     *types.Params
	cachedParamsTime time.Time
}

// NewToKClient creates a new agent SDK client.
// For production use, pass a real ChainClient implementation.
// For testing, use NewToKClientWithChain with a mock.
func NewToKClient(cfg Config) (*ToKClient, error) {
	if cfg.NodeURL == "" {
		return nil, fmt.Errorf("NodeURL is required")
	}
	if cfg.ChainID == "" {
		return nil, fmt.Errorf("ChainID is required")
	}
	if cfg.FromName == "" {
		return nil, fmt.Errorf("FromName is required")
	}
	if cfg.KeyringDir == "" {
		cfg.KeyringDir = "~/.zeroned"
	}
	if cfg.KeyringBackend == "" {
		cfg.KeyringBackend = "test"
	}
	if cfg.GasAdjustment == 0 {
		cfg.GasAdjustment = 1.5
	}
	if cfg.Gas == "" {
		cfg.Gas = "auto"
	}
	if cfg.BroadcastMode == "" {
		cfg.BroadcastMode = "sync"
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 2 * time.Second
	}

	salts := NewSaltStore(cfg.KeyringDir)

	return &ToKClient{
		config: cfg,
		salts:  salts,
	}, nil
}

// NewToKClientWithChain creates a ToKClient with a provided ChainClient.
// This is the primary constructor for both production and testing.
func NewToKClientWithChain(cfg Config, chain ChainClient) *ToKClient {
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 2 * time.Second
	}
	keyringDir := cfg.KeyringDir
	if keyringDir == "" {
		keyringDir = "~/.zeroned"
	}

	return &ToKClient{
		config: cfg,
		chain:  chain,
		salts:  NewSaltStore(keyringDir),
	}
}

// Salts returns the salt store for direct access.
func (c *ToKClient) Salts() *SaltStore {
	return c.salts
}

// Address returns the agent's bech32 address.
func (c *ToKClient) Address() string {
	if c.chain != nil {
		return c.chain.GetAddress()
	}
	return ""
}

// getBaseStake queries chain params for the minimum submission stake, with caching.
func (c *ToKClient) getBaseStake(ctx context.Context) string {
	c.mu.RLock()
	if c.cachedParams != nil && time.Since(c.cachedParamsTime) < 5*time.Minute {
		stake := c.cachedParams.MinSubmissionStake
		c.mu.RUnlock()
		if stake != "" {
			return stake
		}
		return defaultBaseStake
	}
	c.mu.RUnlock()

	// Query chain
	params, err := c.chain.QueryParams(ctx)
	if err != nil || params == nil {
		return defaultBaseStake
	}

	c.mu.Lock()
	c.cachedParams = params
	c.cachedParamsTime = time.Now()
	c.mu.Unlock()

	if params.MinSubmissionStake != "" {
		return params.MinSubmissionStake
	}
	return defaultBaseStake
}

// computeStake calculates the stake from difficulty, querying chain params.
func (c *ToKClient) computeStake(ctx context.Context, difficulty, stakeOverride string) (string, error) {
	if stakeOverride != "" {
		return stakeOverride, nil
	}

	mult, err := difficultyMultiplier(difficulty)
	if err != nil {
		return "", err
	}

	baseStake := c.getBaseStake(ctx)
	return calculateStake(baseStake, mult)
}

// broadcastWithRetry broadcasts a message with configurable retry logic.
func (c *ToKClient) broadcastWithRetry(ctx context.Context, msg interface{}) (string, error) {
	var lastErr error
	for i := 0; i < c.config.MaxRetries; i++ {
		txHash, err := c.chain.BroadcastMsg(ctx, msg)
		if err == nil {
			return txHash, nil
		}
		lastErr = err

		if i < c.config.MaxRetries-1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}
	}
	return "", fmt.Errorf("broadcast failed after %d retries: %w", c.config.MaxRetries, lastErr)
}
