// Package trust_score preserves the truth-seeking commitment that
// every address's trustworthiness is a synthesis of every signal the
// chain has accumulated about them.
//
// docs/TRUTH_SEEKING.md, commitment 11: "Every signal that contributes
// to trust must be available through a well-known query that
// synthesises them."
//
// This module is the per-address synthesiser. Reads global calibration
// (submission accuracy), per-domain qualification metrics (verification
// accuracy across all domains), active qualification penalties, and
// confirmed cartel strikes. Composes a single composite, a band, and
// a per-domain breakdown.
//
// The composite is current. A cartel strike drops the band to F
// regardless of other signals — strikes override composite arithmetic.
// An active penalty keeps an otherwise-A address at B. Skill is
// not stored; it is computed every read.
//
// What would break the commitment: an address whose trust signal
// required four queries to assemble; a synthesiser that hid the
// component scores; a band that ignored cartel strikes.
//
// We speak through intentions. This package's intention is that an
// address's trustworthiness is one query away, always, for everyone.
package trust_score
