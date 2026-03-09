package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R54: Strategic Curation ────────────────────────────────────────────────
//
// Agents shift from reactive (filling bounties) to proactive (creating them).
//
// The cycle:
//   1. Compute domain health snapshots
//   2. Identify knowledge gaps (low coverage, low fitness, orphans, stale data)
//   3. Agents register curation strategies with focus domains + priorities
//   4. Auto-create bounties for critical gaps
//   5. Agents fill gaps → domain health improves → models improve
//   6. Strategy effectiveness tracked: did this gap identification lead to improvement?
//   7. Better models detect subtler gaps → GOTO 1
//
// Agents that identify real gaps earn a discovery reward.
// Strategies with good track records get more weight.

// ─── ComputeDomainHealth ────────────────────────────────────────────────────

// ComputeDomainHealth creates a health snapshot for a domain.
// Examines TDU count, fitness distribution, graph connectivity,
// freshness, and open gaps.
func (k Keeper) ComputeDomainHealth(ctx context.Context, domain string) (*types.DomainHealth, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlock := uint64(sdkCtx.BlockHeight())

	health := &types.DomainHealth{
		Domain:     domain,
		ComputedAt: currentBlock,
	}

	// Collect TDU fitness scores.
	var fitnessScores []sdkmath.LegacyDec
	var totalAge uint64
	var newestBlock, oldestBlock uint64

	k.IterateSamples(ctx, func(sample *types.Sample) bool {
		if sample.Domain != domain || !isActiveSample(sample.Status) {
			return false
		}
		health.TotalTDUs++
		health.ActiveTDUs++

		// Fitness.
		fitnessRec, ok := k.GetFitnessRecord(ctx, sample.Id)
		if ok {
			fitnessScores = append(fitnessScores, fitnessRec.GetFitnessScore())
		}

		// Age tracking.
		sampleBlock := sample.VerifiedAtBlock
		if newestBlock == 0 || sampleBlock > newestBlock {
			newestBlock = sampleBlock
		}
		if oldestBlock == 0 || sampleBlock < oldestBlock {
			oldestBlock = sampleBlock
		}
		if currentBlock > sampleBlock {
			totalAge += currentBlock - sampleBlock
		}

		return false
	})

	health.NewestTDUBlock = newestBlock
	health.OldestTDUBlock = oldestBlock
	if health.ActiveTDUs > 0 {
		health.AvgAge = totalAge / health.ActiveTDUs
	}

	// Fitness statistics.
	if len(fitnessScores) > 0 {
		sort.Slice(fitnessScores, func(i, j int) bool {
			return fitnessScores[i].LT(fitnessScores[j])
		})

		total := sdkmath.LegacyZeroDec()
		for _, f := range fitnessScores {
			total = total.Add(f)
		}
		health.AvgFitness = total.Quo(sdkmath.LegacyNewDec(int64(len(fitnessScores)))).String()
		health.MinFitness = fitnessScores[0].String()
		health.MaxFitness = fitnessScores[len(fitnessScores)-1].String()

		// Median.
		mid := len(fitnessScores) / 2
		health.MedianFitness = fitnessScores[mid].String()
	}

	// Graph connectivity.
	var totalEdges uint64
	var orphanCount uint64
	k.IterateSamples(ctx, func(sample *types.Sample) bool {
		if sample.Domain != domain || !isActiveSample(sample.Status) {
			return false
		}
		outEdges := k.GetOutgoingEdges(ctx, sample.Id)
		inEdges := k.GetIncomingEdges(ctx, sample.Id)
		edgeCount := uint64(len(outEdges) + len(inEdges))
		totalEdges += uint64(len(outEdges)) // count each edge once (outgoing)

		if edgeCount == 0 {
			orphanCount++
		}

		// Count contradictions.
		for _, e := range outEdges {
			if e.EdgeType == types.EdgeTypeContradicts {
				health.ContradictionCount++
			}
		}

		return false
	})
	health.TotalEdges = totalEdges
	health.OrphanCount = orphanCount
	if health.ActiveTDUs > 0 {
		avgConn := sdkmath.LegacyNewDec(int64(totalEdges)).Quo(sdkmath.LegacyNewDec(int64(health.ActiveTDUs)))
		health.AvgConnectivity = avgConn.String()
	}

	// Count open gaps.
	gaps := k.GetOpenGapsByDomain(ctx, domain)
	health.OpenGaps = uint64(len(gaps))
	for _, gap := range gaps {
		if gap.GetSeverity().GT(sdkmath.LegacyNewDecWithPrec(8, 1)) {
			health.CriticalGaps++
		}
	}

	// Compute composite health score.
	health.HealthScore = k.computeHealthScore(ctx, health).String()

	// Store.
	if err := k.setDomainHealth(ctx, health); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventDomainHealthComputed,
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute(types.AttributeHealthScore, health.HealthScore),
		sdk.NewAttribute("total_tdus", strconv.FormatUint(health.TotalTDUs, 10)),
		sdk.NewAttribute("orphans", strconv.FormatUint(health.OrphanCount, 10)),
	))

	return health, nil
}

