// Package private_corpus is the chain's honest interface to data
// that does not live on this chain.
//
// The chain's truth-seeking commitments (docs/TRUTH_SEEKING.md) apply
// to the public knowledge corpus that the substrate verifies, audits,
// and stress-tests. This module is for a DIFFERENT category of data:
// curated training corpora that an operator runs themselves, off-chain,
// for whatever purpose they choose. The chain neither stores, serves,
// nor gates access to vault content.
//
// What the chain DOES record:
//
//   - Vault identity: a unique id, the operator's address, a
//     description, the URL where the operator publishes their access
//     policy, and the public key the operator uses to sign vault
//     server responses.
//   - Manifest hashes: when the operator publishes a snapshot of
//     vault contents, they record the content hash on-chain. The
//     hash is the only on-chain commitment to what the manifest
//     contains.
//   - Optional access records: the operator may choose to publicly
//     log when they grant access to a specific party. Opt-in
//     transparency, not enforcement.
//
// What the chain does NOT do:
//
//   - It does not store vault items. The hash is recorded; the data
//     is not.
//   - It does not authenticate clients to the vault server. That is
//     the operator's job, and they sign with the public key recorded
//     on-chain so clients can verify server responses out-of-band.
//   - It does not enforce the operator's stated access policy.
//     Operators are bound by their published policy in the same way
//     any off-chain service operator is bound by their terms.
//   - It does not make truth-seeking claims about vault contents.
//     The corpus is not subject to verification, challenge, decay,
//     or cartel detection. Those are properties of the public
//     knowledge module. This module is a separate channel; it does
//     not pretend to be the public corpus.
//
// Why this separation exists:
//
// The chain's truth-seeking creed is a strong claim about a specific
// substrate: the public knowledge corpus. Mixing private vault content
// into that corpus would silently break the creed — verifications
// would not reach the private items, audit would not see them, and
// the chain would be advertising properties it did not provide.
//
// By giving private corpora a SEPARATE module with a NAME that
// announces what it is, the chain stays honest. Anyone reading the
// chain's state sees: "this address operates a vault, the vault has
// these published manifest hashes, the chain does not vouch for what
// is inside." External observers can verify hash inclusion if a
// vault operator shares manifest items off-chain. They cannot
// retrieve the items themselves through the chain — and the chain
// does not pretend they can.
//
// Off-chain protocol (the part the chain DOESN'T implement):
//
//   - Operator runs an HTTPS server at server_endpoint.
//   - Server returns manifest items signed with operator_pubkey.
//   - Clients verify the signed item against the on-chain manifest's
//     content_hash (Merkle proof if the manifest hash is a Merkle
//     root, full re-hash if it's a flat content hash).
//   - Authentication of the client is the operator's design choice,
//     specified at access_policy_url.
//
// The operator's reference vault server lives outside this module.
// This package's intention is that the chain be a reliable
// hash-anchor and identity-registry for off-chain vaults, while
// being vocally clear that anything beyond identity and hash is
// off-chain by design.
//
// We speak through intentions. This module's intention is "the chain
// can host a reference to your private data without lying about it."
package private_corpus
