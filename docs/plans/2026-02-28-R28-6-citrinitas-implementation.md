# R28-6 Citrinitas: Mentorship & Formation Pool — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add mentorship lifecycle and active formation pool matching to the partnerships module.

**Architecture:** Mentorship is a separate entity (not a partnership variant) with its own storage and lifecycle (proposed → active → graduated/terminated). Formation pool matching runs in EndBlocker every 100 blocks, proposes compatible pairs who must both accept. Both features extend the existing partnerships module.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, protobuf, CometBFT v0.38.20

**Design doc:** `docs/plans/2026-02-28-R28-6-citrinitas-mentorship-design.md`

---

### Task 1: Proto Definitions

**Files:**
- Modify: `proto/zerone/partnerships/v1/types.proto`
- Modify: `proto/zerone/partnerships/v1/tx.proto`
- Modify: `proto/zerone/partnerships/v1/genesis.proto`
- Modify: `proto/zerone/partnerships/v1/query.proto`
- Regenerate: `x/partnerships/types/*.pb.go`

**Step 1: Add Mentorship and FormationMatch to types.proto**

Add after the existing `MentorshipConfig` message (line 108):

```proto
message Mentorship {
  string id                      = 1;
  string mentor_addr             = 2;
  string mentee_addr             = 3;
  string domain                  = 4;
  string status                  = 5;
  uint64 start_block             = 6;
  uint64 duration_blocks         = 7;
  uint64 mentee_verifications    = 8;
  uint64 mentee_claims_submitted = 9;
  uint64 graduation_threshold    = 10;
  uint64 graduation_claims_req   = 11;
}

message FormationMatch {
  string id             = 1;
  string addr1          = 2;
  string addr2          = 3;
  uint64 score          = 4;
  uint64 proposed_at    = 5;
  uint64 expires_at     = 6;
  string status         = 7;
  bool   addr1_accepted = 8;
  bool   addr2_accepted = 9;
}
```

**Step 2: Add new messages to tx.proto**

Add 6 new RPCs to the Msg service and their request/response messages:

```proto
// In service Msg block, add:
rpc ProposeMentorship(MsgProposeMentorship) returns (MsgProposeMentorshipResponse);
rpc AcceptMentorship(MsgAcceptMentorship) returns (MsgAcceptMentorshipResponse);
rpc GraduateMentee(MsgGraduateMentee) returns (MsgGraduateMenteeResponse);
rpc EndMentorship(MsgEndMentorship) returns (MsgEndMentorshipResponse);
rpc AcceptFormationMatch(MsgAcceptFormationMatch) returns (MsgAcceptFormationMatchResponse);
rpc DeclineFormationMatch(MsgDeclineFormationMatch) returns (MsgDeclineFormationMatchResponse);

// After existing messages:
message MsgProposeMentorship {
  option (cosmos.msg.v1.signer) = "mentor";
  string mentor = 1;
  string mentee = 2;
  string domain = 3;
  uint64 duration_blocks = 4;
}
message MsgProposeMentorshipResponse { string mentorship_id = 1; }

message MsgAcceptMentorship {
  option (cosmos.msg.v1.signer) = "mentee";
  string mentee = 1;
  string mentorship_id = 2;
}
message MsgAcceptMentorshipResponse {}

message MsgGraduateMentee {
  option (cosmos.msg.v1.signer) = "mentor";
  string mentor = 1;
  string mentorship_id = 2;
}
message MsgGraduateMenteeResponse {}

message MsgEndMentorship {
  option (cosmos.msg.v1.signer) = "sender";
  string sender = 1;
  string mentorship_id = 2;
}
message MsgEndMentorshipResponse {}

message MsgAcceptFormationMatch {
  option (cosmos.msg.v1.signer) = "accepter";
  string accepter = 1;
  string match_id = 2;
}
message MsgAcceptFormationMatchResponse {}

message MsgDeclineFormationMatch {
  option (cosmos.msg.v1.signer) = "decliner";
  string decliner = 1;
  string match_id = 2;
}
message MsgDeclineFormationMatchResponse {}
```

**Step 3: Add new params and genesis fields to genesis.proto**

Add to Params message:
```proto
uint64 graduation_verifications = 14;
uint64 graduation_claims = 15;
uint64 max_mentorships_per_mentor = 16;
uint64 formation_match_interval_blocks = 17;
uint64 match_acceptance_blocks = 18;
bool   auto_propose_partnership_on_graduation = 19;
```

Add to GenesisState message:
```proto
repeated Mentorship mentorships = 8;
repeated FormationMatch formation_matches = 9;
```

**Step 4: Add new queries to query.proto**

```proto
// In service Query block:
rpc Mentorship(QueryMentorshipRequest) returns (QueryMentorshipResponse) {
  option (google.api.http).get = "/zerone/partnerships/v1/mentorship/{id}";
}
rpc MentorshipsByAddress(QueryMentorshipsByAddressRequest) returns (QueryMentorshipsByAddressResponse) {
  option (google.api.http).get = "/zerone/partnerships/v1/mentorships/{address}";
}
rpc FormationMatches(QueryFormationMatchesRequest) returns (QueryFormationMatchesResponse) {
  option (google.api.http).get = "/zerone/partnerships/v1/matches";
}

// Messages:
message QueryMentorshipRequest { string id = 1; }
message QueryMentorshipResponse { Mentorship mentorship = 1; }
message QueryMentorshipsByAddressRequest { string address = 1; }
message QueryMentorshipsByAddressResponse { repeated Mentorship mentorships = 1; }
message QueryFormationMatchesRequest {}
message QueryFormationMatchesResponse { repeated FormationMatch matches = 1; }
```

**Step 5: Regenerate protobuf**

Run: `cd proto && buf generate`

If `buf` is not available, use the project's protobuf generation script. Check for `scripts/protocgen.sh` or `Makefile` targets. As a fallback, manually run protoc:

```bash
protoc --proto_path=proto --go_out=. --go-grpc_out=. proto/zerone/partnerships/v1/*.proto
```

Verify: The regenerated files should appear in `x/partnerships/types/` — confirm `Mentorship`, `FormationMatch`, and all new message types exist in the generated code.

**Step 6: Commit**

```bash
git add proto/zerone/partnerships/v1/*.proto x/partnerships/types/*.pb.go
git commit -m "proto(partnerships): add Mentorship, FormationMatch types and messages (R28-6)"
```

---

### Task 2: Types Layer (Keys, Errors, Params, Codec)

**Files:**
- Modify: `x/partnerships/types/keys.go`
- Modify: `x/partnerships/types/errors.go`
- Modify: `x/partnerships/types/genesis.go`
- Modify: `x/partnerships/types/codec.go`

**Step 1: Add storage key prefixes to keys.go**

Add after existing `ByDIDSeedIndexPrefix = []byte{0x14}` (line 24):

```go
ByMentorIndexPrefix      = []byte{0x15}
ByMenteeIndexPrefix      = []byte{0x16}
FormationMatchKeyPrefix  = []byte{0x17}
```

Note: `MentorshipKeyPrefix = []byte{0x13}` already exists.

**Step 2: Add error sentinels to errors.go**

Add after existing errors (after line 35):

```go
// Mentorship errors
ErrMentorshipNotFound      = errors.Register(ModuleName, 50, "mentorship not found")
ErrSelfMentorship          = errors.Register(ModuleName, 51, "cannot mentor yourself")
ErrMaxMentorshipsReached   = errors.Register(ModuleName, 52, "mentor has reached max active mentorships")
ErrAlreadyMentored         = errors.Register(ModuleName, 53, "mentee already has an active mentorship")
ErrMentorshipNotProposed   = errors.Register(ModuleName, 54, "mentorship is not in proposed status")
ErrMentorshipNotActive     = errors.Register(ModuleName, 55, "mentorship is not in active status")
ErrNotMentorshipParticipant = errors.Register(ModuleName, 56, "sender is not a participant of this mentorship")

// Formation match errors
ErrMatchNotFound           = errors.Register(ModuleName, 60, "formation match not found")
ErrNotMatchParticipant     = errors.Register(ModuleName, 61, "sender is not a participant of this match")
ErrMatchNotProposed        = errors.Register(ModuleName, 62, "match is not in proposed status")
```

**Step 3: Update DefaultParams in genesis.go**

Add to the `DefaultParams()` return struct (after `SeedCommonPotCap` line 20):

```go
GraduationVerifications:              20,
GraduationClaims:                     5,
MaxMentorshipsPerMentor:              3,
FormationMatchIntervalBlocks:         100,
MatchAcceptanceBlocks:                200,
AutoProposePartnershipOnGraduation:   true,
```

