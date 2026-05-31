package domain

// Role is a publisher-scoped authorization role (ADR 0006). Roles form a
// lattice rather than a linear hierarchy: Viewer ⊂ {Editor, Reviewer} ⊂ Admin.
// Editor and Reviewer are independent — neither implies the other — so by
// default editors and reviewers are different people (separation of duties).
type Role string

const (
	// RoleViewer can read a publisher's private entries (public reads need
	// no role).
	RoleViewer Role = "viewer"
	// RoleEditor can create / edit / submit resources and versions.
	RoleEditor Role = "editor"
	// RoleReviewer can approve / reject submitted versions and pending
	// deletions.
	RoleReviewer Role = "reviewer"
	// RoleAdmin can do everything on the publisher — author/edit/delete, manage
	// metadata and role grants, toggle visibility — EXCEPT approve changes.
	// Approval is reserved to RoleReviewer so that no single per-publisher
	// principal can both author and sign off a change (separation of duties).
	// (The global Server Admin is the one break-glass exception that can
	// approve; that is resolved above the role lattice, not here.)
	RoleAdmin Role = "admin"
)

// ValidRole reports whether s is one of the four publisher-scoped roles.
func ValidRole(s string) bool {
	switch Role(s) {
	case RoleViewer, RoleEditor, RoleReviewer, RoleAdmin:
		return true
	default:
		return false
	}
}

// Satisfies reports whether a principal holding the roles in held meets the
// required capability. The lattice means:
//
//   - viewer is implied by editor, reviewer, and admin (anyone who can write,
//     review, or administer can also read private entries);
//   - admin implies editor (admins author) and admin, but NOT reviewer;
//   - a directly-held role satisfies a requirement for that same role.
//
// Crucially, **reviewer is the sole approver**: neither editor nor admin
// satisfies a reviewer requirement. Admin is the most powerful per-publisher
// role for everything except approval, but it cannot sign off a change — that
// must be a principal holding reviewer, so authoring and approving stay
// separable (separation of duties). Likewise reviewer does not imply editor.
// The global Server Admin break-glass is handled by a middleware short-circuit
// above this lattice, not here.
func Satisfies(held map[Role]bool, required Role) bool {
	switch required {
	case RoleViewer:
		return held[RoleAdmin] || held[RoleViewer] || held[RoleEditor] || held[RoleReviewer]
	case RoleEditor:
		return held[RoleAdmin] || held[RoleEditor]
	case RoleReviewer:
		return held[RoleReviewer]
	case RoleAdmin:
		return held[RoleAdmin]
	default:
		return false
	}
}

// PrincipalType is the kind of subject a role grant is attached to. Only two
// principal types exist (ADR 0006 §4): a user or a group. Service-account /
// API-key principals are future work (ADR 0006 F2).
type PrincipalType string

const (
	PrincipalUser  PrincipalType = "user"
	PrincipalGroup PrincipalType = "group"
)

// ValidPrincipalType reports whether s is a recognised principal type.
func ValidPrincipalType(s string) bool {
	switch PrincipalType(s) {
	case PrincipalUser, PrincipalGroup:
		return true
	default:
		return false
	}
}

// GrantSource records whether a role grant was created through the API or
// seeded from configuration (the reviewer-group seed, ADR 0006 §5). Config
// grants are re-applied on every boot, so deleting one via the API only
// sticks if the seed is also removed.
type GrantSource string

const (
	GrantSourceAPI    GrantSource = "api"
	GrantSourceConfig GrantSource = "config"
)
