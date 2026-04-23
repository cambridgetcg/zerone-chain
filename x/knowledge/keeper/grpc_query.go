package keeper

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

type queryServer struct {
	keeper Keeper
	types.UnimplementedQueryServer
}

// NewQueryServerImpl returns a types.QueryServer backed by the given Keeper.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{keeper: keeper}
}

func (q *queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryParamsResponse{Params: params}, nil
}

func (q *queryServer) Fact(ctx context.Context, req *types.QueryFactRequest) (*types.QueryFactResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "fact id is required")
	}
	fact, found := q.keeper.GetFact(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.Id)
	}

	// Track query — increment counter and record receipt for satisfaction rating
	if req.TrackQuery {
		q.keeper.IncrementFactQueryCount(ctx, req.Id)
		if req.Querier != "" {
			_ = q.keeper.RecordQueryReceipt(ctx, req.Querier, req.Id)
		}
	}

	return &types.QueryFactResponse{Fact: fact}, nil
}

func (q *queryServer) Facts(ctx context.Context, req *types.QueryFactsRequest) (*types.QueryFactsResponse, error) {
	var facts []*types.Fact

	// If domain filter is specified, use the secondary index
	if req.Domain != "" {
		q.keeper.IterateFactsByDomain(ctx, req.Domain, func(factID string) bool {
			fact, found := q.keeper.GetFact(ctx, factID)
			if found {
				if matchesFactFilters(fact, req.Status, req.Category, req.ClaimType) {
					facts = append(facts, fact)
				}
			}
			return false
		})
	} else {
		q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
			if matchesFactFilters(fact, req.Status, req.Category, req.ClaimType) {
				facts = append(facts, fact)
			}
			return false
		})
	}

	return &types.QueryFactsResponse{Facts: facts}, nil
}

func (q *queryServer) FactsByDomain(ctx context.Context, req *types.QueryFactsByDomainRequest) (*types.QueryFactsByDomainResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	var facts []*types.Fact
	q.keeper.IterateFactsByDomain(ctx, req.Domain, func(factID string) bool {
		fact, found := q.keeper.GetFact(ctx, factID)
		if found {
			facts = append(facts, fact)
		}
		return false
	})

	return &types.QueryFactsByDomainResponse{Facts: facts}, nil
}

func (q *queryServer) FactsBySubmitter(ctx context.Context, req *types.QueryFactsBySubmitterRequest) (*types.QueryFactsBySubmitterResponse, error) {
	if req.Submitter == "" {
		return nil, status.Error(codes.InvalidArgument, "submitter is required")
	}

	var facts []*types.Fact
	q.keeper.IterateFactsBySubmitter(ctx, req.Submitter, func(factID string) bool {
		fact, found := q.keeper.GetFact(ctx, factID)
		if found {
			facts = append(facts, fact)
		}
		return false
	})

	return &types.QueryFactsBySubmitterResponse{Facts: facts}, nil
}

func (q *queryServer) Claim(ctx context.Context, req *types.QueryClaimRequest) (*types.QueryClaimResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "claim id is required")
	}
	claim, found := q.keeper.GetClaim(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "claim %s not found", req.Id)
	}
	return &types.QueryClaimResponse{Claim: claim}, nil
}

func (q *queryServer) PendingClaims(ctx context.Context, _ *types.QueryPendingClaimsRequest) (*types.QueryPendingClaimsResponse, error) {
	var claims []*types.Claim
	q.keeper.IterateClaims(ctx, func(claim *types.Claim) bool {
		if claim.Status == types.ClaimStatus_CLAIM_STATUS_PENDING {
			claims = append(claims, claim)
		}
		return false
	})
	return &types.QueryPendingClaimsResponse{Claims: claims}, nil
}

func (q *queryServer) VerificationRound(ctx context.Context, req *types.QueryVerificationRoundRequest) (*types.QueryVerificationRoundResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "round id is required")
	}
	round, found := q.keeper.GetVerificationRound(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "verification round %s not found", req.Id)
	}
	return &types.QueryVerificationRoundResponse{Round: round}, nil
}

func (q *queryServer) Domain(ctx context.Context, req *types.QueryDomainRequest) (*types.QueryDomainResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "domain name is required")
	}
	domain, found := q.keeper.GetDomain(ctx, req.Name)
	if !found {
		return nil, status.Errorf(codes.NotFound, "domain %s not found", req.Name)
	}
	return &types.QueryDomainResponse{Domain: domain}, nil
}

func (q *queryServer) Domains(ctx context.Context, _ *types.QueryDomainsRequest) (*types.QueryDomainsResponse, error) {
	var domains []*types.Domain
	q.keeper.IterateDomains(ctx, func(domain *types.Domain) bool {
		domains = append(domains, domain)
		return false
	})
	return &types.QueryDomainsResponse{Domains: domains}, nil
}

func (q *queryServer) FactConfidence(ctx context.Context, req *types.QueryFactConfidenceRequest) (*types.QueryFactConfidenceResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "fact id is required")
	}
	fact, found := q.keeper.GetFact(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.Id)
	}
	return &types.QueryFactConfidenceResponse{Confidence: fact.Confidence}, nil
}

func (q *queryServer) FactCitationCount(ctx context.Context, req *types.QueryFactCitationCountRequest) (*types.QueryFactCitationCountResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "fact id is required")
	}
	fact, found := q.keeper.GetFact(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.Id)
	}
	return &types.QueryFactCitationCountResponse{
		Count: fact.CitationCount + fact.IncomingCitationCount,
	}, nil
}

func (q *queryServer) FactRelations(ctx context.Context, req *types.QueryFactRelationsRequest) (*types.QueryFactRelationsResponse, error) {
	if req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id is required")
	}

	// Verify fact exists
	if _, found := q.keeper.GetFact(ctx, req.FactId); !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}

	direction := req.Direction
	if direction == "" {
		direction = "both"
	}

	var relations []*types.FactRelation

	if direction == "outgoing" || direction == "both" {
		outgoing, err := q.keeper.GetFactRelations(ctx, req.FactId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		relations = append(relations, outgoing...)
	}

	if direction == "incoming" || direction == "both" {
		incoming, err := q.keeper.GetIncomingRelations(ctx, req.FactId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		relations = append(relations, incoming...)
	}

	// Apply optional type filter
	if req.Relation != types.RelationType_RELATION_TYPE_UNSPECIFIED {
		var filtered []*types.FactRelation
		for _, rel := range relations {
			if rel.Relation == req.Relation {
				filtered = append(filtered, rel)
			}
		}
		relations = filtered
	}

	return &types.QueryFactRelationsResponse{Relations: relations}, nil
}

func (q *queryServer) FactsBySubject(ctx context.Context, req *types.QueryFactsBySubjectRequest) (*types.QueryFactsBySubjectResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}
	if req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}

	factID := q.keeper.FindFactBySubjectPredicate(ctx, req.Domain, req.Subject, "")
	if factID == "" {
		return &types.QueryFactsBySubjectResponse{}, nil
	}

	fact, found := q.keeper.GetFact(ctx, factID)
	if !found {
		return &types.QueryFactsBySubjectResponse{}, nil
	}

	return &types.QueryFactsBySubjectResponse{Facts: []*types.Fact{fact}}, nil
}