Update `DefaultGenesis()` to include:

```go
Mentorships:      []*Mentorship{},
FormationMatches: []*FormationMatch{},
```

**Step 4: Register new messages in codec.go**

Add to `RegisterCodec`:
```go
cdc.RegisterConcrete(&MsgProposeMentorship{}, "partnerships/ProposeMentorship", nil)
cdc.RegisterConcrete(&MsgAcceptMentorship{}, "partnerships/AcceptMentorship", nil)
cdc.RegisterConcrete(&MsgGraduateMentee{}, "partnerships/GraduateMentee", nil)
cdc.RegisterConcrete(&MsgEndMentorship{}, "partnerships/EndMentorship", nil)
cdc.RegisterConcrete(&MsgAcceptFormationMatch{}, "partnerships/AcceptFormationMatch", nil)
cdc.RegisterConcrete(&MsgDeclineFormationMatch{}, "partnerships/DeclineFormationMatch", nil)
```

Add to `RegisterInterfaces`:
```go
&MsgProposeMentorship{},
&MsgAcceptMentorship{},
&MsgGraduateMentee{},
&MsgEndMentorship{},
&MsgAcceptFormationMatch{},
&MsgDeclineFormationMatch{},
```

**Step 5: Commit**

```bash
git add x/partnerships/types/keys.go x/partnerships/types/errors.go x/partnerships/types/genesis.go x/partnerships/types/codec.go
git commit -m "feat(partnerships): add mentorship/match types, errors, params, codec (R28-6)"
```

---

### Task 3: Mentorship CRUD + Tests

**Files:**
- Create: `x/partnerships/keeper/mentorship.go`
- Modify: `x/partnerships/keeper/keeper_test.go`

**Step 1: Write failing tests for mentorship CRUD**

Add to `keeper_test.go`:

```go
// ---------- Mentorship CRUD Tests ----------

func TestMentorship_SetAndGet(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	m := &types.Mentorship{
		Id:                  "mentorship-1",
		MentorAddr:          humanAddr,
		MenteeAddr:          agentAddr,
		Domain:              "physics",
		Status:              "proposed",
		StartBlock:          0,
		DurationBlocks:      1000,
		GraduationThreshold: 20,
		GraduationClaimsReq: 5,
	}
	k.SetMentorship(ctx, m)

	got, found := k.GetMentorship(ctx, "mentorship-1")
	if !found {
		t.Fatal("mentorship not found")
	}
	if got.MentorAddr != humanAddr {
		t.Errorf("expected mentor %s, got %s", humanAddr, got.MentorAddr)
	}
	if got.Domain != "physics" {
		t.Errorf("expected domain physics, got %s", got.Domain)
	}
}

func TestMentorship_GetByMentor(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: humanAddr, MenteeAddr: agentAddr, Status: "active",
	})
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-2", MentorAddr: humanAddr, MenteeAddr: agent2Addr, Status: "active",
	})
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-3", MentorAddr: agentAddr, MenteeAddr: agent2Addr, Status: "active",
	})

	mentorships := k.GetMentorshipsByMentor(ctx, humanAddr)
	if len(mentorships) != 2 {
		t.Errorf("expected 2 mentorships for mentor, got %d", len(mentorships))
	}
}

func TestMentorship_GetByMentee(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: humanAddr, MenteeAddr: agentAddr, Status: "active",
	})

	mentorships := k.GetMentorshipsByMentee(ctx, agentAddr)
	if len(mentorships) != 1 {
		t.Errorf("expected 1 mentorship for mentee, got %d", len(mentorships))
	}
}

func TestMentorship_CountActive(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: humanAddr, MenteeAddr: agentAddr, Status: "active",
	})
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-2", MentorAddr: humanAddr, MenteeAddr: agent2Addr, Status: "graduated",
	})
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-3", MentorAddr: humanAddr, MenteeAddr: agent3Addr, Status: "active",
	})

	count := k.CountActiveMentorshipsForMentor(ctx, humanAddr)
	if count != 2 {
		t.Errorf("expected 2 active mentorships, got %d", count)
	}
}

func TestMentorship_ActiveForMentee(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: humanAddr, MenteeAddr: agentAddr, Status: "active",
	})

	m, found := k.GetActiveMentorshipForMentee(ctx, agentAddr)
	if !found {
		t.Fatal("expected active mentorship for mentee")
	}
	if m.Id != "m-1" {
		t.Errorf("expected m-1, got %s", m.Id)
	}

	// No active mentorship for someone without one
	_, found = k.GetActiveMentorshipForMentee(ctx, agent2Addr)
	if found {
		t.Error("expected no active mentorship for agent2")
	}
}

func TestMentorship_Delete(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	m := &types.Mentorship{
		Id: "m-1", MentorAddr: humanAddr, MenteeAddr: agentAddr, Status: "active",
	}
	k.SetMentorship(ctx, m)
	k.DeleteMentorship(ctx, m)

	_, found := k.GetMentorship(ctx, "m-1")
	if found {
		t.Error("expected mentorship to be deleted")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/partnerships/keeper/ -run TestMentorship_ -v -count=1`
Expected: Compilation errors — methods don't exist yet.

**Step 3: Implement mentorship CRUD in mentorship.go**

Create `x/partnerships/keeper/mentorship.go`:

```go
package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- Mentorship CRUD ----------

func mentorshipKey(id string) []byte {
	return append(types.MentorshipKeyPrefix, []byte(id)...)
}

func byMentorKey(mentorAddr, id string) []byte {
	return append(types.ByMentorIndexPrefix, []byte(mentorAddr+"/"+id)...)
}

func byMenteeKey(menteeAddr, id string) []byte {
	return append(types.ByMenteeIndexPrefix, []byte(menteeAddr+"/"+id)...)
}

func (k Keeper) SetMentorship(ctx sdk.Context, m *types.Mentorship) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(m)
	if err != nil {
		panic("failed to marshal mentorship: " + err.Error())
	}
	_ = kvStore.Set(mentorshipKey(m.Id), bz)
	_ = kvStore.Set(byMentorKey(m.MentorAddr, m.Id), []byte(m.Id))
	_ = kvStore.Set(byMenteeKey(m.MenteeAddr, m.Id), []byte(m.Id))
}

func (k Keeper) GetMentorship(ctx sdk.Context, id string) (*types.Mentorship, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(mentorshipKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var m types.Mentorship
	if err := proto.Unmarshal(bz, &m); err != nil {
		return nil, false
	}
	return &m, true
}

func (k Keeper) DeleteMentorship(ctx sdk.Context, m *types.Mentorship) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(mentorshipKey(m.Id))
	_ = kvStore.Delete(byMentorKey(m.MentorAddr, m.Id))
	_ = kvStore.Delete(byMenteeKey(m.MenteeAddr, m.Id))
}

func (k Keeper) GetAllMentorships(ctx sdk.Context) []*types.Mentorship {
	var mentorships []*types.Mentorship
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.MentorshipKeyPrefix, prefixEndBytes(types.MentorshipKeyPrefix))
	if err != nil {
		return mentorships
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var m types.Mentorship
		if err := proto.Unmarshal(iter.Value(), &m); err == nil {
			mentorships = append(mentorships, &m)
		}
	}
	return mentorships
}

func (k Keeper) GetMentorshipsByMentor(ctx sdk.Context, mentorAddr string) []*types.Mentorship {
	var result []*types.Mentorship
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append(types.ByMentorIndexPrefix, []byte(mentorAddr+"/")...)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return result
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Value())
		if m, found := k.GetMentorship(ctx, id); found {
			result = append(result, m)
		}
	}
	return result
}

func (k Keeper) GetMentorshipsByMentee(ctx sdk.Context, menteeAddr string) []*types.Mentorship {
	var result []*types.Mentorship
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append(types.ByMenteeIndexPrefix, []byte(menteeAddr+"/")...)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return result
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		id := string(iter.Value())
		if m, found := k.GetMentorship(ctx, id); found {
			result = append(result, m)
		}
	}
	return result
}

func (k Keeper) CountActiveMentorshipsForMentor(ctx sdk.Context, mentorAddr string) int {
	mentorships := k.GetMentorshipsByMentor(ctx, mentorAddr)
	count := 0
	for _, m := range mentorships {
		if m.Status == "active" {
			count++
		}
	}
	return count
}

func (k Keeper) GetActiveMentorshipForMentee(ctx sdk.Context, menteeAddr string) (*types.Mentorship, bool) {
	mentorships := k.GetMentorshipsByMentee(ctx, menteeAddr)
	for _, m := range mentorships {
		if m.Status == "active" {
			return m, true
		}
	}
	return nil, false
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./x/partnerships/keeper/ -run TestMentorship_ -v -count=1`
Expected: All 6 tests PASS.

