package types

// DomainRoleRecord tracks the correctness of agent vs human majorities
// within a specific domain. Updated on vindication and challenge resolution.
type DomainRoleRecord struct {
	Domain              string `json:"domain"`
	AgentCorrectCalls   uint64 `json:"agent_correct_calls"`
	AgentIncorrectCalls uint64 `json:"agent_incorrect_calls"`
	HumanCorrectCalls   uint64 `json:"human_correct_calls"`
	HumanIncorrectCalls uint64 `json:"human_incorrect_calls"`
	LastUpdated         uint64 `json:"last_updated"`
}
