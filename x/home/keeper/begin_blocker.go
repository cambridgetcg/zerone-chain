package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/home/types"
)

// BeginBlocker runs at the beginning of each block.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	k.CheckDeadmanSwitches(ctx)
	k.CleanupExpiredSessions(ctx)
	return nil
}

// CheckDeadmanSwitches checks all active homes for deadman switch triggers.
func (k Keeper) CheckDeadmanSwitches(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	k.IterateHomes(ctx, func(home *types.AgentHome) bool {
		if home.Status == "archived" {
			return false
		}
		if home.Guardian == nil || home.Guardian.Deadman == nil || !home.Guardian.Deadman.Enabled {
			return false
		}

		threshold := home.Guardian.Deadman.InactivityThreshold
		if threshold == 0 {
			return false
		}

		if height > home.LastActiveBlock+threshold {
			k.triggerDeadman(ctx, home, height)
		}
		return false
	})
}

// triggerDeadman executes the deadman switch action.
func (k Keeper) triggerDeadman(ctx context.Context, home *types.AgentHome, height uint64) {
	// Create critical alert (silently skip if limit reached — status change still proceeds).
	alertID := fmt.Sprintf("deadman-%s-%d", home.HomeId, height)
	alert := &types.Alert{
		AlertId:   alertID,
		HomeId:    home.HomeId,
		AlertType: "deadman_triggered",
		Priority:  "critical",
		Message: fmt.Sprintf(
			"Deadman switch triggered: inactive for %d blocks (action: %s)",
			home.Guardian.Deadman.InactivityThreshold,
			home.Guardian.Deadman.Action,
		),
		CreatedAt: height,
	}
	k.SetAlertWithLimit(ctx, alert)

	// Set home status to guarded.
	home.Status = "guarded"
	k.SetHome(ctx, home)
}

// CleanupExpiredSessions removes sessions that have exceeded their expiry.
func (k Keeper) CleanupExpiredSessions(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	k.IterateHomes(ctx, func(home *types.AgentHome) bool {
		if home.Status == "archived" {
			return false
		}

		// Collect expired sessions first to avoid modifying during iteration.
		var expired []string
		k.IterateSessions(ctx, home.HomeId, func(s *types.ActiveSession) bool {
			if height > s.ExpiresAt {
				expired = append(expired, s.SessionId)
			}
			return false
		})

		for _, sid := range expired {
			k.DeleteSession(ctx, home.HomeId, sid)

			// Create low-priority alert (silently skip if limit reached — cleanup must not be blocked).
			alertID := fmt.Sprintf("session-expired-%s-%d", sid, height)
			alert := &types.Alert{
				AlertId:   alertID,
				HomeId:    home.HomeId,
				AlertType: "session_expired",
				Priority:  "low",
				Message:   fmt.Sprintf("Session %s expired", sid),
				CreatedAt: height,
			}
			k.SetAlertWithLimit(ctx, alert)
		}
		return false
	})
}