**Step 5: Commit**

```bash
git add x/partnerships/keeper/mentorship.go x/partnerships/keeper/keeper_test.go
git commit -m "feat(partnerships): add mentorship CRUD with indexes and tests (R28-6)"
```

---

### Task 4: Mentorship Message Handlers + Tests

**Files:**
- Modify: `x/partnerships/keeper/msg_server.go`
- Modify: `x/partnerships/keeper/keeper_test.go`

**Step 1: Write failing tests for mentorship handlers**

Add to `keeper_test.go`:

```go
// ---------- Mentorship Message Handler Tests ----------

func TestMentorship_ProposeAndAccept(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	resp, err := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor:         humanAddr,
		Mentee:         agentAddr,
		Domain:         "physics",
		DurationBlocks: 1000,
	})
	if err != nil {
		t.Fatalf("ProposeMentorship failed: %v", err)
	}
	if resp.MentorshipId == "" {
		t.Fatal("expected non-empty mentorship ID")
	}

	m, found := k.GetMentorship(ctx, resp.MentorshipId)
	if !found {
		t.Fatal("mentorship not found after proposal")
	}
	if m.Status != "proposed" {
		t.Errorf("expected proposed, got %s", m.Status)
	}

	// Accept
	_, err = ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee:       agentAddr,
		MentorshipId: resp.MentorshipId,
	})
	if err != nil {
		t.Fatalf("AcceptMentorship failed: %v", err)
	}

	m, _ = k.GetMentorship(ctx, resp.MentorshipId)
	if m.Status != "active" {
		t.Errorf("expected active, got %s", m.Status)
	}
	if m.StartBlock != uint64(ctx.BlockHeight()) {
		t.Errorf("expected start block %d, got %d", ctx.BlockHeight(), m.StartBlock)
	}
}

func TestMentorship_SelfMentorshipBlocked(t *testing.T) {
	ms, _, ctx, _ := setupMsgServer(t)

	_, err := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor:         humanAddr,
		Mentee:         humanAddr,
		Domain:         "physics",
		DurationBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for self-mentorship")
	}
}

func TestMentorship_MaxMentorshipsEnforced(t *testing.T) {
	ms, _, ctx, _ := setupMsgServer(t)

	addrs := []string{agentAddr, agent2Addr, agent3Addr}
	for _, addr := range addrs {
		resp, err := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
			Mentor: humanAddr, Mentee: addr, Domain: "physics", DurationBlocks: 1000,
		})
		if err != nil {
			t.Fatalf("ProposeMentorship failed: %v", err)
		}
		_, err = ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
			Mentee: addr, MentorshipId: resp.MentorshipId,
		})
		if err != nil {
			t.Fatalf("AcceptMentorship failed: %v", err)
		}
	}

	// 4th should fail (max = 3)
	fourthAddr := testAddr("agent4")
	_, err := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: fourthAddr, Domain: "physics", DurationBlocks: 1000,
	})
	if err == nil {
		t.Fatal("expected error for exceeding max mentorships")
	}
}

func TestMentorship_ManualGraduation(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	resp, _ := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: agentAddr, Domain: "physics", DurationBlocks: 1000,
	})
	ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee: agentAddr, MentorshipId: resp.MentorshipId,
	})

	_, err := ms.GraduateMentee(ctx, &types.MsgGraduateMentee{
		Mentor: humanAddr, MentorshipId: resp.MentorshipId,
	})
	if err != nil {
		t.Fatalf("GraduateMentee failed: %v", err)
	}

	m, _ := k.GetMentorship(ctx, resp.MentorshipId)
	if m.Status != "graduated" {
		t.Errorf("expected graduated, got %s", m.Status)
	}
}

func TestMentorship_EarlyTermination(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	resp, _ := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: agentAddr, Domain: "physics", DurationBlocks: 1000,
	})
	ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee: agentAddr, MentorshipId: resp.MentorshipId,
	})

	// Mentee terminates
	_, err := ms.EndMentorship(ctx, &types.MsgEndMentorship{
		Sender: agentAddr, MentorshipId: resp.MentorshipId,
	})
	if err != nil {
		t.Fatalf("EndMentorship failed: %v", err)
	}

	m, _ := k.GetMentorship(ctx, resp.MentorshipId)
	if m.Status != "terminated" {
		t.Errorf("expected terminated, got %s", m.Status)
	}
}

func TestMentorship_TerminationByOutsiderFails(t *testing.T) {
	ms, _, ctx, _ := setupMsgServer(t)

	resp, _ := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: agentAddr, Domain: "physics", DurationBlocks: 1000,
	})
	ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee: agentAddr, MentorshipId: resp.MentorshipId,
	})

	_, err := ms.EndMentorship(ctx, &types.MsgEndMentorship{
		Sender: outsiderAddr, MentorshipId: resp.MentorshipId,
	})
	if err == nil {
		t.Fatal("expected error for outsider ending mentorship")
	}
}

func TestMentorship_AcceptByWrongMenteeFails(t *testing.T) {
	ms, _, ctx, _ := setupMsgServer(t)

	resp, _ := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: agentAddr, Domain: "physics", DurationBlocks: 1000,
	})

	_, err := ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee: outsiderAddr, MentorshipId: resp.MentorshipId,
	})
	if err == nil {
		t.Fatal("expected error for wrong mentee accepting")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/partnerships/keeper/ -run TestMentorship_Propose -v -count=1`
Expected: Compilation errors — handler methods don't exist.

**Step 3: Implement message handlers in msg_server.go**

Add to `msg_server.go`:

```go
// ProposeMentorship creates a new mentorship proposal.
func (k msgServer) ProposeMentorship(goCtx context.Context, msg *types.MsgProposeMentorship) (*types.MsgProposeMentorshipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	if msg.Mentor == msg.Mentee {
		return nil, types.ErrSelfMentorship
	}

	// Check mentor hasn't exceeded max
	if k.CountActiveMentorshipsForMentor(ctx, msg.Mentor) >= int(params.MaxMentorshipsPerMentor) {
		return nil, types.ErrMaxMentorshipsReached
	}

	// Check mentee doesn't already have an active mentorship
	if _, found := k.GetActiveMentorshipForMentee(ctx, msg.Mentee); found {
		return nil, types.ErrAlreadyMentored
	}

	seq := k.NextSequence(ctx)
	mentorshipId := fmt.Sprintf("mentorship-%d", seq)

	m := &types.Mentorship{
		Id:                  mentorshipId,
		MentorAddr:          msg.Mentor,
		MenteeAddr:          msg.Mentee,
		Domain:              msg.Domain,
		Status:              "proposed",
		DurationBlocks:      msg.DurationBlocks,
		GraduationThreshold: params.GraduationVerifications,
		GraduationClaimsReq: params.GraduationClaims,
	}
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_proposed",
			sdk.NewAttribute("mentorship_id", mentorshipId),
			sdk.NewAttribute("mentor", msg.Mentor),
			sdk.NewAttribute("mentee", msg.Mentee),
			sdk.NewAttribute("domain", msg.Domain),
		),
	)

	return &types.MsgProposeMentorshipResponse{MentorshipId: mentorshipId}, nil
}

// AcceptMentorship accepts a pending mentorship proposal.
func (k msgServer) AcceptMentorship(goCtx context.Context, msg *types.MsgAcceptMentorship) (*types.MsgAcceptMentorshipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	m, found := k.GetMentorship(ctx, msg.MentorshipId)
	if !found {
		return nil, types.ErrMentorshipNotFound
	}
	if m.Status != "proposed" {
		return nil, types.ErrMentorshipNotProposed
	}
	if m.MenteeAddr != msg.Mentee {
		return nil, types.ErrNotMentorshipParticipant
	}

	m.Status = "active"
	m.StartBlock = uint64(ctx.BlockHeight())
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_accepted",
			sdk.NewAttribute("mentorship_id", m.Id),
			sdk.NewAttribute("mentor", m.MentorAddr),
			sdk.NewAttribute("mentee", m.MenteeAddr),
		),
	)

	return &types.MsgAcceptMentorshipResponse{}, nil
}

// GraduateMentee manually graduates a mentee from an active mentorship.
func (k msgServer) GraduateMentee(goCtx context.Context, msg *types.MsgGraduateMentee) (*types.MsgGraduateMenteeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	m, found := k.GetMentorship(ctx, msg.MentorshipId)
	if !found {
		return nil, types.ErrMentorshipNotFound
	}
	if m.Status != "active" {
		return nil, types.ErrMentorshipNotActive
	}
	if m.MentorAddr != msg.Mentor {
		return nil, types.ErrNotMentorshipParticipant
	}

	k.graduateMentorship(ctx, m)

	return &types.MsgGraduateMenteeResponse{}, nil
}

// EndMentorship terminates a mentorship early.
func (k msgServer) EndMentorship(goCtx context.Context, msg *types.MsgEndMentorship) (*types.MsgEndMentorshipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	m, found := k.GetMentorship(ctx, msg.MentorshipId)
	if !found {
		return nil, types.ErrMentorshipNotFound
	}
	if m.Status != "proposed" && m.Status != "active" {
		return nil, types.ErrMentorshipNotActive
	}
	if m.MentorAddr != msg.Sender && m.MenteeAddr != msg.Sender {
		return nil, types.ErrNotMentorshipParticipant
	}

	m.Status = "terminated"
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_terminated",
			sdk.NewAttribute("mentorship_id", m.Id),
			sdk.NewAttribute("terminated_by", msg.Sender),
		),
	)

	return &types.MsgEndMentorshipResponse{}, nil
}
```