// computeHealthScore produces a composite [0, 1] score from domain health factors.
func (k Keeper) computeHealthScore(_ context.Context, health *types.DomainHealth) sdkmath.LegacyDec {
	params := types.DefaultCurationStrategyParams()
	score := sdkmath.LegacyZeroDec()
	factors := 0

	// Coverage factor: min(totalTDUs / minForHealthy, 1.0).
	if params.MinTDUsForHealthy > 0 {
		coverageRatio := sdkmath.LegacyNewDec(int64(health.ActiveTDUs)).Quo(
			sdkmath.LegacyNewDec(int64(params.MinTDUsForHealthy)))
		if coverageRatio.GT(sdkmath.LegacyOneDec()) {
			coverageRatio = sdkmath.LegacyOneDec()
		}
		score = score.Add(coverageRatio)
		factors++
	}

	// Fitness factor: avg fitness (already [0, 1]).
	if health.AvgFitness != "" {
		avg, err := sdkmath.LegacyNewDecFromStr(health.AvgFitness)
		if err == nil {
			score = score.Add(avg)
			factors++
		}
	}

	// Connectivity factor: 1 - (orphanRatio).
	if health.ActiveTDUs > 0 {
		orphanRatio := sdkmath.LegacyNewDec(int64(health.OrphanCount)).Quo(
			sdkmath.LegacyNewDec(int64(health.ActiveTDUs)))
		connScore := sdkmath.LegacyOneDec().Sub(orphanRatio)
		if connScore.IsNegative() {
			connScore = sdkmath.LegacyZeroDec()
		}
		score = score.Add(connScore)
		factors++
	}

	// Contradiction factor: 1 - min(contradictions / totalTDUs, 1.0).
	if health.ActiveTDUs > 0 {
		contradRatio := sdkmath.LegacyNewDec(int64(health.ContradictionCount)).Quo(
			sdkmath.LegacyNewDec(int64(health.ActiveTDUs)))
		if contradRatio.GT(sdkmath.LegacyOneDec()) {
			contradRatio = sdkmath.LegacyOneDec()
		}
		score = score.Add(sdkmath.LegacyOneDec().Sub(contradRatio))
		factors++
	}

	if factors == 0 {
		return sdkmath.LegacyZeroDec()
	}
	return score.Quo(sdkmath.LegacyNewDec(int64(factors)))
}

// ─── IdentifyKnowledgeGaps ──────────────────────────────────────────────────

