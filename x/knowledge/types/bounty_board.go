package types

// ─── Bounty Board Events & Attributes (R47) ────────────────────────────────

const (
	EventBountySubmission = "bounty_submission"
	EventBountyPoolAdded  = "bounty_pool_added"
	EventBountyResolved   = "bounty_resolved"
	EventBountyCancelled  = "bounty_cancelled"
	EventBountyExpired    = "bounty_expired"

	AttributeBountyID     = "bounty_id"
	AttributeSubmissionID = "submission_id"
	AttributeSampleID     = "sample_id"
)
