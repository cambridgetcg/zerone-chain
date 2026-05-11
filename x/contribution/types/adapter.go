package types

import "context"

// ContributionAdapter is the per-class plug-in contract. The keeper
// dispatches to the registered adapter based on Contribution.class.
//
// Adapters are stateless dispatchers: they read the Contribution and
// the world (their owning module's keeper, plus any cross-module
// readers) and return a verdict. The orchestrator keeper is the only
// writer of x/contribution state.
type ContributionAdapter interface {
	// Class returns the ContributionClass this adapter handles.
	Class() ContributionClass

	// Classify is called at Stage ②. Checks payload shape,
	// (class, phase) coherence, and contributor qualification.
	// Returns nil on success; typed error on CLASSIFICATION_FAILED.
	Classify(ctx context.Context, c *Contribution) error

	// SubstrateLink is called at Stage ② (after Classify succeeds)
	// to compute the M2 substrate-link weight L (BPS, 0..10_000).
	// Zero L blocks the reward path (M4 enforces R=0 when L=0).
	SubstrateLink(ctx context.Context, c *Contribution) (uint32, error)

	// Verify is called at Stage ③. Returns verification_score in BPS
	// (0..1_000_000) and an optional error. Score >= MinVerificationScoreBps
	// + nil error → STATUS_VERIFIED. Otherwise → STATUS_VERIFICATION_FAILED.
	Verify(ctx context.Context, c *Contribution) (uint32, error)
}

// AdapterRegistry maps ContributionClass → ContributionAdapter.
// Built in-memory at app init; not persisted.
type AdapterRegistry map[ContributionClass]ContributionAdapter

// NewAdapterRegistry constructs an empty registry.
func NewAdapterRegistry() AdapterRegistry {
	return AdapterRegistry{}
}

// Get returns the adapter registered for a class, or (nil, false).
func (r AdapterRegistry) Get(class ContributionClass) (ContributionAdapter, bool) {
	a, ok := r[class]
	return a, ok
}

// Register adds an adapter to the registry, keyed by its declared Class().
// Panics on duplicate registration of the same class — registration is
// app-init only and a duplicate indicates a wiring bug.
func (r AdapterRegistry) Register(a ContributionAdapter) {
	if _, exists := r[a.Class()]; exists {
		panic("contribution: adapter already registered for class " + a.Class().String())
	}
	r[a.Class()] = a
}
