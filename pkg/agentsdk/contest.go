package agentsdk

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ContestSample disputes a validated sample, triggering re-review.
func (c *ToKClient) ContestSample(ctx context.Context, req ContestRequest) (*ContestResult, error) {
	if req.SampleID == "" {
		return nil, fmt.Errorf("sample ID is required")
	}
	if req.Stake == "" {
		return nil, fmt.Errorf("stake is required")
	}
	if req.Reason == "" {
		return nil, fmt.Errorf("reason is required")
	}

	contestType := types.ContestType_CONTEST_TYPE_UNSPECIFIED
	if req.ContestType != "" {
		ct, err := parseContestType(req.ContestType)
		if err != nil {
			return nil, err
		}
		contestType = ct
	}

	msg := &types.MsgContestSample{
		Challenger:  c.chain.GetAddress(),
		SampleId:    req.SampleID,
		Stake:       req.Stake,
		Reason:      req.Reason,
		ContestType: contestType,
	}

	txHash, err := c.broadcastWithRetry(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to contest sample: %w", err)
	}

	return &ContestResult{
		TxHash: txHash,
	}, nil
}

// SponsorSample funds a sample to prevent pruning.
func (c *ToKClient) SponsorSample(ctx context.Context, req SponsorRequest) (string, error) {
	if req.SampleID == "" {
		return "", fmt.Errorf("sample ID is required")
	}
	if req.Amount == "" {
		return "", fmt.Errorf("amount is required")
	}
	if req.DurationBlocks == 0 {
		return "", fmt.Errorf("duration blocks must be > 0")
	}

	msg := &types.MsgSponsorSample{
		Sponsor:        c.chain.GetAddress(),
		SampleId:       req.SampleID,
		Amount:         req.Amount,
		DurationBlocks: req.DurationBlocks,
	}

	txHash, err := c.broadcastWithRetry(ctx, msg)
	if err != nil {
		return "", fmt.Errorf("failed to sponsor sample: %w", err)
	}

	return txHash, nil
}
