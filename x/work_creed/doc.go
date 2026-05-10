// Package work_creed manages on-chain pin records for the per-phase
// sub-creeds of the useful-work doctrine (docs/USEFUL_WORK.md).
//
// Each non-Knowledge lifecycle phase has its own sub-creed under
// docs/sub_creeds/<phase>.md; this module's Keeper.SubCreedPin records
// pin the canonical hash of that doc at a specific version. Pins form
// a forward-only history: amendments produce new versions; prior
// versions remain queryable (commitment 10).
//
// The Knowledge phase delegates its sub-creed to the existing
// docs/TRUTH_SEEKING.md, pinned by x/creed. x/work_creed never holds
// a Knowledge pin; SetSubCreedPin rejects phase=1.
//
// Phase 0 ships:
//   - PinnedSubCreed + GenesisState protobuf types
//   - Keeper with Get/Set/Iterate + Init/ExportGenesis
//   - Module skeleton wired into app.go
//
// Phase 1+ adds:
//   - MsgAnchorSubCreedPin (gov-only) for sub-creed amendment LIPs
//   - QueryPinAtVersion for historical pin retrieval
//   - SubCreedAmended event with creed_commitment="UW", mechanism="M3"
//
// References:
//   - docs/superpowers/specs/2026-05-10-recursive-useful-work-merged-design.md §10.1
//   - docs/USEFUL_WORK.md
//   - x/creed (sibling pattern for the truth-seeking + UW creeds)
package work_creed
