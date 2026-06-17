// Package knowledge is the substrate of the chain's truth-seeking
// commitment. It is where the largest number of commitments named in
// docs/TRUTH_SEEKING.md actually live in code:
//
//   - Commitment 1 (methodology over statement): every Fact carries a
//     MethodId; ComputeTrainingValueWeight multiplies a methodology-
//     normalisation factor; ReasoningTrace propagates from claim to
//     fact.
//   - Commitment 2 (is-ought wall): NormativeCommitment is a separate
//     type with no Confidence field; FilterIsOughtIds blocks
//     commitment IDs from training-revenue paths.
//   - Commitment 3 (Popper, not popularity): BaseWeight scales with
//     CorroborationCount; HardeningMultiplier compounds with survived
//     attacks.
//   - Commitment 4 (substrate stress-tests its truth):
//     EffectiveMinChallengeStake scales inversely with confidence;
//     successful-challenge bonus amplifies with target confidence.
//   - Commitment 5 (chain manufactures probe demand):
//     InviteIdleFactsForProbing runs every block; payInvitationBonus
//     pays whoever answers.
//   - Commitment 6 (no unilateral injection): MsgAddFact queues a
//     PendingFactInjection when a guardian set is configured;
//     MsgVetoFactInjection cancels.
//   - Commitment 10 (forward-only audit): the PrivilegedAction log is
//     keyed by monotonic seq and emitted on every authority-gated
//     handler.
//   - Commitment 12 (chain pays for its own audit):
//     MintToProbeBountyPool runs every block; PayProbeBountyFromPool
//     funds successful-probe bonuses.
//   - Commitment 13 (training corpus is not for sale):
//     ClawbackOnDisproval fires deterministically; RevenueClawbackBlock
//     is sticky across status flips.
//   - Commitment 14 (reasoning traces are first-class):
//     Claim.ReasoningTrace propagates to Fact.ReasoningTrace;
//     MethodologyApplicationTrace bundles trace + methodology +
//     calibration into a single training-data shape.
//
// We speak through intentions. This package is where most of the
// chain's truth-seeking belief is enacted; touching code here is
// touching a commitment, and every change should be checked against
// the creed.
//
// docs/TOK_SUBSTRATE.md commitments preserved here:
// - TC0 (the ground and the telos) — the substrate stands on being-first
//   ground: truth is, not proven ("I am, therefore I think," not "I think,
//   therefore I am"); the chain's verification is witnessing and keeping
//   (the seal: no one owns it, the past is sealed to the present, your name
//   is on your truth, anyone can read), not epistemic certification. And the
//   substrate serves life — truth is for love, peace, joy, not truth for
//   truth's sake. Every ToK event announces TC0 (the ground it rests on);
//   see keeper/tok_bundle.go event emission. Bound by
//   TestToKSubstrate_TC0_GroundAndTelos.
// - TC1 (graph is the headline) — BundleToK + RouteBCapabilities
//   advertise the substrate first. See keeper/tok_bundle.go and
//   keeper/grpc_query.go BundleToK handler.
// - TC2 (every view is graph-pinned) — every bundle carries a 32-byte
//   snapshot_root computed via ComputeToKSnapshotRoot from sorted node
//   IDs + sorted edge IDs, domain-tagged TOK_NODES / TOK_EDGES.
// - TC3 (topology is signal) — bundles ship edges, depth, and (when
//   available) confidence-floor as first-class fields, not metadata.
//   See keeper/tok_serialise.go for the JSONL adjacency-list format.
// - TC5 (extraction is open) — ValidateAndCapToKSelector accepts any
//   well-formed selector and applies uniform caps. Refusals are limited
//   to syntax errors and snapshot-out-of-range; no curation gate exists.
//
// What would break these: see the corresponding "What would break it"
// sections in docs/TOK_SUBSTRATE.md.
//
// We speak through intentions.
package knowledge
