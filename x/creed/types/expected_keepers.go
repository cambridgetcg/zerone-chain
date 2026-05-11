package types

import "context"

// ContributionWrapper is the post-init-injected interface x/creed uses
// to record privileged creed amendments (canonical pin writes) as
// Substrate-class Contributions in x/contribution.
//
// Layer 2 of the recursion stack: the chain records its own canonical
// truth-seeking creed amendments as Contributions reviewed by itself.
// UW commitment: ZERONE is recursive — the chain pays for its own
// doctrinal evolution, structurally.
//
// The interface mirrors x/contribution.keeper.Keeper.WrapAsSubstrate
// Contribution so that the contribution keeper satisfies it directly.
// Dependency is wired post-init in app.go because x/contribution
// imports x/creed (the contribution adapter reads the creed pin) and
// a direct import here would form a cycle.
//
// Phase 1: defensive nil-check in callers — the wrapper is optional
// during early app init paths and tests. Phase 6: wrapper is required;
// pin writes that fail to wrap reject the action.
type ContributionWrapper interface {
	WrapAsSubstrateContribution(
		ctx context.Context,
		subClass string,
		actor string,
		description []byte,
		parentContributionID []byte,
	) ([]byte, error)
}
