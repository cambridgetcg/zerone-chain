package types

// CanonicalCommitments is the canonical name-by-number registry
// of the chain's commitments at the time this binary was built.
// Operators authoring genesis.json should use this list (via
// BuildGenesisCreed) so the on-chain Genesis Creed matches the
// docs/TRUTH_SEEKING.md the binary is shipping.
//
// To add a new commitment:
//  1. Add the section to docs/TRUTH_SEEKING.md (with **Echoes**
//     line citing other commitments it depends on).
//  2. Bump .creed-hash to the new sha256 of the normalized file.
//  3. Append the (number, name) pair to the slice below.
//  4. Add a binding `// Commitment N:` banner test in
//     tests/cross_stack/truth_seeking_invariants_test.go.
//  5. Add a doc.go citation in some x/<module>/doc.go.
//  6. (optional) Tag at least one event with `creed_commitment="N"`.
//
// The TestTruthSeeking_CreedAndContractStayInSync meta-test catches
// a step omitted from this list.
var CanonicalCommitments = []struct {
	Number uint32
	Name   string
}{
	{1, "Methodology over statement"},
	{2, "Is-ought wall"},
	{3, "Popper, not popularity"},
	{4, "The substrate stress-tests its own truth"},
	{5, "The chain manufactures probe demand"},
	{6, "No individual can unilaterally inject truth"},
	{7, "Skill is current, not historical"},
	{8, "The panel weights skill, not bond"},
	{9, "Cartel detection has consequence"},
	{10, "Forward-only audit"},
	{11, "Trust is queryable"},
	{12, "The chain pays for its own audit"},
	{13, "The training corpus is not for sale"},
	{14, "Reasoning traces are first-class"},
	{15, "Counterexamples are part of the corpus"},
	{16, "The chain pays for exploration of the unknown"},
	{17, "Disagreement is structure, not noise"},
	{18, "The chain manufactures exploration demand"},
	{19, "The creed is governance-gated"},
	{20, "Issuance follows participation"},
}

// BuildGenesisCreed materializes the canonical commitment list
// into a v1 PinnedCreed for use in GenesisState.GenesisPin. The
// canonical_hash MUST match the actual sha256 of the normalized
// docs/TRUTH_SEEKING.md the binary is shipping; the
// TestTruthSeeking_GenesisCreedReflectsCurrentTruthSeeking
// invariant test catches drift between the two.
//
// IntroducedViaLip is intentionally empty for genesis-installed
// commitments — no LIP precedes genesis, and post-genesis
// amendments must cite the LIP that authorized them.
func BuildGenesisCreed(canonicalHash []byte, atHeight uint64) *PinnedCreed {
	cs := make([]*CommitmentEntry, 0, len(CanonicalCommitments))
	for _, c := range CanonicalCommitments {
		cs = append(cs, &CommitmentEntry{
			Number:             c.Number,
			Name:               c.Name,
			IntroducedAtHeight: atHeight,
		})
	}
	return &PinnedCreed{
		Version:        1,
		CanonicalHash:  canonicalHash,
		PinnedAtHeight: atHeight,
		Commitments:    cs,
	}
}