// IdentifyKnowledgeGaps analyzes a domain and returns detected gaps.
// Can be called by an agent (earning discovery reward) or by protocol.
func (k Keeper) IdentifyKnowledgeGaps(ctx context.Context, domain, detectedBy string) ([]types.KnowledgeGap, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetCurationStrategyParams(ctx)
	currentBlock := uint64(sdkCtx.BlockHeight())

	health, err := k.ComputeDomainHealth(ctx, domain)
	if err != nil {
		return nil, err
	}

	var gaps []types.KnowledgeGap

	// 1. Coverage gap: too few TDUs.
	if health.ActiveTDUs < params.MinTDUsForHealthy {
		severity := sdkmath.LegacyOneDec().Sub(
			sdkmath.LegacyNewDec(int64(health.ActiveTDUs)).Quo(
				sdkmath.LegacyNewDec(int64(params.MinTDUsForHealthy))))
		if severity.IsNegative() {
			severity = sdkmath.LegacyZeroDec()
		}
		gaps = append(gaps, types.KnowledgeGap{
			GapID:       k.nextGapID(ctx),
			Domain:      domain,
			GapType:     types.GapTypeCoverage,
			Description: fmt.Sprintf("domain %s has %d TDUs, minimum %d needed", domain, health.ActiveTDUs, params.MinTDUsForHealthy),
			Severity:    severity.String(),
			Coverage:    health.AvgFitness,
			DetectedBy:  detectedBy,
			DetectedAt:  currentBlock,
			Status:      "open",
		})
	}

	// 2. Fitness gap: average fitness too low.
	minFitness, _ := sdkmath.LegacyNewDecFromStr(params.MinFitnessForHealthy)
	if health.AvgFitness != "" {
		avgFit, _ := sdkmath.LegacyNewDecFromStr(health.AvgFitness)
		if avgFit.LT(minFitness) {
			severity := minFitness.Sub(avgFit).Quo(minFitness)
			gaps = append(gaps, types.KnowledgeGap{
				GapID:       k.nextGapID(ctx),
				Domain:      domain,
				GapType:     types.GapTypeFitness,
				Description: fmt.Sprintf("domain %s avg fitness %s below threshold %s", domain, health.AvgFitness, params.MinFitnessForHealthy),
				Severity:    severity.String(),
				AvgFitness:  health.AvgFitness,
				DetectedBy:  detectedBy,
				DetectedAt:  currentBlock,
				Status:      "open",
			})
		}
	}

	// 3. Connectivity gap: too many orphans.
	if health.ActiveTDUs > 0 {
		orphanRatio := sdkmath.LegacyNewDec(int64(health.OrphanCount)).Quo(
			sdkmath.LegacyNewDec(int64(health.ActiveTDUs)))
		threshold := sdkmath.LegacyNewDecWithPrec(3, 1) // 30% orphans = problem
		if orphanRatio.GT(threshold) {
			severity := orphanRatio // higher orphan ratio = higher severity
			if severity.GT(sdkmath.LegacyOneDec()) {
				severity = sdkmath.LegacyOneDec()
			}
			gaps = append(gaps, types.KnowledgeGap{
				GapID:       k.nextGapID(ctx),
				Domain:      domain,
				GapType:     types.GapTypeConnectivity,
				Description: fmt.Sprintf("domain %s has %d orphan TDUs (%s%% disconnected)", domain, health.OrphanCount, orphanRatio.Mul(sdkmath.LegacyNewDec(100)).TruncateInt()),
				Severity:    severity.String(),
				DetectedBy:  detectedBy,
				DetectedAt:  currentBlock,
				Status:      "open",
			})
		}
	}

	// 4. Contradiction gap: unresolved contradictions.
	if health.ContradictionCount > 0 {
		severity := sdkmath.LegacyNewDecWithPrec(7, 1) // contradictions are always important
		if health.ContradictionCount > 5 {
			severity = sdkmath.LegacyNewDecWithPrec(9, 1)
		}
		gaps = append(gaps, types.KnowledgeGap{
			GapID:       k.nextGapID(ctx),
			Domain:      domain,
			GapType:     types.GapTypeContradiction,
			Description: fmt.Sprintf("domain %s has %d unresolved contradictions", domain, health.ContradictionCount),
			Severity:    severity.String(),
			DetectedBy:  detectedBy,
			DetectedAt:  currentBlock,
			Status:      "open",
		})
	}

	// 5. Staleness gap: no recent data.
	if health.NewestTDUBlock > 0 {
		blocksSinceNewest := currentBlock - health.NewestTDUBlock
		if blocksSinceNewest > params.StalenessThreshold {
			severity := sdkmath.LegacyNewDecWithPrec(6, 1) // stale but not critical
			gaps = append(gaps, types.KnowledgeGap{
				GapID:       k.nextGapID(ctx),
				Domain:      domain,
				GapType:     types.GapTypeStale,
				Description: fmt.Sprintf("domain %s has no new data for %d blocks", domain, blocksSinceNewest),
				Severity:    severity.String(),
				DetectedBy:  detectedBy,
				DetectedAt:  currentBlock,
				Status:      "open",
			})
		}
	}

	// Store all detected gaps.
	for i := range gaps {
		if err := k.setKnowledgeGap(ctx, &gaps[i]); err != nil {
			continue
		}
	}

	return gaps, nil
}

