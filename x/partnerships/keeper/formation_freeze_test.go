package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/partnerships/keeper"
	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// Test 5: Earth→Water — freeze blocks partnership formation.
func TestEarthWater_FreezeBlocksFormation(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Set a formation freeze on domain "physics".
	k.SetDomainFormationFreeze(ctx, "physics", 200, "governance review")

	// Fund proposer.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)

	// Attempt to propose partnership in frozen domain — should be blocked.
	_, err := ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agentAddr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "physics",
	})
	if err == nil {
		t.Fatal("expected error due to formation freeze, got nil")
	}
	if !types.ErrDomainFrozen.Is(err) {
		t.Errorf("expected ErrDomainFrozen, got: %v", err)
	}
}

// Test 6: Earth→Water — expired freeze allows formation.
func TestEarthWater_ExpiredFreezeAllowsFormation(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Set freeze expiring at block 50 (current is 100).
	k.SetDomainFormationFreeze(ctx, "physics", 50, "old freeze")

	// Run expiry sweep.
	k.ExpireFormationFreezes(ctx)

	// Fund proposer.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)

	// Partnership should succeed now.
	_, err := ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agentAddr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "physics",
	})
	if err != nil {
		t.Fatalf("expected success after freeze expired, got: %v", err)
	}
}

// Test 7: Earth→Water — freeze on domain A doesn't affect domain B.
func TestEarthWater_FreezeIsDomainSpecific(t *testing.T) {
	k, ctx, bk := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Freeze only "physics".
	k.SetDomainFormationFreeze(ctx, "physics", 200, "governance review")

	// Fund proposer.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)

	// Partnership in "biology" should succeed.
	_, err := ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agentAddr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "biology",
	})
	if err != nil {
		t.Fatalf("expected success for unfrozen domain, got: %v", err)
	}

	// Partnership in "physics" should fail.
	bk.setBalance(humanAddr, "uzrn", 10_000_000)
	_, err = ms.ProposePartnership(ctx, &types.MsgProposePartnership{
		Proposer:       humanAddr,
		Partner:        agent2Addr,
		ProposedTier:   0,
		InitialDeposit: "1000000",
		Domain:         "physics",
	})
	if err == nil {
		t.Fatal("expected error for frozen domain physics, got nil")
	}
}
