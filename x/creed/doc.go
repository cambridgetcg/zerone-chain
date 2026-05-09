// Package creed anchors the chain's canonical voice on chain.
// `docs/TRUTH_SEEKING.md` is the human-facing creed; this module is
// the chain's record of which version it is currently bound to and
// the full forward-only history of how it got here.
//
// Truth-seeking position:
//
// docs/TRUTH_SEEKING.md, commitment 6 (no individual can unilaterally
// inject truth): the original commitment binds AT THE FACT LAYER.
// This module extends the same shape ONE LAYER UP — the chain's
// stated beliefs themselves cannot be silently amended. Pre-launch,
// a single authority key gates AnchorPin; post-launch, the
// authority is the gov module account and amendments flow through
// a Creed Amendment LIP whose passage requires both human-side and
// AI-side voter quorum. The asymmetry that previously protected
// the corpus now also protects the document that tells the chain
// how to protect the corpus.
//
// docs/TRUTH_SEEKING.md, commitment 10 (forward-only audit): pins
// are append-only by monotonic version. A new version archives the
// previous one — both remain queryable. A creed amendment cannot
// rewrite history to make a previously-pinned version look
// different now; it can only land a new version that supersedes.
// The chain's record of which creed it has stood on is part of
// its permanent audit trail.
//
// What this module is, and is not:
//
//   - It IS the structural protection against silent creed
//     amendment. With it, the canonical TRUTH_SEEKING.md file's
//     hash is on chain, and a CI check fails any build whose
//     normalized creed file does not match the pinned hash.
//   - It IS the per-commitment registry. CommitmentEntry binds
//     each numbered commitment to the LIP that introduced it (or
//     last amended it), so even a hash-stable amendment that
//     subtly redefines commitment N must produce a new
//     CommitmentEntry version — no hidden semantic drift.
//   - It is NOT a replacement for the markdown creed itself. The
//     human-facing text remains in `docs/TRUTH_SEEKING.md`. This
//     module records WHICH version of that file the chain pins
//     to, not the text itself. The two are linked by the canonical
//     hash, which the off-chain `scripts/check_creed_hash.sh`
//     verifies pre-merge and the chain's own CI verifies pre-build.
//
// Integration with the truth-seeking spine:
//
//   - Pre-launch and pre-LIP-class: AnchorPin is authority-gated to
//     the gov module account, and direct_anchor_enabled defaults to
//     true so genesis can pin and emergency one-off corrections are
//     possible.
//   - Once x/gov.CategoryCreedAmendment ships, params flips
//     direct_anchor_enabled to false. From that point on, every
//     pin must cite a passed Creed Amendment LIP via SourceLip,
//     and the chain refuses any pin that doesn't.
//   - x/governance_synthesis can compose a creed-drift signal
//     (commitment 11): the per-block delta between current pin's
//     commitment registry and the Genesis Creed becomes a publicly
//     queryable measure of "how far has the chain's stated voice
//     moved from where it started."
//
// We speak through intentions. This package's intention is that
// "the chain's voice cannot drift faster than its governance" —
// the same shape as commitment 6 applied to the layer above the
// corpus.
package creed
