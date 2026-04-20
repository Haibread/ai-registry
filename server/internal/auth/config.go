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
	// Useful when the server can only reach Keycloak via an internal hostname
	// (e.g. http://keycloak:8080/...) while the JWT `iss` uses an external URL.
	// When empty, the JWKS endpoint is derived from OIDCIssuer.
	OIDCJWKSUrl string

	// OIDCClientID is the public OAuth 2.0 client ID for the browser SPA.
	// It is served as-is via GET /config.json so the SPA can bootstrap its
	// OIDC client without baking configuration into the Docker image.
	OIDCClientID string

	// OIDCAudience is the expected `aud` claim value on incoming access tokens.
	// When set, tokens whose `aud` does not contain this value are rejected.
	// Required by the MCP authorization spec (OAuth 2.1 resource indicators /
	// audience binding) to prevent tokens minted for other clients on the same
	// realm from being accepted here. Leave empty only in dev when every token
	// in the realm targets this resource.
	OIDCAudience string

	// AuthStorage is the SPA token-store selector exposed via /config.json:
	// "session" (default, XSS-safer) or "local" (E2E only — localStorage is
	// captured by Playwright's storageState).
	AuthStorage string
}

// JWKSEndpoint returns the URL to fetch Keycloak signing keys from.
// Uses OIDCJWKSUrl when set; otherwise derives from OIDCIssuer.
func (c Config) JWKSEndpoint() string {
	if c.OIDCJWKSUrl != "" {
		return c.OIDCJWKSUrl
	}
	return c.OIDCIssuer + "/protocol/openid-connect/certs"
}
