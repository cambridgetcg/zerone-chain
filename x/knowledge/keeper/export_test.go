package keeper

// Export unexported functions for external test package (keeper_test).
var (
	Normalize = normalize
	ParseUzrn = parseUzrn
)
