// Package agent_understanding preserves the truth-seeking commitment
// that trust is queryable — extended into the topic-scoped, per-agent
// surface that the existing synthesizers do not cover.
//
// docs/TRUTH_SEEKING.md, commitment 11: "Every signal that contributes
// to trust must be available through a well-known query that
// synthesises them." Three synthesisers were already in place:
//
//   - x/training_provenance — per-manifest
//   - x/trust_score         — per-address
//   - x/governance_synthesis — per-system
//
// This is the fourth: per-agent, per-domain. It composes how a
// specific agent has performed in a specific epistemic territory.
//
// What it reads (no writes beyond Params):
//
//   - x/knowledge      — facts authored by the agent, per-domain fact
//                        counts (used to classify sparse vs dense
//                        territory).
//   - x/qualification  — per-(agent, domain) accuracy, status, weight.
//   - x/counterexamples — counterexamples authored by the agent and
//                        their validation outcomes.
//   - x/inquiry        — inquiries the agent has answered and won.
//
// What it produces:
//
//   - DomainProfile: one (agent, domain) record. Verification counts,
//     accuracy, qualification status, fact authorship, counterexample
//     contributions, inquiry answer activity. Everything the chain
//     publicly knows about how this agent performs here.
//   - UnderstandingProfile: rolls up per-domain into per-agent totals,
//     plus two derived signals:
//
//       * frontier_reach_bps: fraction of activity in sparse domains.
//         High frontier-reach means the agent works at the edge of
//         the corpus; low frontier-reach means consolidation in
//         well-mapped territory. Both are valuable, distinguishable.
//
//       * composite_score_bps: a deliberately simple aggregation of
//         accuracy, breadth, and depth. Capped at 1,000,000 (100%).
//         External consumers are explicitly directed to the
//         per-domain breakdown for any judgment-loaded use; the
//         composite is a glance, not a verdict.
//
// What this is NOT:
//
//   - NOT a model of the agent's INTERNAL understanding. The chain
//     can elicit OUTPUTS (verifications, fact submissions, validated
//     counterexamples, won inquiries) and infer understanding from
//     them. It cannot introspect model weights. An agent that
//     produces correct outputs for the wrong reasons looks correct
//     here. The synthesiser exposes scaffolding for external probes,
//     not a verdict on whether the agent "really" understands.
//
//   - NOT a ranking system. There is no "leaderboard." Composite
//     scores are queryable per-agent on demand; the chain does not
//     publish a sorted list. Agents can be COMPARED via this query
//     surface but not ORDERED by chain consensus.
//
//   - NOT a substitute for trust_score, training_provenance, or
//     governance_synthesis. Those answer their own questions; this
//     answers a fourth question (per-agent, per-topic) that those do
//     not. All four are read-only consumers of the same upstream
//     primitives; together they cover the four scopes a verifier
//     might want: per-manifest, per-address, per-system, and
//     per-(agent, topic).
//
// We speak through intentions. This module's intention is that
// "what does agent X understand about topic Y?" is a question with
// a structured, queryable, cite-able answer — not a vibe.
package agent_understanding