func (q *queryServer) FactsByTag(ctx context.Context, req *types.QueryFactsByTagRequest) (*types.QueryFactsByTagResponse, error) {
	if req.Tag == "" {
		return nil, status.Error(codes.InvalidArgument, "tag is required")
	}

	factIDs, err := q.keeper.FindFactsByTag(ctx, req.Tag)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var facts []*types.Fact
	for _, id := range factIDs {
		fact, found := q.keeper.GetFact(ctx, id)
		if found {
			facts = append(facts, fact)
		}
	}

	return &types.QueryFactsByTagResponse{Facts: facts}, nil
}

func (q *queryServer) FactByCanonical(ctx context.Context, req *types.QueryFactByCanonicalRequest) (*types.QueryFactByCanonicalResponse, error) {
	canonicalHash := req.CanonicalHash
	if canonicalHash == "" && req.CanonicalForm != "" {
		// Hash the provided form server-side
		normalized := types.NormalizeCanonicalForm(req.CanonicalForm)
		canonicalHash = types.HashCanonicalForm(normalized)
	}
	if canonicalHash == "" {
		return nil, status.Error(codes.InvalidArgument, "canonical_hash or canonical_form is required")
	}

	id, found := q.keeper.GetClaimByCanonicalHash(ctx, canonicalHash)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no fact/claim found for canonical hash %s", canonicalHash)
	}

	// Try fact first, then claim's provisional fact
	fact, found := q.keeper.GetFact(ctx, id)
	if found {
		return &types.QueryFactByCanonicalResponse{Fact: fact}, nil
	}

	// The index might point to a claim ID — check if that claim has a fact
	claim, found := q.keeper.GetClaim(ctx, id)
	if found && claim.ProvisionalFactId != "" {
		fact, found = q.keeper.GetFact(ctx, claim.ProvisionalFactId)
		if found {
			return &types.QueryFactByCanonicalResponse{Fact: fact}, nil
		}
	}

	// Search for fact with matching canonical hash
	var matchedFact *types.Fact
	q.keeper.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.CanonicalHash == canonicalHash {
			matchedFact = f
			return true
		}
		return false
	})
	if matchedFact != nil {
		return &types.QueryFactByCanonicalResponse{Fact: matchedFact}, nil
	}

	return nil, status.Errorf(codes.NotFound, "no fact found for canonical hash %s", canonicalHash)
}

func (q *queryServer) FactsByFitness(ctx context.Context, req *types.QueryFactsByFitnessRequest) (*types.QueryFactsByFitnessResponse, error) {
	ascending := req.Order == "asc"
	facts := q.keeper.GetFactsByFitness(ctx, req.Domain, req.MinFitness, req.Limit, ascending)
	return &types.QueryFactsByFitnessResponse{Facts: facts}, nil
}

// matchesFactFilters checks if a fact passes optional status, category, and claim type filters.
func matchesFactFilters(fact *types.Fact, statusFilter, categoryFilter string, claimTypeFilter types.ClaimType) bool {
	if statusFilter != "" {
		if fact.Status.String() != statusFilter {
			return false
		}
	}
	if categoryFilter != "" {
		if fact.Category != categoryFilter {
			return false
		}
	}
	if claimTypeFilter != types.ClaimType_CLAIM_TYPE_UNSPECIFIED {
		if fact.ClaimType != claimTypeFilter {
			return false
		}
	}
	return true
}

func (q *queryServer) BootstrapFundStatus(ctx context.Context, _ *types.QueryBootstrapFundStatusRequest) (*types.QueryBootstrapFundStatusResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	balance := q.keeper.GetBootstrapFundBalance(ctx)

	// Calculate total claims funded (sum of all per-address counts would be expensive,
	// so we track via epoch counts across all epochs — approximate via current epoch)
	epoch := q.keeper.CurrentEpoch(ctx)
	var totalClaims uint64
	for e := uint64(0); e <= epoch; e++ {
		totalClaims += q.keeper.GetBootstrapEpochCount(ctx, e)
	}

	// Calculate remaining per epoch
	maxPerEpoch, _ := strconv.ParseUint(params.BootstrapFundMaxPerEpoch, 10, 64)
	currentEpochCount := q.keeper.GetBootstrapEpochCount(ctx, epoch)
	remaining := uint64(0)
	if maxPerEpoch > currentEpochCount {
		remaining = maxPerEpoch - currentEpochCount
	}

	return &types.QueryBootstrapFundStatusResponse{
		Balance:            balance.Amount.String(),
		Enabled:            params.BootstrapFundEnabled,
		TotalClaimsFunded:  fmt.Sprintf("%d", totalClaims),
		TotalAmountSpent:   "0", // Not tracked separately — can be derived from genesis allocation minus balance
		RemainingPerEpoch:  fmt.Sprintf("%d", remaining),
	}, nil
}

func (q *queryServer) FactsAtRisk(ctx context.Context, req *types.QueryFactsAtRiskRequest) (*types.QueryFactsAtRiskResponse, error) {
	limit := req.Limit
	if limit == 0 {
		limit = 50
	}

	var facts []*types.Fact
	q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
		if fact.Status != types.FactStatus_FACT_STATUS_AT_RISK &&
			fact.Status != types.FactStatus_FACT_STATUS_EXPIRED {
			return false
		}
		if req.Domain != "" && fact.Domain != req.Domain {
			return false
		}
		facts = append(facts, fact)
		return uint64(len(facts)) >= limit
	})

	return &types.QueryFactsAtRiskResponse{Facts: facts}, nil
}

