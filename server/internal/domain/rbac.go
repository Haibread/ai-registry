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
	// RoleAdmin can do everything Editor and Reviewer can, plus edit
	// publisher metadata and assign role grants on the publisher.
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
//   - admin satisfies every requirement;
//   - a directly-held role satisfies a requirement for that same role;
//   - viewer is implied by editor, reviewer, and admin (anyone who can write
//     or review can also read private entries).
//
// Crucially, editor does NOT satisfy a reviewer requirement and reviewer does
// NOT satisfy an editor requirement — that independence is the separation of
// duties. Holding both is the only way one principal can author and approve
// the same change.
func Satisfies(held map[Role]bool, required Role) bool {
	if held[RoleAdmin] {
		return true
	}
	switch required {
	case RoleViewer:
		return held[RoleViewer] || held[RoleEditor] || held[RoleReviewer]
	case RoleEditor:
		return held[RoleEditor]
	case RoleReviewer:
		return held[RoleReviewer]
	case RoleAdmin:
		return false // admin already short-circuited above
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
