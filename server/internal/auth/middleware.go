package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/haibread/ai-registry/internal/problem"
)

type contextKey string

const (
	claimsKey  contextKey = "auth_claims"
	isAdminKey contextKey = "auth_is_admin"
)

// ServiceTokenVerifier validates an IdP-issued (e.g. Keycloak service-account)
// access token presented directly on the API and maps it to identity plus the
// authorization inputs (groups, Server-Admin flag). It is the M2M counterpart to
// the brokered browser login. *OIDCBroker satisfies it; it is wired only when
// OIDC_AUDIENCE is configured, so a nil verifier disables the path.
type ServiceTokenVerifier interface {
	VerifyServiceToken(ctx context.Context, accessToken string) (*ServiceIdentity, error)
}

// Authenticator validates a bearer token and populates the request context with
// the resolved Principal. The primary, hot-path credential is a registry-issued
// access token (verified by pure crypto, no DB). When an IdP service-token
// verifier is configured, a bearer that is NOT a registry token is given a
// second chance as a directly-presented IdP access token (machine-to-machine);
// that path too is offline crypto against the cached IdP JWKS. Authenticate never
// blocks an unauthenticated or stale-token request; the guards (RequireAdmin,
// RequirePublisherRole) gate writes.
type Authenticator struct {
	tokens *TokenAuthority
	idp    ServiceTokenVerifier
}

// NewAuthenticator builds the bearer authenticator. A nil TokenAuthority (e.g.
// route-walk tests that construct the router with no signing key) makes
// Authenticate a pass-through. A nil ServiceTokenVerifier disables direct
// acceptance of IdP service-account tokens (the default).
func NewAuthenticator(tokens *TokenAuthority, idp ServiceTokenVerifier) *Authenticator {
	return &Authenticator{tokens: tokens, idp: idp}
}

// Authenticate is chi middleware that resolves a valid bearer token into a
// Principal. A missing, malformed, expired, or otherwise invalid token proceeds
// unauthenticated (public reads still work; the SPA refreshes on the resulting
// 401 from a guarded route).
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.tokens == nil {
			next.ServeHTTP(w, r)
			return
		}
		raw := bearerToken(r)
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := a.tokens.Verify(raw)
		if err != nil {
			// Not a registry token. When configured, give it a second chance as a
			// directly-presented IdP service-account token (M2M).
			if princ := a.serviceTokenPrincipal(r.Context(), raw); princ != nil {
				next.ServeHTTP(w, r.WithContext(contextWithPrincipal(r.Context(), princ)))
				return
			}
			// Invalid / expired token: proceed unauthenticated rather than 401,
			// so the contract ("Authenticate never blocks") holds and the guards
			// produce the 401 that drives the SPA's silent refresh.
			next.ServeHTTP(w, r)
			return
		}

		princ := &Principal{
			UserID:        claims.Subject,
			Email:         claims.Email,
			ClaimGroups:   claims.Groups,
			IsServerAdmin: claims.SrvAdmin,
			AuthMethod:    claims.AuthMethod,
		}
		next.ServeHTTP(w, r.WithContext(contextWithPrincipal(r.Context(), princ)))
	})
}

// serviceTokenPrincipal resolves a directly-presented IdP service-account token
// into a Principal, or nil when no verifier is configured or the token is not a
// valid service token. The principal carries no registry users row (its UserID
// is the IdP subject): authorization runs off the claim groups and the
// realm-admin flag, exactly like a federated group-writer with no provisioned
// row (see IdentityFromContext).
func (a *Authenticator) serviceTokenPrincipal(ctx context.Context, raw string) *Principal {
	if a.idp == nil {
		return nil
	}
	id, err := a.idp.VerifyServiceToken(ctx, raw)
	if err != nil {
		return nil
	}
	return &Principal{
		UserID:        id.Subject,
		Email:         id.Email,
		ClaimGroups:   id.Groups,
		IsServerAdmin: id.IsAdmin,
		AuthMethod:    "oidc",
	}
}

// contextWithPrincipal stores the resolved principal and the synthesized claims
// the older context helpers (ClaimsFromContext, IsServerAdminFromContext,
// IdentityFromContext, auditActor) read. Both the registry-token and the
// service-token paths funnel through here so they expose an identical context
// shape downstream.
func contextWithPrincipal(ctx context.Context, princ *Principal) context.Context {
	kc := &OIDCClaims{Email: princ.Email, Groups: princ.ClaimGroups}
	kc.Subject = princ.UserID
	ctx = context.WithValue(ctx, principalKey, princ)
	ctx = context.WithValue(ctx, claimsKey, kc)
	ctx = context.WithValue(ctx, isAdminKey, princ.IsServerAdmin)
	return ctx
}

// bearerToken extracts the token from an `Authorization: Bearer <token>` header,
// or "" when absent / malformed.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// RequireAdmin is chi middleware that gates a route to Server Admins. It must be
// chained after Authenticate. Server Admin is dual-sourced: the snapshotted
// claim flag OR the local is_server_admin flag (baked into the token at login),
// via IsServerAdminFromContext.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Missing or invalid bearer token", r.URL.Path)
			return
		}
		if !IsServerAdminFromContext(r.Context()) {
			problem.Write(w, http.StatusForbidden, "forbidden",
				"Insufficient permissions: Server Admin required", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClaimsFromContext retrieves the synthesized claims for the authenticated
// caller (nil/false when unauthenticated).
func ClaimsFromContext(ctx context.Context) (*OIDCClaims, bool) {
	c, ok := ctx.Value(claimsKey).(*OIDCClaims)
	return c, ok
}

// IsAdminFromContext reports whether the current request is authenticated as admin.
func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(isAdminKey).(bool)
	return v
}

// ContextWithClaims injects decoded token claims into a context. Used in tests
// to simulate an authenticated, group-bearing caller without a real token.
// Claims never carry the Server-Admin decision (that lives on Principal), so
// this does not mark the caller admin — use ContextWithPrincipal for that.
func ContextWithClaims(ctx context.Context, claims *OIDCClaims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}
