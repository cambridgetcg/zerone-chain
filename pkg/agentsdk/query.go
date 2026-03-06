package agentsdk

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// GetDashboard queries the agent's aggregate dashboard: submissions, reputation, stakes, fitness.
func (c *ToKClient) GetDashboard(ctx context.Context) (*Dashboard, error) {
	addr := c.chain.GetAddress()
	if addr == "" {
		return nil, fmt.Errorf("no address configured")
	}

	dash := &Dashboard{Address: addr}

	// 1) Submissions by submitter
	samples, err := c.chain.QuerySamplesBySubmitter(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to query samples: %w", err)
	}

	dash.TotalSubmissions = len(samples)
	for _, s := range samples {
		switch s.Status {
		case types.SampleStatus_SAMPLE_STATUS_GOLD,
			types.SampleStatus_SAMPLE_STATUS_SILVER,
			types.SampleStatus_SAMPLE_STATUS_BRONZE:
			dash.AcceptedCount++
		case types.SampleStatus_SAMPLE_STATUS_REJECTED:
			dash.RejectedCount++
		case types.SampleStatus_SAMPLE_STATUS_PENDING,
			types.SampleStatus_SAMPLE_STATUS_IN_REVIEW:
			dash.PendingCount++

			// Track as active stake
			dash.ActiveStakes = append(dash.ActiveStakes, StakeEntry{
				SampleID: s.Id,
				Domain:   s.Domain,
				Status:   s.Status.String(),
			})
		}
	}

	if dash.TotalSubmissions > 0 {
		dash.AcceptanceRate = float64(dash.AcceptedCount) / float64(dash.TotalSubmissions) * 100
	}

	// 2) Reputation per domain
	domains, err := c.chain.QueryDomains(ctx)
	if err == nil {
		_ = domains // Reputation query would need store-level access
		// For SDK, we populate what we can from the chain client
	}

	// 3) Pending reviews (active rounds count)
	activeRounds, err := c.chain.QueryProtocolStats(ctx)
	if err == nil {
		dash.PendingReviews = int(activeRounds)
	}

	return dash, nil
}

// GetReputation queries the agent's reputation in a specific domain.
func (c *ToKClient) GetReputation(ctx context.Context, domain string) (*DomainReputation, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	// Reputation data requires store-level queries not available via standard gRPC.
	// Return a stub that agents can populate from the dashboard or direct store queries.
	return &DomainReputation{
		Domain: domain,
	}, nil
}

// GetFitness queries the fitness record for a TDU (sample).
func (c *ToKClient) GetFitness(ctx context.Context, tduID string) (*FitnessEntry, error) {
	if tduID == "" {
		return nil, fmt.Errorf("TDU ID is required")
	}

	// Query the sample to check it exists
	sample, err := c.chain.QuerySample(ctx, tduID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sample: %w", err)
	}

	scoreStr := fmt.Sprintf("%d", sample.FitnessScore)
	return &FitnessEntry{
		SampleID:        sample.Id,
		FitnessScore:    scoreStr,
		LifecycleStatus: fitnessStatusFromUint(sample.FitnessScore),
	}, nil
}

// ListOpenBounties lists unclaimed data bounties, optionally filtered by domain.
func (c *ToKClient) ListOpenBounties(ctx context.Context, domain string) ([]BountyEntry, error) {
	bounties, err := c.chain.QueryDataBounties(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to query bounties: %w", err)
	}

	var result []BountyEntry
	for _, b := range bounties {
		if b.Claimed {
			continue
		}
		result = append(result, BountyEntry{
			ID:           b.Id,
			Domain:       b.Domain,
			Topic:        b.Subject,
			RewardAmount: b.RewardAmount,
			ExpiresAt:    b.ExpiresAtBlock,
			Claimed:      b.Claimed,
		})
	}
	return result, nil
}

// GetMyActiveRounds returns quality rounds where this agent is an active reviewer.
func (c *ToKClient) GetMyActiveRounds(ctx context.Context) ([]ActiveRound, error) {
	// This requires iterating rounds, which is expensive.
	// For now, list pending salts as a proxy for committed rounds.
	pendingRounds, err := c.salts.ListPending()
	if err != nil {
		return nil, fmt.Errorf("failed to list pending: %w", err)
	}

	var result []ActiveRound
	myAddr := c.chain.GetAddress()

	for _, roundID := range pendingRounds {
		round, err := c.chain.QueryQualityRound(ctx, roundID)
		if err != nil {
			continue
		}

		ar := ActiveRound{
			RoundID:      round.Id,
			SubmissionID: round.SubmissionId,
			Phase:        round.Phase.String(),
		}

		// Check if we committed/revealed
		for _, commit := range round.Commits {
			if commit.Verifier == myAddr {
				ar.Committed = true
				break
			}
		}
		for _, reveal := range round.Reveals {
			if reveal.Verifier == myAddr {
				ar.Revealed = true
				break
			}
		}

		result = append(result, ar)
	}

	return result, nil
}

// fitnessStatusFromUint derives lifecycle status from a uint64 fitness score.
// Scores are in basis points: 700000 = 0.7, 300000 = 0.3, 100000 = 0.1.
func fitnessStatusFromUint(score uint64) string {
	switch {
	case score >= 700000:
		return "core"
	case score >= 300000:
		return "active"
	case score >= 100000:
		return "dormant"
	default:
		return "pruned"
	}
}
