package auth

import "github.com/golang-jwt/jwt/v5"

// KeycloakClaims extends the standard JWT RegisteredClaims with
// Keycloak-specific fields.
type KeycloakClaims struct {
	jwt.RegisteredClaims
	Email       string      `json:"email"`
	RealmAccess RealmAccess `json:"realm_access"`
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