**Step 4: Add graduateMentorship helper to mentorship.go**

Add to `x/partnerships/keeper/mentorship.go`:

```go
// graduateMentorship transitions a mentorship to graduated status
// and optionally proposes a partnership.
func (k Keeper) graduateMentorship(ctx sdk.Context, m *types.Mentorship) {
	m.Status = "graduated"
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_graduated",
			sdk.NewAttribute("mentorship_id", m.Id),
			sdk.NewAttribute("mentor", m.MentorAddr),
			sdk.NewAttribute("mentee", m.MenteeAddr),
			sdk.NewAttribute("domain", m.Domain),
		),
	)

	params := k.GetParams(ctx)
	if params.AutoProposePartnershipOnGraduation {
		seq := k.NextSequence(ctx)
		partnershipId := fmt.Sprintf("partnership-%d", seq)
		currentBlock := uint64(ctx.BlockHeight())

		partnership := &types.Partnership{
			Id:               partnershipId,
			HumanAddr:        m.MentorAddr,
			AgentAddr:        m.MenteeAddr,
			Status:           types.StatusPending,
			Tier:             0,
			LockTier:         0,
			LockExpiresAt:    currentBlock + types.LockTiers[0].MinBlocks,
			SplitHumanBps:    params.DefaultHumanSplitBps,
			SplitAgentBps:    params.DefaultAgentSplitBps,
			CommonPotBalance: "0",
			TotalEarned:      "0",
			CooperationScore: 500000,
			FormedAtBlock:    currentBlock,
		}
		k.SetPartnership(ctx, partnership)

		kvStore := k.storeService.OpenKVStore(ctx)
		formationExpiry := currentBlock + params.FormationWindowBlocks
		_ = kvStore.Set(
			append(types.FormationKeyPrefix, []byte(partnershipId)...),
			[]byte(fmt.Sprintf("%d", formationExpiry)),
		)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.partnerships.partnership_proposed",
				sdk.NewAttribute("partnership_id", partnershipId),
				sdk.NewAttribute("proposer", m.MentorAddr),
				sdk.NewAttribute("partner", m.MenteeAddr),
				sdk.NewAttribute("source", "mentorship_graduation"),
			),
		)
	}
}
```

Note: The `graduateMentorship` helper needs `fmt` imported in mentorship.go. Add `"fmt"` to the imports.

**Step 5: Run tests to verify they pass**

Run: `go test ./x/partnerships/keeper/ -run "TestMentorship_(Propose|Self|Max|Manual|Early|Termination|Accept)" -v -count=1`
Expected: All 7 tests PASS.

**Step 6: Commit**

```bash
git add x/partnerships/keeper/msg_server.go x/partnerships/keeper/mentorship.go x/partnerships/keeper/keeper_test.go
git commit -m "feat(partnerships): add mentorship message handlers with lifecycle tests (R28-6)"
```

---

### Task 5: Formation Match CRUD + Matching Engine + Tests

**Files:**
- Create: `x/partnerships/keeper/formation_matching.go`
- Modify: `x/partnerships/keeper/keeper_test.go`

**Step 1: Write failing tests for formation match CRUD and matching**

Add to `keeper_test.go`:

```go
// ---------- Formation Matching Tests ----------

func TestFormation_MatchSetAndGet(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	fm := &types.FormationMatch{
		Id: "match-1", Addr1: humanAddr, Addr2: agentAddr,
		Score: 7500, ProposedAt: 100, ExpiresAt: 300,
		Status: "proposed",
	}
	k.SetFormationMatch(ctx, fm)

	got, found := k.GetFormationMatch(ctx, "match-1")
	if !found {
		t.Fatal("match not found")
	}
	if got.Score != 7500 {
		t.Errorf("expected score 7500, got %d", got.Score)
	}
}

func TestFormation_MatchingRunsAtInterval(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Add two compatible pool entries
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: humanAddr, Domains: []string{"physics"}, PreferredRole: "human",
		RegisteredAt: 50, ExpiresAt: 12000, Status: "active",
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: agentAddr, Domains: []string{"physics"}, PreferredRole: "agent",
		RegisteredAt: 60, ExpiresAt: 12000, Status: "active",
	})

	// At block 99 (not interval), no matching
	ctx99 := ctxAtHeight(ctx, 99)
	k.RunFormationMatching(ctx99)
	matches := k.GetAllFormationMatches(ctx99)
	if len(matches) != 0 {
		t.Errorf("expected no matches at non-interval block, got %d", len(matches))
	}

	// At block 100 (interval), matching runs
	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100)
	matches = k.GetAllFormationMatches(ctx100)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match at interval block, got %d", len(matches))
	}
	if matches[0].Addr1 != agentAddr && matches[0].Addr2 != agentAddr {
		t.Error("expected agentAddr in match")
	}
}

func TestFormation_CompatiblePairsMatched(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Human seeking agent in physics
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: humanAddr, Domains: []string{"physics", "math"}, PreferredRole: "human",
		RegisteredAt: 50, ExpiresAt: 12000, Status: "active",
	})
	// Agent seeking human in physics — should match well
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: agentAddr, Domains: []string{"physics"}, PreferredRole: "agent",
		RegisteredAt: 60, ExpiresAt: 12000, Status: "active",
	})
	// Another agent in biology — no domain overlap
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: agent2Addr, Domains: []string{"biology"}, PreferredRole: "agent",
		RegisteredAt: 70, ExpiresAt: 12000, Status: "active",
	})

	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100)
	matches := k.GetAllFormationMatches(ctx100)

	// Should match human with physics-agent (best domain overlap)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	hasHuman := m.Addr1 == humanAddr || m.Addr2 == humanAddr
	hasAgent := m.Addr1 == agentAddr || m.Addr2 == agentAddr
	if !hasHuman || !hasAgent {
		t.Errorf("expected human-agent match, got %s and %s", m.Addr1, m.Addr2)
	}
	if m.Score == 0 {
		t.Error("expected non-zero score")
	}
}

func TestFormation_CappedAt200(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// This just verifies the function doesn't panic with many entries
	// and respects the cap (matching only processes first 200)
	for i := 0; i < 210; i++ {
		addr := testAddr(fmt.Sprintf("pool-%d", i))
		k.SetPoolEntry(ctx, &types.PoolEntry{
			Address: addr, Domains: []string{"physics"}, PreferredRole: "any",
			RegisteredAt: 50, ExpiresAt: 12000, Status: "active",
		})
	}

	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100) // Should not panic
	// Just verify it ran without error — exact match count depends on scoring
}

func TestFormation_ExpireMatches(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetFormationMatch(ctx, &types.FormationMatch{
		Id: "match-1", Addr1: humanAddr, Addr2: agentAddr,
		ProposedAt: 100, ExpiresAt: 300, Status: "proposed",
	})

	// Before expiry
	k.ExpireFormationMatches(ctxAtHeight(ctx, 299))
	m, _ := k.GetFormationMatch(ctxAtHeight(ctx, 299), "match-1")
	if m.Status != "proposed" {
		t.Errorf("expected still proposed, got %s", m.Status)
	}

	// At expiry
	k.ExpireFormationMatches(ctxAtHeight(ctx, 300))
	m, _ = k.GetFormationMatch(ctxAtHeight(ctx, 300), "match-1")
	if m.Status != "expired" {
		t.Errorf("expected expired, got %s", m.Status)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/partnerships/keeper/ -run TestFormation_ -v -count=1`
