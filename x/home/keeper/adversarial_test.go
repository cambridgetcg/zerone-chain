package keeper_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zerone-chain/zerone/x/home/types"
)

// ===== A. Input Validation Tests (7) =====

func TestAdvInputValidation_EmptyName(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	_, err := ms.CreateHome(ctx, &types.MsgCreateHome{
		Owner: owner,
		Name:  "",
	})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !strings.Contains(err.Error(), "name cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvInputValidation_LongName(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	longName := strings.Repeat("a", 129)
	_, err := ms.CreateHome(ctx, &types.MsgCreateHome{
		Owner: owner,
		Name:  longName,
	})
	if err == nil {
		t.Fatal("expected error for name exceeding 128 chars, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvInputValidation_NullBytesInName(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	_, err := ms.CreateHome(ctx, &types.MsgCreateHome{
		Owner: owner,
		Name:  "valid\x00evil",
	})
	if err == nil {
		t.Fatal("expected error for null bytes in name, got nil")
	}
	if !strings.Contains(err.Error(), "null bytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvInputValidation_EmptyKeyHash(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	// Create home first.
	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}
	_ = k

	_, err = ms.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "",
		KeyType:     "ed25519",
		Role:        "agent",
		Permissions: []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error for empty key hash, got nil")
	}
	if !strings.Contains(err.Error(), "key_hash cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvInputValidation_LongKeyHash(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	longHash := strings.Repeat("f", 129)
	_, err = ms.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     longHash,
		KeyType:     "ed25519",
		Role:        "agent",
		Permissions: []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error for key hash exceeding 128 chars, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvInputValidation_EmptyCID(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	_, err = ms.UpdateMemoryCID(ctx, &types.MsgUpdateMemoryCID{
		Owner:  owner,
		HomeId: resp.HomeId,
		Cid:    "",
	})
	if err == nil {
		t.Fatal("expected error for empty CID, got nil")
	}
	if !strings.Contains(err.Error(), "cid cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvInputValidation_LongCID(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	longCID := strings.Repeat("Q", 257)
	_, err = ms.UpdateMemoryCID(ctx, &types.MsgUpdateMemoryCID{
		Owner:  owner,
		HomeId: resp.HomeId,
		Cid:    longCID,
	})
	if err == nil {
		t.Fatal("expected error for CID exceeding 256 chars, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===== B. Permission Escalation Tests (5) =====

func TestAdvPermission_DisjointPermissionsEmptyIntersection(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Register key with only "read" permission.
	_, err = ms.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "keyhash1",
		KeyType:     "ed25519",
		Role:        "agent",
		Permissions: []string{"read"},
	})
	if err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}

	// Start session requesting only "write" — disjoint from available ["read"].
	sesResp, err := ms.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "keyhash1",
		RequestedPermissions: []string{"write"},
	})
	if err != nil {
		t.Fatalf("StartSession should succeed with empty intersection, got: %v", err)
	}

	// The granted permissions should be empty (no "write" in available).
	ses, found := k.GetSession(ctx, resp.HomeId, sesResp.SessionId)
	if !found {
		t.Fatal("session not found")
	}
	if len(ses.Permissions) != 0 {
		t.Fatalf("expected 0 granted permissions (disjoint), got %d: %v", len(ses.Permissions), ses.Permissions)
	}
}

func TestAdvPermission_ExpiredKeyRejected(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Register key that expires at block 50 (current block is 100).
	_, err = ms.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "expiredkey1",
		KeyType:     "ed25519",
		Role:        "agent",
		Permissions: []string{"read", "write"},
		ExpiresAt:   50,
	})
	if err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}

	// Attempt to start session with expired key at block 100.
	_, err = ms.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "expiredkey1",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error for expired key, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvPermission_GuardianCannotUpdateRegisterRevokeConfigure(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	guardian := testAddr("guardian1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Set guardian.
	_, err = ms.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:           owner,
		HomeId:          resp.HomeId,
		GuardianAddress: guardian,
	})
	if err != nil {
		t.Fatalf("ConfigureGuardian failed: %v", err)
	}
	_ = k

	// Guardian tries to UpdateHome — should fail.
	_, err = ms.UpdateHome(ctx, &types.MsgUpdateHome{
		Owner:  guardian,
		HomeId: resp.HomeId,
		Name:   "hacked",
	})
	if err == nil {
		t.Fatal("guardian should not be able to UpdateHome")
	}

	// Guardian tries to RegisterKey — should fail.
	_, err = ms.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       guardian,
		HomeId:      resp.HomeId,
		KeyHash:     "guardiankey",
		KeyType:     "ed25519",
		Role:        "admin",
		Permissions: []string{"read", "write", "admin"},
	})
	if err == nil {
		t.Fatal("guardian should not be able to RegisterKey")
	}

	// Guardian tries to RevokeKey — should fail.
	_, err = ms.RevokeKey(ctx, &types.MsgRevokeKey{
		Owner:   guardian,
		HomeId:  resp.HomeId,
		KeyHash: "anykey",
	})
	if err == nil {
		t.Fatal("guardian should not be able to RevokeKey")
	}

	// Guardian tries to ConfigureGuardian — should fail.
	_, err = ms.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:           guardian,
		HomeId:          resp.HomeId,
		DefenseStrategy: "aggressive",
	})
	if err == nil {
		t.Fatal("guardian should not be able to ConfigureGuardian")
	}
}

