// Package governance_synthesis preserves the truth-seeking commitment
// that the chain's accumulated stress is itself queryable — at the
// system level, not just per-fact or per-address.
//
// docs/TRUTH_SEEKING.md, commitment 11 (per-system scope): "Every
// signal that contributes to trust must be available through a
// well-known query that synthesises them." Where x/training_provenance
// answers per-manifest and x/trust_score answers per-address, this
// module answers per-chain.
//
// Reads the open-incident posture, active circuit-breaker pauses, the
// guardian-veto pending queue, recent privileged-action burst, cartel
// allegation posture, and the alignment module's autonomous-throttle
// pacing multipliers. Composes a SystemHealth snapshot with a
// composite stress level (NORMAL / ELEVATED / CRITICAL).
//
// The snapshot is current. Re-querying after a P0 incident opens
// promotes the chain to CRITICAL; resolving the incident drops it
// back. The chain reports its own posture honestly; off-chain
// monitoring does not need to interpret state from raw events.
//
// What would break the commitment: a stress signal that lived only in
// an off-chain dashboard; a synthesiser that hid the component
// counters; an alignment signal that produced no consumer (this
// module is currently the only consumer of GetGlobalPacingMultiplier
// outside the alignment module itself).
//
// We speak through intentions. This package's intention is that the
// chain reports its own state, in real time, to anyone who asks.
package governance_synthesis