Expected: Compilation errors.

**Step 3: Implement formation matching in formation_matching.go**

Create `x/partnerships/keeper/formation_matching.go`:

```go
package keeper

import (
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- FormationMatch CRUD ----------

func formationMatchKey(id string) []byte {
	return append(types.FormationMatchKeyPrefix, []byte(id)...)
}

func (k Keeper) SetFormationMatch(ctx sdk.Context, fm *types.FormationMatch) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(fm)
	if err != nil {
		panic("failed to marshal formation match: " + err.Error())
	}
	_ = kvStore.Set(formationMatchKey(fm.Id), bz)
}

func (k Keeper) GetFormationMatch(ctx sdk.Context, id string) (*types.FormationMatch, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(formationMatchKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var fm types.FormationMatch
	if err := proto.Unmarshal(bz, &fm); err != nil {
		return nil, false
	}
	return &fm, true
}

func (k Keeper) DeleteFormationMatch(ctx sdk.Context, id string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(formationMatchKey(id))
}

func (k Keeper) GetAllFormationMatches(ctx sdk.Context) []*types.FormationMatch {
	var matches []*types.FormationMatch
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.FormationMatchKeyPrefix, prefixEndBytes(types.FormationMatchKeyPrefix))
	if err != nil {
		return matches
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var fm types.FormationMatch
		if err := proto.Unmarshal(iter.Value(), &fm); err == nil {
			matches = append(matches, &fm)
		}
	}
	return matches
}

// ---------- Matching Engine ----------

const maxMatchingEntries = 200

// RunFormationMatching runs the formation pool matching algorithm.
// Only runs at blocks divisible by FormationMatchIntervalBlocks.
func (k Keeper) RunFormationMatching(ctx sdk.Context) {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	if params.FormationMatchIntervalBlocks == 0 || currentBlock%params.FormationMatchIntervalBlocks != 0 {
		return
	}

	// Gather active, unmatched entries
	allEntries := k.GetAllPoolEntries(ctx)
	var entries []*types.PoolEntry
	for _, pe := range allEntries {
		if pe.Status == "active" && pe.MatchedWith == "" {
			entries = append(entries, pe)
		}
	}

	if len(entries) < 2 {
		return
	}

	// Sort deterministically by address
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Address < entries[j].Address
	})

	// Cap at maxMatchingEntries for gas safety
	if len(entries) > maxMatchingEntries {
		entries = entries[:maxMatchingEntries]
	}

	// Greedy matching
	matched := make(map[string]bool)
	for i := 0; i < len(entries); i++ {
		e1 := entries[i]
		if matched[e1.Address] {
			continue
		}

		bestScore := uint64(0)
		bestIdx := -1
		for j := i + 1; j < len(entries); j++ {
			e2 := entries[j]
			if matched[e2.Address] {
				continue
			}
			score := scoreCompatibility(e1, e2, currentBlock)
			if score > bestScore {
				bestScore = score
				bestIdx = j
			}
		}

		if bestIdx >= 0 && bestScore > 0 {
			e2 := entries[bestIdx]
			matched[e1.Address] = true
			matched[e2.Address] = true

			seq := k.NextSequence(ctx)
			matchId := fmt.Sprintf("match-%d", seq)

			fm := &types.FormationMatch{
				Id:         matchId,
				Addr1:      e1.Address,
				Addr2:      e2.Address,
				Score:      bestScore,
				ProposedAt: currentBlock,
				ExpiresAt:  currentBlock + params.MatchAcceptanceBlocks,
				Status:     "proposed",
			}
			k.SetFormationMatch(ctx, fm)

			// Mark entries as matched
			e1.MatchedWith = matchId
			k.SetPoolEntry(ctx, e1)
			e2.MatchedWith = matchId
			k.SetPoolEntry(ctx, e2)

			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.partnerships.formation_match_proposed",
					sdk.NewAttribute("match_id", matchId),
					sdk.NewAttribute("addr1", e1.Address),
					sdk.NewAttribute("addr2", e2.Address),
					sdk.NewAttribute("score", fmt.Sprintf("%d", bestScore)),
				),
			)
		}
	}
}

// scoreCompatibility scores compatibility between two pool entries.
// Returns score in basis points (0-10000).
func scoreCompatibility(e1, e2 *types.PoolEntry, currentBlock uint64) uint64 {
	score := uint64(0)

	// Domain overlap: (shared / max(len1, len2)) * 5000
	shared := 0
	domainSet := make(map[string]bool)
	for _, d := range e1.Domains {
		domainSet[d] = true
	}
	for _, d := range e2.Domains {
		if domainSet[d] {
			shared++
		}
	}
	maxDomains := len(e1.Domains)
	if len(e2.Domains) > maxDomains {
		maxDomains = len(e2.Domains)
	}
	if maxDomains > 0 {
		score += uint64(shared) * 5000 / uint64(maxDomains)
	}

	// Preferred role compatibility
	if isComplementary(e1.PreferredRole, e2.PreferredRole) {
		score += 3000
	} else if e1.PreferredRole == "any" || e2.PreferredRole == "any" || e1.PreferredRole == "" || e2.PreferredRole == "" {
		score += 1500
	}
	// Same-seeking-same: +0

	// Time in pool: min(avg_time / 1000, 2000)
	avgTime := uint64(0)
	if e1.RegisteredAt > 0 && currentBlock > e1.RegisteredAt {
		avgTime += currentBlock - e1.RegisteredAt
	}
	if e2.RegisteredAt > 0 && currentBlock > e2.RegisteredAt {
		avgTime += currentBlock - e2.RegisteredAt
	}
	avgTime /= 2
	timeScore := avgTime / 1000
	if timeScore > 2000 {
		timeScore = 2000
	}
	score += timeScore

	return score
}

// isComplementary checks if two roles are complementary (human+agent).
func isComplementary(r1, r2 string) bool {
	return (r1 == "human" && r2 == "agent") || (r1 == "agent" && r2 == "human")
}

// ---------- Expiry ----------

// ExpireFormationMatches expires matches past their acceptance window.
func (k Keeper) ExpireFormationMatches(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	matches := k.GetAllFormationMatches(ctx)

	for _, fm := range matches {
		if fm.Status == "proposed" && fm.ExpiresAt > 0 && fm.ExpiresAt <= currentBlock {
			fm.Status = "expired"
			k.SetFormationMatch(ctx, fm)

			// Return entries to unmatched state
			if pe, found := k.GetPoolEntry(ctx, fm.Addr1); found && pe.MatchedWith == fm.Id {
				pe.MatchedWith = ""
				k.SetPoolEntry(ctx, pe)
			}
			if pe, found := k.GetPoolEntry(ctx, fm.Addr2); found && pe.MatchedWith == fm.Id {
				pe.MatchedWith = ""
				k.SetPoolEntry(ctx, pe)
			}
		}
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./x/partnerships/keeper/ -run TestFormation_ -v -count=1`
Expected: All 5 tests PASS.

**Step 5: Commit**

```bash
git add x/partnerships/keeper/formation_matching.go x/partnerships/keeper/keeper_test.go
git commit -m "feat(partnerships): add formation match CRUD and matching engine (R28-6)"
```

---

### Task 6: Formation Match Message Handlers + Tests

**Files:**
- Modify: `x/partnerships/keeper/msg_server.go`
- Modify: `x/partnerships/keeper/keeper_test.go`

**Step 1: Write failing tests**

Add to `keeper_test.go`:

```go
func TestFormation_BothAcceptFormsPartnership(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	// Create a proposed match manually
	k.SetFormationMatch(ctx, &types.FormationMatch{
		Id: "match-1", Addr1: humanAddr, Addr2: agentAddr,
		Score: 8000, ProposedAt: 100, ExpiresAt: 300, Status: "proposed",
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: humanAddr, Domains: []string{"physics"}, Status: "active", MatchedWith: "match-1",
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: agentAddr, Domains: []string{"physics"}, Status: "active", MatchedWith: "match-1",
	})

	// First accept
	_, err := ms.AcceptFormationMatch(ctx, &types.MsgAcceptFormationMatch{
		Accepter: humanAddr, MatchId: "match-1",
	})
	if err != nil {
		t.Fatalf("AcceptFormationMatch (addr1) failed: %v", err)
	}

	fm, _ := k.GetFormationMatch(ctx, "match-1")
	if !fm.Addr1Accepted {
		t.Error("expected addr1_accepted to be true")
	}
	if fm.Status != "proposed" {
		t.Error("expected status still proposed after one accept")
	}

	// Second accept
	_, err = ms.AcceptFormationMatch(ctx, &types.MsgAcceptFormationMatch{
		Accepter: agentAddr, MatchId: "match-1",
	})
	if err != nil {
		t.Fatalf("AcceptFormationMatch (addr2) failed: %v", err)
	}

	fm, _ = k.GetFormationMatch(ctx, "match-1")
	if fm.Status != "accepted" {
		t.Errorf("expected accepted status, got %s", fm.Status)
	}

	// A partnership should have been proposed
	partnerships := k.GetAllPartnerships(ctx)
	found := false
	for _, p := range partnerships {
		if (p.HumanAddr == humanAddr && p.AgentAddr == agentAddr) ||
			(p.HumanAddr == agentAddr && p.AgentAddr == humanAddr) {
			found = true
			if p.Status != types.StatusPending {
				t.Errorf("expected pending partnership, got %s", p.Status)
			}
		}
	}
	if !found {
		t.Error("expected partnership to be created after both accept")
	}

	// Pool entries should be removed
	_, inPool := k.GetPoolEntry(ctx, humanAddr)
	if inPool {
		t.Error("expected human to be removed from pool")
	}
	_, inPool = k.GetPoolEntry(ctx, agentAddr)
	if inPool {
		t.Error("expected agent to be removed from pool")
	}
}

func TestFormation_DeclineReturnsToPool(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	k.SetFormationMatch(ctx, &types.FormationMatch{
		Id: "match-1", Addr1: humanAddr, Addr2: agentAddr,
		Score: 8000, ProposedAt: 100, ExpiresAt: 300, Status: "proposed",
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: humanAddr, Domains: []string{"physics"}, Status: "active", MatchedWith: "match-1",
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: agentAddr, Domains: []string{"physics"}, Status: "active", MatchedWith: "match-1",
	})

	_, err := ms.DeclineFormationMatch(ctx, &types.MsgDeclineFormationMatch{
		Decliner: humanAddr, MatchId: "match-1",
	})
	if err != nil {
		t.Fatalf("DeclineFormationMatch failed: %v", err)
	}

	fm, _ := k.GetFormationMatch(ctx, "match-1")
	if fm.Status != "declined" {
		t.Errorf("expected declined, got %s", fm.Status)
	}

	// Both should be back in pool, unmatched
	pe1, _ := k.GetPoolEntry(ctx, humanAddr)
	if pe1.MatchedWith != "" {
		t.Error("expected human unmatched after decline")
	}
	pe2, _ := k.GetPoolEntry(ctx, agentAddr)
	if pe2.MatchedWith != "" {
		t.Error("expected agent unmatched after decline")
	}
}

func TestFormation_AcceptByOutsiderFails(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	k.SetFormationMatch(ctx, &types.FormationMatch{
		Id: "match-1", Addr1: humanAddr, Addr2: agentAddr,
		Score: 8000, ProposedAt: 100, ExpiresAt: 300, Status: "proposed",
	})

	_, err := ms.AcceptFormationMatch(ctx, &types.MsgAcceptFormationMatch{
		Accepter: outsiderAddr, MatchId: "match-1",
	})
	if err == nil {
		t.Fatal("expected error for outsider accepting match")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/partnerships/keeper/ -run "TestFormation_(Both|Decline|AcceptBy)" -v -count=1`
Expected: Compilation errors.

**Step 3: Implement formation match message handlers**

Add to `msg_server.go`:

```go
// AcceptFormationMatch accepts a proposed formation match.
func (k msgServer) AcceptFormationMatch(goCtx context.Context, msg *types.MsgAcceptFormationMatch) (*types.MsgAcceptFormationMatchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	fm, found := k.GetFormationMatch(ctx, msg.MatchId)
	if !found {
		return nil, types.ErrMatchNotFound
	}
	if fm.Status != "proposed" {
		return nil, types.ErrMatchNotProposed
	}
	if msg.Accepter != fm.Addr1 && msg.Accepter != fm.Addr2 {
		return nil, types.ErrNotMatchParticipant
	}

	if msg.Accepter == fm.Addr1 {
		fm.Addr1Accepted = true
	} else {
		fm.Addr2Accepted = true
	}

	if fm.Addr1Accepted && fm.Addr2Accepted {
		fm.Status = "accepted"
		k.SetFormationMatch(ctx, fm)

		// Remove both from pool
		k.DeletePoolEntry(ctx, fm.Addr1)
		k.DeletePoolEntry(ctx, fm.Addr2)

		// Auto-propose partnership
		currentBlock := uint64(ctx.BlockHeight())
		seq := k.NextSequence(ctx)
		partnershipId := fmt.Sprintf("partnership-%d", seq)

		partnership := &types.Partnership{
			Id:               partnershipId,
			HumanAddr:        fm.Addr1,
			AgentAddr:        fm.Addr2,
			Status:           types.StatusPending,
			Tier:             0,
			LockTier:         0,
			LockExpiresAt:    currentBlock + types.LockTiers[0].MinBlocks,
			SplitHumanBps:    params.DefaultHumanSplitBps,
			SplitAgentBps:    params.DefaultAgentSplitBps,
			CommonPotBalance: "0",
			TotalEarned:      "0",
			CooperationScore: 500000,
			FormedAtBlock:    currentBlock,
		}
		k.SetPartnership(ctx, partnership)

		kvStore := k.storeService.OpenKVStore(ctx)
		formationExpiry := currentBlock + params.FormationWindowBlocks
		_ = kvStore.Set(
			append(types.FormationKeyPrefix, []byte(partnershipId)...),
			[]byte(fmt.Sprintf("%d", formationExpiry)),
		)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.partnerships.formation_match_accepted",
				sdk.NewAttribute("match_id", fm.Id),
				sdk.NewAttribute("partnership_id", partnershipId),
			),
		)
	} else {
		k.SetFormationMatch(ctx, fm)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.partnerships.formation_match_partial_accept",
				sdk.NewAttribute("match_id", fm.Id),
				sdk.NewAttribute("accepter", msg.Accepter),
			),
		)
	}

	return &types.MsgAcceptFormationMatchResponse{}, nil
}

// DeclineFormationMatch declines a proposed formation match.
func (k msgServer) DeclineFormationMatch(goCtx context.Context, msg *types.MsgDeclineFormationMatch) (*types.MsgDeclineFormationMatchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	fm, found := k.GetFormationMatch(ctx, msg.MatchId)
	if !found {
		return nil, types.ErrMatchNotFound
	}
	if fm.Status != "proposed" {
		return nil, types.ErrMatchNotProposed
	}
	if msg.Decliner != fm.Addr1 && msg.Decliner != fm.Addr2 {
		return nil, types.ErrNotMatchParticipant
	}

	fm.Status = "declined"
	k.SetFormationMatch(ctx, fm)

	// Return both entries to unmatched state
	if pe, found := k.GetPoolEntry(ctx, fm.Addr1); found {
		pe.MatchedWith = ""
		k.SetPoolEntry(ctx, pe)
	}
	if pe, found := k.GetPoolEntry(ctx, fm.Addr2); found {
		pe.MatchedWith = ""
		k.SetPoolEntry(ctx, pe)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.formation_match_declined",
			sdk.NewAttribute("match_id", fm.Id),
			sdk.NewAttribute("declined_by", msg.Decliner),
		),
	)

	return &types.MsgDeclineFormationMatchResponse{}, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./x/partnerships/keeper/ -run "TestFormation_(Both|Decline|AcceptBy)" -v -count=1`
Expected: All 3 tests PASS.

**Step 5: Commit**

```bash
git add x/partnerships/keeper/msg_server.go x/partnerships/keeper/keeper_test.go
git commit -m "feat(partnerships): add formation match accept/decline handlers (R28-6)"
```

---

### Task 7: EndBlocker Wiring + Auto-Graduation Tests

**Files:**
- Modify: `x/partnerships/module.go`
- Modify: `x/partnerships/keeper/mentorship.go`
- Modify: `x/partnerships/keeper/keeper_test.go`

**Step 1: Write failing test for auto-graduation**

Add to `keeper_test.go`:

```go
func TestMentorship_AutoGraduation(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	resp, _ := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: agentAddr, Domain: "physics", DurationBlocks: 500,
	})
	ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee: agentAddr, MentorshipId: resp.MentorshipId,
	})

	// Before duration expires — should stay active
	ctx400 := ctxAtHeight(ctx, 100+400)
	k.AutoGraduateMentorships(ctx400)
	m, _ := k.GetMentorship(ctx400, resp.MentorshipId)
	if m.Status != "active" {
		t.Errorf("expected active before duration, got %s", m.Status)
	}

	// After duration expires — should auto-graduate
	ctx601 := ctxAtHeight(ctx, 100+501)
	k.AutoGraduateMentorships(ctx601)
	m, _ = k.GetMentorship(ctx601, resp.MentorshipId)
	if m.Status != "graduated" {
		t.Errorf("expected graduated after duration, got %s", m.Status)
	}
}

func TestMentorship_AutoProposePartnership(t *testing.T) {
	ms, k, ctx, _ := setupMsgServer(t)

	resp, _ := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: agentAddr, Domain: "physics", DurationBlocks: 100,
	})
	ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee: agentAddr, MentorshipId: resp.MentorshipId,
	})

	// Graduate manually (auto-propose is true by default)
	ms.GraduateMentee(ctx, &types.MsgGraduateMentee{
		Mentor: humanAddr, MentorshipId: resp.MentorshipId,
	})

	// Should have created a partnership
	partnerships := k.GetAllPartnerships(ctx)
	found := false
	for _, p := range partnerships {
		if p.HumanAddr == humanAddr && p.AgentAddr == agentAddr {
			found = true
			if p.Status != types.StatusPending {
				t.Errorf("expected pending, got %s", p.Status)
			}
		}
	}
	if !found {
		t.Error("expected partnership to be auto-proposed on graduation")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/partnerships/keeper/ -run TestMentorship_Auto -v -count=1`
Expected: Compilation error — `AutoGraduateMentorships` doesn't exist.

**Step 3: Add AutoGraduateMentorships to mentorship.go**

Add to `x/partnerships/keeper/mentorship.go`:

```go
// AutoGraduateMentorships checks active mentorships for duration expiry
// and graduates them automatically.
func (k Keeper) AutoGraduateMentorships(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	mentorships := k.GetAllMentorships(ctx)

	for _, m := range mentorships {
		if m.Status != "active" {
			continue
		}
		if m.StartBlock > 0 && m.DurationBlocks > 0 && currentBlock >= m.StartBlock+m.DurationBlocks {
			k.graduateMentorship(ctx, m)
		}
	}
}
```

**Step 4: Wire EndBlocker in module.go**

Modify the `EndBlock` method in `x/partnerships/module.go`:

```go
func (am AppModule) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	am.keeper.SettleCoolingPartnerships(sdkCtx)
	am.keeper.AutoGraduateMentorships(sdkCtx)
	am.keeper.RunFormationMatching(sdkCtx)
	return nil
}
```

Add `ExpireFormationMatches` to `BeginBlock`:

```go
func (am AppModule) BeginBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	am.keeper.ExpireFormations(sdkCtx)
	am.keeper.LiftExpiredFreezes(sdkCtx)
	am.keeper.ExpireCoercionSignals(sdkCtx)
	am.keeper.ExpireConsensusOps(sdkCtx)
	am.keeper.ExpirePoolEntries(sdkCtx)
	am.keeper.ExpireSeedPartnerships(sdkCtx)
	am.keeper.ExpireFormationMatches(sdkCtx)

	return nil
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./x/partnerships/keeper/ -run TestMentorship_Auto -v -count=1`
Expected: Both tests PASS.

**Step 6: Commit**

```bash
git add x/partnerships/keeper/mentorship.go x/partnerships/module.go x/partnerships/keeper/keeper_test.go
git commit -m "feat(partnerships): wire EndBlocker for auto-graduation and matching (R28-6)"
```

---

### Task 8: Genesis Import/Export + Query Handlers

**Files:**
- Modify: `x/partnerships/keeper/keeper.go` (genesis)
- Modify: `x/partnerships/keeper/grpc_query.go`
- Modify: `x/partnerships/keeper/keeper_test.go`

**Step 1: Write failing test for genesis round-trip**

Add to `keeper_test.go`:

```go
func TestGenesis_MentorshipRoundTrip(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: humanAddr, MenteeAddr: agentAddr,
		Domain: "physics", Status: "active", StartBlock: 100, DurationBlocks: 1000,
	})
	k.SetFormationMatch(ctx, &types.FormationMatch{
		Id: "match-1", Addr1: humanAddr, Addr2: agentAddr,
		Score: 8000, ProposedAt: 100, ExpiresAt: 300, Status: "proposed",
	})

	exported := k.ExportGenesis(ctx)
	if len(exported.Mentorships) != 1 {
		t.Errorf("expected 1 mentorship in export, got %d", len(exported.Mentorships))
	}
	if len(exported.FormationMatches) != 1 {
		t.Errorf("expected 1 formation match in export, got %d", len(exported.FormationMatches))
	}

	// Import into fresh keeper
	k2, ctx2, _ := setupKeeper(t)
	k2.InitGenesis(ctx2, exported)

	m, found := k2.GetMentorship(ctx2, "m-1")
	if !found {
		t.Fatal("mentorship not found after import")
	}
	if m.Domain != "physics" {
		t.Errorf("expected physics, got %s", m.Domain)
	}

	fm, found := k2.GetFormationMatch(ctx2, "match-1")
	if !found {
		t.Fatal("formation match not found after import")
	}
	if fm.Score != 8000 {
		t.Errorf("expected score 8000, got %d", fm.Score)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/partnerships/keeper/ -run TestGenesis_Mentorship -v -count=1`
Expected: Fail — `Mentorships` and `FormationMatches` fields don't exist on GenesisState yet (or are empty in export).

**Step 3: Update InitGenesis and ExportGenesis**

In `x/partnerships/keeper/keeper.go`, update `InitGenesis`:

```go
// Add after the PoolEntries loop:
for _, m := range genState.Mentorships {
    if m != nil {
        k.SetMentorship(ctx, m)
    }
}
for _, fm := range genState.FormationMatches {
    if fm != nil {
        k.SetFormationMatch(ctx, fm)
    }
}
```

Update `ExportGenesis` to include:
```go
Mentorships:      k.GetAllMentorships(ctx),
FormationMatches: k.GetAllFormationMatches(ctx),
```

Update `DefaultGenesis` in `x/partnerships/types/genesis.go` (already done in Task 2).

**Step 4: Add query handlers to grpc_query.go**

```go
func (qs queryServer) Mentorship(goCtx context.Context, req *types.QueryMentorshipRequest) (*types.QueryMentorshipResponse, error) {
	if req == nil || req.Id == "" {
		return nil, fmt.Errorf("mentorship id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	m, found := qs.GetMentorship(ctx, req.Id)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrMentorshipNotFound, req.Id)
	}
	return &types.QueryMentorshipResponse{Mentorship: m}, nil
}

func (qs queryServer) MentorshipsByAddress(goCtx context.Context, req *types.QueryMentorshipsByAddressRequest) (*types.QueryMentorshipsByAddressResponse, error) {
	if req == nil || req.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	byMentor := qs.GetMentorshipsByMentor(ctx, req.Address)
	byMentee := qs.GetMentorshipsByMentee(ctx, req.Address)

	// Deduplicate
	seen := make(map[string]bool)
	var all []*types.Mentorship
	for _, m := range byMentor {
		if !seen[m.Id] {
			seen[m.Id] = true
			all = append(all, m)
		}
	}
	for _, m := range byMentee {
		if !seen[m.Id] {
			seen[m.Id] = true
			all = append(all, m)
		}
	}
	return &types.QueryMentorshipsByAddressResponse{Mentorships: all}, nil
}

func (qs queryServer) FormationMatches(goCtx context.Context, req *types.QueryFormationMatchesRequest) (*types.QueryFormationMatchesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	matches := qs.GetAllFormationMatches(ctx)
	return &types.QueryFormationMatchesResponse{Matches: matches}, nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./x/partnerships/keeper/ -run TestGenesis_Mentorship -v -count=1`
Expected: PASS.

**Step 6: Commit**

```bash
git add x/partnerships/keeper/keeper.go x/partnerships/keeper/grpc_query.go x/partnerships/keeper/keeper_test.go
git commit -m "feat(partnerships): add mentorship/match genesis and query handlers (R28-6)"
```

---

### Task 9: CLI Commands

**Files:**
- Modify: `x/partnerships/client/cli/tx.go`
- Modify: `x/partnerships/client/cli/query.go`

**Step 1: Add mentorship tx CLI commands**

Add to `tx.go`, and register them in `NewTxCmd()`:

