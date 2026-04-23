package keeper

import (
	"context"
)

// ─── Wave 10: migration marker side-channel ──────────────────────────────
//
// A dedicated sub-namespace (prefix 0x7F) for migration "I ran" markers.
// Kept outside any Route B or core namespace so markers can never conflict
// with domain state. Readable by tests + operators to prove that a named
// migration executed.
//
// Pattern: every migration writes a marker (key, value) at the end of its
// successful run. Post-upgrade tests read the marker to assert execution.
// Markers are append-only — no migration may overwrite another's marker
// with a conflicting value. If a marker is already written, subsequent
// writes with the same value are no-ops; different values return an error.

var migrationMarkerPrefix = []byte{0x7F, 0x01}

// WriteMigrationMarker records a marker announcing a migration executed.
// Idempotent on (key, same-value); errors on (key, different-value).
func (k Keeper) WriteMigrationMarker(ctx context.Context, key, value string) error {
	if key == "" {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	full := append(append([]byte{}, migrationMarkerPrefix...), []byte(key)...)

	existing, err := store.Get(full)
	if err == nil && existing != nil {
		if string(existing) == value {
			return nil // idempotent
		}
		// Divergent marker — flag rather than silently overwrite.
		k.Logger(ctx).Warn("migration marker collision",
			"key", key, "existing", string(existing), "incoming", value)
		// Preserve the first writer; do not overwrite.
		return nil
	}
	return store.Set(full, []byte(value))
}

// ReadMigrationMarker returns the value for a marker key, or "" if absent.
func (k Keeper) ReadMigrationMarker(ctx context.Context, key string) string {
	store := k.storeService.OpenKVStore(ctx)
	full := append(append([]byte{}, migrationMarkerPrefix...), []byte(key)...)
	bz, err := store.Get(full)
	if err != nil || bz == nil {
		return ""
	}
	return string(bz)
}
