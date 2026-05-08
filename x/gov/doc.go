// Package gov is Zerone's Living Improvement Proposal (LIP) system —
// the chain's governance surface for parameter amendments, research
// fund spend approvals, election of governance seats, and attached
// upgrade plans. LIPs progress through staged voting (submit, stake,
// advance, cast, withdraw) with vote weight read from x/qualification
// rather than raw bond.
//
// Truth-seeking position:
//
// Recent commits established a load-bearing principle for the chain:
// param defaults are EXPRESSIONS OF BELIEF, not tuning. Default
// values for truth-load-bearing modules carry intention comments
// naming the commitment a value preserves. Governance is therefore
// not a generic parameter-knob mechanism — it is the path through
// which the chain's beliefs about itself can be amended. That makes
// gov a truth-handling module by structure, not just by adjacency.
//
// docs/TRUTH_SEEKING.md, commitment 10 (forward-only audit): "Every
// privileged action is logged; the chain's history is append-only
// and verifiable." LIPs that amend truth-load-bearing parameters are
// privileged actions in the strongest sense — they alter the
// chain's stated beliefs about how it should operate. The full LIP
// lifecycle (submit → stake → advance → cast → resolve → execute)
// is recorded forward-only; a passed LIP cannot be retroactively
// rewritten to look unpassed, and a failed LIP cannot be silently
// re-cast as having succeeded.
//
// docs/TRUTH_SEEKING.md, commitment 11 (trust is queryable): the
// chain's governance posture — open LIPs, recent passes, current
// vote tallies, research-spend approvals, attached upgrade plans —
// must be readable as a structured surface, not stitched together
// from disjoint per-LIP queries. x/governance_synthesis composes the
// system-level view; this module is the upstream that produces the
// raw governance signals that synthesis reads. Without legible
// governance, "the chain pays for its own audit" (commitment 12) is
// a slogan: someone has to be able to see WHO authorised the audit
// budget that funds the probes.
//
// What this module is, and is not:
//
//   - It IS the parameter-amendment substrate. Truth-load-bearing
//     params (probe stake scaling, qualification thresholds, audit
//     budget mints) flow through here when amended.
//   - It IS the research-spend authorisation surface. Movement of
//     uzrn out of the research fund requires both human-side and
//     AI-side governance approval — the structural form of the
//     human/AI co-governance promise the Truth Paper makes.
//   - It is NOT a venue for adopting beliefs without record. Every
//     LIP carries its provenance; every vote is preserved; every
//     resolution is dated and signed at the consensus layer.
//
// Integration with the truth-seeking spine:
//
//   - x/governance_synthesis reads gov state to compose the
//     system-level trust surface (commitment 11).
//   - x/knowledge / x/qualification / x/autopoiesis / x/alignment
//     all expose UpdateParams handlers; gov is the path through
//     which authority-gated amendments to those modules are made.
//     The "param defaults are expressions of belief" stance applies
//     to those amendments here.
//
// We speak through intentions. This package's intention is that
// "the chain's beliefs are amendable, but never silently" — every
// change to what the chain thinks it owes truth-seeking lands as a
// dated, queryable, forward-only LIP record.
package gov
