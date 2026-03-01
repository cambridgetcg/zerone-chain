package keeper

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/tree/types"
)

// marshalDeterministic marshals a protobuf message with deterministic output.
// This is critical for consensus: validators must produce identical bytes for
// the same state, especially for types containing proto map fields (e.g.
// DemandSignal.Evidence). Without Deterministic:true, proto.Marshal does not
// guarantee stable ordering of map keys across Go/protobuf versions.
var marshalDeterministic = proto.MarshalOptions{Deterministic: true}.Marshal

// ======================== Project CRUD ========================

func (k Keeper) SetProject(ctx sdk.Context, project *types.ProductProject) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalDeterministic(project)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.ProjectKey(project.Id), bz); err != nil {
		panic(err)
	}
	if project.Founder != "" {
		if err := store.Set(types.ProjectFounderIndexKey(project.Founder, project.Id), []byte{0x01}); err != nil {
			panic(err)
		}
	}
	if project.KnowledgeDomain != "" {
		if err := store.Set(types.ProjectDomainIndexKey(project.KnowledgeDomain, project.Id), []byte{0x01}); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) GetProject(ctx sdk.Context, projectId string) (*types.ProductProject, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ProjectKey(projectId))
	if err != nil {
		panic(err)
	}
	if bz == nil {
		return nil, false
	}
	var project types.ProductProject
	if err := proto.Unmarshal(bz, &project); err != nil {
		panic(err)
	}
	return &project, true
}

func (k Keeper) DeleteProject(ctx sdk.Context, project *types.ProductProject) {
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Delete(types.ProjectKey(project.Id)); err != nil {
		panic(err)
	}
	if project.Founder != "" {
		_ = store.Delete(types.ProjectFounderIndexKey(project.Founder, project.Id))
	}
	if project.KnowledgeDomain != "" {
		_ = store.Delete(types.ProjectDomainIndexKey(project.KnowledgeDomain, project.Id))
	}
}

func (k Keeper) IterateProjects(ctx sdk.Context, cb func(*types.ProductProject) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ProjectKeyPrefix, prefixEndBytes(types.ProjectKeyPrefix))
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var project types.ProductProject
		if err := proto.Unmarshal(iter.Value(), &project); err != nil {
			panic(err)
		}
		if cb(&project) {
			break
		}
	}
}

func (k Keeper) GetProjectsByFounder(ctx sdk.Context, founder string) []*types.ProductProject {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(types.ProjectFounderIndexPrefix, []byte(founder+"/")...)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	var projects []*types.ProductProject
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		projectId := string(key[len(prefix):])
		project, found := k.GetProject(ctx, projectId)
		if found {
			projects = append(projects, project)
		}
	}
	return projects
}

func (k Keeper) GetNextProjectId(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.Uint64ToBytes(0x20))
	if err != nil {
		panic(err)
	}
	var counter uint64
	if bz != nil {
		counter = types.BytesToUint64(bz)
	}
	counter++
	if err := store.Set(types.Uint64ToBytes(0x20), types.Uint64ToBytes(counter)); err != nil {
		panic(err)
	}
	return counter
}

// ======================== Task CRUD ========================

func (k Keeper) SetTask(ctx sdk.Context, task *types.ProjectTask) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalDeterministic(task)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.TaskKey(task.Id), bz); err != nil {
		panic(err)
	}
	if task.ProjectId != "" {
		if err := store.Set(types.TaskProjectIndexKey(task.ProjectId, task.Id), []byte{0x01}); err != nil {
			panic(err)
		}
	}
	if task.Assignee != "" {
		if err := store.Set(types.TaskAssigneeIndexKey(task.Assignee, task.Id), []byte{0x01}); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) GetTask(ctx sdk.Context, taskId string) (*types.ProjectTask, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TaskKey(taskId))
	if err != nil {
		panic(err)
	}
	if bz == nil {
		return nil, false
	}
	var task types.ProjectTask
	if err := proto.Unmarshal(bz, &task); err != nil {
		panic(err)
	}
	return &task, true
}

func (k Keeper) DeleteTask(ctx sdk.Context, task *types.ProjectTask) {
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Delete(types.TaskKey(task.Id)); err != nil {
		panic(err)
	}
	if task.ProjectId != "" {
		_ = store.Delete(types.TaskProjectIndexKey(task.ProjectId, task.Id))
	}
	if task.Assignee != "" {
		_ = store.Delete(types.TaskAssigneeIndexKey(task.Assignee, task.Id))
	}
}

func (k Keeper) IterateTasks(ctx sdk.Context, cb func(*types.ProjectTask) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.TaskKeyPrefix, prefixEndBytes(types.TaskKeyPrefix))
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var task types.ProjectTask
		if err := proto.Unmarshal(iter.Value(), &task); err != nil {
			panic(err)
		}
		if cb(&task) {
			break
		}
	}
}

