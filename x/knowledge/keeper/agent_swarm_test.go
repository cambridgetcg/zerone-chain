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
	if params.MinMembersDefault != 2 {
		t.Errorf("default min members: got %d, want 2", params.MinMembersDefault)
	}
	if params.MaxMembersDefault != 21 {
		t.Errorf("default max members: got %d, want 21", params.MaxMembersDefault)
	}
	if params.MaxSwarmObjectives != 5 {
		t.Errorf("default max objectives: got %d, want 5", params.MaxSwarmObjectives)
	}
}

func TestSwarmParamsValidation(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	valid := types.DefaultSwarmParams()
	if err := k.SetSwarmParams(ctx, valid); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}

	// Invalid: min members = 0.
	invalid := valid
	invalid.MinMembersDefault = 0
	if err := k.SetSwarmParams(ctx, invalid); err == nil {
		t.Error("should reject min_members = 0")
	}

	// Invalid: max < min.
	invalid2 := valid
	invalid2.MaxMembersDefault = 1
	invalid2.MinMembersDefault = 5
	if err := k.SetSwarmParams(ctx, invalid2); err == nil {
		t.Error("should reject max < min")
	}

	// Invalid: contrib rate > 1.
	invalid3 := valid
	invalid3.DefaultContribRate = "1.500000000000000000"
	if err := k.SetSwarmParams(ctx, invalid3); err == nil {
		t.Error("should reject contrib rate > 1")
	}
}

func TestSwarmCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	swarm := &types.AgentSwarm{
		SwarmID: "swarm-test-1",
		Name:    "Math Collective",
		Domain:  "mathematics",
		Status:  types.SwarmStatusActive,
		Members: []types.SwarmMember{
			{AgentID: "agent-1", Role: types.SwarmRoleCurator, Contribution: "0.500000000000000000", RewardsEarned: "0"},
			{AgentID: "agent-2", Role: types.SwarmRoleReviewer, Contribution: "0.500000000000000000", RewardsEarned: "0"},
		},
		MinMembers:           2,
		MaxMembers:           21,
		CollectiveReputation: "0.700000000000000000",
		TreasuryBalance:      "10000000",
		TreasuryAddr:         "treasury-addr",
		FormedAt:             100,
		CreatorID:            "agent-1",
	}

	// Store.
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := swarm.Marshal()
	_ = kvStore.Set(types.AgentSwarmKey(swarm.SwarmID), bz)
	_ = kvStore.Set(types.SwarmByDomainKey(swarm.Domain, swarm.SwarmID), []byte{0x01})
	_ = kvStore.Set(types.SwarmByMemberKey("agent-1", swarm.SwarmID), []byte{0x01})
	_ = kvStore.Set(types.SwarmByMemberKey("agent-2", swarm.SwarmID), []byte{0x01})

	// Get.
	got, found := k.GetAgentSwarm(ctx, "swarm-test-1")
	if !found {
		t.Fatal("swarm not found")
	}
	if got.Name != "Math Collective" {
		t.Errorf("name: got %s", got.Name)
	}
	if got.MemberCount() != 2 {
		t.Errorf("members: got %d, want 2", got.MemberCount())
	}
	if !got.HasQuorum() {
		t.Error("should have quorum with 2 members")
	}

	// Get by domain.
	domainSwarms := k.GetSwarmsByDomain(ctx, "mathematics")
	if len(domainSwarms) != 1 {
		t.Fatalf("expected 1 swarm, got %d", len(domainSwarms))
	}

	// Get by member.
	memberSwarms := k.GetSwarmsByAgent(ctx, "agent-1")
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

	obj := &types.SwarmObjective{
		ObjectiveID: "obj-test-1",
		SwarmID:     "swarm-1",
		Description: "fill coverage gap in biology",
		TargetGapID: "gap-bio-1",
		TargetTDUs:  20,
		TargetFitness: "0.600000000000000000",
		Deadline:    10000,
		RewardPool:  "5000000",
		Status:      "active",
		CreatedAt:   100,
	}

	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, _ := obj.Marshal()
	_ = kvStore.Set(types.SwarmObjectiveKey(obj.ObjectiveID), bz)
	_ = kvStore.Set(types.SwarmObjectiveBySwarmKey(obj.SwarmID, obj.ObjectiveID), []byte{0x01})

	// Get.
	got, found := k.GetSwarmObjective(ctx, "obj-test-1")
	if !found {
		t.Fatal("objective not found")
	}
	if got.TargetTDUs != 20 {
		t.Errorf("target TDUs: got %d, want 20", got.TargetTDUs)
	}
	if got.Status != "active" {
		t.Errorf("status: got %s, want active", got.Status)
	}

	// Get by swarm.
	objectives := k.GetSwarmObjectives(ctx, "swarm-1")
	if len(objectives) != 1 {
		t.Fatalf("expected 1 objective, got %d", len(objectives))
	}
}

