package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/haibread/ai-registry/internal/problem"
)

type contextKey string

const (
	claimsKey  contextKey = "auth_claims"
	isAdminKey contextKey = "auth_is_admin"
)

// Validator validates incoming JWTs and populates request context with claims.
type Validator struct {
	jwks     *JWKSCache
	issuer   string
	audience string
}

// NewValidator creates a Validator using the provided JWKSCache and issuer.
// When audience is non-empty, tokens whose `aud` claim does not contain it are
// rejected — required by the MCP authorization spec (OAuth 2.1 resource
// indicators) to prevent cross-client token reuse.
func NewValidator(jwks *JWKSCache, issuer, audience string) *Validator {
	return &Validator{jwks: jwks, issuer: issuer, audience: audience}
}

// Authenticate is chi middleware that parses the Bearer token when present.
// It does NOT block requests without a token — unauthenticated requests
// proceed with no claims in context. Use RequireAdmin to gate write endpoints.
func (v *Validator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		claims := &KeycloakClaims{}
		parseOpts := []jwt.ParserOption{
			jwt.WithIssuedAt(),
			jwt.WithIssuer(v.issuer),
			jwt.WithExpirationRequired(),
		}
		if v.audience != "" {
			parseOpts = append(parseOpts, jwt.WithAudience(v.audience))
		}
		_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			kid, _ := t.Header["kid"].(string)
			return v.jwks.GetKey(r.Context(), kid)
		}, parseOpts...)

		if err != nil {
			// A token was provided but is invalid (expired, bad signature, etc.).
			// Return 401 rather than silently treating it as unauthenticated,
			// so clients with broken tokens get immediate diagnostic feedback.
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Invalid or expired bearer token", r.URL.Path)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		ctx = context.WithValue(ctx, isAdminKey, claims.IsAdmin())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is chi middleware that returns 401/403 if the request is not
// authenticated as an admin. It must be chained after Authenticate.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Missing or invalid bearer token", r.URL.Path)
			return
		}
		if !claims.IsAdmin() {
			problem.Write(w, http.StatusForbidden, "forbidden",
				"Insufficient permissions: registry:admin role required", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClaimsFromContext retrieves the parsed Keycloak claims from the context.
func ClaimsFromContext(ctx context.Context) (*KeycloakClaims, bool) {
	c, ok := ctx.Value(claimsKey).(*KeycloakClaims)
	return c, ok
}

// IsAdminFromContext reports whether the current request is authenticated as admin.
func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(isAdminKey).(bool)
	return v
}

// ContextWithClaims injects claims into a context. Used in tests to simulate
// authenticated requests without a real JWT.
func ContextWithClaims(ctx context.Context, claims *KeycloakClaims) context.Context {
	ctx = context.WithValue(ctx, claimsKey, claims)
	ctx = context.WithValue(ctx, isAdminKey, claims.IsAdmin())
	return ctx
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