func (k Keeper) GetTasksByProject(ctx sdk.Context, projectId string) []*types.ProjectTask {
	store := k.storeService.OpenKVStore(ctx)
	prefix := append(types.TaskProjectIndexPrefix, []byte(projectId+"/")...)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	var tasks []*types.ProjectTask
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		taskId := string(key[len(prefix):])
		task, found := k.GetTask(ctx, taskId)
		if found {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (k Keeper) GetNextTaskId(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.Uint64ToBytes(0x21))
	if err != nil {
		panic(err)
	}
	var counter uint64
	if bz != nil {
		counter = types.BytesToUint64(bz)
	}
	counter++
	if err := store.Set(types.Uint64ToBytes(0x21), types.Uint64ToBytes(counter)); err != nil {
		panic(err)
	}
	return counter
}

// ======================== Service CRUD ========================

func (k Keeper) SetService(ctx sdk.Context, service *types.ServiceLeaf) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalDeterministic(service)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.ServiceKey(service.Id), bz); err != nil {
		panic(err)
	}
	if service.ProjectId != "" {
		if err := store.Set(types.ServiceProjectIndexKey(service.ProjectId, service.Id), []byte{0x01}); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) GetService(ctx sdk.Context, serviceId string) (*types.ServiceLeaf, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ServiceKey(serviceId))
	if err != nil {
		panic(err)
	}
	if bz == nil {
		return nil, false
	}
	var service types.ServiceLeaf
	if err := proto.Unmarshal(bz, &service); err != nil {
		panic(err)
	}
	return &service, true
}

func (k Keeper) IterateServices(ctx sdk.Context, cb func(*types.ServiceLeaf) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ServiceKeyPrefix, prefixEndBytes(types.ServiceKeyPrefix))
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var service types.ServiceLeaf
		if err := proto.Unmarshal(iter.Value(), &service); err != nil {
			panic(err)
		}
		if cb(&service) {
			break
		}
	}
}

func (k Keeper) GetNextServiceId(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.Uint64ToBytes(0x22))
	if err != nil {
		panic(err)
	}
	var counter uint64
	if bz != nil {
		counter = types.BytesToUint64(bz)
	}
	counter++
	if err := store.Set(types.Uint64ToBytes(0x22), types.Uint64ToBytes(counter)); err != nil {
		panic(err)
	}
	return counter
}

// ======================== Seed CRUD ========================

func (k Keeper) SetSeed(ctx sdk.Context, seed *types.OpportunitySeed) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalDeterministic(seed)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.SeedKey(seed.Id), bz); err != nil {
		panic(err)
	}
	if seed.KnowledgeDomain != "" {
		if err := store.Set(types.SeedDomainIndexKey(seed.KnowledgeDomain, seed.Id), []byte{0x01}); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) GetSeed(ctx sdk.Context, seedId string) (*types.OpportunitySeed, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SeedKey(seedId))
	if err != nil {
		panic(err)
	}
	if bz == nil {
		return nil, false
	}
	var seed types.OpportunitySeed
	if err := proto.Unmarshal(bz, &seed); err != nil {
		panic(err)
	}
	return &seed, true
}

func (k Keeper) IterateSeeds(ctx sdk.Context, cb func(*types.OpportunitySeed) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.SeedKeyPrefix, prefixEndBytes(types.SeedKeyPrefix))
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var seed types.OpportunitySeed
		if err := proto.Unmarshal(iter.Value(), &seed); err != nil {
			panic(err)
		}
		if cb(&seed) {
			break
		}
	}
}

func (k Keeper) GetNextSeedId(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.Uint64ToBytes(0x23))
	if err != nil {
		panic(err)
	}
	var counter uint64
	if bz != nil {
		counter = types.BytesToUint64(bz)
	}
	counter++
	if err := store.Set(types.Uint64ToBytes(0x23), types.Uint64ToBytes(counter)); err != nil {
		panic(err)
	}
	return counter
}

func (k Keeper) GetExpiredSeeds(ctx sdk.Context, currentBlock uint64) []*types.OpportunitySeed {
	var expired []*types.OpportunitySeed
	k.IterateSeeds(ctx, func(s *types.OpportunitySeed) bool {
		if s.Status != string(types.SeedExpired) && s.ExpiresAtBlock > 0 && currentBlock >= s.ExpiresAtBlock {
			expired = append(expired, s)
		}
		return false
	})
	return expired
}

// ======================== Subscription CRUD ========================

func (k Keeper) SetSubscription(ctx sdk.Context, sub types.ServiceSubscription) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(sub)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.SubscriptionKey(sub.Id), bz); err != nil {
		panic(err)
	}
	if sub.ServiceId != "" {
		if err := store.Set(types.SubscriptionServiceIndexKey(sub.ServiceId, sub.Id), []byte{0x01}); err != nil {
			panic(err)
		}
	}
}

func (k Keeper) GetSubscription(ctx sdk.Context, subId string) (types.ServiceSubscription, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.SubscriptionKey(subId))
	if err != nil {
		panic(err)
	}
	if bz == nil {
		return types.ServiceSubscription{}, false
	}
	var sub types.ServiceSubscription
	if err := json.Unmarshal(bz, &sub); err != nil {
		panic(err)
	}
	return sub, true
}

