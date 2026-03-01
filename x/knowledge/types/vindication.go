package types

// VindicationEntry tracks a minority voter who was slashed and may be vindicated.
// Stored as JSON under VindicationPendingPrefix.
type VindicationEntry struct {
	Verifier    string `json:"verifier"`
	Vote        string `json:"vote"`
	SlashAmount string `json:"slash_amount"` // string for big.Int compat
	SlashBps    uint64 `json:"slash_bps"`
	RoundId     string `json:"round_id"`
	FactId      string `json:"fact_id"`
	Height      uint64 `json:"height"`
}

// VindicationRecord is an immutable record of an executed vindication.
// Stored as JSON under VindicationRecordPrefix.
type VindicationRecord struct {
	Verifier     string `json:"verifier"`
	FactId       string `json:"fact_id"`
	RefundAmount string `json:"refund_amount"`
	BonusAmount  string `json:"bonus_amount"`
	VindicatedAt uint64 `json:"vindicated_at"`
	DisprovenBy  string `json:"disproven_by"`
	RoundId      string `json:"round_id"`
}

// VindicationEscrowModuleName is the module account that holds slashed minority tokens
// until vindication fires or the window expires.
const VindicationEscrowModuleName = "vindication_escrow"
