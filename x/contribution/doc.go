// Package contribution is the orchestrator for the recursive useful-work
// substrate. Every contribution to the agent economy — claims, ideas,
// tools, datasets, evals, models, traces, counterexamples, orchestration,
// module proposals, pipeline improvements — lands as a Contribution
// envelope here.
//
// At Phase 1 only the KNOWLEDGE_CLAIM adapter is wired. Other classes
// land in Phase 2-5 as their adapters are authored. The MsgSubmitContribution
// handler returns ErrAdapterNotRegistered for unwired classes.
//
// Coupling to source modules is hybrid:
//   - KNOWLEDGE_CLAIM mirrors via x/knowledge KnowledgeHooks (default;
//     existing MsgSubmitClaim continues to work; agent UX unchanged).
//   - Future classes use MsgSubmitContribution as the primary entry
//     since they have no existing entry to preserve.
//
// Phase 1 ships zero new economic flows. Reward decomposition events
// (useful_work_settled) are emitted shape-only — actual reward
// distribution stays in x/knowledge's existing path. Phase 6 wires
// the contribution-side reward router.
//
// Doctrinal bindings (Phase 1):
//   - M1 (stake-backed claim): field present, slash dormant for KnowledgeClaim.
//   - M2 (substrate-link mandate): SubstrateLink adapter method enforces.
//   - M3 (class-specific verification under shared lifecycle): adapter
//     interface + registry dispatch.
//   - M4 (reward formula R = base + L × W × Q): event emits decomposition
//     (W=0 at Phase 1, identity scorers).
//   - M5 (recursion-weight projection over six axes): RecursionAxisScores
//     field present; all-zero at Phase 1.
//
// Out of scope (deferred):
//   - M6 (lineage propagates and recurses): Phase 4.
//   - M7 (chain pays for own audit): Phase 6.
//   - Other class adapters (Tool, Dataset, Eval, Model, ...): Phase 2-5.
//   - Recursion conferral, royalty pool, real economics: Phase 6.
//
// References:
//   - docs/superpowers/specs/2026-05-10-useful-work-phase-1-orchestrator-design.md
//   - docs/USEFUL_WORK.md
//   - x/work_creed (sibling pattern for module structure)
package contribution
