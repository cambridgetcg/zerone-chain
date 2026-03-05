package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// CuratorCommissionBPS is the curator's share of access fees (95%).
	CuratorCommissionBPS = 950_000
	// MaxBPSDenom is the denominator for BPS calculations.
	MaxBPSDenom = 1_000_000
)

// CreateDataset creates a new dataset with the given filter criteria.
func (k Keeper) CreateDataset(ctx context.Context, msg *types.MsgCreateDataset) (*types.MsgCreateDatasetResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	datasetID := k.NextDatasetID(ctx)
	dataset := &types.Dataset{
		Id:             datasetID,
		Name:           msg.Name,
		Description:    msg.Description,
		Domain:         msg.Domain,
		License:        msg.License,
		FilterType:     msg.FilterType,
		FilterLanguage: msg.FilterLanguage,
		FilterTags:     msg.FilterTags,
		MinQuality:     msg.MinQuality,
		PricePerSample: msg.PricePerSample,
		BulkPrice:      msg.BulkPrice,
		Curator:        msg.Curator,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
	}

	// Compute initial stats by running filter
	count, tokens := k.countMatchingSamples(ctx, dataset)
	dataset.SampleCount = count
	dataset.TotalTokens = tokens

	if err := k.SetDataset(ctx, dataset); err != nil {
		return nil, err
	}

	// Index by domain if domain filter is set
	if dataset.Domain != "" {
		_ = k.SetDatasetDomainIndex(ctx, dataset.Domain, datasetID)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"dataset_created",
		sdk.NewAttribute("dataset_id", datasetID),
		sdk.NewAttribute("curator", msg.Curator),
		sdk.NewAttribute("name", msg.Name),
		sdk.NewAttribute("sample_count", fmt.Sprintf("%d", dataset.SampleCount)),
	))

	return &types.MsgCreateDatasetResponse{DatasetId: datasetID}, nil
}

// AccessDataset processes a dataset access request with bulk pricing and per-sample updates.
func (k Keeper) AccessDataset(ctx context.Context, msg *types.MsgAccessDataset) (*types.MsgAccessDatasetResponse, error) {
	return k.AccessDatasetWithPricing(ctx, msg)
}

// countMatchingSamples counts samples matching a dataset's filters and estimates total tokens.
func (k Keeper) countMatchingSamples(ctx context.Context, dataset *types.Dataset) (uint64, uint64) {
	var count, totalTokens uint64

	if dataset.Domain != "" {
		// Use domain index for efficient first filter
		sampleIDs := k.GetSamplesByDomain(ctx, dataset.Domain)
		for _, id := range sampleIDs {
			sample, found := k.GetSample(ctx, id)
			if !found {
				continue
			}
			if matchesDatasetFilter(sample, dataset) {
				count++
				totalTokens += estimateTokens(sample)
			}
		}
	} else {
		// No domain filter — iterate all samples
		k.IterateSamples(ctx, func(sample *types.Sample) bool {
			if matchesDatasetFilter(sample, dataset) {
				count++
				totalTokens += estimateTokens(sample)
			}
			return false
		})
	}

	return count, totalTokens
}

// matchesDatasetFilter checks if a sample matches all dataset filter criteria.
func matchesDatasetFilter(sample *types.Sample, dataset *types.Dataset) bool {
	// Must be active (gold/silver/bronze)
	if !isActiveSample(sample.Status) {
		return false
	}

	// Domain match (if set)
	if dataset.Domain != "" && sample.Domain != dataset.Domain {
		return false
	}

	// SampleType match (if set, 0 = UNSPECIFIED means no filter)
	if dataset.FilterType != types.SampleType_SAMPLE_TYPE_UNSPECIFIED && sample.SampleType != dataset.FilterType {
		return false
	}

	// Language match (if set)
	if dataset.FilterLanguage != "" && sample.Language != dataset.FilterLanguage {
		return false
	}

	// Tag intersection (if set) — sample must have at least one matching tag
	if len(dataset.FilterTags) > 0 {
		if !hasTagIntersection(sample.Tags, dataset.FilterTags) {
			return false
		}
	}

	// MinQuality threshold
	if dataset.MinQuality > 0 && sample.QualityScore < dataset.MinQuality {
		return false
	}

	return true
}

// isActiveSample returns true if the sample status is gold, silver, or bronze.
func isActiveSample(status types.SampleStatus) bool {
	return status == types.SampleStatus_SAMPLE_STATUS_GOLD ||
		status == types.SampleStatus_SAMPLE_STATUS_SILVER ||
		status == types.SampleStatus_SAMPLE_STATUS_BRONZE
}

// hasTagIntersection returns true if the two tag slices share at least one element.
func hasTagIntersection(sampleTags, filterTags []string) bool {
	tagSet := make(map[string]struct{}, len(filterTags))
	for _, t := range filterTags {
		tagSet[t] = struct{}{}
	}
	for _, t := range sampleTags {
		if _, ok := tagSet[t]; ok {
			return true
		}
	}
	return false
}

// estimateTokens provides a rough token estimate (content bytes / 4).
func estimateTokens(sample *types.Sample) uint64 {
	contentLen := len(sample.Content)
	if contentLen == 0 {
		return 0
	}
	return uint64(contentLen) / 4
}