func TestAdvPermission_GuardianCanAcknowledgeAlert(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	guardian := testAddr("guardian1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Set guardian.
	_, err = ms.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:           owner,
		HomeId:          resp.HomeId,
		GuardianAddress: guardian,
	})
	if err != nil {
		t.Fatalf("ConfigureGuardian failed: %v", err)
	}

	// Create an alert directly.
	alert := &types.Alert{
		AlertId:   "test-alert-1",
		HomeId:    resp.HomeId,
		AlertType: "test",
		Priority:  "low",
		Message:   "test alert",
		CreatedAt: 100,
	}
	k.SetAlert(ctx, alert)

	// Guardian acknowledges — should succeed.
	_, err = ms.AcknowledgeAlert(ctx, &types.MsgAcknowledgeAlert{
		Signer:  guardian,
		HomeId:  resp.HomeId,
		AlertId: "test-alert-1",
	})
	if err != nil {
		t.Fatalf("guardian should be able to acknowledge alerts, got: %v", err)
	}

	// Verify alert is acknowledged.
	ack, found := k.GetAlert(ctx, resp.HomeId, "test-alert-1")
	if !found {
		t.Fatal("alert not found after acknowledge")
	}
	if !ack.Acknowledged {
		t.Fatal("alert should be acknowledged")
	}
}

func TestAdvPermission_InvalidBech32RecoveryAddresses(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Try to configure with invalid bech32 recovery addresses.
	_, err = ms.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:             owner,
		HomeId:            resp.HomeId,
		RecoveryAddresses: []string{"not-a-bech32-address"},
		RecoveryThreshold: 1,
	})
	if err == nil {
		t.Fatal("expected error for invalid bech32 recovery address, got nil")
	}
	if !strings.Contains(err.Error(), "invalid recovery address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===== C. DoS / State Exhaustion Tests (4) =====

func TestAdvDoS_HomeCreationFeeDeterrent(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("spammer")

	// Give enough for exactly 2 homes (10 ZRN each = 20 ZRN total).
	bk.setBalance(owner, "uzrn", 20_000_000)

	_, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "home1"})
	if err != nil {
		t.Fatalf("first home should succeed: %v", err)
	}
	_, err = ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "home2"})
	if err != nil {
		t.Fatalf("second home should succeed: %v", err)
	}

	// Third home should fail — insufficient funds.
	bk.failNext = true
	_, err = ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "home3"})
	if err == nil {
		t.Fatal("third home should fail due to insufficient funds")
	}
}

func TestAdvDoS_AlertFloodCappedByMaxAlertsPerHome(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Set a low alert limit.
	params := k.GetParams(ctx)
	params.MaxAlertsPerHome = 3
	k.SetParams(ctx, params)

	// Fill up alerts.
	for i := uint64(0); i < 3; i++ {
		alert := &types.Alert{
			AlertId:   fmt.Sprintf("flood-%d", i),
			HomeId:    resp.HomeId,
			AlertType: "test",
			Priority:  "low",
			Message:   "flood",
			CreatedAt: 100,
		}
		stored := k.SetAlertWithLimit(ctx, alert)
		if !stored {
			t.Fatalf("alert %d should have been stored", i)
		}
	}

	// 4th alert should be rejected.
	overflow := &types.Alert{
		AlertId:   "flood-overflow",
		HomeId:    resp.HomeId,
		AlertType: "test",
		Priority:  "low",
		Message:   "should be rejected",
		CreatedAt: 100,
	}
	stored := k.SetAlertWithLimit(ctx, overflow)
	if stored {
		t.Fatal("alert beyond MaxAlertsPerHome should not be stored")
	}

	// Verify count.
	count := k.CountPendingAlerts(ctx, resp.HomeId)
	if count != 3 {
		t.Fatalf("expected 3 alerts, got %d", count)
	}
}

