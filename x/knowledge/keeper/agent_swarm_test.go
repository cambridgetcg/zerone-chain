package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R55: Agent Swarm Tests ─────────────────────────────────────────────────

func TestSwarmParamsDefaults(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	params := k.GetSwarmParams(ctx)
	if params.MinSwarmSize != 2 {
		t.Errorf("default min size: got %d, want 2", params.MinSwarmSize)
	}
	if params.MaxSwarmSize != 21 {
		t.Errorf("default max size: got %d, want 21", params.MaxSwarmSize)
	}
	if params.MaxObjectives != 5 {
		t.Errorf("default max objectives: got %d, want 5", params.MaxObjectives)
	}
}

func TestSwarmParamsValidation(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	valid := types.DefaultSwarmParams()
	if err := k.SetSwarmParams(ctx, valid); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}

	// Invalid: min > max.
	invalid := valid
	invalid.MinSwarmSize = 30
	invalid.MaxSwarmSize = 10
	if err := k.SetSwarmParams(ctx, invalid); err == nil {
		t.Error("should reject min > max")
	}

	// Invalid: min = 0.
	invalid2 := valid
	invalid2.MinSwarmSize = 0
	if err := k.SetSwarmParams(ctx, invalid2); err == nil {
		t.Error("should reject min = 0")
	}

	// Invalid: treasury tax > 1.
	invalid3 := valid
	invalid3.TreasuryTax = "2.000000000000000000"
	if err := k.SetSwarmParams(ctx, invalid3); err == nil {
		t.Error("should reject treasury tax > 1")
	}
}

func TestSwarmCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	swarm := &types.AgentSwarm{
		SwarmID: "swarm-test-1",
		Name:    "Math Collective",
		Domain:  "mathematics",
		Members: []types.SwarmMember{
			{AgentID: "agent-1", Role: types.SwarmRoleCurator, JoinedAt: 100},
			{AgentID: "agent-2", Role: types.SwarmRoleReviewer, JoinedAt: 100},
		},
		MinMembers:           2,
		MaxMembers:           21,
		CollectiveReputation: "0.700000000000000000",
		TreasuryBalance:      "5000000",
		TreasuryAddr:         "treasury-addr-123",
		Status:               types.SwarmStatusActive,
		FormedAt:             100,
		Creator:              "agent-1",
	}

	// Store.
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := swarm.Marshal()
	_ = kvStore.Set(types.AgentSwarmKey(swarm.SwarmID), bz)
	_ = kvStore.Set(types.SwarmByDomainKey(swarm.Domain, swarm.SwarmID), []byte{0x01})
	_ = kvStore.Set(types.SwarmByMemberKey("agent-1", swarm.SwarmID), []byte{0x01})
	_ = kvStore.Set(types.SwarmByMemberKey("agent-2", swarm.SwarmID), []byte{0x01})
	_ = kvStore.Set(types.SwarmActiveKey(swarm.SwarmID), []byte{0x01})

	// Get.
	got, found := k.GetAgentSwarm(ctx, "swarm-test-1")
	if !found {
		t.Fatal("swarm not found")
	}
	if got.Name != "Math Collective" {
		t.Errorf("name: got %s, want Math Collective", got.Name)
	}
	if got.MemberCount() != 2 {
		t.Errorf("members: got %d, want 2", got.MemberCount())
	}
	if !got.HasMember("agent-1") {
		t.Error("should have agent-1 as member")
	}
	if got.HasMember("agent-3") {
		t.Error("should not have agent-3 as member")
	}

	// By domain.
	domainSwarms := k.GetSwarmsByDomain(ctx, "mathematics")
	if len(domainSwarms) != 1 {
		t.Fatalf("expected 1 swarm for mathematics, got %d", len(domainSwarms))
	}

	// By member.
	memberSwarms := k.GetSwarmsByMember(ctx, "agent-1")
	if len(memberSwarms) != 1 {
		t.Fatalf("expected 1 swarm for agent-1, got %d", len(memberSwarms))
	}

	// Not found.
	_, found = k.GetAgentSwarm(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent swarm")
	}
}

func TestSwarmObjectiveCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	objective := &types.SwarmObjective{
		ObjectiveID: "sobj-test-1",
		SwarmID:     "swarm-1",
		Description: "Collect 50 TDUs on algebra",
		TargetTDUs:  50,
		TargetFitness: "0.600000000000000000",
		TDUsSubmitted: 30,
		AvgFitness:    "0.650000000000000000",
		RewardPool:    "10000000",
		Status:        "active",
		CreatedAt:     100,
	}

	// Store.
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := objective.Marshal()
	_ = kvStore.Set(types.SwarmObjectiveKey(objective.ObjectiveID), bz)
	_ = kvStore.Set(types.SwarmObjBySwarmKey(objective.SwarmID, objective.ObjectiveID), []byte{0x01})

	// Get.
	got, found := k.GetSwarmObjective(ctx, "sobj-test-1")
	if !found {
		t.Fatal("objective not found")
	}
	if got.TargetTDUs != 50 {
		t.Errorf("target TDUs: got %d, want 50", got.TargetTDUs)
	}
	if got.IsComplete() {
		t.Error("30/50 TDUs should not be complete")
	}

	// Mark as complete.
	got.TDUsSubmitted = 50
	if !got.IsComplete() {
		t.Error("50/50 TDUs with fitness >= target should be complete")
	}

	// By swarm.
	swarmObjs := k.GetSwarmObjectives(ctx, "swarm-1")
	if len(swarmObjs) != 1 {
		t.Fatalf("expected 1 objective for swarm-1, got %d", len(swarmObjs))
	}
}