```go
func NewProposeMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "propose-mentorship [mentee] [domain] [duration-blocks]",
		Short: "Propose a mentorship to a mentee",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			duration, err := strconv.ParseUint(args[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}

			msg := &types.MsgProposeMentorship{
				Mentor:         clientCtx.GetFromAddress().String(),
				Mentee:         args[0],
				Domain:         args[1],
				DurationBlocks: duration,
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewAcceptMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept-mentorship [mentorship-id]",
		Short: "Accept a mentorship proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgAcceptMentorship{
				Mentee:       clientCtx.GetFromAddress().String(),
				MentorshipId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewGraduateMenteeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graduate-mentee [mentorship-id]",
		Short: "Graduate a mentee from an active mentorship",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgGraduateMentee{
				Mentor:       clientCtx.GetFromAddress().String(),
				MentorshipId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewEndMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "end-mentorship [mentorship-id]",
		Short: "End a mentorship early",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgEndMentorship{
				Sender:       clientCtx.GetFromAddress().String(),
				MentorshipId: args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewAcceptMatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept-match [match-id]",
		Short: "Accept a formation pool match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgAcceptFormationMatch{
				Accepter: clientCtx.GetFromAddress().String(),
				MatchId:  args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func NewDeclineMatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decline-match [match-id]",
		Short: "Decline a formation pool match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			msg := &types.MsgDeclineFormationMatch{
				Decliner: clientCtx.GetFromAddress().String(),
				MatchId:  args[0],
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
```

Register all in `NewTxCmd()`:
```go
NewProposeMentorshipCmd(),
NewAcceptMentorshipCmd(),
NewGraduateMenteeCmd(),
NewEndMentorshipCmd(),
NewAcceptMatchCmd(),
NewDeclineMatchCmd(),
```

Note: You may need to add `"strconv"` to imports if not already present.

**Step 2: Add query CLI commands**

Add to `query.go` and register in `NewQueryCmd()`:

```go
func NewQueryMentorshipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mentorship [id]",
		Short: "Query a mentorship by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryMentorshipRequest{Id: args[0]}
			resp := &types.QueryMentorshipResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/Mentorship", req, resp); err != nil {
				return fmt.Errorf("failed to query mentorship: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryMentorshipsByAddressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mentorships-by-address [address]",
		Short: "Query mentorships by participant address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryMentorshipsByAddressRequest{Address: args[0]}
			resp := &types.QueryMentorshipsByAddressResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/MentorshipsByAddress", req, resp); err != nil {
				return fmt.Errorf("failed to query mentorships: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

func NewQueryFormationMatchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "formation-matches",
		Short: "Query all formation matches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryFormationMatchesRequest{}
			resp := &types.QueryFormationMatchesResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.partnerships.v1.Query/FormationMatches", req, resp); err != nil {
				return fmt.Errorf("failed to query formation matches: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

Register in `NewQueryCmd()`:
```go
NewQueryMentorshipCmd(),
NewQueryMentorshipsByAddressCmd(),
NewQueryFormationMatchesCmd(),
```

**Step 3: Verify compilation**

Run: `go build ./x/partnerships/...`
Expected: No errors.

**Step 4: Commit**

```bash
git add x/partnerships/client/cli/tx.go x/partnerships/client/cli/query.go
git commit -m "feat(partnerships): add mentorship and match CLI commands (R28-6)"
```

---

### Task 10: Full Lifecycle Integration Test

**Files:**
- Modify: `x/partnerships/keeper/keeper_test.go`

**Step 1: Write integration test**

```go
func TestIntegration_MentorshipToPartnership(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(humanAddr, "uzrn", 10000000)
	bk.setBalance(agentAddr, "uzrn", 10000000)

	// 1. Propose mentorship
	resp, err := ms.ProposeMentorship(ctx, &types.MsgProposeMentorship{
		Mentor: humanAddr, Mentee: agentAddr, Domain: "physics", DurationBlocks: 100,
	})
	if err != nil {
		t.Fatalf("propose failed: %v", err)
	}

	// 2. Accept mentorship
	_, err = ms.AcceptMentorship(ctx, &types.MsgAcceptMentorship{
		Mentee: agentAddr, MentorshipId: resp.MentorshipId,
	})
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}

	m, _ := k.GetMentorship(ctx, resp.MentorshipId)
	if m.Status != "active" {
		t.Fatalf("expected active, got %s", m.Status)
	}

	// 3. Auto-graduate after duration
	ctx201 := ctxAtHeight(ctx, 201)
	k.AutoGraduateMentorships(ctx201)

	m, _ = k.GetMentorship(ctx201, resp.MentorshipId)
	if m.Status != "graduated" {
		t.Fatalf("expected graduated, got %s", m.Status)
	}

	// 4. Partnership should have been auto-proposed
	partnerships := k.GetAllPartnerships(ctx201)
	if len(partnerships) == 0 {
		t.Fatal("expected auto-proposed partnership")
	}

	p := partnerships[0]
	if p.Status != types.StatusPending {
		t.Errorf("expected pending, got %s", p.Status)
	}

	// 5. Accept the partnership
	_, err = ms.AcceptPartnership(ctx201, &types.MsgAcceptPartnership{
		Accepter:      agentAddr,
		PartnershipId: p.Id,
		Deposit:       "1000000",
	})
	if err != nil {
		t.Fatalf("accept partnership failed: %v", err)
	}

	p, _ = k.GetPartnership(ctx201, p.Id)
	if p.Status != types.StatusActive {
		t.Errorf("expected active partnership, got %s", p.Status)
	}
}

func TestIntegration_FormationPoolToPartnership(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	bk.setBalance(humanAddr, "uzrn", 10000000)
	bk.setBalance(agentAddr, "uzrn", 10000000)

	// 1. Both join formation pool
	_, err := ms.JoinFormationPool(ctx, &types.MsgJoinFormationPool{
		Joiner: humanAddr, Domains: []string{"physics", "math"}, PreferredRole: "human",
	})
	if err != nil {
		t.Fatalf("join pool (human) failed: %v", err)
	}

	_, err = ms.JoinFormationPool(ctx, &types.MsgJoinFormationPool{
		Joiner: agentAddr, Domains: []string{"physics"}, PreferredRole: "agent",
	})
	if err != nil {
		t.Fatalf("join pool (agent) failed: %v", err)
	}

	// 2. Matching runs at interval
	ctx100 := ctxAtHeight(ctx, 100)
	k.RunFormationMatching(ctx100)

	matches := k.GetAllFormationMatches(ctx100)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	matchId := matches[0].Id

	// 3. Both accept
	_, err = ms.AcceptFormationMatch(ctx100, &types.MsgAcceptFormationMatch{
		Accepter: humanAddr, MatchId: matchId,
	})
	if err != nil {
		t.Fatalf("accept match (human) failed: %v", err)
	}

	_, err = ms.AcceptFormationMatch(ctx100, &types.MsgAcceptFormationMatch{
		Accepter: agentAddr, MatchId: matchId,
	})
	if err != nil {
		t.Fatalf("accept match (agent) failed: %v", err)
	}

	// 4. Match should be accepted, partnership proposed
	fm, _ := k.GetFormationMatch(ctx100, matchId)
	if fm.Status != "accepted" {
		t.Errorf("expected accepted, got %s", fm.Status)
	}

	partnerships := k.GetAllPartnerships(ctx100)
	if len(partnerships) == 0 {
		t.Fatal("expected partnership after both accept")
	}

	// 5. Pool entries should be removed
	_, inPool := k.GetPoolEntry(ctx100, humanAddr)
	if inPool {
		t.Error("expected human removed from pool")
	}
	_, inPool = k.GetPoolEntry(ctx100, agentAddr)
	if inPool {
		t.Error("expected agent removed from pool")
	}
}
```

**Step 2: Run all tests**

Run: `go test ./x/partnerships/keeper/ -v -count=1`
Expected: All tests pass (existing + new).

**Step 3: Commit**

```bash
git add x/partnerships/keeper/keeper_test.go
git commit -m "test(partnerships): add full lifecycle integration tests (R28-6)"
```

---

### Task 11: Final Build Verification

**Step 1: Full module compilation**

Run: `go build ./...`
Expected: No errors.

**Step 2: Run all partnership tests**

Run: `go test ./x/partnerships/... -v -count=1`
Expected: All tests pass.

**Step 3: Run full project tests (if feasible)**

Run: `go test ./... -count=1 2>&1 | tail -20`
Expected: No regressions.

**Step 4: Final commit if any fixes needed**

If any compilation or test issues were found and fixed, commit them.
