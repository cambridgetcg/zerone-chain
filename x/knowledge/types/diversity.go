package types

// RoundDiversity stores per-round vote entropy and raw headcounts.
type RoundDiversity struct {
	RoundID     string `json:"round_id"`
	Entropy     uint64 `json:"entropy"`       // BPS: 0 = unanimous, 1_000_000 = 50/50 split
	AcceptCount uint64 `json:"accept_count"`  // raw headcount
	RejectCount uint64 `json:"reject_count"`  // raw headcount
	TotalVoters uint64 `json:"total_voters"`
	Domain      string `json:"domain"`
	Epoch       uint64 `json:"epoch"`
}

// DomainDiversityScore stores per-domain, per-epoch aggregated diversity.
type DomainDiversityScore struct {
	Domain         string `json:"domain"`
	Epoch          uint64 `json:"epoch"`
	AvgEntropy     uint64 `json:"avg_entropy"`     // BPS average across rounds
	RoundCount     uint64 `json:"round_count"`
	UnanimousCount uint64 `json:"unanimous_count"` // rounds with entropy = 0
}

// ValidatorIndependence tracks how often a validator dissents from the majority.
type ValidatorIndependence struct {
	Validator     string `json:"validator"`
	TotalVotes    uint64 `json:"total_votes"`
	MinorityVotes uint64 `json:"minority_votes"`
	LastEpoch     uint64 `json:"last_epoch"`
}

// ConformityStreak tracks consecutive low-diversity epochs for a domain.
type ConformityStreak struct {
	Domain            string `json:"domain"`
	ConsecutiveEpochs uint64 `json:"consecutive_epochs"`
	LastEpoch         uint64 `json:"last_epoch"`
}
