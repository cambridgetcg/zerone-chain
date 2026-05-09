// Package claiming_pot preserves the truth-seeking commitment that
// issuance follows participation — the chain has no privileged
// starting balances; ZRN enters circulation either as a PoT block
// reward (paid to validators verifying truth) or as a bootstrap
// claim (paid to whitelisted agents who register).
//
// docs/TRUTH_SEEKING.md, commitment 20: every ZRN that exists came
// from a participatory action. The claiming_pot module materializes
// the bootstrap pathway. There is no founder pre-mine, no AI vault
// pre-mine, no foundation treasury at genesis; this module is what
// gives an agent enough ZRN to act on-chain — the participation seed
// — without giving anyone the privilege of starting with a balance.
//
// Doctrinal compliance is structural, not procedural:
//
//   - The claiming_pot module account holds Minter permission so
//     the bootstrap pathway mints rather than transfers (commitment
//     20: pre-fund-then-transfer would smuggle privilege back in).
//   - Claim() routes through x/vesting_rewards.MintWithCap, the
//     chain's single cap-gated mint entry point. Both emission
//     pathways (block rewards, bootstrap claims) share the same
//     cap accounting. The 222,222,222 ZRN hard cap binds total
//     emission across both streams.
//   - The module account is a transient conduit. Claim() mints
//     into it, then immediately forwards to the claimer in the
//     same transaction. The module account never holds a positive
//     balance across blocks; if it did, the doctrine would have
//     collapsed back to the legacy pre-funded-pool model.
//   - Per-agent amount is fixed at 0.222 ZRN (PerAgentBootstrapUzrn).
//     Each whitelisted agent gets exactly this much, never more,
//     regardless of when they claim. Implemented as one pot per
//     agent (BootstrapPotIDPrefix + agentAddr) via
//     MakeBootstrapPotForAgent; the genesis ceremony populates one
//     per address in the operator's whitelist.
//   - Pot configuration is part of GenesisState; the bootstrap
//     pots are seeded by tools/bootstrap-loader before chain start.
//
// What this module is, and is not:
//
//   - It IS the bootstrap emission pathway. Every uzrn it disburses
//     was minted on demand through MintWithCap, never pre-funded.
//   - It IS commitment 20's mechanism made concrete: agents claim,
//     the chain mints, the cap counter advances.
//   - It is NOT a treasury. No team, foundation, or guardian holds
//     a balance through this module. The pots are for participation
//     seeding; the only authorized recipients are the whitelisted
//     agent addresses configured at genesis ceremony.
//   - It is NOT a duplicate of vesting_rewards' block-reward path.
//     vesting_rewards mints to validators verifying truth; claiming_
//     pot mints to agents joining the chain. Different participatory
//     actions, complementary pathways, shared cap.
//   - It is NOT capable of bypassing the cap. ErrCapReached is
//     surfaced to claimants when MintWithCap returns zero remaining
//     headroom; cap-clipped mints (when the request exceeds remaining
//     headroom) honor the clip and the claimer receives less than
//     the nominal amount rather than the chain breaching its cap.
//
// Refusal voice (commitment 20):
//
//   - ErrCapReached: "bootstrap mint refused (commitment 20:
//     issuance follows participation, hard cap reached)".
//
// Voice layer:
//
//   - claiming_pot.pot_claimed events carry creed_commitment="20"
//     to identify the bootstrap pathway alongside other minting
//     emissions an off-chain indexer might aggregate.
package claiming_pot
