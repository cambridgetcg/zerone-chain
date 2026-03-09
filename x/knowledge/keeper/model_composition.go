package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R56: Model Composition — Ensemble Registry ─────────────────────────────
//
// Specialized models combine into ensembles. The ensemble routes queries
// to the best component model per domain. Ensemble output becomes training
// signal for distillation — extracting collective knowledge into a single
// more capable model.
//
// On-chain: routing decisions, ensemble registry, distillation jobs.
// Off-chain: actual model inference (in TEE).
//
// Integration:
//   - R51: agents can pay for ensemble API access (routed to components)
//   - R52: attribution flows to component model curators
//   - R55: swarms can create ensembles from their collective models

// ─── CreateEnsemble ─────────────────────────────────────────────────────────

// CreateEnsemble registers a new model ensemble.
func (k Keeper) CreateEnsemble(ctx context.Context, name, creator string, routingType types.RoutingType, componentModelIDs []string) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetCompositionParams(ctx)

	// Validate routing type.
	if !types.ValidRoutingTypes[routingType] {
		return "", fmt.Errorf("invalid routing type: %s", routingType)
	}

	// Validate component count.
	if uint64(len(componentModelIDs)) < params.MinComponentsForEnsemble {
		return "", fmt.Errorf("need at least %d components, got %d", params.MinComponentsForEnsemble, len(componentModelIDs))
	}
	if uint64(len(componentModelIDs)) > params.MaxComponentsPerEnsemble {
		return "", fmt.Errorf("max %d components, got %d", params.MaxComponentsPerEnsemble, len(componentModelIDs))
	}

	// Validate each component model exists and meets benchmark.
	minBenchmark, _ := sdkmath.LegacyNewDecFromStr(params.MinBenchmarkForComponent)
	var components []types.EnsembleComponent
	var domains []string
	domainSet := make(map[string]bool)

	for _, modelID := range componentModelIDs {
		model, found := k.GetModelRecord(ctx, modelID)
		if !found {
			return "", types.ErrModelNotFound.Wrapf("component model %s", modelID)
		}
		if model.Status != types.ModelStatusActive {
			return "", types.ErrModelNotActive.Wrapf("model %s status: %s", modelID, model.Status)
		}
		if model.GetBenchmarkScore().LT(minBenchmark) {
			return "", fmt.Errorf("model %s benchmark %s below minimum %s", modelID, model.BenchmarkScore, params.MinBenchmarkForComponent)
		}

		// Check for backing agent.
		agentID := ""
		agent, agentFound := k.GetAgentByModel(ctx, modelID)
		if agentFound {
			agentID = agent.AgentID
		}

		components = append(components, types.EnsembleComponent{
			ModelID: modelID,
			Domain:  model.Domain,
			Weight:  "1.000000000000000000", // equal weight initially
			AgentID: agentID,
		})

		if !domainSet[model.Domain] {
			domainSet[model.Domain] = true
			domains = append(domains, model.Domain)
		}
	}

	// Generate ensemble ID.
	ensembleID := k.nextEnsembleID(ctx)

	// Compute initial composite benchmark (weighted average).
	compositeBenchmark := k.computeCompositeBenchmark(components)

	ensemble := &types.ModelEnsemble{
		EnsembleID:     ensembleID,
		Name:           name,
		Components:     components,
		RoutingType:    routingType,
		BenchmarkScore: compositeBenchmark.String(),
		Domains:        domains,
		CreatedAt:      uint64(sdkCtx.BlockHeight()),
		Creator:        creator,
		Status:         "draft",
		AvgResponseCost: "0",
	}

	if err := k.setModelEnsemble(ctx, ensemble); err != nil {
		return "", err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEnsembleCreated,
		sdk.NewAttribute(types.AttributeEnsembleID, ensembleID),
		sdk.NewAttribute(types.AttributeRoutingType, string(routingType)),
		sdk.NewAttribute(types.AttributeComponentCount, strconv.Itoa(len(components))),
	))

	return ensembleID, nil
}

// ─── ActivateEnsemble ───────────────────────────────────────────────────────

