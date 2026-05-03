package domain

import "time"

// ReviewState captures the change-approval workflow position of a version
// row. See ADR 0003. The column is orthogonal to the existing version
// `Status` and `PublishedAt` fields — a non-`none` review state always
// implies `PublishedAt IS NULL` (enforced by a CHECK constraint).
type ReviewState string

const (
	ReviewStateNone           ReviewState = "none"
	ReviewStatePendingReview  ReviewState = "pending_review"
	ReviewStateRejected       ReviewState = "rejected"
)

// ReviewDecision is the terminal value written to a version's
// `review_decision` column when an approver acts. Empty string while the
// row has never been reviewed.
type ReviewDecision string

const (
	ReviewDecisionApproved ReviewDecision = "approved"
	ReviewDecisionRejected ReviewDecision = "rejected"
)

// ReviewMetadata bundles the audit columns added by the
// change-approval workflow. Populated lazily — every field is the zero
// value on a freshly-created Draft version.
type ReviewMetadata struct {
	State            ReviewState
	Revision         int
	SubmittedAt      *time.Time
	SubmittedBy      string // JWT subject (UUID)
	SubmittedByEmail string // denormalised at action time (ADR 0003)
	ReviewedAt       *time.Time
	ReviewedBy       string
	ReviewedByEmail  string
	Decision         ReviewDecision
	RejectionReason  string
}
