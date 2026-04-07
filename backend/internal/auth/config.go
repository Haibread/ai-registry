// Package auth provides JWT validation middleware for Keycloak-issued tokens.
// Admin check: JWT payload field realm_access.roles must contain "admin".
package auth

// Config holds OIDC/Keycloak settings.
type Config struct {
	// OIDCIssuer is the full Keycloak realm issuer URL, e.g.
	// http://keycloak:8080/realms/registry
	// The JWKS endpoint is derived as {OIDCIssuer}/protocol/openid-connect/certs
	OIDCIssuer string
}

// JWKSEndpoint returns the Keycloak JWKS URL for the configured realm.
func (c Config) JWKSEndpoint() string {
	return c.OIDCIssuer + "/protocol/openid-connect/certs"
}
