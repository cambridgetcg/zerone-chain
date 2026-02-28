package types

// QueryGlobalPacingRequest is the request for GlobalPacing query.
type QueryGlobalPacingRequest struct{}

// ModulePacingEffect describes how pacing affects one module parameter.
type ModulePacingEffect struct {
	Module         string `json:"module"`
	Parameter      string `json:"parameter"`
	BaseValue      uint64 `json:"base_value"`
	EffectiveValue uint64 `json:"effective_value"`
}

// QueryGlobalPacingResponse is the response for GlobalPacing query.
type QueryGlobalPacingResponse struct {
	HealthCategory        string                `json:"health_category"`
	CreationMultiplierBps uint64                `json:"creation_multiplier_bps"`
	AnalysisMultiplierBps uint64                `json:"analysis_multiplier_bps"`
	AffectedModules       []*ModulePacingEffect `json:"affected_modules"`
}
