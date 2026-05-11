package types

import "context"

// ContributionWrapper is the post-init-injected interface x/work_creed
// uses to record privileged actions (sub-creed pin writes) as
// Substrate-class Contributions in x/contribution.
//
// Layer 2 of the recursion stack: the same chain that orchestrates
// useful-work Contributions records its own doctrinal amendments as
// Contributions reviewed by itself. UW commitment: ZERONE is recursive.
//
// The interface mirrors keeper.Keeper.WrapAsSubstrateContribution
// signature so that x/contribution.Keeper satisfies it directly without
// an adapter. The dependency is wired post-init (in app.go) because
// x/contribution depends on x/work_creed via the contribution adapter
// path and a direct keeper import here would create a cycle.
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
