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
package knowledge