// MetabolismStatus returns aggregate metabolism health statistics.
func (q *queryServer) MetabolismStatus(ctx context.Context, req *types.QueryMetabolismStatusRequest) (*types.QueryMetabolismStatusResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	var totalFacts, activeCount, atRiskCount, expiredCount, prunedCount uint64
	var totalEnergy uint64

	q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
		totalFacts++
		totalEnergy += fact.Energy
		switch fact.Status {
		case types.FactStatus_FACT_STATUS_VERIFIED, types.FactStatus_FACT_STATUS_ACTIVE, types.FactStatus_FACT_STATUS_PROVISIONAL:
			activeCount++
		case types.FactStatus_FACT_STATUS_AT_RISK:
			atRiskCount++
		case types.FactStatus_FACT_STATUS_EXPIRED:
			expiredCount++
		case types.FactStatus_FACT_STATUS_PRUNED:
			prunedCount++
		}
		return false
	})

	avgEnergy := uint64(0)
	if totalFacts > 0 {
		avgEnergy = totalEnergy / totalFacts
	}

	currentEpoch := uint64(0)
	nextEpochBlock := uint64(0)
	if params.FitnessEpochBlocks > 0 {
		currentEpoch = height / params.FitnessEpochBlocks
		nextEpochBlock = (currentEpoch + 1) * params.FitnessEpochBlocks
	}

	return &types.QueryMetabolismStatusResponse{
		TotalFacts:     totalFacts,
		ActiveCount:    activeCount,
		AtRiskCount:    atRiskCount,
		ExpiredCount:   expiredCount,
		PrunedCount:    prunedCount,
		AvgEnergy:      avgEnergy,
		CurrentEpoch:   currentEpoch,
		NextEpochBlock: nextEpochBlock,
	}, nil
}

// FactLineage traces a fact's ancestry up to the root.
func (q *queryServer) FactLineage(ctx context.Context, req *types.QueryFactLineageRequest) (*types.QueryFactLineageResponse, error) {
	if req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id is required")
	}

	fact, found := q.keeper.GetFact(ctx, req.FactId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}

	maxDepth := req.Depth
	if maxDepth == 0 {
		maxDepth = 100 // safe upper bound to reach root
	}

	var ancestors []*types.Fact
	currentID := fact.ParentFactId
	depth := uint64(0)

	for currentID != "" && depth < maxDepth {
		ancestor, found := q.keeper.GetFact(ctx, currentID)
		if !found {
			break
		}
		ancestors = append(ancestors, ancestor)
		currentID = ancestor.ParentFactId
		depth++
	}

	rootID := ""
	if len(ancestors) > 0 {
		rootID = ancestors[len(ancestors)-1].Id
	} else if fact.LineageRootId != "" {
		rootID = fact.LineageRootId
	}

	return &types.QueryFactLineageResponse{
		Ancestors: ancestors,
		RootId:    rootID,
	}, nil
}

// FactProgeny returns a fact's descendant tree.
func (q *queryServer) FactProgeny(ctx context.Context, req *types.QueryFactProgenyRequest) (*types.QueryFactProgenyResponse, error) {
	if req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id is required")
	}

	root, found := q.keeper.GetFact(ctx, req.FactId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}

	maxDepth := req.Depth
	if maxDepth == 0 {
		maxDepth = 3
	}

	tree := q.buildProgenyTree(ctx, root, maxDepth, 0)

	return &types.QueryFactProgenyResponse{
		Root: root,
		Tree: tree,
	}, nil
}

// buildProgenyTree recursively builds the descendant tree for a fact.
func (q *queryServer) buildProgenyTree(ctx context.Context, parent *types.Fact, maxDepth, currentDepth uint64) []*types.FactWithChildren {
	if currentDepth >= maxDepth || len(parent.ChildFactIds) == 0 {
		return nil
	}

	var result []*types.FactWithChildren
	for _, childID := range parent.ChildFactIds {
		child, found := q.keeper.GetFact(ctx, childID)
		if !found {
			continue
		}
		node := &types.FactWithChildren{
			Fact:     child,
			Children: q.buildProgenyTree(ctx, child, maxDepth, currentDepth+1),
		}
		result = append(result, node)
	}
	return result
}

// Methodologies returns every registered methodology (Phase 1).
func (q *queryServer) Methodologies(ctx context.Context, _ *types.QueryMethodologiesRequest) (*types.QueryMethodologiesResponse, error) {
	return &types.QueryMethodologiesResponse{
		Methodologies: q.keeper.GetAllMethodologies(ctx),
	}, nil
}

// Methodology returns a single methodology by id, or found=false.
func (q *queryServer) Methodology(ctx context.Context, req *types.QueryMethodologyRequest) (*types.QueryMethodologyResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	m, found := q.keeper.GetMethodology(ctx, req.Id)
	return &types.QueryMethodologyResponse{
		Methodology: m,
		Found:       found,
	}, nil
}

// NormativeCommitments returns every registered commitment (Phase 6).
func (q *queryServer) NormativeCommitments(ctx context.Context, _ *types.QueryNormativeCommitmentsRequest) (*types.QueryNormativeCommitmentsResponse, error) {
	return &types.QueryNormativeCommitmentsResponse{
		Commitments: q.keeper.GetAllNormativeCommitments(ctx),
	}, nil
}

// NormativeCommitment returns a single commitment by id.
func (q *queryServer) NormativeCommitment(ctx context.Context, req *types.QueryNormativeCommitmentRequest) (*types.QueryNormativeCommitmentResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	c, found := q.keeper.GetNormativeCommitment(ctx, req.Id)
	return &types.QueryNormativeCommitmentResponse{
		Commitment: c,
		Found:      found,
	}, nil
}

// ─── Training pipeline exports (Phase 9) ───────────────────────────────────

// MethodCorpus exports method-stamped facts for positive-exemplar training.
func (q *queryServer) MethodCorpus(ctx context.Context, req *types.QueryMethodCorpusRequest) (*types.QueryMethodCorpusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	limit := req.Limit
	if limit == 0 || limit > 1000 {
		limit = 100
	}
	offset := req.Offset

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshotHeight := uint64(sdkCtx.BlockHeight())

	var entries []*types.MethodCorpusEntry
	var total uint32
	var skipped uint32

	q.keeper.IterateFactsForTraining(ctx, req.MethodId, req.MinCorroboration, req.MinTier,
		func(fact *types.Fact, tier types.TrainingQualityTier) bool {
			total++
			if skipped < offset {
				skipped++
				return false
			}
			if uint32(len(entries)) >= limit {
				return false
			}
			// Support edges for training: typed inference chain out of this fact.
			edges, _ := q.keeper.GetFactRelations(ctx, fact.Id)
			// Only support-bearing edges are useful as reasoning training.
			filtered := edges[:0]
			for _, rel := range edges {
				if isSupportBearing(rel.Relation) {
					filtered = append(filtered, rel)
				}
			}
			entries = append(entries, &types.MethodCorpusEntry{
				Fact:         fact,
				Tier:         tier,
				SupportEdges: filtered,
			})
			return false
		})

	return &types.QueryMethodCorpusResponse{
		Entries:             entries,
		Total:               total,
		SnapshotBlockHeight: snapshotHeight,
	}, nil
}