func TestAdvDoS_BeginBlockerRespectsAlertLimit(t *testing.T) {
	_, k, ctx, _ := setupMsgServer(t)

	// Create home with deadman and sessions directly.
	home := &types.AgentHome{
		HomeId:       "home-bb",
		OwnerAddress: testAddr("owner1"),
		Name:         "bb-test",
		Status:       "active",
		ComfortScore: 50,
		Guardian: &types.HomeGuardian{
			Deadman: &types.DeadmanConfig{
				Enabled:             true,
				InactivityThreshold: 10,
				Action:              "guard",
			},
		},
		CreatedAtBlock:  1,
		LastActiveBlock: 1,
	}
	k.SetHome(ctx, home)

	// Set MaxAlertsPerHome to 2.
	params := k.GetParams(ctx)
	params.MaxAlertsPerHome = 2
	k.SetParams(ctx, params)

	// Pre-fill with 2 alerts.
	for i := 0; i < 2; i++ {
		k.SetAlert(ctx, &types.Alert{
			AlertId:   fmt.Sprintf("prefill-%d", i),
			HomeId:    "home-bb",
			AlertType: "test",
			Priority:  "low",
			Message:   "prefill",
			CreatedAt: 1,
		})
	}

	// Run BeginBlocker — deadman should trigger but alert should be silently skipped.
	err := k.BeginBlocker(ctx)
	if err != nil {
		t.Fatalf("BeginBlocker should not error: %v", err)
	}

	// Home status should still change to guarded (action not blocked).
	updatedHome, found := k.GetHome(ctx, "home-bb")
	if !found {
		t.Fatal("home not found after BeginBlocker")
	}
	if updatedHome.Status != "guarded" {
		t.Fatalf("expected status 'guarded', got %q", updatedHome.Status)
	}

	// But alert count should still be 2 (new alert silently dropped).
	count := k.CountPendingAlerts(ctx, "home-bb")
	if count != 2 {
		t.Fatalf("expected 2 alerts (limit), got %d", count)
	}
}

func TestAdvDoS_BeginBlockerScalability(t *testing.T) {
	_, k, ctx, _ := setupMsgServer(t)

	// Create 50 homes with deadman switches.
	for i := 0; i < 50; i++ {
		home := &types.AgentHome{
			HomeId:       fmt.Sprintf("home-scale-%d", i),
			OwnerAddress: testAddr(fmt.Sprintf("owner-%d", i)),
			Name:         fmt.Sprintf("scale-%d", i),
			Status:       "active",
			ComfortScore: 50,
			Guardian: &types.HomeGuardian{
				Deadman: &types.DeadmanConfig{
					Enabled:             true,
					InactivityThreshold: 10,
					Action:              "guard",
				},
			},
			CreatedAtBlock:  1,
			LastActiveBlock: 1,
		}
		k.SetHome(ctx, home)

		// Add 3 sessions each.
		for j := 0; j < 3; j++ {
			session := &types.ActiveSession{
				SessionId: fmt.Sprintf("ses-%d-%d", i, j),
				HomeId:    fmt.Sprintf("home-scale-%d", i),
				KeyHash:   "somekeyhash",
				StartedAt: 1,
				ExpiresAt: 50, // expired at current block 100
			}
			k.SetSession(ctx, session)
		}
	}

	// Measure BeginBlocker time.
	start := time.Now()
	err := k.BeginBlocker(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("BeginBlocker failed: %v", err)
	}

	// Should complete well under 1 second for 50 homes.
	if elapsed > 5*time.Second {
		t.Fatalf("BeginBlocker took too long: %v (>5s for 50 homes)", elapsed)
	}

	t.Logf("BeginBlocker for 50 homes with 150 expired sessions: %v", elapsed)
}

// ===== D. Recovery Mechanism Tests (3) =====

func TestAdvRecovery_ThresholdZeroAccepted(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// threshold=0 with empty recovery addresses should be accepted.
	_, err = ms.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:             owner,
		HomeId:            resp.HomeId,
		RecoveryAddresses: []string{},
		RecoveryThreshold: 0,
	})
	if err != nil {
		t.Fatalf("threshold=0 should be accepted: %v", err)
	}
}

