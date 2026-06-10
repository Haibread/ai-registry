package domain

import "time"

// EntryResourceType discriminates which entry table an entry-change request
// targets. The change-request table carries no DB-level FK, so the store
// resolves the entry against the right table using this value.
type EntryResourceType string

const (
	EntryResourceMCPServer EntryResourceType = "mcp_server"
	EntryResourceAgent     EntryResourceType = "agent"
)

// EntryChangeAction names the entry-level mutation a change request will apply
// on approval. Each action implies a payload shape (see the store's apply
// dispatch): visibility -> {visibility}, deprecation -> {}
// (published->deprecated), undeprecation -> {} (deprecated->published),
// metadata_edit -> the entry's editable fields.
type EntryChangeAction string

const (
	EntryChangeVisibility    EntryChangeAction = "visibility"
	EntryChangeDeprecation   EntryChangeAction = "deprecation"
	EntryChangeUndeprecation EntryChangeAction = "undeprecation"
	EntryChangeMetadataEdit  EntryChangeAction = "metadata_edit"
)

// EntryChangeState is the change-approval workflow position of a change
// request. Unlike a version's ReviewState it has a terminal 'approved' value
// because the request row is the record of the decision, not the entry itself.
type EntryChangeState string

const (
	EntryChangePending  EntryChangeState = "pending_review"
	EntryChangeApproved EntryChangeState = "approved"
	EntryChangeRejected EntryChangeState = "rejected"
)

// EntryChangeRequest is one proposed entry-level mutation awaiting (or having
// received) a review decision. Payload is the raw JSON applied on approval;
// callers decode it per Action.
type EntryChangeRequest struct {
	ID               string
	ResourceType     EntryResourceType
	EntryID          string
	Action           EntryChangeAction
	Payload          []byte
	State            EntryChangeState
	Revision         int
	SubmittedAt      time.Time
	SubmittedBy      string
	SubmittedByEmail string
	ReviewedAt       *time.Time
	ReviewedBy       string
	ReviewedByEmail  string
	Decision         ReviewDecision
	RejectionReason  string
}