// ─── RegisterCurationStrategy ───────────────────────────────────────────────

// RegisterCurationStrategy creates or updates an agent's curation approach.
func (k Keeper) RegisterCurationStrategy(ctx context.Context, agentID string, focusDomains []string, priorities []types.GapType) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate agent exists and is active.
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return "", types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	if agent.Status != types.AgentStatusActive {
		return "", types.ErrAgentNotActive.Wrapf("agent %s status: %s", agentID, agent.Status)
	}

	// Validate priorities.
	for _, p := range priorities {
		if !types.ValidGapTypes[p] {
			return "", fmt.Errorf("invalid gap type priority: %s", p)
		}
	}

	strategyID := k.nextStrategyID(ctx)

	strategy := &types.CurationStrategy{
		StrategyID:   strategyID,
		AgentID:      agentID,
		FocusDomains: focusDomains,
		Priorities:   priorities,
		Effectiveness: "0.000000000000000000",
		CreatedAt:    uint64(sdkCtx.BlockHeight()),
		UpdatedAt:    uint64(sdkCtx.BlockHeight()),
	}

	if err := k.setCurationStrategy(ctx, strategy); err != nil {
		return "", err
	}

	return strategyID, nil
}

// ─── FillGap ────────────────────────────────────────────────────────────────

// FillGap marks a knowledge gap as filled when new data addresses it.
// Called when a submission or bounty entry covers the gap's domain/type.
func (k Keeper) FillGap(ctx context.Context, gapID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	gap, found := k.GetKnowledgeGap(ctx, gapID)
	if !found {
		return fmt.Errorf("gap %s not found", gapID)
	}
	if gap.Status != "open" && gap.Status != "filling" {
		return nil // already resolved
	}

	gap.Status = "filled"
	gap.FilledAt = uint64(sdkCtx.BlockHeight())

	if err := k.setKnowledgeGap(ctx, gap); err != nil {
		return err
	}

	// Remove from open index.
	_ = kvStore.Delete(types.KnowledgeGapOpenKey(gapID))

	// Reward the agent that identified this gap.
	if gap.DetectedBy != "" && gap.DetectedBy != "protocol" {
		k.rewardGapDiscovery(ctx, gap.DetectedBy, gapID)
	}

	// Update strategy effectiveness for the discovering agent.
	if gap.DetectedBy != "" {
		k.updateStrategyAfterFill(ctx, gap.DetectedBy)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventGapFilled,
		sdk.NewAttribute(types.AttributeGapID, gapID),
		sdk.NewAttribute("domain", gap.Domain),
		sdk.NewAttribute("detected_by", gap.DetectedBy),
	))

	return nil
}

// ─── Auto-Create Bounties from Gaps ─────────────────────────────────────────