// ActivateEnsemble transitions a draft ensemble to active.
func (k Keeper) ActivateEnsemble(ctx context.Context, ensembleID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	ensemble, found := k.GetModelEnsemble(ctx, ensembleID)
	if !found {
		return fmt.Errorf("ensemble %s not found", ensembleID)
	}
	if ensemble.Status != "draft" {
		return fmt.Errorf("can only activate draft ensembles, current: %s", ensemble.Status)
	}

	ensemble.Status = "active"
	if err := k.setModelEnsemble(ctx, ensemble); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEnsembleActivated,
		sdk.NewAttribute(types.AttributeEnsembleID, ensembleID),
	))

	return nil
}

// ─── RouteQuery ─────────────────────────────────────────────────────────────

// RouteQuery selects which component model should handle a query.
// The routing decision is stored on-chain for auditability and
// becomes training signal for future distillation.
func (k Keeper) RouteQuery(ctx context.Context, ensembleID, queryDomain string) (*types.RoutingDecision, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	ensemble, found := k.GetModelEnsemble(ctx, ensembleID)
	if !found {
		return nil, fmt.Errorf("ensemble %s not found", ensembleID)
	}
	if ensemble.Status != "active" {
		return nil, fmt.Errorf("ensemble %s not active", ensembleID)
	}

	var selectedModel string
	var confidence sdkmath.LegacyDec

	switch ensemble.RoutingType {
	case types.RoutingDomain:
		selectedModel, confidence = k.routeByDomain(ensemble, queryDomain)
	case types.RoutingConfidence:
		selectedModel, confidence = k.routeByBenchmark(ensemble)
	case types.RoutingVoting:
		// For voting, all components participate — select the primary.
		selectedModel, confidence = k.routeByBenchmark(ensemble)
	case types.RoutingCascade:
		selectedModel, confidence = k.routeByBenchmark(ensemble)
	default:
		selectedModel, confidence = k.routeByBenchmark(ensemble)
	}

	if selectedModel == "" {
		return nil, fmt.Errorf("no suitable component for domain %s", queryDomain)
	}

	// Record routing decision.
	decisionID := k.nextRoutingDecisionID(ctx)
	decision := &types.RoutingDecision{
		DecisionID:    decisionID,
		EnsembleID:    ensembleID,
		SelectedModel: selectedModel,
		Domain:        queryDomain,
		Confidence:    confidence.String(),
		BlockHeight:   uint64(sdkCtx.BlockHeight()),
	}

	if err := k.setRoutingDecision(ctx, decision); err != nil {
		return nil, err
	}

	// Update ensemble stats.
	ensemble.TotalQueries++
	ensemble.TotalRoutings++
	_ = k.setModelEnsemble(ctx, ensemble)

	// Update component stats.
	for i := range ensemble.Components {
		if ensemble.Components[i].ModelID == selectedModel {
			ensemble.Components[i].QueriesHandled++
			break
		}
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventRoutingDecision,
		sdk.NewAttribute(types.AttributeEnsembleID, ensembleID),
		sdk.NewAttribute(types.AttributeSelectedModel, selectedModel),
		sdk.NewAttribute("domain", queryDomain),
	))

	return decision, nil
}

// ─── InitiateDistillation ───────────────────────────────────────────────────

