package auth

import "github.com/golang-jwt/jwt/v5"

// KeycloakClaims extends the standard JWT RegisteredClaims with
// Keycloak-specific fields.
type KeycloakClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	// EmailVerified gates the federated bind-once path: a first
	// OIDC login only binds onto a pre-invited local row when the token's email
	// is verified, never on an unverified one.
	EmailVerified bool        `json:"email_verified"`
	RealmAccess   RealmAccess `json:"realm_access"`
	// Groups carries the Keycloak group memberships. Authorization resolves
	// these to in-registry groups and their role grants. The
	// Keycloak group-membership mapper must emit a claim named "groups"
	// with bare group names (Full group path disabled).
	Groups []string `json:"groups"`
	// Nonce echoes the value the broker sent on the authorize request. The
	// callback rejects an id_token whose nonce does not match the one bound to
	// the login transaction (replay protection). Present only on id_tokens.
	Nonce string `json:"nonce"`
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
// Keycloak group. Authorization is grant-based — claims carry
// group membership only — so this is a claim-parsing accessor.
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
