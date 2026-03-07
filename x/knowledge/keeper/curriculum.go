package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Curriculum Training ────────────────────────────────────────────────────
//
// Curriculum training structures TDU consumption into pedagogical order.
// Instead of agents training on random data, curricula define:
//   1. What to learn first (foundations)
//   2. What comes next (intermediate, builds on foundations)
//   3. What comes last (advanced, requires full prerequisite chain)
//
// This mirrors how humans learn: you study algebra before calculus,
// data structures before distributed systems.
//
// The knowledge graph's "prerequisite" edges define the DAG.
// Curricula are the paths through that DAG.

// ─── CreateCurriculum ───────────────────────────────────────────────────────

// CreateCurriculum constructs a new training curriculum for a domain.
// Validates the prerequisite DAG is acyclic and stages are well-ordered.
func (k Keeper) CreateCurriculum(ctx context.Context, curriculum *types.Curriculum) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Validate domain.
	if curriculum.Domain != "" {
		if _, found := k.GetDomain(ctx, curriculum.Domain); !found {
			return "", types.ErrDomainNotFound.Wrapf("domain %s", curriculum.Domain)
		}
	}

	// Validate stages.
	if len(curriculum.Stages) == 0 {
		return "", types.ErrCurriculumEmpty.Wrap("curriculum must have at least one stage")
	}

	// Validate DAG: no prerequisite cycles.
	if err := validateCurriculumDAG(curriculum.Stages); err != nil {
		return "", err
	}

	// Count total TDUs.
	totalTDUs := uint64(0)
	for i, stage := range curriculum.Stages {
		totalTDUs += uint64(len(stage.TDUIDs))
		// Assign stage IDs if not set.
		if stage.StageID == "" {
			stageInput := fmt.Sprintf("%s:stage:%d", curriculum.Name, i)
			stageHash := sha256.Sum256([]byte(stageInput))
			curriculum.Stages[i].StageID = hex.EncodeToString(stageHash[:8])
		}
		curriculum.Stages[i].Order = uint64(i)
	}

	// Generate curriculum ID.
	curriculumID := k.nextCurriculumID(ctx)
	curriculum.CurriculumID = curriculumID
	curriculum.TotalTDUs = totalTDUs
	curriculum.CreatedAt = sdkCtx.BlockHeight()
	curriculum.UpdatedAt = sdkCtx.BlockHeight()
	if curriculum.Version == 0 {
		curriculum.Version = 1
	}
	if curriculum.Status == "" {
		curriculum.Status = types.CurriculumStatusDraft
	}

	// Compute average fitness of included TDUs.
	curriculum.AvgFitness = k.computeCurriculumFitness(ctx, curriculum).String()

	// Store.
	if err := k.setCurriculum(ctx, curriculum); err != nil {
		return "", err
	}

	// Index: domain → curriculumID.
	if curriculum.Domain != "" {
		_ = kvStore.Set(types.CurriculumDomainIndexKey(curriculum.Domain, curriculumID), []byte{0x01})
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventCurriculumCreated,
		sdk.NewAttribute(types.AttributeCurriculumID, curriculumID),
		sdk.NewAttribute("domain", curriculum.Domain),
		sdk.NewAttribute("stages", fmt.Sprintf("%d", len(curriculum.Stages))),
		sdk.NewAttribute("total_tdus", fmt.Sprintf("%d", totalTDUs)),
	))

	return curriculumID, nil
}

// ─── ActivateCurriculum ─────────────────────────────────────────────────────

// ActivateCurriculum transitions a draft curriculum to active.
func (k Keeper) ActivateCurriculum(ctx context.Context, curriculumID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	curriculum, found := k.GetCurriculum(ctx, curriculumID)
	if !found {
		return types.ErrCurriculumNotFound.Wrapf("curriculum %s", curriculumID)
	}
	if curriculum.Status != types.CurriculumStatusDraft {
		return fmt.Errorf("can only activate draft curricula, current status: %s", curriculum.Status)
	}

	curriculum.Status = types.CurriculumStatusActive
	curriculum.UpdatedAt = sdkCtx.BlockHeight()
	return k.setCurriculum(ctx, curriculum)
}

// ─── ArchiveCurriculum ──────────────────────────────────────────────────────