// DisprovenCorpus exports DISPROVEN facts — the negative-example dataset.
func (q *queryServer) DisprovenCorpus(ctx context.Context, req *types.QueryDisprovenCorpusRequest) (*types.QueryDisprovenCorpusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	limit := req.Limit
	if limit == 0 || limit > 1000 {
		limit = 100
	}
	offset := req.Offset

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshotHeight := uint64(sdkCtx.BlockHeight())

	var entries []*types.DisprovenCorpusEntry
	var total uint32
	var skipped uint32

	q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
		if fact == nil || fact.Status != types.FactStatus_FACT_STATUS_DISPROVEN {
			return false
		}
		total++
		if skipped < offset {
			skipped++
			return false
		}
		if uint32(len(entries)) >= limit {
			return false
		}
		entries = append(entries, &types.DisprovenCorpusEntry{
			DisprovenFact:    fact,
			MethodId:         fact.MethodId,
			DisprovenAtBlock: fact.LastVerifiedBlock, // closest available stamp; full tracking in Phase 9 follow-up
		})
		return false
	})

	return &types.QueryDisprovenCorpusResponse{
		Entries:             entries,
		Total:               total,
		SnapshotBlockHeight: snapshotHeight,
	}, nil
}

// VindicationCorpus exports vindication records — correct-dissent examples.
func (q *queryServer) VindicationCorpus(ctx context.Context, req *types.QueryVindicationCorpusRequest) (*types.QueryVindicationCorpusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	limit := req.Limit
	if limit == 0 || limit > 1000 {
		limit = 100
	}
	offset := req.Offset

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshotHeight := uint64(sdkCtx.BlockHeight())

	var entries []*types.VindicationCorpusEntry
	var total uint32
	var skipped uint32

	q.keeper.IterateAllVindicationRecords(ctx, func(rec *types.VindicationRecord) bool {
		if rec == nil {
			return false
		}
		total++
		if skipped < offset {
			skipped++
			return false
		}
		if uint32(len(entries)) >= limit {
			return false
		}
		entries = append(entries, &types.VindicationCorpusEntry{
			FactId:             rec.FactId,
			Verifier:           rec.Verifier,
			// Vote is captured in VindicationEntry (pending) but not retained
			// on the executed record; training-pipeline consumers reconstruct
			// it from the round reveals if needed.
			Vote:              "",
			RefundAmount:      rec.RefundAmount,
			BonusAmount:       rec.BonusAmount,
			VindicatedAtBlock: rec.VindicatedAt,
			DisprovenByFactId: rec.DisprovenBy,
		})
		return false
	})

	return &types.QueryVindicationCorpusResponse{
		Entries:             entries,
		Total:               total,
		SnapshotBlockHeight: snapshotHeight,
	}, nil
}

// AgentCalibration returns the feedback-loop track record for a submitter.
func (q *queryServer) AgentCalibration(ctx context.Context, req *types.QueryAgentCalibrationRequest) (*types.QueryAgentCalibrationResponse, error) {
	if req == nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address is required")
	}
	c, found := q.keeper.GetAgentCalibration(ctx, req.Address)
	return &types.QueryAgentCalibrationResponse{
		Calibration: c,
		Found:       found,
	}, nil
}

// AgentLeaderboard ranks submitters by calibration score, optionally
// restricted to a single methodology's per-method stats.
func (q *queryServer) AgentLeaderboard(ctx context.Context, req *types.QueryAgentLeaderboardRequest) (*types.QueryAgentLeaderboardResponse, error) {
	if req == nil {
		req = &types.QueryAgentLeaderboardRequest{}
	}
	limit := req.Limit
	if limit == 0 || limit > 500 {
		limit = 50
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	snapshotHeight := uint64(sdkCtx.BlockHeight())

	type rankEntry struct {
		e *types.AgentLeaderboardEntry
	}
	var all []rankEntry

	q.keeper.IterateAgentCalibrations(ctx, func(c *types.AgentCalibration) bool {
		var submissions, accepted, corroborations, disproven uint64
		if req.MethodId == "" {
			submissions = c.TotalSubmissions
			accepted = c.Accepted
			corroborations = c.CorroborationsEarned
			disproven = c.DisprovenCount
		} else {
			ms, ok := c.PerMethod[req.MethodId]
			if !ok {
				return false
			}
			submissions = ms.Submissions
			accepted = ms.Accepted
			corroborations = ms.CorroborationsEarned
			disproven = ms.Disproven
		}
		if submissions < req.MinSubmissions {
			return false
		}
		all = append(all, rankEntry{e: &types.AgentLeaderboardEntry{
			Address:              c.Address,
			AccountType:          c.AccountType,
			Submissions:          submissions,
			Accepted:             accepted,
			CorroborationsEarned: corroborations,
			DisprovenCount:       disproven,
			CalibrationScoreBps:  c.CalibrationScoreBps,
		}})
		return false
	})

	// Sort by calibration_score_bps descending; tie-break by accepted.
	sort.Slice(all, func(i, j int) bool {
		if all[i].e.CalibrationScoreBps != all[j].e.CalibrationScoreBps {
			return all[i].e.CalibrationScoreBps > all[j].e.CalibrationScoreBps
		}
		return all[i].e.Accepted > all[j].e.Accepted
	})

	entries := make([]*types.AgentLeaderboardEntry, 0, limit)
	for i, r := range all {
		if uint32(i) >= limit {
			break
		}
		entries = append(entries, r.e)
	}

	return &types.QueryAgentLeaderboardResponse{
		Entries:             entries,
		SnapshotBlockHeight: snapshotHeight,
	}, nil
}

// TrainingQuality returns the computed tier for a single fact, with reason.
func (q *queryServer) TrainingQuality(ctx context.Context, req *types.QueryTrainingQualityRequest) (*types.QueryTrainingQualityResponse, error) {
	if req == nil || req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id required")
	}
	fact, found := q.keeper.GetFact(ctx, req.FactId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}
	tier, reason := ClassifyTrainingQuality(fact)
	return &types.QueryTrainingQualityResponse{
		Tier:               tier,
		CorroborationCount: fact.CorroborationCount,
		MethodId:           fact.MethodId,
		Status:             fact.Status,
		Reason:             reason,
	}, nil
}

