// Package counterexamples preserves the truth-seeking commitment that
// the training corpus must include not just what is true, but what
// is wrong AND WHY.
//
// docs/TRUTH_SEEKING.md, commitment 15: a model trained on conclusions
// alone learns the predictor; a model trained on conclusions paired
// with their structured negations learns the discriminator.
// Discrimination is the cognitive primitive that lets a model resist
// manipulation rather than absorb it. The chain therefore treats
// counterexamples as first-class corpus material — proposed, audited,
// and rewarded just like facts themselves.
//
// What a Counterexample is:
//
//   - A pairing (fact_id, wrong_claim, error_type, reasoning) that
//     teaches the discriminator: "this would seem to follow but is
//     wrong because <reasoning>."
//   - Optionally tagged with the methodology IDs whose mis-application
//     would yield the wrong answer (commitment 1: methodology over
//     statement).
//   - Auditable: any qualified validator may affirm or reject.
//     Resolution requires a supermajority (default 66.6%) of at least
//     min_votes (default 3).
//   - Economically encouraged: a validated counterexample's bond is
//     returned plus a reward; rejected counterexamples burn bond.
//     The reward exceeds the bond at the margin so the chain
//     ECONOMICALLY ENCOURAGES contribution — alignment-by-structure
//     is a public good the chain pays for.
//
// What a Counterexample is NOT:
//
//   - A challenge to the parent fact. The fact stays true; the
//     counterexample is the structural negation that helps a model
//     learn to tell the two apart. Challenges to facts go through
//     x/knowledge's existing adversarial verification.
//   - A guarantee of alignment. Sufficiently determined adversaries
//     can fine-tune away whatever a corpus instills. What
//     counterexamples DO is raise the cost of misuse and lower the
//     cost of aligned use: the default trajectory through the corpus
//     produces a model that knows what wrong answers look like.
//
// Integration with x/knowledge:
//
// ComputeTrainingValueWeight reads HasValidatedCounterexample(fact_id)
// via the FactCounterexampleAdapter. Facts with at least one validated
// counterexample receive a TVW multiplier (default 1.2x) — meaning
// the chain pays meaningfully more for facts that come with
// alignment-supporting structure than for bare facts.
//
// Without this gate, "we believe in counterexamples" would be a
// slogan. With this gate, the corpus's training value weight
// itself enforces the structure: training pipelines preferentially
// consume facts with counterexample coverage, and contributors
// preferentially supply them.
//
// We speak through intentions. This package's intention is that
// "the training corpus" means "facts AND their structured negations,"
// because anything less produces models that know what is true
// without knowing what wrong looks like.
package counterexamples
