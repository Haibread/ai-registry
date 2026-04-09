// Package auth provides JWT validation middleware for Keycloak-issued tokens.
// Admin check: JWT payload field realm_access.roles must contain "admin".
package auth

// Config holds OIDC/Keycloak settings.
type Config struct {
	// OIDCIssuer is the issuer URL that appears in JWT `iss` claims, e.g.
	// http://localhost:8080/realms/ai-registry (the browser-visible URL).
	// This is validated against the token's `iss` claim.
	OIDCIssuer string

	// OIDCJWKSUrl overrides the JWKS endpoint used to fetch signing keys.
	// Useful when the backend can only reach Keycloak via an internal hostname
	// (e.g. http://keycloak:8080/...) while the JWT `iss` uses an external URL.
	// When empty, the JWKS endpoint is derived from OIDCIssuer.
	OIDCJWKSUrl string
}

// JWKSEndpoint returns the URL to fetch Keycloak signing keys from.
// Uses OIDCJWKSUrl when set; otherwise derives from OIDCIssuer.
func (c Config) JWKSEndpoint() string {
	if c.OIDCJWKSUrl != "" {
		return c.OIDCJWKSUrl
	}
	return c.OIDCIssuer + "/protocol/openid-connect/certs"
}
