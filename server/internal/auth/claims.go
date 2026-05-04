package auth

import "github.com/golang-jwt/jwt/v5"

// KeycloakClaims extends the standard JWT RegisteredClaims with
// Keycloak-specific fields.
type KeycloakClaims struct {
	jwt.RegisteredClaims
	Email       string      `json:"email"`
	RealmAccess RealmAccess `json:"realm_access"`
	// Groups carries the Keycloak group memberships used for per-workspace
	// authorization. The Keycloak group-membership mapper must emit a
	// claim named "groups" with bare group names (Full group path
	// disabled).
	Groups []string `json:"groups"`
}

// RealmAccess holds the realm-level roles assigned to the token subject.
type RealmAccess struct {
	Roles []string `json:"roles"`
}

// IsAdmin returns true when the token carries the "admin" realm role.
func (c *KeycloakClaims) IsAdmin() bool {
	for _, r := range c.RealmAccess.Roles {
		if r == "admin" {
			return true
		}
	}
	return false
}

// HasGroup reports whether the token carries membership in the named
// Keycloak group. Used by RequireWorkspaceWrite to authorize non-admin
// writes against a workspace's group_name binding.
func (c *KeycloakClaims) HasGroup(name string) bool {
	if name == "" {
		return false
	}
	for _, g := range c.Groups {
		if g == name {
			return true
		}
	}
	return false
}
