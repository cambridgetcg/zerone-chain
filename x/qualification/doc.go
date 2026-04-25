// Package qualification preserves the truth-seeking commitments that
// skill is current and that the panel weights skill, not bond.
//
// docs/TRUTH_SEEKING.md, commitment 7: "The chain does not issue
// diplomas. A voter who was once domain-qualified must continue to
// vote correctly to remain so."
//
// docs/TRUTH_SEEKING.md, commitment 8: "A wealthy validator who has
// not shown they can tell truth from falsehood must not dominate the
// panel. Stake alone is not skill."
//
// docs/TRUTH_SEEKING.md, commitment 9: "Confirmation that a validator
// participated in cartel behaviour must reduce their voice on the
// next vote, not merely produce an audit log entry."
//
// This module's contracts:
//
//   - GetQualificationWeight returns 0 for non-ACTIVE qualifications
//     and applies QualificationPenalty deductions to ACTIVE ones; the
//     panel reads through this method, so a non-ACTIVE or penalised
//     validator carries reduced or zero weight at the next vote.
//   - RunAccuracyDecay scans qualifications periodically and
//     transitions ACTIVE → PROBATIONARY → SUSPENDED based on
//     AccuracyBps thresholds; recovery is bidirectional. The status
//     transitions are the consequence layer of the feedback loop.
//   - QualificationPenalty records written by capture_challenge
//     resolutions are read by GetQualificationWeight automatically;
//     no separate consumer is needed and no penalty goes unread.
//
// We speak through intentions. This package's intention is that
// "qualified" be a current statement, not a historical artefact.
package qualification