// TrustProfile returns the consolidated provenance view for a single fact:
// own confidence, inherited floor, axiom distance, direct supporter /
// descendant counts, min confidence found in the support chain, and a
// computed grounded_score (ToK Wave 7).
//
// The grounded score is a single 0-1,000,000 BPS metric for UI/ranking:
//
//	axiom_weight = BPS² / (BPS + axiom_distance × AXIOM_DISTANCE_DECAY_BPS)
//	floor_weight = min(floor/own_confidence, 1.0)  (if floor > 0)
//	grounded    = own_confidence × axiom_weight × floor_weight / BPS²
//
// AXIOM_DISTANCE_DECAY_BPS = 50_000 → each hop from axioms trims ~5% of score.
func (q *queryServer) TrustProfile(ctx context.Context, req *types.QueryTrustProfileRequest) (*types.QueryTrustProfileResponse, error) {
	if req == nil || req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id is required")
	}
	fact, found := q.keeper.GetFact(ctx, req.FactId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}

	// Count direct support-bearing edges (outgoing = supporters).
	directSupporters := uint32(0)
	if outRels, err := q.keeper.GetFactRelations(ctx, fact.Id); err == nil {
		for _, rel := range outRels {
			if isSupportBearing(rel.Relation) {
				directSupporters++
			}
		}
	}
	// Count direct descendants (incoming = facts that cite me).
	directDescendants := uint32(0)
	if inRels, err := q.keeper.GetIncomingRelations(ctx, fact.Id); err == nil {
		for _, rel := range inRels {
			if isSupportBearing(rel.Relation) {
				directDescendants++
			}
		}
	}

	// Minimum confidence across the support chain: reuse ProofTree math but
	// without building the full tree — walk support edges BFS to small depth.
	minInAncestry := fact.Confidence
	if fact.DependencyConfidenceFloor > 0 && fact.DependencyConfidenceFloor < minInAncestry {
		minInAncestry = fact.DependencyConfidenceFloor
	}
	visited := map[string]bool{fact.Id: true}
	frontier := []string{fact.Id}
	const ancestryDepthCap = 6
	for depth := 0; depth < ancestryDepthCap && len(frontier) > 0; depth++ {
		var next []string
		for _, fid := range frontier {
			outRels, err := q.keeper.GetFactRelations(ctx, fid)
			if err != nil {
				continue
			}
			for _, rel := range outRels {
				if !isSupportBearing(rel.Relation) {
					continue
				}
				if visited[rel.TargetFactId] {
					continue
				}
				visited[rel.TargetFactId] = true
				target, ok := q.keeper.GetFact(ctx, rel.TargetFactId)
				if !ok {
					continue
				}
				conf := target.Confidence
				if target.DependencyConfidenceFloor > 0 && target.DependencyConfidenceFloor < conf {
					conf = target.DependencyConfidenceFloor
				}
				if conf > 0 && conf < minInAncestry {
					minInAncestry = conf
				}
				next = append(next, target.Id)
			}
		}
		frontier = next
	}

	grounded := computeGroundedScore(fact)

	return &types.QueryTrustProfileResponse{
		Fact:                          fact,
		OwnConfidenceBps:              fact.Confidence,
		DependencyConfidenceFloor:     fact.DependencyConfidenceFloor,
		AxiomDistance:                 fact.AxiomDistance,
		DirectSupporters:              directSupporters,
		DirectDescendants:             directDescendants,
		MinimumConfidenceInAncestry:   minInAncestry,
		Status:                        fact.Status,
		GroundedScoreBps:              grounded,
		MethodId:                      fact.MethodId,
		CorroborationCount:            fact.CorroborationCount,
		LastCorroboratedBlock:         fact.LastCorroboratedBlock,
	}, nil
}

func isSupportBearing(r types.RelationType) bool {
	switch r {
	case types.RelationType_RELATION_TYPE_SUPPORTS,
		types.RelationType_RELATION_TYPE_REQUIRES,
		types.RelationType_RELATION_TYPE_REFINES,
		types.RelationType_RELATION_TYPE_GENERALIZES,
		types.RelationType_RELATION_TYPE_CITES:
		return true
	}
	return false
}

// computeGroundedScore aggregates axiom distance + confidence floor +
// corroboration count + own confidence into a single BPS metric.
//
// Formula (Phase 2 update):
//
//	axiom_weight        = BPS² / (BPS + distance × AXIOM_DISTANCE_DECAY_BPS)
//	floor_weight        = min(floor/own, 1.0)   (1.0 if no floor)
//	corroboration_boost = 1 + min(count, MAX_CORR) × CORR_WEIGHT_BPS / BPS
//	grounded            = own × axiom_weight × floor_weight × corroboration_boost / BPS³
//	                    (clamped to [0, BPS])
//
// Popperian intuition: a fact's epistemic warrant can grow beyond its initial
// verification confidence as it survives falsification attempts. The score
// is still bounded at BPS (100%) because we cannot claim absolute certainty,
// but a well-corroborated low-confidence claim can now rise above a poorly-
// corroborated high-confidence one.
func computeGroundedScore(fact *types.Fact) uint64 {
	const bps uint64 = 1_000_000
	const axiomDistanceDecayBps uint64 = 50_000 // 5% per hop
	const maxCorroboration uint64 = 10          // cap: 10 surviving challenges saturates
	const corrWeightBps uint64 = 50_000         // +5% per corroboration up to cap → +50% max

	own := fact.Confidence
	if own == 0 {
		return 0
	}

	// axiom_weight: BPS² / (BPS + distance × decay). Bounded in (0, BPS].
	distance := uint64(fact.AxiomDistance)
	axiomDivisor := bps + distance*axiomDistanceDecayBps
	axiomWeight := bps * bps / axiomDivisor
	if axiomWeight > bps {
		axiomWeight = bps
	}

	// floor_weight: floor/own, capped at 1.0. If no floor declared, 1.0.
	var floorWeight uint64 = bps
	if fact.DependencyConfidenceFloor > 0 && fact.DependencyConfidenceFloor < own {
		floorWeight = fact.DependencyConfidenceFloor * bps / own
	}

	// corroboration_boost: BPS + min(count, MAX) × CORR_WEIGHT_BPS.
	// count=0 → 1× (no boost); count=10 → 1.5× (saturated).
	corr := fact.CorroborationCount
	if corr > maxCorroboration {
		corr = maxCorroboration
	}
	corrBoost := bps + corr*corrWeightBps

	// grounded = own × axiomWeight × floorWeight × corrBoost / BPS³
	// Intermediate steps kept in uint64 space.
	mid := own * axiomWeight / bps              // ≤ own
	mid = mid * floorWeight / bps               // ≤ own
	grounded := mid * corrBoost / bps           // can exceed own if corrBoost > BPS
	if grounded > bps {
		grounded = bps // absolute 100% cap — no claim earns more than this
	}
	return grounded
}