// InitiateDistillation begins the process of extracting an ensemble's
// collective knowledge into a single model.
func (k Keeper) InitiateDistillation(ctx context.Context, ensembleID string) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetCompositionParams(ctx)

	ensemble, found := k.GetModelEnsemble(ctx, ensembleID)
	if !found {
		return "", fmt.Errorf("ensemble %s not found", ensembleID)
	}
	if ensemble.Status != "active" {
		return "", fmt.Errorf("can only distill active ensembles")
	}
	if ensemble.TotalQueries < params.DistillationMinCaptures {
		return "", fmt.Errorf("need %d queries for distillation, have %d", params.DistillationMinCaptures, ensemble.TotalQueries)
	}

	jobID := k.nextDistillationID(ctx)

	job := &types.DistillationJob{
		JobID:          jobID,
		EnsembleID:     ensembleID,
		CaptureCount:   ensemble.TotalQueries,
		DomainsCovered: ensemble.Domains,
		Status:         "pending",
		StartAt:        uint64(sdkCtx.BlockHeight()),
		MinBenchmark:   params.DistillationMinBenchmark,
	}

	if err := k.setDistillationJob(ctx, job); err != nil {
		return "", err
	}

	// Mark ensemble as distilling.
	ensemble.Status = "distilling"
	ensemble.DistillationJob = jobID
	_ = k.setModelEnsemble(ctx, ensemble)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventDistillationStarted,
		sdk.NewAttribute(types.AttributeEnsembleID, ensembleID),
		sdk.NewAttribute(types.AttributeDistillJobID, jobID),
	))

	return jobID, nil
}

// ─── CompleteDistillation ───────────────────────────────────────────────────

// CompleteDistillation finalizes a distillation job with the output model.
func (k Keeper) CompleteDistillation(ctx context.Context, jobID, outputModelID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	job, found := k.GetDistillationJob(ctx, jobID)
	if !found {
		return fmt.Errorf("distillation job %s not found", jobID)
	}
	if job.Status != "pending" && job.Status != "training" {
		return fmt.Errorf("job %s already resolved (status: %s)", jobID, job.Status)
	}

	// Verify the output model exists and meets quality gate.
	model, modelFound := k.GetModelRecord(ctx, outputModelID)
	if !modelFound {
		return types.ErrModelNotFound.Wrapf("output model %s", outputModelID)
	}

	minBench, _ := sdkmath.LegacyNewDecFromStr(job.MinBenchmark)
	if model.GetBenchmarkScore().LT(minBench) {
		// Quality gate failed — distillation produced inferior model.
		job.Status = "failed"
		job.EndAt = uint64(sdkCtx.BlockHeight())
		_ = k.setDistillationJob(ctx, job)

		// Revert ensemble to active.
		ensemble, found := k.GetModelEnsemble(ctx, job.EnsembleID)
		if found {
			ensemble.Status = "active"
			_ = k.setModelEnsemble(ctx, ensemble)
		}
		return fmt.Errorf("distilled model benchmark %s below minimum %s", model.BenchmarkScore, job.MinBenchmark)
	}

	// Success.
	job.Status = "complete"
	job.OutputModelID = outputModelID
	job.EndAt = uint64(sdkCtx.BlockHeight())
	_ = k.setDistillationJob(ctx, job)

	// Link ensemble to distilled model.
	ensemble, found := k.GetModelEnsemble(ctx, job.EnsembleID)
	if found {
		ensemble.DistilledModelID = outputModelID
		ensemble.Status = "retired" // ensemble succeeded in producing a generalist
		_ = k.setModelEnsemble(ctx, ensemble)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventDistillationCompleted,
		sdk.NewAttribute(types.AttributeDistillJobID, jobID),
		sdk.NewAttribute(types.AttributeEnsembleID, job.EnsembleID),
		sdk.NewAttribute(types.AttributeModelID, outputModelID),
	))

	return nil
}

// ─── Queries ────────────────────────────────────────────────────────────────

func (k Keeper) GetModelEnsemble(ctx context.Context, ensembleID string) (*types.ModelEnsemble, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ModelEnsembleKey(ensembleID))
	if err != nil || bz == nil {
		return nil, false
	}
	var ensemble types.ModelEnsemble
	if err := ensemble.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &ensemble, true
}

func (k Keeper) GetEnsemblesByDomain(ctx context.Context, domain string) []*types.ModelEnsemble {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.EnsembleByDomainPfx(domain)
	var ensembles []*types.ModelEnsemble
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		eID := string(iter.Key()[len(prefix):])
		e, found := k.GetModelEnsemble(ctx, eID)
		if found {
			ensembles = append(ensembles, e)
		}
	}
	return ensembles
}

