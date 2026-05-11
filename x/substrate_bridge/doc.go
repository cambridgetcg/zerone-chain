// Package substrate_bridge is the Tier-1 foundation for external
// recursive work modules in ZERONE. It is the one place external work
// meets ZERONE substrate; every external work class (x/translation,
// x/curriculum, x/hypothesis_market, etc.) registers with this module.
//
// Three sub-systems share one keeper:
//
//   - Adapter framework (M3): gov-gated registry of typed external-source
//     converters. Adapter is a recipe (binary hash + axis bounds + bond
//     + qualification requirements + slash gradient), not a service.
//     Registered via CategoryAdapterRegistration LIP.
//
//   - Substrate-link compiler (M2): permissive two-section provenance.
//     cited_facts must exist in x/knowledge at commit time; pending_claims
//     are auto-submitted as Claims and the attestation is held in
//     AWAITING_RESOLUTION until they resolve. Settlement is partial-
//     proportional to verified ratio (M4 generalized).
//
//   - Cross-class lineage propagator (M6): DAG-by-timestamp citation
//     graph; depth-decayed royalty propagation at downstream
//     settlement; revenue-stream amplification (cumulative accumulator
//     at LineageRoyaltyAccumulatorPrefix). Self-citation capped at
//     self_citation_cap_bps to prevent self-funneling.
//
// Doctrinal commitments preserved here:
//
//   - UW (ZERONE is recursive): every reward path requires substrate-link
//     and is scored against per-axis projection; non-recursive verified
//     work earns base only.
//   - M1 (stake-backed claim): submitter bonds locked at submit; slash
//     gradient applied per rejection mode.
//   - M2 (substrate-link mandate): re-derivable link_hash; pending claims
//     materialize as real Claims in x/knowledge.
//   - M3 (class-specific verification under shared lifecycle): adapter
//     registry gov-gated; submitter qualification enforced.
//   - M5 (recursion-weight projection): per-axis bounds at adapter level;
//     AxisProjection enforced at submit.
//   - M6 (lineage propagates AND recurses): cross-class DAG with
//     depth-decayed propagation; cumulative accumulator realizes the
//     revenue-stream amplification interpretation.
//   - M7 (chain pays for own audit): useful_work_audit_bounty_pool
//     module account declared here; passive at Phase 0; actively used
//     when challenge mechanism lands.
//
// Phase 0 ships substrate_bridge as standalone-usable via
// MsgSubmitExternalAttestation. When x/work Phase 1 lands, it will
// call PrepareExternalAttestation and SettleExternalAttestation as the
// integrated submission path; the standalone MsgSubmitExternalAttestation
// path is preserved as the direct-submit fallback.
//
// We speak through intentions.
package substrate_bridge