// DescendantTree is the dual of ProofTree: returns facts that transitively
// derive from the given fact by walking INCOMING support edges. The typed
// edge is named from the descendant's perspective (this descendant cited
// the parent-in-tree via relation X / inference Y).
func (q *queryServer) DescendantTree(ctx context.Context, req *types.QueryDescendantTreeRequest) (*types.QueryDescendantTreeResponse, error) {
	if req == nil || req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id is required")
	}
	root, found := q.keeper.GetFact(ctx, req.FactId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}
	maxDepth := req.MaxDepth
	if maxDepth == 0 {
		maxDepth = 5
	}
	visited := make(map[string]bool)
	nodeCount := uint32(1) // count the root
	maxDepthReached := uint32(0)
	descendants := q.walkDescendants(ctx, root.Id, 1, maxDepth, visited, &nodeCount, &maxDepthReached)
	return &types.QueryDescendantTreeResponse{
		Root:            root,
		Descendants:     descendants,
		TotalNodes:      nodeCount,
		MaxDepthReached: maxDepthReached,
	}, nil
}

// walkDescendants returns DescendantNodes for facts that cite factID via
// support-bearing relations.
func (q *queryServer) walkDescendants(
	ctx context.Context,
	factID string,
	depth, maxDepth uint32,
	visited map[string]bool,
	nodeCount *uint32,
	maxDepthReached *uint32,
) []*types.DescendantNode {
	if depth > maxDepth {
		return nil
	}
	incoming, err := q.keeper.GetIncomingRelations(ctx, factID)
	if err != nil {
		return nil
	}
	var out []*types.DescendantNode
	for _, rel := range incoming {
		switch rel.Relation {
		case types.RelationType_RELATION_TYPE_SUPPORTS,
			types.RelationType_RELATION_TYPE_REQUIRES,
			types.RelationType_RELATION_TYPE_REFINES,
			types.RelationType_RELATION_TYPE_GENERALIZES,
			types.RelationType_RELATION_TYPE_CITES:
		default:
			continue
		}
		if visited[rel.SourceFactId] {
			continue
		}
		visited[rel.SourceFactId] = true
		descendant, ok := q.keeper.GetFact(ctx, rel.SourceFactId)
		if !ok {
			continue
		}
		*nodeCount++
		if depth > *maxDepthReached {
			*maxDepthReached = depth
		}
		node := &types.DescendantNode{
			Fact:                     descendant,
			EdgeRelation:             rel.Relation,
			EdgeInference:            rel.Inference,
			EdgeInferenceStrengthBps: rel.InferenceStrengthBps,
			Depth:                    depth,
		}
		if depth < maxDepth {
			node.Descendants = q.walkDescendants(ctx, descendant.Id, depth+1, maxDepth, visited, nodeCount, maxDepthReached)
		} else {
			node.Truncated = true
		}
		out = append(out, node)
	}
	return out
}

// ProofTree returns the transitive support ancestry for a fact (ToK Wave 3).
// Walks SUPPORTS / REQUIRES / REFINES / GENERALIZES / CITES edges outward
// (excludes CONTRADICTS / SUPERSEDES). Each node carries the typed edge by
// which it supports its parent in the tree, so auditors can trace HOW a
// derivation was made — not merely WHICH facts are connected.
func (q *queryServer) ProofTree(ctx context.Context, req *types.QueryProofTreeRequest) (*types.QueryProofTreeResponse, error) {
	if req == nil || req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id is required")
	}

	root, found := q.keeper.GetFact(ctx, req.FactId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}

	maxDepth := req.MaxDepth
	if maxDepth == 0 {
		maxDepth = 5
	}

	// Traversal state: track visited to avoid cycles (DAG is acyclic in
	// principle but the store allows cycles in practice).
	visited := make(map[string]bool)
	nodeCount := uint32(0)
	minConf := root.Confidence
	if root.DependencyConfidenceFloor > 0 && root.DependencyConfidenceFloor < minConf {
		minConf = root.DependencyConfidenceFloor
	}
	maxDepthReached := uint32(0)

	rootNode := q.buildProofNode(ctx, root,
		types.RelationType_RELATION_TYPE_UNSPECIFIED,
		types.InferenceType_INFERENCE_TYPE_UNSPECIFIED,
		0, 0, maxDepth, req.IncludeAxioms, visited,
		&nodeCount, &minConf, &maxDepthReached)

	return &types.QueryProofTreeResponse{
		Root:                    rootNode,
		TotalNodes:              nodeCount,
		MaxDepthReached:         maxDepthReached,
		MinimumConfidenceInTree: minConf,
	}, nil
}

// buildProofNode recursively constructs a ProofTreeNode. Only outgoing
// edges that represent support are followed. CONTRADICTS and SUPERSEDES
// are intentionally excluded — they are disagreement, not derivation.
func (q *queryServer) buildProofNode(
	ctx context.Context,
	fact *types.Fact,
	edgeRelation types.RelationType,
	edgeInference types.InferenceType,
	edgeStrengthBps uint64,
	currentDepth uint32,
	maxDepth uint32,
	includeAxioms bool,
	visited map[string]bool,
	nodeCount *uint32,
	minConf *uint64,
	maxDepthReached *uint32,
) *types.ProofTreeNode {
	*nodeCount++
	if currentDepth > *maxDepthReached {
		*maxDepthReached = currentDepth
	}

	isAxiom := fact.AxiomDistance == 0
	node := &types.ProofTreeNode{
		Fact:                    fact,
		EdgeRelation:            edgeRelation,
		EdgeInference:           edgeInference,
		EdgeInferenceStrengthBps: edgeStrengthBps,
		Depth:                   currentDepth,
		IsAxiom:                 isAxiom,
	}

	if fact.Confidence > 0 && fact.Confidence < *minConf {
		*minConf = fact.Confidence
	}

	// Stop conditions.
	if currentDepth >= maxDepth {
		node.Truncated = true
		return node
	}
	if isAxiom && !includeAxioms {
		return node
	}
	if visited[fact.Id] {
		// Already expanded this fact elsewhere in the tree — return as leaf
		// to avoid exponential duplication.
		return node
	}
	visited[fact.Id] = true

	// Follow outgoing support edges.
	rels, err := q.keeper.GetFactRelations(ctx, fact.Id)
	if err != nil {
		return node
	}
	for _, rel := range rels {
		switch rel.Relation {
		case types.RelationType_RELATION_TYPE_SUPPORTS,
			types.RelationType_RELATION_TYPE_REQUIRES,
			types.RelationType_RELATION_TYPE_REFINES,
			types.RelationType_RELATION_TYPE_GENERALIZES,
			types.RelationType_RELATION_TYPE_CITES:
			// support-bearing edge — follow
		default:
			// CONTRADICTS / SUPERSEDES / UNSPECIFIED — skip
			continue
		}
		target, ok := q.keeper.GetFact(ctx, rel.TargetFactId)
		if !ok {
			continue
		}
		child := q.buildProofNode(ctx, target,
			rel.Relation, rel.Inference, rel.InferenceStrengthBps,
			currentDepth+1, maxDepth, includeAxioms, visited,
			nodeCount, minConf, maxDepthReached)
		node.Supports = append(node.Supports, child)
	}
	return node
}