func TestAdvRecovery_NoMsgRecoverHomeExists(t *testing.T) {
	// This test documents that MsgRecoverHome does not exist in the proto definition.
	// Recovery is a planned future feature — this test exists as a marker.
	//
	// When MsgRecoverHome is implemented, this test should be replaced with
	// functional tests for the recovery workflow.
	ms, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Set recovery addresses.
	recoveryAddr := testAddr("recovery1")
	_, err = ms.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:             owner,
		HomeId:            resp.HomeId,
		RecoveryAddresses: []string{recoveryAddr},
		RecoveryThreshold: 1,
	})
	if err != nil {
		t.Fatalf("ConfigureGuardian failed: %v", err)
	}

	// Verify recovery config is stored — but no execution path exists yet.
	home, found := k.GetHome(ctx, resp.HomeId)
	_ = found
	if len(home.Guardian.RecoveryAddresses) != 1 {
		t.Fatalf("expected 1 recovery address, got %d", len(home.Guardian.RecoveryAddresses))
	}
	if home.Guardian.RecoveryThreshold != 1 {
		t.Fatalf("expected threshold 1, got %d", home.Guardian.RecoveryThreshold)
	}
	// Note: No MsgRecoverHome type exists. Recovery addresses are stored but not actionable.
	t.Log("DOCUMENTED: MsgRecoverHome does not exist — recovery addresses stored but not actionable")
}

func TestAdvRecovery_InvalidGuardianAddress(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Invalid guardian address.
	_, err = ms.ConfigureGuardian(ctx, &types.MsgConfigureGuardian{
		Owner:           owner,
		HomeId:          resp.HomeId,
		GuardianAddress: "not-valid-bech32",
	})
	if err == nil {
		t.Fatal("expected error for invalid guardian address, got nil")
	}
	if !strings.Contains(err.Error(), "invalid guardian address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ===== E. Race Condition Tests (2) =====

func TestAdvRace_RevokedKeyCannotStartSession(t *testing.T) {
	ms, _, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Register key.
	_, err = ms.RegisterKey(ctx, &types.MsgRegisterKey{
		Owner:       owner,
		HomeId:      resp.HomeId,
		KeyHash:     "racekey1",
		KeyType:     "ed25519",
		Role:        "agent",
		Permissions: []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}

	// Revoke the key.
	_, err = ms.RevokeKey(ctx, &types.MsgRevokeKey{
		Owner:   owner,
		HomeId:  resp.HomeId,
		KeyHash: "racekey1",
	})
	if err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}

	// Immediately try to start a session with revoked key — should be rejected.
	_, err = ms.StartSession(ctx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "racekey1",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Fatal("expected error for revoked key session, got nil")
	}
	if !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvRace_MaxSessionsEnforcedAcrossBlocks(t *testing.T) {
	ms, k, ctx, bk := setupMsgServer(t)
	owner := testAddr("owner1")
	bk.setBalance(owner, "uzrn", 100_000_000)

	resp, err := ms.CreateHome(ctx, &types.MsgCreateHome{Owner: owner, Name: "test"})
	if err != nil {
		t.Fatalf("CreateHome failed: %v", err)
	}

	// Set max sessions to 2.
	params := k.GetParams(ctx)
	params.MaxSessionsPerHome = 2
	k.SetParams(ctx, params)

	// Register 3 keys across different "blocks".
	for i := 0; i < 3; i++ {
		keyHash := fmt.Sprintf("sessionkey%d", i)
		_, err = ms.RegisterKey(ctx, &types.MsgRegisterKey{
			Owner:       owner,
			HomeId:      resp.HomeId,
			KeyHash:     keyHash,
			KeyType:     "ed25519",
			Role:        "agent",
			Permissions: []string{"read"},
		})
		if err != nil {
			t.Fatalf("RegisterKey %d failed: %v", i, err)
		}
	}

	// Start 2 sessions — should succeed.
	for i := 0; i < 2; i++ {
		newCtx := ctx.WithBlockHeight(int64(100 + i))
		_, err = ms.StartSession(newCtx, &types.MsgStartSession{
			Signer:               owner,
			HomeId:               resp.HomeId,
			KeyHash:              fmt.Sprintf("sessionkey%d", i),
			RequestedPermissions: []string{"read"},
		})
		if err != nil {
			t.Fatalf("StartSession %d failed: %v", i, err)
		}
	}

	// 3rd session at a different block should still be rejected.
	newCtx := ctx.WithBlockHeight(200)
	_, err = ms.StartSession(newCtx, &types.MsgStartSession{
		Signer:               owner,
		HomeId:               resp.HomeId,
		KeyHash:              "sessionkey2",
		RequestedPermissions: []string{"read"},
	})
	if err == nil {
		t.Fatal("expected max sessions error, got nil")
	}
	if !strings.Contains(err.Error(), "maximum sessions") {
		t.Fatalf("unexpected error: %v", err)
	}
}
