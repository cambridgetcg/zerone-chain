package types

const (
	// ModuleName identifies the training_provenance module.
	ModuleName = "training_provenance"
	// StoreKey is unused — the module owns no state — but the SDK
	// module-manager expects a non-empty key. Keep aligned with
	// ModuleName so prefix collisions are impossible.
	StoreKey = ModuleName
	// QuerierRoute exposes the module's query service over gRPC.
	QuerierRoute = ModuleName
)