// ─── Novelty detection queries ────────────────────────────────────────────────

func (q queryServer) CommonKnowledge(ctx context.Context, req *types.QueryCommonKnowledgeRequest) (*types.QueryCommonKnowledgeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	var entries []*types.CommonKnowledgeEntry
	if req.Domain != "" {
		entries = q.keeper.GetCommonKnowledgeByDomain(ctx, req.Domain)
	} else {
		entries = q.keeper.GetAllCommonKnowledge(ctx)
	}

	return &types.QueryCommonKnowledgeResponse{Entries: entries}, nil
}

func (q queryServer) CheckNovelty(ctx context.Context, req *types.QueryCheckNoveltyRequest) (*types.QueryCheckNoveltyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}
	if req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}

	noveltyScore, commonKnowledgeMatch, matchedEntry, overlapCount :=
		q.keeper.CheckNoveltyPreSubmission(ctx, req.Domain, req.Subject, req.Content)

	return &types.QueryCheckNoveltyResponse{
		NoveltyScore:         noveltyScore,
		CommonKnowledgeMatch: commonKnowledgeMatch,
		MatchedEntry:         matchedEntry,
		SubjectOverlapCount:  overlapCount,
	}, nil
}

// ─── Agent demand queries ────────────────────────────────────────────────────

func (q *queryServer) ActiveBounties(ctx context.Context, req *types.QueryActiveBountiesRequest) (*types.QueryActiveBountiesResponse, error) {
	bounties := q.keeper.GetActiveBounties(ctx, req.Domain)
	return &types.QueryActiveBountiesResponse{Bounties: bounties}, nil
}

func (q *queryServer) DemandSignals(ctx context.Context, req *types.QueryDemandSignalsRequest) (*types.QueryDemandSignalsResponse, error) {
	var signals []*types.DemandSignal
	q.keeper.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if req.Domain != "" && signal.Domain != req.Domain {
			return false
		}
		if req.MinUnfulfilled > 0 && signal.UnfulfilledCount < req.MinUnfulfilled {
			return false
		}
		signals = append(signals, signal)
		return false
	})
	return &types.QueryDemandSignalsResponse{Signals: signals}, nil
}

func (q *queryServer) TopDemandGaps(ctx context.Context, req *types.QueryTopDemandGapsRequest) (*types.QueryTopDemandGapsResponse, error) {
	gaps := q.keeper.GetTopDemandGaps(ctx, req.Limit)
	return &types.QueryTopDemandGapsResponse{Gaps: gaps}, nil
}

// ─── Niche competition queries ────────────────────────────────────────────────

func (q *queryServer) NicheInfo(ctx context.Context, req *types.QueryNicheInfoRequest) (*types.QueryNicheInfoResponse, error) {
	if req.NicheKey == "" {
		return nil, status.Error(codes.InvalidArgument, "niche_key is required")
	}

	members := q.keeper.GetNicheMembers(ctx, req.NicheKey)
	if len(members) == 0 {
		return nil, status.Errorf(codes.NotFound, "niche %s not found or empty", req.NicheKey)
	}

	// Sort by fitness desc
	sort.Slice(members, func(i, j int) bool {
		return members[i].FitnessScore > members[j].FitnessScore
	})

	leader := members[0]
	domain := leader.Domain
	subject := ""
	if leader.Structure != nil {
		subject = leader.Structure.Subject
	}

	totalEnergy := uint64(0)
	for _, m := range members {
		totalEnergy += m.Energy
	}

	return &types.QueryNicheInfoResponse{
		NicheKey:    req.NicheKey,
		Domain:      domain,
		Subject:     subject,
		Leader:      leader,
		Members:     members,
		TotalEnergy: totalEnergy,
	}, nil
}

func (q *queryServer) NichesByDomain(ctx context.Context, req *types.QueryNichesByDomainRequest) (*types.QueryNichesByDomainResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	allNiches := q.keeper.GetAllNiches(ctx)
	var result []*types.QueryNicheInfoResponse

	for _, nicheKey := range allNiches {
		members := q.keeper.GetNicheMembers(ctx, nicheKey)
		if len(members) == 0 {
			continue
		}

		// Filter by domain
		if members[0].Domain != req.Domain {
			continue
		}

		// Sort by fitness desc
		sort.Slice(members, func(i, j int) bool {
			return members[i].FitnessScore > members[j].FitnessScore
		})

		leader := members[0]
		subject := ""
		if leader.Structure != nil {
			subject = leader.Structure.Subject
		}

		totalEnergy := uint64(0)
		for _, m := range members {
			totalEnergy += m.Energy
		}

		result = append(result, &types.QueryNicheInfoResponse{
			NicheKey:    nicheKey,
			Domain:      req.Domain,
			Subject:     subject,
			Leader:      leader,
			Members:     members,
			TotalEnergy: totalEnergy,
		})
	}

	return &types.QueryNichesByDomainResponse{Niches: result}, nil
}

// ─── Consensus diversity queries ──────────────────────────────────────────────

func (q *queryServer) DomainDiversity(ctx context.Context, req *types.QueryDomainDiversityRequest) (*types.QueryDomainDiversityResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	epoch := q.keeper.CurrentEpoch(ctx)
	rec, found, err := q.keeper.GetDomainDiversity(ctx, req.Domain, epoch)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		// Try previous epoch
		if epoch > 0 {
			rec, found, err = q.keeper.GetDomainDiversity(ctx, req.Domain, epoch-1)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			if found {
				epoch = epoch - 1
			}
		}
	}
	if !found {
		return &types.QueryDomainDiversityResponse{
			Domain: req.Domain,
			Epoch:  epoch,
		}, nil
	}

	return &types.QueryDomainDiversityResponse{
		Domain:         req.Domain,
		Epoch:          epoch,
		MeanEntropyBps: rec.AvgEntropy,
		RoundCount:     rec.RoundCount,
	}, nil
}