// ArchiveCurriculum marks a curriculum as archived (superseded by a newer version).
func (k Keeper) ArchiveCurriculum(ctx context.Context, curriculumID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	curriculum, found := k.GetCurriculum(ctx, curriculumID)
	if !found {
		return types.ErrCurriculumNotFound.Wrapf("curriculum %s", curriculumID)
	}

	curriculum.Status = types.CurriculumStatusArchived
	curriculum.UpdatedAt = sdkCtx.BlockHeight()

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventCurriculumArchived,
		sdk.NewAttribute(types.AttributeCurriculumID, curriculumID),
	))

	return k.setCurriculum(ctx, curriculum)
}

// ─── EnrollAgent ────────────────────────────────────────────────────────────

// EnrollAgent registers an agent to follow a curriculum's training sequence.
func (k Keeper) EnrollAgent(ctx context.Context, curriculumID, agentID string) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Validate curriculum.
	curriculum, found := k.GetCurriculum(ctx, curriculumID)
	if !found {
		return "", types.ErrCurriculumNotFound.Wrapf("curriculum %s", curriculumID)
	}
	if curriculum.Status != types.CurriculumStatusActive {
		return "", types.ErrCurriculumNotActive.Wrapf("status: %s", curriculum.Status)
	}

	// Validate agent.
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return "", types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	if agent.Status != types.AgentStatusActive {
		return "", types.ErrAgentNotActive.Wrapf("agent %s status: %s", agentID, agent.Status)
	}

	// Check not already enrolled.
	existingBz, err := kvStore.Get(types.EnrollmentByAgentKey(agentID, curriculumID))
	if err == nil && existingBz != nil {
		return "", types.ErrAlreadyEnrolled.Wrapf("agent %s in curriculum %s", agentID, curriculumID)
	}

	// Create enrollment.
	enrollmentID := k.nextEnrollmentID(ctx)
	enrollment := &types.CurriculumEnrollment{
		EnrollmentID: enrollmentID,
		CurriculumID: curriculumID,
		AgentID:      agentID,
		EnrolledAt:   sdkCtx.BlockHeight(),
		CurrentStage: 0,
		Status:       "active",
	}

	if err := k.setEnrollment(ctx, enrollment); err != nil {
		return "", err
	}

	// Indexes.
	_ = kvStore.Set(types.EnrollmentByAgentKey(agentID, curriculumID), []byte(enrollmentID))
	_ = kvStore.Set(types.EnrollmentByCurriculumKey(curriculumID, enrollmentID), []byte{0x01})

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAgentEnrolled,
		sdk.NewAttribute(types.AttributeCurriculumID, curriculumID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute(types.AttributeEnrollmentID, enrollmentID),
	))

	return enrollmentID, nil
}

// ─── AdvanceStage ───────────────────────────────────────────────────────────

// AdvanceStage marks the current stage as completed and moves to the next.
// Validates prerequisite completion before advancing.
func (k Keeper) AdvanceStage(ctx context.Context, enrollmentID string, completedTDUs []string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	enrollment, found := k.GetEnrollment(ctx, enrollmentID)
	if !found {
		return types.ErrEnrollmentNotFound.Wrapf("enrollment %s", enrollmentID)
	}
	if enrollment.Status == "completed" {
		return types.ErrEnrollmentCompleted.Wrapf("enrollment %s", enrollmentID)
	}

	curriculum, found := k.GetCurriculum(ctx, enrollment.CurriculumID)
	if !found {
		return types.ErrCurriculumNotFound.Wrapf("curriculum %s", enrollment.CurriculumID)
	}

	currentIdx := enrollment.CurrentStage
	if currentIdx >= uint64(len(curriculum.Stages)) {
		// Already past all stages — mark completed.
		enrollment.Status = "completed"
		return k.setEnrollment(ctx, enrollment)
	}

	currentStage := curriculum.Stages[currentIdx]

	// Validate prerequisites are completed.
	for _, prereqID := range currentStage.Prerequisites {
		completed := false
		for _, cs := range enrollment.CompletedStages {
			if cs == prereqID {
				completed = true
				break
			}
		}
		if !completed {
			return types.ErrStagePrereqNotMet.Wrapf("stage %s requires %s", currentStage.StageID, prereqID)
		}
	}

	// Mark TDUs as consumed.
	enrollment.CompletedTDUs = append(enrollment.CompletedTDUs, completedTDUs...)
	enrollment.TotalConsumed += uint64(len(completedTDUs))
	enrollment.CompletedStages = append(enrollment.CompletedStages, currentStage.StageID)
	enrollment.CurrentStage = currentIdx + 1

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventStageCompleted,
		sdk.NewAttribute(types.AttributeEnrollmentID, enrollmentID),
		sdk.NewAttribute(types.AttributeStageID, currentStage.StageID),
		sdk.NewAttribute(types.AttributeAgentID, enrollment.AgentID),
	))

	// Check if curriculum is complete.
	if enrollment.CurrentStage >= uint64(len(curriculum.Stages)) {
		enrollment.Status = "completed"
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventCurriculumComplete,
			sdk.NewAttribute(types.AttributeEnrollmentID, enrollmentID),
			sdk.NewAttribute(types.AttributeCurriculumID, enrollment.CurriculumID),
			sdk.NewAttribute(types.AttributeAgentID, enrollment.AgentID),
		))
	}

	return k.setEnrollment(ctx, enrollment)
}