func TestObjectiveCompletion(t *testing.T) {
	tests := []struct {
		name     string
		target   uint64
		actual   uint64
		targetF  string
		actualF  string
		complete bool
	}{
		{"not enough TDUs", 50, 30, "", "", false},
		{"enough TDUs no fitness", 50, 50, "", "", true},
		{"enough TDUs fitness met", 50, 50, "0.6", "0.7", true},
		{"enough TDUs fitness not met", 50, 50, "0.8", "0.5", false},
		{"zero target", 0, 0, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &types.SwarmObjective{
				TargetTDUs:    tt.target,
				TDUsSubmitted: tt.actual,
				TargetFitness: tt.targetF,
				AvgFitness:    tt.actualF,
			}
			if got := obj.IsComplete(); got != tt.complete {
				t.Errorf("IsComplete() = %v, want %v", got, tt.complete)
			}
		})
	}
}

func TestSwarmMemberContribution(t *testing.T) {
	tests := []struct {
		name         string
		contribution string
		want         string
	}{
		{"has contribution", "0.450000000000000000", "0.450000000000000000"},
		{"zero", "0.000000000000000000", "0.000000000000000000"},
		{"empty", "", "0.000000000000000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &types.SwarmMember{Contribution: tt.contribution}
			got := m.GetContribution()
			expected, _ := sdkmath.LegacyNewDecFromStr(tt.want)
			if !got.Equal(expected) {
				t.Errorf("got %s, want %s", got, expected)
			}
		})
	}
}

func TestSwarmRoleValidation(t *testing.T) {
	validRoles := []types.SwarmRole{
		types.SwarmRoleCurator,
		types.SwarmRoleReviewer,
		types.SwarmRoleStrategist,
		types.SwarmRoleTrainer,
	}
	for _, role := range validRoles {
		if !types.ValidSwarmRoles[role] {
			t.Errorf("role %s should be valid", role)
		}
	}
	if types.ValidSwarmRoles["nonexistent"] {
		t.Error("nonexistent role should be invalid")
	}
}

func TestDeriveSwarmTreasury(t *testing.T) {
	// Deterministic.
	addr1 := types.DeriveSwarmTreasury("swarm-1")
	addr2 := types.DeriveSwarmTreasury("swarm-1")
	if addr1 != addr2 {
		t.Error("treasury address should be deterministic")
	}

	// Different swarms → different addresses.
	addr3 := types.DeriveSwarmTreasury("swarm-2")
	if addr1 == addr3 {
		t.Error("different swarms should have different treasury addresses")
	}

	// Not empty.
	if addr1 == "" {
		t.Error("treasury address should not be empty")
	}
}

func TestCollectiveReputation(t *testing.T) {
	swarm := &types.AgentSwarm{
		CollectiveReputation: "0.750000000000000000",
	}
	got := swarm.GetCollectiveReputation()
	expected := sdkmath.LegacyNewDecWithPrec(75, 2)
	if !got.Equal(expected) {
		t.Errorf("reputation: got %s, want %s", got, expected)
	}

	empty := &types.AgentSwarm{}
	if !empty.GetCollectiveReputation().Equal(sdkmath.LegacyZeroDec()) {
		t.Error("empty reputation should be zero")
	}
}

func TestSwarmMarshalRoundtrip(t *testing.T) {
	swarm := types.AgentSwarm{
		SwarmID: "swarm-rt",
		Name:    "Test Swarm",
		Domain:  "physics",
		Members: []types.SwarmMember{
			{AgentID: "a1", Role: types.SwarmRoleCurator},
			{AgentID: "a2", Role: types.SwarmRoleReviewer},
		},
		Status: types.SwarmStatusActive,
	}

	bz, err := swarm.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded types.AgentSwarm
	if err := decoded.Unmarshal(bz); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.SwarmID != "swarm-rt" {
		t.Errorf("swarm ID: got %s", decoded.SwarmID)
	}
	if len(decoded.Members) != 2 {
		t.Errorf("members: got %d, want 2", len(decoded.Members))
	}
}

func TestDefaultSwarmParamsValid(t *testing.T) {
	params := types.DefaultSwarmParams()
	if err := params.Validate(); err != nil {
		t.Fatalf("defaults should be valid: %v", err)
	}
}