func (q *queryServer) DomainDiversityHistory(ctx context.Context, req *types.QueryDomainDiversityHistoryRequest) (*types.QueryDomainDiversityHistoryResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	epochs := req.Epochs
	if epochs == 0 {
		epochs = 10
	}

	currentEpoch := q.keeper.CurrentEpoch(ctx)
	var history []*types.DomainDiversityEpoch

	for i := uint64(0); i < epochs && i <= currentEpoch; i++ {
		ep := currentEpoch - i
		rec, found, err := q.keeper.GetDomainDiversity(ctx, req.Domain, ep)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if found {
			history = append(history, &types.DomainDiversityEpoch{
				Epoch:          ep,
				MeanEntropyBps: rec.AvgEntropy,
				RoundCount:     rec.RoundCount,
			})
		}
	}

	return &types.QueryDomainDiversityHistoryResponse{
		Domain:  req.Domain,
		History: history,
	}, nil
}

func (q *queryServer) ValidatorIndependence(ctx context.Context, req *types.QueryValidatorIndependenceRequest) (*types.QueryValidatorIndependenceResponse, error) {
	if req.Validator == "" {
		return nil, status.Error(codes.InvalidArgument, "validator is required")
	}

	rec, found, err := q.keeper.GetValidatorIndependence(ctx, req.Validator)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &types.QueryValidatorIndependenceResponse{
			Validator: req.Validator,
		}, nil
	}

	independenceBPS := uint64(0)
	if rec.TotalVotes > 0 {
		independenceBPS = rec.MinorityVotes * 1_000_000 / rec.TotalVotes
	}

	return &types.QueryValidatorIndependenceResponse{
		Validator:       req.Validator,
		TotalVotes:      rec.TotalVotes,
		DissentingVotes: rec.MinorityVotes,
		IndependenceBps: independenceBPS,
	}, nil
}

func (q *queryServer) ConformityAlerts(ctx context.Context, _ *types.QueryConformityAlertsRequest) (*types.QueryConformityAlertsResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	thresholdEpochs := params.DiversityConformityAlertEpochs
	thresholdBPS := params.DiversityConformityAlertThreshold

	var alerts []*types.ConformityAlert

	// Iterate all domains and check their conformity streaks
	q.keeper.IterateDomains(ctx, func(domain *types.Domain) bool {
		streak, found, sErr := q.keeper.GetConformityStreak(ctx, domain.Name)
		if sErr != nil || !found {
			return false
		}
		if streak.ConsecutiveEpochs >= thresholdEpochs {
			alerts = append(alerts, &types.ConformityAlert{
				Domain:            domain.Name,
				ConsecutiveEpochs: streak.ConsecutiveEpochs,
				ThresholdBps:      thresholdBPS,
			})
		}
		return false
	})

	return &types.QueryConformityAlertsResponse{Alerts: alerts}, nil
}

// EpistemicTemperature queries a domain's epistemic temperature state (R29-2).
func (q *queryServer) EpistemicTemperature(ctx context.Context, req *types.QueryEpistemicTemperatureRequest) (*types.QueryEpistemicTemperatureResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	state, err := q.keeper.GetOrInitDomainEpistemicState(ctx, req.Domain)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Calculate effective confidence cap
	effectiveCap := params.MaxConfidence
	if effectiveCap == 0 {
		effectiveCap = 880_000
	}
	if state.Temperature < 300_000 && params.EpistemicColdConfidenceCapBps > 0 {
		if params.EpistemicColdConfidenceCapBps < effectiveCap {
			effectiveCap = params.EpistemicColdConfidenceCapBps
		}
	}
	if state.Temperature > 800_000 && params.SurvivedChallengeConfidenceCap > effectiveCap {
		effectiveCap = params.SurvivedChallengeConfidenceCap
	}

	// Calculate effective growth rate
	growthRate := params.ConfidenceGrowthPerEpochBps
	if state.Temperature > 700_000 && params.EpistemicHotConfidenceGrowthBps > 0 {
		growthRate = safeMulDiv(growthRate, params.EpistemicHotConfidenceGrowthBps, BPS)
	}
	if state.Temperature < 300_000 {
		growthRate = safeMulDiv(growthRate, 500_000, BPS)
	}

	return &types.QueryEpistemicTemperatureResponse{
		Domain:                 req.Domain,
		TemperatureBps:         state.Temperature,
		Category:               TemperatureCategory(state.Temperature),
		ConformityStreak:       state.ConformityStreak,
		RecentVindications:     state.VindicationCount,
		EffectiveConfidenceCap: effectiveCap,
		EffectiveGrowthRate:    growthRate,
	}, nil
}

// RoleElasticity queries domain role elasticity and track record (R29-3).
func (q *queryServer) RoleElasticity(ctx context.Context, req *types.QueryRoleElasticityRequest) (*types.QueryRoleElasticityResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	record, _ := q.keeper.GetDomainRoleRecord(ctx, req.Domain)
	agentBonus, humanBonus := q.keeper.GetRoleElasticity(ctx, req.Domain)
	agentAcc, humanAcc := q.keeper.GetRoleAccuracies(ctx, req.Domain)

	resp := &types.QueryRoleElasticityResponse{
		Domain:           req.Domain,
		AgentBonusBps:    agentBonus,
		HumanBonusBps:    humanBonus,
		AgentAccuracyBps: agentAcc,
		HumanAccuracyBps: humanAcc,
	}

	if record != nil {
		resp.AgentCorrect = record.AgentCorrectCalls
		resp.AgentIncorrect = record.AgentIncorrectCalls
		resp.HumanCorrect = record.HumanCorrectCalls
		resp.HumanIncorrect = record.HumanIncorrectCalls
	}

	return resp, nil
}

// DomainCapacity queries carrying capacity and pressure for a domain (R29-1).
func (q *queryServer) DomainCapacity(ctx context.Context, req *types.QueryDomainCapacityRequest) (*types.QueryDomainCapacityResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}
	stats, _ := q.keeper.GetDomainStats(ctx, req.Domain)
	capacity := q.keeper.GetDomainCarryingCapacity(ctx, req.Domain)
	pressure := q.keeper.GetDomainPressure(ctx, req.Domain)

	return &types.QueryDomainCapacityResponse{
		Domain:      req.Domain,
		ActiveCount: stats.ActiveCount,
		AtRiskCount: stats.AtRiskCount,
		Capacity:    capacity,
		PressureBps: pressure,
		Category:    PressureCategory(pressure),
		TotalEnergy: stats.TotalEnergy,
	}, nil
}
