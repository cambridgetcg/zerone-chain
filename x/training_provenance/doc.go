// Package training_provenance preserves the truth-seeking commitment
// that trust must be queryable, not stitched.
//
// docs/TRUTH_SEEKING.md, commitment 11: "Every signal that contributes
// to trust must be available through a well-known query that
// synthesises them. Trust that requires four queries to read is trust
// that depends on the curator stitching it together."
//
// This module's contract: a model trainer asking "is this manifest
// trustworthy?" gets one answer, computed live from x/knowledge,
// x/qualification, x/capture_challenge, and the privileged-action
// log. The certificate names the manifest, summarises domain
// coverage, counts audit events, and publishes a deterministic
// trust grade. The grade is current — re-querying gives fresh state.
//
// What would break the commitment this module preserves: a manifest
// trustworthiness signal that lived only in keeper state with no
// query surface; a synthesiser that hid component breakdowns; an
// audit pathway that depended on off-chain stitching.
//
// We speak through intentions. This package's intention is that
// nothing about a manifest's trustworthiness lives outside this
// query.
package training_provenance