func (k Keeper) GetNextSubscriptionId(ctx sdk.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.Uint64ToBytes(0x24))
	if err != nil {
		panic(err)
	}
	var counter uint64
	if bz != nil {
		counter = types.BytesToUint64(bz)
	}
	counter++
	if err := store.Set(types.Uint64ToBytes(0x24), types.Uint64ToBytes(counter)); err != nil {
		panic(err)
	}
	return counter
}

func (k Keeper) HasActiveSubscription(ctx sdk.Context, serviceId, subscriber string) bool {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.SubscriptionServicePrefix(serviceId)
	end := prefixEndBytes(prefix)
	iter, err := store.Iterator(prefix, end)
	if err != nil {
		return false
	}
	defer iter.Close()

	currentBlock := uint64(ctx.BlockHeight())
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		subId := string(key[len(prefix):])
		sub, found := k.GetSubscription(ctx, subId)
		if !found {
			continue
		}
		if sub.Subscriber == subscriber && currentBlock < sub.ExpiresAtBlock {
			return true
		}
	}
	return false
}

// ======================== Params ========================

func (k Keeper) SetParams(ctx sdk.Context, params *types.Params) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalDeterministic(params)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.ParamsKey, bz); err != nil {
		panic(err)
	}
}

func (k Keeper) GetParams(ctx sdk.Context) *types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil {
		return types.DefaultParams()
	}
	if bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		return types.DefaultParams()
	}
	return &params
}

// ======================== Phase Transition Validation ========================

var ValidPhaseTransitions = map[types.ProjectPhase][]types.ProjectPhase{
	types.PhaseSeed:     {types.PhaseSprout, types.PhaseWithered},
	types.PhaseSprout:   {types.PhaseGrowing, types.PhaseWithered},
	types.PhaseGrowing:  {types.PhaseMature, types.PhaseDormant, types.PhaseWithered},
	types.PhaseMature:   {types.PhaseFruiting, types.PhaseDormant, types.PhaseWithered},
	types.PhaseFruiting: {types.PhaseSeeding, types.PhaseDormant, types.PhaseWithered},
	types.PhaseSeeding:  {types.PhaseMature, types.PhaseDormant, types.PhaseWithered},
	types.PhaseDormant:  {types.PhaseGrowing, types.PhaseMature, types.PhaseFruiting, types.PhaseWithered},
	types.PhaseWithered: {},
}

func IsValidPhaseTransition(from, to types.ProjectPhase) bool {
	validTargets, ok := ValidPhaseTransitions[from]
	if !ok {
		return false
	}
	for _, target := range validTargets {
		if target == to {
			return true
		}
	}
	return false
}

func IsContributor(project *types.ProductProject, addr string) bool {
	for _, c := range project.Contributors {
		if c.Did == addr {
			return true
		}
	}
	return false
}

func IsFounderOrContributor(project *types.ProductProject, addr string) bool {
	if project.Founder == addr {
		return true
	}
	return IsContributor(project, addr)
}

// ======================== Pending Abandon Proposals (SEC-1) ========================

func (k Keeper) SetPendingAbandon(ctx sdk.Context, pa types.PendingAbandon) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(pa)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.PendingAbandonKey(pa.ProjectId), bz); err != nil {
		panic(err)
	}
}

func (k Keeper) GetPendingAbandon(ctx sdk.Context, projectId string) (types.PendingAbandon, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.PendingAbandonKey(projectId))
	if err != nil || bz == nil {
		return types.PendingAbandon{}, false
	}
	var pa types.PendingAbandon
	if err := json.Unmarshal(bz, &pa); err != nil {
		return types.PendingAbandon{}, false
	}
	return pa, true
}

func (k Keeper) DeletePendingAbandon(ctx sdk.Context, projectId string) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.PendingAbandonKey(projectId))
}

func ContributorMajorityConsented(project *types.ProductProject, pa types.PendingAbandon) bool {
	nonFounderCount := 0
	for _, c := range project.Contributors {
		if c.Did != project.Founder {
			nonFounderCount++
		}
	}
	if nonFounderCount == 0 {
		return true
	}

	consentSet := make(map[string]bool)
	for _, addr := range pa.Consented {
		consentSet[addr] = true
	}

	consentedCount := 0
	for _, c := range project.Contributors {
		if c.Did != project.Founder && consentSet[c.Did] {
			consentedCount++
		}
	}

	return consentedCount*2 > nonFounderCount
}

// ======================== Agent Availability ========================

func (k Keeper) SetAgentAvailability(ctx sdk.Context, avail *types.AgentAvailability) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(avail)
	if err != nil {
		panic(err)
	}
	if err := store.Set(types.AgentAvailabilityKey(avail.Agent), bz); err != nil {
		panic(err)
	}
}

func (k Keeper) GetAgentAvailability(ctx sdk.Context, agent string) (*types.AgentAvailability, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AgentAvailabilityKey(agent))
	if err != nil {
		panic(err)
	}
	if bz == nil {
		return nil, false
	}
	var avail types.AgentAvailability
	if err := json.Unmarshal(bz, &avail); err != nil {
		panic(err)
	}
	return &avail, true
}
