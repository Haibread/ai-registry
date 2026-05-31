package domain_test

import (
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
)

func roleSet(roles ...domain.Role) map[domain.Role]bool {
	m := make(map[domain.Role]bool, len(roles))
	for _, r := range roles {
		m[r] = true
	}
	return m
}

// TestSatisfies pins the authorization lattice: Viewer is implied by any higher
// role; Editor and Reviewer are INDEPENDENT (neither implies the other — the
// separation of duties); Admin implies Editor and Viewer but NOT Reviewer
// (Reviewer is the sole approver). The negative cases that matter most are
// "Reviewer alone cannot write", "Editor alone cannot approve", and "Admin
// cannot approve".
func TestSatisfies(t *testing.T) {
	tests := []struct {
		name     string
		held     map[domain.Role]bool
		required domain.Role
		want     bool
	}{
		{"no roles can't view", roleSet(), domain.RoleViewer, false},
		{"viewer can view", roleSet(domain.RoleViewer), domain.RoleViewer, true},
		{"editor implies viewer", roleSet(domain.RoleEditor), domain.RoleViewer, true},
		{"reviewer implies viewer", roleSet(domain.RoleReviewer), domain.RoleViewer, true},
		{"admin implies viewer", roleSet(domain.RoleAdmin), domain.RoleViewer, true},

		{"editor can edit", roleSet(domain.RoleEditor), domain.RoleEditor, true},
		{"reviewer alone cannot edit", roleSet(domain.RoleReviewer), domain.RoleEditor, false},
		{"viewer alone cannot edit", roleSet(domain.RoleViewer), domain.RoleEditor, false},
		{"admin can edit", roleSet(domain.RoleAdmin), domain.RoleEditor, true},

		{"reviewer can review", roleSet(domain.RoleReviewer), domain.RoleReviewer, true},
		{"editor alone cannot review", roleSet(domain.RoleEditor), domain.RoleReviewer, false},
		{"admin cannot review (reviewer is the sole approver)", roleSet(domain.RoleAdmin), domain.RoleReviewer, false},
		{"admin+reviewer can review", roleSet(domain.RoleAdmin, domain.RoleReviewer), domain.RoleReviewer, true},

		{"editor cannot admin", roleSet(domain.RoleEditor), domain.RoleAdmin, false},
		{"reviewer cannot admin", roleSet(domain.RoleReviewer), domain.RoleAdmin, false},
		{"admin can admin", roleSet(domain.RoleAdmin), domain.RoleAdmin, true},

		{"editor+reviewer can both edit and review", roleSet(domain.RoleEditor, domain.RoleReviewer), domain.RoleEditor, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.Satisfies(tt.held, tt.required); got != tt.want {
				t.Errorf("Satisfies(%v, %q) = %v, want %v", tt.held, tt.required, got, tt.want)
			}
		})
	}
}

// TestSatisfies_BothEditorAndReviewer confirms that holding both roles is the
// only way one principal can author AND approve the same change.
func TestSatisfies_BothEditorAndReviewer(t *testing.T) {
	both := roleSet(domain.RoleEditor, domain.RoleReviewer)
	if !domain.Satisfies(both, domain.RoleEditor) || !domain.Satisfies(both, domain.RoleReviewer) {
		t.Error("a principal with both Editor and Reviewer must satisfy both capabilities")
	}
}

func TestValidRole(t *testing.T) {
	for _, r := range []string{"viewer", "editor", "reviewer", "admin"} {
		if !domain.ValidRole(r) {
			t.Errorf("ValidRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"", "Admin", "owner", "superuser"} {
		if domain.ValidRole(r) {
			t.Errorf("ValidRole(%q) = true, want false", r)
		}
	}
}

func TestValidPrincipalType(t *testing.T) {
	if !domain.ValidPrincipalType("user") || !domain.ValidPrincipalType("group") {
		t.Error("user and group must be valid principal types")
	}
	for _, p := range []string{"", "service", "apikey", "User"} {
		if domain.ValidPrincipalType(p) {
			t.Errorf("ValidPrincipalType(%q) = true, want false", p)
		}
	}
}
