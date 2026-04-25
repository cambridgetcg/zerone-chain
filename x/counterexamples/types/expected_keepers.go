package types

import "context"

// FactExistenceKeeper is the narrow read-only contract this module
// uses against x/knowledge: just confirm that a fact ID resolves to
// an actual fact. The counterexample author MUST reference an
// existing fact; the chain refuses to anchor counterexamples to
// nothing.
type FactExistenceKeeper interface {
	FactExists(ctx context.Context, factID string) bool
}