// CreateBountiesFromGaps generates bounties for critical knowledge gaps.
// Called from BeginBlocker periodically.
func (k Keeper) CreateBountiesFromGaps(ctx context.Context) (created uint64) {
	params := k.GetCurationStrategyParams(ctx)
	if !params.AutoBountyEnabled {
		return 0
	}

	minSeverity, _ := sdkmath.LegacyNewDecFromStr(params.AutoBountySeverityMin)

	openGaps := k.GetAllOpenGaps(ctx)
	for _, gap := range openGaps {
		if gap.AutoBountyCreated {
			continue // already has a bounty
		}

		severity := gap.GetSeverity()
		if severity.LT(minSeverity) {
			continue // not critical enough
		}

		// Create a bounty for this gap.
		bountySubject := fmt.Sprintf("Fill %s gap in %s", gap.GapType, gap.Domain)
		bounty, err := k.FundBounty(ctx, &types.MsgFundBounty{
			Funder: k.GetAuthority(),
			Domain: gap.Domain,
			Topic:  bountySubject,
			Amount: params.AutoBountyReward,
		})
		if err != nil || bounty == nil {
			continue
		}

		// Link gap to bounty.
		gap.AutoBountyCreated = true
		gap.BountyID = bounty.BountyId
		gap.SuggestedBountyReward = params.AutoBountyReward
		_ = k.setKnowledgeGap(ctx, gap)

		created++

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventStrategicBounty,
			sdk.NewAttribute(types.AttributeGapID, gap.GapID),
			sdk.NewAttribute("bounty_id", bounty.BountyId),
			sdk.NewAttribute(types.AttributeSeverity, severity.String()),
		))
	}

	return created
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetKnowledgeGap retrieves a gap by ID.
func (k Keeper) GetKnowledgeGap(ctx context.Context, gapID string) (*types.KnowledgeGap, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KnowledgeGapKey(gapID))
	if err != nil || bz == nil {
		return nil, false
	}
	var gap types.KnowledgeGap
	if err := gap.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &gap, true
}

// GetOpenGapsByDomain returns all open gaps in a domain.
func (k Keeper) GetOpenGapsByDomain(ctx context.Context, domain string) []*types.KnowledgeGap {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.KnowledgeGapByDomainPrefix(domain)

	var gaps []*types.KnowledgeGap
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		gapID := string(iter.Key()[len(prefix):])
		gap, found := k.GetKnowledgeGap(ctx, gapID)
		if found && (gap.Status == "open" || gap.Status == "filling") {
			gaps = append(gaps, gap)
		}
	}
	return gaps
}

// GetAllOpenGaps returns all open gaps across all domains.
func (k Keeper) GetAllOpenGaps(ctx context.Context) []*types.KnowledgeGap {
	kvStore := k.storeService.OpenKVStore(ctx)

	var gaps []*types.KnowledgeGap
	iter, err := kvStore.Iterator(types.KnowledgeGapOpenPfx, prefixEndBytes(types.KnowledgeGapOpenPfx))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		gapID := string(iter.Key()[len(types.KnowledgeGapOpenPfx):])
		gap, found := k.GetKnowledgeGap(ctx, gapID)
		if found {
			gaps = append(gaps, gap)
		}
	}
	return gaps
}

// GetDomainHealth retrieves the latest health snapshot for a domain.
func (k Keeper) GetDomainHealth(ctx context.Context, domain string) (*types.DomainHealth, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.DomainHealthKey(domain))
	if err != nil || bz == nil {
		return nil, false
	}
	var health types.DomainHealth
	if err := health.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &health, true
}

// GetCurationStrategy retrieves a strategy by ID.
func (k Keeper) GetCurationStrategy(ctx context.Context, strategyID string) (*types.CurationStrategy, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurationStrategyKey(strategyID))
	if err != nil || bz == nil {
		return nil, false
	}
	var strategy types.CurationStrategy
	if err := strategy.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &strategy, true
}

// GetStrategiesByAgent returns all strategies for an agent.
func (k Keeper) GetStrategiesByAgent(ctx context.Context, agentID string) []*types.CurationStrategy {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.CurationStrategyByAgentPrefix(agentID)

	var strategies []*types.CurationStrategy
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		stratID := string(iter.Key()[len(prefix):])
		strategy, found := k.GetCurationStrategy(ctx, stratID)
		if found {
			strategies = append(strategies, strategy)
		}
	}
	return strategies
}

// ─── Params ─────────────────────────────────────────────────────────────────

// GetCurationStrategyParams retrieves curation strategy parameters.
func (k Keeper) GetCurationStrategyParams(ctx context.Context) types.CurationStrategyParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurationStrategyParamsKey)
	if err != nil || bz == nil {
		return types.DefaultCurationStrategyParams()
	}
	var params types.CurationStrategyParams
	if err := params.Unmarshal(bz); err != nil {
		return types.DefaultCurationStrategyParams()
	}
	return params
}