func (k Keeper) GetDistillationJob(ctx context.Context, jobID string) (*types.DistillationJob, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.DistillationJobKey(jobID))
	if err != nil || bz == nil {
		return nil, false
	}
	var job types.DistillationJob
	if err := job.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &job, true
}

func (k Keeper) GetRoutingDecision(ctx context.Context, decisionID string) (*types.RoutingDecision, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.RoutingDecisionKey(decisionID))
	if err != nil || bz == nil {
		return nil, false
	}
	var decision types.RoutingDecision
	if err := decision.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &decision, true
}

// ─── Params ─────────────────────────────────────────────────────────────────

func (k Keeper) GetCompositionParams(ctx context.Context) types.CompositionParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CompositionParamsKey)
	if err != nil || bz == nil {
		return types.DefaultCompositionParams()
	}
	var params types.CompositionParams
	if err := params.Unmarshal(bz); err != nil {
		return types.DefaultCompositionParams()
	}
	return params
}

func (k Keeper) SetCompositionParams(ctx context.Context, params types.CompositionParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := params.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.CompositionParamsKey, bz)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) routeByDomain(ensemble *types.ModelEnsemble, domain string) (string, sdkmath.LegacyDec) {
	for _, comp := range ensemble.Components {
		if comp.Domain == domain {
			return comp.ModelID, sdkmath.LegacyOneDec()
		}
	}
	// Fallback to highest weight.
	return k.routeByBenchmark(ensemble)
}

func (k Keeper) routeByBenchmark(ensemble *types.ModelEnsemble) (string, sdkmath.LegacyDec) {
	bestModel := ""
	bestWeight := sdkmath.LegacyZeroDec()
	for _, comp := range ensemble.Components {
		w := comp.GetWeight()
		if w.GT(bestWeight) {
			bestWeight = w
			bestModel = comp.ModelID
		}
	}
	return bestModel, bestWeight
}

func (k Keeper) computeCompositeBenchmark(components []types.EnsembleComponent) sdkmath.LegacyDec {
	if len(components) == 0 {
		return sdkmath.LegacyZeroDec()
	}
	// Simple average of component weights (proxy for benchmark until measured).
	total := sdkmath.LegacyZeroDec()
	for _, c := range components {
		total = total.Add(c.GetWeight())
	}
	return total.Quo(sdkmath.LegacyNewDec(int64(len(components))))
}

func (k Keeper) setModelEnsemble(ctx context.Context, ensemble *types.ModelEnsemble) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := ensemble.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal ensemble: %w", err)
	}
	if err := kvStore.Set(types.ModelEnsembleKey(ensemble.EnsembleID), bz); err != nil {
		return err
	}
	for _, domain := range ensemble.Domains {
		_ = kvStore.Set(types.EnsembleByDomainKey(domain, ensemble.EnsembleID), []byte{0x01})
	}
	return nil
}

func (k Keeper) setDistillationJob(ctx context.Context, job *types.DistillationJob) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := job.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.DistillationJobKey(job.JobID), bz)
}

func (k Keeper) setRoutingDecision(ctx context.Context, decision *types.RoutingDecision) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := decision.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.RoutingDecisionKey(decision.DecisionID), bz)
}

func (k Keeper) nextEnsembleID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, _ := kvStore.Get(types.EnsembleSeqKey)
	var seq uint64
	if len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.EnsembleSeqKey, newBz)
	hash := sha256.Sum256([]byte(fmt.Sprintf("ensemble:%d", seq)))
	return fmt.Sprintf("ens-%x", hash[:8])
}

func (k Keeper) nextDistillationID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, _ := kvStore.Get(types.DistillationSeqKey)
	var seq uint64
	if len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.DistillationSeqKey, newBz)
	hash := sha256.Sum256([]byte(fmt.Sprintf("distill:%d", seq)))
	return fmt.Sprintf("dist-%x", hash[:8])
}

func (k Keeper) nextRoutingDecisionID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, _ := kvStore.Get(types.RoutingDecisionSeqKey)
	var seq uint64
	if len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.RoutingDecisionSeqKey, newBz)
	return fmt.Sprintf("route-%d", seq)
}
