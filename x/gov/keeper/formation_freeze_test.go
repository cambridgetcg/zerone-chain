package keeper_test

import (
	"context"
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Mock PartnershipsKeeper ----------

type mockPartnershipsKeeper struct {
	freezes map[string]freezeRecord
}

type freezeRecord struct {
	expiryHeight uint64
	reason       string
}

func newMockPartnershipsKeeper() *mockPartnershipsKeeper {
	return &mockPartnershipsKeeper{freezes: make(map[string]freezeRecord)}
}

func (m *mockPartnershipsKeeper) SetDomainFormationFreeze(_ context.Context, domain string, expiryHeight uint64, reason string) {
	m.freezes[domain] = freezeRecord{expiryHeight: expiryHeight, reason: reason}
}

// ---------- Tests ----------

func TestDomainFormationFreeze_AuthorityOnly(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	pk := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(pk)

	ms := keeper.NewMsgServerImpl(k)

	// Non-authority should fail.
	_, err := ms.DomainFormationFreeze(ctx, &types.MsgDomainFormationFreeze{
		Authority:      testAddr("random"),
		Domain:         "physics",
		DurationBlocks: 1000,
		Reason:         "governance review",
	})
	if err == nil {
		t.Fatal("expected unauthorized error, got nil")
	}

	// Should not have set any freeze.
	if len(pk.freezes) != 0 {
		t.Errorf("expected no freezes after unauthorized attempt, got %d", len(pk.freezes))
	}
}

func TestDomainFormationFreeze_DelegateToPartnerships(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	pk := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(pk)

	ms := keeper.NewMsgServerImpl(k)

	// Authority call should succeed.
	_, err := ms.DomainFormationFreeze(ctx, &types.MsgDomainFormationFreeze{
		Authority:      k.GetAuthority(),
		Domain:         "physics",
		DurationBlocks: 1000,
		Reason:         "governance review",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// Check freeze was set with correct expiry.
	freeze, ok := pk.freezes["physics"]
	if !ok {
		t.Fatal("expected freeze to be set on partnerships keeper")
	}
	expectedExpiry := uint64(ctx.BlockHeight()) + 1000
	if freeze.expiryHeight != expectedExpiry {
		t.Errorf("expected expiryHeight=%d, got %d", expectedExpiry, freeze.expiryHeight)
	}
	if freeze.reason != "governance review" {
		t.Errorf("expected reason='governance review', got %q", freeze.reason)
	}

	// Check event emitted.
	events := ctx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == "zerone.gov.domain_formation_freeze" {
			found = true
			for _, attr := range e.Attributes {
				if attr.Key == "domain" && attr.Value != "physics" {
					t.Errorf("expected domain=physics, got %s", attr.Value)
				}
			}
		}
	}
	if !found {
		t.Error("expected domain_formation_freeze event")
	}
}

func TestDomainFormationFreeze_EmptyDomain(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	pk := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(pk)

	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.DomainFormationFreeze(ctx, &types.MsgDomainFormationFreeze{
		Authority:      k.GetAuthority(),
		Domain:         "",
		DurationBlocks: 1000,
		Reason:         "test",
	})
	if err == nil {
		t.Fatal("expected error for empty domain, got nil")
	}
}

func TestDomainFormationFreeze_ZeroDuration(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	pk := newMockPartnershipsKeeper()
	k.SetPartnershipsKeeper(pk)

	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.DomainFormationFreeze(ctx, &types.MsgDomainFormationFreeze{
		Authority:      k.GetAuthority(),
		Domain:         "physics",
		DurationBlocks: 0,
		Reason:         "test",
	})
	if err == nil {
		t.Fatal("expected error for zero duration, got nil")
	}
}

func TestDomainFormationFreeze_NilPartnershipsKeeper(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	// Do NOT set partnerships keeper — it should still succeed (nil-safe).
	_, err := ms.DomainFormationFreeze(ctx, &types.MsgDomainFormationFreeze{
		Authority:      k.GetAuthority(),
		Domain:         "physics",
		DurationBlocks: 1000,
		Reason:         "governance review",
	})
	if err != nil {
		t.Fatalf("expected success even with nil partnerships keeper, got: %v", err)
	}
}