func TestSwarmMemberLookup(t *testing.T) {
	swarm := &types.AgentSwarm{
		Members: []types.SwarmMember{
			{AgentID: "agent-a", Role: types.SwarmRoleCurator},
			{AgentID: "agent-b", Role: types.SwarmRoleReviewer},
		},
	}

	// Found.
	member := swarm.GetMember("agent-a")
	if member == nil {
		t.Fatal("should find agent-a")
	}
	if member.Role != types.SwarmRoleCurator {
		t.Errorf("role: got %s, want curator", member.Role)
	}

	// Not found.
	member = swarm.GetMember("agent-c")
	if member != nil {
		t.Error("should not find agent-c")
	}
}

func TestSwarmQuorum(t *testing.T) {
	tests := []struct {
		name       string
		members    int
		minMembers uint64
		want       bool
	}{
		{"has quorum", 3, 2, true},
		{"exact quorum", 2, 2, true},
		{"no quorum", 1, 2, false},
		{"empty", 0, 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			swarm := &types.AgentSwarm{MinMembers: tt.minMembers}
			for i := 0; i < tt.members; i++ {
				swarm.Members = append(swarm.Members, types.SwarmMember{AgentID: fmt.Sprintf("agent-%d", i)})
			}
			if got := swarm.HasQuorum(); got != tt.want {
				t.Errorf("HasQuorum() = %v, want %v", got, tt.want)
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

	if types.ValidSwarmRoles["invalid"] {
		t.Error("invalid role should not be valid")
	}
}

func TestSwarmTreasuryDerivation(t *testing.T) {
	addr1 := types.DeriveSwarmTreasuryAddr("swarm-1")
	addr2 := types.DeriveSwarmTreasuryAddr("swarm-2")
	addr1Again := types.DeriveSwarmTreasuryAddr("swarm-1")

	// Deterministic.
	if addr1 != addr1Again {
		t.Error("same swarm ID should produce same treasury address")
	}

	// Different swarms → different addresses.
	if addr1 == addr2 {
		t.Error("different swarm IDs should produce different treasury addresses")
	}

	// Non-empty.
	if addr1 == "" {
		t.Error("treasury address should not be empty")
	}
}

func TestMemberContribution(t *testing.T) {
	tests := []struct {
		name         string
		contribution string
		want         string
	}{
		{"normal", "0.600000000000000000", "0.600000000000000000"},
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

func TestSwarmMarshal(t *testing.T) {
	swarm := types.AgentSwarm{
		SwarmID:    "swarm-marshal",
		Name:       "Test Swarm",
		Domain:     "cs",
		Status:     types.SwarmStatusActive,
		MinMembers: 3,
		MaxMembers: 10,
		Members: []types.SwarmMember{
			{AgentID: "a1", Role: types.SwarmRoleCurator, Contribution: "1.000000000000000000"},
		},
	}

	bz, err := swarm.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded types.AgentSwarm
	if err := decoded.Unmarshal(bz); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.SwarmID != "swarm-marshal" {
		t.Errorf("swarm ID: got %s", decoded.SwarmID)
	}
	if len(decoded.Members) != 1 {
		t.Errorf("members: got %d", len(decoded.Members))
	}
	if decoded.Status != types.SwarmStatusActive {
		t.Errorf("status: got %s", decoded.Status)
	}
}

func TestDefaultSwarmParamsValid(t *testing.T) {
	params := types.DefaultSwarmParams()
	if err := params.Validate(); err != nil {
		t.Fatalf("defaults should be valid: %v", err)
	}
}
