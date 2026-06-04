package auth

import "github.com/golang-jwt/jwt/v5"

// OIDCClaims is the identity the registry resolves from an IdP token and then
// carries through the request context. Only `nonce` (a value the broker itself
// generated) and the standard `sub` are read straight off the wire by tag.
// Email and group memberships are NOT bound to fixed tags: their claim names
// are not standardised across IdPs, so the broker reads them from the
// configurable EmailClaim / GroupsClaim paths and stores the results here. The
// Server-Admin decision is likewise resolved elsewhere and lives on Principal,
// never on this type.
type OIDCClaims struct {
	jwt.RegisteredClaims
	// Email is the resolved email. The broker reads it from the configurable
	// EmailClaim path and stores it here; it is never decoded from a fixed wire
	// tag, so it has none.
	Email string `json:"-"`
	// Groups is the resolved group memberships. The broker reads them from the
	// configurable GroupsClaim path (the claim name varies by IdP — `groups`,
	// `roles`, `memberOf`, …) and stores the result here; this field is never
	// decoded directly from the wire, so it has no json tag.
	Groups []string `json:"-"`
	// Nonce echoes the value the broker sent on the authorize request. The
	// callback rejects an id_token whose nonce does not match the one bound to
	// the login transaction (replay protection). Present only on id_tokens.
	Nonce string `json:"nonce"`
}

// HasGroup reports whether the token carries membership in the named group.
// Authorization is grant-based — claims carry group membership only — so this
// is a claim-parsing accessor.
func (c *OIDCClaims) HasGroup(name string) bool {
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