// ─── GetNextTDUs ────────────────────────────────────────────────────────────

// GetNextTDUs returns the TDUs the agent should train on next, based on
// their current stage in the curriculum. Filters by minimum fitness.
func (k Keeper) GetNextTDUs(ctx context.Context, enrollmentID string) ([]string, string, error) {
	enrollment, found := k.GetEnrollment(ctx, enrollmentID)
	if !found {
		return nil, "", types.ErrEnrollmentNotFound.Wrapf("enrollment %s", enrollmentID)
	}
	if enrollment.Status == "completed" {
		return nil, "", types.ErrEnrollmentCompleted.Wrapf("enrollment %s", enrollmentID)
	}

	curriculum, found := k.GetCurriculum(ctx, enrollment.CurriculumID)
	if !found {
		return nil, "", types.ErrCurriculumNotFound.Wrapf("curriculum %s", enrollment.CurriculumID)
	}

	if enrollment.CurrentStage >= uint64(len(curriculum.Stages)) {
		return nil, "", nil // past all stages
	}

	stage := curriculum.Stages[enrollment.CurrentStage]

	// Filter TDUs by minimum fitness if specified.
	var qualified []string
	if stage.MinFitness != "" {
		minFit, err := sdkmath.LegacyNewDecFromStr(stage.MinFitness)
		if err == nil {
			for _, tduID := range stage.TDUIDs {
				fitnessRec, ok := k.GetFitnessRecord(ctx, tduID)
				if ok && fitnessRec.GetFitnessScore().GTE(minFit) {
					qualified = append(qualified, tduID)
				} else if !ok {
					// No fitness record — include by default (new TDU).
					qualified = append(qualified, tduID)
				}
			}
		} else {
			qualified = stage.TDUIDs
		}
	} else {
		qualified = stage.TDUIDs
	}

	// Remove already-consumed TDUs.
	consumedSet := make(map[string]bool, len(enrollment.CompletedTDUs))
	for _, id := range enrollment.CompletedTDUs {
		consumedSet[id] = true
	}
	var remaining []string
	for _, id := range qualified {
		if !consumedSet[id] {
			remaining = append(remaining, id)
		}
	}

	return remaining, stage.StageID, nil
}

// ─── BuildCurriculumFromGraph ───────────────────────────────────────────────