// SetCurationStrategyParams stores curation strategy parameters.
func (k Keeper) SetCurationStrategyParams(ctx context.Context, params types.CurationStrategyParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := params.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.CurationStrategyParamsKey, bz)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setKnowledgeGap(ctx context.Context, gap *types.KnowledgeGap) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := gap.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal knowledge gap: %w", err)
	}
	if err := kvStore.Set(types.KnowledgeGapKey(gap.GapID), bz); err != nil {
		return err
	}
	// Domain index.
	if gap.Domain != "" {
		_ = kvStore.Set(types.KnowledgeGapByDomainKey(gap.Domain, gap.GapID), []byte{0x01})
	}
	// Open index (if still open).
	if gap.Status == "open" || gap.Status == "filling" {
		_ = kvStore.Set(types.KnowledgeGapOpenKey(gap.GapID), []byte{0x01})
	}
	return nil
}

func (k Keeper) setCurationStrategy(ctx context.Context, strategy *types.CurationStrategy) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := strategy.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal curation strategy: %w", err)
	}
	if err := kvStore.Set(types.CurationStrategyKey(strategy.StrategyID), bz); err != nil {
		return err
	}
	// Agent index.
	_ = kvStore.Set(types.CurationStrategyByAgentKey(strategy.AgentID, strategy.StrategyID), []byte{0x01})
	return nil
}

func (k Keeper) setDomainHealth(ctx context.Context, health *types.DomainHealth) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := health.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal domain health: %w", err)
	}
	return kvStore.Set(types.DomainHealthKey(health.Domain), bz)
}

func (k Keeper) nextGapID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KnowledgeGapSeqKey)
	var seq uint64
	if err == nil && len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.KnowledgeGapSeqKey, newBz)

	hash := sha256.Sum256([]byte(fmt.Sprintf("gap:%d", seq)))
	return fmt.Sprintf("gap-%x", hash[:8])
}

func (k Keeper) nextStrategyID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurationStrategySeqKey)
	var seq uint64
	if err == nil && len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.CurationStrategySeqKey, newBz)

	hash := sha256.Sum256([]byte(fmt.Sprintf("strategy:%d", seq)))
	return fmt.Sprintf("strat-%x", hash[:8])
}

// rewardGapDiscovery rewards an agent for identifying a real gap (one that got filled).
func (k Keeper) rewardGapDiscovery(ctx context.Context, agentID, gapID string) {
	params := k.GetCurationStrategyParams(ctx)
	reward, ok := sdkmath.NewIntFromString(params.GapIdentificationReward)
	if !ok || !reward.IsPositive() {
		return
	}

	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return
	}

	agentAddr, err := sdk.AccAddressFromBech32(agent.Address)
	if err != nil {
		return
	}

	coins := sdk.NewCoins(sdk.NewCoin("uzrn", reward))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, agentAddr, coins); err != nil {
		return
	}

	_ = k.AddAgentEarnings(ctx, agentID, reward)
	_ = k.AutoReplenishCredits(ctx, agentID, reward) // R51 integration
}

// updateStrategyAfterFill updates an agent's strategy stats after a gap was filled.
func (k Keeper) updateStrategyAfterFill(ctx context.Context, agentID string) {
	strategies := k.GetStrategiesByAgent(ctx, agentID)
	for _, strategy := range strategies {
		strategy.GapsFilled++

		// Recompute effectiveness.
		if strategy.GapsIdentified > 0 {
			effectiveness := sdkmath.LegacyNewDec(int64(strategy.GapsFilled)).Quo(
				sdkmath.LegacyNewDec(int64(strategy.GapsIdentified)))
			if effectiveness.GT(sdkmath.LegacyOneDec()) {
				effectiveness = sdkmath.LegacyOneDec()
			}
			strategy.Effectiveness = effectiveness.String()
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		strategy.UpdatedAt = uint64(sdkCtx.BlockHeight())
		_ = k.setCurationStrategy(ctx, strategy)
	}
}
