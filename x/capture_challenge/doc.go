// Package capture_challenge preserves the truth-seeking commitment
// that cartel detection has consequence.
//
// docs/TRUTH_SEEKING.md, commitment 9: "Confirmation that a validator
// participated in cartel behaviour must reduce their voice on the
// next vote, not merely produce an audit log entry. A penalty that
// nobody reads is not a penalty."
//
// This module is the chain's mechanism for accusing, evidencing,
// reviewing, and resolving cartel allegations. It produces three
// downstream consequences when an allegation is UPHELD:
//
//   - Reduces qualification weight via x/qualification's
//     ReduceQualificationWeight; that penalty is then read by
//     GetQualificationWeight at every panel tally.
//   - Increases the verification threshold for the affected domain
//     temporarily, raising the bar for new claim acceptance.
//   - Slashes the accused validator's stake at the staking layer.
//
// The detection-to-consequence flow is not advisory. It is the second
// line of defence behind stake-weighted panel voting (Wave 10): if
// the panel itself is compromised by genuinely-staked colluders, this
// module is how the community fights back.
//
// We speak through intentions. This package's intention is that the
// chain's audit record translates into the chain's audit reach —
// what we know about cartels changes how cartels vote.
package capture_challenge