// BuildCurriculumFromGraph auto-generates a curriculum from the knowledge graph's
// prerequisite edges. Performs topological sort to determine stage ordering.
// TDUs with no prerequisites go in stage 0, TDUs requiring stage-0 TDUs go in
// stage 1, and so on.
func (k Keeper) BuildCurriculumFromGraph(ctx context.Context, domain, name, creator string, tduIDs []string) (*types.Curriculum, error) {
	if len(tduIDs) == 0 {
		return nil, types.ErrCurriculumEmpty.Wrap("no TDUs provided")
	}

	// Build adjacency from prerequisite edges.
	// prerequisite edge: A → B means "A must be learned before B"
	// So B depends on A (B has prerequisite A).
	tduSet := make(map[string]bool, len(tduIDs))
	for _, id := range tduIDs {
		tduSet[id] = true
	}

	deps := make(map[string][]string)     // tduID → prerequisite tduIDs
	revDeps := make(map[string][]string)  // tduID → TDUs that depend on this
	for _, tduID := range tduIDs {
		outEdges := k.GetOutgoingEdges(ctx, tduID)
		for _, edge := range outEdges {
			if edge.EdgeType != types.EdgeTypePrerequisite {
				continue
			}
			if !tduSet[edge.TargetID] {
				continue // target not in our set
			}
			// tduID is prerequisite for edge.TargetID
			deps[edge.TargetID] = append(deps[edge.TargetID], tduID)
			revDeps[tduID] = append(revDeps[tduID], edge.TargetID)
		}
	}

	// Topological sort using Kahn's algorithm to determine levels.
	inDegree := make(map[string]int, len(tduIDs))
	for _, id := range tduIDs {
		inDegree[id] = len(deps[id])
	}

	// Level assignment: BFS from roots.
	levels := make(map[string]uint64)
	queue := make([]string, 0)
	for _, id := range tduIDs {
		if inDegree[id] == 0 {
			queue = append(queue, id)
			levels[id] = 0
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++

		for _, dependent := range revDeps[current] {
			// Update level: max of all prerequisite levels + 1.
			newLevel := levels[current] + 1
			if existing, ok := levels[dependent]; !ok || newLevel > existing {
				levels[dependent] = newLevel
			}
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Cycle detection.
	if processed < len(tduIDs) {
		return nil, types.ErrCurriculumCycle.Wrap("prerequisite edges form a cycle")
	}

	// Group TDUs by level.
	maxLevel := uint64(0)
	levelGroups := make(map[uint64][]string)
	for _, id := range tduIDs {
		level := levels[id]
		levelGroups[level] = append(levelGroups[level], id)
		if level > maxLevel {
			maxLevel = level
		}
	}

	// Build stages.
	stageNames := []string{"foundations", "basics", "intermediate", "advanced", "expert", "mastery"}
	var stages []types.CurriculumStage
	for level := uint64(0); level <= maxLevel; level++ {
		ids, ok := levelGroups[level]
		if !ok || len(ids) == 0 {
			continue
		}
		sort.Strings(ids) // deterministic ordering

		stageName := fmt.Sprintf("level-%d", level)
		if int(level) < len(stageNames) {
			stageName = stageNames[level]
		}

		// Generate deterministic StageID.
		stageInput := fmt.Sprintf("%s:stage:%d", name, level)
		stageHash := sha256.Sum256([]byte(stageInput))
		stageID := hex.EncodeToString(stageHash[:8])

		stage := types.CurriculumStage{
			StageID: stageID,
			Name:    stageName,
			Order:   uint64(len(stages)),
			TDUIDs:  ids,
		}

		// Stages after the first require the previous stage.
		if len(stages) > 0 {
			stage.Prerequisites = []string{stages[len(stages)-1].StageID}
		}

		stages = append(stages, stage)
	}

	curriculum := &types.Curriculum{
		Name:    name,
		Domain:  domain,
		Creator: creator,
		Status:  types.CurriculumStatusDraft,
		Stages:  stages,
	}

	return curriculum, nil
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetCurriculum retrieves a curriculum by ID.
func (k Keeper) GetCurriculum(ctx context.Context, curriculumID string) (*types.Curriculum, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurriculumKey(curriculumID))
	if err != nil || bz == nil {
		return nil, false
	}
	var curriculum types.Curriculum
	if err := json.Unmarshal(bz, &curriculum); err != nil {
		return nil, false
	}
	return &curriculum, true
}

// GetCurriculaByDomain returns all curricula in a domain.
func (k Keeper) GetCurriculaByDomain(ctx context.Context, domain string) []*types.Curriculum {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.CurriculumByDomainPfx(domain)

	var curricula []*types.Curriculum
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		cID := string(iter.Key()[len(prefix):])
		c, found := k.GetCurriculum(ctx, cID)
		if found {
			curricula = append(curricula, c)
		}
	}
	return curricula
}

// GetEnrollment retrieves an enrollment by ID.
func (k Keeper) GetEnrollment(ctx context.Context, enrollmentID string) (*types.CurriculumEnrollment, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurriculumEnrollmentKey(enrollmentID))
	if err != nil || bz == nil {
		return nil, false
	}
	var enrollment types.CurriculumEnrollment
	if err := json.Unmarshal(bz, &enrollment); err != nil {
		return nil, false
	}
	return &enrollment, true
}

// GetAgentEnrollment retrieves an agent's enrollment in a specific curriculum.
func (k Keeper) GetAgentEnrollment(ctx context.Context, agentID, curriculumID string) (*types.CurriculumEnrollment, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	enrollmentIDBz, err := kvStore.Get(types.EnrollmentByAgentKey(agentID, curriculumID))
	if err != nil || enrollmentIDBz == nil {
		return nil, false
	}
	return k.GetEnrollment(ctx, string(enrollmentIDBz))
}

// GetCurriculumEnrollments returns all enrollments for a curriculum.
func (k Keeper) GetCurriculumEnrollments(ctx context.Context, curriculumID string) []*types.CurriculumEnrollment {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.EnrollmentByCurriculumPfx(curriculumID)

	var enrollments []*types.CurriculumEnrollment
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		eID := string(iter.Key()[len(prefix):])
		e, found := k.GetEnrollment(ctx, eID)
		if found {
			enrollments = append(enrollments, e)
		}
	}
	return enrollments
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setCurriculum(ctx context.Context, curriculum *types.Curriculum) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(curriculum)
	if err != nil {
		return fmt.Errorf("failed to marshal curriculum: %w", err)
	}
	return kvStore.Set(types.CurriculumKey(curriculum.CurriculumID), bz)
}

// SetEnrollment stores an enrollment (exported for testing and progression tracking).
func (k Keeper) SetEnrollment(ctx context.Context, enrollment *types.CurriculumEnrollment) error {
	return k.setEnrollment(ctx, enrollment)
}

func (k Keeper) setEnrollment(ctx context.Context, enrollment *types.CurriculumEnrollment) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(enrollment)
	if err != nil {
		return fmt.Errorf("failed to marshal enrollment: %w", err)
	}
	return kvStore.Set(types.CurriculumEnrollmentKey(enrollment.EnrollmentID), bz)
}

func (k Keeper) nextCurriculumID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurriculumSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	input := fmt.Sprintf("curriculum:%d", seq)
	hash := sha256.Sum256([]byte(input))
	id := hex.EncodeToString(hash[:16])
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = kvStore.Set(types.CurriculumSeqKey, next)
	return id
}

func (k Keeper) nextEnrollmentID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.EnrollmentSeqKey)
	var seq uint64 = 1
	if err == nil && len(bz) == 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	input := fmt.Sprintf("enrollment:%d", seq)
	hash := sha256.Sum256([]byte(input))
	id := hex.EncodeToString(hash[:16])
	next := make([]byte, 8)
	binary.BigEndian.PutUint64(next, seq+1)
	_ = kvStore.Set(types.EnrollmentSeqKey, next)
	return id
}

func (k Keeper) computeCurriculumFitness(ctx context.Context, curriculum *types.Curriculum) sdkmath.LegacyDec {
	total := sdkmath.LegacyZeroDec()
	count := 0
	for _, stage := range curriculum.Stages {
		for _, tduID := range stage.TDUIDs {
			fitnessRec, ok := k.GetFitnessRecord(ctx, tduID)
			if ok {
				total = total.Add(fitnessRec.GetFitnessScore())
				count++
			}
		}
	}
	if count == 0 {
		return sdkmath.LegacyZeroDec()
	}
	return total.Quo(sdkmath.LegacyNewDec(int64(count)))
}

// validateCurriculumDAG checks that stage prerequisites form a DAG (no cycles).
func validateCurriculumDAG(stages []types.CurriculumStage) error {
	// Build stage index.
	stageIdx := make(map[string]int, len(stages))
	for i, s := range stages {
		if s.StageID != "" {
			stageIdx[s.StageID] = i
		}
	}

	// Simple cycle detection: for each stage, prerequisites must reference
	// earlier stages (lower order). Since stages are ordered, any backward
	// reference is a cycle.
	for i, stage := range stages {
		for _, prereq := range stage.Prerequisites {
			prereqIdx, ok := stageIdx[prereq]
			if ok && prereqIdx >= i {
				return types.ErrCurriculumCycle.Wrapf(
					"stage %d (%s) requires stage %d (%s) which is not earlier",
					i, stage.StageID, prereqIdx, prereq,
				)
			}
		}
	}
	return nil
}
